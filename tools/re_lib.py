"""Tiny static-analysis helpers for the 32-bit KMISE exe (capstone + pefile)."""
import re
import struct
import sys

import capstone
import pefile

EXE = r"C:\Users\brohd\kmise_loop\re\unpacked\KMISE LOOP&DRUM V1.0.1 Setup_win.exe"


class Image:
    def __init__(self, path=EXE):
        self.pe = pefile.PE(path, fast_load=True)
        self.base = self.pe.OPTIONAL_HEADER.ImageBase
        self.data = self.pe.get_memory_mapped_image()
        text = next(s for s in self.pe.sections if s.Name.startswith(b".text"))
        self.text_lo = self.base + text.VirtualAddress
        self.text_hi = self.text_lo + text.Misc_VirtualSize
        self.md = capstone.Cs(capstone.CS_ARCH_X86, capstone.CS_MODE_32)
        self.md.detail = False
        self._insns = None

    def va_of(self, needle: bytes):
        return [self.base + m.start() for m in re.finditer(re.escape(needle), self.data)]

    def read(self, va, n):
        o = va - self.base
        return self.data[o:o + n]

    def cstr(self, va, maxlen=200):
        b = self.read(va, maxlen)
        return b.split(b"\0", 1)[0]

    def insns(self):
        """Linear sweep of .text (cached)."""
        if self._insns is None:
            code = self.read(self.text_lo, self.text_hi - self.text_lo)
            out = []
            off = 0
            while off < len(code):
                got = False
                for i in self.md.disasm_lite(code[off:], self.text_lo + off):
                    out.append(i)
                    off = i[0] + i[1] - self.text_lo
                    got = True
                if not got:
                    off += 1
                elif off < len(code):
                    off += 0  # disasm_lite stopped on bad byte; skip it
                    off += 1
            self._insns = out
        return self._insns

    def xrefs_imm(self, va):
        """Instructions whose operand text mentions this VA as an immediate/address."""
        h = hex(va)
        return [i for i in self.insns() if h in i[3]]

    def calls_to(self, va):
        h = hex(va)
        return [i for i in self.insns() if i[2] in ("call", "jmp") and i[3] == h]

    def func_start(self, va):
        """Walk back to the nearest preceding 'push ebp; mov ebp, esp' or int3/ret padding boundary."""
        ins = self.insns()
        idx = self.index(va)
        for j in range(idx, max(idx - 4000, 0), -1):
            a = ins[j]
            if a[2] == "push" and a[3] == "ebp" and j + 1 < len(ins) and ins[j + 1][2] == "mov" and ins[j + 1][3] == "ebp, esp":
                return a[0]
        return None

    def index(self, va):
        import bisect
        ins = self.insns()
        keys = getattr(self, "_keys", None)
        if keys is None:
            keys = self._keys = [i[0] for i in ins]
        return bisect.bisect_right(keys, va) - 1

    def dis(self, va, n=60):
        ins = self.insns()
        j = self.index(va)
        lines = []
        for a in ins[j:j + n]:
            note = ""
            for tok in re.findall(r"0x[0-9a-f]{6,8}", a[3]):
                t = int(tok, 16)
                if self.base <= t < self.base + len(self.data) and not (self.text_lo <= t < self.text_hi):
                    s = self.cstr(t, 80)
                    if len(s) >= 3 and all(32 <= c < 127 for c in s):
                        note = f"   ; {s.decode()!r}"
            lines.append(f"{a[0]:08x}  {a[2]} {a[3]}{note}")
        return "\n".join(lines)

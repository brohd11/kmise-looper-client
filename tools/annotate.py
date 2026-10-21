"""Write an annotated disassembly of the app-code region (imports, libusb names, strings)."""
import json
import re
import subprocess
import sys

import pefile

sys.path.insert(0, ".")
from re_lib import Image

im = Image()
im.pe.parse_data_directories(directories=[pefile.DIRECTORY_ENTRY["IMAGE_DIRECTORY_ENTRY_IMPORT"]])
iat = {}
for entry in im.pe.DIRECTORY_ENTRY_IMPORT:
    for imp in entry.imports:
        nm = imp.name.decode() if imp.name else f"ord{imp.ordinal}"
        iat[imp.address] = f"{entry.dll.decode()}!{nm}"


def demangle(n):
    if "!_Z" not in n:
        return n
    dll, sym = n.split("!", 1)
    try:
        out = subprocess.run(["c++filt"], input=sym, capture_output=True, text=True).stdout.strip()
        return f"{dll}!{out or sym}"
    except FileNotFoundError:
        return n


names = {int(k, 16): v for k, v in json.load(open("libusb_names.json")).items()}
# thunks: jmp dword ptr [IAT]
for i in im.insns():
    m = re.fullmatch(r"dword ptr \[(0x[0-9a-f]+)\]", i[3])
    if i[2] == "jmp" and m and int(m.group(1), 16) in iat:
        names[i[0]] = iat[int(m.group(1), 16)]
cache = {}


def nm(t):
    if t not in cache:
        cache[t] = demangle(names[t]) if t in names else None
    return cache[t]


lo, hi = int(sys.argv[1], 16), int(sys.argv[2], 16)
out = []
for i in im.insns():
    if i[0] < lo:
        continue
    if i[0] >= hi:
        break
    note = ""
    ops = i[3]
    for tok in re.findall(r"0x[0-9a-f]{6,8}", ops):
        t = int(tok, 16)
        if nm(t):
            note = f"   ; {nm(t)}"
        elif t in iat:
            note = f"   ; [{demangle(iat[t])}]"
        elif im.base <= t < im.base + len(im.data) and not (im.text_lo <= t < im.text_hi):
            s = im.cstr(t, 100)
            if len(s) >= 2 and all(32 <= c < 127 or c in (9, 10) for c in s):
                note = f"   ; {s.decode()!r}"
    if i[2] == "push" and ops == "ebp":
        out.append("")
    out.append(f"{i[0]:08x}  {i[2]:6} {ops}{note}")
open(sys.argv[3], "w").write("\n".join(out))
print(len(out), "lines")

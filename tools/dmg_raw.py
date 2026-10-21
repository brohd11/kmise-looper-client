"""Decompress a UDIF .dmg into a raw disk image (zlib/bzip2/raw/zero chunks)."""
import bz2
import plistlib
import struct
import sys
import zlib

src, dst = sys.argv[1], sys.argv[2]
f = open(src, "rb")
f.seek(-512, 2)
koly = f.read(512)
xml_off, xml_len = struct.unpack_from(">QQ", koly, 0xD8)
f.seek(xml_off)
plist = plistlib.loads(f.read(xml_len))
out = open(dst, "wb")
for part in plist["resource-fork"]["blkx"]:
    mish = part["Data"]
    sector0, = struct.unpack_from(">Q", mish, 8)
    nchunks, = struct.unpack_from(">I", mish, 0xC8)
    for i in range(nchunks):
        typ, _, sec, nsec, coff, clen = struct.unpack_from(">IIQQQQ", mish, 0xCC + 40 * i)
        out.seek((sector0 + sec) * 512)
        if typ in (0, 2):  # zero / ignored
            continue
        if typ == 0xFFFFFFFF or typ == 0x7FFFFFFE:  # end / comment
            continue
        f.seek(coff)
        data = f.read(clen)
        if typ == 1:
            out.write(data)
        elif typ == 0x80000005:
            out.write(zlib.decompress(data))
        elif typ == 0x80000006:
            out.write(bz2.decompress(data))
        else:
            sys.exit(f"unsupported chunk type {typ:#x} in {part.get('Name')}")
    print(part.get("Name"), "ok")
out.close()

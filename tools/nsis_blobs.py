"""Dump the data blobs of an NSIS installer (solid or non-solid LZMA/zlib/bzip2), without running it."""
import bz2
import lzma
import os
import struct
import sys
import zlib

src, out_dir = sys.argv[1], sys.argv[2]
d = open(src, "rb").read()
sig = d.find(b"\xef\xbe\xad\xdeNullsoftInst")
if sig < 4:
    sys.exit("no NSIS firstheader")
fh = sig - 4
flags, _, _, hdr_len, arc_len = struct.unpack_from("<I4s12sII", d, fh)
data = d[fh + 28: fh + arc_len]
print(f"firstheader @{fh:#x} flags={flags:#x} header={hdr_len} archive={arc_len}")


def decomp(buf):
    if buf[:3] == b"\x5d\x00\x00":
        return lzma.LZMADecompressor(lzma.FORMAT_RAW, filters=[{
            "id": lzma.FILTER_LZMA1, **lzma._decode_filter_properties(lzma.FILTER_LZMA1, buf[:5])}]).decompress(buf[5:])
    if buf[:2] == b"BZ" or buf[:1] == b"1":
        return bz2.decompress(buf)
    return zlib.decompressobj(-15).decompress(buf)


os.makedirs(out_dir, exist_ok=True)
(first,) = struct.unpack_from("<I", data, 0)
if first & 0x80000000 or first == hdr_len:  # non-solid: each block is [len|0x80000000][compressed]
    blocks, off = [], 0
    while off + 4 <= len(data):
        (n,) = struct.unpack_from("<I", data, off)
        comp, n = bool(n & 0x80000000), n & 0x7FFFFFFF
        raw = data[off + 4: off + 4 + n]
        blocks.append(decomp(raw) if comp else raw)
        off += 4 + n
else:  # solid: one compressed stream of [len][bytes] records
    stream = decomp(data)
    blocks, off = [], 0
    while off + 4 <= len(stream):
        (n,) = struct.unpack_from("<I", stream, off)
        blocks.append(stream[off + 4: off + 4 + n])
        off += 4 + n
for i, b in enumerate(blocks):
    open(os.path.join(out_dir, f"{i:04d}.bin"), "wb").write(b)
print(len(blocks), "blocks; block 0 is the NSIS header")

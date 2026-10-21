"""Extract an Electron app.asar (no node needed)."""
import json
import os
import struct
import sys

src, out = sys.argv[1], sys.argv[2]
f = open(src, "rb")
_, hdr_size, _, json_len = struct.unpack("<IIII", f.read(16))
header = json.loads(f.read(json_len))
base = 8 + hdr_size


def walk(node, path):
    for name, ent in node.get("files", {}).items():
        p = os.path.join(path, name)
        if "files" in ent:
            walk(ent, p)
        elif not ent.get("unpacked"):
            os.makedirs(os.path.dirname(p), exist_ok=True)
            f.seek(base + int(ent["offset"]))
            open(p, "wb").write(f.read(ent["size"]))


walk(header, out)

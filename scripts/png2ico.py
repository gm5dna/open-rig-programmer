#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Assemble a Windows .ico whose entries are the given PNG files, stored as PNG.

Usage: png2ico.py OUT.ico 256.png 128.png ... 16.png

ImageMagick writes ICO entries as uncompressed bitmaps (372 KB for the seven-size
set); Windows Vista and later read PNG-compressed entries, which is what the
Wails template shipped (21 KB). Standard library only, byte-deterministic.
"""
import struct
import sys


def png_size(data):
    if data[:8] != b"\x89PNG\r\n\x1a\n" or data[12:16] != b"IHDR":
        raise SystemExit("png2ico: not a PNG")
    w, h = struct.unpack(">II", data[16:24])
    return w, h


def main(argv):
    if len(argv) < 3:
        raise SystemExit(__doc__)
    out, srcs = argv[1], argv[2:]
    blobs = [open(p, "rb").read() for p in srcs]
    header = struct.pack("<HHH", 0, 1, len(blobs))
    offset = len(header) + 16 * len(blobs)
    entries = []
    for data in blobs:
        w, h = png_size(data)
        if w > 256 or h > 256:
            raise SystemExit("png2ico: ICO entries are at most 256 px")
        # width/height bytes are 0 for 256; colour count 0; reserved 0; planes 1; bpp 32
        entries.append(struct.pack("<BBBBHHII", w % 256, h % 256, 0, 0, 1, 32, len(data), offset))
        offset += len(data)
    with open(out, "wb") as f:
        f.write(header)
        f.writelines(entries)
        f.writelines(blobs)


if __name__ == "__main__":
    main(sys.argv)

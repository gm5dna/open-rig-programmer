#!/usr/bin/env python3
# SPDX-License-Identifier: GPL-3.0-or-later
"""Assemble a Windows .ico whose entries are the given PNG files, stored as PNG.

Usage: png2ico.py OUT.ico 256.png 128.png ... 16.png

ImageMagick writes ICO entries as uncompressed bitmaps (372 KB for the seven-size
set); Windows Vista and later read PNG-compressed entries, which is what the
Wails template shipped (21 KB). Standard library only, byte-deterministic.
Inputs must be 8-bit RGBA PNGs (what rsvg-convert emits) of at most 256 px,
with no two the same size; every input is validated before OUT.ico is opened.
"""
import struct
import sys


def png_header(data):
    if data[:8] != b"\x89PNG\r\n\x1a\n" or data[12:16] != b"IHDR":
        raise SystemExit("png2ico: not a PNG")
    w, h, depth, colour = struct.unpack(">IIBB", data[16:26])
    if (depth, colour) != (8, 6):
        raise SystemExit("png2ico: entries must be 8-bit RGBA (colour type 6)")
    return w, h


def main(argv):
    if len(argv) < 3:
        raise SystemExit(__doc__)
    out, srcs = argv[1], argv[2:]
    blobs = []
    for p in srcs:
        with open(p, "rb") as f:
            blobs.append(f.read())
    header = struct.pack("<HHH", 0, 1, len(blobs))
    offset = len(header) + 16 * len(blobs)
    entries, seen = [], set()
    for data in blobs:
        w, h = png_header(data)
        if w > 256 or h > 256:
            raise SystemExit("png2ico: ICO entries are at most 256 px")
        if (w, h) in seen:
            raise SystemExit("png2ico: duplicate entry size %dx%d" % (w, h))
        seen.add((w, h))
        # width/height bytes are 0 for 256; colour count 0; reserved 0; planes 1; bpp 32
        entries.append(struct.pack("<BBBBHHII", w % 256, h % 256, 0, 0, 1, 32, len(data), offset))
        offset += len(data)
    with open(out, "wb") as f:
        f.write(header)
        f.writelines(entries)
        f.writelines(blobs)


if __name__ == "__main__":
    main(sys.argv)

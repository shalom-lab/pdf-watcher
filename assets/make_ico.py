"""Rasterize logo.svg geometry into multi-size icon.ico for the tray."""
from __future__ import annotations

import io
import os
import struct

from PIL import Image, ImageDraw

OUT_ICO = os.path.join(os.path.dirname(__file__), "icon.ico")
OUT_PNG = os.path.join(os.path.dirname(__file__), "icon.png")
TEAL = (15, 118, 110, 255)  # #0F766E
CORAL = (224, 122, 95, 255)  # #E07A5F
SIZES = [16, 24, 32, 48, 64, 128, 256]


def draw_logo(size: int) -> Image.Image:
    im = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(im)
    # Match enlarged logo.svg: ~9% padding, thicker stroke, larger pupil.
    pad = size * 0.09
    cx = cy = size / 2
    rx = (size - 2 * pad) / 2
    ry = rx * 0.62
    stroke = max(2, int(round(size * (100 / 1024))))
    bbox = [cx - rx, cy - ry, cx + rx, cy + ry]
    draw.ellipse(bbox, outline=TEAL, width=stroke)
    pr = size * (148 / 1024)
    draw.ellipse([cx - pr, cy - pr, cx + pr, cy + pr], fill=CORAL)
    return im


def png_bytes(img: Image.Image) -> bytes:
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()


def write_ico(path: str, images: list[tuple[int, bytes]]) -> None:
    header = struct.pack("<HHH", 0, 1, len(images))
    offset = 6 + 16 * len(images)
    directory = b""
    blob = b""
    for s, data in images:
        wb = 0 if s >= 256 else s
        hb = 0 if s >= 256 else s
        directory += struct.pack("<BBBBHHII", wb, hb, 0, 0, 1, 32, len(data), offset)
        blob += data
        offset += len(data)
    with open(path, "wb") as f:
        f.write(header + directory + blob)


def main() -> None:
    entries = [(s, png_bytes(draw_logo(s))) for s in SIZES]
    write_ico(OUT_ICO, entries)
    draw_logo(256).save(OUT_PNG)
    verify = Image.open(OUT_ICO)
    print("wrote", OUT_ICO, os.path.getsize(OUT_ICO))
    print("sizes", list(verify.ico.sizes()))


if __name__ == "__main__":
    main()

#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Derive every raster icon from the one vector source, app/build/appicon.svg.
# Nothing under app/build is hand-edited: run this after changing the SVG and
# commit the outputs together with it.
#
#   app/build/appicon.png                       1024x1024; wails build derives the
#                                               macOS iconfile.icns from it
#   app/build/windows/icon.ico                  256/128/64/48/32/24/16, the set
#                                               the Wails template shipped plus 48
#   app/build/linux/open-rig-programmer-512.png the hicolor 512x512 entry (nfpm.yaml)
#   app/build/linux/open-rig-programmer.svg     the hicolor scalable entry (nfpm.yaml)
#
# Usage: scripts/render-icons.sh            render into the tree
#        scripts/render-icons.sh --check    render to a scratch dir and fail if
#                                           any tree file differs (pixel-exact)
#
# Needs rsvg-convert (librsvg) and python3 (for scripts/png2ico.py, which
# stores the ICO entries as PNG — ImageMagick's writer stores bitmaps, 17x the
# size). Both fail loudly when absent; there is deliberately no fallback
# renderer so two machines cannot disagree on what "derived" means.
set -u
here=$(cd "$(dirname "$0")/.." && pwd)
src="$here/app/build/appicon.svg"
mode=${1:-render}
case "$mode" in render|--check) ;; *) echo "usage: $0 [--check]" >&2; exit 2 ;; esac

for tool in rsvg-convert python3; do
  command -v "$tool" >/dev/null 2>&1 || { echo "render-icons: $tool not found" >&2; exit 1; }
done
[ -s "$src" ] || { echo "render-icons: $src missing or empty" >&2; exit 1; }

if [ "$mode" = "--check" ]; then
  out=$(mktemp -d) || exit 1
  trap 'rm -rf "$out"' EXIT
  mkdir -p "$out/windows" "$out/linux"
else
  out="$here/app/build"
fi

png() { # png <size> <dest>
  rsvg-convert -w "$1" -h "$1" "$src" -o "$2" || { echo "render-icons: rsvg-convert failed at $1 px" >&2; exit 1; }
}

png 1024 "$out/appicon.png"
png 512 "$out/linux/open-rig-programmer-512.png"
cp "$src" "$out/linux/open-rig-programmer.svg" || exit 1

tmp=$(mktemp -d) || exit 1
for s in 256 128 64 48 32 24 16; do png "$s" "$tmp/$s.png"; done
python3 "$here/scripts/png2ico.py" "$out/windows/icon.ico" \
  "$tmp/256.png" "$tmp/128.png" "$tmp/64.png" "$tmp/48.png" "$tmp/32.png" "$tmp/24.png" "$tmp/16.png" \
  || { echo "render-icons: ico assembly failed" >&2; rm -rf "$tmp"; exit 1; }
rm -rf "$tmp"

for f in appicon.png windows/icon.ico linux/open-rig-programmer-512.png linux/open-rig-programmer.svg; do
  [ -s "$out/$f" ] || { echo "render-icons: $f came out empty" >&2; exit 1; }
done

if [ "$mode" = "--check" ]; then
  status=0
  for f in appicon.png windows/icon.ico linux/open-rig-programmer-512.png linux/open-rig-programmer.svg; do
    if ! cmp -s "$out/$f" "$here/app/build/$f"; then
      echo "render-icons: app/build/$f differs from a fresh render of appicon.svg" >&2; status=1
    fi
  done
  [ "$status" -eq 0 ] && echo "render-icons: tree matches appicon.svg"
  exit "$status"
fi
echo "render-icons: wrote appicon.png, windows/icon.ico, linux/open-rig-programmer-512.png, linux/open-rig-programmer.svg"

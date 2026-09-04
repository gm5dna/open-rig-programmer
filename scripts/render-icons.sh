#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Derive every raster icon from the one vector source, app/build/appicon.svg.
# Nothing under app/build is hand-edited: run this after changing the SVG and
# commit the outputs together with it.
#
#   app/build/appicon.png                        1024x1024; wails build derives the
#                                                macOS iconfile.icns from it
#   app/build/windows/icon.ico                   256/128/64/48/32/24/16, PNG entries
#   app/build/linux/open-rig-programmer-512.png  the hicolor 512x512 entry (nfpm.yaml
#                                                ships appicon.svg itself as the
#                                                scalable entry)
#
# Usage: scripts/render-icons.sh            render into the tree (from any cwd)
#        scripts/render-icons.sh --check    render to a scratch dir and fail if
#                                           any tree file is not byte-identical
#
# Needs rsvg-convert (librsvg) and python3 (for scripts/png2ico.py, which
# stores the ICO entries as PNG — ImageMagick's writer stores bitmaps, 17x the
# size). Both fail loudly when absent.
#
# The committed bytes were rendered by librsvg 2.62.3 / cairo 1.18.4 (Homebrew,
# 04/09/2026). --check is a BYTE comparison, and antialiasing differs between
# librsvg/cairo versions, so a mismatch on another machine may be a harmless
# renderer difference rather than a stale tree: the mismatch message prints the
# local rsvg-convert version for that reason. Re-render and commit only when
# the SVG actually changed.
set -u
here=$(cd "$(dirname "$0")/.." && pwd)
src="$here/app/build/appicon.svg"
mode=${1:-render}
case "$mode" in render|--check) ;; *) echo "usage: $0 [--check]" >&2; exit 2 ;; esac

# Declared once; every loop below iterates these.
outputs=(appicon.png windows/icon.ico linux/open-rig-programmer-512.png)
ico_sizes=(256 128 64 48 32 24 16)

for tool in rsvg-convert python3; do
  command -v "$tool" >/dev/null 2>&1 || { echo "render-icons: $tool not found" >&2; exit 1; }
done
[ -s "$src" ] || { echo "render-icons: $src missing or empty" >&2; exit 1; }

# Every render goes to a scratch tree first, in both modes: a failure part-way
# must never leave app/build with files from two different renders. One EXIT
# trap owns the scratch paths because png() exits from inside a failed render.
scratch=""
cleanup() { [ -n "$scratch" ] && rm -rf "$scratch"; return 0; }
trap cleanup EXIT
scratch=$(mktemp -d) || { echo "render-icons: mktemp failed" >&2; exit 1; }
out="$scratch/build"
mkdir -p "$out/windows" "$out/linux" "$scratch/ico" || { echo "render-icons: cannot create scratch tree" >&2; exit 1; }

png() { # png <size> <dest>
  rsvg-convert -w "$1" -h "$1" "$src" -o "$2" || { echo "render-icons: rsvg-convert failed at $1 px" >&2; exit 1; }
}

png 1024 "$out/appicon.png"
png 512 "$out/linux/open-rig-programmer-512.png"

ico_inputs=()
for s in "${ico_sizes[@]}"; do png "$s" "$scratch/ico/$s.png"; ico_inputs+=("$scratch/ico/$s.png"); done
python3 "$here/scripts/png2ico.py" "$out/windows/icon.ico" "${ico_inputs[@]}" \
  || { echo "render-icons: ico assembly failed" >&2; exit 1; }

for f in "${outputs[@]}"; do
  [ -s "$out/$f" ] || { echo "render-icons: $f came out empty" >&2; exit 1; }
done

if [ "$mode" = "--check" ]; then
  status=0
  for f in "${outputs[@]}"; do
    if ! cmp -s "$out/$f" "$here/app/build/$f"; then
      echo "render-icons: app/build/$f differs from a fresh render of appicon.svg (local $(rsvg-convert --version 2>/dev/null | head -1); see the header note on renderer versions)" >&2
      status=1
    fi
  done
  [ "$status" -eq 0 ] && echo "render-icons: tree matches appicon.svg"
  exit "$status"
fi

for f in "${outputs[@]}"; do
  cp "$out/$f" "$here/app/build/$f" || { echo "render-icons: cannot write app/build/$f" >&2; exit 1; }
done
echo "render-icons: wrote ${outputs[*]}"

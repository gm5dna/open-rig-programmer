#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
#
# shellcheck disable=SC2016
# (the single-quoted grep patterns below match a literal `$INSTDIR`,
# `$TEMP` etc. inside project.nsi's own NSIS syntax — no shell
# expansion is wanted there, so single quotes are correct)
#
# Asserts the four Windows release assets (two NSIS installers, two CLI
# zips, one pair per arch) are exactly what the packaging design
# promises: the installer really contains the CLI it claims to carry,
# the extracted bytes match what was built, and project.nsi's uninstall
# section still does not delete a user-editable directory tree. Used by
# the release workflow after `wails build -nsis` and by the local
# rehearsal; runs under Git Bash on the runner and under bash on macOS
# with `7zz`. Usage: check-windows.sh <tag> <assets-dir>
#
# Run with cwd = repo root. Expects:
#   app/build/bin/open-rig-programmer-{amd64,arm64}.exe   (raw Wails GUI output)
#   app/dist/{amd64,arm64}/rigprog.exe                     (CLI, built before wails -nsis)
#   app/build/windows/installer/project.nsi
#   <assets-dir>/open-rig-programmer-<tag>-windows-{amd64,arm64}-installer.exe
#   <assets-dir>/rigprog-<tag>-windows-{amd64,arm64}.zip
#
# CHECK_WIN_SKIP_TAG=1 skips only the tag-string assertion (unstamped
# local builds carry "v0.0.0-check", not a real tag) — never set in CI.
set -u

tag="${1:?usage: check-windows.sh <tag> <assets-dir>}"
assets="${2:?usage: check-windows.sh <tag> <assets-dir>}"
nsi="app/build/windows/installer/project.nsi"
scratch="app/build/bin/check-tmp"

fail=0
err() { echo "FAIL: $*" >&2; fail=1; }

# 7-Zip lookup: `7z` on the runner's Git Bash, `7zz` (Homebrew sevenzip)
# on macOS — never both, so the first one found on PATH wins.
SEVENZ=""
if command -v 7z >/dev/null 2>&1; then
  SEVENZ="7z"
elif command -v 7zz >/dev/null 2>&1; then
  SEVENZ="7zz"
else
  echo "7-Zip not found" >&2
  exit 2
fi

# PE machine field: e_lfanew (the offset of the PE header) is a
# little-endian u32 at file offset 60; the machine word is a u16 at
# e_lfanew+4. `od -t u2/x2` reads multi-byte units in the host's native
# byte order, which is little-endian on both the runner and this Mac,
# so this recipe needs no explicit byte-swap on either.
pe_machine() {
  local f="$1" off
  off=$(od -An -t u4 -j 60 -N 4 "$f" | tr -d ' ')
  od -An -t x2 -j $((off + 4)) -N 2 "$f" | tr -d ' '
}

want_machine() {
  case "$1" in
    amd64) echo "8664" ;;
    arm64) echo "aa64" ;;
    *) echo "NO-SUCH-MACHINE" ;;
  esac
}

check_machine() {
  local f="$1" arch="$2" label="$3" got want
  [ -f "$f" ] || { err "$label missing: $f"; return; }
  got="$(pe_machine "$f")"
  want="$(want_machine "$arch")"
  [ "$got" = "$want" ] || err "$label ($f) has PE machine $got, want $want ($arch)"
}

# SHA-256 tool lookup: `sha256sum` (the runner's house tool, coreutils via
# Git Bash) first, else `shasum -a 256` (macOS). Resolved ONCE up front,
# same shape as the 7-Zip lookup above — not `shasum ... || sha256sum ...`
# piped through awk, which binds `||` to the pipeline (awk's exit status,
# not shasum's) and so never fires: when shasum is absent, awk reads empty
# input, exits 0, and sha() silently returns "", making every SHA-256 tie
# in this script compare "" = "" and pass.
if command -v sha256sum >/dev/null 2>&1; then
  _sha_raw() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  _sha_raw() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  echo "no sha256sum/shasum found" >&2
  exit 2
fi

# Fail loudly rather than degrade: a resolved tool that still produces no
# (or malformed) output — an unreadable file, a broken pipe — must not be
# allowed to silently make a tie compare "" = "".
sha() {
  local digest
  digest="$(_sha_raw "$1")"
  case "$digest" in
    [0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F]*) ;;
    *) echo "sha() produced no usable SHA-256 digest for: $1" >&2; exit 2 ;;
  esac
  printf '%s\n' "$digest"
}

for arch in amd64 arm64; do
  raw_gui="app/build/bin/open-rig-programmer-${arch}.exe"
  dist_cli="app/dist/${arch}/rigprog.exe"
  installer="${assets}/open-rig-programmer-${tag}-windows-${arch}-installer.exe"
  zip="${assets}/rigprog-${tag}-windows-${arch}.zip"

  [ -f "$raw_gui" ] || err "raw GUI exe missing: $raw_gui"
  [ -f "$dist_cli" ] || err "dist CLI missing: $dist_cli"
  [ -f "$installer" ] || err "installer asset missing: $installer"
  [ -f "$zip" ] || err "zip asset missing: $zip"

  if [ -f "$raw_gui" ]; then check_machine "$raw_gui" "$arch" "raw GUI exe"; fi
  if [ -f "$dist_cli" ]; then check_machine "$dist_cli" "$arch" "dist CLI"; fi

  if [ -f "$installer" ]; then
    xdir="${scratch}/${arch}"
    rm -rf "$xdir"
    mkdir -p "$xdir"
    "$SEVENZ" x -o"$xdir" -y "$installer" >/dev/null || err "extraction failed for $installer"

    ex_gui="${xdir}/Open Rig Programmer.exe"
    ex_cli="${xdir}/rigprog.exe"
    ex_lic="${xdir}/LICENSE"
    [ -f "$ex_gui" ] || err "installer ($arch) does not contain 'Open Rig Programmer.exe'"
    [ -f "$ex_cli" ] || err "installer ($arch) does not contain rigprog.exe"
    [ -f "$ex_lic" ] || err "installer ($arch) does not contain LICENSE"

    if [ -f "$ex_cli" ] && [ -f "$dist_cli" ]; then
      [ "$(sha "$ex_cli")" = "$(sha "$dist_cli")" ] \
        || err "installer ($arch) rigprog.exe does not match $dist_cli (sha256 differs)"
      check_machine "$ex_cli" "$arch" "installer ($arch) rigprog.exe"
    fi
    if [ -f "$ex_gui" ] && [ -f "$raw_gui" ]; then
      [ "$(sha "$ex_gui")" = "$(sha "$raw_gui")" ] \
        || err "installer ($arch) 'Open Rig Programmer.exe' does not match $raw_gui (sha256 differs)"
      check_machine "$ex_gui" "$arch" "installer ($arch) GUI exe"
    fi
  fi

  if [ -f "$zip" ]; then
    listing="$("$SEVENZ" l -ba "$zip" 2>/dev/null)"
    lines="$(printf '%s\n' "$listing" | grep -c .)"
    [ "$lines" -eq 1 ] || err "zip ($arch) lists $lines entries, want exactly 1"
    name="$(printf '%s\n' "$listing" | awk '{print $NF}')"
    [ "$name" = "rigprog.exe" ] || err "zip ($arch) entry is '$name', want rigprog.exe"

    zdir="${scratch}/${arch}-zip"
    rm -rf "$zdir"
    mkdir -p "$zdir"
    "$SEVENZ" x -o"$zdir" -y "$zip" >/dev/null || err "extraction failed for $zip"
    ex_zip_cli="${zdir}/rigprog.exe"
    if [ -f "$ex_zip_cli" ] && [ -f "$dist_cli" ]; then
      [ "$(sha "$ex_zip_cli")" = "$(sha "$dist_cli")" ] \
        || err "zip ($arch) rigprog.exe does not match $dist_cli (sha256 differs)"
    else
      err "zip ($arch) does not contain rigprog.exe"
    fi
  fi

  if [ "${CHECK_WIN_SKIP_TAG:-0}" != "1" ]; then
    if [ -f "$raw_gui" ]; then
      LC_ALL=C grep -qa -- "$tag" "$raw_gui" || err "tag '$tag' not found in raw GUI exe ($arch)"
    fi
    if [ -f "$dist_cli" ]; then
      LC_ALL=C grep -qa -- "$tag" "$dist_cli" || err "tag '$tag' not found in dist CLI ($arch)"
    fi
  fi
done
if [ "${CHECK_WIN_SKIP_TAG:-0}" = "1" ]; then
  echo "check-windows: SKIPPED tag-string assertion (CHECK_WIN_SKIP_TAG=1)"
fi

# nsi source assertions. Grep patterns tolerate the mix of quote styles
# and incidental whitespace NSIS scripts accumulate; they are not a
# full NSIS parse. Full-line comments (this file's own explanatory
# comments included — they discuss the very RMDir/rigprog lines being
# asserted about) are stripped first so a comment can never fake a
# pass or a fail; a code line's own trailing comment is left in place
# since it follows real code on the same line.
if [ -f "$nsi" ]; then
  code="$(grep -vE '^[[:space:]]*#' "$nsi")"
  code_numbered="$(grep -nvE '^[[:space:]]*#' "$nsi")"

  printf '%s\n' "$code" | grep -Eiq 'RMDir[[:space:]]*/r[[:space:]]*"?\$INSTDIR' \
    && err "nsi: found a recursive RMDir /r on \$INSTDIR (classic NSIS hazard)"
  printf '%s\n' "$code" | grep -Eiq 'RMDir[[:space:]]*/r[[:space:]]*"?\$(AppData|APPDATA)' \
    && err "nsi: found a recursive RMDir /r under \$AppData (deletes user data)"

  temp_line="$(printf '%s\n' "$code_numbered" | grep -E 'SetOutPath[[:space:]]+"\$TEMP"' | head -1 | cut -d: -f1)"
  instdir_line="$(printf '%s\n' "$code_numbered" | grep -E 'RMDir[[:space:]]+"\$INSTDIR"' | head -1 | cut -d: -f1)"
  if [ -z "$temp_line" ]; then
    err "nsi: no SetOutPath \"\$TEMP\" line found (needed before removing \$INSTDIR)"
  elif [ -z "$instdir_line" ]; then
    err "nsi: no non-recursive RMDir \"\$INSTDIR\" line found"
  elif [ "$temp_line" -ge "$instdir_line" ]; then
    err "nsi: SetOutPath \"\$TEMP\" (line $temp_line) does not precede RMDir \"\$INSTDIR\" (line $instdir_line)"
  fi

  uninstall_block="$(printf '%s\n' "$code" | sed -nE '/Section[[:space:]]+"[Uu]ninstall"/,/SectionEnd/p')"
  if [ -z "$uninstall_block" ]; then
    err "nsi: no uninstall Section found"
  else
    bad_rigprog="$(printf '%s\n' "$uninstall_block" | grep -i 'rigprog' | grep -Ev 'Delete[[:space:]]+"\$INSTDIR\\rigprog\.exe"')"
    [ -z "$bad_rigprog" ] || err "nsi: uninstall Section mentions rigprog outside the Delete \"\$INSTDIR\\rigprog.exe\" line:
$bad_rigprog"
    printf '%s\n' "$uninstall_block" | grep -Eq 'Delete[[:space:]]+"\$INSTDIR\\rigprog\.exe"' \
      || err "nsi: uninstall Section has no Delete \"\$INSTDIR\\rigprog.exe\" line"
  fi

  amd64_block="$(printf '%s\n' "$code" | sed -n '/IsNativeAMD64/,/EndIf/p')"
  printf '%s\n' "$amd64_block" | grep -Eq 'File[[:space:]].*dist\\amd64\\rigprog\.exe' \
    || err "nsi: no File line for dist\\amd64\\rigprog.exe inside an IsNativeAMD64 branch"
  arm64_block="$(printf '%s\n' "$code" | sed -n '/IsNativeARM64/,/EndIf/p')"
  printf '%s\n' "$arm64_block" | grep -Eq 'File[[:space:]].*dist\\arm64\\rigprog\.exe' \
    || err "nsi: no File line for dist\\arm64\\rigprog.exe inside an IsNativeARM64 branch"

  printf '%s\n' "$code" | grep -Eq 'File[[:space:]].*LICENSE"' \
    || err "nsi: no File line for LICENSE"
else
  err "nsi source missing: $nsi"
fi

[ "$fail" -eq 0 ] && echo "check-windows: all assertions passed"
exit "$fail"

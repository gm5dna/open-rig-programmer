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
# CHECK_WIN_SKIP_TAG=1 skips the tag-string assertion and the raw GUI
# exes' PE version-resource assertions (unstamped local builds carry
# "v0.0.0-check" and 0.0.0/1.0.0 respectively, not a real tag or
# productVersion) — never set in CI.
set -u

tag="${1:?usage: check-windows.sh <tag> <assets-dir>}"
assets="${2:?usage: check-windows.sh <tag> <assets-dir>}"
nsi="app/build/windows/installer/project.nsi"
scratch="app/build/bin/check-tmp"

# Same derivation as release.yml's "Tag shape" step: strip the leading
# `v` and any `-…` pre-release suffix, leaving the plain X.Y.Z the PE
# version resource carries (NSIS's VIProductVersion and PRODUCTVERSION
# only accept four dot/comma-separated integers, never a `v` or a
# suffix).
numeric="${tag#v}"
numeric="${numeric%%-*}"

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

# Reads the PE version resource itself rather than trusting .NET's
# FileVersionInfo lookup: on release run 33950484060 (05/09/2026),
# `(Get-Item $path).VersionInfo.ProductVersion` came back EMPTY for
# both raw GUI exes even though `.rsrc/0/version.txt` held the correct
# StringFileInfo block — the block is language-neutral ("000004b0",
# codepage 1200), which is a shape .NET's FileVersionInfo does not
# resolve; release.yml keeps that .NET read only as a diagnostic now.
# `7z`/`7zz x` extracts the resource as UTF-16LE-with-embedded-NUL
# text; `tr -d '\0\r'` collapses it to plain ASCII for grep.
check_version_resource() {
  local f="$1" arch="$2" label="$3" vdir vtxt txt a b c
  [ -f "$f" ] || { err "$label missing: $f"; return; }
  vdir="${scratch}/${arch}-ver"
  rm -rf "$vdir"
  mkdir -p "$vdir"
  "$SEVENZ" x -y -o"$vdir" "$f" ".rsrc/0/version.txt" >/dev/null 2>&1
  vtxt="${vdir}/.rsrc/0/version.txt"
  [ -f "$vtxt" ] || { err "$label ($f): could not extract .rsrc/0/version.txt"; return; }
  txt="$(tr -d '\0\r' < "$vtxt")"
  IFS=. read -r a b c <<< "$numeric"
  printf '%s\n' "$txt" | grep -qE "VALUE \"ProductVersion\",[[:space:]]+\"${numeric}\"" \
    || err "$label ($f): version resource has no VALUE \"ProductVersion\", \"${numeric}\" (read from $vtxt):
$txt"
  printf '%s\n' "$txt" | grep -qE "^[[:space:]]*FILEVERSION[[:space:]]+${a},${b},${c},0[[:space:]]*\$" \
    || err "$label ($f): version resource has no FILEVERSION ${a},${b},${c},0 line (read from $vtxt):
$txt"
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
    [ -f "$ex_zip_cli" ] || err "zip ($arch) does not contain rigprog.exe"

    if [ -f "$ex_zip_cli" ] && [ -f "$dist_cli" ]; then
      [ "$(sha "$ex_zip_cli")" = "$(sha "$dist_cli")" ] \
        || err "zip ($arch) rigprog.exe does not match $dist_cli (sha256 differs)"
    elif [ -f "$ex_zip_cli" ] && [ ! -f "$dist_cli" ]; then
      err "zip ($arch) rigprog.exe cannot be verified: dist CLI missing: $dist_cli"
    fi
  fi

  if [ "${CHECK_WIN_SKIP_TAG:-0}" != "1" ]; then
    if [ -f "$raw_gui" ]; then
      LC_ALL=C grep -qaF -- "$tag" "$raw_gui" || err "tag '$tag' not found in raw GUI exe ($arch)"
      check_version_resource "$raw_gui" "$arch" "raw GUI exe"
    fi
    if [ -f "$dist_cli" ]; then
      LC_ALL=C grep -qaF -- "$tag" "$dist_cli" || err "tag '$tag' not found in dist CLI ($arch)"
    fi
  fi
done
if [ "${CHECK_WIN_SKIP_TAG:-0}" = "1" ]; then
  echo "check-windows: SKIPPED tag-string and version-resource assertions (CHECK_WIN_SKIP_TAG=1)"
fi

# nsi source assertions. Grep patterns tolerate the mix of quote styles
# and incidental whitespace NSIS scripts accumulate; they are not a
# full NSIS parse.
#
# NSIS has three comment forms — `#`, `;`, and `/* ... */` — and a
# grep that only strips `#` is defeated both ways: a `;` (or a block
# comment) hides a real hazard line from every assertion below (a
# fake pass), while documenting the hazard in prose using either form
# ("; never RMDir /r "$INSTDIR" here") makes an assertion fire on the
# comment itself (a fake fail) — the exact trap this file's own `#`
# comments were already written to avoid. nsi_source() is used by
# EVERY assertion below so neither direction is possible: it folds
# backslash line-continuations, deletes `/* ... */` blocks (which may
# span lines), then walks each remaining line with a quote-tracking
# state machine so only a `#`/`;` outside a double-quoted string ends
# the line — a trailing code-line comment is dropped like a full-line
# one, since NSIS treats both the same way and nothing here needs to
# keep a trailing comment's text.
nsi_source() {
  awk '
    BEGIN { RS="\001"; ORS="" }
    {
      s = $0
      gsub(/\\\n/, " ", s)
      out = ""
      while ((i = index(s, "/*")) > 0) {
        rest = substr(s, i + 2)
        j = index(rest, "*/")
        if (j == 0) { s = substr(s, 1, i - 1); break }
        out = out substr(s, 1, i - 1)
        s = substr(rest, j + 2)
      }
      s = out s
      print s
    }
  ' "$1" | awk '
    {
      line = $0
      n = length(line)
      out = ""
      inq = 0
      for (k = 1; k <= n; k++) {
        c = substr(line, k, 1)
        if (c == "\"") { inq = !inq; out = out c; continue }
        if (!inq && (c == "#" || c == ";")) { break }
        out = out c
      }
      print out
    }
  '
}

if [ -f "$nsi" ]; then
  code="$(nsi_source "$nsi")"

  # Target-specific, single-line, literal-`$INSTDIR`/`$AppData` greps are
  # dodged three ways: `!define`-indirecting the target
  # (`!define UNROOT "$INSTDIR"` then `RMDir /r "${UNROOT}"`), a
  # backslash line-continuation between `/r` and the target, or
  # reordering flags (`RMDir /REBOOTOK /r "..."` — `/r` no longer
  # anchors the match). None of those change what the line does, so the
  # rule is now blanket and target-agnostic: no `RMDir` invocation in
  # the whole nsi may carry a `/r` flag, in any position, on any target
  # — this repository's uninstall Section never needs a recursive
  # remove (see the Section's own comment), so the flag itself is the
  # hazard. Tokenising after nsi_source() (continuations already
  # folded) sidesteps all three dodges at once.
  rmdir_r_hit="$(printf '%s\n' "$code" | awk '
    {
      n = split($0, toks, /[ \t]+/)
      for (i = 1; i <= n; i++) {
        if (toks[i] == "RMDir") {
          for (j = i + 1; j <= n; j++) {
            t = toks[j]
            if (t == "/r" || t == "/R") { print; next }
            if (substr(t, 1, 1) != "/") break
          }
        }
      }
    }
  ')"
  [ -z "$rmdir_r_hit" ] || err "nsi: found a recursive RMDir /r (classic NSIS hazard, any flag order or target):
$rmdir_r_hit"

  # Indirection is not needed anywhere in this nsi and is how the
  # hazard above hides, so ban it outright: no !define's value may name
  # any of the directories a recursive remove would be dangerous on.
  printf '%s\n' "$code" | grep -Eiq '^[[:space:]]*!define[[:space:]]+[A-Za-z0-9_]+[[:space:]]+.*\$(INSTDIR|AppData|APPDATA|LOCALAPPDATA|PROGRAMFILES)' \
    && err "nsi: a !define's value contains \$INSTDIR/\$AppData/\$LOCALAPPDATA/\$PROGRAMFILES (indirection is not needed here and is how the RMDir /r check above gets dodged)"

  uninstall_block="$(printf '%s\n' "$code" | sed -nE '/Section[[:space:]]+"[Uu]ninstall"/,/SectionEnd/p')"
  if [ -z "$uninstall_block" ]; then
    err "nsi: no uninstall Section found"
  else
    # temp_pos/instdir_pos are positions WITHIN the uninstall Section,
    # not the nsi file — the whole-file line numbers this used to
    # report let a $TEMP staged anywhere earlier in the file (e.g. the
    # install Section) satisfy "precedes $INSTDIR" even though the
    # uninstall Section itself never ran it, leaving RMDir "$INSTDIR"
    # to execute with its working directory still inside $INSTDIR.
    temp_pos="$(printf '%s\n' "$uninstall_block" | grep -n 'SetOutPath[[:space:]]\+"\$TEMP"' | head -1 | cut -d: -f1)"
    instdir_pos="$(printf '%s\n' "$uninstall_block" | grep -n 'RMDir[[:space:]]\+"\$INSTDIR"' | head -1 | cut -d: -f1)"
    if [ -z "$temp_pos" ]; then
      err "nsi: no SetOutPath \"\$TEMP\" line found in the uninstall Section (needed before removing \$INSTDIR)"
    elif [ -z "$instdir_pos" ]; then
      err "nsi: no non-recursive RMDir \"\$INSTDIR\" line found in the uninstall Section"
    elif [ "$temp_pos" -ge "$instdir_pos" ]; then
      err "nsi: SetOutPath \"\$TEMP\" (uninstall Section line $temp_pos) does not precede RMDir \"\$INSTDIR\" (line $instdir_pos)"
    fi

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

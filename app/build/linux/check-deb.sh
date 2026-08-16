#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Asserts a built open-rig-programmer .deb is exactly what the
# packaging design promises: identity, dependencies, contents, and
# maintainer scripts. Used by the release workflow after nfpm and
# runnable anywhere dpkg-deb exists (Linux; macOS via `brew install
# dpkg`). Usage: check-deb.sh <deb> <version-without-v> <arch>
set -u

deb="${1:?usage: check-deb.sh <deb> <version> <arch>}"
version="${2:?usage: check-deb.sh <deb> <version> <arch>}"
arch="${3:?usage: check-deb.sh <deb> <version> <arch>}"
here="$(cd "$(dirname "$0")" && pwd)"
fail=0
err() { echo "FAIL: $*" >&2; fail=1; }

command -v dpkg-deb >/dev/null 2>&1 || { echo "dpkg-deb not found" >&2; exit 2; }

# Exact field comparisons (dpkg-deb -f prints the raw control value):
# substring greps would accept extra dependencies or a malformed
# version that happens to contain the expected text.
[ "$(dpkg-deb -f "$deb" Package)" = "open-rig-programmer" ] || err "Package name"
[ "$(dpkg-deb -f "$deb" Version)" = "$version" ] || err "Version (got '$(dpkg-deb -f "$deb" Version)', want '$version')"
[ "$(dpkg-deb -f "$deb" Architecture)" = "$arch" ] || err "Architecture"
[ "$(dpkg-deb -f "$deb" Depends)" = "libwebkit2gtk-4.1-0, libgtk-3-0t64 | libgtk-3-0" ] || err "Depends is not exactly the declared pair (got '$(dpkg-deb -f "$deb" Depends)')"

# Exact set comparison, not a presence check: a per-path search passes a
# package that ships everything expected PLUS something extra (a stray
# /usr/bin/backdoor), and an unanchored match also accepts a renamed
# neighbour (rigprogXYZ contains rigprog). The shipped file list must be
# exactly these six paths — no more, no fewer. Directories are excluded;
# nfpm synthesises the parent tree, which is not a packaging decision.
actual="$(dpkg-deb --fsys-tarfile "$deb" | tar -t | grep -v '/$' | LC_ALL=C sort)"
expected="$(printf '%s\n' \
  ./usr/bin/open-rig-programmer \
  ./usr/bin/rigprog \
  ./usr/share/applications/open-rig-programmer.desktop \
  ./usr/share/icons/hicolor/512x512/apps/open-rig-programmer.png \
  ./usr/lib/udev/rules.d/99-open-rig-programmer.rules \
  ./usr/share/doc/open-rig-programmer/copyright | LC_ALL=C sort)"
[ "$actual" = "$expected" ] || err "package contents differ from the expected set
(< expected, > actual):
$(diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual"))"

ctl="$(mktemp -d)" || exit 2
data="$(mktemp -d)" || exit 2
trap 'rm -rf "$ctl" "$data"' EXIT
dpkg-deb -e "$deb" "$ctl/DEBIAN" || err "control extraction"
for script in postinst postrm; do
  [ -f "$ctl/DEBIAN/$script" ] || { err "$script missing"; continue; }
  grep -q 'command -v udevadm' "$ctl/DEBIAN/$script" || err "$script lacks udevadm guard"
done
dpkg-deb -x "$deb" "$data" || err "data extraction"
diff -q "$here/open-rig-programmer.desktop" \
  "$data/usr/share/applications/open-rig-programmer.desktop" || err "desktop file drifted from repo copy"
diff -q "$here/99-open-rig-programmer.rules" \
  "$data/usr/lib/udev/rules.d/99-open-rig-programmer.rules" || err "udev rule drifted from repo copy"
diff -q "$here/copyright" \
  "$data/usr/share/doc/open-rig-programmer/copyright" || err "copyright file drifted from repo copy"
diff -q "$here/open-rig-programmer-512.png" \
  "$data/usr/share/icons/hicolor/512x512/apps/open-rig-programmer.png" || err "icon drifted from repo copy"
for s in postinstall postremove; do
  ctlname="$( [ "$s" = postinstall ] && echo postinst || echo postrm )"
  diff -q "$here/scripts/$s.sh" "$ctl/DEBIAN/$ctlname" || err "$ctlname drifted from repo $s.sh"
done
for bin in open-rig-programmer rigprog; do
  [ -x "$data/usr/bin/$bin" ] || err "$bin not executable"
done
# ELF checks (CI and Linux only — readelf is absent on the macOS dev
# box, and the stub-deb dry run uses shell scripts, not ELF binaries;
# set CHECK_DEB_SKIP_ELF=1 there).
#
# Every optional group below announces whether it ran or was skipped,
# and why. A silently-skipped group is indistinguishable from a passing
# one in a log, which matters most on the one-shot release run: that is
# exactly where a missing tool would quietly downgrade this script to a
# weaker check than the reader believes they are getting.
if [ "${CHECK_DEB_SKIP_ELF:-0}" = "1" ]; then
  echo "check-deb: SKIPPED ELF checks (CHECK_DEB_SKIP_ELF=1)"
elif ! command -v readelf >/dev/null 2>&1; then
  echo "check-deb: SKIPPED ELF checks (readelf not found)"
else
  case "$arch" in
    amd64) want_machine='X86-64' ;;
    arm64) want_machine='AArch64' ;;
    # A sentinel no readelf line can match: an unrecognised arch must
    # fail loudly, never silently skip the machine assertion.
    *) err "unrecognised arch '$arch'"; want_machine='NO-SUCH-MACHINE' ;;
  esac
  for bin in open-rig-programmer rigprog; do
    readelf -h "$data/usr/bin/$bin" | grep -q "Machine:.*${want_machine}" \
      || err "$bin is not a ${want_machine} ELF"
  done
  readelf -d "$data/usr/bin/open-rig-programmer" | grep -q 'NEEDED.*libwebkit2gtk-4\.1' \
    || err "GUI does not link libwebkit2gtk-4.1"
  readelf -d "$data/usr/bin/open-rig-programmer" | grep -q 'NEEDED.*libgtk-3' \
    || err "GUI does not link libgtk-3"
  if readelf -d "$data/usr/bin/rigprog" 2>/dev/null | grep -q 'NEEDED'; then
    err "rigprog has dynamic dependencies (expected static CGO_ENABLED=0 build)"
  fi
  echo "check-deb: ELF checks ran ($arch)"
fi
if command -v file >/dev/null 2>&1; then
  file "$data/usr/share/icons/hicolor/512x512/apps/open-rig-programmer.png" \
    | grep -q 'PNG image data, 512 x 512' || err "icon is not a 512x512 PNG"
  echo "check-deb: icon dimension check ran"
else
  echo "check-deb: SKIPPED icon dimension check (file not found)"
fi
if command -v desktop-file-validate >/dev/null 2>&1; then
  desktop-file-validate "$data/usr/share/applications/open-rig-programmer.desktop" || err "desktop-file-validate"
  echo "check-deb: desktop-file-validate ran"
else
  echo "check-deb: SKIPPED desktop-file-validate (desktop-file-utils not installed)"
fi

[ "$fail" -eq 0 ] && echo "check-deb: all assertions passed"
exit "$fail"

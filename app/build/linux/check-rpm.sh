#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Asserts a built open-rig-programmer .rpm is exactly what the
# packaging design promises: identity, dependencies, contents, and
# maintainer scripts. Metadata check only — no install, since the
# release runner (Ubuntu) has no Fedora rpm transaction stack to
# install into; that leg is why the Downloads table and README say
# the rpm is built but untested on a real Fedora install. Runnable
# anywhere `rpm` exists (Linux; macOS via `brew install rpm`). Usage:
#   check-rpm.sh <rpm> <version-without-v> <arch: amd64|arm64>
set -u

rpm_file="${1:?usage: check-rpm.sh <rpm> <version> <arch>}"
version="${2:?usage: check-rpm.sh <rpm> <version> <arch>}"
arch="${3:?usage: check-rpm.sh <rpm> <version> <arch>}"
# Absolute path: the payload extraction below runs inside a cd into a temp dir.
case "$rpm_file" in /*) ;; *) rpm_file="$PWD/$rpm_file" ;; esac
here="$(cd "$(dirname "$0")" && pwd)"
fail=0
err() { echo "FAIL: $*" >&2; fail=1; }

command -v rpm >/dev/null 2>&1 || { echo "rpm not found" >&2; exit 2; }

# Same split the packaging step applies before calling nfpm: rpm
# forbids "-" in the Version header, so a prerelease tag
# (1.8.1-rc1) becomes Version 1.8.1 / Release rc1; a published
# version (1.8.1, no hyphen) is Version 1.8.1 / Release 1 (nfpm's
# default when Release is left empty).
case "$version" in
  *-*) want_version="${version%%-*}"; want_release="${version#*-}" ;;
  *) want_version="$version"; want_release="1" ;;
esac
case "$arch" in
  amd64) want_arch='x86_64' ;;
  arm64) want_arch='aarch64' ;;
  *) err "unrecognised arch '$arch'"; want_arch='no-such-arch' ;;
esac

qf() { rpm -qp --queryformat "$1" "$rpm_file" 2>/dev/null; }

[ "$(qf '%{NAME}')" = "open-rig-programmer" ] || err "Name"
[ "$(qf '%{VERSION}')" = "$want_version" ] || err "Version (got '$(qf '%{VERSION}')', want '$want_version')"
[ "$(qf '%{RELEASE}')" = "$want_release" ] || err "Release (got '$(qf '%{RELEASE}')', want '$want_release')"
[ "$(qf '%{ARCH}')" = "$want_arch" ] || err "Architecture (got '$(qf '%{ARCH}')', want '$want_arch')"

# Exact set comparison, same reasoning as check-deb.sh: the shipped
# file list must be exactly these seven paths, no more, no fewer.
actual="$(rpm -qpl "$rpm_file" | LC_ALL=C sort)"
expected="$(printf '%s\n' \
  /usr/bin/open-rig-programmer \
  /usr/bin/rigprog \
  /usr/share/applications/open-rig-programmer.desktop \
  /usr/share/icons/hicolor/512x512/apps/open-rig-programmer.png \
  /usr/share/icons/hicolor/scalable/apps/open-rig-programmer.svg \
  /usr/lib/udev/rules.d/99-open-rig-programmer.rules \
  /usr/share/doc/open-rig-programmer/copyright | LC_ALL=C sort)"
[ "$actual" = "$expected" ] || err "package contents differ from the expected set
(< expected, > actual):
$(diff <(printf '%s\n' "$expected") <(printf '%s\n' "$actual"))"

# Exact set comparison for Requires too: this nfpm/rpm combination
# does not synthesise rpmlib()/interpreter auto-requires the way a
# real rpmbuild dependency generator would (verified empirically with
# nfpm v2.47.0), so the declared pair is everything there is.
actual_req="$(rpm -qpR "$rpm_file" | LC_ALL=C sort)"
expected_req="$(printf '%s\n' gtk3 webkit2gtk4.1 | LC_ALL=C sort)"
[ "$actual_req" = "$expected_req" ] || err "Requires is not exactly the declared Fedora pair (got: $(rpm -qpR "$rpm_file" | tr '\n' ' '))"

scripts="$(rpm -qp --scripts "$rpm_file" 2>/dev/null)"
echo "$scripts" | grep -q 'postinstall scriptlet' || err "postinstall scriptlet missing"
echo "$scripts" | grep -q 'postuninstall scriptlet' || err "postuninstall scriptlet missing"
echo "$scripts" | grep -q 'command -v udevadm' || err "scriptlets lack udevadm guard"

data="$(mktemp -d)" || exit 2
trap 'rm -rf "$data"' EXIT
if command -v rpm2cpio >/dev/null 2>&1 && command -v cpio >/dev/null 2>&1; then
  (cd "$data" && rpm2cpio "$rpm_file" | cpio -idm --quiet --no-absolute-filenames) || err "payload extraction"
  diff -q "$here/open-rig-programmer.desktop" \
    "$data/usr/share/applications/open-rig-programmer.desktop" || err "desktop file drifted from repo copy"
  diff -q "$here/99-open-rig-programmer.rules" \
    "$data/usr/lib/udev/rules.d/99-open-rig-programmer.rules" || err "udev rule drifted from repo copy"
  diff -q "$here/copyright" \
    "$data/usr/share/doc/open-rig-programmer/copyright" || err "copyright file drifted from repo copy"
  diff -q "$here/open-rig-programmer-512.png" \
    "$data/usr/share/icons/hicolor/512x512/apps/open-rig-programmer.png" || err "icon drifted from repo copy"
  diff -q "$here/../appicon.svg" \
    "$data/usr/share/icons/hicolor/scalable/apps/open-rig-programmer.svg" || err "scalable icon drifted from build/appicon.svg"
  for bin in open-rig-programmer rigprog; do
    [ -x "$data/usr/bin/$bin" ] || err "$bin not executable"
  done

  if [ "${CHECK_RPM_SKIP_ELF:-0}" = "1" ]; then
    echo "check-rpm: SKIPPED ELF checks (CHECK_RPM_SKIP_ELF=1)"
  elif ! command -v readelf >/dev/null 2>&1; then
    echo "check-rpm: SKIPPED ELF checks (readelf not found)"
  else
    case "$arch" in
      amd64) want_machine='X86-64' ;;
      arm64) want_machine='AArch64' ;;
      *) want_machine='NO-SUCH-MACHINE' ;;
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
    echo "check-rpm: ELF checks ran ($arch)"
  fi
  if command -v file >/dev/null 2>&1; then
    file "$data/usr/share/icons/hicolor/512x512/apps/open-rig-programmer.png" \
      | grep -q 'PNG image data, 512 x 512' || err "icon is not a 512x512 PNG"
    echo "check-rpm: icon dimension check ran"
  else
    echo "check-rpm: SKIPPED icon dimension check (file not found)"
  fi
  if command -v desktop-file-validate >/dev/null 2>&1; then
    desktop-file-validate "$data/usr/share/applications/open-rig-programmer.desktop" || err "desktop-file-validate"
    echo "check-rpm: desktop-file-validate ran"
  else
    echo "check-rpm: SKIPPED desktop-file-validate (desktop-file-utils not installed)"
  fi
else
  echo "check-rpm: SKIPPED payload extraction and everything downstream of it (rpm2cpio/cpio not found)"
fi

[ "$fail" -eq 0 ] && echo "check-rpm: all assertions passed"
exit "$fail"

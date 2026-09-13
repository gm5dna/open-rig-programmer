#!/usr/bin/env bash
# SPDX-License-Identifier: GPL-3.0-or-later
#
# Asserts a built .flatpak bundle installs and starts. Used by the
# release workflow after flatpak build-bundle. Usage:
#   check-flatpak.sh <bundle> <app-id>
#
# The GUI binary parses no CLI flags (app/main.go calls wails.Run with
# no os.Args/flag handling — confirmed, not assumed), so there is no
# "--version" to check like check-deb.sh gets from rigprog. The
# smallest real smoke here is: does the installed app launch at all
# under a virtual display without immediately dying (missing library,
# broken desktop entry, wrong command). A `timeout` still counting
# down when it fires (exit 124) means the process was alive when
# killed, i.e. it started; a fast non-124 exit means it crashed on
# launch.
set -u

bundle="${1:?usage: check-flatpak.sh <bundle> <app-id>}"
app_id="${2:?usage: check-flatpak.sh <bundle> <app-id>}"
fail=0
err() { echo "FAIL: $*" >&2; fail=1; }

command -v flatpak >/dev/null 2>&1 || { echo "flatpak not found" >&2; exit 2; }

flatpak install --user --bundle --noninteractive -y "$bundle" \
  || err "flatpak install --bundle failed"

flatpak info --user "$app_id" >/dev/null 2>&1 || err "$app_id not installed after bundle install"

if command -v xvfb-run >/dev/null 2>&1; then
  timeout 10 xvfb-run -a flatpak run "$app_id" >flatpak-run.log 2>&1
  rc=$?
  if [ "$rc" -eq 124 ] || [ "$rc" -eq 0 ]; then
    echo "check-flatpak: launch smoke passed (exit $rc)"
  else
    err "launch smoke failed (exit $rc); log follows"
    cat flatpak-run.log >&2
  fi
else
  echo "check-flatpak: SKIPPED launch smoke (xvfb-run not found)"
fi

flatpak uninstall --user --noninteractive -y "$app_id" >/dev/null 2>&1 || true

[ "$fail" -eq 0 ] && echo "check-flatpak: all assertions passed"
exit "$fail"

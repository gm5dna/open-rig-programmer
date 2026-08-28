#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
# Reload udev so the packaged ModemManager-ignore rule takes effect
# without a replug where possible. Guarded: containers and chroots
# without udevadm must not fail the install.
set -u
if command -v udevadm >/dev/null 2>&1; then
  udevadm control --reload-rules || true
  udevadm trigger --subsystem-match=tty || true
fi
exit 0

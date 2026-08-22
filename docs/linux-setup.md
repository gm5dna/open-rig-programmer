<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Linux serial setup

This document covers the two things a Linux user needs to do before
`rigprog` (CLI or GUI) can open a real FT-710's serial port: join the
`dialout` group, and stop ModemManager from probing the radio's
USB-serial adapter. See `README.md`'s quick-start section for where
this fits into the overall install/connect flow, and
`docs/hardware-notes.md` for the hardware facts this project has
actually confirmed (macOS only, so far — see "Status" below).

## 1. Join the `dialout` group

On most Linux distributions, a serial device such as `/dev/ttyUSB0`
is owned by `root:dialout` with group read/write permissions. Add
your user to that group:

```sh
sudo usermod -aG dialout "$USER"
```

Group membership changes only take effect for new login sessions —
log out and back in (or reboot) before trying to connect. `newgrp
dialout` also works for the current shell session only, which is
useful for testing without a full logout.

Without this, opening the port fails with a permissions error (e.g.
`permission denied` on the device node), not a "port not found"
error — if `rigprog ports` lists the device but `rigprog probe` (or
the GUI's Connect) can't open it, this is the first thing to check:

```sh
ls -l /dev/ttyUSB*
groups
```

## 2. Exclude the radio from ModemManager

Many Linux desktop distributions run ModemManager, which watches for
newly-attached serial devices and sends AT commands to them to check
whether they are a modem it should manage. Sent to the FT-710's CAT
port, this is exactly the kind of unsolicited traffic that can
interfere with a CAT session — at minimum it causes brief contention
for the port; NetworkManager/ModemManager may also hold the device
open transiently.

The fix is a udev rule that tells ModemManager to ignore the FT-710's
USB-serial adapter. The FT-710's built-in USB interface presents a
Silicon Labs CP2105 dual UART bridge, USB vendor:product ID
`10C4:EA70` — the same adapter this project's own macOS hardware
session characterised (see `docs/hardware-notes.md`'s "Port mapping"
section: the CP2105 exposes two UART interfaces, only one of which is
the CAT-capable "Enhanced" port), and the same VID:PID `rigprog
ports` scores highest (`core/transport/discover.go`).

Create `/etc/udev/rules.d/99-ft710.rules`:

```
# Yaesu FT-710 (Silicon Labs CP2105 dual USB-UART bridge):
# tell ModemManager to leave this device alone.
SUBSYSTEM=="tty", ATTRS{idVendor}=="10c4", ATTRS{idProduct}=="ea70", ENV{ID_MM_DEVICE_IGNORE}="1"
```

Then reload udev rules (no reboot required; replugging the adapter
after this is the simplest way to be sure the rule has taken effect):

```sh
sudo udevadm control --reload-rules
sudo udevadm trigger
```

If your FT-710 enumerates under a different VID:PID (check with
`lsusb` while it's plugged in), adjust `idVendor`/`idProduct`
accordingly.

If you installed the `.deb`, this section is already done for you. The
package installs this exact rule at
`/usr/lib/udev/rules.d/99-open-rig-programmer.rules`, from
`app/build/linux/99-open-rig-programmer.rules` in this repository —
its `SUBSYSTEM=="tty"` line is byte for byte the one above. Creating
the rule by hand is therefore only needed for the static CLI tarball
or a build from source.

## Identifying the CAT (Enhanced UART) port

The CP2105 exposes two USB-serial interfaces from one physical
adapter — on macOS these showed up as separate device nodes, only one
of which (the "Enhanced UART") could actually open and exchange CAT
traffic; the other ("Standard UART") failed to configure at all (see
`docs/hardware-notes.md`'s "Port mapping" section). On Linux, both
interfaces will likely enumerate as separate `/dev/ttyUSB*` nodes
(e.g. `ttyUSB0` and `ttyUSB1`) sharing the same `10C4:EA70` USB
identity — but **which numbered node corresponds to the Enhanced/CAT
interface has not yet been confirmed on Linux**; that check is
pending (see "Status" below). Use `rigprog ports` to rank candidate
ports, and `rigprog probe --port <path>` to positively confirm which
node answers the FT-710's CAT identity query — that is the
authoritative check, not the device node's number.

## Status: pending real-hardware verification on Linux

Everything above follows from general Linux/udev/ModemManager
practice and the CP2105's known macOS behaviour; **none of it has yet
been verified against a real FT-710 on Linux**. The project's plan
requires a Linux real-radio session — confirming the `dialout`/udev
steps above actually work, and identifying which `/dev/ttyUSB*` node
is the CAT-capable Enhanced UART — before Linux release artefacts
ship publicly (see `docs/hardware-notes.md`'s "Explicitly not probed"
sections — "Linux everything"). Until that session has run and this
document has been updated with its findings, treat this page as a
best-effort starting point, and the project's release stays a DRAFT
(see `.github/workflows/release.yml`) because of it.

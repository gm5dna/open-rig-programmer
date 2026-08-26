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

## Status: the package is verified, the radio is still pending

**What has been verified.** On 23/08/2026 the Debian packages built by
`.github/workflows/release.yml` were installed on clean Ubuntu 24.04.4
LTS desktop virtual machines — one arm64, one amd64, both stock
images carrying no development tools — and exercised there:

- `sudo apt install ./<the .deb>` succeeds on both architectures and
  installs every packaged path. On the arm64 VM the removal cycle was
  exercised too: `apt remove` took the binaries, the udev rule, the
  desktop entry, the icon and the doc directory away again, and
  reinstalling restored all of them.
- The rule the package installs at
  `/usr/lib/udev/rules.d/99-open-rig-programmer.rules` is byte for byte
  the one this repository ships in
  `app/build/linux/99-open-rig-programmer.rules`, and `udevadm verify`
  accepts it (1 success, 0 failures) on both architectures.
- ModemManager is installed, enabled and running on the stock arm64
  Ubuntu 24.04 desktop image — so section 2 addresses something really
  there, not a hypothetical.
- The GUI starts from its installed desktop entry on both
  architectures and connects to the built-in Demo radio; on arm64 the
  whole Demo workflow was driven through, edit and Send included.
- `rigprog --version` prints the packaged version on both
  architectures, and `rigprog ports` exits cleanly: on the arm64 VM it
  listed that machine's own console UART (`/dev/ttyAMA0`, score 0, "no
  ranking signal matched"); on the amd64 VM it found no serial ports
  at all.

**What has not.** None of the above involved a radio. No FT-710 has
ever been connected to a Linux machine by this project, so **every
instruction on this page that depends on the device is still
unconfirmed**: whether the `dialout` step is sufficient in practice,
which `/dev/ttyUSB*` node is the CAT-capable Enhanced UART, and
whether the udev rule actually keeps ModemManager off the radio — the
VM sessions observed that ModemManager is running, not how it behaves
towards an FT-710. Everything in sections 1 and 2 still follows from
general Linux/udev/ModemManager practice and the CP2105's known macOS
behaviour, nothing more.

The project's plan therefore still requires a Linux real-radio session
(see `docs/hardware-notes.md`'s "Explicitly not probed" section —
"Linux port-mapping recheck"). Until it has run and this document has
been updated with its findings, treat the serial-port instructions
above as a best-effort starting point.

Every GitHub release this project makes starts as a DRAFT:
`.github/workflows/release.yml` never clears that flag, so whether any
artefact ships is a separate human decision. This pending Linux
hardware evidence is one of the things that decision has to weigh.

<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Windows setup

This document covers the things a Windows user needs to know before
`rigprog` (CLI or GUI) can open a real FT-710's serial port, and what
to expect from an unsigned installer. See `README.md`'s "Install" and
"First use" sections for where this fits into the overall
install/connect flow, and `docs/hardware-notes.md` for the hardware
facts this project has actually confirmed (macOS only, so far — see
"Status" below; nothing on this page has itself been tried on a
Windows machine yet).

## 1. Driver

The FT-710's built-in USB interface presents a Silicon Labs CP2105
dual USB-to-UART bridge, USB vendor:product ID `10C4:EA70` — the same
adapter this project's own macOS hardware session characterised (see
`docs/hardware-notes.md`'s "Port mapping" section). Windows needs the
Silicon Labs CP210x Virtual COM Port (VCP) driver bound to it before
either port shows up as a `COMn` device.

On Windows 10/11 x64, Windows Update normally supplies this driver
automatically the first time the adapter is plugged in — **ASSUMED,
register entry W2**; no x64 Windows machine has been available to this
project to confirm it, and W2 says explicitly that it cannot be lifted
this milestone. If Windows Update does not find it, or you are offline,
Silicon Labs publish the driver directly.

On Windows 11 on ARM64 (the platform this milestone's verification
session targets), whether Windows binds a working CP210x driver at all
is itself unconfirmed — **ASSUMED, register entry W1** — and Silicon
Labs' own driver page says nothing about ARM64 support one way or the
other. See "Status" below for what the planned verification session is
expected to settle.

## 2. Two COM ports

The CP2105 is a *dual* UART bridge: it presents **two** separate serial
interfaces from one physical adapter. Device Manager, under "Ports (COM
& LPT)", will show two devices, each with its own `COMn` number. On
macOS, only one of the two ("Enhanced") is CAT-capable at all; the
other ("Standard") fails to configure (see `docs/hardware-notes.md`'s
"Port mapping" section). On Windows, both are expected to enumerate as
ordinary COM ports, with exactly one answering the FT-710's `ID;`
identity query and the other either silent or failing to open —
**ASSUMED, register entry W3**; this has not been confirmed on
Windows, only inferred from the macOS behaviour.

`rigprog ports` lists candidate ports and scores each one, but on
Windows the scoring most likely ranks both ports **equally** rather
than picking the Enhanced port out — **ASSUMED, register entry W4**:
the driver's per-port friendly name is overwritten by a device-level
string shared by both UARTs before `rigprog`'s ranking logic ever sees
it, so the two ports tie. **No document in this project claims the
Enhanced port ranks first on Windows.** Use `rigprog probe --port
COMn` against each listed port in turn — that is the authoritative,
active check (it sends the FT-710's CAT identity query and only the
CAT-capable port answers), not the port's ranking or its number:

```sh
rigprog ports
rigprog probe --port COM3
rigprog probe --port COM4
```

## 3. SmartScreen and Defender

The installer and the GUI are not code-signed this milestone —
running the downloaded installer for the first time is expected to
show Windows SmartScreen's "Windows protected your PC" dialogue. Click
**More info**, then **Run anyway** to proceed; this is the same
unknown-publisher warning every unsigned Windows program shows, not
something specific to this project. Whether
that dialogue actually appears, and in what form, is recorded as
**ASSUMED, register entry W9** until the verification session observes
it.

Windows Defender may also scan the download; it is expected not to
quarantine it (**ASSUMED, register entry W10**) since the binaries are
built by a public GitHub Actions pipeline from this project's own
source, but that has likewise not yet been observed on real Windows.

## 4. Where things live

The installer (machine-scoped, admin required) puts everything under
`C:\Program Files\Open Rig Programmer\`: `Open Rig Programmer.exe` (the
GUI), `rigprog.exe` (the CLI), `LICENSE`, and `uninstall.exe`, plus
Start-menu and desktop shortcuts. There is no PATH modification, so the
CLI's full path (`"C:\Program Files\Open Rig Programmer\rigprog.exe"`)
is what you type, or add to your own PATH by hand.

**The installer's architecture check is native-only.** An amd64
installer refuses to run on an ARM64 Windows machine, even though
ARM64 Windows can emulate x64 programs — install the **arm64**
installer on an ARM64 machine; installing the amd64 one there will not
work around it.

The GUI needs Microsoft's WebView2 runtime — Windows 11 is expected to
already ship it; if it is missing, the installer is configured to
download it, which needs an internet connection — **ASSUMED, register
entry W8**; not yet tried by this project.

Settings and read-back snapshots live under
`%AppData%\rigprog\settings.json` and `%AppData%\rigprog\snapshots\` —
the same `os.UserConfigDir()`-based path this project uses on every
platform, just resolving to Windows' per-user roaming profile here
instead of `~/Library/Application Support` or `$XDG_CONFIG_HOME`. The
program opens these files with Unix-style `0600`/`0700` permission
bits, but **that sets no Windows ACL**: files under `%AppData%\rigprog`
simply get whatever permissions the containing folder already grants,
inherited the ordinary Windows way — the bits are a no-op on this
platform, not a weaker guarantee than the wording might suggest.

**Uninstalling** removes exactly what the installer put down — the GUI
exe, the CLI exe, `LICENSE`, `uninstall.exe`, the Start-menu and
desktop shortcuts, and the install directory itself — and leaves
`%AppData%\rigprog` (your settings and snapshots) and the per-user
WebView2 profile folder untouched. A package never deletes user data;
see `app/build/windows/README.md` for the uninstall Section's exact
contents.

## 5. Status

Nothing on this page has been tried on a Windows machine by this
project yet; an ARM64 verification session is planned. Every driver,
COM-port, SmartScreen/Defender and WebView2 claim above is labelled
ASSUMED with its register entry (W1, W2, W3, W4, W8, W9, W10)
precisely because none of it has been observed on real Windows
hardware; W5, W6, W7, W11, W12 and W13 concern behaviour this page
does not describe directly (a field in `probe`/`read` output, TX
safety at open, read/write timing, USB passthrough, the amd64 GUI, and
the exact driver wording) but are lifted, or not, by the same session
or are explicitly not liftable this milestone. The full register and
what would lift each entry are kept in this project's internal design
record, not in this document; `docs/hardware-notes.md` and
`docs/building.md` carry the parts of it this project has published.

Until that session has run and this page has been updated with its
findings, treat everything above as a best-effort starting point drawn
from the CP2105's documented behaviour and this project's macOS
record, nothing more.

Every GitHub release this project makes starts as a DRAFT:
`.github/workflows/release.yml` never clears that flag, so whether any
artefact ships is a separate human decision. This pending Windows
hardware evidence is one of the things that decision has to weigh.

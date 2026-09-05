<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Windows setup

This document covers the things a Windows user needs to know before
`rigprog` (CLI or GUI) can open a real FT-710's serial port, and what
to expect from an unsigned installer. See `README.md`'s "Install" and
"First use" sections for where this fits into the overall
install/connect flow, and `docs/hardware-notes.md` for the hardware
facts this project has actually confirmed — macOS, and, since
05/09/2026, a Windows 11 ARM64 VM with a real FT-710 attached (see
"Status" below for exactly what that session did and did not settle).

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

On Windows 11 on ARM64, Windows has **no in-box CP210x driver**, and
Windows Update did not supply one either, in the roughly three minutes
this project waited for it before installing by hand — **register
entry W1, LIFTED (qualified)**: confirmed on a real Windows 11 ARM64 VM
with a real FT-710 attached, 05/09/2026 (`docs/hardware-notes.md`'s
"Windows (ARM64 VM) session — 05/09/2026", "Driver"). What works is
Silicon Labs' **"CP210x Universal Windows Driver" package, version
11.6.0.420** (`DriverVer 08/14/2026`): although that package's own
release notes list only x64/x86 support, its zip in fact ships an
`arm64\silabser.sys` driver and an INF section covering the CP2105's
two interfaces. The zip is **INF-only — it has no setup/installer
executable**. Either extract it and, in Device Manager, select the
"Enhanced Com Port" device (the one in the `CM_PROB_FAILED_INSTALL`
error state) → **Update driver** → **Browse my computer for drivers**
→ point it at the extracted folder; or, from an admin PowerShell, run
`pnputil /add-driver silabser.inf /install` — this is what the
verification session did, and it bound both COM ports in one step.
**Do not wait for Windows Update to find a driver on ARM64 — install
the Silicon Labs package directly.**

## 2. Two COM ports

The CP2105 is a *dual* UART bridge: it presents **two** separate serial
interfaces from one physical adapter. Device Manager, under "Ports (COM
& LPT)", will show two devices, each with its own `COMn` number. On
macOS, only one of the two ("Enhanced") is CAT-capable at all; the
other ("Standard") fails to configure (see `docs/hardware-notes.md`'s
"Port mapping" section). On Windows, both enumerate as ordinary COM
ports, and exactly one answers the FT-710's `ID;` identity query — the
other fails to open outright rather than staying silent — **confirmed,
register entry W3, LIFTED** (`docs/hardware-notes.md`'s "Windows (ARM64
VM) session — 05/09/2026", "Port mapping": COM3 answered, COM4 failed
with "Invalid serial port").

`rigprog ports` lists candidate ports and scores each one; on Windows
the scoring ranks both ports **equally**, tied at 50, rather than
picking the Enhanced port out — **confirmed, register entry W4,
LIFTED**: the driver's per-port friendly name is overwritten by a
device-level string shared by both UARTs before `rigprog`'s ranking
logic ever sees it, so the two ports tie with the identical description
"CP2105 Dual USB to UART Bridge Controller". The same tie was observed
on macOS the same session (`docs/hardware-notes.md`'s "fleet note" in
that section): **this scoring rule has now been checked on both
platforms and fires on neither.** **No document in this project claims
the Enhanced port ranks first on Windows, or on any platform.** Use
`rigprog probe --port COMn` against each listed port in turn — that is
the authoritative, active check (it sends the FT-710's CAT identity
query and only the CAT-capable port answers), not the port's ranking or
its number. `rigprog.exe` is not on `PATH` (§5 below), so run it from
the folder that holds it:

```powershell
.\rigprog.exe ports
.\rigprog.exe probe --port COM3
.\rigprog.exe probe --port COM4
```

## 3. SmartScreen and Defender

The installer and the GUI are not code-signed this milestone —
running the downloaded installer for the first time shows Windows
SmartScreen's "Windows protected your PC" dialogue. Click **More
info**, then **Run anyway** to proceed; this is the same
unknown-publisher warning every unsigned Windows program shows, not
something specific to this project. **Confirmed, register entry W9,
LIFTED**: observed exactly as described, on the Edge-downloaded
installer on the Windows 11 ARM64 VM, 05/09/2026
(`docs/hardware-notes.md`'s "Windows (ARM64 VM) session — 05/09/2026").

Windows Defender may also scan the download; it did not quarantine or
block any binary this project ran that session — **confirmed, register
entry W10, LIFTED**.

## 4. The CLI zip

The verification session above used the installer throughout, not the
CLI zip — the three points below are **untested by this project**.

A zip downloaded through a browser and extracted with Explorer marks
the files it contains with the mark-of-the-web, the same zone
information that triggers SmartScreen on the installer (§3 above); a
`rigprog.exe` extracted that way may show the same "Windows protected
your PC" dialogue on its first run, with the same **More info** → **Run
anyway** remedy. If it does not appear, or you would rather avoid it,
right-click the **zip** (before extracting) → **Properties** →
**Unblock**, then extract.

`rigprog.exe` is a console program: double-clicking it in Explorer
opens and immediately closes a window, because there is no interactive
session to hold it open. Run it from PowerShell or Windows Terminal
instead — the same way the CLI is used from `C:\Program Files\Open Rig
Programmer\` after an installer run.

If Windows Update does not supply the CP210x driver (§1 above), Silicon
Labs' driver downloads are at
<https://www.silabs.com/developer-tools/usb-to-uart-bridge-vcp-drivers> —
the "CP210x Universal Windows Driver" package.

## 5. Where things live

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
download it, which needs an internet connection. **Confirmed, register
entry W8, LIFTED**: the Windows 11 ARM64 VM already had the WebView2
Evergreen runtime (version 152.0.4191.62) before the installer ran, so
the bootstrapper's download step had nothing to do
(`docs/hardware-notes.md`'s "Windows (ARM64 VM) session — 05/09/2026",
"GUI and Demo"); the download path itself has still not been exercised.

**Version information shown on installed files differs by tool.**
Explorer's Properties → Details on `Open Rig Programmer.exe` correctly
shows File description, File version, Product name, Product version
and the licence line — confirmed on the Windows 11 ARM64 VM,
05/09/2026 (`docs/hardware-notes.md`'s "Windows (ARM64 VM) session —
05/09/2026", "The install"). PowerShell's `(Get-Item …).VersionInfo`,
by contrast, reads every one of those string fields as empty: that is
a .NET `FileVersionInfo` quirk with the language-neutral ("0000")
version-resource block Wails writes, not a defect in the installed
binary. Check Explorer's Properties dialogue, not a PowerShell
`VersionInfo` read, when confirming what shipped.

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
WebView2 profile folder untouched. **Confirmed on the Windows 11 ARM64
VM, 05/09/2026**: after uninstalling via Settings → Apps, the install
directory, both shortcuts and the Programs-and-Features entry were
gone, while `%AppData%\rigprog` (all three snapshot+journal pairs from
the session) and the per-user WebView2 profile were still present, and
no `rigprog` process remained running
(`docs/hardware-notes.md`'s "Windows (ARM64 VM) session — 05/09/2026",
"Uninstall"). A package never deletes user data; see
`app/build/windows/README.md` for the uninstall Section's exact
contents.

## 6. Status

**An ARM64 verification session has run** (05/09/2026): a Windows 11
Pro ARM64 virtual machine (UTM, on this project's own macOS host), a
real FT-710 passed through over USB, the rehearsal installer and CLI
downloaded, installed and driven end to end, and two writes made and
then restored — the full write-up is `docs/hardware-notes.md`'s
"Windows (ARM64 VM) session — 05/09/2026". Every driver, COM-port,
SmartScreen/Defender and WebView2 claim above that carries a register
entry from that list — **W1 (qualified — the vendor package installed
by hand), W3, W4, W8, W9, W10** — is now **LIFTED**, confirmed on that
session, not inferred from macOS. **W2** (whether an x64 machine's
Windows Update supplies the CP210x driver on its own) stays
**ASSUMED**: this session's VM was ARM64, not x64, so it cannot speak
to that claim. W6, W7, W11 and W13 concern behaviour this page does
not describe directly (TX safety at open, read/write timing, USB
passthrough, and the exact driver wording) and are likewise **LIFTED**
by the same session; **W5** is **observed (recorded only)** rather
than lifted — the USB serial field stayed empty in the `probe` output
on Windows, and no code change follows from that — see
`docs/hardware-notes.md` for each. **W12** (the amd64 GUI, launched by
a person) stays **ASSUMED**: the amd64 CLI has run on a Windows x64
host in CI (`rigprog.exe version` and `rigprog.exe ports`), but the
amd64 GUI has never been launched by anyone.

**Console output.** `rigprog.exe`'s prose uses em dashes; captured
under a legacy console code page (as PowerShell's redirected output
was on the ARM64 VM) they render as `ÔÇö` rather than "—" — a code-page
display quirk, not a difference in what the CLI printed. Run `chcp
65001` first, or use Windows Terminal, to see them correctly.

**What remains untried on Windows**: the amd64 GUI (the amd64 builds
are produced by the same pipeline on a Windows x64 host and the amd64
CLI runs there too, but its GUI has never been launched by anyone),
Windows 10, and a physical Windows machine of either architecture.
Everything above that is not qualified as ASSUMED is
confirmed for the platform the 05/09/2026 session actually used —
Windows 11 on ARM64, virtualised — and should not be read as a claim
about amd64 or about a physical machine.

Every GitHub release this project makes starts as a DRAFT:
`.github/workflows/release.yml` never clears that flag, so whether any
artefact ships is a separate human decision. The remaining Windows
hardware gap — amd64, and a physical machine of either architecture —
is one of the things that decision has to weigh.

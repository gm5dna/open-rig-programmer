# open-rig-programmer

Read your Yaesu or Icom radio's memory channels into a file, edit them
in a grid or a spreadsheet, and send them back. Free and open source,
for macOS and Linux.

It comes as a desktop app and a command-line tool, `rigprog`. Both use
the radio's ordinary USB cable — no programming cable, no SD-card
shuffling.

## Which radios

| Radio | Read | Write |
| --- | --- | --- |
| **FT-710** | ✅ | ✅ verified on a real radio |
| **FTdx10**, **FTdx101D**, **FTdx101MP** | ✅ | ⚠️ opt-in |
| **FT-891** | ✅ | ⚠️ opt-in |
| **IC-7610**, **IC-7300**, **IC-7300MK2**, **IC-705**, **IC-9700**, **IC-905**, **IC-7851**, **IC-7850**, **IC-7760**, **IC-7100** | ✅ | ⚠️ opt-in |
| **IC-R8600** (a receiver) | ✅ | ⚠️ opt-in |

Only the FT-710 has ever been connected to this program. The other
fifteen were built from the manufacturers' published protocol manuals
and tested against simulators. Reading them is safe: the program sends
only documented read commands. Writing to them is switched off until
you switch it on, one radio at a time.

If you own one of these fifteen radios and would like to help test it,
please open an issue.

### Switching on writes for an unverified radio

Every write command for these fifteen radios is taken from the maker's
own manual, but none has been proven on a real radio. So the program
refuses to send any of them until you say so:

```sh
rigprog settings unverified-writes                # list every radio and its setting
rigprog settings unverified-writes IC-7610 on     # allow writes to the IC-7610
rigprog settings unverified-writes IC-7610 off    # withdraw that permission
```

The app asks the same question the first time you connect to one of
these radios, and its *Unverified writes…* button opens the list at
any time. The app and the command line share one settings file, so a
decision made in either holds for both.

Permission changes what the program may send, not how carefully it
sends it. Every send still reads the radio fresh, saves a snapshot,
shows you the exact changes for approval, and reads each channel back
after writing it — stopping at the first mismatch. Deleting a channel
and changing menu settings stay refused whatever you permit.

## Install

Download from the [Releases page](../../releases). `SHA256SUMS` on that
page lets you check any download.

| Platform | File |
| --- | --- |
| macOS app | `open-rig-programmer-<version>-darwin-universal.app.zip` |
| macOS command line | `rigprog-<version>-darwin-universal.tar.gz` |
| Debian, Ubuntu, Mint (app + command line) | `open-rig-programmer_<version>_amd64.deb` or `_arm64.deb` (version without the leading `v`) |
| Other Linux (command line only) | `rigprog-<version>-linux-amd64.tar.gz` or `-arm64.tar.gz` |
| Windows (installer, app + command line) | `open-rig-programmer-<version>-windows-amd64-installer.exe` or `-arm64-installer.exe` |
| Windows (command line only, zip) | `rigprog-<version>-windows-amd64.zip` or `-arm64.zip` |

**macOS**: the app is not notarised, so the first time, right-click it
and choose *Open*. After that it opens normally.

**Debian, Ubuntu, Mint**: `sudo apt install ./<downloaded file>.deb`
installs the app, the command line, a menu entry and the serial-port
rule. The package is built and tested on Ubuntu 24.04; it should also
install on Ubuntu 22.04 and Debian 12 or later, but nobody has tried
yet.

**Other Linux**: the command-line tarball is a single static binary —
unpack and run. To build the app yourself, see
[docs/building.md](docs/building.md).

**Windows**: the installer is not code-signed, so the first time,
Windows SmartScreen shows *Windows protected your PC* — click *More
info*, then *Run anyway*. Pick the installer that matches your
machine: it refuses to run on the wrong architecture, and the amd64
installer will not run on an ARM64 machine even though ARM64 Windows
can emulate x64 programs. The installer puts the app, the command line
and a Start-menu entry under `Program Files`; the zip is a single
`rigprog.exe` — unpack and run, no install. The GUI needs Microsoft's
WebView2 runtime: Windows 11 is expected to already ship it (confirmed
present on the ARM64 VM below, so its bootstrapper had nothing to
install); if it is missing, the installer is configured to download
it, which needs an internet connection — the download path itself has
not been tried. Tested on a Windows 11 ARM64 virtual machine with a
real FT-710 attached (05/09/2026): installer, driver, GUI, CLI and two
writes-then-restore all worked as described. The amd64 builds are
produced by the same pipeline on a Windows x64 host (GitHub's
`windows-2025` runner) and the amd64 CLI has run there too — but **the
amd64 GUI has never been launched by anyone**, and no physical Windows
machine of either architecture has been tried. See
[docs/windows-setup.md](docs/windows-setup.md) for the full picture.

## First use

**FT-710 firmware**: memory programming needs firmware V01-10 or
later. Check the version on the radio's front panel; the program
cannot ask the radio.

**Serial port, macOS**: nothing to install. The radio's USB adapter
shows two ports; only the right one opens on macOS, so you cannot pick
the wrong one.

**Serial port, Linux**: add yourself to the `dialout` group
(`sudo usermod -aG dialout "$USER"`, then log out and in). The `.deb`
installs the rule that keeps ModemManager off the radio; if you use
the tarball, create it by hand as described in
[docs/linux-setup.md](docs/linux-setup.md). The Linux port setup has
been tested in virtual machines, not yet with a radio attached.

**Serial port, Windows**: on ARM64, Windows Update did not supply a
driver in this project's testing — install Silicon Labs' CP210x
Universal Windows Driver by hand (see
[docs/windows-setup.md](docs/windows-setup.md)); x64 is expected to get
one from Windows Update automatically, but that has not been tried.
The radio's USB adapter shows two COM ports in Device Manager, and
unlike macOS, Windows enumerates both as ordinary COM ports, tied and
indistinguishable by name — but only one of the two actually opens; the
other failing to open outright is expected, not a fault. Probe each
one (`.\rigprog.exe probe --port COM3`,
`.\rigprog.exe probe --port COM4`, and so on) to find the one that
answers. `rigprog.exe` is not on `PATH`, so run it from the folder
that holds it — the zip's extraction folder, or `C:\Program
Files\Open Rig Programmer\` after an installer run (see
[docs/windows-setup.md](docs/windows-setup.md)).

**In the app**: choose the radio and the port, connect (or press
*Demo* to try it with a simulated radio), read, edit in the grid, then
*Send*. Before anything is transmitted, the app shows every change it
is about to make, and anything it refuses to make, with the reason.

**On the command line**:

```sh
rigprog ports                                  # list likely serial ports
rigprog probe --port /dev/cu.SLAB_USBtoUART    # confirm which radio answers
.\rigprog.exe probe --port COM3                # Windows: the port name from Device Manager
rigprog read  --port ... --out radio.json      # save the radio's memories to a file
rigprog diff  --port ... edited.json           # preview what would change
rigprog write --port ... edited.json           # send the changes
```

Add `--model IC-7300` (or any supported name) for a radio other than
the FT-710, and `--fake` to use the simulator instead of a port.
`export` and `import` convert to and from CSV — including CHIRP's CSV —
without a radio. `rigprog version` reports the build; quote it in bug
reports.

## What it will not do

Some things are refused on purpose, with the reason shown:

- **Delete a channel.** The Yaesu radios have no delete command. The
  Icom radios do, but this program deliberately does not implement it.
- **Change menu settings.** The program can read and display a Yaesu
  radio's menu settings, never write them. The reasoning is in
  [docs/menu-write-decision.md](docs/menu-write-decision.md). Icom
  menus are not read at all.
- **FT-710**: tone, scan-skip and clarifier cannot be set over CAT, so
  edits to them are refused rather than silently dropped.
- **FT-891**: tone and scan-skip cannot be set over CAT here either —
  this radio's memory record has no tone-number byte and no scan-skip
  flag at all — and a transmit-clarifier flag arriving in a file
  written for another radio is refused rather than sent, that position
  being printed as fixed. A CHIRP file's `CW`, `CWR` and `RTTY` rows
  are not imported: they resolve to the sideband-specific names
  `CW-U`, `CW-L` and `RTTY-U`, which this radio's own mode list does
  not print — it prints `CW`, `CW-R`, `RTTY-LSB` and `RTTY-USB`
  instead — so the row is blocked rather than guessed at. Two more
  things are honest guesses. Its speed: the manual prints the four
  rates its CAT RATE menu row offers and marks none of them the
  factory setting, so the program opens at 38400 and, if that is wrong,
  the only remedy is menu 0506 on the radio — there is no speed setting
  in the program. And its socket: the radio's USB connection is a
  dual-UART bridge, so it appears as two serial ports, and the manual
  never says which one carries CAT; if the first is silent, try the
  other. The manual also contradicts itself about whether a memory
  channel may be read at all, so a read refused for a channel that is
  plainly in use is the manual's ambiguity showing, not a fault in the
  program.
- **Icom radios**: each talks only to its factory CI-V address; a few
  channel states (a channel in a Select scan group on most models, a
  split channel on the IC-7300s and the IC-7100, DATA modes on the
  IC-7610, the IC-7851/IC-7850 and the IC-7760, D-STAR call signs on the
  IC-7100, the digital-squelch bytes of a D-STAR, P25, NXDN, DCR or dPMR
  channel on the IC-R8600) cannot be written back; the IC-705,
  IC-905 and IC-R8600 only
  list the memories their start-up scan finds; and the IC-9700's
  repeater-offset scale is an unresolved question in the manual — a
  wrong reading would put every offset out by ten times, and the program
  cannot detect it.
- **IC-7851 and IC-7850**: these two share one manual, one CI-V address
  and one memory format, and the program cannot tell them apart. The
  model it reports is the one you picked from the list, never one it
  detected. It also offers all six of the radio's USB speeds even if you
  have wired the older remote-jack path, which stops at 19200.
- **IC-7760**: this radio is two boxes, and only one connection is
  supported — the USB socket on the back of the control head, which
  appears on your computer as two serial ports. Which of the two answers
  is a setting on the radio and the manual prints no default, so if the
  first port is silent, try the other. The remote socket on the RF deck
  is not supported. Its speed is a guess as well: the manual gives no
  CI-V speed anywhere, so the program opens at 19200 and a wrong guess
  simply times out.
- **IC-R8600**: this one is a RECEIVER, not a transceiver, and the grid
  says so: there are no transmit-frequency or transmit-tone columns at
  all, because the radio has no transmitter and its memory record has no
  such bytes. Four things about it are guesses this program is honest
  about. Its speed: the CI-V guide prints no factory default, mentions no
  automatic setting, and never even lists the speeds the menu offers, so
  the program opens at 19200 and a wrong guess simply times out. Its
  capacity: the guide never states how many memories the receiver will
  hold, so the program has no total to show and cannot warn you before
  the receiver is full — what it does when full is unknown. Its connection: the
  receiver has four possible control terminals — a remote jack, a front
  and a rear USB port, and a network connection — and the program talks
  over USB, so if one port is silent, check which terminal the receiver
  has been told to use. And its digital modes: tone squelch is read and
  written on FM channels only, while a D-STAR, P25, NXDN, DCR or dPMR
  channel cannot be written back unless its digital-squelch bytes match
  what the program assumes, and switching a channel INTO one of those
  modes is refused outright — set the digital squelch at the receiver.
  It also cannot create a channel in an empty slot at all: the record's
  Select-group setting has no honest default to write, so the program
  refuses the write rather than inventing one — the same refusal the
  IC-7300s make, for the same reason. A channel the operator has marked
  as skipped is refused rather than rewritten as unskipped.
- **IC-7100**: the program lists this radio's 495 ordinary memories —
  banks A to E, 99 channels each — and NOTHING ELSE. The six programmed
  scan edges and four call channels are real channels on the radio, but
  the manual never says what bank number addresses them, so the program
  does not read them rather than guess an address and read the wrong
  thing. Their absence from the list is not evidence the radio has none.
  This is also the only Icom model here that the program opens with two
  stop bits rather than one, because its manual states no serial format
  for the CI-V link at all.

  The full list of Icom limitations, with the evidence for each, is in
  [docs/icom-models.md](docs/icom-models.md).

## For developers

[docs/building.md](docs/building.md) covers building from source and
the layout of the repository. Protocol facts come from Yaesu's CAT
Operation Reference Manuals (FT-710 2306-C, FTdx10 2308-F, FTdx101D/MP
2308-L, FT-891 1909-C), Icom's CI-V Reference Guides (IC-7610 rev 4, IC-7300 Full
Manual §19 rev 12b, IC-7300MK2 rev 0, IC-705 rev 6, IC-9700 rev 4,
IC-905 rev 2, IC-7760 rev 2, IC-R8600 rev 3a) and, where no separate
CI-V guide is published, the radio's own manual (IC-7850/IC-7851 rev 3,
section 18; IC-7100 A7085-2EX-5, section 20).

## Licence

GPL-3.0-or-later; see [LICENSE](LICENSE). This program comes
**without any warranty**. If you connect it to your transceiver, you
do so entirely at your own risk; the authors accept no liability for
damage to your radio, its firmware or its memory contents.

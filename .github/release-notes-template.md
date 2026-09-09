<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<!--
  Body of every GitHub release, written by release.yml's "Write release
  notes" step: a sed pass replaces the two version placeholders (the
  tag, and the tag without its leading "v" for the .deb file names)
  before this becomes the release text, so never write either token in
  this comment. Before tagging: paste the new version's CHANGELOG.md
  entry into "What changed in this version", and keep the Downloads
  table and the first-launch sections in step with README.md's
  "Install" and "First use". Links into the repository are pinned to
  the tag, so the release page keeps reading correctly after the docs
  move. The history of this file's edits is a private record
  (.superpowers/sdd/release-notes-sync-log.md, not in the repository).
-->

Open Rig Programmer __VERSION__: a free memory-channel programmer for
Yaesu, Icom and Kenwood radios. Read the memories into a file, edit them in a
grid or a spreadsheet, send them back over the radio's ordinary USB
cable. Desktop app and `rigprog` command line for macOS, Windows and
Linux.

Which radios are supported, and how far each has been tested, is in
[docs/radio-notes.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/radio-notes.md).
Only the FT-710 has ever been connected to this program; every other
radio's writes stay switched off until you switch them on for that
radio, as the
[README](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/README.md)
describes under *Switching on writes for an unverified radio*.

## What changed in this version

- **Kenwood TS-890S and TS-990S: read, opt-in write, menu-settings read.**
  The 100 memory channels and the menu inventory — 158 settings on the
  TS-890S, 194 on the TS-990S — over Kenwood's own PC control commands.
  Tone and scan skip are read and written, as on the TS-590 pair, and
  one frame carries a whole channel, which removes that pair's largest
  cost: a channel read off one of these radios comes back with its
  transmit frequency already known, so reading the memories, editing a
  name and sending them straight back works. Every published mode can be
  written — 16 on the TS-890S, 26 on the TS-990S, the data modes spelt
  into the mode names themselves.
- **What these two radios refuse is published rather than discovered.**
  A write to a channel the radio does not already hold is refused: the
  program reads the target first and will not create a channel the
  manual never says the command can create. Registering these radios
  does not mean the program can fill a blank one — they can be
  re-programmed, not programmed from empty. A channel whose secondary
  side on the radio is not what the program's own frame would write is
  refused, naming the parameter and both values. A 1750 Hz receive tone
  is refused. On the TS-990S, a channel with dual reception switched on
  is refused outright, and so is a section-defined channel. Channels are
  never deleted, although both manuals print a deletion command. Slots
  100–119 are offered on neither radio, no band edges are published, and
  the 9600 port speed is assumed.
- **A CHIRP file's ordinary rows import on both new radios** on the same
  terms as the TS-590 pair's, with one further cost: a single frame
  carries the transmit frequency, the tone mode, the transmit tone and
  the receive tone together, and the write path requires all four to be
  known. A CHIRP row states none of them, and nothing supplies a value
  the file did not carry, so an imported CHIRP channel is refused at the
  write until you fill all four in. A `TSQL` row is refused on the tone
  column besides. The program's own CSV import and export are unaffected.
- **Also in this version:** a CHIRP file's ordinary rows now import on
  the Kenwood TS-590S and TS-590SG and on the six Icom models whose
  memory bank carries no duplex field (a blank `Duplex` column is
  simplex; `off` is still refused); and a CHIRP `Name` is sanitised
  against each radio's own published tag charset, so a `;` in a name is
  kept on the eleven Icom rows that allow it.
- **No Kenwood radio has ever answered a frame from this program.**
  Writing stays switched off on both new rows until you switch it on for
  that radio. Every claim above rests on the two service manuals and on
  simulated radios transcribed from them.
- **Byte identity.** This milestone's own command-line capture — 479
  artefacts on each side, a narrower instrument than v1.4.1's 608 — is
  identical by hash for every previously supported radio, with one
  exception: the three surfaces that print the supported-model list
  gain exactly two rows, TS-890S and TS-990S. Reverting the single
  registration commit alone returns every artefact to the v1.4.1 base.
  Every golden vector, transcription CSV and evidence checksum is
  untouched.

## Downloads

| Platform | What it is | File |
| --- | --- | --- |
| macOS (Intel + Apple Silicon, universal) | App (.app, zipped) | `open-rig-programmer-__VERSION__-darwin-universal.app.zip` |
| macOS (Intel + Apple Silicon, universal) | Command line | `rigprog-__VERSION__-darwin-universal.tar.gz` |
| Windows amd64 | App + command line (installer) | `open-rig-programmer-__VERSION__-windows-amd64-installer.exe` |
| Windows arm64 | App + command line (installer) | `open-rig-programmer-__VERSION__-windows-arm64-installer.exe` |
| Windows amd64 | Command line only (zip) | `rigprog-__VERSION__-windows-amd64.zip` |
| Windows arm64 | Command line only (zip) | `rigprog-__VERSION__-windows-arm64.zip` |
| Linux amd64 (Debian, Ubuntu, Mint) | App + command line (.deb) | `open-rig-programmer___VERSION_NO_V___amd64.deb` |
| Linux arm64 (Debian, Ubuntu, Mint) | App + command line (.deb) | `open-rig-programmer___VERSION_NO_V___arm64.deb` |
| Linux amd64 | Command line | `rigprog-__VERSION__-linux-amd64.tar.gz` |
| Linux arm64 | Command line | `rigprog-__VERSION__-linux-arm64.tar.gz` |

The Windows installers are native-only: the amd64 installer refuses to
run on an ARM64 PC even though ARM64 Windows can emulate x64 programs.
The Debian package installs the app, the command line, a desktop entry
and the ModemManager udev rule; `sudo apt install ./<file>` resolves
its GTK and WebKit dependencies (built and tested on Ubuntu 24.04;
Ubuntu 22.04, Debian 12 and the Mint releases built from them carry the
same packages but have not been tried). On other distributions take
the command-line tarball, a single static binary.

`SHA256SUMS` covers every file above. Verify with:

```sh
sha256sum -c SHA256SUMS --ignore-missing     # Linux
shasum -a 256 -c SHA256SUMS --ignore-missing # macOS
certutil -hashfile <file> SHA256             # Windows, one file at a time
```

## First launch

- **macOS**: the app is only ad-hoc signed, so Gatekeeper refuses it
  the first time. Right-click the app, choose *Open*, and confirm. Once.
- **Windows**: the installer is unsigned. Edge may flag the download,
  and SmartScreen shows *Windows protected your PC* the first time:
  click *More info*, then *Run anyway*. The app needs Microsoft's
  WebView2 runtime, which Windows 11 already ships; if it is missing
  the installer is set to download it, a path this project has not
  tried. On ARM64 install Silicon Labs' CP210x
  driver by hand before connecting; the radio shows as two COM ports
  and only one answers. See
  [docs/windows-setup.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/windows-setup.md).
- **Linux**: join the `dialout` group and log out and in; keep
  ModemManager off the radio (the .deb installs the rule; the tarball
  needs it by hand). See
  [docs/linux-setup.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/linux-setup.md).

**FT-710 firmware**: memory programming needs V01-10 or later. There is
no CAT query for the version; check the radio's front panel.

**Which version you have**: `rigprog version` prints it (on Windows,
`.\rigprog.exe version` from the folder that holds it; the installer
adds nothing to PATH). The app shows it at the right-hand end of the
status bar. Quote it in any bug report; a build reporting
`dev (unreleased build)` did not come from this page.

## What the hardware evidence covers

Every "works on the radio" claim comes from recorded sessions against
**one UK FT-710**, on macOS and, since 05/09/2026, on a Windows 11
ARM64 virtual machine;
[docs/hardware-notes.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/hardware-notes.md)
is the record. Not yet exercised against a real radio: **Linux** (the
packages have been installed and launched on Ubuntu 24.04 virtual
machines, never with a radio attached, so which serial node is the CAT
port and whether the udev rule keeps ModemManager off it are
unconfirmed); **Windows on amd64**, or any physical Windows PC; and
**every radio other than the FT-710**, all of which come from the
makers' manuals and simulators built from them, which is why their
writes are opt-in. Each of those drivers carries a register of every
assumption it makes and the capture from a real radio that would
settle it. If you own one of these radios, a read of a single channel
is the most valuable thing you could send:
[open an issue](https://github.com/gm5dna/open-rig-programmer/issues/new/choose).

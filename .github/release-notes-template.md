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

- **The Yaesu FT-991A is now supported**, for reading and for opt-in
  writes, on the same terms as every other manual-derived model: its 99
  memories, its 9 PMS pairs and its menu settings, CSV and CHIRP.
- **The settings list shows 152 items where this radio's menu chart
  prints 153 rows.** Row 087, RADIO ID, is left out because the chart
  gives it neither a width nor a parameter — ten printed hyphens and
  nothing else — so the program cannot size an answer to it and will not
  send a question it cannot read. One `EX087;` read on a real FT-991A
  would settle it.
- **The PMS pairs are listed as the channel numbers 100 to 117**, which
  is what this radio's own CAT record uses, while its front panel and
  its manual print the same eighteen slots as `P-1L` to `P-9U`. The
  numbers are the wire's and the letters are the panel's; they name the
  same slots in the same order.
- **This radio's memory channels can carry a DCS state**, the first here
  that can: its tone byte has five values — CTCSS off, CTCSS encode and
  decode, CTCSS encode, DCS encode and decode, DCS encode — and the
  program reads and writes all five. What it cannot reach is *which*
  tone or *which* DCS code, neither of which is a per-channel field on
  this radio, so a CHIRP file's `DTCS` and `Cross` rows are still
  refused — and the reason now says that the state can be written and
  the code cannot, instead of denying the state.
- **Three CHIRP limitations on this radio.** `CW`, `CWR` and `RTTY` rows
  are not imported: they resolve to names this radio's own mode list
  does not print (it prints `CW`, `CW-R`, `RTTY-LSB` and `RTTY-USB`), so
  the row is blocked rather than guessed at. C4FM is one of its fourteen
  modes and CHIRP has no name for it at all, so no CHIRP file can
  describe a C4FM channel. And scan skip has no place in this radio's
  memory record, so a `Skip` cell asking for one is dropped and the loss
  reported, row by row.
- **Its CAT speed is a guess** (38400; menu 031 CAT RATE on the radio is
  the only remedy — menu 029 sets the rear-panel RS-232C jack's rate, a
  different port), and its USB socket is a dual-UART bridge presenting two serial
  ports with no statement of which carries CAT: if one is silent, try
  the other.
- **The Kenwood TS-590S and TS-590SG are now supported**, for reading and
  for opt-in writes, on the same terms as every other manual-derived
  model — and they are the first radios here whose tone and scan-skip
  columns can actually be read and written. Both read the 100 memory
  channels, the 10 programmable scan ranges and the menu settings (88
  items on the S, 100 on the SG). **A memory channel is written back only
  once you supply its transmit frequency**, and a channel read off the
  radio does not carry one: the manual never says what these radios
  answer for the transmit side of a simplex channel, so the program
  leaves it unavailable rather than guessing and refuses the write
  (register entry A9). Reading the memories, editing them and sending
  them straight back is therefore refused on every memory channel until
  you fill that column in — typing the receive frequency there writes the
  channel as simplex, which is what the one frame this program sends can
  express. A genuine split is refused even then, and a scan range is not
  affected. Only FM channels are written, and a 1750 Hz receive tone is
  refused where a 1750 Hz transmit tone is written. On the
  TS-590S alone the filter column cannot be set at all, and channel
  writes are refused on a radio reporting firmware 2.00 or later — or a
  firmware version the program cannot read: the manual guarantees the
  relevant byte is unused only on the 1.xx firmware, and a version that
  cannot be compared cannot be shown to be a 1.xx one. `rigprog probe`
  prints the radio's own firmware answer verbatim so you can see what
  that decision was taken on. Their speed is a guess (9600; there is no
  speed setting, and a wrong speed looks like a dead port).
  **CHIRP import is not available for these two radios in this release:
  every row is blocked.** A CHIRP file's blank `Duplex` column means
  simplex, and these radios declare no shift vocabulary at all — their
  memory record carries no duplex selector — so every ordinary row is
  refused on that column and the import writes nothing; `CW`, `CWR` and
  `RTTY` rows are refused a second time on the mode besides. The
  program's own CSV import and export are unaffected.
- **The Kenwood TS-480 is NOT selectable**, although its driver ships in
  this build. Nothing in its 2003 manual says what a memory channel that
  has never been written answers when it is read; the TS-590SG's manual
  says an all-zero record, and if a TS-480 rejects that read instead, a
  brand-new one cannot be read by this program at all. It stays
  unavailable until somebody observes what a real TS-480 answers — at
  least three unwritten channels, each confirmed at the front panel,
  across at least two sessions, with the exact bytes kept and an observer
  named; `internal/wiring/testdata/README.md` says how.

No existing radio's behaviour changed. The stored comparison output of
every previously registered radio is byte for byte identical apart from
the lists of supported models, which gain the FT-991A, the TS-590S and
the TS-590SG. The TS-480 appears in no list, because it is not
registered.

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

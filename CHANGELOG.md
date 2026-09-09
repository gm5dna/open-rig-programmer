# Changelog

All notable changes to Open Rig Programmer, newest first. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); the
version numbers follow the rule that a MINOR release adds a capability
(a radio, a platform, a feature), a PATCH release adds none, and a
MAJOR release would break compatibility. Dates are the GitHub
publication dates, which for two releases are a day or two after the
tag. The full release notes for each version are on the
[Releases page](https://github.com/gm5dna/open-rig-programmer/releases).

## [Unreleased]

### Added
- **Kenwood TS-890S and TS-990S**: read, opt-in write, menu-settings
  read. The 100 memory channels and the menu inventory — 158 settings on
  the TS-890S, 194 on the TS-990S — over Kenwood's own PC control
  commands. Tone and scan skip are read AND written, as on the TS-590
  pair, and **one frame carries a whole channel**, which is what removes
  that pair's largest cost: a channel read off one of these radios comes
  back with its transmit frequency already known, so reading the
  memories, editing a name and sending them straight back works. Every
  published mode can be written too — 16 on the TS-890S, 26 on the
  TS-990S, the data modes spelt into the mode names themselves, so a
  channel's data disposition survives a CSV or CHIRP round trip in its
  mode column.
- **What these two radios refuse, and it is published rather than
  discovered.** A write to a channel the radio does not already hold is
  refused: one frame carries the whole channel, so the program reads the
  target first and will not create a channel the manual never says the
  command can create. **Registering these radios does not mean the
  program can fill a blank one** — they can be re-programmed, not
  programmed from empty. A channel whose secondary side on the radio is
  not what the program's own frame would write is refused, naming the
  parameter and both values, rather than overwritten. A 1750 Hz receive
  tone is refused on both rows. On the TS-990S alone, a channel with
  dual reception switched on is refused outright — that flag describes a
  second receiver the program's channel model cannot hold — and so is a
  channel the radio types as section defined. Channels are never
  deleted, although both manuals print a deletion command. Slots 100-119
  exist on both radios and are offered on neither: neither manual prints
  what selects a scan range's start frequency and what its end, so a
  bank of them would be a reading rather than a transcription. No band
  edges are published, because neither manual prints a frequency range.
  The port speed of 9600 is assumed, as it is for the TS-590 pair, and
  there is still no way to open at another.
- **A CHIRP file's ordinary rows import on both new radios** on the same
  terms as the TS-590 pair's — a blank `Duplex` column is simplex, `off`
  is refused, and `CW`, `CWR` and `RTTY` are refused on the mode — with
  one further cost, which is four values rather than one. A single frame
  carries the transmit frequency, the tone mode, the transmit tone and
  the receive tone together, and these radios' write path requires all
  four to be known. A CHIRP row states no transmit frequency, and its
  blank `Tone` column states only that tone is off, leaving both tone
  values unsaid; a `TSQL` row is refused on the tone column besides,
  because neither radio's memory chart prints a transmit-and-receive tone
  mode. Nothing supplies a value the file did not carry, so an imported
  CHIRP channel is refused at the write until you fill all four in. The
  program's own CSV import and export are unaffected.
- No Kenwood radio has ever answered a frame from this program. Writing
  stays switched off on both rows until you switch it on for that radio.

### Changed
- **A CHIRP file's ordinary rows now import on the Kenwood TS-590S and
  TS-590SG.** A CHIRP file's blank `Duplex` column is its ordinary
  simplex row, and these radios declare no shift vocabulary at all —
  their 50-byte memory record carries no duplex selector — so until now
  every ordinary row was refused with a blocking entry and the import
  wrote nothing. A blank column asks for nothing these radios cannot do,
  so it is no longer treated as a loss: the row imports as simplex and
  nothing is reported. A `Duplex` column reading `off` is still refused,
  because that asserts "no duplex configured" as distinct from simplex
  and the record has nowhere to carry the distinction, and `CW`, `CWR`
  and `RTTY` rows are still refused on the mode. The same change applies
  to the six Icom models whose memory bank carries no duplex field
  either — the IC-7300, IC-7300MK2, IC-7610, IC-7760, IC-7850 and
  IC-7851 — none of which published a shift vocabulary for a blank
  column to land in. No other radio's blank-`Duplex` outcome moves.
- **A CHIRP file's `Name` is now sanitised against each radio's own
  published tag charset, not one literal rule for every radio.** A radio
  that publishes its own charset (nine Icom driver packages, covering
  the eleven registered IC-705, IC-7100, IC-7300, IC-7300MK2, IC-7610,
  IC-7760, IC-7850, IC-7851, IC-905, IC-9700 and IC-R8600 rows) is now
  judged by that charset rather than the printable-ASCII-excluding-`;`
  default; every one of those charsets contains `;`, so a `;` in a
  `Name` is now kept on all eleven rows instead of being replaced with a
  space. A radio with no published charset — every Yaesu and Kenwood
  row — is unaffected: the default rule, and the loss message's wording,
  are unchanged.

## [1.4.1] - 2026-09-07

### Changed
- **A simplification sweep, and no new capability.** Nine lanes removed
  about 10,600 net lines across the tree without changing what any
  radio is sent or told: the frozen command-line capture (608 artefacts
  across every supported model) is byte-for-byte identical to v1.4.0,
  every golden vector, transcription CSV and evidence checksum is
  untouched, and every fence test still stands.
- **The desktop app's dialogs are now the platform's own.** Confirmations
  and the send flow use the browser's native `<dialog>` element, so
  Escape, focus containment and focus return come from the platform
  rather than from hand-written code. This is the one change a user can
  see.
- **Under the hood.** The five Yaesu drivers share one write, settings
  and probe body, each radio contributing only its own differences. The
  IC-7300 and IC-7300MK2 drivers are one package driven by a per-model
  table, as the FTdx101D and FTdx101MP already were. The simulated
  radios share one protocol-free pipe chassis and parse their own
  transcription CSVs at start-up instead of carrying generated tables,
  while still importing nothing from the codec they test. Hand-rolled
  helpers gave way to the standard library throughout.

### Not included
- **Folding the IC-7610, IC-7760 and IC-7851 drivers into one package.**
  Their provenance pins are per-model registers by construction; a fold
  would disable them rather than refactor them, so it waits for a
  milestone that first decides what replaces the pin.

## [1.4.0] - 2026-09-06

### Added
- **Yaesu FT-991A**: read, opt-in write, menu-settings read, CSV and
  CHIRP, on the same terms as every other manual-derived radio. Its 99
  memories and 9 PMS pairs, and 152 of the 153 menu rows its chart
  prints — row 087, RADIO ID, is left out because the chart gives it
  neither a width nor a parameter, so the program cannot size an answer
  to it. The PMS pairs are listed as the channel numbers 100 to 117,
  which is what the radio's CAT record uses, where the radio's own panel
  and manual print `P-1L` to `P-9U`. Its memory record carries a
  five-state tone byte, including two DCS states, which the program
  reads and writes; the tone number and the DCS code are not per-channel
  fields on this radio and are not touched. In CHIRP files, `DTCS` and
  `Cross` rows are refused (the state can be written, the code cannot),
  `CW`, `CWR` and `RTTY` rows are not imported (this radio's legend
  prints `CW`, `CW-R`, `RTTY-LSB` and `RTTY-USB`), and C4FM has no CHIRP
  name at all. Its CAT speed is a guess (38400; menu 031 CAT RATE on the
  radio is the only remedy), and its USB socket presents two serial
  ports with no statement of which carries CAT.
- **Kenwood TS-590S and TS-590SG**: read, opt-in write, menu-settings
  read and CSV, on the same terms as every other manual-derived radio.
  **CHIRP import is not available for these two radios: every row is
  blocked** — a CHIRP file's blank `Duplex` column means simplex, and
  these radios declare no shift vocabulary at all, their memory record
  carrying no duplex selector, so every ordinary row is refused on that
  column and the import writes nothing (`CW`, `CWR` and `RTTY` rows are
  refused on the mode besides). The program's own CSV import and export
  are unaffected.
  They are the first radios here whose tone and scan-skip columns
  can be read and written. A memory channel is written back only once
  you supply its transmit frequency, and a channel read off the radio
  does not carry one: the manual never says what these radios answer for
  the transmit side of a simplex channel, so the program leaves it
  unavailable rather than guessing and refuses the write, naming register
  entry A9. The ordinary round trip — read the memories, edit, send them
  back — is therefore refused on every memory channel until you fill that
  column in; a genuine split is refused even then, and a scan range is not
  affected. Only FM channels are written, and a 1750 Hz receive tone is
  refused where a 1750 Hz transmit tone is written. On the TS-590S alone
  the filter column cannot be set and channel writes are refused on
  firmware 2.00 or later — which the manual stops guaranteeing at exactly
  that point — or on a firmware version the program cannot read, a
  version that cannot be compared not being one that can be shown to be
  1.xx. `rigprog probe` prints the radio's own firmware answer
  verbatim.
- Per-radio Kenwood detail in `docs/radio-notes.md`, with the evidence in
  a new `docs/kenwood-models.md`.

### Changed
- Two completeness checks that were silently passing now walk the model
  registry: the simulated-profile confinement guard and the CHIRP
  fixtures. A radio registered without its row in either is now a test
  failure rather than a silence.

### Not included
- **Kenwood TS-480**: the driver is written and shipped but the radio is
  not selectable. Nothing in its 2003 manual says what a memory channel
  that has never been written answers when it is read; the TS-590SG's
  manual says an all-zero record, and if a TS-480 rejects that read
  instead, a brand-new one cannot be read by this program at all. It
  stays unavailable until somebody observes what a real TS-480 answers —
  at least three unwritten channels, each confirmed at the front panel,
  across at least two sessions, with the exact bytes kept and an observer
  named. `internal/wiring/testdata/README.md` says how.

## [1.3.0] - 2026-09-05

### Added
- **Windows**: an installer (app + command line) and a command-line zip,
  each for amd64 and ARM64. Tested on a Windows 11 ARM64 virtual machine
  with a real FT-710; the amd64 app has not yet been launched by anyone.
- **Yaesu FT-891**: read, opt-in write, menu-settings read, CSV and
  CHIRP, on the same terms as every other manual-derived radio. Its menu
  addresses are four digits, and files accept either width.
- **An icon of its own**, drawn once as an SVG and rendered for all
  three platforms.
- In the grid, an editor for every Icom tier column (duplex, offset,
  tone mode, filter and the rest), chosen by the field's kind; the
  connection bar says when the radio is a receiver.
- `CHANGELOG.md`, issue templates, and a documentation restructure: the
  README is written for radio owners, per-radio limits live in
  `docs/radio-notes.md`, developer material in `docs/developing.md`.

### Changed
- The serial port is opened with RTS and DTR requested low on every
  platform, so a radio wired for RTS/DTR keying is never keyed by the
  act of connecting.
- CSV import tolerates a UTF-8 byte-order mark and CRLF line endings,
  as saved by Windows spreadsheets.
- On the Yaesu radios, every channel field passes through one shared
  state check before the write gate: a known value is judged, a value
  under an unknown, unavailable or absent state is refused, and an
  absent field with no value is admitted. The clarifier bound is taken
  from each radio's dialect.

### Fixed
- An unanswered Icom field can no longer slip past the send gate by
  being saved and reloaded, or by a round trip through CSV: the file
  keeps the distinction between "nothing said" and "not available".
- IC-7610, IC-7760 and IC-7851: a frequency the radio's record cannot
  hold is refused when it is read, with the value and the limit, rather
  than later without a reason.
- IC-905 and IC-705: the write refusal and the probe note no longer
  over-claim how much of the memory the start-up walk covers.

## [1.2.2] - 2026-08-31

### Fixed
- IC-R8600: a write can no longer overwrite a channel nothing read. The
  pre-write read refuses a slot the session never listed, and says
  which walk ran.
- IC-7100: an absent transmit frequency or repeater offset is reported
  as unavailable rather than as 0 Hz.
- IC-9700: a record whose length belongs to a different Icom radio is
  reported as the wrong radio.
- The IC-R8600's scan-skip refusal (a skipped channel is refused rather
  than rewritten as unskipped) is now documented.

## [1.2.1] - 2026-08-30

### Added
- Five more Icom models: IC-7851 and IC-7850, IC-7760, IC-7100, and the
  IC-R8600 receiver, the first receive-only model, with its per-channel
  receiver settings carried in codeplug schema 5.

### Fixed
- FT-710: a read no longer aborts on a PMS pair whose kind byte the
  radio has set to 0 (field report, 30/08/2026).

## [1.2.0] - 2026-08-30

### Added
- Icom support: IC-7610, IC-7300, IC-7300MK2, IC-705, IC-9700 and
  IC-905 over CI-V, read and opt-in write, with CSV and CHIRP. Icom
  menus are not read.

## [1.1.0] - 2026-08-27

### Added
- Linux app as a Debian package (amd64 and arm64) with a desktop entry
  and the ModemManager udev rule; the command-line tarball remains for
  other distributions.
- Per-radio consent for unverified writes: writes to the FTdx10,
  FTdx101D and FTdx101MP, previously disabled in the code, are now
  switched on one radio at a time from the command line or the app.
  Both share one settings file.

## [1.0.0] - 2026-08-09

### Added
- First release. Yaesu FT-710 read, write and menu-settings read,
  verified on a real radio; FTdx10, FTdx101D and FTdx101MP read-only
  from their manuals. Desktop app and `rigprog` command line for macOS
  and Linux, a spreadsheet-style grid, CSV and CHIRP import and export,
  and the safe-send ladder: read before write, snapshot, reviewed
  diff, per-channel read-back.

[Unreleased]: https://github.com/gm5dna/open-rig-programmer/compare/v1.4.0...HEAD
[1.4.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.2...v1.3.0
[1.2.2]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/gm5dna/open-rig-programmer/releases/tag/v1.0.0

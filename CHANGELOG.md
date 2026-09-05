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

[Unreleased]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.2...HEAD
[1.2.2]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/gm5dna/open-rig-programmer/releases/tag/v1.0.0

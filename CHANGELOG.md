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
- **FT-710 menu settings gain a write path**, hardware-characterised
  address by address: `rigprog write --settings FILE` and the app's
  editable settings cells write only a menu address whose Set
  behaviour Stuart has confirmed on a real FT-710, then read it back
  byte-for-byte to check. 58 of the FT-710's 296 menu addresses are
  permanently read-only — the CAT link settings, PTT and keying over
  RTS, DTR or DAKY (including PC KEYING), tuner and antenna routing,
  TX power and safety, MOD SOURCE, and the text fields — and 4 more
  are held read-only pending further investigation; the rest become
  writable as each one's hardware characterisation lands. There is no
  consent step for menu writes, unlike an opt-in radio's channel
  writes: the manual's own settings chart has already been shown
  wrong, so only testing on hardware can be trusted. Every other
  radio's menu settings stay read-only.

### Fixed
- FT-710, FTdx10 and FTdx101: a channel write no longer accepts a
  frequency outside the radio's own 30 kHz-75 MHz storable range;
  previously only the wire codec's 9-digit field width was checked.
- IC-7100, IC-7300, IC-7300MK2, IC-905, IC-9700 and IC-R8600: a channel
  read now refuses a frequency the radio's own record cannot express,
  matching the equivalent check these models' writes already made.
- Importing a version-1 CSV (or a version-2 CSV with no receiver-fields
  columns) whose model has tier fields (duplex, offset, tone mode, DTCS
  and the rest) no longer settles the fields that file's header has no
  column for at all to "this radio has no such field"; they are left
  unanswered, so validation catches them as missing rather than silently
  treating the radio as lacking a field it has. A field whose column IS
  present, including an explicit "n/a" cell, is left exactly as the file
  states.

## [1.10.0] - 2026-09-18

### Added
- **FTX-1** joins the supported Yaesu models, paper-only and opt-in for
  writes via the unverified-write consent gate — no FTX-1 has ever
  answered a frame from this project. Reads and writes memory and PMS
  channels on a widened version of the FT-710's own MR/MW record:
  5-digit addresses (`00001`-`00999`), 50 PMS pairs as a hyphenated
  two-digit token (`P-01L`-`P-50U`), a 5 MHz band (`50001`-`50020`)
  and a named `EMGCH` channel, both banks read-only even under
  consent. Tags carry no display byte, so this build shows no Tag
  Display column for them. Tone widens to six states, adding `PR
  FREQ` and `REV TONE` to the fleet's vocabulary; no tone-frequency
  chart exists in the manual, so only the six-state selector is read
  and written. `EX`, `GT` and `VM` are not implemented; `MC` goes
  unused — reads go via `MR`/`MT` only, since `MC` carries a
  per-port byte this project's CAT codec cannot build or parse. This
  build cannot tell an FTX-1 "Field" body from an "Optima" body: both
  share CAT identity `0840` and nothing in the manual gives CAT a way
  to ask, so the body is never inferred. The manual requires main
  firmware V1.08 or later for CAT to work at all; this build cannot
  check it — the wire carries no version byte — so a pre-V1.08 radio
  simply never answers. Further ASSUMED values (channel range past
  99, which of three printed 5 MHz ranges is real, MW's P7 enum,
  TagFill, the clarifier's step size, and more) are listed with their
  own probe list in `docs/radio-notes.md`.
- **TS-2000, TS-2000X and B2000** gain a Satellite Memory bank
  (`SA`/`SI`, 10 channels): a name plus three per-channel flags —
  band swap, trace and trace-reverse. The record carries no
  frequency; satellite operation reads that live off `FA`/`FB`
  instead. The radio answers `SA` only for the currently selected
  channel, so a whole-radio read skips this bank (marked
  current-channel-only) and a single-channel read of the selected slot
  works; other slots return an honest mismatch error. Unverified,
  consent-gated, on the same terms as the ordinary memory bank.

### Changed
- Internal: `core/cat` gained four axes for FTX-1's own record shapes
  — 5-digit addresses, the dash-token PMS form, a tag form with no
  display byte, and an `MC`-unsupported gate. That gate closed a
  pre-existing hole: a fixed `MC;` read frame was previously admitted
  regardless of policy. `dialecttest` was taught all four axes, so
  every existing dialect stays exercised (byte-identical for every
  registered three-digit dialect).
- Internal: Yaesu's separate `ToneSemantics`/`CTCSSStates` vocabulary
  is gone. Every Yaesu driver now publishes the same `spec.ToneMode`
  list Icom and Kenwood already used, plus two new DCS members;
  byte-identical for every existing dialect. The unification caught
  two latent bugs: a Yaesu-detection check (in the codeplug validator
  and the app's UI spec) was keying off `CTCSSStates`, and the shared
  write-gate was feeding the now-shared `ToneModes` list into a tier
  check meant for Icom only. Both fixed.
- Internal: three new `spec.Field` constants for the satellite flags
  ship fleet-wide — every radio other than TS-2000/2000X/B2000
  answers Unavailable for them. The IC-R8600 CSV export, which lists
  every field explicitly, gains three empty columns.
- **FTdx3000 AM-N storability** closed on paper: the CAT manual's own
  MW/MR memory-record mode legend prints only A/B/C, never D, so
  AM-N has no byte position in a stored channel at all and no real
  radio could store it regardless of what this driver writes.
- Internal: supported models 45 → 46.

### Not included
- **Settings write.** Parked for this milestone; still on the
  roadmap.

## [1.9.0] - 2026-09-15

### Added
- **FT-890 and FT-900** join the supported Yaesu models on this
  project's first binary-CAT family: 5-byte opcode frames, no ASCII,
  no semicolon terminator, distinct from the NEWCAT/MR-MW protocol
  every other Yaesu row uses. Neither radio carries a CAT-ID byte on
  the wire; two fixed probe frames establish identity instead. Every
  write runs the family's VFO→M choreography (select VFO-A, set
  frequency, mode, clarifier, shift and tone, then Store) — never a
  partial update of one field. NoTag: no channel-name route over CAT.
  Paper-only, opt-in for writes via the unverified-write consent gate
  — no FT-890 or FT-900 has ever answered a frame from this project.
  The 19-byte memory record's second 9-byte half cannot be preserved:
  Store overwrites the whole record from live VFO state, so this
  build has no way to read those bytes aside and write them back
  unchanged.
- **FT-1000MP** (and Mark-V) joins on the same binary-CAT protocol and
  VFO→M write model, narrowed to its own 16-byte, tone-less record.
  Also paper-only and opt-in. Store's channel-argument byte position
  and its channel-numbering base are both assumed, not confirmed; a
  mismatched write is always reported as failed, never hidden or
  retried. Owner probes that would settle both are listed in
  `docs/radio-notes.md`.
- The FT-920 — a related but different Yaesu radio from the same era
  — stays unregistered: its manual documents the Memory Store verb
  but not how the memory record encodes frequency, so no codec can be
  written from paper. Deferred to the roadmap, capture-gated on an
  owner's Status Update dump of a real radio.

### Changed
- Internal: new `core/bincat` package for the binary-CAT codec shared
  by FT-890, FT-900 and FT-1000MP. Supported models: 42 → 45.

## [1.8.2] - 2026-09-14

### Added
- **Flathub-ready metainfo**: `app/build/flatpak/io.github.gm5dna.open-rig-programmer.metainfo.xml`
  now carries screenshots, a release history and full AppStream metadata for a
  Flathub submission, with three screenshots of the real app committed
  alongside it.
- **Source-build Flathub manifest**: `flathub/io.github.gm5dna.open-rig-programmer.yml`
  builds the app from source (Go 1.25.0 and Node 22 SDK extensions, no
  vendored binaries) with generated `go-sources.json`/`node-sources.json`,
  plus a `.github/workflows/flathub.yml` CI job that builds and lints the
  manifest, appstream and repo on every push touching `flathub/`.
- No functional change to the app itself.

## [1.8.1] - 2026-09-14

### Added
- **Flatpak aarch64**: the Flatpak bundle introduced in 1.8.0 now
  builds on both amd64 and arm64 runners (native, no cross-compile),
  attaching `open-rig-programmer-<version>-x86_64.flatpak` and
  `-aarch64.flatpak` to every release.
- **RPM package** (`.rpm`, nfpm from the same source as the `.deb`)
  for Fedora and openSUSE, x86_64 and aarch64, attached to every
  release alongside the `.deb`; built and metadata-checked in CI, not
  yet installed on a real Fedora or openSUSE machine. rpm forbids a
  `-` in the Version header, so a prerelease tag (e.g. `1.8.1-rc1`)
  splits into rpm's Version/Release fields for the rpm build only —
  the `.deb` Version is unaffected.

## [1.8.0] - 2026-09-13

### Added
- **FTdx3000** joins the supported Yaesu models: a paper-only
  registration, opt-in for writes via the unverified-write consent
  gate, since no real radio has ever answered this program. NoTag: no
  channel-name field over CAT. CTCSS tone is read-only over CAT (the
  write command's own tone field is fixed to `"00"`); AM-N is assumed
  not storable via CAT and is excluded from the write-capable mode
  list.
- **FTdx1200** joins the supported Yaesu models as one radio
  identified by either of two CAT IDs (`0582` with the FFT-1 filter
  fitted, `0583` without): the same paper-only, opt-in-write, NoTag
  shape as the FTdx3000.
- **FT-450D** joins the supported Yaesu models, also paper-only and
  opt-in-write, in a deliberately safe shape: only memory channels
  001-500 are written; the Programmable Memory Scan channels (501-504)
  stay read-only until an owner probes a real radio; the 60 m and
  Alaska-emergency channels are not exposed at all. Six probes that
  would lift these limits are listed in `docs/radio-notes.md`.
- **Light/dark mode**: a System/Light/Dark picker in the settings
  panel; the app follows the OS setting by default.
- **Flatpak bundle** (`.flatpak`, app id
  `io.github.gm5dna.open-rig-programmer`, GNOME 47 runtime,
  `--device=all` for the radio's serial port) is now attached to each
  release; Flathub submission is not yet done.

### Changed
- Internal: a new `cat.MemoryP9Policy` value, `P9ToneIndexReadOnly`,
  covers a write dialect whose tone field cannot be set. Supported
  models: 39 → 42.

## [1.7.1] - 2026-09-13

### Changed
- **macOS release builds are now signed with a Developer ID and
  notarised.** Gatekeeper no longer needs a right-click.
- Internal: the CHIRP per-model fixture ledger now covers all eleven
  pre-v1.7.0 Icom models (`chirpFixtureExceptions` empty); no
  user-visible change.

## [1.7.0] - 2026-09-13

### Added
- **A radio whose memory write route has no channel-name field can now be
  registered.** The new NoTag capability marks a profile as having nothing
  for a channel name to occupy, rather than forcing every profile through
  the existing tag-field machinery. On such a radio the desktop grid hides
  the Tag column, a CHIRP import drops the file's `Name` column with a
  single warning per import instead of refusing the file outright, and the
  native CSV export writes an empty `Name` column so the file's shape
  still matches every other radio's export. A profile that claims NoTag
  but whose bank still lists a tag field is rejected at validation, so the
  two can never disagree.
- **IC-7800** joins the supported Icom models (v1.7.0 Icom wave) at CI-V
  address 6Ah: memory and scan-edge channels, opt-in unverified writes, a
  HIGH-proximity clone of the IC-7610's record shape at its own address.
  Manual-derived only: no real IC-7800 has ever answered a frame from this
  program, and writing to it stays switched off until you switch it on.
- **IC-7600** joins the supported Icom models (v1.7.0 Icom wave) at CI-V
  address 7Ah: another HIGH-proximity clone of the IC-7610's record shape,
  at its own address. Manual-derived only: no real IC-7600 has ever
  answered a frame from this program, and writing to it stays switched off
  until you switch it on.
- **IC-7410** joins the supported Icom models (v1.7.0 Icom wave) at CI-V
  address 80h: not a clone — a 40-byte record with a genuine TX-duplicate
  block, and a write that leaves the transmit frequency unset mirrors the
  receive frequency into it rather than refusing. Manual-derived only: no
  real IC-7410 has ever answered a frame from this program, and writing to
  it stays switched off until you switch it on.
- **IC-7700** joins the supported Icom models (v1.7.0 Icom wave) at CI-V
  address 74h: reads and writes the transmit (split) frequency, and shares
  the already-registered IC-7300's 39-byte record shape. Manual-derived
  only: no real IC-7700 has ever answered a frame from this program, and
  writing to it stays switched off until you switch it on.
- **IC-9100** joins the supported Icom models (v1.7.0 Icom wave) at CI-V
  address 7Ch: one band, richer than its siblings (duplex, offset, DTCS
  code and polarity), and opens at the port's own default framing rather
  than a driver-asserted stop-bit count. Manual-derived only: no real
  IC-9100 has ever answered a frame from this program, and writing to it
  stays switched off until you switch it on.
- **IC-7200** joins the supported Icom models (v1.7.0 Icom wave, the last
  of six) at CI-V address 76h: NoTag — no channel-name field over CI-V at
  all, so no Tag column is shown for it — and no tone or scan-skip field
  of any kind, on a 17-byte record. Manual-derived only: no real IC-7200
  has ever answered a frame from this program, and writing to it stays
  switched off until you switch it on.
- **TS-2000, TS-2000X and TS-B2000** join the supported tier as one
  package covering three model rows on a shared 50-byte memory record:
  tone mode, both transmit and receive tone numbers, scan skip and an
  eight-character channel name are all read and written — the only
  package in this release with a channel name. A channel is written back
  only after its own transmit frequency, DCS code, REVERSE state and
  memory group are first read from the radio, so this driver can write an
  existing channel but not create a new one. Manual-derived only: no radio
  of this family has ever answered a frame from this program, and writing
  to it stays switched off until you switch it on.
- **TS-570D, TS-570S and TS-570DG** join the supported tier as one
  package covering three model rows on a shared 28-byte memory record with
  no channel-name field at all. Tone mode, both tone numbers and scan skip
  are read and written. The TS-570DG row is UNVERIFIED-BY-INHERITANCE: the
  manual behind this package documents the D and S models only, and the
  DG's own values are inferred from its siblings' command set rather than
  read from a DG-specific page. Manual-derived only: no TS-570 of any row
  has ever answered a frame from this program, and writing to it stays
  switched off until you switch it on.
- **TS-870S** joins the supported tier with its own 22-byte memory
  record — a different shape from the TS-2000/TS-570 family, not a
  narrower cut of it — and no channel-name field. Tone mode and one shared
  transmit/receive tone index are read and written; the record has no
  separate receive-tone byte at all. Manual-derived only: no TS-870S has
  ever answered a frame from this program, and writing to it stays
  switched off until you switch it on.
- **FTdx5000** joins the supported tier: memory and programmable-memory-
  scan channels on a 27-byte record with no channel-name field. The CTCSS
  tone is a live tone-table index, read and written along with the
  clarifier, shift and CTCSS state. Manual-derived only: no FTdx5000 has
  ever answered a frame from this program, and writing to it stays
  switched off until you switch it on.
- **FT-2000 and FT-2000D** join the supported tier as one package on the
  same 27-byte record shape as the FTdx5000, again with no channel-name
  field; one manual documents both models, and this program tells them
  apart only by which one you chose when you connected. Manual-derived
  only: no radio of this family has ever answered a frame from this
  program, and writing to it stays switched off until you switch it on.
- **FTdx9000** joins the supported tier on the same 27-byte record shape,
  no channel-name field. Its own manual never actually prints the string
  "FT-9000" — the export-market name this radio is also sold under — so
  this program treats that name as descriptive text only, not a second
  model row or an alias you can type. Manual-derived only: no FTdx9000 has
  ever answered a frame from this program, and writing to it stays
  switched off until you switch it on.
- **FT-950** joins the supported tier on the same 27-byte record shape, no
  channel-name field; it has one extra regular memory channel over the
  rest of this family, numbered from 000 rather than 001. Manual-derived
  only: no FT-950 has ever answered a frame from this program, and writing
  to it stays switched off until you switch it on.
- None of the Control Command Lists for FT-2000, FTdx5000, FTdx9000 or
  FT-950 documents a memory-tag/combined write command at all — only a
  plain read and write pair — which is simply a fact about those radios,
  not a limitation this programme imposes. Nine of this release's twelve
  new Kenwood and Yaesu rows have no channel-name field over their own CAT
  protocol at all; only the three TS-2000 rows carry one.
- **Every write now runs the same field-safety check.** The check that
  refuses a field carrying a value its own radio never actually read —
  already run by every other driver — is now also run by all fifteen Icom
  drivers, closing a gap where such a value could previously be silently
  dropped from the frame rather than refused.

### Notes
- Nothing changes for any existing radio: byte identity is verified
  across every captured artefact (1,063 in total); the only movers are
  the bare-usage banner, `help`, an unrecognised `--model` and an
  unrecognised subcommand — all four just reflecting the new model
  count and list — plus the TS-2000 and TS-870S capture legs, which move
  from an unrecognised-model error under the pre-release base to a real
  reading now that both are registered.
- None of this release's eighteen new models (six Icom, twelve Kenwood and
  Yaesu) has ever been connected to a real radio; each is opt-in and
  refuses to write until you switch it on for that model (README,
  *Switching on writes for an unverified radio*). Thirty-nine models are
  now registered in total, up from twenty-one.

## [1.6.0] - 2026-09-12

### Added
- **A CHIRP row is now a complete statement of a channel's tone state.**
  Every registered radio that expresses tones as a mode plus independent
  transmit/receive indices (the TS-590 pair, TS-890S, TS-990S and the
  Icom tier) now carries CHIRP's `rToneFreq`/`cToneFreq`/`DtcsCode`/
  `DtcsPolarity` columns even on a row whose own `Tone` column does not
  select that field — so an ordinary CHIRP export's `88.5`/`023`/`NN`
  fill values populate the radio's own idle-tone state instead of being
  left unread. **The TS-890S and TS-990S are the headline case:** a
  blank-`Tone` CHIRP row previously imported with both tone indices
  Unknown (decision B, ruled B2, 09/09/2026) and could never pass the
  write's tone rung; measured on the fake, an imported channel now
  writes and verifies on a consented session (`rigprog settings
  unverified-writes TS-890S on`), same for the TS-990S. A value the
  radio does not admit in one of these newly-read columns is a
  non-blocking loss-report entry rather than a refusal.

### Changed
- Some CHIRP loss-report wording for the tone-family columns on the
  Icom tier changed to describe the new non-blocking "kept, but this
  channel's mode does not use it" and "no such field" cases (see Added,
  above); `DtcsCode`/`DtcsPolarity`'s existing "not really data" fill
  values are now shared with `rToneFreq`/`cToneFreq` under one
  `chirpToneFillValues` table.
- **The send dialogue no longer asks for a firmware version.** The
  FT-710's memory CAT arrived in firmware V01-10 and the radio has no
  version query, so every send used to open with a box to type the
  version read off the front panel. But the send flow always reads every
  memory over that same CAT first, so a radio that reaches the dialogue
  has already proved its firmware by answering. The box, the `rigprog
  write --firmware` flag and its interactive prompt are gone; the first
  write on a session needs nothing beyond the ordinary confirmation.
  Codeplug files that carry a `firmware_confirmed` value still load; the
  value is no longer written.

## [1.5.1] - 2026-09-09

### Fixed
- The desktop app no longer shows a pair of document scrollbars after the
  window is resized, and the *Unverified writes* panel no longer squashes
  its radio list to a single clipped row (both seen on macOS in v1.4.1).

### Changed
- **A CHIRP file's blank `Duplex` column now states a transmit
  disposition** on the six radios whose memory record grades a transmit
  frequency but has no duplex selector — the TS-590S, TS-590SG, TS-890S,
  TS-990S, IC-7300 and IC-7300MK2. A blank column is CHIRP's ordinary
  simplex row, and the import used to leave the transmit frequency
  *unknown*, which said the file had been silent when it had not; that
  unknown then travelled into the CSV export, into `rigprog diff` and
  into the loss report. Each radio's own record now supplies the value:
  zero on the TS-890S and TS-990S, whose books print that a simplex
  channel's split parameters all read zero, and the channel's own receive
  frequency on the other four, whose records have no split flag at all. A
  `Duplex` column reading `off` is unchanged and still refused. No
  channel becomes writable that was not writable before.
- **The published recovery for an imported CHIRP channel was overstated
  on five radios and is now measured.** The TS-890S and TS-990S ask for
  two values, the transmit tone and the receive tone, not four. The
  TS-590S asks for the data mode and both tone numbers, the TS-590SG for
  those plus the filter, and the IC-7300 and IC-7300MK2 for the filter,
  the data mode and a slot the radio already holds — and CHIRP has no
  column for a data mode or a filter, so on those four rows a CHIRP file
  alone can never complete a write. Said plainly now in the model list
  and in `docs/kenwood-models.md` for the TS-590S, TS-590SG, TS-890S and
  TS-990S, and in `docs/radio-notes.md` for the IC-7300 pair.
- `rigprog diff` and the write plan now name only the fields that actually
  changed on a modified channel, instead of restating unchanged ones as
  `X→X`; a channel modified only in a field the summary does not print
  says so.

## [1.5.0] - 2026-09-09

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
  one further cost, which is the two tone numbers. A single frame carries
  the tone mode, the transmit tone and the receive tone together, and
  these radios' write path requires all three to be known; a CHIRP row's
  blank `Tone` column states only that tone is off, leaving both numbers
  unsaid, and a `TSQL` row is refused on the tone column besides, because
  neither radio's memory chart prints a transmit-and-receive tone mode.
  Nothing supplies a value the file did not carry, so an imported CHIRP
  channel is refused at the write until you fill both numbers in. (The
  transmit frequency was a third cost when this version shipped; v1.5.1
  removed it, and this bullet describes today's behaviour rather than the
  history of the tag.) The program's own CSV import and export are
  unaffected.
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

[Unreleased]: https://github.com/gm5dna/open-rig-programmer/compare/v1.10.0...HEAD
[1.10.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.9.0...v1.10.0
[1.9.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.8.2...v1.9.0
[1.8.2]: https://github.com/gm5dna/open-rig-programmer/compare/v1.8.1...v1.8.2
[1.8.1]: https://github.com/gm5dna/open-rig-programmer/compare/v1.8.0...v1.8.1
[1.8.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.7.1...v1.8.0
[1.7.1]: https://github.com/gm5dna/open-rig-programmer/compare/v1.7.0...v1.7.1
[1.7.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.6.0...v1.7.0
[1.6.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.5.1...v1.6.0
[1.5.1]: https://github.com/gm5dna/open-rig-programmer/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.4.1...v1.5.0
[1.4.1]: https://github.com/gm5dna/open-rig-programmer/compare/v1.4.0...v1.4.1
[1.4.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.2...v1.3.0
[1.2.2]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/gm5dna/open-rig-programmer/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/gm5dna/open-rig-programmer/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/gm5dna/open-rig-programmer/releases/tag/v1.0.0

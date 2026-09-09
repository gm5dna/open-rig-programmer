<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Kenwood models — per-model limitations and evidence

The evidence behind the Kenwood entries in `docs/radio-notes.md`. Every
claim below cites the code that makes it true; the citations are for
reviewers and contributors.

## The rows built, and which are selectable

The **TS-590S**, the **TS-590SG**, the **TS-890S** and the **TS-990S**
are in the model list. The **TS-480** is not, although its driver is
written, tested and shipped in the binary — see *The TS-480 is built and
not registered* below, which is the longest entry on this page because
an absence needs more explaining than a presence.

These radios talk a third wire protocol: neither Yaesu's CAT nor Icom's
CI-V, but Kenwood's own semicolon-terminated PC control commands. The
envelope codec is `core/kw`; the two 590 layouts are `core/kw/ts590`, the
TS-480's is `core/kw/ts480`, and the 890S's and 990S's are both in
`core/kw/ma` — one package holding two layouts, two hand-written record
codecs and its own outbound gate. The drivers are `core/driver/ts590`
(both 590 rows, one package, a REQUIRED row argument),
`core/driver/ts480`, and `core/driver/ts890` and `core/driver/ts990`, one
package each.

**Why the second pair gets two driver packages where the first pair
shares one.** The 590 pair's two radios share a 50-byte memory record, so
one driver serves both with a row argument. The 890S's and 990S's records
are different shapes — 40 to 50 bytes with thirteen parameters against a
fixed 57 with eighteen — so there is no shared body for a row argument to
select between, and a crossed registration of that pair is a compile
error rather than a working driver for the wrong radio.

**Two registry rows over one driver, and no radio has ever answered
either.** `core/driver/ts590/caps.go` keeps a SEPARATE write-trial guard
per row, because evidence for one sibling is never evidence for the
other, and both are false. Every write is therefore behind the opt-in
consent route, and every refusal below still fires ahead of it: consent
widens what may be attempted, never how carefully it is attempted.

## Costs both 590 rows pay

- **A split channel is refused, not flattened.** These radios express a
  split as TWO frames over one channel number, `MW` P1 selecting the
  receive or the transmit side, and this program sends ONE frame per
  channel whose P1 comes from the SLOT's class. There is therefore no
  frame for the transmit half of a genuine split to go in. Writing the
  channel anyway would make it simplex — the manual says so in terms,
  "even if it was already a split channel" — read back as simplex, and
  report the write verified. So it is refused instead, naming the
  reason (`core/driver/ts590/write.go`, the `tx_frequency` rungs; the
  matrix's §2.4 and plan decision P12).

- **A memory channel is written back only once its transmit frequency is
  SUPPLIED.** The same arithmetic bites a plain round trip: what an `MR`
  with P1=1 answers on a SIMPLEX channel is printed nowhere in the book,
  so a fresh read never learns a channel's transmit side and reports it
  unavailable, and the write refuses rather than guess (register entry
  A9, `core/driver/ts590/write.go`). A channel is written only when its
  `tx_frequency` state is `Known`, which today means the user set it, so
  the ordinary round trip — read the memories, edit, send them back — is
  refused on EVERY memory channel until that column is filled in. A
  scan-range write is NOT refused for this reason: in that bank P1
  selects a range's start or end frequency rather than a transmit
  frequency, so there is no transmit disposition to require (matrix
  erratum M-E2, pinned per bank by `app/uispec_test.go`'s
  `TestBankTierFields_RegisteredTS590Pair_PerBank`). A9 lifts PER ROW,
  and only on an observation of what a real radio of that row answers.
  The cost is published to users in `internal/radiotext`'s two 590
  `GridLegendNote`s, in `docs/radio-notes.md` and in the release notes,
  and `TestRadiotext_TS590Pair_GridLegendCarriesItsPublishedCosts`
  pins that it stays there.

- **A 1750 Hz RECEIVE tone is refused; a 1750 Hz TRANSMIT tone is
  written.** `TN`'s printed chart runs 00-42 and its last entry is 1750
  Hz; the `CN` tone-squelch chart prints 00-41 and has no equivalent.
  `spec.Capabilities` carries ONE tone domain and one predicate for both
  directions, so the 43-entry list admits as a `tone_rx` a value the
  receive chart does not print. The resolution is to publish 43 and
  refuse a `Known` `tone_rx` of 1750 Hz in the write path (design
  decision 14, matrix erratum M-E1, `core/driver/ts590/write.go`'s
  `registerDecision14`), rather than to claim a narrower domain in the
  capability table. A 1750 Hz `tone_rx` cannot come off a radio in the
  first place: `core/kw` bounds P9 at `CN`'s own 41.

- **Only FM channels are written.** No `MW` P14 value is known to be
  meaningful outside FM, so a write of a channel in any other mode is
  refused, naming register entry A23. Reading is unaffected — every mode
  reads normally.

- **No channel can be deleted.** The only clearing route the **590 book**
  prints is a side effect of a SHORTENED memory-write frame (590:1579),
  and the length of that short frame is a reading of one sentence rather
  than a number printed anywhere (register entry A5, errata schedule
  E19). The codec admits a 50-byte `MW` and no other (`core/kw`'s
  outbound gate), so the short form cannot be sent even by accident.
  **The 480 book prints no memory-clear route at all** — its `MW` section
  ends without that sentence (480:949), and its only printed "clear" is
  `RC`, which clears the RIT offset (480:1205) — so the TS-480 is a radio
  whose own book gives no clearing route rather than one whose printed
  route this program declines to send.

- **No band edges are published.** `MinFreqHz` and `MaxFreqHz` are both
  ZERO on all three rows (`core/driver/ts590/caps.go`,
  `core/driver/ts480/caps.go`), and that is CANNOT ESTABLISH rather than
  a default: neither PC control command reference prints a frequency
  range for any of these radios, and inventing one from a marketing page
  would be a bound the program then enforced against a user. A zero
  ceiling reads as "no ceiling" to `core/codeplug`'s validator, so an
  out-of-range frequency is refused by the radio rather than by this
  program.

- **The opening speed is a GUESS, and a wrong one is not a safe one.**
  `DefaultBaud` is 9600 (register entry A15, matrix §1.12) and neither
  book prints a factory value anywhere: the 590 pair's hardware page
  lists the selectable rates and no default, and the TS-480's list is a
  menu legend with none marked. There is **no speed setting** in this
  program's command line or window, and it never probes the port at
  several speeds. The failure mode is a timeout that looks exactly like a
  dead port or a bad cable, which is why the radiotext and the register
  entry both say so rather than leaving 9600 to look like a fact.

- **4800 is deliberately absent from the offered rates**, which are 9600,
  19200, 38400, 57600 and 115200. Both books print 4800 too, and both
  attach a condition to it that a flat list cannot express: on the 590
  pair it cannot be used over the USB-B connector most owners will use,
  and on the TS-480 it requires two stop bits where every other rate
  requires one, which is a per-session port setting this program does not
  vary by baud (matrix §1.11, erratum M-E4).

- **A CHIRP file's ordinary rows import on the TS-590S/SG.** A CHIRP
  file's blank `Duplex` column means simplex, and this family declares no
  shift vocabulary at all — the 50-byte record carries no duplex
  selector, so `ShiftOptions` is empty (`core/driver/ts590/caps.go`,
  matrix §1.16). A blank column nevertheless asks for nothing the radio
  cannot do, so such a row imports as simplex and nothing is reported.
  A `Duplex` column reading `off` is still refused: that asserts "no
  duplex configured" as distinct from simplex, which this record cannot
  carry (`core/csvio/chirp_test.go`'s
  `TestImportCHIRP_TS590PairBlocksCWAndRTTYRows`). In v1.4.1 and earlier the
  blank column was refused too and no row imported at all.

- **A CHIRP file's `CW`, `CWR` and `RTTY` rows are refused a second time,
  on the `Mode` column.** Those resolve to the sideband-specific names
  `CW-U`, `CW-L` and `RTTY-U`, and these radios' own mode legend prints
  `CW`, `CW-R`, `FSK` and `FSK-R`, so each such row is blocked with a
  reason naming both the CHIRP name and the name it mapped to, exactly as
  the eleven Icom models and the FT-891 already block them. **Kenwood
  spells RTTY "FSK"**, which makes a THIRD spelling family in this
  registry; teaching the importer to consult the radio's own legend for a
  sideband-agnostic alternative would change every Icom model's and the
  FT-891's CHIRP outcome as well, so it is a fleet question recorded as a
  follow-up and acted on nowhere. It would not open CHIRP import here on
  its own: the `Duplex` refusal above stands independently of it.

- **Tone and scan skip ARE reachable here**, which no registered radio
  before this family could say. The 50-byte record carries a tone mode, a
  transmit tone number, a receive tone number and a channel-lockout flag
  at printed positions, so those columns read and write like any other
  and a CHIRP `Skip` cell is carried literally instead of being dropped
  (`core/csvio/chirp_test.go`'s
  `TestImportCHIRP_TS590PairTakesTheLiteralScanSkipBranch`) — the
  importer's own step, reached whatever the row's fate: while the
  `Duplex` refusal above stands, no CHIRP row completes an import here at
  all, so that is a property of the mapping rather than something an
  owner can use in v1.4.0. The Kenwood
  tone chart has 43 entries and is NOT this project's shared 50-tone
  chart — it is that chart minus eight interstitial tones, so every index
  above 25 differs — and `core/driver/ts590` transcribes its own copy,
  with a test that the two tables are not equal.

- **Menu settings are READ and never written**, as on every radio this
  program supports. The tree is one flat list because these books print
  no group structure at all: 88 items on the TS-590S, 100 on the
  TS-590SG. Inventing a tens-decade grouping to look like the FT-710's
  two-level tree would put structure in a book that prints none.

## Costs the TS-590S pays and the TS-590SG does not

One book covers both radios, but it QUALIFIES BY ROW in one place, and
that one place costs the older radio two things.

- **The filter column cannot be set on a TS-590S at all.** Byte 28 of the
  memory record selects filter A or B. The book guarantees it is always
  zero on the **1.xx** firmware — it says so twice, in two slightly
  different wordings, which is errata entry E7 — and says nothing
  whatever about 2.00 and later. `spec.Capabilities` is a STATIC
  per-model value and the TS-590S is ONE registry row, so no row can
  publish "settable if the firmware is at least 2.00". The cost is
  published rather than hidden: a TS-590S at 2.00 or later loses its
  filter selection through this program. Byte 28 is still ACCEPTED as
  '0' or '1' on PARSE on both rows, because requiring '0' would make a
  2.00 radio using filter B fail the whole channel read
  (`core/driver/ts590/caps.go`'s `bankFields`; `core/kw`'s
  `Byte28FilterEither`; matrix §2.7).

- **Channel writes are refused outright on a TS-590S reporting firmware
  2.00 or later** — and on one whose firmware answer this program cannot
  read as a version at all (register entries A13 and A14). That refusal
  is CONSERVATIVE rather than evidenced: nobody here has seen a 2.00
  radio answer anything, and the alternative is to write a byte whose
  meaning the book stops guaranteeing at exactly that point. Reading is
  never blocked by the firmware version.

- **The firmware answer is printed verbatim.** `rigprog probe` shows a
  `Firmware answer:` line carrying the radio's own reply, quoted, for
  both 590 rows (`core/driver.FirmwareAnswerReporter`,
  `cmd/rigprog/probe.go`). It is the A13 mitigation: a user whose writes
  are refused is entitled to see the exact bytes the decision was taken
  on, and in the UNPARSEABLE case this program's own reading of those
  bytes is precisely what is in doubt. No other registered radio has such
  a line, because no other registered radio answers a firmware query at
  all.

The **TS-590SG** has neither cost: its byte 28 is live on every radio the
book describes, with no firmware condition attached, so it sets the
filter normally and no version gates its writes. The two radiotext
entries are pinned to differ in more than the model name for exactly this
reason (`internal/radiotext`'s
`TestRadiotext_TS590SAndSGDifferInMoreThanTheModelName`).

**The TS-590SG's channels 110–119 are not published.** One printed phrase
would make them ordinary memory records; no radio has confirmed it, and
publishing ten slots on the strength of a phrase would mean this program
offering to write channels nobody has read. They are a known, unreached
part of the radio rather than a bank, and publishing them later is
additive (plan decision P11, spec decision 15).

## Costs both 890S and 990S rows pay

- **A blank channel cannot be filled: "registered" does not mean "can
  programme a fresh radio".** One `MA0` Set carries the WHOLE channel, so
  the write path reads the target first and refuses when that read
  satisfies the blank predicate, naming register entry **A3**. Whether an
  `MA0` Set can CREATE an unassigned channel is printed nowhere for that
  command, while five sibling commands print an unassigned-channel
  prohibition in the 890S book (`890:3265`, `890:3282`, `890:3300-3301`,
  `890:3324`, `890:3356`) and four in the 990S's (`990:3008`, `990:3023`,
  `990:3037-3038`, `990:3058`) — so a create path exists in the family and
  only `MA0`'s own participation is unknown. A3 is scoped PER ROW and its
  lift is **L-HW-3**: set a channel confirmed blank from the front panel
  and read it back. Until then these radios can be re-programmed, not
  programmed from empty. `core/driver/ts890/write.go`,
  `core/driver/ts990/write.go` (rung 10).
- **The secondary side is readable and is not rewritable from nothing,
  and that is a different statement from the absence list below.** One
  Set rewrites the whole record, primary and secondary sides together,
  and this program has a source for the primary side and none for the
  secondary. Rung 11 therefore compares the radio's OWN secondary
  parameters against what the Set would emit and refuses, naming the
  parameter and both values, rather than overwriting a side the file
  never described. **M-E5's distinction applies here in the opposite
  direction**: the fields listed as absent below (`attenuator`, `preamp`,
  `antenna`, `filter`, the tuning steps) have no position in the record at
  all and their absence says nothing about the radio, whereas the
  secondary side HAS a position, IS read, and is refused on the write —
  a refusal about a field the program can see.
- **A 1750 Hz receive tone is refused**, on both rows. Each row's
  tone-number chart runs to 1750 Hz as its last entry and each row's
  tone-squelch chart has no entry for it at all, so the program publishes
  51 tones, writes 1750 Hz as a transmit tone and refuses it as a receive
  one. It cannot come OFF a radio either: the receive chart stops short
  of it, so no answer can carry it.
- **Slots 100-119 are unreached, and the reason is evidential rather
  than a vocabulary limit.** Both books print the whole slot space,
  "000 ~ 119", with "Channels P0 ~ P9 are represented as 100 ~ 109 and
  channels E0 ~ E9 are represented as 110 ~ 119" (`890:3167-3169`,
  `990:2893-2896`). What neither book prints is which parameter of an
  `MA0` read selects a section channel's START frequency and which its
  END — the 590 book does print that for its own command, which is why
  that row HAS a scan bank — so a two-slot-per-index bank here would be a
  reading. The CODEC admits those channel numbers (`core/kw/ma`'s slot
  ranges carry `SlotScan` 100-109 and `SlotExtension` 110-119, so an
  answer for one parses rather than being refused); the DRIVER publishes
  no bank containing them, and no channel is ever produced for a class
  published in no bank. Publishing them later is additive (plan decision
  P11).
- **`MinFreqHz` and `MaxFreqHz` are 0 on both rows.** Neither PC control
  command reference prints a frequency range for its radio, and a band
  plan from a specification sheet would be a claim from a document this
  project does not hold. An out-of-range frequency is refused by the
  radio, not by the program.
- **The speed is assumed: 9600, with no override and no probing.**
  No Kenwood book held here prints a factory rate. A wrong speed is not a
  safe failure but an unreachable radio, and its symptom is a timeout
  indistinguishable from a dead port or a bad cable — which is why the
  assumption is stated to the user rather than buried.
- **A write to the channel the radio is displaying may read back old.**
  Both books print it in the same words: "When setting the channel
  currently being accessed, the new settings are reflected the next time
  that channel is accessed" (`890:3211-3212`, `990:2958-2959`). The
  program's read-back verification can therefore show the previous
  contents for that one channel.
- **No channel is ever deleted, over a PRINTED command.** Both books
  print a dedicated deletion command — `MA5`, "Memory Channel (Channel
  Deletion)" (`890:3305`) and "Channel Deletion" (`990:3042-3047`) — which
  is STRONGER evidence than the 590 pair's ambiguous short-frame side
  effect, and the standing no-erase rule declines it anyway.
  `core/kw/ma`'s outbound gate refuses any `MA5` frame besides, so the
  refusal is positive rather than a matter of nobody having written the
  builder.
- **The data modes are IN the mode names (M-E3).** The 890S publishes
  `LSB-D`, `USB-D`, `FM-D`, `FM-D-N` and `AM-D` among its sixteen names;
  the 990S publishes three numbered sets (`D1`, `D2`, `D3`) among its
  twenty-six. Neither record has a separate data-mode byte, so
  `data_mode` is the zero `FieldSupport` on both rows and a channel's
  data disposition travels in the mode column of a CSV or a CHIRP file.
  A round trip preserves it; a file carrying a `data_mode` column for one
  of these radios would be describing a field the record has not got.
- **Auto Information is switched off PER CONNECTOR.** "The AI function
  can be set separately for USB connector, COM connector, or LAN
  connector" (`990:187-188`; the 890S has three connectors likewise), so a
  session's `AI0;` affects the one it is using and leaves the others
  alone.
- **CHIRP.** A blank `Duplex` column imports as simplex on both rows —
  these are the first two rows DESIGNED under that arm rather than having
  it applied retrospectively — and `off` still blocks, because it asserts
  "no duplex configured" as distinct from simplex and the record has
  nowhere to carry the distinction. `CW`, `CWR` and `RTTY` block on the
  mode: `chirpModeMap` resolves them to `CW-U`, `CW-L` and `RTTY-U`, and
  Kenwood spells RTTY `FSK` and prints CW without a sideband suffix. **A
  CHIRP import is refused at the write on these two rows today**, and
  that is a fleet question rather than a Kenwood one: a CHIRP row states
  no transmit frequency, `core/csvio` leaves `TxFreqHz` Unknown for any
  bank that grades the field, and rung 6 refuses a candidate with no
  Known transmit disposition (M-E8). The root fix — grading a blank or
  `off` `Duplex` on such a bank as Known 0, the radio's own simplex
  statement — moves the TS-590 pair's AND the IC-7300/IC-7300MK2's import
  artefacts too: every bank that grades `FieldTxFrequency` without grading
  `FieldDuplex` takes this branch (`core/driver/ts590/caps.go:399`,
  `core/driver/ic7300/caps.go:349` against `:363`), which is six
  registered rows, so it is deferred to a v1.5.x follow-up rather than
  landed inside this registration.

## Costs the TS-990S pays and the TS-890S does not

- **A channel with dual reception ON is refused unconditionally.** P16,
  "1: Dual reception ON" (`990:2949-2951`), describes a second RECEIVER
  over the channel's frequency-2 side. No field of this program's channel
  model names such a thing, and one Set rewrites the whole record, so a
  write would silently switch the second receiver off. The refusal does
  not consult the split flag beside it: what P15 says about frequency 2
  is a separate statement, and neither reading makes the second
  receiver's frequency something this program holds. The 890S's record
  carries no such flag at all, so it has no such refusal.
- **A channel whose own answer types it as SECTION DEFINED is refused at
  the write**, naming register entry **A8**: the book decides the channel
  type "while setting the P9 and P10 values" (`990:2901-2903`), and what a
  Set would put in a section-defined channel's frequency-2 window is not
  something this program can state. Reading is unaffected.
- **The lockout flag is printed `1`/`2` here and `0`/`1` on the 890S**
  (erratum E8): `990:2952-2954` prints "1: Scan Lockout OFF / 2: Scan
  Lockout ON" for the memory record, while the same book's `MA3` prints
  `0`/`1` a few pages later (`990:3020-3021`). Each row's codec accepts
  only the two values its own memory-record chart prints, so neither
  convention can leak into the other.
- **Twenty-six modes against sixteen**, from each book's own OM P2
  legend. The difference is entirely in the data sets: one on the 890S,
  three on the 990S.
- **194 menu settings against 158**, each row's own generated inventory.

## The TS-480 is built and not registered

`core/driver/ts480` is complete: identity probe, hardware-variant read,
the whole read path, the 61-item menu read, and a write path that refuses
everything. It is absent from the model list, from the real-driver table
and from the simulated-driver table, so nothing can select it.

**Why.** Register entry A4, which reads in full: "An `MR` of an empty
channel **answers** (with P4–P15 zero) rather than rejecting, on the
TS-480 too." That is documented for the TS-590SG (`590:1492-1493`) and
printed NOWHERE in the 2003 TS-480 book, which says nothing about an
empty channel at all. If A4 is false — if an `MR` of an unwritten channel
answers `?;` — then decision 5 makes that `?;` a definitive rejection,
the session read fails whole, and a fresh TS-480 out of the box cannot be
read at all. Registering the row would mean shipping a radio that may not
answer its first channel.

**The gate is L-HW-3, hardware confirmation item 3, and it is not one
read.** It is a valid zero record on at least **three separate channels**,
each confirmed unwritten from the radio's front panel, across at least
**two sessions**, with no silence and no `?;` among them, every request
and answer kept as the exact bytes that went over the wire, and an
**observer named**. `internal/wiring/testdata/README.md` is written for
the person who would take that observation, and
`checkTS480ObservationBar` in `internal/wiring/ts480gate_test.go` is what
decides — if that guard and this page ever disagree, the guard is what
ships.

**How the absence is enforced.** By absence, and by the guard in
`internal/wiring/ts480gate_test.go`.
`internal/wiring/testdata/ts480-a4-observation.json` is where the
observation goes, on a path tracked in git precisely so that a fresh
clone cannot read "no evidence" merely because a file was never handed to
it, and the guard has three legs — absent means ABSENT (a branch, not a
skip), present means present only after parsing the file and checking the
bar it records, and a non-vacuity leg so that a file recording zero
trials fails loudly rather than counting as absence.

**Registering it later is TEN edits, not one**, and nine of the ten now
fail loudly in the test suite the moment the row appears without them —
the tenth only at the byte-identity gate. The last two became loud at
this milestone: `internal/guards`' simulated-token table gained a
completeness guard against the registry, and `core/csvio` gained one for
its CHIRP fixtures. Both were silent before.

**Its prose already exists.** `internal/radiotext` carries a TS-480 entry
today, although the row is unregistered: nothing in the shipped binary
can reach it, and landing it now is what keeps that prose under the same
non-borrowing and vocabulary discipline as every other radio's from
birth, rather than arriving unreviewed inside the registration commit.

**What that entry has to say, and what a future owner should know:**

- **Every channel write is refused** (register entry A22), and a write of
  a channel whose tone mode is not OFF names a SECOND cause alongside it
  (Q2): this radio's two printed tone charts do not agree with each other
  about how many tones there are, so there is no honest tone number to
  send. The two travel as one typed error with a primary cause and a
  subsidiary list, because a separate tone rung placed after "refuse
  everything" could never execute.

- **Half of the memory-frame read is unreachable.** `MR` P1 selects a
  frame's receive or transmit side on this radio too, but the TS-480
  publishes no scan-range bank and no split, so P1=1 is never sent: the
  transmit half of a channel is simply not read, and the program says so
  rather than reporting a simplex channel it has not confirmed.

- **Memory GROUP membership cannot be preserved.** The radio has ten
  memory groups and an `SU` command that chooses which of them are
  scanned, but NO memory frame carries a channel's group and no command
  reads or writes one. A codeplug round trip therefore loses which group
  a channel belonged to. There is nothing to refuse and no column to
  grade, so the loss is silent unless it is written down — which is why
  it is written down here and in that radio's own radiotext entry
  (matrix §3.10).

- **There is no readable firmware version at all** — no version query in
  the whole printed command set, no revision number, no part code and no
  firmware statement anywhere in the book (errata entry E15). The second
  frame of this radio's probe asks a different question: it reads the
  HARDWARE VARIANT, and an unexpected value there refuses the session
  outright, where an unreadable firmware version on a 590 only degrades
  the session to reading. The asymmetry is deliberate — refuse where the
  book is complete and the radio is outside it, degrade where the book is
  thin and this program may have guessed its grammar wrong.

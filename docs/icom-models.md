# Icom models — per-model limitations and evidence

Moved verbatim from the README on 28/08/2026 so the README can stay short. Every claim below cites the code that makes it true; the citations are for reviewers and contributors.
## The eleven Icom models

IC-7610, IC-7300, IC-7300MK2, IC-705, IC-9700, IC-905, IC-7851,
IC-7850, IC-7760, IC-7100 and IC-R8600 talk a different wire protocol
from the Yaesu models (Icom's CI-V, rather than Yaesu's CAT), and each
carries its own honesty rows beyond the README's shared "no radio has
ever been connected" small print.

Eleven models, TEN memory formats: the IC-7851 and the IC-7850 are two
entries in the model list over one manual, one address and one record
format, because this program cannot tell them apart (see the sections
below).

TEN TRANSCEIVERS AND ONE RECEIVER. The IC-R8600 is a communications
receiver, and this program says so rather than treating its missing
transmit fields as fields it declines to write: its capabilities declare
`spec.ReceiveOnly`, which removes the transmit-frequency and
transmitted-tone columns from the grid by anatomy and makes grading
either of them above "unsupported" a validation FAILURE rather than a
choice (`core/driver/icr8600/caps.go`'s `Transmit`; additions spec D4.2's
invariant, pinned by `internal/wiring`'s
`TestEveryRegisteredModelDeclaresItsTransmitAnatomy`, which also refuses
a future row that forgot to say either way).

Three costs are shared by all eleven:

- **No `--civ-address` option.** Each driver talks only to its one
  factory CI-V address (98h IC-7610, 94h IC-7300, B6h IC-7300MK2, A4h
  IC-705, A2h IC-9700, ACh IC-905, B2h IC-7760, 88h IC-7100, 96h
  IC-R8600, 8Eh IC-7851 AND IC-7850 — that last address is printed in the
  manual as the default for both radios) and
  there is no setting to change it. Two different things can happen when this driver meets a radio
  it did not expect. A different Icom model at ITS OWN factory address
  simply does not answer — nothing was heard from, so nothing can be
  attributed, and Open reports a plain timeout
  (`core/driver/ic7610/doc.go:188-192`; `core/driver/ic7300/doc.go:229-234`;
  `core/civ/tier_test.go:27-33`). A radio actually sitting ON this
  driver's one address is instead caught by the probe's
  address-geometry and record-length fingerprint — but whether that
  refusal NAMES a wrong radio depends on the model, and attribution is
  not the default: IC-7300, IC-7300MK2, IC-705 and IC-905 each mint a
  `driver.WrongRadioError` naming what they found
  (`core/driver/ic7300/ic7300.go:270`; `core/driver/ic7300mk2/ic7300mk2.go:272`;
  `core/driver/ic705/ic705.go:364`; `core/driver/ic905/ic905.go:495`).
  The IC-7100 mints one too but names NOBODY: it carries the two
  record-only lengths and leaves both model fields empty unless a caller
  supplies an attribution table, and no registered composition supplies
  one, so today its refusal always renders as the lengths-only form
  (`core/driver/ic7100/ic7100.go:211-221`; `core/driver/ic7100/doc.go:35-39`).
  The IC-R8600 is in the same position and for the same reason: its walk
  turns a record-length mismatch into a `driver.WrongRadioError` carrying
  its own six accepted lengths and the one it got, and NO model name —
  which is the only honest thing it could carry, since no other registered
  profile accepts any of those six over a four-byte address
  (`core/driver/icr8600/read.go:65-72`).
  Meanwhile IC-7610, IC-9700, IC-7851, IC-7850 and IC-7760 NEVER mint one
  for any same-address collision, by design — they hold no cross-model
  table and refuse to guess an identity they cannot support
  (`core/driver/ic7610/ic7610.go:142-145`;
  `core/driver/ic9700/ic9700.go:325-329`; `core/driver/ic9700/doc.go:391`;
  `core/driver/ic7851/ic7851.go:168-185`;
  `core/driver/ic7760/ic7760.go:141-151`).
  Even where a driver CAN attribute, one pair defeats it in practice:
  an IC-9700 moved onto the IC-705's A4h address fails the IC-705's
  open as an unattributed address parse error, not as a named
  wrong-radio refusal, because the address-geometry check pre-empts
  the length check that would otherwise have named it
  (`core/civ/tier_test.go:68-80`).
- **The tone picker stays list-driven while every model's tone range
  is numeric (enabler E3).** All eleven declare a numeric
  `CTCSSToneRange` rather than a fixed tone chart, because their tone
  spans are BCD frequencies, not indices — but the picker widget
  itself was built for a list. The channel grid still shows and
  round-trips tones on every model; only the picker cannot offer them
  (`core/driver/ic7610/caps.go:342-345`; `core/driver/ic9700/caps.go:177-179`;
  same declaration in each of the other models' own `caps.go`).
- **A channel outside the write gate's template is refused, never
  silently changed (ruling E6).** Every one of these records carries
  at least one region no `codeplug` field maps, and a slot may be
  written only when those unmapped regions already match the
  profile's template. What that costs differs by model:
  - **IC-7610, IC-705, IC-9700, IC-905, IC-7851, IC-7850, IC-7760**: a
    channel already in a Select scan group (★1/★2/★3) cannot be written
    by this program at all — the SELECT nibble is unmapped and there is
    no honest value to preserve, so the write is refused, naming the
    reason
    (`core/driver/ic7610/doc.go:220-249`; `core/driver/ic705/write.go:265-280`;
    `core/driver/ic9700/write.go:529-531`; `core/driver/ic905/write.go:578-590`;
    `core/driver/ic7851/doc.go:274-306`;
    `core/driver/ic7760/write.go:36-42,54-60`;
    `core/driver/ic7100/write.go:114-116`).
    The IC-7610, the IC-7851/IC-7850 and the IC-7760 additionally refuse
    a channel whose data mode is DATA 1/2/3, for the same
    unmapped-nibble reason
    (`core/driver/ic7610/doc.go:235-238`;
    `core/driver/ic7851/caps.go:202-224`;
    `core/driver/ic7760/caps.go:186-207`). The IC-7100 does NOT: its data
    mode is a two-valued OFF/ON byte of its own, mapped and writable
    (`core/civ/ic7100/profile.go`'s `dataModeNames`). What it refuses
    instead is a Split-ON channel — the split flag shares record byte ④
    with the select nibble and its high half is unmapped, so writing
    would silently clear it — and any channel whose D-STAR call-sign,
    DSQL or CSQL bytes differ from the assumed template
    (`core/driver/ic7100/write.go:169-198`).
  - **IC-7300 and IC-7300MK2**: a Select-group channel writes
    normally — the SELECT nibble round-trips, carried through
    unchanged from the record the radio holds
    (`core/driver/ic7300/write.go:455-477`;
    `core/driver/ic7300mk2/write.go:463-485`). What is refused instead
    is a Split-ON channel: it reads normally but cannot be written
    back, because the split flag shares record byte ③ with the SELECT
    nibble and the profile leaves the whole byte's high nibble
    unmapped (`core/driver/ic7300/doc.go:174-178`;
    `core/driver/ic7300mk2/doc.go:187-195`). Both also refuse a CREATE
    into an empty slot, since the SELECT nibble has no honest default
    to write (`core/driver/ic7300/write.go:433`;
    `core/driver/ic7300mk2/write.go:441`).
  - **IC-R8600**: three costs. The first is the IC-7300s' own, and it
    applies here for the same reason: a CREATE into an empty slot is
    refused, since the record's SELECT group has no honest default to
    write (`core/driver/icr8600/write.go:267-272`). The other two are
    this receiver's own, and neither is the Select-group one the other
    models pay — this receiver's record carries the Select group in
    the LOW half of its first byte and a printed scan-skip setting
    (SKIP OFF / SKIP / PSKIP) in the HIGH half, and it is the HIGH half
    that E6 guards, so a channel the operator has marked as skipped is
    refused rather than rewritten as unskipped
    (`core/driver/icr8600/write.go`'s `commonUnmappedHighNibbles`, and
    matrix §2 row 9 for the printed enum). The third is this radio's
    own: its five digital classes — D-STAR, P25, NXDN, DCR and dPMR —
    each carry a squelch tail no `codeplug` field maps, so a stored
    channel whose tail differs from this build's assumed template cannot
    be written back, and a change of mode INTO one of those classes is
    refused too, its whole tail having to be invented
    (`core/civ/icr8600/profile.go`'s `DigitalTailRefusalReason` and
    `digitalTailTemplate`, register entry `icr8600-tail-templates`;
    `core/driver/icr8600/write.go`'s target-class arm). Its DATA mode is
    not a cost at all: this record has no data-mode byte to refuse.
  In every case the write is refused, naming the reason, never
  downgraded or cleared.

All eleven open at 19200 baud by default, but what grades that default
differs, and none of it is a reading of a printed factory value:

- **IC-7610** — ASSUMED, an arbitrary pick among the six rates the
  guide names; it marks no default at all
  (`core/driver/ic7610/doc.go:200-219`; `core/driver/ic7610/caps.go:349,359`).
- **IC-7300** — a CHOICE: the highest rate common to this radio's
  printed `[USB]` and `[REMOTE]` rate lists, since both its baud items
  ship set to `Auto` (`core/driver/ic7300/caps.go:261-268`).
- **IC-7300MK2** — a conservative derivation from a wake-up-command
  table this guide prints for an unrelated purpose; it names no baud
  list and no factory default at all
  (`core/driver/ic7300mk2/doc.go:301-317`).
- **IC-705** — ASSUMED, and so is the whole baud list: this radio's
  CI-V Reference Guide prints no baud information for the CI-V port at
  all (`core/driver/ic705/caps.go:197-205`; matrix §1 #9).
- **IC-9700** — ASSUMED, the middle of the six rates this guide
  prints; the guide itself defers the factory setting to the
  instruction manual, which this project does not hold
  (`core/driver/ic9700/doc.go:141-144`).
- **IC-905** — ASSUMED (the default) over a CHOICE (the five-rate
  list it is chosen from is the wider CI-V family's conventional set,
  not a claim about what this radio itself accepts): this radio's
  guide prints no rate figure anywhere
  (`core/driver/ic905/doc.go:141-163`).
- **IC-7851 and IC-7850** — ASSUMED, and arbitrary in a way the manual
  itself forces: both CI-V speed settings print "(Default: Auto)", so
  there is no numeric factory value to prefer and 19200 is a pick from
  the printed set. A wrong pick costs a clean timeout at connect and
  never a wrong byte, because the identity probe requires an
  address-matched reply and silence is silence
  (`core/driver/ic7851/doc.go:245-265`).
- **IC-7760** — ASSUMED, and so is the whole six-rate list it is picked
  from: this radio's CI-V Reference Guide prints no baud figure anywhere,
  about any port, and its own CI-V settings block carries no speed item
  at all. The only adjacent printed line says a "data communication
  speed" needs setting when the cable goes to the RF deck's remote jack,
  which is not the path this build supports
  (`core/driver/ic7760/caps.go:361-374`; `core/driver/ic7760/doc.go:77-81`;
  matrix §1 row 10, §3.3).
- **IC-7100** — ASSUMED: 19200 is the highest of the five rates this
  radio's manual prints, picked because the radio's own CI-V baud item
  ships set to `Auto` and names no number to prefer. The manual adds a
  reason to distrust any printed default here at all — it states that its
  own factory settings differ between transceiver versions — which is why
  the register entry asks a lift to record the radio's version alongside
  the speed it confirms (`core/driver/ic7100/caps.go:33,38,131-135`;
  `core/driver/ic7100/register.go`, entry `ic7100-default-baud-auto`).
- **IC-R8600** — ASSUMED on BOTH halves, which is worth stating plainly
  rather than letting it sit in a list of near-identical hedges. This
  receiver's CI-V Reference Guide prints no factory default speed,
  mentions no `Auto` setting anywhere in a baud context, and never prints
  the list of speeds its own menu offers either. So both the 19200 opening
  rate AND the six-rate list it was picked from are assumed here, under two
  separate register entries rather than one
  (`core/driver/icr8600/caps.go`'s `Bauds`/`DefaultBaud`;
  `core/driver/icr8600/doc.go`, entries `icr8600-baud-set` and
  `icr8600-default-baud`; matrix §3.3). A wrong pick costs a clean
  timeout at connect and never a wrong byte, because the identity probe
  requires an address-matched reply and silence is silence. It is not
  alone in this: the IC-7760, IC-705 and IC-905 entries above record the
  same condition — a guide that prints no baud figure, so rate and list
  both assumed — and no ranking among them is claimed.

What is specific to one or two models:

- **IC-705 default discovery finds CALL only.** This radio's MEM bank
  is sparse (a 100-group × 100-channel space), and its slots are only
  known once a walk has visited them. `Connect Demo`, and any Open
  against a radio with nothing seeded, therefore surfaces the four
  fixed CALL channels and zero memories on a whole-radio read
  (`core/driver/ic705/inventory.go:99-103`;
  `internal/fakeic705/fakeic705.go:53-61`; `internal/wiring/fake.go:463-465`).
  The default walk covers only display groups G01-G10 (1,000 CI-V
  exchanges); `WithFullInventoryWalk()` widens it to the whole space
  (10,000 exchanges), but that option has no `rigprog` flag and no GUI
  control — it can only be reached by code that imports the driver
  package directly (`core/driver/ic705/inventory.go:12-16`;
  `core/driver/ic705/ic705.go:52-65`).
- **IC-705's CALL-group channel cap is narrower in practice than in
  the protocol layer.** This radio only documents CALL-group channels
  0-3, but the underlying CI-V layer would technically admit 4-99 too
  (ruling O-9, deliberately deferred). Nothing reaches those extra
  channels in this build: this driver's own slot parser refuses every
  one of them before any wire traffic is sent
  (`core/driver/ic705/doc.go:158-184`).
- **IC-9700's duplex offset scale is an open question (Erratum 14),
  and this project's own write-then-read-back check cannot catch a
  wrong answer.** Two independent readings of the offset field
  disagree by a factor of ten, and the disagreement is unresolved —
  get it wrong and every offset this driver reads or writes is out by
  ×10. Because the encoder and decoder consult the same scale
  constant, a write followed by a read-back is internally consistent
  either way; only a hardware capture against a known physical offset
  can settle which reading is right
  (`core/driver/ic9700/doc.go:308-317`; `core/civ/ic9700/profile.go:293,312`).
- **IC-905's demo radio starts empty by design.** The underlying fake
  rig's own factory-shaped default is ten occupied channels holding an
  all-zero record this driver's own filter refuses to decode, so
  `Connect Demo` deliberately empties all ten before opening rather
  than ship a demo whose first read fails
  (`internal/wiring/fake.go:471-500`).
- **IC-905's default Open discovers its MEM bank by a bounded walk**
  — group 0 in full, then channel 00 of every other group, descending
  into the rest of a group only where its channel 00 answered — not
  the whole 100×100 space, and there is no setting that widens it. A
  channel stored outside that walk is simply not listed; its absence
  from the grid is not evidence the channel is empty
  (`internal/radiotext/radiotext.go:1111`).
- **A non-octal DTCS code (IC-705 and IC-905) reads back as Unknown,
  not as a wrong number, and the same check blocks writing one until
  it is corrected.** Both radios' printed DTCS range is octal (digits
  0-7 only); a decoded value with an 8 or a 9 in it is a real nibble
  pattern but not a DTCS code this project's vocabulary recognises
  (`core/driver/ic705/read.go:160-164`; `core/driver/ic705/write.go:115`;
  `core/driver/ic905/read.go:189-210`; `core/driver/ic905/write.go:458-470`).
- **The IC-7851 and the IC-7850 cannot be told apart, and the model
  shown is the one you chose.** They share one instruction manual, one
  CI-V address (8Eh, printed as the default for both), one frame shape
  and one memory record, and the identity command's reply value is
  printed nowhere for either — so the probe has nothing to separate them
  with. The program lists them as two entries and reports back whichever
  you selected; that is a choice recorded, never a detection. Evidence
  for one is never evidence for the other, which is why the two write
  guards are separate constants
  (`core/driver/ic7851/doc.go:46-84`; `core/driver/ic7851/caps.go:41-42`;
  `core/civ/tier_test.go`'s `indistinguishable` table).
- **The IC-7851/IC-7850 baud list is the USB port's, and the older
  remote-jack path cannot reach the top of it.** The manual prints two
  rate lists — six rates for `[USB B]`, but only 4800/9600/19200 for the
  `[REMOTE]` jack a CT-17-style level converter uses — and the program
  can declare only one. It declares the USB superset, so the port picker
  will offer 38400, 57600 and 115200 to a user wired to `[REMOTE]`,
  where the radio cannot go above 19200. The program has no way to tell
  which path is wired (`core/driver/ic7851/doc.go:266-273`;
  `core/driver/ic7851/caps.go:387-392`).
- **An IC-7851/IC-7850 channel whose record disagrees with the manual's
  printed fixed digits is refused on READ, not silently mis-read.**
  Three bytes of this radio's record are drawn with a literal zero in
  both halves — one in the frequency block and one at the head of each
  repeater-tone group — so the program maps no field onto them. A radio
  that answered with a digit in one of those bytes would otherwise be
  read as a frequency a hundred times too small and written back with
  the byte quietly zeroed; instead the read fails, naming the byte
  (`core/driver/ic7851/doc.go:307-329`).
- **The IC-7760 is two boxes, and only one of its connections is
  supported.** The CI-V link this build talks to is the USB socket on
  the back of the controller, which enumerates as TWO virtual COM ports
  (called USB (A) and USB (B)); which of them carries CI-V is a
  front-panel setting and the guide prints no default for it, so if one
  port is silent the other is worth trying before the radio is blamed.
  The RF deck's own remote jack is a second, unsupported path, the
  bridge address at `1A 05 01 51` is not the radio's own address, and
  the LAN CI-V path has no documented framing at all
  (`core/driver/ic7760/doc.go:22-24,114-116`; matrix §3.15.4).
- **Whether the IC-7760's two scan edges can be cleared at all is not
  known.** This radio prints two clear forms — a `1A 00` set carrying
  `FF` in place of the record, and a top-level memory-clear command —
  and this build sends neither, exactly as for every other Icom model
  here. What is specific to this radio is the silence: the clear block
  names "Memory channel (00 01~00 99)" and says nothing whatever about
  P1 or P2, so the program tells the user that nobody knows rather than
  that the radio refuses (register entry `ic7760-clear-scope`;
  `core/driver/ic7760/doc.go:107-110`; matrix §3.13).
- **The IC-7760's memory inventory is the one part of it that is NOT
  assumed.** Its 99 memories plus the two programmed scan edges are
  printed with their own addresses on two separate pages of the guide,
  so the count carries no assumption of its own (additions-spec
  Erratum 5; `core/driver/ic7760/caps.go:135-156`;
  `core/driver/ic7760/doc.go:118-121`). Almost everything else about the
  radio's link — its framing, its baud, its control-line behaviour, its
  identity reply and whether an empty slot answers at all — is an entry
  in that driver's assumption register, each with the one capture that
  would settle it.
- **The IC-7610, the IC-7851/IC-7850 and the IC-7760 cannot be told
  apart by the length check that guards against a mis-set bus.** All
  three read and write a 25-byte record at a two-byte channel address,
  because all three manuals draw the same 27-byte block — which is what
  the design predicted, and which three independently derived profiles
  then produced. It costs nothing in normal use: the three factory
  addresses (98h, 8Eh, B2h) are all different, so a wrong radio at its
  own address never answers. It costs the fallback: a radio MOVED onto
  another of those addresses would answer a record of exactly the length
  expected, and none of the three drivers would name it
  (`core/civ/tier_test.go`'s `indistinguishable` table, which carries all
  three pairings with their citations).
- **Two of those three records are byte-identical inside, and the third
  is not — deliberately.** The IC-7610's and the IC-7760's field layouts
  agree exactly, span for span. The IC-7851/IC-7850's differs in three
  places: where the other two map a printed always-zero pad cell inside
  the frequency and tone fields and bound the value further up, that
  profile leaves the pad cells out of the fields altogether, so a radio
  answering a digit in one of them fails the READ rather than being
  re-encoded with the byte quietly zeroed. Both readings are defensible
  and each is the one its own document supports; the difference is
  measured and recorded rather than smoothed over
  (`core/civ/tier_test.go`'s `TestTierRecordShapes_7610CloneFamily` and
  its `declaredCloneDivergences` table).
- **The IC-7100's ten special channels are not read at all, and that is
  a refusal rather than an omission.** This program lists the radio's 495
  ordinary memories — banks A to E, channels 001 to 099, the only Icom
  model here whose slot names carry a bank letter — and stops there. The
  radio also has six programmed scan edges (0100–0105) and four call
  channels (0106–0109), and the manual's field legend names them; what it
  never says is what the BANK byte carries for any of them, and its
  clearing block omits that byte altogether. Reading one would mean
  putting an invented address on the wire, so the profile declares no
  extra range and the driver refuses every channel outside 1–99 before
  any traffic (`core/driver/ic7100/read.go:28-44`;
  `core/civ/ic7100/doc.go`, register entry `ic7100-special-bank-byte`,
  whose lift is to select scan edge 0100 and call channel 0106 at the
  front panel, read each with `1A 00` and record byte ①). Their absence
  from the channel list is not evidence the radio has none.
- **The IC-7100 is the one Icom model here opened with TWO stop bits.**
  Every other Icom driver reports 8-N-1 to the serial layer; this one
  reports nothing, so the port opens at the program's own 8-N-2 default.
  The reason is an absence in the document rather than a reading of it:
  the single 8-N-1 sentence anywhere in the manual belongs to the DV
  low-speed DATA application and not to the CI-V link, so this project
  holds no framing evidence for this radio at all and declines to invent
  one (`core/civ/ic7100/doc.go`, register entry `ic7100-serial-framing`;
  `core/driver/ic7100/doc.go:10-13`; pinned at the wiring seam by
  `internal/wiring`'s `TestOpenRealSessionFor_IC7100OpensAtEightNTwo`).
- **The IC-9700 and the IC-7100 cannot be told apart by the length check
  either, and this pair fails differently from the 25-byte three.** Both
  read a 111-byte record behind a three-byte address whose leading byte
  is a small index — a band 01–03 on one, a bank 01–05 on the other —
  and nothing on the wire distinguishes those. The third radio with a
  111-byte record, the IC-705, IS separable: it addresses a channel in
  four bytes, and the program proves that separation exhaustively rather
  than declaring it. What makes this pair's limitation sharper than the
  25-byte set's is that these two records are NOT claimed to be alike
  inside: the IC-7100's carries a 47-byte transmit duplicate and a
  sixteen-character name where the IC-9700's does not, and they still
  fingerprint identically, because a fingerprint sees only length and
  address width. In normal use it costs nothing — the factory addresses
  (A2h and 88h) differ, so a wrong radio at its own address never answers
  — and it costs the fallback, a radio MOVED onto the other's address
  (`core/civ/tier_test.go`'s `indistinguishable` table, entry
  `IC-9700|IC-7100`; `core/driver/ic7100/doc.go:14-45`).
- **The IC-R8600's memory capacity is not documented, and the program
  says so rather than showing a total it invented.** Every other model
  here has a printed number of memories; this receiver's CI-V guide has
  none, in either of the two places one would be printed, and the words
  never appear anywhere in the document. So the bank declares
  `BudgetUnstated` — a positive statement of the silence rather than a
  zero that could be mistaken for "none" — and `codeplug.Diff` skips the
  over-budget refusal it applies to every other sparse bank
  (`core/codeplug/diff.go:536-543`). The flag is carried through to the
  UI layer on the bank itself (`app/types.go`'s `BankView`), so nothing
  downstream has to infer the silence from a zero. What the receiver does
  when its memory is full is unknown to this program, and it cannot warn
  you beforehand
  (`core/civ/icr8600/profile.go`'s `MemoryBank`;
  `core/driver/icr8600/caps.go`'s `BudgetUnstated`; additions spec D3.4,
  register entry `icr8600-budget`, whose lift is to fill ordinary
  memories on a real receiver until another write is refused).
- **The IC-R8600's memory space is zero-based in both dimensions, which
  no other model here is.** Its slots run `G00-000` to `G99-099`: groups
  0000–0099 by channels 00–99, with BOTH counting from zero, where the
  other group-addressed models count their groups or their channels from
  one (`core/civ/icr8600/profile.go`'s `MemoryGroupBase` and
  `MemoryChannelBase`, additions spec Erratum 2). The program discovers
  what is actually stored by the same kind of BOUNDED walk the IC-705 and
  IC-905 use — group 0 in full, then each later group's channel 00 as a
  sample, reading the rest of a group only when that sample is occupied
  — so between 199 and 10,000 reads, never outside the declared space.
  A channel stored in a group whose channel 00 is empty is therefore not
  read, and its absence is not evidence the receiver has none
  (`core/driver/icr8600/read.go`'s `discover`). Two further groups the
  guide names — 0100 Auto-Write and 0101 Scan Skip — are radio-managed
  lists this program deliberately does not address, and group 0102, the
  programmed scan edges, stays out until its A/B-suffixed channel
  numbering is established (register entry
  `icr8600-scan-edge-encoding`).
- **The IC-R8600 is the only model here whose record length depends on
  the MODE, and the only one this program can still tell apart from
  every other radio.** Its memory record is a fixed 37-byte head followed
  by a tail chosen by the mode byte, so it accepts six record-only
  lengths — 37, 39, 41, 43, 44 and 45 — where every other model accepts
  one or two. FM and DCR both land on 44 and are separated by the mode
  byte rather than by length, which is what `civ.DiscriminatorModeByte`
  exists for (`core/civ/icr8600/profile.go`'s `recordLayouts`; additions
  spec D3.3). Two of those lengths, 39 and 45, are exactly the IC-7300's
  and the IC-7300MK2's, so the length fingerprint alone cannot separate
  those two pairs — but the address width does, this receiver using four
  address bytes where both IC-7300s use two, and the program MEASURES
  that separation rather than declaring it. It is the first model in
  this tier to add no entry at all to the declared-indistinguishable
  table (`core/civ/tier_test.go`'s
  `TestTierRecordShapes_DistinctOrDeclared`, whose printed table shows
  this row separated from all nine others).
- **The IC-R8600 is the only model here that shows the seven receiver
  columns.** Tuning step and its on/off flag, the programmable tuning
  step, the attenuator, the preamplifier, the antenna and IP+ are neutral
  channel fields this cycle added for it (additions spec D8), and every
  other registered radio grades all seven as absent, because their memory
  records carry no such bytes. Nothing in the grid was special-cased to
  achieve that: the column list is derived per bank from each radio's own
  capabilities, so those seven appear on this receiver and on nothing
  else purely because `core/driver/icr8600/caps.go` maps them
  (`app/uispec.go`'s `bankTierFields`, pinned by
  `TestGetUISpec_RegisteredICR8600_EveryBankFieldsAndTagDisplay`). The
  same derivation is why its transmit-frequency and transmitted-tone
  columns are absent — see the receiver note at the top of this file.
- **The IC-R8600 has four possible CI-V terminals and this program uses
  one.** The guide names a `[REMOTE]` jack, a rear `[USB]` port, a front
  `[USB]` port and a `[LAN]` connection, and which of them is live is a
  receiver setting; this program speaks over USB and ships no network
  path (additions spec D3.5 rules LAN CI-V out of scope for this cycle).
  Two further settings have no printed default at all — the transceive
  function, and echo-back, which is set separately for each of the two
  USB ports — so this program cannot tell you whether unsolicited frames
  should be expected from the receiver of its own accord. Any that arrive
  are counted and ignored, never acted on, which is how every model here
  handles them (`core/driver/icr8600/doc.go`, entries
  `icr8600-transceive-default` and `icr8600-echo-default`; matrix §3.5,
  §3.6).
- **The IC-R8600's clear form is printed, and this program still will not
  send it.** Unlike the Yaesu models, this receiver's guide DOES document
  how to clear a memory — a memory-set frame carrying `FF` where the
  record would go — and it even names a limit on it, group 0102 being
  excluded. That changes nothing here: this tier ships no erase builder,
  the outbound gate admits only the identity read, a memory read and a
  re-validated memory set, and the erase field is graded as absent on
  every Icom driver, so the consent transform structurally cannot enable
  it. What the printed form actually does has never been confirmed on a
  receiver, and clearing the wrong channel is not a mistake worth risking
  for a convenience the front panel already offers (matrix §3.13;
  `internal/radiotext`'s `icr8600Text` says the same thing to the user).

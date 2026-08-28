# Icom models — per-model limitations and evidence

Moved verbatim from the README on 28/08/2026 so the README can stay short. Every claim below cites the code that makes it true; the citations are for reviewers and contributors.
## The six Icom models

IC-7610, IC-7300, IC-7300MK2, IC-705, IC-9700 and IC-905 talk a
different wire protocol from the Yaesu models (Icom's CI-V,
rather than Yaesu's CAT), and each carries its own honesty rows beyond
the README's shared "no radio has ever been connected" small print.

Three costs are shared by all six:

- **No `--civ-address` option.** Each driver talks only to its one
  factory CI-V address (98h IC-7610, 94h IC-7300, B6h IC-7300MK2, A4h
  IC-705, A2h IC-9700, ACh IC-905) and there is no setting to change
  it. Two different things can happen when this driver meets a radio
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
  `core/driver/ic705/ic705.go:364`; `core/driver/ic905/ic905.go:495`),
  while IC-7610 and IC-9700 NEVER mint one for any same-address
  collision, by design — they hold no cross-model table and refuse to
  guess an identity they cannot support
  (`core/driver/ic7610/ic7610.go:142-145`;
  `core/driver/ic9700/ic9700.go:325-329`; `core/driver/ic9700/doc.go:391`).
  Even where a driver CAN attribute, one pair defeats it in practice:
  an IC-9700 moved onto the IC-705's A4h address fails the IC-705's
  open as an unattributed address parse error, not as a named
  wrong-radio refusal, because the address-geometry check pre-empts
  the length check that would otherwise have named it
  (`core/civ/tier_test.go:68-80`).
- **The tone picker stays list-driven while every model's tone range
  is numeric (enabler E3).** All six declare a numeric
  `CTCSSToneRange` rather than a fixed tone chart, because their tone
  spans are BCD frequencies, not indices — but the picker widget
  itself was built for a list. The channel grid still shows and
  round-trips tones on every model; only the picker cannot offer them
  (`core/driver/ic7610/caps.go:342-345`; `core/driver/ic9700/caps.go:177-179`;
  same declaration in each of the other four models' own `caps.go`).
- **A channel outside the write gate's template is refused, never
  silently changed (ruling E6).** Every one of the six records carries
  at least one region no `codeplug` field maps, and a slot may be
  written only when those unmapped regions already match the
  profile's template. What that costs differs by model:
  - **IC-7610, IC-705, IC-9700, IC-905**: a channel already in a
    Select scan group (★1/★2/★3) cannot be written by this program at
    all — the SELECT nibble is unmapped and there is no honest value
    to preserve, so the write is refused, naming the reason
    (`core/driver/ic7610/doc.go:220-249`; `core/driver/ic705/write.go:265-280`;
    `core/driver/ic9700/write.go:529-531`; `core/driver/ic905/write.go:578-590`).
    The IC-7610 additionally refuses a channel whose data mode is
    DATA 1/2/3, for the same unmapped-nibble reason
    (`core/driver/ic7610/doc.go:235-238`).
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
  In every case the write is refused, naming the reason, never
  downgraded or cleared.

All six open at 19200 baud by default, but what grades that default
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

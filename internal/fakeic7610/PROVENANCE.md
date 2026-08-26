<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakeic7610` — where every byte came from

## Hardware status: UNVERIFIED

**No IC-7610 has ever been asked anything by this project.** Not by this
package, not by the legs that produced the evidence artefacts it was built
from, not by anything upstream of either. Nothing here has been observed on a
radio. Every claim this package makes is a claim about a printed document, or
about a decision this package took in the absence of one.

That sentence governs everything below and should be read into every row of
every table. A test that passes against this fake proves that a consumer agrees
with **this fake**; it does not prove that either agrees with an IC-7610.

## What was read

Three things, and nothing else.

| what | path | how it was used |
| --- | --- | --- |
| memory-record transcription, leg B | `core/civ/ic7610/testdata/ic7610-transcription-b.md` and `.csv` | the field widths whose sum gives `RecordLen`; the printed channel-selector codes; the memory-name width; the character-table footnote behind `NamePad`; the recorded fact that the page states **no numeric encoding** for the selector codes |
| memory-record geometry witness | `core/civ/ic7610/testdata/ic7610-geometry-witness.md` and `.csv` | an independent confirmation of the same 27-byte total, and the STOP findings about the drawn-cell count that this package does **not** resolve |
| the task's own wire-facts block | quarantined, manual-derived, inlined in this package's brief | the framing skeleton, the OK/NG codes, the addresses, the command surface, the refusals, and each of those facts' grade |

The wire-facts block is itself second-hand: every fact in it was read off the
IC-7610 CI-V Reference Guide rev 4 by other agents on other legs. This package's
author never opened the PDF.

Both evidence artefacts name the same source document — cover title `CI-V
REFERENCE GUIDE`, model `IC-7610`, revision code `A7380-7EX-4`, 17 PDF pages —
and both attest that every value in them was read from that PDF's rendered page
images and from nothing else.

## What was NOT read

The independence rule this package was built under forbade its author from
opening any of the following, and none of them was opened:

- `core/civ/ic7610/*.go` — this radio's own CI-V codec
- `core/driver/ic7610/*.go` — the driver this fake exists to be tested against
- `core/civ/*.go` — the shared CI-V machinery: framing, accumulator, address
  handling, BCD, record codec, profiles
- any golden vector, including `core/civ/ic7610/testdata/ic7610-vectors.golden`
- the other artefacts in that testdata directory that were not on the reading
  list: `ic7610-field-ledger.md`/`.csv`, `ic7610-golden-assumptions.csv`,
  `ic7610-golden-provenance.md`

The directory listings of `core/civ/` and `core/driver/` were seen — file names
only, in the course of confirming the worktree's shape. No file in either was
opened.

`internal/fakedx101/doc.go`, `fakedx101.go`, `imports_test.go` and
`PROVENANCE.md` **were** read, as the **structural** exemplar this package's
brief named: the pipe-and-goroutine shape, the options list, the import fence,
the shape of this document. **No protocol was taken from them.** CAT is ASCII
and semicolon-terminated; CI-V is binary and addressed. They share a shape and
nothing else.

The import fence (`imports_test.go`) makes the code side of that rule
mechanical rather than a matter of good intentions: `TestNoCoreImports` walks
this directory **and every directory beneath it** and fails on any import whose
path begins with this project's module path. It was written and proven green
**before a line of protocol code in this package existed**, so that anything
which arrives here later arrives inside a fence rather than in front of one.

## Every byte this fake invents

**One.** It is stated here in full because a fake that invents quietly is worse
than no fake at all.

### The ID token — INVENTED, lifts nothing

`19 00` is the transceiver-ID command. **The command is manual-evidenced. Its
reply value is not.** The document prints the request — with its Data cell
blank, which is how this package knows the request carries no data bytes — and
prints **no reply value for it anywhere**: not in the command table, not in the
worked example, not in any diagram.

There is therefore no printed value to transcribe and no capture to copy. The
default this package answers with is **`0xA5`, one byte**. Both the value and
the width are invented.

`0xA5` was chosen **because it is obviously synthetic** — an alternating bit
pattern, and not one of the addresses or codes this package names (`0x98`,
`0xE0`, `0x00`, `0xFB`, `0xFA`, `0xFD`, `0xFE`). A plausible-looking default
would be worse: a consumer whose ID probe happened to expect the right value
would pass against a fake that had guessed, and nobody would ever learn the
guess was a guess. This one fails loudly. `WithIDToken` is how a consumer that
has a real value — from hardware or from a capture, neither of which this
project has — supplies one, and it is the only route by which a correct value
can ever get into this package.

**It lifts nothing.** It is not derived from any Icom document, any other
radio's identity, any file in this repository, or any capture.

### No default record contents — nothing invented

`New()` seeds **no channel**. A read of any channel answers `FA` until something
sets it, over the wire or through `SetSlot`. That is a deliberate refusal to
invent: the guide prints no shipped record for any channel, so there is nothing
to model, and a plausible-looking factory image would be a fabrication a
consumer could mistake for evidence. A record's bytes are only ever what a
consumer put there.

Two constants are exported for a consumer building a record, and **this package
writes neither of them anywhere**: `NameLen` = 10 (manual-evidenced — the
`⑱ ~ ㉗` row's `width_bytes`, and the page's own "Up to 10 characters.") and
`NamePad` = `0x20` (**ASSUMED**, and weakly — see the assumption register
below).

## The derived record length

`RecordLen` = **25**. **Derived, not read.** No page prints it.

It is the sum of the `width_bytes` column of the D1 rows of
`ic7610-transcription-b.csv`, less the two selector bytes that CSV counts as its
first field:

| printed index | field | width |
| --- | --- | --- |
| `①, ②` | Memory channel numbers | 2 — **the selector, excluded** |
| `③` | Select memory setting | 1 |
| `④ ~ ⑧` | Operating frequency setting | 5 |
| `⑨, ⑩` | Operating mode setting | 2 |
| `⑪` | Data mode and tone type settings | 1 |
| `⑫ ~ ⑭` | Repeater tone frequency setting | 3 |
| `⑮ ~ ⑰` | Tone squelch frequency setting | 3 |
| `⑱ ~ ㉗` | Memory name settings | 10 |

2+1+5+2+1+3+3+10 = 27; 27 − 2 = **25**.
`TestRecordLengthIsTheSumOfTheD1FieldWidthsLessTheSelector` re-does that
arithmetic in the test, so a change to the constant has to argue with the
numbers rather than merely move them.

The geometry witness agrees independently: its own printed-numbering table
reaches 27, and its last printed index is `㉗` = 27.

### A disagreement this package does not resolve

Both artefacts record that the same strip is **drawn in 18 cells** while its
**printed indices run to 27** — two of the drawn cells are dashed `...`
abbreviation cells. The witness raises this as its **STOP 1, STOP 2 and
STOP 3** and resolves none of them.

This package **follows the printed numbering**, because that is the numbering
the `width_bytes` column records and the only one that yields a count of *bytes*
at all — a drawn-cell count of 18 is a count of pictures. That is a statement of
which printed thing was followed, **not** evidence that 25 is right.
`WithRecordLength` exists precisely because a derivation can be wrong.

## Assumption register

Each of these is a decision this package took where the document does not
speak. None is evidence.

| # | assumption | scope of what backs it |
| --- | --- | --- |
| 1 | The `1A 00 <hi> <lo>` **read request form** exists at all | **Nothing.** The document prints no `1A 00` read request anywhere. The set form is printed; the read form is inferred from it. |
| 2 | An **unwritten memory channel** answers `FA` | One capture, covering **one unwritten memory channel**. The document prints the `FA` code but never says an unwritten channel provokes it. |
| 3 | An **unset P1 or P2** answers `FA` — **see the scope warning below** | **No named capture at all.** |
| 4 | Unsolicited (transceive) frames carry `to` = `00` | **Nothing.** The document prints no broadcast frame; the only answer-direction skeleton it prints has `to` = `E0`. |
| 5 | The memory name's pad byte is `0x20` (space) | Two printed things that disagree: the character tables have **no row for a space at all**, while the same block's footnote lists "(space)" among the usable characters. Unresolved. Nothing in this package writes it. |
| 6 | The channel selector's low byte is read **as printed** — one decimal digit per nibble, so channel 99 is `0x99` | The transcription's own note that the page "prints whole-byte codes against meanings and states no numeric encoding; it does not say BCD or binary". This package reproduces the printed codes rather than choosing an encoding for them. |
| 7 | The **echo sits before the address filter** | **Nothing.** The document says nothing about which side of an address filter an echo sits on. See "Modelling decisions" below. |
| 8 | A frame addressed here but carrying **no command at all** is answered `FA` | **Nothing.** It is addressed to this radio and there is nothing in it to act on, so it is refused rather than ignored; silence is reserved for frames that are not this radio's business. |

### SCOPE WARNING — assumption 3, and it is the widest thing here

Assumption 2's single capture covers **one unwritten MEMORY channel**. It says
**nothing** about the programmed scan edges P1 and P2. And the capture that
covers **P1** says nothing about **P2**.

**This fake answers `FA` for an unset P1 and for an unset P2.** That is
**wider than any capture this project has named** — wider than assumption 2,
and wider again for P2 than for P1. It is asserted here because a fake has to do
*something* when asked for a scan edge it has never been given, and answering
`FA` is at least consistent with what it does for a memory channel. Consistency
is not evidence.

`TestMemoryReadOfAnUnsetChannelAnswersNG` names all three cases separately, and
labels the two scan-edge sub-tests as wider than the capture, so that the
warning is visible at the place it is asserted rather than only here.

## Deliberate divergences from the page

Three wire forms this fake **refuses with `FA`** which a real IC-7610 would very
likely **honour**. Each is a choice to fail loudly rather than act quietly.

| form | what the page prints | what this fake does, and why |
| --- | --- | --- |
| `1A 00 <hi> <lo> FF` | "To clear the memory channel contents on `1A 00`", with `③: "FF"` | **Refused.** So that any code path which ever emits a clear fails in a test rather than silently emptying a channel in a simulator someone is reading. |
| `0B` | "Memory clear" | **Refused**, same reason. |
| `18 01` | the power-ON command; the guide's one worked example frame illustrates its FE-padding | **Refused.** A fake radio has no power state to switch, and answering `FB` would assert one it does not have. |

Both clear forms are matched **explicitly** in `parser.go` rather than left to
fall through the record-length check, so that each refusal is a decision at a
named place and not a by-product of arithmetic. `TestTheFiveRefusedFormsAnswerNG`
asserts, for each, that the channel is **still there** afterwards.

`ClearSlot` is the Go-side control the wire deliberately does not offer: a test
that needs a channel empty says so in Go, where the statement is visible, rather
than by sending a byte sequence.

**`1A 05` is also refused, and that is not a divergence.** It is the menu
surface this tier does not ship, and refusing it is what the tier means.

## Modelling decisions

Places where this package had to choose and the document does not say. They are
not assumptions about a radio's behaviour so much as decisions about what this
simulator is; each is stated at the code that implements it as well as here.

- **The echo runs before the address filter.** `WithUSBEcho` echoes every
  received frame verbatim, including one addressed to another radio, which is
  then ignored — echo, and no answer, and no state change, and no `CommandLog`
  entry. The reasoning: an echo is a property of the **line** (a USB codec or a
  bus reflecting what was put on it), and answering is a property of the
  **radio**. This is the one place "a frame addressed elsewhere is ignored
  entirely" and "echo every frame verbatim" could be read as contradicting each
  other, and a consumer needs to know which wins.
  (`TestWithUSBEchoEchoesAFrameAddressedElsewhereAndDoesNotAnswerIt`.)
- **`WithLatency` delays answers only** — not echoes and not flood frames. A
  reflection is not a reply, and a transceive frame is not one either.
- **The controller-addressed flood is synthetic.** `WithAddressedFlood` /
  `StartAddressedFlood` emit frames addressed to `E0` as though the radio were
  answering continuously and would not stop. **The document describes no radio
  doing this**, and this package does not claim one does. It exists because a
  consumer that must survive a jabbering peer has to be shown surviving one, and
  because it is the line condition a broadcast flood is most easily confused
  with. The broadcast flood's `to` = `00` is assumption 4 above; the addressed
  flood's `to` = `E0` is not an assumption about a radio at all.
- **The flood frame is the ID answer with its `to` byte swapped** —
  `FE FE <to> 98 19 00 <token> FD`. So the two floods differ from each other in
  exactly the one byte that distinguishes the two line conditions, and neither
  invents a command this radio does not otherwise answer.
- **Data bytes are not escaped.** The first `FD` after the address pair ends the
  frame, and an `FE FE` pair inside a payload is indistinguishable from a
  preamble. A record or an ID token containing `FD` or `FE` will truncate or
  resynchronise the frame carrying it. That is the protocol as printed; this
  package deliberately does not paper over it, and says so at `SetSlot` and at
  `WithIDToken`.
- **A command with no sub-command logs a zero sub-command.** `0B` and a
  hypothetical `0B 00` would appear alike in `CommandLog`. Accepted rather than
  papered over: the alternative is a wider type for a distinction no command in
  this package's surface needs.
- **An unframable stream is dropped silently** once the accumulator passes its
  bound. CI-V has no rejection code for "I could not find a frame", and
  inventing one — an `FA` addressed to a controller this radio has not heard
  from — would put a frame on the wire that no radio would send.

## What this fake deliberately does not model

- **Anything outside the memory surface and `19 00`.** No menu, no frequency, no
  mode, no scan, no power state, no transceive negotiation. Everything else is
  `FA`.
- **The meaning of any record byte.** `MemState.Raw` is uninterpreted. This
  package parses no field of a record and knows nothing of what any byte means;
  the field layout that gives the record its length lives in `doc.go` and in
  this file, as arithmetic over an evidence artefact, and nowhere in the code.
- **Whether a record of `FF` bytes means "empty".** Undocumented, and **this
  fake does not decide it**. A record of `FF` bytes is stored and returned like
  any other. "Unset" here means *never set, or `ClearSlot`'d* — a property of the
  fake's map, not of the bytes in a record.
- **Faults.** No dropped replies, garbled replies, spurious frames or scripted
  disconnects. The floods and `WithLatency` are line conditions, not faults, and
  the fault surface that exists for the FT-710 exercises transport machinery that
  is model-independent.
- **Timing.** No IC-7610 timing has ever been observed by this project, so every
  reply is near-instant unless `WithLatency` says otherwise, and that option
  models nothing real.

## Licence

GPL-3.0-or-later, as the whole repository is (`LICENSE`); this note carries the
SPDX identifier in an HTML comment, the convention the repository's other
Markdown documents use.

The underlying document is Icom Inc.'s IC-7610 CI-V Reference Guide, revision
code `A7380-7EX-4`. The manual PDF is **not** in this repository
(`docs/fixtures-private/manuals/`, gitignored). What is committed here is an
independent implementation of factual protocol data — frame shapes, command
codes and field widths — for interoperability.

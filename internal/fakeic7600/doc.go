// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7600 simulates an Icom IC-7600's CI-V behaviour over an
// in-memory duplex connection (Radio.Port()). It is the test double this
// radio's own layers are meant to run against, the role internal/fakeic7610
// plays for the IC-7610 and internal/fakeic7851 for the IC-7851/IC-7850.
//
// # NO IC-7600 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
//
// Everything below is UNVERIFIED against hardware (matrix §0, §3.14 —
// writeTrialsComplete is FALSE and stays FALSE). It was built from
// docs/superpowers/icom-matrices/ic7600-capability-matrix.md alone, a
// document the matrix's own author wrote from the IC-7600 Instruction
// Manual, Section 12 ("CONTROL COMMAND" — this model has no separate CI-V
// Reference Guide), and never opened core/civ/ic7600 or core/driver/ic7600
// to write it. Every claim this package makes is a claim about that matrix,
// not about a transceiver.
//
// # THE QUARANTINE
//
// This package was authored independently of the production IC-7600 codec
// and driver: their .go files were never opened while writing this one, and
// TestNoCoreImports (imports_test.go, copied from fakeic7851's own) makes
// the fence mechanical rather than a matter of good intentions. It covers
// this directory and every directory beneath it. The only project-internal
// import permitted is internal/fakepipe, which is PROTOCOL-FREE (a net.Pipe
// pair, goroutine bookkeeping, an interruptible latency wait, a raw write —
// no framing, no field layout, no reply). A systematic bug in the production
// codec — an off-by-one field offset, a validation rule subtly wrong — would
// be invisible to a test that checked the codec against itself; two
// independent implementations checked against each other is what makes that
// class of bug visible.
//
// internal/fakeic7851 is this package's structural SHAPE exemplar (the pipe,
// the goroutine, the options list, the address/flood shape);
// internal/fakeic7610 additionally supplies the 25-byte record-length
// arithmetic this radio happens to share, and internal/fakeic7800 is a
// sibling built to the identical driver shape (spec.md §1: "same as
// IC-7800 — copy ic7610, narrow two enums, change address"). Neither
// package's PROTOCOL facts — addresses, command bytes, reply codes — were
// carried across: every wire fact below is cited to the IC-7600 matrix,
// independently.
//
// # Framing
//
// MANUAL-EVIDENCED, matrix §3.4 (address) and §3.10 (frame skeleton):
//
//	request  FE FE 7A E0 <cn> [<sc>] <data...> FD
//	answer   FE FE E0 7A <cn> [<sc>] <data...> FD
//	OK       FE FE E0 7A FB FD
//	NG       FE FE E0 7A FA FD
//
// 0x7A (AddrRadio) is the IC-7600's default address and 0xE0
// (AddrController) the controller's — matrix §3.4, PDF p.151 (folio 142),
// "The IC-7600's address is 7Ah", corroborated PDF p.168 (folio 159) frame
// diagram. A frame whose `to` byte is not 0x7A is not for this radio and is
// answered with silence: no reply, no state change, no CommandLog entry —
// TestOnlyThisRadioIsAnswered.
//
// Preamble padding, line noise before the first FE FE run, and the
// unescaped-data-bytes truncation-on-interior-FD rule are all reproduced
// exactly as internal/fakeic7610 documents them (its own doc.go, "Framing"):
// this project treats CI-V framing as a family-wide convention, evidenced
// once per model at the address/OK/NG level and shared in shape thereafter.
// parser.go's reassembler is nonetheless its own independent implementation,
// not a shared call — see THE QUARANTINE above.
//
// The front panel's CI-V Baud Rate item (matrix §1 row 13, PDF p.151 folio
// 142) lists 300, 1200, 4800, 9600, 19200 bps plus "Auto" as the choice set.
// This package, like every fake in this wave, has nothing to simulate here:
// net.Pipe carries bytes, not a bit rate, so no Option models a baud — the
// figures are recorded here only so a reader hunting for them does not
// conclude they were missed.
//
// # What this radio answers, and what it refuses
//
//   - 19 00              the transceiver-ID answer. MANUAL-EVIDENCED as a
//     command (matrix §3.12(i), PDF p.170/folio 161: "Read the transceiver
//     ID", Data cell blank). ITS REPLY VALUE IS NOT PRINTED ANYWHERE. The
//     token this fake answers with is INVENTED — see defaultIDToken in
//     options.go — and lifts nothing (register FAKE-1).
//   - 1A 00 <hi> <lo>    read one memory record. ASSUMED wire form (matrix
//     §3.7, register D5-1: the document prints no 1A 00 read request at
//     all, only the set form). Answers the stored record at RecordLen
//     bytes, or NG if that channel has never been set (ASSUMED, matrix
//     §3.8(a), register D5-2a) — or, with WithAllFFEmpty, a record of 0xFF
//     bytes: matrix §3.8(b) grades that reading as equally undocumented,
//     so this package offers it as the alternative rather than picking
//     silently.
//   - 1A 00 <hi> <lo> <RecordLen bytes>
//     set one memory record. Answers OK. ASSUMED that a 1A 00 set is
//     acknowledged with FB/FA at all (matrix §3.10, register
//     ic7600-1a00-set-ack: no 1A 00-specific acknowledgement statement was
//     found).
//
// Refused, each on purpose:
//
//   - 1A 00 <hi> <lo> FF      the clear form the page prints (matrix §3.13,
//     PDF p.178: "To program the blank channel, enter 'FF' to ③ after the
//     memory channel number"). Refused with NG by default so a code path
//     that ever emits a clear fails loudly in a test rather than silently
//     emptying a simulated channel — same deliberate divergence
//     internal/fakeic7610 and internal/fakeic7800 record for their own
//     documents. This tier ships no erase surface (FieldErase carries the
//     zero FieldSupport — matrix §2 row 10).
//   - 0B                      "Memory clear" (matrix §3.13, PDF p.169/folio
//     160, command 0B). Refused, same reason.
//   - 1A 05 <anything>        the menu surface this tier does not ship
//     (matrix §3.3/§3.4/§3.5 all cite 1A 05 sub-commands for baud/address/
//     transceive-toggle settings, none of which this fake exposes).
//     Refusing it is what the tier means, not a divergence.
//   - 18 01                   power ON. Not printed as a command this
//     matrix cites for this radio, but refused anyway: a fake radio has no
//     power state to switch, and answering OK would assert one it does
//     not have. core/civ/ic7600/testdata/ic7600-vectors.golden's
//     "manual-example-14" is a padded 18 01 frame used to prove preamble
//     tolerance, not a claim that this model's own manual prints that
//     worked example — see that file's golden-provenance.md
//     ("inherited_assumed", carried from the IC-7610 exemplar's own vector
//     of the same name).
//
// # Channel selectors
//
// MANUAL-EVIDENCED, matrix §1 row 5 and §1b "The banks", PDF p.169's
// command-table rows and PDF p.178's `①,② Memory channel number` legend:
//
//	00 01 .. 00 99   memory channels 1..99
//	01 00            programmed scan edge P1
//	01 01            programmed scan edge P2
//
// Packed two decimal digits per byte, the identical shape
// internal/fakeic7610 and internal/fakeic7800 reproduce for their own
// documents. Anything outside those three forms addresses nothing and is
// refused with NG. No CALL bank and no group-addressed slot form exist on
// this model (matrix §1b) — there is nothing else to select.
//
// # Record length
//
// RecordLen is 25, DERIVED — not printed anywhere (matrix §3.11: "the
// manual prints no byte-count or '25'/'27' length statement"). It is the
// sum of the matrix §3.11 term table:
//
//	③        1   select memory setting
//	④–⑧     5   operating frequency setting
//	⑨,⑩     2   operating mode and filter setting
//	⑪        1   data mode and tone-type setting (nibble pair)
//	⑫–⑭     3   repeater tone frequency setting
//	⑮–⑰     3   tone squelch frequency setting
//	⑱–㉗    10   memory name setting
//
// 1+5+2+1+3+3+10 = 25, excluding the two channel-selector bytes (①,②),
// which the matrix counts separately (its own 27-byte "data area" total,
// matching core/civ/ic7600/testdata/ic7600-geometry-witness.md exactly).
//
// Byte ③ (select memory setting) is printed as ONE UNDIVIDED enum cell on
// this radio's own page — matrix §2 row 9, §3.15(a): unlike the IC-7610's
// diagram, which splits the byte into a low-nibble group marker and a
// stated Fixed-0 high nibble, the IC-7600's page states no nibble split and
// no high-nibble constraint at all. MemState.Raw is UNINTERPRETED, exactly
// as internal/fakeic7610's and internal/fakeic7800's own MemState: this
// package parses no field of a record and knows nothing of what any byte
// means, so this divergence — real for a driver's codec — makes no
// difference to what this fake stores or returns.
//
// Byte ⑪'s two nibbles carry data_mode and tone_mode — matrix §1 row 21,
// §2 rows 14/20: LOW nibble is tone_mode (0 OFF / 1 TONE / 2 TSQL, no
// DTCS), HIGH nibble is data_mode (0 OFF / 1 DATA1 / 2 DATA2 / 3 DATA3).
// core/civ/ic7600/testdata/ic7600-golden-assumptions.csv's own worked
// example confirms the byte value 0x01 = "low nibble TONE (0x1), high
// nibble 0x0 (data mode OFF)" — this is the OPPOSITE nibble assignment
// from internal/fakeic7800's own document, which reads tone-type as the
// high nibble on its page. Recorded here because a reader comparing the
// two sibling fakes' test fixtures would otherwise take the difference for
// a mistake; again, invisible to this package either way, since Raw is
// never parsed.
//
// # Name length and pad byte
//
// NameLen is 10 (MANUAL-EVIDENCED, matrix §1 row 7 / TagLen). NamePad is
// 0x20 (ASSUMED, matrix §3.9(iv): "No statement anywhere of what a
// controller sends... for a name shorter than ten characters"). NOTHING IN
// THIS PACKAGE WRITES EITHER: New seeds no channel, and a record's bytes
// are only ever what a consumer supplied.
//
// The matrix's own §1 TagCharset entry records a printed contradiction
// (two character tables covering only lowercase letters and symbols,
// against an applicability line reading "All characters are available")
// and resolves it as a CHOICE, not a fact — this package takes no position
// on the charset either, because MemState.Raw is bytes, never characters.
//
// # Empty channels
//
// A channel never SetSlot'd or written over the wire answers NG by default
// — ASSUMED, matrix §3.8(a), from a single capture this project has never
// made. WithAllFFEmpty switches the default to answering an all-0xFF
// record instead, modelling the alternative reading matrix §3.8(b) grades
// as equally open, rather than this package silently preferring one.
// Neither option decides what a record READ BACK as all-0xFF, because a
// consumer actually stored it, means — that question stays open regardless
// of which empty-reply mode is in force, same as internal/fakeic7610's and
// internal/fakeic7800's own packages.
//
// This fake answers the same way for an unset P1 or P2 as for an unset
// memory channel. That is WIDER than the single capture register D5-2a
// names (which says nothing about either scan edge) — asserted only
// because a fake has to answer *something*, and consistency with the
// memory-channel case is not evidence for it.
//
// # No echo option — a deliberate omission, unlike its siblings
//
// This package offers NO WithUSBEcho (or any echo) option, and dispatch is
// the only place that says so — the underlying reason is already on the
// record at matrix §3.6: this radio's document names no USB-echo-back
// setting at all (the only two "echo" hits in the whole manual are audio
// monitor effects, unrelated to CI-V), and unlike internal/fakeic7800's own
// FAKE-4 choice to add a synthetic echo knob anyway ("a consumer proving
// the accumulator's echo-removal path needs a line that echoes"), this
// package does not duplicate that synthetic surface here: the
// accumulator's echo removal is structural (NoteSent + drop-first-match,
// matrix §3.6's own conclusion) and is already exercised against the
// siblings that do carry the option. No register entry: there is nothing
// on this model's own document to lift, same conclusion matrix §3.6 itself
// records.
//
// # Transceive: two floods, and they are not the same
//
// WithTransceiveFlood starts a BROADCAST flood, `to` = 0x00 — ASSUMED
// (matrix §3.5(b): "the only answer-direction skeleton this document
// prints... shows to = E0"; no broadcast frame is printed at all).
// WithAddressedFlood starts a CONTROLLER-ADDRESSED flood, `to` = 0xE0 — a
// SYNTHETIC line condition the document describes no radio producing,
// included so a consumer that must survive a jabbering peer is shown to.
// Both emit the ID answer with `to` swapped, so the two floods differ from
// each other in exactly the byte under test and neither invents a command
// this radio does not otherwise answer.
//
// # Short-set handling
//
// The default and only wire behaviour: a 1A 00 set whose record is not
// exactly RecordLen bytes is refused with NG (matrix §3.10, "Whether the
// full record is mandatory on write: ASSUMED"). WithShortSetAccepted
// switches a set of FEWER than RecordLen bytes to being accepted and
// stored zero-padded on the tail (ASSUMED padding, register FAKE-3, which
// this package opens as its own register entry rather than reusing a
// shared one, because no matrix entry names a pad convention for the
// accepted case at all). A LONGER-than-RecordLen set is always refused, in
// both modes: nothing in the matrix suggests a radio would truncate rather
// than reject an overlong write.
//
// # Concurrency and the pipe
//
// A Radio is safe for concurrent use — SetSlot, SlotState, ClearSlot,
// CommandLog, BytesWritten, the flood controls and Close may all be called
// from goroutines other than whatever is reading or writing Port(). Run
// tests with -race. Port() is one end of a net.Pipe, unbuffered; every byte
// the radio sends goes through a bounded output queue drained by one writer
// goroutine, so a flood and an answer can never interleave mid-frame, and a
// flood a consumer never reads drops its oldest frames rather than growing
// without limit or wedging the reader. This shape is internal/fakeic7610's
// own outQueue, reproduced here for the same reason it exists there:
// fakepipe.Pipe's unbuffered Write would otherwise deadlock a flood against
// an unread port.
//
// # This package's own register
//
// Numbered independently of the matrix's own register entries (which this
// package cites throughout above), because these are decisions THIS
// package took that the matrix does not itself register:
//
//	FAKE-1   defaultIDToken = 0xA6, one byte, INVENTED, lifts nothing
//	         (see options.go). Chosen for the same reason
//	         internal/fakeic7610's own 0xA5 and internal/fakeic7800's own
//	         0x5A were: a bit pattern that is none of this package's own
//	         reserved bytes (0x7A, 0xE0, 0x00, 0xFB, 0xFA, 0xFD, 0xFE), so
//	         a consumer whose probe happens to match it learns nothing.
//	FAKE-2   WithAllFFEmpty's all-FF record answers matrix §3.8(a)'s
//	         alternative (0xFF-filled instead of NG); matrix §3.8(b)
//	         grades the underlying question and this package does not
//	         resolve it, only offers both readings.
//	FAKE-3   WithShortSetAccepted's zero-tail padding: no matrix entry
//	         names a pad byte for an accepted short set, because the
//	         matrix does not grade short sets as accepted at all. 0x00 was
//	         chosen as the least meaningful value available, matching no
//	         printed enum's own zero-is-OFF convention by coincidence
//	         rather than citation.
package fakeic7600

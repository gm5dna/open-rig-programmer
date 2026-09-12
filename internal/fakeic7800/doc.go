// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7800 simulates an Icom IC-7800's CI-V behaviour over an
// in-memory duplex connection (Radio.Port()). It is the test double this
// radio's own layers are meant to run against, the role internal/fakeic7610
// plays for the IC-7610 and internal/fakeic7851 for the IC-7851/IC-7850.
//
// # NO IC-7800 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
//
// Everything below is UNVERIFIED against hardware (matrix §0, §3.14 —
// writeTrialsComplete is FALSE and stays FALSE). It was built from
// docs/superpowers/icom-matrices/ic7800-capability-matrix.md alone, a
// document the matrix's own author wrote from the IC-7800 Instruction
// Manual, Section 14, and never opened core/civ/ic7800 or core/driver/ic7800
// to write it. Every claim this package makes is a claim about that matrix,
// not about a transceiver.
//
// # THE QUARANTINE
//
// This package was authored independently of the production IC-7800 codec
// and driver: their .go files were never opened while writing this one, and
// TestNoCoreImports (imports_test.go, copied from fakeic7851's own, itself
// copied from internal/fakeicr8600's — see that file) makes the fence
// mechanical rather than a matter of good intentions. It covers this
// directory and every directory beneath it. The only project-internal import
// permitted is internal/fakepipe, which is PROTOCOL-FREE (a net.Pipe pair,
// goroutine bookkeeping, an interruptible latency wait, a raw write — no
// framing, no field layout, no reply). A systematic bug in the production
// codec — an off-by-one field offset, a validation rule subtly wrong — would
// be invisible to a test that checked the codec against itself; two
// independent implementations checked against each other is what makes that
// class of bug visible.
//
// internal/fakeic7851 is this package's structural SHAPE exemplar (the pipe,
// the goroutine, the options list, the address/echo/flood shape);
// internal/fakeic7610 additionally supplies the 25-byte record-length
// arithmetic this radio happens to share. Neither package's PROTOCOL facts —
// addresses, command bytes, reply codes — were carried across: every wire
// fact below is cited to the IC-7800 matrix, independently.
//
// # Framing
//
// MANUAL-EVIDENCED, matrix §3.4 (address) and §3.10 (frame skeleton):
//
//	request  FE FE 6A E0 <cn> [<sc>] <data...> FD
//	answer   FE FE E0 6A <cn> [<sc>] <data...> FD
//	OK       FE FE E0 6A FB FD
//	NG       FE FE E0 6A FA FD
//
// 0x6A (AddrRadio) is the IC-7800's default address and 0xE0
// (AddrController) the controller's — matrix §3.4, PDF p.200 (folio 14-2)
// and p.177 (folio 12-21): "The IC-7800's address is 6Ah." A frame whose
// `to` byte is not 0x6A is not for this radio and is answered with silence:
// no reply, no state change, no CommandLog entry — TestOnlyThisRadioIsAnswered.
//
// Preamble padding, line noise before the first FE FE run, and the
// unescaped-data-bytes truncation-on-interior-FD rule are all reproduced
// exactly as internal/fakeic7610 documents them (its own doc.go, "Framing"):
// this project treats CI-V framing as a family-wide convention, evidenced
// once per model at the address/OK/NG level and shared in shape thereafter.
// parser.go's reassembler is nonetheless its own independent implementation,
// not a shared call — see THE QUARANTINE above.
//
// # What this radio answers, and what it refuses
//
//   - 19 00              the transceiver-ID answer. MANUAL-EVIDENCED as a
//     command (matrix §3.12(i), PDF p.202/folio 14-4: "Read the
//     transceiver ID", Data cell blank). ITS REPLY VALUE IS NOT PRINTED
//     ANYWHERE. The token this fake answers with is INVENTED — see
//     defaultIDToken in options.go — and lifts nothing (register 7800-R7).
//   - 1A 00 <hi> <lo>    read one memory record. ASSUMED wire form (matrix
//     §3.7, register 7800-R1: the document prints no 1A 00 read request at
//     all, only the set form). Answers the stored record at RecordLen
//     bytes, or NG if that channel has never been set (ASSUMED, matrix
//     §3.8(a), register 7800-R2a) — or, with WithAllFFEmpty, a record of
//     0xFF bytes: matrix §3.8(b) grades that reading as equally
//     undocumented ("unresolved either way", register 7800-R2b), so this
//     package offers it as the alternative rather than picking silently.
//   - 1A 00 <hi> <lo> <RecordLen bytes>
//     set one memory record. Answers OK. ASSUMED that a 1A 00 set is
//     acknowledged with FB/FA at all (matrix §3.10, register 7800-R14: no
//     1A 00-specific acknowledgement statement was found).
//
// Refused, each on purpose:
//
//   - 1A 00 <hi> <lo> FF      the clear form the page prints (matrix §3.13,
//     PDF p.211: "add the code 'FF' after the memory channel number").
//     Refused with NG by default so a code path that ever emits a clear
//     fails loudly in a test rather than silently emptying a simulated
//     channel — same deliberate divergence internal/fakeic7610 records for
//     its own document. This tier ships no erase surface (FieldErase
//     carries the zero FieldSupport — matrix §2 row 10).
//   - 0B                      "Memory clear" (matrix §3.13, PDF p.201/folio
//     14-3, command 0B). Refused, same reason.
//   - 1A 05 <anything>        the menu surface this tier does not ship
//     (matrix §3.16(f), PDF p.202: "1A 01"/"1A 02" family out of scope;
//     1A 05 is the same family). Refusing it is what the tier means, not a
//     divergence.
//   - 18 01                   power ON. Not printed in the IC-7800's own
//     command-table excerpt this matrix cites, but refused anyway: a fake
//     radio has no power state to switch, and answering OK would assert
//     one it does not have. core/civ/ic7800/testdata/ic7800-vectors.golden's
//     "manual-example-14" is a padded 18 01 frame used to prove preamble
//     tolerance, not a claim that this model's own manual prints that
//     worked example — see that file's golden-provenance.md.
//
// # Channel selectors
//
// MANUAL-EVIDENCED, matrix §1 row 5 and §1b "The banks", PDF p.211's
// `q, w Memory channel number` legend:
//
//	00 01 .. 00 99   memory channels 1..99
//	01 00            programmed scan edge P1
//	01 01            programmed scan edge P2
//
// Packed two decimal digits per byte (transcription CSV, field "q,w",
// encoding "bcd_packed"), the identical shape and identical range
// internal/fakeic7610 reproduces for its own document. Anything outside
// those three forms addresses nothing and is refused with NG. No CALL bank
// and no group-addressed slot form exist on this model (matrix §1b) — there
// is nothing else to select.
//
// # Record length
//
// RecordLen is 25, DERIVED — not printed anywhere (matrix §3.11: "No record
// byte-count and no length statement of any kind is printed for 1A 00
// anywhere in Section 14"). It is the sum of the matrix §3.11 term table:
//
//	e         1   select memory setting
//	r-i       5   operating frequency
//	o,!0      2   operating mode and filter
//	!1        1   data mode and tone type (nibble pair)
//	!2-!4     3   repeater tone frequency
//	!5-!7     3   tone squelch frequency
//	!8-@7    10   memory name
//
// 1+5+2+1+3+3+10 = 25, excluding the two channel-selector bytes (q, w),
// which the matrix counts separately (its own 27-byte "data area" total).
// Register 7800-R6 carries the derivation as ASSUMED, and register 7800-R5
// carries the wire order (assumed to equal the printed field order, there
// being no duplicated TX block to reorder around it). This is the one
// figure this package took from internal/fakeic7610's own arithmetic as a
// STRUCTURAL cue (both radios' records happen to sum to the same total) —
// the number itself is independently re-derived above from the IC-7800
// matrix's own table, not copied.
//
// MemState.Raw is UNINTERPRETED, exactly as internal/fakeic7610's own
// MemState: this package parses no field of a record and knows nothing of
// what any byte means, including the byte-8 nibble swap the matrix records
// against the IC-7610 family (§1b: this model's `!1` high nibble is
// tone-type/`tone_mode`, low nibble is `data_mode` — the reverse of the
// IC-7610's own `(11)` byte). That swap matters to a driver's codec; it is
// invisible to this package, which only ever stores and returns whatever
// bytes it was given.
//
// # Name length and pad byte
//
// NameLen is 10 (MANUAL-EVIDENCED, matrix §1 row 7 / §3.9(i): "Up to 10
// characters"). NamePad is 0x20 (ASSUMED, matrix §3.9(iv), register
// 7800-R4: "the document nowhere states what a controller sends... for a
// name shorter than ten characters"). NOTHING IN THIS PACKAGE WRITES
// EITHER: New seeds no channel, and a record's bytes are only ever what a
// consumer supplied.
//
// # Empty channels
//
// A channel never SetSlot'd or written over the wire answers NG by default
// — ASSUMED, matrix §3.8(a), register 7800-R2a, from a single capture this
// project has never made. WithAllFFEmpty switches the default to answering
// an all-0xFF record instead, modelling the alternative reading matrix
// §3.8(b) grades as equally open ("unresolved either way", register
// 7800-R2b) rather than this package silently preferring one. Neither
// option decides what a record READ BACK as all-0xFF, because a consumer
// actually stored it, means — that question stays open regardless of which
// empty-reply mode is in force, same as internal/fakeic7610's own package.
//
// This fake answers the same way for an unset P1 or P2 as for an unset
// memory channel. That is WIDER than the single capture register 7800-R2a
// names (which says nothing about either scan edge) — asserted only because
// a fake has to answer *something*, and consistency with the memory-channel
// case is not evidence for it.
//
// # Echo
//
// WithUSBEcho makes the radio echo every received frame back verbatim
// before any answer to it, running BEFORE the address filter (a frame
// addressed to another radio is echoed, then ignored). Unlike the IC-7610
// and IC-7851/IC-7850, THIS RADIO'S DOCUMENT NAMES NO ECHO SETTING AT ALL
// (matrix §3.6, MANUAL-EVIDENCED absence): its [REMOTE] jack is a
// 2-conductor 3.5 mm mini-jack, not a USB CI-V port, and no "echo" hit in
// the whole 230-page sweep is about a serial protocol. This option therefore
// models no named menu item — it exists only because a consumer proving the
// accumulator's echo-removal path needs a line that echoes, and the matrix's
// own §3.6 conclusion is that the mechanism is structural and
// model-independent, not gated by a per-model setting this radio happens to
// lack.
//
// # Transceive: two floods, and they are not the same
//
// WithTransceiveFlood starts a BROADCAST flood, `to` = 0x00 — ASSUMED
// (matrix §3.5(b), register 7800-R9: "the only answer-direction skeleton
// this document prints... shows to = E0"; no broadcast frame is printed at
// all). WithAddressedFlood starts a CONTROLLER-ADDRESSED flood, `to` = 0xE0
// — a SYNTHETIC line condition the document describes no radio producing,
// included so a consumer that must survive a jabbering peer is shown to.
// Both emit the ID answer with `to` swapped, so the two floods differ from
// each other in exactly the byte under test and neither invents a command
// this radio does not otherwise answer.
//
// # Short-set handling
//
// The default and only wire behaviour: a 1A 00 set whose record is not
// exactly RecordLen bytes is refused with NG (matrix §3.10, "Whether the
// full record is mandatory on write: ASSUMED... The driver always sends the
// full 25 bytes"). WithShortSetAccepted switches a set of FEWER than
// RecordLen bytes to being accepted and stored zero-padded on the tail
// (ASSUMED padding, register 7800-R15, which this package opens as its own
// register entry rather than reusing one of the shared D5 numbers, because
// no other model's matrix grades this exact question the same way) — the
// alternative reading register 7800-R15 leaves open. A LONGER-than-RecordLen
// set is always refused, in both modes: nothing in the matrix suggests a
// radio would truncate rather than reject an overlong write.
//
// # Concurrency and the pipe
//
// A Radio is safe for concurrent use — SetSlot, SlotState, ClearSlot,
// CommandLog, BytesWritten, the flood controls and Close may all be called
// from goroutines other than whatever is reading or writing Port(). Run
// tests with -race. Port() is one end of a net.Pipe, unbuffered; every byte
// the radio sends goes through a bounded output queue drained by one writer
// goroutine, so a flood, an echo and an answer can never interleave
// mid-frame, and a flood a consumer never reads drops its oldest frames
// rather than growing without limit or wedging the reader. This shape is
// internal/fakeic7610's own outQueue, reproduced here for the same reason it
// exists there: fakepipe.Pipe's unbuffered Write would otherwise deadlock a
// flood against an unread port.
//
// # This package's own register
//
// Numbered independently of the matrix's own 7800-R numbers (which this
// package cites throughout above), because these are decisions THIS
// package took that the matrix does not itself register:
//
//	FAKE-1   defaultIDToken = 0x5A, one byte, INVENTED, lifts nothing
//	         (see options.go). Chosen for the same reason
//	         internal/fakeic7610's own 0xA5 was: an alternating bit
//	         pattern that is none of this package's own reserved bytes
//	         (0x6A, 0xE0, 0x00, 0xFB, 0xFA, 0xFD, 0xFE), so a consumer
//	         whose probe happens to match it learns nothing.
//	FAKE-2   WithAllFFEmpty's all-FF record answers matrix §3.8(a)'s
//	         alternative (0xFF-filled instead of NG); register 7800-R2b
//	         grades the underlying question and this package does not
//	         resolve it, only offers both readings.
//	FAKE-3   WithShortSetAccepted's zero-tail padding: no matrix entry
//	         names a pad byte for an accepted short set, because the
//	         matrix does not grade short sets as accepted at all (register
//	         7800-R15 is "whether the full record is mandatory", not "what
//	         pads a short one"). 0x00 was chosen as the least meaningful
//	         value available, matching no printed enum's own zero-is-OFF
//	         convention by coincidence rather than citation.
//	FAKE-4   The echo option models no named setting (see "Echo" above) —
//	         its existence at all, on a radio whose document names no echo
//	         item, is this package's own choice, not the matrix's.
package fakeic7800

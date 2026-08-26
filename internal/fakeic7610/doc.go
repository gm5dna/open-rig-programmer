// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7610 simulates an Icom IC-7610's CI-V behaviour over an
// in-memory duplex connection (Radio.Port()). It is the test double this
// radio's own layers are meant to run against — the transport engine, the
// IC-7610 driver, a --fake CLI mode, a GUI demo mode — the role
// internal/fakeradio plays for the FT-710 and internal/fakedx101 for the
// FTdx101 pair.
//
// internal/fakedx101 is this package's STRUCTURAL exemplar and nothing more.
// The two share a shape — a pipe, a servicing goroutine, an independent frame
// reassembler, an options list, a slot map — and share no protocol whatever.
// CAT is ASCII and terminated by a semicolon; CI-V is binary, addressed, and
// terminated by 0xFD. Nothing in this file was reasoned from that package's
// wire behaviour and nothing should be read across.
//
// # NO IC-7610 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
//
// Everything below is UNVERIFIED against hardware. Nothing in this package has
// been observed on a radio; it was built from a printed page and from two
// evidence artefacts transcribed off that page, and every claim it makes is
// therefore a claim about a document, not about a transceiver. PROVENANCE.md
// records what was read, what was refused, what is assumed and what is
// invented, one item at a time. Read it before trusting a byte of this.
//
// # The hard rule: NOTHING project-internal
//
// fakeic7610 MUST NOT import any package of this project — not core/civ, not
// core/civ/ic7610, not core/driver/ic7610, not core/codeplug, not core/spec,
// not internal/fakedx101, not internal/fakeradio, not internal/fakedx10.
// Standard library only, in every non-test file, in this directory AND every
// directory beneath it. TestNoCoreImports (imports_test.go) enforces it with a
// recursive go/parser scan, and that file was written and proven green BEFORE
// a line of protocol code in this package existed.
//
// The reasoning is internal/fakeradio's, and it is as true here as there: if
// this fake reused the production codec, a systematic bug in that codec — an
// off-by-one in a field offset, a validation rule subtly wrong, a channel
// selector decoded the wrong way round — would be applied identically on both
// sides of every "send a command, check the reply" test this project runs. The
// bug would never surface. The fake would misbehave in exactly the way the
// buggy codec expects, and every end-to-end test would pass anyway. Two
// independent implementations of one protocol, checked against each other, is
// what makes that class of bug visible.
//
// The rule bound the AUTHOR of this package as well as its imports. It was
// written by an agent that was forbidden to open core/civ/ic7610, core/driver/
// ic7610, core/civ or any golden vector, and did not. Its only inputs were the
// two evidence artefacts named in PROVENANCE.md and a quarantined block of
// wire facts read off the manual by other agents.
//
// # Framing
//
// MANUAL-EVIDENCED, from the guide's "About the data format" page:
//
//	request  FE FE 98 E0 <cn> [<sc>] <data...> FD
//	answer   FE FE E0 98 <cn> [<sc>] <data...> FD
//	OK       FE FE E0 98 FB FD
//	NG       FE FE E0 98 FA FD
//
// 0x98 (AddrRadio) is the transceiver's default address and 0xE0
// (AddrController) the controller's. A frame whose `to` byte is not 0x98 is
// not for this radio and is IGNORED ENTIRELY — no answer, no state change, no
// entry in CommandLog. That silence is what makes a driver's own address
// filter provable: a driver that answers frames addressed elsewhere has a bug
// this fake will not paper over.
//
// Extra leading 0xFE bytes are preamble padding and carry no meaning; a run of
// two or more begins a frame and the whole run is skipped. Bytes before the
// first such run are line noise and are discarded silently.
//
// DATA BYTES ARE NOT ESCAPED, exactly as the printed framing implies: the first
// 0xFD after the address pair ends the frame, and a 0xFE 0xFE pair inside a
// payload is indistinguishable from a preamble. A consumer that seeds a slot
// (SetSlot) or an ID token (WithIDToken) with 0xFD or 0xFE in it will see the
// frame carrying those bytes truncate or resynchronise on the wire. That is a
// property of the protocol as printed, not a defect of this package, and this
// package deliberately does not paper over it.
//
// # What this radio answers, and what it refuses
//
// The surface is deliberately tiny. It is the memory surface this tier ships
// and the one identity command needed to find the radio, and nothing else.
// Everything not listed as answered is refused with NG (0xFA):
//
//   - 19 00              the transceiver-ID answer. The command is
//     MANUAL-EVIDENCED; ITS REPLY VALUE IS NOT. The
//     document prints the request's Data cell blank, so the
//     request carries no data bytes, and it prints no reply
//     value anywhere at all. The token this fake answers with
//     is INVENTED (see WithIDToken) and lifts nothing.
//   - 1A 00 <hi> <lo>    read one memory record. ASSUMED: the document prints
//     no 1A 00 read request at all. It answers the stored
//     record at RecordLen bytes, or NG if that channel has
//     never been set.
//   - 1A 00 <hi> <lo> <RecordLen bytes>
//     set one memory record. Answers OK (0xFB). A record of
//     any other length is refused with NG.
//
// The five REFUSED forms, each refused on purpose and each with its own reason
// under "Deliberate divergences" below: 1A 00 <hi> <lo> FF (clear a channel),
// 0B (memory clear), 1A 05 in any form (the menu surface this tier does not
// ship), and 18 01 (power ON).
//
// # Channel selectors
//
// MANUAL-EVIDENCED, from the memory-content page:
//
//	00 01 .. 00 99   memory channels 01..99
//	01 00            programmed scan edge P1
//	01 01            programmed scan edge P2
//
// Anything else is not an addressable channel and is refused with NG.
//
// The low byte of a memory-channel selector is taken EXACTLY AS PRINTED: two
// decimal digits, one per nibble, so channel 99 is the byte 0x99 and not 0x63.
// The transcription is explicit that the page "prints whole-byte codes against
// meanings and states no numeric encoding; it does not say BCD or binary", so
// this package reproduces the printed codes rather than choosing an encoding
// for them. A low byte with a nibble above 9, or 0x00, addresses nothing.
//
// The two scan edges have no channel number, so this package's Go-side channel
// argument uses the named constants ChanP1 and ChanP2, which are negative
// precisely so that no arithmetic on a memory channel can land on one by
// accident.
//
// # Record length
//
// RecordLen is 25, DERIVED — not read off a page, and not read off any codec.
// It is the sum of the width_bytes column of the D1 rows of
// core/civ/ic7610/testdata/ic7610-transcription-b.csv, less the two selector
// bytes that CSV counts as its first field:
//
//	①,②    2   memory channel numbers   <- the two SELECTOR bytes, excluded
//	③      1   select memory setting
//	④~⑧    5   operating frequency
//	⑨,⑩    2   operating mode
//	⑪      1   data mode and tone type
//	⑫~⑭    3   repeater tone frequency
//	⑮~⑰    3   tone squelch frequency
//	⑱~㉗   10   memory name
//
// 2+1+5+2+1+3+3+10 = 27, and 27-2 = 25. The geometry witness
// (core/civ/ic7610/testdata/ic7610-geometry-witness.csv) agrees independently:
// its printed-numbering arithmetic reaches the same 27 and its last printed
// index is ㉗ = 27.
//
// BOTH ARTEFACTS ALSO RECORD A DISAGREEMENT ON THE SAME STRIP, and this
// package does not resolve it. The strip is DRAWN in 18 cells, two of which are
// dashed "..." abbreviation cells, while the printed indices run to 27; the
// witness raises that as its STOP 1, STOP 2 and STOP 3. This package follows
// the PRINTED NUMBERING, because that is the numbering the width_bytes column
// records and the only one that yields a byte count at all — a drawn-cell count
// of 18 would be a count of pictures, not of bytes. The choice is recorded in
// PROVENANCE.md; it is not evidence that 25 is right, only a statement of which
// printed thing was followed.
//
// WithRecordLength overrides it, for a consumer that needs to prove its own
// length handling without waiting on hardware to settle the question.
//
// # Two floods, and they are not the same
//
// A consumer has to tell apart two line conditions that look alike on an
// oscilloscope and behave completely differently:
//
//   - a BROADCAST flood, frames addressed to 0x00 — to nobody in particular.
//     This is transceive traffic, and a controller must ignore it without
//     losing its place in its own conversation. That unsolicited frames carry
//     `to` = 00 is ASSUMED: the document prints no broadcast frame at all, and
//     the only answer-direction skeleton it prints has `to` = E0. Asserting the
//     assumption here is not evidence for it.
//   - a CONTROLLER-ADDRESSED flood, frames addressed to 0xE0 — as though the
//     radio were answering continuously. This is a SYNTHETIC line condition.
//     The document describes no radio doing it. It exists so that a consumer
//     which must survive a jabbering peer can be shown to.
//
// They are separate options (WithTransceiveFlood, WithAddressedFlood) and
// separate post-construction controls (StartBroadcastFlood,
// StartAddressedFlood), never one option with a flag, because the consumer
// switches on WHICH IS RUNNING. Either may run alone, both may run together,
// and StopFloods stops whichever are.
//
// The flood frame is deliberately the ID answer with its `to` byte swapped:
//
//	FE FE <to> 98 19 00 <id token...> FD
//
// so the two floods differ from each other in exactly one byte, which is the
// difference under test, and neither invents a command this fake does not
// otherwise answer.
//
// # Echo
//
// WithUSBEcho makes the radio echo every received frame back verbatim BEFORE
// any answer to it. The document records a "CI-V USB Echo Back" setting and a
// [REMOTE]-linked bus case; both look identical on the wire, which is why one
// option covers both.
//
// THE ECHO IS A PROPERTY OF THE LINE, NOT OF THE COMMAND HANDLER, and this
// package places it accordingly: a frame is echoed BEFORE the address filter
// runs, so a frame addressed to some other radio is echoed and then ignored —
// echo, and no answer, and no state change. That ordering is a modelling
// decision, recorded in PROVENANCE.md; the document says nothing about which
// side of an address filter an echo sits on. It is stated here because it is
// the one place "ignored entirely" and "echo every frame verbatim" could be
// read as contradicting each other, and a consumer needs to know which wins.
//
// The echoed bytes are the frame's bytes AS RECEIVED, including any preamble
// padding, and excluding any line noise that preceded them (noise is not part
// of a frame).
//
// # Deliberate divergences from the page
//
// Three refusals here are NOT what a real IC-7610 is likely to do. Each is a
// choice to fail loudly rather than act quietly, and each is recorded again in
// PROVENANCE.md:
//
//   - 1A 00 <hi> <lo> FF is refused with NG. The page prints it, under "To
//     clear the memory channel contents on 1A 00", so a real radio would very
//     likely accept it and clear the channel.
//   - 0B is refused with NG. The page prints it as "Memory clear".
//   - 18 01 is refused with NG. The page prints it as the power-ON command,
//     and its one worked example frame illustrates that command's FE padding.
//
// The first two are refused so that any code path which ever emits a clear
// fails in a test instead of silently destroying a channel's contents in a
// simulator a human is reading. The third is refused because a fake radio has
// no power state to switch and answering OK would assert one.
//
// 1A 05 is refused too, but that is not a divergence: it is the menu surface
// this tier does not ship, and refusing it is what this radio's tier means.
//
// # Empty channels
//
// A channel that has never been set answers NG. That is ASSUMED — the document
// prints the NG code but never says an unwritten channel provokes it — and the
// single capture behind the assumption covers ONE unwritten MEMORY channel.
//
// This fake answers NG for an unset P1 and an unset P2 as well, which is WIDER
// THAN ANY CAPTURE THIS PROJECT HAS NAMED: the memory-channel capture says
// nothing about the scan edges, and the capture that covers P1 says nothing
// about P2. PROVENANCE.md carries that as an assumption in its own right.
//
// Whether a record of 0xFF bytes read back means "empty" is undocumented, and
// THIS FAKE DOES NOT DECIDE IT. A record of 0xFF bytes is stored and returned
// like any other record; "unset" here means "never set, or ClearSlot'd", and it
// is a property of the fake's map, not of the bytes in a record.
//
// # Concurrency and the pipe
//
// A Radio is safe for concurrent use: SetSlot, SlotState, ClearSlot,
// CommandLog, BytesWritten, the flood controls and Close may all be called from
// goroutines other than whatever is reading or writing Port(). Run tests with
// -race.
//
// Port() is one end of a net.Pipe, which is UNBUFFERED: a write to it blocks
// until the radio reads. Output does not block, though — every byte the radio
// sends goes through an internal queue drained by a single writer goroutine, so
// a flood, an echo and an answer can never interleave mid-frame, and the radio
// keeps reading the host's commands even when nothing is reading its replies.
// The queue is bounded (maxQueuedFrames); a consumer that starts a flood and
// then never reads will silently lose the oldest frames rather than grow
// without limit.
package fakeic7610

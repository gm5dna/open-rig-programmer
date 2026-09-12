// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7410 is an independent, stdlib-only IC-7410 CI-V simulator.
//
// # NO IC-7410 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
//
// Everything below is UNVERIFIED against hardware — there is no
// writeTrialsComplete flag in this package because it makes no hardware
// claim to flag. It was built from
// docs/superpowers/icom-matrices/ic7410-capability-matrix.md alone, without
// reading core/civ/ic7410 or core/driver/ic7410 — see "The hard rule"
// below.
//
// # The record: 40 bytes, not 25 — the matrix's central finding
//
// `spec.md` §1's IC-7410 clause and the S1 evidence file both state the
// `1A 00` record is 25 bytes ("repacked... unchanged by coincidence"). The
// capability matrix corrects this to 40 bytes (matrix §3.11, spec.md §7): a
// 400 dpi render of the manual's PDF p.115 shows a SECOND field group, in
// FILLED reference circles (❹–⑱), between the tone-squelch bracket and the
// name bracket, that a crop not extending far enough right — and
// `pdftotext`, which renders both the filled and outline circles as plain
// "4-18" — both missed.
//
//	③        Select-memory + Split setting     1 B  (UNMAPPED, both nibbles)
//	④~⑧      Operating frequency (RX)           5 B
//	⑨        Operating mode                     1 B
//	⑩        Filter                             1 B
//	⑪        Data mode (whole byte)              1 B
//	⑫        Tone setting (hi nibble; lo fixed) 1 B
//	⑬~⑮      Repeater tone frequency (TX tone)   3 B
//	⑯~⑱      Tone squelch frequency (RX tone)    3 B
//	❹~⑱      TX-duplicate block (mirrored)      15 B
//	⑲~㉗      Memory name                         9 B
//	                                            ----
//	                                            40 B
//
// The TX-duplicate block itself decomposes (matrix §1b) as 5 B TX frequency
// (mapped to `spec.FieldTxFrequency` by the wave's §7 ruling) followed by
// 10 B of TX-side mode/filter/data-mode/tone-mode/tone bytes with no
// neutral field home (UNMAPPED, "second TX-side copy, no neutral field").
//
// This package parses NO field of the record. It is entirely
// UNINTERPRETED bytes in, bytes back out — the layout above is recorded
// for provenance, not decoded anywhere in this file. A consumer wanting
// tx_frequency or any other field must build the 40-byte record itself,
// per the matrix.
//
// # The hard rule: NOTHING project-internal
//
// fakeic7410 MUST NOT import core/civ, core/civ/ic7410, core/driver,
// core/driver/ic7410, core/codeplug, core/spec, or any sibling fake.
// Standard library only, in this directory and every directory beneath
// it, with the one exception below. imports_test.go enforces it with a
// recursive go/parser scan, proven green before any protocol code in this
// package existed.
//
// This package's AUTHOR was quarantined the same way: forbidden to read
// core/civ/ic7410/*.go or core/driver/ic7410/*.go, and did not. Its only
// inputs were docs/superpowers/icom-matrices/ic7410-capability-matrix.md,
// spec.md §1/§7, the Tier spec documents, the manual layout text,
// core/civ/ic7410/testdata/ (frozen vectors, read-only), and the SHAPE
// (not the protocol) of internal/fakeic7851 and internal/fakeic7610 — with
// internal/fakeic7200 read too, as this wave's own precedent for the same
// kind of record-width correction. If this fake reused the production
// codec, a systematic misreading of the IC-7410 record could agree with
// itself on both sides of a test and never surface; two independent
// implementations checked against each other is what makes that bug
// visible.
//
// THE ONE EXCEPTION IS internal/fakepipe: the net.Pipe pair, the
// goroutine bookkeeping, the interruptible latency wait and the raw
// write. PROTOCOL-FREE — []byte and a duration, nothing else — so a bug
// in it cannot make a wrong codec look right.
//
// # Framing
//
// MANUAL-EVIDENCED (matrix §1 row 2, PDF p.108 frame skeleton and OK/NG
// strips):
//
//	request  FE FE 80 E0 <cn> [<sc>] <data...> FD
//	answer   FE FE E0 80 <cn> [<sc>] <data...> FD
//	OK       FE FE E0 80 FB FD
//	NG       FE FE E0 80 FA FD
//
// 0x80 (radioAddrDefault) is the IC-7410's default address; 0xE0
// (controllerAddr) the controller's. A frame whose `to` byte is not this
// radio's address is not for it and is ignored entirely: no answer, no
// state change. WithRadioAddress moves the listening address AND the
// source byte of every reply.
//
// A frame whose `from` byte is not the controller (0xE0) is also ignored
// — TestOnlyTheControllerIsAnswered — matching the precedent
// `ic7610-capability-matrix.md`/fakeic7610, fakeic7851 and fakeic7200 all
// set: a real transceive-capable radio does not answer traffic from a
// second controller or another radio's transceive frames, it only answers
// the controller addressing it.
//
// # What this radio answers, and what it refuses
//
//   - 19 00              the transceiver-ID answer. The command is
//     MANUAL-EVIDENCED (matrix §3.12); ITS REPLY VALUE IS NOT — the
//     manual prints no `19 00` example frame anywhere in the 124 pages.
//     This fake echoes back `19 00` plus the model name bytes
//     (WithModelName, default "IC-7410"), following the fakeic7851/
//     fakeic7200 convention: INVENTED, lifts nothing.
//   - 1A 00 <hi> <lo>    read one record. ASSUMED (matrix §3.7): the
//     manual prints the command and only the full-record answer form,
//     never a bare read-request frame. Answers the stored record at
//     RecordLen bytes, or the configured empty behaviour if that
//     channel has never been set.
//   - 1A 00 <hi> <lo> <RecordLen bytes>
//     set one record. Answers OK (0xFB). A record of any other length
//     is refused with NG — ASSUMED (matrix §3.10): the manual never
//     states whether a short record is accepted, padded or rejected.
//   - 1A 00 <hi> <lo> FF  the printed clear form (matrix §3.13,
//     "③: FF / ④ or later: None") — REFUSED. MANUAL-EVIDENCED as a wire
//     form; refusing it is a CHOICE fixed by the tier (FieldErase carries
//     zero FieldSupport everywhere in this project, and the matrix
//     records the wire form's existence without deciding a driver's
//     admission policy). Falls through the record-length check like any
//     other malformed set.
//   - 0B                 "Memory clear" — REFUSED, same reasoning as
//     above (matrix §3.13, PDF p.109 command table).
//
// Everything else is out of this package's surface and refused with NG:
// this is a memory-record simulator, not a full live-command surface,
// matching the scope fakeic7610, fakeic7851 and fakeic7200 all keep.
//
// # Channel selectors
//
// MANUAL-EVIDENCED (matrix §1 row 5, §1b banks): a flat, four-digit
// packed-BCD selector across the two bytes following `1A 00`:
//
//	0001 .. 0099   memory channels 1..99
//	0100           programmed scan edge P1
//	0101           programmed scan edge P2
//
// No group byte, no CALL bank (matrix §1b: absent, MANUAL-EVIDENCED). The
// manual additionally states byte ③ (select-memory/split, offset 0 of the
// record) must read 0/0 on P1 and P2 — this fake does not enforce it, since
// the record is opaque (see above); a consumer wanting that constraint
// checked builds it into the record it passes to SetSlot.
//
// Anything outside this space is refused with NG. The two scan edges are
// negative Go-side sentinels (ScanEdgeP1, ScanEdgeP2) so that no
// arithmetic on a memory channel can land on one by accident — the same
// convention fakeic7851 and fakeic7200 use for their own scan-edge pairs.
//
// # Empty channels
//
// An unwritten channel answers NG by default — ASSUMED (matrix §3.8,
// register entries ic7410-empty-read-answers-fa / ic7410-ff-record-
// meaning; nothing in the 124 pages describes a read of an empty channel).
// WithAllFFEmpty models the alternative empty convention the matrix leaves
// equally open: an all-0xFF record. This fake decides nothing about which
// is right — it only offers both, exactly as its sibling fakes do for
// their own equally-undocumented radios.
//
// # Transceive: ON at the factory, broadcast form ASSUMED
//
// MANUAL-EVIDENCED (matrix §3.5): PDF p.96 item 44, "CI-V Transceive
// (Default: ON)." What is NOT printed is the `to=00` broadcast address
// form — ASSUMED (matrix §3.5), the same assumption every sibling matrix
// makes. WithTransceiveFlood models that assumed `to=00` traffic.
// WithAddressedFlood is the tier-wide SYNTHETIC line condition (a
// jabbering peer answering `to=E0` continuously) that no document
// describes any radio doing; it exists only so a consumer surviving one
// can be shown to, the same reason fakeic7610, fakeic7851 and fakeic7200
// all carry it.
//
// # No echo option — a genuine model difference, not an omission
//
// fakeic7610 and fakeic7851 both offer a USB-echo option, because their
// manuals document an adjacent "CI-V USB Echo Back" setting or an
// echoing REMOTE-linked bus. The IC-7410's manual raises NEITHER: matrix
// §3.6 records a swept set-mode list (items 1–48) and interface section
// for "echo" finding zero hits. There is therefore nothing on this model
// for an echo option to model, and none is offered — following the same
// call fakeic7200's matrix drew for its own model.
//
// # Record length, and proving an alternative
//
// RecordLen is 40, the matrix's corrected figure (see above).
// WithRecordLength overrides it — a consumer that wants to prove the
// superseded 25-byte reading, or a length this project does not yet know
// of, sets it here rather than editing a constant and quietly changing
// what every other test means.
//
// # Concurrency and the pipe
//
// A Radio is safe for concurrent use. Port() is one end of an unbuffered
// net.Pipe; every byte the radio sends goes through internal/fakepipe's
// machinery so a flood and an answer can never interleave mid-frame.
package fakeic7410

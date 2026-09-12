// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7200 is an independent, stdlib-only IC-7200 CI-V simulator.
//
// # NO IC-7200 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
//
// Everything below is UNVERIFIED against hardware (writeTrialsComplete:
// false — there is no such flag in this package because it makes no
// hardware claim to flag). It was built from
// docs/superpowers/icom-matrices/ic7200-capability-matrix.md alone, without
// reading core/civ/ic7200 or core/driver/ic7200 — see "The hard rule"
// below.
//
// # The record: 17 bytes, not 9 — the matrix's central finding
//
// `spec.md` §1's IC-7200 clause and the S1 evidence file both state the
// `1A 00` record is 9 bytes. The capability matrix corrects this to 17
// bytes (matrix §3.11, spec.md §7): a 300 dpi render of the manual's PDF
// p.120 shows a SECOND field group, in filled/reversed circled numerals
// (❹–⓫), that `pdftotext -layout` renders indistinguishably from the
// primary outline-numeral indices (④–⑪) and which both the S1 sweep and a
// first read of the layout text mistook for a repeated citation of the
// same span rather than a second one.
//
//	③        Split setting                    1 B
//	④~⑧      Operating frequency (RX)          5 B
//	⑨,⑩      Operating mode + filter           2 B
//	⑪        Data mode                         1 B
//	❹~⓫      TX-duplicate block (mirrored)     8 B
//	                                           ----
//	                                           17 B
//
// This package follows the matrix's corrected figure. RecordLen is 17.
// WithRecordLength lets a consumer prove the other reading (9) instead,
// exactly as the matrix's own lift (`ic7200-record-length-ch01`)
// proposes.
//
// This package parses NO field of the record. It is entirely
// UNINTERPRETED bytes in, bytes back out — the layout above is recorded
// for provenance, not decoded anywhere in this file. A consumer wanting
// tx_frequency or any other field must build the 17-byte record itself,
// per the matrix.
//
// # The hard rule: NOTHING project-internal
//
// fakeic7200 MUST NOT import core/civ, core/civ/ic7200, core/driver,
// core/driver/ic7200, core/codeplug, core/spec, or any sibling fake.
// Standard library only, in this directory and every directory beneath
// it, with the one exception below. imports_test.go enforces it with a
// recursive go/parser scan, proven green before any protocol code in this
// package existed.
//
// This package's AUTHOR was quarantined the same way: forbidden to read
// core/civ/ic7200/*.go or core/driver/ic7200/*.go, and did not. Its only
// inputs were docs/superpowers/icom-matrices/ic7200-capability-matrix.md,
// spec.md §1/§7, the Tier spec documents, the manual layout text,
// core/civ/ic7200/testdata/ (frozen vectors, read-only), and the SHAPE
// (not the protocol) of internal/fakeic7851 and internal/fakeic7610. If
// this fake reused the production codec, a systematic misreading of the
// IC-7200 record could agree with itself on both sides of a test and
// never surface; two independent implementations checked against each
// other is what makes that bug visible.
//
// THE ONE EXCEPTION IS internal/fakepipe: the net.Pipe pair, the
// goroutine bookkeeping, the interruptible latency wait and the raw
// write. PROTOCOL-FREE — []byte and a duration, nothing else — so a bug
// in it cannot make a wrong codec look right.
//
// # Framing
//
// MANUAL-EVIDENCED (matrix §3.4, §3.10):
//
//	request  FE FE 76 E0 <cn> [<sc>] <data...> FD
//	answer   FE FE E0 76 <cn> [<sc>] <data...> FD
//	OK       FE FE E0 76 FB FD
//	NG       FE FE E0 76 FA FD
//
// 0x76 (radioAddrDefault) is the IC-7200's default address; 0xE0
// (controllerAddr) the controller's. A frame whose `to` byte is not this
// radio's address is not for it and is ignored entirely: no answer, no
// state change. WithRadioAddress moves the listening address AND the
// source byte of every reply.
//
// A frame whose `from` byte is not the controller (0xE0) is also ignored
// — TestOnlyTheControllerIsAnswered — matching the precedent
// `ic7610-capability-matrix.md`/fakeic7610 and fakeic7851 both set: a real
// transceive-capable radio does not answer traffic from a second
// controller or another radio's transceive frames, it only answers the
// controller addressing it.
//
// # What this radio answers, and what it refuses
//
//   - 19 00              the transceiver-ID answer. The command is
//     MANUAL-EVIDENCED (matrix §3.12(i)); ITS REPLY VALUE IS NOT — the
//     manual prints the request with a blank data cell and no reply
//     value anywhere. This fake echoes back `19 00` plus the model name
//     bytes (WithModelName, default "IC-7200"), following the
//     fakeic7851 convention: INVENTED, lifts nothing.
//   - 1A 00 <hi> <lo>    read one record. ASSUMED (matrix §3.7): the
//     manual prints the command and only the full-record answer form,
//     never a bare read-request frame. Answers the stored record at
//     RecordLen bytes, or the configured empty behaviour if that
//     channel has never been set.
//   - 1A 00 <hi> <lo> <RecordLen bytes>
//     set one record. Answers OK (0xFB). A record of any other length
//     is refused with NG — ASSUMED (matrix §3.10, register entry
//     ic7200-full-record-mandatory): the manual prints one full form and
//     says nothing about a partial one.
//   - 0B                 "Memory clear" — REFUSED. MANUAL-EVIDENCED as a
//     command (matrix §3.13); refusing it is a CHOICE fixed by the tier
//     (FieldErase carries zero FieldSupport everywhere in this project).
//     Unlike IC-7610, this manual prints no second, `1A 00 <hi> <lo> FF`
//     clear form at all (matrix §3.13), so there is only the one
//     refusal to make.
//
// Everything else — including the live `0F` split command and the live
// `1A 02` filter-width command the matrix records as separate, unrelated
// commands (§1b, §3.15(a)) — is out of this package's surface and
// refused with NG: this is a memory-record simulator, not a full live-
// command surface, matching the scope fakeic7610 and fakeic7851 both
// keep.
//
// # Channel selectors
//
// MANUAL-EVIDENCED (matrix §1 row 4, §1b banks): a flat, four-digit
// packed-BCD selector across the two bytes following `1A 00`:
//
//	0001 .. 0199   memory channels 1..199
//	0200           programmed scan edge P1
//	0201           programmed scan edge P2
//
// No group byte, no CALL bank (matrix §1b: absent, MANUAL-EVIDENCED).
// Anything outside this space is refused with NG. The two scan edges are
// negative Go-side sentinels (ScanEdgeP1, ScanEdgeP2) so that no
// arithmetic on a memory channel can land on one by accident — the same
// convention fakeic7851 uses for its own (smaller) scan-edge pair.
//
// # Empty channels
//
// An unwritten channel answers NG by default — ASSUMED (matrix §3.8(a),
// register entry, "not stated to be what an unwritten channel provokes").
// WithAllFFEmpty models the alternative the manual leaves equally open:
// an all-0xFF record. Matrix §3.8(b) records that this radio's manual is
// silent on the question in a stronger way than IC-7610's (no clear-list
// form is printed at all to disambiguate it), so this fake decides
// nothing about which is right — it only offers both, exactly as its
// sibling fakes do for their own equally-undocumented radios.
//
// # Transceive: ON at the factory, broadcast form ASSUMED
//
// Unusually for this tier, MANUAL-EVIDENCED (matrix §3.5(a)): CI-V
// Transceive is ON by default, and the two broadcast verbs are printed
// ("Send frequency data (for transceive operation)" etc). What is NOT
// printed is the `to=00` broadcast address form — ASSUMED (matrix
// §3.5(b)), the same assumption every sibling matrix makes. This
// package's WithTransceiveFlood models that assumed `to=00` traffic.
// WithAddressedFlood is the tier-wide SYNTHETIC line condition (a
// jabbering peer answering `to=E0` continuously) that no document
// describes any radio doing; it exists only so a consumer surviving one
// can be shown to, the same reason fakeic7610 and fakeic7851 both carry
// it.
//
// # No echo option — a genuine model difference, not an omission
//
// fakeic7610 and fakeic7851 both offer a USB-echo option, because their
// manuals document an adjacent "CI-V USB Echo Back" setting or an
// echoing REMOTE-linked bus. The IC-7200's manual raises NEITHER: matrix
// §3.6 records a targeted grep of the complete layout text for "echo"
// finding zero hits, and matrix §3.2 records that this radio's CI-V path
// is not a USB CDC endpoint at all — a dedicated 3.5 mm [REMOTE] jack
// through an external CT-17 level converter to an RS-232C port. There is
// therefore nothing on this model for an echo option to model, and none
// is offered; the matrix itself draws this conclusion ("no register
// entry: there is nothing to lift on a topic this document never
// raises").
//
// # Record length, and proving the alternative
//
// RecordLen is 17, the matrix's corrected figure (see above).
// WithRecordLength overrides it — a consumer that wants to prove the
// competing 9-byte reading, or that learns the true length from a
// capture this project does not have, sets it here rather than editing a
// constant and quietly changing what every other test means. The
// matrix's own lift (`ic7200-record-length-ch01`) names this exact
// fork: "a count of 17 confirms this matrix; a count of 9 would mean the
// TX-duplicate block is written but never read back."
//
// # Concurrency and the pipe
//
// A Radio is safe for concurrent use. Port() is one end of an unbuffered
// net.Pipe; every byte the radio sends goes through internal/fakepipe's
// machinery so a flood, an echo (there is none here) and an answer can
// never interleave mid-frame.
package fakeic7200

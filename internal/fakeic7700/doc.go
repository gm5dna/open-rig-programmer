// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7700 is an independent, stdlib-only IC-7700 CI-V simulator.
// Its wire register is the manual-derived 39-byte record described below,
// addressed by two packed-BCD selector bytes.
//
// # NO IC-7700 HAS EVER BEEN CONNECTED TO THIS PROJECT
//
// Everything below is UNVERIFIED against hardware. Every claim is a claim
// about `docs/superpowers/icom-matrices/ic7700-capability-matrix.md`
// ("the matrix"), which is itself built from the IC-7700's own Instruction
// Manual (Section 14, "CONTROL COMMAND") — not a claim about a transceiver.
// writeTrialsComplete is FALSE: no capture of any kind exists against this
// model (matrix §3.14).
//
// # THE HARD RULE: NOTHING project-internal
//
// fakeic7700 MUST NOT import any package of this project — not core/civ, not
// core/civ/ic7700, not core/driver/ic7700, not core/codeplug, not core/spec,
// not any sibling fake. Standard library only, in every non-test file, in
// this directory and every directory beneath it. TestNoCoreImports
// (imports_test.go) enforces it with a recursive go/parser scan.
//
// This package was authored under quarantine: the agent that wrote it was
// forbidden to open core/civ/ic7700/*.go or core/driver/ic7700/*.go, and did
// not. Its inputs were the matrix, spec.md §1 (the IC-7700 section) and §7
// (the TX-duplicate-block ruling), the Tier design specs, the manual's own
// PDF/layout text, the frozen vectors in core/civ/ic7700/testdata/ (byte
// requests only — no reply is recorded there for this fake to have copied),
// and the SHAPE of internal/fakeic7851 (+ internal/fakeic7610 for the
// generalised 7610-family record-length arithmetic). If this fake reused the
// production codec, a systematic bug in it — an off-by-one in the record
// length, a selector decoded the wrong way round — would agree with itself
// on both sides of every "send a command, check the reply" test this project
// runs, and the bug would never surface. Two independent implementations of
// one protocol, checked against each other, is what makes that class of bug
// visible.
//
// THE ONE EXCEPTION IS internal/fakepipe: the net.Pipe pair, the goroutine
// bookkeeping, the interruptible latency wait and the raw write. It is
// PROTOCOL-FREE — it sees []byte and a duration and nothing else, so it
// carries no framing, no field layout and no reply building. Everything
// above the wire stays here, written independently.
//
// # Framing
//
// MANUAL-EVIDENCED, matrix §3.10 / §1 row 2, citing PDF p.202 (folio 14-2),
// "D Data format":
//
//	set     FE FE 74 E0 1A 00 <data...> FD
//	answer  FE FE E0 74 <data...> FD
//	OK      FE FE E0 74 FB FD
//	NG      FE FE E0 74 FA FD
//
// 0x74 is the transceiver's factory default address (matrix §3.4, "The
// IC-7700's address is 74h") and 0xE0 the controller's. A frame whose `to`
// byte is not this radio's configured address is IGNORED ENTIRELY: no
// answer, no state change. WithRadioAddress moves both the address this
// radio answers to and the `from` byte of every reply it sends, matching the
// matrix's note that the address is user-changeable (01h-DFh).
//
// Preamble/terminator handling (extra leading 0xFE is padding; the first
// 0xFD after the address pair ends the frame; data bytes are not escaped)
// follows the same convention as every 7610-family fake in this project —
// the matrix cites no framing detail specific to this radio beyond the
// skeleton above.
//
// # What this radio answers, and what it refuses
//
//   - 19 00              the transceiver-ID answer. MANUAL-EVIDENCED as a
//     wire form (matrix §1 row 2, §3.12: PDF p.204, command table, "Read the
//     transceiver ID"); its REPLY VALUE IS NOT PRINTED anywhere in the
//     manual. This fake answers with the ASCII model name
//     ("IC-7700" by default, WithModelName to change it) — an INVENTED
//     token, diagnostics only, matching the matrix's own posture (register
//     ic7700-id-token: "diagnostics only, per the tier's stated posture").
//   - 1A 00 <hi> <lo>            read one memory record. The request form
//     itself is ASSUMED (register ic7700-1a00-read-request, matrix §3.7:
//     "the manual prints only the answer-carrying diagram... it does not
//     print a worked example of a bare 1A 00 request"). Answers the stored
//     39-byte record, or the configured empty behaviour if that channel has
//     never been set (see "Empty channels" below).
//   - 1A 00 <hi> <lo> <39 bytes>  set one memory record. Answers OK (0xFB).
//     A record of any other length is refused with NG — including the
//     printed single-byte clear form "1A 00 <hi> <lo> FF" (matrix §3.13:
//     "add the code 'FF' after the memory channel number... This completes
//     the memory clearing"). This fake refuses that form deliberately,
//     rather than acting on it: `FieldErase` carries zero support in this
//     tier (matrix §3.13, §2 row 10) and no clear command is shipped, so a
//     driver code path that ever emitted a clear should fail loudly here
//     rather than silently empty a channel in a simulator a human is
//     reading. Whether a SHORT write (any other wrong length) is accepted
//     at all is UNTESTED and ASSUMED-refused (matrix §3.10, "consistent
//     with the tier's general write-choreography posture").
//   - 0B                 "Memory clear" (matrix §3.13, PDF p.203, command
//     table). Refused with NG for the same reason as the inline-FF form
//     above.
//
// Everything else this radio has not been told to answer is refused with
// NG (0xFA). There is no richer error surface: the manual describes none.
//
// # Channel selectors
//
// MANUAL-EVIDENCED, matrix §1 row 5 / §1b "The banks", citing PDF p.213
// (folio 14-13), "q,w Memory channel number":
//
//	00 01 .. 00 99   memory channels 1..99   (bank MEM, 99 slots)
//	01 00            programmed scan edge P1 (bank SCAN)
//	01 01            programmed scan edge P2 (bank SCAN)
//
// There is no CALL bank on this model (matrix §1 row 5, "swept the command
// table and the Memory operation chapter; no call-channel address, no
// call-channel command, no mention of a call channel anywhere"). The two
// selector bytes are packed BCD, one decimal digit per nibble: channel 99 is
// the bytes 0x00, 0x99. Any nibble above 9, or a value outside 1..101,
// addresses nothing and is refused with NG. The Go-side channel name for the
// two scan edges is the string "P1"/"P2" (parseChannel); memory channels are
// their three-digit decimal string, e.g. "001".."099", matching the golden
// vector core/civ/ic7700/testdata/IC-7700-vectors.golden's own
// "read-record" request (selector 00 01 = channel "001").
//
// # Record length: 39 bytes
//
// MANUAL-EVIDENCED, matrix §3.11, derived from the printed byte-cell diagram
// (PDF p.213) read at 400 dpi (bi/ic7700-p213-400dpi.png), not merely the
// `pdftotext -layout` text, which abbreviates the diagram's longer spans
// with an ellipsis:
//
//	select/split nibble byte        1 B
//	RX frequency                    5 B
//	RX mode + RX filter             2 B
//	tone-type/data-mode nibble byte 1 B
//	repeater tone frequency         3 B
//	tone squelch frequency          3 B
//	TX-duplicate block (mirrors     14 B  (matrix §3.11: "programmed in the
//	  the 14-byte RX span above)            same manner as ④-⑰")
//	memory name                     10 B
//	                                 --
//	                                 39 B
//
// RecordLen is this total. This package does not decode a single byte of a
// record's contents — matching every sibling fake in this tree, and the
// D-STAR-byte precedent the tier spec cites for the IC-9100 — the record is
// opaque, stored and returned verbatim. In particular the TX-duplicate
// block's own mode/filter/tone-type/tone bytes (matrix §3.15, idx20-28) are
// UNMAPPED to any spec.Field and are simply seven of this record's 39 bytes
// to this fake; whether a driver mirrors the RX span into them on write is a
// driver-level choice (spec.md §7), not something this wire-level simulator
// enacts or needs to know.
//
// NameLen (10) is the name field's own width (matrix §1 row 7, "Up to 10
// characters"), exported for a consumer building a record, not used by this
// package's own logic (which does not parse the record). NamePad (ASCII
// space, 0x20) is likewise exported and unenforced: the matrix's own
// grading of the pad byte is ASSUMED (§3.9, "not stated... swept PDF
// p.211/p.213... none found"), and matches the space-padded name this
// tier's own core/civ/ic7700/testdata golden vector already shows
// ("TESTCH    ") — offered here for a consumer's convenience, not asserted
// as a fact this fake enforces.
//
// # Empty channels
//
// A channel that has never been set answers NG by default (register
// ic7700-empty-channel-reply, matrix §3.8(a): "Not stated. ASSUMED (FA/NG
// assumed, per the general OK/NG scheme)"). WithAllFFEmpty switches to the
// alternative reading (register ic7700-all-ff-empty, matrix §3.8(b): "Not
// stated separately... this cannot be inferred from (a)'s capture") — a
// record of 39 0xFF bytes, framed as an ordinary read answer. The two are
// genuinely separate assumptions, not two readings of one fact, exactly as
// the matrix records them, so they are exposed as separate options rather
// than folded into one with a flag.
//
// # Echo and transceive broadcasts
//
// WithUSBEcho makes the radio echo every received frame back verbatim
// before any answer to it, BEFORE the address filter runs (a frame
// addressed elsewhere is echoed and then ignored). The matrix's own
// evidence on this point is an ABSENCE, not a setting to model faithfully
// (§3.6: "MANUAL-EVIDENCED absence of any echo-back setting... there is no
// USB-CDC port on this model for a USB-echo-back setting to apply to").
// register ic7700-no-echo-setting therefore covers the ASSUMPTION that no
// echo exists to configure on real hardware; this package still offers the
// option, as every sibling fake does, so a consumer exercising a
// [REMOTE]+CT-17-linked bus (which DOES reflect whatever it carries, being
// a shared line, independently of any radio setting) can be tested against
// one.
//
// WithTransceiveFlood starts a BROADCAST flood (`to` = 0x00, unsolicited
// frames — matrix §3.5: transceive is ON by default, "Transceive operation
// is possible... connected to other Icom HF transceivers or receivers",
// though the manual prints no broadcast frame skeleton itself, so the `to`
// = 0x00 form is ASSUMED here, by the same reasoning and for the same
// tier-wide consistency as every sibling fake, not because this matrix
// names it). WithAddressedFlood starts a SYNTHETIC `to` = 0xE0 flood, for a
// consumer proving it survives a jabbering peer; the matrix describes no
// radio doing this either.
//
// # Concurrency
//
// A Radio is safe for concurrent use. Port() is one end of a net.Pipe.
package fakeic7700

// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeftdx5000 simulates a Yaesu FTdx5000's memory-channel CAT
// behaviour over an in-memory serial connection (Radio.Port()) — the test
// double the FTdx5000's own layers will run against once registered: the
// transport engine, core/driver/ftdx5000, the CLI's --fake mode, and the
// GUI's demo mode, the role internal/fakeft991a plays for the FT-991A and
// internal/fakedx101 for the FTdx101.
//
// # NO FTDX5000 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
//
// Everything below was built from
// docs/superpowers/ftdx5000-capability-matrix.md and
// docs/fixtures-private/manuals/ftdx5000_layout.txt alone — see "The hard
// rule" below. There is no writeTrialsComplete flag in this package because
// it makes no hardware claim to flag, the same reasoning fakeic7200's doc.go
// gives for its own NoTag radio.
//
// # The record: MR/MW, 27 bytes, NoTag, single row, no MT at all
//
// Unlike the FT-991A/FT-891/FTdx101 family this package's shape templates
// come from, THIS radio's Control Command List carries no combined MT
// command — only MR (MEMORY READ, "SET READ ANS. AI" = "X O O X") and MW
// (MEMORY WRITE, "O X X X") (layout:104-108's list). The two commands are
// asymmetric by design, not by omission:
//
//   - MR has no Set form and no AI broadcast: a read REQUEST is "MR" plus a
//     3-digit slot (P1, 001-117) plus ";" (layout:944, the "M R P1 P1 P1 ;"
//     row under MR's own "Read" heading); the ANSWER is the full 27-byte
//     record: opcode, P1 slot, P2 (8-digit FreqHz), P3 (clarifier sign and
//     4-digit magnitude), P4 (RxClar), P5 (TxClar), P6 (Mode), P7 (Kind),
//     P8 (CTCSSState), P9 (2-digit tone index), P10 (Shift), terminator
//     (layout:945-952, matrix §2).
//   - MW has no Read form and, per the command list's own ANS column, NO
//     ANSWER AT ALL — not even an echo. A Set is either stored or it is
//     not; nothing on the wire ever says which. That is a MANUAL-EVIDENCED
//     fact about this radio (layout:108), not a gap this package invented
//     an acknowledgement for.
//
// P7 in an MR answer is always '1' (Memory): MR's own P7 legend is
// "0: VFO 1: Memory" (layout:949), and MEMORY CHANNEL READ never answers a
// VFO record — there is no other frame this command could be reporting.
// This is read directly off the legend plus the command's own scope, not a
// guess, so it carries no register entry below. MW's P7 is a fixed literal
// "0" instead ("P7 0: (Fixed)", layout:981) — a different byte with the same
// spelling, not the VFO/Memory flag repeated.
//
// ID (IDENTIFICATION) is also modelled: no Set form, read request "ID;",
// answer "ID" + the 4-digit CAT ID + ";" (layout:769-775). This radio's ID
// is "0362" (layout:770, matrix §4) — FIXED, because this is a bare-`New`,
// single-row package (matrix's own §0: "one model row, bare `New`"); there
// is no WithModelName option here, unlike the three multi-row packages this
// wave also builds.
//
// Nothing else on this radio's 60-plus-command surface is modelled. This is
// a memory-record simulator, not a full live-command surface — the same
// scope fakeic7200's doc.go states for its own radio, and for the same
// reason: every command this package does not recognise is silently
// ignored (see the ASSUMED register below), which is cheaper and no less
// honest than fabricating a refusal token this radio's manual never prints.
//
// # NoTag — no name byte anywhere in this record
//
// The matrix's §0 REGISTRABLE-NO-NAME finding and §2's record table agree:
// no TAG/NAME/LABEL field exists after P10 Shift on either MR or MW, and a
// whole-manual grep for "tag"/"name"/"label" finds nothing but the unrelated
// pinout header "PIN NAME" (layout:26). This package therefore never reads
// or writes a channel name — there is no field for one in MemState, and no
// code path that could invent one.
//
// # The hard rule: NOTHING project-internal
//
// fakeftdx5000 MUST NOT import core/cat, core/driver, core/driver/ftdx5000,
// core/driver/internal/yaesu, core/codeplug, core/spec, or any sibling fake.
// Standard library only, in this directory and every directory beneath it,
// with the one exception below. imports_test.go enforces it with a
// recursive go/parser scan.
//
// This package's author was quarantined the same way: forbidden to read
// core/driver/ftdx5000/*.go, and did not. Its only inputs were
// docs/superpowers/ftdx5000-capability-matrix.md, spec.md §1/§3 (the wave's
// spec), reviews/plan.md, the manual layout text, and the SHAPE (not the
// protocol) of internal/fakeft991a, internal/fakedx101 and internal/fakets480.
// If this fake reused the driver's understanding of the wire, a systematic
// bug in that understanding — an off-by-one offset, a validation rule
// subtly wrong — would sit on both sides of every "send a command, check the
// reply" test this project runs, and never surface; two independent
// implementations checked against each other is what makes that bug visible.
//
// THE ONE EXCEPTION IS internal/fakepipe: the net.Pipe pair, the goroutine
// bookkeeping, the interruptible latency wait and the raw write.
// PROTOCOL-FREE — []byte and a duration, nothing else — so a bug in it
// cannot make a wrong codec look right.
//
// # The ASSUMED register
//
// Four places this package had to guess, because the manual leaves the
// question open. Each is cited by name at the code that implements it
// (register_test.go's TestASSUMEDRegisterIsComplete enforces the pairing).
//
//  1. EMPTY-SLOT ANSWERS. MR's own ANS column is unconditionally "O"
//     (layout:108) — nothing in the command's chart makes the answer
//     conditional on the channel ever having been written. What content an
//     unwritten channel answers with is NOT stated, so this package answers
//     the zero record (FreqHz 0, clarifier "+0000", both clarifiers OFF,
//     Mode '1' LSB, CTCSSState '0' OFF, tone index 0, Shift '0' Simplex) —
//     the same "zero record, not silence" convention internal/fakets480
//     registers for its own NoTag radio. WithSlot overrides it per channel.
//  2. OUT-OF-RANGE AND MALFORMED REQUESTS ARE SILENT. A malformed MR
//     request, an MR for a slot outside 001-117, or any frame this package
//     does not recognise produces no reply at all. Nothing in this manual
//     documents an error token (no "?;", no NG byte, anywhere in the 20
//     pages) for this radio's ASCII CAT protocol, so inventing one would be
//     fabrication with no citation; silence is instead the one response
//     shape this radio's own command list already proves exists (MW's own
//     ANS "X", layout:108) — this package extends that same shape to every
//     other case it cannot answer, rather than making one up.
//  3. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Bytes accumulate until a ';'
//     terminator is seen (the manual's own stated framing rule, layout:110).
//     If more than maxAccumulatorBytes arrive with no terminator, the buffer
//     is dropped and resyncs on the next ';' — a bound this package needs to
//     stay memory-safe against a runaway peer, which the manual has no
//     occasion to discuss.
//  4. AN INVALID MW SET IS DISCARDED, NOT ACKNOWLEDGED. A Set frame of the
//     wrong length, or one whose fields are outside their documented
//     legends (an unlisted mode nibble, a non-'0' P7, digits where the
//     legend requires them), is neither stored nor answered — matching
//     entry 2's reasoning: MW has no channel to report a validation failure
//     through (ANS "X"), so silent rejection is the only behaviour that does
//     not fabricate one.
//
// # Concurrency and the pipe
//
// A Radio is safe for concurrent use. Port() is one end of an unbuffered
// net.Pipe; every byte the radio sends goes through internal/fakepipe's
// machinery so replies can never interleave mid-frame. slots is guarded by
// Radio.mu; the single serving goroutine and SlotState (called from test or
// consumer goroutines) both take it.
package fakeftdx5000

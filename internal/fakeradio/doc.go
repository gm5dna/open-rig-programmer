// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeradio simulates an FT-710's CAT behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double every
// later layer runs against: the transport engine, the driver, the clone
// service, the CLI's --fake mode, and the GUI's demo mode.
//
// # The hard rule: no core/cat, core/codeplug, or core/spec
//
// fakeradio MUST NOT import core/cat (nor core/codeplug or core/spec —
// this is a pure byte-level simulator). Every byte offset, field width,
// and validation rule in parser.go is re-derived directly from the
// protocol reference, independently of the production codec in core/cat.
//
// This is not a style preference. If fakeradio reused core/cat's codec,
// a systematic bug in that codec — say, an off-by-one in a field offset,
// or a validation rule that's subtly wrong — would be applied
// identically on both sides of every "send a command, check the reply"
// test the rest of this project ever runs. The bug would never surface:
// the fake would misbehave in exactly the way the buggy codec expects,
// and every end-to-end test would pass anyway. Two independent
// implementations of the same protocol, checked against each other (and
// against golden byte vectors recomputed by hand in tests — never by
// calling fakeradio's own builders), is what makes that class of bug
// visible. TestNoCoreImports (imports_test.go) enforces this with a
// go/parser scan of the package's own source, asserting no
// project-internal import path appears.
//
// # The ASSUMED register
//
// The manual (FT-710 CAT Operation Reference Manual, 2306-C) does not
// fully specify radio behaviour at the CAT protocol's edges — empty
// slots, malformed input, several field semantics on write. Every place
// this package had to guess is listed here, in one place, so a reviewer
// (or M5a/M5b, which will run these exchanges against real hardware and
// correct this package) does not have to hunt through source comments to
// find them all. Each entry also appears as an inline comment next to the
// code that implements it.
//
//  1. Frame accumulator overflow/resync: byte cap ~256; on overflow,
//     reply "?;" once and discard bytes up to and including the next ';'
//     before resuming normal framing. Not documented in the manual — our
//     own bounded-input policy. (parser.go, reassembler)
//
//  2. MR read of an empty (unpopulated) slot answers "?;", not some
//     other, more specific error. HW-CONFIRMED 2026-07-13 (see
//     docs/hardware-notes.md §Empty/out-of-range slots): live probe
//     "MR010;" (M-10, never populated) -> "?;"; "MR100;" (a
//     grammatically well-formed but out-of-inventory slot number) also
//     -> "?;" — the radio's single unattributed NAK covers both cases
//     identically, exactly as this fake models. The manual itself does
//     not document either case; hardware now does. (parser.go, handleMR)
//
//  3. MC-set (recall) of an empty (unpopulated) slot answers "?;",
//     paired with rule 2 for consistency — you cannot recall a channel
//     with no stored data. NOT itself probed at M5a (the live session
//     only exercised the MC READ query, "MC;" -> "MC006;" — see rule
//     below on MC-answer semantics in core/driver/ft710/doc.go); this
//     specific MC-SET-of-an-empty-slot case remains ASSUMED, by analogy
//     with rule 2, pending a session that actually issues writes (M5b).
//     Not documented in the manual. (parser.go, handleMC)
//
//  4. MT read of a NEVER-TOUCHED slot (no MW, no factory image entry, no
//     prior MT-set — key entirely absent from the state map) answers
//     "?;", exactly like MR/MC. HW-CONFIRMED 2026-07-13 (see
//     docs/hardware-notes.md §Empty/out-of-range slots): live probe
//     "MT010;" -> "?;", reproduced BOTH immediately after "MR010;" -> "?;"
//     and as a standalone exchange with no preceding MR on that slot.
//     This OVERTURNS the former ASSUMED design (MT read of an empty slot
//     unconditionally SUCCEEDING with display=0, tag="") — the manual
//     did not document either behaviour; hardware settled it. A slot
//     that DOES exist in the state map — Populated via MW/the factory
//     image, OR tag-only via a prior MT-set (rule 5) — still answers
//     with whatever Tag/TagDisplay it holds even when Populated is
//     false: M5a was read-only and never tested that write-adjacent
//     case, so it remains exactly as ASSUMED as before. (parser.go,
//     handleMT)
//
//  5. MT-set does NOT mark a slot Populated. Only a successful MW write
//     (or the factory image) does. A slot that exists solely because of
//     an MT tag write still answers "?;" to MR and MC-set, but (rule 4,
//     now HW-CONFIRMED for the read GATE, though not for this specific
//     write-then-read sequence, which M5a never exercised) still
//     answers SUCCESSFULLY to MT read, because it IS present in the
//     state map. Tag state and channel-data state are modelled as fully
//     independent. This whole item is write-path and remains ASSUMED —
//     M5a was read-only throughout; verify at M5b. Not documented in the
//     manual. (state.go, MemState.Populated; parser.go, handleMW/handleMT)
//
//  6. MT-set is accepted for EVERY slot kind the manual's MT grammar
//     table allows, including 5xx and EMG. A later, host-side policy may
//     refuse to SEND such a write — that is a different layer's
//     decision. This package models what the radio accepts, not our
//     policy. (parser.go, handleMT via mrReadableSlot)
//
//  7. MT tag charset: printable ASCII 0x20-0x7E, excluding ';' (0x3B —
//     accepting it would make command injection possible), all control
//     bytes rejected, validated by BYTE length <= 12 (not rune count).
//     SAFETY-CRITICAL per the reference, but the manual only says "ASCII
//     code" — the exact radio-accepted charset is unconfirmed. This
//     validator's job today is to make command injection impossible, not
//     to be a definitive oracle for real hardware. ADDITIONALLY,
//     HW-CONFIRMED 13/07/2026 (docs/fixtures-private/
//     m5b-trials.private-capture, stages tagclear/tagclear2;
//     docs/hardware-notes.md §Empty-slot create, tag-clear): a
//     ZERO-byte-tag Set is REJECTED with "?;" (~4 ms) and the existing
//     tag survives — this fake formerly ACCEPTED the 0-byte form and
//     cleared the tag, a proven divergence, now fixed to mirror the
//     radio. The radio's one proven tag-CLEAR mechanism is the
//     all-spaces 12-byte tag (accepted through the normal path;
//     cat.Dialect.BuildMTSet emits exactly that form for an empty tag).
//     (parser.go, validTag and handleMT's zero-byte rejection)
//
//  8. MT read replies echo the stored tag exactly as last written (0-12
//     bytes), never trimmed or padded — a DELIBERATE simplification, not
//     changed by the tag-normalisation fix below. HW-CONFIRMED
//     2026-07-13 (see docs/hardware-notes.md §MT short-form answer): the
//     live M-06 read came back with its (front-panel-set) tag
//     space-padded to the full 12 bytes, proving the radio's MT-answer
//     WIRE FORM carries a fixed-width, space-padded tag field, at least
//     for a front-panel origin tag. What was still ASSUMED after M5a
//     (issued no writes) — whether a tag written via CAT MT-set, as
//     opposed to the front panel, comes back similarly padded — is now
//     ALSO HW-CONFIRMED, the hard way: the first real-radio production
//     write (13/07/2026, docs/fixtures-private/) wrote an unpadded tag
//     via CAT MT-set and read it back padded, which aborted clone's
//     write-verify with a false mismatch until fixed (Fix: tag
//     normalisation). core/cat.Dialect.ParseMTAnswer now TRIMS trailing
//     spaces on parse instead of preserving them verbatim, so the model's
//     canonical tag is never padded regardless of which side (radio or
//     this fake) chose to pad the wire reply. This fake's OWN default
//     behaviour is intentionally UNCHANGED by that fix — it still stores
//     and echoes a tag exactly as last MT-set, whatever length that is —
//     because every other test in this suite depends on that wire-level
//     fidelity staying put; core/clone's
//     TestExecute_LiveBugRepro_UnpaddedTagWriteReadBackPadded reproduces
//     the real radio's padding behaviour for its one specific scenario
//     via a test-local wire interceptor instead, rather than teaching
//     this shared fake a quirk every other caller would then have to
//     account for. (state.go, MemState.Tag; parser.go, handleMT)
//
//  9. MW's P7 (kind) field on write: HW-CONFIRMED 2026-07-13 (M5b write
//     trials against Stuart's real UK FT-710, docs/hardware-notes.md) —
//     kind '0' (VFO) and kind '5' (PMS) are BOTH REJECTED with an
//     immediate "?;", the kind '5' rejection holding even on a PMS slot
//     (the manual's own worked example, and this project's former
//     ASSUMED pairing, implied kind '5' should be accepted there — both
//     WRONG). Every other valid P7 digit ('1'-'4') is stored verbatim,
//     whatever arrives — never corrected, defaulted, or second-guessed;
//     kinds '2'/'3'/'4' were never probed on write, so this fake mirrors
//     exactly what M5b tested, not a blanket "only '1' is accepted"
//     policy the trials did not establish. (parser.go, handleMW,
//     mwKindAccepted)
//
//  10. 60m ("5xx") slot numbering is ASSUMED sequential from 501 (i.e.
//     501, 502, 503, ...); the manual documents 5xx as region-dependent
//     without fixing the numbering. STILL ASSUMED after M5a: Stuart's UK
//     FT-710 has no 5xx bank at all (see item 12 below), so this session
//     could not confirm or refute any numbering scheme. Only relevant to
//     ImageUS (STILL-ASSUMED) until a 60m-bearing radio is characterised.
//     (image.go, sixtyMetreChannel)
//
//  11. 5xx/EMG channels report P7 (kind) = 1 (Memory-like) on MR. The
//     manual's kind table (0-5) has no distinct code for 60m/emergency
//     channels. STILL ASSUMED after M5a, for the same reason as item 10
//     (no 5xx/EMG slots existed on the characterised radio to probe).
//     (image.go, kind60mEMG)
//
//  12. UK factory image (ImageUK): HW-CONFIRMED 2026-07-13 (see
//     docs/hardware-notes.md §60m regional finding) to have NO 60m ("5xx")
//     bank and NO EMG channel at all — front-panel confirmed on Stuart's
//     UK FT-710, no 5-xx channels anywhere in the 117-slot inventory. UK
//     5 MHz operation on this radio lives in ordinary memory channels,
//     not a dedicated bank. This OVERTURNS the former ASSUMED design (7
//     invented placeholder channels, 501-507, at round 20 kHz steps from
//     5.260 MHz — never the real Ofcom-assigned UK 60m channel plan,
//     which this project now knows is not a separate CAT-visible bank on
//     this variant at all). ImageUK is now simply baseImage(), unchanged
//     from any other UK FT-710 unless a future session finds otherwise.
//     (image.go, ImageUK)
//
//  13. US factory image (ImageUS): STILL-ASSUMED, UNCHANGED by M5a (which
//     characterised only a UK-variant radio — see item 12): 60m channels
//     501-515 populated with invented placeholder frequencies; EMG
//     populated at 5.1675 MHz USB, the well-known conventional Alaska
//     emergency frequency, used here only as a plausible placeholder —
//     the exact value a real US-variant radio's CAT interface would
//     report for EMG (channel count, numbering, EMG frequency) remains
//     unverified pending a US-region hardware session. Kept exactly as
//     it was so tests that need 60m/EMG bank coverage still have a
//     fixture — see the M5a repo-consequences commits for where ImageUK's
//     old 60m/EMG callers moved to ImageUS. (image.go, ImageUS)
//
//  14. PMS (P1L-P9U) factory band-edge frequencies are invented, plausible
//     amateur band-edge values — not sourced from any specific programmed
//     radio. (image.go, baseImage)
//
//  15. "000" is never accepted as an explicit request slot for MR, MT, or
//     MC in this fake — it only ever appears inside answers, per the
//     manual's own "(answer only)" annotation. A request naming "000" is
//     treated as a malformed slot ("?;"). (parser.go, slotVFO000 /
//     mrReadableSlot)
//
//  16. Command name case-insensitivity (upper or lower accepted) follows
//     the manual's explicit statement. Every OTHER field value — the mode
//     nibble's hex letters, the PMS L/U suffix, tag charset — is treated
//     as case-SENSITIVE; the manual does not address this. (parser.go,
//     handleFrame; parseSlotForm)
//
//  17. The clarifier magnitude's range and step (0000-9990 Hz, 10 Hz
//     steps) are enforced by the WIRE-level MW-set parser, not merely by
//     an internal builder helper — assumed because the range and step
//     are stated directly in the manual's field description, not just an
//     internal convenience. Whether real hardware actually rejects
//     out-of-range/non-step values (rather than silently rounding them)
//     is unconfirmed. (parser.go, validClarMagDigits)
//
//  18. AI defaults to OFF ('0') when a Radio is constructed, mirroring the
//     manual's "AI resets to OFF at radio power-off" (New() models a
//     freshly powered-on radio). (fakeradio.go, New)
//
//  19. Fault-injection "exchange" counting (the N in FaultDropReplies(N),
//     FaultGarbleReply(N), etc.) counts every ';'-terminated unit handed
//     to command processing, INCLUDING an accumulator-overflow resync
//     event — this is a test-harness convention documented for
//     Fault-index clarity, not a claim about real radio behaviour.
//     (options.go, faultConfig)
//
//  20. MW's clarifier fields (P3 sign+value, P4 RX flag, P5 TX flag) are
//     ACCEPTED but silently IGNORED: HW-CONFIRMED 2026-07-13 (M5b write
//     trials, docs/hardware-notes.md) — live MW frames carrying non-zero
//     clarifier values and flags drew no "?;" rejection, and every
//     readback showed zeros. This fake validates the fields exactly as
//     before (wire-form validity, range/step — item 17 is unchanged;
//     out-of-range values were never probed and remain ASSUMED-rejected)
//     and then stores ZEROS, never the transmitted values. One residual
//     ASSUMPTION inside the confirmed finding: every observed write
//     targeted a channel whose STORED clarifier was already zero, so
//     "ignored" could mean either "preserves the stored value" or
//     "zeroes on every write" — indistinguishable in every observed
//     exchange. This fake stores zeros (per the M5b brief); if a future
//     session ever observes a non-zero STORED clarifier surviving an MW,
//     switch to preserve-stored. The capability model marks
//     FieldClarifier's Write spec.Inert on every profile
//     (core/driver/ft710/caps.go), and codeplug.Diff blocks a CHANGED
//     clarifier before a send ever reaches a radio, so in practice the
//     two interpretations are not reachable-different through this
//     project's own write path. (parser.go, handleMW)
//
//  21. EX (MENU) defaults (EXDefaults(), exDefaults): every numeric item's
//     placeholder is n x '0' and every text item's placeholder is 12
//     spaces — this fake's OWN construction-time convenience, not a claim
//     about any real radio's factory MENU settings. The manual's Table 2
//     documents each item's VALID RANGE, never a shipped default value,
//     so there is nothing to source a real default from; unlike the MR/MW
//     factory image (image.go), no attempt is made here to look
//     plausible. STILL ASSUMED after M8c: that session observed one
//     radio's CURRENT USER settings, which are not factory defaults, so
//     there is still nothing to source a real default from. The M8c
//     overlay (exHardwareOverrides) changes only the WIDTH and SHAPE of
//     what the fake answers, never claiming a value. (ex.go,
//     buildEXDefaults)
//
//  22. EX item WIDTHS (exGroups' widths tokens) are transcribed directly
//     from Table 2's own "Digits" column, with a sign counted in the
//     width (e.g. "-20".."+10" is width 3, matching the Digits column's
//     own "3" for every such item). M8c compared every one of them
//     against a real radio: in two successive full sweeps of one UK
//     FT-710, CAT ID 0800, firmware V01-12, in one configuration, on
//     24/07/2026, the observed READ widths matched this transcription for
//     295 of 296 addresses. The exception is 01 03 21 (MODE FM, TONE
//     FREQ), where Table 2 prints Digits 2 and the radio answered a
//     3-byte P4 ("EX010321012;"): the manual is wrong, and the observed
//     width is supplied by exHardwareOverrides rather than by editing
//     this transcription (see that table's own comment for why).
//     Scope: observed READ widths on one radio/firmware/configuration —
//     not a verified property of the model, and no evidence at all about
//     EX Set frame widths, which M8c did not probe. The interpretive
//     judgement call within the transcription — P1=03/P2=01 (GENERAL)
//     items 20-25 (MIC P1..P4, MIC UP, MIC DOWN) sharing Table 2's "2"
//     Digits annotation as a single merged cell, read here as all six
//     being width 2 — was borne out: all six answered 2-byte P4s in
//     those sweeps. (ex.go, exGroups, exHardwareOverrides)
//
//  23. EX read of a valid-shape but out-of-inventory address (a 6-digit
//     address whose (P1,P2,P3) is not in exGroups, e.g. no P1=05 group,
//     or a P3 beyond a group's item count) answers "?;". This fake
//     applies that behaviour to the WHOLE out-of-inventory space, which
//     remains ASSUMED by analogy with MR's out-of-inventory NAK
//     (register item 2) — but the analogy is no longer the only
//     support: at M8c (24/07/2026) SIX such addresses were put to a real
//     radio and every one answered the same generic "?;" — EX050101,
//     EX050505, EX010199, EX019901, EX079901, EX999999. Six samples, not
//     a survey: the generalisation to every other out-of-inventory
//     address is still this fake's assumption. (ex.go, handleEX)
//
//  24. EX Set is NOT modelled this phase: handleEX rejects every body that
//     is not exactly 6 ASCII digits with "?;", including a well-formed
//     Set body (a valid 6-digit address immediately followed by a raw P4
//     payload, e.g. "EX0301051;") — state is left unchanged. This is a
//     deliberate, phase-scoped modelling gap, not a hardware claim, and
//     is KNOWN-DIVERGENT from the manual's own documented grammar (the
//     "EX" section, lines ~628-642, plainly documents a Set form).
//     Real per-address EX-set behaviour (which addresses are writable at
//     all, what a write does to related items, whether out-of-range P4
//     values are rejected or clamped) is unknown pending hardware
//     evidence; a later milestone implements EX-set once that evidence
//     exists. M8c did NOT provide that evidence: it was read-only by
//     construction (the outbound allowlist accepts EX in the 9-byte read
//     shape only) and probed no Set frame, so this remains an unmodelled
//     gap with no hardware evidence in either direction. (ex.go,
//     handleEX)
//
//  25. P1=06-not-05 anomaly — EVIDENCE AT M8c (24/07/2026), consistent
//     with the reading this fake already followed. The EX grammar's own
//     P1 range note (manual line ~629) reads "P1 : 01 - 04, 05" — five
//     values, the last written as if it were "05" — while Table 2 (line
//     ~904) labels its fifth P1 group "06 EXTENSION SETTING" and shows
//     no P1=05 group at all. Both readings were possible from the text
//     alone, so this fake followed Table 2 (no P1=05 in exGroups, no
//     address with P1=05 ever answers). TWO P1=05 addresses were then
//     put to a real radio, EX050101 and EX050505, and both were
//     rejected with "?;" — consistent with Table 2 being right and the
//     grammar note's "05" being a typo, though two samples do not
//     survey the P1=05 space. Either way this fake's behaviour needs no
//     change, and the reading it already followed is the one the
//     evidence supports. (ex.go, exGroups)
//
// Entries 1-20 above are expected to be regenerated, corrected, or removed
// once M5a captures real CAT transcripts against physical FT-710
// hardware — this register exists so that reconciliation has a single,
// complete list to work from, rather than a source-wide comment hunt.
// Entries 21-25 (EX) were added after M5a, which did not probe EX at all,
// and were RECONCILED at M8c (24/07/2026, the menu hardware
// read-characterisation session — docs/hardware-notes.md): 22 now rests
// on observation across the whole inventory; 23's whole-space rule stays
// ASSUMED with six supporting samples; 25 has two samples consistent with
// the reading this fake already followed; 21 and 24 remain deliberate gaps
// for the reasons each records.
//
// # Real-hardware behaviour observed at M5a (13/07/2026)
//
// The following are not ASSUMED — they are facts recorded live against
// Stuart's UK FT-710 (full detail: docs/hardware-notes.md). They are not
// modelled byte-for-byte by this fake (fakeradio answers every command
// near-instantly and never emits unsolicited pushes), but they justify
// design decisions made elsewhere in this project, most directly
// core/transport.Engine's drain-to-quiet discipline (DrainToQuiet,
// quarantineAfterWrite) and its QuietPeriod/timeout constants.
//
//   - AI1 (Auto Information ON) flood: with the VFO dial spinning
//     continuously for 8 seconds, the radio emitted 879 unsolicited
//     frames (13,994 bytes, ~110 frames/second) with no CAT command sent
//     at all — prefix mix FA (330), RM (272), IF (243), FD (34). This is
//     real, sustained, high-rate chatter of the exact kind
//     core/transport's drain-to-quiet design exists to absorb before
//     trusting a Do call's own answer; AI0; still cleanly restored quiet
//     operation afterwards (verified: "AI;" -> "AI0;" post-flood).
//   - Timing: MR answers landed in 10-11 ms flat over 20 back-to-back
//     zero-settle reads (no inter-command delay, no choke observed); a
//     coalesced 3-command write ("MR001;MR002;MR003;" sent as one burst)
//     produced three clean, correctly-ordered answers in 26 ms total —
//     the radio pipelines back-to-back requests rather than requiring
//     one full round trip before accepting the next. A full 117-slot
//     ReadAll (MR+MT per slot) completed in 4.7 s. These numbers inform
//     the parked "pacing-knob" simulator-timing item; the fake's own
//     constants are unchanged by this task (timing constants are
//     out of scope for M5a's repo consequences — see the task brief).
//
// # Real-hardware behaviour observed at M5b (13/07/2026 evening, write trials)
//
// Also not ASSUMED — write-direction facts recorded live against Stuart's
// UK FT-710 (full detail: docs/hardware-notes.md), modelled by this fake
// where a design decision elsewhere in the project depends on them:
//
//   - MW moves the radio's selection to the written slot, hands-off, with
//     no MC-set involved — confirmed for a single write AND for a bulk
//     sequence of writes (the selection drags through every written
//     channel in turn). Modelled here (parser.go, handleMW) so
//     core/clone's Execute can be tested end-to-end snapshotting and
//     restoring this selection through the REAL driver, not a mock (see
//     core/clone's MemorySelector).
//   - MW timing: accept is silent (fire-and-forget, as already modelled);
//     REJECT is an IMMEDIATE "?;" (~10ms in the live session — this fake
//     already answers near-instantly either way, so no timing constant
//     changed).
//   - Tag preservation, empty-slot MW create, and NO CAT ERASE (a
//     properly isolated 13/07/2026 re-probe — four range/mode-isolated
//     candidate MW frames, every one rejected; see
//     docs/hardware-notes.md's "No CAT erase" section) all matched this
//     fake's existing modelling exactly — recorded here as confirming
//     evidence, not a behavioural change.
package fakeradio

// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftx1 is the Yaesu FTX-1 driver, built on an INLINE
// cat.DialectConfig (dialect.go) rather than a core/cat/ftx1 subpackage —
// the same shape ftdx1200/ftdx3000 use, chosen here because this radio
// needed no core/cat lift beyond what F1 (this milestone's seam phase)
// already added generically: SlotSpace.SlotDigits, PMSFormDashToken,
// MTFormShortNoDisplay, MCSelectsUnsupported and ToneStatesSix
// (core/cat/dialectconfig.go).
//
// NO FTX-1 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. No hardware
// exists to ask, and no owner has run a write trial
// (no-additional-hardware-2026-09-13). Every byte here came from the
// FTX-1 CAT Operation Manual (docs/fixtures-private/manuals/
// ftx1_cat_2508-C.pdf, provenance ftx1-manual-provenance.md) through
// .superpowers/sdd/2026-09-18-v1100-ftx1/reviews/spec.md and
// matrix-ftx1.md, and from nothing else — no Hamlib, no CHIRP, no
// community protocol write-up (Stuart's provenance rule, 12/09/2026).
//
// # This radio's shape, in one paragraph
//
// The FT-710's own 27-byte MR/MW field block (P2-P10), with its address
// field (P1/P0) widened from 3 to 5 bytes — the whole slot space widened
// with it, not just the memory range: 00001-00999 memory, P-01L-P-50U
// PMS (a hyphenated, two-digit, 50-pair token — not the registered
// family's single-digit form), 50001-50020 "5 MHz BAND" (the same
// physical allocation every sibling's 60m bank names), and a named
// EMGCH token rather than a numeric emergency channel. MT is a SHORT
// form like the FT-710's own, but with NO display byte at all — "MT" +
// slot(5) + tag(12, fixed-width, TagFill-padded) + ";" = 20 bytes,
// always. The mode table gains two real new members, C4FM-DN ('H') and
// C4FM-VW ('I') — this codec's first C4FM digital-voice nibbles. The
// tone (P8) domain widens to SIX states, one of them ('3', "DCS")
// UNSPLIT where the FT-991A's own five-state domain splits it, and two
// (PR FREQ, REV TONE) with no analogue anywhere else in this project's
// tone vocabulary. MC is a genuinely different frame shape (a leading
// per-port byte before the slot) that nothing in core/cat can build or
// parse — this driver declares MCSelectsUnsupported and never uses MC at
// all: reads go via MR+MT only, exactly as the milestone plan scopes.
//
// # Write posture
//
// EVERY WRITABLE FIELD IS spec.Unverified, consent-gated
// (spec.ConsentedUnverified, via WithConsentedUnverifiedWrites), NEVER
// enabled by default (spec.md §11) — writeTrialsComplete (caps.go) is
// false and has never been anything else. The 5 MHz and EMGCH banks are
// additionally write-Unsupported on EVERY profile, consent included:
// cat.Dialect.writableSlot structurally excludes them from MW regardless
// of consent, the same treatment the FT-710's discovered 60m/EMG banks
// get.
//
// # Firmware floor
//
// "The CAT operation does not work with MAIN Firmware before Ver. 1.08"
// (spec.md §2, the manual's own opening "Important Notes"). This is a
// DOCUMENTED PRECONDITION, not an enforced one: the wire carries no
// firmware-version byte any command here could fail on, and a
// pre-V1.08 radio simply does not answer CAT at all — which Open's own
// ID-probe failure already treats as "no radio present". No code exists
// to check it, deliberately; this paragraph is the whole of its
// enforcement.
//
// # Register — every ASSUMED value and OPEN question this driver carries
//
//  1. Body identity ("field" vs "optima") — UNKNOWABLE over CAT, and
//     never guessed at. CATID "0840" (dialect.go) is shared, unmodified,
//     by both bodies (spec.md §1); the AC command's internal-tuner
//     answer is the nearest proxy, and it is a capability probe, not an
//     identity byte. Nothing in core/spec.Capabilities carries a body
//     axis, and this milestone does not add one.
//
//  2. MC/MT channel range ships 999, the manual's own printed "099" on
//     MC's/MT's P0/P2 legends taken as an erratum per the roadmap's
//     ruling (spec.md §10) — re-confirmed directly against the manual
//     while writing spec.md, not taken on the roadmap's word alone.
//     Which range a real radio actually accepts past channel 99 is OPEN
//     (matrix §2, probe 2).
//
//  3. The "5 MHz BAND" ships 50001-50020 (the majority reading — four of
//     the manual's own five printed variants agree; spec.md §10). MC's
//     own "50000-50020" and MT's own "50001-50009" are footnoted, not
//     shipped. Which channels a real radio actually stores/recalls is
//     OPEN (matrix §2, probe 3).
//
//  4. dialect.go's MT.TagFill (' ') is ASSUMED — the FT-710's own value,
//     unconfirmed for the FTX-1: the manual gives no worked example of
//     an empty tag (spec.md §8).
//
//  5. dialect.go's Clarifier.StepHz (10 Hz) is ASSUMED: the manual
//     states the range (0000-9990 Hz), not the step granularity
//     (spec.md §7) — the same gap every registered dialect's own
//     ClarifierPolicy carries.
//
//  6. dialect.go's MWWriteKind (cat.KindMemory) is ASSUMED, by analogy
//     with the FT-710's own HW-CONFIRMED finding (the radio requires
//     KindMemory on EVERY MW write, regardless of bank) — never itself
//     confirmed for the FTX-1. Whether a real MW genuinely accepts the
//     full 0-5/PMS enum its own P7 legend prints is OPEN (matrix §2,
//     probe 4; spec.md §12).
//
//  7. dialect.go's EXAddressForm (cat.EXAddressTriple) is an ASSUMED
//     PLACEHOLDER, not a fact about this radio: EX/GT/VM are all
//     deferred this milestone (spec.md §10) — EXItems stays nil, and
//     this driver's read path never issues an EX command at all — but
//     NewDialect's own V12 refuses the zero EXAddressForm even for an
//     empty EXItems (it also sizes the EX read frame the outbound gate
//     measures). The value is inert either way.
//
//  8. caps.go's Bauds/DefaultBaud describe CAT-1/CAT-3's factory rate
//     (38400) only. The manual documents PER-PORT rates (CAT-2's own
//     factory default is 4800, matrix §1/§6) — wider than
//     spec.Capabilities' single-axis Bauds/DefaultBaud field. OPEN for a
//     future per-port capability axis, not resolved here.
//
//  9. caps.go's MinFreqHz/MaxFreqHz stay 0 (no bound): no general-
//     coverage tuning-range statement, and no CTCSS tone-frequency
//     chart, was located in the manual excerpts this spec pass read
//     (matrix §6) — both OPEN.
//
//  10. dialect.go's modeNames excludes 'G' and 'J': the manual prints
//     both "-", the same placeholder '0' (cat.ModeUnset) uses — ASSUMED
//     reserved/unused nibbles on documentation-depth grounds alone, not
//     a printed statement either way (spec.md §5, open question 3).
//
// See .superpowers/sdd/2026-09-18-v1100-ftx1/reviews/spec.md and
// matrix-ftx1.md for the full citation trail behind every item above.
//
// # Out of scope (deliberately)
//
// MC as a live command (dialect.go declares MCSelectsUnsupported; a
// future seam would need a genuinely new frame shape carrying a leading
// per-port byte — spec.md §3.4). EX/GT settings surface. VM (a two-way
// opcode collision on this radio between a Set-only trigger and a
// genuine Set/Read/Answer command — spec.md §10; this driver's read path
// never issues it, so the collision is inert here). Settings write (a
// separate, parked roadmap line). Any body-identity mechanism (register
// item 1 — none exists on the wire).
package ftx1

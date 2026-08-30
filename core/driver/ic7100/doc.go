// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7100 implements safe IC-7100 memory cloning over CI-V.
//
// The driver is manual-derived and unverified on hardware. RealHardware is
// therefore fail-closed unless the user explicitly consents to unverified
// writes. Erase is never consented, writes are never retransmitted, and Open
// sends no mutation. The package is deliberately not registered here.
//
// CI-V serial framing is not reported. The sole 8-N-1 sentence in the manual
// is PDF p.174's DV low-speed data application, not the CI-V/REMOTE link;
// TestNewProfilesFailSafeAndDoNotExposeSerialFraming pins that distinction.
//
// # The printed 50-tone chart, and why it is not the declared domain
//
// PDF p.91 (folio 4-26) prints "• Selectable tone frequencies (Unit: Hz)",
// a 50-value table running 67.0 to 254.1 — the family-standard CTCSS set,
// and what this radio's PANEL offers. It is recorded here, as prose, and
// not in spec.Capabilities.
//
// The record does not index that table. The three-byte tone spans hold a
// BCD FREQUENCY, whose printed per-digit legend admits 000.0-299.9 Hz, so
// caps.go declares that capacity ({1, 2999, 1}, the floor raised off zero
// because 0 Hz is not a tone) rather than the chart's bounds. Declaring
// the chart would fail closed on every encodable value outside it: a tone
// between 254.2 and 299.9 Hz would read Unknown and become unwritable,
// whilst the same wire bytes round-trip on every sibling driver.
//
// This is the tier's recorded doctrine and not a decision taken here. The
// IC-7300 met the identical artefact and settled it the same way
// (core/driver/ic7300/caps.go:242-251), which landed as IC-7300 matrix
// erratum 12; core/driver/ic7760 and the other eight declare the same
// shape. TestCTCSSToneDomainAdmitsEveryChartTone still pins that every one
// of the 50 printed tones is admitted — now trivially, by capacity, which
// is the point: the chart is a subset of what the record can carry, and
// what the RADIO accepts off-chart remains register entry
// ic7100-tone-range-step's open question.
//
// # Wave-4 hand-off: the 111-byte record-length set
//
// THIS MODEL IS NOT SEPARABLE FROM THE IC-9700 BY RECORD GEOMETRY ALONE, and
// Wave 4 inherits that fact rather than a solution to it. Both were checked
// in this worktree at the time of writing:
//
//   - IC-7100: 111-byte record-only length, three address bytes,
//     civ.AddressFormBankChannel (bank 01–05 plus a two-byte channel).
//   - IC-9700: 111-byte record-only length, three address bytes,
//     civ.AddressFormBandChannel (core/civ/ic9700/profile.go).
//   - IC-705: 111-byte record-only length too, but FOUR address bytes,
//     civ.AddressFormWideGroupChannel (core/civ/ic705/profile.go) — so it is
//     separable on the wire's own geometry, and the other two are not.
//
// The consequence for a probe is exact: measuring 111 record bytes proves the
// radio is not an IC-7610 or an IC-7300, and proves NOTHING about whether it
// is an IC-7100 or an IC-9700. Both lengths are moreover ASSUMED derivations
// from printed field widths — this one is register entry ic7100-record-length
// — so even a table separating them would be provisional until one of them is
// measured against a radio.
//
// WHAT THIS PACKAGE THEREFORE DOES. The probe refuses a foreign record length
// with a driver.WrongRadioError carrying the two RECORD-ONLY lengths and NO
// model name. A name appears only when tier integration injects one through
// WithSiblingRecordLengths, and the error then says the attribution is
// PROVISIONAL in the same sentence.
//
// WHAT THIS PACKAGE DOES NOT CLAIM. There is no tier-wide record-shape
// distinctness check here, no declared table of every model's accepted
// lengths, and no TestTierRecordShapes_DistinctOrDeclared: that check and its
// declared-indistinguishable table are Wave-4's, and this package neither
// implements nor anticipates their shape. Registration is Wave-4's too —
// nothing here adds a row to internal/wiring, to a guard table, or to the CLI.
package ic7100

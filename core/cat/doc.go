// SPDX-License-Identifier: GPL-3.0-or-later

// Package cat implements the Yaesu CAT protocol codec (frame building and
// parsing) for the FT-710 family. Pure functions, no I/O.
//
// # The policy-gated write path (composition-root discipline)
//
// BuildMWSet, BuildMTSet and BuildMTSetCombined are MECHANISM: they encode
// the Set frames that mutate a radio's memory — the memory record, and the
// channel tag in each of the two evidenced MT frame forms (the combined one
// since M9c-3, mtcombined.go) — validating wire grammar and per-field
// safety (charset, ranges, slot writability) — but they know nothing of
// the hardware write guard's POLICY layers (capability profiles,
// codeplug.Diff's gates, the clone service's choreography,
// driver.Session.WriteChannel's re-check), which live entirely above
// them. Within THIS repository, these builders are therefore used
// outside the core/cat/** tree only from core/driver/** — enforced by the
// import-graph guard test (internal/guards), whose carve-out is the
// core/cat tree by prefix; the one sanctioned in-tree consumer outside
// this package is core/cat/dialecttest, the conformance suite, which
// builds frames only under a *testing.T and reaches no transport. The
// guard's threat model is our own composition, not external importers. The compiler-enforced version
// of this boundary (a separate write-capability split) is a ledgered
// M5b-flip precondition.
//
// # Slot-domain refusals name only the banks their dialect declares
//
// The MW and MT slot refusals used to spell "memory 001-099", "P1L-P9U"
// and "5xx/EMG" as literals. Since S0.2 they are composed from the
// receiver's own slot space (Dialect.mwSlotDomainRefusal and
// mtSlotDomainRefusal, slot.go), because all three are false on a radio
// whose PMS pairs are decimal channel numbers and which has neither a 5 MHz
// bank nor an emergency channel — a shape the FT-991A's manual prints. A
// diagnostic quoting the old text would have told such a radio's owner it
// had banks awaiting hardware verification that it does not have.
//
// THE M5a POLICY CITATION IS UNCHANGED WHEREVER A RADIO HAS THE BANKS THAT
// POLICY GOVERNS. It appears in the MT sentence exactly when the dialect
// declares a 5 MHz bank or an emergency channel, which is every dialect
// registered today, so all four render byte-for-byte what they always did
// and core/cat/testdata/frame-corpus.golden does not move. On a dialect
// with neither bank the clause is absent.
//
// THE NONE FORM IS DERIVED TOO. The sentences quoted "000" as a literal
// until the Stage 0 close review (seat 2, MEDIUM-3) measured it against
// noneWireDialect, whose none form is "900" and for which "000" is an
// ordinary writable memory channel: the MW sentence offered "memory
// 000-005" and rejected "000" in the same breath. It now comes from
// slotSpace.noneWire through Dialect.noneFormText, and is omitted entirely
// by a dialect that declares no none form. Every registered dialect
// declares NoneWire "000", so this too moves no byte —
// TestSlotDomainRefusals_EveryRegisteredDialectIsByteIdentical
// (core/transport) is the measurement, over all five, rather than the
// claim.
package cat

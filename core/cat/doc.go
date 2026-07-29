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
package cat

// SPDX-License-Identifier: GPL-3.0-or-later

// Package cat implements the Yaesu CAT protocol codec (frame building and
// parsing) for the FT-710 family. Pure functions, no I/O.
//
// # The policy-gated write path (composition-root discipline)
//
// BuildMWSet and BuildMTSet are MECHANISM: they encode the two Set frames
// that mutate a radio's memory, validating wire grammar and per-field
// safety (charset, ranges, slot writability) — but they know nothing of
// the hardware write guard's POLICY layers (capability profiles,
// codeplug.Diff's gates, the clone service's choreography,
// driver.Session.WriteChannel's re-check), which live entirely above
// them. Within THIS repository, these two builders are therefore used
// outside this package only from core/driver/** — enforced by the
// import-graph guard test (internal/guards), whose threat model is our
// own composition, not external importers. The compiler-enforced version
// of this boundary (a separate write-capability split) is a ledgered
// M5b-flip precondition.
package cat

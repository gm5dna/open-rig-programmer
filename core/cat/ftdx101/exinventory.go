// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ftdx101

// The generated exinventory_gen.go declares exactly one identifier:
//
//	var exItems []cat.EXItem
//
// the FTdx101D/MP's EX address inventory, sorted by (P1,P2,P3) and derived
// from table2.csv by the directive above. It is this package's ONLY access to
// the inventory, and from M9d-1 task 6 it is consumed in exactly one place —
// dialect.go's DialectConfig literal, which hands it to cat.MustNewDialect as
// EXItems. Everything else reaches those items through the built Dialect
// (Dialect().EXItems(), KnownEXAddress, ParseEXAnswer's width bound), never
// through the variable, so the inventory has one owner rather than several
// readers of a package-level slice.
//
// ONE inventory, TWO models. The FTDX101MP and the FTDX101D share one printed
// MENU Chart, and every property this inventory stores is printed identically
// for both (see table2.csv's header and the ledger's applicability
// attestation). Where the two radios do differ — the ID answer's value, and
// the P4 VALUE ranges of the three MAX POWER rows — the difference is not a
// field of an EXItem, so it is carried by the per-model dialect and by the
// audit-only P4 column respectively, not by a second generated file. Two
// generated inventories differing in nothing would be two things to keep in
// step for no evidence gained.
//
// Nothing else is declared here. This file exists for the directive and for
// that note: a second identifier next to a generated variable is how a
// hand-edit of that variable eventually gets rationalised. Regenerate with
// `go generate ./core/cat/ftdx101`; the package's staleness test refuses a
// generated file that has drifted from table2.csv.

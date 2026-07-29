// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ftdx10

// The generated exinventory_gen.go declares exactly one identifier:
//
//	var exItems []cat.EXItem
//
// the FTdx10's EX address inventory, sorted by (P1,P2,P3) and derived from
// table2.csv by the directive above. It is this package's ONLY access to the
// inventory, and it is consumed in exactly one place — dialect.go's
// DialectConfig literal, which hands it to cat.MustNewDialect as EXItems.
// Everything else reaches those items through the built Dialect
// (Dialect().EXItems(), KnownEXAddress, ParseEXAnswer's width bound), never
// through the variable, so the inventory has one owner rather than several
// readers of a package-level slice.
//
// Nothing else is declared here. This file exists for the directive and for
// this note: the FT-710's core/cat/exinventory.go carries its EXAddress and
// EXItem types alongside its own directive because those types live in
// core/cat, but they are core/cat's to define, and a second model package
// declaring anything of its own next to a generated variable is how a
// hand-edit of that variable eventually gets rationalised. Regenerate with
// `go generate ./core/cat/ftdx10`; the package's staleness test refuses a
// generated file that has drifted from table2.csv.

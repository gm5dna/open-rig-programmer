// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ft891

// The generated exinventory_gen.go declares exactly one identifier:
//
//	var exItems []cat.EXItem
//
// the FT-891's EX address inventory, sorted by (P1,P2) — every item's P3 is
// 0, because this radio's EX address is a PAIR and its wire field is the
// four digits of the chart's MENU Number — and derived from table2.csv by
// the directive above. It is this package's ONLY access to the inventory,
// and it is consumed in exactly one place: dialect.go's DialectConfig
// literal, which hands it to cat.MustNewDialect as EXItems. Everything else
// reaches those items through the built Dialect (Dialect().EXItems(),
// KnownEXAddress, ParseEXAnswer's width bound), never through the variable,
// so the inventory has one owner rather than several readers of a
// package-level slice.
//
// Nothing else is declared here, and that is a RETURN to the siblings'
// rule rather than an inheritance of it. Until dialect.go existed this file
// carried an exported EXItems() accessor, because the generated variable
// had no in-package consumer at all and the cross-check task had to reach
// the inventory before the dialect could. The departure was documented as
// temporary and is now closed: a second identifier beside a generated
// variable is how a hand-edit of that variable eventually gets rationalised
// (core/cat/ftdx10/exinventory.go's rationale), and the copy-returning
// accessor's one real claim is a property cat.Dialect already provides.
// Regenerate with `go generate ./core/cat/ft891`; the package's staleness
// test refuses a generated file that has drifted from table2.csv.

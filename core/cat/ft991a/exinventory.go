// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ft991a

// The generated exinventory_gen.go declares exactly one identifier:
//
//	var exItems []cat.EXItem
//
// the FT-991A's EX address inventory, sorted by (P1,P2,P3) — every item's P2
// and P3 are 0, because this radio's EX address is a SINGLE component and its
// wire field is the three digits of the chart's MENU Number — and derived
// from table2.csv by the directive above.
//
// IT IS 152 ITEMS FOR A 153-ROW CHART. Menu 087 RADIO ID is transcribed and
// counted in table2.csv, because the chart prints it, and omitted here by the
// ft991a profile's ParameterlessAddresses: its parameter column is ten
// hyphens and its Digits cell one, so it names no field an EX frame could
// read or write. The generated file's own header records the omission by
// address, so a reader of the artefact alone can see which row is missing and
// why.
//
// Nothing else is declared here. That is the siblings' rule
// (core/cat/ft891/exinventory.go, core/cat/ftdx10/exinventory.go): a second
// identifier beside a generated variable is how a hand-edit of that variable
// eventually gets rationalised. The variable has NO IN-PACKAGE CONSUMER YET —
// this package is the inventory and nothing else so far — and it stays
// unexported regardless: the consumer this milestone is building towards is a
// DialectConfig literal in this same package, which reaches it directly.
//
// Regenerate with `go generate ./core/cat/ft991a`; the package's staleness
// test refuses a generated file that has drifted from table2.csv.

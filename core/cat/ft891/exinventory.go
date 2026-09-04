// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import cat "github.com/gm5dna/open-rig-programmer/core/cat"

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ft891

// The generated exinventory_gen.go declares exactly one identifier:
//
//	var exItems []cat.EXItem
//
// the FT-891's EX address inventory, sorted by (P1,P2) — every item's P3 is
// 0, because this radio's EX address is a PAIR and its wire field is the
// four digits of the chart's MENU Number — and derived from table2.csv by
// the directive above. It is this package's ONLY access to the inventory.
// Regenerate with `go generate ./core/cat/ft891`; the package's staleness
// test refuses a generated file that has drifted from table2.csv.
//
// EXItems is the one identifier this file adds, and it is a DEPARTURE from
// core/cat/ftdx10/exinventory.go and core/cat/ftdx101/exinventory.go, which
// declare nothing beside the directive on the stated ground that a second
// identifier next to a generated variable is how a hand-edit of that
// variable eventually gets rationalised. The departure is deliberate and
// narrow: those two packages have a dialect.go whose DialectConfig literal
// consumes exItems in-package, and this one does not yet — the dialect is a
// later task — so without an accessor the generated variable would have no
// consumer at all and the inventory would be unreachable to the tasks that
// must check it before the dialect exists. It returns a COPY, pinned by
// TestEXItems_ReturnsACopy, so a caller cannot reach into the generated
// slice; the underlying variable stays unexported and stays the single
// owner.
func EXItems() []cat.EXItem {
	out := make([]cat.EXItem, len(exItems))
	copy(out, exItems)
	return out
}

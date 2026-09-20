// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import "fmt"

// WriteBoundaryRow is one EX address admitted to this driver's write gate
// (post denylist/held filter), in plain, core/cat-free form — the shape
// "rigprog settings write-boundary" dumps for the out-of-repo Session W
// bench tool (cmd/rigprog/writeboundary.go). The fields mirror the three
// facts core/cat's own write gate consults (core/cat/ex.go's
// exSetP4OK) plus the manual read width.
type WriteBoundaryRow struct {
	ID               string
	ReadWidth, Width int
	Codes            []int
	Lo, Hi, Step     int
	Signed           bool
}

// WriteBoundaryRows walks this driver's full EX inventory in inventory
// order and returns one row per address catDialect.EXWriteDescriptor
// reports admitted — the post-denylist/held-filter set core/cat's own
// write gate uses, never re-derived by hand here.
//
// Lives here, not in cmd/rigprog (where it started, task h2), because
// building a row needs catDialect.EXItems() and EXWriteDescriptor, both
// core/cat-typed: cmd/rigprog and app/ must never import core/cat or any
// package beneath it (internal/guards' composition-root discipline). This
// is the same route "rigprog settings" itself already uses for a driver's
// static settings tree (SettingsDescriptor below, reached from cmd/rigprog
// via internal/wiring rather than a direct import).
func WriteBoundaryRows() []WriteBoundaryRow {
	items := catDialect.EXItems()
	rows := make([]WriteBoundaryRow, 0, len(items))
	for _, it := range items {
		domain, width, admitted := catDialect.EXWriteDescriptor(it.Addr)
		if !admitted {
			continue
		}
		rows = append(rows, WriteBoundaryRow{
			ID:        fmt.Sprintf("%02d%02d%02d", it.Addr.P1, it.Addr.P2, it.Addr.P3),
			ReadWidth: it.Digits,
			Width:     width,
			Codes:     domain.Codes,
			Lo:        domain.Lo,
			Hi:        domain.Hi,
			Step:      domain.Step,
			Signed:    domain.Signed,
		})
	}
	return rows
}

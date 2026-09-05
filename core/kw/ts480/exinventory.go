// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ts480

import "github.com/gm5dna/open-rig-programmer/core/kw"

// THIS FILE HOLDS AN ACCESSOR ONLY — no var declaration of any kind. See
// core/kw/ts590/exinventory.go for the whole reasoning: the generated file
// declares the variable at its own path, so generation REPLACES the
// bootstrap declaration rather than joining it, and the two lanes'
// declaration surfaces are disjoint by construction.
//
// THE `go:generate` DIRECTIVE ABOVE CANNOT SUCCEED UNTIL TASK 9 REGISTERS
// THE ts480 PROFILE. Regenerate with `go generate ./core/kw/ts480`.

// EXItems returns the TS-480 menu inventory: the rows of the TS-480's EX
// parameter list (480:424-539), whose declared domain is 000 ~ 060
// (480:401), transcribed into menu480.csv and generated into
// exinventory_gen.go.
//
// A COPY, on every call: the generated variable is package-level state and
// a caller that got the slice itself could rewrite the menu table every
// later caller then reads. kw.CopyEXItems is where that copy's behaviour is
// pinned, with real data.
//
// THE NAME IS PART OF THE FILE SPLIT'S CONTRACT: lane P must not touch it,
// and task 8's layout value and task 10's cross-check consume it as
// written. It is EXItems and not EXItems480 because the package clause
// already says which radio this is.
func EXItems() []kw.EXItem { return kw.CopyEXItems(exItems480) }

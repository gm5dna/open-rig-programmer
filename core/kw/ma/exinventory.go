// SPDX-License-Identifier: GPL-3.0-or-later

package ma

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ts890s
//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ts990s

import "github.com/gm5dna/open-rig-programmer/core/kw"

// THIS FILE HOLDS ACCESSORS ONLY — no var declaration of any kind, and that
// is the file split, not a style.
//
// internal/extable's RenderGo emits "var <VarName> = []kw.EXItem{...}" into
// the profile's own OutFile. If this file also declared those variables, the
// package would end up with TWO declarations of one variable after
// generation and would not compile; and the escape — deleting the
// placeholder — would be a lane P edit to a lane K file. Putting the empty
// declarations in the GENERATED files' own paths instead means generation
// REPLACES them, and the two lanes' declaration surfaces are disjoint by
// construction rather than by instruction.
//
// So: this file is lane K's outright and lane P never opens it;
// exinventory890s_gen.go and exinventory990s_gen.go are lane P's from the
// commit that created them and lane K never reads their contents.
//
// TWO INVENTORIES IN ONE PACKAGE, AND HERE THEY ARE TWO DIFFERENT BOOKS'
// CHARTS RATHER THAN ONE BOOK'S TWO LISTS. The TS-890S chart is printed in
// the TS-890S PC command reference and the TS-990S chart in the TS-990S's,
// and they are not the same chart and not the same size. Reusing either
// table for the other would publish wrong setting names, wrong widths and a
// menu domain the radio does not have. internal/extable permits two profiles
// in one package because validateRegistry keys its collision checks on
// Package + OutFile, and the two OutFiles differ.
//
// NO ROW COUNT APPEARS IN THIS FILE, DELIBERATELY. Each row's count is its
// own registered profile's ExpectedRows, asserted by that row's own
// staleness test against a fresh render of its own CSV; a figure written
// here would be a second copy of it, and the transcription legs that produce
// those charts are quarantined from every count they did not derive.
//
// MEMBERSHIP, NOT A SCALAR BOUND, IS WHAT THESE INVENTORIES ARE FOR HERE, and
// that is the difference from core/kw's three older rows. core/kw bounds an
// EX read with Layout.maxEXAddress because pair 1's books print a CONTIGUOUS
// menu domain ("000 ~ 087", "000 ~ 099", "000 ~ 060"); neither of these books
// prints a contiguous domain at all — both charts are sparse, with gaps
// inside every category — so a scalar bound would admit an address no chart
// prints, breaching the standing rule that every frame this programme sends
// is one the book describes. core/kw could not have asked its own inventories
// this question (core/kw/ex.go says why: its rows' inventories live in
// packages that IMPORT it, so a membership check there would be an import
// cycle). Here both layouts and both inventories live in one package, so
// Layout references the generated variable directly — no cycle, no copy, one
// datum, asked by the builder, the parser and the outbound gate alike.
//
// THE `go:generate` DIRECTIVES ABOVE CANNOT SUCCEED UNTIL THE TWO PROFILES
// ARE REGISTERED (lane P). They are written now because this is the file
// that carries them on every existing model package, and because a directive
// that arrives with the accessors is one nobody has to remember to add.
// Regenerate with `go generate ./core/kw/ma`.

// EXItems890S returns the TS-890S menu inventory: the rows of that book's
// "EX Command Parameter Lists", transcribed into menu890s.csv and generated
// into exinventory890s_gen.go.
//
// A COPY, on every call. The generated variable is package-level state and a
// caller that got the slice itself could reorder or rewrite the menu table
// every later caller then reads. kw.CopyEXItems is where that copy's
// behaviour is pinned, with real data, for the reason its doc comment gives.
//
// THE NAME IS PART OF THE FILE SPLIT'S CONTRACT: lane P must not touch it,
// and the layout values below and the cross-check consume it as written.
func EXItems890S() []kw.EXItem { return kw.CopyEXItems(exItems890S) }

// EXItems990S returns the TS-990S menu inventory: the rows of that book's
// "EX Command Parameter Lists", transcribed into menu990s.csv and generated
// into exinventory990s_gen.go.
//
// A copy, and a fixed name, on EXItems890S's terms exactly. The two
// accessors are separate functions over separate variables because the two
// charts are separate charts, in separate books; a single accessor taking a
// row argument would put the two tables one typo apart, which is the
// cross-model borrowing the Tier 4b sweep exists to forbid.
func EXItems990S() []kw.EXItem { return kw.CopyEXItems(exItems990S) }

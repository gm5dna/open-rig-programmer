// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ts590s
//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ts590sg

import "github.com/gm5dna/open-rig-programmer/core/kw"

// THIS FILE HOLDS ACCESSORS ONLY — no var declaration of any kind, and that
// is the file split, not a style.
//
// internal/extable's RenderGo emits "var <VarName> = []kw.EXItem{...}" into
// the profile's own OutFile. If this file also declared those variables,
// each package would end up with TWO declarations of one variable after
// generation and would not compile; and the escape — deleting the
// placeholder — would be a lane P edit to a lane K file. Putting the empty
// declarations in the GENERATED files' own paths instead means generation
// REPLACES them, and the two lanes' declaration surfaces are disjoint by
// construction rather than by instruction.
//
// So: this file is lane K's outright and lane P never opens it;
// exinventory590s_gen.go and exinventory590sg_gen.go are lane P's from the
// commit that created them and lane K never reads their contents.
//
// TWO INVENTORIES IN ONE PACKAGE, DELIBERATELY. The TS-590S and the
// TS-590SG do not share a menu table: the book prints two separate lists
// (590:564 for the S, 590:744 for the SG) over COLLIDING addresses with
// different meanings — the SG's is the S's shifted by two with a new
// read-only row at the top, so address 000 is Display brightness on the S
// (590:569) and Firmware Version, read only, on the SG (590:749), and every
// address above it differs too. Reusing either table for the other would
// publish wrong setting names, wrong widths and wrong read-only status, and
// would let an EX read reach 088-099 on an S, outside its printed domain.
// internal/extable permits two profiles in one package because
// validateRegistry keys its collision checks on OutFile, VarName and
// ManualCSV, all three of which differ here.
//
// THE `go:generate` DIRECTIVES ABOVE CANNOT SUCCEED UNTIL TASK 9 REGISTERS
// THE TWO PROFILES. They are written now because this is the file that
// carries them on every existing model package, and because a directive
// that arrives with the accessors is one nobody has to remember to add.
// Regenerate with `go generate ./core/kw/ts590`.

// EXItemsS returns the TS-590S menu inventory: the rows of "EX Command
// Parameter List (for TS-590S)" (590:564-743), transcribed into
// menu590s.csv and generated into exinventory590s_gen.go.
//
// A COPY, on every call. The generated variable is package-level state and
// a caller that got the slice itself could reorder or rewrite the menu
// table every later caller then reads. kw.CopyEXItems is where that copy's
// behaviour is pinned, with real data, for the reason its doc comment
// gives.
//
// THE NAME IS PART OF THE FILE SPLIT'S CONTRACT: lane P must not touch it,
// and task 8's layout values and task 10's cross-check consume it as
// written.
func EXItemsS() []kw.EXItem { return kw.CopyEXItems(exItems590S) }

// EXItemsSG returns the TS-590SG menu inventory: the rows of "EX Command
// Parameter List (for TS-590SG)" (from 590:744), transcribed into
// menu590sg.csv and generated into exinventory590sg_gen.go.
//
// A copy, and a fixed name, on EXItemsS's terms exactly. The two accessors
// are separate functions over separate variables because the two charts are
// separate charts; a single accessor taking a row argument would put the
// two tables one typo apart, which is the cross-model borrowing the Tier 4b
// sweep exists to forbid.
func EXItemsSG() []kw.EXItem { return kw.CopyEXItems(exItems590SG) }

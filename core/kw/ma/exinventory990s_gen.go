// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import kw "github.com/gm5dna/open-rig-programmer/core/kw"

// exItems990S is the TS-990S's EX menu inventory.
//
// THIS FILE IS A BOOTSTRAP PLACEHOLDER. It exists so that the package
// compiles before the TS-990S extable profile is registered and its
// transcription exists, and it holds an EMPTY inventory on purpose: an empty
// membership set refuses every EX address, which is the closed direction.
//
// IT IS LANE P'S FILE FROM THIS COMMIT ONWARD. `go generate` OVERWRITES IT
// WHOLESALE from menu990s.csv — the header above will be replaced by the
// generator's own "Code generated ... DO NOT EDIT" line and this comment
// will be gone. IF YOU ARE READING THIS COMMENT IN A MERGED BRANCH,
// GENERATION HAS NOT RUN: the row's staleness test asserts the inventory's
// length equals its profile's ExpectedRows, and the cross-check re-asserts
// it, so a bootstrap file cannot ship.
//
// Regenerate with `go generate ./core/kw/ma`; do not edit by hand.
var exItems990S = []kw.EXItem{}

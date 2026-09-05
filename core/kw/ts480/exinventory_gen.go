// SPDX-License-Identifier: GPL-3.0-or-later

// BOOTSTRAP PLACEHOLDER — NOT GENERATED, AND NOT LANE K'S FILE.
//
// This file holds the empty declaration of exItems480 so that package ts480
// COMPILES between task 5, which created it, and lane P's task 9c, which
// transcribes the TS-480 EX parameter list (480:424-539) into menu480.csv
// and generates this file from it.
//
// IT IS LANE P'S FILE FROM THIS COMMIT ONWARD. `go generate` OVERWRITES IT
// WHOLESALE — internal/extable's RenderGo emits the whole file, at this
// exact path, including its own "Code generated ... DO NOT EDIT" header —
// so the declaration is REPLACED rather than joined, and there is never a
// moment with two declarations of one variable.
//
// IF YOU ARE READING THIS COMMENT IN A MERGED BRANCH, GENERATION HAS NOT
// RUN. The inventory is empty, the menu table this package would publish is
// empty, and that is a defect. Lane P's own staleness test asserts this
// inventory's length equals the ts480 profile's ExpectedRows, and task 10
// re-asserts all three, so an ungenerated or half-generated bootstrap file
// fails loudly rather than presenting an empty menu table as a valid one.
//
// Nothing on lane K reads this file's contents.

package ts480

import kw "github.com/gm5dna/open-rig-programmer/core/kw"

var exItems480 = []kw.EXItem{}

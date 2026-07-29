# M9c-4 task 1 — the red-proof record

**Date:** 29/07/2026
**Branch:** `m9c4-ftdx10-dialect`
**Task:** M9c-4 task 1, the four closures
**Plan:** `docs/superpowers/plans/2026-07-29-m9c4-ftdx10-dialect.md`
**Spec:** `docs/superpowers/specs/2026-07-29-m9c4-ftdx10-dialect-design.md`

## Why this file exists

Three of task 1's four closures are GUARD changes, and a guard that has
never been seen to fail is indistinguishable from a guard that cannot
fail. Two of them make the point sharper than usual:

- **Rule 3 is a forbidden-import rule.** It has no legitimate positive
  call site anywhere in the repository, so it can carry no non-vacuity
  counter of the kind the other rules in
  `composition_imports_test.go` use — there is nothing true to count. An
  explicit transient decoy, recorded failing, is the only proof of teeth
  available (Codex 8).
- **The fence carve-out and Rule 2 were NARROWED and WIDENED
  respectively.** In both cases the change is invisible to every existing
  green test: the fence was already green with the prefix carve-out, and
  Rule 2 was already green matching the bare path exactly. Only a decoy
  distinguishes the new behaviour from the old.

Each proof below records, verbatim: the decoy source, the command, the
exact failing output, the deletion, and the green re-run. **No decoy
survives in the working tree** — `git status` is clean of them, verified
at the end of this file.

Line numbers in the recorded output are those of the guard files as
committed by this task.

---

## Proof (a) — the Set-builder fence fires on a `core/cat` subpackage

**Closure:** narrow the fence's carve-out from the `core/cat` prefix to
exactly `{core/cat, core/cat/dialecttest}`
(`internal/guards/importgraph_test.go`).

**What it proves:** a NON-test file in a `core/cat` SUBPACKAGE calling a
Set-frame builder is now caught. Under the previous prefix carve-out
(`inTree(relDir, "core/cat")`) this decoy was exempt and the fence stayed
green — which is precisely the hole M9c-4's data-only model packages
would otherwise inherit.

**Decoy source** — `core/cat/ftdx10/decoy.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx10 is a TRANSIENT RED-PROOF DECOY (M9c-4 task 1, proof (a)):
// a NON-test file in a core/cat SUBPACKAGE that calls a Set-frame builder.
// Under the old prefix carve-out the fence would have waved this through.
// Deleted the moment the failure is recorded.
package ftdx10

import "github.com/gm5dna/open-rig-programmer/core/cat"

func decoy(d cat.Dialect, s cat.Slot, tag string) (cat.Command, error) {
	return d.BuildMTSet(s, false, tag)
}
```

**Command:**

```
go test ./internal/guards/ -run TestWritePathReachableOnlyThroughDriver -count=1
```

**Exact failing output:**

```
--- FAIL: TestWritePathReachableOnlyThroughDriver (0.04s)
    importgraph_test.go:359: core/cat/ftdx10/decoy.go: references .BuildMTSet — the Set-frame builders may be used only from core/cat, core/cat/dialecttest (the conformance suite) and core/driver/**; other core/cat subpackages are NOT exempt, the carve-out having been narrowed from the core/cat prefix to those two packages at M9c-4 (composition-root discipline; see this test's doc comment)
FAIL
FAIL	github.com/gm5dna/open-rig-programmer/internal/guards	0.341s
FAIL
```

**Deletion:** `rm -rf core/cat/ftdx10`

**Green re-run** (same command):

```
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.215s
```

---

## Proof (b) — Rule 2 fires on `app/` importing the BARE `core/cat`

**Closure:** Rule 2 becomes bare-OR-prefix
(`internal/guards/composition_imports_test.go`).

**What it proves:** the ORIGINAL rule survives the widening. This is the
regression the plan singles out: writing the new rule with "Rule 1's
trailing-slash technique" would have matched `core/cat/` only and
silently stopped catching the bare import — the exact case Rule 2 was
written for. Rule 1 excludes its bare path deliberately (`core/driver` is
the neutral seam); Rule 2 must not.

**Decoy source** — `app/decoy_m9c4.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

// TRANSIENT RED-PROOF DECOY (M9c-4 task 1, proof (b)): an app/ production
// file importing the BARE core/cat. Deleted once recorded failing.
package main

import "github.com/gm5dna/open-rig-programmer/core/cat"

var _ = cat.KindVFO
```

**Command:**

```
go test ./internal/guards/ -run TestCompositionRootImportDiscipline -count=1
```

**Exact failing output:**

```
--- FAIL: TestCompositionRootImportDiscipline (0.03s)
    composition_imports_test.go:141: app/decoy_m9c4.go: imports "github.com/gm5dna/open-rig-programmer/core/cat" — app/ and cmd/rigprog must never import core/cat or any package beneath it; the CAT frame layer is a driver-internal detail behind the neutral driver.Session contract, and a core/cat/** model package drags it in transitively (M9a neutral-core discipline; prefix half added M9c-4)
FAIL
FAIL	github.com/gm5dna/open-rig-programmer/internal/guards	0.218s
FAIL
```

**Deletion:** `rm -f app/decoy_m9c4.go`

**Green re-run** (same command):

```
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.203s
```

---

## Proof (c) — Rule 2's PREFIX half fires on `app/` importing `core/cat/ftdx10`

**Closure:** the same Rule 2 widening, its new half.

**What it proves:** the case that fired NOTHING before M9c-4. Rule 2
matched the import path exactly, so a UI layer importing a `core/cat`
model package — which drags `core/cat` in transitively, defeating the
isolation entirely — passed the guard silently (Fable 4).

Note the imported package need not exist for this guard to fire: the
walk is a `go/parser` AST pass over import paths and resolves nothing.

**Decoy source** — `app/decoy_m9c4.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

// TRANSIENT RED-PROOF DECOY (M9c-4 task 1, proof (c)): an app/ production
// file importing a core/cat SUBPACKAGE, which drags core/cat in
// transitively. The exact-path Rule 2 in force before M9c-4 fired nothing
// here. Deleted once recorded failing.
package main

import _ "github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
```

**Command:**

```
go test ./internal/guards/ -run TestCompositionRootImportDiscipline -count=1
```

**Exact failing output:**

```
--- FAIL: TestCompositionRootImportDiscipline (0.04s)
    composition_imports_test.go:141: app/decoy_m9c4.go: imports "github.com/gm5dna/open-rig-programmer/core/cat/ftdx10" — app/ and cmd/rigprog must never import core/cat or any package beneath it; the CAT frame layer is a driver-internal detail behind the neutral driver.Session contract, and a core/cat/** model package drags it in transitively (M9a neutral-core discipline; prefix half added M9c-4)
FAIL
FAIL	github.com/gm5dna/open-rig-programmer/internal/guards	0.229s
FAIL
```

**Deletion:** `rm -f app/decoy_m9c4.go`

**Green re-run** (same command):

```
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.200s
```

**(b) and (c) together** are what make the `rel == catPath ||
strings.HasPrefix(rel, catPath+"/")` form necessary: each half fires on a
case the other misses.

---

## Proof (d) — Rule 3 fires on a production import of `core/cat/dialecttest`

**Closure:** no production package imports the conformance suite
(`internal/guards/composition_imports_test.go`, Rule 3).

**What it proves:** the forbidden-import rule has teeth. The decoy is
placed in `core/driver/ft710` deliberately — Rule 3 carries NO tree
qualifier, so not even `core/driver`, which the Set-builder fence does
exempt, may import a `*testing.T`-driven package. The walk's standing
`_test.go` exclusion is what keeps a model package's own
`dialect_test.go` import legal.

**Decoy source** — `core/driver/ft710/decoy_m9c4.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

// TRANSIENT RED-PROOF DECOY (M9c-4 task 1, proof (d)): a PRODUCTION file
// importing the testing-only conformance suite. Placed in a driver package
// deliberately — Rule 3 carries no tree qualifier, so not even core/driver,
// which the fence does exempt for builders, may import it. Deleted once
// recorded failing.
package ft710

import _ "github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
```

**Command:**

```
go test ./internal/guards/ -run TestCompositionRootImportDiscipline -count=1
```

**Exact failing output:**

```
--- FAIL: TestCompositionRootImportDiscipline (0.03s)
    composition_imports_test.go:150: core/driver/ft710/decoy_m9c4.go: imports "github.com/gm5dna/open-rig-programmer/core/cat/dialecttest" — core/cat/dialecttest is an exported TESTING-ONLY package (it drives a dialect through the conformance corpus against a *testing.T) and may be imported only from _test.go files, such as a model package's own dialect_test.go; a production import would link the test machinery into a shipped binary (M9c-4 closure 2)
FAIL
FAIL	github.com/gm5dna/open-rig-programmer/internal/guards	0.222s
FAIL
```

**Deletion:** `rm -f core/driver/ft710/decoy_m9c4.go`

**Green re-run** (same command):

```
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.199s
```

---

## Proof (e) — `dialecttest`'s REAL builder calls stay green under the narrowed fence

**No decoy.** This proof runs against the tree as committed: the point is
that narrowing the carve-out did not break the one legitimate
subpackage call site the old prefix form was covering.

**The call sites are real and really walked.** `dialecttest.go` is a
NON-test file, so `parseRepo` walks it (its sibling
`dialecttest_test.go` is excluded, as every guard intends):

```
$ ls core/cat/dialecttest/
dialecttest_test.go
dialecttest.go

$ rg -n '\.BuildMWSet\(|\.BuildMTSet\(|\.BuildMTSetCombined\(' core/cat/dialecttest/dialecttest.go
469:		if cmd, err := r.d.BuildMTSet(s, display, ""); err == nil {
477:			cmd, err := r.d.BuildMTSet(s, display, tag)
586:		cmd, err := r.d.BuildMTSetCombined(m, tag)
636:		if _, err := r.d.BuildMTSetCombined(m, tag); err == nil {
732:			cmd, err := r.d.BuildMWSet(recordFor(s, m, r.d.MWWriteKind(), 0, ctcss, shift))
760:			cmd, err := r.d.BuildMWSet(recordFor(writable, r.emittable[0], r.d.MWWriteKind(), 0, state, sh))
775:		cmd, err := r.d.BuildMWSet(recordFor(writable, r.emittable[0], r.d.MWWriteKind(), int16(hz), ctcss, shift))
786:		cmd, err := r.d.BuildMWSet(recordFor(writable, r.emittable[0], r.d.MWWriteKind(), int16(over), ctcss, shift))
804:		if _, err := r.d.BuildMWSet(recordFor(s, m, r.d.MWWriteKind(), 0, ctcss, shift)); err == nil {
858:		cmd, err = r.d.BuildMTSetCombined(cat.MemoryData{}, "AB")
863:		cmd, err = r.d.BuildMTSet(r.slots[0], false, "AB")
1037:	mtsCmd, mtsErr := zero.BuildMTSet(slotless, false, "AB")
1039:	mtcCmd, mtcErr := zero.BuildMTSetCombined(cat.MemoryData{}, "AB")
1041:	mwCmd, mwErr := zero.BuildMWSet(cat.MemoryData{})
```

**Command:**

```
go test ./internal/guards/ -run TestWritePathReachableOnlyThroughDriver -count=1 -v
```

**Output:**

```
=== RUN   TestWritePathReachableOnlyThroughDriver
--- PASS: TestWritePathReachableOnlyThroughDriver (0.04s)
PASS
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.222s
```

### (e) supplementary — the `dialecttest` member is LOAD-BEARING

A green run alone cannot distinguish "the exemption is needed and works"
from "the exemption is decorative". So the carve-out was TEMPORARILY
perturbed to `core/cat` alone — no decoy file, a one-expression edit to
the guard, reverted immediately:

```go
// temporary, reverted:
if !(pf.relDir == "core/cat") {
```

**Exact failing output (abridged — 14 identical-shaped lines, one per
call site above; the first three and the count are given):**

```
--- FAIL: TestWritePathReachableOnlyThroughDriver (0.04s)
    importgraph_test.go:359: core/cat/dialecttest/dialecttest.go: references .BuildMTSet — the Set-frame builders may be used only from core/cat, core/cat/dialecttest (the conformance suite) and core/driver/**; other core/cat subpackages are NOT exempt, the carve-out having been narrowed from the core/cat prefix to those two packages at M9c-4 (composition-root discipline; see this test's doc comment)
    importgraph_test.go:359: core/cat/dialecttest/dialecttest.go: references .BuildMTSet — ...
    importgraph_test.go:359: core/cat/dialecttest/dialecttest.go: references .BuildMTSetCombined — ...
    [... 11 further lines, one per selector: 14 in total ...]
FAIL
FAIL	github.com/gm5dna/open-rig-programmer/internal/guards	0.344s
FAIL
```

The guard file was then restored byte-for-byte from its backup and
re-run:

```
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.254s
```

So the second member of the carve-out set is doing real work: without
it, 14 legitimate conformance-suite call sites fire the fence.

---

## Working tree clean of decoys

Verified after all five proofs — no decoy file, and no `core/cat/ftdx10`
directory, survives:

```
$ git status --porcelain
 M core/cat/dialecttest/dialecttest.go
 M core/cat/mtcombined.go
 M core/cat/mtcombined_test.go
 M internal/guards/composition_imports_test.go
 M internal/guards/importgraph_test.go
?? docs/superpowers/m9c4-red-proofs.md

$ ls core/cat/ftdx10 2>&1
ls: core/cat/ftdx10: No such file or directory
```

Every modified path is a task-1 closure; the only untracked file is this
record. `core/cat/ftdx10` is created for real by task 3, not by task 1.

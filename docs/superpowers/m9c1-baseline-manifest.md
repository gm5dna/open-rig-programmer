# M9c-1 baseline manifest

Captured **28/07/2026**, as Task 8's post-hoc verification gate for the
M9c-1 registration-gate milestone (`core/csvio`, `core/spec` and
`internal/wiring` made model-neutral so a second radio can register,
required to be behaviour-preserving for the FT-710). **Re-minted, same
date, at the true final HEAD** after the Codex adversarial milestone
review's three dispatched fix rounds (A, B, C) landed — see "Third pass"
below; the original two-pass record (at `34f7b4e`/`6fd57f0`) is kept
intact for provenance rather than overwritten.

This file is the FT-710 byte-identity reference and full-gate record for
the milestone. **A difference that is not a declared/sanctioned field is a
defect, never a baseline to update** — this gate found exactly one such
defect, on its first pass, and it is documented in full below rather than
quietly folded into a clean-looking report.

- **BASE commit:** `adc2ab16d14f1883d8b9df99f2b00ba27b13ecf0` (M9c-1 plan
  commit; `git diff --stat ce7fcee..adc2ab1` shows two docs-only additions
  and nothing else, so `adc2ab1` is behaviourally identical to `ce7fcee`,
  the tip of `main` this milestone branched from). Unchanged by the
  re-mint.
- **Milestone HEAD under test (superseded by the re-mint below):**
  `6fd57f0f8530c78655534178a1d6708261f43576` ("M9c-1: gofmt the two test
  files the gate found dirty"), one commit on top of
  `34f7b4e3a14046163de7919691389b6da2fbbb4f` ("M9c-1 task 7 fix round 1:
  error on empty model slug (D9)"), the last of the seven milestone task
  commits.
- **True final HEAD (current):** `d00b2d2cb2441429b5af350f26a38c10d2197eb6`
  ("M9c1 registration-gate fix C3: narrow the milestone's documented claim
  to what actually closed") — twelve commits on top of `6fd57f0`: the
  Codex milestone review's fix-wave (`292d723`, `a45ccba`, `2381c19`,
  `5586824`) and the three dispatched fix rounds this handles, dispatch A
  (`609d27d`, `3ca1c8e`, `1b77741`), dispatch B (`6adab37`, `76768e1`) and
  dispatch C (`6b5a208`, `670405a`, `d00b2d2`).
- **Branch:** `m9c1-registration-gate`
- **Toolchain:** `go1.26.5 darwin/arm64`
- **Artefacts** were captured to an ephemeral scratch directory (not
  preserved beyond this run); this manifest is the tracked, durable record,
  per the M9b review finding that artefacts alone have no provenance.

## Two gate passes, one milestone

This gate ran in two passes because the first pass found a real defect and,
per project policy, stopped and reported it rather than fixing it in a
verification-only task:

1. **First pass, at `34f7b4e`:** built the FT-710 CLI binary (not `go run`,
   which collapses exit codes — a known artefact from the M9a gate notes),
   captured `probe --fake` / `read --fake` / a CHIRP import over
   `core/csvio/testdata/chirp_sample.csv`, and diffed them against an
   identically-built binary from a `git worktree` at `adc2ab1`. Also ran
   the full local gate. Result: byte-identity held with exactly one
   sanctioned difference, but `gofmt -l .` was **dirty** — two test files
   carried formatting drift introduced during the milestone (bisected
   below). Everything else passed. No manifest was committed at this
   point; the defect was reported instead of fixed.
2. **Fix:** a separate, mechanical, gofmt-only commit `6fd57f0`
   (whitespace-only — `git diff -w 34f7b4e..6fd57f0` is empty; `git show
   --stat 6fd57f0` touches exactly `core/codeplug/validate_test.go` (+6/-6)
   and `core/csvio/chirp_test.go` (+1/-1), 7 insertions/7 deletions total)
   resolved it.
3. **Second pass, at `6fd57f0`:** `gofmt -l .` / `go build` / `go vet` /
   `go test ./...` / `go test ./internal/guards/ -v` were re-run in full
   and now pass. A targeted `-race` re-run covered only the two packages
   the fix touched (`core/codeplug`, `core/csvio`).

**Byte-identity baselines and the full `-race ./core/...` / `-race ./app/`
suites were deliberately NOT re-run at `6fd57f0`.** Justification, so a
later reader can audit the decision rather than assume everything ran at
HEAD: the fix commit touches only `_test.go` files, which `go build`
unconditionally excludes from any binary — it cannot change one byte of
`probe --fake` / `read --fake` / CHIRP-import output captured from a built
binary at `34f7b4e`. It also touches no file outside `core/codeplug` and
`core/csvio`, so no other package's tests (race or otherwise) can be
affected by it. This is not an assumption taken on faith: `git diff -w
34f7b4e..6fd57f0 --stat` was independently confirmed empty (whitespace-only)
and `git show --stat 6fd57f0` was independently confirmed to touch exactly
those two files, before this decision was made.

## Third pass, at `d00b2d2` (this re-mint)

Twelve commits landed on `m9c1-registration-gate` after `6fd57f0`, taking
the branch through the Codex adversarial milestone review's own fix-wave
and then three further dispatched fix rounds (A: registration-gate
tightening in `core/spec`; B: `app/` capability resolution and locking; C:
this dispatch — tone-chart ownership, a loosened test, doc-claim scope,
and this re-mint). None of the previous two passes' evidence covered any
commit past `6fd57f0`, so this pass re-runs the byte-identity recipe from
freshly compiled binaries — full, not targeted — against the same `adc2ab1`
base, following exactly the reproduction recipe below (a second `git
worktree add <dir> adc2ab1`; the original worktree from passes 1-2 no
longer exists).

**Result: byte-identical, with exactly the same one sanctioned wording
change already on record — nothing new.** Every artefact hash below is
IDENTICAL, byte for byte, to the corresponding hash already recorded in
the "Artefact hashes (raw)" table for the `34f7b4e`/`adc2ab1` pass: the
twelve intervening commits changed no byte of `probe --fake`, `read
--fake` or CHIRP-import output for the FT-710. This is expected and
confirms it directly rather than assuming it: none of dispatch A's, B's or
C's changes touch a code path this recipe exercises for the FT-710 —
dispatch C's own C1 fix (routing `codeplug.ToneField.Valid` through
`caps.CTCSSTones`) changes a `codeplug.Validate` message, but `rigprog
import` here never reaches `codeplug.Validate`: the fixture's CHIRP-side
Blocking loss entries make it exit 3 (`exitBlocked`) beforehand, on both
trees, by the same design already noted below.

- Binaries built fresh: `go build ./cmd/rigprog` at `d00b2d2` (working
  tree) and at a fresh `git worktree add <dir> adc2ab1`.
- Recipe run exactly as below, each binary from its own working directory,
  `--chirp` pointed at that tree's own copy of
  `core/csvio/testdata/chirp_sample.csv` (independently re-confirmed
  byte-identical between the two trees, `diff` empty).
- Exit-code files hashed WITHOUT a trailing newline (`printf`, not `echo
  $?`), matching this manifest's own existing convention — confirmed by
  reproducing the recorded `probe.exit`/`read.exit`
  (`5feceb66…fb57e9`) and `import.exit` (`4e074085…9b49fce`) hashes
  exactly.
- `probe.stdout`, `probe.stderr`, `probe.exit`, `read.stdout`,
  `read.stderr`, `read.exit`, `read-fake.json` (`read_at`-normalised),
  `import.stderr` and `import.exit`: identical hash to the existing table,
  both across the two `d00b2d2`/`adc2ab1` trees AND against the original
  `34f7b4e`/`6fd57f0`-era capture.
- `import.stdout` (raw, un-normalised): `d00b2d2`-built hash
  `fa5ee2aaf512e88fbab094993ef8dcb2ea7bd3a757749c091a16f534b65a77b7` —
  identical to the ORIGINAL `34f7b4e`-built hash already on record.
  `adc2ab1`-built hash (this pass's fresh worktree build)
  `b07b44c16e6fbeec166167854874b7849cec962dbcb14cef46a70f139c0eaacf` —
  identical to the original `adc2ab1`-built hash already on record. The
  single-line diff between the two is the same one sanctioned wording
  change (below) — verbatim.
- `read-fake.json` raw (unnormalised, `read_at` differs as always):
  `d00b2d2`-built `d5796305f62f000aa0fd59cf47f8c3d77ab66be95d3e1f238e78847a2912b6bb`;
  fresh-`adc2ab1`-worktree-built
  `50d61c57646644ff58eca18836c4cc7b0a0ceaaf9e01eaf301a785bf14e6e87c` — both
  differ from each other (timestamp only, as always) and both differ from
  the original pass's raw hashes (a fresh clock reading each run), but both
  normalise to the SAME `370af96a…fe492c0` already on record.

Full local gate, re-run at `d00b2d2`: `gofmt -l .` (empty), `go vet ./...`
(clean), `go test ./... -count=1` (18 packages, all `ok`), `go test -race
./app/ -count=1` (`ok`, 97.5s), `go test ./internal/guards/ -v -count=1`
(8/8 PASS), `git diff adc2ab1..HEAD -- core/cat/testdata/` (empty). See
the updated gate table below for the full per-item record.

Worktree used for this pass was removed immediately after capture
(`git worktree remove`); `git worktree list` shows none outstanding.

## Reproduction — byte-identity baselines

Run once against a binary built at `34f7b4e` and once against a binary
built at `adc2ab1` (via `git worktree add <dir> adc2ab1`), each invoked
from its own working directory so `--out`-echoed text is byte-comparable:

```bash
set -e
$BIN probe --fake                                                          >probe.stdout  2>probe.stderr;  echo $? >probe.exit
$BIN read --fake --out read-fake.json                                      >read.stdout   2>read.stderr;   echo $? >read.exit
$BIN import --chirp core/csvio/testdata/chirp_sample.csv \
            --into read-fake.json --out import-out.json                   >import.stdout 2>import.stderr; echo $? >import.exit
```

`core/csvio/testdata/chirp_sample.csv` (the fixture used) is confirmed
byte-identical between `adc2ab1` and HEAD (`git diff adc2ab1..HEAD --
core/csvio/testdata/chirp_sample.csv` is empty) — it exercises 22 CHIRP
rows, including several Blocking loss entries, one of which is exactly the
tone-chart failure message this milestone sanctioned changing. Import
exits 3 (`exitBlocked`) on both trees by design — the fixture is
deliberately not a clean merge.

## Artefact hashes (raw)

| SHA-256 | Artefact | Tree |
|---|---|---|
| `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` | `probe.stdout` | both — identical |
| `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | `probe.stderr` (empty) | both — identical |
| `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | `probe.exit` ("0") | both — identical |
| `4ca2b7992e125c8aabff4535357e3ca9bcfceaa08ea5b2f62697cd7dedb13418` | `read.stdout` | both — identical |
| `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | `read.stderr` | both — identical |
| `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | `read.exit` ("0") | both — identical |
| `0341cf68b62c5c26fe96e70b85c0dd8cb5ad1067e043468bcd73bc73ad951b04` | `read-fake.json` (raw) | 34f7b4e-built |
| `05d2c6e48d6c5391261bae11b5ff585075e7a7c2b722d8aa91dc708fea83e54b` | `read-fake.json` (raw) | adc2ab1-built |
| `370af96ac8890567b98cf70eccc8518e5a1c99321890f4b3e896df6acfe492c0` | `read-fake.json` (`read_at`-normalised) | both — identical |
| `094856b06f952559216319da39a46c4ce23f06e8e9bcc09ed355d5a523319c7a` | `import.stderr` | both — identical |
| `4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce` | `import.exit` ("3") | both — identical |
| `fa5ee2aaf512e88fbab094993ef8dcb2ea7bd3a757749c091a16f534b65a77b7` | `import.stdout` | 34f7b4e-built |
| `b07b44c16e6fbeec166167854874b7849cec962dbcb14cef46a70f139c0eaacf` | `import.stdout` | adc2ab1-built |

`probe.stdout`, `probe.stderr`, `probe.exit`, `read.stdout`, `read.stderr`,
`read.exit` and `import.stderr`/`import.exit` match **byte-for-byte with NO
normalisation** across trees. `read-fake.json` and `import.stdout` each
differ raw for a declared reason — see below.

## Declared noise field

| Artefact | Noise | Normalisation | Normalised SHA-256 |
|---|---|---|---|
| `read-fake.json` | `read_at` timestamp | `sed 's/"read_at": "[^"]*"/"read_at": "NORMALISED"/'` | `370af96ac8890567b98cf70eccc8518e5a1c99321890f4b3e896df6acfe492c0` (matches across trees) |

## The one sanctioned wording change

`import.stdout` differs raw between the two trees by exactly one line — the
CHIRP tone-chart Blocking entry for line 12 (`rToneFreq`):

```
< (34f7b4e)  ... unsupported — tone frequency is not in the FT-710's CTCSS chart [BLOCKING]
> (adc2ab1)  ... unsupported — tone frequency is not in the FT-710's standard CTCSS chart (spec.StandardCTCSSTones) [BLOCKING]
```

This is the **one and only** sanctioned wording change the milestone made:
the message no longer names `spec.StandardCTCSSTones`. No other byte
differs anywhere across `probe --fake`, `read --fake` or the CHIRP import,
across stdout, stderr or exit code.

**Confirmed to still be the ONLY difference at the true final HEAD
(`d00b2d2`, third pass, above):** re-running this exact comparison after
twelve further commits (the milestone review's own fix-wave plus dispatch
A/B/C) reproduces the identical single-line diff, byte for byte, and
nothing else. Dispatch C's own fix C1 (`codeplug.ToneField.Valid` now
consulting `caps.CTCSSTones` rather than `spec.ValidTone`) does NOT show
up here: `rigprog import`'s fixture never reaches `codeplug.Validate` in
this recipe (it exits 3, blocked by CHIRP-side loss entries, first) — see
the third-pass section above for why that is expected rather than an
oversight.

## Independent cross-check

`probe.stdout`'s SHA-256
(`ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71`)
matches the hash recorded for `probe-fake.txt` in
`docs/superpowers/m9c0-baseline-manifest.md` verbatim — `probe --fake` has
not drifted since M9c-0 either, checked against a figure derived
independently of this capture. Still true, unchanged, at `d00b2d2` (third
pass).

## Full local gate results

Every item below is annotated with the exact commit it was run at, per the
three-pass structure explained above (`34f7b4e`/`6fd57f0` from passes 1-2;
`d00b2d2` is the third pass, this re-mint).

| Gate item | Run at | Result |
|---|---|---|
| `gofmt -l .` | `34f7b4e` | **FAIL** — `core/codeplug/validate_test.go`, `core/csvio/chirp_test.go` listed |
| `gofmt -l .` | `6fd57f0` | PASS — no output |
| `gofmt -l .` | `d00b2d2` | PASS — no output |
| `go vet ./...` | `34f7b4e` | PASS |
| `go vet ./...` | `6fd57f0` | PASS |
| `go vet ./...` | `d00b2d2` | PASS |
| `go build ./...` | `34f7b4e` | PASS |
| `go build ./...` | `6fd57f0` | PASS |
| `go build ./...` | `d00b2d2` | PASS (both `d00b2d2`'s own tree and the fresh `adc2ab1` worktree built cleanly) |
| `go test ./... -count=1` | `34f7b4e` | PASS (18 test packages, all `ok`) |
| `go test ./... -count=1` | `6fd57f0` | PASS (18 test packages, all `ok`) |
| `go test ./... -count=1` | `d00b2d2` | PASS (18 test packages, all `ok`) |
| `go test ./internal/guards/ -v -count=1` | `34f7b4e` | PASS (8/8 guard tests) |
| `go test ./internal/guards/ -v -count=1` | `6fd57f0` | PASS (8/8 guard tests) |
| `go test ./internal/guards/ -v -count=1` | `d00b2d2` | PASS (8/8 guard tests) |
| `go test -race ./app/ -count=1` | `34f7b4e` | PASS — not re-run at `6fd57f0`; see justification above (no file in `app/` changed) |
| `go test -race ./app/ -count=1` | `d00b2d2` | PASS (97.5s) — re-run in full: dispatch B touched `app/` |
| `go test -race ./core/... -count=1` | `34f7b4e` | PASS (8/8 core packages) — not re-run in full at `6fd57f0` |
| `go test -race ./core/codeplug/ ./core/csvio/ -count=1` | `6fd57f0` | PASS — targeted re-run of the only two packages the fix touched |
| Importgraph/driver-seam pins (`TestCompositionRootImportDiscipline`, `TestDriverSeamPackageDoesNotImportCAT`, `TestNewEngineReachableOnlyFromDriver`, `TestWritePathReachableOnlyThroughDriver`) | `34f7b4e` and `6fd57f0` | PASS both times (subset of the guards run above) |
| Importgraph/driver-seam pins (same four) | `d00b2d2` | PASS (subset of the full `internal/guards` run above) |
| `wails generate module` from `app/`, checked as `git diff --stat app/frontend/wailsjs` from repo root | `34f7b4e` | PASS — zero diff, zero collateral changes anywhere in the repo, beyond Task 5's already-committed field |
| `git diff adc2ab1..HEAD -- core/cat/testdata/` | `34f7b4e` | PASS — empty. Still true at `6fd57f0`: the fix commit touches only `core/codeplug/validate_test.go` and `core/csvio/chirp_test.go`, nothing under `core/cat/testdata/` |
| `git diff adc2ab1..HEAD -- core/cat/testdata/` | `d00b2d2` | PASS — still empty; none of the twelve intervening commits touch `core/cat/testdata/` |

(`internal/guards` has 8 tests, not 5 — file inventory:
`composition_imports_test.go`, `dialectglobals_test.go` (×3),
`driver_seam_imports_test.go`, `engine_construction_test.go`,
`importgraph_test.go`, `simulated_tokens_test.go`. All 8 PASS on all three
passes.)

`go test -race ./core/... -count=1` was NOT re-run in full at `d00b2d2`
(it exceeds a 10-minute foreground limit, per HANDOFF-m9c.md's own standing
note) — the dispatch's own required verification set
(`gofmt`/`build`/`vet`/`go test ./...`/`-race ./app/`/`internal/guards`)
was run in full instead, and is the set reported above and in the final
report.

## The gofmt defect this gate found, bisected, and its fix

First-pass evidence (`gofmt -d`, at `34f7b4e`):

```
--- core/codeplug/validate_test.go.orig
+++ core/codeplug/validate_test.go
@@ -390,12 +390,12 @@
-		Modes:        []string{"USB"},
-		TagLen:       12,
+		Modes:       []string{"USB"},
+		TagLen:      12,
 ... (Bauds, ClarMaxHz, ClarStepHz, DefaultBaud alignment, same shape)

--- core/csvio/chirp_test.go.orig
+++ core/csvio/chirp_test.go
@@ -709,7 +709,7 @@
-			_, _, err := ImportCHIRP(strings.NewReader(tc.header + "\n"), ft710LikeCapabilities())
+			_, _, err := ImportCHIRP(strings.NewReader(tc.header+"\n"), ft710LikeCapabilities())
```

Bisected by extracting each file's content at each M9c-1 commit
(`git show <rev>:<path>`) and running `gofmt -l` on the standalone
extraction:

| Commit | `validate_test.go` | `chirp_test.go` | Subject |
|---|---|---|---|
| `adc2ab1` (base) | clean | clean | plan commit |
| `0594a00` task 1 | **went dirty here** | clean | ShiftOption gains Direction, ToneState gains Encodes/Decodes |
| `e0e367f` task 2 | dirty | **went dirty here** | ImportCHIRP takes Capabilities |
| tasks 3–7 through `34f7b4e` | dirty | dirty | unchanged |

`validate_test.go` went dirty at task 1: adding a `ShiftOptions:
[]spec.ShiftOption{...}` field to a struct literal widened the required
column padding for the shorter field names above it (`Modes:`, `TagLen:`,
etc.), and nobody re-ran `gofmt` after. `chirp_test.go` went dirty at task
2: adding a second argument (`ft710LikeCapabilities()`) to an existing
`ImportCHIRP(...)` call changed gofmt's spacing rule for the `+` in
`tc.header + "\n"` in that argument position. Both are confined to
`_test.go` files — no production/build-shape code was ever affected — but
the gate's bar is unqualified (`gofmt -l .` must produce no output), so
this was reported as a defect rather than adjudicated away.

**Fix:** commit `6fd57f0` (`M9c-1: gofmt the two test files the gate found
dirty`) ran `gofmt -w` on exactly these two files and nothing else.
Confirmed whitespace-only: `git diff -w 34f7b4e..6fd57f0 --stat` is empty;
`git show --stat 6fd57f0` shows 7 insertions/7 deletions across exactly
`core/codeplug/validate_test.go` and `core/csvio/chirp_test.go`.

## Golden corpora

Unchanged since Task 51 of M9b, per the M9c-0 baseline manifest, and this
milestone's `git diff adc2ab1..HEAD -- core/cat/testdata/` confirms they
remain untouched across all of M9c-1 too, including through the third
pass at `d00b2d2` — see the gate results table above.

## Scope of the claim these hashes support

They support: **the FT-710's read, probe and CHIRP-import paths are
byte-identical from `adc2ab1` through to the true final HEAD, `d00b2d2`**
— covering M9c-1's original seven task commits, the Codex milestone
review's fix-wave, and dispatch A/B/C's fix rounds in full — save the one
sanctioned wording change, and the milestone's only other defect
(test-file gofmt drift, never reaching any compiled binary) is fixed and
independently re-verified at `6fd57f0`.

They do not support a claim about the write path, nor — beyond the
targeted evidence recorded above (`gofmt`/`vet`/`build`/`go test
./...`/`-race ./app/`/`internal/guards`, all re-run in full at `d00b2d2`)
— a claim that every individual package's own full `-race` suite was
re-run at every single intermediate commit between `6fd57f0` and
`d00b2d2`: `go test -race ./core/...` specifically was last run in full at
`34f7b4e` (see the standing 10-minute-foreground-limit note) and was not
re-run in full at `d00b2d2`, only the required, narrower verification set
the C-dispatch specified.

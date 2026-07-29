# M9c-4 baseline note

Captured **29/07/2026**, as Task 8's full local gate and byte-identity
re-run for the M9c-4 FTdx10 dialect milestone (`core/cat/ftdx10` — the
FTdx10's dialect and chart-transcribed EX inventory, plus the four
guard/API closures).

M9c-4 adds a **second radio's** dialect package alongside the FT-710's;
it is not a change to the FT-710. The milestone bar is therefore the
same one M9c-3 set and met: **not one byte of the FT-710's behaviour
moves.** This note is that milestone's byte-identity record.

Following the standing rule inherited from the M9c-1 and M9c-3
manifests: **a difference that is not a declared/sanctioned field is a
defect, never a baseline to update.** This gate found no such
difference — every compared artefact matched the M9c-3 recorded hash on
the first pass, and the single raw mismatch is the one declared noise
field (a wall-clock timestamp) already on record from M9c-1 and M9c-3.

- **HEAD under test:** `699f7c96243eaa96539d6c1501ce15967c731f8a`
  ("M9c-4 task 7b: the golden byte-compare tests"), the eighth and last
  task commit on the branch.
- **Branch:** `m9c4-ftdx10-dialect`, forked from
  `166edfb95794cdb5eca1a2f13a900e23da7c60ba` ("M9c-4 plan revision 2:
  material quarantine; fold both plan reviews").
- **Comparison target:** the sixteen hashes recorded in
  `docs/superpowers/m9c3-baseline-manifest.md`.
- **Toolchain:** `go1.26.5 darwin/arm64`
- **OS:** macOS 26.5.1 (build 25F80), arm64
- **Artefacts** were captured to an ephemeral scratch directory (not
  preserved beyond this run); this note is the tracked, durable record,
  per the M9b review finding that artefacts alone have no provenance.

## Scope of the check, and why no fresh base worktree was built

M9c-3's gate compared **two builds** — one from its BASE commit
(`3b75fcc`) in a throwaway worktree, one from its HEAD (`76f3f77`) —
because the question there was whether M9c-3's own edits to shared
`core/cat` code had moved the FT-710. Both builds were run and the
resulting hashes recorded.

This gate asks a different question, and so needs only one build. The
comparison target here is **the M9c-3 manifest's recorded hashes
themselves**, which are already the settled, published values for the
FT-710's offline paths. `76f3f77` (M9c-3's HEAD under test) is a
verified ancestor of `699f7c9` (`git merge-base --is-ancestor` returns
true), so a single build at M9c-4's HEAD, compared against those
recorded numbers, closes the chain end to end:

> `3b75fcc` ≡ `76f3f77` (proved by M9c-3's two-worktree gate) ≡
> `699f7c9` (proved below).

Rebuilding `76f3f77` in a fresh worktree would only re-derive numbers
this repository already has under version control, and would prove
nothing the recorded hashes do not already assert. The recorded hashes
are the baseline; a build that reproduces them is the evidence.

The a-priori expectation was that every artefact would be **trivially
identical**, because M9c-4 touches no FT-710 path. The branch diff
(`166edfb..HEAD`, 25 files) confirms this by inspection: nothing under
`cmd/rigprog/`, `app/`, `core/driver/`, `core/codeplug/`, `core/csvio/`
or `internal/fakeradio/` is modified at all. The only edits outside the
new `core/cat/ftdx10/` package are the `CombinedMTSetKind` rename in
`core/cat/mtcombined.go` (combined-form code, which the FT-710 —
`MTFormShort` — never reaches), the new `"ftdx10"` registration in
`internal/extable/profile.go`, a `core/cat/dialecttest` call-site
migration, and the three guard tests. **Expectation is not evidence,
though, which is why the recipe was rerun in full rather than argued
from the diff.**

`core/csvio/testdata/chirp_sample.csv` (the import fixture) is confirmed
byte-identical between `76f3f77` and HEAD
(`git diff 76f3f77..HEAD -- core/csvio/testdata/chirp_sample.csv` is
empty), so the import leg is fed exactly the input M9c-3 fed it.

## Reproduction

One binary was compiled from source (never `go run`, which collapses
exit codes — the standing artefact from the M9a gate notes):

```bash
go build -o <scratch>/rigprog ./cmd/rigprog          # from the repo root at 699f7c9
```

The M9c-3 recipe runs the binary **from a tree root**, writing to the
relative path `.capture/`, so that every path string the CLI echoes is
byte-comparable. `.capture/` is not covered by `.gitignore`, so rather
than write it into the working tree this run used a minimal scratch
mirror containing only `.capture/` and the one input the recipe reads,
`core/csvio/testdata/chirp_sample.csv`, at its identical relative path.
Every path string passed to the CLI — and hence every path string it
echoes — is therefore character-for-character what M9c-3 used, and the
repository was left untouched. That the echoed paths did reproduce is
not assumed: `read.stdout`, whose `Output:` summary line contains
`.capture/read-fake.json`, matches its recorded hash exactly.

```bash
$BIN probe --fake                                              >.capture/probe.stdout  2>.capture/probe.stderr;  printf '%s' $? >.capture/probe.exit
$BIN read  --fake --out .capture/read-fake.json                >.capture/read.stdout   2>.capture/read.stderr;   printf '%s' $? >.capture/read.exit
$BIN import --chirp core/csvio/testdata/chirp_sample.csv \
            --into .capture/read-fake.json \
            --out  .capture/import-out.json                    >.capture/import.stdout 2>.capture/import.stderr; printf '%s' $? >.capture/import.exit
$BIN export --csv .capture/export.csv .capture/read-fake.json  >.capture/export.stdout 2>.capture/export.stderr; printf '%s' $? >.capture/export.exit
sed 's/"read_at": "[^"]*"/"read_at": "NORMALISED"/' .capture/read-fake.json >.capture/read-fake.normalised.json
```

Exit-code files are hashed **without** a trailing newline (`printf`, not
`echo $?`), matching the M9c-1 and M9c-3 convention so the recorded
hashes are directly comparable across milestones. `set -e` is omitted
(M9c-3's transcription shows it, but it cannot have been in force: the
import leg exits 3 by design and would have aborted the script); exit
codes are captured explicitly instead, which is the behaviour the
artefacts record either way.

As in M9c-3, import exits 3 (`exitBlocked`) — the fixture is
deliberately not a clean merge — and writes no `--out` file.

## Artefact hashes — sixteen rows, against M9c-3's recorded values

The rows below are the M9c-3 manifest's own sixteen, in its order, so
the correspondence is auditable line by line.

| # | Artefact | M9c-3 recorded SHA-256 | This run's SHA-256 | Result |
|---|---|---|---|---|
| 1 | `probe.stdout` | `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` | identical | **MATCH** |
| 2 | `probe.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | identical | **MATCH** |
| 3 | `probe.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** |
| 4 | `read.stdout` | `89408d3a4721055714b42c7eeb7e9cc510a7bc152d2e127282341f351ba0c19c` | identical | **MATCH** |
| 5 | `read.stderr` | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | identical | **MATCH** |
| 6 | `read.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** |
| 7 | `read-fake.json` (raw), M9c-3 HEAD-built | `2323e6a2e508ab22fce9e673c55695f2baf2d985249274169d5f0ee914747e3d` | `94b1819ab143caa233ffd1828e43a0331101eeda60a830e844791649acd09979` | **DECLARED NOISE** — see below |
| 8 | `read-fake.json` (raw), M9c-3 base-built | `484f4e3754887dbbe6cf7f1a0923be4d456371927c9731fdced5b2db8a3cddc6` | (same single capture as row 7) | **DECLARED NOISE** — rows 7 and 8 already differ from *each other* in M9c-3 |
| 9 | `read-fake.json` (`read_at`-normalised) | `370af96ac8890567b98cf70eccc8518e5a1c99321890f4b3e896df6acfe492c0` | identical | **MATCH** |
| 10 | `import.stdout` | `fa5ee2aaf512e88fbab094993ef8dcb2ea7bd3a757749c091a16f534b65a77b7` | identical | **MATCH** |
| 11 | `import.stderr` | `094856b06f952559216319da39a46c4ce23f06e8e9bcc09ed355d5a523319c7a` | identical | **MATCH** |
| 12 | `import.exit` ("3") | `4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce` | identical | **MATCH** |
| 13 | `export.stdout` | `fffb2b0a3bd10756a6f5f0c655e4e4278a4a379fbb390dddaf4b2b1c59656af0` | identical | **MATCH** |
| 14 | `export.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | identical | **MATCH** |
| 15 | `export.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** |
| 16 | `export.csv` (the exported CSV itself) | `6d741c2ba228ce1fda134791b944dc9fe8a95d5c0832866e7974712c8754d88f` | identical | **MATCH** |

**Fourteen comparable artefacts, fourteen matches, no normalisation
beyond the one declared field — every stdout, every stderr, every exit
code, and the exported CSV.** Rows 7 and 8 are the two captures of the
single wall-clock-bearing artefact, which M9c-3 itself recorded as
differing between its own two runs; they are not a comparison this or
any run can pass, which is precisely why row 9 exists.

**MISMATCHES: none.**

## Declared noise field

| Artefact | Noise | Normalisation | Normalised SHA-256 |
|---|---|---|---|
| `read-fake.json` | `read_at` timestamp (`codeplug.RadioInfo.ReadAt`, a fresh wall-clock reading each run) | `sed 's/"read_at": "[^"]*"/"read_at": "NORMALISED"/'` | `370af96ac8890567b98cf70eccc8518e5a1c99321890f4b3e896df6acfe492c0` (matches M9c-3's recorded value) |

That the timestamp is the **only** thing the normalisation changed is
confirmed directly by `diff`, not inferred from the normalised hash
matching:

```
7c7
<     "read_at": "2026-07-29T11:12:28.552195+01:00"
---
>     "read_at": "NORMALISED"
```

One line, line 7, in a 9,063-byte snapshot; the 9,041-byte normalised
file that results hashes to M9c-3's recorded normalised value.

## Full local gate results

All items run at HEAD (`699f7c9`) in the repo working tree.

| Gate item | Result |
|---|---|
| `gofmt -l .` | **PASS** — no output |
| `go build ./...` | **PASS** — clean |
| `go vet ./...` | **PASS** — clean |
| `go test ./... -count=1` (foreground) | **PASS** — 20 test packages all `ok`, plus `internal/extable/gen` (no test files); 3 min 46 s wall |
| `go test ./internal/guards/ -v -count=1` | **PASS** — 8/8 guards by name (below) |
| `git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go` | **PASS** — exit 0, empty |
| `git diff --exit-code -- core/cat/ftdx10/testdata/ core/cat/ftdx10/exinventory_gen.go` | **PASS** — exit 0, empty |
| Both of the above in one invocation | **PASS** — exit 0, empty |
| `git status --short` | **PASS** — clean; the built binary and all capture artefacts live in the scratch directory only |

The 20 packages: `app`, `cmd/rigprog`, `core/cat`,
`core/cat/dialecttest`, **`core/cat/ftdx10` (new this milestone)**,
`core/clone`, `core/codeplug`, `core/csvio`, `core/driver`,
`core/driver/ft710`, `core/spec`, `core/transport`,
`internal/buildinfo`, `internal/csvmerge`, `internal/extable`,
`internal/extable/observe`, `internal/fakeradio`, `internal/guards`,
`internal/radiotext`, `internal/wiring` — M9c-3's nineteen plus the
milestone's own package, none dropped.

The 8 guards, by name, all `--- PASS` — the same eight M9c-3 recorded,
none lost to this milestone's three guard-rule changes:
`TestCompositionRootImportDiscipline`,
`TestDialectPromotedDataIsNotAPackageGlobal`,
`TestGateReachingValidatorsAreDialectMethods`,
`TestTransitiveGlobalReachSetIsReported`,
`TestDriverSeamPackageDoesNotImportCAT`,
`TestNewEngineReachableOnlyFromDriver`,
`TestWritePathReachableOnlyThroughDriver`,
`TestSimulatedProfileTokensConfinement`.

## Golden corpora

Untouched, both corpora. The FT-710's
(`core/cat/testdata/`, `core/cat/exinventory_gen.go`) and the FTdx10's
new one (`core/cat/ftdx10/testdata/`,
`core/cat/ftdx10/exinventory_gen.go`) both exit 0 under
`git diff --exit-code`. No golden was regenerated at any point in
M9c-4; the quarantined artefacts committed in Tasks 2, 4 and 7a stand
at their commit-time bytes.

## Scope of the claim these hashes support

They support: **the FT-710's probe, read, CHIRP-import and native
CSV-export paths are byte-identical — stdout, stderr and exit code —
across the whole of M9c-4, and (chaining M9c-3's own two-worktree gate)
back through `76f3f77` to `3b75fcc`, with no sanctioned exceptions at
all, the sole raw difference being the `read_at` wall-clock timestamp
inside the read snapshot.**

They do **not** support a claim about: the write path to a real radio
(no radio is opened anywhere in this recipe); the `settings`, `diff`,
`ports` or real-`--port` paths; **the FTdx10's correctness against real
hardware** — the entire `core/cat/ftdx10` package is UNVERIFIED,
manual-derived and fixture-exercised only, and this recipe never
invokes it (no CLI path selects the FTdx10 yet), so these hashes say
nothing whatever about it; or any `-race` result — `go test -race` was
**not** run in this gate, which ran the full non-race
`go test ./...` in the foreground instead, per the plan's Task 8.

## Evidence addenda (M9c-4 milestone review fix wave)

- **The gate-vs-tip invariant, stated:** the byte-identity capture was
  taken at the last CODE commit of the branch; every commit after a
  capture must be documentation-only for the capture to speak for the
  branch tip. That held here (the only post-capture commits are this
  note and the review fix wave, all prose), and it is the invariant —
  not any particular hash — that a merge reviewer should check.
- **The ledger companion's hash, recorded** (task 2's commit recorded
  the CSV's only): `group-ledger.md` SHA-256
  `367c369ce2c62f6dd6636957ddbb7e3ae8426452831316e6940e347cc38643f7`,
  unchanged since its placement commit `99976bc` (verified by
  `git log --follow` — one touching commit).

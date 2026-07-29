# M9c-3 baseline manifest

Captured **29/07/2026**, as Task 10's byte-identity gate for the M9c-3
MT frame-form seam milestone (`core/cat` taught a second MT frame form —
the FTdx10 family's combined record — behind the `MTForm` discriminator,
plus two receiver-vs-global fixes in the FT-710 driver's write path).
The milestone bar is that **not one byte of the FT-710's behaviour
moves**, so this manifest is the FT-710 byte-identity reference and
full-gate record for it.

Following the M9c-1 manifest's own standing rule: **a difference that is
not a declared/sanctioned field is a defect, never a baseline to
update.** This gate found no such difference — every artefact matched on
the first pass, and the single raw mismatch is the one declared noise
field (a wall-clock timestamp) already on record from M9c-1.

- **BASE commit:** `3b75fcc3589c9d7e685759c7e5cd7667af4eff5e` ("M9c-3 plan
  revision 2: fold both adversarial plan reviews") — the commit
  `m9c3-mt-form-seam` forked from, confirmed by `git merge-base main
  m9c3-mt-form-seam` and by `git rev-parse e0882a3^` (the parent of the
  branch's first task commit).
- **HEAD under test:** `76f3f77d544bf906e030b4c23c0f593d8924a10b` ("M9c-3
  task 9: the write path consults its receiver — MWWriteKind and
  Clarifier accessors replace the literals"), the eleventh and last
  commit on the branch (ten task commits plus the Task 4 plan
  adjudication `f7f8140`).
- **Branch:** `m9c3-mt-form-seam`
- **Toolchain:** `go1.26.5 darwin/arm64`
- **OS:** macOS 26.5.1 (build 25F80), arm64
- **Artefacts** were captured to an ephemeral scratch directory (not
  preserved beyond this run); this manifest is the tracked, durable
  record, per the M9b review finding that artefacts alone have no
  provenance.
- **Worktree:** `git worktree add <scratch>/m9c3-base 3b75fcc`, removed
  immediately after capture; `git worktree list` shows none outstanding.

## Reproduction — byte-identity baselines

Two binaries were compiled from source (never `go run`, which collapses
exit codes — the standing artefact from the M9a gate notes):

```bash
go build -o <scratch>/bin/rigprog-head ./cmd/rigprog          # from the repo root at 76f3f77
go build -o <scratch>/bin/rigprog-base ./cmd/rigprog          # from the 3b75fcc worktree
```

Each binary was then run **from its own tree root**, writing to the same
relative path `.capture/`, so that every path string the CLI echoes is
byte-comparable between the two runs:

```bash
set -e
$BIN probe --fake                                              >.capture/probe.stdout  2>.capture/probe.stderr;  printf '%s' $? >.capture/probe.exit
$BIN read  --fake --out .capture/read-fake.json                >.capture/read.stdout   2>.capture/read.stderr;   printf '%s' $? >.capture/read.exit
$BIN import --chirp core/csvio/testdata/chirp_sample.csv \
            --into .capture/read-fake.json \
            --out  .capture/import-out.json                    >.capture/import.stdout 2>.capture/import.stderr; printf '%s' $? >.capture/import.exit
$BIN export --csv .capture/export.csv .capture/read-fake.json  >.capture/export.stdout 2>.capture/export.stderr; printf '%s' $? >.capture/export.exit
sed 's/"read_at": "[^"]*"/"read_at": "NORMALISED"/' .capture/read-fake.json >.capture/read-fake.normalised.json
```

Exit-code files are hashed **without** a trailing newline (`printf`, not
`echo $?`), matching the M9c-1 manifest's convention so the recorded
hashes are directly comparable across milestones.

### Which CHIRP path this exercises, and why

The plan's Step 1 asks for "the CHIRP export path". **`rigprog` has no
CHIRP export**: `rigprog --help` lists `export` as "export a codeplug
file to CSV" (rigprog's own native schema — `core/csvio.Export`), and
the only CHIRP-format entry point in the tree is `core/csvio.ImportCHIRP`,
reached from the CLI as `rigprog import --chirp`. So the offline CHIRP
path covered here is the **import** direction, exactly the recipe M9c-1
used, and the **native CSV export** was added on top so the export
direction is covered too. Both are offline; neither opens a radio.

`core/csvio/testdata/chirp_sample.csv` (the fixture) is confirmed
byte-identical between `3b75fcc` and HEAD (`git diff 3b75fcc..HEAD --
core/csvio/testdata/chirp_sample.csv` is empty). Import exits 3
(`exitBlocked`) on both trees by design — the fixture is deliberately not
a clean merge, and writes no `--out` file.

## Artefact hashes (raw)

| SHA-256 | Artefact | Result |
|---|---|---|
| `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` | `probe.stdout` | **MATCH** — both trees |
| `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | `probe.stderr` (empty) | **MATCH** — both trees |
| `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | `probe.exit` ("0") | **MATCH** — both trees |
| `89408d3a4721055714b42c7eeb7e9cc510a7bc152d2e127282341f351ba0c19c` | `read.stdout` | **MATCH** — both trees |
| `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | `read.stderr` | **MATCH** — both trees |
| `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | `read.exit` ("0") | **MATCH** — both trees |
| `2323e6a2e508ab22fce9e673c55695f2baf2d985249274169d5f0ee914747e3d` | `read-fake.json` (raw) | HEAD-built — differs, declared noise field only (below) |
| `484f4e3754887dbbe6cf7f1a0923be4d456371927c9731fdced5b2db8a3cddc6` | `read-fake.json` (raw) | base-built — ditto |
| `370af96ac8890567b98cf70eccc8518e5a1c99321890f4b3e896df6acfe492c0` | `read-fake.json` (`read_at`-normalised) | **MATCH** — both trees |
| `fa5ee2aaf512e88fbab094993ef8dcb2ea7bd3a757749c091a16f534b65a77b7` | `import.stdout` | **MATCH** — both trees |
| `094856b06f952559216319da39a46c4ce23f06e8e9bcc09ed355d5a523319c7a` | `import.stderr` | **MATCH** — both trees |
| `4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce` | `import.exit` ("3") | **MATCH** — both trees |
| `fffb2b0a3bd10756a6f5f0c655e4e4278a4a379fbb390dddaf4b2b1c59656af0` | `export.stdout` | **MATCH** — both trees |
| `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | `export.stderr` (empty) | **MATCH** — both trees |
| `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | `export.exit` ("0") | **MATCH** — both trees |
| `6d741c2ba228ce1fda134791b944dc9fe8a95d5c0832866e7974712c8754d88f` | `export.csv` (the exported CSV itself) | **MATCH** — both trees |

Fifteen of the sixteen compared artefacts match **byte-for-byte with NO
normalisation**, including every stdout, every stderr and every exit
code. **Unlike M9c-1, there is not even one sanctioned wording change:
`import.stdout` is identical raw.**

## Declared noise field

| Artefact | Noise | Normalisation | Normalised SHA-256 |
|---|---|---|---|
| `read-fake.json` | `read_at` timestamp (`codeplug.RadioInfo.ReadAt`, a fresh wall-clock reading each run) | `sed 's/"read_at": "[^"]*"/"read_at": "NORMALISED"/'` | `370af96ac8890567b98cf70eccc8518e5a1c99321890f4b3e896df6acfe492c0` (matches across trees) |

`diff` over the two raw snapshots confirms directly that this is the
**only** differing line — it is not inferred from the normalised hash
matching:

```
7c7
<     "read_at": "2026-07-29T02:19:39.152011+01:00"      (HEAD-built)
>     "read_at": "2026-07-29T02:19:43.73324+01:00"       (base-built)
```

## Independent cross-checks against M9c-1's manifest

Three of this pass's hashes were derived independently of any figure in
`docs/superpowers/m9c1-baseline-manifest.md`, and then compared to it:

- `probe.stdout` = `ad4bf761…70b6a71` — **identical** to the hash M9c-1
  recorded (and, through it, to M9c-0's `probe-fake.txt`). `probe --fake`
  has not drifted across three milestones.
- `read-fake.json` (`read_at`-normalised) = `370af96a…fe492c0` —
  **identical** to M9c-1's recorded normalised hash. The FT-710 fake
  read produces the same codeplug, byte for byte, as it did at `d00b2d2`.
- `import.stdout` = `fa5ee2aa…65a77b7` — **identical** to M9c-1's
  HEAD-side (`34f7b4e`/`d00b2d2`) hash. The CHIRP-import loss report has
  not drifted since M9c-1 either.
- `read.stdout` = `89408d3a…1ba0c19c` differs from M9c-1's recorded
  `4ca2b799…dedb13418` **for one reason, verified rather than assumed**:
  this pass wrote the snapshot to `.capture/read-fake.json` where M9c-1
  wrote it to `read-fake.json`, and `read` echoes that path in its
  `Output:` summary line. Substituting the M9c-1 path string back
  (`sed 's|\.capture/read-fake\.json|read-fake.json|'`) reproduces
  `4ca2b7992e125c8aabff4535357e3ca9bcfceaa08ea5b2f62697cd7dedb13418`
  exactly. Nothing else in `read --fake`'s output changed.

## Full local gate results

All items run at HEAD (`76f3f77`) in the repo working tree; the base
worktree was used only for the two builds above.

| Gate item | Result |
|---|---|
| `gofmt -l .` | **PASS** — no output |
| `go build ./...` | **PASS** — clean, both trees |
| `go vet ./...` | **PASS** — clean |
| `go test ./... -count=1` (foreground) | **PASS** — 19 test packages all `ok`, plus `internal/extable/gen` (no test files); 3 min 40 s wall |
| `go test ./internal/guards/ -v -count=1` | **PASS** — 8/8 guards by name (below) |
| `git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go` | **PASS** — exit 0, empty: no golden regenerated |
| `grep -rn "MTPolicy{" --include=*.go .` | **PASS** — 22 composite literals, **every one carries `Form:`** |
| `grep -rn "mtAnswerMaxLen" --include=*.go .` | **PASS** — exactly one hit, `internal/guards/dialectglobals_test.go:59`, the `promotedConstants` entry that forbids the name returning |
| Geometry-constant check | **PASS** — see the adjudication below |

The 19 packages: `app`, `cmd/rigprog`, `core/cat`,
`core/cat/dialecttest` (new this milestone), `core/clone`,
`core/codeplug`, `core/csvio`, `core/driver`, `core/driver/ft710`,
`core/spec`, `core/transport`, `internal/buildinfo`, `internal/csvmerge`,
`internal/extable`, `internal/extable/observe`, `internal/fakeradio`,
`internal/guards`, `internal/radiotext`, `internal/wiring`.

The 8 guards, by name, all `--- PASS`:
`TestCompositionRootImportDiscipline`,
`TestDialectPromotedDataIsNotAPackageGlobal`,
`TestGateReachingValidatorsAreDialectMethods`,
`TestTransitiveGlobalReachSetIsReported`,
`TestDriverSeamPackageDoesNotImportCAT`,
`TestNewEngineReachableOnlyFromDriver`,
`TestWritePathReachableOnlyThroughDriver`,
`TestSimulatedProfileTokensConfinement`.

### The geometry-constant check, in full

The rule (Task 10 Step 2) is: **no new package-level constant carrying
dialect-varying MT geometry**. It was checked exhaustively rather than by
eyeballing the new files — every package-level `const` in `core/cat` and
`core/cat/dialecttest` (non-test files) was enumerated at `3b75fcc` and
at HEAD and the two sets differenced. The complete delta is:

| New package-level constant | File | Verdict |
|---|---|---|
| `MTFormUnspecified`, `MTFormShort`, `MTFormCombined` | `dialectconfig.go` | Not geometry — the milestone's own zero-invalid form discriminator (a typed enum). |
| `mtCombinedP11Offset` | `mtcombined.go` | **Sanctioned by name** (Task 4 adjudication, 29/07/2026): form-invariant, identical for every combined dialect, shared between the builder and the gate. |
| `mtCombinedTagOffset` | `mtcombined.go` | **Sanctioned by name** — ditto. |
| `combinedMTP11` | `mtcombined.go` | **Sanctioned by name** — ditto. |
| `combinedMTSetKind` | `mtcombined.go` | Not geometry: the combined Set's fixed P7 **kind byte** ("0: (Fixed)"), i.e. schema, not an offset/length/width. Form-invariant, and declared in the plan by name (File Structure: "the Set-kind schema constant"; Self-review: "`combinedMTSetKind` declared T4, consumed T5"). Flagged here explicitly because it is a fourth new constant beyond the three the adjudication names, rather than passed over silently. |
| `conformanceFreqHz`, `minConformanceFrames` | `dialecttest/dialecttest.go` | Not MT geometry — the exported conformance suite's own inputs (a test frequency; a floor on frames built). |

And exactly one constant was **deleted**: `mtAnswerMaxLen` from `mt.go`,
the receiver-varying datum this milestone existed to remove. The frame
LENGTH stayed a receiver method as required — `func (d Dialect)
mtCombinedLen() int` (`mtcombined.go:56`) and `func (d Dialect)
mtShortAnswerMax() int` (`mt.go:42`) — with `internal/guards`'
`promotedConstants` list now naming `mtAnswerMaxLen` so it can never
return as a package global.

## Golden corpora

Untouched. `git diff --exit-code -- core/cat/testdata/
core/cat/exinventory_gen.go` exits 0 across the whole branch, and the
`git diff 3b75fcc..HEAD --stat` for those paths is empty:
`core/cat/testdata/` remains at exactly its two commits (`ff5c19b`,
`1d38941`). No golden was regenerated at any point in M9c-3.

## Scope of the claim these hashes support

They support: **the FT-710's probe, read, CHIRP-import and native
CSV-export paths are byte-identical — stdout, stderr and exit code —
from `3b75fcc` through to `76f3f77`, the whole of M9c-3, with no
sanctioned exceptions at all, the sole raw difference being the
`read_at` wall-clock timestamp inside the read snapshot.**

They do **not** support a claim about: the write path to a real radio
(no radio is opened anywhere in this recipe — `write` was not exercised,
and the two driver write-path fixes in Task 9 are covered by unit tests
in `core/driver/ft710/write_test.go`, not by these binaries); the
`settings`, `diff`, `ports` or real-`--port` paths; the FTdx10 combined
form's correctness against real hardware (M9c-4/M9c-5's business —
the combined form is exercised only by fixtures and the new
`core/cat/dialecttest` conformance suite here); or any `-race` result —
`go test -race ./core/...` exceeds the ten-minute foreground limit per
the plan's own Global Constraints and was **not** run in this gate, which
ran the full non-race `go test ./...` in the foreground instead, as Task
10 Step 2 specifies.

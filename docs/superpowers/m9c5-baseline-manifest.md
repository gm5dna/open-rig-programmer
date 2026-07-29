# M9c-5 baseline manifest

Captured **29/07/2026**, as Task 11's byte-identity gate for the M9c-5
registration-enablers milestone (six enablers: `TagDisplay` becomes a
three-state `BoolField`; the serial baud comes from the driver's
capabilities; `Engine` binds to one dialect; `app/` becomes model-aware;
the fake-driver table becomes interface-typed; `driver.WriteResult`
becomes step-neutral).

**This milestone's bar is NOT "not one byte moves."** M9c-3 and M9c-4
could hold that bar because neither changed the FT-710's own data. M9c-5
does: E1 changes the on-disk schema, the exported CSV's spelling of one
column, and the content digest computed over a codeplug. The bar here is
the harder one to state and the easier one to cheat, so it is stated
precisely:

> **Every artefact is byte-identical to the base build EXCEPT for an
> ENUMERATED list of sanctioned carve-outs, and each "except" is proven
> by removing exactly that carve-out and showing the remaining diff
> EMPTY — never inferred from a hash that changed for a reason that
> sounded plausible.**

The standing rule inherited from the M9c-1, M9c-3 and M9c-4 manifests
applies unchanged, and with more force here than ever: **a difference
that is not a declared/sanctioned field is a defect, never a baseline to
update.** This gate found no such difference. Every removal listed below
left an empty diff on the first pass.

- **BASE commit:** `6b843356660d290bc2e91c79fce48a1357e8defb` ("M9c-5 plan
  revision 2: the compilable E1 shape; fold both plan reviews") — the
  commit `m9c5-registration-enablers` forked from, confirmed by
  `git merge-base main m9c5-registration-enablers`.
- **HEAD under test:** `f64f688` ("M9c-5 task 10: WriteResult reports
  neutral steps; the journal projects step records"), the tenth and last
  CODE commit on the branch. Ten task commits, 75 files, +4,756/-755.
- **Branch:** `m9c5-registration-enablers`
- **Toolchain:** `go1.26.5 darwin/arm64`
- **OS:** macOS 26.5.1 (build 25F80), arm64
- **Artefacts** were captured to an ephemeral scratch directory (not
  preserved beyond this run); this manifest is the tracked, durable
  record, per the M9b review finding that artefacts alone have no
  provenance.
- **Worktree:** `git worktree add <scratch>/m9c5-base 6b84335`, removed
  immediately after capture; `git worktree list` shows none outstanding.

## Reproduction

Two binaries were compiled from source (never `go run`, which collapses
exit codes — the standing artefact from the M9a gate notes):

```bash
go build -o <scratch>/bin/rigprog-head ./cmd/rigprog   # from the repo root at f64f688
go build -o <scratch>/bin/rigprog-base ./cmd/rigprog   # from the 6b84335 worktree
```

Each binary was run **from its own tree root**, writing to the same
relative path `.capture/`, so that every path string the CLI echoes is
byte-comparable between the two runs. As in M9c-4, `.capture/` is not
covered by `.gitignore`, so rather than write it into a working tree
each side used a minimal **mirror tree** containing only `.capture/` and
the two inputs the recipe reads, at their identical relative paths:

```
<mirror>/.capture/
<mirror>/core/csvio/testdata/chirp_sample.csv
<mirror>/docs/superpowers/m9c5-fixtures/chirp_minimal.csv
```

Every path string passed to the CLI — and hence every path string it
echoes — is therefore character-for-character identical between the two
runs, and neither the repository nor the base worktree was written to.

```bash
$BIN probe --fake                                              >.capture/probe.stdout  2>.capture/probe.stderr;  printf '%s' $? >.capture/probe.exit
$BIN read  --fake --out .capture/read-fake.json                >.capture/read.stdout   2>.capture/read.stderr;   printf '%s' $? >.capture/read.exit
$BIN import --chirp core/csvio/testdata/chirp_sample.csv \
            --into .capture/read-fake.json \
            --out  .capture/import-out.json                    >.capture/import.stdout 2>.capture/import.stderr; printf '%s' $? >.capture/import.exit
$BIN export --csv .capture/export.csv .capture/read-fake.json  >.capture/export.stdout 2>.capture/export.stderr; printf '%s' $? >.capture/export.exit
$BIN import --chirp docs/superpowers/m9c5-fixtures/chirp_minimal.csv \
            --into .capture/read-fake.json \
            --out  .capture/import-min.json                    >.capture/importmin.stdout 2>.capture/importmin.stderr; printf '%s' $? >.capture/importmin.exit
sed 's/"read_at": "[^"]*"/"read_at": "NORMALISED"/' .capture/read-fake.json  >.capture/read-fake.normalised.json
sed 's/"read_at": "[^"]*"/"read_at": "NORMALISED"/' .capture/import-min.json >.capture/import-min.normalised.json
```

Exit-code files are hashed **without** a trailing newline (`printf`, not
`echo $?`), matching the M9c-1/M9c-3/M9c-4 convention so the recorded
hashes are directly comparable across milestones. `set -e` is omitted:
the historical import leg exits 3 by design and would abort the script.

### Two corrections to the inherited recipe, both material

**1. The historical import leg writes NO output file.** M9c-3's and
M9c-4's transcriptions of the recipe name `--out .capture/import-out.json`,
which invites the reader to believe an `import-out.json` artefact exists
and was compared. It never did: that leg exits 3 (`exitBlocked` — the
`chirp_sample.csv` fixture is deliberately not a clean merge, carrying
BADTONE/BADMODE/BADFREQ/over-length-name rows) and returns before
`saveCodeplugNoClobber` is ever reached. This leg therefore contributes
**stdout, stderr and exit code only**, and all three are expected — and
proved below — UNCHANGED. The flag is kept in the command purely so the
invocation stays character-identical to the one M9c-3 and M9c-4 ran.

**2. A NEW deterministic successful-import leg was added**, because the
import DIRECTION is one of the two E1 changed a lot and the historical
leg cannot evidence it (it produces no file to compare).
`docs/superpowers/m9c5-fixtures/chirp_minimal.csv` is a minimal, valid,
loss-free CHIRP CSV — three channels, plain HF modes, no duplex, no
tone, no skip, names well inside the tag width — committed alongside
this manifest so the recipe is reproducible verbatim:

| Fixture | SHA-256 |
|---|---|
| `docs/superpowers/m9c5-fixtures/chirp_minimal.csv` | `87c8a9b1ac12c188a8eb726cbe1757ac01e8253ccb1741f6fe9d1af12dc02ddd` |

It imports into the fake-read codeplug and exits **0** on BOTH sides,
printing `No CHIRP import loss.` and writing `.capture/import-min.json`.
Its three channels land in slots 001-003, where 001 was already
populated — so the output carries 21 populated channels: the read's 19
plus two newly-added, with three of the 21 rewritten by the import.
That mix is deliberate: on the HEAD side the three CHIRP-derived
channels carry `tag_display` **Unknown** (E1's honest provenance) while
the other eighteen carry the migrated **Known-false**, so a single
artefact evidences both states.

The fixture bytes are identical on both sides (it is an INPUT, copied
into both mirror trees from one source), as is
`core/csvio/testdata/chirp_sample.csv`
(`git diff 6b84335..HEAD -- core/csvio/testdata/chirp_sample.csv` is
empty).

## The enumerated carve-outs

Exactly four things are sanctioned to differ, all four consequences of
E1 and all four declared in the spec BEFORE this gate ran:

| # | Carve-out | Where it appears | Why |
|---|---|---|---|
| C1 | The `schema` field, 2 -> 3 | both codeplug JSONs | E1 bumps `CurrentSchema` for the `TagDisplay` shape change |
| C2 | Every channel's `tag_display` SHAPE | both codeplug JSONs | `bool` -> `codeplug.BoolField` (an object carrying `state`, and `value` when true) |
| C3 | `radio.baseline_digest` | both codeplug JSONs, and `read.stdout`'s `Baseline digest:` line | the CONTENT digest is schema-sensitive; a migrated snapshot's embedded digest is explicitly non-recomputable legacy evidence (spec E1, "The digests, distinguished") |
| C4 | The `tag_display` COLUMN's spelling | `export.csv` | Known-false's spelling changes from `""` to `"no"` under the BoolField CSV idiom |

Plus the one **declared noise** field carried from M9c-1/3/4:
`read_at`, a fresh wall-clock reading each run, normalised FIRST by the
recorded `sed` command above.

**Nothing else was permitted to move, and nothing else did.**

## Artefact table — twenty rows

`cmp` means byte-for-byte identical with NO normalisation of any kind.
"EMPTY AFTER REMOVAL" means the two files were diffed after removing
ONLY the named carve-out, and the diff was empty — the removal commands
are given verbatim in the next section.

| # | Artefact | HEAD SHA-256 | BASE SHA-256 | Verdict |
|---|---|---|---|---|
| 1 | `probe.stdout` | `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` | identical | **MATCH** (cmp) |
| 2 | `probe.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | identical | **MATCH** (cmp) |
| 3 | `probe.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** (cmp) |
| 4 | `read.stdout` | `a9cc9fc3834159660fdf7357c3ef3e9ad8864f36b1935bdb21e8c4f1fa252448` | `89408d3a4721055714b42c7eeb7e9cc510a7bc152d2e127282341f351ba0c19c` | **EMPTY AFTER REMOVAL** — C3 (the digest line) |
| 5 | `read.stderr` | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | identical | **MATCH** (cmp) |
| 6 | `read.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** (cmp) |
| 7 | `read-fake.json` (raw) | `679a36bd34b91973379517ad6c688725486845f40c02f2504704ff0099e0e415` | `aaaea23f535f6813506137bb538152cbc450775dc0523c4e374290479d4b6584` | raw; superseded by row 8 (carries the `read_at` noise) |
| 8 | `read-fake.json` (`read_at`-normalised) | `3faad8a7a46a14c3d624fa88c42f16065cf2c1cee756e234a8836dd9c3bf6653` | `370af96ac8890567b98cf70eccc8518e5a1c99321890f4b3e896df6acfe492c0` | **EMPTY AFTER REMOVAL** — C1+C2+C3 |
| 9 | `import.stdout` (historical leg) | `fa5ee2aaf512e88fbab094993ef8dcb2ea7bd3a757749c091a16f534b65a77b7` | identical | **MATCH** (cmp) |
| 10 | `import.stderr` (historical leg) | `094856b06f952559216319da39a46c4ce23f06e8e9bcc09ed355d5a523319c7a` | identical | **MATCH** (cmp) |
| 11 | `import.exit` ("3") | `4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce` | identical | **MATCH** (cmp) |
| 12 | `export.stdout` | `fffb2b0a3bd10756a6f5f0c655e4e4278a4a379fbb390dddaf4b2b1c59656af0` | identical | **MATCH** (cmp) |
| 13 | `export.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | identical | **MATCH** (cmp) |
| 14 | `export.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** (cmp) |
| 15 | `export.csv` | `56643e7f520ded75de2957175e9b96d61d35f5205c9866cbd54d31cad78ab7fb` | `6d741c2ba228ce1fda134791b944dc9fe8a95d5c0832866e7974712c8754d88f` | **EMPTY AFTER REMOVAL** — C4 (the `tag_display` column) |
| 16 | `importmin.stdout` (new leg) | `8040f49b3cbb56293676d7cbbe7fb5cdbf232ffb96728c3ec4dc0add235a62d0` | identical | **MATCH** (cmp) |
| 17 | `importmin.stderr` (new leg, empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | identical | **MATCH** (cmp) |
| 18 | `importmin.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** (cmp) |
| 19 | `import-min.json` (raw, new leg) | `1370413fde95a8af058aee8e1f438570bd88a25521add0ad1340b737dfbd06d0` | `6f4f179858f0d53c97d1e79f1bcdc44acc0fe7148e0c916858cb1b6b015d39a8` | raw; superseded by row 20 (carries the `read_at` noise) |
| 20 | `import-min.json` (`read_at`-normalised) | `03bd53ae4a1a6b7a6f1ff8df41a5aa27eadb4a4fad0e988730f243af754db4a8` | `52ed8d20998839b4f1d6912d9d8cfecee5c96e6080808a1115b923533975c826` | **EMPTY AFTER REMOVAL** — C1+C2+C3 |

**The count, stated honestly.** Twenty artefact rows. **Fourteen are
byte-identical with no normalisation of any kind** — every stdout, every
stderr and every exit code across all five legs. **Four are
empty-after-removal** against the enumerated carve-outs. **Two are the
raw, noise-bearing captures** of the two wall-clock-bearing JSONs, which
are not a comparison any run can pass and which rows 8 and 20 exist to
replace. **MISMATCHES OUTSIDE THE CARVE-OUTS: none.**

## The removal commands, verbatim

Every "EMPTY AFTER REMOVAL" verdict above was produced by exactly one of
these. `$B` and `$H` are the base and head `.capture/` directories.

**Row 4 — `read.stdout` minus the digest line:**

```bash
diff <(grep -v '^Baseline digest: ' "$B/read.stdout") \
     <(grep -v '^Baseline digest: ' "$H/read.stdout")
```

**Rows 8 and 20 — the two JSONs, `read_at`-normalised first (the
recorded `sed` in the recipe above), then minus schema / `tag_display` /
`baseline_digest`.** Run BOTH ways, because they fail differently:

```bash
# (a) structural, via jq
JQ='del(.schema) | del(.radio.baseline_digest) | .channels |= map(if .data then .data |= del(.tag_display) else . end)'
diff <(jq "$JQ" "$B/read-fake.normalised.json") <(jq "$JQ" "$H/read-fake.normalised.json")
diff <(jq "$JQ" "$B/import-min.normalised.json") <(jq "$JQ" "$H/import-min.normalised.json")
```

```awk
# (b) textual — strip.awk. Deletes ONLY the three sanctioned members and
# passes every other byte, INCLUDING WHITESPACE, through untouched.
skip { if ($0 ~ /^[[:space:]]*\},?$/) { skip=0 } ; next }
/"tag_display":/ { if ($0 ~ /\{[[:space:]]*$/) skip=1 ; next }
/^[[:space:]]*"schema":/ { next }
/^[[:space:]]*"baseline_digest":/ { next }
{ print }
```

```bash
diff <(awk -f strip.awk "$B/read-fake.normalised.json") <(awk -f strip.awk "$H/read-fake.normalised.json")
diff <(awk -f strip.awk "$B/import-min.normalised.json") <(awk -f strip.awk "$H/import-min.normalised.json")
```

Both forms were run and both were empty. The pair is not belt-and-braces
padding: **(a) re-serialises through `jq`, which would silently absorb a
pure formatting or whitespace change**, and (b) does not, so only the
two together prove that nothing outside the three members moved. The
line arithmetic corroborates both:

| File | HEAD lines -> filtered | BASE lines -> filtered | Removed |
|---|---|---|---|
| `read-fake.normalised.json` | 651 -> 592 | 594 -> 592 | head 59 = 19x3 `tag_display` + `schema` + `baseline_digest`; base 2 (it emits no `tag_display` line at all — pre-E1 the field was `omitempty` and every value was false) |
| `import-min.normalised.json` | 684 -> 619 | 621 -> 619 | head 65 = 21x3 + 2; base 2 |

**Row 15 — `export.csv` minus the `tag_display` column.** The column is
field 12 of 13; both files were first checked to contain zero `"`
characters and exactly 13 fields on every line, so a plain `cut` is
exact:

```bash
diff <(cut -d, -f1-11,13 "$B/export.csv") <(cut -d, -f1-11,13 "$H/export.csv")
```

## Raw diffs, recorded separately — timestamp noise included

Every differing line of every changed artefact, before any removal,
classified. The arithmetic closes exactly in all four cases, which is
the point: there is no unexplained remainder.

| Artefact | Raw differing lines | Census |
|---|---|---|
| `read.stdout` | 2 | 1 line each side: `Baseline digest:` (C3) |
| `read-fake.json` | 63 | 19x3 `tag_display` object lines (C2) + 2 `schema` (C1) + 2 `baseline_digest` (C3) + 2 `read_at` (declared noise) |
| `export.csv` | 38 | 19 populated channels x 2 sides; each row's ONLY change is the `tag_display` column, `""` -> `"no"` (C4) |
| `import-min.json` | 69 | 21x3 `tag_display` (C2) + 2 `schema` + 2 `baseline_digest` + 2 `read_at` |

The `read_at` values, quoted rather than described:

```
base: "read_at": "2026-07-29T20:13:36.115424+01:00"
head: "read_at": "2026-07-29T20:13:31.731818+01:00"
```

(the same value appears in each side's `read-fake.json` and its
`import-min.json`, since the import inherits the snapshot's `RadioInfo`).

A representative `export.csv` row, both sides, so C4 is visible rather
than asserted:

```
base: 001,M-01,7000000,LSB,,,,OFF,,SIMPLEX,,,
head: 001,M-01,7000000,LSB,,,,OFF,,SIMPLEX,,no,
```

## Independent authentication of the base side

The base build is not merely asserted to be a valid baseline: **fourteen
of its hashes reproduce the values M9c-3 and M9c-4 already recorded**,
derived here from a fresh worktree build with no reference to those
documents while the recipe ran.

`probe.stdout` `ad4bf761…`, `probe.stderr`/`export.stderr` `e3b0c442…`,
`probe.exit`/`read.exit`/`export.exit` `5feceb66…`, `read.stdout`
`89408d3a…`, `read.stderr` `8e174833…`, `import.stdout` `fa5ee2aa…`,
`import.stderr` `094856b0…`, `import.exit` `4e074085…`,
`export.stdout` `fffb2b0a…`, `export.csv` `6d741c2b…`, and — the
strongest single row — `read-fake.json` (`read_at`-normalised)
`370af96a…`, byte-identical to the value recorded in BOTH the M9c-3
manifest and the M9c-4 note.

That closes the chain end to end:

> `3b75fcc` = `76f3f77` (M9c-3's two-worktree gate) = `699f7c9`
> (M9c-4's note) = `6b84335` (this gate's base) -> `f64f688` (this
> gate's head, with the four enumerated carve-outs and nothing else).

## The schema-2 -> 3 load round-trip

Three legs, each run with the HEAD binary over a genuine schema-2 file —
the base build's own `read-fake.json` (`"schema": 2`), beside the head
build's (`"schema": 3`).

**R1 — the migrated file is indistinguishable from a fresh read.**

```bash
rigprog-head diff --fake schema2.json    # a schema-2 file, migrated on load
rigprog-head diff --fake schema3.json    # the native schema-3 file
rigprog-base diff --fake schema2.json    # the OLD binary, same schema-2 file
```

All three print, character for character:

```
No changes.

Added 0, Modified 0, Erased 0, Blocked 0, Unchanged 117
```

**`Blocked 0` is the load-bearing number.** E1's diff gate blocks any
channel whose `TagDisplay` is not Known, and the migration rule
(absent -> `{Known, false}`) was chosen precisely so that legacy files
are not mass-blocked. This is that decision demonstrated rather than
argued: 117 slots, none blocked, and the new binary's verdict on the old
file is identical to the old binary's own.

**R2 — the migrated file EXPORTS identically.**

```bash
rigprog-head export --csv from-schema2.csv schema2.json
rigprog-head export --csv from-schema3.csv schema3.json
diff from-schema2.csv from-schema3.csv        # EMPTY
```

Byte-identical, with no removal at all. Every migrated channel's every
field renders exactly as the natively-schema-3 file's does — including
the `tag_display` column, where both produce `no`.

**R3 — load + save: schema 2 in, schema 3 out.**

```bash
rigprog-head import --chirp docs/superpowers/m9c5-fixtures/chirp_minimal.csv --into schema2.json --out saved-from-schema2.json
rigprog-head import --chirp docs/superpowers/m9c5-fixtures/chirp_minimal.csv --into schema3.json --out saved-from-schema3.json
```

Both write `"schema": 3`. After `read_at` normalisation the two outputs
differ in **exactly one line**, `radio.baseline_digest`, and are empty
after removing it:

```bash
diff <(grep -v '"baseline_digest":' saved-from-schema2.norm.json) \
     <(grep -v '"baseline_digest":' saved-from-schema3.norm.json)   # EMPTY
```

That one line is carve-out C3 behaving exactly as the spec declares.
Each file carries through the digest its SOURCE carried
(`ccdf39f3…` from the schema-2 snapshot, `b1cb3e60…` from the schema-3
one) — **preserved, never recomputed**. A migrated snapshot's embedded
digest is legacy evidence of what was read at the time, and the load
path does not silently re-derive it to make the file look consistent.

**Semantic identity reached**, by all three legs.

## Full local gate results

All items run at HEAD (`f64f688`) in the repo working tree; the base
worktree was used only for its build.

| Gate item | Result |
|---|---|
| `gofmt -l .` | **PASS** — no output |
| `go build ./...` | **PASS** — clean, both trees |
| `go vet ./...` | **PASS** — clean |
| `go test ./... -count=1` (foreground) | **PASS** — 20 test packages all `ok`, plus `internal/extable/gen` (no test files); 4 min 3 s wall |
| `go test ./internal/guards/ -v -count=1` | **PASS** — 9/9 guards by name (below) |
| `git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go core/cat/ftdx10/testdata/ core/cat/ftdx10/exinventory_gen.go` | **PASS** — exit 0, empty (all four paths, one invocation) |
| E6 grep gate, all eight retired names | **PASS** — see below |
| `git status --short` | **PASS** — clean; binaries and capture artefacts live in the scratch directory only |

The 20 packages: `app`, `cmd/rigprog`, `core/cat`,
`core/cat/dialecttest`, `core/cat/ftdx10`, `core/clone`, `core/codeplug`,
`core/csvio`, `core/driver`, `core/driver/ft710`, `core/spec`,
`core/transport`, `internal/buildinfo`, `internal/csvmerge`,
`internal/extable`, `internal/extable/observe`, `internal/fakeradio`,
`internal/guards`, `internal/radiotext`, `internal/wiring` — M9c-4's
twenty, none dropped, none added.

The 9 guards, by name, all `--- PASS` — M9c-4's eight plus this
milestone's own:
`TestCompositionRootImportDiscipline`,
`TestDialectPromotedDataIsNotAPackageGlobal`,
`TestGateReachingValidatorsAreDialectMethods`,
`TestTransitiveGlobalReachSetIsReported`,
`TestDriverSeamPackageDoesNotImportCAT`,
`TestNewEngineReachableOnlyFromDriver`,
`TestWritePathReachableOnlyThroughDriver`,
**`TestRetiredWriteResultNamesAreGone` (new, E6)**,
`TestSimulatedProfileTokensConfinement`.

### The E6 grep gate, in full

```bash
grep -rInE 'MWSent|MWConfirmed|MTSent|MTConfirmed|mw_sent|mw_confirmed|mt_sent|mt_confirmed' . | grep -v '^./.git/'
```

**PASS.** Outside `docs/superpowers/` (this milestone's own written
history — the M9c-5 spec and plan, and the M9c-3 spec that ledgered the
obligation) there is **exactly one hit**: the
`retiredWriteResultNames` list in
`internal/guards/retirednames_test.go`, the entry that forbids the names
returning. That is the convention this repository already recorded for
`mtAnswerMaxLen` in the M9c-3 manifest, followed deliberately.

The guard reads the Go tree AND the frontend (`.js`/`.ts`/`.svelte`/
`.json`), because the wailsjs bindings are GENERATED from the Go types —
a resurrected field name would travel straight into the GUI. Two doc
comments that describe the retired format (`core/driver/driver.go`'s
`WriteResult` rationale and `core/clone/execute.go`'s
`writeResultFormat`) refer to the retired keys **by reference** rather
than spelling them, so the gate stays literal and the explanation stays
available.

## Golden corpora

Untouched, both corpora. `core/cat/testdata/`,
`core/cat/exinventory_gen.go`, `core/cat/ftdx10/testdata/` and
`core/cat/ftdx10/exinventory_gen.go` all exit 0 under
`git diff --exit-code`, in one invocation, at every task commit and at
the branch tip. **No golden was regenerated at any point in M9c-5.**

## The gate-at-final-code-tip invariant

Per the M9c-4 note's evidence addendum, and re-stated here because it is
the invariant a merge reviewer should check rather than any particular
hash:

> The byte-identity capture is taken at the LAST CODE COMMIT of the
> branch. Every commit after the capture must be documentation-only for
> the capture to speak for the branch tip.

That holds here. The capture was taken at `f64f688`, the tenth and last
code commit. The only commit after it is Task 11's own, and it TRACKS
exactly two files:

| Path | Kind |
| --- | --- |
| `docs/superpowers/m9c5-baseline-manifest.md` | this manifest |
| `docs/superpowers/m9c5-fixtures/chirp_minimal.csv` | a test INPUT, read by no production code path |

No `.go` file, no frontend file, and no golden is touched by it —
`git show --stat 80f95ee` is the whole of that commit's contents.

The handoff updates that accompanied the same piece of work are a
separate thing and are deliberately NOT in it: `.superpowers/` is
gitignored (see `.gitignore`, "Superpowers scratch state"), as is
`docs/fixtures-private/`, so `HANDOFF-m9c.md` in either location is an
ON-DISK working file that no commit on this branch contains. An earlier
version of this paragraph listed them alongside the manifest and the
fixture as though all three were commit contents, which would have had a
reviewer looking for a third tracked path that does not exist. The
distinction also matters to the invariant above: only tracked files can
affect whether a post-capture commit is documentation-only, and the
gitignored handoff cannot make it otherwise however much it changes.

## Scope of the claim this manifest supports

It supports: **across the whole of M9c-5, from `6b84335` to `f64f688`,
the FT-710's probe, read, CHIRP-import (both a blocked and a clean one)
and native CSV-export paths are unchanged — stdout, stderr and exit code
alike — except for four enumerated, spec-declared carve-outs, each
proven by removal rather than inferred; and a schema-2 codeplug loaded
by the new binary is semantically identical to the schema-3 one, by
diff, by export and by load-and-save.**

It does **not** support a claim about: the write path to a real radio
(no radio is opened anywhere in this recipe — E6's own changes to
`WriteChannel` and the clone journal are covered by unit tests in
`core/driver/ft710/write_test.go` and `core/clone/execute_test.go`, not
by these binaries); the `settings`, `ports` or real-`--port` paths (the
`diff` path IS exercised, in the round-trip section, but against the
fake); the FTdx10's correctness against real hardware (still UNVERIFIED
and UNREGISTERED — no CLI path selects it); the GUI at runtime (the
frontend is typechecked and unit-tested, not driven); or any `-race`
result — `go test -race` was **not** run in this gate, which ran the
full non-race `go test ./...` in the foreground instead.

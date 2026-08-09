# M9d-2 baseline manifest

Captured **09/08/2026**, as Task 9's byte-identity gate, first per-sibling
baselines, red-proof index and full local gate for the M9d-2
FTdx101-registration milestone. The FTdx101D and the FTdx101MP become
registered, selectable models: one parameterised driver
(`core/driver/ftdx101`), one parameterised fake (`internal/fakedx101`),
two registrations, per-model radiotext, per-model UI pins — and one
caps-aware fold to the CHIRP importer's blank-`Skip` construction.

**This milestone does NOT clear the M9c-6/M9d-1 zero-diff bar, and was
never meant to.** M9d-1 registered no model and so moved nothing at all.
M9d-2 registers two, and Task 8 deliberately changed how a blank CHIRP
`Skip` cell is constructed. Both consequences were named in writing —
and enumerated to the ROW — before this gate ran. The bar is therefore
the M9c-6 form with an adjudicated exception list:

> **Every recorded value in `docs/superpowers/m9c6-baseline-manifest.md`
> is reproduced exactly, EXCEPT the rows of two named classes — the
> model-list class and the CHIRP-skip class — whose members were fixed by
> the plan review BEFORE this gate ran. A row inside a class that
> re-runs identical is recorded as identical, never waved through. A row
> that moves and is NOT in a class is a DEFECT, never a baseline to
> update. Every delta that did occur is recorded VERBATIM (old value,
> new value) and PROVED BY REMOVAL, never inferred from a hash.**

This gate found no row outside its class. **95 recorded values compared:
82 reproduced identically, 13 moved, all 13 inside their adjudicated
class, all 13 recorded verbatim and all 13 reproduced back to M9c-6's
recorded values by reverting the commit that caused them.** Zero
unexplained differences. Zero carve-outs invoked.

Additionally: **80 new recorded values** minted as the FIRST baselines
for `FTdx101D` and `FTdx101MP` (40 each), and a **twenty-row red-proof
index** — eleven mutations run and reverted at this task, nine more
cited from Tasks 5-7 — every row shown to FIRE.

- **BASE commit:** `6a4e5e4bfaeb6d58426706ffa1e2284d706d7d04` — the
  commit `m9d2-ftdx101-registration` forked from, confirmed by
  `git merge-base main m9d2-ftdx101-registration`.
- **HEAD under test:** `1564d318b76d28cb57dcfe2ee8f2d70ee03097d6`
  ("M9d-2 task 8 fix round 1: the stale scan_skip assertion this commit
  falsified, and three comment narrowings"), the fourteenth and **last
  CODE commit** on the branch. 65 files, +15,572/−360.
- **Branch:** `m9d2-ftdx101-registration`
- **Comparison target:** every value recorded in
  `docs/superpowers/m9c6-baseline-manifest.md`, compared **by that
  manifest's own tables** — hash-compared only where it records a hash,
  literal-compared where it records a literal, and nothing outside its
  tables claimed. This is the M9d-1 baseline-note method
  (`docs/superpowers/m9d1-baseline-note.md`).
- **Chain:** `490c38c` (M9c-6's head under test) and `fef25cb` (M9d-1's)
  are both verified ancestors of `1564d31`
  (`git merge-base --is-ancestor`, both true), so this run's numbers
  speak for the whole span from M9c-6's capture to here.
- **Toolchain:** `go1.26.5 darwin/arm64`; npm 11.19.0; wails from
  `~/go/bin/wails`
- **OS:** macOS 26.5.1 (build 25F80), arm64
- **Artefacts** were captured to an ephemeral scratch directory (not
  preserved beyond this run); this manifest is the tracked, durable
  record, per the M9b review finding that artefacts alone have no
  provenance.
- **Worktrees:** three created and three removed
  (`git worktree list` shows only the repository itself); the repository
  working tree was never written to by the recipe.

---

## The adjudicated delta classes, stated before the measurement

Fixed at plan review against the M9c-6 manifest's own tables and
`cmd/rigprog/import.go`'s stream layout. **These rows and no others.**

**Model-list class** — a registry-driven print gains exactly two names:

- bare `rigprog`, `rigprog help`, `rigprog nosuchcmd` (the top usage
  text, `cmd/rigprog/usage.go:25`, `%s` filled at print time from
  `strings.Join(wiring.SupportedModels(), ", ")`);
- `UnknownModelError.Supported`'s rendered text
  (`internal/wiring/wiring.go:173-175`, the `Error()` method the plan
  cited by its pre-Task-7 location);
- `GetSupportedModels` (`app/connection.go:39`, the Wails binding).

**CHIRP-skip class** — Task 8's blank-`Skip` construction, AT ROW LEVEL:

- Part 1 row 7 `import.stdout` (`chirp_sample.csv` carries an `S` row, so
  it gains a dropped-loss line) and row 18 `import-min.json`;
- Part 3 stdout legs **9, 10, 11**;
- Part 3 artefacts `ftdx10-import-chirp.json`, `ftdx10-import-skip.json`,
  `ftdx10-import-chirp.csv`.

**Everything else: ZERO diffs, byte-identical** — explicitly including
Part 1 rows 8-9 and 14-16, Part 3 legs 8, 12, 13 and 14, both
supplementary FT-710 legs S1 and S2, and every FT-710 native-export row.

The reasoning that fixed the CHIRP list, restated because the
measurement confirms it: the loss report prints to STDOUT; **a blank
`Skip` cell produces NO loss entry under EITHER construction**, so
clean-import stdout cannot move (row 14 and leg 8 are the predictions,
and both held); export stdout prints only a row count and a path.

### The masking fact — why an unchanged count is not a missing delta

Verified against `core/codeplug/diff.go`'s gate order, and load-bearing
for reading the tables below.

On the **FT-710**, `tag_display` is Unknown on a CHIRP-imported channel,
and diff's tag-display gate (gate 3) fires BEFORE the per-field gate and
short-circuits. It therefore **MASKS** the `scan_skip` refusal: the
FT-710's CHIRP-derived deltas are visible in the JSON states, the
digests and the new `S` loss entry — but NOT necessarily in the headline
`Blocked` count. Leg S1 below is the worked example: its stdout is
byte-identical across the change even though the underlying `scan_skip`
state moved from Known to Unknown underneath it.

On the **FTdx10/FTdx101** legs, `tag_display` is `Unavailable` (Write
Unsupported), so gate 3 does NOT fire, and the count delta IS visible —
`Blocked 3` → `Blocked 0` on Part 3's legs 10 and 11.

**A "no count change" on an FT-710 CHIRP leg is EXPECTED, not a missing
delta.** Every row below is classified accordingly.

---

## Reproduction

One binary compiled from source (never `go run`, which collapses exit
codes — the standing artefact from the M9a gate notes):

```bash
go build -o <scratch>/bin/rigprog-head ./cmd/rigprog   # from the repo root at 1564d31
```

Two further binaries were built inside throwaway worktrees for the
removal proofs (Part 4), and discarded with them.

The M9c-6 recipe runs its binary **from a tree root**, writing to the
relative path `.capture/`, so every path string the CLI echoes is
byte-comparable. `.capture/` is not covered by `.gitignore`, so — as in
M9c-4, M9c-5, M9c-6 and M9d-1 — this run used minimal **mirror trees**
holding only `.capture/` and the inputs each recipe reads, at their
identical repo-relative paths, and the repository was left untouched
(`git status --short` empty before and after; verified):

```
<mirror-ft710>/.capture/
<mirror-ft710>/core/csvio/testdata/chirp_sample.csv
<mirror-ft710>/docs/superpowers/m9c5-fixtures/chirp_minimal.csv

<mirror-dx10>/.capture/            <mirror-d>/.capture/     <mirror-mp>/.capture/
  … each with docs/superpowers/m9c5-fixtures/chirp_minimal.csv
    and docs/superpowers/m9c6-fixtures/chirp_skip.csv

<mirror-usage>/                    # Part 2 echoes no paths at all
```

The three recipe inputs are byte-identical to the ones M9c-6 fed in,
compared against the manifest's own recorded values:

| Input | Recorded SHA-256 | Result |
|---|---|---|
| `core/csvio/testdata/chirp_sample.csv` | `ee3f2664242bdd2292f7afc49d46d53338f3afa227cccfd57b24855e4216c7be` | **MATCH** |
| `docs/superpowers/m9c5-fixtures/chirp_minimal.csv` | `87c8a9b1ac12c188a8eb726cbe1757ac01e8253ccb1741f6fe9d1af12dc02ddd` | **MATCH** |
| `docs/superpowers/m9c6-fixtures/chirp_skip.csv` | `f5f23977d1ef67ee23943324e9796d508bbec12efdedbd0a1bac44b78e07b6c5` | **MATCH** |

The Part 1 recipe was run **verbatim**, exactly as the M9c-6 manifest
transcribes it, including its supplementary legs S1 and S2. Exit-code
files are hashed **without** a trailing newline (`printf`, not
`echo $?`), matching the M9c-1/M9c-3/M9c-4/M9c-5/M9c-6/M9d-1 convention
so the recorded hashes are directly comparable across milestones.
`set -e` is omitted: the historical import leg exits 3 by design and
would abort the script. That leg wrote **no** output file
(`ls .capture/import-out.json` → "No such file or directory"), as the
manifest records; the `--out` flag is kept so the invocation stays
character-identical.

M9d-1's two method notes were carried forward unchanged: S2's
intermediate file names are free variables (`export-skip.csv`,
`csvskip.json`) that no recorded artefact can reach, and A7's `--into`
source is a byte copy of the model's own fresh read. Both choices are
confirmed by outcome rather than asserted — the rows that ARE recorded
reproduce.

The one declared noise field is normalised by the recipe's own `sed`,
and that the normalisation changed **only** the timestamp is confirmed
by `diff`, not inferred from a hash matching:

```
$ diff read-fake.json read-fake.normalised.json
7c7
<     "read_at": "2026-08-09T05:48:55.213325+01:00",
---
>     "read_at": "NORMALISED",
```

— one line, line 7, and identically one line for `import-min.json`. The
raw un-normalised hashes are recorded for completeness and are **not** a
comparison any run can pass (the M9c-6 manifest says so itself):
`read-fake.json`
`ba332fa4a91e1fdba1720e0ff424ad485952fae07270480c1cee1cd451924171`,
`import-min.json`
`db85ea52c1bfa5e3a9c45e3b06e23dea65145b4115b2573fde3405a9e40d6a3d`.

Every comparison was made mechanically — a recorded-value table driven
through `shasum -a 256` and string equality, not by reading numbers off
a screen. The comparator names its loop variable `fp`, never `path`,
which in zsh is the array view of `PATH`; that was M9d-1's spurious
52/52 mismatch and the lesson is inherited rather than relearnt.

---

## Part 1 — the FT-710 tables: 29 identical, 2 designed deltas

The M9c-6 manifest's own eighteen, in its order, so the correspondence is
auditable line by line. Every row is compared on **exactly** what that
manifest records for it.

| # | Artefact | M9c-6 recorded SHA-256 | Result |
|---|---|---|---|
| 1 | `probe.stdout` | `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` | **MATCH** |
| 2 | `probe.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | **MATCH** |
| 3 | `probe.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** |
| 4 | `read.stdout` | `a9cc9fc3834159660fdf7357c3ef3e9ad8864f36b1935bdb21e8c4f1fa252448` | **MATCH** |
| 5 | `read.stderr` | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | **MATCH** |
| 6 | `read.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** |
| **7** | **`import.stdout`** (historical, blocked) | `fa5ee2aaf512e88fbab094993ef8dcb2ea7bd3a757749c091a16f534b65a77b7` | **DESIGNED DELTA** → `4ffb42cc77c241b427dcca1900860c4f26bd0601bf2e184f9bb1bac2b17a1416` |
| 8 | `import.stderr` (historical, blocked) | `094856b06f952559216319da39a46c4ce23f06e8e9bcc09ed355d5a523319c7a` | **MATCH** |
| 9 | `import.exit` ("3") | `4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce` | **MATCH** |
| 10 | `export.stdout` | `fffb2b0a3bd10756a6f5f0c655e4e4278a4a379fbb390dddaf4b2b1c59656af0` | **MATCH** |
| 11 | `export.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | **MATCH** |
| 12 | `export.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** |
| 13 | `export.csv` | `56643e7f520ded75de2957175e9b96d61d35f5205c9866cbd54d31cad78ab7fb` | **MATCH** |
| 14 | `importmin.stdout` (clean import) | `8040f49b3cbb56293676d7cbbe7fb5cdbf232ffb96728c3ec4dc0add235a62d0` | **MATCH** |
| 15 | `importmin.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | **MATCH** |
| 16 | `importmin.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** |
| 17 | `read-fake.json` (`read_at`-normalised) | `3faad8a7a46a14c3d624fa88c42f16065cf2c1cee756e234a8836dd9c3bf6653` | **MATCH** |
| **18** | **`import-min.json`** (`read_at`-normalised) | `03bd53ae4a1a6b7a6f1ff8df41a5aa27eadb4a4fad0e988730f243af754db4a8` | **DESIGNED DELTA** → `da5f2a7a05d54f292a826048a68ddd439eb27837cebbd2cb38cbc257acd246ae` |

**Exactly the two rows the class names, and no others.** Rows 8-9 and
14-16 — the rows the class list explicitly required to be identical —
are identical.

### Row 7, verbatim: the one gained line

`chirp_sample.csv` carries a `Skip=S` row at line 3. Under the old
construction that produced no loss entry (the value was silently mapped
to a Known `false`); under Task 8's it is reported honestly as dropped.
The whole diff, so the arithmetic closes at one line and no remainder:

```
$ diff base/import.stdout head/import.stdout
2a3
>   line 3, column Skip, value "S": dropped — CHIRP Skip "S" dropped: scan-skip is not reachable over CAT on FT-710; scan-skip left unresolved
```

The other nineteen loss lines — the offsets, the tone refusals, the
duplex and mode rejections, the name truncations, the out-of-range
Locations — are byte-identical. **`import.exit` is still "3"** (row 9)
and `import.stderr` is unchanged (row 8): the new line is an added
DROPPED entry, not a new blocking one, and the historical leg still
exits 3 for exactly the reasons it always did.

### Row 18, verbatim: three state words

`chirp_minimal.csv` has blank `Skip` cells, so it gains no loss line
(row 14 is identical, as predicted). What moves is the state stored for
each of the three imported channels:

```
$ diff base/import-min.normalised.json head/import-min.normalised.json
29c29
<           "state": "known"
---
>           "state": "unknown"
48c48
<           "state": "known"
---
>           "state": "unknown"
67c67
<           "state": "known"
---
>           "state": "unknown"
```

**Six lines, three pairs, one per imported channel, and nothing else.**
Confirmed by `jq` over the file: all 21 populated channels now carry
`scan_skip {"state":"unknown"}` — the 18 read-derived ones were already
Unknown and did not move, and the 3 CHIRP-derived ones joined them.

### S1 — the masking, measured

`diff --fake` over the clean CHIRP import. All three recorded hashes:

| Artefact | Recorded SHA-256 | Result |
|---|---|---|
| `diffmin.stdout` | `099227a776e130295a342ccce8567b0550e77d3c2326138972f1db08fabe6f8d` | **MATCH** |
| `diffmin.stderr` | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | **MATCH** |
| `diffmin.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** |

**This is the masking fact demonstrated, not argued.** The file S1 diffs
is `import-min.json` — row 18, which DID move. Its stdout nevertheless
reproduces byte for byte, because the tag-display gate fires first:

```
Added:
  M-02: freq 7150000 Hz, mode LSB, tag "MINIMALLSB"
    BLOCKED: tag display unknown — set On or Off before sending
  M-03: freq 14050000 Hz, mode CW-U, tag "MINIMALCW"
    BLOCKED: tag display unknown — set On or Off before sending
Modified:
  M-01: freq 7000000→14200000 Hz, mode LSB→USB, tag ""→"MINIMALUSB"
    BLOCKED: tag display unknown — set On or Off before sending

Added 2, Modified 1, Erased 0, Blocked 3, Unchanged 114
```

A row inside the CHIRP class that re-ran identical would have been
recorded as identical; this is a row OUTSIDE the class whose input
moved and whose output did not, which is exactly what the gate order
predicts. It is recorded rather than passed over because it is the
single clearest piece of evidence that "Blocked 3 unchanged" on an
FT-710 leg is not a missing delta.

### S2 — the native-CSV round trip: UNAFFECTED, as predicted

M9c-6 records two hashes here and, for the remaining streams, the
literal exit value `"0"`. Only the recorded values are compared:

| Artefact | Recorded | Result |
|---|---|---|
| the `awk`-edited CSV | `0cac78ae84eb226ad5f2d600493912d0bd4542ccccdfd75668cd71db5eeb46cb` | **MATCH** (hash) |
| `csvskip-diff.stdout` | `7521da5c15a5496ccaa86314dc385c747fc73427e8d332009c04f99b2f7e774d` | **MATCH** (hash) |
| `csvskip-import` .stdout/.stderr/.exit | exit `"0"` recorded; no hash | **MATCH** (exit literal) |
| `csvskip-diff` .stderr/.exit | exit `"0"` recorded; no hash | **MATCH** (exit literal) |

The M9d-1 S2 leg is a NATIVE-CSV path, not a CHIRP one, and its channels
were read-derived and already Unknown. It was predicted unaffected and
is. Its verdict block reproduces word for word:

```
Modified:
  M-01: freq 7000000→7000000 Hz, mode LSB→LSB, tag ""→""
    BLOCKED: scan_skip not writable on this radio

Added 0, Modified 1, Erased 0, Blocked 1, Unchanged 116
```

That the edited CSV reproduces `0cac78ae…` can only happen because
`export.csv` reproduced row 13 first.

**Part 1 totals: 31 recorded values compared (23 hashes + 8 exit
literals); 29 MATCH, 2 DESIGNED DELTAS, 0 outside class.**

---

## Part 2 — the model-list class: the two rows, verbatim

`topUsageTextTemplate` (`cmd/rigprog/usage.go:25`) carries a `%s`
placeholder filled at PRINT time by `printUsage` from
`strings.Join(wiring.SupportedModels(), ", ")`. That is **task 40's
design** (M9a-4, the CLI neutralisation), and M9c-6 exercised it once
already by registering the FTdx10. M9d-2 registers two more.

**BEFORE** (M9c-6's recorded AFTER value, reproduced by the removal
proof in Part 4):

```
rigprog is a command-line memory programmer for Yaesu radios (currently: FT-710, FTdx10).
```

**AFTER** (this gate's head, `1564d31`):

```
rigprog is a command-line memory programmer for Yaesu radios (currently: FT-710, FTdx10, FTdx101D, FTdx101MP).
```

**The list gains exactly the two registered names, in
`SupportedModels()`'s own sorted order, and nothing else changes.**

Every value M9c-6's Part 2 table records for its **head** column,
compared:

| Surface | Stream | Recorded (head) | This run | Result |
|---|---|---|---|---|
| bare `rigprog` | stderr | 18 lines, exit 2 | 18 lines, exit 2 | **MATCH** |
| bare `rigprog` | stdout | 0 (empty) | 0 | **MATCH** |
| `rigprog help` | stdout | 18 lines, exit 0 | 18 lines, exit 0 | **MATCH** |
| `rigprog help` | stderr | 0 (empty) | 0 | **MATCH** |
| `rigprog nosuchcmd` | stderr | 20 lines, exit 2 | 20 lines, exit 2 | **MATCH** |
| `rigprog nosuchcmd` | stdout | 0 (empty) | 0 | **MATCH** |

**Line counts and exit codes do not move** — the delta is one line's
CONTENT, on each of three surfaces, and the surfaces keep their shape.

| Recorded value | M9c-6 | This run | Result |
|---|---|---|---|
| `bare/stderr` ≡ `help/stdout` (`cmp`) | identical | identical | **MATCH** |
| that pair's SHA-256 | `50714a88…` | `8723e9d6be03e4ad0a10c0e3d7943fd19b13ebeca3f3840b12c1f25db77fb576` | **DESIGNED DELTA** |
| model-list line, bare/stderr | `…FT-710, FTdx10).` | `…FT-710, FTdx10, FTdx101D, FTdx101MP).` | **DESIGNED DELTA** |
| model-list line, help/stdout | as above | as above | **DESIGNED DELTA** |
| model-list line, unk/stderr | as above | as above | **DESIGNED DELTA** |
| unknown-subcommand first line | `rigprog: unknown subcommand "nosuchcmd"` | identical | **MATCH** |
| `UnknownModelError` rendered text | `…(supported: FT-710, FTdx10)` | `…(supported: FT-710, FTdx10, FTdx101D, FTdx101MP)` | **DESIGNED DELTA** |
| `UnknownModelError` exit / stderr lines | 2 / 16 | 2 / 16 | **MATCH** |

The unknown-subcommand surface's own first line is unchanged, so the
delta really is the model list and not the error prose. The
`UnknownModelError` line in full:

```
BEFORE: rigprog probe: wiring: unknown model "NO-SUCH-MODEL" (supported: FT-710, FTdx10)
AFTER : rigprog probe: wiring: unknown model "NO-SUCH-MODEL" (supported: FT-710, FTdx10, FTdx101D, FTdx101MP)
```

`NO-SUCH-MODEL` is the repo's own `unknownModelSentinel`.

### Proved by removal, on all seven streams

Strip only the model-list line from both sides and the remaining diff is
EMPTY everywhere. Differing-line counts before removal are given first,
so the arithmetic closes:

| Stream | Differing lines before removal | After removal |
|---|---|---|
| `bare.stdout` | 0 | EMPTY AFTER REMOVAL |
| `bare.stderr` | **2** (one changed line) | EMPTY AFTER REMOVAL |
| `help.stdout` | **2** (one changed line) | EMPTY AFTER REMOVAL |
| `help.stderr` | 0 | EMPTY AFTER REMOVAL |
| `unk.stdout` | 0 | EMPTY AFTER REMOVAL |
| `unk.stderr` | **2** (one changed line) | EMPTY AFTER REMOVAL |
| `unm.stderr` (`UnknownModelError`) | **2** (one changed line) | EMPTY AFTER REMOVAL |

```bash
diff <(grep -v '^rigprog is a command-line memory programmer for Yaesu radios (currently: ' base) \
     <(grep -v '^rigprog is a command-line memory programmer for Yaesu radios (currently: ' head)
```

**None of the five Part 1 recipe legs prints this line**, which is why
Part 1's only deltas are CHIRP-class ones and this is a separate part.

### `GetSupportedModels` — the third surface in the class

`app/connection.go:39` returns `wiring.SupportedModels()` directly. It is
a Wails binding with no CLI leg, so no recipe row exists for it and none
is claimed; it is in the class because it interpolates the same list, and
it is pinned by `app/connection_test.go`'s
`TestGetSupportedModels_ContainsDefaultModel` (:131), which asserts
equality with `wiring.SupportedModels()` rather than against a literal —
so it grows with the registry by construction and cannot go stale. The
whole-tree suite (Part 7) runs it green at this tip.

**Part 2 totals: 18 recorded values compared; 13 MATCH, 5 DESIGNED
DELTAS, 0 outside class.**

---

## Part 3 — the FTdx10 baselines: 34 identical, 6 designed deltas

M9c-6 minted these as "the hash table a future milestone diffs", and
M9d-1 diffed them to nothing. M9d-2 moves exactly the six rows its class
names. Run from `<mirror-dx10>/`.

| # | Leg | Recorded exit | Recorded stdout SHA-256 | Result |
|---|---|---|---|---|
| 1 | `probe --fake --model FTdx10` | 0 | `cf9a8b077c09d182bc18a13553e8d1c9ff3291f0d739107a3341ca373d128728` | **MATCH** |
| 2 | `read --fake --model FTdx10` | 0 | `2006a6b4d4b5b72ae43e33b6fb0fd6da1c28b8bacdf5816ba2cdd7121e510757` | **MATCH** |
| 3 | `read --fake --settings` | 0 | `4ddf9915838309571bf0f313cd0c8e9d452bf909ee91d0efbc4d441c95c4e0ab` | **MATCH** |
| 4 | `diff --fake` (fresh read) | 0 | `7917a3bde4d3bec60710504d0cde4682aaafeea573273b5d683803aee2cbbad6` | **MATCH** |
| 5 | `settings` | 0 | `4c70117d9c195e4a5b091a18123d3112ff1a51807a73758b04f63b0c7d6cc1b8` | **MATCH** |
| 6 | `settings --csv` | 0 | `266fbc318032f6ff2c3d5995b3d7113f30ebc780f8171811c3aca518e2b8256a` | **MATCH** |
| 7 | `export` (of the read) | 0 | `e0f0036efba8821306d5bc55cb64074c696bffdf337425ca86ed2a3ac8cf1d4c` | **MATCH** |
| 8 | `import --chirp chirp_minimal.csv` | 0 | `2990ed9d60a81e89aa828effe4220c0d5638a25e1d5bf0464f948e237c9942be` | **MATCH** |
| **9** | **`import --chirp chirp_skip.csv`** | 0 | `bdefb46eda7af6a6fcd1eba8e4aea3ceb7a658a8ddeda999384760ac53dc7b88` | **DESIGNED DELTA** → `6f7afec5b3fe24affed1619857998cc0a3144871bafa1ee6184551bb7b9d90e4` |
| **10** | **`diff` of the clean CHIRP import** | 0 | `f206cd30c873eb137fdc61d35d915b531a5db6c331f04c77388f9c5fc943e609` | **DESIGNED DELTA** → `fa38200db561ae68ba30c17ffdc809a724b5140c3b0f036b758292a5b7a4fe13` |
| **11** | **`diff` of the skip CHIRP import** | 0 | `cd91cdf6550e88e3c9e5471dc39a15524bf3eeeda30f45babbb7a501dada895b` | **DESIGNED DELTA** → `cd157dc6f2c87d0cdf3b26e9fb9aa30ec1b00d81bc57478c9cc46042cc84f48c` |
| 12 | `import --csv` (native round trip) | 0 | `2a45ec33b50709c19382711cdd508ec0741682e74556915a26dd31fb9ad4ba5d` | **MATCH** |
| 13 | `export` of the CHIRP-imported file | 0 | `1281522d68705d7feb5dbe67d3e9bbfdd7928b8ac3ebcefc967dca203c1279ef` | **MATCH** |
| 14 | `export` of the round-tripped file | 0 | `b16c03c9cf7e2062fef1bc782d40333ce138fd6fc1f5faef2b5158af11335dfb` | **MATCH** |

**Fourteen exit values, fourteen unchanged (all 0).** Legs 8, 12, 13 and
14 — the ones the class list explicitly required to be identical — are
identical. Leg 13's stdout is unchanged even though the CSV it WRITES
moved, because export stdout prints only a row count and a path; that
prediction is now measured on both halves.

The nine-entry file-artefact table:

| Artefact | Recorded SHA-256 | Note | Result |
|---|---|---|---|
| `ftdx10-read.json` | `95e2c8fbeaa689d8100392fa986029b63741ec2e7e577025b9836cb815d14737` | `read_at`-normalised | **MATCH** |
| `ftdx10-read-settings.json` | `7be4f8e126e49f1f4cb5a0c488569476fdd27f4c980fd7e839ccf603bf170cd6` | `read_at`-normalised | **MATCH** |
| **`ftdx10-import-chirp.json`** | `35d45120862791e24d6a2fa2c0a05696b8de6fb762642a4b463ebb518388c79e` | `read_at`-normalised | **DESIGNED DELTA** → `95b4f19dbc7012dfe839dd93c8f562e1b3c7fe077a2218d9a7dfca9230e1c130` |
| **`ftdx10-import-skip.json`** | `db120d701a8dd20acff886098a103c78a8ad66b13b20848d4318c1ef35958fc7` | `read_at`-normalised | **DESIGNED DELTA** → `7404462e82f939ffb8eea250d5fd792d42ac140164c7a28ffefe684f56f7bea5` |
| `ftdx10-roundtrip.json` | `ab08427f807995920227e52d398617a540c5f39f7acd7b9654eca6fb25d33099` | `read_at`-normalised | **MATCH** |
| `ftdx10-export.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw | **MATCH** |
| `ftdx10-roundtrip.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw — equal to the line above | **MATCH**, and still equal |
| **`ftdx10-import-chirp.csv`** | `819b4c4ff9e7f11029b6ea65e7f77ab68725ef1fa8413424c9718f3647fc80a5` | raw | **DESIGNED DELTA** → `9efbc427bb9b37789d4e47aec6e6b2ea824b048d3943f5482194417c1363e391` |
| `ftdx10-settings.csv` | `bd4db08f0047225db744b5802269dd81867117bf092920aaaf82badf02f5f390` | raw | **MATCH** |

The three Part 3 stderr hashes M9c-6 records in prose:

| Leg | Recorded stderr SHA-256 | Result |
|---|---|---|
| A1 `probe` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | **MATCH** |
| A2 `read` (117 progress lines) | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | **MATCH** |
| A3 `read --settings` (314 lines) | `8e96308b714b5657582aa6e63080ea23a99e4915d53c665753f5c2522f2cb87e` | **MATCH** |

**Exactly the six rows the class names, and no others.**

### Leg 9, verbatim: three honest loss lines where there were none

```
BEFORE:
No CHIRP import loss.
offline validation notes — authoritative validation happens at write time against the connected radio:
  none.
Output: .capture/ftdx10-import-skip.json

AFTER:
CHIRP import loss:
  line 2, column Skip, value "S": dropped — CHIRP Skip "S" dropped: scan-skip is not reachable over CAT on FTdx10; scan-skip left unresolved
  line 3, column Skip, value "S": dropped — CHIRP Skip "S" dropped: scan-skip is not reachable over CAT on FTdx10; scan-skip left unresolved
  line 4, column Skip, value "S": dropped — CHIRP Skip "S" dropped: scan-skip is not reachable over CAT on FTdx10; scan-skip left unresolved
Output: .capture/ftdx10-import-skip.json
```

The exit code is **0 on both sides**. M9c-6 recorded this leg's old
behaviour — "It **imports cleanly** … the refusal is not an import-time
one" — with a raised eyebrow: a file explicitly asking for scan-skip on
a radio that cannot do it reported no loss at all. It now reports the
loss, and still does not refuse the import.

### Legs 10 and 11, verbatim: `Blocked 3` → `Blocked 0`

**BEFORE** (leg 10, the clean fixture — this is M9c-6's recorded text,
reproduced by the Part 4 removal proof):

```
Added:
  M-02: freq 7150000 Hz, mode LSB, tag "MINIMALLSB"
    BLOCKED: scan_skip not writable on this radio
  M-03: freq 14050000 Hz, mode CW-U, tag "MINIMALCW"
    BLOCKED: scan_skip not writable on this radio
Modified:
  M-01: freq 7000000→14200000 Hz, mode LSB→USB, tag ""→"MINIMALUSB"
    BLOCKED: scan_skip not writable on this radio

Added 2, Modified 1, Erased 0, Blocked 3, Unchanged 114
```

**AFTER** (leg 10, this gate's head):

```
Added:
  M-02: freq 7150000 Hz, mode LSB, tag "MINIMALLSB"
  M-03: freq 14050000 Hz, mode CW-U, tag "MINIMALCW"
Modified:
  M-01: freq 7000000→14200000 Hz, mode LSB→USB, tag ""→"MINIMALUSB"

Added 2, Modified 1, Erased 0, Blocked 0, Unchanged 114
```

Leg 11 is the identical shape with the skip fixture's own tags
(`SKIPLSB`, `SKIPCW`, `SKIPUSB`) and the same `Blocked 3` → `Blocked 0`.
**Added, Modified, Erased and Unchanged do not move — only `Blocked`,
and only because the three `BLOCKED:` reason lines are gone.**

**This closes M9c-6's A7 ledger entry.** That manifest recorded, as a
finding it deliberately did not fix, that "a CHIRP file cannot today be
imported and then SENT to either radio without the user first resolving
a per-channel state the protocol cannot carry", and that the FTdx10's
route out was NARROWER than the FT-710's because `scan_skip` has no
in-grid route to a writable value. Task 8 removed the cause rather than
widening the route: a blank `Skip` cell no longer manufactures a Known
state for a field the radio cannot reach, so there is no longer a
question outstanding to resolve. The FTdx10 CHIRP import is now
sendable.

### The three artefacts, verbatim

`ftdx10-import-chirp.json` and `ftdx10-import-skip.json` — by `jq` over
the three imported slots, both files, both constructions:

```
BEFORE: {"slot":"001","tag":"MINIMALUSB","tag_display":{"state":"unavailable"},"scan_skip":{"state":"known"}}
AFTER : {"slot":"001","tag":"MINIMALUSB","tag_display":{"state":"unavailable"},"scan_skip":{"state":"unknown"}}
```

`tag_display` does not move. Across all 21 populated channels the
`scan_skip` state is now `unknown` — 18 read-derived (already Unknown,
unmoved) and 3 CHIRP-derived (moved).

`ftdx10-import-chirp.csv` — the `scan_skip` column:

```
BEFORE: 001,M-01,14200000,USB,,,,OFF,,SIMPLEX,MINIMALUSB,n/a,no
AFTER : 001,M-01,14200000,USB,,,,OFF,,SIMPLEX,MINIMALUSB,n/a,
```

Column tallies over all 117 rows confirm the whole of the movement:
`scan_skip` was 21 × `no` + 96 blank, and is now **117 blank**;
`tag_display` is **21 × `n/a` + 96 blank on BOTH sides**, reproducing
M9c-6's recorded A6a property exactly.

**Part 3 totals: 40 recorded values compared (26 hashes + 14 exits); 34
MATCH, 6 DESIGNED DELTAS, 0 outside class.**

---

## The comparison, counted

| Group | Values compared | Identical | Designed deltas | Outside class |
|---|---|---|---|---|
| Recipe inputs (3 fixtures) | 3 | 3 | 0 | **0** |
| Part 1 — hashes (the eighteen + S1's 3 + S2's 2) | 23 | 21 | 2 | **0** |
| Part 1 — exit literals (5 in the eighteen, 1 S1, 2 S2) | 8 | 8 | 0 | **0** |
| Part 2 — model-list surfaces | 18 | 13 | 5 | **0** |
| Part 3 — the fourteen legs | 14 | 11 | 3 | **0** |
| Part 3 — nine file artefacts | 9 | 6 | 3 | **0** |
| Part 3 — three stderr hashes | 3 | 3 | 0 | **0** |
| Part 3 — fourteen exit values | 14 | 14 | 0 | **0** |
| M9c-6 Part 5 generated inventories | 3 | 3 | 0 | **0** |
| **Total** | **95** | **82** | **13** | **0** |

**95 recorded values compared. 82 identical. 13 designed deltas, every
one inside its adjudicated class, every one recorded verbatim above and
proved by removal below. ZERO rows outside class.** No carve-out was
invoked and nothing outside the M9c-6 manifest's own tables is claimed.

The three M9c-6 Part 5 values, compared directly rather than only via
`git diff --exit-code` (which proves unchanged-since-HEAD, a weaker
statement):

| Generated file | M9c-6 recorded SHA-256 | Result |
|---|---|---|
| `internal/fakedx10/exinventory_gen.go` | `0d44f04bef5ece957fc324c8350cef9af5a9d6899b83639a2ad3bdff803dad48` | **MATCH** |
| `core/cat/ftdx10/exinventory_gen.go` | `9311fc928b540110539d5dc40c921193b39890fbd6ef8c6f27c1e0db3c2171d4` | **MATCH** |
| `core/cat/exinventory_gen.go` | `fbf4f02e3e564357eecd020f612de58cd3d26ad58a8a6e8ee9cb1249815ada22` | **MATCH** |

The fakedx10 value is the one M9c-6's task 5 recorded — the staleness
property now holding across two further milestones.

---

## Part 4 — the deltas proved by REMOVAL, not inferred from a hash

Every one of the 13 designed deltas was reproduced back to M9c-6's
recorded value by reverting the commit that caused it, in a throwaway
worktree, and re-running the affected legs. **13 deltas, 13 removal
proofs, 13 reproductions.** Both worktrees were then discarded
(`git worktree list` shows only the repository).

### Proof A — the CHIRP-skip class: revert Task 8

```bash
git worktree add <scratch>/rev-chirp 1564d31
cd <scratch>/rev-chirp
git revert --no-commit 5a36ed6           # → CONFLICT in core/csvio/chirp_test.go
git revert --abort
git revert --no-commit 1564d31 5a36ed6   # → clean
```

**Reverting `5a36ed6` alone was attempted first and CONFLICTED**, as the
brief allowed for: `1564d31` (Task 8's comment-only fix round) edits the
same test file. Both Task 8 commits were therefore reverted together,
and this is recorded rather than glossed. That the pair is a faithful
removal is not asserted but measured — the reverted tree is
byte-identical to `96e79fd`, the parent of `5a36ed6`:

```
$ git diff --stat 96e79fd
$ echo $?
0        # empty diff: the tree IS the pre-Task-8 commit
```

`1564d31` touches one file, `core/csvio/chirp_test.go`, and no
production code, so its inclusion cannot affect a CLI artefact.

A binary was built from that worktree and the eight CHIRP-class rows
re-run through the same mirror-tree recipe:

| Row | M9c-6 recorded value | Reverted binary |
|---|---|---|
| P1 row 7 `import.stdout` | `fa5ee2aa…` | **REPRODUCED** |
| P1 row 18 `import-min.json` | `03bd53ae…` | **REPRODUCED** |
| P3 leg 9 `import --chirp chirp_skip` | `bdefb46e…` | **REPRODUCED** |
| P3 leg 10 `diff` clean CHIRP | `f206cd30…` | **REPRODUCED** |
| P3 leg 11 `diff` skip CHIRP | `cd91cdf6…` | **REPRODUCED** |
| P3 `ftdx10-import-chirp.json` | `35d45120…` | **REPRODUCED** |
| P3 `ftdx10-import-skip.json` | `db120d70…` | **REPRODUCED** |
| P3 `ftdx10-import-chirp.csv` | `819b4c4f…` | **REPRODUCED** |

```
-----------------------------------------
rows compared : 8
MATCH         : 8
MISMATCH      : 0
MISSING       : 0
```

**All eight CHIRP-class rows are caused by Task 8's construction change
and by nothing else.** The old verbatim values quoted throughout Part 1
and Part 3 above are this run's output, not a transcription from an
older document.

### Proof B — the model-list class: revert Task 7

```bash
git worktree add <scratch>/rev-reg 1564d31
cd <scratch>/rev-reg
git revert --no-commit df8f44b                   # → CONFLICT in internal/radiotext/radiotext.go
git revert --abort
git revert --no-commit 96e79fd c5ba2ad df8f44b   # → clean (Task 7's three commits)
```

Reverting the registration commit alone conflicted with Task 7's own two
follow-up commits, which edit the same files; the whole Task 7 trio was
reverted instead, and the restoration verified against `2c12227`, the
parent of `df8f44b`:

```
$ git diff --stat 2c12227 -- internal/wiring/ internal/radiotext/ app/ internal/guards/
(empty)
```

The residual diff under those roots is Task 8's frontend and `uispec.go`
work, which is a different commit and a different class — correctly left
in place, so the model-list class is isolated.

One check was made before trusting the proof: `cmd/rigprog/usage.go` is
also touched across the Task 7 trio, which would confound an
"empty after removal" result if the usage TEXT had changed. It has not —
`git diff 2c12227..1564d31 -- cmd/rigprog/usage.go` is a comment-only
change to the template's doc comment, and `topUsageTextTemplate`'s
string literal is untouched. (The comment change is itself of interest;
see the ledger, note 5.)

With Task 7 removed, every model-list value reproduces M9c-6's recorded
one:

| Recorded value | M9c-6 | Reverted binary |
|---|---|---|
| `bare/stderr` ≡ `help/stdout` SHA-256 | `50714a88…` (prefix recorded) | `50714a88db8725654e37076afdf4cffa9931b69cee939df1fd070127e3d1b389` — **REPRODUCED**, full hash |
| model-list line (all three surfaces) | `…(currently: FT-710, FTdx10).` | **REPRODUCED** |
| `UnknownModelError` text | `…(supported: FT-710, FTdx10)` | **REPRODUCED** |
| `cmp bare.stderr help.stdout` | identical | **REPRODUCED** |
| line counts / exits (6 surfaces) | 18/18/20/16, 2/0/2/2 | **REPRODUCED** |

**All five model-list deltas are caused by the registration rows and by
nothing else.** The full 64-hex hash reproducing M9d-1's recorded value
is a stronger result than the eight-digit prefix M9c-6 published, and it
is recorded here so a future milestone can compare against either.

---

## Part 5 — the FIRST FTdx101D and FTdx101MP baselines

Head binary only — there is no base to compare against, because at
`6a4e5e4` no CLI path could select either model. These are the baselines
a future milestone will be measured against, so they are recorded with
hashes and with the verdict lines verbatim, in the M9c-6 Part 3
fourteen-leg form, once per sibling. Run from `<mirror-d>/` and
`<mirror-mp>/`, each holding `.capture/` and the two CHIRP fixtures at
their repo-relative paths.

**Every expected internal check the plan named is met, on both
siblings:** 117 channels (99 MEM + 18 PMS), 193 settings, descriptor
`ftdx101-ex@1`, `Blocked 0` on the fresh-read diff, and the CHIRP legs
**born caps-aware** — they carry no delta, because Task 8 landed before
these baselines were minted, which is exactly why the plan ordered it
that way.

### D1 / M1 — `probe --fake --model FTdx101{D,MP}` → exit 0

The FTdx101D's, verbatim:

```
Model:         FTdx101D
CAT ID:        0681
Port:          fake
USB serial:    SIM0001
Region:        -
60 m channels: 0
EMG channel:   no
Unexpected frames: 0

Firmware version has no CAT query on the FTdx101D, and no minimum version is established for it — read it off the radio's display. If nothing answered on this port at all, check which port it is: this radio presents two virtual COM ports, and only the Enhanced COM Port carries CAT. The Standard COM Port is for TX control (PTT, CW keying, digital modes) and will answer nothing here, which looks exactly like a wrong baud rate.
```

The FTdx101MP's is the same ten lines with `FTdx101MP` for `FTdx101D`
(twice — the `Model:` line and the firmware note) and `0682` for `0681`.
That is not a summary: it is measured, and proved by removal, in the
"D-versus-MP identity" subsection below.

`CAT ID: 0681` / `0682` is the whole of the two radios' difference on the
wire. `Region: -` is the driver implementing no `RegionReporter`.
`60 m channels: 0` / `EMG channel: no` is the full-range discovery walk
finding nothing in the default fake image — an EMPTY result reported
honestly, not an absent capability. The firmware note carries the spec
§3.12 two-COM-port warning, which is this milestone's own addition and
the one place a registered radio's probe now tells a user something the
FT-710's and FTdx10's cannot.

### D2 / M2 — `read` → exit 0, 117 slots

In the blocks below, `{d,mp}` is **shorthand for the two per-model
paths** — each run echoed its own `--out` path literally, and those
literal path strings are why the two siblings' stdout hashes differ in
the tables that follow. The identity subsection at the end of this part
re-runs both with character-identical paths so the genuine difference
can be seen on its own.

```
Slots read:      117
Populated:       19
Region:          -
Baseline digest: ddfbd375f6ae (truncated)
Output:          .capture/ftdx101{d,mp}-read.json
```

stderr is **117 progress lines** on both, and its SHA-256
`8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` is
**byte-identical to the FT-710's own `read.stderr`** (Part 1 row 5) and
to the FTdx10's — verified by `cmp`, not by eye. Four radios now present
117 static slots with the same display names through one model-neutral
read path.

| Property | FTdx101D | FTdx101MP |
|---|---|---|
| `schema` | `3` | `3` |
| `radio.model` | `FTdx101D` | `FTdx101MP` |
| `radio.baseline_digest` | `ddfbd375f6aed6570598145c463dd59503678050a6d0368a645ed8ac74ca6297` | same |
| channels / populated | 117 / 19 | 117 / 19 |

**117 = 99 MEM (001-099) + 18 PMS (9 pairs).** The `baseline_digest` is
a CONTENT digest over the channel data and carries no wall clock; it is
identical between the siblings **and identical to the FTdx10's recorded
`ddfbd375f6ae…`**, because all three fakes present the same minimal
default image (M-01 plus the nine PMS pairs) and the digest does not
include the model name. That is a property of the fakes, not a claim
about the radios, and it is recorded here so a future reader does not
mistake it for a bug.

### D3 / M3 — `read --settings` → exit 0, **193 settings**

```
Slots read:      117
Populated:       19
Region:          -
Baseline digest: ddfbd375f6ae (truncated)
Output:          .capture/ftdx101{d,mp}-read-settings.json
Settings read:        193
Settings unavailable: 0
```

`menus.entries` **193**; `menus.complete` **true**; `menus.descriptor`
**`ftdx101-ex@1`** — all three as the plan required. stderr is **310
lines** (117 channel + 193 settings progress) on both, hashing
`3c1266fffded112aed2b32db9f6f3862d491ade09a62a1f54a54938007812f11`.

193 is the whole inventory read over the wire, one EX exchange per item,
zero unavailable — against the FTdx10's 197, which is the real
difference between the two radios' menu sets and not a truncation.

### D4 / M4 — `diff` of the fresh read → exit 0, **`Blocked 0`**

```
No changes.

Added 0, Modified 0, Erased 0, Blocked 0, Unchanged 117
```

Identical text on both siblings and **byte-identical to the FTdx10's
recorded leg-4 hash** `7917a3bde4d3bec60710504d0cde4682aaafeea573273b5d683803aee2cbbad6`
— the verdict is model-neutral prose, so equality here is expected and
is recorded as a property rather than a coincidence.

`Blocked 0` on all 117 slots is the load-bearing number: it is the
TagDisplay-Unavailable decision demonstrated rather than argued. Had the
read produced `Unknown` for `tag_display`, every populated channel would
be blocked by a gate the user could only clear by asserting a value the
radio cannot store.

### D5 / M5 — `settings` and `settings --csv` → exit 0

`settings --model FTdx101{D,MP}` renders **218 lines: 4 menus, 18
groups, 193 item lines, and NO "Unrecognised settings" section**, stdout
`0f13e3203a6df9730faa6e920c8a5fc6bd6961bb353bafece3d4393ba6f42e7b` on
BOTH siblings. First lines:

```
RADIO SETTING
  MODE SSB
    01-01-01  AGC FAST DELAY  0000  
    01-01-02  AGC MID DELAY   0000  
```

`settings --csv` writes **194 lines** (header + 193), columns
`id,menu,group,label,state,value`, the CSV
`fc595c49141b8a50374754a0aeace011467f5c480db4df664e6a7f71cba58b82` on
both.

**Non-vacuity of `--model`, measured.** The same file rendered WITHOUT
the flag (defaulting to FT-710) also exits 0 — but produces an
`Unrecognised settings` section with **18 entries** and **217 lines**.
With the flag: 0 unrecognised, 218 lines. The flag is doing real work:
it selects the FTdx101 descriptor, and only that descriptor recognises
all 193.

The EX values shown (`0000`, `0`, …) are the fake's INVENTED defaults —
fakedx101's ASSUMED-register entry 4. This baseline records what the
FAKE answers; it is not a claim about any real FTdx101's menu values.

### D6 / M6 — export, the CHIRP legs and the native round trip

The CHIRP legs are **born caps-aware**. The clean import reports no
loss; the skip fixture reports its three drops honestly, naming the
model; and BOTH diffs come out at `Blocked 0`:

```
$ rigprog import --chirp docs/superpowers/m9c6-fixtures/chirp_skip.csv --model FTdx101D …
CHIRP import loss:
  line 2, column Skip, value "S": dropped — CHIRP Skip "S" dropped: scan-skip is not reachable over CAT on FTdx101D; scan-skip left unresolved
  line 3, column Skip, value "S": dropped — …
  line 4, column Skip, value "S": dropped — …
offline validation notes — authoritative validation happens at write time against the connected radio:
  none.

$ rigprog diff --fake --model FTdx101D .capture/ftdx101d-import-skip.json      # exit 0
Added:
  M-02: freq 7150000 Hz, mode LSB, tag "SKIPLSB"
  M-03: freq 14050000 Hz, mode CW-U, tag "SKIPCW"
Modified:
  M-01: freq 7000000→14200000 Hz, mode LSB→USB, tag ""→"SKIPUSB"

Added 2, Modified 1, Erased 0, Blocked 0, Unchanged 114
```

The clean-fixture diff (leg 10) is the same shape with the minimal
fixture's tags, and also `Blocked 0`. **These models never had a
`Blocked 3` baseline to carry, because Task 8 landed before Task 9** —
where the FTdx10's tables above record a `Blocked 3` → `Blocked 0`
transition, the siblings simply have no `BLOCKED:` line and never did.
The `scan_skip` CSV column is blank on all 117 rows and `tag_display` is
`n/a` × 21 + blank × 96, on both siblings.

**The native CSV round trip is LOSSLESS, two ways, on both siblings:**
re-exporting the round-tripped file is `cmp`-identical to the original
export; and the JSON differs from the source read in **exactly one
line**, `"generator"` (`open-rig-programmer/core/clone` → `rigprog/dev`),
with `diff <(grep -v '"generator":' a) <(grep -v '"generator":' b)`
**EMPTY**.

### FTdx101D — the hash table a future milestone diffs

| Leg | Exit | stdout SHA-256 |
|---|---|---|
| `probe --fake --model FTdx101D` | 0 | `e771984fd1af64b83e75cfaccd209d16cdd4efb6da756b69adcdd77f3777431e` |
| `read --fake --model FTdx101D` | 0 | `abceff8553e5956666fb8703db497b637a54db28581ef68e2362b05633ff5da9` |
| `read --fake --settings` | 0 | `dcdcf9ff86ccf0843f4d4b23cd0a01e715c1e1b14b2f095e3944ee0590f550b1` |
| `diff --fake` (fresh read) | 0 | `7917a3bde4d3bec60710504d0cde4682aaafeea573273b5d683803aee2cbbad6` |
| `settings` | 0 | `0f13e3203a6df9730faa6e920c8a5fc6bd6961bb353bafece3d4393ba6f42e7b` |
| `settings --csv` | 0 | `d8ebc3e552abdc52d81501552b46feef293c8a6c2ecbb0febb7f097a15e02906` |
| `export` (of the read) | 0 | `e30c19d4b4e18d7c42684a0f170ea498833371160e9aa480b07f86c6e887fa19` |
| `import --chirp chirp_minimal.csv` | 0 | `90ba614fe9ea0551f1f1f9b8b6e51f1d6ab6a1812184161c498f3ebd59ad02a6` |
| `import --chirp chirp_skip.csv` | 0 | `b9b2cf36b9e53828e4bce015f912023eb378b160f5c28ac9af98fd47cd6cec72` |
| `diff` of the clean CHIRP import | 0 | `fa38200db561ae68ba30c17ffdc809a724b5140c3b0f036b758292a5b7a4fe13` |
| `diff` of the skip CHIRP import | 0 | `cd157dc6f2c87d0cdf3b26e9fb9aa30ec1b00d81bc57478c9cc46042cc84f48c` |
| `import --csv` (native round trip) | 0 | `7f6c3056b3fc0ef39721a0dcad73654662ee0223d11c202afc49b3ec8988a2b9` |
| `export` of the CHIRP-imported file | 0 | `082f262ebd2ef3bdd1f11575bcd15b00f372b00ae7213a6a5bd03dbcbeb269c9` |
| `export` of the round-tripped file | 0 | `cbb473fc2981ac02d98215c4848312f62a8d6c32c1fbe3d7a112d69644504842` |

**Fourteen legs, every one exit 0.** File artefacts:

| Artefact | SHA-256 | Note |
|---|---|---|
| `ftdx101d-read.json` | `eedffa55960ba7d6867b9f1926cbdf38442ca4c8911e4bd235044e48373ce2a5` | `read_at`-normalised |
| `ftdx101d-read-settings.json` | `2b1488b3538e7917ae11cdf9ae08c190c4fd230232f56ac7891a90585085c48c` | `read_at`-normalised |
| `ftdx101d-import-chirp.json` | `b43c9452ff44390dbb2ee78acef185c69cce7c9b80ac0484a634cbe95d5c6621` | `read_at`-normalised |
| `ftdx101d-import-skip.json` | `8105be4b74c8466d676f882d53485a1dbd29e96aad14fef2639004009f4e9b52` | `read_at`-normalised |
| `ftdx101d-roundtrip.json` | `085ed6ad6cd8eb2f1ae3a6bb7d373ed00ac3b57439feb9c99b8da2bdcd2cafb7` | `read_at`-normalised |
| `ftdx101d-export.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw |
| `ftdx101d-roundtrip.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw — **equal to the line above** |
| `ftdx101d-import-chirp.csv` | `9efbc427bb9b37789d4e47aec6e6b2ea824b048d3943f5482194417c1363e391` | raw |
| `ftdx101d-settings.csv` | `fc595c49141b8a50374754a0aeace011467f5c480db4df664e6a7f71cba58b82` | raw |

stderr: probe `e3b0c442…` (empty), read `8e174833…` (117 lines),
read --settings `3c1266ff…` (310 lines).

### FTdx101MP — the hash table a future milestone diffs

| Leg | Exit | stdout SHA-256 |
|---|---|---|
| `probe --fake --model FTdx101MP` | 0 | `545bc48deba5cc1097ba69a0afb2e33be3a634a6241caa5b9c7aeadfdd5fa095` |
| `read --fake --model FTdx101MP` | 0 | `bfec916cb09e26ae9c73fc8817a2ca23da676e135fa8a688fc510aab1ec2a4be` |
| `read --fake --settings` | 0 | `e7e81bc150512d256442d224602b21d551cafac8e67159d312d2022e3abd5bf5` |
| `diff --fake` (fresh read) | 0 | `7917a3bde4d3bec60710504d0cde4682aaafeea573273b5d683803aee2cbbad6` |
| `settings` | 0 | `0f13e3203a6df9730faa6e920c8a5fc6bd6961bb353bafece3d4393ba6f42e7b` |
| `settings --csv` | 0 | `32a012304dc092da0dbf1409e11286baac7c4374c722cd97b163260748ca6996` |
| `export` (of the read) | 0 | `783bea4580dcfba6de43bee996571c9273b73aa6b496aef3c84d4f27abae900f` |
| `import --chirp chirp_minimal.csv` | 0 | `78f40e2e7fce3d4d800f1ff4603a30eed2055a0f7fec14002a66a9699df89350` |
| `import --chirp chirp_skip.csv` | 0 | `b6b98c21e7db05427a5be15436f329cbe512873c0eb7f515602787e1b87e48df` |
| `diff` of the clean CHIRP import | 0 | `fa38200db561ae68ba30c17ffdc809a724b5140c3b0f036b758292a5b7a4fe13` |
| `diff` of the skip CHIRP import | 0 | `cd157dc6f2c87d0cdf3b26e9fb9aa30ec1b00d81bc57478c9cc46042cc84f48c` |
| `import --csv` (native round trip) | 0 | `8456cf8463a301c895b72b4d4a822f60e2736b3d3a0e29da4c1de6f6cd5ca879` |
| `export` of the CHIRP-imported file | 0 | `65d900b3134f33c1a8701935b7573ad47f550f435506989a8f8a111eda585c12` |
| `export` of the round-tripped file | 0 | `201c10c2c1f2ebd08dafebdef23765e0c098bd94d8448eebd2a2ad8ca0a9fd53` |

**Fourteen legs, every one exit 0.** File artefacts:

| Artefact | SHA-256 | Note |
|---|---|---|
| `ftdx101mp-read.json` | `c16cbfa12557648f4a9bff118c96fa3fd87d8e4e85e3e7be53928a6249761fbf` | `read_at`-normalised |
| `ftdx101mp-read-settings.json` | `f3e505cd535b734332790a86d1f4c745e1bd07019e430899f1dd030730de09b9` | `read_at`-normalised |
| `ftdx101mp-import-chirp.json` | `e18aa8237ce03fe67492fae5894afd7acb17a9b4e7e05affaad186057515bf57` | `read_at`-normalised |
| `ftdx101mp-import-skip.json` | `eadbae77b06503b4ba3292f92da3c465800488acf800d5394d96b9818ecf7d63` | `read_at`-normalised |
| `ftdx101mp-roundtrip.json` | `86441fb0e4fc2696fd5f06412a33d95736b4a9cd23fd7d75c1b9b7b79ab1b6e0` | `read_at`-normalised |
| `ftdx101mp-export.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw |
| `ftdx101mp-roundtrip.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw — **equal to the line above** |
| `ftdx101mp-import-chirp.csv` | `9efbc427bb9b37789d4e47aec6e6b2ea824b048d3943f5482194417c1363e391` | raw |
| `ftdx101mp-settings.csv` | `fc595c49141b8a50374754a0aeace011467f5c480db4df664e6a7f71cba58b82` | raw |

stderr: identical to the D's three, by `cmp`.

### D-versus-MP identity, measured with character-identical paths

The two tables above differ in more rows than the radios do, because
each sibling wrote to its own `--out` paths and the CLI echoes them. To
separate the genuine difference from the filenames, both siblings were
re-run in two further mirror trees using an **identical** capture prefix,
so every path string passed to the CLI is character-for-character the
same on both sides.

**Result: of 54 captured artefacts, 47 are byte-identical between the D
and the MP.** The 7 that differ do so in exactly two tokens:

| Artefact | Differing lines | What differs |
|---|---|---|
| `probe.stdout` | 6 | `Model:`, `CAT ID:`, and the model name inside the firmware note |
| `import --chirp chirp_skip` stdout | 6 | the model name in each of the three loss lines |
| `read.json`, `read-settings.json`, `import-chirp.json`, `import-skip.json`, `roundtrip.json` | 4 each | `"model"` and `"cat_id"` |

**Proved by removal:** replace the model names with a common token and
the CAT IDs with a common token, and the diff is EMPTY on all seven:

```
probe.stdout                     EMPTY AFTER REMOVAL
import-skip stdout               EMPTY AFTER REMOVAL
read.norm.json                   EMPTY AFTER REMOVAL
read-settings.norm.json          EMPTY AFTER REMOVAL
import-chirp.norm.json           EMPTY AFTER REMOVAL
import-skip.norm.json            EMPTY AFTER REMOVAL
roundtrip.norm.json              EMPTY AFTER REMOVAL
```

**The two radios differ, end to end through the CLI, in their model name
and their CAT ID and in nothing else.** That is the one-driver,
two-registrations design made observable rather than asserted — and it
is the reason the registration deliberately provides no shared
`"FTdx101"` handle: there are two models, and a user must say which.

### What these baselines do NOT support

No real FTdx101 was involved and none can be: `writeTrialsComplete=false`
is pinned, the RealHardware profile reports Unverified write support, and
the capability gate refuses before a frame is built. **Every value above
is the FAKE's answer**, the fake's EX values are invented by the
documented convention, and the entire FTdx101 dialect is
manual-derived and UNVERIFIED — no FTdx101 has ever been asked anything
by this project. The write path is exercised only through the Simulated
profile in unit tests
(`TestOpenFakeSessionFor_FTdx101DSimulatedWriteRoundTrip` and its MP
twin), never by this recipe. These are baselines for REGRESSION
detection, not evidence about hardware.

**80 new recorded values: 2 models × (14 stdout hashes + 14 exit values
+ 9 file artefacts + 3 stderr hashes).**

---

## Part 6 — the red-proof index

Every guard, pin and cross-check this milestone added was shown to FIRE
on a deliberate violation, per the M9c-4/M9c-6 discipline. **Eleven
mutations were run at THIS task**, in a throwaway worktree at `1564d31`,
each reverted and each revert verified byte-exact by SHA-256 on all five
touched files. **Nine further rows are CITED from the tasks that ran
them**, with the commit that holds the transcript — this index is the
map, and a re-run of an already-recorded proof would add nothing.

| # | Commit | Defect injected | What fired |
|---|---|---|---|
| **Run at Task 9 — the registration surface** | | | |
| 9.1 | `df8f44b` | **FTdx101D removed from BOTH tables** | **ONLY `TestSupportedModels_ContainsEveryRegisteredModel` (the presence pin).** `TestRealAndFakeDriverTablesAgree`, `TestDriverTableKeysMatchDriverModel`, `TestOpenFakeSessionFor_EveryRegisteredModel` and `TestEverySupportedModelHasRadiotext` **ALL PASSED** — the last even ran its walk over the surviving three and was satisfied. This reproduces M9c-6's row 6.4 on a second registration and remains the most valuable row in the index. |
| 9.2 | `df8f44b` | FTdx101MP removed from `realDrivers` only | `TestRealAndFakeDriverTablesAgree` — *"model \"FTdx101MP\" is in fakeDrivers but not realDrivers"* — plus the presence pin |
| 9.3 | `df8f44b` | key const respelt `"FTDX101D"`, drivers still `Model()=="FTdx101D"` | `TestDriverTableKeysMatchDriverModel` (both tables named separately), plus the presence pin and `TestEverySupportedModelHasRadiotext` |
| 9.4 | `df8f44b` | the `"FTdx101MP"` radiotext entry deleted | `TestEverySupportedModelHasRadiotext`, `TestRadiotext_FTdx101MPVerbatim` and `TestRadiotext_FTdx101DAndMPDifferOnlyInTheModelName` — and the D's own verbatim test PASSED, so the failure is attributable to the deleted row and not to a broken walk |
| **Run at Task 9 — the guard's two ftdx101 rows, THREE mutations** | | | |
| 9.5 | `df8f44b` | a second `ftdx101.Simulated` reference in a second non-test file | `TestSimulatedProfileTokensConfinement` — *"appears in 2 non-test files repo-wide ([internal/wiring/fake.go internal/wiring/scratch_leak.go]), want exactly 1"*, reported **TWICE, once per ftdx101 row** |
| 9.6 | `df8f44b` | the `fakedx101.NewD` CALL renamed (token reference left intact) | the **D row's** ctor clause alone: *"the sole file referencing ftdx101.Simulated … does not also call fakedx101.NewD"* |
| 9.7 | `df8f44b` | the `fakedx101.NewMP` CALL renamed (token reference left intact) | the **MP row's** ctor clause alone: *"… does not also call fakedx101.NewMP"* |
| **Run at Task 9 — the driver and option seams** | | | |
| 9.8 | `e123f04` | one word changed in `WrongRadioError.Error()`'s **ID-only** branch | `TestWrongRadioError_Text` — 3 of its 4 subtests, the "both names" subtest correctly PASSING, so the ID-only text is pinned independently of the named form |
| 9.9 | `df8f44b` | the MP's `fakeDrivers` row made to read `FTdx101DFakeSessionOpts` | **BOTH** no-leakage tests, in both directions: the D's fires on *"another model's option source reached this model's fake rig"*, the MP's on *"this model's own option source did not reach its fake rig"* |
| 9.10 | `e6cfe4f` | the sibling refusal weakened to accept any KNOWN sibling ID | `TestOpen_WrongRadio` **and** `TestOpen_WrongRadio_RendersBothNames`, on the `FTdx101D` and `FTdx101MP` subtests alike; the "foreign radio" subtests correctly PASSED |
| 9.11 | `e6cfe4f` | `GotModel` forced empty at the refusal site | the same two tests, both directions — *"refusal text … does not contain \"FTdx101MP\" — a refusal between two radios whose IDs differ in one character must name both models"* |
| **Cited — Task 5, `4a42e07`** | | | |
| 5.1 | `4a42e07` | `internal/fakedx101/gen/main.go` importing `internal/extable` | `TestNoCoreImports` — **the fence covered `gen/` before task 6 created it** |
| 5.2 | `4a42e07` | `buildIDAnswer` returning a hardcoded `"0761"` | `TestID_AnswersTheModelsOwnCATID` (both subtests) and `TestTheTwoModelsDifferOnlyInTheIDAnswer` |
| 5.3 | `4a42e07` | `handleFrame` matching case-sensitively | `TestCommandNamesAreAcceptedInEitherCase` (4 failures) |
| 5.4 | `4a42e07` | `parseMemoryBlock` zeroing the clarifier | `TestMTSet_RoundTripsByteFaithfully` (positions 14-18) |
| **Cited — Task 6, `5dbfd7a`** | | | |
| 6.1 | `5dbfd7a` | a project-internal import inside `gen/` | `TestNoCoreImports`, naming the file |
| 6.2 | `5dbfd7a` | one width token `1`→`2` at address 010105 | `TestGeneratedInventory_NotStale` — *"committed 3552 bytes, regenerated 3552 bytes … First divergence: at byte 1449"*. **Same length, one byte** — a size check would have missed it. Also fired `TestFTdx101TranscriptionBCopy_ByteIdenticalToTheDialects`. |
| 6.3 | `5dbfd7a` | the same divergence, regenerated so the fake really carried it | `TestEXInventoryCrossCheck_FTdx101WidthsAndShapesAgree` (*"dialect Digits=1, fake default is 2 bytes"*) **and** `TestEXFTdx101RoundTrip_AllAddressesRawPort` — table leg and wire leg, independently |
| **Cited — Task 7, `df8f44b`** | | | |
| 7.1 | `df8f44b` | presence pin landed FIRST, before any table row | **TWO reds captured in order**: the constant-spelling asserts did not compile, then *"SupportedModels() = [FT-710 FTdx10], want it to contain \"FTdx101D\""* |
| 7.2 | `df8f44b` | `fakeDrivers` crossed: `fakedx101.NewD` → `NewMP`, D's option var kept | `TestOpenFakeSessionFor_EveryRegisteredModel/FTdx101D`, failing **at `Open`** — one layer earlier than the `Identity().CATID` assertion, because the driver's `caps.CATID` and the rig's answer disagree |

**Totals: 20 rows across five commits, every one fired** — 21 injected
defects in all, since row 7.1 records two reds captured in order.
**Eleven mutations were run and reverted at this task** with byte-exact
restoration verified
(`shasum -a 256 -c` OK on `internal/wiring/wiring.go`,
`internal/wiring/fake.go`, `internal/radiotext/radiotext.go`,
`core/driver/driver.go` and `app/connection.go` after every proof;
`git status --short` empty in the mutation worktree before it was
removed). The other nine rows are cited from their own tasks' recorded
transcripts.

**Why the guard needed three mutations and not one.** The two ftdx101
rows share a package and a token and differ only in `fakeCtor`. The
guard's loop `continue`s after a cardinality failure
(`internal/guards/simulated_tokens_test.go:151-157` — the `continue` at
:153 sits between the two clauses), so a
cardinality mutation **never reaches the ctor clause** and cannot prove
it. Proof 9.5 fires the cardinality clause twice, proving both rows
walk; proofs 9.6 and 9.7 then fire one ctor clause each, proving the
second row earns its place. A single mutation would have left the
pairing clause — the whole reason a row is
`(package, token, constructor)` rather than `(package, token)` —
unproven.

---

## Part 7 — the full local gate at the tip

**All items run at `1564d31` in the repo working tree.** Each line below
is the actual final result of the actual run, taken from the scrollback
of THIS task — the M9c-5 lesson, restated: evidence per commit, from
what was observed, never from intention.

| Gate item | Result |
|---|---|
| `gofmt -l .` | **PASS** — exit 0, no output |
| `go build ./...` | **PASS** — exit 0, clean |
| `go vet ./...` | **PASS** — exit 0, clean |
| `go test ./... -count=1` (whole tree, foreground, awaited) | **PASS** — exit 0; 27 test packages `ok` + 1 with no test files; 4 min 03 s (04:56:45Z → 05:00:48Z) |
| `go test -race ./core/... -count=1` (backgrounded FIRST, **exit status collected**) | **PASS** — **exit 0**, 13/13 packages `ok`, 4 min 05 s (04:48:17Z → 04:52:22Z) |
| `go test ./internal/guards/ -v -count=1` | **PASS** — `ok … 0.488s`, 9/9 `--- PASS` by name (below) |
| frontend `npm run check` | **PASS** — exit 0, `COMPLETED 189 FILES 0 ERRORS 0 WARNINGS 0 FILES_WITH_PROBLEMS` |
| frontend `npm test` | **PASS** — exit 0, `Test Files 16 passed (16)` / `Tests 401 passed (401)` |
| frontend `npm run build` | **PASS** — exit 0, `✓ built in 476ms` |
| `go generate` ×2, all FIVE generators | **PASS** — idempotent and non-vacuous, below |
| `wails generate module` ×2 (from `app/`) | **PASS** — idempotent and non-vacuous, below |
| TEN-path golden gate | **PASS** — exit 0, empty, as ONE invocation and as TEN, before AND after regeneration |
| `core/cat/testdata/` commit count | **PASS** — still exactly two (`ff5c19b`, `1d38941`) |
| `git status --short` | **PASS** — clean, before and after every item |

`/opt/homebrew/bin/npm` (11.19.0) was used for all three frontend items,
run from `app/frontend`.

### The full Go suite, package by package

`go test ./... -count=1`, **exit 0**. Twenty-seven test packages, every
one `ok`, plus one with no test files:

```
ok  	github.com/gm5dna/open-rig-programmer/app	129.894s
ok  	github.com/gm5dna/open-rig-programmer/cmd/rigprog	161.402s
ok  	github.com/gm5dna/open-rig-programmer/core/cat	0.589s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/dialecttest	1.055s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx10	1.295s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx101	1.534s
ok  	github.com/gm5dna/open-rig-programmer/core/clone	242.490s
ok  	github.com/gm5dna/open-rig-programmer/core/codeplug	2.320s
ok  	github.com/gm5dna/open-rig-programmer/core/csvio	2.233s
ok  	github.com/gm5dna/open-rig-programmer/core/driver	2.456s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ft710	58.191s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ftdx10	68.952s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ftdx101	99.123s
ok  	github.com/gm5dna/open-rig-programmer/core/spec	2.422s
ok  	github.com/gm5dna/open-rig-programmer/core/transport	18.326s
ok  	github.com/gm5dna/open-rig-programmer/internal/buildinfo	1.944s
ok  	github.com/gm5dna/open-rig-programmer/internal/csvmerge	2.094s
ok  	github.com/gm5dna/open-rig-programmer/internal/extable	2.188s
?   	github.com/gm5dna/open-rig-programmer/internal/extable/gen	[no test files]
ok  	github.com/gm5dna/open-rig-programmer/internal/extable/observe	1.204s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakedx10	3.149s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakedx10/gen	0.972s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakedx101	3.527s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakedx101/gen	0.960s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakeradio	6.566s
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.635s
ok  	github.com/gm5dna/open-rig-programmer/internal/radiotext	0.279s
ok  	github.com/gm5dna/open-rig-programmer/internal/wiring	33.245s
```

**Twenty-seven, against M9d-1's twenty-four. None dropped; three added,
and they are exactly this milestone's three new packages** —
`core/driver/ftdx101` (tasks 2-4), `internal/fakedx101` (task 5) and
`internal/fakedx101/gen` (task 6). The arithmetic closes with no
unexplained package on either side.

`internal/wiring` at **33.245 s**, against M9d-1's 10.895 s, is the cost
of the E5-class walk now opening **FOUR** models rather than two: the
budgeted full-range 5xx/EMG discovery runs per registered model, roughly
2.8 s each, several times over across that package's tests. It is
flagged, as Task 7 flagged it, and not optimised — nobody should trim a
discovery budget to make a test suite faster.

### The 9 guards, by name

All `--- PASS`, from `go test ./internal/guards/ -v -count=1`:

```
--- PASS: TestCompositionRootImportDiscipline (0.04s)
--- PASS: TestDialectPromotedDataIsNotAPackageGlobal (0.02s)
--- PASS: TestGateReachingValidatorsAreDialectMethods (0.02s)
--- PASS: TestTransitiveGlobalReachSetIsReported (0.02s)
--- PASS: TestDriverSeamPackageDoesNotImportCAT (0.02s)
--- PASS: TestNewEngineReachableOnlyFromDriver (0.02s)
--- PASS: TestWritePathReachableOnlyThroughDriver (0.02s)
--- PASS: TestRetiredWriteResultNamesAreGone (0.02s)
--- PASS: TestSimulatedProfileTokensConfinement (0.02s)
PASS
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.488s
```

**Nine, unchanged in number from M9c-5, M9c-6 and M9d-1.** M9d-2 added
TWO ROWS to `TestSimulatedProfileTokensConfinement` — not a tenth test —
so both FTdx101 siblings are confined by exactly the check that confines
the FT-710's and the FTdx10's. Part 6's proofs 9.5-9.7 are what show the
two rows are not one row written twice.

### `go test -race ./core/...`

Started FIRST, in the background, before any other work in this task
(`04:48:17Z`), and **its exit status was collected** (`04:52:22Z`) —
4 min 05 s, exit **0**. The background allowance exists only for the
foreground time limit; an uncollected race run would be no gate, so the
status was written to a file by the job itself and read back before this
manifest was written and before the commit. Every package:

```
ok  	github.com/gm5dna/open-rig-programmer/core/cat	1.857s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/dialecttest	1.859s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx10	1.429s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx101	2.110s
ok  	github.com/gm5dna/open-rig-programmer/core/clone	243.722s
ok  	github.com/gm5dna/open-rig-programmer/core/codeplug	4.580s
ok  	github.com/gm5dna/open-rig-programmer/core/csvio	2.320s
ok  	github.com/gm5dna/open-rig-programmer/core/driver	2.780s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ft710	57.765s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ftdx10	70.079s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ftdx101	99.873s
ok  	github.com/gm5dna/open-rig-programmer/core/spec	2.680s
ok  	github.com/gm5dna/open-rig-programmer/core/transport	18.481s
```

**Thirteen core packages, race detector on, zero reports** — twelve as in
M9d-1, plus `core/driver/ftdx101`.

### Regeneration idempotence, ×2, non-vacuously — all FIVE generators

`go generate ./internal/fakedx10/... ./internal/fakedx101/... ./core/cat/...`
run twice. That command reaches all five `//go:generate` directives in
the repository: `core/cat` (ft710), `core/cat/ftdx10`,
`core/cat/ftdx101`, `internal/fakedx10` and `internal/fakedx101`. Exit 0
each time; `git status --porcelain` **EMPTY after both passes**; and all
five generated inventories unchanged at every step:

| Generated file | SHA-256 (before, after pass 1, after pass 2) |
|---|---|
| `core/cat/exinventory_gen.go` | `fbf4f02e3e564357eecd020f612de58cd3d26ad58a8a6e8ee9cb1249815ada22` |
| `core/cat/ftdx10/exinventory_gen.go` | `9311fc928b540110539d5dc40c921193b39890fbd6ef8c6f27c1e0db3c2171d4` |
| `core/cat/ftdx101/exinventory_gen.go` | `f2f0fdf2fce2e92eba7aa8ecfb2cea9134b67b382cee52fe01fc558fd736800d` |
| `internal/fakedx10/exinventory_gen.go` | `0d44f04bef5ece957fc324c8350cef9af5a9d6899b83639a2ad3bdff803dad48` |
| `internal/fakedx101/exinventory_gen.go` | `2f6dfaa23245febeb351b9e9a3c9046780d497f3013fa6d1f431e1955e1d5e51` |

Three of those five reproduce M9c-6's recorded Part 5 values (counted in
the 95 above). The `internal/fakedx101` value is the one Task 6's
pre-proof hash recorded, holding across three further commits.

**Non-vacuity measured by mtime**, because a generator that silently did
nothing would also produce a clean diff: all five files carry mtime
`10:07:09`, written by pass 2 seconds before the check. They really were
regenerated and really were byte-identical.

`wails generate module` (`~/go/bin/wails`) run twice from `app/`, exit 0
each time. **Non-vacuity measured the same way**: the bindings were last
written at **2026-07-30 04:36:23** (M9c-6's own task 9 run — untouched by
the whole of M9d-1 and M9d-2), and the two passes rewrote them at
**2026-08-09 10:07:32** and **10:07:41**. After each pass, from the REPO
ROOT:

```
$ git diff --exit-code -- app/frontend/wailsjs   → exit 0
$ git status --porcelain                          → empty
```

**Six tracked binding files, rewritten twice, byte-identical both
times.** Registering two models changed no Wails binding, which is the
expected consequence of `GetSupportedModels` returning a `[]string` whose
CONTENTS grew rather than a new method or type.

### The golden corpora — all TEN paths

Untouched, every one. Run as a single invocation and then again path by
path, both exit 0 with empty output, **before and after** the
regeneration passes:

```bash
git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go \
  core/cat/ftdx10/testdata/ core/cat/ftdx10/exinventory_gen.go \
  internal/fakedx10/exinventory_gen.go internal/fakedx10/transcription-b.csv \
  core/cat/ftdx101/testdata/ core/cat/ftdx101/exinventory_gen.go \
  internal/fakedx101/transcription-b.csv internal/fakedx101/exinventory_gen.go
```

| Path | Result |
|---|---|
| `core/cat/testdata/` | **PASS** — exit 0 |
| `core/cat/exinventory_gen.go` | **PASS** — exit 0 |
| `core/cat/ftdx10/testdata/` | **PASS** — exit 0 |
| `core/cat/ftdx10/exinventory_gen.go` | **PASS** — exit 0 |
| `internal/fakedx10/exinventory_gen.go` | **PASS** — exit 0 |
| `internal/fakedx10/transcription-b.csv` | **PASS** — exit 0 |
| `core/cat/ftdx101/testdata/` | **PASS** — exit 0 |
| `core/cat/ftdx101/exinventory_gen.go` | **PASS** — exit 0 |
| `internal/fakedx101/transcription-b.csv` | **PASS** — exit 0 |
| `internal/fakedx101/exinventory_gen.go` | **PASS** — exit 0 |
| **all ten in one invocation** | **PASS** — exit 0, empty |

**Eight paths at M9d-1, ten now** — Task 6 added the fakedx101 pair, and
that is the intended growth. **No golden was regenerated at any point in
M9d-2.** `core/cat/testdata/` is still at exactly two commits:

```
1d38941 m9b: task 51 fix round — close blind spots found by mutation review
ff5c19b m9b: task 51 — mint three evidence corpora before anything moves
```

---

## Part 8 — the ledger: five notes this milestone must carry forward

Recorded here, in the durable tracked document, because each is a real
limit or a real correction that a working file would lose.

### Note 1 — the four-radio CHIRP fixture table is HAND-WRITTEN

`core/csvio/chirp_test.go`'s `registeredRadioCapabilities()` (:829-836)
returns one fixture per registered model:

```go
return []spec.Capabilities{
	ft710LikeCapabilities(),
	ftdx10LikeCapabilities(),
	ftdx101LikeCapabilities("FTdx101D", "0681"),
	ftdx101LikeCapabilities("FTdx101MP", "0682"),
}
```

It is a table of FIXTURES rather than a walk of the registry for a
layering reason that stands: `core/csvio` must not import `core/driver`,
and `internal/wiring` imports every driver. **A fifth registration must
add a row here, and nothing in this package will notice if it does
not.** The registry-walk pin that WOULD notice lives in wiring —
`TestSupportedModels_ContainsEveryRegisteredModel`, red-proved at 9.1 —
and drift between the fixtures and the real caps is caught end to end by
the CLI byte-identity baseline in this document, which does use the real
drivers. The gap is named in the function's own doc comment (Task 8 fix
round 1, MINOR 3); it is repeated here because a manifest is where the
next registrant will look.

### Note 2 — data compatibility: no migration is owed, and none is performed

Codeplugs saved under the OLD blank-`Skip` construction carry
`scan_skip {Known, false}` on their CHIRP-derived channels. The new
build does not rewrite them and does not reclassify them; it reads them
faithfully, which means **an old saved codeplug still meets the write
gate that a freshly imported one no longer meets.** Measured directly,
head binary, same fixture, two files:

```
$ rigprog diff --fake --model FTdx10 <codeplug saved under the OLD construction>
Added:
  M-02: … tag "MINIMALLSB"
    BLOCKED: scan_skip not writable on this radio
  …
Added 2, Modified 1, Erased 0, Blocked 3, Unchanged 114

$ rigprog diff --fake --model FTdx10 <the same CHIRP file imported under the NEW construction>
Added:
  M-02: … tag "MINIMALLSB"
  …
Added 2, Modified 1, Erased 0, Blocked 0, Unchanged 114
```

**No migration is owed**, for a reason that is a fact about the radios
rather than a convenience: **no registered radio can write `scan_skip`
at all** — the FT-710's 28-byte MR/MW layout has no scan-skip position,
nor has the FTdx10's, nor the FTdx101's. A stored Known-false therefore
cannot have come from the radio and cannot be sent to it; the only harm
it does is the block above, and the user's route out is to re-import the
CHIRP file under the new build. Against a fresh import the old file's
three channels also compare as differing in that field, so a
same-source comparison will show them as Modified; that is the same fact
seen from the other side, not a second one.

### Note 3 — the frontend `dist/` is NOT tracked

`app/.gitignore:3` ignores `frontend/dist`, and `git ls-files
app/frontend/dist` returns nothing. `npm run build` therefore leaves
`git status --short` clean, and a clean status after a frontend build is
**not** evidence that the built output is unchanged. No claim in this
manifest is premised on `dist/`; the frontend is covered by
`npm run check`, `npm test` and the fact that the build exits 0.

### Note 4 — `writeTrialsComplete=false` on both siblings

Unchanged by this milestone and deliberately so. Neither FTdx101 can be
written to over a real port; the RealHardware profile reports Unverified
write support and the capability gate refuses before a frame is built.
Registration makes the models SELECTABLE and READABLE, not writable.
Nothing in Part 5 touched a real radio.

### Note 5 — `4a42e07`'s commit message carries a claim later found false

Task 5's commit message (`4a42e07`) states:

> *"Two evidence-led divergences from internal/fakedx10, both from this
> manual's own text: command names are accepted in EITHER CASE (layout
> 204-205 says so; **the FTdx10's manual does not**)…"*

The parenthetical comparison is **FALSE**. `64519d9` ("M9d-2 task 5 fix
round 1: the FTdx10-manual comparison was false") found both sentences
present in the FTdx10's own extraction (`ftdx10_layout.txt:161` and
`:317`), in the same words, and dropped every such comparison from the
code and comments; `91d3c6d` finished the sweep. **The FTdx101 readings
themselves were verified against the extraction and stand — only the
comparisons were false.**

Git commit messages are immutable, so the corrected code and the
uncorrected message will disagree for as long as the history exists.
This note is the ledger entry that says so, and it is the reason a
reader should take `64519d9`'s and `91d3c6d`'s bodies as authoritative
over `4a42e07`'s wherever the two touch the FTdx10's manual.

### The digit sweep (review fold C1)

Prose carrying a registered-model COUNT was swept at this task, since a
count is the thing a registration falsifies. **Result: every
count-bearing statement in the tree is DATED**, which is the discipline
working — a dated claim becomes historical rather than wrong:

| Site | Text | Verdict |
|---|---|---|
| `app/app.go:319` | "internal/wiring registers FOUR models since M9d-2" | dated — stays true |
| `app/app_test.go:266` | "internal/wiring registers four models since M9d-2" | dated — stays true |
| `app/connection.go:82` | "M9d-2 made it four models" | dated — stays true |
| `internal/wiring/wiring.go:327` | "FOUR registered values agree with transport's today" | carries "today" |
| `internal/wiring/wiring_test.go:918` | "FOUR since M9d-2 task 7" | dated |
| `internal/wiring/wiring_test.go:1011` | "THREE OF THE FOUR REGISTERED MODELS…" | dated to M9d-2 |
| `internal/radiotext/radiotext.go:387` | "the four models internal/wiring registers" | **UNDATED — a fifth registration falsifies this line.** Flagged, not changed: it is accurate today, and the pin that enforces it (`TestEverySupportedModelHasRadiotext`, red-proved at 9.4) fires on the code rather than the comment. |

`cmd/rigprog/usage.go`'s doc comment is the worked example of the fix:
Task 7 rewrote it to name no model and no count, recording that an
earlier version *"named the list's contents (\"today just \\\"FT-710\\\"\")
and was falsified twice over, at M9c-6 and again at M9d-2, while the
CODE it describes never needed touching."* Every hit above that says
"two radios" refers to the FTdx101 sibling PAIR — a fact about those two
radios, count-stable under any further registration — and was checked
individually rather than pattern-matched away.

---

## The gate-at-final-code-tip invariant

Re-stated, because it is the invariant a merge reviewer should check
rather than any particular hash:

> The byte-identity capture is taken at the LAST CODE COMMIT of the
> branch. Every commit after the capture must be documentation-only for
> the capture to speak for the branch tip.

**The last CODE commit on `m9d2-ftdx101-registration` is `1564d31`**
(task 8 fix round 1). Every measurement in this manifest was taken at
that commit, in that working tree, with a binary built from it.

**This manifest's own commit is DOCUMENTATION-ONLY** in the invariant's
sense. It tracks exactly one path:

| Path | Kind |
|---|---|
| `docs/superpowers/m9d2-baseline-manifest.md` | this manifest |

No `.go` file, no frontend file, no generated binding and no golden is
touched by it — `git show --stat` of this commit is the whole of its
contents, and it is the check to run rather than to take on trust. The
capture therefore speaks for the branch tip **as of this commit**.

If a milestone-review wave lands after this one, this manifest does
**not** retroactively cover it. That is not hypothetical: M9c-5's wave
`bc3b6f1`/`8721a91` and M9c-6's wave `8baab59` both landed after their
manifests' captures and were covered only by their own gate runs — until
a later milestone's span happened to include them. **Any M9d-2 review
wave must do the same: re-run, re-record, per commit.**

The planning and handoff documents are deliberately not in this commit:
`.superpowers/` is gitignored, as is `docs/fixtures-private/`, so the
task briefs and reports are on-disk working files no commit contains.
Only tracked files can affect whether a post-capture commit is
documentation-only.

---

## Scope of the claim this manifest supports

It supports: **across the whole of M9d-2, and across the whole span from
M9c-6's capture at `490c38c` to `1564d31`, every recorded value in the
M9c-6 baseline manifest is reproduced exactly EXCEPT thirteen rows in two
classes fixed in writing before this gate ran — the registry-driven model
list gaining its two new names, and Task 8's caps-aware CHIRP blank-`Skip`
construction — each of the thirteen recorded verbatim, old value and new,
and each proved by REVERTING the commit that caused it and watching
M9c-6's recorded value come back; no row moved outside its class; the
FT-710's read, export and native-CSV round-trip paths did not move at
all; and the FTdx101D and FTdx101MP are registered, selectable models
whose probe, read, settings read, diff, settings render, export, CHIRP
import (clean and skip-carrying) and native CSV round trip all work end
to end against `internal/fakedx101`, at exit 0, with 117 channels, 193
settings, `ftdx101-ex@1`, `Blocked 0`, and their first baselines
recorded — the two siblings differing, end to end through the CLI, in
their model name and their CAT ID and in nothing else.**

It does **not** support a claim about: the write path to a real radio of
any model (no radio is opened anywhere in this recipe;
`writeTrialsComplete=false` is pinned for both FTdx101 siblings and the
RealHardware profile refuses before a frame is built); **the FTdx101's
correctness against real hardware in any respect** — the dialect, the
driver and the fake are all manual-derived and fixture-exercised only, no
FTdx101 has ever been asked anything, and every FTdx101 value in Part 5
is the FAKE's answer with EX values invented by a documented convention;
the FTdx10's or the FT-710's correctness against real hardware; the
`ports`, `write` or real-`--port` paths; the GUI at runtime (the frontend
is typechecked, unit-tested and built, not driven, and its `dist/` is not
tracked); or the base-versus-head `cmp` verdicts M9c-6 recorded for
streams it published no hash for — those required its two-binary setup,
and rows carrying no recorded value are compared on nothing and claimed
for nothing.

# M9d-1 baseline note

Captured **06/08/2026**, as Task 8's full local gate and the FULL
byte-identity re-run for the M9d-1 FTdx101 dialect milestone
(`core/cat/ftdx101` — the FTdx101D/MP's dialect, its PDF-derived Table 2
inventory and the two model instances, plus one `internal/extable`
profile row).

M9d-1 adds a **third radio's** dialect package alongside the FT-710's
and the FTdx10's. It registers **no new model**: no CLI path can select
an FTdx101, and `core/cat/ftdx101` is not in the CLI's link graph at
all. The milestone bar is therefore the hard one M9c-4 and M9c-6 set —
**not one byte of the FT-710's OR the FTdx10's observable behaviour
moves, and this time not even the sanctioned model-list line moves,**
because nothing is registered for it to grow by.

The standing rule, inherited from the M9c-1/M9c-3/M9c-4/M9c-6
manifests, applies unchanged: **a difference that is not a
declared/sanctioned field is a defect, never a baseline to update.**
This gate found no such difference. **Ninety-two recorded values
compared from the manifest's Part 1/2/3 recipe tables — 52 full SHA-256
hashes and 40 further recorded values (exit codes, verbatim literals,
line counts, one recorded hash prefix and one `cmp` equality) —
ninety-two matches, zero mismatches, zero carve-outs invoked, and no
sanctioned delta claimed or needed.** Three further hashes from the
manifest's Part 5 regeneration table also match, for **95 of 95**.

One process fact belongs in this summary rather than in a footnote: the
mechanical comparator's FIRST pass reported 52/52 mismatch. That was a
zsh `path`/`PATH` fault in the comparison harness which meant the 52
comparisons never executed at all; it is diagnosed, fixed and its
re-run transcript quoted under "Reproduction" below. No artefact, no
recorded value and no recipe was altered in response to it.

- **HEAD under test:** `fef25cbb4955f7b7c7b7e7b831cb487908f88f69`
  ("M9d-1 task 7b: the golden byte-compare tests and the D-vs-MP frame
  identity"), the eleventh and **last CODE commit** on the branch.
- **Branch:** `m9d1-ftdx101-dialect`, forked from
  `bba69e256e3e246b64f559e7876a24f2acccd50f` ("M9d-1 plan rev 2:
  fourteen review folds …"), confirmed by
  `git merge-base main m9d1-ftdx101-dialect`.
- **Comparison target:** every value recorded in
  `docs/superpowers/m9c6-baseline-manifest.md`, compared **by that
  manifest's own tables** — hash-compared only where it records a hash,
  literal-compared where it records a literal.
- **Toolchain:** `go1.26.5 darwin/arm64`
- **OS:** macOS 26.5.1 (build 25F80), arm64
- **Artefacts** were captured to an ephemeral scratch directory (not
  preserved beyond this run); this note is the tracked, durable record,
  per the M9b review finding that artefacts alone have no provenance.
- **Worktrees:** none created and none outstanding; the repository
  working tree was never written to by the recipe (see "Reproduction").

---

## Scope of the check, and why no fresh base worktree was built

M9c-6's gate compared **two builds** — one from its base commit
(`78b73ac`) in a throwaway worktree, one from its head (`490c38c`) —
because the question there was whether registering the FTdx10 had moved
the FT-710. Both builds were run and the resulting hashes recorded, and
the manifest is the durable, version-controlled record of them.

This gate asks a different question, and so needs only one build — the
M9c-4 note's method, restated. The comparison target is **the M9c-6
manifest's recorded values themselves**, which are the settled,
published numbers for both radios' offline paths. `490c38c` (M9c-6's
head under test) is a verified ancestor of `fef25cb`
(`git merge-base --is-ancestor` returns true), so a single build at
M9d-1's head, compared against those recorded numbers, closes the chain
end to end:

> `3b75fcc` ≡ `76f3f77` (M9c-3's two-worktree gate) ≡ `699f7c9`
> (M9c-4's note) ≡ `6b84335` → `f64f688` (M9c-5, four carve-outs) ≡
> `78b73ac` ≡ `490c38c` (M9c-6's gate, no carve-outs) ≡ **`fef25cb`
> (this note, no carve-outs)**.

Rebuilding `490c38c` in a fresh worktree would only re-derive numbers
this repository already holds under version control, and would prove
nothing the recorded values do not already assert. The recorded values
are the baseline; a build that reproduces them is the evidence.

### What the branch actually touches

The a-priori expectation was that every artefact would be **trivially
identical**, because M9d-1 touches no path either radio reaches. The
branch diff (`bba69e2..fef25cb`, 22 files, +5,742/−15) confirms it by
inspection — every changed file is either under `core/cat/ftdx101/` or
is `internal/extable/profile.go` (one new `ftdx101Profile` var and one
new `registry` row; the two existing rows move only by gofmt
realignment) and its test:

```
core/cat/ftdx101/…            20 files (new package: dialect, tests, testdata)
internal/extable/profile.go      46 insertions, 2 deletions
internal/extable/profile_test.go 91 insertions, 13 deletions
```

Those are `git diff --numstat bba69e2..fef25cb` figures. `--stat`'s
combined per-file count for `profile.go` is 48, which is insertions plus
deletions and not a pair of numbers; numstat's 46/2 is the honest split.

Two structural corroborations, both measured rather than argued —
and both **corroboration only**; the evidence is the hashes below:

- `go list -deps ./cmd/rigprog | grep core/cat` returns exactly
  `core/cat` and `core/cat/ftdx10`. **`core/cat/ftdx101` is not in the
  CLI's dependency graph.**
- `go list -deps ./cmd/rigprog | grep -c internal/extable` returns
  **0**. The one non-`ftdx101` file the branch touches is not linked
  into the CLI at all; `internal/extable` is build tooling for the
  generators.

**Expectation is not evidence, though, which is why the whole recipe
was rerun rather than argued from the diff.**

### A wider span than the branch, and one result that is new

The re-run's zero-diff outcome is measured at `fef25cb` against numbers
recorded at `490c38c`, so it speaks for the **entire span between
them** — 18 commits, not just M9d-1's eleven. Three of those commits'
non-documentation files lie outside M9d-1's own two directories, and
all three belong to the **M9c-6 milestone review fix wave `8baab59`**:
`internal/radiotext/radiotext.go`, `internal/radiotext/radiotext_test.go`
and `internal/wiring/fake.go`.

The M9c-6 manifest was explicit that it did **not** retroactively cover
a review wave landing after its capture, and required any such wave to
be re-gated per commit. That wave had its own gate run recorded in its
commit body. This run adds an independent measurement on top: it shows
that the wave changed **no CLI-observable output either**, on any leg of
either radio's recipe.

The wave's one changed user-visible string is the FTdx10's
`ToneScanSkipNote`. That it does not appear in the recipe is measured,
not assumed: its only consumer is `app/uispec.go` (the GUI channel
grid's standing legend), and `grep -rl "Tone and Scan Skip"` over
**every** captured artefact of this run — all three mirror trees —
returns nothing.

---

## Reproduction

One binary was compiled from source (never `go run`, which collapses
exit codes — the standing artefact from the M9a gate notes):

```bash
go build -o <scratch>/bin/rigprog-head ./cmd/rigprog   # from the repo root at fef25cb
```

The M9c-6 recipe runs its binary **from a tree root**, writing to the
relative path `.capture/`, so that every path string the CLI echoes is
byte-comparable. `.capture/` is not covered by `.gitignore`, so — as in
M9c-4, M9c-5 and M9c-6 — this run used minimal **mirror trees** holding
only `.capture/` and the inputs each recipe reads, at their identical
repo-relative paths, and the repository was left untouched
(`git status --short` empty before and after; verified):

```
<mirror-ft710>/.capture/
<mirror-ft710>/core/csvio/testdata/chirp_sample.csv
<mirror-ft710>/docs/superpowers/m9c5-fixtures/chirp_minimal.csv

<mirror-dx10>/.capture/
<mirror-dx10>/docs/superpowers/m9c5-fixtures/chirp_minimal.csv
<mirror-dx10>/docs/superpowers/m9c6-fixtures/chirp_skip.csv

<mirror-usage>/                       # Part 2 echoes no paths at all
```

Every path string passed to the CLI — and hence every path string it
echoes — is therefore character-for-character what M9c-6 used. That the
echoed paths did reproduce is not assumed: `read.stdout` and
`a2-read.stdout`, whose `Output:` summary lines contain
`.capture/read-fake.json` and `.capture/ftdx10-read.json`, both match
their recorded hashes exactly.

The three recipe inputs are byte-identical to the ones M9c-6 fed in,
compared against the manifest's own recorded values:

| Input | Recorded SHA-256 | Result |
|---|---|---|
| `core/csvio/testdata/chirp_sample.csv` | `ee3f2664242bdd2292f7afc49d46d53338f3afa227cccfd57b24855e4216c7be` | **MATCH** |
| `docs/superpowers/m9c5-fixtures/chirp_minimal.csv` | `87c8a9b1ac12c188a8eb726cbe1757ac01e8253ccb1741f6fe9d1af12dc02ddd` | **MATCH** |
| `docs/superpowers/m9c6-fixtures/chirp_skip.csv` | `f5f23977d1ef67ee23943324e9796d508bbec12efdedbd0a1bac44b78e07b6c5` | **MATCH** |

The Part 1 recipe was run **verbatim**, exactly as the manifest
transcribes it, including its two supplementary legs S1 and S2. Exit-code
files are hashed **without** a trailing newline (`printf`, not
`echo $?`), matching the M9c-1/M9c-3/M9c-4/M9c-5/M9c-6 convention so the
recorded hashes are directly comparable across milestones. `set -e` is
omitted: the historical import leg exits 3 by design and would abort the
script. That leg wrote **no** output file (`ls import-out.json` → no
such file), as the manifest records; the `--out` flag is kept so the
invocation stays character-identical.

### Method notes — how two under-specified rows were reproduced

Two recipe rows are recorded by the manifest without a fully literal
command, and both were reproduced so that the recorded value is the one
being compared:

- **S2's intermediate file names.** The manifest gives S2's `awk` edit
  and its two hashes but not the scratch file names. Neither name can
  reach a recorded artefact: `import` echoes only its `--out` path (and
  S2's import stdout carries **no** recorded hash, "—"), and `diff`
  echoes no path at all. The names chosen here (`export-skip.csv`,
  `csvskip.json`) are therefore free variables, and the two rows that
  ARE recorded — the edited CSV and `csvskip-diff.stdout` — both match.
- **A7's `--into` source.** The manifest names A6a's as a copy of the
  FTdx10 read (`ftdx10-into-chirp.json`) but leaves A7's implicit. A
  byte copy of `ftdx10-read.json` was used, by symmetry. That the choice
  was right is confirmed by the outcome rather than asserted:
  `ftdx10-import-skip.json` reproduces its recorded normalised hash
  `db120d70…` exactly, which it could not do if the merge source had
  differed.

### The first comparator pass reported 52/52 MISMATCH — and why that was the harness

Recorded here, in the durable document rather than only in a working
file, because **a mismatch verdict is a STOP condition under this
milestone's own rule**, and it matters that this one was proved spurious
rather than assumed to be.

**What happened.** The first run of the mechanical comparator printed a
`MISMATCH` line for every one of the 52 rows and this total:

```
hash rows compared : 52
MATCH              : 0
MISMATCH           : 52
MISSING            : 0
```

**Why it was not a data difference.** Every `MISMATCH` line carried an
EMPTY `got=` value, and each was preceded by two errors from the shell:

```
(eval):12: command not found: shasum
(eval):12: command not found: cut
MISMATCH P1-in-01 core/csvio/testdata/chirp_sample.csv  want=ee3f2664… got=
```

The comparator loop assigned each artefact's location to a shell
variable it had named `path`. **In zsh, `path` is tied to `PATH`** (it
is the array view of that parameter), so the first assignment destroyed
the search path and every external command in the loop body — `shasum`
and `cut` included — became unresolvable. The 52 comparisons were
therefore **never executed**: the loop compared a recorded hash against
the empty string, 52 times. This is a diagnosis from the shell's own
error output, not an inference from the verdict.

**The fix, and the actual re-run.** The variable was renamed (`path` →
`fp`) and nothing else was changed — not the recorded-value table, not
the artefacts, not the recipe. The comparator's output on re-run, quoted
as it prints (a clean run emits no per-row line, so the summary is the
whole of it):

```
=== comparator re-run, verbatim ===
-----------------------------------------
hash rows compared : 52
MATCH              : 52
MISMATCH           : 0
MISSING            : 0
```

The distinction the STOP rule turns on is exactly this one: a harness
that cannot run a comparison is not a comparison that failed. Had any
`got=` carried a real hash differing from a recorded one, this note
would report BLOCKED and no commit would have been made.

The one declared noise field is normalised by the recipe's own `sed`,
and that the normalisation changed **only** the timestamp is confirmed
by `diff`, not inferred from the normalised hash matching:

```
$ diff read-fake.json read-fake.normalised.json
7c7
<     "read_at": "2026-08-06T00:59:11.55498+01:00",
---
>     "read_at": "NORMALISED",
```

— one line, line 7, and identically one line for `import-min.json`. The
raw un-normalised hashes are recorded for completeness and are **not** a
comparison any run can pass (the manifest says so itself): `read-fake.json`
`c3e0c2580c542c173ddf813e048f5387b71c42df875de6a5c54e2e61f1fdb797`,
`import-min.json`
`951068e6c33d308998d28326827e1273b365f5bb04913aa34b4663015ab9604d`.

---

## Part 1 — the FT-710 tables: 18 + 3 + 2 rows, all MATCH

The manifest's own eighteen, in its order, so the correspondence is
auditable line by line. Every row is compared on **exactly** what the
manifest records for it.

| # | Artefact | M9c-6 recorded SHA-256 | This run | Result |
|---|---|---|---|---|
| 1 | `probe.stdout` | `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` | identical | **MATCH** |
| 2 | `probe.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | identical | **MATCH** |
| 3 | `probe.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** |
| 4 | `read.stdout` | `a9cc9fc3834159660fdf7357c3ef3e9ad8864f36b1935bdb21e8c4f1fa252448` | identical | **MATCH** |
| 5 | `read.stderr` | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | identical | **MATCH** |
| 6 | `read.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** |
| 7 | `import.stdout` (historical, blocked) | `fa5ee2aaf512e88fbab094993ef8dcb2ea7bd3a757749c091a16f534b65a77b7` | identical | **MATCH** |
| 8 | `import.stderr` (historical, blocked) | `094856b06f952559216319da39a46c4ce23f06e8e9bcc09ed355d5a523319c7a` | identical | **MATCH** |
| 9 | `import.exit` ("3") | `4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce` | identical | **MATCH** |
| 10 | `export.stdout` | `fffb2b0a3bd10756a6f5f0c655e4e4278a4a379fbb390dddaf4b2b1c59656af0` | identical | **MATCH** |
| 11 | `export.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | identical | **MATCH** |
| 12 | `export.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** |
| 13 | `export.csv` | `56643e7f520ded75de2957175e9b96d61d35f5205c9866cbd54d31cad78ab7fb` | identical | **MATCH** |
| 14 | `importmin.stdout` (clean import) | `8040f49b3cbb56293676d7cbbe7fb5cdbf232ffb96728c3ec4dc0add235a62d0` | identical | **MATCH** |
| 15 | `importmin.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | identical | **MATCH** |
| 16 | `importmin.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | identical | **MATCH** |
| 17 | `read-fake.json` (`read_at`-normalised) | `3faad8a7a46a14c3d624fa88c42f16065cf2c1cee756e234a8836dd9c3bf6653` | identical | **MATCH** |
| 18 | `import-min.json` (`read_at`-normalised) | `03bd53ae4a1a6b7a6f1ff8df41a5aa27eadb4a4fad0e988730f243af754db4a8` | identical | **MATCH** |

**S1 — `diff --fake` over the clean CHIRP import.** All three recorded
hashes:

| Artefact | Recorded SHA-256 | Result |
|---|---|---|
| `diffmin.stdout` | `099227a776e130295a342ccce8567b0550e77d3c2326138972f1db08fabe6f8d` | **MATCH** |
| `diffmin.stderr` | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | **MATCH** |
| `diffmin.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** |

**S2 — the native-CSV round trip carrying a Known `scan_skip`.** The
manifest records two hashes here and, for the remaining streams, the
literal exit value `"0"` and a `cmp` verdict against a base build this
run does not have. Only the recorded hashes and the recorded exit
literals are compared; nothing else is claimed:

| Artefact | Recorded | Result |
|---|---|---|
| the `awk`-edited CSV | `0cac78ae84eb226ad5f2d600493912d0bd4542ccccdfd75668cd71db5eeb46cb` | **MATCH** (hash) |
| `csvskip-diff.stdout` | `7521da5c15a5496ccaa86314dc385c747fc73427e8d332009c04f99b2f7e774d` | **MATCH** (hash) |
| `csvskip-import` .stdout/.stderr/.exit | exit `"0"` recorded; no hash | **MATCH** (exit literal); no hash claimed |
| `csvskip-diff` .stderr/.exit | exit `"0"` recorded; no hash | **MATCH** (exit literal); no hash claimed |

That the edited CSV reproduces `0cac78ae…` is a consequence worth
naming: it can only do so because `export.csv` reproduced row 13 first.

The verdict block S2 exists to show is reproduced word for word:

```
Modified:
  M-01: freq 7000000→7000000 Hz, mode LSB→LSB, tag ""→""
    BLOCKED: scan_skip not writable on this radio

Added 0, Modified 1, Erased 0, Blocked 1, Unchanged 116
```

**Part 1 totals: 23 recorded hashes compared, 23 MATCH; 8 exit literals
compared, 8 MATCH. MISMATCHES: none. Carve-outs invoked: none.**

---

## Part 2 — the model-list surfaces: IN SCOPE, and unmoved

M9c-6's Part 2 recorded the ONE designed delta of that milestone: a
registry-driven model list, interpolated at print time by
`printUsage` from `strings.Join(wiring.SupportedModels(), ", ")`, grew
from `FT-710` to `FT-710, FTdx10`.

**M9d-1 registers no model at all**, so the prediction is that this
surface — the only one M9c-6 was permitted to move — does not move
either. It does not. The AFTER literal the manifest recorded is
reproduced character for character on all three surfaces:

```
rigprog is a command-line memory programmer for Yaesu radios (currently: FT-710, FTdx10).
```

Every value the manifest's Part 2 table records for its **head**
column, compared:

| Surface | Stream | Recorded (head) | This run | Result |
|---|---|---|---|---|
| bare `rigprog` | stderr | 18 lines, exit 2 | 18 lines, exit 2 | **MATCH** |
| bare `rigprog` | stdout | 0 (empty) | 0 | **MATCH** |
| `rigprog help` | stdout | 18 lines, exit 0 | 18 lines, exit 0 | **MATCH** |
| `rigprog help` | stderr | 0 (empty) | 0 | **MATCH** |
| `rigprog nosuchcmd` | stderr | 20 lines, exit 2 | 20 lines, exit 2 | **MATCH** |
| `rigprog nosuchcmd` | stdout | 0 (empty) | 0 | **MATCH** |

The manifest additionally records that `bare/stderr` and `help/stdout`
**hash identically to each other** on the head side, at `50714a88…`
(the manifest truncates it to eight hex digits, so eight is what is
compared). Both properties reproduce:

- `cmp bare.stderr help.stdout` → **identical**;
- full SHA-256 of both:
  `50714a88db8725654e37076afdf4cffa9931b69cee939df1fd070127e3d1b389`,
  whose recorded prefix `50714a88` **MATCHES**.

The two remaining recorded literals:

| Literal | Recorded | Result |
|---|---|---|
| unknown-subcommand first line | `rigprog: unknown subcommand "nosuchcmd"` | **MATCH** |
| `UnknownModelError` (head) | `rigprog probe: wiring: unknown model "NO-SUCH-MODEL" (supported: FT-710, FTdx10)` | **MATCH** |

`probe --fake --model NO-SUCH-MODEL` exits **2** with **16** stderr
lines, both as recorded. `NO-SUCH-MODEL` is the repo's own
`unknownModelSentinel`.

No full-file hash is recorded for `nosuchcmd`'s stderr or for the
`UnknownModelError` stderr, so none is claimed here; those rows are
compared on the line counts, exit values and literals the manifest
does record.

**Part 2 totals: 18 comparisons, 18 MATCH** — 1 recorded hash prefix,
1 stream-equality (`cmp`) property, 6 line counts, 3 exit values, 3
model-list literals, the unknown-subcommand first line, and the
`UnknownModelError` line/exit/line-count triple. **The one surface
M9c-6 was allowed to move did not move.**

---

## Part 3 — the FTdx10 baselines: 14 legs + 9 file artefacts, all MATCH

M9c-6 minted these as "the hash table a future milestone diffs". M9d-1
is that future milestone, and it diffs to nothing. Run from
`<mirror-dx10>/`.

| Leg | Recorded exit | Recorded stdout SHA-256 | Result |
|---|---|---|---|
| `probe --fake --model FTdx10` | 0 | `cf9a8b077c09d182bc18a13553e8d1c9ff3291f0d739107a3341ca373d128728` | **MATCH** |
| `read --fake --model FTdx10` | 0 | `2006a6b4d4b5b72ae43e33b6fb0fd6da1c28b8bacdf5816ba2cdd7121e510757` | **MATCH** |
| `read --fake --settings --model FTdx10` | 0 | `4ddf9915838309571bf0f313cd0c8e9d452bf909ee91d0efbc4d441c95c4e0ab` | **MATCH** |
| `diff --fake --model FTdx10` (fresh read) | 0 | `7917a3bde4d3bec60710504d0cde4682aaafeea573273b5d683803aee2cbbad6` | **MATCH** |
| `settings --model FTdx10` | 0 | `4c70117d9c195e4a5b091a18123d3112ff1a51807a73758b04f63b0c7d6cc1b8` | **MATCH** |
| `settings --csv --model FTdx10` | 0 | `266fbc318032f6ff2c3d5995b3d7113f30ebc780f8171811c3aca518e2b8256a` | **MATCH** |
| `export` (of the read) | 0 | `e0f0036efba8821306d5bc55cb64074c696bffdf337425ca86ed2a3ac8cf1d4c` | **MATCH** |
| `import --chirp chirp_minimal.csv` | 0 | `2990ed9d60a81e89aa828effe4220c0d5638a25e1d5bf0464f948e237c9942be` | **MATCH** |
| `import --chirp chirp_skip.csv` | 0 | `bdefb46eda7af6a6fcd1eba8e4aea3ceb7a658a8ddeda999384760ac53dc7b88` | **MATCH** |
| `diff` of the clean CHIRP import | 0 | `f206cd30c873eb137fdc61d35d915b531a5db6c331f04c77388f9c5fc943e609` | **MATCH** |
| `diff` of the skip CHIRP import | 0 | `cd91cdf6550e88e3c9e5471dc39a15524bf3eeeda30f45babbb7a501dada895b` | **MATCH** |
| `import --csv` (native round trip) | 0 | `2a45ec33b50709c19382711cdd508ec0741682e74556915a26dd31fb9ad4ba5d` | **MATCH** |
| `export` of the CHIRP-imported file | 0 | `1281522d68705d7feb5dbe67d3e9bbfdd7928b8ac3ebcefc967dca203c1279ef` | **MATCH** |
| `export` of the round-tripped file | 0 | `b16c03c9cf7e2062fef1bc782d40333ce138fd6fc1f5faef2b5158af11335dfb` | **MATCH** |

**Fourteen legs, fourteen stdout hashes, fourteen exit values — every
one as recorded.**

The nine-entry file-artefact table:

| Artefact | Recorded SHA-256 | Note | Result |
|---|---|---|---|
| `ftdx10-read.json` | `95e2c8fbeaa689d8100392fa986029b63741ec2e7e577025b9836cb815d14737` | `read_at`-normalised | **MATCH** |
| `ftdx10-read-settings.json` | `7be4f8e126e49f1f4cb5a0c488569476fdd27f4c980fd7e839ccf603bf170cd6` | `read_at`-normalised | **MATCH** |
| `ftdx10-import-chirp.json` | `35d45120862791e24d6a2fa2c0a05696b8de6fb762642a4b463ebb518388c79e` | `read_at`-normalised | **MATCH** |
| `ftdx10-import-skip.json` | `db120d701a8dd20acff886098a103c78a8ad66b13b20848d4318c1ef35958fc7` | `read_at`-normalised | **MATCH** |
| `ftdx10-roundtrip.json` | `ab08427f807995920227e52d398617a540c5f39f7acd7b9654eca6fb25d33099` | `read_at`-normalised | **MATCH** |
| `ftdx10-export.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw | **MATCH** |
| `ftdx10-roundtrip.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw — equal to the line above | **MATCH**, and still equal |
| `ftdx10-import-chirp.csv` | `819b4c4ff9e7f11029b6ea65e7f77ab68725ef1fa8413424c9718f3647fc80a5` | raw | **MATCH** |
| `ftdx10-settings.csv` | `bd4db08f0047225db744b5802269dd81867117bf092920aaaf82badf02f5f390` | raw | **MATCH** |

### The three Part 3 stderr hashes the manifest does record

The task brief anticipated that Part 3 recorded no stderr hashes. Three
are in fact recorded in its prose (A1's, A2's and A3's), so all three
were compared rather than skipped — the rule being to compare wherever
a hash exists:

| Leg | Recorded stderr SHA-256 | Result |
|---|---|---|
| A1 `probe` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | **MATCH** |
| A2 `read` (117 progress lines) | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | **MATCH** |
| A3 `read --settings` (314 lines) | `8e96308b714b5657582aa6e63080ea23a99e4915d53c665753f5c2522f2cb87e` | **MATCH** |

A2's recorded property that its stderr is **byte-identical to the
FT-710's own `read.stderr`** (Part 1 row 5) also still holds, and was
re-verified the way the manifest verified it — by `cmp` between this
run's two mirror trees, not by eye. Line counts as recorded: 117 and
314.

### Corroborating properties (entailed, and spot-checked anyway)

Because the file hashes above match byte for byte, the manifest's Part 3
property tables are entailed rather than independently at risk. They
were spot-checked regardless, and every value reproduces:

| Recorded property | Value | Result |
|---|---|---|
| A2 `schema` / `radio.model` / channels | `3` / `FTdx10` / 117 | **MATCH** |
| A2 `radio.baseline_digest` | `ddfbd375f6aed6570598145c463dd59503678050a6d0368a645ed8ac74ca6297` | **MATCH** |
| A3 `menus.entries` / `complete` / `descriptor` | 197 / `true` / `ftdx10-ex@1` | **MATCH** |
| A5 rendered lines with `--model FTdx10` | 222, no "Unrecognised settings" section | **MATCH** |
| A5 CSV lines / header | 198 / `id,menu,group,label,state,value` | **MATCH** |
| A5 non-vacuity: same file WITHOUT the flag | 221 lines, 13 unrecognised, exit 0 | **MATCH** |
| A6a `tag_display` column over 117 rows | 21 × `n/a`, 96 empty | **MATCH** |
| A6b round trip lossless, way 1 | re-export `cmp`-identical to the original export | **MATCH** |
| A6b round trip lossless, way 2 | differs from the source read in `"generator"` alone; diff EMPTY once that line is removed | **MATCH** |

The A1 probe block, the A4 verdict (`Blocked 0`, `Unchanged 117`) and
both A7 verdict blocks — skip fixture and clean fixture, `Added 2,
Modified 1, Erased 0, Blocked 3, Unchanged 114` on each — reproduce
verbatim, including the `BLOCKED: scan_skip not writable on this radio`
line the manifest records as designed, pre-existing and
model-independent. A7's finding is unchanged by this milestone and is
neither re-litigated nor re-recorded here.

**Part 3 totals: 26 recorded hashes compared (14 stdout + 9 file + 3
stderr), 26 MATCH; 14 exit values compared, 14 MATCH.**

---

## The comparison, counted

Every comparison was made mechanically — a recorded-value table driven
through `shasum -a 256` and string equality, not by reading numbers off
a screen. The comparator's first pass reported a total 52/52 mismatch;
that was a zsh `path`/`PATH` harness fault which meant the comparisons
never ran at all, and the incident, its diagnosis and the re-run
transcript are recorded in full above ("The first comparator pass
reported 52/52 MISMATCH"). The figures below are the re-run's:

| Group | Full-hash rows | Other recorded rows | Mismatches |
|---|---|---|---|
| Recipe inputs (3 fixtures) | 3 | — | 0 |
| Part 1 — the eighteen | 18 | 5 exit literals | 0 |
| Part 1 — S1 | 3 | 1 exit literal | 0 |
| Part 1 — S2 | 2 | 2 exit literals | 0 |
| Part 2 — model-list surfaces | — | 18 (incl. 1 hash prefix, 1 `cmp`) | 0 |
| Part 3 — the fourteen legs | 14 | 14 exit values | 0 |
| Part 3 — nine file artefacts | 9 | — | 0 |
| Part 3 — the three stderr hashes | 3 | — | 0 |
| **Total** | **52** | **40** | **0** |

**92 recorded values compared. 92 matches. ZERO diffs.** No carve-out
was invoked, no delta was sanctioned, and nothing outside the
manifest's own tables is claimed.

---

## Full local gate results

All items run at HEAD (`fef25cb`) in the repo working tree. Each line
is the actual final result of the actual run — the M9c-5 lesson,
restated: evidence per commit, from what was observed, never from
intention.

| Gate item | Result |
|---|---|
| `gofmt -l .` | **PASS** — exit 0, no output |
| `go build ./...` | **PASS** — exit 0, clean |
| `go vet ./...` | **PASS** — exit 0, clean |
| `go test ./... -count=1` (whole tree, foreground, awaited) | **PASS** — exit 0; 24 test packages `ok` + 1 with no test files; 4 min 04 s (23:54:52Z → 23:58:56Z) |
| `go test ./internal/guards/ -v -count=1` | **PASS** — `ok … 0.532s`, 9/9 `--- PASS` by name (below) |
| `go test -race ./core/... -count=1` (backgrounded first, **exit status collected**) | **PASS** — **exit 0**, 12/12 packages `ok`, 4 min 10 s (23:50:36Z → 23:54:46Z) |
| eight-path golden diff gate | **PASS** — exit 0, empty, as one invocation and as eight |
| `git status --short` | **PASS** — clean; the binary and all capture artefacts live in the scratch mirror trees only |

### The full Go suite, package by package

`go test ./... -count=1`, **exit 0**. Twenty-four test packages, every
one `ok`, plus one with no test files:

```
ok  	github.com/gm5dna/open-rig-programmer/app	107.669s
ok  	github.com/gm5dna/open-rig-programmer/cmd/rigprog	163.254s
ok  	github.com/gm5dna/open-rig-programmer/core/cat	1.126s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/dialecttest	0.844s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx10	2.285s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx101	0.617s
ok  	github.com/gm5dna/open-rig-programmer/core/clone	243.192s
ok  	github.com/gm5dna/open-rig-programmer/core/codeplug	1.682s
ok  	github.com/gm5dna/open-rig-programmer/core/csvio	1.566s
ok  	github.com/gm5dna/open-rig-programmer/core/driver	2.523s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ft710	58.213s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ftdx10	69.033s
ok  	github.com/gm5dna/open-rig-programmer/core/spec	2.410s
ok  	github.com/gm5dna/open-rig-programmer/core/transport	18.309s
ok  	github.com/gm5dna/open-rig-programmer/internal/buildinfo	2.163s
ok  	github.com/gm5dna/open-rig-programmer/internal/csvmerge	2.285s
ok  	github.com/gm5dna/open-rig-programmer/internal/extable	1.936s
?   	github.com/gm5dna/open-rig-programmer/internal/extable/gen	[no test files]
ok  	github.com/gm5dna/open-rig-programmer/internal/extable/observe	2.087s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakedx10	3.771s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakedx10/gen	1.346s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakeradio	7.203s
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	1.401s
ok  	github.com/gm5dna/open-rig-programmer/internal/radiotext	1.037s
ok  	github.com/gm5dna/open-rig-programmer/internal/wiring	10.895s
```

**Twenty-four, against M9c-6's twenty-three. None dropped; one added,
and it is exactly this milestone's one new package** —
`core/cat/ftdx101`. The arithmetic closes with no unexplained package on
either side.

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
--- PASS: TestRetiredWriteResultNamesAreGone (0.01s)
--- PASS: TestSimulatedProfileTokensConfinement (0.02s)
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.532s
```

Nine, unchanged in number from M9c-6 and M9c-5. M9d-1 adds no tenth
guard: the new package is held by the existing `core/cat/**` rules —
the Set-builder fence, the CAT-isolation rule and the dialecttest
production-import guard — rather than by a rule of its own.

### `go test -race ./core/...`

Started FIRST, in the background, before any other work in this task
(`23:50:36Z`), and **its exit status was collected** (`23:54:46Z`) —
4 min 10 s, exit **0**. The background allowance exists only for the
foreground time limit; an uncollected race run would be no gate, so the
status was written to a file by the job itself and read back before this
note was written, and before the commit. Every package:

```
ok  	github.com/gm5dna/open-rig-programmer/core/cat	1.783s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/dialecttest	1.829s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx10	1.409s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx101	2.197s
ok  	github.com/gm5dna/open-rig-programmer/core/clone	244.842s
ok  	github.com/gm5dna/open-rig-programmer/core/codeplug	4.982s
ok  	github.com/gm5dna/open-rig-programmer/core/csvio	2.391s
ok  	github.com/gm5dna/open-rig-programmer/core/driver	2.601s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ft710	58.803s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ftdx10	70.039s
ok  	github.com/gm5dna/open-rig-programmer/core/spec	3.305s
ok  	github.com/gm5dna/open-rig-programmer/core/transport	18.612s
```

**Twelve core packages, race detector on, zero reports** — eleven as in
M9c-6, plus `core/cat/ftdx101`.

### The golden corpora — all EIGHT paths

Untouched, every one. Run as a single invocation and then again
path by path, both exit 0 with empty output:

```bash
git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go \
  core/cat/ftdx10/testdata/ core/cat/ftdx10/exinventory_gen.go \
  internal/fakedx10/exinventory_gen.go internal/fakedx10/transcription-b.csv \
  core/cat/ftdx101/testdata/ core/cat/ftdx101/exinventory_gen.go
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
| all eight in one invocation | **PASS** — exit 0, empty |

**No golden was regenerated at any point in M9d-1**; the quarantined
artefacts committed in tasks 1, 2, 3, 4 and 7a stand at their
commit-time bytes, and the FTdx101 corpus joins the FT-710's and the
FTdx10's under the same freeze.

### The three generated inventories, against M9c-6's Part 5 table

`git diff --exit-code` proves those files are unchanged **since HEAD**,
which is a weaker statement than the manifest's. M9c-6's regeneration
table published absolute SHA-256 values for the three generated EX
inventories, so those are compared directly too — three more recorded
values, and three more matches:

| Generated file | M9c-6 recorded SHA-256 | This run | Result |
|---|---|---|---|
| `internal/fakedx10/exinventory_gen.go` | `0d44f04bef5ece957fc324c8350cef9af5a9d6899b83639a2ad3bdff803dad48` | identical | **MATCH** |
| `core/cat/ftdx10/exinventory_gen.go` | `9311fc928b540110539d5dc40c921193b39890fbd6ef8c6f27c1e0db3c2171d4` | identical | **MATCH** |
| `core/cat/exinventory_gen.go` | `fbf4f02e3e564357eecd020f612de58cd3d26ad58a8a6e8ee9cb1249815ada22` | identical | **MATCH** |

The fakedx10 value is the one task 5 of M9c-6 recorded and the manifest
re-recorded — the staleness property now holding across a further 19
commits and a whole milestone. M9d-1's own generated inventory,
`core/cat/ftdx101/exinventory_gen.go`, has no M9c-6 value to compare
against (it did not exist); it is covered by the eight-path golden gate
above and by `core/cat/ftdx101/staleness_test.go` in the full suite.

**These three are additional to the 92 recorded values counted above**,
which are the manifest's Part 1/2/3 recipe tables; Part 5 is a gate
record rather than a recipe table. Counting them, the run compares **95
recorded values with 95 matches.**

---

## The gate-at-final-code-tip invariant

Re-stated, because it is the invariant a merge reviewer should check
rather than any particular hash:

> The byte-identity capture is taken at the LAST CODE COMMIT of the
> branch. Every commit after the capture must be documentation-only for
> the capture to speak for the branch tip.

**The last CODE commit on `m9d1-ftdx101-dialect` is `fef25cb`** (task
7b). Every measurement in this note was taken at that commit, in that
working tree, with a binary built from it.

**This note's own commit is DOCUMENTATION-ONLY** in the invariant's
sense. It tracks exactly one path:

| Path | Kind |
|---|---|
| `docs/superpowers/m9d1-baseline-note.md` | this note |

No `.go` file, no frontend file, no generated binding and no golden is
touched by it — `git show --stat` of this commit is the whole of its
contents, and it is the check to run rather than to take on trust. The
capture therefore speaks for the branch tip **as of this commit**.

If a milestone-review wave lands after this one, this note does **not**
retroactively cover it. That is not hypothetical: the worked example is
in this very run, where M9c-6's own review wave `8baab59` — landed after
that manifest's capture and explicitly excluded from it — is covered
here only because this run's span happens to include it, and was covered
before that only by its own gate. Any M9d-1 review wave must do the
same: re-run, re-record, per commit.

The planning and handoff documents are deliberately not in this commit:
`.superpowers/` is gitignored, as is `docs/fixtures-private/`, so the
task briefs and reports are on-disk working files no commit contains.
Only tracked files can affect whether a post-capture commit is
documentation-only.

---

## Scope of the claim this note supports

It supports: **across the whole of M9d-1, and in fact across the whole
span from M9c-6's capture at `490c38c` to `fef25cb`, the FT-710's probe,
read, CHIRP-import (both a blocked and a clean one), native
CSV-export/round-trip and `diff` paths, AND every one of the FTdx10's
fourteen recorded acceptance legs, reproduce the M9c-6 manifest's
recorded values exactly — stdout, stderr, exit code and file artefact
alike, with no normalisation beyond the one declared `read_at` noise
field, no carve-out of any kind, and no sanctioned delta; the
registry-driven model list and its `UnknownModelError` twin — the only
surfaces M9c-6 was permitted to move — are unmoved, because M9d-1
registers no model; and the FTdx101 dialect package exists, is fully
tested, and is reachable from nothing the CLI links.**

It does **not** support a claim about: the write path to a real radio of
any model (no radio is opened anywhere in this recipe); **the FTdx101's
correctness against real hardware in any respect** — the entire
`core/cat/ftdx101` package is UNVERIFIED, manual-derived and
fixture-exercised only, no FTdx101 has ever been asked anything, and
this recipe never invokes it, so these values say nothing whatever about
it; the FTdx10's correctness against real hardware (every FTdx10 value
here is the fake's answer, and the fake's EX values are invented by a
documented convention); the `ports`, `write` or real-`--port` paths; the
GUI at runtime — the frontend was **not** typechecked, unit-tested or
built in this gate, which ran the Go gate only, so `internal/radiotext`'s
GUI-facing `ToneScanSkipNote` is covered here only by its unit tests and
by the measured fact that no CLI leg prints it; or the base-versus-head
`cmp` verdicts M9c-6 recorded for streams it published no hash for —
those required its two-binary setup, and rows carrying no recorded value
are compared on nothing and claimed for nothing.

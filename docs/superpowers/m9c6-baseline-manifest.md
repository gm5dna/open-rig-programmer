# M9c-6 baseline manifest

Captured **30/07/2026**, as Task 8's byte-identity gate for the M9c-6
FTdx10-registration milestone — the LAST milestone of M9c. The FTdx10
becomes a registered, selectable model: a driver, a fake, wiring/
radiotext/guards rows, and the caps-derived frontend items.

**This milestone's bar is the hard one again.** M9c-5's bar was
"byte-identical EXCEPT an enumerated list of carve-outs", because E1
changed the FT-710's own on-disk data. M9c-6 changes no FT-710 data at
all, so the bar returns to the M9c-3/M9c-4 form, with exactly one
designed exception the spec named BEFORE this gate ran:

> **Every FT-710 recipe leg is byte-identical to the base build —
> stdout, stderr and exit code, by `cmp`, with NO normalisation of any
> kind and NO carve-outs. Outside the recipe, EXACTLY ONE surface is
> sanctioned to differ: the model list a registry-driven print
> interpolates. Any other difference anywhere is a DEFECT, never a
> baseline to update.**

This gate found no such difference. **Eighteen artefact rows, eighteen
`cmp`-identical, zero carve-outs invoked.**

- **BASE commit:** `78b73acb26b65f14eae51280079b180387bb03ea` ("Merge
  m9c5-registration-enablers: the six neutral-core registration
  enablers") — the commit `m9c6-ftdx10-registration` forked from,
  confirmed by `git merge-base main m9c6-ftdx10-registration`.
- **HEAD under test:** `490c38c` ("M9c-6 task 7: capability-derived core
  fields, honest CHIRP imports, the in-cell route to Known, and the E1
  acceptance test"), the seventh and **last CODE commit** on the branch.
  Seven task commits, 63 files, +13,062/-265.
- **Branch:** `m9c6-ftdx10-registration`
- **Toolchain:** `go1.26.5 darwin/arm64`; npm 11.17.0; wails from
  `~/go/bin/wails`
- **OS:** macOS 26.5.1 (build 25F80), arm64
- **Artefacts** were captured to an ephemeral scratch directory (not
  preserved beyond this run); this manifest is the tracked, durable
  record, per the M9b review finding that artefacts alone have no
  provenance.
- **Worktree:** `git worktree add <scratch>/m9c6-base 78b73ac`, removed
  immediately after capture; `git worktree list` shows none outstanding.

---

## Part 1 — FT-710 byte identity: ZERO diffs

### Reproduction

Two binaries compiled from source (never `go run`, which collapses exit
codes — the standing artefact from the M9a gate notes):

```bash
go build -o <scratch>/bin/rigprog-head ./cmd/rigprog   # from the repo root at 490c38c
go build -o <scratch>/bin/rigprog-base ./cmd/rigprog   # from the 78b73ac worktree
```

Each binary was run **from its own mirror tree root**, writing to the
same relative path `.capture/`, so that every path string the CLI echoes
is byte-comparable between the two runs. `.capture/` is not covered by
`.gitignore`, so — as in M9c-4 and M9c-5 — each side used a minimal
**mirror tree** containing only `.capture/` and the two inputs the
recipe reads, at their identical relative paths:

```
<mirror>/.capture/
<mirror>/core/csvio/testdata/chirp_sample.csv
<mirror>/docs/superpowers/m9c5-fixtures/chirp_minimal.csv
```

Every path string passed to the CLI — and hence every path string it
echoes — is therefore character-for-character identical between the two
runs, and neither the repository nor the base worktree was written to.
Both inputs are byte-identical between the mirrors (`cmp`), and both are
untouched on the branch (`git diff 78b73ac..490c38c -- <both>` empty):

| Input | SHA-256 |
|---|---|
| `core/csvio/testdata/chirp_sample.csv` | `ee3f2664242bdd2292f7afc49d46d53338f3afa227cccfd57b24855e4216c7be` |
| `docs/superpowers/m9c5-fixtures/chirp_minimal.csv` | `87c8a9b1ac12c188a8eb726cbe1757ac01e8253ccb1741f6fe9d1af12dc02ddd` |

The second hash is the value the M9c-5 manifest recorded for the same
fixture, reproduced here from a fresh copy — the fixture is REUSED, not
re-minted.

The recipe, verbatim, exactly as M9c-5 executed it:

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
`echo $?`), matching the M9c-1/M9c-3/M9c-4/M9c-5 convention so the
recorded hashes are directly comparable across milestones. `set -e` is
omitted: the historical import leg exits 3 by design and would abort the
script. The historical leg writes NO output file (it exits 3 before
`saveCodeplugNoClobber`) and contributes stdout, stderr and exit code
only — M9c-5's correction 1, carried forward; the `--out` flag is kept
so the invocation stays character-identical to the one M9c-3, M9c-4 and
M9c-5 ran.

### Artefact table — eighteen rows, eighteen `cmp`-identical

`cmp` means byte-for-byte identical with **NO normalisation of any
kind**. Rows 17 and 18 are the two wall-clock-bearing JSONs after the
recipe's own recorded `sed` for the ONE declared noise field, `read_at`
— the only normalisation anywhere in Part 1, declared since M9c-1.

| # | Artefact | SHA-256 (both sides) | Verdict |
|---|---|---|---|
| 1 | `probe.stdout` | `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` | **MATCH** (cmp) |
| 2 | `probe.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | **MATCH** (cmp) |
| 3 | `probe.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** (cmp) |
| 4 | `read.stdout` | `a9cc9fc3834159660fdf7357c3ef3e9ad8864f36b1935bdb21e8c4f1fa252448` | **MATCH** (cmp) |
| 5 | `read.stderr` | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | **MATCH** (cmp) |
| 6 | `read.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** (cmp) |
| 7 | `import.stdout` (historical, blocked) | `fa5ee2aaf512e88fbab094993ef8dcb2ea7bd3a757749c091a16f534b65a77b7` | **MATCH** (cmp) |
| 8 | `import.stderr` (historical, blocked) | `094856b06f952559216319da39a46c4ce23f06e8e9bcc09ed355d5a523319c7a` | **MATCH** (cmp) |
| 9 | `import.exit` ("3") | `4e07408562bedb8b60ce05c1decfe3ad16b72230967de01f640b7e4729b49fce` | **MATCH** (cmp) |
| 10 | `export.stdout` | `fffb2b0a3bd10756a6f5f0c655e4e4278a4a379fbb390dddaf4b2b1c59656af0` | **MATCH** (cmp) |
| 11 | `export.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | **MATCH** (cmp) |
| 12 | `export.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** (cmp) |
| 13 | `export.csv` | `56643e7f520ded75de2957175e9b96d61d35f5205c9866cbd54d31cad78ab7fb` | **MATCH** (cmp) |
| 14 | `importmin.stdout` (clean import) | `8040f49b3cbb56293676d7cbbe7fb5cdbf232ffb96728c3ec4dc0add235a62d0` | **MATCH** (cmp) |
| 15 | `importmin.stderr` (empty) | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` | **MATCH** (cmp) |
| 16 | `importmin.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** (cmp) |
| 17 | `read-fake.json` (`read_at`-normalised) | `3faad8a7a46a14c3d624fa88c42f16065cf2c1cee756e234a8836dd9c3bf6653` | **MATCH** (cmp) |
| 18 | `import-min.json` (`read_at`-normalised) | `03bd53ae4a1a6b7a6f1ff8df41a5aa27eadb4a4fad0e988730f243af754db4a8` | **MATCH** (cmp) |

**MISMATCHES: none. Carve-outs invoked: none.** There is no
"empty after removal" row in this table and no removal command section,
because nothing had to be removed from anything.

The two raw, un-normalised JSONs differ, as declared, in exactly one
line each — `read_at`, and nothing else. Quoted rather than described,
with the whole diff shown so the arithmetic closes at 2 lines and no
remainder:

```
$ diff base/.capture/read-fake.json head/.capture/read-fake.json
7c7
<     "read_at": "2026-07-30T04:26:10.54473+01:00",
---
>     "read_at": "2026-07-30T04:26:15.239835+01:00",

$ diff base/.capture/import-min.json head/.capture/import-min.json
7c7
<     "read_at": "2026-07-30T04:26:10.54473+01:00",
---
>     "read_at": "2026-07-30T04:26:15.239835+01:00",
```

Raw hashes for the record (a comparison no run can pass, which is why
rows 17-18 exist): `read-fake.json` head
`a7025b2da7e940426d63c8655d71e7e987b6e89f9d8eb3e5f63905242164200c`, base
`515fcb89cec625c3d21f11818b8e28dbbd561f804748d66c1b5189ec9a803072`;
`import-min.json` head
`25079d554409489488f69f44052db7f90f84372f555ed6083803c491db138739`,
base `7151de177e5909329f7709e7b489e692fa88b9dd1efc709924381c5c27b33239`.

### Two supplementary FT-710 legs, added by this gate

Both are zero-diff too, and both exist to answer a question this
milestone raises rather than to re-prove the recipe. They are recorded
as ADDITIONS, not as part of the inherited recipe.

**S1 — `diff --fake` over the clean CHIRP import.** M9c-5's recipe never
diffed `import-min.json`; Part 3 below does exactly that for the FTdx10,
so the FT-710's answer is needed as the control.

| Artefact | SHA-256 (both sides) | Verdict |
|---|---|---|
| `diffmin.stdout` | `099227a776e130295a342ccce8567b0550e77d3c2326138972f1db08fabe6f8d` | **MATCH** (cmp) |
| `diffmin.stderr` | `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf` | **MATCH** (cmp) |
| `diffmin.exit` ("0") | `5feceb66ffc86f38d952786c6d696c79c2dbc239dd4e91b46729d73a27fb57e9` | **MATCH** (cmp) |

**S2 — a native-CSV round trip carrying a Known `scan_skip`.** The
FT-710's own `export.csv`, with `scan_skip` set to `yes` on slot 001
(`awk -F, 'BEGIN{OFS=","} NR>1 && $1=="001" {$13="yes"} {print}'`),
imported back with `import --csv` and then diffed. The edited CSV is
itself byte-identical between the two sides
(`0cac78ae84eb226ad5f2d600493912d0bd4542ccccdfd75668cd71db5eeb46cb`),
because `export.csv` is (row 13).

| Artefact | SHA-256 (both sides) | Verdict |
|---|---|---|
| `csvskip-import.stdout` / `.stderr` / `.exit` ("0") | — | **MATCH** (cmp), all three |
| `csvskip-diff.stdout` | `7521da5c15a5496ccaa86314dc385c747fc73427e8d332009c04f99b2f7e774d` | **MATCH** (cmp) |
| `csvskip-diff.stderr` / `.exit` ("0") | — | **MATCH** (cmp), both |

Why S2 matters is Part 3's `scan_skip` question, and it is answered
there.

### Independent authentication of both sides

The base build is not merely asserted to be a valid baseline: **every
one of its eighteen hashes reproduces a value already recorded in the
M9c-5 manifest's HEAD column** (`f64f688`), derived here from a fresh
worktree build at a DIFFERENT commit (`78b73ac`, the merge) with no
reference to that document while the recipe ran.

`probe.stdout` `ad4bf761…`, `probe.stderr`/`export.stderr`/
`importmin.stderr` `e3b0c442…`, the four "0" exits `5feceb66…`,
`read.stdout` `a9cc9fc3…`, `read.stderr` `8e174833…`, `import.stdout`
`fa5ee2aa…`, `import.stderr` `094856b0…`, `import.exit` `4e074085…`,
`export.stdout` `fffb2b0a…`, `export.csv` `56643e7f…`,
`importmin.stdout` `8040f49b…`, and the two strongest single rows —
`read-fake.json` normalised `3faad8a7…` and `import-min.json`
normalised `03bd53ae…`.

That closes the chain end to end, across four milestones:

> `3b75fcc` = `76f3f77` (M9c-3's two-worktree gate) = `699f7c9`
> (M9c-4's note) = `6b84335` (M9c-5's base) -> `f64f688` (M9c-5's head,
> four enumerated carve-outs) = `78b73ac` (the merge, and this gate's
> base) = `490c38c` (this gate's head, **no carve-outs at all**).

The last equality is the milestone's whole FT-710 claim, and it is an
equality of hashes, not of intentions.

Two of those links are worth naming separately, because neither was
established before this run.

**`f64f688` = `78b73ac` is a NEW result, not an inherited one.** Three
commits sit between them: M9c-5's six-fix review wave `bc3b6f1`, its
re-review follow-up `8721a91`, and the merge itself. The wave changed
production frontend code, Go comments and tests, and the M9c-5 manifest
was explicit that its capture did NOT speak for them — they were covered
only by their own gate runs. This gate's base build is at the MERGE, so
its eighteen hashes reproducing M9c-5's HEAD column is independent
evidence that **the M9c-5 review wave changed no CLI-observable output
either**. That was previously asserted by scope, never measured; it is
measured now.

**`78b73ac` = `490c38c` is measured at eighteen artefacts with no
carve-out and no removal.** Not "diffs explained" — no diffs.

---

## Part 2 — the ONE designed delta, recorded verbatim

### The top-usage surface

`topUsageTextTemplate` (cmd/rigprog/usage.go:13-43) carries a `%s`
placeholder filled at PRINT time by `printUsage` from
`strings.Join(wiring.SupportedModels(), ", ")`. That is **task 40's
design** (M9a-4, the CLI neutralisation), stated in the constant's own
doc comment — "so this line stays accurate without a hand edit whenever
a second driver is registered" — and pinned by
`cmd/rigprog/usage_test.go`'s `TestPrintUsage_RegistryDriven` (:98),
which asserts the printed line names every model in
`wiring.SupportedModels()`' own order.
The M9c-6 spec sanctions the consequence in advance (spec, "Byte
identity and gates", bullet 2): registering the FTdx10 changes this line
and nothing else, and **the manifest records the before/after verbatim
rather than absorbing it silently.**

**BEFORE** (base, `78b73ac`):

```
rigprog is a command-line memory programmer for Yaesu radios (currently: FT-710).
```

**AFTER** (head, `490c38c`):

```
rigprog is a command-line memory programmer for Yaesu radios (currently: FT-710, FTdx10).
```

The line is printed by all three of the surfaces the doc comment names.
Each was captured on both binaries. **Every one differs in exactly that
one line and in nothing else**, and every exit code is unchanged:

| Surface | Stream | Head lines | Differing lines | Exit base/head |
|---|---|---|---|---|
| bare `rigprog` | stderr | 18 | **2** (one line, both sides) | 2 / 2 |
| bare `rigprog` | stdout | 0 (empty) | 0 | — |
| `rigprog help` | stdout | 18 | **2** (one line, both sides) | 0 / 0 |
| `rigprog help` | stderr | 0 (empty) | 0 | — |
| `rigprog nosuchcmd` | stderr | 20 | **2** (one line, both sides) | 2 / 2 |
| `rigprog nosuchcmd` | stdout | 0 (empty) | 0 | — |

The unknown-subcommand surface's own first line is unchanged —
`rigprog: unknown subcommand "nosuchcmd"` — so the delta really is the
model list and not the error prose. `bare/stderr` and `help/stdout` hash
identically to each other on each side (base `4baf26a6…`, head
`50714a88…`), i.e. the same 18 lines on different streams, which is
another way of saying nothing but the one line moved.

**Proved by removal, not inferred.** Strip only the model-list line from
both sides and the remaining diff is EMPTY on all six streams:

```bash
diff <(grep -v '^rigprog is a command-line memory programmer for Yaesu radios (currently: ' base) \
     <(grep -v '^rigprog is a command-line memory programmer for Yaesu radios (currently: ' head)
```

```
bare/stdout: EMPTY AFTER REMOVAL      help/stdout: EMPTY AFTER REMOVAL      unk/stdout: EMPTY AFTER REMOVAL
bare/stderr: EMPTY AFTER REMOVAL      help/stderr: EMPTY AFTER REMOVAL      unk/stderr: EMPTY AFTER REMOVAL
```

**None of the five recipe legs prints this line** — which is why Part 1
is a zero-diff table and this is a separate part. The spec predicted
exactly that ("None of those legs prints the model list") and Part 1
confirms it by measurement.

### `UnknownModelError.Supported` — the same growth on the error path

Already model-list-bearing by design; sanctioned by the same spec
bullet. `probe --fake --model NO-SUCH-MODEL` on both binaries
(`NO-SUCH-MODEL` is the repo's own `unknownModelSentinel`, introduced by
task 6 precisely because the tests used to spell the sentinel
"FTdx10"):

```
base: rigprog probe: wiring: unknown model "NO-SUCH-MODEL" (supported: FT-710)
head: rigprog probe: wiring: unknown model "NO-SUCH-MODEL" (supported: FT-710, FTdx10)
```

Exit **2** on both sides. 16 stderr lines each; **2 differing lines**
(one changed line); the 15 lines of probe usage that follow are
identical, and removing the one `unknown model` line leaves an EMPTY
diff.

**That is the complete enumeration. Two surfaces, one cause, one line
each, both sanctioned in writing before the gate ran. Nothing else in
this repository's observable output moved.**

---

## Part 3 — the FTdx10 acceptance legs: the FIRST FTdx10 baselines

Head binary only — there is no base to compare against, because at
`78b73ac` no CLI path could select this model at all. These are the
baselines a future milestone will be measured against, so they are
recorded with hashes and with the verdict lines verbatim.

Run from a third mirror tree, `<mirror-dx10>/`, holding `.capture/` and
the two CHIRP fixtures at their repo-relative paths, so every echoed
path string is reproducible:

```
<mirror>/.capture/
<mirror>/docs/superpowers/m9c5-fixtures/chirp_minimal.csv
<mirror>/docs/superpowers/m9c6-fixtures/chirp_skip.csv
```

### A1 — `probe --fake --model FTdx10` → exit 0

```
Model:         FTdx10
CAT ID:        0761
Port:          fake
USB serial:    SIM0001
Region:        -
60 m channels: 0
EMG channel:   no
Unexpected frames: 0

Firmware version has no CAT query — check the front panel. No minimum version is established for the FTdx10: this build knows of none to require.
```

stdout `cf9a8b077c09d182bc18a13553e8d1c9ff3291f0d739107a3341ca373d128728`,
stderr empty (`e3b0c442…`), exit **0**.

Every line is an M9c-6 decision made observable. `CAT ID: 0761` is
D-probe's dialect identity. `Region: -` is D-probe's "no FT-710 region
string is borrowed" — the FTdx10 driver implements no `RegionReporter`.
`60 m channels: 0` / `EMG channel: no` is D-banks' full-range discovery
finding nothing in the default fake image (99 5xx probes plus one EMG
probe, every one answered `?;`) — an EMPTY result reported honestly, not
an absent capability. The firmware note is D-radiotext's
`ProbeFirmwareNote` + `FirmwareGuidance`, both worded to claim nothing:
no CAT firmware query exists, and no minimum version is established.

### A2 — `read --fake --model FTdx10 --out .capture/ftdx10-read.json` → exit 0

```
Slots read:      117
Populated:       19
Region:          -
Baseline digest: ddfbd375f6ae (truncated)
Output:          .capture/ftdx10-read.json
```

stdout `2006a6b4d4b5b72ae43e33b6fb0fd6da1c28b8bacdf5816ba2cdd7121e510757`,
stderr `8e1748332cb9c7e09ff26a304f37279eb7d7b0604bc773d2bdb8a20397c2facf`
(117 progress lines, `read 1/117 M-01` … `read 117/117 P9U`), exit **0**.
That stderr hash is **byte-identical to the FT-710's own `read.stderr`**
(Part 1, row 5) — verified by `cmp`, not by eye: both radios present 117
static slots with the same display names, and the progress format is
model-neutral. It is a small thing that says something real about the
registration: the FTdx10 walked the CLI's model-neutral read path, not a
parallel one.

**117 = 99 MEM (001-099) + 18 PMS (9 pairs)** — D-banks' static
inventory. **19 populated** = the fake's minimal default image, M-01 plus
the nine PMS pairs.

| Property | Value |
|---|---|
| `schema` | `3` |
| `radio.model` | `FTdx10` |
| `radio.baseline_digest` | `ddfbd375f6aed6570598145c463dd59503678050a6d0368a645ed8ac74ca6297` |
| channels / populated | 117 / 19 |
| SHA-256, `read_at`-normalised | `95e2c8fbeaa689d8100392fa986029b63741ec2e7e577025b9836cb815d14737` |

The `baseline_digest` is a CONTENT digest and carries no wall clock, so
it is the reproducible identity of this read; the file hash is quoted
after the recipe's standard `read_at` normalisation, since the raw file
carries the one declared noise field.

**Every populated channel's three "honest" fields, by `jq` over all 19:**

| Field | State | Count | Decision |
|---|---|---|---|
| `tag_display` | `unavailable` | 19 / 19 | D-tagdisplay — the combined MT form has no display flag |
| `scan_skip` | `unknown` | 19 / 19 | D-tone-skip — zero `FieldSupport`, nothing verifies liveness |
| `ctcss_tone` | `unknown` | 19 / 19 | D-tone-skip, same reason |

This is the FTdx10 as the **first real `Unavailable` producer** (spec
D-tagdisplay), reaching a user through the CLI rather than only through
a unit test.

### A3 — `read --fake --settings --model FTdx10` → exit 0, 197 settings

```
Slots read:      117
Populated:       19
Region:          -
Baseline digest: ddfbd375f6ae (truncated)
Output:          .capture/ftdx10-read-settings.json
Settings read:        197
Settings unavailable: 0
```

stdout `4ddf9915838309571bf0f313cd0c8e9d452bf909ee91d0efbc4d441c95c4e0ab`,
stderr `8e96308b714b5657582aa6e63080ea23a99e4915d53c665753f5c2522f2cb87e`
(314 lines: 117 channel + 197 settings progress, `read-settings 1/197`
… `read-settings 197/197 040302`), exit **0**.

`menus.entries` length **197**; `menus.complete` **true**;
`menus.descriptor` **`ftdx10-ex@1`**. SHA-256, `read_at`-normalised:
`7be4f8e126e49f1f4cb5a0c488569476fdd27f4c980fd7e839ccf603bf170cd6`.

197 is D-settings' whole inventory read over the wire, one EX exchange
per item, zero unavailable.

### A4 — `diff --fake --model FTdx10 .capture/ftdx10-read.json` → exit 0, Blocked 0

Verdict lines verbatim:

```
No changes.

Added 0, Modified 0, Erased 0, Blocked 0, Unchanged 117
```

stdout `7917a3bde4d3bec60710504d0cde4682aaafeea573273b5d683803aee2cbbad6`,
exit **0**.

**`Blocked 0` on all 117 slots is the load-bearing number**, and it is
the D-tagdisplay decision demonstrated rather than argued. Had the read
produced `Unknown` for `tag_display` instead of `Unavailable`, every one
of the 19 populated channels would have been blocked here by a gate the
user could only clear by asserting a value the radio cannot store.
`Unavailable` is not a question outstanding, so it does not block. (An
UNCHANGED entry is not field-gated at all; A6 below is where the gates
actually bite, and that is the interesting result.)

### A5 — offline `settings` → exit 0, and `settings --csv`

`settings --model FTdx10 .capture/ftdx10-read-settings.json` → exit
**0**, stdout
`4c70117d9c195e4a5b091a18123d3112ff1a51807a73758b04f63b0c7d6cc1b8`,
222 lines: **4 menus, 18 groups, 197 item lines, and NO "Unrecognised
settings" section**. First lines:

```
RADIO SETTING
  MODE SSB
    01-01-01  AF TREBLE GAIN       000   
    01-01-02  AF MIDDLE TONE GAIN  000   
```

`settings --csv .capture/ftdx10-settings.csv --model FTdx10 …` → exit
**0**, stdout
`266fbc318032f6ff2c3d5995b3d7113f30ebc780f8171811c3aca518e2b8256a`;
the CSV is `bd4db08f0047225db744b5802269dd81867117bf092920aaaf82badf02f5f390`,
**198 lines** (header + 197), columns
`id,menu,group,label,state,value` exactly as the usage text promises.

**Non-vacuity of `--model FTdx10`, measured.** The same file rendered
WITHOUT the flag (defaulting to FT-710) also exits 0 — but produces an
`Unrecognised settings` section with **13 entries** and 221 lines. With
the flag: 0 unrecognised, 222 lines. The flag is therefore doing real
work: it selects the FTdx10 descriptor, and the FTdx10 descriptor is the
only one that recognises all 197.

The EX values shown (`000`, `0000`, `0`, …) are the fake's INVENTED
defaults — fakedx10's ASSUMED-register entry 4, numeric → zeros, text →
12 spaces. This baseline records what the FAKE answers; it is not a
claim about any real FTdx10's menu values.

### A6 — `export` → exit 0, and the CHIRP/native import legs

```
$ rigprog export --csv .capture/ftdx10-export.csv .capture/ftdx10-read.json
Rows written: 117
Output:       .capture/ftdx10-export.csv
```

exit **0**, stdout
`e0f0036efba8821306d5bc55cb64074c696bffdf337425ca86ed2a3ac8cf1d4c`, CSV
`429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556`.

**A6a — CLEAN CHIRP import** (`chirp_minimal.csv`, the M9c-5 fixture
reused: three channels, plain HF modes, no duplex, no tone, no skip,
names well inside the tag width), into a copy of the FTdx10 read:

```
$ rigprog import --chirp docs/superpowers/m9c5-fixtures/chirp_minimal.csv \
                 --model FTdx10 --into .capture/ftdx10-into-chirp.json \
                 --out .capture/ftdx10-import-chirp.json
No CHIRP import loss.
offline validation notes — authoritative validation happens at write time against the connected radio:
  none.
Output: .capture/ftdx10-import-chirp.json
```

exit **0**, LOSS-FREE, zero offline validation notes. stdout
`2990ed9d60a81e89aa828effe4220c0d5638a25e1d5bf0464f948e237c9942be`;
result file (normalised)
`35d45120862791e24d6a2fa2c0a05696b8de6fb762642a4b463ebb518388c79e`.

**`tag_display` is `Unavailable` on the imported channels — verified two
ways**, because this is D-tagdisplay's end-to-end acceptance and one
rendering is not proof.

1. In the file, by `jq` over the three imported slots:

```json
{"slot":"001","tag":"MINIMALUSB","tag_display":{"state":"unavailable"},"scan_skip":{"state":"known"}}
{"slot":"002","tag":"MINIMALLSB","tag_display":{"state":"unavailable"},"scan_skip":{"state":"known"}}
{"slot":"003","tag":"MINIMALCW", "tag_display":{"state":"unavailable"},"scan_skip":{"state":"known"}}
```

2. Through `export` of that file (`ftdx10-import-chirp.csv`,
   `819b4c4ff9e7f11029b6ea65e7f77ab68725ef1fa8413424c9718f3647fc80a5`),
   where `Unavailable` has its own CSV spelling `n/a`:

```
slot,display,freq_hz,mode,clar_hz,rx_clar,tx_clar,ctcss,ctcss_tone,shift,tag,tag_display,scan_skip
001,M-01,14200000,USB,,,,OFF,,SIMPLEX,MINIMALUSB,n/a,no
002,M-02,7150000,LSB,,,,OFF,,SIMPLEX,MINIMALLSB,n/a,no
003,M-03,14050000,CW-U,,,,OFF,,SIMPLEX,MINIMALCW,n/a,no
```

Across all 117 rows the `tag_display` column takes exactly two values:
**`n/a` on all 21 populated rows** (19 read + 2 newly added by the
import) and empty on the 96 empty slots. On the M9c-5 FT-710 baseline
the same column reads `no` (Known-false) and `` (Unknown) — a different
radio, a different honest answer, one derivation.

**A6b — native CSV round trip.** `import --csv` of the FTdx10's own
export back onto the read:

```
$ rigprog import --csv .capture/ftdx10-export.csv --model FTdx10 \
                 --into .capture/ftdx10-read.json --out .capture/ftdx10-roundtrip.json
offline validation notes — authoritative validation happens at write time against the connected radio:
  none.
Output: .capture/ftdx10-roundtrip.json
```

exit **0**, stdout
`2a45ec33b50709c19382711cdd508ec0741682e74556915a26dd31fb9ad4ba5d`.
**LOSSLESS, two ways:**

- re-exporting the round-tripped file yields
  `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` —
  **byte-identical to the original export**, by `cmp`, no normalisation;
- the JSON itself differs from the source read in **exactly one line**,
  `"generator"` (`open-rig-programmer/core/clone` → `rigprog/dev`: the
  read was written by the clone service, the import by the CLI), and
  `diff <(grep -v '"generator":' a) <(grep -v '"generator":' b)` is
  **EMPTY**. Every channel, including all 19 `Unavailable` tag displays
  and 19 `Unknown` tone/skip states, survives the round trip untouched.

### A7 — the BLOCKED-BY-DESIGN leg, and what it actually revealed

The brief anticipated that only a **Skip-carrying** CHIRP row would meet
the `scan_skip` refusal. The measurement says something sharper, and it
is recorded as measured:

> **`core/csvio/chirp.go:610-616` maps a BLANK `Skip` cell to
> `{Known, false}`, not to `Unknown`.** So EVERY CHIRP row — skip-
> carrying or not — arrives with a KNOWN `scan_skip`; `addedFields`
> (`core/codeplug/diff.go:227-229`) admits `FieldScanSkip` exactly when
> the state is Known; and `FieldScanSkip` is zero-support on the FTdx10.
> Therefore **every** CHIRP-imported FTdx10 channel is blocked at plan
> time, including the "clean" fixture's.

A dedicated fixture was minted anyway, so the refusal is documented
against a row that asks for the skip explicitly rather than one that
merely fails to deny it:

| Fixture | SHA-256 |
|---|---|
| `docs/superpowers/m9c6-fixtures/chirp_skip.csv` | `f5f23977d1ef67ee23943324e9796d508bbec12efdedbd0a1bac44b78e07b6c5` |

Three rows, identical to `chirp_minimal.csv` except `Skip=S` on each.
It **imports cleanly** (exit **0**, `No CHIRP import loss.`, stdout
`bdefb46eda7af6a6fcd1eba8e4aea3ceb7a658a8ddeda999384760ac53dc7b88`) —
the refusal is not an import-time one — and surfaces at plan time:

```
$ rigprog diff --fake --model FTdx10 .capture/ftdx10-import-skip.json      # exit 0
Added:
  M-02: freq 7150000 Hz, mode LSB, tag "SKIPLSB"
    BLOCKED: scan_skip not writable on this radio
  M-03: freq 14050000 Hz, mode CW-U, tag "SKIPCW"
    BLOCKED: scan_skip not writable on this radio
Modified:
  M-01: freq 7000000→14200000 Hz, mode LSB→USB, tag ""→"SKIPUSB"
    BLOCKED: scan_skip not writable on this radio

Added 2, Modified 1, Erased 0, Blocked 3, Unchanged 114
```

and the CLEAN fixture gives the identical verdict, word for word, with
its own tags:

```
$ rigprog diff --fake --model FTdx10 .capture/ftdx10-import-chirp.json     # exit 0
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

stdout hashes `cd91cdf6550e88e3c9e5471dc39a15524bf3eeeda30f45babbb7a501dada895b`
(skip) and `f206cd30c873eb137fdc61d35d915b531a5db6c331f04c77388f9c5fc943e609`
(clean); exit **0** on both (`diff` exits 0 whenever the diff was
computed and rendered — its documented contract).

**This is DESIGNED behaviour of a pre-existing class, not an M9c-6
defect.** Four independent lines of evidence, none of them an assertion:

1. **The FT-710 does the same thing, on the same message, on BOTH
   binaries.** Leg S2 above: an FT-710 channel carrying a Known
   `scan_skip` (via native CSV, where `tag_display` stays Known and so
   the earlier gate does not fire) is blocked with the byte-identical
   reason —

   ```
   Modified:
     M-01: freq 7000000→7000000 Hz, mode LSB→LSB, tag ""→""
       BLOCKED: scan_skip not writable on this radio

   Added 0, Modified 1, Erased 0, Blocked 1, Unchanged 116
   ```

   — and base and head produce that output byte-for-byte identically.
   The refusal is model-independent and predates this branch.

2. **The gate's code is untouched by this milestone.**
   `git diff --name-only 78b73ac..490c38c -- core/codeplug/diff.go
   core/driver/ft710/ core/csvio/export.go core/csvio/import.go` is
   EMPTY. The entire `core/driver/ft710` tree is untouched; the only
   `core/codeplug` file that moved at all is `channel.go`, and its diff
   contains **zero non-comment changed lines** (task 6's stale-prose
   sweep).

3. **The FT-710's CHIRP import is blocked too — on a different, earlier
   gate.** Leg S1: `diff --fake` over the FT-710's own
   `import-min.json` blocks the same three channels with `BLOCKED: tag
   display unknown — set On or Off before sending`. Diff's gate 3 is
   ordered ahead of the field-gate aggregation and short-circuits, so on
   the FT-710 the `tag_display` refusal MASKS the `scan_skip` one. On
   the FTdx10, D-tagdisplay correctly removes the masking gate
   (`Unavailable` is not a question), and the next gate in line becomes
   visible for the first time. Nothing new fires; something old stops
   being hidden.

4. **Task 7 predicted this in writing, before the measurement.**
   `core/csvio/chirp_test.go`, in
   `TestImportCHIRP_UnavailableTagDisplayDoesNotBlockTheDiff`'s fixture
   comment: *"Against a real FTdx10 — or a real FT-710 — a CHIRP import
   also meets the scan_skip gate, since neither radio can write that
   field and CHIRP's Skip column produces a Known one. That is
   E1-independent, predates this milestone, and is exactly the masking
   this fixture removes."*

**Consequence, stated plainly and NOT fixed here** (this is a
documentation-only task; a production change would belong to a
milestone that specs it): a CHIRP file cannot today be imported and then
SENT to either radio without the user first resolving a per-channel
state the protocol cannot carry. The FTdx10's route out is narrower
than the FT-710's, because `scan_skip` — unlike `tag_display` — has no
in-grid route to a writable value. Ledgered here as a finding, with the
evidence above establishing that it is neither new nor introduced by
registration.

### FTdx10 baseline summary — the hash table a future milestone diffs

| Leg | Exit | stdout SHA-256 |
|---|---|---|
| `probe --fake --model FTdx10` | 0 | `cf9a8b077c09d182bc18a13553e8d1c9ff3291f0d739107a3341ca373d128728` |
| `read --fake --model FTdx10` | 0 | `2006a6b4d4b5b72ae43e33b6fb0fd6da1c28b8bacdf5816ba2cdd7121e510757` |
| `read --fake --settings --model FTdx10` | 0 | `4ddf9915838309571bf0f313cd0c8e9d452bf909ee91d0efbc4d441c95c4e0ab` |
| `diff --fake --model FTdx10` (fresh read) | 0 | `7917a3bde4d3bec60710504d0cde4682aaafeea573273b5d683803aee2cbbad6` |
| `settings --model FTdx10` | 0 | `4c70117d9c195e4a5b091a18123d3112ff1a51807a73758b04f63b0c7d6cc1b8` |
| `settings --csv --model FTdx10` | 0 | `266fbc318032f6ff2c3d5995b3d7113f30ebc780f8171811c3aca518e2b8256a` |
| `export` (of the read) | 0 | `e0f0036efba8821306d5bc55cb64074c696bffdf337425ca86ed2a3ac8cf1d4c` |
| `import --chirp chirp_minimal.csv` | 0 | `2990ed9d60a81e89aa828effe4220c0d5638a25e1d5bf0464f948e237c9942be` |
| `import --chirp chirp_skip.csv` | 0 | `bdefb46eda7af6a6fcd1eba8e4aea3ceb7a658a8ddeda999384760ac53dc7b88` |
| `diff` of the clean CHIRP import | 0 | `f206cd30c873eb137fdc61d35d915b531a5db6c331f04c77388f9c5fc943e609` |
| `diff` of the skip CHIRP import | 0 | `cd91cdf6550e88e3c9e5471dc39a15524bf3eeeda30f45babbb7a501dada895b` |
| `import --csv` (native round trip) | 0 | `2a45ec33b50709c19382711cdd508ec0741682e74556915a26dd31fb9ad4ba5d` |
| `export` of the CHIRP-imported file | 0 | `1281522d68705d7feb5dbe67d3e9bbfdd7928b8ac3ebcefc967dca203c1279ef` |
| `export` of the round-tripped file | 0 | `b16c03c9cf7e2062fef1bc782d40333ce138fd6fc1f5faef2b5158af11335dfb` |

**Fourteen FTdx10 legs, every one exit 0.** File artefacts:

| Artefact | SHA-256 | Note |
|---|---|---|
| `ftdx10-read.json` | `95e2c8fbeaa689d8100392fa986029b63741ec2e7e577025b9836cb815d14737` | `read_at`-normalised |
| `ftdx10-read-settings.json` | `7be4f8e126e49f1f4cb5a0c488569476fdd27f4c980fd7e839ccf603bf170cd6` | `read_at`-normalised |
| `ftdx10-import-chirp.json` | `35d45120862791e24d6a2fa2c0a05696b8de6fb762642a4b463ebb518388c79e` | `read_at`-normalised |
| `ftdx10-import-skip.json` | `db120d701a8dd20acff886098a103c78a8ad66b13b20848d4318c1ef35958fc7` | `read_at`-normalised |
| `ftdx10-roundtrip.json` | `ab08427f807995920227e52d398617a540c5f39f7acd7b9654eca6fb25d33099` | `read_at`-normalised |
| `ftdx10-export.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw |
| `ftdx10-roundtrip.csv` | `429e7249ef25c6fb5ffd2f55e2a558c8a6f0ce8070eb13b62cc76bb0bd947556` | raw — **equal to the line above** |
| `ftdx10-import-chirp.csv` | `819b4c4ff9e7f11029b6ea65e7f77ab68725ef1fa8413424c9718f3647fc80a5` | raw |
| `ftdx10-settings.csv` | `bd4db08f0047225db744b5802269dd81867117bf092920aaaf82badf02f5f390` | raw |

**What these baselines do NOT support.** No real FTdx10 was involved and
none can be: `writeTrialsComplete=false` is pinned, the RealHardware
profile reports Unverified write support, and the capability gate
refuses before a frame is built. Every value above is the FAKE's answer,
and the fake's EX values are invented by the documented convention. The
write path is exercised only through the Simulated profile in unit tests
(`TestOpenFakeSessionFor_FTdx10SimulatedWriteRoundTrip`, task 6), never
by this recipe. These are baselines for REGRESSION detection, not
evidence about hardware.

---

## Part 4 — the red-proof index

Every guard, pin and cross-check this milestone added was shown to FIRE
on a deliberate violation, per the M9c-4 discipline. The proofs were run
by each task's own implementer and recorded verbatim in that task's
commit body; this index is the map, not a re-run. **What fires / what
proves it**, with the commit that holds the transcript.

| # | Commit | The defect injected | What fired |
|---|---|---|---|
| **Task 1 — `7ffcf26`, driver skeleton: 15 mutations** | | | |
| 1.1 | `7ffcf26` | discovery terminates early on first rejection | transcript test at frame 3 + empty-inventory frame count |
| 1.2 | `7ffcf26` | probe/read via MR instead of combined MT | transcript test + the never-MR test |
| 1.3 | `7ffcf26` | `TagDisplay` mapped Unknown, not Unavailable | the mapping test |
| 1.4 | `7ffcf26` | Simulated clarifier declared Inert | the profile matrix, MEM and PMS |
| 1.5 | `7ffcf26` | `MaxFreqHz` left zero | the caps-explicitness test |
| 1.6 | `7ffcf26` | `writeTrialsComplete` flipped true | its pin |
| 1.7 | `7ffcf26` | 115200 added to `Bauds` | the baseline-shape test |
| 1.8 | `7ffcf26` | a `Region()` method added | the RegionReporter-absence test |
| 1.9 | `7ffcf26` | MT `ExpectLen` dropped | the derivation test |
| 1.10 | `7ffcf26` | the geometry guard removed | the derivation test |
| 1.11 | `7ffcf26` | discovered banks inherit static banks' writes | the read-only assertions |
| 1.12 | `7ffcf26` | empty-slot mapping removed | the empty-slot test |
| 1.13 | `7ffcf26` | slot-echo check removed | its `errors.As` leg |
| 1.14 | `7ffcf26` | parse error flattened to `fmt.Errorf` | its `errors.As` leg |
| 1.15 | `7ffcf26` | mode derivation reversed / placeholder included | the order and count assertions |
| — | `7ffcf26` | **`SynthesiseDiscoveredBanks`' static exclusion deleted** | **NOTHING fired — recorded, not hidden.** The exclusion is kept (it guards a future static bank claiming a 5xx form) and the test now states what it can and cannot attribute. |
| **Task 2 — `9cb9dd5`, MT-only write: 4 mutations** | | | |
| 2.1 | `9cb9dd5` | Kind taken from `dialect.MWWriteKind()` | `TestBuildWriteCommand_P7IsTheFormConstant` |
| 2.2 | `9cb9dd5` | RxClar/TxClar swapped | the 41-byte literal pin, showing both frames |
| 2.3 | `9cb9dd5` | the FT-710's non-Known-TagDisplay refusal transplanted in | `TestBuildWriteCommand_NoTagDisplayRefusalTakesPriority` (3 rows), `TestWriteChannel_UnavailableTagDisplayIsNotRefused` (2), the literal pin |
| 2.4 | `9cb9dd5` | `requestedFields`' TagDisplay conditional made unconditional | the ordinary-channel row + the literal pin (every write refused) |
| **Task 3 — `f7b1ec9`, settings: 4 mutations** | | | |
| 3.1 | `f7b1ec9` | `%02d` → `%d` on the menu ID | the descriptor-shape test (4 menu IDs + a 0-item total) |
| 3.2 | `f7b1ec9` | a bare `"EX"` `ExpectPrefix` | the spec test, on the prefix AND the shared-prefix check |
| 3.3 | `f7b1ec9` | `strings.TrimSpace` on the raw value | the Text-item round trip |
| 3.4 | `f7b1ec9` | `Clone()` dropped | the defensive-copy test, all three getters |
| **Task 4 — `0a82785`, the fake: 10 mutations incl. the fence** | | | |
| 4.1 | `0a82785` | clarifier stored as zeros (fakeradio's borrowed behaviour) | 4 tests incl. the byte-faithful round trip |
| 4.2 | `0a82785` | answer P7 echoes the Set's placeholder | 6 tests incl. the round trip's P7 half |
| 4.3 | `0a82785` | MT Set refuses to create an absent channel | 3 tests |
| 4.4 | `0a82785` | MW clears the tag | 1 test |
| 4.5 | `0a82785` | a Set moves the selection | 1 test |
| 4.6 | `0a82785` | an empty slot answers instead of `?;` | 6 tests |
| 4.7 | `0a82785` | tag stored untrimmed | 1 test |
| 4.8 | `0a82785` | P11 strictness dropped | 1 test |
| 4.9 | `0a82785` | lower-case command names accepted | 1 test |
| 4.10 | `0a82785` | **a forbidden import in `gen/`** | **`TestNoCoreImports`** — and the counterfactual is MEASURED: a fakeradio-style `parser.ParseDir(".")` over the same tree parsed 6 files and found 0, while the recursive fence named the file. It bit even with `//go:build ignore`. |
| **Task 5 — `54a9fe4`, the generated EX projection: 3 proofs** | | | |
| 5.1 | `54a9fe4` | one width token `1`→`2` at address 030101 | `gen/TestGeneratedInventory_NotStale` — *"committed 3310 bytes, regenerated 3310 bytes … First divergence: at byte 2379"*. **Same length, one byte** — a size check would have missed it. |
| 5.2 | `54a9fe4` | the same single-row divergence | `TestEXInventoryCrossCheck_FTdx10WidthsAndShapesAgree` (*"dialect Digits=1, fake default is 2 bytes"*) AND `TestEXFTdx10RoundTrip_AllAddressesRawPort` — table leg and wire leg, independently |
| 5.3 | `54a9fe4` | `_ "internal/extable"` imported inside `gen/main.go` | `TestNoCoreImports` — the exact import the two-source design forbids |
| **Task 6 — `b7cf2d6`, registration: 5 proofs** | | | |
| 6.1 | `b7cf2d6` | FTdx10 removed from `realDrivers` only | `TestRealAndFakeDriverTablesAgree` (+ the presence pin, + the synthesis test) |
| 6.2 | `b7cf2d6` | key renamed `"FTDX10"`, drivers still `Model()=="FTdx10"` | `TestDriverTableKeysMatchDriverModel` (+ 4 more) |
| 6.3 | `b7cf2d6` | radiotext entry deleted | `TestEverySupportedModelHasRadiotext`, `TestRadiotext_FTdx10Verbatim` |
| 6.4 | `b7cf2d6` | **removed from BOTH tables** | **only `TestSupportedModels_ContainsEveryRegisteredModel` (the presence pin) + the guard's non-vacuity clause.** Four two-sided tests — `TestRealAndFakeDriverTablesAgree`, `TestDriverTableKeysMatchDriverModel`, `TestOpenFakeSessionFor_EveryRegisteredModel`, `TestEverySupportedModelHasRadiotext` — **ALL PASSED**. That is the exact blindness the pin exists for, and it is the most valuable single row in this index. |
| 6.5 | `b7cf2d6` | a second `ftdx10.Simulated` reference in a second non-test file | `TestSimulatedProfileTokensConfinement` — *"appears in 2 non-test files repo-wide … want exactly 1"* |
| **Task 7 — `490c38c`, caps-derived fields + honest imports: 3 proofs** | | | |
| 7.1 | `490c38c` | the unconditional `TagDisplay{Unknown}` constant restored in `chirp.go` | all three new csvio tests **plus** the app-level `TestGetUISpec_RegisteredFTdx10_EveryBankUnavailable` |
| 7.2 | `490c38c` | `bankCoreFields` pinned back to the fixed seven | all five D5a tests, **including the registered-FTdx10 leg** |
| 7.3 | `490c38c` | **either** D5b gate reverted alone (`isCellEditable` or `toggleCell`) | the Unknown→Off→On walk through the real component and the real edit queue — an EITHER-gate proof, not a both-gates one |

**Totals: 44 injected defects across seven commits, 43 fired, 1 recorded
as not firing with its reason and its test amended.** Every mutation was
reverted and the revert verified — tasks 5 and 6 additionally confirmed
byte-identical restoration by SHA-256 on every touched file
(`0d44f04b…` for the generated inventory; all three files at task 6).

---

## Part 5 — the full local gate at the tip

**All items run at `490c38c` in the repo working tree** (the base
worktree was used only for its build). Each line below is the actual
final line of the actual run, taken from the scrollback of THIS task —
which is the M9c-5 manifest's own hard-won lesson, restated: state
evidence PER COMMIT, from what was observed, never from intention.

| Gate item | Result |
|---|---|
| `gofmt -l .` | **PASS** — exit 0, no output |
| `go vet ./...` | **PASS** — exit 0, no output |
| `go test ./... -count=1` (whole tree, awaited) | **PASS** — see the package list below |
| `go test ./internal/guards/ -v -count=1` | **PASS** — `ok … 0.341s`, 9/9 `--- PASS` by name (below) |
| `go test -race ./core/... -count=1` (backgrounded first, collected last) | **PASS** — 11/11 packages `ok`, exit 0, 4 min 06 s wall |
| frontend `npm run check` | **PASS** — exit 0, `COMPLETED 189 FILES 0 ERRORS 0 WARNINGS 0 FILES_WITH_PROBLEMS` |
| frontend `npm test` | **PASS** — exit 0, `Test Files 16 passed (16)` / `Tests 401 passed (401)` |
| frontend `npm run build` | **PASS** — exit 0, `✓ built in 477ms` |
| `go generate ./internal/fakedx10/... ./core/cat/...` ×2 | **PASS** — idempotent, below |
| `wails generate module` ×2 (from `app/`) | **PASS** — idempotent and non-vacuous, below |
| four-path golden gate | **PASS** — exit 0, empty, before AND after regen |
| `core/cat/testdata/` commit count | **PASS** — exactly two (`ff5c19b`, `1d38941`) |
| `git status --short` | **PASS** — the single new fixture directory and nothing else |

`/opt/homebrew/bin/npm` (11.17.0) was used for all three frontend items,
run from `app/frontend`.

### The full Go suite, package by package

`go test ./... -count=1`, started `03:35:22Z`, finished `03:39:24Z`
(4 min 02 s), **exit 0**. Twenty-three test packages, every one `ok`,
plus one with no test files:

```
ok  	github.com/gm5dna/open-rig-programmer/app	108.247s
ok  	github.com/gm5dna/open-rig-programmer/cmd/rigprog	161.631s
ok  	github.com/gm5dna/open-rig-programmer/core/cat	1.047s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/dialecttest	1.727s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx10	0.335s
ok  	github.com/gm5dna/open-rig-programmer/core/clone	241.489s
ok  	github.com/gm5dna/open-rig-programmer/core/codeplug	2.742s
ok  	github.com/gm5dna/open-rig-programmer/core/csvio	1.488s
ok  	github.com/gm5dna/open-rig-programmer/core/driver	2.176s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ft710	57.519s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ftdx10	69.095s
ok  	github.com/gm5dna/open-rig-programmer/core/spec	2.519s
ok  	github.com/gm5dna/open-rig-programmer/core/transport	17.945s
ok  	github.com/gm5dna/open-rig-programmer/internal/buildinfo	2.070s
ok  	github.com/gm5dna/open-rig-programmer/internal/csvmerge	2.054s
ok  	github.com/gm5dna/open-rig-programmer/internal/extable	1.827s
?   	github.com/gm5dna/open-rig-programmer/internal/extable/gen	[no test files]
ok  	github.com/gm5dna/open-rig-programmer/internal/extable/observe	1.496s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakedx10	3.793s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakedx10/gen	1.138s
ok  	github.com/gm5dna/open-rig-programmer/internal/fakeradio	6.983s
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	1.353s
ok  	github.com/gm5dna/open-rig-programmer/internal/radiotext	1.139s
ok  	github.com/gm5dna/open-rig-programmer/internal/wiring	10.939s
```

**Twenty-three, against M9c-5's twenty. None dropped; three added, and
they are exactly this milestone's three new packages** —
`core/driver/ftdx10` (tasks 1-3), `internal/fakedx10` (task 4) and
`internal/fakedx10/gen` (task 5). The arithmetic closes with no
unexplained package on either side.

`internal/wiring` at 10.939 s is the cost task 6 flagged and did not
optimise: roughly 2.8 s per FTdx10 open, the budgeted full-range
discovery walk.

### The 9 guards, by name

All `--- PASS`, from `go test ./internal/guards/ -v -count=1`:

```
--- PASS: TestCompositionRootImportDiscipline (0.03s)
--- PASS: TestDialectPromotedDataIsNotAPackageGlobal (0.02s)
--- PASS: TestGateReachingValidatorsAreDialectMethods (0.02s)
--- PASS: TestTransitiveGlobalReachSetIsReported (0.02s)
--- PASS: TestDriverSeamPackageDoesNotImportCAT (0.02s)
--- PASS: TestNewEngineReachableOnlyFromDriver (0.02s)
--- PASS: TestWritePathReachableOnlyThroughDriver (0.02s)
--- PASS: TestRetiredWriteResultNamesAreGone (0.01s)
--- PASS: TestSimulatedProfileTokensConfinement (0.02s)
PASS
ok  	github.com/gm5dna/open-rig-programmer/internal/guards	0.341s
```

Nine, unchanged in number from M9c-5 — this milestone added a ROW to
`TestSimulatedProfileTokensConfinement`, deliberately not a tenth test,
so the FTdx10's Simulated profile is confined by exactly the check that
confines the FT-710's.

### `go test -race ./core/...`

Started FIRST, in the background, before any other work in this task
(`03:24:37Z`), and collected at the end (`03:28:43Z`) — 4 min 06 s.
Exit **0**. Every package:

```
ok  	github.com/gm5dna/open-rig-programmer/core/cat	1.596s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/dialecttest	1.851s
ok  	github.com/gm5dna/open-rig-programmer/core/cat/ftdx10	1.657s
ok  	github.com/gm5dna/open-rig-programmer/core/clone	244.493s
ok  	github.com/gm5dna/open-rig-programmer/core/codeplug	4.824s
ok  	github.com/gm5dna/open-rig-programmer/core/csvio	3.715s
ok  	github.com/gm5dna/open-rig-programmer/core/driver	3.462s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ft710	58.941s
ok  	github.com/gm5dna/open-rig-programmer/core/driver/ftdx10	69.659s
ok  	github.com/gm5dna/open-rig-programmer/core/spec	2.784s
ok  	github.com/gm5dna/open-rig-programmer/core/transport	19.000s
```

M9c-5's manifest closed by recording that it had run NO `-race` gate at
all. This one has: **eleven core packages, race detector on, zero
reports** — including `core/driver/ftdx10` and the transport-level
FTdx10 EX cross-check.

### Regeneration idempotence, ×2, non-vacuously

`go generate ./internal/fakedx10/... ./core/cat/...` run twice. Exit 0
each time; `git status --porcelain` **identical after both passes**
(`?? docs/superpowers/m9c6-fixtures/` — this commit's own new fixture,
nothing else); and all three generated inventories unchanged at every
step:

| Generated file | SHA-256 (before, after pass 1, after pass 2) |
|---|---|
| `internal/fakedx10/exinventory_gen.go` | `0d44f04bef5ece957fc324c8350cef9af5a9d6899b83639a2ad3bdff803dad48` |
| `core/cat/ftdx10/exinventory_gen.go` | `9311fc928b540110539d5dc40c921193b39890fbd6ef8c6f27c1e0db3c2171d4` |
| `core/cat/exinventory_gen.go` | `fbf4f02e3e564357eecd020f612de58cd3d26ad58a8a6e8ee9cb1249815ada22` |

The fakedx10 value matches the one task 5's commit recorded, which is
the staleness property holding across three further commits.

`wails generate module` (`~/go/bin/wails`) run twice from `app/`, exit 0
each time. **Non-vacuity measured by mtime**, because a generator that
silently did nothing would also produce a clean diff: the bindings were
last written at `04:03:35` (task 7), and the two passes rewrote them at
`04:36:22` and `04:36:23` respectively. After each pass,
`git diff --exit-code -- app/frontend/wailsjs` from the REPO ROOT
returned **exit 0** and `git status --porcelain` showed only the new
fixture directory. The files really were regenerated and really were
byte-identical.

### The golden corpora

Untouched. `core/cat/testdata/`, `core/cat/exinventory_gen.go`,
`core/cat/ftdx10/testdata/` and `core/cat/ftdx10/exinventory_gen.go` all
exit 0 under one `git diff --exit-code` invocation, both before and
after the regeneration passes. Extending the same check to
`internal/fakedx10/exinventory_gen.go` and
`internal/fakedx10/transcription-b.csv` also exits 0.
`core/cat/testdata/` is still at **exactly two commits**:

```
1d38941 m9b: task 51 fix round — close blind spots found by mutation review
ff5c19b m9b: task 51 — mint three evidence corpora before anything moves
```

**No golden was regenerated at any point in M9c-6.**

---

## The gate-at-final-code-tip invariant

Re-stated because it is the invariant a merge reviewer should check
rather than any particular hash, and stated the M9c-5-CORRECTED way —
**per commit, from observed evidence, never inherited**:

> The byte-identity capture is taken at the LAST CODE COMMIT of the
> branch. Every commit after the capture must be documentation-only for
> the capture to speak for the branch tip.

**The last CODE commit on `m9c6-ftdx10-registration` is `490c38c`**
(task 7). Every measurement in Parts 1, 2, 3 and 5 above was taken at
that commit, in that working tree, with binaries built from it.

**This manifest's own commit is DOCUMENTATION-ONLY** in the invariant's
sense. It tracks exactly two paths:

| Path | Kind |
|---|---|
| `docs/superpowers/m9c6-baseline-manifest.md` | this manifest |
| `docs/superpowers/m9c6-fixtures/chirp_skip.csv` | a test INPUT, read by no production code path |

No `.go` file, no frontend file, no generated binding and no golden is
touched by it — `git show --stat` of this commit is the whole of its
contents, and it is the check to run rather than to take on trust. The
capture therefore speaks for the branch tip **as of this commit**.

If a milestone-review wave lands after this one — as M9c-5's did — this
manifest does NOT retroactively cover it. M9c-5's own close-out is the
worked example: its capture was taken at `f64f688`, task 11's manifest
commit `80f95ee` was documentation-only and so covered by it, but the
later review commits `bc3b6f1` and `8721a91` changed production frontend
code, Go comments and tests, and were covered instead by their OWN full
gate runs, re-run whole at each commit and recorded per commit. Any
M9c-6 review wave must do the same: re-run, re-record, per commit. This
manifest's claim is scoped to `78b73ac` → `490c38c` plus its own
documentation-only commit, and to nothing beyond.

The handoff and planning documents that accompanied this work are
deliberately NOT in this commit: `.superpowers/` is gitignored (see
`.gitignore`, "Superpowers scratch state"), as is
`docs/fixtures-private/`, so `HANDOFF-m9c.md`, `m9c6-spec.md` and
`m9c6-plan.md` are ON-DISK working files that no commit on this branch
contains. Only tracked files can affect whether a post-capture commit is
documentation-only.

---

## Scope of the claim this manifest supports

It supports: **across the whole of M9c-6, from `78b73ac` to `490c38c`,
the FT-710's probe, read, CHIRP-import (both a blocked and a clean one)
and native CSV-export paths are byte-identical — stdout, stderr and exit
code alike, by `cmp` with no normalisation beyond the one declared
`read_at` noise field — with NO carve-outs of any kind; the only
observable change anywhere in the CLI is one line of the top-usage text
and its `UnknownModelError` twin, both interpolating a registry-driven
model list by task-40 design and both recorded here verbatim; and the
FTdx10 is a registered, selectable model whose probe, read, settings
read, diff, settings render, export, CHIRP import and native CSV round
trip all work end to end against `internal/fakedx10`, at exit 0, with
its first baselines recorded.**

It does **not** support a claim about: the write path to a real radio of
either model (no radio is opened anywhere in this recipe;
`writeTrialsComplete=false` is pinned for the FTdx10 and the RealHardware
profile refuses before a frame is built); the FTdx10's correctness
against real hardware in any respect — every FTdx10 value above is the
FAKE's answer, and the fake's EX values are invented by a documented
convention; the `ports` or real-`--port` paths; the GUI at runtime (the
frontend is typechecked, unit-tested and built, not driven); or the
sendability of a CHIRP-imported channel on either radio, which Part 3's
A7 records as blocked-by-design on a pre-existing, model-independent
gate this milestone did not touch.

# M9c-0 baseline manifest

Captured **26/07/2026**, before any source change for the M9c-0 exported
`Dialect` constructor.

This file is the FT-710 byte-identity reference for the milestone. Task 69
re-captures every artefact below and compares. **A difference that is not a
declared noise field is a defect, never a baseline to update.**

- **BASE commit:** `d327ac44da483e2bbc122f9e885c57673b3195d4`
- **Branch:** `m9c0-dialect-constructor`
- **Toolchain:** `go1.26.5 darwin/arm64`
- **Artefacts live at** `.superpowers/sdd/m9c0-baselines/` — git-ignored on
  purpose. This manifest is the tracked, durable record; the M9b review
  found the artefacts alone had no provenance, which is why it exists.

## Reproduction

**This sequence has been run.** Revision 1 of the plan carried a different
one that did not work — `settings` has no `--fake` flag and takes a file
argument (`cmd/rigprog/settings.go:32`), `read` uses `--out` not `-o`
(`cmd/rigprog/read.go:115`), and `export` uses `--csv OUT FILE`
(`cmd/rigprog/export.go:23`).

```bash
set -e                                   # without this a failed command
                                         # leaves an empty file to be hashed
B=.superpowers/sdd/m9c0-baselines
mkdir -p "$B"
go run ./cmd/rigprog probe --fake                                     > "$B/probe-fake.txt" 2>&1
go run ./cmd/rigprog read --fake --settings --out "$B/read-fake.json" > "$B/read-fake.txt"  2>&1
go run ./cmd/rigprog settings "$B/read-fake.json"                     > "$B/settings.txt"   2>&1
go run ./cmd/rigprog export --csv "$B/export.csv" "$B/read-fake.json" > "$B/export.txt"     2>&1
go run ./cmd/rigprog help                                             > "$B/help.txt"       2>&1
git rev-parse HEAD > "$B/BASELINE-COMMIT"

find "$B" -size 0 -print                 # MUST print nothing
cd "$B" && shasum -a 256 *
```

`$B` must be the **relative** path shown: `read-fake.txt` and `export.txt`
echo the output path they were given, so an absolute path changes their
bytes. That is a declared noise field below, but keeping the path relative
means only one thing needs normalising rather than two.

## Artefact hashes (raw)

| SHA-256 | Artefact | Bytes |
|---|---|---|
| `6b048bbd7c0c974eeeae58a9097ac7579814dcfff674021f34d4032bf784477f` | `BASELINE-COMMIT` | 41 |
| `6d741c2ba228ce1fda134791b944dc9fe8a95d5c0832866e7974712c8754d88f` | `export.csv` | 2,813 |
| `3a8b7997858ae0988683168eb3f9f2c8f9669849a26283d9773e12ef8f4ab9e4` | `export.txt` | 75 |
| `4baf26a6d06a11e1784bfe35b04ef89da906352fa8137942e7e7b0558a019f4d` | `help.txt` | 915 |
| `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` | `probe-fake.txt` | 284 |
| `d2e4361218f2c4f36e759bf254fee81f265c3b7d650d4999a3bae232c5798e73` | `read-fake.json` | 35,577 |
| `7cdea602052d01d21be23db0f30f1d220890998f215264fdc79daa0750956532` | `read-fake.txt` | 10,677 |
| `7a41ef1af537d917252cf2ff56cacbf095488a71d828d15a0dec53efaef9a83c` | `settings.txt` | 13,388 |

**Four must match with NO normalisation at all:** `probe-fake.txt`,
`settings.txt`, `export.csv`, `help.txt`. `export.csv` is the load-bearing
one — it renders every channel's mode string, so a single changed character
in mode rendering shows there.

## Declared noise fields, and their normalised hashes

Three artefacts contain a value that legitimately varies between runs.
**Only these three, and only these fields.** Schema, baseline digest and
every other JSON field stay inside the comparison — they are exactly what a
mistake would change.

| Artefact | Noise | Normalisation | Normalised SHA-256 |
|---|---|---|---|
| `read-fake.json` | `read_at` timestamp | `sed 's/"read_at": "[^"]*"/"read_at": "NORMALISED"/'` | `d2c40ad54a1a00790f4c4c679acd12c9ee6a56c89674b01c6b95de14face5476` |
| `read-fake.txt` | echoed `Output:` path | `sed 's\|^\(Output: *\).*\|\1NORMALISED\|'` | `a352f8a537eb0a2cf9d705089044e632b563f5db330c40ea0f669626f9dc6360` |
| `export.txt` | echoed `Output:` path | same as above | `048fee3ffa06b26671c2110081f6b808bb78fbff36cac1e1f36dc1a584e08f44` |

**Each normalised hash was verified to DIFFER from its raw hash.** That
check is not ceremony: the first attempt at the `read_at` rule matched
nothing — the JSON is pretty-printed as `"read_at": "…"` with a space,
not the compact `"read_at":"…"` the pattern assumed — and produced a
"normalised" hash identical to the raw one. A normalisation that silently
does nothing would let a real timestamp difference read as a real defect,
or worse, let the rule be quietly widened later to hide one.

## Golden corpora

Unchanged since Task 51 of M9b and **never to be regenerated**. If one
fails, the change is wrong, not the corpus.

| SHA-256 | Corpus |
|---|---|
| `c41846d255adcb7de546c5bc4cbadbb3ad523bf68a3ebae16c1fe06574bab237` | `allowlist-corpus.golden` |
| `0371fcb6a020ff3921744216a3863fe374c0429f161f986ebc2516c888a4967f` | `evidence-literals.golden` |
| `37c343d52078132543dfa22fd0b00b4ab3ae07ee3023b7cc32738f12def18469` | `frame-corpus.golden` |
| `3376f719fb31f8712b0c4b23d2aa2098580bc66bf1b2f77e162f5ebf3681873d` | `parser-corpus.golden` |

`git log --oneline -- core/cat/testdata/` must continue to show exactly two
commits — `ff5c19b` (the mint) and `1d38941` (its one sanctioned fix round).
A third is a regenerated golden and a milestone failure.

## Independent cross-check

`read-fake.txt` reports `Baseline digest: ccdf39f3d793`, which is the value
M9b's own summary recorded. FT-710's rendered output has therefore not
drifted between the two milestones, checked against a figure derived
independently of this capture.

## Scope of the claim these hashes support

They support: **the FT-710's read and render paths are byte-identical
across M9c-0.** Everything captured here is a read or render path against
`--fake`.

They do not support a claim about the write path. No CLI artefact exercises
a write, and no hardware runs. Write-path frame bytes are covered by a
*different* instrument — `frame-corpus.golden`'s records, minted before
anything moved and never regenerated — so a changed write frame fails that
corpus rather than these diffs.

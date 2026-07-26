<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# M9b baseline manifest — durable provenance for the CLI baselines

**Baseline commit:** `bed4544ffa93ce1183114f302e7892320eee63b7` (`bed4544`,
the `main` commit M9b branched from)
**Captured:** 25/07/2026, at Task 51, before any M9b code moved
**Artefacts:** `.superpowers/sdd/m9b-baselines/` — **git-ignored**, which
is why this file exists
**Reproduced and verified:** 26/07/2026, at the M9b fix wave (Codex
milestone review, finding 4)

## Why this file is tracked and the artefacts are not

M9b's headline claim is that the FT-710's read and render paths are
byte-identical across the milestone. The evidence is a set of CLI captures
taken at `bed4544` and re-run at branch HEAD. Those captures, the task
reports comparing them, and the `BASELINE-COMMIT` marker all live under
`.superpowers/`, which `.gitignore` excludes in full — so the branch itself
carried **no durable proof that those exact bytes came from `bed4544`**.
An external review confirmed the comparison's normalisation was honest and
then made exactly that point: the provenance was unwitnessed.

This manifest is the witness. It is tracked, it names the commit, it gives
the commands verbatim, and it records the SHA-256 of every byte captured —
so anyone can re-mint the artefacts from `bed4544` and check them against
this file without trusting a local directory or a report's summary of it.

## Reproduce

From a clean checkout of the baseline commit. A worktree is the least
disruptive way and is what the verification below actually used:

```bash
git worktree add /tmp/m9b-baseline bed4544
cd /tmp/m9b-baseline
mkdir -p .superpowers/sdd/m9b-baselines
B=.superpowers/sdd/m9b-baselines
go run ./cmd/rigprog probe --fake > "$B/probe-fake.txt" 2>&1
go run ./cmd/rigprog read --fake --settings --out "$B/read-fake.json" > "$B/read-fake.txt" 2>&1
go run ./cmd/rigprog settings "$B/read-fake.json" > "$B/settings.txt" 2>&1
go run ./cmd/rigprog export --csv "$B/export.csv" "$B/read-fake.json" > "$B/export.txt" 2>&1
go run ./cmd/rigprog help > "$B/help.txt" 2>&1
git rev-parse HEAD > "$B/BASELINE-COMMIT"
cd "$B" && shasum -a 256 *
```

Two details are load-bearing and not incidental:

- **`$B` must be the relative path `.superpowers/sdd/m9b-baselines`.**
  `read` and `export` echo the caller's `--out`/`--csv` argument in an
  `Output:` line, so `read-fake.txt` and `export.txt` contain that path
  verbatim. Run with an absolute or different relative path and those two
  hashes will not match — the *bytes* changed, not the behaviour.
- **`--fake` throughout.** No hardware ran and none is needed; every
  artefact here is a read or render path against the simulator. Nothing in
  this set performs a write. (The write path is pinned separately, by
  `core/cat/testdata/frame-corpus.golden`.)

## The artefacts

SHA-256 over the exact bytes captured at Task 51 and still on disk at the
fix wave:

| Artefact | Bytes | SHA-256 |
| --- | ---: | --- |
| `BASELINE-COMMIT` | 41 | `6197d87e87e3a76be4a9f09099afc3b2fc1b6a8ef5c82e3f6807d660fe916814` |
| `probe-fake.txt` | 284 | `ad4bf761119c77bf91a1726af5ec15e76cc9af5346a4d4642ab86a95e70b6a71` |
| `read-fake.txt` | 10,676 | `3f0b2f18b68981ade98df0b4d2d11f5e0048ada98ccad989a01ae5267ce8dca8` |
| `read-fake.json` | 35,577 | `b37fb33b874d5c61b9eeb4c8e7a6ad2d289b8a59dfadb879252009c4a881f69a` |
| `settings.txt` | 13,388 | `7a41ef1af537d917252cf2ff56cacbf095488a71d828d15a0dec53efaef9a83c` |
| `export.csv` | 2,813 | `6d741c2ba228ce1fda134791b944dc9fe8a95d5c0832866e7974712c8754d88f` |
| `export.txt` | 74 | `4c835a1a3e481df0c1d35d5e27fe153fd8bcb14ccad51ed33256979a8affccba` |
| `help.txt` | 915 | `4baf26a6d06a11e1784bfe35b04ef89da906352fa8137942e7e7b0558a019f4d` |

**One of the eight is not reproducible byte-for-byte, by construction.**
`read-fake.json` carries a `read_at` capture timestamp, so its raw hash is
a function of when it was taken. For that artefact the reproducible value
is the hash with the timestamp line removed:

| Artefact | Normalisation | SHA-256 |
| --- | --- | --- |
| `read-fake.json` | `grep -v '"read_at"'` | `82d281d944a1fc10111c5d0fb9dc2b988b9439637e752be4247466a66157b756` |

That is the same noise floor the milestone's own comparison declares — the
`read_at` timestamp and the echoed `Output:` path, both established at Task
56 as differing run to run independent of any code change. It is stated
here as a limit of reproducibility, not as a waiver: the codeplug body, the
`schema: 2` field and the `baseline_digest`
`ccdf39f3d7939b9fc1167d91d6f13899b9959bdd16028417c0e1f69a9be8424f` are all
inside the normalised hash.

## Verification actually performed

Not asserted from the recipe — run. On 26/07/2026, on the fix-wave branch,
Go 1.26.5 (darwin/arm64):

1. A git worktree was created at `bed4544` and confirmed with
   `git rev-parse HEAD` → `bed4544ffa93ce1183114f302e7892320eee63b7`.
2. The six capture commands above were re-run there, with `$B` as the
   relative path.
3. Every artefact was hashed and compared against the table above.

**Result: seven of the eight raw hashes matched exactly** —
`BASELINE-COMMIT`, `probe-fake.txt`, `read-fake.txt`, `settings.txt`,
`export.csv`, `export.txt`, `help.txt`.

**`read-fake.json` did not, and was expected not to.** The full diff
against the Task 51 capture is **one hunk, one changed line**:

```diff
-    "read_at": "2026-07-25T23:26:52.353678+01:00",
+    "read_at": "2026-07-26T06:52:19.602726+01:00",
```

Both files are 35,577 bytes and their normalised hashes are identical
(`82d281d9…57b756`), so the 35,577 bytes minus that one timestamp are the
same bytes. This is the declared noise floor behaving exactly as declared,
and is recorded as a mismatch rather than smoothed away.

**Nothing in this manifest was adjusted to make a hash agree.** The table
records what was captured at Task 51; the verification records what came
back at the fix wave; where they differ, the difference is shown.

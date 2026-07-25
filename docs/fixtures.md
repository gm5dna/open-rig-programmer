# Fixture redaction policy

Raw radio backups and serial protocol transcripts captured from a real
FT-710 contain personal data: the owner's callsign, and whatever
frequencies, memory labels, and channel contents happen to be programmed
into that radio at capture time. This data must not end up in a public
git history.

## Where raw captures live

Raw, unredacted captures (`.MemList` exports, `MEMORY*.dat` backups, raw
serial transcripts, etc.) live only in `docs/fixtures-private/`. That
directory is git-ignored (see `.gitignore`) and must never be committed.

Current contents include, from the M8c menu/EX read-characterisation
session (24/07/2026):

| File | Contents |
| --- | --- |
| `m8c-settings-2026-07-24-run1.json` | First full `read --settings` capture: every channel plus all 296 menu values, including `MY CALL` and the five `PRESET NAME` strings |
| `m8c-settings-2026-07-24-run2.json` | Second sweep, byte-identical to the first — the repeat-read comparison |
| `m8c-run1.private-capture`, `m8c-run2.private-capture` | Per-item progress logs for those two reads |
| `m8c-exprobe.private-capture` | Scratch probe transcript: out-of-inventory rejections and the latency loop |

The artefacts those captures produced are committed:
`core/cat/table2-observed.csv`, which carries addresses, wire widths and
shape classes and no values at all, and `core/cat/table2-corrections.csv`,
which additionally quotes the two values the operator explicitly consented
to publish. Regenerating the
observation CSV needs a private capture that is not in the repository;
`internal/extable/observe` is the tool, and its own tests prove it cannot
emit a captured value.

## What may be committed

Anything committed under `docs/fixtures/` (or used as a test fixture
elsewhere in the repo) must be redacted first:

- Callsigns are replaced with the placeholder `MYCALL`.
- Personal frequencies are swapped for generic band-plan examples (e.g.
  well-known repeater or beacon frequencies, or clearly fictitious
  round-number values) rather than the operator's actual programmed
  channels.
- Any other operator-identifying free text (memory labels, notes) is
  replaced with generic placeholders.

## Before the repository goes public

Because this repo starts private, it is easy to forget this policy while
iterating quickly. Before flipping the repository to public, do an
explicit audit of every committed fixture file to confirm it contains no
real callsign, no real personal frequency list, and no other
identifying data.

This audit must cover **all reachable git history and objects, not just
the current HEAD**. A file that was committed and later deleted, or a raw
fixture that was force-pushed over, is still present in the repository's
object database and remains reachable (and cloneable) unless it has been
removed from every ref, the reflog has expired, and the objects have been
garbage-collected. Check the full history (e.g. `git log --all --diff-filter=A -- '<pattern>'`,
or scan every blob reachable from every ref) rather than only the working
tree or the latest commit.

If a raw, unredacted fixture is found anywhere in history, deleting it
from the working tree or a later commit is **not sufficient**: the
history must be rewritten (e.g. with `git filter-repo`) to remove the
object from every commit that references it, or the repository must be
recreated from a clean history. Either way, verify afterwards that the
object is genuinely unreachable — it is gone from every branch and tag,
the reflog no longer references it, and `git gc --prune=now` (or the
hosting provider's equivalent) has actually pruned it — before treating
the exposure as resolved.

## Pre-public audit — carried out 25/07/2026

The audit above was performed against the whole object database before
the public flip. Recorded here so it is not repeated from scratch, and
so the conclusion is not mistaken for an oversight.

**Working tree: clean.** No tracked file matches any private-fixture
pattern (this is also enforced on every CI run — see the "Guard against
committed private fixtures" step in `.github/workflows/ci.yml`), and no
tracked file carries a callsign, device identifier or other personal
datum. The project owner's own identity appears deliberately, in the Go
module path (`github.com/gm5dna/open-rig-programmer`) and the macOS
bundle identifier — a public GitHub account name, published knowingly.

**History: scanned blob by blob across every ref.** Three findings, and
nothing else:

| Found | Extent | Disposition |
| --- | --- | --- |
| `01600D4F`, the USB-serial adapter's device identifier, embedded in a macOS port node name | **One blob**: `docs/hardware-notes.md` at commit 9e17539. Redacted in the very next commit (9ae7d7a) | Accepted. It identifies a USB adapter, not a person; the exposure is one line of one superseded revision of one file |
| `GB3TST`, in two `cmd/rigprog` test files | ~16 commits, replaced by `MYCALL` at 10b4877 | Not personal data: an invented test string. It was changed for consistency with this document's `MYCALL` convention (Codex M4 finding #8, LOW), not for privacy |
| The owner's callsign, in `core/cat/ex_test.go` | ~68 commits, replaced with a synthetic vector during M8c | Accepted deliberately (owner's decision, 25/07/2026): the module path and bundle identifier carry the same identity publicly, so a history rewrite would hide nothing |

**No raw capture ever entered history.** No `fixtures-private/` path,
`.MemList`, `MEMORY*.dat` or `.private-capture` object exists in any
reachable commit — the case this document's rewrite guidance above was
written for did not arise. **No history rewrite was performed, and none
is planned**; the commit SHAs cited throughout `docs/` and the guard
pins remain valid.

If a future session finds something this audit missed, the rewrite
guidance above still stands — it is the procedure, and the fact that it
was not needed once does not retire it.

## Redacted-fixture convention

Redacted fixtures that are safe to commit live under `docs/fixtures/`
and must carry a `.redacted.` infix immediately before the file
extension, e.g. `read-all.redacted.txt`. This naming convention exists
so that a redacted, committable fixture can never collide with (or be
mistaken for) the ignored raw-capture patterns in `.gitignore`
(`**/fixtures-private/`, `*.MemList`, `MEMORY*.dat`, `*.private-capture`):
none of those patterns can ever match a `*.redacted.*` filename, and the
`.redacted.` infix makes it visually obvious in a diff or directory
listing that the file has already been through the redaction process
described above.

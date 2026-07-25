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

## Pre-public audit and history reset — 25/07/2026

The audit described above was carried out against the whole object
database of the project's pre-publication development repository, and
this repository's history was then reset. Recorded here so neither is
repeated from scratch, and so the single-commit history below is not
mistaken for carelessness.

**This repository begins at a squashed initial commit.** The project
was developed privately over several months; that history was collapsed
into one commit before publication so that the author's personal e-mail
address, which git records on every commit as author and committer
metadata, does not appear in the published record. No change to a
tracked file can remove an address from commit metadata — only a rewrite
can — and the development history is retained privately by the project
owner rather than destroyed.

Because the reset produced a brand-new repository rather than a
force-push over the old one, no unreachable objects from the previous
history survive here to be fetched by direct SHA. That was deliberate:
whether a hosting provider has actually pruned unreachable objects is
not verifiable from outside.

**What the audit found before the reset.** Scanned blob by blob across
every ref of the development repository. Three findings, and nothing
else — none of which reaches this repository, since none is in the tree:

| Found | Extent in the old history | Disposition |
| --- | --- | --- |
| `01600D4F`, the USB-serial adapter's device identifier, embedded in a macOS port node name | One blob, in an early revision of `docs/hardware-notes.md`; redacted in the very next commit | Identifies a USB adapter, not a person. Superseded by the reset in any case |
| `GB3TST`, in two `cmd/rigprog` test files | ~16 commits, replaced by `MYCALL` partway through | Not personal data: an invented test string. It was changed for consistency with the `MYCALL` convention above, not for privacy |
| The owner's callsign, in `core/cat/ex_test.go` | ~68 commits, replaced with a synthetic vector later | Accepted deliberately: the module path and bundle identifier carry the same identity publicly |

**No raw capture ever entered the history.** No `fixtures-private/`
path, `.MemList`, `MEMORY*.dat` or `.private-capture` object existed in
any reachable commit — the case the rewrite guidance above was written
for did not arise, and the reset was done for the e-mail metadata, not
because a fixture had leaked.

**Current state of the tree**, re-verified after the reset: no tracked
file carries a callsign, device identifier, or e-mail address. The
owner's name appears where he is credited as the author of a decision
(`docs/menu-write-decision.md`) and as the git author, both by choice.
The guidance above still stands for any future finding; the fact that
it was not needed for a fixture leak once does not retire it.

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

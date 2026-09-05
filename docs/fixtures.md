# Fixture redaction policy

Raw radio backups and serial protocol transcripts captured from a real
radio contain personal data: the owner's callsign, and whatever
frequencies, memory labels and channel contents happen to be programmed
into that radio at capture time. This data must not end up in a public
git history. This page is the standing policy; the one audit carried
out under it is recorded in [fixtures-history.md](fixtures-history.md).

## Where raw captures live

Raw, unredacted captures (`.MemList` exports, `MEMORY*.dat` backups, raw
serial transcripts, `read --settings` output and the like) live only in
`docs/fixtures-private/`. That directory is git-ignored (see
`.gitignore`) and must never be committed. CI and the versioned
pre-push hook (`scripts/git-hooks/pre-push`) both refuse any tracked
path that matches the raw-capture patterns.

The artefacts derived from those captures are committed only when they
carry no values: `core/cat/table2-observed.csv` carries addresses, wire
widths and shape classes and nothing else, and
`core/cat/table2-corrections.csv` additionally quotes the two values the
operator explicitly consented to publish. Regenerating the observation
CSV needs a private capture that is not in the repository;
`internal/extable/observe` is the tool, and its own tests prove it
cannot emit a captured value.

## What may be committed

Anything committed under `docs/fixtures/`, or used as a test fixture
elsewhere in the repository, must be redacted first:

- Callsigns are replaced with the placeholder `MYCALL`.
- Personal frequencies are swapped for generic band-plan examples (well-
  known repeater or beacon frequencies, or clearly fictitious
  round-number values) rather than the operator's actual programmed
  channels.
- Any other operator-identifying free text (memory labels, notes) is
  replaced with generic placeholders.

## Redacted-fixture convention

Redacted fixtures that are safe to commit live under `docs/fixtures/`
and carry a `.redacted.` infix immediately before the file extension,
for example `read-all.redacted.txt`. None of the ignored raw-capture
patterns in `.gitignore` (`**/fixtures-private/`, `*.MemList`,
`MEMORY*.dat`, `*.private-capture`) can ever match a `*.redacted.*`
filename, and the infix makes it obvious in a diff or a directory
listing that the file has been through the redaction process above.

## If a raw fixture is ever found in history

Deleting it from the working tree or in a later commit is **not
sufficient**. A file that was committed and later deleted, or a raw
fixture that was force-pushed over, is still present in the
repository's object database and remains reachable (and cloneable)
until it has been removed from every ref, the reflog has expired and
the objects have been garbage-collected.

1. Check the full history, not only the working tree or the latest
   commit: `git log --all --diff-filter=A -- '<pattern>'`, or scan every
   blob reachable from every ref.
2. Rewrite the history (for example with `git filter-repo`) to remove
   the object from every commit that references it, or recreate the
   repository from a clean history.
3. Verify afterwards that the object is genuinely unreachable: gone from
   every branch and tag, no longer referenced by the reflog, and pruned
   by `git gc --prune=now` or the hosting provider's equivalent.

Only then is the exposure resolved.

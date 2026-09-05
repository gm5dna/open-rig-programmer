# Fixture audit history

The record of the one audit carried out under
[fixtures.md](fixtures.md). Kept so that neither the audit nor the
history reset is repeated from scratch, and so the repository's
squashed initial commit is not mistaken for carelessness.

## Pre-public audit and history reset, 25/07/2026

The audit the policy describes was carried out against the whole object
database of the project's pre-publication development repository, and
this repository's history was then reset.

**This repository begins at a squashed initial commit.** The project
was developed privately over several months; that history was collapsed
into one commit before publication so that the author's personal e-mail
address, which git records on every commit as author and committer
metadata, does not appear in the published record. No change to a
tracked file can remove an address from commit metadata (only a rewrite
can), and the development history is retained privately by the project
owner rather than destroyed.

Because the reset produced a brand-new repository rather than a
force-push over the old one, no unreachable objects from the previous
history survive here to be fetched by direct SHA. That was deliberate:
whether a hosting provider has actually pruned unreachable objects is
not verifiable from outside.

**What the audit found before the reset.** Scanned blob by blob across
every ref of the development repository. Three findings, and nothing
else, none of which reaches this repository, since none is in the tree:

| Found | Extent in the old history | Disposition |
| --- | --- | --- |
| `01600D4F`, the USB-serial adapter's device identifier, embedded in a macOS port node name | One blob, in an early revision of `docs/hardware-notes.md`; redacted in the very next commit | Identifies a USB adapter, not a person. Superseded by the reset in any case |
| `GB3TST`, in two `cmd/rigprog` test files | About 16 commits, replaced by `MYCALL` partway through | Not personal data: an invented test string. It was changed for consistency with the `MYCALL` convention, not for privacy |
| The owner's callsign, in `core/cat/ex_test.go` | About 68 commits, replaced with a synthetic vector later | Accepted deliberately: the module path and bundle identifier carry the same identity publicly |

**No raw capture ever entered the history.** No `fixtures-private/`
path, `.MemList`, `MEMORY*.dat` or `.private-capture` object existed in
any reachable commit. The case the rewrite guidance was written for did
not arise; the reset was done for the e-mail metadata, not because a
fixture had leaked.

**State of the tree after the reset**: no tracked file carries a
callsign, device identifier or e-mail address. The owner's name appears
where he is credited as the author of a decision
(`docs/menu-write-decision.md`) and as the git author, both by choice.

## Private captures on record

The raw captures the committed evidence was derived from, all in the
git-ignored `docs/fixtures-private/`:

| File | Session | Contents |
| --- | --- | --- |
| `m5a-transcript.private-capture`, `m5b-trials.private-capture` | M5a and M5b, 13/07/2026 | The read-characterisation and write-trial transcripts behind `docs/hardware-notes.md` |
| `m8c-settings-2026-07-24-run1.json`, `-run2.json` | M8c, 24/07/2026 | Two full `read --settings` captures, byte-identical, including `MY CALL` and the five `PRESET NAME` strings |
| `m8c-run1.private-capture`, `m8c-run2.private-capture` | M8c | Per-item progress logs for those two reads |
| `m8c-exprobe.private-capture` | M8c | Scratch probe transcript: out-of-inventory rejections and the latency loop |

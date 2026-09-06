# `ts480-a4-observation.json` — the TS-480's evidence artefact

This directory holds one file, and today it holds none: `ts480-a4-observation.json`
does not exist, and that absence is what keeps the TS-480 out of this program's
model list.

## What it is for

The TS-480 driver is written, tested and faked, and it is **not registered**. The
reason is assumption A4: whether an `MR` of an unwritten channel *answers* — with
P4 to P15 zero — rather than rejecting is documented for the TS-590SG and printed
nowhere at all in the 2003 TS-480 book. Registering the radio on the wrong reading
would mean reading channels wrongly and writing channels nobody could check.

A4 lifts on one thing only: a record of what a real TS-480 actually answered. That
record is this file. While it is absent the row stays out; when it is present and
meets the bar, the row must be in. `internal/wiring/ts480gate_test.go` asserts both
directions, and it is the guard that decides — not a release-day judgement.

## The bar

A valid zero record on **at least three separate unwritten channels**, in **at
least two sessions**, with **no silence and no `?;`** among them. One silent trial
proves nothing and must not be recorded as a pass; the manual says the `?;` may be
suppressed altogether, so silence is inconclusive rather than absence.

## Its shape

The schema is the `ts480Observation` type in `../ts480gate_test.go` and the field
names are its json tags; the bar is `checkTS480ObservationBar` in the same file.
Read them — they are short, and they are deliberately the only copy, so there is
nothing here to drift out of date.

Two things that file cannot tell you, and this one can:

- **Every byte field is the wire, verbatim.** It is a wire observation and not a
  transcription. The guard re-derives the request this program would have sent and
  re-parses the answer through the TS-480's own codec, so a plausible frame typed
  from memory is refused rather than accepted.
- **Do not commit a file you did not observe.** A guard that can be satisfied by
  writing a file is no gate at all, which is why the guard has a non-vacuity leg —
  but the leg checks shape, and only the person at the radio can check truth.

The file is tracked in git on purpose. It is not under `docs/superpowers/` or
`docs/fixtures-private/`, both of which are gitignored: an artefact there would be
missing from a fresh clone and from CI, and the guard would read "no evidence" on a
machine that had simply never been handed the file. This one is present or absent
for everybody at once.

## Registering the row is ten edits, not one

Adding this file is the first. The other nine are listed in the milestone plan's
decision P3, and nine of the ten now fail loudly in the test suite the moment the
row appears without them; the tenth shows up only at the byte-identity gate.

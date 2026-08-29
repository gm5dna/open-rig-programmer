# IC-7760 fake provenance

The independent fake derives its record from the frozen B/W artefacts:
`IC-7760-transcription-b.csv` and `IC-7760-geometry-witness.csv`. Their 27
printed bytes less the two selector bytes gives 25 record bytes; the name is
ten bytes. Selectors and `1A 00` are copied from those artefacts. `FB`/`FA`
are the documented ACK/NG codes.

Assumptions are explicit: `ic7760-id-reply` uses a synthetic configurable
token; `ic7760-empty-reply-fa` makes unset slots NG; `ic7760-empty-reply-ff`
refuses the printed all-FF clear form; `ic7760-echo-default` reflects before
address filtering; `ic7760-broadcast-form` uses `to=00`; and
`ic7760-scan-edge-record-shape` applies the 25-byte shape to P1/P2. Erase is
not admitted. Short sets and wrong sibling/length forms return NG.

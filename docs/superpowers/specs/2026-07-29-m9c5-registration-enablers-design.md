# M9c-5 — the registration enablers

**Date:** 29/07/2026
**Status:** revision 2, review findings folded
**Milestone:** M9c-5, the neutral-core enablers a registered second model
requires. M9c-6 (driver, fake, registration — the last M9c milestone)
follows.

> **Revision 2 (29/07/2026).** Revision 1 was reviewed adversarially by
> Codex (NEEDS-REVISION: 1 CRITICAL, 5 HIGH) and Fable (NEEDS-REVISION:
> 1 CRITICAL, 3 HIGH); adjudication in
> `.superpowers/sdd/m9c5-spec-review-adjudication.md`. Both found the
> same CRITICAL: revision 1 never decided what an FT-710 send of a
> non-Known TagDisplay does, while its own CHIRP change creates exactly
> that state. Codex additionally proved revision 1's E3 premise FALSE
> (the "foreign gate refuses the init frame" defect cannot occur — every
> dialect builds and admits the same "AI0;"), and both showed the
> byte-identity carve-out contradicted the spec's own sanctions, the
> migration broke schema-1 loads, and `driver.WriteResult` falsified the
> milestone claim if left. The reviewers split on absent-field migration
> semantics; the adjudication records the resolution. Six enablers now.
> **Do not implement revision 1.**

## Decomposition decision, recorded

As revision 1, plus the honest correction: with E6 added, M9c-5's claim
— "the neutral core no longer assumes the FT-710 anywhere a second
REGISTERED model would notice" — is again true; without E6 it was false
(`WriteResult`'s field names are FT-710 choreography).

## The six enablers

### E1 — `TagDisplay` becomes an honest three-state field

`ChannelData.TagDisplay` becomes `codeplug.BoolField` (the existing
idiom; `Unavailable` for a radio whose frame has no display flag — the
first real producer of that state, said plainly).

**The send-path decision (the CRITICAL, resolved): a mandatory-wire
field that is not Known BLOCKS ITS CHANNEL AT PLAN TIME.** `Diff` blocks
a channel whose `TagDisplay.State != Known` when the target's
`FieldTagDisplay.Write != Unsupported`, with a named BlockReason
("tag display unknown — set On or Off before sending"). Blocked
channels are excluded per-channel; a multi-slot send proceeds for the
rest — refuse-early, never mid-`Execute`. Defence in depth behind it:
`codeplug.Validate` and the driver's own gate validate
`TagDisplay.Valid()` and the driver refuses a non-Known display before
`BuildMTSet` (unreachable in practice; belt and braces — without the
Valid() checks, `{Unknown, true}` could carry a value to the wire).
The GUI's blank-row factory defaults per-model: FT-710-created rows are
`{Known, false}`, editable.

**Migration (both schemas; the reviewers' split, adjudicated):**
`CurrentSchema` 2 → 3. Frozen legacy channel decode structs for v1 AND
v2 (`loadV1` breaks otherwise — both reviewers). Rule, both versions: a
PRESENT bool (true or false — present-false was unspecified in rev 1)
→ `{Known, value}`; ABSENT → `{Known, false}`, justified as **strict
behaviour-preservation, not provenance**: a legacy file's absent field
would have been sent as false before this change and sends the same
byte after. The caveat is recorded: CHIRP-sourced schema-2 channels
carry a manufactured false that this baptises as Known-false — it was
already being sent as false, so no behaviour worsens, and post-E1 CHIRP
imports produce honest `Unknown` (which the diff then blocks until the
user decides — the correct new friction). The alternative (absent →
Unknown) would mass-block every legacy FT-710 file's channels and was
rejected for that reason, on the record.

**The digests, distinguished (rev 1 conflated them):** the
CONFIRMATION digest is ephemeral by design (in-memory plan, one
session). CONTENT digests are durable and schema-sensitive:
`RadioInfo.BaselineDigest` in saved snapshots and the journal's
baseline/candidate records. After migration these become explicitly
**non-recomputable legacy evidence** — a migrated snapshot's embedded
digest no longer matches recomputation over its migrated channels, and
that is recorded in both docs rather than papered over with digest
versioning (rejected as machinery without a consumer: nothing replays
journals today).

**Native CSV**: the `tag_display` column moves to the BoolField
spelling (`"yes"`/`"no"`/`""`=Unknown/`"n/a"`). Two recorded
consequences: Known-false's spelling changes (`""` → `"no"`), and a
pre-E1 CSV re-imported yields Unknown → blocked until set (mitigation:
explicit values or a bulk UI set). `parseBoolFieldCell`'s hardcoded
`scan_skip` diagnostic is parameterised. CHIRP import sets `Unknown`.

**"For free", stated correctly** (rev 1 misstated it): `changedFields`
needs NOTHING (whole-struct compare already handles state transitions);
only `addedFields` and `requestedFields` gain the Known-conditional,
with `FieldTagDisplay`'s POSITION preserved (before the tone/skip
appends) — membership AND order pinned by test, exact
refusal/BlockReason strings pinned, and FT-710 wire identity proven by
an explicit MW+MT frame test, not inference.

**Frontend**: bindings regenerate; `columns.js`'s FIVE shape-sensitive
behaviours (editability, rendering, added-row defaults, cloning, paste
patches) and `ChannelGrid.svelte`'s toggle all go state-aware on the
`skip` pattern; the `?? false` invention goes.

### E2 — the serial baud comes from the driver's capabilities

As revision 1 (`OpenRealSessionFor` passes
`d.Capabilities().DefaultBaud`; sole production `OpenSerial` site;
FT-710 opens at 38400 unchanged). Additions: the disagreeing-baud proof
uses an `internal/wiring` openSerial TEST SEAM (the transport serial
config is package-private — rev 1's proof was not expressible); stop
bits stay the fixed transport default BY RECORDED DECISION
(`Capabilities` gains no framing field; the FTdx10's framing is
verified from its manual at M9c-6, ASSUMED 8-N-2 until then).

### E3 — Engine binds to ONE dialect (impurity removal, not a bug fix)

**Revision 1's premise was false** (Codex): every configured dialect
builds exactly "AI0;" and every configured dialect's gate ADMITS that
form — the gate sees bytes, not provenance, so the claimed
foreign-gate refusal cannot occur and no fixture could demonstrate it.
The handoff's P9 "fails closed against a foreign AllowFunc" is
CORRECTED: it never failed at all.

What remains is architectural impurity worth removing while the
constructor is open: `NewEngine` takes the `cat.Dialect` as a REQUIRED
input (not a defaulted Option — a default does not enforce same-dialect
binding), and both the gate (`d.AllowedCommand`) and the init frame
(`d.BuildAISet(false)`) derive from that one input. Behaviour provably
unchanged (same bytes, same gate); the ft710 driver's call site passes
`d.dialect` instead of the method value; the construction guard holds.

### E4 — `app/` becomes model-aware

As revision 1's seven sites, with two corrections: **all seven share
ONE resolver** (the `currentCaps` pattern — rev 1 mixed raw
`Radio.Model` into the synthesis site, which would silently drop
discovered banks for a legacy file); and **the frontend does NOT "keep
working unchanged"** — the wailsjs bindings regenerate to the new
arity and the two bridge call sites pass `""` explicitly in this
milestone (compat wrappers rejected as churn without a consumer).

### E5 — the fake driver table becomes interface-typed

As revision 1, with the compilable shape (both reviewers): the
interface is `{ Port() io.ReadWriteCloser; Close() error }` — a
`Port() transport.Port` method is unsatisfiable by `*fakeradio.Radio`
(exact-signature matching; the returned value still satisfies
`transport.Port` by assignability at `drv.Open`) — and
`newRadio func() fakeRadio` with each closure capturing its own
option source. `FakeSessionOpts` stays FT-710-specific by documented
design. The table-driven fake-path test over `SupportedModels()`
closes the mismatched-pairing gap.

### E6 — `driver.WriteResult` becomes step-neutral (NEW)

Both reviewers: the four bools (`MWSent/MWConfirmed/MTSent/
MTConfirmed`) and clone's journalled `mw_sent`/…`mt_confirmed` keys are
FT-710 write choreography baked into the neutral seam — and mapping a
combined-MT write onto them would be misleading (an MT-without-MW
reads as "tag-only write" under FT-710 semantics). `WriteResult`
becomes:

```go
type WriteStep struct {
    Command   string // the frame's mnemonic, e.g. "MW", "MT"
    Sent      bool
    Confirmed bool
}
type WriteResult struct { Steps []WriteStep }
```

The four bools are REMOVED (unpushed pre-1.0 history; every consumer
is in-repo — the driver, clone's journal writer, their tests). The
journal writes step records with a format-version note (append-only
local audit; pre-change journals are the same non-recomputable legacy
evidence E1 already declares). The FT-710 reports its MW and MT steps;
a combined-MT driver will report one MT step, honestly. The M9c-3
choreography obligation (which read/write SEQUENCE the FTdx10 driver
uses) remains M9c-6's — E6 makes the REPORTING seam neutral so that
decision no longer forces a neutral-core change mid-driver-milestone.

## Out of scope, and recorded

As revision 1 (driver/fake/registration rows, caps table, choreography
decision, model picker UI, FTX-1/FTdx101, FieldSupport.Read
consumption), minus what E6 absorbed.

## Proof obligations

- **Byte identity with the ENUMERATED carve-outs** (rev 1's single
  carve-out was false by its own sanctions; the plan review corrected
  a fiction here — the historical import leg exits 3 and writes NO
  `import-out.json`, so that artefact never existed). Expected diffs:
  `read.stdout` (the digest line only); `read-fake.json` raw and
  normalised (after the recorded `read_at` normalisation: the schema
  field, every channel's `tag_display` shape, AND
  `radio.baseline_digest`); `export.csv` (the `tag_display` column's
  spelling); a NEW deterministic successful-import recipe evidences
  the import direction's changed class on both sides. EVERYTHING else
  byte-identical, including `probe.*`, both stderr streams, all exit
  codes (the historical import leg's stdout/stderr/exit unchanged),
  and every diff/refusal string — each "only" proven by diff against a
  base worktree build, not inferred from hashes. A schema-2→3 load
  round-trip reaches semantic identity.
- **The blocked-send scenario end-to-end**: an FT-710-bound channel with
  Unknown TagDisplay → Diff Blocked with the named reason → excluded
  per-channel while siblings send → never reaches the wire; the
  defence-in-depth refusal unit-tested; `Validate` rejects
  `{Unknown, true}` shapes.
- **Migration tests**: v1 and v2 fixtures × {present-true, present-false,
  absent} → Known with the right value; schema-3 Unavailable
  round-trips; ErrSchemaTooNew still refuses.
- **Diff order/strings pinned**: `addedFields` membership and ORDER;
  exact BlockReason strings; the MW+MT wire-identity frame test.
- **E2/E3**: FT-710 baud and "AI0;" bytes unchanged; the openSerial
  seam proves a disagreeing DefaultBaud reaches the config; Engine's
  gate and init provably from one dialect (a constructor test).
- **E4**: the one-resolver threading pinned; the `ok`-honouring
  silence pinned; the bridge call sites updated and typechecking.
- **E5**: the table-driven fake-path test; the guard suite; the
  compile-time assertion.
- **E6**: the journal's step records for an FT-710 write (MW then MT,
  sent+confirmed) pinned; the format-version note present; no
  `mw_sent`-style key survives repo-wide.
- gofmt/golden/evidence gates per task as always.

## Acceptance

1. All six enablers landed with their pins.
2. The manifest with the enumerated carve-outs, committed as
   `docs/superpowers/m9c5-baseline-manifest.md`.
3. Full local gate green; guards green.
4. The handoff: preconditions 8, 10, 11 struck with commit refs; P9
   struck WITH ITS CORRECTION (never a live defect); obligation 1
   closed; obligation 2 carried to M9c-6 with E6's reporting seam
   noted.

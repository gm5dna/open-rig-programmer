# M9c-5 — the registration enablers

**Date:** 29/07/2026
**Status:** revision 1, awaiting adversarial review
**Milestone:** M9c-5, the neutral-core enablers a registered second model
requires. M9c-6 (driver, fake, registration — the last M9c milestone)
follows.

## Decomposition decision, recorded

The handoff bundled these enablers with the FTdx10 driver/fake/
registration as one milestone. They are split, on the discipline that has
held through M9c: one architectural claim per milestone. M9c-5's claim is
"the neutral core no longer assumes the FT-710 anywhere a second
registered model would notice"; every change here is provable against the
FT-710 alone. M9c-6's claim is "the FTdx10 registers"; it consumes this
milestone and cannot be started safely before it.

## Scope: five enablers

### E1 — `TagDisplay` becomes an honest three-state field (the model decision)

**The defect** (M9c-3 ledgered obligation 1, mapped at HEAD): the
FTdx10's combined MT carries NO display flag, and
`codeplug.Channel.TagDisplay bool` makes the unknowable read as a
plausible `false` — worse, `json:"tag_display,omitempty"` serialises
that false as ABSENCE, so an FTdx10 codeplug and an FT-710 one with
display genuinely off are byte-identical on disk and hash identically in
the send-confirmation digest. The highest-consequence consumer is
`core/clone`'s write-verify (`execute.go:138-139`), where a wrong default
aborts a legitimate multi-slot send mid-run — while the file's own doc
(`execute.go:112-116`) already excludes CTCSS/ScanSkip for EXACTLY this
reason.

**The decision:** `ChannelData.TagDisplay` becomes `codeplug.BoolField`
— the existing `{State, Value}` idiom that sits one line either side of
it in the FT-710 driver's read path, with the existing `Unavailable`
state whose doc already says "this radio/protocol has no such field at
all". No new mechanism is invented; the field joins the one that
`ScanSkip` already uses. What follows for free, via the existing
`State == Known` guards: `diff.go`'s `addedFields`/`changedFields` gain
the same conditional the tone/skip fields have; `write.go`'s
`requestedFields` likewise; `writableFieldsMismatch` compares only
mutually-Known values (the CTCSS-exclusion rationale, applied by
mechanism instead of by exclusion list). The FT-710 driver reads it
`Known` from `ParseMTAnswer`'s display return and writes it only when
`Known`; a driver whose form has no display flag sets `Unavailable` —
**the first real producer of that state** (today only CSV "n/a"
round-trips produce it; the spec says so, so the reviews can attack the
newly-load-bearing paths).

**The sanctioned costs, stated so nobody restates them smaller:**

1. **This is a codeplug schema change.** JSON `"tag_display": true`
   (bool, omitempty) becomes the BoolField object shape. `CurrentSchema`
   bumps 2 → 3. `codeplug.Load` migrates schema-2 files: present `true`
   → `{Known, true}`; absent → `{Known, false}` **for schema-2 files
   only**, because under schema 2 absence genuinely meant
   FT-710-display-off (every schema-2 file came from an FT-710 path);
   the load-time normalisation precedent is the legacy padded-tag rule.
2. **The digest changes for byte-identical channel content**, because
   the digest hashes the marshalled struct. Sanctioned: the digest binds
   a send to a reviewed diff within one session; it is not a cross-
   version identity. Recorded in the digest's doc.
3. **The byte-identity bar carves out exactly this**: the M9c-5
   manifest's snapshot-JSON artefact is EXPECTED to differ in the
   `tag_display` shape (and schema field) and MUST be identical after a
   schema-2→3 load round-trip; every other artefact holds byte-identical.
4. **Frontend**: `models.ts` regenerates; `columns.js` renders the field
   state-aware exactly as `skip` already does ("—" for non-Known,
   editability gated on `state === 'known'`); `ChannelGrid.svelte`'s
   `?? false` nullish-invention goes. CSV: `TagDisplay` moves from
   `yesEmpty` to the `exportBoolField`/"n/a" spelling ScanSkip uses —
   **a CSV format change for one column**, sanctioned; CHIRP import
   sets `Unknown` (it carries no display data; today's silent `false`
   was the same defect in miniature).
5. **`spec.FieldSupport.Read` remains unconsumed** — recorded, not
   fixed: with the field itself carrying honest state, a read-side
   capability gate is redundant for this defect; inventing a consumer
   for symmetry would be speculative. The FTdx10 caps table (M9c-6)
   will declare `FieldTagDisplay: {Unsupported, Unsupported}` and the
   write gate already consumes `.Write`.

### E2 — the serial baud comes from the driver's capabilities (P8)

`internal/wiring/wiring.go:194-197` opens at `transport.DefaultBaud`;
`Capabilities.Bauds`/`DefaultBaud` are populated and validated but read
by nothing. Fix: `OpenRealSessionFor` (which already holds the driver
three statements earlier) passes `d.Capabilities().DefaultBaud`.
`spec.Validate` already refuses a non-positive `DefaultBaud`, and
`SerialConfig`'s zero-falls-back-to-default behaviour is therefore
unreachable from a validated driver — stated, with a test pinning that
the FT-710's opened baud is unchanged (38400 both ways; behaviour-
identical for the FT-710 by construction).

### E3 — `Engine.Init`'s frame comes from the injected dialect (P9)

`core/transport/engine.go:364-370` builds `cat.FT710.BuildAISet(false)`
against a gate that may belong to another dialect — the repo's own
ledgered "hardwired frame meets injected gate" site. The frame bytes are
dialect-invariant ("AI0;"), so this is a GATE problem: a foreign
`AllowFunc` refuses the FT-710-built frame. Fix: `NewEngine` gains the
dialect's builder — the minimal shape is an `Option` carrying a
`func() cat.Command` (or the `cat.Dialect` itself; the plan chooses
under the constraint that `TestNewEngineReachableOnlyFromDriver` and the
option pattern hold), defaulting to today's behaviour so the FT-710
driver's call site changes by one argument and nothing else moves.
FT-710 byte-identity: the same "AI0;" bytes either way.

### E4 — `app/` becomes model-aware (P10)

Eight sites; one (`app/app.go:270`'s `currentCaps`) is already the fixed
reference pattern. The other seven consult `wiring.DefaultModel` where
they must consult the session's/working copy's model:
`connection.go:71,84,86` (snapshot dir + both connects — `Connect`/
`ConnectDemo` gain a model parameter, validated against
`SupportedModels()`, defaulting to `DefaultModel` when empty so the
existing frontend keeps working until M9c-6 adds a picker),
`uispec.go:141` (offline bank synthesis → the working copy's model),
`uispec.go:248` and `send.go:216` (prose → the `currentCaps`-resolved
model, honouring `radiotext.For`'s `ok` the way the CLI does — silence,
never fabricated copy), `settings.go:52` (the descriptor follows the
resolved model; the doc's recorded FT-710-descriptor-for-foreign-session
defect closes). Every change is behaviour-identical while only the
FT-710 exists — pinned by the existing UISpec literal tests plus new
ones asserting the resolved-model threading.

### E5 — the fake driver table becomes interface-typed (P11)

`fakeDriverEntry.newRadio` returns `*fakeradio.Radio` concretely; the
consumer needs exactly `{ Port() transport.Port; Close() error }` (and
`transport.Port` is already `io.ReadWriteCloser`). Fix: a two-method
unexported interface in `wiring`; `newRadio` returns it; the per-fake
options stay inside each entry's closure (the process-global
`FakeSessionOpts []fakeradio.Option` remains FT-710-specific by
documented design — a second fake gets its own seam if and when a test
needs one, recorded). The guard's one-file/textual-call constraint
(`simulated_tokens_test.go`) holds — the closures keep calling their
constructors textually. **Plus the gap the handoff flagged, closed:**
`TestOpenFakeSessionFor_DefaultModel` becomes table-driven over
`SupportedModels()`, so a mismatched driver/fake pairing fails in CI,
not at runtime.

## Out of scope, and recorded

`core/driver/ftdx10`, `internal/fakedx10`, the wiring/radiotext/CLI/GUI
registration rows, the FTdx10 caps table, the write/read choreography
decision (M9c-6 — it is a DRIVER design question and the driver is
M9c-6's; the M9c-3 obligation 2 note carries forward), any model picker
UI (M9c-6 decides how the GUI selects models), the FTX-1/FTdx101
anything, `FieldSupport.Read` consumption (recorded above).

## Proof obligations

- **FT-710 byte identity with the E1 carve-out**: the full 16-artefact
  recipe; every artefact byte-identical EXCEPT the snapshot JSON, whose
  diff must be exactly the schema field + the `tag_display` shape, and
  which must round-trip schema-2→3 to semantic identity. The digest
  change is demonstrated (old ≠ new for identical content) and its doc
  updated.
- **Schema migration tests**: schema-2 fixtures (present-true, absent)
  load to Known-true/Known-false; a schema-3 file with `Unavailable`
  round-trips; an FTdx10-shaped codeplug (hand fixture) diffs and
  CSV-exports with "—"/"n/a" and never blocks on TagDisplay.
- **The write-verify honesty test**: `writableFieldsMismatch` with one
  side Unknown/Unavailable does not mismatch — the CTCSS rationale, now
  by mechanism.
- **E2/E3**: the FT-710 opens at 38400 and inits with "AI0;" bytes
  unchanged; a fixture dialect/caps pair proves the parameterisation
  (disagreeing values reach the port config and the gate).
- **E4**: UISpec/prose/settings resolved-model threading pinned; the
  `ok`-honouring silence pinned (a registered-model-without-radiotext
  case cannot exist by the registration test, so the pin uses a fake
  resolver seam).
- **E5**: the table-driven fake-path test over `SupportedModels()`;
  the guard suite green; a compile-time interface-satisfaction
  assertion for `*fakeradio.Radio`.
- Every task ends gofmt-clean with the golden/generated diff gates; the
  evidence-literal discipline holds (`core/cat` untouched by this
  milestone except nothing at all — E1-E5 live outside `core/cat`).

## Acceptance

1. All five enablers landed, each with its pins.
2. The manifest with the E1 carve-out, committed as
   `docs/superpowers/m9c5-baseline-manifest.md`.
3. Full local gate green; guards green.
4. The handoff's preconditions 8-11 struck through with commit refs;
   obligation 1 (TagDisplay) closed; obligation 2 explicitly carried to
   M9c-6.

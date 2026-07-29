# M9c-5 registration enablers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** the six neutral-core enablers of
`docs/superpowers/specs/2026-07-29-m9c5-registration-enablers-design.md`
(revision 2 — READ IT IN FULL FIRST; it is the authority on every
decision, including the adjudicated migration semantics, the plan-time
block, E3's corrected premise and E6's step-neutral shape).

**Architecture:** eleven sequential tasks. E1 spans five (model+schema,
diff+clone, driver, csvio, frontend+docs); E2-E6 one each; the manifest
closes. Every task: `gofmt -l .` silent; `git diff --exit-code --
core/cat/testdata/ core/cat/exinventory_gen.go core/cat/ftdx10/testdata/
core/cat/ftdx10/exinventory_gen.go`; the named test commands green.
British English; SPDX on new files; no golden regeneration; no push.
`core/cat` is untouched by this milestone — any diff under it is a STOP.

## Global facts (verified at source, cited for the implementers)

- `BoolField` serialises `{"state":"...","value":true-or-absent}`
  (`fieldstate.go:104-107`, `value,omitempty`); `ScanSkip` is the
  in-file precedent. `CurrentSchema = 2` (`file.go:20`);
  `ErrSchemaTooNew` machinery at `file.go:49-76`; `loadV1` decodes v1
  channels into the CURRENT struct (the thing Task 1 must stop).
- Diff's conditional-append precedent: `diff.go:200-206` (`addedFields`
  — tone/skip appended only when Known, AFTER the unconditional seven);
  `changedFields` (`diff.go:151-181`) compares whole structs and NEEDS
  NO CHANGE.
- The journal's write-record keys today: `execute.go:456-457`
  (`mw_sent`/`mw_confirmed`/`mt_sent`/`mt_confirmed` from
  `res.MWSent`…); the four bools live at `driver.go:51-63` and are set
  at `write.go:182-196`.
- The app resolver pattern: `app/app.go:262-272` (`currentCaps`); the
  CLI prose pattern honouring `ok`: `cmd/rigprog/write.go:116`,
  `probe.go:140`.
- The seven `wiring.DefaultModel` drift sites:
  `connection.go:71,84,86`, `uispec.go:141,248`, `send.go:216`,
  `settings.go:52`.
- The fake table: `fake.go:42-50` (`newRadio … *fakeradio.Radio`);
  consumers use only `Port()`/`Close()`; `Port()` returns
  `io.ReadWriteCloser`.
- Serial open: `wiring.go:194-197`; caps `Bauds`/`DefaultBaud` at
  `core/driver/ft710/caps.go:326-327`, validated at
  `spec/validate.go:241-252`, read by nothing.
- Engine init: `engine.go:364-370`; sole constructor call
  `ft710.go:186`; sole `Init` production call `ft710.go:205`;
  `BuildAISet` is already a Dialect method producing "AI0;" for every
  dialect.

---

### Task 1: E1a — the field, the schema, the migration

**Files:** `core/codeplug/channel.go` (the field), `fieldstate.go` (no
change expected — verify `BoolField` suffices), `validate.go`
(TagDisplay `Valid()` + the `{Unknown, true}` refusal), `file.go`
(`CurrentSchema = 3`; frozen `legacyChannelV1`/`legacyChannelV2` decode
structs with `TagDisplay *bool `json:"tag_display"``; the migration
rule — pointer-present true/false → `{Known, v}`, nil → `{Known,
false}`, with the spec's behaviour-preservation comment verbatim in
spirit; `loadV1` and the v2 path both route through the frozen
structs), plus their tests.

Test matrix (all in `file_test.go`/`validate_test.go` style, table-
driven): v1 × {present-true, present-false, absent}; v2 × the same;
schema-3 round-trips for Known-true/Known-false/Unknown/Unavailable;
`ErrSchemaTooNew` for schema 4; `Validate` refuses `{Unknown, true}`
and `{Unavailable, true}`; a saved schema-3 file re-loads identically.
Commit: `M9c-5 task 1: TagDisplay is a BoolField; schema 3 with frozen
legacy migration`.

### Task 2: E1b — diff and clone honour the state

**Files:** `core/codeplug/diff.go` (`addedFields`: `FieldTagDisplay`
moves from the unconditional list to a Known-conditional append AT ITS
CURRENT POSITION — before the tone/skip appends; the new plan-time
block: in the per-field gate, a channel whose `TagDisplay.State !=
Known` while the target's `FieldTagDisplay.Write != Unsupported` gains
`Blocked` with reason exactly `"tag display unknown — set On or Off
before sending"`; `changedFields` UNTOUCHED), `core/clone/execute.go`
(`writableFieldsMismatch` compares TagDisplay only when both sides
Known — the mechanism replacing the exclusion-list rationale; extend
that doc comment), tests: the blocked-send end-to-end (one Unknown
channel among Known siblings → that channel Blocked with the exact
reason, siblings plan and send; nothing reaches the wire for it);
membership AND ORDER of `addedFields` pinned; the FT-710 all-Known path
produces byte-identical diff output (pin exact strings).
Commit: `M9c-5 task 2: the diff blocks non-Known TagDisplay at plan
time; clone verifies only mutual knowledge`.

### Task 3: E1c — the FT-710 driver produces and consumes state

**Files:** `core/driver/ft710/read.go` (`TagDisplay:
codeplug.BoolField{State: Known, Value: display}`), `write.go`
(`requestedFields` conditional; the defence-in-depth refusal — a
non-Known display reaching `buildWriteCommands` returns
`WriteRefusedError` naming the field, before any wire traffic;
`BuildMTSet(sl, data.TagDisplay.Value, …)` only after the Known check),
`caps.go` untouched (rw stands). Tests: read produces Known; the
refusal unit test; **the MW+MT wire-identity test** — a reference
Known-true and Known-false channel's frames byte-equal their pre-E1
literals.
Commit: `M9c-5 task 3: the ft710 driver reads Known and refuses
non-Known display before the wire`.

### Task 4: E1d — csvio

**Files:** `core/csvio/export.go` (`tag_display` via `exportBoolField`),
`import.go` (`parseBoolFieldCell` gains a column-name parameter — its
diagnostic hardcodes `scan_skip` today; `tag_display` parses
yes/no/""/n-a → Known-t/Known-f/Unknown/Unavailable), `chirp.go`
(imported channels: `TagDisplay: {State: Unknown}` with a comment
naming the pre-E1 manufactured-false defect). Tests: export spellings
for all four states; import round-trips; the CHIRP fixture's channels
carry Unknown; the recorded reinterpretation (an old CSV's "" now
Unknown) demonstrated in a test comment + case.
Commit: `M9c-5 task 4: csvio speaks TagDisplay's four states; CHIRP
imports honestly Unknown`.

### Task 5: E1e — frontend and the digest docs

**Files:** regenerate wailsjs (`models.ts`); `columns.js` (five
behaviours on the `skip` pattern: editability `state === 'known'`,
render "—" for non-Known, added-row default `{state:'known',
value:false}`, clone and paste-patch carry the object), 
`ChannelGrid.svelte` (the toggle: state-aware, no `?? false`);
`core/codeplug/digest.go` + `radioinfo.go` doc updates (the two
digests distinguished; migrated snapshots and pre-change journals =
non-recomputable legacy evidence). Frontend checks: svelte-check +
vitest + build (the M9a-established commands); regen idempotent.
Commit: `M9c-5 task 5: the frontend renders TagDisplay's states; the
digest docs tell the truth`.

### Task 6: E2 — the baud

`wiring.go`: `Baud: d.Capabilities().DefaultBaud` (resolve `d` before
open — it already is); an unexported `openSerial` seam var for tests;
tests: FT-710 opens 38400 (the seam records the config); a fixture
driver with `DefaultBaud: 4800` reaches 4800; the stop-bits decision
recorded in a comment (fixed transport default; FTdx10 framing verified
at M9c-6).
Commit: `M9c-5 task 6: the serial baud comes from the driver's
capabilities`.

### Task 7: E3 — Engine binds to one dialect

`transport.NewEngine(port, d cat.Dialect, opts...)` — the dialect
REQUIRED; `AllowFunc` derived internally (`d.AllowedCommand`), `Init`
uses `d.BuildAISet(false)`; the old allow-func parameter GOES (sole
caller updated: `ft710.go:186` passes `d.dialect`). Every transport
test's constructor call updates (they are the other callers — enumerate
by grep, expect churn, it is sanctioned); the nil-gate refusal becomes
an unconfigured-dialect refusal (same fail-closed posture — check
`NewEngine`'s current nil-check and mirror it as
`!d.Configured()`). Guard `TestNewEngineReachableOnlyFromDriver` must
stay green unchanged. Tests: a constructor test proving gate and init
frame come from the SAME dialect value; FT-710 init bytes "AI0;"
unchanged. Include the handoff/doc corrections: `engine.go:350-363`'s
ledger note and `doc.go:180-186` rewritten (the impurity is CLOSED and
the old "fails closed" claim corrected — it never failed).
Commit: `M9c-5 task 7: Engine takes its dialect whole — gate and init
frame from one binding`.

### Task 8: E4 — app model-awareness

One unexported resolver (the `currentCaps` shape, returning the model
string) consumed by all seven sites; `Connect`/`ConnectDemo` gain a
`model string` parameter (empty → `DefaultModel`, else validated
against `SupportedModels()` with the CLI's error shape); prose sites
honour `radiotext.For`'s `ok` (silence on false); `settings.go`'s
descriptor follows the resolved model; wailsjs regenerated; the TWO
bridge call sites (`bindings.js:170/187`-region) pass `""`. Tests: the
threading pins (a seam-injected model reaches snapshot dir, prose,
settings); UISpec literals unchanged for the FT-710; frontend
svelte-check/vitest/build green.
Commit: `M9c-5 task 8: app resolves its model once and threads it
everywhere`.

### Task 9: E5 — the fake table interface

`fake.go`: `type fakeRadio interface { Port() io.ReadWriteCloser;
Close() error }`; `newRadio func() fakeRadio` (options captured in the
FT-710 entry's closure; `FakeSessionOpts` untouched and documented
FT-710-specific); `var _ fakeRadio = (*fakeradio.Radio)(nil)`;
`TestOpenFakeSessionFor_DefaultModel` → table-driven over
`SupportedModels()` asserting each model's fake session identifies as
its own driver (CATID via the session). Guards green (the textual
`fakeradio.New` call survives).
Commit: `M9c-5 task 9: the fake table is interface-typed; the
fake-path test covers every registered model`.

### Task 10: E6 — WriteResult goes step-neutral

`driver.go`: the spec's `WriteStep`/`WriteResult` verbatim; the four
bools DELETED. `write.go:170-196`: build `Steps` (MW step, MT step —
sent/confirmed at the same points). `execute.go:450-460`: the journal
record becomes `"steps": [...]` (each `{"command","sent","confirmed"}`)
plus `"write_result_format": 2` — the format-version note; the old four
keys go. Repo-wide grep gate: `MWSent|MTSent|mw_sent|mt_sent` → zero
hits outside historical docs. Tests: the FT-710 write's journal steps
pinned (MW then MT, both sent+confirmed on success; the partial-failure
cases mirror the old bools' semantics — read the existing tests and
carry each case over); clone's consumers updated.
Commit: `M9c-5 task 10: WriteResult reports neutral steps; the journal
is step-keyed`.

### Task 11: the manifest with enumerated carve-outs

The full 16-artefact recipe vs the M9c-3/M9c-4 recorded hashes, with
the spec's EXPECTED-DIFF table verified artefact by artefact:
`read.stdout` differs ONLY in the digest line; both JSONs ONLY in
schema/`tag_display`/`baseline_digest`; `import-out.json` the same
class; `export.csv` ONLY the `tag_display` column; probe.*, both
stderr, all exit codes, and every other byte IDENTICAL. Prove each
"only" by diff, not hash (the diffs are the evidence). A schema-2→3
load round-trip to semantic identity. Full local gate; guards -v.
Write `docs/superpowers/m9c5-baseline-manifest.md`; update the handoff
per the spec's acceptance item 4.
Commit: `M9c-5 task 11: the enumerated-carve-out manifest and full
gate`.

## Self-review

Spec coverage: E1→T1-5, E2→T6, E3→T7, E4→T8, E5→T9, E6→T10,
manifest→T11. Ordering: T1 before T2-5 (the type); T2 before T3 (the
block the driver's refusal backstops); T5 after T1-4 (regen once);
T6-T10 independent after T1 where they touch it (T10 touches clone —
after T2); T11 last. The evidence-literal discipline: no `core/cat`
file is touched. Placeholders: none; every rule is the spec's, cited.

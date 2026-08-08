# M9d-2 FTdx101D/MP driver, fake, registration — Implementation Plan (revision 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> superpowers:subagent-driven-development to implement this plan
> task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. There
> are NO quarantined legs in this milestone — every evidence artefact
> it consumes was produced and review-bound at M9d-1; every task here
> is an ordinary code task whose implementer reads this plan, the spec
> and the capability matrix.

> **Revision 2 (09/08/2026).** Revision 1 was reviewed by Codex
> (NEEDS-REVISION, 3 HIGH + 6 MEDIUM + 2 LOW) and a Fable agent
> (APPROVE-WITH-FIXES, 2 MEDIUM + 3 LOW); adjudication in
> `.superpowers/sdd/m9d2-plan-review-adjudication.md`, all merged
> findings folded here. The blockers: per-model write-guard constants
> (one shared constant could not represent a single-model flip); the
> designed-delta classification blessed rows that cannot move; and the
> plan's own digit hygiene. The FTdx10 prose step survived adjudication
> re-scoped (prose-only, own commit, provenance stated). Do not
> implement revision 1.

**Goal:** register `FTdx101D` and `FTdx101MP` end to end — ONE driver
(`core/driver/ftdx101`) and ONE fake (`internal/fakedx101`), each
internally parameterised by model, producing two registered models —
plus the sanctioned caps-aware CHIRP blank-Skip fold, the
wrong-sibling probe refusal with both names spelled out, and the
byte-identity close with per-sibling baselines.

**Spec:** `docs/superpowers/specs/2026-08-05-m9d-ftdx101-design.md`
(rev 2), M9d-2 section. **Capability authority:**
`docs/superpowers/m9d2-capability-matrix.md` (the A4 artefact,
review-clean at `2745e14`) — every capability VALUE and its evidence
status comes from the matrix, never from this plan's restatement of
it; where the two disagree, the matrix wins and the disagreement is a
STOP. The settings read is **193** items (matrix §3.9) — the spec's
class phrase for it is sized by the FTdx10's inventory, and per the
matrix no count other than 193 may appear anywhere in this milestone's
code or prose (this plan included; sweeping the milestone's diff for
the FTdx10's count is part of Task 9's gate).

**Architecture:** exact sibling of M9c-6 (plan `.superpowers/sdd/
m9c6-plan.md`, executed rev 2): driver skeleton → write path →
settings → fake core → fake EX projection → registration → the CHIRP
fold → byte identity. The deltas from M9c-6 are: two registered models
from one driver/fake pair; the additive `WrongRadioError` found-model
resolution (spec A5); the CHIRP blank-Skip designed delta (spec
decision 5, Stuart-sanctioned); and the golden gate growing eight →
TEN.

**Tech stack:** Go, `core/cat/ftdx101` (DialectD/DialectMP),
`core/driver` seam, net.Pipe scripted responders, Wails/Svelte
frontend (prose-only changes), stdlib-only generator under
`internal/fakedx101/gen`.

## Global Constraints

- Branch `m9d2-ftdx101-registration` from main at this plan's commit;
  tasks execute SEQUENTIALLY, one shared worktree, no parallel
  commits. Fresh Opus implementer per task; orchestrator verifies
  every landing. Commit trailers: Co-Authored-By + Claude-Session.
- British English; SPDX (`GPL-3.0-or-later`) on every new `.go` file;
  `gofmt -l .` silent per task; `go vet ./...` clean per task.
- **Never regenerate any golden.** Per-task mechanical gate, EVERY
  task from its creation onward:
  `git diff --exit-code -- core/cat/testdata/ core/cat/exinventory_gen.go
  core/cat/ftdx10/testdata/ core/cat/ftdx10/exinventory_gen.go
  internal/fakedx10/exinventory_gen.go internal/fakedx10/transcription-b.csv
  core/cat/ftdx101/testdata/ core/cat/ftdx101/exinventory_gen.go`
  AND (from Task 6 onward, when they exist)
  `git diff --exit-code -- internal/fakedx101/transcription-b.csv
  internal/fakedx101/exinventory_gen.go`
  — the golden gate is EIGHT paths entering this milestone and TEN
  when it closes.
- **The model names are load-bearing and must never be negative
  fixtures.** `"FTdx101D"` and `"FTdx101MP"` are the registry keys
  (matrix §1.1; the spec fixes the spelling). The M9c-6 sentinel
  lesson (`cmd/rigprog/wiring_test.go:25-38`) generalises: any test
  needing an unknown model uses `unknownModelSentinel`
  (`"NO-SUCH-MODEL"`, `cmd/rigprog/wiring_test.go:38`) or
  `"FT-NONEXISTENT"` (the wiring package's local convention), NEVER a
  name this or any future milestone might register.
- **Wall-clock is budgeted, not optimised.** Discovery walks 100
  exchanges/Open (~2-2.5 s); registering two discovery-walking models
  roughly triples `TestOpenFakeSessionFor_EveryRegisteredModel`'s cost
  and adds two models' worth of per-model suites (matrix §3.4). Gates
  budget minutes. NOBODY narrows discovery (settle override, early
  termination, range shrink) without an orchestrator-adjudicated
  change.
- File lists and line numbers are HYPOTHESES verified at task time —
  the verification gates hold, not the enumeration.
- STOPs are real and orchestrator-arbitrated: any disagreement between
  this plan and the matrix; any capability value not derivable from
  the matrix; any golden/manifest diff outside the designed-delta list
  (Task 9); any guard firing where Task 6's pre-verification said it
  would not.

## Plan-level decisions

- **(D1) Two thin exported constructors over ONE parameterised
  implementation, in both new packages.** The spec's "ONE driver
  parameterised by model name + CAT ID" is satisfied internally:
  `ftdx101.NewD`/`ftdx101.NewMP` wrap a private
  `newDriver(m modelParams, profile Profile)`, exactly as the dialect
  package exposes `DialectD()`/`DialectMP()` over one `newDialect` —
  the two-instances rationale at `core/cat/ftdx101/dialect.go:67-72`,
  the accessor-over-var rationale at `dialect.go:126-136`, and the
  no-bare-`Dialect()` rule at
  `core/cat/ftdx101/exinventory.go:19-21` ("there are two models, so
  there are two of them"). No exported model enum: an enum's
  zero value would need its own fail-safe arm, and registration-table
  closures never hold a model value that could be forged. The fake
  mirrors this: `fakedx101.NewD`/`fakedx101.NewMP` over
  `newRadio(catID string, opts ...Option)`.
- **(D2) The A5 mechanism is ADDITIVE OPTIONAL NAME FIELDS on
  `driver.WrongRadioError`,** populated by drivers that can name the
  models involved, with the ID-only `Error()` text byte-pinned
  unchanged for the FT-710 and FTdx10 (which never populate them). The
  sibling ID→name mapping is `core/driver/ftdx101` package-local
  knowledge (it owns both registrations); `cmd/rigprog/probe.go` gains
  no ID→name table — its formatter renders the name only when the
  error carries one. Exact strings in Task 1. The registry-fed
  render-site alternative the spec also permits is REJECTED here
  because it would put a wiring dependency (or a second ID→name table)
  into every render site, and the driver already knows both names at
  the refusal site.
- **(D3) The FTdx101 Simulated profile's write set:** all SIX
  combined-MT core fields write-Supported including clarifier
  Supported, NOT Inert — the matrix §2.1 profile table verbatim, for
  the M9c-6 reason (Inert is an FT-710 HARDWARE finding, caps.go's
  non-borrowing note; `internal/fakedx101` stores the clarifier and
  round-trips it byte-faithfully). RealHardware: every candidate
  Unverified; invalid profile fail-safe Unverified. Enumerated per
  field in caps.go, pinned by a static profile-matrix test (no fake
  involved).
- **(D4) ONE settings descriptor id for both models:
  `ftdx101-ex@1`.** The descriptor describes the shared 193-item
  Table 2 surface, which the M9d-1 applicability attestation proves
  model-unconditional; model identity lives in `Capabilities().Model`
  and the CAT ID, not in the menu inventory. (The FTdx10's is
  `ftdx10-ex@1`; a per-model id here would claim a difference the
  evidence says does not exist.)
- **(D5) The caps-aware skip predicate moves to `core/spec`:**
  `FieldSupport.Unreachable() bool` (Read AND Write `Unsupported`).
  `core/csvio/chirp.go:353-360` says in terms that when a third caller
  of the both-Unsupported rule appears it must move to a shared
  package rather than be copied again; `chirpScanSkip` is that third
  caller (after `chirpTagDisplay`, chirp.go:367, and
  `bankTagDisplayDefault`, app/uispec.go:175). Both existing callers
  are refactored onto the method behaviour-identically (their pins
  hold), and the duplication note is updated.
- **(D6) fakedx101 carries NO fault options,** for fakedx10's recorded
  reason (`internal/fakedx10/options.go:9-16`, doc.go:90-105: the
  seven fakeradio faults exercise MODEL-INDEPENDENT transport
  behaviour); `WithLatency` is kept because it is not a fault. The
  omission and the seven names are restated in fakedx101's doc.go.
- **(D7) The scripted-port seam is the M9c-6 command-parsing
  responder,** built once in Task 2 as `respondingPort` (ftdx101 test
  package), parameterised by CAT ID so one helper serves both models;
  every driver test answers Open's full choreography (AI0, ID, 100
  discovery probes). A fixed-transcript stub is explicitly
  insufficient.
- **(D8) D-vs-MP radiotext prose may differ ONLY where it names the
  model.** The two radios share one manual; inventing per-model
  operational prose would violate the honesty rule. Each model still
  gets its own pinned-verbatim `Text` value.

## File Structure

| File | Task | Responsibility |
|---|---|---|
| `core/driver/driver.go` (modify ~:187-201) | 1 | `WrongRadioError` additive `WantModel`/`GotModel` + extended `Error()` |
| `cmd/rigprog/probe.go` (modify ~:87-89), `probe_test.go` | 1 | name-aware `wrongRadioMessage`, ID-only text pinned |
| `core/driver/ftdx101/doc.go` | 2 | package doc; the NINE-entry per-model ASSUMED register; dialect-register citations BY NAME; the §1.3.5 policy precision; MT-only/discovery/no-region notes |
| `core/driver/ftdx101/ftdx101.go` | 2 | `NewD`/`NewMP`, `modelParams`, Open (AI0, ID probe + wrong-sibling refusal, discovery, synthesis), `DiscoveredBankSynthesizer` |
| `core/driver/ftdx101/caps.go` | 2 | all 15 fields explicit from the matrix incl. labels + NoBlank; profiles; `writeTrialsCompleteD` / `writeTrialsCompleteMP`, both false |
| `core/driver/ftdx101/read.go` | 2 | MT-only atomic `ReadChannel` |
| `core/driver/ftdx101/respondingport_test.go` + `ftdx101_test.go`, `caps_test.go`, `dialect_test.go`, `read_test.go` | 2 | responder; probe/refusal/discovery/read/profile-matrix/pin tests |
| `core/driver/ftdx101/write.go`, `write_test.go` | 3 | MT-only write choreography; refusal ladder; NAMED INVERSION |
| `core/driver/ftdx101/settings.go`, `settings_test.go` | 4 | 193-item descriptor `ftdx101-ex@1`; StaticSettingsProvider; EX round-trip |
| `internal/fakedx101/doc.go`, `fakedx101.go`, `options.go`, `parser.go`, `state.go`, `image.go`, `imports_test.go`, behaviour tests | 5 | ID-parameterised fake core; recursive import fence FROM BIRTH; no-fault note |
| `internal/fakedx101/transcription-b.csv`, `PROVENANCE.md`, `gen/main.go`, `gen/main_test.go`, `ex.go`, `exinventory_gen.go` | 6 | the B-projection with teeth; staleness; golden gate → TEN |
| `core/transport/ex_crosscheck_ftdx101_test.go` | 6 | dialect-from-A vs fake-from-B cross-check + wire round-trip + CSV byte-identity |
| `internal/wiring/wiring.go`, `fake.go`, `wiring_test.go` | 7 | two constants, two real rows, two fake rows, TWO option vars, presence pins FIRST, slug/synthesis/static tests |
| `internal/radiotext/radiotext.go`, `radiotext_test.go` | 7 | two entries, all eight fields, verbatim pins, near-miss additions |
| `internal/guards/simulated_tokens_test.go` | 7 | two ftdx101 rows (NewD/NewMP) |
| `app/uispec_test.go` | 7 | per-model membership/bankReadOnly/TagDisplayDefault/GetUISpec pins |
| `core/cat/ftdx10/doc.go` + citation sites (modify, prose-only) | 7 | the seventh (0x2D) register entry; de-positionalised register citations |
| `core/spec/support.go`, `core/csvio/chirp.go`, `chirp_test.go`, `app/uispec.go` (bankTagDisplayDefault refactor onto `Unreachable()` + comment), frontend comment sites | 8 | `Unreachable()`; caps-aware `chirpScanSkip`; per-radio tests; prose updates |
| `docs/superpowers/m9d2-baseline-manifest.md` | 9 | byte identity; designed deltas verbatim + removal-proven; 2×14 sibling legs; red-proof index; full gate |

---

### Task 1: the additive found-model resolution on WrongRadioError (spec A5)

**Files:** modify `core/driver/driver.go:187-201`,
`cmd/rigprog/probe.go:87-89`; tests in `core/driver/driver_test.go`
(or the file holding error tests), `cmd/rigprog/probe_test.go`.

- [ ] **Write the failing tests first.** In core/driver's test file:
  (a) ID-only text PINNED — `(&WrongRadioError{Want: "0800", Got:
  "0761"}).Error()` equals EXACTLY
  `driver: connected radio identified as CAT ID "0761", want "0800" — wrong radio model on this port`
  (today's format string at `driver.go:196-198`, asserted as a
  literal so the extension cannot move it); (b) named text —
  `(&WrongRadioError{Want: "0681", Got: "0682", WantModel: "FTdx101D",
  GotModel: "FTdx101MP"}).Error()` equals EXACTLY
  `driver: connected radio identifies as FTdx101MP (CAT ID "0682"); you selected FTdx101D (CAT ID "0681") — wrong radio model on this port`;
  (c) ONE name populated (either alone) → the ID-only text (the
  extension fires only when BOTH are present); (d)
  `errors.Is(err, ErrWrongRadio)` and `errors.As` with Want/Got
  preserved, for both shapes. In `probe_test.go`: extend
  `TestWrongRadioMessage_NamesSelectedModel` (`probe_test.go:155`)
  with a named-error case, and pin the existing ID-only expected line
  (`probe_test.go:159`) UNCHANGED.
- [ ] Run: `go test ./core/driver/ ./cmd/rigprog/ -count=1 -run
  'WrongRadio'` — expected FAIL (fields undefined).
- [ ] Implement. `driver.go` — add to the struct, after `Got`:

```go
	// WantModel and GotModel are OPTIONAL display names, additive as of
	// M9d-2 (spec A5): a driver that can NAME the models its IDs belong
	// to may populate both, and Error() then spells the refusal out in
	// model names as well as IDs. Drivers that cannot (the FT-710's and
	// FTdx10's never do) leave them empty and Error()'s ID-only text is
	// byte-identical to what it was before these fields existed — that
	// text is pinned by test, because rendered refusals are recorded in
	// baselines. Is/As semantics are unchanged either way.
	WantModel string
	GotModel  string
```

  and rewrite `Error()`:

```go
func (e *WrongRadioError) Error() string {
	if e.WantModel != "" && e.GotModel != "" {
		return fmt.Sprintf("driver: connected radio identifies as %s (CAT ID %q); you selected %s (CAT ID %q) — wrong radio model on this port", e.GotModel, e.Got, e.WantModel, e.Want)
	}
	return fmt.Sprintf("driver: connected radio identified as CAT ID %q, want %q — wrong radio model on this port", e.Got, e.Want)
}
```

  `probe.go` — rewrite `wrongRadioMessage`:

```go
func wrongRadioMessage(model string, wr *driver.WrongRadioError) string {
	if wr.GotModel != "" {
		return fmt.Sprintf("rigprog probe: wrong radio: radio identifies as %s (CAT ID %q); you selected %s — this port's radio does not identify as %s\n", wr.GotModel, wr.Got, model, model)
	}
	return fmt.Sprintf("rigprog probe: wrong radio: got CAT ID %q, want %q — this port's radio does not identify as %s\n", wr.Got, wr.Want, model)
}
```

- [ ] Run: `go test ./core/driver/ ./cmd/rigprog/ -count=1` — PASS;
  then the whole-repo quick check `go test ./... -count=1 -run
  'WrongRadio|Probe'`, gofmt, vet, eight-path golden gate.
- [ ] Commit: `M9d-2 task 1: the additive found-model resolution on
  WrongRadioError, ID-only text pinned`.

### Task 2: driver skeleton — caps, register, probe, MT-only read, discovery, synthesis

**Files:** create `core/driver/ftdx101/{doc.go,ftdx101.go,caps.go,
read.go}` + the test files in the table. Structural exemplar:
`core/driver/ftdx10` (M9c-6 tasks 1's deliverable) — its SHAPE is the
template, its VALUES are never evidence (matrix, "How to read an
entry"). Every capability value comes from the matrix §1/§2 with its
citation; the implementer transcribes the matrix, not the FTdx10.

- [ ] `ftdx101.go`: `modelParams{name string, dialect cat.Dialect}`;
  `var modelD = modelParams{name: "FTdx101D", dialect: ftdx101.DialectD()}`,
  `modelMP` likewise with `DialectMP()`;
  `func NewD(profile Profile, opts ...Option) driver.Driver` /
  `NewMP(...)` over `newDriver(m modelParams, profile Profile, ...)`.
  Open choreography mirrors ftdx10.go: AI0, ID probe, full discovery,
  effective capabilities. **The wrong-sibling refusal populates the
  Task-1 fields from a package-local map:**

```go
// siblingNames maps every CAT ID this PACKAGE registers to its display
// name, so a probe refusal can spell out both models involved (spec A5).
// It deliberately knows nothing beyond this package's own two radios: a
// foreign ID (an FT-710's "0800", say) resolves to "", and the error's
// ID-only text renders — naming other packages' radios is their business.
var siblingNames = map[string]string{
	modelD.dialect.CATID():  modelD.name,
	modelMP.dialect.CATID(): modelMP.name,
}
```

  refusal site (the ftdx10.go:191 shape):
  `return nil, &driver.WrongRadioError{Want: catID, Got: got,
  WantModel: m.name, GotModel: siblingNames[got]}` — refusal BEFORE
  any discovery frame, port closed by Open's error path.
  `DiscoveredBankSynthesizer` IMPLEMENTED with the compile assertion
  (M9c-6/Codex 5: its absence fails silently). RegionReporter NOT
  implemented; doc.go says why (no FTdx101 inventory ever observed).
- [ ] `caps.go`: ALL FIFTEEN fields explicit, each with the matrix
  citation in its comment (§1.1-§1.15): Model/CATID from
  `modelParams` (CATID via `m.dialect.CATID()` — one source);
  banks MEM 001-099 / PMS P1L-P9U through the dialect's own
  MemorySlot/PMSSlot walk-until-refusal; `bankFields` with all TEN
  `spec.Field`s explicit (§2.1: six rw, tag_display/ctcss_tone/
  scan_skip/erase the written-down zeros, each comment stating
  MANUAL-EVIDENCED-absence vs ASSUMED per the matrix); Modes by
  enumerating the dialect (ftdx10's `modeNames()` shape, ModeUnset
  excluded); TagLen 12; ClarMaxHz 9990 / ClarStepHz 10 (dialect
  register cited BY NAME); CTCSSTones `spec.StandardCTCSSTones()`
  (Table 1, layout 566-575, matrix §1.8); Bauds {4800, 9600, 19200,
  38400} (layout 863) / DefaultBaud 38400 (DRIVER register entry);
  Min/MaxFreqHz 30_000/75_000_000 (DRIVER register entry, BS-legend
  refusal noted); RequiredSlots {"001"} (DRIVER register entry);
  ShiftOptions/CTCSSStates standard. Profiles per D3.
  **Bank labels and NoBlank transcribed from the matrix, all four
  banks:** MEM `Label: "Memories"`, `NoBlank: false` stated (§1.3.1);
  PMS `Label: "Scan limits (PMS)"`, `NoBlank: false` stated — with the
  M5b lesson in the comment (§1.3.2); discovered 60M
  `Label: "60 m channels"` and EMG `Label: "Emergency (EMG)"`, both
  `NoBlank: true` (§1.3.4), each label commented CHOICE.
  **TWO write-guard constants, PER MODEL (spec: "pinned PER MODEL"):**
  `const writeTrialsCompleteD = false` and
  `const writeTrialsCompleteMP = false`, each with the two-part-change
  doc naming ITS model's evidence, each consulted by no production
  code — one shared constant could not represent a single-model flip,
  and a D trial must never flip the MP's (matrix §3.11).
  **The discovered banks' read-only derivation comment states the
  §1.3.5 PRECISION — MW exclusion manual-evidenced (layout 1353), MT
  exclusion project policy (`core/cat/mt.go:100-108`) — never the
  FTdx10 comment's compression.**
- [ ] `doc.go`: provenance (everything through `core/cat/ftdx101`'s
  dialect; NO FTdx101 of either model ever asked anything); the
  **NINE-entry ASSUMED register** — the matrix §5 list verbatim in
  scope: FRAMING 8-N-2; CONTROL-LINE POLICY (CAT RTS is (03,01,13)
  here, layout 865 — not the FTdx10's (03,01,10)); DefaultBaud 38400;
  Min/MaxFreqHz; RequiredSlots {"001"}; TONE AND SCAN-SKIP
  UNREACHABILITY; "?;" discovery-probe ABSENT; "?;" empty-slot read
  EMPTY; SINGLE COMBINED MT SET SUFFICES (landing here, one task ahead
  of the write path, per matrix §3.6) — each with its per-model Stage
  R/W lift from the matrix, and the register preamble carrying the
  PER-MODEL rule (§3.10: a D capture never lifts an MP entry; an entry
  stays open for whichever model has not been asked) and the §3.12
  port rule (every capture records its port; AI is USB-only, layout
  381). **ALL SEVEN dialect-register entries cited BY NAME** at the
  points of dependence (never "entry 6" — the M9d-1 forward item),
  including NoneWire (the read path's refusal of the none form) and
  the 0x2D minus byte (the frames the write path builds).
- [ ] Tests (all through `respondingPort`, D7): capabilities
  explicitness (reflect over 15 fields, both models) INCLUDING the
  bank labels and NoBlank values asserted for the two static banks of
  each model AND for the effective/synthesised discovered banks;
  `TestWriteTrialsComplete_PinnedFalse` table-driven per model, each
  row asserting ITS constant (`writeTrialsCompleteD` /
  `writeTrialsCompleteMP`) is false AND that model's RealHardware
  baseline is genuinely nothing-writable; the STATIC
  profile matrix (per-field membership per profile, no fake); wrong
  radio BOTH DIRECTIONS with names — NewD against `"ID0682;"` yields
  `*driver.WrongRadioError{Want:"0681", Got:"0682",
  WantModel:"FTdx101D", GotModel:"FTdx101MP"}` via `errors.As`, NewMP
  against `"ID0681;"` the mirror, NewD against `"ID0800;"` yields
  empty `GotModel` (ID-only text) — each asserting
  `errors.Is(ErrWrongRadio)`, the responder transcript contains NO
  discovery probe, and the port is closed; MT-only read
  (populated / empty "?;" → `Channel{Slot}` / out-of-vocab kind →
  wrapped `*cat.ParseError` via errors.As / slot-echo mismatch → the
  driver's typed error; malformed answers REFUSE, never guess);
  full-transcript discovery per model (ordered 501..599+EMG, sparse
  population at 503, 599 and EMG); **malformed-discovery refusal**
  (spec Error handling: discovery refuses rather than guesses) — a
  responder serving a MALFORMED combined answer to one 5xx probe, and
  one serving a WRONG-SLOT echo, each making `Open` fail with the
  typed parse/mismatch error via `errors.As`, NO bank synthesised
  from the partial walk, port closed (the ftdx10 discovery error
  shape, `core/driver/ftdx10/ftdx10.go:265-303`); live-vs-offline synthesis
  equivalence; zero-dialect Open refusal; the tone-table pin
  (caps.CTCSSTones == `spec.StandardCTCSSTones()`, fixture note citing
  Table 1); a D-vs-MP capability-identity test —
  `reflect.DeepEqual` over the two models' `Capabilities()` after
  normalising Model and CATID, pinning "identical except the two
  model-conditional values" (matrix §4).
- [ ] Gate: `go test ./core/driver/... -count=1`, gofmt, vet,
  eight-path golden gate. Commit: `M9d-2 task 2: the FTdx101 driver
  skeleton — caps from the matrix, per-model register, wrong-sibling
  refusal, MT-only read, full discovery`.

### Task 3: write path — the MT-only choreography

**Files:** create `core/driver/ftdx101/write.go`, `write_test.go`.
Exemplar `core/driver/ftdx10/write.go` (shape only).

- [ ] Refusal ladder (ParseSlot, bankFor, erase refusal, `Valid()`
  checks incl. `ScanSkip.Valid()`, capability gate over
  requestedFields); the **NAMED INVERSION documented at the exact spot
  the FT-710 refuses non-Known TagDisplay** (matrix §3.7: no display
  flag exists; a Known value is caught by the capability gate); ONE
  `BuildMTSetCombined` frame; steps `[{Command:"MT"}]` declared after
  the frame exists, before the wire; Sent/Confirmed per
  `core/driver/driver.go:44-57`.
- [ ] Tests (via respondingPort, BOTH models where the frame differs
  by nothing — assert that too): literal-frame pin (a known channel →
  the exact 41-byte Set HAND-DERIVED from the matrix §2.1 position
  map / geometry witness, NOT via the builder); refusal ladder order
  incl. Known-TagDisplay → `ErrWriteRefused` naming the field;
  one-step WriteResult truth table (success / `cat.ErrRejected` →
  Sent-only / transport error → neither). The Simulated end-to-end
  write test lands in Task 7 (needs the registered fake) — stated,
  not silent.
- [ ] Gate: as Task 2. Commit: `M9d-2 task 3: the FTdx101 MT-only
  write choreography`.

### Task 4: settings read — 193 items

**Files:** create `core/driver/ftdx101/settings.go`,
`settings_test.go`. Exemplar `core/driver/ftdx10/settings.go`.

- [ ] Descriptor id **`ftdx101-ex@1`** (D4), built from the dialect's
  `EXItems()` in inventory order — 193 items, 4 menus, 18 groups
  (matrix §3.9; the counts asserted from the dialect, not hardcoded
  beyond the pin); StaticSettingsProvider + SettingsReader + compile
  assertions; exSpec full-address ExpectPrefix. `ReadSetting` returns
  P4 verbatim — no value legend interpreted (M9d-1's boundary).
- [ ] Tests: descriptor Validate/uniqueness/count==193, scripted EX
  round-trip via respondingPort, the one Text item (04,01,01)
  behaves; both models serve the SAME descriptor (assert equality —
  D4's claim); **the defensive-copy contract** (the
  `driver.StaticSettingsProvider` obligation, `core/driver/optional.go`
  — the ftdx10 mutation-test shape at
  `core/driver/ftdx10/settings_test.go:346-397`): mutate nested
  menus/groups/items/slices of one returned descriptor and assert
  later calls — the same getter, the static getter, the session
  getter, AND the sibling model's — are unaffected.
- [ ] Gate: as Task 2. Commit: `M9d-2 task 4: the 193-item FTdx101
  settings read`.

### Task 5: fakedx101 core

**Files:** create `internal/fakedx101/{doc.go,fakedx101.go,options.go,
parser.go,state.go,image.go,imports_test.go}` + behaviour tests.
Exemplar `internal/fakedx10` (shape); the HARD RULE (no
project-internal imports) from birth.

- [ ] `NewD(opts ...Option) *Radio` / `NewMP(...)` over
  `newRadio(catID string, opts ...Option)` (D1); the ID answer is
  built from the Radio's configured ID (`buildIDAnswer` becomes a
  method — the one deliberate divergence from fakedx10's zero-argument
  literal at `internal/fakedx10/parser.go:698`; fakedx101's own doc.go
  records the divergence and why); parser mirrors fakedx10's command set (ID,
  AI, MC, MR answers, MW Set, combined MT Set/read — 41-byte answers
  to the position chart, tag space-padded, clarifier STORED);
  `state.go`, `image.go` (default M-01 + 9 PMS pairs;
  `WithSlot`/`With5xx`/`WithEMG`; no region claim); options per D6
  (five + the two image options, NO faults, the seven fakeradio fault
  names restated in doc.go with the reason).
- [ ] `doc.go`: the fake's OWN ASSUMED register (empty-slot "?;",
  PMS/5xx/EMG kind '1' on combined answers, invented EX values, +
  implementer additions), each entry noting it is a claim about the
  FAKE consumed by tests, per model where the ID matters.
- [ ] `imports_test.go`: the SUBDIRECTORY-WALKING no-imports fence
  copied from `internal/fakedx10/imports_test.go` — landing in THIS
  task so the fence covers `gen/` before Task 6 creates it.
- [ ] Tests: frame-level per command class (populated/empty/malformed,
  "?;"), combined Set→read round-trip byte-faithful (incl.
  clarifier), option coverage, ID answer per model (`"ID0681;"` from
  a NewD radio, `"ID0682;"` from NewMP, malformed body rejected).
- [ ] Gate: `go test ./internal/... -count=1`, gofmt, vet, eight-path
  golden gate. Commit: `M9d-2 task 5: the ID-parameterised fakedx101
  core with the recursive import fence`.

### Task 6: fakedx101 EX — the B-projection with teeth; the gate goes to TEN

**Files:** create `internal/fakedx101/{transcription-b.csv,
PROVENANCE.md,gen/main.go,gen/main_test.go,ex.go,exinventory_gen.go}`,
`core/transport/ex_crosscheck_ftdx101_test.go`.

- [ ] COPY `core/cat/ftdx101/testdata/transcription-b.csv` →
  `internal/fakedx101/transcription-b.csv` (copy-not-move — the
  dialect's copy is bound by `crosscheck_test.go:106`); PROVENANCE.md
  records source path, the copy-not-move rule, the shared SHA-256 at
  copy time, values-free tool-derived status, GPL treatment (the
  fakedx10 PROVENANCE.md shape).
- [ ] `gen/`: stdlib-only generator (fakedx10 gen's shape: `-csv`,
  `-out`, pinned B header, textWidth 12, `Up to` discriminator) —
  FORBIDDEN from importing `internal/extable` or anything
  project-internal; **red proof that Task 5's fence FIRES on a scratch
  project-internal import inside `gen/`** (recorded, reverted);
  `ex.go` with the `//go:generate` directive
  (`go run github.com/gm5dna/open-rig-programmer/internal/fakedx101/gen -csv transcription-b.csv -out exinventory_gen.go`)
  and the fakeradio value convention (zeros/spaces) stated in doc.go;
  generate; commit the generated file. Expected group shape: 18
  subgroups, 193 items — a generator refusal here is a defect, never
  an edit to the CSV.
- [ ] Staleness test (gen/main_test.go): recompute from the CSV
  through the real pipeline, byte-compare; RED PROOF: a scratch
  one-row divergence fires it (recorded, reverted).
- [ ] `core/transport/ex_crosscheck_ftdx101_test.go` (the
  ex_crosscheck_ftdx10_test.go shape): address sets identical both
  directions between `ftdx101.DialectD().EXItems()` and
  `fakedx101.EXDefaults()`; widths/shapes agree (the one Text item
  named); wire round-trip of ALL addresses against a NewD radio AND
  the ID-divergence leg — the same EX read against a NewMP radio
  answers identically (the inventory is shared; only ID differs);
  out-of-inventory refused ("050101" both radios); **the copied-CSV
  byte-identity test** (`TestFTdx101TranscriptionBCopy_...`, the
  `:293-297` constants shape). RED PROOF: a deliberate one-row
  divergence fires the cross-check (recorded, reverted).
  Pre-verified at plan time: this import placement violates no guard
  (composition rule 1 is core/driver-only; simulated-tokens walks
  non-test files; transport tests importing fakes is standing
  precedent — ex_crosscheck_ftdx10_test.go). If a guard fires anyway,
  STOP.
- [ ] Gate: as Task 5 + `go test ./core/transport/ -count=1` + regen
  idempotence ×2 now including the fakedx101 generator
  (`go generate ./internal/fakedx10/... ./internal/fakedx101/...
  ./core/cat/...` twice, tree clean after both) + the golden gate NOW
  TEN PATHS (both invocations in Global Constraints). Commit: `M9d-2
  task 6: the fakedx101 EX projection — copy, generator, staleness,
  cross-check; the golden gate is ten paths`.

### Task 7: registration — sentinels swept, presence pins first, then wiring, radiotext, guards, UI pins

**Files:** modify `internal/wiring/{wiring.go,fake.go,wiring_test.go}`,
`internal/radiotext/{radiotext.go,radiotext_test.go}`,
`internal/guards/simulated_tokens_test.go`, `app/uispec_test.go`,
`core/cat/ftdx10/doc.go` + positional-citation sites,
`app/app_test.go` and other stale-prose sites found by the sweep.

- [ ] **The sentinel/collision sweep FIRST, as a GREP-VERIFIED
  ENUMERATION** (M9c-6 found 11 sites against the plan's 5): grep
  `FTdx101|FTDX101|ftdx101` across `cmd/ app/ internal/ core/`
  (the frontend lives under `app/frontend/` and is covered; excluding
  `core/cat/ftdx101` itself and generated
  files); classify EVERY hit — (a) registration surface this task
  edits, (b) dialect/extable prose that stands (the
  `"FTdx101D/MP"` inventory model string at
  `internal/extable/profile.go:370` and `TestModelSlug`'s
  `{"FTDX101D/MP", "ftdx101d-mp"}` row STAND — they are the joint
  inventory form, not the registry keys), (c) a test using the names
  as a negative fixture — class (c) must be EMPTY or migrated before
  anything registers. Record the enumeration in the task report.
- [ ] **Presence pins land FIRST and RED** (M9c-6 red proof 4):
  extend `TestSupportedModels_ContainsEveryRegisteredModel`'s want
  list (`wiring_test.go:615`) to
  `{"FT-710", "FTdx10", "FTdx101D", "FTdx101MP"}` with
  constant-spelling asserts for the two new constants; run — FAIL.
  Then register.
- [ ] `wiring.go`: `const FTdx101DModel = "FTdx101D"` /
  `FTdx101MPModel = "FTdx101MP"` (doc per the FTdx10Model precedent);
  two `realDrivers` rows → `NewFTdx101DRealDriver`
  (`ftdx101.NewD(ftdx101.RealHardware)`) and the MP sibling, doc
  recording writeTrialsComplete=false ⇒ read/probe-only.
  `fake.go`: TWO typed option vars —
  `var FTdx101DFakeSessionOpts []fakedx101.Option` and
  `var FTdx101MPFakeSessionOpts []fakedx101.Option` (spec A6: no
  option leakage across siblings; the doc comment says a D test
  setting options must not steer an MP session) — and two
  `fakeDrivers` rows whose `newRadio` closures TEXTUALLY call
  `fakedx101.NewD(FTdx101DFakeSessionOpts...)` /
  `fakedx101.NewMP(FTdx101MPFakeSessionOpts...)` in this file (the
  guard's AST-walk requirement, fake.go:134-166); the `fakeRadio`
  compile assertion for `*fakedx101.Radio`.
- [ ] `internal/guards/simulated_tokens_test.go`: TWO rows —
  `{"ftdx101", "Simulated", "fakedx101.NewD", "internal/fakedx101"}`
  and `{"ftdx101", "Simulated", "fakedx101.NewMP",
  "internal/fakedx101"}` (one driver package, one token, two ctor
  calls that must both live in wiring's one file). Red-proven in Task
  9's index (a scratch second Simulated reference).
- [ ] `internal/radiotext/radiotext.go`: `ftdx101dText` and
  `ftdx101mpText` — ALL EIGHT fields
  (`EraseProcedure`, `FirmwareGuidance`, `ToneScanSkipNote`,
  `ToneScanSkipVerification`, `EraseDialogNote`,
  `PreservationTooltips{Tone,ScanSkip}`, `FirmwarePlaceholder`,
  `ProbeFirmwareNote`), the honesty rule (`radiotext.go:130-149`'s
  discipline: nothing invented; no FT-710/FTdx10 particulars);
  `EraseProcedure`/`FirmwareGuidance`/`ProbeFirmwareNote` non-empty,
  `ToneScanSkipVerification` empty while writeTrialsComplete is false
  (the ftdx10Text:188 justification shape); **`ProbeFirmwareNote`
  carries the §3.12 two-port caveat** (Enhanced COM for CAT vs
  Standard COM for TX control; a radio on the wrong port answers
  nothing — layout 75-79); D and MP prose identical except where it
  names the model (D8). Two map rows (bare string keys — this package
  must not import wiring). Tests: verbatim pins for both; the
  differ-from-FT-710 and differ-from-FTdx10 loops; a D-vs-MP loop
  over the model-naming fields only; `TestFor_UnknownModel`
  near-misses extended with `"FTDX101D"`, `"FTDX101MP"`,
  `"ftdx101d"`, `"FTdx101"`, `"FTdx101D "`, `"FT-DX101D"` (the report
  notes `"FTDX101D"` is a live collision with the manual's spelling —
  exactly why it must stay unknown).
- [ ] `wiring_test.go` additions: `TestModelSlug` rows
  `{"FTdx101D", "ftdx101d"}` and `{"FTdx101MP", "ftdx101mp"}`;
  `TestSynthesiseDiscoveredBanks_FTdx101DMatchesDriver` (+MP) — the
  FTdx10 sibling's shape (`wiring_test.go:837`);
  `TestOpenFakeSessionFor_FTdx101DOptionSourceIsItsOwn` (+MP) proving
  each var steers ONLY its own model's session (set a slot option in
  D's var, open MP, assert unaffected — the no-leakage pin);
  the task-3-deferred end-to-end test: Simulated-profile MT-only
  write through `OpenFakeSessionFor(ctx, "FTdx101D")` (probe → read →
  write → read-back), and the MP sibling;
  `TestOpenRealSessionFor_BaudIsTheDriversDefault`'s doc comment
  updated (it names models). E5
  (`TestOpenFakeSessionFor_EveryRegisteredModel`) picks the rows up
  automatically — verify the crossed-pairing leg
  (`Identity().CATID == caps.CATID`, wiring_test.go:138) now guards
  the D/MP pairing too (a swapped fakeDrivers row fails it).
- [ ] `app/uispec_test.go`: membership want map
  (`uispec_test.go:314-317`) gains `"FTdx101D"` and `"FTdx101MP"`,
  both mapping to the six-field combined-MT core (reuse
  `ftdx10CoreSix` with a comment citing matrix §2.1 — the shape is
  the combined record's, shared by construction); bankReadOnly class:
  the `TestBankReadOnly_RegisteredFTdx10_RealHardwareProfile` sibling
  run FOR EACH new model independently — static AND live/discovered
  bank behaviour per sibling (the `app/uispec_test.go:341-404` shape),
  with a D-vs-MP DeepEqual over the resulting bankReadOnly maps as a
  SUPPLEMENTAL assertion only (spec A6 says both models, and an
  equality proof alone would let a shared wrong answer pass);
  TagDisplayDefault class + GetUISpec-through-the-
  registered-fake (`TestGetUISpec_RegisteredFTdx10_EveryBankUnavailable`
  sibling) for both models — every bank serves `{unavailable}`
  (matrix §3.7).
- [ ] **Stale-prose sweep + the M9d-1 forward items — PROSE/COMMENT
  ONLY, in its OWN commit.** Repo-wide grep for prose registration
  falsifies (the `app/app_test.go:180-184` class). Then the two
  forward items the M9d-1 MILESTONE ADJUDICATION assigned to M9d-2
  (record: `.superpowers/sdd/m9d1-execution-ledger.md`, milestone fix
  wave — orchestrator-held; the same authority's prose amendments to
  ftdx10 files already landed in tracked commit `de547b6`):
  `core/cat/ftdx10/doc.go` gains its SEVENTH register entry — the
  0x2D minus-direction byte, mirroring `core/cat/ftdx101/doc.go`'s
  seventh (same inheritance, same lift wording, THAT manual's own
  citation for the glyph) — and every positional register citation
  ("entry 3", "entry 6") in `core/driver/ftdx10/*.go` and
  `core/cat/ftdx10/*` becomes a citation BY NAME (positional
  citations break on reorder/insert — as this very insertion proves).
  **Scope note, adjudicated at plan review:** spec decision 4 ("the
  FTdx10 packages are not reopened") is the no-family-refactor rule;
  it does not bar milestone-mandated comment corrections. This step
  changes NO code, NO generated file, NO golden — the commit's
  `git diff` must show comment/doc lines only, and the orchestrator
  verifies that at landing.
- [ ] Gate: full `go test ./internal/... ./core/... ./cmd/... ./app/
  -count=1` (cmd/rigprog EXPLICITLY), gofmt, vet, ten-path golden
  gate. Commit: `M9d-2 task 7: FTdx101D and FTdx101MP registered —
  presence pins, wiring, radiotext, guards, UI pins, the prose sweep`.

### Task 8: the caps-aware CHIRP blank-Skip fold (spec decision 5, sanctioned)

**Files:** modify `core/spec/support.go`, `core/csvio/chirp.go`,
`core/csvio/chirp_test.go`, `app/uispec.go` (the
`bankTagDisplayDefault` refactor onto `Unreachable()`, plus its
comment), `app/frontend/src/lib/grid/columns.js` (comment only),
`app/frontend/src/lib/ChannelGrid.svelte` (comment only).

- [ ] `core/spec/support.go`: add

```go
// Unreachable reports whether this field is Unsupported in BOTH
// directions — the radio's protocol is not believed to reach it at all,
// as opposed to reaching it unverifiedly (Unverified) or inertly
// (Inert). Callers use it to decide whether an externally supplied
// value for the field can even be a claim about this radio: see
// core/csvio's CHIRP skip/tag-display construction and app's
// tag-display default.
func (fs FieldSupport) Unreachable() bool {
	return fs.Read == Unsupported && fs.Write == Unsupported
}
```

  and refactor `chirpTagDisplay` (chirp.go:367-373) and
  `bankTagDisplayDefault` (app/uispec.go:175-181) onto it,
  behaviour-identically (their pins hold un-edited); update the
  third-caller note at chirp.go:353-360.
- [ ] **Write the failing tests first** in chirp_test.go,
  TABLE-DRIVEN OVER EVERY REGISTERED RADIO'S REAL CAPABILITIES (the
  spec's "per-radio tests": `wiring.StaticCapabilities` for
  `FT-710`, `FTdx10`, `FTdx101D`, `FTdx101MP` — all four scan-skip
  Unreachable): for each, blank Skip → `{State: Unknown}` with NO
  loss entry; `"S"` → `{State: Unknown}` plus a NON-BLOCKING
  `LossEntry{Column: "Skip", Value: "S", Action: ActionDropped,
  Blocking: false}` whose Detail is EXACTLY
  `CHIRP Skip "S" dropped: scan-skip is not reachable over CAT on
  <Model>; scan-skip left unresolved`; unrecognised values (`"P"`)
  unchanged (today's arm). (If a wiring import into core/csvio's
  tests trips a guard, mirror the four capability values as fixtures
  instead, each citing its driver's caps site — the behaviour table
  is the requirement, not the import.) Plus, against
  `writableCapabilities` (scan-skip rw, chirp_test.go:625-651),
  blank → `{Known,false}` and `"S"` → `{Known,true}` UNCHANGED — the
  future-radio branch pinned. Run — FAIL.
- [ ] Implement `chirpScanSkip` in chirp.go, called from the :609
  switch site (caps and memBank already in scope, :382/:404):

```go
// chirpScanSkip converts a CHIRP Skip cell to this project's ScanSkip
// field state, CAPABILITY-AWARE (M9d-2, spec decision 5): on a radio
// whose scan-skip is Unreachable — today, EVERY registered radio — a
// blank cell yields Unknown (not Known-false: Known admits the field
// into diff's requestedFields, which blocked every CHIRP-imported
// channel on such radios — the M9c-6 manifest's A7 finding), and an "S"
// cell yields Unknown plus a NON-BLOCKING loss entry recording the
// dropped intent. A radio with genuine skip support keeps the literal
// reading: blank means Known-false, "S" means Known-true. Unrecognised
// values are unchanged in both worlds.
func chirpScanSkip(caps spec.Capabilities, bank spec.BankID, raw string, line int) (codeplug.BoolField, []LossEntry)
```

  with exactly the branch behaviour the tests pin (the unrecognised
  arm shared, byte-identical Detail to today's
  `CHIRP Skip value %q has no %s equivalent; scan-skip left
  unresolved`). Run — PASS.
- [ ] Update the three moved fixture assertions on `chirp_sample.csv`
  (chirp_test.go:774-775 blank→Unknown, :791-792 S→Unknown+loss,
  :1030-1034 unchanged) and any import-count assertions they feed.
- [ ] **Consumer enumeration, each verified in this task's report**
  (the spec's list, resolved against the mapped sites): `fieldstate.go`
  — `{Unknown}` already Valid (fieldstate.go:118-122), no change;
  `validate.go:331-336` — shape-only, no change; diff membership
  (diff.go:227-229) — Unknown no longer admitted, THE INTENDED
  effect; diff whole-struct state (diff.go:180-182, :435) — imported
  codeplugs SAVED UNDER THE OLD CONSTRUCTION reclassify against new
  imports (a data-compat note in the manifest, not a code change);
  both drivers' write gates — mechanics unchanged, fewer Known fields
  arrive; JSON + digest (digest.go:79-103) — `{Known,false}` vs
  `{Unknown}` marshal differently, so CHIRP-derived digests change
  (designed delta, Task 9); native CSV (export.go:99-113/:144,
  import.go:111-130) — all three states already round-trip; blank
  export cell where `no` was (designed delta); frontend — behaviour
  unchanged (skip stays Known-only editable: columns.js:94-95,
  ChannelGrid.svelte:419), but the TWO rationale comments justifying
  it by "the protocol cannot write them at all" (columns.js:74-81,
  ChannelGrid.svelte:382-387) are reworded to the caps-derived
  rationale — comment-only, no behavioural edit; clone verification
  (execute.go:116-123) — ScanSkip excluded unconditionally, no
  change, noted. **There is NO CHIRP export path**
  (`core/csvio/doc.go:15-21`; only native `Export` exists) — the
  spec's "import/export" consumer resolves to import-derived surfaces
  only, recorded as such.
- [ ] Gate: `go test ./core/... ./cmd/... ./app/ -count=1`, frontend
  check/test/build, gofmt, vet, ten-path golden gate. Commit: `M9d-2
  task 8: the caps-aware CHIRP blank-Skip construction —
  spec.FieldSupport.Unreachable and the sanctioned delta`.

### Task 9: byte identity, the designed deltas, per-sibling baselines, full gate

**Files:** create `docs/superpowers/m9d2-baseline-manifest.md`.

- [ ] **The M9c-6 recipe by the manifest's OWN tables** (the M9d-1
  baseline-note method, `docs/superpowers/m9d1-baseline-note.md`):
  mirror trees, both binaries where the recipe says so. **Before
  running anything, classify EVERY recorded row as EXPECTED-IDENTICAL
  or DESIGNED-DELTA, from this list and nothing else:**
  - *model-list class* (M9c-6 Part 2 + the same class Part 1
    touches): bare invocation, `help`, `nosuchcmd`,
    `UnknownModelError.Supported`'s rendered text
    (cmd/rigprog/wiring.go:107-117), top usage (usage.go:43),
    `GetSupportedModels` — gain exactly the two rows;
  - *CHIRP-skip class* (Task 8), AT ROW LEVEL — these rows and no
    others (adjudicated at plan review against the manifest's own
    tables and `cmd/rigprog/import.go`'s stream layout: the loss
    report prints to STDOUT; a blank Skip cell produces NO loss entry
    under either construction, so clean-import stdout cannot move;
    export stdout prints only row count + path):
    Part 1 row 7 `import.stdout` (chirp_sample carries an `S` row —
    it gains the dropped-loss line) and row 18 `import-min.json`;
    Part 3 stdout legs **9, 10, 11** (skip-fixture import; both diff
    verdicts, `Blocked 3` → `Blocked 0`); Part 3 artefacts
    `ftdx10-import-chirp.json`, `ftdx10-import-skip.json`,
    `ftdx10-import-chirp.csv` (states, digests, `no`/`yes` →
    blank cells);
  - everything else: ZERO diffs, byte-identical — EXPLICITLY
    including Part 1 rows 8-9 and 14-16, Part 3 legs 8, 12, 13 and
    14, and every FT-710 native-export row.
  Any row moving outside its class is a STOP — and a row INSIDE the
  CHIRP class that re-runs identical is recorded as identical, never
  waved through: every diff that does occur must be matched to its
  named cause (the specific construction change that produced it)
  before it is accepted. Record every designed-delta row VERBATIM
  (old value, new value) in the new manifest.
- [ ] **Deltas proven by REMOVAL, never inferred from a hash:** in a
  scratch worktree, revert Task 8's commit → the CHIRP-class rows
  reproduce M9c-6's recorded values; revert Task 7's registration
  commit → the model-list rows reproduce M9c-6's. Both reverts'
  re-runs recorded, then discarded.
- [ ] **First fake-only baselines for EACH sibling** — the M9c-6
  Part 3 FOURTEEN-leg class per model
  (probe / read / read --settings / diff-fresh / settings /
  settings --csv / export / import --chirp clean / import --chirp
  skip / diff-chirp-clean / diff-chirp-skip / import --csv round
  trip / export-of-chirp-import / export-of-round-trip), exit codes,
  stdout hashes, and the nine-entry file-artefact table per model,
  recorded as `FTdx101D` and `FTdx101MP` sections. Expected internal
  checks: 117 channels, 193 settings, `ftdx101-ex@1`, `Blocked 0` on
  the fresh-read diff, and the CHIRP legs already caps-aware (born
  under Task 8's construction — no delta to carry).
- [ ] **Red-proof index** (the M9c-6 Part 4 discipline): every guard,
  pin and cross-check this milestone added shown to FIRE — the
  presence pin (full removal of one registration), table agreement
  (one-table removal), keys-vs-Model mismatch, radiotext deletion,
  the TWO simulated-tokens rows with THREE mutations — a scratch
  second `Simulated` reference (fires the cardinality clause), then
  SEPARATELY a rename of the textual `fakedx101.NewD` call and of the
  `fakedx101.NewMP` call, one token reference left intact each time
  (the guard `continue`s past the ctor clause on a cardinality
  failure, `internal/guards/simulated_tokens_test.go:137-143`, so
  only a ctor-specific mutation proves each ctor clause bites) —
  the fakedx101 fence + staleness + cross-check (Task 5/6
  reds cited), the wrong-sibling both-direction tests, the
  ID-only-text pin (scratch edit to Error()), the option-var
  no-leakage pin. Each with its commit and revert.
- [ ] **Full local gate at tip:** gofmt; `go build ./...`;
  `go vet ./...`; `go test ./... -count=1` foreground; `go test -race
  ./core/... -count=1` backgrounded WITH ITS EXIT STATUS COLLECTED
  and required 0 before the commit; `go test ./internal/guards/ -v`
  (all guards by name); frontend check/test/build; regen idempotence
  ×2 across ALL FIVE generators —
  `go generate ./internal/fakedx10/... ./internal/fakedx101/...
  ./core/cat/...` twice AND `wails generate module` from `app/`
  twice, tree clean after every pass
  (`git diff --exit-code -- app/frontend/wailsjs` included), with the
  wails run's non-vacuity shown (the M9c-6 mtime method); the
  TEN-path golden gate as one invocation and as ten.
- [ ] Commit the manifest. Milestone then goes to the standing
  parallel Codex + Fable milestone review before merge (`--no-ff`).

## Self-review (revision 2)

**Review folds (all merged Codex + Fable findings):** C1 digit
hygiene (the class phrase reworded, the digit-sweep added to Task 9);
C2 per-model write-guard constants (Task 2, File Structure); C3
adjudicated — the FTdx10 prose step stands re-scoped
prose/comment-only in its own commit with provenance and the
decision-4 boundary stated (Task 7); C4 labels + NoBlank transcribed
and asserted (Task 2); C5+F1 the designed-delta classes replaced by
the exact row table with the named-cause rule (Task 9); C6 the three
guard mutations (Task 9); C7 malformed-discovery refusal tests
(Task 2); C8+F5 per-registered-radio CHIRP tables and per-sibling
static+discovered UI classes with equality supplemental (Tasks 7-8);
C9 the defensive-copy mutation tests (Task 4); C10 the wails ×2 gate
made explicit (Task 9); C11+F4 citation corrections
(`profile.go:370`, `ChannelGrid.svelte:382-387`, the D1 three-site
citation, the sweep roots); F2 the uispec.go annotation; F3 the
fakedx10-doc provenance claim reworded.

**Spec coverage:** A4 matrix consumed as the capability authority
(Global Constraints, Task 2) ✓; one parameterised driver, both
registrations, every field explicit, writeTrialsComplete per model,
MT-only reads, full-range discovery with budget, MT-only writes,
TagDisplay Unavailable, settings sized by the real count (193) → Tasks
2-4 ✓; A5 both-names refusal with pinned shared-type text, Is/As,
no-discovery, closure, both directions → Tasks 1-2 ✓; fakedx101 with
generated inventory, staleness, cross-check, import fence, register,
fault-omission note → Tasks 5-6 ✓; registration with presence pins
FIRST, wiring rows, per-model radiotext (non-empty trio,
ToneScanSkipVerification empty, §3.12 port note), simulated-tokens
rows, sentinel sweep as grep-verified enumeration → Task 7 ✓; CHIRP
blank-Skip per decision 5 with the FULL consumer list resolved
(fieldstate, validate, diff membership+state, both write gates,
JSON+digest, native CSV, frontend render/editability, clone-verify
exclusion; CHIRP export nonexistent, recorded) → Task 8 ✓; byte
identity with the two delta classes (model lists incl.
`UnknownModelError.Supported`; CHIRP skip), removal-proven, golden
gate eight→TEN (Task 6), per-sibling fourteen-leg baselines, E5
auto-pickup, per-model UI pins, two FakeSessionOpts vars, D-vs-MP
identity pins → Tasks 6-9 ✓; M9d-1 forward items (ftdx10 0x2D seventh
entry, de-positionalised citations) → Task 7 ✓; error handling
(malformed answers refuse; currentCaps fallback untouched — non-goal)
✓; non-goals untouched (GUI picker cluster, FTX-1, menu writes, no
writeTrialsComplete flip) ✓.

**Placeholders:** none — every new exported identifier, format
string, Detail string, table row, and test obligation is spelled out;
radiotext prose is constrained by field list, honesty rule, required
port note and D8 rather than inlined draft prose, per the M9c-6 plan
precedent (its per-task review enforces the honesty rule on the
actual words).

**Type consistency:** `WrongRadioError.WantModel/GotModel` (Task 1)
are what Task 2's refusal populates and Task 7's probe tests consume;
`ftdx101.NewD/NewMP(profile Profile, opts ...Option) driver.Driver`
(Task 2) are what wiring's rows call (Task 7);
`fakedx101.NewD/NewMP(opts ...Option) *Radio` (Task 5) are what
fake.go's closures and the guard rows name (Task 7);
`spec.FieldSupport.Unreachable()` (Task 8) is consumed by
`chirpScanSkip`, `chirpTagDisplay`, `bankTagDisplayDefault`;
`ftdx101-ex@1` (Task 4) is what Task 9's settings legs expect; the
golden-gate path lists in Global Constraints, Task 6 and Task 9
agree (eight → ten).

**Ordering:** 1 (shared type, smallest) → 2-4 (driver, sequential) →
5-6 (fake; fence before gen) → 7 (registration; needs driver + fake) →
8 (CHIRP fold; before baselines so the new models' first baselines are
born caps-aware) → 9 (identity + baselines + gate, last).

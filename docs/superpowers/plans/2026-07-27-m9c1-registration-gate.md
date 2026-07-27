# M9c-1 Second-Model Registration Gate — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove every place a model-neutral package decides something by knowing it is the FT-710, so a second radio model can register without being silently wrong.

**Architecture:** `spec.Capabilities` gains semantic vocabulary (`ShiftOption{Value,Direction}`, `ToneState.Encodes/.Decodes`) so `core/csvio/chirp.go` can ask "which of *this* radio's values means +shift / encode-only" instead of hardcoding FT-710 literals. `radiotext` gains the last hardcoded prose string plus a test forcing every registered model to have an entry. Three wiring/snapshot items close the remaining registration hazards.

**Tech Stack:** Go 1.x (stdlib only in `core/spec`, `core/csvio`, `internal/radiotext`), Svelte 5 + Wails v2 for the one frontend touch, `go test` throughout.

**Spec:** `docs/superpowers/specs/2026-07-27-m9c1-registration-gate-design.md` (commit `246d8d1`)

## Global Constraints

- **Branch:** `m9c1-registration-gate` off `main` (`ce7fcee`), merged `--no-ff` at the end.
- **British English** in all prose, comments and user-facing strings.
- **FT-710 byte-identity is the acceptance bar.** Every `LossEntry.Detail`, CLI stdout/stderr byte and exit code must be unchanged for the FT-710 unless the change is explicitly enumerated in Task 4.
- **`core/spec`, `core/csvio` and `internal/radiotext` import stdlib only** (plus `core/codeplug`+`core/spec` for csvio). Never `core/cat`, never a driver package.
- **Never regenerate a golden.** `core/cat/testdata/` must stay at exactly two commits (`ff5c19b`, `1d38941`).
- **`go test -race ./core/...` exceeds a 10-minute foreground limit — run it in the background.**
- **Check the `wailsjs` drift diff from the REPO ROOT**, not after `cd app`.
- Commit after every task. Do not squash.

---

### Task 1: Vocabulary semantics in `core/spec`, and every consumer

The `ShiftOptions` type change does not compile until its consumers follow, so type, standards, invariants and all consumers are **one task and one commit**.

**Files:**
- Modify: `core/spec/vocab.go` (add `ShiftDirection`, `ShiftOption`; change `standardShiftOptions`/`StandardShiftOptions`; extend `ToneState` and `standardCTCSSStates`)
- Modify: `core/spec/capabilities.go:48` (`ShiftOptions []string` → `[]ShiftOption`)
- Modify: `core/spec/validate.go:181` (+ new `shiftOptionValues` helper and two invariant blocks)
- Modify: `core/codeplug/validate.go:339-343`
- Modify: `core/driver/ft710/ft710.go:527`
- Modify: `app/uispec.go:229`
- Test: `core/spec/vocab_test.go`, `core/spec/validate_test.go`, `core/codeplug/validate_test.go:399-404`, `core/driver/ft710/ft710_test.go:349-384`

**Interfaces:**
- Consumes: nothing (first task).
- Produces: `spec.ShiftDirection` with constants `spec.ShiftNone`, `spec.ShiftUp`, `spec.ShiftDown`; `spec.ShiftOption{Value string; Direction ShiftDirection}`; `spec.StandardShiftOptions() []ShiftOption`; `spec.ToneState{Value string; RequiresTone, Encodes, Decodes bool}`. Task 3 relies on all of these.

- [ ] **Step 1: Write the failing tests for the three new `spec.Validate` invariants**

Add to `core/spec/validate_test.go`:

```go
func TestValidate_ShiftOptionsDuplicateDirection(t *testing.T) {
	c := validCapabilities()
	c.ShiftOptions = []ShiftOption{
		{Value: "SIMPLEX", Direction: ShiftNone},
		{Value: "PLUS", Direction: ShiftUp},
		{Value: "UP-ALSO", Direction: ShiftUp},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: two ShiftOptions share ShiftUp")
	}
	if !strings.Contains(err.Error(), "same direction") {
		t.Errorf("Validate() error = %q, want it to mention \"same direction\"", err)
	}
}

func TestValidate_CTCSSStatesDuplicateEncodeDecodePair(t *testing.T) {
	c := validCapabilities()
	c.CTCSSStates = []ToneState{
		{Value: "OFF", RequiresTone: false, Encodes: false, Decodes: false},
		{Value: "ENC", RequiresTone: true, Encodes: true, Decodes: false},
		{Value: "ENC-AGAIN", RequiresTone: true, Encodes: true, Decodes: false},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: two ToneStates share the encode/decode pair")
	}
	if !strings.Contains(err.Error(), "same encode/decode pair") {
		t.Errorf("Validate() error = %q, want it to mention \"same encode/decode pair\"", err)
	}
}

func TestValidate_CTCSSStatesRequiresToneInconsistent(t *testing.T) {
	c := validCapabilities()
	c.CTCSSStates = []ToneState{
		{Value: "OFF", RequiresTone: false, Encodes: false, Decodes: false},
		{Value: "BROKEN", RequiresTone: true, Encodes: false, Decodes: false},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: RequiresTone true but neither Encodes nor Decodes")
	}
	if !strings.Contains(err.Error(), "RequiresTone") {
		t.Errorf("Validate() error = %q, want it to mention RequiresTone", err)
	}
}
```

If `validCapabilities()` is not the existing helper name in that file, use whatever helper `core/spec/validate_test.go:48` builds — do not invent a second one.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./core/spec/ -run 'TestValidate_(ShiftOptionsDuplicateDirection|CTCSSStatesDuplicateEncodeDecodePair|CTCSSStatesRequiresToneInconsistent)' -v`
Expected: FAIL — compile error, `ShiftOption`/`ShiftNone`/`Encodes` undefined.

- [ ] **Step 3: Add the types and standards in `core/spec/vocab.go`**

Replace `standardShiftOptions` and `StandardShiftOptions` with:

```go
// ShiftDirection is the semantic content of a repeater shift option:
// which way the transmit frequency moves relative to receive, if at all.
// Generic code (a CSV importer mapping a foreign dialect's "+"/"-", the
// UI) needs this fact about a shift value; re-deriving it from the
// wire-form string would put radio vocabulary literals straight back into
// the neutral layer task 38 removed them from.
type ShiftDirection int

const (
	// ShiftNone is simplex: transmit and receive on one frequency.
	ShiftNone ShiftDirection = iota
	// ShiftUp transmits above the receive frequency.
	ShiftUp
	// ShiftDown transmits below the receive frequency.
	ShiftDown
)

// ShiftOption is one repeater shift value this radio's wire protocol
// expresses, paired with the semantic fact generic code needs about it —
// the same Value-plus-semantics shape ToneState uses, for the same
// reason.
type ShiftOption struct {
	// Value is the wire-form shift string, e.g. "SIMPLEX", "PLUS".
	Value string
	// Direction is which way this option moves the transmit frequency.
	Direction ShiftDirection
}

// standardShiftOptions is the repeater shift vocabulary shared across the
// radio family this project targets, in the FT-710 CAT manual's own P4
// order (SHIFT command).
//
// Unexported: callers get at it only through StandardShiftOptions (a
// fresh slice copy every call), matching StandardCTCSSTones' own pattern.
var standardShiftOptions = []ShiftOption{
	{Value: "SIMPLEX", Direction: ShiftNone},
	{Value: "PLUS", Direction: ShiftUp},
	{Value: "MINUS", Direction: ShiftDown},
}

// StandardShiftOptions returns a copy of the repeater shift vocabulary
// shared across the radio family this project targets — see
// standardShiftOptions for its provenance. Every call returns an
// independently-allocated slice, so a caller is free to mutate its own
// copy without affecting this package's data or any other caller's copy.
func StandardShiftOptions() []ShiftOption {
	out := make([]ShiftOption, len(standardShiftOptions))
	copy(out, standardShiftOptions)
	return out
}
```

Extend `ToneState` (keep its existing doc comment, add the two fields):

```go
	// Encodes is true iff a channel in this state TRANSMITS a CTCSS tone.
	Encodes bool
	// Decodes is true iff a channel in this state requires a matching
	// RECEIVED tone before it will open squelch.
	Decodes bool
```

And fill them in `standardCTCSSStates` (order unchanged — legacy literal order, task 38):

```go
var standardCTCSSStates = []ToneState{
	{Value: "OFF", RequiresTone: false, Encodes: false, Decodes: false},
	{Value: "ENC-DEC", RequiresTone: true, Encodes: true, Decodes: true},
	{Value: "ENC", RequiresTone: true, Encodes: true, Decodes: false},
}
```

- [ ] **Step 4: Change the field type in `core/spec/capabilities.go`**

At line 48, `ShiftOptions []string` becomes `ShiftOptions []ShiftOption`. Update its doc comment's example to read `e.g. {Value: "PLUS", Direction: ShiftUp}` and keep the "Typically built from StandardShiftOptions()" sentence.

- [ ] **Step 5: Add the invariants in `core/spec/validate.go`**

Add beside `validateVocab`:

```go
// shiftOptionValues returns the Value of every entry in opts, in order —
// so validateVocab can check a ShiftOption list with the same blank and
// duplicate rules it applies to every other vocabulary, without a
// []string being built by hand at the call site.
func shiftOptionValues(opts []ShiftOption) []string {
	values := make([]string, len(opts))
	for i, o := range opts {
		values[i] = o.Value
	}
	return values
}
```

Replace line 181 and add the two invariant blocks after the existing `CTCSSStates` `validateVocab` call:

```go
	problems = append(problems, validateVocab("ShiftOptions", shiftOptionValues(c.ShiftOptions))...)

	// Each ShiftDirection must be expressed by AT MOST ONE option:
	// core/csvio maps a foreign dialect's "+"/"-" by asking for the option
	// with a given Direction, and that question must have exactly one
	// answer. Two options sharing a direction would make the answer
	// depend on slice order.
	seenDirection := make(map[ShiftDirection]string, len(c.ShiftOptions))
	for _, o := range c.ShiftOptions {
		if prev, dup := seenDirection[o.Direction]; dup {
			problems = append(problems, fmt.Sprintf("ShiftOptions %q and %q express the same direction", prev, o.Value))
			continue
		}
		seenDirection[o.Direction] = o.Value
	}
```

```go
	// RequiresTone is derivable from Encodes/Decodes (a state that either
	// transmits or listens for a tone needs one), so it is CHECKED here
	// rather than left free to drift out of step with them. And, for the
	// same reason ShiftOptions' directions must be unique, each
	// encode/decode combination must name at most one state.
	type encodeDecodePair struct{ encodes, decodes bool }
	seenPair := make(map[encodeDecodePair]string, len(c.CTCSSStates))
	for _, ts := range c.CTCSSStates {
		if ts.RequiresTone != (ts.Encodes || ts.Decodes) {
			problems = append(problems, fmt.Sprintf("CTCSSStates %q has RequiresTone %t but Encodes %t and Decodes %t", ts.Value, ts.RequiresTone, ts.Encodes, ts.Decodes))
		}
		p := encodeDecodePair{ts.Encodes, ts.Decodes}
		if prev, dup := seenPair[p]; dup {
			problems = append(problems, fmt.Sprintf("CTCSSStates %q and %q express the same encode/decode pair", prev, ts.Value))
			continue
		}
		seenPair[p] = ts.Value
	}
```

Add three bullets to `Validate`'s doc comment (after the existing `CTCSSStates` bullet), matching the existing style:

```
//   - No two ShiftOptions may express the same ShiftDirection.
//   - No two CTCSSStates may express the same Encodes/Decodes pair.
//   - Every CTCSSStates entry's RequiresTone must equal Encodes||Decodes.
```

- [ ] **Step 6: Update `core/codeplug/validate.go:339-343`**

```go
	if !containsString(shiftOptionValues(caps.ShiftOptions), d.Shift) {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldShift, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: shift %q must be one of %s", slot, d.Shift, quotedList(shiftOptionValues(caps.ShiftOptions))),
		})
	}
```

`core/spec`'s `shiftOptionValues` is unexported, so add a local one beside `toneStateValues` (`core/codeplug/validate.go:78`), mirroring it exactly:

```go
// shiftOptionValues returns the Value of every entry in opts, in order —
// for building a caps-driven vocabulary list for an error message
// without re-deriving a []string by hand at the call site.
func shiftOptionValues(opts []spec.ShiftOption) []string {
	values := make([]string, len(opts))
	for i, o := range opts {
		values[i] = o.Value
	}
	return values
}
```

**The message text must not change.** For the FT-710 this still renders `shift "X" must be one of "SIMPLEX", "PLUS", "MINUS"`.

- [ ] **Step 7: Update `core/driver/ft710/ft710.go:527` and `app/uispec.go:229`**

`ft710.go:527`:

```go
	out.ShiftOptions = append([]spec.ShiftOption(nil), caps.ShiftOptions...)
```

`app/uispec.go:229` — replace the one-line copy with a flatten mirroring line 230's `CTCSSStates` loop:

```go
	shiftOptions := make([]string, len(caps.ShiftOptions))
	for i, o := range caps.ShiftOptions {
		shiftOptions[i] = o.Value
	}
```

Extend the comment block above it: `ShiftOptions extracts each ShiftOption's Value, preserving caps' own order; the Direction each option also carries is not needed by the grid's option list today.` **`app/types.go:352` `UISpecView.ShiftOptions` stays `[]string` — do not change it, and no wailsjs regeneration arises from this task.**

- [ ] **Step 8: Update the four test fixtures that set these fields as literals**

These are the complete set; a repo-wide `grep -rn 'ShiftOptions:\|CTCSSStates:\|ToneState{' --include='*.go' .` confirms no others.

`core/codeplug/validate_test.go:399-403` — the deviant-vocab fixture. Keep the deviant names (that is the whole point of the fixture) and make it invariant-consistent:

```go
		ShiftOptions: []spec.ShiftOption{
			{Value: "SPLIT-MINUS", Direction: spec.ShiftDown},
			{Value: "SPLIT-PLUS", Direction: spec.ShiftUp},
			{Value: "SPLIT-NONE", Direction: spec.ShiftNone},
		},
		CTCSSStates: []spec.ToneState{
			{Value: "DISABLED", RequiresTone: false, Encodes: false, Decodes: false},
			{Value: "TONE", RequiresTone: true, Encodes: true, Decodes: false},
		},
```

`core/driver/ft710/ft710_test.go:349-384` — the deep-copy test. Line 349 becomes `[]spec.ShiftOption{{Value: "SIMPLEX", Direction: spec.ShiftNone}, {Value: "PLUS", Direction: spec.ShiftUp}, {Value: "MINUS", Direction: spec.ShiftDown}}`; line 350's states gain `Encodes`/`Decodes` per `standardCTCSSStates`; the tamper lines (358, 360) become `ShiftOption` values; the assertions at 363-364, 380-381 compare `.Value`; and 369, 383 compare against `spec.ToneState{Value: "OFF", RequiresTone: false, Encodes: false, Decodes: false}`.

`core/spec/vocab_test.go:26,62,67` — `wantCTCSS` gains the two fields per `standardCTCSSStates`; the tamper at 62 and the assertion at 67 likewise. Add a `StandardShiftOptions` copy-independence test mirroring the `ToneState` one at 62-67 if the file does not already have one.

`core/spec/validate_test.go:245,252` — **these two subtests will now report an extra problem** (`RequiresTone: true` with neither `Encodes` nor `Decodes`) alongside the blank/duplicate problem they exist to test. Make them invariant-consistent so each tests exactly one thing:

```go
	c.CTCSSStates = []ToneState{{Value: "OFF"}, {Value: "", RequiresTone: true, Encodes: true}}
```
```go
	c.CTCSSStates = []ToneState{{Value: "OFF"}, {Value: "OFF", RequiresTone: true, Encodes: true}}
```

- [ ] **Step 9: Run the full affected suite**

Run: `go build ./... && go vet ./... && go test ./core/spec/ ./core/codeplug/ ./core/driver/... ./app/ -count=1`
Expected: PASS, including the three new invariant tests.

- [ ] **Step 10: Commit**

```bash
git add core/spec core/codeplug core/driver app/uispec.go
git commit -m "M9c-1 task 1: ShiftOption gains Direction, ToneState gains Encodes/Decodes

Makes ShiftOptions symmetric with CTCSSStates so core/csvio can ask which
of this radio's values means +shift or encode-only, rather than
hardcoding the FT-710's literals. spec.Validate gains three invariants:
unique ShiftDirection, unique encode/decode pair, and RequiresTone ==
Encodes||Decodes (now derivable, so checked rather than left to drift).

UISpecView.ShiftOptions stays []string; no frontend change."
```

---

### Task 2: `ImportCHIRP` takes Capabilities — slot space and tag width

**Files:**
- Modify: `core/csvio/chirp.go:244-269` (`sanitizeCHIRPName`), `:278-311` (`importCHIRPRow` signature + Location), `:526` (`ImportCHIRP` signature)
- Modify: `cmd/rigprog/import.go` (hoist caps above the import branch)
- Modify: `app/importexport.go` (hoist `currentCaps` above the import)
- Test: `core/csvio/chirp_test.go`

**Interfaces:**
- Consumes: `spec.Capabilities`, `spec.BankMemory` from Task 1's package (unchanged by Task 1).
- Produces: `csvio.ImportCHIRP(rd io.Reader, caps spec.Capabilities) ([]codeplug.Channel, LossReport, error)`; test helpers `ft710LikeCapabilities()` and `deviantCapabilities()` in `chirp_test.go`. Tasks 3 and 4 use all of these.

- [ ] **Step 1: Write the failing tests — a deviant radio's slot space and tag width**

Add to `core/csvio/chirp_test.go`. `ft710LikeCapabilities` is the fixture every existing test will be threaded onto in Step 4:

```go
// ft710LikeCapabilities mirrors the FT-710 fields ImportCHIRP consults
// (core/driver/ft710/caps.go). It is a hand-built fixture rather than the
// real driver's Capabilities because core/csvio sits BELOW core/driver in
// the import graph and must not depend on it, even in tests. Drift
// between this and the real driver is caught end-to-end by the CLI
// byte-identity baseline, which does use the real driver.
func ft710LikeCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	slots := make([]string, 0, 99)
	for i := 1; i <= 99; i++ {
		slots = append(slots, fmt.Sprintf("%03d", i))
	}
	return spec.Capabilities{
		Model: "FT-710",
		CATID: "0800",
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: "Memories", Slots: slots},
		},
		Modes:        []string{"LSB", "USB", "CW-U", "CW-L", "FM", "AM", "RTTY-U", "FM-N"},
		TagLen:       12,
		CTCSSTones:   tones[:],
		ShiftOptions: spec.StandardShiftOptions(),
		CTCSSStates:  spec.StandardCTCSSStates(),
	}
}

// deviantCapabilities is a radio that agrees with the FT-710 about
// NOTHING ImportCHIRP consults: 4 memory slots named differently, a
// 6-byte tag, renamed shift and CTCSS vocabulary, one mode. A chirp.go
// that threaded its caps parameter through and then ignored it would
// still pass every ft710LikeCapabilities test; only this fixture can tell
// the difference.
func deviantCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: "DEVIANT-1",
		CATID: "0001",
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: "Memories", Slots: []string{"M1", "M2", "M3", "M4"}},
		},
		Modes:      []string{"USB"},
		TagLen:     6,
		CTCSSTones: tones[:],
		ShiftOptions: []spec.ShiftOption{
			{Value: "SPLIT-NONE", Direction: spec.ShiftNone},
			{Value: "SPLIT-PLUS", Direction: spec.ShiftUp},
			{Value: "SPLIT-MINUS", Direction: spec.ShiftDown},
		},
		CTCSSStates: []spec.ToneState{
			{Value: "DISABLED", RequiresTone: false, Encodes: false, Decodes: false},
			{Value: "TONE-TX", RequiresTone: true, Encodes: true, Decodes: false},
			{Value: "TONE-BOTH", RequiresTone: true, Encodes: true, Decodes: true},
		},
	}
}

func TestImportCHIRP_SlotSpaceFromCaps(t *testing.T) {
	csv := "Location,Frequency,Mode\n2,145.500000,USB\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("ImportCHIRP: unexpected blocking entries: %+v", report.Entries)
	}
	if len(channels) != 1 {
		t.Fatalf("len(channels) = %d, want 1", len(channels))
	}
	if channels[0].Slot != "M2" {
		t.Errorf("Slot = %q, want %q (deviant bank's second slot, NOT the FT-710's \"002\")", channels[0].Slot, "M2")
	}
}

func TestImportCHIRP_LocationBeyondBankBlocks(t *testing.T) {
	csv := "Location,Frequency,Mode\n5,145.500000,USB\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if len(channels) != 0 {
		t.Errorf("len(channels) = %d, want 0: Location 5 is beyond the deviant radio's 4 slots", len(channels))
	}
	if !report.HasBlocking() {
		t.Fatal("HasBlocking() = false, want true for an out-of-range Location")
	}
}

func TestImportCHIRP_TagLenFromCaps(t *testing.T) {
	csv := "Location,Name,Frequency,Mode\n1,ABCDEFGHIJ,145.500000,USB\n"

	channels, _, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("len(channels) = %d, want 1", len(channels))
	}
	if got := channels[0].Data.Tag; got != "ABCDEF" {
		t.Errorf("Tag = %q, want %q (truncated to the deviant radio's TagLen 6, not the FT-710's 12)", got, "ABCDEF")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./core/csvio/ -run 'TestImportCHIRP_(SlotSpaceFromCaps|LocationBeyondBankBlocks|TagLenFromCaps)' -v`
Expected: FAIL — compile error, `ImportCHIRP` takes 1 argument.

- [ ] **Step 3: Thread capabilities through `chirp.go`**

Change the two signatures:

```go
func ImportCHIRP(rd io.Reader, caps spec.Capabilities) ([]codeplug.Channel, LossReport, error) {
```
```go
func importCHIRPRow(line int, colIndex map[string]int, record []string, caps spec.Capabilities) (*codeplug.Channel, []LossEntry) {
```

Update the call at `chirp.go:608` to pass `caps`.

Replace the Location block (`:294-311`) — note the message templates reproduce the FT-710's current text exactly:

```go
	// Location -> slot. No slot means no Channel can be built at all:
	// this is the one field whose failure drops the whole row rather
	// than just leaving a field unresolved. CHIRP Locations are 1-based
	// positions in the radio's main memory bank, so Location N is that
	// bank's Nth slot — the bank supplies both the range and the slot's
	// canonical wire form, neither of which this package may assume.
	memBank, haveMemBank := caps.Bank(spec.BankMemory)
	locRaw := cell("Location")
	locN, err := strconv.Atoi(strings.TrimSpace(locRaw))
	if err != nil {
		return nil, append(entries, LossEntry{
			Line: line, Column: "Location", Value: locRaw, Action: ActionUnsupported, Blocking: true,
			Detail: "Location is not a valid integer; cannot map to a memory slot",
		})
	}
	if !haveMemBank || len(memBank.Slots) == 0 {
		return nil, append(entries, LossEntry{
			Line: line, Column: "Location", Value: locRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("%s has no memory bank to import into", caps.Model),
		})
	}
	if locN < 1 || locN > len(memBank.Slots) {
		return nil, append(entries, LossEntry{
			Line: line, Column: "Location", Value: locRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("%s memory slots are %s-%s; Location is out of range", caps.Model, memBank.Slots[0], memBank.Slots[len(memBank.Slots)-1]),
		})
	}
	slot := memBank.Slots[locN-1]
```

Replace `sanitizeCHIRPName`'s signature and truncation (`:244`, `:260-267`):

```go
func sanitizeCHIRPName(line int, name string, caps spec.Capabilities) (string, []LossEntry) {
```
```go
	if len(b) > caps.TagLen {
		full := string(b)
		b = b[:caps.TagLen]
		entries = append(entries, LossEntry{
			Line: line, Column: "Name", Value: name, Action: ActionApproximated, Blocking: false,
			Detail: fmt.Sprintf("Name %q is %d bytes, exceeds the %s's %d-byte tag limit; truncated to %q", full, len(full), caps.Model, caps.TagLen, string(b)),
		})
	}
```

Also update its charset `LossEntry.Detail` (`:257`) to `fmt.Sprintf("Name contained a byte outside the %s tag charset (printable ASCII 0x20-0x7E, excluding ';'); replaced with a space", caps.Model)` and its doc comment to say "the radio's tag charset" and "truncated to caps.TagLen". Update the call at `:316` to pass `caps`.

- [ ] **Step 4: Thread `ft710LikeCapabilities()` through every existing test**

Every existing `ImportCHIRP(...)` call in `chirp_test.go` gains `, ft710LikeCapabilities()`; every `sanitizeCHIRPName(...)` call in `TestSanitizeCHIRPName` (`:762`) gains `, ft710LikeCapabilities()`. **No existing expectation changes** — that is the behaviour-preservation proof.

- [ ] **Step 5: Update the two call sites**

`cmd/rigprog/import.go` — move the `caps, err := wiring.StaticCapabilities(*model)` block (currently at `:263`) to above the `if *csvIn != ""` branch, and pass `caps` at `:211`. Keep its existing comment with it.

`app/importexport.go` — in `ImportCHIRP`, add `caps, _ := currentCaps(a.conn)` above the `csvio.ImportCHIRP` call at `:120` and pass it. The existing `caps, _ := currentCaps(a.conn)` at `:154` then reuses that variable rather than re-fetching; delete the duplicate declaration.

- [ ] **Step 6: Run the tests**

Run: `go build ./... && go test ./core/csvio/ ./cmd/rigprog/ ./app/ -count=1`
Expected: PASS — the three new tests and all 792 lines of pre-existing chirp tests.

- [ ] **Step 7: Commit**

```bash
git add core/csvio cmd/rigprog/import.go app/importexport.go
git commit -m "M9c-1 task 2: ImportCHIRP takes Capabilities; slot space and tag width from caps

CHIRP Location is now the Nth slot of caps' BankMemory rather than a
hardcoded 001-099 with %03d formatting, and tags truncate at caps.TagLen
rather than a literal 12. Detail templates are chosen to reproduce the
FT-710's current wording byte for byte.

Both call sites already had capabilities in scope; each needed only a
hoist."
```

---

### Task 3: CHIRP vocabulary from Capabilities

**Files:**
- Modify: `core/csvio/chirp.go:208-227` (`parseCHIRPTone`), `:336-345` (Mode), `:347-371` (Duplex), `:381-435` (Tone)
- Test: `core/csvio/chirp_test.go`

**Interfaces:**
- Consumes: `ImportCHIRP(rd, caps)`, `ft710LikeCapabilities()`, `deviantCapabilities()` from Task 2; `spec.ShiftUp`/`ShiftDown`/`ShiftNone`, `spec.ToneState.Encodes`/`.Decodes` from Task 1.
- Produces: unexported `shiftFor(caps spec.Capabilities, d spec.ShiftDirection) (string, bool)` and `toneStateFor(caps spec.Capabilities, encodes, decodes bool) (string, bool)` in `chirp.go`.

- [ ] **Step 1: Write the failing tests**

```go
func TestImportCHIRP_ShiftVocabFromCaps(t *testing.T) {
	csv := "Location,Frequency,Mode,Duplex\n1,145.500000,USB,+\n2,145.500000,USB,-\n3,145.500000,USB,\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("unexpected blocking entries: %+v", report.Entries)
	}
	want := []string{"SPLIT-PLUS", "SPLIT-MINUS", "SPLIT-NONE"}
	if len(channels) != 3 {
		t.Fatalf("len(channels) = %d, want 3", len(channels))
	}
	for i, w := range want {
		if got := channels[i].Data.Shift; got != w {
			t.Errorf("channels[%d].Data.Shift = %q, want %q (deviant vocabulary, not the FT-710's)", i, got, w)
		}
	}
}

func TestImportCHIRP_CTCSSVocabFromCaps(t *testing.T) {
	csv := "Location,Frequency,Mode,Tone,rToneFreq,cToneFreq\n" +
		"1,145.500000,USB,Tone,88.5,88.5\n" +
		"2,145.500000,USB,TSQL,88.5,88.5\n" +
		"3,145.500000,USB,,,\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("unexpected blocking entries: %+v", report.Entries)
	}
	want := []string{"TONE-TX", "TONE-BOTH", "DISABLED"}
	if len(channels) != 3 {
		t.Fatalf("len(channels) = %d, want 3", len(channels))
	}
	for i, w := range want {
		if got := channels[i].Data.CTCSS; got != w {
			t.Errorf("channels[%d].Data.CTCSS = %q, want %q (deviant vocabulary, not the FT-710's)", i, got, w)
		}
	}
}

func TestImportCHIRP_ModeAbsentFromCapsBlocks(t *testing.T) {
	// FM maps to the display name "FM", which deviantCapabilities does
	// not list — a radio that cannot express the mapped mode must refuse
	// the row, not write a mode it has no equivalent for.
	csv := "Location,Frequency,Mode\n1,145.500000,FM\n"

	_, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if !report.HasBlocking() {
		t.Fatalf("HasBlocking() = false, want true: %+v", report.Entries)
	}
}

func TestImportCHIRP_MissingShiftDirectionBlocks(t *testing.T) {
	caps := deviantCapabilities()
	caps.ShiftOptions = []spec.ShiftOption{{Value: "SPLIT-NONE", Direction: spec.ShiftNone}}

	_, report, err := ImportCHIRP(strings.NewReader("Location,Frequency,Mode,Duplex\n1,145.500000,USB,+\n"), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if !report.HasBlocking() {
		t.Fatalf("HasBlocking() = false, want true: a radio with no up-shift option must refuse a \"+\" row: %+v", report.Entries)
	}
}

func TestImportCHIRP_ToneNotInCapsChartBlocks(t *testing.T) {
	caps := deviantCapabilities()
	caps.CTCSSTones = []spec.Tone{670} // 67.0 Hz only

	_, report, err := ImportCHIRP(strings.NewReader("Location,Frequency,Mode,Tone,rToneFreq\n1,145.500000,USB,Tone,88.5\n"), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if !report.HasBlocking() {
		t.Fatalf("HasBlocking() = false, want true: 88.5 is not in this radio's chart: %+v", report.Entries)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./core/csvio/ -run 'TestImportCHIRP_(ShiftVocabFromCaps|CTCSSVocabFromCaps|ModeAbsentFromCapsBlocks|MissingShiftDirectionBlocks|ToneNotInCapsChartBlocks)' -v`
Expected: FAIL — deviant rows still get `"PLUS"`, `"ENC"`, etc.

- [ ] **Step 3: Add the two lookup helpers to `chirp.go`**

```go
// shiftFor returns the wire-form shift value caps uses for direction, and
// true, or ("", false) when this radio expresses no such shift. Capabilities
// with two options for one direction cannot reach here: spec.Validate
// rejects them, so the answer is unambiguous by construction.
func shiftFor(caps spec.Capabilities, d spec.ShiftDirection) (string, bool) {
	for _, o := range caps.ShiftOptions {
		if o.Direction == d {
			return o.Value, true
		}
	}
	return "", false
}

// toneStateFor returns the wire-form CTCSS state caps uses for the given
// encode/decode combination, and true, or ("", false) when this radio
// expresses no such state. As with shiftFor, spec.Validate guarantees at
// most one state per combination.
func toneStateFor(caps spec.Capabilities, encodes, decodes bool) (string, bool) {
	for _, s := range caps.CTCSSStates {
		if s.Encodes == encodes && s.Decodes == decodes {
			return s.Value, true
		}
	}
	return "", false
}

// capsHasTone reports whether t is in this radio's CTCSS chart.
func capsHasTone(caps spec.Capabilities, t spec.Tone) bool {
	for _, x := range caps.CTCSSTones {
		if x == t {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Make `parseCHIRPTone` consult caps**

```go
func parseCHIRPTone(s string, caps spec.Capabilities) (spec.Tone, bool) {
	deciHz, err := parseExactToneDeciHz(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	t := spec.Tone(deciHz)
	if !capsHasTone(caps, t) {
		return 0, false
	}
	return t, true
}
```

Update its doc comment: the chart is now "this radio's own CTCSS chart (`caps.CTCSSTones`)" rather than `spec.ValidTone`. Update both call sites (`:393`, `:412`) to pass `caps`, and `TestParseCHIRPTone` (`:709`) to pass `ft710LikeCapabilities()`.

**Do not delete `spec.ValidTone`.** It stops being used by `core/csvio`, but `core/codeplug/fieldstate.go:66` still depends on it — it is not newly-dead code.

- [ ] **Step 5: Rewrite the Duplex block (`chirp.go:347-371`)**

```go
	// Duplex -> Shift. Which wire value means "up", "down" or "none" is
	// this radio's own vocabulary, so it is asked for by DIRECTION rather
	// than named here.
	switch duplexRaw := cell("Duplex"); duplexRaw {
	case "", "off":
		v, ok := shiftFor(caps, spec.ShiftNone)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no simplex shift option", caps.Model),
			})
			break
		}
		data.Shift = v
		if duplexRaw == "off" {
			entries = append(entries, LossEntry{
				Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionDropped, Blocking: false,
				Detail: fmt.Sprintf("CHIRP \"off\" duplex mapped to %s; the distinction between \"no duplex configured\" and \"simplex\" is not representable", v),
			})
		}
	case "+", "-":
		dir, label := spec.ShiftUp, "up"
		if duplexRaw == "-" {
			dir, label = spec.ShiftDown, "down"
		}
		v, ok := shiftFor(caps, dir)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no %s-shift option", caps.Model, label),
			})
			break
		}
		data.Shift = v
	case "split":
		entries = append(entries, LossEntry{
			Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("split-frequency duplex (independent TX/RX frequencies) has no %s equivalent", caps.Model),
		})
	default:
		entries = append(entries, LossEntry{
			Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("unrecognised Duplex value %q", duplexRaw),
		})
	}
```

**Wording note:** the `"off"` entry's Detail previously said `mapped to SIMPLEX`; it now interpolates the radio's own value, which for the FT-710 renders `mapped to SIMPLEX` — byte-identical.

- [ ] **Step 6: Rewrite the Tone block (`chirp.go:381-435`)**

Replace each hardcoded state with a `toneStateFor` lookup. `""` → `toneStateFor(caps, false, false)`; `"Tone"` → `toneStateFor(caps, true, false)` reading `rToneFreq`; `"TSQL"` → `toneStateFor(caps, true, true)` reading `cToneFreq`. Each missing state is a blocking entry:

```go
			Detail: fmt.Sprintf("%s expresses no encode-only CTCSS state", caps.Model),
```

The tone-chart failure entries at `:406` and `:417` become:

```go
			Detail: fmt.Sprintf("tone frequency is not in the %s's CTCSS chart", caps.Model),
```

The `"DTCS", "Cross"` and `default` cases keep their shape; their Details substitute `caps.Model` for the `FT-710` literal (`:426`, `:433`). Preserve the existing comment at `:396-400` about `Known` versus the CAT write gap.

- [ ] **Step 7: Membership-check the mapped mode (`chirp.go:336-345`)**

```go
	// Mode. chirpModeMap is CHIRP-dialect knowledge — which of this radio
	// family's display modes a CHIRP mode name should become — and stays
	// here. Whether the radio actually HAS that mode is caps' question,
	// and a mapped mode the radio lacks blocks rather than being written.
	modeRaw := cell("Mode")
	mapped, mappable := chirpModeMap[modeRaw]
	switch {
	case !mappable:
		entries = append(entries, LossEntry{
			Line: line, Column: "Mode", Value: modeRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("CHIRP mode %q has no %s equivalent", modeRaw, caps.Model),
		})
	case !containsMode(caps, mapped):
		entries = append(entries, LossEntry{
			Line: line, Column: "Mode", Value: modeRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("CHIRP mode %q maps to %q, which %s does not support", modeRaw, mapped, caps.Model),
		})
	default:
		data.Mode = mapped
	}
```

with

```go
// containsMode reports whether caps lists the given display-name mode.
func containsMode(caps spec.Capabilities, mode string) bool {
	for _, m := range caps.Modes {
		if m == mode {
			return true
		}
	}
	return false
}
```

Update `chirpModeMap`'s doc comment (`:121-123`): it maps to "this radio family's display-name equivalent", and the result is membership-checked against `caps.Modes`.

- [ ] **Step 8: Run the tests**

Run: `go test ./core/csvio/ -count=1 -v`
Expected: PASS — all five new tests plus every pre-existing test unchanged.

- [ ] **Step 9: Commit**

```bash
git add core/csvio
git commit -m "M9c-1 task 3: CHIRP shift, CTCSS and tone vocabulary from Capabilities

Duplex and Tone now ask caps WHICH VALUE means up-shift or encode-only,
via ShiftDirection and the Encodes/Decodes pair, instead of naming the
FT-710's literals. Tones are checked against caps.CTCSSTones rather than
the global standard chart.

chirpModeMap stays: CHIRP-to-display-name is CHIRP-dialect knowledge. Its
RESULT is membership-checked against caps.Modes, so a radio lacking the
mapped mode blocks the row instead of being written a mode it has no
equivalent for.

Every missing capability produces a blocking LossEntry — refuse, never
corrupt."
```

---

### Task 4: The `ImportCHIRP` doc contract, and `doc.go` (m40d)

Documentation only — no behaviour change. Kept separate so the wording is reviewed on its own rather than buried under Task 3's logic.

**Files:**
- Modify: `core/csvio/chirp.go:503-507` (the SYNTACTIC paragraph), `:229-236` (`chirpTagByteOK`), `:20-32` (Action constants), `:79`, `:377`, `:447`, `:461`
- Modify: `core/csvio/doc.go:17`, `:28-35`

- [ ] **Step 1: Rewrite `ImportCHIRP`'s SYNTACTIC paragraph (`chirp.go:503-507`)**

```go
// Unlike Import, this consults caps — but only for VOCABULARY AND SHAPE:
// the memory bank's slot space, the tag length, the shift and CTCSS state
// vocabularies, and the mode and CTCSS-tone tables. It still does NOT run
// codeplug.Validate, and a channel with no Blocking entries against it is
// still not thereby guaranteed valid for the radio (its frequency band,
// its per-field write support, its radio identity). Validate remains the
// semantic gate a caller must run before a send.
```

- [ ] **Step 2: Neutralise the remaining FT-710 mentions in `chirp.go` comments**

- `:22-23` `ActionDropped` — "the FT-710 has no field to hold it at all" → "the radio has no field to hold it at all"
- `:26` `ActionApproximated` — "to fit the FT-710's constraints" → "to fit the radio's constraints"
- `:79` `chirpExtraColumns` — "no FT-710 field for" → "no radio field for"
- `:229-233` `chirpTagByteOK` — "a legal FT-710 tag byte" → "a legal tag byte for this radio family". **Keep the function as-is and keep its existing explanation that it restates codeplug's unexported `validTagByte`.** Add: "printable ASCII excluding ';' is a family-wide CAT fact — ';' is the protocol terminator, not a per-model choice — so this needs no capability."
- `:377` the Offset entry Detail — `fmt.Sprintf("%s stores no per-channel repeater offset; shift magnitude is a global menu setting", caps.Model)`
- `:447` the Skip entry Detail — `fmt.Sprintf("CHIRP Skip value %q has no %s equivalent; scan-skip left unresolved", skipRaw, caps.Model)`
- `:461` the extra-column Detail — `fmt.Sprintf("%s has no equivalent field for CHIRP %s; value discarded", caps.Model, col)`

Each renders byte-identically for the FT-710.

- [ ] **Step 3: Rewrite `core/csvio/doc.go`**

At `:17`, "fields the FT-710 has no equivalent for are dropped" → "fields the target radio has no equivalent for are dropped".

Replace the `:28-35` paragraph:

```go
// Import is SYNTACTIC ONLY: it turns well-formed CSV cells into
// codeplug.Channel/ChannelData values without judging whether those values
// make sense for any particular radio. ImportCHIRP additionally consults a
// spec.Capabilities, but only for VOCABULARY AND SHAPE — slot space, tag
// length, shift/CTCSS vocabulary, mode and tone tables — never for
// semantic validity. Neither import runs codeplug.Validate, and
// successfully imported data is not thereby guaranteed valid:
// codeplug.Validate remains the one semantic gate a caller must run before
// treating any codeplug (regardless of its origin) as ready to send. This
// package does not duplicate any of Validate's rules.
```

Keep the dependency-rule paragraph at `:37-39` unchanged — it is still true.

- [ ] **Step 4: Verify no FT-710 literal survives in `core/csvio` non-test code**

Run: `grep -rn 'FT-710\|FT710' core/csvio/*.go | grep -v '_test.go'`
Expected: **no output.**

- [ ] **Step 5: Run the suite and confirm nothing moved**

Run: `go build ./... && go test ./core/csvio/ -count=1`
Expected: PASS with no test expectation edited in this task.

- [ ] **Step 6: Commit**

```bash
git add core/csvio
git commit -m "M9c-1 task 4: csvio doc contract and prose go neutral (m40d)

ImportCHIRP does now consult Capabilities, so the paragraph claiming it
consults none is rewritten rather than deleted: the distinction it
protects is still real and still load-bearing — vocabulary and shape,
never semantic validity, and Validate remains the gate before a send.

Closes ledger minor m40d. grep for FT-710 in core/csvio non-test code is
now clean."
```

---

### Task 5: `radiotext` gains the last hardcoded string (m42a)

**Files:**
- Modify: `internal/radiotext/radiotext.go` (add `ToneScanSkipVerification` to `Text` and `ft710Text`)
- Modify: `app/types.go` (add the field to `UISpecView`), `app/uispec.go:254` area
- Modify: `app/frontend/src/lib/ChannelGrid.svelte:1020-1024`
- Test: `internal/radiotext/radiotext_test.go`, `app/uispec_test.go`

**Interfaces:**
- Consumes: nothing from Tasks 1-4.
- Produces: `radiotext.Text.ToneScanSkipVerification string`; `app.UISpecView.ToneScanSkipVerification string`. Task 6's coverage test does not depend on this field specifically.

- [ ] **Step 1: Write the failing tests**

In `internal/radiotext/radiotext_test.go`, extend the existing verbatim pin (`TestRadiotext_FT710Verbatim`) with the new field, asserting the exact sentence:

```go
	if got, want := ft710Text.ToneScanSkipVerification, "Preservation across a rewrite is hardware-verified for Tone; Scan Skip preservation is not yet verified (see each cell's tooltip)."; got != want {
		t.Errorf("ToneScanSkipVerification = %q, want %q", got, want)
	}
```

In `app/uispec_test.go`, extend the existing `ServesProse` test (the one sourcing want-values from `radiotext`) with the same field, following that test's established pattern exactly.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/radiotext/ ./app/ -run 'Verbatim|ServesProse' -v`
Expected: FAIL — `ToneScanSkipVerification` undefined.

- [ ] **Step 3: Add the field to `radiotext.Text` and `ft710Text`**

```go
	// ToneScanSkipVerification states what is and is not hardware-verified
	// about Tone/Scan Skip preservation across a rewrite for this radio.
	// Verbatim: the SECOND sentence of app/frontend/src/lib/
	// ChannelGrid.svelte's grid-legend paragraph, which task 41
	// deliberately left behind when it captured the first (ledger minor
	// m42a). It cannot stay in the frontend: it is a claim about THIS
	// radio's write trials, and for a model pinned at
	// writeTrialsComplete=false it would be an outright false statement
	// about hardware.
	ToneScanSkipVerification string
```

```go
	ToneScanSkipVerification: "Preservation across a rewrite is hardware-verified for Tone; Scan Skip preservation is not yet verified (see each cell's tooltip).",
```

- [ ] **Step 4: Serve it through `UISpecView`**

Add `ToneScanSkipVerification string` to `UISpecView` in `app/types.go`, beside `ToneScanSkipNote` (~`:360`), with a doc comment matching that field's style and naming `internal/radiotext.Text.ToneScanSkipVerification` as its source. Add `ToneScanSkipVerification: text.ToneScanSkipVerification,` to the returned struct in `app/uispec.go`.

- [ ] **Step 5: Consume it in the frontend**

`app/frontend/src/lib/ChannelGrid.svelte:1020-1024`:

```svelte
		<p class="grid-legend" title="See docs/hardware-notes.md § M5b write trials">
			{appState.uiSpec.ToneScanSkipNote}
			{appState.uiSpec.ToneScanSkipVerification}
		</p>
```

- [ ] **Step 6: Regenerate wailsjs and confirm the only diff is the new field**

Run **from the repo root**: the project's standard wailsjs regeneration, then `git diff --stat app/frontend/wailsjs`
Expected: the generated model gains `ToneScanSkipVerification` and nothing else moves.

- [ ] **Step 7: Run the Go and frontend suites**

Run: `go test ./internal/radiotext/ ./app/ -count=1`
Then, in `app/frontend`: `npm run check && npx vitest run && npm run build`
Expected: PASS; svelte-check 0 errors 0 warnings.

- [ ] **Step 8: Commit**

```bash
git add internal/radiotext app/types.go app/uispec.go app/frontend
git commit -m "M9c-1 task 5: the grid legend's verification sentence moves to radiotext (m42a)

Task 41 captured the legend's first sentence into radiotext and left the
second in Svelte. That sentence claims Tone preservation is
hardware-verified — true of Stuart's FT-710 (13/07/2026), and an outright
false statement for any model pinned at writeTrialsComplete=false.

Closes ledger minor m42a. This is the only frontend touch in M9c-1."
```

---

### Task 6: `radiotext` coverage, and the driver-table invariants (m39a, m39b)

**Files:**
- Modify: `internal/wiring/wiring.go:207-218` (`StaticCapabilities`)
- Test: `internal/wiring/wiring_test.go` (or the file where `TestStaticCapabilities_FT710EqualsDriver` lives)

**Interfaces:**
- Consumes: `radiotext.For` (Task 5 changed its `Text`, not its signature).
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Write the three failing tests**

```go
// Every model this package can open a real session against MUST have
// user-facing prose, or its CLI and GUI silently serve blank advisories
// (cmd/rigprog/write.go's erase procedure, probe.go's firmware note,
// app/uispec.go's grid legend all degrade to "" rather than failing).
// This is the M9c registration precondition: adding a driver without
// prose fails here rather than shipping.
func TestEverySupportedModelHasRadiotext(t *testing.T) {
	for _, model := range SupportedModels() {
		if _, ok := radiotext.For(model); !ok {
			t.Errorf("radiotext.For(%q) = _, false; every model in SupportedModels() must have prose", model)
		}
	}
}

// realDrivers and fakeDrivers must offer the same models: a model
// openable for real but not simulated (or vice versa) would fail only at
// the moment a user tried it.
func TestRealAndFakeDriverTablesAgree(t *testing.T) {
	for model := range realDrivers {
		if _, ok := fakeDrivers[model]; !ok {
			t.Errorf("model %q is in realDrivers but not fakeDrivers", model)
		}
	}
	for model := range fakeDrivers {
		if _, ok := realDrivers[model]; !ok {
			t.Errorf("model %q is in fakeDrivers but not realDrivers", model)
		}
	}
}

// Each table key must equal the driver's own Model(). StaticCapabilities
// registers a driver and then looks it up BY THE CALLER'S KEY; if the two
// disagreed the lookup would miss and the result would be nil.
func TestDriverTableKeysMatchDriverModel(t *testing.T) {
	for model, ctor := range realDrivers {
		if got := ctor().Model(); got != model {
			t.Errorf("realDrivers[%q] builds a driver whose Model() = %q", model, got)
		}
	}
	for model, entry := range fakeDrivers {
		if got := entry.newDriver().Model(); got != model {
			t.Errorf("fakeDrivers[%q] builds a driver whose Model() = %q", model, got)
		}
	}
}
```

Add `"github.com/gm5dna/open-rig-programmer/internal/radiotext"` to the test file's imports.

- [ ] **Step 2: Run them**

Run: `go test ./internal/wiring/ -run 'TestEverySupportedModelHasRadiotext|TestRealAndFakeDriverTablesAgree|TestDriverTableKeysMatchDriverModel' -v`
Expected: **PASS** — the FT-710 satisfies all three today. That is correct and expected: these are *forcing functions* for the next model, not bug reproductions. Confirm each genuinely bites by temporarily adding a bogus entry to `realDrivers` (e.g. `"NOPE": NewRealDriver`), re-running to see all three fail, then removing it. **Record that red-proof in the commit message.**

- [ ] **Step 3: Fix m39a — stop discarding the lookup result**

`internal/wiring/wiring.go:212-217`:

```go
	reg, err := NewRegistry(d)
	if err != nil {
		return spec.Capabilities{}, err
	}
	drv, ok := reg.Get(model)
	if !ok {
		// Unreachable while TestDriverTableKeysMatchDriverModel holds: d
		// was just registered under its own Model(), which that test pins
		// equal to this table key. Returned rather than ignored so a
		// future table whose key drifted from its driver's Model() fails
		// with this package's own typed error instead of a nil-pointer
		// panic inside Capabilities().
		return spec.Capabilities{}, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}
	return drv.Capabilities(), nil
```

- [ ] **Step 4: Run the wiring suite**

Run: `go test ./internal/wiring/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/wiring
git commit -m "M9c-1 task 6: radiotext coverage and driver-table invariants (m39a, m39b)

Three forcing functions for the second model: every SupportedModels()
entry must have radiotext prose; realDrivers and fakeDrivers must offer
the same models; each table key must equal its driver's Model().

StaticCapabilities no longer discards reg.Get's ok. The third test makes
that branch unreachable, so it returns the package's typed error rather
than nil-panicking (m39a).

Red-proof: adding a bogus \"NOPE\" entry to realDrivers fails all three;
removed before commit.

Closes ledger minors m39a and m39b."
```

---

### Task 7: Per-model snapshot directory (D9)

**Files:**
- Modify: `cmd/rigprog/fileio.go:159-176`, `cmd/rigprog/read.go:168`, `cmd/rigprog/write.go:702`
- Modify: `internal/wiring/wiring.go:289-306`, `app/connection.go:71`
- Test: `cmd/rigprog/fileio_test.go:124-150`, `internal/wiring/wiring_test.go`

**Interfaces:**
- Consumes: `wiring.DefaultModel`.
- Produces: `resolveSnapshotDir(override, model string) (string, error)` in `cmd/rigprog`; `wiring.ResolveSnapshotDir(override, model string) (string, error)`; `wiring.ModelSlug(model string) string`.

- [ ] **Step 1: Write the failing tests**

In `internal/wiring/wiring_test.go`:

```go
func TestModelSlug(t *testing.T) {
	for _, tc := range []struct{ model, want string }{
		{"FT-710", "ft-710"},
		{"FTdx10", "ftdx10"},
		{"FTDX101D/MP", "ftdx101d-mp"},
		{"FTX-1", "ftx-1"},
	} {
		if got := ModelSlug(tc.model); got != tc.want {
			t.Errorf("ModelSlug(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

func TestResolveSnapshotDir_DefaultModelStaysAtRoot(t *testing.T) {
	got, err := ResolveSnapshotDir("/tmp/snaps", DefaultModel)
	if err != nil {
		t.Fatalf("ResolveSnapshotDir: %v", err)
	}
	if got != "/tmp/snaps" {
		t.Errorf("ResolveSnapshotDir(override, DefaultModel) = %q, want %q unchanged — existing FT-710 snapshots must keep working", got, "/tmp/snaps")
	}
}

func TestResolveSnapshotDir_OtherModelGetsSubdir(t *testing.T) {
	got, err := ResolveSnapshotDir("/tmp/snaps", "FTdx10")
	if err != nil {
		t.Fatalf("ResolveSnapshotDir: %v", err)
	}
	if want := "/tmp/snaps/ftdx10"; got != want {
		t.Errorf("ResolveSnapshotDir(override, %q) = %q, want %q", "FTdx10", got, want)
	}
}
```

Mirror the last two in `cmd/rigprog/fileio_test.go` against `resolveSnapshotDir`.

- [ ] **Step 2: Run them**

Run: `go test ./internal/wiring/ ./cmd/rigprog/ -run 'SnapshotDir|ModelSlug' -v`
Expected: FAIL — `ModelSlug` undefined; `ResolveSnapshotDir` takes 1 argument.

- [ ] **Step 3: Add `ModelSlug` to `internal/wiring/wiring.go`**

```go
// ModelSlug turns a model name into a filesystem-safe directory
// component: lowercased, with each run of non-alphanumeric characters
// collapsed to a single "-" (e.g. "FTDX101D/MP" -> "ftdx101d-mp"). Used
// to give each model its own snapshot/journal directory.
func ModelSlug(model string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(model) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
```

- [ ] **Step 4: Add the model parameter to both resolvers**

`internal/wiring/wiring.go`:

```go
func ResolveSnapshotDir(override, model string) (string, error) {
	base := override
	if base == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("determining default snapshot directory: %w", err)
		}
		base = filepath.Join(cfgDir, "rigprog", "snapshots")
	}
	// DefaultModel stays at the base directory so every snapshot written
	// before this rule existed is still found. Any other model gets its
	// own subdirectory — applied to an explicit override too, since two
	// models sharing one named directory is exactly the collision this
	// rule exists to prevent.
	if model == DefaultModel {
		return base, nil
	}
	return filepath.Join(base, ModelSlug(model)), nil
}
```

Extend its doc comment with the two-sentence rule. Apply the identical change to `cmd/rigprog/fileio.go`'s `resolveSnapshotDir` (the deliberate duplication documented at `wiring.go:298-301` stays deliberate).

- [ ] **Step 5: Update the three call sites**

- `cmd/rigprog/read.go:168` → `resolveSnapshotDir(*snapshotDirFlag, *model)`
- `cmd/rigprog/write.go:702` → `resolveSnapshotDir(*snapshotDirFlag, *model)` (confirm the model flag variable's name in that function first)
- `app/connection.go:71` → `wiring.ResolveSnapshotDir("", wiring.DefaultModel)`

The `--snapshot-dir` help strings (`read.go:119`, `write.go:666`, `usage.go:101`, `usage.go:168`) are **unchanged**: they document the default for the default model, which is still correct.

- [ ] **Step 6: Run the suites**

Run: `go build ./... && go test ./internal/wiring/ ./cmd/rigprog/ ./app/ -count=1`
Expected: PASS, including the pre-existing `resolveSnapshotDir` override test at `fileio_test.go:128` once threaded with `DefaultModel`.

- [ ] **Step 7: Commit**

```bash
git add cmd/rigprog internal/wiring app/connection.go
git commit -m "M9c-1 task 7: per-model snapshot directory (D9)

DefaultModel stays at <UserConfigDir>/rigprog/snapshots byte-identically,
so existing FT-710 snapshots keep working. Any other model gets
<base>/<model-slug>/ — applied to an explicit --snapshot-dir too, since
two models sharing one named directory is precisely the collision the
rule exists to prevent.

Closes ledger item D9."
```

---

### Task 8: Baselines and the full local gate

CI is billing-dead; the local gate is the verification. **No code changes in this task** — if the gate finds a defect, fix it in a task-scoped commit and re-run.

- [ ] **Step 1: Mint the M9c byte-identity baselines**

Following `docs/superpowers/m9c0-baseline-manifest.md`'s format, capture from a **built binary** (not `go run`, which collapses exit codes — the known artefact recorded in the M9a gate notes): `probe --fake`, `read --fake`, and a CHIRP import over the existing test fixture. Record stdout, stderr and exit code for each. Write `docs/superpowers/m9c1-baseline-manifest.md`.

- [ ] **Step 2: Compare against the pre-M9c-1 tree**

`git stash` or a worktree at `ce7fcee`, capture the same three, and diff.
Expected: **byte-identical**, including stderr. Any difference that is not enumerated in Task 4's wording notes is a defect.

- [ ] **Step 3: Run the full gate**

```bash
gofmt -l . && go vet ./... && go build ./...
go test ./... -count=1
go test ./internal/guards/ -v -count=1
```
Then in the background: `go test -race ./core/... -count=1`
And: `go test -race ./app/ -count=1`
Expected: all PASS; all 5 guards PASS verbose.

- [ ] **Step 4: Confirm the pins and the wailsjs drift**

Verify the importgraph and driver-seam pins still hold, and from the **repo root** re-run the wailsjs regeneration and confirm zero diff beyond Task 5's field.

- [ ] **Step 5: Commit the baseline manifest**

```bash
git add docs/superpowers/m9c1-baseline-manifest.md
git commit -m "M9c-1 task 8: byte-identity baselines and full local gate

FT-710 probe/read/CHIRP-import output verified byte-identical to ce7fcee
across stdout, stderr and exit code."
```

- [ ] **Step 6: Codex adversarial milestone review**

Dispatch per standing convention: **repo-relative paths only** (an absolute path outside the workspace hangs the job silently), and dispatch standalone with `< /dev/null`. Adjudicate findings, fix in a wave, re-review.

- [ ] **Step 7: Merge**

```bash
git checkout main
git merge --no-ff m9c1-registration-gate
```

Then update `.superpowers/sdd/progress.md` and `.superpowers/sdd/HANDOFF-m9c.md`: all seven registration preconditions are closed, and the next item is `internal/extable` generator parameterisation.

---

## Self-Review

**Spec coverage:** all seven items mapped — §1→Task 1, §2→Tasks 2/3, §2 doc contract + m40d→Task 4, §3→Tasks 5/6, §4 m39a/m39b→Task 6, §4 D9→Task 7, §5 (`currentCaps` untouched) honoured in Task 2 Step 5, Verification→Task 8.

**Known follow-ups deliberately left out of scope:** the FTdx10's own `radiotext` prose (Task 6's coverage test forces it when that model registers) and `internal/extable` generator parameterisation (the handoff's next item, not a registration precondition).

**Type consistency:** `ShiftOption`/`ShiftDirection`/`ShiftNone`/`ShiftUp`/`ShiftDown` and `ToneState.Encodes`/`.Decodes` are defined in Task 1 and used with those exact names in Tasks 2, 3 and 7's fixtures. `ImportCHIRP(rd, caps)` is established in Task 2 and used unchanged in Tasks 3 and 4. `ft710LikeCapabilities()`/`deviantCapabilities()` are defined in Task 2 Step 1 and reused in Task 3. `resolveSnapshotDir`/`ResolveSnapshotDir` gain the same `(override, model)` shape in Task 7.

**One deliberate asymmetry:** `shiftOptionValues` exists twice — unexported in `core/spec` (Task 1 Step 5) and again in `core/codeplug` (Task 1 Step 6). That mirrors `toneStateValues`, which the codebase already duplicates the same way rather than exporting a helper from `core/spec`.

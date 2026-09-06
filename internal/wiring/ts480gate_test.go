// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	ts480drv "github.com/gm5dna/open-rig-programmer/core/driver/ts480"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts480 "github.com/gm5dna/open-rig-programmer/core/kw/ts480"
)

// THE TS-480 REGISTRATION GATE.
//
// The TS-480 row is BUILT and NOT REGISTERED: core/kw/ts480, core/driver/ts480
// and internal/fakets480 are complete and tested, and this package carries no
// row for the model in realDrivers, in fakeDrivers or in SupportedModels().
// The reason is A4 — whether an MR of an unwritten channel ANSWERS (with
// P4-P15 zero) rather than rejecting is documented for the TS-590SG
// (590:1492-1493) and entirely unprinted for the 2003 TS-480 book — and A4's
// lift is L-HW-3, hardware confirmation item 3, the release gate for this row.
// Spec decision 10 removed the release-time override that used to sit beside
// it, so the row's registration is gated on a committed evidence artefact and
// on nothing else.
//
// THE BAR IN SPEC HARDWARE ITEM 3 AND THE ASSERTIONS BELOW ARE THE SAME
// SENTENCE IN TWO FORMS, AND IF THEY EVER DIVERGE THE GUARD IS WHAT SHIPS.
// That sentence is: a valid zero record on at least three separate unwritten
// channels, in at least two sessions, with no silence and no "?;" among them.
// A document may be edited by anybody at any time and an edit to it changes
// nothing about what this repository will register; checkTS480ObservationBar
// is what a `go test` run consults. A future author who finds the two
// disagreeing should fix the document to match this file, or change this file
// deliberately and say so in the commit — not assume the prose is authority.
//
// WHY THE ARTEFACT IS TRACKED IN GIT and not under docs/superpowers/ or
// docs/fixtures-private/: both are gitignored, so an artefact there would be
// absent in a fresh clone and in CI, and this guard would read "no evidence"
// on a machine that had simply not been handed the file. That is the failure
// internal/guards' fresh-clone guard exists to catch, one level up. This
// artefact must be present or absent for everybody at once.
//
// THE THREE LEGS.
//
//  1. ABSENT means ABSENT. With the artefact off disk — the normal state of
//     this repository, and the leg that actually runs — the row must be in
//     none of the three registry surfaces. An accidental or premature
//     registration fails here. This is a BRANCH, not a skip: a skip reports
//     nothing and would leave the ordinary state of the tree unasserted.
//  2. PRESENT means PRESENT. With the artefact on disk, it is parsed, held to
//     the bar above, and only then is the row required to BE registered. A
//     hand-waved artefact fails the bar and the row stays out.
//  3. NON-VACUITY. A file that parses but records no trials, or trials whose
//     outcome fields are absent, fails LOUDLY rather than reading as absence.
//     That is icr8600_golden_test.go's lesson in this package already: a guard
//     satisfiable by an empty artefact is satisfiable by WRITING an empty
//     artefact, which is spec decision 10's override wearing a JSON hat.
//
// Legs 2 and 3 are exercised over fixture files in the test's own temp
// directory (TestTS480ObservationBar_OverFixtures), through the same loader
// and the same bar checker leg 2 uses, so the two cannot drift apart while the
// real artefact is absent.
//
// WHAT THIS GUARD DOES NOT DO, AND WHERE THAT WORK ACTUALLY LIVES.
// Registering this row later is TEN edits and not one (plan P3). An earlier
// revision of the plan gave this file a leg that would "derive" that list from
// here by inspecting internal/radiotext's ownParticulars, app/'s UI membership
// map, internal/guards' simulatedProfiles and core/csvio's CHIRP expectations.
// THAT LEG CANNOT BE WRITTEN: all four are test-only symbols in four separate
// Go test binaries, and three of them are function-local (simulatedProfiles
// inside TestSimulatedProfileTokensConfinement, the UI map inside
// TestBankCoreFields_EveryRegisteredModel_Membership, the CHIRP expectations
// in package csvio). No import from internal/wiring reaches any of them, and
// no test discovers a surface added later by reflecting over binaries it does
// not link. The leg is deleted, and P3's ten-row table is a REVIEWED
// CHECKLIST: no test derives it.
//
// What holds the checklist honest instead is four per-package completeness
// checks, each keyed off wiring.SupportedModels() and each firing in the
// package a future contributor is actually editing:
//
//   - internal/radiotext/radiotext_test.go — ownParticulars' completeness
//     PANIC ("%q is registered but ownParticulars carries no entry for it").
//   - app/uispec_test.go — TestBankCoreFields_EveryRegisteredModel_Membership's
//     walk ("model %q is registered but has no expected core set here").
//   - internal/guards/simulated_tokens_test.go —
//     assertEveryRegisteredModelIsConfined, plus
//     assertNoTS480RowUntilTheRowRegisters, which names THIS row's absence.
//   - core/csvio/chirp_test.go — TestChirpFixtures_CoverEveryRegisteredModel.
//
// The first two already existed; the last two landed at this milestone. So
// NINE of the ten edits now fail loudly the moment the row registers without
// them, and the tenth — the model-list baseline re-capture — only at the
// byte-identity gate. That is a stronger practical guarantee than the deleted
// leg would have given even had it compiled.
//
// THE SCHEMA is the Go types below, and the field names are their json tags.
// internal/wiring/testdata/README.md points a future observer here rather than
// restating them, so there is no second copy to drift.

// ts480RowKey is the registry key this row would take. It is deliberately a
// literal and not a TS480Model constant: introducing that constant is edit 2
// of P3's ten, and it does not exist yet.
const ts480RowKey = "TS-480"

// ts480ObservationFile is the evidence artefact's name, under this package's
// testdata directory. It does not exist, and creating it is edit 1 of the ten.
const ts480ObservationFile = "ts480-a4-observation.json"

// The five outcomes hardware item 3 enumerates, one per wire result an MR of a
// supposedly unwritten TS-480 channel can produce. They are a CLOSED
// vocabulary: a trial carrying anything else — including the empty string an
// absent JSON field decodes to — is a schema failure rather than a sixth
// reading, because the point of the enumeration is that the reader of the
// artefact never has to interpret a free-text note.
//
//   - ts480OutcomeValidZeroRecord: a valid 50-byte record with P4-P15 all
//     zero. A4 lifted — the 590SG's rule holds on the 480. THE ONLY OUTCOME
//     THAT COUNTS TOWARDS THE GATE.
//   - ts480OutcomeRejected: "?;". A4 FALSE, definitively (decision 5): a fresh
//     TS-480 is unreadable.
//   - ts480OutcomeSilence: a read timeout. INCONCLUSIVE, not "absent" —
//     480:136-138 says the "?;" may be suppressed altogether, and decision 5
//     forbids reading silence as absence. mr_answer is null on this outcome
//     and on no other.
//   - ts480OutcomeLinkEvent: "E;" or "O;". INCONCLUSIVE — a link-health event,
//     not an answer about the channel.
//   - ts480OutcomeUnexpectedFrame: a 50-byte record that is not all-zero, or
//     any other frame. A4 false in a new way; the trial must be repeated on a
//     channel confirmed unwritten from the front panel.
const (
	ts480OutcomeValidZeroRecord = "valid_zero_record"
	ts480OutcomeRejected        = "rejected"
	ts480OutcomeSilence         = "silence"
	ts480OutcomeLinkEvent       = "link_event"
	ts480OutcomeUnexpectedFrame = "unexpected_frame"
)

var ts480Outcomes = []string{
	ts480OutcomeValidZeroRecord,
	ts480OutcomeRejected,
	ts480OutcomeSilence,
	ts480OutcomeLinkEvent,
	ts480OutcomeUnexpectedFrame,
}

// ts480Observation is the evidence artefact's whole shape, in the form spec
// decision 10 fixes it: the assumption and lift it claims, the radio it was
// taken from, who took it, and the sessions.
//
// EVERY BYTE FIELD IS THE WIRE, VERBATIM. It is a wire observation and not a
// transcription, and that distinction is the whole point of the file: the bar
// checker re-derives the request this programme would have sent and re-parses
// the answer through this row's own codec, so a plausible-looking frame typed
// from memory fails rather than passing.
type ts480Observation struct {
	Assumption string            `json:"assumption"`
	Lift       string            `json:"lift"`
	Radio      ts480RadioIdent   `json:"radio"`
	Observer   string            `json:"observer"`
	Sessions   []ts480ObsSession `json:"sessions"`
}

// ts480RadioIdent identifies the radio the observation came from, by its own
// answers rather than by a name somebody typed. No observation on one registry
// row lifts an assumption for another, and the ID answer is what says which
// row this file is about.
type ts480RadioIdent struct {
	Model string `json:"model"`
	// IDAnswer is the exact "ID;" answer, e.g. six bytes ending in ';'.
	IDAnswer string `json:"id_answer"`
	// TYAnswer is the exact "TY;" answer — this radio's hardware variant
	// read, which the 590 book does not have at all (decision 4).
	TYAnswer string `json:"ty_answer"`
}

// ts480ObsSession is one sitting at the radio. TWO SESSIONS ARE REQUIRED and
// what makes them two is that they are two entries here, each with trials of
// its own: a guard cannot tell a power cycle from a copy-paste, and the weight
// in the bar is carried by the front-panel confirmations and by the three
// distinct channels rather than by this count alone.
type ts480ObsSession struct {
	Date   string       `json:"date"`
	Port   string       `json:"port"`
	Baud   int          `json:"baud"`
	Trials []ts480Trial `json:"trials"`
}

// ts480Trial is one MR of one channel.
type ts480Trial struct {
	// Channel is the slot identifier this programme publishes for the
	// TS-480 — two digits, "00".."99" (core/driver/ts480's slotID and its
	// one flat MEM bank), NOT core/kw's three-digit Slot.String form.
	Channel string `json:"channel"`
	// FrontPanelConfirmedUnwritten must be true. A4 is about what an
	// UNWRITTEN channel answers; a zero record from a channel nobody checked
	// evidences nothing, and this field is the human half of the trial that
	// no wire capture can supply.
	FrontPanelConfirmedUnwritten bool `json:"front_panel_confirmed_unwritten"`
	// MRRequest is the exact request put on the wire.
	MRRequest string `json:"mr_request"`
	// MRAnswer is the exact answer, or null for silence.
	MRAnswer *string `json:"mr_answer"`
	// Outcome is one of the five above.
	Outcome string `json:"outcome"`
}

// loadTS480Observation reads path, reporting whether the file is there at all.
// A missing file is the ABSENT branch and not an error; anything else is.
//
// UNKNOWN FIELDS ARE REFUSED because this file is hand-written at a bench. A
// misspelt key would otherwise decode to a zero value and read as a considered
// "no" — and the two keys where that matters most,
// front_panel_confirmed_unwritten and outcome, are exactly the two a typo
// would silently turn into "unconfirmed" and "no outcome recorded".
func loadTS480Observation(path string) (ts480Observation, bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ts480Observation{}, false, nil
	}
	if err != nil {
		return ts480Observation{}, false, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	var obs ts480Observation
	if err := dec.Decode(&obs); err != nil {
		return ts480Observation{}, true, fmt.Errorf("decode: %w", err)
	}
	return obs, true, nil
}

// ts480SlotFor resolves a published TS-480 channel identifier to the codec's
// slot, through the row's OWN published inventory and the layout's OWN slot
// space rather than through a range this file would otherwise restate.
func ts480SlotFor(channel string) (kw.Slot, error) {
	published := false
	for _, b := range ts480drv.CapabilitiesUnverified().Banks {
		if slices.Contains(b.Slots, channel) {
			published = true
			break
		}
	}
	if !published {
		return kw.Slot{}, fmt.Errorf("channel %q is not a slot core/driver/ts480 publishes", channel)
	}
	n, err := strconv.Atoi(channel)
	if err != nil {
		return kw.Slot{}, fmt.Errorf("channel %q: %w", channel, err)
	}
	return kwts480.Layout().NewSlot(n, kw.ScanHalfNone)
}

// checkTS480ObservationBar returns every way obs falls short of hardware item
// 3's bar. An empty result means the bar is met and the row may be registered.
//
// IT RETURNS ALL OF THEM RATHER THAN THE FIRST. Somebody reading this output
// is standing at a radio deciding what to observe next, and one complaint at a
// time makes that several `go test` runs.
func checkTS480ObservationBar(obs ts480Observation) []string {
	var problems []string
	bad := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if obs.Assumption != "A4" {
		bad("assumption is %q, want %q — this artefact lifts A4 and nothing else", obs.Assumption, "A4")
	}
	if obs.Lift != "L-HW-3" {
		bad("lift is %q, want %q — hardware confirmation item 3, the release gate for this row", obs.Lift, "L-HW-3")
	}
	if strings.TrimSpace(obs.Observer) == "" {
		bad("observer is empty — an observation nobody is named for is not one this repository will register a radio on")
	}

	layout := kwts480.Layout()
	catID := ts480drv.CapabilitiesUnverified().CATID

	if obs.Radio.Model != ts480RowKey {
		bad("radio.model is %q, want %q", obs.Radio.Model, ts480RowKey)
	}
	if got, err := layout.ParseIDAnswer([]byte(obs.Radio.IDAnswer)); err != nil {
		bad("radio.id_answer %q is not an ID answer this row's codec accepts: %v", obs.Radio.IDAnswer, err)
	} else if got != catID {
		bad("radio.id_answer %q names CAT identity %q, and the TS-480's is %q — an observation of another radio lifts nothing for this row", obs.Radio.IDAnswer, got, catID)
	}
	if _, err := layout.ParseTYAnswer([]byte(obs.Radio.TYAnswer)); err != nil {
		bad("radio.ty_answer %q is not a TY answer this row's codec accepts: %v", obs.Radio.TYAnswer, err)
	}

	if len(obs.Sessions) < 2 {
		bad("the file records %d session(s); the gate needs at least two, because one sitting cannot show that an answer survives a power cycle", len(obs.Sessions))
	}

	channels := map[string]bool{}
	trials := 0
	for si, s := range obs.Sessions {
		where := fmt.Sprintf("sessions[%d]", si)
		if strings.TrimSpace(s.Date) == "" {
			bad("%s: date is empty", where)
		}
		if strings.TrimSpace(s.Port) == "" {
			bad("%s: port is empty", where)
		}
		if s.Baud <= 0 {
			bad("%s: baud is %d", where, s.Baud)
		}
		if len(s.Trials) == 0 {
			bad("%s records no trials — a session with nothing in it pads the session count and evidences nothing", where)
		}
		for ti, tr := range s.Trials {
			trials++
			channels[tr.Channel] = true
			problems = append(problems, checkTS480Trial(fmt.Sprintf("%s.trials[%d]", where, ti), tr)...)
		}
	}

	if trials == 0 {
		bad("the file records no trials at all — a file that parses is not an observation")
	}
	if len(channels) < 3 {
		bad("the trials cover %d distinct channel(s); the gate needs at least three separate unwritten channels", len(channels))
	}
	return problems
}

// checkTS480Trial holds one trial to the bar: the outcome vocabulary, the
// front-panel confirmation, and both frames re-derived through this row's own
// codec.
func checkTS480Trial(where string, tr ts480Trial) []string {
	var problems []string
	bad := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(where+": "+format, args...))
	}

	if !tr.FrontPanelConfirmedUnwritten {
		bad("front_panel_confirmed_unwritten is not true — A4 is about what an UNWRITTEN channel answers, so a trial on a channel nobody checked at the panel proves nothing")
	}
	switch {
	case !slices.Contains(ts480Outcomes, tr.Outcome):
		bad("outcome is %q, which is not one of the five item 3 enumerates %v (an absent field decodes to the empty string, and reads here as a failure rather than as absence)", tr.Outcome, ts480Outcomes)
	case tr.Outcome != ts480OutcomeValidZeroRecord:
		bad("outcome is %q; the gate is met only by %q on every trial, with no silence and no %q anywhere in the file — one inconclusive trial proves nothing and must not be recorded as a pass", tr.Outcome, ts480OutcomeValidZeroRecord, "?;")
	}

	slot, err := ts480SlotFor(tr.Channel)
	if err != nil {
		bad("channel: %v", err)
		return problems
	}
	cmd, err := kwts480.Layout().BuildMRRead(slot)
	if err != nil {
		bad("BuildMRRead(%v): %v", slot, err)
		return problems
	}
	if want := string(cmd.Bytes()); tr.MRRequest != want {
		bad("mr_request is %q, and the read this programme sends for channel %q is %q", tr.MRRequest, tr.Channel, want)
	}

	if tr.MRAnswer == nil {
		if tr.Outcome != ts480OutcomeSilence {
			bad("mr_answer is null, which the schema admits only on outcome %q", ts480OutcomeSilence)
		}
		return problems
	}
	rec, err := kwts480.Layout().ParseMRAnswer([]byte(*tr.MRAnswer))
	if err != nil {
		bad("mr_answer %q is not a record this row's codec accepts: %v", *tr.MRAnswer, err)
		return problems
	}
	if !rec.Empty {
		bad("mr_answer decodes as a POPULATED channel, not the P4-P15 zero record A4 is about — the channel was not unwritten, and the trial must be repeated on one confirmed unwritten from the front panel")
	}
	if rec.Slot != slot {
		bad("mr_answer names channel %v, not %v — an answer correlated to another channel evidences nothing about this one", rec.Slot, slot)
	}
	return problems
}

// TestTS480RegistrationGate is legs 1 and 2: the correspondence between the
// evidence artefact's presence and this row's registration.
//
// It is ONE test with a branch rather than two tests with a skip, because
// which branch runs is itself the fact being asserted.
func TestTS480RegistrationGate(t *testing.T) {
	path := filepath.Join("testdata", ts480ObservationFile)

	obs, present, err := loadTS480Observation(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}

	if !present {
		t.Logf("%s is absent, which is the normal state of this repository: the TS-480 row is BUILT and NOT REGISTERED (A4 unlifted, spec decision 10), and the leg below is the one that runs", path)
		assertTS480RowIsAbsent(t)
		return
	}

	if problems := checkTS480ObservationBar(obs); len(problems) > 0 {
		t.Fatalf("%s does not meet hardware item 3's bar, so the TS-480 row may NOT be registered:\n  %s", path, strings.Join(problems, "\n  "))
	}
	assertTS480RowIsRegistered(t)
}

// assertTS480RowIsAbsent is leg 1's assertion: all three registry surfaces
// carry no TS-480.
func assertTS480RowIsAbsent(t *testing.T) {
	t.Helper()
	models := SupportedModels()
	if len(models) == 0 {
		t.Fatal("SupportedModels() is empty — every assertion here would hold vacuously")
	}
	if _, ok := realDrivers[ts480RowKey]; ok {
		t.Errorf("realDrivers carries %q while %s is absent: A4 is unlifted, and registering this row without the observation is what spec decision 10 removed the override to prevent", ts480RowKey, ts480ObservationFile)
	}
	if _, ok := fakeDrivers[ts480RowKey]; ok {
		t.Errorf("fakeDrivers carries %q while %s is absent — the simulated path is registration too: it puts the row in front of a user", ts480RowKey, ts480ObservationFile)
	}
	if slices.Contains(models, ts480RowKey) {
		t.Errorf("SupportedModels() contains %q while %s is absent", ts480RowKey, ts480ObservationFile)
	}
}

// assertTS480RowIsRegistered is leg 2's assertion, reached only after the bar
// is met: an artefact that satisfies the gate and a row still missing means
// the other nine of P3's ten edits were never made.
func assertTS480RowIsRegistered(t *testing.T) {
	t.Helper()
	if _, ok := realDrivers[ts480RowKey]; !ok {
		t.Errorf("%s meets the bar but realDrivers carries no %q row — registering it is edit 2 of the ten P3 lists", ts480ObservationFile, ts480RowKey)
	}
	if _, ok := fakeDrivers[ts480RowKey]; !ok {
		t.Errorf("%s meets the bar but fakeDrivers carries no %q row — edit 3 of the ten", ts480ObservationFile, ts480RowKey)
	}
	if !slices.Contains(SupportedModels(), ts480RowKey) {
		t.Errorf("%s meets the bar but SupportedModels() does not contain %q", ts480ObservationFile, ts480RowKey)
	}
}

// TestTS480ObservationBar_OverFixtures is leg 3, and the half of leg 2 that
// can run while the real artefact is absent.
//
// EVERY FIXTURE GOES THROUGH THE SAME LOADER AND THE SAME BAR CHECKER the
// registration gate uses, from a file in the test's own temp directory. A
// second, test-only parser would be one edit from admitting what the gate
// refuses. Nothing here is committed and nothing here is evidence: the
// conforming document names no observer, no date and no port that could be
// mistaken for one.
func TestTS480ObservationBar_OverFixtures(t *testing.T) {
	for _, tc := range []struct {
		name string
		// mutate edits the conforming document; nil leaves it conforming.
		mutate func(t *testing.T, doc map[string]any)
		// wantLoadErr, when set, is a substring of the error the loader
		// must return. wantProblem, when set, is a substring of the bar's
		// complaint. Both empty means the bar must be met.
		wantLoadErr string
		wantProblem string
	}{
		{
			name: "the conforming document meets the bar",
		},
		{
			name:        "an unknown field is refused rather than read as a considered no",
			mutate:      func(_ *testing.T, doc map[string]any) { doc["front_panel_confirmed"] = true },
			wantLoadErr: "unknown field",
		},
		{
			name:        "no sessions at all",
			mutate:      func(_ *testing.T, doc map[string]any) { doc["sessions"] = []any{} },
			wantProblem: "at least two",
		},
		{
			name: "one session only",
			mutate: func(t *testing.T, doc map[string]any) {
				doc["sessions"] = ts480Sessions(t, doc)[:1]
			},
			wantProblem: "at least two",
		},
		{
			name: "a session that records no trials",
			mutate: func(t *testing.T, doc map[string]any) {
				ts480SessionAt(t, doc, 1)["trials"] = []any{}
			},
			wantProblem: "records no trials",
		},
		{
			name: "every trial removed",
			mutate: func(t *testing.T, doc map[string]any) {
				for i := range ts480Sessions(t, doc) {
					ts480SessionAt(t, doc, i)["trials"] = []any{}
				}
			},
			wantProblem: "no trials at all",
		},
		{
			name: "the same channel re-observed instead of three distinct ones",
			mutate: func(t *testing.T, doc map[string]any) {
				first := ts480Sessions(t, doc)[0].(map[string]any)["trials"].([]any)[0]
				ts480SessionAt(t, doc, 1)["trials"] = []any{first}
			},
			wantProblem: "at least three separate unwritten channels",
		},
		{
			name: "an outcome field that is simply absent",
			mutate: func(t *testing.T, doc map[string]any) {
				delete(ts480TrialAt(t, doc, 0, 0), "outcome")
			},
			wantProblem: "not one of the five",
		},
		{
			name: "a silent trial recorded as if it counted",
			mutate: func(t *testing.T, doc map[string]any) {
				tr := ts480TrialAt(t, doc, 0, 0)
				tr["outcome"] = ts480OutcomeSilence
				tr["mr_answer"] = nil
			},
			wantProblem: "the gate is met only by",
		},
		{
			name: "a \"?;\" among the trials",
			mutate: func(t *testing.T, doc map[string]any) {
				ts480TrialAt(t, doc, 0, 0)["outcome"] = ts480OutcomeRejected
			},
			wantProblem: "the gate is met only by",
		},
		{
			name: "a null answer on an outcome that is not silence",
			mutate: func(t *testing.T, doc map[string]any) {
				ts480TrialAt(t, doc, 0, 0)["mr_answer"] = nil
			},
			wantProblem: "admits only on outcome",
		},
		{
			name: "the front-panel confirmation not made",
			mutate: func(t *testing.T, doc map[string]any) {
				ts480TrialAt(t, doc, 0, 0)["front_panel_confirmed_unwritten"] = false
			},
			wantProblem: "front_panel_confirmed_unwritten is not true",
		},
		{
			name: "a request naming a channel the trial does not",
			mutate: func(t *testing.T, doc map[string]any) {
				ts480TrialAt(t, doc, 0, 0)["mr_request"] = "MR0099;"
			},
			wantProblem: "mr_request is",
		},
		{
			name: "a channel outside the row's published inventory",
			mutate: func(t *testing.T, doc map[string]any) {
				ts480TrialAt(t, doc, 0, 0)["channel"] = "100"
			},
			wantProblem: "not a slot core/driver/ts480 publishes",
		},
		{
			name: "an answer that is not a record this codec accepts",
			mutate: func(t *testing.T, doc map[string]any) {
				ts480TrialAt(t, doc, 0, 0)["mr_answer"] = "MR000;"
			},
			wantProblem: "is not a record this row's codec accepts",
		},
		{
			name: "an answer from a channel that was not empty after all",
			mutate: func(t *testing.T, doc map[string]any) {
				tr := ts480TrialAt(t, doc, 0, 0)
				tr["mr_answer"] = ts480PopulatedAnswer(t, tr["mr_answer"].(string))
			},
			wantProblem: "decodes as a POPULATED channel",
		},
		{
			name: "an answer correlated to another channel",
			mutate: func(t *testing.T, doc map[string]any) {
				other := ts480TrialAt(t, doc, 0, 1)["mr_answer"].(string)
				ts480TrialAt(t, doc, 0, 0)["mr_answer"] = other
			},
			wantProblem: "names channel",
		},
		{
			name:        "the wrong assumption",
			mutate:      func(_ *testing.T, doc map[string]any) { doc["assumption"] = "A3" },
			wantProblem: "assumption is",
		},
		{
			name:        "the wrong lift",
			mutate:      func(_ *testing.T, doc map[string]any) { doc["lift"] = "L-HW-2b" },
			wantProblem: "lift is",
		},
		{
			name:        "nobody named as the observer",
			mutate:      func(_ *testing.T, doc map[string]any) { doc["observer"] = "  " },
			wantProblem: "observer is empty",
		},
		{
			name: "another radio's ID answer",
			mutate: func(t *testing.T, doc map[string]any) {
				// "021" is the TS-590S (590:1114). No observation on one
				// registry row lifts an assumption for another.
				ts480Radio(t, doc)["id_answer"] = "ID021;"
			},
			wantProblem: "lifts nothing for this row",
		},
		{
			name: "a TY answer the decision-4 grammar refuses",
			mutate: func(t *testing.T, doc map[string]any) {
				// P2 prints exactly four variants, '0'..'3' (480:1626-1629).
				ts480Radio(t, doc)["ty_answer"] = "TY009;"
			},
			wantProblem: "is not a TY answer",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := ts480ConformingDocument(t)
			if tc.mutate != nil {
				tc.mutate(t, doc)
			}

			path := filepath.Join(t.TempDir(), ts480ObservationFile)
			body, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				t.Fatalf("marshal fixture: %v", err)
			}
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			obs, present, err := loadTS480Observation(path)
			if !present {
				t.Fatalf("the fixture this test just wrote reads as absent: %s", path)
			}
			if tc.wantLoadErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantLoadErr) {
					t.Fatalf("load error = %v, want one naming %q", err, tc.wantLoadErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("load: %v", err)
			}

			problems := checkTS480ObservationBar(obs)
			joined := strings.Join(problems, "\n  ")
			if tc.wantProblem == "" {
				if len(problems) > 0 {
					t.Fatalf("the conforming document was refused:\n  %s", joined)
				}
				return
			}
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.wantProblem) }) {
				t.Fatalf("no complaint naming %q; the bar reported:\n  %s", tc.wantProblem, joined)
			}
		})
	}
}

// ts480ConformingDocument builds a document that meets the bar, as the generic
// JSON a mutation can reach into.
//
// IT IS BUILT THROUGH THE CODEC, not typed out: the request bytes come from
// BuildMRRead and the answer from the read's own bytes, so a fixture cannot
// pass a check by restating what the check expects. AND IT IS NOT EVIDENCE —
// its observer, date and port say so in words, because the one accident this
// file must not enable is a fixture being copied into testdata/ and read as an
// observation.
func ts480ConformingDocument(t *testing.T) map[string]any {
	t.Helper()

	trial := func(channel string) ts480Trial {
		slot, err := ts480SlotFor(channel)
		if err != nil {
			t.Fatalf("ts480SlotFor(%q): %v", channel, err)
		}
		cmd, err := kwts480.Layout().BuildMRRead(slot)
		if err != nil {
			t.Fatalf("BuildMRRead(%v): %v", slot, err)
		}
		request := string(cmd.Bytes())
		answer := ts480EmptyAnswerFor(t, request)
		return ts480Trial{
			Channel:                      channel,
			FrontPanelConfirmedUnwritten: true,
			MRRequest:                    request,
			MRAnswer:                     &answer,
			Outcome:                      ts480OutcomeValidZeroRecord,
		}
	}

	const notAnObservation = "FIXTURE, NOT AN OBSERVATION"
	obs := ts480Observation{
		Assumption: "A4",
		Lift:       "L-HW-3",
		Radio: ts480RadioIdent{
			Model:    ts480RowKey,
			IDAnswer: "ID" + ts480drv.CapabilitiesUnverified().CATID + ";",
			// P1's two bytes are printed "Reserved" and are opaque;
			// P2 '0' is one of the four printed variants (480:1626-1629).
			TYAnswer: "TY000;",
		},
		Observer: notAnObservation,
		Sessions: []ts480ObsSession{
			{Date: notAnObservation, Port: notAnObservation, Baud: 9600, Trials: []ts480Trial{trial("00"), trial("07")}},
			{Date: notAnObservation, Port: notAnObservation, Baud: 9600, Trials: []ts480Trial{trial("42")}},
		},
	}

	body, err := json.Marshal(obs)
	if err != nil {
		t.Fatalf("marshal conforming document: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("unmarshal conforming document: %v", err)
	}
	return doc
}

// ts480EmptyAnswerFor builds what an unwritten channel WOULD answer if A4
// holds: the read's own six bytes, then P4-P15 zero (positions 7-41), then P16
// blank — eight spaces, which is A3's reading of "P16 will be blank"
// (590:1492-1493) — then the terminator.
//
// NO RADIO HAS SENT THIS FRAME. It is a fixture for the bar checker, and the
// checker's own ParseMRAnswer is what proves the construction right: get the
// widths wrong and every fixture fails at once rather than silently.
func ts480EmptyAnswerFor(t *testing.T, request string) string {
	t.Helper()
	// Positions 7-41 and 42-49 of the 50-byte grid (480:923-943).
	const (
		bodyLen = 35
		nameLen = 8
	)
	answer := request[:kw.MRReadLen-1] + strings.Repeat("0", bodyLen) + strings.Repeat(" ", nameLen) + ";"
	if len(answer) != kw.RecordLen {
		t.Fatalf("built a %d-byte answer, want %d", len(answer), kw.RecordLen)
	}
	rec, err := kwts480.Layout().ParseMRAnswer([]byte(answer))
	if err != nil {
		t.Fatalf("the empty-channel fixture %q is not one this row's codec accepts: %v", answer, err)
	}
	if !rec.Empty {
		t.Fatalf("the empty-channel fixture %q did not decode as empty", answer)
	}
	return answer
}

// ts480PopulatedAnswer turns an empty-channel answer into a valid record for a
// channel that was NOT empty: a frequency in P4 and a mode in P5.
func ts480PopulatedAnswer(t *testing.T, empty string) string {
	t.Helper()
	b := []byte(empty)
	// P4, eleven digits at positions 7-17 (480:957).
	copy(b[6:17], "00014195000")
	// P5 at position 18, the MD nibble (480:959); '1' is LSB.
	b[17] = '1'
	rec, err := kwts480.Layout().ParseMRAnswer(b)
	if err != nil {
		t.Fatalf("the populated-channel fixture %q is not one this row's codec accepts: %v", b, err)
	}
	if rec.Empty {
		t.Fatalf("the populated-channel fixture %q decoded as empty", b)
	}
	return string(b)
}

// The three reach-ins the mutation table uses. They fail the test rather than
// panicking, so a fixture-shape change reports where it broke.

func ts480Radio(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	radio, ok := doc["radio"].(map[string]any)
	if !ok {
		t.Fatalf("the fixture has no radio object: %v", doc["radio"])
	}
	return radio
}

func ts480Sessions(t *testing.T, doc map[string]any) []any {
	t.Helper()
	sessions, ok := doc["sessions"].([]any)
	if !ok {
		t.Fatalf("the fixture has no sessions array: %v", doc["sessions"])
	}
	return sessions
}

func ts480SessionAt(t *testing.T, doc map[string]any, i int) map[string]any {
	t.Helper()
	sessions := ts480Sessions(t, doc)
	if i >= len(sessions) {
		t.Fatalf("the fixture has %d session(s); wanted index %d", len(sessions), i)
	}
	s, ok := sessions[i].(map[string]any)
	if !ok {
		t.Fatalf("sessions[%d] is not an object: %v", i, sessions[i])
	}
	return s
}

func ts480TrialAt(t *testing.T, doc map[string]any, si, ti int) map[string]any {
	t.Helper()
	trials, ok := ts480SessionAt(t, doc, si)["trials"].([]any)
	if !ok {
		t.Fatalf("sessions[%d] has no trials array", si)
	}
	if ti >= len(trials) {
		t.Fatalf("sessions[%d] has %d trial(s); wanted index %d", si, len(trials), ti)
	}
	tr, ok := trials[ti].(map[string]any)
	if !ok {
		t.Fatalf("sessions[%d].trials[%d] is not an object: %v", si, ti, trials[ti])
	}
	return tr
}

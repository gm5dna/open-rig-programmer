// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// These tests drive both models against respondingPort — a scripted,
// frame-parsing CAT peer (respondingport_test.go) — through the driver's
// own real transport.Engine, constructed inside Open. There is no fake
// radio here: internal/fakedx101 does not exist until task 5, and the error
// paths these tests need (a SIBLING's CAT ID, a foreign one, an answer
// naming the wrong slot, an out-of-vocabulary kind byte) are answers a
// self-consistent fake would never give.
//
// EVERY Open in this package costs about 2-3 s of wall clock: the AI0
// init's error window and drain, then ~100 discovery probes at the engine's
// default per-exchange settle. TWO REGISTERED MODELS MEANS TWO MODELS'
// WORTH OF THAT, and it is the accepted, budgeted price of walking the
// whole declared 5xx range for each (doc.go; M9d-2 plan, "Wall-clock is
// budgeted, not optimised"). It is why these tests share sessions where
// they can — NOT something to be optimised away with a settle override.
const testCtxTimeout = 60 * time.Second

// Compile-time proof of the seams this package satisfies. Any signature
// drift fails the BUILD, not merely a test.
//
// DiscoveredBankSynthesizer is the one that matters most here, because its
// ABSENCE fails silently: internal/wiring's synthesis helper asks for the
// interface and returns false when a driver does not satisfy it, and the
// app then renders no discovered banks for an offline FTdx101 codeplug — no
// error anywhere. A compile-time assertion is the only thing that turns a
// method renamed or a receiver changed into a loud failure. (M9c-6's Codex
// review finding 5 is the precedent: the FTdx10 driver carries the same
// four assertions for the same reason.)
var (
	_ driver.Driver                    = (*ftdx101Driver)(nil)
	_ driver.Session                   = (*Session)(nil)
	_ driver.DiscoveredBankSynthesizer = (*ftdx101Driver)(nil)
	_ driver.DiagnosticsReporter       = (*Session)(nil)
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// testIdentity is the caller-side Identity every test session opens with.
var testIdentity = driver.Identity{Port: "scripted-pipe", USBSerial: "SIM0681"}

// testModel is one row of the per-model table nearly every test in this
// package walks: the two radios this ONE driver package registers.
//
// The table exists because "the D and the MP differ in a name and a CAT ID
// and in nothing else" (matrix §4) is a CLAIM this package makes, and a
// suite that exercised only one model would leave the other's registration
// entirely untested while every test passed. catID is written out as a
// literal rather than read from the dialect: it is the manual's own fact
// (layout 1070 and 1072) and a test that derived it from the code under
// test would agree with a wrong value.
type testModel struct {
	name   string
	params modelParams
	newDrv func(Profile, ...Option) driver.Driver
	catID  string
	// sibling is the OTHER model's row index material: its name and CAT ID,
	// which the wrong-sibling refusal must name.
	siblingName  string
	siblingCATID string
}

var testModels = []testModel{
	{
		name: "FTdx101D", params: modelD, newDrv: NewD, catID: "0681",
		siblingName: "FTdx101MP", siblingCATID: "0682",
	},
	{
		name: "FTdx101MP", params: modelMP, newDrv: NewMP, catID: "0682",
		siblingName: "FTdx101D", siblingCATID: "0681",
	},
}

// openSession starts a respondingPort serving img, opens a session over it
// for model m with the given profile, and registers cleanup for both. It
// returns the port (for its transcript) and the concrete *Session.
//
// It fills img.catID from the model being opened when the test has not said
// otherwise — the only place that knows which of the two radios a test
// means — so an ordinary test needs no ceremony and only a wrong-radio test
// says anything about identity. slotImage.catID's doc comment has the rest.
func openSession(t *testing.T, m testModel, profile Profile, img slotImage) (*respondingPort, *Session) {
	t.Helper()
	if img.catID == "" {
		img.catID = m.params.dialect.CATID()
	}
	p := newRespondingPort(t, img)

	sess, err := m.newDrv(profile).Open(testCtx(t), p.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned a %T, want *ftdx101.Session", sess)
	}
	return p, s
}

// TestDriver_ModelAndIdentity: each constructor names its own model, and
// core/driver.Driver's contract that Model() equals Capabilities().Model
// holds on every profile for both.
//
// The cross-check at the end is what a one-model version of this test could
// not do: NewD and NewMP must not return the same model, which is the
// failure a copy-pasted constructor produces and which no per-model
// assertion alone would catch.
func TestDriver_ModelAndIdentity(t *testing.T) {
	for _, m := range testModels {
		for _, profile := range []struct {
			name string
			p    Profile
		}{
			{"RealHardware", RealHardware},
			{"Simulated", Simulated},
			{"unrecognised", Profile(99)},
		} {
			t.Run(m.name+"/"+profile.name, func(t *testing.T) {
				d := m.newDrv(profile.p)
				if d.Model() != m.name {
					t.Errorf("Model() = %q, want %q", d.Model(), m.name)
				}
				if got := d.Capabilities().Model; got != d.Model() {
					t.Errorf("Capabilities().Model = %q, Model() = %q — driver.Driver requires them equal", got, d.Model())
				}
				if got := d.Capabilities().CATID; got != m.catID {
					t.Errorf("Capabilities().CATID = %q, want %q (the manual's ID P1 legend, layout 1070/1072)", got, m.catID)
				}
			})
		}
	}

	if NewD(Simulated).Model() == NewMP(Simulated).Model() {
		t.Error("NewD and NewMP report the SAME model — the two constructors must build two different registrations, which is the whole reason this package parameterises one implementation instead of holding a package-level model")
	}
}

// TestOpen_ProbesIdentityAndPopulatesIt: a successful Open probes ID; and
// puts the ANSWER on the session's Identity, keeping the caller's port and
// USB serial — for each model, against its own CAT ID.
func TestOpen_ProbesIdentityAndPopulatesIt(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p, sess := openSession(t, m, Simulated, slotImage{})

			id := sess.Identity()
			if id.CATID != m.catID {
				t.Errorf("Identity().CATID = %q, want %q (the probe's answer is authoritative)", id.CATID, m.catID)
			}
			if id.Port != testIdentity.Port || id.USBSerial != testIdentity.USBSerial {
				t.Errorf("Identity() = %+v, want the caller's Port/USBSerial preserved (%+v)", id, testIdentity)
			}

			transcript := p.Transcript()
			if len(transcript) < 2 || transcript[0] != "AI0;" || transcript[1] != "ID;" {
				t.Errorf("Open's first two frames = %v, want [\"AI0;\" \"ID;\"] — the session init then the identity probe", transcript[:min(2, len(transcript))])
			}
		})
	}
}

// TestOpen_WrongRadio covers the refusal in BOTH DIRECTIONS between the
// siblings, plus a foreign radio, and asserts the model NAMES as well as
// the IDs.
//
// The sibling case is the one this package exists to get right: an
// FTDX101MP on the other end of a D driver's probe is not some unrelated
// radio but the other half of this very package, and the two answer
// four-digit IDs that differ in their last character. A refusal naming only
// "0682" and "0681" is technically complete and practically useless; the
// M9d-2 A5 extension exists so the refusal can say "identifies as
// FTdx101MP; you selected FTdx101D".
//
// BOTH name fields are asserted, and that is deliberate rather than
// thorough: driver.WrongRadioError.Error() renders its named form only when
// WantModel AND GotModel are set, while cmd/rigprog's probe formatter keys
// on GotModel alone, so a driver populating one field alone would produce
// two different renderings of one refusal. Nothing but an explicit
// assertion on both fields catches that (M9d-2 task 1 review, minor 3).
//
// The foreign case pins the OTHER half of the contract: an ID this package
// does not register leaves GotModel EMPTY, so the ID-only text renders.
// This package knows its own two radios and nothing else — naming another
// package's radio is that package's business — and "0800" is the FT-710,
// the exact radio most likely to be on the other end of a mistake, since it
// shares this family's connector, baud menu and CAT grammar.
//
// Every case additionally pins that the refusal happens BEFORE any
// discovery frame and that the port Open took ownership of is closed.
func TestOpen_WrongRadio(t *testing.T) {
	for _, m := range testModels {
		for _, tt := range []struct {
			name         string
			answeredID   string
			wantGotModel string
		}{
			{
				name:         "the sibling model answers",
				answeredID:   m.siblingCATID,
				wantGotModel: m.siblingName,
			},
			{
				// An FT-710. Not a sentinel model name anywhere: the ID is
				// a real radio's, and no model NAME is asserted because the
				// point is that none is produced.
				name:         "a foreign radio answers, so no name is produced",
				answeredID:   "0800",
				wantGotModel: "",
			},
		} {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				p := newRespondingPort(t, slotImage{catID: tt.answeredID})

				_, err := m.newDrv(RealHardware).Open(testCtx(t), p.Port(), testIdentity)
				if !errors.Is(err, driver.ErrWrongRadio) {
					t.Fatalf("Open = %v, want errors.Is match against driver.ErrWrongRadio", err)
				}
				var wre *driver.WrongRadioError
				if !errors.As(err, &wre) {
					t.Fatalf("Open error %v is not a *driver.WrongRadioError", err)
				}
				if wre.Want != m.catID || wre.Got != tt.answeredID {
					t.Errorf("WrongRadioError IDs = {Want:%q Got:%q}, want {Want:%q Got:%q}", wre.Want, wre.Got, m.catID, tt.answeredID)
				}
				if wre.WantModel != m.name {
					t.Errorf("WrongRadioError.WantModel = %q, want %q — the selected model must always be named, since this driver always knows which one it is", wre.WantModel, m.name)
				}
				if wre.GotModel != tt.wantGotModel {
					t.Errorf("WrongRadioError.GotModel = %q, want %q — siblingNames maps this package's own two CAT IDs and nothing else", wre.GotModel, tt.wantGotModel)
				}

				// The wrong radio is rejected BEFORE discovery: a sibling
				// (or an FT-710) must not be walked through a hundred
				// probes built for another model.
				for _, frame := range p.Transcript() {
					if len(frame) == mtReadFrameLen && strings.HasPrefix(frame, "MT") {
						t.Errorf("Open sent %q after the ID probe failed — discovery must not run against a radio that did not identify", frame)
						break
					}
				}

				// Open took ownership of the port and failed: it must have
				// closed it.
				if _, werr := p.Port().Write([]byte("x")); werr == nil {
					t.Error("the port is still writable after a failed Open — Open must close the port it took ownership of on every error path")
				}
			})
		}
	}
}

// TestOpen_WrongRadio_RendersBothNames pins that the two name fields this
// driver populates actually reach the user-visible text, for the sibling
// case in both directions.
//
// The Error() FORMAT is core/driver's and is pinned there as a literal
// (M9d-2 task 1); what is pinned HERE is that this driver's refusal
// arrives in a shape that renders the named form at all — a refusal
// carrying one name alone would render the ID-only text, which is the
// failure the field assertions above are aimed at and this is the
// end-to-end confirmation of.
func TestOpen_WrongRadio_RendersBothNames(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p := newRespondingPort(t, slotImage{catID: m.siblingCATID})

			_, err := m.newDrv(RealHardware).Open(testCtx(t), p.Port(), testIdentity)
			if err == nil {
				t.Fatal("Open succeeded against the sibling model's CAT ID")
			}
			text := err.Error()
			for _, want := range []string{m.name, m.siblingName, m.catID, m.siblingCATID} {
				if !strings.Contains(text, want) {
					t.Errorf("refusal text %q does not contain %q — a refusal between two radios whose IDs differ in one character must name both models", text, want)
				}
			}
		})
	}
}

// TestOpen_DiscoveryProbesEveryDeclaredSlot is THE discovery test, run for
// EACH model, and it asserts the full ordered transcript rather than only
// the outcome.
//
// The image is deliberately SPARSE, with a gap before every populated slot:
// 503 (after empty 501-502), 599 (the very last declared 5xx slot, after 95
// empty ones) and EMG. Every termination rule this driver refuses would
// produce a DIFFERENT, plausible-looking result against it —
// stop-at-first-rejection finds nothing at all; contiguity-from-501 finds
// nothing; a 15-slot cap finds 503 and misses 599; a sentinel scheme finds
// 503 and reports an anomaly — so the discovered set alone would catch most
// regressions. The TRANSCRIPT is what catches the rest, and catches them by
// name: it pins that all 99 declared 5xx slots were asked, in ascending
// order, with EMG last and nothing else in between.
//
// If this test ever becomes slow enough to be tempting, read doc.go's
// discovery section before touching it. The ~100 probes ARE the design, and
// two models means two of them.
func TestOpen_DiscoveryProbesEveryDeclaredSlot(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			img := slotImage{mtAnswers: map[string]string{
				"503": populatedAnswer("503"),
				"599": populatedAnswer("599"),
				"EMG": populatedAnswer("EMG"),
			}}
			p, sess := openSession(t, m, Simulated, img)

			// --- the transcript: order and completeness ---
			var want []string
			want = append(want, "AI0;", "ID;")
			for n := 501; n <= 599; n++ {
				want = append(want, fmt.Sprintf("MT%03d;", n))
			}
			want = append(want, "MTEMG;")

			got := p.Transcript()
			if !reflect.DeepEqual(got, want) {
				// Report the divergence precisely: a full 102-frame diff is
				// unreadable, and the first difference is always the
				// diagnosis.
				for i := 0; i < len(want) || i < len(got); i++ {
					switch {
					case i >= len(got):
						t.Fatalf("transcript stopped after %d frames at %q; want %q next (%d frames total) — an early-terminating or shortened discovery walk", len(got), got[len(got)-1], want[i], len(want))
					case i >= len(want):
						t.Fatalf("transcript has %d frames, want %d; the first extra is %q", len(got), len(want), got[i])
					case got[i] != want[i]:
						t.Fatalf("transcript[%d] = %q, want %q (frames %d vs %d total)", i, got[i], want[i], len(got), len(want))
					}
				}
			}

			// --- the discovered set ---
			caps := sess.Capabilities()
			sixty, ok := caps.Bank(spec.Bank60m)
			if !ok {
				t.Fatal("session capabilities have no 60M bank, want one discovered from 503 and 599")
			}
			if wantSlots := []string{"503", "599"}; !reflect.DeepEqual(sixty.Slots, wantSlots) {
				t.Errorf("60M.Slots = %v, want %v — a sparse bank must be reported whole, in probe order", sixty.Slots, wantSlots)
			}
			if sixty.Label != "60 m channels" {
				t.Errorf("60M.Label = %q, want %q (matrix §1.3.4 — a CHOICE; the manual's own words are \"5xx (5MHz BAND)\")", sixty.Label, "60 m channels")
			}
			if !sixty.NoBlank {
				t.Error("60M.NoBlank = false, want true (matrix §1.3.4: these channels exist because they answered, and this project's write surface offers no way to blank them)")
			}

			emg, ok := caps.Bank(spec.BankEMG)
			if !ok {
				t.Fatal("session capabilities have no EMG bank, want one discovered from the EMG probe")
			}
			if wantSlots := []string{"EMG"}; !reflect.DeepEqual(emg.Slots, wantSlots) {
				t.Errorf("EMG.Slots = %v, want %v", emg.Slots, wantSlots)
			}
			if emg.Label != "Emergency (EMG)" {
				t.Errorf("EMG.Label = %q, want %q (matrix §1.3.4 — a CHOICE)", emg.Label, "Emergency (EMG)")
			}
			if !emg.NoBlank {
				t.Error("EMG.NoBlank = false, want true (matrix §1.3.4)")
			}

			// --- the discovered banks are READ-ONLY on every field ---
			// No profile may claim a 5xx/EMG slot writable: this session is
			// Simulated, whose static banks ARE write-Supported, so a
			// discovered bank inheriting those writes is a real hazard and
			// not a theoretical one. Half the reason is manual-evidenced
			// (MW's restricted legend, layout 1353) and half is project
			// policy (core/cat/mt.go:100-108) — see caps.go's
			// readOnlyFields.
			for _, bank := range []spec.Bank{sixty, emg} {
				for _, f := range allFields {
					if fs := bank.Fields[f]; fs.Write != spec.Unsupported {
						t.Errorf("discovered bank %s field %s: Write = %s, want Unsupported", bank.ID, f, fs.Write)
					}
				}
				if len(bank.Fields) != len(allFields) {
					t.Errorf("discovered bank %s lists %d fields, want all %d", bank.ID, len(bank.Fields), len(allFields))
				}
			}

			// --- live vs offline, on this very session's own result ---
			// The offline synthesiser, handed exactly the wires discovery
			// found, must produce exactly the banks this live session
			// carries. The table-driven version of this equivalence lives
			// below; this is the one leg that uses a REAL session's output
			// as the input, so the two derivations are compared on data
			// neither test invented.
			synth := m.newDrv(Simulated).(driver.DiscoveredBankSynthesizer)
			discoveredWires := append(append([]string(nil), sixty.Slots...), emg.Slots...)
			base := m.newDrv(Simulated).Capabilities()
			if got, want := synth.SynthesiseDiscoveredBanks(discoveredWires), caps.Banks[len(base.Banks):]; !reflect.DeepEqual(got, want) {
				t.Errorf("SynthesiseDiscoveredBanks(%v) = %#v,\nwant the live session's own discovered banks %#v", discoveredWires, got, want)
			}
		})
	}
}

// TestOpen_NoDiscoveredBanksWhenNothingAnswers: a radio with no 5xx bank
// and no EMG channel — all ~100 probes rejected — is an ordinary outcome,
// not an error, and yields a session with the static banks only.
//
// This is a real variant, not a contrived one: the FT-710 this project
// characterised has no 5xx bank at all (front-panel confirmed), so the
// empty inventory is what a UK-market radio of this family may well answer.
// One model suffices for it — the walk is model-independent, matrix §2.5 —
// and the per-model legs are the transcript test above.
func TestOpen_NoDiscoveredBanksWhenNothingAnswers(t *testing.T) {
	p, sess := openSession(t, testModels[0], Simulated, slotImage{})

	caps := sess.Capabilities()
	if _, ok := caps.Bank(spec.Bank60m); ok {
		t.Error("session has a 60M bank although every 5xx probe was rejected")
	}
	if _, ok := caps.Bank(spec.BankEMG); ok {
		t.Error("session has an EMG bank although the EMG probe was rejected")
	}
	if len(caps.Banks) != 2 {
		t.Errorf("session has %d banks, want the 2 static ones", len(caps.Banks))
	}

	// The walk still happened in full: "nothing answered" must not be
	// reached by asking less.
	if got, want := len(p.Transcript()), 2+99+1; got != want {
		t.Errorf("Open sent %d frames, want %d (AI0, ID, 99 x 5xx, EMG) — an empty inventory is discovered by probing everything, not by stopping early", got, want)
	}
}

// TestOpen_DiscoveryRefusesRatherThanGuesses is the malformed-discovery
// refusal the M9d spec's error-handling rule requires: discovery refuses
// rather than guesses.
//
// Two answers a radio has no business giving, each served for ONE 5xx probe
// mid-walk:
//
//   - a well-formed 41-byte frame whose P7 kind byte is outside the
//     combined record's documented {'0','1'} read pair, which the DIALECT's
//     parser refuses with a *cat.ParseError;
//   - a well-formed frame that echoes a DIFFERENT slot, which this
//     package's own *AnswerMismatchError refuses.
//
// In both cases Open must FAIL with the typed error reachable by
// errors.As — not skip the slot, not stop the walk early and report what it
// had — and must leave nothing behind: no session, no bank synthesised from
// the partial walk, and the port it took ownership of closed. A driver that
// treated either answer as "not populated" would silently under-report an
// inventory; one that treated it as "populated" would put a channel it
// could not read into a capability bank.
func TestOpen_DiscoveryRefusesRatherThanGuesses(t *testing.T) {
	for _, tt := range []struct {
		name   string
		answer string
		check  func(t *testing.T, err error)
	}{
		{
			name:   "a malformed answer mid-walk",
			answer: malformedAnswer("503"),
			check: func(t *testing.T, err error) {
				var pe *cat.ParseError
				if !errors.As(err, &pe) {
					t.Fatalf("Open error %v (%T) is not a wrapped *cat.ParseError — the answer vocabulary is the PARSER's to enforce and its typed verdict must survive discovery's wrap", err, err)
				}
				if !strings.Contains(pe.Reason, "P7") {
					t.Errorf("ParseError.Reason = %q, want it to name the offending field (P7)", pe.Reason)
				}
			},
		},
		{
			name:   "an answer echoing the wrong slot mid-walk",
			answer: wrongSlotAnswer("504"),
			check: func(t *testing.T, err error) {
				if !errors.Is(err, ErrAnswerMismatch) {
					t.Fatalf("Open = %v, want errors.Is match against ErrAnswerMismatch", err)
				}
				var ame *AnswerMismatchError
				if !errors.As(err, &ame) {
					t.Fatalf("Open error %v (%T) is not an *AnswerMismatchError", err, err)
				}
				if ame.Requested != "503" || ame.Answered != "504" {
					t.Errorf("AnswerMismatchError = {Requested:%q Answered:%q}, want {\"503\" \"504\"}", ame.Requested, ame.Answered)
				}
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := testModels[0]
			// 501 answers properly, so the walk is genuinely under way and
			// has something it COULD have reported when the bad answer
			// arrives at 503.
			p := newRespondingPort(t, slotImage{
				catID: m.catID,
				mtAnswers: map[string]string{
					"501": populatedAnswer("501"),
					"503": tt.answer,
				},
			})

			sess, err := m.newDrv(Simulated).Open(testCtx(t), p.Port(), testIdentity)
			if err == nil {
				_ = sess.Close()
				t.Fatal("Open succeeded against a radio that answered a discovery probe with something this protocol does not define — discovery must refuse rather than guess")
			}
			if sess != nil {
				t.Errorf("Open returned a session (%T) alongside its error — a failed discovery must yield no session at all, and certainly no bank synthesised from the partial walk", sess)
			}
			tt.check(t, err)

			// The walk stopped AT the bad answer rather than carrying on:
			// AI0, ID, then 501, 502, 503 and nothing after it.
			if got, want := len(p.Transcript()), 2+3; got != want {
				t.Errorf("Open sent %d frames, want %d (AI0, ID, MT501, MT502, MT503) — the walk must stop at the refusal, not continue past it", got, want)
			}

			if _, werr := p.Port().Write([]byte("x")); werr == nil {
				t.Error("the port is still writable after a failed Open — Open must close the port it took ownership of on every error path")
			}
		})
	}
}

// TestSession_CapabilitiesIsADefensiveCopy: mutating what
// Session.Capabilities returned must never alter what the session itself
// enforces. This matters more than usual for Fields: that map IS the write
// gate's data.
func TestSession_CapabilitiesIsADefensiveCopy(t *testing.T) {
	_, sess := openSession(t, testModels[0], Simulated, slotImage{})

	first := sess.Capabilities()
	mem, ok := first.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("missing MEM bank")
	}
	mem.Fields[spec.FieldErase] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	first.Banks[0].Slots[0] = "TAMPERED"
	first.Bauds[0] = 1
	first.RequiredSlots[0] = "999"

	second := sess.Capabilities()
	if fs := second.FieldSupport(spec.BankMemory, spec.FieldErase); fs.CanWrite() {
		t.Error("FieldErase became writable in a fresh Capabilities() after a caller tampered with an earlier copy — the write gate's data is aliased")
	}
	if second.Banks[0].Slots[0] != "001" {
		t.Errorf("MEM.Slots[0] = %q after tampering with an earlier copy, want \"001\"", second.Banks[0].Slots[0])
	}
	if second.Bauds[0] != 4800 || second.RequiredSlots[0] != "001" {
		t.Errorf("Bauds/RequiredSlots = %v/%v after tampering, want [4800 ...]/[001]", second.Bauds, second.RequiredSlots)
	}
}

// TestSession_CloseIdempotent: Close may be called twice.
func TestSession_CloseIdempotent(t *testing.T) {
	_, sess := openSession(t, testModels[0], Simulated, slotImage{})

	if err := sess.Close(); err != nil {
		t.Errorf("first Close() = %v, want nil", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil (idempotent)", err)
	}
}

// TestSession_DiagnosticsCountsUnexpectedFrames: the session surfaces the
// engine's own unexpected-frame counter, which is otherwise unreachable. A
// fresh session has seen none.
//
// cmd/rigprog's probe command prints this for any session satisfying
// driver.DiagnosticsReporter, so implementing it is what gives `probe
// --model FTdx101D` the same wire-health line an FT-710 probe prints.
func TestSession_DiagnosticsCountsUnexpectedFrames(t *testing.T) {
	_, sess := openSession(t, testModels[0], Simulated, slotImage{})

	if got := sess.Diagnostics(); got.UnexpectedFrames != 0 {
		t.Errorf("Diagnostics() = %+v on a fresh session, want UnexpectedFrames 0", got)
	}
}

// TestSession_DoesNotReportRegion pins an ABSENCE, deliberately: this
// driver does not implement driver.RegionReporter, for either model, and
// must not acquire one by accident.
//
// The FT-710's Region() maps a discovered 5xx/EMG count onto a
// regulatory-region name ("UK" at 7 channels and no EMG, "US" at 15 plus
// EMG) using fingerprints that are that radio's own — partly
// hardware-confirmed, partly still assumed. No FTdx101 inventory of either
// model has ever been observed, so there is no vocabulary this driver could
// report honestly, and borrowing the FT-710's would mean answering a
// question about one radio with another radio's evidence. Callers already
// tolerate absence (the capability is reached by a two-result type
// assertion), so codeplug.RadioInfo.Region simply stays unset.
//
// A Stage R session that enumerates a real 5xx bank is what would make a
// region vocabulary possible — per model — at which point this test is
// deleted with the same commit that adds the method, and the evidence goes
// in the commit message. It is NOT a gap to be filled meanwhile, which is
// exactly why the absence is asserted rather than merely mentioned.
func TestSession_DoesNotReportRegion(t *testing.T) {
	var sess driver.Session = &Session{}
	if rr, ok := sess.(driver.RegionReporter); ok {
		t.Errorf("*ftdx101.Session implements driver.RegionReporter (Region() = %q) — this driver has no honest region vocabulary; see this test's doc comment and doc.go before adding one", rr.Region())
	}
}

// The write path's tests are in write_test.go. (TestWriteChannel_RefusedUntilTask3
// stood here while WriteChannel was a task-2 placeholder, and was deleted by
// the same commit that replaced the placeholder — its own doc comment named
// that handover.)

// TestSynthesiseDiscoveredBanks_MatchesLiveDiscovery is the live-vs-offline
// equivalence, table-driven over every input class the offline path can
// actually meet: sparse wires, input order that is not sorted order, the
// LAST declared 5xx slot, EMG alone and alongside, malformed wire forms,
// and slots a static bank already claims — for BOTH models.
//
// The expectation is not hand-written bank literals but
// effectiveCapabilities' OWN output for the equivalent discovery — the same
// function Open calls — so this asserts the two derivations agree across
// the whole surface (label, ID, Slots, NoBlank and every field-support
// value) rather than across whichever fields a hand-written fixture
// happened to mention. SynthesiseDiscoveredBanks reuses that function
// internally, which is what makes the agreement structural; this test is
// what keeps it so if either side is ever rewritten.
//
// The companion leg, over a REAL session's discovered wires, is in the
// discovery test above — the one place the input is data neither test
// invented.
func TestSynthesiseDiscoveredBanks_MatchesLiveDiscovery(t *testing.T) {
	for _, m := range testModels {
		drv := m.newDrv(Simulated)
		synth, ok := drv.(driver.DiscoveredBankSynthesizer)
		if !ok {
			t.Fatalf("%s: the driver does not implement driver.DiscoveredBankSynthesizer", m.name)
		}
		base := drv.Capabilities()

		for _, tt := range []struct {
			name    string
			input   []string
			want60m []string
			wantEMG bool
		}{
			{
				name:    "sparse wires, as discovery would have found them",
				input:   []string{"503", "599"},
				want60m: []string{"503", "599"},
			},
			{
				name:    "input ORDER is preserved, not sorted",
				input:   []string{"599", "503", "501"},
				want60m: []string{"599", "503", "501"},
			},
			{
				name:    "the last declared 5xx slot alone",
				input:   []string{"599"},
				want60m: []string{"599"},
			},
			{
				name:    "EMG alone",
				input:   []string{"EMG"},
				wantEMG: true,
			},
			{
				name:    "5xx and EMG together",
				input:   []string{"503", "EMG"},
				want60m: []string{"503"},
				wantEMG: true,
			},
			{
				// Unclassifiable wire forms are OMITTED, never guessed into
				// a bank: "600" is three digits but outside every declared
				// range, "5011" is the right prefix at the wrong width, and
				// the empty string is what a malformed file hands over.
				name:  "malformed wires are omitted",
				input: []string{"0X1", "garbage", "", "600", "5011"},
			},
			{
				// A slot one of this driver's own static banks already
				// claims never appears in a discovered bank.
				//
				// HONEST ABOUT WHAT THIS CANNOT ATTRIBUTE: two independent
				// refusals stand in the way, and for this radio's bank
				// shapes no input can separate them. SynthesiseDiscoveredBanks
				// excludes claimed slots explicitly, AND a MEM ("001"-"099")
				// or PMS ("P1L"-"P9U") wire form classifies as neither 60m
				// nor EMG, so it would be omitted even with the exclusion
				// deleted. The case therefore pins the OUTCOME the contract
				// promises, not the mechanism. The explicit exclusion earns
				// its keep against a future static bank that claimed a 5xx
				// or EMG wire form — a shape this driver's capability data
				// cannot currently produce, which is why no test here can
				// exercise it — where its absence would put one slot in two
				// banks and fail spec.Capabilities.Validate.
				name:  "statically-claimed slots are excluded",
				input: []string{"001", "099", "P1L", "P9U"},
			},
			{
				name:    "a realistic mixture of all five classes",
				input:   []string{"001", "503", "0X1", "EMG", "P1L", "599"},
				want60m: []string{"503", "599"},
				wantEMG: true,
			},
			{
				name:  "nil input produces no banks",
				input: nil,
			},
		} {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				got := synth.SynthesiseDiscoveredBanks(tt.input)
				want := effectiveCapabilities(m.params.dialect, base, tt.want60m, tt.wantEMG).Banks[len(base.Banks):]
				if !reflect.DeepEqual(got, want) {
					t.Errorf("SynthesiseDiscoveredBanks(%v) =\n %#v\nwant (effectiveCapabilities' own discovered banks)\n %#v", tt.input, got, want)
				}
			})
		}
	}
}

// TestSynthesiseDiscoveredBanks_DuplicateEMGCollapses pins this driver's
// choice for a semantically invalid input: a repeated "EMG" yields ONE EMG
// slot, the single physical channel.
//
// Live discovery probes one EMG slot and can never produce a duplicate, so
// collapsing keeps offline synthesis identical to what a live session
// carries — which is the property the equivalence test above depends on.
// core/driver/ft710 makes the OPPOSITE choice, preserving every occurrence,
// for a compatibility reason of its own; that is its history, not a rule
// this driver inherits. Pinned so it stays a decision rather than an
// accident of whichever loop shape was written first.
func TestSynthesiseDiscoveredBanks_DuplicateEMGCollapses(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			synth := m.newDrv(Simulated).(driver.DiscoveredBankSynthesizer)

			got := synth.SynthesiseDiscoveredBanks([]string{"EMG", "EMG", "EMG"})

			var emg *spec.Bank
			for i := range got {
				if got[i].ID == spec.BankEMG {
					emg = &got[i]
				}
			}
			if emg == nil {
				t.Fatalf("SynthesiseDiscoveredBanks([EMG EMG EMG]) produced no EMG bank; got %#v", got)
			}
			if want := []string{"EMG"}; !reflect.DeepEqual(emg.Slots, want) {
				t.Errorf("EMG.Slots = %v, want %v (one physical channel, however many times the input names it)", emg.Slots, want)
			}
		})
	}
}

// TestSynthesiseDiscoveredBanks_NeverClaimsAWritableDiscoveredBank: the
// offline path must force every Write to Unsupported exactly as live
// discovery does, on the SIMULATED profile in particular — the one whose
// static banks are genuinely write-Supported, so an inherited Fields map
// would advertise writes to 5xx/EMG slots that cat.Dialect's own write
// policies refuse to build.
func TestSynthesiseDiscoveredBanks_NeverClaimsAWritableDiscoveredBank(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			synth := m.newDrv(Simulated).(driver.DiscoveredBankSynthesizer)

			banks := synth.SynthesiseDiscoveredBanks([]string{"501", "EMG"})
			if len(banks) != 2 {
				t.Fatalf("SynthesiseDiscoveredBanks([501 EMG]) produced %d banks, want 2", len(banks))
			}
			for _, b := range banks {
				if !b.NoBlank {
					t.Errorf("bank %s NoBlank = false, want true", b.ID)
				}
				for _, f := range allFields {
					if fs := b.Fields[f]; fs.Write != spec.Unsupported {
						t.Errorf("bank %s field %s: Write = %s, want Unsupported on a discovered bank", b.ID, f, fs.Write)
					}
				}
			}
		})
	}
}

// captureLogger is a transport.Logger that keeps what it was told.
type captureLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *captureLogger) Printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *captureLogger) Lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.lines...)
}

// TestWithTransportLogger_ReachesTheEngine proves the option is plumbed
// rather than merely accepted: an UNEXPECTED frame arriving ahead of a
// read's real answer must be surfaced to the caller's own logger and
// counted, and the read must still succeed.
//
// Transport safety obligation 3 is "unexpected frames are surfaced, never
// silently discarded", and the engine's default logger drops everything —
// so without this option a driver gives its caller no way to receive that
// signal at all, and a test that only checked the field was set would not
// notice the wiring being removed. A nonzero unexpected-frame count on an
// otherwise-working session is real diagnostic information (another
// application sharing the port; replies arriving late enough to miss their
// windows), and cmd/rigprog's probe prints it.
func TestWithTransportLogger_ReachesTheEngine(t *testing.T) {
	m := testModels[0]
	img := readTestImage()
	img.catID = m.catID
	p := newRespondingPort(t, img)
	log := &captureLogger{}

	opened, err := m.newDrv(Simulated, WithTransportLogger(log)).Open(testCtx(t), p.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	sess := opened.(*Session)

	// Slot 020's answer is preceded by a junk frame — see readTestImage.
	ch, err := sess.ReadChannel(testCtx(t), "020")
	if err != nil {
		t.Fatalf("ReadChannel(\"020\") = %v, want nil: an unexpected frame is logged and counted, not fatal, and the real answer still arrives inside the same timeout budget", err)
	}
	if ch.Data == nil || ch.Data.Tag != "CALLING" {
		t.Errorf("ReadChannel(\"020\") = %+v, want the populated channel behind the junk frame", ch.Data)
	}

	if got := sess.Diagnostics().UnexpectedFrames; got != 1 {
		t.Errorf("Diagnostics().UnexpectedFrames = %d, want 1", got)
	}

	lines := log.Lines()
	if len(lines) == 0 {
		t.Fatal("the caller's transport.Logger received nothing — WithTransportLogger is not reaching the engine, so every transport diagnostic this session produces is being dropped")
	}
	found := false
	for _, l := range lines {
		if strings.Contains(l, "unexpected frame") {
			found = true
		}
	}
	if !found {
		t.Errorf("logger lines %q do not mention an unexpected frame", lines)
	}
}

// TestWithTransportLogger_NilIsIgnored: a nil logger leaves the engine's own
// drop-everything default in place rather than installing a nil that would
// panic on the first diagnostic. Asserted for both constructors, since each
// threads its own options through newDriver.
func TestWithTransportLogger_NilIsIgnored(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			d, ok := m.newDrv(Simulated, WithTransportLogger(nil)).(*ftdx101Driver)
			if !ok {
				t.Fatal("the constructor did not return a *ftdx101Driver")
			}
			if d.transportLogger != nil {
				t.Error("WithTransportLogger(nil) installed a logger, want the engine's default kept")
			}
		})
	}
}

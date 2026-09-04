// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// These tests drive the driver against respondingPort — a scripted,
// frame-parsing CAT peer (respondingport_test.go) — through the driver's
// own real transport.Engine, constructed inside Open. There is no fake
// radio here: internal/fakeft891 is lane B's, and the error paths these
// tests need (a foreign CAT ID, an answer naming the wrong slot, an MT
// rejection over a slot MR reports as occupied, silence) are answers a
// self-consistent fake would never give.
//
// Every Open in this package costs the AI0 init's error window plus THIRTEEN
// exchanges — ID and the eleven discovery probes — at the engine's default
// 20 ms per-exchange settle. That is a fraction of the FTdx10's ~100 probes,
// because this radio's manual prints its 5 MHz bank's actual bounds (501-510,
// layout 962) where the FTdx10's prints only "5xx"; it is still the accepted,
// budgeted price of walking the WHOLE declared range (doc.go) and NOT
// something to be optimised away with a settle override.
const testCtxTimeout = 60 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// testIdentity is the caller-side Identity every test session opens with.
var testIdentity = driver.Identity{Port: "scripted-pipe", USBSerial: "SIM0650"}

// openSession starts a respondingPort serving img, opens a session over it
// with the given profile, and registers cleanup for both. It returns the
// port (for its transcript) and the concrete *Session.
func openSession(t *testing.T, profile Profile, img slotImage, opts ...Option) (*respondingPort, *Session) {
	t.Helper()
	p := newRespondingPort(t, img)

	sess, err := New(profile, opts...).Open(testCtx(t), p.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned a %T, want *ft891.Session", sess)
	}
	return p, s
}

// TestDriver_ModelAndIdentity: the driver names itself consistently, and
// core/driver.Driver's contract that Model() equals Capabilities().Model
// holds on every profile.
func TestDriver_ModelAndIdentity(t *testing.T) {
	for _, profile := range []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
		{"unrecognised", Profile(99)},
	} {
		t.Run(profile.name, func(t *testing.T) {
			d := New(profile.p)
			if d.Model() != "FT-891" {
				t.Errorf("Model() = %q, want \"FT-891\"", d.Model())
			}
			if got := d.Capabilities().Model; got != d.Model() {
				t.Errorf("Capabilities().Model = %q, Model() = %q — driver.Driver requires them equal", got, d.Model())
			}
		})
	}
}

// TestDriver_ProfileSelection pins which capability set each Profile value
// selects, INCLUDING the zero value and an unrecognised one (matrix §2.1).
//
// The zero value must be RealHardware — a forgotten Profile must not select
// the simulator's writable set — and an unrecognised value must fail safe
// to the same all-Unverified set, which is the property that holds whatever
// a forged or corrupted Profile carries. caps_test.go pins the CONTENTS of
// both sets field by field; this pins the SELECTION.
func TestDriver_ProfileSelection(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Profile
		want spec.Capabilities
	}{
		{"RealHardware (writeTrialsComplete false)", RealHardware, CapabilitiesUnverified()},
		{"the zero-value Profile is RealHardware", Profile(0), CapabilitiesUnverified()},
		{"an unrecognised Profile fails safe", Profile(99), CapabilitiesUnverified()},
		{"a negative Profile fails safe", Profile(-1), CapabilitiesUnverified()},
		{"Simulated", Simulated, CapabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.p).Capabilities(); !reflect.DeepEqual(got, tt.want) {
				reportCapsDifference(t, got, tt.want)
				t.Error("New(profile).Capabilities() is not the pinned profile constructor's product (see above)")
			}
		})
	}
}

// TestOpen_ProbesIdentityAndPopulatesIt: a successful Open probes ID; and
// puts the ANSWER on the session's Identity, keeping the caller's port and
// USB serial.
//
// The first two frames are pinned here as well as in the discovery test:
// AI0; then ID; (matrix erratum M-E1, spec erratum S-E5). The AI0 preamble
// is transport.Engine.Init's, shared by every registered Yaesu driver;
// "ID before AI0" is a fleet seam outside this milestone, and the spec's
// "before any OTHER frame" was withdrawn in favour of "before any DISCOVERY
// frame", which the wrong-radio test pins.
func TestOpen_ProbesIdentityAndPopulatesIt(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	id := sess.Identity()
	if id.CATID != "0650" {
		t.Errorf("Identity().CATID = %q, want \"0650\" (the probe's answer is authoritative)", id.CATID)
	}
	if id.Port != testIdentity.Port || id.USBSerial != testIdentity.USBSerial {
		t.Errorf("Identity() = %+v, want the caller's Port/USBSerial preserved (%+v)", id, testIdentity)
	}

	transcript := p.Transcript()
	if len(transcript) < 2 || transcript[0] != "AI0;" || transcript[1] != "ID;" {
		t.Errorf("Open's first two frames = %v, want [\"AI0;\" \"ID;\"] — the family's AI0 preamble then the identity probe (matrix M-E1)", transcript[:min(2, len(transcript))])
	}
}

// TestOpen_WrongRadio: the port answers ID; with "ID0800;" — an FT-710, the
// exact radio most likely to be on the other end of a mistake, since it
// shares this family's connector, baud menu and CAT grammar and would answer
// plenty of FT-891 frames plausibly.
//
// Open must fail with a typed *driver.WrongRadioError, must never reach
// discovery, and must close the port it took ownership of.
//
// THE ERROR'S SHAPE IS PLAN DECISION P1 AND SPEC ERRATUM S-E1, and both
// halves are pinned: WantModel is "FT-891" and GotModel is EMPTY, so
// Error() renders its ID-ONLY sentence. driver.WrongRadioError.Error()
// renders the NAMED form only when BOTH names are populated, and
// cmd/rigprog's probe formatter keys on GotModel alone, so a driver filling
// one alone would render the same refusal two different ways. THE FT-891
// HAS NO SIBLING ID TABLE — naming another radio would mean carrying
// another manual's ID as this package's data — so the honest answer is the
// WANT side only (matrix §3.10, recorded there and decided by the spec).
//
// The rendered text is pinned VERBATIM because rendered refusals are
// recorded in baselines.
func TestOpen_WrongRadio(t *testing.T) {
	p := newRespondingPort(t, slotImage{catID: "0800"})

	_, err := New(RealHardware).Open(testCtx(t), p.Port(), testIdentity)
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open = %v, want errors.Is match against driver.ErrWrongRadio", err)
	}
	var wre *driver.WrongRadioError
	if !errors.As(err, &wre) {
		t.Fatalf("Open error %v is not a *driver.WrongRadioError", err)
	}
	if wre.Want != "0650" || wre.Got != "0800" {
		t.Errorf("WrongRadioError = {Want:%q Got:%q}, want {Want:\"0650\" Got:\"0800\"}", wre.Want, wre.Got)
	}
	if wre.WantModel != "FT-891" {
		t.Errorf("WrongRadioError.WantModel = %q, want \"FT-891\" (P1/S-E1: \"with names\" is satisfied on the WANT side)", wre.WantModel)
	}
	if wre.GotModel != "" {
		t.Errorf("WrongRadioError.GotModel = %q, want \"\" — this driver has no sibling ID table, and populating one name alone makes Error() and cmd/rigprog's probe formatter render the same refusal two different ways (P1/S-E1)", wre.GotModel)
	}
	const wantText = `driver: connected radio identified as CAT ID "0800", want "0650" — wrong radio model on this port`
	if got := wre.Error(); got != wantText {
		t.Errorf("Error() = %q,\nwant the ID-only sentence %q", got, wantText)
	}

	// The wrong radio is rejected BEFORE any discovery traffic: an FT-710
	// must not be walked through eleven FT-891 probes. Asserted as the
	// WHOLE transcript rather than as "no MR frame", so a future extra
	// frame of any kind on this path fails here too.
	if got, want := p.Transcript(), []string{"AI0;", "ID;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want exactly %v — a wrong radio receives the AI0 preamble and the ID probe and NOTHING more (S-E5)", got, want)
	}

	// Open took ownership of the port and failed: it must have closed it.
	if _, werr := p.Port().Write([]byte("x")); werr == nil {
		t.Error("the port is still writable after a failed Open — Open must close the port it took ownership of on every error path")
	}
}

// discoveryTranscript is the FULL ordered frame list a successful Open
// sends: the AI0 preamble, the ID probe, then an MR read of every declared
// 5xx slot ascending and EMG last. ELEVEN probes, and the numbers are the
// MANUAL's (MR's slot legend, "501 - 510 (5 MHz, U.S. and U.K. version
// only)" and "EMG (Emergency)", layout 962 and 964) — written out here
// rather than derived from the dialect, so this fixture asserts the chart
// rather than agreeing with whatever the code walked.
func discoveryTranscript() []string {
	want := []string{"AI0;", "ID;"}
	for n := 501; n <= 510; n++ {
		want = append(want, fmt.Sprintf("MR%03d;", n))
	}
	return append(want, "MREMG;")
}

// TestOpen_DiscoveryProbesEveryDeclaredSlot is THE discovery test, and it
// asserts the full ordered transcript rather than only the outcome.
//
// The image is deliberately SPARSE, with a gap before every populated slot:
// 503 (after empty 501-502), 510 (the very last declared 5xx slot) and EMG.
// Every termination rule this driver refuses would produce a DIFFERENT,
// plausible-looking result against it — stop-at-first-rejection finds
// nothing at all; contiguity-from-501 finds nothing; a cap short of ten
// finds 503 and misses 510; a sentinel scheme finds 503 and reports an
// anomaly — so the discovered set alone would catch most regressions. The
// TRANSCRIPT is what catches the rest, and catches them by name.
//
// THE PROBES ARE MR READS. That is this radio's own departure from both
// combined-form siblings, and it is not a preference: MT's slot legend here
// prints memory and PMS only (layout 998-999) where MR's prints all four
// classes (960-964), so the dialect's MTPolicy.ReadSlots =
// cat.MTReadsMemoryPMS makes the codec and the outbound gate refuse to
// build an "MT501;" at all. The negative half is its own test below.
func TestOpen_DiscoveryProbesEveryDeclaredSlot(t *testing.T) {
	img := slotImage{mrAnswers: map[string]string{
		"503": populatedMR("503"),
		"510": populatedMR("510"),
		"EMG": populatedMR("EMG"),
	}}
	p, sess := openSession(t, Simulated, img)

	want := discoveryTranscript()
	got := p.Transcript()
	if !reflect.DeepEqual(got, want) {
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
		t.Fatal("session capabilities have no 60M bank, want one discovered from 503 and 510")
	}
	if wantSlots := []string{"503", "510"}; !reflect.DeepEqual(sixty.Slots, wantSlots) {
		t.Errorf("60M.Slots = %v, want %v — a sparse bank must be reported whole, in probe order", sixty.Slots, wantSlots)
	}
	if sixty.Label != bank60mLabel {
		t.Errorf("60M.Label = %q, want %q", sixty.Label, bank60mLabel)
	}
	if !sixty.NoBlank {
		t.Error("60M.NoBlank = false, want true (these channels exist because they answered, and this radio's CAT surface cannot blank them — matrix §2.4)")
	}

	emg, ok := caps.Bank(spec.BankEMG)
	if !ok {
		t.Fatal("session capabilities have no EMG bank, want one discovered from the EMG probe")
	}
	if wantSlots := []string{"EMG"}; !reflect.DeepEqual(emg.Slots, wantSlots) {
		t.Errorf("EMG.Slots = %v, want %v", emg.Slots, wantSlots)
	}
	if emg.Label != bankEMGLabel {
		t.Errorf("EMG.Label = %q, want %q", emg.Label, bankEMGLabel)
	}

	// --- the discovered banks are READ-ONLY, and their tag pair is ZERO ---
	// No profile may claim a 5xx/EMG slot writable: this session is
	// Simulated, whose static banks ARE write-Supported, so a discovered
	// bank inheriting those writes is a real hazard and not a theoretical
	// one. And the tag pair is the ZERO FieldSupport, not merely
	// unwritable, because MR's 28-position answer carries neither (P4,
	// matrix §2.5).
	for _, bank := range []spec.Bank{sixty, emg} {
		if len(bank.Fields) != len(allFields) {
			t.Errorf("discovered bank %s lists %d fields, want all %d", bank.ID, len(bank.Fields), len(allFields))
		}
		for _, f := range allFields {
			fs := bank.Fields[f]
			if fs.Write != spec.Unsupported {
				t.Errorf("discovered bank %s field %s: Write = %s, want Unsupported", bank.ID, f, fs.Write)
			}
			if (f == spec.FieldTag || f == spec.FieldTagDisplay) && fs != (spec.FieldSupport{}) {
				t.Errorf("discovered bank %s field %s = {Read:%s Write:%s}, want the ZERO FieldSupport — MR carries neither (P4, matrix §2.5)", bank.ID, f, fs.Read, fs.Write)
			}
		}
	}

	// --- live vs offline, on this very session's own result ---
	// The offline synthesiser, handed exactly the wires discovery found,
	// must produce exactly the banks this live session carries. The
	// table-driven version of this equivalence lives in optional_test.go;
	// this is the one leg whose input is data neither test invented.
	synth := New(Simulated).(driver.DiscoveredBankSynthesizer)
	discoveredWires := append(append([]string(nil), sixty.Slots...), emg.Slots...)
	base := New(Simulated).Capabilities()
	if got, want := synth.SynthesiseDiscoveredBanks(discoveredWires), caps.Banks[len(base.Banks):]; !reflect.DeepEqual(got, want) {
		t.Errorf("SynthesiseDiscoveredBanks(%v) = %#v,\nwant the live session's own discovered banks %#v", discoveredWires, got, want)
	}
}

// TestOpen_NeverBuildsAnMTReadOfADiscoveredSlot is the NEGATIVE pin the
// spec's §Testing requires alongside the discovery transcript: no MT read
// of a 5xx or EMG slot is ever built, by discovery or by anything else.
//
// It is not redundant with the transcript equality above. That test pins
// what a SUCCESSFUL walk sends; this one states the prohibition as its own
// property, over a radio that answers every probe, so that a future change
// which added an MT confirmation to discovery fails with the reason named
// rather than as an unexplained transcript diff. The codec and the outbound
// gate refuse such a frame anyway (MTPolicy.ReadSlots =
// cat.MTReadsMemoryPMS), so a driver that tried would fail — this is what
// makes the failure legible.
//
// THE TRANSCRIPT EQUALITY BELOW IS THE NON-VACUITY HALF (LOW-1, task-1
// review). A `continue`-loop that only fires on an "MT" prefix passes
// silently over an empty transcript, and Open sends no "MT" frame at all —
// so without this assertion the loop's body never runs and the test would
// pass just as happily if discovery were removed outright. This image
// answers every one of the eleven declared probes, so it must produce
// discoveryTranscript() exactly; asserting that first proves the walk
// actually happened in full before the negative check over its frames
// means anything.
//
// THE WALK DOES NOT STOP AT OPEN (LOW-1, task-1 review). The prohibition
// this test states — no MT read of a 5xx or EMG slot, ever — is a claim
// about the whole session, and readDiscovered (read.go) is the OTHER site
// that could violate it, at ReadChannel time rather than Open's. So after
// Open this test reads back every discovered slot the image answered, and
// re-checks the negative property over the FULL Open+read transcript. Each
// such ReadChannel call sends its own "MR5xx;"/"MREMG;" frame, which is
// this walk's non-vacuity for the read-time half specifically: the count
// assertion below fails if readDiscovered stopped sending anything at all,
// exactly as the Open-only equality above fails if discovery did.
func TestOpen_NeverBuildsAnMTReadOfADiscoveredSlot(t *testing.T) {
	img := slotImage{mrAnswers: map[string]string{}}
	discovered := make([]string, 0, 11)
	for n := 501; n <= 510; n++ {
		slot := fmt.Sprintf("%03d", n)
		img.mrAnswers[slot] = populatedMR(slot)
		discovered = append(discovered, slot)
	}
	img.mrAnswers["EMG"] = populatedMR("EMG")
	discovered = append(discovered, "EMG")

	p, sess := openSession(t, Simulated, img)

	got := p.Transcript()
	if want := discoveryTranscript(); !reflect.DeepEqual(got, want) {
		t.Fatalf("transcript = %v, want %v — this image answers every probe, so the full eleven-frame walk must have happened before the negative MT check below means anything", got, want)
	}

	beforeReads := len(p.Transcript())
	for _, slot := range discovered {
		if _, err := sess.ReadChannel(testCtx(t), slot); err != nil {
			t.Fatalf("ReadChannel(%q) after Open: unexpected error %v", slot, err)
		}
	}
	if got, want := len(p.Transcript())-beforeReads, len(discovered); got != want {
		t.Fatalf("the read-back sent %d frames after Open, want exactly %d (one MR frame per discovered slot) — the read-time half of this walk is vacuous unless each ReadChannel call actually reached the wire", got, want)
	}

	full := p.Transcript()
	for i, frame := range full {
		if !strings.HasPrefix(frame, "MT") {
			continue
		}
		slot := ""
		if len(frame) >= 5 {
			slot = frame[2:5]
		}
		if slot == "EMG" || (len(slot) == 3 && slot[0] == '5') {
			t.Errorf("frame %d = %q: this driver must NEVER build an MT read of a 5xx or EMG slot — MT's own slot legend prints memory and PMS only (layout 998-999) and the dialect refuses the frame (spec §Testing, matrix §3.4)", i, frame)
		}
	}
}

// TestOpen_NoDiscoveredBanksWhenNothingAnswers: a radio with no 5xx bank
// and no EMG channel — all eleven probes rejected — is an ordinary outcome,
// not an error, and yields a session with the static banks only.
//
// This is a real variant, not a contrived one: the bank is printed "U.S.
// and U.K. version only" (layout 962), so a unit from another market
// answering nothing is exactly what discovery exists to find out.
func TestOpen_NoDiscoveredBanksWhenNothingAnswers(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

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
	if got, want := p.Transcript(), discoveryTranscript(); !reflect.DeepEqual(got, want) {
		t.Errorf("Open sent %v,\nwant %v — an empty inventory is discovered by probing everything, not by stopping early", got, want)
	}
}

// TestOpen_DiscoveryRefusesRatherThanGuesses: a malformed MR answer, and an
// answer naming a DIFFERENT slot than the probe asked about, each make Open
// FAIL with the typed error rather than quietly treating the slot as absent
// or adding the wrong channel to a bank.
//
// "?;" means absent (the ASSUMED register's "?;" ON A 5xx/EMG DISCOVERY
// PROBE entry) and a well-formed answer means present; ANYTHING ELSE is a
// radio this driver does not understand, and the honest response is to
// refuse the session rather than to publish an inventory derived from a
// walk that went wrong. The plan says so in terms: "malformed → refuse".
func TestOpen_DiscoveryRefusesRatherThanGuesses(t *testing.T) {
	t.Run("a malformed answer is a *cat.ParseError, not an absent slot", func(t *testing.T) {
		// P9 (positions 25-26, offsets 24-25) must be the documented fixed
		// "00" — MR's own legend at layout 978 — so an answer carrying "99"
		// there is one core/cat refuses. Corrupted BY POSITION rather than
		// by substring, so the fixture cannot silently stop corrupting
		// anything if a neighbouring field's digits change.
		malformed := []byte(populatedMR("502"))
		malformed[24], malformed[25] = '9', '9'
		img := slotImage{mrAnswers: map[string]string{"502": string(malformed)}}
		p := newRespondingPort(t, img)
		sess, err := New(Simulated).Open(testCtx(t), p.Port(), testIdentity)
		if err == nil {
			_ = sess.Close()
			t.Fatal("Open succeeded against a malformed discovery answer — a frame this driver cannot parse must refuse the session, not be read as an absent slot")
		}
		if !strings.Contains(err.Error(), "502") {
			t.Errorf("Open error %q does not name the slot being probed", err.Error())
		}
		if _, werr := p.Port().Write([]byte("x")); werr == nil {
			t.Error("the port is still writable after a failed Open")
		}
	})

	t.Run("an answer naming another slot is the driver's own typed error", func(t *testing.T) {
		img := slotImage{mrAnswers: map[string]string{
			"504": populatedMR("505"),
		}}
		p := newRespondingPort(t, img)
		sess, err := New(Simulated).Open(testCtx(t), p.Port(), testIdentity)
		if err == nil {
			_ = sess.Close()
			t.Fatal("Open succeeded against an answer naming a different slot")
		}
		var ame *AnswerMismatchError
		if !errors.As(err, &ame) {
			t.Fatalf("Open error %v (%T) is not an *AnswerMismatchError", err, err)
		}
		if ame.Requested != "504" || ame.Answered != "505" {
			t.Errorf("AnswerMismatchError = {Requested:%q Answered:%q}, want {\"504\" \"505\"}", ame.Requested, ame.Answered)
		}
	})
}

// TestSession_CapabilitiesIsADefensiveCopy: mutating what
// Session.Capabilities returned must never alter what the session itself
// enforces. This matters more than usual for Fields: that map IS the write
// gate's data.
func TestSession_CapabilitiesIsADefensiveCopy(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

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
	_, sess := openSession(t, Simulated, slotImage{})

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
// --model FT-891` the same wire-health line an FT-710 probe prints.
func TestSession_DiagnosticsCountsUnexpectedFrames(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	if got := sess.Diagnostics(); got.UnexpectedFrames != 0 {
		t.Errorf("Diagnostics() = %+v on a fresh session, want UnexpectedFrames 0", got)
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
// notice the wiring being removed.
func TestWithTransportLogger_ReachesTheEngine(t *testing.T) {
	p := newRespondingPort(t, readTestImage())
	log := &captureLogger{}

	opened, err := New(Simulated, WithTransportLogger(log)).Open(testCtx(t), p.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	sess := opened.(*Session)

	// Slot 020's MT answer is preceded by a junk frame — see readTestImage.
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
// panic on the first diagnostic.
func TestWithTransportLogger_NilIsIgnored(t *testing.T) {
	d, ok := New(Simulated, WithTransportLogger(nil)).(*ft891Driver)
	if !ok {
		t.Fatal("New did not return a *ft891Driver")
	}
	if d.transportLogger != nil {
		t.Error("WithTransportLogger(nil) installed a logger, want the engine's default kept")
	}
}

// capsContains reports whether ANY bank field of caps carries s, on EITHER
// side — Read or Write.
//
// Both sides on purpose, even though the consent transform is write-only:
// the tests below use it to assert the ABSENCE of spec.ConsentedUnverified,
// and a search that looked only where the transform is meant to write would
// be blind to the one failure that matters most, a consent label leaking
// onto the read side (which spec.Capabilities.Validate refuses outright).
func capsContains(caps spec.Capabilities, s spec.Support) bool {
	for _, b := range caps.Banks {
		for _, fs := range b.Fields {
			if fs.Read == s || fs.Write == s {
				return true
			}
		}
	}
	return false
}

// reportCapsDifference logs the first per-field divergence between two
// capability sets — bank IDs and order first, then each bank's field map.
// Silent when the sets differ only OUTSIDE the bank field maps, which the
// caller must report for itself.
func reportCapsDifference(t *testing.T, got, want spec.Capabilities) {
	t.Helper()
	if len(got.Banks) != len(want.Banks) {
		t.Errorf("bank count = %d, want %d", len(got.Banks), len(want.Banks))
		return
	}
	for i, wb := range want.Banks {
		gb := got.Banks[i]
		if gb.ID != wb.ID {
			t.Errorf("Banks[%d].ID = %s, want %s", i, gb.ID, wb.ID)
			return
		}
		for f, wfs := range wb.Fields {
			if gfs := gb.Fields[f]; gfs != wfs {
				t.Errorf("bank %s field %s: FieldSupport = %+v, want %+v", wb.ID, f, gfs, wfs)
				return
			}
		}
	}
}

// consentTestImage is the scripted radio the consent tests share: one 5xx
// slot and EMG answer, so every session they open carries DISCOVERED
// read-only banks alongside the profile's static MEM and PMS. That is the
// interesting shape for a transform that must leave read-only banks alone —
// a discovered bank's Write is spec.Unsupported (caps.go's readOnlyFields),
// never Unverified, so consent has nothing to convert there and must not
// invent anything.
func consentTestImage() slotImage {
	return slotImage{mrAnswers: map[string]string{
		"503": populatedMR("503"),
		"EMG": populatedMR("EMG"),
	}}
}

// TestConsentOption_SessionCapsTransformed is the option's whole point: a
// RealHardware session built WITH WithConsentedUnverifiedWrites carries
// exactly spec.ConsentUnverifiedWrites' product over the capability set the
// same session would otherwise have had — the ONE transform, applied at the
// ONE assembly point, with no driver-local reinterpretation of what consent
// means.
func TestConsentOption_SessionCapsTransformed(t *testing.T) {
	img := consentTestImage()
	_, plain := openSession(t, RealHardware, img)
	_, consented := openSession(t, RealHardware, img, WithConsentedUnverifiedWrites())

	if capsContains(plain.Capabilities(), spec.ConsentedUnverified) {
		t.Fatal("an UNCONSENTED RealHardware session already carries ConsentedUnverified — the option is not what put it there, so this test can prove nothing")
	}

	want := spec.ConsentUnverifiedWrites(plain.Capabilities())
	got := consented.Capabilities()
	if !reflect.DeepEqual(got, want) {
		reportCapsDifference(t, got, want)
		t.Error("consented session capabilities differ from spec.ConsentUnverifiedWrites' product (see above)")
	}

	// The consequences, stated separately from the equality so a failure
	// says WHICH property broke.
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldFrequency); fs.Write != spec.ConsentedUnverified || !fs.CanWrite() {
		t.Errorf("MEM frequency Write = %v (CanWrite %v), want ConsentedUnverified and writable", fs.Write, fs.CanWrite())
	}
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldTagDisplay); fs.Write != spec.ConsentedUnverified {
		t.Errorf("MEM tag_display Write = %v, want ConsentedUnverified — this radio's P11 is a LIVE flag, so consent reaches it exactly as it reaches the other six (matrix §3.7)", fs.Write)
	}
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldErase); fs.CanWrite() {
		t.Error("MEM erase became writable under consent — FieldErase is exempt from the transform structurally (core/spec/consent.go), and this radio has no erase command at all")
	}
	if fs := got.FieldSupport(spec.Bank60m, spec.FieldFrequency); fs.Write != spec.Unsupported {
		t.Errorf("discovered 60M frequency Write = %v, want Unsupported — consent must not reach a read-only discovered bank", fs.Write)
	}
	for _, b := range got.Banks {
		for f, fs := range b.Fields {
			if fs.Read == spec.ConsentedUnverified {
				t.Errorf("bank %s field %s: Read = ConsentedUnverified — consent is a write-side state and Capabilities.Validate rejects it read-side", b.ID, f)
			}
		}
	}
}

// TestConsentOption_StaticCapabilitiesNeverConsented: the option changes
// what a SESSION carries and nothing else. A driver built with it still
// describes the radio exactly as one built without it does.
//
// That boundary is load-bearing above this package: internal/wiring's
// registry publishes driver.Capabilities() and refuses a registered set
// carrying ConsentedUnverified on either side, and the app's static
// surfaces describe the model rather than one user's decision.
func TestConsentOption_StaticCapabilitiesNeverConsented(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Profile
	}{
		{"Simulated", Simulated},
		{"RealHardware", RealHardware},
		{"unrecognised", Profile(99)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.p, WithConsentedUnverifiedWrites()).Capabilities()
			if capsContains(got, spec.ConsentedUnverified) {
				t.Error("the consent option reached the STATIC capability set — it must apply only at session-capability assembly")
			}
			if !reflect.DeepEqual(got, New(tt.p).Capabilities()) {
				t.Error("a consented driver's static Capabilities() differ from an unconsented one's")
			}
		})
	}
}

// TestConsentOption_DefaultByteIdentical: WITHOUT the option nothing moves
// — a session's assembled set is precisely effectiveCapabilities' own
// product, discovered banks and all, with the expectation built here from
// the pinned profile constructor rather than from anything the driver just
// computed.
func TestConsentOption_DefaultByteIdentical(t *testing.T) {
	_, sess := openSession(t, RealHardware, consentTestImage())
	want := effectiveCapabilities(CapabilitiesUnverified(), []string{"503"}, true)
	if got := sess.Capabilities(); !reflect.DeepEqual(got, want) {
		reportCapsDifference(t, got, want)
		t.Error("an unconsented session's capabilities are no longer effectiveCapabilities' own product (see above)")
	}
}

// TestConsentOption_UnrecognisedProfileStaysFailSafe: the fail-safe
// direction survives consent. A driver built with an unrecognised Profile
// AND the consent option gets NO ConsentedUnverified anywhere, so its
// sessions stay exactly as unwritable as an unconsented one's.
//
// spec.ConsentUnverifiedWrites is profile-agnostic (it transforms whatever
// it is handed), so the only place that can refuse to apply it to a profile
// nobody declared is the driver's own assembly point. The guarantee it
// preserves is Profile's own: no value a caller can pass — forged,
// corrupted, or from a future constant this build has never heard of —
// produces a writable session.
func TestConsentOption_UnrecognisedProfileStaysFailSafe(t *testing.T) {
	_, sess := openSession(t, Profile(99), consentTestImage(), WithConsentedUnverifiedWrites())
	caps := sess.Capabilities()

	if capsContains(caps, spec.ConsentedUnverified) {
		t.Error("an unrecognised Profile + the consent option produced ConsentedUnverified — the profile gate has drifted open")
	}
	for _, b := range caps.Banks {
		for _, f := range allFields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("bank %s field %s: CanWrite() = true on an unrecognised Profile with consent — the fail-safe must survive the option", b.ID, f)
			}
		}
	}
}

// TestProfileRecognised_MatchesTheDeclaredConstants is the consent gate's
// DRIFT GUARD, and the sibling of the tests of the same name in the three
// registered Yaesu drivers: profileRecognised must be true for exactly the
// two Profile constants this package declares and false for everything
// else.
//
// The dangerous direction is the one this test exists for. A profile the
// GATE recognised but ft891Driver.Capabilities' switch did not would take
// the default arm's all-Unverified fail-safe set and then have the consent
// transform applied to it — fail-safe labels turned writable, the precise
// opposite of what the fail-safe is for. (The other direction merely
// withholds consent from a declared profile: unhelpful, not unsafe.)
//
// The two sides are restated in two switches on purpose —
// profileRecognised's and Capabilities' — because Go offers no way to
// derive one from the other for an open integer type. The sweep
// deliberately includes the values NEXT to the declared ones (a constant
// added without a gate arm lands there), a negative, and the extremes.
func TestProfileRecognised_MatchesTheDeclaredConstants(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
	} {
		t.Run("declared/"+tt.name, func(t *testing.T) {
			d, ok := New(tt.p).(*ft891Driver)
			if !ok {
				t.Fatal("New did not return a *ft891Driver")
			}
			if !d.profileRecognised() {
				t.Errorf("profileRecognised() = false for the declared constant %s — a declared profile must be able to receive consent", tt.name)
			}
		})
	}
	for _, p := range []Profile{
		-1, -2, 2, 3, 4, 7, 42, 99, 1000,
		Profile(math.MinInt), Profile(math.MaxInt),
	} {
		t.Run(fmt.Sprintf("other/%d", int(p)), func(t *testing.T) {
			d, ok := New(p).(*ft891Driver)
			if !ok {
				t.Fatal("New did not return a *ft891Driver")
			}
			if d.profileRecognised() {
				t.Errorf("profileRecognised() = true for Profile(%d), which this package does not declare — Capabilities' switch hands that profile the all-Unverified fail-safe set, and the gate would then let consent make it writable", int(p))
			}
		})
	}
}

// TestEffectiveCapabilities_Validate: every capability set a session can
// ever carry passes spec.Capabilities.Validate — profiles x discovered
// inventories x consent, assembled through the one seam that builds them
// (sessionCapabilities).
//
// TestProfiles_Validate covers the static baselines only, and the sets this
// driver actually hands out are strictly larger: they carry the discovered
// read-only banks, and the consent transform's relabelling too. Validate is
// meaningful for a consented set in particular because its read-side rule
// refuses ConsentedUnverified outright, so a transform that leaked onto the
// read side fails HERE rather than at whatever layer first tried to enforce
// it.
func TestEffectiveCapabilities_Validate(t *testing.T) {
	for _, prof := range []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
		{"unrecognised", Profile(99)},
	} {
		for _, disc := range []struct {
			name     string
			slots60m []string
			emg      bool
		}{
			{"no discovery", nil, false},
			{"60m only", []string{"503", "510"}, false},
			{"EMG only", nil, true},
			{"60m and EMG", []string{"501"}, true},
		} {
			for _, consent := range []bool{false, true} {
				name := prof.name + "/" + disc.name
				if consent {
					name += "/consented"
				}
				t.Run(name, func(t *testing.T) {
					var opts []Option
					if consent {
						opts = append(opts, WithConsentedUnverifiedWrites())
					}
					d, ok := New(prof.p, opts...).(*ft891Driver)
					if !ok {
						t.Fatal("New did not return a *ft891Driver")
					}
					if err := d.sessionCapabilities(disc.slots60m, disc.emg).Validate(); err != nil {
						t.Errorf("Validate() = %v, want nil", err)
					}
				})
			}
		}
	}
}

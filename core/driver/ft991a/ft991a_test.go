// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

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
// frame-parsing CAT peer (respondingport_test.go) — through the driver's own
// real transport.Engine, constructed inside Open. There is no fake radio
// here: internal/fakeft991a is lane B's, and the error paths these tests
// need (a foreign CAT ID, an answer naming the wrong slot, silence) are
// answers a self-consistent fake would never give.
//
// EVERY Open IN THIS PACKAGE COSTS TWO EXCHANGES — the AI0 init's error
// window and the ID probe — and nothing else, because this radio discovers
// nothing (matrix §3.4). The FT-891's equivalent comment budgets TWELVE and
// the FTdx10's about a hundred; that this one is short is the whole shape of
// this driver's Open, and TestOpen_SendsExactlyTwoFrames is what keeps it so.
const testCtxTimeout = 60 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// testIdentity is the caller-side Identity every test session opens with.
var testIdentity = driver.Identity{Port: "scripted-pipe", USBSerial: "SIM0670"}

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
		t.Fatalf("Open returned a %T, want *ft991a.Session", sess)
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
			if d.Model() != "FT-991A" {
				t.Errorf("Model() = %q, want \"FT-991A\"", d.Model())
			}
			if d.Model() == "FT-991" {
				t.Error("Model() is \"FT-991\" — that is a DIFFERENT REAL RADIO, not a shortening of this one (matrix §1.1, plan P15)")
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
// the simulator's writable set — and an unrecognised value must fail safe to
// the same all-Unverified set, which is the property that holds whatever a
// forged or corrupted Profile carries. caps_test.go pins the CONTENTS of
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
func TestOpen_ProbesIdentityAndPopulatesIt(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	id := sess.Identity()
	if id.CATID != "0670" {
		t.Errorf("Identity().CATID = %q, want \"0670\" (the probe's answer is authoritative)", id.CATID)
	}
	if id.Port != testIdentity.Port || id.USBSerial != testIdentity.USBSerial {
		t.Errorf("Identity() = %+v, want the caller's Port/USBSerial preserved (%+v)", id, testIdentity)
	}
}

// TestOpen_SendsExactlyTwoFrames is THE Open test on this radio, and it is
// a NEGATIVE pin because the behaviour it protects is an ABSENCE (matrix
// §3.4, plan P10 and task 10).
//
// AI0; then ID; and NOTHING ELSE: there is no discovery phase, no slot walk
// and no probe of any bank, because "5xx", "5 MHz", "5MHz" and "EMG" appear
// in no slot legend of this manual and both banks are dense and complete at
// construction. The FT-891 spends up to eleven MR exchanges here; this radio
// spends none.
//
// THE WHOLE TRANSCRIPT IS ASSERTED, not "no MR frame". A regression that
// added a walk would otherwise be invisible — it would simply work, slowly,
// against any peer that answered — and an absence cannot be caught by
// watching the right thing happen. Any extra frame of any kind fails here.
func TestOpen_SendsExactlyTwoFrames(t *testing.T) {
	for _, profile := range []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
	} {
		t.Run(profile.name, func(t *testing.T) {
			// A richly populated image, so a driver that DID walk would
			// find something to report rather than failing for want of
			// answers: every memory and PMS slot answers, as does a 5xx
			// and an EMG form no bank of this radio contains.
			img := slotImage{mtAnswers: map[string]string{
				"001": populatedMT("001"),
				"117": populatedMT("117"),
				"501": populatedMT("501"),
				"EMG": populatedMT("EMG"),
			}}
			p, sess := openSession(t, profile.p, img)

			if got, want := p.Transcript(), []string{"AI0;", "ID;"}; !reflect.DeepEqual(got, want) {
				t.Errorf("Open's transcript = %v, want exactly %v — this radio has nothing to discover, so Open sends the AI0 preamble and the identity probe and NOTHING more (matrix §3.4)", got, want)
			}
			if n := len(sess.Capabilities().Banks); n != 2 {
				t.Errorf("the session carries %d banks, want 2 — no bank is ever discovered on this radio, whatever a peer answers", n)
			}
		})
	}
}

// TestOpen_WrongRadio: the port answers ID; with "ID0800;" — an FT-710, the
// exact radio most likely to be on the other end of a mistake, since it
// shares this family's connector, baud menu and CAT grammar and would answer
// plenty of FT-991A frames plausibly.
//
// Open must fail with a typed *driver.WrongRadioError and must close the
// port it took ownership of.
//
// THE ERROR'S SHAPE IS PLAN DECISION P10, and both halves are pinned:
// WantModel is "FT-991A" and GotModel is EMPTY, so Error() renders its
// ID-ONLY sentence. driver.WrongRadioError.Error() renders the NAMED form
// only when BOTH names are populated, and cmd/rigprog's probe formatter keys
// on GotModel alone, so a driver filling one alone would render the same
// refusal two different ways. THERE IS NO SIBLING ID TABLE, and in
// particular NO attempt to name "FT-991" on the GOT side: this project has
// never seen that radio's ID answer, and putting a guessed name in a refusal
// about identity would be the one place a guess is least excusable (matrix
// §3.10).
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
	if wre.Want != "0670" || wre.Got != "0800" {
		t.Errorf("WrongRadioError = {Want:%q Got:%q}, want {Want:\"0670\" Got:\"0800\"}", wre.Want, wre.Got)
	}
	if wre.WantModel != "FT-991A" {
		t.Errorf("WrongRadioError.WantModel = %q, want \"FT-991A\" (P10: \"with names\" is satisfied on the WANT side)", wre.WantModel)
	}
	if wre.GotModel != "" {
		t.Errorf("WrongRadioError.GotModel = %q, want \"\" — this driver has no sibling ID table, and populating one name alone makes Error() and cmd/rigprog's probe formatter render the same refusal two different ways (P10)", wre.GotModel)
	}
	const wantText = `driver: connected radio identified as CAT ID "0800", want "0670" — wrong radio model on this port`
	if got := wre.Error(); got != wantText {
		t.Errorf("Error() = %q,\nwant the ID-only sentence %q", got, wantText)
	}

	// TWO FRAMES ON A MISMATCH TOO, which is the other half of §3.4's pin:
	// the refusal costs a wrong radio exactly what a right one costs.
	if got, want := p.Transcript(), []string{"AI0;", "ID;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want exactly %v — a wrong radio receives the AI0 preamble and the ID probe and NOTHING more", got, want)
	}

	// Open took ownership of the port and failed: it must have closed it.
	if _, werr := p.Port().Write([]byte("x")); werr == nil {
		t.Error("the port is still writable after a failed Open — Open must close the port it took ownership of on every error path")
	}
}

// TestOpen_NeverBuildsAnMROfANonExistentBank is the negative pin the spec's
// §Testing and plan task 10 both name: no MR of a 5xx or EMG slot is ever
// built, BECAUSE NO SUCH SLOT EXISTS.
//
// It is asserted over a WHOLE SESSION — Open plus reads of both banks —
// rather than over Open alone, because the frame that would betray a
// borrowed FT-891 shape could come from either. Two properties are checked
// and they fail differently: not one frame begins "MR" (this driver's read
// path is MT-only, matrix §3.5), and no frame anywhere names a slot outside
// the 001-117 span this manual prints.
func TestOpen_NeverBuildsAnMROfANonExistentBank(t *testing.T) {
	img := slotImage{mtAnswers: map[string]string{
		"001": populatedMT("001"),
		"100": populatedMT("100"),
		// A peer that WOULD answer the frames a discovery walk sends, so
		// the absence below is the driver's choice and not the fixture's.
		"501": populatedMT("501"),
		"EMG": populatedMT("EMG"),
	}}
	p, sess := openSession(t, Simulated, img)

	for _, slot := range []string{"001", "100"} {
		if _, err := sess.ReadChannel(testCtx(t), slot); err != nil {
			t.Fatalf("ReadChannel(%q) = %v, want nil", slot, err)
		}
	}

	for _, frame := range p.Transcript() {
		if strings.HasPrefix(frame, "MR") {
			t.Errorf("the session sent %q — this driver's read path is MT-only and MR is never sent (matrix §3.5)", frame)
		}
		if len(frame) >= 5 && (strings.Contains(frame, "501") || strings.Contains(frame, "EMG")) {
			t.Errorf("the session sent %q — this manual prints no 5 MHz bank and no emergency channel at all (matrix §1.4.3), so no frame may name one", frame)
		}
	}
}

// TestSession_CapabilitiesIsADefensiveCopy: Capabilities hands out copies,
// and a caller mutating one must never alter what the write gate enforces.
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
	first.CTCSSStates[0] = spec.ToneState{}

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
	if second.CTCSSStates[0].Value != "OFF" {
		t.Errorf("CTCSSStates[0] = %+v after tampering, want the OFF state — this radio's five-member vocabulary must be copied like every other slice", second.CTCSSStates[0])
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
// --model FT-991A` the same wire-health line an FT-710 probe prints.
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
// silently discarded", and the engine's default logger drops everything — so
// without this option a driver gives its caller no way to receive that
// signal at all, and a test that only checked the field was set would not
// notice the wiring being removed.
func TestWithTransportLogger_ReachesTheEngine(t *testing.T) {
	img := slotImage{
		mtAnswers:  map[string]string{"020": populatedMT("020")},
		junkBefore: map[string]string{"020": "ZZ0;"},
	}
	p := newRespondingPort(t, img)
	log := &captureLogger{}

	opened, err := New(Simulated, WithTransportLogger(log)).Open(testCtx(t), p.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	sess := opened.(*Session)

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
	d, ok := New(Simulated, WithTransportLogger(nil)).(*ft991aDriver)
	if !ok {
		t.Fatal("New did not return a *ft991aDriver")
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

// TestConsentOption_SessionCapsTransformed is the option's whole point: a
// RealHardware session built WITH WithConsentedUnverifiedWrites carries
// exactly spec.ConsentUnverifiedWrites' product over the capability set the
// same session would otherwise have had — the ONE transform, applied at the
// ONE assembly point, with no driver-local reinterpretation of what consent
// means.
func TestConsentOption_SessionCapsTransformed(t *testing.T) {
	_, plain := openSession(t, RealHardware, slotImage{})
	_, consented := openSession(t, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())

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
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldCTCSSState); fs.Write != spec.ConsentedUnverified {
		t.Errorf("MEM ctcss_state Write = %v, want ConsentedUnverified — consent reaches the five-state P8 vocabulary exactly as it reaches the other five graded fields, which is what puts a '3' or '4' within reach of the wire (matrix §2.4's [STUART] cell, ruled writable)", fs.Write)
	}
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldTagDisplay); fs != (spec.FieldSupport{}) {
		t.Errorf("MEM tag_display = %+v under consent, want the zero FieldSupport — consent relabels Unverified, and this field is UNSUPPORTED rather than unverified because the frame has no such byte (matrix §2.3)", fs)
	}
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldErase); fs.CanWrite() {
		t.Error("MEM erase became writable under consent — FieldErase is exempt from the transform structurally (core/spec/consent.go), and this radio has no erase command at all")
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
// carrying ConsentedUnverified on either side, and the app's static surfaces
// describe the model rather than one user's decision.
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

// TestConsentOption_DefaultByteIdentical: WITHOUT the option nothing moves —
// a session's assembled set is precisely the pinned profile constructor's
// own product, with the expectation built here rather than from anything the
// driver just computed.
//
// On this radio the claim is total rather than approximate, and that is
// §3.4's consequence: there are no discovered banks for an assembled set to
// carry, so a session's capabilities and the static profile's are the SAME
// VALUE. The FT-891's namesake has to compare against effectiveCapabilities'
// product instead.
func TestConsentOption_DefaultByteIdentical(t *testing.T) {
	_, sess := openSession(t, RealHardware, slotImage{})
	want := CapabilitiesUnverified()
	if got := sess.Capabilities(); !reflect.DeepEqual(got, want) {
		reportCapsDifference(t, got, want)
		t.Error("an unconsented session's capabilities are no longer the profile baseline itself (see above)")
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
// preserves is Profile's own: no value a caller can pass — forged, corrupted,
// or from a future constant this build has never heard of — produces a
// writable session.
func TestConsentOption_UnrecognisedProfileStaysFailSafe(t *testing.T) {
	_, sess := openSession(t, Profile(99), slotImage{}, WithConsentedUnverifiedWrites())
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
// DRIFT GUARD, and the sibling of the tests of the same name in the four
// registered Yaesu drivers: profileRecognised must be true for exactly the
// two Profile constants this package declares and false for everything else.
//
// The dangerous direction is the one this test exists for. A profile the
// GATE recognised but ft991aDriver.Capabilities' switch did not would take
// the default arm's all-Unverified fail-safe set and then have the consent
// transform applied to it — fail-safe labels turned writable, the precise
// opposite of what the fail-safe is for. (The other direction merely
// withholds consent from a declared profile: unhelpful, not unsafe.)
//
// The two sides are restated in two switches on purpose — profileRecognised's
// and Capabilities' — because Go offers no way to derive one from the other
// for an open integer type. The sweep deliberately includes the values NEXT
// to the declared ones (a constant added without a gate arm lands there), a
// negative, and the extremes.
func TestProfileRecognised_MatchesTheDeclaredConstants(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
	} {
		t.Run("declared/"+tt.name, func(t *testing.T) {
			d, ok := New(tt.p).(*ft991aDriver)
			if !ok {
				t.Fatal("New did not return a *ft991aDriver")
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
			d, ok := New(p).(*ft991aDriver)
			if !ok {
				t.Fatal("New did not return a *ft991aDriver")
			}
			if d.profileRecognised() {
				t.Errorf("profileRecognised() = true for Profile(%d), which this package does not declare — Capabilities' switch hands that profile the all-Unverified fail-safe set, and the gate would then let consent make it writable", int(p))
			}
		})
	}
}

// TestSessionCapabilities_Validate: every capability set a session can ever
// carry passes spec.Capabilities.Validate — profiles x consent, assembled
// through the one seam that builds them (sessionCapabilities).
//
// TestProfiles_Validate covers the static baselines only, and a consented
// set is strictly different: Validate's read-side rule refuses
// ConsentedUnverified outright, so a transform that leaked onto the read
// side fails HERE rather than at whatever layer first tried to enforce it.
//
// There is no discovered-inventory axis, unlike the FT-891's namesake, and
// that is §3.4 again rather than a gap in the sweep.
func TestSessionCapabilities_Validate(t *testing.T) {
	for _, prof := range []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
		{"unrecognised", Profile(99)},
	} {
		for _, consent := range []bool{false, true} {
			name := prof.name
			if consent {
				name += "/consented"
			}
			t.Run(name, func(t *testing.T) {
				var opts []Option
				if consent {
					opts = append(opts, WithConsentedUnverifiedWrites())
				}
				d, ok := New(prof.p, opts...).(*ft991aDriver)
				if !ok {
					t.Fatal("New did not return a *ft991aDriver")
				}
				if err := d.sessionCapabilities().Validate(); err != nil {
					t.Errorf("Validate() = %v, want nil", err)
				}
			})
		}
	}
}

// TestDriverDeclaresNoOptionalCapabilitiesItMustNot is the ABSENCE pin for
// two optional interfaces, both of which are decisions rather than
// omissions.
//
//   - driver.SerialFramingReporter: NOT implemented, so the port opens at
//     core/transport's DefaultStopBits — 8-N-2 reached BY ABSENCE rather
//     than by a claim (matrix §3.1, the dialect register's FRAMING entry).
//     internal/wiring's own fleet test asserts the same thing from the other
//     side, for every Yaesu model.
//   - driver.DiscoveredBankSynthesizer: NOT implemented, because there is
//     nothing to discover (matrix §1.4.3, §3.4). On the FT-891 its ABSENCE
//     would be a silent bug — the GUI would render no discovered banks for
//     an offline codeplug — and here its PRESENCE would be: it would
//     classify slots into banks this radio does not have.
func TestDriverDeclaresNoOptionalCapabilitiesItMustNot(t *testing.T) {
	d := New(RealHardware)
	if _, ok := d.(driver.SerialFramingReporter); ok {
		t.Error("the FT-991A driver implements driver.SerialFramingReporter — 8-N-2 must stay this radio's port configuration by ABSENCE (matrix §3.1); a driver that states a framing is making a claim this manual does not support")
	}
	if _, ok := d.(driver.DiscoveredBankSynthesizer); ok {
		t.Error("the FT-991A driver implements driver.DiscoveredBankSynthesizer — this radio has no 5 MHz and no EMG bank at all (matrix §1.4.3), so there is no offline classification to perform and any bank it produced would be one the radio does not have")
	}
}

// TestSession_DoesNotReportRegion: driver.RegionReporter is NOT implemented,
// and doc.go says why — this manual prints no region-conditional memory bank
// at all, where the FT-891's 5 MHz legend carries "U.S. and U.K. version
// only". There is no region for a session to report, and implementing the
// interface would mean answering a question this radio has not been asked.
func TestSession_DoesNotReportRegion(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})
	if _, ok := any(sess).(driver.RegionReporter); ok {
		t.Error("the FT-991A Session implements driver.RegionReporter — nothing in Open asks the radio anything that could answer it")
	}
}

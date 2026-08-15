// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

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
// radio here: internal/fakedx10 does not exist until task 4, and the error
// paths these tests need (a foreign CAT ID, an answer naming the wrong
// slot, an out-of-vocabulary kind byte) are answers a self-consistent fake
// would never give.
//
// EVERY Open in this package costs about 2-3 s of wall clock: the AI0
// init's error window and drain, then ~100 discovery probes at the
// engine's default 20 ms per-exchange settle. That is the accepted,
// budgeted price of walking the whole declared 5xx range (doc.go) and it
// is why these tests share sessions where they can — NOT something to be
// optimised away with a settle override.
const testCtxTimeout = 60 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// testIdentity is the caller-side Identity every test session opens with.
var testIdentity = driver.Identity{Port: "scripted-pipe", USBSerial: "SIM0761"}

// openSession starts a respondingPort serving img, opens a session over it
// with the given profile, and registers cleanup for both. It returns the
// port (for its transcript) and the concrete *Session.
//
// opts are the DRIVER-construction Options New is given — the consent
// option's tests need a session built with one, and the variadic slot is
// free here because this helper's scripted peer is described by img rather
// than by options of its own. (core/driver/ft710's namesake helper has its
// variadic slot occupied by fakeradio options and cannot be extended this
// way.)
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
		t.Fatalf("Open returned a %T, want *ftdx10.Session", sess)
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
			if d.Model() != "FTdx10" {
				t.Errorf("Model() = %q, want \"FTdx10\"", d.Model())
			}
			if got := d.Capabilities().Model; got != d.Model() {
				t.Errorf("Capabilities().Model = %q, Model() = %q — driver.Driver requires them equal", got, d.Model())
			}
		})
	}
}

// TestOpen_ProbesIdentityAndPopulatesIt: a successful Open probes ID; and
// puts the ANSWER on the session's Identity, keeping the caller's port and
// USB serial.
func TestOpen_ProbesIdentityAndPopulatesIt(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	id := sess.Identity()
	if id.CATID != "0761" {
		t.Errorf("Identity().CATID = %q, want \"0761\" (the probe's answer is authoritative)", id.CATID)
	}
	if id.Port != testIdentity.Port || id.USBSerial != testIdentity.USBSerial {
		t.Errorf("Identity() = %+v, want the caller's Port/USBSerial preserved (%+v)", id, testIdentity)
	}

	transcript := p.Transcript()
	if len(transcript) < 2 || transcript[0] != "AI0;" || transcript[1] != "ID;" {
		t.Errorf("Open's first two frames = %v, want [\"AI0;\" \"ID;\"] — the session init then the identity probe", transcript[:min(2, len(transcript))])
	}
}

// TestOpen_WrongRadio: the port answers ID; with "ID0800;" — an FT-710,
// the exact radio most likely to be on the other end of a mistake, since
// it shares this family's connector, baud menu and CAT grammar and would
// answer plenty of FTdx10 frames plausibly.
//
// Open must fail with a typed *driver.WrongRadioError carrying BOTH IDs,
// must never reach discovery, and must close the port it took ownership
// of.
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
	if wre.Want != "0761" || wre.Got != "0800" {
		t.Errorf("WrongRadioError = {Want:%q Got:%q}, want {Want:\"0761\" Got:\"0800\"}", wre.Want, wre.Got)
	}

	// The wrong radio is rejected BEFORE discovery: an FT-710 must not be
	// walked through a hundred FTdx10 probes.
	for _, frame := range p.Transcript() {
		if len(frame) == mtReadFrameLen && frame[:2] == "MT" {
			t.Errorf("Open sent %q after the ID probe failed — discovery must not run against a radio that did not identify", frame)
			break
		}
	}

	// Open took ownership of the port and failed: it must have closed it.
	if _, werr := p.Port().Write([]byte("x")); werr == nil {
		t.Error("the port is still writable after a failed Open — Open must close the port it took ownership of on every error path")
	}
}

// TestOpen_DiscoveryProbesEveryDeclaredSlot is THE discovery test, and it
// asserts the full ordered transcript rather than only the outcome.
//
// The image is deliberately SPARSE, with a gap before every populated
// slot: 503 (after empty 501-502), 599 (the very last declared 5xx slot,
// after 95 empty ones) and EMG. Every termination rule this driver refuses
// would produce a DIFFERENT, plausible-looking result against it —
// stop-at-first-rejection finds nothing at all; contiguity-from-501 finds
// nothing; a 15-slot cap finds 503 and misses 599; a sentinel scheme finds
// 503 and reports an anomaly — so the discovered set alone would catch
// most regressions. The TRANSCRIPT is what catches the rest, and catches
// them by name: it pins that all 99 declared 5xx slots were asked, in
// ascending order, with EMG last and nothing else in between.
//
// If this test ever becomes slow enough to be tempting, read doc.go's
// discovery section before touching it. The ~100 probes ARE the design.
func TestOpen_DiscoveryProbesEveryDeclaredSlot(t *testing.T) {
	img := slotImage{mtAnswers: map[string]string{
		"503": populatedAnswer("503"),
		"599": populatedAnswer("599"),
		"EMG": populatedAnswer("EMG"),
	}}
	p, sess := openSession(t, Simulated, img)

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
		// unreadable, and the first difference is always the diagnosis.
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
	if !sixty.NoBlank {
		t.Error("60M.NoBlank = false, want true (these channels exist because they answered, and this radio's CAT surface cannot blank them)")
	}

	emg, ok := caps.Bank(spec.BankEMG)
	if !ok {
		t.Fatal("session capabilities have no EMG bank, want one discovered from the EMG probe")
	}
	if wantSlots := []string{"EMG"}; !reflect.DeepEqual(emg.Slots, wantSlots) {
		t.Errorf("EMG.Slots = %v, want %v", emg.Slots, wantSlots)
	}

	// --- the discovered banks are READ-ONLY on every field ---
	// No profile may claim a 5xx/EMG slot writable: this session is
	// Simulated, whose static banks ARE write-Supported, so a discovered
	// bank inheriting those writes is a real hazard and not a theoretical
	// one.
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
	// The offline synthesiser, handed exactly the wires discovery found,
	// must produce exactly the banks this live session carries. The
	// table-driven version of this equivalence (over sparse order, 599,
	// EMG, malformed and statically-claimed inputs) lives in
	// optional_test.go; this is the one leg that uses a REAL session's
	// output as the input, so the two derivations are compared on data
	// neither test invented.
	synth := New(Simulated).(driver.DiscoveredBankSynthesizer)
	discoveredWires := append(append([]string(nil), sixty.Slots...), emg.Slots...)
	base := New(Simulated).Capabilities()
	if got, want := synth.SynthesiseDiscoveredBanks(discoveredWires), caps.Banks[len(base.Banks):]; !reflect.DeepEqual(got, want) {
		t.Errorf("SynthesiseDiscoveredBanks(%v) = %#v,\nwant the live session's own discovered banks %#v", discoveredWires, got, want)
	}
}

// TestOpen_NoDiscoveredBanksWhenNothingAnswers: a radio with no 5xx bank
// and no EMG channel — all ~100 probes rejected — is an ordinary outcome,
// not an error, and yields a session with the static banks only.
//
// This is a real variant, not a contrived one: the FT-710 this project
// characterised has no 5xx bank at all (front-panel confirmed), so the
// empty inventory is what a UK-market radio of this family may well
// answer.
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
	if got, want := len(p.Transcript()), 2+99+1; got != want {
		t.Errorf("Open sent %d frames, want %d (AI0, ID, 99 x 5xx, EMG) — an empty inventory is discovered by probing everything, not by stopping early", got, want)
	}
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
// engine's own unexpected-frame counter, which is otherwise unreachable.
// A fresh session has seen none.
//
// cmd/rigprog's probe command prints this for any session satisfying
// driver.DiagnosticsReporter, so implementing it is what gives `probe
// --model FTdx10` the same wire-health line an FT-710 probe prints.
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
// notice the wiring being removed. A nonzero unexpected-frame count on an
// otherwise-working session is real diagnostic information (another
// application sharing the port; replies arriving late enough to miss their
// windows), and cmd/rigprog's probe prints it.
func TestWithTransportLogger_ReachesTheEngine(t *testing.T) {
	p := newRespondingPort(t, readTestImage())
	log := &captureLogger{}

	opened, err := New(Simulated, WithTransportLogger(log)).Open(testCtx(t), p.Port(), testIdentity)
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

// TestWithTransportLogger_NilIsIgnored: a nil logger leaves the engine's
// own drop-everything default in place rather than installing a nil that
// would panic on the first diagnostic.
func TestWithTransportLogger_NilIsIgnored(t *testing.T) {
	d, ok := New(Simulated, WithTransportLogger(nil)).(*ftdx10Driver)
	if !ok {
		t.Fatal("New did not return a *ftdx10Driver")
	}
	if d.transportLogger != nil {
		t.Error("WithTransportLogger(nil) installed a logger, want the engine's default kept")
	}
}

// capsContains reports whether ANY bank field of caps carries s, on
// EITHER side — Read or Write.
//
// Both sides on purpose, even though the consent transform is write-only:
// the tests below use it to assert the ABSENCE of
// spec.ConsentedUnverified, and a search that looked only where the
// transform is meant to write would be blind to the one failure that
// matters most, a consent label leaking onto the read side (which
// spec.Capabilities.Validate refuses outright — see
// TestEffectiveCapabilities_Validate).
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
// interesting shape for a transform that must leave read-only banks alone
// — a discovered bank's Write is spec.Unsupported (caps.go's
// readOnlyFields), never Unverified, so consent has nothing to convert
// there and must not invent anything.
func consentTestImage() slotImage {
	return slotImage{mtAnswers: map[string]string{
		"503": populatedAnswer("503"),
		"EMG": populatedAnswer("EMG"),
	}}
}

// TestConsentOption_SessionCapsTransformed is the option's whole point: a
// RealHardware session built WITH WithConsentedUnverifiedWrites carries
// exactly spec.ConsentUnverifiedWrites' product over the capability set
// the same session would otherwise have had — the ONE transform, applied
// at the ONE assembly point, with no driver-local reinterpretation of what
// consent means.
//
// Deep equality against `spec.ConsentUnverifiedWrites(plain.Capabilities())`
// is what makes that a proof rather than a spot check: any label this
// driver converted differently, any bank it treated specially, and any
// non-bank datum it disturbed would show up as an inequality.
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
		// Report the divergence precisely rather than dumping two whole
		// capability sets: a full diff of this structure is unreadable,
		// and the first differing field is always the diagnosis. A
		// difference OUTSIDE the bank field maps falls through to the
		// generic message, which is the honest answer for it.
		reportCapsDifference(t, got, want)
		t.Error("consented session capabilities differ from spec.ConsentUnverifiedWrites' product (see above)")
	}

	// The consequences, stated separately from the equality so a failure
	// says WHICH property broke.
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldFrequency); fs.Write != spec.ConsentedUnverified || !fs.CanWrite() {
		t.Errorf("MEM frequency Write = %v (CanWrite %v), want ConsentedUnverified and writable", fs.Write, fs.CanWrite())
	}
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldErase); fs.CanWrite() {
		t.Error("MEM erase became writable under consent — FieldErase is exempt from the transform structurally (core/spec/consent.go), and a consented erase would unblock codeplug.Diff's erase gate")
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

// TestConsentOption_DefaultByteIdentical: WITHOUT the option nothing moves
// — neither the static baseline nor a session's assembled set.
//
// The session half is the one worth having. Introducing
// sessionCapabilities put a new step between effectiveCapabilities and the
// Session, and the default path must still produce precisely
// effectiveCapabilities' own product, discovered banks and all; the
// expectation is built here from the pinned profile constructor rather
// than from anything the driver just computed.
func TestConsentOption_DefaultByteIdentical(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Profile
		want spec.Capabilities
	}{
		{"RealHardware", RealHardware, CapabilitiesUnverified()},
		{"Simulated", Simulated, CapabilitiesSimulated()},
		{"unrecognised", Profile(99), CapabilitiesUnverified()},
	} {
		t.Run("static/"+tt.name, func(t *testing.T) {
			if got := New(tt.p).Capabilities(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("New(%v).Capabilities() is no longer the pinned profile constructor's product", tt.name)
			}
		})
	}

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
// This is the plan's spec amendment 4, and it is a driver-side gate by
// design: spec.ConsentUnverifiedWrites is profile-agnostic (it transforms
// whatever it is handed), so the only place that can refuse to apply it to
// a profile nobody declared is the driver's own assembly point. The
// guarantee it preserves is Profile's own: no value a caller can pass —
// forged, corrupted, or from a future constant this build has never heard
// of — produces a writable session.
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
// DRIFT GUARD, and the sibling of the tests of the same name in
// core/driver/ft710 and core/driver/ftdx101: profileRecognised must be true
// for exactly the two Profile constants this package declares (caps.go —
// RealHardware, Simulated) and false for everything else.
//
// The dangerous direction is the one this test exists for. A profile the
// GATE recognised but ftdx10Driver.Capabilities' switch did not would take
// the default arm's all-Unverified fail-safe set and then have the consent
// transform applied to it — fail-safe labels turned writable, which is the
// precise opposite of what the fail-safe is for, and on this radio the
// fail-safe set is the one with the most write-side Unverified labels to
// turn. (The other direction merely withholds consent from a declared
// profile: unhelpful, not unsafe.)
//
// TestConsentOption_UnrecognisedProfileStaysFailSafe above pins the same
// property through a whole opened session, for one unrecognised value; this
// pins the gate itself across a sweep, and costs no Open. The two sides are
// restated in two switches on purpose — profileRecognised's and
// Capabilities' — because Go offers no way to derive one from the other for
// an open integer type, so a test is what holds them together. The sweep
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
			d, ok := New(tt.p).(*ftdx10Driver)
			if !ok {
				t.Fatal("New did not return a *ftdx10Driver")
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
			d, ok := New(p).(*ftdx10Driver)
			if !ok {
				t.Fatal("New did not return a *ftdx10Driver")
			}
			if d.profileRecognised() {
				t.Errorf("profileRecognised() = true for Profile(%d), which this package does not declare — Capabilities' switch hands that profile the all-Unverified fail-safe set, and the gate would then let consent make it writable", int(p))
			}
		})
	}
}

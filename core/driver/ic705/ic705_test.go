// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic705 "github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// addrOf resolves a display slot to its wire address, failing the test on
// a slot string this driver does not recognise.
func addrOf(t *testing.T, slot string) civ.ChannelAddress {
	t.Helper()
	a, _, err := slotToAddress(slot)
	if err != nil {
		t.Fatalf("slotToAddress(%q): %v", slot, err)
	}
	return a
}

// encodedRecord renders one civ.MemoryRecord as the 111 raw record bytes a
// radio would serve for it: build the set frame, then strip the six
// framing/command bytes, the four address bytes and the trailing FD.
func encodedRecord(t *testing.T, rec civ.MemoryRecord) []byte {
	t.Helper()
	cmd, err := civic705.Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	f := cmd.Bytes()
	return append([]byte(nil), f[10:len(f)-1]...)
}

// openSession opens a session against r and registers its Close.
//
// EVERY TEST GETS THE NO-PACING CLOCK. See noSettleClock: it removes the
// engine's 20 ms inter-exchange sleep and nothing else, because an
// inventory walk is a thousand exchanges and the pacing is a fact about a
// serial link rather than about this driver.
func openSession(t *testing.T, r *scriptedRadio, opts ...Option) *Session {
	t.Helper()
	sess, err := openSessionErr(t, r, opts...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return sess
}

func openSessionErr(t *testing.T, r *scriptedRadio, opts ...Option) (*Session, error) {
	t.Helper()
	opts = append(opts, withEngineOptions(transport.WithClock(noSettleClock{})))
	d := New(RealHardware, opts...)
	sess, err := d.Open(context.Background(), r.Port(), driver.Identity{Port: "/dev/scripted", USBSerial: "SN-1"})
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), nil
}

// occupiedRadio is a scripted radio holding one ordinary record at slot.
func occupiedRadio(t *testing.T, slot string) *scriptedRadio {
	t.Helper()
	a := addrOf(t, slot)
	return newScriptedRadio(t, radioImage{
		records: map[civ.ChannelAddress][]byte{a: encodedRecord(t, fullRecord(a))},
	})
}

func TestModelIsTheCapabilitiesModel(t *testing.T) {
	d := New(RealHardware)
	if d.Model() != d.Capabilities().Model {
		t.Errorf("Model() = %q but Capabilities().Model = %q — the registry key and the capability data must be one string", d.Model(), d.Capabilities().Model)
	}
	if d.Model() != "IC-705" {
		t.Errorf("Model() = %q, want \"IC-705\"", d.Model())
	}
}

func TestOpenRequiresAnAddressMatchedIDReply(t *testing.T) {
	r := newScriptedRadio(t, radioImage{silentID: true})
	sess, err := openSessionErr(t, r)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open succeeded against a radio that never answered 19 00 — the probe requires an ADDRESS-MATCHED reply, and silence is not one")
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("Open failed with %v, want a timeout on the ID probe", err)
	}
	if r.Reads() != 0 {
		t.Errorf("the driver read %d memories after the ID probe failed — nothing may follow an unanswered probe", r.Reads())
	}
}

func TestOpenRecordsTheIDTokenButNeverMatchesIt(t *testing.T) {
	// The `19 00` reply VALUE is undocumented on all six models in this
	// tier (D5 entry 7, lift L-IDTOKEN), so two radios answering two
	// different tokens must BOTH open, and both tokens must be recorded.
	for _, payload := range [][]byte{{0x94}, {0xA4, 0x01}} {
		r := newScriptedRadio(t, radioImage{idPayload: payload})
		sess := openSession(t, r)
		got := sess.SessionInfo().IDToken
		if got == "" {
			t.Errorf("payload % X: no ID token recorded", payload)
		}
		wantCATID := "A4:" + got
		if sess.Identity().CATID != wantCATID {
			t.Errorf("payload % X: Identity().CATID = %q, want %q — the address, then the observed token (plan O-8)", payload, sess.Identity().CATID, wantCATID)
		}
	}

	// A radio that answers the ENVELOPE with no data has still proved its
	// address, which is the whole of what this step asks. The identity is
	// then the address alone.
	r := newScriptedRadio(t, radioImage{idPayload: []byte{}})
	sess := openSession(t, r)
	if sess.SessionInfo().IDToken != "" {
		t.Errorf("IDToken = %q for a reply with no data area", sess.SessionInfo().IDToken)
	}
	if sess.Identity().CATID != "A4" {
		t.Errorf("Identity().CATID = %q, want \"A4\" when no token was observed (plan O-8)", sess.Identity().CATID)
	}
}

func TestOpenKeepsTheCallersPortAndSerial(t *testing.T) {
	sess := openSession(t, occupiedRadio(t, "G01-001"))
	if sess.Identity().Port != "/dev/scripted" || sess.Identity().USBSerial != "SN-1" {
		t.Errorf("Identity() = %+v — Port and USBSerial are the CALLER's and the probe must not overwrite them", sess.Identity())
	}
}

func TestProbeFingerprintsOnTheFirstOccupiedSlot(t *testing.T) {
	sess := openSession(t, occupiedRadio(t, "G01-001"))
	if !sess.SessionInfo().Fingerprinted {
		t.Error("a session opened against a radio whose first memory holds a 111-byte record is not fingerprinted")
	}
}

func TestProbeFingerprintsOnALaterOccupiedSlotToo(t *testing.T) {
	// The probe does not stop at the first EMPTY slot: it reads up to
	// sixteen, so a radio whose only memory in group 1 sits at channel 16
	// is still fingerprinted.
	sess := openSession(t, occupiedRadio(t, "G01-016"))
	if !sess.SessionInfo().Fingerprinted {
		t.Error("the probe missed an occupied slot inside its own bounded range")
	}
}

func TestProbeRefusesAForeignRecordLength(t *testing.T) {
	// A plausible foreign length, named here as a FIXTURE and explicitly
	// NOT as a claim about any other model: cross-model record-length
	// distinctness is a Wave-4 tier check this driver must not make.
	a := addrOf(t, "G01-001")
	r := newScriptedRadio(t, radioImage{
		records: map[civ.ChannelAddress][]byte{a: make([]byte, 39)},
	})
	sess, err := openSessionErr(t, r)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open accepted a radio whose memory record is 39 bytes")
	}
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open failed with %v, want a wrong-radio refusal", err)
	}
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("error %v is not a *driver.WrongRadioError", err)
	}
	if wrong.WantModel != "IC-705" {
		t.Errorf("WantModel = %q, want \"IC-705\"", wrong.WantModel)
	}
	if wrong.GotModel != "" {
		t.Errorf("GotModel = %q, want EMPTY — naming the model a foreign length belongs to needs the other models' length sets, which is Wave 4's check and not this driver's claim", wrong.GotModel)
	}
	if wrong.Want != "A4/111" {
		t.Errorf("Want = %q, want \"A4/111\" (the address and the record-only length)", wrong.Want)
	}
	if wrong.Got != "A4/39" {
		t.Errorf("Got = %q, want \"A4/39\"", wrong.Got)
	}
}

func TestEmptyRadioOpensUnfingerprinted(t *testing.T) {
	// Every probe read answered FA: the session opens on ADDRESS EVIDENCE
	// ALONE, with the unfingerprinted state recorded (D5 entry 2(a), lift
	// L-EMPTY-FA). Refusing here would make a factory-fresh radio
	// unreadable.
	r := newScriptedRadio(t, radioImage{})
	sess := openSession(t, r)
	if sess.SessionInfo().Fingerprinted {
		t.Error("a radio with no occupied memory reported itself fingerprinted")
	}
	if r.Reads() < probeSlots {
		t.Errorf("the probe read %d memories, want at least %d — it must exhaust its bound before concluding the radio is empty", r.Reads(), probeSlots)
	}
}

func TestAnAllFFRecordIsNotAFingerprint(t *testing.T) {
	// The second unverified empty shape (D5 entry 2(b), lift L-EMPTY-FF).
	// It is recognised on the RAW bytes, because 0xFF fails the enum
	// decode and testing for it after parsing would be too late.
	a := addrOf(t, "G01-001")
	ff := make([]byte, 111)
	for i := range ff {
		ff[i] = 0xFF
	}
	r := newScriptedRadio(t, radioImage{records: map[civ.ChannelAddress][]byte{a: ff}})
	sess := openSession(t, r)
	if sess.SessionInfo().Fingerprinted {
		t.Error("an all-FF record was taken for an occupied channel")
	}
}

func TestProbeIsBounded(t *testing.T) {
	// SIXTEEN FRAMES, NEVER TEN THOUSAND. Asserted against
	// fingerprintProbe itself rather than through Open, because Open also
	// performs the inventory walk (inventory.go) and a count taken at that
	// level would measure the two together and stop meaning anything about
	// the probe's own bound.
	r := newScriptedRadio(t, radioImage{})
	eng, _, err := newEngine(r.Port(), transport.WithClock(noSettleClock{}))
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}
	defer eng.Close()
	fingerprinted, err := fingerprintProbe(context.Background(), eng)
	if err != nil {
		t.Fatalf("fingerprintProbe: %v", err)
	}
	if fingerprinted {
		t.Error("an empty radio fingerprinted")
	}
	if got := r.Reads(); got != probeSlots {
		t.Errorf("the probe read %d memories, want exactly %d", got, probeSlots)
	}
	// And every one of them inside group 1's display space.
	for _, f := range r.Transcript() {
		if len(f) < 11 || f[4] != 0x1A {
			continue
		}
		a, err := decodeWireAddress(f[6:10])
		if err != nil {
			t.Fatalf("probe frame % X carries a malformed address", f)
		}
		if a.Group != 0 || a.Channel < 0 || a.Channel > probeSlots-1 {
			t.Errorf("the probe read %v — outside display group G01's first %d channels", a, probeSlots)
		}
	}
}

func TestInitSendsNothingToTheRadio(t *testing.T) {
	// THE TEST THAT WOULD CATCH A TRANSCEIVE-OFF WRITE CREEPING IN. No
	// radio mutation at Init, ever: no transceive-off write, no clear, no
	// 1A 05. The framing's InitSequence is EMPTY and broadcasts are
	// handled by address filtering, so the FIRST frame this radio ever
	// receives is the identity read.
	r := occupiedRadio(t, "G01-001")
	_ = openSession(t, r)
	frames := r.Transcript()
	if len(frames) == 0 {
		t.Fatal("the radio received nothing at all")
	}
	first := frames[0]
	if len(first) < 6 || first[4] != 0x19 || first[5] != 0x00 {
		t.Errorf("the first frame the radio saw was % X, want the 19 00 identity read — Init must write nothing", first)
	}
	for _, f := range frames {
		if len(f) >= 6 && f[4] == 0x1A && f[5] == 0x05 {
			t.Errorf("the driver sent a 1A 05 menu frame (% X) — this tier has no menu surface at all", f)
		}
		if r.Sets() != 0 {
			t.Errorf("the driver sent a memory SET during Open (% X)", f)
		}
	}
}

func TestCapabilityProfilesMatchTheConstructors(t *testing.T) {
	if got, want := New(RealHardware).Capabilities(), capabilitiesUnverified(); !reflect.DeepEqual(got, want) {
		t.Error("New(RealHardware).Capabilities() is not capabilitiesUnverified()")
	}
	if got, want := New(Simulated).Capabilities(), capabilitiesSimulated(); !reflect.DeepEqual(got, want) {
		t.Error("New(Simulated).Capabilities() is not capabilitiesSimulated()")
	}
	// Any unrecognised Profile fails safe to the unverified set.
	if got, want := New(Profile(42)).Capabilities(), capabilitiesUnverified(); !reflect.DeepEqual(got, want) {
		t.Error("an unrecognised Profile did not fail safe to the unverified capability set")
	}
}

func TestConsentIsAppliedOnlyToSessionCapabilities(t *testing.T) {
	r := occupiedRadio(t, "G01-001")
	d := New(RealHardware, WithConsentedUnverifiedWrites(), withEngineOptions(transport.WithClock(noSettleClock{})))
	// The STATIC set is never transformed — internal/wiring reads exactly
	// this value to decide whether consent is needed at all.
	if got := d.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency); got.CanWrite() {
		t.Errorf("the static Capabilities() carries a consented write label (%+v)", got)
	}
	sess, err := d.Open(context.Background(), r.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()
	if got := sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency); !got.CanWrite() {
		t.Errorf("the SESSION's frequency support is %+v — consent must open the write gate for this session", got)
	}
	if got := sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldErase); got.CanWrite() {
		t.Error("consent minted a writable erase")
	}
}

func TestConsentDoesNotReachAnUnrecognisedProfile(t *testing.T) {
	r := occupiedRadio(t, "G01-001")
	d := New(Profile(42), WithConsentedUnverifiedWrites(), withEngineOptions(transport.WithClock(noSettleClock{})))
	sess, err := d.Open(context.Background(), r.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()
	if got := sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency); got.CanWrite() {
		t.Errorf("a forged Profile plus consent produced a writable session (%+v) — the fail-safe direction must survive consent", got)
	}
}

func TestBroadcastFloodOpensCleanlyAndIsCounted(t *testing.T) {
	// R9-SPLIT, half one. A BROADCAST flood (to = 00) is dropped by the
	// CI-V accumulator BEFORE the engine sees a frame, so it can never
	// reach a drain and never trips the cap. Init therefore succeeds
	// NORMALLY — no ErrDrainCapExceeded, no diagnostic about one — the
	// engine's own counter reads ZERO, and the ADAPTER's counter is where
	// the traffic shows up. All three, including the zero.
	r := occupiedRadio(t, "G01-001")
	r.StartBroadcastFlood(10 * time.Millisecond)
	sess := openSession(t, r)
	if sess.SessionInfo().InitDrainCapExceeded {
		t.Error("a broadcast flood tripped the drain cap — the adapter is supposed to drop those before the engine, so this would mean the address filter is not working")
	}
	if got := sess.eng.UnexpectedFrames(); got != 0 {
		t.Errorf("the engine counted %d unexpected frames — a broadcast never reaches it", got)
	}
	if got := sess.Diagnostics().UnexpectedFrames; got == 0 {
		t.Error("Diagnostics() reports zero unexpected frames on a line saturated with broadcasts — this is exactly the healthy-looking zero the adapter's counter exists to prevent")
	}
}

func TestDiagnosticsCountsBroadcastsTheAccumulatorAte(t *testing.T) {
	r := occupiedRadio(t, "G01-001")
	sess := openSession(t, r)
	before := sess.Diagnostics().UnexpectedFrames
	const n = 25
	if err := r.EmitBroadcast(n); err != nil {
		t.Fatalf("EmitBroadcast: %v", err)
	}
	// Sampled AFTER the frames have been taken off the pipe, and polled
	// because the reader goroutine pushes them through the accumulator
	// asynchronously.
	deadline := time.Now().Add(2 * time.Second)
	var got uint64
	for time.Now().Before(deadline) {
		got = sess.Diagnostics().UnexpectedFrames
		if got >= before+n {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got < before+n {
		t.Errorf("Diagnostics().UnexpectedFrames = %d after %d broadcasts, want at least %d — against an engine-only counter this reads zero, which is the point", got, n, before+n)
	}
}

func TestAddressedFloodIsNonfatalAtOpenButLaterQuarantineFailsClosed(t *testing.T) {
	// R9-SPLIT, half two, and it is the ONLY flood shape that reaches a
	// drain: frames addressed to the CONTROLLER pass the accumulator's
	// address filter and postpone "quiet" until the cap.
	r := occupiedRadio(t, "G01-001")
	r.StartAddressedFlood(40 * time.Millisecond)

	// (a) NONFATAL AT OPEN. Init's initial ErrDrainCapExceeded is
	// diagnosed and the session opens: transceive is factory-ON on this
	// radio and this tier ships no off-switch, so a line that never goes
	// quiet is a normal operating state at open rather than a fault.
	sess := openSession(t, r)
	if !sess.SessionInfo().InitDrainCapExceeded {
		t.Fatal("Init did not report a drain-cap failure under a controller-addressed flood — either the flood is not reaching the engine or the diagnostic is not recorded")
	}

	// (b) LATER, IT FAILS CLOSED. The radio goes silent while the flood
	// continues: a read times out, which marks the engine suspect, and the
	// NEXT exchange's entry quarantine cannot drain the line — so it
	// refuses to transmit rather than pretending the stream is clean.
	r.GoSilent()
	p := civic705.Profile()
	cmd, err := p.BuildMemoryRead(addrOf(t, "G01-001"))
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	once := civ.CIVReadSpec(p.MemoryAnswerMatcher(), 0)
	if _, err := sess.eng.Do(context.Background(), cmd, once); !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("the silent read returned %v, want a timeout", err)
	}
	_, err = sess.eng.Do(context.Background(), cmd, once)
	if !errors.Is(err, transport.ErrQuarantineFailed) {
		t.Errorf("the next exchange returned %v, want ErrQuarantineFailed — every quarantine drain after Init stays fail-closed", err)
	}
}

func TestSessionInfoCarriesWhatTheNeutralShapeCannot(t *testing.T) {
	// driver.SessionDiagnostics holds ONE counter; the ID token, the
	// fingerprint verdict and the flood observation are three facts it
	// cannot carry, so the model surface holds them.
	r := occupiedRadio(t, "G01-001")
	sess := openSession(t, r)
	info := sess.SessionInfo()
	if info.IDToken == "" {
		t.Error("SessionInfo carries no ID token")
	}
	if !info.Fingerprinted {
		t.Error("SessionInfo does not report the fingerprint")
	}
	if info.InitDrainCapExceeded {
		t.Error("SessionInfo reports a flood on a quiet line")
	}
	// The neutral shape genuinely cannot hold these: it is one unsigned
	// counter, asserted here so that a future widening of the seam is a
	// deliberate change rather than a silent one.
	var neutral driver.SessionDiagnostics
	if reflect.TypeOf(neutral).NumField() != 1 {
		t.Errorf("driver.SessionDiagnostics now has %d fields — re-read whether SessionInfo still carries only what the seam cannot", reflect.TypeOf(neutral).NumField())
	}
}

func TestOpenClosesThePortWhenTheProbeFails(t *testing.T) {
	// Open takes ownership of the port on BOTH outcomes. A failed probe
	// that left the port open would leak a serial device the caller was
	// told not to close.
	r := newScriptedRadio(t, radioImage{silentID: true})
	if _, err := openSessionErr(t, r); err == nil {
		t.Fatal("Open succeeded against a silent radio")
	}
	if _, err := r.Port().Write([]byte{0x00}); err == nil {
		t.Error("the port is still open after Open failed — Open owns it on both outcomes")
	}
}

func TestSessionCloseIsIdempotent(t *testing.T) {
	sess := openSession(t, occupiedRadio(t, "G01-001"))
	first := sess.Close()
	second := sess.Close()
	if !errors.Is(second, first) && second != first {
		t.Errorf("Close() returned %v then %v — it must be idempotent", first, second)
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// 1. A probe that finds a record confirms the fingerprint.
func TestOpen_FingerprintConfirmed(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer)
	d := civDiagnostics(t, sess)
	if !d.Fingerprinted {
		t.Errorf("CIVDiagnostics().Fingerprinted = false after a probe that read a 39-byte record — the length fingerprint is what spec D3.2 asks the probe for")
	}
	if d.ProbeSlotsRead != 1 {
		t.Errorf("ProbeSlotsRead = %d, want 1 — the search stops at the FIRST record", d.ProbeSlotsRead)
	}
	if got, want := sess.Identity().CATID, "b6:00"; got != want {
		t.Errorf("Identity().CATID = %q, want %q — the address hex, a colon, and the observed 19 00 token", got, want)
	}
}

// 2. An all-FA search opens the session UNFINGERPRINTED, with the
// diagnostic recorded and no error.
func TestOpen_EmptyRadioOpensOnAddressEvidence(t *testing.T) {
	peer := newRespondingPort(t) // no records at all: every probe read is answered FA
	sess := openSession(t, peer)
	d := civDiagnostics(t, sess)
	if d.Fingerprinted {
		t.Error("Fingerprinted = true on a radio that answered FA to every probe slot — no record was ever seen, so no length was ever checked")
	}
	if d.ProbeSlotsRead != probeSlots {
		t.Errorf("ProbeSlotsRead = %d, want %d — an FA is an EMPTY channel (D5 entry 2(a)), not an error, so the search runs to its bound", d.ProbeSlotsRead, probeSlots)
	}
	if got := sess.Identity().CATID; got != "b6:00" {
		t.Errorf("Identity().CATID = %q, want %q — an empty radio still identified itself", got, "b6:00")
	}
}

// 3. A record of the sibling's length is WrongRadioError with PROVISIONAL
// attribution naming the IC-7300, and the error text says both lengths are
// ASSUMED derivations.
func TestOpen_ForeignRecordLengthIsWrongRadio(t *testing.T) {
	peer := newRespondingPort(t, withRecordOfLength(1, 39))
	_, err := New(Simulated).Open(context.Background(), peer, driver.Identity{Port: "test"})
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want *driver.WrongRadioError", err)
	}
	if wrong.WantModel != "IC-7300MK2" {
		t.Errorf("WantModel = %q, want %q", wrong.WantModel, "IC-7300MK2")
	}
	if wrong.GotModel != "IC-7300 (provisional)" {
		t.Errorf("GotModel = %q, want %q — the sibling's 39-byte record, attributed provisionally and to that model alone", wrong.GotModel, "IC-7300 (provisional)")
	}
	if wrong.Got == "" {
		t.Error("WrongRadioError.Got is empty — driver.WrongRadioError.Error() renders it as the connected radio's ID, and an empty string there prints (CAT ID \"\")")
	}
	for _, want := range []string{"provisional", "39", "45", "ASSUMED"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error text %q does not contain %q — the attribution is provisional and BOTH record lengths are ASSUMED derivations from printed field widths (spec D3.2; plan decision D10)", err, want)
		}
	}
}

// 4. A record of an unrecognised length is refused WITHOUT attribution.
func TestOpen_UnknownRecordLengthIsRefusedWithoutAttribution(t *testing.T) {
	peer := newRespondingPort(t, withRecordOfLength(1, 41))
	_, err := New(Simulated).Open(context.Background(), peer, driver.Identity{Port: "test"})
	if err == nil {
		t.Fatal("Open succeeded against a radio answering a 41-byte record — no model in this tier declares that length (41 is the IC-7300's DATA-AREA figure, record plus channel address, which is exactly the sort of near-miss a length table must not guess a model from)")
	}
	var wrong *driver.WrongRadioError
	if errors.As(err, &wrong) {
		t.Errorf("Open error is a *driver.WrongRadioError naming %q — 41 bytes is in no model's table, and this driver must claim NO model at all rather than guess one", wrong.GotModel)
	}
	if !errors.Is(err, civ.ErrRecordLength) {
		t.Errorf("Open error = %v, want it to wrap civ.ErrRecordLength", err)
	}
	if !strings.Contains(err.Error(), "41") {
		t.Errorf("error text %q does not name the observed length", err)
	}
}

// 5. A 19 00 answer addressed to someone else is not accepted as identity.
func TestOpen_RequiresAnAddressMatchedIdentityReply(t *testing.T) {
	peer := newRespondingPort(t, withMisaddressedIDAnswer(), withRecord(1, populatedRecord))
	_, err := New(Simulated).Open(context.Background(), peer, driver.Identity{Port: "test"})
	if err == nil {
		t.Fatal("Open succeeded on an identity reply addressed to a different controller — what identifies the radio at this step is that an ADDRESS-MATCHED reply arrived at all (spec D3.2)")
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("Open error = %v, want a timeout: the accumulator's address filter drops a frame addressed elsewhere BEFORE the engine sees it, so the probe hears silence", err)
	}
}

// 6. The 19 00 token VALUE is never matched — two different tokens both
// open, and both appear in Identity.CATID after the address hex.
func TestOpen_IdentityTokenIsRecordedNeverMatched(t *testing.T) {
	for _, tc := range []struct {
		token []byte
		want  string
	}{
		{[]byte{0x00}, "b6:00"},
		{[]byte{0xB6, 0x01}, "b6:b601"},
	} {
		peer := newRespondingPort(t, withIDToken(tc.token), withRecord(1, populatedRecord))
		sess, err := New(Simulated).Open(context.Background(), peer, driver.Identity{Port: "test"})
		if err != nil {
			t.Fatalf("token % X: Open: %v — the 19 00 reply VALUE is undocumented on every model in this tier (D5 entry 7) and must never be compared against an expected one", tc.token, err)
		}
		if got := sess.Identity().CATID; got != tc.want {
			t.Errorf("token % X: Identity().CATID = %q, want %q", tc.token, got, tc.want)
		}
		_ = sess.Close()
	}
}

// 7. Init writes NOTHING — no transceive-off, no clear, no 1A 05. The FIRST
// frame the radio ever sees is the identity read.
func TestOpen_InitSendsNothing(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess, err := New(Simulated).Open(context.Background(), peer, driver.Identity{Port: "test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()
	frames := peer.Received()
	if len(frames) == 0 {
		t.Fatal("the peer saw no frames at all — the probe did not run and every assertion below would be vacuous")
	}
	want := []byte{0xFE, 0xFE, 0xB6, 0xE0, 0x19, 0x00, 0xFD}
	if !bytes.Equal(frames[0], want) {
		t.Errorf("first frame = % X, want % X — CI-V Init sends NOTHING (spec D2, adjudication 3): broadcasts are excluded by address matching, never by writing a setting to the radio", frames[0], want)
	}
	for i, f := range frames {
		if cn, sc, ok := civ.FrameCommand(f); ok && cn == 0x1A && sc == 0x05 {
			t.Errorf("frame %d = % X is a 1A 05 menu write — nothing in this tier may send one", i, f)
		}
	}
}

// 8a. A to=00 broadcast flood: Init SUCCEEDS, zero engine events, and the
// adapter's AccumulatorStats counts the frames.
func TestOpen_BroadcastFloodDoesNotReachTheEngine(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord), withBroadcasts(5*time.Millisecond))
	sess := openSession(t, peer)
	d := civDiagnostics(t, sess)
	if d.InitDrainCapExceeded {
		t.Error("Init's drain hit its cap under a to=00 flood — civ.FrameAccumulator filters a broadcast BEFORE any engine event, so a transceive flood CANNOT reach DrainPolicy.Cap (enablers decision 5)")
	}
	if d.Unexpected == 0 {
		t.Error("AccumulatorStats().Unexpected = 0 under a broadcast flood — the frames are dropped by the address filter and COUNTED there; a zero here means the driver is reading the wrong counter")
	}
	// THE TEST looks at the engine's own counter; the DRIVER never does
	// (R1). It is the direct statement of "zero engine events", and it is
	// what distinguishes this case from the addressed-flood one below.
	if n := session(t, sess).eng.UnexpectedFrames(); n != 0 {
		t.Errorf("the engine counted %d unexpected frames — a to=00 broadcast must never become an engine event at all", n)
	}
}

// 8b. A controller-addressed flood: Init returns ErrDrainCapExceeded and the
// driver treats it as nonfatal-with-diagnostic — the open still succeeds.
func TestOpen_AddressedFloodAtInitIsNonfatalWithDiagnostic(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord), withAddressedFlood(5*time.Millisecond))
	sess, err := New(Simulated).Open(context.Background(), peer, driver.Identity{Port: "test"})
	if err != nil {
		t.Fatalf("Open: %v — CI-V's bounded drain to quiet CANNOT fail the open (spec D2): transceive is factory-ON with no off-switch shipped, so a line that never goes quiet is a NORMAL operating state at open", err)
	}
	defer sess.Close()
	if !civDiagnostics(t, sess).InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded = false after a controller-addressed flood — nonfatal does not mean unrecorded: a line that never went quiet is a wire-health fact the user is entitled to")
	}
}

// 8c. A LATER quarantine drain failure is FAIL-CLOSED, not swallowed.
//
// The engine's drain rule is split by WHEN: Init's initial drain is
// nonfatal (8b), and every drain after it stays fail-closed exactly as the
// engine has it. The later drain available at this task is the one
// Engine.Do runs BETWEEN a read's attempts, so the peer is let go quiet
// through Init and the flood is started once the identity read has been
// received; the probe's first memory read then times out, its retry drain
// hits the cap, and the failure comes OUT of Open rather than being
// diagnosed away.
func TestSession_LaterQuarantineDrainFailureFailsClosed(t *testing.T) {
	peer := newRespondingPort(t,
		withSilentReads(),
		withAddressedFloodAfter(1, 5*time.Millisecond),
	)
	_, err := New(Simulated).Open(context.Background(), peer, driver.Identity{Port: "test"})
	if err == nil {
		t.Fatal("Open succeeded despite a drain failure after Init — only the INITIAL drain is nonfatal; everything after it is fail-closed, because Do's quarantine exists to stop an abandoned exchange's reply being read as this one's answer")
	}
	if !errors.Is(err, transport.ErrDrainCapExceeded) {
		t.Errorf("Open error = %v, want it to wrap transport.ErrDrainCapExceeded", err)
	}
}

// 9. StopBits() is 1.
func TestStopBits(t *testing.T) {
	var d any = New(RealHardware)
	r, ok := d.(interface{ StopBits() int })
	if !ok {
		t.Fatal("the driver value does not report its serial framing — internal/wiring consults driver.SerialFramingReporter before the port is opened, and a driver that answers nothing is opened at the transport's own 8-N-2 default")
	}
	if got := r.StopBits(); got != 1 {
		t.Errorf("StopBits() = %d, want 1 — ASSUMED at spec D5 entry 8, lift MK2-R5. This document prints no character format anywhere (matrix §3.1); an Icom manual's \"8 bit / 1 stop\" line about the DATA/RTTY port is NOT evidence about CI-V.", got)
	}
}

// session narrows a session value to this package's concrete type.
//
// It takes `any` rather than driver.Session so the SAME call site works
// whether the constructor's declared result is the concrete type or the
// neutral seam — which is what it becomes once write.go completes the
// Session's methods.
func session(t *testing.T, sess any) *Session {
	t.Helper()
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("session is %T, not *ic7300.Session", sess)
	}
	return s
}

// civDiagnostics reads the model-specific diagnostics surface.
func civDiagnostics(t *testing.T, sess any) CIVDiagnostics {
	t.Helper()
	return session(t, sess).CIVDiagnostics()
}

// The reporter is asserted on the CONCRETE DRIVER, unconditionally:
// internal/wiring holds the driver value before any port exists, so a
// Session-side reporter could only ever be consulted after the framing had
// already been guessed.
var _ driver.SerialFramingReporter = (*ic7300mk2Driver)(nil)

// The probe is BOUNDED and confined to MEM. A P1/P2 record's shape is
// itself ASSUMED — this document never states whether a scan-edge record
// carries the same 45 bytes (§3.16 A5) — so the probe must not learn the
// length fingerprint from a record whose layout is not established
// (ic7300mk2-scan-edge-record-layout, lift MK2-R10).
func TestProbeIsBoundedAndConfinedToMemory(t *testing.T) {
	if probeSlots != 8 {
		t.Errorf("probeSlots = %d, want 8 — bounded per spec D3.2 and small", probeSlots)
	}
	peer := newRespondingPort(t)
	openSession(t, peer)
	for i, f := range peer.Received() {
		cn, sc, ok := civ.FrameCommand(f)
		if !ok || cn != 0x1A || sc != 0x00 {
			continue
		}
		if len(f) != 9 {
			t.Errorf("frame %d = % X is a 1A 00 SET — the probe reads and never writes", i, f)
			continue
		}
		if ch := bcdChannel(f[6], f[7]); ch < 1 || ch > probeSlots {
			t.Errorf("frame %d probes channel %d — the search is MEM channels 1..%d, and 100/101 are P1/P2, whose record shape is not established", i, ch, probeSlots)
		}
	}
}

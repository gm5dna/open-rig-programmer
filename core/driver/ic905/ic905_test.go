// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The optional capabilities this driver claims, asserted by the COMPILER
// rather than left to shape: a renamed method or a changed receiver
// becomes a build failure instead of a capability that silently stops
// being offered (internal/wiring's optional-capability helpers return
// false for a type that does not satisfy the interface, with no error and
// no log).
var (
	_ driver.Driver              = (*ic905Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)

// testToken is a plausible 19 00 reply value. Its CONTENT is arbitrary
// and that is the point: the reply value is undocumented on all six of
// this tier's documents (D5 entry 7), so the probe records it and
// compares it against nothing.
var testToken = []byte{0x94}

// openFor Opens a session against a scripted radio serving img, failing
// the test if the open does not succeed. It returns both, because most
// tests want to inspect the transcript as well as the session.
func openFor(t *testing.T, img radioImage) (*respondingPort, *Session) {
	t.Helper()
	if len(img.idToken) == 0 && !img.idOnce {
		img.idToken = testToken
	}
	p := newRespondingPort(t, img)
	sess, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	return p, s
}

// TestRecordFixture_MatchesTheGoldenVectors proves the hand-assembled
// fixture in records_test.go reproduces the FROZEN evidence, byte for
// byte, at BOTH declared lengths.
//
// It is the first test in this file on purpose. Every read, write and
// probe test below is driven from that fixture, so a fixture that had
// drifted from the vectors would make every one of them assert the wrong
// thing in agreement. The expected bytes are transcribed by hand from
// core/civ/ic905/testdata/ic905-vectors.golden's two set vectors, with
// the seven-byte envelope and the four address bytes removed (spec
// Erratum 1's record-only convention).
func TestRecordFixture_MatchesTheGoldenVectors(t *testing.T) {
	// PARALLEL because this package's tests are wire-paced, not
	// CPU-bound: transport.Engine applies a 20 ms settle after every
	// exchange, so an Open spends seconds asleep and several can overlap
	// at no cost. See TestOpen_FullWalkIsOptInAndReportsComplete for the
	// one that makes it worth doing.
	t.Parallel()
	spaces := bytes.Repeat([]byte{0x20}, 24)
	name := []byte("HIGHLAND BASE905")

	// set-record-name-with-space-68, record bytes only: 64 of them.
	want64 := concat(
		[]byte{0x00},                                     // ⑤
		[]byte{0x00, 0x00, 0x50, 0x44, 0x01},             // ⑥~⑩ 144.500000 MHz
		[]byte{0x05, 0x01, 0x00, 0x00, 0x00},             // ⑪ ⑫ ⑬ ⑭ ⑮
		[]byte{0x00, 0x08, 0x85, 0x00, 0x08, 0x85},       // ⑯~⑱, ⑲~㉑
		[]byte{0x00, 0x00, 0x23, 0x00, 0x00, 0x00, 0x00}, // ㉒ ㉓㉔ ㉕ ㉖~㉘
		spaces, name,
	)
	// set-record-name-with-space-69: the same record with a SIX-byte
	// frequency at 10.25 GHz. 65 bytes.
	want65 := concat(
		[]byte{0x00},
		[]byte{0x00, 0x00, 0x00, 0x50, 0x02, 0x01}, // 10 250.000000 MHz
		[]byte{0x05, 0x01, 0x00, 0x00, 0x00},
		[]byte{0x00, 0x08, 0x85, 0x00, 0x08, 0x85},
		[]byte{0x00, 0x00, 0x23, 0x00, 0x00, 0x00, 0x00},
		spaces, name,
	)

	for _, tt := range []struct {
		name string
		got  []byte
		want []byte
	}{
		{"64-byte record", goldenRecord(144_500_000, 5).build(), want64},
		{"65-byte record", goldenRecord(10_250_000_000, 6).build(), want65},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.got) != len(tt.want) {
				t.Fatalf("fixture is %d bytes, the golden vector's record is %d", len(tt.got), len(tt.want))
			}
			if !bytes.Equal(tt.got, tt.want) {
				t.Fatalf("the fixture does not reproduce the golden vector:\n  fixture % X\n  golden  % X", tt.got, tt.want)
			}
		})
	}
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// TestOpen_RequiresAnAddressMatchedIdentityReply is the probe's whole
// content: something answered AT AC, TO E0, and the reply's VALUE is
// RECORDED without being matched.
//
// The 19 00 reply value is undocumented on all six of this tier's
// documents (spec D5 entry 7; matrix §3.12 gives the command table row a
// description and an EMPTY Data cell and nothing more). So the probe
// cannot compare a token. Register: D5 entry 7. Lift: ic905-R-02.
func TestOpen_RequiresAnAddressMatchedIdentityReply(t *testing.T) {
	t.Parallel()
	p, s := openFor(t, radioImage{idToken: []byte{0x94}})

	if got, want := s.Identity().CATID, "AC:94"; got != want {
		t.Errorf("Identity().CATID = %q, want %q — the address and the observed token, in the ONE format spec D3.2 fixes", got, want)
	}
	// THE SESSION'S CAPABILITIES CARRY IT TOO, and that is the half REV 1
	// missed (Codex 12): core/clone's ReadAll records the SESSION
	// capabilities' CATID into the codeplug, so a driver that set it only
	// on Identity would never get the observed token into a saved file.
	if got, want := s.Capabilities().CATID, "AC:94"; got != want {
		t.Errorf("Capabilities().CATID = %q, want %q", got, want)
	}
	// And the STATIC set keeps the bare address: there is no observed
	// token before a session exists.
	if got, want := New(RealHardware).Capabilities().CATID, "AC"; got != want {
		t.Errorf("static Capabilities().CATID = %q, want %q", got, want)
	}

	// Init transmitted NOTHING. The first frame on the wire is the probe.
	frames := p.Transcript()
	if len(frames) == 0 {
		t.Fatal("the radio received no frames at all")
	}
	want := []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD}
	if !bytes.Equal(frames[0], want) {
		t.Errorf("the first frame is % X, want % X — CI-V Init performs NO radio mutation (spec D2, adjudication 3), so the identity read must be the first thing on the wire", frames[0], want)
	}
}

// TestOpen_TheIdentityTokenIsRecordedNotMatched drives two DIFFERENT
// tokens through the same probe and requires both to open. A matcher that
// checked the value would refuse every real radio this tier has never
// seen, which is all of them.
func TestOpen_TheIdentityTokenIsRecordedNotMatched(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		token []byte
		want  string
	}{
		{[]byte{0x94}, "AC:94"},
		{[]byte{0x00}, "AC:00"},
		{[]byte{0xAC, 0x01}, "AC:AC01"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			_, s := openFor(t, radioImage{idToken: tt.token})
			if got := s.Identity().CATID; got != tt.want {
				t.Errorf("Identity().CATID = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestOpen_ATimeoutIsATimeoutAndNotAWrongRadio.
//
// A wrong radio at a DIFFERENT default address simply TIMES OUT: its
// reply is addressed to the controller, so the accumulator returns it,
// and the codec's own matcher then refuses it because the `from` byte is
// not this profile's 0xAC. Nothing about that is a wrong-radio finding,
// and this driver must not dress it up as one.
//
// THE RECORD-LENGTH FINGERPRINT PROTECTS AGAINST SAME-ADDRESS CONFUSION
// ONLY (spec D3.2), and this test says so rather than implying wider
// protection: a radio that is not at AC is a radio this driver never
// hears from.
func TestOpen_ATimeoutIsATimeoutAndNotAWrongRadio(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		img  radioImage
	}{
		{"a radio at another address", radioImage{idToken: []byte{0x94}, idFrom: 0x94}},
		{"nothing on the port at all", radioImage{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := newRespondingPort(t, tt.img)
			_, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
			if err == nil {
				t.Fatal("Open succeeded — an unanswered 19 00 must fail")
			}
			if !errors.Is(err, transport.ErrTimeout) {
				t.Errorf("Open error = %v, want a transport.ErrTimeout", err)
			}
			if errors.Is(err, driver.ErrWrongRadio) {
				t.Errorf("Open error = %v — a timeout is a TIMEOUT, not a wrong radio: this driver knows nothing about what is on the port", err)
			}
		})
	}
}

// TestOpen_ABroadcastFloodDoesNotReachTheEngineAndInitSucceeds is
// R9-SPLIT branch (a).
//
// A transceive flood addressed to 00 is dropped by the accumulator's
// ADDRESS FILTER before any engine event exists, so the idle timer never
// re-arms and Init SUCCEEDS normally. Zero engine events; the frames
// appear only in AccumulatorStats().Unexpected. InitDrainCapExceeded is
// FALSE here, and that is the whole distinction REV 2 collapsed.
func TestOpen_ABroadcastFloodDoesNotReachTheEngineAndInitSucceeds(t *testing.T) {
	t.Parallel()
	_, s := openFor(t, radioImage{idToken: testToken, floodOnOpen: true, floodTo: 0x00})

	d := s.Diagnostics905()
	if d.InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is true under a BROADCAST flood — those frames die at the address filter and never reach the drain (R9-SPLIT branch (a))")
	}
	if d.Accumulator.Unexpected == 0 {
		t.Error("the accumulator counted no unexpected frames under a broadcast flood — the frames must be COUNTED, not silently dropped (transport safety obligation 3)")
	}
}

// TestOpen_AControllerAddressedFloodMakesInitDrainCapExceededNonfatal is
// R9-SPLIT branch (b).
//
// Frames addressed to the CONTROLLER reach the engine, re-arm the idle
// timer on every arrival, and drive the initial drain into DrainCap. Init
// returns ErrDrainCapExceeded and the driver treats it
// NONFATAL-WITH-DIAGNOSTIC, because the spec's bounded initial drain
// "cannot fail the open" — the line is noisy, not wrong.
func TestOpen_AControllerAddressedFloodMakesInitDrainCapExceededNonfatal(t *testing.T) {
	t.Parallel()
	_, s := openFor(t, radioImage{idToken: testToken, floodOnOpen: true, floodTo: 0xE0})

	d := s.Diagnostics905()
	if !d.InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is false under a CONTROLLER-ADDRESSED flood — the drain must have reached its absolute cap, and the open must record it")
	}
	if s.Identity().CATID == "" {
		t.Error("the session opened with no identity — the nonfatal branch must still probe")
	}
}

// TestSession_ALaterQuarantineDrainFailureFailsClosed.
//
// The initial drain's leniency is the INITIAL drain's alone. A LATER
// drain failure means an exchange's own outcome is unknowable, and
// Engine.Do returns the failure unchanged: the driver adds no second
// leniency anywhere.
//
// The radio here answers 19 00 exactly once — enough to open — and then
// goes silent while a controller-addressed flood starts. The next read
// times out, its retry's quarantine drain meets the flood, and the
// failure comes back rather than being swallowed.
func TestSession_ALaterQuarantineDrainFailureFailsClosed(t *testing.T) {
	t.Parallel()
	p, s := openFor(t, radioImage{idToken: testToken, idOnce: true})

	p.silence()
	p.startFlood(0xE0, 4*time.Second)
	defer p.stopFlood()

	cmd, err := s.profile.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	_, err = s.eng.Do(context.Background(), cmd, s.idSpec())
	if err == nil {
		t.Fatal("the read succeeded — the radio is silent and the line is flooded")
	}
	if !errors.Is(err, transport.ErrDrainCapExceeded) {
		t.Errorf("error = %v, want a transport.ErrDrainCapExceeded — a LATER drain failure must fail closed, never be treated as the open's nonfatal case", err)
	}
}

// TestSession_BroadcastCountsComeFromTheAdapterNotTheEngine is the
// executable half of ruling R1, and the reason the per-model diagnostics
// carrier exists at all.
//
// Two numbers answering two different questions: under a to = 00 flood
// the ENGINE's UnexpectedFrames stays 0 — the accumulator swallowed every
// frame before the engine could count one — while the ADAPTER's
// Unexpected is non-zero. A driver that reached past the adapter for
// Engine.UnexpectedFrames would report a healthy zero on a line saturated
// with transceive.
func TestSession_BroadcastCountsComeFromTheAdapterNotTheEngine(t *testing.T) {
	t.Parallel()
	p, s := openFor(t, radioImage{idToken: testToken, floodOnOpen: true, floodTo: 0x00})

	if got := s.Diagnostics().UnexpectedFrames; got != 0 {
		t.Errorf("Diagnostics().UnexpectedFrames = %d, want 0 — a broadcast never becomes an engine event", got)
	}
	if got := s.Diagnostics905().Accumulator.Unexpected; got == 0 {
		t.Error("Diagnostics905().Accumulator.Unexpected = 0 — the broadcasts must be counted on the ADAPTER's side of the address filter")
	}
	// LIVE, not cached: a snapshot taken now, against traffic that
	// arrives AFTER the open, must exceed one taken before it. A stored
	// value would leave the broadcast counts frozen at whatever the open
	// happened to see — which is the one number this carrier exists to
	// report.
	//
	// A FRESH FLOOD, because the open's own one is long over: discovery
	// walks two hundred addresses at the transport's pacing, so several
	// seconds pass between the flood the Init assertions above need and
	// this one.
	first := s.Diagnostics905().Accumulator.Unexpected
	p.startFlood(0x00, 3*time.Second)
	defer p.stopFlood()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if s.Diagnostics905().Accumulator.Unexpected > first {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("Diagnostics905().Accumulator.Unexpected never advanced while a flood ran — the snapshot must be SUMMED LIVE, not stored at Open")
}

// TestIDSpec_IsAReadWithOneRetry pins the probe's spec as E1's helper
// returns it: a ClassRead with exactly one additional attempt. Retrying a
// read is safe (transport safety obligation 2) because it is idempotent;
// retrying anything else on this driver is not, and no other spec here
// carries a non-zero count.
func TestIDSpec_IsAReadWithOneRetry(t *testing.T) {
	t.Parallel()
	_, s := openFor(t, radioImage{idToken: testToken})
	sp := s.idSpec()
	if sp.Class != transport.ClassRead {
		t.Errorf("idSpec().Class = %v, want ClassRead", sp.Class)
	}
	if sp.RetryReads != 1 {
		t.Errorf("idSpec().RetryReads = %d, want 1", sp.RetryReads)
	}
	if sp.Match == nil {
		t.Error("idSpec().Match is nil — the matcher must come from the CODEC (deviation (a)), never be omitted")
	}
}

// TestOpen_TakesOwnershipOfThePortOnFailure: core/driver's contract is
// that Open owns port on BOTH outcomes, so a failed open must leave
// nothing for the caller to close and nothing running.
func TestOpen_TakesOwnershipOfThePortOnFailure(t *testing.T) {
	t.Parallel()
	p := newRespondingPort(t, radioImage{}) // answers no 19 00 at all
	_, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded against a silent radio")
	}
	if _, werr := p.Port().Write([]byte{0x00}); werr == nil {
		t.Error("the port is still open after a failed Open — Open must close what it holds before returning an error")
	}
}

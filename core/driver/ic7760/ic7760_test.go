// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// goldenRecord is this package's 25-byte record fixture, laid out by hand
// from the frozen geometry witness (core/civ/ic7760/testdata/
// IC-7760-geometry-witness.csv) and written out in wire bytes rather than
// built by the codec: a fixture assembled by the encoder under test would
// agree with a wrong offset as happily as a right one.
//
// IT IS NOT THE FROZEN GOLDEN VECTOR, and stage-review finding F1 is why
// that is said out loud. An earlier comment here claimed these bytes WERE
// IC-7760-vectors.golden's `set-record-name-with-space` record; they are
// not — that vector carries 14 100 000 Hz, tone/tone OFF and "ALPHA BETA".
// The frozen vector is replayed byte-for-byte where it belongs, in
// core/civ/ic7760's TestGolden. These bytes exercise the DRIVER against a
// second, differently-valued record of the same documented shape.
//
//	offset  0      00            UNMAPPED (E6): SELECT-group nibble, OFF
//	offset  1-5    00 00 25 14 00  14 250 000 Hz, little-endian BCD
//	offset  6      01            USB
//	offset  7      01            FIL1
//	offset  8      01            hi = data mode OFF (E6), lo = TONE
//	offset  9-11   00 08 85      tone TX 885 deci-Hz (88.5 Hz)
//	offset  12-14  00 10 00      tone RX 1000 deci-Hz (100.0 Hz)
//	offset  15-24  "HOME QTH01"
var goldenRecord = []byte{
	0x00,
	0x00, 0x00, 0x25, 0x14, 0x00,
	0x01,
	0x01,
	0x01,
	0x00, 0x08, 0x85,
	0x00, 0x10, 0x00,
	0x48, 0x4F, 0x4D, 0x45, 0x20, 0x51, 0x54, 0x48, 0x30, 0x31,
}

// The two probe frames this package's tests compare byte for byte, from
// the plan's ONE TABLE and from the golden vectors' own `read-record` and
// `read-transceiver-id` lines.
var (
	idReadFrame = []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x19, 0x00, 0xFD}
)

func memReadFrame(ch int) []byte {
	hi, lo := encodeChannel(ch)
	return []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, hi, lo, 0xFD}
}

// openWith opens a session against p, failing the test on error.
func openWith(t *testing.T, p *scriptedPort, opts ...Option) *Session {
	t.Helper()
	d := New(Simulated, opts...)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open: %v\ntranscript:\n  %s", err, hexFrames(p.Transcript()))
	}
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// occupiedRadio is the ordinary image: an ID answer and a record in
// channel 1, so the probe fingerprints on its first read.
func occupiedRadio() radioImage {
	return radioImage{
		idToken: []byte{0xB2},
		records: map[int][]byte{1: goldenRecord},
	}
}

// openErr opens against p and returns the error, failing if a session was
// somehow produced.
func openErr(t *testing.T, p *scriptedPort) error {
	t.Helper()
	sess, err := New(Simulated).Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err == nil {
		_ = sess.Close()
		t.Fatalf("Open succeeded; transcript:\n  %s", hexFrames(p.Transcript()))
	}
	return err
}

// TestOpen_AddressMatchedIDIsRequired pins spec D3.2's opening move: what
// identifies the radio is that an ADDRESS-MATCHED 19 00 reply arrived at
// all, so a reply from another station, a broadcast, or silence must not
// open a session.
//
// THE VALUE IS NEVER MATCHED, only recorded: the 19 00 reply is
// undocumented on every model in this tier (D5 entry 7, register entry
// ic7760-id-reply), so the accepting case below uses a token no reading of
// this radio's guide predicts, and the session still opens.
func TestOpen_AddressMatchedIDIsRequired(t *testing.T) {
	t.Run("silence does not open", func(t *testing.T) {
		// A radio moved off B2, an unsuitable virtual COM port, an absent
		// or powered-off radio, and a wrong assumed baud all look exactly
		// like this: nothing comes back, and nothing can be attributed.
		p := newScriptedPort(t, radioImage{idToken: nil})
		if err := openErr(t, p); !strings.Contains(err.Error(), "identity probe") {
			t.Errorf("err = %v, want the 19 00 identity probe to name itself", err)
		}
	})
	t.Run("a reply from another station does not open", func(t *testing.T) {
		p := newScriptedPort(t, radioImage{idToken: []byte{0x42}, idFrom: 0x94})
		_ = openErr(t, p)
	})
	t.Run("a broadcast does not open", func(t *testing.T) {
		// to=00 is a transceive broadcast, not an answer to this
		// controller. The address check is the CODEC's — the matcher from
		// Profile.TransceiverIDAnswerMatcher checks both to and from — so
		// this arm is here to prove the driver leans on it.
		p := newScriptedPort(t, radioImage{idToken: []byte{0x42}, idBroadcast: true})
		_ = openErr(t, p)
	})
	t.Run("an address-matched reply opens and its token is recorded", func(t *testing.T) {
		p := newScriptedPort(t, radioImage{
			idToken: []byte{0x7A},
			records: map[int][]byte{1: goldenRecord},
		})
		s := openWith(t, p)
		if got := s.OpenDiagnostics().IDToken; !bytes.Equal(got, []byte{0x7A}) {
			t.Errorf("IDToken = % X, want 7A", got)
		}
		if got := s.Identity().CATID; got != "b27a" {
			t.Errorf("CATID = %q, want %q (the address, then what this radio answered)", got, "b27a")
		}
		if got := p.Transcript()[0]; !bytes.Equal(got, idReadFrame) {
			t.Errorf("the first frame was % X, want % X", got, idReadFrame)
		}
	})
}

// TestOpen_InitWritesNothing: E1's InitSequence is EMPTY, so a CI-V Init is
// a DRAIN ALONE. Open's whole traffic is the identity read and the bounded
// probe schedule — no 1A 05, no transceive setting, no clear, nothing.
func TestOpen_InitWritesNothing(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	openWith(t, p)
	got := p.Transcript()
	want := [][]byte{idReadFrame, memReadFrame(1)}
	if len(got) != len(want) {
		t.Fatalf("Open put %d frames on the wire, want %d:\n  %s", len(got), len(want), hexFrames(got))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("frame %d = % X, want % X", i, got[i], want[i])
		}
	}
}

// TestOpen_ControlLinesAreNeverToggled: on this radio 1A 05 01 33, 01 34
// and 01 35 can assign USB (A)/(B) DTR or RTS as the PTT or keying source,
// so a controller that asserts either at open can key the transmitter of a
// radio whose owner has made that setting. transport.OpenSerial drives both
// low at open under safety obligation 4 and THIS DRIVER TOUCHES NEITHER
// AFTERWARDS.
//
// transport.Port is an io.ReadWriteCloser and carries neither method, so
// the only way to reach one is a type assertion. scriptedPort provides both
// methods and counts the calls precisely so such an assertion, if anyone
// ever writes one, becomes visible here.
func TestOpen_ControlLinesAreNeverToggled(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)
	if _, err := s.ReadChannel(t.Context(), "001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if rts, dtr := p.controlLineCalls(); rts != 0 || dtr != 0 {
		t.Errorf("SetRTS called %d times and SetDTR %d times; both must be zero — register entry ic7760-control-lines records why", rts, dtr)
	}
}

// TestOpen_FingerprintIsRecordOnlyTwentyFive: the fingerprint the probe
// confirms is the RECORD-ONLY length, 25, not the 27-byte data area that
// includes the two selector bytes.
func TestOpen_FingerprintIsRecordOnlyTwentyFive(t *testing.T) {
	s := openWith(t, newScriptedPort(t, occupiedRadio()))
	n, confirmed := s.Fingerprint()
	if !confirmed || n != 25 {
		t.Errorf("Fingerprint() = (%d, %v), want (25, true)", n, confirmed)
	}
}

// TestOpen_WrongRecordLengthIsRefusedWithANamedReason: a radio answering a
// record at a length this profile does not declare fails the open.
//
// THE REFUSAL NAMES NO OTHER MODEL, and that is deliberate: this package
// holds no table of other radios' record lengths, and cross-model
// distinctness is a Wave-4 tier check. It says what was measured, what was
// expected, and that the expectation is itself an ASSUMED derivation.
func TestOpen_WrongRecordLengthIsRefusedWithANamedReason(t *testing.T) {
	for _, n := range []int{24, 26} {
		p := newScriptedPort(t, radioImage{
			idToken: []byte{0x42},
			records: map[int][]byte{1: make([]byte, n)},
		})
		err := openErr(t, p)
		var mismatch *RecordLengthMismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("%d-byte record: err = %v, want *RecordLengthMismatchError", n, err)
		}
		if mismatch.Got != n || mismatch.Want != 25 {
			t.Errorf("%d-byte record: Got/Want = %d/%d, want %d/25", n, mismatch.Got, mismatch.Want, n)
		}
		if n == 24 {
			drivertest.AssertRecordLengthMismatch(t, mismatch, mismatch.Got, mismatch.Want,
				"ic7760: g0/ch1 answered a 24-byte memory record, want 25 — the expected length is itself an ASSUMED derivation from one document (D5 entry 6, register entry ic7760-record-length), and this refusal names no other model because cross-model record-length distinctness is a Wave-4 tier check")
		}
		if !errors.Is(err, driver.ErrWrongRadio) {
			t.Errorf("%d-byte record: err does not satisfy errors.Is(err, driver.ErrWrongRadio)", n)
		}
		if strings.Contains(err.Error(), "IC-76") && !strings.Contains(err.Error(), "ic7760:") {
			t.Errorf("the refusal names another model: %v", err)
		}
		if !strings.Contains(err.Error(), "ic7760-record-length") {
			t.Errorf("the refusal does not cite its register entry: %v", err)
		}
	}
}

// TestOpen_LaterQuarantineFailureFailsClosed is the asymmetry Open's Init
// comment states, executed. A flood addressed to THIS CONTROLLER re-arms
// every drain's idle timer:
//
//   - during Init it is NONFATAL, recorded as InitDrainCapExceeded and
//     stepped over, because a busy bus must not be indistinguishable from
//     a broken radio;
//   - once the session is exchanging frames it is FATAL, because a drain
//     that cannot find quiet means this programme can no longer tell its
//     own answers from somebody else's.
//
// A to=00 BROADCAST flood is the control: the accumulator counts and drops
// those before any engine event, so they never re-arm anything and the
// open is completely ordinary.
func TestOpen_LaterQuarantineFailureFailsClosed(t *testing.T) {
	t.Run("an addressed flood spanning Init alone is non-fatal", func(t *testing.T) {
		p := newScriptedPort(t, occupiedRadio())
		p.startFlood(0xE0)
		// Stop the flood the moment Init is over — the first frame in the
		// transcript is the identity read, which Open cannot have written
		// before Init returned. The flood therefore covers the whole of
		// Init's drain and none of what follows, so the drain reaches its
		// absolute cap deterministically rather than by racing a timer.
		go func() {
			for {
				if len(p.Transcript()) >= 1 {
					p.stopFlood()
					return
				}
				time.Sleep(time.Millisecond)
			}
		}()
		s := openWith(t, p)
		if !s.OpenDiagnostics().InitDrainCapExceeded {
			t.Error("InitDrainCapExceeded is false; the flood outlasted Init's absolute drain cap")
		}
		if !s.OpenDiagnostics().Fingerprinted {
			t.Error("the open did not fingerprint; a busy bus must not fail the open")
		}
	})
	t.Run("a flood beginning after Init fails the open", func(t *testing.T) {
		// The line is quiet while Init drains, so Init succeeds and the
		// failure below is unambiguously a LATER one. The radio then goes
		// silent on memory reads while the flood starts, so the first probe
		// read times out and retries — and a retry's drain-to-quiet is a
		// later quarantine drain, which meets the same absolute cap.
		img := occupiedRadio()
		img.memSilent = true
		p := newScriptedPort(t, img)
		go func() {
			for {
				if len(p.Transcript()) >= 2 {
					p.startFlood(0xE0)
					return
				}
				time.Sleep(2 * time.Millisecond)
			}
		}()
		defer p.stopFlood()

		err := openErr(t, p)
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			t.Errorf("err = %v, want one satisfying errors.Is(err, transport.ErrDrainCapExceeded) — a later drain that cannot find quiet must reach the caller, never be stepped over the way Init's is", err)
		}
	})
	t.Run("a broadcast flood never reaches the engine", func(t *testing.T) {
		p := newScriptedPort(t, occupiedRadio())
		p.startFlood(0x00)
		s := openWith(t, p)
		if s.OpenDiagnostics().InitDrainCapExceeded {
			t.Error("InitDrainCapExceeded is true under a to=00 flood; the accumulator must have dropped those before any engine event")
		}
		if s.OpenDiagnostics().WireAtOpen.Unexpected == 0 {
			t.Error("the accumulator counted no unexpected frames, so the to=00 flood is not visible in the diagnostics at all")
		}
	})
}

func TestProbeScheduleIncludesP1AndP2AfterBoundedMEMSearch(t *testing.T) {
	p := newScriptedPort(t, radioImage{
		idToken: []byte{0x42},
		records: map[int][]byte{101: goldenRecord},
	})
	s := openWith(t, p)
	report := s.OpenDiagnostics()
	if !report.Fingerprinted || report.RecordLength != 25 || report.SlotsTried != 12 {
		t.Fatalf("OpenDiagnostics = %+v, want P2 fingerprint after 12 bounded slots", report)
	}

	want := [][]byte{idReadFrame}
	for ch := 1; ch <= 10; ch++ {
		want = append(want, memReadFrame(ch))
	}
	want = append(want, memReadFrame(100), memReadFrame(101))
	got := p.Transcript()
	if len(got) != len(want) {
		t.Fatalf("probe transcript has %d frames, want %d:\n  %s", len(got), len(want), hexFrames(got))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("probe frame %d = % X, want % X", i, got[i], want[i])
		}
	}
}

func TestProbeScheduleOpensEmptyMEMAndSCANOnAddressEvidence(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0x42}, records: map[int][]byte{}})
	s := openWith(t, p)
	if report := s.OpenDiagnostics(); report.Fingerprinted || report.RecordLength != 0 || report.SlotsTried != 12 {
		t.Fatalf("OpenDiagnostics = %+v, want un-fingerprinted open after all 12 bounded slots", report)
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func openTestSession(t *testing.T, port *respondingPort, opts ...Option) *Session {
	t.Helper()
	sess, err := New(RealHardware, opts...).Open(context.Background(), port.Port(), driver.Identity{Port: "test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session)
}

func TestProbeContinuesFromAddressEvidenceToOccupiedFingerprint(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 2, occupiedRecord(t)))
	s := openTestSession(t, p)
	d := s.CIVDiagnostics()
	if !d.Fingerprinted || d.ProbeSlotsRead != 2 || d.Status != "FINGERPRINTED 111 B" {
		t.Errorf("diagnostics = %+v, want second-slot 111-byte fingerprint", d)
	}
}

func TestProbeRejectsWrongRecordLengthContinuously(t *testing.T) {
	p := newRespondingPort(t, withRecordLength(1, 1, 110))
	_, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	var wrong *driver.WrongRadioError
	if !errors.Is(err, driver.ErrWrongRadio) || !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want unattributed WrongRadioError", err)
	}
	if wrong.Want != "record 111" || wrong.Got != "record 110" || wrong.WantModel != "" || wrong.GotModel != "" {
		t.Fatalf("WrongRadioError = %+v, want record lengths and no unsupported model attribution", wrong)
	}
}

func TestProbeWrongRecordLengthCanAttributeInjectedSiblingProvisionally(t *testing.T) {
	p := newRespondingPort(t, withRecordLength(1, 1, 110))
	_, err := New(RealHardware, WithSiblingRecordLengths(SiblingLengths{110: "Synthetic sibling"})).Open(context.Background(), p.Port(), driver.Identity{})
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want WrongRadioError", err)
	}
	if wrong.WantModel != "IC-7100" || wrong.GotModel != "Synthetic sibling" || !strings.Contains(strings.ToLower(err.Error()), "provisional") {
		t.Fatalf("Open error = %v; WrongRadioError = %+v, want provisional injected attribution", err, wrong)
	}
}

func TestProbeDoesNotFingerprintAnAllFFEmptyRecord(t *testing.T) {
	ff := make([]byte, 111)
	for i := range ff {
		ff[i] = 0xFF
	}
	p := newRespondingPort(t, withRecord(1, 1, ff), withRecord(1, 2, occupiedRecord(t)))
	s := openTestSession(t, p)
	d := s.CIVDiagnostics()
	if !d.Fingerprinted || d.ProbeSlotsRead != 2 {
		t.Errorf("diagnostics = %+v; all-FF A-001 must be empty and A-002 must supply the fingerprint", d)
	}
}

func TestProbeDoesNotFingerprintAnUndecodable111ByteRecord(t *testing.T) {
	corrupt := occupiedRecord(t)
	corrupt[1] = 0xFA // invalid packed BCD in the RX frequency
	p := newRespondingPort(t, withRecord(1, 1, corrupt))
	sess, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open fingerprinted an undecodable 111-byte record")
	}
}

func TestOpenEmptyRadioIsExplicitlyUnfingerprinted(t *testing.T) {
	s := openTestSession(t, newRespondingPort(t))
	d := s.CIVDiagnostics()
	if d.Fingerprinted || d.Status != "UNFINGERPRINTED" || d.ProbeSlotsRead != probeSlots {
		t.Errorf("empty-radio diagnostics = %+v", d)
	}
}

func TestOpenIgnoresIdentityFrameFromAnotherAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	p := newRespondingPort(t, withIDSource(0x89))
	sess, err := New(RealHardware).Open(ctx, p.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open accepted another address's 19 00 reply")
	}
}

func TestNoInitMutation(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	openTestSession(t, p)
	for _, frame := range p.frames() {
		cn, sc, ok := civ.FrameCommand(frame)
		if !ok || !((cn == 0x19 && sc == 0x00 && len(frame) == 7) || (cn == 0x1A && sc == 0x00 && len(frame) == 10)) {
			t.Errorf("Open sent mutation or undocumented grammar: % X", frame)
		}
	}
}

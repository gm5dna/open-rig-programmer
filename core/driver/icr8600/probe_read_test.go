// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func openTestSession(t *testing.T, image testRadioImage, opts ...Option) (*respondingPort, *Session) {
	t.Helper()
	p := newRespondingPort(t, image)
	opts = append(opts, fastTiming())
	sess, err := New(RealHardware, opts...).Open(context.Background(), p.Port(), driver.Identity{Port: "test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return p, sess.(*Session)
}

func TestOpen_ProbeAndDiscoverySendNoMutation(t *testing.T) {
	addr := testWireAddress{0, 0}
	p, s := openTestSession(t, testRadioImage{idToken: []byte{0x01}, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "AM")}})
	if s.Identity().CATID != "96:01" {
		t.Errorf("CATID = %q, want 96:01", s.Identity().CATID)
	}
	for _, frame := range p.Transcript() {
		idRead := len(frame) == 7 && frame[4] == 0x19 && frame[5] == 0x00
		memoryRead := len(frame) == 11 && frame[4] == 0x1A && frame[5] == 0x00
		if !idRead && !memoryRead {
			t.Errorf("Open transmitted a frame outside the admitted read grammars: % X", frame)
		}
	}
	d := s.OpenDiagnostics()
	if !d.Fingerprinted || d.RecordLength != 37 || d.FirstOccupied != "G00-000" {
		t.Errorf("diagnostics = %+v, want first occupied G00-000/37", d)
	}
	if d.SlotsTried > 100*100 {
		t.Errorf("SlotsTried = %d, exceeds the 100x100 bound", d.SlotsTried)
	}
	mem, _ := s.Capabilities().Bank(spec.BankMemory)
	if !slices.Equal(mem.Slots, []string{"G00-000"}) {
		t.Errorf("materialised MEM slots = %v", mem.Slots)
	}
}

func TestOpen_EmptyRadioIsUnfingerprintedAndBounded(t *testing.T) {
	_, s := openTestSession(t, testRadioImage{idToken: []byte{0x55}})
	d := s.OpenDiagnostics()
	if d.Fingerprinted || d.RecordLength != 0 || !strings.Contains(d.AddressDiagnostic, "UNFINGERPRINTED") {
		t.Errorf("diagnostics = %+v, want UNFINGERPRINTED address evidence", d)
	}
	if d.SlotsTried <= 0 || d.SlotsTried > 100*100 {
		t.Errorf("SlotsTried = %d, want a positive value bounded by 100x100", d.SlotsTried)
	}
}

func TestProbe_WrongAddressDoesNotIdentifyTheRadio(t *testing.T) {
	p := newRespondingPort(t, testRadioImage{idToken: []byte{0x01}, idFrom: 0x94})
	_, err := New(RealHardware, fastTiming()).Open(context.Background(), p.Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded on a 19 00 reply from the wrong address")
	}
}

func TestRead_EveryModeLayoutAndFreshD8Fields(t *testing.T) {
	modes := []string{"AM", "D-STAR", "P25", "NXDN-VN", "FM", "DCR", "dPMR"}
	records := map[testWireAddress][]byte{}
	for i, mode := range modes {
		a := testWireAddress{0, i}
		records[a] = testRecord(t, a, mode)
	}
	_, s := openTestSession(t, testRadioImage{idToken: []byte{0x01}, records: records})
	for i, mode := range modes {
		slot := spec.SparseSlot(0, i)
		ch, err := s.ReadChannel(context.Background(), slot)
		if err != nil {
			t.Fatalf("ReadChannel(%s/%s): %v", slot, mode, err)
		}
		if ch.Data == nil || ch.Data.Mode != mode {
			t.Fatalf("ReadChannel(%s) = %+v, want mode %s", slot, ch, mode)
		}
		d := ch.Data
		if d.TxFreqHz.State != codeplug.Unavailable || d.ToneTx.State != codeplug.Unavailable || d.ScanSkip.State != codeplug.Unavailable {
			t.Errorf("%s receive-only/unmapped states = tx %v tone_tx %v scan_skip %v", mode, d.TxFreqHz.State, d.ToneTx.State, d.ScanSkip.State)
		}
		if d.TuningStepEnabled.State != codeplug.Known || d.TuningStep.State != codeplug.Known || d.ProgramTuningStepHz.State != codeplug.Known || d.AttenuatorDB.State != codeplug.Known || d.Preamp.State != codeplug.Known || d.Antenna.State != codeplug.Known || d.IPPlus.State != codeplug.Known {
			t.Errorf("%s did not initialise all seven D8 fields Known: %+v", mode, *d)
		}
		if mode == "FM" {
			if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "TSQL" || d.ToneRx.Value != 885 || d.DTCSCode.Value != 23 || d.DTCSPolarity.Value != "Reverse" {
				t.Errorf("FM tail = %+v", *d)
			}
		} else if d.ToneMode.State != codeplug.Unavailable || d.ToneRx.State != codeplug.Unavailable || d.DTCSCode.State != codeplug.Unavailable || d.DTCSPolarity.State != codeplug.Unavailable {
			t.Errorf("%s digital/non-FM tail fields were not Unavailable", mode)
		}
	}
}

func TestRead_ContinuousFingerprintAndAddressChecks(t *testing.T) {
	addr := testWireAddress{0, 0}
	p, s := openTestSession(t, testRadioImage{idToken: []byte{0x01}, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "AM")}})

	t.Run("undeclared mode", func(t *testing.T) {
		r := testRecord(t, addr, "AM")
		r[6] = 0x99
		p.setRecord(addr, r)
		_, err := s.ReadChannel(context.Background(), "G00-000")
		if err == nil || !strings.Contains(err.Error(), "undeclared mode") {
			t.Errorf("error = %v, want undeclared mode refusal", err)
		}
	})

	t.Run("mode length mismatch", func(t *testing.T) {
		r := testRecord(t, addr, "FM")[:37]
		p.setRecord(addr, r)
		_, err := s.ReadChannel(context.Background(), "G00-000")
		var lengthErr *civ.RecordLengthError
		if !errors.As(err, &lengthErr) || lengthErr.Mode != "FM" {
			t.Errorf("error = %v, want FM *civ.RecordLengthError", err)
		}
	})

	t.Run("undeclared length", func(t *testing.T) {
		p.setRecord(addr, make([]byte, 38))
		_, err := s.ReadChannel(context.Background(), "G00-000")
		var lengthErr *civ.RecordLengthError
		if !errors.As(err, &lengthErr) {
			t.Errorf("error = %v, want continuous *civ.RecordLengthError", err)
		}
	})

	t.Run("wrong answer address", func(t *testing.T) {
		p.setRecord(addr, testRecord(t, addr, "AM"))
		p.misdirect(addr, testWireAddress{0, 1})
		_, err := s.ReadChannel(context.Background(), "G00-000")
		if !errors.Is(err, ErrAnswerMismatch) {
			t.Errorf("error = %v, want ErrAnswerMismatch", err)
		}
	})
}

func TestRead_AllFFIsEmptyBeforeModeSelection(t *testing.T) {
	addr := testWireAddress{0, 0}
	_, s := openTestSession(t, testRadioImage{idToken: []byte{0x01}, records: map[testWireAddress][]byte{addr: bytesOf(0xFF, 44)}})
	ch, err := s.ReadChannel(context.Background(), "G00-000")
	if err != nil || !ch.Empty() {
		t.Fatalf("ReadChannel(all FF) = %+v, %v; want empty before mode selection", ch, err)
	}
}

func TestFraming_StopBitsAndByteIdentityEcho(t *testing.T) {
	drv := New(RealHardware)
	r, ok := drv.(interface{ StopBits() int })
	if !ok || r.StopBits() != 1 {
		t.Fatalf("SerialFramingReporter = %T/%v, want 1 stop bit (register icr8600-serial-framing)", drv, ok)
	}
	addr := testWireAddress{0, 0}
	_, exact := openTestSession(t, testRadioImage{idToken: []byte{0x01}, echo: true, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "AM")}})
	if exact.WireStats().Echoes == 0 {
		t.Error("byte-identical outbound echoes were not suppressed")
	}
	_, falseMatch := openTestSession(t, testRadioImage{idToken: []byte{0x01}, falseEcho: true, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "AM")}})
	stats := falseMatch.WireStats()
	if stats.Unexpected == 0 || stats.Echoes != 0 {
		t.Errorf("non-identical same-position traffic stats = %+v; want unexpected and zero echoes", stats)
	}
}

func bytesOf(value byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = value
	}
	return b
}

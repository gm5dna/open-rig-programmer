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

func TestRead_ValuesOutsideTheDeclaredDomainsReadUnknownNotKnown(t *testing.T) {
	// THE CIV SPANS ARE WIDER THAN THE DECLARED CAPABILITY DOMAINS.
	// Attenuator is any BCD 00-99 against AttenuatorDB{0,10,20,30}; DTCS
	// is any BCD 000-999 against the 512 octal-shaped codes; the
	// programmable step admits 0000 against an ASSUMED MinHz of 100
	// (matrix 1b.2 says the document sets no floor and does not say
	// whether 0000 is admissible).
	//
	// A radio that answers such a value is not lying and the read must
	// not fail — but calling it Known fabricates a codeplug that
	// codeplug.Validate then refuses to save. Unknown is the honest
	// state, and it is already what ToneRx does two lines away.
	addr := testWireAddress{0, 0}
	record := testRecord(t, addr, "FM")
	record[15], record[16] = 0x00, 0x00 // programmable step 0 Hz, below the ASSUMED floor
	record[17] = 0x55                   // 55 dB, not one of the four printed steps
	record[42], record[43] = 0x08, 0x88 // DTCS 888, whose digits are not octal

	_, s := openTestSession(t, testRadioImage{idToken: []byte{0x01}, records: map[testWireAddress][]byte{addr: record}})
	ch, err := s.ReadChannel(context.Background(), "G00-000")
	if err != nil || ch.Data == nil {
		t.Fatalf("ReadChannel = %+v, %v; a value outside a declared domain must not fail the read", ch, err)
	}
	if got := ch.Data.ProgramTuningStepHz.State; got != codeplug.Unknown {
		t.Errorf("ProgramTuningStepHz state = %v (value %d), want Unknown", got, ch.Data.ProgramTuningStepHz.Value)
	}
	if got := ch.Data.AttenuatorDB.State; got != codeplug.Unknown {
		t.Errorf("AttenuatorDB state = %v (value %d), want Unknown", got, ch.Data.AttenuatorDB.Value)
	}
	if got := ch.Data.DTCSCode.State; got != codeplug.Unknown {
		t.Errorf("DTCSCode state = %v (value %d), want Unknown", got, ch.Data.DTCSCode.Value)
	}

	// The point of Unknown: what came back is saveable.
	cp := &codeplug.Codeplug{
		Radio:    codeplug.RadioInfo{Model: s.Capabilities().Model, CATID: s.Capabilities().CATID},
		Channels: []codeplug.Channel{ch},
	}
	for _, issue := range codeplug.Validate(cp, s.Capabilities()) {
		switch issue.Field {
		case spec.FieldProgramTuningStep, spec.FieldAttenuator, spec.FieldDTCSCode:
			t.Errorf("a read the radio answered is unsaveable: %s: %s", issue.Field, issue.Msg)
		}
	}

	// An in-domain record still reads Known, so the guard is a domain
	// check and not a blanket downgrade.
	inDomain := testRecord(t, addr, "FM")
	_, s2 := openTestSession(t, testRadioImage{idToken: []byte{0x01}, records: map[testWireAddress][]byte{addr: inDomain}})
	ok, err := s2.ReadChannel(context.Background(), "G00-000")
	if err != nil || ok.Data == nil {
		t.Fatalf("ReadChannel(in-domain) = %+v, %v", ok, err)
	}
	if ok.Data.ProgramTuningStepHz.State != codeplug.Known || ok.Data.AttenuatorDB.State != codeplug.Known || ok.Data.DTCSCode.State != codeplug.Known {
		t.Errorf("in-domain read = step %v atten %v dtcs %v, want all Known", ok.Data.ProgramTuningStepHz.State, ok.Data.AttenuatorDB.State, ok.Data.DTCSCode.State)
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

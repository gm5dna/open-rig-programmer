// SPDX-License-Identifier: GPL-3.0-or-later

package ts870s

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts870s "github.com/gm5dna/open-rig-programmer/core/kw/ts870s"
)

// mrAnswer builds the raw 22-byte MR ANSWER string for channel, the way a
// scripted radio would print it — reusing kwts870s.Layout's own codec so a
// test fixture can never disagree with the layout Open actually wires up.
func mrAnswer(t *testing.T, rec kw.Record870) string {
	t.Helper()
	cmd, err := kwts870s.Layout.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := cmd.Bytes()
	frame[0], frame[1] = 'M', 'R'
	return string(frame)
}

func TestOpen_Success(t *testing.T) {
	sess, port := openSession(t, RealHardware, radioImage{})
	if sess.Identity().CATID != catID {
		t.Errorf("Identity().CATID = %q, want %q", sess.Identity().CATID, catID)
	}
	got := port.Transcript()
	if len(got) != 2 || got[0] != "AI0;" || got[1] != "ID;" {
		t.Errorf("Open sent %v, want exactly [AI0; ID;] — a wrong radio must see no more than the preamble and the probe", got)
	}
}

func TestOpen_WrongRadio(t *testing.T) {
	_, err := New(RealHardware).Open(context.Background(), newRespondingPort(t, radioImage{catID: "020"}).Port(), driver.Identity{})
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want a *driver.WrongRadioError", err)
	}
	if wrong.Got != "020" || wrong.GotModel != "TS-480" {
		t.Errorf("WrongRadioError = %+v, want Got 020, GotModel TS-480", wrong)
	}
}

func TestOpen_IDSilence_Refuses(t *testing.T) {
	_, err := New(RealHardware).Open(context.Background(), newRespondingPort(t, radioImage{idSilent: true}).Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded against a radio that never answered ID;")
	}
}

func TestReadChannel_Populated(t *testing.T) {
	rec := kw.Record870{Channel: 7, FreqHz: 14_250_000, Mode: kw.ModeUSB, Lockout: '1', ToneMode: kw.ToneModeTone, ToneIndex: 1}
	sess, _ := openSession(t, RealHardware, radioImage{mrAnswers: map[string]string{"07": mrAnswer(t, rec)}})

	ch, err := sess.ReadChannel(context.Background(), "07")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Empty() {
		t.Fatal("channel 07 read back empty")
	}
	if ch.Data.FreqHz != 14_250_000 || ch.Data.Mode != "USB" {
		t.Errorf("Data = %+v, want FreqHz 14250000, Mode USB", ch.Data)
	}
	if !ch.Data.ScanSkip.Value || ch.Data.ScanSkip.State != codeplug.Known {
		t.Errorf("ScanSkip = %+v, want Known true", ch.Data.ScanSkip)
	}
	if ch.Data.ToneMode.Value != "TONE" || ch.Data.ToneTx.State != codeplug.Known {
		t.Errorf("ToneMode/ToneTx = %+v/%+v", ch.Data.ToneMode, ch.Data.ToneTx)
	}
	if ch.Data.ToneTx.Value != 670 {
		t.Errorf("ToneTx.Value = %v, want the chart's first entry (670 decihertz, matrix §1.9)", ch.Data.ToneTx.Value)
	}
	if ch.Data.TxFreqHz.State != codeplug.Unavailable {
		t.Errorf("TxFreqHz.State = %v, want Unavailable (package doc comment)", ch.Data.TxFreqHz.State)
	}
}

func TestReadChannel_Vacant(t *testing.T) {
	// The manual's own vacant-channel sentence: "the Answer command sends
	// '0' for all parameters except the memory channel number"
	// (ts870s:9101-9104). Built from a real MW frame with P4-P8 zeroed,
	// the same shape core/kw/ts870s's own vacant-channel test uses,
	// rather than hand-counted byte positions here.
	full := mrAnswer(t, kw.Record870{Channel: 0, FreqHz: 14_250_000, Mode: kw.ModeLSB, Lockout: '0', ToneMode: kw.ToneModeTone, ToneIndex: 1})
	vacant := []byte(full)
	for i := 5; i <= 20; i++ {
		vacant[i] = '0'
	}
	sess, _ := openSession(t, RealHardware, radioImage{mrAnswers: map[string]string{"00": string(vacant)}})

	ch, err := sess.ReadChannel(context.Background(), "00")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if !ch.Empty() {
		t.Errorf("channel 00 read back populated: %+v", ch.Data)
	}
}

func TestReadChannel_UnknownSlot(t *testing.T) {
	sess, _ := openSession(t, RealHardware, radioImage{})
	if _, err := sess.ReadChannel(context.Background(), "100"); err == nil {
		t.Error("ReadChannel(\"100\") succeeded, want a refusal (not two digits)")
	}
}

// populatedData is a fully Known channel this row's write path admits, once
// the capability gate is consented past.
func populatedData() codeplug.ChannelData {
	return codeplug.ChannelData{
		FreqHz:   7_100_000,
		Mode:     "LSB",
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "TONE"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 670},
		TxFreqHz: codeplug.FreqField{State: codeplug.Unknown},
	}
}

func TestWriteChannel_Success(t *testing.T) {
	sess, port := openSession(t, Simulated, radioImage{}, WithConsentedUnverifiedWrites())
	res, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "03", Data: ptr(populatedData())})
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MW" || !res.Steps[0].Sent {
		t.Fatalf("WriteResult = %+v, want one sent MW step", res)
	}
	got := port.Transcript()
	if len(got) != 3 || got[2][:2] != "MW" {
		t.Errorf("transcript = %v, want [AI0; ID; MW...]", got)
	}
}

func TestWriteChannel_RefusesEmpty(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{}, WithConsentedUnverifiedWrites())
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "03"})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel(empty): %v, want ErrWriteRefused", err)
	}
}

func TestWriteChannel_RefusesKnownTxFrequency(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{}, WithConsentedUnverifiedWrites())
	data := populatedData()
	data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 7_150_000}
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "03", Data: &data})
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) || len(refused.Fields) != 1 || refused.Fields[0] != "tx_frequency" {
		t.Fatalf("WriteChannel(Known tx_frequency) = %v, want a WriteRefusedError naming tx_frequency", err)
	}
}

func TestWriteChannel_RefusesWithoutConsent(t *testing.T) {
	sess, _ := openSession(t, RealHardware, radioImage{})
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "03", Data: ptr(populatedData())})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel on an unconsented RealHardware session: %v, want ErrWriteRefused", err)
	}
}

func TestWriteChannel_RefusesUnknownToneTx(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{}, WithConsentedUnverifiedWrites())
	data := populatedData()
	data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "03", Data: &data})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel(Unknown tone_tx while TONE is requested): %v, want ErrWriteRefused", err)
	}
}

func ptr(d codeplug.ChannelData) *codeplug.ChannelData { return &d }

// tierFieldStates is internal/wiring's own wiringTierFieldStates, restated
// here so this package's test can hold ReadChannel to the SAME invariant
// without importing a test-only helper from another package: the
// seventeen codeplug FieldState members the Icom-tier D4/D8 extension
// added (spec.AllFields() minus the ten pre-tier ones), each of which a
// fresh read must state Known, Unknown or Unavailable — never left at
// its Go zero value, Absent.
func tierFieldStates(d *codeplug.ChannelData) map[string]codeplug.FieldState {
	return map[string]codeplug.FieldState{
		"tx_frequency":        d.TxFreqHz.State,
		"duplex":              d.Duplex.State,
		"offset":              d.OffsetHz.State,
		"tone_mode":           d.ToneMode.State,
		"tone_tx":             d.ToneTx.State,
		"tone_rx":             d.ToneRx.State,
		"dtcs_code":           d.DTCSCode.State,
		"dtcs_polarity":       d.DTCSPolarity.State,
		"filter":              d.Filter.State,
		"data_mode":           d.DataMode.State,
		"tuning_step_enabled": d.TuningStepEnabled.State,
		"tuning_step":         d.TuningStep.State,
		"program_tuning_step": d.ProgramTuningStepHz.State,
		"attenuator":          d.AttenuatorDB.State,
		"preamp":              d.Preamp.State,
		"antenna":             d.Antenna.State,
		"ip_plus":             d.IPPlus.State,
		// Pre-tier fields this row also carries a FieldState for, held to
		// the same rule: TagDisplay (NoTag) and CTCSSTone (this row's
		// tone axis is tone_tx/tone_mode, not the Yaesu ctcss_tone pair).
		"tag_display": d.TagDisplay.State,
		"ctcss_tone":  d.CTCSSTone.State,
	}
}

// TestReadChannel_NoTierFieldIsLeftAbsent is the wiring-level read
// invariant (internal/wiring's
// TestOpenFakeSessionFor_EveryRegisteredModel_ReadsEveryDefaultSlot)
// restated at package level: "ReadChannel(...) left <field> Absent; a
// fresh read must state Known, Unknown or Unavailable before Save
// chooses a schema" is exactly the failure this pins against, for both
// a channel with tone ON (TestReadChannel_Populated's own fixture) and
// one with tone OFF (a second populated channel, so tone_tx's own
// Unavailable-when-OFF arm is covered too).
func TestReadChannel_NoTierFieldIsLeftAbsent(t *testing.T) {
	toneOn := kw.Record870{Channel: 1, FreqHz: 14_250_000, Mode: kw.ModeUSB, Lockout: '0', ToneMode: kw.ToneModeTone, ToneIndex: 1}
	toneOff := kw.Record870{Channel: 2, FreqHz: 7_100_000, Mode: kw.ModeLSB, Lockout: '1', ToneMode: kw.ToneModeOff, ToneIndex: 1}
	sess, _ := openSession(t, RealHardware, radioImage{mrAnswers: map[string]string{
		"01": mrAnswer(t, toneOn),
		"02": mrAnswer(t, toneOff),
	}})

	for _, slot := range []string{"01", "02"} {
		ch, err := sess.ReadChannel(context.Background(), slot)
		if err != nil {
			t.Fatalf("ReadChannel(%q): %v", slot, err)
		}
		if ch.Empty() {
			t.Fatalf("ReadChannel(%q) came back empty", slot)
		}
		for field, state := range tierFieldStates(ch.Data) {
			if state == codeplug.Absent {
				t.Errorf("ReadChannel(%q) left %s Absent; a fresh read must state Known, Unknown or Unavailable before Save chooses a schema", slot, field)
			}
		}
	}
}

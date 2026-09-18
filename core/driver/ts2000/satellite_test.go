// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakets2000"
)

// openFakeSession opens NewTS2000 (Simulated profile, consented writes)
// against a real internal/fakets2000.Radio — unlike this package's own
// respondingPort/radioImage (a hand-scripted responder that predates
// SA/SI), the fake is the shared, independently-written simulator, and
// is what actually exercises the round trip through core/kw/ts2000's
// codec AND the fake's own SA/SI parser on both ends of one wire.
func openFakeSession(t *testing.T, opts ...fakets2000.Option) (*Session, *fakets2000.Radio) {
	t.Helper()
	r := fakets2000.New(opts...)
	t.Cleanup(func() { _ = r.Close() })

	d := NewTS2000(testTiming(), WithSimulatedProfile())
	sess, err := d.Open(context.Background(), r.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), r
}

// TestReadChannel_Satellite_MatchesCurrentlySelected pins the read
// limitation core/kw/ts2000/satellite.go's own doc comment states: "SA;"
// has no per-channel address, so ReadChannel(id) succeeds only when id
// is the channel the fake currently has selected (0, at construction).
func TestReadChannel_Satellite_MatchesCurrentlySelected(t *testing.T) {
	sess, _ := openFakeSession(t)

	ch, err := sess.ReadChannel(context.Background(), "0")
	if err != nil {
		t.Fatalf("ReadChannel(\"0\"): %v", err)
	}
	if ch.Empty() {
		t.Fatal("ReadChannel(\"0\") returned an empty channel")
	}
	if got := ch.Data.Tag; got != "        " {
		t.Errorf("Tag = %q, want the factory eight-space name", got)
	}
	for _, tt := range []struct {
		name string
		got  codeplug.BoolField
	}{
		{"sat_band_swap", ch.Data.SatBandSwap},
		{"sat_trace", ch.Data.SatTrace},
		{"sat_trace_rev", ch.Data.SatTraceRev},
	} {
		if tt.got.State != codeplug.Known || tt.got.Value != false {
			t.Errorf("%s = %+v, want {Known false} — the factory image is all-OFF", tt.name, tt.got)
		}
	}
}

// TestReadChannel_Satellite_OtherChannelIsAnswerMismatch pins the other
// half: a channel the fake is NOT currently on is refused, honestly,
// rather than silently returning the wrong channel's data.
func TestReadChannel_Satellite_OtherChannelIsAnswerMismatch(t *testing.T) {
	sess, _ := openFakeSession(t)

	_, err := sess.ReadChannel(context.Background(), "3")
	if !errors.Is(err, driver.ErrAnswerMismatch) {
		t.Fatalf("ReadChannel(\"3\"): %v, want errors.Is(_, driver.ErrAnswerMismatch) (the fake defaults to channel 0 selected)", err)
	}
}

// satelliteWriteData builds a populated satellite channel with every
// field this bank reaches Known — the shape WriteChannel requires.
func satelliteWriteData() codeplug.ChannelData {
	return codeplug.ChannelData{
		Tag:                 "SO-50",
		TagDisplay:          codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:            codeplug.BoolField{State: codeplug.Unavailable},
		TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:              codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
		ToneMode:            codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
		SatBandSwap:         codeplug.BoolField{State: codeplug.Known, Value: true},
		SatTrace:            codeplug.BoolField{State: codeplug.Known, Value: true},
		SatTraceRev:         codeplug.BoolField{State: codeplug.Known, Value: false},
	}
}

// TestWriteChannel_Satellite_SendsSAThenSI writes channel 5's flags and
// name and checks BOTH frames landed on the fake — the SA/SI order the
// brief specifies, and the read-back proof that BOTH commands actually
// changed the fake's stored state (not merely "no rejection").
func TestWriteChannel_Satellite_SendsSAThenSI(t *testing.T) {
	sess, r := openFakeSession(t)

	data := satelliteWriteData()
	res, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "5", Data: &data})
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 2 || res.Steps[0].Command != "SA" || res.Steps[1].Command != "SI" {
		t.Fatalf("Steps = %+v, want [{SA ...} {SI ...}]", res.Steps)
	}
	if !res.Steps[0].Sent || !res.Steps[1].Sent {
		t.Errorf("Steps = %+v, want both Sent", res.Steps)
	}

	swap, trace, traceRev, name := r.SatelliteChannel(5)
	if !swap || !trace || traceRev {
		t.Errorf("SatelliteChannel(5) flags = swap=%v trace=%v traceRev=%v, want true/true/false", swap, trace, traceRev)
	}
	if name != "SO-50   " {
		t.Errorf("SatelliteChannel(5) name = %q, want \"SO-50   \" (space-padded to eight)", name)
	}
	if _, channel, _, _ := r.SatelliteSelected(); channel != 5 {
		t.Errorf("SatelliteSelected() channel = %d, want 5 — the SA Set selects the channel it addresses", channel)
	}
}

// TestWriteChannel_Satellite_PreservesLiveState pins the pre-write-read
// preservation core/driver/ts2000/satellite.go's own doc comment
// promises: this bank's write.go has no per-channel Field for P1
// (satellite mode)/P4 (CTRL)/P7 (MULTI/CH mode) at all, so writing
// channel 5's flags must thread the fake's own current live values for
// those three positions straight through — never invent, never zero.
//
// The fake is seeded directly via WithSatelliteLiveState — NOT by a prior
// driver write, which would only prove the driver echoes back what it
// itself just sent — to a MIXED, non-default combination (mode ON, CTRL
// still main, MULTI/CH ON): a driver that hardcoded any single fixed
// value (all false, all true, or any other constant triple) fails this,
// because gotMode/gotCtrl/gotMulti would then disagree with at least one
// of the three independently-seeded values.
func TestWriteChannel_Satellite_PreservesLiveState(t *testing.T) {
	const wantMode, wantCtrl, wantMulti = true, false, true
	sess, r := openFakeSession(t, fakets2000.WithSatelliteLiveState(wantMode, wantCtrl, wantMulti))

	data := satelliteWriteData()
	if _, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "5", Data: &data}); err != nil {
		t.Fatalf("WriteChannel(5): %v", err)
	}

	gotMode, channel, gotCtrl, gotMulti := r.SatelliteSelected()
	if channel != 5 {
		t.Fatalf("SatelliteSelected() channel = %d, want 5", channel)
	}
	if gotMode != wantMode || gotCtrl != wantCtrl || gotMulti != wantMulti {
		t.Errorf("live state after writing channel 5 = (mode=%v ctrl=%v multi=%v), want the seeded state (mode=%v ctrl=%v multi=%v) — WriteChannel must preserve it, not invent it",
			gotMode, gotCtrl, gotMulti, wantMode, wantCtrl, wantMulti)
	}
}

// TestWriteChannel_Satellite_RefusesUnknownFlag pins the mandatory-Known
// gate: SA always transmits P3/P5/P6, with no "leave it alone" encoding,
// so a channel that has not answered one of the three per-channel flags
// must be refused before any wire traffic — the write.go candidate()
// precedent, restated for this bank.
func TestWriteChannel_Satellite_RefusesUnknownFlag(t *testing.T) {
	sess, _ := openFakeSession(t)

	data := satelliteWriteData()
	data.SatTrace = codeplug.BoolField{}
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "0", Data: &data})
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel with sat_trace Absent: %v, want a *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldSatTrace {
		t.Errorf("WriteRefusedError.Fields = %v, want [sat_trace]", refused.Fields)
	}
}

// TestWriteChannel_Satellite_RefusesEmptyChannel pins the no-erase rule
// for this bank: no MR/MW-reachable erase route exists for it either.
func TestWriteChannel_Satellite_RefusesEmptyChannel(t *testing.T) {
	sess, _ := openFakeSession(t)

	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "0"})
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel(empty): %v, want a *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldErase {
		t.Errorf("WriteRefusedError.Fields = %v, want [erase]", refused.Fields)
	}
}

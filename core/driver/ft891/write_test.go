// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// writableChannel is a valid, unremarkable channel this driver can write:
// 14.250 MHz USB, clarifier -150 Hz with the RX clarifier on, CTCSS ENC-DEC,
// PLUS shift, tag "CALLING", TAG display ON.
//
// It is the WRITE-direction twin of respondingport_test.go's populatedFields
// and carries the same values on purpose, so the byte-level expectations
// below can be compared against that helper's read-direction ones by eye.
// Every field is stated: a write of this radio's combined form carries the
// whole record whether or not a value changed, so a channel with an
// unstated field would be testing a default rather than a decision.
//
// THE SEVENTEEN ICOM-TIER FIELDS ARE tierUnavailable'D, not left at their
// zero value, and the reason has CHANGED without the fixture changing
// (write-gate sweep item (i), 05/09/2026). Under closing-review wave 2's
// own fieldStateChecks a zero FieldState — codeplug.Absent — was REFUSED
// outright, so the fixture had to state them or the write rung would report
// "invalid State" for a field nobody meant to say anything about. The fleet
// walk this driver now calls ADMITS Absent, so the fixture no longer needs
// to; it stays as it is because tierUnavailable is the SAME production shape
// read.go's channelData actually produces (plan P12: every FT-891 read sets
// all seventeen Unavailable), which is what makes this an honest "otherwise
// writable" channel. The Absent case has its own test now —
// TestWriteChannel_AbsentFieldStatesStillWrite — rather than being something
// every fixture has to dodge.
func writableChannel() codeplug.Channel {
	d := tierUnavailable(codeplug.ChannelData{
		FreqHz:     14_250_000,
		Mode:       "USB",
		ClarHz:     -150,
		RxClar:     true,
		TxClar:     false,
		CTCSS:      "ENC-DEC",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
		Shift:      "PLUS",
		Tag:        "CALLING",
		TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
	})
	return codeplug.Channel{Slot: "001", Data: &d}
}

// writableChannelFrame is the 41-byte combined MT Set writableChannel()
// produces, re-derived by hand: see TestWriteChannel_OneCombinedMTSetFrame
// for the position-by-position derivation it is checked against.
//
//	MT|001|014250000|-|0150|1|0|2|0|1|00|1|1|CALLING_____|;
const writableChannelFrame = "MT001014250000-0150102010011CALLING     ;"

// withData returns writableChannel() with mutate applied to its data — the
// one-field-wrong shape most of the ladder's rungs need.
func withData(mutate func(*codeplug.ChannelData)) codeplug.Channel {
	ch := writableChannel()
	mutate(ch.Data)
	return ch
}

// TestWriteChannel_OneCombinedMTSetFrame pins the BYTES, position by
// position, for both states of this radio's live TAG display flag.
//
// THE EXPECTED FRAMES ARE RE-DERIVED BY HAND from rev 1909-C's own MT
// position chart (layout 996-1027), field by field, and are deliberately
// NOT built through cat.Dialect.BuildMTSetCombinedDisplay — a fixture
// produced by the builder under test would agree with a wrong offset as
// happily as a right one. The 41 positions are:
//
//	1-2    "MT"
//	3-5    P1  slot
//	6-14   P2  frequency, nine digits
//	15     P3  clarifier direction ('+' or '-')
//	16-19  P3  clarifier magnitude, four digits
//	20     P4  RX clarifier flag
//	21     P5  "(Fixed)" — always '0' on this radio (matrix §2.2)
//	22     P6  mode nibble
//	23     P7  "(Fixed)" — the combined Set's form constant (layout 1011)
//	24     P8  CTCSS state
//	25-26  P9  "00", documented fixed (layout 1013)
//	27     P10 shift
//	28     P11 the TAG display flag — '0' OFF, '1' ON (layout 1016)
//	29-40  P12 the twelve-byte tag field, fill-padded (layout 1017)
//	41     ';'
//
// ONE frame per channel and no MW: the 41-byte Set carries the whole field
// block, the display flag and the tag together, where MW's 28 bytes could
// carry neither the tag nor the flag (layout 1034-1042). That one Set
// SUFFICES to create or overwrite a channel is the driver register's A
// SINGLE COMBINED MT SET SUFFICES TO CREATE OR OVERWRITE A CHANNEL entry,
// and that its silence means acceptance is that entry's second half (also
// stated in its own right as THE ACKNOWLEDGEMENT CONVENTIONS).
func TestWriteChannel_OneCombinedMTSetFrame(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ch    codeplug.Channel
		frame string
	}{
		{
			name:  "MEM, TAG display ON",
			ch:    writableChannel(),
			frame: writableChannelFrame,
		},
		{
			// The other end of every vocabulary AND the other bank: a PMS
			// upper limit, the manual's last mode nibble 'D' (AM-N), the
			// clarifier at its declared maximum with the opposite sign,
			// CTCSS off, MINUS shift, a two-character tag whose 12-byte
			// wire field is fill-padded, and the TAG display flag OFF.
			name: "PMS, TAG display OFF",
			ch: func() codeplug.Channel {
				d := tierUnavailable(codeplug.ChannelData{
					FreqHz:     30_000,
					Mode:       "AM-N",
					ClarHz:     9990,
					RxClar:     false,
					TxClar:     false,
					CTCSS:      "OFF",
					CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
					Shift:      "MINUS",
					Tag:        "AB",
					TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
					ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
				})
				return codeplug.Channel{Slot: "P9U", Data: &d}
			}(),
			//   MT|P9U|000030000|+|9990|0|0|D|0|0|00|2|0|AB__________|;
			frame: "MTP9U000030000+999000D000020AB          ;",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, sess := openSession(t, Simulated, slotImage{})
			before := len(p.Transcript())

			res, err := sess.WriteChannel(testCtx(t), tt.ch)
			if err != nil {
				t.Fatalf("WriteChannel = %v, want nil", err)
			}
			if len(tt.frame) != 41 {
				t.Fatalf("the hand-derived expectation is %d bytes, not the chart's 41 — fix the test, not the driver", len(tt.frame))
			}
			if got, want := p.Transcript()[before:], []string{tt.frame}; !reflect.DeepEqual(got, want) {
				t.Errorf("wire carried %q\nwant             %q", got, want)
			}
			want := []driver.WriteStep{{Command: "MT", Sent: true, Confirmed: true}}
			if !reflect.DeepEqual(res.Steps, want) {
				t.Errorf("WriteResult.Steps = %+v, want %+v", res.Steps, want)
			}
		})
	}
}

// TestWriteChannel_AbsentFieldStatesStillWrite is the fleet FieldState
// stance's Absent rule (write-gate sweep item (i), 05/09/2026), and it is a
// DELIBERATE RELAXATION of what closing-review wave 2 shipped here: this
// driver's own fieldStateChecks called Valid() on all twenty FieldState
// fields unconditionally, and codeplug's typed validators reject
// codeplug.Absent outright, so a hand-built ChannelData that simply left the
// seventeen tier fields alone was REFUSED. The fleet stance — the IC-9700's
// (core/driver/ic9700/write.go's validateKnownValues), which the sweep
// adopted for all four Yaesu drivers — admits Absent: a caller who set
// nothing has requested nothing, and refusing those would refuse every
// ordinary MODIFY.
//
// The C-M1 half is UNCHANGED and is still pinned, by this suite's ladder row
// "an incoherent TxFreqHz field is refused, not interpreted (C-M1)": a
// non-Known state carrying a value is still refused.
//
// TAGDISPLAY IS THE ONE EXCEPTION on THIS radio, and it is not this rung's
// doing: byte 28 is mandatory on the frame, so a non-Known TagDisplay —
// Absent included — is refused by buildWriteCommand however it got that way,
// which the ladder's "a non-Known TagDisplay would manufacture byte 28" row
// pins separately.
//
// RED-PROOF (against the wave-2 fieldStateChecks, before the rewrite onto
// driver.CheckFieldStates), this test's own body unchanged:
//
//	WriteChannel = driver: write to slot "001" refused (ctcss_tone): codeplug: ToneField: invalid State ""
func TestWriteChannel_AbsentFieldStatesStillWrite(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	// EVERY FieldState field left at its zero value except TagDisplay, and
	// the plain fields set to writableChannel's own values, so the frame
	// below is comparable with writableChannelFrame byte for byte.
	d := codeplug.ChannelData{
		FreqHz:     14_250_000,
		Mode:       "USB",
		ClarHz:     -150,
		RxClar:     true,
		TxClar:     false,
		CTCSS:      "ENC-DEC",
		Shift:      "PLUS",
		Tag:        "CALLING",
		TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
	}
	ch := codeplug.Channel{Slot: "001", Data: &d}
	for _, c := range driver.FieldStateChecks(CapabilitiesSimulated(), d) {
		if c.Field == spec.FieldTagDisplay {
			continue
		}
		if c.Err != nil {
			t.Fatalf("the fixture's %s is not Absent-and-admitted (%v) — this test asserts nothing unless every other FieldState field is left at its zero value", c.Field, c.Err)
		}
	}

	before := len(p.Transcript())
	res, err := sess.WriteChannel(testCtx(t), ch)
	if err != nil {
		t.Fatalf("WriteChannel = %v, want nil — an all-Absent channel requests nothing and must WRITE", err)
	}
	if got, want := p.Transcript()[before:], []string{writableChannelFrame}; !reflect.DeepEqual(got, want) {
		// The fields writableChannel states and this one leaves Absent
		// (CTCSSTone, ScanSkip and the seventeen) have no position in the
		// 41-byte record, so an Absent state cannot move a byte — a frame
		// that differed would mean the walk had let a non-Known field reach
		// the wire.
		t.Errorf("wire carried %q\nwant             %q", got, want)
	}
	if want := []driver.WriteStep{{Command: "MT", Sent: true, Confirmed: true}}; !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("WriteResult.Steps = %+v, want %+v", res.Steps, want)
	}
}

// TestWriteChannel_NoVerifyInsideWriteChannel: the write is ONE frame and
// the driver never reads the slot back.
//
// Plan P3 and matrix M-E2 assign write-then-verify to core/clone, which
// holds both sides of the comparison and the policy for what to do about a
// mismatch. A driver that read back for itself would either duplicate that
// policy or (worse) quietly diverge from it, and it would double the frames
// every send costs.
func TestWriteChannel_NoVerifyInsideWriteChannel(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})
	before := len(p.Transcript())

	if _, err := sess.WriteChannel(testCtx(t), writableChannel()); err != nil {
		t.Fatalf("WriteChannel = %v, want nil", err)
	}
	got := p.Transcript()[before:]
	if len(got) != 1 {
		t.Fatalf("WriteChannel sent %v — want exactly ONE frame; a read-back here belongs to core/clone (P3)", got)
	}
	if strings.HasPrefix(got[0], "MT001;") || len(got[0]) == mtReadFrameLen {
		t.Errorf("WriteChannel sent %q, which is an MT READ — no verify happens inside the driver", got[0])
	}
}

// TestWriteChannel_RefusalLadder walks plan P7's ladder, rung by rung, on
// the Simulated profile — the profile matrix M-E3 says the SEMANTIC
// refusals are pinned on, because on unconsented RealHardware the
// capability gate answers first (see the next test).
//
// Every rung must refuse BEFORE any byte reaches the wire, with a typed
// *driver.WriteRefusedError naming the field responsible where there is
// one, and with an explicitly EMPTY (never nil) step list.
func TestWriteChannel_RefusalLadder(t *testing.T) {
	for _, tt := range []struct {
		name   string
		ch     codeplug.Channel
		fields []spec.Field
		reason string // a substring the refusal must carry
	}{
		{
			name:   "a slot this dialect does not define",
			ch:     codeplug.Channel{Slot: "0X1", Data: writableChannel().Data},
			reason: "not a valid slot",
		},
		{
			// "000" parses — the DIALECT register's ASSUMED
			// SlotSpace.NoneWire entry — but it is in no bank, so the
			// bank rung refuses it before the builder ever would.
			name:   "the grammatical none form belongs to no bank",
			ch:     codeplug.Channel{Slot: "000", Data: writableChannel().Data},
			reason: "not part of any bank",
		},
		{
			name:   "a 5xx slot this session never discovered",
			ch:     codeplug.Channel{Slot: "503", Data: writableChannel().Data},
			reason: "not part of any bank",
		},
		{
			name:   "an empty channel is an erase request",
			ch:     codeplug.Channel{Slot: "001"},
			fields: []spec.Field{spec.FieldErase},
			reason: "erase",
		},
		{
			// reason pins this to the Valid() rung specifically (MEDIUM-1):
			// "must have zero Value" is BoolField.Valid's own message, and
			// it is NOT "P11", the DIFFERENT refusal buildWriteCommand
			// gives a well-formed-but-non-Known TagDisplay further down
			// the ladder (see the "would manufacture byte 28" row below).
			// Without the Valid() check this row would still be refused —
			// by that later rung, with the wrong reason — so the reason
			// substring is what makes this row discriminating rather than
			// coincidentally green.
			name: "an incoherent TagDisplay field is refused, not interpreted",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable, Value: true}
			}),
			fields: []spec.Field{spec.FieldTagDisplay},
			reason: "must have zero Value",
		},
		{
			// ScanSkip's own incoherent case has NO later rung to catch
			// it: unlike TagDisplay, an Unavailable ScanSkip is never
			// requested at all (requestedFields' Known-only conditional),
			// so if this Valid() check were deleted the incoherent value
			// would reach neither the capability gate nor
			// buildWriteCommand — it would simply be ignored and the
			// write would proceed. This row is MEDIUM-1's
			// "Unavailable-with-a-value" case.
			name: "an incoherent ScanSkip field is refused, not interpreted",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ScanSkip = codeplug.BoolField{State: codeplug.Unavailable, Value: true}
			}),
			fields: []spec.Field{spec.FieldScanSkip},
			reason: "must have zero Value",
		},
		{
			// MEDIUM-1's "a bogus FieldState value" case: a State no
			// FieldState constant names. Nothing past this rung — not the
			// capability gate, not buildWriteCommand — inspects State for
			// well-formedness; only BoolField.Valid does.
			name: "a bogus ScanSkip FieldState is refused, not interpreted",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ScanSkip = codeplug.BoolField{State: codeplug.FieldState("bogus")}
			}),
			fields: []spec.Field{spec.FieldScanSkip},
			reason: `invalid State "bogus"`,
		},
		{
			// C-M1 (closing review wave 2, ACCEPT MEDIUM): this rung used to
			// check only CTCSSTone/ScanSkip/TagDisplay, and TxFreqHz — one
			// of the seventeen Icom-tier fields — was NOT among them.
			// codeplug.Validate also does not catch it (it skips every
			// non-Recorded field outright), and requestedFields is
			// Known-only, so nothing named TxFreqHz at all: this exact
			// channel reached buildWriteCommand and went out on the wire
			// with the malformed value silently DROPPED.
			//
			// RED-PROOF (captured against the pre-fix write.go, then
			// reverted — not re-run as part of this suite): with only the
			// three-field rung in place, this sub-test's own body — same
			// channel, same assertions — produced no refusal at all:
			//
			//	WriteChannel = {Steps:[{Command:MT Sent:true Confirmed:true}]}, err = <nil>
			//	frames sent: [MT001014250000-0150102010011CALLING     ;]
			//
			// i.e. the 41-byte MT Set went out exactly as writableChannel()
			// alone would have produced it — TxFreqHz has no place in this
			// radio's combined frame at all, so the caller's explicit
			// Known-shaped claim (Unavailable, yet carrying Value 1) was
			// simply thrown away rather than refused.
			name: "an incoherent TxFreqHz field is refused, not interpreted (C-M1)",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable, Value: 1}
			}),
			fields: []spec.Field{spec.FieldTxFrequency},
			reason: "must have zero Value",
		},
		{
			// MEDIUM-1 (Opus review of this sweep, 05/09/2026), the
			// erratum to the write-gate sweep's (i) DECISION: codeplug.
			// Absent IS the zero FieldState, so a caller who sets a Value
			// and forgets to set State — a copy/paste slip, not a
			// hand-built ChannelData that genuinely left a field
			// untouched — produces exactly the struct
			// TestWriteChannel_AbsentFieldStatesStillWrite's fixture is
			// built from. This is C-M1 again with the State OMITTED
			// instead of wrong.
			//
			// CTCSSTONE IS THE PROOF FIELD, the reviewer's own
			// reproduction: this radio's 41-byte combined Set has no
			// CTCSS-tone position at all (the C-M1 row above makes the
			// same point for TxFreqHz), so nothing past the walk would
			// ever have noticed it.
			//
			// RED-PROOF, captured against the pre-erratum judge (Absent
			// admitted regardless of Value), this sub-test's own body
			// unchanged: no refusal happened at all —
			//
			//	WriteChannel = {Steps:[{Command:MT Sent:true Confirmed:true}]}, err = <nil>
			//	frames sent: [MT001014250000-0150102010011CALLING     ;]
			//
			// i.e. the 41-byte MT Set went out exactly as writableChannel()
			// alone would have produced it, with the caller's CTCSSTone
			// value silently DROPPED.
			name: "an Absent CTCSSTone field carrying a non-zero value is refused, not interpreted (MEDIUM-1)",
			ch: withData(func(d *codeplug.ChannelData) {
				d.CTCSSTone = codeplug.ToneField{Value: 1000}
			}),
			fields: []spec.Field{spec.FieldCTCSSTone},
			reason: "must have zero Value",
		},
		{
			name: "a Known CTCSS tone the record cannot express",
			ch: withData(func(d *codeplug.ChannelData) {
				d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 1000}
			}),
			fields: []spec.Field{spec.FieldCTCSSTone},
		},
		{
			name: "a Known scan-skip flag the record cannot express",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
			}),
			fields: []spec.Field{spec.FieldScanSkip},
			reason: "not write-Supported",
		},
		{
			// C-M1: the FieldState rung (driver.CheckFieldStates) passes
			// Duplex.Valid THIS RADIO'S OWN (empty) DuplexOptions, so a
			// Known value is caught HERE, by that rung's own vocabulary
			// check, rather than reaching the capability gate further down
			// — an empty vocab
			// fails closed for every Known value (StringField.Valid's own
			// doc comment), which on this radio is every Icom-tier
			// StringField there is. Still the same field named, still a
			// refusal, still no frame; only WHICH rung catches it moved.
			name: "a Known Icom-tier field this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
			}),
			fields: []spec.Field{spec.FieldDuplex},
			reason: "not one of this radio's values",
		},
		// The seven D8 receiver-tier fields (HIGH-1): before this fix,
		// tierRequestedFields carried only the ten D4 fields, so a Known
		// value for any of these seven never reached requestedFields at
		// all — not refused, not written correctly, silently DROPPED from
		// the frame. Each row below is that exact shape: a channel this
		// radio's 41-byte record cannot express, which must be refused by
		// the capability gate naming the field, not dropped.
		{
			name: "a Known TuningStepEnabled this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TuningStepEnabled = codeplug.BoolField{State: codeplug.Known, Value: true}
			}),
			fields: []spec.Field{spec.FieldTuningStepEnabled},
			reason: "not write-Supported",
		},
		{
			// C-M1: same move as Duplex's above — TuningSteps is one of
			// the twelve EMPTY vocab/table caps fields on this radio, so
			// the walk's StringField.Valid catches a Known value
			// first.
			name: "a Known TuningStep this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TuningStep = codeplug.StringField{State: codeplug.Known, Value: "5"}
			}),
			fields: []spec.Field{spec.FieldTuningStep},
			reason: "not one of this radio's values",
		},
		{
			// ProgramTuningStepHz is a FreqField: Valid() takes no
			// vocab/table at all, so it stays coherence-only and this row
			// is unaffected by C-M1 — still caught by the capability gate
			// further down, as before.
			name: "a Known ProgramTuningStep this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Known, Value: 5000}
			}),
			fields: []spec.Field{spec.FieldProgramTuningStep},
			reason: "not write-Supported",
		},
		{
			// C-M1: AttenuatorDB is one of the twelve EMPTY caps fields
			// (s.caps.AttenuatorDB is nil on this radio), so
			// the walk's IntField.Valid catches a Known value
			// first, the same move as Duplex's above.
			name: "a Known Attenuator this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.AttenuatorDB = codeplug.IntField{State: codeplug.Known, Value: 20}
			}),
			fields: []spec.Field{spec.FieldAttenuator},
			reason: "not one of this radio's values",
		},
		{
			// C-M1: same move as Duplex's above.
			name: "a Known Preamp this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Preamp = codeplug.StringField{State: codeplug.Known, Value: "1"}
			}),
			fields: []spec.Field{spec.FieldPreamp},
			reason: "not one of this radio's values",
		},
		{
			// C-M1: same move as Duplex's above.
			name: "a Known Antenna this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Antenna = codeplug.StringField{State: codeplug.Known, Value: "ANT1"}
			}),
			fields: []spec.Field{spec.FieldAntenna},
			reason: "not one of this radio's values",
		},
		{
			name: "a Known IPPlus this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.IPPlus = codeplug.BoolField{State: codeplug.Known, Value: true}
			}),
			fields: []spec.Field{spec.FieldIPPlus},
			reason: "not write-Supported",
		},
		{
			name: "a non-Known TagDisplay would manufacture byte 28",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TagDisplay = codeplug.BoolField{State: codeplug.Unknown}
			}),
			fields: []spec.Field{spec.FieldTagDisplay},
			reason: "P11",
		},
		{
			name: "a TX-clarifier flag this radio's P5 legend prints (Fixed)",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TxClar = true
			}),
			fields: []spec.Field{spec.FieldClarifier},
			reason: "P5",
		},
		{
			name: "a mode this radio's legend does not print",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Mode = "PSK"
			}),
			fields: []spec.Field{spec.FieldMode},
		},
		{
			name: "a CTCSS state outside the three the record carries",
			ch: withData(func(d *codeplug.ChannelData) {
				d.CTCSS = "DCS"
			}),
			fields: []spec.Field{spec.FieldCTCSSState},
		},
		{
			name: "a shift outside the three the record carries",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Shift = "SPLIT"
			}),
			fields: []spec.Field{spec.FieldShift},
		},
		{
			name: "a clarifier beyond this dialect's declared maximum",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ClarHz = 10_000
			}),
			fields: []spec.Field{spec.FieldClarifier},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, sess := openSession(t, Simulated, slotImage{})
			before := len(p.Transcript())

			res, err := sess.WriteChannel(testCtx(t), tt.ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel = %v, want errors.Is match against driver.ErrWriteRefused", err)
			}
			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("error %v (%T) is not a *driver.WriteRefusedError", err, err)
			}
			if wre.Slot != tt.ch.Slot {
				t.Errorf("WriteRefusedError.Slot = %q, want %q", wre.Slot, tt.ch.Slot)
			}
			if tt.fields != nil && !reflect.DeepEqual(wre.Fields, tt.fields) {
				t.Errorf("WriteRefusedError.Fields = %v, want %v", wre.Fields, tt.fields)
			}
			if tt.reason != "" && !strings.Contains(wre.Reason, tt.reason) {
				t.Errorf("WriteRefusedError.Reason = %q, want it to contain %q", wre.Reason, tt.reason)
			}
			if res.Steps == nil || len(res.Steps) != 0 {
				t.Errorf("WriteResult.Steps = %#v, want an EMPTY, non-nil slice — nothing was attempted", res.Steps)
			}
			if after := p.Transcript(); len(after) != before {
				t.Errorf("the refusal sent %v — every rung of this ladder is PRE-WIRE", after[before:])
			}
		})
	}
}

// TestWriteChannel_CapabilityGateAnswersFirstOnUnconsentedRealHardware is
// matrix erratum M-E3 and plan P7's ordering clause, from the side that
// makes the order observable.
//
// The channel here is wrong in TWO ways at once — a non-Known TagDisplay
// and a true TX-clarifier flag — and on an unconsented RealHardware session
// the answer is NEITHER semantic refusal: writeTrialsComplete is false, so
// every field is Unverified, nothing is writable, and the capability gate
// refuses first, naming every field the write requested. That is the
// FT-710's shipped order and the one this driver keeps.
//
// The requested set omits spec.FieldTagDisplay precisely because the
// display value is not Known (requestedFields' conditional), so the gate
// complains about the fields the caller actually asked to write rather than
// about one nobody asked for.
func TestWriteChannel_CapabilityGateAnswersFirstOnUnconsentedRealHardware(t *testing.T) {
	p, sess := openSession(t, RealHardware, slotImage{})
	before := len(p.Transcript())

	ch := withData(func(d *codeplug.ChannelData) {
		d.TagDisplay = codeplug.BoolField{State: codeplug.Unknown}
		d.TxClar = true
	})
	res, err := sess.WriteChannel(testCtx(t), ch)
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel = %v (%T), want a *driver.WriteRefusedError", err, err)
	}
	want := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
		spec.FieldCTCSSState, spec.FieldShift, spec.FieldTag,
	}
	if !reflect.DeepEqual(wre.Fields, want) {
		t.Errorf("WriteRefusedError.Fields = %v, want %v — the CAPABILITY gate answers before either semantic refusal (M-E3)", wre.Fields, want)
	}
	if !strings.Contains(wre.Reason, "not write-Supported") {
		t.Errorf("WriteRefusedError.Reason = %q, want the capability gate's reason, not a semantic one", wre.Reason)
	}
	if res.Steps == nil || len(res.Steps) != 0 {
		t.Errorf("WriteResult.Steps = %#v, want an EMPTY, non-nil slice", res.Steps)
	}
	if after := p.Transcript(); len(after) != before {
		t.Errorf("the refusal sent %v — the gate is pre-wire", after[before:])
	}
}

// TestWriteChannel_ConsentedRealHardwareMeetsTheSemanticRefusals is the
// other half of M-E3: consent is the ONE thing that opens the capability
// gate on a RealHardware session (spec.ConsentUnverifiedWrites, applied in
// sessionCapabilities), and once it is open the two mandatory semantic
// refusals are what the channel meets next — in P7's order, TagDisplay
// before TxClar.
//
// Consent is a decision about RISK, not evidence: it lets a user write a
// field this project has never proven against a radio. It does not and must
// not reach past a refusal grounded in the radio's own printed legends,
// which is what both of these are.
func TestWriteChannel_ConsentedRealHardwareMeetsTheSemanticRefusals(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ch    codeplug.Channel
		field spec.Field
	}{
		{
			name: "TagDisplay first",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TagDisplay = codeplug.BoolField{State: codeplug.Unknown}
				d.TxClar = true
			}),
			field: spec.FieldTagDisplay,
		},
		{
			name:  "then TxClar",
			ch:    withData(func(d *codeplug.ChannelData) { d.TxClar = true }),
			field: spec.FieldClarifier,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, sess := openSession(t, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())
			before := len(p.Transcript())

			_, err := sess.WriteChannel(testCtx(t), tt.ch)
			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("WriteChannel = %v (%T), want a *driver.WriteRefusedError", err, err)
			}
			if want := []spec.Field{tt.field}; !reflect.DeepEqual(wre.Fields, want) {
				t.Errorf("WriteRefusedError.Fields = %v, want %v", wre.Fields, want)
			}
			if after := p.Transcript(); len(after) != before {
				t.Errorf("the refusal sent %v — both semantic refusals are pre-wire", after[before:])
			}
		})
	}
}

// TestWriteChannel_DiscoveredBanksAreNeverWritable: a 5xx or EMG slot that
// this session really did discover is still refused, on EVERY profile
// including Simulated.
//
// readOnlyFields (caps.go) forces every Write on those banks to
// spec.Unsupported, so the capability gate refuses — and the refusal names
// the fields rather than the bank, because that is what the gate knows. A
// Supported label there would advertise a write the codec will not build:
// this dialect's combined-MT write policy excludes 5xx/EMG slots outright.
func TestWriteChannel_DiscoveredBanksAreNeverWritable(t *testing.T) {
	img := slotImage{mrAnswers: map[string]string{"503": populatedMR("503"), "EMG": populatedMR("EMG")}}
	p, sess := openSession(t, Simulated, img)

	for _, slot := range []string{"503", "EMG"} {
		t.Run(slot, func(t *testing.T) {
			before := len(p.Transcript())
			ch := writableChannel()
			ch.Slot = slot

			res, err := sess.WriteChannel(testCtx(t), ch)
			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("WriteChannel(%q) = %v (%T), want a *driver.WriteRefusedError", slot, err, err)
			}
			if len(wre.Fields) == 0 {
				t.Errorf("WriteRefusedError.Fields is empty, want the unwritable fields named")
			}
			if res.Steps == nil || len(res.Steps) != 0 {
				t.Errorf("WriteResult.Steps = %#v, want an EMPTY, non-nil slice", res.Steps)
			}
			if after := p.Transcript(); len(after) != before {
				t.Errorf("the refusal sent %v — a discovered bank is never written", after[before:])
			}
		})
	}
}

// TestWriteChannel_RejectedByRadio: a "?;" answer to the Set is the radio
// explicitly refusing a frame that WAS transmitted.
//
// Sent true, Confirmed false — the outcome is attributable, and it is a
// refusal. The error wraps cat.ErrRejected so a caller handling rejections
// generically sees one, and it is NOT a *driver.WriteRefusedError: this
// driver refused nothing, the radio did.
//
// That a rejected Set draws exactly one "?;" is ASSUMED — the driver
// register's THE ACKNOWLEDGEMENT CONVENTIONS entry, whose lift is the first
// write trial with the port watched between the Set and the read-back.
func TestWriteChannel_RejectedByRadio(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{rejectSets: true})
	before := len(p.Transcript())

	res, err := sess.WriteChannel(testCtx(t), writableChannel())
	if !errors.Is(err, cat.ErrRejected) {
		t.Fatalf("WriteChannel = %v, want errors.Is match against cat.ErrRejected", err)
	}
	if errors.Is(err, driver.ErrWriteRefused) {
		t.Error("a radio's rejection must not be reported as this DRIVER's refusal — the frame went out")
	}
	want := []driver.WriteStep{{Command: "MT", Sent: true, Confirmed: false}}
	if !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("WriteResult.Steps = %+v, want %+v — the frame was transmitted and explicitly refused", res.Steps, want)
	}
	if got := p.Transcript()[before:]; len(got) != 1 {
		t.Errorf("wire carried %v, want exactly the one rejected Set — a write is NEVER resent", got)
	}
}

// TestWriteChannel_TransportFailureLeavesSentFalse: when the transport
// itself fails, the frame's fate is NOT attributable — the host cannot tell
// whether it reached the radio — so Sent stays false and the error, not the
// flags, carries the distinction.
//
// The step list is still DECLARED in full: an MT step present but never
// Sent says "this write intended one MT frame and it never provably went
// out", which a caller journaling the result can act on; an empty list
// would be indistinguishable from a driver that intended nothing.
func TestWriteChannel_TransportFailureLeavesSentFalse(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})
	if err := sess.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	res, err := sess.WriteChannel(testCtx(t), writableChannel())
	if err == nil {
		t.Fatal("WriteChannel over a closed engine = nil error, want a transport failure")
	}
	if errors.Is(err, cat.ErrRejected) {
		t.Error("a transport failure must not be reported as a radio rejection")
	}
	want := []driver.WriteStep{{Command: "MT", Sent: false, Confirmed: false}}
	if !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("WriteResult.Steps = %+v, want %+v — an unattributable frame is not Sent", res.Steps, want)
	}
}

// TestWriteChannel_CannotLandInsideAReadsCrossCheck is plan P3's
// concurrency pin for the WRITE half: opMu guards a single driver
// operation, so a write cannot land between a concurrent ReadChannel's MT
// "?;" and the MR that interprets it.
//
// One MT Set is one exchange, and transport.Engine already serialises each
// exchange — so the lock buys nothing for the write CONSIDERED ALONE. What
// it buys is this: the read's cross-check spans TWO exchanges, and a Set
// landing in that gap would change the very slot whose MT rejection the MR
// is about to interpret, turning "empty" into a statement about a radio
// state that no longer exists. The gap is forced deterministically through
// readChannelGapHook rather than by hammering, for the reason
// TestReadChannel_CrossCheckIsAtomicUnderOpMu records.
func TestWriteChannel_CannotLandInsideAReadsCrossCheck(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())
	before := len(p.Transcript())

	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	readChannelGapHook = func() {
		once.Do(func() { close(reached) })
		<-release
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	readDone := make(chan error, 1)
	go func() {
		_, err := sess.ReadChannel(testCtx(t), "002") // the cross-check path
		readDone <- err
	}()

	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("ReadChannel never reached the gap between its MT rejection and its MR read within 5s")
	}

	writeDone := make(chan error, 1)
	go func() {
		_, err := sess.WriteChannel(testCtx(t), writableChannel())
		writeDone <- err
	}()

	// A generous, deterministic window for the write to reach the wire if
	// nothing is holding it back.
	time.Sleep(500 * time.Millisecond)
	if got := p.Transcript()[before:]; len(got) != 1 || got[0] != "MT002;" {
		t.Errorf("while one ReadChannel was between its MT and MR, the wire carried %v — want only [\"MT002;\"]: a WRITE must not interleave with a cross-check (P3)", got)
	}

	close(release)
	for _, done := range []chan error{readDone, writeDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("concurrent operation: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a concurrent operation never completed after the gap hook was released")
		}
	}

	if got, want := p.Transcript()[before:], []string{"MT002;", "MR002;", writableChannelFrame}; !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v — the cross-check's two frames must be adjacent and the Set must follow them", got, want)
	}
}

// TestNameMaps_AreExactInverses walks read.go's ctcssNames/shiftNames and
// write.go's ctcssByName/shiftByName in both directions.
//
// Two hand-written maps meant to be exact inverses are worth a test rather
// than an adjacency: a spelling added to one and forgotten in the other
// would silently refuse a legitimate value at the write gate, or — the
// worse direction — map it onto the wrong byte.
func TestNameMaps_AreExactInverses(t *testing.T) {
	if len(ctcssNames) != len(ctcssByName) {
		t.Errorf("ctcssNames has %d entries, ctcssByName %d", len(ctcssNames), len(ctcssByName))
	}
	for state, name := range ctcssNames {
		if back, ok := ctcssByName[name]; !ok || back != state {
			t.Errorf("ctcssByName[%q] = %q (present=%v), want %q", name, back, ok, state)
		}
	}
	for name, state := range ctcssByName {
		if back, ok := ctcssNames[state]; !ok || back != name {
			t.Errorf("ctcssNames[%q] = %q (present=%v), want %q", state, back, ok, name)
		}
	}

	if len(shiftNames) != len(shiftByName) {
		t.Errorf("shiftNames has %d entries, shiftByName %d", len(shiftNames), len(shiftByName))
	}
	for sh, name := range shiftNames {
		if back, ok := shiftByName[name]; !ok || back != sh {
			t.Errorf("shiftByName[%q] = %q (present=%v), want %q", name, back, ok, sh)
		}
	}
	for name, sh := range shiftByName {
		if back, ok := shiftNames[sh]; !ok || back != name {
			t.Errorf("shiftNames[%q] = %q (present=%v), want %q", sh, back, ok, name)
		}
	}

	// Non-vacuity: the vocabularies this driver's capability data
	// advertises must be exactly the keys of the write-direction maps, so
	// neither test above can pass over an empty pair.
	caps := CapabilitiesSimulated()
	if len(caps.CTCSSStates) != len(ctcssByName) || len(caps.ShiftOptions) != len(shiftByName) {
		t.Fatalf("advertised vocabularies (%d CTCSS states, %d shifts) do not match the write maps (%d, %d)",
			len(caps.CTCSSStates), len(caps.ShiftOptions), len(ctcssByName), len(shiftByName))
	}
	for _, st := range caps.CTCSSStates {
		if _, ok := ctcssByName[st.Value]; !ok {
			t.Errorf("Capabilities advertises CTCSS state %q, which the write path cannot resolve", st.Value)
		}
	}
	for _, so := range caps.ShiftOptions {
		if _, ok := shiftByName[so.Value]; !ok {
			t.Errorf("Capabilities advertises shift %q, which the write path cannot resolve", so.Value)
		}
	}
}

// TestRequestedFields_MembershipAndOrder pins the set this driver's
// capability gate judges — the mirror of codeplug.Diff's own requested-set
// derivation, same membership, same conditionals, same order, so the two
// gates judge the same channel the same way.
//
// The six plain fields are ALWAYS requested: this radio's combined Set
// carries frequency, mode, clarifier, CTCSS state, shift AND the tag in one
// frame whether or not any of them changed. TagDisplay keeps the seventh
// place it holds whenever it appears at all, and the seventeen Icom-tier
// conditionals come LAST, in codeplug.ChannelData's declaration order.
//
// TagDisplay's conditional needs a word its neighbours do not, because on
// THIS radio byte 28 is MANDATORY on the frame: a non-Known display value
// is never quietly omitted from the wire, it is REFUSED by
// buildWriteCommand. What the conditional fixes is which gate such a
// channel meets FIRST — without it, a session whose FieldTagDisplay is not
// write-Supported would refuse naming a field nobody asked to write,
// instead of the refusal that names the real problem.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	plain := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
		spec.FieldCTCSSState, spec.FieldShift, spec.FieldTag,
	}

	t.Run("a channel with nothing else Known requests exactly the six", func(t *testing.T) {
		d := *writableChannel().Data
		d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable}
		if got := requestedFields(d); !reflect.DeepEqual(got, plain) {
			t.Errorf("requestedFields = %v, want %v", got, plain)
		}
	})

	t.Run("a Known TagDisplay is seventh", func(t *testing.T) {
		want := append(append([]spec.Field(nil), plain...), spec.FieldTagDisplay)
		if got := requestedFields(*writableChannel().Data); !reflect.DeepEqual(got, want) {
			t.Errorf("requestedFields = %v, want %v", got, want)
		}
	})

	t.Run("the tone and skip conditionals follow it, then the tier", func(t *testing.T) {
		d := *writableChannel().Data
		d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 1000}
		d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
		d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
		d.Filter = codeplug.StringField{State: codeplug.Known, Value: "WIDE"}
		want := append(append([]spec.Field(nil), plain...),
			spec.FieldTagDisplay, spec.FieldCTCSSTone, spec.FieldScanSkip,
			spec.FieldDuplex, spec.FieldFilter)
		if got := requestedFields(d); !reflect.DeepEqual(got, want) {
			t.Errorf("requestedFields = %v, want %v", got, want)
		}
	})

	t.Run("every tier predicate is reachable", func(t *testing.T) {
		// Non-vacuity for the tier half: each entry's own predicate must
		// answer true for SOME channel, or the entry is dead weight the
		// gate would never consult.
		if len(tierRequestedFields) != 17 {
			t.Fatalf("tierRequestedFields has %d entries, want the seventeen codeplug's tierAddedFieldFor carries", len(tierRequestedFields))
		}
		for _, tr := range tierRequestedFields {
			if tr.present(*writableChannel().Data) {
				t.Errorf("%s is requested by an ordinary FT-891 channel — every tier field reads back Unavailable on this radio", tr.field)
			}
		}
	})

	// TestRequestedFields' seventeen must be exactly the seventeen fields
	// codeplug's own tierAddedFieldFor carries, in the same order — and
	// since tierAddedFieldFor is unexported (this table cannot import it;
	// see tierRequestedFields' doc comment for what codeplug DOES export),
	// this is the fallback the brief calls for: derive the same seventeen
	// independently, from spec.AllFields() (an exported enumeration built
	// from the very same spec.Field constants tierAddedFieldFor's own
	// table is built from) minus the ten pre-tier fields, and require
	// EXACT membership and order against that derivation. A tier field
	// this table forgets to mirror — or the codeplug side gains and this
	// one doesn't — fails HERE rather than passing silently until a caller
	// hits the hole HIGH-1 measured.
	t.Run("the seventeen are exactly spec.AllFields()'s tier-added tail", func(t *testing.T) {
		preTier := []spec.Field{
			spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
			spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift,
			spec.FieldTag, spec.FieldTagDisplay, spec.FieldScanSkip,
			spec.FieldErase,
		}
		all := spec.AllFields()
		if len(all) < len(preTier) || !reflect.DeepEqual(all[:len(preTier)], preTier) {
			t.Fatalf("spec.AllFields() = %v, want it to start with the ten pre-tier fields %v — this test's derivation assumes that prefix", all, preTier)
		}
		wantTier := all[len(preTier):]

		gotTier := make([]spec.Field, len(tierRequestedFields))
		for i, tr := range tierRequestedFields {
			gotTier[i] = tr.field
		}
		if !reflect.DeepEqual(gotTier, wantTier) {
			t.Errorf("tierRequestedFields names\n %v\nbut spec.AllFields() carries the tier-added\n %v\n(the two must be the same seventeen fields in the same order)", gotTier, wantTier)
		}
	})
}

// The FT-891 no longer carries its own "every FieldState field is
// covered" pin. Wave 2's TestFieldStateChecks_CoversExactlyTheFieldStateFields
// asserted that against this package's own fieldStateChecks table; the
// write-gate sweep's item (i) moved that table into core/driver, where the
// four Yaesu drivers share it, and the assertion moved with it —
// driver_test.TestFieldStateWalk_CoversEveryFieldStateField makes exactly
// the same derivation from spec.AllFields() minus the seven plain fields.
// The walk's field list does not depend on which radio's capabilities it is
// handed, so a copy here would be the same assertion twice.

// TestMTSetSpec_IsFireAndForgetAndNeverRetries pins the transport spec the
// combined Set goes out under, and every part of it is load-bearing.
//
// NO answer matcher: on the ASSUMED convention an accepted Set draws no
// reply at all, so a spec that waited for an "MT" answer would spend the
// whole read timeout and then report a timeout for a write the radio had
// accepted perfectly. The write class is what selects the transport's
// fire-and-forget path — a bounded listen for a late "?;" and silence
// otherwise.
//
// RETRYREADS 0, necessarily: a write is NEVER resent (transport safety
// obligation 2), and transport.Engine.Do refuses a write-class spec with a
// non-zero RetryReads before writing anything.
//
// It is a SEPARATE function from read.go's mtSpec rather than a reuse of
// it: the read's spec pins the combined ANSWER's exact geometry from the
// dialect, which is right for a read and would be exactly the bug above
// here.
func TestMTSetSpec_IsFireAndForgetAndNeverRetries(t *testing.T) {
	got := mtSetSpec()
	if want := transport.CATWriteSpec(); !reflect.DeepEqual(got, want) {
		t.Errorf("mtSetSpec() = %+v, want transport.CATWriteSpec() %+v", got, want)
	}
	if got.RetryReads != 0 {
		t.Errorf("mtSetSpec().RetryReads = %d, want 0 — a write is never resent", got.RetryReads)
	}
	if read, err := mtSpec(catDialect); err != nil {
		t.Fatalf("mtSpec(catDialect) = %v", err)
	} else if reflect.DeepEqual(got, read) {
		t.Error("the Set spec equals the READ spec — a Set that waited for the read's 41-byte answer would time out on every accepted write")
	}
}

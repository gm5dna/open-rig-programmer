// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// writableChannel is a valid, unremarkable channel this driver can write:
// 145.500 MHz FM, clarifier -150 Hz with BOTH clarifier flags on, CTCSS
// ENC-DEC, PLUS shift, tag "CALLING".
//
// It is the WRITE-direction twin of respondingport_test.go's
// populatedFields("001") and carries the same values on purpose, so the
// byte-level expectations below can be compared against that helper's
// read-direction ones by eye. Every field is stated: a write of this radio's
// combined form carries the whole record whether or not a value changed, so
// a channel with an unstated field would be testing a default rather than a
// decision.
//
// TWO OF ITS CELLS ARE THIS RADIO'S OWN AND EACH INVERTS THE FT-891
// EXEMPLAR (matrix erratum M-E3), which is why this fixture cannot be that
// one with the model name changed:
//
//   - TxClar is TRUE. Byte 21 is a live TX-clarifier state here — `P5 0: TX
//     CLAR "OFF" 1: TX CLAR "ON"` on all five blocks carrying the grid (MR
//     971, MT 1004, MW 1042, IF 787, OI 1122) — where the FT-891 prints
//     "0: (Fixed)" and its write path REFUSES a TxClar-true record outright.
//     A driver copied from that package would refuse this perfectly ordinary
//     channel.
//   - TagDisplay is UNAVAILABLE, not Known. MT's P11 legend reads
//     "P11 0: (Fixed)" (layout 1015), so the field does not exist on this
//     radio; caps.go grades spec.FieldTagDisplay the zero FieldSupport and
//     read.go reports it Unavailable. Unavailable is exactly what a read of
//     this radio produces, which is what makes this an honest "otherwise
//     writable" channel — and a KNOWN value here is refused by the
//     capability gate, which the ladder pins as its own row.
//
// The seventeen Icom-tier fields are stated Unavailable through
// unavailableTierFields (read_test.go), the SAME production shape read.go's
// mapping actually produces (plan P12). The fleet FieldState walk admits
// codeplug.Absent for them too, and that case has its own test
// (TestWriteChannel_AbsentFieldStatesStillWrite) rather than being something
// every fixture has to dodge.
func writableChannel() codeplug.Channel {
	d := unavailableTierFields()
	d.FreqHz = 145_500_000
	d.Mode = "FM"
	d.ClarHz = -150
	d.RxClar = true
	d.TxClar = true
	d.CTCSS = "ENC-DEC"
	d.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
	d.Shift = "PLUS"
	d.Tag = "CALLING"
	d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable}
	d.ScanSkip = codeplug.BoolField{State: codeplug.Unknown}
	return codeplug.Channel{Slot: "001", Data: &d}
}

// writableChannelFrame is the 41-byte combined MT Set writableChannel()
// produces, re-derived by hand: see TestWriteChannel_OneCombinedMTSetFrame
// for the position-by-position derivation it is checked against.
//
//	MT|001|145500000|-|0150|1|1|4|0|1|00|1|0|CALLING_____|;
const writableChannelFrame = "MT001145500000-0150114010010CALLING     ;"

// withData returns writableChannel() with mutate applied to its data — the
// one-field-wrong shape most of the ladder's rungs need.
func withData(mutate func(*codeplug.ChannelData)) codeplug.Channel {
	ch := writableChannel()
	mutate(ch.Data)
	return ch
}

// TestWriteChannel_OneCombinedMTSetFrame pins the BYTES, position by
// position, on both banks and at both ends of every vocabulary this record
// carries.
//
// THE EXPECTED FRAMES ARE RE-DERIVED BY HAND from manual revision 1711-D's
// own MT position chart (layout 998-1033), field by field, and are
// deliberately NOT built through cat.Dialect.BuildMTSetCombined — a fixture
// produced by the builder under test would agree with a wrong offset as
// happily as a right one. The 41 positions are:
//
//	1-2    "MT"
//	3-5    P1  slot
//	6-14   P2  frequency, nine digits
//	15     P3  clarifier direction ('+' or '-')
//	16-19  P3  clarifier magnitude, four digits
//	20     P4  RX clarifier flag
//	21     P5  the TX clarifier flag — LIVE on this radio (layout 1004)
//	22     P6  mode nibble
//	23     P7  "Set: 0: (Fixed)" — the combined Set's form constant (1009)
//	24     P8  CTCSS state, FIVE-valued here (1010-1011)
//	25-26  P9  "00", documented fixed (1012)
//	27     P10 shift
//	28     P11 "0: (Fixed)" — SCHEMA on this radio (1015)
//	29-40  P12 the twelve-byte tag field, fill-padded (1017)
//	41     ';'
//
// ONE frame per channel and no MW: the 41-byte Set carries the whole field
// block and the tag together, where MW's 28 bytes would write the same
// fields redundantly and could not carry the tag at all (layout 1036-1051),
// and MW's Read and Answer grids are printed EMPTY (1048, 1051). That one
// Set SUFFICES to create or overwrite a channel — that the radio accepts it
// as a complete channel definition — is the DRIVER register's own A SINGLE
// COMBINED MT SET SUFFICES TO CREATE OR OVERWRITE A CHANNEL entry (doc.go);
// that its silence means acceptance is the SHARED register's THE
// ACKNOWLEDGEMENT CONVENTIONS entry.
//
// P11 IS NEVER THIS DRIVER'S BYTE TO CHOOSE. It is written by the builder
// under cat.P11Fixed, not passed in, and there is no display argument at
// this call site at all — the other half of erratum M-E3, and the reason
// this test has no "display ON/OFF" axis where the FT-891's has one.
func TestWriteChannel_OneCombinedMTSetFrame(t *testing.T) {
	for _, tt := range []struct {
		name  string
		ch    codeplug.Channel
		frame string
	}{
		{
			name:  "MEM, with the LIVE TX clarifier flag set",
			ch:    writableChannel(),
			frame: writableChannelFrame,
		},
		{
			// The other end of every vocabulary AND the other bank: this
			// radio's TOP PMS slot as a WIRE NUMBER (117, not "P9U" — the
			// numeric PMS form its MC legend prints at layout 916), the
			// mode legend's last nibble 'E' (C4FM here, PSK on the FTdx10),
			// the clarifier at its declared maximum with the opposite
			// sign, both clarifier flags off, the FIVE-state P8's top value
			// DCS-ENC, MINUS shift, and a two-character tag whose 12-byte
			// wire field is fill-padded.
			name: "PMS by wire number, at the top of every vocabulary",
			ch: func() codeplug.Channel {
				d := unavailableTierFields()
				d.FreqHz = 30_000
				d.Mode = "C4FM"
				d.ClarHz = 9990
				d.RxClar = false
				d.TxClar = false
				d.CTCSS = "DCS-ENC"
				d.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
				d.Shift = "MINUS"
				d.Tag = "AB"
				d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable}
				d.ScanSkip = codeplug.BoolField{State: codeplug.Unknown}
				return codeplug.Channel{Slot: "117", Data: &d}
			}(),
			//   MT|117|000030000|+|9990|0|0|E|0|4|00|2|0|AB__________|;
			frame: "MT117000030000+999000E040020AB          ;",
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

// TestWriteChannel_TxClarTrueIsWrittenNotRefused is matrix erratum M-E3's
// first half, stated as its own pin rather than left implicit in the frame
// test's fixture.
//
// The FT-891 exemplar REFUSES a TxClar-true record before building a frame,
// naming spec.FieldClarifier and P5, because that radio's P5 legend prints
// "0: (Fixed)". Here the legend prints `0: TX CLAR "OFF" 1: TX CLAR "ON"`
// on all five blocks that carry the grid, the dialect declares
// cat.P5TxClar, and core/cat ENCODES the flag (memdata.go's P5TxClar arm).
// A driver that carried the sibling's rung across would refuse, on every
// write, a field this radio's own manual prints as live — which is why the
// test asserts the BYTE and not merely the absence of an error.
func TestWriteChannel_TxClarTrueIsWrittenNotRefused(t *testing.T) {
	for _, tt := range []struct {
		name   string
		txClar bool
		want   byte
	}{
		{name: "TX clarifier ON", txClar: true, want: '1'},
		{name: "TX clarifier OFF", txClar: false, want: '0'},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, sess := openSession(t, Simulated, slotImage{})
			before := len(p.Transcript())

			ch := withData(func(d *codeplug.ChannelData) { d.TxClar = tt.txClar })
			if _, err := sess.WriteChannel(testCtx(t), ch); err != nil {
				t.Fatalf("WriteChannel = %v, want nil — P5 is a LIVE flag on this radio (M-E3)", err)
			}
			got := p.Transcript()[before:]
			if len(got) != 1 {
				t.Fatalf("wire carried %v, want exactly one Set", got)
			}
			// Position 21, one-based: byte 20 of the frame.
			if got[0][20] != tt.want {
				t.Errorf("frame %q carries %q at position 21, want %q", got[0], got[0][20], tt.want)
			}
		})
	}
}

// TestWriteChannel_NonKnownTagDisplayWritesUnchanged is matrix erratum
// M-E3's second half.
//
// On the FT-891 byte 28 is a live TAG flag with no "leave it alone"
// encoding, so a non-Known TagDisplay is REFUSED by that driver's
// buildWriteCommand — it would otherwise manufacture a value for a field
// whose FieldState says "preserve whatever the radio has". Here P11 is
// printed "0: (Fixed)" (layout 1015), the field does not exist, the
// display-LESS builder is the only one this dialect admits, and byte 28 is
// the builder's constant. So there is NO TagDisplay rung to write, and every
// non-Known state — Unknown, Unavailable, and the zero Absent — produces the
// SAME 41 bytes, which is what this test asserts rather than merely that
// nothing failed.
func TestWriteChannel_NonKnownTagDisplayWritesUnchanged(t *testing.T) {
	for _, tt := range []struct {
		name  string
		state codeplug.FieldState
	}{
		{"Unknown", codeplug.Unknown},
		{"Unavailable", codeplug.Unavailable},
		{"Absent (the zero FieldState)", codeplug.Absent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, sess := openSession(t, Simulated, slotImage{})
			before := len(p.Transcript())

			ch := withData(func(d *codeplug.ChannelData) {
				d.TagDisplay = codeplug.BoolField{State: tt.state}
			})
			if _, err := sess.WriteChannel(testCtx(t), ch); err != nil {
				t.Fatalf("WriteChannel = %v, want nil — this radio's P11 is SCHEMA, so there is nothing for a display state to manufacture", err)
			}
			if got, want := p.Transcript()[before:], []string{writableChannelFrame}; !reflect.DeepEqual(got, want) {
				t.Errorf("wire carried %q\nwant             %q — byte 28 is the builder's constant, not this field's", got, want)
			}
		})
	}
}

// TestWriteChannel_AbsentFieldStatesStillWrite is the fleet FieldState
// stance's Absent rule (the Yaesu write-gate sweep's item (i), 05/09/2026),
// which this driver inherits by CALLING driver.CheckFieldStates rather than
// carrying a table of its own: a caller who set nothing has requested
// nothing, so an all-Absent channel must WRITE rather than be refused for
// fields nobody meant to speak about.
//
// The C-M1 half is unchanged and is pinned by the ladder's own
// "must have zero Value" rows: a non-Known state CARRYING a value is still
// refused.
//
// THIS RADIO HAS NO EXCEPTION TO THE RULE, where the FT-891 has one: there,
// an Absent TagDisplay is still refused one rung further down, because byte
// 28 is mandatory on its frame. Here it is not, so every FieldState field
// without exception may be left alone.
func TestWriteChannel_AbsentFieldStatesStillWrite(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	// EVERY FieldState field left at its zero value, and the plain fields
	// set to writableChannel's own values, so the frame below is comparable
	// with writableChannelFrame byte for byte.
	d := codeplug.ChannelData{
		FreqHz: 145_500_000,
		Mode:   "FM",
		ClarHz: -150,
		RxClar: true,
		TxClar: true,
		CTCSS:  "ENC-DEC",
		Shift:  "PLUS",
		Tag:    "CALLING",
	}
	ch := codeplug.Channel{Slot: "001", Data: &d}
	for _, c := range driver.FieldStateChecks(CapabilitiesSimulated(), d) {
		if c.Err != nil {
			t.Fatalf("the fixture's %s is not Absent-and-admitted (%v) — this test asserts nothing unless every FieldState field is left at its zero value", c.Field, c.Err)
		}
	}

	before := len(p.Transcript())
	res, err := sess.WriteChannel(testCtx(t), ch)
	if err != nil {
		t.Fatalf("WriteChannel = %v, want nil — an all-Absent channel requests nothing and must WRITE", err)
	}
	if got, want := p.Transcript()[before:], []string{writableChannelFrame}; !reflect.DeepEqual(got, want) {
		// The fields writableChannel states and this one leaves Absent
		// (TagDisplay, CTCSSTone, ScanSkip and the seventeen) have no
		// position in the 41-byte record, so an Absent state cannot move a
		// byte — a frame that differed would mean the walk had let a
		// non-Known field reach the wire.
		t.Errorf("wire carried %q\nwant             %q", got, want)
	}
	if want := []driver.WriteStep{{Command: "MT", Sent: true, Confirmed: true}}; !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("WriteResult.Steps = %+v, want %+v", res.Steps, want)
	}
}

// TestWriteChannel_NoVerifyInsideWriteChannel: the write is ONE frame and
// the driver never reads the slot back (plan P12, matrix §3.6).
//
// core/driver/driver.go assigns positive read-back verification to
// core/clone, which holds both sides of the comparison and the policy for
// what to do about a mismatch. A driver that read back for itself would
// either duplicate that policy or quietly diverge from it, and it would
// double the frames every send costs. On THIS radio it would also change
// what the transcript counts say, since read and write share one prefix.
func TestWriteChannel_NoVerifyInsideWriteChannel(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})
	before := len(p.Transcript())

	if _, err := sess.WriteChannel(testCtx(t), writableChannel()); err != nil {
		t.Fatalf("WriteChannel = %v, want nil", err)
	}
	got := p.Transcript()[before:]
	if len(got) != 1 {
		t.Fatalf("WriteChannel sent %v — want exactly ONE frame; a read-back here belongs to core/clone (P12)", got)
	}
	if len(got[0]) == mtReadFrameLen {
		t.Errorf("WriteChannel sent %q, which is an MT READ — no verify happens inside the driver", got[0])
	}
}

// TestWriteChannel_RefusalLadder walks the folded matrix §3.6 ladder, rung
// by rung, on the Simulated profile — the profile the ordering ruling
// (decisions.md cell 10) says the post-gate refusals are pinned on, because
// on unconsented RealHardware the capability gate answers first (the next
// test).
//
// THE LADDER HAS FIVE RUNGS HERE, AND TWO OF THE FT-891'S ARE ABSENT:
// ParseSlot, the empty-channel rung, driver.CheckFieldStates, the capability
// gate, then core/cat's own builders. There is NO TxClar rung and NO
// TagDisplay rung (erratum M-E3) — the two rows that would carry them are
// instead POSITIVE tests above.
//
// Every rung must refuse BEFORE any byte reaches the wire, with a typed
// *driver.WriteRefusedError naming the field responsible where there is one,
// and with an explicitly EMPTY (never nil) step list.
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
			// THIS RADIO'S OWN NEGATIVE, and it is a slot on every
			// registered sibling: the FT-991A's PMS pairs are the decimal
			// channel numbers 100-117 (its MC legend, layout 916), so the
			// TOKEN form every sibling's legend prints is a NON-SLOT here
			// and dies at rung 1 rather than being quietly rewritten.
			name:   "the sibling PMS TOKEN form is a non-slot here",
			ch:     codeplug.Channel{Slot: "P1L", Data: writableChannel().Data},
			reason: "not a valid slot",
		},
		{
			// The FTdx10 and the FT-891 both have a 5 MHz bank; "5xx",
			// "5 MHz" and "5MHz" appear in no FT-991A slot legend, so this
			// dialect declares none and the slot is not grammatical at all
			// — it never reaches the bank rung.
			name:   "a 5xx slot this radio has no bank for",
			ch:     codeplug.Channel{Slot: "503", Data: writableChannel().Data},
			reason: "not a valid slot",
		},
		{
			// "000" parses — the DIALECT register's ASSUMED
			// SlotSpace.NoneWire entry, grammatical because cat.SlotSpace
			// structurally requires a none form — but it is in no bank, so
			// the bank lookup refuses it before the builder ever would.
			name:   "the grammatical none form belongs to no bank",
			ch:     codeplug.Channel{Slot: "000", Data: writableChannel().Data},
			reason: "not part of any bank",
		},
		{
			name:   "an empty channel is an erase request",
			ch:     codeplug.Channel{Slot: "001"},
			fields: []spec.Field{spec.FieldErase},
			reason: "erase",
		},
		{
			// THE FIELDSTATE WALK'S OWN ROW FOR A FIELD THIS RADIO CANNOT
			// EXPRESS, and it is more load-bearing here than on the FT-891.
			// There, a malformed TagDisplay has a SECOND rung waiting for
			// it (byte 28 is mandatory on that frame). Here there is none:
			// requestedFields' Known-only conditional never names an
			// Unavailable field, and byte 28 is the builder's constant — so
			// without driver.CheckFieldStates this incoherent value would
			// be silently DROPPED rather than refused.
			name: "an incoherent TagDisplay field is refused, not interpreted",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable, Value: true}
			}),
			fields: []spec.Field{spec.FieldTagDisplay},
			reason: "must have zero Value",
		},
		{
			name: "an incoherent ScanSkip field is refused, not interpreted",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ScanSkip = codeplug.BoolField{State: codeplug.Unavailable, Value: true}
			}),
			fields: []spec.Field{spec.FieldScanSkip},
			reason: "must have zero Value",
		},
		{
			// A State no FieldState constant names. Nothing past this rung
			// — not the capability gate, not buildWriteCommand — inspects
			// State for well-formedness.
			name: "a bogus ScanSkip FieldState is refused, not interpreted",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ScanSkip = codeplug.BoolField{State: codeplug.FieldState("bogus")}
			}),
			fields: []spec.Field{spec.FieldScanSkip},
			reason: `invalid State "bogus"`,
		},
		{
			// C-M1 (the FT-891's closing review wave 2), inherited whole by
			// calling the fleet walk: a tier field carrying a non-Known
			// State AND a value has no position in this record, so before
			// the walk covered every field it was silently dropped.
			name: "an incoherent TxFreqHz field is refused, not interpreted (C-M1)",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable, Value: 1}
			}),
			fields: []spec.Field{spec.FieldTxFrequency},
			reason: "must have zero Value",
		},
		{
			// The Yaesu gate sweep's MEDIUM-1: codeplug.Absent IS the zero
			// FieldState, so a caller who sets a Value and forgets to set
			// State is refused exactly as one alongside Unknown/Unavailable
			// is. This radio's 41-byte record has no CTCSS-tone position at
			// all, so nothing past the walk would ever have noticed it.
			name: "an Absent CTCSSTone field carrying a non-zero value is refused (MEDIUM-1)",
			ch: withData(func(d *codeplug.ChannelData) {
				d.CTCSSTone = codeplug.ToneField{Value: 1000}
			}),
			fields: []spec.Field{spec.FieldCTCSSTone},
			reason: "must have zero Value",
		},
		{
			// The register's TONE-NUMBER UNREACHABILITY entry, made
			// user-visible: the record has no tone-number position, so a
			// value the caller explicitly marked Known is REFUSED rather
			// than dropped.
			name: "a Known CTCSS tone the record cannot express",
			ch: withData(func(d *codeplug.ChannelData) {
				d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 1000}
			}),
			fields: []spec.Field{spec.FieldCTCSSTone},
			reason: "not write-Supported",
		},
		{
			// The register's SCAN-SKIP UNREACHABILITY entry, likewise.
			name: "a Known scan-skip flag the record cannot express",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
			}),
			fields: []spec.Field{spec.FieldScanSkip},
			reason: "not write-Supported",
		},
		{
			// THE M-E3 INVERSION AS A REFUSAL. On the FT-891 a Known
			// TagDisplay is the ORDINARY case and its absence is the
			// refusal; here the field does not exist (P11 "0: (Fixed)",
			// layout 1015), caps.go grades it the zero FieldSupport, and a
			// caller who explicitly asks to write it is refused BY THE
			// CAPABILITY GATE — not by a semantic rung, because there is
			// none. requestedFields' Known-only conditional is what routes
			// it here: it names the field the caller actually asked for.
			name: "a Known TagDisplay this radio's frame has no flag for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
			}),
			fields: []spec.Field{spec.FieldTagDisplay},
			reason: "not write-Supported",
		},
		{
			// The walk passes Duplex.Valid THIS RADIO'S OWN (empty)
			// DuplexOptions, and an empty vocabulary fails closed for every
			// Known value, so this is caught by the walk rather than by the
			// capability gate below it. Same field named, same refusal, no
			// frame; only WHICH rung catches it differs.
			name: "a Known Icom-tier field this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
			}),
			fields: []spec.Field{spec.FieldDuplex},
			reason: "not one of this radio's values",
		},
		{
			name: "a Known TuningStepEnabled this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TuningStepEnabled = codeplug.BoolField{State: codeplug.Known, Value: true}
			}),
			fields: []spec.Field{spec.FieldTuningStepEnabled},
			reason: "not write-Supported",
		},
		{
			name: "a Known TuningStep this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.TuningStep = codeplug.StringField{State: codeplug.Known, Value: "5"}
			}),
			fields: []spec.Field{spec.FieldTuningStep},
			reason: "not one of this radio's values",
		},
		{
			// ProgramTuningStepHz is a FreqField: Valid() takes no
			// vocabulary at all, so it stays coherence-only and the
			// capability gate is what catches it.
			name: "a Known ProgramTuningStep this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Known, Value: 5000}
			}),
			fields: []spec.Field{spec.FieldProgramTuningStep},
			reason: "not write-Supported",
		},
		{
			name: "a Known Attenuator this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.AttenuatorDB = codeplug.IntField{State: codeplug.Known, Value: 20}
			}),
			fields: []spec.Field{spec.FieldAttenuator},
			reason: "not one of this radio's values",
		},
		{
			name: "a Known Preamp this frame has no room for",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Preamp = codeplug.StringField{State: codeplug.Known, Value: "IPO"}
			}),
			fields: []spec.Field{spec.FieldPreamp},
			reason: "not one of this radio's values",
		},
		{
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
			// "PSK" is the FTdx10's word for nibble 'E'; this manual prints
			// "C4FM" for the same nibble, which is what makes core/cat's
			// package-level fallback ACTIVELY WRONG here. So this row is
			// not a nonsense string but a SIBLING'S REAL MODE, which is
			// exactly what a codeplug written for another radio carries.
			name: "a mode this radio's legend does not print",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Mode = "PSK"
			}),
			fields: []spec.Field{spec.FieldMode},
		},
		{
			// "DCS" alone is not one of the FIVE: this radio's P8 legend
			// distinguishes DCS ENC/DEC from DCS ENC, and a caller who
			// names neither has said something the record cannot carry.
			name: "a CTCSS state outside the five the record carries",
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
			// 10 000 Hz is inside the manual's PRINTED "0000 - 9999 (Hz)"
			// range and outside the dialect's ASSUMED ceiling of 9990, the
			// largest multiple of the ASSUMED 10 Hz step inside it. The
			// bound is consulted from the dialect, in the comparison and in
			// the message alike (the shared register's ClarifierPolicy
			// entry).
			name: "a clarifier beyond this dialect's declared maximum",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ClarHz = 10_000
			}),
			fields: []spec.Field{spec.FieldClarifier},
		},
		{
			// TWO inputs land in buildWriteCommand's UNFIELDED catch-all
			// (task 11 review, LOW-2; it was three until the closing
			// review's O-L3 gave ModeUnset its own named rung below): every
			// adjacent refusal names a spec.Field and this one does not,
			// because BuildMTSetCombined's error carries no field of its
			// own. Pinned here AS IT STANDS TODAY — no Fields — so the
			// shape does not drift unnoticed; a fielded catch-all is a
			// fleet follow-up, not this task's.
			//
			// 155 Hz is inside the dialect's declared +/-9990 Hz ceiling and
			// not a multiple of the ASSUMED 10 Hz step, so it clears the
			// magnitude rung above and meets the builder's own step check.
			name: "a clarifier that is not a multiple of this dialect's step",
			ch: withData(func(d *codeplug.ChannelData) {
				d.ClarHz = 155
			}),
			reason: "must be a multiple of",
		},
		{
			// Same catch-all: a tag past this dialect's TagMaxBytes (12)
			// clears every rung above (Tag carries no FieldState and no
			// vocabulary of its own) and meets only the builder's length
			// check.
			name: "a tag longer than this dialect admits",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Tag = "THIRTEEN BYTE"
			}),
			reason: "tag must be 0-12 bytes",
		},
		{
			// NOT the catch-all: the closing review's O-L3. The mode rung's
			// OWN ok check cannot catch this — dialect.ModeByName("-")
			// answers ok=true, it being cat.ModeUnset, the SHARED
			// register's own ASSUMED member — so until this rung existed
			// the value fell through to the builder's Set-frame check and
			// was refused with NO Fields, making the one mode-shaped
			// refusal a caller can actually trip the one whose report named
			// no field. It is refused BY NAME in the mode rung now, still
			// pre-wire, and the builder keeps its own check behind it.
			name: "a mode that resolves to cat.ModeUnset",
			ch: withData(func(d *codeplug.ChannelData) {
				d.Mode = "-"
			}),
			fields: []spec.Field{spec.FieldMode},
			reason: "must not be ModeUnset",
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

// TestWriteChannel_FrequencyAgainstTheDeclaredRange walks BOTH ends of this
// driver's own declared frequency range, one hertz inside and one hertz
// outside each.
//
// The closing review's C-H1: `cat.MemoryFreqHz` bounds the nine-digit
// ENCODING (999 999 999) and nothing bounded the RADIO, so before the rung
// this test pins existed, `Session.WriteChannel` — which is public, and does
// NOT call `codeplug.Validate` — put 470 000 001 Hz on the wire, one hertz
// above the ceiling this very driver declares. The clone service's callers
// are safe by a DIFFERENT route (`clone.PrepareSend` runs `codeplug.Validate`,
// which refuses the range), and that route is not this driver's to rely on:
// the bound is the DRIVER register's own entry "MinFreqHz 30 000 /
// MaxFreqHz 470 000 000 — THE FA/FB RANGE READ AS THE MEMORY-STORABLE
// RANGE", so it is consulted where its datum lives.
//
// Both bounds are read from THIS SESSION'S caps in the assertions too, not
// restated as literals: a caps edit must move the refusal, not the test's
// idea of it. The four frequencies themselves ARE literals, because they are
// what the register entry claims.
func TestWriteChannel_FrequencyAgainstTheDeclaredRange(t *testing.T) {
	for _, tt := range []struct {
		name    string
		freqHz  uint64
		refused bool
	}{
		{name: "the declared floor is written", freqHz: 30_000},
		{name: "one hertz below the declared floor is refused", freqHz: 29_999, refused: true},
		{name: "the declared ceiling is written", freqHz: 470_000_000},
		{name: "one hertz above the declared ceiling is refused", freqHz: 470_000_001, refused: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, sess := openSession(t, Simulated, slotImage{})
			before := len(p.Transcript())
			if tt.freqHz == 30_000 && sess.caps.MinFreqHz != 30_000 {
				t.Fatalf("caps.MinFreqHz = %d, want 30000 — this test's four literals are the register entry's", sess.caps.MinFreqHz)
			}
			if tt.freqHz == 470_000_000 && sess.caps.MaxFreqHz != 470_000_000 {
				t.Fatalf("caps.MaxFreqHz = %d, want 470000000 — this test's four literals are the register entry's", sess.caps.MaxFreqHz)
			}

			ch := withData(func(d *codeplug.ChannelData) { d.FreqHz = tt.freqHz })
			res, err := sess.WriteChannel(testCtx(t), ch)
			sent := p.Transcript()[before:]

			if !tt.refused {
				if err != nil {
					t.Fatalf("WriteChannel = %v, want nil — %d Hz is inside the declared range", err, tt.freqHz)
				}
				if len(sent) != 1 {
					t.Fatalf("wire carried %q, want exactly one frame", sent)
				}
				// Positions 6-14 of the 41-byte chart: P2, nine digits.
				if got, want := sent[0][5:14], fmt.Sprintf("%09d", tt.freqHz); got != want {
					t.Errorf("frame's P2 = %q, want %q (frame %q)", got, want, sent[0])
				}
				return
			}

			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("WriteChannel = %v (%T) and the wire carried %q, want a *driver.WriteRefusedError and nothing sent — %d Hz is outside %d..%d", err, err, sent, tt.freqHz, sess.caps.MinFreqHz, sess.caps.MaxFreqHz)
			}
			if want := []spec.Field{spec.FieldFrequency}; !reflect.DeepEqual(wre.Fields, want) {
				t.Errorf("WriteRefusedError.Fields = %v, want %v", wre.Fields, want)
			}
			if !strings.Contains(wre.Reason, "outside this radio's") {
				t.Errorf("WriteRefusedError.Reason = %q, want it to name the radio's declared range", wre.Reason)
			}
			if res.Steps == nil || len(res.Steps) != 0 {
				t.Errorf("WriteResult.Steps = %#v, want an EMPTY, non-nil slice — nothing was attempted", res.Steps)
			}
			if len(sent) != 0 {
				t.Errorf("the refusal sent %q — this rung is PRE-WIRE", sent)
			}
		})
	}
}

// TestWriteChannel_CapabilityGateAnswersFirstOnUnconsentedRealHardware is
// decisions.md cell 10 (the fleet ladder order, ruled) from the side that
// makes the order observable.
//
// The channel here is wrong in a way the BUILDERS would catch — a mode this
// radio's legend does not print — and on an unconsented RealHardware session
// the answer is not that refusal: writeTrialsComplete is false, so every
// field is Unverified, nothing is writable, and the capability gate refuses
// first, naming every field the write requested. While that constant is
// false this is EVERY write to a real FT-991A (matrix §3.11).
//
// The requested set is exactly the six plain fields: nothing else on this
// channel is Known, and spec.FieldTagDisplay is not among them because it is
// Unavailable — so the gate complains about the fields the caller actually
// asked to write rather than about one nobody asked for.
func TestWriteChannel_CapabilityGateAnswersFirstOnUnconsentedRealHardware(t *testing.T) {
	p, sess := openSession(t, RealHardware, slotImage{})
	before := len(p.Transcript())

	ch := withData(func(d *codeplug.ChannelData) { d.Mode = "PSK" })
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
		t.Errorf("WriteRefusedError.Fields = %v, want %v — the CAPABILITY gate answers before the builders (decisions.md cell 10)", wre.Fields, want)
	}
	if !strings.Contains(wre.Reason, "not write-Supported") {
		t.Errorf("WriteRefusedError.Reason = %q, want the capability gate's reason, not the builders'", wre.Reason)
	}
	if res.Steps == nil || len(res.Steps) != 0 {
		t.Errorf("WriteResult.Steps = %#v, want an EMPTY, non-nil slice", res.Steps)
	}
	if after := p.Transcript(); len(after) != before {
		t.Errorf("the refusal sent %v — the gate is pre-wire", after[before:])
	}
}

// TestWriteChannel_ConsentedRealHardwareMeetsTheBuilders is the other half
// of the ordering: consent is the ONE thing that opens the capability gate
// on a RealHardware session (spec.ConsentUnverifiedWrites, applied in
// sessionCapabilities), and once it is open the SAME channel meets the
// refusal that names the real problem.
//
// Consent is a decision about RISK, not evidence: it lets a user write a
// field this project has never proven against a radio. It does not and must
// not reach past a refusal grounded in the radio's own printed legends,
// which is what the mode refusal is.
//
// An unconsented session cannot show this, and that asymmetry is the whole
// point of pinning both directions.
func TestWriteChannel_ConsentedRealHardwareMeetsTheBuilders(t *testing.T) {
	p, sess := openSession(t, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())
	before := len(p.Transcript())

	ch := withData(func(d *codeplug.ChannelData) { d.Mode = "PSK" })
	_, err := sess.WriteChannel(testCtx(t), ch)
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel = %v (%T), want a *driver.WriteRefusedError", err, err)
	}
	if want := []spec.Field{spec.FieldMode}; !reflect.DeepEqual(wre.Fields, want) {
		t.Errorf("WriteRefusedError.Fields = %v, want %v — with the gate open, the builders are what this channel meets", wre.Fields, want)
	}
	if after := p.Transcript(); len(after) != before {
		t.Errorf("the refusal sent %v — the builders' refusals are pre-wire too", after[before:])
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
// That a rejected Set draws exactly one "?;" is ASSUMED — the SHARED
// register's THE ACKNOWLEDGEMENT CONVENTIONS entry, whose lift is one write
// session's raw transcript. And on THIS radio "?;" carries a second reading
// on a different command (an MT READ of an empty slot, the driver register's
// own entry); the two must not be conflated, which is why the write path
// reports a rejection and never "the slot is empty".
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

// TestWriteChannel_UnexpectedFrameAfterASetIsCountedNotFatal is the THIRD
// thing a radio could say to a Set, and the one no self-consistent fake can
// produce: neither the silence that means accepted nor the "?;" that means
// rejected, but some other frame arriving inside the transport's error
// window.
//
// The engine counts it (transport safety obligation 3) and the write still
// succeeds, because nothing rejected it. That is the right answer under the
// ASSUMED convention — silence-or-"?;" is the whole vocabulary the register
// claims, so an unrecognised frame cannot be read as a refusal without
// inventing a third meaning — and the count is how a real transcript would
// show that the assumption needs revisiting.
func TestWriteChannel_UnexpectedFrameAfterASetIsCountedNotFatal(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{junkAfterSet: "ZZ0;"})
	if got := sess.Diagnostics().UnexpectedFrames; got != 0 {
		t.Fatalf("Diagnostics().UnexpectedFrames = %d before the write, want 0", got)
	}
	before := len(p.Transcript())

	res, err := sess.WriteChannel(testCtx(t), writableChannel())
	if err != nil {
		t.Fatalf("WriteChannel = %v, want nil — an unrecognised frame is not a rejection", err)
	}
	if want := []driver.WriteStep{{Command: "MT", Sent: true, Confirmed: true}}; !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("WriteResult.Steps = %+v, want %+v", res.Steps, want)
	}
	if got := p.Transcript()[before:]; len(got) != 1 {
		t.Errorf("wire carried %v, want exactly one Set", got)
	}
	if got := sess.Diagnostics().UnexpectedFrames; got != 1 {
		t.Errorf("Diagnostics().UnexpectedFrames = %d, want 1 — the frame must be COUNTED even though it did not fail the write", got)
	}
}

// TestWriteChannel_TransportFailureLeavesSentFalse: when the transport
// itself fails, the frame's fate is NOT attributable — the host cannot tell
// whether it reached the radio — so Sent stays false and the error, not the
// flags, carries the distinction.
//
// THIS IS THE WRITE PATH'S ANALOGUE OF THE READ'S TIMEOUT ROW, and it is not
// a timeout: transport.CATWriteSpec is ClassWrite, for which silence IS the
// success signal, so no amount of silence can time a Set out. The
// unattributable outcome has to come from the transport failing outright,
// which is what closing the session produces.
//
// The step list is still DECLARED in full: an MT step present but never Sent
// says "this write intended one MT frame and it never provably went out",
// which a caller journaling the result can act on; an empty list would be
// indistinguishable from a driver that intended nothing.
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
	if !strings.Contains(err.Error(), "001") {
		t.Errorf("error %q does not name the slot — every error on this path carries it", err)
	}
	want := []driver.WriteStep{{Command: "MT", Sent: false, Confirmed: false}}
	if !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("WriteResult.Steps = %+v, want %+v — an unattributable frame is not Sent", res.Steps, want)
	}
}

// TestDCSState_AcceptedHereAndRefusedAtAllThreeGatesOnASibling is the seam
// this radio exists to exercise (matrix §3.7), walked end to end.
//
// ONE record, ONE 41-byte frame, and TWO dialects that differ on exactly ONE
// axis. The FTdx10 is the sibling chosen deliberately: it declares
// cat.MTFormCombined, cat.P11Fixed, cat.P5TxClar, TagMaxBytes 12 and
// memory slots 001-099 — identical to this radio on every axis the frame
// touches EXCEPT cat.ToneStates, where it is ToneStatesCTCSS and this radio
// is ToneStatesCTCSSAndDCS. The FT-891 would be the wrong control: its
// P11TagDisplay and P5Fixed would make the display-less builder and a
// TxClar-true record refuse for reasons that have nothing to do with P8.
//
// THE THREE GATES ARE THE THREE SITES matrix §3.7 enumerates, and all three
// route through the RECEIVER Dialect.ParseCTCSSState rather than the
// package-level cat.ParseCTCSSState, which S0.3 retained unchanged at three
// states for its external callers:
//
//  1. the CODEC — parseMemoryFields, reached through ParseMTAnswerCombined;
//  2. the BUILDER — validateCombinedMTFields, reached through
//     BuildMTSetCombined;
//  3. the OUTBOUND WRITE GATE — AllowedCommand, which every frame this
//     driver's engine sends is checked against before it reaches the wire.
//
// Two of the three are the write gate, which is what made the five-state
// vocabulary a three-way change rather than a one-site one.
//
// WHAT IS ASSUMED AND WHAT IS NOT: that the FT-991A's P8 legend PRINTS five
// states is transcribed (layout 795-796, 977-978, 1010-1011, 1048-1049,
// 1128-1129). That the radio ACCEPTS a DCS state written with no CN code
// sent first is the SHARED register's THE DCS STATES' SET ACCEPTANCE entry,
// and what the radio does with the code it already holds is the DRIVER
// register's A DCS-STATE CHANNEL'S CODE SURVIVES A REWRITE entry. No test
// can settle either.
func TestDCSState_AcceptedHereAndRefusedAtAllThreeGatesOnASibling(t *testing.T) {
	sibling := ftdx10.Dialect()
	if sibling.ToneStates() != cat.ToneStatesCTCSS {
		t.Fatalf("the control dialect declares %v, want ToneStatesCTCSS — this test asserts nothing unless the sibling really is three-state", sibling.ToneStates())
	}
	if catDialect.ToneStates() != cat.ToneStatesCTCSSAndDCS {
		t.Fatalf("this radio's dialect declares %v, want ToneStatesCTCSSAndDCS", catDialect.ToneStates())
	}

	// The record, and its frame re-derived BY HAND from the position chart
	// exactly as TestWriteChannel_OneCombinedMTSetFrame's are: slot 001,
	// 145.500 MHz, no clarifier, both clarifier flags off, FM, the Set's
	// form constant, P8 '3' (DCS ENC/DEC), simplex, tag "DCS".
	//
	//	MT|001|145500000|+|0000|0|0|4|0|3|00|0|0|DCS_________|;
	const dcsFrame = "MT001145500000+0000004030000DCS         ;"
	if len(dcsFrame) != 41 {
		t.Fatalf("the hand-derived frame is %d bytes, not the chart's 41", len(dcsFrame))
	}
	slot, err := catDialect.ParseSlot("001")
	if err != nil {
		t.Fatalf("ParseSlot: %v", err)
	}
	record := cat.MemoryData{
		Slot:   slot,
		FreqHz: 145_500_000,
		Mode:   cat.Mode('4'),
		Kind:   cat.CombinedMTSetKind,
		CTCSS:  cat.CTCSSDCSEncDec,
		Shift:  cat.ShiftSimplex,
	}

	t.Run("gate 1: the codec", func(t *testing.T) {
		m, tag, err := catDialect.ParseMTAnswerCombined([]byte(dcsFrame))
		if err != nil {
			t.Fatalf("this radio's parse of a DCS record = %v, want nil", err)
		}
		if m.CTCSS != cat.CTCSSDCSEncDec || tag != "DCS" {
			t.Errorf("parsed CTCSS %q tag %q, want %q / \"DCS\"", m.CTCSS, tag, cat.CTCSSDCSEncDec)
		}
		_, _, err = sibling.ParseMTAnswerCombined([]byte(dcsFrame))
		if err == nil {
			t.Fatal("the ToneStatesCTCSS sibling PARSED a DCS record — its P8 domain is '0'-'2'")
		}
		var pe *cat.ParseError
		if !errors.As(err, &pe) {
			t.Errorf("the sibling's refusal %v (%T) is not a *cat.ParseError", err, err)
		}
		if !strings.Contains(err.Error(), "P8") {
			t.Errorf("the sibling's refusal %q does not name P8 — it must fail on the tone state, not on some other field", err)
		}
	})

	t.Run("gate 2: the builder", func(t *testing.T) {
		cmd, err := catDialect.BuildMTSetCombined(record, "DCS")
		if err != nil {
			t.Fatalf("this radio's BuildMTSetCombined = %v, want nil", err)
		}
		if got := string(cmd.Bytes()); got != dcsFrame {
			t.Errorf("built %q\nwant    %q", got, dcsFrame)
		}
		siblingRecord := record
		siblingSlot, err := sibling.ParseSlot("001")
		if err != nil {
			t.Fatalf("the sibling's ParseSlot: %v", err)
		}
		siblingRecord.Slot = siblingSlot
		if _, err := sibling.BuildMTSetCombined(siblingRecord, "DCS"); err == nil {
			t.Fatal("the ToneStatesCTCSS sibling BUILT a DCS record")
		} else if !strings.Contains(err.Error(), "P8") {
			t.Errorf("the sibling's refusal %q does not name P8", err)
		}
	})

	t.Run("gate 3: the outbound write gate", func(t *testing.T) {
		if !catDialect.AllowedCommand([]byte(dcsFrame)) {
			t.Error("this radio's outbound gate REFUSED a DCS record it must send")
		}
		if sibling.AllowedCommand([]byte(dcsFrame)) {
			t.Error("the ToneStatesCTCSS sibling's outbound gate ADMITTED a DCS record — the gate is the last thing between a wrong byte and the wire")
		}
		// POSITIVE CONTROL (task 11 review, LOW-3): without this, the refusal
		// above would pass just as well if the sibling's gate refused every
		// MT frame wholesale. Swap P8's DCS ENC/DEC ('3', position 24) for
		// the sibling's own ENC/DEC ('1') and confirm its gate ADMITS the
		// otherwise-identical, ordinary CTCSS frame.
		ctcssFrame := dcsFrame[:23] + "1" + dcsFrame[24:]
		if !sibling.AllowedCommand([]byte(ctcssFrame)) {
			t.Error("the ToneStatesCTCSS sibling's outbound gate refused an ordinary CTCSS record — gate 3's DCS refusal above would be vacuous if the gate refused every MT frame")
		}
	})

	t.Run("and the driver writes one, byte for byte", func(t *testing.T) {
		p, sess := openSession(t, Simulated, slotImage{})
		before := len(p.Transcript())

		ch := withData(func(d *codeplug.ChannelData) {
			d.ClarHz, d.RxClar, d.TxClar = 0, false, false
			d.CTCSS = "DCS-ENC-DEC"
			d.Shift = "SIMPLEX"
			d.Tag = "DCS"
		})
		if _, err := sess.WriteChannel(testCtx(t), ch); err != nil {
			t.Fatalf("WriteChannel = %v, want nil — all five P8 values are writable here (decisions.md cell 7)", err)
		}
		if got, want := p.Transcript()[before:], []string{dcsFrame}; !reflect.DeepEqual(got, want) {
			t.Errorf("wire carried %q\nwant             %q", got, want)
		}
	})
}

// TestNameMaps_AreExactInverses walks read.go's ctcssNames/shiftNames and
// write.go's ctcssByName/shiftByName in both directions.
//
// Two hand-written maps meant to be exact inverses are worth a test rather
// than an adjacency: a spelling added to one and forgotten in the other
// would silently refuse a legitimate value at the write gate, or — the worse
// direction — map it onto the wrong byte. On THIS radio the risk is
// concentrated in the two DCS spellings, which no sibling map carries and
// which a driver written from a sibling's would simply be missing.
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

	// Non-vacuity: the vocabularies this driver's capability data advertises
	// must be exactly the keys of the write-direction maps, so neither walk
	// above can pass over an empty pair. FIVE and three on this radio, and
	// the five is what makes the CTCSS half non-trivial.
	caps := CapabilitiesSimulated()
	if len(caps.CTCSSStates) != len(ctcssByName) || len(caps.ShiftOptions) != len(shiftByName) {
		t.Fatalf("advertised vocabularies (%d CTCSS states, %d shifts) do not match the write maps (%d, %d)",
			len(caps.CTCSSStates), len(caps.ShiftOptions), len(ctcssByName), len(shiftByName))
	}
	if len(ctcssByName) != 5 {
		t.Errorf("ctcssByName has %d entries, want the FIVE this radio's P8 legend prints", len(ctcssByName))
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
// TAGDISPLAY IS NEVER REQUESTED BY A CHANNEL THIS DRIVER READ, because this
// radio has no display flag and read.go reports it Unavailable — which is
// exactly the inversion of the FT-891, where it is Known on every read and
// therefore always seventh. The conditional is kept all the same, so that a
// caller who explicitly marks it Known is REFUSED by the capability gate
// rather than silently dropped.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	plain := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier,
		spec.FieldCTCSSState, spec.FieldShift, spec.FieldTag,
	}

	t.Run("a channel this driver read requests exactly the six", func(t *testing.T) {
		if got := requestedFields(*writableChannel().Data); !reflect.DeepEqual(got, plain) {
			t.Errorf("requestedFields = %v, want %v", got, plain)
		}
	})

	t.Run("a Known TagDisplay is seventh", func(t *testing.T) {
		d := *writableChannel().Data
		d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
		want := append(append([]spec.Field(nil), plain...), spec.FieldTagDisplay)
		if got := requestedFields(d); !reflect.DeepEqual(got, want) {
			t.Errorf("requestedFields = %v, want %v", got, want)
		}
	})

	t.Run("the tone and skip conditionals follow it, then the tier", func(t *testing.T) {
		d := *writableChannel().Data
		d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
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
			if tr.Present(*writableChannel().Data) {
				t.Errorf("%s is requested by an ordinary FT-991A channel — every tier field reads back Unavailable on this radio", tr.Field)
			}
		}
	})

	// The seventeen must be exactly the seventeen fields codeplug's own
	// tierAddedFieldFor carries, in the same order — and since
	// tierAddedFieldFor is unexported (see tierRequestedFields' doc comment
	// for what codeplug DOES export), this derives the same seventeen
	// independently, from spec.AllFields() minus the ten pre-tier fields,
	// and requires EXACT membership and order against that derivation. A
	// tier field this table forgets to mirror — or that the codeplug side
	// gains and this one does not — fails HERE rather than passing silently
	// until a caller hits the hole. The FT-891 shipped that hole for a
	// while, which is why the pin exists from this driver's first day.
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
			gotTier[i] = tr.Field
		}
		if !reflect.DeepEqual(gotTier, wantTier) {
			t.Errorf("tierRequestedFields names\n %v\nbut spec.AllFields() carries the tier-added\n %v\n(the two must be the same seventeen fields in the same order)", gotTier, wantTier)
		}
	})
}

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
// IT IS A SEPARATE FUNCTION FROM read.go's mtSpec rather than a reuse of it,
// and on this radio the confusion would be easy to make: the read's spec
// pins the combined ANSWER's exact geometry from the dialect, and the Set
// and the Answer are the SAME 41 positions under the SAME prefix here — so a
// Set sent under the read's spec would look entirely plausible right up to
// the timeout on every accepted write.
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

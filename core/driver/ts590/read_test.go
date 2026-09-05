// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestParseSlotID is the P7 ladder's first rung on its own: SYNTAX, and no
// row. It must not consult a slot space, because a refusal that depended on
// the row would refuse the TS-590SG's 110-119 by one mechanism and the
// TS-590S's by another, and a user would see a different message depending on
// which sibling they own.
func TestParseSlotID(t *testing.T) {
	for _, tc := range []struct {
		id     string
		number int
		half   kw.ScanHalf
		bad    bool
	}{
		{id: "000", number: 0},
		{id: "042", number: 42},
		{id: "099", number: 99},
		{id: "100L", number: 100, half: kw.ScanLower},
		{id: "109U", number: 109, half: kw.ScanUpper},
		// Syntactically fine on BOTH rows; membership is what refuses them.
		{id: "115", number: 115},
		{id: "999", number: 999},
		{id: "", bad: true},
		{id: "12", bad: true},
		{id: "0042", bad: true},
		{id: "100X", bad: true},
		{id: "10AL", bad: true},
		{id: "100LL", bad: true},
	} {
		number, half, err := parseSlotID(tc.id)
		if tc.bad {
			if err == nil {
				t.Errorf("parseSlotID(%q) succeeded, want a refusal", tc.id)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSlotID(%q): %v", tc.id, err)
			continue
		}
		if number != tc.number || half != tc.half {
			t.Errorf("parseSlotID(%q) = %d/%v, want %d/%v", tc.id, number, half, tc.number, tc.half)
		}
	}
}

// TestParseSlotID_IsTheInverseOfWhatTheBanksPublish is the pin that keeps the
// renderer and the reader one datum: every identifier in every bank of every
// row parses back to the number and half kw.Slot.String rendered it from.
func TestParseSlotID_IsTheInverseOfWhatTheBanksPublish(t *testing.T) {
	for _, row := range bothRows {
		l, _ := layoutFor(row)
		for _, b := range CapabilitiesUnverified(row).Banks {
			for _, id := range b.Slots {
				number, half, err := parseSlotID(id)
				if err != nil {
					t.Fatalf("%s: bank %s publishes %q, which parseSlotID refuses: %v", modelNameFor(row), b.ID, id, err)
				}
				slot, err := l.NewSlot(number, half)
				if err != nil {
					t.Fatalf("%s: %q does not resolve against this row's slot space: %v", modelNameFor(row), id, err)
				}
				if got := slot.String(); got != id {
					t.Errorf("%s: %q round-trips to %q", modelNameFor(row), id, got)
				}
			}
		}
	}
}

// TestReadChannel_APopulatedMEMChannel maps one ordinary record field by
// field, on both rows, and runs the fleet's fresh-read helper over it.
func TestReadChannel_APopulatedMEMChannel(t *testing.T) {
	for _, row := range bothRows {
		sess, p := openTestSession(t, row, radioImage{
			mrAnswers: map[string]string{mrAddr("042"): populatedMR("042")},
		})
		ch, err := sess.ReadChannel(context.Background(), "042")
		if err != nil {
			t.Fatalf("%s: ReadChannel: %v", modelNameFor(row), err)
		}
		if ch.Slot != "042" || ch.Data == nil {
			t.Fatalf("%s: ReadChannel = %+v, want a populated 042", modelNameFor(row), ch)
		}
		// ONE frame, and it is an MR read with P1=0 (P13).
		want := []string{"AI0;", "ID;", "FV;", "MR0042;"}
		if got := p.Transcript(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: transcript = %v, want %v", modelNameFor(row), got, want)
		}

		filter := codeplug.StringField{State: codeplug.Unavailable}
		if row == RowSG {
			filter = codeplug.StringField{State: codeplug.Known, Value: "FILTER A"}
		}
		wantData := codeplug.ChannelData{
			FreqHz: 145_500_000,
			// P5 = '4' FM with P14 "00" FM Normal — the synthesis, not a
			// transcription (590:1358, 590:1569-1571).
			Mode:       "FM",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
			Tag:        "SIMPLEX",
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Known, Value: false},
			// A9: what an MR with P1=1 answers on a simplex channel is
			// unprinted, so a fresh read learns nothing about the TX side.
			TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
			Duplex:              codeplug.StringField{State: codeplug.Unavailable},
			OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
			ToneMode:            codeplug.StringField{State: codeplug.Known, Value: "TONE"},
			ToneTx:              codeplug.ToneField{State: codeplug.Known, Value: 885},
			ToneRx:              codeplug.ToneField{State: codeplug.Known, Value: 885},
			DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
			DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
			Filter:              filter,
			DataMode:            codeplug.BoolField{State: codeplug.Known, Value: false},
			TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
			TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
			ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
			AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
			Preamp:              codeplug.StringField{State: codeplug.Unavailable},
			Antenna:             codeplug.StringField{State: codeplug.Unavailable},
			IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
		}
		if !reflect.DeepEqual(*ch.Data, wantData) {
			t.Errorf("%s: ChannelData =\n %+v\nwant\n %+v", modelNameFor(row), *ch.Data, wantData)
		}
		drivertest.AssertFreshReadSaveLoad(t, ch, sess.Capabilities(), codeplug.Load)
	}
}

// TestReadChannel_TheFieldsThatVaryWithTheRECORD walks the record bytes this
// mapper reads, so that each mapping is pinned by a value that differs from
// the last rather than by one unremarkable channel.
func TestReadChannel_TheFieldsThatVaryWithTheRECORD(t *testing.T) {
	for _, tc := range []struct {
		name   string
		edit   func(*recordFields)
		assert func(*testing.T, *codeplug.ChannelData)
	}{
		{
			// P14 "01" is FM Narrow, and the mode NAME is the synthesis of
			// P5 x P14 (590:1569-1571). Nine names over eight nibbles.
			name: "FM Narrow is synthesised from P5 and P14",
			edit: func(f *recordFields) { f.p14 = "01" },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Mode != "FM-N" {
					t.Errorf("Mode = %q, want FM-N", d.Mode)
				}
			},
		},
		{
			// A non-FM record's P14 is not read into the name at all: the
			// synthesis is keyed on the FM nibble.
			name: "a non-FM mode is named from P5 alone",
			edit: func(f *recordFields) { f.mode = '2'; f.p14 = "00" },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Mode != "USB" {
					t.Errorf("Mode = %q, want USB", d.Mode)
				}
			},
		},
		{
			// P6, "refer to the DA command" (590:1546-1548) — the byte the
			// TS-480 spends on its channel lockout instead.
			name: "byte 19 is the data mode on these rows",
			edit: func(f *recordFields) { f.b19 = '1' },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.DataMode != (codeplug.BoolField{State: codeplug.Known, Value: true}) {
					t.Errorf("DataMode = %+v, want Known true", d.DataMode)
				}
			},
		},
		{
			// P15 at byte 41 (590:1572-1574) — the 480 carries its lockout
			// at byte 19, which is the mirror image.
			name: "byte 41 is the channel lockout on these rows",
			edit: func(f *recordFields) { f.b41 = '1' },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.ScanSkip != (codeplug.BoolField{State: codeplug.Known, Value: true}) {
					t.Errorf("ScanSkip = %+v, want Known true", d.ScanSkip)
				}
			},
		},
		{
			// The two indices are INDEPENDENT, which is what makes Cross
			// Tone expressible (590:1167-1171). Index 42 is TN's own top,
			// 1750 Hz, and CN has no equivalent — so P8 may carry it and P9
			// may not (core/kw bounds P9 at 41).
			name: "cross tone carries two different indices",
			edit: func(f *recordFields) { f.tone = '3'; f.p8 = "42"; f.p9 = "41" },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.ToneMode.Value != "CROSS" {
					t.Errorf("ToneMode = %+v, want CROSS", d.ToneMode)
				}
				if d.ToneTx.Value != 17500 {
					t.Errorf("ToneTx = %v, want 1750.0 Hz (TN index 42)", d.ToneTx.Value)
				}
				if d.ToneRx.Value != 2541 {
					t.Errorf("ToneRx = %v, want 254.1 Hz (CN index 41)", d.ToneRx.Value)
				}
			},
		},
		{
			name: "tone OFF still carries both indices, because the record does",
			edit: func(f *recordFields) { f.tone = '0'; f.p8 = "00"; f.p9 = "00" },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.ToneMode.Value != "OFF" {
					t.Errorf("ToneMode = %+v, want OFF", d.ToneMode)
				}
				if d.ToneTx.State != codeplug.Known || d.ToneTx.Value != 670 {
					t.Errorf("ToneTx = %+v, want Known 67.0 Hz", d.ToneTx)
				}
			},
		},
		{
			// A1: the name is right-trimmed of the spaces a write pads it
			// with. Eight characters is the printed maximum (590:1576).
			name: "an eight-character name survives whole",
			edit: func(f *recordFields) { f.name = "GB3IN-DV" },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Tag != "GB3IN-DV" {
					t.Errorf("Tag = %q, want the whole eight characters", d.Tag)
				}
			},
		},
		{
			name: "an empty name comes back empty rather than as spaces",
			edit: func(f *recordFields) { f.name = "" },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Tag != "" {
					t.Errorf("Tag = %q, want the empty string (A1's right-trim)", d.Tag)
				}
			},
		},
		{
			// MC's own space convention, which MR and MW inherit by A10:
			// "For a response command, a space is entered for a channel
			// number less than 100" (590:1334-1337). Most of a radio's
			// channels answer this way.
			name: "byte 4 may be MC's printed SPACE below channel 100",
			edit: func(f *recordFields) { f.p2 = ' ' },
			assert: func(t *testing.T, d *codeplug.ChannelData) {
				if d.FreqHz != 145_500_000 {
					t.Errorf("FreqHz = %d, want the record's own", d.FreqHz)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, row := range bothRows {
				f := populatedFields("042")
				tc.edit(&f)
				sess, _ := openTestSession(t, row, radioImage{
					mrAnswers: map[string]string{mrAddr("042"): f.frame()},
				})
				ch, err := sess.ReadChannel(context.Background(), "042")
				if err != nil {
					t.Fatalf("%s: ReadChannel: %v", modelNameFor(row), err)
				}
				if ch.Data == nil {
					t.Fatalf("%s: ReadChannel returned an empty channel", modelNameFor(row))
				}
				tc.assert(t, ch.Data)
			}
		})
	}
}

// TestReadChannel_Byte28IsAcceptedAsZeroOrOneOnBOTHRows is Q12's read half
// and matrix §2.7's second bullet, and the SECOND row is the one that
// matters: the TS-590S's byte 28 is "always 0" only at firmware 1.xx
// (590:1478), so an S at >= 2.00 using FILTER B answers '1' — and REQUIRING
// '0' there would make that radio fail the WHOLE channel read.
//
// The SG additionally PUBLISHES the value; the S accepts it and reports the
// field Unavailable, because a static per-model capability table cannot say
// "Supported iff FV >= 2.00" and reporting a Known value under an Unsupported
// grade would offer a column the write path must refuse.
func TestReadChannel_Byte28IsAcceptedAsZeroOrOneOnBOTHRows(t *testing.T) {
	for _, b28 := range []byte{'0', '1'} {
		for _, row := range bothRows {
			f := populatedFields("042")
			f.b28 = b28
			sess, _ := openTestSession(t, row, radioImage{
				mrAnswers: map[string]string{mrAddr("042"): f.frame()},
			})
			ch, err := sess.ReadChannel(context.Background(), "042")
			if err != nil {
				t.Fatalf("%s with byte 28 %q: ReadChannel: %v", modelNameFor(row), b28, err)
			}
			want := codeplug.StringField{State: codeplug.Unavailable}
			if row == RowSG {
				want = codeplug.StringField{State: codeplug.Known, Value: map[byte]string{'0': "FILTER A", '1': "FILTER B"}[b28]}
			}
			if ch.Data.Filter != want {
				t.Errorf("%s with byte 28 %q: Filter = %+v, want %+v", modelNameFor(row), b28, ch.Data.Filter, want)
			}
		}
	}
}

// TestReadChannel_AnUnparseableFVStillReadsTheWholeRadio is the design's
// asymmetry carried to its consequence: an FV this programme cannot read as
// A13's M.NN form disables nothing on the read path, on either row.
func TestReadChannel_AnUnparseableFVStillReadsTheWholeRadio(t *testing.T) {
	for _, row := range bothRows {
		answers := map[string]string{}
		var ids []string
		for _, b := range CapabilitiesUnverified(row).Banks {
			for _, id := range b.Slots {
				ids = append(ids, id)
				answers[mrAddr(id)] = populatedMR(id)
			}
		}
		sess, _ := openTestSession(t, row, radioImage{fvAnswer: "FV????;", mrAnswers: answers})
		if sess.fvGrammarOK {
			t.Fatalf("%s: the fixture's FV parsed after all", modelNameFor(row))
		}
		if len(ids) != 120 {
			t.Fatalf("%s: %d published slots, want 120 (100 MEM + 20 SCAN)", modelNameFor(row), len(ids))
		}
		for _, id := range ids {
			ch, err := sess.ReadChannel(context.Background(), id)
			if err != nil {
				t.Fatalf("%s: whole-radio read failed at %q: %v", modelNameFor(row), id, err)
			}
			if ch.Slot != id || ch.Data == nil {
				t.Fatalf("%s: read of %q = %+v", modelNameFor(row), id, ch)
			}
		}
	}
}

// TestReadChannel_TheScanPairIsTwoFramesAndNeitherIsATransmitFrequency is
// P12: "1NNL" reads MR P1=0, the printed START frequency, and "1NNU" reads
// MR P1=1, the printed END frequency (590:1449-1451). Both are ordinary
// FieldFrequency values, and TxFreqHz is not a value in either.
func TestReadChannel_TheScanPairIsTwoFramesAndNeitherIsATransmitFrequency(t *testing.T) {
	for _, row := range bothRows {
		lower := populatedFields("100L")
		lower.freq = "00007000000"
		upper := populatedFields("100U")
		upper.freq = "00007200000"
		sess, p := openTestSession(t, row, radioImage{mrAnswers: map[string]string{
			mrAddr("100L"): lower.frame(),
			mrAddr("100U"): upper.frame(),
		}})

		for _, tc := range []struct {
			id    string
			frame string
			hz    uint64
		}{
			{"100L", "MR0100;", 7_000_000},
			{"100U", "MR1100;", 7_200_000},
		} {
			ch, err := sess.ReadChannel(context.Background(), tc.id)
			if err != nil {
				t.Fatalf("%s: ReadChannel(%q): %v", modelNameFor(row), tc.id, err)
			}
			if ch.Data.FreqHz != tc.hz {
				t.Errorf("%s: %q FreqHz = %d, want %d", modelNameFor(row), tc.id, ch.Data.FreqHz, tc.hz)
			}
			if ch.Data.TxFreqHz.State != codeplug.Unavailable {
				t.Errorf("%s: %q TxFreqHz = %+v; P1=1 here is the END frequency, not a transmit frequency (M-E2)", modelNameFor(row), tc.id, ch.Data.TxFreqHz)
			}
		}
		want := []string{"AI0;", "ID;", "FV;", "MR0100;", "MR1100;"}
		if got := p.Transcript(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: transcript = %v, want %v — P1 comes from the SLOT CLASS", modelNameFor(row), got, want)
		}

		// The capability half, in the same test, because the two must agree:
		// the field is graded in MEM and ZERO in SCAN.
		caps := sess.Capabilities()
		mem, _ := caps.Bank(spec.BankMemory)
		scan, _ := caps.Bank(spec.BankScan)
		if mem.Fields[spec.FieldTxFrequency] == (spec.FieldSupport{}) {
			t.Errorf("%s: FieldTxFrequency is the zero support in MEM, where the P1=1 frame IS a transmit frequency (590:1519-1520)", modelNameFor(row))
		}
		if scan.Fields[spec.FieldTxFrequency] != (spec.FieldSupport{}) {
			t.Errorf("%s: FieldTxFrequency is graded in SCAN, where P1=1 is the END frequency (M-E2)", modelNameFor(row))
		}
	}
}

// TestReadChannel_AnEmptyChannelIsNotAFailure is A18a, and on these rows it
// is DOCUMENTARY FACT rather than an assumption: "If the selected channel is
// empty, P4 ~ P15 will be 0 and P16 will be blank" (590:1492-1493).
//
// The empty test is the WHOLE P4-P15 WINDOW, never "P5 is zero" and never
// "the frame was rejected" — a driver that read a zero mode nibble as a parse
// failure would refuse at the first empty channel and fail a whole-radio read
// of a fresh radio.
func TestReadChannel_AnEmptyChannelIsNotAFailure(t *testing.T) {
	for _, row := range bothRows {
		sess, _ := openTestSession(t, row, radioImage{mrAnswers: map[string]string{
			mrAddr("007"):  emptyMR("007"),
			mrAddr("101L"): emptyMR("101L"),
		}})
		for _, id := range []string{"007", "101L"} {
			ch, err := sess.ReadChannel(context.Background(), id)
			if err != nil {
				t.Fatalf("%s: ReadChannel(%q) of an EMPTY channel: %v", modelNameFor(row), id, err)
			}
			if ch.Slot != id {
				t.Errorf("%s: empty read = %+v, want the slot carried through", modelNameFor(row), ch)
			}
			if ch.Data != nil {
				t.Errorf("%s: empty read carries Data %+v, want nil", modelNameFor(row), ch.Data)
			}
		}
	}
}

// TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence is decision 5 and
// P13. "?;" is EITHER "Command syntax was incorrect" OR "Command was not
// executed due to the current status of the transceiver" (590:100-105) —
// indistinguishable — so it is a typed rejection that fails the session read
// whole, and NEVER an empty channel.
func TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence(t *testing.T) {
	for _, row := range bothRows {
		// No mrAnswers entry at all: this image answers "?;".
		sess, p := openTestSession(t, row, radioImage{})
		ch, err := sess.ReadChannel(context.Background(), "042")
		if err == nil {
			t.Fatalf("%s: a \"?;\" read succeeded, returning %+v", modelNameFor(row), ch)
		}
		if !errors.Is(err, transport.ErrRejected) {
			t.Errorf("%s: errors.Is(err, transport.ErrRejected) = false for %v", modelNameFor(row), err)
		}
		var rej *kw.RejectionError
		if !errors.As(err, &rej) {
			t.Fatalf("%s: errors.As(err, **kw.RejectionError) = false for %v", modelNameFor(row), err)
		}
		if rej.Book != kw.Book590 || rej.Command != "MR" {
			t.Errorf("%s: RejectionError = %v/%q, want the 590 book and MR", modelNameFor(row), rej.Book, rej.Command)
		}
		// ONE frame: a rejection is never retried (decision 5).
		want := []string{"AI0;", "ID;", "FV;", "MR0042;"}
		if got := p.Transcript(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: transcript = %v, want %v", modelNameFor(row), got, want)
		}
	}
}

// TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence is P13's other half: the
// books state that the NAK itself is unreliable — "Occasionally, this message
// may not appear due to microprocessor transients in the transceiver"
// (590:106-108) — so silence carries NO information at all and the typed
// error says so.
func TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence(t *testing.T) {
	sess, _ := openTestSession(t, RowSG, radioImage{
		mrSilent: map[string]bool{mrAddr("042"): true},
	})
	ch, err := sess.ReadChannel(context.Background(), "042")
	if err == nil {
		t.Fatalf("a silent read succeeded, returning %+v", ch)
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("errors.Is(err, transport.ErrTimeout) = false for %v", err)
	}
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("errors.As(err, **kw.TimeoutError) = false for %v", err)
	}
	if to.Command != "MR" {
		t.Errorf("TimeoutError.Command = %q, want MR", to.Command)
	}
}

// TestReadChannel_TheSGExtensionChannelsAreNotSlotIDs is the refusal pin
// RULED 05/09/2026 (Stuart decisions row 6): "110", "115" and "119" are NOT
// slot IDs on the TS-590SG row, and a read naming one is refused THE SAME
// SHAPE as any other out-of-domain slot on any row — never dispatched as a
// frame, and with no radio behaviour invented for it.
//
// THE SG IS THE HALF THAT MATTERS. On the TS-590S the codec's own slot space
// stops at 109 (A12), so nothing but the driver's own bank membership could
// refuse them anyway; on the TS-590SG the codec DECLARES 110-119, because the
// book prints them (590:1346-1347) and an "MR0115;" from a front-panel recall
// of E00 must parse. A driver that dispatched on the codec's domain would
// therefore read those ten channels on the SG and refuse them on the S — the
// asymmetry the ruling exists to prevent.
func TestReadChannel_TheSGExtensionChannelsAreNotSlotIDs(t *testing.T) {
	for _, row := range bothRows {
		// The image WOULD answer an extension read, so a driver that sent
		// one would succeed and this test would catch it by that success
		// rather than by a coincidental "?;".
		answers := map[string]string{}
		for _, id := range []string{"110", "115", "119"} {
			answers[mrAddr(id)] = populatedMR(id)
		}
		sess, p := openTestSession(t, row, radioImage{mrAnswers: answers})
		for _, id := range []string{"110", "115", "119", "999"} {
			ch, err := sess.ReadChannel(context.Background(), id)
			if err == nil {
				t.Fatalf("%s: ReadChannel(%q) succeeded, returning %+v", modelNameFor(row), id, ch)
			}
			if !errors.Is(err, ErrUnknownSlot) {
				t.Errorf("%s: ReadChannel(%q): errors.Is(err, ErrUnknownSlot) = false for %v", modelNameFor(row), id, err)
			}
			var unknown *UnknownSlotError
			if !errors.As(err, &unknown) {
				t.Errorf("%s: ReadChannel(%q): errors.As(err, **UnknownSlotError) = false for %v", modelNameFor(row), id, err)
			}
		}
		// NOT DISPATCHED AS A FRAME: nothing beyond the probe went out.
		want := []string{"AI0;", "ID;", "FV;"}
		if got := p.Transcript(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: transcript = %v, want %v — an unpublished slot is refused before any frame is built", modelNameFor(row), got, want)
		}
	}
}

// TestReadChannel_TheCODECStillParsesAnExtensionChannel is the other half of
// the ruling, and it is what makes the refusal above a DRIVER decision rather
// than a codec limitation: the SG layout declares 110-119 (A12's counterpart,
// 590:1346-1347), so an MC answer or an MR answer naming 115 parses. If this
// ever stops being true the refusal above would still pass while the codec had
// quietly narrowed, which is why both halves are pinned.
func TestReadChannel_TheCODECStillParsesAnExtensionChannel(t *testing.T) {
	sg, _ := layoutFor(RowSG)
	rec, err := sg.ParseMRAnswer([]byte(populatedMR("115")))
	if err != nil {
		t.Fatalf("the SG codec refuses an extension-channel answer: %v", err)
	}
	if rec.Slot.Number() != 115 || rec.Slot.Class() != kw.SlotExtension {
		t.Errorf("parsed slot = %d/%v, want 115/SlotExtension", rec.Slot.Number(), rec.Slot.Class())
	}
	s, _ := layoutFor(RowS)
	if _, err := s.ParseMRAnswer([]byte(populatedMR("115"))); err == nil {
		t.Error("the S codec parsed an extension-channel answer; the book gives 110-119 to the SG and never states the S's ceiling (A12)")
	}
}

// TestReadChannel_AnAnswerNamingAnotherSlotIsRefused: the driver refuses to
// map a reply onto the wrong slot rather than storing one channel's content
// under another's identifier.
func TestReadChannel_AnAnswerNamingAnotherSlotIsRefused(t *testing.T) {
	sess, _ := openTestSession(t, RowS, radioImage{mrAnswers: map[string]string{
		mrAddr("042"): populatedMR("043"),
		// A section channel answering the OTHER half of its own pair is the
		// same mistake one level finer, and P1 is what distinguishes them.
		mrAddr("100L"): populatedMR("100U"),
	}})
	for _, tc := range []struct{ requested, answered string }{
		{"042", "043"},
		{"100L", "100U"},
	} {
		_, err := sess.ReadChannel(context.Background(), tc.requested)
		if !errors.Is(err, ErrAnswerMismatch) {
			t.Errorf("ReadChannel(%q) against an answer naming %q: err = %v, want ErrAnswerMismatch", tc.requested, tc.answered, err)
		}
		var mismatch *AnswerMismatchError
		if errors.As(err, &mismatch) && (mismatch.Requested != tc.requested || mismatch.Answered != tc.answered) {
			t.Errorf("AnswerMismatchError = %q/%q, want %q/%q", mismatch.Requested, mismatch.Answered, tc.requested, tc.answered)
		}
	}
}

// TestReadChannel_AShortMRAnswerNeverReachesTheParser is L5's substance, and
// it is TWO facts rather than one.
//
// FIRST, THE CORRELATION FACT. The read spec's matcher is
// kw.PrefixLenMatcher("MR", kw.RecordLen), an EXACT width, so a 49-byte "MR…"
// is not this read's answer at all: it is never delivered, the read times out,
// and the parser is never given the chance to interpret a frame the command
// did not ask for. That ordering is the right one, and this test pins it —
// including that the mis-sized frame did arrive at the host, so the outcome is
// the matcher's decision and not the fixture failing to send.
//
// SECOND, THE WIDTH PREDICATE ITSELF. core/kw's own checkRecordLen is what
// says a memory frame is fifty bytes, and its refusal is typed, carries BOTH
// measured lengths and renders exact text. That contract is asserted here in
// this family's own error vocabulary — see assertKenwoodRecordLengthMismatch,
// whose doc comment records why the fleet helper of the same shape could not
// be the one called.
func TestReadChannel_AShortMRAnswerNeverReachesTheParser(t *testing.T) {
	// FORTY-NINE BYTES: the whole record less its last name byte, then the
	// terminator, so the frame is one byte short of the printed grid and is
	// otherwise well formed.
	short := populatedMR("042")[:kw.RecordLen-2] + ";"
	// BOTH ROWS, because the width predicate is one predicate: core/kw's
	// checkRecordLen is not row-conditional, and running the pair is what
	// stops that becoming an assumption.
	for _, row := range bothRows {
		sess, p := openTestSession(t, row, radioImage{
			mrAnswers: map[string]string{mrAddr("042"): short},
		})
		_, err := sess.ReadChannel(context.Background(), "042")
		if !errors.Is(err, transport.ErrTimeout) {
			t.Errorf("%s: a 49-byte MR answer: err = %v, want a timeout — an exact-width matcher must not correlate it", modelNameFor(row), err)
		}
		if errors.Is(err, kw.ErrParse) {
			t.Errorf("%s: a 49-byte MR answer reached the parser: %v", modelNameFor(row), err)
		}
		if got := p.Transcript(); len(got) < 4 || got[3] != "MR0042;" {
			t.Errorf("%s: transcript = %v, want the MR read to have gone out", modelNameFor(row), got)
		}

		// The codec's own width predicate, on the same bytes.
		_, perr := sess.layout.ParseMRAnswer([]byte(short))
		assertKenwoodRecordLengthMismatch(t, perr, "MR", kw.RecordLen-1, kw.RecordLen)
	}
}

// assertKenwoodRecordLengthMismatch pins the record-length refusal contract:
// a caller can CLASSIFY the failure, RECOVER the codec's measured lengths,
// and the user-facing text is EXACT.
//
// IT STANDS IN FOR drivertest.AssertRecordLengthMismatch, WHICH THIS TASK WAS
// NAMED AS THE CONSUMER OF AND WHICH CANNOT BE CALLED HERE. That helper's
// contract is CI-V's: it asserts errors.Is(err, driver.ErrWrongRadio) and
// recovers a *civ.RecordLengthError (core/driver/internal/drivertest/
// record_length.go), because on the Icom side a mis-sized record is a
// PROBE-TIME radio classification. A Kenwood short MR answer is neither — it
// is a malformed memory frame on the READ path, typed *kw.RecordLengthError
// and wrapping kw.ErrParse — so calling the fleet helper would require this
// driver to import core/civ and to claim "wrong radio" for a frame width,
// which would be false. The SHAPE of the contract is what matters and is
// reproduced here in this family's own vocabulary; the divergence is reported
// rather than papered over.
func assertKenwoodRecordLengthMismatch(t testing.TB, err error, command string, got, want int) {
	t.Helper()
	if !errors.Is(err, kw.ErrParse) {
		t.Errorf("errors.Is(err, kw.ErrParse) = false for %v", err)
	}
	var lengthErr *kw.RecordLengthError
	if !errors.As(err, &lengthErr) {
		t.Fatalf("errors.As(err, **kw.RecordLengthError) = false for %v", err)
	}
	if lengthErr.Command != command || lengthErr.Got != got || lengthErr.Want != want {
		t.Errorf("kw.RecordLengthError = %s %d/%d, want %s %d/%d", lengthErr.Command, lengthErr.Got, lengthErr.Want, command, got, want)
	}
	if text := lengthErr.Error(); !strings.Contains(text, fmt.Sprintf("kw: %s memory frame is %d bytes, want exactly %d bytes", command, got, want)) {
		t.Errorf("Error() = %q, want it to open with the two measured lengths", text)
	}
}

// TestReadChannel_IsAtomicUnderOpMu is P13/P14's concurrency pin, and it is
// a NEGATIVE one made deterministic by a test-only hook rather than by
// hammering: Go's sync.Mutex favours an immediately-re-locking goroutine so
// heavily that the interleaving opMu forbids is near-impossible to reproduce
// otherwise.
//
// The hook runs with opMu ALREADY HELD, so entering it is proof that a
// goroutine got past the lock. While one operation is parked in there, a
// second must not enter it AT ALL — which is what "the lock guards a whole
// driver operation" means, as against "the transport serialises each
// exchange". The bounded wait is what makes the negative real: an
// unsynchronised second goroutine reaches the hook in microseconds.
//
// RED PROOF, observed: with the two opMu lines removed from ReadChannel the
// second operation enters the hook immediately and this test fails at "a
// second operation entered ReadChannel while the first held opMu".
func TestReadChannel_IsAtomicUnderOpMu(t *testing.T) {
	sess, p := openTestSession(t, RowSG, radioImage{mrAnswers: map[string]string{
		mrAddr("001"): populatedMR("001"),
		mrAddr("002"): populatedMR("002"),
	}})

	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var parkOnce sync.Once
	readChannelGapHook = func() {
		// Non-blocking, so a later call can never deadlock on a receiver
		// the test has stopped providing.
		select {
		case entered <- struct{}{}:
		default:
		}
		parkOnce.Do(func() { <-release })
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), "001"); err != nil {
			t.Errorf("parked ReadChannel: %v", err)
		}
	}()
	<-entered // the first operation is inside the lock and parked

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), "002"); err != nil {
			t.Errorf("second ReadChannel: %v", err)
		}
	}()

	select {
	case <-entered:
		t.Fatal("a second operation entered ReadChannel while the first held opMu")
	case <-time.After(250 * time.Millisecond):
	}
	// Nothing has reached the wire either: the parked operation is ahead of
	// its own frame, and the second is ahead of the lock.
	if got := p.Transcript(); len(got) != 3 {
		t.Errorf("transcript while one operation is parked inside opMu = %v, want the probe's three frames alone", got)
	}

	close(release)
	wg.Wait()

	want := []string{"AI0;", "ID;", "FV;", "MR0001;", "MR0002;"}
	if got := p.Transcript(); !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v — the two operations must not interleave", got, want)
	}
}

// TestReadAll_FailsWholeOnTheFirstRefusalOrTimeout pins the consequence of
// the two typed refusals above, walked the way core/clone's ReadAll walks a
// bank: the FIRST error ends the read, and every later slot goes unasked.
//
// THE ALTERNATIVE IS THE FAILURE THIS ORDERING EXISTS TO PREVENT. A walk that
// skipped a refused channel and carried on would hand the user a codeplug
// they could not tell from a complete one, and on these radios a "?;" carries
// no reason code (590:100-105) while a timeout carries no information at all
// (590:106-108), so neither can honestly be read as "this slot is empty".
//
// The walk is written out here rather than delegated: core/clone imports
// core/driver, and a driver package's own test asserting its errors' SHAPE is
// what makes clone's propagation correct, not the reverse.
func TestReadAll_FailsWholeOnTheFirstRefusalOrTimeout(t *testing.T) {
	for _, tc := range []struct {
		name string
		img  func(map[string]string) radioImage
		is   error
	}{
		{
			// No answer scripted for 002: this image serves "?;".
			name: "a rejection",
			img:  func(ans map[string]string) radioImage { return radioImage{mrAnswers: ans} },
			is:   transport.ErrRejected,
		},
		{
			name: "silence",
			img: func(ans map[string]string) radioImage {
				return radioImage{mrAnswers: ans, mrSilent: map[string]bool{mrAddr("002"): true}}
			},
			is: transport.ErrTimeout,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			answers := map[string]string{
				mrAddr("000"): populatedMR("000"),
				mrAddr("001"): emptyMR("001"),
				mrAddr("003"): populatedMR("003"),
			}
			sess, p := openTestSession(t, RowSG, tc.img(answers))

			var read int
			var walkErr error
			for _, id := range []string{"000", "001", "002", "003"} {
				if _, err := sess.ReadChannel(context.Background(), id); err != nil {
					walkErr = err
					break
				}
				read++
			}
			if walkErr == nil {
				t.Fatal("the walk completed; a refused channel must end the read")
			}
			if !errors.Is(walkErr, tc.is) {
				t.Errorf("errors.Is(err, %v) = false for %v", tc.is, walkErr)
			}
			if read != 2 {
				t.Errorf("%d channels read before the failure, want 2 — a populated one and an empty one", read)
			}
			// 003 was never asked, which is what "fails whole" means on the
			// wire rather than merely in the return value.
			for _, frame := range p.Transcript() {
				if frame == "MR0003;" {
					t.Errorf("transcript = %v; the walk continued past the failure", p.Transcript())
				}
			}
		})
	}
}

// TestReadChannel_AnAnswerWhoseP1DisagreesIsRefusedOnAMEMSlot is the MEM half
// of the answer-mismatch guard, and it is a DIFFERENT fact from the SCAN half
// above. On a section channel P1 is folded into the slot's own string ("100L"
// against "100U"), so the string comparison catches a wrong half; on a MEM
// slot Slot.String() renders three digits and discards P1 entirely.
//
// An MR answer carrying P1='1' is the TRANSMIT side of a split channel
// (590:1519-1520). Accepting one for the P1='0' read this driver sent would
// store a transmit frequency as the channel's receive frequency with TxFreqHz
// left Unavailable — the silent loss decision 11 exists to prevent, and one
// codeplug.Validate cannot see. The write side has made the same comparison
// since core/kw's builder (kw.BuildMWSet refuses a record whose AnswerP1
// disagrees with its slot's class, 590:1529-1531); this is the read side's.
//
// RED PROOF, observed before the guard existed: the read SUCCEEDED and
// returned FreqHz=145500000 from an answer the driver never asked for.
func TestReadChannel_AnAnswerWhoseP1DisagreesIsRefusedOnAMEMSlot(t *testing.T) {
	for _, row := range bothRows {
		f := populatedFields("042")
		f.p1 = '1'
		sess, _ := openTestSession(t, row, radioImage{mrAnswers: map[string]string{
			mrAddr("042"): f.frame(),
		}})
		ch, err := sess.ReadChannel(context.Background(), "042")
		if err == nil {
			t.Fatalf("%s: ReadChannel(\"042\") accepted a P1='1' answer and stored %+v", modelNameFor(row), ch.Data)
		}
		if !errors.Is(err, ErrAnswerMismatch) {
			t.Errorf("%s: errors.Is(err, ErrAnswerMismatch) = false for %v", modelNameFor(row), err)
		}
		var mismatch *AnswerP1MismatchError
		if !errors.As(err, &mismatch) {
			t.Fatalf("%s: errors.As(err, **AnswerP1MismatchError) = false for %v", modelNameFor(row), err)
		}
		if mismatch.Slot != "042" || mismatch.Requested != '0' || mismatch.Answered != '1' {
			t.Errorf("%s: AnswerP1MismatchError = %q %q/%q, want \"042\" '0'/'1'", modelNameFor(row), mismatch.Slot, mismatch.Requested, mismatch.Answered)
		}
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic905 "github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// openWritable Opens a SIMULATED-profile session against a scripted
// radio.
//
// Simulated, because on the RealHardware profile NOTHING IS WRITABLE and
// rung 7 refuses every channel before any later rung can be reached —
// which is the profile doing its job, not a limitation of this ladder.
// TestWrite_TheRealHardwareProfileRefusesEverything pins that half; every
// other write test needs a profile that lets the choreography run.
func openWritable(t *testing.T, img radioImage) (*respondingPort, *Session) {
	t.Helper()
	if len(img.idToken) == 0 {
		img.idToken = testToken
	}
	p := newRespondingPort(t, img)
	sess, err := New(Simulated).Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return p, sess.(*Session)
}

// writableChannel is a fully specified channel carrying exactly the
// values both golden vectors carry, so a write of it to wire group 0
// channel 1 must reproduce the golden set frame byte for byte.
func writableChannel(slot string) codeplug.Channel {
	return codeplug.Channel{Slot: slot, Data: &codeplug.ChannelData{
		FreqHz: 144_500_000,
		Mode:   "FM",
		Tag:    "HIGHLAND BASE905",
		// The eight fields this record does not express, in the states a
		// read of this radio produces.
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
		TxFreqHz:   codeplug.FreqField{State: codeplug.Unavailable},
		// The nine it does.
		Duplex:       codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		OffsetHz:     codeplug.FreqField{State: codeplug.Known, Value: 0},
		ToneMode:     codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:       codeplug.ToneField{State: codeplug.Known, Value: 885},
		ToneRx:       codeplug.ToneField{State: codeplug.Known, Value: 885},
		DTCSCode:     codeplug.IntField{State: codeplug.Known, Value: 23},
		DTCSPolarity: codeplug.StringField{State: codeplug.Known, Value: "NN"},
		Filter:       codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
		DataMode:     codeplug.BoolField{State: codeplug.Known, Value: false},
	}}
}

// occupiedAt seeds an image so that slot's wire address holds the golden
// record AND the bounded discovery walk will materialise it.
func occupiedAt(addrs ...wireAddr) radioImage {
	img := radioImage{idToken: testToken}
	populate(&img, addrs...)
	return img
}

// requireRefused asserts a refusal that named the expected fields and
// that NOTHING went on the wire.
func requireRefused(t *testing.T, p *respondingPort, res driver.WriteResult, err error, wantFields ...spec.Field) *driver.WriteRefusedError {
	t.Helper()
	if err == nil {
		t.Fatal("the write was accepted, want a refusal")
	}
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("error = %v, want errors.Is(err, driver.ErrWriteRefused)", err)
	}
	if res.Steps == nil {
		t.Error("WriteResult.Steps is nil — a refusal must report an EXPLICITLY EMPTY step list, because a nil slice marshals as JSON null and an auditor would have to read that as \"unknown\"")
	}
	if len(res.Steps) != 0 {
		t.Errorf("WriteResult.Steps = %+v, want empty — no frame was built, so nothing was attempted", res.Steps)
	}
	if sets := p.sets(); len(sets) != 0 {
		t.Errorf("%d set frames reached the radio despite a refusal:\n  % X", len(sets), sets)
	}
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("error = %v, want a *driver.WriteRefusedError", err)
	}
	if len(wantFields) > 0 && !slices.Equal(wre.Fields, wantFields) {
		t.Errorf("refused fields = %v, want %v", wre.Fields, wantFields)
	}
	return wre
}

// TestRequestedFields_MembershipAndOrder spells the two halves out
// SEPARATELY, so the comparison is not tautological, and then asserts the
// thing that would have caught REV 2's error: no field in either list is
// graded ZERO by this driver's own real-hardware profile.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	t.Parallel()
	wantUnconditional := []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldTag}
	if !slices.Equal(unconditionalFields, wantUnconditional) {
		t.Errorf("unconditionalFields = %v, want %v — matrix section 2's supported set minus the Known-conditionals, NOT codeplug.addedFields' Yaesu six", unconditionalFields, wantUnconditional)
	}

	wantTier := []spec.Field{
		spec.FieldDuplex, spec.FieldOffset, spec.FieldToneMode,
		spec.FieldToneTx, spec.FieldToneRx, spec.FieldDTCSCode,
		spec.FieldDTCSPolarity, spec.FieldFilter, spec.FieldDataMode,
	}
	var gotTier []spec.Field
	for _, tr := range tierRequestedFields {
		gotTier = append(gotTier, tr.field)
	}
	if !slices.Equal(gotTier, wantTier) {
		t.Errorf("tierRequestedFields = %v, want %v — ChannelData's own declaration order, and NO tx_frequency (matrix section 2 row 11)", gotTier, wantTier)
	}

	// A fully specified channel requests all twelve, in order.
	got := requestedFields(*writableChannel("G01-001").Data)
	if want := append(append([]spec.Field{}, wantUnconditional...), wantTier...); !slices.Equal(got, want) {
		t.Errorf("requestedFields = %v, want %v", got, want)
	}

	// AND NOT ONE OF THEM IS GRADED ZERO. This is the half that would
	// have caught a driver copying the Yaesu unconditional set: every
	// write would have been refused by its own capability gate.
	caps := capabilitiesUnverified()
	for _, f := range got {
		if caps.FieldSupport(spec.BankMemory, f).Unreachable() {
			t.Errorf("requestedFields includes %s, which this model grades ZERO on MEM — every write would be refused by this driver's own capability gate", f)
		}
	}
}

// TestRequestedFields_ExcludesTheYaesuTrio names the three by name, with
// the matrix rows that zero them: clarifier (§2 row 3), ctcss_state (row
// 4) and shift (row 6). R6-COMPLETION is explicit that the trio "must not
// survive anywhere".
func TestRequestedFields_ExcludesTheYaesuTrio(t *testing.T) {
	t.Parallel()
	got := requestedFields(*writableChannel("G01-001").Data)
	for _, f := range []spec.Field{
		spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldShift,
		spec.FieldTagDisplay, spec.FieldScanSkip, spec.FieldErase, spec.FieldTxFrequency,
	} {
		if slices.Contains(got, f) {
			t.Errorf("requestedFields includes %s, which this matrix grades zero on BOTH banks", f)
		}
	}
}

// TestWrite_EveryTierFieldMeetsTheCapabilityGate is the Wave-1 C2 class:
// each of the nine tier fields, requested by a direct Session.WriteChannel
// against the REAL-HARDWARE profile, is refused BY NAME.
//
// SEVEN OF THE NINE CANNOT BE ISOLATED, and that is rung 4 rather than a
// weak test: duplex, offset, tone_mode, dtcs_code, dtcs_polarity, filter
// and data_mode are MANDATORY-Known, so a channel that reaches the
// capability gate at all necessarily carries all seven and requests all
// seven together. Each case therefore asserts that ITS field is among the
// refused ones, which is the property that matters: no tier field escapes
// the gate.
func TestWrite_EveryTierFieldMeetsTheCapabilityGate(t *testing.T) {
	t.Parallel()
	p := newRespondingPort(t, occupiedAt(wireAddr{0, 0}))
	sess, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	res, err := sess.WriteChannel(context.Background(), writableChannel("G01-001"))
	wre := requireRefused(t, p, res, err)

	for _, f := range []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldDuplex, spec.FieldOffset, spec.FieldToneMode,
		spec.FieldToneTx, spec.FieldToneRx, spec.FieldDTCSCode,
		spec.FieldDTCSPolarity, spec.FieldFilter, spec.FieldDataMode,
	} {
		t.Run(string(f), func(t *testing.T) {
			if !slices.Contains(wre.Fields, f) {
				t.Errorf("%s is not among the refused fields %v — on the all-Unverified profile NOTHING is writable, and every requested field must be named", f, wre.Fields)
			}
		})
	}
}

// TestWrite_TheRealHardwareProfileRefusesEverything states the same fact
// as a property rather than a field list: no channel, of any shape, is
// writable to a real IC-905 today. The CAPABILITY PROFILE is what
// enforces that — not this ladder.
func TestWrite_TheRealHardwareProfileRefusesEverything(t *testing.T) {
	t.Parallel()
	p := newRespondingPort(t, occupiedAt(wireAddr{0, 0}))
	sess, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	res, err := sess.WriteChannel(context.Background(), writableChannel("G01-001"))
	requireRefused(t, p, res, err)
}

// TestWrite_ProducesTheGoldenSetFrame is the strongest pin in this file:
// a write of a channel carrying the golden values, to the golden vector's
// own address, must put THE GOLDEN VECTOR on the wire, byte for byte.
//
// The expectation is the frozen evidence itself
// (core/civ/ic905/testdata/ic905-vectors.golden's
// set-record-name-with-space-68), transcribed here rather than rebuilt,
// so a mapping that quietly moved a field fails against the manual rather
// than against itself.
func TestWrite_ProducesTheGoldenSetFrame(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 1}))

	res, err := s.WriteChannel(context.Background(), writableChannel("G01-002"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "1A 00" {
		t.Fatalf("Steps = %+v, want one 1A 00 step — this radio's write choreography IS one frame", res.Steps)
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps[0] = %+v, want Sent and Confirmed — the radio answered its printed OK message", res.Steps[0])
	}

	sets := p.sets()
	if len(sets) != 1 {
		t.Fatalf("the radio received %d set frames, want exactly 1", len(sets))
	}
	want := concat(
		[]byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00}, // envelope, controller -> radio
		[]byte{0x00, 0x00, 0x00, 0x01},             // ①,② group 00 and ③,④ channel 01
		goldenRecord(144_500_000, 5).build(),
		[]byte{0xFD},
	)
	if len(want) != 75 {
		t.Fatalf("the transcribed golden frame is %d bytes, want 75 (7 + 4 address + 64 record)", len(want))
	}
	if !bytes.Equal(sets[0], want) {
		t.Errorf("the set frame does not match the golden vector:\n  sent   % X\n  golden % X", sets[0], want)
	}
}

// --- The ladder, rung by rung. Every rung above the read is its own
// test, and every one of them asserts that NOTHING reached the wire.

func TestWrite_RungOne_RefusesASlotNeitherNamespaceParses(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))
	for _, slot := range []string{"001", "P1L", "G5-12", "C1", ""} {
		ch := writableChannel(slot)
		res, err := s.WriteChannel(context.Background(), ch)
		requireRefused(t, p, res, err)
	}
}

// TestWrite_RungTwo_RefusesASlotInNoEffectiveBank.
//
// RUNG 2 IS CURRENTLY UNREACHABLE THROUGH RUNG 1, and saying so is better
// than a test that pretends otherwise. slotAddress is strict in both
// namespaces — spec.ParseSparseSlot bounds MEM to 1…100 × 1…100 and
// ic905.ParseCallSlot bounds CALL to twelve — so every slot that PARSES
// is inside a bank this session has. The profile's known over-admission
// (civ's single global channel range admits CALL-group channels 12…99)
// therefore never reaches this rung either: "C13" is refused one rung
// earlier.
//
// The rung is kept as defence in depth and is exercised DIRECTLY, on
// bankFor, so a future slot form that widened rung 1 would still meet it.
//
// A SPARSE BANK CLAIMS ITS WHOLE SPACE, not only what discovery
// materialised: an address no read has ever returned is a legal place for
// a user to ADD a channel, which is exactly why the occupied-surprise
// rung exists further down.
func TestWrite_RungTwo_RefusesASlotInNoEffectiveBank(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))

	for _, slot := range []string{"G101-001", "G01-101", "C13", "001", "EMG"} {
		if bank, ok := s.bankFor(slot); ok {
			t.Errorf("bankFor(%q) = %s, want no bank", slot, bank)
		}
	}
	// Inside the sparse space but never materialised: IN the bank, and
	// deliberately so.
	if _, ok := s.bankFor("G77-042"); !ok {
		t.Error("bankFor(\"G77-042\") reports no bank — a sparse bank claims its whole addressable space, not only the slots discovery materialised")
	}

	// And a slot no namespace parses is refused before any of that.
	res, err := s.WriteChannel(context.Background(), writableChannel("G101-001"))
	requireRefused(t, p, res, err)
}

// TestWrite_RungThree_RefusesAnEmptyChannel: THIS TIER SHIPS NO ERASE
// PATH. The wire form exists on this radio and is recorded in doc.go;
// nothing implements it, FieldErase is the zero FieldSupport in both
// profiles, and spec.ConsentUnverifiedWrites structurally never consents
// it.
//
// The rung also has to precede the FieldState checks STRUCTURALLY: an
// empty channel has no Data, and every rung below dereferences it.
func TestWrite_RungThree_RefusesAnEmptyChannel(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))
	res, err := s.WriteChannel(context.Background(), codeplug.Channel{Slot: "G01-001"})
	requireRefused(t, p, res, err, spec.FieldErase)
}

// TestWrite_RungFour_RefusesANonKnownMandatoryField is ruling R6: only
// Known values are ever encoded, and a non-Known mandatory field is
// REFUSED, never synthesised and never preserved-by-cache.
func TestWrite_RungFour_RefusesANonKnownMandatoryField(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))

	for _, tt := range []struct {
		name  string
		mutin func(*codeplug.ChannelData)
		want  spec.Field
	}{
		{"duplex", func(d *codeplug.ChannelData) { d.Duplex = codeplug.StringField{State: codeplug.Unknown} }, spec.FieldDuplex},
		{"offset", func(d *codeplug.ChannelData) { d.OffsetHz = codeplug.FreqField{State: codeplug.Unknown} }, spec.FieldOffset},
		{"tone_mode", func(d *codeplug.ChannelData) { d.ToneMode = codeplug.StringField{State: codeplug.Unknown} }, spec.FieldToneMode},
		{"dtcs_code", func(d *codeplug.ChannelData) { d.DTCSCode = codeplug.IntField{State: codeplug.Unknown} }, spec.FieldDTCSCode},
		{"dtcs_polarity", func(d *codeplug.ChannelData) { d.DTCSPolarity = codeplug.StringField{State: codeplug.Unknown} }, spec.FieldDTCSPolarity},
		{"filter", func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{State: codeplug.Unknown} }, spec.FieldFilter},
		{"data_mode", func(d *codeplug.ChannelData) { d.DataMode = codeplug.BoolField{State: codeplug.Unknown} }, spec.FieldDataMode},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("G01-001")
			tt.mutin(ch.Data)
			res, err := s.WriteChannel(context.Background(), ch)
			requireRefused(t, p, res, err, tt.want)
		})
	}
}

// TestWrite_RungFive_RefusesTheTenGigahertzWriteBeforeTheWire is OQ-1's
// consequence, and the whole reason BuildLength is 64.
//
// The tier writes only the shape its document draws. A 10 GHz record
// needs the six-byte frequency form, which is ASSUMED and which no
// IC-905 has ever been asked to accept, so the write is refused HERE —
// before the wire, naming the field and the lift — rather than failing
// inside the encoder or, worse, going out at an assumed width.
func TestWrite_RungFive_RefusesTheTenGigahertzWriteBeforeTheWire(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))

	ch := writableChannel("G01-001")
	ch.Data.FreqHz = 10_250_000_000
	// DD is the only mode the 10 GHz band's own footnote admits, and the
	// cross-field rung would otherwise fire first on a 10 GHz FM channel.
	ch.Data.Mode = "DD"
	ch.Data.Duplex = codeplug.StringField{State: codeplug.Known, Value: "RPS"}

	res, err := s.WriteChannel(context.Background(), ch)
	wre := requireRefused(t, p, res, err, spec.FieldFrequency)
	if !strings.Contains(wre.Reason, "ic905-R-06") {
		t.Errorf("reason = %q, want it to name the lift ic905-R-06", wre.Reason)
	}

	// 9,999,999,999 Hz is the largest ten packed-BCD digits reach and the
	// boundary the rule turns on. It is past this radio's declared
	// ceiling, so it is not writable either — but it must not be refused
	// by THIS rung.
	ch.Data.FreqHz = 9_999_999_999
	res, err = s.WriteChannel(context.Background(), ch)
	if err != nil {
		wre := requireRefused(t, p, res, err)
		if slices.Contains(wre.Fields, spec.FieldFrequency) && strings.Contains(wre.Reason, "six-byte") {
			t.Errorf("9999999999 Hz was refused as needing the wide form: the boundary is > 9999999999, not >=")
		}
	}
}

// TestWrite_RungSix_RefusesADTCSDigitAboveSeven is OQ-6's driver-seam
// defence in depth. The PRIMARY gate is the explicit 512-code table
// codeplug.Validate consults; this re-check is what the driver seam's own
// contract requires, and civ's BCD encoder would otherwise accept 0–9.
func TestWrite_RungSix_RefusesADTCSDigitAboveSeven(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))
	for _, code := range []int{8, 18, 80, 778, 999} {
		ch := writableChannel("G01-001")
		ch.Data.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: code}
		res, err := s.WriteChannel(context.Background(), ch)
		requireRefused(t, p, res, err, spec.FieldDTCSCode)
	}
}

// TestWrite_RungEight_TheCrossFieldRulesThePagePrints.
//
// The generic codec validates each enum independently, so without these
// a consented write could send a combination the manual forbids. All
// three are LOCALLY DECIDABLE and therefore sit above the read.
func TestWrite_RungEight_TheCrossFieldRulesThePagePrints(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))

	for _, tt := range []struct {
		name  string
		mutin func(*codeplug.ChannelData)
		want  []spec.Field
	}{
		{
			"RPS without DD",
			func(d *codeplug.ChannelData) {
				d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "RPS"}
			},
			[]spec.Field{spec.FieldDuplex, spec.FieldMode},
		},
		{
			"DUP+ with DD",
			func(d *codeplug.ChannelData) {
				d.Mode = "DD"
				d.FreqHz = 1_240_000_000
				d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
			},
			[]spec.Field{spec.FieldDuplex, spec.FieldMode},
		},
		{
			"DUP- with DD",
			func(d *codeplug.ChannelData) {
				d.Mode = "DD"
				d.FreqHz = 1_240_000_000
				d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP-"}
			},
			[]spec.Field{spec.FieldDuplex, spec.FieldMode},
		},
		{
			"DD below the 1200 MHz band",
			func(d *codeplug.ChannelData) {
				d.Mode = "DD"
				d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "RPS"}
			},
			[]spec.Field{spec.FieldMode, spec.FieldFrequency},
		},
		{
			"ATV below the 1200 MHz band",
			func(d *codeplug.ChannelData) { d.Mode = "ATV" },
			[]spec.Field{spec.FieldMode, spec.FieldFrequency},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("G01-001")
			tt.mutin(ch.Data)
			res, err := s.WriteChannel(context.Background(), ch)
			wre := requireRefused(t, p, res, err, tt.want...)
			if !strings.Contains(wre.Reason, "ic905-R-19") {
				t.Errorf("reason = %q, want it to name the register lift ic905-R-19", wre.Reason)
			}
		})
	}

	// And the combinations the page ALLOWS are not refused by this rung,
	// so it is a rule rather than a ban.
	for _, tt := range []struct {
		name  string
		mutin func(*codeplug.ChannelData)
	}{
		{"RPS with DD on the 1200 MHz band", func(d *codeplug.ChannelData) {
			d.Mode = "DD"
			d.FreqHz = 1_240_000_000
			d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "RPS"}
		}},
		{"DUP+ with FM", func(d *codeplug.ChannelData) {
			d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
		}},
	} {
		t.Run("allowed: "+tt.name, func(t *testing.T) {
			ch := writableChannel("G01-001")
			tt.mutin(ch.Data)
			if err := crossFieldRefusal(ch.Slot, *ch.Data); err != nil {
				t.Errorf("the page allows this combination but it was refused: %v", err)
			}
		})
	}
}

// TestUnmappedRanges_CoverExactlyTheLayoutsUnmappedBytes is the
// structural cross-check behind rung 10: the six named runs must be
// EXACTLY the bytes no FieldSpan maps, in BOTH layouts. A run dropped
// from the table, or a field that stopped being mapped, fails here rather
// than becoming a silent hole in the preservation check.
func TestUnmappedRanges_CoverExactlyTheLayoutsUnmappedBytes(t *testing.T) {
	t.Parallel()
	p := civProfile()
	for _, length := range []int{64, 65} {
		t.Run(string(rune('0'+length/10%10))+string(rune('0'+length%10)), func(t *testing.T) {
			layout, ok := p.LayoutFor(length)
			if !ok {
				t.Fatalf("the profile declares no layout of length %d", length)
			}
			mapped := make([]bool, layout.Length)
			for _, span := range layout.Fields {
				for i := span.Offset; i < span.Offset+span.Length; i++ {
					mapped[i] = true
				}
			}
			covered := make([]bool, layout.Length)
			total := 0
			for _, r := range unmappedRanges {
				off := r.offsetIn(layout.Length)
				for i := off; i < off+r.length; i++ {
					if i >= layout.Length {
						t.Fatalf("range %s reaches byte %d, past the %d-byte record", r.printed, i, layout.Length)
					}
					if covered[i] {
						t.Errorf("byte %d is covered by more than one named range", i)
					}
					covered[i] = true
					total++
				}
			}
			if total != 27 {
				t.Errorf("the named ranges total %d bytes, want 27 (OQ-4: byte ⑤, ⑮, ㉕ and the three eight-byte call-sign blocks)", total)
			}
			for i := 0; i < layout.Length; i++ {
				switch {
				case mapped[i] && covered[i]:
					t.Errorf("byte %d is both mapped by a field span and named as unmapped", i)
				case !mapped[i] && !covered[i]:
					t.Errorf("byte %d is mapped by no field span and named by no range — it would be rewritten from the template with nothing checking it", i)
				}
			}
		})
	}
}

// TestWrite_RefusesAChannelWhoseSelectTagWouldBeCleared is the regression
// test for this plan's one CRITICAL review finding.
//
// REV 1's ladder compared twenty-six bytes, not twenty-seven: byte ⑤ was
// described in its prose as joining the preserved region and was not in
// its comparison. A MEM channel carrying SELECT ★1/★2/★3 would therefore
// have passed the preservation read and been REWRITTEN AS SELECT OFF — a
// silent conversion of an unsupported state into a drop, which is exactly
// what E6 forbids and what the tier's scan_skip-is-SELECT constraint
// exists to protect.
func TestWrite_RefusesAChannelWhoseSelectTagWouldBeCleared(t *testing.T) {
	t.Parallel()
	starred := goldenRecord(144_500_000, 5)
	starred.select5 = 0x02 // SELECT ★2, from the ⑤ breakout's right nibble
	img := radioImage{idToken: testToken, records: map[wireAddr][]byte{{0, 0}: starred.build()}}

	p, s := openWritable(t, img)
	res, err := s.WriteChannel(context.Background(), writableChannel("G01-001"))
	wre := requireRefused(t, p, res, err)
	if !strings.Contains(wre.Reason, "record bytes 0..0") {
		t.Errorf("reason = %q, want it to name record byte 0 (printed ⑤)", wre.Reason)
	}
	if !strings.Contains(wre.Reason, "SELECT") {
		t.Errorf("reason = %q, want it to say a SELECT tag is set", wre.Reason)
	}
}

// TestWrite_RefusesEachUnmappedRangeIndependently, so no single range can
// be dropped from the comparison without a test going red.
func TestWrite_RefusesEachUnmappedRangeIndependently(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		mutin   func(*recordFields)
		printed string
	}{
		{"⑤ SELECT", func(r *recordFields) { r.select5 = 0x03 }, "⑤"},
		{"⑮ digital squelch", func(r *recordFields) { r.digitalSquelch = 0x01 }, "⑮"},
		{"㉕ DV code squelch", func(r *recordFields) { r.dvSquelch = 0x0A }, "㉕"},
		{"㉙~㊱ UR call sign", func(r *recordFields) { r.urCall = "GM5DNA" }, "㉙~㊱"},
		{"㊲~㊹ R1 call sign", func(r *recordFields) { r.r1Call = "GB3IN B" }, "㊲~㊹"},
		{"㊺~52 R2 call sign", func(r *recordFields) { r.r2Call = "GB3IN G" }, "㊺~52"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := goldenRecord(144_500_000, 5)
			tt.mutin(&rec)
			img := radioImage{idToken: testToken, records: map[wireAddr][]byte{{0, 0}: rec.build()}}

			p, s := openWritable(t, img)
			res, err := s.WriteChannel(context.Background(), writableChannel("G01-001"))
			wre := requireRefused(t, p, res, err)
			if !strings.Contains(wre.Reason, tt.printed) {
				t.Errorf("reason = %q, want it to name the printed range %s", wre.Reason, tt.printed)
			}
		})
	}
}

// TestWrite_AnAddToASlotTheBoundedWalkMissedIsRefused is the round-3
// CRITICAL's own test (ruling T3).
//
// The radio holds G06-038 (wire group 5, channel 37) and leaves G06-001
// (wire group 5, channel 0) EMPTY, so the bounded walk's channel-00 probe
// misses the whole group. codeplug.Diff would then offer the write as an
// ADD — and comparing unmapped bytes is not enough to catch it, because
// this record's unmapped region matches the template perfectly.
func TestWrite_AnAddToASlotTheBoundedWalkMissedIsRefused(t *testing.T) {
	t.Parallel()
	img := occupiedAt(wireAddr{0, 0}, wireAddr{5, 37})
	p, s := openWritable(t, img)

	if slices.Contains(memBankSlots(t, s), "G06-038") {
		t.Fatal("the bounded walk materialised G06-038 — this test's premise is gone")
	}
	res, err := s.WriteChannel(context.Background(), writableChannel("G06-038"))
	wre := requireRefused(t, p, res, err)
	if !strings.Contains(wre.Reason, "WithFullInventoryWalk") {
		t.Errorf("reason = %q, want it to NAME the remedy the user can act on", wre.Reason)
	}
	if !strings.Contains(wre.Reason, "G06-038") && wre.Slot != "G06-038" {
		t.Errorf("the refusal does not name the slot: %+v", wre)
	}
}

// TestWrite_TheSameSlotIsWritableAfterAFullWalk: the same radio, opened
// WithFullInventoryWalk, has the slot in its inventory, so the write is a
// MODIFY and proceeds.
func TestWrite_TheSameSlotIsWritableAfterAFullWalk(t *testing.T) {
	t.Parallel()
	img := occupiedAt(wireAddr{0, 0}, wireAddr{5, 37})
	port := newRespondingPort(t, img)
	sess, err := New(Simulated, WithFullInventoryWalk()).
		Open(context.Background(), port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	res, err := sess.WriteChannel(context.Background(), writableChannel("G06-038"))
	if err != nil {
		t.Fatalf("WriteChannel after a full walk: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one confirmed step", res.Steps)
	}
}

// TestWrite_AnAddToAGenuinelyEmptySlotProceeds is the case that must NOT
// be refused, so the guard is not a blanket ban on adds: an
// inventory-absent slot whose read returns FA is a genuine add.
func TestWrite_AnAddToAGenuinelyEmptySlotProceeds(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))

	if slices.Contains(memBankSlots(t, s), "G01-050") {
		t.Fatal("G01-050 is in the inventory — this test's premise is gone")
	}
	res, err := s.WriteChannel(context.Background(), writableChannel("G01-050"))
	if err != nil {
		t.Fatalf("an add to a genuinely empty slot was refused: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one confirmed step", res.Steps)
	}
	if n := len(p.sets()); n != 1 {
		t.Errorf("the radio received %d set frames, want 1", n)
	}
}

// TestWrite_ACreateWithoutAToneIsRefusedNamingTheField is ruling T1(5)'s
// otherwise-branch.
//
// An ADD has no prior record, so a non-Known tone has no source. This
// manual documents NO DEFAULT TONE — it prints the field's digit ranges
// (PDF p.24, folio 23) and no default value anywhere, unlike the models
// whose manuals print "Default: 88.5 Hz" — so the create REFUSES rather
// than inventing one. Register: ic905.create_default_tone, lift
// ic905-R-18.
func TestWrite_ACreateWithoutAToneIsRefusedNamingTheField(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))

	ch := writableChannel("G01-050")
	ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	res, err := s.WriteChannel(context.Background(), ch)
	wre := requireRefused(t, p, res, err, spec.FieldToneTx)
	if !strings.Contains(wre.Reason, "ic905-R-18") {
		t.Errorf("reason = %q, want it to name the register lift ic905-R-18", wre.Reason)
	}
}

// TestWrite_ACreateWithEveryFieldKnownProceeds, so the create rule is a
// rule about missing values rather than a ban on creating channels.
func TestWrite_ACreateWithEveryFieldKnownProceeds(t *testing.T) {
	t.Parallel()
	p, s := openWritable(t, occupiedAt(wireAddr{0, 0}))
	res, err := s.WriteChannel(context.Background(), writableChannel("G01-050"))
	if err != nil {
		t.Fatalf("a fully specified create was refused: %v", err)
	}
	if len(p.sets()) != 1 {
		t.Errorf("the radio received %d set frames, want 1", len(p.sets()))
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one confirmed step", res.Steps)
	}
}

// TestWrite_TonePreservationPutsTheRadiosOwnBytesBack is ruling T1(4),
// and the distinction it turns on is the whole point: a non-Known tone on
// an OCCUPIED slot is filled from the JUST-READ RECORD, verbatim.
//
// THAT IS PRESERVATION, NOT SYNTHESIS. Nothing is chosen, invented or
// defaulted — the radio's own byte goes back — and it is available
// precisely because the E6/T5 read at rung 9 is already mandatory.
func TestWrite_TonePreservationPutsTheRadiosOwnBytesBack(t *testing.T) {
	t.Parallel()
	stored := goldenRecord(144_500_000, 5)
	stored.toneTX, stored.toneRX = 1000, 1234 // 100.0 Hz and 123.4 Hz
	img := radioImage{idToken: testToken, records: map[wireAddr][]byte{{0, 1}: stored.build()}}

	p, s := openWritable(t, img)
	ch := writableChannel("G01-002")
	ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}

	if _, err := s.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	sets := p.sets()
	if len(sets) != 1 {
		t.Fatalf("the radio received %d set frames, want 1", len(sets))
	}
	// The record starts after the seven-byte envelope's first six bytes
	// plus the four address bytes; ⑯~⑱ and ⑲~㉑ are at record offsets
	// 11 and 14.
	sent := sets[0][10 : len(sets[0])-1]
	if got, want := sent[11:14], []byte{0x00, 0x10, 0x00}; !bytes.Equal(got, want) {
		t.Errorf("⑯~⑱ = % X, want % X — the radio's own 100.0 Hz put back verbatim", got, want)
	}
	if got, want := sent[14:17], []byte{0x00, 0x12, 0x34}; !bytes.Equal(got, want) {
		t.Errorf("⑲~㉑ = % X, want % X — the radio's own 123.4 Hz put back verbatim", got, want)
	}
}

// TestMemorySetSpec_IsNeverRetried pins E1's helper AS IT RETURNS,
// because this driver's whole safety argument for the write path rests on
// it: an acknowledged write is still a write, a timeout never resolves
// into a retransmission, and Engine.Do refuses a non-zero RetryReads on
// this class before writing anything.
func TestMemorySetSpec_IsNeverRetried(t *testing.T) {
	t.Parallel()
	_, s := openWritable(t, occupiedAt(wireAddr{0, 0}))
	sp := s.memorySetSpec()
	if sp.Class != transport.ClassWriteWithAck {
		t.Errorf("memorySetSpec().Class = %v, want ClassWriteWithAck — a CI-V memory set has a positive acknowledgement, unlike a CAT Set's silence", sp.Class)
	}
	if sp.RetryReads != 0 {
		t.Errorf("memorySetSpec().RetryReads = %d, want 0 — resending a set could write the channel twice", sp.RetryReads)
	}
	if sp.Match == nil {
		t.Error("memorySetSpec().Match is nil — the ack matcher must come from the CODEC (deviation (a))")
	}
}

// TestWrite_AnUnacknowledgedSetIsSentNotConfirmedAndIsNeverResent.
//
// The set goes out; the radio says nothing. RetryReads is zero on this
// class, so no second frame is even representable — and the report says
// SENT (the frame provably left the port, so the slot's on-radio state
// must now be treated as unverified) and NOT CONFIRMED (the outcome is
// UNATTRIBUTABLE). Reporting it as un-sent would invite the one action
// that must never be taken: sending it again.
func TestWrite_AnUnacknowledgedSetIsSentNotConfirmedAndIsNeverResent(t *testing.T) {
	t.Parallel()
	img := occupiedAt(wireAddr{0, 1})
	img.setOutcome = setIgnored
	p, s := openWritable(t, img)

	res, err := s.WriteChannel(context.Background(), writableChannel("G01-002"))
	if err == nil {
		t.Fatal("the write reported success although no acknowledgement arrived")
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("error = %v, want a transport.ErrTimeout", err)
	}
	if len(res.Steps) != 1 {
		t.Fatalf("Steps = %+v, want one step", res.Steps)
	}
	if !res.Steps[0].Sent {
		t.Error("Steps[0].Sent is false — the frame provably left the port, and an operator must treat the slot as unverified rather than un-written")
	}
	if res.Steps[0].Confirmed {
		t.Error("Steps[0].Confirmed is true — nothing acknowledged it")
	}
	if n := len(p.sets()); n != 1 {
		t.Errorf("the radio received %d set frames, want exactly 1 — a write is NEVER resent", n)
	}
}

// TestWrite_ARejectedSetIsSentAndAttributable: the radio answered its
// printed NG message, so the outcome is known and it is a refusal.
func TestWrite_ARejectedSetIsSentAndAttributable(t *testing.T) {
	t.Parallel()
	img := occupiedAt(wireAddr{0, 1})
	img.setOutcome = setRejected
	p, s := openWritable(t, img)

	res, err := s.WriteChannel(context.Background(), writableChannel("G01-002"))
	if err == nil {
		t.Fatal("the write reported success although the radio rejected it")
	}
	if !errors.Is(err, transport.ErrRejected) {
		t.Errorf("error = %v, want a transport.ErrRejected", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one step Sent and not Confirmed", res.Steps)
	}
	if n := len(p.sets()); n != 1 {
		t.Errorf("the radio received %d set frames, want exactly 1", n)
	}
}

// civProfile is the CI-V dialect this driver is built on, reached without
// a session so the structural tests above need no wire at all.
func civProfile() civ.Profile { return civic905.Profile() }

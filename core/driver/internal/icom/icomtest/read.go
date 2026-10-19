// SPDX-License-Identifier: GPL-3.0-or-later

package icomtest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func (fx Fixture) withRecord(offset int, bytes ...byte) []byte {
	rec := append([]byte(nil), fx.Golden...)
	copy(rec[offset:], bytes)
	return rec
}
func (fx Fixture) readOne(t *testing.T, ch int, slot string, rec []byte) (codeplug.Channel, error) {
	t.Helper()
	img := fx.occupiedRadio()
	img.Records = map[int][]byte{1: fx.Golden, ch: rec}
	p := fx.port(t, img)
	s := fx.openWith(t, p)
	return s.ReadChannel(t.Context(), slot)
}
func (fx Fixture) recordAtFrequency(t *testing.T, hz uint64) []byte {
	t.Helper()
	cmd, err := fx.Params.Profile().BuildMemorySet(civ.MemoryRecord{
		Address:      civ.ChannelAddress{Channel: 42},
		RXFreqHz:     civ.Available(hz),
		Mode:         civ.Available("USB"),
		Filter:       civ.Available("FIL1"),
		ToneMode:     civ.Available("TONE"),
		ToneTXDeciHz: civ.Available[uint64](885),
		ToneRXDeciHz: civ.Available[uint64](1000),
		Name:         civ.Available("FREQ LIMIT"),
	})
	if err != nil {
		t.Fatalf("BuildMemorySet(%d Hz): %v", hz, err)
	}
	frame := cmd.Bytes()
	return frame[8 : len(frame)-1]
}
func (fx Fixture) tReadChannel_RefusesFrequencyAboveCeiling(t *testing.T) {
	hz := uint64(fx.MaxFreqHz + 1)
	ch, err := fx.readOne(t, 42, "042", fx.recordAtFrequency(t, hz))
	domain := fx.AsDomain(err)
	if domain == nil {
		t.Fatalf("ReadChannel = (%+v, %v), want an OutOfDomainError", ch, err)
	}
	if domain.Field != spec.FieldFrequency || domain.Value != hz || domain.Max != fx.MaxFreqHz {
		t.Errorf("OutOfDomainError = %+v, want {frequency, %d, %d}", domain, hz, fx.MaxFreqHz)
	}
	if !errors.Is(err, driver.ErrOutOfDomain) {
		t.Errorf("errors.Is(%v, driver.ErrOutOfDomain) = false", err)
	}
	if msg := err.Error(); !strings.Contains(msg, fmt.Sprint(hz)) || !strings.Contains(msg, fmt.Sprint(fx.MaxFreqHz)) {
		t.Errorf("error %q does not render measured frequency %d and ceiling %d", msg, hz, fx.MaxFreqHz)
	}
	if !ch.Empty() {
		t.Errorf("refused read returned a populated channel: %+v", ch)
	}
}
func (fx Fixture) tReadChannel_AcceptsFrequencyAtCeiling(t *testing.T) {
	ch, err := fx.readOne(t, 42, "042", fx.recordAtFrequency(t, fx.MaxFreqHz))
	if err != nil {
		t.Fatalf("ReadChannel at %d Hz: %v", fx.MaxFreqHz, err)
	}
	if ch.Empty() || ch.Data.FreqHz != fx.MaxFreqHz {
		t.Errorf("ReadChannel at ceiling = %+v, want Known frequency %d", ch, fx.MaxFreqHz)
	}
}
func (fx Fixture) tSlotAddressRoundTrip(t *testing.T) {
	for ch := 1; ch <= 99; ch++ {
		slot := fmt.Sprintf("%03d", ch)
		a, bank, err := fx.slotToAddress(slot)
		if err != nil {
			t.Fatalf("fx.slotToAddress(%q): %v", slot, err)
		}
		if a.Channel != ch || a.Group != 0 {
			t.Errorf("fx.slotToAddress(%q) = %s, want channel %d in a flat space", slot, a, ch)
		}
		if bank != spec.BankMemory {
			t.Errorf("fx.slotToAddress(%q) bank = %s, want MEM", slot, bank)
		}
		back, err := fx.addressToSlot(a)
		if err != nil || back != slot {
			t.Errorf("fx.addressToSlot(%s) = (%q, %v), want (%q, nil)", a, back, err, slot)
		}
	}
	for _, tt := range []struct {
		slot string
		ch   int
	}{{"P1", 100}, {"P2", 101}} {
		a, bank, err := fx.slotToAddress(tt.slot)
		if err != nil {
			t.Fatalf("fx.slotToAddress(%q): %v", tt.slot, err)
		}
		if a.Channel != tt.ch {
			t.Errorf("fx.slotToAddress(%q) = %s, want channel %d", tt.slot, a, tt.ch)
		}
		if bank != spec.BankScan {
			t.Errorf("fx.slotToAddress(%q) bank = %s, want SCAN", tt.slot, bank)
		}
		back, err := fx.addressToSlot(a)
		if err != nil || back != tt.slot {
			t.Errorf("fx.addressToSlot(%s) = (%q, %v), want (%q, nil)", a, back, err, tt.slot)
		}
	}
	for _, bad := range []string{"000", "100", "P0", "P3", "1", "", "G05-012", "0001", "abc"} {
		if _, _, err := fx.slotToAddress(bad); err == nil {
			t.Errorf("fx.slotToAddress(%q) succeeded; it is not a slot on this radio", bad)
		}
	}
	for _, bad := range []civ.ChannelAddress{{Channel: 0}, {Channel: 102}, {Channel: -1}, {Group: 1, Channel: 1}} {
		if _, err := fx.addressToSlot(bad); err == nil {
			t.Errorf("fx.addressToSlot(%s) succeeded; it is not an address on this radio", bad)
		}
	}
}
func (fx Fixture) tReadChannel_MapsEveryField(t *testing.T) {
	ch, err := fx.readOne(t, 7, "007", fx.Golden)
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Slot != "007" {
		t.Errorf("Slot = %q, want %q", ch.Slot, "007")
	}
	if ch.Empty() {
		t.Fatal("the channel came back empty; the record is populated")
	}
	d := ch.Data

	if d.FreqHz != 14_250_000 {
		t.Errorf("FreqHz = %d, want 14250000", d.FreqHz)
	}
	if d.Mode != "USB" {
		t.Errorf("Mode = %q, want %q", d.Mode, "USB")
	}
	if d.Filter != (codeplug.StringField{State: codeplug.Known, Value: "FIL1"}) {
		t.Errorf("Filter = %+v, want Known FIL1", d.Filter)
	}
	if d.ToneMode != (codeplug.StringField{State: codeplug.Known, Value: "TONE"}) {
		t.Errorf("ToneMode = %+v, want Known TONE", d.ToneMode)
	}
	if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: 885}) {
		t.Errorf("ToneTx = %+v, want Known 885", d.ToneTx)
	}
	if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: 1000}) {
		t.Errorf("ToneRx = %+v, want Known 1000", d.ToneRx)
	}
	if d.Tag != "HOME QTH01" {
		t.Errorf("Tag = %q, want %q", d.Tag, "HOME QTH01")
	}
	for _, tt := range []struct {
		name string
		got  codeplug.BoolField
	}{
		{"ScanSkip", d.ScanSkip},
		{"DataMode", d.DataMode},
	} {
		if tt.got != (codeplug.BoolField{State: codeplug.Unavailable}) {
			t.Errorf("%s = %+v, want Unavailable - ruling E6 leaves its nibble UNMAPPED, and an unmapped region is not decoded", tt.name, tt.got)
		}
	}
	if d.TagDisplay != (codeplug.BoolField{State: codeplug.Unavailable}) {
		t.Errorf("TagDisplay = %+v, want Unavailable", d.TagDisplay)
	}
	if d.CTCSSTone != (codeplug.ToneField{State: codeplug.Unavailable}) {
		t.Errorf("CTCSSTone = %+v, want Unavailable", d.CTCSSTone)
	}
	if d.TxFreqHz != (codeplug.FreqField{State: codeplug.Unavailable}) {
		t.Errorf("TxFreqHz = %+v, want Unavailable", d.TxFreqHz)
	}
	if d.Duplex != (codeplug.StringField{State: codeplug.Unavailable}) {
		t.Errorf("Duplex = %+v, want Unavailable", d.Duplex)
	}
	if d.OffsetHz != (codeplug.FreqField{State: codeplug.Unavailable}) {
		t.Errorf("OffsetHz = %+v, want Unavailable", d.OffsetHz)
	}
	if d.DTCSCode != (codeplug.IntField{State: codeplug.Unavailable}) {
		t.Errorf("DTCSCode = %+v, want Unavailable", d.DTCSCode)
	}
	if d.DTCSPolarity != (codeplug.StringField{State: codeplug.Unavailable}) {
		t.Errorf("DTCSPolarity = %+v, want Unavailable", d.DTCSPolarity)
	}
	if d.ClarHz != 0 || d.RxClar || d.TxClar {
		t.Errorf("a clarifier value was invented: %d/%v/%v", d.ClarHz, d.RxClar, d.TxClar)
	}
	if d.CTCSS != "" || d.Shift != "" {
		t.Errorf("a Yaesu vocabulary value was invented: CTCSS %q, Shift %q", d.CTCSS, d.Shift)
	}
	drivertest.AssertFreshReadSaveLoad(t, ch, fx.Simulated(), fx.Load)
}
func (fx Fixture) tReadChannel_EmptySlotIsAnEmptyChannel(t *testing.T) {
	allFF := make([]byte, len(fx.Golden))
	for i := range allFF {
		allFF[i] = 0xFF
	}
	for _, tt := range []struct {
		name string
		slot string
		rec  []byte // nil means "absent from the image", i.e. answered FA
	}{
		{"FA rejection, memory", "042", nil},
		{"FA rejection, scan edge P1", "P1", nil},
		{"FA rejection, scan edge P2", "P2", nil},
		{"all-FF record, memory", "042", allFF},
		{"all-FF record, scan edge P1", "P1", allFF},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := fx.occupiedRadio()
			if tt.rec != nil {
				a, _, err := fx.slotToAddress(tt.slot)
				if err != nil {
					t.Fatalf("slotToAddress: %v", err)
				}
				img.Records[a.Channel] = tt.rec
			}
			p := fx.port(t, img)
			s := fx.openWith(t, p)
			ch, err := s.ReadChannel(t.Context(), tt.slot)
			if err != nil {
				t.Fatalf("ReadChannel: %v - an empty slot must never be an error that aborts a caller's ReadAll", err)
			}
			if !ch.Empty() {
				t.Errorf("Data = %+v, want nil - Data == nil is the SOLE discriminator between empty and populated", ch.Data)
			}
			if ch.Slot != tt.slot {
				t.Errorf("Slot = %q, want %q - the slot is carried through even on an empty answer", ch.Slot, tt.slot)
			}
		})
	}
}
func (fx Fixture) tReadChannel_WrongLengthAborts(t *testing.T) {
	for _, n := range []int{24, 26, 27, 39} {
		rec := make([]byte, n)
		copy(rec, fx.Golden)
		ch, err := fx.readOne(t, 42, "042", rec)
		if err == nil {
			t.Fatalf("a %d-byte record read successfully as %+v", n, ch)
		}
		if !errors.Is(err, civ.ErrRecordLength) {
			t.Errorf("a %d-byte record gave %v, want one satisfying errors.Is(err, civ.ErrRecordLength)", n, err)
		}
		if !ch.Empty() {
			t.Errorf("a %d-byte record produced a populated channel alongside its error", n)
		}
	}
}
func (fx Fixture) tReadChannel_UndecodableEnumIsAnError(t *testing.T) {
	for _, tt := range []struct {
		name   string
		rec    []byte
		offset int
	}{
		{"unlisted mode code 0x06 at ⑨", fx.withRecord(6, 0x06), 6},
		{"unlisted mode code 0x11 at ⑨", fx.withRecord(6, 0x11), 6},
		{"filter 0x00 at ⑩", fx.withRecord(7, 0x00), 7},
		{"filter 0x04 at ⑩", fx.withRecord(7, 0x04), 7},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fx.readOne(t, 42, "042", tt.rec)
			if err == nil {
				t.Fatal("the record decoded; an unlisted enum value must fail rather than be read as something")
			}
			if !errors.Is(err, civ.ErrParse) {
				t.Errorf("err = %v, want one satisfying errors.Is(err, civ.ErrParse)", err)
			}
			if !containsOffset(err.Error(), tt.offset) {
				t.Errorf("err = %v, want a message naming offset %d", err, tt.offset)
			}
		})
	}
}
func containsOffset(msg string, offset int) bool {
	for _, form := range []string{
		fmt.Sprintf("offset %d", offset),
		fmt.Sprintf("byte %d", offset),
		fmt.Sprintf(" %d ", offset),
	} {
		if strings.Contains(msg, form) {
			return true
		}
	}
	return false
}
func (fx Fixture) tReadChannel_OutOfDomainToneReadsAsUnknown(t *testing.T) {
	for _, tt := range []struct {
		deciHz uint64
		state  codeplug.FieldState
		why    string
	}{
		{0, codeplug.Unknown, "0 Hz is not a tone; a tone-OFF channel plausibly reads back 000.0"},
		{3000, codeplug.Unknown, "above the printed 100Hz digit's 0-2 range"},
		{1, codeplug.Known, "the declared floor, and itself admissible"},
		{885, codeplug.Known, "88.5 Hz"},
		{2999, codeplug.Known, "the declared ceiling, and itself admissible"},
	} {
		t.Run(fmt.Sprintf("%d deci-Hz", tt.deciHz), func(t *testing.T) {
			rec := fx.withRecord(9, BCD3(tt.deciHz)...)
			copy(rec[12:], BCD3(tt.deciHz))
			ch, err := fx.readOne(t, 42, "042", rec)
			if err != nil {
				t.Fatalf("ReadChannel: %v", err)
			}
			for name, got := range map[string]codeplug.ToneField{"ToneTx": ch.Data.ToneTx, "ToneRx": ch.Data.ToneRx} {
				if got.State != tt.state {
					t.Errorf("%s.State = %q, want %q (%s)", name, got.State, tt.state, tt.why)
				}
				if tt.state == codeplug.Known && got.Value != spec.Tone(tt.deciHz) {
					t.Errorf("%s.Value = %d, want %d", name, got.Value, tt.deciHz)
				}
				if tt.state == codeplug.Unknown && got.Value != 0 {
					t.Errorf("%s carries the value %d alongside Unknown; only a Known field may carry one", name, got.Value)
				}
			}
			for _, issue := range codeplug.Validate(fx.fullCodeplug(ch), fx.Unverified()) {
				if issue.Severity == codeplug.SeverityError {
					t.Errorf("codeplug.Validate refused a channel this driver produced: %s (%s)", issue.Msg, issue.Field)
				}
			}
		})
	}
}
func BCD3(deciHz uint64) []byte {
	d := [6]byte{}
	for i := 5; i >= 0; i-- {
		d[i] = byte(deciHz % 10)
		deciHz /= 10
	}
	return []byte{d[0]<<4 | d[1], d[2]<<4 | d[3], d[4]<<4 | d[5]}
}
func (fx Fixture) fullCodeplug(ch codeplug.Channel) *codeplug.Codeplug {
	caps := fx.Unverified()
	cp := &codeplug.Codeplug{
		Radio: codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
	}
	for _, b := range caps.Banks {
		for _, slot := range b.Slots {
			if slot == ch.Slot {
				cp.Channels = append(cp.Channels, ch)
				continue
			}
			cp.Channels = append(cp.Channels, codeplug.Channel{Slot: slot})
		}
	}
	return cp
}
func (fx Fixture) tReadChannel_AnswerForAnotherChannelIsCaughtBeforeAnyUse(t *testing.T) {
	allFF := make([]byte, len(fx.Golden))
	for i := range allFF {
		allFF[i] = 0xFF
	}
	for _, tt := range []struct {
		name string
		rec  []byte
	}{
		{"a populated record for the wrong channel", fx.Golden},
		{"an all-FF record for the wrong channel", allFF},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := RadioImage{
				IDToken: []byte{fx.Addr},
				Records: map[int][]byte{1: fx.Golden, 42: tt.rec},
				AnswerAddress: func(asked int) int {
					if asked == 1 {
						return 1
					}
					return 7
				},
			}
			p := fx.port(t, img)
			s := fx.openWith(t, p)
			before := s.AnswerMismatches()

			ch, err := s.ReadChannel(t.Context(), "042")
			if err == nil {
				t.Fatalf("ReadChannel succeeded with %+v; the answer named channel 7 and the read asked about 42", ch)
			}
			if !ch.Empty() {
				t.Error("a populated channel was returned alongside the error - nothing may be mapped from a mis-addressed answer")
			}
			if !errors.Is(err, driver.ErrAnswerMismatch) {
				t.Errorf("err = %v, want one satisfying errors.Is(err, driver.ErrAnswerMismatch)", err)
			}
			var mismatch *driver.AnswerMismatchError[civ.ChannelAddress]
			if !errors.As(err, &mismatch) {
				t.Fatalf("err = %v, want an *driver.AnswerMismatchError[civ.ChannelAddress] naming both channels", err)
			}
			if mismatch.Requested.Channel != 42 || mismatch.Answered.Channel != 7 {
				t.Errorf("*driver.AnswerMismatchError[civ.ChannelAddress] = {Requested: %s, Answered: %s}, want {ch42, ch7}", mismatch.Requested, mismatch.Answered)
			}
			if got := s.AnswerMismatches(); got != before+1 {
				t.Errorf("the diagnostic counter went %d -> %d, want one increment", before, got)
			}
		})
	}
}
func (fx Fixture) tReadChannel_UnmappedNibblesAreNotSmuggledThrough(t *testing.T) {
	for _, row := range fx.SmuggleRows {
		tt := struct {
			name string
			rec  []byte
		}{row.Name, fx.withRecord(row.Off, row.Val)}
		t.Run(tt.name, func(t *testing.T) {
			ch, err := fx.readOne(t, 42, "042", tt.rec)
			if err != nil {
				t.Fatalf("ReadChannel: %v - E6 unmaps the nibble; it does not make the channel unreadable", err)
			}
			if ch.Data.ScanSkip.State != codeplug.Unavailable {
				t.Errorf("ScanSkip = %+v, want Unavailable - the SELECT nibble must never be smuggled into a boolean", ch.Data.ScanSkip)
			}
			if ch.Data.DataMode.State != codeplug.Unavailable {
				t.Errorf("DataMode = %+v, want Unavailable - the data-mode nibble must never be smuggled into a boolean", ch.Data.DataMode)
			}
			if ch.Data.ToneMode != (codeplug.StringField{State: codeplug.Known, Value: "TONE"}) {
				t.Errorf("ToneMode = %+v, want Known TONE - byte ⑪'s LOW nibble IS mapped, whatever its high nibble carries", ch.Data.ToneMode)
			}
		})
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// withRecord returns a copy of goldenRecord with b written at offset.
func withRecord(offset int, bytes ...byte) []byte {
	rec := append([]byte(nil), goldenRecord...)
	copy(rec[offset:], bytes)
	return rec
}

// readOne opens a session against an image carrying rec at channel ch and
// reads slot.
func readOne(t *testing.T, ch int, slot string, rec []byte) (codeplug.Channel, error) {
	t.Helper()
	img := occupiedRadio()
	img.records = map[int][]byte{1: goldenRecord, ch: rec}
	p := newScriptedPort(t, img)
	s := openWith(t, p)
	return s.ReadChannel(t.Context(), slot)
}

// TestSlotAddressRoundTrip pins the slot map in both directions over all
// 101 addressable channels, and pins what is NOT a slot.
//
// MATRIX §3.15(d): P1 AND P2 ARE NOT A SEPARATE BANK IN THE WIRE PROTOCOL.
// They are two more values of the same two-byte selector — 100 and 101 in
// one contiguous space that core/civ/ic7760's profile declares as base
// MEM 1..99 plus one ExtraRange 100..101. This project models them as a
// SCAN bank only
// because the neutral memory model needs the distinction between a memory
// and a scan edge.
func TestSlotAddressRoundTrip(t *testing.T) {
	for ch := 1; ch <= 99; ch++ {
		slot := fmt.Sprintf("%03d", ch)
		a, bank, err := slotToAddress(slot)
		if err != nil {
			t.Fatalf("slotToAddress(%q): %v", slot, err)
		}
		if a.Channel != ch || a.Group != 0 {
			t.Errorf("slotToAddress(%q) = %s, want channel %d in a flat space", slot, a, ch)
		}
		if bank != spec.BankMemory {
			t.Errorf("slotToAddress(%q) bank = %s, want MEM", slot, bank)
		}
		back, err := addressToSlot(a)
		if err != nil || back != slot {
			t.Errorf("addressToSlot(%s) = (%q, %v), want (%q, nil)", a, back, err, slot)
		}
	}
	for _, tt := range []struct {
		slot string
		ch   int
	}{{"P1", 100}, {"P2", 101}} {
		a, bank, err := slotToAddress(tt.slot)
		if err != nil {
			t.Fatalf("slotToAddress(%q): %v", tt.slot, err)
		}
		if a.Channel != tt.ch {
			t.Errorf("slotToAddress(%q) = %s, want channel %d", tt.slot, a, tt.ch)
		}
		if bank != spec.BankScan {
			t.Errorf("slotToAddress(%q) bank = %s, want SCAN", tt.slot, bank)
		}
		back, err := addressToSlot(a)
		if err != nil || back != tt.slot {
			t.Errorf("addressToSlot(%s) = (%q, %v), want (%q, nil)", a, back, err, tt.slot)
		}
	}
	for _, bad := range []string{"000", "100", "P0", "P3", "1", "", "G05-012", "0001", "abc"} {
		if _, _, err := slotToAddress(bad); err == nil {
			t.Errorf("slotToAddress(%q) succeeded; it is not a slot on this radio", bad)
		}
	}
	for _, bad := range []civ.ChannelAddress{{Channel: 0}, {Channel: 102}, {Channel: -1}, {Group: 1, Channel: 1}} {
		if _, err := addressToSlot(bad); err == nil {
			t.Errorf("addressToSlot(%s) succeeded; it is not an address on this radio", bad)
		}
	}
}

// TestReadChannel_MapsEveryField is the exhaustive mapping table, with NO
// FIELD OMITTED — a partial table is how a field ends up in neither the
// "Known" list nor the "Unavailable" list and is quietly never decided
// about.
func TestReadChannel_MapsEveryField(t *testing.T) {
	ch, err := readOne(t, 7, "007", goldenRecord)
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
	// 885 and 1000 deci-Hz are both INSIDE the declared {1, 2999, 1}
	// domain, so both are Known.
	if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: 885}) {
		t.Errorf("ToneTx = %+v, want Known 885", d.ToneTx)
	}
	if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: 1000}) {
		t.Errorf("ToneRx = %+v, want Known 1000", d.ToneRx)
	}
	if d.Tag != "HOME QTH01" {
		t.Errorf("Tag = %q, want %q", d.Tag, "HOME QTH01")
	}

	// E6-UNMAPPED: never decoded, so Unavailable rather than Unknown.
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

	// THE RECORD HAS NO SUCH FIELD: Unavailable, not Unknown. Unknown
	// would claim the radio has one and this read did not learn it.
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

	// The Yaesu-shaped plain fields carry no state at all and this radio
	// has none of them, so they stay at their zero values and NOTHING is
	// guessed into them. codeplug.Validate's CTCSS-state and shift checks
	// key on the VOCABULARY being supplied, and this driver supplies
	// neither, so neither check runs.
	if d.ClarHz != 0 || d.RxClar || d.TxClar {
		t.Errorf("a clarifier value was invented: %d/%v/%v", d.ClarHz, d.RxClar, d.TxClar)
	}
	if d.CTCSS != "" || d.Shift != "" {
		t.Errorf("a Yaesu vocabulary value was invented: CTCSS %q, Shift %q", d.CTCSS, d.Shift)
	}
	drivertest.AssertFreshReadSaveLoad(t, ch, capabilitiesSimulated(), codeplug.Load)
}

// TestReadChannel_EmptySlotIsAnEmptyChannel — the empty-slot hook and tier
// ruling T4.
//
// TWO SEPARATE REGISTER ENTRIES, and ONE CAPTURE CANNOT ESTABLISH BOTH:
// D5 entry 2(a) / register entry ic7760-empty-reply-fa is the FA reading;
// D5 entry 2(b) / register entry ic7760-empty-reply-ff is the all-FF one.
// The -fa lift clears MEMORY CHANNEL 99, so its scope excludes the scan
// edges; P1/P2 emptiness rides ic7760-scan-edge-record-shape — and that
// lift reads 01 00 only, so P2 is uncovered even by that.
//
// The rejection arm drives the FA THROUGH THE ENGINE rather than handing
// the driver an FA frame, because after Do there IS no frame: Engine.Do
// consumes the FA and returns transport.ErrRejected with nothing attached.
// A test that handed the driver an "FA frame" would be testing a shape the
// driver can never meet.
func TestReadChannel_EmptySlotIsAnEmptyChannel(t *testing.T) {
	allFF := make([]byte, len(goldenRecord))
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
			img := occupiedRadio()
			if tt.rec != nil {
				a, _, err := slotToAddress(tt.slot)
				if err != nil {
					t.Fatalf("slotToAddress: %v", err)
				}
				img.records[a.Channel] = tt.rec
			}
			p := newScriptedPort(t, img)
			s := openWith(t, p)
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

// TestReadChannel_WrongLengthAborts: a record at any length but 25 fails
// with an error satisfying errors.Is(err, civ.ErrRecordLength). No partial
// parse and no fake Unavailable channel (spec D4, adjudication 13).
//
// THIS IS THE FINGERPRINT BEING CONTINUOUS rather than one-shot, and it is
// DISTINCT from the empty-slot case above: A WRONG LENGTH IS A WRONG
// RADIO; AN ABSENT RECORD IS AN EMPTY SLOT.
func TestReadChannel_WrongLengthAborts(t *testing.T) {
	for _, n := range []int{24, 26, 27, 39} {
		rec := make([]byte, n)
		copy(rec, goldenRecord)
		ch, err := readOne(t, 42, "042", rec)
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

// TestReadChannel_UndecodableEnumIsAnError: a record whose ⑨ is 0x06, or
// whose ⑩ is 0x00, fails with a parse error naming the offset.
//
// The mode and filter enums are MANUAL-EVIDENCED, not assumed: matrix §1
// rows 4 and 21 read PDF p.18 (folio 17)'s "Operating mode" table at
// 400 dpi. Column ① prints ten codes and the enum is SPARSE — 06 and
// 09-11 are absent from the table and name nothing, and nothing may fill
// the gaps. Column ② "Filter setting" prints 01 FIL1, 02 FIL2, 03 FIL3
// and no default, so byte ⑩ carries only those three and inventing a
// fourth would be a radio claim.
func TestReadChannel_UndecodableEnumIsAnError(t *testing.T) {
	for _, tt := range []struct {
		name   string
		rec    []byte
		offset int
	}{
		{"unlisted mode code 0x06 at ⑨", withRecord(6, 0x06), 6},
		{"unlisted mode code 0x11 at ⑨", withRecord(6, 0x11), 6},
		{"filter 0x00 at ⑩", withRecord(7, 0x00), 7},
		{"filter 0x04 at ⑩", withRecord(7, 0x04), 7},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readOne(t, 42, "042", tt.rec)
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

// containsOffset reports whether msg mentions the numeric offset, in
// decimal or as an offset-N phrase. A loose check on purpose: the exact
// wording is the codec's, and pinning it here would make this test a
// second copy of core/civ's own message format.
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

// TestReadChannel_OutOfDomainToneReadsAsUnknown — tier ruling T1(3), and
// the question REV 2 left unasked.
//
// A record whose tone bytes decode to 0 is PLAUSIBLE on a tone-OFF
// channel: the printed digit range includes 000.0. THE LAYERING: the civ
// layer is lossless and semantics-free and hands up the number 0 unharmed
// (T1(1)); the CAPABILITY does not admit it, because 0 Hz is not a tone
// and spec.ToneRange requires MinDeciHz > 0 (T1(2)); so the DRIVER is
// where it becomes Unknown, and A READ NEVER CONSTRUCTS A KNOWN VALUE
// codeplug.Validate WOULD THEN REFUSE.
func TestReadChannel_OutOfDomainToneReadsAsUnknown(t *testing.T) {
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
			rec := withRecord(9, bcd3(tt.deciHz)...)
			copy(rec[12:], bcd3(tt.deciHz))
			ch, err := readOne(t, 42, "042", rec)
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

			// THE END THIS ARM EXISTS TO PROTECT: whatever the wire said,
			// the channel this driver produces must pass the model
			// layer's own validation against this radio's capabilities.
			for _, issue := range codeplug.Validate(fullCodeplug(ch), capabilitiesUnverified()) {
				if issue.Severity == codeplug.SeverityError {
					t.Errorf("codeplug.Validate refused a channel this driver produced: %s (%s)", issue.Msg, issue.Field)
				}
			}
		})
	}
}

// bcd3 renders a tone in tenths of a hertz as this record's three-byte
// BIG-ENDIAN packed BCD. Written out here rather than taken from the
// encoder under test: a fixture built by the encoder would agree with a
// wrong byte order as happily as a right one. PDF p.24 (folio 23)'s three-cell strip
// prints cell 1 as "0 | 0" and then 100 Hz, 10 Hz, 1 Hz, 0.1 Hz — most
// significant pair first, and the OPPOSITE of the frequency field's
// convention on the same radio.
func bcd3(deciHz uint64) []byte {
	d := [6]byte{}
	for i := 5; i >= 0; i-- {
		d[i] = byte(deciHz % 10)
		deciHz /= 10
	}
	return []byte{d[0]<<4 | d[1], d[2]<<4 | d[3], d[4]<<4 | d[5]}
}

// fullCodeplug wraps one channel in a complete codeplug for this radio —
// every declared slot present, the one under test populated and the rest
// empty — so codeplug.Validate's completeness check has nothing to
// complain about and any Error it reports is about the channel itself.
func fullCodeplug(ch codeplug.Channel) *codeplug.Codeplug {
	caps := capabilitiesUnverified()
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

// TestReadChannel_AnswerForAnotherChannelIsCaughtBeforeAnyUse — tier
// ruling T2.
//
// The landed MemoryAnswerMatcher is envelope-only (to/from/cn/sc), so an
// answer for channel 7 satisfies the spec for a read of channel 42
// perfectly well. The driver therefore compares the decoded address
// against the one it asked for BEFORE ANY USE of the answer.
//
// THE ORDERING IS PROVEN, not merely asserted, by making the mis-addressed
// answer one that WOULD otherwise have been recognised as EMPTY: an
// all-FF record. If the mismatch check ran after recordIsAbsent, this read
// would return a cheerful empty channel instead of an error — so the
// all-FF row failing is what shows the check runs first. The populated row
// shows the same for record mapping.
func TestReadChannel_AnswerForAnotherChannelIsCaughtBeforeAnyUse(t *testing.T) {
	allFF := make([]byte, len(goldenRecord))
	for i := range allFF {
		allFF[i] = 0xFF
	}
	for _, tt := range []struct {
		name string
		rec  []byte
	}{
		{"a populated record for the wrong channel", goldenRecord},
		{"an all-FF record for the wrong channel", allFF},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := radioImage{
				idToken: []byte{0xB2},
				records: map[int][]byte{1: goldenRecord, 42: tt.rec},
				// Honest for the probe's channel 1, and mis-addressed for
				// everything else — so Open succeeds and the read under
				// test is the one that meets the mismatch.
				answerAddress: func(asked int) int {
					if asked == 1 {
						return 1
					}
					return 7
				},
			}
			p := newScriptedPort(t, img)
			s := openWith(t, p)
			before := s.AnswerMismatches()

			ch, err := s.ReadChannel(t.Context(), "042")
			if err == nil {
				t.Fatalf("ReadChannel succeeded with %+v; the answer named channel 7 and the read asked about 42", ch)
			}
			if !ch.Empty() {
				t.Error("a populated channel was returned alongside the error - nothing may be mapped from a mis-addressed answer")
			}
			if !errors.Is(err, ErrAnswerMismatch) {
				t.Errorf("err = %v, want one satisfying errors.Is(err, ErrAnswerMismatch)", err)
			}
			var mismatch *AnswerMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf("err = %v, want an *AnswerMismatchError naming both channels", err)
			}
			if mismatch.Want.Channel != 42 || mismatch.Got.Channel != 7 {
				t.Errorf("*AnswerMismatchError = {Want: %s, Got: %s}, want {ch42, ch7}", mismatch.Want, mismatch.Got)
			}
			if got := s.AnswerMismatches(); got != before+1 {
				t.Errorf("the diagnostic counter went %d -> %d, want one increment", before, got)
			}
		})
	}
}

// TestReadChannel_UnmappedNibblesAreNotSmuggledThrough: a record whose
// byte 0 is 0x02 (SELECT ★2) or whose byte 8 is 0x21 (DATA 2 + TONE) still
// READS successfully, with ScanSkip and DataMode Unavailable and ToneMode
// Known "TONE".
//
// E6 unmaps those nibbles; it does not make the channel unreadable. TASK
// 12 IS WHERE SUCH A CHANNEL BECOMES UNWRITABLE.
func TestReadChannel_UnmappedNibblesAreNotSmuggledThrough(t *testing.T) {
	for _, tt := range []struct {
		name string
		rec  []byte
	}{
		{"byte 0 = 0x02, SELECT ★2", withRecord(0, 0x02)},
		{"byte 0 = 0x03, SELECT ★3", withRecord(0, 0x03)},
		{"byte 8 = 0x21, DATA 2 + TONE", withRecord(8, 0x21)},
		{"byte 8 = 0x31, DATA 3 + TONE", withRecord(8, 0x31)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := readOne(t, 42, "042", tt.rec)
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

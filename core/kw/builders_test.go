// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"strings"
	"testing"
)

// populatedRecord is a valid, non-empty channel for slot: 14.250 MHz USB,
// no tone, everything else at its printed-quiet value. Every build refusal
// below spoils exactly one of its fields.
func populatedRecord(slot Slot) Record {
	return Record{
		Slot:     slot,
		FreqHz:   14_250_000,
		Mode:     ModeUSB,
		Byte19:   '0',
		ToneMode: ToneModeOff,
		Byte28:   '0',
		Byte3940: "00",
		Byte41:   '0',
		Name:     "TEST",
	}
}

// mustSlot resolves number under l or fails the test.
func mustSlot(t *testing.T, l Layout, number int, half ScanHalf) Slot {
	t.Helper()
	s, err := l.NewSlot(number, half)
	if err != nil {
		t.Fatalf("NewSlot(%d, %v): %v", number, half, err)
	}
	return s
}

// TestBuildMRRead_IsSevenBytes pins the read request both books print:
// "M R P1 P2 P3 P3 ;" (590:1442, 480:918). The 590SG chart prints its
// terminator cell as ':' — erratum E1 — and the frame is a ';' frame.
func TestBuildMRRead_IsSevenBytes(t *testing.T) {
	tests := []struct {
		name   string
		layout Layout
		number int
		half   ScanHalf
		want   string
	}{
		{"TS-590SG memory 007", layout590SG(), 7, ScanHalfNone, "MR0007;"},
		{"TS-590SG section 103 start", layout590SG(), 103, ScanLower, "MR0103;"},
		{"TS-590SG section 103 end", layout590SG(), 103, ScanUpper, "MR1103;"},
		{"TS-590SG extension 115", layout590SG(), 115, ScanHalfNone, "MR0115;"},
		{"TS-480 memory 007", layout480(), 7, ScanHalfNone, "MR0007;"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.layout.BuildMRRead(mustSlot(t, tt.layout, tt.number, tt.half))
			if err != nil {
				t.Fatalf("BuildMRRead = %v, want nil", err)
			}
			if got := string(cmd.Bytes()); got != tt.want {
				t.Errorf("BuildMRRead = %q, want %q", got, tt.want)
			}
			if len(cmd.Bytes()) != MRReadLen {
				t.Errorf("BuildMRRead produced %d bytes, want %d", len(cmd.Bytes()), MRReadLen)
			}
		})
	}
}

// TestBuildMWSet_IsFiftyBytesAndRoundTrips: the builder emits exactly 50
// bytes, and what it emits is what this layout's own parser reads back.
func TestBuildMWSet_IsFiftyBytesAndRoundTrips(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
	}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
		t.Run(tt.name, func(t *testing.T) {
			l := tt.layout
			rec := populatedRecord(mustSlot(t, l, 7, ScanHalfNone))
			cmd, err := l.BuildMWSet(rec)
			if err != nil {
				t.Fatalf("BuildMWSet = %v, want nil", err)
			}
			frame := cmd.Bytes()
			if len(frame) != RecordLen {
				t.Fatalf("BuildMWSet produced %d bytes, want exactly %d — the short erase form must never escape (A5, decision 8)", len(frame), RecordLen)
			}
			if string(frame[:2]) != "MW" || frame[RecordLen-1] != ';' {
				t.Fatalf("BuildMWSet = %q, want an \"MW\" frame terminated with ';'", frame)
			}

			// The MR answer and the MW Set frame are the same grid, so the
			// parser reads the builder's own bytes back with the prefix
			// swapped — which is what makes the two halves of this codec one
			// codec rather than two independent readings of one chart.
			asAnswer := append([]byte("MR"), frame[2:]...)
			back, err := l.ParseMRAnswer(asAnswer)
			if err != nil {
				t.Fatalf("ParseMRAnswer of the builder's own bytes = %v, want nil", err)
			}
			if back.FreqHz != rec.FreqHz || back.Mode != rec.Mode || back.Name != rec.Name {
				t.Errorf("round trip = %+v, want the frequency, mode and name of %+v", back, rec)
			}
		})
	}
}

// TestBuildMWSet_P1IsDerivedFromTheSlotClassNotTheSplitState is M9, pinned
// BOTH WAYS.
//
// The books give P1 two jobs. On an ordinary memory channel it selects
// simplex or split on the 590 pair (590:1519-1520) and the RX or TX
// frequency on the 480 (480:951); on a section-defined channel it selects
// the START or the END frequency — "set parameter P1 to 0 to enter the Start
// frequency, then set P1 to 1 to set the End frequency" (590:1529-1531).
// A builder that took P1 from a channel's split state would write '0' into a
// section channel's END slot, putting the START frequency where the user had
// edited the END: a silent data loss.
//
// THE 'U' CASE IS THE RED PROOF. An implementation that emitted P1='0'
// unconditionally passes every other case in this file and fails only here.
func TestBuildMWSet_P1IsDerivedFromTheSlotClassNotTheSplitState(t *testing.T) {
	l := layout590SG()
	tests := []struct {
		name   string
		number int
		half   ScanHalf
		wantP1 byte
	}{
		{"an ordinary memory slot", 7, ScanHalfNone, '0'},
		{"a section channel's start frequency", 103, ScanLower, '0'},
		{"a section channel's end frequency", 103, ScanUpper, '1'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slot := mustSlot(t, l, tt.number, tt.half)
			if got := slot.P1(); got != tt.wantP1 {
				t.Errorf("Slot.P1() = %q, want %q", got, tt.wantP1)
			}
			cmd, err := l.BuildMWSet(populatedRecord(slot))
			if err != nil {
				t.Fatalf("BuildMWSet = %v, want nil", err)
			}
			if got := cmd.Bytes()[recP1Off]; got != tt.wantP1 {
				t.Errorf("MW P1 = %q, want %q", got, tt.wantP1)
			}
		})
	}
}

// TestBuildMWSet_RefusesARecordWhoseAnswerP1DisagreesWithItsSlot is the
// second half of M9's guard: a record read from a section channel's START
// half cannot be written back to its END half without the caller saying so.
func TestBuildMWSet_RefusesARecordWhoseAnswerP1DisagreesWithItsSlot(t *testing.T) {
	l := layout590SG()
	rec := populatedRecord(mustSlot(t, l, 103, ScanUpper))
	rec.AnswerP1 = '0' // as read from the START half

	if _, err := l.BuildMWSet(rec); err == nil {
		t.Fatal("BuildMWSet accepted a record read with P1='0' for a slot whose class derives '1'")
	}

	rec.AnswerP1 = '1'
	if _, err := l.BuildMWSet(rec); err != nil {
		t.Errorf("BuildMWSet refused a record whose AnswerP1 agrees with its slot: %v", err)
	}
}

// TestBuildMWSet_RefusesASlotFromAnotherLayout: a Slot is a value and may
// have been minted anywhere, so the receiver re-resolves its number. Without
// that, a TS-590SG extension slot would be written to a TS-480 whose slot
// space stops at 99.
func TestBuildMWSet_RefusesASlotFromAnotherLayout(t *testing.T) {
	foreign := mustSlot(t, layout590SG(), 115, ScanHalfNone)
	if _, err := layout480().BuildMWSet(populatedRecord(foreign)); err == nil {
		t.Error("the TS-480 built a write for slot 115, which is outside its slot space")
	}
	if _, err := layout590S().BuildMWSet(populatedRecord(foreign)); err == nil {
		t.Error("the TS-590S built a write for slot 115, which is above the ceiling A12 leaves it")
	}
}

// TestBuildMWSet_RefusesTheModeNibblesThatNameNoMode is A18b. Both books
// call nibbles 0 and 8 "None (setting failure)" or "Not used" (590:1353,
// 590:1362; 480:843, 480:853) without saying what a Set carrying one does,
// and the programme has no reason to send one: a codeplug channel either has
// a mode or is empty, and an empty channel is not written.
func TestBuildMWSet_RefusesTheModeNibblesThatNameNoMode(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
	}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
		t.Run(tt.name, func(t *testing.T) {
			for _, m := range []Mode{ModeNone, ModeTune} {
				rec := populatedRecord(mustSlot(t, tt.layout, 7, ScanHalfNone))
				rec.Mode = m
				if _, err := tt.layout.BuildMWSet(rec); err == nil {
					t.Errorf("BuildMWSet accepted mode nibble %q", byte(m))
				}
			}
		})
	}
}

// TestBuildMWSet_RefusesAToneIndexOutsideItsPrintedChart is A21 on the write
// side, where it matters most: "An entered value of 43 or higher results in
// an error" is printed for TN (590:2309) and nothing at all for CN, so
// refusing rather than clamping is this programme's choice and it is the one
// that never silently stores a tone the user did not ask for.
func TestBuildMWSet_RefusesAToneIndexOutsideItsPrintedChart(t *testing.T) {
	l := layout590SG()
	base := func() Record {
		r := populatedRecord(mustSlot(t, l, 7, ScanHalfNone))
		r.ToneMode = ToneModeTone
		return r
	}

	ok := base()
	ok.ToneIndex, ok.CTCSSIndex = MaxToneIndex, MaxCTCSSIndex
	if _, err := l.BuildMWSet(ok); err != nil {
		t.Errorf("BuildMWSet refused the last index of each printed chart: %v", err)
	}

	high := base()
	high.ToneIndex = MaxToneIndex + 1
	if _, err := l.BuildMWSet(high); err == nil {
		t.Errorf("BuildMWSet accepted tone index %d, and TN prints 00 ~ %d", high.ToneIndex, MaxToneIndex)
	}

	highCN := base()
	highCN.CTCSSIndex = MaxCTCSSIndex + 1
	if _, err := l.BuildMWSet(highCN); err == nil {
		t.Errorf("BuildMWSet accepted CTCSS index %d, and CN prints 00 ~ %d", highCN.CTCSSIndex, MaxCTCSSIndex)
	}

	negative := base()
	negative.ToneIndex = -1
	if _, err := l.BuildMWSet(negative); err == nil {
		t.Error("BuildMWSet accepted a negative tone index")
	}
}

// TestBuildMWSet_NamePaddingIsEightSpaces is A1, cited at the site: the name
// is padded to 8 bytes with SPACES on write and right-trimmed on read.
// Neither book states the rule for P16; the 480's KY gives the
// same-document precedent for a different command (480:785-787), and the
// lift is a write-then-read on each registry row.
func TestBuildMWSet_NamePaddingIsEightSpaces(t *testing.T) {
	l := layout590SG()
	for _, name := range []string{"", "A", "ABC", "ABCDEFGH"} {
		rec := populatedRecord(mustSlot(t, l, 7, ScanHalfNone))
		rec.Name = name
		cmd, err := l.BuildMWSet(rec)
		if err != nil {
			t.Fatalf("BuildMWSet with name %q = %v, want nil", name, err)
		}
		got := string(cmd.Bytes()[recNameOff : recNameOff+recNameLen])
		want := name + strings.Repeat(" ", recNameLen-len(name))
		if got != want {
			t.Errorf("P16 for name %q = %q, want %q", name, got, want)
		}
	}
}

// TestBuildMWSet_RefusesANameTooLongOrOutsideA2sCharset. A2's claim is
// bounded at 0x7F: 0x80-0xFF is unevidenced AND unclaimed, so this codec
// refuses those bytes by its own rule and says nothing about what a radio
// would do with one.
func TestBuildMWSet_RefusesANameTooLongOrOutsideA2sCharset(t *testing.T) {
	l := layout590SG()
	bad := []string{
		"ABCDEFGHI",              // nine bytes
		"AB;CD",                  // 590:1577, "';' cannot be used"
		"AB\x00CD",               // 480:127-129, the control codes
		"AB\x7fCD",               // the boundary byte no sentence reaches
		"AB\xffCD",               // above A2's bound
		strings.Repeat("A", 100), // grossly over
	}
	for _, name := range bad {
		rec := populatedRecord(mustSlot(t, l, 7, ScanHalfNone))
		rec.Name = name
		if _, err := l.BuildMWSet(rec); err == nil {
			t.Errorf("BuildMWSet accepted the name %q", name)
		}
	}
}

// TestBuildMWSet_EmitsEveryPrintedFixedByte, per layout: the 590SG's
// thirteen hard-wired bytes and the 480's sixteen.
func TestBuildMWSet_EmitsEveryPrintedFixedByte(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
	}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := tt.layout.BuildMWSet(populatedRecord(mustSlot(t, tt.layout, 7, ScanHalfNone)))
			if err != nil {
				t.Fatalf("BuildMWSet = %v, want nil", err)
			}
			frame := cmd.Bytes()
			fixed := tt.layout.PrintedFixed()
			if len(fixed) == 0 {
				t.Fatal("the layout declares no printed-fixed bytes, so this test would pass vacuously")
			}
			for _, ff := range fixed {
				got := string(frame[ff.Pos-1 : ff.Pos-1+len(ff.Printed)])
				if got != ff.Printed {
					t.Errorf("positions %d-%d = %q, want the printed %q", ff.Pos, ff.Pos+len(ff.Printed)-1, got, ff.Printed)
				}
			}
		})
	}
}

// TestBuildMWSet_RefusesAnEmptyRecord: the short MW erases a channel
// (590:1579-1581) and this milestone never builds it (decision 8, A5), so a
// record marked empty must not become a 50-byte frame of zeroes either. An
// empty channel is not written; it is left alone.
func TestBuildMWSet_RefusesAnEmptyRecord(t *testing.T) {
	l := layout590SG()
	rec := populatedRecord(mustSlot(t, l, 7, ScanHalfNone))
	rec.Empty = true
	if _, err := l.BuildMWSet(rec); err == nil {
		t.Error("BuildMWSet built a frame for a record marked Empty")
	}

	zero := populatedRecord(mustSlot(t, l, 7, ScanHalfNone))
	zero.FreqHz = 0
	if _, err := l.BuildMWSet(zero); err == nil {
		t.Error("BuildMWSet built a frame for a channel at 0 Hz")
	}
}

// TestBuildMWSet_ZeroLayoutBuildsNothing covers all three of the zero
// Layout's gates — the MW Set, the MR read and NewSlot — because a gate that
// authorised bytes on behalf of no radio is the one failure the whole layout
// arrangement exists to prevent, and one of the three passing while another
// did not would be a partial answer.
func TestBuildMWSet_ZeroLayoutBuildsNothing(t *testing.T) {
	var l Layout
	if _, err := l.BuildMWSet(Record{}); err == nil {
		t.Error("a zero Layout built an MW Set")
	}
	if _, err := l.BuildMRRead(Slot{}); err == nil {
		t.Error("a zero Layout built an MR read")
	}
	if _, err := l.NewSlot(7, ScanHalfNone); err == nil {
		t.Error("a zero Layout resolved a slot")
	}
}

// TestNewSlot_ASectionChannelMustNameItsHalf: a section channel holds two
// frequencies and a record naming one is incomplete without saying which;
// an ordinary memory slot holds one and has no half to name.
func TestNewSlot_ASectionChannelMustNameItsHalf(t *testing.T) {
	l := layout590SG()
	if _, err := l.NewSlot(103, ScanHalfNone); err == nil {
		t.Error("NewSlot accepted a section channel with no half named")
	}
	if _, err := l.NewSlot(7, ScanUpper); err == nil {
		t.Error("NewSlot gave an ordinary memory slot a half")
	}
	if _, err := l.NewSlot(115, ScanUpper); err == nil {
		t.Error("NewSlot gave an extension channel a half, and the book explains nothing about one (A11)")
	}
}

// TestBuildMWSet_RefusesARecordWhoseP14WasNeverSet. NO BYTE IS EMITTED
// WITHOUT A SOURCE. Bytes 39-40 are the FM Normal/Narrow flag on the 590
// pair (590:1569-1571) and the tuning step index on the 480 (480:979), and
// neither book says what value means "no change" — that absence IS A22 and
// A23. Writing "00" for a caller that said nothing would invent the one byte
// those two entries exist because nobody can supply.
func TestBuildMWSet_RefusesARecordWhoseP14WasNeverSet(t *testing.T) {
	for _, tt := range []struct {
		name   string
		layout Layout
	}{{"TS-590SG", layout590SG()}, {"TS-480", layout480()}} {
		t.Run(tt.name, func(t *testing.T) {
			for _, p14 := range []string{"", "0", "000"} {
				rec := populatedRecord(mustSlot(t, tt.layout, 7, ScanHalfNone))
				rec.Byte3940 = p14
				if _, err := tt.layout.BuildMWSet(rec); err == nil {
					t.Errorf("BuildMWSet accepted a record whose P14 was %q", p14)
				}
			}
		})
	}
}

// TestBuildMWSet_TheAxisRefusalsRunInTheBUILDDirectionToo, and per radio.
// The parse-side difference pins (difference_test.go) say what each row
// ADMITS; these say what each row will put on the wire, which is the
// direction that reaches a user's radio. Byte 28 and byte 41 are a live
// field on the 590 pair and a printed constant on the 480, so '1' is a legal
// write on one row and a refusal on the other; bytes 39-40 run the other way,
// with "05" a legal step index on the 480 and a byte the 590 book never
// prints.
func TestBuildMWSet_TheAxisRefusalsRunInTheBUILDDirectionToo(t *testing.T) {
	sg, ts480 := layout590SG(), layout480()

	// Byte 28: FILTER B.
	filterB := populatedRecord(mustSlot(t, sg, 7, ScanHalfNone))
	filterB.Byte28 = '1'
	if _, err := sg.BuildMWSet(filterB); err != nil {
		t.Errorf("the TS-590SG refused to write FILTER B (590:1560-1563): %v", err)
	}
	filterB480 := populatedRecord(mustSlot(t, ts480, 7, ScanHalfNone))
	filterB480.Byte28 = '1'
	if _, err := ts480.BuildMWSet(filterB480); err == nil {
		t.Error("the TS-480 built a write carrying '1' at byte 28, where its book prints \"Always 0\" (480:973)")
	}

	// Byte 41: Channel Lockout ON.
	lockout := populatedRecord(mustSlot(t, sg, 7, ScanHalfNone))
	lockout.Byte41 = '1'
	if _, err := sg.BuildMWSet(lockout); err != nil {
		t.Errorf("the TS-590SG refused to write Channel Lockout ON (590:1572-1574): %v", err)
	}
	lockout480 := populatedRecord(mustSlot(t, ts480, 7, ScanHalfNone))
	lockout480.Byte41 = '1'
	if _, err := ts480.BuildMWSet(lockout480); err == nil {
		t.Error("the TS-480 built a write carrying '1' at byte 41, where its book prints \"Always 0\" (480:982)")
	}

	// Bytes 39-40: a step index the 590 book never prints.
	step := populatedRecord(mustSlot(t, ts480, 7, ScanHalfNone))
	step.Byte3940 = "05"
	if _, err := ts480.BuildMWSet(step); err != nil {
		t.Errorf("the TS-480 refused to write step index 05 (480:1494-1500): %v", err)
	}
	step590 := populatedRecord(mustSlot(t, sg, 7, ScanHalfNone))
	step590.Byte3940 = "05"
	if _, err := sg.BuildMWSet(step590); err == nil {
		t.Error("the TS-590SG built a write carrying \"05\" at bytes 39-40, where its P14 prints only \"00\" and \"01\"")
	}

	// Byte 19 carries the same two values on both rows and so cannot be told
	// apart by a frame; what each row refuses is a THIRD value, and its
	// refusal names its own meaning.
	for _, tt := range []struct {
		name     string
		layout   Layout
		wantText string
	}{{"TS-590SG", sg, "Byte19DataMode"}, {"TS-480", ts480, "Byte19Lockout"}} {
		rec := populatedRecord(mustSlot(t, tt.layout, 7, ScanHalfNone))
		rec.Byte19 = '2'
		_, err := tt.layout.BuildMWSet(rec)
		if err == nil {
			t.Fatalf("%s built a write carrying '2' at byte 19", tt.name)
		}
		if !strings.Contains(err.Error(), tt.wantText) {
			t.Errorf("%s refusal = %q, want it to name %s", tt.name, err, tt.wantText)
		}
	}
}

// TestSlotWire_HasNoPermissiveDefaultAndNoSilentTruncation.
//
// slotWire is the last site in this package to read a layout axis, and it is
// reached only behind Configured(); that is exactly what the FT-891 Stage 0
// site had going for it too. A ZERO LAYOUT FAILS CLOSED (layout.go), so an
// unset byte-4 policy must refuse rather than emit three digits on behalf of
// no radio.
//
// The ceiling arm is an ASSERTION rather than the mechanism: NewLayout now
// refuses P2FixedZero alongside any slot above 99 (480:953 against 480:955),
// so no layout this package mints can reach it. It is pinned here because a
// Slot is a value that may have been minted anywhere, and the failure it
// guards is a frame that names a DIFFERENT channel and reports success.
func TestSlotWire_HasNoPermissiveDefaultAndNoSilentTruncation(t *testing.T) {
	// The zero Layout: byte 4's policy is unset and no byte may be emitted.
	var zero Layout
	if got, err := zero.slotWire(Slot{number: 7, class: SlotMemory}); err == nil {
		t.Errorf("a zero Layout rendered byte 4 as %q; an unset axis must refuse, never default", got)
	}

	// A slot above the two-digit ceiling handed to a fixed-zero row. The
	// truncating form emitted "003" for channel 103 and returned success.
	if got, err := layout480().slotWire(Slot{number: 103, class: SlotMemory}); err == nil {
		t.Errorf("the TS-480 rendered slot 103 as %q; byte 4 is \"Always 0\" there (480:953) and P3 holds \"00 ~ 99\" (480:955), so there is no channel 103 to name", got)
	}

	// Both policies still render what their books print.
	for _, tt := range []struct {
		name   string
		layout Layout
		number int
		want   string
	}{
		{"the TS-480's fixed zero below 100", layout480(), 7, "007"},
		{"the 590 pair's hundreds digit below 100", layout590SG(), 7, "007"},
		{"the 590 pair's hundreds digit above 99", layout590SG(), 103, "103"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.layout.slotWire(Slot{number: tt.number, class: SlotMemory})
			if err != nil {
				t.Fatalf("slotWire = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("slotWire = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCheckSlot_RefusesASlotWhoseCLASSDisagreesWithThisLayout is the arm the
// two foreign-slot tests above cannot reach: slot 115 is outside the 480's
// and the 590S's spaces ENTIRELY, so both stop at the earlier
// SlotClassInvalid arm. The arm that matters is a slot number legal on both
// rows that resolves to a DIFFERENT class on each — the shape a TS-590SG
// section channel takes when it is handed to a row whose 103 is an ordinary
// memory.
//
// No layout in this tree resolves 103 to SlotMemory: the three fixtures
// happen to agree on 0-99 MEM and 100-109 SCAN. So the witness is an
// in-package Slot literal, which is precisely the value another radio's
// layout would mint and precisely what checkSlot exists to catch — "trusting
// the class the value carries would let a frame legal only on another radio
// out of this builder" (builders.go). A fourth fixture layout would be a row
// no book prints, which testlayouts_test.go must not carry.
func TestCheckSlot_RefusesASlotWhoseCLASSDisagreesWithThisLayout(t *testing.T) {
	l := layout590SG()
	for _, tt := range []struct {
		name string
		slot Slot
		want string
	}{
		{
			// THE ESCAPE. Neither half arm below can see this one — the
			// carried class is not SlotScan and the slot names no half — so
			// without the class arm checkSlot returns nil and a frame legal
			// only on another radio leaves this builder.
			"an ordinary memory number carrying another row's EXTENSION class",
			Slot{number: 7, class: SlotExtension},
			"resolved against another layout",
		},
		{
			"a section channel's number carrying another row's MEMORY class",
			Slot{number: 103, class: SlotMemory},
			"resolved against another layout",
		},
		{
			"a section channel with no half named",
			Slot{number: 103, class: SlotScan},
			"which of its two frequencies",
		},
		{
			"an ordinary memory slot carrying a half",
			Slot{number: 7, class: SlotMemory, half: ScanUpper},
			"no half to name",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := l.checkSlot("MW set", tt.slot); err == nil {
				t.Fatalf("checkSlot accepted %v carrying %v", tt.slot, tt.slot.class)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("refusal = %q, want it to say %q", err, tt.want)
			}
			if _, err := l.BuildMWSet(populatedRecord(tt.slot)); err == nil {
				t.Error("BuildMWSet built a frame for it")
			}
			if _, err := l.BuildMRRead(tt.slot); err == nil {
				t.Error("BuildMRRead built a frame for it")
			}
		})
	}
}

// TestBuildMRRead_RefusesASlotFromAnotherLayoutAndAnUnresolvedOne: the read
// side re-resolves the slot against ITS OWN receiver exactly as the write
// side does, because a Slot is a value and may have been minted anywhere.
func TestBuildMRRead_RefusesASlotFromAnotherLayoutAndAnUnresolvedOne(t *testing.T) {
	foreign := mustSlot(t, layout590SG(), 115, ScanHalfNone)
	if _, err := layout480().BuildMRRead(foreign); err == nil {
		t.Error("the TS-480 built a read for slot 115, which is outside its slot space")
	}
	if _, err := layout590S().BuildMRRead(foreign); err == nil {
		t.Error("the TS-590S built a read for slot 115, which is above the ceiling A12 leaves it")
	}
	if _, err := layout590SG().BuildMRRead(Slot{}); err == nil {
		t.Error("BuildMRRead accepted a slot that was never resolved against a layout")
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// slotOf resolves n on l or fails the test: every codec fixture below needs a
// slot and none of them is testing NewSlot itself.
func slotOf(t *testing.T, l Layout, n int) Slot {
	t.Helper()
	s, err := l.NewSlot(n)
	if err != nil {
		t.Fatalf("NewSlot(%d) on %s: %v", n, l.Model(), err)
	}
	return s
}

// assertParseRefusal checks the family's typed refusal and that the message
// says what it says. Every refusal in this package is a *kw.ParseError
// wrapping kw.ErrParse, which is what lets ONE driver-side errors.As arm
// cover both codecs.
func assertParseRefusal(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want one mentioning %q", want)
	}
	if !errors.Is(err, kw.ErrParse) {
		t.Errorf("errors.Is(err, kw.ErrParse) = false for %v", err)
	}
	var pe *kw.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("errors.As(err, **kw.ParseError) = false for %v", err)
	}
	if !strings.Contains(pe.Reason, want) {
		t.Errorf("reason = %q, want it to contain %q", pe.Reason, want)
	}
}

func TestNewSlot_ResolvesTheThreeClassesBothBooksPrint(t *testing.T) {
	for _, l := range []Layout{Layout890(), Layout990()} {
		for _, tt := range []struct {
			number int
			class  kw.SlotClass
		}{
			{0, kw.SlotMemory},
			{99, kw.SlotMemory},
			{100, kw.SlotScan},
			{109, kw.SlotScan},
			{110, kw.SlotExtension},
			{119, kw.SlotExtension},
		} {
			s, err := l.NewSlot(tt.number)
			if err != nil {
				t.Fatalf("%s: NewSlot(%d) = %v", l.Model(), tt.number, err)
			}
			if s.Number() != tt.number || s.Class() != tt.class {
				t.Errorf("%s: NewSlot(%d) = %d/%v, want %d/%v", l.Model(), tt.number, s.Number(), s.Class(), tt.number, tt.class)
			}
			if s.IsZero() {
				t.Errorf("%s: NewSlot(%d).IsZero() = true", l.Model(), tt.number)
			}
			if got, want := s.String(), map[int]string{0: "000", 99: "099", 100: "100", 109: "109", 110: "110", 119: "119"}[tt.number]; got != want {
				t.Errorf("%s: NewSlot(%d).String() = %q, want %q", l.Model(), tt.number, got, want)
			}
		}
	}
}

func TestNewSlot_RefusesWhatIsOutsideThePrintedDomain(t *testing.T) {
	for _, l := range []Layout{Layout890(), Layout990()} {
		for _, n := range []int{-1, 120, 999} {
			if _, err := l.NewSlot(n); err == nil {
				t.Errorf("%s: NewSlot(%d) = nil error, want a refusal", l.Model(), n)
			} else {
				assertParseRefusal(t, err, "slot space")
			}
		}
	}
	if _, err := (Layout{}).NewSlot(0); err == nil {
		t.Error("the zero layout resolved a slot")
	}
}

// A18 IS CITED AT THIS BUILDER AND IT IS THE MILESTONE'S LOAD-BEARING READ
// ASSUMPTION: the frame carries its own channel number and no MN precedes it.
// If A18 is false, no read works at all.
func TestBuildMA0Read_IsTheSevenBytesBOTHBooksPrint(t *testing.T) {
	for _, l := range []Layout{Layout890(), Layout990()} {
		cmd, err := l.BuildMA0Read(slotOf(t, l, 7))
		if err != nil {
			t.Fatalf("%s: BuildMA0Read: %v", l.Model(), err)
		}
		if got, want := string(cmd.Bytes()), "MA0007;"; got != want {
			t.Errorf("%s: BuildMA0Read(007) = %q, want %q", l.Model(), got, want)
		}
		if got := len(cmd.Bytes()); got != 7 {
			t.Errorf("%s: BuildMA0Read is %d bytes, want 7 (890:3184-3186, 990:2916-2918)", l.Model(), got)
		}
	}
}

func TestBuildMA0Read_RefusesTheZeroLayoutAndTheZeroSlot(t *testing.T) {
	if _, err := (Layout{}).BuildMA0Read(Slot{}); err == nil {
		t.Error("the zero layout built an MA0 read")
	}
	if _, err := Layout890().BuildMA0Read(Slot{}); err == nil {
		t.Error("the zero slot built an MA0 read")
	} else {
		assertParseRefusal(t, err, "no slot")
	}
}

// THE CORRELATION KEY IS A PREFIX ON BOTH ROWS, and the two rows differ in
// exactly one thing: the 990S pins an exact 57 and the 890S admits the
// printed range 40-50. A bare unbounded matcher would correlate a 200-byte
// run of noise that happened to start with the right six bytes.
func TestMA0AnswerMatcher_990SIsExactAndThe890SIsARange(t *testing.T) {
	pad := func(n int) []byte {
		f := make([]byte, n)
		copy(f, "MA0007")
		for i := 6; i < n-1; i++ {
			f[i] = '0'
		}
		f[n-1] = ';'
		return f
	}

	m890 := Layout890().MA0AnswerMatcher(slotOf(t, Layout890(), 7))
	for _, n := range []int{40, 44, 50} {
		if !m890(pad(n)) {
			t.Errorf("890S matcher refused a %d-byte answer, which the printed range admits", n)
		}
	}
	for _, n := range []int{7, 39, 51, 57, 200} {
		if m890(pad(n)) {
			t.Errorf("890S matcher accepted a %d-byte frame, outside the printed 40-50 range", n)
		}
	}
	if m890(pad(44)[:6]) {
		t.Error("890S matcher accepted a 6-byte frame")
	}

	m990 := Layout990().MA0AnswerMatcher(slotOf(t, Layout990(), 7))
	if !m990(pad(57)) {
		t.Error("990S matcher refused the printed 57-byte answer")
	}
	for _, n := range []int{40, 50, 56, 58} {
		if m990(pad(n)) {
			t.Errorf("990S matcher accepted a %d-byte frame; that grid is fixed at 57", n)
		}
	}

	// A DIFFERENT CHANNEL'S ANSWER IS NEVER THIS READ'S ANSWER — the whole
	// three-digit number is in the prefix on both rows.
	other := pad(44)
	copy(other[3:6], "008")
	if m890(other) {
		t.Error("890S matcher correlated channel 008's answer with a read of 007")
	}
	other57 := pad(57)
	copy(other57[3:6], "008")
	if m990(other57) {
		t.Error("990S matcher correlated channel 008's answer with a read of 007")
	}
}

// THE ROUND-TRIP GOLDENS. A hand-built Record, through BuildMA0Set, against
// a frozen frame, and back through ParseMA0Answer to the same Record.
//
// THE 890S NEEDS THREE NAME LENGTHS — 0, 3 and 10 — because its frame length
// is a function of the name (the terminator floats at 40 + len(name),
// 890:3181-3182) and a fixed-offset bug is invisible at one length. The 990S
// needs two, 3 and 10, because its ten-byte window is padded on write (A1)
// and a full name must not be truncated.
//
// THE VECTORS ARE DERIVED FROM THE BOOKS' OWN POSITION RULERS, not from any
// wire observation: nothing here claims a radio has produced these bytes.
func TestMA0RoundTrip_GoldenFrames(t *testing.T) {
	for _, tt := range []struct {
		file   string
		layout Layout
		rec    Record
	}{
		{
			file:   "ma0-890s-name0.golden",
			layout: Layout890(),
			rec: Record{
				Slot: slotOf(t, Layout890(), 0), FreqHz: 3_700_000, Mode: '1', ToneType: '0', Name: "",
			},
		},
		{
			file:   "ma0-890s-name3.golden",
			layout: Layout890(),
			rec: Record{
				Slot: slotOf(t, Layout890(), 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3",
			},
		},
		{
			file:   "ma0-890s-name10.golden",
			layout: Layout890(),
			rec: Record{
				Slot: slotOf(t, Layout890(), 99), FreqHz: 145_000_000, Mode: '4', FMNarrow: true,
				ToneType: '1', ToneIndex: 8, CTCSSIndex: 12,
				TXFreqHz: 145_600_000, TXMode: '4', TXFMNarrow: true,
				Split: true, Lockout: true, Name: "GB3IV MTHR",
			},
		},
		{
			file:   "ma0-990s-name3.golden",
			layout: Layout990(),
			rec: Record{
				Slot: slotOf(t, Layout990(), 7), FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3",
			},
		},
		{
			file:   "ma0-990s-name10.golden",
			layout: Layout990(),
			rec: Record{
				Slot: slotOf(t, Layout990(), 99), FreqHz: 145_000_000, Mode: '4', FMNarrow: true,
				ToneType: '1', ToneIndex: 8, CTCSSIndex: 12,
				TXFreqHz: 145_600_000, TXMode: 'I', TXFMNarrow: true,
				TXToneType: '2', TXToneIndex: 9, TXCTCSSIndex: 13,
				Split: true, Lockout: true, Name: "GB3IV MTHR",
			},
		},
	} {
		t.Run(tt.file, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatalf("reading the frozen vector: %v", err)
			}
			want = trimGoldenNewline(want)

			cmd, err := tt.layout.BuildMA0Set(tt.rec)
			if err != nil {
				t.Fatalf("BuildMA0Set: %v", err)
			}
			if got := cmd.Bytes(); string(got) != string(want) {
				t.Fatalf("BuildMA0Set =\n %q\nwant\n %q", got, want)
			}

			back, err := tt.layout.ParseMA0Answer(want)
			if err != nil {
				t.Fatalf("ParseMA0Answer of the frozen vector: %v", err)
			}
			// Class is a PARSE OUTPUT and the 990S's Set emits '0' for it
			// (A14), so the round trip restores it rather than preserving
			// what the caller did not supply.
			wantRec := tt.rec
			if tt.layout.Book() == kw.Book990 {
				wantRec.Class = '0'
			}
			if back != wantRec {
				t.Errorf("round trip =\n %+v\nwant\n %+v", back, wantRec)
			}
		})
	}
}

// trimGoldenNewline drops the single trailing newline a text editor adds, so
// the frozen files stay ordinary text files.
func trimGoldenNewline(b []byte) []byte {
	if n := len(b); n > 0 && b[n-1] == '\n' {
		return b[:n-1]
	}
	return b
}

// THE PER-RADIO DIFFERENCE PINS, EACH STATED IN BOTH DIRECTIONS, so that a
// value copy-pasted from one codec to the other fails here rather than
// shipping. Each is a fact about TWO radios.
func TestMA0_PerRadioDifferencesArePinnedInBothDirections(t *testing.T) {
	l890, l990 := Layout890(), Layout990()
	s890, s990 := slotOf(t, l890, 7), slotOf(t, l990, 7)
	base890 := Record{Slot: s890, FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "AB"}
	base990 := Record{Slot: s990, FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "AB"}

	// FRAME LENGTH: 40 + len(name) against a fixed 57.
	f890 := mustBuild(t, l890, base890)
	f990 := mustBuild(t, l990, base990)
	if len(f890) != 42 {
		t.Errorf("890S frame with a 2-character name is %d bytes, want 42 = 40 + len(name) (890:3181-3182)", len(f890))
	}
	if len(f990) != 57 {
		t.Errorf("990S frame is %d bytes, want the fixed 57 (990:2915, 990:2938)", len(f990))
	}
	if len(f890) == len(f990) {
		t.Error("the two grids produced the same length; they are 40-50 and 57")
	}

	// PARAMETER COUNT: thirteen against eighteen, measured where the two
	// grids disagree first — the 890S's frequency starts at position 7 and
	// the 990S's at position 8, because the 990S spends position 7 on its
	// channel-type P2.
	if got := f990[6]; got != '0' {
		t.Errorf("990S P2 is %q, want '0' — the milestone's ONE defaulted byte (A14, 990:2901-2903)", got)
	}
	if got := string(f890[6:17]); got != "00014250000" {
		t.Errorf("890S P2 (positions 7-17) = %q, want the frequency; the 990S has no frequency there", got)
	}
	if got := string(f990[7:18]); got != "00014250000" {
		t.Errorf("990S P3 (positions 8-18) = %q, want the frequency; the 890S has no frequency there", got)
	}
	// The five extra parameters push the name seven positions later: the
	// 890S's P13 opens at 40 and the 990S's P18 at 47.
	if got := string(f890[39:41]); got != "AB" {
		t.Errorf("890S name window opens at %q, want it at position 40 (890:3208-3209)", got)
	}
	if got := string(f990[46:48]); got != "AB" {
		t.Errorf("990S name window opens at %q, want it at position 47 (990:2955-2956)", got)
	}

	// THE MODE LEGEND'S CEILING: 'F' on the 890S, 'N' on the 990S.
	if _, ok := l890.ModeName('G'); ok {
		t.Error("the 890S legend names 'G'; its chart stops at 'F' (890:3976-3992)")
	}
	if _, ok := l990.ModeName('N'); !ok {
		t.Error("the 990S legend does not name 'N'; its chart runs to 'N' (990:3706-3730)")
	}
	if _, err := l890.BuildMA0Set(withMode(base890, 'I')); err == nil {
		t.Error("the 890S built mode 'I', which only the 990S's chart prints")
	}
	if _, err := l990.BuildMA0Set(withMode(base990, 'I')); err != nil {
		t.Errorf("the 990S refused mode 'I' (FM-D2), which its chart prints: %v", err)
	}

	// THE LOCKOUT ENCODING: 0/1 on the 890S, 1/2 on the 990S — E8.
	on890 := base890
	on890.Lockout = true
	on990 := base990
	on990.Lockout = true
	if got := mustBuild(t, l890, on890)[38]; got != '1' {
		t.Errorf("890S P12 with lockout ON is %q, want '1' (890:3205-3207)", got)
	}
	if got := mustBuild(t, l890, base890)[38]; got != '0' {
		t.Errorf("890S P12 with lockout OFF is %q, want '0' (890:3205-3207)", got)
	}
	if got := mustBuild(t, l990, on990)[45]; got != '2' {
		t.Errorf("990S P17 with lockout ON is %q, want '2' — E8, and a codec importing MA3's 0/1 convention here would emit '1' (990:2952-2954)", got)
	}
	if got := mustBuild(t, l990, base990)[45]; got != '1' {
		t.Errorf("990S P17 with lockout OFF is %q, want '1' — E8 again: OFF is not zero on this radio (990:2952-2954)", got)
	}

	// THE NUMBER OF TONE TUPLES: one against two. The 890S grid has no
	// position for a second, so a record carrying one is refused rather
	// than silently shortened.
	two890 := base890
	two890.TXMode, two890.TXToneType, two890.TXToneIndex = '2', '1', 8
	if _, err := l890.BuildMA0Set(two890); err == nil {
		t.Error("the 890S built a record carrying a second tone tuple; its grid has one (890:3186-3190)")
	} else {
		assertParseRefusal(t, err, "second tone tuple")
	}
	two990 := base990
	two990.TXMode, two990.TXToneType, two990.TXToneIndex = '2', '1', 8
	if _, err := l990.BuildMA0Set(two990); err != nil {
		t.Errorf("the 990S refused a second tone tuple, which its grid prints at P12-P14 (990:2935-2945): %v", err)
	}
}

// withMode returns rec with its mode byte replaced.
func withMode(rec Record, mode byte) Record {
	rec.Mode = mode
	return rec
}

// mustBuild builds rec on l or fails.
func mustBuild(t *testing.T, l Layout, rec Record) []byte {
	t.Helper()
	cmd, err := l.BuildMA0Set(rec)
	if err != nil {
		t.Fatalf("%s: BuildMA0Set: %v", l.Model(), err)
	}
	return cmd.Bytes()
}

// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// --- BuildMTSet: golden vectors G8, G9 ---

func TestBuildMTSet_G8(t *testing.T) {
	s, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := BuildMTSet(s, true, "CALLING FREQ")
	if err != nil {
		t.Fatalf("BuildMTSet: unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MT0011CALLING FREQ;"
	if string(got) != want {
		t.Errorf("BuildMTSet() = %q, want %q", got, want)
	}
	if len(got) != 19 {
		t.Errorf("len(BuildMTSet()) = %d, want 19 (2+3+1+12+1)", len(got))
	}
}

func TestBuildMTSet_G9(t *testing.T) {
	s, err := MemorySlot(5)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := BuildMTSet(s, false, "40M")
	if err != nil {
		t.Fatalf("BuildMTSet: unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MT005040M;"
	if string(got) != want {
		t.Errorf("BuildMTSet() = %q, want %q", got, want)
	}
}

// TestBuildMTSet_EmptyTagEmitsClearForm: an EMPTY tag is the one
// special case where BuildMTSet does NOT emit the variable-length
// unpadded form — HW-PROBED 13/07/2026 (docs/fixtures-private/, stages
// tagclear/tagclear2): the 0-byte form ("MT0011;") this test used to
// require is REJECTED by the radio ("?;", tag survives); the all-spaces
// 12-byte tag is the hardware-proven clear mechanism, so that is what
// an empty tag encodes to. See
// TestBuildMTSet_HWDerived_ZeroByteFormNeverEmitted
// (hw_derived_m5b_test.go) for the paired HW-derived vector.
func TestBuildMTSet_EmptyTagEmitsClearForm(t *testing.T) {
	s, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := BuildMTSet(s, true, "")
	if err != nil {
		t.Fatalf("BuildMTSet with empty tag: unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MT0011            ;" // 12-space clear form, never the rejected 0-byte form
	if string(got) != want {
		t.Errorf("BuildMTSet(empty tag) = %q, want %q (the all-spaces clear form)", got, want)
	}
	if len(mtClearTag) != mtTagMaxBytes {
		t.Fatalf("mtClearTag is %d bytes, want %d (must fill the tag field exactly)", len(mtClearTag), mtTagMaxBytes)
	}
}

// TestBuildMTSet_NonEmptyTagsStayUnpadded pins the boundary of the
// empty-tag special case: a non-empty tag — even a single byte — is
// still emitted variable-length, unpadded, exactly as the M5b write
// trials proved the radio accepts (hw_derived_m5b_test.go, and the
// 13/07/2026 production write, whose unpadded "PROD TEST" MT-set was
// accepted on the wire).
func TestBuildMTSet_NonEmptyTagsStayUnpadded(t *testing.T) {
	s, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := BuildMTSet(s, false, "X")
	if err != nil {
		t.Fatalf("BuildMTSet: unexpected error: %v", err)
	}
	if got, want := string(cmd.Bytes()), "MT0010X;"; got != want {
		t.Errorf("BuildMTSet(1-byte tag) = %q, want %q (non-empty tags are never padded)", got, want)
	}
}

func TestBuildMTSet_AllowsPMSSlot(t *testing.T) {
	s, err := PMSSlot(1, true)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := BuildMTSet(s, false, "TOP")
	if err != nil {
		t.Fatalf("BuildMTSet(PMS): unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MTP1U0TOP;"
	if string(got) != want {
		t.Errorf("BuildMTSet(PMS) = %q, want %q", got, want)
	}
}

// TestBuildMTSet_RejectTable covers the reference's "Rejection cases"
// paragraph and the Task 3 brief's explicit MT test list: injection via
// embedded ';', control bytes (\r \n \x00 \x1B), byte 0x80, a multi-byte
// rune ('é'), a 13-byte ASCII tag, and a tag that is 12 runes but 13
// bytes. Also: policy-reject of 5xx/EMG slots, and "000".
func TestBuildMTSet_RejectTable(t *testing.T) {
	memSlot, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	sixtyM, err := SixtyMSlot(1)
	if err != nil {
		t.Fatal(err)
	}
	noneSlot, err := ParseSlot("000")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		slot Slot
		tag  string
	}{
		{"embedded ';' mid-tag: injection, brief example X;TX1;", memSlot, "X;TX1;"},
		{"tag with \\r", memSlot, "AB\rCD"},
		{"tag with \\n", memSlot, "AB\nCD"},
		{"tag with 0x00", memSlot, "AB\x00CD"},
		{"tag with 0x1B (ESC)", memSlot, "AB\x1BCD"},
		{"tag with byte 0x80", memSlot, "AB\x80CD"},
		{"tag with multi-byte rune 'é'", memSlot, "café freq"},
		{"tag 13 ASCII chars: exceeds 12-byte max", memSlot, "ABCDEFGHIJKLM"},
		{"tag 12 runes but 13 bytes (multi-byte rune)", memSlot, "ABCDEFGHIJKé"}, // 11 ASCII + é (2 bytes) = 12 runes, 13 bytes
		{"slot 5xx rejected: project policy pending M5a", sixtyM, "40M"},
		{"slot EMG rejected: project policy pending M5a", EMGSlot(), "EMG"},
		{"slot 000 rejected: reference MT column ✗", noneSlot, "VFO"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := BuildMTSet(tc.slot, true, tc.tag)
			if err == nil {
				t.Fatalf("BuildMTSet(%q, true, %q): want error, got %q", tc.slot.Wire(), tc.tag, out.Bytes())
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("BuildMTSet(%q, true, %q): error is %T, want *ParseError", tc.slot.Wire(), tc.tag, err)
			}
		})
	}
}

// TestBuildMTSet_RejectTable_ByteLengthVsRuneLength directly checks the
// 12-rune-but-13-byte case rejects on BYTE length, not rune count.
func TestBuildMTSet_ByteLengthNotRuneCount(t *testing.T) {
	tag := "ABCDEFGHIJKé" // 11 ASCII runes + 1 two-byte rune = 12 runes
	if n := len([]rune(tag)); n != 12 {
		t.Fatalf("test tag has %d runes, want 12 (fix the fixture)", n)
	}
	if n := len(tag); n != 13 {
		t.Fatalf("test tag has %d bytes, want 13 (fix the fixture)", n)
	}
	s, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildMTSet(s, true, tag); err == nil {
		t.Fatal("BuildMTSet: 12-rune/13-byte tag must be rejected on byte length")
	}
}

func TestBuildMTSet_AcceptsExactly12Bytes(t *testing.T) {
	s, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	tag := "ABCDEFGHIJKL" // exactly 12 bytes
	if _, err := BuildMTSet(s, true, tag); err != nil {
		t.Errorf("BuildMTSet(12-byte tag): unexpected error: %v", err)
	}
}

func TestBuildMTSet_AcceptsBoundaryPrintableBytes(t *testing.T) {
	s, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	// 0x20 (space) and 0x7E ('~') are the inclusive charset boundaries.
	tag := "\x20\x7E"
	if _, err := BuildMTSet(s, true, tag); err != nil {
		t.Errorf("BuildMTSet(boundary bytes 0x20,0x7E): unexpected error: %v", err)
	}
}

func TestBuildMTSet_RejectsJustOutsideBoundary(t *testing.T) {
	s, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []byte{0x1F, 0x7F} {
		if _, err := BuildMTSet(s, true, string(b)); err == nil {
			t.Errorf("BuildMTSet(tag=%#02x): want error (just outside 0x20-0x7E)", b)
		}
	}
}

// --- BuildMTRead: golden vector G10 ---

func TestBuildMTRead_G10(t *testing.T) {
	s, err := MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := BuildMTRead(s)
	if err != nil {
		t.Fatalf("BuildMTRead: unexpected error: %v", err)
	}
	got := cmd.Bytes()
	want := "MT001;"
	if string(got) != want {
		t.Errorf("BuildMTRead() = %q, want %q", got, want)
	}
}

func TestBuildMTRead_Allows5xxAndEMG(t *testing.T) {
	sixtyM, err := SixtyMSlot(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildMTRead(sixtyM); err != nil {
		t.Errorf("BuildMTRead(5xx): unexpected error: %v (reads have no write-direction hardware concern)", err)
	}
	if _, err := BuildMTRead(EMGSlot()); err != nil {
		t.Errorf("BuildMTRead(EMG): unexpected error: %v", err)
	}
}

func TestBuildMTRead_RejectsNone(t *testing.T) {
	none, err := ParseSlot("000")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildMTRead(none); err == nil {
		t.Fatal("BuildMTRead(\"000\"): want error, reference MT column: ✗")
	}
}

func TestBuildMTSet_RejectsZeroValueSlot(t *testing.T) {
	if _, err := BuildMTSet(Slot{}, true, "40M"); err == nil {
		t.Fatal("BuildMTSet(Slot{}): want error for uninitialised/invalid slot")
	}
}

func TestBuildMTRead_RejectsZeroValueSlot(t *testing.T) {
	if _, err := BuildMTRead(Slot{}); err == nil {
		t.Fatal("BuildMTRead(Slot{}): want error for uninitialised/invalid slot")
	}
}

// --- ParseMTAnswer: golden vectors G8, G9; round-trips with BuildMTSet ---

func TestParseMTAnswer_G8(t *testing.T) {
	slot, display, tag, err := ParseMTAnswer([]byte("MT0011CALLING FREQ;"))
	if err != nil {
		t.Fatalf("ParseMTAnswer: unexpected error: %v", err)
	}
	wantSlot, _ := MemorySlot(1)
	if slot != wantSlot || !display || tag != "CALLING FREQ" {
		t.Errorf("ParseMTAnswer(G8) = (%q,%v,%q), want (%q,true,%q)", slot.Wire(), display, tag, wantSlot.Wire(), "CALLING FREQ")
	}
}

func TestParseMTAnswer_G9(t *testing.T) {
	slot, display, tag, err := ParseMTAnswer([]byte("MT005040M;"))
	if err != nil {
		t.Fatalf("ParseMTAnswer: unexpected error: %v", err)
	}
	wantSlot, _ := MemorySlot(5)
	if slot != wantSlot || display || tag != "40M" {
		t.Errorf("ParseMTAnswer(G9) = (%q,%v,%q), want (%q,false,%q)", slot.Wire(), display, tag, wantSlot.Wire(), "40M")
	}
}

func TestParseMTAnswer_EmptyTag(t *testing.T) {
	slot, display, tag, err := ParseMTAnswer([]byte("MT0011;"))
	if err != nil {
		t.Fatalf("ParseMTAnswer: unexpected error: %v", err)
	}
	wantSlot, _ := MemorySlot(1)
	if slot != wantSlot || !display || tag != "" {
		t.Errorf("ParseMTAnswer(empty tag) = (%q,%v,%q), want (%q,true,\"\")", slot.Wire(), display, tag, wantSlot.Wire())
	}
}

// TestParseMTAnswer_TrimsTrailingSpaces is the Fix (tag normalisation)
// reversal of this package's former "preserve verbatim" design: the
// radio pads short tags with trailing spaces on read (HW-CONFIRMED,
// docs/hardware-notes.md §MT short-form answer), and — the hard way, via
// the first real-radio production write, 13/07/2026 — a tag written
// UNPADDED via CAT MT-set also comes back read padded. Storing that
// padding in the model made an unpadded hand-edited tag fail clone's
// post-write verify with a false mismatch (see
// core/clone's TestExecute_LiveBugRepro_UnpaddedTagWriteReadBackPadded).
// ParseMTAnswer now trims trailing spaces so the model's canonical tag
// form is never padded; this generic 2-char case is additive to
// TestParseMTAnswer_HWDerived_M06_ShortForm's real wire frame
// (hw_derived_test.go).
func TestParseMTAnswer_TrimsTrailingSpaces(t *testing.T) {
	frame := "MT0011AB          ;" // "AB" + 10 trailing spaces = 12 bytes
	_, _, tag, err := ParseMTAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMTAnswer: unexpected error: %v", err)
	}
	want := "AB"
	if tag != want {
		t.Errorf("ParseMTAnswer tag = %q, want %q (trailing spaces trimmed)", tag, want)
	}
}

// TestParseMTAnswer_AllSpacesTagTrimsToEmpty pins the radio's tag-CLEAR
// form (an all-spaces 12-byte tag, HW-CONFIRMED hw_derived_m5b_test.go)
// trimming to "" — matching the model's "no tag" representation exactly,
// not a 12-space string.
func TestParseMTAnswer_AllSpacesTagTrimsToEmpty(t *testing.T) {
	frame := "MT0010            ;" // 12 spaces
	_, _, tag, err := ParseMTAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMTAnswer: unexpected error: %v", err)
	}
	if tag != "" {
		t.Errorf("ParseMTAnswer tag = %q, want \"\" (all-spaces tag trims to no-tag)", tag)
	}
}

// TestParseMTAnswer_PaddedAndUnpaddedTagsCompareEqualPostNormalisation
// directly proves the fix's core invariant: a candidate written with an
// unpadded tag and a read-back the radio pads both normalise to the SAME
// model-level value, so a byte-equality compare (as clone's write-verify
// does) never manufactures a false mismatch out of padding alone.
func TestParseMTAnswer_PaddedAndUnpaddedTagsCompareEqualPostNormalisation(t *testing.T) {
	_, _, padded, err := ParseMTAnswer([]byte("MT0011PROD TEST   ;")) // radio's padded read-back
	if err != nil {
		t.Fatalf("ParseMTAnswer(padded): unexpected error: %v", err)
	}
	_, _, unpadded, err := ParseMTAnswer([]byte("MT0011PROD TEST;")) // caller's unpadded candidate, itself parsed
	if err != nil {
		t.Fatalf("ParseMTAnswer(unpadded): unexpected error: %v", err)
	}
	if padded != unpadded {
		t.Errorf("padded tag %q != unpadded tag %q, want equal post-normalisation", padded, unpadded)
	}
	if padded != "PROD TEST" {
		t.Errorf("normalised tag = %q, want %q", padded, "PROD TEST")
	}
}

func TestParseMTAnswer_RejectTable(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{"too short (missing display digit)", "MT001;"},
		{"too long (13-byte tag)", "MT0011ABCDEFGHIJKLM;"},
		{"empty", ""},
		{"wrong prefix", "XX0011CALLING FREQ;"},
		{"lower case prefix", "mt0011CALLING FREQ;"},
		{"missing terminator", "MT0011CALLING FREQX"},
		{"bad slot", "MTXYZ1CALLING FREQ;"},
		{"slot 000 rejected: reference MT column ✗", "MT0001CALLING FREQ;"},
		{"display digit garbage", "MT0012CALLING FREQ;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := ParseMTAnswer([]byte(tc.frame))
			if err == nil {
				t.Fatalf("ParseMTAnswer(%q): want error, got none", tc.frame)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseMTAnswer(%q): error is %T, want *ParseError", tc.frame, err)
			}
		})
	}
}

// TestParseMTAnswer_LenientOnTagCharset documents and verifies a
// deliberate design choice: unlike BuildMTSet, ParseMTAnswer does not
// re-validate the tag body's charset. The reference documents the tag
// charset only as "ASCII code" and flags it "TBD at M5a"; both the
// reference's "Rejection cases" list and the Task 3 brief's test list
// scope charset enforcement to the BUILDER only (mirrors ParseIDAnswer's
// precedent in id.go for the same reason: the reference does not
// constrain this field's charset on the read/parse path).
func TestParseMTAnswer_LenientOnTagCharset(t *testing.T) {
	// A control byte in the tag body must not cause a parse failure: the
	// frame's SHAPE (length, prefix, slot, display digit, terminator) is
	// still exactly valid.
	frame := "MT0011AB\x00CD;" // 4-byte tag body, mid-tag NUL
	_, _, tag, err := ParseMTAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMTAnswer(%q): unexpected error: %v (charset enforcement is BUILD-only)", frame, err)
	}
	want := "AB\x00CD"
	if tag != want {
		t.Errorf("ParseMTAnswer tag = %q, want %q", tag, want)
	}
}

// TestBuildMTSet_ParseMTAnswer_RoundTrip: MT build -> parse round-trips
// tags exactly.
func TestBuildMTSet_ParseMTAnswer_RoundTrip(t *testing.T) {
	tests := []struct {
		slot    Slot
		display bool
		tag     string
	}{
		{mustMemorySlot(t, 1), true, "CALLING FREQ"},
		{mustMemorySlot(t, 5), false, "40M"},
		{mustMemorySlot(t, 1), true, ""},
		{mustPMSSlot(t, 1, true), false, "TOP"},
	}
	for _, tc := range tests {
		cmd, err := BuildMTSet(tc.slot, tc.display, tc.tag)
		if err != nil {
			t.Fatalf("BuildMTSet(%q,%v,%q): unexpected error: %v", tc.slot.Wire(), tc.display, tc.tag, err)
		}
		built := cmd.Bytes()
		gotSlot, gotDisplay, gotTag, err := ParseMTAnswer(built)
		if err != nil {
			t.Fatalf("ParseMTAnswer(%q): unexpected error: %v", built, err)
		}
		if gotSlot != tc.slot || gotDisplay != tc.display || gotTag != tc.tag {
			t.Errorf("round trip %q: got (%q,%v,%q), want (%q,%v,%q)",
				built, gotSlot.Wire(), gotDisplay, gotTag, tc.slot.Wire(), tc.display, tc.tag)
		}
	}
}

func mustMemorySlot(t *testing.T, n int) Slot {
	t.Helper()
	s, err := MemorySlot(n)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustPMSSlot(t *testing.T, pair int, upper bool) Slot {
	t.Helper()
	s, err := PMSSlot(pair, upper)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// FuzzParseMTAnswer requires ParseMTAnswer never panics and only ever
// returns a typed *ParseError on failure.
func FuzzParseMTAnswer(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(""),
		[]byte("MT0011CALLING FREQ;"), // G8
		[]byte("MT005040M;"),          // G9
		[]byte("MT0011;"),
		[]byte("MT001;"),
		[]byte("MT0001CALLING FREQ;"),
		[]byte("MTXYZ1CALLING FREQ;"),
		[]byte("MT0011ABCDEFGHIJKLM;"),
		[]byte("MT0011AB\x00CD;"),
		[]byte("MT0011X;TX1;"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		slot, _, tag, err := ParseMTAnswer(frame)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseMTAnswer(%q) returned non-ParseError: %T (%v)", frame, err, err)
			}
			return
		}
		if len(slot.Wire()) != 3 {
			t.Fatalf("ParseMTAnswer(%q) succeeded with malformed Slot %q", frame, slot.Wire())
		}
		if len(tag) > mtTagMaxBytes {
			t.Fatalf("ParseMTAnswer(%q) succeeded with tag %q longer than %d bytes", frame, tag, mtTagMaxBytes)
		}
	})
}

// FuzzBuildMTSet asserts the injection-defence invariant: if BuildMTSet
// succeeds, the output contains EXACTLY one ';' and it is the last byte,
// and AllowedCommand(output) is true.
func FuzzBuildMTSet(f *testing.F) {
	type seed struct {
		slot    string
		display bool
		tag     string
	}
	seeds := []seed{
		{"001", true, "CALLING FREQ"},
		{"005", false, "40M"},
		{"099", true, ""},
		{"P1L", false, "X;TX1;"},
		{"501", true, "40M"},
		{"EMG", true, "40M"},
		{"000", true, "40M"},
		{"100", true, "AAAAAAAAAAAAA"},
		{"001", true, "AB\rCD"},
		{"001", true, "café"},
	}
	for _, s := range seeds {
		f.Add(s.slot, s.display, s.tag)
	}
	f.Fuzz(func(t *testing.T, slotWire string, display bool, tag string) {
		slot, err := ParseSlot(slotWire)
		if err != nil {
			return // not a well-formed slot at all; nothing to exercise
		}
		cmd, err := BuildMTSet(slot, display, tag)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("BuildMTSet(%q,%v,%q) returned non-ParseError: %T (%v)", slotWire, display, tag, err, err)
			}
			if !cmd.IsZero() {
				t.Fatalf("BuildMTSet(%q,%v,%q) returned error but non-zero Command %q", slotWire, display, tag, cmd.Bytes())
			}
			return
		}
		out := cmd.Bytes()
		count := 0
		for _, b := range out {
			if b == ';' {
				count++
			}
		}
		if count != 1 || out[len(out)-1] != ';' {
			t.Fatalf("BuildMTSet(%q,%v,%q) = %q: want exactly one ';' at the end", slotWire, display, tag, out)
		}
		if !AllowedCommand(out) {
			t.Fatalf("BuildMTSet(%q,%v,%q) = %q: AllowedCommand = false, want true", slotWire, display, tag, out)
		}
	})
}

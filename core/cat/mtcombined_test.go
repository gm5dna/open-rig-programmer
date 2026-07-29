// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"strings"
	"testing"
)

// mustCombinedDialect builds a COMBINED-form fixture with the tag width,
// fill byte and MW write kind the caller names, over the FT-710's own slot
// space and mode table and a minimal EX inventory.
//
// The three parameters are exactly the dimensions this file's cases need to
// disagree on: tagMax fixes the derived frame length (29 + tagMax), fill is
// both the outbound pad byte and the answer trim byte, and mwKind is the
// carrier of the P7 DECOUPLING proof — the combined Set's P7 is a form
// schema constant, so a fixture declaring some other MW write kind must
// still emit '0'.
//
// The FT-710's slot and mode tables rather than the validate-test baseline's
// so that the frames here are readable against the manual's own worked
// examples (memory slot 007, mode 'USB'), and because the mode table must
// contain ModeUnset for the ModeUnset refusal to be a refusal rather than an
// unknown-mode complaint. The CAT ID is deliberately NOT the FT-710's: this
// is a fiction with a real radio's geometry, not that radio.
func mustCombinedDialect(t *testing.T, tagMax int, fill byte, mwKind byte) Dialect {
	t.Helper()
	return MustNewDialect(DialectConfig{
		CATID:     "1234",
		ModeNames: modeNames,
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 501, SixtyHi: 599,
			PMSPairs:      9,
			EmergencyWire: "EMG",
			NoneWire:      "000",
		},
		EXItems: []EXItem{
			{Addr: EXAddress{P1: 7, P2: 1, P3: 1}, Name: "ITEM ONE", Digits: 2},
		},
		MT:          MTPolicy{Form: MTFormCombined, TagMaxBytes: tagMax, TagFill: fill},
		Clarifier:   ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MWWriteKind: mwKind,
	})
}

// validCombinedMemory is the canonical VALID record every case here builds
// from: memory slot 007, 14.250 MHz, a -10 Hz clarifier with RX clar on and
// TX clar off, USB, the combined Set's own P7 constant, CTCSS ENC/DEC and
// plus shift.
//
// Every one of those bytes appears in wantCQDXFrame below. The duplication
// is deliberate: that literal pins the LAYOUT, and a test that derived it
// from this record would restate the encoder rather than check it.
func validCombinedMemory(t *testing.T, d Dialect) MemoryData {
	t.Helper()
	slot, err := d.MemorySlot(7)
	if err != nil {
		t.Fatalf("MemorySlot(7): %v", err)
	}
	return MemoryData{
		Slot:   slot,
		FreqHz: 14_250_000,
		ClarHz: -10,
		RxClar: true,
		TxClar: false,
		Mode:   ModeUSB,
		Kind:   combinedMTSetKind,
		CTCSS:  CTCSSEncDec,
		Shift:  ShiftPlus,
	}
}

// mustCombinedFrame builds one combined Set frame or fails the test, and
// returns its bytes — the starting point for every case that mutates a
// single position of an otherwise valid frame.
func mustCombinedFrame(t *testing.T, d Dialect, tag string) []byte {
	t.Helper()
	cmd, err := d.BuildMTSetCombined(validCombinedMemory(t, d), tag)
	if err != nil {
		t.Fatalf("BuildMTSetCombined(%q) = %v, want a frame", tag, err)
	}
	return cmd.Bytes()
}

// wantCQDXFrame is the exact 35-byte combined record for
// validCombinedMemory plus the tag "CQ DX", on a 6-byte-tag dialect filling
// with ' ':
//
//	MT | 007 | 014250000 | - | 0010 | 1 | 0 | 2 | 0 | 1 | 00 | 1 | 0 | "CQ DX " | ;
//	     P1    P2          P3 sign/mag P4  P5  P6  P7  P8   P9  P10 P11  P12       term
//
// P7 is '0', the form's Set constant ("(Fixed)"), NOT the dialect's MW write
// kind. P12 is the tag padded to the full field width with TagFill, which is
// why a five-byte tag shows one trailing space.
const wantCQDXFrame = "MT007014250000-0010102010010CQ DX ;"

// TestBuildMTSetCombined_RoundTripsExactly is the milestone's central case:
// one record and one tag out, the same record and the same tag back.
//
// It asserts the frame at three levels deliberately — its derived length,
// its exact bytes, and the tag field in isolation — because a builder that
// got the length right by putting the tag in the wrong place would satisfy
// only the first, and one that padded with the wrong byte only the first
// two.
func TestBuildMTSetCombined_RoundTripsExactly(t *testing.T) {
	d := mustCombinedDialect(t, 6, ' ', KindMemory)
	m := validCombinedMemory(t, d)

	cmd, err := d.BuildMTSetCombined(m, "CQ DX")
	if err != nil {
		t.Fatalf("BuildMTSetCombined() = %v, want a frame", err)
	}
	got := cmd.Bytes()

	if len(got) != 35 {
		t.Errorf("frame is %d bytes, want 35 — 29 + TagMaxBytes(6)", len(got))
	}
	if string(got) != wantCQDXFrame {
		t.Errorf("frame = %q, want %q", got, wantCQDXFrame)
	}
	if tagField := string(got[28 : 28+6]); tagField != "CQ DX " {
		t.Errorf("tag field = %q, want %q — the field is padded to its full width with TagFill", tagField, "CQ DX ")
	}

	gotM, gotTag, err := d.ParseMTAnswerCombined(got)
	if err != nil {
		t.Fatalf("ParseMTAnswerCombined(%q) = %v, want the record back", got, err)
	}
	if gotM != m {
		t.Errorf("ParseMTAnswerCombined() record = %+v, want %+v", gotM, m)
	}
	if gotTag != "CQ DX" {
		t.Errorf("ParseMTAnswerCombined() tag = %q, want %q — the fill padding is a wire-encoding concern and must not survive the parse", gotTag, "CQ DX")
	}
}

// TestBuildMTSetCombined_EmptyTagIsTheAllFillField pins the combined form's
// ONE tag special case: there is no distinct clear encoding, so an empty tag
// is simply the all-fill field, and that field decodes back to "".
//
// The short form's clear byte is a separate declaration precisely because
// its clear form and its answer padding are different events; here one byte
// does both jobs, which is why MTPolicy carries TagFill alone for this form.
func TestBuildMTSetCombined_EmptyTagIsTheAllFillField(t *testing.T) {
	d := mustCombinedDialect(t, 6, ' ', KindMemory)

	got := mustCombinedFrame(t, d, "")
	if tagField := string(got[28 : 28+6]); tagField != "      " {
		t.Errorf("tag field = %q, want six fill bytes — an empty tag IS the all-fill field", tagField)
	}

	_, gotTag, err := d.ParseMTAnswerCombined(got)
	if err != nil {
		t.Fatalf("ParseMTAnswerCombined(%q) = %v, want accepted", got, err)
	}
	if gotTag != "" {
		t.Errorf("ParseMTAnswerCombined() tag = %q, want \"\" — an all-fill field is no tag", gotTag)
	}

	// The same property on a dialect filling with something other than a
	// space: a decoder trimming spaces by assumption passes the case above
	// and fails this one.
	other := mustCombinedDialect(t, 6, '_', KindMemory)
	otherFrame := mustCombinedFrame(t, other, "")
	if tagField := string(otherFrame[28 : 28+6]); tagField != "______" {
		t.Errorf("underscore-filling dialect tag field = %q, want six underscores", tagField)
	}
	if _, tag, err := other.ParseMTAnswerCombined(otherFrame); err != nil {
		t.Fatalf("other.ParseMTAnswerCombined(%q) = %v, want accepted", otherFrame, err)
	} else if tag != "" {
		t.Errorf("other.ParseMTAnswerCombined() tag = %q, want \"\"", tag)
	}
}

// TestBuildMTSetCombined_RefusesTagsThatWouldNotRoundTrip covers the
// builder's LOGICAL-tag rules: charset, width, and the trailing-fill
// refusal.
//
// The trailing-fill rule is what makes the round trip EXACT for every
// accepted tag: the field is padded to full width with TagFill and trimmed
// of it on the way back, so a logical tag ending in that byte would return
// shorter than it went out. Validate, don't canonicalise — the caller is
// told to trim it rather than having it silently trimmed. A tag of ONLY fill
// bytes is refused by the same rule, "" being its canonical spelling.
//
// This is a BUILDER-INPUT rule and it does not transfer to the wire: every
// padded field ends in TagFill by construction, which is why the gate (task
// 5) validates the raw field per-byte without it.
func TestBuildMTSetCombined_RefusesTagsThatWouldNotRoundTrip(t *testing.T) {
	d := mustCombinedDialect(t, 6, ' ', KindMemory)
	m := validCombinedMemory(t, d)

	tests := []struct {
		name string
		tag  string
	}{
		{"trailing fill byte", "AB "},
		{"only fill bytes", "   "},
		{"a full field of fill bytes", "      "},
		{"one byte over the width", "ABCDEFG"},
		{"a terminator smuggled into the tag", "A;B"},
		{"a control byte", "A\x01B"},
		{"a multi-byte rune", "CQ é"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := d.BuildMTSetCombined(m, tc.tag)
			if err == nil {
				t.Fatalf("BuildMTSetCombined(%q) succeeded, want a refusal", tc.tag)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("BuildMTSetCombined(%q): error is %T, want *ParseError", tc.tag, err)
			}
			if !cmd.IsZero() {
				t.Errorf("BuildMTSetCombined(%q) returned a non-zero Command alongside its error; every fallible builder returns Command{} (command.go)", tc.tag)
			}
		})
	}

	// The positive control: a tag whose LAST byte is not the fill byte but
	// which contains one is fine, and round-trips whole.
	got := mustCombinedFrame(t, d, "CQ DX")
	if _, tag, err := d.ParseMTAnswerCombined(got); err != nil {
		t.Fatalf("ParseMTAnswerCombined(%q) = %v", got, err)
	} else if tag != "CQ DX" {
		t.Errorf("tag = %q, want %q — an interior fill byte is data", tag, "CQ DX")
	}
}

// TestBuildMTSetCombined_ValidationChecklist is validateCombinedMTFields'
// complete list, one case per rule: slot, kind, mode (both halves),
// clarifier, frequency (both halves), CTCSS and shift.
//
// It is validateMWFields' list with exactly two substitutions — the slot
// predicate is MT's own (mtSlotValid) and the kind is the FORM's schema
// constant rather than the dialect's MW write kind — and nothing dropped.
// Two of those rules (ModeUnset, and FreqHz == 0) were omitted from an
// earlier draft of this milestone's plan and restored by review, which is
// why each is its own entry here rather than a clause of another.
//
// Every entry asserts the zero-Command contract as well as the refusal: a
// builder returning a half-built frame beside its error would satisfy a
// test that only checked the error.
func TestBuildMTSetCombined_ValidationChecklist(t *testing.T) {
	d := mustCombinedDialect(t, 6, ' ', KindMemory)

	sixtyM, err := d.ParseSlot("501")
	if err != nil {
		t.Fatalf("ParseSlot(\"501\"): %v", err)
	}
	noneSlot, err := d.ParseSlot("000")
	if err != nil {
		t.Fatalf("ParseSlot(\"000\"): %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*MemoryData)
		want   string // a substring the refusal must name
	}{
		{"a 60m slot, refused by MT's own write policy", func(m *MemoryData) { m.Slot = sixtyM }, "slot"},
		{"the \"000\" none slot", func(m *MemoryData) { m.Slot = noneSlot }, "slot"},
		{"an invalid Slot value", func(m *MemoryData) { m.Slot = Slot{} }, "slot"},
		{"kind '1', the read form's Memory", func(m *MemoryData) { m.Kind = KindMemory }, "Kind"},
		{"kind '5', PMS", func(m *MemoryData) { m.Kind = KindPMS }, "Kind"},
		{"a mode outside this dialect's table", func(m *MemoryData) { m.Mode = Mode('Z') }, "mode"},
		{"ModeUnset, which parsers accept and builders must never emit", func(m *MemoryData) { m.Mode = ModeUnset }, "ModeUnset"},
		{"a clarifier off this dialect's step", func(m *MemoryData) { m.ClarHz = 5 }, "ClarHz"},
		{"a clarifier past this dialect's range", func(m *MemoryData) { m.ClarHz = 9999 }, "ClarHz"},
		{"FreqHz zero", func(m *MemoryData) { m.FreqHz = 0 }, "FreqHz"},
		{"FreqHz wider than the 9-digit field", func(m *MemoryData) { m.FreqHz = memFreqMax + 1 }, "FreqHz"},
		{"a forged CTCSSState", func(m *MemoryData) { m.CTCSS = CTCSSState('9') }, "CTCSS"},
		{"a forged Shift", func(m *MemoryData) { m.Shift = Shift('9') }, "shift"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := validCombinedMemory(t, d)
			tc.mutate(&m)

			cmd, err := d.BuildMTSetCombined(m, "CQ")
			if err == nil {
				t.Fatalf("BuildMTSetCombined() succeeded, want a refusal naming %q", tc.want)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("error is %T, want *ParseError", err)
			}
			if !strings.Contains(pe.Reason, tc.want) {
				t.Errorf("refusal = %q, want it to name %q — a rule reporting the wrong field sends whoever has to fix it to the wrong place", pe.Reason, tc.want)
			}
			if !cmd.IsZero() {
				t.Errorf("returned a non-zero Command alongside its error; every fallible builder returns Command{} (command.go)")
			}
		})
	}

	// The positive control: the unmutated record builds. Without it every
	// entry above would pass against a builder that refused everything.
	if _, err := d.BuildMTSetCombined(validCombinedMemory(t, d), "CQ"); err != nil {
		t.Errorf("the unmutated record = %v, want a frame", err)
	}
}

// TestBuildMTSetCombined_P7IsAFormConstantNotTheMWWriteKind is the
// DECOUPLING proof.
//
// MT-Set P7 and MW-Set P7 are two command-specific facts that happen to
// coincide on the evidenced radio; deriving one from the other is the
// PadByte conflation this project has already paid for once. So a dialect
// whose MW writes carry '1' still emits '0' in a combined MT Set — asserted
// on the frame byte itself, at the shared field block's kind offset — and
// still REFUSES a record carrying that same '1', even though it is the
// dialect's own MW write kind.
func TestBuildMTSetCombined_P7IsAFormConstantNotTheMWWriteKind(t *testing.T) {
	d := mustCombinedDialect(t, 6, ' ', KindMemory)
	if d.MWWriteKind() != KindMemory {
		t.Fatalf("fixture MWWriteKind() = %q, want %q — the case needs a dialect whose MW kind is NOT the MT Set constant", d.MWWriteKind(), KindMemory)
	}
	if KindMemory == combinedMTSetKind {
		t.Fatal("the fixture's MW write kind equals the MT Set constant, so this test proves nothing")
	}

	got := mustCombinedFrame(t, d, "CQ")
	if got[memKindOffset] != combinedMTSetKind {
		t.Errorf("P7 = %q, want %q — the combined Set's kind is the FORM's schema constant (\"(Fixed)\"), not the dialect's MW write kind (%q)", got[memKindOffset], combinedMTSetKind, d.MWWriteKind())
	}

	// And the mirror: the dialect's own MW write kind is not admissible here.
	m := validCombinedMemory(t, d)
	m.Kind = d.MWWriteKind()
	cmd, err := d.BuildMTSetCombined(m, "CQ")
	if err == nil {
		t.Fatalf("BuildMTSetCombined() with Kind = MWWriteKind() succeeded, want a refusal — the two fields are not the same fact")
	}
	if !cmd.IsZero() {
		t.Errorf("returned a non-zero Command alongside its error; every fallible builder returns Command{} (command.go)")
	}

	// A second fixture, differing only in MW write kind, emits the same P7.
	other := mustCombinedDialect(t, 6, ' ', KindMemTune)
	otherFrame := mustCombinedFrame(t, other, "CQ")
	if otherFrame[memKindOffset] != combinedMTSetKind {
		t.Errorf("P7 = %q on a %q-writing dialect, want %q", otherFrame[memKindOffset], other.MWWriteKind(), combinedMTSetKind)
	}
}

// TestParseMTAnswerCombined_KindVocabularyNarrows pins the read-side P7
// vocabulary to the documented pair.
//
// The shared field block's decoder accepts MR's wider '0'-'5', which is
// right for MR and wrong here: the combined record's manual documents only
// "0: VFO, 1: Memory" on a read, so accepting '2'-'5' would turn an
// undocumented frame into data with a plausible Kind in it. The narrowing
// happens AFTER parseMemoryFields, which is why '2'-'5' reach it at all.
func TestParseMTAnswerCombined_KindVocabularyNarrows(t *testing.T) {
	d := mustCombinedDialect(t, 6, ' ', KindMemory)
	base := mustCombinedFrame(t, d, "CQ")

	for _, kind := range []byte{KindVFO, KindMemory} {
		frame := append([]byte(nil), base...)
		frame[memKindOffset] = kind
		m, _, err := d.ParseMTAnswerCombined(frame)
		if err != nil {
			t.Errorf("ParseMTAnswerCombined() with P7 %q = %v, want accepted — it is documented read vocabulary", kind, err)
			continue
		}
		if m.Kind != kind {
			t.Errorf("ParseMTAnswerCombined() Kind = %q, want %q", m.Kind, kind)
		}
	}

	for _, kind := range []byte{KindMemTune, KindQMB, KindUnset, KindPMS} {
		frame := append([]byte(nil), base...)
		frame[memKindOffset] = kind
		if _, _, err := d.ParseMTAnswerCombined(frame); err == nil {
			t.Errorf("ParseMTAnswerCombined() with P7 %q succeeded, want a refusal — MR's wider vocabulary must not become MT data", kind)
		} else {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseMTAnswerCombined() with P7 %q: error is %T, want *ParseError", kind, err)
			}
		}
	}
}

// TestParseMTAnswerCombined_RefusesShapeDeviations covers the framing the
// parser owns: length, prefix, terminator and P11.
//
// THE EXACT LENGTH IS ASSUMED UNTIL STAGE R. The manual's tag legend says
// "up to 12 characters" — the same variable-length wording as the FT-710's,
// whose hardware accepts short Sets — so whether an FTdx10 ANSWERS
// fixed-width is unverified. The recorded contingency, if Stage R shows it
// answers variable-width, is a parser/gate window of 30..29+TagMaxBytes: a
// contained change to this length check and the gate's, not a schema
// change. The gate's exactness is safe either way, since it checks outbound
// frames and this package only ever builds full-width.
func TestParseMTAnswerCombined_RefusesShapeDeviations(t *testing.T) {
	d := mustCombinedDialect(t, 6, ' ', KindMemory)
	base := mustCombinedFrame(t, d, "CQ")
	if len(base) != 35 {
		t.Fatalf("fixture frame is %d bytes, want 35", len(base))
	}

	// 34 bytes: one fill byte short of the field width, terminator intact,
	// so only the LENGTH is wrong.
	short := append([]byte(nil), base[:len(base)-2]...)
	short = append(short, ';')
	// 36 bytes: one fill byte too many, terminator intact.
	long := append([]byte(nil), base[:len(base)-1]...)
	long = append(long, ' ', ';')

	badPrefix := append([]byte(nil), base...)
	badPrefix[1] = 'W'
	noTerm := append([]byte(nil), base...)
	noTerm[len(noTerm)-1] = 'X'
	badP11 := append([]byte(nil), base...)
	badP11[27] = '1'

	tests := []struct {
		name  string
		frame []byte
	}{
		{"34 bytes, one short of the derived length", short},
		{"36 bytes, one over the derived length", long},
		{"a short-form MT Set frame", []byte("MT0071CQ;")},
		{"the wrong prefix", badPrefix},
		{"no terminator", noTerm},
		{"P11 is '1', not the fixed '0'", badP11},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.frame) == 34 || len(tc.frame) == 36 {
				// Guard the fixtures themselves: a length case that was not
				// the length it claims would pass for the wrong reason.
				if tc.frame[len(tc.frame)-1] != ';' {
					t.Fatalf("length fixture %q does not end in ';' — it would be refused by the terminator rule instead", tc.frame)
				}
			}
			_, _, err := d.ParseMTAnswerCombined(tc.frame)
			if err == nil {
				t.Fatalf("ParseMTAnswerCombined(%q) succeeded, want a refusal", tc.frame)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseMTAnswerCombined(%q): error is %T, want *ParseError", tc.frame, err)
			}
		})
	}
}

// TestMTCombinedAPIs_RefuseAShortDialect is the form seam's other
// direction, symmetric with mtform_test.go's
// TestMTShortAPIs_RefuseACombinedDialect: the combined API offered a
// short-form dialect must refuse BY FORM and name it.
//
// A silent success is what this prevents. Both forms' frames begin "MT" and
// end ';', so a combined builder handed the FT-710 would emit a 41-byte
// record to a radio that reads its first six bytes as a slot and a display
// flag.
func TestMTCombinedAPIs_RefuseAShortDialect(t *testing.T) {
	slot, err := FT710.MemorySlot(7)
	if err != nil {
		t.Fatalf("MemorySlot(7): %v", err)
	}
	m := MemoryData{
		Slot: slot, FreqHz: 14_250_000, ClarHz: 0,
		Mode: ModeUSB, Kind: combinedMTSetKind,
		CTCSS: CTCSSOff, Shift: ShiftSimplex,
	}

	cmd, err := FT710.BuildMTSetCombined(m, "CQ")
	if err == nil {
		t.Fatalf("FT710.BuildMTSetCombined() succeeded, want a form refusal")
	}
	if !strings.Contains(err.Error(), "MTFormShort") {
		t.Errorf("FT710.BuildMTSetCombined() = %q, want the message to name the dialect's form (MTFormShort)", err)
	}
	if !cmd.IsZero() {
		t.Errorf("FT710.BuildMTSetCombined() returned a non-zero Command alongside its error; every fallible builder returns Command{} (command.go)")
	}

	if _, _, err := FT710.ParseMTAnswerCombined([]byte(wantCQDXFrame)); err == nil {
		t.Fatalf("FT710.ParseMTAnswerCombined() succeeded, want a form refusal")
	} else if !strings.Contains(err.Error(), "MTFormShort") {
		t.Errorf("FT710.ParseMTAnswerCombined() = %q, want the message to name the dialect's form (MTFormShort)", err)
	}

	// The zero Dialect, which has no form at all, refuses both too.
	var zero Dialect
	if cmd, err := zero.BuildMTSetCombined(m, "CQ"); err == nil {
		t.Errorf("zero.BuildMTSetCombined() succeeded, want a form refusal")
	} else if !cmd.IsZero() {
		t.Errorf("zero.BuildMTSetCombined() returned a non-zero Command alongside its error")
	}
	if _, _, err := zero.ParseMTAnswerCombined([]byte(wantCQDXFrame)); err == nil {
		t.Errorf("zero.ParseMTAnswerCombined() succeeded, want a form refusal")
	}
}

// TestMTAnswerBounds pins the receiver-derived geometry M9c-5's transport
// spec factories consume instead of hardcoding 41 or re-deriving
// 29+TagMaxBytes for themselves.
//
// The zero Dialect's case is the one worth writing down: it returns an
// ERROR, not (0, 0, nil). A driver reading plausible zeros would build a
// CommandSpec accepting a zero-length answer, which is a gate that admits
// nothing and reports no reason.
func TestMTAnswerBounds(t *testing.T) {
	min, max, err := FT710.MTAnswerBounds()
	if err != nil {
		t.Fatalf("FT710.MTAnswerBounds() = %v, want bounds", err)
	}
	if min != 7 || max != 19 {
		t.Errorf("FT710.MTAnswerBounds() = (%d, %d), want (7, 19) — the short form's floor plus its own 12-byte tag", min, max)
	}

	d := mustCombinedDialect(t, 6, ' ', KindMemory)
	min, max, err = d.MTAnswerBounds()
	if err != nil {
		t.Fatalf("combined.MTAnswerBounds() = %v, want bounds", err)
	}
	if min != 35 || max != 35 {
		t.Errorf("combined.MTAnswerBounds() = (%d, %d), want (35, 35) — the combined form's length is exact, 29 + TagMaxBytes(6)", min, max)
	}

	wide := mustCombinedDialect(t, 12, '_', KindMemory)
	if min, max, err := wide.MTAnswerBounds(); err != nil {
		t.Errorf("wide.MTAnswerBounds() = %v, want bounds", err)
	} else if min != 41 || max != 41 {
		t.Errorf("wide.MTAnswerBounds() = (%d, %d), want (41, 41) — the evidenced 12-byte-tag geometry", min, max)
	}

	var zero Dialect
	if min, max, err := zero.MTAnswerBounds(); err == nil {
		t.Errorf("zero.MTAnswerBounds() = (%d, %d), nil — want an error, never plausible zeros", min, max)
	} else if !strings.Contains(err.Error(), "MTFormUnspecified") {
		t.Errorf("zero.MTAnswerBounds() = %q, want the message to name the dialect's form (MTFormUnspecified)", err)
	}
}

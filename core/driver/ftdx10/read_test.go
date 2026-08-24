// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// readTestImage is the scripted radio the read tests share. Each slot is a
// case:
//
//	001 — a populated channel, every field a different value from 010's
//	010 — a second populated channel, covering the other end of each
//	      vocabulary (mode 'F', CTCSS off, MINUS shift, a SHORT tag whose
//	      wire field is space-padded, a POSITIVE clarifier with TX clar on)
//	002 — absent from the image, so answered "?;": the empty-slot path
//	003 — an answer carrying kind '2', which parseMemoryFields accepts as
//	      structurally valid but the combined record's own vocabulary does
//	      not: the out-of-vocabulary path
//	004 — an answer that names slot 005: the slot-echo path
//	020 — a JUNK frame ahead of a valid answer, for the transport-logger
//	      and unexpected-frame path (ftdx10_test.go)
//
// One session serves all of them (each Open costs ~2.8 s — see
// ftdx10_test.go), which is safe because these are independent single
// exchanges with no state on either side.
func readTestImage() slotImage {
	return slotImage{mtAnswers: map[string]string{
		"001": mtAnswerFields{
			slot: "001", freq: "014250000",
			clarSign: '-', clarMag: "0150", rxClar: '1', txClar: '0',
			mode: '2', kind: '1', ctcss: '1', shift: '1',
			tag: "CALLING",
		}.frame(),
		"010": mtAnswerFields{
			slot: "010", freq: "000030000",
			clarSign: '+', clarMag: "9990", rxClar: '0', txClar: '1',
			mode: 'F', kind: '0', ctcss: '0', shift: '2',
			tag: "AB",
		}.frame(),
		"003": mtAnswerFields{
			slot: "003", freq: "014250000",
			clarSign: '+', clarMag: "0000", rxClar: '0', txClar: '0',
			// '2' is cat.KindMemTune: inside parseMemoryFields' documented
			// read vocabulary '0'-'5', OUTSIDE the combined record's own
			// documented {'0' VFO, '1' Memory} pair.
			mode: '1', kind: '2', ctcss: '0', shift: '0',
			tag: "MEMTUNE",
		}.frame(),
		"004": mtAnswerFields{
			// The frame answers for a DIFFERENT slot than the read asked
			// about — the stale-reply shape the transport's quarantine
			// discipline is meant to prevent, checked anyway because
			// mapping an answer onto the wrong channel would corrupt a
			// codeplug silently.
			slot: "005", freq: "014250000",
			clarSign: '+', clarMag: "0000", rxClar: '0', txClar: '0',
			mode: '1', kind: '1', ctcss: '0', shift: '0',
			tag: "WRONGSLOT",
		}.frame(),
		// A complete but UNEXPECTED frame arriving ahead of the real
		// answer — another application sharing the port, or a reply to
		// something this session never sent. The transport must surface it
		// (safety obligation 3) and keep waiting within the same budget,
		// so the read still succeeds. Two frames in one write, which the
		// accumulator splits.
		"020": "XX;" + populatedAnswer("020"),
	}}
}

// TestReadChannel_MappingsFromThePositionChart drives ReadChannel against
// answers assembled BY POSITION from the manual's MT chart and pins the
// whole wire -> codeplug.ChannelData mapping. Every expected value is
// written out literally, never computed by the code under test.
//
// TagDisplay is codeplug.Unavailable on both channels and it is NOT an
// omission: this radio's combined record has no display flag at all, so
// there is no value to report and "Unknown" — which means "the radio has
// one and this read did not learn it" — would be a different and false
// claim. The FTdx10 is this project's first radio to produce Unavailable
// from a real read path.
//
// CTCSSTone and ScanSkip are Unknown: the TONE AND SCAN-SKIP
// UNREACHABILITY register entry, nothing readable.
func TestReadChannel_MappingsFromThePositionChart(t *testing.T) {
	_, sess := openSession(t, Simulated, readTestImage())

	for _, tt := range []struct {
		name string
		slot string
		want codeplug.ChannelData
	}{
		{
			name: "14.250 MHz USB, clarifier -150 Hz RX, ENC-DEC, PLUS, tagged",
			slot: "001",
			want: codeplug.ChannelData{
				FreqHz:     14_250_000,
				Mode:       "USB",
				ClarHz:     -150,
				RxClar:     true,
				TxClar:     false,
				CTCSS:      "ENC-DEC",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "PLUS",
				Tag:        "CALLING",
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		},
		{
			// The other end of every vocabulary: the mode legend's last
			// nibble, the clarifier at its declared maximum with the
			// opposite sign and the TX flag, CTCSS off, MINUS shift, and a
			// two-character tag whose 12-byte wire field is space-padded
			// (the padding is the DIALECT's ASSUMED TagFill — its own
			// register entry MTPolicy.TagFill — and it must be trimmed off
			// before the
			// value reaches a codeplug).
			name: "30 kHz DATA-FM-N, clarifier +9990 Hz TX, off, MINUS, short tag",
			slot: "010",
			want: codeplug.ChannelData{
				FreqHz:     30_000,
				Mode:       "DATA-FM-N",
				ClarHz:     9990,
				RxClar:     false,
				TxClar:     true,
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "MINUS",
				Tag:        "AB",
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch, err := sess.ReadChannel(testCtx(t), tt.slot)
			if err != nil {
				t.Fatalf("ReadChannel(%q) = %v, want nil", tt.slot, err)
			}
			if ch.Slot != tt.slot {
				t.Errorf("Channel.Slot = %q, want %q", ch.Slot, tt.slot)
			}
			if ch.Data == nil {
				t.Fatal("Channel.Data = nil, want populated")
			}
			want := tierUnavailable(tt.want)
			if !reflect.DeepEqual(*ch.Data, want) {
				t.Errorf("ChannelData =\n %+v\nwant\n %+v", *ch.Data, want)
			}
		})
	}
}

// TestReadChannel_EmptySlotIsNotAnError: a "?;" rejection maps to an EMPTY
// channel — Data nil, the slot carried through — and no error. ASSUMED:
// "?;" ON A COMBINED-MT READ OF AN EMPTY SLOT register entry. "?;" is the
// protocol's single unattributed NAK, so this
// mapping is an interpretation, and it is the interpretation the whole read
// path rests on (a full-radio read hits it 98 times on a nearly-empty
// radio).
func TestReadChannel_EmptySlotIsNotAnError(t *testing.T) {
	_, sess := openSession(t, Simulated, readTestImage())

	ch, err := sess.ReadChannel(testCtx(t), "002")
	if err != nil {
		t.Fatalf("ReadChannel(\"002\") = %v, want nil (a rejection is an empty slot, not a failure)", err)
	}
	if ch.Slot != "002" {
		t.Errorf("Channel.Slot = %q, want \"002\" — the slot must survive an empty read", ch.Slot)
	}
	if !ch.Empty() {
		t.Errorf("Channel.Empty() = false (Data = %+v), want an empty channel", ch.Data)
	}
}

// TestReadChannel_ErrorTyping covers the two failure classes the M9c-6
// plan requires to be TYPED and distinguishable, both via errors.As.
//
// A malformed or out-of-vocabulary ANSWER is the parser's verdict and stays
// a *cat.ParseError under this driver's wrap, so a caller can tell "the
// radio said something this protocol does not define" apart from "the
// radio answered about the wrong channel", which is this driver's own
// *AnswerMismatchError. Neither is a bare fmt.Errorf, and both carry the
// slot: the bare parser cannot know it, and an error naming no channel is
// nearly useless in a 99-slot read.
func TestReadChannel_ErrorTyping(t *testing.T) {
	_, sess := openSession(t, Simulated, readTestImage())

	t.Run("out-of-vocabulary kind byte is a wrapped cat.ParseError", func(t *testing.T) {
		_, err := sess.ReadChannel(testCtx(t), "003")
		if err == nil {
			t.Fatal("ReadChannel(\"003\") = nil error, want a refusal: the answer's P7 is '2', outside the combined record's documented {'0','1'} read pair")
		}
		var pe *cat.ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("error %v (%T) is not a wrapped *cat.ParseError — the kind vocabulary is the PARSER's to enforce, and its typed verdict must survive this driver's wrap", err, err)
		}
		if !strings.Contains(pe.Reason, "P7") {
			t.Errorf("ParseError.Reason = %q, want it to name the offending field (P7)", pe.Reason)
		}
		if !strings.Contains(err.Error(), "003") {
			t.Errorf("error text %q does not name slot \"003\" — the driver's wrap is what adds the slot context the parser cannot know", err.Error())
		}
	})

	t.Run("slot-echo mismatch is the driver's own typed error", func(t *testing.T) {
		_, err := sess.ReadChannel(testCtx(t), "004")
		if !errors.Is(err, ErrAnswerMismatch) {
			t.Fatalf("ReadChannel(\"004\") = %v, want errors.Is match against ErrAnswerMismatch", err)
		}
		var ame *AnswerMismatchError
		if !errors.As(err, &ame) {
			t.Fatalf("error %v (%T) is not an *AnswerMismatchError", err, err)
		}
		if ame.Requested != "004" || ame.Answered != "005" {
			t.Errorf("AnswerMismatchError = {Requested:%q Answered:%q}, want {\"004\" \"005\"}", ame.Requested, ame.Answered)
		}
	})

	t.Run("a slot this dialect does not define is refused before the wire", func(t *testing.T) {
		_, err := sess.ReadChannel(testCtx(t), "0X1")
		var pe *cat.ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("ReadChannel(\"0X1\") = %v (%T), want a wrapped *cat.ParseError from ParseSlot", err, err)
		}
	})

	t.Run("the none form is grammatical but never a read target", func(t *testing.T) {
		// "000" parses (the dialect's ASSUMED SlotSpace.NoneWire, its own
		// register entry
		// 3) but BuildMTRead refuses it: the manual's MT column marks it
		// unavailable and its semantics are unknown.
		_, err := sess.ReadChannel(testCtx(t), "000")
		var pe *cat.ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("ReadChannel(\"000\") = %v (%T), want a wrapped *cat.ParseError from BuildMTRead", err, err)
		}
	})
}

// TestReadChannel_SendsExactlyOneMTAndNeverMR is the MT-ONLY decision's
// enforcement on the wire.
//
// One read is ONE frame: that is what makes the answer an atomic snapshot
// of the channel, and it is why this Session needs no operation mutex
// (doc.go). And no MR frame is sent by this driver at ANY point — not by a
// read, and not by discovery, which probes with MT reads for this same
// reason. A future "completion" of the read path that added the FT-710's MR
// exchange would make doc.go's statement false while every other test kept
// passing; this is what catches it.
func TestReadChannel_SendsExactlyOneMTAndNeverMR(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	before := len(p.Transcript())
	if _, err := sess.ReadChannel(testCtx(t), "001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	after := p.Transcript()

	if got := after[before:]; len(got) != 1 || got[0] != "MT001;" {
		t.Errorf("one ReadChannel sent %v, want exactly [\"MT001;\"]", got)
	}
	for i, frame := range after {
		if strings.HasPrefix(frame, "MR") {
			t.Errorf("frame %d = %q: this driver must never send MR — the read path is MT-only and discovery probes with MT (see doc.go)", i, frame)
		}
	}
}

// TestMTSpec_DerivesItsLengthFromTheDialect pins that the MT read's
// expected answer length comes from the DIALECT's own geometry and not from
// a number in this package.
//
// There is no 41 in the production code, deliberately: the combined
// answer's exactness is itself an assumption the dialect carries (its
// register entry "the combined MT answer's EXACT length"), whose recorded
// Stage R contingency is a 30..41 WINDOW.
// If that contingency is ever taken, the bounds move in core/cat and this
// spec must move with them — which it does only while the length is
// derived. The unconfigured-dialect case is the same guard from the other
// side: a zero dialect has no MT form, so it gets an error rather than a
// plausible zero ExpectLen (which would admit any answer at all).
func TestMTSpec_DerivesItsLengthFromTheDialect(t *testing.T) {
	lo, hi, err := catDialect.MTAnswerBounds()
	if err != nil {
		t.Fatalf("catDialect.MTAnswerBounds() = %v, want the combined form's exact bounds", err)
	}
	if lo != hi {
		t.Fatalf("MTAnswerBounds() = %d..%d, want equal bounds for the combined form", lo, hi)
	}
	// The chart's own arithmetic, asserted HERE where the manual is the
	// authority: "MT" + the 28-position field block + P11 + a 12-byte tag
	// + ';' = 41 (manual lines 1217-1256).
	if hi != 41 {
		t.Errorf("the dialect's combined MT answer length = %d, want 41 per the manual's MT position chart", hi)
	}

	spec, err := mtSpec(catDialect)
	if err != nil {
		t.Fatalf("mtSpec(catDialect) = %v, want nil", err)
	}
	if spec.Class != transport.ClassRead {
		t.Errorf("Class = %v, want transport.ClassRead", spec.Class)
	}
	// The prefix and the exact length, asserted THROUGH THE MATCHER
	// rather than off the struct: D2 moved answer matching into an
	// opaque transport.CommandSpec.Match built by the codec, so the
	// fields this used to read (ExpectPrefix, ExpectLen) no longer
	// exist. What they asserted still holds, and these three cases say
	// so in the terms the engine now uses — a well-formed answer of the
	// dialect's own length matches, one byte short does not, and another
	// command's answer of the right length does not.
	rightLength := "MT" + strings.Repeat("0", hi-3) + ";"
	oneShort := "MT" + strings.Repeat("0", hi-4) + ";"
	wrongCommand := "MR" + strings.Repeat("0", hi-3) + ";"
	if !spec.Match([]byte(rightLength)) {
		t.Errorf("Match(%q) = false, want true — that is the dialect's own %d-byte combined MT answer", rightLength, hi)
	}
	if spec.Match([]byte(oneShort)) {
		t.Errorf("Match(%q) = true, want false — the length is pinned to the dialect's %d, not merely to the prefix", oneShort, hi)
	}
	if spec.Match([]byte(wrongCommand)) {
		t.Errorf("Match(%q) = true, want false — the prefix must discriminate the command", wrongCommand)
	}
	if spec.RetryReads != 1 {
		t.Errorf("RetryReads = %d, want 1 (a read is idempotent; a single swallowed reply must not fail an operation)", spec.RetryReads)
	}

	if _, err := mtSpec(cat.Dialect{}); err == nil {
		t.Error("mtSpec(zero dialect) = nil error, want a refusal — an unconfigured dialect has no MT geometry, and a zero exact length would admit any MT answer")
	}
}

// tierUnavailable returns d with every one of the ten fields the Icom
// tier added to codeplug.ChannelData set to Unavailable — what this
// radio's ReadChannel reports for all of them, because its memory frame
// carries none of them (design D4; the TagDisplay precedent, applied ten
// times).
//
// The read tests' `want` literals name the fields this radio actually
// HAS and wrap the result in this, rather than spelling out ten
// Unavailable lines each: the interesting content of every case stays
// visible, and "and everything the Icom tier added is Unavailable" is
// stated once, where it can be read as the single fact it is.
func tierUnavailable(d codeplug.ChannelData) codeplug.ChannelData {
	d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.Duplex = codeplug.StringField{State: codeplug.Unavailable}
	d.OffsetHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.ToneMode = codeplug.StringField{State: codeplug.Unavailable}
	d.ToneTx = codeplug.ToneField{State: codeplug.Unavailable}
	d.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
	d.DTCSCode = codeplug.IntField{State: codeplug.Unavailable}
	d.DTCSPolarity = codeplug.StringField{State: codeplug.Unavailable}
	d.Filter = codeplug.StringField{State: codeplug.Unavailable}
	d.DataMode = codeplug.BoolField{State: codeplug.Unavailable}
	return d
}

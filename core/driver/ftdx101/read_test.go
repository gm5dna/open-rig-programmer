// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// readTestImage is the scripted radio the read tests share. Each slot is a
// case:
//
//	001 — a populated channel, every field a different value from 010's
//	010 — a second populated channel, covering the other end of each
//	      vocabulary (mode 'F', CTCSS off, MINUS shift, a SHORT tag whose
//	      wire field is space-padded, a POSITIVE clarifier with TX clar on)
//	002 — absent from the image, so answered "?;": the empty-slot path
//	003 — an answer carrying kind '2', which core/cat's structural parse
//	      accepts but the combined record's own documented {'0','1'} read
//	      pair does not: the out-of-vocabulary path
//	004 — an answer that names slot 005: the slot-echo path
//	020 — a JUNK frame ahead of a valid answer, for the transport-logger
//	      and unexpected-frame path (ftdx101_test.go)
//
// It carries no catID: openSession fills that in from the model being
// opened, so this one image serves either radio (slotImage.catID's doc
// comment has the reasoning).
//
// One session serves all of them (each Open costs ~2-3 s — see
// ftdx101_test.go), which is safe because these are independent single
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
		"003": malformedAnswer("003"),
		"004": wrongSlotAnswer("005"),
		// A complete but UNEXPECTED frame arriving ahead of the real
		// answer — another application sharing the port, or a reply to
		// something this session never sent. The transport must surface it
		// (safety obligation 3) and keep waiting within the same budget, so
		// the read still succeeds. Two frames in one write, which the
		// accumulator splits.
		"020": "XX;" + populatedAnswer("020"),
	}}
}

// TestReadChannel_MappingsFromThePositionChart drives ReadChannel against
// answers assembled BY POSITION from this manual's MT chart (layout
// 1311-1330) and pins the whole wire -> codeplug.ChannelData mapping. Every
// expected value is written out literally, never computed by the code under
// test.
//
// BOTH MODELS, because the read path renders through the session's own
// dialect and a session that had picked up the wrong model's would still
// pass a one-model version of this test (the two dialects agree on every
// byte but the CAT ID — matrix §2.5 — which is exactly why an accidental
// swap would be invisible unless both are exercised).
//
// TagDisplay is codeplug.Unavailable on both channels and it is NOT an
// omission: this radio's combined record has no display flag at all (P11
// fixed "0", layout 1329 — a MANUAL-EVIDENCED ABSENCE, matrix §3.7), so
// there is no value to report and "Unknown" — which means "the radio has
// one and this read did not learn it" — would be a different and false
// claim.
//
// CTCSSTone and ScanSkip are Unknown: register entry 6, nothing readable.
func TestReadChannel_MappingsFromThePositionChart(t *testing.T) {
	for _, m := range testModels {
		_, sess := openSession(t, m, Simulated, readTestImage())

		for _, tt := range []struct {
			name string
			slot string
			want codeplug.ChannelData
		}{
			{
				name: "14.250 MHz USB, clarifier -150 Hz RX, ENC-DEC, PLUS, tagged",
				slot: "001",
				want: codeplug.ChannelData{
					FreqHz: 14_250_000,
					Mode:   "USB",
					// A NEGATIVE offset, so this leg exercises the sign
					// position whose minus byte is the DIALECT register's
					// unlifted entry ("the ASCII HYPHEN-MINUS 0x2D"): core/cat
					// accepts only '+' or '-' there, so a radio using some
					// other byte fails the parse loudly rather than reading a
					// negative offset as positive.
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
				// opposite sign and the TX flag, CTCSS off, MINUS shift, and
				// a two-character tag whose 12-byte wire field is
				// space-padded (the padding is the DIALECT register's
				// ASSUMED "MTPolicy.TagFill = ' '", and it must be trimmed
				// off before the value reaches a codeplug).
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
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
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
				if !reflect.DeepEqual(*ch.Data, tt.want) {
					t.Errorf("ChannelData =\n %+v\nwant\n %+v", *ch.Data, tt.want)
				}
			})
		}
	}
}

// TestReadChannel_EmptySlotIsNotAnError: a "?;" rejection maps to an EMPTY
// channel — Data nil, the slot carried through — and no error. ASSUMED:
// register entry 8. "?;" is the protocol's single unattributed NAK, so this
// mapping is an interpretation, and it is the interpretation the whole read
// path rests on (a full-radio read hits it 98 times on a nearly-empty
// radio).
func TestReadChannel_EmptySlotIsNotAnError(t *testing.T) {
	_, sess := openSession(t, testModels[0], Simulated, readTestImage())

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

// TestReadChannel_ErrorTyping covers the failure classes the plan requires
// to be TYPED and distinguishable, all via errors.As. MALFORMED ANSWERS
// REFUSE — none of them is guessed at.
//
// A malformed or out-of-vocabulary ANSWER is the parser's verdict and stays
// a *cat.ParseError under this driver's wrap, so a caller can tell "the
// radio said something this protocol does not define" apart from "the radio
// answered about the wrong channel", which is this package's own
// *AnswerMismatchError. Neither is a bare fmt.Errorf, and both carry the
// slot: the bare parser cannot know it, and an error naming no channel is
// nearly useless in a 99-slot read.
func TestReadChannel_ErrorTyping(t *testing.T) {
	_, sess := openSession(t, testModels[0], Simulated, readTestImage())

	t.Run("out-of-vocabulary kind byte is a wrapped cat.ParseError", func(t *testing.T) {
		_, err := sess.ReadChannel(testCtx(t), "003")
		if err == nil {
			t.Fatal("ReadChannel(\"003\") = nil error, want a refusal: the answer's P7 is '2', outside the combined record's documented {'0','1'} read pair (MT's P7 legend, layout 1324)")
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

	t.Run("slot-echo mismatch is this package's own typed error", func(t *testing.T) {
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
		// "000" parses — the DIALECT register's ASSUMED
		// "SlotSpace.NoneWire = \"000\"", a form that appears in no FTdx101
		// slot legend — but BuildMTRead refuses it: it is what an MR answer
		// carries when the source is not a memory, and never a slot to
		// address.
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
	p, sess := openSession(t, testModels[0], Simulated, readTestImage())

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
// a number in this package's production code.
//
// There is no 41 there, deliberately: the combined answer's exactness is
// itself an assumption the dialect carries — its register entry "The
// combined MT answer's EXACT length (consumed here as MTAnswerBounds() =
// (41, 41))" — whose recorded Stage R contingency, per model, is a 30..41
// WINDOW. If that contingency is ever taken, the bounds move in core/cat
// and this spec must move with them, which it does only while the length is
// derived. The unconfigured-dialect case is the same guard from the other
// side: a zero dialect has no MT form, so it gets an error rather than a
// plausible zero ExpectLen (which would admit any answer at all).
func TestMTSpec_DerivesItsLengthFromTheDialect(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			lo, hi, err := m.params.dialect.MTAnswerBounds()
			if err != nil {
				t.Fatalf("MTAnswerBounds() = %v, want the combined form's exact bounds", err)
			}
			if lo != hi {
				t.Fatalf("MTAnswerBounds() = %d..%d, want equal bounds for the combined form", lo, hi)
			}
			// The chart's own arithmetic, asserted HERE where the manual is
			// the authority: "MT" + the 28-position field block + P11 + a
			// 12-byte tag + ';' = 41 (layout 1311-1330, and the independent
			// 300 dpi geometry witness).
			if hi != 41 {
				t.Errorf("the dialect's combined MT answer length = %d, want 41 per this manual's MT position chart", hi)
			}

			cmdSpec, err := mtSpec(m.params.dialect)
			if err != nil {
				t.Fatalf("mtSpec(dialect) = %v, want nil", err)
			}
			if cmdSpec.ExpectPrefix != "MT" {
				t.Errorf("ExpectPrefix = %q, want \"MT\"", cmdSpec.ExpectPrefix)
			}
			if cmdSpec.ExpectLen != hi {
				t.Errorf("ExpectLen = %d, want the dialect's own %d", cmdSpec.ExpectLen, hi)
			}
			if cmdSpec.RetryReads != 1 {
				t.Errorf("RetryReads = %d, want 1 (a read is idempotent; a single swallowed reply must not fail an operation)", cmdSpec.RetryReads)
			}
		})
	}

	if _, err := mtSpec(cat.Dialect{}); err == nil {
		t.Error("mtSpec(zero dialect) = nil error, want a refusal — an unconfigured dialect has no MT geometry, and a zero ExpectLen would admit any answer")
	}
}

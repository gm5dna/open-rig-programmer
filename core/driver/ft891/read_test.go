// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// readTestImage is the scripted radio the read tests share. Each slot is a
// case, and between them they are the whole of matrix §3.5's truth table:
//
//	001 — MEM, MT answers: the atomic populated read, TAG flag ON
//	010 — MEM, MT answers: the other end of every vocabulary, TAG flag OFF
//	P1L — PMS, MT answers: the same path on the other static bank
//	002 — MT "?;" then MR "?;": the EMPTY slot
//	006 — MT "?;" then MR RECORD: the occupied-slot refusal
//	007 — MT SILENT: the timeout branch, one frame and no MR
//	003 — MT answers with kind '2': out of the combined record's read pair
//	004 — MT answers for slot 005: the slot-echo path
//	020 — a JUNK frame ahead of a valid MT answer (ft891_test.go's logger)
//	503, EMG — MR answers: the discovered banks, read by MR alone
//
// One session serves all of them (each Open costs an AI0 init plus thirteen
// exchanges), which is safe because these are independent operations with
// no state on either side.
func readTestImage() slotImage {
	return slotImage{
		mtAnswers: map[string]string{
			"001": memoryFields{
				slot: "001", freq: "014250000",
				clarSign: '-', clarMag: "0150", rxClar: '1',
				mode: '2', kind: '1', ctcss: '1', shift: '1',
			}.mtFrame(true, "CALLING"),
			"010": memoryFields{
				slot: "010", freq: "000030000",
				clarSign: '+', clarMag: "9990", rxClar: '0',
				mode: 'D', kind: '0', ctcss: '0', shift: '2',
			}.mtFrame(false, "AB"),
			"P1L": memoryFields{
				slot: "P1L", freq: "007100000",
				clarSign: '+', clarMag: "0000", rxClar: '0',
				mode: '1', kind: '1', ctcss: '2', shift: '0',
			}.mtFrame(true, "SCAN LOW"),
			"003": memoryFields{
				slot: "003", freq: "014250000",
				clarSign: '+', clarMag: "0000", rxClar: '0',
				// '2' is cat.KindMemTune: inside parseMemoryFields'
				// documented '0'-'5' read vocabulary, OUTSIDE the combined
				// record's own {'0' VFO, '1' Memory} pair.
				mode: '1', kind: '2', ctcss: '0', shift: '0',
			}.mtFrame(false, "MEMTUNE"),
			"004": memoryFields{
				// The frame answers for a DIFFERENT slot than the read
				// asked about — the stale-reply shape the transport's
				// quarantine discipline is meant to prevent, checked
				// anyway because mapping an answer onto the wrong channel
				// would corrupt a codeplug silently.
				slot: "005", freq: "014250000",
				clarSign: '+', clarMag: "0000", rxClar: '0',
				mode: '1', kind: '1', ctcss: '0', shift: '0',
			}.mtFrame(false, "WRONGSLOT"),
			// A complete but UNEXPECTED frame arriving ahead of the real
			// answer — another application sharing the port, or a reply to
			// something this session never sent. The transport must surface
			// it (safety obligation 3) and keep waiting within the same
			// budget, so the read still succeeds. Two frames in one write,
			// which the accumulator splits.
			"020": "XX;" + populatedMT("020"),
		},
		mrAnswers: map[string]string{
			// 006's MT is absent (so "?;") while its MR carries a record:
			// the pair the manual's own contradiction predicts, and the one
			// this driver refuses loudly rather than diagnosing.
			"006": populatedMR("006"),
			// The discovered banks, which are read by MR alone.
			"503": populatedMR("503"),
			"EMG": populatedMR("EMG"),
		},
		mtSilent: map[string]bool{"007": true},
	}
}

// TestReadChannel_MappingsFromThePositionChart drives ReadChannel against
// answers assembled BY POSITION from the manual's MT chart and pins the
// whole wire -> codeplug.ChannelData mapping. Every expected value is
// written out literally, never computed by the code under test.
//
// TagDisplay comes back KNOWN, with the flag the answer's byte 28 carried,
// and that is this radio's inversion of both combined-form siblings: MT's
// P11 legend here reads `0: TAG "OFF" 1: TAG "ON"` (layout 1016) where the
// FTdx10's and FTdx101's print "0: (Fixed)" and their drivers therefore
// report Unavailable. Known means "this read learned it"; that the radio
// really REPORTS the byte rather than always answering '0' is the ASSUMED
// register's READ-BACK OF THE TAG DISPLAY FLAG entry.
//
// TxClar is false on every row and cannot be otherwise: under the dialect's
// MemoryP5 = cat.P5Fixed, core/cat REQUIRES '0' at byte 21 and returns
// TxClar false, so no channel read from an FT-891 ever carries a true TX
// flag (matrix §2.2).
//
// CTCSSTone and ScanSkip are Unknown: the ASSUMED register's TONE AND
// SCAN-SKIP UNREACHABILITY entry — "Unknown" means "preserve whatever the
// radio has" to every write path downstream, which is the only honest
// instruction for a field this driver cannot see.
func TestReadChannel_MappingsFromThePositionChart(t *testing.T) {
	_, sess := openSession(t, Simulated, readTestImage())

	for _, tt := range []struct {
		name string
		slot string
		want codeplug.ChannelData
	}{
		{
			name: "14.250 MHz USB, clarifier -150 Hz RX, ENC-DEC, PLUS, tagged, TAG ON",
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
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		},
		{
			// The other end of every vocabulary: the manual's LAST mode
			// nibble ('D', AM-N — the ASSUMED register's THE MODE NIBBLE'S
			// TOP END entry), the clarifier at its declared maximum with
			// the opposite sign, CTCSS off, MINUS shift, a two-character
			// tag whose 12-byte wire field is space-padded (the padding is
			// the DIALECT register's ASSUMED MTPolicy.TagFill, and it must
			// be trimmed off before the value reaches a codeplug), and the
			// TAG flag OFF.
			name: "30 kHz AM-N, clarifier +9990 Hz, off, MINUS, short tag, TAG OFF",
			slot: "010",
			want: codeplug.ChannelData{
				FreqHz:     30_000,
				Mode:       "AM-N",
				ClarHz:     9990,
				RxClar:     false,
				TxClar:     false,
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "MINUS",
				Tag:        "AB",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		},
		{
			// PMS takes the same path as MEM: the memory-channel surface is
			// printed once and carries no per-bank qualifier (matrix §2.7),
			// and MT's slot legend admits memory and PMS alike.
			name: "PMS lower limit, 7.100 MHz LSB, ENC, SIMPLEX",
			slot: "P1L",
			want: codeplug.ChannelData{
				FreqHz:     7_100_000,
				Mode:       "LSB",
				ClarHz:     0,
				RxClar:     false,
				TxClar:     false,
				CTCSS:      "ENC",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				Tag:        "SCAN LOW",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
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
			drivertest.AssertFreshReadSaveLoad(t, ch, sess.Capabilities(), codeplug.Load)
		})
	}
}

// TestReadChannel_OnePopulatedReadIsOneFrame: the whole point of reading
// memory and PMS by the COMBINED MT form is that a populated channel costs
// ONE exchange and the answer is an ATOMIC snapshot — field block, display
// flag and tag from one radio state. The cross-check's extra frame is paid
// only on the "?;" path (matrix §3.5).
func TestReadChannel_OnePopulatedReadIsOneFrame(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	before := len(p.Transcript())
	if _, err := sess.ReadChannel(testCtx(t), "001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if got := p.Transcript()[before:]; len(got) != 1 || got[0] != "MT001;" {
		t.Errorf("one populated ReadChannel sent %v, want exactly [\"MT001;\"]", got)
	}
}

// TestReadChannel_EmptySlotIsTheCrossChecksSecondRejection is the truth
// table's SECOND row: MT answers "?;", so ONE MR read of the same slot
// follows, and MR "?;" too means the slot is EMPTY — Data nil, the slot
// carried through, and no error.
//
// BOTH interpretations are ASSUMED and they are separate entries. That MR's
// "?;" means "empty" is the register's "?;" ON AN MR READ OF A MEMORY OR
// PMS SLOT MEANS THE SLOT IS EMPTY; that MT's "?;" is not itself sufficient
// is the design of the cross-check, which exists because the manual
// contradicts itself about whether MT can be read at all (matrix §3.12).
//
// The transcript is pinned because the SECOND frame is the whole point: a
// driver that read an MT "?;" as "empty" without asking MR would report an
// occupied channel as blank on a radio whose command list turns out to be
// the true record.
func TestReadChannel_EmptySlotIsTheCrossChecksSecondRejection(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	before := len(p.Transcript())
	ch, err := sess.ReadChannel(testCtx(t), "002")
	if err != nil {
		t.Fatalf("ReadChannel(\"002\") = %v, want nil (both rejections together are an empty slot, not a failure)", err)
	}
	if ch.Slot != "002" {
		t.Errorf("Channel.Slot = %q, want \"002\" — the slot must survive an empty read", ch.Slot)
	}
	if !ch.Empty() {
		t.Errorf("Channel.Empty() = false (Data = %+v), want an empty channel", ch.Data)
	}
	if got, want := p.Transcript()[before:], []string{"MT002;", "MR002;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the empty-slot path sent %v, want %v — ONE MR read follows an MT rejection, and exactly one", got, want)
	}
}

// TestReadChannel_MTRejectedOnAnOccupiedSlot is the truth table's THIRD row
// and the one the whole read design exists for: MT answers "?;" and the
// same slot's MR returns a RECORD, so the slot is occupied and MT refused
// it.
//
// The session read fails WHOLE — not per-slot — because a partial read that
// silently dropped occupied channels would be a codeplug the user could not
// tell from a complete one. The error NAMES the contradiction (the command
// list against the detail block) and the capture that would settle it, and
// DOES NOT DIAGNOSE it: "?;" carries no reason code, so three readings stay
// consistent with the manual and this project cannot tell them apart
// (matrix §3.8.3).
//
// It wraps cat.ErrRejected because that is what the radio actually said;
// errors.Is must find it, so a caller that handles rejections generically
// still can.
func TestReadChannel_MTRejectedOnAnOccupiedSlot(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	before := len(p.Transcript())
	_, err := sess.ReadChannel(testCtx(t), "006")
	if err == nil {
		t.Fatal("ReadChannel(\"006\") = nil error, want a refusal: MT rejected a slot MR reports as occupied")
	}
	var refusal *MTReadRejectedForOccupiedSlotError
	if !errors.As(err, &refusal) {
		t.Fatalf("error %v (%T) is not an *MTReadRejectedForOccupiedSlotError", err, err)
	}
	if !errors.Is(err, ErrMTReadRejectedForOccupiedSlot) {
		t.Error("errors.Is(err, ErrMTReadRejectedForOccupiedSlot) = false — the sentinel is what a caller compares against")
	}
	if !errors.Is(err, cat.ErrRejected) {
		t.Error("errors.Is(err, cat.ErrRejected) = false — the radio's answer WAS a rejection, and a caller handling rejections generically must still see one")
	}
	if refusal.Slot != "006" {
		t.Errorf("Slot = %q, want \"006\"", refusal.Slot)
	}
	text := refusal.Error()
	for _, want := range []string{"006", "layout 166", "1016", "MT read"} {
		if !strings.Contains(text, want) {
			t.Errorf("Error() = %q, want it to contain %q — the message must name the slot and the manual's two disagreeing records", text, want)
		}
	}

	if got, want := p.Transcript()[before:], []string{"MT006;", "MR006;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the refusal path sent %v, want %v", got, want)
	}
}

// TestReadChannel_MTTimeoutFailsWholeWithNoRetryAndNoMR is the truth
// table's FOURTH row, and the transcript is half the assertion: an MT read
// that TIMES OUT produces ONE MT frame and nothing else.
//
//   - NO RETRY (plan P11): the MT read's transport spec carries RetryReads
//     0, so a timeout is final. A retry would put a second MT frame on the
//     wire against a radio that may not answer MT reads at all.
//   - NO MR EITHER: the cross-check is the answer to a REJECTION, not to
//     silence. A timeout says nothing about whether the slot is occupied,
//     so asking MR would be interpreting a transport failure as a protocol
//     answer.
//   - The session read fails WHOLE, in the same typed family as the
//     rejection refusal, with the transport's own error wrapped so
//     errors.Is(err, transport.ErrTimeout) still matches.
func TestReadChannel_MTTimeoutFailsWholeWithNoRetryAndNoMR(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	before := len(p.Transcript())
	_, err := sess.ReadChannel(testCtx(t), "007")
	if err == nil {
		t.Fatal("ReadChannel(\"007\") = nil error, want a timeout refusal: the scripted radio answered nothing at all")
	}
	var timeout *MTReadTimeoutError
	if !errors.As(err, &timeout) {
		t.Fatalf("error %v (%T) is not an *MTReadTimeoutError", err, err)
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Error("errors.Is(err, transport.ErrTimeout) = false — the transport's own error must survive the wrap")
	}
	if timeout.Slot != "007" {
		t.Errorf("Slot = %q, want \"007\"", timeout.Slot)
	}

	if got, want := p.Transcript()[before:], []string{"MT007;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the timeout path sent %v, want exactly %v — ONE MT frame: no retry (P11), and no MR, because a cross-check answers a REJECTION and not silence", got, want)
	}
}

// TestReadChannel_DiscoveredBanksAreReadByMRAlone is the truth table's
// FIFTH row: a 5xx or EMG slot is read with ONE MR frame and never an MT
// one, and its Tag and TagDisplay come back Unavailable because MR's
// 28-position answer carries neither (matrix §2.5, §3.5).
//
// Unavailable, not Unknown: Unknown means "the radio has one and this read
// did not learn it", Unavailable means "this read cannot reach it". The
// capability half is readOnlyFields' zero FieldSupport for the pair, so
// every downstream path (Diff, the grid, csvio) already agrees.
//
// The negative half — that no MT frame is built for such a slot — is not
// merely a preference: MT's own slot legend prints memory and PMS only
// (layout 998-999), so the dialect and the outbound gate refuse the frame.
func TestReadChannel_DiscoveredBanksAreReadByMRAlone(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	for _, slot := range []string{"503", "EMG"} {
		t.Run(slot, func(t *testing.T) {
			before := len(p.Transcript())
			ch, err := sess.ReadChannel(testCtx(t), slot)
			if err != nil {
				t.Fatalf("ReadChannel(%q) = %v, want nil", slot, err)
			}
			if got, want := p.Transcript()[before:], []string{"MR" + slot + ";"}; !reflect.DeepEqual(got, want) {
				t.Errorf("reading %q sent %v, want exactly %v — MR alone, and never an MT read of a slot MT's legend does not name", slot, got, want)
			}
			if ch.Data == nil {
				t.Fatalf("ReadChannel(%q) returned an empty channel, want the populated record MR served", slot)
			}
			want := tierUnavailable(codeplug.ChannelData{
				FreqHz:    14_250_000,
				Mode:      "USB",
				ClarHz:    -150,
				RxClar:    true,
				CTCSS:     "ENC-DEC",
				CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
				Shift:     "PLUS",
				// MR carries no tag field at all, so there is no value to
				// report: the string stays empty and the flag says
				// Unavailable.
				Tag:        "",
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			})
			if !reflect.DeepEqual(*ch.Data, want) {
				t.Errorf("ChannelData =\n %+v\nwant\n %+v", *ch.Data, want)
			}
			drivertest.AssertFreshReadSaveLoad(t, ch, sess.Capabilities(), codeplug.Load)
		})
	}
}

// TestReadChannel_DiscoveredSlotThatStopsAnsweringIsARefusal is MEDIUM-1
// (task-1 review): a 5xx or EMG slot that answered a well-formed MR read
// during Open and rejects the IDENTICAL MR read at ReadChannel time gets a
// typed refusal, not an empty channel — matrix erratum M-E6, §3.8.4, and
// the driver register's A DISCOVERED SLOT KEEPS ANSWERING MR WITHIN A
// SESSION entry.
//
// RED-PROOF (recorded here, not re-run as part of this suite): run against
// the pre-fix readDiscovered — whose errors.Is(err, cat.ErrRejected) arm
// returned `codeplug.Channel{Slot: sl.Wire()}, nil` — this exact test body
// showed ReadChannel("503") returning {Slot:"503" Data:<nil>}, err=nil (an
// EMPTY channel, no error), and the ReadAll sub-test below returning a
// non-nil *codeplug.Codeplug carrying a blank "503" channel with nil error
// rather than failing. That is the silent-anomaly-deferred-to-Validate
// failure the review's own empirical check demonstrated: a fresh read
// followed by codeplug.Validate on that result fails with `slot "503" is
// part of NoBlank bank "60M" and must stay populated, but is empty` —
// blaming the codeplug for something the radio did.
func TestReadChannel_DiscoveredSlotThatStopsAnsweringIsARefusal(t *testing.T) {
	img := slotImage{mrAnswersOnce: map[string]string{"503": populatedMR("503")}}
	p, sess := openSession(t, Simulated, img)

	// Open's own discovery probe is the "first" MR read of 503 — confirm
	// it actually discovered the bank, or this test is not exercising the
	// scenario it claims to.
	caps := sess.Capabilities()
	sixty, ok := caps.Bank(spec.Bank60m)
	if !ok || !reflect.DeepEqual(sixty.Slots, []string{"503"}) {
		t.Fatalf("discovered 60M bank slots = %v (present=%v), want exactly [\"503\"] from Open's own probe", sixty.Slots, ok)
	}

	before := len(p.Transcript())
	_, err := sess.ReadChannel(testCtx(t), "503")
	if err == nil {
		t.Fatal("ReadChannel(\"503\") = nil error, want a refusal: this slot answered MR at Open and now rejects the identical read")
	}
	var refusal *MRReadRejectedForDiscoveredSlotError
	if !errors.As(err, &refusal) {
		t.Fatalf("error %v (%T) is not an *MRReadRejectedForDiscoveredSlotError", err, err)
	}
	if !errors.Is(err, ErrMRReadRejectedForDiscoveredSlot) {
		t.Error("errors.Is(err, ErrMRReadRejectedForDiscoveredSlot) = false — the sentinel is what a caller compares against")
	}
	if !errors.Is(err, cat.ErrRejected) {
		t.Error("errors.Is(err, cat.ErrRejected) = false — the radio's answer WAS a rejection, and a caller handling rejections generically must still see one")
	}
	if refusal.Slot != "503" {
		t.Errorf("Slot = %q, want \"503\"", refusal.Slot)
	}
	text := refusal.Error()
	for _, want := range []string{"503", "Open", "NoBlank", "DISCOVERED SLOT KEEPS ANSWERING"} {
		if !strings.Contains(text, want) {
			t.Errorf("Error() = %q, want it to contain %q", text, want)
		}
	}
	if got, want := p.Transcript()[before:], []string{"MR503;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the refusal path sent %v, want exactly %v — ONE MR frame, the second read of the same slot", got, want)
	}

	t.Run("ReadAll fails whole and returns no partial codeplug", func(t *testing.T) {
		service := clone.NewService(sess, clone.SnapshotStore{})
		cp, err := service.ReadAll(testCtx(t))
		if err == nil {
			t.Fatal("ReadAll = nil error, want the discovered-slot refusal to propagate")
		}
		if !errors.Is(err, ErrMRReadRejectedForDiscoveredSlot) {
			t.Errorf("ReadAll error %v does not wrap ErrMRReadRejectedForDiscoveredSlot", err)
		}
		if cp != nil {
			t.Errorf("ReadAll returned a non-nil codeplug (%+v) alongside its error, want no partial codeplug", cp)
		}
	})
}

// TestReadChannel_ErrorTyping covers the failure classes that must be TYPED
// and distinguishable, all via errors.As.
//
// A malformed or out-of-vocabulary ANSWER is the parser's verdict and stays
// a *cat.ParseError under this driver's wrap, so a caller can tell "the
// radio said something this protocol does not define" apart from "the radio
// answered about the wrong channel", which is this driver's own
// *AnswerMismatchError. Neither is a bare fmt.Errorf, and both carry the
// slot: the bare parser cannot know it, and an error naming no channel is
// nearly useless in a 99-slot read.
func TestReadChannel_ErrorTyping(t *testing.T) {
	_, sess := openSession(t, Simulated, readTestImage())

	t.Run("out-of-vocabulary kind byte is a wrapped cat.ParseError", func(t *testing.T) {
		_, err := sess.ReadChannel(testCtx(t), "003")
		if err == nil {
			t.Fatal("ReadChannel(\"003\") = nil error, want a refusal: the answer's P7 is '2', outside the combined record's tolerated {'0','1'} read pair")
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
		// "000" parses — the DIALECT register's ASSUMED SlotSpace.NoneWire
		// entry, that form appearing in NO FT-891 slot legend — but
		// BuildMTRead refuses it, and so would BuildMRRead.
		_, err := sess.ReadChannel(testCtx(t), "000")
		var pe *cat.ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("ReadChannel(\"000\") = %v (%T), want a wrapped *cat.ParseError from the builder", err, err)
		}
	})
}

// TestReadChannel_CrossCheckIsAtomicUnderOpMu is plan P3's concurrency pin:
// two goroutines racing ReadChannel cannot interleave an MT "?;" and the MR
// that interprets it.
//
// A memory or PMS read is potentially TWO exchanges here, and
// transport.Engine serialises each individual exchange rather than a pair
// (matrix §3.5's "Session mutex" note), so without opMu a concurrent
// operation can land in the gap and the cross-check ends up reasoning about
// a different radio state — reading "empty" from an MT rejection whose MR
// answer belonged to a slot somebody else had just changed.
//
// The race is forced deterministically through readChannelGapHook rather
// than by hammering: Go's sync.Mutex favours an immediately-re-locking
// goroutine so heavily that this specific interleaving is near-impossible to
// reproduce by scheduling luck, which is the same finding
// core/driver/ft710's own gap hook records.
func TestReadChannel_CrossCheckIsAtomicUnderOpMu(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())
	before := len(p.Transcript())

	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	readChannelGapHook = func() {
		once.Do(func() { close(reached) })
		<-release
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	firstDone := make(chan error, 1)
	go func() {
		_, err := sess.ReadChannel(testCtx(t), "002") // the cross-check path
		firstDone <- err
	}()

	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("ReadChannel never reached the gap between its MT rejection and its MR read within 5s")
	}

	secondDone := make(chan error, 1)
	go func() {
		_, err := sess.ReadChannel(testCtx(t), "001") // a plain one-frame read
		secondDone <- err
	}()

	// Give the concurrent read a generous, deterministic window to reach
	// the wire if nothing is holding it back.
	time.Sleep(500 * time.Millisecond)
	if got := p.Transcript()[before:]; len(got) != 1 || got[0] != "MT002;" {
		t.Errorf("while one ReadChannel was between its MT and MR, the wire carried %v — want only [\"MT002;\"]: a second operation must not interleave with a cross-check (P3)", got)
	}

	close(release)
	for _, done := range []chan error{firstDone, secondDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("ReadChannel: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("a ReadChannel never completed after the gap hook was released")
		}
	}

	if got, want := p.Transcript()[before:], []string{"MT002;", "MR002;", "MT001;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v — the cross-check's two frames must be adjacent", got, want)
	}
}

// TestMTSpec_DerivesItsLengthFromTheDialectAndNeverRetries pins two
// properties of the MT read's transport spec.
//
// THE LENGTH comes from the DIALECT's own geometry and not from a number in
// this package: there is no 41 in the production code, deliberately,
// because the combined answer's exactness is itself an assumption the
// dialect carries (its register entry THE COMBINED MT ANSWER'S EXACT
// LENGTH, 41) whose recorded Stage R contingency is a 30..41 WINDOW. If
// that contingency is ever taken the bounds move in core/cat and this spec
// moves with them — which it does only while the length is derived. The
// unconfigured-dialect case is the same guard from the other side: a zero
// dialect has no MT form, so it gets an error rather than a plausible zero
// length (which would admit any answer at all).
//
// RETRYREADS IS 0 (plan P11), and that is a decision, not an omission: a
// timeout on this radio's MT read is ONE frame and then MTReadTimeoutError.
// Every sibling's read spec carries one retry on the "a read is idempotent"
// reasoning, which is true here too — but on a radio whose command list says
// MT has no Read at all, a silent retry doubles the frames sent to test a
// premise that is already registered as an assumption.
func TestMTSpec_DerivesItsLengthFromTheDialectAndNeverRetries(t *testing.T) {
	lo, hi, err := catDialect.MTAnswerBounds()
	if err != nil {
		t.Fatalf("catDialect.MTAnswerBounds() = %v, want the combined form's exact bounds", err)
	}
	if lo != hi {
		t.Fatalf("MTAnswerBounds() = %d..%d, want equal bounds for the combined form", lo, hi)
	}
	// The chart's own arithmetic, asserted HERE where the manual is the
	// authority: "MT" + the 28-position field block + P11 + a 12-byte tag +
	// ';' = 41 (layout 996-1027).
	if hi != 41 {
		t.Errorf("the dialect's combined MT answer length = %d, want 41 per the manual's MT position chart", hi)
	}

	got, err := mtSpec(catDialect)
	if err != nil {
		t.Fatalf("mtSpec(catDialect) = %v, want nil", err)
	}
	if got.Class != transport.ClassRead {
		t.Errorf("Class = %v, want transport.ClassRead", got.Class)
	}
	// The prefix and the exact length, asserted THROUGH THE MATCHER rather
	// than off the struct: answer matching lives in an opaque
	// transport.CommandSpec.Match built by the codec.
	rightLength := "MT" + strings.Repeat("0", hi-3) + ";"
	oneShort := "MT" + strings.Repeat("0", hi-4) + ";"
	wrongCommand := "MR" + strings.Repeat("0", hi-3) + ";"
	if !got.Match([]byte(rightLength)) {
		t.Errorf("Match(%q) = false, want true — that is the dialect's own %d-byte combined MT answer", rightLength, hi)
	}
	if got.Match([]byte(oneShort)) {
		t.Errorf("Match(%q) = true, want false — the length is pinned to the dialect's %d, not merely to the prefix", oneShort, hi)
	}
	if got.Match([]byte(wrongCommand)) {
		t.Errorf("Match(%q) = true, want false — the prefix must discriminate the command", wrongCommand)
	}
	if got.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 (plan P11: an MT read that times out is ONE frame and then MTReadTimeoutError)", got.RetryReads)
	}

	if _, err := mtSpec(cat.Dialect{}); err == nil {
		t.Error("mtSpec(zero dialect) = nil error, want a refusal — an unconfigured dialect has no MT geometry, and a zero exact length would admit any MT answer")
	}
}

// TestMRSpec_MatchesTheAnswerChart pins the MR read's spec against the
// manual's own chart: prefix "MR", exactly 28 bytes.
//
// THE 28 IS WRITTEN DOWN in this package rather than derived, because
// core/cat exposes no accessor for the shared block's width — the FT-710's
// driver carries the same literal for the same reason. The authority is
// MR's Answer chart, which runs to 28 (layout 968-975), matching
// core/cat/memdata.go's field block position for position; the FT-891
// geometry witness counted it independently ("2 MR Answer frames (28
// bytes)").
//
// ONE RETRY, unlike the MT read's zero: nothing about MR is in doubt on
// this radio — its availability row is X O O X (layout 164) and no record
// contradicts it — so the ordinary idempotent-read reasoning applies. It is
// the MT read whose premise is registered as an assumption, and only that
// spec declines the retry.
func TestMRSpec_MatchesTheAnswerChart(t *testing.T) {
	got := mrSpec()
	if got.Class != transport.ClassRead {
		t.Errorf("Class = %v, want transport.ClassRead", got.Class)
	}
	rightLength := "MR" + strings.Repeat("0", 25) + ";"
	oneShort := "MR" + strings.Repeat("0", 24) + ";"
	wrongCommand := "MT" + strings.Repeat("0", 25) + ";"
	if !got.Match([]byte(rightLength)) {
		t.Errorf("Match(%q) = false, want true — the manual's MR answer chart runs to 28 (layout 968-975)", rightLength)
	}
	if got.Match([]byte(oneShort)) {
		t.Errorf("Match(%q) = true, want false", oneShort)
	}
	if got.Match([]byte(wrongCommand)) {
		t.Errorf("Match(%q) = true, want false — the prefix must discriminate the command", wrongCommand)
	}
	if got.RetryReads != 1 {
		t.Errorf("RetryReads = %d, want 1 (a read is idempotent, and nothing about MR is in doubt on this radio)", got.RetryReads)
	}
}

// tierUnavailable returns d with all SEVENTEEN fields the Icom model
// extensions added to codeplug.ChannelData set to Unavailable — what BOTH
// of this driver's read mappers report for all of them (plan P12), because
// this radio's memory frame carries none of them.
//
// The read tests' `want` literals name the fields this radio actually HAS
// and wrap the result in this, rather than spelling out seventeen
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
	d.TuningStepEnabled = codeplug.BoolField{State: codeplug.Unavailable}
	d.TuningStep = codeplug.StringField{State: codeplug.Unavailable}
	d.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.AttenuatorDB = codeplug.IntField{State: codeplug.Unavailable}
	d.Preamp = codeplug.StringField{State: codeplug.Unavailable}
	d.Antenna = codeplug.StringField{State: codeplug.Unavailable}
	d.IPPlus = codeplug.BoolField{State: codeplug.Unavailable}
	return d
}

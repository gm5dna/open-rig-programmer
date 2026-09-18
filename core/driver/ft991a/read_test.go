// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// readTestImage is the scripted radio the read tests share. Each slot is a
// case:
//
//	001 — a populated MEMORY channel: 145.5 MHz FM, a NEGATIVE clarifier
//	      with BOTH clarifier flags on, CTCSS ENC-DEC, PLUS shift, a
//	      seven-character tag in a twelve-byte field
//	117 — the last PMS slot, covering the other end of every vocabulary:
//	      the floor frequency, a POSITIVE clarifier at the declared
//	      ceiling with both flags off, CTCSS off, MINUS shift, a
//	      two-character tag
//	104 — a DCS-state channel, P8 '4': the vocabulary no registered
//	      sibling has, and the reason this radio's Stage 0 seams exist
//	002 — absent from the image, so answered "?;": the empty-slot path
//	003 — an answer carrying kind '2', structurally valid to the field
//	      parser and outside the combined record's own {'0','1'} read
//	      pair: the out-of-vocabulary path
//	004 — an answer that names slot 005: the slot-echo path
//	005 — silence: the timeout path, which no "?;" can stand in for
//
// One session serves all of them, which is safe because these are
// independent single exchanges with no state on either side.
func readTestImage() slotImage {
	return slotImage{
		mtAnswers: map[string]string{
			"001": memoryFields{
				slot: "001", freq: "145500000",
				clarSign: '-', clarMag: "0150", rxClar: '1', txClar: '1',
				mode: '4', kind: '1', ctcss: '1', shift: '1',
			}.mtFrame("CALLING"),
			"117": memoryFields{
				slot: "117", freq: "000030000",
				clarSign: '+', clarMag: "9990", rxClar: '0', txClar: '0',
				mode: 'E', kind: '0', ctcss: '0', shift: '2',
			}.mtFrame("AB"),
			"104": memoryFields{
				slot: "104", freq: "430025000",
				clarSign: '+', clarMag: "0000", rxClar: '0', txClar: '0',
				mode: 'B', kind: '1', ctcss: '4', shift: '0',
			}.mtFrame("DCSTEST"),
			"003": memoryFields{
				slot: "003", freq: "014250000",
				clarSign: '+', clarMag: "0000", rxClar: '0', txClar: '0',
				// '2' is cat.KindMemTune: inside the field parser's
				// documented read vocabulary, OUTSIDE the combined
				// record's own {'0' VFO, '1' Memory} pair, which THIS
				// radio's MT legend prints in full ("Read: 0: VFO
				// 1: Memory", layout 1009) rather than leaving to a
				// register entry as the FT-891 must.
				mode: '1', kind: '2', ctcss: '0', shift: '0',
			}.mtFrame("MEMTUNE"),
			"004": memoryFields{
				// The frame answers for a DIFFERENT slot than the read
				// asked about — the stale-reply shape the transport's
				// quarantine discipline is meant to prevent, checked
				// anyway because mapping an answer onto the wrong channel
				// would corrupt a codeplug silently.
				slot: "005", freq: "014250000",
				clarSign: '+', clarMag: "0000", rxClar: '0', txClar: '0',
				mode: '1', kind: '1', ctcss: '0', shift: '0',
			}.mtFrame("WRONGSLOT"),
		},
		mtSilent: map[string]bool{"005": true},
	}
}

// unavailableTierFields is the seventeen Icom-tier fields as EVERY read of
// this radio must answer them: Unavailable, never Absent and never Unknown.
//
// Unavailable is the positive statement "this frame has no such field",
// which is what a READ of this radio is entitled to say; Absent means
// "nobody has said anything", which a read has no business producing, and
// Unknown means "the radio has one and this read did not learn it", which
// would be false. drivertest.AssertFreshReadSaveLoad enforces seven of them
// plus the no-Absent rule fleet-wide; this literal is what pins the mapping
// value by value.
func unavailableTierFields() codeplug.ChannelData {
	return codeplug.ChannelData{
		TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:              codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
		ToneMode:            codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}
}

// TestReadChannel_MappingsFromThePositionChart drives ReadChannel against
// answers assembled BY POSITION from the manual's MT chart and pins the
// whole wire -> codeplug.ChannelData mapping. Every expected value is
// written out literally, never computed by the code under test.
//
// TWO CELLS HERE ARE THIS RADIO'S OWN, and each inverts the FT-891 exemplar
// (matrix erratum M-E3):
//
//   - TxClar comes back TRUE on slot 001, from byte 21. On the FT-891 that
//     byte is printed "(Fixed)" and TxClar can NEVER come back true; here
//     `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"` is printed on all five blocks
//     carrying the grid, so the parser carries the state through.
//   - TagDisplay comes back UNAVAILABLE, not Known. On the FT-891 P11 is a
//     live TAG flag and its read reports it Known; here it is printed
//     "0: (Fixed)" (layout 1015), so there is no value to report and
//     "Unknown" — which means "the radio has one and this read did not
//     learn it" — would be a different and false claim.
//
// CTCSSTone and ScanSkip are Unknown: the register's TONE-NUMBER
// UNREACHABILITY and SCAN-SKIP UNREACHABILITY entries respectively, nothing
// readable under either. They are two entries and not one because their
// lifting captures are two (matrix erratum M-E10).
func TestReadChannel_MappingsFromThePositionChart(t *testing.T) {
	_, sess := openSession(t, Simulated, readTestImage())

	for _, tt := range []struct {
		name string
		slot string
		want codeplug.ChannelData
	}{
		{
			name: "a populated MEMORY channel",
			slot: "001",
			want: codeplug.ChannelData{
				FreqHz: 145_500_000,
				// Mode '4' is FM in THIS radio's legend. Rendered through
				// the session's own dialect, never cat.Mode.String.
				Mode:       "FM",
				ClarHz:     -150,
				RxClar:     true,
				TxClar:     true,
				CTCSS:      "ENC-DEC",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "PLUS",
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
				Tag:        "CALLING",
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			},
		},
		{
			name: "the last PMS slot, at the other end of every vocabulary",
			slot: "117",
			want: codeplug.ChannelData{
				FreqHz: 30_000,
				// 'E' is C4FM HERE and PSK on the FTdx10 — one nibble, two
				// REAL and different modes, which is why nothing may render
				// through core/cat's package-level fallback (matrix §1.5).
				Mode:       "C4FM",
				ClarHz:     9990,
				RxClar:     false,
				TxClar:     false,
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "MINUS",
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
				Tag:        "AB",
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			},
		},
		{
			name: "a DCS-state channel, the vocabulary no sibling has",
			slot: "104",
			want: codeplug.ChannelData{
				FreqHz:     430_025_000,
				Mode:       "FM-N",
				ClarHz:     0,
				RxClar:     false,
				TxClar:     false,
				CTCSS:      "DCS-ENC",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
				Tag:        "DCSTEST",
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
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
				t.Fatal("Channel.Data = nil, want a populated channel")
			}

			want := tt.want
			tier := unavailableTierFields()
			want.TxFreqHz = tier.TxFreqHz
			want.Duplex = tier.Duplex
			want.OffsetHz = tier.OffsetHz
			want.ToneMode = tier.ToneMode
			want.ToneTx = tier.ToneTx
			want.ToneRx = tier.ToneRx
			want.DTCSCode = tier.DTCSCode
			want.DTCSPolarity = tier.DTCSPolarity
			want.Filter = tier.Filter
			want.DataMode = tier.DataMode
			want.TuningStepEnabled = tier.TuningStepEnabled
			want.TuningStep = tier.TuningStep
			want.ProgramTuningStepHz = tier.ProgramTuningStepHz
			want.AttenuatorDB = tier.AttenuatorDB
			want.Preamp = tier.Preamp
			want.Antenna = tier.Antenna
			want.IPPlus = tier.IPPlus

			if !reflect.DeepEqual(*ch.Data, want) {
				t.Errorf("ChannelData =\n%+v\nwant\n%+v", *ch.Data, want)
			}

			// The fleet's own no-Absent rule and the D8 fresh-read
			// contract, on this driver's single read path.
			drivertest.AssertFreshReadSaveLoad(t, ch, sess.Capabilities(), codeplug.Load)
		})
	}
}

// TestReadChannel_FiveStateP8IsMappedEndToEnd walks EVERY wire byte of this
// radio's P8 vocabulary through the read path and pins the display string
// each one produces (matrix §1.17, §3.7).
//
// The last two are the point: no registered sibling's P8 legend prints a
// value above '2', and a driver that reused the family's three-entry map
// would fail these two rows with an "unmapped CTCSS state" refusal rather
// than silently mislabelling — which is the right direction, and still a
// failure the fleet has never had a test for until this radio.
//
// The strings are the ones caps.go advertises and Stage 0's own tests fix
// across four packages; a read that produced any other spelling would put a
// value in a codeplug that codeplug.Validate refuses.
func TestReadChannel_FiveStateP8IsMappedEndToEnd(t *testing.T) {
	answers := map[string]string{}
	for _, tt := range []struct {
		slot string
		wire byte
	}{
		{"010", '0'}, {"011", '1'}, {"012", '2'}, {"013", '3'}, {"014", '4'},
	} {
		answers[tt.slot] = memoryFields{
			slot: tt.slot, freq: "145500000",
			clarSign: '+', clarMag: "0000", rxClar: '0', txClar: '0',
			mode: '4', kind: '1', ctcss: tt.wire, shift: '0',
		}.mtFrame("P8")
	}
	_, sess := openSession(t, Simulated, slotImage{mtAnswers: answers})

	caps := sess.Capabilities()
	for i, tt := range []struct {
		slot string
		want string
	}{
		{"010", "OFF"},
		{"011", "ENC-DEC"},
		{"012", "ENC"},
		{"013", "DCS-ENC-DEC"},
		{"014", "DCS-ENC"},
	} {
		ch, err := sess.ReadChannel(testCtx(t), tt.slot)
		if err != nil {
			t.Fatalf("ReadChannel(%q) = %v, want nil — P8 wire byte %q is one of this radio's five (layout 1010-1011)", tt.slot, err, '0'+byte(i))
		}
		if ch.Data.CTCSS != tt.want {
			t.Errorf("P8 %q read back as %q, want %q", '0'+byte(i), ch.Data.CTCSS, tt.want)
		}
		if caps.ToneModes[i].Value != tt.want {
			t.Errorf("caps.ToneModes[%d] = %q, but the read maps that wire byte to %q — the driver's map and its published vocabulary must be the same five", i, caps.ToneModes[i].Value, tt.want)
		}
	}
}

// TestCTCSSNames_CoverExactlyTheAdvertisedVocabulary is the drift guard
// between read.go's map and caps.go's published list: same size, same
// strings. The FT-891's write_test.go carries the same shape for its own
// three; here it is five, and the two DCS members are the ones a copy from a
// sibling would silently drop.
func TestCTCSSNames_CoverExactlyTheAdvertisedVocabulary(t *testing.T) {
	caps := CapabilitiesUnverified()
	if len(ctcssNames) != len(caps.ToneModes) {
		t.Fatalf("read.go's ctcssNames has %d entries, Capabilities advertises %d states — a state the radio can send and this driver cannot name fails every read of that channel", len(ctcssNames), len(caps.ToneModes))
	}
	published := map[string]bool{}
	for _, st := range caps.ToneModes {
		published[st.Value] = true
	}
	for wire, name := range ctcssNames {
		if !published[name] {
			t.Errorf("ctcssNames[%q] = %q, which Capabilities.ToneModes does not advertise", wire, name)
		}
	}
}

// TestReadChannel_EmptySlotIsNotAnError: a "?;" is an EMPTY channel, not a
// failure — Data nil, the slot carried through, no error, ONE frame.
//
// ASSUMED, and it is the register's MT "?;" ON A MEMORY OR PMS SLOT MEANS
// THE SLOT IS EMPTY entry: "?;" is the protocol's single unattributed NAK,
// so reading "empty" out of it is an interpretation. THIS DRIVER MAKES
// EXACTLY ONE SUCH INTERPRETATION AND HAS EXACTLY ONE SITE FOR IT, where the
// FT-891 has four — the benefit of having no discovery walk and no
// cross-check.
//
// THERE IS NO CROSS-CHECK HERE AND STAGE 2 MUST NOT INVENT ONE. The FT-891
// answers a "?;" with an MR read because ITS manual contradicts itself about
// whether MT can be read at all (ft891_layout.txt:166 against :1016); this
// manual's availability row (layout 181) and its MT block (998-1033) agree,
// so there is no contradiction for a second frame to resolve. The frame
// count is asserted for that reason and not merely for tidiness.
func TestReadChannel_EmptySlotIsNotAnError(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	before := len(p.Transcript())
	ch, err := sess.ReadChannel(testCtx(t), "002")
	if err != nil {
		t.Fatalf("ReadChannel(\"002\") = %v, want nil for an empty slot", err)
	}
	if ch.Slot != "002" || ch.Data != nil {
		t.Errorf("empty read = %+v, want {Slot:\"002\" Data:nil}", ch)
	}
	if got := p.Transcript()[before:]; !reflect.DeepEqual(got, []string{"MT002;"}) {
		t.Errorf("an empty read sent %v, want exactly [\"MT002;\"] — a \"?;\" costs NO second frame on this radio (matrix §3.5)", got)
	}
}

// TestReadChannel_TimeoutIsTheTransportsOwnErrorAndOneFrame pins the
// timeout row of the truth table, and both halves are decisions the plan and
// the spec make in terms.
//
// EXACTLY ONE MT FRAME. mtSpec declares RetryReads 0 (see its own doc
// comment), so a timeout is one frame and then a failure — never two.
// transport.CATReadSpec takes retryReads as a parameter, so this radio's
// spec must CHOOSE, and the choice matters MORE here than on the FT-891, not
// less: with no MR to fall back on, a retried timeout cannot be told from a
// slow radio.
//
// NO SECOND FRAME OF ANY KIND FOLLOWS IT EITHER — no MR, because a
// cross-check answers a REJECTION and not silence, and there is no
// cross-check on this radio at all.
//
// THE ERROR IS THE TRANSPORT'S OWN AND IS NOT RE-TYPED (plan §Plan-vs-spec
// ruling 5, spec §Error handling: "no timeout branch of that family"). There
// is deliberately no MTReadTimeoutError analogue and no timeout arm in
// read.go's error handling at all: the transport's error reaches the caller
// through the same wrap every other transport failure on this path gets, so
// errors.Is finds transport.ErrTimeout and errors.As finds no driver type
// standing between them. The FT-891's typed timeout exists for a reason this
// radio does not supply — that its MT read's very availability is in
// question.
func TestReadChannel_TimeoutIsTheTransportsOwnErrorAndOneFrame(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	before := len(p.Transcript())
	_, err := sess.ReadChannel(testCtx(t), "005")
	if !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("ReadChannel(\"005\") = %v, want errors.Is match against transport.ErrTimeout", err)
	}
	if !strings.Contains(err.Error(), "005") {
		t.Errorf("error text %q does not name slot \"005\" — the wrap is what adds the slot context the transport cannot know", err.Error())
	}
	if got := p.Transcript()[before:]; !reflect.DeepEqual(got, []string{"MT005;"}) {
		t.Errorf("a timed-out read sent %v, want exactly [\"MT005;\"] — RetryReads is 0 and no MR follows a silence (matrix §3.5, plan task 10)", got)
	}
}

// TestReadChannel_SendsExactlyOneMTAndNeverMR is the MT-ONLY decision's
// enforcement on the wire, for both banks.
//
// One read is ONE frame: that is what makes the answer an ATOMIC snapshot of
// the channel — the two-frame stitch the FT-710's MR+MT read has to guard
// against is structurally impossible here rather than merely locked against.
// And no MR frame is sent by this driver at ANY point, not by a read and not
// by Open, which probes nothing. A "completion" of the read path that added
// the FT-891's cross-check would make doc.go's statement false while every
// other test kept passing; this is what catches it.
func TestReadChannel_SendsExactlyOneMTAndNeverMR(t *testing.T) {
	p, sess := openSession(t, Simulated, readTestImage())

	for _, slot := range []string{"001", "117"} {
		before := len(p.Transcript())
		if _, err := sess.ReadChannel(testCtx(t), slot); err != nil {
			t.Fatalf("ReadChannel(%q): %v", slot, err)
		}
		if got, want := p.Transcript()[before:], []string{"MT" + slot + ";"}; !reflect.DeepEqual(got, want) {
			t.Errorf("one ReadChannel(%q) sent %v, want exactly %v", slot, got, want)
		}
	}
	for i, frame := range p.Transcript() {
		if strings.HasPrefix(frame, "MR") {
			t.Errorf("frame %d = %q: this driver must never send MR — the read path is MT-only and Open probes nothing (doc.go, matrix §3.5)", i, frame)
		}
	}
}

// TestReadChannel_ErrorTyping: every failure the read path can produce is a
// TYPED error, never a bare fmt.Errorf, and each carries the slot — the bare
// parser cannot know it, and an error naming no channel is nearly useless in
// a 117-slot read.
func TestReadChannel_ErrorTyping(t *testing.T) {
	_, sess := openSession(t, Simulated, readTestImage())

	t.Run("out-of-vocabulary kind byte is a wrapped cat.ParseError", func(t *testing.T) {
		_, err := sess.ReadChannel(testCtx(t), "003")
		if err == nil {
			t.Fatal("ReadChannel(\"003\") = nil error, want a refusal: the answer's P7 is '2', outside the read pair MT's own legend prints (\"Read: 0: VFO 1: Memory\", layout 1009)")
		}
		var pe *cat.ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("error %v (%T) is not a wrapped *cat.ParseError — the kind vocabulary is the PARSER's to enforce, and its typed verdict must survive this driver's wrap", err, err)
		}
		if !strings.Contains(err.Error(), "003") {
			t.Errorf("error text %q does not name slot \"003\"", err.Error())
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

	t.Run("the sibling PMS token form is a NON-SLOT here", func(t *testing.T) {
		// "P1L" is a perfectly good PMS slot on every registered sibling
		// and NOT A SLOT AT ALL on this radio: its PMS pairs are decimal
		// channel numbers 100-117 (layout 916), so the token form never
		// reaches the wire and ParseSlot refuses it. This is the read-side
		// half of the hazard caps_test.go pins on the capability side —
		// spec Stage 2 item 7's PMS-by-string sweep.
		_, err := sess.ReadChannel(testCtx(t), "P1L")
		var pe *cat.ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("ReadChannel(\"P1L\") = %v (%T), want a wrapped *cat.ParseError — the token form is a non-slot on this radio", err, err)
		}
	})

	t.Run("the none form is grammatical but never a read target", func(t *testing.T) {
		// "000" parses (the DIALECT register's ASSUMED SlotSpace.NoneWire =
		// "000" entry — it appears in no FT-991A slot legend and is
		// supplied because cat.SlotSpace structurally requires a none form)
		// but BuildMTRead refuses it: it is the wire form of "no slot", not
		// a channel anything can be read from.
		_, err := sess.ReadChannel(testCtx(t), "000")
		var pe *cat.ParseError
		if !errors.As(err, &pe) {
			t.Fatalf("ReadChannel(\"000\") = %v (%T), want a wrapped *cat.ParseError from BuildMTRead", err, err)
		}
	})
}

// TestMTSpec_DerivesItsLengthFromTheDialectAndNeverRetries pins both halves
// of the read spec.
//
// THE LENGTH COMES FROM THE DIALECT and there is no 41 in this package's
// production code, deliberately: the combined answer's exactness is itself
// an assumption the dialect carries (its register entry THE COMBINED MT
// ANSWER'S EXACT LENGTH, 41), whose recorded Stage R contingency is a 30..41
// WINDOW. If that contingency is ever taken the bounds move in core/cat and
// this spec moves with them — which happens only while the length is
// derived.
//
// RETRYREADS IS 0, AND THAT IS THE DECISION plan task 10 asks for by name.
// The ID probe next door takes one retry on the ordinary reasoning (a read
// is idempotent, so a single swallowed reply should not fail an operation),
// and this spec declines it because a retried MT timeout cannot be told from
// a slow radio when there is no second frame to disambiguate it — and
// because a silently doubled transcript is a worse failure than a clean
// timeout. The FT-891 reaches the same value from a different premise
// (whether its MT read exists at all), which is why this is asserted here
// rather than inherited.
func TestMTSpec_DerivesItsLengthFromTheDialectAndNeverRetries(t *testing.T) {
	lo, hi, err := catDialect.MTAnswerBounds()
	if err != nil {
		t.Fatalf("catDialect.MTAnswerBounds() = %v, want the combined form's exact bounds", err)
	}
	if lo != hi {
		t.Fatalf("MTAnswerBounds() = %d..%d, want equal bounds for the combined form", lo, hi)
	}
	// The chart's own arithmetic, asserted HERE where the manual is the
	// authority: "MT" + the 28-position field block + P11 + a 12-byte tag
	// + ';' = 41 (the MT block, layout 998-1033; evidence leg G counted it
	// twice at 600 dpi).
	if hi != mtAnswerLen {
		t.Errorf("the dialect's combined MT answer length = %d, want %d per the manual's MT position chart", hi, mtAnswerLen)
	}

	sp, err := mtSpec(catDialect)
	if err != nil {
		t.Fatalf("mtSpec(catDialect) = %v, want nil", err)
	}
	if sp.Class != transport.ClassRead {
		t.Errorf("Class = %v, want transport.ClassRead", sp.Class)
	}
	if sp.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 — a retried MT timeout cannot be told from a slow radio on a driver with no second frame (matrix §3.5, plan task 10)", sp.RetryReads)
	}

	// The prefix and the exact length, asserted THROUGH THE MATCHER rather
	// than off the struct: answer matching is an opaque
	// transport.CommandSpec.Match built by the codec.
	rightLength := "MT" + strings.Repeat("0", hi-3) + ";"
	oneShort := "MT" + strings.Repeat("0", hi-4) + ";"
	wrongCommand := "MR" + strings.Repeat("0", hi-3) + ";"
	if !sp.Match([]byte(rightLength)) {
		t.Errorf("Match(%q) = false, want true — that is the dialect's own %d-byte combined MT answer", rightLength, hi)
	}
	if sp.Match([]byte(oneShort)) {
		t.Errorf("Match(%q) = true, want false — the length is pinned to the dialect's %d, not merely to the prefix", oneShort, hi)
	}
	if sp.Match([]byte(wrongCommand)) {
		t.Errorf("Match(%q) = true, want false — another command's answer of the right length must not match", wrongCommand)
	}

	// The unconfigured-dialect case is the same guard from the other side:
	// a zero dialect has no MT form, so it gets an error rather than a
	// plausible zero length (which would admit any answer at all).
	if _, err := mtSpec(cat.Dialect{}); err == nil {
		t.Error("mtSpec(zero dialect) = nil error, want a refusal — a zero length would match any answer")
	}
}

// TestReadChannel_ConcurrentReadsDoNotCrossAnswers is the concurrency pin
// plan P12 asks for, stated as what it can honestly assert on this radio.
//
// ReadChannel is ONE exchange here, and transport.Engine already serialises
// each individual exchange — so unlike the FT-891, where a concurrent
// operation could land between the MT rejection and the MR that interprets
// it, there is no gap inside this operation for opMu to close. What opMu is
// for is the operations this session gains later (a write, a settings read),
// which must not interleave their frames with each other; what this test can
// check today is the property a user would actually lose if the lock or the
// engine's serialisation broke — that racing reads never return each other's
// answers.
//
// Run under -race it also covers the Session's own fields.
func TestReadChannel_ConcurrentReadsDoNotCrossAnswers(t *testing.T) {
	_, sess := openSession(t, Simulated, readTestImage())

	slots := []string{"001", "117", "104", "001", "117", "104"}
	got := make([]codeplug.Channel, len(slots))
	errs := make([]error, len(slots))
	var wg sync.WaitGroup
	for i, slot := range slots {
		wg.Add(1)
		go func(i int, slot string) {
			defer wg.Done()
			got[i], errs[i] = sess.ReadChannel(testCtx(t), slot)
		}(i, slot)
	}
	wg.Wait()

	for i, slot := range slots {
		if errs[i] != nil {
			t.Fatalf("concurrent ReadChannel(%q) = %v, want nil", slot, errs[i])
		}
		if got[i].Slot != slot || got[i].Data == nil {
			t.Fatalf("concurrent ReadChannel(%q) returned %+v — a read must never be answered with another read's channel", slot, got[i])
		}
	}
	// The three distinct slots carry three distinct frequencies, so a
	// crossed answer shows up as a value rather than only as a slot string.
	byslot := map[string]uint64{"001": 145_500_000, "117": 30_000, "104": 430_025_000}
	for i, slot := range slots {
		if got[i].Data.FreqHz != byslot[slot] {
			t.Errorf("concurrent ReadChannel(%q) returned %d Hz, want %d — the answers have been crossed", slot, got[i].Data.FreqHz, byslot[slot])
		}
	}
}

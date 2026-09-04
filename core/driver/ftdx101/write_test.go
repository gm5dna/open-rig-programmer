// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"errors"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// writableChannel returns the ORDINARY FTdx101 channel: a populated slot
// whose three FieldState-carrying fields hold exactly what this driver's own
// read path produces for them (read.go) — TagDisplay Unavailable, tone and
// scan skip Unknown — and whose plain fields hold codec-expressible values.
//
// TagDisplay UNAVAILABLE is the point, and it is why this fixture cannot be
// the FT-710's: over there the equivalent helper carries a Known-true display
// flag, because that radio's MT frame has one and a non-Known value is
// refused outright. Here Unavailable is what EVERY channel legitimately
// carries, so a fixture with a Known display value would exercise only the
// error path and never the ordinary write.
//
// ITS VALUES ARE DELIBERATELY NOT THE FTDX10 FIXTURE'S. That package's
// combined-form write test uses 14.250 MHz USB, clarifier -150, tag
// "CALLING"; this one uses different values in every position for which the
// two manuals could in principle disagree, so that the frame literal below is
// visibly derived from THIS radio's charts rather than copied from a sibling
// package whose bytes happen to coincide. The values also each buy something:
//
//   - freq 7_074_000 exercises P2's LEADING ZEROS (two of the nine digits);
//   - a NEGATIVE clarifier exercises P3's sign position, whose byte is the
//     DIALECT register's "CLARIFIER'S MINUS-DIRECTION BYTE" entry (matrix
//     §2.1: this manual prints the glyph unreadably and the golden deriver
//     declined to guess), and 1230 exercises all four magnitude digits;
//   - RxClar false / TxClar true is the OPPOSITE pairing to the read
//     fixture's, so P4 and P5 transposed would show;
//   - mode "DATA-U" is wire 'C', a HEX-LETTER member of this manual's 1-F
//     legend, where a digit mode would leave the letter half untested;
//   - CTCSS "ENC" ('2') and shift "MINUS" ('2') are the highest member of
//     each three-value legend;
//   - tag "FT8 CALL" carries an INTERIOR SPACE, which is the dialect's own
//     TagFill byte: it must survive into the frame while the TRAILING fill is
//     padding, which is the distinction core/cat's tag round-trip rule rests
//     on. It is 8 characters, so 4 bytes of padding remain visible.
func writableChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz:     7_074_000,
			Mode:       "DATA-U",
			ClarHz:     -1230,
			RxClar:     false,
			TxClar:     true,
			CTCSS:      "ENC",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      "MINUS",
			Tag:        "FT8 CALL",
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		},
	}
}

// wantSetFrame is THE 41-byte combined MT Set that writableChannel("042")
// must put on the wire, HAND-DERIVED FROM THE POSITION CHART and written out
// as a literal.
//
// DERIVED, NEVER COMPUTED: the expectation is NOT built by calling
// cat.Dialect.BuildMTSetCombined, because a reference produced by the code
// under test agrees with a wrong offset exactly as happily as with a right
// one — the same reasoning respondingport_test.go's mtAnswerFields.frame
// records for the READ direction, and the reason that helper exists rather
// than the builder.
//
// The chart is THIS radio's MT position table, rev 2308-L (layout 1311-1330,
// printed pages 16-17), whose 41 positions were additionally counted off 300
// dpi raster renders into core/cat/ftdx101/testdata/geometry-witness.csv —
// its `MT,set,*` rows are the column boundaries below, field by field. The
// legends that decide the VALUE bytes are this manual's own: P6's mode legend
// (1321-1323), P7's "Set: 0: (Fixed)" (1324), P8's CTCSS legend (1325), P9's
// documented fixed "00" (1326), P10's shift legend (1327), P11's "0: (Fixed)"
// (1329) and P12's 12-byte tag field (1330).
//
// Position by position, for writableChannel's values:
//
//	pos 1-2    "MT"           the command
//	pos 3-5    "042"          P1 slot
//	pos 6-14   "007074000"    P2 frequency, 7_074_000 Hz as 9 zero-padded digits
//	pos 15     "-"            P3 sign: ClarHz -1230 is negative. THE BYTE IS
//	                          ASSUMED — the DIALECT register's "CLARIFIER'S
//	                          MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS
//	                          0x2D" entry; this position's WIDTH is
//	                          manual-evidenced, its content is not (matrix §2.1)
//	pos 16-19  "1230"         P3 magnitude, |-1230| as 4 zero-padded digits
//	pos 20     "0"            P4 RX clarifier off
//	pos 21     "1"            P5 TX clarifier on
//	pos 22     "C"            P6 mode: this manual's legend "C: DATA-U"
//	pos 23     "0"            P7 kind: cat.CombinedMTSetKind, "Set: 0: (Fixed)"
//	pos 24     "2"            P8 CTCSS state: "2: CTCSS ENC" = codeplug "ENC"
//	pos 25-26  "00"           P9, documented fixed
//	pos 27     "2"            P10 shift: "2: Minus Shift" = codeplug "MINUS"
//	pos 28     "0"            P11, documented fixed — THE POSITION WHERE THE
//	                          FT-710 KEEPS ITS DISPLAY FLAG AND THIS RADIO
//	                          KEEPS A CONSTANT (matrix §3.7, the named
//	                          inversion's evidence)
//	pos 29-40  "FT8 CALL    " P12 tag, 8 characters padded to 12 with the
//	                          dialect's fill (the DIALECT register's
//	                          "MTPolicy.TagFill = ' '" entry — ASSUMED)
//	pos 41     ";"            terminator
//
// 41 bytes, and the literal 41 belongs in this file for the same reason it
// must not appear in the production code: here it is the CHART being
// asserted, there it would be a length the driver believed instead of one the
// dialect derived (see read.go's mtSpec).
//
// ONE literal for BOTH MODELS, and that is itself an assertion the pin makes
// (see TestWriteChannel_LiteralFramePin): matrix §2.5 records that the MT
// block and every one of its P-legends is printed once with no model
// qualifier, so a D frame and an MP frame for the same channel must be the
// same bytes. The two dialects differ in the ID answer and in nothing else.
const wantSetFrame = "MT042007074000-123001C020020FT8 CALL    ;"

// wantOneStep builds the expected ONE-step result: this radio's whole write
// choreography is a single MT frame, so every non-refused outcome differs
// only in the two flags. Every step assertion in this file goes through it,
// so the mnemonic and the LENGTH are asserted alongside the flags rather than
// being respelt (and possibly mis-spelt) at each site.
func wantOneStep(sent, confirmed bool) []driver.WriteStep {
	return []driver.WriteStep{{Command: "MT", Sent: sent, Confirmed: confirmed}}
}

// assertSteps fails t unless res.Steps is exactly want, element for element.
func assertSteps(t *testing.T, res driver.WriteResult, want []driver.WriteStep) {
	t.Helper()
	if !slices.Equal(res.Steps, want) {
		t.Errorf("WriteResult.Steps = %+v, want %+v", res.Steps, want)
	}
}

// assertEmptyNonNilSteps fails t unless res.Steps is an EXPLICITLY EMPTY,
// non-nil slice — the shape core/driver documents for a refusal that happens
// before any frame is built.
//
// The nil check is separate from the length check because len() cannot tell
// the two apart and the difference is durable and user-visible: the clone
// service marshals this list into an append-only audit file, where nil
// renders as `null` — which an auditor must read as "unknown", where the
// truth is the far stronger "no frame was ever built, so nothing whatever was
// attempted".
func assertEmptyNonNilSteps(t *testing.T, res driver.WriteResult) {
	t.Helper()
	if res.Steps == nil {
		t.Fatal("WriteResult.Steps is nil, want an explicitly EMPTY slice (nil marshals as JSON null in the clone journal)")
	}
	if len(res.Steps) != 0 {
		t.Errorf("WriteResult.Steps = %+v, want empty — no frame was built, so no step was intended", res.Steps)
	}
}

// refusedFields runs sess.WriteChannel(ch), requires a typed
// *driver.WriteRefusedError with an empty non-nil step list, and returns it.
func refusedFields(t *testing.T, sess *Session, ch codeplug.Channel) *driver.WriteRefusedError {
	t.Helper()
	res, err := sess.WriteChannel(testCtx(t), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel = %v, want errors.Is match against driver.ErrWriteRefused", err)
	}
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel error %v (%T) is not a *driver.WriteRefusedError", err, err)
	}
	assertEmptyNonNilSteps(t, res)
	return wre
}

// failableWritePort wraps a port and, once armed, fails every Write with a
// transport-level error: the outcome the write path CANNOT attribute, since
// the host has no way to tell whether the frame reached the radio.
//
// Arming happens after Open, deliberately — it keeps the fixture independent
// of how many exchanges Open's probe-and-discovery choreography spends, which
// on this driver is about a hundred per model.
type failableWritePort struct {
	inner io.ReadWriteCloser
	armed atomic.Bool
}

func (p *failableWritePort) Read(b []byte) (int, error) { return p.inner.Read(b) }

func (p *failableWritePort) Write(b []byte) (int, error) {
	if p.armed.Load() {
		return 0, errors.New("failableWritePort: injected transport-level write failure")
	}
	return p.inner.Write(b)
}

func (p *failableWritePort) Close() error { return p.inner.Close() }

// TestWriteChannel_LiteralFramePin is this task's flagship: one write of a
// fully-populated channel puts EXACTLY the hand-derived 41-byte Set on the
// wire, exactly ONE frame, and the SAME frame for both models.
//
// Asserted against the scripted port's own transcript rather than against the
// returned Command, so what is pinned is the bytes that crossed the wire —
// through the driver's real transport.Engine and its outbound write gate,
// both of which sit between buildWriteCommand and the port and either of
// which could in principle alter or refuse the frame.
//
// The one-frame assertion is the MT-ONLY decision's enforcement in the write
// direction, the exact counterpart of the read path's own single-MT pin: no
// MW is sent, by this write or by anything else in the session's life, so a
// future "completion" of the write path that added the FT-710's MW exchange
// fails here rather than quietly making doc.go and matrix §3.6 false.
//
// THE CROSS-MODEL LEG IS NOT DECORATION. This package's whole claim is that
// the D and the MP differ in a name and a CAT ID and in nothing else (matrix
// §4), and the memory-channel write surface is where that claim would be
// most expensive to get wrong. Comparing the two models' actual wire bytes to
// each other — not merely each to the literal — is what turns "identical" from
// a statement into a test.
func TestWriteChannel_LiteralFramePin(t *testing.T) {
	if len(wantSetFrame) != 41 {
		t.Fatalf("the hand-derived frame is %d bytes, want 41 per the manual's MT position chart — the literal is wrong before the driver is even asked", len(wantSetFrame))
	}

	perModel := make(map[string]string, len(testModels))

	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p, sess := openSession(t, m, Simulated, slotImage{})

			before := len(p.Transcript())
			res, err := sess.WriteChannel(testCtx(t), writableChannel("042"))
			if err != nil {
				t.Fatalf("WriteChannel = %v, want nil", err)
			}
			assertSteps(t, res, wantOneStep(true, true))

			after := p.Transcript()
			got := after[before:]
			if len(got) != 1 {
				t.Fatalf("one WriteChannel sent %d frames (%q), want exactly 1 — the FTdx101's write choreography is ONE combined MT Set", len(got), got)
			}
			if got[0] != wantSetFrame {
				t.Errorf("Set frame =\n %q\nwant\n %q", got[0], wantSetFrame)
			}
			perModel[m.name] = got[0]

			for i, frame := range after {
				if strings.HasPrefix(frame, "MW") {
					t.Errorf("frame %d = %q: this driver must never send MW — the write path is MT-only (see doc.go and matrix §3.6)", i, frame)
				}
			}
		})
	}

	if len(perModel) == len(testModels) {
		d, mp := perModel[testModels[0].name], perModel[testModels[1].name]
		if d != mp {
			t.Errorf("the %s wrote %q and the %s wrote %q — the two models' combined MT Sets must be BYTE-IDENTICAL: this manual prints the MT block and every P-legend once, with no model qualifier (matrix §2.5, §4)", testModels[0].name, d, testModels[1].name, mp)
		}
	}
}

// withKnownTone and withKnownSkip mark the two protocol-unreachable fields
// Known, which is what makes the capability gate refuse them (DRIVER register
// entry 6: the combined record carries no tone NUMBER and no skip flag).
func withKnownTone(ch codeplug.Channel) codeplug.Channel {
	ch.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}
	return ch
}

func withKnownSkip(ch codeplug.Channel) codeplug.Channel {
	ch.Data.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
	return ch
}

// TestWriteChannel_RefusalLadder walks every rung of the ladder IN ORDER, for
// BOTH models, and every case here asserts the ORDER rather than merely the
// existence of a refusal: each fixture is wrong in the rung under test AND in
// at least one LATER rung, so a refusal naming the later problem would mean
// the ladder had been reordered. A one-sided test cannot tell an ordering
// from a swallow.
//
// Each model's cases run against ONE session, opened once and shared by its
// subtests — every case is a refusal, so no case changes any state either
// side of the wire, and an Open here costs ~3 s (see ftdx101_test.go on why
// that is budgeted rather than optimised). The whole test asserts ZERO wire
// traffic across every rung: the refusals are defence in depth below the
// clone service, and a refusal that had already transmitted something would
// be no defence at all.
func TestWriteChannel_RefusalLadder(t *testing.T) {
	// Each helper starts from an otherwise-valid channel and breaks exactly
	// what its name says, so the fixtures differ in one thing at a time.
	badMode := func(ch codeplug.Channel) codeplug.Channel {
		ch.Data.Mode = "NOT-A-MODE"
		return ch
	}
	knownDisplay := func(ch codeplug.Channel) codeplug.Channel {
		ch.Data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
		return ch
	}

	// A channel broken in the FieldState rung AND in every rung after it.
	invalidTone := func() codeplug.Channel {
		ch := knownDisplay(badMode(writableChannel("042")))
		// A non-Known state may not carry a value: incoherent, and refused
		// rather than interpreted.
		ch.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown, Value: 670}
		return ch
	}
	invalidSkip := func() codeplug.Channel {
		ch := knownDisplay(badMode(writableChannel("042")))
		ch.Data.ScanSkip = codeplug.BoolField{State: codeplug.Unknown, Value: true}
		return ch
	}
	invalidDisplay := func() codeplug.Channel {
		ch := badMode(writableChannel("042"))
		ch.Data.TagDisplay = codeplug.BoolField{State: codeplug.FieldState("maybe")}
		return ch
	}

	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			// The image POPULATES 501, so that Open's discovery walk reports a
			// 60M bank and the read-only rung below has a real discovered slot
			// to be refused for. Without it "501" would be in no bank at all
			// and would meet rung 2 instead — which would still be a refusal,
			// and would test nothing about read-only banks.
			p, sess := openSession(t, m, Simulated, slotImage{
				mtAnswers: map[string]string{"501": populatedAnswer("501")},
			})

			// Rung 1 and 2's fixtures are EMPTY channels as well as
			// slot-wrong, so they prove the slot rungs outrank the erase rung.
			for _, tt := range []struct {
				name       string
				ch         codeplug.Channel
				wantFields []spec.Field
				reasonHas  string
			}{
				{
					// RUNG 1 — ParseSlot, ahead of both bankFor and the erase
					// refusal (this channel is empty too).
					name:      "1: an ungrammatical slot outranks bankFor and erase",
					ch:        codeplug.Channel{Slot: "0X1"},
					reasonHas: "not a valid slot",
				},
				{
					// RUNG 2 — bankFor, ahead of the erase refusal. "000" is
					// the DIALECT register's ASSUMED NoneWire, in no FTdx101
					// slot legend: it PARSES, so rung 1 passes, and it belongs
					// to no bank.
					name:      "2: a slot in no bank outranks erase",
					ch:        codeplug.Channel{Slot: "000"},
					reasonHas: "not part of any bank",
				},
				{
					// RUNG 3 — the erase refusal, naming spec.FieldErase (this
					// radio's command set contains no erase command at all —
					// matrix §2.3). It must also come before the FieldState
					// rung STRUCTURALLY, not merely by preference: an empty
					// channel has no Data for those checks to dereference.
					name:       "3: an empty channel is an erase, naming FieldErase",
					ch:         codeplug.Channel{Slot: "042"},
					wantFields: []spec.Field{spec.FieldErase},
					reasonHas:  "no erase command",
				},
				{
					// RUNG 4a — CTCSSTone.Valid, ahead of ScanSkip/TagDisplay's
					// checks, the capability gate (this channel's display value
					// is Known) and the frame build (its mode is invalid).
					name:       "4a: an incoherent CTCSS tone outranks the gate and the build",
					ch:         invalidTone(),
					wantFields: []spec.Field{spec.FieldCTCSSTone},
					reasonHas:  "must have zero Value",
				},
				{
					// RUNG 4b — ScanSkip.Valid, same construction.
					name:       "4b: an incoherent scan skip outranks the gate and the build",
					ch:         invalidSkip(),
					wantFields: []spec.Field{spec.FieldScanSkip},
					reasonHas:  "must have zero Value",
				},
				{
					// RUNG 4c — TagDisplay.Valid. This rung SURVIVES the named
					// inversion and is not the same check the FT-710's
					// buildWriteCommands makes: it refuses an INCOHERENT
					// BoolField (an unrecognised State), which is wrong
					// whatever the radio can express, and says nothing about
					// whether a coherent non-Known value may be written. An
					// "unrecognised State" is not Known, so requestedFields
					// does not request the field and the capability gate below
					// would never see it — this rung is the only thing standing
					// between a malformed field and the frame builder.
					name:       "4c: an unrecognised TagDisplay State outranks the build",
					ch:         invalidDisplay(),
					wantFields: []spec.Field{spec.FieldTagDisplay},
					reasonHas:  "invalid State",
				},
				{
					// RUNG 5 — the capability gate, ahead of the frame build.
					// THE INVERSION'S TEST: a Known TagDisplay is refused HERE,
					// and the mode is invalid too, so a build-time refusal
					// would name mode instead.
					name:       "5: a Known TagDisplay is refused by the capability gate, ahead of the build",
					ch:         knownDisplay(badMode(writableChannel("042"))),
					wantFields: []spec.Field{spec.FieldTagDisplay},
					reasonHas:  "not write-Supported for this session",
				},
				{
					// RUNG 5 again, the tone and skip halves: a Known value for
					// a field this codec cannot express is refused rather than
					// silently dropped (DRIVER register entry 6).
					name:       "5: a Known CTCSS tone is refused by the capability gate",
					ch:         withKnownTone(writableChannel("042")),
					wantFields: []spec.Field{spec.FieldCTCSSTone},
					reasonHas:  "not write-Supported for this session",
				},
				{
					name:       "5: a Known scan skip is refused by the capability gate",
					ch:         withKnownSkip(writableChannel("042")),
					wantFields: []spec.Field{spec.FieldScanSkip},
					reasonHas:  "not write-Supported for this session",
				},
				{
					// RUNG 5 once more, the DISCOVERED-BANK half, which the
					// FTdx10's ladder has no equivalent of: a 5xx slot this
					// radio actually reported IS found by bankFor (so rung 2
					// passes) and is then refused field by field, every field
					// of a discovered bank being read-only (§1.3.5). "this
					// radio has no such channel" and "this channel is not
					// writable" are different refusals and must read
					// differently.
					name:       "5: a discovered 60M slot passes bankFor and is refused as read-only",
					ch:         writableChannel("501"),
					wantFields: writeGateSixFields(),
					reasonHas:  "not write-Supported for this session",
				},
				{
					// RUNG 6 — the frame build, reached only once every rung
					// above has passed. Named here so the ladder's own last
					// step is part of the ordered walk rather than only of the
					// builder's tests.
					name:       "6: an inexpressible mode is refused by the frame build",
					ch:         badMode(writableChannel("042")),
					wantFields: []spec.Field{spec.FieldMode},
					reasonHas:  "is not a mode this radio supports",
				},
			} {
				t.Run(tt.name, func(t *testing.T) {
					before := len(p.Transcript())
					wre := refusedFields(t, sess, tt.ch)

					if !slices.Equal(wre.Fields, tt.wantFields) {
						t.Errorf("WriteRefusedError.Fields = %v, want %v — the refusal must name the rung it came from, not a later one", wre.Fields, tt.wantFields)
					}
					if !strings.Contains(wre.Reason, tt.reasonHas) {
						t.Errorf("WriteRefusedError.Reason = %q, want it to contain %q", wre.Reason, tt.reasonHas)
					}
					if wre.Slot != tt.ch.Slot {
						t.Errorf("WriteRefusedError.Slot = %q, want %q", wre.Slot, tt.ch.Slot)
					}
					if got := p.Transcript(); len(got) != before {
						t.Errorf("a refused WriteChannel sent %d frames (%q), want 0 — every rung must refuse BEFORE any wire traffic", len(got)-before, got[before:])
					}
				})
			}
		})
	}
}

// writeGateSixFields is requestedFields' output for an ORDINARY channel — the
// six plain fields, in its own order — which is also the field list the
// capability gate names when a whole bank is unwritable. Spelt out rather
// than derived from requestedFields, so that the two are compared rather than
// one being the other's echo.
func writeGateSixFields() []spec.Field {
	return []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
	}
}

// TestWriteChannel_KnownTagDisplayRefusedByTheGate is THE NAMED INVERSION's
// own test (matrix §3.7), and what it pins is WHICH refusal fires — not that
// one does.
//
// On the FT-710 a display value that cannot be honoured is refused inside
// buildWriteCommands, because that radio's MT frame has a mandatory display
// flag and sending the channel would have to invent one. This radio's
// combined form has no flag at all — P11 is "0: (Fixed)" at layout 1329 and
// the geometry witness accounts for all 41 positions with no gap — so there
// is nothing to invent and nothing for a builder to refuse; the refusal
// instead comes from the CAPABILITY GATE, because requestedFields includes
// spec.FieldTagDisplay exactly when the state is Known and this driver's
// FieldTagDisplay Write support is spec.Unsupported in every profile and on
// every bank.
//
// The distinction is testable and is tested three ways: the refusal names
// exactly spec.FieldTagDisplay (not a build error's field, and not the six
// plain fields alongside it — those ARE writable on this profile, which is
// what makes the singleton meaningful); its Reason is the GATE's own
// sentence; and buildWriteCommand accepts the very same channel when called
// directly, for BOTH models' dialects, proving the builder has no opinion
// about the field at all.
//
// The gate half runs on ONE model's session and the builder half on both
// dialects, because the builder half costs no Open at all: the gate's own
// per-model behaviour is already walked by the refusal ladder above, which
// includes this very rung for each model.
func TestWriteChannel_KnownTagDisplayRefusedByTheGate(t *testing.T) {
	_, sess := openSession(t, testModels[0], Simulated, slotImage{})

	for _, tt := range []struct {
		name  string
		value bool
	}{
		// Known-FALSE is as real a Known value as Known-true — "the radio
		// should show the frequency" is a decision this radio cannot be told,
		// exactly like its opposite.
		{"Known true", true},
		{"Known false", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("042")
			ch.Data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: tt.value}

			wre := refusedFields(t, sess, ch)
			if !slices.Equal(wre.Fields, []spec.Field{spec.FieldTagDisplay}) {
				t.Errorf("WriteRefusedError.Fields = %v, want exactly [%s]", wre.Fields, spec.FieldTagDisplay)
			}
			if want := "not write-Supported for this session"; !strings.Contains(wre.Reason, want) {
				t.Errorf("WriteRefusedError.Reason = %q, want the CAPABILITY GATE's reason (containing %q) — a build-time display refusal would be the FT-710's answer, and this radio has no display flag to refuse", wre.Reason, want)
			}

			// The builder's side of the same claim, for BOTH models: it
			// accepts the channel, because it makes no TagDisplay decision
			// whatsoever.
			for _, m := range testModels {
				if _, err := buildWriteCommand(m.params.dialect, ch); err != nil {
					t.Errorf("%s: buildWriteCommand = %v, want success — the combined form has no display flag, so a Known value is not the BUILDER's to refuse (the gate above is what refuses it)", m.name, err)
				}
			}
		})
	}
}

// TestWriteChannel_UnavailableTagDisplayIsNotRefused is the inversion's
// positive half, and the one that would fail loudest if the FT-710's refusal
// were ever transplanted here: an Unavailable TagDisplay — the state EVERY
// channel this driver reads carries (read.go) — must not be refused for that
// field, and the write must proceed to the wire.
//
// Unknown is included alongside it because both are non-Known and the
// FT-710's refusal covers both; either being refused here would make an
// ordinary FTdx101 write impossible.
//
// The bytes are asserted too, not merely the success: TagDisplay's state
// cannot move a byte of a frame that has no place for it, so all three states
// this driver ever meets must produce the identical 41 bytes.
func TestWriteChannel_UnavailableTagDisplayIsNotRefused(t *testing.T) {
	p, sess := openSession(t, testModels[0], Simulated, slotImage{})

	for _, tt := range []struct {
		name       string
		tagDisplay codeplug.BoolField
	}{
		{"Unavailable (what this driver's own read path produces)", codeplug.BoolField{State: codeplug.Unavailable}},
		{"Unknown", codeplug.BoolField{State: codeplug.Unknown}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("042")
			ch.Data.TagDisplay = tt.tagDisplay

			before := len(p.Transcript())
			res, err := sess.WriteChannel(testCtx(t), ch)
			if err != nil {
				t.Fatalf("WriteChannel = %v, want nil — a non-Known TagDisplay requests nothing and this frame has no field it could reach", err)
			}
			assertSteps(t, res, wantOneStep(true, true))

			got := p.Transcript()[before:]
			if len(got) != 1 || got[0] != wantSetFrame {
				t.Errorf("frames sent = %q, want exactly [%q] — the display state must move no byte", got, wantSetFrame)
			}
		})
	}
}

// TestWriteChannel_OutcomeTable is the one-step WriteResult's truth table:
// the three outcomes a single-frame write can reach, each with the exact step
// list it must report.
//
// Read it as a whole. The ONLY difference between "the radio refused this
// frame" and "we could not tell what happened to it" is the Sent flag — and
// the step is PRESENT in all three rows, because the write always intended
// exactly one frame. A caller that sees {MT, Sent:false} knows the slot's
// on-radio state is unverified; one that sees {MT, Sent:true,
// Confirmed:false} knows the frame went out and was refused.
//
// One model throughout: the outcomes are the TRANSPORT's and the step list is
// this method's, neither of which has a model in it, and the cross-model
// identity of the frame itself is the literal pin's job.
func TestWriteChannel_OutcomeTable(t *testing.T) {
	t.Run("success: silence within the window is acceptance", func(t *testing.T) {
		// Fire-and-forget: on the ASSUMED acknowledgement convention (doc.go's
		// register entry 9, second half) a CAT Set produces NO acknowledgement,
		// so Confirmed means exactly "no rejection arrived in the window" and
		// never "the radio said yes" (see driver.WriteStep). What this test
		// pins is the FAKE's modelling of that convention, not any real
		// FTdx101's behaviour.
		_, sess := openSession(t, testModels[0], Simulated, slotImage{})

		res, err := sess.WriteChannel(testCtx(t), writableChannel("042"))
		if err != nil {
			t.Fatalf("WriteChannel = %v, want nil", err)
		}
		assertSteps(t, res, wantOneStep(true, true))
	})

	t.Run("rejected: Sent true, Confirmed false, error surfaced", func(t *testing.T) {
		// rejectSets makes the scripted radio answer "?;" to every combined MT
		// Set — the protocol's single unattributed NAK (matrix §3.8), and the
		// only negative answer a Set can draw.
		_, sess := openSession(t, testModels[0], Simulated, slotImage{rejectSets: true})

		res, err := sess.WriteChannel(testCtx(t), writableChannel("042"))
		if !errors.Is(err, cat.ErrRejected) {
			t.Fatalf("WriteChannel = %v, want errors.Is match against cat.ErrRejected", err)
		}
		if !strings.Contains(err.Error(), "042") {
			t.Errorf("error text %q does not name the slot", err.Error())
		}
		assertSteps(t, res, wantOneStep(true, false))
	})

	t.Run("transport failure: Sent stays false", func(t *testing.T) {
		// A transport-level failure is NOT a rejection: the host cannot
		// attribute the frame's fate at all, so Sent stays false and the
		// error, not the flags, carries the distinction.
		p := newRespondingPort(t, slotImage{catID: testModels[0].catID})
		port := &failableWritePort{inner: p.Port()}

		opened, err := testModels[0].newDrv(Simulated).Open(testCtx(t), port, testIdentity)
		if err != nil {
			t.Fatalf("Open: unexpected error: %v", err)
		}
		t.Cleanup(func() { _ = opened.Close() })
		port.armed.Store(true)

		res, err := opened.WriteChannel(testCtx(t), writableChannel("042"))
		if err == nil {
			t.Fatal("WriteChannel = nil error, want the injected transport write failure")
		}
		if errors.Is(err, cat.ErrRejected) {
			t.Fatalf("WriteChannel = %v, want a transport failure, NOT a radio rejection", err)
		}
		if errors.Is(err, driver.ErrWriteRefused) {
			t.Fatalf("WriteChannel = %v, want a transport failure, NOT a refusal — the frame was built and the step list must describe it", err)
		}
		// The step is present and un-Sent: "this frame was part of the plan
		// and never attributably went out".
		assertSteps(t, res, wantOneStep(false, false))
	})
}

// TestWriteChannel_RealHardwareProfileRefusesEveryRequestedField is the
// milestone's write guard asserted at the driver's own seam, FOR BOTH MODELS:
// on the RealHardware profile — which, while writeTrialsCompleteD and
// writeTrialsCompleteMP are both false, is the all-Unverified fail-safe
// (caps.go, matrix §3.11) — a perfectly valid, ordinary channel is refused
// before any wire traffic, with EVERY requested field named.
//
// PER MODEL is the point, and it is this package's difference from the
// FTdx10's equivalent test: the write guard is a per-radio fact (a capture
// from the D is never evidence about the MP — doc.go's register preamble), so
// a suite that checked one model's fail-safe would leave the other model
// free to write to real hardware while every test passed.
//
// This is also why every other test in this file uses the Simulated profile:
// on RealHardware the capability gate refuses everything, so the choreography
// below it could never be exercised at all. Stating that here, as a test, is
// what keeps "the tests use Simulated" from looking like a convenience.
//
// The field list is the whole of requestedFields for an ordinary channel —
// six, in its own order, tag_display absent because an Unavailable state
// requests nothing even here. The refusal naming all six (rather than
// stopping at the first) is deliberate: a user told only "frequency is not
// writable" would fix the frequency and try again.
func TestWriteChannel_RealHardwareProfileRefusesEveryRequestedField(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p, sess := openSession(t, m, RealHardware, slotImage{})

			before := len(p.Transcript())
			wre := refusedFields(t, sess, writableChannel("042"))

			if want := writeGateSixFields(); !slices.Equal(wre.Fields, want) {
				t.Errorf("WriteRefusedError.Fields = %v, want %v — every requested field, in requestedFields' order", wre.Fields, want)
			}
			if got := p.Transcript(); len(got) != before {
				t.Errorf("a refused WriteChannel sent %d frames, want 0", len(got)-before)
			}

			// And the same channel IS accepted on Simulated, which is what
			// makes the refusal above attributable to the PROFILE rather than
			// to the channel being invalid in some way this test cannot see.
			_, sim := openSession(t, m, Simulated, slotImage{})
			if _, err := sim.WriteChannel(testCtx(t), writableChannel("042")); err != nil {
				t.Errorf("the same channel on the Simulated profile = %v, want nil — otherwise the refusal above proves nothing about the profile", err)
			}
		})
	}
}

// tierFieldsInOrder is the Icom tier's ten spec.Fields in ChannelData's
// declaration order — the order codeplug's tierAddedFieldFor uses and the
// order requestedFields must append them in.
//
// Spelt out rather than derived from requestedFields, so that the two are
// COMPARED rather than one being the other's echo. (This package's OWN copy,
// as the FT-710's and FTdx10's namesakes are theirs: unexported test helpers
// do not cross package boundaries, and these drivers import one another
// nowhere by the rule in doc.go.)
//
// MODEL-INDEPENDENT, like requestedFields itself: §2 of the matrix is
// identical for the D and the MP throughout, so there is nothing here a
// per-model variant could differ in.
func tierFieldsInOrder() []spec.Field {
	return []spec.Field{
		spec.FieldTxFrequency,
		spec.FieldDuplex,
		spec.FieldOffset,
		spec.FieldToneMode,
		spec.FieldToneTx,
		spec.FieldToneRx,
		spec.FieldDTCSCode,
		spec.FieldDTCSPolarity,
		spec.FieldFilter,
		spec.FieldDataMode,
	}
}

// withEveryTierFieldKnown marks all ten tier fields Known on data. The values
// are arbitrary and never reach a wire — this driver's capability map has no
// entry for any of the ten, so the gate refuses them all — but the STATE is
// what requestedFields keys on, so it must be Known.
func withEveryTierFieldKnown(data codeplug.ChannelData) codeplug.ChannelData {
	data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_255_000}
	data.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
	data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 600_000}
	data.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "TSQL"}
	data.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 670}
	data.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 670}
	data.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}
	data.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: "NN"}
	data.Filter = codeplug.StringField{State: codeplug.Known, Value: "FIL1"}
	data.DataMode = codeplug.BoolField{State: codeplug.Known, Value: true}
	return data
}

// TestWriteChannel_KnownTierFieldRefusedBeforeWire is the tier half of the
// gate's stated contract: a value the caller explicitly marked Known must be
// REFUSED rather than silently dropped, for the ten fields the Icom tier
// added as much as for the tone and skip this driver has always refused.
//
// The channel is otherwise the ordinary one this profile accepts, so the
// refusal is attributable to the one Known tier value and to nothing else.
// DTCSCode is this driver's chosen representative — the FT-710 pins ToneMode
// and the FTdx10 TxFrequency, so the three tests between them exercise three
// of the ten.
//
// BOTH MODELS, because the gate is the capability profile's and the profiles
// are per model: a tier field that had somehow acquired write support on one
// model's map and not the other's would pass a single-model test.
//
// The MECHANISM is a lookup MISS: this radio's capability map (caps.go's
// bankFields) has no entry for spec.FieldDTCSCode on any bank of either
// model, so FieldSupport answers the ZERO spec.FieldSupport, which is neither
// CanWrite nor spec.Inert. Nothing had to be added to caps.go for the refusal
// to happen, and the first assertion below is what keeps that true.
func TestWriteChannel_KnownTierFieldRefusedBeforeWire(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p, sess := openSession(t, m, Simulated, slotImage{})

			bank, ok := sess.bankFor("042")
			if !ok {
				t.Fatalf("bankFor(%q) found no bank — the fixture is wrong, not the gate", "042")
			}
			if fs := sess.caps.FieldSupport(bank, spec.FieldDTCSCode); fs.CanWrite() || fs.Write == spec.Inert {
				t.Fatalf("FieldSupport(%q, %s) = %+v, want the zero FieldSupport (no tier field is in this radio's capability map)", bank, spec.FieldDTCSCode, fs)
			}

			ch := writableChannel("042")
			ch.Data.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}

			before := len(p.Transcript())
			wre := refusedFields(t, sess, ch)

			if !slices.Contains(wre.Fields, spec.FieldDTCSCode) {
				t.Errorf("WriteRefusedError.Fields = %v, want %s named — a refusal that does not name the field is not the contract", wre.Fields, spec.FieldDTCSCode)
			}
			if !strings.Contains(wre.Reason, "not write-Supported for this session") {
				t.Errorf("WriteRefusedError.Reason = %q, want the capability gate's own sentence", wre.Reason)
			}
			if wre.Slot != "042" {
				t.Errorf("WriteRefusedError.Slot = %q, want %q", wre.Slot, "042")
			}
			if got := p.Transcript(); len(got) != before {
				t.Errorf("a refused WriteChannel sent %d frames (%q), want 0 — the refusal must precede ALL wire traffic", len(got)-before, got[before:])
			}
		})
	}
}

// TestRequestedFields_MembershipAndOrder pins the gate's field set, which
// requestedFields' doc comment claims mirrors core/driver/ft710's and
// core/driver/ftdx10's — and through them codeplug.Diff's addedFields —
// EXACTLY. A comment is not a gate, and the packages cannot import one
// another, so the mirror is held by each pinning the SAME shape.
//
// Order is part of the contract, not incidental: this slice is what the gate
// walks, and therefore the order in which a WriteRefusedError names fields
// (see the RealHardware test's six). TagDisplay sits seventh whenever it
// appears at all — after Tag, before the tone/skip conditionals.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	base := writeGateSixFields()
	known := func(v bool) codeplug.BoolField {
		return codeplug.BoolField{State: codeplug.Known, Value: v}
	}

	for _, tt := range []struct {
		name string
		data codeplug.ChannelData
		want []spec.Field
	}{
		{
			// The ORDINARY FTdx101 channel, and the case that matters most
			// here: exactly the six, with tag_display ABSENT. If this row ever
			// grew a seventh entry, every write this driver can make would be
			// refused (FieldTagDisplay is Unsupported everywhere).
			name: "the ordinary channel: six fields, no tag_display",
			data: *writableChannel("042").Data,
			want: base,
		},
		{
			name: "Known TagDisplay adds it seventh",
			data: codeplug.ChannelData{TagDisplay: known(true)},
			want: append(append([]spec.Field(nil), base...), spec.FieldTagDisplay),
		},
		{
			// Known-FALSE requests the field exactly as Known-true does: the
			// conditional is on the STATE, never on the value.
			name: "Known-false TagDisplay adds it too",
			data: codeplug.ChannelData{TagDisplay: known(false)},
			want: append(append([]spec.Field(nil), base...), spec.FieldTagDisplay),
		},
		{
			name: "Known tone adds it after tag_display's slot",
			data: codeplug.ChannelData{CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: 670}},
			want: append(append([]spec.Field(nil), base...), spec.FieldCTCSSTone),
		},
		{
			name: "Known scan skip adds it last",
			data: codeplug.ChannelData{ScanSkip: known(true)},
			want: append(append([]spec.Field(nil), base...), spec.FieldScanSkip),
		},
		{
			name: "all three Known: tag_display, tone, scan_skip, in that order",
			data: codeplug.ChannelData{
				TagDisplay: known(true),
				CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: 670},
				ScanSkip:   known(true),
			},
			want: append(append([]spec.Field(nil), base...),
				spec.FieldTagDisplay, spec.FieldCTCSSTone, spec.FieldScanSkip),
		},
		{
			// Unavailable and Unknown alike request NOTHING: both mean
			// "preserve whatever the radio has".
			name: "Unavailable and Unknown states request nothing",
			data: codeplug.ChannelData{
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
			want: base,
		},
		{
			// The ZERO ChannelData still requests exactly the pre-tier six:
			// no tier State is Known on it, so none of the ten joins.
			name: "the zero ChannelData requests only the six",
			data: codeplug.ChannelData{},
			want: base,
		},
		{
			// THE TIER EXTENSION, one field at a time: a Known DTCSCode is
			// REQUESTED, so the capability gate gets to see it. Before the fix
			// wave this row came back as the bare six and the value was
			// silently dropped.
			name: "a Known DTCSCode is requested, after the pre-tier set",
			data: codeplug.ChannelData{DTCSCode: codeplug.IntField{State: codeplug.Known, Value: 23}},
			want: append(append([]spec.Field(nil), base...), spec.FieldDTCSCode),
		},
		{
			// The tier ten never displace the pre-tier three: tag_display is
			// still seventh, tone eighth, skip ninth, and the ten follow.
			name: "all three pre-tier conditionals and all ten tier fields, in order",
			data: withEveryTierFieldKnown(codeplug.ChannelData{
				TagDisplay: known(true),
				CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: 670},
				ScanSkip:   known(true),
			}),
			want: append(append(append([]spec.Field(nil), base...),
				spec.FieldTagDisplay, spec.FieldCTCSSTone, spec.FieldScanSkip),
				tierFieldsInOrder()...),
		},
		{
			// The ten alone, with every pre-tier conditional absent: the
			// declaration order is visible with nothing in front of it.
			name: "the ten tier fields alone keep ChannelData's declaration order",
			data: withEveryTierFieldKnown(codeplug.ChannelData{}),
			want: append(append([]spec.Field(nil), base...), tierFieldsInOrder()...),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := requestedFields(tt.data); !slices.Equal(got, tt.want) {
				t.Errorf("requestedFields = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNameMaps_AreExactInverses walks read.go's forward maps and write.go's
// backward ones in both directions.
//
// The hazard is asymmetric drift: a spelling added to one map and forgotten
// in the other would silently refuse at the write gate a value this driver's
// own read path had just produced, and the failure would surface as a
// mysterious refusal rather than as a broken test. The cross-check is also
// what keeps the vocabularies tied to what this driver ADVERTISES — the third
// leg below checks both name sets against the capability data's own
// CTCSSStates and ShiftOptions, for BOTH models, so a name that reaches no
// user's screen cannot sit in either map unnoticed.
func TestNameMaps_AreExactInverses(t *testing.T) {
	t.Run("ctcss", func(t *testing.T) {
		if len(ctcssNames) != len(ctcssByName) {
			t.Errorf("ctcssNames has %d entries, ctcssByName %d — they must be exact inverses", len(ctcssNames), len(ctcssByName))
		}
		for state, name := range ctcssNames {
			back, ok := ctcssByName[name]
			if !ok {
				t.Errorf("ctcssByName has no entry for %q (read.go maps %q to it)", name, state.Wire())
				continue
			}
			if back != state {
				t.Errorf("ctcssByName[%q] = %q, want %q", name, back.Wire(), state.Wire())
			}
		}
	})

	t.Run("shift", func(t *testing.T) {
		if len(shiftNames) != len(shiftByName) {
			t.Errorf("shiftNames has %d entries, shiftByName %d — they must be exact inverses", len(shiftNames), len(shiftByName))
		}
		for state, name := range shiftNames {
			back, ok := shiftByName[name]
			if !ok {
				t.Errorf("shiftByName has no entry for %q (read.go maps %q to it)", name, state.Wire())
				continue
			}
			if back != state {
				t.Errorf("shiftByName[%q] = %q, want %q", name, back.Wire(), state.Wire())
			}
		}
	})

	t.Run("both vocabularies are the ones this driver advertises", func(t *testing.T) {
		for _, m := range testModels {
			caps := capabilitiesSimulated(m.params)
			for _, s := range caps.CTCSSStates {
				if _, ok := ctcssByName[s.Value]; !ok {
					t.Errorf("%s: Capabilities.CTCSSStates advertises %q, which the write path cannot map to a wire byte", m.name, s.Value)
				}
			}
			if len(caps.CTCSSStates) != len(ctcssByName) {
				t.Errorf("%s: Capabilities advertises %d CTCSS states, ctcssByName maps %d", m.name, len(caps.CTCSSStates), len(ctcssByName))
			}
			for _, s := range caps.ShiftOptions {
				if _, ok := shiftByName[s.Value]; !ok {
					t.Errorf("%s: Capabilities.ShiftOptions advertises %q, which the write path cannot map to a wire byte", m.name, s.Value)
				}
			}
			if len(caps.ShiftOptions) != len(shiftByName) {
				t.Errorf("%s: Capabilities advertises %d shift options, shiftByName maps %d", m.name, len(caps.ShiftOptions), len(shiftByName))
			}
		}
	})
}

// TestMTSetSpec_IsFireAndForget pins the Set's transport spec, and each half
// of it is a hazard rather than a preference.
//
// ClassWrite, and no answer matcher: that is what selects transport's
// fire-and-forget path (before D2 it was the ABSENCE of a prefix). A
// spec pinning the ANSWER geometry read.go's mtSpec derives — the same "MT"
// prefix, the same exact 41 — would wait the whole read timeout for a reply a
// Set never produces, and then report a timeout for a write the radio had
// accepted.
//
// RetryReads 0: a write is never resent (transport safety obligation 2), and
// transport.Engine.Do refuses any write-class spec with a non-zero
// RetryReads outright. The contrast with mtSpec's RetryReads 1 is the whole
// point — a READ is idempotent and a WRITE is not — and it is asserted for
// both models' dialects, since the read spec is derived per dialect while
// this one is a constant.
func TestMTSetSpec_IsFireAndForget(t *testing.T) {
	got := mtSetSpec()
	if got.Class != transport.ClassWrite {
		t.Errorf("Class = %v, want transport.ClassWrite — that is what selects transport's fire-and-forget path since D2 made the class explicit", got.Class)
	}
	if got.Match != nil {
		t.Error("Match is non-nil, want nil — an answer matcher makes transport wait for a reply a Set never sends, and transport.CommandSpec.validate refuses one on a ClassWrite outright")
	}
	if got.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 — a write is NEVER resent (transport safety obligation 2)", got.RetryReads)
	}

	// The read spec's contrast, asserted here so the pair is legible in one
	// place: the same command, opposite retry policy.
	for _, m := range testModels {
		read, err := mtSpec(m.params.dialect)
		if err != nil {
			t.Fatalf("%s: mtSpec = %v", m.name, err)
		}
		if read.RetryReads == got.RetryReads {
			t.Errorf("%s: the MT READ spec's RetryReads (%d) equals the SET spec's (%d) — a read is idempotent and a write is not, and the two specs must not have converged", m.name, read.RetryReads, got.RetryReads)
		}
	}
}

// TestBuildWriteCommand_RefusesInexpressibleValues walks the builder's own
// checklist, for BOTH models' dialects. Every refusal is typed
// (*driver.WriteRefusedError), names the field it is about where a field is
// to blame, and returns the ZERO Command alongside — a caller that ignored
// the error must not find a plausible frame in its hand.
//
// A free function taking the dialect, exactly like the FT-710's and the
// FTdx10's, so these cases cost no session: they are pure mapping decisions
// and none of them reaches the wire. Taking the dialect is additionally what
// makes the per-model sweep free here.
func TestBuildWriteCommand_RefusesInexpressibleValues(t *testing.T) {
	for _, tt := range []struct {
		name string
		// slot defaults to the ordinary memory channel when empty; only the
		// 5xx row needs to say otherwise.
		slot       string
		mutate     func(*codeplug.ChannelData)
		wantFields []spec.Field
		reasonHas  string
	}{
		{
			name:       "a mode this radio's legend does not list",
			mutate:     func(d *codeplug.ChannelData) { d.Mode = "C4FM" },
			wantFields: []spec.Field{spec.FieldMode},
			reasonHas:  `mode "C4FM" is not a mode this radio supports`,
		},
		{
			// cat.ModeUnset ('0', "-") is in this dialect's table so that
			// PARSERS may accept the placeholder (the DIALECT register's entry
			// "the cat.ModeUnset member of the mode table"), and core/cat
			// refuses to EMIT it. A channel asking for it must be refused, not
			// encoded — and the refusal comes from the BUILDER, not from this
			// function's own mode lookup, because "-" IS in the dialect's
			// name table.
			name:      "the ModeUnset placeholder is not writable",
			mutate:    func(d *codeplug.ChannelData) { d.Mode = "-" },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			name:       "a CTCSS state outside OFF/ENC-DEC/ENC",
			mutate:     func(d *codeplug.ChannelData) { d.CTCSS = "TSQL" },
			wantFields: []spec.Field{spec.FieldCTCSSState},
			reasonHas:  "is not one of OFF/ENC-DEC/ENC",
		},
		{
			name:       "a shift outside SIMPLEX/PLUS/MINUS",
			mutate:     func(d *codeplug.ChannelData) { d.Shift = "SPLIT" },
			wantFields: []spec.Field{spec.FieldShift},
			reasonHas:  "is not one of SIMPLEX/PLUS/MINUS",
		},
		{
			// Beyond this dialect's manual-evidenced +/-9990 Hz range. The
			// bounds check exists AHEAD of the int16 conversion precisely so a
			// value like this cannot wrap into a plausible small offset.
			name:       "a clarifier beyond the dialect's range",
			mutate:     func(d *codeplug.ChannelData) { d.ClarHz = 10_000 },
			wantFields: []spec.Field{spec.FieldClarifier},
			reasonHas:  "exceeds +/-9990 Hz",
		},
		{
			// A magnitude that would WRAP an int16 (32767 max): the refusal
			// must come from the explicit bounds check, never from a
			// conversion that turned 40000 into a legal-looking -25536.
			name:       "a clarifier large enough to wrap int16",
			mutate:     func(d *codeplug.ChannelData) { d.ClarHz = 40_000 },
			wantFields: []spec.Field{spec.FieldClarifier},
			reasonHas:  "exceeds +/-9990 Hz",
		},
		{
			// In range, but not a multiple of the ASSUMED 10 Hz step (the
			// DIALECT register's "ClarifierPolicy.StepHz = 10"). This one is
			// the BUILDER's refusal, not this function's, which is the design:
			// the step rule lives in core/cat and is enforced once.
			name:      "a clarifier off the step grid",
			mutate:    func(d *codeplug.ChannelData) { d.ClarHz = 5 },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			// The combined form pads to full width with TagFill and refuses a
			// tag that would not round-trip: a trailing fill byte would be
			// trimmed on the way back, so it is refused rather than silently
			// dropped.
			name:      "a tag ending in the fill byte would not round-trip",
			mutate:    func(d *codeplug.ChannelData) { d.Tag = "FT8 " },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			name:      "a tag longer than the 12-byte field",
			mutate:    func(d *codeplug.ChannelData) { d.Tag = "THIRTEEN CHRS" },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			// The command-injection defence: a ';' inside a tag would
			// terminate the frame early and turn the remainder into a second
			// command.
			name:      "a tag containing the frame terminator",
			mutate:    func(d *codeplug.ChannelData) { d.Tag = "FT8;MT" },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			// A 5xx slot is grammatical and parses, but core/cat's own
			// combined-MT policy refuses it as a SET target (project policy
			// pending M5a). WriteChannel's capability gate refuses it first in
			// practice; the builder refusing it too is the defence in depth.
			name:      "a 5xx slot is not a legal combined-Set target",
			slot:      "501",
			mutate:    func(d *codeplug.ChannelData) {},
			reasonHas: "cannot encode the combined MT Set frame",
		},
	} {
		for _, m := range testModels {
			t.Run(tt.name+"/"+m.name, func(t *testing.T) {
				slot := tt.slot
				if slot == "" {
					slot = "042"
				}
				ch := writableChannel(slot)
				tt.mutate(ch.Data)

				cmd, err := buildWriteCommand(m.params.dialect, ch)
				var wre *driver.WriteRefusedError
				if !errors.As(err, &wre) {
					t.Fatalf("buildWriteCommand = %v (%T), want a *driver.WriteRefusedError", err, err)
				}
				if tt.wantFields != nil && !slices.Equal(wre.Fields, tt.wantFields) {
					t.Errorf("WriteRefusedError.Fields = %v, want %v", wre.Fields, tt.wantFields)
				}
				if !strings.Contains(wre.Reason, tt.reasonHas) {
					t.Errorf("WriteRefusedError.Reason = %q, want it to contain %q", wre.Reason, tt.reasonHas)
				}
				if wre.Slot != ch.Slot {
					t.Errorf("WriteRefusedError.Slot = %q, want %q", wre.Slot, ch.Slot)
				}
				if len(cmd.Bytes()) != 0 {
					t.Errorf("buildWriteCommand returned %q alongside its error, want the ZERO Command", cmd.Bytes())
				}
			})
		}
	}
}

// TestBuildWriteCommand_NoTagDisplayRefusalTakesPriority pins the named
// inversion as an ORDER property, which is the half a "does it succeed?" test
// cannot reach.
//
// Given a channel that is wrong in the display field AND in three ways the
// builder DOES check (mode, ctcss state, clarifier), the refusal must name
// MODE — the first of those three — for every one of the four display states.
// If the FT-710's refusal were ever transplanted into this function, the two
// non-Known rows would name tag_display instead and this test would say so.
func TestBuildWriteCommand_NoTagDisplayRefusalTakesPriority(t *testing.T) {
	multiplyInvalid := func() codeplug.Channel {
		ch := writableChannel("042")
		ch.Data.Mode = "NOT-A-MODE"
		ch.Data.CTCSS = "NOT-A-CTCSS-STATE"
		ch.Data.ClarHz = 999_999
		return ch
	}

	for _, tt := range []struct {
		name       string
		tagDisplay codeplug.BoolField
	}{
		{"Unavailable", codeplug.BoolField{State: codeplug.Unavailable}},
		{"Unknown", codeplug.BoolField{State: codeplug.Unknown}},
		{"Known true", codeplug.BoolField{State: codeplug.Known, Value: true}},
		// The zero BoolField: what a composite-literal ChannelData that
		// forgets the field produces. WriteChannel's FieldState rung refuses
		// it (an unset State is not one of the three), but the BUILDER must
		// not have an opinion about it either.
		{"zero value (State unset)", codeplug.BoolField{}},
	} {
		for _, m := range testModels {
			t.Run(tt.name+"/"+m.name, func(t *testing.T) {
				ch := multiplyInvalid()
				ch.Data.TagDisplay = tt.tagDisplay

				_, err := buildWriteCommand(m.params.dialect, ch)
				var wre *driver.WriteRefusedError
				if !errors.As(err, &wre) {
					t.Fatalf("buildWriteCommand = %v, want a *driver.WriteRefusedError", err)
				}
				if !slices.Equal(wre.Fields, []spec.Field{spec.FieldMode}) {
					t.Fatalf("WriteRefusedError.Fields = %v, want exactly [%s] — no TagDisplay state may outrank the builder's first real check, because the combined form has no display flag to refuse", wre.Fields, spec.FieldMode)
				}
			})
		}
	}
}

// TestBuildWriteCommand_P7IsTheFormConstant pins the combined Set's kind byte
// to cat.CombinedMTSetKind — the FORM's schema constant, this radio's "Set:
// 0: (Fixed)" at layout 1324 — and pins that it is NOT read from the
// dialect's MWWriteKind.
//
// The two are EQUAL for core/cat/ftdx101 (its MWWriteKind is
// cat.CombinedMTSetKind, because this radio's MW-Set P7 legend also reads
// "(Fixed)" at layout 1364), so an assertion against either model's dialect
// alone cannot tell the two sources apart. The second half therefore builds a
// PEER dialect identical in every respect that matters here EXCEPT its MW
// write kind, and requires the same success and the same P7: a write path
// that consulted MWWriteKind would be refused outright by
// BuildMTSetCombined, which validates P7 against the form constant and does
// not rewrite it.
//
// Why it matters, since the byte is the same today: matrix §3.6 records
// MT-Set P7 and MW-Set P7 as "two supporting facts, both MANUAL-EVIDENCED and
// both easy to conflate" — two command-specific facts of this radio that
// happen to agree, not one fact used twice. Deriving one from the other is
// the PadByte conflation core/cat has already paid for once (see
// cat.CombinedMTSetKind's doc comment and
// TestBuildMTSetCombined_P7IsAFormConstantNotTheMWWriteKind). This driver
// sends no MW frame at all (doc.go), so it has no business consulting MW's
// kind.
func TestBuildWriteCommand_P7IsTheFormConstant(t *testing.T) {
	// The chart's own position: P7 is at index 22 of the frame (position 23),
	// which the geometry witness records as `MT,set,P7,23,23` — written out
	// here because this is the CHART being asserted (respondingport_test.go's
	// mtAnswerFields records the same offsets for the read direction).
	const p7Index = 22

	for _, m := range testModels {
		cmd, err := buildWriteCommand(m.params.dialect, writableChannel("042"))
		if err != nil {
			t.Fatalf("%s: buildWriteCommand = %v, want a frame", m.name, err)
		}
		if got := cmd.Bytes()[p7Index]; got != cat.CombinedMTSetKind {
			t.Errorf("%s: P7 = %q, want %q (cat.CombinedMTSetKind, the combined Set's fixed \"(Fixed)\" kind)", m.name, got, cat.CombinedMTSetKind)
		}

		// The coincidence, recorded so it cannot quietly become load-bearing.
		if m.params.dialect.MWWriteKind() != cat.CombinedMTSetKind {
			t.Fatalf("%s: core/cat/ftdx101's MWWriteKind() = %q, no longer equal to cat.CombinedMTSetKind — the peer-dialect leg below is now the ONLY thing pinning this driver's P7 source, and this dialect has become a discriminating case: assert it directly", m.name, m.params.dialect.MWWriteKind())
		}
	}

	// The discriminating case: a combined-form dialect whose MW write kind is
	// cat.KindMemory ('1'), which BuildMTSetCombined refuses as a P7.
	peer := cat.MustNewDialect(cat.DialectConfig{
		CATID:     "0683",
		ModeNames: map[cat.Mode]string{cat.ModeUnset: "-", cat.Mode('C'): "DATA-U"},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 501, SixtyHi: 599,
			PMSPairs:      9,
			EmergencyWire: "EMG",
			NoneWire:      "000",
			MCSelects:     cat.MCSelectsAll,
		},
		MT:          cat.MTPolicy{Form: cat.MTFormCombined, P11: cat.P11Fixed, ReadSlots: cat.MTReadsReadable, TagMaxBytes: 12, TagFill: ' '},
		Clarifier:   cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:    cat.P5TxClar,
		MWWriteKind: cat.KindMemory,
	})
	if peer.MWWriteKind() == cat.CombinedMTSetKind {
		t.Fatal("the peer dialect's MWWriteKind equals the form constant — it cannot discriminate")
	}

	cmd, err := buildWriteCommand(peer, writableChannel("042"))
	if err != nil {
		t.Fatalf("buildWriteCommand on the peer dialect = %v, want a frame — a write path reading MWWriteKind() would be refused here by BuildMTSetCombined, which validates P7 against the form constant", err)
	}
	if got := cmd.Bytes()[p7Index]; got != cat.CombinedMTSetKind {
		t.Errorf("peer dialect P7 = %q, want %q — P7 is the FORM's constant and must not track MWWriteKind", got, cat.CombinedMTSetKind)
	}
}

// TestWriteChannel_ConsentedSessionWrites: on a REAL-HARDWARE session built
// with WithConsentedUnverifiedWrites, for EACH model, the ordinary channel
// that TestWriteChannel_RealHardwareProfileRefusesEveryRequestedField sees
// refused is written instead — one combined MT Set, sent and unrejected,
// and byte-for-byte the same hand-derived frame the Simulated profile puts
// on the wire (wantSetFrame).
//
// WHAT THIS PROVES, EXACTLY: the DRIVER-level capability gate opens
// (spec.ConsentedUnverified makes FieldSupport.CanWrite true, so
// WriteChannel's own gate passes the six requested fields), and the MT frame
// round-trips against this package's scripted peer. That is CHOREOGRAPHY
// ONLY.
//
// It is NOT the project's write-and-verify pair, and this test must not be
// read as claiming one. Verification — write, read back, compare — lives in
// core/clone/execute.go, one layer above every driver, and it is exercised
// with consented capabilities there, in its own task. Nothing here reads the
// slot back, and this driver's WriteChannel reports only sent/unrejected by
// design (see its doc comment).
//
// PER MODEL, because consent lifts a PER-RADIO fail-safe: writeTrialsCompleteD
// and writeTrialsCompleteMP are separate constants for separate radios, and a
// one-model test would leave the other model's consented path unexercised.
func TestWriteChannel_ConsentedSessionWrites(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p, sess := openSession(t, m, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())

			before := len(p.Transcript())
			res, err := sess.WriteChannel(testCtx(t), writableChannel("042"))
			if err != nil {
				t.Fatalf("WriteChannel = %v, want nil — the consented session's gate must open", err)
			}
			assertSteps(t, res, wantOneStep(true, true))

			got := p.Transcript()[before:]
			if len(got) != 1 {
				t.Fatalf("the write sent %d frames (%q), want exactly 1 — this radio's whole write choreography is one combined MT Set", len(got), got)
			}
			if got[0] != wantSetFrame {
				t.Errorf("Set frame =\n %q\nwant\n %q — consent changes WHETHER a write is permitted, never what it puts on the wire", got[0], wantSetFrame)
			}
		})
	}
}

// TestWriteChannel_NoConsent_StillRefused is the other half of the pair
// above, and the one that keeps consent a DECISION: the same RealHardware
// profile, the same ordinary channel, no option — and the write is refused
// before any wire traffic, naming every requested field, exactly as it was
// before the option existed.
//
// It mirrors TestWriteChannel_RealHardwareProfileRefusesEveryRequestedField's
// shape deliberately. That test states the fail-safe; this one states that
// adding a way to lift it did not lift it by default.
func TestWriteChannel_NoConsent_StillRefused(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p, sess := openSession(t, m, RealHardware, slotImage{})

			before := len(p.Transcript())
			wre := refusedFields(t, sess, writableChannel("042"))

			if want := writeGateSixFields(); !slices.Equal(wre.Fields, want) {
				t.Errorf("WriteRefusedError.Fields = %v, want %v — every requested field, in requestedFields' order", wre.Fields, want)
			}
			if got := p.Transcript(); len(got) != before {
				t.Errorf("an unconsented, refused WriteChannel sent %d frames, want 0", len(got)-before)
			}
		})
	}
}

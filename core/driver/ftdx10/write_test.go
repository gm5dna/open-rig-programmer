// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

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

// writableChannel returns the ORDINARY FTdx10 channel: a populated slot
// whose three FieldState-carrying fields hold exactly what this driver's
// own read path produces for them (read.go) — TagDisplay Unavailable, tone
// and scan skip Unknown — and whose plain fields hold codec-expressible
// values.
//
// TagDisplay UNAVAILABLE is the point, and it is why this fixture cannot be
// the FT-710's: over there the equivalent helper carries a Known-true
// display flag, because that radio's MT frame has one and a non-Known value
// is refused outright. Here Unavailable is what EVERY channel legitimately
// carries, so a fixture with a Known display value would exercise only the
// error path and never the ordinary write.
func writableChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
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
	}
}

// wantOneStep builds the expected ONE-step result: this radio's whole write
// choreography is a single MT frame, so every non-refused outcome differs
// only in the two flags. Every step assertion in this file goes through it,
// so the mnemonic and the LENGTH are asserted alongside the flags rather
// than being respelt (and possibly mis-spelt) at each site.
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
// non-nil slice — the shape core/driver documents for a refusal that
// happens before any frame is built.
//
// The nil check is separate from the length check because len() cannot tell
// the two apart and the difference is durable and user-visible: the clone
// service marshals this list into an append-only audit file, where nil
// renders as `null` — which an auditor must read as "unknown", where the
// truth is the far stronger "no frame was ever built, so nothing whatever
// was attempted".
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
// Arming happens after Open, deliberately — it keeps the fixture
// independent of how many exchanges Open's probe-and-discovery choreography
// spends, which on this driver is about a hundred.
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

// wantSetFrame is THE 41-byte combined MT Set that writableChannel("010")
// must put on the wire, HAND-DERIVED FROM THE POSITION CHART and written
// out as a literal.
//
// Derived, never computed: the expectation is NOT built by calling
// cat.Dialect.BuildMTSetCombined, because a reference produced by the code
// under test agrees with a wrong offset exactly as happily as with a right
// one — the same reasoning respondingport_test.go's mtAnswerFields.frame
// records for the READ direction, and the reason that helper exists rather
// than the builder.
//
// The chart is the FTdx10 CAT manual's MT position table, rev 2308-F
// (manual lines 1217-1256; the shared field block's offsets are
// core/cat/memdata.go's, cited by core/cat/ftdx10/doc.go's difference pins,
// and the combined form's tail is core/cat/mtcombined.go's). Position by
// position, for writableChannel's values:
//
//	pos 1-2    "MT"          the command
//	pos 3-5    "010"         P1 slot
//	pos 6-14   "014250000"   P2 frequency, 14_250_000 Hz as 9 zero-padded digits
//	pos 15     "-"           P3 sign: ClarHz -150 is negative
//	pos 16-19  "0150"        P3 magnitude, |-150| as 4 zero-padded digits
//	pos 20     "1"           P4 RX clarifier on
//	pos 21     "0"           P5 TX clarifier off
//	pos 22     "2"           P6 mode: the FTdx10 legend's "2: USB"
//	pos 23     "0"           P7 kind: cat.CombinedMTSetKind, "0: (Fixed)"
//	pos 24     "1"           P8 CTCSS state: "1: ENC/DEC" = codeplug "ENC-DEC"
//	pos 25-26  "00"          P9, documented fixed
//	pos 27     "1"           P10 shift: "1: plus" = codeplug "PLUS"
//	pos 28     "0"           P11, documented fixed
//	pos 29-40  "CALLING     " P12 tag, 7 characters padded to 12 with the
//	                          dialect's TagFill (ASSUMED — its own register entry)
//	pos 41     ";"           terminator
//
// 41 bytes, and the literal 41 belongs in this file for the same reason it
// must not appear in the production code: here it is the CHART being
// asserted, there it would be a length the driver believed instead of one
// the dialect derived (see TestMTSpec_DerivesItsLengthFromTheDialect).
const wantSetFrame = "MT010014250000-0150102010010CALLING     ;"

// TestWriteChannel_LiteralFramePin is this task's flagship: one write of a
// fully-populated channel puts EXACTLY the hand-derived 41-byte Set on the
// wire, and exactly ONE frame.
//
// Asserted against the scripted port's own transcript rather than against
// the returned Command, so what is pinned is the bytes that crossed the
// wire — through the driver's real transport.Engine and its outbound write
// gate, both of which sit between buildWriteCommand and the port and either
// of which could in principle alter or refuse the frame.
//
// The one-frame assertion is the MT-ONLY decision's enforcement in the
// write direction, the exact counterpart of
// TestReadChannel_SendsExactlyOneMTAndNeverMR: no MW is sent, by this write
// or by anything else in the session's life, so a future "completion" of
// the write path that added the FT-710's MW exchange fails here rather than
// quietly making doc.go false.
func TestWriteChannel_LiteralFramePin(t *testing.T) {
	if len(wantSetFrame) != 41 {
		t.Fatalf("the hand-derived frame is %d bytes, want 41 per the manual's MT position chart — the literal is wrong before the driver is even asked", len(wantSetFrame))
	}

	p, sess := openSession(t, Simulated, slotImage{})

	before := len(p.Transcript())
	res, err := sess.WriteChannel(testCtx(t), writableChannel("010"))
	if err != nil {
		t.Fatalf("WriteChannel = %v, want nil", err)
	}
	assertSteps(t, res, wantOneStep(true, true))

	after := p.Transcript()
	got := after[before:]
	if len(got) != 1 {
		t.Fatalf("one WriteChannel sent %d frames (%q), want exactly 1 — the FTdx10's write choreography is ONE combined MT Set", len(got), got)
	}
	if got[0] != wantSetFrame {
		t.Errorf("Set frame =\n %q\nwant\n %q", got[0], wantSetFrame)
	}
	for i, frame := range after {
		if strings.HasPrefix(frame, "MW") {
			t.Errorf("frame %d = %q: this driver must never send MW — the write path is MT-only (see doc.go and WriteChannel)", i, frame)
		}
	}
}

// TestWriteChannel_RefusalLadder walks every rung of the ladder IN ORDER,
// and every case here asserts the ORDER rather than merely the existence of
// a refusal: each fixture is wrong in the rung under test AND in at least
// one LATER rung, so a refusal naming the later problem would mean the
// ladder had been reordered. A one-sided test cannot tell an ordering from a
// swallow.
//
// All of it runs against ONE session, opened by this function and shared by
// its subtests — every case is a refusal, so no case changes any state
// either side of the wire, and an Open here costs ~3 s (see ftdx10_test.go
// on why that is budgeted rather than optimised). The whole test asserts
// ZERO wire traffic across every rung: the refusals are defence in depth
// below the clone service, and a refusal that had already transmitted
// something would be no defence at all.
func TestWriteChannel_RefusalLadder(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

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
		ch := knownDisplay(badMode(writableChannel("010")))
		// A non-Known state may not carry a value: incoherent, and refused
		// rather than interpreted.
		ch.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown, Value: 670}
		return ch
	}
	invalidSkip := func() codeplug.Channel {
		ch := knownDisplay(badMode(writableChannel("010")))
		ch.Data.ScanSkip = codeplug.BoolField{State: codeplug.Unknown, Value: true}
		return ch
	}
	invalidDisplay := func() codeplug.Channel {
		ch := badMode(writableChannel("010"))
		ch.Data.TagDisplay = codeplug.BoolField{State: codeplug.FieldState("maybe")}
		return ch
	}

	// Rung 1 and 2's fixtures are EMPTY channels as well as slot-wrong, so
	// they prove the slot rungs outrank the erase rung.
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
			// RUNG 2 — bankFor, ahead of the erase refusal. "000" is the
			// dialect's ASSUMED SlotSpace.NoneWire (its own register entry):
			// it PARSES,
			// so rung 1 passes, and it belongs to no bank.
			name:      "2: a slot in no bank outranks erase",
			ch:        codeplug.Channel{Slot: "000"},
			reasonHas: "not part of any bank",
		},
		{
			// RUNG 3 — the erase refusal, naming spec.FieldErase. It must
			// also come before the FieldState rung STRUCTURALLY, not merely
			// by preference: an empty channel has no Data for those checks
			// to dereference.
			name:       "3: an empty channel is an erase, naming FieldErase",
			ch:         codeplug.Channel{Slot: "010"},
			wantFields: []spec.Field{spec.FieldErase},
			reasonHas:  "no erase command",
		},
		{
			// RUNG 4a — CTCSSTone.Valid, ahead of ScanSkip/TagDisplay's
			// checks, the capability gate (this channel's display value is
			// Known) and the frame build (its mode is invalid).
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
			// buildWriteCommands makes: it refuses an INCOHERENT BoolField
			// (an unrecognised State), which is wrong whatever the radio can
			// express, and says nothing about whether a coherent non-Known
			// value may be written. An "unrecognised State" is not Known, so
			// requestedFields does not request the field and the capability
			// gate below would never see it — this rung is the only thing
			// standing between a malformed field and the frame builder.
			name:       "4c: an unrecognised TagDisplay State outranks the build",
			ch:         invalidDisplay(),
			wantFields: []spec.Field{spec.FieldTagDisplay},
			reasonHas:  "invalid State",
		},
		{
			// RUNG 5 — the capability gate, ahead of the frame build. THE
			// INVERSION'S TEST: a Known TagDisplay is refused HERE, and the
			// mode is invalid too, so a build-time refusal would name mode
			// instead.
			name:       "5: a Known TagDisplay is refused by the capability gate, ahead of the build",
			ch:         knownDisplay(badMode(writableChannel("010"))),
			wantFields: []spec.Field{spec.FieldTagDisplay},
			reasonHas:  "not write-Supported for this session",
		},
		{
			// RUNG 5 again, the tone and skip halves: a Known value for a
			// field this codec cannot express is refused rather than
			// silently dropped.
			name:       "5: a Known CTCSS tone is refused by the capability gate",
			ch:         withKnownTone(writableChannel("010")),
			wantFields: []spec.Field{spec.FieldCTCSSTone},
			reasonHas:  "not write-Supported for this session",
		},
		{
			name:       "5: a Known scan skip is refused by the capability gate",
			ch:         withKnownSkip(writableChannel("010")),
			wantFields: []spec.Field{spec.FieldScanSkip},
			reasonHas:  "not write-Supported for this session",
		},
		{
			// RUNG 6 — the frame build, reached only once every rung above
			// has passed. Named here so the ladder's own last step is part
			// of the ordered walk rather than only of the builder's tests.
			name:       "6: an inexpressible mode is refused by the frame build",
			ch:         badMode(writableChannel("010")),
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
}

// withKnownTone and withKnownSkip mark the two protocol-unreachable fields
// Known, which is what makes the capability gate refuse them.
func withKnownTone(ch codeplug.Channel) codeplug.Channel {
	ch.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}
	return ch
}

func withKnownSkip(ch codeplug.Channel) codeplug.Channel {
	ch.Data.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
	return ch
}

// TestWriteChannel_KnownTagDisplayRefusedByTheGate is THE NAMED INVERSION's
// own test, and what it pins is WHICH refusal fires — not that one does.
//
// On the FT-710 a display value that cannot be honoured is refused inside
// buildWriteCommands, because that radio's MT frame has a mandatory display
// flag and sending the channel would have to invent one. This radio's
// combined form has no flag at all, so there is nothing to invent and
// nothing for a builder to refuse; the refusal instead comes from the
// CAPABILITY GATE, because requestedFields includes spec.FieldTagDisplay
// exactly when the state is Known and this driver's FieldTagDisplay Write
// support is spec.Unsupported in every profile and on every bank.
//
// The distinction is testable and is tested three ways: the refusal names
// exactly spec.FieldTagDisplay (not a build error's field, and not the
// six plain fields alongside it — those ARE writable on this profile, which
// is what makes the singleton meaningful); its Reason is the GATE's own
// sentence; and buildWriteCommand accepts the very same channel when called
// directly, proving the builder has no opinion about the field at all.
func TestWriteChannel_KnownTagDisplayRefusedByTheGate(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	for _, tt := range []struct {
		name  string
		value bool
	}{
		// Known-FALSE is as real a Known value as Known-true — "the radio
		// should show the frequency" is a decision this radio cannot be
		// told, exactly like its opposite.
		{"Known true", true},
		{"Known false", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("010")
			ch.Data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: tt.value}

			wre := refusedFields(t, sess, ch)
			if !slices.Equal(wre.Fields, []spec.Field{spec.FieldTagDisplay}) {
				t.Errorf("WriteRefusedError.Fields = %v, want exactly [%s]", wre.Fields, spec.FieldTagDisplay)
			}
			if want := "not write-Supported for this session"; !strings.Contains(wre.Reason, want) {
				t.Errorf("WriteRefusedError.Reason = %q, want the CAPABILITY GATE's reason (containing %q) — a build-time display refusal would be the FT-710's answer, and this radio has no display flag to refuse", wre.Reason, want)
			}

			// The builder's side of the same claim: it accepts the channel,
			// because it makes no TagDisplay decision whatsoever.
			if _, err := buildWriteCommand(catDialect, ch); err != nil {
				t.Errorf("buildWriteCommand = %v, want success — the combined form has no display flag, so a Known value is not the BUILDER's to refuse (the gate above is what refuses it)", err)
			}
		})
	}
}

// TestWriteChannel_UnavailableTagDisplayIsNotRefused is the inversion's
// positive half, and the one that would fail loudest if the FT-710's
// refusal were ever transplanted here: an Unavailable TagDisplay — the
// state EVERY channel this driver reads carries — must not be refused for
// that field, and the write must proceed to the wire.
//
// Unknown is included alongside it because both are non-Known and the
// FT-710's refusal covers both; either being refused here would make an
// ordinary FTdx10 write impossible.
func TestWriteChannel_UnavailableTagDisplayIsNotRefused(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	for _, tt := range []struct {
		name       string
		tagDisplay codeplug.BoolField
	}{
		{"Unavailable (what this driver's own read path produces)", codeplug.BoolField{State: codeplug.Unavailable}},
		{"Unknown", codeplug.BoolField{State: codeplug.Unknown}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("010")
			ch.Data.TagDisplay = tt.tagDisplay

			before := len(p.Transcript())
			res, err := sess.WriteChannel(testCtx(t), ch)
			if err != nil {
				t.Fatalf("WriteChannel = %v, want nil — a non-Known TagDisplay requests nothing and this frame has no field it could reach", err)
			}
			assertSteps(t, res, wantOneStep(true, true))

			// And the bytes are the SAME bytes either way: TagDisplay's
			// state cannot move a byte of a frame that has no place for it.
			got := p.Transcript()[before:]
			if len(got) != 1 || got[0] != wantSetFrame {
				t.Errorf("frames sent = %q, want exactly [%q] — the display state must move no byte", got, wantSetFrame)
			}
		})
	}
}

// TestWriteChannel_OutcomeTable is the one-step WriteResult's truth table:
// the three outcomes a single-frame write can reach, each with the exact
// step list it must report.
//
// Read it as a whole. The ONLY difference between "the radio refused this
// frame" and "we could not tell what happened to it" is the Sent flag — and
// the step is PRESENT in all three rows, because the write always intended
// exactly one frame. A caller that sees {MT, Sent:false} knows the slot's
// on-radio state is unverified; one that sees {MT, Sent:true,
// Confirmed:false} knows the frame went out and was refused.
func TestWriteChannel_OutcomeTable(t *testing.T) {
	t.Run("success: silence within the window is acceptance", func(t *testing.T) {
		// Fire-and-forget: on the ASSUMED acknowledgement convention (doc.go's
		// register entry "A SINGLE COMBINED MT SET SUFFICES", second half) a
		// CAT Set produces NO acknowledgement, so Confirmed means exactly "no
		// rejection arrived in the window" and never "the radio said yes"
		// (see driver.WriteStep). What this test pins is the FAKE's modelling
		// of that convention, not any real FTdx10's behaviour.
		_, sess := openSession(t, Simulated, slotImage{})

		res, err := sess.WriteChannel(testCtx(t), writableChannel("010"))
		if err != nil {
			t.Fatalf("WriteChannel = %v, want nil", err)
		}
		assertSteps(t, res, wantOneStep(true, true))
	})

	t.Run("rejected: Sent true, Confirmed false, error surfaced", func(t *testing.T) {
		// rejectSets makes the scripted radio answer "?;" to every combined
		// MT Set — the protocol's single unattributed NAK, and the only
		// negative answer a Set can draw.
		_, sess := openSession(t, Simulated, slotImage{rejectSets: true})

		res, err := sess.WriteChannel(testCtx(t), writableChannel("010"))
		if !errors.Is(err, cat.ErrRejected) {
			t.Fatalf("WriteChannel = %v, want errors.Is match against cat.ErrRejected", err)
		}
		if !strings.Contains(err.Error(), "010") {
			t.Errorf("error text %q does not name the slot", err.Error())
		}
		assertSteps(t, res, wantOneStep(true, false))
	})

	t.Run("transport failure: Sent stays false", func(t *testing.T) {
		// A transport-level failure is NOT a rejection: the host cannot
		// attribute the frame's fate at all, so Sent stays false and the
		// error, not the flags, carries the distinction.
		p := newRespondingPort(t, slotImage{})
		port := &failableWritePort{inner: p.Port()}

		opened, err := New(Simulated).Open(testCtx(t), port, testIdentity)
		if err != nil {
			t.Fatalf("Open: unexpected error: %v", err)
		}
		t.Cleanup(func() { _ = opened.Close() })
		port.armed.Store(true)

		res, err := opened.WriteChannel(testCtx(t), writableChannel("010"))
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
// milestone's write guard, asserted at the driver's own seam: on the
// RealHardware profile — which, while writeTrialsComplete is false, is the
// all-Unverified fail-safe (caps.go) — a perfectly valid, ordinary channel
// is refused before any wire traffic, with EVERY requested field named.
//
// This is why every other test in this file uses the Simulated profile: on
// RealHardware the capability gate refuses everything, so the choreography
// below it could never be exercised at all. Stating that here, as a test,
// is what keeps "the tests use Simulated" from looking like a convenience.
//
// The field list is the whole of requestedFields for an ordinary channel —
// six, in requestedFields' own order, tag_display absent because an
// Unavailable state requests nothing even here. The refusal naming all six
// (rather than stopping at the first) is deliberate: a user told only
// "frequency is not writable" would fix the frequency and try again.
func TestWriteChannel_RealHardwareProfileRefusesEveryRequestedField(t *testing.T) {
	p, sess := openSession(t, RealHardware, slotImage{})

	before := len(p.Transcript())
	wre := refusedFields(t, sess, writableChannel("010"))

	want := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
	}
	if !slices.Equal(wre.Fields, want) {
		t.Errorf("WriteRefusedError.Fields = %v, want %v — every requested field, in requestedFields' order", wre.Fields, want)
	}
	if got := p.Transcript(); len(got) != before {
		t.Errorf("a refused WriteChannel sent %d frames, want 0", len(got)-before)
	}

	// And the same channel IS accepted on Simulated, which is what makes
	// the refusal above attributable to the PROFILE rather than to the
	// channel being invalid in some way this test cannot see.
	_, sim := openSession(t, Simulated, slotImage{})
	if _, err := sim.WriteChannel(testCtx(t), writableChannel("010")); err != nil {
		t.Errorf("the same channel on the Simulated profile = %v, want nil — otherwise the refusal above proves nothing about the profile", err)
	}
}

// tierFieldsInOrder is the Icom tier's ten spec.Fields in ChannelData's
// declaration order — the order codeplug's tierAddedFieldFor uses and the
// order requestedFields must append them in.
//
// Spelt out rather than derived from requestedFields, so that the two are
// COMPARED rather than one being the other's echo. (This package's OWN
// copy, as the FT-710's and FTdx101's namesakes are theirs: unexported test
// helpers do not cross package boundaries, and these drivers import one
// another nowhere by the rule in doc.go.)
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

// withEveryTierFieldKnown marks all ten tier fields Known on data. The
// values are arbitrary and never reach a wire — this driver's capability
// map has no entry for any of the ten, so the gate refuses them all — but
// the STATE is what requestedFields keys on, so it must be Known.
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
// The channel is otherwise the ORDINARY FTdx10 channel this profile accepts,
// so the refusal is attributable to the one Known tier value and to nothing
// else. TxFrequency is this driver's chosen representative — the FT-710 pins
// ToneMode and the FTdx101 DTCSCode, so the three tests between them
// exercise three of the ten.
//
// The MECHANISM is a lookup MISS: this radio's capability map (caps.go's
// bankFields) has no entry for spec.FieldTxFrequency on any bank, so
// FieldSupport answers the ZERO spec.FieldSupport, which is neither CanWrite
// nor spec.Inert. Nothing had to be added to caps.go for the refusal to
// happen, and the first assertion below is what keeps that true.
func TestWriteChannel_KnownTierFieldRefusedBeforeWire(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	bank, ok := sess.bankFor("010")
	if !ok {
		t.Fatalf("bankFor(%q) found no bank — the fixture is wrong, not the gate", "010")
	}
	if fs := sess.caps.FieldSupport(bank, spec.FieldTxFrequency); fs.CanWrite() || fs.Write == spec.Inert {
		t.Fatalf("FieldSupport(%q, %s) = %+v, want the zero FieldSupport (no tier field is in this radio's capability map)", bank, spec.FieldTxFrequency, fs)
	}

	ch := writableChannel("010")
	ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_255_000}

	before := len(p.Transcript())
	wre := refusedFields(t, sess, ch)

	if !slices.Contains(wre.Fields, spec.FieldTxFrequency) {
		t.Errorf("WriteRefusedError.Fields = %v, want %s named — a refusal that does not name the field is not the contract", wre.Fields, spec.FieldTxFrequency)
	}
	if !strings.Contains(wre.Reason, "not write-Supported for this session") {
		t.Errorf("WriteRefusedError.Reason = %q, want the capability gate's own sentence", wre.Reason)
	}
	if wre.Slot != "010" {
		t.Errorf("WriteRefusedError.Slot = %q, want %q", wre.Slot, "010")
	}
	if got := p.Transcript(); len(got) != before {
		t.Errorf("a refused WriteChannel sent %d frames (%q), want 0 — the refusal must precede ALL wire traffic", len(got)-before, got[before:])
	}
}

// TestRequestedFields_MembershipAndOrder pins the gate's field set, which
// requestedFields' doc comment claims mirrors core/driver/ft710's — and
// through it codeplug.Diff's addedFields — EXACTLY. A comment is not a
// gate, and the two sides cannot import one another, so the mirror is held
// by both pinning the SAME shape.
//
// Order is part of the contract, not incidental: this slice is what the
// gate walks, and therefore the order in which a WriteRefusedError names
// fields (see the RealHardware test's six). TagDisplay sits seventh
// whenever it appears at all — after Tag, before the tone/skip conditionals.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	base := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
	}
	known := func(v bool) codeplug.BoolField {
		return codeplug.BoolField{State: codeplug.Known, Value: v}
	}

	for _, tt := range []struct {
		name string
		data codeplug.ChannelData
		want []spec.Field
	}{
		{
			// The ORDINARY FTdx10 channel, and the case that matters most
			// here: exactly the six, with tag_display ABSENT. If this row
			// ever grew a seventh entry, every write this driver can make
			// would be refused (FieldTagDisplay is Unsupported everywhere).
			name: "the ordinary FTdx10 channel: Unavailable display, Unknown tone and skip",
			data: codeplug.ChannelData{
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
			want: base,
		},
		{
			name: "TagDisplay Known true is requested, seventh",
			data: codeplug.ChannelData{
				TagDisplay: known(true),
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
			want: append(append([]spec.Field{}, base...), spec.FieldTagDisplay),
		},
		{
			// Known-FALSE is a Known value: requested, not omitted.
			name: "TagDisplay Known false is requested too",
			data: codeplug.ChannelData{
				TagDisplay: known(false),
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
			want: append(append([]spec.Field{}, base...), spec.FieldTagDisplay),
		},
		{
			name: "all three Known: tag_display, ctcss_tone, scan_skip in that order",
			data: codeplug.ChannelData{
				TagDisplay: known(true),
				CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: 670},
				ScanSkip:   known(true),
			},
			want: append(append([]spec.Field{}, base...), spec.FieldTagDisplay, spec.FieldCTCSSTone, spec.FieldScanSkip),
		},
		{
			// The zero ChannelData: no State is Known, so nothing
			// conditional is requested. (Such a channel is refused by the
			// FieldState checks a rung earlier — see the ladder test — but
			// this function must still answer honestly for it.)
			name: "the zero ChannelData requests only the six",
			data: codeplug.ChannelData{},
			want: base,
		},
		{
			// THE TIER EXTENSION, one field at a time: a Known TxFrequency is
			// REQUESTED, so the capability gate gets to see it. Before the fix
			// wave this row came back as the bare six and the value was
			// silently dropped.
			name: "a Known TxFrequency is requested, after the pre-tier set",
			data: codeplug.ChannelData{
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
				TxFreqHz:   codeplug.FreqField{State: codeplug.Known, Value: 14_255_000},
			},
			want: append(append([]spec.Field{}, base...), spec.FieldTxFrequency),
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
			want: append(append(append([]spec.Field{}, base...),
				spec.FieldTagDisplay, spec.FieldCTCSSTone, spec.FieldScanSkip),
				tierFieldsInOrder()...),
		},
		{
			// The ten alone, with every pre-tier conditional absent: the
			// declaration order is visible with nothing in front of it.
			name: "the ten tier fields alone keep ChannelData's declaration order",
			data: withEveryTierFieldKnown(codeplug.ChannelData{}),
			want: append(append([]spec.Field{}, base...), tierFieldsInOrder()...),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := requestedFields(tt.data); !slices.Equal(got, tt.want) {
				t.Errorf("requestedFields = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNameMaps_AreExactInverses walks read.go's wire-to-name maps against
// write.go's name-to-wire maps in both directions.
//
// Four hand-written maps describing two three-member vocabularies is
// exactly the shape that drifts: a spelling corrected on the read side and
// forgotten on the write side would make the write gate refuse a value the
// read path had just produced, and the failure would surface as a
// mysterious refusal rather than as a broken test. The cross-check is also
// what keeps the vocabularies tied to what this driver ADVERTISES — the
// third leg below checks both name sets against the capability data's own
// CTCSSStates and ShiftOptions, so a name that reaches no user's screen
// cannot sit in either map unnoticed.
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
		caps := CapabilitiesSimulated()
		for _, s := range caps.CTCSSStates {
			if _, ok := ctcssByName[s.Value]; !ok {
				t.Errorf("Capabilities.CTCSSStates advertises %q, which the write path cannot map to a wire byte", s.Value)
			}
		}
		if len(caps.CTCSSStates) != len(ctcssByName) {
			t.Errorf("Capabilities advertises %d CTCSS states, ctcssByName maps %d", len(caps.CTCSSStates), len(ctcssByName))
		}
		for _, s := range caps.ShiftOptions {
			if _, ok := shiftByName[s.Value]; !ok {
				t.Errorf("Capabilities.ShiftOptions advertises %q, which the write path cannot map to a wire byte", s.Value)
			}
		}
		if len(caps.ShiftOptions) != len(shiftByName) {
			t.Errorf("Capabilities advertises %d shift options, shiftByName maps %d", len(caps.ShiftOptions), len(shiftByName))
		}
	})
}

// TestMTSetSpec_IsFireAndForget pins the Set's transport spec, and each
// half of it is a hazard rather than a preference.
//
// ClassWrite, and no answer matcher: that is what selects transport's
// fire-and-forget path (before D2 it was the ABSENCE of a prefix). A
// spec pinning the ANSWER geometry read.go's mtSpec derives — the same "MT"
// prefix, the same exact 41 — would wait the whole read timeout for a reply
// a Set never produces, and then report a timeout for a write the radio had
// accepted.
//
// RetryReads 0: a write is never resent (transport safety obligation 2),
// and transport.Engine.Do refuses any write-class spec with a non-zero
// RetryReads outright. The contrast with mtSpec's RetryReads 1 is the whole
// point — a READ is idempotent and a WRITE is not.
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
	read, err := mtSpec(catDialect)
	if err != nil {
		t.Fatalf("mtSpec(catDialect) = %v", err)
	}
	if read.RetryReads == got.RetryReads {
		t.Errorf("the MT READ spec's RetryReads (%d) equals the SET spec's (%d) — a read is idempotent and a write is not, and the two specs must not have converged", read.RetryReads, got.RetryReads)
	}
}

// TestBuildWriteCommand_RefusesInexpressibleValues walks the builder's own
// checklist. Every refusal is typed (*driver.WriteRefusedError), names the
// field it is about where a field is to blame, and returns the ZERO Command
// alongside — a caller that ignored the error must not find a plausible
// frame in its hand.
//
// A free function taking the dialect, exactly like the FT-710's, so these
// cases cost no session: they are pure mapping decisions and none of them
// reaches the wire.
func TestBuildWriteCommand_RefusesInexpressibleValues(t *testing.T) {
	for _, tt := range []struct {
		name       string
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
			// PARSERS may accept the placeholder (its ASSUMED register entry
			// 4), and core/cat refuses to EMIT it. A channel asking for it
			// must be refused, not encoded.
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
			// The bound is THIS DIALECT'S 9990 Hz, and the check exists
			// ahead of the int -> int16 conversion so an absurd value
			// cannot wrap into a plausible one.
			name:       "a clarifier beyond this dialect's range",
			mutate:     func(d *codeplug.ChannelData) { d.ClarHz = 10_000 },
			wantFields: []spec.Field{spec.FieldClarifier},
			reasonHas:  "exceeds +/-9990 Hz",
		},
		{
			name:       "a clarifier beyond it in the other direction",
			mutate:     func(d *codeplug.ChannelData) { d.ClarHz = -99_999 },
			wantFields: []spec.Field{spec.FieldClarifier},
			reasonHas:  "exceeds +/-9990 Hz",
		},
		{
			// Inside the range, but not a multiple of the dialect's ASSUMED
			// 10 Hz step (its register entry ClarifierPolicy.StepHz): the
			// BUILDER refuses this
			// one, which is why the bound check above does not duplicate the
			// step rule.
			name:      "a clarifier off this dialect's step",
			mutate:    func(d *codeplug.ChannelData) { d.ClarHz = 155 },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			name:      "a tag longer than the 12-byte field",
			mutate:    func(d *codeplug.ChannelData) { d.Tag = "THIRTEEN CHAR" },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			// The command-injection defence: a ';' inside a tag would split
			// one frame into two on the wire.
			name:      "a tag carrying the frame terminator",
			mutate:    func(d *codeplug.ChannelData) { d.Tag = "A;MD2" },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			// The combined form pads to full width with TagFill and trims
			// trailing fill on the way back, so a tag ENDING in the fill
			// byte would not round-trip. The builder refuses it rather than
			// silently trimming.
			name:      "a tag ending in the fill byte",
			mutate:    func(d *codeplug.ChannelData) { d.Tag = "CQ " },
			reasonHas: "cannot encode the combined MT Set frame",
		},
		{
			name:      "a zero frequency",
			mutate:    func(d *codeplug.ChannelData) { d.FreqHz = 0 },
			reasonHas: "cannot encode the combined MT Set frame",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("010")
			tt.mutate(ch.Data)

			cmd, err := buildWriteCommand(catDialect, ch)
			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("buildWriteCommand = %v (%T), want a *driver.WriteRefusedError", err, err)
			}
			if !cmd.IsZero() {
				t.Error("buildWriteCommand returned a non-zero Command alongside its error")
			}
			if tt.wantFields != nil && !slices.Equal(wre.Fields, tt.wantFields) {
				t.Errorf("WriteRefusedError.Fields = %v, want %v", wre.Fields, tt.wantFields)
			}
			if !strings.Contains(wre.Reason, tt.reasonHas) {
				t.Errorf("WriteRefusedError.Reason = %q, want it to contain %q", wre.Reason, tt.reasonHas)
			}
			if wre.Slot != "010" {
				t.Errorf("WriteRefusedError.Slot = %q, want \"010\"", wre.Slot)
			}
		})
	}

	t.Run("an ungrammatical slot", func(t *testing.T) {
		ch := writableChannel("010")
		ch.Slot = "0X1"
		if _, err := buildWriteCommand(catDialect, ch); !errors.Is(err, driver.ErrWriteRefused) {
			t.Errorf("buildWriteCommand = %v, want a refusal", err)
		}
	})

	t.Run("a discovered 5xx slot, which no profile may write", func(t *testing.T) {
		// WriteChannel's capability gate refuses these first (discovered
		// banks are read-only on every profile — caps.go's readOnlyFields),
		// but the CODEC refuses them too: cat.Dialect's combined-MT write
		// policy admits memory and PMS slots only. Both layers, deliberately.
		ch := writableChannel("501")
		if _, err := buildWriteCommand(catDialect, ch); !errors.Is(err, driver.ErrWriteRefused) {
			t.Errorf("buildWriteCommand(\"501\") = %v, want a refusal", err)
		}
	})

	t.Run("the unmutated fixture builds", func(t *testing.T) {
		// Non-vacuity: every row above would pass against a builder that
		// refused everything.
		if _, err := buildWriteCommand(catDialect, writableChannel("010")); err != nil {
			t.Errorf("buildWriteCommand(writableChannel) = %v, want a frame", err)
		}
	})
}

// TestBuildWriteCommand_NoTagDisplayRefusalTakesPriority pins the named
// inversion as an ORDER property — the exact inverse of
// core/driver/ft710's TestBuildWriteCommands_NonKnownTagDisplayRefusedFirst,
// which asserts that a non-Known TagDisplay outranks every later check
// there.
//
// Here NO TagDisplay state may outrank anything, because the builder makes
// no TagDisplay decision at all: over one channel that is invalid in three
// ways the builder DOES check (mode, ctcss state, clarifier), the refusal
// must name MODE — the first of those three — for every one of the four
// display states. If the FT-710's refusal were ever transplanted into this
// function, the two non-Known rows would name tag_display instead and this
// test would say so.
func TestBuildWriteCommand_NoTagDisplayRefusalTakesPriority(t *testing.T) {
	multiplyInvalid := func() codeplug.Channel {
		ch := writableChannel("010")
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
		t.Run(tt.name, func(t *testing.T) {
			ch := multiplyInvalid()
			ch.Data.TagDisplay = tt.tagDisplay

			_, err := buildWriteCommand(catDialect, ch)
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

// TestBuildWriteCommand_P7IsTheFormConstant pins the combined Set's kind
// byte to cat.CombinedMTSetKind — the FORM's schema constant, "0: (Fixed)"
// — and pins that it is NOT read from the dialect's MWWriteKind.
//
// The two are EQUAL for core/cat/ftdx10 (its MWWriteKind is
// cat.CombinedMTSetKind, because this radio's MW P7 legend also reads
// "(Fixed)"), so an assertion against this dialect alone cannot tell the two
// sources apart. The second half therefore builds a PEER dialect identical
// in every respect that matters here EXCEPT its MW write kind, and requires
// the same success and the same P7: a write path that consulted MWWriteKind
// would be refused outright by BuildMTSetCombined, which validates P7
// against the form constant and does not rewrite it.
//
// Why it matters, since the byte is the same today: MT-Set P7 and MW-Set P7
// are two command-specific facts that coincide on this radio, and deriving
// one from the other is the PadByte conflation core/cat has already paid
// for once — see cat.CombinedMTSetKind's doc comment and
// TestBuildMTSetCombined_P7IsAFormConstantNotTheMWWriteKind. This driver
// sends no MW frame at all (doc.go), so it has no business consulting MW's
// kind.
func TestBuildWriteCommand_P7IsTheFormConstant(t *testing.T) {
	// The chart's own position: P7 is at index 22 of the frame (position
	// 23), which is core/cat/memdata.go's memKindOffset — written out here
	// because this is the CHART being asserted (respondingport_test.go's
	// mtAnswerFields records the same offsets for the read direction).
	const p7Index = 22

	cmd, err := buildWriteCommand(catDialect, writableChannel("010"))
	if err != nil {
		t.Fatalf("buildWriteCommand = %v, want a frame", err)
	}
	if got := cmd.Bytes()[p7Index]; got != cat.CombinedMTSetKind {
		t.Errorf("P7 = %q, want %q (cat.CombinedMTSetKind, the combined Set's fixed \"(Fixed)\" kind)", got, cat.CombinedMTSetKind)
	}

	// The coincidence, recorded so it cannot quietly become load-bearing.
	if catDialect.MWWriteKind() != cat.CombinedMTSetKind {
		t.Fatalf("core/cat/ftdx10's MWWriteKind() = %q, no longer equal to cat.CombinedMTSetKind — the peer-dialect leg below is now the ONLY thing pinning this driver's P7 source, and this driver's own dialect has become a discriminating case: assert it directly", catDialect.MWWriteKind())
	}

	// The discriminating case: a combined-form dialect whose MW write kind
	// is cat.KindMemory ('1'), which BuildMTSetCombined refuses as a P7.
	peer := cat.MustNewDialect(cat.DialectConfig{
		CATID:     "0761",
		ModeNames: map[cat.Mode]string{cat.ModeUnset: "-", cat.Mode('2'): "USB"},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 501, SixtyHi: 599,
			PMSPairs:      9,
			EmergencyWire: "EMG",
			NoneWire:      "000",
		},
		MT:          cat.MTPolicy{Form: cat.MTFormCombined, TagMaxBytes: 12, TagFill: ' '},
		Clarifier:   cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MWWriteKind: cat.KindMemory,
	})
	if peer.MWWriteKind() == cat.CombinedMTSetKind {
		t.Fatal("the peer dialect's MW write kind equals the form constant, so this leg proves nothing")
	}

	peerCmd, err := buildWriteCommand(peer, writableChannel("010"))
	if err != nil {
		t.Fatalf("buildWriteCommand on a dialect whose MW kind is %q = %v, want a frame — the combined Set's P7 is the FORM's constant, so a differing MW kind must be irrelevant here", peer.MWWriteKind(), err)
	}
	if got := peerCmd.Bytes()[p7Index]; got != cat.CombinedMTSetKind {
		t.Errorf("P7 on the peer dialect = %q, want %q", got, cat.CombinedMTSetKind)
	}
}

// TestWriteChannel_ConsentedSessionWrites: on a REAL-HARDWARE session
// built with WithConsentedUnverifiedWrites, the ordinary channel that
// TestWriteChannel_RealHardwareProfileRefusesEveryRequestedField sees
// refused is written instead — one MT Set, sent and unrejected.
//
// WHAT THIS PROVES, EXACTLY: the DRIVER-level capability gate opens
// (spec.ConsentedUnverified makes FieldSupport.CanWrite true, so
// WriteChannel's own gate passes the six requested fields), and the MT
// frame round-trips against this package's scripted peer. That is
// CHOREOGRAPHY ONLY.
//
// It is NOT the project's write-and-verify pair, and this test must not be
// read as claiming one. Verification — write, read back, compare — lives
// in core/clone/execute.go, one layer above every driver, and it is
// exercised with consented capabilities there, in its own task. Nothing
// here reads the slot back, and this driver's WriteChannel reports only
// sent/unrejected by design (see its doc comment).
func TestWriteChannel_ConsentedSessionWrites(t *testing.T) {
	p, sess := openSession(t, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())

	before := len(p.Transcript())
	res, err := sess.WriteChannel(testCtx(t), writableChannel("003"))
	if err != nil {
		t.Fatalf("WriteChannel = %v, want nil — the consented session's gate must open", err)
	}
	assertSteps(t, res, wantOneStep(true, true))

	got := p.Transcript()
	if len(got) != before+1 {
		t.Fatalf("the write sent %d frames, want exactly 1 (this radio's whole write choreography is one combined MT Set)", len(got)-before)
	}
	if frame := got[len(got)-1]; !strings.HasPrefix(frame, "MT003") {
		t.Errorf("the frame sent was %q, want a combined MT Set addressed to slot 003", frame)
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
	p, sess := openSession(t, RealHardware, slotImage{})

	before := len(p.Transcript())
	wre := refusedFields(t, sess, writableChannel("003"))

	want := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
	}
	if !slices.Equal(wre.Fields, want) {
		t.Errorf("WriteRefusedError.Fields = %v, want %v — every requested field, in requestedFields' order", wre.Fields, want)
	}
	if got := p.Transcript(); len(got) != before {
		t.Errorf("an unconsented, refused WriteChannel sent %d frames, want 0", len(got)-before)
	}
}

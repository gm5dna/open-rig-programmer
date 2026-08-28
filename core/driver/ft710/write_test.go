// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"errors"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// wantSteps builds the expected two-step MW-then-MT result for the four
// flags, in the fixed order this driver always reports: MW first, MT
// second. Every outcome-table assertion in this package goes through it,
// so the ORDER and the mnemonics are asserted alongside the flags rather
// than being spelt out (and possibly mis-spelt) at each site.
func wantSteps(mwSent, mwConfirmed, mtSent, mtConfirmed bool) []driver.WriteStep {
	return []driver.WriteStep{
		{Command: "MW", Sent: mwSent, Confirmed: mwConfirmed},
		{Command: "MT", Sent: mtSent, Confirmed: mtConfirmed},
	}
}

// assertSteps fails t unless res.Steps is exactly want, element for
// element and in order. driver.WriteResult is no longer comparable with
// == (its Steps slice), so this replaces the `res != want` form the four
// bools allowed.
func assertSteps(t *testing.T, res driver.WriteResult, want []driver.WriteStep) {
	t.Helper()
	if !slices.Equal(res.Steps, want) {
		t.Errorf("WriteResult.Steps = %+v, want %+v", res.Steps, want)
	}
}

// failableWritePort wraps a port and, once armed, fails every Write with a
// transport-level error: the outcome the write path CANNOT attribute,
// since the host has no way to tell whether the frame reached the radio.
//
// Arming happens after Open, deliberately — it keeps the fixture
// independent of how many exchanges the probe and discovery sequence
// happen to spend, which the exchange-numbered fault tests are not.
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

// writableChannel returns a fully-writable populated channel for slot:
// every FieldState-carrying field Unknown (nothing inexpressible
// requested), everything else set to plain, codec-expressible values.
func writableChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz:     14_250_000,
			Mode:       "USB",
			ClarHz:     0,
			CTCSS:      "OFF",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      "SIMPLEX",
			Tag:        "TEST",
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		},
	}
}

// TestWriteChannel_HappyPath (Simulated profile): MW then MT are sent and
// unrejected, and the fake's stored slot state shows the exact bytes the
// wire carried — memory slot AND PMS slot both with Kind '1'. HW-CONFIRMED
// 2026-07-13 (M5b write trials, docs/hardware-notes.md): this test
// formerly expected Kind '5' for a PMS write (the ASSUMED pairing) —
// hardware-refuted (a live PMS write carrying Kind '5' was REJECTED; see
// TestReadChannel_HWDerived_M5b_PMSKindLeniency and
// internal/fakeradio's TestHWDerived_MW_KindMatrix_M5b).
func TestWriteChannel_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		slot     string
		wantKind byte
	}{
		{"memory slot -> Kind '1'", "010", '1'},
		{"PMS slot -> Kind '1' (HW-CONFIRMED, was wrongly '5')", "P2U", '1'},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			radio, sess := openSession(t, Simulated)

			res, err := sess.WriteChannel(testCtx(t), writableChannel(tt.slot))
			if err != nil {
				t.Fatalf("WriteChannel: unexpected error: %v", err)
			}
			assertSteps(t, res, wantSteps(true, true, true, true))

			state, ok := radio.SlotState(tt.slot)
			if !ok {
				t.Fatalf("SlotState(%q): no state stored after WriteChannel", tt.slot)
			}
			if !state.Populated {
				t.Error("slot not marked Populated after MW")
			}
			if state.Freq != "014250000" {
				t.Errorf("stored Freq = %q, want \"014250000\"", state.Freq)
			}
			if state.Mode != '2' {
				t.Errorf("stored Mode = %q, want '2' (USB)", state.Mode)
			}
			if state.Kind != tt.wantKind {
				t.Errorf("stored Kind = %q, want %q (HW-CONFIRMED Kind-on-write pairing)", state.Kind, tt.wantKind)
			}
			if state.ClarSign != '+' || state.ClarMag != "0000" {
				t.Errorf("stored clarifier = %c%s, want +0000", state.ClarSign, state.ClarMag)
			}
			if state.CTCSS != '0' || state.Shift != '0' {
				t.Errorf("stored CTCSS/Shift = %q/%q, want '0'/'0'", state.CTCSS, state.Shift)
			}
			if state.Tag != "TEST" || !state.TagDisplay {
				t.Errorf("stored Tag/TagDisplay = %q/%v, want \"TEST\"/true", state.Tag, state.TagDisplay)
			}
		})
	}
}

// TestWriteChannel_InertClarifierTransmitted (M5b): the driver's
// defence-in-depth gate treats the clarifier's spec.Inert Write support
// as acceptable-to-TRANSMIT — a channel carrying a non-zero clarifier is
// NOT refused here, whatever its value: the radio provably ignores the
// field (HW-CONFIRMED 2026-07-13), and this layer lacks the baseline
// needed to tell a changed value from an unchanged one — that half of
// the Inert rule lives in codeplug.Diff (see spec.Inert). fakeradio
// mirrors the radio: the transmitted clarifier is accepted and stored as
// zeros.
func TestWriteChannel_InertClarifierTransmitted(t *testing.T) {
	radio, sess := openSession(t, Simulated)

	ch := writableChannel("010")
	ch.Data.ClarHz = 100
	ch.Data.RxClar = true

	res, err := sess.WriteChannel(testCtx(t), ch)
	if err != nil {
		t.Fatalf("WriteChannel (non-zero Inert clarifier): unexpected error: %v", err)
	}
	assertSteps(t, res, wantSteps(true, true, true, true))

	state, ok := radio.SlotState("010")
	if !ok || !state.Populated {
		t.Fatalf("SlotState(\"010\") = %+v, ok=%v, want populated after the write", state, ok)
	}
	if state.ClarSign != '+' || state.ClarMag != "0000" || state.RXClar || state.TXClar {
		t.Errorf("stored clarifier = %c%s rx=%v tx=%v, want +0000 false false (radio ignores the transmitted clarifier)", state.ClarSign, state.ClarMag, state.RXClar, state.TXClar)
	}
}

// TestWriteChannel_DelayedRejection: the MW frame draws a "?;" that
// arrives DELAYED but still inside the transport's error window
// (fakeradio FaultDelayedRejection). WriteChannel must report the MW step
// Sent (the frame went out) but NOT Confirmed, must not send MT at all,
// and must surface the rejection as a typed error.
//
// The MT step is still PRESENT, flags false: the write intended two
// frames, and the second never went out. That is the report the
// preallocated step list exists to make possible — an appended list could
// only omit MT, which is indistinguishable from a driver that never
// intended one.
//
// Exchange arithmetic (minimalImage pins it): Open produces AI0;=1,
// ID;=2, MR501;=3 (rejected: no 60m), MREMG;=4 (rejected: no EMG) — so
// WriteChannel's MW frame is exchange 5.
func TestWriteChannel_DelayedRejection(t *testing.T) {
	_, sess := openSession(t, Simulated,
		fakeradio.WithFactoryImage(minimalImage),
		fakeradio.WithFault(fakeradio.FaultDelayedRejection(5, 50*time.Millisecond)),
	)

	res, err := sess.WriteChannel(testCtx(t), writableChannel("010"))
	if !errors.Is(err, cat.ErrRejected) {
		t.Fatalf("WriteChannel = %v, want errors.Is match against cat.ErrRejected", err)
	}
	assertSteps(t, res, wantSteps(true, false, false, false))
}

// TestWriteChannel_OutcomeTable is E6's pin: the FOUR outcomes a
// two-frame write can reach, each with the exact step list it must
// report. The three rows below that end in an error are the partial
// outcomes the old four booleans encoded; they are carried over here
// unchanged in meaning, one row per pre-existing case, plus the MT
// rejection (which the driver's own tests never covered, only clone's)
// and the transport-ambiguous case.
//
// Read the table as a whole: the ONLY difference between "the radio
// refused this frame" and "we could not tell what happened to it" is the
// Sent flag, and the only difference between a refused frame and one that
// was never reached is which step carries it.
//
// Exchange arithmetic, minimalImage as above: Open spends 1..4, so
// WriteChannel's MW is exchange 5 and its MT exchange 6.
func TestWriteChannel_OutcomeTable(t *testing.T) {
	t.Run("success: both frames sent and unrejected", func(t *testing.T) {
		_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

		res, err := sess.WriteChannel(testCtx(t), writableChannel("010"))
		if err != nil {
			t.Fatalf("WriteChannel: unexpected error: %v", err)
		}
		assertSteps(t, res, wantSteps(true, true, true, true))
	})

	t.Run("MW rejected: MT never reached", func(t *testing.T) {
		_, sess := openSession(t, Simulated,
			fakeradio.WithFactoryImage(minimalImage),
			fakeradio.WithFault(fakeradio.FaultDelayedRejection(5, 50*time.Millisecond)),
		)

		res, err := sess.WriteChannel(testCtx(t), writableChannel("010"))
		if !errors.Is(err, cat.ErrRejected) {
			t.Fatalf("WriteChannel = %v, want errors.Is match against cat.ErrRejected", err)
		}
		assertSteps(t, res, wantSteps(true, false, false, false))
	})

	t.Run("MT rejected: MW already confirmed", func(t *testing.T) {
		// The dangerous outcome, and the reason the steps are per-frame:
		// the radio's memory HAS changed (MW landed and drew no rejection)
		// even though the write as a whole failed. Reported as a single
		// pair of "did it go" flags, that state would be invisible.
		_, sess := openSession(t, Simulated,
			fakeradio.WithFactoryImage(minimalImage),
			fakeradio.WithFault(fakeradio.FaultDelayedRejection(6, 50*time.Millisecond)),
		)

		res, err := sess.WriteChannel(testCtx(t), writableChannel("010"))
		if !errors.Is(err, cat.ErrRejected) {
			t.Fatalf("WriteChannel = %v, want errors.Is match against cat.ErrRejected", err)
		}
		assertSteps(t, res, wantSteps(true, true, true, false))
	})

	t.Run("transport-ambiguous MW: Sent stays false", func(t *testing.T) {
		// A transport-level failure is NOT a rejection: the host cannot
		// attribute the frame's fate at all, so Sent stays false — exactly
		// as it did before E6 — and the error, not the flags, carries the
		// distinction.
		r := fakeradio.New(fakeradio.WithFactoryImage(minimalImage))
		t.Cleanup(func() { _ = r.Close() })
		port := &failableWritePort{inner: r.Port()}

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
		assertSteps(t, res, wantSteps(false, false, false, false))
	})
}

// TestWriteChannel_RefusedBeforeBuild_StepsAreEmptyNotNil pins the shape
// of a refusal that happens before either frame is built: an EXPLICITLY
// EMPTY step list, never nil.
//
// The distinction is durable and user-visible, not internal: clone's
// journal marshals this list into an append-only audit file, where nil
// renders as `null` — an auditor would have to read that as "unknown",
// where the truth is the far stronger "no frame was ever built, so
// nothing whatever was attempted". len() cannot tell the two apart, so
// the nil check is explicit.
func TestWriteChannel_RefusedBeforeBuild_StepsAreEmptyNotNil(t *testing.T) {
	unknownDisplay := writableChannel("010")
	unknownDisplay.Data.TagDisplay = codeplug.BoolField{State: codeplug.Unknown}

	knownTone := writableChannel("010")
	knownTone.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}

	for _, tt := range []struct {
		name string
		ch   codeplug.Channel
	}{
		{"refused by the capability gate (Known tone)", knownTone},
		{"refused inside buildWriteCommands (non-Known TagDisplay)", unknownDisplay},
		{"refused as an erase (empty channel)", codeplug.Channel{Slot: "010"}},
		{"refused as an unknown slot", writableChannel("000")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

			res, err := sess.WriteChannel(testCtx(t), tt.ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel = %v, want errors.Is match against driver.ErrWriteRefused", err)
			}
			if res.Steps == nil {
				t.Fatal("WriteResult.Steps is nil, want an explicitly EMPTY slice (nil marshals as JSON null in the clone journal)")
			}
			if len(res.Steps) != 0 {
				t.Errorf("WriteResult.Steps = %+v, want empty — no frame was built, so no step was intended", res.Steps)
			}
		})
	}
}

// TestWriteChannel_RefusedBeforeWire is the defence-in-depth test: every
// refusal below must happen BEFORE any wire traffic at all — zero Write
// calls reach the port — and carry driver.ErrWriteRefused.
func TestWriteChannel_RefusedBeforeWire(t *testing.T) {
	knownTone := writableChannel("010")
	knownTone.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}

	knownSkip := writableChannel("010")
	knownSkip.Data.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}

	tests := []struct {
		name    string
		profile Profile
		ch      codeplug.Channel
	}{
		// The codec cannot express a tone or scan-skip write: a Known
		// value must be refused, never silently dropped.
		{"Known CTCSS tone (Simulated)", Simulated, knownTone},
		{"Known scan skip (Simulated)", Simulated, knownSkip},
		// The write gate still functions on the RealHardware profile
		// post-M5b-flip: a plain writable channel is now legitimately
		// accepted there (see TestWriteTrialsComplete_FlippedTrue_M5b),
		// so the honest safety assertion — replacing the old "any channel
		// on the Unverified profile is refused" pin — is that a channel
		// requesting something the verified profile does NOT support is
		// still refused before any wire traffic, exactly as on Simulated.
		{"Known CTCSS tone (RealHardware, post-flip)", RealHardware, knownTone},
		{"Known scan skip (RealHardware, post-flip)", RealHardware, knownSkip},
		// Erase (empty channel) is never expressible by this codec.
		{"erase via empty channel", Simulated, codeplug.Channel{Slot: "010"}},
		// Discovered banks are read-only.
		{"60m slot", Simulated, writableChannel("501")},
		// A slot outside every bank cannot be gated per-field, so it is
		// refused outright.
		{"slot in no bank", Simulated, writableChannel("000")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, sess := openCountingSession(t, tt.profile)

			baseline := cp.writes.Load()
			_, err := sess.WriteChannel(testCtx(t), tt.ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel = %v, want errors.Is match against driver.ErrWriteRefused", err)
			}
			if got := cp.writes.Load(); got != baseline {
				t.Errorf("refused WriteChannel produced %d wire writes, want 0 (refusal must precede ALL wire traffic)", got-baseline)
			}
		})
	}
}

// TestWriteChannel_RefusalNamesFields: the Known-tone refusal must name
// the offending field, so the clone service (and a human reading a log)
// can see exactly why.
func TestWriteChannel_RefusalNamesFields(t *testing.T) {
	_, sess := openSession(t, Simulated)

	ch := writableChannel("010")
	ch.Data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 670}

	_, err := sess.WriteChannel(testCtx(t), ch)
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel = %v, want a *driver.WriteRefusedError", err)
	}
	found := false
	for _, f := range wre.Fields {
		if f == "ctcss_tone" {
			found = true
		}
	}
	if !found {
		t.Errorf("WriteRefusedError.Fields = %v, want ctcss_tone named", wre.Fields)
	}
	if wre.Slot != "010" {
		t.Errorf("WriteRefusedError.Slot = %q, want \"010\"", wre.Slot)
	}
}

// TestWriteChannel_NonKnownTagDisplayRefusedBeforeWire is E1's
// defence-in-depth pin: a channel whose TagDisplay is not Known must be
// refused BEFORE any wire traffic, naming spec.FieldTagDisplay, in every
// non-Known shape.
//
// MT's display flag (P1) is mandatory — the frame has no "leave it alone"
// encoding — so the only alternatives to refusing are to invent a value or
// to skip the tag write entirely, and both would break codeplug's write
// rule that a non-Known field is never present on the wire.
//
// The zero BoolField is included deliberately: it is what a
// composite-literal ChannelData that simply forgets the field produces, and
// it must be refused rather than read as "off".
func TestWriteChannel_NonKnownTagDisplayRefusedBeforeWire(t *testing.T) {
	tests := []struct {
		name       string
		tagDisplay codeplug.BoolField
	}{
		{"Unknown", codeplug.BoolField{State: codeplug.Unknown}},
		{"Unavailable", codeplug.BoolField{State: codeplug.Unavailable}},
		{"zero value (State unset)", codeplug.BoolField{}},
		{"unrecognised State", codeplug.BoolField{State: codeplug.FieldState("maybe")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, sess := openCountingSession(t, Simulated)
			ch := writableChannel("010")
			ch.Data.TagDisplay = tt.tagDisplay

			baseline := cp.writes.Load()
			_, err := sess.WriteChannel(testCtx(t), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel = %v, want errors.Is match against driver.ErrWriteRefused", err)
			}
			if got := cp.writes.Load(); got != baseline {
				t.Errorf("refused WriteChannel produced %d wire writes, want 0 (refusal must precede ALL wire traffic)", got-baseline)
			}
			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("WriteChannel = %v, want a *driver.WriteRefusedError", err)
			}
			if len(wre.Fields) != 1 || wre.Fields[0] != spec.FieldTagDisplay {
				t.Errorf("WriteRefusedError.Fields = %v, want exactly [%s]", wre.Fields, spec.FieldTagDisplay)
			}
			if wre.Slot != "010" {
				t.Errorf("WriteRefusedError.Slot = %q, want %q", wre.Slot, "010")
			}
		})
	}
}

// TestWriteChannel_KnownTierFieldRefusedBeforeWire is the tier half of the
// gate's stated contract: "silently dropping a value the caller explicitly
// marked Known would be a lie" must hold for the ten fields the Icom tier
// added, not only for the three this driver's CAT codec has always known
// about.
//
// The channel is otherwise the ordinary writable one this profile ACCEPTS
// (see TestWriteChannel_HappyPath), so the refusal is attributable to the
// one Known tier value and to nothing else. ToneMode is this driver's
// chosen representative; the FTdx10 and FTdx101 pin a different field each,
// so the three tests between them exercise three of the ten.
//
// The MECHANISM is a lookup MISS, and the test says so directly: this
// radio's capability map has no entry for spec.FieldToneMode on any bank,
// FieldSupport therefore answers the ZERO spec.FieldSupport, and the zero
// Support is neither CanWrite nor spec.Inert — so the gate refuses. Nothing
// had to be added to caps.go for the refusal to happen, and the assertion
// below is what keeps that true.
func TestWriteChannel_KnownTierFieldRefusedBeforeWire(t *testing.T) {
	cp, sess := openCountingSession(t, Simulated)

	bank, ok := sess.bankFor("010")
	if !ok {
		t.Fatalf("bankFor(%q) found no bank — the fixture is wrong, not the gate", "010")
	}
	if fs := sess.caps.FieldSupport(bank, spec.FieldToneMode); fs.CanWrite() || fs.Write == spec.Inert {
		t.Fatalf("FieldSupport(%q, %s) = %+v, want the zero FieldSupport (no tier field is in this radio's capability map)", bank, spec.FieldToneMode, fs)
	}

	ch := writableChannel("010")
	ch.Data.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "TSQL"}

	baseline := cp.writes.Load()
	_, err := sess.WriteChannel(testCtx(t), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel = %v, want errors.Is match against driver.ErrWriteRefused", err)
	}
	if got := cp.writes.Load(); got != baseline {
		t.Errorf("refused WriteChannel produced %d wire writes, want 0 (refusal must precede ALL wire traffic)", got-baseline)
	}
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel = %v, want a *driver.WriteRefusedError", err)
	}
	if !slices.Contains(wre.Fields, spec.FieldToneMode) {
		t.Errorf("WriteRefusedError.Fields = %v, want %s named — a refusal that does not name the field is not the contract", wre.Fields, spec.FieldToneMode)
	}
	if !strings.Contains(wre.Reason, "not write-Supported for this session") {
		t.Errorf("WriteRefusedError.Reason = %q, want the capability gate's own sentence", wre.Reason)
	}
	if wre.Slot != "010" {
		t.Errorf("WriteRefusedError.Slot = %q, want %q", wre.Slot, "010")
	}
}

// TestBuildWriteCommands_NonKnownTagDisplayRefusedFirst pins the REFUSAL'S
// PLACEMENT, not merely its existence: buildWriteCommands must reject a
// non-Known TagDisplay before it maps any other field, so a channel that is
// wrong in several ways at once still names the one field whose failure
// mode would otherwise be a silently wrong byte on the wire.
//
// Both directions are pinned, because "first" is a claim about ORDER and a
// one-sided test cannot tell an ordering apart from a swallow: over one
// fixture that is invalid in three further ways the function checks LATER
// (mode, ctcss state, clarifier), a non-Known TagDisplay must outrank all
// three, and a KNOWN one must stand aside and let the next check —
// mode — surface its own refusal.
func TestBuildWriteCommands_NonKnownTagDisplayRefusedFirst(t *testing.T) {
	// multiplyInvalid is the shared fixture. writableChannel supplies a
	// Known-true TagDisplay, so only the sub-test that wants a non-Known one
	// touches the field: the two cases differ in that alone.
	multiplyInvalid := func() codeplug.Channel {
		ch := writableChannel("010")
		ch.Data.Mode = "NOT-A-MODE"
		ch.Data.CTCSS = "NOT-A-CTCSS-STATE"
		ch.Data.ClarHz = 999_999
		return ch
	}

	t.Run("non-Known TagDisplay outranks all three later checks", func(t *testing.T) {
		ch := multiplyInvalid()
		ch.Data.TagDisplay = codeplug.BoolField{State: codeplug.Unknown}

		_, _, err := buildWriteCommands(cat.FT710, ch)
		var wre *driver.WriteRefusedError
		if !errors.As(err, &wre) {
			t.Fatalf("buildWriteCommands = %v, want a *driver.WriteRefusedError", err)
		}
		if len(wre.Fields) != 1 || wre.Fields[0] != spec.FieldTagDisplay {
			t.Fatalf("WriteRefusedError.Fields = %v, want exactly [%s] — the TagDisplay check must run before every other field mapping", wre.Fields, spec.FieldTagDisplay)
		}
		if !strings.Contains(wre.Reason, "only a Known value is ever sent") {
			t.Errorf("WriteRefusedError.Reason = %q, want it to state the Known-only write rule", wre.Reason)
		}
	})

	t.Run("Known TagDisplay yields to the next check", func(t *testing.T) {
		// The converse, and the half that makes the first meaningful: the
		// top-of-function check is a PRIORITY, not a gate that swallows every
		// other diagnosis. With TagDisplay Known the same broken channel must
		// report the FIRST of the three later failures — mode — by its own
		// name, exactly as it did before E1 added the check above it.
		_, _, err := buildWriteCommands(cat.FT710, multiplyInvalid())
		var wre *driver.WriteRefusedError
		if !errors.As(err, &wre) {
			t.Fatalf("buildWriteCommands = %v, want a *driver.WriteRefusedError", err)
		}
		if len(wre.Fields) != 1 || wre.Fields[0] != spec.FieldMode {
			t.Fatalf("WriteRefusedError.Fields = %v, want exactly [%s] — a Known TagDisplay must not mask the next failure", wre.Fields, spec.FieldMode)
		}
		if want := `mode "NOT-A-MODE" is not a mode this radio supports`; wre.Reason != want {
			t.Errorf("WriteRefusedError.Reason = %q, want %q", wre.Reason, want)
		}
	})
}

// TestBuildWriteCommands_TagDisplayWireIdentity is E1's WIRE-IDENTITY bar,
// and the extension of Task 1's
// TestBuildWriteCommands_KnownTagDisplayReachesTheFrame, which it replaces:
// where that test proved only that a Known value reached the MT frame, this
// pins BOTH frames, for Known-true and Known-false alike, to exactly the
// bytes the pre-E1 plain `TagDisplay bool` put on the wire. Known-FALSE is
// the case worth stating: it is a real value, not an absence, and must be
// sent.
//
// The expectations are STRING LITERALS, deliberately, and this is the
// change of substance over the test it replaces: that one built its
// reference by calling cat.FT710.BuildMTSet itself, which is the code under
// test — a reference built by the same code cannot catch an error the two
// share, so it would have passed unchanged had the flip corrupted the
// display byte at its source. The literals are derived from the frame
// grammar instead, which makes them deterministic:
//
// MW Set, 28 bytes, offsets per core/cat/memdata.go — IDENTICAL in both
// cases, because cat.MemoryData carries no display field at all, so
// TagDisplay cannot reach this frame:
//
//	"MW" | slot "010" | freq %09d of 14_250_000 -> "014250000" |
//	clar sign '+' (ClarHz 0) | clar magnitude "0000" | RxClar '0' |
//	TxClar '0' | mode "USB" -> '2' | kind cat.FT710.MWWriteKind() =
//	KindMemory -> '1' | ctcss "OFF" -> '0' | P9 fixed "00" |
//	shift "SIMPLEX" -> '0' | ';'
//	= "MW010014250000+000000210000;"
//
// MT Set, short form, per core/cat/mt.go's BuildMTSet — the ONE byte
// TagDisplay controls:
//
//	"MT" | slot "010" | boolDigit(display) | tag "TEST" | ';'
//	boolDigit (core/cat/memdata.go): true -> '1', false -> '0'
//	true  = "MT0101TEST;"
//	false = "MT0100TEST;"
//
// The true-case pair is independently authenticated: they are the very
// bytes commit 6b84335's TestBuildWriteCommands_FT710ByteIdentity asserted
// BEFORE the type flip, over the same reference channel.
func TestBuildWriteCommands_TagDisplayWireIdentity(t *testing.T) {
	const wantMW = "MW010014250000+000000210000;"

	for _, tt := range []struct {
		name    string
		display bool
		wantMT  string
	}{
		{"Known true", true, "MT0101TEST;"},
		{"Known false", false, "MT0100TEST;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("010")
			ch.Data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: tt.display}

			mwCmd, mtCmd, err := buildWriteCommands(cat.FT710, ch)
			if err != nil {
				t.Fatalf("buildWriteCommands = %v, want success for a Known TagDisplay", err)
			}
			if got := string(mwCmd.Bytes()); got != wantMW {
				t.Errorf("MW frame = %q, want %q (TagDisplay's state must move no MW byte — the MW frame carries no display field)", got, wantMW)
			}
			if got := string(mtCmd.Bytes()); got != tt.wantMT {
				t.Errorf("MT frame = %q, want %q (the Known value must reach the wire unchanged, in P1 and nowhere else)", got, tt.wantMT)
			}
		})
	}
}

// tierFieldsInOrder is the Icom tier's ten spec.Fields in ChannelData's
// declaration order — the order codeplug's tierAddedFieldFor uses and the
// order requestedFields must append them in.
//
// Spelt out rather than derived from either side, so that the two are
// COMPARED rather than one being the other's echo: a reordering made in
// requestedFields alone would agree with a list built from requestedFields
// and disagree with this one.
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

// TestRequestedFields_MembershipAndOrder pins the driver gate's field set,
// which requestedFields' doc comment claims mirrors codeplug.Diff's
// addedFields EXACTLY. That claim was momentarily untrue mid-branch — Task 2
// gave addedFields its TagDisplay conditional while this side still listed
// the field unconditionally — and a comment is not a gate, so the shape is
// asserted rather than described.
//
// addedFields is unexported, so the mirror is held by the two sides pinning
// the SAME expectations in the same shape: this test is the deliberate twin
// of codeplug's TestAddedFields_MembershipAndOrder (diff_test.go), and a
// change made to one gate but not the other fails one of the pair.
//
// Order is part of the contract, not incidental: this slice is what
// WriteChannel's gate walks, and therefore the order in which a
// WriteRefusedError names fields. TagDisplay sits seventh whenever it
// appears at all — after Tag, before the tone/skip conditionals — exactly
// where it sat when it was unconditional. The TIER TEN follow all three,
// in ChannelData's declaration order, so no BlockReason a user has ever
// read is reordered by their arrival.
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
			// writableChannel's shape: only TagDisplay Known.
			name: "TagDisplay Known, tone and skip Unknown",
			data: codeplug.ChannelData{
				TagDisplay: known(true),
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
			want: append(append([]spec.Field{}, base...), spec.FieldTagDisplay),
		},
		{
			// Known-FALSE is a Known value: it is requested, not omitted.
			name: "TagDisplay Known false",
			data: codeplug.ChannelData{
				TagDisplay: known(false),
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
			want: append(append([]spec.Field{}, base...), spec.FieldTagDisplay),
		},
		{
			name: "TagDisplay Unknown drops out",
			data: codeplug.ChannelData{
				TagDisplay: codeplug.BoolField{State: codeplug.Unknown},
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
			want: base,
		},
		{
			name: "TagDisplay Unavailable drops out",
			data: codeplug.ChannelData{
				TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
			want: base,
		},
		{
			// The zero ChannelData: a composite literal that forgets every
			// FieldState field requests none of the three.
			name: "zero data requests only the six plain fields",
			data: codeplug.ChannelData{},
			want: base,
		},
		{
			// All three Known: the seventh/eighth/ninth positions in order.
			name: "all three Known keep TagDisplay seventh",
			data: codeplug.ChannelData{
				TagDisplay: known(true),
				CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: 670},
				ScanSkip:   known(true),
			},
			want: append(append([]spec.Field{}, base...), spec.FieldTagDisplay, spec.FieldCTCSSTone, spec.FieldScanSkip),
		},
		{
			// The case that would survive a wrongly-ordered conditional:
			// TagDisplay absent, its neighbours present, the gap closing
			// without disturbing what follows.
			name: "TagDisplay Unknown, tone and skip Known — order preserved around the gap",
			data: codeplug.ChannelData{
				TagDisplay: codeplug.BoolField{State: codeplug.Unknown},
				CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: 670},
				ScanSkip:   known(true),
			},
			want: append(append([]spec.Field{}, base...), spec.FieldCTCSSTone, spec.FieldScanSkip),
		},
		{
			// THE TIER EXTENSION, one field at a time: a Known ToneMode is
			// REQUESTED, so the capability gate gets to see it. Before the
			// fix wave this row came back as the bare six and the field was
			// silently omitted from the frame.
			name: "a Known ToneMode is requested, after the pre-tier set",
			data: codeplug.ChannelData{
				TagDisplay: codeplug.BoolField{State: codeplug.Unknown},
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
				ToneMode:   codeplug.StringField{State: codeplug.Known, Value: "TSQL"},
			},
			want: append(append([]spec.Field{}, base...), spec.FieldToneMode),
		},
		{
			// The tier ten never displace the pre-tier three: TagDisplay is
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
			got := requestedFields(tt.data)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("requestedFields = %v, want %v (membership AND order are the contract)", got, tt.want)
			}
		})
	}
}

// mwKindOffset is P7's 0-indexed offset in the 28-byte MW Set frame. It
// duplicates core/cat's own memKindOffset (memdata.go), which is unexported
// there — a driver test cannot see it, and the alternative (re-deriving the
// position by parsing) would test the parser rather than the byte that was
// emitted.
const mwKindOffset = 22

// receiverDialect builds an otherwise-FT710-equivalent cat.Dialect carrying
// the caller's MW write kind and clarifier policy — the two facts M9c-3
// task 9 moved out of write.go's literals and onto the receiver.
func receiverDialect(t *testing.T, mwKind byte, clar cat.ClarifierPolicy) cat.Dialect {
	t.Helper()
	cfg := ft710EquivalentConfig()
	cfg.MWWriteKind = mwKind
	cfg.Clarifier = clar
	d, err := cat.NewDialect(cfg)
	if err != nil {
		t.Fatalf("cat.NewDialect(MWWriteKind %q, clarifier %+v): unexpected error: %v", mwKind, clar, err)
	}
	return d
}

// TestBuildWriteCommands_MWKindComesFromTheReceiver (M9c-3 task 9): the MW
// frame's P7 byte is the DIALECT's declared write kind, not this package's
// own cat.KindMemory literal.
//
// The peer fixture declares cat.KindPMS — a value V11 accepts (see
// core/cat's TestMWWriteKind_PeerAcceptsWhatFT710Rejects) and the FT-710's
// own dialect rejects. Before this task write.go hardcoded cat.KindMemory
// into the MemoryData it built, so BuildMWSet refused the peer's own
// legitimate write outright: the driver wrote the FT-710's hardware finding
// onto whatever dialect it was handed.
func TestBuildWriteCommands_MWKindComesFromTheReceiver(t *testing.T) {
	ft710Clar := cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990}
	peer := receiverDialect(t, cat.KindPMS, ft710Clar)
	if peer.MWWriteKind() != cat.KindPMS {
		t.Fatalf("fixture MWWriteKind() = %q, want %q — the case needs a dialect whose write kind is NOT the FT-710's", peer.MWWriteKind(), cat.KindPMS)
	}

	for _, tt := range []struct {
		name    string
		dialect cat.Dialect
	}{
		{"FT-710 (KindMemory)", cat.FT710},
		{"peer declaring KindPMS", peer},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mwCmd, _, err := buildWriteCommands(tt.dialect, writableChannel("010"))
			if err != nil {
				t.Fatalf("buildWriteCommands: unexpected error: %v (the write path emitted a kind byte its receiver does not accept)", err)
			}
			frame := mwCmd.Bytes()
			if got, want := frame[mwKindOffset], tt.dialect.MWWriteKind(); got != want {
				t.Errorf("MW P7 = %q, want %q (the receiver's own MWWriteKind()); frame = %q", got, want, frame)
			}
		})
	}
}

// TestWriteChannel_ClarifierBoundComesFromTheReceiver (M9c-3 task 9): the
// pre-wire clarifier bounds check consults the DIALECT's clarifier policy,
// and its refusal text names that dialect's bound.
//
// The peer's range stops at 4990 Hz, so 5000 Hz is one step past it while
// sitting well inside the FT-710's +-9990. Before this task the comparison
// AND the "+/-9990" in the message were both hardcoded, so the peer's
// over-range value sailed past this check and was caught only downstream by
// BuildMWSet — refused, but as an opaque "cannot encode MW frame" naming no
// field. The refusal must still precede ALL wire traffic.
func TestWriteChannel_ClarifierBoundComesFromTheReceiver(t *testing.T) {
	cp, sess := openCountingSession(t, Simulated)
	sess.dialect = receiverDialect(t, cat.KindMemory, cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 4990})

	ch := writableChannel("010")
	ch.Data.ClarHz = 5000

	baseline := cp.writes.Load()
	_, err := sess.WriteChannel(testCtx(t), ch)
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel = %v, want a *driver.WriteRefusedError", err)
	}
	if want := "clarifier 5000 Hz exceeds +/-4990 Hz"; wre.Reason != want {
		t.Errorf("WriteRefusedError.Reason = %q, want %q (the receiver's own bound, not the FT-710's)", wre.Reason, want)
	}
	found := false
	for _, f := range wre.Fields {
		if f == spec.FieldClarifier {
			found = true
		}
	}
	if !found {
		t.Errorf("WriteRefusedError.Fields = %v, want %v named", wre.Fields, spec.FieldClarifier)
	}
	if got := cp.writes.Load(); got != baseline {
		t.Errorf("refused WriteChannel produced %d wire writes, want 0 (refusal must precede ALL wire traffic)", got-baseline)
	}
}

// TestBuildWriteCommands_FT710ByteIdentity is M9c-3's byte-identity bar for
// the write path: for one reference channel, the FT-710's MW and MT Set
// frames are exactly these bytes. Task 9 replaced two literals in
// buildWriteCommands with receiver accessors, and cat.FT710 declares
// precisely the values those literals carried — so not a byte may move.
func TestBuildWriteCommands_FT710ByteIdentity(t *testing.T) {
	mwCmd, mtCmd, err := buildWriteCommands(cat.FT710, writableChannel("010"))
	if err != nil {
		t.Fatalf("buildWriteCommands(cat.FT710): unexpected error: %v", err)
	}
	if got, want := string(mwCmd.Bytes()), "MW010014250000+000000210000;"; got != want {
		t.Errorf("MW frame = %q, want %q", got, want)
	}
	if got, want := string(mtCmd.Bytes()), "MT0101TEST;"; got != want {
		t.Errorf("MT frame = %q, want %q", got, want)
	}
}

// TestBuildWriteCommands_FT710ClarifierRefusalText is the other half of the
// byte-identity bar: interpolating the receiver's MaxAbsHz must render, for
// the FT-710's 9990, exactly the text the hardcoded "+/-9990" rendered.
func TestBuildWriteCommands_FT710ClarifierRefusalText(t *testing.T) {
	for _, tt := range []struct {
		name   string
		clarHz int
		want   string
	}{
		{"over the positive bound", 10000, "clarifier 10000 Hz exceeds +/-9990 Hz"},
		{"under the negative bound", -10000, "clarifier -10000 Hz exceeds +/-9990 Hz"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := writableChannel("010")
			ch.Data.ClarHz = tt.clarHz

			_, _, err := buildWriteCommands(cat.FT710, ch)
			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("buildWriteCommands = %v, want a *driver.WriteRefusedError", err)
			}
			if wre.Reason != tt.want {
				t.Errorf("WriteRefusedError.Reason = %q, want %q", wre.Reason, tt.want)
			}
		})
	}
}

// TestBuildWriteCommands_FrequencyTooWideForTheFrame exercises the ONE
// checked conversion between the neutral model's uint64 frequency and
// core/cat's uint32 (design D4, item 7).
//
// The path is UNREACHABLE in normal operation and that is the point of
// testing it here rather than trusting it: codeplug.Validate refuses any
// frequency above this radio's 75 MHz ceiling long before a write is
// planned, so nothing but a direct call gets a value this wide as far as
// the frame builder. What the test proves is that when one does arrive,
// the driver REFUSES it — naming the frequency field — instead of
// casting it to the plausible 14.25 MHz a bare uint32() would have
// produced and sending that to a radio.
func TestBuildWriteCommands_FrequencyTooWideForTheFrame(t *testing.T) {
	ch := writableChannel("010")
	ch.Data.FreqHz = uint64(1)<<32 | 14_250_000

	_, _, err := buildWriteCommands(cat.FT710, ch)
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("buildWriteCommands = %v, want a *driver.WriteRefusedError", err)
	}
	if len(wre.Fields) != 1 || wre.Fields[0] != spec.FieldFrequency {
		t.Fatalf("WriteRefusedError.Fields = %v, want exactly [%s]", wre.Fields, spec.FieldFrequency)
	}
	if !strings.Contains(wre.Reason, "too large for this protocol's memory frame") {
		t.Errorf("WriteRefusedError.Reason = %q, want it to say the frame cannot hold the value", wre.Reason)
	}
}

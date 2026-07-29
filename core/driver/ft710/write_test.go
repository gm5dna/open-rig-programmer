// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

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
			TagDisplay: true,
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
			want := driver.WriteResult{MWSent: true, MWConfirmed: true, MTSent: true, MTConfirmed: true}
			if res != want {
				t.Errorf("WriteResult = %+v, want %+v", res, want)
			}

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
	want := driver.WriteResult{MWSent: true, MWConfirmed: true, MTSent: true, MTConfirmed: true}
	if res != want {
		t.Errorf("WriteResult = %+v, want %+v", res, want)
	}

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
// (fakeradio FaultDelayedRejection). WriteChannel must report MWSent
// (the frame went out) but NOT MWConfirmed, must not send MT at all, and
// must surface the rejection as a typed error.
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
	want := driver.WriteResult{MWSent: true, MWConfirmed: false, MTSent: false, MTConfirmed: false}
	if res != want {
		t.Errorf("WriteResult = %+v, want %+v (MW sent but rejected; MT never attempted)", res, want)
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

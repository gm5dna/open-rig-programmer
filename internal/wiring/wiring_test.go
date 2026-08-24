// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx10"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx101"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
)

const testCtxTimeout = 30 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// TestNewRealDriver_HWVerifiedWriteSet is the post-M5b-flip rewrite of
// TestNewRealDriver_AllFieldsUnverified (task-11 brief §3, moved from
// cmd/rigprog/wiring_test.go by task-15's extraction). The old test
// pinned "the real wiring path can write NOTHING" — a safety assertion
// the M5b hardware trials deliberately retired (13/07/2026,
// docs/hardware-notes.md "M5b write trials"; writeTrialsComplete
// flipped with evidence). Its honest replacement, NOT a deletion: the
// driver the REAL wiring path constructs must be write-capable for
// EXACTLY the six hardware-verified fields and NOTHING else — an
// over-broad writable set here would arm writes the trials never
// verified. Real-radio writes are gated by the clone service's
// choreography (confirmation digest, firmware gate, per-slot
// write-then-verify) and internal/guards' import-graph pin, not by a
// capability veto. Asserted via realDrivers[DefaultModel] — task 39's
// model-keyed real-driver table's FT-710 entry, which is NewRealDriver
// itself and the exact constructor OpenRealSessionFor looks up — so no
// serial port is ever opened.
func TestNewRealDriver_HWVerifiedWriteSet(t *testing.T) {
	writable := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
		spec.FieldTagDisplay: true,
	}

	// consent false — the default path: this pins what the real wiring path
	// builds for a user who has consented to nothing.
	d := realDrivers[DefaultModel](false)
	caps := d.Capabilities()
	if len(caps.Banks) == 0 {
		t.Fatal("NewRealDriver().Capabilities() has zero banks — sanity check failed, the guard below would pass vacuously")
	}
	fieldsChecked := 0
	for _, b := range caps.Banks {
		if len(b.Fields) == 0 {
			t.Errorf("bank %s: zero fields — sanity check failed for this bank", b.ID)
		}
		for f, fs := range b.Fields {
			fieldsChecked++
			if got, want := fs.CanWrite(), writable[f]; got != want {
				t.Errorf("bank %s field %s: CanWrite()==%v, want %v — the real wiring path must be write-capable for exactly the M5b-verified field set", b.ID, f, got, want)
			}
		}
	}
	if fieldsChecked == 0 {
		t.Fatal("examined zero fields — sanity check failed, the guard above would pass vacuously")
	}
	// The clarifier must be Inert specifically (transmitted but ignored,
	// HW-CONFIRMED) — not merely "not writable".
	if fs := caps.FieldSupport(spec.BankMemory, spec.FieldClarifier); fs.Write != spec.Inert {
		t.Errorf("MEM FieldClarifier.Write = %s, want Inert", fs.Write)
	}
}

// TestOpenFakeSessionFor_EveryRegisteredModel exercises the fake wiring
// path end-to-end for EVERY model SupportedModels lists, one subtest each
// (M9c-5 E5: the table-driven rewrite of the old DefaultModel-only test).
// It closes the MISMATCHED-PAIRING gap the single-model version could not
// even express: fakeDrivers pairs a simulated-profile DRIVER with a fake
// RIG per model, TestRealAndFakeDriverTablesAgree checks only that the
// keys line up, and nothing before this test checked that the two halves
// of an entry describe the SAME radio. A second entry that paired one
// model's driver with another's rig would satisfy every other test in
// this file.
//
// The identity check is what catches it: the session's Identity().CATID
// is what the RIG answered when the DRIVER probed it, and it must equal
// the CAT ID that model's own driver declares in its static capabilities.
// A crossed pairing answers the wrong one (or fails the probe outright).
// Capabilities().Model is checked alongside it so a session cannot merely
// be well-formed — it must be THIS model's.
//
// Structure over content, deliberately — and M9c-6 task 6 cashed that
// promise: SupportedModels() returned TWO rows, and the FTdx10 got this
// whole check (its own subtest, its own crossed-pairing proof) by being
// registered, with no new test written. M9d-2 task 7 cashed it twice more:
// FOUR rows now, and the FTDX101D and FTDX101MP inherited the same subtest
// the same way.
//
// THE CROSSED-PAIRING LEG NOW GUARDS THE SIBLING PAIRING TOO, and that is
// the strongest thing this test does for M9d-2. The FTDX101D and FTDX101MP
// differ on the wire in the ID answer ALONE — same dialect config, same
// simulator type, same driver implementation — so a fakeDrivers row that
// paired the D's driver with fakedx101.NewMP's rig would produce a session
// that read, wrote and discovered flawlessly and was simply the wrong
// radio. Every other test in this file would pass. The identity leg catches
// it because caps.CATID comes from the DRIVER (0681 for the D) and
// Identity().CATID comes from what the RIG answered (0682 from an MP rig),
// and a swapped row makes those two disagree.
//
// The FTdx10 and FTdx101 subtests are the slowest things in this package by
// design: those drivers' Open probes the entire declared 5xx range plus EMG
// (~100 exchanges each) because neither radio has a verified discovery
// termination rule. Seconds per open are budgeted (M9c-6 and M9d-2 plans),
// and M9d-2 tripled the number of such opens here; nobody trims those walks
// to speed this up.
func TestOpenFakeSessionFor_EveryRegisteredModel(t *testing.T) {
	models := SupportedModels()
	if len(models) == 0 {
		t.Fatal("SupportedModels() is empty — this table would run zero cases and pass vacuously")
	}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			caps, err := StaticCapabilities(model)
			if err != nil {
				t.Fatalf("StaticCapabilities(%q): unexpected error: %v", model, err)
			}
			if caps.CATID == "" {
				t.Fatalf("StaticCapabilities(%q).CATID is empty — the identity check below would pass vacuously", model)
			}

			sess, closeAll, err := OpenFakeSessionFor(testCtx(t), model)
			if err != nil {
				t.Fatalf("OpenFakeSessionFor(%q): unexpected error: %v", model, err)
			}
			if sess == nil {
				t.Fatal("OpenFakeSessionFor: nil session with nil error")
			}
			if got := sess.Capabilities().Model; got != model {
				t.Errorf("session Capabilities().Model = %q, want %q", got, model)
			}
			if got := sess.Identity().CATID; got != caps.CATID {
				t.Errorf("Identity().CATID = %q, want %q — the fake rig answering this session is not %s's own", got, caps.CATID, model)
			}
			if err := closeAll(); err != nil {
				t.Errorf("closeAll: unexpected error: %v", err)
			}
		})
	}
}

// TestOpenFakeSessionFor_DefaultModelDefaultImage keeps the one assertion
// the table above cannot carry: the DEFAULT fakeradio image is ImageUK,
// HW-CONFIRMED 2026-07-13 to have no 5xx bank, so an FT-710 fake session
// opened with no FakeSessionOpts reports region "no-60m". That is a fact
// about internal/fakeradio's FT-710 simulator and its default image, not
// a property every registered model has (driver.RegionReporter is an
// OPTIONAL capability), so it stays model-specific rather than being
// forced into the table.
func TestOpenFakeSessionFor_DefaultModelDefaultImage(t *testing.T) {
	sess, closeAll, err := OpenFakeSessionFor(testCtx(t), DefaultModel)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor: unexpected error: %v", err)
	}
	t.Cleanup(func() {
		if err := closeAll(); err != nil {
			t.Errorf("closeAll: unexpected error: %v", err)
		}
	})
	region, ok := sess.(driver.RegionReporter)
	if !ok {
		t.Fatal("session does not implement driver.RegionReporter — sanity check failed")
	}
	if got := region.Region(); got != "no-60m" {
		t.Errorf("Region() = %q, want %q (default fakeradio image is ImageUK, HW-CONFIRMED 2026-07-13 to have no 5xx bank)", got, "no-60m")
	}
}

// ftdx10WritableChannel is the channel the round-trip test below sends: the
// ORDINARY FTdx10 channel, whose three FieldState-carrying fields hold
// exactly what core/driver/ftdx10's read path produces (TagDisplay
// UNAVAILABLE — that radio's combined memory record has no display flag —
// and tone/scan-skip Unknown, which its CAT surface does not carry).
//
// TagDisplay Unavailable is load-bearing, not incidental: a Known display
// value would be REFUSED by the driver's capability gate (the field's Write
// is Unsupported on every FTdx10 bank), so a fixture copied from the
// FT-710's Known-true one would exercise only the refusal path and never
// the ordinary write this test is for.
func ftdx10WritableChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 14_250_000,
			Mode:   "USB",
			// The CLARIFIER is here deliberately: see the round-trip
			// assertions for why a non-zero value with asymmetric Rx/Tx
			// flags is the interesting case for THIS model.
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

// TestOpenFakeSessionFor_FTdx10SimulatedWriteRoundTrip is the end-to-end
// write test M9c-6 task 2 DEFERRED to task 6: the FTdx10's MT-only write
// choreography, driven through the REGISTERED fake path
// (OpenFakeSessionFor) rather than a hand-built session, from probe to
// read-back.
//
// It can only live here. The choreography needs a Simulated-profile driver
// paired with internal/fakedx10, and that pairing exists in exactly one
// place repo-wide — fake.go's fakeDrivers entry, pinned there by
// internal/guards' TestSimulatedProfileTokensConfinement. core/driver/ftdx10's
// own write tests drive a scripted responder port, which proves the frames;
// this proves the WIRING: that a registered model's simulated profile,
// its fake rig and its write path actually compose.
//
// Four properties, in order:
//
//  1. IDENTITY. The session was probed and the rig answered as an FTdx10
//     ("0761", the driver's own declared CAT ID). A crossed table entry
//     would answer the FT-710's or fail Open outright.
//  2. READ of a POPULATED slot. Slot 001 is populated in fakedx10's default
//     image, and the read must produce this driver's documented field shape
//     — TagDisplay Unavailable (E1's first real Unavailable producer),
//     tone/skip Unknown — not merely a non-empty channel.
//  3. WRITE, into a slot the default image leaves EMPTY, so this is a
//     CREATE and not an overwrite of something already shaped correctly.
//     The result must be the ONE-step WriteResult this radio's choreography
//     declares: a single MT frame, Sent AND Confirmed. Two steps here would
//     mean somebody added the MW frame the FT-710 needs and this radio's
//     combined form makes redundant.
//  4. READ-BACK, field by field, INCLUDING THE CLARIFIER. The clarifier is
//     the one field whose FTdx10 Simulated behaviour deliberately diverges
//     from the FT-710's: over there Write is Inert on every profile — a
//     HARDWARE finding, that radio ACCEPTS clarifier bytes and reads back
//     zeros — and it is NOT borrowed here, because no FTdx10 has ever been
//     asked. This profile's claim is about the FAKE, which stores the value
//     and returns it byte-faithfully, and this assertion is what makes that
//     claim checkable rather than merely written down. If a Stage W session
//     ever shows a real FTdx10 ignoring the clarifier, the fix is a
//     per-profile Inert in the driver AND the same change in the fake —
//     and this test is where the second half gets noticed.
//
// The write is against the FAKE, and nothing here is evidence about any
// physical FTdx10: that model's RealHardware profile reports every Write
// Unverified while writeTrialsComplete is false, so the capability gate
// refuses before a frame is built. This test exercises the SIMULATED
// profile exactly as the FT-710's was pre-M5b.
func TestOpenFakeSessionFor_FTdx10SimulatedWriteRoundTrip(t *testing.T) {
	ctx := testCtx(t)

	sess, closeAll, err := OpenFakeSessionFor(ctx, FTdx10Model)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor(%q): unexpected error: %v", FTdx10Model, err)
	}
	t.Cleanup(func() {
		if err := closeAll(); err != nil {
			t.Errorf("closeAll: unexpected error: %v", err)
		}
	})

	// 1. Identity: the rig answered as this model's own radio.
	wantCATID := NewFTdx10RealDriver().Capabilities().CATID
	if wantCATID == "" {
		t.Fatal("the ftdx10 driver declares an empty CATID — the identity check below would pass vacuously")
	}
	if got := sess.Identity().CATID; got != wantCATID {
		t.Errorf("Identity().CATID = %q, want %q — the fake rig answering this session is not the FTdx10's own", got, wantCATID)
	}

	// 2. Read a slot the default image populates.
	const populated = "001"
	before, err := sess.ReadChannel(ctx, populated)
	if err != nil {
		t.Fatalf("ReadChannel(%q): unexpected error: %v", populated, err)
	}
	if before.Data == nil {
		t.Fatalf("ReadChannel(%q): Data is nil, want a populated channel (fakedx10's default image populates M-01)", populated)
	}
	if got := before.Data.TagDisplay.State; got != codeplug.Unavailable {
		t.Errorf("ReadChannel(%q): TagDisplay.State = %v, want Unavailable — this radio's combined memory record has no display flag", populated, got)
	}
	if got := before.Data.CTCSSTone.State; got != codeplug.Unknown {
		t.Errorf("ReadChannel(%q): CTCSSTone.State = %v, want Unknown", populated, got)
	}
	if got := before.Data.ScanSkip.State; got != codeplug.Unknown {
		t.Errorf("ReadChannel(%q): ScanSkip.State = %v, want Unknown", populated, got)
	}

	// 3. Write into a slot the default image leaves empty — a create.
	const target = "002"
	empty, err := sess.ReadChannel(ctx, target)
	if err != nil {
		t.Fatalf("ReadChannel(%q): unexpected error: %v", target, err)
	}
	if empty.Data != nil {
		t.Fatalf("ReadChannel(%q): Data = %+v, want nil — this test needs an EMPTY target slot, so the write below is a create", target, empty.Data)
	}

	ch := ftdx10WritableChannel(target)
	res, err := sess.WriteChannel(ctx, ch)
	if err != nil {
		t.Fatalf("WriteChannel(%q): unexpected error: %v (the Simulated profile must be write-capable against the fake)", target, err)
	}
	wantSteps := []driver.WriteStep{{Command: "MT", Sent: true, Confirmed: true}}
	if !reflect.DeepEqual(res.Steps, wantSteps) {
		t.Errorf("WriteResult.Steps = %+v, want %+v — this radio's whole write choreography is ONE combined MT frame", res.Steps, wantSteps)
	}

	// 4. Read it back, field by field, including the clarifier.
	after, err := sess.ReadChannel(ctx, target)
	if err != nil {
		t.Fatalf("ReadChannel(%q) after write: unexpected error: %v", target, err)
	}
	if after.Data == nil {
		t.Fatalf("ReadChannel(%q) after write: Data is nil, want the channel just written", target)
	}
	if after.Slot != target {
		t.Errorf("read-back Slot = %q, want %q", after.Slot, target)
	}
	sent := ch.Data
	got := after.Data
	if got.FreqHz != sent.FreqHz {
		t.Errorf("read-back FreqHz = %d, want %d", got.FreqHz, sent.FreqHz)
	}
	if got.Mode != sent.Mode {
		t.Errorf("read-back Mode = %q, want %q", got.Mode, sent.Mode)
	}
	if got.ClarHz != sent.ClarHz {
		t.Errorf("read-back ClarHz = %d, want %d — the FTdx10's Simulated clarifier is Supported, NOT the FT-710's Inert (that is an FT-710 hardware finding, deliberately not borrowed)", got.ClarHz, sent.ClarHz)
	}
	if got.RxClar != sent.RxClar || got.TxClar != sent.TxClar {
		t.Errorf("read-back RxClar/TxClar = %v/%v, want %v/%v — the two flags are independent and must not be collapsed", got.RxClar, got.TxClar, sent.RxClar, sent.TxClar)
	}
	if got.CTCSS != sent.CTCSS {
		t.Errorf("read-back CTCSS = %q, want %q", got.CTCSS, sent.CTCSS)
	}
	if got.Shift != sent.Shift {
		t.Errorf("read-back Shift = %q, want %q", got.Shift, sent.Shift)
	}
	if got.Tag != sent.Tag {
		t.Errorf("read-back Tag = %q, want %q (the combined form carries the tag in the SAME frame as the fields)", got.Tag, sent.Tag)
	}
	// The three FieldState fields are NOT round-tripped values: they are
	// what this radio's read path always reports, whatever was sent.
	if got.TagDisplay.State != codeplug.Unavailable {
		t.Errorf("read-back TagDisplay.State = %v, want Unavailable", got.TagDisplay.State)
	}
	if got.CTCSSTone.State != codeplug.Unknown {
		t.Errorf("read-back CTCSSTone.State = %v, want Unknown", got.CTCSSTone.State)
	}
	if got.ScanSkip.State != codeplug.Unknown {
		t.Errorf("read-back ScanSkip.State = %v, want Unknown", got.ScanSkip.State)
	}
}

// TestOpenFakeSessionFor_FTdx10OptionSourceIsItsOwn pins the OTHER half of
// M9c-5 E5's design, now that a second model exists to test it with: each
// fakeDrivers entry reads its OWN option source, inside its own newRadio
// closure, at CALL time.
//
// FTdx10FakeSessionOpts is set here to populate the fake's 5 MHz bank and
// its EMG channel — deliberately NOT expressible any other way, because
// fakedx10's default image has neither (its doc comment: a fake shipping a
// full inventory would make every "this slot is empty" assertion a fixture
// accident). Two things follow if the seam works, and both are asserted:
// the option reached the FTdx10's rig, and core/driver/ftdx10's discovery
// walk found what the option added and reported it as discovered BANKS in
// the session's own capabilities — the whole registered read path, through
// the same constructor a real "--fake --model FTdx10" invocation uses.
//
// The crossing this design prevents is a COMPILE error, not a test case:
// FakeSessionOpts is []fakeradio.Option and FTdx10FakeSessionOpts is
// []fakedx10.Option, so neither model's options can be applied to the
// other's rig even by mistake. That is why there is no "the FT-710 ignored
// it" assertion below — there is nothing to ignore.
func TestOpenFakeSessionFor_FTdx10OptionSourceIsItsOwn(t *testing.T) {
	prev := FTdx10FakeSessionOpts
	FTdx10FakeSessionOpts = []fakedx10.Option{fakedx10.With5xx(), fakedx10.WithEMG()}
	t.Cleanup(func() { FTdx10FakeSessionOpts = prev })

	sess, closeAll, err := OpenFakeSessionFor(testCtx(t), FTdx10Model)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor(%q): unexpected error: %v", FTdx10Model, err)
	}
	t.Cleanup(func() {
		if err := closeAll(); err != nil {
			t.Errorf("closeAll: unexpected error: %v", err)
		}
	})

	banks := map[spec.BankID][]string{}
	for _, b := range sess.Capabilities().Banks {
		banks[b.ID] = b.Slots
	}
	// The static banks must still be there — a discovered bank ADDS, never
	// replaces.
	if _, ok := banks[spec.BankMemory]; !ok {
		t.Errorf("session banks = %v, want the static MEM bank present alongside the discovered ones", banks)
	}
	// fakedx10's With5xx populates a deliberately SPARSE, non-contiguous
	// set, which is the fixture that would catch a discovery walk that
	// stopped at the first rejection or capped itself: the ceiling 599 is
	// only reachable by walking the whole declared range.
	got60m, ok := banks[spec.Bank60m]
	if !ok {
		t.Fatalf("session banks = %v, want a discovered 60m bank — FTdx10FakeSessionOpts (With5xx) did not reach this model's own fake rig", banks)
	}
	if !slices.Equal(got60m, []string{"501", "503", "599"}) {
		t.Errorf("discovered 60m slots = %v, want [501 503 599] — fakedx10's sparse fixture, in probe order (a truncated list means discovery terminated early)", got60m)
	}
	gotEMG, ok := banks[spec.BankEMG]
	if !ok {
		t.Fatalf("session banks = %v, want a discovered EMG bank — WithEMG did not reach this model's own fake rig", banks)
	}
	if !slices.Equal(gotEMG, []string{"EMG"}) {
		t.Errorf("discovered EMG slots = %v, want [EMG]", gotEMG)
	}
}

// ftdx101WritableChannel is the channel the FTdx101 round-trip tests below
// send. It is the FTdx10 fixture's shape for the same reason that radio's
// is: the FTdx101's combined MT record carries no display flag either, so
// TagDisplay must be Unavailable — a Known value would be REFUSED by the
// driver's capability gate (FieldTagDisplay's Write is Unsupported on every
// bank of every FTdx101 profile), and the test would exercise only the
// refusal path.
//
// The three FieldState fields are what this radio's read path always
// reports, whatever is sent: TagDisplay Unavailable (the frame has no such
// field), tone and scan skip Unknown (the frame carries a CTCSS STATE byte
// but no tone NUMBER and no skip flag).
//
// ONE FIXTURE FOR BOTH SIBLINGS, deliberately: the two radios share a
// dialect config and differ on the wire in the ID answer alone, so a
// per-model fixture would differ in nothing and imply a distinction that
// does not exist. What is NOT shared is the assertion — each model gets its
// own round trip, through its own registered pairing.
func ftdx101WritableChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 14_250_000,
			Mode:   "USB",
			// A non-zero clarifier with ASYMMETRIC Rx/Tx flags: the
			// interesting case, and the one that catches a read path
			// collapsing the two independent flags into one.
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

// TestOpenFakeSessionFor_FTdx101DSimulatedWriteRoundTrip is the end-to-end
// write test M9d-2 task 3 DEFERRED to this task: the FTDX101D's MT-only
// write choreography, driven through the REGISTERED fake path
// (OpenFakeSessionFor) rather than a hand-built session, probe to read-back.
//
// It can only live here. The choreography needs a Simulated-profile driver
// paired with internal/fakedx101, and that pairing exists in exactly one
// place repo-wide — fake.go's fakeDrivers entry, pinned there by
// internal/guards' TestSimulatedProfileTokensConfinement.
// core/driver/ftdx101's own write tests drive a scripted responder port,
// which proves the FRAMES; this proves the WIRING: that a registered model's
// simulated profile, its fake rig and its write path actually compose.
//
// The write is against the FAKE, and nothing here is evidence about any
// physical FTDX101D: that model's RealHardware profile reports every Write
// Unverified while writeTrialsCompleteD is false, so the capability gate
// refuses before a frame is built.
func TestOpenFakeSessionFor_FTdx101DSimulatedWriteRoundTrip(t *testing.T) {
	assertFTdx101WriteRoundTrip(t, FTdx101DModel, NewFTdx101DRealDriver())
}

// TestOpenFakeSessionFor_FTdx101MPSimulatedWriteRoundTrip is the same trip
// for the MP, and it is a SEPARATE test over a SEPARATE session for the
// reason the whole sibling pair keeps forcing: the two models are wired by
// two independent fakeDrivers rows, and a row that paired the MP's driver
// with the D's rig (or built the wrong constructor entirely) would leave
// this test green if it shared the D's session. The identity assertion
// inside is what catches that, and it needs its own open to make it.
func TestOpenFakeSessionFor_FTdx101MPSimulatedWriteRoundTrip(t *testing.T) {
	assertFTdx101WriteRoundTrip(t, FTdx101MPModel, NewFTdx101MPRealDriver())
}

// assertFTdx101WriteRoundTrip is the shared body of the two tests above.
// Four properties, in order:
//
//  1. IDENTITY. The session was probed and the rig answered as THIS model
//     ("0681" for the D, "0682" for the MP — the driver's own declared CAT
//     ID, read from realDriver here rather than written as a literal so the
//     assertion cannot drift from the driver). A crossed table entry answers
//     the SIBLING's ID, which is the whole of the difference between these
//     two radios on the wire, and fails here.
//  2. READ of a POPULATED slot. Slot 001 is populated in fakedx101's default
//     image, and the read must produce this driver's documented field shape
//     — TagDisplay Unavailable, tone/skip Unknown — not merely a non-empty
//     channel.
//  3. WRITE into a slot the default image leaves EMPTY, so this is a CREATE
//     and not an overwrite of something already shaped correctly. The result
//     must be the ONE-step WriteResult this radio's choreography declares: a
//     single MT frame, Sent AND Confirmed. Two steps would mean somebody
//     added the MW frame the FT-710 needs and this radio's combined form
//     makes redundant.
//  4. READ-BACK, field by field, INCLUDING THE CLARIFIER. The clarifier is
//     the field whose Simulated behaviour deliberately diverges from the
//     FT-710's: over there Write is Inert on every profile — a HARDWARE
//     finding, that radio ACCEPTS clarifier bytes and reads back zeros — and
//     it is NOT borrowed here, because no FTDX101 has ever been asked. This
//     profile's claim is about the FAKE, which stores the value and returns
//     it byte-faithfully, and this assertion makes that claim checkable
//     rather than merely written down.
func assertFTdx101WriteRoundTrip(t *testing.T, model string, realDriver driver.Driver) {
	t.Helper()
	ctx := testCtx(t)

	sess, closeAll, err := OpenFakeSessionFor(ctx, model)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor(%q): unexpected error: %v", model, err)
	}
	t.Cleanup(func() {
		if err := closeAll(); err != nil {
			t.Errorf("closeAll: unexpected error: %v", err)
		}
	})

	// 1. Identity: the rig answered as this model's own radio, not its
	// sibling's.
	wantCATID := realDriver.Capabilities().CATID
	if wantCATID == "" {
		t.Fatalf("the %s driver declares an empty CATID — the identity check below would pass vacuously", model)
	}
	if got := sess.Identity().CATID; got != wantCATID {
		t.Errorf("Identity().CATID = %q, want %q — the fake rig answering this session is not %s's own", got, wantCATID, model)
	}
	if got := sess.Capabilities().Model; got != model {
		t.Errorf("session Capabilities().Model = %q, want %q", got, model)
	}

	// 2. Read a slot the default image populates.
	const populated = "001"
	before, err := sess.ReadChannel(ctx, populated)
	if err != nil {
		t.Fatalf("ReadChannel(%q): unexpected error: %v", populated, err)
	}
	if before.Data == nil {
		t.Fatalf("ReadChannel(%q): Data is nil, want a populated channel (fakedx101's default image populates M-01)", populated)
	}
	if got := before.Data.TagDisplay.State; got != codeplug.Unavailable {
		t.Errorf("ReadChannel(%q): TagDisplay.State = %v, want Unavailable — this radio's combined memory record has no display flag", populated, got)
	}
	if got := before.Data.CTCSSTone.State; got != codeplug.Unknown {
		t.Errorf("ReadChannel(%q): CTCSSTone.State = %v, want Unknown", populated, got)
	}
	if got := before.Data.ScanSkip.State; got != codeplug.Unknown {
		t.Errorf("ReadChannel(%q): ScanSkip.State = %v, want Unknown", populated, got)
	}

	// 3. Write into a slot the default image leaves empty — a create.
	const target = "002"
	empty, err := sess.ReadChannel(ctx, target)
	if err != nil {
		t.Fatalf("ReadChannel(%q): unexpected error: %v", target, err)
	}
	if empty.Data != nil {
		t.Fatalf("ReadChannel(%q): Data = %+v, want nil — this test needs an EMPTY target slot, so the write below is a create", target, empty.Data)
	}

	ch := ftdx101WritableChannel(target)
	res, err := sess.WriteChannel(ctx, ch)
	if err != nil {
		t.Fatalf("WriteChannel(%q): unexpected error: %v (the Simulated profile must be write-capable against the fake)", target, err)
	}
	wantSteps := []driver.WriteStep{{Command: "MT", Sent: true, Confirmed: true}}
	if !reflect.DeepEqual(res.Steps, wantSteps) {
		t.Errorf("WriteResult.Steps = %+v, want %+v — this radio's whole write choreography is ONE combined MT frame", res.Steps, wantSteps)
	}

	// 4. Read it back, field by field, including the clarifier.
	after, err := sess.ReadChannel(ctx, target)
	if err != nil {
		t.Fatalf("ReadChannel(%q) after write: unexpected error: %v", target, err)
	}
	if after.Data == nil {
		t.Fatalf("ReadChannel(%q) after write: Data is nil, want the channel just written", target)
	}
	if after.Slot != target {
		t.Errorf("read-back Slot = %q, want %q", after.Slot, target)
	}
	sent := ch.Data
	got := after.Data
	if got.FreqHz != sent.FreqHz {
		t.Errorf("read-back FreqHz = %d, want %d", got.FreqHz, sent.FreqHz)
	}
	if got.Mode != sent.Mode {
		t.Errorf("read-back Mode = %q, want %q", got.Mode, sent.Mode)
	}
	if got.ClarHz != sent.ClarHz {
		t.Errorf("read-back ClarHz = %d, want %d — the FTdx101's Simulated clarifier is Supported, NOT the FT-710's Inert (that is an FT-710 hardware finding, deliberately not borrowed)", got.ClarHz, sent.ClarHz)
	}
	if got.RxClar != sent.RxClar || got.TxClar != sent.TxClar {
		t.Errorf("read-back RxClar/TxClar = %v/%v, want %v/%v — the two flags are independent and must not be collapsed", got.RxClar, got.TxClar, sent.RxClar, sent.TxClar)
	}
	if got.CTCSS != sent.CTCSS {
		t.Errorf("read-back CTCSS = %q, want %q", got.CTCSS, sent.CTCSS)
	}
	if got.Shift != sent.Shift {
		t.Errorf("read-back Shift = %q, want %q", got.Shift, sent.Shift)
	}
	if got.Tag != sent.Tag {
		t.Errorf("read-back Tag = %q, want %q (the combined form carries the tag in the SAME frame as the fields)", got.Tag, sent.Tag)
	}
	if got.TagDisplay.State != codeplug.Unavailable {
		t.Errorf("read-back TagDisplay.State = %v, want Unavailable", got.TagDisplay.State)
	}
	if got.CTCSSTone.State != codeplug.Unknown {
		t.Errorf("read-back CTCSSTone.State = %v, want Unknown", got.CTCSSTone.State)
	}
	if got.ScanSkip.State != codeplug.Unknown {
		t.Errorf("read-back ScanSkip.State = %v, want Unknown", got.ScanSkip.State)
	}
}

// TestOpenFakeSessionFor_FTdx101DOptionSourceIsItsOwn and its MP sibling pin
// M9c-5 E5's design for the ONE registered pairing where the compiler cannot
// pin it.
//
// For every other pair of models in this package the crossing is a BUILD
// error: FakeSessionOpts is []fakeradio.Option and FTdx10FakeSessionOpts is
// []fakedx10.Option, so neither can be applied to the other's rig even by
// mistake, and the FTdx10 test above says so explicitly. The FTdx101 pair
// breaks that: BOTH vars are []fakedx101.Option, because one simulator
// serves both radios. A fake.go closure reading the wrong variable would
// COMPILE and would silently steer the wrong model's session.
//
// So these two tests are the substitute for the type system, and they are
// stated as NON-INTERFERENCE rather than as reachability: set an option in
// ONE sibling's var, open the OTHER sibling, and assert the option did NOT
// arrive. Reachability is asserted alongside it — the option must reach the
// model whose var was set — because a "did not arrive" assertion alone would
// pass just as well against a seam that reached nothing at all.
//
// The fixture is With5xx() plus WithEMG(), for the same reason the FTdx10's
// is: fakedx101's default image has neither bank, so their PRESENCE is
// unambiguous evidence that the option reached that rig and this driver's
// discovery walk found what it added.
func TestOpenFakeSessionFor_FTdx101DOptionSourceIsItsOwn(t *testing.T) {
	prev := FTdx101DFakeSessionOpts
	FTdx101DFakeSessionOpts = []fakedx101.Option{fakedx101.With5xx(), fakedx101.WithEMG()}
	t.Cleanup(func() { FTdx101DFakeSessionOpts = prev })

	// Reached the D, which is the var that was set.
	assertFTdx101DiscoveredBanks(t, FTdx101DModel, true)
	// Did NOT reach the MP, whose own var is untouched.
	assertFTdx101DiscoveredBanks(t, FTdx101MPModel, false)
}

// TestOpenFakeSessionFor_FTdx101MPOptionSourceIsItsOwn is the mirror image,
// and both directions are tested because a closure that read
// FTdx101DFakeSessionOpts in BOTH rows would pass the D's test outright.
func TestOpenFakeSessionFor_FTdx101MPOptionSourceIsItsOwn(t *testing.T) {
	prev := FTdx101MPFakeSessionOpts
	FTdx101MPFakeSessionOpts = []fakedx101.Option{fakedx101.With5xx(), fakedx101.WithEMG()}
	t.Cleanup(func() { FTdx101MPFakeSessionOpts = prev })

	assertFTdx101DiscoveredBanks(t, FTdx101MPModel, true)
	assertFTdx101DiscoveredBanks(t, FTdx101DModel, false)
}

// assertFTdx101DiscoveredBanks opens model's registered fake session and
// asserts whether discovery found the 60m and EMG banks the With5xx/WithEMG
// options add.
//
// The static MEM bank is asserted present in BOTH directions: a discovered
// bank ADDS, never replaces, and a session that had lost its static banks
// would otherwise satisfy the want==false case for entirely the wrong
// reason.
//
// When want is true the 60m slot LIST is checked exactly, not merely its
// presence: fakedx101's With5xx populates a deliberately sparse,
// non-contiguous set (501, 503, 599), which is the fixture that catches a
// discovery walk that stopped at the first rejection or capped itself short
// of the declared ceiling.
func assertFTdx101DiscoveredBanks(t *testing.T, model string, want bool) {
	t.Helper()

	sess, closeAll, err := OpenFakeSessionFor(testCtx(t), model)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor(%q): unexpected error: %v", model, err)
	}
	t.Cleanup(func() {
		if err := closeAll(); err != nil {
			t.Errorf("closeAll(%q): unexpected error: %v", model, err)
		}
	})

	banks := map[spec.BankID][]string{}
	for _, b := range sess.Capabilities().Banks {
		banks[b.ID] = b.Slots
	}
	if _, ok := banks[spec.BankMemory]; !ok {
		t.Errorf("%s: session banks = %v, want the static MEM bank present whatever discovery found", model, banks)
	}

	got60m, has60m := banks[spec.Bank60m]
	_, hasEMG := banks[spec.BankEMG]
	if !want {
		if has60m || hasEMG {
			t.Errorf("%s: session banks = %v, want NO discovered 60m or EMG bank — another model's option source reached this model's fake rig, which is exactly the leakage the two typed vars exist to prevent (spec A6)", model, banks)
		}
		return
	}

	if !has60m {
		t.Fatalf("%s: session banks = %v, want a discovered 60m bank — this model's own option source did not reach its fake rig", model, banks)
	}
	if !slices.Equal(got60m, []string{"501", "503", "599"}) {
		t.Errorf("%s: discovered 60m slots = %v, want [501 503 599] — fakedx101's sparse fixture, in probe order (a truncated list means discovery terminated early)", model, got60m)
	}
	if !hasEMG {
		t.Errorf("%s: session banks = %v, want a discovered EMG bank — WithEMG did not reach this model's own fake rig", model, banks)
	}
}

// TestOpenRealSessionFor_BadPort confirms the real wiring path surfaces a
// port-open failure as a plain error (not a panic), for a path that
// cannot possibly exist.
func TestOpenRealSessionFor_BadPort(t *testing.T) {
	sess, closeAll, err := OpenRealSessionFor(testCtx(t), DefaultModel, "/dev/nonexistent-rigprog-test-port")
	if err == nil {
		t.Fatal("OpenRealSessionFor: expected an error opening a nonexistent port, got nil")
	}
	if sess != nil || closeAll != nil {
		t.Errorf("OpenRealSessionFor: expected nil session/closeAll on error, got sess=%v closeAllIsNil=%v", sess, closeAll == nil)
	}
}

// errSeamRefused is what the recording openSerial seam below returns
// instead of a port. The seam's job is to capture the transport.SerialConfig
// OpenRealSessionFor built and then stop the call dead: no port is opened,
// no driver session is attempted, and the test asserts against the captured
// config rather than against anything that touched hardware.
var errSeamRefused = errors.New("wiring test: the openSerial seam opens no port")

// recordSerialConfig swaps the package's openSerial seam for a recorder,
// restoring it on cleanup, and returns a pointer to the transport.SerialConfig
// the next OpenRealSessionFor call builds. It is the only way to observe that
// config at all — see openSerial's own doc comment for why transport cannot
// answer this question itself.
func recordSerialConfig(t *testing.T) *transport.SerialConfig {
	t.Helper()
	var got transport.SerialConfig
	prev := openSerial
	openSerial = func(_ string, cfg transport.SerialConfig) (transport.Port, error) {
		got = cfg
		return nil, errSeamRefused
	}
	t.Cleanup(func() { openSerial = prev })
	return &got
}

// baudFixtureDriver is a minimal driver.Driver carrying nothing but a
// canned spec.Capabilities: enough to be registered (Registry.Register
// runs Capabilities().Validate) and looked up, and never opened — the
// recording seam above fails before OpenRealSessionFor reaches Open.
type baudFixtureDriver struct{ caps spec.Capabilities }

func (d baudFixtureDriver) Model() string                   { return d.caps.Model }
func (d baudFixtureDriver) Capabilities() spec.Capabilities { return d.caps }
func (d baudFixtureDriver) Open(context.Context, transport.Port, driver.Identity) (driver.Session, error) {
	return nil, errors.New("baudFixtureDriver: Open is unreachable — the openSerial seam refuses first")
}

// TestOpenRealSessionFor_BaudIsTheDriversDefault pins E2's FT-710 half:
// the real wiring path opens the serial port at the DRIVER's own
// Capabilities().DefaultBaud, which for the FT-710 is 38400 — the same
// value transport.DefaultBaud carries, so this is a no-change pin for that
// model and the baseline the disagreeing-driver test below is measured
// against. The FTdx10 (M9c-6) and the FTDX101D and FTDX101MP (M9d-2) do not
// change that: every one of their DefaultBauds is 38400 too — an ASSUMED
// entry in each driver's own register, with its own named per-model lift,
// and NOT a coincidence to rely on — so all four registered models still
// agree with transport's default and only the fixture below can tell the
// two sources apart. Stop bits are asserted too, because they are the
// half that deliberately did NOT become model-derived (see the call
// site's recorded decision).
func TestOpenRealSessionFor_BaudIsTheDriversDefault(t *testing.T) {
	got := recordSerialConfig(t)

	_, _, err := OpenRealSessionFor(testCtx(t), DefaultModel, "/dev/nonexistent-rigprog-test-port")
	if !errors.Is(err, errSeamRefused) {
		t.Fatalf("OpenRealSessionFor: err = %v, want it to wrap the seam's own error (the seam must have been consulted)", err)
	}

	want := NewRealDriver().Capabilities().DefaultBaud
	if want != 38400 {
		t.Fatalf("sanity check failed: the FT-710 driver reports DefaultBaud %d, want 38400", want)
	}
	if got.Baud != want {
		t.Errorf("SerialConfig.Baud = %d, want %d (the driver's own DefaultBaud)", got.Baud, want)
	}
	if got.StopBits != transport.DefaultStopBits {
		t.Errorf("SerialConfig.StopBits = %d, want transport.DefaultStopBits (%d) — stop bits stay the fixed transport default by recorded decision", got.StopBits, transport.DefaultStopBits)
	}
}

// TestOpenRealSessionFor_BaudFollowsADisagreeingDriver is E2's actual
// proof, and the one the FT-710 alone cannot give: with a registered
// model whose DefaultBaud DISAGREES with transport.DefaultBaud, the port
// must be opened at the driver's 4800, not at transport's 38400. Before
// this milestone the call site passed transport.DefaultBaud outright, so
// this fixture would have been opened at the FT-710's rate — the exact
// failure a second registered model would have met.
func TestOpenRealSessionFor_BaudFollowsADisagreeingDriver(t *testing.T) {
	const fixtureModel = "TEST-BAUD-FIXTURE"
	const fixtureBaud = 4800
	if fixtureBaud == transport.DefaultBaud {
		t.Fatalf("sanity check failed: the fixture baud %d equals transport.DefaultBaud, so this test could not distinguish the two sources", fixtureBaud)
	}

	// The fixture ignores consent: what it exists to disagree about is the
	// baud, and a driver with no writable field has nothing to consent to.
	realDrivers[fixtureModel] = func(bool) driver.Driver {
		return baudFixtureDriver{caps: spec.Capabilities{
			Model:        fixtureModel,
			CATID:        "9999",
			Bauds:        []int{fixtureBaud},
			DefaultBaud:  fixtureBaud,
			TagLen:       12,
			ShiftOptions: spec.StandardShiftOptions(),
			CTCSSStates:  spec.StandardCTCSSStates(),
		}}
	}
	t.Cleanup(func() { delete(realDrivers, fixtureModel) })

	got := recordSerialConfig(t)

	_, _, err := OpenRealSessionFor(testCtx(t), fixtureModel, "/dev/nonexistent-rigprog-test-port")
	if !errors.Is(err, errSeamRefused) {
		t.Fatalf("OpenRealSessionFor: err = %v, want it to wrap the seam's own error", err)
	}
	if got.Baud != fixtureBaud {
		t.Errorf("SerialConfig.Baud = %d, want %d — the baud must come from the driver's own capabilities, not from transport.DefaultBaud (%d)", got.Baud, fixtureBaud, transport.DefaultBaud)
	}
}

// TestOpenRealSessionFor_UnknownModel confirms an unrecognised model fails
// with a typed *UnknownModelError BEFORE any port is touched — the error
// must name the supported list, not merely "unknown".
func TestOpenRealSessionFor_UnknownModel(t *testing.T) {
	sess, closeAll, err := OpenRealSessionFor(testCtx(t), "FT-NONEXISTENT", "/dev/nonexistent-rigprog-test-port")
	if sess != nil || closeAll != nil {
		t.Errorf("OpenRealSessionFor(unknown model): expected nil session/closeAll, got sess=%v closeAllIsNil=%v", sess, closeAll == nil)
	}
	var unknownErr *UnknownModelError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("OpenRealSessionFor(unknown model): err = %v, want *UnknownModelError", err)
	}
	if unknownErr.Model != "FT-NONEXISTENT" {
		t.Errorf("UnknownModelError.Model = %q, want %q", unknownErr.Model, "FT-NONEXISTENT")
	}
	if !reflect.DeepEqual(unknownErr.Supported, SupportedModels()) {
		t.Errorf("UnknownModelError.Supported = %v, want %v", unknownErr.Supported, SupportedModels())
	}
}

// TestSupportedModels_SortedNonEmpty pins SupportedModels' two structural
// guarantees: sorted order (so output is deterministic for a CLI listing
// or GUI picker) and non-empty (the FT-710 entry is always present).
func TestSupportedModels_SortedNonEmpty(t *testing.T) {
	got := SupportedModels()
	if len(got) == 0 {
		t.Fatal("SupportedModels() returned an empty slice, want at least DefaultModel")
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("SupportedModels() = %v, want sorted", got)
	}
	found := false
	for _, m := range got {
		if m == DefaultModel {
			found = true
		}
	}
	if !found {
		t.Errorf("SupportedModels() = %v, want it to contain DefaultModel %q", got, DefaultModel)
	}
}

// TestSupportedModels_ContainsEveryRegisteredModel is the PRESENCE PIN
// (M9c-6 task 6): SupportedModels() must name EVERY registered model, by
// literal string. FOUR since M9d-2 task 7, which registered the FTdx101D
// and the FTdx101MP.
//
// It exists because every other registration test in this file is
// N-SIDED and therefore blind to symmetric deletion.
// TestRealAndFakeDriverTablesAgree compares the two tables to each other;
// TestDriverTableKeysMatchDriverModel checks each key against its own
// driver; TestOpenFakeSessionFor_EveryRegisteredModel and
// TestEverySupportedModelHasRadiotext iterate whatever SupportedModels()
// happens to return. Delete the FTdx10 from realDrivers AND fakeDrivers —
// exactly what a careless merge conflict resolution or a "revert the
// registration" commit would do — and every one of them still passes,
// vacuously for the FTdx10 and truthfully for the FT-710. The model would
// silently stop existing: no CLI --model, no GUI listing, and not one red
// test.
//
// So the assertion is deliberately UNGENERALISED and by literal name: the
// list of models this build supports is a promise to users, and losing an
// entry must be a test failure, not a quiet reduction in scope. Each
// further model adds a line here (and inherits every structural test above
// for free) — the FTdx101D and the FTdx101MP added theirs at M9d-2 task 7.
//
// The SIBLING PAIR sharpens the point rather than merely lengthening the
// list. core/driver/ftdx101 drives both radios from one type, differing in
// a name and a CAT ID, so a registration that dropped ONE of the two — a
// copy-paste that left both fakeDrivers rows building NewD, say — would
// leave the surviving model working perfectly and every structural test
// green. Both names are here for that reason, and the crossed-pairing leg
// of TestOpenFakeSessionFor_EveryRegisteredModel is what catches the
// half-crossed case this pin cannot see.
//
// FTdx10Model, FTdx101DModel and FTdx101MPModel are spelt out as literals
// rather than used as the constants they are, precisely so that renaming or
// deleting a constant cannot make this test agree with the change.
func TestSupportedModels_ContainsEveryRegisteredModel(t *testing.T) {
	got := SupportedModels()
	for _, want := range []string{"FT-710", "FTdx10", "FTdx101D", "FTdx101MP"} {
		found := false
		for _, m := range got {
			if m == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("SupportedModels() = %v, want it to contain %q — a registered model must not silently disappear from the supported list (the structural table tests above all pass on symmetric removal)", got, want)
		}
	}
	// The constants must be the strings this test asserts, or a rename
	// would leave the pin above passing while every caller keyed off the
	// constant went somewhere else.
	if DefaultModel != "FT-710" {
		t.Errorf("DefaultModel = %q, want \"FT-710\"", DefaultModel)
	}
	if FTdx10Model != "FTdx10" {
		t.Errorf("FTdx10Model = %q, want \"FTdx10\"", FTdx10Model)
	}
	if FTdx101DModel != "FTdx101D" {
		t.Errorf("FTdx101DModel = %q, want \"FTdx101D\"", FTdx101DModel)
	}
	if FTdx101MPModel != "FTdx101MP" {
		t.Errorf("FTdx101MPModel = %q, want \"FTdx101MP\"", FTdx101MPModel)
	}
}

// Every model this package can open a real session against MUST have
// user-facing prose, or its CLI and GUI silently serve blank advisories
// (cmd/rigprog/write.go's erase procedure, probe.go's firmware note,
// app/uispec.go's grid legend all degrade to "" rather than failing).
// This is the M9c registration precondition: adding a driver without
// prose fails here rather than shipping.
//
// ok==true alone is not enough: a texts["SomeModel"] = radiotext.Text{}
// entry — every field blank — satisfies radiotext.For's ok return just
// as well as a properly populated one, which is exactly the silent-
// blank-advisory outcome this test exists to prevent. So this also
// requires a NAMED SUBSET of fields to be non-empty:
// EraseProcedure, FirmwareGuidance, ProbeFirmwareNote. Deliberately not
// the full set — ToneScanSkipVerification states what IS and is NOT
// hardware-verified about Tone/Scan Skip preservation for this radio,
// and for a model pinned at writeTrialsComplete=false it legitimately has
// nothing true to say yet; requiring it here would force either a false
// hardware claim or a registration failure for a model correctly
// awaiting its own M5b-equivalent trials.
//
// That exclusion stopped being hypothetical at M9c-6: the FTdx10 is
// registered, its writeTrialsComplete IS false, and its radiotext entry
// leaves ToneScanSkipVerification EMPTY for exactly the reason written
// above (internal/radiotext's TestRadiotext_FTdx10Verbatim asserts the
// emptiness, so the two tests agree rather than contradict). The three
// fields this test DOES require are populated for it, and none of them
// borrows a word of the FT-710's wording.
//
// THREE OF THE FOUR REGISTERED MODELS ARE NOW IN THAT POSITION. M9d-2
// registered the FTDX101D and FTDX101MP with writeTrialsCompleteD and
// writeTrialsCompleteMP both false, and both entries leave
// ToneScanSkipVerification empty on the same grounds
// (TestRadiotext_FTdx101DVerbatim and its MP sibling assert it). The
// exclusion is therefore the ordinary case for a newly registered radio and
// the FT-710's populated field is the exception — which is the right way
// round: a radio earns that sentence with write trials, it does not start
// with it.
func TestEverySupportedModelHasRadiotext(t *testing.T) {
	for _, model := range SupportedModels() {
		text, ok := radiotext.For(model)
		if !ok {
			t.Errorf("radiotext.For(%q) = _, false; every model in SupportedModels() must have prose", model)
			continue
		}
		if text.EraseProcedure == "" {
			t.Errorf("radiotext.For(%q).EraseProcedure is empty; every model must have prose", model)
		}
		if text.FirmwareGuidance == "" {
			t.Errorf("radiotext.For(%q).FirmwareGuidance is empty; every model must have prose", model)
		}
		if text.ProbeFirmwareNote == "" {
			t.Errorf("radiotext.For(%q).ProbeFirmwareNote is empty; every model must have prose", model)
		}
	}
}

// realDrivers and fakeDrivers must offer the same models: a model
// openable for real but not simulated (or vice versa) would fail only at
// the moment a user tried it.
func TestRealAndFakeDriverTablesAgree(t *testing.T) {
	for model := range realDrivers {
		if _, ok := fakeDrivers[model]; !ok {
			t.Errorf("model %q is in realDrivers but not fakeDrivers", model)
		}
	}
	for model := range fakeDrivers {
		if _, ok := realDrivers[model]; !ok {
			t.Errorf("model %q is in fakeDrivers but not realDrivers", model)
		}
	}
}

// Each table key must equal the driver's own Model(). StaticCapabilities
// registers a driver and then looks it up BY THE CALLER'S KEY; if the two
// disagreed the lookup would miss and the result would be nil.
func TestDriverTableKeysMatchDriverModel(t *testing.T) {
	for model, ctor := range realDrivers {
		// BOTH consent arms: a model's identity cannot depend on the user's
		// consent, and each row has two arms that could disagree about it —
		// a consented arm calling the sibling's constructor (NewD where NewMP
		// belongs) would build a perfectly valid driver for the WRONG radio,
		// and every capability assertion in this file would pass.
		for _, consent := range []bool{false, true} {
			if got := ctor(consent).Model(); got != model {
				t.Errorf("realDrivers[%q](consent=%v) builds a driver whose Model() = %q", model, consent, got)
			}
		}
	}
	for model, entry := range fakeDrivers {
		if got := entry.newDriver().Model(); got != model {
			t.Errorf("fakeDrivers[%q] builds a driver whose Model() = %q", model, got)
		}
	}
}

// TestStaticCapabilities_FT710EqualsDriver pins StaticCapabilities'
// equivalence to the table's own constructor: for DefaultModel, it must
// return exactly what realDrivers[DefaultModel](false).Capabilities() (i.e.
// NewRealDriver().Capabilities()) reports.
func TestStaticCapabilities_FT710EqualsDriver(t *testing.T) {
	got, err := StaticCapabilities(DefaultModel)
	if err != nil {
		t.Fatalf("StaticCapabilities(%q): unexpected error: %v", DefaultModel, err)
	}
	want := NewRealDriver().Capabilities()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StaticCapabilities(%q) != NewRealDriver().Capabilities()", DefaultModel)
	}
}

// TestStaticCapabilities_UnknownModel confirms an unrecognised model fails
// with a typed *UnknownModelError rather than a zero-value success.
func TestStaticCapabilities_UnknownModel(t *testing.T) {
	_, err := StaticCapabilities("FT-NONEXISTENT")
	var unknownErr *UnknownModelError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("StaticCapabilities(unknown model): err = %v, want *UnknownModelError", err)
	}
	if unknownErr.Model != "FT-NONEXISTENT" {
		t.Errorf("UnknownModelError.Model = %q, want %q", unknownErr.Model, "FT-NONEXISTENT")
	}
}

// TestStaticSettingsDescriptor_FT710 pins StaticSettingsDescriptor's
// equivalence to the table's own driver: for DefaultModel it must report
// present=true and the exact tree
// NewRealDriver().(driver.StaticSettingsProvider).StaticSettingsDescriptor()
// returns — the FT-710 driver implements the optional capability
// unconditionally of profile (task 37).
func TestStaticSettingsDescriptor_FT710(t *testing.T) {
	got, ok, err := StaticSettingsDescriptor(DefaultModel)
	if err != nil {
		t.Fatalf("StaticSettingsDescriptor(%q): unexpected error: %v", DefaultModel, err)
	}
	if !ok {
		t.Fatalf("StaticSettingsDescriptor(%q): ok = false, want true (the FT-710 driver implements driver.StaticSettingsProvider)", DefaultModel)
	}
	provider, providerOK := NewRealDriver().(driver.StaticSettingsProvider)
	if !providerOK {
		t.Fatal("NewRealDriver() does not implement driver.StaticSettingsProvider — sanity check failed")
	}
	want := provider.StaticSettingsDescriptor()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StaticSettingsDescriptor(%q) != driver's own StaticSettingsDescriptor()", DefaultModel)
	}
}

// TestStaticSettingsDescriptor_UnknownModel confirms an unrecognised model
// fails with a typed *UnknownModelError, distinct from the ok=false case a
// known-but-non-implementing model would report.
func TestStaticSettingsDescriptor_UnknownModel(t *testing.T) {
	_, ok, err := StaticSettingsDescriptor("FT-NONEXISTENT")
	if ok {
		t.Error("StaticSettingsDescriptor(unknown model): ok = true, want false")
	}
	var unknownErr *UnknownModelError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("StaticSettingsDescriptor(unknown model): err = %v, want *UnknownModelError", err)
	}
}

// TestSynthesiseDiscoveredBanks_UnknownModelFalse: an unrecognised model
// returns (nil, false) — SynthesiseDiscoveredBanks' signature carries no
// error return, so this is how it reports "no classification happened"
// for a model this package does not support at all.
func TestSynthesiseDiscoveredBanks_UnknownModelFalse(t *testing.T) {
	banks, ok := SynthesiseDiscoveredBanks("FT-NONEXISTENT", []string{"501", "502", "EMG"})
	if ok {
		t.Error("SynthesiseDiscoveredBanks(unknown model): ok = true, want false")
	}
	if banks != nil {
		t.Errorf("SynthesiseDiscoveredBanks(unknown model): banks = %#v, want nil", banks)
	}
}

// TestSynthesiseDiscoveredBanks_FT710MatchesDriver pins
// SynthesiseDiscoveredBanks' equivalence to the table's own driver: for
// DefaultModel it must report ok=true and the exact banks
// NewRealDriver().(driver.DiscoveredBankSynthesizer).SynthesiseDiscoveredBanks
// returns for the same slot list (the same fixture
// core/driver/ft710's TestSynthesiseDiscoveredBanks_MatchesLiveDiscovery
// uses: a 60m pair, EMG, and one unclassifiable slot).
func TestSynthesiseDiscoveredBanks_FT710MatchesDriver(t *testing.T) {
	slots := []string{"501", "502", "EMG", "0X1"}

	got, ok := SynthesiseDiscoveredBanks(DefaultModel, slots)
	if !ok {
		t.Fatalf("SynthesiseDiscoveredBanks(%q, ...): ok = false, want true (the FT-710 driver implements driver.DiscoveredBankSynthesizer)", DefaultModel)
	}

	synth, synthOK := NewRealDriver().(driver.DiscoveredBankSynthesizer)
	if !synthOK {
		t.Fatal("NewRealDriver() does not implement driver.DiscoveredBankSynthesizer — sanity check failed")
	}
	want := synth.SynthesiseDiscoveredBanks(slots)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SynthesiseDiscoveredBanks(%q, ...) = %#v,\nwant %#v (must equal the driver's own classification)", DefaultModel, got, want)
	}
}

// TestSynthesiseDiscoveredBanks_FTdx10MatchesDriver is the test above's
// sibling for the model M9c-6 registered, and it is not decoration: the
// capability it exercises is OPTIONAL, so a driver that does not implement
// it fails SILENTLY here — SynthesiseDiscoveredBanks returns ok=false, and
// app/ then renders no discovered banks at all for an offline FTdx10
// codeplug (data loaded, rows invisible, no error anywhere). ok=true is
// therefore the load-bearing assertion, and the DeepEqual behind it pins
// that the wiring-level answer is the DRIVER's own classification rather
// than a second implementation that could drift from live discovery.
//
// The same fixture shape as the FT-710's, with this radio's own slots: a
// 5xx pair, EMG, and one slot that classifies as neither. The two models
// need not agree on the RESULT — each driver classifies with its own
// dialect, and the FTdx10's collapses a repeated EMG where the FT-710's
// preserves it (core/driver/ftdx10's own documented divergence) — so what
// is asserted is equivalence to ITS driver, never equality between models.
func TestSynthesiseDiscoveredBanks_FTdx10MatchesDriver(t *testing.T) {
	slots := []string{"501", "599", "EMG", "0X1"}

	got, ok := SynthesiseDiscoveredBanks(FTdx10Model, slots)
	if !ok {
		t.Fatalf("SynthesiseDiscoveredBanks(%q, ...): ok = false, want true (the ftdx10 driver implements driver.DiscoveredBankSynthesizer — its absence would drop discovered banks from the GUI silently)", FTdx10Model)
	}

	synth, synthOK := NewFTdx10RealDriver().(driver.DiscoveredBankSynthesizer)
	if !synthOK {
		t.Fatal("NewFTdx10RealDriver() does not implement driver.DiscoveredBankSynthesizer — sanity check failed")
	}
	want := synth.SynthesiseDiscoveredBanks(slots)
	if len(want) == 0 {
		t.Fatal("the ftdx10 driver classified none of the fixture slots — this comparison would hold vacuously")
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SynthesiseDiscoveredBanks(%q, ...) = %#v,\nwant %#v (must equal the driver's own classification)", FTdx10Model, got, want)
	}
}

// TestSynthesiseDiscoveredBanks_FTdx101DMatchesDriver and its MP sibling are
// the FTdx10 test above's counterparts for the two models M9d-2 registered,
// and they carry the same load-bearing assertion for the same reason: the
// driver.DiscoveredBankSynthesizer capability is OPTIONAL, so a driver that
// does not implement it fails SILENTLY here — ok=false, and app/ then
// renders no discovered banks at all for an offline FTdx101 codeplug (data
// loaded, rows invisible, no error anywhere).
//
// TWO TESTS, NOT ONE PARAMETERISED OVER BOTH MODELS, and not a D-vs-MP
// equality proof either. The two radios really do classify identically —
// they share a dialect config, so they share a slot space — but asserting
// that they AGREE would be satisfied by both being wrong together, which is
// precisely the failure a sibling pair invites. Each is compared to ITS OWN
// driver's classification instead, which is the property that matters and
// the one that would survive the models diverging.
func TestSynthesiseDiscoveredBanks_FTdx101DMatchesDriver(t *testing.T) {
	assertSynthesiseMatchesDriver(t, FTdx101DModel, NewFTdx101DRealDriver())
}

// TestSynthesiseDiscoveredBanks_FTdx101MPMatchesDriver: see the D's doc
// comment. Separate because the MP's registration is a separate fact.
func TestSynthesiseDiscoveredBanks_FTdx101MPMatchesDriver(t *testing.T) {
	assertSynthesiseMatchesDriver(t, FTdx101MPModel, NewFTdx101MPRealDriver())
}

// assertSynthesiseMatchesDriver is the shared body of the two FTdx101
// synthesis tests: SynthesiseDiscoveredBanks(model, ...) must report ok=true
// and exactly what d's own DiscoveredBankSynthesizer returns for the same
// slot list. The fixture is the FTdx10 test's shape with this dialect's own
// slots: a 5xx pair, EMG, and one slot that classifies as neither.
func assertSynthesiseMatchesDriver(t *testing.T, model string, d driver.Driver) {
	t.Helper()
	slots := []string{"501", "599", "EMG", "0X1"}

	got, ok := SynthesiseDiscoveredBanks(model, slots)
	if !ok {
		t.Fatalf("SynthesiseDiscoveredBanks(%q, ...): ok = false, want true (the ftdx101 driver implements driver.DiscoveredBankSynthesizer — its absence would drop discovered banks from the GUI silently)", model)
	}

	synth, synthOK := d.(driver.DiscoveredBankSynthesizer)
	if !synthOK {
		t.Fatalf("the %s real driver does not implement driver.DiscoveredBankSynthesizer — sanity check failed", model)
	}
	want := synth.SynthesiseDiscoveredBanks(slots)
	if len(want) == 0 {
		t.Fatalf("the %s driver classified none of the fixture slots — this comparison would hold vacuously", model)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SynthesiseDiscoveredBanks(%q, ...) = %#v,\nwant %#v (must equal the driver's own classification)", model, got, want)
	}
}

// TestOpenFakeSessionFor_UnknownModel confirms OpenFakeSessionFor fails
// with a typed *UnknownModelError, naming the supported list, when asked
// for a model this package does not support — BEFORE any fake rig is
// constructed (a leaked fakeradio.Radio here would hang the test process
// on t.Cleanup-less exit).
func TestOpenFakeSessionFor_UnknownModel(t *testing.T) {
	sess, closeAll, err := OpenFakeSessionFor(testCtx(t), "FT-NONEXISTENT")
	if sess != nil || closeAll != nil {
		t.Errorf("OpenFakeSessionFor(unknown model): expected nil session/closeAll, got sess=%v closeAllIsNil=%v", sess, closeAll == nil)
	}
	var unknownErr *UnknownModelError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("OpenFakeSessionFor(unknown model): err = %v, want *UnknownModelError", err)
	}
	if unknownErr.Model != "FT-NONEXISTENT" {
		t.Errorf("UnknownModelError.Model = %q, want %q", unknownErr.Model, "FT-NONEXISTENT")
	}
	if !reflect.DeepEqual(unknownErr.Supported, SupportedModels()) {
		t.Errorf("UnknownModelError.Supported = %v, want %v", unknownErr.Supported, SupportedModels())
	}
}

// TestResolveSnapshotDir_Override pins the same --snapshot-dir override
// rule cmd/rigprog's own resolveSnapshotDir pins (fileio.go): given a
// non-empty override AND DefaultModel, return the override verbatim.
// internal/wiring's copy exists so app/ (which cannot import
// cmd/rigprog, a cmd-local package) has somewhere shared to get this
// 3-line UserConfigDir rule from, per task-15 brief §2's Connect
// bullet. Threaded with DefaultModel by task-7 (D9) so it keeps pinning
// the same override-passthrough property now that ResolveSnapshotDir is
// model-keyed — see TestResolveSnapshotDir_OtherModelGetsSubdir for the
// non-default-model override case.
func TestResolveSnapshotDir_Override(t *testing.T) {
	got, err := ResolveSnapshotDir("/tmp/some/override", DefaultModel)
	if err != nil {
		t.Fatalf("ResolveSnapshotDir(override): unexpected error: %v", err)
	}
	if got != "/tmp/some/override" {
		t.Errorf("ResolveSnapshotDir(override) = %q, want %q", got, "/tmp/some/override")
	}
}

// TestResolveSnapshotDir_Default pins the default:
// <UserConfigDir>/rigprog/snapshots — the same default cmd/rigprog uses,
// so a GUI snapshot/journal and a CLI one land in the same place absent
// an override. Threaded with DefaultModel by task-7 (D9): DefaultModel
// stays at this base directory unchanged, so every snapshot written
// before per-model subdirectories existed is still found.
func TestResolveSnapshotDir_Default(t *testing.T) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("os.UserConfigDir unavailable in this environment: %v", err)
	}
	got, err := ResolveSnapshotDir("", DefaultModel)
	if err != nil {
		t.Fatalf("ResolveSnapshotDir(\"\"): unexpected error: %v", err)
	}
	want := filepath.Join(cfgDir, "rigprog", "snapshots")
	if got != want {
		t.Errorf("ResolveSnapshotDir(\"\") = %q, want %q", got, want)
	}
}

// TestModelSlug pins ModelSlug's filesystem-safe-directory-component
// rule (task-7, D9): lowercase, with each run of non-alphanumeric
// characters collapsed to a single "-".
func TestModelSlug(t *testing.T) {
	for _, tc := range []struct{ model, want string }{
		{"FT-710", "ft-710"},
		{"FTdx10", "ftdx10"},
		// The two REGISTERED FTdx101 models. Their slugs must differ from
		// each other — each radio gets its own snapshot/journal directory,
		// and two siblings sharing one would be exactly the collision D9's
		// rule exists to prevent — and neither may collide with "ftdx10",
		// which is a PREFIX of both.
		{"FTdx101D", "ftdx101d"},
		{"FTdx101MP", "ftdx101mp"},
		// The JOINT inventory form internal/extable uses for the shared EX
		// profile. It is not a registered model and never reaches
		// ResolveSnapshotDir; it is here because it is the case that
		// exercises the "/" collapse, and because keeping it beside the two
		// real keys shows at a glance that the three are different strings.
		{"FTDX101D/MP", "ftdx101d-mp"},
		{"FTX-1", "ftx-1"},
	} {
		if got := ModelSlug(tc.model); got != tc.want {
			t.Errorf("ModelSlug(%q) = %q, want %q", tc.model, got, tc.want)
		}
	}
}

// TestResolveSnapshotDir_DefaultModelStaysAtRoot pins task-7's (D9) most
// important property: DefaultModel resolves to the base directory
// unchanged, byte-identical to the pre-task-7 behaviour, so every
// snapshot written before per-model subdirectories existed is still
// found.
func TestResolveSnapshotDir_DefaultModelStaysAtRoot(t *testing.T) {
	got, err := ResolveSnapshotDir("/tmp/snaps", DefaultModel)
	if err != nil {
		t.Fatalf("ResolveSnapshotDir: %v", err)
	}
	if got != "/tmp/snaps" {
		t.Errorf("ResolveSnapshotDir(override, DefaultModel) = %q, want %q unchanged — existing FT-710 snapshots must keep working", got, "/tmp/snaps")
	}
}

// TestResolveSnapshotDir_OtherModelGetsSubdir pins task-7's (D9)
// collision-avoidance rule: any model other than DefaultModel gets its
// own <base>/<model-slug>/ subdirectory — applied to an explicit
// override too, since two models sharing one named directory is exactly
// the collision this rule exists to prevent.
func TestResolveSnapshotDir_OtherModelGetsSubdir(t *testing.T) {
	got, err := ResolveSnapshotDir("/tmp/snaps", "FTdx10")
	if err != nil {
		t.Fatalf("ResolveSnapshotDir: %v", err)
	}
	if want := "/tmp/snaps/ftdx10"; got != want {
		t.Errorf("ResolveSnapshotDir(override, %q) = %q, want %q", "FTdx10", got, want)
	}
}

// TestResolveSnapshotDir_EmptySlugIsError pins fix-round-1's finding: a
// non-DefaultModel name that slugs to "" must not silently fall back to
// the base directory. filepath.Join drops empty elements, so
// filepath.Join(base, "") == base — without this guard such a model
// would collapse into exactly DefaultModel's own directory, precisely
// the collision this task exists to prevent, with no error raised.
func TestResolveSnapshotDir_EmptySlugIsError(t *testing.T) {
	for _, model := range []string{"", "---", "!!!", ".", ".."} {
		if got, err := ResolveSnapshotDir("/tmp/snaps", model); err == nil {
			t.Errorf("ResolveSnapshotDir(override, %q) = %q, <nil error>, want an error (empty slug must not silently collapse into DefaultModel's directory)", model, got)
		}
	}
}

// fakePortSeam points this package's openSerial seam at a live in-process
// fake rig for model, restoring the seam (and closing the rig) on cleanup.
//
// It is what makes a SESSION-LEVEL consent assertion possible at all. Consent
// is a statement about a session, never about a radio: the option leaves a
// driver's STATIC capabilities untouched and shows up only in the set Open
// assembles, so no static-capability assertion — however carefully written —
// can tell an option-bearing driver from a plain one. Observing the option
// therefore means actually opening a session, and opening one means answering
// the real driver's ID probe and its whole discovery walk. That is precisely
// what each model's simulator does.
//
// The rig is fake.go's OWN — entry.newRadio() from the very fakeDrivers
// table OpenFakeSessionFor looks up, so it is the same constructor and the
// same per-model option source, read at call time, BY SHARING rather than by
// a restatement here that could drift. The port these tests serve is
// therefore the port a "--fake --model X" invocation would get.
//
// newRadio ONLY, never entry.newDriver: the pairing under test is a
// REAL-HARDWARE driver (the one OpenRealSessionWith builds from realDrivers,
// carrying the consent option) against a fake RIG. Reaching for the fake
// half's simulated-profile driver would test fake.go's pairing over again
// and prove nothing about consent, whose whole subject is the real-hardware
// path.
func fakePortSeam(t *testing.T, model string) {
	t.Helper()
	entry, ok := fakeDrivers[model]
	if !ok {
		t.Fatalf("fakePortSeam: model %q has no fakeDrivers entry, so no rig can answer a real driver's probe", model)
	}
	r := entry.newRadio()
	t.Cleanup(func() { _ = r.Close() })

	prev := openSerial
	openSerial = func(string, transport.SerialConfig) (transport.Port, error) { return r.Port(), nil }
	t.Cleanup(func() { openSerial = prev })
}

// assertNoReadSideConsent fails if ANY field of caps carries
// spec.ConsentedUnverified on its READ side. Consent is a write-only state
// (spec.Capabilities.Validate rejects a read-side one outright), so this is
// the leg that catches a transform applied to the wrong half of a
// FieldSupport — a mistake that would otherwise look like a consented
// session working perfectly.
func assertNoReadSideConsent(t *testing.T, what string, caps spec.Capabilities) {
	t.Helper()
	for _, b := range caps.Banks {
		for f, fs := range b.Fields {
			if fs.Read == spec.ConsentedUnverified {
				t.Errorf("%s: bank %s field %s has Read = ConsentedUnverified — consent is a write-only state", what, b.ID, f)
			}
		}
	}
}

// assertNoConsentAnywhere fails if any field of caps carries
// spec.ConsentedUnverified on EITHER side.
func assertNoConsentAnywhere(t *testing.T, what string, caps spec.Capabilities) {
	t.Helper()
	for _, b := range caps.Banks {
		for f, fs := range b.Fields {
			if fs.Read == spec.ConsentedUnverified || fs.Write == spec.ConsentedUnverified {
				t.Errorf("%s: bank %s field %s = {Read: %s, Write: %s}, want no ConsentedUnverified anywhere", what, b.ID, f, fs.Read, fs.Write)
			}
		}
	}
}

// TestOpenRealSessionWith_ConsentedSessionCaps is this task's heart, and it
// runs at SESSION level for every consent-eligible model: with
// SessionOptions{ConsentUnverifiedWrites: true}, the session the real wiring
// path returns must carry the consent transform — MEM's frequency write,
// spec.Unverified in each of these radios' real-hardware baselines (no
// FTdx10, FTDX101D or FTDX101MP has been written to by this project), becomes
// spec.ConsentedUnverified — and must carry it on the write side only.
//
// EVERY consent-eligible model, one subtest each, deliberately: the three
// rows differ in which driver package they reach (ftdx10, and ftdx101 twice
// over two constructors), and a table that threaded the option through one
// row and dropped it in another would leave a user's recorded consent
// silently inert for that radio. The FT-710 is not here because its row's
// option is a proven no-op (its real-hardware set has no Unverified write to
// transform); core/driver/ft710's own tests own that proof, and
// TestRealDriverFor_DefaultPathByteIdentical below covers its default path.
func TestOpenRealSessionWith_ConsentedSessionCaps(t *testing.T) {
	for _, model := range []string{FTdx10Model, FTdx101DModel, FTdx101MPModel} {
		t.Run(model, func(t *testing.T) {
			fakePortSeam(t, model)

			sess, closeAll, err := OpenRealSessionWith(testCtx(t), model, "test-port", SessionOptions{ConsentUnverifiedWrites: true})
			if err != nil {
				t.Fatalf("OpenRealSessionWith(%q, consent): unexpected error: %v", model, err)
			}
			t.Cleanup(func() { _ = closeAll() })

			caps := sess.Capabilities()
			if got := caps.FieldSupport(spec.BankMemory, spec.FieldFrequency).Write; got != spec.ConsentedUnverified {
				t.Errorf("%s: session MEM FieldFrequency.Write = %s, want ConsentedUnverified — the consent option must reach the driver this table builds", model, got)
			}
			assertNoReadSideConsent(t, model+" consented session", caps)
		})
	}
}

// TestOpenRealSessionFor_DelegatesZeroOptions pins the delegation: the
// session OpenRealSessionFor returns must be indistinguishable from the one
// OpenRealSessionWith returns for zero options — same capability set, and no
// consent anywhere in it. The FTdx10 alone is enough here, because what is
// being pinned is the DELEGATION (one real implementation, one zero-option
// caller of it), not any per-model behaviour; the per-model leg is the test
// above.
func TestOpenRealSessionFor_DelegatesZeroOptions(t *testing.T) {
	fakePortSeam(t, FTdx10Model)
	viaFor, closeFor, err := OpenRealSessionFor(testCtx(t), FTdx10Model, "test-port")
	if err != nil {
		t.Fatalf("OpenRealSessionFor(%q): unexpected error: %v", FTdx10Model, err)
	}
	forCaps := viaFor.Capabilities()
	if err := closeFor(); err != nil {
		t.Errorf("closing the OpenRealSessionFor session: %v", err)
	}

	fakePortSeam(t, FTdx10Model)
	viaWith, closeWith, err := OpenRealSessionWith(testCtx(t), FTdx10Model, "test-port", SessionOptions{})
	if err != nil {
		t.Fatalf("OpenRealSessionWith(%q, zero options): unexpected error: %v", FTdx10Model, err)
	}
	withCaps := viaWith.Capabilities()
	if err := closeWith(); err != nil {
		t.Errorf("closing the OpenRealSessionWith session: %v", err)
	}

	if !reflect.DeepEqual(forCaps, withCaps) {
		t.Errorf("OpenRealSessionFor's session capabilities differ from OpenRealSessionWith(zero options)':\n for  = %#v\n with = %#v", forCaps, withCaps)
	}
	assertNoConsentAnywhere(t, "OpenRealSessionFor session", forCaps)
}

// TestRealDriverFor_DefaultPathByteIdentical is the no-change pin: with
// consent false — every existing caller's path, and OpenRealSessionFor's own
// — realDriverFor must build exactly what the pinned zero-argument
// constructors build, for all four registered models. It is what makes the
// table's new closure parameter safe to add: the default path is not merely
// "still working" but byte-identical to the one it replaced.
//
// IT COMPARES THE DRIVER VALUES, NOT THEIR Capabilities(), and that is the
// whole strength of it. The consent option deliberately leaves static
// capabilities untouched, so a capability comparison here would be BLIND in
// exactly the direction that matters: a false arm that leaked
// WithConsentedUnverifiedWrites() into a row would hand a user who consented
// to nothing a consented SESSION, while every capability assertion in this
// file stayed green. Each driver struct carries its consent flag as a plain
// field (consentUnverifiedWrites, nil transportLogger on both sides, and a
// cat.Dialect of pure data), so reflect.DeepEqual over the constructed values
// sees the leak directly — and sees it for ALL FOUR models, where the
// session-level delegation test can only afford one.
func TestRealDriverFor_DefaultPathByteIdentical(t *testing.T) {
	for _, tc := range []struct {
		model string
		want  func() driver.Driver
	}{
		{DefaultModel, NewRealDriver},
		{FTdx10Model, NewFTdx10RealDriver},
		{FTdx101DModel, NewFTdx101DRealDriver},
		{FTdx101MPModel, NewFTdx101MPRealDriver},
	} {
		got, err := realDriverFor(tc.model, false)
		if err != nil {
			t.Fatalf("realDriverFor(%q, false): unexpected error: %v", tc.model, err)
		}
		want := tc.want()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("realDriverFor(%q, false) != the pinned constructor's driver:\n got  = %#v\n want = %#v\nthe default path must be byte-identical — an option leaked into a consent-false arm would consent on a user's behalf", tc.model, got, want)
		}
		// Belt and braces: the capability sets must agree too. A future
		// driver whose fields DeepEqual cannot compare (a func-typed member,
		// say) would make the check above vacuously strict or vacuously
		// loose; this leg keeps the original assertion standing either way.
		if !reflect.DeepEqual(got.Capabilities(), want.Capabilities()) {
			t.Errorf("realDriverFor(%q, false).Capabilities() differs from the pinned constructor's", tc.model)
		}
	}
}

// TestRealDriverFor_StaticNeverConsented is the REGISTRY-boundary companion
// to the session test above, not a second proof of the option: even with
// consent true, a driver's STATIC capabilities carry no ConsentedUnverified
// anywhere. Static surfaces describe the radio — what the app names the model
// with, what offline synthesis classifies against — and a consent transform
// leaking into them would restate one user's decision as a fact about the
// hardware.
func TestRealDriverFor_StaticNeverConsented(t *testing.T) {
	for _, model := range SupportedModels() {
		d, err := realDriverFor(model, true)
		if err != nil {
			t.Fatalf("realDriverFor(%q, true): unexpected error: %v", model, err)
		}
		assertNoConsentAnywhere(t, "realDriverFor("+model+", true) static capabilities", d.Capabilities())
	}
}

// TestModelSlugsUnique pins what ResolveSnapshotDir's per-model directory
// rule silently assumes: every registered model slugs to a DISTINCT,
// non-empty directory component. Two models sharing a slug would share a
// snapshot/journal directory — one radio's saved codeplugs offered against
// another's — and a model slugging to "" would collapse onto DefaultModel's
// own base directory (ResolveSnapshotDir refuses that at call time; this
// catches it at registration time, where the fix belongs).
func TestModelSlugsUnique(t *testing.T) {
	seen := make(map[string]string, len(realDrivers))
	for _, model := range SupportedModels() {
		slug := ModelSlug(model)
		if slug == "" {
			t.Errorf("ModelSlug(%q) = \"\" — a registered model must slug to a non-empty directory component", model)
			continue
		}
		if other, dup := seen[slug]; dup {
			t.Errorf("ModelSlug(%q) == ModelSlug(%q) == %q — two registered models would share one snapshot directory", model, other, slug)
			continue
		}
		seen[slug] = model
	}
}

// TestNeedsUnverifiedConsent_PerModel pins the shared eligibility
// predicate, model by model, against the fact it is derived from: a
// radio is consent-eligible exactly when its REAL-HARDWARE baseline
// still carries a write-side spec.Unverified somewhere.
//
// The FT-710 is the one registered model that is not: its write trials
// are complete, so its six writable fields are spec.Supported and
// nothing in its set is Unverified — asking that user for consent would
// be asking them to authorise a risk this project has already retired.
// The other three have never been written to by this project at all.
//
// EVERY registered model, one row each, deliberately. This predicate is
// what the CLI's "settings unverified-writes" listing and its refusal
// both key off, and what the GUI will key off too; a model missing from
// the table would let the two surfaces disagree about which radios can
// be consented to at all.
func TestNeedsUnverifiedConsent_PerModel(t *testing.T) {
	want := map[string]bool{
		DefaultModel:   false,
		FTdx10Model:    true,
		FTdx101DModel:  true,
		FTdx101MPModel: true,
	}
	models := SupportedModels()
	if len(models) != len(want) {
		t.Fatalf("SupportedModels() = %v — this table has %d rows and must name every registered model", models, len(want))
	}
	for _, model := range models {
		expected, ok := want[model]
		if !ok {
			t.Fatalf("registered model %q has no row in this table", model)
		}
		got, err := NeedsUnverifiedConsent(model)
		if err != nil {
			t.Fatalf("NeedsUnverifiedConsent(%q): unexpected error: %v", model, err)
		}
		if got != expected {
			t.Errorf("NeedsUnverifiedConsent(%q) = %v, want %v", model, got, expected)
		}
	}
}

// TestNeedsUnverifiedConsent_MatchesStaticCapabilities pins the
// derivation rather than the answer: for every registered model, the
// predicate must agree with a write-side Unverified scan of that model's
// own StaticCapabilities. It is what stops the answer becoming a
// hand-maintained list of model names that a newly registered radio
// would silently be absent from.
func TestNeedsUnverifiedConsent_MatchesStaticCapabilities(t *testing.T) {
	for _, model := range SupportedModels() {
		caps, err := StaticCapabilities(model)
		if err != nil {
			t.Fatalf("StaticCapabilities(%q): unexpected error: %v", model, err)
		}
		scanned := false
		for _, b := range caps.Banks {
			for f, fs := range b.Fields {
				// The same erase exemption the predicate applies, restated
				// here rather than borrowed, so the scan this test checks
				// against stays an independent statement of the rule.
				if f != spec.FieldErase && fs.Write == spec.Unverified {
					scanned = true
				}
			}
		}
		got, err := NeedsUnverifiedConsent(model)
		if err != nil {
			t.Fatalf("NeedsUnverifiedConsent(%q): unexpected error: %v", model, err)
		}
		if got != scanned {
			t.Errorf("NeedsUnverifiedConsent(%q) = %v, but a write-side Unverified scan of its static capabilities says %v", model, got, scanned)
		}
	}
}

// TestConsentCouldUnlockAWrite_EraseOnlyIsNotEligible pins the exemption
// the predicate has to share with the transform (final review, Codex
// MINOR): spec.ConsentUnverifiedWrites skips spec.FieldErase, so an
// Unverified label THERE is not something any consent can turn into a
// writable field. A radio whose only write-side Unverified sits on
// FieldErase is therefore not consent-eligible: prompting its owner would
// be asking them to authorise a write that the grant provably cannot
// unlock, and — in the GUI — to sit through a disconnect/reconnect for it.
//
// It runs against a FIXTURE rather than a registered model deliberately.
// No registered radio has that shape today (which is why nothing pinned
// the old behaviour), so the only way to state the rule is to build the
// capability set that exhibits it. The transform is invoked alongside, so
// the test fails if the two ever stop agreeing about erase.
func TestConsentCouldUnlockAWrite_EraseOnlyIsNotEligible(t *testing.T) {
	eraseOnly := spec.Capabilities{
		Banks: []spec.Bank{{
			ID:    "MEM",
			Slots: []string{"001"},
			Fields: map[spec.Field]spec.FieldSupport{
				spec.FieldFrequency: {Read: spec.Supported, Write: spec.Supported},
				spec.FieldErase:     {Read: spec.Unsupported, Write: spec.Unverified},
			},
		}},
	}

	if consentCouldUnlockAWrite(eraseOnly) {
		t.Error("consentCouldUnlockAWrite(erase-only Unverified) = true — consent would be asked for, and could unlock nothing: spec.ConsentUnverifiedWrites exempts FieldErase")
	}
	// The other half of the same fact, read off the transform itself: a
	// grant applied to this set changes nothing at all.
	if got := spec.ConsentUnverifiedWrites(eraseOnly); !reflect.DeepEqual(got, eraseOnly) {
		t.Errorf("spec.ConsentUnverifiedWrites moved an erase-only Unverified set: got %+v, want it unchanged", got)
	}

	// The control: the SAME set with one non-erase Unverified is eligible,
	// so the test cannot pass by the predicate answering false to everything.
	withWritable := spec.Capabilities{
		Banks: []spec.Bank{{
			ID:    "MEM",
			Slots: []string{"001"},
			Fields: map[spec.Field]spec.FieldSupport{
				spec.FieldFrequency: {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldErase:     {Read: spec.Unsupported, Write: spec.Unverified},
			},
		}},
	}
	if !consentCouldUnlockAWrite(withWritable) {
		t.Error("consentCouldUnlockAWrite(a set with a non-erase Unverified write) = false, want true")
	}
}

// TestNeedsUnverifiedConsent_UnknownModel pins the error path: an
// unrecognised model is *UnknownModelError, and the bool is false — a
// caller that ignored the error must not be told a radio it cannot even
// name is consent-eligible.
func TestNeedsUnverifiedConsent_UnknownModel(t *testing.T) {
	got, err := NeedsUnverifiedConsent("NO-SUCH-MODEL")
	if err == nil {
		t.Fatalf("NeedsUnverifiedConsent(unknown) = %v, <nil error>, want an error", got)
	}
	var unknown *UnknownModelError
	if !errors.As(err, &unknown) {
		t.Errorf("NeedsUnverifiedConsent(unknown) error = %v, want an *UnknownModelError", err)
	}
	if got {
		t.Error("NeedsUnverifiedConsent(unknown) = true, want false alongside the error")
	}
}

// --- E2: driver-reported serial framing (spec D3.1) ---------------------

// framingFixtureDriver is baudFixtureDriver plus a StopBits report: the
// shape spec D3.1 gives every Icom driver, and the only way this package
// can be shown to consult one, since no registered Yaesu model reports.
//
// THE REPORTER IS ON THE DRIVER, not the session, and that placement is
// the whole point of the fixture: wiring holds the driver BEFORE the port
// opens and the session does not exist yet, so a Session-side reporter
// could never be consulted at the moment the port's framing is chosen.
type framingFixtureDriver struct {
	baudFixtureDriver
	stopBits int
}

func (d framingFixtureDriver) StopBits() int { return d.stopBits }

var _ driver.SerialFramingReporter = framingFixtureDriver{}

// registerFramingFixture registers a driver reporting stopBits under a
// throwaway model name, removing it again on cleanup.
func registerFramingFixture(t *testing.T, model string, stopBits int) {
	t.Helper()
	realDrivers[model] = func(bool) driver.Driver {
		return framingFixtureDriver{
			baudFixtureDriver: baudFixtureDriver{caps: spec.Capabilities{
				Model:        model,
				CATID:        "9999",
				Bauds:        []int{transport.DefaultBaud},
				DefaultBaud:  transport.DefaultBaud,
				TagLen:       12,
				ShiftOptions: spec.StandardShiftOptions(),
				CTCSSStates:  spec.StandardCTCSSStates(),
			}},
			stopBits: stopBits,
		}
	}
	t.Cleanup(func() { delete(realDrivers, model) })
}

// TestOpenRealSessionFor_StopBitsFollowAReportingDriver is spec D3.1's
// half that the four registered Yaesu models cannot prove: a driver
// implementing driver.SerialFramingReporter has its answer carried into
// the port's own configuration, so an Icom radio's 8-N-1 line is opened
// 8-N-1 rather than at transport's fixed 8-N-2.
func TestOpenRealSessionFor_StopBitsFollowAReportingDriver(t *testing.T) {
	const fixtureModel = "TEST-FRAMING-FIXTURE"
	if transport.DefaultStopBits == 1 {
		t.Fatal("sanity check failed: transport.DefaultStopBits is already 1, so this test could not distinguish the two sources")
	}
	registerFramingFixture(t, fixtureModel, 1)

	got := recordSerialConfig(t)

	_, _, err := OpenRealSessionFor(testCtx(t), fixtureModel, "/dev/nonexistent-rigprog-test-port")
	if !errors.Is(err, errSeamRefused) {
		t.Fatalf("OpenRealSessionFor: err = %v, want it to wrap the seam's own error", err)
	}
	if got.StopBits != 1 {
		t.Errorf("SerialConfig.StopBits = %d, want 1 — the driver's own report must reach the port, not transport.DefaultStopBits (%d)", got.StopBits, transport.DefaultStopBits)
	}
}

// TestOpenRealSessionFor_StopBitsRefuseAnImpossibleReport is the rule's
// fail-closed half. A reported value other than 1 or 2 is REFUSED, and
// zero is refused with the rest: a driver whose StopBits() returns the
// zero value has not said "use the default", it has failed to say
// anything, and silently selecting 8-N-2 for it would put a guess on the
// wire under the appearance of a driver's own statement.
//
// The refusal happens BEFORE the port is opened, which the recording seam
// proves by never being reached.
func TestOpenRealSessionFor_StopBitsRefuseAnImpossibleReport(t *testing.T) {
	for _, stopBits := range []int{0, -1, 3, 8} {
		t.Run(fmt.Sprintf("reports %d", stopBits), func(t *testing.T) {
			const fixtureModel = "TEST-FRAMING-REFUSED"
			registerFramingFixture(t, fixtureModel, stopBits)

			got := recordSerialConfig(t)

			sess, closeAll, err := OpenRealSessionFor(testCtx(t), fixtureModel, "/dev/nonexistent-rigprog-test-port")
			if errors.Is(err, errSeamRefused) {
				t.Fatalf("the port was opened at StopBits %d — an unsupported report must be refused before any port is touched", got.StopBits)
			}
			var bad *UnsupportedStopBitsError
			if !errors.As(err, &bad) {
				t.Fatalf("err = %v, want an *UnsupportedStopBitsError", err)
			}
			if bad.StopBits != stopBits {
				t.Errorf("error reports StopBits %d, want %d", bad.StopBits, stopBits)
			}
			if bad.Model != fixtureModel {
				t.Errorf("error names model %q, want %q", bad.Model, fixtureModel)
			}
			if sess != nil || closeAll != nil {
				t.Errorf("expected nil session/closeAll on refusal, got sess=%v closeAllIsNil=%v", sess, closeAll == nil)
			}
		})
	}
}

// TestOpenRealSessionFor_EveryYaesuModelOpensAtEightNTwo is the pin the
// adjudication asks for, on the honest observable: the PORT CONFIGURATION
// each of the four registered models is opened with. None of them
// implements SerialFramingReporter, so each must still reach the serial
// layer at transport.DefaultStopBits — before and after E2, unchanged.
func TestOpenRealSessionFor_EveryYaesuModelOpensAtEightNTwo(t *testing.T) {
	models := SupportedModels()
	if len(models) != 4 {
		t.Fatalf("SupportedModels() = %v (%d models), want the four registered Yaesu models — update this pin deliberately", models, len(models))
	}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			d, err := realDriverFor(model, false)
			if err != nil {
				t.Fatalf("realDriverFor(%q): %v", model, err)
			}
			if r, ok := d.(driver.SerialFramingReporter); ok {
				t.Fatalf("%s implements SerialFramingReporter (reporting %d) — the four Yaesu models must not, so that 8-N-2 stays their port configuration by default rather than by a driver's statement", model, r.StopBits())
			}

			got := recordSerialConfig(t)
			_, _, err = OpenRealSessionFor(testCtx(t), model, "/dev/nonexistent-rigprog-test-port")
			if !errors.Is(err, errSeamRefused) {
				t.Fatalf("OpenRealSessionFor(%q): err = %v, want it to wrap the seam's own error", model, err)
			}
			if got.StopBits != 2 {
				t.Errorf("SerialConfig.StopBits = %d, want 2 (transport.DefaultStopBits) for %s", got.StopBits, model)
			}
		})
	}
}

// TestEveryYaesuModelDeclaresAToneListAndNoRange is E3's Yaesu pin, taken
// at the composition root because it is the one place that can see all
// four registered models at once.
//
// The tier added an OPTIONAL numeric tone domain (spec.Capabilities.
// CTCSSToneRange) for CI-V models whose tone field is a number rather than
// an index into a chart. No Yaesu model declares one, and every one of
// them must still admit exactly the fifty tones its own CTCSSTones lists
// and nothing else — which is what makes the shared predicate's arrival a
// no-change event on this side of the tier.
func TestEveryYaesuModelDeclaresAToneListAndNoRange(t *testing.T) {
	models := SupportedModels()
	if len(models) != 4 {
		t.Fatalf("SupportedModels() = %v (%d models), want the four registered Yaesu models — update this pin deliberately", models, len(models))
	}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			caps, err := StaticCapabilities(model)
			if err != nil {
				t.Fatalf("StaticCapabilities(%q): %v", model, err)
			}
			if caps.CTCSSToneRange != nil {
				t.Fatalf("%s declares a CTCSSToneRange (%+v) — the Yaesu models name their tones by chart index, and a range would be a claim about hardware nobody has made", model, *caps.CTCSSToneRange)
			}
			if len(caps.CTCSSTones) == 0 {
				t.Fatalf("%s declares no CTCSSTones at all", model)
			}
			// AdmitsTone must answer exactly what the list says, for every
			// tone in the standard chart and for a value outside it.
			for _, tone := range caps.CTCSSTones {
				if !caps.AdmitsTone(tone) {
					t.Errorf("AdmitsTone(%v) = false for a tone %s's own chart lists", tone, model)
				}
			}
			if caps.AdmitsTone(spec.Tone(1)) {
				t.Errorf("AdmitsTone(0.1 Hz) = true for %s — the list is the whole domain, and a range predicate leaking in would admit the gaps between entries", model)
			}
			if caps.AdmitsTone(spec.Tone(700)) {
				t.Errorf("AdmitsTone(70.0 Hz) = true for %s — 70.0 is between two chart entries and is not a tone this radio can express", model)
			}
		})
	}
}

// TestEveryYaesuModelStillValidatesUnchanged is E5's pin, and it is
// deliberately taken on the two rules E5 rewrote rather than on Validate's
// verdict alone.
//
// E5b made the "ShiftOptions must not be empty" pair rule CONDITIONAL on
// some bank reaching the field, so that a model whose bank legitimately
// carries no shift or duplex vocabulary is admitted. That condition must
// still HOLD for every Yaesu model — each declares FieldShift and
// FieldCTCSSState — or the rule would have been quietly switched off for
// the radios it was written for rather than relaxed for the ones it was
// not.
//
// E5a replaced the "at most one option per direction" rule on the Icom
// vocabularies with the canonical-entry rule. No Yaesu model declares
// either vocabulary, so that change must be invisible here, which the
// empty-slice assertions say.
func TestEveryYaesuModelStillValidatesUnchanged(t *testing.T) {
	models := SupportedModels()
	if len(models) != 4 {
		t.Fatalf("SupportedModels() = %v (%d models), want the four registered Yaesu models — update this pin deliberately", models, len(models))
	}
	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			caps, err := StaticCapabilities(model)
			if err != nil {
				t.Fatalf("StaticCapabilities(%q): %v", model, err)
			}
			if err := caps.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}

			var reachesShift, reachesTone bool
			for _, b := range caps.Banks {
				if !caps.FieldSupport(b.ID, spec.FieldShift).Unreachable() {
					reachesShift = true
				}
				if !caps.FieldSupport(b.ID, spec.FieldCTCSSState).Unreachable() {
					reachesTone = true
				}
			}
			if !reachesShift {
				t.Errorf("no bank reaches FieldShift — E5b's condition would switch the empty-ShiftOptions refusal OFF for this model, which is not what E5b relaxed")
			}
			if !reachesTone {
				t.Errorf("no bank reaches FieldCTCSSState — same reasoning, on the tone pair")
			}
			if len(caps.DuplexOptions) != 0 {
				t.Errorf("DuplexOptions = %v, want empty for a Yaesu model", caps.DuplexOptions)
			}
			if len(caps.ToneModes) != 0 {
				t.Errorf("ToneModes = %v, want empty for a Yaesu model", caps.ToneModes)
			}

			// And the E5b condition really does bite: strip the
			// vocabulary this model DOES declare and Validate must still
			// refuse, because its banks reach the field.
			stripped := caps
			stripped.ShiftOptions = nil
			if err := stripped.Validate(); err == nil {
				t.Error("Validate() accepted this model with no ShiftOptions — a bank reaching FieldShift must still name the values it can hold")
			}
		})
	}
}

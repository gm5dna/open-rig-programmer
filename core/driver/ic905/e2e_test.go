// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic905"
)

// THE FAKE IS A SIBLING OF THE DIALECT, NOT A REFACTOR OF IT.
// internal/fakeic905 imports nothing from this module — not core/civ, not
// core/civ/ic905, not this package — and re-derived every offset and
// every framing byte from the IC-905 CI-V REFERENCE GUIDE's own diagrams
// by way of its own quarantined artefacts. Where it and core/civ/ic905
// agree, they agree because two independent readings of one document
// landed in the same place, which is EVIDENCE. Where one imported the
// other, agreement would be a tautology.
//
// That is what these tests are for, and it is why they are not a
// duplicate of respondingport_test.go's. The scripted port answers
// per-frame from a table and can serve deliberately WRONG answers, which
// is what the error paths need; the fake models a radio's STATE and can
// therefore be READ BACK — Record() shows what a write actually left
// behind, and Frames() shows what the radio was actually asked. Neither
// substitutes for the other.

// e2eImage assembles a fake's options, with the INVENTED default image
// cleared out of the way first.
//
// A fresh Radio holds ten channels in group 0, each 64 ZERO bytes. Those
// bytes assert nothing about any radio — the fake's own doc says so — and
// they are not a decodable record either: byte ⑫ zero is not one of the
// three printed filter values, so a read of one fails in the parser. Every
// test below therefore starts from an EMPTY radio and seeds exactly what
// it means to assert.
func e2eImage(opts ...fakeic905.Option) []fakeic905.Option {
	const defaultImageChannels = 10
	out := make([]fakeic905.Option, 0, defaultImageChannels+len(opts))
	for ch := 0; ch < defaultImageChannels; ch++ {
		out = append(out, fakeic905.WithEmpty(0, ch))
	}
	return append(out, opts...)
}

// openFake Opens a session of the given profile against a fresh fake.
func openFake(t *testing.T, profile Profile, drvOpts []Option, fakeOpts ...fakeic905.Option) (*fakeic905.Radio, *Session) {
	t.Helper()
	radio := fakeic905.New(fakeOpts...)
	t.Cleanup(func() { _ = radio.Close() })

	sess, err := New(profile, drvOpts...).Open(context.Background(), radio.Port(), driver.Identity{Port: "/dev/fake"})
	if err != nil {
		t.Fatalf("Open against the fake: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return radio, sess.(*Session)
}

// setFrames returns the 1A 00 SET frames the fake actually received —
// what a refusal test must find NONE of.
func setFrames(radio *fakeic905.Radio) [][]byte {
	var out [][]byte
	for _, f := range radio.Frames() {
		if len(f) > memoryReadFrameLen && len(f) >= 6 && f[4] == 0x1A && f[5] == 0x00 {
			out = append(out, f)
		}
	}
	return out
}

// The two flood frames. BOTH OPTIONS TAKE THE FRAME AND PANIC IF IT IS
// ADDRESSED THE WRONG WAY, which is the fake refusing to let a test think
// it is exercising one branch while exercising the other: a broadcast
// (to = 00) dies at the accumulator's address filter and never reaches
// the engine, and only a controller-addressed frame (to = E0) can drive a
// drain to its cap.
//
// The payload is an unsolicited 1C 00 the driver never asked for and no
// matcher accepts. Both FORMS are ASSUMED — the reference prints four
// frames and none of them is a broadcast (ic905.transceive_default, lift
// ic905-R-11).
var (
	broadcastFrame = []byte{0xFE, 0xFE, 0x00, 0xAC, 0x1C, 0x00, 0x01, 0xFD}
	addressedFrame = []byte{0xFE, 0xFE, 0xE0, 0xAC, 0x1C, 0x00, 0x01, 0xFD}
)

// TestE2E_ProbeFingerprintAcceptsBothLengths.
//
// The accepted set IS the fingerprint (spec D3.2), and this model's set
// has TWO members because its frequency field is documented at two
// widths. Both must open, and the observed length must be recorded.
func TestE2E_ProbeFingerprintAcceptsBothLengths(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name      string
		freqHz    uint64
		freqBytes int
		wantLen   int
	}{
		{"the 64-byte shape the diagram draws", 144_500_000, 5, 64},
		{"the 65-byte 10 GHz shape", 10_250_000_000, 6, 65},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, s := openFake(t, RealHardware, nil, e2eImage(
				fakeic905.WithRecord(0, 0, goldenRecord(tt.freqHz, tt.freqBytes).build()),
			)...)

			d := s.Diagnostics905()
			if !d.Fingerprinted {
				t.Fatal("Fingerprinted is false — a record at a DECLARED length is what confirms this radio")
			}
			if d.ObservedRecordLength != tt.wantLen {
				t.Errorf("ObservedRecordLength = %d, want %d", d.ObservedRecordLength, tt.wantLen)
			}
			// The identity token is RECORDED, never matched. The fake's
			// default is DE AD, chosen to be implausible precisely so a
			// driver that matched it would be testing the fake's guess.
			if got, want := s.Identity().CATID, "AC:DEAD"; got != want {
				t.Errorf("Identity().CATID = %q, want %q — the 19 00 reply VALUE is undocumented (D5 entry 7, lift ic905-R-02), so the probe records whatever arrives", got, want)
			}
		})
	}
}

// TestE2E_ReadAllMaterialisesTheOccupiedSlots drives core/clone's own
// ReadAll over the discovered sparse inventory, so the bank walk and this
// driver's discovery are exercised TOGETHER rather than separately.
//
// It is the test that would have caught a discovery that published
// nothing: ReadAll walks Capabilities().Banks[].Slots, and a sparse bank
// whose Slots stayed empty returns no memories at all (ruling R12).
func TestE2E_ReadAllMaterialisesTheOccupiedSlots(t *testing.T) {
	t.Parallel()
	rec := goldenRecord(144_500_000, 5).build()
	_, s := openFake(t, RealHardware, nil, e2eImage(
		fakeic905.WithRecord(0, 0, rec),
		fakeic905.WithRecord(0, 4, rec),
		fakeic905.WithRecord(0, 99, rec),
	)...)

	// The MEM bank publishes exactly the three that answered.
	if got, want := memBankSlots(t, s), []string{"G01-001", "G01-005", "G01-100"}; !slices.Equal(got, want) {
		t.Fatalf("materialised slots = %v, want %v", got, want)
	}

	svc := clone.NewService(s, clone.SnapshotStore{Dir: t.TempDir()})
	cp, err := svc.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("clone.ReadAll: %v", err)
	}

	// Three MEM channels plus the CALL bank's twelve static slots: ReadAll
	// walks every slot the session publishes, populated or not.
	if got, want := len(cp.Channels), 3+12; got != want {
		t.Fatalf("ReadAll returned %d channels, want %d (three MEM plus twelve CALL)", got, want)
	}
	populated := 0
	for _, ch := range cp.Channels {
		if ch.Empty() {
			continue
		}
		populated++
		if ch.Data.FreqHz != 144_500_000 || ch.Data.Mode != "FM" || ch.Data.Tag != "HIGHLAND BASE905" {
			t.Errorf("slot %s came back as %d Hz / %q / %q", ch.Slot, ch.Data.FreqHz, ch.Data.Mode, ch.Data.Tag)
		}
	}
	if populated != 3 {
		t.Errorf("ReadAll found %d populated channels, want 3", populated)
	}
	if cp.Radio.Model != "IC-905" || cp.Radio.CATID != "AC:DEAD" {
		t.Errorf("codeplug Radio = %s/%s — ReadAll records the SESSION capabilities' CATID, which is where the observed token has to be", cp.Radio.Model, cp.Radio.CATID)
	}
}

// TestE2E_WriteOneChannelWithAndWithoutConsent.
//
// WITHOUT consent a RealHardware session cannot write anything — the
// capability profile is what enforces that, not the ladder — and the fake
// must see NOTHING. WITH consent the write lands, and the fake's Record()
// is compared BYTE FOR BYTE against the golden record.
//
// The comparison is against 64 bytes, not 68: Record() returns the RECORD
// (spec Erratum 1's record-only convention), and 68 is the same record
// counted WITH the four channel-address bytes, which the frame carries
// separately.
func TestE2E_WriteOneChannelWithAndWithoutConsent(t *testing.T) {
	t.Parallel()
	seed := goldenRecord(144_500_000, 5).build()

	t.Run("without consent, nothing reaches the radio", func(t *testing.T) {
		t.Parallel()
		radio, s := openFake(t, RealHardware, nil, e2eImage(
			fakeic905.WithRecord(0, 1, seed),
		)...)

		res, err := s.WriteChannel(context.Background(), writableChannel("G01-002"))
		if err == nil {
			t.Fatal("an unconsented real-hardware write was accepted")
		}
		if !errors.Is(err, driver.ErrWriteRefused) {
			t.Errorf("error = %v, want errors.Is(err, driver.ErrWriteRefused)", err)
		}
		if len(res.Steps) != 0 {
			t.Errorf("Steps = %+v, want empty — no frame was built", res.Steps)
		}
		if n := len(setFrames(radio)); n != 0 {
			t.Errorf("the fake saw %d set frames — a refusal must reach no radio", n)
		}
	})

	t.Run("with consent, the golden record lands byte for byte", func(t *testing.T) {
		t.Parallel()
		radio, s := openFake(t, RealHardware, []Option{WithConsentedUnverifiedWrites()}, e2eImage(
			// Seeded with a DIFFERENT frequency, so the comparison below
			// proves the write happened rather than that the seed was
			// already right.
			fakeic905.WithRecord(0, 1, goldenRecord(430_000_000, 5).build()),
		)...)

		res, err := s.WriteChannel(context.Background(), writableChannel("G01-002"))
		if err != nil {
			t.Fatalf("a consented write was refused: %v", err)
		}
		if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
			t.Fatalf("Steps = %+v, want one Sent and Confirmed 1A 00 step", res.Steps)
		}

		got, ok := radio.Record(0, 1)
		if !ok {
			t.Fatal("the fake holds no record for group 0 channel 1 after the write")
		}
		want := goldenRecord(144_500_000, 5).build()
		if len(got) != 64 {
			t.Errorf("the fake holds a %d-byte record, want 64 (spec Erratum 1's record-only length; 68 is the same record counted with the four address bytes)", len(got))
		}
		if !bytes.Equal(got, want) {
			t.Errorf("the record the radio kept is not the golden one:\n  kept   % X\n  golden % X", got, want)
		}
	})
}

// TestE2E_AChannelWithASelectTagOrACallSignIsRefusedNotCorrupted is the
// end-to-end half of the plan's one CRITICAL regression test.
//
// Twenty-seven record bytes have no home in the neutral model, and civ's
// encoder fills every unmapped byte from the profile's template — so a
// write to a channel that really holds one of them would silently replace
// it. Byte ⑤ is the one the earlier ladder missed: a MEM channel carrying
// SELECT ★1/★2/★3 would have been rewritten as SELECT OFF.
//
// Here the RADIO holds the value, the write is REFUSED, and the fake's
// own Record() proves the byte survived untouched — which is the half a
// scripted port cannot show.
func TestE2E_AChannelWithASelectTagOrACallSignIsRefusedNotCorrupted(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		mutin   func(*recordFields)
		printed string
	}{
		{"⑤ a SELECT star tag", func(r *recordFields) { r.select5 = 0x02 }, "⑤"},
		{"⑮ a digital squelch", func(r *recordFields) { r.digitalSquelch = 0x01 }, "⑮"},
		{"㉕ a DV code squelch", func(r *recordFields) { r.dvSquelch = 0x0A }, "㉕"},
		{"㉙~㊱ a UR call sign", func(r *recordFields) { r.urCall = "GM5DNA" }, "㉙~㊱"},
		{"㊲~㊹ an R1 call sign", func(r *recordFields) { r.r1Call = "GB3IN B" }, "㊲~㊹"},
		{"㊺~52 an R2 call sign", func(r *recordFields) { r.r2Call = "GB3IN G" }, "㊺~52"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			held := goldenRecord(144_500_000, 5)
			tt.mutin(&held)
			seed := held.build()

			radio, s := openFake(t, RealHardware, []Option{WithConsentedUnverifiedWrites()}, e2eImage(
				fakeic905.WithRecord(0, 0, seed),
			)...)

			_, err := s.WriteChannel(context.Background(), writableChannel("G01-001"))
			if err == nil {
				t.Fatal("the write was accepted over a channel this tier cannot model")
			}
			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("error = %v, want a *driver.WriteRefusedError", err)
			}
			if !strings.Contains(wre.Reason, tt.printed) {
				t.Errorf("reason = %q, want it to name the printed range %s", wre.Reason, tt.printed)
			}
			if n := len(setFrames(radio)); n != 0 {
				t.Errorf("the fake saw %d set frames — the channel must be refused, never rewritten", n)
			}
			// AND THE RADIO STILL HOLDS WHAT IT HELD. This is the
			// assertion the scripted port cannot make.
			after, ok := radio.Record(0, 0)
			if !ok {
				t.Fatal("the fake no longer holds the channel")
			}
			if !bytes.Equal(after, seed) {
				t.Errorf("the stored record changed:\n  before % X\n  after  % X", seed, after)
			}
		})
	}
}

// TestE2E_AnUndeclaredRecordLengthIsRefusedWithoutAttribution is the
// Wave-3 default branch: with no sibling table, EVERY unrecognised length
// is refused with BOTH model fields empty, which is the honest value for
// a driver that cannot name what it found.
func TestE2E_AnUndeclaredRecordLengthIsRefusedWithoutAttribution(t *testing.T) {
	t.Parallel()
	radio := fakeic905.New(e2eImage(fakeic905.WithRecord(0, 0, make([]byte, 63)))...)
	t.Cleanup(func() { _ = radio.Close() })

	_, err := New(RealHardware).Open(context.Background(), radio.Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded against a radio answering a 63-byte record")
	}
	var wre *driver.WrongRadioError
	if !errors.As(err, &wre) {
		t.Fatalf("error = %v, want a *driver.WrongRadioError", err)
	}
	if wre.WantModel != "" || wre.GotModel != "" {
		t.Errorf("WantModel = %q, GotModel = %q — both must be EMPTY", wre.WantModel, wre.GotModel)
	}
	if strings.Contains(err.Error(), "PROVISIONAL") {
		t.Errorf("error = %q — branch (b) attributes nothing, so it has no attribution to qualify", err)
	}
}

// TestE2E_ASiblingRecordLengthIsAProvisionalWrongRadio is the
// wrong-sibling case proper. Wave 4 populates the table from the registry
// in the same commit that registers the tier's models; Wave 3 proves the
// branch works with a synthetic one.
func TestE2E_ASiblingRecordLengthIsAProvisionalWrongRadio(t *testing.T) {
	t.Parallel()
	radio := fakeic905.New(e2eImage(fakeic905.WithRecord(0, 0, make([]byte, 39)))...)
	t.Cleanup(func() { _ = radio.Close() })

	_, err := New(RealHardware, WithSiblingRecordLengths(SiblingLengths{39: "IC-7300"})).
		Open(context.Background(), radio.Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded against a radio answering a foreign record length")
	}
	var wre *driver.WrongRadioError
	if !errors.As(err, &wre) {
		t.Fatalf("error = %v, want a *driver.WrongRadioError", err)
	}
	if wre.GotModel != "IC-7300" || wre.WantModel != "IC-905" {
		t.Errorf("WantModel/GotModel = %q/%q, want IC-905/IC-7300", wre.WantModel, wre.GotModel)
	}
	if !strings.Contains(err.Error(), "PROVISIONAL") {
		t.Errorf("error = %q, want it to say the attribution is PROVISIONAL — the record lengths this tier compares are themselves ASSUMED derivations", err)
	}
}

// TestE2E_EraseIsRefusedEvenWithConsent.
//
// This tier ships NO ERASE PATH for any Icom, and consent cannot mint
// one: spec.ConsentUnverifiedWrites structurally never touches
// spec.FieldErase. The wire form exists on this radio and is recorded in
// doc.go; nothing implements it.
func TestE2E_EraseIsRefusedEvenWithConsent(t *testing.T) {
	t.Parallel()
	seed := goldenRecord(144_500_000, 5).build()

	for _, tt := range []struct {
		name    string
		drvOpts []Option
	}{
		{"unconsented", nil},
		{"consented", []Option{WithConsentedUnverifiedWrites()}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			radio, s := openFake(t, RealHardware, tt.drvOpts, e2eImage(
				fakeic905.WithRecord(0, 0, seed),
			)...)

			res, err := s.WriteChannel(context.Background(), codeplug.Channel{Slot: "G01-001"})
			if err == nil {
				t.Fatal("an erase was accepted")
			}
			var wre *driver.WriteRefusedError
			if !errors.As(err, &wre) {
				t.Fatalf("error = %v, want a *driver.WriteRefusedError", err)
			}
			if !slices.Equal(wre.Fields, []spec.Field{spec.FieldErase}) {
				t.Errorf("refused fields = %v, want [erase]", wre.Fields)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if n := len(radio.Frames()); n == 0 {
				t.Fatal("the fake saw no frames at all — the session cannot have opened")
			}
			if n := len(setFrames(radio)); n != 0 {
				t.Errorf("the fake saw %d set frames for an erase", n)
			}
			// And the channel is untouched.
			after, ok := radio.Record(0, 0)
			if !ok || !bytes.Equal(after, seed) {
				t.Error("the channel was altered by a refused erase")
			}
		})
	}
}

// TestE2E_AnAddOverAnUndiscoveredOccupiedSlotIsRefused is the round-3
// CRITICAL, end to end (ruling T3).
//
// The fake holds G06-038 and leaves G06-001 EMPTY, so the bounded default
// walk's channel-00 probe misses the whole group. ReadAll returns no
// channel for it, a Diff would then offer the write as an ADD, and the
// write must be REFUSED — because comparing unmapped bytes cannot catch
// it: this record's unmapped region matches the template perfectly.
func TestE2E_AnAddOverAnUndiscoveredOccupiedSlotIsRefused(t *testing.T) {
	t.Parallel()
	seed := goldenRecord(144_500_000, 5).build()
	image := e2eImage(
		fakeic905.WithRecord(0, 0, seed),
		// G06-038 is wire group 5, channel 37. Its group's channel 00 is
		// left unheld, which is what the bounded walk probes.
		fakeic905.WithRecord(5, 37, seed),
	)

	t.Run("the bounded default walk misses it and the add is refused", func(t *testing.T) {
		t.Parallel()
		radio, s := openFake(t, RealHardware, []Option{WithConsentedUnverifiedWrites()}, image...)

		if slices.Contains(memBankSlots(t, s), "G06-038") {
			t.Fatal("the bounded walk materialised G06-038 — this test's premise is gone")
		}
		// ReadAll agrees: the slot is not in the codeplug at all, which
		// is exactly how a Diff comes to offer the write as an ADD.
		svc := clone.NewService(s, clone.SnapshotStore{Dir: t.TempDir()})
		cp, err := svc.ReadAll(context.Background())
		if err != nil {
			t.Fatalf("clone.ReadAll: %v", err)
		}
		for _, ch := range cp.Channels {
			if ch.Slot == "G06-038" {
				t.Fatal("ReadAll returned G06-038 — the bounded walk cannot have missed it")
			}
		}

		before := len(setFrames(radio))
		_, err = s.WriteChannel(context.Background(), writableChannel("G06-038"))
		if err == nil {
			t.Fatal("the add was accepted over a channel the radio already holds")
		}
		var wre *driver.WriteRefusedError
		if !errors.As(err, &wre) {
			t.Fatalf("error = %v, want a *driver.WriteRefusedError", err)
		}
		if !strings.Contains(wre.Reason, "WithFullInventoryWalk") {
			t.Errorf("reason = %q, want it to NAME the remedy the user can act on", wre.Reason)
		}
		if n := len(setFrames(radio)); n != before {
			t.Errorf("%d set frames reached the fake", n-before)
		}
		if after, ok := radio.Record(5, 37); !ok || !bytes.Equal(after, seed) {
			t.Error("the undiscovered channel was overwritten")
		}
	})

	t.Run("after a full walk the same write proceeds", func(t *testing.T) {
		t.Parallel()
		radio, s := openFake(t, RealHardware,
			[]Option{WithConsentedUnverifiedWrites(), WithFullInventoryWalk()}, image...)

		if !slices.Contains(memBankSlots(t, s), "G06-038") {
			t.Fatal("the full walk did not materialise G06-038")
		}
		res, err := s.WriteChannel(context.Background(), writableChannel("G06-038"))
		if err != nil {
			t.Fatalf("the write was refused after a full walk: %v", err)
		}
		if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
			t.Errorf("Steps = %+v, want one confirmed step", res.Steps)
		}
		if got, ok := radio.Record(5, 37); !ok || !bytes.Equal(got, goldenRecord(144_500_000, 5).build()) {
			t.Error("the write did not land")
		}
	})
}

// TestE2E_CreateIntoAnEmptySlotSucceedsAndAToneLessCreateIsRefused.
//
// The fake does not hold the slot, so the read answers FA; a fully
// specified channel is written and the fake SEEDS it at the length that
// arrived. A channel with a non-Known tone is REFUSED naming the field,
// because an ADD has no prior record to preserve one from and this manual
// prints no default tone anywhere (T1(5); ic905.create_default_tone, lift
// ic905-R-18).
func TestE2E_CreateIntoAnEmptySlotSucceedsAndAToneLessCreateIsRefused(t *testing.T) {
	t.Parallel()
	radio, s := openFake(t, RealHardware, []Option{WithConsentedUnverifiedWrites()}, e2eImage(
		fakeic905.WithRecord(0, 0, goldenRecord(144_500_000, 5).build()),
	)...)

	if _, ok := radio.Record(0, 50); ok {
		t.Fatal("the fake already holds group 0 channel 50 — this test's premise is gone")
	}

	// The tone-less create is refused, and nothing is seeded.
	toneless := writableChannel("G01-051")
	toneless.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	_, err := s.WriteChannel(context.Background(), toneless)
	if err == nil {
		t.Fatal("a create with no tone was accepted")
	}
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("error = %v, want a *driver.WriteRefusedError", err)
	}
	if !slices.Contains(wre.Fields, spec.FieldToneTx) {
		t.Errorf("refused fields = %v, want tone_tx named", wre.Fields)
	}
	if !strings.Contains(wre.Reason, "ic905-R-18") {
		t.Errorf("reason = %q, want it to name the register lift", wre.Reason)
	}
	if _, ok := radio.Record(0, 50); ok {
		t.Error("the refused create seeded the channel anyway")
	}

	// The fully specified create lands, and the fake seeds it.
	res, err := s.WriteChannel(context.Background(), writableChannel("G01-051"))
	if err != nil {
		t.Fatalf("a fully specified create was refused: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Fatalf("Steps = %+v, want one confirmed step", res.Steps)
	}
	got, ok := radio.Record(0, 50)
	if !ok {
		t.Fatal("the fake did not seed the created channel")
	}
	if !bytes.Equal(got, goldenRecord(144_500_000, 5).build()) {
		t.Errorf("the created record is not the golden one:\n  seeded % X", got)
	}
}

// TestE2E_TenGigahertzReadsAndIsRefusedOnWrite is the asymmetry OQ-1
// settles, end to end: this model READS both declared lengths and WRITES
// only the shape its document draws.
//
// The 65-byte record reads back at 10.25 GHz and survives the codeplug
// schema-4 round trip; writing it back is REFUSED before the wire, naming
// the field and the lift, because BuildLength is 64.
func TestE2E_TenGigahertzReadsAndIsRefusedOnWrite(t *testing.T) {
	t.Parallel()
	radio, s := openFake(t, RealHardware, []Option{WithConsentedUnverifiedWrites()}, e2eImage(
		fakeic905.WithRecord(0, 0, goldenRecord(10_250_000_000, 6).build()),
	)...)

	if got := s.Diagnostics905().ObservedRecordLength; got != 65 {
		t.Fatalf("ObservedRecordLength = %d, want 65", got)
	}

	ch, err := s.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Empty() || ch.Data.FreqHz != 10_250_000_000 {
		t.Fatalf("the 10 GHz channel read back as %+v", ch.Data)
	}

	// The schema-4 round trip, on the model that forced the widening.
	cp := &codeplug.Codeplug{
		Radio:    codeplug.RadioInfo{Model: s.Capabilities().Model, CATID: s.Capabilities().CATID},
		Channels: []codeplug.Channel{ch},
	}
	dir := t.TempDir()
	if err := codeplug.Save(dir+"/cp.json", cp); err != nil {
		t.Fatalf("Save: %v", err)
	}
	back, err := codeplug.Load(dir + "/cp.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if back.Schema != 4 {
		t.Errorf("the saved file is schema %d, want 4 — a frequency past uint32 forces it (D4)", back.Schema)
	}
	if back.Channels[0].Data.FreqHz != 10_250_000_000 {
		t.Errorf("after a round trip FreqHz = %d", back.Channels[0].Data.FreqHz)
	}

	// And writing it back is refused BEFORE the wire.
	before := len(setFrames(radio))
	_, err = s.WriteChannel(context.Background(), back.Channels[0])
	if err == nil {
		t.Fatal("the 10 GHz write was accepted")
	}
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("error = %v, want a *driver.WriteRefusedError", err)
	}
	if !slices.Contains(wre.Fields, spec.FieldFrequency) {
		t.Errorf("refused fields = %v, want frequency named", wre.Fields)
	}
	if !strings.Contains(wre.Reason, "ic905-R-06") {
		t.Errorf("reason = %q, want it to name the lift ic905-R-06", wre.Reason)
	}
	if n := len(setFrames(radio)); n != before {
		t.Errorf("%d set frames reached the fake", n-before)
	}
}

// TestE2E_ABroadcastFloodIsInvisibleToTheEngineAndOperationsComplete is
// R9-SPLIT branch (a), against a radio that NEVER GOES QUIET.
//
// Address filtering prevents false matches, not starvation — every drain,
// purge, quarantine and answer wait carries an ABSOLUTE deadline — and
// the IC-905's own transceive default is ASSUMED with no off-switch this
// tier ships, so a permanent flood is the case that matters.
//
// The broadcasts die at the accumulator's address filter before any
// engine event, so the idle timer never re-arms and Init simply SUCCEEDS.
// The engine's own counter stays a healthy zero while the adapter's
// climbs — two numbers answering two different questions.
func TestE2E_ABroadcastFloodIsInvisibleToTheEngineAndOperationsComplete(t *testing.T) {
	t.Parallel()
	radio, s := openFake(t, RealHardware, []Option{WithConsentedUnverifiedWrites()}, e2eImage(
		fakeic905.WithRecord(0, 0, goldenRecord(144_500_000, 5).build()),
		fakeic905.WithTransceiveBroadcasts(2*time.Millisecond, broadcastFrame),
	)...)

	d := s.Diagnostics905()
	if d.InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is true under a BROADCAST flood — those frames die at the address filter (R9-SPLIT branch (a))")
	}
	if d.UnexpectedFrames != 0 {
		t.Errorf("Diagnostics().UnexpectedFrames = %d, want 0 — a broadcast never becomes an engine event", d.UnexpectedFrames)
	}
	if d.Accumulator.Unexpected == 0 {
		t.Error("Accumulator.Unexpected = 0 — the broadcasts must be COUNTED on the adapter's side of the filter, not silently dropped")
	}
	if !d.Fingerprinted {
		t.Error("the probe did not fingerprint through the flood")
	}

	// AND EVERY OPERATION STILL COMPLETES, which is the half that matters:
	// the flood is still running throughout all of this.
	ch, err := s.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel under a flood: %v", err)
	}
	if ch.Empty() {
		t.Fatal("ReadChannel returned an empty channel under a flood")
	}
	if _, err := s.WriteChannel(context.Background(), writableChannel("G01-001")); err != nil {
		t.Fatalf("WriteChannel under a flood: %v", err)
	}
	if got, ok := radio.Record(0, 0); !ok || !bytes.Equal(got, goldenRecord(144_500_000, 5).build()) {
		t.Error("the write did not land through the flood")
	}
}

// TestE2E_AControllerAddressedFloodMakesInitNonfatalAndDiagnosed is
// R9-SPLIT branch (b) — the half that needs the fake's addressed-flood
// option, because only a controller-addressed frame reaches the engine at
// all.
//
// The initial drain hits its absolute cap, Init returns
// ErrDrainCapExceeded, and the driver treats it NONFATAL: a bounded
// initial drain cannot fail an open, because the line is noisy rather
// than wrong. It is RECORDED so an operator can see it.
func TestE2E_AControllerAddressedFloodMakesInitNonfatalAndDiagnosed(t *testing.T) {
	t.Parallel()
	_, s := openFake(t, RealHardware, nil, e2eImage(
		fakeic905.WithRecord(0, 0, goldenRecord(144_500_000, 5).build()),
		fakeic905.WithAddressedFlood(4*time.Millisecond, addressedFrame),
	)...)

	d := s.Diagnostics905()
	if !d.InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is false under a CONTROLLER-ADDRESSED flood — the drain must have reached its absolute cap and the open must record it")
	}
	if s.Identity().CATID == "" {
		t.Error("the session opened with no identity — the nonfatal branch must still probe")
	}
	// These frames DO reach the engine, which is the whole difference from
	// branch (a).
	if d.UnexpectedFrames == 0 {
		t.Error("Diagnostics().UnexpectedFrames = 0 — a controller-addressed frame is one the engine sees and counts")
	}
}

// TestE2E_TheDriverNeverWritesToTheRadioAtInit is the safety property
// every other test rests on, asserted against a radio that RECORDS WHAT
// IT WAS ASKED.
//
// CI-V Init transmits nothing (spec D2, adjudication 3): no transceive-off
// write, no clear, no 1A 05. The only commands this driver may put on a
// wire are 19 00 and the two 1A 00 forms, and the gate refuses everything
// else. Frames() is where that becomes checkable rather than asserted.
func TestE2E_TheDriverNeverWritesToTheRadioAtInit(t *testing.T) {
	t.Parallel()
	radio, _ := openFake(t, RealHardware, nil, e2eImage(
		fakeic905.WithRecord(0, 0, goldenRecord(144_500_000, 5).build()),
	)...)

	frames := radio.Frames()
	if len(frames) == 0 {
		t.Fatal("the fake received no frames at all")
	}
	if !bytes.Equal(frames[0], []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x19, 0x00, 0xFD}) {
		t.Errorf("the first frame is % X, want the 19 00 read — Init must transmit NOTHING", frames[0])
	}
	for i, f := range frames {
		if len(f) < 6 {
			t.Errorf("frame %d is %d bytes: % X", i, len(f), f)
			continue
		}
		cn, sc := f[4], f[5]
		switch {
		case cn == 0x19 && sc == 0x00:
		case cn == 0x1A && sc == 0x00:
		default:
			t.Errorf("frame %d carries command %02X %02X — this driver sends 19 00 and 1A 00 and nothing else: no 1A 05, no clear, no transceive-off", i, cn, sc)
		}
		// The clear form is 1A 00 with a lone FF where the record goes.
		// It is DOCUMENTED for this radio and implemented NOWHERE.
		if cn == 0x1A && sc == 0x00 && len(f) == memoryReadFrameLen+1 && f[10] == 0xFF {
			t.Errorf("frame %d is the documented CLEAR form: % X — this tier ships no erase path", i, f)
		}
	}
}

// TestE2E_MemorySetIsNeverRetransmitted pins, against a radio that
// records what it was asked, that an acknowledged write is still a WRITE:
// exactly ONE set frame ever arrives.
//
// RetryReads is zero on this class and Engine.Do refuses a non-zero value
// outright, so a retransmission is not representable in the spec — but
// the property worth pinning is the one on the WIRE, because resending an
// accepted set would write the channel twice, and this is the only
// vantage point from which the wire can be counted.
func TestE2E_MemorySetIsNeverRetransmitted(t *testing.T) {
	t.Parallel()
	radio, s := openFake(t, RealHardware, []Option{WithConsentedUnverifiedWrites()}, e2eImage(
		fakeic905.WithRecord(0, 1, goldenRecord(430_000_000, 5).build()),
	)...)

	res, err := s.WriteChannel(context.Background(), writableChannel("G01-002"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Fatalf("Steps = %+v, want one confirmed step", res.Steps)
	}
	if n := len(setFrames(radio)); n != 1 {
		t.Errorf("the fake received %d set frames, want exactly 1 — a write is NEVER resent", n)
	}
	if got, ok := radio.Record(0, 1); !ok || !bytes.Equal(got, goldenRecord(144_500_000, 5).build()) {
		t.Error("the single set frame did not land the golden record")
	}
}

// TestE2E_TheEngineIsClosedWithTheSession pins the ownership contract
// against a real pipe: after Close, the port is gone and a further
// exchange cannot reach the radio.
func TestE2E_TheEngineIsClosedWithTheSession(t *testing.T) {
	t.Parallel()
	radio := fakeic905.New(e2eImage(
		fakeic905.WithRecord(0, 0, goldenRecord(144_500_000, 5).build()),
	)...)
	t.Cleanup(func() { _ = radio.Close() })

	sess, err := New(RealHardware).Open(context.Background(), radio.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("second Close: %v — Close must be idempotent", err)
	}
	before := len(radio.Frames())
	if _, err := sess.ReadChannel(context.Background(), "G01-001"); err == nil {
		t.Error("a read succeeded after Close")
	}
	if n := len(radio.Frames()); n != before {
		t.Errorf("%d frames reached the radio after Close", n-before)
	}
}

// THE ONE TEST THIS FILE DOES NOT HAVE, AND WHY — a STOP, recorded here
// rather than worked around.
//
// The plan's Task 17 names an eighth end-to-end case: the answer-address
// check of ruling T2, driven by "the fake … configured to answer one read
// with a record addressed to a different channel". THE LANDED FAKE HAS NO
// SUCH OPTION. Its whole exported surface is New/Port/Close/Record/Frames
// plus WithLatency, WithIdentityToken, WithRecord, WithEmpty,
// WithTransceiveBroadcasts and WithAddressedFlood; nothing misaddresses an
// answer, and the fake is frozen.
//
// It is arguably right that it has none. A fake models a radio's STATE and
// answers consistently from it, so a misaddressed answer is a thing it
// cannot produce without being taught to lie — which is precisely the
// division of labour respondingport_test.go's own doc comment draws: the
// scripted port "can therefore serve deliberately WRONG answers … which is
// exactly what the error paths need and what a self-consistent fake will
// never produce".
//
// THE BEHAVIOUR IS NOT UNCOVERED. It is pinned driver-side, against the
// scripted port, in two places:
//
//   - TestReadChannel_AnAnswerForAnotherChannelIsAnErrorNotAStore — the
//     read fails with ErrAnswerMismatch, no channel is returned, and
//     Diagnostics905().AnswerMismatches counts it.
//   - TestProbe_ARecordForAnotherChannelIsFatalToTheOpen — the same check
//     during the probe, where it fails the open.
//
// What is missing is the end-to-end TWIN, and it stays missing until
// either the fake gains a misaddressing option or the tier decides it
// should not have one. Reported upward rather than improvised around.

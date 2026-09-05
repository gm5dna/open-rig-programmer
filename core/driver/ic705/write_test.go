// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// writableRecord is the manual's own golden record with its three DV call
// signs (and their TX-block copies) blanked to 0x20 — the profile's Fixed
// template value.
//
// IT IS THE WRITE PATH THIS TIER ACTUALLY SHIPS. The golden vector as
// transcribed carries CQCQCQ in the UR call sign, an area no spec.Field
// claims, so under E6 a write to a slot holding it is REFUSED rather than
// rewritten. Blanking those areas is what an ordinary FM/SSB channel looks
// like, and it is the only shape this tier can write.
func writableRecord(t *testing.T) []byte {
	t.Helper()
	rec := goldenRecord(t)
	for _, r := range [][2]int{{24, 47}, {71, 94}} {
		for i := r[0]; i <= r[1]; i++ {
			rec[i] = 0x20
		}
	}
	return rec
}

// writableSession opens a CONSENTED session against a radio holding a
// writable record at slot, and returns the session, the radio and the
// channel as read back.
func writableSession(t *testing.T, slot string) (*Session, *scriptedRadio, codeplug.Channel) {
	t.Helper()
	r := radioHolding(t, slot, writableRecord(t))
	sess := openSession(t, r, WithConsentedUnverifiedWrites())
	ch, err := sess.ReadChannel(context.Background(), slot)
	if err != nil {
		t.Fatalf("ReadChannel(%q): %v", slot, err)
	}
	if ch.Data == nil {
		t.Fatalf("slot %q read back empty", slot)
	}
	return sess, r, ch
}

// refusalNames reports whether err is a write refusal naming field.
func refusalNames(err error, field spec.Field) bool {
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		return false
	}
	for _, f := range refused.Fields {
		if f == field {
			return true
		}
	}
	return false
}

func TestWriteIsRefusedOnTheUnverifiedProfile(t *testing.T) {
	// The write guard, end to end: writeTrialsComplete is false, so every
	// field's Write is Unverified, CanWrite is false, and the capability
	// gate refuses before a frame is built.
	r := radioHolding(t, "G01-001", writableRecord(t))
	sess := openSession(t, r) // no consent
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	setsBefore := r.Sets()
	res, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel returned %v, want a write refusal", err)
	}
	if res.Steps == nil || len(res.Steps) != 0 {
		t.Errorf("Steps = %v, want an EMPTY, non-nil slice — a refusal before any frame was built has no sequence to describe, and the value is journalled as JSON", res.Steps)
	}
	if r.Sets() != setsBefore {
		t.Error("a refused write put a memory set on the wire")
	}
}

func TestWriteWithConsentSendsOneAcknowledgedFrame(t *testing.T) {
	sess, r, ch := writableSession(t, "G01-001")
	ch.Data.Tag = "RENAMED CH"
	setsBefore := r.Sets()

	res, err := sess.WriteChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 {
		t.Fatalf("Steps = %+v, want exactly one", res.Steps)
	}
	if res.Steps[0].Command != "1A 00" {
		t.Errorf("Steps[0].Command = %q, want \"1A 00\"", res.Steps[0].Command)
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps[0] = %+v, want Sent and Confirmed — the radio answered FB", res.Steps[0])
	}
	if got := r.Sets() - setsBefore; got != 1 {
		t.Errorf("the radio saw %d memory sets, want exactly 1", got)
	}
	// And the radio now holds the new name.
	stored, ok := r.SlotState(addrOf(t, "G01-001"))
	if !ok {
		t.Fatal("the slot is no longer occupied")
	}
	if got := strings.TrimRight(string(stored[95:111]), " "); got != "RENAMED CH" {
		t.Errorf("the stored name is %q, want \"RENAMED CH\"", got)
	}
}

func TestWriteNeverRetransmitsOnTimeout(t *testing.T) {
	// A CI-V memory set is ClassWriteWithAck with RetryReads 0: a timeout
	// leaves the write's outcome genuinely unknown, and the one thing that
	// must not happen is sending it again to find out.
	sess, r, ch := writableSession(t, "G01-001")
	ch.Data.Tag = "NO ANSWER"
	setsBefore := r.Sets()
	// Reads still work: the ladder's own preservation read must succeed,
	// or this would be testing a read timeout rather than a write one.
	r.IgnoreSets(true)

	res, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("WriteChannel returned %v, want a timeout", err)
	}
	if got := r.Sets() - setsBefore; got != 1 {
		t.Errorf("the radio saw %d memory sets, want exactly 1 — a retransmitted set could write the channel twice", got)
	}
	if len(res.Steps) != 1 || res.Steps[0].Sent {
		t.Errorf("Steps = %+v, want one step with Sent false — a timeout leaves the outcome unknowable", res.Steps)
	}
}

func TestAnFAIsARejectionNotASuccess(t *testing.T) {
	sess, r, ch := writableSession(t, "G01-001")
	// A tag the radio's own record cannot hold is not how this is
	// provoked — instead the slot is moved to an address the scripted
	// radio refuses outright, which is what a real NG answer looks like.
	ch.Slot = "G01-002"
	ch.Data.Tag = "REJECTED"
	r.RejectSets(true)

	res, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, transport.ErrRejected) {
		t.Fatalf("WriteChannel returned %v, want ErrRejected", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one step Sent but not Confirmed — an FA is an attributable outcome and it is a refusal", res.Steps)
	}
}

func TestEraseIsRefusedBeforeAnythingElse(t *testing.T) {
	// Spec D4 adjudication 19: no erase path at all this tier, though TWO
	// clear wire forms exist on this radio.
	sess, r, _ := writableSession(t, "G01-001")
	before := len(r.Transcript())
	res, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "G01-001"})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel of an empty channel returned %v, want a write refusal", err)
	}
	if !refusalNames(err, spec.FieldErase) {
		t.Errorf("the refusal %q does not name the erase field", err)
	}
	if len(res.Steps) != 0 {
		t.Errorf("Steps = %+v, want empty", res.Steps)
	}
	if len(r.Transcript()) != before {
		t.Error("an erase refusal put a frame on the wire")
	}
}

func TestANonKnownMappedFieldIsRefusedNotSynthesised(t *testing.T) {
	// R6, per field: only Known values are ever encoded, and a mapped
	// field that is not Known is REFUSED — never synthesised, never
	// preserved-by-cache.
	//
	// THE TWO TONE SPANS ARE NOT IN THIS TABLE, and their absence is the
	// T1(4) exception rather than an oversight: a not-Known tone is
	// resolved from the just-read record at rung 10, which is the radio's
	// OWN value rather than an invented one. The two cases that covers are
	// tested by TestToneOffChannelIsWritableByPreservingTheRadiosOwnNumber
	// (modify) and TestCreateWithAnUnknownToneIsRefused (create).
	for _, tc := range []struct {
		field  spec.Field
		break_ func(*codeplug.ChannelData)
	}{
		{spec.FieldTxFrequency, func(d *codeplug.ChannelData) { d.TxFreqHz = codeplug.FreqField{State: codeplug.Unknown} }},
		{spec.FieldDuplex, func(d *codeplug.ChannelData) { d.Duplex = codeplug.StringField{State: codeplug.Unknown} }},
		{spec.FieldOffset, func(d *codeplug.ChannelData) { d.OffsetHz = codeplug.FreqField{State: codeplug.Unknown} }},
		{spec.FieldToneMode, func(d *codeplug.ChannelData) { d.ToneMode = codeplug.StringField{State: codeplug.Unknown} }},
		{spec.FieldDTCSCode, func(d *codeplug.ChannelData) { d.DTCSCode = codeplug.IntField{State: codeplug.Unknown} }},
		{spec.FieldDTCSPolarity, func(d *codeplug.ChannelData) { d.DTCSPolarity = codeplug.StringField{State: codeplug.Unknown} }},
		{spec.FieldFilter, func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{State: codeplug.Unknown} }},
		{spec.FieldDataMode, func(d *codeplug.ChannelData) { d.DataMode = codeplug.BoolField{State: codeplug.Unknown} }},
	} {
		t.Run(string(tc.field), func(t *testing.T) {
			sess, r, ch := writableSession(t, "G01-001")
			before := len(r.Transcript())
			tc.break_(ch.Data)
			res, err := sess.WriteChannel(context.Background(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel returned %v, want a refusal", err)
			}
			if !refusalNames(err, tc.field) {
				t.Errorf("the refusal %q does not name %s", err, tc.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if len(r.Transcript()) != before {
				t.Error("ZERO bytes must reach the port: this refusal is locally decidable and precedes the read")
			}
		})
	}
}

func TestCreateIntoAnEmptySlotRequiresExplicitValues(t *testing.T) {
	// R6's last clause. A create with a bare frequency and mode is refused
	// with the missing fields named — never quietly completed with zeros,
	// which would write a tone, a filter and a duplex the caller never
	// chose.
	sess, r, full := writableSession(t, "G01-001")

	bare := codeplug.Channel{Slot: "G01-050", Data: &codeplug.ChannelData{
		FreqHz: 145500000,
		Mode:   "FM",
		Tag:    "NEW",
	}}
	before := len(r.Transcript())
	if _, err := sess.WriteChannel(context.Background(), bare); !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("a bare create returned %v, want a refusal", err)
	} else if !refusalNames(err, spec.FieldFilter) {
		t.Errorf("the refusal %q does not name the missing fields", err)
	}
	if len(r.Transcript()) != before {
		t.Error("a bare create put a frame on the wire")
	}

	// The same slot, with explicit values for every mapped field, is
	// written.
	complete := codeplug.Channel{Slot: "G01-050", Data: new(codeplug.ChannelData)}
	*complete.Data = *full.Data
	complete.Data.Tag = "CREATED"
	res, err := sess.WriteChannel(context.Background(), complete)
	if err != nil {
		t.Fatalf("a complete create: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one confirmed step", res.Steps)
	}
	if _, ok := r.SlotState(addrOf(t, "G01-050")); !ok {
		t.Error("the radio does not hold the created channel")
	}
}

func TestToneOffChannelIsWritableByPreservingTheRadiosOwnNumber(t *testing.T) {
	// T1(4). The channel's ToneTx/ToneRx are Unknown, because the read
	// mapped the wire zero that way (AdmitsTone(0) is false under any
	// legal range). The write is NOT refused and NOT synthesised: rung 10
	// copies the JUST-READ record's tone number verbatim.
	rec := fullRecord(addrOf(t, "G01-001"))
	rec.ToneTXDeciHz = civ.Available[uint64](0)
	rec.ToneRXDeciHz = civ.Available[uint64](0)
	rec.ToneMode = civ.Available("OFF")
	// BOTH records are seeded BEFORE the session opens, so the inventory
	// walk sees them: a slot the walk never visited is refused as an
	// occupied surprise (ruling T3), which is a different rule and has its
	// own test.
	rec2 := fullRecord(addrOf(t, "G02-001"))
	rec2.ToneTXDeciHz = civ.Available[uint64](885)
	rec2.ToneRXDeciHz = civ.Available[uint64](885)
	r := newScriptedRadio(t, radioImage{records: map[civ.ChannelAddress][]byte{
		addrOf(t, "G01-001"): encodedRecord(t, rec),
		addrOf(t, "G02-001"): encodedRecord(t, rec2),
	}})
	sess := openSession(t, r, WithConsentedUnverifiedWrites())
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.ToneTx.State != codeplug.Unknown {
		t.Fatalf("the fixture's ToneTx came back %v, want Unknown", ch.Data.ToneTx)
	}
	ch.Data.Tag = "TONE OFF"
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	stored, _ := r.SlotState(addrOf(t, "G01-001"))
	for _, span := range [][2]int{{11, 13}, {14, 16}} {
		for i := span[0]; i <= span[1]; i++ {
			if stored[i] != 0x00 {
				t.Errorf("record offset %d went out as %#02x, want 0x00 — byte-identical to what the radio already held", i, stored[i])
			}
		}
	}

	// THE HARDER HALF: preservation means the radio's value, whatever it
	// is. A record whose tone spans hold 88.5 Hz, presented with an
	// Unknown ToneTx, must put 88.5 Hz back out — not zero, not a default.
	ch2, err := sess.ReadChannel(context.Background(), "G02-001")
	if err != nil {
		t.Fatalf("ReadChannel(G02-001): %v", err)
	}
	ch2.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	ch2.Data.ToneRx = codeplug.ToneField{State: codeplug.Unknown}
	ch2.Data.Tag = "PRESERVED"
	if _, err := sess.WriteChannel(context.Background(), ch2); err != nil {
		t.Fatalf("WriteChannel(G02-001): %v", err)
	}
	stored2, _ := r.SlotState(addrOf(t, "G02-001"))
	want := []byte{0x00, 0x08, 0x85}
	for i, b := range want {
		if stored2[11+i] != b || stored2[14+i] != b {
			t.Fatalf("the preserved tone went out as % X / % X, want % X — preservation is the RADIO's value, not a default", stored2[11:14], stored2[14:17], want)
		}
	}
}

func TestCreateWithAnUnknownToneIsRefused(t *testing.T) {
	// O-11 / T1(5). A CREATE has no prior record, so there is nothing to
	// preserve, and this radio's CI-V Reference Guide prints NO default
	// tone anywhere — matrix erratum 4 established that it marks no
	// factory defaults at all, and the Basic Manual's admission covers
	// three values that do not include one. Refusing is the ABSENCE of an
	// assumption; a synthesised 88.5 Hz would be an assumption this driver
	// has not filed.
	sess, r, full := writableSession(t, "G01-001")
	create := codeplug.Channel{Slot: "G01-050", Data: new(codeplug.ChannelData)}
	*create.Data = *full.Data
	create.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	create.Data.Tag = "NO TONE"

	setsBefore := r.Sets()
	res, err := sess.WriteChannel(context.Background(), create)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel returned %v, want a refusal", err)
	}
	if !refusalNames(err, spec.FieldToneTx) {
		t.Errorf("the refusal %q does not name tone_tx", err)
	}
	if len(res.Steps) != 0 || r.Sets() != setsBefore {
		t.Error("a refused create sent something")
	}
}

func TestAnAddToASlotTheInventoryNeverSawIsRefusedIfOccupied(t *testing.T) {
	// T3, this plan's own ruling, and BOTH halves or the rule is untested.
	// G11-001 is outside the bounded walk's ten groups, so the
	// materialised inventory does not know it.
	r := radioHolding(t, "G01-001", writableRecord(t))
	r.SetRecord(addrOf(t, "G11-001"), writableRecord(t))
	sess := openSession(t, r, WithConsentedUnverifiedWrites())
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if sess.inventoryKnows("G11-001") {
		t.Fatal("the bounded walk materialised G11-001 — the fixture is not testing what it claims")
	}

	surprise := codeplug.Channel{Slot: "G11-001", Data: new(codeplug.ChannelData)}
	*surprise.Data = *ch.Data
	surprise.Data.Tag = "OVERWRITE"
	setsBefore := r.Sets()
	res, err := sess.WriteChannel(context.Background(), surprise)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel returned %v, want an occupied-surprise refusal", err)
	}
	// THE MESSAGE IS PINNED VERBATIM, because a refusal that names a
	// remedy is only as good as the remedy: an earlier wording offered
	// "or read this slot first", which cannot work — ReadChannel never
	// adds a slot to the inventory, so a user who did that met this
	// refusal again. The one remedy named here changes the answer, and
	// the sentence after it states the bound rather than a second
	// remedy. That bound now NAMES WithFullInventoryWalk() again (side
	// lanes fix round 1, review icom-minors-review-opus.md MEDIUM-1): the
	// earlier "no setting that widens the walk" wording was untrue, since
	// this package exports the option — naming it while saying in the
	// same clause that no registered composition passes it is the honest
	// bound, not the removed remedy reappearing, and matches the
	// IC-R8600's and IC-905's identical rungs.
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("%v is not a *driver.WriteRefusedError", err)
	}
	want := "slot G11-001 holds a record this session's inventory never saw, and writing here would overwrite a channel nobody has looked at: this session's walk covered display groups G01-G10, so re-open the session to run discovery again — a slot outside that range stays unlisted, and nothing on this build's command line or in its window widens it (the driver's own WithFullInventoryWalk is a Go-level option no registered composition passes). Reading the slot does not help: ReadChannel never adds one to the inventory"
	if refused.Reason != want {
		t.Errorf("the refusal reads\n  %q\nwant\n  %q", refused.Reason, want)
	}
	if strings.Contains(refused.Reason, "read this slot first") {
		t.Error("the refusal still offers a remedy that cannot work")
	}
	if strings.Contains(refused.Reason, "no setting that widens") {
		t.Error("the refusal still claims no setting exists when the driver exports one")
	}
	if len(res.Steps) != 0 || r.Sets() != setsBefore {
		t.Error("the occupied-surprise refusal sent something")
	}
	stored, ok := r.SlotState(addrOf(t, "G11-001"))
	if !ok || strings.TrimRight(string(stored[95:111]), " ") == "OVERWRITE" {
		t.Error("the seeded record at G11-001 was overwritten")
	}

	// THE MIRROR CASE: the same unvisited slot, but the radio has nothing
	// there — the add PROCEEDS.
	fresh := codeplug.Channel{Slot: "G12-001", Data: new(codeplug.ChannelData)}
	*fresh.Data = *ch.Data
	fresh.Data.Tag = "FRESH ADD"
	if _, err := sess.WriteChannel(context.Background(), fresh); err != nil {
		t.Fatalf("an add to an unvisited but EMPTY slot was refused: %v", err)
	}
	if _, ok := r.SlotState(addrOf(t, "G12-001")); !ok {
		t.Error("the add did not reach the radio")
	}
}

func TestFrequencyAtOrAbove500MHzIsRefused(t *testing.T) {
	// The ceiling comes from the printed "1 GHz digit: (fixed)" and
	// "100 MHz digit: 0 ~ 4" leaders (PDF p.18, folio 17; matrix erratum
	// 7), which no byte-width check can see: five packed-BCD bytes hold
	// 500 MHz perfectly well, so civ cannot catch this and this rung is
	// the only thing between a consented write and a digit the manual
	// bounds at 4.
	for _, tc := range []struct {
		name    string
		set     func(*codeplug.ChannelData, uint64)
		field   spec.Field
		refused bool
		value   uint64
	}{
		{"FreqHz accepted", func(d *codeplug.ChannelData, v uint64) { d.FreqHz = v }, spec.FieldFrequency, false, 499999999},
		{"FreqHz refused", func(d *codeplug.ChannelData, v uint64) { d.FreqHz = v }, spec.FieldFrequency, true, 500000000},
		{"TxFreqHz accepted", func(d *codeplug.ChannelData, v uint64) {
			d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: v}
		}, spec.FieldTxFrequency, false, 499999999},
		{"TxFreqHz refused", func(d *codeplug.ChannelData, v uint64) {
			d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: v}
		}, spec.FieldTxFrequency, true, 500000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, r, ch := writableSession(t, "G01-001")
			before := len(r.Transcript())
			tc.set(ch.Data, tc.value)
			_, err := sess.WriteChannel(context.Background(), ch)
			if !tc.refused {
				if err != nil {
					t.Fatalf("WriteChannel(%d) = %v, want acceptance at the boundary", tc.value, err)
				}
				return
			}
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel(%d) = %v, want a refusal", tc.value, err)
			}
			if !refusalNames(err, tc.field) {
				t.Errorf("the refusal %q does not name %s", err, tc.field)
			}
			if !strings.Contains(err.Error(), "500") {
				t.Errorf("the refusal %q does not name the printed ceiling", err)
			}
			if len(r.Transcript()) != before {
				t.Error("the refusal is locally decidable and must precede the read")
			}
		})
	}
}

func TestOffsetAboveTheManualsCeilingIsRefused(t *testing.T) {
	// The offset field's three BCD bytes have a FIXED 10 MHz digit (PDF
	// p.18, folio 17; matrix erratum 8: 9.9999 MHz, not 9.99), and civ's
	// span would happily write a sixth digit into it.
	sess, r, ch := writableSession(t, "G01-001")
	ch.Data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 9999900}
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("9 999 900 Hz was refused: %v", err)
	}

	before := len(r.Transcript())
	ch.Data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 10000000}
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("10 000 000 Hz returned %v, want a refusal", err)
	}
	if !refusalNames(err, spec.FieldOffset) {
		t.Errorf("the refusal %q does not name the offset field", err)
	}
	if len(r.Transcript()) != before {
		t.Error("the offset refusal put a frame on the wire")
	}
}

func TestANonEmptyInvalidModeIsRefusedBeforeAnyWireTraffic(t *testing.T) {
	// F3: a non-empty mode outside this radio's vocabulary used to be
	// refused only by civ's own encoder (BuildMemorySet), which is AFTER
	// rung 9's preservation read has already put a frame on the wire.
	// Mode membership is decidable locally, from the channel and s.caps
	// alone, so T5 requires it to precede ALL wire traffic — zero frames,
	// not one.
	sess, r, ch := writableSession(t, "G01-001")
	before := len(r.Transcript())
	ch.Data.Mode = "INVALID"
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel(mode=INVALID) = %v, want a refusal", err)
	}
	if !refusalNames(err, spec.FieldMode) {
		t.Errorf("the refusal %q does not name the mode field", err)
	}
	if got := len(r.Transcript()) - before; got != 0 {
		t.Errorf("the invalid-mode refusal put %d frames on the wire, want 0 — it must precede the preservation read", got)
	}
}

func TestAStarMarkedChannelIsRefusedNotDemoted(t *testing.T) {
	// O-6 + E6. The radio's record holds 0x01 at offset 0 — a ★1 Select
	// marking, in the nibble this profile deliberately leaves UNMAPPED.
	// An earlier draft would have written 0x00 there and silently
	// un-marked the channel.
	rec := writableRecord(t)
	rec[0] = 0x01
	r := radioHolding(t, "G01-001", rec)
	sess := openSession(t, r, WithConsentedUnverifiedWrites())
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	ch.Data.Tag = "RENAME ME"
	setsBefore := r.Sets()
	res, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel returned %v, want a preservation refusal", err)
	}
	if !strings.Contains(err.Error(), "Select") && !strings.Contains(err.Error(), "Split") {
		t.Errorf("the refusal %q does not name the marking it is protecting", err)
	}
	if len(res.Steps) != 0 || r.Sets() != setsBefore {
		t.Error("the refusal sent something")
	}
	stored, _ := r.SlotState(addrOf(t, "G01-001"))
	if stored[0] != 0x01 {
		t.Errorf("record offset 0 is now %#02x — the radio's own marking was rewritten", stored[0])
	}
}

func TestWriteRefusesAChannelCarryingDVRouting(t *testing.T) {
	// The golden record AS TRANSCRIBED carries CQCQCQ in the UR call sign
	// — an area no spec.Field claims and this tier's template fixes at
	// 0x20. E6 rules: refuse, with the reason named, never rewrite.
	r := radioHolding(t, "G01-001", goldenRecord(t))
	sess := openSession(t, r, WithConsentedUnverifiedWrites())
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	ch.Data.Tag = "RENAME ME"
	setsBefore := r.Sets()
	_, err = sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel returned %v, want a preservation refusal", err)
	}
	if !strings.Contains(err.Error(), "call sign") {
		t.Errorf("the refusal %q does not name the DV routing it is protecting", err)
	}
	if r.Sets() != setsBefore {
		t.Error("the refusal sent something")
	}
	stored, _ := r.SlotState(addrOf(t, "G01-001"))
	if string(stored[24:30]) != "CQCQCQ" {
		t.Errorf("the UR call sign is now %q — it must be untouched", stored[24:30])
	}
}

func TestWriteRefusesASplitOnChannel(t *testing.T) {
	// Split ON lives in the high nibble of record offset 0, which is
	// unmapped for the same reason the Select marking is.
	rec := writableRecord(t)
	rec[0] = 0x10
	r := radioHolding(t, "G01-001", rec)
	sess := openSession(t, r, WithConsentedUnverifiedWrites())
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	ch.Data.Tag = "RENAME ME"
	if _, err := sess.WriteChannel(context.Background(), ch); !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel returned %v, want a preservation refusal", err)
	}
}

func TestWriteRefusesADigitalSquelchSetting(t *testing.T) {
	// Record offset 10 (data-area position 15) is the digital squelch
	// setting, and offset 20 the DV code squelch: both unmapped, both
	// refused rather than rewritten.
	for _, offset := range []int{10, 20, 57, 67} {
		rec := writableRecord(t)
		rec[offset] = 0x01
		r := radioHolding(t, "G01-001", rec)
		sess := openSession(t, r, WithConsentedUnverifiedWrites())
		ch, err := sess.ReadChannel(context.Background(), "G01-001")
		if err != nil {
			t.Fatalf("offset %d: ReadChannel: %v", offset, err)
		}
		ch.Data.Tag = "RENAME ME"
		if _, err := sess.WriteChannel(context.Background(), ch); !errors.Is(err, driver.ErrWriteRefused) {
			t.Errorf("offset %d: WriteChannel returned %v, want a preservation refusal", offset, err)
		}
	}
}

func TestWriteAcceptsAChannelWhoseUnmappedBytesMatchTheTemplate(t *testing.T) {
	// The other side of E6: an ordinary FM channel with no DV routing, no
	// digital squelch, no Split and no ★ marking IS writable, and that is
	// the whole of what this tier ships.
	sess, r, ch := writableSession(t, "G01-001")
	ch.Data.Tag = "ORDINARY"
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("an ordinary channel was refused: %v", err)
	}
	if r.Sets() != 1 {
		t.Errorf("the radio saw %d sets, want 1", r.Sets())
	}
}

func TestEveryLocalRefusalPrecedesTheRead(t *testing.T) {
	// T5's ordering, ASSERTED rather than assumed. Rungs 1-8 are local:
	// for one failing example of each, the port sees ZERO frames — not
	// even the preservation read. The two read-dependent refusals see
	// EXACTLY ONE, the read itself.
	local := []struct {
		rung int
		name string
		make func(t *testing.T, base codeplug.Channel) codeplug.Channel
	}{
		{1, "malformed slot", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			base.Slot = "G101-005"
			return base
		}},
		{1, "channel outside the group's space", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			base.Slot = "G01-101"
			return base
		}},
		{3, "erase", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			return codeplug.Channel{Slot: base.Slot}
		}},
		{4, "invalid tri-state", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			d := *base.Data
			d.ToneTx = codeplug.ToneField{State: codeplug.Unknown, Value: 885} // a non-Known field must carry zero
			base.Data = &d
			return base
		}},
		{5, "capability gate", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			d := *base.Data
			d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
			base.Data = &d
			return base
		}},
		{6, "mandatory-Known", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			d := *base.Data
			d.Filter = codeplug.StringField{State: codeplug.Unknown}
			base.Data = &d
			return base
		}},
		{6, "invalid mode", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			d := *base.Data
			d.Mode = "INVALID"
			base.Data = &d
			return base
		}},
		{7, "range ceiling", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			d := *base.Data
			d.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 10000000}
			base.Data = &d
			return base
		}},
		{8, "unencodable value", func(t *testing.T, base codeplug.Channel) codeplug.Channel {
			d := *base.Data
			d.Tag = strings.Repeat("X", 40) // longer than this radio's 16-byte name
			base.Data = &d
			return base
		}},
	}
	for _, tc := range local {
		t.Run(tc.name, func(t *testing.T) {
			sess, r, base := writableSession(t, "G01-001")
			before := len(r.Transcript())
			ch := tc.make(t, base)
			if _, err := sess.WriteChannel(context.Background(), ch); err == nil {
				t.Fatalf("rung %d (%s) did not refuse", tc.rung, tc.name)
			}
			if got := len(r.Transcript()) - before; got != 0 {
				t.Errorf("rung %d (%s) put %d frames on the wire — rungs 1-8 are locally decidable and every one precedes ALL wire traffic", tc.rung, tc.name, got)
			}
		})
	}

	// RUNG 2, the bank check, which needs a doctored session to reach at
	// all. In the shipped configuration rungs 1 and 2 agree by
	// construction — slotToAddress's space IS the two banks — so rung 2 is
	// defence in depth, and the only way to exercise it is to take a bank
	// away from a session that would otherwise carry it. It is worth
	// exercising because the CAPABILITY SET, not slots.go, is what every
	// other layer of this project enforces against, and doc.go's O-9
	// section names this check as one of the two compensating controls for
	// the residual gate width.
	t.Run("rung 2 bank check", func(t *testing.T) {
		sess, r, base := writableSession(t, "G01-001")
		var banks []spec.Bank
		for _, b := range sess.caps.Banks {
			if b.ID != spec.BankCall {
				banks = append(banks, b)
			}
		}
		caps := sess.caps
		caps.Banks = banks
		sess.caps = caps

		ch := base
		ch.Slot = "G101-001" // parses at rung 1, in no bank this session now has
		if _, _, err := slotToAddress(ch.Slot); err != nil {
			t.Fatalf("the fixture slot must PASS rung 1: %v", err)
		}
		before := len(r.Transcript())
		_, err := sess.WriteChannel(context.Background(), ch)
		if !errors.Is(err, driver.ErrWriteRefused) {
			t.Fatalf("WriteChannel returned %v, want a refusal from the bank check", err)
		}
		if !strings.Contains(err.Error(), "not in any bank this session supports") {
			t.Errorf("the refusal %q is not rung 2's", err)
		}
		if got := len(r.Transcript()) - before; got != 0 {
			t.Errorf("rung 2 put %d frames on the wire", got)
		}
	})

	// Rung 11, the E6 template check: EXACTLY ONE frame, the read.
	t.Run("rung 11 template", func(t *testing.T) {
		r := radioHolding(t, "G01-001", goldenRecord(t)) // CQCQCQ in the UR call sign
		sess := openSession(t, r, WithConsentedUnverifiedWrites())
		ch, err := sess.ReadChannel(context.Background(), "G01-001")
		if err != nil {
			t.Fatalf("ReadChannel: %v", err)
		}
		ch.Data.Tag = "RENAME ME"
		before := len(r.Transcript())
		if _, err := sess.WriteChannel(context.Background(), ch); err == nil {
			t.Fatal("the template check did not refuse")
		}
		if got := len(r.Transcript()) - before; got != 1 {
			t.Errorf("the template refusal put %d frames on the wire, want exactly 1 — the single read it necessarily follows", got)
		}
	})

	// Rung 12, the occupied surprise: likewise exactly one.
	t.Run("rung 12 occupied surprise", func(t *testing.T) {
		r := radioHolding(t, "G01-001", writableRecord(t))
		r.SetRecord(addrOf(t, "G11-001"), writableRecord(t))
		sess := openSession(t, r, WithConsentedUnverifiedWrites())
		ch, err := sess.ReadChannel(context.Background(), "G01-001")
		if err != nil {
			t.Fatalf("ReadChannel: %v", err)
		}
		ch.Slot = "G11-001"
		ch.Data.Tag = "SURPRISE"
		before := len(r.Transcript())
		if _, err := sess.WriteChannel(context.Background(), ch); err == nil {
			t.Fatal("the occupied-surprise check did not refuse")
		}
		if got := len(r.Transcript()) - before; got != 1 {
			t.Errorf("the occupied-surprise refusal put %d frames on the wire, want exactly 1", got)
		}
	})
}

func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	// The set derives from THIS MODEL'S matrix §2, not from the Yaesu
	// contract's "six unconditional fields" (which are frequency, mode,
	// clarifier, ctcss_state, shift and tag — three of them Unsupported
	// here, so an unconditional request for them could never pass the
	// capability gate). Order is part of the contract because it is the
	// order WriteRefusedError names fields in.
	sess, _, ch := writableSession(t, "G01-001")
	_ = sess

	want := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldTxFrequency, spec.FieldDuplex, spec.FieldOffset,
		spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
		spec.FieldDTCSCode, spec.FieldDTCSPolarity, spec.FieldFilter,
		spec.FieldDataMode,
	}
	got := requestedFields(*ch.Data)
	if len(got) != len(want) {
		t.Fatalf("requestedFields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("requestedFields[%d] = %s, want %s (full: %v)", i, got[i], want[i], got)
		}
	}

	// The Yaesu-shaped six are requested WHEN PRESENT — that is the whole
	// point of the Wave-1 C2 contract: a cross-loaded Yaesu channel is
	// then REFUSED by the capability gate rather than silently dropped.
	d := *ch.Data
	d.ClarHz = 100
	d.CTCSS = "ENC"
	d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 885}
	d.Shift = "PLUS"
	d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
	d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
	got = requestedFields(d)
	for _, f := range []spec.Field{spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift, spec.FieldTagDisplay, spec.FieldScanSkip} {
		found := false
		for _, g := range got {
			if g == f {
				found = true
			}
		}
		if !found {
			t.Errorf("a channel carrying %s does not request it — it would be silently dropped instead of refused", f)
		}
	}
}

func TestDirectSessionWriteRefusesEachKnownUnsupportedField(t *testing.T) {
	// The Wave-1 C2 class, per field: a Known Yaesu-shaped field on a
	// direct Session.WriteChannel call is REFUSED by the capability gate,
	// not dropped by the codec.
	for _, tc := range []struct {
		field spec.Field
		set   func(*codeplug.ChannelData)
	}{
		{spec.FieldClarifier, func(d *codeplug.ChannelData) { d.ClarHz = 100 }},
		{spec.FieldCTCSSState, func(d *codeplug.ChannelData) { d.CTCSS = "ENC" }},
		{spec.FieldCTCSSTone, func(d *codeplug.ChannelData) {
			d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 885}
		}},
		{spec.FieldShift, func(d *codeplug.ChannelData) { d.Shift = "PLUS" }},
		{spec.FieldTagDisplay, func(d *codeplug.ChannelData) {
			d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
		}},
		{spec.FieldScanSkip, func(d *codeplug.ChannelData) {
			d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
		}},
	} {
		t.Run(string(tc.field), func(t *testing.T) {
			sess, r, ch := writableSession(t, "G01-001")
			before := len(r.Transcript())
			tc.set(ch.Data)
			_, err := sess.WriteChannel(context.Background(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel returned %v, want a capability refusal", err)
			}
			if !refusalNames(err, tc.field) {
				t.Errorf("the refusal %q does not name %s", err, tc.field)
			}
			if len(r.Transcript()) != before {
				t.Error("the capability refusal put a frame on the wire")
			}
		})
	}
}

func TestBudgetIsNeverEnforcedOnTheWire(t *testing.T) {
	// Over-budget is refused at Diff time (inventory_test.go), never sent.
	// This driver has NO budget check of its own and must not grow one:
	// what an IC-705 does when over budget is undocumented
	// (ic705-group-budget / L-BUDGET-CEILING, and the separate
	// over-budget entry, lift L-OVERBUDGET).
	records := map[civ.ChannelAddress][]byte{}
	rec := writableRecord(t)
	for g := 1; g <= 6; g++ {
		for c := 1; c <= 100; c++ {
			records[addrOf(t, spec.SparseSlot(g, c))] = rec
		}
	}
	r := newScriptedRadio(t, radioImage{records: records})
	sess := openSession(t, r, WithConsentedUnverifiedWrites())
	if n := sess.SessionInfo().InventorySlots; n <= 500 {
		t.Fatalf("the fixture materialised %d slots, want more than the 500-channel budget", n)
	}
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	ch.Data.Tag = "OVER BUDGET"
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("the driver refused a write on a radio holding more than its budget: %v — that refusal belongs at Diff time and nowhere else", err)
	}
}

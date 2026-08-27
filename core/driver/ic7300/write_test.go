// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	ic7300civ "github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// setFrames returns every 1A 00 SET frame the peer saw. A set is a 1A 00
// frame longer than the nine-byte read.
func setFrames(peer *respondingPort) [][]byte {
	var out [][]byte
	for _, f := range peer.Received() {
		if cn, sc, ok := civ.FrameCommand(f); ok && cn == 0x1A && sc == 0x00 && len(f) > 9 {
			out = append(out, f)
		}
	}
	return out
}

// recordOf returns the record bytes of a 1A 00 set frame: everything after
// FE FE <to> <from> 1A 00 <ch-hi> <ch-lo> and before the terminating FD.
func recordOf(frame []byte) []byte { return frame[8 : len(frame)-1] }

// --- The refusal ladder, in order, each with its own test. ---

func TestWriteChannel_RefusesAnUnparseableSlot(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	for _, slot := range []string{"", "1", "0001", "100", "000", "P3", "M-01"} {
		ch := channelFor(slot)
		before := len(peer.Received())
		_, err := sess.WriteChannel(context.Background(), ch)
		if !errors.Is(err, driver.ErrWriteRefused) {
			t.Errorf("WriteChannel(%q) error = %v, want ErrWriteRefused", slot, err)
		}
		if after := len(peer.Received()); after != before {
			t.Errorf("WriteChannel(%q) reached the wire before deciding the slot was unknown", slot)
		}
	}
}

// The SECOND rung, and it is not the first one wearing a different hat: a
// slot may be perfectly well formed and still be in no bank THIS SESSION
// has. The session's effective banks are narrowed here to make the rung
// reachable, which is the only way to reach it on a model whose two banks
// are both static.
func TestWriteChannel_RefusesASlotInNoBank(t *testing.T) {
	peer := newRespondingPort(t, withRecord(100, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	s := session(t, sess)
	var kept []spec.Bank
	for _, b := range s.caps.Banks {
		if b.ID != spec.BankScan {
			kept = append(kept, b)
		}
	}
	s.caps.Banks = kept

	before := len(peer.Received())
	_, err := sess.WriteChannel(context.Background(), channelFor("P1"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want ErrWriteRefused — \"this radio has no such channel\" and \"this channel is not writable\" are different refusals and must read differently", err)
	}
	if after := len(peer.Received()); after != before {
		t.Error("the bank refusal reached the wire — it is rung 2, before the read")
	}
}

// erase, and it must precede the FieldState checks STRUCTURALLY: an empty
// channel has no Data at all, and every check below it dereferences one.
func TestWriteChannel_RefusesAnEmptyChannel(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "001"})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel(empty) = %v, want ErrWriteRefused", err)
	}
	if !strings.Contains(err.Error(), string(spec.FieldErase)) {
		t.Errorf("refusal %q does not name erase", err)
	}
}

func TestWriteChannel_RefusesAFieldThisSessionCannotWrite(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	// NO CONSENT: the RealHardware profile grades every field Unverified,
	// which is unwritable, because no IC-7300 has ever been asked anything.
	sess := openSession(t, peer)
	before := len(peer.Received())
	_, err := sess.WriteChannel(context.Background(), channelFor("001"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want ErrWriteRefused", err)
	}
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want a *driver.WriteRefusedError naming the fields", err)
	}
	for _, f := range []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldTag, spec.FieldTxFrequency, spec.FieldFilter, spec.FieldDataMode, spec.FieldToneMode} {
		if !slices.Contains(refused.Fields, f) {
			t.Errorf("refusal does not name %s — the seven unconditional fields are requested on EVERY write, changed or not, because the 1A 00 record always carries them", f)
		}
	}
	if after := len(peer.Received()); after != before {
		t.Error("the capability refusal reached the wire — it is rung 5, before the read")
	}
}

func TestWriteChannel_RefusesAKnownValueForAFieldTheRecordLacks(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	ch := channelFor("001")
	ch.Data.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
	before := len(peer.Received())
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want ErrWriteRefused — a Known value is a REQUEST, and this project refuses a request it cannot honour rather than dropping it", err)
	}
	if !strings.Contains(err.Error(), string(spec.FieldScanSkip)) {
		t.Errorf("refusal %q does not name scan_skip", err)
	}
	if after := len(peer.Received()); after != before {
		t.Error("the refusal reached the wire")
	}
}

// The SCAN bank's printed constraint: ③ must be zero for P1 and P2
// (matrix §3.16 A8, "ⓘSet both 0 for P1 and P2.").
//
// IT IS READ-DEPENDENT, and it therefore sits with E6's own check AFTER the
// single read rather than among the locally decidable rungs: the SELECT
// value it judges is the one the RADIO holds, and no field of the channel
// carries it (D4). One read reaches the wire and no set does.
func TestWriteChannel_ScanEdgeRefusesANonZeroSelectByte(t *testing.T) {
	rec := append([]byte(nil), populatedRecord...)
	rec[0] = 0x01 // ③: SELECT = SEL1 on a scan edge
	peer := newRespondingPort(t, withRecord(100, rec))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	_, err := sess.WriteChannel(context.Background(), channelFor("P1"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want ErrWriteRefused", err)
	}
	if !strings.Contains(err.Error(), "P1") {
		t.Errorf("refusal %q does not name the slot", err)
	}
	if n := len(setFrames(peer)); n != 0 {
		t.Errorf("%d set frames reached the wire despite the scan-edge refusal", n)
	}
}

// --- The choreography. ---

func TestWriteChannel_SendsOneSetAndWaitsForFB(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	res, err := sess.WriteChannel(context.Background(), channelFor("001"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "1A 00" {
		t.Fatalf("Steps = %+v, want exactly one 1A 00 step — this radio's write choreography IS one frame", res.Steps)
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps[0] = %+v, want Sent and Confirmed — a CI-V memory set is ACKNOWLEDGED, so Confirmed here means the radio's own FB arrived, not merely that nothing was heard", res.Steps[0])
	}
	sets := setFrames(peer)
	if len(sets) != 1 {
		t.Fatalf("the peer saw %d set frames, want exactly 1", len(sets))
	}
	if len(sets[0]) != 48 {
		t.Errorf("the set frame is %d bytes, want 48 (2 preamble + 2 address + 2 command + 2 channel + 39 record + 1 terminator)", len(sets[0]))
	}
	if d := civDiagnostics(t, sess); d.Acknowledgements == 0 {
		t.Error("AccumulatorStats().Acknowledgements = 0 after a confirmed set — the FB is counted where it arrives")
	}
}

func TestWriteChannel_FAIsAnAttributableRefusal(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord), withRejectSets())
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	res, err := sess.WriteChannel(context.Background(), channelFor("001"))
	if err == nil {
		t.Fatal("WriteChannel succeeded against a radio that answered FA")
	}
	if !errors.Is(err, transport.ErrRejected) {
		t.Errorf("error = %v, want transport.ErrRejected", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want Sent true and Confirmed false — the frame WAS transmitted and the radio explicitly refused it, which is an ATTRIBUTABLE outcome", res.Steps)
	}
}

func TestWriteChannel_ZeroFramesWhenTheGateRefuses(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer) // no consent
	before := len(peer.Received())
	res, err := sess.WriteChannel(context.Background(), channelFor("001"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("error = %v, want ErrWriteRefused", err)
	}
	if res.Steps == nil {
		t.Error("Steps is nil — a refusal reports an EXPLICITLY EMPTY step list, because the clone service journals this and a nil slice marshals as JSON null, which an auditor would have to read as \"unknown\"")
	}
	if len(res.Steps) != 0 {
		t.Errorf("Steps = %+v, want empty — no frame was ever built, so nothing was attempted", res.Steps)
	}
	if after := len(peer.Received()); after != before {
		t.Errorf("the transcript grew by %d frames during a refusal that precedes all wire traffic", after-before)
	}
}

// A ClassWriteWithAck command is NEVER retransmitted on timeout (spec D2).
// The proof is a frame count, not a comment: a peer that answers the read
// and then goes silent must see the set frame exactly once.
func TestWriteChannel_TimeoutIsNEVERRetransmitted(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord), withNoAnswerToSets())
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	res, err := sess.WriteChannel(context.Background(), channelFor("001"))
	if err == nil {
		t.Fatal("WriteChannel succeeded against a peer that never acknowledges")
	}
	if len(res.Steps) != 1 || res.Steps[0].Sent {
		t.Errorf("Steps = %+v, want Sent false — a timeout leaves the frame's fate UNATTRIBUTABLE: the host cannot tell whether it reached the radio", res.Steps)
	}
	sets := 0
	for _, f := range peer.Received() {
		if cn, sc, ok := civ.FrameCommand(f); ok && cn == 0x1A && sc == 0x00 && len(f) == 48 {
			sets++
		}
	}
	if sets != 1 {
		t.Errorf("the peer saw %d set frames, want exactly 1 — ClassWriteWithAck is never retransmitted on timeout, and a second copy would be a second write to a radio the caller never asked for (spec D2)", sets)
	}
}

// --- E6, and the split flag. ---

// E6: the unmapped region must equal the Fixed template, or the write is
// REFUSED. On this pair that region is byte ③'s high nibble — the split flag.
func TestWriteChannel_SplitONChannelIsRefusedNotCleared(t *testing.T) {
	rec := append([]byte(nil), populatedRecord...)
	rec[0] |= 0x10 // Split ON, as the radio would store it
	peer := newRespondingPort(t, withRecord(1, rec))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	_, err := sess.WriteChannel(context.Background(), channelFor("001"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want ErrWriteRefused — enablers E6: a slot may be written ONLY when its unmapped regions equal the profile's Fixed template, and writing this one would clear the user's split flag", err)
	}
	if !strings.Contains(err.Error(), "split") {
		t.Errorf("refusal %q does not name the split flag — E6 requires the reason to be named", err)
	}
	for _, fr := range peer.Received() {
		if cn, sc, ok := civ.FrameCommand(fr); ok && cn == 0x1A && sc == 0x00 && len(fr) > 9 {
			t.Errorf("a set frame reached the wire despite the E6 refusal: % X", fr)
		}
	}
}

// A Split-OFF channel writes normally: the refusal must not be a blanket one.
func TestWriteChannel_SplitOFFChannelWritesNormally(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	if _, err := sess.WriteChannel(context.Background(), channelFor("001")); err != nil {
		t.Fatalf("WriteChannel: %v — E6 refuses a NON-CONFORMING unmapped region, not every write", err)
	}
	sets := setFrames(peer)
	if len(sets) != 1 {
		t.Fatalf("the peer saw %d set frames, want 1", len(sets))
	}
	if got := recordOf(sets[0])[0] & 0xF0; got != 0x00 {
		t.Errorf("③'s high nibble goes out as %#02x, want 0x00 — Split OFF is what the Fixed template declares", got)
	}
}

// THE PAIRED PIN. Without this, a later edit that MAPS ③'s high nibble turns
// the E6 test above into a no-op: the check would compare a nibble the codec
// now owns, and a Split-ON record would sail through.
func TestSplitNibbleIsUnmapped(t *testing.T) {
	layouts := ic7300civ.Profile().Layouts()
	if len(layouts) != 1 {
		t.Fatalf("the profile declares %d layouts, want 1", len(layouts))
	}
	l := layouts[0]
	if len(l.Fixed) != 39 {
		t.Fatalf("Fixed is %d bytes, want a FULL-LENGTH template of 39 — civ's V8 permits an empty template or one of exactly Length bytes, and only the full-length one can declare the unmapped nibble", len(l.Fixed))
	}
	for i, b := range l.Fixed {
		if b != 0x00 {
			t.Errorf("Fixed[%d] = %#02x, want 0x00 — an all-zero template is the only shape that leaves every mapped nibble to its own span", i, b)
		}
	}
	for _, sp := range l.Fields {
		if sp.Offset != 0 {
			continue
		}
		if sp.Field != civ.FieldSelect || sp.Nibble != civ.NibbleLow {
			t.Errorf("record byte 0 carries span %+v — ③'s LOW nibble is SELECT and its HIGH nibble is the SPLIT flag, which must stay UNMAPPED so E6's check has something to compare", sp)
		}
	}
}

// The SELECT value the radio holds is carried through unchanged.
func TestWriteChannel_SelectNibbleRoundTripsUnchanged(t *testing.T) {
	for _, nibble := range []byte{0x00, 0x01, 0x02, 0x03} {
		rec := append([]byte(nil), populatedRecord...)
		rec[0] = nibble
		peer := newRespondingPort(t, withRecord(1, rec))
		sess := openSession(t, peer, WithConsentedUnverifiedWrites())
		if _, err := sess.WriteChannel(context.Background(), channelFor("001")); err != nil {
			t.Fatalf("③ = %#02x: WriteChannel: %v", nibble, err)
		}
		sets := setFrames(peer)
		if len(sets) != 1 {
			t.Fatalf("③ = %#02x: %d set frames, want 1", nibble, len(sets))
		}
		if got := recordOf(sets[0])[0]; got != nibble {
			t.Errorf("③ went out as %#02x, want %#02x — no spec.Field carries the SELECT group (D4), so a driver that did not carry it through would silently move the user's channel out of its scan group", got, nibble)
		}
	}
}

// D18: a CREATE has nothing to preserve and invents nothing — it refuses.
func TestWriteChannel_CreateRefusesRatherThanInventingTheSelectNibble(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []peerOption
	}{
		{"answers FA", nil},
		{"answers an all-FF record", []peerOption{withRecord(10, allFFRecord())}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := newRespondingPort(t, tc.opts...)
			sess := openSession(t, peer, WithConsentedUnverifiedWrites())
			_, err := sess.WriteChannel(context.Background(), channelFor("010"))
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel error = %v, want ErrWriteRefused — an empty slot has no SELECT value to preserve, and no spec.Field carries one (D4), so writing OFF would put the channel in a scan group the user never chose", err)
			}
			if !strings.Contains(err.Error(), "SELECT") {
				t.Errorf("refusal %q does not name the SELECT nibble", err)
			}
			if n := len(setFrames(peer)); n != 0 {
				t.Errorf("%d set frames reached the wire on a refused create", n)
			}
		})
	}
}

// D18/R6: a non-Known mandatory field is REFUSED, never synthesised. REV 1
// wrote FreqHz into the TX field when TxFreqHz was not Known; that
// manufactured a value from Absent/Unknown/Unavailable and could overwrite a
// split channel's distinct transmit frequency.
func TestWriteChannel_NonKnownTxFrequencyIsRefusedNotSubstituted(t *testing.T) {
	ch := channelFor("001")
	ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Unknown}
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want ErrWriteRefused", err)
	}
	if !strings.Contains(err.Error(), string(spec.FieldTxFrequency)) {
		t.Errorf("refusal %q does not name tx_frequency", err)
	}
	if n := len(setFrames(peer)); n != 0 {
		t.Errorf("%d set frames reached the wire", n)
	}
}

// Same rule, every other mandatory field.
//
// FIVE cases, not REV 2's seven: ruling T1(4) moved tone_tx and tone_rx OUT
// of the mandatory set, because a non-Known tone is PRESERVED from the
// just-read record rather than refused. The test below this one is their
// witness, and it is what replaces the two dropped rows.
func TestWriteChannel_NonKnownMandatoryFieldsAreRefused(t *testing.T) {
	for _, tc := range []struct {
		field  spec.Field
		break_ func(*codeplug.ChannelData)
		why    string
	}{
		{spec.FieldMode, func(d *codeplug.ChannelData) { d.Mode = "" }, "⑨ is a mandatory enum byte and an empty name is not one of this profile's values"},
		{spec.FieldMode, func(d *codeplug.ChannelData) { d.Mode = "FT8" }, "a mode this radio does not print is not a mode it can store"},
		{spec.FieldFilter, func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{State: codeplug.Unknown} }, "⑩ is mandatory and the record cannot omit it"},
		{spec.FieldDataMode, func(d *codeplug.ChannelData) { d.DataMode = codeplug.BoolField{State: codeplug.Unavailable} }, "⑪'s HIGH nibble is mandatory"},
		{spec.FieldToneMode, func(d *codeplug.ChannelData) { d.ToneMode = codeplug.StringField{State: codeplug.Unknown} }, "⑪'s LOW nibble is mandatory"},
		{spec.FieldTag, func(d *codeplug.ChannelData) { d.Tag = "ELEVEN CHAR" }, "⑱–㉗ is ten bytes, and truncating would write a name the caller did not choose"},
		{spec.FieldTag, func(d *codeplug.ChannelData) { d.Tag = "TAB\there" }, "a byte outside this radio's charset would be refused by the codec anyway, and is named here"},
	} {
		t.Run(string(tc.field)+"/"+tc.why, func(t *testing.T) {
			peer := newRespondingPort(t, withRecord(1, populatedRecord))
			sess := openSession(t, peer, WithConsentedUnverifiedWrites())
			ch := channelFor("001")
			tc.break_(ch.Data)
			before := len(peer.Received())
			_, err := sess.WriteChannel(context.Background(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel error = %v, want ErrWriteRefused (%s)", err, tc.why)
			}
			if !strings.Contains(err.Error(), string(tc.field)) {
				t.Errorf("refusal %q does not name %s", err, tc.field)
			}
			if after := len(peer.Received()); after != before {
				t.Errorf("the transcript grew by %d frames — a mandatory-field refusal is locally decidable and precedes the read", after-before)
			}
		})
	}
}

// T1(4): a non-Known tone is PRESERVED from the just-read record verbatim,
// not refused and not synthesised. The value is the one the radio itself
// holds, which is available because the preservation read is mandatory
// before any write anyway.
func TestWriteChannel_NonKnownTonesArePreservedFromTheRecord(t *testing.T) {
	ch := channelFor("001")
	ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel: %v — a non-Known tone is preserved, not refused: requesting the field would ask the gate about a value the caller never set", err)
	}
	sets := setFrames(peer)
	if len(sets) != 1 {
		t.Fatalf("%d set frames, want 1", len(sets))
	}
	rec := recordOf(sets[0])
	for _, span := range []struct {
		name string
		off  int
	}{{"⑫–⑭", 9}, {"⑮–⑰", 12}, {"⓬–⓮", 23}, {"⓯–⓱", 26}} {
		got := rec[span.off : span.off+3]
		want := populatedRecord[span.off : span.off+3]
		if string(got) != string(want) {
			t.Errorf("%s went out as % X, want % X — the preserved value is the one the RADIO holds, taken from the record just read", span.name, got, want)
		}
	}
}

// D15: the read and the set are one critical section.
func TestWriteChannel_ReadAndSetAreSerialised(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	openFrames := len(peer.Received())

	const writers = 4
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := sess.WriteChannel(context.Background(), channelFor("001")); err != nil {
				t.Errorf("WriteChannel: %v", err)
			}
		}()
	}
	wg.Wait()

	frames := peer.Received()[openFrames:]
	if len(frames) != 2*writers {
		t.Fatalf("the peer saw %d frames after open, want %d — one read and one set per write", len(frames), 2*writers)
	}
	for i := 0; i < len(frames); i += 2 {
		if len(frames[i]) != 9 {
			t.Fatalf("frame %d is % X, want a nine-byte READ — Engine.Do locks ONE exchange, not a read-modify-write SEQUENCE, so without the session's own mutex two writers could interleave their reads and each build against the other's record (D15)", i, frames[i])
		}
		if len(frames[i+1]) != 48 {
			t.Fatalf("frame %d is % X, want the 48-byte SET that belongs to the read before it", i+1, frames[i+1])
		}
	}
}

// The duplicated TX block always goes out, in full.
func TestWriteChannel_AlwaysSendsTheFullRecordIncludingTheTXBlock(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	if _, err := sess.WriteChannel(context.Background(), channelFor("001")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	rec := recordOf(setFrames(peer)[0])
	if len(rec) != 39 {
		t.Fatalf("the record is %d bytes, want the whole 39 — the driver has no way to send half a record, and D5 entry 4 records that the full form is required", len(rec))
	}
	// ❹–⑧ is the TRANSMIT frequency, a distinct field.
	if string(rec[15:20]) != string(populatedRecord[15:20]) {
		t.Errorf("❹–⑧ = % X, want % X", rec[15:20], populatedRecord[15:20])
	}
	// ❾–⓱ MIRROR ⑨–⑰: nine bytes with no neutral codeplug field of their
	// own, so the encoder writes the receive side's values into both copies.
	if string(rec[20:29]) != string(rec[6:15]) {
		t.Errorf("❾–⓱ = % X but ⑨–⑰ = % X — the nine duplicated bytes reuse their RX copies' field ids, so a record whose halves disagree could not be built at all", rec[20:29], rec[6:15])
	}
}

// Consent.
func TestWriteChannel_RefusesEverythingWithoutConsent(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord), withRecord(100, populatedRecord))
	sess := openSession(t, peer)
	for _, slot := range []string{"001", "P1"} {
		before := len(peer.Received())
		if _, err := sess.WriteChannel(context.Background(), channelFor(slot)); !errors.Is(err, driver.ErrWriteRefused) {
			t.Errorf("WriteChannel(%q) = %v, want ErrWriteRefused — writeTrialsComplete is FALSE, so a RealHardware session writes nothing without the user's recorded consent", slot, err)
		}
		if after := len(peer.Received()); after != before {
			t.Errorf("WriteChannel(%q) reached the wire on an unconsented session", slot)
		}
	}
}

func TestWriteChannel_ErasesNothingEvenWithConsent(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	if got := sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldErase); got.CanWrite() {
		t.Fatalf("MEM/erase = %+v after consent — spec.ConsentUnverifiedWrites excludes FieldErase STRUCTURALLY, so no profile's labels can mint a consented erase", got)
	}
	before := len(peer.Received())
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "001"})
	if !errors.Is(err, driver.ErrWriteRefused) || !strings.Contains(err.Error(), string(spec.FieldErase)) {
		t.Fatalf("WriteChannel(empty) = %v, want an ErrWriteRefused naming erase", err)
	}
	if after := len(peer.Received()); after != before {
		t.Error("the erase refusal reached the wire")
	}
}

// --- D17's set. ---

// D17's set, with the predicate each field's REAL representation permits.
// FieldErase is absent BY DESIGN: it has no ChannelData member (erasure is
// Channel.Data == nil) and is refused at ladder rung 3.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	want := []spec.Field{
		// Unconditional: the seven the 1A 00 record always carries.
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldTxFrequency, spec.FieldFilter, spec.FieldDataMode, spec.FieldToneMode,
		// Conditional, in ChannelData declaration order.
		spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift,
		spec.FieldTagDisplay, spec.FieldScanSkip,
		spec.FieldDuplex, spec.FieldOffset, spec.FieldToneTx, spec.FieldToneRx,
		spec.FieldDTCSCode, spec.FieldDTCSPolarity,
	}
	got := requestedFields(everyFieldSet())
	if !slices.Equal(got, want) {
		t.Errorf("requestedFields = %v,\nwant %v", got, want)
	}
	if len(want) != 19 {
		t.Errorf("requestedFields covers %d fields, want 19", len(want))
	}
	if len(allFields) != 20 || !slices.Contains(allFields, spec.FieldErase) {
		t.Fatalf("allFields = %v — the pin below assumes the twenty spec.Fields including erase", allFields)
	}
	// The twentieth is erase, and its absence here is the design.
	if slices.Contains(got, spec.FieldErase) {
		t.Error("requestedFields carries FieldErase — erase has no ChannelData member (Channel.Data == nil IS the erasure) and belongs to the empty-channel rung, not the field gate")
	}
	// AN ORDINARY CHANNEL — one this driver produced — requests the seven
	// unconditional fields plus whichever tones it actually carries, and
	// NOTHING ELSE. That is what makes an ordinary write possible at all:
	// every one of the ten fields this radio cannot express answers false.
	ordinary := *channelFor("001").Data
	if got, w := requestedFields(ordinary), append(append([]spec.Field(nil), want[:7]...), spec.FieldToneTx, spec.FieldToneRx); !slices.Equal(got, w) {
		t.Errorf("a channel this driver produced requests %v, want %v", got, w)
	}
	ordinary.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	ordinary.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
	if got := requestedFields(ordinary); !slices.Equal(got, want[:7]) {
		t.Errorf("a channel whose tones are not Known requests %v, want just the seven unconditional fields — a non-Known tone is PRESERVED from the record, so requesting it would ask the gate about a value the caller never set (T1(4))", got)
	}
}

// Ten table-driven cases: a direct Session.WriteChannel call REQUESTING one
// field this radio cannot express is refused BY NAME, before any wire
// traffic. The witness is a TRANSCRIPT DELTA of zero: scanning for set
// frames would have let a premature READ through.
func TestWriteChannel_RefusesEveryRequestedUnsupportedField(t *testing.T) {
	for _, f := range []spec.Field{
		spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift,
		spec.FieldTagDisplay, spec.FieldScanSkip,
		spec.FieldDuplex, spec.FieldOffset, spec.FieldDTCSCode, spec.FieldDTCSPolarity,
	} {
		t.Run(string(f), func(t *testing.T) {
			peer := newRespondingPort(t, withRecord(1, populatedRecord))
			sess := openSession(t, peer, WithConsentedUnverifiedWrites())
			before := len(peer.Received())
			_, err := sess.WriteChannel(context.Background(), channelRequesting(f))
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel error = %v, want ErrWriteRefused", err)
			}
			if !strings.Contains(err.Error(), string(f)) {
				t.Errorf("refusal %q does not name %s — a refusal that does not say which field is not one a caller can act on", err, f)
			}
			if after := len(peer.Received()); after != before {
				t.Errorf("the transcript grew by %d frames during a locally decidable refusal — D21 puts every such rung BEFORE the single read, so nothing may reach the wire", after-before)
			}
		})
	}
}

// The twentieth field, at its own rung: an empty channel is an erase request.
func TestWriteChannel_EmptyChannelIsRefusedAsErase(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	before := len(peer.Received())
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "001"})
	if !errors.Is(err, driver.ErrWriteRefused) || !strings.Contains(err.Error(), string(spec.FieldErase)) {
		t.Fatalf("WriteChannel(empty) = %v, want an ErrWriteRefused naming erase — this tier ships no erase path (spec D4, adjudication 19)", err)
	}
	if after := len(peer.Received()); after != before {
		t.Error("the erase refusal reached the wire — it is rung 3, before the read")
	}
}

// D20 ON THE WRITE PATH. The preservation read's answer must have its channel
// address checked BEFORE any other use of it, exactly as ReadChannel's is —
// D20 says the order applies "in ReadChannel and in the write path's
// preservation read alike", and calls a per-driver mismatch regression test
// mandatory. The read path had both witnesses and this one had neither.
//
// THE SECOND SUB-CASE IS THE ONE THAT PINS THE ORDER. With the all-FF branch
// ahead of the address check, an all-FF answer for the WRONG channel reports
// "this slot is empty", and the write becomes a CREATE refusal naming the
// SELECT nibble — a plausible-looking refusal about the wrong thing, on a
// slot that may well be populated. Asserting ErrAnswerMismatch rather than
// merely "some error" is what makes the sub-case discriminate.
func TestWriteChannel_PreservationReadAddressMismatchIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		rec  []byte
	}{
		{"a populated record for the wrong channel", populatedRecord},
		{"an all-FF record for the wrong channel", allFFRecord()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Channel 9 is past the open probe's bound, so the misdirection
			// is met for the first time HERE, on the write path.
			peer := newRespondingPort(t,
				withRecord(9, tc.rec),
				withAnswerAddressedElsewhere(9, 10),
			)
			sess := openSession(t, peer, WithConsentedUnverifiedWrites())
			_, err := sess.WriteChannel(context.Background(), channelFor("009"))
			if !errors.Is(err, ErrAnswerMismatch) {
				t.Fatalf("WriteChannel error = %v, want ErrAnswerMismatch — civ's MemoryAnswerMatcher is ENVELOPE-ONLY by design, so the channel address is the driver's to check, on this path as much as on the read path (T2, D20)", err)
			}
			if errors.Is(err, driver.ErrWriteRefused) {
				t.Errorf("the refusal is a *driver.WriteRefusedError (%v) — a misaddressed answer is not a fact about the requested slot, and reporting it as a create refusal would name the wrong problem on a slot that may be populated", err)
			}
			if n := len(setFrames(peer)); n != 0 {
				t.Errorf("%d set frames reached the wire after a mismatched preservation read", n)
			}
			if n := civDiagnostics(t, sess).AnswerMismatches; n != 1 {
				t.Errorf("AnswerMismatches = %d, want 1 — the refusal carries a diagnostic count beside it, on this path too", n)
			}
		})
	}
}

// A LATER quarantine drain failure is fail-closed on the write path too: the
// flood starts once the preservation read has gone out, so Do's post-write
// quarantine and the next exchange meet a line that never goes quiet.
func TestWriteChannel_LaterDrainFailureFailsClosed(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord), withNoAnswerToSets())
	sess := openSession(t, peer, WithConsentedUnverifiedWrites())
	if _, err := sess.WriteChannel(context.Background(), channelFor("001")); err == nil {
		t.Fatal("WriteChannel succeeded against a peer that never acknowledges a set")
	}
	// The engine is now SUSPECT: the acknowledgement never arrived, so the
	// next exchange must drain before it trusts anything as its own answer.
	go peer.flood(peerControllerAddr, 5*time.Millisecond)
	_, err := sess.WriteChannel(context.Background(), channelFor("001"))
	if err == nil {
		t.Fatal("a second write succeeded on a suspect engine over a line that never goes quiet — Do's entry quarantine exists to stop an abandoned exchange's reply being read as this one's answer")
	}
	if !errors.Is(err, transport.ErrQuarantineFailed) && !errors.Is(err, transport.ErrDrainCapExceeded) {
		t.Errorf("error = %v, want a quarantine/drain failure", err)
	}
}

// everyFieldSet is a ChannelData in which EVERY conditional predicate
// answers true, so requestedFields returns its whole membership. It is a
// fixture for the pin above and nothing else: no radio would ever produce
// one, which is exactly why it is written by hand here.
func everyFieldSet() codeplug.ChannelData {
	d := *channelFor("001").Data
	d.ClarHz = 100
	d.RxClar = true
	d.TxClar = true
	d.CTCSS = "OFF"
	d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)}
	d.Shift = "SIMPLEX"
	d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
	d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
	d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "OFF"}
	d.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 600_000}
	d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}
	d.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: "NN"}
	return d
}

// channelRequesting returns an otherwise ordinary channel for slot 001 that
// REQUESTS exactly one extra field: the ten-case table's input.
func channelRequesting(f spec.Field) codeplug.Channel {
	ch := channelFor("001")
	d := ch.Data
	switch f {
	case spec.FieldClarifier:
		d.ClarHz = 100
	case spec.FieldCTCSSState:
		d.CTCSS = "OFF"
	case spec.FieldCTCSSTone:
		d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)}
	case spec.FieldShift:
		d.Shift = "SIMPLEX"
	case spec.FieldTagDisplay:
		d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
	case spec.FieldScanSkip:
		d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
	case spec.FieldDuplex:
		d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "OFF"}
	case spec.FieldOffset:
		d.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 600_000}
	case spec.FieldDTCSCode:
		d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}
	case spec.FieldDTCSPolarity:
		d.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: "NN"}
	default:
		panic("channelRequesting: " + string(f) + " is not one of the fields this radio cannot express")
	}
	return ch
}

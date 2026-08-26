// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// allFFRecord is 39 bytes of 0xFF: the second of D5 entry 2's two
// unverified empty-channel answers.
func allFFRecord() []byte {
	rec := make([]byte, 39)
	for i := range rec {
		rec[i] = 0xFF
	}
	return rec
}

// A populated MEM slot maps every record field into ChannelData.
func TestReadChannel_PopulatedMemorySlot(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer)
	ch, err := sess.ReadChannel(context.Background(), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Slot != "001" {
		t.Errorf("Slot = %q, want %q", ch.Slot, "001")
	}
	if ch.Empty() {
		t.Fatal("channel is EMPTY after reading a populated record")
	}
	d := ch.Data
	if d.FreqHz != 14_250_000 {
		t.Errorf("FreqHz = %d, want 14250000 — ④–⑧, five packed-BCD bytes, least significant pair first", d.FreqHz)
	}
	if d.Mode != "USB" {
		t.Errorf("Mode = %q, want %q", d.Mode, "USB")
	}
	if d.Tag != "TEST CHAN1" {
		t.Errorf("Tag = %q, want %q — the name is trimmed of its 0x20 pad, and a space INSIDE it survives", d.Tag, "TEST CHAN1")
	}
	if d.TxFreqHz != (codeplug.FreqField{State: codeplug.Known, Value: 14_250_000}) {
		t.Errorf("TxFreqHz = %+v, want Known 14250000 — ❹–⑧ is a DISTINCT field, so a split channel round trips", d.TxFreqHz)
	}
	if d.Filter != (codeplug.StringField{State: codeplug.Known, Value: "FIL1"}) {
		t.Errorf("Filter = %+v, want Known FIL1", d.Filter)
	}
	if d.DataMode != (codeplug.BoolField{State: codeplug.Known, Value: false}) {
		t.Errorf("DataMode = %+v, want Known false — ⑪'s HIGH nibble, 0 = OFF", d.DataMode)
	}
	if d.ToneMode != (codeplug.StringField{State: codeplug.Known, Value: "OFF"}) {
		t.Errorf("ToneMode = %+v, want Known OFF — ⑪'s LOW nibble", d.ToneMode)
	}
	if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)}) {
		t.Errorf("ToneTx = %+v, want Known 88.5 Hz — ⑫–⑭, BCD TENTHS of a hertz", d.ToneTx)
	}
	if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)}) {
		t.Errorf("ToneRx = %+v, want Known 88.5 Hz — ⑮–⑰", d.ToneRx)
	}
	// A Known tone this driver produced must be one the capabilities admit,
	// or the very next thing to touch this channel refuses it.
	if err := d.ToneTx.Valid(sess.Capabilities()); err != nil {
		t.Errorf("ToneTx.Valid: %v — a read must never construct a Known value the radio's own tone domain refuses (T1(3))", err)
	}
}

// An FA answer is an EMPTY channel, not an error (D5 entry 2(a)).
func TestReadChannel_RejectionIsAnEmptyChannel(t *testing.T) {
	peer := newRespondingPort(t)
	sess := openSession(t, peer)
	ch, err := sess.ReadChannel(context.Background(), "042")
	if err != nil {
		t.Fatalf("ReadChannel: %v — an FA is an unwritten channel, and an error here would abort the whole ReadAll", err)
	}
	if !ch.Empty() {
		t.Error("channel is populated after an FA answer")
	}
	if ch.Slot != "042" {
		t.Errorf("Slot = %q, want %q — an empty channel still names its slot", ch.Slot, "042")
	}
}

// R10 — an all-FF record is an EMPTY channel too, recognised BEFORE the
// record parser. Without the pre-parse hook it would reach
// ParseMemoryAnswer and die on its first BCD nibble or its first unknown
// enum value — a failure INDISTINGUISHABLE from a corrupted record.
func TestReadChannel_AllFFRecordIsAnEmptyChannel(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, allFFRecord()))
	sess := openSession(t, peer)
	ch, err := sess.ReadChannel(context.Background(), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v — an all-FF record is D5 entry 2(b)'s empty channel (ic7300-ff-record), never an error that aborts ReadAll", err)
	}
	if !ch.Empty() {
		t.Error("channel is populated after an all-FF record")
	}
}

// ...and the recognition is EXACT: a record that is all-FF except one byte
// is NOT empty, and must not be quietly treated as such.
func TestReadChannel_NearlyAllFFRecordIsNotEmpty(t *testing.T) {
	rec := allFFRecord()
	rec[6] = 0x01 // the mode byte, a legal USB
	peer := newRespondingPort(t, withRecord(1, rec))
	sess := openSession(t, peer)
	ch, err := sess.ReadChannel(context.Background(), "001")
	if err == nil {
		t.Fatalf("ReadChannel succeeded (empty = %v) on a record that is all-FF except one byte — the empty test is EXACT equality, not a heuristic, and a nearly-all-FF record is a CORRUPT record", ch.Empty())
	}
	if !errors.Is(err, civ.ErrParse) {
		t.Errorf("error = %v, want a *civ.ParseError — the record reached the parser, which is where a corrupt record belongs", err)
	}
}

// A record of the wrong LENGTH fails the read with *civ.RecordLengthError —
// the fingerprint is continuous, not one-shot (spec D3.2, D4 "Malformed
// records": no partial parse, no fake Unavailable channel).
func TestReadChannel_WrongLengthFailsTheRead(t *testing.T) {
	// Channel 9 is past the open probe's bound, so the session opens
	// UNFINGERPRINTED and the length is met for the first time HERE — which
	// is the point: the check is re-asked on every record read.
	peer := newRespondingPort(t, withRecordOfLength(9, 45))
	sess := openSession(t, peer)
	if civDiagnostics(t, sess).Fingerprinted {
		t.Fatal("the session opened FINGERPRINTED — this test needs the length to be met for the first time at the read")
	}
	ch, err := sess.ReadChannel(context.Background(), "009")
	if err == nil {
		t.Fatalf("ReadChannel succeeded (empty = %v) on a 45-byte record — the length fingerprint is CONTINUOUS: a wrong-model session cannot be opened once and then trusted", ch.Empty())
	}
	if !errors.Is(err, civ.ErrRecordLength) {
		t.Errorf("error = %v, want it to wrap civ.ErrRecordLength", err)
	}
	if !ch.Empty() || ch.Data != nil {
		t.Error("a failed read returned channel data — there is no partial parse and no fake Unavailable channel")
	}
}

// Mode code 06 is printed nowhere and is invented nowhere: the read fails
// with a *civ.ParseError naming the byte and offset (plan decision D12).
func TestReadChannel_UnprintedModeCodeFailsHonestly(t *testing.T) {
	rec := append([]byte(nil), populatedRecord...)
	rec[6] = 0x06  // ⑨
	rec[20] = 0x06 // ❾, so the duplicated spans still agree
	peer := newRespondingPort(t, withRecord(1, rec))
	sess := openSession(t, peer)
	_, err := sess.ReadChannel(context.Background(), "001")
	if err == nil {
		t.Fatal("ReadChannel succeeded on mode code 06 — that value is printed NOWHERE in this document (matrix §3.16 A7) and no meaning is invented for it")
	}
	if !errors.Is(err, civ.ErrParse) {
		t.Errorf("error = %v, want a *civ.ParseError: civ's decodeRecord has no \"unknown enum\" representation, so the read FAILS and ReadAll aborts honestly", err)
	}
}

// A record whose TX block disagrees with its RX block fails the WHOLE read,
// rather than one copy silently winning. That cost travels to the user.
func TestReadChannel_DisagreeingTXBlockFailsTheRead(t *testing.T) {
	rec := append([]byte(nil), populatedRecord...)
	rec[20] = 0x02 // ❾ says AM where ⑨ says USB
	peer := newRespondingPort(t, withRecord(1, rec))
	sess := openSession(t, peer)
	_, err := sess.ReadChannel(context.Background(), "001")
	if err == nil {
		t.Fatal("ReadChannel succeeded on a record whose TX mode differs from its RX mode — the duplicated spans are checked for AGREEMENT, and letting the last copy win would silently lose half the record")
	}
	if !errors.Is(err, civ.ErrParse) {
		t.Errorf("error = %v, want a *civ.ParseError", err)
	}
}

// P1 and P2 read through the same record shape (ASSUMED —
// ic7300-scan-edge-record-shape).
func TestReadChannel_ScanEdges(t *testing.T) {
	peer := newRespondingPort(t,
		withRecord(100, populatedRecord),
		withRecord(101, populatedRecord),
	)
	sess := openSession(t, peer)
	for _, slot := range []string{"P1", "P2"} {
		ch, err := sess.ReadChannel(context.Background(), slot)
		if err != nil {
			t.Fatalf("ReadChannel(%q): %v", slot, err)
		}
		if ch.Empty() || ch.Data.FreqHz != 14_250_000 {
			t.Errorf("ReadChannel(%q) = %+v, want the same 39-byte record shape a memory channel has (ASSUMED, ic7300-scan-edge-record-shape)", slot, ch)
		}
	}
	// The WIRE forms, which is what D11's table fixes: 01 00 and 01 01.
	var reads [][]byte
	for _, f := range peer.Received() {
		if cn, sc, ok := civ.FrameCommand(f); ok && cn == 0x1A && sc == 0x00 && len(f) == 9 {
			reads = append(reads, f)
		}
	}
	last := reads[len(reads)-2:]
	if last[0][6] != 0x01 || last[0][7] != 0x00 {
		t.Errorf("P1 was read as % X, want channel bytes 01 00 (D11)", last[0])
	}
	if last[1][6] != 0x01 || last[1][7] != 0x01 {
		t.Errorf("P2 was read as % X, want channel bytes 01 01 (D11)", last[1])
	}
}

// Fields this record does not carry read back Unavailable, not Unknown, and
// not a guess.
func TestReadChannel_AbsentFieldsAreUnavailable(t *testing.T) {
	peer := newRespondingPort(t, withRecord(1, populatedRecord))
	sess := openSession(t, peer)
	ch, err := sess.ReadChannel(context.Background(), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	d := ch.Data
	// The three Yaesu SCALARS have no FieldState to carry Unavailable, so
	// the honest reading is their zero value: nothing was set, because this
	// record has nothing to set it from.
	if d.ClarHz != 0 || d.RxClar || d.TxClar {
		t.Errorf("clarifier = %d/%v/%v, want all zero — the 1A 00 record has no clarifier field (matrix §1 rows 6, 7)", d.ClarHz, d.RxClar, d.TxClar)
	}
	if d.CTCSS != "" {
		t.Errorf("CTCSS = %q, want empty — ctcss_state is displaced by tone_mode on Icom models (spec D4)", d.CTCSS)
	}
	if d.Shift != "" {
		t.Errorf("Shift = %q, want empty — no shift or duplex field exists on this model (matrix §1 row 14)", d.Shift)
	}
	for _, tc := range []struct {
		name  string
		state codeplug.FieldState
	}{
		{"CTCSSTone", d.CTCSSTone.State},
		{"TagDisplay", d.TagDisplay.State},
		{"ScanSkip", d.ScanSkip.State},
		{"Duplex", d.Duplex.State},
		{"OffsetHz", d.OffsetHz.State},
		{"DTCSCode", d.DTCSCode.State},
		{"DTCSPolarity", d.DTCSPolarity.State},
	} {
		if tc.state != codeplug.Unavailable {
			t.Errorf("%s.State = %q, want Unavailable — this radio/protocol has NO such field, which is a positive statement and not the same as Unknown (\"not read yet\")", tc.name, tc.state)
		}
	}
}

// T1(3): a tone number OUTSIDE the declared domain — zero included — maps to
// Unknown. A read must never construct a Known value ToneField.Valid would
// then refuse.
func TestReadChannel_ZeroToneReadsBackUnknown(t *testing.T) {
	rec := append([]byte(nil), populatedRecord...)
	for _, off := range []int{9, 10, 11, 12, 13, 14, 23, 24, 25, 26, 27, 28} {
		rec[off] = 0x00
	}
	peer := newRespondingPort(t, withRecord(1, rec))
	sess := openSession(t, peer)
	ch, err := sess.ReadChannel(context.Background(), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v — the civ layer is LOSSLESS over the whole encodable range, zero included (T1(1)); the DOMAIN is a capability, and a value outside it is not a parse failure", err)
	}
	want := codeplug.ToneField{State: codeplug.Unknown}
	if ch.Data.ToneTx != want {
		t.Errorf("ToneTx = %+v, want %+v — 0 Hz is not a tone, the declared domain starts at 1 deciHz, and a Known zero would be refused by the very next validator (ic7300-zero-tone-means-unset)", ch.Data.ToneTx, want)
	}
	if ch.Data.ToneRx != want {
		t.Errorf("ToneRx = %+v, want %+v", ch.Data.ToneRx, want)
	}
}

// The answer's echoed channel address must match the one requested, and the
// check precedes EVERY use of the answer.
func TestReadChannel_AnswerAddressMismatchIsAnError(t *testing.T) {
	peer := newRespondingPort(t,
		withRecord(9, populatedRecord),
		withAnswerAddressedElsewhere(9, 10),
	)
	sess := openSession(t, peer)
	ch, err := sess.ReadChannel(context.Background(), "009")
	if !errors.Is(err, ErrAnswerMismatch) {
		t.Fatalf("ReadChannel error = %v, want ErrAnswerMismatch — civ's MemoryAnswerMatcher is ENVELOPE-ONLY by design, so the channel address is the driver's to check (T2, D20)", err)
	}
	if !ch.Empty() {
		t.Error("a mismatched answer produced channel data — refusing to map a reply onto the wrong slot is the whole point")
	}
	if n := civDiagnostics(t, sess).AnswerMismatches; n != 1 {
		t.Errorf("AnswerMismatches = %d, want 1 — the refusal carries a diagnostic count beside it", n)
	}
}

// THE CASE AN UNORDERED IMPLEMENTATION ANSWERS WRONGLY AND SILENTLY: an
// all-FF record for the WRONG address. With the all-FF branch first this
// reads as "this slot is empty"; with the address check first it is the
// mismatch it actually is.
func TestReadChannel_AllFFForTheWrongAddressIsAMismatchNotAnEmptySlot(t *testing.T) {
	peer := newRespondingPort(t,
		withRecord(9, allFFRecord()),
		withAnswerAddressedElsewhere(9, 10),
	)
	sess := openSession(t, peer)
	ch, err := sess.ReadChannel(context.Background(), "009")
	if err == nil {
		t.Fatalf("ReadChannel succeeded (empty = %v) — an all-FF answer for the WRONG channel is not evidence that the REQUESTED channel is empty, and accepting it would silently blank a populated slot in a codeplug", ch.Empty())
	}
	if !errors.Is(err, ErrAnswerMismatch) {
		t.Errorf("error = %v, want ErrAnswerMismatch: D20 puts the address check BEFORE the all-FF branch for exactly this case", err)
	}
}

// A slot this radio does not have is refused before any wire traffic.
func TestReadChannel_RefusesASlotThisRadioDoesNotHave(t *testing.T) {
	peer := newRespondingPort(t)
	sess := openSession(t, peer)
	for _, slot := range []string{"", "1", "0001", "100", "000", "P3", "M-01", "EMG"} {
		before := len(peer.Received())
		if _, err := sess.ReadChannel(context.Background(), slot); err == nil {
			t.Errorf("ReadChannel(%q) succeeded — this radio's slots are 001..099, P1 and P2, in exactly those spellings", slot)
		}
		if after := len(peer.Received()); after != before {
			t.Errorf("ReadChannel(%q) reached the wire before deciding the slot was unknown", slot)
		}
	}
}

// parseSlot is D11's table in both directions, and Task 16 reuses it.
func TestParseSlot_D11Table(t *testing.T) {
	for _, tc := range []struct {
		slot    string
		channel int
		bank    spec.BankID
	}{
		{"001", 1, spec.BankMemory},
		{"099", 99, spec.BankMemory},
		{"P1", 100, spec.BankScan},
		{"P2", 101, spec.BankScan},
	} {
		addr, bank, ok := parseSlot(tc.slot)
		if !ok {
			t.Errorf("parseSlot(%q) = not ok", tc.slot)
			continue
		}
		if addr.Channel != tc.channel || addr.Group != 0 {
			t.Errorf("parseSlot(%q) = %v, want group 0 channel %d", tc.slot, addr, tc.channel)
		}
		if bank != tc.bank {
			t.Errorf("parseSlot(%q) bank = %q, want %q", tc.slot, bank, tc.bank)
		}
	}
}

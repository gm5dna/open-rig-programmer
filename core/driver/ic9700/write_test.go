// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic9700 "github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// requestedFields is the package's own derivation, reached through the
// test-only alias in export_test.go rather than duplicated here.
var requestedFields = ic9700.RequestedFieldsForTest

// consentedSession opens a RealHardware session whose driver carries the
// user's recorded consent, then arms the image's faults and forgets the
// probe's traffic.
//
// CONSENT IS WHAT MAKES A WRITE REACHABLE AT ALL on this radio today:
// writeTrialsComplete is false, so every write column is Unverified and
// CanWrite is false without it. A write test on an unconsented session
// tests the capability gate and nothing past it.
func consentedSession(t *testing.T, opts ...imageOption) (*ic9700.Session, *recordingPort) {
	t.Helper()
	return openSessionWith(t, ic9700.New(ic9700.RealHardware, ic9700.WithConsentedUnverifiedWrites()), opts...)
}

// unconsentedSession opens a plain RealHardware session: nothing writable.
func unconsentedSession(t *testing.T, opts ...imageOption) (*ic9700.Session, *recordingPort) {
	t.Helper()
	if len(opts) == 0 {
		opts = []imageOption{withTemplateStateAt("144-001")}
	}
	return openSessionWith(t, ic9700.New(ic9700.RealHardware), opts...)
}

func openSessionWith(t *testing.T, d driver.Driver, opts ...imageOption) (*ic9700.Session, *recordingPort) {
	t.Helper()
	port := newRecordingPort(t, baseImage(opts...))
	sess, err := d.Open(context.Background(), port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	port.arm()
	port.clearTranscript()
	return sess.(*ic9700.Session), port
}

// withNoAcknowledgement makes every memory set draw silence — the case
// ClassWriteWithAck must report as a failure without ever resending.
func withNoAcknowledgement() imageOption {
	return func(img *radioImage) { img.silentSets = true }
}

// withRejection makes every memory set draw FA.
func withRejection() imageOption {
	return func(img *radioImage) { img.rejectSets = true }
}

// withStoredCallSign stores a channel whose UR call sign is not the
// template's — a D-STAR channel carrying bytes this tier cannot name.
func withStoredCallSign(slot, call string) imageOption {
	return func(img *radioImage) {
		img.records[mustAddress(slot)] = recordWithCallSignRaw(slot, call)
	}
}

// withStoredSelect stores a channel in a SELECT-memory star group.
func withStoredSelect(slot string, nibble byte) imageOption {
	return func(img *radioImage) {
		rec := templateRecord(mustAddress(slot))
		rec[offSelect] = rec[offSelect]&0xF0 | nibble&0x0F
		img.records[mustAddress(slot)] = rec
	}
}

// withStoredDuplexNibble stores a channel whose ⑬ high nibble carries the
// given value — 3 being RPS, which caps.DuplexOptions cannot name.
func withStoredDuplexNibble(slot string, nibble byte) imageOption {
	return func(img *radioImage) {
		rec := templateRecord(mustAddress(slot))
		for _, off := range []int{offDuplexTone, offDuplexTone + dupShift} {
			rec[off] = rec[off]&0x0F | nibble<<4
		}
		img.records[mustAddress(slot)] = rec
	}
}

// withStoredToneDeciHz stores a channel whose tone spans carry deci
// tenths of a hertz, in both copies.
func withStoredToneDeciHz(slot string, deci uint64) imageOption {
	return func(img *radioImage) {
		rec := templateRecord(mustAddress(slot))
		b := bcdBE(deci, 3)
		copy(rec[offToneTX:], b)
		copy(rec[offToneTX+dupShift:], b)
		img.records[mustAddress(slot)] = rec
	}
}

// recordWithCallSignRaw is recordWithCallSign without a *testing.T, for
// the imageOption closures that have none in scope.
func recordWithCallSignRaw(slot, call string) []byte {
	rec := templateRecord(mustAddress(slot))
	copy(rec[offURCallSign:], call)
	copy(rec[offURCallSign+dupShift:], call)
	return rec
}

// emptyChannelData is the ZERO ChannelData: nothing known about anything.
func emptyChannelData() codeplug.ChannelData { return codeplug.ChannelData{} }

// fullyKnownChannelData carries a Known value for EVERY conditional field,
// including the six this radio does not support.
//
// It exists to exercise requestedFields' membership and order, and it is
// deliberately NOT a channel that could be written: a Known value for a
// field this radio has no bytes for is exactly what the capability gate
// must REFUSE by name rather than silently drop.
func fullyKnownChannelData() codeplug.ChannelData {
	return codeplug.ChannelData{
		FreqHz:     145_500_000,
		Mode:       "FM",
		Tag:        "TEST",
		ClarHz:     100,
		RxClar:     true,
		CTCSS:      "OFF",
		Shift:      "SIMPLEX",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
		TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
		ScanSkip:   codeplug.BoolField{State: codeplug.Known, Value: true},

		TxFreqHz:     codeplug.FreqField{State: codeplug.Known, Value: 145_500_000},
		Duplex:       codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		OffsetHz:     codeplug.FreqField{State: codeplug.Known, Value: 600_000},
		ToneMode:     codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:       codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
		ToneRx:       codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
		DTCSCode:     codeplug.IntField{State: codeplug.Known, Value: 23},
		DTCSPolarity: codeplug.StringField{State: codeplug.Known, Value: "NN"},
		Filter:       codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
		DataMode:     codeplug.BoolField{State: codeplug.Known, Value: false},
	}
}

// writableChannelData is fullyKnownChannelData with the six unsupported
// fields left absent: every field the RECORD maps, and nothing else. It is
// what a CREATE needs, and what an ordinary write looks like.
func writableChannelData() codeplug.ChannelData {
	d := fullyKnownChannelData()
	d.ClarHz, d.RxClar, d.TxClar = 0, false, false
	d.CTCSS, d.Shift = "", ""
	d.CTCSSTone = codeplug.ToneField{State: codeplug.Unavailable}
	d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable}
	d.ScanSkip = codeplug.BoolField{State: codeplug.Unavailable}
	return d
}

// readBackChannelData is a ChannelData exactly as ReadChannel produces
// one, read through a real session against the golden record.
func readBackChannelData(t *testing.T) codeplug.ChannelData {
	t.Helper()
	return *mustRead(t, "144-001", withTemplateStateAt("144-001")).Data
}

// channelWithKnown is a writable channel at 144-001 carrying a Known value
// for f — one of the six fields this radio does not support.
func channelWithKnown(f spec.Field) codeplug.Channel {
	d := writableChannelData()
	switch f {
	case spec.FieldClarifier:
		d.ClarHz, d.RxClar = 500, true
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
	case spec.FieldToneTx:
		d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)}
	}
	return codeplug.Channel{Slot: "144-001", Data: &d}
}

// channelAt is a pure MODIFY: the frequency, mode and tag a write always
// carries, and nothing else Known, so every other field is preserved from
// the read.
func channelAt(slot string) codeplug.Channel {
	d := codeplug.ChannelData{FreqHz: 145_500_000, Mode: "FM", Tag: "TEST"}
	d.CTCSSTone = codeplug.ToneField{State: codeplug.Unavailable}
	d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable}
	d.ScanSkip = codeplug.BoolField{State: codeplug.Unavailable}
	return codeplug.Channel{Slot: slot, Data: &d}
}

// channelWith puts data at slot.
func channelWith(slot string, data codeplug.ChannelData) codeplug.Channel {
	d := data
	return codeplug.Channel{Slot: slot, Data: &d}
}

// withMode is a MODIFY carrying a Known mode and nothing else.
func withMode(t *testing.T, mode string) codeplug.ChannelData {
	t.Helper()
	d := *channelAt("144-001").Data
	d.Mode = mode
	return d
}

// withModeAndDuplex is withMode plus a Known duplex.
func withModeAndDuplex(t *testing.T, mode, duplex string) codeplug.ChannelData {
	t.Helper()
	d := withMode(t, mode)
	d.Duplex = codeplug.StringField{State: codeplug.Known, Value: duplex}
	return d
}

// withToneDeciHz is a MODIFY carrying a Known transmit tone.
func withToneDeciHz(t *testing.T, deci int) codeplug.ChannelData {
	t.Helper()
	d := *channelAt("144-001").Data
	d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(deci)}
	return d
}

// withDTCSCode is a MODIFY carrying a Known DTCS code.
func withDTCSCode(t *testing.T, code int) codeplug.ChannelData {
	t.Helper()
	d := *channelAt("144-001").Data
	d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: code}
	return d
}

// channelWithUnknownTonesAt is a MODIFY whose tone spans are NOT Known, so
// the write must carry the just-read record's own numbers.
func channelWithUnknownTonesAt(slot string) codeplug.Channel {
	ch := channelAt(slot)
	ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Unknown}
	return ch
}

// createWithToneModeOffButNoTones is a CREATE with every mandatory field
// Known EXCEPT the two tone spans, and ToneMode Known OFF.
//
// T1(5) would let a CREATE write a manual-DOCUMENTED default tone in
// exactly this case. This manual documents none, so the refusal stands.
func createWithToneModeOffButNoTones(t *testing.T, slot string) codeplug.Channel {
	t.Helper()
	d := writableChannelData()
	d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "OFF"}
	d.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	d.ToneRx = codeplug.ToneField{State: codeplug.Unknown}
	return channelWith(slot, d)
}

// channelMissing is a CREATE at 144-009 with one mandatory field left
// non-Known.
func channelMissing(t *testing.T, f spec.Field) codeplug.Channel {
	t.Helper()
	d := writableChannelData()
	switch f {
	case spec.FieldToneTx:
		d.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	default:
		t.Fatalf("channelMissing has no arm for %s", f)
	}
	return channelWith("144-009", d)
}

// fullyKnownChannelAt is a CREATE with every field the record maps Known.
func fullyKnownChannelAt(slot string) codeplug.Channel {
	return channelWith(slot, writableChannelData())
}

// toneDeciHzOf reads ⑮~⑰ back out of a memory-set frame.
//
// The frame's record begins at byte 9 — FE FE, two addresses, 1A 00, and
// the three-byte channel address — and the tone occupies record offsets
// 11..13 as three packed-BCD bytes, most significant pair first.
func toneDeciHzOf(frame []byte) uint64 {
	const recordStart = 9
	b := frame[recordStart+offToneTX : recordStart+offToneTX+3]
	var v uint64
	for _, x := range b {
		v = v*100 + uint64(x>>4)*10 + uint64(x&0x0F)
	}
	return v
}

func TestRequestedFields_UnconditionalSetComesFromThisModelsMatrix(t *testing.T) {
	// R6-COMPLETION, and the defect it fixes. The YAESU unconditional six
	// — frequency, mode, CLARIFIER, CTCSS_STATE, SHIFT, tag — would be
	// wrong here: on this radio the middle three are Unsupported on every
	// bank, and the very next ladder rung capability-checks every
	// requested field, so copying them would REFUSE EVERY IC-9700 WRITE.
	// The unconditional set is derived from THIS model's §2 grid: the
	// three fields graded Unverified on MEM.
	got := requestedFields(emptyChannelData())
	want := []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldTag}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unconditional requestedFields =\n%v\nwant\n%v", got, want)
	}
}

func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	// Every field a channel carries a KNOWN value for is requested — the
	// Wave-1 C2 contract ("silently dropping a value the caller
	// explicitly marked Known would be a lie") — including the six
	// legacy fields this radio does not support, so that a Known one is
	// REFUSED by the capability gate rather than dropped.
	got := requestedFields(fullyKnownChannelData())
	want := []spec.Field{
		// unconditional (matrix §2: Unverified on MEM)
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		// pre-tier conditionals — Unsupported here, requested when Known
		spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldCTCSSTone,
		spec.FieldShift, spec.FieldTagDisplay, spec.FieldScanSkip,
		// the ten tier conditionals, ChannelData declaration order
		spec.FieldTxFrequency, spec.FieldDuplex, spec.FieldOffset,
		spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
		spec.FieldDTCSCode, spec.FieldDTCSPolarity, spec.FieldFilter,
		spec.FieldDataMode,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requestedFields =\n%v\nwant\n%v", got, want)
	}
}

func TestEveryUnsupportedFieldIsRefusedWhenKnown(t *testing.T) {
	// The per-field direct-session refusal tests R6-COMPLETION requires:
	// one per field this model does not support, each refused by NAME.
	for _, f := range []spec.Field{
		spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldCTCSSTone,
		spec.FieldShift, spec.FieldTagDisplay, spec.FieldScanSkip,
	} {
		t.Run(string(f), func(t *testing.T) {
			sess, port := consentedSession(t, withTemplateStateAt("144-001"))
			_, err := sess.WriteChannel(context.Background(), channelWithKnown(f))
			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want *driver.WriteRefusedError", err)
			}
			if !strings.Contains(refused.Error(), string(f)) {
				t.Errorf("refusal %q does not name %s", refused.Error(), f)
			}
			if port.countSets() != 0 {
				t.Error("a set frame was sent before refusing")
			}
			if port.countReads() != 0 {
				t.Error("a locally decidable refusal read the radio first (T5)")
			}
		})
	}
}

func TestOrdinaryWriteRequestsNoUnsupportedField(t *testing.T) {
	// The other half: a channel this driver itself produced (every
	// unsupported field Unavailable) requests none of them, so the
	// ordinary write is not blocked by the defence-in-depth above.
	data := readBackChannelData(t) // as ReadChannel produces it
	for _, f := range requestedFields(data) {
		if fs := ic9700.CapabilitiesUnverified().FieldSupport(spec.BankMemory, f); fs.Unreachable() {
			t.Errorf("an ordinary write requests %s, which is Unsupported", f)
		}
	}
}

func TestWriteChannelRefusesAKnownFieldTheSessionCannotWrite(t *testing.T) {
	// One Known tier field on an unconsented session, refused before any
	// wire traffic.
	sess, port := unconsentedSession(t)
	_, err := sess.WriteChannel(context.Background(), channelWithKnown(spec.FieldToneTx))
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want *driver.WriteRefusedError", err)
	}
	if got := port.frames(); len(got) != 0 {
		t.Errorf("the driver wrote % X before refusing", got)
	}
}

func TestWriteChannelRefusesAChannelCarryingBytesThisTierCannotName(t *testing.T) {
	// register entry ic9700-unmapped-regions-refused. A channel whose UR
	// call sign differs from the template must not be silently blanked.
	sess, port := consentedSession(t, withStoredCallSign("144-001", "GB3CFR  "))
	_, err := sess.WriteChannel(context.Background(), channelAt("144-001"))
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a refusal naming the DV call sign", err)
	}
	if !strings.Contains(refused.Error(), "call sign") {
		t.Errorf("refusal %q does not name the region", refused.Error())
	}
	if port.countSets() != 0 {
		t.Error("a set frame was sent before refusing")
	}
	// The refusal FOLLOWS the single read, which is T5's one recorded
	// exception: a driver cannot know a slot's unmapped bytes without
	// reading it.
	if port.countReads() != 1 {
		t.Errorf("the read-dependent refusal drew %d reads, want exactly 1", port.countReads())
	}
}

func TestWriteChannelRefusesAStarGroupChannel(t *testing.T) {
	// OQ-4 under R6/E6: ④ is four-valued, the neutral field is boolean,
	// and preservation-by-cache is forbidden — so a star-group channel is
	// REFUSED, never flattened and never rewritten.
	sess, port := consentedSession(t, withStoredSelect("144-001", 2)) // ★2
	_, err := sess.WriteChannel(context.Background(), channelAt("144-001"))
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a refusal naming the select-memory group", err)
	}
	if !strings.Contains(refused.Error(), "SEL2") {
		t.Errorf("refusal %q does not name the group the channel is in", refused.Error())
	}
	if port.countSets() != 0 {
		t.Error("the driver sent a set frame before refusing")
	}
}

func TestWriteChannelRefusesAStoredRPSChannel(t *testing.T) {
	// OQ-6 under R6/E6, the READ-dependent half: the stored value cannot
	// be named by caps.DuplexOptions, so the channel is refused rather
	// than rewritten as OFF.
	//
	// THE BAND-3 CASE IS WHAT MAKES THE E6 CLAUSE CARRY ITS OWN WEIGHT.
	// Over 144-001 with an incoming mode of FM the stored RPS is
	// preserved onto that mode and the POST-MERGE cross-field check
	// refuses it instead — "RPS can be set only when DD mode is
	// selected" — which contains the same "RPS" a laxer assertion greps
	// for, so disabling the template guard's clause left the suite green.
	// At 1200-001 with mode DD the cross-field rule is satisfied, so the
	// guard's own clause is the only thing that can refuse, and its
	// wording is asserted rather than the bare substring.
	for _, tc := range []struct {
		name, slot string
		freqHz     uint64
		mode       string
		wantText   string
	}{
		{
			name: "in band 3 with DD, where only the template guard can refuse",
			slot: "1200-001", freqHz: 1_240_000_000, mode: "DD",
			wantText: "the stored channel is set to RPS",
		},
		{
			name: "in band 1, where the merged record also violates the manual",
			slot: "144-001", freqHz: 145_500_000, mode: "FM",
			wantText: "RPS",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, port := consentedSession(t, withStoredDuplexNibble(tc.slot, 3))
			data := withMode(t, tc.mode)
			data.FreqHz = tc.freqHz
			_, err := sess.WriteChannel(context.Background(), channelWith(tc.slot, data))
			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want a refusal naming RPS", err)
			}
			if !strings.Contains(refused.Error(), tc.wantText) {
				t.Errorf("refusal %q does not contain %q", refused.Error(), tc.wantText)
			}
			if port.countSets() != 0 {
				t.Error("the driver sent a set frame before refusing")
			}
		})
	}
}

func TestWriteChannelRefusesAnIncomingRPSBeforeAnyWireTraffic(t *testing.T) {
	// THE LOCAL HALF of OQ-6, and the rung REV 3 added to close Codex
	// re-review finding 5: a Known Duplex of "RPS" is not in
	// caps.DuplexOptions, so StringField.Valid refuses it — before the
	// read, whatever the slot and whatever the mode.
	//
	// THE FIXTURE HAS TO BE BAND 3, and that is the whole finding. This
	// test's original fixture wrote mode DD to 144-001, where the
	// CROSS-FIELD rung has its own reason to refuse — "mode DD can be
	// selected only in the 1200 MHz band". Rung 3 does run first, so the
	// duplex refusal was the one observed; but with rung 3's duplex check
	// DELETED the cross-field rung caught the very same channel and
	// produced the very same *driver.WriteRefusedError, so an assertion
	// that "some refusal occurred before any wire traffic" passed either
	// way. The rung REV 3 added to close Codex re-review finding 5 was
	// deletable with the whole suite green.
	//
	// At 1200-001 with mode DD every cross-field constraint is satisfied
	// — DD is in band 3, RPS is permitted WITH DD, and no DUP± is present
	// — so the duplex vocabulary is the ONLY thing left that can refuse,
	// and deleting it puts an RPS record on the wire.
	//
	// The companion case pins the band constraint beside it, with a
	// duplex the vocabulary CAN name so that rung 3 has nothing to say
	// and the two refusals stay distinguishable by the field they name.
	for _, tc := range []struct {
		name, slot string
		freqHz     uint64
		duplex     string
		wantField  spec.Field
		wantText   string
	}{
		{
			name: "RPS in band 3, where only the duplex vocabulary can refuse",
			slot: "1200-001", freqHz: 1_240_000_000, duplex: "RPS",
			wantField: spec.FieldDuplex, wantText: "RPS",
		},
		{
			name: "a nameable duplex in band 1, where the DD band constraint refuses",
			slot: "144-001", freqHz: 145_500_000, duplex: "DUP+",
			wantField: spec.FieldMode, wantText: "DD",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, port := consentedSession(t, withTemplateStateAt(tc.slot))
			data := withModeAndDuplex(t, "DD", tc.duplex)
			data.FreqHz = tc.freqHz
			_, err := sess.WriteChannel(context.Background(), channelWith(tc.slot, data))

			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want *driver.WriteRefusedError", err)
			}
			if !strings.Contains(refused.Error(), tc.wantText) {
				t.Errorf("refusal %q does not name %q", refused.Error(), tc.wantText)
			}
			if !refusalNames(refused, tc.wantField) {
				t.Errorf("refusal names fields %v, want %s among them", refused.Fields, tc.wantField)
			}
			if port.countReads() != 0 || port.countSets() != 0 {
				t.Errorf("a locally decidable refusal sent wire traffic: %d reads, %d sets",
					port.countReads(), port.countSets())
			}
		})
	}
}

// refusalNames reports whether a refusal names field among its Fields.
func refusalNames(refused *driver.WriteRefusedError, field spec.Field) bool {
	for _, f := range refused.Fields {
		if f == field {
			return true
		}
	}
	return false
}

func TestWriteChannelRefusesOutOfDomainToneAndDTCSValues(t *testing.T) {
	// Rung 3 for the numeric domains: caps.AdmitsTone covers 1..2999
	// deciHz and caps.DTCSCodes the 512 octal-digit codes. Neither the
	// codec nor BuildMemorySet would object, which is why the driver
	// must.
	for _, tc := range []struct {
		name string
		data codeplug.ChannelData
	}{
		{"tone below the domain", withToneDeciHz(t, 0)},
		{"tone above the domain", withToneDeciHz(t, 3000)},
		{"DTCS code with an 8 digit", withDTCSCode(t, 823)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, port := consentedSession(t, withTemplateStateAt("144-001"))
			_, err := sess.WriteChannel(context.Background(), channelWith("144-001", tc.data))
			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want *driver.WriteRefusedError", err)
			}
			if port.countReads() != 0 {
				t.Error("a locally decidable refusal read the radio first (T5)")
			}
		})
	}
}

func TestModifyPreservesAnUnknownToneFromTheJustReadRecord(t *testing.T) {
	// T1(4): a tone field that is not Known carries the just-read
	// record's civ-layer number VERBATIM. Not synthesis, not a cache —
	// the value comes from the read this write already had to make.
	sess, port := consentedSession(t, withStoredToneDeciHz("144-001", 1230))
	if _, err := sess.WriteChannel(context.Background(), channelWithUnknownTonesAt("144-001")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if got := toneDeciHzOf(port.lastSet()); got != 1230 {
		t.Errorf("tone written = %d deciHz, want the stored 1230", got)
	}
}

func TestCreateRefusesWhenTheToneHasNoSource(t *testing.T) {
	// T1(5) on a manual that documents no default tone: register entry
	// ic9700-no-documented-default-tone, lift R24. ToneMode Known OFF is
	// not enough — the tone SPANS still need a value, and inventing one
	// is what this refusal exists to prevent.
	sess, port := consentedSession(t, withEmptySlot("144-009"))
	_, err := sess.WriteChannel(context.Background(), createWithToneModeOffButNoTones(t, "144-009"))
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a refusal naming the tone field", err)
	}
	if !strings.Contains(refused.Error(), string(spec.FieldToneTx)) {
		t.Errorf("refusal %q does not name the tone field", refused.Error())
	}
	if port.countSets() != 0 {
		t.Error("a set frame was sent before refusing")
	}
}

func TestWriteChannelRefusesTheCombinationsTheManualExcludes(t *testing.T) {
	// matrix §3.16 A2 / adjudication R11. All three constraints are
	// MANUAL-EVIDENCED and are enforced BEFORE the frame is built; R21
	// lifts the radio's REACTION, not the constraints themselves.
	for _, tc := range []struct {
		name, slot string
		data       codeplug.ChannelData
		want       string // substring the refusal must name
	}{
		{"DD outside the 1200 MHz band", "144-001", withMode(t, "DD"), "DD"},
		{"DD in the 430 MHz band", "430-001", withMode(t, "DD"), "DD"},
		{"RPS without DD", "1200-001", withModeAndDuplex(t, "FM", "RPS"), "RPS"},
		{"duplex + in DD", "1200-002", withModeAndDuplex(t, "DD", "DUP+"), "DD"},
		{"duplex - in DD", "1200-003", withModeAndDuplex(t, "DD", "DUP-"), "DD"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, port := consentedSession(t, withTemplateStateAt(tc.slot))
			_, err := sess.WriteChannel(context.Background(), channelWith(tc.slot, tc.data))
			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want *driver.WriteRefusedError", err)
			}
			if !strings.Contains(refused.Error(), tc.want) {
				t.Errorf("refusal %q does not name %q", refused.Error(), tc.want)
			}
			// LOCALLY DECIDABLE, so it precedes ALL wire traffic (T5).
			// The band comes from the slot string and the mode and
			// duplex from the incoming channel, so nothing here needs
			// to ask the radio anything — and a ladder that asked would
			// have put a read in front of a refusal it did not need.
			if port.countReads() != 0 || port.countSets() != 0 {
				t.Errorf("a locally decidable refusal sent wire traffic: %d reads, %d sets",
					port.countReads(), port.countSets())
			}
		})
	}
}

func TestDDInTheTwelveHundredBandIsAccepted(t *testing.T) {
	// The constraint is a constraint, not a ban: the one combination the
	// manual permits must still go through, or the refusals above are
	// indistinguishable from "DD is unsupported".
	sess, port := consentedSession(t, withTemplateStateAt("1200-004"))
	if _, err := sess.WriteChannel(context.Background(), channelWith("1200-004", withMode(t, "DD"))); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if port.countSets() != 1 {
		t.Fatalf("sent %d set frames, want 1", port.countSets())
	}
}

func TestCreatingAChannelInAnEmptySlotRequiresEveryMandatoryField(t *testing.T) {
	// R6: empty-slot CREATE requires explicit values or refuses. There is
	// no synthesis and no "partial create".
	sess, port := consentedSession(t, withEmptySlot("144-009"))
	_, err := sess.WriteChannel(context.Background(), channelMissing(t, spec.FieldToneTx))
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a refusal naming the non-Known mandatory field", err)
	}
	if port.countSets() != 0 {
		t.Error("the driver sent a set frame before refusing")
	}

	// …and a fully specified create succeeds, writing the template into
	// the regions this tier cannot name.
	if _, err := sess.WriteChannel(context.Background(), fullyKnownChannelAt("144-009")); err != nil {
		t.Fatalf("a fully specified create: %v", err)
	}
	if port.countSets() != 1 {
		t.Errorf("sent %d set frames, want 1", port.countSets())
	}
}

func TestAWrittenRecordCarriesTheTemplateInEveryRegionThisTierCannotName(t *testing.T) {
	// The other side of the E6 guard: what the driver actually SENDS in
	// the 52 unmapped bytes is the profile's Fixed template — the frozen
	// golden's own state — and never zeros or whatever happened to be in
	// a buffer.
	sess, port := consentedSession(t, withEmptySlot("144-009"))
	if _, err := sess.WriteChannel(context.Background(), fullyKnownChannelAt("144-009")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	set := port.lastSet()
	const recordStart = 9
	record := set[recordStart : len(set)-1]
	// The template is reached through the PROFILE's own layout rather
	// than through core/civ/ic9700's FixedTemplateForTest alias: an
	// export_test.go is compiled only for its OWN package's tests, so
	// that alias is unreachable from here by construction. LayoutFor
	// returns a deep copy, which is the public path and the honest one.
	layout, ok := civic9700.Profile().LayoutFor(civic9700.RecordLength)
	if !ok {
		t.Fatalf("the profile declares no %d-byte layout", civic9700.RecordLength)
	}
	template := layout.Fixed
	for _, off := range []int{10, 20, 57, 67} {
		if record[off] != template[off] {
			t.Errorf("record byte %d = %#02x, want the template's %#02x", off, record[off], template[off])
		}
	}
	for _, span := range [][2]int{{24, 48}, {71, 95}} {
		if got, want := record[span[0]:span[1]], template[span[0]:span[1]]; string(got) != string(want) {
			t.Errorf("record[%d:%d] = % X, want % X", span[0], span[1], got, want)
		}
	}
}

func TestWriteIsNeverRetransmitted(t *testing.T) {
	// ClassWriteWithAck: waits for FB, NO retransmission on timeout,
	// write-quarantine engaged.
	sess, port := consentedSession(t, withTemplateStateAt("144-001"), withNoAcknowledgement())
	_, err := sess.WriteChannel(context.Background(), channelAt("144-001"))
	if err == nil {
		t.Fatal("a write that drew no FB must fail")
	}
	if n := port.countSets(); n != 1 {
		t.Fatalf("the set frame was sent %d times; ClassWriteWithAck never retransmits", n)
	}
}

func TestRejectionBecomesErrRejected(t *testing.T) {
	sess, _ := consentedSession(t, withTemplateStateAt("144-001"), withRejection())
	res, err := sess.WriteChannel(context.Background(), channelAt("144-001"))
	if !errors.Is(err, transport.ErrRejected) {
		t.Fatalf("err = %v, want transport.ErrRejected", err)
	}
	// The sentinel's own text is NEWCAT's "?;". The FA this driver
	// actually saw is named in the message this package builds around it.
	if !strings.Contains(err.Error(), "FA") {
		t.Errorf("err = %q, and an IC-9700 user's rejection message never names the FA the radio sent", err)
	}
	// SENT IS ABOUT ATTRIBUTION, NOT SUCCESS. driver.WriteStep defines it
	// as "transmitted with an attributable outcome — success or an
	// explicit rejection", and a radio that answered FA received the
	// frame and refused it. Reporting that identically to a silent radio
	// throws away the distinction the seam exists to make.
	if len(res.Steps) != 1 {
		t.Fatalf("%d steps, want 1", len(res.Steps))
	}
	if !res.Steps[0].Sent {
		t.Error("Sent = false after an EXPLICIT rejection; the frame reached the radio and the radio refused it")
	}
	if res.Steps[0].Confirmed {
		t.Error("Confirmed = true on a refused write")
	}
}

func TestATimedOutWriteIsNotReportedAsAttributablySent(t *testing.T) {
	// The other side of MINOR-4's split, so the fix cannot be widened
	// into "Sent means transmitted". A radio that answers nothing may
	// never have received the frame at all: the outcome is genuinely
	// unknown, and Sent stays false.
	sess, port := consentedSession(t, withTemplateStateAt("144-001"), withNoAcknowledgement())
	res, err := sess.WriteChannel(context.Background(), channelAt("144-001"))
	if err == nil {
		t.Fatal("a write that drew no FB must fail")
	}
	if errors.Is(err, transport.ErrRejected) {
		t.Fatal("silence was reported as a rejection")
	}
	if len(res.Steps) != 1 || res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("steps = %+v, want one step neither sent nor confirmed", res.Steps)
	}
	if port.countSets() != 1 {
		t.Errorf("the set frame was sent %d times; ClassWriteWithAck never retransmits", port.countSets())
	}
}

func TestConsentNeverReachesErase(t *testing.T) {
	consented := spec.ConsentUnverifiedWrites(ic9700.CapabilitiesUnverified())
	for _, bank := range consented.Banks {
		if fs := consented.FieldSupport(bank.ID, spec.FieldErase); fs.CanWrite() {
			t.Errorf("%s: consent opened the erase gate", bank.ID)
		}
	}
}

func TestNoClearFrameCanBeBuiltOrAdmitted(t *testing.T) {
	// The clear form is printed (matrix §3.13) and deliberately unshipped.
	p := civic9700.Profile()
	clear := []byte{0xFE, 0xFE, 0xA2, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFD}
	if p.AllowedCommand(clear) {
		t.Fatal("the gate admitted the clear frame")
	}
}

func TestDriverReportsOneStopBit(t *testing.T) {
	// OQ-3 / enabler E2 / adjudication R2. Spec D3.1 requires every Icom
	// driver to report 1, and E2 lands both the interface and the wiring
	// consultation. The reporter is on the DRIVER, not the Session:
	// wiring holds the driver before the port opens.
	d := ic9700.New(ic9700.RealHardware)
	r, ok := d.(driver.SerialFramingReporter)
	if !ok {
		t.Fatal("the IC-9700 driver does not implement driver.SerialFramingReporter (spec D3.1)")
	}
	if got := r.StopBits(); got != 1 {
		t.Errorf("StopBits() = %d, want 1 (ASSUMED, D5 entry 8, lift R1)", got)
	}
}

func TestWriteResultDescribesTheOneFrameThisRadioSends(t *testing.T) {
	// The neutral seam reports steps rather than named flags, and this
	// radio's choreography is ONE acknowledged memory set. A refusal
	// before any frame was built reports an empty, non-nil Steps slice.
	sess, _ := consentedSession(t, withTemplateStateAt("144-001"))
	res, err := sess.WriteChannel(context.Background(), channelAt("144-001"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 {
		t.Fatalf("%d steps, want 1", len(res.Steps))
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("step = %+v, want sent and confirmed", res.Steps[0])
	}

	refusedRes, err := sess.WriteChannel(context.Background(), channelWithKnown(spec.FieldShift))
	if err == nil {
		t.Fatal("the refusal did not happen")
	}
	if refusedRes.Steps == nil || len(refusedRes.Steps) != 0 {
		t.Errorf("Steps = %v, want an empty non-nil slice for a refusal before any frame was built", refusedRes.Steps)
	}
}

func TestWriteChannelRefusesASlotAndAChannelItCannotName(t *testing.T) {
	sess, port := consentedSession(t, withTemplateStateAt("144-001"))
	if _, err := sess.WriteChannel(context.Background(), channelAt("144-100")); err == nil {
		t.Error("WriteChannel accepted a slot string this radio cannot name")
	}
	if _, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "144-001"}); err == nil {
		t.Error("WriteChannel accepted an EMPTY channel; erase is unshipped on this tier")
	}
	if port.countSets() != 0 {
		t.Errorf("the driver sent %d set frames for channels it cannot write", port.countSets())
	}
}

func TestAnAnswerMismatchDuringTheWritesReadAbortsTheWrite(t *testing.T) {
	// T2 reaches the write path through its single read: a reply naming
	// another slot must abort rather than let one slot's unmapped state
	// validate another's write.
	sess, port := consentedSession(t,
		withTemplateStateAt("144-001"),
		withAnswerForAddress(civ.ChannelAddress{Group: 1, Channel: 7}))
	_, err := sess.WriteChannel(context.Background(), channelAt("144-001"))
	if !errors.Is(err, ic9700.ErrAnswerMismatch) {
		t.Fatalf("err = %v, want ErrAnswerMismatch", err)
	}
	if port.countSets() != 0 {
		t.Error("the driver sent a set frame after a mismatched read")
	}
}

func TestWriteChannelRefusesAnOutOfBandFrequencyInEitherDirection(t *testing.T) {
	// THE ASYMMETRY THIS CLOSES. Rung 3 domain-checks every Known value
	// against this radio's own capabilities, and it once checked the
	// RECEIVE frequency while leaving the TRANSMIT one unchecked — so a
	// consented modify carrying TxFreqHz = 2 GHz put a set frame on the
	// wire from a driver whose capabilities declare 144–1300 MHz, while
	// the identical value in FreqHz was refused. Five packed-BCD bytes
	// hold ten digits, so the codec had no reason to object either, and
	// the register entry `ic9700-storable-frequency-bounds` is precisely
	// about the bounds that were not being enforced.
	//
	// Both directions, both in and out of band, so neither check can be
	// deleted without a failure and neither can be tightened into
	// refusing an ordinary write.
	const inBand = 145_500_000
	const outOfBand = 2_000_000_000

	for _, tc := range []struct {
		name  string
		field spec.Field
		data  codeplug.ChannelData
		want  bool // want a refusal
	}{
		{"receive frequency in band", spec.FieldFrequency, dataWithFreq(inBand), false},
		{"receive frequency above the band plan", spec.FieldFrequency, dataWithFreq(outOfBand), true},
		{"transmit frequency in band", spec.FieldTxFrequency, dataWithTxFreq(inBand), false},
		{"transmit frequency above the band plan", spec.FieldTxFrequency, dataWithTxFreq(outOfBand), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, port := consentedSession(t, withTemplateStateAt("144-001"))
			_, err := sess.WriteChannel(context.Background(), channelWith("144-001", tc.data))
			if !tc.want {
				if err != nil {
					t.Fatalf("WriteChannel: %v — this value is inside the declared band plan", err)
				}
				if port.countSets() != 1 {
					t.Errorf("sent %d set frames, want 1", port.countSets())
				}
				return
			}
			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want *driver.WriteRefusedError", err)
			}
			if !strings.Contains(refused.Error(), string(tc.field)) {
				t.Errorf("refusal %q does not name %s", refused.Error(), tc.field)
			}
			// Locally decidable: the bounds are in this driver's own
			// capabilities, so nothing needs to be asked of the radio.
			if port.countReads() != 0 || port.countSets() != 0 {
				t.Errorf("a locally decidable refusal sent wire traffic: %d reads, %d sets",
					port.countReads(), port.countSets())
			}
		})
	}
}

// dataWithFreq is a MODIFY carrying a receive frequency.
func dataWithFreq(hz uint64) codeplug.ChannelData {
	d := *channelAt("144-001").Data
	d.FreqHz = hz
	return d
}

// dataWithTxFreq is a MODIFY carrying a Known transmit frequency.
func dataWithTxFreq(hz uint64) codeplug.ChannelData {
	d := *channelAt("144-001").Data
	d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: hz}
	return d
}

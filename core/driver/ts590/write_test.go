// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"bytes"
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// writeImage is a radioImage that also says what an MW Set draws back.
//
// IT IS A SEPARATE IMAGE RATHER THAN A FIELD ON radioImage BECAUSE THIS
// TASK'S FILE LIST DOES NOT INCLUDE respondingport_test.go, and the plan's
// lane rule is that a task touches the files its own entry names (plan
// §Stage 2). The scripted radio the read path needs answers "?;" to every
// frame it does not know, MW included, so the write path needs a peer that
// knows one more grammar; everything else — the identity probe, FV, the MR
// answers, the silence rows — is served by DELEGATION to radioImage.reply,
// so the two images cannot disagree about what a probe says.
type writeImage struct {
	radioImage
	// mwReject makes every 50-byte MW Set answer "?;" — the radio's
	// explicit rejection, which is attributable and therefore reported
	// with Sent true.
	mwReject bool
}

// replyWrite answers frame: the 50-byte MW Set here, everything else through
// the read path's own image.
//
// SILENCE IS THE ACCEPTANCE SIGNAL AND IT IS AN ASSUMED CONVENTION APPLIED,
// NOT AN OBSERVED RADIO TRANSCRIBED (A6). No Kenwood radio has ever been
// written to by this project; that a "?;" is a rejection at all is the books'
// own error table (590:100-105) and that an accepted Set draws nothing at all
// is assumed. It is precisely because the convention is assumed that
// WriteChannel reports Sent and never Confirmed.
//
// AN MW OF ANY OTHER WIDTH FALLS THROUGH TO "?;", which is the right answer
// for this peer: the 42-byte erase form of 590:1579-1581 is a frame this
// milestone never builds, and a test that saw one accepted would be the
// failure the codec's own 50-byte gate exists to prevent.
func (img writeImage) replyWrite(frame string) string {
	if strings.HasPrefix(frame, "MW") && len(frame) == kw.RecordLen {
		if img.mwReject {
			return "?;"
		}
		return ""
	}
	return img.radioImage.reply(frame)
}

// mwPort is the write path's scripted radio: respondingPort's shape over
// writeImage. See writeImage for why it is not respondingPort itself.
type mwPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// newMWPort starts a scripted radio serving img and registers its cleanup.
func newMWPort(t *testing.T, row Row, img writeImage) *mwPort {
	t.Helper()
	if img.catID == "" {
		img.catID = catIDFor(row)
	}
	if img.fvAnswer == "" {
		img.fvAnswer = "FV1.00;"
	}
	host, remote := net.Pipe()
	p := &mwPort{host: host, remote: remote}
	t.Cleanup(func() {
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve(img)
	return p
}

// Port returns the end handed to the driver, which takes ownership of it.
func (p *mwPort) Port() transport.Port { return p.host }

// Transcript returns a copy of every complete frame received, in order.
func (p *mwPort) Transcript() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.received...)
}

// serve splits the driver's bytes into ';'-terminated frames, records each
// and answers per img.
func (p *mwPort) serve(img writeImage) {
	buf := make([]byte, 256)
	var acc []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				i := bytes.IndexByte(acc, ';')
				if i < 0 {
					break
				}
				frame := string(acc[:i+1])
				acc = acc[i+1:]
				p.mu.Lock()
				p.received = append(p.received, frame)
				p.mu.Unlock()
				if reply := img.replyWrite(frame); reply != "" {
					if _, werr := p.remote.Write([]byte(reply)); werr != nil {
						return
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// openWriteSession opens row at profile against a scripted radio serving img.
//
// THE PROFILE IS AN ARGUMENT AND NOT A DEFAULT, and that is plan P7's H2 in
// one signature: the capability-gate rung is pinned on an unconsented
// RealHardware session and every SEMANTIC rung on a session that has already
// passed that gate (Simulated, or RealHardware with consent). A helper that
// chose the profile for its callers is exactly how a whole ladder of
// semantic pins goes green with none of the rungs implemented.
func openWriteSession(t *testing.T, row Row, profile Profile, img writeImage, opts ...Option) (*Session, *mwPort) {
	t.Helper()
	p := newMWPort(t, row, img)
	d := New(row, profile, append([]Option{testTiming()}, opts...)...)
	sess, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open(%s): %v", modelNameFor(row), err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), p
}

// probeFrames is what every session sends before anything a test asked for:
// Init's AI0; and the two-frame identity probe (plan P9).
var probeFrames = []string{"AI0;", "ID;", "FV;"}

// writableChannel is the ONE channel shape this milestone can actually put on
// the wire, for slot id on row — and the fact that it takes this much saying
// is the refusal ladder's own summary.
//
// It is populatedFields' record read back as neutral fields, so a channel
// built here and a channel READ from that fixture describe the same radio
// state: 145.500 MHz FM Normal, data mode off, TONE on with TN index 08 and
// CN index 08 (88.5 Hz on both charts, kenwoodCTCSSTones[8]), lockout off,
// named "SIMPLEX".
//
// TWO CELLS ARE PER-CONTEXT, and they are the two the capability table
// varies on, so a fixture that set them unconditionally would be refused by
// the capability gate for a reason no test was asking about:
//
//   - TxFreqHz is Known ONLY on a MEM slot, where the bank publishes
//     FieldTxFrequency. The value equals FreqHz, which is the only TX
//     disposition a single MW can express: its P1 comes from the slot's
//     class (M9, kw.Slot.P1) and a P1='0' write makes the channel simplex
//     "even if it was already a split channel" (590:1521-1523). In the SCAN
//     bank the field is the zero FieldSupport (M-E2), so a Known value there
//     would be refused by the capability gate.
//   - Filter is Known ONLY on the SG row, which is the only row publishing
//     the two labels (Q12, §2.7).
func writableChannel(row Row, id string) codeplug.Channel {
	data := codeplug.ChannelData{
		FreqHz:     145_500_000,
		Mode:       "FM",
		Tag:        "SIMPLEX",
		ScanSkip:   codeplug.BoolField{State: codeplug.Known, Value: false},
		DataMode:   codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode:   codeplug.StringField{State: codeplug.Known, Value: "TONE"},
		ToneTx:     codeplug.ToneField{State: codeplug.Known, Value: kenwoodCTCSSTones[8]},
		ToneRx:     codeplug.ToneField{State: codeplug.Known, Value: kenwoodCTCSSTones[8]},
		TxFreqHz:   codeplug.FreqField{State: codeplug.Unavailable},
		Filter:     codeplug.StringField{State: codeplug.Unavailable},
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
	}
	if len(id) == 3 {
		data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: data.FreqHz}
	}
	if row == RowSG {
		data.Filter = codeplug.StringField{State: codeplug.Known, Value: filterALabel}
	}
	return codeplug.Channel{Slot: id, Data: &data}
}

// TestMWSetSpec_IsFireAndForgetAndNeverRetries pins the three properties of
// the write's transport spec that are load-bearing rather than incidental.
//
// NO Match, and therefore no answer length: on the assumed convention an
// accepted Set produces no answer at all, so a spec that waited for an "MW"
// reply would spend a whole read timeout and then report a timeout for a
// write the radio had accepted. transport.Engine refuses a ClassWrite spec
// carrying a Match outright.
//
// RetryReads 0, NECESSARILY: transport safety obligation 2 forbids resending
// a write, and Do refuses a write-class spec with a non-zero RetryReads
// before writing anything. Resending an accepted Set would write the channel
// twice; resending one whose fate is unknown would write it a second time on
// top of a first that may have landed.
func TestMWSetSpec_IsFireAndForgetAndNeverRetries(t *testing.T) {
	got := mwSetSpec()
	if got.Class != transport.ClassWrite {
		t.Errorf("mwSetSpec().Class = %v, want transport.ClassWrite", got.Class)
	}
	if got.Match != nil {
		t.Error("mwSetSpec() carries a Match; a fire-and-forget write must not wait for an answer")
	}
	if got.RetryReads != 0 {
		t.Errorf("mwSetSpec().RetryReads = %d, want 0 — a write is never resent", got.RetryReads)
	}
}

// TestRequestedFields_MembershipAndOrder pins the requested-set table against
// this package's own literal list of the twenty-seven spec.Fields (plan P5:
// no Kenwood file names spec.AllFields).
//
// THE TABLE NAMES TWENTY-SIX OF THE TWENTY-SEVEN, AND THE ONE IT OMITS IS
// spec.FieldErase, BY NAME. An erase is not a field a write requests: it is
// the whole shape of a DIFFERENT frame, the short MW of 590:1579-1581 this
// milestone never builds, and WriteChannel refuses an empty channel one rung
// above this table. A field silently missing from the table would be a field
// a caller could set and have dropped from the frame without a refusal, which
// is the C-M1 class of defect the fleet's FieldState walk exists to close.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	var want []spec.Field
	for _, f := range allSpecFields {
		if f == spec.FieldErase {
			continue
		}
		want = append(want, f)
	}
	var got []spec.Field
	for _, r := range requestedFieldRules {
		got = append(got, r.field)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requestedFieldRules names\n %v\nwant the twenty-seven less spec.FieldErase, in the same order\n %v", got, want)
	}
}

// TestRequestedFields_TheEightTheRecordAlwaysCarries pins which fields a
// write requests UNCONDITIONALLY, and it is the frame's own shape: the
// 50-byte record has a position for each of them on every write, with no
// "leave it alone" encoding anywhere in the grid (590:1518-1538, the
// positional grid; 590:1539-1577 is the parameter definitions that follow
// it).
//
// The other eighteen are requested only when the channel actually carries a
// value for them, so an ordinary write is not refused by the capability gate
// naming a field nobody asked to write.
func TestRequestedFields_TheEightTheRecordAlwaysCarries(t *testing.T) {
	want := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag, spec.FieldScanSkip,
		spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx, spec.FieldDataMode,
	}
	got := requestedFields(codeplug.ChannelData{})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("requestedFields(zero data) = %v, want the eight the record always carries %v", got, want)
	}
}

// TestRequestedFields_EveryConditionalIsReachable is the other half: each of
// the eighteen conditional entries is requested by SOME channel, so a
// predicate that could never fire — a mirror of the neutral model that has
// drifted from codeplug.ChannelData — fails here rather than silently letting
// a Known value be dropped from a frame it has no room for.
func TestRequestedFields_EveryConditionalIsReachable(t *testing.T) {
	always := map[spec.Field]bool{
		spec.FieldFrequency: true, spec.FieldMode: true, spec.FieldTag: true,
		spec.FieldScanSkip: true, spec.FieldToneMode: true, spec.FieldToneTx: true,
		spec.FieldToneRx: true, spec.FieldDataMode: true,
	}
	// One channel carrying a value for EVERY conditional field at once.
	data := codeplug.ChannelData{
		ClarHz:              10,
		CTCSS:               "ENC",
		Shift:               "PLUS",
		CTCSSTone:           codeplug.ToneField{State: codeplug.Known, Value: 885},
		TagDisplay:          codeplug.BoolField{State: codeplug.Known, Value: true},
		TxFreqHz:            codeplug.FreqField{State: codeplug.Known, Value: 1},
		Duplex:              codeplug.StringField{State: codeplug.Known, Value: "SIMPLEX"},
		OffsetHz:            codeplug.FreqField{State: codeplug.Known, Value: 1},
		DTCSCode:            codeplug.IntField{State: codeplug.Known, Value: 23},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Known, Value: "NN"},
		Filter:              codeplug.StringField{State: codeplug.Known, Value: filterALabel},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Known, Value: true},
		TuningStep:          codeplug.StringField{State: codeplug.Known, Value: "10k"},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Known, Value: 1},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Known, Value: 12},
		Preamp:              codeplug.StringField{State: codeplug.Known, Value: "ON"},
		Antenna:             codeplug.StringField{State: codeplug.Known, Value: "ANT1"},
		IPPlus:              codeplug.BoolField{State: codeplug.Known, Value: true},
	}
	got := map[spec.Field]bool{}
	for _, f := range requestedFields(data) {
		got[f] = true
	}
	for _, r := range requestedFieldRules {
		if always[r.field] {
			continue
		}
		if !got[r.field] {
			t.Errorf("%s is never requested: no channel this test can build reaches its predicate", r.field)
		}
	}
	// And the clarifier's three Go members are one spec.Field, so each of
	// them alone must request it (core/codeplug/diff.go's own grouping).
	for _, tc := range []struct {
		name string
		data codeplug.ChannelData
	}{
		{"offset", codeplug.ChannelData{ClarHz: -10}},
		{"rx flag", codeplug.ChannelData{RxClar: true}},
		{"tx flag", codeplug.ChannelData{TxClar: true}},
	} {
		found := false
		for _, f := range requestedFields(tc.data) {
			if f == spec.FieldClarifier {
				found = true
			}
		}
		if !found {
			t.Errorf("a channel carrying a clarifier %s does not request spec.FieldClarifier", tc.name)
		}
	}
}

// TestWriteChannel_OneMWReportedSentNeverConfirmed is the write path's whole
// choreography (plan P14, matrix §3.9): ONE 50-byte MW, and a result that
// says Sent and never Confirmed.
//
// CONFIRMED ON SILENCE WOULD BE ASSERTING A6 AS A FACT. That an accepted Set
// draws nothing back is an assumed convention on this family — no Kenwood
// radio has ever been written to by this project — so silence is
// INCONCLUSIVE and the honest report is "the frame went out". The read-back
// verification is core/clone's (core/clone/execute.go's write-then-verify
// pair), which is why the transcript below carries no MR: WriteChannel never
// calls ReadChannel, and a driver that did would double every write.
func TestWriteChannel_OneMWReportedSentNeverConfirmed(t *testing.T) {
	for _, row := range bothRows {
		sess, p := openWriteSession(t, row, Simulated, writeImage{})
		res, err := sess.WriteChannel(context.Background(), writableChannel(row, "042"))
		if err != nil {
			t.Fatalf("%s: WriteChannel: %v", modelNameFor(row), err)
		}
		want := []driver.WriteStep{{Command: "MW", Sent: true, Confirmed: false}}
		if !reflect.DeepEqual(res.Steps, want) {
			t.Errorf("%s: Steps = %+v, want %+v — Sent, never Confirmed", modelNameFor(row), res.Steps, want)
		}
		got := p.Transcript()
		if len(got) != len(probeFrames)+1 {
			t.Fatalf("%s: transcript = %v, want the probe's three frames and exactly one more", modelNameFor(row), got)
		}
		frame := got[len(got)-1]
		if !strings.HasPrefix(frame, "MW") || len(frame) != kw.RecordLen {
			t.Errorf("%s: wrote %q, want one 50-byte MW", modelNameFor(row), frame)
		}
		for _, f := range got {
			if strings.HasPrefix(f, "MR") {
				t.Errorf("%s: transcript contains %q — WriteChannel must never read back (P14)", modelNameFor(row), f)
			}
		}
	}
}

// TestWriteChannel_TheFiftyByteFrameIsHandDerived re-derives the whole frame
// from the book's own printed grid rather than from the builder under test,
// on TWO records — one MEM slot and one SCAN upper half.
//
// THE SCAN 'U' ROW IS THE M9 PIN RE-ASSERTED AT DRIVER LEVEL. MW's P1 comes
// from the SLOT'S CLASS and never from the channel's split state: '0' for a
// MEM slot and for a SCAN 'L' slot, '1' for a SCAN 'U' slot
// (590:1529-1531). A driver that derived P1 from anywhere else would write
// the START frequency into the slot the user edited as its END — the silent
// data loss decision 11 exists to prevent — and it would pass every other
// assertion in this file.
func TestWriteChannel_TheFiftyByteFrameIsHandDerived(t *testing.T) {
	// Positions, 1-indexed as the book prints them (590:1518-1538; the
	// parameter definitions that follow are at 590:1539-1577):
	// 1-2 "MW" | 3 P1 | 4 P2 | 5-6 P3 | 7-17 P4, 11 digits | 18 P5 | 19 P6
	// 20 P7 | 21-22 P8 | 23-24 P9 | 25-27 P10 "000" | 28 P11 | 29 P12 "0"
	// 30-38 P13 "000000000" | 39-40 P14 | 41 P15 | 42-49 P16 | 50 ";"
	for _, tc := range []struct {
		slot string
		want string
	}{
		{
			slot: "042",
			want: "MW" + "0" + "0" + "42" + "00145500000" + "4" + "0" + "1" +
				"08" + "08" + "000" + "0" + "0" + "000000000" + "00" + "0" + "SIMPLEX " + ";",
		},
		{
			slot: "100U",
			want: "MW" + "1" + "1" + "00" + "00145500000" + "4" + "0" + "1" +
				"08" + "08" + "000" + "0" + "0" + "000000000" + "00" + "0" + "SIMPLEX " + ";",
		},
	} {
		if len(tc.want) != kw.RecordLen {
			t.Fatalf("%s: the hand-derived frame is %d bytes, want %d — the derivation is wrong, not the code", tc.slot, len(tc.want), kw.RecordLen)
		}
		sess, p := openWriteSession(t, RowSG, Simulated, writeImage{})
		if _, err := sess.WriteChannel(context.Background(), writableChannel(RowSG, tc.slot)); err != nil {
			t.Fatalf("%s: WriteChannel: %v", tc.slot, err)
		}
		got := p.Transcript()
		if frame := got[len(got)-1]; frame != tc.want {
			t.Errorf("%s: MW frame\n got %q\nwant %q", tc.slot, frame, tc.want)
		}
	}
}

// TestWriteChannel_TheSRowsByte28IsTheDocumentedZero pins the one byte this
// milestone writes without reading it, and the register entry that licenses
// it: on a TS-590S at firmware 1.xx byte 28 is a printed-fixed '0' — "In
// firmware version 1.xx of TS-590S, always 0" (590:1478, and MW's differing
// wording at 590:1564, which is erratum E7) — so the write emits '0' under
// A14 rather than defaulting it.
//
// It is reachable only because the A13/A14 rung has already refused every S
// whose FV is >= 2.00 or unparseable: on an S that gets this far the
// document GUARANTEES the zero, which is what makes this a defaulted byte
// WITH a register entry rather than an invented one.
func TestWriteChannel_TheSRowsByte28IsTheDocumentedZero(t *testing.T) {
	sess, p := openWriteSession(t, RowS, Simulated, writeImage{})
	if _, err := sess.WriteChannel(context.Background(), writableChannel(RowS, "042")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got := p.Transcript()
	frame := got[len(got)-1]
	if frame[27] != '0' {
		t.Errorf("byte 28 = %q on a TS-590S at FV 1.00, want '0' (A14, 590:1478)", frame[27])
	}
}

// TestWriteChannel_TheSGsByte28CarriesTheChosenFilter is the SG half: byte 28
// is a LIVE FILTER A/B selector there (590:1560-1563), so the byte comes from
// the channel and a write of FILTER B is not silently normalised to A.
func TestWriteChannel_TheSGsByte28CarriesTheChosenFilter(t *testing.T) {
	for _, tc := range []struct {
		label string
		want  byte
	}{{filterALabel, '0'}, {filterBLabel, '1'}} {
		sess, p := openWriteSession(t, RowSG, Simulated, writeImage{})
		ch := writableChannel(RowSG, "042")
		ch.Data.Filter = codeplug.StringField{State: codeplug.Known, Value: tc.label}
		if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
			t.Fatalf("%s: WriteChannel: %v", tc.label, err)
		}
		got := p.Transcript()
		if frame := got[len(got)-1]; frame[27] != tc.want {
			t.Errorf("%s: byte 28 = %q, want %q (590:1560-1563)", tc.label, frame[27], tc.want)
		}
	}
}

// TestWriteChannel_ARejectionIsTypedAndAttributable pins the one wire outcome
// that is not silence: a "?;" is the typed kw.RejectionError, which names the
// two indistinguishable causes the books print for it and cites the
// transient-suppression sentence, and the step reports Sent TRUE — the frame
// provably went out and the radio provably refused it — with Confirmed false.
func TestWriteChannel_ARejectionIsTypedAndAttributable(t *testing.T) {
	sess, _ := openWriteSession(t, RowSG, Simulated, writeImage{mwReject: true})
	res, err := sess.WriteChannel(context.Background(), writableChannel(RowSG, "042"))
	if !errors.Is(err, transport.ErrRejected) {
		t.Fatalf("WriteChannel err = %v, want a rejection", err)
	}
	var rej *kw.RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("WriteChannel err = %v (%T), want *kw.RejectionError", err, err)
	}
	if rej.Command != "MW" {
		t.Errorf("RejectionError.Command = %q, want %q", rej.Command, "MW")
	}
	want := []driver.WriteStep{{Command: "MW", Sent: true, Confirmed: false}}
	if !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("Steps = %+v, want %+v — a rejected frame still went out", res.Steps, want)
	}
}

// TestWriteChannel_ARoundTripFromTheReadPathReachesTheWire closes the loop
// this package's two paths make together: a channel READ off the scripted
// radio, given the one thing a fresh read cannot supply — a Known TX
// disposition (A9) — writes back byte for byte as the same record.
//
// It is the positive control for the whole ladder in its most honest form:
// every value in the frame came from the radio rather than from a fixture.
func TestWriteChannel_ARoundTripFromTheReadPathReachesTheWire(t *testing.T) {
	const id = "042"
	sess, p := openWriteSession(t, RowSG, Simulated, writeImage{
		radioImage: radioImage{mrAnswers: map[string]string{mrAddr(id): populatedMR(id)}},
	})
	ch, err := sess.ReadChannel(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	// The one edit A9 requires, and the only TX disposition one MW can
	// express: this channel is simplex (590:1521-1523).
	ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: ch.Data.FreqHz}
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got := p.Transcript()
	frame := got[len(got)-1]
	answer := populatedMR(id)
	if frame[2:] != answer[2:] {
		t.Errorf("the written record differs from the one read\n got %q\nwant %q (the MR answer's own parameter block)", frame, answer)
	}
}

// TestWriteChannel_TheMandatoryLiveBytesRequireAKnownValue pins the refusals
// the FRAME's own shape earns, one per byte that is live on this row and has
// no "leave it alone" encoding.
//
// EVERY ONE OF THESE POSITIONS IS TRANSMITTED ON EVERY WRITE (590:1539-1577
// accounts for all 47 parameter bytes), so a non-Known value cannot be
// omitted from the frame — it can only be MANUFACTURED, which is what
// codeplug's write rule forbids for a field whose state says "preserve
// whatever the radio has". The alternative — writing a zero — is the
// silent-default this milestone refuses everywhere else.
func TestWriteChannel_TheMandatoryLiveBytesRequireAKnownValue(t *testing.T) {
	for _, tc := range []struct {
		name  string
		row   Row
		blank func(*codeplug.ChannelData)
		field spec.Field
	}{
		{"data mode, byte 19", RowSG, func(d *codeplug.ChannelData) {
			d.DataMode = codeplug.BoolField{State: codeplug.Unavailable}
		}, spec.FieldDataMode},
		{"tone mode, P7", RowSG, func(d *codeplug.ChannelData) {
			d.ToneMode = codeplug.StringField{State: codeplug.Unavailable}
		}, spec.FieldToneMode},
		{"tone tx, P8", RowSG, func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Unavailable}
		}, spec.FieldToneTx},
		{"tone rx, P9", RowSG, func(d *codeplug.ChannelData) {
			d.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
		}, spec.FieldToneRx},
		{"scan skip, P15", RowSG, func(d *codeplug.ChannelData) {
			d.ScanSkip = codeplug.BoolField{State: codeplug.Unavailable}
		}, spec.FieldScanSkip},
		{"filter, byte 28 on the SG", RowSG, func(d *codeplug.ChannelData) {
			d.Filter = codeplug.StringField{State: codeplug.Unavailable}
		}, spec.FieldFilter},
	} {
		sess, p := openWriteSession(t, tc.row, Simulated, writeImage{})
		ch := writableChannel(tc.row, "042")
		tc.blank(ch.Data)
		_, err := sess.WriteChannel(context.Background(), ch)
		var ref *driver.WriteRefusedError
		if !errors.As(err, &ref) {
			t.Fatalf("%s: err = %v (%T), want *driver.WriteRefusedError", tc.name, err, err)
		}
		if !reflect.DeepEqual(ref.Fields, []spec.Field{tc.field}) {
			t.Errorf("%s: refusal names %v, want [%s]", tc.name, ref.Fields, tc.field)
		}
		if got := p.Transcript(); len(got) != len(probeFrames) {
			t.Errorf("%s: transcript = %v, want no frame beyond the probe", tc.name, got)
		}
	}
}

// TestWriteChannel_ATagTheRecordCannotHoldIsRefused pins the frame-shaped
// refusals the CODEC owns, reached through the driver: P16 holds eight bytes
// (590:1576) and ';' cannot appear in it (590:1577, A2's charset). The
// refusal is the driver's typed one and no frame is built.
func TestWriteChannel_ATagTheRecordCannotHoldIsRefused(t *testing.T) {
	for _, tag := range []string{"TOOLONGATAG", "SEMI;COLON"} {
		sess, p := openWriteSession(t, RowSG, Simulated, writeImage{})
		ch := writableChannel(RowSG, "042")
		ch.Data.Tag = tag
		_, err := sess.WriteChannel(context.Background(), ch)
		if !errors.Is(err, driver.ErrWriteRefused) {
			t.Fatalf("tag %q: err = %v, want a write refusal", tag, err)
		}
		if got := p.Transcript(); len(got) != len(probeFrames) {
			t.Errorf("tag %q: transcript = %v, want no frame beyond the probe", tag, got)
		}
	}
}

// TestWriteChannel_AFrequencyWiderThanTheFieldIsRefusedByTheCodec pins that
// the only frequency guard this milestone ships is the ENCODABILITY one, and
// that it comes from the codec with its own message: MinFreqHz/MaxFreqHz are
// 0/0 on both rows (M-E6), which DISABLES codeplug.Validate's floor and
// ceiling, so a channel at 1 Hz writes and one needing more than eleven
// digits is refused by the field width (A17).
func TestWriteChannel_AFrequencyWiderThanTheFieldIsRefusedByTheCodec(t *testing.T) {
	sess, p := openWriteSession(t, RowSG, Simulated, writeImage{})
	ch := writableChannel(RowSG, "042")
	ch.Data.FreqHz = kw.MaxRecordFreqHz + 1
	ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: ch.Data.FreqHz}
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, kw.ErrOutOfDomain) {
		t.Fatalf("err = %v, want the codec's out-of-domain refusal", err)
	}
	if got := p.Transcript(); len(got) != len(probeFrames) {
		t.Errorf("transcript = %v, want no frame beyond the probe", got)
	}

	// And the other end of the same disabled check: 1 Hz is written.
	sess2, p2 := openWriteSession(t, RowSG, Simulated, writeImage{})
	low := writableChannel(RowSG, "042")
	low.Data.FreqHz = 1
	low.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 1}
	if _, err := sess2.WriteChannel(context.Background(), low); err != nil {
		t.Fatalf("a 1 Hz channel was refused: %v", err)
	}
	got := p2.Transcript()
	if frame := got[len(got)-1]; frame[6:17] != "00000000001" {
		t.Errorf("P4 = %q, want the 11-digit encoding of 1 Hz", frame[6:17])
	}
}

// TestWriteChannel_IsAtomicUnderOpMu is MEDIUM-1's fix (Opus review, T12 fix
// round 1): the read path has its own deterministic negative pin
// (read_test.go's TestReadChannel_IsAtomicUnderOpMu), and the WRITE path
// reused none of it — deleting the two opMu lines from WriteChannel left the
// whole 103-second package green.
//
// THE HOOK IS read.go's readChannelGapHook, REUSED RATHER THAN DUPLICATED: it
// parks a ReadChannel deterministically inside opMu (already held, before any
// frame is built), which is what makes a WriteChannel racing it against
// scheduling alone near-impossible to reproduce otherwise. While the read is
// parked, WriteChannel must not so much as build its frame — the mutex is
// P13/P14's WHOLE-OPERATION guarantee, on the one operation that changes the
// radio.
//
// RED PROOF, observed: with the two opMu lines removed from WriteChannel, the
// MW leaves within microseconds of the parked ReadChannel and this test fails
// at "WriteChannel returned while a ReadChannel held opMu".
func TestWriteChannel_IsAtomicUnderOpMu(t *testing.T) {
	const id = "001"
	sess, p := openWriteSession(t, RowSG, Simulated, writeImage{
		radioImage: radioImage{mrAnswers: map[string]string{mrAddr(id): populatedMR(id)}},
	})

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	readChannelGapHook = func() {
		entered <- struct{}{}
		<-release
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), id); err != nil {
			t.Errorf("parked ReadChannel: %v", err)
		}
	}()
	<-entered // the read is inside opMu and parked

	writeDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.WriteChannel(context.Background(), writableChannel(RowSG, "042")); err != nil {
			t.Errorf("WriteChannel: %v", err)
		}
		close(writeDone)
	}()

	select {
	case <-writeDone:
		t.Fatal("WriteChannel returned while a ReadChannel held opMu")
	case <-time.After(250 * time.Millisecond):
	}
	if got := p.Transcript(); len(got) != len(probeFrames) {
		t.Errorf("transcript while the read is parked inside opMu = %v, want the probe's three frames alone", got)
	}

	close(release)
	wg.Wait()

	got := p.Transcript()
	if len(got) != len(probeFrames)+2 {
		t.Fatalf("transcript = %v, want the probe, one MR and one MW", got)
	}
	if !strings.HasPrefix(got[len(probeFrames)], "MR") || !strings.HasPrefix(got[len(probeFrames)+1], "MW") {
		t.Errorf("transcript = %v, want the parked ReadChannel's MR before the released WriteChannel's MW", got)
	}
}

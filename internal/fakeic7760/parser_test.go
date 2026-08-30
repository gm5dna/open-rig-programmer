package fakeic7760

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func frame(to, from byte, payload ...byte) []byte {
	out := append([]byte{0xFE, 0xFE, to, from}, payload...)
	return append(out, 0xFD)
}
func wireTo(payload ...byte) []byte   { return frame(AddrRadio, AddrController, payload...) }
func wireFrom(payload ...byte) []byte { return frame(AddrController, AddrRadio, payload...) }
func record(seed byte) []byte {
	b := make([]byte, RecordLen)
	for i := range b {
		b[i] = seed + byte(i)
	}
	return b
}

func readFrame(t *testing.T, r *Radio) []byte {
	t.Helper()
	c := r.Port()
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	defer c.SetReadDeadline(time.Time{})
	var got []byte
	one := make([]byte, 1)
	for {
		n, err := c.Read(one)
		if n > 0 {
			got = append(got, one[0])
			if one[0] == 0xFD {
				return got
			}
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecordGeometryAndSelectors(t *testing.T) {
	if RecordLen != 25 || NameLen != 10 {
		t.Fatalf("geometry = record %d/name %d, want 25/10", RecordLen, NameLen)
	}
	tests := []struct {
		ch     int
		hi, lo byte
	}{{1, 0, 1}, {99, 0, 0x99}, {ChanP1, 1, 0}, {ChanP2, 1, 1}}
	for _, tt := range tests {
		hi, lo, ok := selectorFor(tt.ch)
		if !ok || hi != tt.hi || lo != tt.lo {
			t.Errorf("selectorFor(%d) = %02X %02X %v", tt.ch, hi, lo, ok)
		}
	}
}

func TestMemoryReadAndSetUseTheIndependentTwentyFiveByteRecord(t *testing.T) {
	r := New()
	defer r.Close()
	rec := record(0x10)
	set := append([]byte{0x1A, 0x00, 0x00, 0x07}, rec...)
	if _, err := r.Port().Write(wireTo(set...)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, r); !bytes.Equal(got, wireFrom(0xFB)) {
		t.Fatalf("set reply = % X, want ACK", got)
	}
	if _, err := r.Port().Write(wireTo(0x1A, 0x00, 0x00, 0x07)); err != nil {
		t.Fatal(err)
	}
	wantPayload := append([]byte{0x1A, 0x00, 0x00, 0x07}, rec...)
	want := wireFrom(wantPayload...)
	if got := readFrame(t, r); !bytes.Equal(got, want) {
		t.Fatalf("read reply = % X, want % X", got, want)
	}
}

func TestWrongLengthAndClearAreRefused(t *testing.T) {
	r := New()
	defer r.Close()
	for _, payload := range [][]byte{{0x1A, 0x00, 0x00, 0x01, 0x00}, append([]byte{0x1A, 0x00, 0x00, 0x01}, make([]byte, RecordLen+1)...), {0x1A, 0x00, 0x00, 0x01, 0xFF}} {
		if _, err := r.Port().Write(wireTo(payload...)); err != nil {
			t.Fatal(err)
		}
		if got := readFrame(t, r); !bytes.Equal(got, wireFrom(0xFA)) {
			t.Fatalf("refusal = % X, want NG", got)
		}
	}
}

func TestWrongAddressIsSilentAndEchoIsOptional(t *testing.T) {
	r := New(WithUSBEcho())
	defer r.Close()
	wrong := []byte{0xFE, 0xFE, 0xB3, AddrController, 0x19, 0x00, 0xFD}
	if _, err := r.Port().Write(wrong); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, r); !bytes.Equal(got, wrong) {
		t.Fatalf("echo = % X, want % X", got, wrong)
	}
	if _, err := r.Port().Write(wireTo(0x19, 0x00)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, r); len(got) == 0 || got[4] != 0x19 {
		t.Fatalf("identity reply = % X", got)
	}
}

func TestPortIsADeadlineCapablePipe(t *testing.T) {
	r := New()
	defer r.Close()
	if _, ok := r.Port().(net.Conn); !ok {
		t.Fatal("Port is not a net.Conn")
	}
}

// expectSilence asserts the radio puts nothing further on the wire. Silence,
// not a code, is what a frame that is not this radio's business earns: an NG
// addressed to a controller this radio has not heard from would be a frame no
// radio would send.
func expectSilence(t *testing.T, r *Radio) {
	t.Helper()
	c := r.Port()
	_ = c.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	defer c.SetReadDeadline(time.Time{})
	one := make([]byte, 1)
	n, err := c.Read(one)
	if n > 0 || err == nil {
		t.Fatalf("radio answered % X, want silence", one[:n])
	}
}

// TestAForeignControllerIsIgnored pins the whole B2/E0 address filter: the
// destination must be this radio AND the source must be the controller
// address the data-format diagram captions "Controller's (PC's) default
// address". A frame from any other source is echoed, because the echo is a
// property of the line rather than of the radio, and then dropped without an
// answer, a state change or a CommandLog entry.
func TestAForeignControllerIsIgnored(t *testing.T) {
	r := New(WithUSBEcho())
	defer r.Close()
	set := append([]byte{0x1A, 0x00, 0x00, 0x07}, record(0x40)...)
	foreign := frame(AddrRadio, 0xE1, set...)
	if _, err := r.Port().Write(foreign); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, r); !bytes.Equal(got, foreign) {
		t.Fatalf("echo = % X, want % X", got, foreign)
	}
	expectSilence(t, r)
	if _, ok := r.SlotState(7); ok {
		t.Fatal("a foreign controller's set reached a slot")
	}
	if log := r.CommandLog(); len(log) != 0 {
		t.Fatalf("command log = %v, want empty", log)
	}
	if _, err := r.Port().Write(wireTo(0x19, 0x00)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, r); !bytes.Equal(got, wireTo(0x19, 0x00)) {
		t.Fatalf("echo of the configured controller = % X", got)
	}
	if got := readFrame(t, r); len(got) < 5 || got[4] != 0x19 {
		t.Fatalf("identity reply = % X", got)
	}
}

// TestTheInboundAllFFRecordInterpretationIsOptional pins the modelled
// assumption ic7760-empty-reply-ff: that a stored record of FF bytes read
// back means "empty". The guide's only FF in the memory context is a value
// the CONTROLLER sends to erase, and nothing licenses reading it backwards,
// so the interpretation is a knob rather than a fact.
func TestTheInboundAllFFRecordInterpretationIsOptional(t *testing.T) {
	ff := make([]byte, RecordLen)
	for i := range ff {
		ff[i] = 0xFF
	}
	on := New()
	defer on.Close()
	on.SetSlot(7, MemState{Raw: ff})
	if _, err := on.Port().Write(wireTo(0x1A, 0x00, 0x00, 0x07)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, on); !bytes.Equal(got, wireFrom(0xFA)) {
		t.Fatalf("default all-FF read = % X, want NG", got)
	}
	off := New(WithEmptyReplyFF(false))
	defer off.Close()
	off.SetSlot(7, MemState{Raw: ff})
	if _, err := off.Port().Write(wireTo(0x1A, 0x00, 0x00, 0x07)); err != nil {
		t.Fatal(err)
	}
	want := wireFrom(append([]byte{0x1A, 0x00, 0x00, 0x07}, ff...)...)
	if got := readFrame(t, off); !bytes.Equal(got, want) {
		t.Fatalf("all-FF read with the interpretation off = % X, want the record back", got)
	}
}

// TestThePrintedClearFormIsRefusedIndependentlyOfShortSets keeps the two FF
// questions apart. Refusing the printed outbound clear form — a single FF in
// the ③ select-memory byte and nothing after it — is a deliberate divergence
// this fake makes always, and it does not turn on ic7760-empty-reply-ff or on
// ic7760-write-full-record.
func TestThePrintedClearFormIsRefusedIndependentlyOfShortSets(t *testing.T) {
	for _, opts := range [][]Option{nil, {WithWriteFullRecord(false)}, {WithEmptyReplyFF(false)}} {
		r := New(opts...)
		r.SetSlot(1, MemState{Raw: record(0x20)})
		if _, err := r.Port().Write(wireTo(0x1A, 0x00, 0x00, 0x01, 0xFF)); err != nil {
			t.Fatal(err)
		}
		if got := readFrame(t, r); !bytes.Equal(got, wireFrom(0xFA)) {
			t.Fatalf("clear form = % X, want NG", got)
		}
		if got, ok := r.Record(1); !ok || !bytes.Equal(got, record(0x20)) {
			t.Fatalf("the clear form emptied the channel: % X %v", got, ok)
		}
		r.Close()
	}
}

// TestFullRecordEnforcementIsTheRegisterEntry pins ic7760-write-full-record:
// the tier always sends the whole layout, and whether the radio insists on it
// is unprinted, so the enforcement is a knob whose default is to insist.
func TestFullRecordEnforcementIsTheRegisterEntry(t *testing.T) {
	short := append([]byte{0x1A, 0x00, 0x00, 0x01}, record(0x30)[:5]...)
	strict := New()
	defer strict.Close()
	if _, err := strict.Port().Write(wireTo(short...)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, strict); !bytes.Equal(got, wireFrom(0xFA)) {
		t.Fatalf("short set under enforcement = % X, want NG", got)
	}
	lax := New(WithWriteFullRecord(false))
	defer lax.Close()
	if _, err := lax.Port().Write(wireTo(short...)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, lax); !bytes.Equal(got, wireFrom(0xFB)) {
		t.Fatalf("short set with enforcement off = % X, want ACK", got)
	}
}

// TestTheScanEdgeRecordShapeIsSeparatelyConfigurable pins
// ic7760-scan-edge-record-shape: that a 1A 00 read of 01 00 or 01 01 returns
// the same 25-byte record-only shape as a memory channel is ASSUMED, so P1
// and P2 can be given a shape of their own.
func TestTheScanEdgeRecordShapeIsSeparatelyConfigurable(t *testing.T) {
	r := New(WithScanEdgeRecordShape(3))
	defer r.Close()
	edge := []byte{0x11, 0x22, 0x33}
	if _, err := r.Port().Write(wireTo(append([]byte{0x1A, 0x00, 0x01, 0x00}, edge...)...)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, r); !bytes.Equal(got, wireFrom(0xFB)) {
		t.Fatalf("three-byte P1 set = % X, want ACK", got)
	}
	if _, err := r.Port().Write(wireTo(0x1A, 0x00, 0x01, 0x00)); err != nil {
		t.Fatal(err)
	}
	want := wireFrom(append([]byte{0x1A, 0x00, 0x01, 0x00}, edge...)...)
	if got := readFrame(t, r); !bytes.Equal(got, want) {
		t.Fatalf("P1 read = % X, want % X", got, want)
	}
	if _, err := r.Port().Write(wireTo(append([]byte{0x1A, 0x00, 0x01, 0x01}, record(0x50)...)...)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, r); !bytes.Equal(got, wireFrom(0xFA)) {
		t.Fatalf("25-byte P2 set against a three-byte edge shape = % X, want NG", got)
	}
	if _, err := r.Port().Write(wireTo(append([]byte{0x1A, 0x00, 0x00, 0x01}, record(0x60)...)...)); err != nil {
		t.Fatal(err)
	}
	if got := readFrame(t, r); !bytes.Equal(got, wireFrom(0xFB)) {
		t.Fatalf("25-byte MEM set alongside a three-byte edge shape = % X, want ACK", got)
	}
}

// TestTheBroadcastFormIsConfigurable pins ic7760-broadcast-form: the guide
// prints no to=00 broadcast frame, so the address unsolicited frames carry is
// a knob whose default is the tier's assumption.
// An unsolicited frame comes FROM the radio, so its source is the radio's own
// address and only its destination varies: to=00 by assumption for a
// broadcast, to=E0 for the synthetic addressed flood.
func TestTheBroadcastFormIsConfigurable(t *testing.T) {
	def := New(WithTransceiveFlood(2 * time.Millisecond))
	defer def.Close()
	want := frame(AddrBroadcast, AddrRadio, 0x19, 0x00, 0xA5)
	if got := readFrame(t, def); !bytes.Equal(got, want) {
		t.Fatalf("default broadcast frame = % X, want % X", got, want)
	}
	r := New(WithBroadcastForm(0xEF), WithTransceiveFlood(2*time.Millisecond))
	defer r.Close()
	want = frame(0xEF, AddrRadio, 0x19, 0x00, 0xA5)
	if got := readFrame(t, r); !bytes.Equal(got, want) {
		t.Fatalf("broadcast frame = % X, want % X", got, want)
	}
	a := New(WithAddressedFlood(2 * time.Millisecond))
	defer a.Close()
	want = frame(AddrController, AddrRadio, 0x19, 0x00, 0xA5)
	if got := readFrame(t, a); !bytes.Equal(got, want) {
		t.Fatalf("addressed flood frame = % X, want % X", got, want)
	}
}

// TestEveryModelledAssumptionIsReachableUnderItsRegisterName is the naming
// pin for F10: every assumption this fake models is reached through an option
// named for the matrix register entry that owns it, so that a reader of the
// options list can find the entry and a reader of the register can find the
// knob.
func TestEveryModelledAssumptionIsReachableUnderItsRegisterName(t *testing.T) {
	tests := []struct {
		entry string
		opt   Option
		ok    func(config) bool
	}{
		{"ic7760-id-reply", WithIDReply([]byte{0x5A}), func(c config) bool { return bytes.Equal(c.id, []byte{0x5A}) }},
		{"ic7760-echo-default", WithEchoDefault(true), func(c config) bool { return c.echo }},
		{"ic7760-broadcast-form", WithBroadcastForm(0xEF), func(c config) bool { return c.broadcastTo == 0xEF }},
		{"ic7760-empty-reply-fa", WithEmptyReplyFA(CodeNG), func(c config) bool { return c.emptyReply == CodeNG }},
		{"ic7760-empty-reply-ff", WithEmptyReplyFF(false), func(c config) bool { return !c.emptyRecordFF }},
		{"ic7760-scan-edge-record-shape", WithScanEdgeRecordShape(3), func(c config) bool { return c.scanEdgeLen == 3 }},
		{"ic7760-write-full-record", WithWriteFullRecord(false), func(c config) bool { return !c.fullRecord }},
		{"ic7760-record-length", WithRecordLength(7), func(c config) bool { return c.recordLen == 7 }},
	}
	for _, tt := range tests {
		c := defaultConfig()
		tt.opt(&c)
		if !tt.ok(c) {
			t.Errorf("%s: its option did not reach the config", tt.entry)
		}
	}
	d := defaultConfig()
	if !d.emptyRecordFF || !d.fullRecord || d.broadcastTo != AddrBroadcast || d.scanEdgeLen != 0 {
		t.Errorf("defaults moved: %+v", d)
	}
}

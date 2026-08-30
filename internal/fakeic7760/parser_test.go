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

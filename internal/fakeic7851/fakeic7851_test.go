package fakeic7851

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestMemoryReadWriteAndDirection(t *testing.T) {
	r := New(WithModelName("IC-7850"))
	defer r.Close()
	rec := bytes.Repeat([]byte{0x11}, RecordLen)
	r.SetSlot("001", rec)
	if got := exchange(t, r.Port(), frame(0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01)); !bytes.Equal(got, answer(append([]byte{0x1a, 0x00, 0x00, 0x01}, rec...)...)) {
		t.Fatalf("read = % X", got)
	}
	if got := exchange(t, r.Port(), frame(0x8e, 0xe0, append([]byte{0x1a, 0x00, 0x00, 0x01}, rec...)...)); !bytes.Equal(got, answer(0xfb)) {
		t.Fatalf("set = % X", got)
	}
	_ = r.Port().SetDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := r.Port().Write(frame(0xe0, 0x8e, 0x19, 0x00)); err != nil {
		t.Fatal(err)
	}
	var ignored [64]byte
	if _, err := r.Port().Read(ignored[:]); err == nil {
		t.Fatal("wrong direction answered")
	}
}

func TestProtocolRefusalsAndEmptyModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []Option
		want []byte
	}{
		{"FA", []Option{WithEmptyReplyFA()}, answer(0xfa)},
		{"all FF", []Option{WithAllFFEmpty()}, answer(append([]byte{0x1a, 0, 0, 1}, bytes.Repeat([]byte{0xff}, RecordLen)...)...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New(tc.opts...)
			defer r.Close()
			got := exchange(t, r.Port(), frame(0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01))
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("got len=%d % X want len=%d % X", len(got), got, len(tc.want), tc.want)
			}
		})
	}
	r := New()
	defer r.Close()
	for _, req := range [][]byte{
		frame(0x8e, 0xe0, 0x1a, 0x05), frame(0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01, 0xff),
	} {
		if got := exchange(t, r.Port(), req); !bytes.Equal(got, answer(0xfa)) {
			t.Fatalf("refusal = % X", got)
		}
	}
}

func TestEchoAndFloodAreConfigurable(t *testing.T) {
	r := New(WithUSBEcho(), WithTransceiveFlood(2*time.Millisecond))
	defer r.Close()
	req := frame(0x8e, 0xe0, 0x19, 0x00)
	if got := exchange(t, r.Port(), req); !bytes.Equal(got, req) {
		t.Fatalf("echo = % X", got)
	}
}

func frame(to, from byte, payload ...byte) []byte {
	return append([]byte{0xfe, 0xfe, to, from}, append(payload, 0xfd)...)
}
func answer(payload ...byte) []byte { return frame(0xe0, 0x8e, payload...) }
func exchange(t *testing.T, c net.Conn, req []byte) []byte {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	if _, err := c.Write(req); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4096)
	n, err := c.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte(nil), b[:n]...)
}

// TestScanEdgeSelectorsAddressP1AndP2 pins the complete flat selector space
// printed in B row D1 (PDF p.263): 0001-0099 are memories 1-99, 0100 is
// programmed scan edge P1 and 0101 is P2, all as two packed-BCD bytes.
func TestScanEdgeSelectorsAddressP1AndP2(t *testing.T) {
	r := New()
	defer r.Close()
	p1 := bytes.Repeat([]byte{0x21}, RecordLen)
	p2 := bytes.Repeat([]byte{0x22}, RecordLen)
	last := bytes.Repeat([]byte{0x99}, RecordLen)
	r.SetSlot("P1", p1)
	r.SetSlot("P2", p2)
	r.SetSlot("099", last)
	for _, tc := range []struct {
		name   string
		sel    []byte
		record []byte
	}{
		{"099", []byte{0x00, 0x99}, last},
		{"P1", []byte{0x01, 0x00}, p1},
		{"P2", []byte{0x01, 0x01}, p2},
	} {
		t.Run("read "+tc.name, func(t *testing.T) {
			got := exchange(t, r.Port(), frame(0x8e, 0xe0, 0x1a, 0x00, tc.sel[0], tc.sel[1]))
			want := answer(append([]byte{0x1a, 0x00, tc.sel[0], tc.sel[1]}, tc.record...)...)
			if !bytes.Equal(got, want) {
				t.Fatalf("read %s = % X, want % X", tc.name, got, want)
			}
		})
	}
	// A set addressed to P2 must land where SetSlot("P2") and SlotState("P2")
	// look, or the fake's two halves disagree about the same channel.
	written := bytes.Repeat([]byte{0x33}, RecordLen)
	if got := exchange(t, r.Port(), frame(0x8e, 0xe0, append([]byte{0x1a, 0x00, 0x01, 0x01}, written...)...)); !bytes.Equal(got, answer(0xfb)) {
		t.Fatalf("set P2 = % X, want % X", got, answer(0xfb))
	}
	st, ok := r.SlotState("P2")
	if !ok || !bytes.Equal(st.Raw, written) {
		t.Fatalf("SlotState(P2) = %v % X, want true % X", ok, st.Raw, written)
	}
}

// TestSelectorsOutsideTheFlatSpaceAreRefused keeps 0102 and non-BCD nibbles
// outside the 1-101 space the manual prints.
func TestSelectorsOutsideTheFlatSpaceAreRefused(t *testing.T) {
	r := New()
	defer r.Close()
	for _, sel := range [][]byte{
		{0x00, 0x00}, // 0000: below the space
		{0x01, 0x02}, // 0102: one past P2
		{0x02, 0x00}, // 0200: far past P2
		{0x00, 0x9a}, // non-BCD low nibble
		{0x00, 0xa0}, // non-BCD high nibble
		{0x10, 0x00}, // non-zero thousands digit
	} {
		if got := exchange(t, r.Port(), frame(0x8e, 0xe0, 0x1a, 0x00, sel[0], sel[1])); !bytes.Equal(got, answer(0xfa)) {
			t.Fatalf("selector % X = % X, want % X", sel, got, answer(0xfa))
		}
	}
}

// TestOnlyTheControllerIsAnswered pins the printed frame convention: this
// radio answers the controller at 0xE0 and stays silent to anything else.
func TestOnlyTheControllerIsAnswered(t *testing.T) {
	r := New()
	defer r.Close()
	for _, from := range []byte{0x00, 0x8e, 0xe1, 0x0e} {
		if got, err := exchangeSilent(t, r.Port(), frame(0x8e, from, 0x19, 0x00)); err == nil {
			t.Fatalf("source %02X answered with % X", from, got)
		}
	}
	// The link is not wedged: the controller is still served afterwards.
	want := answer(append([]byte{0x19, 0x00}, []byte("IC-7851")...)...)
	if got := exchange(t, r.Port(), frame(0x8e, 0xe0, 0x19, 0x00)); !bytes.Equal(got, want) {
		t.Fatalf("after ignored sources: % X, want % X", got, want)
	}
}

// TestMovedRadioAddressFramesEveryReply pins that WithRadioAddress moves both
// the frame the radio listens for and the source byte of every reply.
func TestMovedRadioAddressFramesEveryReply(t *testing.T) {
	const moved = 0x1c
	r := New(WithRadioAddress(moved), WithModelName("IC-7850"))
	defer r.Close()
	rec := bytes.Repeat([]byte{0x44}, RecordLen)
	r.SetSlot("P1", rec)
	for _, tc := range []struct {
		name string
		req  []byte
		want []byte
	}{
		{"model", frame(moved, 0xe0, 0x19, 0x00), frame(0xe0, moved, append([]byte{0x19, 0x00}, []byte("IC-7850")...)...)},
		{"read P1", frame(moved, 0xe0, 0x1a, 0x00, 0x01, 0x00), frame(0xe0, moved, append([]byte{0x1a, 0x00, 0x01, 0x00}, rec...)...)},
		{"refusal", frame(moved, 0xe0, 0x1a, 0x05), frame(0xe0, moved, 0xfa)},
	} {
		if got := exchange(t, r.Port(), tc.req); !bytes.Equal(got, tc.want) {
			t.Fatalf("%s = % X, want % X", tc.name, got, tc.want)
		}
	}
	// The factory address is no longer this radio's.
	if got, err := exchangeSilent(t, r.Port(), frame(0x8e, 0xe0, 0x19, 0x00)); err == nil {
		t.Fatalf("old address answered with % X", got)
	}
}

// TestModelNameDiagnosticsAreVerbatim pins the 19 00 token the driver records
// but never matches.
func TestModelNameDiagnosticsAreVerbatim(t *testing.T) {
	for _, name := range []string{"IC-7851", "IC-7850", ""} {
		r := New(WithModelName(name))
		want := answer(append([]byte{0x19, 0x00}, []byte(name)...)...)
		if got := exchange(t, r.Port(), frame(0x8e, 0xe0, 0x19, 0x00)); !bytes.Equal(got, want) {
			r.Close()
			t.Fatalf("19 00 for %q = % X, want % X", name, got, want)
		}
		r.Close()
	}
}

// TestMalformedLengthsAreRefused keeps the single-length discriminator: only a
// 25-byte record is a set, and a truncated selector is not a read.
func TestMalformedLengthsAreRefused(t *testing.T) {
	r := New()
	defer r.Close()
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"one selector byte", []byte{0x1a, 0x00, 0x00}},
		{"no selector", []byte{0x1a, 0x00}},
		{"bare 1A", []byte{0x1a}},
		{"24-byte record", append([]byte{0x1a, 0x00, 0x00, 0x01}, bytes.Repeat([]byte{0x11}, RecordLen-1)...)},
		{"26-byte record", append([]byte{0x1a, 0x00, 0x00, 0x01}, bytes.Repeat([]byte{0x11}, RecordLen+1)...)},
		{"erase form", []byte{0x1a, 0x00, 0x01, 0x00, 0xff}},
	} {
		if got := exchange(t, r.Port(), frame(0x8e, 0xe0, tc.payload...)); !bytes.Equal(got, answer(0xfa)) {
			t.Fatalf("%s = % X, want % X", tc.name, got, answer(0xfa))
		}
	}
	if _, ok := r.SlotState("P1"); ok {
		t.Fatal("a refused set wrote a record")
	}
}

// TestCloseUnderFloodReturns pins clean shutdown while both flood goroutines
// are mid-write against an unread port.
func TestCloseUnderFloodReturns(t *testing.T) {
	r := New(WithTransceiveFlood(time.Millisecond), WithAddressedFlood(time.Millisecond), WithUSBEcho())
	time.Sleep(20 * time.Millisecond)
	closed := make(chan error, 1)
	go func() { closed <- r.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked under flood")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close = %v", err)
	}
}

func exchangeSilent(t *testing.T, c net.Conn, req []byte) ([]byte, error) {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(150 * time.Millisecond))
	if _, err := c.Write(req); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4096)
	n, err := c.Read(b)
	return append([]byte(nil), b[:n]...), err
}

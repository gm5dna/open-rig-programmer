package fakeic9100

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestMemoryReadWriteAndDirection(t *testing.T) {
	r := New(WithModelName("IC-9100"))
	defer r.Close()
	rec := bytes.Repeat([]byte{0x11}, RecordLen)
	r.SetSlot("HF-001", rec)
	if got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, 0x1a, 0x00, 0x00, 0x00, 0x01)); !bytes.Equal(got, answer(append([]byte{0x1a, 0x00, 0x00, 0x00, 0x01}, rec...)...)) {
		t.Fatalf("read = % X", got)
	}
	if got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, append([]byte{0x1a, 0x00, 0x00, 0x00, 0x01}, rec...)...)); !bytes.Equal(got, answer(0xfb)) {
		t.Fatalf("set = % X", got)
	}
	_ = r.Port().SetDeadline(time.Now().Add(100 * time.Millisecond))
	if _, err := r.Port().Write(frame(controllerAddr, radioAddrDefault, 0x19, 0x00)); err != nil {
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
		{"all FF", []Option{WithAllFFEmpty()}, answer(append([]byte{0x1a, 0, 0x00, 0x00, 0x01}, bytes.Repeat([]byte{0xff}, RecordLen)...)...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New(tc.opts...)
			defer r.Close()
			got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, 0x1a, 0x00, 0x00, 0x00, 0x01))
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("got len=%d % X want len=%d % X", len(got), got, len(tc.want), tc.want)
			}
		})
	}
	r := New()
	defer r.Close()
	for _, req := range [][]byte{
		frame(radioAddrDefault, controllerAddr, 0x1a, 0x05),
		frame(radioAddrDefault, controllerAddr, 0x1a, 0x00, 0x00, 0x00, 0x01, 0xff),
		frame(radioAddrDefault, controllerAddr, 0x0b),
	} {
		if got := exchange(t, r.Port(), req); !bytes.Equal(got, answer(0xfa)) {
			t.Fatalf("refusal = % X", got)
		}
	}
}

func TestEchoAndFloodAreConfigurable(t *testing.T) {
	r := New(WithUSBEcho(), WithTransceiveFlood(2*time.Millisecond))
	defer r.Close()
	req := frame(radioAddrDefault, controllerAddr, 0x19, 0x00)
	if got := exchange(t, r.Port(), req); !bytes.Equal(got, req) {
		t.Fatalf("echo = % X", got)
	}
}

func frame(to, from byte, payload ...byte) []byte {
	return append([]byte{0xfe, 0xfe, to, from}, append(payload, 0xfd)...)
}
func answer(payload ...byte) []byte { return frame(controllerAddr, radioAddrDefault, payload...) }
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

// TestBandsAreIndependentChannelSpaces pins the band byte as the top of the
// selector: the same channel number in a different band is a different
// record, and SetSlot/SlotState and a wire read agree about which is which
// (matrix §1b, "Banks"; doc.go's band-byte ruling).
func TestBandsAreIndependentChannelSpaces(t *testing.T) {
	r := New()
	defer r.Close()
	hf := bytes.Repeat([]byte{0x21}, RecordLen)
	vhf := bytes.Repeat([]byte{0x22}, RecordLen)
	uhf := bytes.Repeat([]byte{0x23}, RecordLen)
	uhf1200 := bytes.Repeat([]byte{0x24}, RecordLen)
	r.SetSlot("HF-001", hf)
	r.SetSlot("144-001", vhf)
	r.SetSlot("430-001", uhf)
	r.SetSlot("1200-001", uhf1200)
	for _, tc := range []struct {
		name   string
		sel    []byte
		record []byte
	}{
		{"HF-001", []byte{bandHF, 0x00, 0x01}, hf},
		{"144-001", []byte{band144, 0x00, 0x01}, vhf},
		{"430-001", []byte{band430, 0x00, 0x01}, uhf},
		{"1200-001", []byte{band1200, 0x00, 0x01}, uhf1200},
	} {
		t.Run("read "+tc.name, func(t *testing.T) {
			got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, append([]byte{0x1a, 0x00}, tc.sel...)...))
			want := answer(append(append([]byte{0x1a, 0x00}, tc.sel...), tc.record...)...)
			if !bytes.Equal(got, want) {
				t.Fatalf("read %s = % X, want % X", tc.name, got, want)
			}
		})
	}
	// A set addressed to 430-001 must land where SetSlot/SlotState look for
	// it, or the fake's two halves disagree about the same channel.
	written := bytes.Repeat([]byte{0x33}, RecordLen)
	if got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, append([]byte{0x1a, 0x00, band430, 0x00, 0x01}, written...)...)); !bytes.Equal(got, answer(0xfb)) {
		t.Fatalf("set 430-001 = % X, want % X", got, answer(0xfb))
	}
	st, ok := r.SlotState("430-001")
	if !ok || !bytes.Equal(st.Raw, written) {
		t.Fatalf("SlotState(430-001) = %v % X, want true % X", ok, st.Raw, written)
	}
	// Untouched bands keep their own records.
	st, ok = r.SlotState("HF-001")
	if !ok || !bytes.Equal(st.Raw, hf) {
		t.Fatalf("SlotState(HF-001) = %v % X, want true % X", ok, st.Raw, hf)
	}
}

// TestSelectorsOutsideTheFlatSpaceAreRefused keeps band values past 1200
// MHz, non-BCD channel nibbles, and the scan-edge/call-channel codes
// (0100-0106, out of scope this cycle per the wave spec) outside the space
// this fake serves.
func TestSelectorsOutsideTheFlatSpaceAreRefused(t *testing.T) {
	r := New()
	defer r.Close()
	for _, sel := range [][]byte{
		{0x04, 0x00, 0x01}, // band 04: past the printed 00-03 range
		{0x00, 0x00, 0x00}, // channel 0000: below the space
		{0x00, 0x01, 0x00}, // channel 0100: programmed scan edge, out of scope
		{0x00, 0x01, 0x06}, // channel 0106: call channel, out of scope
		{0x00, 0x00, 0x9a}, // non-BCD low nibble
		{0x00, 0x00, 0xa0}, // non-BCD high nibble
	} {
		if got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, append([]byte{0x1a, 0x00}, sel...)...)); !bytes.Equal(got, answer(0xfa)) {
			t.Fatalf("selector % X = % X, want % X", sel, got, answer(0xfa))
		}
	}
}

// TestOnlyTheControllerIsAnswered pins the printed frame convention: this
// radio answers the controller at 0xE0 and stays silent to anything else.
func TestOnlyTheControllerIsAnswered(t *testing.T) {
	r := New()
	defer r.Close()
	for _, from := range []byte{0x00, radioAddrDefault, 0xe1, 0x0e} {
		if got, err := exchangeSilent(t, r.Port(), frame(radioAddrDefault, from, 0x19, 0x00)); err == nil {
			t.Fatalf("source %02X answered with % X", from, got)
		}
	}
	// The link is not wedged: the controller is still served afterwards.
	want := answer(append([]byte{0x19, 0x00}, []byte("IC-9100")...)...)
	if got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, 0x19, 0x00)); !bytes.Equal(got, want) {
		t.Fatalf("after ignored sources: % X, want % X", got, want)
	}
}

// TestMovedRadioAddressFramesEveryReply pins that WithRadioAddress moves
// both the frame the radio listens for and the source byte of every reply.
func TestMovedRadioAddressFramesEveryReply(t *testing.T) {
	const moved = 0x1c
	r := New(WithRadioAddress(moved), WithModelName("IC-9100"))
	defer r.Close()
	rec := bytes.Repeat([]byte{0x44}, RecordLen)
	r.SetSlot("HF-001", rec)
	for _, tc := range []struct {
		name string
		req  []byte
		want []byte
	}{
		{"model", frame(moved, controllerAddr, 0x19, 0x00), frame(controllerAddr, moved, append([]byte{0x19, 0x00}, []byte("IC-9100")...)...)},
		{"read HF-001", frame(moved, controllerAddr, 0x1a, 0x00, 0x00, 0x00, 0x01), frame(controllerAddr, moved, append([]byte{0x1a, 0x00, 0x00, 0x00, 0x01}, rec...)...)},
		{"refusal", frame(moved, controllerAddr, 0x1a, 0x05), frame(controllerAddr, moved, 0xfa)},
	} {
		if got := exchange(t, r.Port(), tc.req); !bytes.Equal(got, tc.want) {
			t.Fatalf("%s = % X, want % X", tc.name, got, tc.want)
		}
	}
	// The factory address is no longer this radio's.
	if got, err := exchangeSilent(t, r.Port(), frame(radioAddrDefault, controllerAddr, 0x19, 0x00)); err == nil {
		t.Fatalf("old address answered with % X", got)
	}
}

// TestModelNameDiagnosticsAreVerbatim pins the 19 00 token the driver
// records but never matches.
func TestModelNameDiagnosticsAreVerbatim(t *testing.T) {
	for _, name := range []string{"IC-9100", ""} {
		r := New(WithModelName(name))
		want := answer(append([]byte{0x19, 0x00}, []byte(name)...)...)
		if got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, 0x19, 0x00)); !bytes.Equal(got, want) {
			r.Close()
			t.Fatalf("19 00 for %q = % X, want % X", name, got, want)
		}
		r.Close()
	}
}

// TestMalformedLengthsAreRefused keeps the single-length discriminator:
// only a 57-byte record is a set, and a truncated selector is not a read.
func TestMalformedLengthsAreRefused(t *testing.T) {
	r := New()
	defer r.Close()
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{"two selector bytes", []byte{0x1a, 0x00, 0x00, 0x00}},
		{"no selector", []byte{0x1a, 0x00}},
		{"bare 1A", []byte{0x1a}},
		{"56-byte record", append([]byte{0x1a, 0x00, 0x00, 0x00, 0x01}, bytes.Repeat([]byte{0x11}, RecordLen-1)...)},
		{"58-byte record", append([]byte{0x1a, 0x00, 0x00, 0x00, 0x01}, bytes.Repeat([]byte{0x11}, RecordLen+1)...)},
		{"erase form", []byte{0x1a, 0x00, 0x00, 0x00, 0x01, 0xff}},
	} {
		if got := exchange(t, r.Port(), frame(radioAddrDefault, controllerAddr, tc.payload...)); !bytes.Equal(got, answer(0xfa)) {
			t.Fatalf("%s = % X, want % X", tc.name, got, answer(0xfa))
		}
	}
	if _, ok := r.SlotState("HF-001"); ok {
		t.Fatal("a refused set wrote a record")
	}
}

// TestCloseUnderFloodReturns pins clean shutdown while both flood
// goroutines are mid-write against an unread port.
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

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

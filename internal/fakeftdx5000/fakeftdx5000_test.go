// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx5000

import (
	"bufio"
	"io"
	"net"
	"testing"
	"time"
)

// hostConn opens the fake's host end as a net.Conn, so tests can use
// SetReadDeadline directly rather than a leaked background goroutine racing
// a later read for the same bytes.
func hostConn(t *testing.T, r *Radio) net.Conn {
	t.Helper()
	conn, ok := r.Port().(net.Conn)
	if !ok {
		t.Fatal("Radio.Port() does not implement net.Conn")
	}
	return conn
}

// readFrame reads bytes up to and including the next ';', with a deadline
// so a test hangs loudly instead of silently on a fake that stays quiet.
func readFrame(t *testing.T, conn net.Conn, r *bufio.Reader) string {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	defer conn.SetReadDeadline(time.Time{})
	s, err := r.ReadString(';')
	if err != nil {
		t.Fatalf("ReadString: %v", err)
	}
	return s
}

// expectSilence asserts nothing arrives within a short deadline — used to
// prove MW's documented no-answer behaviour.
func expectSilence(t *testing.T, conn net.Conn, r *bufio.Reader) {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	defer conn.SetReadDeadline(time.Time{})
	b, err := r.ReadByte()
	if err == nil {
		t.Fatalf("expected silence, got byte %q", b)
	}
}

func TestID_AnswersTheFixedCATID(t *testing.T) {
	radio := New()
	defer radio.Close()
	conn := hostConn(t, radio)
	r := bufio.NewReader(conn)

	if _, err := conn.Write([]byte("ID;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got, want := readFrame(t, conn, r), "ID0362;"; got != want {
		t.Errorf("ID; answer = %q, want %q", got, want)
	}
}

func TestMR_UnwrittenSlotAnswersTheZeroRecord(t *testing.T) {
	radio := New()
	defer radio.Close()
	conn := hostConn(t, radio)
	r := bufio.NewReader(conn)

	if _, err := conn.Write([]byte("MR001;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := "MR00100000000+000000110000;"
	if got := readFrame(t, conn, r); got != want {
		t.Errorf("MR001; answer = %q, want %q", got, want)
	}
}

func TestMW_ThenMR_RoundTrips(t *testing.T) {
	radio := New()
	defer radio.Close()
	conn := hostConn(t, radio)
	r := bufio.NewReader(conn)

	// P1=100 P2=14250000 P3=+0010 P4=1 P5=0 P6=2(USB) P7=0(fixed) P8=1
	// P9=05 P10=1
	set := "MW10014250000+001010201051;"
	if _, err := conn.Write([]byte(set)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// MW never answers (ANS "X") — prove silence before reading via MR.
	expectSilence(t, conn, r)

	if _, err := conn.Write([]byte("MR100;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	// Same fields, but P7 (Kind) in an MR answer is always '1' (Memory),
	// not MW's written '0' — see doc.go.
	want := "MR10014250000+001010211051;"
	if got := readFrame(t, conn, r); got != want {
		t.Errorf("MR100; answer = %q, want %q", got, want)
	}

	s, ok := radio.SlotState("100")
	if !ok {
		t.Fatal("SlotState(100) reports unwritten after a valid MW")
	}
	if s.FreqHz != 14250000 || s.Mode != '2' || s.ToneIndex != 5 {
		t.Errorf("SlotState(100) = %+v, want FreqHz 14250000, Mode '2', ToneIndex 5", s)
	}
}

func TestMW_OutOfRangeSlotIsDiscardedNotStored(t *testing.T) {
	radio := New()
	defer radio.Close()
	conn := hostConn(t, radio)
	r := bufio.NewReader(conn)

	// Slot 118 is one past the documented 001-117 range.
	if _, err := conn.Write([]byte("MW11814250000+001010201051;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	expectSilence(t, conn, r)

	if _, ok := radio.SlotState("118"); ok {
		t.Error("SlotState(118) reports written — an out-of-range MW must be discarded")
	}
}

func TestMR_MalformedRequestIsSilent(t *testing.T) {
	radio := New()
	defer radio.Close()
	conn := hostConn(t, radio)
	r := bufio.NewReader(conn)

	if _, err := conn.Write([]byte("MR;")); err != nil { // no slot digits at all
		t.Fatalf("Write: %v", err)
	}
	expectSilence(t, conn, r)
}

func TestUnrecognisedOpcodeIsSilent(t *testing.T) {
	radio := New()
	defer radio.Close()
	conn := hostConn(t, radio)
	r := bufio.NewReader(conn)

	if _, err := conn.Write([]byte("FA14250000;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	expectSilence(t, conn, r)
}

func TestWithSlot_OverridesTheZeroRecordDefault(t *testing.T) {
	seed := MemState{FreqHz: 7100000, ClarSign: '-', Mode: '3', CTCSSState: '1', ToneIndex: 12, Shift: '2'}
	radio := New(WithSlot("050", seed))
	defer radio.Close()
	conn := hostConn(t, radio)
	r := bufio.NewReader(conn)

	if _, err := conn.Write([]byte("MR050;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := buildMRAnswer("050", seed)
	if got := readFrame(t, conn, r); got != string(want) {
		t.Errorf("MR050; answer = %q, want %q", got, want)
	}
}

func TestClose_IsPromptDespiteLatency(t *testing.T) {
	radio := New(WithLatency(5 * time.Second))
	conn := hostConn(t, radio)

	if _, err := conn.Write([]byte("ID;")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- radio.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Close did not return promptly despite a pending latency wait")
	}

	// The host end should now see EOF rather than hang forever.
	buf := make([]byte, 1)
	_, err := conn.Read(buf)
	if err != io.EOF {
		t.Errorf("expected the host end to observe EOF after Close, got %v", err)
	}
}

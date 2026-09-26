// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// testProfile builds a synthetic fixture Profile for this package's own
// tests — no real radio's values (Phase 2's job).
func testProfile(schedule []Block, imageLen int) Profile {
	return Profile{
		Model:              "TEST",
		ProfileID:          "test-1",
		ImageLen:           imageLen,
		BlockSchedule:      schedule,
		StartDeadline:      time.Second,
		InterBlockDeadline: time.Second,
		TotalDeadline:      2 * time.Second,
	}
}

// xorChecksum is a synthetic per-block checksum for tests: the XOR of
// every byte but the last must equal the last byte. No real radio in this
// family is known to use this algorithm — Phase 2's job, informed-by
// CHIRP, per family.
func xorChecksum(block []byte) bool {
	if len(block) == 0 {
		return false
	}
	var sum byte
	for _, b := range block[:len(block)-1] {
		sum ^= b
	}
	return sum == block[len(block)-1]
}

// countingPort wraps a net.Conn so a test can assert Arm never touches the
// port at all.
type countingPort struct {
	net.Conn
	mu    sync.Mutex
	reads int
}

func (c *countingPort) Read(p []byte) (int, error) {
	c.mu.Lock()
	c.reads++
	c.mu.Unlock()
	return c.Conn.Read(p)
}

func (c *countingPort) readCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads
}

func TestArmSignalsBeforeAnyByteRead(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	cp := &countingPort{Conn: pcConn}
	p := testProfile([]Block{{Len: 2}}, 2)

	r, err := Arm(context.Background(), cp, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	select {
	case <-r.Armed():
	default:
		t.Fatal("Armed() was not already signalled once Arm returned")
	}
	if n := cp.readCount(); n != 0 {
		t.Fatalf("Arm read from the port %d time(s), want 0", n)
	}
}

func TestArmRejectsEmptyCandidateSet(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	if _, err := Arm(context.Background(), pcConn, nil); err == nil {
		t.Fatal("Arm with no candidates: want an error, got nil")
	}
}

func TestReceiveStartDeadline(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	p := testProfile([]Block{{Len: 4}}, 4)
	p.StartDeadline = 30 * time.Millisecond

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}
	if _, err := r.Receive(context.Background()); !errors.Is(err, ErrImageIncomplete) {
		t.Fatalf("err = %v, want ErrImageIncomplete", err)
	}
}

func TestReceiveInterBlockDeadline(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	p := testProfile([]Block{{Len: 2}, {Len: 2}}, 4)
	p.InterBlockDeadline = 30 * time.Millisecond

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Receive(context.Background())
		errCh <- err
	}()

	if _, err := radioConn.Write([]byte{1, 2}); err != nil {
		t.Fatalf("radio write: %v", err)
	}
	// The second block never arrives: the inter-block deadline must fire.
	select {
	case err := <-errCh:
		if !errors.Is(err, ErrImageIncomplete) {
			t.Fatalf("err = %v, want ErrImageIncomplete", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not return after the inter-block deadline")
	}
}

func TestReceiveTotalDeadline(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	p := testProfile([]Block{{Len: 1}, {Len: 1}, {Len: 1}, {Len: 1}}, 4)
	p.TotalDeadline = 50 * time.Millisecond

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Receive(context.Background())
		errCh <- err
	}()

	// Trickle bytes well within each inter-block gap, but slower overall
	// than the total deadline, so only TotalDeadline can be what fires.
	go func() {
		for i := 0; i < 4; i++ {
			time.Sleep(30 * time.Millisecond)
			radioConn.Write([]byte{byte(i)})
		}
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrImageIncomplete) {
			t.Fatalf("err = %v, want ErrImageIncomplete", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not return after the total deadline")
	}
}

func TestReceiveChecksumFailureRefusesWhole(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	p := testProfile([]Block{{Len: 3, Checksum: xorChecksum}}, 3)

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Receive(context.Background())
		errCh <- err
	}()

	// 0x01 ^ 0x02 = 0x03, not the 0x99 sent: deliberately wrong checksum.
	if _, err := radioConn.Write([]byte{0x01, 0x02, 0x99}); err != nil {
		t.Fatalf("radio write: %v", err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrImageIncomplete) {
			t.Fatalf("err = %v, want ErrImageIncomplete", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not return")
	}
}

func TestReceiveShortImageRefusesWhole(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()

	p := testProfile([]Block{{Len: 4}}, 4)

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Receive(context.Background())
		errCh <- err
	}()

	if _, err := radioConn.Write([]byte{1, 2}); err != nil {
		t.Fatalf("radio write: %v", err)
	}
	radioConn.Close() // the radio hangs up mid-block

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrImageIncomplete) {
			t.Fatalf("err = %v, want ErrImageIncomplete", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not return")
	}
}

func TestReceiveTrailingBytesRefusesWhole(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	p := testProfile([]Block{{Len: 2}}, 2)

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Receive(context.Background())
		errCh <- err
	}()

	// One byte beyond the schedule's only block.
	if _, err := radioConn.Write([]byte{1, 2, 3}); err != nil {
		t.Fatalf("radio write: %v", err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrImageIncomplete) {
			t.Fatalf("err = %v, want ErrImageIncomplete", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not return")
	}
}

func TestReceiveSuccessSendsAckAtBlockBoundaries(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	p := testProfile([]Block{
		{Len: 2, Ack: true},
		{Len: 3, Ack: true},
	}, 5)
	p.AckExpected = true

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	type result struct {
		img Image
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		img, err := r.Receive(context.Background())
		resCh <- result{img, err}
	}()

	ackBuf := make([]byte, 1)
	radioConn.SetReadDeadline(time.Now().Add(2 * time.Second))

	if _, err := radioConn.Write([]byte{1, 2}); err != nil {
		t.Fatalf("write block 1: %v", err)
	}
	if n, err := radioConn.Read(ackBuf); err != nil || n != 1 || ackBuf[0] != ackByte {
		t.Fatalf("ACK after block 1: n=%d err=%v byte=%v", n, err, ackBuf)
	}

	if _, err := radioConn.Write([]byte{3, 4, 5}); err != nil {
		t.Fatalf("write block 2: %v", err)
	}
	if n, err := radioConn.Read(ackBuf); err != nil || n != 1 || ackBuf[0] != ackByte {
		t.Fatalf("ACK after block 2: n=%d err=%v byte=%v", n, err, ackBuf)
	}

	select {
	case res := <-resCh:
		if res.err != nil {
			t.Fatalf("Receive: %v", res.err)
		}
		if string(res.img.Raw) != string([]byte{1, 2, 3, 4, 5}) {
			t.Fatalf("Raw = %v, want {1 2 3 4 5}", res.img.Raw)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not complete")
	}
}

func TestReceiveNoAckWhenAckExpectedFalse(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	// Ack is true at the block level, but the Profile's master switch is
	// off: no byte must ever go outbound (spec.md Decisions item 5).
	p := testProfile([]Block{{Len: 2, Ack: true}}, 2)
	p.AckExpected = false

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Receive(context.Background())
		errCh <- err
	}()

	if _, err := radioConn.Write([]byte{1, 2}); err != nil {
		t.Fatalf("radio write: %v", err)
	}

	buf := make([]byte, 1)
	radioConn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if n, err := radioConn.Read(buf); err == nil {
		t.Fatalf("unexpected byte %v arrived outbound when AckExpected was false", buf[:n])
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Receive: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not complete")
	}
}

func TestReceiveIncompatibleImage(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	p := testProfile([]Block{{Len: 4}}, 4)
	p.ImageLen = 999 // deliberately does not match what the schedule reads

	r, err := Arm(context.Background(), pcConn, []Profile{p})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Receive(context.Background())
		errCh <- err
	}()

	if _, err := radioConn.Write([]byte{1, 2, 3, 4}); err != nil {
		t.Fatalf("radio write: %v", err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrImageIncompatible) {
			t.Fatalf("err = %v, want ErrImageIncompatible", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not return")
	}
}

func TestReceiveAmbiguousImage(t *testing.T) {
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	p1 := testProfile([]Block{{Len: 4}}, 4)
	p2 := p1
	p2.Model = "TEST2"
	p2.ProfileID = "test-2"
	// Same length as p1: genuinely ambiguous.

	r, err := Arm(context.Background(), pcConn, []Profile{p1, p2})
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := r.Receive(context.Background())
		errCh <- err
	}()

	if _, err := radioConn.Write([]byte{1, 2, 3, 4}); err != nil {
		t.Fatalf("radio write: %v", err)
	}

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrImageAmbiguous) {
			t.Fatalf("err = %v, want ErrImageAmbiguous", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Receive did not return")
	}
}

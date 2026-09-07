// SPDX-License-Identifier: GPL-3.0-or-later

package fakepipe

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// TestEcho is the whole of the plumbing in one exchange: a host write reaches
// ReadLoop, Write's reply reaches the host, and Close ends the goroutine and
// leaves the host at EOF.
func TestEcho(t *testing.T) {
	p := New()
	p.Go(func() { p.ReadLoop(func(b []byte) { p.Write(append([]byte(nil), b...)) }) })

	if _, err := p.Host().Write([]byte("ID;")); err != nil {
		t.Fatalf("host write: %v", err)
	}
	got := make([]byte, 3)
	if _, err := io.ReadFull(p.Host(), got); err != nil {
		t.Fatalf("host read: %v", err)
	}
	if !bytes.Equal(got, []byte("ID;")) {
		t.Errorf("read %q, want %q", got, "ID;")
	}

	if err := p.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := p.Host().Read(got); err != io.EOF {
		t.Errorf("host read after Close = %v, want io.EOF — Close must shut only the radio's end", err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestClose_IsPromptDespiteAPendingLatency is the property internal/wiring
// relies on for every fake rig: a scripted latency must not be waited out.
func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	p := New()
	p.Latency = time.Hour
	p.Go(func() { p.ReadLoop(func([]byte) { p.Write([]byte("OK")) }) })

	if _, err := p.Host().Write([]byte("x")); err != nil {
		t.Fatalf("host write: %v", err)
	}
	// Let the read loop reach the latency wait; the wait itself is what is
	// being interrupted, so it does not matter if we arrive a touch early.
	time.Sleep(10 * time.Millisecond)

	done := make(chan struct{})
	go func() { defer close(done); _ = p.Close() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close sat out the pending latency")
	}
}

// TestSleep reports elapsed for a short wait and interrupted after Shutdown.
func TestSleep(t *testing.T) {
	p := New()
	if !p.Sleep(0) {
		t.Error("Sleep(0) = false, want true")
	}
	if !p.Sleep(time.Millisecond) {
		t.Error("Sleep(1ms) = false, want true")
	}
	if err := p.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if p.Sleep(time.Hour) {
		t.Error("Sleep after Shutdown = true, want false")
	}
	if p.Write([]byte("x")) {
		t.Error("Write after Shutdown = true, want false")
	}
}

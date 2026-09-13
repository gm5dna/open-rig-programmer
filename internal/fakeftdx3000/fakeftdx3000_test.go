// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx3000

import (
	"bytes"
	"testing"
	"time"
)

func TestNew_AnswersID0462(t *testing.T) {
	r := New()
	defer r.Close()
	if r.CATID() != "0462" {
		t.Errorf("CATID() = %q, want %q", r.CATID(), "0462")
	}
}

func TestPort_RoundTrip(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()
	if _, err := port.Write([]byte("ID;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf := make([]byte, 64)
	n, err := port.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got, want := buf[:n], []byte("ID0462;"); !bytes.Equal(got, want) {
		t.Errorf("reply over the port = %q, want %q", got, want)
	}
}

func TestClose_IsPromptDespiteAPendingLatency(t *testing.T) {
	r := New(WithLatency(time.Hour))
	port := r.Port()
	if _, err := port.Write([]byte("ID;")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	done := make(chan struct{})
	go func() {
		r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return promptly despite a one-hour reply latency")
	}
}

func TestAI_AcceptedAndNeverPushedUnsolicited(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	if _, err := port.Write([]byte("AI1;")); err != nil {
		t.Fatalf("Write AI1;: %v", err)
	}
	// A fire-and-forget Set answers with silence: prove there is nothing
	// waiting by round-tripping an ID query next and checking it is the
	// only thing that arrives.
	if _, err := port.Write([]byte("ID;")); err != nil {
		t.Fatalf("Write ID;: %v", err)
	}
	buf := make([]byte, 64)
	n, err := port.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got, want := buf[:n], []byte("ID0462;"); !bytes.Equal(got, want) {
		t.Errorf("reply over the port = %q, want %q (AI1; must not have pushed anything first)", got, want)
	}

	if _, err := port.Write([]byte("AI;")); err != nil {
		t.Fatalf("Write AI;: %v", err)
	}
	n, err = port.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got, want := buf[:n], []byte("AI1;"); !bytes.Equal(got, want) {
		t.Errorf("AI; reply = %q, want %q", got, want)
	}
}

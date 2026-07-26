// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// TestNewEngine_NilAllowFuncIsRefused: an Engine without a gate cannot be
// constructed at all. The reader goroutine must not have started either —
// NewEngine's nil check runs before it, so a refused construction leaves
// nothing running; the stub port's Read blocks until Close, so a leaked
// reader would be observable as a goroutine parked on a port this test
// never closes through an Engine.
func TestNewEngine_NilAllowFuncIsRefused(t *testing.T) {
	port := newStubPort("")
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngine(port, nil)
	if err == nil {
		t.Fatal("NewEngine(port, nil) returned no error — an ungated Engine must not be constructable")
	}
	if !errors.Is(err, ErrNoAllowlist) {
		t.Errorf("NewEngine(port, nil) error = %v, want it to wrap ErrNoAllowlist", err)
	}
	if e != nil {
		t.Error("NewEngine returned a non-nil Engine alongside its error")
	}

	// A refused construction touches the port not at all: nothing written,
	// and (checked via the same recorder) nothing about to be.
	port.mu.Lock()
	writes := len(port.writes)
	port.mu.Unlock()
	if writes != 0 {
		t.Errorf("refused NewEngine wrote %d frames to the port, want 0", writes)
	}
}

// TestNewEngine_NilAllowFuncIsRefusedBeforeReaderStarts pins the ORDER
// the nil check happens in: refusing after `go e.readLoop()` would leave a
// goroutine reading a port whose Engine the caller never received (and so
// can never Close). stubPort's Read parks until Close, so a started reader
// is still parked here; asserting the port was never read at all is the
// black-box form of "the goroutine never started".
func TestNewEngine_NilAllowFuncIsRefusedBeforeReaderStarts(t *testing.T) {
	port := newStubPort("ID0800;") // a reader that ran would consume this
	t.Cleanup(func() { _ = port.Close() })

	if _, err := NewEngine(port, nil); err == nil {
		t.Fatal("NewEngine(port, nil) returned no error")
	}

	// Give any (wrongly) started reader goroutine a chance to run.
	time.Sleep(50 * time.Millisecond)

	port.mu.Lock()
	remaining := len(port.toRead)
	port.mu.Unlock()
	if remaining != len("ID0800;") {
		t.Errorf("port had %d unread bytes, want %d — a reader goroutine was started despite the refused construction", remaining, len("ID0800;"))
	}
}

// TestEngineDo_RefusesWithNoAllowlist is the defence-in-depth half: even
// if an Engine reaches Do without a gate, nothing reaches the wire.
// Unreachable through NewEngine by construction — checked anyway, exactly
// as ErrDisallowedCommand's own doc comment argues for the layer below.
func TestEngineDo_RefusesWithNoAllowlist(t *testing.T) {
	port := newStubPort("ID0800;")
	t.Cleanup(func() { _ = port.Close() })

	// A permissive gate, only so construction succeeds: the property under
	// test is what Do does when the gate is ABSENT, not what any particular
	// gate admits.
	e, err := NewEngine(port, func([]byte) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	e.allow = nil // in-package: simulate the unreachable state

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = e.Do(ctx, cat.FT710.BuildIDRead(), CommandSpec{ExpectPrefix: "ID", ExpectLen: 7})
	if !errors.Is(err, ErrNoAllowlist) {
		t.Errorf("Do with a nil allowlist returned %v, want ErrNoAllowlist", err)
	}
	if errors.Is(err, ErrDisallowedCommand) {
		t.Error("Do with a nil allowlist reported ErrDisallowedCommand — a missing gate is a composition bug, and must not be blamed on the frame")
	}

	port.mu.Lock()
	writes := port.writes
	port.mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("Do wrote %q with no allowlist in force; want nothing to reach the wire", writes)
	}
}

// TestEngineDo_RejectedFrameIsNeverWritten pins the OTHER half of the
// gate's contract on the injected seam: a frame the supplied AllowFunc
// refuses is reported as ErrDisallowedCommand and never written — and the
// gate that decides is the injected one, not any package-level allowlist.
// The frame here (ID;) is one cat.FT710.AllowedCommand accepts, so only an
// Engine consulting ITS OWN gate can fail this write.
func TestEngineDo_RejectedFrameIsNeverWritten(t *testing.T) {
	port := newStubPort("ID0800;")
	t.Cleanup(func() { _ = port.Close() })

	var seen [][]byte
	e, err := NewEngine(port, func(frame []byte) bool {
		seen = append(seen, append([]byte(nil), frame...))
		return false // this dialect permits nothing
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = e.Do(ctx, cat.FT710.BuildIDRead(), CommandSpec{ExpectPrefix: "ID", ExpectLen: 7})
	if !errors.Is(err, ErrDisallowedCommand) {
		t.Fatalf("Do = %v, want errors.Is match against ErrDisallowedCommand", err)
	}

	port.mu.Lock()
	writes := port.writes
	port.mu.Unlock()
	if len(writes) != 0 {
		t.Errorf("Do wrote %q despite the injected gate refusing it", writes)
	}

	// Safety obligation 1's "same slice" rule, from the gate's side: the
	// gate is consulted exactly once per transmission attempt, on the very
	// bytes that would have been written.
	if len(seen) != 1 {
		t.Fatalf("gate was consulted %d times for one Do, want exactly 1", len(seen))
	}
	if got, want := string(seen[0]), "ID;"; got != want {
		t.Errorf("gate was handed %q, want %q — the checked slice must be the frame itself", got, want)
	}
}

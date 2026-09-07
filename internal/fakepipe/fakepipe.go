// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakepipe is the PROTOCOL-FREE plumbing every fake rig shares: the
// net.Pipe pair, the goroutine bookkeeping, the interruptible latency wait and
// the raw write. It is the ONE permitted share between the fakes (Stuart,
// 06/09/2026), and it is permitted precisely because there is no protocol in
// it — not a framing byte, not a field layout, not a reply. It sees []byte and
// time.Duration and nothing else, so a systematic bug in it cannot make a
// wrong codec look right: it can only stop bytes moving, which every fake's
// own tests notice at once.
//
// Everything above the wire stays per fake: each keeps its own reassembler,
// parser, image and reply builder, written independently from that radio's own
// documentation. See any fake's doc.go, "A SIBLING, not a refactor".
//
// This package imports the standard library only (imports_test.go).
package fakepipe

import (
	"net"
	"sync"
	"time"
)

// readChunkBytes is ReadLoop's buffer size. Frames may split across reads and
// several may arrive in one; every caller reassembles, so this is only a
// sizing choice.
const readChunkBytes = 4096

// Pipe is one fake radio's end of an in-memory duplex connection, plus the
// goroutines servicing it.
//
// The zero value is not usable; call New. Latency may be set before the first
// Go or ReadLoop and must not be touched afterwards — the serving goroutines
// read it without a lock, exactly as each fake's own options do.
type Pipe struct {
	host  net.Conn // returned by Host(); the caller's end
	radio net.Conn // the radio's own end

	// Latency is the per-reply delay Write honours. Set it from a
	// constructor option, before any goroutine starts.
	Latency time.Duration

	shutdown  chan struct{}
	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
}

// New makes a connected pair and returns the radio's side of it.
func New() *Pipe {
	host, radio := net.Pipe()
	return &Pipe{host: host, radio: radio, shutdown: make(chan struct{})}
}

// Host returns the caller's end of the connection — what a fake's Port()
// hands out. Repeated calls return the same connection.
func (p *Pipe) Host() net.Conn { return p.host }

// Conn returns the RADIO's end, for the rare caller that needs the net.Conn
// itself (a write deadline, say). Writing it outside Write/WriteNow is the
// caller's business to serialise.
func (p *Pipe) Conn() net.Conn { return p.radio }

// Done is closed when the radio goes away. Select against it instead of
// blocking forever on a send, a ticker or a sleep.
func (p *Pipe) Done() <-chan struct{} { return p.shutdown }

// Go runs fn in a goroutine that Close waits for.
func (p *Pipe) Go(fn func()) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		fn()
	}()
}

// ReadLoop reads the radio's end until it fails, handing each chunk of bytes
// to handle. It runs on the calling goroutine — wrap it in Go to start it.
// The slice handed to handle is reused between reads, so a handler that keeps
// bytes must copy them.
func (p *Pipe) ReadLoop(handle func([]byte)) {
	buf := make([]byte, readChunkBytes)
	for {
		n, err := p.radio.Read(buf)
		if n > 0 {
			handle(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// Sleep waits d, returning early (false) if the radio shuts down first.
// Returns true when the full d genuinely elapsed; d <= 0 returns true at once.
func (p *Pipe) Sleep(d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-p.shutdown:
		return false
	}
}

// Write puts bytes on the wire after the configured Latency. A Close during
// the wait abandons the write — the pipe is gone, so the bytes could never
// arrive. Reports whether the bytes actually went out.
func (p *Pipe) Write(b []byte) bool {
	if !p.Sleep(p.Latency) {
		return false
	}
	return p.WriteNow(b)
}

// WriteNow puts bytes on the wire immediately, ignoring Latency. Errors are
// not reported as errors: a write failing because the peer has gone away is an
// expected outcome of a test double being torn down, not a bug — the bool says
// only whether the caller may keep going.
func (p *Pipe) WriteNow(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	_, err := p.radio.Write(b)
	return err == nil
}

// Shutdown closes the radio's end WITHOUT waiting for the goroutines, so it is
// safe to call from inside one of them. Idempotent and race-safe.
//
// It deliberately closes only the radio's end, never the host's. net.Pipe
// reports io.ErrClosedPipe to a read or write made against the end you closed
// yourself, whilst the other, still-open end sees io.EOF — which is exactly
// the signal a host should get from "the radio went away". Closing the host's
// end here too would turn that into io.ErrClosedPipe, a worse and less
// consistent signal. A caller wanting to release its own handle may still
// close Host() itself.
func (p *Pipe) Shutdown() error {
	p.closeOnce.Do(func() {
		// shutdown closes FIRST, so anything parked in a latency wait or on a
		// channel send wakes before, or regardless of, the pipe closing under
		// it. That is what makes Close prompt despite a pending latency.
		close(p.shutdown)
		p.closeErr = p.radio.Close()
	})
	return p.closeErr
}

// Close shuts the pipe down and waits for every goroutine started with Go.
// Safe to call more than once.
func (p *Pipe) Close() error {
	err := p.Shutdown()
	p.wg.Wait()
	return err
}

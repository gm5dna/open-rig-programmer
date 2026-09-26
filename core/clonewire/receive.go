// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"context"
	"fmt"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ackByte is the ONLY outbound byte this package ever sends (spec.md
// Decisions item 5) — a per-block acknowledgement, never a memory-content
// write.
const ackByte = 0x06

// ImageReader is core/clonewire's own whole-image reader seam — NOT added
// to core/driver.Session, which has no per-slot operation this family
// could implement Session with at all (spec.md §Read model, point 7).
type ImageReader interface {
	Arm(ctx context.Context, port transport.Port, profiles []Profile) (*Reception, error)
}

// Reader is the default ImageReader: a thin method wrapper around the
// package-level Arm, so a caller (Phase 4's wiring) can hold an
// ImageReader value without depending on the free function directly.
type Reader struct{}

// Arm implements ImageReader.
func (Reader) Arm(ctx context.Context, port transport.Port, profiles []Profile) (*Reception, error) {
	return Arm(ctx, port, profiles)
}

var _ ImageReader = Reader{}

// Reception is one armed clone-mode read: the deadline clocks it carries
// start ticking the instant Arm returns, not when Receive is later called.
type Reception struct {
	port       transport.Port
	candidates []Profile

	armed chan struct{}

	startDeadline time.Time
	totalDeadline time.Time
}

// Arm starts the start/inter-block/total deadline clocks on the
// already-open port and returns immediately — before a single byte is
// read. profiles is the candidate set Receive will later match a fully
// received image's length against (spec.md §Identity probe); every
// candidate is assumed to share the wire framing of profiles[0] (see
// doc.go). A caller must wait on the returned Reception's Armed() channel
// before prompting the operator to start the transfer (spec.md §Read
// model, point 1) — in practice Armed() is already closed by the time Arm
// returns, but the signal, not Arm's return alone, is the documented
// contract Phase 4's CLI/GUI code is built against.
func Arm(ctx context.Context, port transport.Port, profiles []Profile) (*Reception, error) {
	if port == nil {
		return nil, fmt.Errorf("clonewire: Arm requires a non-nil port")
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("clonewire: Arm requires at least one candidate profile")
	}
	ref := profiles[0]
	if len(ref.BlockSchedule) == 0 {
		return nil, fmt.Errorf("clonewire: profile %s has an empty BlockSchedule", ref.Model)
	}

	now := time.Now()
	r := &Reception{
		port:          port,
		candidates:    profiles,
		armed:         make(chan struct{}),
		startDeadline: now.Add(ref.StartDeadline),
		totalDeadline: now.Add(ref.TotalDeadline),
	}
	// Deadlines are already live (computed from now above) — closing
	// armed only signals a caller that it is safe to prompt the operator.
	close(r.armed)
	return r, nil
}

// Armed is closed the instant Arm's deadlines go live — the readiness
// signal a caller waits on before printing "put the radio into clone-send
// mode now" (spec.md §Read model, point 1).
func (r *Reception) Armed() <-chan struct{} {
	return r.armed
}

// readResult is one raw chunk (or terminal error) from the background
// reader goroutine readLoop feeds Receive's select loop with.
type readResult struct {
	buf []byte
	err error
}

// readLoop issues blocking Reads against port and forwards every non-empty
// chunk, then any terminal error, over out. It exits once it has forwarded
// a terminal error, or once done is closed and it is not currently blocked
// inside Read — a Read blocked past that point (nothing more ever arrives
// and the port is never closed) is expected to unblock only when the
// caller closes the underlying port, same as any other reader goroutine
// against an io.ReadWriteCloser.
func readLoop(port transport.Port, out chan<- readResult, done <-chan struct{}) {
	buf := make([]byte, 4096)
	for {
		n, err := port.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			select {
			case out <- readResult{buf: chunk}:
			case <-done:
				return
			}
		}
		if err != nil {
			select {
			case out <- readResult{err: err}:
			case <-done:
			}
			return
		}
	}
}

// Receive accumulates bytes against r's running deadlines, sends the
// per-block ACK only where the matched candidate's schedule expects one,
// and returns the completed Image or a typed refusal
// (ErrImageIncomplete/ErrImageIncompatible/ErrImageAmbiguous) — never a
// partial Image (spec.md §Read model, point 5).
func (r *Reception) Receive(ctx context.Context) (Image, error) {
	ref := r.candidates[0]

	reads := make(chan readResult)
	done := make(chan struct{})
	go readLoop(r.port, reads, done)
	defer close(done)

	var raw []byte
	var pending []byte
	started := false
	blockIdx := 0

	for blockIdx < len(ref.BlockSchedule) {
		block := ref.BlockSchedule[blockIdx]

		var waitTimer *time.Timer
		if !started {
			waitTimer = time.NewTimer(time.Until(r.startDeadline))
		} else {
			waitTimer = time.NewTimer(ref.InterBlockDeadline)
		}
		totalTimer := time.NewTimer(time.Until(r.totalDeadline))

		select {
		case <-ctx.Done():
			waitTimer.Stop()
			totalTimer.Stop()
			return Image{}, fmt.Errorf("%w: %v", ErrImageIncomplete, ctx.Err())

		case <-totalTimer.C:
			waitTimer.Stop()
			return Image{}, fmt.Errorf("%w: total deadline exceeded after %d byte(s)", ErrImageIncomplete, len(raw))

		case <-waitTimer.C:
			totalTimer.Stop()
			if !started {
				return Image{}, fmt.Errorf("%w: no data within the start deadline", ErrImageIncomplete)
			}
			return Image{}, fmt.Errorf("%w: no data within the inter-block deadline (block %d)", ErrImageIncomplete, blockIdx)

		case rr := <-reads:
			waitTimer.Stop()
			totalTimer.Stop()
			if rr.err != nil {
				return Image{}, fmt.Errorf("%w: %v", ErrImageIncomplete, rr.err)
			}
			started = true
			pending = append(pending, rr.buf...)

			for blockIdx < len(ref.BlockSchedule) && len(pending) >= ref.BlockSchedule[blockIdx].Len {
				block = ref.BlockSchedule[blockIdx]
				chunk := pending[:block.Len]
				if block.Checksum != nil && !block.Checksum(chunk) {
					return Image{}, fmt.Errorf("%w: block %d checksum failed", ErrImageIncomplete, blockIdx)
				}
				// HeaderBytes/TrailerBytes are wire framing (e.g. a
				// leading block number, a trailing checksum byte) that
				// Checksum needed to see but Image.Raw must not carry —
				// see Block's doc comment (profile.go).
				content := chunk
				if block.HeaderBytes > 0 || block.TrailerBytes > 0 {
					content = chunk[block.HeaderBytes : len(chunk)-block.TrailerBytes]
				}
				raw = append(raw, content...)
				pending = pending[block.Len:]

				if ref.AckExpected && block.Ack {
					if _, werr := r.port.Write([]byte{ackByte}); werr != nil {
						return Image{}, fmt.Errorf("%w: writing ACK for block %d: %v", ErrImageIncomplete, blockIdx, werr)
					}
				}
				blockIdx++
			}
		}
	}

	// The schedule is fully consumed. Anything already buffered, or
	// anything that arrives within one more inter-block window, is a
	// trailing byte the profile's schedule did not account for — refused
	// whole, per spec.md §Read model point 5 (over-length is refused
	// exactly like short).
	if len(pending) > 0 {
		return Image{}, fmt.Errorf("%w: %d trailing byte(s) after the last expected block", ErrImageIncomplete, len(pending))
	}
	select {
	case rr := <-reads:
		if rr.err == nil {
			return Image{}, fmt.Errorf("%w: %d trailing byte(s) after the last expected block", ErrImageIncomplete, len(rr.buf))
		}
	case <-time.After(ref.InterBlockDeadline):
		// Clean end: nothing more arrived within the drain window.
	}

	return matchAndParse(raw, r.candidates)
}

// matchAndParse checks a fully-received image's length against every
// candidate Profile (spec.md §Identity probe): zero matches is
// ErrImageIncompatible, more than one is ErrImageAmbiguous, exactly one
// proceeds to ParseImage.
func matchAndParse(raw []byte, candidates []Profile) (Image, error) {
	var matched []Profile
	for _, c := range candidates {
		if c.ImageLen == len(raw) {
			matched = append(matched, c)
		}
	}
	switch len(matched) {
	case 0:
		return Image{}, fmt.Errorf("%w: %d byte(s) matches none of %d candidate profile(s)", ErrImageIncompatible, len(raw), len(candidates))
	case 1:
		return ParseImage(raw, matched[0])
	default:
		return Image{}, fmt.Errorf("%w: %d byte(s) matches %d of %d candidate profiles", ErrImageAmbiguous, len(raw), len(matched), len(candidates))
	}
}

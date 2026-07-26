// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// ErrTimeout means Engine.Do's configured Timeout elapsed while waiting for
// a read's answer frame, with no matching frame and no "?;" rejection
// having arrived. Do returns this sentinel directly (never wrapped) so
// errors.Is(err, ErrTimeout) is the reliable way to detect "no retries
// remain, and none will be attempted for a fire-and-forget spec" — see
// CommandSpec.RetryReads and ErrInvalidSpec.
var ErrTimeout = errors.New("transport: timed out waiting for a reply")

// ErrPortClosed means the Engine's underlying Port is no longer usable —
// either a caller called Engine.Close, or the reader goroutine observed the
// port go away on its own (a disconnect, or a real serial cable pulled
// out). Every Do, Init, and DrainToQuiet call made from this point on
// returns an error satisfying errors.Is(err, ErrPortClosed) immediately,
// without touching the port again.
//
// When the closure had an underlying cause (a spontaneous disconnect, not
// an explicit Close call), the error Do/DrainToQuiet actually return is a
// *PortClosedError wrapping it — errors.Is(err, ErrPortClosed) still holds
// via its Unwrap.
var ErrPortClosed = errors.New("transport: port is closed")

// ErrContaminated means cat.FrameAccumulator reported a frame exceeding the
// Engine's configured maximum length (cat.ErrFrameTooLong): the byte stream
// is desynchronised and cannot be trusted. Every Do call made from this
// point on fails fast with an error satisfying errors.Is(err,
// ErrContaminated) — without writing anything to the port — until a
// DrainToQuiet call observes a full QuietPeriod of silence and clears the
// state. See doc.go, "The CONTAMINATED state".
//
// The error Do/DrainToQuiet actually return for this condition is a
// *ContaminatedError wrapping the underlying *cat.FrameTooLongError —
// errors.Is(err, ErrContaminated) still holds via its Unwrap.
var ErrContaminated = errors.New("transport: port contaminated, awaiting DrainToQuiet")

// ErrInvalidSpec means a CommandSpec passed to Engine.Do violates one of
// Do's structural invariants. Today there is exactly one: RetryReads must
// be 0 when ExpectPrefix == "" (a fire-and-forget spec) — safety obligation
// 2 requires that a write's timeout/failure is NEVER resolved by resending,
// and this is enforced here, structurally, rather than merely by
// convention: Do returns this error WITHOUT writing anything to the port at
// all, before even attempting the exchange.
var ErrInvalidSpec = errors.New("transport: invalid CommandSpec")

// ErrQuarantineFailed means Do's entry-time suspect drain — the
// full drain-to-quiet Do runs, before transmitting anything, when a
// PRIOR exchange left the stream's state uncertain (a read's terminal
// timeout with retries exhausted, or a context cancellation observed
// after that exchange's frame had already been written; see the
// "suspect" flag in doc.go) — could not confirm the stream quiet.
// Do returns this WITHOUT writing anything to the port: transmitting
// into a stream that might still be about to deliver a stale, abandoned
// exchange's reply is exactly the hazard this whole mechanism exists to
// avoid. errors.Is(err, ErrQuarantineFailed) is the reliable way to
// detect this specific failure mode; the underlying cause the failed
// drain itself returned (typically ErrPortClosed, or the fresh bounded
// context's own deadline exceeded if traffic kept resetting the quiet
// timer) is reachable too — see QuarantineFailedError.
var ErrQuarantineFailed = errors.New("transport: entry quarantine drain failed, refusing to transmit")

// ErrDisallowedCommand means cmd.Bytes() failed the Engine's injected
// AllowFunc gate (in this repository's composition, the driver's own
// cat.Dialect.AllowedCommand) and was refused before ever reaching the
// wire — safety obligation 1. This should be unreachable for any Command
// actually produced by that same dialect's builders
// (TestAllowedCommand_PropertyEveryBuilderOutput in core/cat pins that
// invariant); Engine still checks defensively, because it is the last
// defence before a physical radio ever sees these bytes.
var ErrDisallowedCommand = errors.New("transport: command failed AllowedCommand, refused")

// ErrNoAllowlist means an Engine was asked to transmit with no allowlist.
// Distinct from ErrDisallowedCommand deliberately: that one means "this
// frame is not permitted", this one means "this Engine was misassembled".
// Both refuse, but conflating them would have a diagnostic blame the
// frame for a composition bug.
//
// NewEngine returns this (wrapped) for a nil AllowFunc, before it starts
// the reader goroutine, so NewEngine cannot RETURN an ungated Engine. A
// HAND-BUILT one still can be — Engine is exported, so `new(transport.Engine)`
// compiles in any package — and that is exactly what this sentinel is for
// at the other end: Do returns it too, so such a value fails closed on the
// last line before the wire rather than being prevented from existing
// (M9b fix wave, Codex finding 3).
var ErrNoAllowlist = errors.New("transport: engine has no allowlist, refusing to transmit")

// PortClosedError wraps ErrPortClosed with the underlying cause, when one
// is known: the io error (typically io.EOF) the reader goroutine observed
// when the port went away on its own. A caller-initiated Engine.Close has
// no such cause and is reported as the bare ErrPortClosed sentinel instead.
type PortClosedError struct {
	// Cause is the I/O error that triggered closure, or nil for an
	// explicit Engine.Close call.
	Cause error
}

// Error implements the error interface.
func (e *PortClosedError) Error() string {
	if e.Cause == nil {
		return ErrPortClosed.Error()
	}
	return fmt.Sprintf("%s: %s", ErrPortClosed.Error(), e.Cause.Error())
}

// Unwrap lets errors.Is(err, ErrPortClosed) match (always), AND lets
// errors.Is/errors.As reach Cause itself (e.g. errors.Is(err, io.EOF) for a
// spontaneous disconnect) — both are part of "what this error means",
// so both are exposed via the multi-error Unwrap form.
func (e *PortClosedError) Unwrap() []error {
	if e.Cause == nil {
		return []error{ErrPortClosed}
	}
	return []error{ErrPortClosed, e.Cause}
}

// ContaminatedError wraps ErrContaminated with the *cat.FrameTooLongError
// that caused it, so a caller or logger can recover DiscardedLen.
type ContaminatedError struct {
	// Cause is the frame-accumulator violation that triggered
	// contamination. Never nil.
	Cause *cat.FrameTooLongError
}

// Error implements the error interface.
func (e *ContaminatedError) Error() string {
	return fmt.Sprintf("%s: %s", ErrContaminated.Error(), e.Cause.Error())
}

// Unwrap lets errors.Is(err, ErrContaminated) match (always), AND lets
// errors.Is(err, cat.ErrFrameTooLong) reach through Cause — both are part
// of "what this error means", so both are exposed via the multi-error
// Unwrap form.
func (e *ContaminatedError) Unwrap() []error {
	return []error{ErrContaminated, e.Cause}
}

// QuarantineFailedError wraps ErrQuarantineFailed with the error the
// failed entry-time suspect drain itself returned (see Engine.Do).
type QuarantineFailedError struct {
	// Cause is the error the failed drain-to-quiet call returned. Never
	// nil in practice — wrapQuarantineFailedErr falls back to the bare
	// sentinel if it ever would be.
	Cause error
}

// Error implements the error interface.
func (e *QuarantineFailedError) Error() string {
	return fmt.Sprintf("%s: %s", ErrQuarantineFailed.Error(), e.Cause.Error())
}

// Unwrap lets errors.Is(err, ErrQuarantineFailed) match (always), AND
// lets errors.Is/errors.As reach Cause itself (e.g. errors.Is(err,
// ErrPortClosed) when the port went away mid-drain) — both are part of
// "what this error means", so both are exposed via the multi-error
// Unwrap form.
func (e *QuarantineFailedError) Unwrap() []error {
	return []error{ErrQuarantineFailed, e.Cause}
}

// wrapQuarantineFailedErr builds the error Do returns when its entry-time
// suspect drain fails, given the error that drain itself returned. If
// cause is nil (defensive: should not happen in practice), it falls back
// to the bare sentinel rather than constructing a struct with a nil Cause
// that would panic on Error().
func wrapQuarantineFailedErr(cause error) error {
	if cause == nil {
		return ErrQuarantineFailed
	}
	return &QuarantineFailedError{Cause: cause}
}

// wrapClosedErr builds the error Do/DrainToQuiet return for a closed
// port, given the stored closure cause (nil for an explicit Close call).
func wrapClosedErr(cause error) error {
	if cause == nil {
		return ErrPortClosed
	}
	return &PortClosedError{Cause: cause}
}

// wrapContaminatedErr builds the error Do/DrainToQuiet return for a
// contamination event, given the *cat.FrameTooLongError that caused it. If
// cause is nil (defensive: should not happen in practice — every caller
// passes the FrameTooLongError that triggered contamination), it falls back
// to the bare sentinel rather than constructing a struct with a nil Cause
// that would panic on Error().
func wrapContaminatedErr(cause *cat.FrameTooLongError) error {
	if cause == nil {
		return ErrContaminated
	}
	return &ContaminatedError{Cause: cause}
}

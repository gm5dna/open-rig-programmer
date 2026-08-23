// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// ErrRejected, ErrFrameTooLong and FrameTooLongError are RE-EXPORTS of
// core/cat's own (D2, adjudication 1, round 2 F1). Neutral code — the
// engine itself, core/civ, and every caller above them — uses the
// TRANSPORT names; errors.Is/errors.As at every existing call site still
// passes, because these are the same values and the same type, not copies
// of them.
//
// THE RECORDED WART, stated rather than hidden: the canonical values still
// LIVE in core/cat, a package whose name says "Yaesu CAT", even though a
// CI-V FA rejection now also produces transport.ErrRejected and a CI-V
// accumulator's oversize frame now also produces a *FrameTooLongError. The
// direction is forced: core/transport already imports core/cat (doc.go
// says why, and NewEngine's cat.Dialect parameter makes it unavoidable),
// so the reverse alias — cat.ErrRejected = transport.ErrRejected — would
// cycle. Re-exporting is the cycle-free half of that pair. A later
// cleanup may move the canonical values to a leaf package neither codec
// package owns; until then, byte identity is claimed for WIRE BEHAVIOUR
// and the recorded manifest recipes, not for the Go API, and this is one
// of the places the Go API shows its history.
//
// The sentinel and the TYPE are distinct things and both are re-exported:
// ErrFrameTooLong is what errors.Is compares against, FrameTooLongError is
// what errors.As reaches for DiscardedLen.
var (
	// ErrRejected is the protocol's NAK — CAT's unattributed "?;", CI-V's
	// FA. See cat.ErrRejected for the full warning against inferring a
	// cause from it: for CAT there is none to infer.
	ErrRejected = cat.ErrRejected
	// ErrFrameTooLong is the sentinel an Accumulator reports when stream
	// data exceeded the configured maximum frame length without forming
	// a legitimate frame. Engine treats it as CONTAMINATION — see
	// ErrContaminated and doc.go.
	ErrFrameTooLong = cat.ErrFrameTooLong
)

// FrameTooLongError is the error type carrying ErrFrameTooLong's
// DiscardedLen — the number of bytes an Accumulator threw away. See the
// re-export note above, and cat.FrameTooLongError for the full
// description.
type FrameTooLongError = cat.FrameTooLongError

// ErrNoFraming means NewEngineWith was handed a nil Framing. It is refused
// before the reader goroutine starts, so NewEngineWith cannot RETURN an
// Engine bound to no protocol.
//
// DISTINCT FROM ErrUnconfiguredDialect: that one means "this cat.Dialect
// describes no radio", a question only the CAT wrapper can ask and one it
// answers before NewEngineWith is ever reached. This one means the seam
// itself was left empty.
var ErrNoFraming = errors.New("transport: engine was given no framing, refusing to construct")

// ErrDrainCapExceeded means a drain reached its framing's absolute
// DrainPolicy.Cap without ever observing the required idle gap: the line
// is delivering frames continuously and is not going to go quiet.
//
// It is deliberately NOT reported as success. A drain's whole job is to
// establish that nothing from a previous, abandoned exchange is still in
// flight, and a stream that never pauses has established no such thing.
// The condition is a NORMAL operating state for an Icom radio in
// transceive mode, not a fault — which is why the engine's response to it
// is bounded and specific (Do's entry drain wraps it in
// ErrQuarantineFailed and refuses to transmit; the post-write quarantine
// logs it and sets suspect; Init returns it) rather than a hang.
var ErrDrainCapExceeded = errors.New("transport: drain exceeded its absolute cap, stream never went quiet")

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

// ErrContaminated means the framing's Accumulator reported a frame
// exceeding the Engine's configured maximum length (ErrFrameTooLong): the byte stream
// is desynchronised and cannot be trusted. Every Do call made from this
// point on fails fast with an error satisfying errors.Is(err,
// ErrContaminated) — without writing anything to the port — until a
// DrainToQuiet call observes a full QuietPeriod of silence and clears the
// state. See doc.go, "The CONTAMINATED state".
//
// The error Do/DrainToQuiet actually return for this condition is a
// *ContaminatedError wrapping the underlying *FrameTooLongError —
// errors.Is(err, ErrContaminated) still holds via its Unwrap.
var ErrContaminated = errors.New("transport: port contaminated, awaiting DrainToQuiet")

// ErrInvalidSpec means a CommandSpec passed to Engine.Do violates one of
// Do's structural invariants — or that Do was handed a nil Command. Every
// one is enforced structurally rather than by convention: Do returns this
// error WITHOUT writing anything to the port at all, before even
// attempting the exchange. The invariants, all in CommandSpec.validate:
//
//   - Class must be stated. The ZERO Class is refused (D2 retired the
//     implicit ExpectPrefix=="" keying, under which an unfilled spec
//     silently became a fire-and-forget write).
//   - RetryReads must be 0 for ClassWrite AND ClassWriteWithAck — safety
//     obligation 2: a write's timeout or failure is NEVER resolved by
//     resending, and an acknowledged write is still a write.
//   - Match is required for ClassRead and ClassWriteWithAck (the engine
//     has no answer-matching rule of its own since D2 moved it into the
//     spec) and must be nil for ClassWrite (which expects no answer; a
//     write whose acknowledgement you mean to wait for is
//     ClassWriteWithAck).
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
// Since M9c-5 (E3) this is the HAND-BUILT Engine's sentinel and nothing
// else: NewEngine takes a cat.Dialect and always sets the gate from it, so
// no Engine it returns can have a nil one, and the constructor's own
// refusal is ErrUnconfiguredDialect instead. A hand-built Engine still
// exists — the type is exported, so `new(transport.Engine)` compiles in
// any package — and that is exactly what this sentinel is for: Do returns
// it, so such a value fails closed on the last line before the wire rather
// than being prevented from existing (M9b fix wave, Codex finding 3).
var ErrNoAllowlist = errors.New("transport: engine has no allowlist, refusing to transmit")

// ErrUnconfiguredDialect means NewEngine was handed a cat.Dialect that
// describes no radio — the zero value, whose Configured() is false. It is
// refused before the reader goroutine starts, so NewEngine cannot RETURN
// an Engine bound to nothing.
//
// DISTINCT FROM ErrNoAllowlist deliberately, and the distinction is not
// cosmetic. cat.Dialect is a struct, so `var d cat.Dialect` yields a
// perfectly non-nil AllowedCommand method value: a missing-gate check
// cannot see it, and the value would have been installed as a real
// Engine's gate. What saves that case is core/cat's own fail-closed rule
// (an unconfigured dialect admits NOTHING), which means the resulting
// Engine refuses every frame — correct, but silently, and at Do rather
// than at construction. This sentinel says the true thing at the true
// moment: the engine was never given a radio to speak for. ErrNoAllowlist
// keeps its own, different meaning — a hand-built Engine reached Do with
// no gate at all.
var ErrUnconfiguredDialect = errors.New("transport: engine was given an unconfigured dialect, refusing to construct")

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

// ContaminatedError wraps ErrContaminated with the *FrameTooLongError
// that caused it, so a caller or logger can recover DiscardedLen.
type ContaminatedError struct {
	// Cause is the frame-accumulator violation that triggered
	// contamination. Never nil.
	Cause *FrameTooLongError
}

// Error implements the error interface.
func (e *ContaminatedError) Error() string {
	return fmt.Sprintf("%s: %s", ErrContaminated.Error(), e.Cause.Error())
}

// Unwrap lets errors.Is(err, ErrContaminated) match (always), AND lets
// errors.Is(err, ErrFrameTooLong) reach through Cause — both are part
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
// contamination event, given the *FrameTooLongError that caused it. If
// cause is nil (defensive: should not happen in practice — every caller
// passes the FrameTooLongError that triggered contamination), it falls back
// to the bare sentinel rather than constructing a struct with a nil Cause
// that would panic on Error().
func wrapContaminatedErr(cause *FrameTooLongError) error {
	if cause == nil {
		return ErrContaminated
	}
	return &ContaminatedError{Cause: cause}
}

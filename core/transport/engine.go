// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// Logger is the injectable sink Engine uses to report unexpected frames and
// other diagnostics (safety obligation 3: unexpected frames are surfaced,
// never silently discarded). NewEngine defaults to a Logger that drops
// everything — production callers that care must supply one via
// WithLogger.
type Logger interface {
	Printf(format string, args ...any)
}

// nopLogger is the default Logger: it drops everything.
type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

// clock abstracts time so Engine's internals are testable without real
// sleeps beyond what the engine itself requires (see WithClock). realClock
// is the production default; a test may substitute a fake for an isolated,
// Port-independent unit test of timing-adjacent logic (e.g. that Settle is
// honoured) without needing fakeradio's real elapsed time at all. Engine
// tests that exercise protocol behaviour against fakeradio deliberately use
// the real clock throughout: fakeradio's own fault timing (WithLatency,
// FaultDelayedRejection) happens in real wall-clock time, so mixing it with
// a virtual clock on the engine side would desynchronise the two rather
// than speed the tests up.
type clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
	Sleep(d time.Duration)
}

// realClock is the production clock: a thin pass-through to the time
// package.
type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (realClock) Sleep(d time.Duration)                  { time.Sleep(d) }

// Timing defaults for CommandSpec and DrainToQuiet.
//
// PRE-HARDWARE ESTIMATES — every value below is a plausible starting point
// chosen without a physical FT-710 to measure against. All four are to be
// measured and, if necessary, corrected at M5a once real CAT transcripts
// are available; nothing in this package should be read as claiming these
// are the radio's actual characteristics.
const (
	// DefaultTimeout is how long Do waits for a matching answer frame (a
	// read command's ExpectPrefix) before giving up. PRE-HARDWARE
	// ESTIMATE — to be measured at M5a.
	DefaultTimeout = 1 * time.Second
	// DefaultErrorWindow is how long a fire-and-forget Do listens for a
	// delayed "?;" rejection before declaring success. PRE-HARDWARE
	// ESTIMATE — to be measured at M5a.
	DefaultErrorWindow = 150 * time.Millisecond
	// DefaultSettle is the pacing delay Do applies after a completed
	// exchange, before releasing its internal mutex for the next Do call.
	// PRE-HARDWARE ESTIMATE — to be measured at M5a.
	DefaultSettle = 20 * time.Millisecond
	// QuietPeriod is how long DrainToQuiet must observe silence on the
	// port — no frame, no accumulator error — before declaring it
	// drained. PRE-HARDWARE ESTIMATE — to be measured at M5a.
	QuietPeriod = 200 * time.Millisecond
)

// CommandSpec tells Engine.Do what to expect back for one command, and how
// to behave when that expectation isn't met. See Do's doc comment for full
// semantics.
type CommandSpec struct {
	// ExpectPrefix is the answer frame's required leading bytes (e.g.
	// "MR"). Empty means the command is fire-and-forget: Do does not
	// expect any answer frame, only listens for a possible delayed "?;"
	// rejection within ErrorWindow.
	//
	// For a command family whose answer frames share a short prefix
	// across MANY distinct addresses — EX (MENU) is the case in point,
	// where every one of the 296 Table 2 addresses answers with a frame
	// starting "EX" — ExpectPrefix MUST include the full address, e.g.
	// "EX"+addr.Wire(), never the bare command name "EX". matches (below)
	// only compares frame[:len(ExpectPrefix)]; a bare "EX" would let it
	// correlate ANY EX answer — a different address's own reply still in
	// flight, or an unsolicited AI-pushed EX frame — as this read's
	// answer, silently returning the wrong address's data. See
	// core/transport/engine_ex_test.go
	// (TestEngine_EXRead_PrefixOnlySpec_DemonstratesWrongAddressHazard)
	// for the negative-space proof, and
	// TestEngine_EXRead_WrongAddressInjectedFirst_NotConsumed for the
	// same injection surviving correctly once ExpectPrefix carries the
	// full address. This is a convention callers must follow, not
	// something Engine enforces: CommandSpec has no address-shaped field
	// of its own, and matches' prefix-comparison logic is otherwise
	// exactly what every other single-address command family (MR, MT,
	// ...) already relies on.
	ExpectPrefix string
	// ExpectLen is the answer frame's exact required length, including
	// the trailing ';'. 0 means variable length (e.g. MT answers, whose
	// tag is 0-12 bytes) — only ExpectPrefix is checked.
	ExpectLen int
	// Timeout bounds how long Do waits for a matching answer frame on a
	// read (ExpectPrefix != ""). <= 0 selects DefaultTimeout. Unused for
	// a fire-and-forget spec.
	Timeout time.Duration
	// ErrorWindow bounds how long a fire-and-forget Do (ExpectPrefix ==
	// "") listens for a delayed "?;" rejection before declaring success.
	// <= 0 selects DefaultErrorWindow.
	ErrorWindow time.Duration
	// Settle is a pacing delay Do applies after a completed exchange,
	// before releasing its internal mutex. <= 0 selects DefaultSettle.
	Settle time.Duration
	// RetryReads is how many additional attempts Do makes after an
	// initial read timeout, each preceded by a DrainToQuiet quarantine.
	// MUST be 0 when ExpectPrefix == "" (fire-and-forget): Do returns
	// ErrInvalidSpec, without writing anything, if this is violated — see
	// safety obligation 2 and ErrInvalidSpec's doc comment.
	RetryReads int
}

// validate reports the ErrInvalidSpec violation in spec, if any.
func (spec CommandSpec) validate() error {
	if spec.ExpectPrefix == "" && spec.RetryReads != 0 {
		return fmt.Errorf("%w: RetryReads (%d) must be 0 for a fire-and-forget spec (ExpectPrefix==\"\") — a write's timeout/failure is never resolved by resending", ErrInvalidSpec, spec.RetryReads)
	}
	return nil
}

// withDefaults returns a copy of spec with every <= 0 timing field replaced
// by its documented default.
func (spec CommandSpec) withDefaults() CommandSpec {
	if spec.Timeout <= 0 {
		spec.Timeout = DefaultTimeout
	}
	if spec.ErrorWindow <= 0 {
		spec.ErrorWindow = DefaultErrorWindow
	}
	if spec.Settle <= 0 {
		spec.Settle = DefaultSettle
	}
	return spec
}

// matches reports whether frame satisfies spec's ExpectPrefix (and
// ExpectLen, when non-zero).
func (spec CommandSpec) matches(frame []byte) bool {
	if len(frame) < len(spec.ExpectPrefix) {
		return false
	}
	if string(frame[:len(spec.ExpectPrefix)]) != spec.ExpectPrefix {
		return false
	}
	if spec.ExpectLen > 0 && len(frame) != spec.ExpectLen {
		return false
	}
	return true
}

// readerEvent is one unit the reader goroutine hands to whichever of
// Do/DrainToQuiet is currently consuming e.events (there is at most one,
// serialised by e.mu — see Engine's doc comment): either a complete frame,
// or a terminal accumulator/read error.
type readerEvent struct {
	frame []byte // nil if err != nil
	err   error  // *cat.FrameTooLongError (stream contamination), or a raw I/O error (port gone)
}

// AllowFunc is the outbound write gate an Engine holds: it reports whether
// frame is safe to write to the radio. It is the last defence before a
// physical radio ever sees these bytes (safety obligation 1), and it is
// DERIVED FROM THE ENGINE'S OWN DIALECT rather than reached for from a
// package global, so an Engine gates for the radio of the driver that
// built it and for no other.
//
// It is no longer a CONSTRUCTOR PARAMETER (M9c-5, E3). NewEngine takes the
// cat.Dialect whole and takes d.AllowedCommand itself — the method value
// matches this signature exactly — so the gate and the session-init frame
// (Init, d.BuildAISet(false)) provably come from ONE value. Passing the
// two separately made it structurally possible to gate for one radio and
// initialise for another; nothing had ever done so, and no defect existed
// (see Init's ledger note), but the shape allowed it and the type does not
// have to. Reaching for a package-level allowlist instead would gate every
// radio by whatever the FT-710 permits, which is a safety failure rather
// than merely a correctness one.
//
// An unconfigured (zero) cat.Dialect is refused by NewEngine
// (ErrUnconfiguredDialect) before the reader goroutine is even started; a
// nil gate — reachable only in a hand-built Engine, never one NewEngine
// returned — is refused by Do (ErrNoAllowlist).
//
// CONTRACT ON frame — an implementation MUST NOT mutate it, and MUST
// NOT retain it beyond the call. Do hands the gate the very slice it
// then writes (safety obligation 1 requires exactly one Bytes() call,
// checked and written), so the slice is writable and live. A gate that
// edited it would make the bytes checked and the bytes written differ
// while still passing every "same slice" check; a gate that kept a
// reference could observe or alter a later attempt's frame, since Do
// re-derives and re-gates inside the retry loop. cat.Command.Bytes()
// already returns a fresh defensive copy per call, so this obligation
// is what makes its promise — that the transport writes bytes nobody
// else can reach — true of the gate as well as of the caller.
type AllowFunc func(frame []byte) bool

// Option configures a *Engine at construction time. See NewEngine.
type Option func(*Engine)

// WithLogger sets the Logger Engine uses to report unexpected frames and
// other diagnostics (safety obligation 3). A nil Logger is ignored (the
// nopLogger default is kept).
func WithLogger(l Logger) Option {
	return func(e *Engine) {
		if l != nil {
			e.logger = l
		}
	}
}

// WithClock overrides Engine's internal clock. Intended for tests that want
// to exercise timing-adjacent logic (e.g. that Settle is applied) in
// isolation, without depending on fakeradio's real elapsed time — see
// clock's doc comment for why engine tests that talk to fakeradio should
// NOT combine it with a virtual clock.
func WithClock(c clock) Option {
	return func(e *Engine) {
		if c != nil {
			e.clk = c
		}
	}
}

// WithMaxFrame sets the maximum frame length (see cat.NewFrameAccumulator)
// the reader goroutine's accumulator enforces. <= 0 selects
// cat.DefaultMaxFrame.
func WithMaxFrame(n int) Option {
	return func(e *Engine) { e.maxFrame = n }
}

// Engine is the transaction engine: the one place raw bytes cross the wire
// to/from a Port, giving every higher layer safe request/response semantics
// over a protocol with no sequence numbers and no write acknowledgements.
// See doc.go for the full safety model.
//
// A single internal mutex (mu) serialises every Do and DrainToQuiet call:
// at most one is ever "in flight" — writing, or waiting for a reply — at a
// time, which is what makes matching an unattributed reply back to the
// request that (probably) caused it sound at all. A dedicated reader
// goroutine (readLoop) owns every Read() call against the Port and is the
// ONLY goroutine that ever touches it for reading; it feeds decoded frames
// (and accumulator errors) to whichever call currently holds mu via the
// buffered events channel.
type Engine struct {
	port   Port
	logger Logger
	clk    clock
	// dialect is the ONE cat.Dialect this Engine is bound to, supplied at
	// construction. Both allow (below) and Init's AI frame derive from
	// it, so the gate and the init frame cannot belong to different
	// radios. Immutable after construction (no Option touches it), and
	// cat.Dialect is a value type carrying only copied maps and slices,
	// so Init reads it without synchronisation. Configured() for an
	// Engine NewEngine actually returned — see ErrUnconfiguredDialect.
	dialect cat.Dialect
	// allow is the outbound write gate, taken from dialect at
	// construction (d.AllowedCommand) and never nil for an Engine
	// NewEngine actually returned — see AllowFunc and ErrNoAllowlist.
	// Immutable after construction (no Option touches it), so Do reads
	// it without synchronisation. Held as a field, rather than called as
	// e.dialect.AllowedCommand at each write, so that Do's own
	// defence-in-depth nil check keeps meaning something for a
	// hand-built Engine, whose zero dialect would otherwise present a
	// non-nil method value that refuses everything for a different
	// reason.
	allow    AllowFunc
	maxFrame int

	mu sync.Mutex // serialises Do/DrainToQuiet: "single outstanding request"

	events chan readerEvent // reader goroutine -> whichever call holds mu

	closeCh      chan struct{} // closed exactly once, unblocks any select waiting on it
	readerDone   chan struct{} // closed by readLoop when it returns (Close joins on this)
	closeOnce    sync.Once
	closed       atomic.Bool
	closeCause   atomic.Pointer[error] // nil = explicit Close; else the io error that closed the port
	portCloseErr error                 // e.port.Close()'s result; written once under closeOnce, read only after

	contaminated atomic.Bool

	// suspect marks that the outcome of the PREVIOUS exchange is
	// uncertain enough that its stream state cannot be trusted for the
	// next transmission — set (i) after a read's terminal timeout
	// (RetryReads exhausted: see Do's read-attempt loop) and (ii) after a
	// ctx cancellation observed once a frame had already been written
	// (fire-and-forget or read). Both are cases where Engine stopped
	// listening without a definitive outcome from the radio, so a reply
	// may still be in flight. Only ever read/written while e.mu is held
	// (every Do/DrainToQuiet call), so a plain bool suffices — see Do's
	// entry-time drain and drainToQuietLocked, which clears it on
	// success.
	suspect bool

	unexpectedFrames atomic.Int64
}

// NewEngine constructs an Engine over p, bound to the cat.Dialect d, and
// starts its reader goroutine. Options: WithLogger, WithClock, WithMaxFrame.
//
// THE DIALECT IS THE BINDING, and it is REQUIRED (M9c-5, E3). Both the
// outbound write gate (d.AllowedCommand — see AllowFunc) and the
// session-init frame (d.BuildAISet(false) — see Init) derive from this one
// value, so an Engine cannot gate for one radio while initialising for
// another. It is a plain parameter rather than a defaulted Option
// precisely because a default does not enforce same-dialect binding: a
// caller could supply one half and inherit the other.
//
// FAIL-CLOSED AT CONSTRUCTION: an UNCONFIGURED (zero) dialect is refused
// with an error wrapping ErrUnconfiguredDialect, returning a nil *Engine —
// and refused BEFORE the reader goroutine is started, so a rejected call
// leaves nothing running and no half-built Engine for a caller to ignore
// the error and use anyway. NewEngine therefore CANNOT RETURN an Engine
// that gates for no radio. The check is Configured() rather than a nil
// test because cat.Dialect is a struct: `var d cat.Dialect` yields a
// non-nil AllowedCommand method value that would have satisfied any nil
// check while describing no radio at all.
//
// That is a claim about this constructor, not about the type, and the
// difference is real (M9b fix wave, Codex finding 3): Engine is exported,
// so any package may write `var e transport.Engine`, `new(transport.Engine)`
// or an empty literal, and neither this function nor the AST guard in
// internal/guards prohibits an external type use. Such a value is not a
// write-safety failure — it has no usable port, and its nil allow makes Do
// fail closed with ErrNoAllowlist — but it exists, and the earlier wording
// here ("an ungated Engine cannot exist") said otherwise.
//
// Do re-checks the gate before every write regardless (defence in depth:
// this is the last line before a physical radio).
//
// NewEngine does NOT take ownership of p on the refusal path: it has not
// touched the port at all by then, so closing it stays the caller's
// business, exactly as it is for any other construction the caller
// abandons.
func NewEngine(p Port, d cat.Dialect, opts ...Option) (*Engine, error) {
	if !d.Configured() {
		return nil, fmt.Errorf("%w: NewEngine requires a configured cat.Dialect (a driver passes its own — the gate and the init frame both come from it)", ErrUnconfiguredDialect)
	}

	e := &Engine{
		port:       p,
		logger:     nopLogger{},
		clk:        realClock{},
		dialect:    d,
		allow:      d.AllowedCommand,
		events:     make(chan readerEvent, 16),
		closeCh:    make(chan struct{}),
		readerDone: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(e)
	}

	go e.readLoop()
	return e, nil
}

// UnexpectedFrames reports how many frames Do/DrainToQuiet have received
// that did not match the spec in force at the time (or, during
// DrainToQuiet, any frame at all) — safety obligation 3's counter. Safe for
// concurrent use.
func (e *Engine) UnexpectedFrames() int64 {
	return e.unexpectedFrames.Load()
}

// Init sends AI0; (disabling Auto Information, the documented default a
// fresh CAT session should establish) as a fire-and-forget command, then
// drains the port to quiet — clearing out anything left over from before
// this session started talking to the radio (a stale unsolicited push, or
// a reply to a command sent by a previous, now-gone session).
//
// The frame is built by THIS Engine's own dialect, the same value its
// gate came from (see NewEngine): one binding, so the frame Init builds
// and the gate that judges it can never belong to different radios.
//
// M9b's ledger note here is CLOSED, and its premise CORRECTED (M9c-5,
// E3). It read: this is the one place a HARDWIRED cat.FT710 frame meets
// an INJECTED gate, and a second dialect whose gate did not admit the
// FT-710's AI0; form would make Init fail closed — safe, but baffling to
// diagnose. THAT FAILURE COULD NOT OCCUR. AllowedCommand sees bytes, not
// provenance; every configured dialect builds exactly "AI0;" and every
// configured dialect's gate admits that form, so no fixture could ever
// have demonstrated the refusal, and none was ever observed. What the
// hardwiring actually was is architectural impurity — a package-level
// dialect reached for inside a type that already held one — and taking
// the dialect whole at construction removes it. The bytes are unchanged
// ("AI0;"), pinned by TestEngine_Init_WritesExactlyAI0.
func (e *Engine) Init(ctx context.Context) error {
	aiCmd := e.dialect.BuildAISet(false)
	if _, err := e.Do(ctx, aiCmd, CommandSpec{}); err != nil {
		return fmt.Errorf("transport: Init: AI0: %w", err)
	}
	return e.DrainToQuiet(ctx)
}

// Do sends cmd and waits for its outcome per spec, serialised against every
// other Do/DrainToQuiet call by e.mu (single outstanding request).
//
// Inter-transaction quarantine (doc.go, "Inter-transaction quarantine"): Do
// starts, still holding e.mu, by (1) running a full drain-to-quiet — using
// a FRESH context independent of ctx, so a caller's own cancelled/expired
// ctx can never prevent it — if the PREVIOUS exchange left e.suspect set
// (a read's terminal timeout, or a ctx cancellation observed after that
// exchange had already written its frame), clearing the flag only if the
// drain succeeds; and then (2) non-blockingly purging anything already
// sitting in e.events, regardless of suspect. Both happen before cmd is
// ever transmitted. See "Fire-and-forget" and "Read" below for what sets
// e.suspect in the first place.
//
// Transmission (safety obligation 1): cmd.Bytes() is called EXACTLY ONCE
// per transmission attempt, the resulting slice is checked with the
// Engine's injected gate (e.allow — see AllowFunc), and — only if that
// check passes — the SAME slice is written to the Port. A command the gate
// rejects is refused with ErrDisallowedCommand and NEVER reaches the wire;
// an Engine with no gate at all refuses with ErrNoAllowlist instead
// (unreachable via NewEngine, which will not build one — defence in depth).
//
// Fire-and-forget (spec.ExpectPrefix == ""): after writing, Do listens for
// spec.ErrorWindow. A "?;" arriving in that window returns cat.ErrRejected.
// Any OTHER frame arriving in the window is logged and counted (not
// fatal, not returned). If the window elapses with no rejection, Do returns
// (nil, nil): success. EVERY fire-and-forget outcome — success or
// rejection — is followed by a best-effort post-write quarantine drain (a
// fresh, caller-independent bounded context; see quarantineAfterWrite)
// before Do releases its mutex: a write is rare and high-stakes enough
// that paying up to ~2*QuietPeriod here, by design, to guarantee a late
// "?;" cannot leak into the NEXT Do call is an accepted cost. If instead
// ctx is cancelled while listening, e.suspect is set (no drain is
// attempted with the caller's own dead ctx — that work is deferred to the
// next Do's entry) and Do returns ctx's error. RetryReads MUST be 0 for a
// fire-and-forget spec (see CommandSpec.validate) — Do returns
// ErrInvalidSpec, without writing anything, otherwise. A fire-and-forget
// exchange is ALWAYS exactly one write: there is no retry path for it at
// all (safety obligation 2).
//
// Read (spec.ExpectPrefix != ""): after writing, Do waits up to
// spec.Timeout for the first frame matching ExpectPrefix (and ExpectLen,
// when non-zero), which is returned. A "?;" arriving first returns
// cat.ErrRejected (not retried: a rejection is a definitive answer, not an
// ambiguous silence). Any OTHER, non-matching frame is logged and counted,
// then Do keeps waiting within the SAME Timeout budget. If Timeout elapses
// with nothing matching, Do quarantines the port (DrainToQuiet) and retries
// — re-transmitting cmd, per safety obligation 1's rule, exactly once per
// attempt — up to spec.RetryReads additional times, since a lost or
// corrupted READ is always safe to resend (idempotent). If EVERY attempt
// times out (retries exhausted), Do sets e.suspect before returning
// ErrTimeout: the matching answer may still be in flight, and the entry
// quarantine on the NEXT Do call is what prevents it from being mistaken
// for that call's own answer (this is the fix for the worst-case finding:
// a slow reply arriving after a read's final timeout must never be
// consumed by a later, different read). A ctx cancellation observed while
// waiting ALSO sets e.suspect (the frame was already written; the
// outcome is unknown) and returns ctx's error immediately, without
// retrying.
//
// If ctx is already cancelled/expired, Do fails immediately with ctx's own
// error and writes nothing at all — checked both at entry (before the
// mutex is even taken) and again after the suspect/entry drains above,
// since neither of those consults the caller's ctx (the suspect drain uses
// its own fresh, caller-independent context; the purge never waits), so a
// ctx that died while either ran would otherwise slip through undetected.
// A retry attempt (attempt > 0, read specs only) is already covered
// without a dedicated check: its own drainToQuietLocked(ctx) call uses the
// caller's ctx directly.
//
// If the port is already known contaminated (ErrContaminated: see
// doc.go) or closed (ErrPortClosed), Do fails immediately without writing
// anything (checked before the entry quarantine above). If the entry
// quarantine's own drain fails (e.g. the port closes mid-drain), Do
// returns an error satisfying errors.Is(err, ErrQuarantineFailed),
// likewise without writing anything.
func (e *Engine) Do(ctx context.Context, cmd cat.Command, spec CommandSpec) ([]byte, error) {
	if err := spec.validate(); err != nil {
		return nil, err
	}
	spec = spec.withDefaults()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed.Load() {
		return nil, e.closedErr()
	}
	if e.contaminated.Load() {
		return nil, ErrContaminated
	}

	if e.suspect {
		qctx, cancel := quarantineContext()
		err := e.drainToQuietLocked(qctx)
		cancel()
		if err != nil {
			return nil, wrapQuarantineFailedErr(err)
		}
		// e.suspect is cleared by drainToQuietLocked itself on success.
	}
	if err := e.purgeBufferedLocked(); err != nil {
		return nil, err
	}
	// Re-check: the suspect/entry drains above use a FRESH,
	// caller-independent context (quarantineContext) and purgeBufferedLocked
	// never waits at all, so neither one would have observed ctx dying
	// while they ran. Without this second check, a ctx that was still
	// alive at entry but died during one of those drains would sail
	// straight through to the write below on attempt 0 — see Do's doc
	// comment; a subsequent retry (attempt > 0) is already covered, since
	// its own drainToQuietLocked(ctx) call uses the caller's ctx directly.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	maxAttempts := 1
	if spec.ExpectPrefix != "" {
		maxAttempts += spec.RetryReads
	}

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			if err := e.drainToQuietLocked(ctx); err != nil {
				return nil, err
			}
		}

		// Safety obligation 1: exactly one Bytes() call per transmission,
		// the SAME slice checked and written — never two independently
		// obtained copies.
		frame := cmd.Bytes()
		if e.allow == nil {
			// Unreachable through NewEngine by construction (it refuses
			// an unconfigured dialect outright and takes allow from the
			// configured one it kept) — checked anyway, exactly as
			// ErrDisallowedCommand's own doc comment argues for the layer
			// below. Distinct sentinel: a missing gate is a composition
			// bug, not a fault of this frame.
			return nil, ErrNoAllowlist
		}
		if !e.allow(frame) {
			return nil, fmt.Errorf("%w: %s", ErrDisallowedCommand, cmd.String())
		}
		if _, err := e.port.Write(frame); err != nil {
			e.closePort(err)
			return nil, e.closedErr()
		}

		if spec.ExpectPrefix == "" {
			// Fire-and-forget: exactly one write, no retry loop
			// possible even in principle (maxAttempts is always 1 here
			// — see above, and CommandSpec.validate forbids
			// RetryReads != 0 reaching this branch at all).
			result, err := e.waitFireAndForget(ctx, spec.ErrorWindow)
			switch {
			case radioResponded(err):
				e.settle(spec.Settle)
				// Rule 1: every fire-and-forget OUTCOME (success or
				// rejection) is quarantined before the mutex is
				// released, so a late "?;" belonging to THIS write
				// can never leak into the next Do call.
				e.quarantineAfterWrite()
			case isCtxErr(err):
				e.suspect = true
			}
			return result, err
		}

		answer, err := e.waitForAnswer(ctx, spec)
		if err == nil {
			e.settle(spec.Settle)
			return answer, nil
		}
		if errors.Is(err, ErrTimeout) {
			lastErr = err
			continue
		}
		if isCtxErr(err) {
			e.suspect = true
			return nil, err
		}
		if radioResponded(err) {
			e.settle(spec.Settle)
		}
		return nil, err // ErrRejected, ErrContaminated, ErrPortClosed — none retried
	}
	// Every attempt timed out: retries (if any) are exhausted. The
	// matching answer may still be in flight — mark suspect so the NEXT
	// Do's entry quarantines the stream before trusting anything it
	// receives as ITS OWN answer (see doc.go and the finding this fixes).
	e.suspect = true
	return nil, lastErr
}

// isCtxErr reports whether err is ctx's own cancellation/deadline error,
// as opposed to any other failure (ErrTimeout, ErrRejected, a closed or
// contaminated port). Do uses this to distinguish "the caller gave up
// early, mid-wait, leaving the exchange's true outcome unknown" — which
// sets e.suspect — from every other, definitively-attributable outcome.
func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// quarantineContext returns a bounded context, INDEPENDENT of any
// caller's ctx, used for both the post-write quarantine drain
// (quarantineAfterWrite) and the entry-time suspect drain in Do: using a
// fresh context here — rather than the caller's own — means neither a
// caller's already-cancelled ctx (entry drain) nor an unrelated,
// possibly very long deadline (post-write drain) can distort how long
// Engine spends quarantining the stream. The bound, 2*QuietPeriod, gives
// drainToQuietLocked room for one genuine QuietPeriod of silence even if
// a single stale frame arrives partway through and postpones "quiet"
// once; a caller-independent cap still exists so a pathological,
// continuously-arriving stream cannot hang either drain forever (see
// drainToQuietLocked's doc comment on the theoretical starvation case).
func quarantineContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*QuietPeriod)
}

// quarantineAfterWrite runs a best-effort drain-to-quiet, using a fresh
// quarantineContext, after a fire-and-forget exchange has produced a
// DEFINITIVE outcome (success or a "?;" rejection — see radioResponded).
// It is called from Do while e.mu is still held, before the mutex is
// released, and its own failure is logged rather than returned: the
// fire-and-forget Do call's result is already determined and must not
// change because a best-effort quarantine afterwards happened not to
// complete. A failure here also sets e.suspect, so the NEXT Do's entry
// runs a full drain-to-quiet before trusting anything as its own answer —
// exactly the same discipline a terminal read timeout or a ctx
// cancellation observed mid-write already gets (see Do's doc comment).
// This is harmless to set unconditionally even when the drain's failure
// was actually e.closed or e.contaminated (rather than the stream simply
// never going quiet within budget): the next Do call checks both of
// those FIRST, before ever consulting e.suspect, so nothing is silently
// lost — see doc.go.
func (e *Engine) quarantineAfterWrite() {
	qctx, cancel := quarantineContext()
	defer cancel()
	if err := e.drainToQuietLocked(qctx); err != nil {
		e.suspect = true
		e.logger.Printf("transport: post-write quarantine drain: %v", err)
	}
}

// purgeBufferedLocked non-blockingly drains every frame ALREADY sitting
// in e.events, logging and counting each as unexpected (safety
// obligation 3) — Do calls this at entry, after any suspect drain,
// before transmitting anything. Unlike drainToQuietLocked this never
// waits: it costs nothing when e.events is empty (the common case), and
// closes most of the "late reply from an abandoned exchange" gap for
// free, since anything already buffered here cannot possibly be the
// answer to the command Do is about to send. A terminal reader error
// found this way (contamination or the port going away) is handled
// exactly as it would be anywhere else and returned to the caller.
func (e *Engine) purgeBufferedLocked() error {
	for {
		select {
		case ev := <-e.events:
			if ev.err != nil {
				return e.handleReaderErr(ev.err)
			}
			e.countUnexpected(ev.frame)
		default:
			return nil
		}
	}
}

// radioResponded reports whether err represents a DEFINITIVE response from
// the radio to the exchange just attempted — success (nil) or a "?;"
// rejection — as opposed to no response at all (ErrTimeout: nothing to
// pace against, and a retry already gets DrainToQuiet's quarantine as its
// pacing) or a condition that isn't about THIS exchange at all
// (ErrContaminated, ErrPortClosed, ctx cancellation). Settle's pacing
// exists to give the radio recovery time after it has actually done
// something — parsing and responding to a command, whether that response
// was an answer or a rejection, both consume the radio's attention — so
// both are settled; the others are not.
func radioResponded(err error) bool {
	return err == nil || errors.Is(err, cat.ErrRejected)
}

// waitOutcome is nextEvent's result: exactly one of (an event), (the
// timeout channel fired), or (a terminal err — closed/ctx-cancelled) is
// populated. Each caller of nextEvent interprets "the timeout fired"
// differently (ErrTimeout for a read, success for fire-and-forget, "quiet
// achieved" for DrainToQuiet), so nextEvent itself stays neutral about it.
type waitOutcome struct {
	ev       readerEvent
	hasEvent bool
	timedOut bool
	err      error
}

// nextEvent waits for the next readerEvent, prioritising anything ALREADY
// buffered in e.events over every other case — including e.closeCh. This
// matters: Go's select picks pseudo-randomly among simultaneously ready
// cases, not in the order they became ready. Without this priority check, a
// reply that arrived (and was already sitting in the buffered e.events
// channel) an instant before Close/a disconnect fires could occasionally
// lose that race against <-e.closeCh becoming ready, and be discarded in
// favour of ErrPortClosed even though the real, correct answer was right
// there waiting to be read — exactly the FaultDisconnect-right-after-a-
// successful-reply scenario this was caught by. Checking e.events with a
// non-blocking select FIRST, on every loop iteration, makes "drain
// everything already queued before honouring closure" deterministic rather
// than a coin flip.
func (e *Engine) nextEvent(ctx context.Context, timeout <-chan time.Time) waitOutcome {
	select {
	case ev := <-e.events:
		return waitOutcome{ev: ev, hasEvent: true}
	default:
	}

	select {
	case ev := <-e.events:
		// e.events is never closed (only the reader goroutine sends to
		// it, and it stops sending — via sendEvent's own closeCh check —
		// rather than closing the channel on shutdown), so a receive
		// here always yields a real value; there is no "!ok" case to
		// handle.
		return waitOutcome{ev: ev, hasEvent: true}
	case <-timeout:
		return waitOutcome{timedOut: true}
	case <-e.closeCh:
		return waitOutcome{err: e.closedErr()}
	case <-ctx.Done():
		return waitOutcome{err: ctx.Err()}
	}
}

// waitForAnswer waits, within spec.Timeout, for a frame matching spec (a
// read's answer), a "?;" rejection, or one of the terminal conditions
// (contamination, port closed, context cancellation). Non-matching frames
// are logged, counted, and do not consume any extra time budget beyond the
// single spec.Timeout window already in force.
func (e *Engine) waitForAnswer(ctx context.Context, spec CommandSpec) ([]byte, error) {
	timeout := e.clk.After(spec.Timeout)
	for {
		out := e.nextEvent(ctx, timeout)
		if out.timedOut {
			return nil, ErrTimeout
		}
		if !out.hasEvent {
			return nil, out.err
		}
		ev := out.ev
		if ev.err != nil {
			return nil, e.handleReaderErr(ev.err)
		}
		if cat.IsRejection(ev.frame) {
			return nil, cat.ErrRejected
		}
		if spec.matches(ev.frame) {
			return ev.frame, nil
		}
		e.countUnexpected(ev.frame)
	}
}

// waitFireAndForget listens for spec.ErrorWindow after a fire-and-forget
// write: a "?;" arriving in that window is a rejection; any other frame is
// logged and counted but does not end the wait early; the window elapsing
// with nothing rejecting is success ((nil, nil)).
func (e *Engine) waitFireAndForget(ctx context.Context, window time.Duration) ([]byte, error) {
	timeout := e.clk.After(window)
	for {
		out := e.nextEvent(ctx, timeout)
		if out.timedOut {
			return nil, nil
		}
		if !out.hasEvent {
			return nil, out.err
		}
		ev := out.ev
		if ev.err != nil {
			return nil, e.handleReaderErr(ev.err)
		}
		if cat.IsRejection(ev.frame) {
			return nil, cat.ErrRejected
		}
		e.countUnexpected(ev.frame)
	}
}

// DrainToQuiet reads and discards (logging and counting each as
// unexpected) every frame arriving on the port until a full QuietPeriod
// passes with no activity at all — no frame, no accumulator error. On
// success it clears the CONTAMINATED state, if set (see doc.go). Serialised
// against Do by e.mu, exactly like any other exchange.
func (e *Engine) DrainToQuiet(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.drainToQuietLocked(ctx)
}

// drainToQuietLocked is DrainToQuiet's body, callable from Do (already
// holding e.mu) without recursive locking. It is also the mechanism
// behind Do's entry-time suspect drain and its post-write quarantine
// drain (quarantineAfterWrite) — both simply call this with a fresh,
// bounded context (see quarantineContext) instead of DrainToQuiet's
// caller-supplied one.
//
// On success (a full QuietPeriod observed with no activity) this clears
// BOTH the CONTAMINATED state (see doc.go) and e.suspect: whichever
// caller invoked the drain, a genuinely quiet stream means any
// previously-uncertain exchange has now definitively finished — there is
// nothing further it could deliver.
//
// Theoretical starvation: a continuous flood of inbound frames could, in
// principle, keep postponing "quiet achieved" forever by repeatedly
// resetting the timer below before it fires. In practice this is bounded
// twice over: cat.FrameAccumulator's own maximum frame length means
// endless non-terminated noise trips CONTAMINATED (a *cat.FrameTooLongError)
// long before it could sustain this indefinitely, and a real link running
// at the reference's 38400 baud simply cannot deliver "constant traffic,
// forever" in the first place. Combined with quarantineContext's
// independent, caller-agnostic deadline for the two internal call sites
// above, no drain can hang forever even in the pathological case.
func (e *Engine) drainToQuietLocked(ctx context.Context) error {
	if e.closed.Load() {
		return e.closedErr()
	}

	// A single reusable timer (Reset pattern), rather than a fresh
	// e.clk.After(QuietPeriod) allocated on every iteration: any activity
	// just postpones "quiet" by re-arming the SAME timer.
	timer := time.NewTimer(QuietPeriod)
	defer timer.Stop()
	for {
		out := e.nextEvent(ctx, timer.C)
		if out.timedOut {
			e.contaminated.Store(false)
			e.suspect = false
			return nil
		}
		if !out.hasEvent {
			return out.err
		}
		ev := out.ev
		if ev.err != nil {
			if errors.Is(ev.err, cat.ErrFrameTooLong) {
				e.logger.Printf("transport: drain: frame too long, re-quarantining: %v", ev.err)
			} else {
				e.closePort(ev.err)
				return e.closedErr()
			}
		} else {
			e.countUnexpected(ev.frame)
		}
		// Any activity postpones "quiet": re-arm the timer. nextEvent's
		// own priority check (see its doc comment) means the timer may
		// occasionally have already fired even though we get here via
		// the event branch — e.g. both became ready in the same instant
		// — so Stop's return value is checked and the channel drained
		// before Reset, per the standard library's documented pattern
		// for safely reusing a timer.
		if !timer.Stop() {
			<-timer.C
		}
		timer.Reset(QuietPeriod)
	}
}

// handleReaderErr turns a readerEvent's err (either a *cat.FrameTooLongError
// or a raw I/O error) into the error Do should return, updating engine
// state (contaminated, or closed) as a side effect.
func (e *Engine) handleReaderErr(err error) error {
	var tooLong *cat.FrameTooLongError
	if errors.As(err, &tooLong) {
		e.contaminated.Store(true)
		e.logger.Printf("transport: stream contaminated: %v", err)
		return wrapContaminatedErr(tooLong)
	}
	e.closePort(err)
	return e.closedErr()
}

// countUnexpected logs and counts frame as an unexpected frame — safety
// obligation 3: surfaced, never silently discarded.
func (e *Engine) countUnexpected(frame []byte) {
	e.unexpectedFrames.Add(1)
	e.logger.Printf("transport: unexpected frame received: %q", frame)
}

// settle sleeps d via the engine's clock — the post-exchange pacing delay,
// applied before Do releases its mutex.
func (e *Engine) settle(d time.Duration) {
	e.clk.Sleep(d)
}

// readLoop is the Engine's own goroutine: it owns every Read() call against
// the Port, reassembles frames via a cat.FrameAccumulator, and delivers
// each complete frame (or terminal error) to e.events for whichever
// Do/DrainToQuiet call currently holds e.mu. It is the ONLY goroutine that
// ever reads the Port.
func (e *Engine) readLoop() {
	defer close(e.readerDone)

	acc := cat.NewFrameAccumulator(e.maxFrame)
	buf := make([]byte, 4096)
	for {
		n, err := e.port.Read(buf)
		if n > 0 {
			frames, ferr := acc.Push(buf[:n])
			for _, f := range frames {
				if !e.sendEvent(readerEvent{frame: f}) {
					return
				}
			}
			if ferr != nil {
				if !e.sendEvent(readerEvent{err: ferr}) {
					return
				}
			}
		}
		if err != nil {
			// closePort FIRST: it closes closeCh, which is what
			// guarantees sendEvent below cannot block forever even if
			// e.events is momentarily full and nobody is draining it —
			// sendEvent's select falls through to <-e.closeCh instead.
			// A waiter that misses this specific event still observes
			// closeCh closing directly in its own select and returns
			// the same e.closedErr(), so nothing is lost by ordering it
			// this way.
			e.closePort(err)
			e.sendEvent(readerEvent{err: err}) // best-effort
			return
		}
	}
}

// sendEvent delivers ev to e.events, or reports false if e.closeCh fired
// first — readLoop uses this to guarantee it never blocks past Close, even
// if nobody is currently consuming e.events (an idle period between Do
// calls).
func (e *Engine) sendEvent(ev readerEvent) bool {
	select {
	case e.events <- ev:
		return true
	case <-e.closeCh:
		return false
	}
}

// closePort marks the engine closed with cause (nil for an explicit
// Engine.Close call), closes closeCh, and closes the underlying Port —
// exactly once, however many times or from however many goroutines it is
// called (readLoop, on a terminal read error, and the exported Close, in
// either order or concurrently — sync.Once serialises them and guarantees
// every caller only returns once the FIRST call's body has fully run, so
// portCloseErr is always safe to read immediately afterwards).
//
// Releasing the OS-level Port as soon as ANY closure cause is known — not
// only when a caller happens to call Engine.Close — matters for a real
// serial device: a spontaneous disconnect a caller never explicitly
// acknowledges with Close must not leak the file descriptor forever.
func (e *Engine) closePort(cause error) {
	e.closeOnce.Do(func() {
		if cause != nil {
			e.closeCause.Store(&cause)
		}
		e.closed.Store(true)
		close(e.closeCh)
		e.portCloseErr = e.port.Close()
	})
}

// closedErr builds the error Do/DrainToQuiet return once the engine is
// closed, wrapping whatever cause closePort recorded (nil for an explicit
// Close call).
func (e *Engine) closedErr() error {
	if p := e.closeCause.Load(); p != nil {
		return wrapClosedErr(*p)
	}
	return wrapClosedErr(nil)
}

// Close shuts the Engine down: unblocks any in-flight Do/DrainToQuiet call
// with ErrPortClosed, closes the underlying Port, and joins the reader
// goroutine before returning — no goroutine is left running once Close
// returns. Idempotent: safe to call more than once (every call returns the
// same result — the Port's Close() is only ever actually invoked once,
// whether triggered by this call or by a spontaneous disconnect readLoop
// observed first).
func (e *Engine) Close() error {
	e.closePort(nil)
	<-e.readerDone
	return e.portCloseErr
}

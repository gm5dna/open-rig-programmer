// SPDX-License-Identifier: GPL-3.0-or-later

// Package transport is the one place raw bytes meet the wire: serial port
// discovery (Discover, rankPorts) and the transaction engine (Engine) that
// gives every higher layer safe request/response semantics over a radio
// control protocol.
//
// # The framing seam (M9d-2, D2)
//
// Engine is generalised over a Framing rather than bound to Yaesu CAT.
// Everything below about the state machine — single outstanding request,
// expected-answer matching, error windows, the never-resend-a-write rule,
// inter-transaction quarantine, CONTAMINATED — is UNCHANGED and belongs to
// Engine. What moved behind the seam is the six things that differ between
// a ';'-terminated CAT stream and Icom's binary FE FE … FD one:
//
//   - Framing.NewAccumulator — where a frame begins and ends.
//   - Framing.IsRejection — what the protocol's NAK looks like ("?;", FA).
//   - Framing.Allow — the outbound write gate (AllowFunc, unchanged).
//   - Framing.InitSequence — what a session opens with. CAT: [AI0;].
//     CI-V: EMPTY, which is a safety property, not an omission (D2,
//     adjudication 3) — nothing this tier sends to an Icom radio mutates
//     it, so a session opens without touching its settings.
//   - Framing.DrainPolicy — the idle gap and the ABSOLUTE cap every
//     drain, purge, quarantine and answer wait is bounded by.
//   - Framing.NoteSent — called BEFORE each port write, so an
//     echo-removing accumulator (CI-V's bus and USB echo) has the frame
//     recorded before its own echo can arrive.
//
// Two things moved into CommandSpec alongside it. ANSWER MATCHING is now
// an opaque Match the CODEC builds (cat.PrefixLenMatcher for CAT; to/from/
// cn/sc for CI-V), because the matching rule belongs to the protocol that
// defines it and Engine has no business knowing what a prefix is. And the
// COMMAND CLASS is now explicit — see "Command classes are stated, not
// inferred" below.
//
// NewEngine(p, cat.Dialect, opts...) survives unchanged as a thin wrapper
// over NewEngineWith(p, Framing, opts...); the CAT adapter (catFraming) is
// in this package, because this package already imports core/cat for
// exactly these things and the reverse import would cycle. Wire behaviour
// for CAT is byte-identical across the change: the same frames, the same
// prefixes and lengths, the same timings.
//
// # The no-sequence-numbers problem
//
// The wire protocol (core/cat) has no request IDs, no sequence numbers, and
// no acknowledgement for a Set (write) command that succeeds — the
// reference's only NAK is a single, unattributed "?;" that means "the LAST
// thing you sent was rejected," with no way to know which of several
// outstanding commands it refers to if more than one were ever in flight at
// once. There is also no way to distinguish "the radio is slow" from "the
// radio never received the command" from "the reply was lost in transit":
// every failure mode looks identical from the host side — silence.
//
// Engine's entire design exists to make this tractable:
//
//   - Single outstanding request. Engine.mu serialises every Do and
//     DrainToQuiet call, so at most one command is ever unanswered at a
//     time. This is what makes "the next frame that arrives is (probably)
//     the answer to what I just sent" a safe assumption at all — without
//     it, an unattributed "?;" or answer frame could belong to ANY
//     previously sent, still-outstanding command.
//
//   - Expected-answer matching, not blind trust. A frame is only accepted
//     as an answer if the CommandSpec's own Match — the codec's rule,
//     opaque here — says it is. Anything else — a stray push, a reply to a
//     command some earlier, now-abandoned attempt sent, chunking noise —
//     is logged, counted (Engine.UnexpectedFrames), and otherwise ignored:
//     surfaced, never silently discarded (safety obligation 3).
//
//   - Error windows for fire-and-forget. A Set command (e.g. MW) gets no
//     positive acknowledgement on success per the reference — "silence
//     means it worked." Engine.Do still LISTENS for a bounded ErrorWindow
//     after writing, in case a "?;" arrives late, before declaring
//     success. This is a best-effort safety net, not a guarantee: a
//     rejection that arrives after the window has already elapsed is
//     indistinguishable from success as far as THIS Do call is concerned.
//
//   - The never-resend-a-write rule (safety obligation 2). Because a lost
//     write's outcome is genuinely ambiguous — did the radio process it
//     and just fail to (or not get a chance to) tell us, or did the
//     command never arrive at all? — resending is NOT safe for anything
//     that isn't idempotent. Engine.Do enforces this structurally:
//     CommandSpec.RetryReads is only meaningful (and only permitted to be
//     nonzero) for ClassRead; setting it on EITHER write class is a
//     validation error (ErrInvalidSpec), and even for a read that
//     legitimately retries, obligation 1's write-what-was-checked rule
//     applies to EVERY attempt independently.
//
//   - Inter-transaction quarantine. Single outstanding request (above)
//     only guarantees that at most one exchange is EVER in flight at
//     once — it says nothing about a reply that outlives the Do call
//     that sent it. A "?;" arriving after a fire-and-forget's
//     ErrorWindow, or a matching answer arriving after a read's final
//     Timeout, is still sitting on the wire, addressed to nobody, when
//     the NEXT Do call starts; without further discipline it would be
//     consumed as THAT call's own answer — in the worst case, one
//     read's stale reply mistaken for a completely different,
//     same-shape read's data (e.g. two consecutive reads of different
//     memory slots). Do closes this gap on every path a reply can take:
//
//     1. Purge at entry. Before transmitting anything, every Do call
//     non-blockingly drains whatever is ALREADY buffered — cost-free
//     when there is nothing there, which is the common case — logging
//     and counting each frame found as unexpected (obligation 3).
//
//     2. Write-path: quarantine is unconditional, for BOTH write
//     classes. A write Do ALWAYS runs a full drain-to-quiet immediately after its
//     outcome (success or rejection) is known, using a fresh context
//     independent of the caller's — before releasing the mutex, and
//     before any other Do can run. Write commands are rare and
//     high-stakes enough that paying up to the framing's
//     DrainPolicy.Cap here, by design, to guarantee a late rejection can
//     never leak into the next transaction is an accepted cost.
//
//     3. Read-path: quarantine is deferred via a "suspect" marker.
//     Blocking every read behind a mandatory post-exchange drain would
//     make the common case (a read that succeeds well within Timeout)
//     needlessly slow. Instead, Engine marks itself suspect ONLY when a
//     read's outcome is left genuinely uncertain — every retry
//     exhausted on a terminal timeout, or a ctx cancellation observed
//     after the frame was already written — and the NEXT Do call, of
//     ANY kind, runs a full drain-to-quiet (again, a fresh,
//     caller-independent context) at entry, before transmitting,
//     clearing the marker only once that drain actually succeeds. A
//     caller whose own ctx is already dead when suspect-worthy is never
//     asked to drain with it — the work is deferred to whichever Do
//     runs next.
//
//     Together, 1-3 mean a stale reply belonging to an abandoned
//     exchange is drained — for free if already buffered, or by an
//     explicit wait if not — before Engine will trust anything it next
//     receives as the answer to a NEW command. The one residual window
//     no purely software discipline over an unattributed protocol can
//     close: a frame arriving DURING an already-in-flight next
//     transaction (i.e. after its own quarantine/purge has already run,
//     but before its own answer arrives) is, by the wire protocol's
//     total absence of sequence numbers, fundamentally unattributable —
//     Engine cannot know it is stale rather than a chunked/delayed
//     piece of the current exchange. What the drain discipline above
//     guarantees is that reaching this residual window at all requires
//     a reply arriving more than QuietPeriod late relative to when it
//     was expected; M5a's real CAT timing measurements are what will
//     bound how likely that is in practice against physical hardware.
//
// # The CONTAMINATED state
//
// The framing's Accumulator enforces a maximum frame length; if the byte
// stream ever exceeds it without completing a legitimate frame (a wedged
// or noisy line, or genuinely non-protocol data), it reports a
// *FrameTooLongError and resets itself. Engine treats this as entering
// a CONTAMINATED state: every subsequent Do call fails immediately with an
// error satisfying errors.Is(err, ErrContaminated) — WITHOUT writing
// anything to the port — because the accumulator's own internal buffer
// reset means Engine can no longer be confident where a frame boundary
// actually is in whatever the radio sends next; writing blind into that
// uncertainty risks compounding it.
//
// The only way out of CONTAMINATED is a successful DrainToQuiet: once a
// full DrainPolicy.IdleGap has passed with no further frames or errors
// arriving, Engine trusts that the line has settled and clears the state.
// Init calls this automatically after its init sequence; a caller that
// observes ErrContaminated from Do is expected to call DrainToQuiet itself
// before trying again. A drain that reaches its absolute cap without ever
// seeing that gap fails with ErrDrainCapExceeded and clears nothing — see
// "Starvation deadlines" below.
//
// # Command classes are stated, not inferred
//
// CommandSpec.Class is explicit on every spec and fails closed: the ZERO
// value describes no exchange and Do refuses it with ErrInvalidSpec,
// having written nothing.
//
// Before D2 the class was INFERRED from ExpectPrefix — empty meant a
// fire-and-forget write, non-empty meant a read. That made "this command
// mutates a radio's memory and must never be retransmitted" and "the
// author left a field blank" the SAME VALUE, which is a heavy claim to
// rest on an absence. It also had no room for a third case: CI-V
// acknowledges a write with FB, which no prefix could have keyed.
//
//   - ClassRead — idempotent. Retryable per RetryReads.
//   - ClassWrite — fire-and-forget. Exactly one write, never
//     retransmitted, a bounded ErrorWindow spent listening for a late
//     rejection, an unconditional post-write quarantine.
//   - ClassWriteWithAck — waits for the codec's acknowledgement answer,
//     with NO retransmission on timeout. The write quarantine applies
//     exactly as it does to ClassWrite. CAT has no acknowledged write
//     form, so no Yaesu driver builds one.
//
// The CAT spec-construction helpers (CATReadSpec, CATWriteSpec) are how
// every Yaesu driver states it. No constructor wrapper rewrites a
// later-built spec: the class is on the spec the caller passes, or the
// call is refused.
//
// # Starvation deadlines
//
// Every drain, purge, quarantine and answer wait carries an ABSOLUTE
// deadline, taken when it starts and honoured AHEAD of any queued event.
// The ordering is the mechanism, not a detail: nextEvent's
// buffered-events priority check (which exists for a real and different
// hazard — see its doc comment) would otherwise win every iteration
// against a stream that never pauses, so the timeout channel and
// ctx.Done() would never be selected on at all. A context deadline cannot
// fix that, because ctx.Done() lives in the very select the flood wins.
// Comparing the clock BEFORE touching the channel is what makes the bound
// hold at any arrival rate.
//
// WHAT MADE THIS NECESSARY, stated plainly because the previous reasoning
// here was not wrong so much as scoped to one radio family: against a
// Yaesu link that answers only what it is asked, "the line will go quiet"
// is a safe assumption, and the earlier note argued exactly that (the
// accumulator's length cap would trip on endless noise; 38400 baud cannot
// deliver constant traffic). Icom's transceive mode BROADCASTS
// unprompted, is factory-ON on at least four of the six models this tier
// registers, and this tier ships no off-switch. A well-formed frame every
// few milliseconds, forever, is that radio's normal operating condition —
// and every one of those frames re-arms a drain's idle timer without ever
// tripping the length cap.
//
// So a drain that never sees its idle gap fails with ErrDrainCapExceeded,
// which is deliberately NOT success: a stream that never pauses has
// established nothing about what is still in flight. Do's entry purge is
// bounded by a frame count and a deadline, and hitting either is NOT an
// error — the purge is best-effort by design, and failing the call there
// would mean a transceive-broadcasting radio could never be talked to at
// all. core/transport/flood_test.go holds the proofs, each against a port
// that never goes quiet, each with a watchdog because the failure mode is
// a hang.
//
// # Safety obligations (binding, from earlier reviews)
//
//  1. Engine.Do calls cmd.Bytes() EXACTLY ONCE per transmission attempt,
//     reports THAT byte slice to the framing (NoteSent), checks the
//     Engine's own gate (AllowFunc — since D2, the framing's Allow; for
//     CAT that is still the AllowedCommand of the cat.Dialect NewEngine
//     was given) on THAT SAME slice, and writes THAT SAME slice — never a
//     second, independently obtained copy. NoteSent's contract (copy what
//     you need, retain nothing) is what keeps the echo hook from
//     weakening this. The gate is fixed at construction and fail-closed at
//     both ends: NewEngine refuses an unconfigured dialect
//     (ErrUnconfiguredDialect) and NewEngineWith refuses a nil framing
//     (ErrNoFraming) before starting the reader goroutine, so neither can
//     RETURN an Engine that speaks for no radio, and Do refuses again
//     (ErrNoAllowlist) before every write regardless — which is what
//     covers the hand-built case the constructor cannot reach, Engine
//     being an exported type (M9b fix wave, Codex finding 3). See Do's
//     doc comment.
//  2. Retries are only for idempotent reads (CommandSpec.RetryReads,
//     ClassRead); a write's timeout or failure is NEVER resolved by
//     resending, for EITHER write class — enforced structurally via
//     ErrInvalidSpec, not merely by convention.
//  3. Unexpected frames are surfaced — logged via the injectable Logger
//     and counted (Engine.UnexpectedFrames) — never silently discarded.
//  4. OpenSerial drives RTS and DTR low immediately after opening the
//     underlying device (SetRTS(false), SetDTR(false)), 8-N-2 by default.
//     What the lines do in the brief window between the OS-level open and
//     these calls landing is unknown and left as an M5a hardware
//     observation item — see OpenSerial's doc comment.
//
// # The policy-gated write path (composition-root discipline)
//
// Engine.Do is MECHANISM: it will transmit any frame its own dialect's
// gate admits, including the Set frames (MW, MT) that mutate a
// radio's memory. It is not, and cannot be, a policy layer — the
// hardware write guard (the capability profiles, codeplug.Diff's gates,
// the clone service's choreography, and driver.Session.WriteChannel's
// own re-check) lives entirely above it. Within THIS repository,
// Engine.Do is therefore reached outside this package only from
// core/driver/** — enforced by the import-graph guard test
// (internal/guards), whose threat model is our own composition, not
// external importers. Both constructors are covered: since D2 the
// construction guard names NewEngine and NewEngineWith, and the .Do
// rule's pre-filter reads the same name set. The compiler-enforced version of this boundary (a
// separate write-capability split) is a ledgered M5b-flip precondition.
//
// # This package's core/cat dependency, and what D2 left of it
//
// M9b (the codec dialect seam) injected the write GATE and nothing else,
// leaving one hardwiring ledgered here: Engine.Init reached for the
// package-level cat.FT710 to build its AI frame while the gate came from
// the caller. M9c-5 (E3) CLOSED IT. NewEngine takes a cat.Dialect whole
// and derives both the gate (d.AllowedCommand) and the init frame
// (d.BuildAISet(false)) from that one value.
//
// THE CORRECTION THAT CAME WITH IT, because the old ledger note claimed
// more than was true: it said a second dialect whose gate did not admit
// the FT-710's AI0; form would make Init "fail closed". It never failed
// at all, and could not have. AllowedCommand judges BYTES, not
// provenance; every configured dialect builds exactly "AI0;", and every
// configured dialect's gate admits that form. There was no live defect
// here and no fixture could have shown one. What there was is
// architectural impurity — a type that already held a dialect reaching
// past it for a package-level one — and that is what was removed, with
// the bytes unchanged.
//
// D2 MOVED THE ENGINE OFF core/cat, BUT NOT THE PACKAGE. Engine's own
// code no longer names core/cat at all: the accumulator, the rejection
// test and the init frame all arrive through Framing. What still imports
// it, and deliberately:
//
//   - catFraming (catframing.go) — the CAT adapter itself, which is what
//     the delegation now goes through, plus the CATReadSpec/CATWriteSpec
//     helpers every Yaesu driver builds its specs with.
//   - NewEngine's cat.Dialect parameter, the CAT wrapper's whole reason
//     for existing.
//   - THE ERROR RE-EXPORTS, and this one is a recorded WART rather than
//     a design (D2, adjudication 1, round 2 F1). transport.ErrRejected,
//     transport.ErrFrameTooLong and transport.FrameTooLongError are
//     aliases of core/cat's. Neutral code and core/civ use the TRANSPORT
//     names; errors.Is/As at every existing call site still passes,
//     because these are the same values and the same type. But the
//     CANONICAL values still live in a package named for Yaesu, even
//     though a CI-V FA rejection now produces transport.ErrRejected too.
//     The direction is forced: this package already imports core/cat, so
//     the reverse alias would cycle, and re-exporting is the cycle-free
//     half of the pair. A later cleanup may move them to a leaf package
//     neither codec owns. Byte identity is claimed for WIRE BEHAVIOUR and
//     the recorded manifest recipes, NOT for the Go API — and this is one
//     of the places the Go API shows its history. See errors.go, where
//     the same note sits with the declarations.
//
// No package-level dialect VALUE is reached for anywhere in this
// package's production code: every outbound frame Do transmits arrives
// from its caller already built, the one frame Init builds for a CAT
// session comes from the dialect its framing was constructed with, and
// every frame is judged solely by that same framing's gate.
package transport

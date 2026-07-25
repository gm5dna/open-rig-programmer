// SPDX-License-Identifier: GPL-3.0-or-later

// Package transport is the one place raw bytes meet the wire: serial port
// discovery (Discover, rankPorts) and the transaction engine (Engine) that
// gives every higher layer safe request/response semantics over the FT-710
// CAT protocol.
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
//     as an answer if it matches the CommandSpec's ExpectPrefix (and
//     ExpectLen, when fixed). Anything else — a stray push, a reply to a
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
//     nonzero) when ExpectPrefix != "" (a read); setting it on a
//     fire-and-forget spec is a validation error (ErrInvalidSpec), and
//     even for a read that legitimately retries, obligation 1's
//     write-what-was-checked rule applies to EVERY attempt independently.
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
//     2. Write-path: quarantine is unconditional. A fire-and-forget
//     Do ALWAYS runs a full drain-to-quiet immediately after its
//     outcome (success or rejection) is known, using a fresh context
//     independent of the caller's — before releasing the mutex, and
//     before any other Do can run. Write commands are rare and
//     high-stakes enough that paying up to ~2*QuietPeriod here, by
//     design, to guarantee a late "?;" can never leak into the next
//     transaction is an accepted cost.
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
// cat.FrameAccumulator enforces a maximum frame length; if the byte stream
// ever exceeds it without completing a legitimate frame (a wedged or noisy
// line, or genuinely non-protocol data), it reports a
// *cat.FrameTooLongError and resets itself. Engine treats this as entering
// a CONTAMINATED state: every subsequent Do call fails immediately with an
// error satisfying errors.Is(err, ErrContaminated) — WITHOUT writing
// anything to the port — because the accumulator's own internal buffer
// reset means Engine can no longer be confident where a frame boundary
// actually is in whatever the radio sends next; writing blind into that
// uncertainty risks compounding it.
//
// The only way out of CONTAMINATED is a successful DrainToQuiet: once a
// full QuietPeriod has passed with no further frames or errors arriving,
// Engine trusts that the line has settled and clears the state. Init calls
// this automatically after establishing AI0; a caller that observes
// ErrContaminated from Do is expected to call DrainToQuiet itself before
// trying again.
//
// # Safety obligations (binding, from earlier reviews)
//
//  1. Engine.Do calls cmd.Bytes() EXACTLY ONCE per transmission attempt,
//     checks cat.AllowedCommand on THAT byte slice, and writes THAT SAME
//     byte slice — never a second, independently obtained copy. See Do's
//     doc comment.
//  2. Retries are only for idempotent reads (CommandSpec.RetryReads,
//     ExpectPrefix != ""); a write's timeout or failure is NEVER resolved
//     by resending — enforced structurally via ErrInvalidSpec, not merely
//     by convention.
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
// Engine.Do is MECHANISM: it will transmit any cat.AllowedCommand frame,
// including the Set frames (MW, MT) that mutate a radio's memory. It is
// not, and cannot be, a policy layer — the hardware write guard (the
// capability profiles, codeplug.Diff's gates, the clone service's
// choreography, and driver.Session.WriteChannel's own re-check) lives
// entirely above it. Within THIS repository, Engine.Do is therefore
// reached outside this package only from core/driver/** — enforced by the
// import-graph guard test (internal/guards), whose threat model is our
// own composition, not external importers. The compiler-enforced version
// of this boundary (a separate write-capability split) is a ledgered
// M5b-flip precondition.
package transport

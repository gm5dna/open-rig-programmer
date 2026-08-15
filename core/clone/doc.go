// SPDX-License-Identifier: GPL-3.0-or-later

// Package clone owns the end-to-end safety choreography for sending a
// codeplug file to a radio: reading a fresh baseline, diffing it against a
// candidate file, and writing only what changed — with every check an
// adversarial review of this project has demanded standing between a
// user's "yes, send it" and a single byte reaching the wire.
//
// clone is POLICY. Everything below it — core/driver (the write guard,
// ReadChannel/WriteChannel), core/codeplug (Validate, Diff, Digest, atomic
// Save), core/transport (the transaction engine) — is MECHANISM. It must
// be impossible to reach driver.Session.WriteChannel except through this
// package's choreography: Service.PrepareSend builds an immutable SendPlan
// from a fresh read, and only Service.Execute may act on one, after passing
// every refusal check below.
//
// # The twelve obligations
//
// These were extracted, one by one, from adversarial reviews of earlier
// milestones (see core/codeplug's Diff/Digest doc comments, which name the
// gap each obligation here closes) and are BINDING on this package's
// design:
//
//  1. Fresh baseline at send time. PrepareSend re-reads the radio in
//     full (ReadAll) every time it is called; the diff a user reviews is
//     always computed against that fresh read, never a stale session
//     read from earlier.
//
//  2. Immutable send plan. SendPlan holds PRIVATE deep copies of the
//     baseline and candidate channels, plus both their digests. It is
//     opaque outside this package — unexported fields, accessors only —
//     so nothing external can mutate what Execute will actually send.
//     Editing the file a caller passed to PrepareSend, after the fact,
//     can never change what a plan built from it will write.
//
//  3. Dual-digest recheck at the execute boundary. Execute recomputes
//     codeplug.Digest over the plan's own private baseline AND candidate
//     copies and compares both against the digests stored in the plan at
//     PrepareSend time (a self-consistency assertion — see
//     ErrStaleBaseline/ErrCandidateChanged). It also re-runs
//     codeplug.Validate on the candidate against the SESSION'S CURRENT
//     effective capabilities. Any mismatch is a typed refusal; nothing is
//     written.
//
//  4. Session identity binding. A SendPlan records the session's
//     Identity plus the Service's own generation counter (assigned once,
//     at NewService time). Execute refuses if either has changed —
//     content digests alone cannot detect a reconnect to a different
//     physical radio that happens to answer with the same CAT
//     ID/port/USB serial as before.
//
//  5. Confirmation binding. Execute requires the caller to pass the
//     exact SendPlan.ConfirmationDigest() for the plan being executed; a
//     mismatch is refused. This is DELIBERATELY NOT the plan's
//     BaselineDigest (Fix 1, adjudicated HIGH, closing a gap the original
//     obligation left open): two plans PrepareSend built from the SAME
//     baseline read but different candidate files always share
//     BaselineDigest, so checking against it alone would let a caller
//     confirm plan A's diff and then Execute plan B. ConfirmationDigest
//     folds in the candidate digest too, plus the session identity and
//     generation obligation 4 already binds independently, making the
//     confirmed value specific to the exact plan being executed. The
//     intended flow: display DiffResult (and, for a human-readable
//     receipt, BaselineDigest), obtain consent, then call Execute with
//     THAT plan's ConfirmationDigest() — never a freshly-recomputed one,
//     and never another plan's.
//
//  6. Delta-write only. Only Added/Modified, unblocked diff entries are
//     ever written. Blocked entries (including every Erased entry — see
//     obligation 10's sibling, the hardware write gate: erase is blocked
//     project-wide in v1) are never written; they are counted and
//     reported as skipped, with their reason. Unchanged entries are
//     untouched.
//
//  7. Per-channel write-then-verify. After each WriteChannel, Execute
//     reads the same slot back (ReadChannel) and compares the WRITABLE
//     fields against what was sent — CTCSSTone/ScanSkip are excluded
//     from this comparison, since they read back Unknown by construction
//     (the CAT protocol cannot read them at all). Any mismatch, or any
//     ambiguity (a read-back error), aborts immediately: no further
//     writes are attempted.
//
//  8. Append-only journal. Every step — prepare (including the snapshot
//     path, and "consented_unverified": whether this SESSION'S
//     capabilities carry a consented-unverified write label anywhere,
//     session-wide rather than scoped to the fields this plan writes —
//     see capsConsented), each per-channel write attempt, each verify
//     result, an abort, and completion — is appended as one JSON line,
//     fsync'd, BEFORE the action's outcome is relied upon for the next
//     decision. The journal file lives beside the snapshot
//     (SnapshotStore).
//
//  9. Snapshot before anything. PrepareSend saves the fresh baseline
//     read as a codeplug JSON snapshot (an atomic codeplug.Save) to a
//     caller-supplied SnapshotStore directory, named
//     "snapshot-<model>-<catid>-<timestamp>.orp.json", BEFORE returning
//     a plan to the caller. The timestamp comes from an injected
//     Now func() time.Time (WithNow), for determinism in tests.
//
//  10. First-write interactive gate. Execute takes
//     ExecuteOptions.FirmwareConfirmed — the caller must supply a
//     non-empty, user-confirmed firmware version string (read off the
//     radio's front panel; CAT has no command to read it) before this
//     Service's session performs its first Execute. An empty value is a
//     typed refusal. This layer enforces PRESENCE only; obtaining the
//     confirmation from a human is the CLI/GUI's job. The audit clause:
//     the confirmed value is not merely gate-keeping state — the first
//     time it is accepted, Execute journals a "firmware_confirmed" event
//     carrying it (before the delta loop, alongside "prepare" in the
//     journal's timeline, not interleaved with per-slot write/verify
//     lines), and every Report from then on (this call's and every later
//     one on the same Service) carries it in Report.FirmwareConfirmed, so
//     a caller can display or persist it without having to have been the
//     one that supplied it.
//
//  11. Radio-side drift, caught before any write. Obligations 1-10 bind
//     the plan to what PrepareSend saw and to what the user confirmed —
//     but say nothing about the radio's memory changing AGAIN between
//     PrepareSend and Execute (a second tool, a front-panel edit, a
//     second clone session against the same radio). For EACH slot the
//     delta-write loop is about to write, Execute first re-reads that
//     SAME slot and compares it against the plan's own baseline for it.
//     Any drift refuses the whole execute (ErrStaleBaseline, naming the
//     slot) before a single WriteChannel call is made for it.
//
//     M3 Codex-review fix wave, Fix 3 (closing the drift-to-write
//     window): this check runs IMMEDIATELY before that slot's own write
//     — inside the SAME per-slot iteration, not as a separate up-front
//     phase that re-reads every to-be-written slot before writing ANY of
//     them. The up-front-phase design left a real gap: a slot's baseline
//     could be confirmed clean, but the radio's memory could still drift
//     AGAIN while every OTHER delta's verify-read ran, before that
//     slot's own write finally happened minutes (or, for a
//     multi-hundred-channel plan, seconds) later. Checking and writing
//     one slot at a time, back to back, shrinks that window to as small
//     as this package can physically make it: the time between one
//     ReadChannel call and the WriteChannel call immediately after it,
//     for the SAME slot, with no other slot's I/O in between.
//
//  12. Memory-selection snapshot/restore, best-effort. HW-CONFIRMED
//     2026-07-13 (M5b write trials, docs/hardware-notes.md): an MW write
//     moves the radio's selection to the written slot, hands-off — a
//     multi-slot delta-write loop therefore drags the radio's operating
//     selection through every slot it writes. Execute snapshots the
//     selection (an MC query) once, AFTER obligations 3/4/5/10's refusal
//     checks and BEFORE the delta-write loop's first write, and
//     best-effort recalls it (an MC-set) on the way out however the loop
//     ends — success, an abort, or a cancelled ctx. Both the snapshot and
//     the restore are journaled ("mc_snapshot", "mc_restore"), and BOTH
//     are best-effort: a session whose driver.Session does not expose
//     this (core/clone.MemorySelector, a type assertion — not every
//     driver need implement it) simply never snapshots; an unparseable
//     snapshot answer (the "000" VFO-state case remains UNTESTED —
//     core/cat/mc.go's doc comment) skips the restore with a journal
//     note rather than guessing a recall target; and a restore FAILURE is
//     a journal warning, never an abort — by the time it runs, every
//     write this Execute call attempted has already passed its own
//     write-then-verify (obligation 7), so the selection restore is a
//     courtesy on top of that guarantee, not a substitute for it.
//
// # Journal durability policy (ratified)
//
// Obligation 8 says every step is journaled; it does not, by itself, say
// what happens when the journal append ITSELF fails. That gap was closed
// by adjudicated review and is now ratified policy, recorded here
// verbatim:
//
//	"The intent-to-write (write_attempt) journal line is FAIL-SAFE: if
//	it cannot be persisted (append or fsync error), Execute aborts
//	BEFORE calling WriteChannel — at that instant nothing has touched
//	the radio, so refusal is free. Journal-append failures AFTER a wire
//	write cannot un-write; they abort all FURTHER writes (standard
//	abort machinery) with the journal failure as the abort reason.
//	PrepareSend's journal lines are likewise fail-safe (refuse to
//	produce a plan without a journal)."
//
// Concretely, in this package: PrepareSend's "prepare" line failing to
// append means PrepareSend returns an error instead of a plan — see
// PrepareSend. Execute's "write_attempt" line failing to append means
// Execute refuses that slot before ever calling WriteChannel — the
// per-slot write/verify pair never starts, so no partial state is
// possible for THAT slot (see Service.appendDeltaJournal). Execute's
// "write_result" and "verify_result" lines are both recorded AFTER a
// wire action has already happened (WriteChannel, or the paired verify
// ReadChannel); a failure appending either cannot undo that action, so it
// is counted in the Report exactly as it truly happened, and Execute
// aborts via the same *AbortedError/*Report machinery a write rejection
// or verify mismatch uses — with a *JournalFailedError (see
// ErrJournalFailed) as the abort's Cause. The terminal "abort" and
// "completion" lines, and the best-effort "firmware_confirmed" line
// (its gate state already lives safely in memory the instant it is set —
// see obligation 10), remain best-effort: there is nothing further left
// to protect by refusing at that point, so a durability hiccup on one of
// those is logged, not surfaced as a failure of an already-decided
// outcome.
//
// # Execute's phase order
//
// Refusal checks (obligations 3, 4, 5, 10, in that order) -> obligation 6's
// diff partition (Blocked entries counted and skipped, Unchanged entries
// counted, unblocked Added/Modified entries become the delta list; an
// EMPTY delta list returns immediately here — no memory-selection
// snapshot, since there will be no write to move it) -> obligation 12's
// memory-selection snapshot (only when the delta list is non-empty) ->
// delta-write loop (for EACH entry, in order: obligation 11's verify-read,
// then obligation 7's write-then-verify, then obligation 8's journaling,
// with progress) -> obligation 12's best-effort memory-selection restore
// (deferred, so it runs on every exit from the delta-write loop below it:
// completion, an abort, or a cancelled ctx) -> report. As of the M3
// Codex-review fix wave's Fix 3, obligation 11 is no longer a separate
// up-front phase — see obligation 11's own text above for why.
// Cancellation (ctx) is honoured BETWEEN slots only — never
// within one slot's own verify-read/write/verify sequence: a write
// without its verify is an unknown state, and abandoning it mid-pair
// would leave the radio's memory and this package's bookkeeping unable to
// agree on what actually happened.
//
// # ReadSettings (task 33, M8b-3)
//
// ReadSettings is a SEPARATE read path, opt-in via the optional
// driver.SettingsReader capability (the MemorySelector precedent — see
// core/clone/memory_selector.go), producing a *codeplug.MenuSnapshot
// rather than a *codeplug.Codeplug. It is bound by the same house
// disciplines as ReadAll, and by nothing else:
//
//   - Read-only: no journal line, no SnapshotStore write — exactly
//     ReadAll's own read-only standing.
//   - Op-locked: acquireOp("ReadSettings") for the whole call, the same
//     try-lock ReadAll/PrepareSend/Execute already share (see
//     Service.acquireOp/ErrBusy) — a settings read can never interleave
//     its wire traffic with a channel read/write, or vice versa.
//   - NEVER part of PrepareSend. PrepareSend's fixed order (obligations 1
//     and 9 above) is channels only: a fresh ReadAll, a snapshot save,
//     Validate, Diff — none of it touches driver.SettingsReader, and a
//     candidate Codeplug's Menus field is inert to PrepareSend/Execute
//     (codeplug.Digest and codeplug.Diff both operate on Channels alone).
//     Tasks 34-36 (the settings SEND path, once it exists) will be their
//     own separate choreography, layered beside this one, never folded
//     into the channel send path's obligations above.
//
// See settings.go for the full read semantics: descriptor validation
// before any wire traffic, per-item cancellation, the returned-ID
// cross-check, and the partial-snapshot rule (a "?;" rejection is
// recorded data, continuing the read; a genuine ReadSetting error aborts
// it, with no snapshot returned).
//
// M3 Codex-review fix wave, Fix 7 (adjudicated MEDIUM) makes this last
// guarantee structural rather than merely conventional: once a slot's
// write_attempt line is durably journaled, Service.writePair runs
// WriteChannel and its paired read-back ReadChannel under an INTERNAL
// context derived from context.Background() (writeVerifyPairTimeout,
// execute.go — a generous, PRE-HARDWARE-ESTIMATE bound, in the same spirit
// as transport's own Default* constants), not the caller's Execute ctx.
// A caller's ctx cancelling mid-pair (the user closed the app, an
// unrelated deadline expired) can therefore no longer abandon a wire
// write with its verify never attempted — the pair always runs to
// completion (or its own internal timeout, or a genuine transport
// failure) before Execute next consults the caller's ctx, which happens
// only between pairs, at the top of the next slot's iteration.
package clone

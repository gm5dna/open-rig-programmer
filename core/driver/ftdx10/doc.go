// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx10 is the Yaesu FTdx10 driver: the project's SECOND
// implementation of core/driver's Driver/Session seam, and the first one
// written for a radio this project has never connected to anything.
//
// It is structured on core/driver/ft710 — the reference implementation —
// and it deliberately does NOT import it. Nothing in this package may:
// the FT-710 driver's shape is a template, but its VALUES are that
// radio's hardware findings, and importing them is how one radio's
// evidence silently becomes another's claim. Where a decision here looks
// like the FT-710's, the comment at the decision says whether the
// agreement is a manual fact of THIS radio, a structural requirement, or
// an assumption (in which case it is in the register below).
//
// # Provenance
//
// Everything protocol-shaped here comes from the Yaesu FTdx10 CAT
// Operation Reference Manual, edition 2308-F, through core/cat/ftdx10's
// dialect — the ONE core/cat instance this package names (catDialect,
// ftdx10.go). The dialect's own doc.go carries the manual's line
// citations, its six-entry ASSUMED register, its
// reused-command verification verdict, and the chart anomalies; this
// package CITES those entries where it depends on them and duplicates
// none of them.
//
// NO FTdx10 HAS EVER BEEN ASKED ANYTHING by this project, and nothing in
// this package has been written to a radio. There is no
// docs/hardware-notes.md section for this model, no golden vector
// captured from one, and no Stage R or Stage W session yet. That is the
// difference between this driver and the FT-710's, and the register below
// is where it is written down rather than glossed.
//
// # The write guard
//
// writeTrialsComplete is FALSE (caps.go) and pinned false by its own test.
// A RealHardware session therefore gets CapabilitiesUnverified — every
// candidate field's Write spec.Unverified, nothing writable anywhere — so
// codeplug.Diff blocks every change, the clone service refuses to execute
// one, and Session.WriteChannel's own capability re-check refuses before
// any frame is built. An unrecognised Profile value fails the same way
// (see ftdx10Driver.Capabilities): the failure direction for a forged or
// corrupted Profile is always "nothing writable".
//
// The Simulated profile is write-Supported, against internal/fakedx10
// only — see the non-borrowing note below.
//
// # The Simulated profile's clarifier is Supported, not Inert
//
// The FT-710's Simulated profile marks FieldClarifier's Write spec.Inert
// in EVERY profile (core/driver/ft710/caps.go's bankFields), and that is
// a HARDWARE FINDING about the FT-710: on 13/07/2026 a real one accepted
// MW frames carrying non-zero clarifier values without rejection and read
// back zeros every time. It is not borrowed here. No FTdx10 has been
// asked what it does with a clarifier value, so this driver has no
// finding to record — and the Simulated profile describes what happens
// against internal/fakedx10, which stores the clarifier and returns it
// byte-faithfully through the combined MT form. Simulated therefore
// carries clarifier Write spec.Supported, and the profile's honesty is
// intact: it is a claim about the fake, which is where that profile is
// the only legal one.
//
// If a Stage W session ever shows the FTdx10 ignoring the clarifier the
// way the FT-710 does, the change is a per-profile Inert here PLUS the
// same change in the fake — never one of the two.
//
// # MR is deliberately unused
//
// The read path is MT-ONLY: one combined 41-byte MT read carries the
// whole field block AND the tag (read.go). MR is never sent by this
// driver — not by ReadChannel, and not by discovery, which probes with MT
// reads for the same reason.
//
// This is a DESIGN DECISION, not an omission, and it is written here so
// that nobody later "completes" the read path by adding the MR exchange
// the FT-710 needs. The combined answer is an ATOMIC snapshot: a
// two-frame stitch (field block from one radio state, tag from a later
// one) is structurally impossible, whereas the FT-710's MR+MT pair has to
// hold a session-wide lock across the gap to avoid tearing (see that
// driver's Session.opMu). MR itself stays fully covered at the DIALECT
// level — golden vectors, core/cat/dialecttest's conformance suite,
// internal/fakedx10's answers — because a dialect describes a radio's
// protocol, not one driver's use of it.
//
// The same one-exchange property is why this Session carries no
// operation mutex: transport.Engine already serialises each individual
// exchange, and every logical operation here is exactly one. A future
// FTdx10 operation needing two frames needs an opMu with it, and must
// not assume this one is safe without it.
//
// # No per-class kind narrowing
//
// The FT-710's driver keeps its own P7 kind vocabulary per bank
// (acceptedKinds, read.go there) because its leniencies — {'0','1','4'}
// on MEM, {'1','5'} on PMS, {'1'} on discovered banks — are ITS live
// observations. This driver has none, so it adds no check of its own:
// cat.Dialect.ParseMTAnswerCombined already narrows P7 to the combined
// record's OWN documented read pair {'0' VFO, '1' Memory}
// (core/cat/mtcombined.go), and an out-of-vocabulary byte surfaces from
// there as a *cat.ParseError, wrapped by this driver with the slot it was
// reading. Adding a per-class narrowing on top would be inventing a
// distinction this manual does not draw and this project has not
// observed.
//
// # RegionReporter is NOT implemented
//
// core/driver.RegionReporter is optional (core/driver/optional.go), and
// this driver does not satisfy it. The FT-710's Region() derives a
// regulatory-region string from its discovered 5xx/EMG inventory using
// fingerprints — 7 channels and no EMG is "UK", 15 plus EMG is "US" —
// that are that radio's own, partly hardware-confirmed and partly still
// assumed. There is no honest FTdx10 vocabulary to report: no FTdx10
// inventory has ever been observed, so a discovered count maps to no
// region name this project could defend, and borrowing the FT-710's
// fingerprints would mean answering a question about one radio with
// another radio's evidence. Callers already tolerate absence (the
// capability is reached by a two-result type assertion), so nothing
// breaks: codeplug.RadioInfo.Region simply stays unset for an FTdx10.
//
// A Stage R session that enumerates a real FTdx10's 5xx/EMG bank is what
// would make a region vocabulary possible. It is not a code gap to be
// filled meanwhile.
//
// # Discovery walks the WHOLE declared range
//
// Open probes EVERY slot core/cat/ftdx10's SlotSpace declares — 501
// through 599, ascending — and then EMG. There is no contiguity
// assumption, no sentinel, no early termination and no bound: the
// FT-710's discovery rules (stop at the first rejection, cap at 15, probe
// one sentinel past the cap) are FT-710 HARDWARE facts about a radio
// whose factory 5xx channels are believed contiguous and non-erasable,
// and none of that is known here. A sparse FTdx10 bank — a populated 503
// after an empty 502 — must not be silently truncated, and the only way
// to be sure of that with no evidence is to ask every declared slot.
//
// The cost is ~100 exchanges per Open, about 2-2.5 s at the engine's
// default per-exchange settle, paid by every FTdx10 session this project
// opens including in tests. That is ACCEPTED and budgeted (M9c-6 plan,
// "Discovery wall-clock is ACKNOWLEDGED and budgeted"). NOBODY narrows it
// — settle override, early termination, range shrink — without an
// orchestrator-adjudicated change: an ad-hoc optimisation here is exactly
// how the termination assumptions this design refused would come back.
// TestOpen_DiscoveryProbesEveryDeclaredSlot pins the full ordered
// transcript so that a regression is a test failure rather than a
// silently shorter walk.
//
// # The ASSUMED register
//
// Nine behaviours this driver encodes are NOT FTdx10-manual facts. Each is
// listed here once, marked ASSUMED at the point of use, and paired with
// the ONE Stage R or Stage W capture that lifts it. The captures are
// individual on purpose: one FTdx10 session does not retire this register
// wholesale, it retires the assumptions its own frames actually speak to,
// and an entry whose capture was not taken stays here afterwards.
//
// This is the driver's register. The DIALECT's six entries
// (core/cat/ftdx10/doc.go) are separate and are CITED below where this
// driver depends on them — MTPolicy.TagFill, ClarifierPolicy.StepHz,
// SlotSpace.NoneWire, the cat.ModeUnset table member, the 501..599
// numbering, and the combined answer's exact 41-byte width. Correcting
// one of those is a dialect change; correcting one of these is a change
// here. Neither list may absorb the other.
//
//  1. FRAMING: 8 data bits, no parity, TWO stop bits (core/transport's
//     DefaultStopBits, which every session this driver opens inherits —
//     there is no framing field in spec.Capabilities and, per the
//     E2-recorded decision, none is added without hardware evidence).
//     The FTdx10 CAT manual is SILENT on stop bits: it states no framing
//     anywhere, and its serial menu block carries CAT RATE, CAT TIME OUT
//     TIMER and CAT RTS only (manual lines 811-813). 8-N-2 is the
//     FT-710's own documented framing (core/driver/ft710/caps.go:324-325),
//     inherited here as a same-generation working value.
//     STAGE R LIFTS IT WITH: an ID exchange at the answering baud with
//     the port opened 8-N-2 — a clean "ID0761;" confirms the framing is
//     at least compatible. THE LIFT MUST DISTINGUISH FRAMING FROM THE
//     CONTROL LINES (entry 2): silence at a known-correct baud is NOT
//     evidence about stop bits until CAT RTS has been toggled at the
//     radio and the exchange retried, because a handshake refusal and a
//     framing mismatch present identically as nothing coming back. Try
//     8-N-1 only after that has been done, and record which of the two
//     changed the outcome.
//
//  2. CONTROL-LINE POLICY: that driving RTS and DTR low unconditionally
//     is safe on this radio. core/transport.OpenSerial does exactly that
//     immediately after opening any port (safety obligation 4,
//     core/transport/port.go:107-119), for every model, with no per-radio
//     policy — and the FTdx10 has a CAT RTS menu item of its own (3-01-10
//     "CAT RTS", manual line 813; core/cat/ftdx10/exinventory_gen.go:155),
//     so this radio evidently has an opinion about the line that this
//     project's transport does not consult. What that menu's factory
//     setting is, and whether a low RTS with it enabled stalls the CAT
//     link, is unknown.
//     STAGE R LIFTS IT WITH: one ID exchange in each CAT RTS menu state,
//     everything else held constant. If the exchange answers in both, the
//     policy is safe on this radio and this entry closes; if it answers
//     in only one, the transport needs a per-radio control-line policy
//     and this becomes a spec'd capability rather than an assumption.
//     Take this capture BEFORE concluding anything about entry 1.
//
//  3. DefaultBaud 38400 (caps.go). The RATE MENU is manual-evidenced —
//     CAT RATE, menu 3-01-08 (manual line 811), four rates
//     {4800, 9600, 19200, 38400} and no 115200, which is this radio's
//     first real divergence from the FT-710's five — but the FACTORY
//     DEFAULT is NOT IN THE CAT MANUAL AT ALL. That manual's chart has no
//     factory-default column: its headers are "P1 P2 P3 Function P4
//     Digits" (manual line 653), and the trailing 1 on line 811 is the
//     DIGITS field, which is exactly how this project's own generated
//     inventory reads it (core/cat/ftdx10/exinventory_gen.go:153,
//     "CAT RATE", Digits: 1). Spec revision 1 misread that 1 as a default
//     index and concluded 9600; the misreading is RECORDED HERE so it
//     cannot recur silently. The FT-710's 38400 default is an
//     OPERATING-manual fact (core/driver/ft710/caps.go:323-327) and the
//     FTdx10's operating manual is not held by this project, so 38400 is
//     the same-generation family default, ASSUMED. It matters because
//     internal/wiring's OpenRealSessionFor opens a real radio at exactly
//     this driver's DefaultBaud.
//     STAGE R LIFTS IT WITH: the baud a FACTORY-CONFIGURED FTdx10's ID
//     exchange actually answers at — try 38400 first, then the other
//     three in turn. The answering rate is the fact; a radio whose CAT
//     RATE has been changed by its owner cannot settle this, so the
//     capture must record whether the menu was known-untouched.
//
//  4. MinFreqHz 30_000 / MaxFreqHz 75_000_000 (caps.go). The FTdx10 CAT
//     manual carries NO range statement: every frequency legend says only
//     "Frequency (Hz)" over a 9-digit field, which bounds the ENCODING
//     (999999999) and says nothing about what this radio will store. The
//     FT-710's receive-range bounds are mirrored as a conservative
//     working value. MaxFreqHz is additionally the ledgered
//     dangerous-zero field — a zero there would read as "no ceiling" to
//     every validator — so it MUST be populated; this entry is the
//     honesty about where the number came from, not a licence to leave it
//     empty.
//     STAGE R LIFTS IT WITH: the specifications page of the FTdx10
//     OPERATING manual (a document, not a session — the cheapest lift in
//     this register), or, failing that, edge probes: MT Sets at the
//     claimed floor and ceiling and just outside them, to a sacrificial
//     channel, recording which are accepted. The radio range-checks
//     frequency on write (the FT-710 demonstrably does), so acceptance
//     and rejection are both informative.
//
//  5. RequiredSlots {"001"} (caps.go). That memory channel 001 must never
//     be empty. The FT-710's M-01 is individually required — its radio
//     keeps it populated — and this manual states no such rule for the
//     FTdx10 anywhere. Claiming it makes codeplug validation refuse a
//     candidate whose 001 is blank, which is the conservative direction
//     (refuse rather than write a state the radio may not tolerate), but
//     it IS a claim.
//     STAGE R LIFTS IT WITH: observation of channel 001 on a real
//     FTdx10 — whether the radio ships with it populated, and whether the
//     front panel will erase it at all. A radio that erases 001 happily
//     drops this from RequiredSlots.
//
//  6. TONE AND SCAN-SKIP UNREACHABILITY (caps.go's zero FieldSupport for
//     spec.FieldCTCSSTone and spec.FieldScanSkip; read.go's Unknown
//     states). Neither field is claimed readable or writable in either
//     direction on any bank or profile. For the FT-710 the tone half is
//     HW-CONFIRMED — a live MR answer's P9 read fixed "00" with a tone
//     demonstrably set and active — and that finding is that radio's, not
//     this one's. What is true HERE is structural: the FTdx10's combined
//     MT record accounts for every one of its 41 bytes
//     (slot/frequency/clarifier/flags/mode/kind/CTCSS state/P9/shift/P11/
//     tag), P9 is documented fixed "00", and no command this driver sends
//     carries a tone NUMBER or a scan-skip flag for a memory channel at
//     all. So the frame HAS the CTCSS-state byte and no tone-number byte,
//     and nothing verifies that the state byte means anything live on
//     this radio. Whether some OTHER command in this manual could reach a
//     memory channel's tone number is not established either way here —
//     the FT-710's answer, that none can, is that radio's finding and is
//     not borrowed.
//     STAGE R LIFTS IT WITH: one channel set to a known CTCSS tone from
//     the front panel, then read over CAT — if any byte of the answer
//     tracks the tone number, this entry is refuted and the capability
//     opens; if P9 reads "00" as documented, the entry closes as a
//     confirmed protocol limit rather than an assumption. The scan-skip
//     half needs the same experiment with the front-panel skip flag, and
//     is the one this project has never even looked for a side channel
//     for.
//
//  7. "?;" ON A 5xx/EMG DISCOVERY PROBE MEANS ABSENT FROM THIS RADIO
//     (ftdx10.go's discoverInventory/probeSlot). Discovery treats a
//     rejection as "this radio does not have that channel" and a
//     well-formed answer as "it does". "?;" is the protocol's SINGLE
//     unattributed NAK (cat.ErrRejected's own doc comment): it is also
//     what an unknown command, a bad parameter and a wrong radio state
//     get, so reading "absent" out of it is an interpretation. The
//     FT-710's equivalent interpretation is hardware-confirmed for that
//     radio (live probes of a never-populated and an out-of-inventory
//     slot both answered "?;"); this one is not.
//     STAGE R LIFTS IT WITH: an MT enumeration of the whole 5xx range on
//     a radio with a POPULATED 5xx bank, cross-checked against the
//     channels the front panel shows. Which wire numbers answer and which
//     reject is then a fact, and it lifts this entry and the dialect's own
//     501..599 numbering entry together — they are separate assumptions
//     and one capture can retire both, so record both explicitly.
//
//  8. "?;" ON A COMBINED-MT READ OF AN EMPTY SLOT (read.go's ReadChannel).
//     A rejection is mapped to an EMPTY codeplug.Channel — Data nil, the
//     slot carried through — rather than an error, so a read of a blank
//     channel is an ordinary outcome. The FT-710's equivalent was
//     verified for its MR read, NOT for this frame, and this radio's
//     combined MT read of an empty channel has never been seen. The
//     failure mode if this is wrong is quiet and bad: a transport-level
//     problem that manifests as "?;" would be recorded as a genuinely
//     empty channel and could be written back as one.
//     STAGE R LIFTS IT WITH: one MT read of a channel known empty from
//     the front panel, and one of a channel known populated, in the same
//     session. Two different answers confirm the mapping; two rejections
//     mean the driver has been reading blank channels out of a broken
//     link.
//
//  9. A SINGLE COMBINED MT SET SUFFICES TO CREATE OR OVERWRITE A CHANNEL,
//     INCLUDING AN EMPTY ONE. Task 2 implements that write path (this
//     entry lands with the driver skeleton because it is the assumption
//     the whole MT-only choreography rests on, and it must not arrive
//     later than the design it justifies). The 41-byte Set carries the
//     full field block and the tag, so MW would write the same fields
//     redundantly; whether this radio accepts the combined Set as a
//     complete channel definition — and whether it does so for a slot
//     that does not yet exist — is unverified. The FT-710's own
//     empty-slot create is HW-CONFIRMED for ITS two-frame MW+MT
//     choreography, which is not this one.
//     STAGE W LIFTS IT WITH: the FIRST write trial — one combined MT Set
//     to a sacrificial EMPTY channel, then an MT read back, then the same
//     against an already-populated channel. Byte-faithful read-back on
//     both is the lift; anything else (rejection, partial field
//     application, tag written without the field block) converts the
//     write path to a two-frame choreography and this entry to a
//     finding.
package ftdx10

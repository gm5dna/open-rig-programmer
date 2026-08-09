// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakedx10 simulates a Yaesu FTdx10's CAT behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double the
// FTdx10's own layers run against: the transport engine, core/driver/ftdx10,
// the CLI's --fake mode, and the GUI's demo mode — the role
// internal/fakeradio plays for the FT-710.
//
// The radio it speaks is the COMBINED-form one. Its MT command carries the
// whole memory record AND the tag in a single 41-byte frame, in both
// directions, which is the one structural difference that shapes everything
// in this package: there is no short MT tag frame, no display flag anywhere,
// and no way for a slot to hold a tag without holding channel data. MR
// answers the shared 28-byte memory frame, MW sets it, MC recalls, ID
// answers "ID0761;", AI is accepted and readable, and EX (MENU) reads are
// answered from a GENERATED PROJECTION of this package's own copy of
// transcription B — see ex.go and PROVENANCE.md.
//
// # The hard rule: NOTHING project-internal
//
// fakedx10 MUST NOT import any package of this project — not core/cat, not
// core/cat/ftdx10, not core/codeplug, not core/spec, and not
// internal/fakeradio. Standard library only, in every non-test file, in this
// directory AND every directory beneath it. Every byte offset, field width
// and validation rule below is re-derived from the FTdx10 CAT manual's own
// position charts (rev 2308-F), as cited by core/cat/ftdx10/doc.go's
// reused-command verification and by core/driver/ftdx10's tests.
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused core/cat's codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake
// would misbehave in exactly the way the buggy codec expects, and every
// end-to-end test would pass anyway. Two independent implementations of one
// protocol, checked against each other (and against expectations recomputed
// by hand in tests — never by calling this package's own builders), is what
// makes that class of bug visible.
//
// TestNoCoreImports (imports_test.go) enforces it with a go/parser scan, and
// that scan WALKS SUBDIRECTORIES — the one deliberate improvement on
// fakeradio's copy of the same test, whose parser.ParseDir(".") is
// non-recursive and would leave the EX inventory's generator in gen/ outside
// the fence entirely. That generator is the piece the rule bites hardest for:
// it must not reach for internal/extable, the machinery that generates the
// DIALECT's inventory from transcription A, because one parser on both sides of
// the cross-check would reproduce a shared parsing bug into both inventories
// invisibly (ex.go states the mechanism in full).
//
// # A SIBLING of internal/fakeradio, not a refactor of it
//
// This package duplicates a good deal of internal/fakeradio's shape: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Image contract, the options. That duplication is
// deliberate and it is not going to be factored into a shared "fake core"
// package.
//
// Two reasons, both load-bearing. The first is mechanical: a shared helper
// package would be a project-internal import, which the hard rule above
// forbids in both fakes, so the only way to share code would be to abandon
// the property that makes either fake worth having. The second is the reason
// core/cat/ftdx10's mode table is typed out afresh rather than borrowed from
// the FT-710's (see its doc comment): two radios agreeing on a frame shape is
// a fact about those two radios, not a shared definition. The moment a third
// disagrees, a shared implementation has to be unpicked from every simulator
// that had quietly adopted it. The cost of duplication is a copy error, and
// this package's tests — which build every expectation independently — are
// what catch one.
//
// Where this fake DIVERGES from fakeradio, the divergence is a decision with
// a reason, and each is stated at the code that implements it:
//
//   - No Populated flag and no TagDisplay. fakeradio models tag state and
//     channel-data state as independent, because the FT-710's short MT frame
//     writes a tag alone. The combined form cannot express that: one frame
//     carries both, so a slot either exists or it does not.
//   - The clarifier is STORED, and a combined Set round-trips it
//     byte-faithfully. fakeradio stores zeros instead, on an FT-710 HARDWARE
//     finding (its own register item 20) that is not this radio's — see
//     register entry 5.
//   - A Set does not move the selected channel. fakeradio's MW does, again on
//     an FT-710 hardware finding — see register entry 10.
//   - No region concept: no ImageUK/ImageUS pair. fakeradio's pair encodes a
//     hardware finding about UK FT-710s (they have no 5xx bank at all). This
//     project has no evidence about any FTdx10 variant, so the 5xx and EMG
//     banks are OPTIONS a test asks for (With5xx, WithEMG), never regions,
//     and core/driver/ftdx10 deliberately implements no RegionReporter.
//   - No fault injection — see the next section.
//
// # What this fake deliberately does NOT model
//
// FAULTS. fakeradio carries a scripted misbehaviour set (FaultDropReplies,
// FaultGarbleReply, FaultSpuriousFrame, FaultDelayedRejection,
// FaultDelayedReply, FaultDisconnect, FaultChunkedReplies) and this package
// carries none of them. The omission is deliberate: those faults exist to
// exercise core/transport.Engine's timeout, resync, drain-to-quiet and
// chunk-reassembly behaviour, which is MODEL-INDEPENDENT — the engine is one
// implementation, already covered by fakeradio's fault suite, and a second
// copy of the same scripting would test the same engine twice while doubling
// the surface a reviewer must read. No wiring, CLI or GUI path uses faults on
// a fake rig. WithLatency IS kept, because it is not a fault: it is the knob
// Close's promptness is proven against (Radio.shutdown). If an
// FTdx10-specific fault case is ever genuinely needed, fakeradio's options.go
// is the template to copy — and copying it is the right move, per the sibling
// section above.
//
// EX SET. Reads are modelled (ex.go); the Set direction is not, exactly as
// fakeradio does not model it (its own register item 24). A set-shaped EX body
// draws "?;" — register entry 17.
//
// TIMING. Every reply is near-instant unless WithLatency says otherwise. No
// FTdx10 timing has ever been observed by this project, so there is nothing
// to model.
//
// # The ASSUMED register
//
// NO FTdx10 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project — the
// statement core/cat/ftdx10/doc.go opens with, and it governs this package
// twice over. That package at least transcribes a manual; this one has to
// decide what a radio DOES at the protocol's edges, and the manual documents
// almost none of them. Every place this fake had to guess is listed here, in
// one place, with the ONE Stage R or Stage W capture that lifts it, so that a
// reviewer — or the first real FTdx10 session — has a single list to work
// from rather than a source-wide comment hunt. Each entry also appears as an
// inline comment beside the code that implements it. That completeness claim
// is the register's whole value, and it carries ONE recorded exception at
// present — see "What is NOT in this register, and why" at the end, which
// holds the two manual facts that left this register at the M9d follow-up
// wave and the one behaviour known to be unregistered.
//
// Assumptions that belong to the DIALECT (core/cat/ftdx10/doc.go's own
// six-entry register: the tag fill byte, the clarifier's 10 Hz step, the
// "000" none form, the ModeUnset placeholder, the 501..599 numbering, the
// combined answer's exact 41-byte length) are CITED here where this fake
// depends on them, never re-registered. Likewise the driver's nine
// (core/driver/ftdx10/doc.go).
//
//  1. EMPTY-SLOT ANSWERS. A slot this fake holds no state for answers "?;" to
//     an MT read, to an MR read, and to an MC-set. The FT-710's equivalent is
//     HW-CONFIRMED for ITS MR frame (13/07/2026, docs/hardware-notes.md
//     §Empty/out-of-range slots) and that is a different frame on a different
//     radio; the FTdx10's combined-MT read of an empty channel has never been
//     observed. Grammatically valid but out-of-inventory slots ("100", "600")
//     answer identically, since "?;" is the protocol's single unattributed
//     NAK. This is the same assumption core/driver/ftdx10's register entry 8
//     makes from the other side, and it is what makes 5xx/EMG discovery mean
//     anything (its entry 7).
//     STAGE R LIFTS IT WITH: one MT read and one MR read of a memory channel
//     the radio has never had written, plus one MC-set of the same slot. An
//     answer rather than "?;" — or a different NAK — moves this fake and the
//     driver's two entries together.
//     (parser.go: handleMT's read arm, handleMR, handleMC)
//
//  2. PMS SLOTS ANSWER KIND '1' (Memory). The combined record's P7 legend
//     reads "Set: 0: (Fixed) / Read: 0: VFO 1: Memory" — a two-value READ
//     vocabulary with no member for a PMS band-edge memory, and nothing in
//     the manual says which of the two a PMS slot answers with. '1' is the
//     only member that is not plainly false (a PMS slot is not a VFO), so it
//     is what this fake serves. Note what this is NOT: fakeradio serves '5'
//     (PMS) on the FT-710's MR answer, which that radio's own wider P7 table
//     (0-5) has a member for and which is an FT-710 fact besides — this
//     radio's combined record documents two values, and inventing a third
//     would be a frame core/cat would rightly refuse to parse.
//     STAGE R LIFTS IT WITH: one MT read of a POPULATED PMS slot (P1L),
//     programmed from the front panel first. The P7 byte of that answer is
//     the fact.
//     (state.go: MemState.Kind; image.go: defaultState; parser.go: handleMT)
//
//  3. 5xx AND EMG SLOTS ANSWER KIND '1' likewise, for the same reason and
//     with the same weakness. fakeradio's equivalent (its register item 11)
//     is STILL-ASSUMED after two hardware sessions, because the radio
//     characterised had neither bank.
//     STAGE R LIFTS IT WITH: one MT read of a populated 5xx channel and one
//     of EMG, on a variant that HAS them — which no radio this project has
//     touched does, so this entry outlives the first FTdx10 session unless
//     that session's radio is a 5 MHz-bank-bearing variant.
//     (image.go: With5xx, WithEMG; parser.go: handleMT)
//
//  4. EX ANSWER VALUES ARE INVENTED. This fake's EX inventory is GENERATED
//     from its own copy of transcription B; the VALUES it answers
//     with are this package's construction-time convenience by fakeradio's
//     convention — every numeric item n × '0', every text item 12 spaces
//     (fakeradio's buildEXDefaults rule) — and not a claim about any FTdx10's
//     factory or user MENU state. The manual's Table 2 documents each item's
//     VALID RANGE and its option legends, never a shipped default, so there
//     is nothing to source a real default from. This matters beyond the test
//     suite: `rigprog read --settings --fake --model FTdx10` renders these
//     values to a user, who must not read them as what an FTdx10 ships with.
//     The register entry landed with the core, ahead of the code that needed
//     it, so the honesty was on record first.
//     STAGE R LIFTS IT WITH: a full EX sweep of an FTdx10 at factory
//     defaults, values recorded per address. Nothing short of that supplies a
//     default, and the manual never will.
//     (ex.go: exDefaultDigit, exTextWidth, buildEXDefaults)
//
//  5. THE CLARIFIER IS STORED, AND ROUND-TRIPS BYTE-FAITHFULLY. A combined MT
//     Set's P3 sign and magnitude and its P4/P5 flags are stored exactly as
//     they arrived, and the next read answers those same bytes. This is a
//     DELIBERATE NON-BORROWING: fakeradio stores zeros and ignores what was
//     sent, on an FT-710 HARDWARE finding (M5b write trials, 13/07/2026 — its
//     register item 20, and the spec.Inert write policy in
//     core/driver/ft710/caps.go:229-236 that rests on it). That is one
//     radio's observed behaviour on one command, and the FTdx10's Simulated
//     profile deliberately writes the clarifier as Supported rather than
//     Inert for exactly this reason (m9c6-plan.md's plan-level decision).
//     Storing what was sent is the honest default for a field nobody has
//     watched a radio handle.
//     STAGE W LIFTS IT WITH: one combined MT Set carrying a non-zero
//     clarifier offset to a channel whose stored offset is zero, then an MT
//     read of that channel. Zeros coming back mean this radio ignores the
//     field and this fake changes to match; the offset coming back confirms
//     it.
//     (parser.go: handleMT's Set arm; state.go: MemState.ClarSign/ClarMag)
//
//  6. AN MT SET CREATES AN ABSENT CHANNEL. A combined Set to a slot this fake
//     holds no state for stores the whole record and the tag, exactly as it
//     would over an existing one. This is the fake's half of
//     core/driver/ftdx10's register entry 9 ("a single combined MT Set
//     suffices to create/overwrite a channel, including an empty slot"), and
//     it has to be modelled because the FTdx10 driver has no other write
//     path: an MT-only driver against a fake that demanded an MW first could
//     not write at all.
//     STAGE W LIFTS IT WITH: the first write trial — one MT Set to a
//     verified-empty channel, then a read. It is the same capture that lifts
//     the driver's entry 9, and it lifts both or neither.
//     (parser.go: handleMT's Set arm)
//
//  7. SET-DIRECTION FIELD STRICTNESS. Every field vocabulary the position
//     charts print is enforced at the WIRE level, and a violation draws "?;"
//     with no state change: a frequency that is not 9 digits, a clarifier sign
//     that is not '+'/'-', a magnitude outside 0000-9990 or off the 10 Hz step
//     (the step is the DIALECT's assumption — its register entry 2 — cited
//     here, not re-registered), an RX/TX flag that is not '0'/'1', a mode
//     nibble outside the legend's 1-F plus the '0' placeholder, a P7 that is
//     not the chart's fixed '0', a P8 outside 0-2, a P9 that is not "00", a
//     P10 outside 0-2, a P11 that is not the fixed '0', or a tag byte outside
//     printable ASCII. Whether a real FTdx10 REJECTS such a frame — rather
//     than rounding it, ignoring the field, or storing it verbatim — is
//     unobserved for every one of them. The tag charset check is separately
//     SAFETY-CRITICAL and stays whatever hardware turns out to do: accepting
//     ';' would make command injection through a tag possible.
//     STAGE W LIFTS IT WITH: one Set per class carrying a deliberately
//     off-vocabulary field, the reply recorded and the channel read back —
//     "?;" confirms an entry, silence-then-changed-value refutes it, and
//     silence-then-unchanged means the radio ignored the field. Expect this
//     entry to split into several when it is finally taken.
//     (parser.go: the validators, handleMT's Set arm, handleMW)
//
//  8. AN MT SET IS ACCEPTED FOR EVERY SLOT THE READ VOCABULARY ALLOWS —
//     001-099, P1L-P9U, 5xx and EMG. This is WIDER than core/cat's own MT
//     write policy, which refuses 5xx and EMG (mtcombined.go's
//     validateCombinedMTFields), and the difference is intentional: that
//     refusal is a PROJECT DECISION pending hardware verification, taken by
//     the layer that talks to real radios, and this fake models what the
//     radio accepts rather than what this project permits itself to send
//     (fakeradio's register item 6, same reasoning). A consequence worth
//     stating: no test can reach this leniency through the driver, because
//     the codec refuses to build such a frame — it is reachable only by
//     writing bytes to Port() directly, which is what this package's own
//     tests do.
//     STAGE W LIFTS IT WITH: one MT Set to a 5xx channel and one to EMG, on a
//     variant that has them. A rejection narrows this fake to the codec's
//     policy and turns that policy into a fact.
//     (parser.go: mtSettableSlot)
//
//  9. MW IS MODELLED AT ALL, AND ITS FIELDS ARE STORED. The FTdx10's
//     availability table gives MW a Set and no Read and no Answer (manual
//     lines 1258-1272, cited by core/cat/ftdx10/doc.go), its frame is the
//     28-byte memory chart under an "MW" prefix, its P1 legend is restricted
//     to 001-099 and P1L-P9U, and its P7 is "0: (Fixed)". core/driver/ftdx10
//     NEVER SENDS ONE — the write path is MT-only by design (its D-write
//     decision) — and this fake implements it anyway, because a fake that
//     rejected a command the radio documents would be faking a different
//     radio, and because MR/MW parity is what keeps the dialect's own MR
//     coverage meaningful. What is ASSUMED, beyond entry 7's strictness: that
//     an MW leaves the TAG untouched (the frame has no tag field, so there is
//     nothing to write, but "untouched" versus "cleared" is a choice), and
//     that an MW CREATES an absent channel.
//     STAGE W LIFTS IT WITH: one MW Set to a channel carrying a known tag,
//     then an MT read — the tag field of that answer settles the first half;
//     one MW Set to a verified-empty channel settles the second.
//     (parser.go: handleMW)
//
//  10. A SET DOES NOT MOVE THE SELECTED CHANNEL. Only an MC-set changes what
//     "MC;" answers. fakeradio's MW moves it, hands-off, on an FT-710
//     HARDWARE finding (M5b, 13/07/2026: "a successful MW moves the radio's
//     selection to the written slot") that core/clone's selection
//     snapshot/restore is built around. That is one radio's observed
//     behaviour on one command and it is NOT borrowed here — inventing a
//     side effect for a radio nobody has written to is exactly the class of
//     borrowed fact this milestone's driver is careful to refuse.
//     STAGE W LIFTS IT WITH: one MW Set, then "MC;"; and separately one MT
//     Set, then "MC;". The two commands may well differ, which is the other
//     reason not to guess.
//     (parser.go: handleMW, handleMT's Set arm)
//
//  11. AN ACCEPTED SET PRODUCES NO REPLY; A REJECTED ONE PRODUCES EXACTLY ONE
//     "?;". Fire-and-forget on success, for MT Set, MW, MC-set and AI-set
//     alike. The FT-710's manual states this as a general-framing rule and
//     this fake inherits it; the FTdx10's manual is cited in this repository
//     for frame SHAPES (its availability table and position charts), not for
//     an acknowledgement convention. core/driver/ftdx10's write path already
//     depends on the silence — mtSetSpec() is the ZERO transport.CommandSpec,
//     with no ExpectPrefix, precisely because a Set has no answer — so a
//     radio that acknowledged would break the driver, not just this fake.
//     STAGE W LIFTS IT WITH: one MT Set to a real FTdx10 with the port
//     watched for any reply at all before the next command is sent.
//     (fakedx10.go: handleEvent; parser.go: every Set arm returning nil)
//
//  12. FIELD VALUES ARE CASE-SENSITIVE, THOUGH COMMAND NAMES ARE NOT. The
//     command-name half is a MANUAL FACT and is therefore not assumed
//     (manual lines 160-161, quoted under "What is NOT in this register"
//     below): "mt001;" is answered here. What is ASSUMED is that the
//     leniency STOPS THERE — that the mode nibble's hex letters, the PMS
//     L/U suffix and "EMG" must arrive upper-case. The manual's statement
//     is about the two-character command name and says nothing about
//     parameters, so extending it would be an invented leniency; refusing
//     to extend it is the narrower claim and the one that cannot silently
//     accept a frame the radio would reject. Nothing is lost either way:
//     every frame this project sends is built upper-case by core/cat.
//     THIS ENTRY IS THE CORRECTED FORM OF A WRONG ONE, recorded rather
//     than quietly reworded. Until the M9d follow-up wave it read "COMMAND
//     NAMES ARE UPPER CASE ONLY", and its stated reason was that "no such
//     statement about the FTdx10 is cited anywhere in this repository".
//     That was false when written: core/cat/ftdx10/testdata/provenance.md's
//     note A4 has cited the sentence since 29/07/2026 (M9c-4 task 7a, one
//     day before this entry was written), calling the upper-case opcode
//     "strictly a choice, not an assumption". The register entry was
//     therefore not a cautious strictness but a defect against this radio's
//     own manual, and it was pinned by a test
//     (TestCommandNamesAreUpperCaseOnly) that asserted the defect.
//     STAGE R LIFTS THE SURVIVING HALF WITH: one read frame carrying a
//     lower-case FIELD value — "MTp1l;" — put to a real FTdx10. An answer
//     makes that radio's parameters case-insensitive too.
//     (parser.go: handleFrame, parseSlotForm, validModeWireByte)
//
//  13. THE TAG IS STORED TRIMMED AND ANSWERED PADDED. The combined record's
//     P12 is a fixed 12-byte field in both directions. This fake stores the
//     tag with trailing fill trimmed and re-pads it to the full width on
//     every answer, so an all-fill field means "no tag" and a Set→read
//     round trip is byte-faithful over the tag field. The FILL BYTE is a
//     space because the DIALECT says so — its register entry 1, ASSUMED and
//     cited here, never re-derived. What has no analogue on this radio, and
//     is therefore NOT modelled rather than assumed: the FT-710's
//     HW-confirmed rejection of a zero-byte-tag MT Set (fakeradio's register
//     item 7). The combined form cannot express a zero-byte tag at all — a
//     41-byte frame always carries the full field — so there is no shape to
//     accept or refuse.
//     STAGE R LIFTS IT WITH: the dialect's own entry-1 capture (one MT Set
//     of a short tag, then an MT read of that channel), which reports the
//     fill byte and the answer's width together. This fake follows whatever
//     it says.
//     (state.go: MemState.Tag; parser.go: buildMTAnswer, handleMT's Set arm)
//
//  14. WITHDRAWN AT THE M9d FOLLOW-UP WAVE — NOT AN ASSUMPTION. This number
//     held "AI DEFAULTS TO OFF ('0') AT CONSTRUCTION", registered as an
//     inheritance from the FT-710 manual's power-off rule on the stated
//     ground that "the FTdx10's manual is cited here for AI's frame shape
//     ... and not for its power-on state". This manual states the power-on
//     state itself, four lines below the frame shape that WAS cited: "This
//     parameter is set to '0' (OFF) automatically when the transceiver is
//     turned 'OFF'" (manual line 317). The behaviour is unchanged and it is
//     now a MANUAL FACT — see "What is NOT in this register, and why"
//     below, where it is listed with that line.
//     THE NUMBER IS HELD RATHER THAN RECLAIMED because entries 15, 16 and
//     17 are cited BY NUMBER from image.go, image_test.go, options.go,
//     parser.go, ex.go, ex_test.go, fakedx10_test.go and from
//     internal/fakedx101/doc.go, and renumbering would silently repoint
//     every one of them. A tombstone that says what left and why is
//     auditable; a shifted register is not.
//
//  15. THE DEFAULT IMAGE'S CONTENT IS INVENTED. M-01 at 7.000000 MHz LSB,
//     the nine PMS pairs at plausible IARU Region 1 band edges, With5xx's
//     placeholder 5 MHz channels and WithEMG's 5.1675 MHz — all placeholders
//     carried over from fakeradio's own invented values (its register items
//     10, 13, 14), not sourced from any programmed FTdx10, because none has
//     been read. The SHAPE is what these fixtures exist for: a populated
//     memory channel, a populated PMS pair, a sparse 5xx bank, an EMG
//     channel. The 5.1675 MHz figure is the well-known conventional Alaska
//     emergency frequency, used here as a plausible placeholder only, and
//     the sparseness of With5xx's set is a deliberate test property (see its
//     doc comment), not a claim about any radio's inventory.
//     STAGE R LIFTS IT WITH: a full MT sweep of a factory-condition FTdx10 —
//     which reports that radio's inventory, and one radio's inventory is not
//     the model's, so expect this entry to stay ASSUMED with better
//     placeholders rather than to retire.
//     (image.go: DefaultImage, With5xx, WithEMG)
//
//  16. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than 256 bytes have
//     accumulated without a ';', this fake replies "?;" once and discards
//     bytes up to and including the next ';' before resuming normal framing.
//     NOT a radio claim and NOT liftable by any capture: it is this
//     package's own bounded-input policy, inherited from fakeradio's
//     (its register item 1), recorded here so that a test relying on it
//     knows what it is relying on.
//     (parser.go: reassembler)
//
//  17. AN EX READ OF AN ADDRESS THIS FAKE HAS NO ENTRY FOR ANSWERS "?;", AND
//     SO DOES A SET-SHAPED EX BODY. Two claims about the EX command's edges,
//     recorded together because one line of code makes both.
//
//     The first is the menu-side twin of entry 1: a grammatically valid
//     six-digit address the chart never enumerated — every 05xxxx, every P3
//     past a subgroup's item count — draws the protocol's single unattributed
//     NAK. fakeradio's equivalent is NOT an assumption (its register item 23:
//     OBSERVED at M8c for six such addresses, including both probed P1=05
//     ones), and that is an FT-710 fact about an FT-710 menu. The FTdx10's
//     chart has its own P1 anomaly — the grammar block says "P1 : 01 - 05"
//     while the chart populates 01-04 with no P1=05 group at all, recorded
//     UNRESOLVED in core/cat/ftdx10/doc.go precisely because it cannot be put
//     to hardware — so what a real FTdx10 answers to 050101 is doubly unknown.
//     This fake answers "?;" because it holds no entry, which is the honest
//     shape of "I have no such item" and is what core/driver/ftdx10's settings
//     reader maps to driver.SettingUnavailable.
//
//     The second is a deliberate MODELLING GAP, KNOWN-DIVERGENT from the
//     documented grammar rather than a hardware claim: the manual documents an
//     EX Set form and this fake does not implement it, so a valid address
//     followed by a P4 payload ("EX0101011;") is simply a too-long body to
//     handleEX and falls through the same length check to the same NAK. The
//     menu surface is READ-ONLY for v1.x by the M8d decision of 25/07/2026
//     (docs/menu-write-decision.md), so nothing in this project sends one.
//     STAGE R LIFTS THE FIRST WITH: one EX read of 050101 and one of a P3 past
//     a subgroup's end, on a real FTdx10 — an answer rather than "?;" would
//     mean the grammar block's range is real and the chart is incomplete,
//     which is a finding for the dialect as much as for this fake. A STAGE W
//     capture would be needed for the second, and none is planned while the
//     menu surface stays read-only.
//     (ex.go: handleEX)
//
//  18. THE "?;" REJECTION CONVENTION ITSELF IS INHERITED, AND THIS MANUAL
//     DOES NOT PRINT IT. Entries 1, 7, 11, 12, 16 and 17 all say what draws
//     a rejection; this one records that the rejection's very EXISTENCE is
//     an assumption on this radio. The layout-preserved extraction of rev
//     2308-F contains NO '?' character anywhere, over all 1,927 lines: the
//     manual describes Set, Read and Answer commands (manual lines 146-149,
//     with the worked FA example at 150-157) and a terminator (183-185),
//     and never says what the radio replies to a command it cannot honour.
//     "?;" is core/cat's ErrRejected, adopted from the FT-710's reference
//     (core/cat/errors.go:10-19), and every "?;" this fake sends is that
//     convention applied to a radio that has never been observed using it.
//     A radio that stayed SILENT instead would leave this fake's rejections
//     indistinguishable from a dead link, and would turn every one of
//     core/driver/ftdx10's rejection-based interpretations (its register
//     entries "?;" ON A 5xx/EMG DISCOVERY PROBE and "?;" ON A COMBINED-MT
//     READ OF AN EMPTY SLOT) into timeouts.
//     REGISTERED AT THE M9d-2 MILESTONE REVIEW, which found this package
//     making the assumption on every refusal path while a register whose
//     preamble claims to list every place this fake had to guess did not
//     hold it. internal/fakedx101/doc.go's entry 16 is the same claim about
//     those radios, and it is CITED here rather than shared: one capture on
//     an FTdx10 settles this entry and says nothing about an FTdx101.
//     STAGE R LIFTS IT WITH: one deliberately unknown command — "ZZ;" —
//     put to a real FTdx10 with the port watched. Whatever comes back, or
//     does not, is the convention.
//     (parser.go: rejection, and every refusal in the file)
//
// # What is NOT in this register, and why
//
// Two behaviours a reader might expect to find registered as assumptions are
// MANUAL FACTS for this radio. Both WERE registered here — as entries 12 and
// 14 — until the M9d follow-up wave found the manual stating each in terms.
// They are listed here, with the line that makes each one a fact, so that the
// absences read as decisions rather than as oversights, and so that the two
// corrections stay auditable rather than vanishing:
//
//   - COMMAND NAMES MAY ARRIVE IN EITHER CASE. "A command consists of 2
//     alphabetical characters. You may use either lower or upper case
//     characters." (manual lines 160-161, under the "Alphabetical Commands"
//     heading at 159). This fake refused lower-case command names until the
//     M9d follow-up wave — a defect against its own manual, not a cautious
//     strictness, and one this repository had the evidence to catch from
//     29/07/2026, the day before the entry that asserted otherwise was
//     written (core/cat/ftdx10/testdata/provenance.md's note A4, M9c-4 task
//     7a; the entry, M9c-6 task 4). What survives as
//     an assumption is the narrower claim about FIELD values — entry 12
//     above.
//   - AI DEFAULTS TO OFF AT CONSTRUCTION. "This parameter is set to '0'
//     (OFF) automatically when the transceiver is turned 'OFF'" (manual line
//     317). New models a freshly-powered radio, and that is this manual's
//     own statement rather than an inheritance from the FT-710's. Entry 14
//     above is the tombstone it left.
//
// NOTE WHAT IS NOT COVERED BY THE SECOND ABSENCE: what this fake does once AI
// is turned ON — answer "AI1;" faithfully and then push nothing, ever — rests
// on no line of this manual and on no observation of any FTdx10. That claim is
// NOT registered above, where internal/fakedx101's register holds it as its
// entry 18. It is recorded here as a KNOWN GAP rather than closed, because the
// M9d follow-up wave that wrote this section was scoped to the four adjudicated
// FTdx10 evidence corrections and adding an eighteenth-behaviour entry was not
// among them; the entry is owed at this package's next review.
package fakedx10

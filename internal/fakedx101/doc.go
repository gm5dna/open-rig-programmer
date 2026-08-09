// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakedx101 simulates a Yaesu FTDX101D's or FTDX101MP's CAT behaviour
// over an in-memory serial connection (Radio.Port()). It is the test double the
// FTdx101 family's own layers run against: the transport engine,
// core/driver/ftdx101, the CLI's --fake mode, and the GUI's demo mode — the
// role internal/fakeradio plays for the FT-710 and internal/fakedx10 for the
// FTdx10.
//
// # Two radios, one fake
//
// NewD builds a fake FTDX101D, NewMP a fake FTDX101MP, over one private
// newRadio taking the CAT ID (plan decision D1, the shape core/cat/ftdx101's
// DialectD/DialectMP and core/driver/ftdx101's own NewD/NewMP both take). The
// two radios differ ON THIS WIRE IN EXACTLY ONE FIELD — the four bytes of the
// ID answer, "0681" against "0682" (layout 1070 and 1072) — and this package
// exists partly to make that claim testable:
// TestTheTwoModelsDifferOnlyInTheIDAnswer puts a battery of frames to both and
// requires byte-identical replies everywhere else.
//
// That single difference is why buildIDAnswer is a METHOD on Radio here while
// every other reply builder in parser.go is a plain function. It is the ONE
// DELIBERATE DIVERGENCE from this package's structural exemplar,
// internal/fakedx10, whose equivalent is a zero-argument function returning the
// hardcoded literal "ID0761;" (internal/fakedx10/parser.go:698). A literal is
// honest for a package faking ONE radio; it is not available to a package
// faking two, and the alternatives — two Radio types, a package-level variable,
// a model Option — each put the identity somewhere a caller or a later
// maintainer could get it wrong. Reading it from the Radio's own configured ID
// makes "which radio is this?" answerable in one place, fixed at construction,
// unforgeable afterwards. (internal/fakedx10's doc.go does not record this
// divergence: the divergence is this package's, and so is the note.)
//
// Everything else is the COMBINED-form radio internal/fakedx10 also models. The
// MT command carries the whole memory record AND the tag in a single 41-byte
// frame, in both directions, which is the one structural difference that shapes
// everything here: there is no short MT tag frame, no display flag anywhere,
// and no way for a slot to hold a tag without holding channel data. MR answers
// the shared 28-byte memory frame, MW sets it, MC recalls, ID answers this
// radio's own CAT ID, and AI is accepted and readable.
//
// EX (MENU) is NOT ANSWERED YET. Every EX read draws "?;" from the dispatch's
// default arm until this package's generated projection of transcription B
// lands beside it (ex.go, the next task of this milestone).
// TestEXNotModelledYet pins the interim behaviour so that the arm's arrival is
// a visible change rather than a quiet one.
//
// # The hard rule: NOTHING project-internal
//
// fakedx101 MUST NOT import any package of this project — not core/cat, not
// core/cat/ftdx101, not core/codeplug, not core/spec, not internal/fakeradio
// and not internal/fakedx10. Standard library only, in every non-test file, in
// this directory AND every directory beneath it. Every byte offset, field width
// and validation rule below is re-derived from the FTDX101MP/FTDX101D CAT
// Operation Reference Manual's own position charts (rev 2308-L), as cited by
// core/cat/ftdx101/doc.go's reused-command verification, by
// core/cat/ftdx101/testdata/geometry-witness.csv's independent raster count,
// and by docs/superpowers/m9d2-capability-matrix.md.
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused core/cat's codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake
// would misbehave in exactly the way the buggy codec expects, and every
// end-to-end test would pass anyway. Two independent implementations of one
// protocol, checked against each other (and against expectations recomputed by
// hand in tests — never by calling this package's own builders), is what makes
// that class of bug visible.
//
// TestNoCoreImports (imports_test.go) enforces it with a go/parser scan, and
// that scan WALKS SUBDIRECTORIES — internal/fakedx10's deliberate improvement
// on fakeradio's copy of the same test, whose parser.ParseDir(".") is
// non-recursive and would leave a generator in gen/ outside the fence entirely.
// It is copied here IN THE SAME TASK THAT CREATES THE PACKAGE, before any
// subdirectory exists, precisely so that the fence is already standing when the
// EX inventory's generator arrives: that generator is the piece the rule bites
// hardest for, because it must not reach for internal/extable, the machinery
// that generates the DIALECT's inventory from transcription A — one parser on
// both sides of the cross-check would reproduce a shared parsing bug into both
// inventories invisibly.
//
// # A SIBLING of internal/fakeradio and internal/fakedx10, not a refactor
//
// This package duplicates a good deal of both: the pipe-and-goroutine Radio,
// the bounded reassembler, the "?;" convention, the per-command handlers, the
// Image contract, the options. That duplication is deliberate and it is not
// going to be factored into a shared "fake core" package.
//
// Two reasons, both load-bearing. The first is mechanical: a shared helper
// package would be a project-internal import, which the hard rule above forbids
// in all three fakes, so the only way to share code would be to abandon the
// property that makes any of them worth having. The second is the reason
// core/cat/ftdx101's mode table is typed out afresh rather than borrowed from
// the FT-710's or the FTdx10's (see its doc comment): three radios agreeing on
// a frame shape is a fact about those three radios, not a shared definition.
// The moment a fourth disagrees, a shared implementation has to be unpicked
// from every simulator that had quietly adopted it. The cost of duplication is
// a copy error, and this package's tests — which build every expectation
// independently — are what catch one.
//
// Where this fake's BEHAVIOUR differs from internal/fakedx10's, the difference
// is a decision with a reason, and each is stated at the code that implements
// it. EACH REASON IS THIS MANUAL'S OWN, and none of them is a claim about the
// FTdx10's manual or about why that package does what it does:
//
//   - The ID answer is built from the Radio's configured CAT ID rather than
//     from a literal. The section above states why.
//   - COMMAND NAMES ARE ACCEPTED IN EITHER CASE (internal/fakedx10 refuses
//     them). The reason here needs no comparison: this manual says so in
//     terms — "A command consists of 2 alphabetical characters. You may use
//     either lower or upper case characters." (layout 204-205), a statement
//     about these radios in these radios' manual. (parser.go: handleFrame)
//   - AI's power-on state is a MANUAL FACT here rather than an assumption:
//     "This parameter is set to '0' (OFF) automatically when the transceiver
//     is turned 'OFF'" (layout 384). It is therefore absent from the register
//     below — see "What is NOT in this register".
//   - No EX handler yet, and so no EX options (WithEXSetting/WithEXUnavailable
//     have nothing to configure until the projection lands). Both arrive with
//     ex.go.
//
// WHY internal/fakedx10 SETTLES THE MIDDLE TWO DIFFERENTLY IS A QUESTION ABOUT
// THAT PACKAGE, and it is under milestone review rather than answered here.
// Nothing in this package rests on the answer: the citations above are to this
// manual, and they would stand unchanged whatever the sibling turns out to have
// been reading.
//
// Where this fake diverges from internal/fakeradio it does so for
// internal/fakedx10's recorded reasons, which are reproduced here because they
// are as true of these radios as of that one:
//
//   - No Populated flag and no TagDisplay. fakeradio models tag state and
//     channel-data state as independent, because the FT-710's short MT frame
//     writes a tag alone. The combined form cannot express that: one frame
//     carries both, so a slot either exists or it does not.
//   - The clarifier is STORED, and a combined Set round-trips it
//     byte-faithfully. fakeradio stores zeros instead, on an FT-710 HARDWARE
//     finding that is not either of these radios' — see register entry 5, and
//     matrix §2.1, whose Simulated profile writes the clarifier Supported
//     rather than Inert BECAUSE of what this fake does.
//   - A Set does not move the selected channel. fakeradio's MW does, again on
//     an FT-710 hardware finding — see register entry 10.
//   - No region concept: no ImageUK/ImageUS pair. fakeradio's pair encodes a
//     hardware finding about UK FT-710s (they have no 5xx bank at all). This
//     project has no evidence about any FTdx101 variant, so the 5xx and EMG
//     banks are OPTIONS a test asks for (With5xx, WithEMG), never regions, and
//     core/driver/ftdx101 deliberately implements no RegionReporter.
//   - No fault injection — see the next section.
//
// # What this fake deliberately does NOT model
//
// FAULTS. fakeradio carries a scripted misbehaviour set — FaultDropReplies,
// FaultGarbleReply, FaultSpuriousFrame, FaultDelayedRejection,
// FaultDelayedReply, FaultDisconnect, FaultChunkedReplies
// (internal/fakeradio/options.go) — and this package carries none of the seven.
// The omission is plan decision D6, on internal/fakedx10's recorded reasoning
// (its options.go:9-16 and doc.go:90-105): those faults exist to exercise
// core/transport.Engine's timeout, resync, drain-to-quiet and chunk-reassembly
// behaviour, which is MODEL-INDEPENDENT — the engine is one implementation,
// already covered by fakeradio's fault suite, and a third copy of the same
// scripting would test the same engine a third time while doubling the surface
// a reviewer must read. No wiring, CLI or GUI path uses faults on a fake rig.
// WithLatency IS kept, because it is not a fault: it is the knob Close's
// promptness is proven against (Radio.shutdown). If an FTdx101-specific fault
// case is ever genuinely needed, fakeradio's options.go is the template to copy
// — and copying it is the right move, per the sibling section above.
//
// EX. Neither direction is modelled yet; the read side lands with ex.go, and
// the SET side will not, exactly as fakeradio does not model it and
// internal/fakedx10 does not either. The menu surface is READ-ONLY for v1.x by
// the M8d decision of 25/07/2026 (docs/menu-write-decision.md), so nothing in
// this project sends one.
//
// TIMING. Every reply is near-instant unless WithLatency says otherwise. No
// FTdx101 timing has ever been observed by this project, so there is nothing to
// model.
//
// # The ASSUMED register
//
// NO FTdx101 OF EITHER MODEL HAS EVER BEEN ASKED ANYTHING by this project — the
// statement core/cat/ftdx101/doc.go and the capability matrix both open with,
// and it governs this package twice over. Those artefacts at least transcribe a
// manual; this one has to decide what a radio DOES at the protocol's edges, and
// the manual documents almost none of them. Every place this fake had to guess
// is listed here, in one place, with the ONE Stage R or Stage W capture that
// lifts it, so that a reviewer — or the first real FTdx101 session — has a
// single list to work from rather than a source-wide comment hunt. Each entry
// also appears as an inline comment beside the code that implements it.
//
// EVERY ENTRY BELOW IS A CLAIM ABOUT THIS FAKE, CONSUMED BY TESTS. It is not a
// claim about an FTDX101D or an FTDX101MP, and nothing downstream may read it
// as one: what an entry says is "this simulator behaves so, and the tests that
// rest on it are resting on a decision". Where a claim is ALSO made by the
// driver or the dialect, the entry says which of their entries it pairs with,
// so that a capture retires both together or neither.
//
// EVERY LIFT IS PER MODEL, and that is this register's one structural
// difference from internal/fakedx10's. There are two radios here and A CAPTURE
// FROM ONE IS NEVER EVIDENCE ABOUT THE OTHER: the D and the MP share a manual,
// not a serial port. An entry stays open for whichever model has not been
// asked, and a half-retired entry is the normal state of this register rather
// than an error in it. Where an entry's own content differs per model, it says
// so; only the ID answer does, and it is not in this register at all, because
// both IDs are manual facts (layout 1070-1072). EVERY CAPTURE MUST ALSO RECORD
// ITS PORT: this radio offers two CAT paths and they are not equivalent (matrix
// §3.12; core/driver/ftdx101/doc.go states the consequences).
//
// Assumptions that belong to the DIALECT (core/cat/ftdx101/doc.go's own
// SEVEN-entry register: the tag fill byte, the clarifier's 10 Hz step, the
// "000" none form, the ModeUnset placeholder, the 501..599 numbering, the
// combined answer's exact 41-byte length, the clarifier's minus-direction byte)
// are CITED here BY NAME where this fake depends on them, never re-registered
// and never by position — a register renumbered at a review leaves positional
// citations pointing at the wrong entry. Likewise the driver's nine
// (core/driver/ftdx101/doc.go).
//
//  1. EMPTY-SLOT ANSWERS. A slot this fake holds no state for answers "?;" to
//     an MT read, to an MR read, and to an MC-set. The FT-710's equivalent is
//     HW-CONFIRMED for ITS MR frame (13/07/2026, docs/hardware-notes.md
//     §Empty/out-of-range slots) and that is a different frame on a different
//     radio; neither FTdx101's combined-MT read of an empty channel has ever
//     been observed. Grammatically valid but out-of-inventory slots ("100",
//     "600") answer identically, since "?;" is the protocol's single
//     unattributed NAK. This is the same assumption core/driver/ftdx101's
//     register entry 8 makes from the other side, and it is what makes 5xx/EMG
//     discovery mean anything (its entry 7).
//     STAGE R LIFTS IT, PER MODEL, WITH: one MT read and one MR read of a
//     memory channel that radio has never had written, plus one MC-set of the
//     same slot. An answer rather than "?;" — or a different NAK — moves this
//     fake and the driver's two entries together, for that model.
//     (parser.go: handleMT's read arm, handleMR, handleMC)
//
//  2. PMS SLOTS ANSWER KIND '1' (Memory). The combined record's P7 legend reads
//     "Set: 0: (Fixed) / Read: 0: VFO 1: Memory" (layout 1324) — a two-value
//     READ vocabulary with no member for a PMS band-edge memory, and nothing in
//     the manual says which of the two a PMS slot answers with. '1' is the only
//     member that is not plainly false (a PMS slot is not a VFO), so it is what
//     this fake serves. Note what this is NOT: fakeradio serves '5' (PMS) on
//     the FT-710's MR answer, which that radio's own wider P7 table (0-5) has a
//     member for and which is an FT-710 fact besides — these radios' combined
//     record documents two values, and inventing a third would be a frame
//     core/cat would rightly refuse to parse.
//     STAGE R LIFTS IT, PER MODEL, WITH: one MT read of a POPULATED PMS slot
//     (P1L), programmed from that radio's front panel first. The P7 byte of
//     that answer is the fact.
//     (state.go: MemState.Kind; image.go: defaultState; parser.go: handleMT)
//
//  3. 5xx AND EMG SLOTS ANSWER KIND '1' likewise, for the same reason and with
//     the same weakness. fakeradio's equivalent (its register item 11) is
//     STILL-ASSUMED after two hardware sessions, because the radio
//     characterised had neither bank.
//     STAGE R LIFTS IT, PER MODEL, WITH: one MT read of a populated 5xx channel
//     and one of EMG, on a radio of that model that HAS them — which no radio
//     this project has touched does, so this entry outlives the first FTdx101
//     session unless that session's radio carries the banks.
//     (image.go: With5xx, WithEMG; parser.go: handleMT)
//
//  4. EX ANSWER VALUES ARE INVENTED. This fake's EX inventory will be GENERATED
//     from its own copy of transcription B; the VALUES it answers with are this
//     package's construction-time convenience by fakeradio's convention — every
//     numeric item n × '0', every text item 12 spaces (fakeradio's
//     buildEXDefaults rule) — and not a claim about any FTDX101D's or
//     FTDX101MP's factory or user MENU state. The manual's Table 2 documents
//     each item's VALID RANGE and its option legends, never a shipped default,
//     so there is nothing to source a real default from. This matters beyond
//     the test suite: `rigprog read --settings --fake --model FTdx101D` will
//     render these values to a user, who must not read them as what an FTdx101
//     ships with.
//     THIS ENTRY LANDS AHEAD OF THE CODE IT GOVERNS, with the package core and
//     one task before ex.go, exactly as internal/fakedx10's entry 4 did, so
//     that the honesty is on record before the first invented value exists.
//     STAGE R LIFTS IT, PER MODEL, WITH: a full EX sweep of a radio of that
//     model at factory defaults, values recorded per address. Nothing short of
//     that supplies a default, and the manual never will.
//     (ex.go, when it lands: exDefaultDigit, exTextWidth, buildEXDefaults)
//
//  5. THE CLARIFIER IS STORED, AND ROUND-TRIPS BYTE-FAITHFULLY. A combined MT
//     Set's P3 sign and magnitude and its P4/P5 flags are stored exactly as they
//     arrived, and the next read answers those same bytes. This is a DELIBERATE
//     NON-BORROWING: fakeradio stores zeros and ignores what was sent, on an
//     FT-710 HARDWARE finding (M5b write trials, 13/07/2026 — its register item
//     20, and the spec.Inert write policy in core/driver/ft710/caps.go:229-236
//     that rests on it). That is one radio's observed behaviour on one command,
//     and BOTH FTdx101 Simulated profiles deliberately write the clarifier as
//     Supported rather than Inert for exactly this reason (matrix §2.1's
//     profile table and plan decision D3; core/driver/ftdx101/caps.go's
//     capabilitiesSimulated). Those profiles are TRUE OF THIS FAKE AND OF
//     NOTHING ELSE, and this entry is what makes them true.
//     The SIGN BYTE this round trip carries for a negative offset is the
//     DIALECT's own seventh entry ("The CLARIFIER'S MINUS-DIRECTION BYTE, the
//     ASCII HYPHEN-MINUS 0x2D ('-')"), cited not re-registered: this manual
//     prints that direction as a two-hyphen glyph and the quarantined golden
//     deriver recorded it UNREADABLE rather than resolving it.
//     STAGE W LIFTS IT, PER MODEL, WITH: one combined MT Set carrying a
//     non-zero clarifier offset to a channel whose stored offset is zero, then
//     an MT read of that channel. Zeros coming back mean that radio ignores the
//     field and this fake plus that model's profile change to match; the offset
//     coming back confirms it. A NEGATIVE offset used for the capture retires
//     the dialect's sign entry for that model at the same time — record both.
//     (parser.go: handleMT's Set arm, parseMemoryBlock; state.go:
//     MemState.ClarSign/ClarMag)
//
//  6. AN MT SET CREATES AN ABSENT CHANNEL. A combined Set to a slot this fake
//     holds no state for stores the whole record and the tag, exactly as it
//     would over an existing one. This is the fake's half of
//     core/driver/ftdx101's register entry 9 ("a single combined MT Set
//     suffices to create or overwrite a channel, including an empty one"), and
//     it has to be modelled because the FTdx101 driver has no other write path:
//     an MT-only driver against a fake that demanded an MW first could not
//     write at all.
//     STAGE W LIFTS IT, PER MODEL, WITH: that model's first write trial — one
//     MT Set to a verified-empty channel, then a read. It is the same capture
//     that lifts the driver's entry 9, and it lifts both or neither. NOTHING
//     MAY BE WRITTEN TO EITHER RADIO WHILE ITS WRITE GUARD IS FALSE, and both
//     are false.
//     (parser.go: handleMT's Set arm)
//
//  7. SET-DIRECTION FIELD STRICTNESS. Every field vocabulary the position
//     charts print is enforced at the WIRE level, and a violation draws "?;"
//     with no state change: a frequency that is not 9 digits, a clarifier sign
//     that is not '+'/'-', a magnitude outside 0000-9990 or off the 10 Hz step
//     (the step is the DIALECT's "ClarifierPolicy.StepHz = 10" entry, cited
//     here, not re-registered), an RX/TX flag that is not '0'/'1', a mode nibble
//     outside the legend's 1-F plus the '0' placeholder, a P7 that is not the
//     chart's fixed '0', a P8 outside 0-2, a P9 that is not "00", a P10 outside
//     0-2, a P11 that is not the fixed '0', or a tag byte outside printable
//     ASCII. Whether a real FTdx101 REJECTS such a frame — rather than rounding
//     it, ignoring the field, or storing it verbatim — is unobserved for every
//     one of them on both models. What the manual DOES say is that a parameter
//     with the wrong number of digits is a mistake (its worked IS-command
//     examples, layout 213-223); it never says what the radio then does.
//     STAGE W LIFTS IT, PER MODEL, WITH: one Set per class carrying a
//     deliberately off-vocabulary field, the reply recorded and the channel read
//     back — "?;" confirms an entry, silence-then-changed-value refutes it, and
//     silence-then-unchanged means the radio ignored the field. Expect this
//     entry to split into several when it is finally taken.
//     (parser.go: the validators, handleMT's Set arm, handleMW)
//
//  8. AN MT SET IS ACCEPTED FOR EVERY SLOT THE READ VOCABULARY ALLOWS —
//     001-099, P1L-P9U, 5xx and EMG. This is WIDER than core/cat's own MT write
//     policy, which refuses 5xx and EMG (mtcombined.go's
//     validateCombinedMTFields), and the difference is intentional: that
//     refusal is a PROJECT DECISION pending hardware verification, taken by the
//     layer that talks to real radios, and this fake models what the radio
//     accepts rather than what this project permits itself to send (fakeradio's
//     register item 6, same reasoning). Matrix §1.3.5 is careful about the
//     evidence on this radio and so is this entry: MT's slot legend is headed
//     "P0/1" (layout 1312), merging the Read direction's slot parameter with
//     the Set direction's under one vocabulary, so the manual does not
//     separately state that an MT SET may address 5xx or EMG. It states it of
//     MT's slot parameter generally. This fake's leniency is therefore an
//     assumption about the Set direction, not a reading of a Set-direction
//     legend. A consequence worth stating: no test can reach the leniency
//     through the driver, because the codec refuses to build such a frame — it
//     is reachable only by writing bytes to Port() directly, which is what this
//     package's own tests do.
//     STAGE W LIFTS IT, PER MODEL, WITH: one MT Set to a 5xx channel and one to
//     EMG, on a radio of that model that has them. A rejection narrows this fake
//     to the codec's policy and turns that policy into a fact for that model.
//     (parser.go: mtSettableSlot)
//
//  9. MW IS MODELLED AT ALL, AND ITS FIELDS ARE STORED. The FTdx101's
//     availability table gives MW a Set and no Read and no Answer (O X X X,
//     layout 336), its frame is the 28-byte memory chart under an "MW" prefix
//     (layout 1352-1367), its P1 legend is restricted to 001-099 and P1L-P9U,
//     and its P7 is "0: (Fixed)". core/driver/ftdx101 NEVER SENDS ONE — the
//     write path is MT-only by design (matrix §3.6) — and this fake implements
//     it anyway, because a fake that rejected a command the radio documents
//     would be faking a different radio, and because MR/MW parity is what keeps
//     the dialect's own MR coverage meaningful. What is ASSUMED, beyond entry
//     7's strictness: that an MW leaves the TAG untouched (the frame has no tag
//     field, so there is nothing to write, but "untouched" versus "cleared" is a
//     choice), and that an MW CREATES an absent channel.
//     STAGE W LIFTS IT, PER MODEL, WITH: one MW Set to a channel of that radio
//     carrying a known tag, then an MT read — the tag field of that answer
//     settles the first half; one MW Set to a verified-empty channel settles the
//     second.
//     (parser.go: handleMW)
//
//  10. A SET DOES NOT MOVE THE SELECTED CHANNEL. Only an MC-set changes what
//     "MC;" answers. fakeradio's MW moves it, hands-off, on an FT-710 HARDWARE
//     finding (M5b, 13/07/2026: "a successful MW moves the radio's selection to
//     the written slot") that core/clone's selection snapshot/restore is built
//     around. That is one radio's observed behaviour on one command and it is
//     NOT borrowed here — inventing a side effect for two radios nobody has
//     written to is exactly the class of borrowed fact this milestone refuses.
//     STAGE W LIFTS IT, PER MODEL, WITH: one MW Set, then "MC;"; and separately
//     one MT Set, then "MC;". The two commands may well differ, which is the
//     other reason not to guess.
//     (parser.go: handleMW, handleMT's Set arm)
//
//  11. AN ACCEPTED SET PRODUCES NO REPLY; A REJECTED ONE PRODUCES EXACTLY ONE
//     "?;". Fire-and-forget on success, for MT Set, MW, MC-set and AI-set
//     alike. PARTIAL EVIDENCE EXISTS FOR ONE OF THE FOUR: MW's availability row
//     is O X X X (layout 336), so that command has no Answer form at all and a
//     successful MW cannot be answered. For MT, MC and AI the Answer column is
//     O because each has a Read, and nothing in this manual says what a Set
//     produces. The FT-710's manual states silence-on-success as a general
//     framing rule and this fake inherits it; this manual is cited in this
//     repository for frame SHAPES (its availability table and position charts),
//     not for an acknowledgement convention. core/driver/ftdx101's write path
//     already depends on the silence — its MT Set spec carries no ExpectPrefix,
//     precisely because a Set has no answer — so a radio that acknowledged
//     would break the driver, not just this fake.
//     STAGE W LIFTS IT, PER MODEL, WITH: one MT Set to a radio of that model
//     with the port watched for any reply at all before the next command is
//     sent.
//     (fakedx101.go: handleEvent; parser.go: every Set arm returning nil)
//
//  12. FIELD VALUES ARE CASE-SENSITIVE, THOUGH COMMAND NAMES ARE NOT. The
//     command-name half is a MANUAL FACT and is therefore not assumed (layout
//     204-205, quoted in the divergence list above): "mt001;" is answered here.
//     What is ASSUMED is that the leniency STOPS THERE — that the mode nibble's
//     hex letters, the PMS L/U suffix and "EMG" must arrive upper-case. The
//     manual's statement is about the two-character command name and says
//     nothing about parameters, so extending it would be an invented leniency;
//     refusing to extend it is the narrower claim and the one that cannot
//     silently accept a frame the radio would reject. Nothing is lost either
//     way: every frame this project sends is built upper-case by core/cat.
//     STAGE R LIFTS IT, PER MODEL, WITH: one read frame carrying a lower-case
//     FIELD value — "MTp1l;" — put to a radio of that model. An answer makes
//     that radio's parameters case-insensitive too.
//     (parser.go: handleFrame, parseSlotForm, validModeWireByte)
//
//  13. THE TAG IS STORED TRIMMED AND ANSWERED PADDED, AND ITS CHARSET CHECK IS
//     NARROWER THAN THE MANUAL'S. The combined record's P12 is a fixed 12-byte
//     field in both directions. This fake stores the tag with trailing fill
//     trimmed and re-pads it to the full width on every answer, so an all-fill
//     field means "no tag" and a Set→read round trip is byte-faithful over the
//     tag field. The FILL BYTE is a space because the DIALECT says so — its
//     "MTPolicy.TagFill = ' '" entry, ASSUMED and cited here, never re-derived.
//     The accepted charset is printable ASCII 0x20-0x7E excluding ';', which is
//     NARROWER than the one charset rule this manual states — that unused
//     parameter digits be "filled using any character except the ASCII control
//     codes (00 to 1Fh) and the terminator (;)" (layout 224-225), which would
//     permit 0x7F and every byte above it. The narrowing is the assumption; the
//     ';' exclusion is the part the manual agrees with, and it is separately
//     SAFETY-CRITICAL and stays whatever hardware turns out to do, because
//     accepting ';' would make command injection through a tag possible.
//     What has no analogue on this radio, and is therefore NOT modelled rather
//     than assumed: the FT-710's HW-confirmed rejection of a zero-byte-tag MT
//     Set (fakeradio's register item 7). The combined form cannot express a
//     zero-byte tag at all — a 41-byte frame always carries the full field — so
//     there is no shape to accept or refuse.
//     STAGE R LIFTS THE PADDING, PER MODEL, WITH: the dialect's own TagFill
//     capture (one MT Set of a short tag, then an MT read of that channel),
//     which reports the fill byte and the answer's width together. This fake
//     follows whatever it says. A STAGE W capture of a tag carrying 0x7F would
//     be needed for the charset half, and none is planned.
//     (state.go: MemState.Tag; parser.go: buildMTAnswer, validTagField,
//     handleMT's Set arm)
//
//  14. THE DEFAULT IMAGE'S CONTENT IS INVENTED. M-01 at 7.000000 MHz LSB, the
//     nine PMS pairs at plausible IARU Region 1 band edges, With5xx's
//     placeholder 5 MHz channels and WithEMG's 5.1675 MHz — all placeholders
//     carried over from fakeradio's own invented values (its register items 10,
//     13, 14), not sourced from any programmed FTdx101 of either model, because
//     none has been read. The SHAPE is what these fixtures exist for: a
//     populated memory channel, a populated PMS pair, a sparse 5xx bank, an EMG
//     channel. The 5.1675 MHz figure is the well-known conventional Alaska
//     emergency frequency, used here as a plausible placeholder only, and the
//     sparseness of With5xx's set is a deliberate test property (see its doc
//     comment), not a claim about any radio's inventory.
//     STAGE R LIFTS IT, PER MODEL, WITH: a full MT sweep of a factory-condition
//     radio of that model — which reports that radio's inventory, and one
//     radio's inventory is not the model's, so expect this entry to stay ASSUMED
//     with better placeholders rather than to retire.
//     (image.go: DefaultImage, With5xx, WithEMG)
//
//  15. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than 256 bytes have
//     accumulated without a ';', this fake replies "?;" once and discards bytes
//     up to and including the next ';' before resuming normal framing. NOT a
//     radio claim and NOT liftable by any capture: it is this package's own
//     bounded-input policy, inherited from fakeradio's (its register item 1),
//     recorded here so that a test relying on it knows what it is relying on.
//     (parser.go: reassembler)
//
//  16. THE "?;" REJECTION CONVENTION ITSELF IS INHERITED, AND THIS MANUAL DOES
//     NOT PRINT IT. Entries 1, 7, 8, 9 and 12 all say what draws a rejection;
//     this one records that the rejection's very existence is an assumption on
//     these radios. The layout-preserved extraction of rev 2308-L contains NO
//     '?' character anywhere, over all 2,051 lines: the manual describes Set,
//     Read and Answer commands (layout 190-193) and a terminator, and never
//     says what the radio replies to a command it cannot honour. "?;" is
//     core/cat's ErrRejected, adopted from the FT-710's reference (core/cat's
//     errors.go:11-19), and every "?;" this fake sends is that convention
//     applied to a radio that has never been observed using it. A radio that
//     stayed SILENT instead would leave this fake's rejections indistinguishable
//     from a dead link, and would turn every one of the driver's rejection-based
//     interpretations (its entries 7 and 8) into timeouts.
//     STAGE R LIFTS IT, PER MODEL, WITH: one deliberately unknown command —
//     "ZZ;" — put to a radio of that model with the port watched. Whatever comes
//     back, or does not, is the convention.
//     (parser.go: rejection, and every refusal in the file)
//
// # What is NOT in this register, and why
//
// Two behaviours a reader might expect to find registered as assumptions are
// MANUAL FACTS for these radios. They are listed here, with the line that makes
// each one a fact, so that the absences read as decisions rather than as
// oversights:
//
//   - AI DEFAULTS TO OFF AT CONSTRUCTION. "This parameter is set to '0' (OFF)
//     automatically when the transceiver is turned 'OFF'" (layout 384). New
//     models a freshly-powered radio, and that is a manual fact for both
//     models — the note carries no model qualifier, and §4 of the matrix shows
//     the manual's whole model-distinguishing surface is three places, none of
//     them this one.
//   - COMMAND NAMES MAY ARRIVE IN EITHER CASE (layout 204-205). What survives
//     as an assumption is the narrower claim about FIELD values — entry 12
//     above.
//
// internal/fakedx10's register settles both differently (its entries 14 and
// 12). That is a question about that package, it is under milestone review, and
// it is deliberately not argued here: these two absences rest on the two lines
// cited above and on nothing else.
//
// The ID answers themselves are likewise absent, and for the strongest reason:
// "0681: FTDX101D" and "0682: FTDX101MP" are printed in ID's own P1 legend
// (layout 1070, 1072). The per-model difference this package exists to model is
// the one thing about it that is not assumed at all.
package fakedx101

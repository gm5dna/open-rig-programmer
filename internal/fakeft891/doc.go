// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeft891 simulates a Yaesu FT-891's CAT behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double the
// FT-891's own layers run against: the transport engine, core/driver/ft891,
// the CLI's --fake mode, and the GUI's demo mode — the role
// internal/fakeradio plays for the FT-710 and internal/fakedx10 for the
// FTdx10.
//
// The radio it speaks is a COMBINED-form one whose combined frame carries a
// LIVE TAG DISPLAY FLAG. Its MT command carries the whole memory record, the
// display flag and the tag in a single 41-byte frame, in both directions.
// Byte 28 of that frame is this radio's one genuinely new axis: MT's P11
// legend prints `0: TAG "OFF" 1: TAG "ON"` (ft891_layout.txt:1016) where
// every registered combined-form sibling prints "0: (Fixed)", so this fake
// STORES the flag per channel and answers it back. MR answers the shared
// 28-byte memory frame for every slot its own legend lists, MC recalls the
// two classes its legend names, ID answers "ID0650;" and AI is accepted and
// readable.
//
// EX (MENU) is answered, READ ONLY, from an inventory generated from this
// package's own copy of transcription B — a FOUR-digit address on this radio
// where every registered sibling's is six (ex.go). An EX Set is not modelled;
// see "What this fake deliberately does NOT model" below.
//
// # The hard rule: NOTHING project-internal
//
// fakeft891 MUST NOT import any package of this project — not core/cat, not
// core/cat/ft891, not core/codeplug, not core/spec, and not
// internal/fakeradio or any sibling fake. Standard library only, in every
// non-test file, in this directory AND every directory beneath it — which
// now includes internal/fakeft891/gen, the stdlib-only generator for this
// radio's transcription B. The fence was recursive from birth for that
// directory's sake, and TestNoCoreImports_ReachesTheGenerator asserts that the
// scan of this package really does parse gen/main.go rather than merely being
// capable of it.
// Every byte offset, field width and validation rule below is re-derived from
// the FT-891 CAT Operation Reference Book's own position charts (rev 1909-C),
// as cited by core/cat/ft891/doc.go's reused-command verification and by
// core/cat/ft891/testdata/provenance.md's frame-geometry witness.
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused core/cat's codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake
// has to be able to DISAGREE with the production codec for a test against it
// to mean anything, and it can only disagree if it was built from the manual
// rather than from the code.
//
// # A SIBLING of internal/fakedx10, not a refactor of it
//
// This package duplicates a good deal of internal/fakedx10's shape: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Image contract, the options. That duplication is
// deliberate and it is not going to be factored into a shared "fake core"
// package.
//
// Two reasons, both load-bearing. The first is mechanical: a shared helper
// package would be a project-internal import, which the hard rule above
// forbids in every fake, so the only way to share code would be to abandon
// the property that makes any of them worth having. The second is the reason
// core/cat/ft891/dialect.go's mode table is typed out afresh rather than
// borrowed from the FTdx10's (see its doc comment): two radios agreeing on a
// frame shape is a fact about those two radios, not a shared definition — and
// THIS RADIO IS THE PROOF. Six of its twelve mode names disagree with the
// FTdx10's at the same nibble, three nibbles the FTdx10 fills are empty here,
// its byte 21 is schema where the FTdx10's is a TX clarifier flag, its byte
// 28 is a live flag where the FTdx10's is schema, its MT read legend names
// two slot classes where the FTdx10's names four, and its 5 MHz bank is ten
// transcribed channels where the FTdx10's is 501..599 on an inherited
// numbering its own register carries as an assumption. A shared
// implementation would have had to be unpicked from every one of those.
//
// Where this fake DIVERGES from fakedx10, the divergence is a decision with a
// reason, and each is stated at the code that implements it:
//
//   - THE TAG DISPLAY FLAG IS STORED AND ANSWERED. fakedx10 has no such
//     field at any position; this radio's P11 is live (state.go's
//     MemState.TagDisplay, parser.go's handleMT).
//   - BYTE 21 IS SCHEMA, NOT A TX CLARIFIER FLAG. "P5 0: (Fixed)" is printed
//     on all five blocks that carry the field (MR 971, MT 1006, MW 1042, IF
//     783, OI 1129), so there is no TX clarifier state to store in either
//     direction — where fakedx10 stores one.
//   - MT NAMES TWO SLOT CLASSES, NOT FOUR. This radio's MT legend prints
//     memory and PMS only (998-999) where its own MR legend prints 5xx and
//     EMG as well (960-964); fakedx10's MT accepts every readable slot, and
//     registers that leniency. Here the narrowness is the LEGEND, not a
//     policy.
//   - THE 5 MHz BANK IS 501-510, TRANSCRIBED. MR's legend prints the actual
//     numbers (962) where the FTdx10's prints only "5xx"; the numbering is
//     therefore not on any ASSUMED register (core/cat/ft891/doc.go, "WHAT IS
//     DELIBERATELY NOT AN ENTRY").
//   - No fault injection, and no MW — see the next section.
//
// # What this fake deliberately does NOT model
//
// EX SET. This radio's EX grammar is four-digit ("EX P1 P1 P1 P1 ;", seven
// bytes — ft891_layout.txt:513-522) where every registered sibling's is six,
// and the READ direction is answered in full, from an inventory generated from
// this package's own copy of transcription B (ex.go, PROVENANCE.md). The SET
// direction is not implemented, so a valid address followed by a P4 payload is
// simply a too-long body to handleEX and draws "?;". That is a MODELLING GAP,
// KNOWN-DIVERGENT from the documented grammar, and it is not a claim that this
// radio refuses EX Set — TestEXSetShaped_NotModelled pins the gap's shape
// (refused, state unchanged) so that a later task adding EX Set has to change
// it deliberately. It is out of scope by this milestone's plan for the same
// reason MW is: no layer above this fake sends one. internal/fakeradio and
// internal/fakedx10 both decline it too.
//
// MW (MEMORY CHANNEL WRITE). This radio documents MW — Set only, no Read and
// no Answer (availability 167), its Set frame the 28-position MR chart under
// an "MW" prefix with a P1 legend restricted to memory and PMS (1034-1050) —
// and this fake does not implement it, so an MW frame answers "?;". That is
// a MODELLING GAP, KNOWN-DIVERGENT from the documented grammar, and it is not
// a claim that this radio lacks the command. It is out of scope by this
// milestone's plan: core/driver/ft891's write path is MT-only (one combined
// Set carries the field block, the display flag and the tag, where MW could
// carry neither of the last two), the dialect's own MW coverage is its golden
// vectors and its conformance tests in core/cat/ft891, and no layer above
// this fake sends one. internal/fakedx10 models MW for the same radio-fidelity
// reason this note declines to, and its handler is the template if a later
// task wants one here.
//
// FAULTS. internal/fakeradio carries a scripted misbehaviour set
// (FaultDropReplies, FaultGarbleReply, FaultSpuriousFrame,
// FaultDelayedRejection, FaultDelayedReply, FaultDisconnect,
// FaultChunkedReplies) and this package carries none of them. The omission is
// deliberate: those faults exercise core/transport.Engine's timeout, resync,
// drain-to-quiet and chunk-reassembly behaviour, which is MODEL-INDEPENDENT —
// the engine is one implementation, already covered by fakeradio's fault
// suite, and a second copy of the same scripting would test the same engine
// twice while doubling the surface a reviewer must read. No wiring, CLI or
// GUI path uses faults on a fake rig. WithLatency IS kept, because it is not
// a fault: it is the knob Close's promptness is proven against
// (Radio.shutdown, TestClose_IsPromptDespiteAPendingLatency).
//
// TIMING. Every reply is near-instant unless WithLatency says otherwise. No
// FT-891 timing has ever been observed by this project, so there is nothing
// to model.
//
// # The ASSUMED register
//
// NO FT-891 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project — the
// statement core/cat/ft891/doc.go opens with, and it governs this package
// twice over. That package at least transcribes a manual; this one has to
// decide what a radio DOES at the protocol's edges, and this manual documents
// almost none of them — and, worse than any sibling's, it CONTRADICTS ITSELF
// about the one command the whole read design turns on. Every place this fake
// had to guess is listed here, in one place, with the ONE Stage R or Stage W
// capture that lifts it, so that a reviewer — or the first real FT-891
// session — has a single list to work from rather than a source-wide comment
// hunt. Each entry also appears as an inline comment beside the code that
// implements it. That completeness claim is the register's whole value and it
// is made without exception: see also "What is NOT in this register, and why"
// at the end, which holds the manual facts a reader might expect to find here
// — an absence with a manual line behind it is not a gap in the claim above.
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION — the rule
// core/cat/ft891/doc.go's own register states in terms, and which this
// package's code follows at every site. The numbering below is for
// readability: a positional citation is correct only until somebody adds or
// reorders an entry, and it then silently points at the wrong assumption
// rather than failing.
//
// Assumptions that belong to the DIALECT (core/cat/ft891/doc.go's own
// eight-entry register: the tag fill byte, the combined answer's exact
// 41-byte length, the "000" none form, the ModeUnset placeholder, the
// clarifier's 10 Hz step and 9990 ceiling, the clarifier's minus-direction
// byte, the combined answer's P7 read domain, the MC answer domain beyond
// memory and PMS) are CITED here where this fake depends on them — all eight
// are — and never re-registered. Likewise the driver's sixteen, which land
// with core/driver/ft891.
//
//  1. EMPTY-SLOT ANSWERS. A slot this fake holds no state for answers "?;" to
//     an MT read, to an MR read, and to an MC-set. The FT-710's equivalent is
//     HW-CONFIRMED for ITS MR frame (13/07/2026, docs/hardware-notes.md
//     §Empty/out-of-range slots) and that is a different frame on a different
//     radio; no FT-891 has been asked. Grammatically valid but
//     out-of-inventory slots ("100", "511") answer identically, since "?;" is
//     the protocol's single unattributed NAK. This is the same assumption
//     core/driver/ft891 makes from the other side — its register entries
//     "\"?;\" ON AN MR READ OF A MEMORY OR PMS SLOT MEANS THE SLOT IS EMPTY"
//     and "\"?;\" ON A 5xx/EMG DISCOVERY PROBE MEANS ABSENT FROM THIS RADIO"
//     — and it is what makes discovery mean anything.
//     STAGE R LIFTS IT WITH: one MT read and one MR read of a memory channel
//     the radio has never had written, plus one MC-set of the same slot. An
//     answer rather than "?;" — or a different NAK — moves this fake and the
//     driver's two entries together.
//     (parser.go: handleMT's read arm, handleMR, handleMC)
//
//  2. MT READ IS ANSWERED, BY DEFAULT. This fake answers an MT read of a
//     populated memory or PMS slot with the 41-byte combined frame, because
//     MT's own detail block prints a filled Read chart and a filled Answer
//     chart (ft891_layout.txt:1016, 1018-1027). THE CONTROL COMMAND LIST SAYS
//     THE OPPOSITE — "MT | MEMORY WRITE & TAG | Set O | Read X | Ans. X"
//     (166) — and both cannot be true. This radio is the only one of the four
//     registered Yaesu models whose two records disagree, and
//     core/cat/ft891/doc.go records the disagreement without resolving it.
//     The geometry witness found direct evidence that these charts DO contain
//     printing errors — three Set charts on folio 18 print two terminators in
//     one row (core/cat/ft891/testdata/provenance.md §Disagreements, item 8) —
//     which does not settle which record is wrong, only that "it is printed,
//     therefore it is true" is unavailable for either.
//     WHAT THIS FAKE DOES ABOUT IT: it plays BOTH radios.
//     WithMTReadUnsupported() honours the command list instead, leaving the
//     Set direction and MR untouched, which is the pair core/driver/ft891
//     turns into its typed whole-session refusal. So the driver's refusal
//     path is reachable against a real fake rather than a scripted
//     transcript, and neither reading is baked into this package.
//     STAGE R LIFTS IT WITH: ONE MT read of a known-populated memory channel
//     on a real FT-891. A well-formed 41-byte answer settles the
//     contradiction in the detail block's favour and lifts this entry; a
//     "?;", with the same slot's MR read returning a record in the same
//     session, confirms the command list and makes WithMTReadUnsupported()
//     the default rather than an option. THE SINGLE MOST VALUABLE CAPTURE ON
//     THIS RADIO — the driver register's entry "MT READ IS SUPPORTED FOR
//     MEMORY AND PMS" is lifted by the same frame.
//     (parser.go: handleMT's read arm; options.go: WithMTReadUnsupported)
//
//  3. P7 IN AN MT ANSWER IS '1' (Memory). MT's own legend prints "P7 0:
//     (Fixed)" (ft891_layout.txt:1011) and NO read vocabulary at all; the
//     only P7 read legend in this manual is MR's, "0: VFO 1: Memory" (976).
//     So the byte this fake puts in position 23 of a combined answer is read
//     ACROSS COMMANDS, which is the same inference core/cat/ft891/doc.go's
//     register entry "THE COMBINED ANSWER'S P7 READ DOMAIN" records for the
//     parse side — cited here, and this is the emitting half of it.
//     STAGE R LIFTS IT WITH: the SAME capture as entry 2 — one MT read of an
//     occupied memory channel, P7 read off the raw answer. Whatever byte is
//     there IS the read domain, and if it is neither '0' nor '1' both this
//     entry and the dialect's tolerance are wrong rather than merely wide.
//     (parser.go: kindMemory, buildMTAnswer)
//
//  4. PMS, 5 MHz AND EMG SLOTS ANSWER P7 '1' LIKEWISE. MR's legend has
//     exactly two members and no manual statement says which of them a PMS
//     band-edge, a 5 MHz channel or EMG answers with. '1' is the only member
//     that is not plainly false (none of the three is a VFO), so it is what
//     this fake serves for all of them. Note what this is NOT:
//     internal/fakeradio serves '5' (PMS) on the FT-710's MR answer, which
//     that radio's own wider P7 table (0-5) has a member for and which is an
//     FT-710 fact besides — this radio's legend documents two values, and
//     inventing a third would produce a frame core/cat would rightly refuse
//     to parse.
//     STAGE R LIFTS IT WITH: one MR read of a populated PMS slot (P1L),
//     programmed from the front panel first, and one of a populated 5 MHz
//     channel and one of EMG on a unit that HAS those banks. The P7 byte of
//     each answer is the fact. The 5 MHz and EMG halves ride on the same
//     capture as the driver register's "THE 5 MHz BANK'S PRESENCE ON A
//     U.K.-MARKET UNIT", and may well outlive the first FT-891 session.
//     (parser.go: kindMemory; image.go: defaultState)
//
//  5. THE CLARIFIER IS STORED, AND ROUND-TRIPS BYTE-FAITHFULLY. A combined MT
//     Set's P3 sign and magnitude and its P4 flag are stored exactly as they
//     arrived, and the next read answers those same bytes. This is a
//     DELIBERATE NON-BORROWING: internal/fakeradio stores zeros and ignores
//     what was sent, on an FT-710 HARDWARE finding (M5b write trials,
//     13/07/2026, and the spec.Inert write policy that rests on it). That is
//     one radio's observed behaviour on one command, and the FT-891's
//     capability matrix declines to borrow it for exactly this reason — no
//     FT-891 has been asked, so there is no finding to record. Storing what
//     was sent is the honest default for a field nobody has watched a radio
//     handle.
//     NOTE THE HALF THIS RADIO DOES NOT HAVE: there is no TX clarifier state
//     anywhere in this record. P5 is "0: (Fixed)" on all five blocks that
//     print it, so the FT-710's finding has no P5 counterpart here to borrow
//     or refuse.
//     STAGE W LIFTS IT WITH: one combined MT Set carrying a non-zero
//     clarifier offset to a channel whose stored offset is zero, then an MT
//     read of that channel. Zeros coming back mean this radio ignores the
//     field and this fake changes to match; the offset coming back confirms
//     it.
//     (parser.go: parseMemoryBlock; state.go: MemState.ClarSign/ClarMag)
//
//  6. AN MT SET CREATES AN ABSENT CHANNEL. A combined Set to a slot this fake
//     holds no state for stores the whole record, the display flag and the
//     tag, exactly as it would over an existing one. This is the fake's half
//     of core/driver/ft891's register entry "A SINGLE COMBINED MT SET
//     SUFFICES TO CREATE OR OVERWRITE A CHANNEL, INCLUDING AN EMPTY ONE", and
//     it has to be modelled because this driver has no other write path: an
//     MT-only driver against a fake that demanded an MW first could not write
//     at all.
//     STAGE W LIFTS IT WITH: the first write trial — one MT Set to a
//     verified-empty channel, then a read. It is the same capture that lifts
//     the driver's entry, and it lifts both or neither.
//     (parser.go: handleMT's Set arm)
//
//  7. SET-DIRECTION FIELD STRICTNESS. Every field vocabulary the position
//     charts print is enforced at the WIRE level, and a violation draws "?;"
//     with no state change: a frequency that is not 9 digits, a clarifier
//     sign that is not '+'/'-', a magnitude that is not 4 digits, an RX
//     clarifier flag that is not '0'/'1', a P5 that is not the fixed '0', a
//     mode nibble outside the legend's 1-9 and B-D plus the '0' placeholder,
//     a P7 that is not the Set chart's fixed '0', a P8 outside 0-2, a P9 that
//     is not "00", a P10 outside 0-2, a P11 outside the TAG flag's '0'/'1',
//     or a tag byte this manual's own folio-2 rule excludes. Whether a real
//     FT-891 REJECTS such a frame — rather than rounding it, ignoring the
//     field, or storing it verbatim — is unobserved for every one of them.
//     TWO BOUNDARIES ARE DELIBERATELY THE MANUAL'S RATHER THAN THE PROJECT'S,
//     and each is stated at the code: the clarifier magnitude is checked
//     against the printed "0000 - 9999" and NOT against the dialect's deduced
//     10 Hz step and 9990 ceiling, and the tag field is checked against
//     folio 2's "any character except the ASCII control codes (00 to 1Fh) and
//     the terminator (;)" (ft891_layout.txt:93-96) rather than the narrower
//     printable-ASCII default the capability table takes. In both the fake
//     accepts what the radio's manual describes and the project refuses to
//     BUILD something narrower, which is the honest direction for a test
//     double. The tag charset check is separately SAFETY-CRITICAL and stays
//     whatever hardware turns out to do: accepting ';' would make command
//     injection through a tag possible.
//     STAGE W LIFTS IT WITH: one Set per class carrying a deliberately
//     off-vocabulary field, the reply recorded and the channel read back —
//     "?;" confirms an entry, silence-then-changed-value refutes it, and
//     silence-then-unchanged means the radio ignored the field. Expect this
//     entry to split into several when it is finally taken.
//     (parser.go: the validators, parseMemoryBlock, handleMT's Set arm)
//
//  8. THE TAG IS STORED TRIMMED AND ANSWERED PADDED. The combined record's
//     P12 is a fixed 12-byte field in both directions. This fake stores the
//     tag with trailing fill trimmed and re-pads it to the full width on
//     every answer, so an all-fill field means "no tag" and a Set-to-read
//     round trip is byte-faithful over the tag field. The FILL BYTE is a
//     space because the DIALECT says so — its register entry
//     "MTPolicy.TagFill = ' '", ASSUMED and cited here, never re-derived.
//     What has no analogue on this radio, and is therefore NOT modelled
//     rather than assumed: the FT-710's HW-confirmed rejection of a
//     zero-byte-tag MT Set. The combined form cannot express a zero-byte tag
//     at all — a 41-byte frame always carries the full field — so there is no
//     shape to accept or refuse.
//     STAGE R LIFTS IT WITH: the dialect's own TagFill capture (one MT Set of
//     a tag shorter than 12 characters, then an MT read of that channel),
//     which reports the fill byte and the answer's width together. This fake
//     follows whatever it says.
//     (state.go: MemState.Tag; parser.go: buildMTAnswer, handleMT's Set arm)
//
//  9. A SET DOES NOT MOVE THE SELECTED CHANNEL. Only an MC-set changes what
//     "MC;" answers. internal/fakeradio's MW moves it, hands-off, on an
//     FT-710 HARDWARE finding (M5b, 13/07/2026: "a successful MW moves the
//     radio's selection to the written slot") that core/clone's selection
//     snapshot/restore is built around. That is one radio's observed
//     behaviour on one command and it is NOT borrowed here — inventing a side
//     effect for a radio nobody has written to is exactly the class of
//     borrowed fact this milestone refuses.
//     STAGE W LIFTS IT WITH: one MT Set, then "MC;", on a real FT-891.
//     (parser.go: handleMT's Set arm, handleMC)
//
//  10. AN ACCEPTED SET PRODUCES NO REPLY; A REJECTED ONE PRODUCES EXACTLY ONE
//     "?;". Fire-and-forget on success, for the MT Set, MC-set and AI-set
//     alike. The FT-710's manual states this as a general framing rule and
//     this fake inherits it; the FT-891's manual is cited in this repository
//     for frame SHAPES (its command list and position charts), not for an
//     acknowledgement convention, and the capability matrix records the same
//     absence from the other side. NOTE THE TRAP IT AVOIDS: MT's availability
//     row marks which FORMS the command has, not what a Set draws back — the
//     "Ans." column marks the existence of the ANSWER FORM, which is why
//     read-only MR carries "Ans. O" too. Reading a reply convention off that
//     row is a mistake this project has made once before and does not repeat.
//     core/driver/ft891's write path depends on the silence, so a radio that
//     acknowledged would break the driver, not just this fake.
//     STAGE W LIFTS IT WITH: one MT Set to a real FT-891 with the port
//     watched for any reply at all before the next command is sent — the
//     capture that also lifts the driver register's acknowledgement entry.
//     (fakeft891.go: handleEvent; parser.go: every Set arm returning nil)
//
//  11. THE DEFAULT IMAGE'S CONTENT IS INVENTED. Memory 001 at 7.000000 MHz
//     LSB, memory 002 at 14.250000 MHz USB tagged "TWENTY" with its TAG
//     display ON, the nine PMS pairs at plausible IARU Region 1 band edges,
//     With5MHz's placeholder 5 MHz channels and WithEMG's 5.1675 MHz — all
//     placeholders, carried over from internal/fakeradio's own invented
//     values, not sourced from any programmed FT-891, because none has been
//     read. The SHAPE is what these fixtures exist for: a plain memory
//     channel, a tagged one whose display flag is ON, a populated PMS pair, a
//     sparse 5 MHz bank, an EMG channel. The 5.1675 MHz figure is the
//     well-known conventional Alaska emergency frequency, used as a plausible
//     placeholder only; the sparseness of With5MHz's set is a deliberate test
//     property (see its doc comment), not a claim about any radio's
//     inventory; and BOTH VALUES OF BYTE 28 appear deliberately, because a
//     default image that never set the flag would leave this radio's one new
//     axis unexercised everywhere the default fake is read.
//     THIS MATTERS BEYOND THE TEST SUITE: a --fake read renders these values
//     to a user, who must not read them as what an FT-891 ships with.
//     STAGE R LIFTS IT WITH: a full MT sweep of a factory-condition FT-891 —
//     which reports that radio's inventory, and one radio's inventory is not
//     the model's, so expect this entry to stay ASSUMED with better
//     placeholders rather than to retire.
//     (image.go: DefaultImage, With5MHz, WithEMG)
//
//  12. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than 256 bytes have
//     accumulated without a ';', this fake replies "?;" once and discards
//     bytes up to and including the next ';' before resuming normal framing.
//     NOT a radio claim and NOT liftable by any capture: it is this package's
//     own bounded-input policy, inherited from internal/fakeradio's, recorded
//     here so that a test relying on it knows what it is relying on.
//     (parser.go: reassembler)
//
//  13. THE "?;" REJECTION CONVENTION ITSELF IS INHERITED. Entries 1, 2 and 7
//     all say what draws a rejection; this one records that the
//     rejection's very EXISTENCE is an assumption on this radio. "?;" is
//     core/cat's ErrRejected, adopted from the FT-710's reference
//     (core/cat/errors.go:10-19), and NO FT-891 LINE IS CITED FOR IT ANYWHERE
//     IN THIS REPOSITORY — not in core/cat/ft891/doc.go's reused-command
//     verification, not in its chart-defect list, and not in the geometry
//     witness's provenance note, which records eight disagreements found
//     inside this manual and no reply convention among them. A radio that
//     stayed SILENT instead would leave this fake's rejections
//     indistinguishable from a dead link, and would turn every one of
//     core/driver/ft891's rejection-based interpretations — discovery's
//     "absent", the cross-check's "empty", and the whole-session refusal —
//     into timeouts.
//     STAGE R LIFTS IT WITH: one deliberately unknown command — "ZZ;" — put
//     to a real FT-891 with the port watched. Whatever comes back, or does
//     not, is the convention.
//     (parser.go: rejection, and every refusal in the file)
//
//  14. AUTOMATIC-INFORMATION SUPPRESSION: THIS FAKE NEVER PUSHES AN
//     UNSOLICITED FRAME, WHATEVER AI IS SET TO. "AI1;" is accepted, stored
//     and read back faithfully, and then nothing follows from it: this radio
//     writes to the port only in reply to a frame that arrived on it, so an
//     AI-on session and an AI-off session are byte-identical on the wire. The
//     assumption is not that this radio is silent — it is that MODELLING IT
//     AS SILENT IS THE HONEST DEFAULT, because NO FT-891 HAS BEEN OBSERVED
//     WITH AI ON. Which frames such a radio volunteers, on what triggers, in
//     what order and with what interleaving against a command in flight are
//     four unknowns, and inventing them would put fabricated wire behaviour
//     underneath every test that ran against this fake — worse than usual
//     here, because an invented push is indistinguishable at the far end of a
//     link from a transport defect.
//     DISTINCT FROM THE MANUAL FACT NEXT DOOR: "AI defaults to off at
//     construction" is ft891_layout.txt:231 and is deliberately NOT in this
//     register. That line says what AI is set to at power-on; it says nothing
//     about what a radio does once AI is ON, which is the whole of this
//     entry. The two must not be read as one absence.
//     WHAT RESTS ON IT: core/transport.Engine's drain-to-quiet discipline is
//     exercised against internal/fakeradio, whose AI-flood behaviour is the
//     FT-710's OWN observed one, and NOT against this fake. So no FT-891 test
//     in this repository exercises the engine against a talking radio, and
//     none may claim to: every FT-891 exchange in every suite is one frame
//     in, at most one frame out.
//     STAGE R LIFTS IT WITH: one session on a real FT-891 with AI set to 1
//     and the port then watched — idle, and while the front panel is operated
//     (VFO turned, mode changed, memory recalled). Whatever that radio
//     pushes, and whatever provokes it, becomes this fake's model; NOTHING
//     MAY BE INVENTED MEANWHILE, and an observation of nothing at all is a
//     result to record rather than a failed capture. RECORD WHICH SERIAL
//     DEVICE IT WAS TAKEN ON: this radio reaches CAT over a built-in USB to
//     DUAL UART bridge (ft891_layout.txt:24-27) and the manual never says
//     which of the two endpoints carries CAT, so a capture that cannot name
//     its port settles nothing about silence.
//     (parser.go: handleAI and the AI section's note; fakeft891.go: serve and
//     handleEvent, whose only write is a reply)
//
//  15. THE EX MENU VALUES ARE INVENTED. Every menu item this fake answers
//     reads back n x '0', n being the width transcription B's digits column
//     prints for it. The chart documents each item's VALID RANGE and its
//     option legends and NEVER a shipped default, so there is nothing to
//     source a real one from. The uniformity is the point: a placeholder that
//     is obviously uniform is harder to mistake for evidence than a
//     plausible-looking spread of values, and this matters beyond the test
//     suite, because `rigprog read --settings --fake --model FT-891` renders
//     these bytes to a user who must not read them as what an FT-891 ships
//     with. It is internal/fakeradio's convention, adopted whole.
//     WHAT IS NOT ASSUMED HERE IS THE WIDTH. Each item's field width is
//     transcribed, not guessed — including the one place the transcription is
//     known to be transcribing a printed DEFECT: 0905 RPT SHIFT 50MHz prints
//     Digits 1 against a legend that needs four, and this fake answers the
//     printed one byte. That is a recorded state (PROVENANCE.md;
//     core/cat/ft891/crosscheck_test.go pins it from the other side), not an
//     assumption of this register, because no comparison in this repository
//     can catch a defect all three derivations read faithfully.
//     STAGE R LIFTS IT WITH: an EX read sweep of a factory-condition FT-891.
//     Expect this entry to stay ASSUMED with better placeholders rather than
//     to retire — one radio's menu is not the model's — but the SAME sweep
//     settles 0905's width outright, which is the more valuable half.
//     (ex.go: exDefaultDigit, buildEXDefaults)
//
//  16. AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". A syntactically valid
//     four-digit address that transcription B never enumerated — past the end
//     of a real group, or a group prefix the chart has no rows for at all —
//     draws the same unattributed NAK an empty slot does. Membership comes
//     from the chart's own rows via the generated inventory, and NOT from the
//     EX block's printed bound: that block prints "P1 : 0101 - 1803 (MENU
//     Number)", which happens to be exactly the first and last rows
//     transcribed, so enforcing it as a range would add a second authority
//     over one fact and would disagree with the inventory the moment the two
//     were edited apart.
//     THIS IS ASSUMED HERE WHERE THE FT-710'S IS OBSERVED: M8c put two
//     out-of-chart EX addresses to a real FT-710 and both drew "?;"
//     (docs/hardware-notes.md), which is that radio's finding on that radio's
//     six-digit grammar, and is not borrowed. No FT-891 has been asked.
//     STAGE R LIFTS IT WITH: one EX read of an address the chart does not
//     carry — "EX1901;" will do, the group after the chart's last — with the
//     port watched. An answer rather than "?;" would mean the chart
//     under-describes this radio's menu, which would be a finding about the
//     transcription as much as about the fake.
//     (ex.go: handleEX; options.go: WithEXUnavailable, which reaches this
//     same answer for a known address without inventing a new behaviour)
//
// # What is NOT in this register, and why
//
// Several behaviours a reader might expect to find registered as assumptions
// are MANUAL FACTS for this radio. They are listed here, with the line that
// makes each one a fact, so that the absences read as decisions rather than
// as oversights:
//
//   - AI DEFAULTS TO OFF AT CONSTRUCTION. "This parameter is set to '0' (OFF)
//     automatically when the transceiver is turned 'OFF'" (ft891_layout.txt:
//     231, inside AI's own block at 226-235). New models a freshly-powered
//     radio, and that is this manual's own statement rather than an
//     inheritance. What is NOT covered by this absence is what the fake does
//     once AI is turned ON — AUTOMATIC-INFORMATION SUPPRESSION above, and the
//     two must not be read as one absence.
//   - P5 IS THE SCHEMA BYTE '0', REQUIRED ON A SET AND EMITTED IN EVERY
//     ANSWER. "P5 0: (Fixed)", printed on all five blocks that carry the
//     field (MR 971, MT 1006, MW 1042, IF 783, OI 1129). This fake follows
//     the legend. That a REAL FT-891 answers '0' is a separate claim and it
//     is the DRIVER register's entry "P5 IS ANSWERED '0'", not this one's —
//     which is why MemState carries a P5 field at all: so that the radio
//     which refutes it can be played here.
//   - MT NAMES MEMORY AND PMS AND NO OTHER SLOT CLASS, in both directions
//     (ft891_layout.txt:998-999), where its own MR block names two more
//     (960-964). internal/fakedx10 registers the opposite as a LENIENCY
//     because that radio's MT legend names all four classes; here the
//     narrowness is the legend, transcribed.
//   - MC NAMES MEMORY AND PMS ONLY (907-909), where every registered
//     sibling's MC legend prints 5xx and EMG as well.
//   - THE 5 MHz BANK IS 501-510. MR's legend prints the actual numbers (962),
//     repeated by IF (776) and OI (1122), where the FTdx10's and the FT-710's
//     print only "5xx" and their 501..599 sits on their own ASSUMED
//     registers. core/cat/ft891/doc.go says the same thing in its own "WHAT
//     IS DELIBERATELY NOT AN ENTRY" section, and this package inherits the
//     transcription rather than re-deriving it.
//   - THE MODE LEGEND'S HOLE AT 'A', AND THE ABSENCE OF 'E' AND 'F'. All
//     three printings run "... 9: RTTY-USB A: - B: FM-N ..." (972-974,
//     1007-1010, 1043-1046): the dash is the chart's way of printing "nothing
//     here". A mode nibble outside 1-9 and B-D is refused because this
//     manual's legend does not print it, not because anything is assumed.
//   - MR HAS NO SET DIRECTION. The command list gives it "X O O X" (164), so
//     a 28-byte MR frame in the Set shape is an unknown frame rather than a
//     write.
//   - THE TAG FIELD'S BYTE RULE. "the parameter digits should be filled using
//     any character except the ASCII control codes (00 to 1Fh) and the
//     terminator (;)" (93-96, printed folio 2). What IS assumed about the tag
//     is the strictness of ENFORCING it on a Set — entry 7 — not the rule.
//   - COMMAND NAMES ARE ACCEPTED IN EITHER CASE. "A command consists of 2
//     alphabetical characters. You may use either lower or upper case
//     charac-/ters." (hyphenated across the column break, ft891_layout.txt:
//     100-102, under the "Alphabetical Commands" heading at 99) — the same
//     sentence, in the same words, that the FTdx10's manual states
//     (ftdx10_layout.txt:159-161) and that internal/fakedx10 already honours.
//     A former register entry read the absence of this sentence from a
//     hyphen-defeated search and a summary row that quoted only its first
//     half; it was a defect against this radio's own manual, not a cautious
//     strictness, corrected the day the folio was re-read directly. Mixed
//     case (e.g. "Mt001;") is admitted too, as a CONSEQUENCE of folding each
//     of the two command bytes independently — "either lower or upper" does
//     not itself license mixing, but per-character folding yields it anyway,
//     and that is stated rather than left implicit
//     (TestCommandNamesAreAcceptedInEitherCase). FIELD values remain
//     case-sensitive — the mode nibble's hex letters, the PMS L/U suffix,
//     "EMG" — which the manual's sentence never touches.
package fakeft891

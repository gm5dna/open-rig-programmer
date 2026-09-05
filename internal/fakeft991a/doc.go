// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeft991a simulates a Yaesu FT-991A's CAT behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double the
// FT-991A's own layers run against: the transport engine, core/driver/ft991a,
// the CLI's --fake mode, and the GUI's demo mode — the role
// internal/fakeradio plays for the FT-710, internal/fakedx10 for the FTdx10
// and internal/fakeft891 for the FT-891.
//
// The radio it speaks is a COMBINED-form one whose memory record has ONE SLOT
// NUMBER LINE and a FIVE-STATE TONE BYTE. Its MT command carries the whole
// memory record and the tag in a single 41-byte frame, in both directions; its
// slot space is the printed span 001-117, whose upper eighteen are the nine PMS
// pairs as DECIMAL CHANNEL NUMBERS rather than a token form; and its P8 prints
// five values where every registered sibling prints three. MR answers the
// shared 28-byte memory frame for every slot its own legend lists, MC recalls
// that whole span, ID answers "ID0670;" and AI is accepted and readable.
//
// EX (MENU) IS MODELLED, READ ONLY, over the 152 addresses this radio's chart
// gives a parameter to — a THREE-digit address, the narrowest read frame in the
// family at six bytes — from a table generated out of this package's own copy
// of transcription B (ex.go). An EX Set is not modelled and answers "?;", as an
// MW frame does; see "What this fake deliberately does NOT model" below.
//
// # The hard rule: NOTHING project-internal
//
// fakeft991a MUST NOT import any package of this project — not core/cat, not
// core/cat/ft991a, not core/codeplug, not core/spec, and not
// internal/fakeradio or any sibling fake. Standard library only, in every
// non-test file, in this directory AND every directory beneath it. The fence
// was RECURSIVE FROM BIRTH, ahead of the subdirectory it had to cover:
// internal/fakeft991a/gen — the stdlib-only generator for this radio's
// transcription B — is the piece most likely to reach for internal/extable, the
// A-side machinery whose Digits parsing was a known defect locus, which is
// exactly the import this package must not have (one parser on both sides of
// the EX cross-check would reproduce a shared parsing bug into both inventories
// invisibly).
// TestScanForbiddenImports_CatchesAForbiddenImportInASubdirectory proved the
// fence would bite there before the directory existed, and now that it does,
// TestNoCoreImports_ReachesTheGenerator asserts by PATH that the real scan
// reaches the real gen/main.go.
//
// Every byte offset, field width and validation rule below is re-derived from
// the FT-991A CAT Operation Reference Manual's own position charts (revision
// 1711-D), as counted by evidence leg G — the blind geometry witness whose
// records are core/cat/ft991a/testdata/{mt,mr,mc,mw,ex}-vectors.golden and
// provenance.md.
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused core/cat's codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake has
// to be able to DISAGREE with the production codec for a test against it to
// mean anything, and it can only disagree if it was built from the manual
// rather than from the code.
//
// # A SIBLING of internal/fakeft891, not a refactor of it
//
// This package duplicates a good deal of internal/fakeft891's shape: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Image contract, the options, the recursive import
// fence and this register's own machinery. That duplication is deliberate and
// it is not going to be factored into a shared "fake core" package.
//
// Two reasons, both load-bearing. The first is mechanical: a shared helper
// package would be a project-internal import, which the hard rule above
// forbids in every fake, so the only way to share code would be to abandon the
// property that makes any of them worth having. The second is the reason
// core/cat/ft991a/dialect.go's mode table is typed out afresh rather than
// borrowed (see its doc comment): two radios agreeing on a frame shape is a
// fact about those two radios, not a shared definition — and THIS RADIO IS THE
// PROOF, because it disagrees with the FT-891 at four of the five bytes that
// matter and disagrees in BOTH directions.
//
// Where this fake DIVERGES from internal/fakeft891, the divergence is this
// manual's legend, and each is stated at the code that implements it:
//
//   - THE PMS SLOTS ARE DECIMAL CHANNEL NUMBERS, 100-117. The MC legend prints
//     "100: P-1L 101: P-1U ~ 116: P-9L 117: P-9U" (ft991a_layout.txt:916)
//     where every registered sibling's prints "P1L - P9U (PMS)". So "P1L" is
//     not a slot form here and is refused (parser.go's parseSlotForm,
//     image.go's pmsSlot). It is the cell a copy-paste from that package gets
//     wrong SILENTLY: eighteen token strings would be eighteen non-slots, not
//     a compile error.
//   - THERE IS NO 5 MHz BANK AND NO EMERGENCY CHANNEL. "5xx", "5 MHz" and
//     "EMG" appear in no slot legend of this manual, checked mechanically over
//     the whole extraction (core/cat/ft991a/dialect.go:114-123, :141-149),
//     where the FT-891's MR legend prints both. So there is no With5MHz, no
//     WithEMG, and no discovery for a driver to walk.
//   - BYTE 21 IS A LIVE TX CLARIFIER FLAG. `P5 0: TX CLAR "OFF" 1: TX CLAR
//     "ON"` is printed on all five blocks that carry the field (MR 971, MT
//     1004, MW 1042, IF 787, OI 1122) where the FT-891 prints "0: (Fixed)", so
//     there IS a TX clarifier state to store in both directions
//     (state.go's MemState.TXClar, parser.go's parseMemoryBlock).
//   - BYTE 28 IS SCHEMA, NOT A TAG DISPLAY FLAG — the exact inversion of the
//     previous point. "P11 0: (Fixed)" (1015) where the FT-891 prints
//     `0: TAG "OFF" 1: TAG "ON"`, so there is no per-channel display flag to
//     store and a Set carrying anything but '0' there is refused
//     (parser.go's combinedP11Fixed).
//   - BYTE 24 HAS FIVE STATES, NOT THREE. `P8 0: CTCSS "OFF" 1: CTCSS ENC/DEC
//     2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC`, printed identically on all five
//     blocks that carry it (IF 795-796, MR 977-978, MT 1010-1011, MW
//     1048-1049, OI 1128-1129). This is the first Yaesu memory record in this
//     fleet with a DCS state, and it is the axis this radio exists to exercise
//     (parser.go's validCTCSSByte, options.go's WithDCSChannels).
//   - MT'S TWO DIRECTIONS AGREE, so there is no WithMTReadUnsupported(). The
//     availability row gives MT "O O O X" (181) and its detail block prints a
//     Read chart and a full Answer chart (998-1033); the FT-891's two records
//     contradict each other and its fake plays both radios. Nothing to play
//     here.
//   - THE MODE LEGEND HAS NO HOLE AND NO 'F'. All five printings run 1..9 then
//     A..E with every nibble named (973-975, 1006-1008, 1044-1046, 789-791,
//     1124-1126), where the FT-891 prints "A: -" and the FTdx10 fills 'F'.
//     Its 'E' is C4FM where the FTdx10's is PSK — one nibble, two different
//     real modes.
//   - No fault injection, no MW and no EX — see the next section.
//
// # What this fake deliberately does NOT model
//
// AN EX (MENU) SET. This radio documents EX in both directions — availability
// 155, block 519-528, "P1 : 001 - 153 (MENU Number)", a THREE-digit address
// whose Read frame "EX P1 P1 P1 ;" is SIX bytes, the narrowest in the family —
// and this fake answers READS only. A Set-shaped frame ("EX0010001;") is simply
// a too-long body to handleEX and draws "?;". That is a MODELLING GAP,
// KNOWN-DIVERGENT from the documented grammar, and it is NOT a claim that this
// radio refuses an EX Set. Nothing above this fake sends one: this milestone's
// plan has core/driver/ft991a's settings path as read-only, and the settings
// WRITE work is a separate, later milestone with its own bench evidence.
// TestEX_SetsAreNotModelled pins the gap's shape, including that the refused
// Set leaves the stored value untouched.
//
// MW (MEMORY CHANNEL WRITE). This radio documents MW — Set only, no Read and
// no Answer (availability 183), its Set frame the 28-position MR chart under an
// "MW" prefix (1036-1051) — and this fake does not implement it, so an MW frame
// answers "?;". That is a MODELLING GAP, KNOWN-DIVERGENT from the documented
// grammar, and it is not a claim that this radio lacks the command. It is out
// of scope by this milestone's plan, which has core/driver/ft991a's write
// path as MT-only (one combined Set carries the field block and the tag,
// where MW could carry neither the tag nor the P11 position); the dialect's
// own MW coverage is its golden vectors and its conformance tests in
// core/cat/ft991a, and no layer above this fake sends one. internal/fakedx10
// models MW for a radio-fidelity
// reason this note declines, and its handler is the template if a later task
// wants one here.
//
// FAULTS. internal/fakeradio carries a scripted misbehaviour set
// (FaultDropReplies, FaultGarbleReply, FaultSpuriousFrame,
// FaultDelayedRejection, FaultDelayedReply, FaultDisconnect,
// FaultChunkedReplies) and this package carries none of them. The omission is
// deliberate: those faults exercise core/transport.Engine's timeout, resync,
// drain-to-quiet and chunk-reassembly behaviour, which is MODEL-INDEPENDENT —
// the engine is one implementation, already covered by fakeradio's fault suite,
// and a second copy of the same scripting would test the same engine twice
// while doubling the surface a reviewer must read. No wiring, CLI or GUI path
// uses faults on a fake rig. WithLatency IS kept, because it is not a fault: it
// is the knob Close's promptness is proven against (Radio.shutdown,
// TestClose_IsPromptDespiteAPendingLatency).
//
// A FRONT PANEL. Nothing here models one, so the selection moves only by an
// MC-set and the memory contents change only by an MT Set.
//
// TIMING. Every reply is near-instant unless WithLatency says otherwise. No
// FT-991A timing has ever been observed by this project, so there is nothing to
// model.
//
// # The ASSUMED register
//
// NO FT-991A HARDWARE HAS EVER BEEN ASKED ANYTHING by this project — the
// statement core/cat/ft991a/doc.go opens with, and it governs this package
// twice over. That package at least transcribes a manual; this one has to
// decide what a radio DOES at the protocol's edges, and this manual documents
// almost none of them. Every place this fake had to guess is listed here, in
// one place, with the ONE Stage R or Stage W capture that lifts it, so that a
// reviewer — or the first real FT-991A session — has a single list to work from
// rather than a source-wide comment hunt. Each entry also appears as an inline
// comment beside the code that implements it. That completeness claim is the
// register's whole value and it is made without exception: see also "What is
// NOT in this register, and why" at the end, which holds the manual facts a
// reader might expect to find here — an absence with a manual line behind it is
// not a gap in the claim above.
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION — the rule
// core/cat/ft991a/doc.go's own register states in terms, and which this
// package's code follows at every site. The numbering below is for
// readability: a positional citation is correct only until somebody adds or
// reorders an entry, and it then silently points at the wrong assumption rather
// than failing. register_test.go's TestASSUMEDRegisterIsComplete makes the rule
// hold rather than merely be asked for.
//
// ASSUMPTIONS THAT BELONG TO THE DIALECT are CITED here where this fake depends
// on them and NEVER RE-REGISTERED. core/cat/ft991a/doc.go's register carries
// ELEVEN, and this package depends on EIGHT of them: "MTPolicy.TagFill = ' '"
// (parser.go's tagFill), "THE COMBINED MT ANSWER'S EXACT LENGTH, 41"
// (buildMTAnswer), `SlotSpace.NoneWire = "000"` (slotNoneWire),
// "THE cat.ModeUnset MEMBER OF THE MODE TABLE" (validModeWireByte),
// "ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990" and
// "THE CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D ('-')"
// (validClarMagDigits and validClarSign), "THE DCS STATES' SET ACCEPTANCE"
// (validCTCSSByte) and "THE ACKNOWLEDGEMENT CONVENTIONS" (parser.go's rejection
// and fakeft991a.go's handleEvent). THAT LAST ONE IS WHY THIS REGISTER HAS NO
// "?;" ENTRY AND NO SILENCE-ON-ACCEPT ENTRY where internal/fakeft891's has
// both: the FT-891's dialect register does not carry an acknowledgement entry
// and this radio's does, so the statement of record already exists and a second
// copy here would be a second authority over one fact.
//
//  1. EMPTY-SLOT ANSWERS. A slot this fake holds no state for answers "?;" to
//     an MT read, to an MR read, and to an MC-set. The FT-710's equivalent is
//     HW-CONFIRMED for ITS MR frame (13/07/2026, docs/hardware-notes.md
//     §Empty/out-of-range slots) and that is a different frame on a different
//     radio; no FT-991A has been asked. Grammatically valid but
//     out-of-inventory slots answer identically, since "?;" is the protocol's
//     single unattributed NAK. This is the same assumption core/driver/ft991a
//     makes from the other side — its register entry "MT \"?;\" ON A MEMORY OR
//     PMS SLOT MEANS THE SLOT IS EMPTY" — and on this radio that is the driver's
//     ONLY interpretation of a "?;", because it sends no other frame that could
//     draw one from a slot.
//     STAGE R LIFTS IT WITH: one MT read of a memory channel known-empty from
//     the front panel and one of a known-populated channel, in the same
//     session, plus one MC-set of the empty one. An answer rather than "?;" —
//     or a different NAK — moves this fake and the driver's entry together.
//     (parser.go: handleMT's read arm, handleMR, handleMC)
//
//  2. PMS SLOTS ANSWER P7 '1'. The P7 read legend has exactly two members,
//     "0: VFO 1: Memory" (ft991a_layout.txt:1009 for MT, 976 for MR, 1127 for
//     OI), and no manual statement says which of them a PMS band-edge answers
//     with. '1' is the only member that is not plainly false — a PMS slot is
//     not a VFO — so it is what this fake serves for 100-117 as well as for
//     001-099. Note what this is NOT: IF's own P7 legend runs to seven values
//     and HAS a "5: PMS" member (792-793), and it is deliberately not read
//     across, because IF's parameter describes what the VFO is doing and MR's
//     and OI's are the narrow pair this record's answers belong to. Inventing a
//     third value would produce a frame core/cat would rightly refuse to parse.
//     THE MEMORY HALF IS NOT ASSUMED at all, unlike internal/fakeft891's: this
//     manual prints both directions in one legend, "P7 Set: 0: (Fixed) / Read:
//     0: VFO 1: Memory".
//     STAGE R LIFTS IT WITH: one MT read of a populated PMS slot — 100, say,
//     programmed from the front panel first. The P7 byte of that answer is the
//     fact.
//     (parser.go: kindMemory)
//
//  3. THE CLARIFIER IS STORED, AND ROUND-TRIPS BYTE-FAITHFULLY. A combined MT
//     Set's P3 sign and magnitude and BOTH its flags — P4 RX and P5 TX — are
//     stored exactly as they arrived, and the next read answers those same
//     bytes. This is a DELIBERATE NON-BORROWING: internal/fakeradio stores
//     zeros and ignores what was sent, on an FT-710 HARDWARE finding (M5b write
//     trials, 13/07/2026, and the spec.Inert write policy that rests on it).
//     That is one radio's observed behaviour on one command, and this radio's
//     capability matrix declines to borrow it for exactly this reason — no
//     FT-991A has been asked, so there is no finding to record. Storing what
//     was sent is the honest default for a field nobody has watched a radio
//     handle.
//     NOTE THE HALF THIS RADIO HAS THAT THE FT-891 DOES NOT: P5 is a live TX
//     clarifier flag on all five blocks that print it, so the FT-710's finding
//     has a P5 counterpart here — and it is refused as an inheritance for the
//     same reason as the rest.
//     STAGE W LIFTS IT WITH: one combined MT Set carrying a non-zero clarifier
//     offset and both flags set, to a channel whose stored offset is zero, then
//     an MT read of that channel. Zeros coming back mean this radio ignores the
//     fields and this fake changes to match; the offset and the flags coming
//     back confirm it.
//     (parser.go: parseMemoryBlock; state.go: MemState.ClarSign/ClarMag/TXClar)
//
//  4. AN MT SET CREATES AN ABSENT CHANNEL. A combined Set to a slot this fake
//     holds no state for stores the whole record and the tag, exactly as it
//     would over an existing one. It has to be modelled because this driver has
//     no other write path: an MT-only driver against a fake that demanded an MW
//     first could not write at all.
//     STAGE W LIFTS IT WITH: the first write trial — one MT Set to a
//     verified-empty channel, then a read.
//     (parser.go: handleMT's Set arm)
//
//  5. SET-DIRECTION FIELD STRICTNESS. Every field vocabulary the position
//     charts print is enforced at the WIRE level, and a violation draws "?;"
//     with no state change: a frequency that is not 9 digits, a clarifier sign
//     that is not '+'/'-', a magnitude that is not 4 digits, an RX or TX
//     clarifier flag that is not '0'/'1', a mode nibble outside the legend's
//     1-9 and A-E plus the '0' placeholder, a P7 that is not the Set chart's
//     fixed '0', a P8 outside the five printed states, a P9 that is not "00", a
//     P10 outside 0-2, a P11 that is not the fixed '0', or a tag byte this
//     manual's own folio-2 rule excludes. Whether a real FT-991A REJECTS such a
//     frame — rather than rounding it, ignoring the field, or storing it
//     verbatim — is unobserved for every one of them.
//     TWO BOUNDARIES ARE DELIBERATELY THE MANUAL'S RATHER THAN THE PROJECT'S,
//     and each is stated at the code: the clarifier magnitude is checked
//     against the printed "0000 - 9999" and NOT against the dialect's deduced
//     10 Hz step and 9990 ceiling, and the tag field is checked against folio
//     2's "any character except the ASCII control codes (00 to 1Fh) and the
//     terminator (;)" (ft991a_layout.txt:106-109) rather than the narrower
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
//     entry to split into several when it is finally taken, and the P8 half to
//     go first, since the dialect's own "THE DCS STATES' SET ACCEPTANCE" asks
//     the same question of the same byte.
//     (parser.go: the validators, parseMemoryBlock, handleMT's Set arm)
//
//  6. THE TAG IS STORED TRIMMED AND ANSWERED PADDED. The combined record's P12
//     is a fixed 12-byte field in both directions. This fake stores the tag with
//     trailing fill trimmed and re-pads it to the full width on every answer, so
//     an all-fill field means "no tag" and a Set-to-read round trip is
//     byte-faithful over the tag field. The FILL BYTE is a space because the
//     DIALECT says so — its register entry "MTPolicy.TagFill = ' '", ASSUMED and
//     cited here, never re-derived.
//     What has no analogue on this radio, and is therefore NOT modelled rather
//     than assumed: the FT-710's HW-confirmed rejection of a zero-byte-tag MT
//     Set. The combined form cannot express a zero-byte tag at all — a 41-byte
//     frame always carries the full field — so there is no shape to accept or
//     refuse.
//     STAGE R LIFTS IT WITH: the dialect's own TagFill capture (one MT Set of a
//     tag shorter than 12 characters, then an MT read of that channel), which
//     reports the fill byte and the answer's width together. This fake follows
//     whatever it says.
//     (state.go: MemState.Tag; parser.go: buildMTAnswer, handleMT's Set arm)
//
//  7. A SET DOES NOT MOVE THE SELECTED CHANNEL. Only an MC-set changes what
//     "MC;" answers. internal/fakeradio's MW moves it, hands-off, on an FT-710
//     HARDWARE finding (M5b, 13/07/2026: "a successful MW moves the radio's
//     selection to the written slot") that core/clone's selection
//     snapshot/restore is built around. That is one radio's observed behaviour
//     on one command and it is NOT borrowed here — inventing a side effect for
//     a radio nobody has written to is exactly the class of borrowed fact this
//     milestone refuses.
//     STAGE W LIFTS IT WITH: one MT Set, then "MC;", on a real FT-991A.
//     (parser.go: handleMT's Set arm, handleMC)
//
//  8. THE DEFAULT IMAGE'S CONTENT IS INVENTED. Memory 001 at 7.000000 MHz LSB,
//     memory 002 at 14.250000 MHz USB tagged "TWENTY", PMS pairs 1 and 9 at
//     plausible IARU Region 1 band edges, and WithDCSChannels' two FM channels
//     — all placeholders, carried over from internal/fakeradio's own invented
//     values, not sourced from any programmed FT-991A, because none has been
//     read. The SHAPE is what these fixtures exist for: a plain memory channel,
//     a tagged one, a populated PMS pair at each END of this radio's numbering
//     with the seven pairs between them left EMPTY, and — behind an option, by
//     this milestone's plan decision P14 — a channel in each of the two DCS
//     states.
//     THIS MATTERS BEYOND THE TEST SUITE: a --fake read renders these values to
//     a user, who must not read them as what an FT-991A ships with.
//     STAGE R LIFTS IT WITH: a full MT sweep of a factory-condition FT-991A —
//     which reports that radio's inventory, and one radio's inventory is not the
//     model's, so expect this entry to stay ASSUMED with better placeholders
//     rather than to retire.
//     (image.go: DefaultImage; options.go: WithDCSChannels)
//
//  9. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than 256 bytes have
//     accumulated without a ';', this fake replies "?;" once and discards bytes
//     up to and including the next ';' before resuming normal framing. NOT a
//     radio claim and NOT liftable by any capture: it is this package's own
//     bounded-input policy, inherited from internal/fakeradio's, recorded here
//     so that a test relying on it knows what it is relying on.
//     (parser.go: reassembler)
//
//  10. AUTOMATIC-INFORMATION SUPPRESSION: THIS FAKE NEVER PUSHES AN UNSOLICITED
//     FRAME, WHATEVER AI IS SET TO. "AI1;" is accepted, stored and read back
//     faithfully, and then nothing follows from it: this radio writes to the
//     port only in reply to a frame that arrived on it, so an AI-on session and
//     an AI-off session are byte-identical on the wire. The assumption is not
//     that this radio is silent — it is that MODELLING IT AS SILENT IS THE
//     HONEST DEFAULT, because NO FT-991A HAS BEEN OBSERVED WITH AI ON. Which
//     frames such a radio volunteers, on what triggers, in what order and with
//     what interleaving against a command in flight are four unknowns, and
//     inventing them would put fabricated wire behaviour underneath every test
//     that ran against this fake — worse than usual here, because an invented
//     push is indistinguishable at the far end of a link from a transport
//     defect. NOTE THAT THIS RADIO'S OWN AVAILABILITY ROW GIVES AI THE "AI"
//     COLUMN "X" (ft991a_layout.txt:130) — that column marks which commands
//     the radio pushes under AI, and AI itself is not one of them; it says
//     nothing about the others, and MC, MR and MT are all "X" there too while
//     twenty-odd commands this fake does not model are "O".
//     DISTINCT FROM THE MANUAL FACT NEXT DOOR: "AI defaults to off at
//     construction" is ft991a_layout.txt:242 and is deliberately NOT in this
//     register. That line says what AI is set to at power-on; it says nothing
//     about what a radio does once AI is ON, which is the whole of this entry.
//     The two must not be read as one absence.
//     WHAT RESTS ON IT: core/transport.Engine's drain-to-quiet discipline is
//     exercised against internal/fakeradio, whose AI-flood behaviour is the
//     FT-710's OWN observed one, and NOT against this fake. By this
//     milestone's plan, no FT-991A test exercises the engine against a
//     talking radio, and none may claim to: every FT-991A exchange in every
//     suite here is one frame in, at most one frame out.
//     STAGE R LIFTS IT WITH: one session on a real FT-991A with AI set to 1 and
//     the port then watched — idle, and while the front panel is operated (VFO
//     turned, mode changed, memory recalled). Whatever that radio pushes, and
//     whatever provokes it, becomes this fake's model; NOTHING MAY BE INVENTED
//     MEANWHILE, and an observation of nothing at all is a result to record
//     rather than a failed capture. RECORD WHICH SERIAL DEVICE IT WAS TAKEN ON:
//     this radio enumerates TWO — a built-in USB to dual UART bridge
//     (ft991a_layout.txt:46-47) and an RS-232C jack gated by menu 028 (558) —
//     so a capture that cannot name its port settles nothing about silence.
//     (parser.go: handleAI and the AI section's note; fakeft991a.go: serve and
//     handleEvent, whose only write is a reply)
//
//  11. THE EX MENU VALUES ARE INVENTED. Every menu item this fake answers reads
//     back n x '0', n being the width transcription B's digits column prints
//     for it. The chart documents each item's VALID RANGE and its option
//     legends and NEVER a shipped default, so there is nothing to source a real
//     one from. The uniformity is the point: a placeholder that is obviously
//     uniform is harder to mistake for evidence than a plausible-looking spread
//     of values, and this matters beyond the test suite, because
//     `rigprog read --settings --fake --model FT-991A` renders these bytes to a
//     user who must not read them as what an FT-991A ships with. It is
//     internal/fakeradio's convention, adopted whole.
//     WHAT IS NOT ASSUMED HERE IS THE WIDTH. Each item's field width is
//     transcribed, not guessed, including the eight-wide one (151 PRESET
//     FREQUENCY) that this radio's alphabet had to be widened for — and the
//     widening is proved from the artefact rather than declared
//     (gen/main_test.go's TestParseB_TheOnlyEightWideRowIs151, against
//     core/cat/ft991a/crosscheck_test.go's widestRowAddr from the other
//     transcription).
//     STAGE R LIFTS IT WITH: an EX read sweep of a factory-condition FT-991A.
//     Expect this entry to stay ASSUMED with better placeholders rather than to
//     retire — one radio's menu is not the model's.
//     (ex.go: exDefaultDigit, expandEXItems)
//
//  12. AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;". A syntactically valid
//     three-digit address the inventory does not carry draws the same
//     unattributed NAK an empty slot does. That covers an address past the
//     chart's last row, the zero address the chart's numbering never reaches,
//     and — the case peculiar to this radio — 087 RADIO ID, which the chart
//     PRINTS but gives no parameter, so that both transcriptions' generators
//     exclude it and neither side of the cross-check carries it (plan decision
//     P18). Membership comes from the chart's own rows via the generated
//     inventory, and NOT from the EX block's printed bound: that block prints
//     "P1 : 001 - 153 (MENU Number)", which is exactly the first and last rows
//     transcribed, so enforcing it as a range would add a second authority over
//     one fact AND would admit 087.
//     THIS IS ASSUMED HERE WHERE THE FT-710'S IS OBSERVED: M8c put two
//     out-of-chart EX addresses to a real FT-710 and both drew "?;"
//     (docs/hardware-notes.md), which is that radio's finding on that radio's
//     six-digit grammar, and is not borrowed. No FT-991A has been asked.
//     STAGE R LIFTS IT WITH: one EX read of an address the chart does not carry
//     — "EX154;" will do, one past the chart's last — and one of "EX087;", with
//     the port watched. An answer rather than "?;" to either would mean the
//     chart under-describes this radio's menu, which would be a finding about
//     the transcriptions as much as about the fake; 087 answering at all would
//     additionally settle what width a row printing no parameter has, which
//     THIS CHART CANNOT SAY (transcription-b.md §2(b) says so in terms).
//     (ex.go: handleEX; options.go: WithEXUnavailable, which reaches this same
//     answer for a known address without inventing a new behaviour)
//
// # What is NOT in this register, and why
//
// Several behaviours a reader might expect to find registered as assumptions
// are MANUAL FACTS for this radio, or belong to a register that already carries
// them. They are listed here, with the line that makes each one a fact, so that
// the absences read as decisions rather than as oversights:
//
//   - THE "?;" REJECTION CONVENTION, AND SILENCE ON AN ACCEPTED SET.
//     internal/fakeft891's register carries both as entries of its own because
//     the FT-891's dialect register has no acknowledgement entry. This radio's
//     does — core/cat/ft991a/doc.go's "THE ACKNOWLEDGEMENT CONVENTIONS", which
//     states exactly this assumption and names its lifting capture (one write
//     session's raw transcript, every byte in both directions) — so this
//     package cites it and does not re-register it.
//   - AI DEFAULTS TO OFF AT CONSTRUCTION. "This parameter is set to '0' (OFF)
//     automatically when the transceiver is turned 'OFF'" (ft991a_layout.txt:
//     242, inside AI's own block at 237-246). New models a freshly-powered
//     radio, and that is this manual's own statement rather than an
//     inheritance. What is NOT covered by this absence is what the fake does
//     once AI is turned ON — AUTOMATIC-INFORMATION SUPPRESSION above, and the
//     two must not be read as one absence.
//   - MT IS READABLE. The availability row gives MT "O O O X"
//     (ft991a_layout.txt:181) and its own detail block prints a Read chart
//     (1018) and a full 41-position Answer chart (1021-1033). The two AGREE,
//     which is why this fake answers an MT read by default with no option to do
//     otherwise, and why there is nothing here answering to
//     internal/fakeft891's register entry "MT READ IS ANSWERED, BY DEFAULT" —
//     that entry exists because that manual contradicts itself.
//   - P11 IS THE SCHEMA BYTE '0', REQUIRED ON A SET AND EMITTED IN EVERY
//     ANSWER. "P11 0: (Fixed)" (1015). This fake follows the legend. That a
//     REAL FT-991A answers '0' is a separate claim and it is the DRIVER
//     register's, not this one's — which is why MemState carries a P11 field at
//     all: so that the radio which refutes it can be played here.
//   - THE SLOT SPAN IS 001-117, AND THE PMS PAIRS ARE NUMBERED. MC's legend
//     prints both the span and its decomposition (913-916) and the other five
//     blocks print the span (966, 999, 1037, 783, 1117).
//     core/cat/ft991a/doc.go says the same thing in its own "WHAT IS
//     DELIBERATELY NOT AN ENTRY" section, and this package inherits the
//     transcription rather than re-deriving it.
//   - THERE IS NO 5 MHz BANK AND NO EMERGENCY CHANNEL. A TRANSCRIBED ABSENCE,
//     established mechanically over the whole extraction
//     (core/cat/ft991a/dialect.go:114-123, :141-149), where the FTdx10's and
//     the FT-710's 501..599 sit on their own ASSUMED registers because their
//     manuals print "5xx" and this one prints nothing at all.
//   - THE MODE LEGEND'S FOURTEEN NAMES, WITH NO HOLE AND NO 'F'. All five
//     printings agree name-for-name and nibble-for-nibble (973-975, 1006-1008,
//     1044-1046, 789-791, 1124-1126). A mode nibble outside 1-9 and A-E is
//     refused because this manual's legend does not print it, not because
//     anything is assumed. (MD's own legend prints the same nibbles with
//     "3: CW-U" and "7: CW-L", 927-929; the memory legend is the one this fake
//     enforces, as core/cat/ft991a does, and the rival spelling changes no
//     byte.)
//   - THE FIVE-STATE P8's MEMBERSHIP. Printed identically on all five blocks
//     that carry the field (795-796, 977-978, 1010-1011, 1048-1049,
//     1128-1129). What IS assumed about it is the radio's SET acceptance of the
//     two DCS members, and that is the DIALECT's entry "THE DCS STATES' SET
//     ACCEPTANCE", cited at validCTCSSByte and not re-registered.
//   - MR HAS NO SET DIRECTION. The command list gives it "X O O X" (178), so a
//     28-byte MR frame in the Answer shape is an unknown frame rather than a
//     write.
//   - THE TAG FIELD'S BYTE RULE. "the parameter digits should be filled using
//     any character except the ASCII control codes (00 to 1Fh) and the
//     terminator (;)" (106-109, printed folio 2). What IS assumed about the tag
//     is the strictness of ENFORCING it on a Set — entry 5 — not the rule.
//   - COMMAND NAMES ARE ACCEPTED IN EITHER CASE. "A command consists of 2
//     alphabetical characters. You may use either lower or upper case
//     characters." (ft991a_layout.txt:113-114, under the "Alphabetical
//     Commands" heading at 112) — the same sentence the FT-891's and the
//     FTdx10's manuals state, printed whole here rather than hyphenated across
//     a column break as the FT-891's is. Mixed case (e.g. "Mt001;") is admitted
//     too, as a CONSEQUENCE of folding each of the two command bytes
//     independently — "either lower or upper" does not itself license mixing,
//     and per-character folding yields it anyway, which is stated rather than
//     left implicit (TestCommandNamesAreAcceptedInEitherCase). FIELD values
//     remain case-sensitive — the mode nibble's hex letters — which the
//     manual's sentence never touches.
package fakeft991a

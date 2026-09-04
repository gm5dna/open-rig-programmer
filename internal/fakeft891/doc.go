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
// EX (MENU) IS DELIBERATELY NOT MODELLED YET — see "What this fake
// deliberately does NOT model" below.
//
// # The hard rule: NOTHING project-internal
//
// fakeft891 MUST NOT import any package of this project — not core/cat, not
// core/cat/ft891, not core/codeplug, not core/spec, and not
// internal/fakeradio or any sibling fake. Standard library only, in every
// non-test file, in this directory AND every directory beneath it (the
// generator internal/fakeft891/gen arrives at this milestone's next task and
// is inside the fence from birth: imports_test.go's scan walks the tree).
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
// channels where the FTdx10's is a hundred. A shared implementation would
// have had to be unpicked from every one of those.
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
//   - COMMAND NAMES ARE UPPER CASE ONLY, and that is registered as an
//     assumption rather than asserted: the FTdx10's manual states the
//     either-case leniency in terms and nothing in this repository cites such
//     a sentence for the FT-891.
//   - No fault injection, and no MW — see the next section.
//
// # What this fake deliberately does NOT model
//
// EX (MENU), FOR NOW. This radio's EX grammar is four-digit
// ("EX P1 P1 P1 P1 ;", seven bytes — ft891_layout.txt:513-522) where every
// registered sibling's is six, and answering it needs this package's own
// independent copy of transcription B and a NEW generator for the FT-891's
// three-column schema. That is the next task of this milestone's plan, and it
// is a task rather than a paragraph because the generator, the transcription
// copy, the staleness test and the core/transport cross-check land together
// or not at all. UNTIL THEN AN EX FRAME OF ANY SHAPE ANSWERS "?;",
// indistinguishably from an unknown command, and that answer is a STUB rather
// than a claim about the radio — TestEX_IsDeliberatelyAbsentUntilItsGenerator
// pins it as one.
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
package fakeft891

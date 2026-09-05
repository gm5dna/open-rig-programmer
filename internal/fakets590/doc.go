// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets590 simulates a Kenwood TS-590S or TS-590SG's PC-control
// behaviour over an in-memory serial connection (Radio.Port()). It is the
// test double the 590 pair's own layers run against: the transport engine,
// core/driver/ts590, the CLI's --fake mode and the GUI's demo mode — the role
// internal/fakeradio plays for the FT-710 and internal/fakeft891 for the
// FT-891.
//
// ONE FAKE, TWO REGISTRY ROWS. The TS-590S and the TS-590SG share one
// PC-command document, one 50-byte memory grid, one MD legend and one
// hard-wired byte set, so one package serves both and New's Row argument says
// which. The row is REQUIRED and has no zero value: the two differ at ID
// ("021: TS-590S", "023: TS-590SG", 590:1114-1116) and at byte 28, whose
// FILTER A/B selection the book calls "always 0" only "in firmware version
// 1.xx of TS-590S" (590:1478). A default row would make every test of the
// other sibling a fixture accident.
//
// # The hard rule: NOTHING project-internal
//
// fakets590 MUST NOT import any package of this project — not core/kw, not
// core/kw/ts590, not core/codeplug, not core/spec, and not internal/fakeft891
// or any sibling fake. Standard library only, in every non-test file, in this
// directory AND every directory beneath it. Every byte offset, field width
// and validation rule below is re-derived from the TS-590S/TS-590SG PC
// Control Command Reference Guide (rev 3) — cited "590:NNNN" throughout, the
// same convention core/kw/doc.go uses, naming where a chart is rather than
// linking to it, because the manual itself is gitignored
// (docs/fixtures-private/manuals/).
//
// This is not a style preference, and the reasoning is internal/fakeradio's
// verbatim: if this fake reused core/kw's codec, a systematic bug in that
// codec — an off-by-one in a field offset, a validation rule subtly wrong —
// would be applied identically on both sides of every "send a command, check
// the reply" test this project runs. The bug would never surface. The fake
// has to be able to DISAGREE with the production codec for a test against it
// to mean anything, and it can only disagree if it was built from the manual
// rather than from the code.
//
// The fence is enforced mechanically and recursively (imports_test.go), from
// birth, ahead of the gen/ subdirectory a later task of this milestone's plan
// brings in.
//
// # A SIBLING of internal/fakeft891, not a refactor of it
//
// This package duplicates a good deal of internal/fakeft891's shape: the
// pipe-and-goroutine Radio, the bounded reassembler, the "?;" convention, the
// per-command handlers, the Image contract, the options. That duplication is
// deliberate and it is not going to be factored into a shared "fake core"
// package. A shared helper would be a project-internal import, which the hard
// rule above forbids in every fake, so the only way to share code would be to
// abandon the property that makes any of them worth having.
//
// And the radios do not in fact agree. Where this fake diverges from the
// Yaesu ones, the divergence is this book's, not a preference:
//
//   - THE "?;" CONVENTION IS PRINTED HERE. The Yaesu fakes inherit it from a
//     sibling's reference and register that they do; this book prints its own
//     error table, with two named causes for "?;" and a note that the message
//     may not appear at all (590:93-113).
//   - THERE ARE TWO MORE ERROR TOKENS. "E;" is a serial-line communication
//     error and "O;" a receive-buffer overrun (590:110-113) — neither is a
//     command outcome, and no Yaesu fake has anything like them.
//   - THE AI LEGEND IS 0/2/4. This book prints "0: AI OFF / 2: AI ON (without
//     backup) / 4: AI ON (with backup)" (590:159-162) with no 1 and no 3;
//     the TS-480's own book prints 0, 1, 2 and 3 with different meanings
//     (480:185-190). Two Kenwood radios do not share this table either, which
//     is why internal/fakets480 will transcribe its own.
//   - THE MEMORY RECORD IS 50 BYTES AND ITS REQUEST CARRIES A P1 SELECTOR.
//     "MR P1 P2 P3 P3 ;" reads either half of a channel (590:1440-1442) where
//     every Yaesu read frame names a slot and nothing else.
//   - THE CHANNEL NUMBER'S HUNDREDS DIGIT MAY BE A SPACE (590:1332-1337).
//
// # What this fake deliberately does NOT model
//
// EX (MENU), IN EITHER DIRECTION. The two siblings print two disjoint menu
// charts in one book, and the fake's inventory is to come from its own copy
// of each chart's independent transcription — the two-source evidence design
// this project uses on the Yaesu side. That work is the NEXT task of this
// milestone's plan; until it lands, an EX frame draws "?;". That is a
// MODELLING GAP, KNOWN-DIVERGENT from the documented grammar, and it is not a
// claim that either radio refuses EX. TestEX_IsNotModelledYet pins the gap's
// shape so that adding EX has to change a test rather than fill a silence.
//
// THE ERASE FORM OF MW. The book describes one: "If you do not specify one
// digit in P16 and execute all the parameters from P4 to P15 set to 0, the
// channels specified by P2 and P3 will be erased." (590:1579-1581). This
// programme builds no erase frame on any radio — a standing rule of this
// repository — so the fake accepts MW at exactly the 50-byte width its chart
// counts (590:1518-1536) and refuses every other width, the erase form
// included. Not a claim that the radio lacks the command.
//
// FAULT INJECTION beyond the book. There is no dropped-reply, garbled-byte,
// spurious-frame or chunked-write option here. Those exercise
// core/transport.Engine, one model-independent implementation already covered
// against internal/fakeradio's fault suite, and nothing is learnt by running
// them past a second dialect. What IS modelled is what THIS BOOK prints: the
// two stream-error tokens, and the transient "?;" suppression its own error
// table flags (590:106-108).
//
// A FRONT PANEL. Nothing here models one, so the selected channel moves only
// by an MC Set and this fake never produces an MC answer naming a channel no
// host asked for.
package fakets590

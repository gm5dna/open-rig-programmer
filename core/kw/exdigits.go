// SPDX-License-Identifier: GPL-3.0-or-later

package kw

// exAnswerFixedBytes is how many bytes of an EX ANSWER are not P5.
//
// The read frame is "E X P1 P1 P1 P2 P2 P3 P4 ;", ten bytes fixed on both
// radios (590:552, 480:410); the Set and the Answer append P5, "String of
// alphanumeric characters for the Menu setting (variable length)"
// (590:555-556) — "A string of characters (Variable length). Normally
// 1-digit for the TS-480. Menu No. 32, 35 and 48 ~ 52 use 2-digit
// parameters" (480:409-411). So an answer is those ten bytes plus P5:
// "EX"(2) + P1(3) + P2(2) + P3(1) + P4(1) + ";"(1) = 10, with P5 inserted
// before the terminator.
const exAnswerFixedBytes = 10

// MaxEXDigits is the largest EXItem.Digits a Kenwood profile may declare:
// THIS PACKAGE'S ceiling, derived from THIS PACKAGE'S own maximum frame.
//
// WHY IT IS NOT internal/extable.MaxDigitsCeiling. That constant is 247 and
// it is CORE/CAT's number, mirroring that package's maxEXDigits, which is
// derived from a Yaesu EX answer's NINE bytes of fixed overhead and pinned
// to it by core/cat/exdigits_ceiling_test.go. A Kenwood profile validated
// against it would be a bound consulted from one place with its datum taken
// from another — the defect shape internal/extable.Profile's own doc
// comment says the type exists to prevent, and the one that appeared four
// times across M9b. Stage 0's task 2 made DigitsCeiling a REQUIRED
// per-profile field for exactly this reason; the three Kenwood stanzas
// carry the value below, and core/kw/exdigits_ceiling_test.go pins them to
// it.
//
// THE STANZAS CARRY A LITERAL, NOT THIS SYMBOL, and that is not sloppiness:
// internal/extable is build-time tooling that renders core/kw source text,
// and importing the package it generates into would cycle the dependency it
// exists to keep one-way. The twin test is what makes the transcription
// safe, and it selects the profiles by ImportPath so a fourth Kenwood
// stanza nobody planned is caught with the three that were.
//
// A wider P5 than this describes an answer frame longer than
// DefaultMaxFrame, which this family's own accumulator would discard as
// contamination — a profile could then declare an item its own transport
// could never read back.
//
// IT IS EXPORTED, WHICH core/cat's EQUIVALENT IS NOT, and the reason is the
// paragraph above: the value has to be transcribed into another package, so
// a reader checking that transcription needs a name to check it against
// rather than a second literal.
const MaxEXDigits = DefaultMaxFrame - exAnswerFixedBytes

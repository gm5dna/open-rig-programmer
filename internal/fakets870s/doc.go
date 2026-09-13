// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakets870s simulates a Kenwood TS-870S's PC-control behaviour over
// an in-memory serial connection (Radio.Port()). It is the test double the
// TS-870S row's own layers will run against, the role internal/fakepipe's
// sibling fakes (internal/fakets480, internal/fakets590) play for their own
// rows.
//
// ONE ROW, AND NO ROW ARGUMENT. The manual describes one PC-control radio and
// prints one identity for it, "The TS-870S number is 015" (Format 16,
// ts870s:8300-8302); there is no sibling document and no second row, so New
// takes no row argument at all — the internal/fakets480 shape, not
// internal/fakets590's required-Row one.
//
// # The hard rule: NOTHING project-internal
//
// fakets870s MUST NOT import any package of this project — not core/kw, not
// core/kw/ts870s, not core/codeplug, not core/spec, and not any sibling fake.
// Standard library only, in every non-test file, in this directory AND every
// directory beneath it. Every byte offset, field width and validation rule
// below is re-derived from the community mirror (rigpix.com) of Kenwood
// document B62-1536-00, the TS-870S's full instruction manual — cited
// "ts870s:NNNN" throughout, naming a line of
// docs/fixtures-private/manuals/ts870s_manual_mirror_layout.txt (gitignored)
// rather than linking to it, the convention core/kw/doc.go uses.
//
// THE QUARANTINE IS FROM THE DRIVER, NOT FROM THE MATRIX. This package was
// authored against docs/superpowers/ts870s-capability-matrix.md (main
// checkout, read-only, a pre-plan artefact) and the manual directly, and
// deliberately NOT against core/kw/ts870s or core/driver/ts870s, which this
// package's author did not read beyond reviews/driver-ts870s.md's own
// `## Verdict` heading. The reasoning is internal/fakets590's own: if this
// fake reused the driver's understanding of the wire, a systematic bug in
// that understanding would sit on both sides of every "send a command, check
// the reply" test this project runs, and never surface. The fake disagreeing
// with the codec is what makes the cross-check mean anything.
//
// The fence is enforced mechanically and recursively (imports_test.go): this
// directory and every one beneath it.
//
// # A DISTINCT GRID, NOT A MEMBER OF THE 590/480 FAMILY
//
// This radio's own document gives its MR/MW frame ONLY 22 BYTES, six live
// parameters (P1, P3-P8) of the shared family's sixteen parameter slots, and
// the seven the family fills are not filler bytes here — THEY ARE ABSENT
// BYTES. P2 (the 590/480 pair's hundreds-digit byte) and P9 (their second
// tone index) do not exist on the wire at all: the Parameter Table prints
// "–" against both, not a digit count, and no diagram cell sits between P1
// and P3 in any of the Read, Set or Answer rows (MR: "M R P1 P3 ;",
// ts870s:9097; MW: "M W P1 P3 P4 P5 P6 P7 P8 ;", ts870s:9139-9163). This
// package shares NO TABLE AND NO LEGEND with either 590/480 fake — every
// offset, width and legend below is transcribed from this book alone.
//
//   - THE CHANNEL NUMBER IS TWO DIGITS AND THERE IS NO BANK BYTE AT ALL. P3 is
//     the whole channel number, "00 ~ 99" (Format 7, ts870s:8267-8268), and
//     there is no P2 byte before it to hold a bank or a hundreds digit.
//   - P1 IS OVERLOADED BY CHANNEL NUMBER, exactly as internal/fakets480's
//     Half is: "0: Receive, 1: Transmit" on channels 00-98 (Format 9,
//     ts870s:8276-8278), and "P1 must be '0' to read the CH 99 Start
//     frequency and '1' to read the End frequency" (ts870s:9105-9106),
//     "...to store a Start frequency and '1' to store an End frequency"
//     (ts870s:9162-9163) on channel 99. One byte, two meanings depending on
//     the channel, same as the TS-480/590 pair's own overload.
//   - THE TONE AXIS HAS ONLY ONE INDEX. P8 is a single two-digit tone number,
//     "01~39" (Format 14, ts870s:8296-8299), used for transmit encode; there
//     is no P9 (receive tone) at all. The tone-mode byte (P7, "SW", Format 1)
//     is a plain "0: OFF, 1: ON" (ts870s:8256) — TWO values, narrower than
//     every sibling in this project's Kenwood lane.
//   - THERE IS NO NAME FIELD ANYWHERE IN THE 22-BYTE FRAME. A case-insensitive
//     search of the whole 9,758-line extraction for "memory name"/"channel
//     name"/"8 character" returns nothing, and the appendix command index has
//     no name-route command. NoTag, TagLen 0 — this package builds no name
//     field, no echo, no charset.
//   - "O;" IS DOCUMENTED HERE TOO, AND MATCHES THE TS-480's WORDING EXACTLY:
//     "Receive data was sent but processing was not completed."
//     (ts870s:8449-8450) against "?;"'s two causes (ts870s:8434-8438) and
//     "E;"'s "A communication error occurred such as an overrun or framing
//     error during a serial data transmission." (ts870s:8445-8447). Both are
//     now SCRIPTABLE (WithStreamError) — see the STREAM ERRORS register entry
//     below, revised once `core/driver/ts870s` wired a live session.
//
// # What this fake deliberately does NOT model
//
// MC (MEMORY CHANNEL SELECTION). The command exists in this book ("Sets or
// reads memory channels.", ts870s:9028) but no `spec.Capabilities` field or
// bank `Field` the matrix grades depends on it — MR and MW both address a
// channel directly by its own P3 digits, never through a prior selection —
// and the driver's own allowed-command set (per this package's dispatch
// brief) does not include it either. Modelling a selector nothing exercises
// would be scaffolding for a test this package has no way to write.
//
// THE EX (MENU) SET. Out of scope for the whole v1.7.0 Kenwood/Yaesu wave
// (spec.md §3): "EXItems empty, MaxEXAddress unset-refused." Nothing here
// parses or answers "EX;".
//
// FAULT INJECTION beyond the book. There is no dropped-reply, garbled-byte,
// spurious-frame or chunked-write option here, for the same reason
// internal/fakets480's doc.go gives: those exercise core/transport.Engine, a
// model-independent implementation, and nothing is learnt by running them
// past a second dialect.
//
// A FRONT PANEL. Nothing here models one; the fake's state moves only by a
// Set this package's own dispatch recognises.
//
// # THE ASSUMED REGISTER
//
// Every place this fake had to decide something the manual does not settle
// is listed here, and each entry appears as an inline comment beside the code
// that implements it, cited BY NAME (renumbering is editorial and must not
// break a citation) — TestASSUMEDRegisterIsComplete holds both halves
// mechanically.
//
//  1. AN ACCEPTED MW PRODUCES NO REPLY. The MW block prints only a Set
//     diagram (ts870s:9139-9163) — no Read, no Answer row — so this fake
//     answers nothing at all to an accepted write, the same silent-success
//     convention internal/fakets480 and internal/fakets590 both use. What a
//     real TS-870S does on the wire after an accepted MW has not been
//     observed by this project.
//
//  2. A WRITE WITH ALL FREQUENCY DIGITS ZERO MARKS THE CHANNEL VACANT, AND
//     THIS RADIO'S OWN BOOK SAYS SO DIRECTLY, UNLIKE THE 480/590 PAIR'S
//     UNPRINTED WRITE SIDE. "The memory channel becomes a vacant channel if
//     all frequency digits are '0'. ... Other parameters are ignored."
//     (ts870s:9155-9163, independently re-read against the two-column trap
//     the matrix warns about — the "Note:" block belongs to MW's own right-
//     hand column, not the MG block sharing its printed lines). This fake
//     therefore stores no record at all for a Set whose eleven frequency
//     digits are all "0": it deletes whatever half was previously stored,
//     and it does NOT validate the mode, lockout, tone-mode or tone-number
//     bytes in that one case, because the book's own sentence says they are
//     ignored. Every other Set still validates every field (entry 3).
//
//  3. SET-DIRECTION FIELD STRICTNESS ON EVERY OTHER WRITE. P1's two values,
//     the two-digit channel number, the eleven-digit frequency, the mode
//     nibble, the lockout flag, the tone-mode flag and the tone number's
//     shape are ASSUMED to be what the radio itself enforces once the
//     all-zero-frequency case above does not apply: the book prints the
//     legends and the vacant-channel exception; it never states what a Set
//     leaving one of the other fields out of its own legend does. Nothing
//     here normalises: a byte outside its legend is refused, never quietly
//     corrected.
//
//  4. THE TONE NUMBER IS STORED, NOT RANGE-CHECKED AGAINST THE PRINTED CHART.
//     Format 14 prints "01~39" (ts870s:8296-8299) but nothing about what a
//     Set carrying "00" or "40"-"99" does to a memory frame specifically;
//     only the field's two-digit SHAPE is enforced here, the same posture
//     internal/fakets480 takes for its own tone indices, so that a test can
//     drive the codec's own range refusal against a real fake rather than
//     against a fake that already refused for it.
//
//  5. AN UNWRITTEN OR VACANT CHANNEL'S EITHER HALF ANSWERS THE ZERO RECORD.
//     THE READ SIDE IS DOCUMENTARY, NOT ASSUMED: "For a vacant channel, the
//     Answer command sends '0' for all parameters except the memory channel
//     number." (ts870s:9101-9104). What IS assumed is applying that one
//     sentence — which does not itself distinguish P1's two halves — to the
//     TX/End half (P1=1) as well as the RX/Start half it most naturally
//     reads as being about; no radio has confirmed the TX/End half answers
//     the same shape rather than refusing outright.
//
//  6. STREAM ERRORS ARE SCRIPTABLE, NOW THAT A LIVE SESSION EXISTS TO
//     INTERRUPT. Revised 13/09/2026: this entry originally refused to build
//     a `WithStreamError` option because `reviews/driver-ts870s.md`'s
//     `## Verdict` (the only part of that report read at the time) showed
//     `core/driver/ts870s.Open` always refusing without ever building a
//     session. The coordinator's follow-up instruction pointed at that
//     report's own `## Follow-up` section and at `reviews/lift-K.md`'s
//     `## Follow-up`: lift K `e7515d0` added `Layout870.Book()`,
//     `NewFramingFor870` and real Book570/Book870S stream-error citations,
//     and `core/driver/ts870s`'s own follow-up (`e97d307`) wired `Open`
//     to a genuine `transport.Engine` over it. There is now a live stream on
//     the driver side for a scripted "E;"/"O;" to interrupt, so
//     `WithStreamError` (options.go) scripts either token at one exchange,
//     the same shape internal/fakets480's and internal/fakets590's own
//     options take, cited to this book's own error table
//     (ts870s:8434-8450).
//
//  7. THE FRAME ACCUMULATOR'S CAP AND RESYNC. The 256-byte bound, the single
//     "?;" per overflow and the discard-to-next-';' resync are this
//     package's own bounded-input policy, copied from internal/fakets480's
//     identical, independently-justified choice. No Kenwood book prints a
//     buffer size.
//
//  8. AI NEVER PUSHES ANYTHING UNSOLICITED. Format 32's own legend ties
//     AI=1/2 to whether the transceiver pushes Answer commands on its own
//     (ts870s:8213-8224 area, "AI NUMBER"), and this fake does not model that
//     push in any AI state — the same gap internal/fakets480's doc.go
//     records for its own AI, and for the same reason: no TS-870S has been
//     observed by this project, and modelling silence is the honest default.
//
//  9. THE DEFAULT IMAGE'S RECORD COMPOSITION. Every byte of every shipped
//     record in image.go is the printed example frequency (Format 4,
//     ts870s:8267-8269, "00014230000") or a printed legend value; no MR or MW
//     frame is printed as a literal anywhere in this book, so the CROSS-FIELD
//     COMBINATION has never been printed or observed, and no byte here is
//     invented, derived from another model, or padded to make a test pass.
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF, AND ITS TRANSIENT-SUPPRESSION NOTE. Both are
// printed in this book's own error table (ts870s:8434-8442) — transcription,
// not assumption — and WithTransientNAKSuppressed plays the note rather than
// assuming it away, the same shape internal/fakets480's option takes.
//
// THE COMMAND-NAME CASE FOLD. "A command is composed of 2 alphabetical
// characters" (ts870s:8195) and "A command may consist of either lower or
// upper case alphabetical characters." (ts870s:8214) are manual facts.
package fakets870s

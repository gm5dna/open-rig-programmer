// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeftx1 is an independent, stdlib-only FTX-1 CAT simulator: the
// test double a future core/driver/ftx1, the transport engine, the CLI's
// --fake mode and the GUI's demo mode run against, the role
// internal/fakeftdx3000 plays for the FTdx3000 and internal/fakeradio for
// the FT-710.
//
// SINGLE ROW, BARE New, CATID "0840". The FTX-1 manual gives ID exactly
// one fixed answer, "0840 (Fixed)" (FTX-1 spec.md §1, citing
// ftx1_layout.txt:1305), shared unmodified by both the "field" and
// "optima" bodies. There is no CAT-visible way to tell them apart — the
// only body-distinguishing text in the manual is the AC command's
// internal-tuner legend, a CAPABILITY probe, not an identity byte (spec.md
// §1) — so New takes no model/body option at all, and CATID() always
// answers "0840". BODY IDENTITY IS NEVER SURFACED, anywhere in this
// package: never inferred, never stored, never an Option.
//
// FIVE-BYTE SLOT SPACE. Every FTX-1 slot form is 5 bytes wide, not the
// 3-byte forms every other registered Yaesu fake in this fleet uses
// (spec.md §4): "00001"-"00999" (memory), "P-01L"-"P-50U" (PMS, a dash
// and a two-digit pair), "50001"-"50020" (5 MHz band), "EMGCH" (the fixed
// literal emergency channel). MR, MT and MW's own domains (spec.md §3.1,
// §3.3) all print this same four-class list; MW's own P1 legend omits the
// 5 MHz/EMGCH lines (see register entry 2 below).
//
// MR/MW SHARE ONE 27-BYTE FIELD BLOCK, MT IS SEPARATE. MR (Read/Answer
// only) and MW (Set only) both carry the SAME P1-P10 block spec.md §3.1
// lays out byte-for-byte: 27 bytes after the two-byte opcode, so a Read
// (5-byte addr) is an 8-byte frame and an Answer/Set is a 30-byte one. MT
// (Set/Read/Answer all real) carries the address and a fixed 12-byte tag
// only — no P2-P10 memory fields at all, and NO display byte (spec.md
// §3.3, f1-report.md item 3, "MTFormShortNoDisplay"): a Read is 8 bytes
// ("MT" + addr(5) + ";"), a Set or its Answer is 20 bytes ("MT" + addr(5)
// + tag(12) + ";").
//
// MC IS NOT MODELLED AT ALL. FTX-1's own MC carries a leading MAIN/SUB
// port byte before its slot — a frame shape nothing in this package's
// "cmd + body + ;" dispatch can build without a dedicated arm (spec.md
// §3.4, f1-report.md item 4, "MCSelectsUnsupported"). Rather than
// mis-parsing a coincidentally-shaped MC frame, this fake gives MC no
// handler at all: it falls through the unknown-command default and
// answers "?;", exactly the plan's own instruction ("MC refuses"). VM,
// GT and EX are UNMODELLED for the same reason the plan gives them (out
// of scope this milestone) and fall through the same default.
//
// # The hard rule: NOTHING project-internal
//
// fakeftx1 MUST NOT import any package of this project — not core/cat,
// not core/cat/ftx1 (which does not exist yet — F2/core/driver/ftx1 is
// being written concurrently, elsewhere), not core/codeplug, not
// core/spec, and not internal/fakeftdx3000 or any sibling fake — in any
// non-test file, in this directory and every directory beneath it
// (imports_test.go). Every field below is re-derived directly from
// `.superpowers/sdd/2026-09-18-v1100-ftx1/reviews/spec.md` (the FTX-1
// manual's own reading, paper-only, no code) and from f1-report.md (the
// core/cat seam this package's WIRE SHAPES happen to agree with, never
// consulted for behaviour) — precisely so that a systematic error in a
// future core/driver/ftx1's own reading of the manual cannot sit on both
// sides of a cross-check run against a fake shaped like this one.
//
// # A SIBLING of internal/fakeftdx3000, not a refactor of it
//
// This package copies that sibling's SHAPE: the pipe-and-goroutine Radio,
// the bounded reassembler, the "?;" convention, the per-command handlers,
// the Option pattern, the recursive import fence. Where FTX-1's OWN wire
// disagrees — the 5-byte slot space, the dash-token PMS form, the 5 MHz
// and EMGCH banks, MT's separate no-display shape, MC's total absence,
// the six-value tone domain, the 18-member mode table — it is this
// milestone's spec, not a preference.
//
// # No FTX-1 has ever been asked anything by this project, and none is
// available to it
//
// Every value below not directly printed in the manual is ASSUMED, listed
// in the register below, cited by name at the code that implements it
// (register_test.go's TestASSUMEDRegisterIsComplete holds both halves
// mechanically).
package fakeftx1

// THE ASSUMED REGISTER
//
//  1. EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS. A slot this fake holds no
//     memory-block state for answers "?;" to an MR read; a slot with no
//     tag state answers "?;" to an MT read. Neither the spec nor the
//     manual states what an unprogrammed channel's read returns; "?;" is
//     this fleet's one unattributed NAK, played identically by every
//     registered fake, including internal/fakeftdx3000. The "00000"
//     none-form ("VFO or MT or QMB", spec.md §3.1) is not specially
//     cased: it falls out of the ordinary numeric-domain check (0 is
//     below memory's floor of 1) as just another out-of-domain address,
//     which is the honest answer for a fake that models memory-bank
//     storage only, never live VFO state.
//     (parser.go: classifySlot, handleMR, handleMT)
//
//  2. MW FOLLOWS MR'S DOMAIN, NOT MW'S OWN NARROWER PRINTED P1. MW's own
//     P1 legend (spec.md §3.2) omits the 5 MHz and EMGCH lines that MR's,
//     MT's, IF's and OI's all print — a gap spec.md §10 calls "moot"
//     because core/cat's project-wide write policy already excludes
//     those banks from every dialect's MW regardless. THIS FAKE HAS NO
//     SUCH POLICY TO INHERIT (it does not import core/cat) and is
//     modelling the WIRE, not the driver's own caution — so it accepts
//     the plan's explicit instruction: MW's domain is MR's domain, the
//     full four-class list, 5 MHz and EMGCH included. A future driver
//     remains free to never send a 5 MHz/EMGCH MW at all; this fake would
//     not refuse one if it did.
//     (parser.go: classifySlot, handleMW)
//
//  3. KIND (P7) AND TONE (P8) ARE STORED VERBATIM, NOT HARDWARE-CONFIRMED.
//     No FTX-1 MW write has ever been probed (spec.md §12 item 4, §11):
//     unlike the FT-710's own MW, which HW-CONFIRMED requires a single
//     fixed Kind value regardless of bank, nothing here says FTX-1's
//     radio is that strict or that lenient. This fake takes the manual's
//     printed grammar at face value on Set — any of P7's six legend
//     bytes ('0'-'5') and any of P8's six tone bytes ('0'-'5') validate
//     and are stored exactly as sent, then read back unmodified. No
//     bank-to-kind inference is made.
//     (parser.go: validKindByte, validToneByte, parseMemoryBlock)
//
//  4. P9 MUST BE THE LITERAL "00" ON EVERY SET. spec.md §3.1's own table
//     prints P9 as `"00" (Fixed)` on both MR's answer and (by the shared
//     block) MW's set — never a live value, unlike FTdx3000's asymmetric
//     P9. A Set whose P9 bytes are not exactly "00" is refused outright,
//     never coerced.
//     (parser.go: p9Fixed, parseMemoryBlock)
//
//  5. MW AND MT MUTATE INDEPENDENT FIELDS. MW's 27-byte block carries no
//     tag byte at all (spec.md §3.1), and MT's 12-byte tag frame carries
//     no memory field at all (spec.md §3.3) — so this fake stores them in
//     two separate maps (Radio.slots, Radio.tags) rather than one
//     combined record. An MW Set never touches a slot's stored tag; an MT
//     Set never touches a slot's stored memory fields. Neither command
//     can be observed clobbering the other's data.
//     (fakeftx1.go: Radio.slots, Radio.tags; parser.go: handleMW, handleMT)
//
//  6. A SET CREATES AN ABSENT SLOT. Neither MW's nor MT's own block names
//     a precondition that the target slot already hold something (spec.md
//     §3.1-§3.3) — a Set that refused to create a new entry would need a
//     second, unprinted rule to do so, so both commands write through
//     unconditionally, memory and tag maps alike.
//     (parser.go: handleMW, handleMT)
//
//  7. MT'S SET ANSWERS WITH AN ECHO; MW'S SET IS FIRE-AND-FORGET. The CAT
//     Control Command List's own Set/Read/Ans/AI quartet (spec.md §9,
//     quoted verbatim) gives MT's Set column an "O" for Answer and MW's a
//     "X" — MT answers every accepted Set with the same 20-byte frame it
//     received (address plus the stored 12-byte tag verbatim); MW answers
//     an accepted Set with silence, the same fire-and-forget convention
//     every other registered fake's memory-write command plays.
//     (parser.go: handleMT, handleMW)
//
//  8. TAG CHARSET AND INJECTION SAFETY. The tag field is validated as
//     printable ASCII 0x20-0x7E, EXCLUDING ';' (the frame terminator —
//     accepting it would make command injection possible), checked over
//     exactly 12 raw bytes (spec.md §8's "up to 12 characters (ASCII)" is
//     a display-side ceiling; the wire field itself is the fixed 12-byte
//     TagFill-padded form f1-report.md item 3 describes, so this fake
//     requires the Set frame's tag field to already be exactly 12 bytes).
//     The exact hardware-accepted charset beyond injection-safety is
//     otherwise ASSUMED, mirroring internal/fakeradio's own MT tag
//     register entry.
//     (parser.go: validTag)
//
//  9. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than
//     maxAccumulatorBytes have accumulated without a ';', this fake
//     replies "?;" once and discards bytes up to and including the next
//     ';' before resuming normal framing. NOT a radio claim: the manual
//     prints no buffer size, inherited in shape from every sibling fake's
//     identical policy.
//     (parser.go: reassembler)
//
//  10. AUTOMATIC-INFORMATION SUPPRESSION. spec.md §9 quotes the FTX-1
//     command table's own AI column: MR, MT, MW and MC are ALL "X" — none
//     is ever pushed unsolicited, whatever AI is set to. This fake goes
//     further than merely agreeing with that column: it has no code path
//     capable of originating a frame at all (Radio.serve's loop only ever
//     replies to what arrives on the port), so AI1 changes nothing about
//     this fake's own behaviour — modelling silence, not claiming the
//     real radio is silent.
//     (parser.go: handleAI; fakeftx1.go: Radio.serve, Radio.handleEvent)
//
//  11. THE DEFAULT IMAGE'S CONTENT IS INVENTED. No MR, MT or MW frame is
//     printed as a literal anywhere in the manual, so every byte of every
//     seeded slot below is a printed legend value assembled into a
//     plausible record, never an observed one — including which bank
//     gets which Kind/Tone byte (register entry 3).
//     (image.go: DefaultImage)
//
//  12. BODY IDENTITY IS NEVER SURFACED. No CAT mechanism distinguishes
//     "FTX-1 field" from "FTX-1 optima" (spec.md §1, §12 item 1); this
//     package carries no body field, no body option, and CATID() answers
//     "0840" unconditionally.
//     (fakeftx1.go: New, CATID)
//
// # What is NOT in this register, and why
//
// THE "?;" CONVENTION ITSELF, AND SILENCE ON AN ACCEPTED MW SET. Every
// registered Yaesu fake in this fleet already plays exactly this
// convention — inherited in shape, not re-assumed per package.
//
// COMMAND NAMES ARE ACCEPTED IN EITHER CASE. Every sibling fake's own
// manual states this for its own radio; FTX-1's command table (spec.md
// §9) is read the same way as every other Yaesu CAT manual this fleet has
// already modelled — inherited, not re-derived from a per-radio citation.
//
// THE FIRMWARE FLOOR (V1.08). spec.md §2: the wire carries no version
// byte MR/MW/MT/MC could fail on; a pre-floor radio simply never answers
// CAT at all. Nothing for this fake to model — it always answers, exactly
// as a floor-or-later real radio would.
const writeTrialsComplete = false

// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeftdx9000 simulates a Yaesu FTdx9000's CAT behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double the
// FTdx9000's own layers run against: the transport engine,
// core/driver/ftdx9000, the CLI's --fake mode and the GUI's demo mode — the
// role internal/fakeft991a plays for the FT-991A and internal/fakeft891 for
// the FT-891.
//
// ONE ROW, NO OPTION NEEDED TO NAME IT. Unlike internal/fakeft991a or
// internal/fakeft891, this radio has no sibling model sharing its package:
// "FTDX9000D"/"FTDX9000Contest"/"FTDX9000MP" are three CAT-ID answers for one
// physical command set (matrix §1.2), and "FT-9000" is a badge alias, never a
// second row (matrix §1.1). New takes no model argument.
//
// The radio it speaks is a SPLIT-FORM one: MR (Read/Answer only, no Set) and
// MW (Set only, no Read, no Answer) carry the same 24-byte field block under
// different two-letter prefixes, and there is NO MT command combining them —
// this manual's Control Command List prints no MT row at all (checked
// mechanically: no "^MT" anywhere in the extraction). There is also NO TAG OR
// NAME FIELD anywhere in the record or the wider command table (matrix §0/§1.6,
// NoTag true) — every byte of the 27-byte frame is accounted for in §2 below
// with nothing left for a name. MC (recall/current-channel) and ID (CAT-ID
// probe) are documented and modelled; AI (automatic information) is documented
// and modelled, accepted and read back, never pushed unsolicited (register
// entry 8).
//
// # The hard rule: NOTHING project-internal
//
// fakeftdx9000 MUST NOT import any package of this project — not core/cat, not
// core/driver/ftdx9000, not core/codeplug, not core/spec, and not any sibling
// fake. Standard library only, in every non-test file, in this directory and
// every directory beneath it (imports_test.go, copied from
// internal/fakeft991a's recursive scan).
//
// Every byte offset, field width and validation rule below is re-derived from
// this project's own capability matrix
// (docs/superpowers/ftdx9000-capability-matrix.md) and, through it, the FTdx9000
// CAT Operation Reference Book — cited "ftdx9000_layout.txt:NNNN" as the matrix
// itself cites, since the manual is gitignored
// (docs/fixtures-private/manuals/) — and NOT from core/driver/ftdx9000, which
// this package is FORBIDDEN to read (the milestone's quarantine: this fake is
// authored independently of the driver's own understanding of the wire, so
// that a systematic bug in that understanding cannot sit on both sides of every
// cross-check test and go unnoticed — internal/fakets590's "THE HARD RULE",
// restated here for the same reason).
//
// # What this fake deliberately does NOT model
//
// AN MT COMMAND. This radio has none (checked mechanically over the full
// extraction); there is nothing to model and no gap to record.
//
// A NAME/TAG FIELD OF ANY KIND. NoTag is a MANUAL FACT for this radio (matrix
// §0/§1.6), not a modelling gap: there is no byte anywhere in this record for a
// channel name, so there is no field to store, echo or charset-check.
//
// SATELLITE MEMORY, EX (MENU) READS, AND ANY SETTINGS SURFACE. Out of this
// milestone's scope for this package (spec §1, open question 1 is ts2000's
// alone, not this one's, but no menu/EX command is documented in this radio's
// command list summary against a driver need this fake has been told about);
// added when a task actually asks for one, per this project's own YAGNI
// convention.
//
// FAULTS AND TIMING. Same reasoning as every sibling fake: those exercise
// core/transport.Engine, a model-independent implementation already covered by
// internal/fakeradio's fault suite. WithLatency is kept — it is not a fault,
// it is the knob Close's promptness is proven against.
//
// # The ASSUMED register
//
// NO FTDX9000 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project (matrix's
// own framing, repeated here). Every place this fake had to decide what the
// radio DOES at the protocol's edges — where the matrix records evidence but
// not behaviour — is listed here, once, cited by NAME at the code that depends
// on it (register_test.go's TestASSUMEDRegisterIsComplete enforces the
// discipline mechanically). This package carries no separate dialect register
// to cite alongside its own: core/driver/ftdx9000 is the one place this
// radio's dialect facts live, and this package is forbidden to read it, so
// every entry below is this fake's own.
//
//  1. THE "?;" REJECTION CONVENTION, AND SILENCE ON AN ACCEPTED SET. Every
//     refusal answers "?;" and nothing else; every accepted Set (MW, an MC-set,
//     an AI-set) is silent. This manual prints NO ACK/NAK vocabulary anywhere
//     (checked mechanically: no "?;", "error" or "not accepted" text in the
//     full extraction) — unlike, say, a Kenwood PC-control guide that spells
//     one out — so this is the fleet-wide convention every sibling Yaesu fake
//     already carries, not a citation this radio's own manual makes. STAGE R
//     LIFTS IT WITH: one write session's raw transcript, every byte in both
//     directions, on a real unit. (parser.go: rejection, handleEvent)
//
//  2. EMPTY-SLOT ANSWERS "?;". A slot this fake holds no state for answers
//     "?;" to an MR read and to an MC-set; MC's read-current and MR's read
//     path never fabricate a record. Same reasoning as every sibling fake's
//     "EMPTY-SLOT ANSWERS" entry; no FTdx9000 has been asked. (parser.go:
//     handleMR, handleMC)
//
//  3. PMS SLOTS ANSWER P7 '1'. MR's own Read-direction legend has exactly two
//     members, "0: VFO 1: Memory" (ftdx9000_layout.txt:1013), and does not say
//     which a PMS band-edge (100-117) answers with. '1' is the only member
//     that is not plainly false — a PMS slot is not a VFO — matching every
//     sibling fake's own reasoning for the identical gap. (parser.go:
//     kindMemory)
//
//  4. THE CLARIFIER IS STORED, BOTH FLAGS, AND ROUND-TRIPS BYTE-FAITHFULLY.
//     P3's sign and magnitude and BOTH P4 (RX) and P5 (TX) flags are stored
//     exactly as an MW Set carried them, and the next MR read answers those
//     same bytes. A DELIBERATE NON-BORROWING of the FT-710's HW-confirmed
//     zeroing (internal/fakeradio, an FT-710-specific finding): no FTdx9000
//     has been asked, so storing what was sent is the honest default.
//     (parser.go: parseFieldBlock; state.go: MemState.ClarSign/ClarMag/RXClar/TXClar)
//
//  5. SET-DIRECTION FIELD STRICTNESS. Every vocabulary the matrix's §2 table
//     prints is enforced at the wire on an MW Set, and a violation draws "?;"
//     with no state change: an 8-digit frequency, a '+'/'-' clarifier sign, a
//     4-digit magnitude, a '0'/'1' RX or TX clarifier flag, a mode byte outside
//     the printed twelve ('1'-'9','A'-'C'), a P7 that is not the Set chart's
//     fixed '0', a P8 outside the printed three ('0'-'2'), a P9 that is not two
//     ASCII digits '00'-'49' (matrix §1.10's live tone-index domain), or a P10
//     outside '0'-'2'. Whether a real FTdx9000 rejects such a frame — rather
//     than clamping, ignoring the field, or storing it verbatim — is
//     unobserved for every one of them. (parser.go: the validators,
//     parseFieldBlock, handleMW)
//
//  6. A SET DOES NOT MOVE THE SELECTED CHANNEL. Only an MC-set changes what
//     "MC;" answers; an MW never does. Same reasoning as every sibling fake:
//     inventing a side effect for a radio nobody has written to would be a
//     borrowed fact this milestone refuses. (parser.go: handleMW, handleMC)
//
//  7. THE DEFAULT IMAGE'S CONTENT IS INVENTED. Placeholder frequencies, modes
//     and PMS pairs, carrying no claim about what an FTdx9000 ships with — see
//     image.go's DefaultImage. Same convention as every sibling fake's
//     equivalent entry.
//
//  8. AUTOMATIC-INFORMATION SUPPRESSION. AI is accepted, stored and read back
//     faithfully (this radio's own manual fact: AI defaults to OFF at power-on,
//     ftdx9000_layout.txt:210 — NOT itself an entry here, only what happens
//     once AI is ON is), but this fake never originates an unsolicited frame
//     whatever AI holds: this radio's manual documents no push behaviour for
//     any command (its own AI column marks which commands push under AI, and
//     nothing here models the ones marked so), and no FTdx9000 has been
//     observed with AI on. (parser.go: handleAI; fakeftdx9000.go: serve,
//     whose only write is a reply)
//
//  9. THE FRAME ACCUMULATOR'S CAP AND RESYNC. Once more than 256 bytes have
//     accumulated without a ';', this fake replies "?;" once and discards up to
//     and including the next ';'. This package's own bounded-input policy, not
//     a radio claim, inherited from every sibling fake's identical one.
//     (parser.go: reassembler)
//
//  10. THE DEFAULT CAT-ID ANSWER, AND ITS OPTION. "ID;" answers one of the
//     three P1 values this radio's own ID legend prints — 0101 (FTDX9000D),
//     0102 (FTDX9000Contest), 0103 (FTDX9000MP) — and the matrix leaves open
//     which a single-row driver probe should expect (matrix §1.2, "this
//     matrix does not resolve which CATID value... the driver's ID-probe
//     should accept"). This fake defaults to 0101, the first-listed and
//     plainest variant name, and WithCATID overrides it to either of the other
//     two so a cross-check can exercise all three the driver's own handshake
//     is said to accept. (options.go: WithCATID; parser.go: handleID)
package fakeftdx9000

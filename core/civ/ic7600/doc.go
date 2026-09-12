// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7600 holds the Icom IC-7600's CI-V dialect: the memory record's
// geometry, its three value enums, its name charset and the civ.Profile
// that binds them. It is DATA ONLY - no driver, no fake, no registration,
// no session, no wire. The package cannot register itself with the
// application: SupportedModels derives solely from internal/wiring's
// driver table.
//
// # Provenance
//
// Everything here comes from the IC-7600's own Instruction Manual (no
// separate CI-V Reference Guide is published for this model -
// docs/fixtures-private/manuals/ic7600_fullmanual_4.pdf, gitignored, so
// page references below are citations rather than links), read for this
// package via:
//
//   - the A4 capability matrix
//     (docs/superpowers/icom-matrices/ic7600-capability-matrix.md rev 1),
//     self-reviewed against a direct 300 dpi render of PDF pp.176-178 - the
//     memory-name character tables and the 1A 00 record diagram itself,
//     the load-bearing byte geometry - and against `pdftotext -layout` for
//     every prose command-table citation;
//   - the Tier 7 S1 evidence leg's own transcription
//     (.superpowers/sdd/2026-09-12-tier7-s1-icom/evidence/
//     ic7600-transcription.csv, reproduced at testdata/
//     ic7600-transcription-b.csv), which reached a REGISTRABLE ruling from
//     the same diagram independently, before the matrix existed.
//
// THIS IS A SINGLE-MATRIX, TWO-SOURCE PROVENANCE, NOT THE IC-7610 EXEMPLAR's
// FOUR-LEG BLIND QUARANTINE. Unlike core/civ/ic7610 (four independent
// agents, each never opening this repository or seeing another's work),
// this package's geometry rests on the matrix's own render-verified
// derivation (matrix S0, S3.11) cross-checked against the S1 leg's
// independently-produced CSV - two readings, not four, and the second was
// not blind to the first's existence (the matrix cites and supersedes it).
// crosscheck_test.go, geometry_test.go and golden_test.go bind those two
// readings to this package's profile; freeze_test.go hashes every
// testdata artefact and no test in this package may modify one.
//
// NO IC-7600 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project. Every
// statement in this package is a reading of a manual, and the register at
// the foot of this file is the list of the places where it is not even
// that.
//
// # Prior art: the IC-7610 package, and where it transfers
//
// spec.md S1's driver-shape note for this model is "copy ic7610, narrow
// two enums, change address." THE TWO ENUMS DID NOT NEED NARROWING: the
// IC-7610 package's own modeEnum and toneModeEnum already exclude WFM, DV
// and DTCS (core/civ/ic7610/profile.go), so this radio's "-WFM,-DV,-DTCS"
// delta from spec.md S1 lands on sets that were already narrow - modeEnum
// and toneModeEnum below are LITERAL, UNMODIFIED copies. What DID need a
// change, beyond Model/CATID/address: byte (3)'s treatment. The IC-7610's
// own doc.go carries TIER RULING E6 for byte (3)'s low-nibble/whole-byte
// select-scan-group marker and byte (11)'s high-nibble data mode, both
// UNMAPPED - directly relevant prior art, cited rather than re-derived.
// SelectByteOffset's comment in profile.go records the ONE genuine
// divergence a driver author needs: this radio's own page draws byte (3)
// as an undivided whole-byte enum, not the IC-7610's two-nibble split with
// a stated "Fixed" high nibble, so there is no textual hook here for a
// nibble-level refusal check - the check in core/driver/ic7600/write.go
// compares the whole byte instead.
//
// # The three lengths, and the offset rule worked through
//
// Spec Erratum 1 requires a per-radio package to pin BOTH length
// conventions with the address width named:
//
//	RecordOnlyLength  25   the 1A 00 data block EXCLUDING the two channel
//	                       selector bytes (1),(2) - what civ.Profile carries,
//	                       and what BuildMemorySet's <record> denotes
//	DataAreaLength    27   the 1A 00 data block INCLUDING them - the matrix's
//	                       own seven-term addition (S3.11), whose sum equals
//	                       the last printed index, (27)
//	AddressBytes       2   <ch-hi> <ch-lo>, civ.AddressFormFlat
//
// A 1A 00 set frame is therefore 34 bytes (6 + 2 + 25 + 1), a 1A 00 read
// frame 9 (6 + 2 + 1) and a 19 00 read frame 7 (6 + 1).
//
// civ.FieldSpan.Offset is 0-BASED FROM THE START OF THE RECORD, and the
// record begins at printed index (3) because (1),(2) are the address and
// lie outside it:
//
//	record byte      = printed index - 2     (1-based within the record)
//	FieldSpan.Offset = printed index - 3     (0-based)
//
// THIS MATRIX'S OWN INDEPENDENT DERIVATION AGREES WITH SPEC.MD S1 EXACTLY:
// 25 bytes, every field at the same position and width as the IC-7610 -
// "no offset differs from spec.md S1" (matrix S3.11, S5). The widths sum
// to 1+5+1+1+1+3+3+10 = 25. The accepted record-length set is {25}, a
// single-length profile under civ.DiscriminatorSingleLength. THIS PACKAGE
// MAKES NO CROSS-MODEL DISTINCTNESS CLAIM (matrix S3.12(iii)): whether 25
// tells this radio apart from a sibling is a tier-level check, not one
// made here.
//
// # The two channel-selector bytes are the ADDRESS, by tier convention
//
// PDF p.178 (folio 169) draws (1),(2) INSIDE the 1A 00 data block, as the
// strip's own leftmost brace - on the page they are record content, not a
// separate address field. The TIER's convention (spec Erratum 1)
// nevertheless puts them outside the record and inside
// civ.ChannelAddress, exactly as the IC-7610 matrix's own S3.11 records
// for itself (matrix S3.11's own scope note).
//
// # Ruling E6, applied: the two regions this record deliberately does not
// # map
//
// Two of this record's regions carry FOUR-VALUED radio fields whose
// neutral homes are BOOLEAN:
//
//   - byte 0 (printed (3)) is a select-scan GROUP marker, printed
//     00 OFF / 01 star1 / 02 star2 / 03 star3 (matrix S2 row 9, S3.16
//     ADDED-1, corroborated by command 0E's B0-B2 sub-commands, PDF p.169
//     folio 160). Its neutral home, codeplug.ChannelData.ScanSkip, is a
//     BoolField. It is not a skip flag at all: a non-zero byte puts the
//     channel into one of three named SELECT groups a select-memory scan
//     can be pointed at.
//   - byte 8's HIGH nibble (printed (11) left nibble) is a four-valued
//     data mode, printed 0 OFF / 1 DATA 1 / 2 DATA 2 / 3 DATA 3 (matrix S2
//     row 20). Its neutral home, codeplug.ChannelData.DataMode, is also a
//     BoolField. NO DIVERGENCE FROM THE IC-7610 here (matrix S2 row 20):
//     both radios print (11) the same way, a single-byte two-nibble
//     sub-diagram with two independent, non-crossing leader labels.
//
// A 4->2 collapse would rewrite a user's SELECT group or data mode on
// every write-back while readback verification compared EQUAL. RULING E6
// (core/civ/ic7610/doc.go) settles this without a local choice: a driver
// may write a slot ONLY when its unmapped regions equal the profile's
// Fixed template; anything else is REFUSED with the reason named, never
// rewritten. Both regions are UNMAPPED in the layout, the profile carries
// an explicit 25-byte all-zero Fixed template, and SelectByteOffset and
// DataModeNibbleOffset are exported so the driver's refusal check names
// them.
//
// THE ONE MECHANICAL DIFFERENCE FROM THE IC-7610 (profile.go's
// SelectByteOffset comment, in full): the IC-7610's own page states a
// "Fixed" high-nibble constraint for byte (3), which is what lets its
// driver refuse a write whose high nibble is non-zero as a violation of
// something the page actually asserts. THIS RADIO'S PAGE ASSERTS NO SUCH
// CONSTRAINT - byte (3) is drawn as one undivided enum cell. The E6
// mechanism still applies (the byte's OFF value is 0x00 and the Fixed
// template is zero, so the comparison is identical whether made as one
// byte or two nibbles), but this package and its driver name it as a
// WHOLE-BYTE comparison rather than splitting it into a "Fixed high
// nibble" claim this document does not make.
//
// THE COSTS, AS E6 REQUIRES: a channel in a SELECT group or whose data
// mode is DATA 1/2/3 CANNOT BE WRITTEN BY THIS PROGRAMME AT ALL, never
// downgraded, never cleared; neither field can be read back, exposed or
// edited; every eligible write costs one extra read exchange, the E6
// comparison.
//
// # No clear builder, and no printed contradiction to reconcile
//
// PDF p.178 (folio 169), under (3), prints: "To program the blank
// channel, enter 'FF' to (3) after the memory channel number ((1) and
// (2)). This completes the memory channel programming." - the frame
// FE FE 7A E0 1A 00 <ch-hi> <ch-lo> FF FD. Command 0B, "Memory clear"
// (PDF p.169, folio 160), is a second, whole-command form, clearing the
// currently-selected channel.
//
// UNLIKE THE IC-7610, THIS RADIO'S OWN PAGE PRINTS NO CONTRADICTION HERE
// (matrix S3.8(b), S3.13, S3.15(c)): its (3) sub-diagram states no
// "Fixed" constraint for the byte's would-be high nibble to disagree with
// the clear list's "FF" over. The clear-list's channel range is printed
// as (1),(2) Memory channel (0001~0099) only - it does not name the scan
// edges, corroborated by the front-panel table's scan-edge CLEAR: No row
// (PDF p.116, folio 107).
//
// core/civ ships NO clear builder for this model, and Profile.AllowedCommand
// has no branch that could admit either frame: it admits only 19 00, a
// valid 1A 00 read and a re-validated 1A 00 set. Every Icom driver gives
// spec.FieldErase the zero FieldSupport, and
// spec.ConsentUnverifiedWrites structurally never consents erase.
//
// # THE REGISTER
//
// Every entry names the assumption, what depends on it, and the capture
// that would lift it. CITE THESE ENTRIES BY NAME, NEVER BY POSITION.
// Shared D5 entries are the tier's own (their capture procedures are
// stated once, at core/civ/ic7610/doc.go, and re-stated here only where
// this radio's own document differs). ic7600-prefixed entries are this
// package's own; the ic7600 DRIVER register lives at
// core/driver/ic7600/doc.go and is named here, not duplicated.
//
// # D5 register (spec-level, shared across the tier)
//
//   - D5 entry 1, the 1A 00 READ-REQUEST FORM (R1). ASSUMED:
//     FE FE 7A E0 1A 00 <ch-hi> <ch-lo> FD, nine bytes (matrix S3.7). No
//     1A 00 read request is printed anywhere on this document either.
//   - D5 entry 2(a), an unwritten channel answers FA (R2a). ASSUMED
//     (matrix S3.8(a)).
//   - D5 entry 2(b), an all-FF 25-byte record read back means empty
//     (R2b). ASSUMED, narrower here than on the IC-7610: this radio's page
//     prints no Fixed-vs-FF contradiction to muddy the question (matrix
//     S3.8(b)) - what remains open is simply whether a full all-FF record
//     (as opposed to the documented single-byte clear form) also reads
//     back as empty.
//   - D5 entry 3, the memory-name SPACE character is 0x20 (R3), and D5
//     entry 3 again for the PAD byte (R4). Both ASSUMED (matrix S3.9(iii),
//     (iv)) - undocumented for 1A00 specifically on this page, inferred by
//     the same cross-command reasoning the IC-7610 matrix uses for its own
//     model.
//   - D5 entry 5, wire order equals printed index order (R5). ASSUMED -
//     this radio has no duplicated TX block, so the two SHOULD coincide,
//     which is not "does" (matrix S3.11).
//   - D5 entry 6, the record is 25 RecordOnlyLength / 27 DataAreaLength
//     bytes (R6, this package's own entry ic7600-record-length). ASSUMED,
//     a DERIVATION not a printed total - the internal check is that the
//     matrix's seven-term sum equals the last printed index, (27), exactly
//     as the S1 leg counted independently (matrix S3.11, "no
//     disagreement... a confirmation, not a new finding").
//   - D5 entry 7, the 19 00 REPLY VALUE (R7), undocumented on ALL SIX wave
//     models (matrix S3.12(i)). Nothing in this package's behaviour
//     depends on it: golden_test.go asserts the token comes back and
//     nothing about WHICH token.
//   - D5 entry 8, serial framing is 8-N-1 (R8). ASSUMED, on NO evidence
//     from this document (matrix S3.1) - the same posture the IC-7610
//     matrix takes for its own model.
//   - D5 entry 9, transceive broadcasts carry to=00 (R9). ASSUMED (matrix
//     S3.5(b)) - this document prints no transceive broadcast frame; the
//     only skeleton printed is the solicited to=E0 case.
//
// # civ PROFILE register (this package's own)
//
//   - ic7600-auto-baud-mechanics (R11) - what the front panel's
//     literal factory default "Auto" resolves to on the wire.
//     BETTER-EVIDENCED THAN THE IC-7610 IN ONE RESPECT AND ASSUMED IN
//     ANOTHER: PDF p.151 (folio 142)'s CI-V Baud Rate item states the
//     factory default directly as "Auto," not a numbered rate - genuinely
//     better evidence than the IC-7610's own CI-V Reference Guide manages
//     (matrix S3.3). What "Auto" resolves to for a given controller rate -
//     which of the five named rates an auto-bauding radio locks onto, and
//     whether it re-locks per frame or per session - is NOT stated and
//     stays ASSUMED.
//   - ic7600-civ-rate-list (R12) - that {300, 1200, 4800, 9600, 19200} are
//     the complete CI-V rate list. CE for completeness (matrix S1 row 13,
//     S3.3): the five rates are MANUAL-EVIDENCED as the front-panel choice
//     set (PDF p.151, folio 142), but unlike the IC-7610's own preamble
//     footnote, no independent corroborating table exists on this
//     document to prove the five are ALL of them.
//   - ic7600-1a00-set-ack (R14) - that a 1A 00 set is acknowledged with FB
//     and rejected with FA. ASSUMED for 1A 00 specifically (matrix S3.10),
//     same reasoning as the IC-7610 matrix.
//   - ic7600-full-record-mandatory (R15) - that the full 25-byte record is
//     mandatory on write. ASSUMED (matrix S3.10): one full-record form and
//     one three-byte clear form are printed, no partial/short form.
//   - NO ECHO REGISTER ENTRY: unlike the IC-7610, this document carries no
//     CI-V USB Echo Back item anywhere in its full 1A 05 sub-command list
//     (matrix S3.6) - this radio predates the feature. Echo suppression is
//     handled structurally regardless (NoteSent + drop-first-match), so
//     there is nothing here to lift.
//   - NO CONTROL-LINE HAZARD ENTRY: unlike the IC-7610, this document
//     carries no USB SEND/USB Keying item assigning DTR or RTS of the
//     CI-V USB port to PTT or keying (matrix S3.2, S3.16 ADDED-2) - that
//     Icom feature post-dates this model. The project CHOICE (open the
//     port with RTS/DTR deasserted and never toggle them) is unchanged,
//     but carries no model-specific consequence to guard against here.
//
// # ic7600 DRIVER register - LISTED HERE, REPRODUCED IN FULL THERE
//
// These entries live in core/driver/ic7600/doc.go:
//
//   - ic7600-ctcss-tone-list (R17) - the radio's actual usable CTCSS tone
//     list, versus the wider 1-2999 deciHz encoding range the 1B 00/1B 01
//     packed-BCD strip admits (matrix S1 row 12).
//   - ic7600-scan-edge-record-fields (R18) - whether every Sup field is
//     honoured on P1/P2, not merely shaped the same (matrix S2).
//   - ic7600-select-marker-semantics (R20) - byte (3)'s whole-byte
//     select-marker semantics on real hardware, mirroring
//     ic7610-select-marker-semantics (matrix S3.16 ADDED-1).
package ic7600

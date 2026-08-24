// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic905 holds the Icom IC-905's CI-V memory-record dialect: the
// civ.ProfileConfig that maps this radio's `1A 00` record onto core/civ's
// neutral field vocabulary, and the frozen quarantine evidence the tests
// bind it to.
//
// # What this package is
//
// DATA ONLY. It builds one civ.Profile from one config literal and
// exports it behind Profile(). It opens no session, touches no transport,
// registers nothing with the application, and cannot: internal/wiring's
// driver table is the only thing that makes a model reachable, and this
// package is not in it. The driver that will use this profile is
// core/driver/ic905; the fake that will answer it is internal/fakeic905.
// Neither is this package's business.
//
// # Provenance
//
// Everything here is a reading of one document, the IC-905 CI-V reference
// manual (ic905_civ_2.pdf), through four quarantined evidence legs that
// never opened this repository and never saw each other's output: a field
// ledger (L), a geometry witness (W), a transcription (B) and a golden
// vector set (G). All four are frozen byte for byte under testdata/ and
// hashed by frozen_test.go. Page references below are PDF pages, with the
// printed folio in brackets where the two differ, because the document's
// own cross-references cite folios and its PDF is two ahead.
//
// The graded evidence is docs/superpowers/icom-matrices/ic905-capability-matrix.md
// (rev 1 plus twelve errata) and its run report and review; citations
// below name matrix sections and errata by number.
//
// # Hardware status: UNVERIFIED
//
// NO IC-905 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Not one frame
// has been sent to one and not one has been received from one. Every byte
// in this package is manual-derived, and the register below records which
// of them the manual does not actually print.
//
// # The record-length convention
//
// This tier carries RECORD-ONLY lengths in civ.Profile — the payload
// AFTER the four channel-address bytes, which is what civ.RecordLayout's
// offsets and BuildMemorySet's record argument denote (spec Erratum 1).
// This model carries two of them, and it is the tier's only multi-length
// profile:
//
//	condition   frequency field    record-only    data area   whole 1A 00
//	            (6)~(10)           length         on the wire set frame
//	A           5 bytes            64             68          75
//	B (10 GHz)  6 bytes            65             69          76
//
// The channel-address width is FOUR bytes in both — (1),(2) group and
// (3),(4) channel, each two packed BCD — and it is EXCLUDED from the
// record length. Frame arithmetic throughout: frame = 7 + 4 + record.
// The tier spec D6's 68/69 figures are the same two records counted WITH
// the address bytes (matrix section 6, EC-1); both accountings are right,
// this package uses record-only everywhere and says "data area" when it
// means the other.
//
// Condition A's 64 is MANUAL-EVIDENCED, addend by addend, each addend a
// printed index range on one drawn diagram (matrix section 3.11
// Condition A, reproduced independently in the run report section 8) —
// AND the tension with the family register stays visible, as matrix
// Erratum 7 requires: spec D5 recurring entry 6 frames record totals
// family-wide as "derived by field arithmetic, never printed", and D6
// labels this model's lengths ASSUMED. Both stand. This package builds on
// the manual grade; it does not claim to have retired D5 entry 6's
// framing. Condition B's 65 is ASSUMED — see the register.
//
// # The two record lengths, and which one this profile builds
//
// THE BUILD LENGTH IS 64.
//
// There is NO READ-PATH problem to solve. Every field but the frequency
// is fixed width, so a 64-byte answer means five frequency bytes and a
// 65-byte answer means six: the length IS the discriminator, and this
// profile declares civ.DiscriminatorRecordLength over both layouts
// (matrix Erratum 8 corrected an earlier framing that treated this as an
// ambiguity).
//
// The real question is the WRITE path, where a width must be chosen
// before there is a length to read. civ.ProfileConfig.BuildLength is a
// single static int and BuildMemorySet emits it and nothing else, so a
// profile cannot pick per record. The choice was between:
//
//   - 64 — the only record shape the memory-content diagram draws
//     (MANUAL-EVIDENCED, matrix section 3.11 Condition A). A 10 GHz
//     record then fails CLOSED inside the encoder: encoding
//     10,250,000,000 into five packed-BCD bytes does not fit, so
//     BuildMemorySet returns an error naming rx_frequency and its width.
//     Nothing assumed reaches a radio.
//   - 65 — which would send an ASSUMED shape on EVERY write, including
//     the sub-10-GHz writes the diagram does draw at 64.
//
// 64 is what this profile declares. The tier writes only the shape its
// document draws, and refuses the 10 GHz write honestly until
// ic905-R-06 lands. SPEC ERRATUM 2'S DELIBERATE GATE WIDTH IS WHAT MAKES
// THAT COHERENT: AllowedCommand admits a memory set at EITHER declared
// length, so a 65-byte record the radio answered with can be validated
// and — once the lift lands — written back with no gate change.
//
// # The band rule, and why the record needs no band field
//
// Ten packed-BCD digits reach at most 9,999,999,999 Hz. The 10G band
// starts at 10,000,000,000 Hz (PDF p.20 folio 19, "Band stacking
// register", "(1): Frequency band codes", row `06 | 10G |
// 10000.000000 ~ 10500.000000`; printed again at PDF p.30 folio 29, per
// matrix Erratum 9), and no band is documented between 5850 MHz and
// 10 GHz. So over the documented storable set, "needs six bytes" and "is
// in band 06" are the SAME predicate:
//
//	six bytes iff freqHz > 9,999,999,999
//
// That equivalence is why the record carrying no band field costs
// nothing on the write path. NeedsWideFrequency and
// RecordLengthForFrequency (length.go) pin it, so that the one-line
// change ic905-R-06 would authorise is already written down and tested
// even while BuildLength is 64.
//
// # The ASSUMED register
//
// NINETEEN members of this package are not IC-905-manual facts. Each is
// listed here with the value it carries, the evidence gap, and the ONE
// capture that lifts it, in the form core/cat/ftdx101/doc.go uses.
//
// EVERY CAPTURE BELOW IS ON AN IC-905, and no capture from any other
// model in this tier lifts any entry here. These are per-model readings
// of a per-model document; a sibling's bytes are not evidence about this
// radio. (core/civ's own doc.go carries the eight tier-wide CONVENTIONS
// this register sits beneath, and does not restate them per model.)
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION, on core/civ's standing
// rule: a positional citation is correct only until somebody adds one.
//
//   - THE `1A 00` READ-REQUEST FORM (D5 entry 1). This profile builds
//     `FE FE AC E0 1A 00 <4 address bytes> FD` and nothing else. The
//     document prints the full SEND layout and exactly one truncated
//     layout — the clear form on the same page, which stops after the
//     same four addressing fields — and no read form at all (matrix
//     section 3.7). Readings it does not exclude: a read carrying no
//     address bytes, or one that also carries field (5).
//     LIFTED BY: ic905-R-01 — send `FE FE AC E0 1A 00 00 00 00 01 FD`
//     and record what comes back, a memory-content reply for group 00
//     channel 01 or an `FA`.
//
//   - AN EMPTY CHANNEL ANSWERS `FA` (D5 entry 2(a)). The document prints
//     the NG frame's FORMAT (PDF p.3, folio 2) and nowhere states what a
//     read of an unwritten channel returns (matrix section 3.8(a)).
//     LIFTED BY: ic905-R-14 — read a front-panel-cleared channel and
//     record the reply frame in full.
//
//   - AN ALL-`FF` RECORD MEANS EMPTY (D5 entry 2(b)). The clear form
//     writes `FF` into field (5) and nothing beyond it; the document
//     never prints a full-length record of `FF`s and never says what an
//     erased channel reads back as (matrix section 3.8(b)). Two entries
//     rather than one because a capture that returns `FA` says nothing
//     about the `FF` case, and one that returns a record says nothing
//     about whether `FA` is ever used.
//     LIFTED BY: ic905-R-15 — on the same known-empty channel, if the
//     reply is a record rather than `FA`, record every byte of it.
//
//   - THE SPACE INSIDE A NAME IS `0x20` (D5 entry 3). nameCharset
//     carries 0x20 as its ninety-fifth member. Neither printed character
//     table contains a space, and the field is sixteen characters fixed,
//     so a short name needs a byte the tables do not print; 0x20 is the
//     only value this document ever prints for a space, and it prints it
//     three times in three tables that govern OTHER fields — CW message
//     contents (PDF p.18), keyer memory entries (PDF p.21) and call-sign
//     characters (PDF p.24) (matrix section 3.9). It is G's assumption
//     A2 and the ninth byte of both golden vectors' names.
//     LIFTED BY: ic905-R-16 — store a name containing a space from the
//     front panel, read the channel, record the byte in that position.
//
//   - THE `19 00` REPLY VALUE (D5 entry 7). The transceiver-ID read is
//     built by this profile; what an IC-905 answers with is printed
//     nowhere in this document (matrix section 1 row 2, section 3.12).
//     Per spec D3.2 the token is recorded in diagnostics and never
//     matched, so nothing here depends on its value — but the read is
//     sent, so the gap is registered.
//     LIFTED BY: ic905-R-02 — send `FE FE AC E0 19 00 FD` and record the
//     data bytes of the address-matched reply.
//
//   - THE TRANSCEIVE BROADCAST ADDRESS FORM, `to=00` (D5 entry 9). The
//     accumulator tolerates transceive traffic by ADDRESS FILTERING, so
//     the form of the address it filters on matters. This document prints
//     exactly four frames, all of them addressed, none with a `00`
//     destination, and never uses the word broadcast (matrix
//     section 3.5(b)).
//     LIFTED BY: ic905-R-12 — with transceive on, change frequency from
//     the front panel and record the `to` byte of the unsolicited frame.
//
//   - THE SIX-BYTE FREQUENCY AND THE 65-BYTE SECOND RECORD LENGTH (D5,
//     model-specific entry for the 905). layoutFor(6) and
//     RecordLengthWide. The memory-content diagram prints ONE shape, at
//     five frequency bytes; the 5/6-byte conditional is printed against
//     four OTHER command lists, none of which includes `1A 00` (matrix
//     section 3.11 Condition B, Erratum 1). Both halves are assumed: that
//     the memory record widens at all, and the width it takes.
//     LIFTED BY: ic905-R-06 — store a channel at 10500.000000 MHz from
//     the front panel, read it with `1A 00`, and record the returned
//     frequency bytes AND THEIR COUNT.
//
//   - OVER-BUDGET BEHAVIOUR (D5, model-specific entry for the 705/905).
//     What the radio does when one more channel is written past a full
//     memory is printed nowhere: no error, no `FA` condition, no note
//     (matrix section 3.15(c)). Per spec D4 the budget is enforced at
//     Diff time and never sent, so this is a check on a refusal the
//     software makes for itself.
//     LIFTED BY: ic905-W-03 — on a radio already holding the budget's
//     worth of channels, send one more `1A 00` set to an unoccupied
//     address and record whether the answer is `FB`, `FA`, or nothing.
//
//   - ic905.bauds — THE OFFERED CI-V RATE LIST. This document prints no
//     rate anywhere; "Preparing" (PDF p.3, folio 2) hands the speed to
//     the Basic manual, and a sweep of every command-table page finds no
//     figure (matrix section 1 row 9, section 3.3).
//     LIFTED BY: ic905-R-04 — open at each offered rate in turn, send
//     `FE FE AC E0 19 00 FD` at each, record which rates answer.
//
//   - ic905.default_baud — THE FACTORY-DEFAULT CI-V RATE. Not printed;
//     the same deferral as ic905.bauds, and a distinct claim from it
//     (matrix section 1 row 10, section 3.3).
//     LIFTED BY: ic905-R-03 — on a factory-default radio, open at the
//     assumed default rate and record whether `19 00` is answered.
//
//   - ic905.transceive_default — WHETHER CI-V TRANSCEIVE IS ON AT
//     FACTORY SETTINGS. The setting exists and is two-valued (`1A 05`
//     `01 42`, PDF p.9 folio 8); no default is printed (matrix
//     section 3.5(a)). NOTHING IN THIS TIER TURNS IT OFF: there is no
//     transceive-set builder, `AllowedCommand` refuses `1A 05` outright,
//     and the CI-V InitSequence is empty. The default is recorded as
//     evidence and the accumulator tolerates whatever it finds.
//     LIFTED BY: ic905-R-11 — on a factory-default radio, open the port,
//     change frequency from the front panel, record whether any
//     unsolicited frame arrives.
//
//   - ic905.usb_echo_default — WHETHER CI-V USB ECHO BACK IS ON AT
//     FACTORY SETTINGS. The setting exists and is two-valued (`1A 05`
//     `01 43`, PDF p.9 folio 8); no default is printed (matrix
//     section 3.6). Echo suppression matches RECORDED BYTES, never
//     position or count, so this profile does not depend on the value.
//     LIFTED BY: ic905-R-13 — send `FE FE AC E0 19 00 FD` on a
//     factory-default radio and record whether the first frame back is a
//     byte-for-byte copy of the frame sent.
//
//   - ic905.name_pad_byte — THE BYTE A SHORT NAME'S TAIL CARRIES.
//     NamePad is 0x20. THIS IS NOT D5 entry 3'S CLAIM and not lifted by
//     ic905-R-16: a radio could accept 0x20 INSIDE a name and still pad
//     with 0x00, or 0xFF, or refuse a short name. The document names no
//     pad byte at all (matrix section 3.9, which grades the two
//     separately and says why).
//     LIFTED BY: ic905-R-17 — with a four-character name stored from the
//     front panel, read that channel and record bytes 5-16 of the name
//     field.
//
//   - ic905.tone_block_assignment — WHICH OF (16)~(18) AND (19)~(21) IS
//     THE TX REPEATER TONE. This profile maps (16)~(18) to
//     civ.FieldToneTX and (19)~(21) to civ.FieldToneRX. PDF p.19
//     (folio 18) prints "Repeater tone frequency setting" for BOTH,
//     under one pointer to a section whose title names two settings; all
//     four quarantine legs raise it as a STOP (matrix section 6, EC-3).
//     It is a CHOICE over a two-valued assumption, not a reading. NO
//     DELIVERED BYTE DEPENDS ON IT BEING RIGHT: both golden vectors
//     carry `00 08 85` in both blocks.
//     LIFTED BY: ic905-R-07 — set the repeater tone and the tone squelch
//     frequency to DIFFERENT values from the front panel and read the
//     channel; one capture settles both ends.
//
//   - ic905.dtcs_polarity_nibbles — WHICH NIBBLE OF (22) IS THE
//     TRANSMIT POLARITY. dtcsPolarityNames reads the HIGH nibble as
//     transmit and the LOW as receive. Matrix Erratum 3 re-read cell (1)
//     of PDF p.24's "DTCS code and polarity setting" at 600 dpi and
//     found the leaders NEST rather than cross, confirming that
//     assignment; it is read off artwork rather than off a printed byte
//     value, so it stays assumed.
//     LIFTED BY: ic905-R-08 — store a channel with ASYMMETRIC polarity
//     (normal transmit, reverse receive) and read it back.
//
//   - ic905.group_budget — HOW MANY CHANNELS THE RADIO ACTUALLY HOLDS.
//     The document prints a 100 x 100 addressable space and no budget
//     (matrix section 1b, section 3.15(c)). Spec D4's figure of 500 is a
//     roadmap-era derivation, not evidence about this radio. This
//     package declares the address SPACE and no budget; the budget is
//     the driver's capability datum.
//     LIFTED BY: ic905-R-09 — on a radio holding a known number of
//     channels, enumerate and record how many the radio accepts.
//
//   - ic905.min_storable_hz — 144,000,000 Hz AS A MEMORY-RECORD FLOOR.
//     The VALUE is manual-evidenced (PDF p.20 folio 19, "Band stacking
//     register", "(1): Frequency band codes", row `01 | 144 |
//     144.000000 ~ 148.000000`); reading a BAND-STACKING table as the
//     MEMORY RECORD's floor is the inference (matrix section 1 row 11).
//     LIFTED BY: ic905-R-05 — store a channel at 144.000000 MHz from the
//     front panel, read it with `1A 00`, record the returned bytes.
//
//   - ic905.max_storable_hz — 10,500,000,000 Hz AS A MEMORY-RECORD
//     CEILING. Same table, row `06 | 10G | 10000.000000 ~ 10500.000000`;
//     same inference (matrix section 1 row 12). It is a SEPARATE entry
//     from the floor, per matrix Erratum 6, because no single capture
//     observes both ends.
//     LIFTED BY: ic905-R-06 — the same capture that settles the six-byte
//     frequency, since the ceiling is in the band that needs it.
//
//   - ic905.call_channel_erasable — WHETHER THE CALL BANK CAN BE ERASED
//     AT ALL. What the document prints is narrower than the capability:
//     the `1A 00` CLEAR FORM forbids group `01 00` (PDF p.19 folio 18,
//     "You cannot specify group \"01 00\" (Call channel group)"), which
//     is evidence about ONE WIRE FORM. That the bank cannot be erased by
//     any means is the assumption — the document records a second clear
//     surface at `0B` (Memory clear) which is nowhere stated to exclude
//     call channels (matrix Erratum 2). NOTHING IN THIS TIER CAN ERASE
//     ANYTHING: there is no clear builder, no clear frame and no gate
//     branch that could admit one, so the entry records a fact about the
//     radio that this package's own structure makes unreachable.
//     LIFTED BY: ic905-W-04 — select a populated call channel, send
//     `0B`, and read it back with `1A 00`. (Matrix Erratum 11 renumbered
//     this from `ic905-W-03`, which the over-budget capture held first.)
//
// # Where the other five entries live
//
// THE COUNT ABOVE IS PER REGISTER, and this note is what keeps a reader
// from expecting one register to hold the lot. core/driver/ic905/doc.go
// carries FIVE more, and NO ENTRY IS COUNTED TWICE:
// ic905.control_lines_at_open (ic905-W-01); ic905.write_full_record_required
// (ic905-W-02); SERIAL FRAMING 8-N-1 (D5 entry 8, ic905-R-10) — which
// lives THERE and not here, because the thing that acts on it is the
// driver's StopBits(); ic905.create_default_tone (ic905-R-18); and
// ic905.mode_band_constraint (ic905-R-19).
//
// Nineteen here plus five there is TWENTY-FOUR distinct entries, and
// every count in this package says which register it is counting.
//
// # What this profile cannot express
//
// TWENTY-SEVEN OF THE SIXTY-FOUR RECORD BYTES HAVE NO NEUTRAL HOME.
// civ.FieldID has no id for them and codeplug.ChannelData has nowhere to
// put them, so they are unmapped and sit in the layout's Fixed template:
//
//	(5)        1 byte    Split and Select memory setting
//	(15)       1 byte    Digital squelch setting
//	(25)       1 byte    DV digital code squelch
//	(29)~(36)  8 bytes   UR (Destination) call sign
//	(37)~(44)  8 bytes   R1 (Access repeater) call sign
//	(45)~(52)  8 bytes   R2 (Gateway/Link repeater) call sign
//
// Spec D4 says "DV/D-STAR call-sign fields: read-only typed capture where
// cheap, never writable this tier" — but there is no neutral field to
// capture them INTO, so this tier cannot even read them out. The
// consequence is a WRITE hazard, not a read one: encodeRecord fills
// unmapped bytes from the template, so a write to a channel carrying real
// call signs, a set digital squelch or a SELECT star would silently
// replace them. THE TIER RULING E6 FORBIDS THAT: a driver may write a
// slot only when its unmapped regions equal this template, and anything
// else is refused with the offending byte range named. That is
// core/driver/ic905's ladder; this package's part is to state the
// template once, and to include byte (5) in it.
//
// (5) IS DELIBERATELY NOT MAPPED TO civ.FieldSelect. The hard tier
// constraint is that scan_skip on an Icom is SELECT-group membership and
// must never be mapped as skip. civ.FieldSelect exists, but the neutral
// codeplug.ChannelData has no home for it, so mapping it would make
// BuildMemorySet demand a value the driver could only invent —
// validateRecordFields refuses a mapped field with no value. Leaving (5)
// in Fixed at 0x00 is also exactly what the CALL bank requires ("* Set 0
// for Call channel.", PDF p.19 folio 18, the (5) breakout footnote) and
// exactly what both golden vectors carry.
//
// THE DTCS CODE'S 0-7 DIGIT RANGE IS NOT ENFORCED HERE. PDF p.24
// (folio 23) prints every DTCS digit as `0 ~ 7`; civ's packed-BCD encoder
// accepts 0-9 and civ.Profile has no digit-subset policy. The primary
// gate is core/driver/ic905's explicit 512-code table, which
// codeplug.Validate applies before the driver is reached; the driver
// re-checks for itself before building. Both are limits of THIS PROFILE,
// not of the radio.
//
// THE CALL BANK'S `(5) = 0` RULE is satisfied by construction: (5) is
// Fixed 0x00 in both layouts, and under E6 a CALL channel whose stored
// (5) is not 0x00 is refused rather than rewritten.
//
// # One known over-admission
//
// civ.ProfileConfig carries a SINGLE GLOBAL ChannelLo/ChannelHi, 0..99,
// so the builder and the gate admit CALL-group (wire group 100) channels
// 12..99, which this document does not define. Nothing reaches them: the
// CALL bank is twelve named slots, "C01".."C12"; a bank walk cannot
// produce such an address; and core/driver/ic905's refusal ladder refuses
// a slot in no effective bank that arrives by hand. It is stated here as
// a known consequence of the shared config's shape, NOT as a claim that
// the radio has ninety-nine call channels.
package ic905

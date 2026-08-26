// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7300 simulates an Icom IC-7300's CI-V behaviour over an
// in-memory serial connection (Radio.Port()). It is the test double the
// IC-7300's own layers run against — the transport engine, the driver, the
// CLI's fake mode — the role internal/fakeradio plays for the FT-710 and
// internal/fakedx101 for the FTdx101 family.
//
// # THE HARD RULE
//
// THIS PACKAGE IMPORTS THE STANDARD LIBRARY AND NOTHING ELSE. No file in this
// directory, or in any directory beneath it, may import anything under
// github.com/gm5dna/open-rig-programmer/ — not core/civ, not core/civ/ic7300,
// not core/driver, not core/codeplug, not core/spec, and not a sibling fake.
// imports_test.go enforces it by walking the tree, and its own self-tests prove
// the walk bites.
//
// The rule exists so that this fake is an INDEPENDENT SECOND OPINION about the
// wire. A fake built on the production codec agrees with the production codec
// by construction: a systematic bug in the record's length, its offsets or its
// vocabularies would be encoded identically on both sides of every end-to-end
// test, and every one of them would pass. Everything this package knows about
// the record it re-derived, from the memory-record diagram's semantic
// transcription (core/civ/ic7300/testdata/ic7300-transcription-b.csv and .md)
// and the diagram's measured geometry
// (core/civ/ic7300/testdata/ic7300-geometry-witness.csv and .md), and from
// nothing else in this repository. No golden file, no field ledger, no plan and
// no production source was consulted. record.go carries the derivation.
//
// # The address is not a literal
//
// This radio's default CI-V address is 94 and the controller's is E0, so a
// frame from the controller reads FE FE 94 E0 <cn> [<sc>] <data> FD and every
// answer swaps the pair: FE FE E0 94 ... FD. The OK frame is the six bytes
// FE FE E0 94 FB FD and the NG frame the six bytes FE FE E0 94 FA FD.
//
// EVERY BYTE ABOVE THAT SHOWS THIS RADIO'S ADDRESS IS THE ADDRESS IT IS
// CURRENTLY CONFIGURED WITH, NOT THE LITERAL 94. WithRadioAddress moves it, and
// when it does, the `from` byte of every answer moves with it — the identity
// answer, the record answer, the OK frame and the NG frame alike — and the
// radio answers only frames whose `to` byte matches its new address, counting
// and ignoring the rest.
//
// # What it answers
//
// It tolerates leading noise before a frame and any number of extra FE preamble
// bytes, and answers only frames addressed to its own address.
//
//	19 00                          → FE FE E0 94 19 00 <token> FD
//	1A 00 <ch hi> <ch lo>          → FE FE E0 94 1A 00 <ch hi> <ch lo> <record> FD,
//	                                 or NG if that channel has never been written
//	1A 00 <ch hi> <ch lo> <record> → OK if the record is exactly 39 bytes and
//	                                 every field's value is one the page prints;
//	                                 NG otherwise
//	anything else                  → NG
//
// The channel address space is the three forms the diagram's own legend prints
// and no fourth: `00 01`-`00 99` (memory channels 01 to 99), `01 00` (scan edge
// P1) and `01 01` (scan edge P2). See slots.go.
//
// # Why the clear forms are refused
//
// The page prints a clear procedure — a three-line list headed "To clear the
// memory channel contents on 1A 00:", giving ①,② the memory channel, ③ the
// value "FF" and ④ "None", i.e. the frame 1A 00 <ch hi> <ch lo> FF. THIS FAKE
// REFUSES IT, DELIBERATELY, and answers NG.
//
// The refusal is not an oversight and not a claim that a real IC-7300 would
// refuse it. It is a fence: the software under test ships no erase path, so
// nothing it can legitimately do should ever put that frame on the wire. A fake
// that accepted the clear would make an accidental erase — a truncated record,
// a mis-built frame, a stray FF — look exactly like success in every test that
// runs against it, and the first place anybody would find out is somebody's
// radio. Answering NG means an end-to-end test can assert, positively, that the
// frame never reached the wire (Received()) and that nothing acknowledged it
// (Sent()). Any 1A 00 set whose data area is a single FF byte is refused by the
// length check in handleMemory, which is the same check that refuses every
// other wrong-length record.
//
// # ASSUMED register
//
// Every place the two artefacts did not determine an answer, what was chosen,
// and why. Nothing outside them was consulted to settle any of it.
//
//  1. THE RECORD IS 39 BYTES, from the printed index ranges and not the drawn
//     cells. The geometry witness measures the band as 19 drawn regions and
//     raises five STOPs over it: `④–⑧` spans three drawn cells for five printed
//     indices, `⑱–㉗` three for ten, `❹–⓱` one undivided region for fourteen.
//     Two of those regions are dashed-outline cells printing "..." and the
//     third a dotted region containing a row of dots — elision markers, which
//     the transcription explicitly distinguishes from byte cells and declines
//     to count as one. Counting drawn regions would make the record 17 bytes,
//     which nothing on the page supports; accumulating the printed index ranges
//     gives 39 after the two channel-address bytes, and 41 with them, which is
//     the number the transcription itself records as the last field's measured
//     byte position (32-41). CHOSEN: the index ranges. See record.go.
//
//  2. THE LEFT HALF OF A NIBBLE-CODED CELL IS THE HIGH NIBBLE. The ③ and ⑪
//     legends each divide one byte with a dotted mid-point rule and label the
//     two halves; both artefacts trace the leaders and both report the same
//     crossing, so WHICH LABEL BELONGS TO WHICH HALF is determined. Which half
//     is the more significant nibble is not printed anywhere. CHOSEN: left is
//     high, the ordinary reading of a byte drawn left to right, which is also
//     how the same diagram draws the channel address (`00 99` = channel 99).
//
//  3. FIELDS THE TRANSCRIPTION MARKS `unstated` ACCEPT ANY BYTE. Operating
//     frequency (④–⑧), operating mode (⑨, ⑩), repeater tone frequency (⑫–⑭) and
//     tone squelch frequency (⑮–⑰) print no encoding and no code list — only a
//     cross-reference to a section that was not transcribed, and which for the
//     tone pair the transcribing leg reports finding on none of the pages it
//     read. CHOSEN: accept anything of the right width. Inventing a range would
//     make this fake refuse records a real radio takes, which is the more
//     damaging error for a test double: a fake that is too strict fails honest
//     software.
//
//  4. THE ❹–⓱ BLOCK IS 14 BYTES AND ITS CONTENTS ARE UNCHECKED. The page prints
//     no label, no encoding and no values for it; the transcription's label and
//     value cells for that row are empty. The only text printed about it is a
//     NOTE saying "The same data as ④–⑰ are stored in ❹–⓱", which would imply a
//     14-byte mirror of ④–⑧, ⑨⑩, ⑪, ⑫–⑭ and ⑮–⑰ (5+2+1+3+3 = 14, matching the
//     printed index range exactly). CHOSEN: take the WIDTH from the printed
//     index range, which is how every other field's width was taken, but apply
//     NO vocabulary to any byte inside it — including the byte the NOTE would
//     put ⑪'s nibble code at. Reading a vocabulary out of the NOTE would be an
//     inference from prose about a field the diagram labels with nothing.
//
//  5. THE MEMORY NAME'S BYTES MUST LIE IN 20-7E. The ⑱–㉗ field is `ascii` by
//     the transcription's own reading, whose evidence is the cross-referenced
//     character table: "A–Z 41–5A", "a-z 61–7A", "0–9 30–39" and a 40-entry
//     symbol table of which six entries are quoted, running from "! 21" to
//     "~ 7E", together with the command table's line "1A 00 | Memory name /
//     All characters are usable." The complete symbol list was not transcribed,
//     and no pad byte for a name shorter than ten characters is printed
//     anywhere. CHOSEN: accept 20 to 7E inclusive — every printed code lies
//     inside it, "all characters are usable" argues against carving holes in
//     it, and 20 is included because a ten-byte field holding "up to 10
//     characters" must be padded and space is the only candidate in the span.
//     REJECTED: enumerating only the printed codes, which would refuse the
//     thirty-four symbols the transcription elided.
//
//  6. `00 00` IS NOT A CHANNEL ADDRESS. The legend's range opens at `00 01`
//     ("Memory channel 01 to 99"), so channel 00 is printed nowhere. CHOSEN:
//     refuse it, and refuse any second byte that is not two BCD decimal digits
//     (`00 1A` is not in the printed range either). The legend's three forms,
//     and no fourth.
//
//  7. A READ'S ANSWER ECHOES `1A 00` AND THE CHANNEL ADDRESS. The page prints
//     the command and the record's field layout; it prints no answer frame.
//     CHOSEN: FE FE E0 94 1A 00 <ch hi> <ch lo> <record> FD, on the diagram's
//     own structure — the data block it draws BEGINS with `①, ②` "Memory
//     channel numbers", so the channel address is part of the block, not merely
//     a selector in front of it. A caller reading the answer finds the record
//     at a fixed offset of six bytes and can check the address it got back
//     against the one it asked for.
//
//  8. THE IDENTITY ANSWER'S SHAPE AND ITS DEFAULT TOKEN. `19 00` is sent with
//     no data area and its answer carries at least one data byte, whose value
//     is undocumented on this radio. CHOSEN: echo the command in the answer, as
//     the read answer does — FE FE E0 94 19 00 <token> FD — and let WithIDToken
//     fix the token. With no such option the token is ONE BYTE, the address the
//     radio is currently configured with, resolved after every option has run
//     so that it follows a WithRadioAddress given in any position.
//
//  9. AN ANSWER'S `to` BYTE IS THE CONSTANT E0, not the `from` byte of the
//     frame being answered. The two coincide in every ordinary exchange, since
//     the controller's address is E0. CHOSEN: the constant, because the
//     controller's address is stated as fixed while only the radio's is
//     described as configurable — so a same-address collision constructed with
//     WithRadioAddress changes the `from` byte of the answers and leaves the
//     `to` byte where it was.
//
//  10. WHAT THE TRANSCRIPTS RECORD. Received() lists every complete frame,
//     including the ones the radio ignored for not being addressed to it,
//     NORMALISED TO EXACTLY TWO PREAMBLE BYTES — leading noise is dropped and a
//     run of three or more FEs is recorded as two, because a variable-length
//     preamble would make an assertion about a frame's bytes depend on how the
//     host chunked its writes. Sent() lists everything the radio put on the
//     wire, answers and echoes and unsolicited traffic alike, recorded at the
//     moment the radio commits a frame rather than after the write returns:
//     net.Pipe returns from Write only once the reader has the bytes, so
//     recording afterwards would leave a window in which a host had read a
//     frame Sent() did not yet list. A frame whose write later fails is
//     therefore still listed.
//
//  11. THE FRAMER'S 256-BYTE CAP is this package's own bounded-input policy and
//     no manual figure. The longest frame this radio has business seeing is a
//     `1A 00` set at 48 bytes. Over the cap with no FD in sight, the partial is
//     dropped and the framer hunts for a fresh FE FE; nothing is answered,
//     because nothing was framed.
//
//  12. THE UNSOLICITED FRAMES' CONTENT. Neither artefact describes a transceive
//     frame at all. CHOSEN: a fixed, meaningless six-byte data area, the same
//     for WithTransceiveBroadcasts and WithAddressedFlood, differing only in
//     the `to` byte (00 against E0). What those options exist to produce is
//     TRAFFIC — bytes arriving at a host that did not ask for them — and no
//     test should be reading meaning out of them.
//
//  13. THE BUS ECHO IS OFF BY DEFAULT AND ECHOES EVERYTHING. Neither artefact
//     mentions echo; it is a property of the single-wire bus rather than of
//     this record. CHOSEN: off unless WithEcho(true), and when on, every
//     complete frame verbatim, before any answer to it, including frames
//     addressed to some other radio.
//
//  14. AN EMPTY CHANNEL ANSWERS NG. The page prints no "this channel is empty"
//     answer and no factory image. CHOSEN: a radio with nothing seeded starts
//     EMPTY, and a read of a channel that has never been written answers the
//     six-byte NG frame. Manufacturing a default record would have required
//     inventing values for the four fields whose vocabularies are unstated.
//
//  15. WHAT AN OPTION DOES WITH A BAD VALUE. Not a wire question at all. CHOSEN:
//     panic. WithIDToken on an empty token, WithRadioAddress on FE or FD, and
//     WithChannel or WithRawChannel on an address outside the three printed
//     forms all panic; WithChannel also panics on a record it would refuse from
//     the wire. Every one of those values is a literal in a test, so a bad one
//     is a programming error, and a fake that quietly ignored it would answer
//     the wrong thing several layers from the typo.
package fakeic7300

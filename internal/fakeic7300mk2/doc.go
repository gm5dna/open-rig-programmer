// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7300mk2 simulates an Icom IC-7300MK2's CI-V behaviour over an
// in-memory serial connection (Radio.Port()). It is a test double: the thing a
// transport engine, a driver, a CLI --fake mode or a GUI demo mode can talk to
// when there is no radio on the desk.
//
// # The wire
//
// CI-V frames are FE FE <to> <from> <cn> [<sc>] <data> FD. FE (preamble) and
// FD (end of message) are reserved and appear nowhere inside a well-formed
// frame. This fake tolerates leading noise and any number of extra preamble
// bytes, so FF FF FE FE FE FE B6 E0 19 00 FD is read as the identity request
// it is.
//
// It answers three things and refuses everything else:
//
//	19 00                        -> 19 00 <token>, the identity answer
//	1A 00 <ch hi> <ch lo>        -> 1A 00 <ch hi> <ch lo> <record>, or FAIL
//	1A 00 <ch hi> <ch lo> <rec>  -> the six-byte PASS frame, or FAIL
//
// Everything else — 0B, 18 00, 18 01, 1A 05, a 1A 00 set whose data area is a
// single FF byte, a 19 with any other sub-command, a 1A 00 set of the wrong
// length, a set carrying a value the manual does not print, a read of a
// channel that has never been written, a channel address outside the three
// printed forms — answers the six-byte FAIL frame.
//
// # The radio's address is not a literal
//
// The default CI-V address is B6 and the controller's is E0, so a frame from
// the controller reads to=B6 from=E0 and every answer swaps them, to=E0
// from=B6. The PASS frame is FE FE E0 B6 FB FD and the FAIL frame is
// FE FE E0 B6 FA FD, six bytes each.
//
// EVERY BYTE ABOVE THAT SHOWS THIS RADIO'S ADDRESS IS THE ADDRESS IT IS
// CURRENTLY CONFIGURED WITH, NOT THE LITERAL B6. If WithRadioAddress changes
// it, the `from` byte of every answer follows — the identity answer, the
// record answer, the OK frame and the NG frame alike — and the radio answers
// only frames whose `to` byte matches its configured address, counting and
// ignoring the rest. A sibling test that hard-codes B6 against a radio built
// with WithRadioAddress will fail for that reason and not for a code one.
//
// # THE HARD RULE: nothing project-internal
//
// fakeic7300mk2 MUST NOT import any package of this project — not core/civ,
// not core/civ/ic7300mk2, not core/driver, not core/codeplug, not core/spec,
// and not any sibling fake, internal/fakeic7300 included. Standard library
// only, in every non-test file, in this directory AND every directory beneath
// it.
//
// The reason is not style. If this fake reused the production codec, a
// systematic bug in that codec — an off-by-one in a field offset, a validation
// rule subtly wrong, a length derived from the wrong column of the
// transcription — would be applied identically on both sides of every "send a
// command, check the reply" test this project runs. The bug would never
// surface: the fake would misbehave in exactly the way the buggy codec
// expects, and every end-to-end test would pass anyway. Two independent
// implementations of one protocol, checked against each other and against
// expectations recomputed by hand in tests, is what makes that class of bug
// visible.
//
// TestNoCoreImports (imports_test.go) enforces it with a go/parser scan that
// WALKS SUBDIRECTORIES, and three sibling tests keep the scan itself honest,
// including the vacuity guards that would otherwise let it pass having
// examined nothing.
//
// # A SIBLING, not a refactor
//
// This package duplicates a good deal of internal/fakedx101, whose shape it
// takes: the pipe-and-goroutine Radio, the interruptible sleep, the shutdown
// channel, the Option list, the recursive import fence. It duplicates a good
// deal of internal/fakeic7300 too, which fakes the MK2's predecessor. Neither
// duplication is going to be factored into a shared "fake core" package: the
// rule above forbids importing anything to share, and a sibling fake is a
// sibling, not a refactor.
//
// The record geometry, the field offsets and the vocabularies below were
// re-derived for this package from the two artefacts named in "Provenance",
// and from nothing else. No sibling's Go source was read while writing it.
//
// # Provenance
//
// Everything in record.go comes from two documents and their prose companions:
//
//	core/civ/ic7300mk2/testdata/ic7300mk2-transcription-b.csv and .md
//	core/civ/ic7300mk2/testdata/ic7300mk2-geometry-witness.csv and .md
//
// Both transcribe the IC-7300MK2 CI-V REFERENCE GUIDE (revision A7841-8EX,
// Oct. 2025), PDF page 17's diagram D1, captioned "• Memory channel content /
// Command: 1A 00", together with the cross-referenced material on pages 16,
// 18 and 23.
//
// # The record
//
// FORTY-FIVE bytes, following the two channel-address bytes. Diagram D1's nine
// printed index blocks total 47 indices; the first block, ①, ② Memory channel
// number, is the channel address the command carries before the record, so the
// record is 47 - 2 = 45. Field by field, offsets counted from the first byte
// after the channel address:
//
//	off  width  field
//	  0      1  ③      Split and Select memory setting
//	  1      5  ④ ~ ⑧  Operating frequency setting
//	  6      2  ⑨, ⑩   Operating mode setting (mode, filter)
//	  8      1  ⑪      Data mode and tone type settings
//	  9      3  ⑫ ~ ⑭  Repeater tone frequency setting
//	 12      3  ⑮ ~ ⑰  Tone squelch frequency setting
//	 15     14  ❹ ~ ⓱  the transmit side: "the same data as ④ ~ ⑰"
//	 29     16  ⑱ ~ ㉝  Memory name settings
//
// and within the ❹ ~ ⓱ block, which repeats ④ ~ ⑰'s layout byte for byte:
//
//	15      5  frequency
//	20      2  mode, filter
//	22      1  data mode and tone type
//	23      3  repeater tone frequency
//	26      3  tone squelch frequency
//
// # The channel address space
//
// Three forms, and no fourth, because the ①, ② legend on page 17 prints three:
//
//	00 01 ~ 00 99   Memory channel 01 ~ 99   (canonically "001" … "099")
//	01 00           Programmed scan edge P1  (canonically "P1")
//	01 01           Programmed scan edge P2  (canonically "P2")
//
// 00 00 is not among them and is refused. So is any 00 nn whose second byte is
// not packed BCD, since the legend's own encoding note says "no hexadecimal
// digit above 9 is used".
//
// # Why the clear forms are refused
//
// The documented clear forms — the 0B command, and a 1A 00 frame whose data
// area after the channel address is the single byte FF — are answered FAIL,
// deliberately, and this fake implements no erase path at all.
//
// THE SOFTWARE UNDER TEST SHIPS NO ERASE PATH. A fake that quietly cleared a
// channel would be a fake with a behaviour nothing on the other side of the
// wire can ask for, and the day something did ask for it — a stray FF where a
// record should be, a 0B leaking out of a half-written command builder — the
// fake would silently destroy state instead of saying no. Refusing is the
// behaviour that makes such a frame visible: it comes back FA, and Received()
// shows exactly what was sent.
//
// The two scan-edge addresses cannot be cleared at all, and a clear aimed at
// either answers FAIL. WHERE THAT WAS READ, precisely: the two addresses
// themselves are from the ①, ② legend on PDF page 17, transcribed in
// ic7300mk2-transcription-b.csv's first row as "01 00: Programmed scan edge P1
// | 01 01: Programmed scan edge P2". NEITHER ARTEFACT PRINTS ANY CLEAR OR
// ERASE BEHAVIOUR WHATEVER — not for the scan edges, not for a memory channel,
// not anywhere; the only thing page 17 prints about P1 and P2 beyond their
// addresses is the ⓘ line under ③, "Set 00 for P1 and P2". So the scan-edge
// rule is not implemented as a special case reading a special rule: it falls
// out of the blanket refusal above, which refuses a clear aimed at any of the
// three address forms alike. That is stated here rather than dressed up as a
// derivation, because the register below is only worth having if it does not
// lie about what was read.
//
// # ASSUMED
//
// Every place the two artefacts did not determine the answer, what was chosen,
// and why. An entry here is a decision that could be wrong; the numbering is
// stable so that a later reading can cite one.
//
//  1. THE RECORD'S LENGTH: 47 printed indices, not 16 drawn cells.
//     The geometry witness counts sixteen byte cells actually drawn in D1 and
//     records "16 ≠ 47" as an unresolved STOP, because three of the nine
//     blocks are drawn abbreviated. ASSUMED: the printed index counts are the
//     byte widths. Why: the witness's own STOP 9 says the elision marks are
//     drawn 1, 3 and 1 cell pitches wide but stand for 3, 14 and 14 omitted
//     indices, "neither equal to nor proportional to the number of cells they
//     elide", so nothing can be recovered from the drawing; the transcription
//     independently records the printed counts in its width_bytes column; and
//     the ⑱ ~ ㉝ count of 16 is corroborated twice in prose, by "Up to 16
//     characters." on page 17 and "(up to 16 characters)" on page 18. The
//     alternative — believing the drawing — would make the memory name two
//     bytes long, which page 18 contradicts outright.
//
//  2. THE CHANNEL ADDRESS IS NOT PART OF THE RECORD. D1 draws ①, ② Memory
//     channel number as the first block of the same byte strip as the rest,
//     and nothing in either artefact separates them. ASSUMED: the two bytes
//     are the command's channel address, not record content, so the record is
//     45 bytes and both a read and a set carry the address ahead of it. Why:
//     a read frame is defined as carrying the two channel-address bytes and
//     nothing more, which only parses if those two bytes are the address; and
//     a record containing its own address would make an address mismatch
//     between frame and record expressible, which no radio would allow.
//
//  3. ❹ ~ ⓱ IS VALIDATED, BUT EQUALITY WITH ④ ~ ⑰ IS NOT REQUIRED. The block
//     has no printed label at all and an encoding of "unstated"; the only
//     printed statements about it are page 17's NOTE box: "The same data as
//     ④ ~ ⑰ are stored in ❹ ~ ⓱", "When the Split function is ON, the data of
//     ❹ ~ ⓱ is used for transmission", and "Even if the Split function is
//     OFF, we recommend that you set the same data as ④ ~ ⑰ into ❹ ~ ⓱".
//     ASSUMED: its fourteen bytes repeat ④ ~ ⑰'s layout and are checked
//     against the same per-field vocabularies, but a set whose transmit side
//     differs from its receive side is ACCEPTED. Why: "the same data … are
//     stored" fixes the layout, which is what makes the sub-offsets at 15, 20,
//     22, 23 and 26 derivable at all; but the third NOTE line only RECOMMENDS
//     equality, in those words, and a fake that enforced a recommendation
//     would refuse the split-frequency channel the second NOTE line describes.
//
//  4. ⑫ ~ ⑭ REPEATER TONE FREQUENCY ACCEPTS ANY THREE BYTES. This is the
//     entry most likely to be wrong, and it is deliberate. Page 17 prints the
//     heading ⑫ ~ ⑭ Repeater tone frequency setting and then NOTHING: the
//     transcription's notes say "no value, no code, no encoding statement and
//     no cross-reference", its values_verbatim is empty and its encoding is
//     "unstated". The single ⓘ cross-reference in that area — to page 23's
//     "Repeater tone/tone squelch frequency settings", a title which plainly
//     covers both fields — is set under the NEXT heading, ⑮ ~ ⑰, and the
//     transcription explicitly did not carry it across. ASSUMED: a field whose
//     printed vocabulary is empty constrains nothing, so any three bytes pass.
//     Why: the rule this fake applies is "every field's value is one the
//     transcription prints", and for this field the transcription prints none
//     — reading that as "reject everything" would make every set fail, and
//     reading it as "borrow ⑮ ~ ⑰'s BCD" would be making exactly the
//     inference the transcription refused to make, silently, inside a test
//     double. The likely truth is that these three bytes carry the same tone
//     BCD as ⑮ ~ ⑰ (first byte 00 fixed, then 100 Hz 0~2, 10 Hz, 1 Hz, 0.1 Hz
//     digits); if a real MK2 confirms it, tighten repeaterToneOK in record.go
//     — that one function, and this entry, are the whole of the change. The
//     same applies to the block's mirror at offset 23.
//
//  5. THE SPACE IN THE MEMORY NAME IS 0x20. Page 18's "ⓘUsable characters"
//     list names "(space)" but no code is printed for it in either character
//     table. ASSUMED: 0x20. Why: both tables head their code column "ASCII
//     code", every other code in them is that character's ASCII value, and
//     0x20 is the space's.
//
//  6. BOTH 27 AND 60 ARE ACCEPTED IN A MEMORY NAME. Page 18 draws an
//     identical right-single-quotation glyph against each of those two codes
//     (the transcription's STOP 2, checked at 900 % and found "pixel-for-pixel
//     the same shape"). ASSUMED: both codes are members of the vocabulary.
//     Why: the contradiction is about which GLYPH each code draws, and this
//     fake validates codes, not glyphs; dropping either code would refuse a
//     byte the manual prints.
//
//  7. A MEMORY NAME IS SIXTEEN BYTES FROM THE PRINTED VOCABULARY, WITH NO PAD
//     BYTE. No terminator, filler or pad value is printed anywhere. ASSUMED:
//     the field is fixed at sixteen bytes and every one of them must be a
//     printed code, so a shorter name is padded by the caller with the printed
//     space; 0x00 is refused. Why: "up to 16 characters" describes the name's
//     length, not the field's, and the field is drawn as a fixed span like
//     every other; accepting 0x00 would be inventing a terminator.
//
//  8. THE ⓘ "Set 00 for P1 and P2" LINE IS NOT ENFORCED. It is printed under
//     ③, beside a table that prints only single-nibble codes; the
//     transcription records the two separately and declines to synthesise a
//     two-digit value from them. ASSUMED: any value ③'s nibble tables print
//     is accepted at every address, scan edges included. Why: enforcing it
//     would mean deciding that the ⓘ line's "00" is the whole byte rather than
//     one of the two nibble columns, which is the synthesis the transcription
//     refused; and 00 is a legal ③ byte everywhere anyway, so a caller
//     following the manual is never refused.
//
//  9. THE FREQUENCY FIELD'S NIBBLE-TO-BYTE PACKING. Page 16 prints ten
//     rotated nibble labels beneath a five-byte row and the transcription
//     records them in the order they are read left to right. ASSUMED: that
//     printed order IS the byte order, giving byte 0 = 10 Hz : 1 Hz, byte 1 =
//     1 kHz : 100 Hz, byte 2 = 100 kHz : 10 kHz, byte 3 = 10 MHz : 1 MHz,
//     byte 4 = 1 GHz : 100 MHz, with the left-printed nibble as the high
//     nibble. Why: the geometry witness fixes left-printed = first for a
//     byte's two halves (its nibble 1 / nibble 2 convention, and both of page
//     17's expansion boxes), and the labels rise vertically to the nibble
//     above them with no crossing, which the transcription checked by eye at
//     600 dpi. The consequence is a least-significant-byte-first field, which
//     is worth noticing because it is not the order the labels' weights run
//     in.
//
//  10. THE 10 MHz DIGIT'S 0 ~ 7 AND BYTE 4'S FIXED ZEROES ARE ENFORCED. Page
//     16 prints "10 MHz digit: 0 ~ 7", "1 GHz digit: 0 (Fixed)" and "100 MHz
//     digit: 0 (Fixed)". ASSUMED: these are limits a set must respect, not
//     descriptions of what the radio happens to produce. Why: they are printed
//     in the same column, in the same form, as every other digit's range, and
//     a fake that ignored the ranges it was given would validate nothing.
//     Likewise ⑮ ~ ⑰'s first byte, whose two nibbles page 23 prints as
//     literal 0s rather than as X, is required to be 00 — the asterisk on
//     those two labels leads to "* Not necessary when setting a frequency",
//     which is about the tone commands 1B 00 / 1B 01 and not about this
//     fixed-width record.
//
//  11. MODE CODE 06 IS REFUSED. Page 16's operating-mode table prints 00 to 05,
//     then 07 and 08, and draws its two remaining cells as "—". ASSUMED: 06 is
//     not a value, rather than a printing slip. Why: the table prints the
//     absence explicitly, with a dash in a cell rather than a gap.
//
//  12. THE IDENTITY ANSWER'S DEFAULT TOKEN IS THE RADIO'S OWN ADDRESS. The
//     value is undocumented for this radio. ASSUMED: one byte, equal to the
//     configured CI-V address, unless WithIDToken says otherwise. Why: it is
//     the only value the fake can produce that is guaranteed to stay
//     self-consistent when WithRadioAddress moves the address, and a caller
//     who cares about the token is expected to fix it.
//
//  13. THE RECORD ANSWER ECHOES THE CHANNEL ADDRESS. Neither artefact prints
//     the shape of an ANSWER frame at all — they describe the record, not the
//     exchange. ASSUMED: a read answers 1A 00 <ch hi> <ch lo> <record>, with
//     the two address bytes repeated ahead of the record. Why: without them
//     the answer says which record but not which channel, and a controller
//     with two reads in flight could not tell them apart.
//
//  14. AN ADDRESSED FRAME CARRYING NO COMMAND BYTE ANSWERS FAIL. FE FE B6 E0 FD
//     is well formed by the grammar and means nothing. ASSUMED: FAIL rather
//     than silence. Why: it is addressed to this radio, and this radio
//     acknowledges everything addressed to it; silence would be
//     indistinguishable from a frame that never arrived.
//
//  15. A LONE FE FOLLOWED BY DATA IS NOISE, NOT A FRAME. The grammar's preamble
//     is two bytes and any number beyond two is tolerated. ASSUMED: fewer than
//     two is not a frame opening, and the reassembler drops back to hunting
//     for a preamble. Why: FE is reserved, so a single one in a noisy stream
//     is far more likely to be noise than a truncated preamble, and accepting
//     it would let a corrupt stream synthesise frames.
//
//  16. Received() RECORDS NORMALISED FRAMES. Leading noise is dropped and a run
//     of three or more preamble bytes is recorded as two. ASSUMED: that is
//     what a caller asserting "this frame never reached the radio" wants to
//     compare against. The frames are complete: two FE, the body as received,
//     one FD.
//
//  17. Sent() IS THE WIRE, NOT ONLY THE ANSWERS. It records everything the
//     radio wrote, in order — answers, and also the echoes of WithEcho and the
//     unsolicited frames of WithTransceiveBroadcasts and WithAddressedFlood.
//     ASSUMED: a caller wants to see what the port saw. Why: with the default
//     options none of those three is on and the two readings coincide, so the
//     difference only appears where a caller asked for the extra traffic. A
//     frame whose write was abandoned is not recorded, because it never
//     arrived.
//
//  18. WithChannel CHECKS THE ADDRESS AND THE LENGTH, BUT NOT THE VALUES. A
//     seeded record is stored verbatim, so a slot may hold a mode byte or a
//     name byte the manual never prints, and a read will answer it. ASSUMED:
//     that seam is wanted. Why: it is how a test drives the READING side's
//     error paths through a real fake instead of a scripted transcript, and it
//     costs nothing, since a set arriving over the wire is still checked field
//     by field. WithRawChannel drops the length check too, and keeps the
//     address check because a slot at an unreadable address could never be
//     read back.
//
//  19. THE UNSOLICITED FRAMES' CONTENT. Nothing in either artefact describes an
//     unsolicited frame. ASSUMED: command 00 carrying five frequency bytes in
//     the packing of entry 9, fixed at 14.250000 MHz (00 00 25 14 00). Why:
//     the flood has to carry something, a well-formed frequency is what a
//     transceive-enabled Icom emits, and a constant payload is one a test can
//     recognise by sight.
//
//  20. UNSOLICITED FRAMES ARE DROPPED IF THE PEER IS NOT READING; ANSWERS ARE
//     NOT. Each unsolicited write gets a short deadline (its own period,
//     capped at 50 ms) and is abandoned if it expires; an answer blocks until
//     the peer reads it or the radio closes. ASSUMED: that asymmetry. Why: the
//     flood options exist to prove a flood does not WEDGE anything, so the
//     flood itself must not be able to park the write lock with an answer
//     queued behind it; whereas an answer that silently evaporated would make
//     the fake far harder to reason about than one that waits.
//
//  21. THE REASSEMBLER'S ACCUMULATOR IS CAPPED AT 4096 BYTES. Past the cap the
//     partial frame is abandoned and the reassembler resynchronises on the
//     next preamble. ASSUMED: a cap is wanted at all. Why: FE FE followed by
//     endless noise and no FD would otherwise grow a buffer without bound; the
//     longest frame this radio ever handles is a memory set at 54 bytes, so
//     the cap cannot bite on real traffic.
package fakeic7300mk2

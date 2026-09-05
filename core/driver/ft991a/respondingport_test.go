// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end PARSES the frames the driver writes and answers each one per a
// configurable slot image, recording every frame it received in order.
//
// It is the M9c-6 "COMMAND-PARSING RESPONDER" seam, inherited in SHAPE from
// core/driver/ft891's and rewritten for this radio's frames. It exists
// because a fixed-transcript stub cannot test this driver at all: every
// session begins with Open's choreography, and a test that wants to
// exercise one read has to answer those frames first.
//
// It is deliberately NOT internal/fakeft991a (which lane B builds): a fake
// radio models a radio's STATE and is the right tool for round-trip and
// end-to-end tests, whereas this answers per-frame from a table and can
// therefore serve deliberately WRONG answers — a foreign CAT ID, an answer
// naming the wrong slot, or SILENCE — which is exactly what the error paths
// need and what a self-consistent fake will never produce.
//
// WHAT IT KNOWS, and how to extend it: AI (any AI frame, answered with
// silence), ID;, the 6-byte MT READ, and the 41-byte combined MT SET. ANY
// OTHER frame is answered "?;", which is also how this file serves the
// negative pins: an MR frame, or an MT read of a slot outside 001-117, is a
// frame this driver must never build, and if one is ever built it appears
// in the transcript and is rejected rather than quietly answered.
//
// THE TWO MT LENGTHS ARE THE ONLY MT FRAMES ADMITTED, and an MT frame of
// any OTHER length falls through to "?;" rather than being taken for a Set.
// The FT-891's helper matches a bare "MT" prefix for its Set arm; this one
// does not, because a driver bug that built a short or long MT frame would
// be answered with silence there — i.e. read as ACCEPTED — where here it is
// rejected loudly.
//
// THE ACKNOWLEDGEMENT SEMANTICS OF THOSE ANSWERS ARE AN ASSUMED CONVENTION
// APPLIED, NOT AN OBSERVED RADIO TRANSCRIBED — no FT-991A has ever been
// connected to this project, so nothing here is evidence of what one does.
// SEMANTICS, narrowly: "?;"-means-rejected, which is the dialect register's
// entry THE ACKNOWLEDGEMENT CONVENTIONS. That "?;" is a NAK on this radio AT
// ALL — that a refusal is a frame rather than silence — is inherited with
// it. A capture, not this file, will settle either.
//
// The answers' SHAPES are a different grade of claim, and where this file
// pins one it cites the manual for it: the frame-length consts and the frame
// builder below quote revision 1711-D's own position charts by layout line.
//
// "?;" IS STILL THE RIGHT DEFAULT here, whatever a real radio turns out to
// do: a task that adds a command class and forgets to teach this helper
// about it sees its frame REJECTED, loudly and immediately, rather than
// silently succeeding.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// slotImage is what a respondingPort's radio "contains": the identity it
// answers with, the MT answers it serves per slot, and the one way it can
// decline to answer at all.
//
// A slot ABSENT from mtAnswers is answered "?;" — which this driver reads as
// "the slot is empty" (the register's MT "?;" ON A MEMORY OR PMS SLOT MEANS
// THE SLOT IS EMPTY entry). THAT IS THE ONLY MEANING THIS DRIVER GIVES A
// "?;", where the FT-891 gives four, because there is no discovery walk and
// no cross-check to give it another.
type slotImage struct {
	// catID is the four-character identity "ID;" answers with. Empty
	// selects the FT-991A's own, so the ordinary case needs no ceremony and
	// only a wrong-radio test says anything about it.
	catID string
	// mtAnswers maps a 3-byte slot wire form to the RAW answer frame served
	// for a combined MT read of it. Raw, not structured, so a test can serve
	// a malformed or contradictory frame on purpose.
	mtAnswers map[string]string
	// mtSilent names slots whose MT read draws NO REPLY AT ALL — the timeout
	// row of the read truth table (matrix §3.5), which no self-consistent
	// radio image can express and no "?;" can stand in for.
	mtSilent map[string]bool
	// junkBefore maps a slot to a frame served AHEAD of its MT answer: an
	// unexpected frame the engine must surface and count without failing the
	// read. Transport safety obligation 3.
	junkBefore map[string]string
	// rejectSets makes every combined MT Set answer "?;" instead of the
	// silence the ASSUMED convention reads as accepted — the write path's
	// radio-rejected row (the register's THE ACKNOWLEDGEMENT CONVENTIONS
	// entry, applied in its rejecting direction).
	rejectSets bool
	// junkAfterSet is a frame served in the error window AFTER an accepted
	// Set: neither the silence that means "accepted" nor the "?;" that
	// means "rejected", but a THIRD thing the radio might say. The engine
	// must count it (transport safety obligation 3) and still report the
	// write as accepted, because nothing rejected it. This is the
	// wrong-answer row of the write path, and no self-consistent fake can
	// produce it.
	junkAfterSet string
	// echoSets makes an ACCEPTED combined MT Set become that slot's MT
	// answer from then on, VERBATIM — available at all only because this
	// radio's MT Set and MT Answer share the SAME 41 positions (the one
	// chart at layout 998-1033 under one prefix), so the bytes the driver
	// wrote are already a well-formed answer.
	//
	// It is the narrowest possible memory, and it is here for ONE thing:
	// core/clone owns write-then-verify (plan P12), so that pair cannot be
	// exercised at all against a peer that answers the same way forever.
	// Deliberately NOT a step towards internal/fakeft991a — no field but
	// the slot is interpreted (positions 3-5, so the echo is per-slot at
	// all), nothing is validated, no state is modelled, and a Set this
	// driver got wrong would be echoed back just as wrongly. What it
	// demonstrates is that this driver's write and its read agree about
	// every position of the frame; whether a REAL FT-991A reports back what
	// it was told is not settleable by any test.
	echoSets bool
}

// newRespondingPort starts a scripted radio serving img and registers its
// cleanup. The returned value's Port is what a driver Opens.
func newRespondingPort(t *testing.T, img slotImage) *respondingPort {
	t.Helper()
	if img.catID == "" {
		img.catID = catDialect.CATID()
	}
	host, remote := net.Pipe()
	p := &respondingPort{host: host, remote: remote}
	t.Cleanup(func() {
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve(img)
	return p
}

// Port returns the end handed to the driver. The driver takes ownership of
// it (Open closes it on failure; Session.Close on success), so a test must
// not close it itself — newRespondingPort's cleanup covers the rest.
func (p *respondingPort) Port() transport.Port { return p.host }

// Transcript returns a copy of every complete frame the port has received,
// in arrival order.
func (p *respondingPort) Transcript() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.received...)
}

// serve reads the driver's bytes, splits them into ';'-terminated frames,
// records each, and writes back whatever img says.
//
// Frame splitting rather than whole-read matching: the transport writes one
// frame per call today, but nothing in the Port contract promises that, and
// a helper that assumed it would break confusingly the first time two frames
// shared a read.
func (p *respondingPort) serve(img slotImage) {
	buf := make([]byte, 256)
	var acc []byte
	// mtWritten holds the Sets echoSets has accepted, per slot. It is local
	// to this one goroutine — serve is the sole reader and sole writer of
	// the pipe's remote end, so nothing else touches it and no lock is
	// needed.
	mtWritten := map[string]string{}
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				i := bytes.IndexByte(acc, ';')
				if i < 0 {
					break
				}
				frame := string(acc[:i+1])
				acc = acc[i+1:]
				p.record(frame)
				if reply := img.reply(frame, mtWritten); reply != "" {
					if _, werr := p.remote.Write([]byte(reply)); werr != nil {
						return
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// record appends one received frame to the transcript.
func (p *respondingPort) record(frame string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, frame)
}

// reply returns the bytes this image answers frame with, or "" for silence.
// See respondingPort's doc comment for the command classes, the register
// entry that holds the convention, and why the default is a NAK.
//
// SILENCE HAS TWO MEANINGS HERE and they are not confusable in practice: a
// fire-and-forget Set's silence is the ASSUMED SUCCESS signal (the shared
// register's THE ACKNOWLEDGEMENT CONVENTIONS entry), while a READ's silence
// (mtSilent) is a radio that did not answer at all, which the engine turns
// into a timeout. Only the second is a fault being scripted; the first is
// the ordinary accepted write.
//
// mtWritten is serve's per-slot memory of the Sets echoSets has accepted.
func (img slotImage) reply(frame string, mtWritten map[string]string) string {
	switch {
	case frame == "ID;":
		return "ID" + img.catID + ";"
	case strings.HasPrefix(frame, "AI"):
		return ""
	case strings.HasPrefix(frame, "MT") && len(frame) == mtReadFrameLen:
		slot := frame[2:5]
		if img.mtSilent[slot] {
			return ""
		}
		// An echoed Set takes priority over the static image: it is the
		// LATER statement about the same slot. See echoSets.
		if ans, ok := mtWritten[slot]; ok {
			return ans
		}
		ans, ok := img.mtAnswers[slot]
		if !ok {
			return "?;"
		}
		return img.junkBefore[slot] + ans
	case strings.HasPrefix(frame, "MT") && len(frame) == mtAnswerLen:
		// A combined MT SET — the ONE frame this driver's write path
		// builds, and the same 41 positions as the Answer above (layout
		// 998-1033). Fire-and-forget on the ASSUMED convention, so silence
		// is what an accepted Set draws.
		if img.rejectSets {
			return "?;"
		}
		if img.echoSets {
			mtWritten[frame[2:5]] = frame
		}
		return img.junkAfterSet
	default:
		return "?;"
	}
}

// mtReadFrameLen is the length of this radio's MT read frame: "MT" + a
// 3-byte slot + ';' (the Read chart "M T P0 P0 P0 ;", layout 1018).
//
// Written out here because a TEST fixture that derived its frame shapes from
// the code under test would answer whatever that code asked for, including a
// wrong shape.
const mtReadFrameLen = 6

// mtAnswerLen is the length of this radio's combined MT answer — AND of its
// combined MT SET, which is the same chart under the same prefix, so this
// one const is what tells a Set apart from a read in reply below: 41 bytes,
// "MT" + the 28-position field block + P11 at 28 + a 12-byte P12 tag at
// 29-40 + ';' at 41 (the Answer chart, layout 1019-1033).
//
// Evidence leg G counted that grid TWICE at 600 dpi — once left-to-right
// through the printed position numbers and once by row totals — and both
// counts gave 41 (testdata/mt-vectors.golden's header). That the radio
// actually ANSWERS at the full width is the dialect register's THE COMBINED
// MT ANSWER'S EXACT LENGTH, 41 entry; what this const states is the printed
// chart, which is the thing that entry doubts.
const mtAnswerLen = 41

// memoryFields is the 28-position field block every FT-991A memory-bearing
// frame carries, in WIRE bytes, field by field, so a test states what the
// radio says rather than what a builder would produce for it.
//
// P5 IS A FIELD OF THIS STRUCT AND ITS FT-891 COUNTERPART HAS NONE, which is
// the first half of matrix erratum M-E3: `P5 0: TX CLAR "OFF" 1: TX CLAR
// "ON"` is printed on every FT-991A block carrying the grid — MR 971, MT
// 1004, MW 1042, IF 787, OI 1122 — where the FT-891 prints `P5 0: (Fixed)`
// and its own helper carries a p5Fixed const instead. A test on this radio
// can and must be able to vary byte 21.
//
// The zero value is not a valid block: every field is set explicitly at each
// call site, which is the point — see mtFrame.
type memoryFields struct {
	slot     string // P1, positions 3-5
	freq     string // P2, positions 6-14, 9 digits
	clarSign byte   // P3 sign, position 15
	clarMag  string // P3 magnitude, positions 16-19
	rxClar   byte   // P4, position 20
	txClar   byte   // P5, position 21 — LIVE on this radio
	mode     byte   // P6, position 22
	kind     byte   // P7, position 23
	ctcss    byte   // P8, position 24
	shift    byte   // P10, position 27
}

// p11Fixed is byte 28 of the combined record, and on this radio it is
// SCHEMA: MT's P11 legend reads `P11 0: (Fixed)` (layout 1015), where the
// FT-891 prints `0: TAG "OFF" 1: TAG "ON"` and its helper takes the flag as
// a parameter. That is the second half of erratum M-E3, and it is why
// mtFrame below has no display argument at all.
//
// A test that needs a NON-'0' byte there — the driver register's THE
// PRINTED-FIXED BYTES ARE ANSWERED AS PRINTED entry, whose refutation is a
// radio that answers something else — edits the byte of a built frame
// explicitly, so the deviation is visible at the call site.
const p11Fixed = '0'

// writeBlock fills frame's 28-position field block from f, BY POSITION, from
// the manual's own charts (MR's Answer at layout 965-981, MT's Set/Answer at
// 998-1033 — the same block under a different prefix). It writes offsets
// 2-26 and nothing else; the caller owns the prefix and whatever its form
// puts after the block.
//
// DELIBERATELY NOT cat.Dialect.BuildMTSetCombined. A fixture built by the
// builder under test would pin the parser against the builder — the two
// would agree about a wrong offset just as happily as a right one — and
// would additionally refuse the malformed frames these tests need.
func (f memoryFields) writeBlock(frame []byte) {
	copy(frame[2:5], f.slot)
	copy(frame[5:14], f.freq)
	frame[14] = f.clarSign
	copy(frame[15:19], f.clarMag)
	frame[19] = f.rxClar
	frame[20] = f.txClar
	frame[21] = f.mode
	frame[22] = f.kind
	frame[23] = f.ctcss
	copy(frame[24:26], "00") // P9, positions 25-26, documented fixed "00"
	frame[26] = f.shift
}

// mtFrame assembles a 41-byte combined MT ANSWER: "MT" + the 28-position
// field block + P11 at 28 + a 12-byte P12 tag field at 29-40 + ';' at 41.
//
// There is NO display argument, and that is this radio's legend rather than
// an omission — see p11Fixed.
//
// The tag field is padded with the DIALECT's ASSUMED TagFill (its own
// register entry, MTPolicy.TagFill = ' '), which is what the driver must
// trim back off.
func (f memoryFields) mtFrame(tag string) string {
	b := sentinelFrame(mtAnswerLen)
	copy(b[0:2], "MT")
	f.writeBlock(b)
	b[27] = p11Fixed
	tagField := b[28:40]
	n := copy(tagField, tag)
	for i := n; i < len(tagField); i++ {
		tagField[i] = ' '
	}
	b[40] = ';'
	return string(b)
}

// sentinelFrame returns an n-byte buffer filled with a VISIBLE sentinel, so
// a position a builder forgets to fill shows up as a parse failure naming
// the field rather than as an accidental zero that happens to be valid
// somewhere.
func sentinelFrame(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = '?'
	}
	return b
}

// populatedFields is a valid, unremarkable channel for slot: 145.500 MHz FM,
// clarifier -150 Hz with BOTH clarifier flags on, CTCSS ENC-DEC, PLUS shift.
//
// 145.5 MHz DELIBERATELY, and it is a fixture that would be out of range on
// the FT-891: this is the first registered Yaesu with VHF/UHF (MaxFreqHz
// 470 MHz against that radio's 56 MHz), and a cross-model fixture reused
// without thought fails loudly in only one of the two directions.
//
// TxClar is '1' DELIBERATELY too — the FT-891's own fixture cannot set that
// byte at all, and a driver that carried its TxClar refusal across would
// refuse this perfectly ordinary channel (erratum M-E3).
//
// Every value is inside this dialect's declared vocabularies (mode '4' of
// the fourteen-name legend, kind '1' Memory, a clarifier magnitude that is a
// multiple of the dialect's ASSUMED 10 Hz step).
//
// Tests that care about the mapping spell the expectation out
// independently; tests that only need "this slot exists" use this.
func populatedFields(slot string) memoryFields {
	return memoryFields{
		slot: slot, freq: "145500000",
		clarSign: '-', clarMag: "0150", rxClar: '1', txClar: '1',
		mode: '4', kind: '1', ctcss: '1', shift: '1',
	}
}

// populatedMT is populatedFields' combined MT answer with the tag "CALLING".
func populatedMT(slot string) string { return populatedFields(slot).mtFrame("CALLING") }

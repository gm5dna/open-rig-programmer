// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

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
// core/driver/ftdx10's and core/driver/ftdx101's and rewritten for this
// radio's frames. It exists because a fixed-transcript stub cannot test
// this driver at all: every session begins with Open's full choreography —
// the AI0 init, the ID probe, then eleven MR discovery probes — so a test
// that wants to exercise one read has to answer a dozen frames first, in
// whatever order the driver chooses to send them.
//
// It is deliberately NOT internal/fakeft891 (which lane B builds): a fake
// radio models a radio's STATE and is the right tool for round-trip and
// end-to-end tests, whereas this answers per-frame from a table and can
// therefore serve deliberately WRONG answers — a foreign CAT ID, an answer
// naming the wrong slot, an MT rejection over a slot MR reports as
// occupied, or SILENCE — which is exactly what the error paths need and
// what a self-consistent fake will never produce.
//
// WHAT IT KNOWS, and how to extend it: AI (any AI frame, answered with
// silence), ID;, the 6-byte MT READ, the 6-byte MR READ, the longer
// combined MT SET, and the 7-byte EX read. ANY OTHER frame is answered
// "?;".
//
// THE ACKNOWLEDGEMENT SEMANTICS OF THOSE ANSWERS ARE AN ASSUMED CONVENTION
// APPLIED, NOT AN OBSERVED RADIO TRANSCRIBED — no FT-891 has ever been
// connected to this project, so nothing here is evidence of what one does.
// SEMANTICS, narrowly: silence-means-accepted and "?;"-means-rejected, which
// is doc.go's ASSUMED-register entry THE ACKNOWLEDGEMENT CONVENTIONS (and
// the second half of A SINGLE COMBINED MT SET SUFFICES...). That "?;" is a
// NAK on this radio AT ALL — that a refusal is a frame rather than silence
// — is inherited with it: this manual prints no '?' character anywhere in
// its layout extraction. A capture, not this file, will settle either.
//
// The answers' SHAPES are a different grade of claim, and where this file
// pins one it cites the manual for it — the frame-length consts and the two
// frame builders below quote rev 1909-C's own position charts by layout
// line.
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
// answers with, the MT and MR answers it serves per slot, the EX answers it
// serves per address, and the two ways it can decline to answer at all.
//
// A slot ABSENT from mrAnswers is answered "?;" — which this driver reads
// as "absent from this radio" during discovery (the ASSUMED register's "?;"
// ON A 5xx/EMG DISCOVERY PROBE entry) and, as the cross-check's second
// frame, as "the slot is empty" (its "?;" ON AN MR READ OF A MEMORY OR PMS
// SLOT entry). The two share one mechanism here for the same reason they
// would share one on the wire IF THE CONVENTION HOLDS: a radio whose only
// refusal is an unattributed "?;" has no way to distinguish them.
type slotImage struct {
	// catID is the four-character identity "ID;" answers with. Empty
	// selects the FT-891's own, so the ordinary case needs no ceremony and
	// only a wrong-radio test says anything about it.
	catID string
	// mtAnswers maps a 3-byte slot wire form to the RAW answer frame served
	// for a combined MT read of it. Raw, not structured, so a test can
	// serve a malformed or contradictory frame on purpose.
	mtAnswers map[string]string
	// mrAnswers maps a 3-byte slot wire form to the RAW answer frame served
	// for an MR read of it. Discovery consults exactly this map, and so
	// does the MEM/PMS cross-check's second frame.
	mrAnswers map[string]string
	// mrAnswersOnce maps a 3-byte slot wire form to the RAW answer frame
	// served for the FIRST MR read of it in the session — ordinarily
	// discovery's own probe — and "?;" for every MR read of that slot
	// after the first. It models a discovered slot that STOPS answering
	// within one session: MEDIUM-1 (task-1 review)'s
	// MRReadRejectedForDiscoveredSlotError exists for exactly this shape,
	// which a static mrAnswers entry cannot express (it would answer the
	// same way forever) and no self-consistent fake radio would ever
	// produce on its own. Takes priority over mrAnswers for a slot present
	// in both.
	mrAnswersOnce map[string]string
	// mtSilent names slots whose MT read draws NO REPLY AT ALL — the
	// timeout row of the read truth table (matrix §3.5), which no
	// self-consistent radio image can express and no "?;" can stand in
	// for.
	mtSilent map[string]bool
	// exAnswers maps a 4-digit EX address to the raw answer frame served
	// for a read of it. Task 3's surface; nil serves "?;" for every
	// address.
	exAnswers map[string]string
	// rejectSets makes every combined MT Set answer "?;" instead of the
	// silence the ASSUMED convention reads as accepted — the write path's
	// radio-rejected row.
	rejectSets bool
	// echoSets makes an ACCEPTED combined MT Set become that slot's MT
	// answer from then on, verbatim — which is available at all only
	// because this radio's MT Set and MT Answer share the SAME 41
	// positions (layout 996-1027, one chart under one prefix), so the
	// bytes the driver wrote are already a well-formed answer.
	//
	// It is the narrowest possible memory, and it is here for ONE thing:
	// core/clone owns write-then-verify (plan P3), so the pair cannot be
	// exercised at all against a peer that answers the same way forever.
	// This is deliberately NOT a step towards a fake radio — no field but
	// the slot is interpreted (positions 3-5, so the echo is per-slot at
	// all), nothing is validated, no state is modelled, and a Set this
	// driver got wrong would be echoed back just as wrongly. What it
	// demonstrates is that the write and the read agree about every
	// position of the frame; whether a REAL FT-891 reports back what it
	// was told is the driver register's A SINGLE COMBINED MT SET SUFFICES
	// entry and no test can settle it.
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
// a helper that assumed it would break confusingly the first time two
// frames shared a read.
func (p *respondingPort) serve(img slotImage) {
	buf := make([]byte, 256)
	var acc []byte
	// mrSeen counts MR reads per slot, for mrAnswersOnce's "first read
	// only" behaviour; mtWritten holds the Sets echoSets has accepted, per
	// slot. Both are local to this one goroutine — serve is the sole
	// reader and sole writer of the pipe's remote end, so nothing else
	// touches them and no lock is needed.
	mrSeen := map[string]int{}
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
				if reply := img.reply(frame, mrSeen, mtWritten); reply != "" {
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
// mrSeen is the per-slot MR-read counter mrAnswersOnce consults, owned and
// mutated here rather than on img: img is a value receiver and its maps are
// the fixed script, while mrSeen is this ONE serve loop's running state.
// See respondingPort's doc comment for the command classes, the register
// entries that hold the convention, and why the default is a NAK.
//
// SILENCE HAS TWO MEANINGS HERE and they are not confusable in practice: a
// fire-and-forget Set's silence is the ASSUMED success signal, while an MT
// read's silence (mtSilent) is a radio that did not answer a read at all,
// which the engine turns into a timeout. Only the second is a fault being
// scripted.
func (img slotImage) reply(frame string, mrSeen map[string]int, mtWritten map[string]string) string {
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
		if ans, ok := img.mtAnswers[slot]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MR") && len(frame) == mrReadFrameLen:
		slot := frame[2:5]
		mrSeen[slot]++
		if ans, ok := img.mrAnswersOnce[slot]; ok {
			if mrSeen[slot] == 1 {
				return ans
			}
			return "?;"
		}
		if ans, ok := img.mrAnswers[slot]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MT"):
		// A combined MT Set: fire-and-forget on the ASSUMED convention, so
		// silence is what this image serves for accepted.
		if img.rejectSets {
			return "?;"
		}
		if img.echoSets && len(frame) >= 5 {
			// Verbatim, prefix and all: the Set and the Answer are the
			// same 41 positions on this radio. See echoSets. The length
			// guard is defensive only — no test sends anything shorter
			// than the 41-byte Set today, and this driver never will —
			// so a future MT-shaped frame reaches "?;" below rather than
			// panicking this serve goroutine on frame[2:5].
			mtWritten[frame[2:5]] = frame
		}
		return ""
	case strings.HasPrefix(frame, "EX") && len(frame) == exReadFrameLen:
		if ans, ok := img.exAnswers[frame[2:6]]; ok {
			return ans
		}
		return "?;"
	default:
		return "?;"
	}
}

// Frame lengths this helper matches on, from the FT-891 CAT Operation
// Reference Book's own frame charts (rev 1909-C): the MT read is "MT" + a
// 3-byte slot + ';' (layout 1016), the MR read is "MR" + a 3-byte slot +
// ';' (965), and the EX read is "EX" + a FOUR-digit address + ';' (513-522)
// — SEVEN bytes, not the siblings' nine, which is the one shared-frame
// length that moves on this radio. Written out here because a TEST fixture
// that derived its frame shapes from the code under test would answer
// whatever that code asked for, including a wrong shape.
const (
	mtReadFrameLen = 6
	mrReadFrameLen = 6
	exReadFrameLen = 7
)

// memoryFields is the 28-position field block every FT-891 memory-bearing
// frame carries, in WIRE bytes, field by field, so a test states what the
// radio says rather than what a builder would produce for it.
//
// The zero value is not a valid block: every field is set explicitly at
// each call site, which is the point — see mrFrame and mtFrame.
type memoryFields struct {
	slot     string // P1, positions 3-5
	freq     string // P2, positions 6-14, 9 digits
	clarSign byte   // P3 sign, position 15
	clarMag  string // P3 magnitude, positions 16-19
	rxClar   byte   // P4, position 20
	mode     byte   // P6, position 22
	kind     byte   // P7, position 23
	ctcss    byte   // P8, position 24
	shift    byte   // P10, position 27
}

// P5, position 21, is NOT a field of the struct above, and that is this
// radio's own legend: `P5 0: (Fixed)` is printed on every FT-891 block
// carrying the 28-position grid — MR 971, MT 1006, MW 1042, IF 783, OI 1129
// — where the three registered siblings print `0: TX CLAR "OFF" 1: TX CLAR
// "ON"`. So a well-formed answer's byte 21 is always '0' and a test cannot
// vary it by accident.
//
// A test that needs a NON-'0' byte there — the ASSUMED register's P5 IS
// ANSWERED '0' entry, whose refutation is a radio that answers something
// else — edits the byte of a built frame explicitly, so the deviation is
// visible at the call site.
const p5Fixed = '0'

// writeBlock fills frame's 28-position field block from f, BY POSITION,
// from the manual's own charts (MR's answer at layout 968-975, MT's
// Set/Answer at 996-1027 — the same block under a different prefix). It
// writes offsets 2-26 and nothing else; the caller owns the prefix and
// whatever its form puts after the block.
//
// DELIBERATELY NOT cat.Dialect.BuildMWSet or BuildMTSetCombinedDisplay. A
// fixture built by the builder under test would pin the parser against the
// builder — the two would agree about a wrong offset just as happily as a
// right one — and would additionally refuse the malformed frames these
// tests need.
func (f memoryFields) writeBlock(frame []byte) {
	copy(frame[2:5], f.slot)
	copy(frame[5:14], f.freq)
	frame[14] = f.clarSign
	copy(frame[15:19], f.clarMag)
	frame[19] = f.rxClar
	frame[20] = p5Fixed
	frame[21] = f.mode
	frame[22] = f.kind
	frame[23] = f.ctcss
	copy(frame[24:26], "00") // P9, positions 25-26, documented fixed "00"
	frame[26] = f.shift
}

// mrFrame assembles a 28-byte MR ANSWER: "MR" + the field block + ';'
// (layout 968-975; the geometry witness counted "2 MR Answer frames (28
// bytes)").
//
// The literal 28 belongs in this file for the same reason the driver's own
// mrSpec has to carry one: here it is the CHART being asserted.
func (f memoryFields) mrFrame() string {
	b := sentinelFrame(28)
	copy(b[0:2], "MR")
	f.writeBlock(b)
	b[27] = ';'
	return string(b)
}

// mtFrame assembles a 41-byte combined MT ANSWER: "MT" + the 28-position
// field block + P11 at 28 + a 12-byte P12 tag field at 29-40 + ';' at 41
// (layout 996-1027; the geometry witness's "Counted frame length: 41 bytes,
// terminator included", counted twice off 300 dpi renders).
//
// display is P11, THE LIVE TAG FLAG this radio has and its combined-form
// siblings do not (`P11 0: TAG "OFF" 1: TAG "ON"`, layout 1016).
//
// The tag field is padded with the DIALECT's ASSUMED TagFill (its own
// register entry, MTPolicy.TagFill = ' '), which is what the driver must
// trim back off.
func (f memoryFields) mtFrame(display bool, tag string) string {
	b := sentinelFrame(41)
	copy(b[0:2], "MT")
	f.writeBlock(b)
	b[27] = '0'
	if display {
		b[27] = '1'
	}
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

// populatedFields is a valid, unremarkable channel for slot: 14.250 MHz
// USB, clarifier -150 Hz with RX clarifier on, CTCSS ENC-DEC, PLUS shift.
// Every value is inside this dialect's declared vocabularies (mode '2' of
// the manual's twelve-name legend, kind '1' Memory, a clarifier magnitude
// that is a multiple of the dialect's ASSUMED 10 Hz step).
//
// Tests that care about the mapping spell the expectation out
// independently; tests that only need "this slot exists" use this.
func populatedFields(slot string) memoryFields {
	return memoryFields{
		slot: slot, freq: "014250000",
		clarSign: '-', clarMag: "0150", rxClar: '1',
		mode: '2', kind: '1', ctcss: '1', shift: '1',
	}
}

// populatedMR is populatedFields' MR answer, the shape discovery reads.
func populatedMR(slot string) string { return populatedFields(slot).mrFrame() }

// populatedMT is populatedFields' combined MT answer with the TAG flag ON
// and the tag "CALLING".
func populatedMT(slot string) string { return populatedFields(slot).mtFrame(true, "CALLING") }

// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

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
// It is the M9c-6 plan's "COMMAND-PARSING RESPONDER, shared" seam, and
// tasks 2 (write) and 3 (settings) reuse it. It exists because a
// fixed-transcript stub CANNOT test this driver at all: every session
// begins with Open's full choreography — the AI0 init, the ID probe, and
// then ~100 discovery probes across the whole declared 5xx range plus EMG
// — so a test that wants to exercise one read has to answer a hundred
// frames first, in whatever order the driver chooses to send them. Scripting
// that by hand would encode the choreography into every fixture and make
// the discovery test (which asserts the choreography) tautological.
//
// It is deliberately NOT internal/fakedx10 (which task 4 builds): a fake
// radio models a radio's STATE and is the right tool for round-trip and
// end-to-end tests, whereas this answers per-frame from a table and can
// therefore serve deliberately WRONG answers — a foreign CAT ID, an
// out-of-vocabulary kind byte, an answer naming the wrong slot — which is
// exactly what the error paths need and what a self-consistent fake will
// never produce.
//
// WHAT IT KNOWS, and how to extend it: AI (any AI frame, answered with
// silence), ID; (the image's catID), the 6-byte combined MT READ, the
// longer combined MT SET, and the 9-byte EX read. ANY OTHER frame is
// answered "?;".
//
// THOSE ANSWERS ARE AN ASSUMED CONVENTION APPLIED, NOT AN OBSERVED RADIO
// TRANSCRIBED — no real FTdx10 has ever been connected to this project, so
// nothing here is evidence of what one does. Two register entries hold the
// two halves. That an accepted Set draws NO reply at all while a rejected
// one draws exactly one "?;" is doc.go's ASSUMED-register entry "A SINGLE
// COMBINED MT SET SUFFICES", second half. That entry covers MT only, and
// says so: it records that this driver's Open states nothing about its AI0
// frame's acknowledgement, unlike the FTdx101's. The AI arm above is
// therefore modelled on internal/fakedx10/doc.go's entry 11, which extends
// the same convention to AI-set.
//
// That "?;" is a NAK on this radio AT ALL — that a refusal is a frame
// rather than silence — is internal/fakedx10/doc.go's entry 18: the
// convention is inherited from the FT-710's reference, and rev 2308-F of
// this radio's manual prints no '?' character anywhere in its 1,927 lines.
// Both halves are FT-710 conventions carried across, and a capture, not
// this file, will settle them.
//
// "?;" IS STILL THE RIGHT DEFAULT here, whatever a real radio turns out to
// do: a task that adds a command class and forgets to teach this helper
// about it sees its Set REJECTED, loudly and immediately, rather than
// silently succeeding.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// slotImage is what a respondingPort's radio "contains": the identity it
// answers with, the combined-MT answers it serves per slot, the EX answers
// it serves per address, and whether it rejects Sets.
//
// A slot ABSENT from mtAnswers is answered "?;" — which this driver reads
// as "absent from this radio" during discovery (the ASSUMED register's
// "?;" ON A 5xx/EMG DISCOVERY PROBE entry) and as "empty channel" during a
// read (its "?;" ON A COMBINED-MT READ OF AN EMPTY SLOT entry). The two
// share one mechanism here for the same reason they would share one on the
// wire IF THE CONVENTION HOLDS: a radio whose only refusal is an
// unattributed "?;" (internal/fakedx10/doc.go's entry 18) has no way to
// distinguish them.
type slotImage struct {
	// catID is the four-character identity "ID;" answers with. Empty
	// selects the FTdx10's own, so the ordinary case needs no ceremony and
	// only a wrong-radio test says anything about it.
	catID string
	// mtAnswers maps a 3-byte slot wire form to the RAW answer frame
	// served for a combined MT read of it. Raw, not structured, so a test
	// can serve a malformed or contradictory frame on purpose.
	mtAnswers map[string]string
	// exAnswers maps a 6-digit EX address to the raw answer frame served
	// for a read of it. Task 3's surface; nil serves "?;" for every
	// address.
	exAnswers map[string]string
	// rejectSets makes every combined MT Set answer "?;" instead of the
	// silence the ASSUMED convention reads as accepted (doc.go's register
	// entry "A SINGLE COMBINED MT SET SUFFICES", second half). Task 2's
	// rejection path.
	rejectSets bool
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
				if reply := img.reply(frame); reply != "" {
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

// reply returns the bytes this image answers frame with, or "" for silence
// (the ASSUMED convention's fire-and-forget success signal). See
// respondingPort's doc comment for the command classes, the register
// entries that hold the convention, and why the default is a NAK.
func (img slotImage) reply(frame string) string {
	switch {
	case frame == "ID;":
		return "ID" + img.catID + ";"
	case strings.HasPrefix(frame, "AI"):
		return ""
	case strings.HasPrefix(frame, "MT") && len(frame) == mtReadFrameLen:
		if ans, ok := img.mtAnswers[frame[2:5]]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MT"):
		// A combined MT Set: fire-and-forget on the ASSUMED convention, so
		// silence is what this image serves for accepted.
		if img.rejectSets {
			return "?;"
		}
		return ""
	case strings.HasPrefix(frame, "EX") && len(frame) == exReadFrameLen:
		if ans, ok := img.exAnswers[frame[2:8]]; ok {
			return ans
		}
		return "?;"
	default:
		return "?;"
	}
}

// Frame lengths this helper matches on, from the FTdx10 CAT manual's own
// position charts (rev 2308-F): the MT read is "MT" + a 3-byte slot + ';'
// (manual lines 1217-1256, and core/cat's mtReadLen), the EX read is "EX"
// + a 6-digit address + ';' (manual lines 636-645, core/cat's exReadLen).
// Written out here because a TEST fixture that derived its frame shapes
// from the code under test would answer whatever that code asked for,
// including a wrong shape.
const (
	mtReadFrameLen = 6
	exReadFrameLen = 9
)

// mtAnswerFields is one combined-MT answer's content in WIRE bytes, field
// by field, so a test states what the radio says rather than what a
// builder would produce for it.
//
// The zero value is not a valid answer: every field is set explicitly at
// each call site, which is the point — see frame.
type mtAnswerFields struct {
	slot     string // P1, positions 3-5
	freq     string // P2, positions 6-14, 9 digits
	clarSign byte   // P3 sign, position 15
	clarMag  string // P3 magnitude, positions 16-19
	rxClar   byte   // P4, position 20
	txClar   byte   // P5, position 21
	mode     byte   // P6, position 22
	kind     byte   // P7, position 23
	ctcss    byte   // P8, position 24
	shift    byte   // P10, position 27
	p11      byte   // P11, position 28; the zero byte means "the fixed '0'"
	tag      string // P12, positions 29-40, space-padded to 12
}

// frame assembles the 41-byte combined MT answer BY POSITION, from the CAT
// manual's own MT chart (rev 2308-F, manual lines 1217-1256): "MT" + the
// 28-position shared field block + P11 + a 12-byte tag field + ';'.
//
// DELIBERATELY NOT cat.Dialect.BuildMTSetCombined. A fixture built by the
// builder under test would pin the parser against the builder — the two
// would agree about a wrong offset just as happily as a right one — and
// would additionally refuse the malformed frames these tests need
// (BuildMTSetCombined validates its input, which is its job and exactly
// what makes it useless here).
//
// The literal 41 belongs in this file for the same reason it must not
// appear in the production code: here it is the CHART being asserted;
// there it would be a length the driver believed instead of one the
// dialect derived.
func (f mtAnswerFields) frame() string {
	b := make([]byte, 41)
	for i := range b {
		// A visible sentinel, so a position this builder forgets to fill
		// shows up as a parse failure naming the field rather than as an
		// accidental zero that happens to be valid somewhere.
		b[i] = '?'
	}
	copy(b[0:2], "MT")
	copy(b[2:5], f.slot)
	copy(b[5:14], f.freq)
	b[14] = f.clarSign
	copy(b[15:19], f.clarMag)
	b[19] = f.rxClar
	b[20] = f.txClar
	b[21] = f.mode
	b[22] = f.kind
	b[23] = f.ctcss
	copy(b[24:26], "00") // P9, positions 25-26, documented fixed "00"
	b[26] = f.shift
	p11 := f.p11
	if p11 == 0 {
		p11 = '0'
	}
	b[27] = p11
	tagField := b[28:40]
	n := copy(tagField, f.tag)
	for i := n; i < len(tagField); i++ {
		tagField[i] = ' '
	}
	b[40] = ';'
	return string(b)
}

// populatedAnswer is a valid, unremarkable combined-MT answer for slot: a
// 14.250 MHz USB memory channel, clarifier -150 Hz with RX clarifier on,
// CTCSS ENC-DEC, PLUS shift, tag "CALLING". Every value is inside this
// dialect's declared vocabularies (mode '2' of the manual's 1-F legend,
// kind '1' Memory of the combined record's {'0','1'} read pair, a
// clarifier magnitude that is a multiple of the dialect's 10 Hz step).
//
// Tests that care about the mapping spell the expectation out
// independently; tests that only need "this slot exists" use this.
func populatedAnswer(slot string) string {
	return mtAnswerFields{
		slot: slot, freq: "014250000",
		clarSign: '-', clarMag: "0150", rxClar: '1', txClar: '0',
		mode: '2', kind: '1', ctcss: '1', shift: '1',
		tag: "CALLING",
	}.frame()
}

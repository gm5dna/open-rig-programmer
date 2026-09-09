// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"bytes"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The two MA0 widths this package's TESTS need in order to build and
// recognise frames. core/kw/ma keeps its own unexported copies and is the
// authority on both; these are the fixture side, transcribed from the same
// two grid rows so that a test frame is the width the book prints.
const (
	// ma0ReadLen is "M A 0 P1 P1 P1 ;" — seven bytes (990:2916-2918).
	ma0ReadLen = 7
	// ma0AnswerLen is the fixed answer width on this row: eighteen
	// parameters, a ten-byte name window at 47-56 and ';' nailed to 57
	// (990:2919-2938).
	ma0AnswerLen = 57
	// exReadLen is "E X P1 P2 P2 P3 P3 ;" — eight bytes, carrying NO P4
	// (990:1734-1736).
	exReadLen = 8
)

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end PARSES the frames the driver writes and answers each one per a
// configurable image, recording every frame it received in order.
//
// It is the COMMAND-PARSING RESPONDER seam, inherited in SHAPE from
// core/driver/ts590's and rewritten for the MA family's frames. A
// fixed-transcript stub cannot test this driver at all: every session begins
// with Open's three-frame choreography (AI0;, ID;, FV;), so a test that wants
// to exercise one read has to answer three frames first.
//
// It is deliberately NOT internal/fakets990 (which lane C builds): a fake
// radio models a radio's STATE and is the right tool for round-trip and
// end-to-end tests, whereas this answers per-frame from a table and can
// therefore serve deliberately WRONG answers — a foreign ID, an answer naming
// the wrong slot, a mis-sized MA0 answer, or SILENCE — which is exactly what
// the error paths need and what a self-consistent fake will never produce.
//
// WHAT IT KNOWS: "AI0;" (silence), "ID;", "FV;", the seven-byte MA0 read and
// — since task 14 — the 57-byte MA0 Set, which draws SILENCE unless
// ma0SetReject scripts a "?;", and the eight-byte EX read. ANY OTHER frame is
// answered "?;", so a task that adds a command class and forgets to teach
// this helper about it sees its frame rejected loudly rather than silently
// succeed.
//
// THE ACKNOWLEDGEMENT SEMANTICS OF THOSE ANSWERS ARE AN ASSUMED CONVENTION
// APPLIED, NOT AN OBSERVED RADIO TRANSCRIBED — no TS-990S has ever been
// connected to this project. That "?;" is a NAK at all is the book's own
// error table; neither it nor the silence after AI0; is evidence of what a
// TS-990S does.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// radioImage is what a respondingPort's radio "contains": the identity it
// answers with, the firmware string, and the MA0 answers it serves per
// channel.
type radioImage struct {
	// catID is the three-character identity "ID;" answers with. Empty
	// selects this row's own, so only a wrong-radio test says anything
	// about it.
	catID string
	// idSilent makes "ID;" draw no reply at all, and idReject makes it
	// answer "?;" — the identity probe's two transport-level failure rows,
	// which no self-consistent radio image can express.
	idSilent bool
	idReject bool
	// fvAnswer is the WHOLE frame "FV;" is answered with. Empty selects
	// "FV1.00;", the book's own worked example (990:2533).
	fvAnswer string
	// fvSilent makes "FV;" draw no reply at all — the probe's timeout row.
	fvSilent bool
	// fvReject makes "FV;" answer "?;".
	fvReject bool
	// ma0Answers maps the THREE channel-number digits of an MA0 read —
	// frame[3:6] — to the RAW answer frame served for it. Raw, not
	// structured, so a test can serve a malformed or contradictory frame on
	// purpose. Keying on the digits is exact here because on this family the
	// channel number is the whole correlation key (990:2916-2918).
	ma0Answers map[string]string
	// ma0Silent names channels whose MA0 read draws NO REPLY AT ALL — the
	// timeout row of the read choreography, which the book states carries no
	// information at all.
	ma0Silent map[string]bool
	// ma0SetReject makes every MA0 SET answer "?;" instead of the silence
	// an accepted Set draws (A6). It is the write path's REJECTION row,
	// which is the only wire outcome of a Set that is attributable at all.
	ma0SetReject bool
	// exAnswers maps the FIVE-DIGIT menu address of an EX read —
	// frame[2:7] — to the RAW answer frame served for it. Raw, so a test can
	// serve a foreign address, a P5 wider than the row's printed width or
	// either of the two answer shapes E19 admits.
	exAnswers map[string]string
	// exReject names addresses answered "?;" — the settings seam's own
	// outcome, which is SettingUnavailable rather than a failure — and
	// exSilent those answered not at all.
	exReject map[string]bool
	exSilent map[string]bool
}

// newRespondingPort starts a scripted radio serving img and registers its
// cleanup. The returned value's Port is what a driver Opens.
func newRespondingPort(t *testing.T, img radioImage) *respondingPort {
	t.Helper()
	if img.catID == "" {
		img.catID = catID
	}
	if img.fvAnswer == "" {
		img.fvAnswer = "FV1.00;"
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

// Port returns the end handed to the driver. The driver takes ownership of it
// (Open closes it on failure; Session.Close on success), so a test must not
// close it itself — newRespondingPort's cleanup covers the rest.
func (p *respondingPort) Port() transport.Port { return p.host }

// Transcript returns a copy of every complete frame the port has received, in
// arrival order.
func (p *respondingPort) Transcript() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.received...)
}

// testTiming is the net.Pipe timing every test in this package opens with: a
// pipe answers instantly and would otherwise spend the fleet's radio-paced
// DefaultSettle on every frame of a hundred-slot bank walk.
func testTiming() Option { return withTiming(80*time.Millisecond, time.Millisecond) }

// openSessionAt opens this row at profile against a scripted radio serving
// img.
//
// THE PROFILE IS AN ARGUMENT AND NOT A DEFAULT, which is plan P7's pinning
// half in one signature: the capability-gate rung is pinned on an unconsented
// RealHardware session and every semantic rung on a session that has already
// passed that gate. A helper that chose the profile for its callers is
// exactly how a whole ladder of semantic pins goes green with none of the
// rungs implemented. T14's write tests are the consumers; the read tests take
// openTestSession below.
func openSessionAt(t *testing.T, profile Profile, img radioImage, opts ...Option) (*Session, *respondingPort) {
	t.Helper()
	p := newRespondingPort(t, img)
	d := New(profile, append([]Option{testTiming()}, opts...)...)
	sess, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), p
}

// openTestSession opens a Simulated session, which is what every read test
// wants: a read needs no profile distinction.
func openTestSession(t *testing.T, img radioImage, opts ...Option) (*Session, *respondingPort) {
	t.Helper()
	return openSessionAt(t, Simulated, img, opts...)
}

// serve reads the driver's bytes, splits them into ';'-terminated frames,
// records each, and writes back whatever img says.
//
// Frame splitting rather than whole-read matching: the transport writes one
// frame per call today, but nothing in the Port contract promises that.
func (p *respondingPort) serve(img radioImage) {
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

// reply returns the bytes this image answers frame with, or "" for silence.
//
// SILENCE HAS TWO MEANINGS HERE and they are not confusable in practice: the
// AI0 Set's silence is the assumed success signal, while a read's silence
// (fvSilent, ma0Silent) is a radio that did not answer a read at all, which
// the engine turns into a timeout. Only the second is a fault being scripted.
func (img radioImage) reply(frame string) string {
	switch {
	case frame == "ID;":
		switch {
		case img.idSilent:
			return ""
		case img.idReject:
			return "?;"
		}
		return "ID" + img.catID + ";"
	case frame == "FV;":
		switch {
		case img.fvSilent:
			return ""
		case img.fvReject:
			return "?;"
		}
		return img.fvAnswer
	case strings.HasPrefix(frame, "AI"):
		return ""
	case strings.HasPrefix(frame, "MA0") && len(frame) == ma0AnswerLen:
		// AN ACCEPTED SET DRAWS NOTHING, which is A6 applied and not a
		// TS-990S observed: no radio of this family has ever been written
		// to by this project. The Set and the Answer share one grid
		// (990:2893-2938), so the two MA0 forms are told apart by LENGTH
		// alone here exactly as the codec's own gate tells them apart.
		if img.ma0SetReject {
			return "?;"
		}
		return ""
	case strings.HasPrefix(frame, "MA0") && len(frame) == ma0ReadLen:
		slot := frame[3:6]
		if img.ma0Silent[slot] {
			return ""
		}
		if ans, ok := img.ma0Answers[slot]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "EX") && len(frame) == exReadLen:
		addr := frame[2:7]
		if img.exSilent[addr] {
			return ""
		}
		if img.exReject[addr] {
			return "?;"
		}
		if ans, ok := img.exAnswers[addr]; ok {
			return ans
		}
		return "?;"
	default:
		return "?;"
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"bytes"
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ma0ReadLen is the MA0 Read's width, "M A 0 P1 P1 P1 ;" (890:3184-3186).
// core/kw/ma keeps the same number unexported, and this peer needs it only to
// tell a READ from a SET, both of which open "MA0".
const ma0ReadLen = 7

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end PARSES the frames the driver writes and answers each one per a
// configurable image, recording every frame it received in order.
//
// It is the COMMAND-PARSING RESPONDER seam, inherited in SHAPE from
// core/driver/ts590's and rewritten for this row's frames. A fixed-transcript
// stub cannot test this driver at all: every session begins with Open's
// three-frame choreography (AI0;, ID;, FV;), so a test that wants to exercise
// one read has to answer three frames first.
//
// It is deliberately NOT internal/fakets890 (which lane C builds): a fake
// radio models a radio's STATE and is the right tool for round-trip and
// end-to-end tests, whereas this answers per-frame from a table and can
// therefore serve deliberately WRONG answers — a foreign ID, an answer naming
// the wrong slot, a short MA0 answer, or SILENCE — which is exactly what the
// error paths need and what a self-consistent fake will never produce.
//
// WHAT IT KNOWS: "AI0;" (silence), "ID;", "FV;", the seven-byte MA0 read, the
// LONGER MA0 Set (T12) and the eight-byte EX read (T12). ANY OTHER frame is
// answered "?;".
//
// THE ACKNOWLEDGEMENT SEMANTICS OF THOSE ANSWERS ARE AN ASSUMED CONVENTION
// APPLIED, NOT AN OBSERVED RADIO TRANSCRIBED — no TS-890S has ever been
// connected to this project. That "?;" is a NAK at all is the book's own
// error table (890:106-112); nothing here is evidence of what a TS-890S does.
//
// "?;" IS STILL THE RIGHT DEFAULT, whatever a real radio turns out to do: a
// task that adds a command class and forgets to teach this helper about it
// sees its frame REJECTED, loudly and immediately, rather than silently
// succeeding.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// radioImage is what a respondingPort's radio "contains": the identity it
// answers with, the firmware string, and the MA0 answers it serves per
// addressed channel.
type radioImage struct {
	// catID is the three-character identity "ID;" answers with. Empty
	// selects this row's own, so only a wrong-radio test says anything about
	// it.
	catID string
	// idSilent makes "ID;" draw no reply at all, and idReject makes it
	// answer "?;" — the identity probe's two transport-level failure rows,
	// which no self-consistent radio image can express.
	idSilent bool
	idReject bool
	// fvAnswer is the WHOLE frame "FV;" is answered with. Empty selects
	// "FV1.00;", the book's own worked example (890:2657).
	fvAnswer string
	// fvSilent makes "FV;" draw no reply at all, and fvReject makes it
	// answer "?;".
	fvSilent bool
	fvReject bool
	// ma0Answers maps the THREE-DIGIT channel number of an MA0 read —
	// frame[3:6] — to the RAW answer frame served for it. Raw, not
	// structured, so a test can serve a malformed or contradictory frame on
	// purpose.
	ma0Answers map[string]string
	// ma0Silent names channels whose MA0 read draws NO REPLY AT ALL — the
	// timeout row of the read choreography, which the book states carries no
	// information at all (890:106-112).
	ma0Silent map[string]bool
	// ma0SetRejects names channels whose MA0 SET is answered "?;" — the one
	// ATTRIBUTABLE write failure (T12). A Set this image does not reject
	// draws SILENCE, which is this family's assumed acceptance and the whole
	// reason WriteChannel reports Sent and never Confirmed.
	ma0SetRejects map[string]bool
	// exAnswers maps the FIVE-character menu address of an EX read —
	// frame[2:7] — to the RAW answer frame served for it, and exSilent names
	// addresses that draw no reply at all. Raw, so a test can serve a
	// malformed, over-wide or foreign-addressed frame on purpose.
	exAnswers map[string]string
	exSilent  map[string]bool
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

// testTiming is the net.Pipe timing override every test in this package
// opens with: a scripted peer answers instantly, and the transport's
// radio-paced defaults would otherwise spend real seconds per frame.
func testTiming() Option { return withTiming(80*time.Millisecond, time.Millisecond) }

// openTestSession opens a session at profile against a scripted radio serving
// img.
//
// THE PROFILE IS AN ARGUMENT AND NOT A DEFAULT, which is plan P7's rule in
// one signature: the capability gate answers first for EVERY write while
// writeTrialsComplete is false, so a helper that chose the profile for its
// callers is exactly how a ladder of semantic pins goes green with none of
// the rungs implemented. It is stated here, at T11, so T12's ladder inherits
// it rather than re-deriving it.
func openTestSession(t *testing.T, profile Profile, img radioImage, opts ...Option) (*Session, *respondingPort) {
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
	case strings.HasPrefix(frame, "MA0") && len(frame) == ma0ReadLen:
		channel := frame[3:6]
		if img.ma0Silent[channel] {
			return ""
		}
		if ans, ok := img.ma0Answers[channel]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MA0") && len(frame) > ma0ReadLen:
		// THE SET, which T12's ladder emits — a longer frame under the same
		// three opening bytes, the two forms told apart by LENGTH exactly as
		// the book's own two grids are (890:3166-3182 Set, 890:3184-3186
		// Read). Silence is the reply, and that silence is an ASSUMED
		// CONVENTION APPLIED rather than an observed radio: no TS-890S has
		// ever been written to by this project, which is why the driver
		// reports Sent and never Confirmed.
		if img.ma0SetRejects[frame[3:6]] {
			return "?;"
		}
		return ""
	case strings.HasPrefix(frame, "EX") && len(frame) == ma.EXReadLen:
		addr := frame[2:7]
		if img.exSilent[addr] {
			return ""
		}
		if ans, ok := img.exAnswers[addr]; ok {
			return ans
		}
		return "?;"
	default:
		return "?;"
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"bytes"
	"context"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end PARSES the frames the driver writes and answers each one per a
// configurable image, recording every frame it received in order.
//
// IT IS core/driver/ts590's respondingPort, ONE RADIO OVER — the same seam,
// with "FV;" replaced by "TY;" and the 480's own frame widths. The T13 review
// ruled that this package must reuse or mirror that shape rather than fork a
// second port type, and mirroring is what is available: the two live in
// different packages and neither may import the other's test files.
//
// A fixed-transcript stub cannot test this driver at all: every session
// begins with Open's three-frame choreography (AI0;, ID;, TY;), so a test
// that wants to exercise one read has to answer three frames first.
//
// It is deliberately NOT internal/fakets480 (which lane B builds): a fake
// radio models a radio's STATE and is the right tool for round-trip and
// end-to-end tests, whereas this answers per-frame from a table and can
// therefore serve deliberately WRONG answers — a foreign ID, an unprinted TY
// variant, an answer naming the wrong slot, a short MR answer, or SILENCE —
// which is exactly what the error paths need and what a self-consistent fake
// will never produce.
//
// WHAT IT KNOWS: "AI0;" (silence), "ID;", "TY;", the seven-byte MR read, the
// 50-byte MW Set and the ten-byte EX read. ANY OTHER frame is answered "?;".
//
// THE ACKNOWLEDGEMENT SEMANTICS OF THOSE ANSWERS ARE AN ASSUMED CONVENTION
// APPLIED, NOT AN OBSERVED RADIO TRANSCRIBED — no Kenwood radio has ever been
// connected to this project. That "?;" is a NAK at all is the book's own
// error table (480:130-135); that silence after a Set means acceptance is A6;
// neither is evidence of what a TS-480 does. THAT AN EMPTY CHANNEL ANSWERS AT
// ALL IS A4, WHICH IS THIS ROW'S REGISTRATION GATE — an image serving an
// empty MR answer is asserting A4, not observing it.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// radioImage is what a respondingPort's radio "contains": the identity it
// answers with, the TY answer, and the MR answers it serves per addressed
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
	// tyAnswer is the WHOLE frame "TY;" is answered with. Empty selects
	// defaultTYAnswer.
	tyAnswer string
	// tySilent makes "TY;" draw no reply at all — the probe's timeout row,
	// which no "?;" can stand in for. tyReject makes it answer "?;".
	tySilent bool
	tyReject bool
	// mrAnswers maps the FOUR addressing bytes of an MR read — P1, P2 and
	// P3's two digits, i.e. frame[2:6] — to the RAW answer frame served for
	// it. Raw, not structured, so a test can serve a malformed or
	// contradictory frame on purpose.
	mrAnswers map[string]string
	// mrSilent names addressed channels whose MR read draws NO REPLY AT ALL
	// — the timeout row of the read choreography, which the book states
	// carries no information at all (480:136-138).
	mrSilent map[string]bool
	// exAnswers maps the THREE-DIGIT menu address of an EX read —
	// frame[2:5] — to the RAW answer frame served for it, and exSilent
	// names addresses whose read draws no reply at all. An address with
	// neither entry is answered "?;", which the settings surface maps to
	// SettingUnavailable.
	exAnswers map[string]string
	exSilent  map[string]bool
	// mwReject makes every 50-byte MW Set answer "?;".
	//
	// NOTHING IN THIS PACKAGE CAN REACH IT TODAY, and that is A22 rather
	// than an oversight: every TS-480 channel write is refused before a
	// frame is built (write.go), so no MW ever leaves this driver. The arm
	// exists so that the peer's answer to an MW is a decision this file
	// records rather than a default nobody chose, and so that the commit
	// that lifts A22 (L-HW-16) finds the seam already here.
	mwReject bool
}

// defaultTYAnswer is the TY answer every image serves unless it says
// otherwise: "T Y P1 P1 P2 ;" (480:1634) with P2 '1', "1: TS-480SAT (100 W +
// AT)" (480:1627).
//
// P1's TWO BYTES ARE THIS FILE'S OWN CHOICE AND THE BOOK PRINTS NOTHING.
// The chart says only "Reserved" (480:1623), so no value here is evidence of
// anything; "00" is chosen because it is unremarkable, and
// TestOpen_TheTYAnswerIsCarriedOpaquely is what pins that the driver reports
// whatever arrives rather than expecting this.
const defaultTYAnswer = "TY001;"

// newRespondingPort starts a scripted radio serving img and registers its
// cleanup. The returned value's Port is what a driver Opens.
func newRespondingPort(t *testing.T, img radioImage) *respondingPort {
	t.Helper()
	if img.catID == "" {
		img.catID = catID
	}
	if img.tyAnswer == "" {
		img.tyAnswer = defaultTYAnswer
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

// openSession opens this row at profile against a scripted radio serving img,
// failing the test if Open does.
//
// THE PROFILE IS AN ARGUMENT AND NOT A DEFAULT, and that is plan P7's H2 in
// one signature: the capability-gate rung is pinned on an unconsented
// RealHardware session and the A22 rung on a session that has already passed
// that gate (Simulated). A helper that chose the profile for its callers is
// exactly how a ladder of semantic pins goes green with none of the rungs
// implemented.
func openSession(t *testing.T, profile driver.Profile, img radioImage, opts ...Option) (*Session, *respondingPort) {
	t.Helper()
	p := newRespondingPort(t, img)
	d := New(profile, append([]Option{testTiming()}, opts...)...)
	sess, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open(%s): %v", modelName, err)
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
// AI0 Set's silence is the ASSUMED success signal (A6), while a read's
// silence (tySilent, mrSilent, exSilent) is a radio that did not answer a
// read at all, which the engine turns into a timeout. Only the second is a
// fault being scripted.
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
	case frame == "TY;":
		switch {
		case img.tySilent:
			return ""
		case img.tyReject:
			return "?;"
		}
		return img.tyAnswer
	case strings.HasPrefix(frame, "AI"):
		return ""
	case strings.HasPrefix(frame, "MR") && len(frame) == kw.MRReadLen:
		addr := frame[2:6]
		if img.mrSilent[addr] {
			return ""
		}
		if ans, ok := img.mrAnswers[addr]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "EX") && len(frame) == kw.EXReadLen:
		addr := frame[2:5]
		if img.exSilent[addr] {
			return ""
		}
		if ans, ok := img.exAnswers[addr]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MW") && len(frame) == kw.RecordLen:
		if img.mwReject {
			return "?;"
		}
		return ""
	default:
		return "?;"
	}
}

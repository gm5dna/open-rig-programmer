// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end PARSES the frames the driver writes and answers each one per a
// configurable image, recording every frame it received in order.
//
// It is the COMMAND-PARSING RESPONDER seam, inherited in SHAPE from
// core/driver/ft891's and rewritten for this family's frames. A
// fixed-transcript stub cannot test this driver at all: every session begins
// with Open's three-frame choreography (AI0;, ID;, FV;), so a test that wants
// to exercise one read has to answer three frames first.
//
// It is deliberately NOT internal/fakets590 (which lane B builds): a fake
// radio models a radio's STATE and is the right tool for round-trip and
// end-to-end tests, whereas this answers per-frame from a table and can
// therefore serve deliberately WRONG answers — a foreign ID, an answer naming
// the wrong slot, a short MR answer, or SILENCE — which is exactly what the
// error paths need and what a self-consistent fake will never produce.
//
// WHAT IT KNOWS: "AI0;" (silence), "ID;", "FV;", the seven-byte MR read, the
// 50-byte MW Set and the ten-byte EX read. ANY OTHER frame is answered "?;".
//
// THE ACKNOWLEDGEMENT SEMANTICS OF THOSE ANSWERS ARE AN ASSUMED CONVENTION
// APPLIED, NOT AN OBSERVED RADIO TRANSCRIBED — no Kenwood radio has ever been
// connected to this project. That "?;" is a NAK at all is the books' own
// error table (590:100-105); that silence after a Set means acceptance is A6;
// neither is evidence of what a TS-590 does.
//
// "?;" IS STILL THE RIGHT DEFAULT here, whatever a real radio turns out to
// do: a task that adds a command class and forgets to teach this helper about
// it sees its frame REJECTED, loudly and immediately, rather than silently
// succeeding.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// radioImage is what a respondingPort's radio "contains": the identity it
// answers with, the firmware string, and the MR answers it serves per
// addressed channel.
type radioImage struct {
	// catID is the three-character identity "ID;" answers with. Empty
	// selects the row's own, so only a wrong-radio test says anything
	// about it.
	catID string
	// idSilent makes "ID;" draw no reply at all, and idReject makes it
	// answer "?;" — the identity probe's two transport-level failure rows,
	// which no self-consistent radio image can express and which the FV
	// pair below has needed since the probe was written.
	idSilent bool
	idReject bool
	// fvAnswer is the WHOLE frame "FV;" is answered with. Empty selects
	// "FV1.00;", the book's own worked example (590:1035).
	fvAnswer string
	// fvSilent makes "FV;" draw no reply at all — the probe's timeout row,
	// which no self-consistent radio image can express and no "?;" can
	// stand in for.
	fvSilent bool
	// fvReject makes "FV;" answer "?;".
	fvReject bool
	// mrAnswers maps the FOUR addressing bytes of an MR read — P1, P2 and
	// P3's two digits, i.e. frame[2:6] — to the RAW answer frame served for
	// it. Raw, not structured, so a test can serve a malformed or
	// contradictory frame on purpose. Keying on all four bytes rather than
	// on the channel number is what lets a section channel's L and U halves
	// be scripted apart (590:1449-1451).
	mrAnswers map[string]string
	// mrSilent names addressed channels whose MR read draws NO REPLY AT ALL
	// — the timeout row of the read choreography, which the books state
	// carries no information at all (590:106-108).
	mrSilent map[string]bool
	// exAnswers maps the THREE-DIGIT menu address of an EX read —
	// frame[2:5] — to the RAW answer frame served for it, and exSilent
	// names addresses whose read draws no reply at all. Raw, so a test can
	// serve an answer wider than the row's printed width or one naming
	// another address on purpose. An address with neither entry is answered
	// "?;", which the settings surface maps to SettingUnavailable.
	exAnswers map[string]string
	exSilent  map[string]bool
	// mwReject makes every 50-byte MW Set answer "?;" — the radio's
	// explicit rejection, which is attributable and is therefore reported
	// with Sent true. Its default, silence, is the ASSUMED acceptance
	// signal (A6): no Kenwood radio has ever been written to by this
	// project, and it is because the convention is assumed that
	// WriteChannel reports Sent and never Confirmed.
	mwReject bool
}

// newRespondingPort starts a scripted radio serving img and registers its
// cleanup. The returned value's Port is what a driver Opens.
func newRespondingPort(t *testing.T, row Row, img radioImage) *respondingPort {
	t.Helper()
	if img.catID == "" {
		img.catID = catIDFor(row)
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
// silence (fvSilent, mrSilent) is a radio that did not answer a read at all,
// which the engine turns into a timeout. Only the second is a fault being
// scripted.
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
		// AN MW OF ANY OTHER WIDTH FALLS THROUGH TO "?;", and that is the
		// right answer for this peer: the 42-byte erase form of
		// 590:1579-1581 is a frame this milestone never builds, and a test
		// that saw one accepted would be the failure the codec's own
		// 50-byte outbound gate exists to prevent.
		if img.mwReject {
			return "?;"
		}
		return ""
	default:
		return "?;"
	}
}

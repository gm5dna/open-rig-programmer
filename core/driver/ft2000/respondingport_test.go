// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end parses the frames the driver writes and answers each one per a
// configurable slot image, recording every frame it received in order.
//
// Deliberately NOT internal/fakeft2000 (Phase 3b, a different agent, not
// built yet and not read here — see doc.go): a fake radio models a
// radio's STATE and is the right tool for round-trip tests once it
// exists; this answers per-frame from a table and can therefore serve
// deliberately WRONG answers (a foreign CAT ID, an out-of-vocabulary
// kind byte, an answer naming the wrong slot), which is what the error
// paths need.
//
// SIMPLER THAN EVERY REGISTERED SIBLING'S OWN HELPER: this radio has no
// MT/tag command at all and no 5xx/EMG inventory to discover
// (yaesuParams.Probe is yaesu.NoProbe), so Open's whole choreography is
// AI0 + one ID probe — no discovery sweep to script around.
//
// WHAT IT KNOWS: AI (any AI frame, answered with silence), ID; (the
// image's catID), a 6-byte MR read ("MR"+slot+";"), and an MW Set. ANY
// OTHER frame is answered "?;". The Set/read acknowledgement semantics
// (silence == accepted, "?;" == rejected) are an ASSUMED convention
// carried from the FT-710's reference (doc.go) — NO FT-2000 OR FT-2000D
// HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, so nothing here is
// evidence of what either radio actually does.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// slotImage is what a respondingPort's radio "contains".
type slotImage struct {
	// catID is the four-character identity "ID;" answers with. Required:
	// newRespondingPort refuses an empty one, for the same reason
	// ftdx101's own helper does — this package drives two models, and a
	// default would silently pick one.
	catID string
	// mrAnswers maps a 3-byte slot wire form to the RAW 27-byte answer
	// served for an MR read of it. A slot absent from this map is
	// answered "?;" (read as "empty" by ReadChannel).
	mrAnswers map[string]string
	// rejectSets makes every MW Set answer "?;" instead of silence.
	rejectSets bool
}

// newRespondingPort starts a scripted radio serving img and registers its
// cleanup. The returned value's Port is what a driver Opens.
func newRespondingPort(t *testing.T, img slotImage) *respondingPort {
	t.Helper()
	if img.catID == "" {
		t.Fatal("slotImage.catID is empty — this package drives TWO models, so a scripted radio must say which one it is")
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

// Port returns the end handed to the driver. The driver takes ownership
// of it, so a test must not close it itself.
func (p *respondingPort) Port() transport.Port { return p.host }

// Transcript returns a copy of every complete frame the port has
// received, in arrival order.
func (p *respondingPort) Transcript() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.received...)
}

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

func (p *respondingPort) record(frame string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, frame)
}

// mrReadFrameLen is "MR" + 3-byte slot + ';'.
const mrReadFrameLen = 6

func (img slotImage) reply(frame string) string {
	switch {
	case frame == "ID;":
		return "ID" + img.catID + ";"
	case strings.HasPrefix(frame, "AI"):
		return ""
	case strings.HasPrefix(frame, "MR") && len(frame) == mrReadFrameLen:
		if ans, ok := img.mrAnswers[frame[2:5]]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MW"):
		if img.rejectSets {
			return "?;"
		}
		return ""
	default:
		return "?;"
	}
}

// mrAnswerFields is one MR answer's content in WIRE bytes, field by
// field, so a test states what the radio says rather than what a
// builder would produce for it.
type mrAnswerFields struct {
	slot     string // P1, positions 3-5
	freq     string // P2, positions 6-13, 8 digits (matrix §1.1)
	clarSign byte   // P3 sign, position 14
	clarMag  string // P3 magnitude, positions 15-18
	rxClar   byte   // P4, position 19
	txClar   byte   // P5, position 20
	mode     byte   // P6, position 21
	kind     byte   // P7, position 22
	ctcss    byte   // P8, position 23
	tone     string // P9, positions 24-25, 2 digits (live tone index)
	shift    byte   // P10, position 26
}

// frame assembles the 27-byte MR answer BY POSITION, from matrix §1.1's
// own offset table. DELIBERATELY NOT cat.Dialect.ParseMRAnswer/BuildMWSet:
// a fixture built by the code under test would pin the parser against
// itself.
func (f mrAnswerFields) frame() string {
	b := make([]byte, 27)
	for i := range b {
		b[i] = '?' // visible sentinel for an unfilled position
	}
	copy(b[0:2], "MR")
	copy(b[2:5], f.slot)
	copy(b[5:13], f.freq)
	b[13] = f.clarSign
	copy(b[14:18], f.clarMag)
	b[18] = f.rxClar
	b[19] = f.txClar
	b[20] = f.mode
	b[21] = f.kind
	b[22] = f.ctcss
	copy(b[23:25], f.tone)
	b[25] = f.shift
	b[26] = ';'
	return string(b)
}

// populatedAnswer is a valid, unremarkable MR answer for slot: a
// 14.250 MHz USB memory channel, no clarifier, CTCSS off, tone index 00,
// simplex.
func populatedAnswer(slot string) string {
	return mrAnswerFields{
		slot: slot, freq: "14250000",
		clarSign: '+', clarMag: "0000", rxClar: '0', txClar: '0',
		mode: '2', kind: '1', ctcss: '0', tone: "00", shift: '0',
	}.frame()
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx3000

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
// Mirrors core/driver/ft2000's own helper, simplified for one row: this
// radio has no MT/tag command at all and no 5xx/EMG inventory to discover,
// so Open's whole choreography is AI0 + one ID probe.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// slotImage is what a respondingPort's radio "contains".
type slotImage struct {
	// catID defaults to this dialect's own CAT ID when empty.
	catID string
	// mrAnswers maps a 3-byte slot wire form to the RAW 27-byte answer
	// served for an MR read of it. A slot absent is answered "?;".
	mrAnswers map[string]string
	// rejectSets makes every MW Set answer "?;" instead of silence.
	rejectSets bool
}

func newRespondingPort(t *testing.T, img slotImage) *respondingPort {
	t.Helper()
	if img.catID == "" {
		img.catID = catID
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
	freq     string // P2, positions 6-13, 8 digits
	clarSign byte   // P3 sign, position 14
	clarMag  string // P3 magnitude, positions 15-18
	rxClar   byte   // P4, position 19
	txClar   byte   // P5, position 20
	mode     byte   // P6, position 21
	kind     byte   // P7, position 22
	ctcss    byte   // P8, position 23
	tone     string // P9, positions 24-25, 2 digits (live on read)
	shift    byte   // P10, position 26
}

func (f mrAnswerFields) frame() string {
	b := make([]byte, 27)
	for i := range b {
		b[i] = '?'
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

// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

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
// configurable slotImage, recording every frame received in order.
//
// SIMPLER than the MT-bearing sibling drivers' helpers of the same name:
// this radio has no MT command, no EX settings surface and no 505-510
// discovery, so the only frames this package ever builds are AI (Init), ID
// (the handshake) and MR/MW (read.go, write.go).
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// slotImage is what a respondingPort's radio "contains".
type slotImage struct {
	// catID is the four-character identity "ID;" answers with. Empty
	// selects this dialect's own CATID.
	catID string
	// mrAnswers maps a 3-byte slot wire form to the RAW answer frame served
	// for an MR read of it. A slot absent from this map is answered "?;",
	// which ReadChannel maps to an empty channel.
	mrAnswers map[string]string
	// rejectWrites makes every MW Set answer "?;" instead of the silence
	// the ASSUMED convention reads as accepted.
	rejectWrites bool
}

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

// Port returns the end handed to the driver.
func (p *respondingPort) Port() transport.Port { return p.host }

// Transcript returns a copy of every complete frame the port has received.
func (p *respondingPort) Transcript() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.received...)
}

func (p *respondingPort) record(frame string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, frame)
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

// reply is img's answer to one complete frame, or "" for no reply at all
// (AI, and an accepted MW).
func (img slotImage) reply(frame string) string {
	switch {
	case strings.HasPrefix(frame, "AI"):
		return ""
	case frame == "ID;":
		return "ID" + img.catID + ";"
	case strings.HasPrefix(frame, "MR"):
		if len(frame) < 5 {
			return "?;"
		}
		slot := frame[2:5]
		if ans, ok := img.mrAnswers[slot]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MW"):
		if img.rejectWrites {
			return "?;"
		}
		return ""
	default:
		return "?;"
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

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

// respondingPort is this package's scripted radio — core/driver/ts480's
// respondingPort, one radio over: a net.Pipe whose remote end parses the
// frames the driver writes and answers each one per a configurable image,
// recording every frame it received in order. No EX handling: this row
// builds no settings surface (brief; EX/menu inventory is out of scope this
// wave).
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// radioImage is what a respondingPort's radio "contains".
type radioImage struct {
	// catID is the three-character identity "ID;" answers with. Empty
	// selects the row newRespondingPort was told to serve.
	catID              string
	idSilent, idReject bool
	// tyAnswer is the whole frame "TY;" is answered with. Empty selects
	// defaultTYAnswer.
	tyAnswer           string
	tySilent, tyReject bool
	// mrAnswers maps the four addressing bytes of an MR read — P1, P2 and
	// P3's two digits, i.e. frame[2:6] — to the raw answer frame served.
	mrAnswers map[string]string
	mrSilent  map[string]bool
	// mwReject makes every 50-byte MW Set answer "?;". Nothing in this
	// package can reach it: every channel write is refused before a frame
	// is built (write.go).
	mwReject bool
}

// defaultTYAnswer is the TY answer every image serves unless it says
// otherwise: "T Y P1 P1 P2 ;" with P2 '0', "0: Overseas type"
// (ts2000:11683). P1's two bytes are this file's own choice, on
// core/driver/ts480's own reasoning: the chart says only "Reserved".
const defaultTYAnswer = "TY000;"

// newRespondingPort starts a scripted radio serving img for catID and
// registers its cleanup.
func newRespondingPort(t *testing.T, catID string, img radioImage) *respondingPort {
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

// Port returns the end handed to the driver.
func (p *respondingPort) Port() transport.Port { return p.host }

// Transcript returns a copy of every complete frame the port has received.
func (p *respondingPort) Transcript() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.received...)
}

// openSession opens row (a New<Variant> constructor) against a scripted
// radio serving img, failing the test if Open does.
func openSession(t *testing.T, newRow func(...Option) driver.Driver, catID string, img radioImage, opts ...Option) (*Session, *respondingPort) {
	t.Helper()
	p := newRespondingPort(t, catID, img)
	d := newRow(append([]Option{testTiming()}, opts...)...)
	sess, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), p
}

// serve reads the driver's bytes, splits them into ';'-terminated frames,
// records each, and writes back whatever img says.
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

func (p *respondingPort) record(frame string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, frame)
}

// reply returns the bytes this image answers frame with, or "" for
// silence.
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
	case strings.HasPrefix(frame, "MW") && len(frame) == int(kw.RecordLen):
		if img.mwReject {
			return "?;"
		}
		return ""
	default:
		return "?;"
	}
}

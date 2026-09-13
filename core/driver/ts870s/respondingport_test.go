// SPDX-License-Identifier: GPL-3.0-or-later

package ts870s

import (
	"bytes"
	"context"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end parses the frames the driver writes and answers each one per a
// configurable image, recording every frame it received in order — the
// ts480/ts590 respondingPort shape (see either package's own file for the
// full rationale), restated for this row's own three grammars: "ID;", an
// MR read, and an MW set (record870.go's own AllowedCommand, which is
// exactly what this port must be able to answer to be worth scripting
// against).
//
// WHAT IT KNOWS: "AI0;" (silence, the assumed session preamble), "ID;", the
// six-byte MR read, and the 22-byte MW Set. ANY OTHER frame is answered
// "?;". THE ACKNOWLEDGEMENT SEMANTICS ARE AN ASSUMED CONVENTION APPLIED,
// NOT AN OBSERVED RADIO TRANSCRIBED — no TS-870S has ever been asked
// anything by this project (matrix header).
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// radioImage is what a respondingPort's radio "contains".
type radioImage struct {
	// catID is the three-character identity "ID;" answers with. Empty
	// selects this row's own.
	catID string
	// idSilent/idReject are the identity probe's two transport-level
	// failure rows.
	idSilent bool
	idReject bool
	// mrAnswers maps a channel's two-digit P3 to the RAW answer frame
	// served for it. mrSilent names channels whose MR read draws no
	// reply at all.
	mrAnswers map[string]string
	mrSilent  map[string]bool
	// mwReject makes every 22-byte MW Set answer "?;".
	mwReject bool
}

func newRespondingPort(t *testing.T, img radioImage) *respondingPort {
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

// Port returns the end handed to the driver. The driver takes ownership of
// it (Open closes it on failure; Session.Close on success), so a test must
// not close it itself.
func (p *respondingPort) Port() transport.Port { return p.host }

func (p *respondingPort) Transcript() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.received...)
}

// openSession opens this row against a scripted radio serving img, failing
// the test if Open does.
func openSession(t *testing.T, profile Profile, img radioImage, opts ...Option) (*Session, *respondingPort) {
	t.Helper()
	p := newRespondingPort(t, img)
	d := New(profile, opts...)
	sess, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open(%s): %v", modelName, err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), p
}

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
	case strings.HasPrefix(frame, "AI"):
		return ""
	case strings.HasPrefix(frame, "MR") && len(frame) == 6:
		addr := frame[3:5]
		if img.mrSilent[addr] {
			return ""
		}
		if ans, ok := img.mrAnswers[addr]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MW") && len(frame) == 22:
		if img.mwReject {
			return "?;"
		}
		return ""
	default:
		return "?;"
	}
}

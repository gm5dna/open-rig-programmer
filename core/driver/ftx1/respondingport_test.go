// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// respondingPort is this package's scripted radio: a net.Pipe whose
// remote end PARSES the frames the driver writes and answers each one
// per a configurable slot image, recording every frame it received in
// order — the same "COMMAND-PARSING RESPONDER" seam every registered
// Yaesu driver package in this fleet carries its own copy of (see
// core/driver/ftdx1200's own respondingport_test.go). Deliberately NOT
// internal/fakeftx1 (lane F3's, a separate phase of this milestone): a
// fake radio models a radio's STATE, this answers per-frame from a
// table, and can therefore serve deliberately WRONG answers the error
// paths need to exercise.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received []string
}

// slotImage is what a respondingPort's radio "contains".
type slotImage struct {
	// catID defaults to this dialect's own CAT ID ("0840") when empty.
	catID string
	// mrAnswers maps a 5-byte slot wire form to the RAW 30-byte answer
	// served for an MR read of it. A slot absent is answered "?;" — the
	// ASSUMED empty-slot convention read.go's own doc comment names.
	mrAnswers map[string]string
	// mtAnswers maps a slot to the RAW 20-byte answer served for an MT
	// read of it.
	mtAnswers map[string]string
	// rejectMW / rejectMT make every MW/MT Set answer "?;" instead of the
	// silence that means "accepted".
	rejectMW, rejectMT bool
}

func newRespondingPort(t *testing.T, img slotImage) *respondingPort {
	t.Helper()
	if img.catID == "" {
		img.catID = dialect.CATID()
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

func (p *respondingPort) Port() transport.Port { return p.host }

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

// Frame lengths this radio's own commands take — see dialect.go
// (SlotDigits 5, MemoryFrameLen 30) and mtnodisplay.go (2+5+12+1=20).
const (
	mrReadFrameLen = 2 + 5 + 1 // "MR" + slot(5) + ";"
	mtReadFrameLen = 2 + 5 + 1 // "MT" + slot(5) + ";"
	mwSetFrameLen  = 30        // "MW" + 27 data bytes + ";"
	mtSetFrameLen  = 2 + 5 + 12 + 1
)

// reply answers ONE frame per img's table. THE TWO MT LENGTHS ARE THE
// ONLY MT FRAMES ADMITTED (mirroring core/driver/ft991a's own
// respondingPort): a READ request (8 bytes) is looked up in mtAnswers, a
// SET (20 bytes) is accepted/rejected per rejectMT; anything else "MT"
// falls through to "?;" — a driver bug that built an MT frame of any
// other length is rejected loudly rather than silently answered.
func (img slotImage) reply(frame string) string {
	switch {
	case frame == "ID;":
		return "ID" + img.catID + ";"
	case strings.HasPrefix(frame, "AI"):
		return ""
	case strings.HasPrefix(frame, "MR") && len(frame) == mrReadFrameLen:
		if ans, ok := img.mrAnswers[frame[2:7]]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MT") && len(frame) == mtReadFrameLen:
		if ans, ok := img.mtAnswers[frame[2:7]]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MT") && len(frame) == mtSetFrameLen:
		if img.rejectMT {
			return "?;"
		}
		return ""
	case strings.HasPrefix(frame, "MW") && len(frame) == mwSetFrameLen:
		if img.rejectMW {
			return "?;"
		}
		return ""
	default:
		return "?;"
	}
}

// mustMemorySlot is a test helper: MemorySlot(n) or t.Fatal.
func mustMemorySlot(t *testing.T, n int) cat.Slot {
	t.Helper()
	s, err := dialect.MemorySlot(n)
	if err != nil {
		t.Fatalf("MemorySlot(%d): %v", n, err)
	}
	return s
}

// populatedMRAnswer builds a valid, unremarkable MR answer for slot: a
// 14.250 MHz USB memory channel, no clarifier, CTCSS off, simplex — by
// building a Set frame through the dialect's own BuildMWSet and
// re-prefixing it "MR", rather than hand-typing offsets: the memory
// record's byte layout is BuildMWSet's job to get right, once, and this
// helper never re-derives it.
func populatedMRAnswer(t *testing.T, slot cat.Slot) string {
	t.Helper()
	cmd, err := dialect.BuildMWSet(cat.MemoryData{
		Slot: slot, FreqHz: 14250000, Mode: cat.ModeUSB,
		Kind: dialect.MWWriteKind(), CTCSS: cat.CTCSSOff, Shift: cat.ShiftSimplex,
	})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	b := append([]byte(nil), cmd.Bytes()...)
	b[0], b[1] = 'M', 'R'
	return string(b)
}

// populatedMTAnswer builds a valid MT answer for slot carrying tag, via
// BuildMTSetNoDisplay: the Set and Answer frames share one wire shape
// under MTFormShortNoDisplay (spec.md §3.3), so a Set frame IS a valid
// scripted Answer.
func populatedMTAnswer(t *testing.T, slot cat.Slot, tag string) string {
	t.Helper()
	cmd, err := dialect.BuildMTSetNoDisplay(slot, tag)
	if err != nil {
		t.Fatalf("BuildMTSetNoDisplay: %v", err)
	}
	return string(cmd.Bytes())
}

// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import (
	"fmt"
	"strings"
)

// This file is the WIRE layer, and only the wire layer. Everything in it is
// PRINTED in the IC-7100 FULL MANUAL's section 20, CONTROL COMMAND, as quoted
// by the two quarantined artefacts PROVENANCE.md names; nothing in it knows
// anything about what a memory record contains. That knowledge lives in
// records.go and comes from the same two artefacts read for a different
// purpose.
//
// The printed frame is, on PDF p.361 (folio 20-2), "◇ Data format":
//
//	Controller to IC-7100:  FE FE  88  E0  Cn  Sc  <data area>  FD
//	IC-7100 to controller:  FE FE  E0  88  Cn  Sc  <data area>  FD
//	OK message:             FE FE  E0  88  FB  FD
//	NG message:             FE FE  E0  88  FA  FD
//
// with (1) "Preamble code (fixed)", (2) the transceiver's default address,
// (3) the controller's default address, (4) the command number, (5) the sub
// command number, (6) the data area and (7) "End of message code (fixed)".
const (
	// preamble is FE, the byte a frame begins with. endOfMessage is FD, the
	// byte it ends with. Both are labelled "(fixed)" on the printed diagram,
	// and neither may appear inside a frame's data — which is what lets the
	// accumulator below resynchronise on either.
	preamble     = 0xFE
	endOfMessage = 0xFD

	// codeOK and codeNG are the printed acknowledge and reject codes, labelled
	// "OK code (fixed)" and "NG code (fixed)" on PDF p.361 (folio 20-2).
	codeOK = 0xFB
	codeNG = 0xFA

	// radioAddress is the transceiver's default CI-V address, printed as the
	// second byte of the "Controller to IC-7100" frame on PDF p.361 and stated
	// again in the set-mode group at PDF p.334 (folio 17-25), "CI-V Address
	// (Default: 88h)". controllerAddress is the controller's, printed as E0 in
	// the same diagram. A frame from the controller carries to=88, from=E0, and
	// this radio's answer swaps them.
	//
	// The radio's address is a front-panel setting on a real IC-7100, so it is
	// the DEFAULT here rather than a fixture: WithRadioAddress changes it.
	radioAddress      = 0x88
	controllerAddress = 0xE0

	// broadcastAddress is the to byte this fake puts on a transceive broadcast.
	// It is ASSUMED, not printed — see doc.go, register entry 5.
	broadcastAddress = 0x00

	// cmdTransceiverID / subTransceiverID is 19 00, whose command-table row on
	// PDF p.364 (folio 20-5) reads "19 | 00 | (Data column empty) | Read the
	// transceiver ID". The empty Data column is why the request carries no data
	// area, and why the REPLY's bytes are undocumented (doc.go, entry 4).
	cmdTransceiverID = 0x19
	subTransceiverID = 0x00

	// cmdMemoryContent / subMemoryContent is 1A 00, whose command-table row on
	// PDF p.364 reads "1A | 00 | see p. 20-16 | Send/read the Memory channel
	// contents"; PDF p.375 (folio 20-16) heads the same material "• Memory
	// content setting / Command: 1A 00".
	cmdMemoryContent = 0x1A
	subMemoryContent = 0x00
)

// bodyPrefixLen is the number of body bytes before the command byte: the to
// address and the from address, indices (2) and (3) of the printed frame.
const bodyPrefixLen = 2

// maxFrameBody caps how many body bytes the accumulator will hold for one frame
// before deciding the end-of-message byte is never coming. It is NOT a protocol
// limit — the manual states none (doc.go, register entry 9) — it is a guard so
// that a stream of garbage cannot grow the buffer without bound. It is set far
// above the longest frame this radio can be asked for, the 121-byte complete
// record set.
const maxFrameBody = 4096

// frame is one parsed CI-V frame: its two addresses and everything between them
// and the end-of-message byte, which is the command byte, its sub-command byte
// if it has one, and the data area.
type frame struct {
	to   byte
	from byte
	data []byte
}

// parseFrame splits a frame body — the bytes between the preamble and the
// end-of-message byte — into addresses and data. A body with no command byte at
// all is not a frame: the printed diagram has no form without index (4).
func parseFrame(body []byte) (frame, bool) {
	if len(body) < bodyPrefixLen+1 {
		return frame{}, false
	}
	return frame{to: body[0], from: body[1], data: body[bodyPrefixLen:]}, true
}

// buildFrame assembles FE FE <to> <from> <data…> FD.
func buildFrame(to, from byte, data ...byte) []byte {
	out := make([]byte, 0, bodyPrefixLen+len(data)+3)
	out = append(out, preamble, preamble, to, from)
	out = append(out, data...)
	return append(out, endOfMessage)
}

// canonicalFrame wraps an already-assembled body in exactly two preamble bytes
// and one end-of-message byte. Frames may arrive preceded by a long run of
// preamble bytes — the manual's own power-ON example on PDF p.363 (folio 20-4)
// prepends seven at 4800 bps and twenty-five at 19200 — and this is where that
// is normalised away, so that Transcript reports what was said rather than how
// slowly it was said.
func canonicalFrame(body []byte) []byte {
	out := make([]byte, 0, len(body)+3)
	out = append(out, preamble, preamble)
	out = append(out, body...)
	return append(out, endOfMessage)
}

// okFrame is the printed OK message, addressed back to whoever asked.
func okFrame(to, from byte) []byte { return buildFrame(to, from, codeOK) }

// ngFrame is the printed NG message, addressed back to whoever asked.
func ngFrame(to, from byte) []byte { return buildFrame(to, from, codeNG) }

// accumulator turns an arbitrarily chopped byte stream into whole frame bodies.
// It is forgiving in exactly the ways the printed frame allows:
//
//   - bytes before a preamble pair are noise and are dropped;
//   - a run of preamble bytes of any length introduces one frame, which is what
//     the power-ON example on folio 20-4 requires of a receiver;
//   - an end-of-message byte outside a frame is dropped.
//
// And it resynchronises where the printed frame lets it: since a preamble byte
// is "(fixed)" at the head of a frame and cannot appear inside data, one that
// does appear there means the frame in hand was lost and a new one is starting.
type accumulator struct {
	preambleRun int
	inBody      bool
	body        []byte
}

func newAccumulator() *accumulator { return &accumulator{} }

// feed consumes b and returns every whole frame body it completed, in order.
// The returned slices are owned by the caller.
func (a *accumulator) feed(b []byte) [][]byte {
	var out [][]byte
	for _, c := range b {
		switch {
		case c == preamble:
			// Inside a body this is illegal data, so the body is abandoned;
			// either way this byte counts towards the run introducing the next
			// frame.
			if a.inBody {
				a.reset()
			}
			a.preambleRun++

		case c == endOfMessage:
			if a.inBody {
				done := make([]byte, len(a.body))
				copy(done, a.body)
				out = append(out, done)
			}
			a.reset()

		default:
			switch {
			case a.inBody:
				if len(a.body) >= maxFrameBody {
					// No end-of-message byte is coming. Drop it and wait for a
					// fresh preamble pair.
					a.reset()
					continue
				}
				a.body = append(a.body, c)
			case a.preambleRun >= 2:
				a.inBody = true
				a.body = append(a.body[:0], c)
				a.preambleRun = 0
			default:
				// Noise, or a lone preamble byte that turned out not to
				// introduce anything.
				a.preambleRun = 0
			}
		}
	}
	return out
}

func (a *accumulator) reset() {
	a.inBody = false
	a.body = a.body[:0]
	a.preambleRun = 0
}

// hexBytes renders a byte slice the way this package's failure messages and
// panics do, so that a wrong frame is readable at a glance.
func hexBytes(b []byte) string {
	if len(b) == 0 {
		return "(empty)"
	}
	var sb strings.Builder
	for i, c := range b {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%02X", c)
	}
	return sb.String()
}

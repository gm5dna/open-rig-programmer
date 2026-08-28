// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"fmt"
	"strings"
)

// This file is the WIRE layer, and only the wire layer. Everything in it comes
// from the CI-V facts the package brief quotes out of the IC-9700 CI-V
// Reference Guide (p.4/folio 3, p.6/folio 5, p.13/folio 12); nothing in it
// knows anything about what a memory record contains. That knowledge lives in
// image.go and comes from the two transcriptions named in PROVENANCE.md.

const (
	// preamble is FE, the byte a frame begins with. endOfMessage is FD, the
	// byte it ends with. Neither may appear inside a frame's data, which is
	// what makes the accumulator below able to resynchronise on either.
	preamble     = 0xFE
	endOfMessage = 0xFD

	// codeOK and codeNG are the acknowledge and reject codes. The whole OK
	// frame from this radio to the default controller is FE FE E0 A2 FB FD and
	// the whole NG frame is FE FE E0 A2 FA FD.
	codeOK = 0xFB
	codeNG = 0xFA

	// radioAddress is the transceiver's default CI-V address; controllerAddress
	// is the controller's. A frame from the controller carries to=A2, from=E0,
	// and this radio's answer swaps them.
	radioAddress      = 0xA2
	controllerAddress = 0xE0

	// broadcastAddress is the to byte a transceive broadcast carries. It is the
	// one byte that separates the two flood species this fake can emit: a
	// to=00 broadcast is dropped by a controller's accumulator, while a frame
	// addressed to the controller reaches its engine.
	broadcastAddress = 0x00

	// cmdTransceiverID / subTransceiverID is 19 00, "read the transceiver ID".
	cmdTransceiverID = 0x19
	subTransceiverID = 0x00

	// cmdMemoryContent / subMemoryContent is 1A 00, "send/read memory
	// contents".
	cmdMemoryContent = 0x1A
	subMemoryContent = 0x00
)

// bodyPrefixLen is the number of body bytes before the command byte: the to
// address and the from address.
const bodyPrefixLen = 2

// maxFrameBody caps how many body bytes the accumulator will hold for one
// frame before deciding the end-of-message byte is never coming. It is not a
// protocol limit — the protocol's own limit is whatever the longest record is,
// a figure this package deliberately does not know (see doc.go, THE RECORD
// LENGTH STOP) — it is a guard so that a stream of garbage cannot grow the
// buffer without bound.
const maxFrameBody = 4096

// frame is one parsed CI-V frame: its two addresses and everything between
// them and the end-of-message byte, which is the command byte, its sub-command
// byte if it has one, and the data.
type frame struct {
	to   byte
	from byte
	data []byte
}

// parseFrame splits a frame body — the bytes between the preamble and the
// end-of-message byte — into addresses and data. A body with no command byte at
// all is not a frame.
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
// and one end-of-message byte. Frames may arrive preceded by up to 119 preamble
// bytes; this is where that is normalised away, so that Transcript reports what
// was said rather than how slowly it was said.
func canonicalFrame(body []byte) []byte {
	out := make([]byte, 0, len(body)+3)
	out = append(out, preamble, preamble)
	out = append(out, body...)
	return append(out, endOfMessage)
}

// okFrame is the acknowledge frame, addressed back to whoever asked.
func okFrame(to byte) []byte { return buildFrame(to, radioAddress, codeOK) }

// ngFrame is the reject frame, addressed back to whoever asked.
func ngFrame(to byte) []byte { return buildFrame(to, radioAddress, codeNG) }

// accumulator turns an arbitrarily chopped byte stream into whole frame bodies.
// It is deliberately forgiving in exactly the ways the printed wire facts say a
// receiver must be:
//
//   - bytes before a preamble pair are noise and are dropped;
//   - a run of preamble bytes of any length introduces one frame;
//   - an end-of-message byte outside a frame is dropped.
//
// And it resynchronises where the facts say it may: since a preamble byte
// cannot appear inside data, one that does appear there means the frame in
// hand was lost and a new one is starting.
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

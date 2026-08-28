// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300mk2

// The CI-V frame grammar, and the reassembler that recovers frames from a
// byte stream that may carry leading noise and any number of extra preamble
// bytes.
//
// FE FE <to> <from> <cn> [<sc>] <data> FD
//
// FE (preamble) and FD (end of message) are reserved: neither appears anywhere
// inside a well-formed frame, which is what lets the reassembler resynchronise
// on FE without any framing state beyond "how many preamble bytes have I seen".

const (
	// preamble is the FE byte that opens a frame. Two of them are the
	// grammar's own opening; more than two are tolerated (a real CI-V bus
	// pads with them to let a sleeping receiver wake).
	preamble = 0xFE
	// endOfMessage is the FD byte that closes a frame.
	endOfMessage = 0xFD

	// controllerAddr is the CI-V address of the controller at the other end
	// of the port. It is fixed: this package fakes a radio, and a radio does
	// not get to choose who is talking to it.
	controllerAddr = 0xE0

	// defaultRadioAddr is this radio's factory CI-V address. WithRadioAddress
	// replaces it; every byte of every answer that names the radio follows
	// whatever it is set to (see doc.go, "The radio's address is not a
	// literal").
	defaultRadioAddr = 0xB6

	// okCode and ngCode are the whole content of the two acknowledgement
	// frames. FE FE <ctrl> <radio> FB FD is six bytes; so is the FA form.
	okCode = 0xFB
	ngCode = 0xFA

	// cmdIdentity / subIdentity is the identity read, 19 00, sent with no
	// data area.
	cmdIdentity = 0x19
	subIdentity = 0x00

	// cmdMemory / subMemory is the memory command, 1A 00.
	cmdMemory = 0x1A
	subMemory = 0x00

	// clearMarker is the single FF byte that, in place of a record, makes a
	// 1A 00 frame a channel clear. This fake REFUSES it — deliberately, see
	// doc.go — rather than implementing it.
	clearMarker = 0xFF

	// cmdTransceive is the command byte the unsolicited frames of
	// WithTransceiveBroadcasts and WithAddressedFlood carry. It is a
	// frequency report: five BCD bytes in the packing derived for the
	// record's own frequency field (record.go).
	cmdTransceive = 0x00

	// maxFrameBytes caps the reassembler's accumulator. A stream of noise
	// containing FE FE and then no FD would otherwise grow a buffer without
	// bound; past the cap the partial frame is abandoned and the reassembler
	// resynchronises on the next preamble. No real frame this radio answers
	// is anywhere near this long — the longest is a memory set at 4 + 2 + 2 +
	// 45 + 1 = 54 bytes.
	maxFrameBytes = 4096
)

// reassembler state.
const (
	stIdle = iota // outside any frame: everything but FE is noise
	stPre         // inside a run of preamble bytes
	stBody        // accumulating <to> <from> <cn> ... up to FD
)

// reassembler recovers complete frames from a byte stream. It is not safe for
// concurrent use; exactly one goroutine (serve) drives it.
type reassembler struct {
	max   int
	state int
	pre   int // preamble bytes seen in the current run
	body  []byte
}

// push feeds p to the reassembler and returns every frame that completed
// within it, in order. Each returned frame is a fresh slice holding the
// NORMALISED frame: exactly two preamble bytes, the body as received, and the
// end-of-message byte. See doc.go's register entry on Received().
func (a *reassembler) push(p []byte) [][]byte {
	var out [][]byte
	for _, b := range p {
		switch a.state {
		case stIdle:
			if b == preamble {
				a.state, a.pre = stPre, 1
			}
			// Anything else is leading noise, and is dropped.

		case stPre:
			switch {
			case b == preamble:
				a.pre++
			case b == endOfMessage:
				// FE ... FE FD carries no body at all. There is nothing to
				// address and nothing to answer, so it is not a frame.
				a.reset()
			case a.pre >= 2:
				a.body = append(a.body[:0], b)
				a.state = stBody
			default:
				// A single FE followed by a data byte is not a frame opening:
				// the grammar's preamble is two bytes. Treat it as noise.
				a.reset()
			}

		case stBody:
			switch {
			case b == preamble:
				// FE is reserved and cannot occur inside a body, so whatever
				// has accumulated was noise and a new preamble has begun.
				a.body = a.body[:0]
				a.state, a.pre = stPre, 1
			case b == endOfMessage:
				out = append(out, frameOf(a.body))
				a.reset()
			case len(a.body) >= a.max:
				a.reset()
			default:
				a.body = append(a.body, b)
			}
		}
	}
	return out
}

// reset returns the reassembler to the idle state, keeping the accumulator's
// backing array for reuse.
func (a *reassembler) reset() {
	a.state, a.pre = stIdle, 0
	a.body = a.body[:0]
}

// frameOf wraps a body in the normalised framing: FE FE <body> FD. The result
// shares nothing with body.
func frameOf(body []byte) []byte {
	f := make([]byte, 0, len(body)+3)
	f = append(f, preamble, preamble)
	f = append(f, body...)
	return append(f, endOfMessage)
}

// bodyOf returns the body of a normalised frame — everything between the two
// preamble bytes and the end-of-message byte. It aliases frame.
func bodyOf(frame []byte) []byte {
	if len(frame) < 3 {
		return nil
	}
	return frame[2 : len(frame)-1]
}

// frameTo builds an answer FROM this radio TO the given address, with head as
// the command (and sub-command) bytes and data as the data area.
func (r *Radio) frameTo(to byte, head, data []byte) []byte {
	f := make([]byte, 0, 4+len(head)+len(data)+1)
	f = append(f, preamble, preamble, to, r.addr)
	f = append(f, head...)
	f = append(f, data...)
	return append(f, endOfMessage)
}

// answerFrame builds an answer addressed to the controller.
func (r *Radio) answerFrame(head, data []byte) []byte {
	return r.frameTo(controllerAddr, head, data)
}

// pass is the six-byte OK frame, FE FE <controller> <this radio> FB FD.
func (r *Radio) pass() []byte { return r.answerFrame([]byte{okCode}, nil) }

// fail is the six-byte NG frame, FE FE <controller> <this radio> FA FD.
func (r *Radio) fail() []byte { return r.answerFrame([]byte{ngCode}, nil) }

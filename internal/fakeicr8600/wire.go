// SPDX-License-Identifier: GPL-3.0-or-later

package fakeicr8600

// ---------------------------------------------------------------------------
// The wire, from the printed frame diagrams
// ---------------------------------------------------------------------------

// The framing bytes, from the IC-R8600 CI-V REFERENCE GUIDE, rev A7375-2EX-3a,
// PDF page 3 (printed folio 2), "◇ About the data format", which draws four
// complete frames:
//
//	Preamble             FE FE           ("Preamble code (fixed)")
//	End of message       FD              ("End of message code (fixed)")
//	Receiver address     96              ("Receiver's default address")
//	Controller address   E0              ("Controller's default address")
//	Controller -> radio  FE FE 96 E0 <cn> [<sc>] [data] FD
//	Radio -> controller  FE FE E0 96 <cn> [<sc>] [data] FD
//	OK  (ack)            FE FE E0 96 FB FD   ("OK code (fixed)")
//	NG  (reject)         FE FE E0 96 FA FD   ("NG code (fixed)")
//
// Every one of these is re-derived here rather than imported from core/civ,
// which also carries them. That is THE HARD RULE (doc.go): two independent
// spellings of one protocol, checked against each other, is what makes a
// systematic error in either one visible.
const (
	preambleByte = 0xFE
	endOfMessage = 0xFD
	// defaultRadioAddr is the receiver's DEFAULT address. It is user-changeable
	// in Set mode (PDF page 3, "Preparing"), which is what WithRadioAddress
	// exists to model.
	defaultRadioAddr = 0x96
	controllerAddr   = 0xE0
	// broadcastAddr is the `to` byte of a transceive frame. ASSUMED — doc.go
	// register entry 3. The guide draws four frames and none is a broadcast.
	broadcastAddr = 0x00
	okCode        = 0xFB
	ngCode        = 0xFA
)

// The two commands this tier sends, from the command table on PDF page 5
// (printed folio 4):
//
//	19 / 00 / <blank Data cell> / Read the receiver ID
//	1A / 00* / See pp. 11 ~ 14  / Send/read memory channel contents
//
// where the asterisk is expanded on PDF page 9 as "*(Asterisk) Send/read data".
const (
	cmdReceiverID = 0x19
	subReadID     = 0x00

	cmdMemory         = 0x1A
	subMemoryContents = 0x00
)

// preambleLen is how many preamble bytes a canonical frame carries. A frame
// arriving with more is understood — the one worked example the guide prints
// (PDF page 9, "Example: When using 4800 bps") pads a power-on command with
// five extra FE bytes — but the frame handed on carries exactly two.
const preambleLen = 2

// minBodyBytes is the shortest body that can be dispatched at all: `to`,
// `from`, `cn`. The OK and NG diagrams are exactly this long.
const minBodyBytes = 3

// maxBodyBytes caps how many bytes may accumulate between a preamble and an
// end-of-message byte before the reassembler gives up on the frame.
//
// THE DOCUMENT STATES NO SUCH LIMIT — ASSUMED, doc.go register entry 12. The
// cap is a property of a reader that must not grow without bound on a line that
// has come up mid-frame or gone mad; it is not a claim about the receiver.
//
// The longest body this fake can legitimately receive is a dPMR memory set: to,
// from, cn, sc, the four address bytes and a 45-byte record — 53 bytes. The cap
// is set well clear of that so a consumer may also push a deliberately
// oversized record at the fake and see it refused rather than swallowed, which
// is what TestTheOverflowCapIsNotHitByTheLongestFrameThisFakeCanReceive pins.
const maxBodyBytes = 256

// ---------------------------------------------------------------------------
// Reassembly
// ---------------------------------------------------------------------------

// accEvent is one unit reassembler.push hands back: either a complete frame or
// an overflow. Overflow is ITS OWN EVENT rather than a silently dropped frame,
// because a consumer needs to be able to see that the fake refused something
// rather than lost it.
type accEvent struct {
	frame    []byte
	overflow bool
}

// reassembler turns a byte stream into frames, scanning for FE FE … FD.
//
// It is written from the printed frame diagram and from nothing else — not from
// any accumulator in this repository. Its rules:
//
//   - Bytes before a preamble are noise and are discarded. A line that comes up
//     mid-frame, or a device that was mid-transmission when the port opened,
//     must not poison the next good frame.
//   - The preamble is TWO bytes. A single FE followed by ordinary bytes starts
//     nothing.
//   - A run of MORE than two FE opens a frame just the same: the guide's own
//     worked example pads with five. The frame handed on carries the canonical
//     two.
//   - An FE inside a body abandons the partial frame and starts a new one. FE
//     is the preamble code; it cannot occur inside a body, so seeing one means
//     the body in hand was truncated.
//   - An FD after a preamble ends the message, even with nothing between them.
//     "End of message code (fixed)" means what it says, and treating that FD as
//     an ordinary body byte would leave the reader looking for a second one
//     that is never coming.
//   - A body that reaches max without an end-of-message byte produces one
//     overflow event, exactly one, and the reader then resynchronises on the
//     next preamble. It does not wedge, and it does not emit an event per byte
//     thereafter.
type reassembler struct {
	body      []byte
	max       int
	preambles int
}

func newReassembler(max int) *reassembler {
	if max <= 0 {
		max = maxBodyBytes
	}
	return &reassembler{max: max}
}

func (a *reassembler) reset() {
	a.body = a.body[:0]
	a.preambles = 0
}

// push feeds one chunk of received bytes and returns every event it completed,
// in order. A frame may span any number of chunks and a chunk may carry any
// number of frames.
func (a *reassembler) push(chunk []byte) []accEvent {
	var events []accEvent

	for _, b := range chunk {
		switch {
		case b == preambleByte:
			a.body = a.body[:0]
			if a.preambles < preambleLen {
				a.preambles++
			}

		case a.preambles < preambleLen:
			// Noise: no preamble has opened a frame, so this byte is not part
			// of one. A lone FE followed by ordinary bytes lands here and
			// resets the count.
			a.preambles = 0

		case b == endOfMessage:
			frame := make([]byte, 0, preambleLen+len(a.body)+1)
			frame = append(frame, preambleByte, preambleByte)
			frame = append(frame, a.body...)
			frame = append(frame, endOfMessage)
			events = append(events, accEvent{frame: frame})
			a.reset()

		default:
			a.body = append(a.body, b)
			if len(a.body) > a.max {
				events = append(events, accEvent{overflow: true})
				a.reset()
			}
		}
	}

	return events
}

// ---------------------------------------------------------------------------
// Splitting one frame
// ---------------------------------------------------------------------------

// parsedFrame is one frame split at its FRAMING boundaries and nowhere else.
//
// payload deliberately holds cn, then any sc, then any data, UNSPLIT: which
// bytes of a payload are sub-command and which are data is a per-command fact,
// so splitting it here would put a record-layout decision in the framing layer.
// Each handler splits its own.
type parsedFrame struct {
	to      byte
	from    byte
	payload []byte
}

// parseFrame splits a reassembled frame. It reports false for a frame too short
// to carry to, from and a command byte — which is malformed, not empty.
func parseFrame(frame []byte) (parsedFrame, bool) {
	if len(frame) < preambleLen+minBodyBytes+1 {
		return parsedFrame{}, false
	}
	body := frame[preambleLen : len(frame)-1]
	return parsedFrame{to: body[0], from: body[1], payload: body[2:]}, true
}

// frameAddressee reports who a frame is addressed to, if it carries an address
// byte at all — which a frame too short to parse may still do.
//
// It exists because the two rejection rules turn on exactly that: a MALFORMED
// frame addressed to the receiver is answered NG, whilst one addressed anywhere
// else draws no reply at all. Without this, a truncated frame could not be told
// apart from a truncated frame meant for some other radio.
func frameAddressee(frame []byte) (byte, bool) {
	if len(frame) < preambleLen+1+1 {
		return 0, false
	}
	return frame[preambleLen], true
}

// ---------------------------------------------------------------------------
// Replies
// ---------------------------------------------------------------------------

// buildReply wraps a payload as a radio -> controller frame:
// FE FE E0 <radio> <payload> FD.
func (r *Radio) buildReply(payload ...byte) []byte {
	out := make([]byte, 0, preambleLen+2+len(payload)+1)
	out = append(out, preambleByte, preambleByte, controllerAddr, r.radioAddr)
	out = append(out, payload...)
	return append(out, endOfMessage)
}

func (r *Radio) ackReply() []byte { return r.buildReply(okCode) }
func (r *Radio) ngReply() []byte  { return r.buildReply(ngCode) }

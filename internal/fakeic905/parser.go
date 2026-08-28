// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic905

// ---------------------------------------------------------------------------
// The wire, from the four printed frame diagrams
// ---------------------------------------------------------------------------

// The framing bytes, from the IC-905 CI-V REFERENCE GUIDE, PDF p.3 (folio 2),
// "◇ About the data format", which prints four complete frame diagrams:
//
//	Preamble           FE FE            ("Preamble code (fixed)")
//	End of message     FD               ("End of message code (fixed)")
//	Radio address      AC               ("Transceiver's default address")
//	Controller address E0               ("Controller's (PC's) default address")
//	Frame, PC -> radio FE FE AC E0 <cn> [<sc>] [data] FD
//	Frame, radio -> PC FE FE E0 AC <cn> [<sc>] [data] FD
//	OK  (ack)          FE FE E0 AC FB FD   ("OK code (fixed)")
//	NG  (reject)       FE FE E0 AC FA FD   ("NG code (fixed)")
//
// Every one of these is re-derived here rather than imported from core/civ,
// which also carries them. That is THE HARD RULE (doc.go): two independent
// spellings of one protocol, checked against each other, is what makes a
// systematic error in either one visible.
const (
	preambleByte   = 0xFE
	endOfMessage   = 0xFD
	radioAddr      = 0xAC
	controllerAddr = 0xE0
	// broadcastAddr is the `to` byte of a transceive frame. ASSUMED —
	// doc.go register entry 2. The reference prints four frames and none of
	// them is a broadcast.
	broadcastAddr = 0x00
	okCode        = 0xFB
	ngCode        = 0xFA
)

// The two commands this tier sends, from the command table on PDF p.6
// (folio 5) as the brief quotes it.
const (
	// cmdTransceiverID / subReadID: "Read transceiver ID", cn 19 sc 00. The
	// request carries NO data bytes.
	cmdTransceiverID = 0x19
	subReadID        = 0x00
	// cmdMemory / subMemoryContents: "Memory contents", cn 1A sc 00,
	// symmetric send and read.
	cmdMemory         = 0x1A
	subMemoryContents = 0x00
)

// preambleLen is how many preamble bytes a canonical frame carries. A frame
// arriving with more is understood (padding is tolerated) but the frame handed
// on carries exactly two.
const preambleLen = 2

// minBodyBytes is the shortest body that can be dispatched at all: `to`,
// `from`, `cn`. The OK and NG diagrams are exactly this long.
const minBodyBytes = 3

// maxBodyBytes caps how many bytes may accumulate between a preamble and an
// end-of-message byte before the reassembler gives up on the frame.
//
// THE DOCUMENT STATES NO SUCH LIMIT — ASSUMED, doc.go register entry 10. The
// cap is a property of a reader that must not grow without bound on a line that
// has come up mid-frame or gone mad; it is not a claim about the radio.
//
// The longest body this fake can legitimately receive is a memory set: to,
// from, cn, sc and the 68-byte printed block, 72 bytes in all. The cap is set
// well clear of that so that a consumer may also seed a deliberately oversized
// record with WithRecord and write it back without tripping the reader, which
// is the point of TestReassembler_TheCapIsNotHitByTheLongestFrameThisFakeCanReceive.
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
//   - A run of MORE than two FE opens a frame just the same: padding is
//     tolerated. The frame handed on carries the canonical two.
//   - An FE inside a body abandons the partial frame and starts a new one. FE
//     is the preamble code; it cannot occur inside a body, so seeing one means
//     the body in hand was truncated.
//   - An FD after a preamble ends the message, even with nothing between them.
//     "End of message code (fixed)" means what it says, and treating that FD as
//     an ordinary body byte would leave the reader looking for a second one
//     that is never coming.
//   - A body that reaches maxBodyBytes without an end-of-message byte produces
//     one overflow event, exactly one, and the reader then resynchronises on
//     the next preamble. It does not wedge, and it does not emit an event per
//     byte thereafter.
type reassembler struct {
	// body holds the bytes seen since the preamble, excluding it.
	body []byte
	// max is the body cap.
	max int
	// preambles counts the consecutive FE bytes seen. Two or more means a
	// frame is open; fewer means the reader is hunting for one.
	preambles int
}

func newReassembler(max int) *reassembler {
	if max <= 0 {
		max = maxBodyBytes
	}
	return &reassembler{max: max}
}

// reset returns the reassembler to hunting for a preamble.
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
			// Any preamble byte, wherever it lands, restarts the frame: a
			// partial body in hand was truncated by it. The count is capped,
			// so a long run of padding opens exactly one frame.
			a.body = a.body[:0]
			if a.preambles < preambleLen {
				a.preambles++
			}

		case a.preambles < preambleLen:
			// Noise: no preamble has opened a frame, so this byte is not part
			// of one. A lone FE followed by ordinary bytes lands here too, and
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
// frame addressed to AC is answered NG, whilst one addressed anywhere else
// draws no reply at all. Without this, a truncated frame could not be told
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

// buildReply wraps a payload as a radio -> PC frame: FE FE E0 AC <payload> FD.
func buildReply(payload ...byte) []byte {
	out := make([]byte, 0, preambleLen+2+len(payload)+1)
	out = append(out, preambleByte, preambleByte, controllerAddr, radioAddr)
	out = append(out, payload...)
	return append(out, endOfMessage)
}

// ackReply and ngReply are the two fixed codes.
func ackReply() []byte { return buildReply(okCode) }
func ngReply() []byte  { return buildReply(ngCode) }

// ---------------------------------------------------------------------------
// Dispatch
// ---------------------------------------------------------------------------

// handleEvent turns one reassembler event into whatever the fake should send,
// or nil for silence.
//
// SILENCE AND REJECTION ARE DIFFERENT ANSWERS HERE, and keeping them different
// is the point. A radio at another address never hears the frame at all and the
// controller times out; a radio that heard a frame it cannot honour says NG. A
// fake that answered both alike would make a driver's timeout branch
// untestable.
func (r *Radio) handleEvent(ev accEvent) []byte {
	if ev.overflow {
		// An over-length run is refused as its own event. The fake cannot know
		// who it was addressed to — the address is long gone behind the cap —
		// and answering NG is the honest reading of "something arrived that
		// could not be a frame". doc.go register entry 10.
		return ngReply()
	}

	r.recordFrame(ev.frame)

	to, ok := frameAddressee(ev.frame)
	if !ok || to != radioAddr {
		return nil
	}

	pf, ok := parseFrame(ev.frame)
	if !ok {
		return ngReply()
	}
	return r.handlePayload(pf.payload)
}

// refusedPrefixes are the command and sub-command forms this tier does not
// send, listed so that the refusal is VISIBLE here rather than merely falling
// through to the default.
//
// Several of them are real commands of a real IC-905 — 1A 01 is the "Band
// stacking register" whose diagram sits on PDF page 20, and 1A 05 heads the
// set-mode command table on PDF page 9 — so refusing them is this FAKE's tier
// policy, not a fact about the radio. doc.go register entry 9.
var refusedPrefixes = [][]byte{
	{0x1A, 0x01}, // Band stacking register (PDF p.20)
	{0x1A, 0x02},
	{0x1A, 0x05}, // set mode (PDF p.9, "SET > Connectors")
	{0x09},
	{0x0A},
	{0x0B},
	{0xA0},
}

func isRefusedPrefix(payload []byte) bool {
	for _, p := range refusedPrefixes {
		if len(payload) >= len(p) && string(payload[:len(p)]) == string(p) {
			return true
		}
	}
	return false
}

// handlePayload dispatches one payload — cn, then any sc, then any data — that
// arrived addressed to this radio. Everything it does not recognise is NG.
func (r *Radio) handlePayload(payload []byte) []byte {
	if len(payload) == 0 || isRefusedPrefix(payload) {
		return ngReply()
	}

	switch payload[0] {
	case cmdTransceiverID:
		return r.handleReadID(payload[1:])
	case cmdMemory:
		if len(payload) >= 2 && payload[1] == subMemoryContents {
			return r.handleMemoryContents(payload[2:])
		}
		return ngReply()
	default:
		return ngReply()
	}
}

// handleReadID answers "Read transceiver ID", cn 19 sc 00.
//
// The request carries NO data bytes, so one that does is malformed. The ANSWER
// carries the fake's configured identity token, whose value this package
// asserts nothing about: see WithIdentityToken and doc.go register entry 4.
func (r *Radio) handleReadID(rest []byte) []byte {
	if len(rest) != 1 || rest[0] != subReadID {
		return ngReply()
	}
	payload := make([]byte, 0, 2+len(r.identityToken))
	payload = append(payload, cmdTransceiverID, subReadID)
	payload = append(payload, r.identityToken...)
	return buildReply(payload...)
}

// handleMemoryContents answers "Memory contents", cn 1A sc 00, in both
// directions. data is everything after the sub-command.
//
// The four printed address bytes come first (indices 1-4: the group field and
// the channel field). What follows them is the record, and this fake NEVER
// LOOKS INSIDE IT. The only question it ever asks about a record is how long it
// is.
func (r *Radio) handleMemoryContents(data []byte) []byte {
	if len(data) < addressBytes {
		return ngReply()
	}

	addr := chanAddr{
		group:   [2]byte{data[0], data[1]},
		channel: [2]byte{data[2], data[3]},
	}
	rest := data[addressBytes:]

	// The clear form, from the D2 block: four address bytes, then a single FF,
	// then nothing ("⑤: “FF,” ⑥ ~ : None"). This tier does not send it, so
	// this fake refuses it — checked BEFORE the length rules so that the
	// refusal is the clear form's own, not an accident of a one-byte record
	// disagreeing with a held length. doc.go register entry 8.
	if len(rest) == clearFormLen-addressBytes && rest[0] == clearFormByte {
		return ngReply()
	}

	if len(rest) == 0 {
		return r.readChannel(addr)
	}
	return r.setChannel(addr, rest)
}

// readChannel answers a read of one channel.
//
// An UNOCCUPIED channel is answered NG. ASSUMED — doc.go register entry 1, lift
// ic905-R-14. Nothing in either artefact says what a real IC-905 does with a
// read of an empty channel; this is the fake's own register entry, and it is
// recorded there rather than presented as a finding.
func (r *Radio) readChannel(addr chanAddr) []byte {
	r.mu.Lock()
	st, ok := r.records[addr]
	r.mu.Unlock()
	if !ok {
		return ngReply()
	}

	payload := make([]byte, 0, 2+addressBytes+st.Length())
	payload = append(payload, cmdMemory, subMemoryContents)
	payload = append(payload, addr.group[0], addr.group[1], addr.channel[0], addr.channel[1])
	payload = append(payload, st.Record...)
	return buildReply(payload...)
}

// setChannel writes one channel, and is where the record-length rejection rule
// lives.
//
// This model's records come in exactly two lengths. A set whose record length
// is not the length the fake HOLDS FOR THAT CHANNEL is answered NG and stores
// nothing. A set to a channel the fake does not hold has no held length to
// disagree with, so it is accepted at whatever length arrived.
//
// The rule is about LENGTH ALONE. The fake stores the bytes raw and never
// interprets them, so it has no opinion about whether 64, 65, 39 or 7 is a
// sensible number — only about whether it matches what is already there.
func (r *Radio) setChannel(addr chanAddr, record []byte) []byte {
	stored := make([]byte, len(record))
	copy(stored, record)

	r.mu.Lock()
	defer r.mu.Unlock()

	if held, ok := r.records[addr]; ok && held.Length() != len(record) {
		return ngReply()
	}
	r.records[addr] = MemState{Record: stored}
	return ackReply()
}

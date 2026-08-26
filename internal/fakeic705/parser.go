// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

// This file is this package's INDEPENDENT reading of the CI-V frame grammar and
// of the two commands this fake answers. It shares no code with core/civ and
// must not (doc.go, THE HARD RULE): two independent implementations of one
// protocol, checked against each other, is what makes a systematic codec bug
// visible. Everything below is written from the frame grammar and the two
// transcripts under core/civ/ic705/testdata, never from this project's encoder.

// The wire's own bytes.
//
//   - preamble: a frame opens FE FE. A LEADING EXTRA FE IS PADDING and is
//     tolerated, so the opening run may be longer than two.
//   - terminator: FD closes a frame, and nothing else does.
//   - radioAddress: this radio's default CI-V address. A frame addressed
//     anywhere else is not this radio's business.
//   - controllerAddress: the controller's. Every answer this fake sends goes
//     from radioAddress to controllerAddress.
//   - broadcastAddress: the "to anyone" address an unsolicited frame carries.
//     This fake never answers one and only ever emits one (see WithNeverQuiet).
const (
	preamble          = 0xFE
	terminator        = 0xFD
	radioAddress      = 0xA4
	controllerAddress = 0xE0
	broadcastAddress  = 0x00
)

// The two reply codes, each carried alone in a six-byte frame.
const (
	codeOK = 0xFB // the manual's OK code
	codeNG = 0xFA // the manual's NG code
)

// The two commands this fake answers, and one it refuses by name.
//
// cmdBandStacking/subBandStacking is not implemented and is named here only
// because the unsolicited-frame emitters use it as a shape that no answer this
// radio owes anyone could be mistaken for. Transcription B records it as a
// separate diagram on the following page — "• Band stacking register / Command:
// 1A 01 ... It is a different command and is not a memory record" — which is
// the whole of what this package knows about it.
const (
	cmdTransceiverID = 0x19
	subTransceiverID = 0x00

	cmdMemoryContent = 0x1A
	subMemoryContent = 0x00

	cmdBandStacking = 0x1A
	subBandStacking = 0x01
)

// addressLen is the width of a memory-record data area's leading address: two
// packed-BCD bytes of memory group, then two of memory channel.
const addressLen = 4

// callChannelGroup is the decimal value of the printed group "0100" — the call
// channel group. Both transcripts record its channel vocabulary as 0000-0003
// ("0000, 0001: 144 C1, C2 | 0002, 0003: 430 C1, C2"), against 0000-0099 for
// every memory group.
const callChannelGroup = 100

// maxMemoryGroup and maxMemoryChannel are the memory groups' own ranges,
// transcribed: "0000 ~ 0099: Memory channel group", and channels "00 ~ 99".
const (
	maxMemoryGroup   = 99
	maxMemoryChannel = 99
	maxCallChannel   = 3
)

// transceiverIDPayload is what this fake answers a `19 00` read with. IT IS
// INVENTED, and it is the only byte in this package that answers a question
// about the radio's identity (doc.go, register entry 7; PROVENANCE.md).
//
// A7560-8EX-6 documents the command; the two transcripts read the memory-record
// pages of that document and neither carries an ID value, and no IC-705 has
// ever been asked. A4 — this radio's own default address — is chosen because it
// is the one byte a reader of a capture could not mistake for evidence about
// some other radio, and because nothing may match on it: the design's own probe
// records the ID reply in diagnostics and never matches it, so any value here
// is behaviourally equivalent and only its honesty is at stake.
var transceiverIDPayload = []byte{0xA4}

// maxAccumulatorBytes bounds the reassembler. The longest frame this radio can
// ever produce is 2 preamble + 2 addresses + 2 command + 4 address + 111 record
// + 1 terminator = 122 bytes, and the cap is a generous multiple of it so that
// a legitimately noisy lead-in is never mistaken for a runaway. See register
// entry 6: the cap and its resync are this package's own policy and no claim
// about any radio.
const maxAccumulatorBytes = 512

// accEvent is one thing the reassembler observed: a terminated byte run
// (frame), or the accumulator filling without one (overflow). Exactly one of
// the two is set.
type accEvent struct {
	frame    []byte
	overflow bool
}

// reassembler turns a byte stream into terminated runs. It is deliberately NOT
// a frame parser: it knows only the terminator and the cap, and hands whatever
// it accumulated up to parseFrame, which is where the grammar lives. Keeping
// the two apart is what lets a leading run of junk be delivered and refused
// rather than silently eaten.
type reassembler struct {
	buf []byte
	cap int
	// discarding is set after an overflow: bytes are dropped, without a second
	// report, until the next terminator resynchronises the stream.
	discarding bool
}

func newReassembler(capBytes int) *reassembler {
	return &reassembler{cap: capBytes}
}

// push feeds bytes in and returns the events they completed, in order.
func (a *reassembler) push(data []byte) []accEvent {
	var evs []accEvent
	for _, c := range data {
		if a.discarding {
			if c == terminator {
				a.discarding = false
			}
			continue
		}

		a.buf = append(a.buf, c)
		if c == terminator {
			evs = append(evs, accEvent{frame: append([]byte(nil), a.buf...)})
			a.buf = a.buf[:0]
			continue
		}
		if len(a.buf) > a.cap {
			a.buf = a.buf[:0]
			a.discarding = true
			evs = append(evs, accEvent{overflow: true})
		}
	}
	return evs
}

// parseFrame takes a terminated byte run apart into the grammar's own pieces:
// the destination and source addresses, and the payload between them and the
// terminator (the command byte, its sub-command, and the data area).
//
// It reports ok=false for a run that is not a CI-V frame at all — one with no
// two-byte preamble, or with no room for both addresses. Such a run carries no
// destination, so this radio cannot know it was meant for it, and the caller
// answers it with silence rather than a rejection (register entry 5).
//
// A frame WITH addresses and no payload parses fine, with an empty payload:
// that is a well-formed frame carrying no command, and the FA ladder is where
// it is judged, not here.
func parseFrame(raw []byte) (to, from byte, payload []byte, ok bool) {
	if len(raw) == 0 || raw[len(raw)-1] != terminator {
		return 0, 0, nil, false
	}
	body := raw[:len(raw)-1]

	pre := 0
	for pre < len(body) && body[pre] == preamble {
		pre++
	}
	if pre < 2 {
		return 0, 0, nil, false
	}
	body = body[pre:]

	if len(body) < 2 {
		return 0, 0, nil, false
	}
	return body[0], body[1], body[2:], true
}

// buildNAK returns the NG frame, written out byte by byte: preamble, to the
// controller, from this radio, the NG code, terminator.
func buildNAK() []byte {
	return []byte{preamble, preamble, controllerAddress, radioAddress, codeNG, terminator}
}

// buildACK returns the OK frame, the same six-byte shape carrying the OK code.
func buildACK() []byte {
	return []byte{preamble, preamble, controllerAddress, radioAddress, codeOK, terminator}
}

// buildIDAnswer returns the answer to a `19 00` read: the same command and
// sub-command, then this fake's invented ID payload.
func buildIDAnswer() []byte {
	out := []byte{preamble, preamble, controllerAddress, radioAddress, cmdTransceiverID, subTransceiverID}
	out = append(out, transceiverIDPayload...)
	return append(out, terminator)
}

// buildMemoryAnswer returns the answer to a `1A 00` read: the command, the
// four-byte address the answer is ABOUT, then the record, verbatim.
//
// The record is copied out unexamined and unmodified, whatever its length and
// whatever bytes it holds — INCLUDING an FD, which would truncate this frame at
// the far end. That is not repaired: a fake that quietly fixed a record an
// operator injected would hide the very thing such a record is injected to
// test.
func buildMemoryAnswer(addr, record []byte) []byte {
	out := make([]byte, 0, 7+len(addr)+len(record))
	out = append(out, preamble, preamble, controllerAddress, radioAddress, cmdMemoryContent, subMemoryContent)
	out = append(out, addr...)
	out = append(out, record...)
	return append(out, terminator)
}

// buildUnsolicited returns the frame the emitters push at nobody's request.
// See doc.go, register entry 9, and WithNeverQuiet.
func buildUnsolicited(to byte) []byte {
	return []byte{preamble, preamble, to, radioAddress, cmdBandStacking, subBandStacking, 0x00, 0x00, terminator}
}

// decodeAddress reads a data area's four leading bytes as memory group and
// memory channel, and applies the ranges the manual states.
//
// Two separate refusals, both reported the same way because the wire has only
// one refusal to report them with:
//
//   - a nibble that is not a decimal digit. The field is packed BCD — four
//     nibbles carrying four printed decimal digits — so 0x0A is not a number
//     this field can hold.
//   - a group or channel outside the printed vocabulary: groups 0000-0099 with
//     channels 0000-0099, and group 0100 (the call channel group) with channels
//     0000-0003. Anything else is not a slot.
func decodeAddress(addr []byte) (group, channel int, ok bool) {
	if len(addr) != addressLen {
		return 0, 0, false
	}
	group, ok = decodeBCD2(addr[0], addr[1])
	if !ok {
		return 0, 0, false
	}
	channel, ok = decodeBCD2(addr[2], addr[3])
	if !ok {
		return 0, 0, false
	}
	if !inRange(group, channel) {
		return 0, 0, false
	}
	return group, channel, true
}

// inRange reports whether the group and channel name a slot this radio has.
func inRange(group, channel int) bool {
	switch {
	case group == callChannelGroup:
		return channel >= 0 && channel <= maxCallChannel
	case group >= 0 && group <= maxMemoryGroup:
		return channel >= 0 && channel <= maxMemoryChannel
	default:
		return false
	}
}

// encodeAddress renders a group and channel as the four packed-BCD bytes the
// address field carries. It applies no range rule: the misbehaviour hook
// deliberately encodes addresses the radio would refuse to be asked for.
func encodeAddress(group, channel int) []byte {
	hi, lo := encodeBCD2(group)
	chi, clo := encodeBCD2(channel)
	return []byte{hi, lo, chi, clo}
}

// decodeBCD2 reads two packed-BCD bytes as a four-digit decimal number, most
// significant byte first. It refuses any nibble above 9.
func decodeBCD2(hi, lo byte) (int, bool) {
	d0, ok0 := nibbles(hi)
	d1, ok1 := nibbles(lo)
	if !ok0 || !ok1 {
		return 0, false
	}
	return d0*100 + d1, true
}

// nibbles reads one packed-BCD byte as a two-digit number.
func nibbles(b byte) (int, bool) {
	hi := int(b >> 4)
	lo := int(b & 0x0F)
	if hi > 9 || lo > 9 {
		return 0, false
	}
	return hi*10 + lo, true
}

// encodeBCD2 renders a number 0-9999 as two packed-BCD bytes. Its callers have
// already established the range (mustBeAddressable, or a decodeAddress round
// trip), so an out-of-range value here is a programming error and is masked
// rather than silently wrapped into a different slot.
func encodeBCD2(v int) (hi, lo byte) {
	if v < 0 || v > 9999 {
		panic("fakeic705: value does not fit two packed-BCD bytes")
	}
	return byte((v/1000)%10<<4 | (v/100)%10), byte((v/10)%10<<4 | v%10)
}

// handleFrame is the whole of this fake's reply policy for one terminated byte
// run. It returns the bytes to send, or nil for silence.
//
// SILENCE AND REJECTION ARE DIFFERENT ANSWERS and the difference is the address
// filter. A frame addressed to some other station, or a byte run that is not a
// frame at all, gets nothing: this radio has no standing to refuse a
// conversation it is not part of, and a bus with two radios on it would
// otherwise answer every frame twice. A frame addressed to THIS radio always
// gets an answer, and the answer is NG unless it is one of the two commands
// below, correctly shaped.
func (r *Radio) handleFrame(raw []byte) []byte {
	r.countFrame()

	to, _, payload, ok := parseFrame(raw)
	if !ok {
		return nil
	}
	if to != radioAddress {
		return nil
	}

	// The SOURCE address is not checked. Every answer goes to the controller
	// whoever asked, which is register entry 4.
	if len(payload) < 2 {
		return buildNAK()
	}
	cn, sc, data := payload[0], payload[1], payload[2:]

	switch {
	case cn == cmdTransceiverID && sc == subTransceiverID:
		return handleTransceiverID(data)
	case cn == cmdMemoryContent && sc == subMemoryContent:
		return r.handleMemoryContent(data)
	default:
		return buildNAK()
	}
}

// handleTransceiverID answers `19 00`. The request carries no data area, so one
// that carries anything is not the read this fake knows.
func handleTransceiverID(data []byte) []byte {
	if len(data) != 0 {
		return buildNAK()
	}
	return buildIDAnswer()
}

// handleMemoryContent answers `1A 00` in both its directions.
//
// The data area's first four bytes are the address. What follows them decides
// the direction: nothing at all is a READ, anything at all is a SET. A data
// area too short to hold an address is neither, and is refused.
func (r *Radio) handleMemoryContent(data []byte) []byte {
	if len(data) < addressLen {
		return buildNAK()
	}
	addr, record := data[:addressLen], data[addressLen:]

	if len(record) == 0 {
		return r.handleMemoryRead(addr)
	}

	// Counted BEFORE any refusal: this counter answers "was a write attempted?"
	// and a refused attempt is still an attempt (SetsSeen).
	r.countSet()
	return r.handleMemorySet(addr, record)
}

// handleMemoryRead answers a read: the record for an occupied slot, NG for an
// unwritten one and for an address outside the printed ranges.
func (r *Radio) handleMemoryRead(addr []byte) []byte {
	group, channel, ok := decodeAddress(addr)
	if !ok {
		return buildNAK()
	}
	record, occupied := r.lookupRecord(group, channel)
	if !occupied {
		return buildNAK()
	}

	// The armed misbehaviour is spent HERE and only here — on an answer that
	// actually carries a record. A read that drew NG above leaves it armed.
	answerAddr := addr
	if wrong, armed := r.takeWrongAddress(); armed {
		answerAddr = encodeAddress(wrong.Group, wrong.Channel)
	}
	return buildMemoryAnswer(answerAddr, record)
}

// handleMemorySet takes a set: it stores the record and answers OK, or refuses
// with NG and stores nothing.
//
// THE RECORD-LENGTH RULE IS ABSOLUTE. A record that is not RecordLen bytes is
// refused — never accepted, never truncated, never padded — whatever else about
// the frame is well formed. That is the rule the wire enforces; an operator
// seeding state through WithRecord or WithFactoryImage stands outside it
// entirely, deliberately (see those options).
func (r *Radio) handleMemorySet(addr, record []byte) []byte {
	group, channel, ok := decodeAddress(addr)
	if !ok {
		return buildNAK()
	}
	if len(record) != RecordLen {
		return buildNAK()
	}
	r.storeRecord(group, channel, record)
	return buildACK()
}

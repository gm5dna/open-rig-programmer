// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7610

// This file is the whole of this package's protocol knowledge: how a frame is
// recognised on a byte stream, and what each recognised frame is answered with.
// It was written from the framing skeleton printed in the IC-7610 CI-V
// Reference Guide and from two evidence artefacts transcribed off that guide's
// memory-content page. It consults no codec, and the author could not have:
// see doc.go, THE HARD RULE, and PROVENANCE.md.

// maxAccumulatorBytes bounds the input reassembler. A stream that reaches it
// without producing a frame is line noise, or a peer that has lost framing
// entirely, and the accumulator is dropped rather than grown.
//
// The drop is SILENT. CI-V has no rejection code for "I could not find a
// frame", and inventing one — an NG addressed to a controller this radio has
// not heard from — would put a frame on the wire that no radio would send.
const maxAccumulatorBytes = 4096

// frame is one complete CI-V frame lifted off the stream.
type frame struct {
	// raw is the frame's bytes AS RECEIVED, from the first byte of its
	// preamble run through its terminator inclusive — preamble padding and
	// all. It is what WithUSBEcho echoes, which is why "verbatim" is
	// achievable: nothing here is rebuilt.
	raw []byte

	to   byte
	from byte

	// data is everything between the address pair and the terminator: the
	// command byte, its sub-command if it has one, and its payload. Empty for
	// a frame that carries no command at all.
	data []byte
}

// cn returns the command byte, and whether the frame carries one.
func (f frame) cn() (byte, bool) {
	if len(f.data) == 0 {
		return 0, false
	}
	return f.data[0], true
}

// sc returns the sub-command byte, and whether the frame carries one.
func (f frame) sc() (byte, bool) {
	if len(f.data) < 2 {
		return 0, false
	}
	return f.data[1], true
}

// reassembler turns a byte stream into frames.
//
// It is deliberately its own implementation and not a call into anything: the
// address filter and the padding tolerance below are exactly the behaviours a
// driver's own framing is checked against, and sharing one implementation
// between the two sides would make a framing bug agree with itself.
type reassembler struct {
	buf []byte
	max int
}

func newReassembler(max int) *reassembler { return &reassembler{max: max} }

// push appends data and returns every complete frame now available, in order.
//
// Three behaviours here are the ones doc.go's "Framing" section promises, and
// each has a test of its own:
//
//   - Bytes before the first run of two 0xFE are LINE NOISE and are discarded
//     without comment. A real bus carries them; a radio does not answer them.
//   - A run of 0xFE longer than two is PREAMBLE PADDING. The whole run is
//     skipped and the byte after it is the frame's `to`. The guide's own worked
//     example frame is padded this way.
//   - The first 0xFD after the address pair ENDS the frame. Data bytes are not
//     escaped, so a payload containing 0xFD truncates its own frame. That is
//     the protocol as printed and this package does not paper over it.
func (a *reassembler) push(data []byte) []frame {
	a.buf = append(a.buf, data...)
	var out []frame

	for {
		// 1. Find the start of a preamble run: two consecutive 0xFE.
		start := -1
		for i := 0; i+1 < len(a.buf); i++ {
			if a.buf[i] == preambleByte && a.buf[i+1] == preambleByte {
				start = i
				break
			}
		}
		if start < 0 {
			// Nothing held can begin a frame. Keep only a trailing 0xFE,
			// which may yet turn out to be the first byte of a preamble whose
			// second byte has not arrived.
			if n := len(a.buf); n > 0 && a.buf[n-1] == preambleByte {
				a.buf = append(a.buf[:0], preambleByte)
			} else {
				a.buf = a.buf[:0]
			}
			break
		}
		if start > 0 {
			a.buf = append(a.buf[:0], a.buf[start:]...)
		}

		// 2. Skip the whole preamble run. Extra 0xFE bytes carry no meaning.
		k := 0
		for k < len(a.buf) && a.buf[k] == preambleByte {
			k++
		}
		if k+1 >= len(a.buf) {
			// Still inside the preamble, or the address pair has not both
			// arrived. Wait for more bytes.
			break
		}

		// 3. Find the terminator.
		end := -1
		for i := k + 2; i < len(a.buf); i++ {
			if a.buf[i] == terminatorByte {
				end = i
				break
			}
		}
		if end < 0 {
			break
		}

		out = append(out, frame{
			raw:  append([]byte(nil), a.buf[:end+1]...),
			to:   a.buf[k],
			from: a.buf[k+1],
			data: append([]byte(nil), a.buf[k+2:end]...),
		})
		a.buf = append(a.buf[:0], a.buf[end+1:]...)
	}

	if len(a.buf) > a.max {
		a.buf = a.buf[:0]
	}
	return out
}

// answerFrame builds an answer-direction frame around payload:
// FE FE E0 98 <payload...> FD. MANUAL-EVIDENCED shape.
func answerFrame(payload ...byte) []byte {
	out := make([]byte, 0, len(payload)+5)
	out = append(out, preambleByte, preambleByte, AddrController, AddrRadio)
	out = append(out, payload...)
	return append(out, terminatorByte)
}

// okAnswer and ngAnswer are the guide's two fixed codes, framed:
// FE FE E0 98 FB FD and FE FE E0 98 FA FD.
//
// Functions rather than package variables, so that a caller mutating a returned
// slice cannot poison every later reply.
func okAnswer() []byte { return answerFrame(CodeOK) }
func ngAnswer() []byte { return answerFrame(CodeNG) }

// floodFrame builds one flood frame addressed to `to`:
// FE FE <to> 98 19 00 <id token...> FD.
//
// It is the ID answer with its `to` byte swapped, deliberately, so that the two
// floods differ from each other in exactly the one byte that distinguishes the
// two line conditions, and so that neither invents a command this radio does
// not otherwise answer. See doc.go, "Two floods".
func (r *Radio) floodFrame(to byte) []byte {
	out := make([]byte, 0, len(r.idToken)+7)
	out = append(out, preambleByte, preambleByte, to, AddrRadio, cnID, scID)
	out = append(out, r.idToken...)
	return append(out, terminatorByte)
}

// handleFrame decides what one frame is answered with. A nil return is silence.
//
// The order of the two gates at the top is load-bearing and is stated in
// doc.go: the ECHO happens in serve(), BEFORE this function is called, because
// an echo is a property of the line; the ADDRESS FILTER happens here, first,
// because answering is a property of the radio. A frame addressed elsewhere is
// therefore echoed (if echo is on) and otherwise ignored completely — no
// answer, no state change, no CommandLog entry.
func (r *Radio) handleFrame(f frame) []byte {
	if f.to != AddrRadio {
		return nil
	}

	cn, ok := f.cn()
	if !ok {
		// Addressed to this radio, carrying no command at all. Refused: it is
		// a frame the radio was meant to act on and could not.
		return ngAnswer()
	}
	sc, _ := f.sc()
	r.logCommand(cn, sc)

	switch cn {
	case cnID:
		return r.handleID(f)
	case cnMemory:
		return r.handleMemory(f)

	// The refusals. Each is a decision with a reason; doc.go, "Deliberate
	// divergences", carries them, and PROVENANCE.md carries them again.
	case cnClear:
		// "Memory clear". A real IC-7610 would very likely honour this. This
		// fake refuses it so that any code path which ever emits one fails
		// loudly in a test rather than silently emptying a channel.
		return ngAnswer()
	case cnPower:
		// 18 01, power ON. Refused because a fake radio has no power state to
		// switch, and answering OK would assert one it does not have. Every
		// other sub-command of 18 lands here too: this radio has no power
		// surface at all, not merely no power-ON.
		return ngAnswer()
	}
	return ngAnswer()
}

// handleID answers 19 00.
//
// The COMMAND is manual-evidenced; the REPLY VALUE is not printed anywhere, and
// the token below is invented (options.go, defaultIDToken).
//
// A 19 with any other sub-command is refused, and so is a 19 00 carrying data:
// the guide prints this request's Data cell blank, so a request with data in it
// is not the request the guide prints.
func (r *Radio) handleID(f frame) []byte {
	if sc, ok := f.sc(); !ok || sc != scID {
		return ngAnswer()
	}
	if len(f.data) != 2 {
		return ngAnswer()
	}
	return answerFrame(append([]byte{cnID, scID}, r.idToken...)...)
}

// handleMemory answers the 1A family.
//
// Only 1A 00 is answered at all. 1A 05 — the menu surface this tier does not
// ship — is refused in any form, and so is every other sub-command.
func (r *Radio) handleMemory(f frame) []byte {
	sc, ok := f.sc()
	if !ok {
		return ngAnswer()
	}
	switch sc {
	case scMemory:
		return r.handleMemoryContent(f.data[2:])
	case scMenu:
		// 1A 05 in any form. Not a divergence: refusing the menu surface is
		// what this radio's tier means.
		return ngAnswer()
	}
	return ngAnswer()
}

// handleMemoryContent answers 1A 00, whose payload is a two-byte channel
// selector optionally followed by a record.
//
// The four outcomes, in the order they are decided:
//
//  1. Fewer than two payload bytes, or a selector that addresses nothing: NG.
//     The three addressable forms are 00 01 .. 00 99, 01 00 and 01 01, and the
//     page prints no others.
//  2. Selector alone: a READ. The stored record, at this radio's record length,
//     or NG if that channel has never been set. That an unset channel answers
//     NG is ASSUMED — see doc.go, "Empty channels" — and the READ REQUEST FORM
//     ITSELF IS ASSUMED: the document prints no 1A 00 read request at all.
//  3. Selector plus exactly one 0xFF byte: the CLEAR form the page prints.
//     REFUSED WITH NG, deliberately, and matched explicitly here so that the
//     refusal is a decision at a named place rather than a by-product of the
//     length check below.
//  4. Selector plus a record: a SET. OK if the record is exactly recordLen
//     bytes, NG otherwise.
func (r *Radio) handleMemoryContent(payload []byte) []byte {
	if len(payload) < 2 {
		return ngAnswer()
	}
	hi, lo := payload[0], payload[1]
	ch, ok := channelFor(hi, lo)
	if !ok {
		return ngAnswer()
	}
	rest := payload[2:]

	if len(rest) == 0 {
		m, set := r.readSlot(ch)
		if !set {
			return ngAnswer()
		}
		return answerFrame(append([]byte{cnMemory, scMemory, hi, lo}, m.Raw...)...)
	}

	if len(rest) == 1 && rest[0] == clearRecordByte {
		return ngAnswer()
	}

	if len(rest) != r.recordLen {
		return ngAnswer()
	}
	r.writeSlot(ch, MemState{Raw: rest})
	return okAnswer()
}

// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7800

// This file is the whole of this package's protocol knowledge: how a frame
// is recognised on a byte stream, and what each recognised frame is
// answered with. It was written from docs/superpowers/icom-matrices/
// ic7800-capability-matrix.md alone — see doc.go, THE QUARANTINE.

// maxAccumulatorBytes bounds the input reassembler. A stream that reaches it
// without producing a frame is dropped rather than grown; CI-V has no
// rejection code for "I could not find a frame".
const maxAccumulatorBytes = 4096

// frame is one complete CI-V frame lifted off the stream.
type frame struct {
	raw  []byte // as received, preamble and all — what WithUSBEcho echoes
	to   byte
	from byte
	data []byte // command byte, sub-command if any, and payload
}

func (f frame) cn() (byte, bool) {
	if len(f.data) == 0 {
		return 0, false
	}
	return f.data[0], true
}

func (f frame) sc() (byte, bool) {
	if len(f.data) < 2 {
		return 0, false
	}
	return f.data[1], true
}

// reassembler turns a byte stream into frames. Its own independent
// implementation — see doc.go, "Framing" — not a shared call with any
// production codec.
type reassembler struct {
	buf []byte
	max int
}

func newReassembler(max int) *reassembler { return &reassembler{max: max} }

func (a *reassembler) push(data []byte) []frame {
	a.buf = append(a.buf, data...)
	var out []frame

	for {
		start := -1
		for i := 0; i+1 < len(a.buf); i++ {
			if a.buf[i] == preambleByte && a.buf[i+1] == preambleByte {
				start = i
				break
			}
		}
		if start < 0 {
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

		k := 0
		for k < len(a.buf) && a.buf[k] == preambleByte {
			k++
		}
		if k+1 >= len(a.buf) {
			break
		}

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

// answerFrame builds an answer-direction frame: FE FE E0 6A <payload...> FD.
// MANUAL-EVIDENCED shape, matrix §3.4.
func answerFrame(payload ...byte) []byte {
	out := make([]byte, 0, len(payload)+5)
	out = append(out, preambleByte, preambleByte, AddrController, AddrRadio)
	out = append(out, payload...)
	return append(out, terminatorByte)
}

func okAnswer() []byte { return answerFrame(CodeOK) }
func ngAnswer() []byte { return answerFrame(CodeNG) }

// floodFrame builds one flood frame addressed to `to`:
// FE FE <to> 6A 19 00 <id token...> FD — the ID answer with `to` swapped.
// See doc.go, "Two floods".
func (r *Radio) floodFrame(to byte) []byte {
	out := make([]byte, 0, len(r.idToken)+7)
	out = append(out, preambleByte, preambleByte, to, AddrRadio, cnID, scID)
	out = append(out, r.idToken...)
	return append(out, terminatorByte)
}

// handleFrame decides what one frame is answered with. A nil return is
// silence.
//
// The ECHO happens in dispatch(), BEFORE this function is called. The
// ADDRESS FILTER happens here, first: a frame addressed elsewhere is
// therefore echoed (if echo is on) and otherwise ignored completely — no
// answer, no state change, no CommandLog entry. See doc.go, "Echo".
func (r *Radio) handleFrame(f frame) []byte {
	if f.to != AddrRadio {
		return nil
	}

	cn, ok := f.cn()
	if !ok {
		// Addressed to this radio, carrying no command at all.
		return ngAnswer()
	}
	sc, _ := f.sc()
	r.logCommand(cn, sc)

	switch cn {
	case cnID:
		return r.handleID(f)
	case cnMemory:
		return r.handleMemory(f)
	case cnClear:
		// "Memory clear" (matrix §3.13). Refused so any code path that
		// ever emits it fails loudly in a test rather than silently
		// emptying a simulated channel.
		return ngAnswer()
	case cnPower:
		// Power ON. Refused: a fake radio has no power state to switch.
		return ngAnswer()
	}
	return ngAnswer()
}

// handleID answers 19 00. The command is MANUAL-EVIDENCED (matrix
// §3.12(i)); the reply value is not printed anywhere, and the token below
// is invented (options.go, defaultIDToken).
func (r *Radio) handleID(f frame) []byte {
	if sc, ok := f.sc(); !ok || sc != scID {
		return ngAnswer()
	}
	if len(f.data) != 2 {
		return ngAnswer()
	}
	return answerFrame(append([]byte{cnID, scID}, r.idToken...)...)
}

// handleMemory answers the 1A family. Only 1A 00 is answered at all; 1A 05
// (the menu surface this tier does not ship) is refused in any form, and so
// is every other sub-command.
func (r *Radio) handleMemory(f frame) []byte {
	sc, ok := f.sc()
	if !ok {
		return ngAnswer()
	}
	switch sc {
	case scMemory:
		return r.handleMemoryContent(f.data[2:])
	case scMenu:
		return ngAnswer()
	}
	return ngAnswer()
}

// handleMemoryContent answers 1A 00, whose payload is a two-byte channel
// selector optionally followed by a record.
//
//  1. Fewer than two payload bytes, or an unaddressable selector: NG.
//  2. Selector alone: a READ. The stored record at this radio's record
//     length, or — per the radio's empty-channel mode — NG (default) or an
//     all-FF record (WithAllFFEmpty) if the channel has never been set.
//     Matrix §3.7/§3.8(a)/§3.8(b); the read request form itself is ASSUMED.
//  3. Selector plus exactly one 0xFF byte: the printed clear form.
//     REFUSED WITH NG, matched explicitly rather than falling through the
//     length check below (matrix §3.13).
//  4. Selector plus a record of exactly recordLen bytes: a SET. OK.
//  5. Selector plus a SHORTER record: refused by default, or — under
//     WithShortSetAccepted — accepted and zero-padded on the tail.
//  6. Selector plus a LONGER-than-recordLen payload: always refused.
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
			if r.allFFEmpty {
				m = MemState{Raw: make([]byte, r.recordLen)}
				for i := range m.Raw {
					m.Raw[i] = 0xFF
				}
			} else {
				return ngAnswer()
			}
		}
		return answerFrame(append([]byte{cnMemory, scMemory, hi, lo}, m.Raw...)...)
	}

	if len(rest) == 1 && rest[0] == clearRecordByte {
		return ngAnswer()
	}

	switch {
	case len(rest) == r.recordLen:
		r.writeSlot(ch, MemState{Raw: rest})
		return okAnswer()
	case len(rest) < r.recordLen && r.shortSetPad:
		padded := make([]byte, r.recordLen)
		copy(padded, rest)
		r.writeSlot(ch, MemState{Raw: padded})
		return okAnswer()
	}
	return ngAnswer()
}

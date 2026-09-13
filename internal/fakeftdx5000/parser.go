// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx5000

import (
	"bytes"
	"fmt"
	"strconv"
)

// validModeNibbles is matrix §1.3's twelve memory-record Mode values, in the
// manual's own legend order (layout:945-947/977-979): D, E and F
// (ModeAMN/ModePSK/ModeDATAFMN in the project's own 15-member enum, which
// this package never imports) are absent from this record.
const validModeNibbles = "123456789ABC"

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func allDigits(b []byte) bool {
	for _, c := range b {
		if !isDigit(c) {
			return false
		}
	}
	return len(b) > 0
}

func validMode(b byte) bool { return bytes.IndexByte([]byte(validModeNibbles), b) >= 0 }

func validClarSign(b byte) bool { return b == '+' || b == '-' }

func validCTCSSState(b byte) bool { return b == '0' || b == '1' || b == '2' }

func validShift(b byte) bool { return b == '0' || b == '1' || b == '2' }

func validBoolFlag(b byte) bool { return b == '0' || b == '1' }

// parseSlot validates and returns a 3-digit slot string in matrix §1.5's
// range, 001-117 (both the 99 regular channels and the 18 PMS pairs).
func parseSlot(b []byte) (string, bool) {
	if len(b) != 3 || !allDigits(b) {
		return "", false
	}
	n, err := strconv.Atoi(string(b))
	if err != nil || n < 1 || n > 117 {
		return "", false
	}
	return string(b), true
}

// reassembler accumulates bytes off the wire and splits them into complete
// frames on the ';' terminator (layout:110's stated rule). See doc.go's
// ASSUMED register entry 3, THE FRAME ACCUMULATOR'S CAP AND RESYNC, for
// the overflow behaviour.
type reassembler struct {
	buf []byte
	max int
}

func newReassembler(max int) *reassembler { return &reassembler{max: max} }

// push appends chunk and returns every complete frame it closes off (the
// bytes between the previous terminator and this one, terminator
// excluded). A frame may be empty (back-to-back ";;" on the wire).
func (a *reassembler) push(chunk []byte) [][]byte {
	a.buf = append(a.buf, chunk...)
	var frames [][]byte
	for {
		i := bytes.IndexByte(a.buf, ';')
		if i < 0 {
			break
		}
		frame := make([]byte, i)
		copy(frame, a.buf[:i])
		frames = append(frames, frame)
		a.buf = a.buf[i+1:]
	}
	if len(a.buf) > a.max {
		a.buf = nil // drop and resync — THE FRAME ACCUMULATOR'S CAP AND RESYNC
	}
	return frames
}

// handleFrame dispatches one terminator-stripped frame to its command
// handler. An unrecognised opcode — anything this package does not model —
// is silently ignored: doc.go's ASSUMED register entry 2, OUT-OF-RANGE AND
// MALFORMED REQUESTS ARE SILENT.
func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) < 2 {
		return nil
	}
	switch string(frame[:2]) {
	case "ID":
		return r.handleID(frame[2:])
	case "MR":
		return r.handleMR(frame[2:])
	case "MW":
		r.handleMW(frame[2:])
		return nil
	default:
		return nil
	}
}

// handleID answers the fixed CAT ID. ID has no Set form and no parameters
// on its Read request (layout:769-775): anything other than a bare "ID;"
// is out of scope and silently ignored.
func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return nil
	}
	return []byte(fmt.Sprintf("ID%s;", catID))
}

// buildMRAnswer renders slot's state as the full 27-byte MR answer
// (matrix §2, layout:945-952). P7 (Kind) is always '1' (Memory) — see
// doc.go's note beside the ASSUMED register, this is read off the legend
// directly and is not a guess.
func buildMRAnswer(slot string, s MemState) []byte {
	const memoryKind = '1' // P7 Kind, MR answer: always Memory — see doc.go
	return []byte(fmt.Sprintf("MR%s%08d%c%04d%c%c%c%c%c%02d%c;",
		slot, s.FreqHz, s.ClarSign, s.ClarHz,
		boolByte(s.RxClar), boolByte(s.TxClar), s.Mode, memoryKind, s.CTCSSState,
		s.ToneIndex, s.Shift))
}

func boolByte(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}

// handleMR answers a memory-channel read request: "MR" + a 3-digit slot +
// ";" (layout:944). A malformed or out-of-range request gets no reply —
// OUT-OF-RANGE AND MALFORMED REQUESTS ARE SILENT (entry 2). A valid,
// never-written slot answers the zero record — EMPTY-SLOT ANSWERS (entry
// 1).
func (r *Radio) handleMR(body []byte) []byte {
	slot, ok := parseSlot(body)
	if !ok {
		return nil
	}
	r.mu.Lock()
	s, written := r.slots[slot]
	r.mu.Unlock()
	if !written {
		s = zeroState
	}
	return buildMRAnswer(slot, s)
}

// mwBodyLen is the length of an MW frame with "MW" and the terminator
// already stripped: slot(3) + freq(8) + clarSign(1) + clarMag(4) +
// rxClar(1) + txClar(1) + mode(1) + kind(1) + ctcssState(1) + tone(2) +
// shift(1) = 24 (matrix §2, 27 bytes total less "MW" and ";").
const mwBodyLen = 24

// parseMWFrame validates body against matrix §2's Set-frame layout and
// returns the slot and decoded state. kind (P7) must be the manual's fixed
// literal '0' ("P7 0: (Fixed)", layout:981) — anything else is treated as
// malformed, the same as an out-of-range digit.
func parseMWFrame(body []byte) (slot string, s MemState, ok bool) {
	if len(body) != mwBodyLen {
		return "", MemState{}, false
	}
	slot, ok = parseSlot(body[0:3])
	if !ok {
		return "", MemState{}, false
	}
	freqStr := body[3:11]
	if !allDigits(freqStr) {
		return "", MemState{}, false
	}
	freq, err := strconv.ParseUint(string(freqStr), 10, 32)
	if err != nil {
		return "", MemState{}, false
	}
	clarSign := body[11]
	if !validClarSign(clarSign) {
		return "", MemState{}, false
	}
	clarMagStr := body[12:16]
	if !allDigits(clarMagStr) {
		return "", MemState{}, false
	}
	clarMag, err := strconv.ParseUint(string(clarMagStr), 10, 16)
	if err != nil || clarMag > 9999 {
		return "", MemState{}, false
	}
	rxClar := body[16]
	if !validBoolFlag(rxClar) {
		return "", MemState{}, false
	}
	txClar := body[17]
	if !validBoolFlag(txClar) {
		return "", MemState{}, false
	}
	mode := body[18]
	if !validMode(mode) {
		return "", MemState{}, false
	}
	kind := body[19]
	if kind != '0' {
		return "", MemState{}, false
	}
	ctcssState := body[20]
	if !validCTCSSState(ctcssState) {
		return "", MemState{}, false
	}
	toneStr := body[21:23]
	if !allDigits(toneStr) {
		return "", MemState{}, false
	}
	tone, err := strconv.ParseUint(string(toneStr), 10, 8)
	if err != nil || tone > 49 {
		return "", MemState{}, false
	}
	shift := body[23]
	if !validShift(shift) {
		return "", MemState{}, false
	}

	return slot, MemState{
		FreqHz:     uint32(freq),
		ClarSign:   clarSign,
		ClarHz:     uint16(clarMag),
		RxClar:     rxClar == '1',
		TxClar:     txClar == '1',
		Mode:       mode,
		CTCSSState: ctcssState,
		ToneIndex:  uint8(tone),
		Shift:      shift,
	}, true
}

// handleMW stores a memory-channel write. MW has no answer at all — the
// Control Command List's own ANS column is "X" (layout:108) — so this never
// returns a reply. An invalid frame is discarded rather than stored:
// AN INVALID MW SET IS DISCARDED, NOT ACKNOWLEDGED (entry 4).
func (r *Radio) handleMW(body []byte) {
	slot, s, ok := parseMWFrame(body)
	if !ok {
		return
	}
	r.mu.Lock()
	r.slots[slot] = s
	r.mu.Unlock()
}

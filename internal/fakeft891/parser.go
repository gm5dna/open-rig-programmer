// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

// This file is fakeft891's own, independent byte-level CAT parser and reply
// builder for the FT-891. It is derived from that radio's own position charts
// in the FT-891 CAT Operation Reference Book, rev 1909-C — the Control
// Command List on printed folio 3, whose rows are cited line by line beside
// each section below, and the per-command charts cited with them — and NOT
// from core/cat. See doc.go for why that
// independence matters and for the full ASSUMED register; individual assumed
// points are flagged inline, next to the code that implements them.
//
// The manual itself is gitignored (docs/fixtures-private/manuals/), so the
// line references here are citations in the sense core/cat/ft891/doc.go uses
// them: they name where the chart is, they are not links.

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;" — an unattributed
// generic command failure. Every refusal in this file answers with it and
// nothing else: an empty slot, an out-of-inventory slot, a malformed frame,
// an unknown command and an overflowed accumulator are indistinguishable to
// the host, which is the whole of the convention.
//
// THE CONVENTION ITSELF IS INHERITED AND NO FT-891 LINE IS CITED FOR IT
// ANYWHERE IN THIS REPOSITORY — doc.go's register entry THE "?;" REJECTION
// CONVENTION ITSELF IS INHERITED. It is core/cat's ErrRejected, adopted from
// the FT-710's reference (core/cat/errors.go:10-19).
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy, not a manual figure (doc.go's register entry THE
// FRAME ACCUMULATOR'S CAP AND RESYNC).
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means.
//
// Overflow behaviour: once more than maxAccumulatorBytes bytes have
// accumulated without completing a frame, push reports one overflow event —
// the caller replies "?;" for it — and discards every byte from that point up
// to and including the next ';', then resumes normal framing
// (TestAccumulatorOverflowRejectsOnceAndResyncs). The zero value is not
// usable; construct with newReassembler.
type reassembler struct {
	buf       []byte
	max       int
	resyncing bool
}

func newReassembler(max int) *reassembler {
	if max <= 0 {
		max = maxAccumulatorBytes
	}
	return &reassembler{max: max}
}

// accEvent is one unit reassembler.push hands back: either a complete frame
// (terminator included) or an overflow signal (frame == nil, overflow true).
type accEvent struct {
	frame    []byte
	overflow bool
}

// push appends chunk to the internal buffer, byte by byte, and returns, in
// arrival order, every complete frame and overflow event it produced.
func (a *reassembler) push(chunk []byte) []accEvent {
	var events []accEvent
	for _, b := range chunk {
		if a.resyncing {
			if b == ';' {
				a.resyncing = false
			}
			continue
		}
		a.buf = append(a.buf, b)
		if b == ';' {
			frame := make([]byte, len(a.buf))
			copy(frame, a.buf)
			events = append(events, accEvent{frame: frame})
			a.buf = a.buf[:0]
			continue
		}
		if len(a.buf) > a.max {
			events = append(events, accEvent{overflow: true})
			a.buf = a.buf[:0]
			a.resyncing = true
		}
	}
	return events
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func validBoolFlagByte(b byte) bool { return b == '0' || b == '1' }

// --- ID (availability 147; frames 762-770) ---
//
// X O O X: no Set, Read "ID;", a 7-byte Answer. The VALUE is this radio's own
// CAT ID — the ID block prints "P1 0650: FT-891" (ft891_layout.txt:763) —
// and it is the one byte-level difference a probe turns into a
// *driver.WrongRadioError.

func buildIDAnswer() []byte { return []byte("ID0650;") }

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return buildIDAnswer()
}

// --- AI (availability 117; frames 226-235) ---
//
// O O O O. Set and Answer are 4 bytes, Read is "AI;". AI-set is
// fire-and-forget. This fake never PUSHES anything unsolicited whatever AI is
// set to: no FT-891's AI behaviour has been observed, and the engine's
// drain-to-quiet discipline is already exercised against internal/fakeradio,
// whose own AI-flood facts are the FT-710's. THAT SUPPRESSION IS AN
// ASSUMPTION AND IS REGISTERED — doc.go's entry AUTOMATIC-INFORMATION
// SUPPRESSION. It is modelling silence as the honest default, not a claim
// that this radio is silent.
//
// core/transport.Engine.Init opens every session with an AI-off Set, so this
// handler's silent-accept path is on the critical path of every fake session.

func buildAIAnswer(ai byte) []byte { return []byte{'A', 'I', ai, ';'} }

func (r *Radio) handleAI(body []byte) []byte {
	switch len(body) {
	case 0:
		r.mu.Lock()
		ai := r.ai
		r.mu.Unlock()
		return buildAIAnswer(ai)
	case 1:
		if !validBoolFlagByte(body[0]) {
			return rejection
		}
		r.mu.Lock()
		r.ai = body[0]
		r.mu.Unlock()
		return nil // fire-and-forget success
	}
	return rejection
}

// --- Top-level dispatch ---

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a fire-and-forget
// success, or a non-nil frame — a real answer, or rejection — otherwise.
// Unknown and garbled commands fall through to rejection.
//
// COMMAND NAMES ARE MATCHED IN UPPER CASE ONLY, and that is an ASSUMPTION on
// this radio — doc.go's register entry COMMAND NAMES ARE UPPER CASE ONLY,
// pinned by TestCommandNamesAreUpperCaseOnly. The FTdx10's manual states the
// either-case leniency in terms (ftdx10_layout.txt:160-161) and
// internal/fakedx10 accepts either case on that line; nothing in this
// repository cites such a sentence for the FT-891, and the transcription of
// this manual's own folio-2 "Control Command" prose
// (core/cat/ft891/testdata/provenance.md, "Pages read", PDF page 3) records
// no statement about case at all. Strictness is the fail-loud direction and
// costs nothing: every frame core/cat builds is upper case.
func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) == 0 || frame[len(frame)-1] != ';' {
		return rejection // defensive: the reassembler never hands us this
	}
	body := frame[:len(frame)-1]
	if len(body) < 2 {
		return rejection
	}
	cmd := [2]byte{body[0], body[1]}
	rest := body[2:]

	switch cmd {
	case [2]byte{'I', 'D'}:
		return r.handleID(rest)
	case [2]byte{'A', 'I'}:
		return r.handleAI(rest)
	default:
		return rejection
	}
}

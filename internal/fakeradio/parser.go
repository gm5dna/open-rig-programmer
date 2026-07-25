// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import "fmt"

// This file is fakeradio's own, independent byte-level CAT parser and
// reply builder. It is derived directly from the FT-710 CAT protocol
// reference (protocol reference doc, "MR/MW/MT/MC" and "General framing"
// sections) — NOT from core/cat. See doc.go for why that independence
// matters and for the full ASSUMED register; individual ASSUMED points
// are also flagged inline, next to the code that implements them.

// --- General framing ---

// rejection is the radio's one and only NAK, "?;" (reference: "The only
// NAK is ?; — an unattributed generic command failure"; golden vector
// G12). HW-CONFIRMED 2026-07-13 (see docs/hardware-notes.md
// §Empty/out-of-range slots): the live radio answered "?;", byte for
// byte, for every rejection case probed — empty slots, a
// grammatically-valid-but-out-of-inventory slot number ("MR100;"), and
// MT reads of never-touched slots alike. No case produced a different or
// more specific NAK; the manual's "one unattributed NAK" claim held for
// every rejection this session tried.
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap. ASSUMED: the brief
// specifies "~256"; the exact figure is not in the manual.
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames (framing only — see doc.go "General framing").
// It is the fake's OWN accumulator, independent of
// core/cat.FrameAccumulator: same general shape (bounded, byte-at-a-time,
// splits on ';'), but re-derived and re-implemented from the protocol
// reference rather than imported.
//
// Overflow behaviour (ASSUMED, see doc.go register): once more than
// maxAccumulatorBytes bytes have accumulated without completing a frame,
// push reports one overflow event (the caller replies "?;" for it) and
// discards every byte from that point up to and including the next ';',
// then resumes normal framing. The zero value is not usable; construct
// with newReassembler.
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

// accEvent is one unit reassembler.push hands back: either a complete
// frame (terminator included, like frame below) or an overflow signal
// (frame == nil, overflow == true).
type accEvent struct {
	frame    []byte
	overflow bool
}

// push appends chunk to the internal buffer, byte by byte, and returns,
// in arrival order, every complete frame and overflow event chunk
// produced. See the reassembler doc comment for overflow/resync
// semantics.
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

// --- Slot grammar ---
//
// Reference, "Slot codes (3 bytes on the wire)":
//   001-099  memory channels M-01..M-99
//   P1L-P9U  PMS pairs (9 lower/upper pairs)
//   5xx      60m channels (region-dependent; ASSUMED 501.. numbering)
//   EMG      Alaska emergency channel
//   000      answer-only ("VFO or MT or QMB"); never a valid request slot
//            in this fake (ASSUMED — see doc.go register).

// slotKind classifies a 3-byte slot code.
type slotKind int

const (
	slotInvalid slotKind = iota // malformed: not one of the forms below
	slotVFO000                  // "000" — answer-only, never a valid request
	slotMemory                  // 001-099
	slotPMS                     // P1L-P9U
	slot60m                     // 5xx (500-599; which values are POPULATED is a state question, see image.go)
	slotEMG                     // "EMG"
)

// parseSlotForm classifies s per the slot grammar above. It is a pure
// grammar check: it says nothing about whether the slot is populated.
func parseSlotForm(s string) slotKind {
	if len(s) != 3 {
		return slotInvalid
	}
	if s == "EMG" {
		return slotEMG
	}
	if s == "000" {
		return slotVFO000
	}
	if isDigit(s[0]) && isDigit(s[1]) && isDigit(s[2]) {
		n := int(s[0]-'0')*100 + int(s[1]-'0')*10 + int(s[2]-'0')
		switch {
		case n >= 1 && n <= 99:
			return slotMemory
		case s[0] == '5':
			return slot60m
		default:
			return slotInvalid // e.g. "100", "600"..."999", "200"..."499"
		}
	}
	if s[0] == 'P' && s[1] >= '1' && s[1] <= '9' && (s[2] == 'L' || s[2] == 'U') {
		return slotPMS
	}
	return slotInvalid
}

// mwAllowedSlot reports whether kind is a slot MW-set may target: memory
// channels and PMS pairs only (reference, "MW": "P1 restricted to
// 001-099, P1L-P9U (no 5xx, no EMG)").
func mwAllowedSlot(kind slotKind) bool {
	return kind == slotMemory || kind == slotPMS
}

// mrReadableSlot reports whether kind is a slot MR/MT/MC may target
// (reference slot table: 001-099, P1L-P9U, 5xx, EMG all have a "✓" for
// MR/MT/MC; 000 is answer-only and never a valid request in this fake).
func mrReadableSlot(kind slotKind) bool {
	switch kind {
	case slotMemory, slotPMS, slot60m, slotEMG:
		return true
	}
	return false
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// --- Field validators (wire-level; lenient, per "the fake models the
// radio, not our policy") ---

func validModeWireByte(b byte) bool {
	switch {
	case b == '0':
		return true // "-" / unset — reference: "appears in reads of odd states"; parser accepts it
	case b >= '1' && b <= '9':
		return true
	case b >= 'A' && b <= 'F':
		return true
	}
	return false
}

func validKindByte(b byte) bool { return b >= '0' && b <= '5' }

// mwKindAccepted reports whether b is a P7 kind byte the radio accepts on
// an MW write. HW-CONFIRMED 2026-07-13 (M5b write trials against
// Stuart's real UK FT-710, docs/hardware-notes.md): kind '0' (VFO) and
// kind '5' (PMS) both produced an immediate "?;" rejection — the kind
// '5' rejection held even when writing to a PMS slot, which this
// project's former ASSUMED pairing (and the manual's own worked example)
// implied should be accepted there; the radio requires kind '1'
// (KindMemory) on every MW write regardless of slot bank. Kind '2'/'3'/'4'
// were never probed on write and remain accepted here, deliberately:
// this mirrors exactly what M5b tested, not a blanket "only '1' is ever
// accepted" policy the trials did not establish (see doc.go register
// item 9).
func mwKindAccepted(b byte) bool {
	switch b {
	case '0', '5':
		return false
	default:
		return validKindByte(b)
	}
}
func validCTCSSByte(b byte) bool    { return b >= '0' && b <= '2' }
func validShiftByte(b byte) bool    { return b >= '0' && b <= '2' }
func validBoolFlagByte(b byte) bool { return b == '0' || b == '1' }

func validClarSign(b byte) bool { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude
// field that is in range (0000-9990 Hz) and on a 10 Hz step, per
// reference: "P3: clarifier: +/- then 4-digit offset 0000-9990 Hz".
// Enforced at the wire level (not just by an internal builder), since the
// range and step are stated directly in the manual, not merely an
// internal convenience — ASSUMED that the radio itself rejects
// out-of-range/non-step values rather than silently rounding them (see
// doc.go register).
func validClarMagDigits(s string) bool {
	if len(s) != 4 {
		return false
	}
	n := 0
	for i := 0; i < 4; i++ {
		if !isDigit(s[i]) {
			return false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n <= 9990 && n%10 == 0
}

// validTag reports whether tag is acceptable as an MT tag value.
// SAFETY-CRITICAL (reference, "MT" section): printable ASCII 0x20-0x7E,
// EXCLUDING ';' (0x3B, the frame terminator — accepting it would make
// command injection possible), all control bytes rejected, and validated
// by BYTE length <= 12 (not rune count, so a tag containing multi-byte
// UTF-8 cannot smuggle more than 12 bytes past a rune-counted check).
// Exact radio-accepted charset is otherwise ASSUMED pending M5a (see
// doc.go register); this function's job today is to make command
// injection impossible, not to be a definitive oracle for what real
// hardware accepts.
func validTag(tag []byte) bool {
	if len(tag) > 12 {
		return false
	}
	for _, b := range tag {
		if b < 0x20 || b > 0x7E || b == ';' {
			return false
		}
	}
	return true
}

// --- Field encoders (semantic builders — used by image.go to construct
// human-readable factory data; NOT used on the hot reply-building path,
// which concatenates already-validated MemState fields directly. These
// are the "builders" the protocol reference's rejection-case table means
// by "clarifier 9995 (not a 10 Hz step)" / "frequency needing >9 digits"
// / "mode code 0 in a build request".) ---

// encodeFreqDigits converts hz to the 9-digit ASCII frequency field (P2
// of MR/MW; reference pos 6-14). It refuses hz values that would need
// more than 9 digits.
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 999_999_999 {
		return "", fmt.Errorf("fakeradio: frequency %d Hz needs more than 9 digits", hz)
	}
	return fmt.Sprintf("%09d", hz), nil
}

// encodeClarifier converts sign ('+' or '-') and magHz into the clarifier
// sign+magnitude fields (P3; reference pos 15-19): magHz must be
// 0-9990 Hz and a multiple of 10 (reference: "4-digit offset
// 0000-9990 Hz").
func encodeClarifier(sign byte, magHz uint16) (signOut byte, magOut string, err error) {
	if !validClarSign(sign) {
		return 0, "", fmt.Errorf("fakeradio: clarifier sign %q invalid", sign)
	}
	if magHz > 9990 {
		return 0, "", fmt.Errorf("fakeradio: clarifier magnitude %d Hz exceeds 9990", magHz)
	}
	if magHz%10 != 0 {
		return 0, "", fmt.Errorf("fakeradio: clarifier magnitude %d Hz is not a 10 Hz step", magHz)
	}
	return sign, fmt.Sprintf("%04d", magHz), nil
}

// validModeBuildByte reports whether m is a mode nibble a BUILDER may
// deliberately emit. '0' ("-"/unset) is documented as parse-accept-only:
// reference, mode table footer: "'0' = '-' (unset); appears in reads of
// odd states; builder rejects it, parser accepts it."
func validModeBuildByte(m byte) bool {
	return validModeWireByte(m) && m != '0'
}

// --- MR: MEMORY CHANNEL READ (reference "MR" section) ---
//
// Read frame (6 bytes): "MR" + 3-byte slot + ";".
// Answer frame (28 bytes), 0-indexed byte offsets re-derived from the
// reference's 1-indexed position table:
//
//	pos(1-idx)  field                 0-idx bytes
//	1-2         cmd "MR"              [0:2]
//	3-5         P1 slot                [2:5]
//	6-14        P2 freq (9 digits)     [5:14]
//	15          P3 clar sign           14
//	16-19       P3 clar mag (4 digits) [15:19]
//	20          P4 RX clar             19
//	21          P5 TX clar             20
//	22          P6 mode                21
//	23          P7 kind                22
//	24          P8 CTCSS               23
//	25-26       P9 "00" fixed          [24:26]
//	27          P10 shift              26
//	28          ';'                    27
//
// Verified against golden vectors G4 ("MR001007000000+000000110000;")
// and G6 ("MRP1L001810000+000000150000;") during derivation.

func boolFlagByte(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}

// buildMRAnswer concatenates an already-validated MemState into a 28-byte
// MR answer frame. It trusts its input (state stored via handleMW/the
// factory image is validated at write time) and does not itself return
// an error; see the field encoders above for the validating builders.
func buildMRAnswer(slot string, s MemState) []byte {
	out := make([]byte, 0, 28)
	out = append(out, 'M', 'R')
	out = append(out, slot...)
	out = append(out, s.Freq...)
	out = append(out, s.ClarSign)
	out = append(out, s.ClarMag...)
	out = append(out, boolFlagByte(s.RXClar))
	out = append(out, boolFlagByte(s.TXClar))
	out = append(out, s.Mode)
	out = append(out, s.Kind)
	out = append(out, s.CTCSS)
	out = append(out, '0', '0')
	out = append(out, s.Shift)
	out = append(out, ';')
	return out
}

// --- MW: MEMORY CHANNEL WRITE (reference "MW" section) ---
//
// Set frame: identical 28-byte layout to the MR answer, with "MW" in
// place of "MR"; P1 restricted to 001-099, P1L-P9U (mwAllowedSlot).
// Fire-and-forget: success produces no reply at all (reference,
// "General framing": "MW ... produce NO acknowledgement on success").
//
// P7 on write: HW-CONFIRMED 2026-07-13 (M5b write trials, doc.go
// register item 9) — kind '0' and '5' are REJECTED (mwKindAccepted);
// every OTHER valid P7 digit ('1'-'4') is stored verbatim, exactly as
// arrived — this fake does not correct, default, or second-guess it.

// mwBodyLen is the length of an MW frame's body (everything after "MW"
// and before the trailing ';'): 3(slot) + 9(freq) + 1(sign) + 4(mag) +
// 1(rx) + 1(tx) + 1(mode) + 1(kind) + 1(ctcss) + 2(P9) + 1(shift) = 25.
const mwBodyLen = 25

// MW body field offsets within the 25-byte body (see mwBodyLen).
const (
	mwSlotStart, mwSlotEnd       = 0, 3
	mwFreqStart, mwFreqEnd       = 3, 12
	mwClarSign                   = 12
	mwClarMagStart, mwClarMagEnd = 13, 17
	mwRXClar                     = 17
	mwTXClar                     = 18
	mwMode                       = 19
	mwKind                       = 20
	mwCTCSS                      = 21
	mwP9Start, mwP9End           = 22, 24
	mwShift                      = 24
)

// handleMW validates and applies an MW-set body (the frame's bytes after
// "MW" and before the trailing ';'). It returns the reply to send (nil
// for fire-and-forget success, rejection for any validation failure) and
// mutates r's stored state on success. Caller holds no lock; handleMW
// takes r.mu itself.
func (r *Radio) handleMW(body []byte) []byte {
	if len(body) != mwBodyLen {
		return rejection
	}
	slot := string(body[mwSlotStart:mwSlotEnd])
	if !mwAllowedSlot(parseSlotForm(slot)) {
		return rejection
	}
	freq := body[mwFreqStart:mwFreqEnd]
	for _, b := range freq {
		if !isDigit(b) {
			return rejection
		}
	}
	sign := body[mwClarSign]
	if !validClarSign(sign) {
		return rejection
	}
	mag := string(body[mwClarMagStart:mwClarMagEnd])
	if !validClarMagDigits(mag) {
		return rejection
	}
	rx, tx := body[mwRXClar], body[mwTXClar]
	if !validBoolFlagByte(rx) || !validBoolFlagByte(tx) {
		return rejection
	}
	mode := body[mwMode]
	if !validModeWireByte(mode) {
		return rejection
	}
	kind := body[mwKind]
	if !mwKindAccepted(kind) {
		return rejection
	}
	ctcss := body[mwCTCSS]
	if !validCTCSSByte(ctcss) {
		return rejection
	}
	if string(body[mwP9Start:mwP9End]) != "00" {
		return rejection
	}
	shift := body[mwShift]
	if !validShiftByte(shift) {
		return rejection
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	existing := r.slots[slot] // zero value if absent — fine, we only reuse Tag/TagDisplay
	r.slots[slot] = MemState{
		Freq: string(freq),
		// Clarifier fields: HW-CONFIRMED 2026-07-13 (M5b write trials,
		// doc.go register item 20) — the radio ACCEPTS the transmitted
		// clarifier value and Rx/Tx flags (validated above, no rejection)
		// but silently IGNORES them: readback is zeros every time. Stored
		// as zeros here, never the transmitted values. (Whether a
		// pre-existing NON-zero stored clarifier would be preserved or
		// zeroed by an MW is unobserved — no non-zero stored clarifier was
		// ever seen; see the register item.)
		ClarSign:   '+',
		ClarMag:    "0000",
		RXClar:     false,
		TXClar:     false,
		Mode:       mode,
		Kind:       kind,
		CTCSS:      ctcss,
		Shift:      shift,
		Tag:        existing.Tag,        // MW does not touch the tag (independent state)
		TagDisplay: existing.TagDisplay, //
		Populated:  true,
	}
	// HW-CONFIRMED 2026-07-13 (M5b write trials, docs/hardware-notes.md):
	// a successful MW moves the radio's selection to the written slot,
	// hands-off — no MC-set involved. A REJECTED MW (returned above,
	// before this point) never reaches here, so it correctly never moves
	// the selection.
	r.currentChannel = slot
	return nil
}

// --- MT: MEMORY CHANNEL TAG (reference "MT" section) ---
//
// Read frame (6 bytes): "MT" + 3-byte slot + ";".
// Set/Answer frame (7-19 bytes): "MT" + 3-byte slot + 1-byte display flag
// + 0-12 byte tag + ";". The two are disambiguated purely by length: a
// Read body (after "MT", before ';') is always exactly 3 bytes; a Set
// body is always >= 4 bytes (display digit present even for an empty
// tag). Verified against golden vectors G8 ("MT0011CALLING FREQ;"), G9
// ("MT005040M;") and G10 ("MT001;") during derivation.
//
// Slot policy (brief, "MT" bullet): accepted for ALL slot kinds the
// manual's grammar allows, INCLUDING 5xx/EMG — a future HOST-side policy
// may refuse to SEND such a write, but that is a different layer; the
// fake models what the radio accepts, not our policy.

const mtReadBodyLen = 3    // "MT" + slot, no display/tag
const mtSetMinBodyLen = 4  // slot(3) + display(1); the 0-byte-tag SHAPE parses as a Set but is then REJECTED (HW-CONFIRMED, see handleMT)
const mtSetMaxBodyLen = 16 // slot(3) + display(1) + tag(<=12)

// buildMTReply is THE single MT reply builder (kept in one function, per
// the task design notes, so a later milestone can swap this short form —
// "MT<slot><display><tag>;" — for Hamlib's long combined form without
// touching call sites). HW-CONFIRMED 2026-07-13 short form (see
// docs/hardware-notes.md §MT short-form answer): live probe "MT006;" ->
// "MT0061" + tag + ";" — the exact Set-shaped layout this fake already
// built; the manual showed no distinct MT answer example and Hamlib
// suggested a longer combined form might exist on some firmware, but
// Stuart's FT-710 does not use it.
func buildMTReply(slot string, display bool, tag string) []byte {
	out := make([]byte, 0, 6+len(tag))
	out = append(out, 'M', 'T')
	out = append(out, slot...)
	out = append(out, boolFlagByte(display))
	out = append(out, tag...)
	out = append(out, ';')
	return out
}

// handleMT validates and applies an MT body (frame bytes after "MT",
// before the trailing ';'), for both the Read and Set forms.
func (r *Radio) handleMT(body []byte) []byte {
	switch {
	case len(body) == mtReadBodyLen:
		slot := string(body)
		if !mrReadableSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		s, ok := r.slots[slot]
		r.mu.Unlock()
		if !ok {
			// Never-touched slot (no MW, no factory image entry, no prior
			// MT-set) -> "?;" — HW-CONFIRMED 2026-07-13, see doc.go
			// register. A slot that DOES exist in the map (Populated via
			// MW/factory image, OR tag-only via a prior MT-set) still
			// answers with whatever Tag/TagDisplay it holds, even if
			// Populated is false — that write-side independence (register
			// item 5) is unchanged and untested by M5a (read-only session).
			return rejection
		}
		return buildMTReply(slot, s.TagDisplay, s.Tag)

	case len(body) >= mtSetMinBodyLen && len(body) <= mtSetMaxBodyLen:
		slot := string(body[0:3])
		if !mrReadableSlot(parseSlotForm(slot)) {
			return rejection
		}
		displayByte := body[3]
		if !validBoolFlagByte(displayByte) {
			return rejection
		}
		tag := body[4:]
		// A ZERO-byte-tag Set is rejected: HW-CONFIRMED 13/07/2026
		// (docs/fixtures-private/m5b-trials.private-capture, stages
		// tagclear/tagclear2; docs/hardware-notes.md §Empty-slot create,
		// tag-clear): the live probe "MT0960;" drew an immediate "?;"
		// (~4 ms) and the slot's tag SURVIVED. The radio's one proven
		// tag-CLEAR mechanism is the all-spaces 12-byte tag, which this
		// fake (like the radio) accepts and stores through the normal
		// path below. This fake's former acceptance of the 0-byte form
		// was a now-proven divergence from hardware. (See doc.go
		// register item 7.)
		if len(tag) == 0 {
			return rejection
		}
		if !validTag(tag) {
			return rejection
		}
		r.mu.Lock()
		existing := r.slots[slot]
		existing.Tag = string(tag)
		existing.TagDisplay = displayByte == '1'
		r.slots[slot] = existing
		r.mu.Unlock()
		return nil // fire-and-forget success

	default:
		return rejection
	}
}

// --- MC: MEMORY CHANNEL (recall) (reference "MC" section) ---
//
// Set frame (6 bytes): "MC" + 3-byte slot + ";" — fire-and-forget;
// recalls the channel (changes the fake's "current channel" state).
// Read frame (3 bytes): "MC;" -> Answer "MC" + 3-byte current + ";".
// Disambiguated by length exactly as MT is.
//
// Empty (unpopulated) slot -> "?;" (ASSUMED, doc.go register): paired
// with MR's identical rule — you cannot recall a channel with no stored
// data.

func buildMCAnswer(current string) []byte {
	out := make([]byte, 0, 6)
	out = append(out, 'M', 'C')
	out = append(out, current...)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMC(body []byte) []byte {
	switch len(body) {
	case 0:
		r.mu.Lock()
		cur := r.currentChannel
		r.mu.Unlock()
		return buildMCAnswer(cur)

	case 3:
		slot := string(body)
		if !mrReadableSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		s, ok := r.slots[slot]
		if !ok || !s.Populated {
			r.mu.Unlock()
			return rejection
		}
		r.currentChannel = slot
		r.mu.Unlock()
		return nil

	default:
		return rejection
	}
}

// --- MR command handler ---

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != 3 {
		return rejection
	}
	slot := string(body)
	if !mrReadableSlot(parseSlotForm(slot)) {
		return rejection // out-of-inventory/malformed slot form (e.g. "MR100;") — HW-CONFIRMED 2026-07-13, doc.go register item 2
	}
	r.mu.Lock()
	s, ok := r.slots[slot]
	r.mu.Unlock()
	if !ok || !s.Populated {
		return rejection // empty slot — HW-CONFIRMED 2026-07-13, doc.go register item 2
	}
	return buildMRAnswer(slot, s)
}

// --- ID (reference: "ID; -> answer ID0800; (0800 fixed for FT-710)") ---

func buildIDAnswer() []byte { return []byte("ID0800;") }

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return buildIDAnswer()
}

// --- AI (reference: "AI0; disables Auto Information ... Set/Read/Answer
// all exist"; AI-set is fire-and-forget per "General framing".) ---

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
		return nil
	default:
		return rejection
	}
}

// --- Top-level dispatch ---

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a
// fire-and-forget success, or a non-nil frame (a real answer, or
// rejection) otherwise. Unknown or garbled commands fall through to
// rejection (reference: "Unknown-but-well-formed commands ... -> ?;" and
// brief: "Unknown/garbled command -> ?;").
func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) == 0 || frame[len(frame)-1] != ';' {
		return rejection // defensive: reassembler never hands us this
	}
	body := frame[:len(frame)-1]
	if len(body) < 2 {
		return rejection
	}
	cmd := [2]byte{toUpperASCII(body[0]), toUpperASCII(body[1])}
	rest := body[2:]

	// Reference: "the radio accepts upper or lower case" for command
	// names ONLY; field values below remain case-sensitive (ASSUMED,
	// doc.go register).
	switch cmd {
	case [2]byte{'I', 'D'}:
		return r.handleID(rest)
	case [2]byte{'A', 'I'}:
		return r.handleAI(rest)
	case [2]byte{'M', 'R'}:
		return r.handleMR(rest)
	case [2]byte{'M', 'W'}:
		return r.handleMW(rest)
	case [2]byte{'M', 'T'}:
		return r.handleMT(rest)
	case [2]byte{'M', 'C'}:
		return r.handleMC(rest)
	case [2]byte{'E', 'X'}:
		return r.handleEX(rest)
	default:
		return rejection
	}
}

func toUpperASCII(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 'a' + 'A'
	}
	return b
}

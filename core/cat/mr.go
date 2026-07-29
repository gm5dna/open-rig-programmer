// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// mrReadLen is the fixed length of an MR read request: "MR" + 3-byte slot
// + ";". Golden vector G3: "MR007;".
const mrReadLen = 6

// BuildMRRead builds an MR (memory channel read) request for slot s under
// this dialect's slot space. Reference: "MR — MEMORY CHANNEL READ
// (Read/Answer only; no Set) ... Read frame (6 bytes): MR P0 P0 P0 ;",
// golden vector G3.
//
// Any slot this dialect's ParseSlot would accept is a legal read target
// EXCEPT the special "000" placeholder: the reference marks its semantics
// UNKNOWN/ASSUMED and says "do not emit". 5xx and EMG slots are explicitly
// readable per the reference's slot table ("MR read" column: ✓ for both) —
// unlike MW/MT set, a read has no write-direction hardware-verification
// concern. See Dialect.readableSlot (slot.go), shared with BuildMTRead and
// AllowedCommand's MR/MT grammar checks.
func (d Dialect) BuildMRRead(s Slot) (Command, error) {
	if !d.readableSlot(s) {
		return Command{}, newParseError([]byte(s.Wire()), "MR: slot must be a readable memory/PMS/60m/EMG slot, not \"000\" or invalid")
	}
	frame := make([]byte, 0, mrReadLen)
	frame = append(frame, 'M', 'R')
	frame = append(frame, s.Wire()...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// ParseMRAnswer strictly parses a 28-byte MR answer frame per the
// reference's fixed-offset position table (P1-P10). Every field is
// validated; any deviation from the documented wire form is rejected with
// a *ParseError. Golden vectors G4, G6; G7 shares this exact body layout
// (see mr_test.go's TestParseMRAnswer_G7SharedLayout and mw.go).
//
// This is a thin wrapper around parseMemoryFrame, the shared decoder also
// used by AllowedCommand's MW grammar check (allowlist.go): the manual
// documents MR's answer and MW's set frame as byte-for-byte identical
// (memdata.go's field offsets are shared by both), so the wire-level field
// validation lives in exactly one place rather than being duplicated
// between a parser and a validator.
func (d Dialect) ParseMRAnswer(frame []byte) (MemoryData, error) {
	return d.parseMemoryFrame(frame, "MR")
}

// parseMemoryFrame strictly parses a 28-byte MR-answer/MW-set-shaped frame
// per the reference's fixed-offset position table (P1-P10), additionally
// checking that its first two bytes equal wantPrefix. See ParseMRAnswer's
// doc comment for why this is factored out: it is shared, unchanged,
// between ParseMRAnswer (wantPrefix "MR") and AllowedCommand's MW grammar
// check (wantPrefix "MW").
//
// Since M9c-3 task 3 this function is the 28-byte form's FRAMING only —
// length, prefix, terminator — and delegates the field block at offsets
// 2-26 to parseMemoryFields (memdata.go), which the combined MT record
// shares. THE ORDER OF THESE THREE CHECKS IS DELIBERATE AND PINNED: a frame
// that is wrong in several ways reports its length first, then its prefix,
// then its terminator, and only then a field. See
// memfields_test.go's TestParseMemoryFrame_DoublyInvalidFrameErrorOrder,
// which was written and run green BEFORE the extraction for exactly this
// reason.
//
// THIS IS THE HELPER THE MILESTONE TURNS ON (Codex plan-review F3). It is
// reached from two Dialect methods with different jobs — a parser and the
// outbound write gate — so every membership decision inside it must be
// taken against the RECEIVER: d.ParseSlot for the slot field (P1) and
// d.ParseMode for the mode field (P6), never the package-level
// delegates. With one dialect configured, a package-level call here would
// pass every test in the tree while the seam was fiction. That obligation
// moved intact into parseMemoryFields, which is a Dialect method for it.
func (d Dialect) parseMemoryFrame(frame []byte, wantPrefix string) (MemoryData, error) {
	if len(frame) != memoryFrameLen {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame must be %d bytes", wantPrefix, memoryFrameLen))
	}
	if frame[0] != wantPrefix[0] || frame[1] != wantPrefix[1] {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame missing %q prefix", wantPrefix, wantPrefix))
	}
	if frame[memTermOffset] != ';' {
		return MemoryData{}, newParseError(frame, fmt.Sprintf("%s frame missing ';' terminator", wantPrefix))
	}
	return d.parseMemoryFields(frame, wantPrefix)
}

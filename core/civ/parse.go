// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import "fmt"

// answerBody checks that frame is an ANSWER to this profile's radio —
// addressed to this controller, from this radio, carrying cn/sc — and
// returns the bytes after the sub-command.
//
// THE DIRECTION IS THE POINT. A command frame and its answer differ only
// in which way round the two address bytes go, so a parser that checked
// only the command number would happily read a frame this program SENT as
// a frame the radio answered — and on a bus that echoes, it would do so
// constantly.
func (p Profile) answerBody(frame []byte, cn, sc byte) ([]byte, error) {
	if !p.Configured() {
		return nil, fmt.Errorf("civ: unconfigured profile parses nothing")
	}
	if !WellFormed(frame) {
		return nil, newParseError(frame, "not a well-formed CI-V frame")
	}
	gotCN, gotSC, ok := FrameCommand(frame)
	if !ok {
		return nil, newParseError(frame, "frame carries no sub-command byte")
	}
	if frame[2] != p.controllerAddr {
		return nil, newParseError(frame, "%s: frame is addressed to %#02x, not to this controller (%#02x)", p.model, frame[2], p.controllerAddr)
	}
	if frame[3] != p.radioAddr {
		return nil, newParseError(frame, "%s: frame is from %#02x, not from this radio (%#02x)", p.model, frame[3], p.radioAddr)
	}
	if gotCN != cn || gotSC != sc {
		return nil, newParseError(frame, "%s: frame carries command %#02x %#02x, want %#02x %#02x", p.model, gotCN, gotSC, cn, sc)
	}
	return frame[6 : len(frame)-1], nil
}

// ParseTransceiverID reads the `19 00` answer and returns its data bytes
// as a compact hex token, e.g. "94".
//
// IT IS A DIAGNOSTIC, NEVER A MATCH (spec D3.2, D5 entry 7). The reply
// VALUE is undocumented on all six models in this tier, so nothing in this
// program compares it against an expected value: what identifies the radio
// at this step is that an ADDRESS-MATCHED reply arrived at all, and the
// token is recorded so a future hardware lift has something to compare
// against.
//
// A token rather than raw bytes because that is the whole of its use: it
// goes into Identity.CATID beside the address, and into a diagnostics
// line. A caller wanting to decide something from it would be doing the
// one thing this parser's contract says cannot be done.
func (p Profile) ParseTransceiverID(frame []byte) (string, error) {
	body, err := p.answerBody(frame, CmdTransceiverID, SubTransceiverID)
	if err != nil {
		return "", err
	}
	if len(body) == 0 {
		return "", newParseError(frame, "%s: transceiver-ID answer carries no data", p.model)
	}
	const digits = "0123456789abcdef"
	out := make([]byte, 0, 2*len(body))
	for _, b := range body {
		out = append(out, digits[b>>4], digits[b&0x0F])
	}
	return string(out), nil
}

// ParseMemoryAnswer reads a `1A 00 <address> <record>` answer into a
// neutral MemoryRecord.
//
// A record whose length is not in this profile's accepted set is an ERROR
// (*RecordLengthError, matching ErrRecordLength) rather than a partial
// parse: the read fails and the caller's ReadAll aborts honestly. That is
// spec D4's adjudication 13, and it is also what makes the probe's LENGTH
// FINGERPRINT continuous — every record read re-checks the length, so a
// wrong-model session cannot be opened once and then trusted.
//
// AN EMPTY CHANNEL IS NOT THIS FUNCTION'S CASE. Spec D5 entry 2 records
// two separate, unverified possibilities — the radio answers FA, or it
// answers a record of 0xFF bytes — and neither is decided here: an FA
// frame never reaches this parser (it carries no 1A 00), and an all-0xFF
// record fails on its enum bytes with a parse error naming the offending
// offset. Inventing an "empty channel" result from either would be a claim
// this tier's evidence does not support.
func (p Profile) ParseMemoryAnswer(frame []byte) (MemoryRecord, error) {
	body, err := p.answerBody(frame, CmdMemory, SubMemoryContents)
	if err != nil {
		return MemoryRecord{}, err
	}
	n := p.addressForm.addressBytes()
	if len(body) < n {
		return MemoryRecord{}, newParseError(frame, "%s: memory answer carries %d bytes, too few for a %d-byte address under %v", p.model, len(body), n, p.addressForm)
	}
	addr, err := p.decodeAddress(body[:n])
	if err != nil {
		return MemoryRecord{}, err
	}
	record := body[n:]
	if !p.AcceptsRecordLength(len(record)) {
		return MemoryRecord{}, &RecordLengthError{Want: p.RecordLengths(), Got: len(record)}
	}
	return p.decodeRecord(record, addr)
}

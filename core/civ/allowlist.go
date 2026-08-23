// SPDX-License-Identifier: GPL-3.0-or-later

package civ

// AllowedCommand reports whether frame is safe to write to the radio: it
// is one of the three command grammars this package can build —
// `19 00`, `1A 00 <address>` read, `1A 00 <address> <record>` set — fully
// re-validated field by field against the same rules the corresponding
// builder enforces, not merely a command-number match.
//
// ONE DELIBERATE WIDTH, AND IT IS THE ONLY ONE. For a profile accepting
// more than one record length (the IC-905, spec D6), the gate admits a set
// at ANY length in RecordLengths(), while BuildMemorySet emits only
// BuildRecordLength(). The admitted set is therefore strictly wider than
// the builder set by exactly the other declared layouts, and nothing else.
//
// It is deliberate rather than an oversight. The accepted-length SET is
// the profile's statement of what its radio's memory records are — it is
// the probe's own length fingerprint (spec D3.2) — so a record at a length
// this profile declares is a record that radio has, and refusing to
// authorise writing one back would mean this package could read a record
// it may not write. The narrowing is not free either: BuildLength is
// DECLARED rather than derived precisely because which length to WRITE is
// a choice the model's data makes, and a gate keyed on that choice would
// refuse a frame built from the very bytes the radio answered with.
//
// Every OTHER length is refused, and both halves are exercised:
// TestGateAdmitsEveryAcceptedLengthWhileTheBuilderEmitsOne walks a
// two-length profile, builds a set at each declared length, and requires
// the gate to admit both while asserting the builder produced exactly
// BuildRecordLength.
//
// WHAT IT REFUSES IS THE INTERESTING HALF:
//
//   - EVERY `1A 05`, every `1A 01`, every clear form and every transceive
//     set. This tier ships no builder for any of them (builders.go says
//     why), and the switch below has no branch that could admit one.
//   - ANY `to` byte other than THIS profile's radio address. A frame
//     addressed to another station on the bus is not this program's to
//     send, and one addressed to the controller is an ANSWER — never a
//     legal outbound command, however well-formed.
//   - ANY interior preamble or terminator byte, through WellFormed: a
//     second command hidden after an interior 0xFD would otherwise be
//     approved unexamined, which is the CI-V form of core/cat's
//     embedded-semicolon attack.
//
// THE SET GRAMMAR IS RE-VALIDATED BY DECODE, VALIDATE AND RE-ENCODE. The
// frame's record is decoded with this profile's own layout, run through
// validateRecordFields — the SAME validator BuildMemorySet uses — and then
// re-encoded; the frame is admitted only if the bytes come back identical.
// That last step is what makes "admits ONLY builder-producible frames"
// literally true rather than approximately: a frame whose unmapped bytes
// carry anything a builder would not have written, or whose fields are
// encoded in some other legal-looking way, differs from the re-encoding
// and is refused. A rule that merely checked each field in isolation would
// have admitted all of those.
//
// IT GATES FOR THE PROFILE IT IS CALLED ON, and for no other. Every check
// below reads the receiver — addresses, address form, channel range,
// layouts, name policy — so a frame legal for one Icom model is refused by
// a profile describing another. A gate that re-validated against a
// package-level datum would accept, on any radio, whatever one radio
// accepts; here that is a safety failure rather than merely a correctness
// one, and it is why allTestProfiles holds a profile built to disagree.
//
// FAIL-CLOSED ON AN UNCONFIGURED PROFILE. A zero Profile is constructible
// by any caller and its AllowedCommand is a non-nil method value, so
// nothing about the METHOD's existence says which radio it speaks for — or
// that it speaks for one at all. The Configured() guard is what answers
// that, and it matters even though the layout checks would refuse
// everything anyway: `19 00` consults no layout, and without the guard a
// zero Profile would admit it for a radio at address 0x00.
//
// Adding a command, or loosening any check below, is a REVIEWED DECISION:
// this is the last defence before bytes reach a physical radio.
func (p Profile) AllowedCommand(frame []byte) bool {
	if !p.Configured() {
		return false
	}
	if !WellFormed(frame) {
		return false
	}
	if frame[2] != p.radioAddr {
		return false
	}
	if frame[3] != p.controllerAddr {
		return false
	}
	cn, sc, ok := FrameCommand(frame)
	if !ok {
		return false
	}
	body := frame[6 : len(frame)-1]

	switch {
	case cn == CmdTransceiverID && sc == SubTransceiverID:
		return p.validTransceiverIDRead(body)
	case cn == CmdMemory && sc == SubMemoryContents:
		return p.validMemoryCommand(body)
	default:
		return false
	}
}

// validTransceiverIDRead reports whether the body after `19 00` is the
// read request's — that is, EMPTY.
//
// The `19 00` ANSWER carries the radio's address byte and is otherwise the
// same shape. It is refused here, as every answer frame is: an answer is
// never a legal outbound command, and admitting one would let a captured
// or replayed answer be written back to the radio.
func (p Profile) validTransceiverIDRead(body []byte) bool {
	return len(body) == 0
}

// validMemoryCommand reports whether the body after `1A 00` is a read
// request or a set this profile's own builder could have produced.
func (p Profile) validMemoryCommand(body []byte) bool {
	n := p.addressForm.addressBytes()
	if len(body) < n {
		return false
	}
	addr, err := p.decodeAddress(body[:n])
	if err != nil {
		return false
	}
	record := body[n:]
	if len(record) == 0 {
		// The READ request: address, no data.
		return true
	}

	// THE SET. Its length must be one this profile accepts — which for
	// the multi-length models is the same fingerprint the probe reads —
	// and the record must survive decode, the builder's own validator, and
	// a re-encode that reproduces it byte for byte.
	//
	// ANY accepted length, not just BuildRecordLength: the one deliberate
	// place this gate is wider than the builders, argued at length in
	// AllowedCommand's own doc comment above. A length this profile does
	// NOT declare is refused here, which is what keeps the width to the
	// declared layouts and no further.
	if !p.AcceptsRecordLength(len(record)) {
		return false
	}
	rec, err := p.decodeRecord(record, addr)
	if err != nil {
		return false
	}
	if err := p.validateRecordFields(rec, len(record)); err != nil {
		return false
	}
	again, err := p.encodeRecord(rec, len(record))
	if err != nil {
		return false
	}
	return string(again) == string(record)
}

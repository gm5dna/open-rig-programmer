// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// AllowedCommand reports whether frame is safe to write to the radio: it
// is EXACTLY one of the seven command grammars this package knows how to
// build — ID read, AI read/set, MR read, MW set, MT read/set, MC
// read/set, EX read — fully re-validated field-by-field against the same
// rules the corresponding builder enforces, not merely a 2-byte
// command-name prefix.
//
// This is deliberately stricter than "well-formed": an inbound ANSWER
// frame this package can PARSE (e.g. a full 28-byte MR answer, or
// "ID0800;") is REJECTED here, because AllowedCommand's job is to gate
// what may be WRITTEN to the radio, and an answer frame is never a legal
// outbound command — see TestAllowedCommand_RejectsGoldenAnswerFrames.
// Likewise a frame with an embedded ';' — two frames concatenated, or an
// injection attempt that slipped past a builder — is rejected even if a
// prefix matches, because exactly one command must reach the wire per
// call, and nothing else.
//
// The per-command checks below share their field-validation logic with
// the corresponding builder (readableSlot, parseMemoryFrame,
// validateMWFields, mtSlotValid, validMTTag, mcValid) rather than
// duplicating it, so "what AllowedCommand accepts" and "what the builders
// produce" cannot drift apart.
//
// EX is deliberately narrower than the other six: only the 9-byte EX READ
// frame is accepted. EX Set and Answer share an identical wire shape —
// "EX" prefix, the same six-digit address field, just a longer body
// (manual lines ~630-637) — and are REJECTED here even though they are
// otherwise syntactically well-formed; see validEXRead's doc comment for
// the REVIEWED DECISION. This was written as a phase restriction; the
// M8d menu-write decision (25/07/2026, no-go — docs/menu-write-decision.md)
// made it the shipped policy.
//
// Adding a command, or loosening any check below, is a REVIEWED DECISION:
// this is the last defence the transport layer relies on before writing
// arbitrary bytes to a physical radio. Every builder's own output MUST
// satisfy AllowedCommand — enforced by
// TestAllowedCommand_PropertyEveryBuilderOutput — and every golden ANSWER
// frame in this package's tests MUST NOT.
//
// IT GATES FOR THE DIALECT IT IS CALLED ON, and for no other (Task 54).
// Every per-command check below is a Dialect method reaching only
// dialect-aware helpers, so a frame legal under one radio's slot space,
// mode set or EX inventory is refused by a dialect that does not share it.
// A gate that re-validated against a package global would accept, on any
// radio, whatever the FT-710 accepts — the failure this milestone exists
// to prevent, and here it is a safety failure rather than merely a
// correctness one.
//
// FAIL-CLOSED ON AN UNCONFIGURED DIALECT. A zero Dialect is constructible
// by any caller (`var d cat.Dialect`) and its AllowedCommand is a non-nil
// method value, so it passes transport.NewEngine's nil-AllowFunc check
// (added at Task 56) and would be installed as a real engine's gate. It
// must therefore accept NOTHING itself: the constructor's check catches a
// MISSING gate, not an empty one. The dialect-aware checks give that for
// free wherever
// slot, mode or EX data is consulted — an empty slot space matches no
// slot — but ID, AI and the bare "MC;" read are literal matches that
// consult no dialect data at all and would otherwise pass. The
// Configured() guard below is what closes them, so the property holds for
// every frame rather than most of them.
func (d Dialect) AllowedCommand(frame []byte) bool {
	if !d.Configured() {
		return false
	}
	if len(frame) < 3 { // shortest possible legal frame: "ID;"
		return false
	}
	if !exactlyOneTrailingSemicolon(frame) {
		return false
	}

	switch string(frame[0:2]) {
	case "ID":
		return d.validIDCommand(frame)
	case "AI":
		return d.validAICommand(frame)
	case "MR":
		return d.validMRCommand(frame)
	case "MW":
		return d.validMWCommand(frame)
	case "MT":
		return d.validMTCommand(frame)
	case "MC":
		return d.validMCCommand(frame)
	case "EX":
		return d.validEXRead(frame)
	default:
		return false
	}
}

// exactlyOneTrailingSemicolon reports whether frame contains exactly one
// ';' byte, and that byte is the very last byte of frame.
func exactlyOneTrailingSemicolon(frame []byte) bool {
	count := 0
	semiPos := -1
	for i, b := range frame {
		if b == ';' {
			count++
			semiPos = i
		}
	}
	return count == 1 && semiPos == len(frame)-1
}

// validIDCommand reports whether frame is the ID read request. ID has no
// Set form and BuildIDRead produces exactly this one frame, so this is a
// literal match rather than a field-by-field parse.
//
// Takes a dialect receiver even though nothing about this frame varies by
// radio — same rationale as BuildIDRead's, plus one specific to the gate:
// a mixed switch, half methods and half package functions, is exactly the
// shape in which a hardwired check hides. Dialect.AllowedCommand's
// Configured() guard, not this function, is what stops a zero Dialect
// accepting "ID;".
func (d Dialect) validIDCommand(frame []byte) bool {
	return string(frame) == "ID;"
}

// validAICommand reports whether frame is a legal AI frame: the Set forms
// BuildAISet produces ("AI0;", "AI1;") plus the documented Read form
// ("AI;") — reference: "AI0; disables Auto Information ... Set/Read/Answer
// all exist." No builder emits "AI;" yet, but it is legitimate protocol
// grammar, not merely a builder-output check.
//
// Takes a dialect receiver even though nothing about this frame varies by
// radio; see validIDCommand's note.
func (d Dialect) validAICommand(frame []byte) bool {
	switch string(frame) {
	case "AI;", "AI0;", "AI1;":
		return true
	default:
		return false
	}
}

// validMRCommand reports whether frame is a legal MR read request: exactly
// mrReadLen (6) bytes, with a slot this dialect's readableSlot accepts (the
// same rule BuildMRRead enforces — see Dialect.readableSlot's doc comment,
// slot.go). MR has no Set form, so anything else with an "MR" prefix —
// including a 28-byte MR ANSWER frame — is rejected here.
func (d Dialect) validMRCommand(frame []byte) bool {
	if len(frame) != mrReadLen {
		return false
	}
	slot, err := d.ParseSlot(string(frame[2:5]))
	if err != nil {
		return false
	}
	return d.readableSlot(slot)
}

// validMWCommand reports whether frame is a legal, fully in-policy MW Set
// frame: it decodes cleanly via parseMemoryFrame (the same fixed-offset
// 28-byte decoder ParseMRAnswer uses, reused here with prefix "MW") and
// then passes validateMWFields — the exact write-direction policy
// BuildMWSet applies (Writable() slot, Kind/slot pairing, ModeUnset
// rejection, forged-byte re-validation, ClarHz/FreqHz bounds). A frame
// that is shaped like a valid memory frame but targets an unwritable slot,
// or pairs the wrong Kind with its slot, is rejected here exactly as
// BuildMWSet would reject it if asked to build the equivalent MemoryData.
func (d Dialect) validMWCommand(frame []byte) bool {
	m, err := d.parseMemoryFrame(frame, "MW")
	if err != nil {
		return false
	}
	return d.validateMWFields(m) == nil
}

// validMTCommand reports whether frame is a legal MT read or Set frame.
//
// Read: exactly mtReadLen (6) bytes, with a slot readableSlot accepts (the
// same rule BuildMTRead enforces).
//
// Set: mtAnswerMinLen-mtAnswerMaxLen (7-19) bytes, "MT" prefix, a slot
// mtSlotValid accepts (memory/PMS only — the same write-direction policy
// BuildMTSet enforces), a valid display digit, and a tag body that passes
// d.validMTTag — the same printable-ASCII-excluding-';' charset AND
// per-dialect length bound (d.mt.TagMaxBytes, M9c-0 task 64) BuildMTSet
// enforces. This is what makes a frame like "MT0011A\x00TX1;" (a hidden
// NUL, no embedded ';') correctly rejected: the old prefix-plus-terminator
// check saw one allowlisted prefix and one trailing ';' and let it
// through; re-validating the tag body byte-for-byte does not. It is also
// what makes THIS DIALECT'S tag-length policy — not the FT-710's — the one
// the gate enforces: d.validMTTag reads d.mt.TagMaxBytes off the receiver
// this method was called on, so a dialect whose tag is narrower than 12
// bytes has its own bound enforced here, not the FT-710's wider one.
func (d Dialect) validMTCommand(frame []byte) bool {
	if len(frame) == mtReadLen {
		slot, err := d.ParseSlot(string(frame[2:5]))
		if err != nil {
			return false
		}
		return d.readableSlot(slot)
	}

	if len(frame) < mtAnswerMinLen || len(frame) > mtAnswerMaxLen {
		return false
	}
	if frame[0] != 'M' || frame[1] != 'T' {
		return false
	}
	slot, err := d.ParseSlot(string(frame[2:5]))
	if err != nil || !d.mtSlotValid(slot) {
		return false
	}
	if _, err := parseBoolDigit(frame[5]); err != nil {
		return false
	}
	tag := string(frame[6 : len(frame)-1])
	return d.validMTTag(tag)
}

// validMCCommand reports whether frame is a legal MC read or Set/Answer
// frame: the fixed "MC;" read request, or mcSetLen (6) bytes with a slot
// mcValid accepts (the same rule BuildMCSet and ParseMCAnswer share).
func (d Dialect) validMCCommand(frame []byte) bool {
	if string(frame) == mcReadFrame {
		return true
	}
	if len(frame) != mcSetLen {
		return false
	}
	slot, err := d.ParseSlot(string(frame[2:5]))
	if err != nil {
		return false
	}
	return d.mcValid(slot)
}

// validEXRead reports whether frame is a legal EX READ: exactly
// exReadLen (9) bytes with an address ParseEXAddress accepts — the same
// membership rule BuildEXRead enforces (shared, not duplicated: the
// "cannot drift apart" rule).
// SHIPPED POLICY, not a phase restriction: EX Set and Answer share an
// identical wire shape (manual lines ~630–637), and this rejects that
// entire shape outbound. Originally scoped to M8a–M8d pending the
// menu-write go/no-go; that decision was taken on 25/07/2026 and the
// answer was NO — the FT-710 menu surface is read-only for v1.x
// (docs/menu-write-decision.md). Nothing is scheduled to loosen this.
//
// If it is ever reopened, the shape to follow is MT's set/answer
// exception — see the MT set/answer-shaped entry in
// TestAllowedCommand_AcceptsAllowlistedSingleFrames (~allowlist_test.go:137)
// for the precedent — and the classes named in that decision document
// stay denied regardless. Loosening this is a REVIEWED DECISION.
func (d Dialect) validEXRead(frame []byte) bool {
	if len(frame) != exReadLen {
		return false
	}
	_, err := d.ParseEXAddress(string(frame[2:8]))
	return err == nil
}

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
// parseMemoryFields, validateMWFields, validateCombinedMTFields,
// mtSlotValid, validMTTag, validMTTagByte, mcSendValid) rather than
// duplicating it, so "what AllowedCommand accepts" and "what the builders
// produce" cannot drift apart.
//
// IT IS AN OUTBOUND GATE, and that direction is load-bearing wherever a Set
// and an Answer share a wire shape. MC is the clearest case: this gate
// re-validates a 6-byte MC frame against the SEND domain (mcSendValid),
// not against the wider domain ParseMCAnswer accepts, because a frame
// offered here is by definition one this program is about to write.
//
// MT IS THE ONE ACKNOWLEDGED SET/ANSWER EXCEPTION, and since M9c-3 it is
// narrower under one form than the other. A SHORT-form MT Set and its
// Answer are one wire shape exactly, so admitting the Set admits the Answer
// with it and no byte tells them apart — the accepted "MT set/answer-shaped
// G8" entry in TestAllowedCommand_AcceptsAllowlistedSingleFrames
// (~allowlist_test.go:138) is that exception written down. The COMBINED
// form carries a P7 kind byte whose SET direction documents exactly one
// value, '0' "(Fixed)", while an Answer reporting a Memory channel carries
// '1'. So under that form the exception shrinks to the P7-'0' case alone:
// a P7-'1' answer is refused outbound even though this package parses it
// happily inbound. This is a NARROWING of an existing exception, not a new
// one — see validMTCommand, which enforces it by reusing the builder's own
// validateCombinedMTFields rather than by a rule of its own.
//
// EX is deliberately narrower than the other six: only the EX READ frame
// is accepted, at exactly this dialect's own read length. EX Set and
// Answer share an identical wire shape — "EX" prefix, the same address
// field, just a longer body
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
// mode set, EX inventory or MT frame form (M9c-3) is refused by a dialect
// that does not share it.
// A gate that re-validated against a package global would accept, on any
// radio, whatever the FT-710 accepts — the failure this milestone exists
// to prevent, and here it is a safety failure rather than merely a
// correctness one.
//
// FAIL-CLOSED ON AN UNCONFIGURED DIALECT. A zero Dialect is constructible
// by any caller (`var d cat.Dialect`) and its AllowedCommand is a non-nil
// method value, so nothing about the METHOD's existence says which radio
// it speaks for — or that it speaks for one at all.
//
// The constructor that used to be the concern here is gone (M9c-5, E3):
// transport.NewEngine no longer takes an AllowFunc parameter to nil-check,
// it takes the cat.Dialect WHOLE and refuses an unconfigured one outright
// (transport.ErrUnconfiguredDialect), taking d.AllowedCommand itself. So a
// zero Dialect can no longer become a real engine's gate by that route.
// This check is not thereby redundant — the two answer different
// questions. NewEngine refuses to BUILD an engine that would gate for no
// radio; the guard below refuses to ADMIT a frame, wherever the judgement
// is asked for, and that is the property this package can actually
// guarantee. Anything holding a zero Dialect outside that constructor — a
// hand-built transport.Engine (whose unexported allow field core/transport
// itself can assign directly), a test double, a future caller asking
// AllowedCommand on its own — gets the same answer: NOTHING is allowed.
//
// The dialect-aware checks give that for free wherever slot, mode or EX
// data is consulted — an empty slot space matches no slot — but ID, AI and
// the bare "MC;" read are literal matches that consult no dialect data at
// all and would otherwise pass. The Configured() guard below is what
// closes them, so the property holds for every frame rather than most of
// them.
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
// BuildMWSet applies (a slot writable under THIS dialect per
// Dialect.writableSlot, Kind/slot pairing, ModeUnset
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

// validMTCommand reports whether frame is a legal MT read or Set frame
// UNDER THIS DIALECT'S OWN MT FORM. MT is the one command whose Set frame
// has two shapes (dialectconfig.go's MTForm), and they share a prefix and a
// terminator: a gate that branched on "MT" alone, or applied one family's
// length window to every receiver, would pass a short-form Set to a radio
// that reads its first six bytes as a slot and a display flag. So the form
// switch below is a safety boundary, not a convenience.
//
// READ (form-independent, and checked FIRST): exactly mtReadLen (6) bytes,
// with a slot mtReadSlotValid accepts — the same rule BuildMTRead enforces,
// which is THIS DIALECT'S declared MT read domain (MTPolicy.ReadSlots) and
// not MR's. Under MTReadsReadable the two coincide exactly; under
// MTReadsMemoryPMS an "MT501;" or "MTEMG;" is refused here as the builder
// refuses it, while the same slots stay legal for MR.
// The read request carries neither a record nor a tag, so its shape is the
// same under both forms, and no Set branch can claim a 6-byte frame: the
// short form's floor is 7 bytes and the combined form's shortest possible
// record is 30.
//
// SHORT Set: mtAnswerMinLen to d.mtShortAnswerMax() bytes — the floor is the
// frame grammar's, the ceiling THIS dialect's own tag width plus that floor
// (7-19 for the FT-710, 7-13 for a 6-byte-tag family) — "MT" prefix, a slot
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
//
// COMBINED Set, P11: judged by this dialect's MTP11Policy — the printed-fixed
// byte under P11Fixed, either documented value of the TAG flag under
// P11TagDisplay. The gate cannot ask "which builder made this", so it asks
// the policy, exactly as the builders do.
//
// COMBINED Set: exactly d.mtCombinedLen() bytes (29 + this dialect's tag
// width — 41 for the evidenced 12-byte family, and a receiver method
// precisely so this line cannot become another radio's number), "MT" prefix,
// ';' terminator, then the shared field block through parseMemoryFields and
// the SAME write-direction policy the builder applies,
// validateCombinedMTFields — slot, kind, mode, clarifier, frequency, CTCSS
// and shift in one place, so "what BuildMTSetCombined produces" and "what
// this admits" cannot drift apart. Then P11, fixed. Then the tag field, on
// the rule below.
//
// THE COMBINED FORM NARROWS MT'S SET/ANSWER COLLISION to the P7-'0' case:
// validateCombinedMTFields requires the Set direction's own kind constant,
// so a combined ANSWER reporting a Memory channel (P7 '1' — legal inbound,
// and accepted by ParseMTAnswerCombined) is REFUSED outbound. The short form
// cannot do that, its Set and Answer being one wire shape; see
// AllowedCommand's exception commentary above.
//
// THE EXACT COMBINED LENGTH IS ASSUMED UNTIL STAGE R, matching
// ParseMTAnswerCombined. It is the safe direction to be wrong in: this gate
// judges OUTBOUND frames and this package only ever builds full width, so
// even if a radio turns out to ANSWER short, nothing this package sends is
// affected.
func (d Dialect) validMTCommand(frame []byte) bool {
	if len(frame) == mtReadLen {
		slot, err := d.ParseSlot(string(frame[2:5]))
		if err != nil {
			return false
		}
		return d.mtReadSlotValid(slot)
	}

	switch d.mt.Form {
	case MTFormShort:
		if len(frame) < mtAnswerMinLen || len(frame) > d.mtShortAnswerMax() {
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

	case MTFormCombined:
		// Framing first, in ParseMTAnswerCombined's order: length, prefix,
		// terminator. The length is exact, so everything below indexes a
		// frame already known to be long enough.
		if len(frame) != d.mtCombinedLen() {
			return false
		}
		if frame[0] != 'M' || frame[1] != 'T' {
			return false
		}
		if frame[len(frame)-1] != ';' {
			return false
		}
		m, err := d.parseMemoryFields(frame, "MT")
		if err != nil {
			return false
		}
		if d.validateCombinedMTFields(m) != nil {
			return false
		}
		// P11, BY THIS DIALECT'S OWN READING, through p11Valid — the same
		// predicate parseMTAnswerCombined consults (mtcombined.go), so one
		// function decides byte 28's admission rule for both the parser and
		// the gate. Under P11Fixed only the printed-fixed byte is admitted,
		// as before. Under P11TagDisplay the byte is a live TAG flag and
		// both of its documented values are admitted — and nothing else, so
		// an undocumented third value is still refused outbound.
		if !d.p11Valid(frame[mtCombinedP11Offset]) {
			return false
		}
		// THE RAW TAG FIELD, PER BYTE, WITH validMTTagByte ONLY —
		// deliberately NOT d.validMTTag, and deliberately NOT the builder's
		// refusal of a tag ENDING IN TagFill (REVIEWER-ADJUDICATED, Codex
		// plan finding 8).
		//
		// Those are INPUT rules on a LOGICAL tag; this is a WIRE field, and
		// they are different rules on different representations. The field
		// is fixed at the full width, so validMTTag's length bound is
		// already decided by the exact frame length above; and the field
		// ALWAYS ends in TagFill whenever the logical tag was shorter than
		// the field — an empty tag IS the all-fill field. Padding erases the
		// data-versus-fill distinction on the wire, so applying the builder's
		// suffix rule here would make the gate refuse its own builder's every
		// non-full-width tag.
		//
		// What does cross the boundary is the CHARSET, because that is the
		// command-injection defence rather than a round-trip property: a ';'
		// or a control byte is refused in whichever representation it
		// appears. TagFill itself is always admissible here — V9 requires it
		// to be a valid wire byte.
		for _, b := range frame[mtCombinedTagOffset : mtCombinedTagOffset+d.mt.TagMaxBytes] {
			if !validMTTagByte(b) {
				return false
			}
		}
		return true

	default:
		// FAIL-CLOSED on a dialect with no form. AllowedCommand's
		// Configured() guard already refuses everything an unconfigured
		// dialect is offered, and this is the second lock: falling through to
		// either form's logic would have a formless dialect judging real
		// frames by a shape it never declared.
		return false
	}
}

// validMCCommand reports whether frame is a legal MC read or Set frame: the
// fixed "MC;" read request, or mcSetLen (6) bytes with a slot mcSendValid
// accepts — the SEND-side predicate, the same rule BuildMCSet enforces.
//
// THIS IS AN OUTBOUND GATE, so the send-side reading is the right one and
// the ONLY safe one. A 6-byte MC frame arriving here can only be a Set: an
// Answer is something the radio sends, never something this program writes,
// even though the two share a wire shape exactly. Judging it by
// ParseMCAnswer's wider read-side domain instead would let a dialect whose
// own MC legend omits the 5xx and EMG banks have a side-effecting recall of
// one of them admitted by its own gate. See mcSendValid (mc.go) for the
// full statement of the split.
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
	return d.mcSendValid(slot)
}

// validEXRead reports whether frame is a legal EX READ: exactly THIS
// DIALECT'S d.exReadLen() bytes — 9 under EXAddressTriple, 7 under
// EXAddressPair — with an address ParseEXAddress accepts, the same
// membership rule BuildEXRead enforces (shared, not duplicated: the
// "cannot drift apart" rule).
//
// The length and the address slice both come from d.EXAddressWidth(), the
// same datum the builder measures. Until the FT-891 Stage 0 seam both were
// the package constant 9, read through this receiver, so this gate would
// have refused a four-digit dialect's own builder's output —
// TestEveryDialect_BuiltFramesAreCleanAndGateAdmissible over pairDialect is
// what reports that.
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
	if len(frame) != d.exReadLen() {
		return false
	}
	_, err := d.ParseEXAddress(string(frame[2 : 2+d.EXAddressWidth()]))
	return err == nil
}

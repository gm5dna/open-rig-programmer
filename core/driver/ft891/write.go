// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ctcssByName is read.go's ctcssNames read backwards: codeplug's display
// spelling to the wire state. shiftByName does the same for read.go's
// shiftNames.
//
// They live HERE, in the write file, rather than beside their forward
// twins, because they are the write direction's own vocabulary — and
// because a pair of hand-written maps meant to be exact inverses is worth a
// test rather than an adjacency: TestNameMaps_AreExactInverses walks both
// directions over both pairs, so a spelling added to one map and forgotten
// in the other fails rather than silently refusing a legitimate value at
// the write gate (or, worse, mapping it onto the wrong byte).
//
// Deliberately NOT cat.CTCSSState.String()/cat.Shift.String(): those
// spellings ("off", "ENC/DEC") are log labels, and the strings these keys
// must match are the ones codeplug.Validate checks for and this driver's
// Capabilities advertises (spec.StandardCTCSSStates,
// spec.StandardShiftOptions).
//
// THREE STATES AND NO MORE, as on the read side: this radio's P8 legend
// prints exactly `0: CTCSS "OFF" 1: CTCSS ENC/DEC 2: CTCSS ENC` on all five
// blocks that carry it (MR 977, MT 1012, MW 1048, IF 790, OI 1136), and
// CT's fourth value — `3: DCS "ON"` (414) — is LIVE STATE on a different
// command, not a memory field (matrix §1.17).
var ctcssByName = map[string]cat.CTCSSState{
	"OFF":     cat.CTCSSOff,
	"ENC-DEC": cat.CTCSSEncDec,
	"ENC":     cat.CTCSSEnc,
}

// shiftByName is shiftNames' write-direction inverse — this radio's P10
// legend, "0: Simplex 1: Plus Shift 2: Minus Shift", printed identically on
// MR (979), MT (1015), MW (1050), IF (792) and OI (1138). See ctcssByName.
var shiftByName = map[string]cat.Shift{
	"SIMPLEX": cat.ShiftSimplex,
	"PLUS":    cat.ShiftPlus,
	"MINUS":   cat.ShiftMinus,
}

// mtSetSpec is the transport spec for the combined MT SET:
// transport.CATWriteSpec(), the transport's fire-and-forget mode — write
// the frame, listen for a bounded window in case a "?;" rejection arrives,
// and treat silence as acceptance.
//
// THAT SHAPE IS ASSUMED ON BOTH COUNTS, NOT MANUAL-EVIDENCED, and the
// driver register carries it twice over: as THE ACKNOWLEDGEMENT
// CONVENTIONS in its own right, and as the second half of A SINGLE COMBINED
// MT SET SUFFICES TO CREATE OR OVERWRITE A CHANNEL — one entry because one
// capture lifts both halves and this one line of code asserts both. This
// manual states no acknowledgement convention anywhere; silence-on-success
// and exactly one "?;" on rejection are the family's inherited working
// values. Nor does MT's availability row supply one: `MT | MEMORY WRITE &
// TAG | O X X X` (layout 166) marks which FORMS the command has, not what a
// Set draws back — reading a reply convention off that row was the FTdx10
// milestone's erratum 1 and is not repeated here (matrix §3.6).
//
// Every part of the spec is load-bearing, and it is why this is a separate
// function from read.go's mtSpec rather than a reuse of it:
//
//   - NO answer matcher, and therefore no derived answer length. mtSpec
//     pins the combined ANSWER's exact geometry from the dialect, which is
//     right for a read and would be a bug here: on the assumed convention
//     an accepted Set produces no answer at all, so a spec that waited for
//     an "MT" reply would spend the whole read timeout and then report a
//     timeout for a write the radio had accepted perfectly.
//
//   - RetryReads 0, necessarily. A write is NEVER resent — transport
//     safety obligation 2 enforces this structurally, and Do refuses any
//     write-class spec with a non-zero RetryReads outright, before writing
//     anything. Resending an accepted Set would write the channel twice;
//     resending one whose fate is unknown would write it a second time on
//     top of a first that may have landed.
//
// TestMTSetSpec_IsFireAndForgetAndNeverRetries pins both properties and
// that this spec is not the read's.
func mtSetSpec() transport.CommandSpec {
	return transport.CATWriteSpec()
}

// bankFor reports which of this session's banks claims slot.
//
// A linear walk over the session's EFFECTIVE banks (s.caps, which includes
// the discovered read-only 60M/EMG banks), so a slot this radio does not
// have is in no bank at all and a write to it is refused outright rather
// than gated per-field against a bank that does not exist.
func (s *Session) bankFor(slot string) (spec.BankID, bool) {
	for _, b := range s.caps.Banks {
		for _, sl := range b.Slots {
			if sl == slot {
				return b.ID, true
			}
		}
	}
	return "", false
}

// requestedFields lists every spec.Field a write of data actually requests:
// the six plain fields ALWAYS (this radio's combined MT Set carries
// frequency, mode, clarifier, CTCSS state, shift AND the tag in one frame,
// whether or not any of them changed), plus TagDisplay, CTCSSTone and
// ScanSkip when — and only when — their FieldState is Known. Per codeplug's
// write rule, Unknown and Unavailable both mean "preserve whatever the
// radio has", i.e. nothing is requested for that field.
//
// It mirrors the DIFF LAYER'S REQUESTED-SET DERIVATION exactly — same
// membership, the same conditionals, the same order — so this driver's
// defence-in-depth gate and the diff layer's gate judge the same set for
// the same channel. That derivation is two pieces on the codeplug side and
// both are mirrored here: addedFields' six unconditional plus three
// conditional fields, and then the SEVENTEEN Icom-tier conditionals
// codeplug carries in tierAddedFieldFor and appends in touchedFields (see
// tierRequestedFields). The seventeen come LAST, in ChannelData's
// declaration order, exactly as they do there, so no BlockReason a user has
// ever read is reordered by their arrival. Mirrored, NOT imported:
// tierAddedFieldFor is unexported, and the mirror is held by both sides
// pinning the same shape (TestRequestedFields_MembershipAndOrder here,
// codeplug's own tests there).
//
// TAGDISPLAY'S CONDITIONAL NEEDS A WORD ITS TWO NEIGHBOURS DO NOT, and on
// this radio it is the FT-710's word rather than the FTdx10's. Byte 28 is a
// LIVE flag here (MT's P11 legend, `0: TAG "OFF" 1: TAG "ON"`, layout
// 1016), MANDATORY on every write, with no "leave it alone" encoding — so a
// non-Known display value is never quietly omitted from the wire, it is
// REFUSED outright by buildWriteCommand. Dropping it from this set
// therefore cannot let a non-Known value through. What the conditional
// fixes is which gate such a channel meets FIRST: on a session whose
// FieldTagDisplay is not write-Supported, the capability gate would
// otherwise refuse naming a field NOBODY ASKED TO WRITE, instead of the
// refusal that names the real problem.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
	}
	if data.TagDisplay.State == codeplug.Known {
		fields = append(fields, spec.FieldTagDisplay)
	}
	if data.CTCSSTone.State == codeplug.Known {
		fields = append(fields, spec.FieldCTCSSTone)
	}
	if data.ScanSkip.State == codeplug.Known {
		fields = append(fields, spec.FieldScanSkip)
	}
	for _, t := range tierRequestedFields {
		if t.present(data) {
			fields = append(fields, t.field)
		}
	}
	return fields
}

// tierRequestedFields pairs each spec.Field the Icom tier added with a
// predicate reporting whether this channel's data actually REQUESTS it —
// i.e. carries a Known value for it. It is the mirror of codeplug's
// tierAddedFieldFor (diff.go), down to the order: ChannelData's own
// declaration order, appended AFTER the pre-tier set.
//
// Every one of these predicates answers false for a channel this driver
// produced: an FT-891 read leaves all seventeen tier fields UNAVAILABLE
// (read.go's channelData, plan P12), and a load of a schema-1/2/3 file
// migrates to the same. So the ordinary write is unchanged by their
// presence, and what they add is the one case the gate would otherwise miss
// — a caller who hands WriteChannel a ChannelData with a Known tier value,
// which this radio's 41-byte record cannot express and which must therefore
// be REFUSED rather than dropped.
//
// SEVENTEEN, matching read.go's channelData, which sets all seventeen
// Unavailable on a read (plan P12): codeplug.diff.go's tierAddedFieldFor
// carries the D4 (Icom-tier) ten AND the D8 (receiver) seven, and
// touchedFields appends all seventeen (core/codeplug/tier_test.go:868 says
// so verbatim). This table used to carry only the first ten — the
// pre-second-extension count, copied unchanged from the FT-710/FTdx10/
// FTdx101 mirror when the D8 seven landed there but not here — which meant
// a channel carrying a Known TuningStep, Preamp or AttenuatorDB was
// silently written with those values dropped rather than refused. Fixed
// here; the sibling drivers carry the identical ten-entry gap and are a
// tracked fleet follow-up, not this task's to touch.
//
// tierAddedFieldFor itself is unexported, so this table cannot import it;
// codeplug offers no exported enumeration bound to ChannelData's tier
// fields either (only the unbound spec.Field constants themselves, via
// spec.AllFields). What follows is therefore the brief's prescribed
// fallback: a LOCAL list of all seventeen, built from the same exported
// building blocks tierAddedFieldFor itself is built from (the spec.Field
// constants and ChannelData's own exported struct fields) — and
// TestRequestedFields_MembershipAndOrder's "every tier predicate is
// reachable" subtest pins both the count AND, independently of any count,
// that it names exactly the seventeen fields spec.AllFields() carries
// after the ten pre-tier ones, so a future spec.Field addition that this
// table fails to mirror fails that test rather than passing silently.
var tierRequestedFields = []struct {
	field   spec.Field
	present func(codeplug.ChannelData) bool
}{
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldToneMode, func(d codeplug.ChannelData) bool { return d.ToneMode.State == codeplug.Known }},
	{spec.FieldToneTx, func(d codeplug.ChannelData) bool { return d.ToneTx.State == codeplug.Known }},
	{spec.FieldToneRx, func(d codeplug.ChannelData) bool { return d.ToneRx.State == codeplug.Known }},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
	{spec.FieldFilter, func(d codeplug.ChannelData) bool { return d.Filter.State == codeplug.Known }},
	{spec.FieldDataMode, func(d codeplug.ChannelData) bool { return d.DataMode.State == codeplug.Known }},
	{spec.FieldTuningStepEnabled, func(d codeplug.ChannelData) bool { return d.TuningStepEnabled.State == codeplug.Known }},
	{spec.FieldTuningStep, func(d codeplug.ChannelData) bool { return d.TuningStep.State == codeplug.Known }},
	{spec.FieldProgramTuningStep, func(d codeplug.ChannelData) bool { return d.ProgramTuningStepHz.State == codeplug.Known }},
	{spec.FieldAttenuator, func(d codeplug.ChannelData) bool { return d.AttenuatorDB.State == codeplug.Known }},
	{spec.FieldPreamp, func(d codeplug.ChannelData) bool { return d.Preamp.State == codeplug.Known }},
	{spec.FieldAntenna, func(d codeplug.ChannelData) bool { return d.Antenna.State == codeplug.Known }},
	{spec.FieldIPPlus, func(d codeplug.ChannelData) bool { return d.IPPlus.State == codeplug.Known }},
}

// WriteChannel implements driver.Session: ONE combined 41-byte MT Set,
// fire-and-forget with the transport's bounded "?;" listen.
//
// MT-ONLY, AND MW IS NEVER SENT — the mirror of the read path's decision.
// The 41-byte Set carries the whole field block, the LIVE display flag and
// the tag in a single frame (layout 996-1027), where MW's 28 bytes would
// write the same fields redundantly and could carry neither the tag nor the
// flag at all (1034-1042); MW's own P1 legend is restricted to memory and
// PMS (1035-1036), a second reason not to reach for it. That ONE combined
// Set SUFFICES to create or overwrite a channel — including one that does
// not yet exist — is the driver register's A SINGLE COMBINED MT SET
// SUFFICES TO CREATE OR OVERWRITE A CHANNEL entry, and it is the assumption
// this whole choreography rests on (matrix §3.6).
//
// THE REFUSAL LADDER, IN PLAN P7's ORDER, ALL PRE-WIRE: ParseSlot, bankFor,
// the erase refusal, the FieldState sanity checks, THE CAPABILITY GATE,
// then — inside buildWriteCommand — the two mandatory semantic refusals a
// non-Known TagDisplay and a true TxClar earn, then the frame. The
// capability gate comes BEFORE the semantic refusals (matrix erratum M-E3,
// the FT-710's shipped order), so on an unconsented RealHardware session
// the all-Unverified gate is the first answer and the semantic refusals are
// what a Simulated — or a consented — session meets. In particular:
//
//   - On the all-Unverified fail-safe profile, which is what a REAL FT-891
//     gets while writeTrialsComplete is false, NOTHING is writable, so
//     every channel is refused here with every requested field named. That
//     is the point of the profile, not a limitation of this method. The one
//     route past it is the user's own recorded consent
//     (WithConsentedUnverifiedWrites).
//
//   - A Known CTCSS tone or scan skip is refused even on the Simulated
//     profile: the 41-byte record has no tone-number and no skip byte (the
//     ASSUMED register's TONE AND SCAN-SKIP UNREACHABILITY entry for what
//     that does and does not establish), so silently dropping a value the
//     caller explicitly marked Known would be a lie. The same holds for any
//     of the seventeen Icom-tier fields.
//
//   - A DISCOVERED 5xx/EMG slot is refused by the same gate on every
//     profile: readOnlyFields (caps.go) forces every Write on those banks
//     to spec.Unsupported, and this dialect would refuse to build the frame
//     in any case.
//
//   - An empty channel (erase) is refused. This radio's Control Command
//     List (layout 111-147) is its entire CAT command set and contains no
//     erase command at all, so there is no erase to offer and
//     spec.FieldErase is nowhere write-Supported. NO CAT-ERASE CLAIM IS
//     MADE FOR THIS RADIO IN EITHER DIRECTION: whether some Set frame has
//     an erasing side effect is unknown, and Unsupported is the direction
//     that needs no evidence (matrix §2.6).
//
//   - The clarifier is NOT spec.Inert in any profile here, so the Inert leg
//     of the gate has nothing to act on — Inert is the FT-710's HARDWARE
//     finding about the FT-710's clarifier and no FT-891 has ever been
//     asked (doc.go's non-borrowing note). The gate still accepts Inert, so
//     that a future Stage W finding marking this radio's clarifier Inert
//     need not also change this method.
//
// THE OPERATION MUTEX IS HELD FOR THE WHOLE CALL (plan P3, spec S-E4,
// matrix M-E2). One Set is one exchange and transport.Engine already
// serialises each exchange, so the lock buys nothing for the write
// considered alone; what it buys is that a write cannot land inside a
// concurrent ReadChannel's TWO-exchange cross-check, where it would change
// the very slot whose MT rejection the MR is about to interpret.
// TestWriteChannel_CannotLandInsideAReadsCrossCheck is the pin. It is taken
// even before the refusal checks, since a refused write returns without
// wire traffic either way.
//
// IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair is core/clone's, as
// the driver interface assigns it (matrix M-E2).
//
// NO READ-BACK HAPPENS HERE. WriteChannel reports only sent/unrejected (see
// driver.WriteResult); reading the slot back and comparing is the clone
// service's job, so verification policy — when, how often, what to do on a
// mismatch — lives in one place above every driver rather than
// half-implemented inside each. On this radio clone's verify read is the
// MT+MR cross-check read.go describes, so a verify that draws "?;" on an
// occupied slot fails the same typed way rather than being read as "the
// write emptied the channel".
//
// THE END-TO-END WRITE through the registered fake is NOT here and is not
// missing: internal/fakeft891 lands on the other Stage 2 lane, and the
// registration task carries the end-to-end leg. What this file's tests
// cover is the choreography against a scripted, frame-parsing peer, which
// is where a wrong BYTE is visible, plus the write→verify pair driven by
// core/clone itself (diffgate_test.go).
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// Every refusal below this point returns res unchanged, i.e. an
	// EXPLICITLY EMPTY step list — never nil. The distinction is not
	// cosmetic: the clone service journals this result, and a nil slice
	// marshals as JSON null, which an auditor would have to read as
	// "unknown" rather than the truth, "no frame was ever built, so nothing
	// was attempted".
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	if _, err := s.dialect.ParseSlot(ch.Slot); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid slot: %v", err)}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		// Also where the answer-only none form "000" stops: grammatical per
		// ParseSlot (the DIALECT register's SlotSpace.NoneWire = "000"
		// entry) and in no bank, so it is refused here rather than by the
		// builder further down.
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	if ch.Empty() {
		// An empty channel is an ERASE request, and it is refused — see
		// this method's doc comment for why no CAT-erase claim is made for
		// this radio in either direction. This rung must also stay AHEAD of
		// the FieldState checks below structurally, not merely by
		// preference: an empty channel has no Data at all, and those checks
		// dereference it.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this radio's CAT command set contains no erase command at all (layout 111-147), so this codec cannot express an erase, and FieldErase is not write-Supported",
		}
	}

	// FieldState sanity before anything else trusts .State: a malformed
	// field (an unrecognised State, or a non-Known value smuggled alongside
	// a value) is refused, not interpreted. EVERY field of ch.Data that
	// carries a FieldState, not the three (CTCSSTone, ScanSkip, TagDisplay)
	// this rung checked before closing-review wave 2 — that is C-M1, the
	// gap it closed: a tier field carrying State Unavailable and a non-zero
	// Value passed both codeplug.Validate (which skips non-Recorded fields
	// outright) and this rung's old three-field list, so requestedFields
	// (Known-only) never named it and the malformed value was silently
	// DROPPED from the frame rather than refused.
	// TestWriteChannel_RefusalLadder's "an incoherent TxFreqHz field is
	// refused, not interpreted (C-M1)" row is still the pin, and its
	// comment still quotes the pre-C-M1 transcript.
	//
	// THE WALK IS THE FLEET'S, driver.CheckFieldStates, not this package's
	// own table any more (write-gate sweep item (i), 05/09/2026): the same
	// twenty fields in the same order, judged against the same typed
	// validators, so the C-M1 verdict is unchanged — with ONE deliberate
	// relaxation recorded here. Wave 2's local table called Valid()
	// unconditionally, which REFUSED codeplug.Absent as well, and the fleet
	// stance (the IC-9700's, ic9700/write.go's validateKnownValues) admits
	// it: a caller who set nothing has requested nothing, and refusing
	// those would refuse every ordinary MODIFY that a hand-built
	// ChannelData produces. TestWriteChannel_AbsentFieldStatesStillWrite is
	// that relaxation's pin.
	//
	// This is NOT the mandatory-flag refusal TagDisplay separately earns,
	// which is a different question about a well-formed field and lives in
	// buildWriteCommand — and which is why an Absent TagDisplay is still
	// refused on THIS radio, one rung further down.
	if field, err := driver.CheckFieldStates(s.caps, *ch.Data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	// THE CAPABILITY GATE (defence in depth below the clone service): every
	// requested field must pass spec.FieldSupport.CanWrite for this slot's
	// bank in THIS session's capabilities — spec.Supported, or
	// spec.ConsentedUnverified, which is the label every writable field of
	// a consented real-hardware FT-891 session carries (this radio's write
	// trials are outstanding, so consent is the only key that opens this
	// gate on RealHardware today — see sessionCapabilities, ft891.go) — OR
	// spec.Inert, which is acceptable to TRANSMIT. The other half of the
	// Inert rule, blocking a CHANGED Inert value, needs the BASELINE and
	// lives in codeplug.Diff, which has both sides.
	var unwritable []spec.Field
	for _, f := range requestedFields(*ch.Data) {
		fs := s.caps.FieldSupport(bank, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session (the CAT codec cannot express the field, or this session's capability profile does not support writing it)",
		}
	}

	// Build the frame before any wire traffic, so a mapping, a semantic
	// refusal or a validation failure can still refuse the whole write
	// cleanly.
	cmd, err := buildWriteCommand(s.dialect, ch)
	if err != nil {
		return res, err
	}

	// THE step list, declared in full HERE: after the frame provably
	// exists, before it goes near the wire. It has ONE element because this
	// radio's write choreography IS one frame — that is the whole content
	// of the MT-only decision, expressed in the neutral seam's own terms. A
	// one-step list is not a degenerate case of the FT-710's two-step one:
	// driver.WriteResult exists in this shape precisely so that a radio
	// writing a channel and its label in ONE combined frame can say so.
	res.Steps = []driver.WriteStep{{Command: "MT"}}
	const mtStep = 0

	if _, err := s.eng.Do(ctx, cmd, mtSetSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			// The frame WAS transmitted; the radio explicitly refused it.
			// Sent true, Confirmed false: the outcome is attributable, and
			// it is a refusal. That a rejected Set draws exactly one "?;"
			// is the register's THE ACKNOWLEDGEMENT CONVENTIONS entry.
			res.Steps[mtStep].Sent = true
			return res, fmt.Errorf("ft891: WriteChannel %s: MT rejected by radio: %w", ch.Slot, err)
		}
		// Transport-level failure: the frame's fate is NOT attributable —
		// the host cannot tell whether it reached the radio — so Sent stays
		// false and the error, not the flags, carries the distinction.
		return res, fmt.Errorf("ft891: WriteChannel %s: MT: %w", ch.Slot, err)
	}
	// Confirmed on SILENCE, which is the other half of the same ASSUMED
	// convention: nothing came back within the transport's bounded listen.
	res.Steps[mtStep].Sent, res.Steps[mtStep].Confirmed = true, true

	return res, nil
}

// buildWriteCommand maps a populated channel onto its ONE combined MT Set
// frame, refusing (typed, via *driver.WriteRefusedError) any value the
// codec cannot express. Called only after WriteChannel's capability gate
// has passed — plan P7's order, matrix M-E3.
//
// SINGULAR — buildWriteCommand, not the FT-710's buildWriteCommands — and
// the name is the design: there is one frame, and a plural name would
// invite a second.
//
// THE TWO MANDATORY SEMANTIC REFUSALS COME FIRST, BEFORE ANY FIELD MAPPING,
// and their POSITION is load-bearing rather than stylistic: a channel that
// is wrong in several ways at once must still name one of THESE fields,
// because these are the two whose failure mode is a silent wrong byte on
// the wire rather than a refusal. This radio is the first registered one to
// need BOTH, and they are the two axes on which its combined form differs
// from every sibling's.
//
//  1. A NON-KNOWN TagDisplay. Byte 28 is a LIVE TAG flag here — MT's P11
//     legend reads `0: TAG "OFF" 1: TAG "ON"` (layout 1016) where every
//     registered combined-form sibling prints "0: (Fixed)" — and the frame
//     has no "leave it alone" encoding, so sending such a channel would
//     MANUFACTURE a value for a field whose FieldState says "preserve
//     whatever the radio has", which is exactly what codeplug's write rule
//     forbids. The FT-710 refuses in this same position for this same
//     reason; the FTdx10 has no such refusal because it has no such flag
//     (matrix §3.7). codeplug.Diff blocks such a channel at PLAN time,
//     which is the user-facing route with the helpful message — a
//     CHIRP-imported row arrives Unknown and blocks there — and this is the
//     belt to that pair of braces. core/cat is the third: under
//     P11TagDisplay the display-LESS builder refuses outright, so there is
//     no path to a manufactured byte 28 at all.
//
//  2. A TRUE TxClar. This radio's P5 legend prints `0: (Fixed)` on every
//     block carrying the 28-position grid — MR 971, MT 1006, MW 1042, IF
//     783, OI 1129 — where the three registered sibling dialects print
//     `0: TX CLAR "OFF" 1: TX CLAR "ON"`. It is transcribed, not assumed.
//     THE REFUSAL IS EXPLICIT RATHER THAN A CAPABILITY GRADE, and matrix
//     §2.2 is why: ClarHz, RxClar and TxClar are three Go fields under ONE
//     spec.Field, spec.FieldClarifier (core/codeplug/diff.go:127-129), so
//     grading the field unwritable would block the offset and the RX flag
//     as well, on a radio whose frame carries both in positions 15-20. The
//     field is graded normally and the TX half is refused HERE, by name.
//     There is NO plan-time (Diff) gate for it this milestone — a shared
//     seam, deliberately not opened — so a foreign TxClar-true channel
//     (from a native file written for an FT-710 or FTdx10, a CSV import, or
//     a GUI paste) aborts a send at this slot rather than being blocked
//     before it starts. core/cat is the backstop: under P5Fixed both
//     builders refuse a TxClar-true record (mw.go:161-163,
//     mtcombined.go:158-160), each naming P5 and the policy.
//
// The rest is the family's checklist: mode, CTCSS state and shift resolved
// through the name maps, the clarifier bounds-checked against THIS
// dialect's policy before the int16 conversion can wrap, the frequency
// converted through the checked narrowing, and the builder given the last
// word on everything else.
func buildWriteCommand(dialect cat.Dialect, ch codeplug.Channel) (cat.Command, error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	if data.TagDisplay.State != codeplug.Known {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTagDisplay},
			Reason: fmt.Sprintf("tag display FieldState is %q, not %q; this radio's P11 (byte 28) is a LIVE TAG flag with no \"leave it alone\" encoding, so only a Known value is ever sent", data.TagDisplay.State, codeplug.Known),
		}
	}
	if data.TxClar {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: "this radio's P5 (byte 21) legend prints \"0: (Fixed)\" on every block that carries the field grid, so it has no TX clarifier flag to set; the offset and the RX flag are writable and only the TX half is refused, because all three are one spec.Field",
		}
	}

	// Resolved through THIS dialect, not a driver-private mode table:
	// caps.go's modeNames enumerates the dialect too, so a dialect that
	// renamed a mode changes both what this radio advertises and what this
	// write path accepts, which is the only coherent arrangement.
	mode, ok := dialect.ModeByName(data.Mode)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not a mode this radio supports", data.Mode),
		}
	}
	ctcss, ok := ctcssByName[data.CTCSS]
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSState},
			Reason: fmt.Sprintf("ctcss state %q is not one of OFF/ENC-DEC/ENC", data.CTCSS),
		}
	}
	shift, ok := shiftByName[data.Shift]
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldShift},
			Reason: fmt.Sprintf("shift %q is not one of SIMPLEX/PLUS/MINUS", data.Shift),
		}
	}
	// Bounds-check BEFORE the int -> int16 conversion below can wrap; the
	// builder re-validates magnitude AND step on top. The bound is THIS
	// DIALECT'S, in the comparison and in the message alike — and on this
	// radio both halves of it are the DIALECT register's single ASSUMED
	// entry ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz =
	// 9990, cited rather than restated (the manual prints 9999 and states
	// no step; caps.go says how 9990 is deduced).
	clar := dialect.Clarifier()
	if data.ClarHz < -clar.MaxAbsHz || data.ClarHz > clar.MaxAbsHz {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: fmt.Sprintf("clarifier %d Hz exceeds +/-%d Hz", data.ClarHz, clar.MaxAbsHz),
		}
	}
	// The ONE checked conversion between the neutral model's uint64
	// frequency and this protocol's uint32 (design D4): core/cat stays
	// uint32 because a NEWCAT memory frame carries nine digits and can
	// express nothing wider, so a bare cast would truncate an out-of-range
	// value into a plausible small one and send it. The arm is unreachable
	// for any channel that came through codeplug.Validate, which refuses
	// anything above this radio's 56 MHz ceiling before WriteChannel ever
	// sees it — true of the clone service's caller, not of WriteChannel
	// itself, which does not call Validate. It is a refusal, not a cast,
	// so it stays unreachable-for-that-caller by construction rather than
	// by habit.
	freqHz, err := cat.MemoryFreqHz(data.FreqHz)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	// ONE frame: the whole channel, its display flag and its tag. The tag
	// is passed as the LOGICAL value — the builder pads it to the full
	// 12-byte field with the dialect's own TagFill (its ASSUMED register
	// entry MTPolicy.TagFill = ' ') and refuses a tag that would not
	// round-trip, so no padding happens here.
	//
	// THE DISPLAY-BEARING BUILDER IS THE ONLY ONE THIS DIALECT ADMITS:
	// under MTPolicy.P11 = cat.P11TagDisplay, BuildMTSetCombined refuses
	// outright rather than writing a '0' the caller never expressed an
	// intention about, and this is the first registered radio for which
	// that path is live (matrix §3.7).
	//
	// KIND: the FORM's schema constant, cat.CombinedMTSetKind ("0:
	// (Fixed)", layout 1011), and deliberately NOT dialect.MWWriteKind().
	// MT-Set P7 and MW-Set P7 are two independent facts of this radio that
	// happen to agree — the manual prints "(Fixed)" for each separately
	// (1011 and 1047) — and the FT-710 is the counter-example that makes
	// the point, documenting '1' there. Deriving one from the other would
	// make this write path depend on a coincidence (matrix §3.6). This
	// driver sends no MW frame at all, so it has no business consulting
	// MW's kind.
	cmd, err := dialect.BuildMTSetCombinedDisplay(cat.MemoryData{
		Slot:   sl,
		FreqHz: freqHz,
		ClarHz: int16(data.ClarHz),
		RxClar: data.RxClar,
		// Always false by the time we reach here — the refusal above is
		// what makes that true — and carried through from the channel
		// rather than hard-coded, so that a change to either policy shows
		// up here rather than being masked.
		TxClar: data.TxClar,
		Mode:   mode,
		Kind:   cat.CombinedMTSetKind,
		CTCSS:  ctcss,
		Shift:  shift,
	}, data.Tag, data.TagDisplay.Value)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Reason: fmt.Sprintf("cannot encode the combined MT Set frame: %v", err),
		}
	}
	return cmd, nil
}

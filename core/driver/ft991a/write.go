// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

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
// They live HERE, in the write file, rather than beside their forward twins,
// because they are the write direction's own vocabulary — and because a pair
// of hand-written maps meant to be exact inverses is worth a test rather
// than an adjacency: TestNameMaps_AreExactInverses walks both directions
// over both pairs, so a spelling added to one map and forgotten in the other
// fails rather than silently refusing a legitimate value at the write gate
// (or, worse, mapping it onto the wrong byte).
//
// Deliberately NOT cat.CTCSSState.String()/cat.Shift.String(): those
// spellings ("off", "ENC/DEC", "DCS ENC/DEC") are LOG LABELS, and the
// strings these keys must match are the ones codeplug.Validate checks for
// and this driver's Capabilities advertises (caps.go's ctcssStates and
// spec.StandardShiftOptions).
//
// FIVE STATES, WHERE EVERY REGISTERED SIBLING HAS THREE, and that is the
// whole reason this radio is in this fleet. Its P8 legend prints
// `0: CTCSS "OFF" 1: CTCSS ENC/DEC 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC`
// identically on all five blocks that carry the field — IF 795-796, MR
// 977-978, MT 1010-1011, MW 1048-1049, OI 1128-1129 (matrix §1.17) — and
// under the dialect's cat.ToneStatesCTCSSAndDCS all five may be BUILT. That
// the radio ACCEPTS a DCS state written with no CN code sent first is the
// SHARED register's THE DCS STATES' SET ACCEPTANCE entry; what happens to
// the code the radio already holds is the DRIVER register's A DCS-STATE
// CHANNEL'S CODE SURVIVES A REWRITE entry. The two spellings are the ruled
// ones (decisions.md cell 5), and TestNameMaps_AreExactInverses holds this
// map against caps.go's published list so neither can drift.
var ctcssByName = map[string]cat.CTCSSState{
	"OFF":         cat.CTCSSOff,
	"ENC-DEC":     cat.CTCSSEncDec,
	"ENC":         cat.CTCSSEnc,
	"DCS-ENC-DEC": cat.CTCSSDCSEncDec,
	"DCS-ENC":     cat.CTCSSDCSEnc,
}

// shiftByName is shiftNames' write-direction inverse — this radio's P10
// legend, "0: Simplex 1: Plus Shift 2: Minus Shift", printed identically on
// MR (981), MT (1014), MW (1051), IF (799) and OI (1132).
var shiftByName = map[string]cat.Shift{
	"SIMPLEX": cat.ShiftSimplex,
	"PLUS":    cat.ShiftPlus,
	"MINUS":   cat.ShiftMinus,
}

// mtSetSpec is the transport spec for the combined MT SET:
// transport.CATWriteSpec(), the transport's fire-and-forget mode — write the
// frame, listen for a bounded window in case a "?;" rejection arrives, and
// treat silence as acceptance.
//
// THAT SHAPE IS ASSUMED ON BOTH COUNTS, NOT MANUAL-EVIDENCED. Silence-on-
// success and exactly one "?;" on rejection are the SHARED register's THE
// ACKNOWLEDGEMENT CONVENTIONS entry, cited here rather than restated: this
// manual describes no ACK/NAK vocabulary at all, and it never says whether
// an accepted Set answers. Nor does MT's availability row supply one —
// `MT | MEMORY CHANNEL TAG | O O O X` (layout 181) marks which FORMS the
// command has, not what a Set draws back; reading a reply convention off
// that row was the FTdx10 milestone's erratum 1 and is not repeated here
// (matrix §3.6).
//
// Every part of the spec is load-bearing, and it is why this is a separate
// function from read.go's mtSpec rather than a reuse of it:
//
//   - NO answer matcher, and therefore no derived answer length. mtSpec pins
//     the combined ANSWER's exact geometry from the dialect, which is right
//     for a read and would be a bug here: on the assumed convention an
//     accepted Set produces no answer at all, so a spec that waited for an
//     "MT" reply would spend the whole read timeout and then report a
//     timeout for a write the radio had accepted perfectly. On THIS radio
//     the two frames share a prefix AND, for the Set and the Answer, the
//     same 41 positions, so the mistake would look plausible.
//
//   - RetryReads 0, necessarily. A write is NEVER resent — transport safety
//     obligation 2 enforces this structurally, and Do refuses any
//     write-class spec with a non-zero RetryReads outright, before writing
//     anything. Resending an accepted Set would write the channel twice;
//     resending one whose fate is unknown would write it a second time on
//     top of a first that may have landed.
//
// TestMTSetSpec_IsFireAndForgetAndNeverRetries pins both properties and that
// this spec is not the read's.
func mtSetSpec() transport.CommandSpec {
	return transport.CATWriteSpec()
}

// bankFor reports which of this session's banks claims slot.
//
// A linear walk over the session's banks, which on this radio are exactly
// the two caps.go declares — there is NO discovery here (doc.go, "There is
// NO discovery"), so the effective set is the profile's own and a slot in no
// bank is one this radio does not have.
//
// IT IS NOT A SIXTH RUNG. It is the second half of resolving the slot, at
// the same site as ParseSlot: the capability gate below needs a bank to
// grade a field against, and a bank that does not exist cannot grade
// anything. What it refuses in its own right is the answer-only none form
// "000" — grammatical per ParseSlot (the DIALECT register's ASSUMED
// SlotSpace.NoneWire entry, which cat.SlotSpace structurally requires) and
// in no bank. THE FT-891 COUNTS THE SAME CODE SHAPE AS ITS OWN RUNG
// ("ParseSlot, bankFor, …", ft891/write.go) — matrix §3.6's own numbered
// ladder omits bankFor, which is why this file does not follow it there.
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
// write rule, Unknown and Unavailable both mean "preserve whatever the radio
// has", i.e. nothing is requested for that field.
//
// It mirrors the DIFF LAYER'S REQUESTED-SET DERIVATION exactly — same
// membership, the same conditionals, the same order — so this driver's
// defence-in-depth gate and the diff layer's gate judge the same set for the
// same channel. That derivation is two pieces on the codeplug side and both
// are mirrored here: addedFields' six unconditional plus three conditional
// fields, and then the SEVENTEEN Icom-tier conditionals codeplug carries in
// tierAddedFieldFor and appends in touchedFields (see tierRequestedFields).
// The seventeen come LAST, in ChannelData's declaration order, exactly as
// they do there, so no BlockReason a user has ever read is reordered by
// their arrival. Mirrored, NOT imported: tierAddedFieldFor is unexported,
// and the mirror is held by both sides pinning the same shape
// (TestRequestedFields_MembershipAndOrder here, codeplug's own tests there).
//
// TAGDISPLAY'S CONDITIONAL IS KEPT, AND ON THIS RADIO IT DOES A DIFFERENT
// JOB FROM THE FT-891'S. There, byte 28 is a live flag and the conditional
// decides which gate a non-Known value meets first. Here the field does not
// exist at all — MT's P11 legend prints "0: (Fixed)" (layout 1015), caps.go
// grades spec.FieldTagDisplay the zero FieldSupport, and the display-LESS
// builder writes byte 28 as the form's own constant — so a non-Known value
// is simply not requested and nothing about it reaches the wire. What the
// conditional buys is the OTHER direction: a caller who explicitly marks
// TagDisplay Known is asking for something this record cannot express, and
// naming the field to the capability gate is what turns that into a refusal
// rather than a silently dropped value. The mirror of codeplug.Diff is exact
// either way, which is the reason the conditional is not dropped as dead.
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
// produced: an FT-991A read leaves all seventeen tier fields UNAVAILABLE
// (read.go's mapping, plan P12), and a load of a schema-1/2/3 file migrates
// to the same. So the ordinary write is unchanged by their presence, and
// what they add is the one case the gate would otherwise miss — a caller who
// hands WriteChannel a ChannelData with a Known tier value, which this
// radio's 41-byte record cannot express and which must therefore be REFUSED
// rather than dropped.
//
// SEVENTEEN, matching read.go's mapping, which sets all seventeen
// Unavailable on a read: codeplug's tierAddedFieldFor carries the D4
// (Icom-tier) ten AND the D8 (receiver) seven, and touchedFields appends all
// seventeen. The FT-891 shipped a TEN-entry version of this table for a
// while — the pre-second-extension count — which meant a channel carrying a
// Known TuningStep, Preamp or AttenuatorDB was silently written with those
// values dropped rather than refused; that was its closing review's HIGH-1,
// and this table starts life with all seventeen rather than repeating it.
//
// tierAddedFieldFor itself is unexported, so this table cannot import it;
// codeplug offers no exported enumeration bound to ChannelData's tier fields
// either (only the unbound spec.Field constants, via spec.AllFields). What
// follows is therefore a LOCAL list of all seventeen, built from the same
// exported building blocks tierAddedFieldFor is built from — and
// TestRequestedFields_MembershipAndOrder pins both the count AND,
// independently of any count, that it names exactly the seventeen fields
// spec.AllFields() carries after the ten pre-tier ones, so a future
// spec.Field addition this table fails to mirror fails that test rather than
// passing silently.
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
// The 41-byte Set carries the whole field block and the tag in a single
// frame (layout 998-1033), where MW's 28 bytes would write the same fields
// redundantly in a strictly smaller frame and could not carry the tag at all
// (1036-1051). MW's Read and Answer LABELS are printed at 1048 and 1051 with
// the grids beneath them EMPTY, which agrees with the availability row
// `MW | MEMORY WRITE | O X X X` (183) and with core/cat/mw.go — a trap for
// anything deriving frame shapes mechanically, and named as one in the spec
// (matrix §3.6). That ONE combined Set SUFFICES to create or overwrite a
// channel — that the radio accepts it as a complete channel definition — is
// the DRIVER register's own A SINGLE COMBINED MT SET SUFFICES TO CREATE OR
// OVERWRITE A CHANNEL entry (doc.go); that its silence means acceptance is
// the SHARED register's THE ACKNOWLEDGEMENT CONVENTIONS entry.
//
// THE REFUSAL LADDER IS THE FOLDED MATRIX §3.6's FIVE RUNGS, ALL PRE-WIRE:
// ParseSlot (with bankFor resolving the slot's bank at the same site), the
// EMPTY-CHANNEL rung, driver.CheckFieldStates, THE CAPABILITY GATE, then
// core/cat's own builders. The capability gate comes BEFORE the builders
// (decisions.md cell 10, the fleet order) and AFTER driver.CheckFieldStates,
// which precedes it in every shipped Yaesu driver. In particular:
//
//   - On the all-Unverified fail-safe profile, which is what a REAL FT-991A
//     gets while writeTrialsComplete is false, NOTHING is writable, so every
//     channel is refused here with every requested field named. That is the
//     point of the profile, not a limitation of this method. The one route
//     past it is the user's own recorded consent
//     (WithConsentedUnverifiedWrites).
//
//   - A Known CTCSS tone or scan skip is refused even on the Simulated
//     profile: the 41-position record has no tone-number and no skip byte
//     (the register's TONE-NUMBER UNREACHABILITY and SCAN-SKIP
//     UNREACHABILITY entries for what that does and does not establish), so
//     silently dropping a value the caller explicitly marked Known would be
//     a lie. The same holds for a Known TagDisplay and for any of the
//     seventeen Icom-tier fields.
//
//   - An empty channel (erase) is refused. This radio's Control Command List
//     (layout 123-196) is its entire CAT command set and contains no erase
//     command at all, so there is no erase to offer and spec.FieldErase is
//     nowhere write-Supported. NO CAT-ERASE CLAIM IS MADE FOR THIS RADIO IN
//     EITHER DIRECTION: whether some Set frame has an erasing side effect is
//     unknown, and Unsupported is the direction that needs no evidence
//     (matrix §2.6).
//
//   - The clarifier is NOT spec.Inert in either profile here, so the Inert
//     leg of the gate has nothing to act on — Inert is the FT-710's HARDWARE
//     finding about the FT-710's clarifier and no FT-991A has ever been
//     asked. The gate still accepts Inert, so that a future Stage W finding
//     marking this radio's clarifier Inert need not also change this method.
//
// TWO RUNGS THE FT-891 HAS ARE DELIBERATELY ABSENT, and their absence is the
// kind a reviewer should be told to look for rather than left to notice
// (matrix erratum M-E3, and the matrix's §3.6 says so in terms):
//
//   - NO TxClar RUNG. Byte 21 is a LIVE TX-clarifier state here — `P5 0: TX
//     CLAR "OFF" 1: TX CLAR "ON"` on MR 971, MT 1004, MW 1042, IF 787 and OI
//     1122 — where the FT-891 prints "0: (Fixed)" and refuses a TxClar-true
//     record by name. Under the dialect's cat.P5TxClar core/cat ENCODES the
//     flag, so a copied rung would refuse, on every write, a field this
//     radio's five legends print as live.
//
//   - NO TagDisplay RUNG. Byte 28 is printed "P11 0: (Fixed)" (1015), so the
//     field does not exist here and the display-LESS builder is the only one
//     this dialect admits. A copied rung would demand a value for a field
//     the frame has no room for and refuse every ordinary channel.
//
// THE OPERATION MUTEX IS HELD FOR THE WHOLE CALL (spec erratum S-E4, matrix
// M-E2). One Set is one exchange and transport.Engine already serialises each
// exchange, so the lock buys nothing for the write considered alone — and on
// this radio the READ is one exchange too, so there is no cross-check for a
// write to land inside, which is the FT-891's reason and is not available
// here. TODAY, WITH EVERY OPERATION THIS SESSION PERFORMS BEING A SINGLE
// EXCHANGE, the engine's own per-exchange serialisation already keeps a
// write's and a read's frames from interleaving, so opMu buys this method
// nothing that the engine does not already give it. It is taken even before
// the refusal checks, since a refused write returns without wire traffic
// either way.
//
// WHAT THE LOCK IS FOR IS TASK 12's SETTINGS READ, a MULTI-exchange operation
// this session does not yet have. Once it lands, a write racing a settings
// read could otherwise put its one MT Set frame between two frames of the
// settings read's own exchange — a DRIVER OPERATION interleaving the engine's
// per-exchange serialisation cannot prevent, because it only serialises one
// exchange at a time. TASK 12 MUST PIN THIS: park the settings read inside
// opMu via a gap hook that lets a test attempt a concurrent WriteChannel
// mid-sequence, and assert that no MT Set frame reaches the wire until the
// settings operation completes.
//
// IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair is core/clone's, as the
// driver interface assigns it.
//
// NO READ-BACK HAPPENS HERE (plan P12). WriteChannel reports only
// sent/unrejected (see driver.WriteResult); reading the slot back and
// comparing is the clone service's job, so verification policy — when, how
// often, what to do on a mismatch — lives in one place above every driver
// rather than half-implemented inside each. An extra MT exchange inside this
// method would also change transcript counts, on a radio whose read and
// write share one prefix.
//
// THE END-TO-END WRITE through the registered fake is NOT here and is not
// missing: internal/fakeft991a lands on the other Stage 2 lane, and the
// registration task carries the end-to-end leg. What this file's tests cover
// is the choreography against a scripted, frame-parsing peer, which is where
// a wrong BYTE is visible, plus the write→verify pair driven by core/clone
// itself (diffgate_test.go).
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// Held for the WHOLE operation — see the doc comment and the Session
	// type's.
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
		// Also where the sibling PMS TOKEN form dies: "P1L" is a slot on
		// every registered sibling and a NON-SLOT here, because this
		// radio's PMS pairs are the decimal channel numbers 100-117 (its MC
		// legend, layout 916). Refused before any frame is built.
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid slot: %v", err)}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		// The answer-only none form "000" — see bankFor.
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	if ch.Empty() {
		// An empty channel is an ERASE request, and it is refused — see
		// this method's doc comment for why no CAT-erase claim is made for
		// this radio in either direction. This rung must also stay AHEAD of
		// the two rungs below STRUCTURALLY, not merely by preference: an
		// empty channel has no Data at all, and both the FieldState walk
		// and the capability gate dereference it. Without this rung,
		// WriteChannel nil-dereferences on the exact input codeplug.Diff
		// produces for a deletion.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this radio's CAT command set contains no erase command at all (layout 123-196), so this codec cannot express an erase, and FieldErase is not write-Supported",
		}
	}

	// FieldState sanity before anything else trusts .State: a malformed
	// field (an unrecognised State, or a non-Known value smuggled alongside
	// a value) is refused, not interpreted. EVERY field of ch.Data that
	// carries a FieldState, through the FLEET's own walk — the 05/09 Yaesu
	// write-gate sweep's item (i), which replaced every driver's hand-picked
	// table with this one (plan P12).
	//
	// IT MATTERS MORE ON THIS RADIO THAN ON THE FT-891, because there is no
	// second rung behind it for the fields the record cannot express.
	// requestedFields is Known-only and this frame has no position for a
	// tone number, a skip flag or a display flag, so a field carrying a
	// non-Known State ALONGSIDE a value would be named by nothing at all
	// and its value silently DROPPED — the FT-891's C-M1 — were this walk
	// not here. It admits codeplug.Absent provided the value is zero: a
	// caller who set nothing has requested nothing, which is the fleet
	// stance and the reason TestWriteChannel_AbsentFieldStatesStillWrite
	// exists.
	if field, err := driver.CheckFieldStates(s.caps, *ch.Data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	// THE CAPABILITY GATE (defence in depth below the clone service): every
	// requested field must pass spec.FieldSupport.CanWrite for this slot's
	// bank in THIS session's capabilities — spec.Supported, or
	// spec.ConsentedUnverified, which is the label every writable field of a
	// consented real-hardware FT-991A session carries (this radio's write
	// trials are outstanding, so consent is the only key that opens this
	// gate on RealHardware today — see sessionCapabilities, ft991a.go) — OR
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

	// Build the frame before any wire traffic, so a mapping, a bound or a
	// validation failure can still refuse the whole write cleanly.
	cmd, err := buildWriteCommand(s.dialect, ch)
	if err != nil {
		return res, err
	}

	// THE step list, declared in full HERE: after the frame provably exists,
	// before it goes near the wire. It has ONE element because this radio's
	// write choreography IS one frame — that is the whole content of the
	// MT-only decision, expressed in the neutral seam's own terms.
	res.Steps = []driver.WriteStep{{Command: "MT"}}
	const mtStep = 0

	if _, err := s.eng.Do(ctx, cmd, mtSetSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			// The frame WAS transmitted; the radio explicitly refused it.
			// Sent true, Confirmed false: the outcome is attributable, and
			// it is a refusal. That a rejected Set draws exactly one "?;"
			// is the SHARED register's THE ACKNOWLEDGEMENT CONVENTIONS
			// entry.
			//
			// AND IT IS NEVER "the slot is empty". "?;" is this protocol's
			// single unattributed NAK, and this driver makes exactly ONE
			// interpretation of it — MT "?;" on a READ means the slot is
			// empty (read.go, the register's own entry). The Set's "?;" is
			// reported as what it is: a rejection.
			res.Steps[mtStep].Sent = true
			return res, fmt.Errorf("ft991a: WriteChannel %s: MT rejected by radio: %w", ch.Slot, err)
		}
		// Transport-level failure: the frame's fate is NOT attributable —
		// the host cannot tell whether it reached the radio — so Sent stays
		// false and the error, not the flags, carries the distinction. The
		// wrap adds the slot the transport cannot know and nothing else,
		// which is the same slot-naming wrap read.go gives every transport
		// failure on its own path.
		return res, fmt.Errorf("ft991a: WriteChannel %s: MT: %w", ch.Slot, err)
	}
	// Confirmed on SILENCE, which is the other half of the same ASSUMED
	// convention: nothing REJECTING came back within the transport's bounded
	// listen. An unrecognised frame arriving in that window is counted by
	// the engine (safety obligation 3) and does not fail the write — the
	// convention claims silence-or-"?;" and nothing else, so a third thing
	// cannot be read as a refusal without inventing a meaning for it.
	res.Steps[mtStep].Sent, res.Steps[mtStep].Confirmed = true, true

	return res, nil
}

// buildWriteCommand maps a populated channel onto its ONE combined MT Set
// frame, refusing (typed, via *driver.WriteRefusedError) any value the codec
// cannot express. Called only after WriteChannel's capability gate has
// passed — decisions.md cell 10's order.
//
// SINGULAR — buildWriteCommand, not the FT-710's buildWriteCommands — and
// the name is the design: there is one frame, and a plural name would invite
// a second.
//
// THERE ARE NO MANDATORY SEMANTIC REFUSALS AT ALL HERE, and that is this
// radio rather than an omission. The FT-891 opens this function with two,
// and each is inverted on the FT-991A (matrix erratum M-E3): its TagDisplay
// refusal exists because byte 28 is a live flag there and it is SCHEMA here;
// its TxClar refusal exists because byte 21 is fixed there and it is LIVE
// here. What is left is the family's checklist — mode, CTCSS state and shift
// resolved through the name maps, the clarifier bounds-checked against THIS
// dialect's policy before the int16 conversion can wrap, the frequency
// converted through the checked narrowing, and the builder given the last
// word on everything else.
func buildWriteCommand(dialect cat.Dialect, ch codeplug.Channel) (cat.Command, error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	// Resolved through THIS dialect, not a driver-private mode table:
	// caps.go's modeNames enumerates the dialect too, so a dialect that
	// renamed a mode changes both what this radio advertises and what this
	// write path accepts, which is the only coherent arrangement. It matters
	// more here than on any sibling: this manual prints "C4FM" for nibble
	// 'E' where the FTdx10 prints "PSK", so core/cat's package-level
	// fallback is ACTIVELY WRONG for this radio and a driver-private table
	// would be a second place for that to go wrong.
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
			Reason: fmt.Sprintf("ctcss state %q is not one of OFF/ENC-DEC/ENC/DCS-ENC-DEC/DCS-ENC", data.CTCSS),
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
	// radio both halves of it are the SHARED register's single ASSUMED entry
	// ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990, cited
	// rather than restated (the manual prints "0000 - 9999 (Hz)" on all five
	// blocks and states no step; 9990 is the largest multiple of the assumed
	// step inside the printed range, which is a DEDUCTION FROM AN ASSUMPTION
	// and not a transcription).
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
	// anything above this radio's 470 MHz ceiling before WriteChannel ever
	// sees it — true of the clone service's caller, not of WriteChannel
	// itself, which does not call Validate. It is a refusal, not a cast, so
	// it stays unreachable-for-that-caller by construction rather than by
	// habit. (That the FA/FB range is also the MEMORY-STORABLE range is the
	// DRIVER register's MinFreqHz 30 000 / MaxFreqHz 470 000 000 entry; this
	// conversion is about the ENCODING's width, which is a different fact.)
	freqHz, err := cat.MemoryFreqHz(data.FreqHz)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	// ONE frame: the whole channel and its tag. The tag is passed as the
	// LOGICAL value — the builder pads it to the full 12-byte field with the
	// dialect's own TagFill (the SHARED register's ASSUMED MTPolicy.TagFill
	// = ' ' entry) and refuses a tag that would not round-trip, so no
	// padding happens here.
	//
	// THE DISPLAY-LESS BUILDER IS THE ONLY ONE THIS DIALECT ADMITS: under
	// MTPolicy.P11 = cat.P11Fixed, BuildMTSetCombinedDisplay refuses
	// outright, because there is no flag for a caller to set. There is no
	// display argument at this call site at all, which is the structural
	// form of "this radio has no TAG display flag".
	//
	// KIND: the FORM's schema constant, cat.CombinedMTSetKind ("Set:
	// 0: (Fixed)", layout 1009), and deliberately NOT dialect.MWWriteKind().
	// MT-Set P7 and MW-Set P7 are two independent facts of this radio that
	// happen to agree — the manual prints "(Fixed)" for each separately
	// (1009 and 1047) — and the FT-710 is the counter-example that makes the
	// point, documenting '1' there. Deriving one from the other would make
	// this write path depend on a coincidence (matrix §3.6). This driver
	// sends no MW frame at all, so it has no business consulting MW's kind.
	cmd, err := dialect.BuildMTSetCombined(cat.MemoryData{
		Slot:   sl,
		FreqHz: freqHz,
		ClarHz: int16(data.ClarHz),
		RxClar: data.RxClar,
		// LIVE ON THIS RADIO, and carried through from the channel rather
		// than forced false: byte 21 is `P5 0: TX CLAR "OFF" 1: TX CLAR
		// "ON"` on all five blocks carrying the grid, the dialect declares
		// cat.P5TxClar, and core/cat encodes the flag. The FT-891 has a
		// refusal here instead; see this file's WriteChannel doc comment.
		TxClar: data.TxClar,
		Mode:   mode,
		Kind:   cat.CombinedMTSetKind,
		CTCSS:  ctcss,
		Shift:  shift,
	}, data.Tag)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Reason: fmt.Sprintf("cannot encode the combined MT Set frame: %v", err),
		}
	}
	return cmd, nil
}

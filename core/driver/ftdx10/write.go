// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

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
// because a pair of hand-written maps that are meant to be exact inverses
// is worth a test rather than an adjacency: TestNameMaps_AreExactInverses
// walks both directions over both pairs, so a spelling added to one map and
// forgotten in the other fails rather than silently refusing a legitimate
// value at the write gate (or, worse, mapping it onto the wrong byte).
//
// Deliberately NOT cat.CTCSSState.String()/cat.Shift.String(): those
// spellings ("off", "ENC/DEC") are log labels, and the strings this map's
// keys must match are the ones codeplug.Validate checks for and this
// driver's Capabilities advertises (spec.StandardCTCSSStates,
// spec.StandardShiftOptions).
var ctcssByName = map[string]cat.CTCSSState{
	"OFF":     cat.CTCSSOff,
	"ENC-DEC": cat.CTCSSEncDec,
	"ENC":     cat.CTCSSEnc,
}

// shiftByName is shiftNames' write-direction inverse. See ctcssByName.
var shiftByName = map[string]cat.Shift{
	"SIMPLEX": cat.ShiftSimplex,
	"PLUS":    cat.ShiftPlus,
	"MINUS":   cat.ShiftMinus,
}

// mtSetSpec is the transport spec for the combined MT SET: the ZERO
// CommandSpec, which is transport's fire-and-forget mode — write the frame,
// then listen for a bounded window in case a "?;" rejection arrives, and
// treat silence as acceptance.
//
// Every part of that zero value is load-bearing, and it is why this is a
// separate function from read.go's mtSpec rather than a reuse of it:
//
//   - NO ExpectPrefix, and therefore no derived ExpectLen. mtSpec pins the
//     combined ANSWER's exact 41-byte geometry from the dialect, which is
//     right for a read and would be a bug here: a CAT Set produces no
//     answer at all, so a spec that waited for an "MT" reply would spend
//     the whole read timeout and then report a timeout for a write that
//     the radio had accepted perfectly. The absence of a prefix is what
//     selects transport's fire-and-forget path (see transport.Engine.Do).
//
//   - RetryReads 0, necessarily. A write is NEVER resent — transport
//     safety obligation 2 enforces this structurally, and Do refuses a
//     fire-and-forget spec with a non-zero RetryReads outright (with
//     ErrInvalidSpec, before writing anything). Resending an accepted Set
//     would write the channel twice; resending one whose fate is unknown
//     would write it a second time on top of a first that may have landed.
//
// The FT-710's driver spells the same thing fnfSpec (core/driver/ft710);
// the mechanics are identical because they are the TRANSPORT's, not either
// radio's, and this driver names it for the one command it uses it with.
func mtSetSpec() transport.CommandSpec {
	return transport.CommandSpec{}
}

// bankFor reports which of this session's banks claims slot.
//
// A linear walk over the session's EFFECTIVE banks (s.caps, which includes
// the discovered read-only 60M/EMG banks), so a slot the radio does not
// have is not in any bank and a write to it is refused outright rather than
// gated per-field against a bank that does not exist.
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
// the six plain fields ALWAYS (the combined MT Set carries frequency, mode,
// clarifier, CTCSS state, shift AND the tag in one frame, whether or not
// any of them changed), plus TagDisplay, CTCSSTone and ScanSkip when — and
// only when — their FieldState is Known. Per codeplug's write rule,
// Unknown and Unavailable both mean "preserve whatever the radio has", i.e.
// nothing is requested for that field.
//
// It mirrors core/driver/ft710's requestedFields, and through it
// codeplug.Diff's addedFields, EXACTLY: same membership, the same three
// conditionals, the same order — so this driver's defence-in-depth gate and
// the diff layer's gate judge the same set for the same channel.
// TestRequestedFields_MembershipAndOrder pins it here as the FT-710's own
// test does there. (Mirrored, NOT imported: core/driver/ftdx10 imports
// core/driver/ft710 nowhere, by the rule in doc.go.)
//
// THE TAGDISPLAY CONDITIONAL IS LOAD-BEARING ON THIS RADIO IN THE OPPOSITE
// DIRECTION TO THE FT-710'S, and this is the mechanical half of doc.go's
// named inversion:
//
//   - On the FT-710 the conditional keeps a NON-Known display value away
//     from the capability gate, so that it meets the refusal that names the
//     real problem (its buildWriteCommands refuses a non-Known TagDisplay
//     outright, MT's display flag being mandatory there) rather than a gate
//     complaining about a field nobody asked to write.
//
//   - On the FTdx10 there is no such refusal and no such flag — the
//     combined form has nowhere to put one — and EVERY channel this driver
//     reads carries TagDisplay Unavailable (read.go). The conditional is
//     therefore what lets an ordinary FTdx10 channel be written AT ALL:
//     without it, spec.FieldTagDisplay would be requested on every write,
//     its Write support is spec.Unsupported in every profile and bank
//     (caps.go's bankFields — a manual fact), and the gate below would
//     refuse every write this driver could ever make.
//
//   - And the same conditional is what CATCHES a Known TagDisplay: a
//     channel carrying one (a file written for a radio that has the flag)
//     requests the field, the gate finds it Unsupported, and the write is
//     refused naming spec.FieldTagDisplay. That refusal is the FTdx10's
//     whole answer to a display value, and it is the capability gate's, not
//     a builder's.
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
	return fields
}

// WriteChannel implements driver.Session: ONE combined MT Set,
// fire-and-forget with the transport's bounded "?;" listen.
//
// MT-ONLY, and MW is never sent — the mirror of the read path's decision
// (doc.go, "MR is deliberately unused"). The 41-byte Set carries the whole
// field block AND the tag in a single frame, so an MW would write the very
// same five fields a second time; a second frame would add a torn-write
// window and nothing else. That a single combined Set SUFFICES to create or
// overwrite a channel — including one that does not yet exist — is ASSUMED,
// register entry 9, and it is the assumption this whole choreography rests
// on. It is also why this Session needs no operation mutex: one logical
// operation is one exchange, and transport.Engine already serialises each
// exchange (see Session's own doc comment).
//
// Refusal comes FIRST, before ANY wire traffic — defence in depth below the
// clone service. Even if every layer above failed, this method re-derives
// the channel's requested field set (requestedFields) and re-checks each
// against THIS session's capabilities. In particular:
//
//   - On the all-Unverified fail-safe profile — which is what a REAL
//     FTdx10 gets while writeTrialsComplete is false — NOTHING is writable,
//     so every channel is refused here, with every requested field named.
//     That is the point of the profile, not a limitation of this method.
//
//   - A Known CTCSS tone or scan skip is refused even on the Simulated
//     profile: the combined record has no tone-number and no skip byte
//     (ASSUMED-register entry 6 for what that does and does not establish),
//     so silently dropping a value the caller explicitly marked Known would
//     be a lie.
//
//   - A KNOWN TagDisplay is refused BY THE CAPABILITY GATE — see
//     requestedFields, and buildWriteCommand's inversion comment. There is
//     no separate display-flag refusal in this ladder, because there is no
//     display flag.
//
//   - An UNAVAILABLE (or Unknown) TagDisplay is NOT refused, on any
//     profile: it is the state every FTdx10 channel legitimately carries,
//     it requests nothing, and the frame has no field it could reach.
//
//   - An empty channel (erase) is refused. The FTdx10's CAT command set
//     contains no erase command, so this driver has no erase to offer and
//     spec.FieldErase is nowhere write-Supported (caps.go). NO CAT-ERASE
//     CLAIM IS MADE FOR THIS RADIO, in either direction: the FT-710's "no
//     CAT erase exists" is HW-CONFIRMED for the FT-710 and is not borrowed,
//     and whether some FTdx10 Set frame has an erasing side effect is
//     simply unknown. Unsupported is the direction that needs no evidence.
//
//   - The clarifier is NOT spec.Inert in any profile here, so the Inert
//     leg of the FT-710's gate has nothing to act on — see doc.go's
//     non-borrowing note. The gate still accepts Inert, so that a future
//     Stage W finding can mark the field Inert without this method also
//     having to change.
//
// NO read-back: WriteChannel reports only sent/unrejected (see
// driver.WriteResult). Reading the slot back and comparing is the clone
// service's job, exactly as for the FT-710 — verification policy lives in
// one place above every driver rather than half-implemented inside each.
//
// The END-TO-END write — Simulated profile, through the registered fake, on
// the real wiring path — is NOT here and is not missing: internal/fakedx10
// does not exist yet, and that test lands with it (M9c-6 plan, task 6:
// "the task-2-deferred end-to-end test lands here"). What this file's tests
// cover is the choreography against a scripted, frame-parsing peer, which
// is where a wrong BYTE is visible; what task 6 adds is a radio that
// remembers what it was told.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
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
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	if ch.Empty() {
		// An empty channel is an ERASE request, and it is refused. See
		// WriteChannel's doc comment for why no CAT-erase claim is made for
		// this radio in either direction. This rung must also stay AHEAD of
		// the FieldState checks below structurally, not merely by
		// preference: an empty channel has no Data at all, and the checks
		// below dereference it.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "the FTdx10's CAT command set has no erase command, so this codec cannot express an erase, and FieldErase is not write-Supported",
		}
	}

	// FieldState sanity before anything else trusts .State: a malformed
	// field (an unrecognised State, or a non-Known value smuggled alongside
	// a value) is refused, not interpreted. TagDisplay is checked here for
	// exactly the same reason as its two neighbours and for no other: an
	// Unavailable-with-a-true-Value BoolField is incoherent whatever the
	// radio can express, and this driver must not read a coherent meaning
	// out of it.
	if err := ch.Data.CTCSSTone.Valid(s.caps); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone}, Reason: err.Error()}
	}
	if err := ch.Data.ScanSkip.Valid(); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldScanSkip}, Reason: err.Error()}
	}
	if err := ch.Data.TagDisplay.Valid(); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldTagDisplay}, Reason: err.Error()}
	}

	// THE write gate (defence in depth below the clone service): every
	// requested field must be write-Supported for this slot's bank in THIS
	// session's capabilities — OR spec.Inert, which is acceptable to
	// TRANSMIT. No field of this driver's is Inert today (doc.go's
	// non-borrowing note: Inert is an FT-710 hardware finding about ITS
	// clarifier, and no FTdx10 has been asked), so the Inert arm is
	// currently unreachable here and is kept deliberately: it is the
	// neutral rule spec.Inert documents, and a Stage W finding that marked
	// this radio's clarifier Inert must not also have to change this gate.
	// The other half of the Inert rule — blocking a CHANGED Inert value —
	// needs the BASELINE and lives in codeplug.Diff, which has both sides.
	//
	// This is also the rung that catches a Known TagDisplay, and the ONLY
	// one that ever will: see requestedFields and buildWriteCommand.
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

	// Build the frame before any wire traffic, so a mapping or validation
	// failure can still refuse the whole write cleanly.
	cmd, err := buildWriteCommand(s.dialect, ch)
	if err != nil {
		return res, err
	}

	// THE step list, declared in full HERE: after the frame provably
	// exists, before it goes near the wire. It has ONE element because this
	// radio's write choreography IS one frame — that is the whole content
	// of the MT-only decision, expressed in the neutral seam's own terms.
	//
	// A one-step list is not a degenerate case of the FT-710's two-step
	// one: driver.WriteResult exists in this shape (rather than as the four
	// booleans M9c-5's E6 retired) precisely so that a radio writing a
	// channel and its label in ONE combined frame can say so, instead of
	// having to answer "did the tag frame go?" about a frame it never
	// intended to send.
	res.Steps = []driver.WriteStep{{Command: "MT"}}
	const mtStep = 0

	if _, err := s.eng.Do(ctx, cmd, mtSetSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			// The frame WAS transmitted; the radio explicitly refused it.
			// Sent true, Confirmed false: the outcome is attributable, and
			// it is a refusal.
			res.Steps[mtStep].Sent = true
			return res, fmt.Errorf("ftdx10: WriteChannel %s: MT rejected by radio: %w", ch.Slot, err)
		}
		// Transport-level failure: the frame's fate is NOT attributable —
		// the host cannot tell whether it reached the radio — so Sent stays
		// false and the error, not the flags, carries the distinction.
		return res, fmt.Errorf("ftdx10: WriteChannel %s: MT: %w", ch.Slot, err)
	}
	res.Steps[mtStep].Sent, res.Steps[mtStep].Confirmed = true, true

	return res, nil
}

// buildWriteCommand maps a populated channel onto its ONE combined MT Set
// frame, refusing (typed, via *driver.WriteRefusedError) any value the
// codec cannot express. Called only after WriteChannel's capability gate has
// passed.
//
// SINGULAR — buildWriteCommand, not the FT-710's buildWriteCommands — and
// the name is the design: there is one frame, and a plural name would
// invite a second.
//
// # THE NAMED INVERSION
//
// THERE IS NO NON-KNOWN-TAGDISPLAY REFUSAL HERE, and its absence is a
// decision, not an omission. This is the exact spot at which
// core/driver/ft710's buildWriteCommands refuses a channel whose TagDisplay
// is not Known — top of the function, before any other field mapping,
// because on that radio MT's display flag (P1) is MANDATORY and the frame
// has no "leave it alone" encoding, so sending such a channel would
// MANUFACTURE a value for a field whose FieldState says "preserve whatever
// the radio has". A reader who knows that code will look for the same
// refusal here, so here is what is true instead:
//
//   - The FTdx10's combined MT record HAS NO DISPLAY FLAG AT ALL. Its 41
//     positions are fully accounted for (caps.go's bankFields lists them),
//     and cat.Dialect.BuildMTSetCombined's signature — (MemoryData, tag) —
//     takes no display argument, because there is nowhere to put one. A
//     manual fact, not an assumption.
//
//   - So a non-Known TagDisplay manufactures NOTHING here. There is no
//     byte to invent, and the write is ordinary. This matters because
//     Unavailable is what EVERY channel this driver reads carries
//     (read.go): a refusal in this position would refuse every FTdx10
//     write there is.
//
//   - A *Known* TagDisplay — a channel from a file written for a radio
//     that does have the flag — is still refused, by the CAPABILITY GATE in
//     WriteChannel, one rung earlier: requestedFields includes
//     spec.FieldTagDisplay exactly when the state is Known, and this
//     driver's FieldTagDisplay Write support is spec.Unsupported in every
//     profile and on every bank. So the value is never silently dropped,
//     and the refusal names the field — it simply comes from the gate
//     rather than from here. TestWriteChannel_KnownTagDisplayRefusedByTheGate
//     pins that it is the gate's refusal and not a build error, and
//     TestBuildWriteCommand_NoTagDisplayRefusalTakesPriority pins the
//     absence here as an ORDER property (a channel wrong in several ways
//     reports the FIRST check this function actually makes — mode — not a
//     display complaint).
//
// The rest of the function is the FT-710's checklist unchanged in shape:
// mode, CTCSS state and shift resolved through name maps, the clarifier
// bounds-checked against THIS dialect's policy before the int16 conversion
// can wrap, and the builder given the last word on everything else.
func buildWriteCommand(dialect cat.Dialect, ch codeplug.Channel) (cat.Command, error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	// Resolved through THIS dialect, not a driver-private mode table: the
	// FT-710's driver keeps one for historical reasons and pins it against
	// its dialect with a test; this package has none at all (caps.go's
	// modeNames enumerates the dialect), so a dialect that renamed a mode
	// changes what this write path accepts, which is the only coherent
	// arrangement.
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
	// DIALECT'S, in the comparison and in the message alike — the FTdx10's
	// 9990 Hz range is manual-evidenced and its 10 Hz step is the dialect's
	// own ASSUMED register entry 2, cited rather than restated here.
	clar := dialect.Clarifier()
	if data.ClarHz < -clar.MaxAbsHz || data.ClarHz > clar.MaxAbsHz {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: fmt.Sprintf("clarifier %d Hz exceeds +/-%d Hz", data.ClarHz, clar.MaxAbsHz),
		}
	}

	// ONE frame: the whole channel and its tag. The tag is passed as the
	// LOGICAL value — BuildMTSetCombined pads it to the full 12-byte field
	// with the dialect's own TagFill (its ASSUMED register entry 1) and
	// refuses a tag that would not round-trip, so no padding happens here.
	//
	// KIND: the FORM's schema constant, cat.CombinedMTSetKind ("0:
	// (Fixed)"), and deliberately NOT dialect.MWWriteKind(). The two are
	// EQUAL for this dialect — core/cat/ftdx10's MWWriteKind is
	// cat.CombinedMTSetKind, because this radio's MW P7 legend also reads
	// "(Fixed)" — so the choice moves no byte; what it decides is which
	// FACT this line names. MT-Set P7 and MW-Set P7 are two
	// command-specific facts that coincide on this radio (that dialect's
	// own comment says so: "equal to the combined MT Set constant AS A FACT
	// OF THIS RADIO, not a rule"), and deriving one from the other is the
	// PadByte conflation core/cat has already paid for once —
	// cat.CombinedMTSetKind's doc comment and
	// TestBuildMTSetCombined_P7IsAFormConstantNotTheMWWriteKind both refuse
	// it explicitly, the latter proving that a combined-form dialect whose
	// MW kind differs still emits '0' here AND has a record carrying its
	// own MW kind REFUSED. Naming MWWriteKind() would therefore have made
	// this driver's write path depend on a coincidence, and break — loudly,
	// but for an incomprehensible reason — for the first FTdx10-family
	// dialect whose MW kind was anything else. This driver sends no MW
	// frame at all (doc.go), so it has no business consulting MW's kind.
	// The constant is EXPORTED for exactly this naming (see its doc
	// comment). TestBuildWriteCommand_P7IsTheFormConstant pins both halves:
	// the byte on the wire, and that it is not read from MWWriteKind().
	cmd, err := dialect.BuildMTSetCombined(cat.MemoryData{
		Slot:   sl,
		FreqHz: data.FreqHz,
		ClarHz: int16(data.ClarHz),
		RxClar: data.RxClar,
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

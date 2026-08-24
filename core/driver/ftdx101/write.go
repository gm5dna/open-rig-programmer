// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

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
// of hand-written maps meant to be exact inverses is worth a test rather than
// an adjacency: TestNameMaps_AreExactInverses walks both directions over both
// pairs, so a spelling added to one map and forgotten in the other fails
// rather than silently refusing a legitimate value at the write gate (or,
// worse, mapping it onto the wrong byte).
//
// Deliberately NOT cat.CTCSSState.String()/cat.Shift.String(): those
// spellings ("off", "ENC/DEC") are log labels, and the strings these maps'
// keys must match are the ones codeplug.Validate checks for and this driver's
// Capabilities advertises (spec.StandardCTCSSStates, spec.StandardShiftOptions).
//
// ONE pair for BOTH models, like read.go's forward pair and for the same
// reason: the P8 and P10 legends are printed once in this manual with no
// model qualifier (matrix §1.14, §1.15), so a per-model table would be two
// copies of one fact.
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

// mtSetSpec is the transport spec for the combined MT Set: the ZERO
// CommandSpec, which is transport's fire-and-forget mode — write the frame,
// then listen for a bounded window in case a "?;" rejection arrives, and
// treat silence as acceptance.
//
// THAT SHAPE IS ASSUMED, NOT MANUAL-EVIDENCED — doc.go's register entry 9,
// second half, where it was registered at the M9d-2 milestone review. This
// comment previously argued it from MT's availability row (Set O, Read O,
// Answer O, AI X, layout 334); that is a non sequitur, because the table's
// "Ans." column (header at layout 236) marks the existence of the command's
// ANSWER FORM — the frame a READ draws — and says nothing about what a Set
// produces. Silence-on-success, and exactly one "?;" on rejection, are the
// FT-710's stated framing convention inherited here; this manual states
// neither. The capability matrix's §3.6 makes the same inference in the same
// words and an erratum is owed at its next revision (recorded in
// docs/superpowers/m9d2-baseline-manifest.md, "Note 6").
//
// Every part of that zero value is load-bearing, and it is why this is a
// separate function from read.go's mtSpec rather than a reuse of it:
//
//   - NO answer matcher, and therefore no answer length. read.go's mtSpec pins the
//     combined ANSWER's exact 41-byte geometry from the dialect, which is
//     right for a read and would be a bug here: on the assumed convention a
//     Set produces no answer at all, so a spec that waited for an "MT" reply
//     would spend the whole read timeout and then report a timeout for a
//     write the radio had accepted perfectly. The absence of a prefix is what
//     selects transport's fire-and-forget path (see transport.Engine.Do).
//
//   - Consequently this function needs no dialect argument and cannot fail,
//     where read.go's mtSpec needs both: the answer geometry it derives is
//     exactly what is absent here.
//
//   - RetryReads 0, necessarily. A write is NEVER resent — transport safety
//     obligation 2 enforces this structurally, and Do refuses a
//     fire-and-forget spec with a non-zero RetryReads outright (with
//     ErrInvalidSpec, before writing anything). Resending an accepted Set
//     would write the channel twice; resending one whose fate is unknown
//     would write it a second time on top of a first that may have landed.
//
// The FT-710's driver spells the same thing fnfSpec (core/driver/ft710) and
// the FTdx10's mtSetSpec; the mechanics are the TRANSPORT's, not any radio's.
func mtSetSpec() transport.CommandSpec {
	return transport.CATWriteSpec()
}

// bankFor reports which of this session's banks claims slot.
//
// A linear walk over the session's EFFECTIVE banks (s.caps, which includes
// the read-only 60M/EMG banks Open discovered), so a slot this radio does not
// have is in no bank and a write to it is refused outright rather than gated
// per-field against a bank that does not exist. Because the walk is over the
// EFFECTIVE set, a discovered 5xx or EMG slot IS found — and then refused by
// the capability gate below, every one of its fields being read-only (§1.3.5).
// That is the intended path: "this radio has no such channel" and "this
// channel is not writable" are different refusals and must read differently.
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
// clarifier, CTCSS state, shift AND the tag in one frame, whether or not any
// of them changed), plus TagDisplay, CTCSSTone and ScanSkip when — and only
// when — their FieldState is Known. Per codeplug's write rule, Unknown and
// Unavailable both mean "preserve whatever the radio has", i.e. nothing is
// requested for that field.
//
// It mirrors core/driver/ft710's and core/driver/ftdx10's requestedFields,
// and through them codeplug.Diff's addedFields, EXACTLY: same membership, the
// same three conditionals, the same order — so this driver's defence-in-depth
// gate and the diff layer's gate judge the same set for the same channel.
// TestRequestedFields_MembershipAndOrder pins it here as those packages' own
// tests pin it there. (Mirrored, NOT imported: this package imports no other
// driver package, by the rule in doc.go.)
//
// It is MODEL-INDEPENDENT, and takes no modelParams for that reason: §2 of
// the matrix is identical for the D and the MP throughout (§2.5), so the set
// of fields a channel requests cannot differ between them. A per-model
// variant would be two copies of one row.
//
// THE TAGDISPLAY CONDITIONAL IS LOAD-BEARING ON THIS RADIO IN THE OPPOSITE
// DIRECTION TO THE FT-710'S, and this is the mechanical half of the named
// inversion (matrix §3.7; the narrative half is at buildWriteCommand):
//
//   - On the FT-710 the conditional keeps a NON-Known display value away from
//     the capability gate, so that it meets the refusal naming the real
//     problem (its buildWriteCommands refuses a non-Known TagDisplay outright,
//     MT's display flag being mandatory there) rather than a gate complaining
//     about a field nobody asked to write.
//
//   - On the FTdx101 there is no such refusal and no such flag — P11 is "0:
//     (Fixed)" at layout 1329 and the combined form has nowhere to put one —
//     and EVERY channel this driver reads carries TagDisplay Unavailable
//     (read.go). The conditional is therefore what lets an ordinary FTdx101
//     channel be written AT ALL: without it, spec.FieldTagDisplay would be
//     requested on every write, its Write support is spec.Unsupported in every
//     profile and on every bank (caps.go's bankFields — matrix §3.7, a
//     MANUAL-EVIDENCED ABSENCE rather than an assumption), and the gate below
//     would refuse every write this driver could ever make.
//
//   - And the same conditional is what CATCHES a Known TagDisplay: a channel
//     carrying one (a file written for a radio that has the flag) requests the
//     field, the gate finds it Unsupported, and the write is refused naming
//     spec.FieldTagDisplay. That refusal is this driver's whole answer to a
//     display value, and it is the capability gate's, not a builder's.
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
// (doc.go, "MR is deliberately unused"; matrix §3.6). The 41-byte Set carries
// the whole field block AND the tag in a single frame, so an MW would write
// the very same fields a second time with a strictly smaller frame (28 bytes,
// layout 1352-1367) and would add a torn-write window for nothing; MW's own
// restriction to "001-099, P1L-P9U" (layout 1353) is a second reason not to
// reach for it. That a single combined Set SUFFICES to create or overwrite a
// channel — including one that does not yet exist — is ASSUMED, DRIVER
// register entry 9, per model, and it is the assumption this whole
// choreography rests on. It is also why this Session needs no operation
// mutex: one logical operation is one exchange, and transport.Engine already
// serialises each exchange (see Session's own doc comment).
//
// NOTHING MAY BE WRITTEN TO EITHER REAL RADIO AT M9d-2 (matrix §3.11), and
// this method is not what enforces that — the CAPABILITY PROFILE is.
// writeTrialsCompleteD and writeTrialsCompleteMP are both false, so a
// RealHardware session gets the all-Unverified fail-safe and the gate below
// refuses every field of every channel. This method is the choreography that
// becomes reachable when a profile allows it, and today only the Simulated
// profile does.
//
// Refusal comes FIRST, before ANY wire traffic — defence in depth below the
// clone service. Even if every layer above failed, this method re-derives the
// channel's requested field set (requestedFields) and re-checks each against
// THIS session's capabilities. In particular:
//
//   - On the all-Unverified fail-safe profile — what a REAL FTdx101 of either
//     model gets — NOTHING is writable, so every channel is refused here with
//     every requested field named. That is the point of the profile, not a
//     limitation of this method.
//
//   - A Known CTCSS tone or scan skip is refused even on the Simulated
//     profile: the combined record has no tone-NUMBER byte and no skip flag
//     (DRIVER register entry 6 for what that does and does not establish), so
//     silently dropping a value the caller explicitly marked Known would be a
//     lie.
//
//   - A KNOWN TagDisplay is refused BY THE CAPABILITY GATE — see
//     requestedFields, and buildWriteCommand's inversion comment. There is no
//     separate display-flag refusal in this ladder, because there is no
//     display flag.
//
//   - An UNAVAILABLE (or Unknown) TagDisplay is NOT refused, on any profile:
//     it is the state every FTdx101 channel legitimately carries (read.go),
//     it requests nothing, and the frame has no field it could reach.
//
//   - An empty channel (erase) is refused. This radio's command availability
//     table (layout 236-337) lists its entire CAT command set and contains NO
//     erase command (matrix §2.3, a MANUAL-EVIDENCED absence), so this driver
//     has no erase to offer and spec.FieldErase is nowhere write-Supported
//     (caps.go). Whether some Set frame has an erasing SIDE EFFECT on this
//     radio is unknown and deliberately not claimed in either direction:
//     Unsupported is the direction that needs no evidence, and the FT-710's
//     "no CAT erase exists" is HW-CONFIRMED for the FT-710 and is not
//     borrowed.
//
//   - The clarifier is NOT spec.Inert in any profile here, so the Inert leg of
//     the FT-710's gate has nothing to act on — see doc.go's "The Simulated
//     profile's clarifier is Supported, not Inert" and matrix §2.1. The gate
//     still accepts Inert, so that a future Stage W finding can mark the field
//     Inert for one model without this method also having to change.
//
// NO read-back: WriteChannel reports only sent/unrejected (see
// driver.WriteResult). Reading the slot back and comparing is the clone
// service's job, exactly as for the FT-710 and the FTdx10 — verification
// policy lives in one place above every driver rather than half-implemented
// inside each.
//
// The END-TO-END write — Simulated profile, through the registered fake, on
// the real wiring path — is NOT here, and it is not missing: it LIVES
// ELSEWHERE, in internal/wiring, as
// TestOpenFakeSessionFor_FTdx101DSimulatedWriteRoundTrip and
// TestOpenFakeSessionFor_FTdx101MPSimulatedWriteRoundTrip (M9d-2 task 7,
// which registered both models). It can only live there: the choreography
// needs a Simulated-profile driver paired with internal/fakedx101, and that
// pairing exists in exactly one place repo-wide — internal/wiring/fake.go's
// fakeDrivers table, pinned there by internal/guards'
// TestSimulatedProfileTokensConfinement.
//
// The division of labour is the point. What THIS file's tests cover is the
// choreography against a scripted, frame-parsing peer, which is where a
// wrong BYTE is visible; what the wiring tests add is a radio that remembers
// what it was told, so the two prove different things and neither
// substitutes for the other.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// Every refusal below this point returns res unchanged, i.e. an
	// EXPLICITLY EMPTY step list — never nil. The distinction is not
	// cosmetic: the clone service journals this result, and a nil slice
	// marshals as JSON null, which an auditor would have to read as "unknown"
	// rather than the truth, "no frame was ever built, so nothing was
	// attempted".
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
		// the FieldState checks below STRUCTURALLY, not merely by preference:
		// an empty channel has no Data at all, and the checks below
		// dereference it.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "the FTdx101's CAT command set has no erase command, so this codec cannot express an erase, and FieldErase is not write-Supported",
		}
	}

	// FieldState sanity before anything else trusts .State: a malformed field
	// (an unrecognised State, or a non-Known value smuggled alongside a
	// value) is refused, not interpreted. TagDisplay is checked here for
	// exactly the same reason as its two neighbours and for no other: an
	// Unavailable-with-a-true-Value BoolField is incoherent whatever the
	// radio can express, and this driver must not read a coherent meaning out
	// of it. This rung SURVIVES the named inversion — it is not the FT-710's
	// non-Known refusal wearing a different hat.
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
	// requested field must pass spec.FieldSupport.CanWrite for this slot's
	// bank in THIS session's capabilities — spec.Supported, or
	// spec.ConsentedUnverified, which is the label EVERY writable field of
	// a consented real-hardware FTdx101 session of either model carries
	// (neither model's write trials have run, so consent is the only key
	// that opens this gate on RealHardware today — see sessionCapabilities,
	// ftdx101.go) — OR spec.Inert, which is acceptable to
	// TRANSMIT. No field of this driver's is Inert today (matrix §2.1: Inert
	// is an FT-710 HARDWARE finding about ITS clarifier, and no FTdx101 of
	// either model has been asked), so the Inert arm is currently unreachable
	// here and is kept deliberately: it is the neutral rule spec.Inert
	// documents, and a Stage W finding that marked one model's clarifier
	// Inert must not also have to change this gate. The other half of the
	// Inert rule — blocking a CHANGED Inert value — needs the BASELINE and
	// lives in codeplug.Diff, which has both sides.
	//
	// This is also the rung that catches a Known TagDisplay, and the ONLY one
	// that ever will: see requestedFields and buildWriteCommand.
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
	// failure can still refuse the whole write cleanly. THIS SESSION'S
	// dialect, never a package-level one: this package drives two radios and
	// holds two dialects, and a write encoded with one model's dialect while
	// the engine gated with the other's would differ from a correct write
	// only in what the ID probe had accepted (see Session.dialect).
	cmd, err := buildWriteCommand(s.dialect, ch)
	if err != nil {
		return res, err
	}

	// THE step list, declared in full HERE: after the frame provably exists,
	// before it goes near the wire. It has ONE element because this radio's
	// write choreography IS one frame — that is the whole content of the
	// MT-only decision, expressed in the neutral seam's own terms.
	//
	// A one-step list is not a degenerate case of the FT-710's two-step one:
	// driver.WriteResult exists in this shape (rather than as the four
	// booleans M9c-5's E6 retired) precisely so that a radio writing a channel
	// and its label in ONE combined frame can say so, instead of having to
	// answer "did the tag frame go?" about a frame it never intended to send.
	res.Steps = []driver.WriteStep{{Command: "MT"}}
	const mtStep = 0

	if _, err := s.eng.Do(ctx, cmd, mtSetSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			// The frame WAS transmitted; the radio explicitly refused it.
			// Sent true, Confirmed false: the outcome is attributable, and it
			// is a refusal. ("?;" is the protocol's single unattributed NAK —
			// matrix §3.8 — so this says the radio would not take the frame,
			// and nothing about why.)
			res.Steps[mtStep].Sent = true
			return res, fmt.Errorf("ftdx101: WriteChannel %s: MT rejected by radio: %w", ch.Slot, err)
		}
		// Transport-level failure: the frame's fate is NOT attributable — the
		// host cannot tell whether it reached the radio — so Sent stays false
		// and the error, not the flags, carries the distinction.
		return res, fmt.Errorf("ftdx101: WriteChannel %s: MT: %w", ch.Slot, err)
	}
	// Confirmed means exactly "no rejection arrived in the error window", and
	// never "the radio acknowledged it" (driver.WriteStep): a CAT Set has no
	// acknowledgement to give.
	res.Steps[mtStep].Sent, res.Steps[mtStep].Confirmed = true, true

	return res, nil
}

// buildWriteCommand maps a populated channel onto its ONE combined MT Set
// frame, refusing (typed, via *driver.WriteRefusedError) any value the codec
// cannot express. Called only after WriteChannel's capability gate has
// passed.
//
// SINGULAR — buildWriteCommand, not the FT-710's buildWriteCommands — and the
// name is the design: there is one frame, and a plural name would invite a
// second.
//
// The DIALECT is a parameter rather than a package-level value because this
// package has no package-level dialect to reach for: two models, two
// dialects, and the caller above passes the one its own session Opened with.
//
// # THE NAMED INVERSION
//
// THERE IS NO NON-KNOWN-TAGDISPLAY REFUSAL HERE, and its absence is a
// decision, not an omission. This is the exact spot at which
// core/driver/ft710's buildWriteCommands refuses a channel whose TagDisplay
// is not Known — top of the function, before any other field mapping, because
// on that radio MT's display flag (P1) is MANDATORY and the frame has no
// "leave it alone" encoding, so sending such a channel would MANUFACTURE a
// value for a field whose FieldState says "preserve whatever the radio has".
// A reader who knows that code will look for the same refusal here, so here
// is what is true instead (matrix §3.7):
//
//   - THE FTDX101'S COMBINED MT RECORD HAS NO DISPLAY FLAG AT ALL. Its P11 is
//     "0: (Fixed)" (layout 1329), a single fixed position at byte 28, and the
//     frame's 41 positions are fully accounted for by an INDEPENDENT geometry
//     witness (core/cat/ftdx101/testdata/geometry-witness.csv, counted off 300
//     dpi raster renders: MT set/answer rows running MT, P1..P12, ';' over
//     1-41 with no gap). cat.Dialect.BuildMTSetCombined's signature —
//     (MemoryData, tag) — takes no display argument, because there is nowhere
//     to put one. A MANUAL-EVIDENCED ABSENCE, not an assumption, and one of
//     the few facts in this driver that needs no register entry.
//
//   - So a non-Known TagDisplay manufactures NOTHING here. There is no byte
//     to invent, and the write is ordinary. This matters because Unavailable
//     is what EVERY channel this driver reads carries (read.go): a refusal in
//     this position would refuse every FTdx101 write there is.
//
//   - A *Known* TagDisplay — a channel from a file written for a radio that
//     does have the flag — is still refused, by the CAPABILITY GATE in
//     WriteChannel, one rung earlier: requestedFields includes
//     spec.FieldTagDisplay exactly when the state is Known, and this driver's
//     FieldTagDisplay Write support is spec.Unsupported in every profile and
//     on every bank. So the value is never silently dropped, and the refusal
//     names the field — it simply comes from the gate rather than from here.
//     TestWriteChannel_KnownTagDisplayRefusedByTheGate pins that it is the
//     gate's refusal and not a build error, and
//     TestBuildWriteCommand_NoTagDisplayRefusalTakesPriority pins the absence
//     here as an ORDER property (a channel wrong in several ways reports the
//     FIRST check this function actually makes — mode — not a display
//     complaint).
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
	// FT-710's driver keeps one for historical reasons and pins it against its
	// dialect with a test; this package has none at all (caps.go's modeNames
	// enumerates the dialect), so a dialect that renamed a mode changes what
	// this write path accepts, which is the only coherent arrangement — and in
	// a two-model package it is also the only arrangement in which the mode
	// vocabulary provably belongs to the model that Opened.
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
	// DIALECT'S, in the comparison and in the message alike — the FTdx101's
	// 9990 Hz range is manual-evidenced (MT P3's four digits at positions
	// 16-19) and its 10 Hz step is the DIALECT register's entry
	// "ClarifierPolicy.StepHz = 10", cited by name rather than restated here.
	//
	// The SIGN the builder will emit for a negative offset is the DIALECT
	// register's entry "The CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII
	// HYPHEN-MINUS 0x2D ('-')" — this manual prints the minus direction as a
	// two-hyphen glyph and the golden deriver recorded it UNREADABLE (matrix
	// §2.1). Nothing here depends on which byte it is; the entry is named so
	// that a Stage W lift knows this call site is one of the two that emit it.
	clar := dialect.Clarifier()
	if data.ClarHz < -clar.MaxAbsHz || data.ClarHz > clar.MaxAbsHz {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: fmt.Sprintf("clarifier %d Hz exceeds +/-%d Hz", data.ClarHz, clar.MaxAbsHz),
		}
	}

	// ONE frame: the whole channel and its tag. The tag is passed as the
	// LOGICAL value — BuildMTSetCombined pads it to the full 12-byte field
	// with the dialect's own fill (the DIALECT register's entry
	// "MTPolicy.TagFill = ' '") and refuses a tag that would not round-trip,
	// so no padding happens here.
	//
	// KIND: the FORM's schema constant, cat.CombinedMTSetKind ("0: (Fixed)",
	// this radio's MT-Set P7 legend at layout 1324), and deliberately NOT
	// dialect.MWWriteKind(). The two are EQUAL for this dialect —
	// core/cat/ftdx101's MWWriteKind is cat.CombinedMTSetKind, because this
	// radio's MW-Set P7 legend also reads "(Fixed)" (layout 1364) — so the
	// choice moves no byte; what it decides is which FACT this line names.
	// Matrix §3.6 calls the pair out as "two supporting facts, both
	// MANUAL-EVIDENCED and both easy to conflate": they are two independent
	// facts of this radio that happen to agree, not one fact used twice.
	// Deriving one from the other is the PadByte conflation core/cat has
	// already paid for once — cat.CombinedMTSetKind's doc comment and
	// TestBuildMTSetCombined_P7IsAFormConstantNotTheMWWriteKind both refuse it
	// explicitly. Naming MWWriteKind() would have made this driver's write
	// path depend on a coincidence, and break — loudly, but for an
	// incomprehensible reason — for the first family dialect whose MW kind was
	// anything else. This driver sends no MW frame at all (doc.go), so it has
	// no business consulting MW's kind. TestBuildWriteCommand_P7IsTheFormConstant
	// pins both halves: the byte on the wire, and that it is not read from
	// MWWriteKind().
	// The ONE checked conversion between the neutral model's uint64
	// frequency and this protocol's uint32 (design D4, item 7):
	// core/cat stays uint32 because a NEWCAT memory frame carries nine
	// digits and can express nothing wider, so a bare cast here would
	// truncate an out-of-range value into a plausible small one and send
	// it. The arm is unreachable for this radio — Validate has already
	// refused anything above its 75 MHz ceiling — and it is a refusal,
	// not a cast, so it stays unreachable by construction rather than by
	// habit.
	freqHz, err := cat.MemoryFreqHz(data.FreqHz)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	cmd, err := dialect.BuildMTSetCombined(cat.MemoryData{
		Slot:   sl,
		FreqHz: freqHz,
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

// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The ASSUMED-register entries and design decisions this file's refusals
// name. They are cited BY NAME rather than by position, and the authoritative
// register is core/kw/ma/doc.go's — no entry is re-registered here.
//
// A REGISTER ENTRY IS PART OF THE REFUSAL, NOT A COMMENT ON IT. Every rung of
// this ladder answers "refused", and on an unconsented RealHardware session
// the CAPABILITY gate answers first for every write (writeTrialsComplete is
// false on this row), so a caller — or a test — that could see only the fact
// of refusal could not tell an unlifted assumption from a missing consent.
// RefusalError.Register is what distinguishes them, and each rung's own pin
// asserts it (write_test.go's assertRegister).
//
// A PRINTED FACT GETS NO REGISTER ENTRY AND NO TYPE. A mode this row does not
// publish, a name wider than the printed window and the fleet's own
// FieldState walk are refused with the PLAIN *driver.WriteRefusedError,
// because naming a register entry there would put an assumption's name on an
// ordinary typo — pair 1's rule (core/driver/ts590/write.go's fmP14), applied
// to this row's own rungs.
const (
	// registerDecision8 is the tone-domain asymmetry: the published TN
	// chart carries 1750 Hz at index 50 (990:4971) and the printed CN chart
	// stops at 49 (990:1262), and spec.Capabilities has no per-direction
	// tone axis to say so (M-E1).
	registerDecision8 = "decision 8"
	// registerDecision9 is the secondary side with no home in the neutral
	// model: the TX disposition a one-frame Set must emit either way
	// (rung 6, M-E8), the dual-reception flag and the frequency-2 tuple
	// (rung 11).
	registerDecision9 = "decision 9"
	// registerDecision15 is the standing no-erase rule, over a PRINTED MA5
	// on this row (990:3042-3047).
	registerDecision15 = "decision 15"
	// registerA2 is the tag charset: that the family default IS this
	// radio's set. The ';' half of it is forced rather than assumed — see
	// checkName. LIFT: L-HW-2, per registry row.
	registerA2 = "A2"
	// registerA3 is create-on-write: whether an MA0 Set alone can register
	// an unassigned channel is nowhere printed, and every other member of
	// the family says it cannot. LIFT: L-HW-3, hardware item 1.
	registerA3 = "A3"
	// registerA8 is the section-defined channel's frequency 2: whether P9
	// carries that slot's end frequency is nowhere printed — MA6 is the
	// end-frequency command here (990:3051-3059) — so what a whole-record
	// Set would overwrite there is unknown. LIFT: L-HW-6, per registry row.
	registerA8 = "A8"
	// registerA14 is the Class byte: the codec emits P2 = '0' on every
	// build because the book says the parameter "is ignored. Enter a dummy
	// value" and names no value (990:2901-2903). The milestone's ONLY
	// defaulted byte, and the reason a SIMPLEX candidate over a Dual TARGET
	// is this driver's refusal rather than the codec's. LIFT: L-HW-11.
	registerA14 = "A14"
)

// writeGapHook, when non-nil, is called by WriteChannel with opMu held,
// AFTER the pre-write MA0 read and BEFORE the Set — a test-only seam,
// mirroring read.go's readChannelGapHook. Always nil in production, so it
// costs one nil check per write.
//
// IT PARKS IN THE ONE GAP THAT MATTERS. This write is two exchanges, and
// what opMu buys over the engine's own per-exchange serialisation is that
// nothing lands BETWEEN them — a concurrent operation there would decide
// against one radio state and write against another. Parking anywhere else
// would pin a property transport.Engine already provides.
var writeGapHook func()

// ma0SetSpec is the transport spec for the ONE MA0 Set this driver sends:
// fire-and-forget — write the frame, listen for a bounded window in case a
// "?;" arrives, and treat silence as the end of the exchange.
//
// IT IS WRITTEN OUT RATHER THAN TAKEN FROM transport.CATWriteSpec(), whose
// value is identical: that helper's name says CAT, this family is not CAT,
// and a Kenwood file naming a Yaesu-codec helper is exactly the borrowing
// this milestone's fence exists to prevent. What is shared is the transport
// CLASS, which is neutral.
//
// SILENCE IS NOT SUCCESS ON THIS FAMILY, and that is why WriteChannel reports
// Sent rather than Confirmed — see there. What the bounded window buys is the
// REJECTION: a "?;" inside it is attributable.
//
// RetryReads 0, NECESSARILY: a write is never resent (transport safety
// obligation 2), and Do refuses a write-class spec with a non-zero
// RetryReads before writing anything.
// TestMA0SetSpec_IsFireAndForgetAndNeverRetries pins all three properties.
func ma0SetSpec() transport.CommandSpec {
	return transport.CommandSpec{Class: transport.ClassWrite}
}

// RefusalError is a write refusal that names the ASSUMED-register entry or
// design decision it comes from.
//
// IT IS A *driver.WriteRefusedError AND MORE, never instead: Unwrap returns
// the embedded fleet error, so errors.Is(err, driver.ErrWriteRefused) holds
// through it, errors.As recovers the fleet type for a caller that wants the
// slot and the fields, and errors.As recovers THIS type for a caller — or a
// test — that needs to know WHICH rung answered.
//
// WHY THE DISTINCTION MATTERS ENOUGH TO MINT A TYPE (plan P7's pinning
// paragraph): the capability gate refuses every write on an unconsented
// RealHardware session while writeTrialsComplete is false, and returns the
// PLAIN fleet error. A semantic pin that asserted only "refused" would
// therefore pass on the capability gate and the rung it claimed to pin need
// not exist at all. This type is what makes that impossible to write by
// accident.
type RefusalError struct {
	driver.WriteRefusedError
	// Register is the entry this refusal comes from — one of the register
	// constants above, and the same string Reason names in prose.
	Register string
}

// Unwrap exposes the fleet refusal, which in turn unwraps to
// driver.ErrWriteRefused.
func (e *RefusalError) Unwrap() error { return &e.WriteRefusedError }

// refuse builds a semantic refusal for slot, naming register in both the
// machine-readable field and the prose.
func refuse(slot, register string, fields []spec.Field, format string, args ...any) *RefusalError {
	return &RefusalError{
		WriteRefusedError: driver.WriteRefusedError{
			Slot:   slot,
			Fields: fields,
			Reason: register + ": " + fmt.Sprintf(format, args...),
		},
		Register: register,
	}
}

// plainRefusal is the fleet refusal a PRINTED fact earns: no register entry,
// because there is no assumption in it.
func plainRefusal(slot string, field spec.Field, format string, args ...any) *driver.WriteRefusedError {
	return &driver.WriteRefusedError{
		Slot:   slot,
		Fields: []spec.Field{field},
		Reason: fmt.Sprintf(format, args...),
	}
}

// toneModeWire is read.go's toneModeNames read backwards: the neutral
// vocabulary this row publishes mapped to the record's P6 byte.
//
// DERIVED RATHER THAN TRANSCRIBED. A hand-written inverse of a hand-written
// map is one edit from disagreeing, and inverting the forward map at package
// initialisation makes the disagreement unrepresentable instead.
var toneModeWire = invertToneModes()

func invertToneModes() map[string]byte {
	m := make(map[string]byte, len(toneModeNames))
	for wire, name := range toneModeNames {
		m[name] = wire
	}
	return m
}

// requestedFieldRules pairs each spec.Field with the predicate that reports
// whether a write of this channel actually REQUESTS it.
//
// TWENTY-SIX ENTRIES — every spec.Field but spec.FieldErase, in the same
// order this package's own literal list carries them (caps_test.go's
// allSpecFields; plan P5 permits these rows to name spec.AllFields,
// reversing pair 1's rule). FieldErase is not a field a write requests: it
// is the whole shape of a DIFFERENT frame — MA5, which this programme never
// builds (990:3042-3047) — and WriteChannel refuses an empty channel a rung
// above this table.
//
// EIGHT ARE UNCONDITIONAL, AND THAT IS THE FRAME'S OWN SHAPE. The 57-byte
// record carries a frequency, a mode, an FM width, a tone function, two tone
// indices, a second frequency with its split flag, a lockout byte and a name
// on EVERY write, changed or not, with no "leave it alone" encoding anywhere
// in the grid (990:2891-2956). A write therefore requests all eight whether
// the caller edited them or not, which is what makes the capability gate
// below a real gate.
//
// TWO OF THE EIGHT DIVERGE FROM PAIR 1, IN OPPOSITE DIRECTIONS, which is why
// this is a per-row table and not a shape borrowed from the sibling family:
// spec.FieldTxFrequency is UNCONDITIONAL here, because P9 and P15 go out on
// every Set (as the printed zeroed form when the channel is simplex, A16),
// where the 590 rows could only reach a transmit side through a second frame;
// and spec.FieldDataMode is CONDITIONAL here, because this grid has no data
// byte at all — the data-ness is inside the mode legend (M-E3).
//
// THE EIGHTEEN CONDITIONALS EXIST FOR ONE CASE: a caller who hands this
// driver a value the record has no room for — a Known IP+ from an Icom native
// file, a Yaesu shift from a CSV import, an offset magnitude this grid has no
// position for. Naming the field only when a value is actually present is
// what lets an ordinary write through while REFUSING that one, rather than
// dropping the value silently from a frame with nowhere to put it.
//
// THREE OF THE CONDITIONALS TEST A VALUE AND NOT A FieldState, because their
// codeplug.ChannelData members are plain scalars with no state to test:
// ClarHz/RxClar/TxClar (three Go members under one spec.Field), CTCSS and
// Shift. A caller who set one has requested it just as surely as a Known
// FieldState does, and the fleet's FieldState walk cannot see them at all.
var requestedFieldRules = []struct {
	field   spec.Field
	present func(codeplug.ChannelData) bool
}{
	{spec.FieldFrequency, always},
	{spec.FieldMode, always},
	{spec.FieldClarifier, func(d codeplug.ChannelData) bool { return d.ClarHz != 0 || d.RxClar || d.TxClar }},
	{spec.FieldCTCSSState, func(d codeplug.ChannelData) bool { return d.CTCSS != "" }},
	{spec.FieldCTCSSTone, func(d codeplug.ChannelData) bool { return d.CTCSSTone.State == codeplug.Known }},
	{spec.FieldShift, func(d codeplug.ChannelData) bool { return d.Shift != "" }},
	{spec.FieldTag, always},
	{spec.FieldTagDisplay, func(d codeplug.ChannelData) bool { return d.TagDisplay.State == codeplug.Known }},
	{spec.FieldScanSkip, always},
	// spec.FieldErase is DELIBERATELY ABSENT; see the doc comment.
	{spec.FieldTxFrequency, always},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldToneMode, always},
	{spec.FieldToneTx, always},
	{spec.FieldToneRx, always},
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

// always is the predicate of a field the record carries on every write.
func always(codeplug.ChannelData) bool { return true }

// requestedFields lists, in requestedFieldRules' order, the spec.Fields this
// channel's write requests.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := make([]spec.Field, 0, len(requestedFieldRules))
	for _, r := range requestedFieldRules {
		if r.present(data) {
			fields = append(fields, r.field)
		}
	}
	return fields
}

// WriteChannel implements driver.Session: ONE 57-byte MA0 Set, reported Sent
// and never Confirmed (plan P13, matrix §3.9), behind the eleven-rung refusal
// ladder of plan P7.
//
// TWO FRAMES GO ON THE WIRE PER CHANNEL WRITE AND ONE OF THEM MUTATES. The
// first is this driver's OWN MA0 read of the target slot — not a call to
// ReadChannel and not a cached answer: WriteChannel receives the candidate
// channel and nothing else, the clone service's fresh verify-read is not
// passed in, and a direct caller bypasses clone entirely. Two of the eleven
// rungs are decidable only from what that read answers.
//
// THE LADDER, IN P7's ORDER:
//
//	PART 1 — locally decidable, before any wire traffic at all:
//	 1 parseSlotID    the identifier's SYNTAX (read.go)
//	 2 bankFor        membership in THIS session's published banks, which is
//	                  also where slots 100-119 are refused (decision 12)
//	 3 the erase      an empty candidate: decision 15
//	 4 CheckFieldStates  the FLEET's FieldState walk
//	 5 THE CAPABILITY GATE  unverified-write consent not given
//	 6 decision 9     no Known TX disposition (M-E8: file-side only)
//	 7 the mode       a name this row does not publish
//	 8 decision 8     a Known tone_rx of 1750 Hz
//	 9 A2             a name over ten characters or outside TagByteOK
//	   the build      the record the Set would emit, with the mandatory-Known
//	                  refusals and core/kw/ma's own
//	THE ONE READ — MA0 Read of the target slot, held with the Set under opMu
//	PART 2 — read-dependent, and every clause quotes the answer:
//	   the answer   a flag and the window it describes disagree
//	10 A3            the answer satisfies the empty predicate
//	11 A8            P2 says Section defined, unconditionally
//	   A14           P2 says Dual and the candidate is simplex
//	   decision 9    P16 = 1, unconditionally
//	   decision 9    any of P10-P14 is not what the Set would emit
//	THEN THE WRITE — one MA0 Set, reported Sent, never Confirmed
//
// THE CAPABILITY GATE COMES BEFORE THE SEMANTIC RUNGS, which is the fleet's
// ruled order: on an unconsented RealHardware session the all-Unverified gate
// is the first answer, and the semantic refusals are what a Simulated — or a
// consented — session meets. This row registers writeTrialsComplete = false,
// so on a REAL TS-990S today NOTHING is writable and every channel is refused
// at that gate with every requested field named. That is the profile working,
// and the one route past it is the user's own recorded consent
// (WithConsentedUnverifiedWrites). Consent widens WHAT may be attempted,
// never HOW carefully: every rung below it still fires.
//
// THERE IS NO 890S-ONLY RUNG. The sibling book prints a rule that a split
// channel's transmit FM width must agree with its receive one; THIS book
// prints no such sentence anywhere in its MA0 parameter list (990:2891-2965),
// so this ladder has no rung for it — and what governs the secondary side
// here is rung 11's comparison against the radio's own answer, which is a
// wider rule than the sibling's and needs no help from a narrower one.
//
// THE FRAME IS BUILT BEFORE THE READ AND SENT AFTER PART 2, and the ordering
// is deliberate: rung 11 compares the radio's current secondary side against
// WHAT THE SET WOULD EMIT, so the record and the comparison must be one datum
// rather than two derivations of it. Building is not sending — no byte
// reaches the wire until every rung has passed, which is what P7's "no frame
// is built until every rung passes" is there to guarantee. THE STRONGER
// REASON IS THE STANDING ONE: BuildMA0Set's own refusals (an over-wide
// frequency against the printed eleven-digit field, 990:2905-2906, M-E6)
// are LOCALLY DECIDABLE from the candidate alone, so
// building last would send the pre-write MA0 read to the radio for a channel
// whose Set could never be built in the first place.
//
// NO READ-BACK VERIFICATION (P13). The verify-read after a write is
// core/clone's, exactly as on every other family; WriteChannel NEVER calls
// ReadChannel and a transcript pin says so.
//
// SENT, NEVER CONFIRMED, and the distinction is this family's most
// load-bearing assumption. That a "?;" is a REJECTION at all is this book's
// own error table (990:108-121); that an ACCEPTED Set draws nothing at all is
// A20/L-HW-3, and no TS-990S has ever been written to by this project.
// Silence is therefore inconclusive, and a driver reporting Confirmed on
// silence would be asserting A20 as a fact. A "?;" inside the bounded window
// is different:
// the frame provably went out and the radio provably refused it, so that
// outcome reports Sent true with the typed kw.RejectionError.
//
// THE OPERATION MUTEX IS HELD FOR THE WHOLE CALL (P12/P13), taken even before
// the refusal checks, since a refused write returns without wire traffic
// either way. It is NOT held across write-then-verify: that pair is
// core/clone's, as the driver seam assigns it.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// Every refusal below returns res unchanged: an EXPLICITLY EMPTY step
	// list, never nil. The clone service journals this result, and a nil
	// slice marshals as JSON null, which an auditor would have to read as
	// "unknown" rather than the truth, "no frame was ever built".
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	// RUNG 1 — the identifier's syntax, and nothing about membership.
	number, err := parseSlotID(ch.Slot)
	if err != nil {
		return res, &UnknownSlotError{Slot: ch.Slot, Reason: err.Error()}
	}
	// RUNG 2 — membership in THIS session's published banks. The same
	// branch, and the same message, that refuses a READ of an unpublished
	// slot: 100-119 are refused here exactly as "999" is, with no special
	// case and no invented radio behaviour (decision 12; see
	// UnknownSlotError, read.go).
	bank, ok := s.caps.BankOf(ch.Slot)
	if !ok {
		return res, &UnknownSlotError{
			Slot:   ch.Slot,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}

	// RUNG 3 — AN EMPTY CHANNEL IS AN ERASE REQUEST, AND IT IS REFUSED
	if ch.Empty() {
		// (decision 15). This radio prints a DEDICATED deletion command
		// where pair 1 had only an ambiguous MW side effect — MA5,
		// "Channel Deletion" (990:3042-3047) — so there is no ambiguity to
		// record and the standing rule declines it anyway: no MA5 is ever
		// built, core/kw/ma's outbound gate admits no frame that begins
		// one, and spec.FieldErase is nowhere write-Supported.
		//
		// The rung must stay AHEAD of the field checks below STRUCTURALLY
		// and not merely by preference: codeplug.Channel.Data is a pointer
		// (core/codeplug/channel.go:19, Empty() at :210) and every rung
		// below dereferences it, so without this one a caller handing an
		// empty channel — which core/clone does, for an emptied row in a
		// loaded file — would PANIC.
		return res, refuse(ch.Slot, registerDecision15, []spec.Field{spec.FieldErase},
			"this milestone builds no erase. This radio prints a dedicated deletion command, MA5 \"Channel Deletion\" (990:3042-3047), and this programme's standing no-erase rule declines it: no MA5 is ever built, the outbound gate admits no frame that begins one, and FieldErase is not write-Supported on this row's bank")
	}
	data := *ch.Data

	// RUNG 4 — P7, and it is THE FLEET'S walk rather than a table of this
	// package's own (driver.CheckFieldStates): every field of ChannelData
	// that carries a FieldState, judged against this session's own
	// vocabularies. What it prevents is silent rather than loud — a value
	// carried alongside a state meaning "preserve whatever the radio has"
	// is never named by requestedFields, so without this rung it would be
	// DROPPED from the frame and the write would report success.
	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return res, plainRefusal(ch.Slot, field, "%s: %v", field, err)
	}

	// RUNG 5 — THE CAPABILITY GATE. Every requested field must pass
	// spec.FieldSupport.CanWrite for THIS slot's bank in THIS session's
	// capabilities — spec.Supported, or spec.ConsentedUnverified, which is
	// the label every writable field of a consented real-hardware session
	// carries. spec.Inert is accepted as acceptable-to-TRANSMIT, which is
	// the fleet's stated stance; no field of this row is Inert, since Inert
	// is a HARDWARE finding and this radio has been asked nothing, so the
	// leg is here for the day one is rather than for anything this
	// milestone ships.
	var unwritable []spec.Field
	for _, f := range requestedFields(data) {
		fs := s.caps.FieldSupport(bank, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session (the 57-byte record cannot express the field on this row, or this session's capability profile does not support writing it)",
		}
	}

	// Rung 6 — decision 9's one surviving clause from pair 1's decision 11,
	// and M-E8 is why its reach is FILE-SIDE ONLY. The gate is whether the
	// BANK PUBLISHES FieldTxFrequency rather than an unconditional field
	// test, so the day this row publishes a bank that does not, the rung
	// stops asking for a disposition that bank has no field for.
	if !s.caps.FieldSupport(bank, spec.FieldTxFrequency).Unreachable() && data.TxFreqHz.State != codeplug.Known {
		return res, refuse(ch.Slot, registerDecision9, []spec.Field{spec.FieldTxFrequency},
			"this bank publishes tx_frequency and this channel's state for it is %q, not %q. One MA0 Set writes P9, frequency 2 at bytes 26-36, together with P15, the split flag at byte 44 (990:2927-2948), on every write with no \"leave it alone\" encoding, so a disposition this programme does not hold could only be GUESSED. On this row a channel this programme has itself read always has one — a single answer carries both frequencies and the flag — so a channel reaching here without it came from a loaded file or a CHIRP import (M-E8)",
			data.TxFreqHz.State, codeplug.Known)
	}

	// Rung 7 — the mode, which also resolves P4 and P5 for the build below.
	mode, narrow, ok := modeWire(s.layout, data.Mode)
	if !ok {
		return res, plainRefusal(ch.Slot, spec.FieldMode,
			"mode %q is not one the %s publishes: this row's legend is OM P2's, twenty-two live values with the four narrow twins (990:3706-3730), and values '0' and '8' are printed \"Unused\" (990:3707, 990:3715) — they name no mode a channel can be in, so no published name maps to either and no write can put one on the wire",
			data.Mode, modelName)
	}

	// Rung 8 — decision 8, the one index the published tone domain carries
	// that the printed CN chart does not. THE BOUND IS CONSULTED FROM THE
	// SAME PLACE AS ITS DATUM: ma.Layout.MaxCTCSSIndex is core/kw/ma's own
	// reading of CN's printed 00-49, which is also what the MA0 builder
	// enforces. The rung is here, ahead of the frame, so the refusal is
	// TYPED and names the decision rather than arriving as a parse error.
	if data.ToneRx.State == codeplug.Known {
		idx, ok := toneIndex(data.ToneRx.Value)
		if !ok || idx > int(s.layout.MaxCTCSSIndex()) {
			return res, refuse(ch.Slot, registerDecision8, []spec.Field{spec.FieldToneRx},
				"tone_rx %v is index %d in the 51-entry TN chart this row publishes (990:4960-4972), and the printed CN chart stops at %d (990:1251-1264). spec.Capabilities carries ONE tone domain and one AdmitsTone predicate for both directions (M-E1), so the domain publishes the value and the write path refuses it; a receive tone this radio cannot be told to listen for is not written",
				data.ToneRx.Value, idx, s.layout.MaxCTCSSIndex())
		}
	}

	// Rung 9 — the name, both arms.
	if err := s.checkName(ch.Slot, data.Tag); err != nil {
		return res, err
	}

	slot, err := s.layout.NewSlot(number)
	if err != nil {
		// Unreachable for a slot the bank published, since every published
		// identifier was rendered by this same layout's NewSlot (caps.go).
		// Refuse rather than build a frame from a slot the codec disowns.
		return res, &UnknownSlotError{Slot: ch.Slot, Reason: err.Error()}
	}
	set, err := s.setRecord(ch.Slot, slot, data, mode, narrow)
	if err != nil {
		return res, err
	}
	cmd, err := s.layout.BuildMA0Set(set)
	if err != nil {
		// TWO SENTINELS, DELIBERATELY (Go's multi-%w). This IS a write
		// refusal on the neutral seam, so errors.Is(err,
		// driver.ErrWriteRefused) must hold for it as for every other
		// pre-wire rung — AND the codec's own typed cause survives, because
		// the design names it: a frequency beyond the eleven-digit field is
		// the codec's refusal and its message names the FIELD WIDTH PRINTED
		// AT 990:2905-2906 rather than any radio's tuning range (M-E6).
		return res, fmt.Errorf("ts990: WriteChannel %s: %w: %w", ch.Slot, driver.ErrWriteRefused, err)
	}

	// THE ONE READ, and the two rungs below quote its answer.
	current, err := s.readCurrent(ctx, ch.Slot, slot)
	if err != nil {
		return res, err
	}
	if writeGapHook != nil {
		writeGapHook()
	}
	if err := readDependentRefusal(ch.Slot, current, set); err != nil {
		return res, err
	}

	// THE step list, declared in full HERE: after the frame provably exists
	// and every rung has passed, before it goes near the wire. ONE element,
	// because this radio's write choreography is one MUTATING frame — the
	// pre-write read is this method's own evidence-gathering and not a step
	// of the write.
	res.Steps = []driver.WriteStep{{Command: "MA0"}}
	const setStep = 0

	if _, err := s.eng.Do(ctx, cmd, ma0SetSpec()); err != nil {
		// A "?;" IS ATTRIBUTABLE and any other transport failure is not:
		// the radio explicitly refused a frame that provably went out, so
		// Sent is true there and false where the host cannot tell whether
		// the frame arrived. Confirmed stays false on every path, including
		// success — see this method's doc comment.
		res.Steps[setStep].Sent = errors.Is(err, transport.ErrRejected)
		return res, fmt.Errorf("ts990: WriteChannel %s: %w", ch.Slot, wireFailure("MA0", err))
	}
	res.Steps[setStep].Sent = true
	return res, nil
}

// readCurrent is the ONE pre-write MA0 read: the driver's own, of the target
// slot, sent under the same opMu hold as the Set that follows it.
//
// ITS FAILURES FAIL THE WRITE WHOLE, and both are typed by the same
// wireFailure the probe and the read path use, so one rule is applied once: a
// "?;" is a definitive rejection and never "absent", and silence is the typed
// kw.TimeoutError, which says in as many words that it is not an inference of
// absence. Neither may be read as "the channel is fine, carry on": rungs 10
// and 11 are decidable only from an answer.
func (s *Session) readCurrent(ctx context.Context, slotID string, slot ma.Slot) (ma.Record, error) {
	cmd, err := s.layout.BuildMA0Read(slot)
	if err != nil {
		return ma.Record{}, fmt.Errorf("ts990: WriteChannel %s: pre-write read: %w", slotID, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.ma0Spec(slot))
	if err != nil {
		return ma.Record{}, fmt.Errorf("ts990: WriteChannel %s: pre-write read: %w", slotID, wireFailure("MA0", err))
	}
	rec, err := s.layout.ParseMA0Answer(frame)
	if err != nil {
		return ma.Record{}, fmt.Errorf("ts990: WriteChannel %s: pre-write read: %w", slotID, err)
	}
	return rec, nil
}

// readDependentRefusal is Part 2: rung 10 and rung 11's four clauses, each
// quoting the answer the pre-write read returned.
//
// EACH CLAUSE IS PINNED SEPARATELY, which is plan P7's own instruction for
// the P16 one and the right shape for all four: a channel can satisfy one
// and not another, and a test that could pass on the wrong clause would not
// be pinning the rung it names.
//
// THE ORDER IS WIDEST FIRST. A channel's P2 is a statement about the whole
// record, dual reception is a statement about the second side's PURPOSE, and
// the P10-P14 comparison is about its contents; a reader meeting the
// narrowest message for the widest problem would be told the least useful
// true thing.
//
// THE TWO P2 CLAUSES REFUSE A CHANGE THIS WRITE WOULD MAKE, NOT THE CLASS
// ITSELF, and that narrowing is review s2-close-review-opus-1.md MED-1's:
// refusing every non-Single target outright made tx_frequency write-dead on
// this row, because the chart's own rule — the type is "decided while setting
// the P9 and P10 values" (990:2901-2903) — means EVERY split channel answers
// P2 = '1'. A split candidate onto such a target changes no class and is
// judged by the P10-P14 comparison below.
func readDependentRefusal(slotID string, current, set ma.Record) error {
	if err := selfContradictoryAnswer(slotID, current); err != nil {
		return err
	}
	if current.Empty {
		// Rung 10 — A3. The channel is unassigned NOW, established by this
		// driver's own read rather than by a cached pass.
		return refuse(slotID, registerA3, nil,
			"channel %s reads back UNASSIGNED: its P2-P18 window is blank, which is this book's own blank-channel answer (990:2962-2963). Whether an MA0 Set ALONE can create a channel is nowhere printed, and every other member of this family says it cannot — \"Setting an unassigned channel causes an error\" on MA2 (990:3008) and MA3 (990:3023), \"You cannot set an unassigned channel\" on MA6 (990:3058), and an unassigned original \"cannot be copied\" on MA4 (990:3037-3038). This programme does not create channels; hardware item 1 is what lifts it",
			slotID)
	}
	// Rung 11, first clause — A8, and it is UNCONDITIONAL. A Section-defined
	// channel is the one class whose frequency-2 side this book never
	// explains: the section's end frequency is MA6's own command, and
	// whether P9 carries it here is unprinted. A whole-record Set therefore
	// cannot know what it would overwrite, whatever the candidate holds.
	if current.Class == '2' {
		return refuse(slotID, registerA8, nil,
			"channel %s reads back with P2 = '2', \"2: Section defined Memory channel\" (990:2897-2903). What such a channel's frequency-2 side holds is nowhere printed — the section's end frequency is MA6's own command (990:3051-3059) — so one MA0 Set, which rewrites the whole record, would overwrite bytes this programme cannot read as anything. That is refused rather than written blind",
			slotID)
	}
	// Rung 11, second clause — A14, and it is SCOPED TO A SIMPLEX CANDIDATE.
	// The Class byte is a PARSER output the codec normalises to '0' on every
	// build, and the chart says the type is "decided while setting the P9
	// and P10 values" — so the Set's own P2 changes nothing and what re-types
	// a channel is the frequency-2 side it carries. A SPLIT candidate leaves
	// a Dual target Dual and is judged below, by the P10-P14 comparison that
	// exists for exactly that question; a SIMPLEX one writes the printed
	// zeroed side and re-types it, which is the change this clause refuses.
	// TestWriteChannel_ASplitChannelRoundTripsWhenTheSetReproducesItsSecondarySide
	// pins the fall-through and the ladder's own "P2 = '1'" row pins this.
	if current.Class != '0' && set.TXMode == 0 {
		return refuse(slotID, registerA14, nil,
			"channel %s reads back with P2 = %q, and this book prints three channel types — \"0: Single / 1: Dual / 2: Section defined\" (990:2897-2903). The type is \"decided while setting the P9 and P10 values\" (990:2901-2903), and this write's candidate is SIMPLEX: its frequency-2 side is the printed zeroed form (990:2964-2965, A16), so the Set would re-type the channel as a Single one. Its own P2 cannot say otherwise — the book calls that parameter \"ignored. Enter a dummy value\" and names no value (A14, the milestone's only defaulted byte). That is refused rather than done silently",
			slotID, current.Class)
	}
	// Rung 11, second clause — decision 9, and it is UNCONDITIONAL on P16.
	// P16 names no field in the neutral model at all, which is why the
	// refusal is decidable only here and could never have been a Part 1
	// rung.
	if current.DualRecv {
		return refuse(slotID, registerDecision9, nil,
			"channel %s reads back with P16 = 1, dual reception (990:2949-2951). Its second side is LIVE CONTENT with no home in the neutral model — no field of codeplug.ChannelData names dual reception — and one MA0 Set rewrites the whole record, so this write would zero it. The refusal is unconditional and does not consult P15: what the split flag says about frequency 2 is a separate statement (990:2946-2948), and neither reading makes the second receiver's frequency something this programme holds",
			slotID)
	}
	// Rung 11, third clause — decision 9 over P10-P14, naming every
	// P-number that differs AND BOTH VALUES. The comparison is against what
	// THIS Set would emit rather than against a rule about the candidate:
	// the Set has a source for the primary side and none for the secondary,
	// so what it would put there is a copy of the primary or the printed
	// zeroed form, and either is a change the radio's own answer can prove.
	if diffs := secondaryDiffs(current, set); len(diffs) > 0 {
		return refuse(slotID, registerDecision9, nil,
			"channel %s's frequency-2 side is not what this write would emit, and one MA0 Set rewrites the whole record: %s (990:2929-2945). codeplug.ChannelData has ONE mode, ONE FM width and ONE tone tuple, so these bytes have no source in the channel and the write would replace them with the primary side's own. Such a channel is READABLE but not rewritable until the neutral model grows a second tuple",
			slotID, strings.Join(diffs, "; "))
	}
	return nil
}

// selfContradictoryAnswer refuses an answer whose FLAG and the window it
// describes disagree, before any other read-dependent rung reads either.
//
// THE NEUTRAL MODEL CARRIES NO P15 OF ITS OWN, and that is the whole reason
// this rung exists (T12's review of the sibling row found the same gap
// there). A read publishes the second frequency as tx_frequency and drops the
// flag; a write RE-DERIVES the flag from tx_frequency. So on a frame where
// the two disagree, the Set this driver builds would carry a P15 THE RADIO
// DID NOT SEND — the one thing a one-frame rewrite must never do — and no
// rung below would notice, because rung 11 compares P10-P14 and the flag is
// not among them.
//
// IT RESTS ON PRINTED AUTHORITY AND SO TAKES THE PLAIN FLEET ERROR. P15 is
// "0: Simplex / 1: Split" (990:2946-2951) and a single memory channel's whole
// frequency-2 side is zero (990:2964-2965, A16): a split channel therefore
// HAS a frequency 2 and a simplex one does not, and a frame claiming both at
// once is outside what this book describes. Nothing is assumed, so no
// register entry is named.
//
// THE P15 ARM IS SCOPED TO A NON-DUAL RECORD, and the exception is
// DOCUMENTED rather than defensive: a DUAL-RECEPTION channel legitimately
// carries a live frequency 2 with P15 = 0, because that second side is a
// sub-band RECEIVE frequency and not a transmit one (990:2949-2951, and
// read.go's channelData says the same in the other direction). Calling that
// state a contradiction would tell a user their radio answered nonsense when
// it answered exactly what this book prints — and the write is refused one
// clause below either way, by decision 9's unconditional P16 rung, whose
// message describes it correctly.
//
// THE P16 ARM IS THE OTHER DIRECTION AND HAS NO SUCH EXCEPTION: a
// dual-reception flag over a frequency-2 side that is entirely zero describes
// a second receiver with no frequency, and it is a record core/kw/ma's own
// builder refuses to produce (the zeroed side and a set flag contradict each
// other, 990:2964-2965). Such an answer cannot be written back at all.
//
// core/kw/ma DECODES THE FLAGS INDEPENDENTLY OF THE WINDOW and refuses the
// contradiction only on BUILD, which is correct for a codec — a parser that
// refused it would make a channel unreadable, and clone.ReadAll abandons a
// whole radio's read on the first channel error. Catching it HERE, on the
// write path alone, is what keeps the read loud-free and the write honest.
func selfContradictoryAnswer(slotID string, current ma.Record) error {
	if current.DualRecv && current.TXFreqHz == 0 {
		return &driver.WriteRefusedError{
			Slot:   slotID,
			Reason: fmt.Sprintf("channel %s answers with P16 = 1, dual reception ON (990:2949-2951), and a frequency-2 side that is entirely zero — a second receiver with no frequency for the record to hold. A single memory channel's whole frequency-2 side is zero (990:2964-2965), and a set flag over that side is a pairing this family's own encoder refuses to build, so this answer cannot be written back in any form. It is refused rather than rebuilt with a flag or a side the radio did not send", slotID),
		}
	}
	if !current.DualRecv && current.Split != (current.TXFreqHz != 0) {
		return &driver.WriteRefusedError{
			Slot:   slotID,
			Reason: fmt.Sprintf("channel %s answers with P15 = %s and a frequency 2 of %d Hz, and the two contradict each other: the book prints P15 as \"0: Simplex / 1: Split\" (990:2946-2951) and says a single memory channel's whole frequency-2 side is zero (990:2964-2965), so a split channel has a frequency 2 and a simplex one does not. The neutral model carries no split flag of its own — a read publishes the second frequency as tx_frequency and a write re-derives P15 from it — so writing this channel back would put a P15 on the wire that the radio did not send. It is refused rather than flipped", slotID, flagText(current.Split), current.TXFreqHz),
		}
	}
	return nil
}

// secondaryDiffs reports, by P-number, every frequency-2 field whose CURRENT
// value differs from what set would emit for it.
//
// P9 AND P15 ARE DELIBERATELY ABSENT from the comparison, and their absence
// is the pair's §2.4 gain rather than an omission: frequency 2 and the split
// flag DO have a home in the neutral model — TxFreqHz — so a difference there
// is an edit the user asked for, not a value with nowhere to go. P10 to P14
// are the five with no home at all.
//
// THE GAIN IS REACHABLE, and rung 11's own A14 clause is what makes it so:
// a split target answers P2 = '1' by the chart's own rule, so a clause
// refusing every non-Single class would leave this comparison judging
// nothing. What the write can carry is an edit BETWEEN split shapes — a
// changed frequency 2 on a channel whose P10-P14 the Set reproduces.
// CLEARING TxFreqHz on such a target never reaches this comparison: a
// simplex candidate over a Dual target is refused one clause above, because
// zeroing the frequency-2 side is what re-types the channel.
//
// THE CONSEQUENCE IS WIDER THAN A SIMPLEX/SPLIT CONVERSION: a split channel's
// secondary side has no source in codeplug.ChannelData at all, so an ORDINARY
// edit to the PRIMARY side of a split channel whose secondary MIRRORS it —
// Mode, say — is refused here too, because the Set the write would build
// carries the edited primary's mode on P10 while the radio's own answer still
// carries the pre-edit one. The channel is readable and its secondary side is
// unchanged; it is the write that has no way to say so.
func secondaryDiffs(current, set ma.Record) []string {
	var diffs []string
	for _, c := range []struct {
		p              string
		current, would string
	}{
		{"P10, the mode for frequency 2", modeByteText(current.TXMode), modeByteText(set.TXMode)},
		{"P11, FM wide/narrow for frequency 2", flagText(current.TXFMNarrow), flagText(set.TXFMNarrow)},
		{"P12, the tone function for frequency 2", modeByteText(current.TXToneType), modeByteText(set.TXToneType)},
		{"P13, the tone frequency for frequency 2", indexText(current.TXToneIndex), indexText(set.TXToneIndex)},
		{"P14, the CTCSS frequency for frequency 2", indexText(current.TXCTCSSIndex), indexText(set.TXCTCSSIndex)},
	} {
		if c.current != c.would {
			diffs = append(diffs, fmt.Sprintf("%s is %s and this write would emit %s", c.p, c.current, c.would))
		}
	}
	return diffs
}

// modeByteText spells a record's secondary mode or tone-function byte as the
// WIRE byte a frame would carry, which is what makes a refusal quotable: the
// zero value on an ma.Record means "the printed zeroed secondary side"
// (990:2964-2965, A16) and goes on the wire as '0'.
func modeByteText(b byte) string {
	if b == 0 {
		b = '0'
	}
	return fmt.Sprintf("%q", b)
}

// flagText spells a two-valued secondary byte as the '0'/'1' the book prints.
func flagText(on bool) string {
	if on {
		return "'1'"
	}
	return "'0'"
}

// indexText spells a secondary tone index as the two digits the frame
// carries.
func indexText(i int) string { return fmt.Sprintf("%02d", i) }

// checkName is rung 9, and it is TWO refusals with two shapes because the two
// facts have two evidence grades.
//
// THE WIDTH IS PRINTED — "Channel Name (Up to 10 digits.)" over a fixed
// ten-byte window at bytes 47-56 (990:2955-2956) — so a wider name earns the
// plain fleet refusal with no register entry on it. THE CHARSET IS ASSUMED:
// this row publishes the empty TagCharset, which selects the family default
// (printable ASCII 0x20-0x7E excluding ';'), and A2 is the register home for
// the claim that the default IS this radio's set. The ';' half of that
// exclusion is FORCED rather than assumed — the terminator's position
// "differs depending on the command used" (990:96-99), so a name carrying one
// would split the frame at the radio's own parser — and the message says so,
// because a user who met "A2" for a semicolon would be told an assumption is
// in doubt when the fact is printed.
//
// THE BOUND IS ASKED OF THE CAPABILITY SET, not of a literal here:
// caps.TagLen and caps.TagByteOK are the same two the CSV importer and
// core/codeplug apply, so a name this rung admits is one every layer above
// admits too.
func (s *Session) checkName(slotID, tag string) error {
	if len(tag) > s.caps.TagLen {
		return plainRefusal(slotID, spec.FieldTag,
			"the name %q is %d characters and this row's window holds %d: P18 is a FIXED ten-byte window at bytes 47-56 with the terminator nailed to 57 (990:2955-2956, 990:2938). A wider name is refused rather than truncated",
			tag, len(tag), s.caps.TagLen)
	}
	for i := 0; i < len(tag); i++ {
		if !s.caps.TagByteOK(tag[i]) {
			return refuse(slotID, registerA2, []spec.Field{spec.FieldTag},
				"the name %q carries %#02x at position %d, outside the byte set this row publishes for a tag — the family default, printable ASCII 0x20-0x7E excluding ';', which A2 is the register home for. The ';' exclusion is FORCED rather than assumed: the terminator's position \"differs depending on the command used\" (990:96-99), so a name carrying one would split the frame at the radio's own parser",
				tag, tag[i], i+1)
		}
	}
	return nil
}

// modeWire resolves a published mode NAME to the two bytes that produce it:
// P4, the OM P2 legend value, and P5, the FM wide/narrow flag.
//
// IT IS THE INVERSE OF caps.go's modeDisplayName AND IS DERIVED FROM IT, so
// the byte a write emits and the name a read publishes are the same pairing
// the capability list advertises. A hand-written table here would be one edit
// from disagreeing with that function, which is the drift core/driver/ts590
// needs a test for.
//
// THE SEARCH IS THE LEGEND'S OWN ALPHABET, walked once per write. modeNames
// walks the same 0x00-0xFF span for the same reason: the legend is a map the
// layout owns, and asking it for every byte is what keeps this package from
// holding a second copy of its keys.
func modeWire(l ma.Layout, name string) (mode byte, narrow, ok bool) {
	for b := 0; b <= 0xFF; b++ {
		for _, n := range []bool{false, true} {
			if got, has := modeDisplayName(l, byte(b), n); has && got == name {
				return byte(b), n, true
			}
		}
	}
	return 0, false, false
}

// toneIndex is the position of tone in the 51-entry chart this row publishes,
// which IS the CAT tone number: spec.Validate requires CTCSSTones to be
// strictly ascending precisely so the slice index doubles as the wire value
// (caps.go's ctcssTones).
func toneIndex(tone spec.Tone) (int, bool) {
	for i, t := range ctcssTones {
		if t == tone {
			return i, true
		}
	}
	return 0, false
}

// setRecord maps a populated channel onto the ma.Record the MA0 Set would
// emit. It is called after every locally decidable rung above has passed, and
// its own refusals are locally decidable too.
//
// THE MANDATORY-KNOWN REFUSALS COME FIRST, and their position is load-bearing
// rather than stylistic: each names a byte that is LIVE on this row and
// transmitted on every write with no "leave it alone" encoding, so a
// non-Known value there cannot be omitted from the frame — it can only be
// MANUFACTURED, which is what codeplug's write rule forbids for a field whose
// state says "preserve whatever the radio has".
//
// THE SECONDARY SIDE IS DECIDED BY THE TX FREQUENCY ALONE, and the rule is
// the read path's own, run backwards: read.go publishes Known 0 for a channel
// whose record states no independent transmit frequency, because that is what
// the printed zeroed form means (990:2964-2965, A16). So a Known 0 here emits
// that same zeroed form with P15 = 0, and ANY other Known value emits
// frequency 2 with P15 = 1. What the Set then puts in P10-P14 is a COPY OF
// THE PRIMARY SIDE, because the neutral model holds no second mode, width or
// tone tuple to take them from — which is exactly why rung 11 compares those
// five bytes against the radio's own answer before any of this reaches the
// wire.
//
// P2 IS NOT SET HERE, and its absence is the design: ma.Record.Class is a
// PARSER output, and core/kw/ma emits '0' on every build because the book
// says the parameter "is ignored. Enter a dummy value" and names no value
// (A14, lift L-HW-11). It is the MILESTONE'S ONLY DEFAULTED BYTE, and
// decision 7 is why that list is one item long and not sixteen: every other
// byte of this 57-byte grid carries a live P-number with a printed domain, so
// there is no printed-fixed class here for pair 1's defaulted-byte register
// to fill. What the dummy class cannot do is PRESERVE a type, which is why
// rung 11 refuses a simplex candidate over a target whose current P2 is not
// '0': the radio re-derives the type from the P9/P10 this Set does carry.
//
// EVERYTHING ELSE IS THE CODEC'S. core/kw/ma re-validates the mode byte
// against this row's legend, both tone indices against their printed charts,
// the tone functions, the eleven-digit frequency fields, this row's own
// lockout encoding (E8) and the name's width and charset, and it refuses a
// record whose secondary side and split flag contradict each other.
func (s *Session) setRecord(slotID string, slot ma.Slot, data codeplug.ChannelData, mode byte, narrow bool) (ma.Record, error) {
	for _, m := range []struct {
		field    spec.Field
		state    codeplug.FieldState
		position string
	}{
		{spec.FieldToneMode, data.ToneMode.State, "P6 at byte 21, the FM tone function (990:2915-2919)"},
		{spec.FieldToneTx, data.ToneTx.State, "P7 at bytes 22-23, the TN index (990:2920-2922)"},
		{spec.FieldToneRx, data.ToneRx.State, "P8 at bytes 24-25, the CN index (990:2924-2926)"},
		{spec.FieldScanSkip, data.ScanSkip.State, "P17 at byte 46, the scan lockout (990:2952-2954)"},
	} {
		if m.state != codeplug.Known {
			return ma.Record{}, plainRefusal(slotID, m.field,
				"%s FieldState is %q, not %q, and %s is transmitted on every write with no \"leave it alone\" encoding — so a non-Known value here could only be manufactured, which is what a state meaning \"preserve whatever the radio has\" forbids",
				m.field, m.state, codeplug.Known, m.position)
		}
	}

	toneType, ok := toneModeWire[data.ToneMode.Value]
	if !ok {
		// Unreachable after CheckFieldStates, which judges a Known tone
		// mode against this session's ToneModes vocabulary — the same four
		// values read.go's forward map carries.
		return ma.Record{}, plainRefusal(slotID, spec.FieldToneMode, "tone mode %q is not one this row publishes", data.ToneMode.Value)
	}
	toneTx, ok := toneIndex(data.ToneTx.Value)
	if !ok {
		return ma.Record{}, plainRefusal(slotID, spec.FieldToneTx, "tone_tx %v is not in the 51-entry chart this row publishes (990:4960-4972)", data.ToneTx.Value)
	}
	toneRx, ok := toneIndex(data.ToneRx.Value)
	if !ok {
		return ma.Record{}, plainRefusal(slotID, spec.FieldToneRx, "tone_rx %v is not in the 51-entry chart this row publishes (990:4960-4972)", data.ToneRx.Value)
	}

	rec := ma.Record{
		Slot: slot,
		// P3, eleven digits at bytes 8-18 (990:2905-2906). The codec
		// refuses anything wider, and its message names the PRINTED FIELD
		// WIDTH rather than a tuning range — the only frequency bound this
		// milestone ships, MinFreqHz/MaxFreqHz being 0/0 (M-E6).
		FreqHz: data.FreqHz,
		// P4 with P5: modeWire resolved both from the one published name.
		Mode:     mode,
		FMNarrow: narrow,
		// P6, P7 and P8. The two indices are INDEPENDENT transmit and
		// receive values, which is what makes "3: Cross Tone" expressible
		// at all (990:2915-2919).
		ToneType:   toneType,
		ToneIndex:  toneTx,
		CTCSSIndex: toneRx,
		// P17, spelt this radio's own way by the codec — '1' OFF, '2' ON
		// (E8, 990:2952-2954) — from the neutral boolean.
		Lockout: data.ScanSkip.Value,
		// P18, passed as the LOGICAL value: the builder pads it to the
		// fixed ten-byte window with spaces (A1) and refuses a name that
		// would not fit or that carries a byte outside A2's charset, both
		// of which rung 9 has already refused with a message of its own.
		Name: data.Tag,
	}
	if tx := data.TxFreqHz.Value; tx != 0 {
		rec.Split = true
		rec.TXFreqHz = tx
		rec.TXMode = mode
		rec.TXFMNarrow = narrow
		rec.TXToneType = toneType
		rec.TXToneIndex = toneTx
		rec.TXCTCSSIndex = toneRx
	}
	return rec, nil
}

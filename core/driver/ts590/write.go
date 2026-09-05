// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The ASSUMED-register entries and design decisions this file's SEMANTIC
// refusals name. They are cited BY NAME rather than by position, and the
// authoritative register is core/kw/doc.go's — no entry is re-registered
// here.
//
// A REGISTER ENTRY IS PART OF THE REFUSAL, NOT A COMMENT ON IT. Every rung of
// this ladder answers "refused", and on an unconsented RealHardware session
// the CAPABILITY gate answers first for every write (writeTrialsComplete is
// false on both rows), so a caller — or a test — that could see only the fact
// of refusal could not tell an unlifted assumption from a missing consent.
// RefusalError.Register is what distinguishes them, and each rung's own pin
// asserts it (refusals_test.go's assertRegister).
const (
	// registerA9 is decision 11's: an MR with P1=1 on a SIMPLEX channel is
	// unprinted on both radios, so a fresh read never learns a channel's TX
	// side and a write without a Known one would flatten a split it never
	// saw. LIFT: L-HW-1, per registry row.
	registerA9 = "A9"
	// registerA13 is the S row's firmware pair: A13 (the FV answer's M.NN
	// grammar, assumed from one worked example) and A14 (byte 28 is
	// printed-fixed '0' only on an S at 1.xx). They are ONE rung because
	// one comparison consults both. LIFTS: L-HW-5 and L-HW-5a.
	registerA13 = "A13/A14"
	// registerA23 is the P14 mode gate: no TS-590 P14 value is known to be
	// meaningful outside FM. LIFT: L-HW-17, per (row, mode) CELL.
	registerA23 = "A23"
	// registerDecision14 is the tone-domain asymmetry: the published TN
	// chart carries 1750 Hz and the printed CN chart does not, and
	// spec.Capabilities has no per-direction tone axis to say so (M-E1).
	registerDecision14 = "decision 14"
)

// fmNormalWire is P14's printed normal value, "00: FM Normal"
// (590:1569-1571), and caps.go's fmNarrowWire is its narrow twin. Neither is
// asked of core/kw, whose own pair is unexported; what IS asked of the layout
// is which NAME each of the two produces (see fmP14), so the byte and the
// published mode name cannot drift apart even though the byte is restated.
const fmNormalWire = "00"

// filterLiveMajor is the firmware major version at and above which a
// TS-590S's byte 28 may be LIVE.
//
// THE DOCUMENT GUARANTEES THE ZERO FOR 1.xx AND SAYS NOTHING ABOUT 2.xx — "In
// firmware version 1.xx of TS-590S, always 0" (590:1478), "This is always set
// to 0 in the firmware version 1.xx of TS-590S" (590:1564, the differing
// wording being erratum E7) — so the comparison is "is this still the
// firmware the guarantee covers?", and everything else takes the conservative
// branch. A14.
const filterLiveMajor = 2

// RefusalError is a write refusal that names the ASSUMED-register entry or
// design decision it comes from.
//
// IT IS A *driver.WriteRefusedError AND MORE, never instead: Unwrap returns
// the embedded fleet error, so errors.Is(err, driver.ErrWriteRefused) holds
// through it, errors.As recovers the fleet type for a caller that wants the
// slot and the fields, and errors.As recovers THIS type for a caller — or a
// test — that needs to know WHICH rung answered. The message is the fleet's
// own sentence, with the register entry named inside Reason so a user reading
// a log sees it too.
//
// WHY THE DISTINCTION MATTERS ENOUGH TO MINT A TYPE (plan P7's H2): the
// capability gate refuses every write on an unconsented RealHardware session
// while writeTrialsComplete is false, and returns the PLAIN fleet error. A
// semantic pin that asserted only "refused" would therefore pass on the
// capability gate and the rung it claimed to pin need not exist at all. This
// type is what makes that impossible to write by accident.
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

// mwSetSpec is the transport spec for the ONE MW Set this driver sends:
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
// TestMWSetSpec_IsFireAndForgetAndNeverRetries pins all three properties.
func mwSetSpec() transport.CommandSpec {
	return transport.CommandSpec{Class: transport.ClassWrite}
}

// toneModeWire is read.go's toneModeNames read backwards: the neutral
// vocabulary this row publishes to the record's P7 byte.
//
// DERIVED RATHER THAN TRANSCRIBED. A hand-written inverse of a hand-written
// map is one edit from disagreeing — the FT-891 needs a test for exactly that
// (TestNameMaps_AreExactInverses) — and inverting the forward map at package
// initialisation makes the disagreement unrepresentable instead.
var toneModeWire = invertToneModes()

func invertToneModes() map[string]kw.ToneMode {
	m := make(map[string]kw.ToneMode, len(toneModeNames))
	for wire, name := range toneModeNames {
		m[name] = wire
	}
	return m
}

// filterWire is read.go's filterLabels read backwards, on toneModeWire's
// terms. Consulted only on the SG row — see buildRecord.
var filterWire = invertFilters()

func invertFilters() map[string]byte {
	m := make(map[string]byte, len(filterLabels))
	for wire, label := range filterLabels {
		m[label] = wire
	}
	return m
}

// requestedFieldRules pairs each spec.Field with the predicate that reports
// whether a write of this channel actually REQUESTS it.
//
// TWENTY-SIX ENTRIES — every spec.Field but spec.FieldErase, in the same
// order this package's own literal list carries them (caps_test.go's
// allSpecFields; plan P5 forbids a Kenwood file naming spec.AllFields).
// FieldErase is not a field a write requests: it is the whole shape of a
// DIFFERENT frame, the short MW of 590:1579-1581 this milestone never builds,
// and WriteChannel refuses an empty channel a rung above this table.
//
// EIGHT ARE UNCONDITIONAL, AND THAT IS THE FRAME'S OWN SHAPE. The 50-byte
// record carries a frequency, a mode, a data-mode flag, a tone mode, two tone
// indices, a lockout flag and a name on EVERY write, changed or not, with no
// "leave it alone" encoding anywhere in the grid (590:1539-1577). A write
// therefore requests all eight whether the caller edited them or not, which
// is what makes the capability gate below a real gate.
//
// THE EIGHTEEN CONDITIONALS EXIST FOR ONE CASE: a caller who hands this
// driver a value the record has no room for — a Known IP+ from an Icom native
// file, a Yaesu shift from a CSV import, a filter on the row that does not
// publish one. Naming the field only when a value is actually present is what
// lets an ordinary write through while REFUSING that one, rather than
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
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldToneMode, always},
	{spec.FieldToneTx, always},
	{spec.FieldToneRx, always},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
	{spec.FieldFilter, func(d codeplug.ChannelData) bool { return d.Filter.State == codeplug.Known }},
	{spec.FieldDataMode, always},
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
// channel's write requests. TestRequestedFields_MembershipAndOrder and its
// two neighbours pin the table's membership, its order and the reachability
// of every conditional.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := make([]spec.Field, 0, len(requestedFieldRules))
	for _, r := range requestedFieldRules {
		if r.present(data) {
			fields = append(fields, r.field)
		}
	}
	return fields
}

// WriteChannel implements driver.Session: ONE 50-byte MW Set, reported Sent
// and never Confirmed (plan P14, matrix §3.9), behind the refusal ladder of
// plan P7.
//
// THE LADDER, IN P7's ORDER, EVERY RUNG PRE-WIRE:
//
//	parseSlotID     the identifier's SYNTAX, and no row (read.go)
//	bankFor         membership in THIS session's published banks — which is
//	                also where the TS-590SG's unpublished extension channels
//	                110-119 are refused (Stuart decisions row 6)
//	the erase       an empty channel, refused: decision 8
//	CheckFieldStates the fleet's FieldState walk, P7's Valid() rung
//	THE CAPABILITY GATE  defence in depth below the clone service
//	A13/A14         a TS-590S whose FV is >= 2.00 or unparseable
//	A23             a non-FM write
//	A9              a non-empty channel whose bank PUBLISHES FieldTxFrequency
//	                and whose TxFreqHz is not Known (decision 11)
//	decision 14     a Known tone_rx of 1750 Hz
//	build           the frame-shaped refusals, core/kw's own
//
// THE CAPABILITY GATE COMES BEFORE THE SEMANTIC RUNGS, which is the FT-891
// plan's P7 and the FT-710's shipped order: on an unconsented RealHardware
// session the all-Unverified gate is the first answer, and the semantic
// refusals are what a Simulated — or a consented — session meets. Both rows
// register writeTrialsComplete = false, so on a REAL TS-590 today NOTHING is
// writable and every channel is refused at that gate with every requested
// field named. That is the profile working, not a limitation of this method,
// and the one route past it is the user's own recorded consent
// (WithConsentedUnverifiedWrites). Consent widens WHAT may be attempted,
// never HOW carefully: every rung below it still fires.
//
// ONE MW, AND NO READ-BACK (P14). The choreography the spec describes — one
// MW followed by an MR of the same channel — is the one core/clone already
// performs (core/clone/execute.go's write-then-verify pair), so performing it
// here too would double every write. WriteChannel NEVER calls ReadChannel.
//
// SENT, NEVER CONFIRMED, and the distinction is this family's most
// load-bearing assumption. That a "?;" is a REJECTION at all is the books'
// own error table (590:100-105); that an ACCEPTED Set draws nothing at all is
// A6, and no Kenwood radio has ever been written to by this project.
// Silence is therefore inconclusive, and a driver reporting Confirmed on
// silence would be asserting A6 as a fact. A "?;" inside the bounded window
// is different: the frame provably went out and the radio provably refused
// it, so that outcome reports Sent true with the typed kw.RejectionError.
//
// THE OPERATION MUTEX IS HELD FOR THE WHOLE CALL (P13/P14, matrix M-E2), and
// is taken even before the refusal checks, since a refused write returns
// without wire traffic either way. It is NOT held across write-then-verify:
// that pair is core/clone's, as the driver seam assigns it.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// Every refusal below returns res unchanged: an EXPLICITLY EMPTY step
	// list, never nil. The clone service journals this result, and a nil
	// slice marshals as JSON null, which an auditor would have to read as
	// "unknown" rather than the truth, "no frame was ever built".
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	number, half, err := parseSlotID(ch.Slot)
	if err != nil {
		return res, &UnknownSlotError{Slot: ch.Slot, Model: modelNameFor(s.row), Reason: err.Error()}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		// The same branch, and the same message, that refuses a READ of an
		// unpublished slot: the TS-590SG's 110-119 are refused here exactly
		// as "999" is on both rows, with no special case and no invented
		// radio behaviour (see UnknownSlotError, read.go).
		return res, &UnknownSlotError{
			Slot:   ch.Slot,
			Model:  modelNameFor(s.row),
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}

	if ch.Empty() {
		// AN EMPTY CHANNEL IS AN ERASE REQUEST, AND IT IS REFUSED
		// (decision 8, §2.8). The only clear either book prints is a side
		// effect of a short MW — "If you do not specify one digit in P16
		// and execute all the parameters from P4 to P15 set to 0, the
		// channels specified by P2 and P3 will be erased" (590:1579-1581) —
		// whose LENGTH is a reading rather than a printed number (A5,
		// erratum E19). This milestone never builds it, core/kw's gate
		// refuses any MW that is not exactly 50 bytes, and spec.FieldErase
		// is nowhere write-Supported.
		//
		// The rung must also stay AHEAD of the field checks below
		// STRUCTURALLY and not merely by preference: an empty channel has
		// no Data at all, and those checks dereference it.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this milestone builds no erase: the only clear either book prints is a side effect of a short MW whose length is a reading rather than a printed number (590:1579-1581, A5), core/kw admits an MW of exactly 50 bytes and no other, and FieldErase is not write-Supported on any bank of either row",
		}
	}
	data := *ch.Data

	// P7's Valid() rung, and it is THE FLEET'S walk rather than a table of
	// this package's own (core/driver.CheckFieldStates): every field of
	// ChannelData that carries a FieldState, judged against this session's
	// own vocabularies. What it prevents is silent rather than loud — a
	// value carried alongside a state meaning "preserve whatever the radio
	// has" is never named by requestedFields, so without this rung it would
	// be DROPPED from the frame and the write would report success (the
	// FT-891 closing review's C-M1, and MEDIUM-1's sharpening of it to
	// codeplug.Absent).
	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	// THE CAPABILITY GATE. Every requested field must pass
	// spec.FieldSupport.CanWrite for THIS slot's bank in THIS session's
	// capabilities — spec.Supported, or spec.ConsentedUnverified, which is
	// the label every writable field of a consented real-hardware session
	// carries. spec.Inert is accepted as acceptable-to-TRANSMIT, which is
	// the fleet's stated stance (core/spec/support.go's Inert); no Kenwood
	// field is Inert today, since Inert is a HARDWARE finding and no Kenwood
	// radio has been asked anything, so the leg is here for the day one is
	// rather than for anything this milestone ships.
	var unwritable []spec.Field
	for _, f := range requestedFields(data) {
		fs := s.caps.FieldSupport(bank.ID, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session (the 50-byte record cannot express the field on this row, or this session's capability profile does not support writing it)",
		}
	}

	// A13/A14 — the S row's write path alone (matrix §4, divergence 2).
	if reason, blocked := s.firmwareBlocksWrites(); blocked {
		return res, refuse(ch.Slot, registerA13, nil, "%s", reason)
	}

	// A23 — the mode gate, which also resolves P14 for the build below.
	p14, err := s.fmP14(ch.Slot, data.Mode)
	if err != nil {
		return res, err
	}

	// A9 / decision 11 — and the gate is whether the BANK PUBLISHES
	// FieldTxFrequency, never an unconditional field test (plan P12,
	// matrix M-E2). In the SCAN bank P1 selects a section channel's START or
	// END frequency (590:1449-1451) rather than a transmit frequency, so the
	// field is the zero FieldSupport there and there is no TX disposition to
	// require; requiring one would refuse every scan-edge write forever
	// under a name that describes something else.
	if !s.caps.FieldSupport(bank.ID, spec.FieldTxFrequency).Unreachable() {
		if data.TxFreqHz.State != codeplug.Known {
			return res, refuse(ch.Slot, registerA9, []spec.Field{spec.FieldTxFrequency},
				"this bank publishes tx_frequency and this channel's state for it is %q, not %q. What an MR with P1=1 answers on a SIMPLEX channel is unprinted on both radios, so a fresh read never learns a channel's TX side — and an MW with P1=0 makes the channel simplex \"even if it was already a split channel\" (590:1521-1523). Writing one anyway would flatten a split this programme never saw, read it back Unavailable and report the write verified; the lift is per registry row",
				data.TxFreqHz.State, codeplug.Known)
		}
		if data.TxFreqHz.Value != data.FreqHz {
			// NOT A9, AND NOT AN ASSUMPTION AT ALL — the arithmetic of a
			// one-frame write against a two-frame representation of split.
			// This choreography sends ONE MW whose P1 comes from the slot's
			// CLASS (P14, M9, kw.Slot.P1), so P1 is '0' on every memory
			// slot and the transmit half of a genuine split has no frame to
			// go in. Dropping the difference silently is the very loss
			// decision 11 exists to prevent, so it is refused instead. The
			// day A9 lifts and a second MW with P1=1 becomes a documented
			// write, this is the rung that changes.
			return res, &driver.WriteRefusedError{
				Slot:   ch.Slot,
				Fields: []spec.Field{spec.FieldTxFrequency},
				Reason: fmt.Sprintf("this channel is split — tx_frequency %d Hz against frequency %d Hz — and one MW cannot express that: its P1 is derived from the slot's class and is '0' here, which writes the channel as simplex (the legend at 590:1519-1520, the consequence sentence at 590:1521-1523). The difference is refused rather than dropped", data.TxFreqHz.Value, data.FreqHz),
			}
		}
	}

	// Decision 14 — the one index the published tone domain carries that the
	// printed CN chart does not. THE BOUND IS CONSULTED FROM THE SAME PLACE
	// AS ITS DATUM: kw.MaxCTCSSIndex is core/kw's own reading of CN's
	// printed 00-41 (590:411), which is also what the MW builder enforces.
	// The rung is here, ahead of the frame, so the refusal is TYPED and
	// names the decision rather than arriving as a codec parse error.
	if data.ToneRx.State == codeplug.Known {
		idx, ok := toneIndex(data.ToneRx.Value)
		if !ok || idx > kw.MaxCTCSSIndex {
			return res, refuse(ch.Slot, registerDecision14, []spec.Field{spec.FieldToneRx},
				"tone_rx %v is index %d in the 43-entry TN chart this row publishes (590:2296-2306), and the printed CN chart stops at %02d (590:411, the chart itself at 590:416-426). spec.Capabilities carries ONE tone domain and one AdmitsTone predicate for both directions (M-E1), so the domain publishes the value and the write path refuses it; a receive tone this radio cannot be told to listen for is not written",
				data.ToneRx.Value, idx, kw.MaxCTCSSIndex)
		}
	}

	slot, err := s.layout.NewSlot(number, half)
	if err != nil {
		// Unreachable for a slot the banks published, since every published
		// identifier was rendered by this same layout's NewSlot (caps.go).
		// Refuse rather than build a frame from a slot the codec disowns.
		return res, &UnknownSlotError{Slot: ch.Slot, Model: modelNameFor(s.row), Reason: err.Error()}
	}
	cmd, err := s.buildMWSet(ch.Slot, slot, data, p14)
	if err != nil {
		return res, err
	}

	// THE step list, declared in full HERE: after the frame provably exists,
	// before it goes near the wire. ONE element, because this radio's write
	// choreography IS one frame.
	res.Steps = []driver.WriteStep{{Command: "MW"}}
	const mwStep = 0

	if derr := s.send(ctx, cmd); derr != nil {
		// A "?;" IS ATTRIBUTABLE and any other transport failure is not:
		// the radio explicitly refused a frame that provably went out, so
		// Sent is true there and false where the host cannot tell whether
		// the frame arrived. Confirmed stays false on every path, including
		// success — see this method's doc comment.
		res.Steps[mwStep].Sent = errors.Is(derr, transport.ErrRejected)
		return res, fmt.Errorf("ts590: WriteChannel %s: %w", ch.Slot, derr)
	}
	res.Steps[mwStep].Sent = true
	return res, nil
}

// send writes cmd and types the two wire events a Kenwood exchange can meet,
// through the same wireFailure the probe and the read path use (ts590.go), so
// one rule is applied once.
func (s *Session) send(ctx context.Context, cmd kw.Command) error {
	if _, err := s.eng.Do(ctx, cmd, mwSetSpec()); err != nil {
		return wireFailure(s.layout, "MW", err)
	}
	return nil
}

// firmwareBlocksWrites reports whether this session's radio is a TS-590S
// whose channel writes A13/A14 close, and the sentence saying why.
//
// TWO CAUSES, ONE RUNG, AND THE ROW IS PART OF THE TEST. On a TS-590S at
// firmware >= 2.00 byte 28 may be LIVE: the document guarantees "always 0"
// for 1.xx and says nothing about 2.xx (590:1478, 590:1564), while the row
// publishes FieldFilter Unsupported because spec.Capabilities is a STATIC
// per-model value that cannot say "Supported iff FV >= 2.00" (Q12). Writing
// such a radio would reset its filter selection on any unrelated edit. And an
// FV this programme cannot read as A13's assumed M.NN form takes the same
// branch, which is what makes A13 load-bearing on a REFUSAL path: if the
// grammar assumption is wrong, the comparison cannot be made, and the
// conservative answer is the one taken.
//
// THE SESSION IS NOT REFUSED EITHER WAY. The radio stays fully readable and
// the raw FV bytes reach the probe note through Session.FirmwareAnswer, so an
// owner of a 2.xx TS-590S can see the capability this row does not publish.
//
// NOTHING ON THE SG ROW BRANCHES ON FV AT ALL (matrix §4, divergence 2): the
// SG's byte 28 is live by construction, unconditionally on firmware, so its
// firmware answers no question this programme asks. The row test is the first
// line of this function for that reason, and
// TestWriteChannel_A13A14RefusesChannelWritesOnAnSWhoseFVIsHighOrUnreadable
// pins the SG rows that must refuse NOTHING.
func (s *Session) firmwareBlocksWrites() (string, bool) {
	if s.row != RowS {
		return "", false
	}
	if !s.fvGrammarOK {
		return fmt.Sprintf("this TS-590S answered %q to FV;, which this programme cannot read as the four-character M.NN form A13 assumes from the book's one worked example (\"for firmware version 1.00, it reads FV1.00;\", 590:1035). Byte 28 is guaranteed '0' only on an S at firmware 1.xx (590:1478, 590:1564), and a version this programme cannot compare takes the conservative branch: the session reads normally and its CHANNEL WRITES are refused", s.fvAnswer), true
	}
	if s.fvMajor >= filterLiveMajor {
		return fmt.Sprintf("this TS-590S reports firmware %s, and byte 28 is guaranteed '0' only in firmware 1.xx (590:1478, 590:1564); at %d.xx it may be a LIVE FILTER A/B selector this row cannot publish, because spec.Capabilities is a static per-model value and cannot say \"Supported iff FV >= 2.00\" (Q12). Writing a channel would reset the radio's filter selection on any unrelated edit, so channel writes are refused while the session reads normally", s.fvAnswer, filterLiveMajor), true
	}
	return "", false
}

// fmP14 resolves a published mode NAME to P14's two bytes, and is the A23
// rung.
//
// P14 IS A FLAG ORTHOGONAL TO THE MODE NIBBLE WHOSE ONLY PRINTED MEANINGS ARE
// "00: FM Normal" and "01: FM Narrow" (590:1569-1571). Nothing in either book
// says what it means in SSB, CW, AM or FSK, so a non-FM write would have to
// put a byte on the wire whose meaning this programme does not hold — which
// is the one thing decision 12 forbids. The refusal lifts per (row, mode)
// CELL (decision 13), so it is stated once here for whichever cell a caller
// reached.
//
// THE NAME COMES BACK FROM THE LAYOUT, not from a literal "FM" here: the
// candidate P14 values are handed to kw.RecordModeName and the answer is
// compared with what the caller asked for, so the byte and the published mode
// name are the same pairing caps.go's modeNames advertises and read.go's
// mapper produces.
//
// A MODE THIS ROW DOES NOT PUBLISH AT ALL IS A DIFFERENT REFUSAL, and keeping
// the two apart is the point: A23 is the narrow statement that a mode this
// radio HAS cannot be written because one byte beside it has no printed
// meaning there, and calling an unknown mode name "A23" would put an
// assumption's name on an ordinary typo.
func (s *Session) fmP14(slotID, mode string) (string, error) {
	for _, p14 := range []string{fmNormalWire, fmNarrowWire} {
		if name, ok := s.layout.RecordModeName(kw.Record{Mode: kw.ModeFM, Byte3940: p14}); ok && name == mode {
			return p14, nil
		}
	}
	for _, name := range modeNames(s.layout) {
		if name == mode {
			return "", refuse(slotID, registerA23, []spec.Field{spec.FieldMode},
				"mode %q is not FM, and P14 (bytes 39-40) is a two-byte flag whose only printed meanings on this row are \"00: FM Normal\" and \"01: FM Narrow\" (590:1569-1571). What it means in any other mode is unprinted, so a write would have to invent the byte; the refusal lifts per (row, mode) cell, not per mode and not per pair",
				mode)
		}
	}
	return "", &driver.WriteRefusedError{
		Slot:   slotID,
		Fields: []spec.Field{spec.FieldMode},
		Reason: fmt.Sprintf("mode %q is not one the %s publishes", mode, modelNameFor(s.row)),
	}
}

// toneIndex is the position of tone in the 43-entry chart this row publishes,
// which IS the CAT tone number: spec.Validate requires CTCSSTones to be
// strictly ascending precisely so the slice index doubles as the wire value
// (caps.go's kenwoodCTCSSTones).
func toneIndex(tone spec.Tone) (int, bool) {
	for i, t := range kenwoodCTCSSTones {
		if t == tone {
			return i, true
		}
	}
	return 0, false
}

// buildMWSet maps a populated channel onto its ONE 50-byte MW Set, refusing
// any value the record cannot carry. Called only after the whole ladder above
// has passed.
//
// THE MANDATORY-KNOWN REFUSALS COME FIRST, and their position is load-bearing
// rather than stylistic: each names a byte that is LIVE on this row and
// transmitted on every write with no "leave it alone" encoding, so a
// non-Known value there cannot be omitted from the frame — it can only be
// MANUFACTURED, which is what codeplug's write rule forbids for a field whose
// state says "preserve whatever the radio has". Writing a zero instead would
// be the silent default this milestone refuses everywhere else. The FT-891
// refuses in this same position for this same reason, on its own live flag.
//
// BYTE 28 IS THE ONE BYTE THIS PATH WRITES WITHOUT READING IT, and it has a
// register entry rather than a default: on the SG it is a live FILTER A/B
// selector and comes from the channel (590:1560-1563), while on the S it is
// the printed-fixed '0' the document guarantees for firmware 1.xx (590:1478,
// 590:1564 — A14). That the S branch is honest depends on the A13/A14 rung
// having already refused every S whose FV is >= 2.00 or unparseable: on an S
// that reaches here, the guarantee holds.
//
// EVERYTHING ELSE IS THE CODEC'S. core/kw re-validates the mode nibble
// (A18b), both tone indices against their printed charts (A21), byte 19, the
// tone mode, byte 28, P14, byte 41, the name's width and charset (A2) and the
// 50-byte width itself, and it writes the thirteen printed-fixed bytes from
// the LAYOUT's own set rather than from the record.
func (s *Session) buildMWSet(slotID string, slot kw.Slot, data codeplug.ChannelData, p14 string) (kw.Command, error) {
	for _, m := range []struct {
		field    spec.Field
		state    codeplug.FieldState
		position string
	}{
		{spec.FieldDataMode, data.DataMode.State, "byte 19, the data mode (590:1546-1548)"},
		{spec.FieldToneMode, data.ToneMode.State, "P7 at byte 20, the tone mode (590:1549-1553)"},
		{spec.FieldToneTx, data.ToneTx.State, "P8 at bytes 21-22, the TN index (590:1554-1555)"},
		{spec.FieldToneRx, data.ToneRx.State, "P9 at bytes 23-24, the CN index (590:1556-1557)"},
		{spec.FieldScanSkip, data.ScanSkip.State, "P15 at byte 41, the channel lockout (590:1572-1574)"},
	} {
		if m.state != codeplug.Known {
			return kw.Command{}, mandatoryFieldRefusal(slotID, m.field, m.state, m.position)
		}
	}

	byte28 := byte('0')
	if s.row == RowSG {
		if data.Filter.State != codeplug.Known {
			return kw.Command{}, mandatoryFieldRefusal(slotID, spec.FieldFilter, data.Filter.State,
				"P11 at byte 28, the FILTER A/B selector (590:1560-1563)")
		}
		wire, ok := filterWire[data.Filter.Value]
		if !ok {
			// Unreachable after CheckFieldStates, which judges a Known
			// filter against this row's own Filters vocabulary — the same
			// two labels this map is built from.
			return kw.Command{}, &driver.WriteRefusedError{
				Slot: slotID, Fields: []spec.Field{spec.FieldFilter},
				Reason: fmt.Sprintf("filter %q is not one of this row's printed labels", data.Filter.Value),
			}
		}
		byte28 = wire
	}

	toneMode, ok := toneModeWire[data.ToneMode.Value]
	if !ok {
		// Unreachable after CheckFieldStates, which judges a Known tone
		// mode against this session's ToneModes vocabulary — the same four
		// values read.go's forward map carries.
		return kw.Command{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneMode},
			Reason: fmt.Sprintf("tone mode %q is not one this row publishes", data.ToneMode.Value),
		}
	}
	toneTx, ok := toneIndex(data.ToneTx.Value)
	if !ok {
		return kw.Command{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneTx},
			Reason: fmt.Sprintf("tone_tx %v is not in the 43-entry chart this row publishes (590:2296-2306)", data.ToneTx.Value),
		}
	}
	toneRx, ok := toneIndex(data.ToneRx.Value)
	if !ok {
		return kw.Command{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneRx},
			Reason: fmt.Sprintf("tone_rx %v is not in the 43-entry chart this row publishes (590:2296-2306)", data.ToneRx.Value),
		}
	}

	cmd, err := s.layout.BuildMWSet(kw.Record{
		Slot: slot,
		// P4, eleven digits at bytes 7-17 (590:1541-1543). The codec
		// refuses anything wider, and its message names the FIELD WIDTH
		// rather than a tuning range (A17) — which is the only frequency
		// bound this milestone ships, MinFreqHz/MaxFreqHz being 0/0 (M-E6).
		FreqHz: data.FreqHz,
		// P5, and it is FM by construction: fmP14 refused every other
		// published mode under A23, so the nibble and the flag agree.
		Mode:     kw.ModeFM,
		Byte3940: p14,
		Byte19:   boolWire(data.DataMode.Value),
		ToneMode: toneMode,
		// P8 and P9 are INDEPENDENT transmit and receive indices, which is
		// what makes "3: Cross Tone ON" expressible at all (590:1167-1171).
		ToneIndex:  toneTx,
		CTCSSIndex: toneRx,
		Byte28:     byte28,
		Byte41:     boolWire(data.ScanSkip.Value),
		// P16, passed as the LOGICAL value: the builder pads it to eight
		// bytes with spaces (A1) and refuses a name that would not fit or
		// that carries a byte outside A2's charset.
		Name: data.Tag,
		// AnswerP1 is a PARSER output and is deliberately left zero: the
		// builder derives P1 from the slot's CLASS (M9, kw.Slot.P1), which
		// is what keeps a section channel's END slot from being written
		// with its START frequency.
	})
	if err != nil {
		// TWO SENTINELS, DELIBERATELY (Go's multi-%w). This IS a write
		// refusal on the neutral seam, so errors.Is(err,
		// driver.ErrWriteRefused) must hold for it as for every other
		// pre-wire rung — AND the codec's own typed cause survives, because
		// the design names it: "Frequency beyond the 11-digit field →
		// OutOfDomainError" (§Error handling). Folding it into a
		// *driver.WriteRefusedError, as the Yaesu line does, would drop that
		// type and leave a caller string-matching a sentence.
		return kw.Command{}, fmt.Errorf("ts590: WriteChannel %s: %w: %w", slotID, driver.ErrWriteRefused, err)
	}
	return cmd, nil
}

// mandatoryFieldRefusal is the refusal a live, always-transmitted byte earns
// when the channel has no Known value for it. See buildMWSet.
func mandatoryFieldRefusal(slotID string, field spec.Field, state codeplug.FieldState, position string) *driver.WriteRefusedError {
	return &driver.WriteRefusedError{
		Slot:   slotID,
		Fields: []spec.Field{field},
		Reason: fmt.Sprintf("%s FieldState is %q, not %q, and %s is transmitted on every write with no \"leave it alone\" encoding — so a non-Known value here could only be manufactured, which is what a state meaning \"preserve whatever the radio has\" forbids", field, state, codeplug.Known, position),
	}
}

// boolWire renders a neutral flag as the '0'/'1' both books print for every
// two-valued byte in this record.
func boolWire(on bool) byte {
	if on {
		return '1'
	}
	return '0'
}

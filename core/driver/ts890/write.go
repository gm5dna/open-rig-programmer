// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

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

// The ASSUMED-register entries and design decisions this file's SEMANTIC
// refusals name. They are cited BY NAME rather than by position, and the
// authoritative registers are elsewhere: A-entries in core/kw/ma/doc.go, the
// numbered decisions in this milestone's design. No entry is re-registered
// here.
//
// A REGISTER ENTRY IS PART OF THE REFUSAL, NOT A COMMENT ON IT. Every rung of
// this ladder answers "refused", and on an unconsented RealHardware session
// the CAPABILITY gate answers first for every write (writeTrialsComplete is
// false), so a caller — or a test — that could see only the fact of refusal
// could not tell an unlifted assumption from a missing consent.
// RefusalError.Register is what distinguishes them, and each rung's own pin
// asserts it (refusals_test.go's assertRegister).
//
// THREE RUNGS DELIBERATELY CARRY NO ENTRY, and the omission is the claim: the
// fleet's FieldState walk, the capability gate, and the refusals whose
// authority is a PRINTED line rather than an assumption — a mode this row does
// not publish (the legend, 890:3976-3992), a name past ten characters
// (890:3208-3209) and a name carrying the frame's own terminator (890:92-96).
// Putting a register name on any of those would attribute an ordinary typo to
// an unlifted claim.
const (
	// registerA2 is the memory name's CHARSET — the printable-ASCII set this
	// design writes, which neither book prints for P13. The width and the
	// ';' exclusion beside it are NOT A2: one is printed and the other is
	// forced by the envelope. LIFT: L-HW-2, per registry row.
	registerA2 = "A2"
	// registerA3 is the blank-target rung: whether an MA0 Set to an
	// unassigned channel can CREATE it is unprinted, while five sibling
	// commands print an unassigned-channel prohibition. LIFT: L-HW-3,
	// hardware item 1 — THE GATE for one-frame writes, per registry row.
	registerA3 = "A3"
	// registerDecision8 is the tone-domain asymmetry: the published TN chart
	// carries 1750 Hz and the printed CN chart does not, and
	// spec.Capabilities has no per-direction tone axis to say so (M-E1
	// recurring).
	registerDecision8 = "decision 8"
	// registerDecision9 is the secondary side: the neutral model carries the
	// PRIMARY side of an MA0 record and nothing else, so a write is refused
	// both when the candidate has no TX disposition at all (rung 6) and when
	// the radio's own secondary fields are not what the Set would emit
	// (rung 11).
	registerDecision9 = "decision 9"
	// registerDecision15 is the standing no-erase rule, which on this row
	// declines a PRINTED MA5 deletion command rather than an ambiguous side
	// effect.
	registerDecision15 = "decision 15"
)

// RefusalError is a write refusal that names the ASSUMED-register entry or
// design decision it comes from.
//
// IT IS A *driver.WriteRefusedError AND MORE, never instead: Unwrap returns
// the embedded fleet error, so errors.Is(err, driver.ErrWriteRefused) holds
// through it, errors.As recovers the fleet type for a caller that wants the
// slot and the fields, and errors.As recovers THIS type for a caller — or a
// test — that needs to know WHICH rung answered. The message is the fleet's
// own sentence, with the register entry named inside Reason so a user reading
// a log sees it too. Pair 1's shape (core/driver/ts590/write.go), minted per
// package because the entries it names are this row's.
type RefusalError struct {
	driver.WriteRefusedError
	// Register is the entry this refusal comes from — one of the constants
	// above, and the same string Reason names in prose.
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

// writeChannelGapHook, when non-nil, is called by WriteChannel with opMu held,
// AFTER the pre-write read and BEFORE the Set — a test-only seam, read.go's
// readChannelGapHook for the write path. Always nil in production, so it costs
// one nil check per write.
//
// It exists for TestWriteChannel_IsAtomicUnderOpMu, to park an operation in
// the exact gap the lock exists to close: this is the milestone's first
// two-exchange write, and an interleaving there is the failure P13 forbids and
// the one goroutine scheduling will not reproduce on demand.
var writeChannelGapHook func()

// ma0SetSpec is the transport spec for the ONE MA0 Set this driver sends:
// fire-and-forget — write the frame, listen for a bounded window in case a
// "?;" arrives, and treat silence as the end of the exchange.
//
// IT IS WRITTEN OUT RATHER THAN TAKEN FROM transport.CATWriteSpec(), whose
// value is identical: that helper's name says CAT, this family is not CAT, and
// a Kenwood file naming a Yaesu-codec helper is exactly the borrowing this
// milestone's fence exists to prevent. What is shared is the transport CLASS,
// which is neutral.
//
// SILENCE IS NOT SUCCESS ON THIS FAMILY, which is why WriteChannel reports
// Sent rather than Confirmed — see there. What the bounded window buys is the
// REJECTION: a "?;" inside it is attributable.
//
// RetryReads 0, NECESSARILY: a write is never resent (transport safety
// obligation 2), and Do refuses a write-class spec with a non-zero RetryReads
// before writing anything.
// TestMA0SetSpec_IsFireAndForgetAndNeverRetries pins all three properties.
func ma0SetSpec() transport.CommandSpec {
	return transport.CommandSpec{Class: transport.ClassWrite}
}

// toneModeWire is read.go's toneModeNames read backwards: the neutral
// vocabulary this row publishes to the record's P5 byte.
//
// DERIVED RATHER THAN TRANSCRIBED. A hand-written inverse of a hand-written
// map is one edit from disagreeing, and inverting the forward map at package
// initialisation makes the disagreement unrepresentable instead.
// TestToneModeWire_IsTheExactInverseOfToneModeNames is the pin.
var toneModeWire = invertToneModes()

func invertToneModes() map[string]byte {
	m := make(map[string]byte, len(toneModeNames))
	for wire, name := range toneModeNames {
		m[name] = wire
	}
	return m
}

// modeWire is caps.go's modeName read backwards: a published mode NAME to the
// two bytes that produce it, P3's legend value and P4's FM width flag.
//
// IT IS A SEARCH OVER THE ONE NAMING RULE AND NOT A SECOND TABLE, for
// toneModeWire's reason and one more: the name is a function of TWO bytes here
// (890:3174-3178), so an inverse map would have to carry pairs, and a pair
// table beside the rule that generates it is the drift this package has
// avoided everywhere else. The domain is 256 bytes checked twice, once per
// session write.
//
// NORMAL IS TRIED BEFORE NARROW, and the order is load-bearing: on a non-FM
// mode modeName ignores the width flag and returns the same name for both, so
// a narrow-first search would report "USB" as narrow and put a P4 of '1' on
// the wire with no source in the channel. The legend's two "Unused" values,
// '0' and '8' (890:3977, 890:3985), have no name at all, so no candidate
// channel can reach them — which is how P7's rung 7 is satisfied by
// construction, and TestModeWire_IsTheExactInverseOfModeName proves both
// halves.
func modeWire(l ma.Layout, name string) (mode byte, fmNarrow bool, ok bool) {
	for b := 0; b <= 0xFF; b++ {
		for _, narrow := range []bool{false, true} {
			if got, found := modeName(l, byte(b), narrow); found && got == name {
				return byte(b), narrow, true
			}
		}
	}
	return 0, false, false
}

// toneIndex is the position of tone in the 51-entry chart this row publishes,
// which IS the CAT tone number: spec.Validate requires CTCSSTones to be
// strictly ascending precisely so the slice index doubles as the wire value
// (caps.go's ctcssTones890).
func toneIndex(tone spec.Tone) (int, bool) {
	for i, t := range ctcssTones890 {
		if t == tone {
			return i, true
		}
	}
	return 0, false
}

// requestedFieldRules pairs each spec.Field with the predicate that reports
// whether a write of this channel actually REQUESTS it.
//
// TWENTY-SIX ENTRIES — every spec.Field but spec.FieldErase, in
// spec.AllFields()' own order (plan P5 permits these rows to name that helper,
// reversing pair 1's rule). FieldErase is not a field a write requests: it is
// the whole shape of a DIFFERENT frame, the printed MA5 of 890:3305 this
// milestone never builds, and WriteChannel refuses an empty channel a rung
// above this table.
//
// EIGHT ARE UNCONDITIONAL, AND THAT IS THE FRAME'S OWN SHAPE. The 40-to-50-byte
// record carries a frequency, a mode, a tone mode, two tone indices, a split
// side, a lockout flag and a name on EVERY write, changed or not, with no
// "leave it alone" encoding anywhere in the grid (890:3164-3209). A write
// therefore requests all eight whether the caller edited them or not, which is
// what makes the capability gate below a real gate.
//
// tx_frequency IS ONE OF THE EIGHT, where pair 1's table has it conditional,
// and that is this pair's largest gain restated on the write side: P8 and P11
// are positions in the one frame, so a Set states this channel's transmit
// disposition every time it goes out. There is no frame in which the field is
// absent and therefore no reading of "unrequested" for it.
//
// THE EIGHTEEN CONDITIONALS EXIST FOR ONE CASE: a caller who hands this driver
// a value the record has no room for — a Known IP+ from an Icom native file, a
// Yaesu shift from a CSV import, a filter from the TS-590SG's own byte 28.
// Naming the field only when a value is actually present is what lets an
// ordinary write through while REFUSING that one, rather than dropping the
// value silently from a frame with nowhere to put it.
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
	// data_mode is CONDITIONAL on this row where pair 1's is unconditional,
	// and that is erratum M-E3 on the write side: this radio has no DA
	// command and this grid has no data byte, so the data-ness is in the mode
	// NAMES (LSB-D, USB-D, FM-D, AM-D at legend values C-F, 890:3989-3992)
	// and a Known flag alongside them is a value with nowhere to go.
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
// channel's write requests. TestRequestedFields_MembershipAndOrder and its two
// neighbours pin the table's membership, its order and the reachability of
// every conditional.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := make([]spec.Field, 0, len(requestedFieldRules))
	for _, r := range requestedFieldRules {
		if r.present(data) {
			fields = append(fields, r.field)
		}
	}
	return fields
}

// WriteChannel implements driver.Session: ONE MA0 Set, reported Sent and never
// Confirmed (plan P13, matrix §3.9), behind the ELEVEN-rung ladder of plan P7 —
// NINE rungs decided locally, then ONE read, then TWO that quote its answer.
//
// PART 1, LOCALLY DECIDABLE. NO FRAME IS BUILT UNTIL EVERY RUNG PASSES:
//
//	parseSlotID      1: the identifier's SYNTAX, and no row (read.go)
//	BankOf           2: membership in THIS session's published banks, which
//	                    is also where classes 100-119 are refused
//	ch.Empty()       3: an erase request, refused over a PRINTED MA5
//	CheckFieldStates 4: the FLEET's FieldState walk, as a black box
//	THE CAPABILITY GATE  5: unverified-write consent, and it comes FIRST of
//	                    the answers a user can act on
//	decision 9       6: no Known TX disposition (a FILE's channel, never a
//	                    channel this programme read — M-E8)
//	candidate        7-9: a mode this row does not publish, a Known tone_rx
//	                    of 1750 Hz, a name outside P13's domain, and the
//	                    live bytes that must carry a Known value
//
// THE CAPABILITY GATE COMES BEFORE THE SEMANTIC RUNGS, which is the fleet's
// ruled order and not a preference (core/driver/ts590/write.go:260-269, the
// whole ladder listed at :244-258): on an unconsented RealHardware session the
// all-Unverified gate is the first answer, and the semantic refusals are what
// a Simulated — or a consented — session meets. writeTrialsComplete is false,
// so on a REAL TS-890S today NOTHING is writable and every channel is refused
// at that gate with every requested field named. That is the profile working,
// not a limitation of this method, and the one route past it is the user's own
// recorded consent (WithConsentedUnverifiedWrites). Consent widens WHAT may be
// attempted, never HOW carefully: every rung below it still fires. Placing the
// gate later would let an unconsented session learn which of its channels the
// programme dislikes before it learned it may not write at all.
//
// THE ONE READ, then PART 2, whose two rungs both QUOTE the answer:
//
//	A3               10: the target is unassigned NOW — this programme does
//	                    not create channels
//	decision 9       11: the radio's SECONDARY fields are not what the Set
//	                    would emit, named by P-number and both values
//
// THERE IS NO 890S-ONLY P4/P10 RUNG, and there never could have been. The book
// requires "the same setting on the transmission side and the reception side
// for FM normal / narrow information (P4, P10)" when setting a split channel
// (890:3219-3221), and codeplug.ChannelData carries ONE FM width — so
// candidate emits P10 FROM P4 by construction and a rung testing the candidate
// alone could never fire. The printed rule is satisfied on emit and enforced on
// the RADIO side by rung 11, which compares the slot's CURRENT P10 against
// what the Set would emit. The codec keeps the same rule as an invariant for
// records nobody built from a channel (core/kw/ma's buildSecond890).
//
// THE READ IS THIS DRIVER'S OWN AND IS NOT ReadChannel. WriteChannel receives
// the candidate and nothing else (core/driver/driver.go:203); clone's fresh
// verify-read is not passed in and a direct caller bypasses clone entirely, so
// a cached answer could be stale. ReadChannel takes opMu, which this method
// holds, so calling it would deadlock as well as read twice.
//
// THE READ-BACK VERIFICATION IS STILL core/clone'S, exactly as on every other
// family. TestWriteChannel_NeverReadsBackAndNeverCallsReadChannel asserts the
// transcript for one write is exactly TWO frames.
//
// SENT, NEVER CONFIRMED. That a "?;" is a REJECTION is the book's own error
// table (890:106-112); that an ACCEPTED Set draws nothing at all is an
// assumption no TS-890S has ever tested. Silence is therefore inconclusive,
// and a driver reporting Confirmed on silence would be asserting that reading
// as a fact. A "?;" inside the bounded window is different: the frame provably
// went out and the radio provably refused it, so that outcome reports Sent
// true with the typed kw.RejectionError.
//
// THE OPERATION MUTEX IS HELD FOR THE WHOLE CALL, and here that is
// load-bearing rather than tidy: the write is TWO exchanges, transport.Engine
// serialises each exchange and not the pair, and a concurrent operation landing
// between them "would decide against one radio state and write against
// another" (core/driver/ic705/write.go:58-60). It is NOT held across
// write-then-verify: that pair is core/clone's, as the driver seam assigns it,
// and TestWriteChannel_IsAtomicUnderOpMu distinguishes the two.
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

	// RUNG 2 — membership in THIS session's published banks. The same branch,
	// and the same message, that refuses a READ of an unpublished slot: the
	// Programmable VFO and extension classes 100-119 are refused here exactly
	// as "999" is, with no special case and no invented radio behaviour (see
	// UnknownSlotError, read.go, and decision 12).
	bankID, ok := s.caps.BankOf(ch.Slot)
	if !ok {
		return res, &UnknownSlotError{
			Slot:   ch.Slot,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}

	// RUNG 3 — AN EMPTY CHANNEL IS AN ERASE REQUEST, AND IT IS REFUSED.
	// This book prints a DEDICATED deletion command, MA5 "Memory Channel
	// (Channel Deletion)" (890:3305, its channel-class note at 890:3311),
	// which is stronger evidence than pair 1's ambiguous short-MW side
	// effect — and the standing no-erase rule declines it anyway: this
	// programme does not delete a user's channels. core/kw/ma's outbound gate
	// refuses an MA5 frame besides, and spec.FieldErase is write-Supported on
	// no bank of this row.
	//
	// THE RUNG STAYS AHEAD OF EVERY FIELD CHECK STRUCTURALLY and not merely
	// by preference: codeplug.Channel.Data is a pointer (core/codeplug/
	// channel.go:19, Empty() at :210) and every rung below dereferences it,
	// so without this one a caller handing an empty channel would PANIC — and
	// core/clone hands one, because an emptied row in a loaded file is
	// exactly that.
	if ch.Empty() {
		return res, refuse(ch.Slot, registerDecision15, []spec.Field{spec.FieldErase},
			"this milestone builds no erase. This book prints a DEDICATED deletion command — MA5, \"Memory Channel (Channel Deletion)\" (890:3305, 890:3311) — and the standing no-erase rule declines to build it, core/kw/ma's outbound gate refuses an MA5 frame, and spec.FieldErase is write-Supported on no bank of this row")
	}
	data := *ch.Data

	// RUNG 4 — the fleet's FieldState walk (core/driver/fieldstate.go:190),
	// consumed as a BLACK BOX exactly as ft710, ts480, ts590 and the Yaesu
	// line consume it. What it prevents is SILENT rather than loud: a value
	// carried alongside a state meaning "preserve whatever the radio has" is
	// never named by requestedFields, so without this rung it would be
	// DROPPED from the frame and the write would report success.
	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	// RUNG 5 — THE CAPABILITY GATE; see this method's doc comment for why it
	// is here and not lower. Every requested field must pass
	// spec.FieldSupport.CanWrite for THIS slot's bank in THIS session's
	// capabilities — spec.Supported, or spec.ConsentedUnverified, which is
	// the label every writable field of a consented real-hardware session
	// carries. spec.Inert is accepted as acceptable-to-TRANSMIT, the fleet's
	// stated stance; no field of this row is Inert today, since Inert is a
	// HARDWARE finding and this radio has been asked nothing.
	var unwritable []spec.Field
	for _, f := range requestedFields(data) {
		fs := s.caps.FieldSupport(bankID, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session (the MA0 record cannot express the field on this row, or this session's capability profile does not support writing it)",
		}
	}

	// RUNG 6 — decision 9's Unknown TX disposition, and it is REACHABLE ONLY
	// FROM A FILE OR A CHIRP IMPORT (M-E8). One MA0 answer carries both
	// frequencies and the split flag (890:3191-3203), so every channel this
	// programme has READ has a Known tx_frequency by construction — which is
	// the difference between this pair and pair 1, where the same clause was
	// a blanket obstruction on every channel. The field is graded on this
	// row's one bank, so there is no per-bank exemption to make.
	if data.TxFreqHz.State != codeplug.Known {
		return res, refuse(ch.Slot, registerDecision9, []spec.Field{spec.FieldTxFrequency},
			"this channel's state for tx_frequency is %q, not %q. The MA0 record states a channel's transmit disposition on EVERY write — P8's eleven digits and P11's split flag are positions in the one frame (890:3191-3203) — so there is no frame in which the field can be left unsaid, and a value this programme does not hold could only be manufactured. A channel READ from this radio always carries one; this refusal is what a file or a CHIRP import meets",
			data.TxFreqHz.State, codeplug.Known)
	}

	// RUNGS 7-9 and the live-byte refusals, then the record the Set would
	// emit. Still locally decidable: nothing has reached the wire.
	slot, err := s.layout.NewSlot(number)
	if err != nil {
		// Unreachable for a slot the bank published, since every published
		// identifier was rendered by this same layout's NewSlot (caps.go).
		// Refuse rather than build a frame from a slot the codec disowns.
		return res, &UnknownSlotError{Slot: ch.Slot, Reason: err.Error()}
	}
	want, err := s.candidate(ch.Slot, slot, data)
	if err != nil {
		return res, err
	}

	// THE ONE READ, held with the Set under this method's single opMu hold.
	current, err := s.readCurrent(ctx, ch.Slot, slot)
	if err != nil {
		return res, err
	}
	if writeChannelGapHook != nil {
		writeChannelGapHook()
	}

	// The radio's own P11 must agree with its own P8-P10. A16 prints the
	// simplex case as the WHOLE split side zeroed (890:3217-3218) while
	// core/kw/ma decodes P11 independently of that window, so a frame in
	// which the two disagree is one the neutral model cannot represent: the
	// read drops P11 and candidate rebuilds it from tx_frequency, so writing
	// such a channel back would flip the split flag with nothing in the file
	// asking for it.
	if current.Split != (current.TXFreqHz != 0) {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Reason: fmt.Sprintf("the pre-write read's P11 says split=%v while its P8 split transmission frequency is %d (890:3191-3203); a single memory channel's whole split side is zero (890:3217-3218), so this answer is one this programme cannot restate and the write is refused rather than made with a P11 the radio did not send", current.Split, current.TXFreqHz),
		}
	}

	// RUNG 10 — the target is unassigned NOW. Whether an MA0 Set can CREATE a
	// channel is A3: MA2, MA3, MA6, MA4 and MA7 all print an
	// unassigned-channel prohibition (890:3265, 890:3282, 890:3300-3301,
	// 890:3324, 890:3356), MA0 prints nothing either way, and MA1 prints the
	// opposite for its own case. So a create path exists and only MA0's own
	// participation is unknown — and THIS PROGRAMME DOES NOT CREATE CHANNELS
	// until it is observed.
	if current.Empty {
		return res, refuse(ch.Slot, registerA3, nil,
			"the pre-write read shows this channel unassigned now (its P2-P12 window is blank, 890:3215-3216), and whether an MA0 Set can CREATE a channel is nowhere printed: five sibling commands print an unassigned-channel prohibition (890:3265, 890:3282, 890:3300-3301, 890:3324, 890:3356) and MA0 says nothing either way. This programme does not create channels — the observation that would settle it is hardware item 1 (L-HW-3), a Set to a channel confirmed blank from the front panel, read back")
	}

	// RUNG 11 — the radio's SECONDARY side against what the Set would emit.
	if err := secondarySideUnchanged(ch.Slot, current, want); err != nil {
		return res, err
	}

	cmd, err := s.layout.BuildMA0Set(want)
	if err != nil {
		// TWO SENTINELS, DELIBERATELY (Go's multi-%w). This IS a write
		// refusal on the neutral seam, so errors.Is(err,
		// driver.ErrWriteRefused) must hold for it as for every other rung —
		// AND the codec's own typed cause survives, because a frequency
		// beyond the eleven-digit field is a *kw.ParseError and a caller
		// should not have to string-match a sentence to find that out.
		return res, fmt.Errorf("ts890: WriteChannel %s: %w: %w", ch.Slot, driver.ErrWriteRefused, err)
	}

	// THE step list, declared in full HERE: after the frame provably exists,
	// before it goes near the wire. ONE element, because ONE of this write's
	// two frames mutates — the other is the read above, which changes
	// nothing and is not a step.
	res.Steps = []driver.WriteStep{{Command: "MA0"}}
	const setStep = 0

	if derr := s.send(ctx, cmd); derr != nil {
		// A "?;" IS ATTRIBUTABLE and any other transport failure is not: the
		// radio explicitly refused a frame that provably went out, so Sent is
		// true there and false where the host cannot tell whether the frame
		// arrived. Confirmed stays false on every path, including success.
		res.Steps[setStep].Sent = errors.Is(derr, transport.ErrRejected)
		return res, fmt.Errorf("ts890: WriteChannel %s: %w", ch.Slot, derr)
	}
	res.Steps[setStep].Sent = true
	return res, nil
}

// candidate maps a populated channel onto the MA0 record the Set would emit,
// and carries P7's rungs 7, 8 and 9 plus the live-byte refusals. It is called
// BEFORE the pre-write read, so every refusal it returns is pre-wire.
//
// P11 HAS NO SOURCE IN THE NEUTRAL MODEL AND IS DERIVED, WITH A STATED LIMIT.
// codeplug.ChannelData carries one frequency and one tx_frequency and no split
// flag at all, so the only honest derivation is the book's own statement of
// the simplex case: "When reading a single memory channel, all parameters for
// Split Transmission become 0" (890:3217-3218, A16, DOCUMENTED). A Known
// tx_frequency of ZERO is therefore the simplex state itself and emits the
// printed zeroed side; anything else is a split. THE COST, both directions:
// a channel whose tx_frequency is Known and EQUAL to its receive frequency is
// written as a SPLIT channel, because nothing in the model distinguishes it
// from one — against a radio slot that is currently simplex that write is
// refused by rung 11 rather than silently made, so the cost is a refusal and
// never a wrong byte; and a genuinely split channel whose two sides are equal
// round-trips as a split, which a "tx != rx" derivation would flatten to
// P11 = 0. NO FLAG IS INVENTED for either. TestWriteChannel_TheStatedSplitLimit
// records both halves, and the day codeplug carries a split flag this is the
// derivation that changes.
//
// P9 AND P10 COME FROM THE PRIMARY SIDE, which is decision 9's whole shape:
// the record gives the transmit side its own mode and its own width
// (890:3193-3200) and the neutral model has nowhere to keep either, so the Set
// says what the candidate says and rung 11 refuses where the radio disagrees.
// P10 = P4 is additionally the book's own rule for a split channel
// (890:3219-3221).
func (s *Session) candidate(slotID string, slot ma.Slot, data codeplug.ChannelData) (ma.Record, error) {
	// RUNG 7 — the mode. A NAME THIS ROW DOES NOT PUBLISH IS A PLAIN FLEET
	// REFUSAL AND NAMES NO REGISTER ENTRY: what produces it is the printed
	// legend, not an assumption, and calling an ordinary typo "A-something"
	// would put an unlifted claim's name on it. The legend's two "Unused"
	// values, '0' and '8' (890:3977, 890:3985), are unreachable from a name —
	// the layout carries no name for either — so this rung is where P7's
	// "the mode is 0 or 8" is answered.
	mode, narrow, ok := modeWire(s.layout, data.Mode)
	if !ok {
		return ma.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not one the %s publishes. P3's vocabulary is the OM P2 legend's (890:3174-3175, the legend at 890:3976-3992), whose values '0' and '8' are printed \"Unused\" and carry no name at all", data.Mode, modelName),
		}
	}

	// RUNG 8 — decision 8, the one index the published tone domain carries
	// that the printed CN chart does not. THE BOUND IS CONSULTED FROM THE
	// SAME PLACE AS ITS DATUM: ma.Layout.MaxCTCSSIndex is core/kw/ma's own
	// reading of CN's printed 00-49 (890:1354-1369), which is also what the
	// MA0 builder enforces. The rung is here, ahead of the frame, so the
	// refusal is TYPED and names the decision rather than arriving as a codec
	// parse error. A 1750 Hz tone_rx cannot come OFF a radio — the parser
	// bounds P7 to the same chart — so this is about a value from a FILE.
	if data.ToneRx.State == codeplug.Known {
		// A tone the chart does not carry AT ALL is a different refusal and
		// is left to the unreachable guard below: calling it "index 0" here
		// would name a tone the caller never asked for.
		if idx, ok := toneIndex(data.ToneRx.Value); ok && idx > int(s.layout.MaxCTCSSIndex()) {
			return ma.Record{}, refuse(slotID, registerDecision8, []spec.Field{spec.FieldToneRx},
				"tone_rx %v is index %d in the 51-entry TN chart this row publishes (890:5149-5163, the 1750.0 entry at 890:5162), and the printed CN chart stops at %02d (890:1354-1369). spec.Capabilities carries ONE tone domain and one AdmitsTone predicate for both directions, so the domain publishes the value and the write path refuses it; a receive tone this radio cannot be told to listen for is not written",
				data.ToneRx.Value, idx, s.layout.MaxCTCSSIndex())
		}
	}

	// RUNG 9 — the name, whose three causes have three different authorities.
	if err := s.checkTag(slotID, data.Tag); err != nil {
		return ma.Record{}, err
	}

	// THE LIVE BYTES, and their position is load-bearing rather than
	// stylistic: each names a byte transmitted on EVERY write with no "leave
	// it alone" encoding, so a non-Known value there cannot be omitted from
	// the frame — it can only be MANUFACTURED, which is what a state meaning
	// "preserve whatever the radio has" forbids. Writing a zero instead would
	// be the silent default this milestone refuses everywhere else.
	for _, m := range []struct {
		field    spec.Field
		state    codeplug.FieldState
		position string
	}{
		{spec.FieldToneMode, data.ToneMode.State, "P5 at byte 20, the FM tone type (890:3180-3185)"},
		{spec.FieldToneTx, data.ToneTx.State, "P6 at bytes 21-22, the TN index (890:3186-3187)"},
		{spec.FieldToneRx, data.ToneRx.State, "P7 at bytes 23-24, the CN index (890:3188-3190)"},
		{spec.FieldScanSkip, data.ScanSkip.State, "P12 at byte 39, the scan lockout (890:3205-3207)"},
	} {
		if m.state != codeplug.Known {
			return ma.Record{}, mandatoryFieldRefusal(slotID, m.field, m.state, m.position)
		}
	}

	toneType, ok := toneModeWire[data.ToneMode.Value]
	if !ok {
		// Unreachable after CheckFieldStates, which judges a Known tone mode
		// against this session's ToneModes vocabulary — the same four values
		// read.go's forward map carries.
		return ma.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneMode},
			Reason: fmt.Sprintf("tone mode %q is not one this row publishes", data.ToneMode.Value),
		}
	}
	toneTx, ok := toneIndex(data.ToneTx.Value)
	if !ok {
		// Unreachable after CheckFieldStates, which judges a Known tone
		// against caps.AdmitsTone — the same 51-entry chart.
		return ma.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneTx},
			Reason: fmt.Sprintf("tone_tx %v is not in the 51-entry chart this row publishes (890:5149-5163)", data.ToneTx.Value),
		}
	}
	toneRx, ok := toneIndex(data.ToneRx.Value)
	if !ok {
		// Unreachable for the same reason as toneTx, and written out rather
		// than discarded with a blank: an ignored miss would encode index 0,
		// which is a REAL tone (67.0 Hz), so the silent failure here would be
		// a wrong value on the wire rather than a refusal.
		return ma.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneRx},
			Reason: fmt.Sprintf("tone_rx %v is not in the 51-entry chart this row publishes (890:5149-5163)", data.ToneRx.Value),
		}
	}

	rec := ma.Record{
		Slot:       slot,
		FreqHz:     data.FreqHz,
		Mode:       mode,
		FMNarrow:   narrow,
		ToneType:   toneType,
		ToneIndex:  toneTx,
		CTCSSIndex: toneRx,
		Lockout:    data.ScanSkip.Value,
		// P13 VERBATIM, with no pad and no trim: the terminator floats at
		// 40 + len(name) (890:3181-3182), so "AB ;" and "AB;" are distinct
		// frames and a trailing space is real content. This row's half of A1
		// is NOT an assumption (the C-MED-1 reversal), so this ladder needs
		// no special case for a CHIRP-sanitised name that keeps one.
		Name: data.Tag,
	}
	if data.TxFreqHz.Value != 0 {
		rec.TXFreqHz, rec.TXMode, rec.TXFMNarrow, rec.Split = data.TxFreqHz.Value, mode, narrow, true
	}
	return rec, nil
}

// checkTag is rung 9, and its three causes are kept apart because their
// EVIDENCE differs — which is why only one of the three names a register
// entry.
//
// The WIDTH is documented: "Channel name / Up to 10 characters"
// (890:3208-3209), and it is asked of caps.TagLen so the bound a write
// enforces is the one the capability table publishes. The ';' EXCLUSION IS
// FORCED and is not an assumption: it terminates a frame, and this book says
// the terminator's position "differs depending on the command used"
// (890:92-96), so a name containing one would split the frame at the radio's
// own parser. Only the CHARSET is assumed — A2, whose claim is bounded at 0x7E
// because the meaning of 80h-FFh depends on a menu setting this programme does
// not read (890:31-43) — and only that case is a RefusalError.
func (s *Session) checkTag(slotID, tag string) error {
	if len(tag) > s.caps.TagLen {
		return &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldTag},
			Reason: fmt.Sprintf("the name %q is %d characters and this row's P13 holds up to %d (890:3208-3209)", tag, len(tag), s.caps.TagLen),
		}
	}
	if strings.ContainsRune(tag, ';') {
		return &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldTag},
			Reason: fmt.Sprintf("the name %q contains ';', which terminates a frame at the radio's own parser (890:92-96) — the exclusion is forced by the envelope and is not an assumption", tag),
		}
	}
	for i := 0; i < len(tag); i++ {
		if !s.caps.TagByteOK(tag[i]) {
			return refuse(slotID, registerA2, []spec.Field{spec.FieldTag},
				"the name %q carries byte %#02x at position %d, outside %s. This book prints no charset for P13 at all; what it prints is a global coding rule whose 80h-FFh half is remapped by a menu setting this programme does not read (890:31-43), so the domain is narrowed to what cannot be ambiguous",
				tag, tag[i], i+1, s.caps.TagCharsetDescription())
		}
	}
	return nil
}

// secondarySideUnchanged is rung 11: the radio's CURRENT P9 and P10 against
// what the Set would emit, refused NAMING THE P-NUMBERS AND BOTH VALUES.
//
// THIS IS WHERE THIS PAIR'S ONE REAL LOSS IS ENFORCED (decision 9). The record
// gives the transmit side its own mode P9 and its own FM width P10
// (890:3193-3200); codeplug.ChannelData has one mode and one width, so those
// two bytes have no home in the neutral model and a Set built from a candidate
// channel can only restate the primary side. A channel a user built from the
// front panel with an independent transmit mode is therefore READABLE BUT NOT
// REWRITABLE by this programme, and it is refused rather than flattened.
//
// P8 AND P11 ARE DELIBERATELY NOT COMPARED. The transmit FREQUENCY and the
// split flag ARE expressible — tx_frequency is a published field and P11 is
// derived from it (see candidate) — so changing a channel's split frequency,
// or making it simplex by clearing tx_frequency, is an edit this model can
// carry. What it cannot carry is a transmit MODE or WIDTH of its own, and
// those two are exactly what this rung tests.
//
// IT IS ALSO WHERE THE PRINTED P4/P10 RULE BITES (890:3219-3221): the Set
// copies P4 into P10 on emit, so the two can never disagree from the candidate
// alone, and the rule reaches a real radio only as this comparison.
func secondarySideUnchanged(slotID string, current, want ma.Record) error {
	if current.TXMode == want.TXMode && current.TXFMNarrow == want.TXFMNarrow {
		return nil
	}
	return refuse(slotID, registerDecision9, nil,
		"the radio's own split-transmission side is not what this Set would emit: P9, the split transmission mode, is %s on the radio and would be written %s, and P10, its FM normal/narrow flag, is %v and would be written %v (890:3193-3200). codeplug.ChannelData carries ONE mode and ONE FM width, so a one-frame Set can only restate the receive side, and writing it would flatten a transmit setting this programme never held. The channel reads correctly; it is this write that is refused",
		secondMode(current.TXMode), secondMode(want.TXMode), current.TXFMNarrow, want.TXFMNarrow)
}

// secondMode names a record's P9 for a refusal message. A ZERO IS NOT A BYTE
// HERE: core/kw/ma uses it for the printed zeroed split side ("all parameters
// for Split Transmission become 0", 890:3217-3218), which is not a mode value
// at all — '0' is a legend value both books print "Unused" — so quoting it as
// a character would tell a reader the radio answered something it did not.
func secondMode(b byte) string {
	if b == 0 {
		return "the printed zeroed simplex side (890:3217-3218)"
	}
	return fmt.Sprintf("'%c'", b)
}

// mandatoryFieldRefusal is the refusal a live, always-transmitted byte earns
// when the channel has no Known value for it. See candidate.
func mandatoryFieldRefusal(slotID string, field spec.Field, state codeplug.FieldState, position string) *driver.WriteRefusedError {
	return &driver.WriteRefusedError{
		Slot:   slotID,
		Fields: []spec.Field{field},
		Reason: fmt.Sprintf("%s FieldState is %q, not %q, and %s is transmitted on every write with no \"leave it alone\" encoding — so a non-Known value here could only be manufactured, which is what a state meaning \"preserve whatever the radio has\" forbids", field, state, codeplug.Known, position),
	}
}

// readCurrent is the ladder's ONE read: the same three steps ReadChannel takes
// — BuildMA0Read, the exchange under this row's own answer matcher, and
// ParseMA0Answer — performed HERE rather than by calling ReadChannel, which
// takes opMu and would deadlock (see WriteChannel's doc comment).
//
// IT ADDS NO SECOND ANSWER-SLOT GUARD, and that is a ruling rather than an
// omission (T11 review, LOW-1): ma.Layout.MA0AnswerMatcher's prefix is "MA0"
// plus the three channel digits, so the whole correlation key is already
// spelled in the matcher and a frame naming another channel never reaches the
// parser. read.go keeps its own comparison as the second line of defence for
// the read path; duplicating it here would be a second copy of a guard neither
// path can reach.
//
// A FAILURE HERE NEVER SENDS. The Set is built after this returns, so a "?;"
// or a timeout on the pre-write read ends the write with an empty step list
// and nothing mutating on the wire.
func (s *Session) readCurrent(ctx context.Context, slotID string, slot ma.Slot) (ma.Record, error) {
	cmd, err := s.layout.BuildMA0Read(slot)
	if err != nil {
		return ma.Record{}, fmt.Errorf("ts890: WriteChannel %s: %w", slotID, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.ma0Spec(slot))
	if err != nil {
		return ma.Record{}, fmt.Errorf("ts890: WriteChannel %s: the pre-write read: %w", slotID, wireFailure(s.layout.Book(), "MA0", err))
	}
	rec, err := s.layout.ParseMA0Answer(frame)
	if err != nil {
		return ma.Record{}, fmt.Errorf("ts890: WriteChannel %s: the pre-write read: %w", slotID, err)
	}
	return rec, nil
}

// send writes cmd and types the two wire events a Kenwood exchange can meet,
// through the same wireFailure the probe and the read path use (ts890.go), so
// one rule is applied once.
func (s *Session) send(ctx context.Context, cmd ma.Command) error {
	if _, err := s.eng.Do(ctx, cmd, ma0SetSpec()); err != nil {
		return wireFailure(s.layout.Book(), "MA0", err)
	}
	return nil
}

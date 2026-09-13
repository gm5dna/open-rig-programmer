// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

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

// THIS ROW WRITES EXISTING CHANNELS (spec.md's own row for this package:
// "tagged, write existing"), on the core/driver/ts890/ts990 READ-THEN-WRITE
// shape: WriteChannel performs ONE pre-write MR read, and the record it
// builds PRESERVES four raw values this milestone models no spec.Field
// for — P10 (DCS code), P11 (REVERSE), P15 (Memory Group), and P14 (the
// tuning-step index) WHEN AND ONLY WHEN preserving it is safe (below).
// Nothing is invented: each preserved byte is copied VERBATIM from the
// SAME channel's own current wire state, never derived or defaulted.
//
// THREE OF THE FOUR HAVE NO MODE DEPENDENCE AND ARE ALWAYS SAFE TO
// PRESERVE. P10 (DCS code), P11 (REVERSE) and P15 (Memory Group) mean the
// same thing regardless of what else the write changes — REVERSE is a flat
// two-value flag, Memory Group a flat one-digit selector, and DCS code an
// independent index — so copying each one, unread and unmodeled, from the
// channel's own most recent MR answer into its own next MW Set carries no
// value this write did not already have.
//
// P14 IS DIFFERENT, AND THIS IS THE BYTE-LEVEL REASON THE ts590/TS-480 "NO
// SAFE VALUE" PROBLEM DOES NOT FULLY DISSOLVE HERE. Bytes 39-40 are a
// tuning-step INDEX whose two-digit value means different things depending
// on which of two mode families the channel is in — "SSB/CW/FSK mode:
// 00~03" against "AM/FM mode: 00~09" (ts2000:11508-11521, this row's own
// ST command, mode-conditional exactly like the TS-480's). Copying the raw
// index verbatim is safe ONLY WHEN THE WRITE DOES NOT CROSS THAT FAMILY
// BOUNDARY: a step index that meant one physical step size under the
// channel's OLD mode would silently mean a DIFFERENT one under a NEW mode
// in the other family, with nothing in the write asking for that change.
// stepFamilyChanged is the one-line, LOCAL, NO-VOCABULARY guard this
// requires — it classifies a mode into one of the two families the ST
// legend prints and refuses a write that would cross them, WITHOUT
// establishing what any step index means in seconds/hertz or exposing a
// spec.Capabilities.TuningSteps vocabulary of any kind, which is what
// spec §6 Q4 excludes this wave. A same-family write (the common case:
// editing frequency, tag, tone, duplex or offset without changing which of
// the two mode groups the channel is in) preserves P14 exactly as it
// preserves P10/P11/P15; a family-crossing write is refused, named, rather
// than silently reinterpreted.
const (
	// registerCreate is A3's TS-2000 restatement (ts890/ts990 precedent):
	// this programme does not create channels. This document's own MR/MW
	// block prints no statement either way about writing an unassigned
	// channel (unlike the ts890/ts990 "MA0" family, whose siblings print an
	// explicit prohibition); the standing no-create rule applies regardless.
	registerCreate = "ts2000-write-existing (no create)"
	// registerP14Family is the ONE new refusal this row's write path adds
	// beyond the family's usual ladder: P14's mode-conditional legend
	// (ts2000:11508-11521) means a preserved step index cannot be carried
	// across the SSB/CW/FSK <-> AM/FM boundary.
	registerP14Family = "ts2000-capability-matrix.md §2, P14 (mode-conditional step legend)"
)

// RefusalError is a write refusal naming one of the two registers above.
type RefusalError struct {
	driver.WriteRefusedError
	Register string
}

// Unwrap exposes the fleet refusal, which in turn unwraps to
// driver.ErrWriteRefused.
func (e *RefusalError) Unwrap() error { return &e.WriteRefusedError }

// refuse builds a *RefusalError, the ts890/ts990 shape.
func refuse(slot, register string, fields []spec.Field, format string, args ...any) *RefusalError {
	return &RefusalError{
		WriteRefusedError: driver.WriteRefusedError{Slot: slot, Fields: fields, Reason: register + ": " + fmt.Sprintf(format, args...)},
		Register:          register,
	}
}

// requestedFieldRules pairs each spec.Field with the predicate that reports
// whether a write of this channel actually requests it — the
// core/driver/ts590 shape, widened for this row's own nine unconditional
// fields (this row's P12/P13 lift adds FieldDuplex/FieldOffset to the
// family's frequency/mode/tag/scan_skip/tone_mode, and this document's own
// tone chart adds tone_tx/tone_rx as unconditional, matching ts590's own
// tone_tx/tone_rx treatment rather than the TS-480's conditional one).
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
	// spec.FieldErase is deliberately absent; WriteChannel refuses an empty
	// channel a rung above this table.
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, always},
	{spec.FieldOffset, always},
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

// requestedFields lists, in requestedFieldRules' order, the spec.Fields
// this channel's write requests.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := make([]spec.Field, 0, len(requestedFieldRules))
	for _, r := range requestedFieldRules {
		if r.present(data) {
			fields = append(fields, r.field)
		}
	}
	return fields
}

// mwSetSpec is the transport spec for the MW Set: fire-and-forget, the
// engine's own bounded "?;" rejection window and nothing else.
func mwSetSpec() transport.CommandSpec { return transport.CommandSpec{Class: transport.ClassWrite} }

// send writes cmd and types the two wire events a Kenwood exchange can
// meet, through the same wireFailure the probe and read path use.
func (s *Session) send(ctx context.Context, cmd kw.Command) error {
	if _, err := s.eng.Do(ctx, cmd, mwSetSpec()); err != nil {
		return wireFailure(s.layout, "MW", err)
	}
	return nil
}

// readCurrent is the write ladder's ONE read — the same three steps
// ReadChannel takes (BuildMRRead, the exchange, ParseMRAnswer), performed
// here rather than by calling ReadChannel, which takes opMu and would
// deadlock.
func (s *Session) readCurrent(ctx context.Context, slotID string, slot kw.Slot) (kw.Record, error) {
	cmd, err := s.layout.BuildMRRead(slot)
	if err != nil {
		return kw.Record{}, fmt.Errorf("ts2000: WriteChannel %s: %w", slotID, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.mrSpec(slot))
	if err != nil {
		return kw.Record{}, fmt.Errorf("ts2000: WriteChannel %s: the pre-write read: %w", slotID, wireFailure(s.layout, "MR", err))
	}
	rec, err := s.layout.ParseMRAnswer(frame)
	if err != nil {
		return kw.Record{}, fmt.Errorf("ts2000: WriteChannel %s: the pre-write read: %w", slotID, err)
	}
	if got := rec.Slot.String(); got != slotID {
		return kw.Record{}, &AnswerMismatchError{Model: "ts2000", Requested: slotID, Answered: got}
	}
	return rec, nil
}

// mandatoryFieldRefusal is the refusal a live, always-transmitted byte
// earns when the channel has no Known value for it — the core/driver/ts590
// shape.
func mandatoryFieldRefusal(slotID string, field spec.Field, state codeplug.FieldState, position string) *driver.WriteRefusedError {
	return &driver.WriteRefusedError{
		Slot:   slotID,
		Fields: []spec.Field{field},
		Reason: fmt.Sprintf("%s FieldState is %q, not %q, and %s is transmitted on every write with no \"leave it alone\" encoding — so a non-Known value here could only be manufactured, which is what a state meaning \"preserve whatever the radio has\" forbids", field, state, codeplug.Known, position),
	}
}

// boolWire renders a neutral flag as the '0'/'1' this record prints for
// every two-valued byte.
func boolWire(on bool) byte {
	if on {
		return '1'
	}
	return '0'
}

// modeWire is caps.go's modeNames read backwards: a published mode NAME to
// the wire nibble that produces it — a search over the one naming rule
// core/kw's own ModeName already states, not a second table (the
// core/driver/ts890 modeWire shape).
func modeWire(l kw.Layout, name string) (kw.Mode, bool) {
	for b := 0; b <= 0xFF; b++ {
		if got, ok := l.ModeName(kw.Mode(byte(b))); ok && got == name {
			return kw.Mode(byte(b)), true
		}
	}
	return 0, false
}

// toneModeWire is read.go's toneModeNames read backwards, derived rather
// than transcribed so the two cannot drift apart.
var toneModeWire = invertToneModes()

func invertToneModes() map[string]kw.ToneMode {
	m := make(map[string]kw.ToneMode, len(toneModeNames))
	for wire, name := range toneModeNames {
		m[name] = wire
	}
	return m
}

// toneIndex is the position of tone in the 39-entry chart this row
// publishes (caps.go's kenwoodTS2000CTCSSTones), which IS the CAT tone
// number: spec.Validate requires CTCSSTones to be strictly ascending
// precisely so the slice index doubles as the wire value.
func toneIndex(tone spec.Tone) (int, bool) {
	for i, t := range kenwoodTS2000CTCSSTones {
		if t == tone {
			return i, true
		}
	}
	return 0, false
}

// duplexWire is caps.go's duplexOptions read backwards. THE WIRE DIGIT IS
// THE VOCABULARY VALUE (caps.go: Value "0"/"1"/"2" literally spells the
// record's own P12 byte), so this is a domain check rather than a
// translation table.
func duplexWire(value string) (byte, bool) {
	if len(value) != 1 {
		return 0, false
	}
	switch value[0] {
	case '0', '1', '2':
		return value[0], true
	default:
		return 0, false
	}
}

// stepFamily is which of P14's two mode-conditional legends a mode falls
// under (ts2000:11508-11521). It is deliberately NOT a step-value type: it
// classifies a MODE, never a step index, and exists only to guard
// preservation — see this file's own doc comment.
type stepFamily int

const (
	stepFamilyUnknown stepFamily = iota
	stepFamilySSBCWFSK
	stepFamilyAMFM
)

// modeStepFamily classifies m. The default (unknown) arm is unreachable in
// practice — every Mode that can reach here already round-tripped through
// this row's own eight-name legend — and is kept as a fail-closed branch
// rather than a panic.
func modeStepFamily(m kw.Mode) stepFamily {
	switch m {
	case kw.ModeLSB, kw.ModeUSB, kw.ModeCW, kw.ModeFSK, kw.ModeCWR, kw.ModeFSKR:
		return stepFamilySSBCWFSK
	case kw.ModeFM, kw.ModeAM:
		return stepFamilyAMFM
	default:
		return stepFamilyUnknown
	}
}

// candidate maps a populated channel onto the fields this row's record
// actually has a home for. P10 (DCS code), P11 (REVERSE), P14 (the
// tuning-step index) and P15 (Memory Group) are DELIBERATELY LEFT ZERO
// here — WriteChannel patches them in from the pre-write read once it has
// one, which is what lets this row write channels no spec.Field models
// every one of its own bytes for. Called only after the whole ladder above
// candidate's call site has passed.
func (s *Session) candidate(slotID string, slot kw.Slot, data codeplug.ChannelData) (kw.Record, error) {
	for _, m := range []struct {
		field    spec.Field
		state    codeplug.FieldState
		position string
	}{
		{spec.FieldScanSkip, data.ScanSkip.State, "P6 at byte 19, the lockout (ts2000:10704)"},
		{spec.FieldToneMode, data.ToneMode.State, "P7 at byte 20, the tone mode (ts2000:10706-10708)"},
		{spec.FieldToneTx, data.ToneTx.State, "P8 at bytes 21-22, the TN index (ts2000:10709-10710)"},
		{spec.FieldToneRx, data.ToneRx.State, "P9 at bytes 23-24, the CN index (ts2000:10711)"},
		{spec.FieldDuplex, data.Duplex.State, "P12 at byte 29, the shift status (ts2000:10715-10716)"},
		{spec.FieldOffset, data.OffsetHz.State, "P13 at bytes 30-38, the offset frequency (ts2000:10717-10718)"},
	} {
		if m.state != codeplug.Known {
			return kw.Record{}, mandatoryFieldRefusal(slotID, m.field, m.state, m.position)
		}
	}

	mode, ok := modeWire(s.layout, data.Mode)
	if !ok {
		return kw.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not one this row publishes", data.Mode),
		}
	}
	toneMode, ok := toneModeWire[data.ToneMode.Value]
	if !ok {
		// Unreachable after CheckFieldStates, which judges a Known tone
		// mode against this session's ToneModes vocabulary.
		return kw.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneMode},
			Reason: fmt.Sprintf("tone mode %q is not one this row publishes", data.ToneMode.Value),
		}
	}
	toneTx, ok := toneIndex(data.ToneTx.Value)
	if !ok {
		return kw.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneTx},
			Reason: fmt.Sprintf("tone_tx %v is not in the 39-entry chart this row publishes (ts2000:3837-3849)", data.ToneTx.Value),
		}
	}
	toneRx, ok := toneIndex(data.ToneRx.Value)
	if !ok {
		return kw.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneRx},
			Reason: fmt.Sprintf("tone_rx %v is not in the 39-entry chart this row publishes (ts2000:3837-3849)", data.ToneRx.Value),
		}
	}
	if toneRx == len(kenwoodTS2000CTCSSTones)-1 {
		// THE 590 PAIR'S OWN M-E1 ERRATUM, RESTATED FOR THIS CHART
		// (caps.go's kenwoodTS2000CTCSSTones doc comment): TN's (P8, tone_tx)
		// legend prints "01~39" and CN's (P9, tone_rx) prints only "01~38"
		// — one entry narrower — so the chart's own 39th value, 1750 Hz, is
		// TN-only. Unlike the 590 pair this is enforced here rather than
		// merely noted, because this row's writes are no longer refused
		// unconditionally.
		return kw.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneRx},
			Reason: "tone_rx 1750 Hz is the chart's 39th entry, and CN's own printed domain stops at the 38th (\"01~38\", matrix §4 M-E1 restated) — a receive tone this radio cannot be told to listen for is not written",
		}
	}
	shift, ok := duplexWire(data.Duplex.Value)
	if !ok {
		// Unreachable after CheckFieldStates for the same reason toneMode's
		// is: a Known duplex is judged against caps.DuplexOptions first.
		return kw.Record{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldDuplex},
			Reason: fmt.Sprintf("duplex %q is not one this row publishes", data.Duplex.Value),
		}
	}

	return kw.Record{
		Slot:       slot,
		FreqHz:     data.FreqHz,
		Mode:       mode,
		Byte19:     boolWire(data.ScanSkip.Value),
		ToneMode:   toneMode,
		ToneIndex:  toneTx,
		CTCSSIndex: toneRx,
		Shift:      shift,
		OffsetHz:   data.OffsetHz.Value,
		Name:       data.Tag,
		// AnswerP1 is a parser output and is deliberately left zero: the
		// builder derives P1 from the slot's own class (M9, kw.Slot.P1).
	}, nil
}

// WriteChannel implements driver.Session: ONE MR pre-write read, then ONE
// MW Set — the core/driver/ts890/ts990 read-then-write shape, adopted here
// because this row's own P10/P11/P14/P15 have no spec.Field a candidate
// channel could carry a value for (this file's own doc comment has the
// full byte-level reasoning, including why P14 is the one of the four that
// is not ALWAYS safe to preserve).
//
// THE LADDER, EVERY RUNG PRE-WIRE UP TO THE READ:
//
//	parseSlotID          the identifier's syntax, and no radio (read.go)
//	bankFor              membership in THIS session's published bank
//	the erase            an empty channel, refused (no MR/MW-reachable
//	                     erase primitive exists either)
//	candidate            the fleet's mandatory-Known checks plus this row's
//	                     own vocabulary lookups — before ANY wire traffic
//	CheckFieldStates     the fleet's FieldState walk
//	THE CAPABILITY GATE  defence in depth below the clone service
//
// THEN, AFTER candidate BUT BEFORE THE SET, the one read and its two
// checks:
//
//	registerCreate       the pre-write read shows this slot unassigned —
//	                     this programme does not create channels
//	registerP14Family     the pre-write read's mode and the write's mode
//	                     fall in different P14 legend families
//
// SENT, NEVER CONFIRMED (the family's standing assumption): a "?;" is a
// REJECTION (this document's own error table, ts2000:9603-9618); an
// ACCEPTED Set draws nothing at all, and no TS-2000/2000X/B2000 has ever
// been asked anything, so silence is inconclusive rather than confirmed.
//
// THE OPERATION MUTEX IS HELD FOR THE WHOLE CALL, including the pre-write
// read: the write is TWO exchanges and a concurrent operation landing
// between them would decide against one radio state and write against
// another.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	number, half, err := parseSlotID(ch.Slot)
	if err != nil {
		return res, &UnknownSlotError{Slot: ch.Slot, Model: s.p.name, Reason: err.Error()}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &UnknownSlotError{
			Slot: ch.Slot, Model: s.p.name,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}
	slot, err := s.layout.NewSlot(number, half)
	if err != nil {
		return res, &UnknownSlotError{Slot: ch.Slot, Model: s.p.name, Reason: err.Error()}
	}

	if ch.Empty() {
		// AN EMPTY CHANNEL IS AN ERASE REQUEST, AND IT IS REFUSED: this
		// document prints no MR/MW-reachable erase primitive (matrix §4,
		// FieldErase), the fleet-wide write posture (spec §3). The rung
		// stays ahead of every field check structurally: an empty channel
		// has no Data at all.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this radio's PC-command reference documents no MR/MW-reachable erase route (matrix §4) — FieldErase is not write-Supported on this row's banks, and the standing no-erase rule needs no exception here",
		}
	}
	data := *ch.Data

	want, err := s.candidate(ch.Slot, slot, data)
	if err != nil {
		return res, err
	}

	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

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
			Reason: "not write-Supported for this session",
		}
	}

	// THE ONE READ. Everything above this line is settled without wire
	// traffic; everything from here on needs the radio's own current
	// answer for this slot.
	current, err := s.readCurrent(ctx, ch.Slot, slot)
	if err != nil {
		return res, err
	}

	if current.Empty {
		return res, refuse(ch.Slot, registerCreate, nil,
			"the pre-write read shows this channel unassigned now — this programme does not create channels, only writes an existing one")
	}

	wantFamily, curFamily := modeStepFamily(want.Mode), modeStepFamily(current.Mode)
	if wantFamily == stepFamilyUnknown || curFamily == stepFamilyUnknown || wantFamily != curFamily {
		return res, refuse(ch.Slot, registerP14Family, []spec.Field{spec.FieldMode},
			"the pre-write read's mode and this write's mode fall in different P14 legend families (\"SSB/CW/FSK mode: 00~03\" against \"AM/FM mode: 00~09\", ts2000:11508-11521): P14 (bytes 39-40) is preserved VERBATIM from the current record, and carrying it across the family boundary would silently reinterpret the same two digits as a different step size with nothing in this write asking for that change")
	}

	// PRESERVE: the four raw values this milestone models no spec.Field
	// for, copied unread and unmodeled from the SAME channel's own current
	// state (this file's own doc comment).
	want.DCSCode = current.DCSCode
	want.Byte28 = current.Byte28
	want.Byte3940 = current.Byte3940
	want.Byte41 = current.Byte41

	cmd, err := s.layout.BuildMWSet(want)
	if err != nil {
		return res, fmt.Errorf("ts2000: WriteChannel %s: %w: %w", ch.Slot, driver.ErrWriteRefused, err)
	}

	res.Steps = []driver.WriteStep{{Command: "MW"}}
	if derr := s.send(ctx, cmd); derr != nil {
		res.Steps[0].Sent = errors.Is(derr, transport.ErrRejected)
		return res, fmt.Errorf("ts2000: WriteChannel %s: %w", ch.Slot, derr)
	}
	res.Steps[0].Sent = true
	return res, nil
}

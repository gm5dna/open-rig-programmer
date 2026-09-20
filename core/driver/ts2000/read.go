// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// toneModeNames maps the record's P7 wire byte to the neutral tone_mode
// vocabulary this row publishes (matrix §4). THREE ENTRIES, not four: P7=3
// means DCS on this row rather than Cross Tone (matrix §6 item 1), and this
// package publishes no fourth ToneMode value to name it — a real answer of
// '3' is refused below rather than mislabelled "cross tone".
var toneModeNames = map[kw.ToneMode]string{
	kw.ToneModeOff:   "OFF",
	kw.ToneModeTone:  "TONE",
	kw.ToneModeCTCSS: "CTCSS",
}

// ErrUnknownSlot is the sentinel a caller compares against (via errors.Is)
// when a slot identifier is not one this row publishes.
var ErrUnknownSlot = driver.ErrUnknownSlot

// UnknownSlotError reports a slot identifier that is not in this row's
// bank — malformed or outside the printed space. No frame is sent. The
// fields are driver.UnknownSlotError's shared shape (core/driver/errors.go);
// this package keeps its own Error() because the message's "ts2000:"
// prefix is a package literal, not a field.
type UnknownSlotError struct {
	driver.UnknownSlotError
}

func (e *UnknownSlotError) Error() string {
	return fmt.Sprintf("ts2000: slot %q is not published by the %s: %s", e.Slot, e.Model, e.Reason)
}
func (e *UnknownSlotError) Unwrap() error { return ErrUnknownSlot }

// ErrAnswerMismatch is the sentinel a caller compares against when a
// slot-addressed answer does not name what the read asked for.
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports the requested and the answered channel.
type AnswerMismatchError = driver.AnswerMismatchError[string]

// AnswerP1MismatchError reports an MR answer whose P1 names a different
// half of the addressing than the read asked for — the core/driver/ts590
// shape (this row, unlike the TS-480, has a real SlotScan class whose two
// halves this error distinguishes).
type AnswerP1MismatchError struct {
	Slot      string
	Requested byte
	Answered  byte
}

func (e *AnswerP1MismatchError) Error() string {
	return fmt.Sprintf("ts2000: slot %q was read with P1=%q but the answer carries P1=%q", e.Slot, e.Requested, e.Answered)
}
func (e *AnswerP1MismatchError) Unwrap() error { return ErrAnswerMismatch }

// parseSlotID reads a canonical slot identifier as a channel number and
// half: three digits, with an optional "L"/"U" suffix for a Program Scan
// channel — kw.Slot.String()'s own rendering, parsed back (matrix §4,
// Banks; this row's MC prints a real hundreds digit, unlike the TS-480's).
func parseSlotID(id string) (int, kw.ScanHalf, error) {
	half := kw.ScanHalfNone
	digits := id
	if strings.HasSuffix(id, "L") {
		half, digits = kw.ScanLower, strings.TrimSuffix(id, "L")
	} else if strings.HasSuffix(id, "U") {
		half, digits = kw.ScanUpper, strings.TrimSuffix(id, "U")
	}
	if len(digits) != 3 {
		return 0, 0, fmt.Errorf("slot %q: a slot identifier on this row is three digits, with an optional \"L\"/\"U\" suffix for a Program Scan channel", id)
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, 0, fmt.Errorf("slot %q: the channel number is three decimal digits", id)
	}
	return n, half, nil
}

// bankFor returns the bank of THIS SESSION's published capabilities that
// holds id, and whether any does.
func (s *Session) bankFor(id string) (spec.Bank, bool) {
	bankID, ok := s.caps.BankOf(id)
	if !ok {
		return spec.Bank{}, false
	}
	return s.caps.Bank(bankID)
}

// bankNames renders this session's bank inventory for a refusal.
func (s *Session) bankNames() string {
	parts := make([]string, 0, len(s.caps.Banks))
	for _, b := range s.caps.Banks {
		if len(b.Slots) == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s-%s", b.ID, b.Slots[0], b.Slots[len(b.Slots)-1]))
	}
	return strings.Join(parts, ", ")
}

// mrSpec is the transport spec for one MR read of slot — kw.Layout's own
// MRAnswerMatcher, and one retry (core/driver/ts480's own reasoning: every
// memory answer is fifty bytes and starts "MR" with the channel number at
// P2/P3, so no prefix separates one channel's answer from another's).
func (s *Session) mrSpec(slot kw.Slot) transport.CommandSpec {
	return s.newReadSpec(s.layout.MRAnswerMatcher(slot), 1)
}

// ReadChannel implements driver.Session: one MR frame per slot.
//
//   - AN UNKNOWN SLOT is refused before any frame is built.
//   - A "?;" IS A DEFINITIVE REJECTION (kw.RejectionError); a TIMEOUT is
//     kw.TimeoutError, which says in as many words that it is not an
//     inference of absence — this document's own error table
//     (ts2000:9603-9618) prints the same "may not appear due to
//     microprocessor transients" warning every book in the family does.
//   - AN EMPTY CHANNEL: this document prints no "P4~P16 will be 0/blank"
//     sentence the way the 590 pair's does (matrix §5 — CANNOT ESTABLISH),
//     so the structural empty-channel test core/kw applies is exercised
//     here on the SAME UNLIFTED-ASSUMPTION footing core/driver/ts480's own
//     A4 states, not on a citation this document supplies.
func (s *Session) ReadChannel(ctx context.Context, id string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// BankSatellite's slots ("0".."9") are not the three-digit shape
	// parseSlotID expects, so dispatch on bank membership BEFORE it —
	// satellite.go's own doc comment has the read shape.
	if bank, ok := s.bankFor(id); ok && bank.ID == spec.BankSatellite {
		return s.readSatelliteChannel(ctx, id)
	}

	number, half, err := parseSlotID(id)
	if err != nil {
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{Slot: id, Model: s.p.name, Reason: err.Error()}}
	}
	bank, ok := s.bankFor(id)
	if !ok {
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{
			Slot: id, Model: s.p.name,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}}
	}
	slot, err := s.layout.NewSlot(number, half)
	if err != nil {
		// Unreachable for a slot the bank published; see core/driver/ts480's
		// own ReadChannel for why this is a refusal rather than a panic.
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{Slot: id, Model: s.p.name, Reason: err.Error()}}
	}

	cmd, err := s.layout.BuildMRRead(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts2000: ReadChannel %s: %w", id, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.mrSpec(slot))
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts2000: ReadChannel %s: %w", id, wireFailure(s.layout, "MR", err))
	}

	rec, err := s.layout.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts2000: ReadChannel %s: %w", id, err)
	}
	if got := rec.Slot.String(); got != id {
		return codeplug.Channel{}, &AnswerMismatchError{Model: "ts2000", Requested: id, Answered: got}
	}
	if want := slot.P1(); rec.AnswerP1 != want {
		return codeplug.Channel{}, &AnswerP1MismatchError{Slot: id, Requested: want, Answered: rec.AnswerP1}
	}
	if rec.Empty {
		return codeplug.Channel{Slot: id}, nil
	}

	data, err := s.channelData(rec, bank)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts2000: ReadChannel %s: %w", id, err)
	}
	return codeplug.Channel{Slot: id, Data: data}, nil
}

// channelData maps one parsed 50-byte record onto the neutral channel
// model.
//
// FIVE FIELDS THIS RECORD EXPRESSES THAT THE REGISTERED ROWS DO NOT
// (matrix §2's five live axes): DCS code (P10) has no honest FieldDTCSCode
// this milestone — the 104-entry chart is cited, not transcribed (matrix §6
// item 4) — so it is parsed and NOT published, exactly the shape
// core/driver/ts480 gives its own unpublished tone indices. REVERSE (P11)
// and Memory Group (P15) have NO spec.Field to publish to at all (matrix
// §2: "UNMAPPED-with-reason") and are likewise parsed and discarded here.
// Shift (P12) and Offset (P13) DO have a home — FieldDuplex and
// FieldOffset, the Icom half (design decision 6) — and are published
// below.
func (s *Session) channelData(rec kw.Record, bank spec.Bank) (*codeplug.ChannelData, error) {
	mode, ok := s.layout.RecordModeName(rec)
	if !ok {
		return nil, fmt.Errorf("unmapped mode nibble %v", rec.Mode)
	}
	toneMode, ok := toneModeNames[rec.ToneMode]
	if !ok {
		// rec.ToneMode == kw.ToneModeCross ('3') is the only way here: this
		// row's ToneModesFour axis admits it (width/count match the 590
		// pair's), but it means DCS on this row and this package publishes
		// no fourth ToneMode value for it (matrix §6 item 1) — refusing
		// rather than reporting it as "cross tone" is the honest reading.
		return nil, fmt.Errorf("ts2000: P7 is %q (DCS on this row, matrix §6 item 1) — no tone_mode value is published for it this milestone", rec.ToneMode.Wire())
	}
	if !bank.Fields[spec.FieldToneMode].Unreachable() && !toneModeAdmitted(s.caps, toneMode) {
		return nil, fmt.Errorf("tone mode %q is not one this row's capability table publishes", toneMode)
	}

	toneTx, err := s.tone(rec.ToneIndex, "P8, the TN index")
	if err != nil {
		return nil, err
	}
	toneRx, err := s.tone(rec.CTCSSIndex, "P9, the CN index")
	if err != nil {
		return nil, err
	}

	duplex, err := s.duplex(rec.Shift)
	if err != nil {
		return nil, err
	}

	return &codeplug.ChannelData{
		FreqHz: rec.FreqHz,
		Mode:   mode,

		// No clarifier position anywhere in the 47-byte account.
		ClarHz: 0, RxClar: false, TxClar: false,
		// The Yaesu half of the vocabulary pair; this row declares the
		// Icom half instead.
		CTCSS: "", Shift: "",

		Tag: rec.Name,
		// No tag-display flag anywhere in the record.
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// P6, byte 19: the lockout (ts2000:10704).
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: rec.Byte19 == '1'},

		// The Icom half: no stored second frequency, the shift-and-offset
		// shape instead. TxFreqHz is Unsupported on the whole row —
		// P1's own RX/TX selector is a SlotScan-half concern (Slot.P1),
		// never a distinct stored transmit frequency this driver reads.
		TxFreqHz: codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:   duplex,
		OffsetHz: codeplug.FreqField{State: codeplug.Known, Value: rec.OffsetHz},

		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneMode},
		ToneTx:   toneTx,
		ToneRx:   toneRx,

		// DCS (P10) is parsed by the codec and NOT published this
		// milestone — see this method's own doc comment.
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		// REVERSE (P11) has no spec.Field to publish to at all.
		Filter: codeplug.StringField{State: codeplug.Unavailable},
		// Byte 19 is the lockout, published above as scan_skip; there is
		// no data-mode position in this record at all.
		DataMode: codeplug.BoolField{State: codeplug.Unavailable},
		// Bytes 39-40 ARE a step here too (ts2000:10720-10722), excluded
		// this wave (spec §6 Q4) exactly as write.go's refusal states.
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		// Additions design D8 — none applies to this wave (matrix §4).
		AttenuatorDB: codeplug.IntField{State: codeplug.Unavailable},
		Preamp:       codeplug.StringField{State: codeplug.Unavailable},
		Antenna:      codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:       codeplug.BoolField{State: codeplug.Unavailable},
		SatBandSwap:  codeplug.BoolField{State: codeplug.Unavailable},
		SatTrace:     codeplug.BoolField{State: codeplug.Unavailable},
		SatTraceRev:  codeplug.BoolField{State: codeplug.Unavailable},
	}, nil
}

// tone maps one printed chart index onto the neutral tone field — the
// core/driver/ts590 shape, over THIS document's own 39-entry chart
// (caps.go's kenwoodTS2000CTCSSTones) rather than the 590 pair's 43-entry
// one.
func (s *Session) tone(index int, field string) (codeplug.ToneField, error) {
	if index < 0 || index >= len(kenwoodTS2000CTCSSTones) {
		return codeplug.ToneField{}, fmt.Errorf("%s is %d, outside the 39-entry chart this row publishes (ts2000:3837-3847)", field, index)
	}
	value := kenwoodTS2000CTCSSTones[index]
	if !s.caps.AdmitsTone(value) {
		return codeplug.ToneField{}, fmt.Errorf("%s is %d, which is %v — a tone this row's capability table does not admit", field, index, value)
	}
	return codeplug.ToneField{State: codeplug.Known, Value: value}, nil
}

// duplex maps P12's wire byte onto FieldDuplex — the wire digit IS the
// vocabulary value (caps.go's duplexOptions), so no translation table is
// needed beyond the domain check.
func (s *Session) duplex(shift byte) (codeplug.StringField, error) {
	value := string(shift)
	if !duplexAdmitted(s.caps, value) {
		// Shift == '3' ("All E-types") is the only way here: core/kw's
		// P12ShiftLive axis admits it (checkP12: '0'..'3'), and this
		// package publishes no fourth DuplexOption for it (matrix §6 item
		// 2) — no document in this milestone's evidence names an
		// E-suffixed TS-2000/2000X/B2000, so inventing a direction for it
		// would be a claim this document does not support.
		return codeplug.StringField{}, fmt.Errorf("ts2000: P12 is %q (\"All E-types\", matrix §6 item 2) — no duplex value is published for it this milestone", shift)
	}
	return codeplug.StringField{State: codeplug.Known, Value: value}, nil
}

// toneModeAdmitted reports whether value is one of the tone-mode strings
// caps publishes.
func toneModeAdmitted(caps spec.Capabilities, value string) bool {
	for _, tm := range caps.ToneModes {
		if tm.Value == value {
			return true
		}
	}
	return false
}

// duplexAdmitted reports whether value is one of the duplex strings caps
// publishes.
func duplexAdmitted(caps spec.Capabilities, value string) bool {
	for _, d := range caps.DuplexOptions {
		if d.Value == value {
			return true
		}
	}
	return false
}

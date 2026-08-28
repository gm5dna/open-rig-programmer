// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic705 "github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// rawRecord is ONE answered memory read, kept WHOLE.
//
// Every one of the four fields has a consumer that the other three cannot
// serve, which is why the seam returns a record rather than a byte slice:
//
//   - frame, because the only landed typed decoder,
//     civ.Profile.ParseMemoryAnswer, takes a COMPLETE CI-V frame. A seam
//     that returned the stripped record could not call it at all.
//   - addr, because ruling T2 requires the DRIVER to check the answered
//     address: the landed memory-answer matcher is deliberately
//     envelope-only, so an answer for the wrong channel matches the read
//     in flight.
//   - record, the raw 111 bytes, because the write path's E6 preservation
//     check compares the radio's own unmapped areas against the profile's
//     Fixed template — a comparison that must see bytes, not decoded
//     fields, since no spec.Field claims those areas at all.
//   - empty, because "this slot is unwritten" is a DRIVER judgement on
//     this model's evidence (D5 entry 2), made before any parser runs.
type rawRecord struct {
	frame  []byte
	addr   civ.ChannelAddress
	record []byte
	empty  bool
}

// readRaw performs one memory read and returns it whole.
//
// THE ANSWER-ADDRESS EQUALITY CHECK IS ITS FIRST ACT (ruling T2). The
// landed MemoryAnswerMatcher matches the ENVELOPE — to this controller,
// from this radio, carrying 1A 00 — and deliberately not the channel, so
// an answer about some other channel satisfies the outstanding read. The
// decoded address is therefore compared against want BEFORE ANY OTHER USE:
// before empty recognition, before record mapping, before the write path's
// template check, before any merge. A mismatch is a typed error naming
// both addresses and a counter on the model surface; nothing downstream
// ever sees a mismatched answer.
//
// (The decode itself necessarily precedes the comparison — the address
// arrives inside the answer, and civ.MemoryAnswerRecord is what reads it
// out, checking the record length in the same call. Decoding is not USING:
// nothing derived from those bytes escapes this function until the
// addresses have been compared.)
//
// AN FA IS AN ERROR, NEVER A FRAME (ruling T4). Engine.Do consumes the NAK
// and returns transport.ErrRejected with no frame at all, so this branch —
// and every other read path in this driver — keys on the sentinel. No code
// here may test for "an FA frame" after Do.
func (s *Session) readRaw(ctx context.Context, want civ.ChannelAddress) (rawRecord, error) {
	p := civic705.Profile()
	cmd, err := p.BuildMemoryRead(want)
	if err != nil {
		return rawRecord{}, fmt.Errorf("ic705: read %v: %w", want, err)
	}
	frame, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(p.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		// The unwritten channel (D5 entry 2(a), ASSUMED, lift
		// L-EMPTY-FA). There is no frame to carry.
		return rawRecord{addr: want, empty: true}, nil
	}
	if err != nil {
		return rawRecord{}, fmt.Errorf("ic705: read %v: %w", want, err)
	}
	got, record, err := p.MemoryAnswerRecord(frame)
	if err != nil {
		// A record at a length this profile does not declare surfaces
		// here as civ's own *RecordLengthError — which is the LENGTH
		// FINGERPRINT being continuous (spec D3.2). It is deliberately
		// NOT a wrong-radio refusal at this point: the probe already
		// decided which radio this is, and a mid-session length surprise
		// is a read that failed, not a radio that changed model.
		return rawRecord{}, fmt.Errorf("ic705: read %v: %w", want, err)
	}
	if got != want {
		s.mismatches.Add(1)
		return rawRecord{}, &AnswerMismatchError{Requested: want, Answered: got}
	}
	if allFF(record) {
		// The other unverified empty shape (D5 entry 2(b), lift
		// L-EMPTY-FF). Recognised on the RAW bytes, because 0xFF fails
		// the enum decode: testing for it after parsing would be too
		// late, and this is precisely why the raw route exists.
		return rawRecord{frame: frame, addr: got, record: record, empty: true}, nil
	}
	return rawRecord{frame: frame, addr: got, record: record}, nil
}

// ReadChannel implements driver.Session: one memory slot, read and mapped
// onto the neutral channel model.
//
// AN EMPTY SLOT IS AN EMPTY CHANNEL, NEVER AN ERROR (R10, and
// core/driver's own contract): both unverified empty shapes — the FA and
// the all-0xFF record — return codeplug.Channel{Slot: slot} with nil Data,
// so a whole-radio read walks past an unwritten channel instead of
// aborting on it. Every OTHER failure is a typed error, including a record
// at a length this profile does not declare.
//
// ReadAll is NOT a method here, deliberately: core/clone loops over
// Capabilities().Banks[].Slots, which for the sparse memory bank is
// whatever Open's inventory walk materialised (inventory.go).
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	addr, _, err := slotToAddress(slot)
	if err != nil {
		return codeplug.Channel{}, err
	}
	raw, err := s.readRaw(ctx, addr)
	if err != nil {
		return codeplug.Channel{}, err
	}
	if raw.empty {
		return codeplug.Channel{Slot: slot}, nil
	}
	rec, err := civic705.Profile().ParseMemoryAnswer(raw.frame)
	if err != nil {
		// The duplicated TX block's copies disagreeing lands here (spec
		// D5 entry 4). It is a genuine cost, stated in doc.go: a channel
		// the radio itself wrote with Split ON and a differing TX-side
		// field fails to parse, so a whole-radio read aborts on it. The
		// alternative — silently preferring one copy — would be a guess
		// about which one the radio honours.
		return codeplug.Channel{}, fmt.Errorf("ic705: read %s: %w", slot, err)
	}
	data, err := channelDataFrom(rec, s.caps)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic705: read %s: %w", slot, err)
	}
	return codeplug.Channel{Slot: slot, Data: &data}, nil
}

// channelDataFrom maps one decoded record onto the neutral channel model.
//
// A READ NEVER CONSTRUCTS A Known VALUE Valid WOULD REFUSE. Every mapped
// field decodes to Known except four cases where the wire value sits
// outside the domain the write path (and codeplug.Valid) enforces:
//
//   - a TONE outside the declared domain (ruling T1(3)). That is the wire
//     zero a tone-mode-OFF channel carries: spec.Capabilities.AdmitsTone(0)
//     is false under any legal CTCSSToneRange, because spec.Validate
//     refuses a MinDeciHz of zero outright, so a Known zero would be a
//     value the codeplug layer itself refuses. The read therefore reports
//     Unknown — and the write path does NOT then refuse the channel: it
//     copies the just-read record's own tone number back out verbatim
//     (T1(4), write.go), which is preservation of the radio's value rather
//     than synthesis of a new one.
//   - a DTCS CODE the 512-entry octal-digit table (spec.Capabilities.
//     DTCSCodes) does not hold. A packed-BCD nibble decodes any digit
//     0-9, but the table is built from octal digits 0-7 only (caps.go's
//     dtcsCodes), so a decoded value such as 888 — a real nibble pattern,
//     not a corrupt one — is outside the vocabulary the table holds. The
//     read reports Unknown rather than a Known value Valid would refuse.
//   - TxFreqHz / OffsetHz at or beyond the printed digit-leader ceilings
//     write.go's rung 7 enforces (maxStorableFreqHz, maxOffsetHz): decoded
//     to Unknown, mirroring the tone precedent, because both carry a
//     FieldState to fall back to.
//   - plain FreqHz (the RX frequency) at or beyond maxStorableFreqHz: it
//     carries no FieldState to fall back to, so an out-of-domain value is
//     reported as a decode error naming the field and the value, not a
//     Known frequency the write path could never accept back.
func channelDataFrom(rec civ.MemoryRecord, caps spec.Capabilities) (codeplug.ChannelData, error) {
	var d codeplug.ChannelData

	num := func(name string, o civ.Optional[uint64]) (uint64, error) {
		v, ok := o.Get()
		if !ok {
			// Unreachable for a record civ decoded against this layout —
			// every field below is mapped by a span, and
			// validateRecordFields refuses a record missing one. Checked
			// anyway, because the alternative to a named error is a
			// silent zero: a frequency of 0 Hz, or a tone this driver
			// then "preserves".
			return 0, fmt.Errorf("the decoded record carries no %s", name)
		}
		return v, nil
	}
	text := func(name string, o civ.Optional[string]) (string, error) {
		v, ok := o.Get()
		if !ok {
			return "", fmt.Errorf("the decoded record carries no %s", name)
		}
		return v, nil
	}

	rx, err := num("rx frequency", rec.RXFreqHz)
	if err != nil {
		return d, err
	}
	if rx >= maxStorableFreqHz {
		// FreqHz carries no FieldState, so there is no Unknown to fall
		// back to (write.go's rung 7 mirror): a read never constructs a
		// Known value Valid would refuse, and here the only honest report
		// is a decode error naming the field and the value.
		return d, fmt.Errorf("ic705: decoded rx frequency %d Hz is at or above %d Hz, which this radio's printed digit leaders cannot express", rx, maxStorableFreqHz)
	}
	d.FreqHz = rx

	if d.Mode, err = text("mode", rec.Mode); err != nil {
		return d, err
	}
	// The trailing pad is already trimmed by civ's name decoder.
	if d.Tag, err = text("name", rec.Name); err != nil {
		return d, err
	}

	tx, err := num("tx frequency", rec.TXFreqHz)
	if err != nil {
		return d, err
	}
	if tx >= maxStorableFreqHz {
		// write.go's rung 7 ceiling, mirrored on read: a read never
		// constructs a Known value Valid would refuse.
		d.TxFreqHz = codeplug.FreqField{State: codeplug.Unknown}
	} else {
		d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: tx}
	}

	offset, err := num("offset", rec.OffsetHz)
	if err != nil {
		return d, err
	}
	if offset > maxOffsetHz {
		// write.go's rung 7 ceiling, mirrored on read.
		d.OffsetHz = codeplug.FreqField{State: codeplug.Unknown}
	} else {
		d.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: offset}
	}

	for _, m := range []struct {
		name string
		src  civ.Optional[string]
		dst  *codeplug.StringField
	}{
		{"duplex", rec.Duplex, &d.Duplex},
		{"tone mode", rec.ToneMode, &d.ToneMode},
		{"filter", rec.Filter, &d.Filter},
		{"DTCS polarity", rec.DTCSPolarity, &d.DTCSPolarity},
	} {
		v, err := text(m.name, m.src)
		if err != nil {
			return d, err
		}
		*m.dst = codeplug.StringField{State: codeplug.Known, Value: v}
	}

	for _, m := range []struct {
		name string
		src  civ.Optional[uint64]
		dst  *codeplug.ToneField
	}{
		{"transmit tone", rec.ToneTXDeciHz, &d.ToneTx},
		{"receive tone", rec.ToneRXDeciHz, &d.ToneRx},
	} {
		v, err := num(m.name, m.src)
		if err != nil {
			return d, err
		}
		if tone := spec.Tone(v); caps.AdmitsTone(tone) {
			*m.dst = codeplug.ToneField{State: codeplug.Known, Value: tone}
		} else {
			// T1(3): outside the declared domain — and ZERO is outside —
			// so the read reports Unknown with a zero value. A read never
			// constructs a Known value that Valid would refuse.
			*m.dst = codeplug.ToneField{State: codeplug.Unknown}
		}
	}

	code, err := num("DTCS code", rec.DTCSCode)
	if err != nil {
		return d, err
	}
	// FALSE UNTIL FIXED HERE: the 512-code table (E3, plan O-10) is built
	// from three OCTAL digits (0-7 each), but this field's two packed-BCD
	// bytes decode any digit 0-9 without complaint (civ's BCD decoder only
	// refuses a nibble above 9). A record whose packed digits are 8 or 9 —
	// not a corrupt frame, just a value outside this field's printed
	// domain — decodes to a number such as 888 that dtcsCodes() never
	// generated. Membership must therefore be checked, not assumed.
	known := false
	for _, c := range caps.DTCSCodes {
		if c == int(code) {
			known = true
			break
		}
	}
	if known {
		d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: int(code)}
	} else {
		// T1(3)'s tone precedent, mirrored: a read never constructs a
		// Known value Valid would refuse.
		d.DTCSCode = codeplug.IntField{State: codeplug.Unknown}
	}

	dataMode, err := text("data mode", rec.DataMode)
	if err != nil {
		return d, err
	}
	d.DataMode = codeplug.BoolField{State: codeplug.Known, Value: dataMode == "ON"}

	// The written-down zeros and unavailables. ClarHz/RxClar/TxClar,
	// CTCSS and Shift are the Yaesu family's fields: this radio has no
	// clarifier and expresses duplex and tone_mode instead, so their zero
	// values are the honest report. CTCSSTone and TagDisplay are
	// Unavailable because the record has no such field at all, and
	// ScanSkip is Unavailable per O-6 — the ★n nibble marks a channel
	// INTO a select-scan group, which is not the two-valued "skip this
	// one" spec.FieldScanSkip means, and the tier's hard constraint is
	// never to map it as skip.
	d.CTCSSTone = codeplug.ToneField{State: codeplug.Unavailable}
	d.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable}
	d.ScanSkip = codeplug.BoolField{State: codeplug.Unavailable}
	return d, nil
}

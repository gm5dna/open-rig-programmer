// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic705 "github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The two printed ceilings this radio's DIGIT LEADERS impose, which no
// byte-width check can see.
//
// civ's BCD validator checks scale and byte width only, and both of these
// values fit their spans perfectly well — 500,000,000 in five packed-BCD
// bytes, 10,000,000 in three. The bounds come from the manual's own
// leaders: the frequency field prints "1 GHz digit: (fixed)" and "100 MHz
// digit: 0 ~ 4" (PDF p.18, folio 17; matrix erratum 7), and the offset
// field's three bytes have a FIXED 10 MHz digit (same page; matrix erratum
// 8 — 9.9999 MHz, not 9.99). Without these two rungs a consented write
// would put a digit on the wire the manual bounds away.
const (
	// maxStorableFreqHz is the first frequency this radio's printed digit
	// leaders cannot express. A write AT or ABOVE it is refused.
	maxStorableFreqHz = 500_000_000
	// maxOffsetHz is the largest offset the three-byte field can express
	// under its fixed 10 MHz digit.
	maxOffsetHz = 9_999_900
)

// WriteChannel implements driver.Session: one channel, written as a single
// acknowledged CI-V memory set.
//
// THE LADDER IS ORDERED BY RULING T5, and the order is the contract rather
// than an implementation detail:
//
//	rungs 1-8   LOCAL. Every refusal decidable from the channel and the
//	            capabilities alone. NO BYTE LEAVES.
//	rung 9      THE SINGLE READ, which performs T2's address check first.
//	rungs 10-12 READ-DEPENDENT — T5's one recorded exception to "before
//	            any wire traffic", because a refusal that needs the SLOT'S
//	            CURRENT STATE cannot precede the read that obtains it.
//	rung 13     the frame is built, still before any write traffic.
//
// Every refusal returns Steps as an EMPTY, non-nil slice: a refusal that
// happens before the frames are built has no sequence to describe, and the
// value is journalled as JSON.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// The write is TWO exchanges — the preservation read and the set — so
	// it is serialised against itself. A concurrent write landing between
	// them would decide against one radio state and write against another.
	s.opMu.Lock()
	defer s.opMu.Unlock()

	none := driver.WriteResult{Steps: []driver.WriteStep{}}
	refuse := func(fields []spec.Field, reason string) (driver.WriteResult, error) {
		return none, &driver.WriteRefusedError{Slot: ch.Slot, Fields: fields, Reason: reason}
	}

	// RUNG 1: the slot parses, and a CALL channel above four is refused
	// before any builder is reached (O-9's first line of defence).
	addr, bankID, err := slotToAddress(ch.Slot)
	if err != nil {
		return refuse(nil, err.Error())
	}

	// RUNG 2: the slot is in a bank THIS SESSION supports. For the sparse
	// memory bank "supports" means WithinSpace, not Slots membership — an
	// add at any addressable slot is legitimate, which is the whole reason
	// the sparse model exists. This is defence in depth behind rung 1,
	// which already refuses every address outside the two banks; it is
	// kept because the capability set, not slots.go, is what every other
	// layer of this project enforces against.
	bank, ok := s.caps.Bank(bankID)
	if !ok || !bank.WithinSpace(ch.Slot) {
		return refuse(nil, fmt.Sprintf("slot %q is not in any bank this session supports", ch.Slot))
	}

	// RUNG 3: an empty channel is an ERASE, and this tier ships no erase
	// path at all (spec D4 adjudication 19) — though TWO clear wire forms
	// exist on this radio (matrix §3.13), and neither is built here, in
	// core/civ, or admitted by the gate.
	if ch.Empty() {
		return refuse([]spec.Field{spec.FieldErase},
			"this tier ships no erase path: an IC-705 memory is never cleared by this program, although the radio itself documents two ways to do it")
	}
	data := *ch.Data

	// RUNG 4: each field's own tri-state sanity, against THIS session's
	// capabilities. EVERY offender is collected into one refusal rather
	// than the first: a caller handed a channel back with one field named
	// would fix it and meet the next one, which for a bare create is
	// eight round trips through the same message.
	var invalid []spec.Field
	var reasons []string
	for _, c := range []struct {
		field spec.Field
		err   error
	}{
		{spec.FieldTxFrequency, data.TxFreqHz.Valid()},
		{spec.FieldOffset, data.OffsetHz.Valid()},
		{spec.FieldDuplex, data.Duplex.Valid(vocabValues(s.caps.DuplexOptions, func(o spec.DuplexOption) string { return o.Value }))},
		{spec.FieldToneMode, data.ToneMode.Valid(vocabValues(s.caps.ToneModes, func(m spec.ToneMode) string { return m.Value }))},
		{spec.FieldToneTx, data.ToneTx.Valid(s.caps)},
		{spec.FieldToneRx, data.ToneRx.Valid(s.caps)},
		{spec.FieldDTCSCode, data.DTCSCode.Valid(s.caps.DTCSCodes)},
		{spec.FieldDTCSPolarity, data.DTCSPolarity.Valid(s.caps.DTCSPolarities)},
		{spec.FieldFilter, data.Filter.Valid(s.caps.Filters)},
		{spec.FieldDataMode, data.DataMode.Valid()},
		{spec.FieldCTCSSTone, data.CTCSSTone.Valid(s.caps)},
		{spec.FieldTagDisplay, data.TagDisplay.Valid()},
		{spec.FieldScanSkip, data.ScanSkip.Valid()},
	} {
		if c.err != nil {
			invalid = append(invalid, c.field)
			reasons = append(reasons, fmt.Sprintf("%s: %v", c.field, c.err))
		}
	}
	if len(invalid) > 0 {
		return refuse(invalid, strings.Join(reasons, "; "))
	}

	// RUNG 5: THE CAPABILITY GATE. Every field this channel REQUESTS must
	// be writable for this bank — spec.FieldSupport.CanWrite, which the
	// consent transform is the only thing that opens while
	// writeTrialsComplete is false. All offenders are named in one
	// refusal, in requestedFields' order.
	var ungated []spec.Field
	for _, f := range requestedFields(data) {
		fs := s.caps.FieldSupport(bankID, f)
		if fs.CanWrite() || fs.Write == spec.Inert {
			continue
		}
		ungated = append(ungated, f)
	}
	if len(ungated) > 0 {
		return refuse(ungated, fmt.Sprintf("this session cannot write %d of the fields this channel requests on bank %s (the radio's write support is unverified, or the field is one this radio does not express at all)", len(ungated), bankID))
	}

	// RUNG 6: MANDATORY-KNOWN (R6). Only Known values are ever encoded; a
	// mapped field that is not Known is REFUSED — never synthesised,
	// never preserved-by-cache.
	//
	// THE TWO TONE SPANS ARE THE ONE RULED EXCEPTION (T1(4)) and are
	// resolved at rung 10 from the read this ladder already performs. They
	// are absent from this list for that reason and no other.
	var unknown []spec.Field
	if data.Mode == "" {
		unknown = append(unknown, spec.FieldMode)
	} else if !slices.Contains(s.caps.Modes, data.Mode) {
		// F3: a non-empty mode outside this radio's vocabulary used to be
		// refused only by civ's own encoder (BuildMemorySet, well after
		// the preservation read at rung 9 has already put a frame on the
		// wire). T5 requires every locally decidable refusal — and mode
		// membership is decidable from the channel and s.caps alone — to
		// precede ALL wire traffic, so it is checked here, at the same
		// rung as mode's own emptiness.
		return refuse([]spec.Field{spec.FieldMode}, fmt.Sprintf("mode %q is not one of this radio's %d modes", data.Mode, len(s.caps.Modes)))
	}
	for _, c := range []struct {
		field spec.Field
		state codeplug.FieldState
	}{
		{spec.FieldTxFrequency, data.TxFreqHz.State},
		{spec.FieldDuplex, data.Duplex.State},
		{spec.FieldOffset, data.OffsetHz.State},
		{spec.FieldToneMode, data.ToneMode.State},
		{spec.FieldDTCSCode, data.DTCSCode.State},
		{spec.FieldDTCSPolarity, data.DTCSPolarity.State},
		{spec.FieldFilter, data.Filter.State},
		{spec.FieldDataMode, data.DataMode.State},
	} {
		if c.state != codeplug.Known {
			unknown = append(unknown, c.field)
		}
	}
	if len(unknown) > 0 {
		return refuse(unknown, "every field this record carries must be explicitly Known: this driver encodes only Known values and never invents one from a default, so an incomplete channel is refused rather than quietly completed")
	}

	// RUNG 7: the two printed ceilings (see the constants above).
	if data.FreqHz >= maxStorableFreqHz {
		return refuse([]spec.Field{spec.FieldFrequency}, fmt.Sprintf("frequency %d Hz is at or above %d Hz, which this radio's printed digit leaders cannot express (the 100 MHz digit is bounded at 4 and the 1 GHz digit is fixed)", data.FreqHz, maxStorableFreqHz))
	}
	if data.TxFreqHz.State == codeplug.Known && data.TxFreqHz.Value >= maxStorableFreqHz {
		return refuse([]spec.Field{spec.FieldTxFrequency}, fmt.Sprintf("transmit frequency %d Hz is at or above %d Hz, which this radio's printed digit leaders cannot express", data.TxFreqHz.Value, maxStorableFreqHz))
	}
	if data.OffsetHz.State == codeplug.Known && data.OffsetHz.Value > maxOffsetHz {
		return refuse([]spec.Field{spec.FieldOffset}, fmt.Sprintf("offset %d Hz is above this radio's printed ceiling of %d Hz (9.9999 MHz): the field's three bytes have a fixed 10 MHz digit", data.OffsetHz.Value, maxOffsetHz))
	}

	// RUNG 8: build the record from the Known values. Nothing is
	// defaulted, nothing is merged, and the two tone spans are left for
	// rung 10.
	rec, err := recordFrom(data)
	if err != nil {
		// recordFrom mints a refusal with the fields named but no slot —
		// it has no slot to name — so it is re-issued here with this
		// write's own.
		var refused *driver.WriteRefusedError
		if errors.As(err, &refused) {
			return refuse(refused.Fields, refused.Reason)
		}
		return refuse(nil, err.Error())
	}
	rec.Address = addr

	// RUNG 9: THE SINGLE READ. readRaw performs T2's answer-address
	// equality check first and returns the whole record.
	raw, err := s.readRaw(ctx, addr)
	if err != nil {
		return none, err
	}

	// RUNG 10: the tone spans (T1(4)). A tone that is not Known takes the
	// JUST-READ record's civ-layer tone number VERBATIM — the radio's own
	// value, preserved at value level, available because this read is
	// already mandatory under E6. It is not synthesis and it is not the
	// cache-merge R6 overrules: the number comes from the one read this
	// ladder already performs.
	//
	// ON A CREATE THERE IS NO PRIOR RECORD, so a not-Known tone is REFUSED
	// naming the field (O-11): this radio's CI-V Reference Guide prints no
	// default tone anywhere, and refusing is the ABSENCE of an assumption.
	if data.ToneTx.State != codeplug.Known || data.ToneRx.State != codeplug.Known {
		var missing []spec.Field
		if data.ToneTx.State != codeplug.Known {
			missing = append(missing, spec.FieldToneTx)
		}
		if data.ToneRx.State != codeplug.Known {
			missing = append(missing, spec.FieldToneRx)
		}
		if raw.empty {
			return refuse(missing, "this is a create into an empty slot, so there is no stored tone to preserve, and this radio's manual prints no default tone value anywhere — set both tones explicitly")
		}
		prior, perr := civic705.Profile().ParseMemoryAnswer(raw.frame)
		if perr != nil {
			return none, fmt.Errorf("ic705: write %s: reading the tone to preserve: %w", ch.Slot, perr)
		}
		if data.ToneTx.State != codeplug.Known {
			v, okv := prior.ToneTXDeciHz.Get()
			if !okv {
				return refuse([]spec.Field{spec.FieldToneTx}, "the stored record carries no transmit tone to preserve")
			}
			rec.ToneTXDeciHz = civ.Available(v)
		}
		if data.ToneRx.State != codeplug.Known {
			v, okv := prior.ToneRXDeciHz.Get()
			if !okv {
				return refuse([]spec.Field{spec.FieldToneRx}, "the stored record carries no receive tone to preserve")
			}
			rec.ToneRXDeciHz = civ.Available(v)
		}
	}

	// RUNG 11: THE E6 PRESERVATION CHECK.
	//
	// The IC-705's record carries three DV call signs, a digital-squelch
	// byte, a DV code-squelch byte, the Split flag and the star-marking
	// Select nibble, and NO spec.Field claims any of them (matrix §3.16 A4
	// as restated by erratum 6, plus O-6's unmapping). The whole 115-byte
	// data area goes out on every set, so writing this channel would
	// overwrite them with this profile's template. Enabler E6 rules:
	// REFUSE, with the reason named, NEVER REWRITE.
	//
	// An EMPTY slot is skipped, and that is not a hole: there are no user
	// bytes there to preserve. Both unverified empty shapes are covered by
	// raw.empty, so this exemption is exactly "the radio holds nothing
	// here".
	if !raw.empty {
		if reason, clash := unpreservableAreas(raw.record); clash {
			return refuse(nil, reason)
		}
	}

	// RUNG 12: T3, THE OCCUPIED SURPRISE. Bounded discovery means the
	// materialised inventory can be a strict subset of what the radio
	// holds, so an ADD to a slot the walk NEVER VISITED must not be
	// allowed to land on top of a record nobody knew about. The read has
	// already told us: empty means proceed, a record means refuse.
	//
	// This is what makes the bounded default walk safe rather than
	// silently destructive, and it is the rung that pays for dropping the
	// early-stop an earlier draft had.
	if !s.inventoryKnows(ch.Slot) && !raw.empty {
		return refuse(nil, occupiedSurpriseReason(ch.Slot))
	}

	// RUNG 13: the frame is built, still before any write traffic.
	cmd, err := civic705.Profile().BuildMemorySet(rec)
	if err != nil {
		return refuse(nil, err.Error())
	}

	// ONE STEP, DECLARED BEFORE IT GOES OUT. ClassWriteWithAck waits for
	// the radio's FB and NEVER retransmits on a timeout: a retransmitted
	// memory set could write the channel twice, and a timeout leaves the
	// outcome genuinely unknown.
	steps := []driver.WriteStep{{Command: "1A 00"}}
	if _, err := s.eng.Do(ctx, cmd, civ.CIVWriteWithAckSpec(civic705.Profile().AcknowledgementMatcher())); err != nil {
		// An FA is an ATTRIBUTABLE outcome — the radio refused — so the
		// step is Sent but not Confirmed. A timeout is not: it leaves
		// Sent false, because the write may or may not have landed and
		// this program must not claim to know which.
		if errors.Is(err, transport.ErrRejected) {
			steps[0].Sent = true
		}
		return driver.WriteResult{Steps: steps}, fmt.Errorf("ic705: write %s: 1A 00: %w", ch.Slot, err)
	}
	steps[0].Sent, steps[0].Confirmed = true, true
	return driver.WriteResult{Steps: steps}, nil
}

// occupiedSurpriseReason is rung 12's refusal text, in one place because a
// test pins it verbatim.
//
// THE REMEDIES IT NAMES MUST ACTUALLY WORK, AND MUST BE REACHABLE. Two
// wordings have been removed on that rule. "Or read this slot first" went
// because ReadChannel never adds a slot to s.inventory, so a user who
// followed it met the identical refusal a second time. "With
// WithFullInventoryWalk() if the slot is outside that range" went for the
// second half of the rule (registration review, deferred minor): the
// advice was true of the Go API and useless to a user, since
// internal/wiring's registry row does not pass that option and no CLI
// flag and no GUI control exposes it — prose naming an unreachable option
// reads as a setting the reader has failed to find. The inventory is
// materialised ONCE, by the walk Open performs, so the one thing that
// changes this answer from where the user stands is re-opening the
// session, which re-runs discovery and is enough whenever the slot is
// inside the bounded walk's range and was merely empty when the session
// opened. For a slot ABOVE that range the honest answer is the BOUND
// itself, which is what the text now gives.
func occupiedSurpriseReason(slot string) string {
	return fmt.Sprintf("slot %s holds a record this session's inventory never saw, and writing here would overwrite a channel nobody has looked at: this session's walk covered display groups G01-G%02d, so re-open the session to run discovery again — a slot outside that range stays unlisted, and this build offers no setting that widens the walk. Reading the slot does not help: ReadChannel never adds one to the inventory", slot, defaultWalkGroups)
}

// requestedFields names every field this channel ASKS this radio to store,
// in the order a refusal names them.
//
// THE SET DERIVES FROM THIS MODEL'S MATRIX §2, not from the Yaesu
// contract's "six unconditional fields" (frequency, mode, clarifier,
// CTCSS state, shift, tag). Three of those six are Unsupported on both
// banks here, so requesting them unconditionally would refuse EVERY write.
//
// Three tiers, and each is a different claim:
//
//   - UNCONDITIONAL: frequency, mode and tag — the fields this record
//     always carries and a channel always states.
//   - CONDITIONAL ON BEING Known: the ten tier fields, in ChannelData's
//     own declaration order. A field the caller left Unknown is not a
//     request; it is either preserved (the tones) or refused (rung 6).
//   - CONDITIONAL ON BEING PRESENT: the six Yaesu-shaped fields this
//     radio does not carry. Requesting them WHEN PRESENT is the whole
//     point of the Wave-1 C2 contract — a cross-loaded Yaesu channel is
//     then REFUSED by the capability gate rather than silently dropped —
//     and requesting them always would refuse every ordinary write.
func requestedFields(d codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldTag}

	known := func(f spec.Field, state codeplug.FieldState) {
		if state == codeplug.Known {
			fields = append(fields, f)
		}
	}
	known(spec.FieldTxFrequency, d.TxFreqHz.State)
	known(spec.FieldDuplex, d.Duplex.State)
	known(spec.FieldOffset, d.OffsetHz.State)
	known(spec.FieldToneMode, d.ToneMode.State)
	known(spec.FieldToneTx, d.ToneTx.State)
	known(spec.FieldToneRx, d.ToneRx.State)
	known(spec.FieldDTCSCode, d.DTCSCode.State)
	known(spec.FieldDTCSPolarity, d.DTCSPolarity.State)
	known(spec.FieldFilter, d.Filter.State)
	known(spec.FieldDataMode, d.DataMode.State)

	if d.ClarHz != 0 || d.RxClar || d.TxClar {
		fields = append(fields, spec.FieldClarifier)
	}
	if d.CTCSS != "" {
		fields = append(fields, spec.FieldCTCSSState)
	}
	known(spec.FieldCTCSSTone, d.CTCSSTone.State)
	if d.Shift != "" {
		fields = append(fields, spec.FieldShift)
	}
	known(spec.FieldTagDisplay, d.TagDisplay.State)
	known(spec.FieldScanSkip, d.ScanSkip.State)
	return fields
}

// recordFrom builds the outgoing civ.MemoryRecord from a channel's KNOWN
// values, and refuses — naming the field — if anything it needs is
// missing or unencodable.
//
// NOTHING IS INVENTED FROM A DEFAULT AND NOTHING IS MERGED FROM A CACHE
// (R6). The two tone spans are deliberately left unset here: they are
// rung 10's, resolved from the record the ladder has already read.
//
// The NAME is checked HERE rather than left to the encoder, and the
// reason is ordering: civ would refuse an over-long or illegal name at
// rung 13, which is AFTER the preservation read, and ruling T5 requires
// every locally decidable refusal to precede all wire traffic.
func recordFrom(d codeplug.ChannelData) (civ.MemoryRecord, error) {
	p := civic705.Profile()
	if len(d.Tag) > p.NameLength() {
		return civ.MemoryRecord{}, &driver.WriteRefusedError{
			Fields: []spec.Field{spec.FieldTag},
			Reason: fmt.Sprintf("the name %q is %d bytes, and this radio's name field holds %d", d.Tag, len(d.Tag), p.NameLength()),
		}
	}
	charset := p.NameCharset()
	legal := make(map[byte]bool, len(charset))
	for _, b := range charset {
		legal[b] = true
	}
	for i := 0; i < len(d.Tag); i++ {
		if !legal[d.Tag[i]] {
			return civ.MemoryRecord{}, &driver.WriteRefusedError{
				Fields: []spec.Field{spec.FieldTag},
				Reason: fmt.Sprintf("the name %q carries byte %#02x, which is not in this radio's printed character set", d.Tag, d.Tag[i]),
			}
		}
	}

	rec := civ.MemoryRecord{
		RXFreqHz:     civ.Available(d.FreqHz),
		TXFreqHz:     civ.Available(d.TxFreqHz.Value),
		OffsetHz:     civ.Available(d.OffsetHz.Value),
		DTCSCode:     civ.Available(uint64(d.DTCSCode.Value)),
		Duplex:       civ.Available(d.Duplex.Value),
		Mode:         civ.Available(d.Mode),
		Filter:       civ.Available(d.Filter.Value),
		ToneMode:     civ.Available(d.ToneMode.Value),
		DTCSPolarity: civ.Available(d.DTCSPolarity.Value),
		Name:         civ.Available(d.Tag),
	}
	// The data-mode flag is the one mapped field whose neutral shape is a
	// bool and whose wire shape is an enum name.
	if d.DataMode.Value {
		rec.DataMode = civ.Available("ON")
	} else {
		rec.DataMode = civ.Available("OFF")
	}
	// The tones, WHEN KNOWN. When they are not, rung 10 fills them from
	// the radio's own record — and refuses if there is none.
	if d.ToneTx.State == codeplug.Known {
		rec.ToneTXDeciHz = civ.Available(uint64(d.ToneTx.Value))
	}
	if d.ToneRx.State == codeplug.Known {
		rec.ToneRXDeciHz = civ.Available(uint64(d.ToneRx.Value))
	}
	return rec, nil
}

// unpreservableArea names one contiguous run of unmapped record offsets
// and what the radio stores there, so a refusal can say what it is
// protecting rather than quoting a byte number at a user.
type unpreservableArea struct {
	lo, hi int
	what   string
}

// unpreservableAreas is the inventory O-2 computes: the 53 whole bytes of
// this record that no spec.Field claims, grouped by what the manual says
// they hold. Record offsets, 0-based (data-area position − 5).
var unpreservableAreaTable = []unpreservableArea{
	{0, 0, "the Split flag and the ★n Select-scan marking"},
	{10, 10, "the digital squelch setting"},
	{20, 20, "the DV digital code squelch setting"},
	{24, 47, "the three DV call signs (UR, R1, R2)"},
	{57, 57, "the digital squelch setting's copy in the duplicated TX block"},
	{67, 67, "the DV code squelch's copy in the duplicated TX block"},
	{71, 94, "the three DV call signs' copies in the duplicated TX block"},
}

// unpreservableAreas reports whether record holds anything this profile
// cannot carry back out, and names it.
//
// The comparison is against the PROFILE'S OWN Fixed template, taken from
// the layout rather than restated here, and it covers exactly the bytes no
// span claims — computed from the layout's spans, so a future change to
// the layout moves this check with it instead of leaving a stale literal
// behind.
func unpreservableAreas(record []byte) (string, bool) {
	layout, ok := civic705.Profile().LayoutFor(len(record))
	if !ok {
		// Unreachable: readRaw has already refused a length this profile
		// does not declare.
		return fmt.Sprintf("this radio answered with a %d-byte record, which this profile does not declare", len(record)), true
	}
	template := layout.Fixed
	for _, offset := range unmappedOffsets(layout) {
		if offset >= len(record) || offset >= len(template) {
			continue
		}
		if record[offset] == template[offset] {
			continue
		}
		return fmt.Sprintf(
			"this channel stores %s (record offset %d holds %#02x, and this driver can only write %#02x there): the whole record goes out on every memory set, so writing this channel would erase what the radio holds. Refusing rather than rewriting it",
			describeOffset(offset), offset, record[offset], template[offset]), true
	}
	return "", false
}

// describeOffset names what the radio stores at one unmapped offset.
func describeOffset(offset int) string {
	for _, a := range unpreservableAreaTable {
		if offset >= a.lo && offset <= a.hi {
			return a.what
		}
	}
	return "a record area no field of this driver's model claims"
}

// unmappedOffsets returns every record offset NEITHER of whose nibbles any
// span of layout claims — the bytes the encoder fills from the Fixed
// template, and therefore the bytes a write cannot preserve.
//
// A byte only ONE of whose nibbles is claimed is deliberately excluded:
// the encoder writes the mapped nibble and takes the other from the
// template, so it is not a whole byte the driver is unable to carry. This
// layout has no such byte — offset 9's two nibbles are claimed by the
// duplex and tone-mode spans together — and the rule is written to survive
// one appearing.
func unmappedOffsets(layout civ.RecordLayout) []int {
	hi := make([]bool, layout.Length)
	lo := make([]bool, layout.Length)
	for _, sp := range layout.Fields {
		for i := sp.Offset; i < sp.Offset+sp.Length && i < layout.Length; i++ {
			switch sp.Nibble {
			case civ.NibbleHigh:
				hi[i] = true
			case civ.NibbleLow:
				lo[i] = true
			default:
				hi[i], lo[i] = true, true
			}
		}
	}
	var out []int
	for i := range hi {
		if !hi[i] && !lo[i] {
			out = append(out, i)
		}
	}
	return out
}

// vocabValues extracts the wire-form strings from a capability vocabulary,
// so a field's own Valid can be handed the plain []string it takes.
func vocabValues[T any](items []T, value func(T) string) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = value(it)
	}
	return out
}

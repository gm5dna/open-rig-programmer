// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic905 "github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The MEM bank's addressable space, as the radio numbers it. Both are
// MANUAL-EVIDENCED: PDF p.19 (folio 18)'s left legend prints
// "①, ②: Memory group number / 00 00 ~ 00 99" and "③, ④: Memory channel
// numbers / … 00 00 ~ 00 99: 00 ~ 99".
const (
	memGroups           = 100
	memChannelsPerGroup = 100
)

// slotAddress decodes a canonical slot string into the wire address this
// driver sends.
//
// IT ACCEPTS TWO NAMESPACES, AND THEY ARE DISJOINT BY CONSTRUCTION
// (ruling R4):
//
//   - MEM uses the shared sparse form, spec.SparseSlot's "G%02d-%03d",
//     which is 1-BASED on its documented contract. The wire address is
//     therefore one less in each dimension: "G01-001" is wire group 0,
//     channel 0.
//   - CALL uses core/civ/ic905's own "C01".."C12", a dense twelve-slot
//     bank mapping to wire group 100, channels 0…11.
//
// spec.ParseSparseSlot refuses any string without a leading "G", so no
// CALL slot can ever be read as a MEM address and no MEM address can ever
// render as a CALL slot. core/civ/ic905's profile_test.go proves that
// over the whole 10,000-address space rather than asserting it.
//
// Both parsers are STRICT — each re-renders what it decoded and refuses
// anything that does not reproduce byte for byte — so "G5-12" and "C1"
// are refused rather than admitted as second names for one slot.
func slotAddress(slot string) (civ.ChannelAddress, error) {
	if n, ok := civic905.ParseCallSlot(slot); ok {
		return civ.ChannelAddress{Group: callWireGroup, Channel: n}, nil
	}
	group, channel, ok := spec.ParseSparseSlot(slot)
	if !ok {
		return civ.ChannelAddress{}, fmt.Errorf("ic905: %q is neither a memory slot (%q) nor a call slot (%q)", slot, spec.SparseSlot(1, 1), civic905.CallSlot(0))
	}
	if group < 1 || group > memGroups || channel < 1 || channel > memChannelsPerGroup {
		return civ.ChannelAddress{}, fmt.Errorf("ic905: memory slot %q is outside this radio's %d x %d address space", slot, memGroups, memChannelsPerGroup)
	}
	return civ.ChannelAddress{Group: group - 1, Channel: channel - 1}, nil
}

// memSlot renders the canonical MEM slot string for a WIRE address, the
// inverse of slotAddress's memory arm.
func memSlot(group, channel int) string {
	return spec.SparseSlot(group+1, channel+1)
}

// isEmptyRecord reports whether every byte of a record is 0xFF.
//
// IT IS THE SECOND, SEPARATE EMPTY-CHANNEL ASSUMPTION, with its own
// register entry and its own lift: D5 entry 2(b), lift ic905-R-15. The
// FIRST is the FA answer (D5 entry 2(a), lift ic905-R-14), which recordAt
// handles. One capture that returns FA says nothing about the FF case and
// vice versa, so this driver implements BOTH readings — and neither guess
// deletes a channel, because an all-0xFF record carries no field values
// to lose.
//
// IT MUST BE ASKED BEFORE THE RECORD PARSER, and that is ruling R10.
// 0xFF is not a valid packed-BCD pair and not a member of any enum this
// layout declares, so ParseMemoryAnswer dies on the frequency field long
// before anything useful comes back — a failure INDISTINGUISHABLE from a
// corrupted record. civ.MemoryAnswerRecord exists to hand a driver the
// raw bytes for exactly this question, which is why recordAt returns them
// undecoded.
func isEmptyRecord(record []byte) bool {
	for _, b := range record {
		if b != 0xFF {
			return false
		}
	}
	return len(record) > 0
}

// ReadChannel implements driver.Session: ONE 1A 00 read, mapped into one
// codeplug.Channel.
//
// The order is contractual, and every step of it is a ruling:
//
//  1. slotAddress — both namespaces, strictly.
//  2. the read, through recordAt, which carries the CONTINUOUS length
//     fingerprint and the T2 address check and maps an FA (T4) to "the
//     slot is empty".
//  3. the ALL-0xFF recognition (R10), before the record parser, because
//     the parser cannot survive one.
//  4. the field mapping, with the tone rule of T1(3).
//
// AN EMPTY SLOT IS NOT AN ERROR, and it must not be: core/driver's own
// contract says so, and an error here would abort core/clone's whole
// ReadAll walk over a channel that simply has nothing in it.
//
// A record that is neither all-0xFF nor decodable IS an error (spec D4,
// malformed records): no partial parse, no fake Unavailable channel. The
// pre-parse hook narrows the empty case; it does not license swallowing a
// corrupt one.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	addr, err := slotAddress(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic905: ReadChannel: %w", err)
	}

	record, present, err := s.recordAt(ctx, addr)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic905: ReadChannel %s: %w", slot, err)
	}
	if !present || isEmptyRecord(record) {
		return codeplug.Channel{Slot: slot}, nil
	}

	rec, err := s.parseRecord(addr, record)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic905: ReadChannel %s: %w", slot, err)
	}
	return neutralChannel(rec, slot, s.caps), nil
}

// parseRecord decodes raw record bytes into a neutral civ.MemoryRecord.
//
// It rebuilds the ANSWER FRAME civ.ParseMemoryAnswer takes, from the
// address and the bytes recordAt already validated, rather than keeping
// the original frame around: the pair (address, record) is what recordAt
// proved things about — the length is in the accepted set, the address is
// the one we asked for — and re-deriving the frame from them is what
// makes it impossible to decode a frame the checks were never applied to.
func (s *Session) parseRecord(addr civ.ChannelAddress, record []byte) (civ.MemoryRecord, error) {
	// The answer's envelope runs radio -> controller, which is the
	// direction that distinguishes an answer from the request that drew
	// it (PDF p.3, folio 2).
	frame := []byte{0xFE, 0xFE, s.profile.ControllerAddress(), s.profile.RadioAddress(), 0x1A, 0x00}
	addrCmd, err := s.profile.BuildMemoryRead(addr)
	if err != nil {
		return civ.MemoryRecord{}, err
	}
	// BuildMemoryRead's frame is FE FE <radio> <controller> 1A 00
	// <address> FD, so its address bytes are everything between the
	// command and the terminator — taken from the CODEC rather than
	// re-encoded here, so this driver never owns an address encoding.
	addrBytes := addrCmd.Bytes()
	frame = append(frame, addrBytes[6:len(addrBytes)-1]...)
	frame = append(frame, record...)
	frame = append(frame, 0xFD)
	return s.profile.ParseMemoryAnswer(frame)
}

// toneField maps a civ-layer tone number to a neutral field. A value
// INSIDE the declared capability domain (1…2999 deciHz) is Known; a value
// outside it — ZERO INCLUDED — is Unknown.
//
// RULING T1(3): A READ NEVER CONSTRUCTS A KNOWN VALUE codeplug.Validate
// WOULD REFUSE. The civ layer is lossless and semantics-free — a tone
// span decodes as a plain BCD number over 0…2999 deciHz, zero included,
// and the gate's byte-identity survives because the decode loses nothing.
// The DRIVER decides what that number means. This record can hold 0 in a
// tone span (it is a legal BCD number, and a radio may well store it on a
// tone-OFF channel), but 0 Hz is not a tone, the declared CTCSSToneRange
// starts at 1, and a Known 0 would fail ToneField.Valid on the very next
// validation.
//
// Unknown, not Unavailable: the radio HAS the field and this read simply
// learned nothing usable from it. Both mean "preserve whatever the radio
// has" to every write path downstream, which is the only honest
// instruction here.
func toneField(caps spec.Capabilities, deciHz uint64) codeplug.ToneField {
	t := spec.Tone(deciHz)
	if caps.AdmitsTone(t) {
		return codeplug.ToneField{State: codeplug.Known, Value: t}
	}
	return codeplug.ToneField{State: codeplug.Unknown}
}

// dtcsCodeField maps a civ-layer DTCS code to a neutral field, and it is
// THE TONE PRECEDENT APPLIED TO THE OTHER OCTAL FIELD: a code every one
// of whose digits is 7 or less is Known; anything else is Unknown.
//
// RULING T1(3) AGAIN. ㉓,㉔ is a plain two-byte big-endian BCD span, and
// the civ layer decodes it losslessly and without judgement — so a radio
// (or a bus fault, or a channel this tier has never seen) can put 080 or
// 778 in it, both perfectly good BCD and neither a DTCS code: the printed
// range is OCTAL in all three digits (PDF p.24, folio 23, "• DTCS code
// and polarity setting"). validDTCSCode is that printed range, already
// written down for the write direction; the read direction now asks the
// same question rather than a different one, so no read can produce a
// value this driver's own rung 6a would refuse to write back.
//
// Unknown, not Unavailable, and not a clamp: the radio HAS the field,
// this read learned nothing usable from it, and Unknown means "preserve
// whatever the radio has" to every write path downstream — which is the
// only honest instruction when the bytes are not a code.
func dtcsCodeField(code uint64) codeplug.IntField {
	if code <= math.MaxInt && validDTCSCode(int(code)) {
		return codeplug.IntField{State: codeplug.Known, Value: int(code)}
	}
	return codeplug.IntField{State: codeplug.Unknown}
}

// neutralChannel maps a decoded record into codeplug's neutral terms.
//
// THE TWELVE FIELDS THIS RECORD EXPRESSES come back Known; the eight it
// does not come back Unavailable — "this radio has no such field", which
// is a different statement from Unknown ("it has one and this read did
// not learn it"). THREE OF THE TWELVE ARE DOMAIN-CHECKED FIRST and come
// back Unknown when the bytes are outside their printed domain: the tone
// pair, and the DTCS code, whose three digits are octal. In all three the
// field exists and the number was read — it simply is not a tone, or is
// not a code.
//
// FreqHz takes rec.RXFreqHz WITHOUT NARROWING. CI-V BCD is uint64 end to
// end since D4's widening, and there is no checked conversion to make —
// which is the whole reason the widening happened on this model.
func neutralChannel(rec civ.MemoryRecord, slot string, caps spec.Capabilities) codeplug.Channel {
	mode, _ := rec.Mode.Get()
	name, _ := rec.Name.Get()
	freq, _ := rec.RXFreqHz.Get()
	offset, _ := rec.OffsetHz.Get()
	duplex, _ := rec.Duplex.Get()
	toneMode, _ := rec.ToneMode.Get()
	toneTX, _ := rec.ToneTXDeciHz.Get()
	toneRX, _ := rec.ToneRXDeciHz.Get()
	dtcsCode, _ := rec.DTCSCode.Get()
	dtcsPol, _ := rec.DTCSPolarity.Get()
	filter, _ := rec.Filter.Get()
	dataMode, _ := rec.DataMode.Get()

	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: freq,
			Mode:   mode,
			Tag:    name,

			// The Yaesu-family fields this record does not express.
			// ClarHz stays 0 because the record carries no clarifier at
			// all (matrix §1 rows 6-7, a MANUAL-EVIDENCED absence), and
			// caps.ClarMaxHz is 0 to match, so codeplug.Validate's
			// clarifier check passes on the honest value rather than on a
			// waiver.
			//
			// CTCSS and Shift stay EMPTY STRINGS, and that is safe by
			// construction rather than by luck: codeplug.Validate's
			// checks for both are capability-keyed — they run only when
			// caps supplies the vocabulary — and this radio supplies
			// neither, carrying the Icom tone_mode and duplex pair
			// instead (D4: the two vocabularies never coexist).
			CTCSS: "",
			Shift: "",
			// UNAVAILABLE, never Unknown and never Known: this record has
			// no tone-NUMBER index, no display flag and no scan-skip
			// flag. Byte ⑤'s star nibble is SELECT-group membership,
			// which the document never calls a scan-skip flag and which
			// this tier must never map as skip.
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
			// MANUAL-EVIDENCED ABSENCE: exactly one frequency field, no
			// duplicated TX block (matrix §2 row 11).
			TxFreqHz: codeplug.FreqField{State: codeplug.Unavailable},

			// The nine tier fields this record does express, beside
			// frequency, mode and tag.
			Duplex:       codeplug.StringField{State: codeplug.Known, Value: duplex},
			OffsetHz:     codeplug.FreqField{State: codeplug.Known, Value: offset},
			ToneMode:     codeplug.StringField{State: codeplug.Known, Value: toneMode},
			ToneTx:       toneField(caps, toneTX),
			ToneRx:       toneField(caps, toneRX),
			DTCSCode:     dtcsCodeField(dtcsCode),
			DTCSPolarity: codeplug.StringField{State: codeplug.Known, Value: dtcsPol},
			Filter:       codeplug.StringField{State: codeplug.Known, Value: filter},
			// The record's ⑬ is an enum of two printed values, "00: Data
			// mode OFF" and "01: Data mode ON"; the neutral model's is a
			// BoolField. The mapping is the only place the two meet.
			DataMode: codeplug.BoolField{State: codeplug.Known, Value: dataMode == dataModeOn},
		},
	}
}

// dataModeOn is byte ⑬'s printed ON spelling — PDF p.19 (folio 18), left
// legend column, "⑬: Data mode setting / 00: Data mode OFF / 01: Data
// mode ON" — and it is the string core/civ/ic905's own enum table maps
// 0x01 to. Named once so the read and write directions cannot spell it
// differently.
const dataModeOn = "ON"

// discoverInventory walks this radio's memory space and returns the MEM
// slots that answered with a record, in walk order, together with whether
// the walk COVERED THE WHOLE SPACE.
//
// IT EXISTS BECAUSE core/clone.Service.ReadAll WALKS Bank.Slots (ruling
// R12). A sparse bank's static Slots are empty — its materialised set is
// a property of the radio, not of the model — so without this the clone
// service would return no memories at all.
//
// THE DEFAULT WALK IS BOUNDED, and the reason is operational rather than
// aesthetic. A complete walk is 10,000 reads; at CI-V rates that is
// MINUTES of Open on a sparse or empty radio (~45 ms per read/answer at
// 19k2 before latency), and a multi-minute default Open trains users to
// interrupt it — an interrupted discovery being exactly the "codeplug
// full of deletions" hazard this guards against. The bound:
//
//   - group 0's hundred channels, in full;
//   - then, for each group 1…99, CHANNEL 00 ONLY, descending into that
//     group's remaining channels only if channel 00 answered with a
//     record;
//   - then the twelve CALL slots.
//
// WithFullInventoryWalk covers the whole 100 × 100 space for a user whose
// channels are scattered.
//
// EITHER WAY THE TWO LOAD-BEARING PARTS ARE THE SAME: the ctx bound, and
// the complete flag. The bounded default reports complete FALSE ALWAYS,
// because it genuinely did not look everywhere — an early or bounded stop
// must never be readable as an empty radio. The same flag goes false when
// the walk stops at Budget or when ctx is cancelled.
//
// The budget of 500 is ASSUMED — this document prints no capacity and no
// over-budget behaviour. Register: ic905.group_budget. Lift: ic905-R-09.
//
// THE CALL SLOTS ARE WALKED BUT NOT RETURNED. They are already the CALL
// bank's twelve static slots, and spec.Capabilities.Validate refuses a
// slot claimed by two banks; what the walk gets from them is membership
// of s.inventory, which is what the write ladder's occupied-surprise rung
// consults (ruling T3).
func (s *Session) discoverInventory(ctx context.Context, full bool, budget int) (slots []string, complete bool, err error) {
	occupied := 0
	stopped := false

	// probe reads one address, records it in the session's inventory when
	// it answers with a record, and reports whether the walk may
	// continue.
	probe := func(addr civ.ChannelAddress, slot string, counts bool) (present bool, err error) {
		record, present, err := s.recordAt(ctx, addr)
		if err != nil {
			if ctx.Err() != nil {
				// The caller gave up. That is a bounded stop, not a
				// failure: the session still opens, and complete goes
				// false so nothing downstream reads a truncated walk as
				// an empty radio.
				stopped = true
				return false, nil
			}
			return false, err
		}
		// THE SAME EMPTY RECOGNITION ReadChannel USES, both halves. An FA
		// answer is empty (D5 entry 2(a)) and so is an all-0xFF record
		// (D5 entry 2(b)) — and the walk must agree with the read about
		// which slots hold something, or a slot ReadChannel calls empty
		// would be published as occupied and core/clone would then plan a
		// deletion for a channel that was never there.
		if !present || isEmptyRecord(record) {
			return false, nil
		}
		s.inventory[slot] = true
		if counts {
			slots = append(slots, slot)
			occupied++
			if occupied >= budget {
				stopped = true
			}
		}
		return true, nil
	}

	for group := 0; group < memGroups && !stopped; group++ {
		first := 0
		if !full && group > 0 {
			// The bounded default's one probe per group: channel 00, and
			// the rest of the group ONLY if it answered.
			present, perr := probe(civ.ChannelAddress{Group: group, Channel: 0}, memSlot(group, 0), true)
			if perr != nil {
				return nil, false, perr
			}
			if stopped {
				break
			}
			if !present {
				continue
			}
			first = 1
		}
		for channel := first; channel < memChannelsPerGroup && !stopped; channel++ {
			if _, perr := probe(civ.ChannelAddress{Group: group, Channel: channel}, memSlot(group, channel), true); perr != nil {
				return nil, false, perr
			}
		}
	}

	for n := 0; n < civic905.CallChannels && !stopped; n++ {
		if _, perr := probe(civ.ChannelAddress{Group: callWireGroup, Channel: n}, civic905.CallSlot(n), false); perr != nil {
			return nil, false, perr
		}
	}

	// A bounded default walk is NEVER complete, whatever it found: it did
	// not look at 9,801 of the 10,000 addresses, and saying otherwise
	// would be the exact claim ruling T3's occupied-surprise refusal
	// exists to distrust.
	return slots, full && !stopped, nil
}

// memBudget reads the MEM bank's declared occupancy budget out of caps,
// so the walk's stopping rule and the value codeplug.Diff enforces are
// the SAME number rather than two copies of one assumption.
func memBudget(caps spec.Capabilities) int {
	mem, ok := caps.Bank(spec.BankMemory)
	if !ok {
		return 0
	}
	return mem.Budget
}

// errNoMemoryBank is returned when the capability baseline this driver
// was built with has no MEM bank at all — unreachable through New, and
// refused loudly rather than walked with a zero budget, which would stop
// the walk before its first read and report an empty radio.
var errNoMemoryBank = errors.New("ic905: this driver's capability baseline has no MEM bank to discover")

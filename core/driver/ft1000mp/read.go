// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// dump fetches and caches the full U=00H Status Update dump — see
// doc.go's read design note and Session.dump's own comment. Callers must
// hold s.opMu.
func (s *Session) dumpBytes(ctx context.Context) ([]byte, error) {
	if s.dump != nil {
		return s.dump, nil
	}
	cmd := bincat.NewCommand(bincat.OpStatusUpdate, [4]byte{bincat.UFullDump, 0, 0, 0})
	frame, err := s.eng.Do(ctx, cmd, bincat.ReadSpec(dumpTimeout, 0))
	if err != nil {
		return nil, fmt.Errorf("ft1000mp: full dump (U=00H): %w", err)
	}
	s.dump = frame
	return s.dump, nil
}

// recordAt fetches (or reuses) the cached full dump and decodes the
// record at the dump's own 0-based position idx — see recordIndex for
// how a slot maps to idx, and dumpVFOARecordIndex/dumpVFOBRecordIndex
// (write.go) for the two fixed positions VFOStateRestorer reads.
// Callers must hold s.opMu.
func (s *Session) recordAt(ctx context.Context, idx int) (record, error) {
	raw, err := s.dumpBytes(ctx)
	if err != nil {
		return record{}, err
	}
	start := dumpHeaderLen + idx*recordLen
	if start+recordLen > len(raw) {
		return record{}, fmt.Errorf("ft1000mp: dump is %d bytes, too short for record %d", len(raw), idx)
	}
	rec, err := parseRecord(raw[start : start+recordLen])
	if err != nil {
		return record{}, fmt.Errorf("%w: %w", driver.ErrRecordDecode, err)
	}
	return rec, nil
}

// ReadChannel implements driver.Session: it fetches (or reuses) the
// cached full dump and indexes into it by the slot's fixed record
// position — see doc.go's read design note for why this driver never
// sends a per-channel Status Update.
//
// The empty-slot rule: a record whose memory-mask flag is set (record.go's
// Masked) maps to an EMPTY channel (Data == nil), this driver's own
// evidence-based reading of "Memory Mask" (matrix's Band Selection byte
// citation) — there is no "?;" rejection in this family at all (§Context,
// no NAK), so this is the family-appropriate analogue of every sibling
// driver's "?;"-means-empty convention.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	idx, err := recordIndex(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft1000mp: ReadChannel: %w", err)
	}

	rec, err := s.recordAt(ctx, idx)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft1000mp: ReadChannel %s: %w", slot, err)
	}

	if rec.Masked {
		return codeplug.Channel{Slot: slot}, nil
	}

	data, err := recordToChannelData(rec)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft1000mp: ReadChannel %s: %w", slot, err)
	}
	return codeplug.Channel{Slot: slot, Data: data}, nil
}

// recordToChannelData maps a decoded record onto codeplug.ChannelData —
// shared by ReadChannel and write.go's SnapshotVFOState (which reads
// VFO-A's own record through the identical dump/record shape) so the two
// paths cannot drift onto different field mappings.
func recordToChannelData(rec record) (*codeplug.ChannelData, error) {
	modeWireByte, ok := recordModeBase[rec.ModeByte]
	if !ok {
		// Unreachable: rec.ModeByte is masked to 3 bits (record.go), and
		// recordModeBase covers all 8 possible 3-bit values (0-6 mapped,
		// 7 unused by the manual's own 7-value legend) — refuse rather
		// than silently mislabel if a future record ever carries 7.
		return nil, fmt.Errorf("record mode code %d is outside the documented 0-6 range", rec.ModeByte)
	}
	modeName, ok := modeNames[modeWireByte]
	if !ok {
		return nil, fmt.Errorf("unmapped mode wire byte %#02x", modeWireByte)
	}

	return &codeplug.ChannelData{
		FreqHz:     rec.FreqHz,
		Mode:       modeName,
		ClarHz:     rec.ClarHz,
		RxClar:     rec.RxClar,
		TxClar:     rec.TxClar,
		CTCSS:      "", // Unsupported — matrix §2, no record field
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
		Shift:      rec.Shift,
		Tag:        "", // NoTag — matrix §0/§3
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// A scan-skip bit exists in the Band Selection byte
		// (record.go's flagScanSkip), but grading FieldScanSkip is
		// out of this milestone's registered scope (matrix §4 does
		// not list it) — Unavailable, never guessed into Known.
		ScanSkip: codeplug.BoolField{State: codeplug.Unavailable},

		// The Icom-tier fields: all Unavailable — none of this
		// vocabulary exists on this radio's record or opcode set
		// (matrix §4's "Icom-family fields" absence list).
		TxFreqHz: codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		// OffsetHz is REAL here (unlike the Icom-tier fields below)
		// but write-only — Unknown, not Unavailable: "not yet read
		// ... because the protocol has no way to read it"
		// (codeplug.FieldState's own doc comment), matching
		// caps.go's FieldOffset Read:Unsupported/Write:rw grading.
		OffsetHz:            codeplug.FreqField{State: codeplug.Unknown},
		ToneMode:            codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
		SatBandSwap:         codeplug.BoolField{State: codeplug.Unavailable},
		SatTrace:            codeplug.BoolField{State: codeplug.Unavailable},
		SatTraceRev:         codeplug.BoolField{State: codeplug.Unavailable},
	}, nil
}

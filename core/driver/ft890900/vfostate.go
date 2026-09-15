// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// readStatusUpdate performs one Status-Update read for u (bincat's
// U*-constants other than UMemoryRecord, which read.go's readRecord
// already covers) and returns the raw answer.
func (s *Session) readStatusUpdate(ctx context.Context, u byte) ([]byte, error) {
	cmd := bincat.NewCommand(bincat.OpStatusUpdate, [4]byte{u, 0, 0, 0})
	frame, err := s.eng.Do(ctx, cmd, bincat.ReadSpec(0, 1))
	if err != nil {
		return nil, fmt.Errorf("ft890900: %s: Status Update U=%d: %w", s.info.Name, u, err)
	}
	return frame, nil
}

// parseVFOSubRecord decodes one bare 9-byte VFO/Memory Data Record (the
// shape U=UBothVFOs' 18-byte reply carries twice, with no leading Memory
// Status Flags byte) by reusing bincat.ParseRecord: a synthetic leading
// flag byte and 9 bytes of trailing padding are prepended/appended so the
// buffer matches profile.RecordLen, since every offset ParseRecord reads
// (freq/clar/mode/tone/flags, all <= flagsOffset==9) falls entirely
// within the real 9 bytes copied in — the synthetic bytes are never
// touched. Ponytail: reuses the tested decoder rather than re-deriving
// the same offsets a second time.
func parseVFOSubRecord(sub9 []byte, profile bincat.Profile) (bincat.Record, error) {
	if len(sub9) != 9 {
		return bincat.Record{}, fmt.Errorf("ft890900: VFO sub-record is %d bytes, want 9", len(sub9))
	}
	full := make([]byte, profile.RecordLen)
	copy(full[1:10], sub9)
	return bincat.ParseRecord(full, profile)
}

// vfoContentFromRecord builds the codeplug.ChannelData buildVFOFrames
// needs to replay rec onto VFO-A. OffsetHz is always Unknown: no wire
// route reads the repeater-offset magnitude back at all (write.go's
// caps.go cross-reference) — RestoreVFOState's requireOffset=false
// tolerates that (write.go's buildVFOFrames doc comment).
func vfoContentFromRecord(rec bincat.Record) codeplug.ChannelData {
	return codeplug.ChannelData{
		FreqHz:    uint64(rec.FreqTensOfHz) * 10,
		Mode:      rec.Mode,
		Shift:     rec.Shift,
		ClarHz:    int(rec.ClarifierHz),
		RxClar:    rec.ClarifierHz != 0,
		CTCSSTone: toneField(rec.Tone),
		OffsetHz:  codeplug.FreqField{State: codeplug.Unknown},
	}
}

// SnapshotVFOState implements clone.VFOStateRestorer: it reads VFO-A's
// current content (U=UBothVFOs, the front 9-byte sub-record) and the
// operator's active-VFO selection.
//
// ACTIVE-VFO DETERMINATION IS ASSUMED (owner-probe): neither manual
// documents a flag naming which VFO is presently selected. This driver
// reads "current operating data" (U=UOperatingData) separately and treats
// an EXACT match of its front 9-byte span against VFO-A's own record as
// "A is active" — the write choreography's own step 1 (A/B-select) is
// what makes "current op" reflect whichever VFO was last selected, so a
// mismatch means B holds different content and is therefore the one
// currently live. This is a heuristic, not a documented fact, and errs
// towards reporting "B" (the safer default: RestoreVFOState will then
// re-select B explicitly, never silently leaving the operator on A when
// they were genuinely on B).
func (s *Session) SnapshotVFOState(ctx context.Context) (clone.VFOSnapshot, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	both, err := s.readStatusUpdate(ctx, bincat.UBothVFOs)
	if err != nil {
		return clone.VFOSnapshot{}, fmt.Errorf("ft890900: SnapshotVFOState: %w", err)
	}
	if len(both) != 18 {
		return clone.VFOSnapshot{}, fmt.Errorf("ft890900: SnapshotVFOState: VFO-A/B reply is %d bytes, want 18", len(both))
	}
	vfoARaw := both[0:9]

	current, err := s.readStatusUpdate(ctx, bincat.UOperatingData)
	if err != nil {
		return clone.VFOSnapshot{}, fmt.Errorf("ft890900: SnapshotVFOState: %w", err)
	}
	if len(current) != s.info.EngineProfile.RecordLen {
		return clone.VFOSnapshot{}, fmt.Errorf("ft890900: SnapshotVFOState: current-operating-data reply is %d bytes, want %d", len(current), s.info.EngineProfile.RecordLen)
	}

	rec, err := parseVFOSubRecord(vfoARaw, s.info.EngineProfile)
	if err != nil {
		return clone.VFOSnapshot{}, fmt.Errorf("ft890900: SnapshotVFOState: decoding VFO-A: %w", err)
	}

	active := "B"
	if bytes.Equal(current[1:10], vfoARaw) {
		active = "A"
	}

	return clone.VFOSnapshot{Content: vfoContentFromRecord(rec), ActiveVFO: active}, nil
}

// RestoreVFOState implements clone.VFOStateRestorer: it re-selects VFO-A,
// replays snapshot.Content onto it (best-effort: requireOffset=false, see
// buildVFOFrames' doc comment — a non-simplex snapshot recovers the shift
// DIRECTION but never the offset MAGNITUDE), and, if the operator had B
// active, re-selects B. A partial failure here is a journal warning, per
// core/clone's own doc comment — never surfaced as an Execute failure.
func (s *Session) RestoreVFOState(ctx context.Context, snapshot clone.VFOSnapshot) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	frames, err := buildVFOFrames(snapshot.Content, false)
	if err != nil {
		return fmt.Errorf("ft890900: RestoreVFOState: %w", err)
	}
	if _, err := s.sendFrames(ctx, frames); err != nil {
		return fmt.Errorf("ft890900: RestoreVFOState: %w", err)
	}

	if snapshot.ActiveVFO == "B" {
		cmd := bincat.NewCommand(bincat.OpABSelect, [4]byte{1, 0, 0, 0})
		if _, err := s.eng.Do(ctx, cmd, bincat.WriteSpec(0)); err != nil {
			return fmt.Errorf("ft890900: RestoreVFOState: re-selecting VFO-B: %w", err)
		}
	}
	return nil
}

var _ clone.VFOStateRestorer = (*Session)(nil)

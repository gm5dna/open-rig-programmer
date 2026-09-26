// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Image is one successfully received and matched clone-mode transfer:
// Raw is the COMPLETE image, byte-for-byte, destined for
// codeplug.RawImageBlob.Bytes (the whole image, never a trimmed subset —
// spec.md §Decisions item 7); Records is that same image sliced into
// Profile.ChannelCount raw per-channel record slices, each RecordWidth
// bytes wide, in wire order.
//
// Records are RAW BYTES ONLY here — decoding a record's bytes into a
// codeplug.Channel's fields is Phase 2's job, per family, informed-by
// CHIRP's own layout. This phase ships the slicing framework and proves it
// round-trips; it does not decode a single field.
type Image struct {
	Profile  Profile
	Raw      []byte
	Records  [][]byte
	Channels []DecodedChannel
}

// DecodedChannel is this package's OWN lightweight per-channel decode —
// FreqHz/Mode/Name/Empty only, informed-by CHIRP's own _get_memory, not
// manual-verified (spec.md §Evidence policy). It is deliberately NOT
// codeplug.Channel: this family has no registered core/driver capabilities
// (Write: Unsupported project-wide, no driver.Session implementation
// exists for it — spec.md §Read model point 7), so the capability-aware
// mapping (the CTCSS/Shift/TagDisplay/... vocabularies codeplug.ChannelData
// carries, each drawn from a per-radio spec.Capabilities this family does
// not have) belongs to whichever future phase registers those
// capabilities, not to this transport-and-parse package.
type DecodedChannel struct {
	FreqHz uint64
	Mode   string
	Name   string
	// Empty is true when this record's frequency reads as CHIRP's own
	// "unused" sentinel (0 or all-0xFF) — informed-by ft817.py's
	// _get_normal ("if not valid or _mem.freq == 0xffffffff"). This
	// package does not decode the separate visible/filled bitmap CHIRP
	// also consults; the frequency sentinel alone is what is checked here.
	Empty bool
}

// yaesuModes is CHIRP's own MODES list, informed-by ft817.py — only
// indices 0-7 are ever addressed directly by a record's 3-bit mode field;
// the narrow variants (NCW/NCWR/NFM) are derived from the is_cwdig_narrow/
// is_fm_narrow flag bits instead (see DecodeRecord), never indexed here.
var yaesuModes = []string{"LSB", "USB", "CW", "CWR", "AM", "FM", "DIG", "PKT"}

// Byte offsets within one clone-mode channel record that are constant
// across every Profile in the FT-817/FT-818/FT-857/FT-897 family —
// informed-by CHIRP's mem_struct, which places the mode byte and the
// duplex/narrow-flag byte identically in every variant this package
// models; only FreqOffset (Profile field) and the derived name offset vary
// per family.
const (
	yaesuModeOffset   = 0
	yaesuNarrowOffset = 1
	yaesuNameLen      = 8
)

// DecodeRecord decodes one RecordWidth-byte record using p's FreqOffset
// (Phase 2's per-family fact) — see DecodedChannel's doc comment for what
// is and is not decoded.
func (p Profile) DecodeRecord(rec []byte) (DecodedChannel, error) {
	nameOffset := p.FreqOffset + 8
	if p.FreqOffset < 0 || nameOffset+yaesuNameLen > len(rec) {
		return DecodedChannel{}, fmt.Errorf("%w: profile %s FreqOffset %d does not fit a %d-byte record", ErrRecord, p.Model, p.FreqOffset, len(rec))
	}

	rawFreq := binary.BigEndian.Uint32(rec[p.FreqOffset : p.FreqOffset+4])
	if rawFreq == 0 || rawFreq == 0xFFFFFFFF {
		return DecodedChannel{Empty: true}, nil
	}

	mode := "UNKNOWN"
	if idx := rec[yaesuModeOffset] & 0x07; int(idx) < len(yaesuModes) {
		mode = yaesuModes[idx]
	}
	narrow := rec[yaesuNarrowOffset]
	isCWDigNarrow := narrow&0x10 != 0
	isFMNarrow := narrow&0x08 != 0
	switch mode {
	case "FM":
		if isFMNarrow {
			mode = "NFM"
		}
	case "CW", "CWR":
		if isCWDigNarrow {
			mode = "N" + mode
		}
	}

	return DecodedChannel{
		FreqHz: uint64(rawFreq) * 10,
		Mode:   mode,
		Name:   decodeYaesuName(rec[nameOffset : nameOffset+yaesuNameLen]),
	}, nil
}

// decodeYaesuName decodes a name[8] field — informed-by ft817.py's
// _get_memory (CHARSET filter, 0xFF terminator, trailing-space trim). An
// unsupported byte is rendered "*", matching CHIRP's own substitution.
func decodeYaesuName(b []byte) string {
	var sb strings.Builder
	for _, c := range b {
		if c == 0xFF {
			break
		}
		if c >= 0x20 && c < 0x7F {
			sb.WriteByte(c)
		} else {
			sb.WriteByte('*')
		}
	}
	return strings.TrimRight(sb.String(), " ")
}

// ErrRecord is the sentinel every record-slicing failure wraps.
var ErrRecord = fmt.Errorf("clonewire: record")

// ParseImage slices raw into p's records, once Receive has already
// confirmed raw is the one candidate Profile that matches. raw must be
// exactly p.ImageLen bytes; a shorter or longer slice is a caller bug, not
// a wire condition Receive would let through unrefused, so it is reported
// via ErrRecord rather than one of the wire-refusal sentinels in errors.go.
//
// p.ChannelCount == 0 is not an error: it means this Profile carries no
// sliced records (Phase 1's own fixtures, or a family Phase 2 has not yet
// filled the record geometry in for) — ParseImage still returns the whole
// raw Image, with a nil Records slice.
func ParseImage(raw []byte, p Profile) (Image, error) {
	if len(raw) != p.ImageLen {
		return Image{}, fmt.Errorf("%w: image is %d bytes, want %d for profile %s", ErrRecord, len(raw), p.ImageLen, p.Model)
	}

	img := Image{
		Profile: p,
		Raw:     append([]byte(nil), raw...),
	}
	if p.ChannelCount == 0 {
		return img, nil
	}
	if p.RecordWidth <= 0 {
		return Image{}, fmt.Errorf("%w: profile %s has %d channels but RecordWidth <= 0", ErrRecord, p.Model, p.ChannelCount)
	}
	if p.RecordOffset < 0 || p.RecordOffset+p.ChannelCount*p.RecordWidth > len(raw) {
		return Image{}, fmt.Errorf("%w: profile %s wants %d records of %d bytes at offset %d, image is only %d bytes", ErrRecord, p.Model, p.ChannelCount, p.RecordWidth, p.RecordOffset, len(raw))
	}

	records := make([][]byte, p.ChannelCount)
	var channels []DecodedChannel
	if p.FreqOffset > 0 {
		channels = make([]DecodedChannel, p.ChannelCount)
	}
	for i := range records {
		off := p.RecordOffset + i*p.RecordWidth
		rec := make([]byte, p.RecordWidth)
		copy(rec, raw[off:off+p.RecordWidth])
		records[i] = rec
		if channels != nil {
			dc, err := p.DecodeRecord(rec)
			if err != nil {
				return Image{}, err
			}
			channels[i] = dc
		}
	}
	img.Records = records
	img.Channels = channels
	return img, nil
}

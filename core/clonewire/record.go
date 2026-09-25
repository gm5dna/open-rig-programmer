// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import "fmt"

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
	Profile Profile
	Raw     []byte
	Records [][]byte
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
	if p.ChannelCount*p.RecordWidth > len(raw) {
		return Image{}, fmt.Errorf("%w: profile %s wants %d records of %d bytes, image is only %d bytes", ErrRecord, p.Model, p.ChannelCount, p.RecordWidth, len(raw))
	}

	records := make([][]byte, p.ChannelCount)
	for i := range records {
		off := i * p.RecordWidth
		rec := make([]byte, p.RecordWidth)
		copy(rec, raw[off:off+p.RecordWidth])
		records[i] = rec
	}
	img.Records = records
	return img, nil
}

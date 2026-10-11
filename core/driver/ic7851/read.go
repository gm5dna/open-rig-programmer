// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"context"
	"errors"
	"fmt"

	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
)

// ErrFixedDigit is the sentinel for a record whose printed-fixed digit
// bytes carry something other than zero.
var ErrFixedDigit = errors.New("ic7851: a record byte the document prints as a fixed zero carried a digit")

// FixedDigitError reports that refusal, naming the byte.
//
// WHY THE READ PATH CHECKS AT ALL. core/civ/ic7851's layout deliberately
// leaves ⑧, ⑫ and ⑮ OUTSIDE their neighbouring numeric spans, because
// civ.FieldSpan carries no numeric domain and a span covering one of them
// would let the builder and the gate write a digit into a byte matrix
// §3.16.3 and §3.16.4 print as fixed zeros. The cost of that exclusion is
// that the record PARSER no longer reads those bytes either — so a record
// carrying 01 in ⑧ would decode as a frequency 100 MHz lower than the one
// on the wire, and WriteChannel would send it back with the byte silently
// zeroed. Both are outcomes a caller cannot tell from success, which is
// why the record is refused here instead.
//
// IT IS NOT AN E6 REFUSAL and does not share UnmappedRegionError. E6 is
// about regions this radio genuinely uses and this programme declines to
// map — the SELECT-group marker and the data mode — and its refusal is a
// WRITE refusal on a legitimate channel. This one says the record is not
// the shape the document draws at all, and it refuses the READ.
type FixedDigitError struct {
	// Offset is the 0-based record byte.
	Offset int
	// Got is what that byte carried; the document prints 0.
	Got byte
	// Printed is the document's own index for the byte.
	Printed string
}

func (e *FixedDigitError) Error() string {
	return fmt.Sprintf(
		"ic7851: this record's byte %d (printed %s) carries %#02x, and the document draws both its nibbles as a literal fixed 0 — the record is refused rather than read, because the profile maps no span over that byte and a digit there would be read as a value 100 times smaller and written back with the byte zeroed (register entries ic7851-fixed-nibble-reencode and ic7851-tone-fixed-byte)",
		e.Offset, e.Printed, e.Got)
}

// Unwrap lets errors.Is(err, ErrFixedDigit) match.
func (e *FixedDigitError) Unwrap() error { return ErrFixedDigit }

// fixedDigitsDiffer reports the first printed-fixed byte carrying a digit.
//
// The three offsets are named by core/civ/ic7851 rather than written out
// here, so the layout that excludes them and the read that refuses them
// cannot come to disagree about which bytes they are.
// TestFixedDigitBytesAreRefusedOnRead pins all three.
func fixedDigitsDiffer(raw []byte) error {
	for _, chk := range []struct {
		offset  int
		printed string
	}{
		{civic7851.FreqFixedOffset, "⑧"},
		{civic7851.ToneTXFixedOffset, "⑫"},
		{civic7851.ToneRXFixedOffset, "⑮"},
	} {
		if chk.offset >= len(raw) {
			// Unreachable: the length fingerprint has already refused any
			// record but this profile's own.
			continue
		}
		if raw[chk.offset] != 0 {
			return &FixedDigitError{Offset: chk.offset, Got: raw[chk.offset], Printed: chk.printed}
		}
	}
	return nil
}

// AnswerMismatches reports how many memory answers this session has seen
// whose decoded channel address was not the one requested (tier ruling
// T2). A diagnostic count beside the typed error, so a bus that
// occasionally mis-attributes is visible even when each individual read
// was refused correctly.
func (s *Session) AnswerMismatches() uint64 { return s.answerMismatches.Load() }

// ReadChannel implements driver.Session: ONE 1A 00 read, mapped into one
// codeplug.Channel by the shared engine.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	return icom.ReadChannel(ctx, &params, s.eng, s.caps, &s.answerMismatches, slot)
}

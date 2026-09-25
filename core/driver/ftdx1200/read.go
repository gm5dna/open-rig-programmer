// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// ctcssVocab is the shared 3-value CTCSS legend-order vocabulary.
var ctcssVocab = yaesu.CTCSSVocab3

// mrParams is this radio's yaesu.MRParams value: the fixed 27-byte MR
// frame (matrix §1.1), the two-value read-side kind check (matrix §1.4),
// and CTCSSTone coming back Unavailable ALWAYS — unlike the sibling
// ftdx3000, P9 is printed-fixed on this radio's READ side too (matrix
// §1.3), so there is no live tone state to report. There is no MT to
// sequence beside a read or write (matrix §0): the shared body itself
// supplies Tag/TagDisplay/ScanSkip's fixed values, and FieldCTCSSTone is
// never requested on write (ToneNeverRequested) since P9 is
// printed-fixed on both directions here.
var mrParams = yaesu.MRParams{
	Name:          "ftdx1200",
	MRAnswerLen:   27,
	CTCSS:         ctcssVocab,
	AcceptedKinds: []byte{cat.KindVFO, cat.KindMemory},
	ToneRead:      yaesu.ToneUnavailable,
	ToneWrite:     yaesu.ToneNeverRequested,
	WriteKind:     func(d cat.Dialect) byte { return d.MWWriteKind() },
	EraseReason:   "erase cannot be expressed by the CAT codec (no erase/clear command exists for a memory channel), and FieldErase is not write-Supported",
}

// ReadChannel implements driver.Session: a single MR read via the shared
// core/driver/internal/yaesu body.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	return yaesu.ReadChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, slot)
}

// KindMismatchError reports that an MR answer's P7 kind byte was not one
// of this radio's accepted read-side values. The shared form
// (yaesu.KindMismatchError) carries the model name so this package needs
// no typed error of its own.
type KindMismatchError = yaesu.KindMismatchError

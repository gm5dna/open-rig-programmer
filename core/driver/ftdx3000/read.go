// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx3000

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
// and CTCSSTone coming back KNOWN — matrix §1.3: P9's read side is a live
// two-digit index into the standard 50-entry CTCSS tone chart, even
// though the write side is fixed (dialect.go's P9ToneIndexReadOnly). A
// write nonetheless requests FieldCTCSSTone whenever the channel's tone
// is Known (ToneOptionalIfKnown, same as ft2000/ft450d/ft950): this
// radio's own caps.go marks the field write-Unsupported, so the shared
// body's capability gate refuses such a write BEFORE buildMWCommand ever
// runs — the asymmetry lives in caps.go, untouched by this milestone, not
// here (spec v2 §2/§6). There is no MT to sequence beside a read or
// write (matrix §0): Tag/TagDisplay/ScanSkip are the shared body's own
// fixed values.
var mrParams = yaesu.MRParams{
	Name:          "ftdx3000",
	MRAnswerLen:   27,
	CTCSS:         ctcssVocab,
	AcceptedKinds: []byte{cat.KindVFO, cat.KindMemory},
	ToneRead:      yaesu.ToneLiveKnown,
	ToneWrite:     yaesu.ToneOptionalIfKnown,
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

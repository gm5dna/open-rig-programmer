// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// ctcssVocab is the shared 3-value CTCSS legend-order vocabulary.
var ctcssVocab = yaesu.CTCSSVocab3

// mrParams is this family's yaesu.MRParams value, shared by both rows
// (FT-2000/FT-2000D — matrix §4: byte-identical on every position but
// CATID, which lives in the session's own dialect, not here): the fixed
// 27-byte MR frame (matrix §1.1), the two-value read-side kind check
// (matrix §1.4 — NO FT-2000 OR FT-2000D HAS EVER BEEN ASKED ANYTHING BY
// THIS PROJECT, so there is no hardware finding to widen this against),
// and CTCSSTone coming back LIVE-KNOWN: P9 is a live two-digit index into
// the standard 50-entry CTCSS tone chart (matrix §1.3), the first memory
// record in this fleet to carry one at all. Model is left unset: the
// shared body's own fallback (dialect.CATID()) is exactly what this
// package's AnswerMismatchError has always used — s.dialect resolves to
// whichever row (FT-2000/FT-2000D) this session opened as. There is no MT
// to sequence beside a read or write (matrix §0): this family has no
// tag/name route over CAT at all, and the shared body itself supplies
// Tag/TagDisplay/ScanSkip's fixed values.
var mrParams = yaesu.MRParams{
	Name:          "ft2000",
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

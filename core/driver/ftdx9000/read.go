// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// ctcssVocab is the shared 3-value CTCSS legend-order vocabulary
// (matrix §1.16 — the same three every sibling driver uses).
var ctcssVocab = yaesu.CTCSSVocab3

// mrAnswerLen is this radio's whole MR-answer/MW-set frame length: 27
// bytes (matrix §2, Lift Y's MemoryFrameLen). Written down here rather
// than derived, for the same reason ft891/read.go gives: core/cat exposes
// no accessor for the shared block's width.
const mrAnswerLen = 27

// mrParams is this radio's yaesu.MRParams value: no P7 kind check
// (AcceptedKinds nil — matrix has none, unlike the 4-rig VFO/Memory
// family), a live CTCSSTone that the write side then REQUIRES Known
// (ToneRequiredKnown — P9 is a live byte on every MW frame with no
// "leave it alone" encoding, matrix §1.10), SkipTierFields (the 27-byte
// record has no room for any of the 17 Icom-tier fields, and this
// radio's write has never asked after them — spec-v2 finding 9, pinned
// by write_test.go's TestWriteChannel_IcomTierFieldKnown), and
// ScanSkipUnavailable (no scan-skip byte in the record either). Model is
// set explicitly to modelName: this radio's AnswerMismatchError has
// always named the registry spelling ("FTdx9000"), not one of
// dialect.CATID()'s three sub-variant IDs, which the shared body's
// unset-Model fallback would silently switch to here. THIS RADIO HAS NO
// MT COMMAND (matrix §2; write.go, ftdx9000.go's params) so, unlike
// every 4-rig MT-family sibling, there is no second (tag) exchange to
// sequence beside the read, and the shared body itself supplies
// Tag/TagDisplay's fixed values.
var mrParams = yaesu.MRParams{
	Name:                "ftdx9000",
	Model:               modelName,
	MRAnswerLen:         mrAnswerLen,
	CTCSS:               ctcssVocab,
	AcceptedKinds:       nil,
	ToneRead:            yaesu.ToneLiveKnown,
	ToneWrite:           yaesu.ToneRequiredKnown,
	WriteKind:           func(d cat.Dialect) byte { return d.MWWriteKind() },
	EraseReason:         "erase cannot be expressed by this CAT codec, and FieldErase is not write-Supported",
	SkipTierFields:      true,
	ScanSkipUnavailable: true,
}

// ReadChannel implements driver.Session: a single MR read via the shared
// core/driver/internal/yaesu body.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	return yaesu.ReadChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, slot)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// mrAnswerLen is this radio's whole MR-answer/MW-set frame length: 27
// bytes (matrix §1.1). Written down here rather than derived, for the same
// reason ftdx9000/read.go gives: core/cat exposes no accessor for the
// shared block's width.
const mrAnswerLen = 27

// mrParams is this radio's yaesu.MRParams value.
//
// AcceptedKinds nil: matrix has no P7 read check at all, unlike the 4-rig
// VFO/Memory family (matrix §2). ToneRead/ToneWrite ToneLiveKnown/
// ToneRequiredKnown: P9 is a LIVE two-digit CTCSS tone-table index on both
// MR and MW (doc.go), unlike every registered sibling's fixed "00" —
// unlike ftdx1200's ToneUnavailable read but like ftdx9000's read, and
// ftdx5000's write then REQUIRES it Known, since there is no "leave it
// alone" encoding for a byte that is always on the wire. WriteKind is a
// func hardcoded to cat.KindVFO (Q2), NOT dialect.MWWriteKind(), even
// though the dialect's own MWWriteKind happens to equal cat.KindVFO too
// (dialect.go:143) — the hardcode is a deliberate, kept design choice
// (spec-rulings Q2), not a value the migration derives from the dialect.
// RequestConditionalTagFields/ExplicitTagRefusal: this radio has no tag,
// display or scan-skip route at all (matrix §2/§3 NoTag; no field after
// P10 Shift; no scan-skip byte), so a write must ask after Tag/TagDisplay/
// ScanSkip only when the channel actually carries one, and must refuse a
// non-empty Tag / Known TagDisplay explicitly rather than let the generic
// capability gate's wording stand in for it. ScanSkipUnavailable: the
// 27-byte record has no scan-skip byte, a positive statement, not an open
// question. Model is set explicitly to modelName ("FTdx5000"), matching
// ftdx9000: this radio's AnswerMismatchError has always named the
// registry spelling, not dialect.CATID()'s wire ID ("0362"), which the
// shared body's unset-Model fallback would silently switch to here.
var mrParams = yaesu.MRParams{
	Name:                        "ftdx5000",
	Model:                       modelName,
	MRAnswerLen:                 mrAnswerLen,
	CTCSS:                       ctcssVocab,
	AcceptedKinds:               nil,
	ToneRead:                    yaesu.ToneLiveKnown,
	ToneWrite:                   yaesu.ToneRequiredKnown,
	WriteKind:                   func(cat.Dialect) byte { return cat.KindVFO },
	EraseReason:                 eraseReason,
	ExplicitTagRefusal:          true,
	RequestConditionalTagFields: true,
	ScanSkipUnavailable:         true,
}

// ReadChannel implements driver.Session: a single MR read via the shared
// core/driver/internal/yaesu body. Unlike every other registered Yaesu
// driver this is NOT an MT read — this radio has no MT command at all
// (doc.go) — so, same as ftdx9000, there is no second (tag) exchange to
// sequence beside the read.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	return yaesu.ReadChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, slot)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ft950

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// ctcssVocab is this radio's 3-value CTCSS state vocabulary, in legend
// order — matches every registered sibling's.
var ctcssVocab = []yaesu.CTCSSName{
	{Name: "OFF", State: cat.CTCSSOff},
	{Name: "ENC-DEC", State: cat.CTCSSEncDec},
	{Name: "ENC", State: cat.CTCSSEnc},
}

// mrParams is this radio's yaesu.MRParams value: the fixed 27-byte MR
// frame (matrix §1.1), the two-value read-side kind check (matrix §1.4,
// layout:857's "0: VFO 1: Memory" — NO FT-950 HAS EVER BEEN ASKED
// ANYTHING BY THIS PROJECT, so there is no hardware finding to widen this
// against), and CTCSSTone coming back LIVE-KNOWN: P9 is a live two-digit
// index into the standard 50-entry CTCSS tone chart (matrix §1.4) — the
// same chart caps.go's standardTones sets as CTCSSTones, so the shared
// body's ToneLiveKnown lookup (caps.CTCSSTones[m.ToneIndex]) is exactly
// caps.go's own toneForIndex table, addressed the shared way. Model is
// set explicitly to modelName ("FT-950", the manual's own cover-page
// name): UNLIKE ft2000/ftdx1200/ftdx3000, this radio's AnswerMismatchError
// has always named the cover-page model, not dialect.CATID() — the shared
// body's unset-Model fallback would silently change that here. There is
// no MT to sequence beside a read or write (matrix §0): this radio has no
// tag/name route over CAT at all, and the shared body itself supplies
// Tag/TagDisplay/ScanSkip's fixed values.
var mrParams = yaesu.MRParams{
	Name:          "ft950",
	Model:         modelName,
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

// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// ctcssVocab is the shared 3-value CTCSS legend-order vocabulary.
var ctcssVocab = yaesu.CTCSSVocab3

// mrParams is this radio's yaesu.MRParams value: the fixed 27-byte MR
// frame (matrix §1.1), the two-value read-side kind check (matrix §1.4,
// the MR answer's own legend printing "0: VFO 1: Memory", layout:796 —
// NO FT-450D HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT, so there is no
// hardware finding to widen this against, on MEM or on PMS alike), and
// CTCSSTone coming back LIVE-KNOWN: P9 is a live two-digit index into the
// standard 50-entry CTCSS tone chart (matrix §1.3) — the same chart
// caps.go's standardTones sets as CTCSSTones, so the shared body's
// ToneLiveKnown lookup (caps.CTCSSTones[m.ToneIndex]) is exactly caps.go's
// own toneForIndex table, addressed the shared way. Model is set
// explicitly to modelName ("FT-450D"): this radio's AnswerMismatchError
// has always named the cover-page model, not dialect.CATID() ("0244") —
// the shared body's unset-Model fallback would silently change that here.
// There is no MT to sequence beside a read or write (matrix §0): this
// radio has no tag/name route over CAT at all, and the shared body itself
// supplies Tag/TagDisplay/ScanSkip's fixed values.
//
// Reading a PMS slot (501-504) uses the SAME MR frame as MEM — only the
// write side is restricted (caps.go's pmsFields); this shared body makes
// no bank distinction at all on the read side, matching the per-driver
// body it replaces.
var mrParams = yaesu.MRParams{
	Name:          "ft450d",
	Model:         modelName,
	MRAnswerLen:   mrAnswerLen,
	CTCSS:         ctcssVocab,
	AcceptedKinds: []byte{cat.KindVFO, cat.KindMemory},
	ToneRead:      yaesu.ToneLiveKnown,
	ToneWrite:     yaesu.ToneOptionalIfKnown,
	WriteKind:     func(d cat.Dialect) byte { return d.MWWriteKind() },
	EraseReason:   "erase cannot be expressed by the CAT codec (no erase/clear command exists for a memory channel), and FieldErase is not write-Supported",
}

// mrAnswerLen is this radio's whole MR-answer/MW-set frame length: 27
// bytes (matrix §1.1, Lift Y's MemoryFrameLen).
const mrAnswerLen = 27

// ReadChannel implements driver.Session: a single MR read via the shared
// core/driver/internal/yaesu body.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	return yaesu.ReadChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, slot)
}

// KindMismatchError reports that an MR answer's P7 kind byte was not one
// of this radio's accepted read-side values (mrParams.AcceptedKinds,
// above). The shared form (yaesu.KindMismatchError) carries the model
// name so this package needs no typed error of its own.
type KindMismatchError = yaesu.KindMismatchError

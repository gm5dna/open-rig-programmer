// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ctcssNames maps the wire CTCSS state to codeplug's display spelling
// ("OFF", "ENC-DEC", "ENC" — the strings codeplug.Validate checks for, and
// the ones this driver's own Capabilities.CTCSSStates advertises).
// Deliberately NOT cat.CTCSSState.String(), whose spellings ("off",
// "ENC/DEC") are log labels rather than model values.
//
// Three states and no more: this radio's P8 legend prints exactly "0: CTCSS
// \"OFF\" 1: CTCSS ENC/DEC 2: CTCSS ENC" in five frame blocks (MR at layout
// 1291, MT 1325, MW 1365, OI 1449; IF at 1095 with its off state
// abbreviated), none of them model-qualified — matrix §1.15.
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:    "OFF",
	cat.CTCSSEncDec: "ENC-DEC",
	cat.CTCSSEnc:    "ENC",
}

// shiftNames maps the wire shift state to codeplug's display spelling
// ("SIMPLEX", "PLUS", "MINUS"), from this radio's own P10 legend "0:
// Simplex 1: Plus Shift 2: Minus Shift" (five frame blocks: IF at layout
// 1097, MR 1294, MT 1327, MW 1367, OI 1452 — matrix §1.14).
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// mtSpec builds the transport spec for a combined MT read against d: the
// prefix, the EXACT answer length, and one retry (a read is idempotent, so
// a single swallowed reply should not fail an operation).
//
// THE LENGTH IS DERIVED FROM THE DIALECT, never written here. d's own
// MTAnswerBounds reports the geometry its declared MT form and tag width
// imply — for the FTdx101's combined form that is an exact 41, being
// 29 + TagMaxBytes — and there is deliberately no 41 in this package's
// production code at all. Two reasons, both load-bearing: a literal would
// silently keep answering 41 for a dialect whose tag field was a different
// width, and the combined answer's exactness is itself an ASSUMPTION the
// dialect carries — its register entry "The combined MT answer's EXACT
// length (consumed here as MTAnswerBounds() = (41, 41))" — whose recorded
// Stage R contingency, PER MODEL, is a 30..41 WINDOW. If that contingency
// is ever taken for either radio, the bounds move in core/cat and this spec
// moves with them.
//
// The equal-bounds check is what makes that safe rather than lucky:
// transport.CommandSpec.ExpectLen is a single exact length, so a dialect
// reporting a genuine window has no honest ExpectLen and gets an error
// instead of its window's top silently becoming a hard requirement (which
// would reject every shorter answer as unmatched, i.e. as a timeout).
func mtSpec(d cat.Dialect) (transport.CommandSpec, error) {
	lo, hi, err := d.MTAnswerBounds()
	if err != nil {
		return transport.CommandSpec{}, fmt.Errorf("ftdx101: MT answer geometry: %w", err)
	}
	if lo != hi {
		return transport.CommandSpec{}, fmt.Errorf("ftdx101: MT answer geometry: this dialect reports a %d..%d length WINDOW, but transport.CommandSpec.ExpectLen is a single exact length — a windowed answer needs a spec that expresses the window, not its top", lo, hi)
	}
	return transport.CommandSpec{ExpectPrefix: "MT", ExpectLen: hi, RetryReads: 1}, nil
}

// ReadChannel implements driver.Session: ONE combined MT read, mapped into
// one codeplug.Channel.
//
// MT-ONLY, and MR is never sent — see doc.go, "MR is deliberately unused"
// (matrix §3.5). The 41-byte combined answer carries the whole field block
// AND the tag in a single frame, so it is an ATOMIC snapshot of the
// channel: the two-frame stitch the FT-710's MR+MT read has to guard
// against (field block read from one radio state, tag from a later one) is
// structurally impossible here rather than merely locked against.
//
// The empty-slot rule: a "?;" rejection is mapped to an EMPTY channel (Data
// nil, the slot carried through), not an error. ASSUMED — register entry 8:
// "?;" is the protocol's single unattributed NAK, so reading "empty" out of
// it is an interpretation, and neither FTdx101's combined-MT read of an
// empty channel has ever been observed. The FT-710's equivalent was
// verified for ITS MR read, which is a different frame on a different
// radio.
//
// TagDisplay comes back codeplug.Unavailable, ALWAYS: the combined record
// has no display flag at all — P11 is "0: (Fixed)" at layout 1329, and the
// frame's 41 positions are fully accounted for (matrix §3.7, a
// MANUAL-EVIDENCED ABSENCE rather than an assumption) — so there is no
// value to report and Unknown would be the wrong word for it. Unknown means
// "the radio has one and this read did not learn it"; Unavailable means
// "there is no such field". Every downstream path already handles it: Diff
// excludes it from the plan, the grid refuses to toggle it, csvio spells
// it. It is also the NAMED INVERSION the write path must carry — on the
// FT-710 a non-Known TagDisplay is a write REFUSAL, and here it must be
// acceptable, because the frame has nowhere to put a display flag.
//
// CTCSSTone and ScanSkip come back codeplug.Unknown, ALWAYS: register entry
// 6. "Unknown" means "preserve whatever the radio has" to every write path
// downstream, which is the only honest instruction for a field this driver
// cannot see.
//
// KIND CHECKING IS THE PARSER'S, not this driver's.
// cat.Dialect.ParseMTAnswerCombined narrows P7 to the combined record's own
// documented read pair {'0' VFO, '1' Memory} — MT's P7 legend reads "Set:
// 0: (Fixed) / Read: 0: VFO 1: Memory" (layout 1324) — so an
// out-of-vocabulary byte comes back as a *cat.ParseError, wrapped here with
// the slot that was being read. No per-class narrowing is added on top, and
// this manual's SIX-value P7 (IF's and OI's, layout 1092-1093 and
// 1447-1448) is deliberately not read across into MT's: see doc.go.
//
// Error typing, both classes: the PARSE failures above stay *cat.ParseError
// under a wrap (errors.As finds them, and the wrap adds the slot the bare
// parser could not know); the SLOT-ECHO check raises this driver's own
// typed *AnswerMismatchError. Neither is a bare fmt.Errorf, so a caller can
// tell "the radio said something malformed" from "the radio answered about
// the wrong channel".
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx101: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMTRead(sl)
	if err != nil {
		// e.g. the answer-only none form — the DIALECT register's entry
		// "SlotSpace.NoneWire = \"000\"", ASSUMED because that form appears
		// in no FTdx101 slot legend: grammatical per ParseSlot, never a
		// legal read target.
		return codeplug.Channel{}, fmt.Errorf("ftdx101: ReadChannel: %w", err)
	}

	cmdSpec, err := mtSpec(s.dialect)
	if err != nil {
		return codeplug.Channel{}, err
	}

	frame, err := s.eng.Do(ctx, cmd, cmdSpec)
	if errors.Is(err, cat.ErrRejected) {
		// ASSUMED empty slot — register entry 8.
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx101: ReadChannel %s: MT: %w", sl.Wire(), err)
	}

	m, tag, err := s.dialect.ParseMTAnswerCombined(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx101: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		// Unreachable after the parser's own CTCSS validation; refuse
		// rather than silently mislabel if it ever is not.
		return codeplug.Channel{}, fmt.Errorf("ftdx101: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx101: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz: m.FreqHz,
			// Rendered through THIS SESSION'S dialect, not cat.Mode.String:
			// the string is user-visible (it lands in the codeplug, the CLI
			// listing and the GUI grid), so it must come from the mode table
			// of the radio that answered — which for this package means the
			// dialect of the model that Opened, not a package default.
			// ModeName gives the display name; the odd-state cat.ModeUnset
			// renders "-" and is mapped through faithfully —
			// codeplug.Validate flags it as not a selectable mode, which is
			// the right outcome for a placeholder this radio's legends do
			// not list (the DIALECT register's "cat.ModeUnset member of the
			// mode table" entry).
			Mode: s.dialect.ModeName(m.Mode),
			// The magnitude is manual-evidenced (P3's four digits at
			// positions 16-19, 0000-9990 Hz); the byte that carries a
			// NEGATIVE sign is not — the DIALECT register's entry "The
			// CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D
			// ('-')", which this manual prints as a two-hyphen glyph and the
			// golden deriver recorded UNREADABLE. core/cat accepts only '+'
			// or '-' when reading the sign position, so a radio using some
			// other byte would fail the parse loudly here rather than
			// silently reading a negative offset as positive.
			ClarHz: int(m.ClarHz),
			RxClar: m.RxClar,
			TxClar: m.TxClar,
			CTCSS:  ctcss,
			// Register entry 6: no tone number is readable.
			CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
			Shift:     shift,
			Tag:       tag,
			// UNAVAILABLE, not Unknown and never Known: this radio's
			// combined record has no display flag. See the doc comment.
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			// Register entry 6: no scan-skip flag is readable.
			ScanSkip: codeplug.BoolField{State: codeplug.Unknown},
		},
	}, nil
}

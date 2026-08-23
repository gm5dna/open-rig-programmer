// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

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
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:    "OFF",
	cat.CTCSSEncDec: "ENC-DEC",
	cat.CTCSSEnc:    "ENC",
}

// shiftNames maps the wire shift state to codeplug's display spelling
// ("SIMPLEX", "PLUS", "MINUS").
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
// imply — for the FTdx10's combined form that is an exact 41, being
// 29 + TagMaxBytes — and there is deliberately no 41 in this package at
// all. Two reasons, both load-bearing: a literal would silently keep
// answering 41 for a dialect whose tag field was a different width, and
// the combined answer's exactness is itself an ASSUMPTION the dialect
// carries (its register entry "the combined MT answer's EXACT length"),
// whose recorded Stage R contingency is a 30..41 WINDOW. If that
// contingency is ever taken, the bounds move in core/cat and this spec
// moves with them.
//
// The equal-bounds check is what makes that safe rather than lucky:
// transport.CATReadSpec takes a single exact length, so a dialect
// reporting a genuine window has no honest exact length and gets an error
// instead of its window's top silently becoming a hard requirement (which
// would reject every shorter answer as unmatched, i.e. as a timeout).
func mtSpec(d cat.Dialect) (transport.CommandSpec, error) {
	lo, hi, err := d.MTAnswerBounds()
	if err != nil {
		return transport.CommandSpec{}, fmt.Errorf("ftdx10: MT answer geometry: %w", err)
	}
	if lo != hi {
		return transport.CommandSpec{}, fmt.Errorf("ftdx10: MT answer geometry: this dialect reports a %d..%d length WINDOW, but transport.CATReadSpec takes a single exact length — a windowed answer needs a spec that expresses the window, not its top", lo, hi)
	}
	return transport.CATReadSpec("MT", hi, 1), nil
}

// ReadChannel implements driver.Session: ONE combined MT read, mapped into
// one codeplug.Channel.
//
// MT-ONLY, and MR is never sent — see doc.go, "MR is deliberately unused".
// The 41-byte combined answer carries the whole field block AND the tag in
// a single frame, so it is an ATOMIC snapshot of the channel: the
// two-frame stitch the FT-710's MR+MT read has to guard against (field
// block read from one radio state, tag from a later one) is structurally
// impossible here rather than merely locked against.
//
// The empty-slot rule: a "?;" rejection is mapped to an EMPTY channel
// (Data nil, the slot carried through), not an error. ASSUMED — register
// "?;" ON A COMBINED-MT READ OF AN EMPTY SLOT entry: "?;" is the
// protocol's single unattributed NAK, so reading
// "empty" out of it is an interpretation, and this radio's combined-MT
// read of an empty channel has never been observed. The FT-710's
// equivalent was verified for ITS MR read, which is a different frame on a
// different radio.
//
// TagDisplay comes back codeplug.Unavailable, ALWAYS: the combined record
// has no display flag at all (see caps.go's bankFields — a manual fact,
// not an assumption), so there is no value to report and Unknown would be
// the wrong word for it. Unknown means "the radio has one and this read
// did not learn it"; Unavailable means "there is no such field". The
// FTdx10 is this project's first radio to produce the latter, and every
// downstream path already handles it: Diff excludes it from the plan, the
// grid refuses to toggle it, csvio spells it.
//
// CTCSSTone and ScanSkip come back codeplug.Unknown, ALWAYS: register
// TONE AND SCAN-SKIP UNREACHABILITY entry. "Unknown" means "preserve
// whatever the radio has" to every
// write path downstream, which is the only honest instruction for a field
// this driver cannot see.
//
// KIND CHECKING IS THE PARSER'S, not this driver's.
// cat.Dialect.ParseMTAnswerCombined narrows P7 to the combined record's
// own documented read pair {'0' VFO, '1' Memory}, so an
// out-of-vocabulary byte — a '2', or the '4' placeholder MR would accept —
// comes back as a *cat.ParseError, wrapped here with the slot that was
// being read. No per-class narrowing is added on top: see doc.go.
//
// Error typing, both classes: the PARSE failures above stay *cat.ParseError
// under a wrap (errors.As finds them, and the wrap adds the slot the bare
// parser could not know); the SLOT-ECHO check raises this driver's own
// typed *AnswerMismatchError. Neither is a bare fmt.Errorf, so a caller
// can tell "the radio said something malformed" from "the radio answered
// about the wrong channel".
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx10: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMTRead(sl)
	if err != nil {
		// e.g. the answer-only none form ("000" for this dialect —
		// its own ASSUMED register entry SlotSpace.NoneWire): grammatical
		// per ParseSlot,
		// never a legal read target.
		return codeplug.Channel{}, fmt.Errorf("ftdx10: ReadChannel: %w", err)
	}

	cmdSpec, err := mtSpec(s.dialect)
	if err != nil {
		return codeplug.Channel{}, err
	}

	frame, err := s.eng.Do(ctx, cmd, cmdSpec)
	if errors.Is(err, cat.ErrRejected) {
		// ASSUMED empty slot — the "?;" ON A COMBINED-MT READ OF AN EMPTY
		// SLOT register entry.
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx10: ReadChannel %s: MT: %w", sl.Wire(), err)
	}

	m, tag, err := s.dialect.ParseMTAnswerCombined(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx10: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		// Unreachable after the parser's own CTCSS validation; refuse
		// rather than silently mislabel if it ever is not.
		return codeplug.Channel{}, fmt.Errorf("ftdx10: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx10: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz: m.FreqHz,
			// Rendered through THIS SESSION'S dialect, not
			// cat.Mode.String: the string is user-visible (it lands in
			// the codeplug, the CLI listing and the GUI grid), so it must
			// come from the mode table of the radio that answered.
			// ModeName gives the display name; the odd-state
			// cat.ModeUnset renders "-" and is mapped through faithfully
			// — codeplug.Validate flags it as not a selectable mode,
			// which is the right outcome for a placeholder this radio's
			// legends do not even list (the dialect's register entry for the
			// cat.ModeUnset table member).
			Mode:   s.dialect.ModeName(m.Mode),
			ClarHz: int(m.ClarHz),
			RxClar: m.RxClar,
			TxClar: m.TxClar,
			CTCSS:  ctcss,
			// The TONE AND SCAN-SKIP UNREACHABILITY entry: no tone number is
			// readable.
			CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
			Shift:     shift,
			Tag:       tag,
			// UNAVAILABLE, not Unknown and never Known: this radio's
			// combined record has no display flag. See the doc comment.
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			// The TONE AND SCAN-SKIP UNREACHABILITY entry: no scan-skip flag
			// is readable.
			ScanSkip: codeplug.BoolField{State: codeplug.Unknown},
		},
	}, nil
}

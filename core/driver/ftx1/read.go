// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mtSpec builds the transport spec for an MT read against d: the prefix,
// the EXACT answer length, and one retry. THE LENGTH IS DERIVED FROM THE
// DIALECT (yaesu.MTSpec, shared with ftdx10/ft991a's own mtSpec — see
// their doc comments): d's own MTAnswerBounds reports the geometry its
// declared MT form implies — for MTFormShortNoDisplay that is an exact
// 20, 2 + SlotDigits(5) + TagMaxBytes(12) + 1 (mtnodisplay.go) — so there
// is deliberately no 20 written out anywhere in this package.
func mtSpec(d cat.Dialect) (transport.CommandSpec, error) {
	return yaesu.MTSpec(d, &params)
}

// ctcssNames maps the wire CTCSS state to codeplug's display spelling —
// the strings codeplug.Validate checks for and caps.go's own
// Capabilities.ToneModes advertises (toneModes). Deliberately NOT
// cat.CTCSSState.String(), whose spellings ("off", "ENC/DEC") are log
// labels, not model values.
//
// SIX ENTRIES: the FTX-1's own P8 domain (spec.md §6) is wider than every
// registered dialect but the FT-991A's, and its own '3' is UNSPLIT ("3:
// DCS", not "DCS ENC/DEC"/"DCS ENC"). Bytes '4' ("PR FREQ") and '5' ("REV
// TONE") carry NO NAMED cat.CTCSSState constant at all — core/cat's own
// register explains why (dialectconfig.go's ToneStatesSix doc comment: a
// second name for byte '4' would collide with the FT-991A's CTCSSDCSEnc
// map key) — so they are constructed here by casting the byte directly,
// which is all cat.CTCSSState(c) needs to round-trip a value with no name.
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:        "OFF",
	cat.CTCSSEncDec:     "ENC-DEC",
	cat.CTCSSEnc:        "ENC",
	cat.CTCSSDCSEncDec:  "DCS",
	cat.CTCSSState('4'): "PR-FREQ",
	cat.CTCSSState('5'): "REV-TONE",
}

// ctcssVocab is ctcssNames' shared-body form, built once FROM ctcssNames
// itself, IN THE SAME ORDER the current legend text implies — a plain map
// range cannot give that order, so each entry is looked up by its known
// key instead. yaesu.CTCSSLegend(ctcssVocab) then renders exactly
// "OFF/ENC-DEC/ENC/DCS/PR-FREQ/REV-TONE", byte-identical to write.go's
// former literal. ctcssNames/ctcssByName stay (not deleted): the read
// side still renders m.CTCSS's exact wire byte back to a display name
// beyond CTCSS-state lookups on unreachable paths, and the write side's
// MT-tag mapping is untouched.
var ctcssVocab = []yaesu.CTCSSName{
	{Name: ctcssNames[cat.CTCSSOff], State: cat.CTCSSOff},
	{Name: ctcssNames[cat.CTCSSEncDec], State: cat.CTCSSEncDec},
	{Name: ctcssNames[cat.CTCSSEnc], State: cat.CTCSSEnc},
	{Name: ctcssNames[cat.CTCSSDCSEncDec], State: cat.CTCSSDCSEncDec},
	{Name: ctcssNames[cat.CTCSSState('4')], State: cat.CTCSSState('4')},
	{Name: ctcssNames[cat.CTCSSState('5')], State: cat.CTCSSState('5')},
}

// mrParams is this radio's yaesu.MRParams value for the shared MR-read/
// MW-write bodies. Model is set explicitly to "ftx1" (not left at the
// zero value): dialect.CATID() is "0840", which AnswerMismatchError must
// not name instead of the driver's own error-prefix spelling — matching
// this package's pre-migration literal (params.Name, ftx1.go).
// ExplicitTagRefusal/SkipTierFields/RequestConditionalTagFields/
// ScanSkipUnavailable all stay false: this driver's own tag/scan-skip
// gate (requestedFields, write.go) runs BEFORE the shared body is ever
// called, so none of the shared body's own tag/tier variants apply here.
var mrParams = yaesu.MRParams{
	Name:          "ftx1",
	Model:         "ftx1",
	MRAnswerLen:   30,
	CTCSS:         ctcssVocab,
	AcceptedKinds: nil,
	ToneRead:      yaesu.ToneUnknown,
	ToneWrite:     yaesu.ToneNeverRequested,
	WriteKind:     func(d cat.Dialect) byte { return d.MWWriteKind() },
	EraseReason:   "erase cannot be expressed by the CAT codec (no erase command is documented for a memory channel), and FieldErase is not write-Supported",
}

// ReadChannel implements driver.Session: MR (channel data) + MT (tag)
// reads, mapped into one codeplug.Channel — the FT-710's own two-frame
// shape, the only precedent in this fleet for a slot-address-only MT form
// beside a separate memory-data frame (every other registered Yaesu
// dialect either has no MT command at all or a COMBINED one carrying both
// in a single frame).
//
// The empty-slot rule: a "?;" rejection of the MR read is mapped to an
// EMPTY channel, not an error. ASSUMED, exactly as it is on every sibling
// dialect: no FTX-1 has ever answered an MR read of an unpopulated slot,
// and "?;" is the protocol's single unattributed NAK.
//
// NO KIND-BYTE NARROWING. Unlike the FT-710 (whose read-side leniency is
// HW-CONFIRMED) and the FTdx1200 (whose narrower read-side legend is
// separately printed in its own manual), the FTX-1's P7 legend states one
// domain for the whole 27-byte block (spec.md §3.1) with no distinct
// read-side chart, and no FTX-1 has ever been read to confirm or narrow
// it further (matrix §2, probe 4 — OPEN). cat.Dialect.ParseMRAnswer
// already validates the byte against the package-wide documented domain
// ('0'-'5'/'-'); this driver adds no further restriction it has no
// evidence for.
//
// TagDisplay comes back UNAVAILABLE, ALWAYS: MTFormShortNoDisplay carries
// no display byte at all (spec.md §3.3/§8) — there is no such flag to
// report, and Unavailable is what codeplug.BoolField means by "there is
// no such field" (as opposed to Unknown, "the radio has one and this read
// did not learn it").
//
// CTCSSTone and ScanSkip come back UNKNOWN, ALWAYS: the CAT protocol has
// no command that reads a memory channel's live tone-table index or
// scan-skip flag on this radio either (spec.md §3.1's own field table has
// no position for either) — Unknown means "preserve whatever the radio
// has" to every write path downstream.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// The single-frame MR half goes through the shared body — this
	// driver's slot-parse error text, ErrRejected-as-empty-channel
	// mapping, MR-answer-decode error and AnswerMismatchError all match
	// yaesu.ReadChannel's own byte-for-byte (spec.md/B1-ftx1-spec.md §2).
	ch, err := yaesu.ReadChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, slot)
	if err != nil {
		return codeplug.Channel{}, err
	}
	if ch.Data == nil {
		// MR "?;" -> empty channel, no MT attempted — same as before.
		return ch, nil
	}

	sl, _ := s.dialect.ParseSlot(slot) // already validated inside yaesu.ReadChannel

	mtCmd, err := s.dialect.BuildMTRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: %w", sl.Wire(), err)
	}
	mtCmdSpec, err := mtSpec(s.dialect)
	if err != nil {
		return codeplug.Channel{}, err
	}
	tframe, err := s.eng.Do(ctx, mtCmd, mtCmdSpec)
	if err != nil {
		// A successful MR followed by a rejected MT is a genuine error
		// here, not an empty-tag signal — the same posture the FT-710's
		// own ReadChannel takes (fakeradio's register item 4, applied by
		// analogy: tag state is modelled as independent of channel-data
		// state).
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: MT: %w", sl.Wire(), err)
	}
	tslot, tag, err := s.dialect.ParseMTAnswerNoDisplay(tframe)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: %w: %w", sl.Wire(), driver.ErrRecordDecode, err)
	}
	if tslot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: params.Name, Requested: sl.Wire(), Answered: tslot.Wire()}
	}

	ch.Data.Tag = tag
	return ch, nil
}

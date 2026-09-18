// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ctcssNames maps the wire CTCSS state to codeplug's display spelling — the
// strings codeplug.Validate checks for, and the ones this driver's own
// Capabilities.ToneModes advertises. Deliberately NOT
// cat.CTCSSState.String(), whose spellings ("off", "ENC/DEC", "DCS ENC/DEC")
// are log labels rather than model values.
//
// FIVE ENTRIES, AND THE LAST TWO ARE WHY THIS RADIO EXISTS IN THIS FLEET.
// The memory record's P8 legend prints `0: CTCSS "OFF" 1: CTCSS ENC/DEC
// 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC`, identically on all five blocks
// that carry it — IF 795-796, MR 977-978, MT 1010-1011, MW 1048-1049, OI
// 1128-1129 — where EVERY registered sibling prints 0/1/2 only (matrix
// §1.17). A driver that reused the family's three-entry map would meet a '3'
// or a '4' with the "unmapped CTCSS state" refusal below: the right
// direction, and a failure of every read of such a channel.
// TestCTCSSNames_CoverExactlyTheAdvertisedVocabulary holds this map and
// caps.go's published list to each other so the two cannot drift.
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:       "OFF",
	cat.CTCSSEncDec:    "ENC-DEC",
	cat.CTCSSEnc:       "ENC",
	cat.CTCSSDCSEncDec: "DCS-ENC-DEC",
	cat.CTCSSDCSEnc:    "DCS-ENC",
}

// shiftNames maps the wire shift state to codeplug's display spelling
// ("SIMPLEX", "PLUS", "MINUS"), from this radio's own P10 legend "0: Simplex
// 1: Plus Shift 2: Minus Shift", printed identically on MR (981), MT (1014),
// MW (1051), IF (799) and OI (1132) — matrix §1.16.
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// mtSpec builds the transport spec for a combined MT read against d: the
// prefix, the EXACT answer length, and NO RETRY.
//
// THE LENGTH IS DERIVED FROM THE DIALECT, never written here. d's own
// MTAnswerBounds reports the geometry its declared MT form and tag width
// imply — for this radio's combined form that is an exact 41, being
// 29 + TagMaxBytes — and there is deliberately no 41 in this package's
// production code at all. Two reasons, both load-bearing: a literal would
// silently keep answering 41 for a dialect whose tag field was a different
// width, and the combined answer's exactness is itself an ASSUMPTION the
// dialect carries (its register entry THE COMBINED MT ANSWER'S EXACT LENGTH,
// 41), whose recorded Stage R contingency is a 30..41 WINDOW. If that
// contingency is ever taken, the bounds move in core/cat and this spec moves
// with them.
//
// The equal-bounds check is what makes that safe rather than lucky:
// transport.CATReadSpec takes a single exact length, so a dialect reporting
// a genuine window has no honest exact length and gets an error instead of
// its window's top silently becoming a hard requirement (which would reject
// every shorter answer as unmatched, i.e. as a timeout).
//
// RETRYREADS IS 0, AND THAT IS A DECISION (plan task 10, matrix §3.5). The
// ID probe next door takes ONE retry on the ordinary reasoning — a read is
// idempotent, so a single swallowed reply should not fail an operation — and
// this spec declines it for a reason peculiar to a driver with no second
// frame. THE FT-891 REACHES THE SAME VALUE FROM A DIFFERENT PREMISE (whether
// its MT read exists at all, which its manual contradicts itself about); on
// THIS radio the manual does not contradict itself, and the choice matters
// MORE rather than less: with no MR to fall back on, a retried timeout
// cannot be told from a slow radio, and a silently doubled transcript is a
// worse failure than a clean timeout. transport.CATReadSpec takes
// retryReads as a parameter, so this radio's spec must CHOOSE.
func mtSpec(d cat.Dialect) (transport.CommandSpec, error) {
	return yaesu.MTSpec(d, &params)
}

// ReadChannel implements driver.Session: ONE combined MT read for MEM and
// PMS alike, mapped into one codeplug.Channel. Matrix §3.5's truth table,
// exactly:
//
//   - a well-formed answer carries the field block and the tag together, so
//     it is an ATOMIC snapshot of the channel;
//   - MT "?;" means the slot is EMPTY (Data nil, the slot carried through,
//     no error);
//   - an MT TIMEOUT is the transport's own error: ONE frame, no retry, no
//     second frame of any kind.
//
// MT-ONLY, AND MR IS NEVER SENT — not here and not by Open, which probes
// nothing at all (doc.go, "There is NO discovery"). The 41-byte combined
// answer carries the whole field block AND the tag in a single frame, so the
// two-frame stitch the FT-710's MR+MT read has to guard against (field block
// read from one radio state, tag from a later one) is structurally
// impossible here rather than merely locked against.
//
// THERE IS NO MR CROSS-CHECK, AND THAT IS A FACT ABOUT THIS MANUAL. The
// FT-891's read design exists to name a contradiction in ITS manual — a
// Control Command List giving MT Set only against a detail block printing a
// Read chart — and this manual has no analogue: its availability row is
// "MT | MEMORY CHANNEL TAG | O O O X" (layout 181) and its MT block prints
// all three charts filled (998-1033). So there is no
// ErrMTReadRejectedForOccupiedSlot analogue here, and this driver makes
// EXACTLY ONE interpretation of "?;" and has exactly one site for it.
//
// THE TIMEOUT HAS NO BRANCH AT ALL, deliberately (plan §Plan-vs-spec ruling
// 5; spec §Error handling: "no timeout branch of that family"). There is no
// MTReadTimeoutError analogue, so a timeout is not re-typed and reaches the
// caller as the transport's own error through the same wrap every other
// transport failure on this path gets — errors.Is finds transport.ErrTimeout
// and errors.As finds no driver type standing between them. The wrap adds
// the slot the transport cannot know and nothing else. Silence is therefore
// never read as emptiness: only a "?;" means empty.
//
// THE WHOLE OPERATION IS HELD UNDER opMu (spec erratum S-E4, matrix M-E2).
// One exchange needs no lock against another read — the engine already
// serialises exchanges — but a read, a write and a settings read are
// different DRIVER OPERATIONS and must not interleave their frames; see the
// Session type's doc comment.
//
// TagDisplay comes back codeplug.Unavailable, ALWAYS, and this is the
// FT-891's most distinctive cell inverted (matrix §2.3, erratum M-E3): MT's
// P11 legend reads "P11 0: (Fixed)" (layout 1015) where that radio prints
// `0: TAG "OFF" 1: TAG "ON"` and its read reports the flag Known. There is
// no value here to report, and Unknown would be the wrong word — Unknown
// means "the radio has one and this read did not learn it". The capability
// half agrees: caps.go grades spec.FieldTagDisplay the zero FieldSupport on
// both banks.
//
// THAT IS A FACT ABOUT THE LEGEND, NOT ABOUT WHAT BYTE A REAL ANSWER
// CARRIES (matrix §2.3 as folded by erratum M-E9). The legend printing
// "0: (Fixed)" is what this manual states; that a genuine answer really
// puts '0' there is the further, ASSUMED step, and it is the register's
// THE PRINTED-FIXED BYTES ARE ANSWERED AS PRINTED entry — which is why a
// real FT-991A answering some other byte would fail the parse outright
// (cat.P11Fixed) rather than reach this mapping at all.
//
// TxClar CAN come back TRUE, which is the other half of M-E3: under the
// dialect's MemoryP5 = cat.P5TxClar the parser carries byte 21 through as a
// live TX-clarifier state, where the FT-891's P5Fixed makes it always false.
//
// CTCSSTone and ScanSkip come back codeplug.Unknown, ALWAYS: the register's
// TONE-NUMBER UNREACHABILITY and SCAN-SKIP UNREACHABILITY entries — two
// entries, because their lifting captures are two (matrix erratum M-E10).
// "Unknown" means "preserve whatever the radio has" to every write path
// downstream, which is the only honest instruction for a field this driver
// cannot see.
//
// KIND CHECKING IS THE PARSER'S, not this driver's, and on this radio the
// read domain is TRANSCRIBED rather than assumed: MT's own P7 legend prints
// both directions in full, "Set: 0: (Fixed) / Read: 0: VFO 1: Memory"
// (layout 1009), so an out-of-vocabulary byte comes back as a
// *cat.ParseError. No per-class narrowing is added on top, and this manual's
// SEVEN-value IF P7 (792-793) and two-value OI P7 (1127) are deliberately
// not read across into the memory record's — see doc.go.
//
// Error typing, three classes on the answering path, none a bare fmt.Errorf:
// PARSE failures stay *cat.ParseError under a wrap (errors.As finds them, and
// the wrap adds the slot the bare parser could not know); the SLOT-ECHO check
// raises this driver's own *AnswerMismatchError; and a transport failure —
// timeout included — reaches the caller as itself under the same slot-naming
// wrap.
//
// THREE DEFENSIVE ARMS SIT OUTSIDE THAT ACCOUNT and each IS a bare
// fmt.Errorf, deliberately: the unmapped-CTCSS and unmapped-shift guards,
// both unreachable after the dialect's own per-radio validation and kept so
// that a widened dialect refuses rather than mislabels; and mtSpec's
// geometry refusal, which is the ONE error here that names no slot because
// it is a fact about the DIALECT and would fail identically for every slot
// (naming one would suggest the failure was that slot's).
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	// Held for the WHOLE operation — see the doc comment and the Session
	// type's.
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		// Also where the sibling PMS TOKEN form dies: "P1L" is a slot on
		// every registered sibling and a NON-SLOT here, because this
		// radio's PMS pairs are the decimal channel numbers 100-117 (its MC
		// legend, layout 916). Refused before any frame is built.
		return codeplug.Channel{}, fmt.Errorf("ft991a: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMTRead(sl)
	if err != nil {
		// e.g. the answer-only none form — the DIALECT register's entry
		// SlotSpace.NoneWire = "000", ASSUMED because that form appears in
		// no FT-991A slot legend: grammatical per ParseSlot, never a legal
		// read target.
		return codeplug.Channel{}, fmt.Errorf("ft991a: ReadChannel: %w", err)
	}

	cmdSpec, err := mtSpec(s.dialect)
	if err != nil {
		return codeplug.Channel{}, err
	}

	frame, err := s.eng.Do(ctx, cmd, cmdSpec)
	if errors.Is(err, cat.ErrRejected) {
		// ASSUMED EMPTY — the register's MT "?;" ON A MEMORY OR PMS SLOT
		// MEANS THE SLOT IS EMPTY entry, and this is its ONLY site: no
		// other frame this driver sends can draw a "?;" from a slot.
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		// NO TIMEOUT ARM, deliberately: a timeout is a transport failure
		// like any other here and is surfaced as the transport's own error
		// rather than re-typed (see the doc comment). No retry preceded it
		// (mtSpec) and no second frame follows it — a cross-check answers a
		// REJECTION, not silence, and this radio has no cross-check at all.
		return codeplug.Channel{}, fmt.Errorf("ft991a: ReadChannel %s: MT: %w", sl.Wire(), err)
	}

	// The display-LESS pair, and it is the ONLY pair this dialect admits:
	// under MTPolicy.P11 = cat.P11Fixed the display-bearing
	// ParseMTAnswerCombinedDisplay refuses outright, because there is no
	// flag for a radio to report (matrix §2.3).
	m, tag, err := s.dialect.ParseMTAnswerCombined(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft991a: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: params.Name, Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		// Unreachable after the dialect's own per-radio CTCSS validation,
		// which narrows P8 to '0'-'4' under cat.ToneStatesCTCSSAndDCS;
		// refuse rather than silently mislabel if it ever is not.
		return codeplug.Channel{}, fmt.Errorf("ft991a: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ft991a: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			// uint64 since the Icom tier widened the neutral model (design
			// D4): a widening conversion from this protocol's uint32, which
			// can never lose anything. The narrowing direction — the write
			// path — is the checked one (cat.MemoryFreqHz).
			FreqHz: uint64(m.FreqHz),
			// Rendered through THIS SESSION'S dialect, not cat.Mode.String:
			// the string is user-visible (it lands in the codeplug, the CLI
			// listing and the GUI grid), so it must come from the mode table
			// of the radio that answered — and on this radio that is not a
			// preference. core/cat's package-level fallback renders 'E' as
			// "PSK", the FTdx10's word, where this manual prints "C4FM":
			// one nibble, two REAL and different modes, which makes the
			// fallback ACTIVELY WRONG here rather than merely
			// unauthoritative (core/cat/ft991a/doc.go, "The mode fallback is
			// WRONG for this radio"). ModeName gives the display name; the
			// odd-state cat.ModeUnset renders "-" and is mapped through
			// faithfully (codeplug.Validate flags it as not a selectable
			// mode, which is the right outcome for a placeholder this
			// radio's legends do not list — the DIALECT register's THE
			// cat.ModeUnset MEMBER OF THE MODE TABLE entry). That the parser
			// refuses any P6 nibble outside the transcribed fourteen is this
			// DRIVER register's THE MODE NIBBLE'S DOMAIN entry: a legend is
			// a statement about what the chart draws, not a guarantee about
			// what the radio will ever send.
			Mode: s.dialect.ModeName(m.Mode),
			// The magnitude is manual-evidenced (P3's four digits at
			// positions 16-19); the byte carrying a NEGATIVE sign is not —
			// the DIALECT register's THE CLARIFIER'S MINUS-DIRECTION BYTE,
			// the ASCII HYPHEN-MINUS 0x2D entry. This manual prints the
			// token as TWO hyphens on all five blocks carrying the legend,
			// and evidence leg G confirmed at 1800 dpi that the doubling is
			// REAL — while P3 has five positions and a four-digit offset,
			// leaving exactly ONE for the direction. core/cat accepts only
			// '+' or '-' when reading the sign, so a radio using another
			// byte fails the parse loudly here rather than silently reading
			// a negative offset as positive.
			ClarHz: int(m.ClarHz),
			RxClar: m.RxClar,
			// CAN BE TRUE ON THIS RADIO, unlike the FT-891 (matrix §2.2,
			// erratum M-E3): `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"` is
			// printed on every block carrying the grid — MR 971, MT 1004,
			// MW 1042, IF 787, OI 1122 — so under the dialect's
			// MemoryP5 = cat.P5TxClar the parser carries byte 21 through as
			// live state. Carried from the parser rather than hard-coded, so
			// a change to that policy shows up here rather than being
			// masked.
			TxClar: m.TxClar,
			CTCSS:  ctcss,
			// The register's TONE-NUMBER UNREACHABILITY entry: no tone
			// number is readable. NOTE THE GAP THIS RADIO HAS AND ITS
			// SIBLINGS DO NOT — the CTCSS field above can say "DCS-ENC-DEC"
			// while this one can never say which code, because the
			// 41-position record has no DCS code field at all. That second
			// half is the register's DCS-CODE UNREACHABILITY entry, kept
			// separate from this one because its capture is separate.
			CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
			Shift:     shift,
			// The register's SCAN-SKIP UNREACHABILITY entry: no scan-skip
			// flag is readable.
			ScanSkip: codeplug.BoolField{State: codeplug.Unknown},
			Tag:      tag,
			// UNAVAILABLE, never Known and never Unknown: MT's P11 LEGEND
			// is printed "0: (Fixed)" (layout 1015), so the frame this
			// driver reads declares no display flag. See the method doc
			// comment — this is the FT-891's cell inverted, and the honest
			// reading is "this radio's frame has no such field", which is
			// STRONGER than "this read could not reach it". What a real
			// answer puts at byte 28 is the register's THE PRINTED-FIXED
			// BYTES ARE ANSWERED AS PRINTED entry, not this mapping's
			// claim.
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			// The seventeen fields the Icom model extensions added to the
			// neutral memory model (design D4/D8). UNAVAILABLE on this
			// radio: this family's memory frame carries none of them, so
			// there is no value to read and no question for the user. The
			// matching half is in caps.go, where this radio's banks grade
			// every one of them the zero FieldSupport.
			//
			// Not Absent (the zero FieldState), deliberately. Absent means
			// "this codeplug never spoke about the field", which is true of
			// a schema-3 FILE and false of a RADIO READ: a read of this
			// radio is a positive statement that the frame has no such
			// field, and Unavailable is the state that says so. There is ONE
			// read path on this radio, so plan P12's requirement is
			// structural rather than duplicated and the fleet's no-Absent
			// pin holds for every bank at once.
			TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
			Duplex:              codeplug.StringField{State: codeplug.Unavailable},
			OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
			ToneMode:            codeplug.StringField{State: codeplug.Unavailable},
			ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
			ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
			DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
			DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
			Filter:              codeplug.StringField{State: codeplug.Unavailable},
			DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
			TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
			TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
			ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
			AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
			Preamp:              codeplug.StringField{State: codeplug.Unavailable},
			Antenna:             codeplug.StringField{State: codeplug.Unavailable},
			IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
			SatBandSwap:         codeplug.BoolField{State: codeplug.Unavailable},
			SatTrace:            codeplug.BoolField{State: codeplug.Unavailable},
			SatTraceRev:         codeplug.BoolField{State: codeplug.Unavailable},
		},
	}, nil
}

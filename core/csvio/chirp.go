// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The three LossEntry.Action values this package produces. See
// LossEntry.
const (
	// ActionDropped means the CHIRP data was discarded outright: the
	// radio has no field to hold it at all.
	ActionDropped = "dropped"
	// ActionApproximated means the CHIRP data was kept but altered to
	// fit the radio's constraints (e.g. a tag truncated or
	// charset-sanitised).
	ActionApproximated = "approximated"
	// ActionUnsupported means the CHIRP data could not be mapped at
	// all; when Blocking is also true, the row's affected field (or the
	// whole row, for an unmappable Location) has no usable value.
	ActionUnsupported = "unsupported"
)

// LossEntry records one place ImportCHIRP could not carry a CHIRP value
// across losslessly.
type LossEntry struct {
	// Line is the 1-based CSV line the offending value came from (the
	// header is line 1, the first data row is line 2).
	Line int
	// Column is the CHIRP column name this entry concerns.
	Column string
	// Value is the offending value, verbatim from the CHIRP cell.
	Value string
	// Action is one of ActionDropped, ActionApproximated,
	// ActionUnsupported.
	Action string
	// Detail is a human-readable explanation, self-contained (never
	// requires cross-referencing anything else).
	Detail string
	// Blocking is true when this entry means the import cannot proceed
	// without the user resolving it — see LossReport.HasBlocking.
	Blocking bool
}

// LossReport is every LossEntry ImportCHIRP produced for one import.
type LossReport struct {
	Entries []LossEntry
}

// HasBlocking reports whether r contains at least one Blocking
// LossEntry. See ImportCHIRP's doc comment for the contract this gates.
func (r LossReport) HasBlocking() bool {
	for _, e := range r.Entries {
		if e.Blocking {
			return true
		}
	}
	return false
}

// chirpCoreColumns must be present in a CHIRP CSV header for
// ImportCHIRP to proceed at all.
var chirpCoreColumns = []string{"Location", "Frequency", "Mode"}

// chirpExtraColumns lists CHIRP columns this package recognises but has
// no radio field for. Per row, an empty cell in one of these is
// silently ignored (it is that column's default/absent state); a
// non-empty cell is a non-blocking ActionDropped LossEntry. DtcsCode and
// DtcsPolarity get a second "default" value each (CHIRP's own
// no-DCS-configured defaults "023"/"NN") so that the overwhelming
// majority of real CHIRP exports — which populate these two on every
// row regardless of whether DTCS is in use — do not generate loss-report
// noise on every single channel.
var chirpExtraColumns = []string{"TStep", "DtcsCode", "DtcsPolarity", "Comment"}

// chirpExtraColumnDefaults gives the non-empty "this is not really data"
// value for the two chirpExtraColumns entries that have one (see its doc
// comment). A column absent from this map has only "" as its default.
var chirpExtraColumnDefaults = map[string]string{
	"DtcsCode":     "023",
	"DtcsPolarity": "NN",
}

// chirpOtherKnownColumns lists every CHIRP column ImportCHIRP consults by
// name OUTSIDE of chirpCoreColumns and chirpExtraColumns: Name (see
// sanitizeCHIRPName), and the Duplex/Offset/Tone/rToneFreq/cToneFreq/Skip
// columns read directly in importCHIRPRow. Together with those two lists
// this is the FULL set of column names this package recognises — see
// chirpKnownColumns.
var chirpOtherKnownColumns = []string{"Name", "Duplex", "Offset", "Tone", "rToneFreq", "cToneFreq", "Skip"}

// chirpKnownColumns is the set of every CHIRP column name ImportCHIRP
// recognises at all, whether or not it maps to a field on the radio
// (chirpCoreColumns + chirpExtraColumns + chirpOtherKnownColumns). A
// header column NOT in this set is unknown to this package entirely: see
// ImportCHIRP's unrecognisedColumns and its Header doc paragraph.
var chirpKnownColumns = func() map[string]bool {
	set := make(map[string]bool)
	for _, c := range chirpCoreColumns {
		set[c] = true
	}
	for _, c := range chirpExtraColumns {
		set[c] = true
	}
	for _, c := range chirpOtherKnownColumns {
		set[c] = true
	}
	return set
}()

// chirpModeMap maps a CHIRP Mode value to this radio family's
// display-name equivalent. CW and CWR both exist on this radio family as
// sideband-specific CW modes (CW-U/CW-L) rather than a single "CW" — see
// the brief. The RESULT is membership-checked against caps.Modes (see
// containsMode): mapping to a mode name is CHIRP-dialect knowledge, but
// whether THIS radio actually has that mode is caps' question.
var chirpModeMap = map[string]string{
	"FM":   "FM",
	"NFM":  "FM-N",
	"AM":   "AM",
	"USB":  "USB",
	"LSB":  "LSB",
	"CW":   "CW-U",
	"CWR":  "CW-L",
	"RTTY": "RTTY-U",
}

// Sentinel errors distinguishing parseCHIRPFrequency's failure modes, so
// importCHIRPRow can give a precise LossEntry.Detail.
var (
	errCHIRPFreqFormat       = errors.New("not a valid decimal MHz value")
	errCHIRPFreqFractionalHz = errors.New("has a sub-Hz fractional remainder")
	errCHIRPFreqRange        = errors.New("exceeds the representable frequency range")
)

// isCHIRPDigits reports whether s is empty or contains only ASCII
// digits.
func isCHIRPDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseCHIRPFrequency parses CHIRP's decimal-MHz Frequency column into
// exact whole Hz, WITHOUT going through floating point (a float64 MHz
// value cannot represent every whole-Hz frequency exactly, and exactness
// here is safety-relevant: this value may be sent to a radio). Any
// digits beyond the 6th decimal place (whole-Hz precision at MHz scale)
// must all be zero, or the value has a sub-Hz remainder the radio
// cannot store (errCHIRPFreqFractionalHz).
//
// uint64 since the Icom tier (design D4, adjudication 5: "CHIRP parsing
// widens"), matching codeplug.ChannelData.FreqHz. The ceiling this
// function enforces is the REPRESENTABLE one — what a uint64 of hertz
// can hold — and nothing narrower: whether a given frequency is one THIS
// radio can store is caps' question, answered by codeplug.Validate
// against MinFreqHz/MaxFreqHz, and answering it twice in two places with
// two different bounds is how the two drift apart. Before the tier the
// representable ceiling happened to be MaxUint32, which is the only
// reason a 5 GHz cell used to be refused here.
func parseCHIRPFrequency(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	intPart, fracPart, hasDot := strings.Cut(s, ".")
	if intPart == "" || !isCHIRPDigits(intPart) {
		return 0, errCHIRPFreqFormat
	}
	if hasDot && !isCHIRPDigits(fracPart) {
		return 0, errCHIRPFreqFormat
	}
	for len(fracPart) < 6 {
		fracPart += "0"
	}
	wholeHzFrac, remainder := fracPart[:6], fracPart[6:]
	for i := 0; i < len(remainder); i++ {
		if remainder[i] != '0' {
			return 0, errCHIRPFreqFractionalHz
		}
	}
	mhz, err := strconv.ParseUint(intPart, 10, 64)
	if err != nil {
		// Only reachable for an intPart too long to fit uint64:
		// isCHIRPDigits already guarantees intPart is all-digits.
		return 0, errCHIRPFreqFormat
	}
	// wholeHzFrac is exactly 6 characters, all verified digits (padded
	// above, sliced from a fracPart isCHIRPDigits already checked): this
	// can never fail to parse or overflow uint64.
	hzFrac, _ := strconv.ParseUint(wholeHzFrac, 10, 64)

	// The range check MUST happen before computing mhz*1_000_000: mhz is
	// parsed with no upper bound beyond "fits in a uint64" (intPart can be
	// arbitrarily many digits), so an absurdly large-but-uint64-legal mhz
	// (e.g. from "18446744073710.500000") multiplied by 1_000_000
	// silently wraps around uint64's own range. That wrapped value can
	// land back inside a plausible frequency range and be accepted as if
	// it were a small, legitimate one — exactly the kind of miscomputed
	// value this project must never send to a radio. Rearranging the
	// bound (mhz > (max-hzFrac)/1_000_000) answers the same question
	// without ever performing the multiplication that could overflow.
	//
	// hzFrac needs no bound of its own: it is exactly six digits, so it
	// is at most 999,999. It appeared in this condition while max was
	// MaxUint32, where a six-digit value could genuinely exceed nothing
	// but did no harm; against MaxUint64 the comparison would be plainly
	// dead, so it is gone rather than left to read as a live check.
	const max = uint64(math.MaxUint64)
	if mhz > (max-hzFrac)/1_000_000 {
		return 0, errCHIRPFreqRange
	}
	return mhz*1_000_000 + hzFrac, nil
}

// parseCHIRPTone parses a CHIRP rToneFreq/cToneFreq cell (decimal Hz,
// e.g. "88.5") EXACTLY (see parseExactToneDeciHz — no floating point, and
// no more than one decimal place of precision; "88.54" is rejected
// outright rather than rounded) and reports whether the result appears in
// this radio's own CTCSS chart (caps.CTCSSTones). A cell that fails to
// parse, carries more precision than one decimal place, or parses but
// matches no tone in caps' chart, returns (0, false): all three are
// equally unusable and the caller reports a single Blocking LossEntry
// either way.
func parseCHIRPTone(s string, caps spec.Capabilities) (spec.Tone, bool) {
	deciHz, err := parseExactToneDeciHz(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	t := spec.Tone(deciHz)
	if !capsHasTone(caps, t) {
		return 0, false
	}
	return t, true
}

// reaches reports whether bank can reach field on this radio — the ONE
// question every capability-aware branch of the CHIRP importer asks
// (design D4, adjudication 16). It is spec.FieldSupport.Unreachable
// inverted, which means "says nothing" and "says no" answer alike: a
// bank absent from caps, or present without the field, is not a bank
// that reaches it.
func reaches(caps spec.Capabilities, bank spec.BankID, field spec.Field) bool {
	return !caps.FieldSupport(bank, field).Unreachable()
}

// duplexFor returns the wire-form duplex value caps uses for direction,
// and true, or ("", false) when this radio expresses no such option. The
// FieldDuplex analogue of shiftFor, and unambiguous for the same reason:
// spec.Capabilities.Validate rejects two options sharing a direction.
func duplexFor(caps spec.Capabilities, d spec.DuplexDirection) (string, bool) {
	for _, o := range caps.DuplexOptions {
		if o.Direction == d {
			return o.Value, true
		}
	}
	return "", false
}

// toneModeFor returns the wire-form tone-mode value caps uses for the
// given semantics, and true, or ("", false) when this radio expresses no
// such mode. The FieldToneMode analogue of toneStateFor.
func toneModeFor(caps spec.Capabilities, semantics spec.ToneModeSemantics) (string, bool) {
	for _, m := range caps.ToneModes {
		if m.Semantics == semantics {
			return m.Value, true
		}
	}
	return "", false
}

// capsHasDTCSCode reports whether code is in this radio's DTCS table.
func capsHasDTCSCode(caps spec.Capabilities, code int) bool {
	for _, c := range caps.DTCSCodes {
		if c == code {
			return true
		}
	}
	return false
}

// capsHasDTCSPolarity reports whether p is in this radio's DTCS polarity
// vocabulary.
func capsHasDTCSPolarity(caps spec.Capabilities, p string) bool {
	for _, v := range caps.DTCSPolarities {
		if v == p {
			return true
		}
	}
	return false
}

// shiftFor returns the wire-form shift value caps uses for direction, and
// true, or ("", false) when this radio expresses no such shift. Capabilities
// with two options for one direction cannot reach here: spec.Validate
// rejects them, so the answer is unambiguous by construction.
func shiftFor(caps spec.Capabilities, d spec.ShiftDirection) (string, bool) {
	for _, o := range caps.ShiftOptions {
		if o.Direction == d {
			return o.Value, true
		}
	}
	return "", false
}

// toneStateFor returns the wire-form CTCSS state caps uses for the given
// tone semantics, and true, or ("", false) when this radio expresses no
// such state. As with shiftFor, spec.Validate guarantees at most one
// state per semantics value.
func toneStateFor(caps spec.Capabilities, semantics spec.ToneSemantics) (string, bool) {
	for _, s := range caps.CTCSSStates {
		if s.Semantics == semantics {
			return s.Value, true
		}
	}
	return "", false
}

// capsHasTone reports whether t is in this radio's CTCSS chart.
func capsHasTone(caps spec.Capabilities, t spec.Tone) bool {
	for _, x := range caps.CTCSSTones {
		if x == t {
			return true
		}
	}
	return false
}

// containsMode reports whether caps lists the given display-name mode.
func containsMode(caps spec.Capabilities, mode string) bool {
	for _, m := range caps.Modes {
		if m == mode {
			return true
		}
	}
	return false
}

// chirpTagByteOK reports whether b is a legal tag byte for this radio
// family: printable ASCII 0x20-0x7E, excluding ';' (0x3B). Printable
// ASCII excluding ';' is a family-wide CAT fact — ';' is the protocol
// terminator, not a per-model choice — so this needs no capability. This
// restates codeplug's validTagByte (which is unexported) rather than
// reaching into that package for it — core/csvio depends only on
// core/codeplug's exported surface, core/spec, and stdlib.
func chirpTagByteOK(b byte) bool {
	return b >= 0x20 && b <= 0x7E && b != ';'
}

// sanitizeCHIRPName turns a CHIRP Name into a radio tag: any byte outside
// the radio's tag charset is replaced with a space (byte for byte, so
// this never has to worry about splitting a multi-byte UTF-8 rune), and
// the result is then truncated to caps.TagLen. Each transformation that
// actually changed something produces its own non-blocking
// ActionApproximated LossEntry.
func sanitizeCHIRPName(line int, name string, caps spec.Capabilities) (string, []LossEntry) {
	var entries []LossEntry
	b := []byte(name)
	sanitized := false
	for i := 0; i < len(b); i++ {
		if !chirpTagByteOK(b[i]) {
			b[i] = ' '
			sanitized = true
		}
	}
	if sanitized {
		entries = append(entries, LossEntry{
			Line: line, Column: "Name", Value: name, Action: ActionApproximated, Blocking: false,
			Detail: fmt.Sprintf("Name contained a byte outside the %s tag charset (printable ASCII 0x20-0x7E, excluding ';'); replaced with a space", caps.Model),
		})
	}
	if len(b) > caps.TagLen {
		full := string(b)
		b = b[:caps.TagLen]
		entries = append(entries, LossEntry{
			Line: line, Column: "Name", Value: name, Action: ActionApproximated, Blocking: false,
			Detail: fmt.Sprintf("Name %q is %d bytes, exceeds the %s's %d-byte tag limit; truncated to %q", full, len(full), caps.Model, caps.TagLen, string(b)),
		})
	}
	return string(b), entries
}

// chirpTagDisplay derives the tag_display value every channel imported
// from a CHIRP file into bank must carry, from that bank's own
// spec.FieldTagDisplay support and nothing else (M9c-6 D-tagdisplay):
//
//   - Read AND Write both spec.Unsupported → codeplug.Unavailable. This
//     radio's memory frame carries no display flag at all, so there is no
//     value for the imported channel to hold and no question for the user
//     to answer: Unavailable is what BoolField means by that
//     (core/codeplug/fieldstate.go), and it is never sent.
//   - anything else → codeplug.Unknown. The flag exists in the frame and
//     CHIRP's schema has no column that speaks to it, so the file simply
//     does not say — see below.
//
// The Unknown arm is M9c-5 (E1d)'s behaviour, unchanged and still the
// point: CHIRP has no display-flag column, so before E1 this line
// manufactured a plain false and every imported channel silently switched
// the radio's display off, with nothing anywhere able to tell that from a
// deliberate choice. The honest Unknown blocks its channel at plan time
// instead (codeplug.Diff, "tag display unknown — set On or Off before
// sending") until the user decides. Refusing to guess costs one decision
// per import; guessing cost a setting the user never made.
//
// What M9c-6 adds is that Unknown is only honest for a radio that HAS the
// flag. The FTdx10's combined MT record has none (core/driver/ftdx10's
// bankFields: spec.FieldTagDisplay is the zero FieldSupport on every bank
// and profile), and an Unknown there would state that the answer is merely
// not yet known — inviting the very in-cell route to Known that M9c-6 D5b
// opens, and so handing the user a way to manufacture a flag the radio
// cannot store. Unavailable says the true thing, and both the grid cell
// and the paste path already refuse it.
//
// The rule is deliberately the SAME rule, applied to the same field, as
// app/uispec.go's bankTagDisplayDefault (the blank-row factory's default):
// "absent from the frame in BOTH directions" is the one trigger for
// Unavailable, and merely unwritable / merely unproven (spec.Unverified) /
// unproven-but-consented (spec.ConsentedUnverified) /
// transmitted-but-ignored (spec.Inert) all remain fields this radio's
// frame carries. Consent in particular changes NOTHING here: Unreachable
// asks about both directions being Unsupported, and a consented session's
// write label is not Unsupported — so a CHIRP import behaves identically
// whether or not the user has consented, which is the honest answer,
// since consent is about authorising a write and this is about whether
// the frame has the field at all.
//
// The third caller M9c-6 said would force the move HAS appeared — M9d-2's
// chirpScanSkip, below, asks the same question of spec.FieldScanSkip — so
// the predicate itself now lives in the package both can import, as
// spec.FieldSupport.Unreachable, and all three sites call it rather than
// re-spelling the two comparisons. What is NOT shared, deliberately, is
// what each site DOES with the answer: this one and bankTagDisplayDefault
// answer Unavailable, chirpScanSkip answers Unknown-plus-loss, and those
// differ because the two fields' unreachability means different things to
// the user (see chirpScanSkip's own comment).
//
// The zero-value lookup covers "bank absent from caps entirely" and "bank
// present but not listing the field" alike (spec.Capabilities.FieldSupport
// returns the zero FieldSupport for either), so both fall out as
// Unavailable with no special-casing: caps that say nothing about a
// display flag are not evidence that one exists.
func chirpTagDisplay(caps spec.Capabilities, bank spec.BankID) codeplug.BoolField {
	if caps.FieldSupport(bank, spec.FieldTagDisplay).Unreachable() {
		return codeplug.BoolField{State: codeplug.Unavailable}
	}
	return codeplug.BoolField{State: codeplug.Unknown}
}

// chirpScanSkip converts a CHIRP Skip cell to this project's ScanSkip
// field state, CAPABILITY-AWARE (M9d-2, spec decision 5): on a radio
// whose scan-skip is Unreachable — today, EVERY registered radio — a
// blank cell yields Unknown (not Known-false: Known admits the field
// into diff's requestedFields, which blocked every CHIRP-imported
// channel on such radios — the M9c-6 manifest's A7 finding), and an "S"
// cell yields Unknown plus a NON-BLOCKING loss entry recording the
// dropped intent. A radio with genuine skip support keeps the literal
// reading: blank means Known-false, "S" means Known-true. Unrecognised
// values are unchanged in both worlds.
//
// The A7 finding is worth stating in full, because the old construction
// looked harmless: a blank Skip column is what nearly every CHIRP file
// has, {Known,false} is a true statement ABOUT THE FILE, and nothing in
// this package could see the consequence. codeplug.Diff's requestedFields
// (diff.go) admits a field on State == Known alone, and the all-or-nothing
// per-field write gate then blocked the WHOLE channel because
// spec.FieldScanSkip is not writable on an FT-710 — so a clean three-row
// import planned as "Blocked 3", over a column the user had left empty.
// The fix is to stop making the claim: on a radio that cannot reach the
// field, an absent cell says nothing this radio can act on, and Unknown
// is the state that means exactly that (core/codeplug/fieldstate.go —
// "preserve whatever the radio has").
//
// Why Unknown rather than chirpTagDisplay's Unavailable, when the caps
// question asked is the identical one: Unavailable would be defensible and
// is what a READ of such a radio reports, but it is a stronger claim than
// an IMPORT is entitled to make. Unknown here also keeps the imported
// channel's scan_skip in the same state a native-CSV round-trip and a
// blank row already produce, so a CHIRP import introduces no fourth
// shape. Neither state reaches the wire and neither asks the user
// anything (the grid cell edits scan_skip only from Known —
// app/frontend/src/lib/grid/columns.js), so the difference is one of
// honesty, not behaviour.
//
// The "S" loss entry is non-blocking on purpose. It records a real intent
// the radio cannot store, so it must not be silent; but refusing the whole
// channel over a flag this protocol does not carry would be the same
// over-blocking A7 found, moved one layer up. bank is the TARGET bank —
// the same per-bank lookup chirpTagDisplay justifies — and line is the
// row's 1-based CSV line, for the entry.
func chirpScanSkip(caps spec.Capabilities, bank spec.BankID, raw string, line int) (codeplug.BoolField, []LossEntry) {
	unreachable := caps.FieldSupport(bank, spec.FieldScanSkip).Unreachable()
	switch raw {
	case "S":
		if unreachable {
			return codeplug.BoolField{State: codeplug.Unknown}, []LossEntry{{
				Line: line, Column: "Skip", Value: raw, Action: ActionDropped, Blocking: false,
				Detail: fmt.Sprintf("CHIRP Skip %q dropped: scan-skip is not reachable over CAT on %s; scan-skip left unresolved", raw, caps.Model),
			}}
		}
		return codeplug.BoolField{State: codeplug.Known, Value: true}, nil
	case "":
		if unreachable {
			return codeplug.BoolField{State: codeplug.Unknown}, nil
		}
		return codeplug.BoolField{State: codeplug.Known, Value: false}, nil
	default:
		// Unrecognised: the same answer on every radio, since the cell says
		// something neither world can map. Byte-identical to the pre-M9d-2
		// wording — this arm did not move and its Detail must not either.
		return codeplug.BoolField{State: codeplug.Unknown}, []LossEntry{{
			Line: line, Column: "Skip", Value: raw, Action: ActionDropped, Blocking: false,
			Detail: fmt.Sprintf("CHIRP Skip value %q has no %s equivalent; scan-skip left unresolved", raw, caps.Model),
		}}
	}
}

// importCHIRPRow builds the Channel one CHIRP data row describes (nil if
// its Location cannot be mapped to a slot at all) and every
// LossEntry that row produced. line is the row's 1-based CSV line;
// header/colIndex/record let cell values be looked up by CHIRP column
// name regardless of column order; a column absent from the header (any
// chirpExtraColumns/Tone-pair column beyond the three required columns)
// reads as "".
func importCHIRPRow(line int, colIndex map[string]int, record []string, caps spec.Capabilities) (*codeplug.Channel, []LossEntry) {
	// cell looks up name by column name; a name absent from the header
	// at all (any recognised column beyond the three required ones, see
	// chirpCoreColumns) reads as "". ImportCHIRP guarantees
	// len(record) == the header length colIndex was built from, so any
	// index colIndex does hold is always in bounds.
	cell := func(name string) string {
		idx, ok := colIndex[name]
		if !ok {
			return ""
		}
		return record[idx]
	}

	var entries []LossEntry

	// Location -> slot. No slot means no Channel can be built at all:
	// this is the one field whose failure drops the whole row rather
	// than just leaving a field unresolved. CHIRP Locations are 1-based
	// positions in the radio's main memory bank, so Location N is that
	// bank's Nth slot — the bank supplies both the range and the slot's
	// canonical wire form, neither of which this package may assume.
	memBank, haveMemBank := caps.Bank(spec.BankMemory)
	locRaw := cell("Location")
	locN, err := strconv.Atoi(strings.TrimSpace(locRaw))
	if err != nil {
		return nil, append(entries, LossEntry{
			Line: line, Column: "Location", Value: locRaw, Action: ActionUnsupported, Blocking: true,
			Detail: "Location is not a valid integer; cannot map to a memory slot",
		})
	}
	if !haveMemBank || len(memBank.Slots) == 0 {
		return nil, append(entries, LossEntry{
			Line: line, Column: "Location", Value: locRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("%s has no memory bank to import into", caps.Model),
		})
	}
	if locN < 1 || locN > len(memBank.Slots) {
		return nil, append(entries, LossEntry{
			Line: line, Column: "Location", Value: locRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("%s memory slots are %s-%s; Location is out of range", caps.Model, memBank.Slots[0], memBank.Slots[len(memBank.Slots)-1]),
		})
	}
	slot := memBank.Slots[locN-1]

	data := &codeplug.ChannelData{}

	// Name -> Tag.
	tag, nameEntries := sanitizeCHIRPName(line, cell("Name"), caps)
	data.Tag = tag
	entries = append(entries, nameEntries...)

	// TagDisplay: from the TARGET BANK's own support, never a constant —
	// see chirpTagDisplay for the rule and why the two answers differ.
	data.TagDisplay = chirpTagDisplay(caps, memBank.ID)

	// Frequency -> FreqHz.
	freqRaw := cell("Frequency")
	freqHz, err := parseCHIRPFrequency(freqRaw)
	if err != nil {
		detail := "Frequency is not a valid decimal MHz value"
		switch {
		case errors.Is(err, errCHIRPFreqFractionalHz):
			detail = fmt.Sprintf("Frequency has a sub-Hz fractional remainder the %s cannot store", caps.Model)
		case errors.Is(err, errCHIRPFreqRange):
			detail = "Frequency exceeds the representable range"
		}
		entries = append(entries, LossEntry{Line: line, Column: "Frequency", Value: freqRaw, Action: ActionUnsupported, Blocking: true, Detail: detail})
	} else {
		data.FreqHz = freqHz
	}

	// Mode. chirpModeMap is CHIRP-dialect knowledge — which of this radio
	// family's display modes a CHIRP mode name should become — and stays
	// here. Whether the radio actually HAS that mode is caps' question,
	// and a mapped mode the radio lacks blocks rather than being written.
	modeRaw := cell("Mode")
	mapped, mappable := chirpModeMap[modeRaw]
	switch {
	case !mappable:
		entries = append(entries, LossEntry{
			Line: line, Column: "Mode", Value: modeRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("CHIRP mode %q has no %s equivalent", modeRaw, caps.Model),
		})
	case !containsMode(caps, mapped):
		entries = append(entries, LossEntry{
			Line: line, Column: "Mode", Value: modeRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("CHIRP mode %q maps to %q, which %s does not support", modeRaw, mapped, caps.Model),
		})
	default:
		data.Mode = mapped
	}

	// Duplex, Offset and the split TX frequency, CAPABILITY-AWARE since
	// the Icom tier (design D4, adjudication 16). A radio whose bank
	// reaches spec.FieldDuplex takes the Icom branch below — which can
	// carry a per-channel offset, and can express CHIRP's "split" as a
	// stored transmit frequency instead of refusing it. Every radio
	// registered before that tier reaches neither field, takes the
	// original shift branch, and behaves exactly as it did.
	if reaches(caps, memBank.ID, spec.FieldDuplex) {
		entries = append(entries, importCHIRPDuplexIcom(line, cell, data, caps, memBank.ID)...)
	} else {
		entries = append(entries, importCHIRPDuplexShift(line, cell, data, caps)...)
	}

	// Tone/rToneFreq/cToneFreq (and, on a radio that has them,
	// DtcsCode/DtcsPolarity), likewise capability-aware: a radio whose
	// bank reaches spec.FieldToneMode gets the tone-mode vocabulary,
	// including the DTCS and cross modes the Yaesu branch has no field
	// for and correctly refuses.
	if reaches(caps, memBank.ID, spec.FieldToneMode) {
		entries = append(entries, importCHIRPToneIcom(line, cell, data, caps, memBank.ID)...)
	} else {
		entries = append(entries, importCHIRPToneCTCSS(line, cell, data, caps)...)
	}

	// Skip -> ScanSkip: from the TARGET BANK's own support, never the
	// cell alone — see chirpScanSkip for the rule and why an unreachable
	// radio may not be told the file's answer.
	scanSkip, scanSkipEntries := chirpScanSkip(caps, memBank.ID, cell("Skip"), line)
	data.ScanSkip = scanSkip
	entries = append(entries, scanSkipEntries...)

	// Recognised-but-unmapped columns: silent when at their default,
	// non-blocking dropped when carrying real data (see
	// chirpExtraColumns). A column this radio DOES have a field for is
	// not unmapped at all and is skipped here — otherwise a successfully
	// imported DTCS code would be reported as discarded in the same
	// breath as it was stored.
	for _, col := range chirpExtraColumns {
		if consumedByThisRadio(caps, memBank.ID, col) {
			continue
		}
		v := cell(col)
		if v == "" || v == chirpExtraColumnDefaults[col] {
			continue
		}
		entries = append(entries, LossEntry{
			Line: line, Column: col, Value: v, Action: ActionDropped, Blocking: false,
			Detail: fmt.Sprintf("%s has no equivalent field for CHIRP %s; value discarded", caps.Model, col),
		})
	}

	return &codeplug.Channel{Slot: slot, Data: data}, entries
}

// consumedByThisRadio reports whether a chirpExtraColumns column is
// actually mapped onto a field of THIS radio, and so must not also be
// reported as dropped. Only the two DTCS columns can be: TStep and
// Comment have no neutral field on any radio this project models.
func consumedByThisRadio(caps spec.Capabilities, bank spec.BankID, column string) bool {
	switch column {
	case "DtcsCode":
		return reaches(caps, bank, spec.FieldDTCSCode)
	case "DtcsPolarity":
		return reaches(caps, bank, spec.FieldDTCSPolarity)
	default:
		return false
	}
}

// importCHIRPDuplexShift is the pre-Icom-tier Duplex mapping, unchanged
// and still the one every registered radio takes: CHIRP's Duplex becomes
// a repeater SHIFT, asked for by direction rather than named here,
// "split" is refused because a Yaesu memory channel has no independent
// transmit frequency, and a non-zero Offset is dropped because the shift
// magnitude is a global menu setting.
func importCHIRPDuplexShift(line int, cell func(string) string, data *codeplug.ChannelData, caps spec.Capabilities) []LossEntry {
	var entries []LossEntry
	switch duplexRaw := cell("Duplex"); duplexRaw {
	case "", "off":
		v, ok := shiftFor(caps, spec.ShiftNone)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no simplex shift option", caps.Model),
			})
			break
		}
		data.Shift = v
		if duplexRaw == "off" {
			entries = append(entries, LossEntry{
				Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionDropped, Blocking: false,
				Detail: fmt.Sprintf("CHIRP \"off\" duplex mapped to %s; the distinction between \"no duplex configured\" and \"simplex\" is not representable", v),
			})
		}
	case "+", "-":
		dir, label := spec.ShiftUp, "up"
		if duplexRaw == "-" {
			dir, label = spec.ShiftDown, "down"
		}
		v, ok := shiftFor(caps, dir)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no %s-shift option", caps.Model, label),
			})
			break
		}
		data.Shift = v
	case "split":
		entries = append(entries, LossEntry{
			Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("split-frequency duplex (independent TX/RX frequencies) has no %s equivalent", caps.Model),
		})
	default:
		entries = append(entries, LossEntry{
			Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("unrecognised Duplex value %q", duplexRaw),
		})
	}

	// Offset: the radio has no per-channel repeater offset at all.
	if offsetRaw := cell("Offset"); isNonZeroCHIRPOffset(offsetRaw) {
		entries = append(entries, LossEntry{
			Line: line, Column: "Offset", Value: offsetRaw, Action: ActionDropped, Blocking: false,
			Detail: fmt.Sprintf("%s stores no per-channel repeater offset; shift magnitude is a global menu setting", caps.Model),
		})
	}
	return entries
}

// importCHIRPToneCTCSS is the pre-Icom-tier Tone mapping, unchanged:
// CHIRP's Tone column becomes a CTCSS STATE from this radio's own
// vocabulary, DTCS and Cross are refused because there is no field for
// them, and rToneFreq/cToneFreq are read ONLY when the Tone column
// selects them — CHIRP populates both on essentially every row
// regardless of which (if either) is active, so treating the inactive
// one as lost data would swamp the report with noise on every ordinary
// row.
func importCHIRPToneCTCSS(line int, cell func(string) string, data *codeplug.ChannelData, caps spec.Capabilities) []LossEntry {
	var entries []LossEntry
	switch toneRaw := cell("Tone"); toneRaw {
	case "":
		data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
		v, ok := toneStateFor(caps, spec.ToneOff)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Tone", Value: toneRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no off CTCSS state", caps.Model),
			})
			break
		}
		data.CTCSS = v
	case "Tone":
		rToneRaw := cell("rToneFreq")
		data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
		v, ok := toneStateFor(caps, spec.ToneEncode)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Tone", Value: toneRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no encode-only CTCSS state", caps.Model),
			})
			break
		}
		data.CTCSS = v
		if tone, ok := parseCHIRPTone(rToneRaw, caps); ok {
			// CAT cannot yet write a per-channel CTCSS tone (see
			// spec.FieldCTCSSTone / testCapabilities Write:Unverified),
			// but the VALUE is genuinely known here, so it is recorded
			// Known rather than guessed away; any consequence of the
			// CAT write gap is a separate, later concern (Validate's
			// tone-pairing warning covers the case where CTCSS is
			// enabled without a Known tone, which this is not).
			data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: tone}
		} else {
			entries = append(entries, LossEntry{
				Line: line, Column: "rToneFreq", Value: rToneRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("tone frequency is not in the %s's CTCSS chart", caps.Model),
			})
		}
	case "TSQL":
		cToneRaw := cell("cToneFreq")
		data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
		v, ok := toneStateFor(caps, spec.ToneEncodeDecode)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Tone", Value: toneRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no encode+decode CTCSS state", caps.Model),
			})
			break
		}
		data.CTCSS = v
		if tone, ok := parseCHIRPTone(cToneRaw, caps); ok {
			data.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: tone}
		} else {
			entries = append(entries, LossEntry{
				Line: line, Column: "cToneFreq", Value: cToneRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("tone frequency is not in the %s's CTCSS chart", caps.Model),
			})
		}
	case "DTCS", "Cross":
		data.CTCSS = ""
		data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
		entries = append(entries, LossEntry{
			Line: line, Column: "Tone", Value: toneRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("%s CAT has no DCS memory write; %s tone squelch cannot be imported", caps.Model, toneRaw),
		})
	default:
		data.CTCSS = ""
		data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
		entries = append(entries, LossEntry{
			Line: line, Column: "Tone", Value: toneRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("unrecognised Tone value %q", toneRaw),
		})
	}
	return entries
}

// isNonZeroCHIRPOffset reports whether a CHIRP Offset cell represents a
// non-zero value. An empty cell is zero. A cell that fails to parse is
// conservatively treated as non-zero (present, unparseable data is
// reported rather than silently ignored) — Offset never blocks, so this
// only affects whether a "dropped" LossEntry is produced.
func isNonZeroCHIRPOffset(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return true
	}
	return v != 0
}

// ImportCHIRP reads a CHIRP-next format CSV export and returns the best
// effort codeplug.Channels it can build, together with a LossReport
// documenting every place a CHIRP value could not be carried across
// losslessly.
//
// Contract (read this before calling): ImportCHIRP ALWAYS returns the
// fullest Channels slice and LossReport it could build, even when the
// report contains Blocking entries or a non-nil error is also returned
// (e.g. a mid-stream CSV read failure) — this lets a caller preview
// exactly what was and was not understood. It is the CALLER's
// responsibility to check report.HasBlocking() before treating the
// returned channels as usable for a send to a radio: a Blocking entry
// means some value on that row (or the whole row, when its Location
// could not be mapped to a slot at all) has no safe/complete
// interpretation and needs the user's decision. This function itself
// never refuses to return data because of a Blocking entry — the CLI/GUI
// enforces the gate, this function only reports it.
//
// Unlike Import, this consults caps — but only for VOCABULARY AND SHAPE:
// the memory bank's slot space, the tag length, the shift and CTCSS state
// vocabularies, and the mode and CTCSS-tone tables. It still does NOT run
// codeplug.Validate, and a channel with no Blocking entries against it is
// still not thereby guaranteed valid for the radio (its frequency band,
// its per-field write support, its radio identity). Validate remains the
// semantic gate a caller must run before a send.
//
// Header: recognised at minimum are Location, Name, Frequency, Duplex,
// Offset, Tone, rToneFreq, cToneFreq, DtcsCode, DtcsPolarity, Mode,
// TStep, Skip, Comment (see chirpKnownColumns), looked up by name
// (column order does not matter). Location, Frequency and Mode are core:
// if the header is missing any of them, ImportCHIRP returns immediately
// with a non-nil error (there is nothing usable to build a best-effort
// report from).
//
// A column present in the header but OUTSIDE the recognised set is
// unknown to this package entirely. An empty cell in an unknown column
// is silent — this package cannot tell "irrelevant column" from "genuine
// value CHIRP happens to leave blank here" and does not try to. A
// NON-EMPTY cell in an unknown column, however, cannot be safely
// discarded without the user's knowledge: it produces a Blocking
// ActionUnsupported LossEntry naming the column, distinct from the
// recognised-but-unmapped columns (chirpExtraColumns), which are
// non-blocking even when they carry data.
func ImportCHIRP(rd io.Reader, caps spec.Capabilities) ([]codeplug.Channel, LossReport, error) {
	cr := csv.NewReader(rd)
	cr.FieldsPerRecord = -1

	chirpHeader, err := cr.Read()
	if err != nil {
		return nil, LossReport{}, &ParseError{Line: 1, Reason: fmt.Sprintf("reading CHIRP header: %v", err)}
	}

	// Duplicate header names make colIndex ambiguous (a later occurrence
	// would silently win, hiding whichever earlier column it shadows) —
	// checked and rejected before colIndex is even built.
	seenHeader := make(map[string]bool, len(chirpHeader))
	var dupHeader []string
	for _, name := range chirpHeader {
		if seenHeader[name] {
			dupHeader = append(dupHeader, name)
			continue
		}
		seenHeader[name] = true
	}
	if len(dupHeader) > 0 {
		return nil, LossReport{}, &ParseError{Line: 1, Reason: fmt.Sprintf("duplicate CHIRP column(s): %s", strings.Join(dupHeader, ", "))}
	}

	colIndex := make(map[string]int, len(chirpHeader))
	for i, name := range chirpHeader {
		colIndex[name] = i
	}
	var missing []string
	for _, c := range chirpCoreColumns {
		if _, ok := colIndex[c]; !ok {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return nil, LossReport{}, &ParseError{Line: 1, Reason: fmt.Sprintf("missing required CHIRP column(s): %s", strings.Join(missing, ", "))}
	}

	// unrecognisedColumns is every header column name this package does
	// not know at all (see chirpKnownColumns), in header order. A
	// non-empty cell under one of these, on any row, is Blocking — see
	// ImportCHIRP's Header doc paragraph.
	var unrecognisedColumns []string
	for _, name := range chirpHeader {
		if !chirpKnownColumns[name] {
			unrecognisedColumns = append(unrecognisedColumns, name)
		}
	}

	var channels []codeplug.Channel
	var report LossReport

	line := 1
	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// No successful record to get a physical line from here —
			// best-effort fallback, one past whatever the last known-good
			// line was.
			line++
			return channels, report, &ParseError{Line: line, Reason: fmt.Sprintf("reading CHIRP row: %v", err)}
		}
		// This row's PHYSICAL starting line, from the CSV reader itself
		// (Go 1.17+) — NOT a naive per-record counter: a quoted field
		// containing an embedded newline earlier in the file spans
		// multiple physical lines within a single record, and a counter
		// that only advanced once per record would silently under-report
		// every line number after it.
		line, _ = cr.FieldPos(0)

		if len(record) != len(chirpHeader) {
			report.Entries = append(report.Entries, LossEntry{
				Line: line, Column: "", Value: strings.Join(record, ","), Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("row has %d fields, header has %d; row skipped", len(record), len(chirpHeader)),
			})
			continue
		}

		ch, entries := importCHIRPRow(line, colIndex, record, caps)
		report.Entries = append(report.Entries, entries...)

		for _, col := range unrecognisedColumns {
			v := record[colIndex[col]] // colIndex[col] is valid: unrecognisedColumns is built from chirpHeader itself, and the length check above guarantees record is exactly as long.
			if v == "" {
				continue
			}
			report.Entries = append(report.Entries, LossEntry{
				Line: line, Column: col, Value: v, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("column %q is not recognised by this importer; a non-empty value in it cannot be safely discarded", col),
			})
		}

		if ch != nil {
			channels = append(channels, *ch)
		}
	}

	return channels, report, nil
}

// parseCHIRPOffsetHz parses a CHIRP Offset cell — decimal MHz, exactly
// as the Frequency column is — into whole hertz. It reuses
// parseCHIRPFrequency rather than ParseFloat because an offset that ends
// up on a radio deserves the same exactness a frequency does: no
// floating point, and a sub-Hz remainder refused rather than rounded.
// (isNonZeroCHIRPOffset's ParseFloat stays where it is: that one only
// ever decides whether to WRITE A LOSS ENTRY, never what to store.)
func parseCHIRPOffsetHz(s string) (uint64, error) {
	return parseCHIRPFrequency(strings.TrimSpace(s))
}

// importCHIRPDuplexIcom is the Duplex/Offset mapping for a radio whose
// bank reaches spec.FieldDuplex — the Icom tier's branch (design D4).
//
// What it does that the shift branch cannot:
//
//   - stores a per-channel OFFSET (spec.FieldOffset) for a "+"/"-" row,
//     instead of dropping it as a global menu setting;
//   - accepts CHIRP's "split" where the radio has a stored transmit
//     frequency (spec.FieldTxFrequency). CHIRP puts the absolute TX
//     frequency in the Offset column for a split row, so that is where
//     it is read from, and the channel's duplex is set to the radio's
//     own OFF value: a split channel has no repeater shift, it has two
//     frequencies. Where the radio has no such field the row is refused
//     exactly as before.
//
// Every field this radio reaches but this row does not speak to is left
// UNKNOWN, never invented: an import is a statement about a file, and
// the file did not say.
func importCHIRPDuplexIcom(line int, cell func(string) string, data *codeplug.ChannelData, caps spec.Capabilities, bank spec.BankID) []LossEntry {
	var entries []LossEntry
	hasOffset := reaches(caps, bank, spec.FieldOffset)
	hasTxFreq := reaches(caps, bank, spec.FieldTxFrequency)

	// Defaults for a row that turns out to say nothing about them.
	data.Duplex = codeplug.StringField{State: codeplug.Unknown}
	if hasOffset {
		data.OffsetHz = codeplug.FreqField{State: codeplug.Unknown}
	}
	if hasTxFreq {
		data.TxFreqHz = codeplug.FreqField{State: codeplug.Unknown}
	}

	duplexRaw := cell("Duplex")
	offsetRaw := cell("Offset")

	setDuplex := func(dir spec.DuplexDirection, label string) bool {
		v, ok := duplexFor(caps, dir)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no %s duplex option", caps.Model, label),
			})
			return false
		}
		data.Duplex = codeplug.StringField{State: codeplug.Known, Value: v}
		return true
	}

	switch duplexRaw {
	case "", "off":
		if !setDuplex(spec.DuplexOff, "simplex") {
			break
		}
		if hasOffset {
			// A simplex channel's offset is not "unknown": it is nothing.
			data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 0}
		}
	case "+", "-":
		dir, label := spec.DuplexUp, "up-shift"
		if duplexRaw == "-" {
			dir, label = spec.DuplexDown, "down-shift"
		}
		if !setDuplex(dir, label) {
			break
		}
		if !hasOffset {
			if isNonZeroCHIRPOffset(offsetRaw) {
				entries = append(entries, LossEntry{
					Line: line, Column: "Offset", Value: offsetRaw, Action: ActionDropped, Blocking: false,
					Detail: fmt.Sprintf("%s stores no per-channel repeater offset; the shift direction was kept and the magnitude discarded", caps.Model),
				})
			}
			break
		}
		hz, err := parseCHIRPOffsetHz(offsetRaw)
		if err != nil {
			entries = append(entries, LossEntry{
				Line: line, Column: "Offset", Value: offsetRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("a %s repeater shift needs an offset, and %q is not a decimal MHz value the %s can store", duplexRaw, offsetRaw, caps.Model),
			})
			break
		}
		data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: hz}
	case "split":
		if !hasTxFreq {
			entries = append(entries, LossEntry{
				Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("split-frequency duplex (independent TX/RX frequencies) has no %s equivalent", caps.Model),
			})
			break
		}
		// CHIRP carries the absolute TX frequency in the Offset column
		// for a split row.
		hz, err := parseCHIRPOffsetHz(offsetRaw)
		if err != nil {
			entries = append(entries, LossEntry{
				Line: line, Column: "Offset", Value: offsetRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("a split row's Offset column carries the transmit frequency, and %q is not a decimal MHz value the %s can store", offsetRaw, caps.Model),
			})
			break
		}
		data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: hz}
		if v, ok := duplexFor(caps, spec.DuplexOff); ok {
			// A split channel has two frequencies, not a shift.
			data.Duplex = codeplug.StringField{State: codeplug.Known, Value: v}
		}
		if hasOffset {
			data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 0}
		}
	default:
		entries = append(entries, LossEntry{
			Line: line, Column: "Duplex", Value: duplexRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("unrecognised Duplex value %q", duplexRaw),
		})
	}
	return entries
}

// importCHIRPToneIcom is the Tone mapping for a radio whose bank reaches
// spec.FieldToneMode — the Icom tier's branch (design D4).
//
// What it does that the CTCSS branch cannot: carry the TRANSMIT and
// RECEIVE tones independently (spec.FieldToneTx / spec.FieldToneRx), and
// map CHIRP's DTCS row onto a real code and polarity instead of refusing
// it. CHIRP's "Cross" is mapped only if this radio declares a cross
// mode, and is otherwise refused with the same blocking entry as before:
// this tier models the cross MODE, not the combination behind it.
//
// A TSQL row sets BOTH tones from cToneFreq, and that is a mapping
// decision worth stating: tone squelch means transmitting a tone and
// requiring the same one back, so one CHIRP column genuinely describes
// both directions. It is not an invention — CHIRP's own semantics say
// so.
func importCHIRPToneIcom(line int, cell func(string) string, data *codeplug.ChannelData, caps spec.Capabilities, bank spec.BankID) []LossEntry {
	var entries []LossEntry
	hasTxTone := reaches(caps, bank, spec.FieldToneTx)
	hasRxTone := reaches(caps, bank, spec.FieldToneRx)
	hasDTCSCode := reaches(caps, bank, spec.FieldDTCSCode)
	hasDTCSPol := reaches(caps, bank, spec.FieldDTCSPolarity)

	// Unknown everywhere this radio has a field and the row has not yet
	// spoken.
	data.ToneMode = codeplug.StringField{State: codeplug.Unknown}
	if hasTxTone {
		data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
	}
	if hasRxTone {
		data.ToneRx = codeplug.ToneField{State: codeplug.Unknown}
	}
	if hasDTCSCode {
		data.DTCSCode = codeplug.IntField{State: codeplug.Unknown}
	}
	if hasDTCSPol {
		data.DTCSPolarity = codeplug.StringField{State: codeplug.Unknown}
	}

	toneRaw := cell("Tone")
	setMode := func(sem spec.ToneModeSemantics, label string) bool {
		v, ok := toneModeFor(caps, sem)
		if !ok {
			entries = append(entries, LossEntry{
				Line: line, Column: "Tone", Value: toneRaw, Action: ActionUnsupported, Blocking: true,
				Detail: fmt.Sprintf("%s expresses no %s tone mode", caps.Model, label),
			})
			return false
		}
		data.ToneMode = codeplug.StringField{State: codeplug.Known, Value: v}
		return true
	}
	// setTone parses one CHIRP tone cell into the named field, reporting
	// a blocking entry when the value is not in this radio's chart —
	// the same rule and the same wording the CTCSS branch uses.
	setTone := func(column, raw string, into *codeplug.ToneField) {
		if tone, ok := parseCHIRPTone(raw, caps); ok {
			*into = codeplug.ToneField{State: codeplug.Known, Value: tone}
			return
		}
		entries = append(entries, LossEntry{
			Line: line, Column: column, Value: raw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("tone frequency is not in the %s's CTCSS chart", caps.Model),
		})
	}

	switch toneRaw {
	case "":
		setMode(spec.ToneModeOff, "off")
	case "Tone":
		if !setMode(spec.ToneModeCTCSS, "encode-only") {
			break
		}
		if hasTxTone {
			setTone("rToneFreq", cell("rToneFreq"), &data.ToneTx)
		}
	case "TSQL":
		if !setMode(spec.ToneModeCTCSSSquelch, "encode+decode") {
			break
		}
		cTone := cell("cToneFreq")
		if hasRxTone {
			setTone("cToneFreq", cTone, &data.ToneRx)
		}
		if hasTxTone {
			// Both directions from one column — see this function's doc
			// comment for why that is CHIRP's own semantics, not a guess.
			setTone("cToneFreq", cTone, &data.ToneTx)
		}
	case "DTCS":
		if !setMode(spec.ToneModeDTCS, "DTCS") {
			break
		}
		if hasDTCSCode {
			raw := cell("DtcsCode")
			code, err := strconv.Atoi(strings.TrimSpace(raw))
			switch {
			case err != nil:
				entries = append(entries, LossEntry{
					Line: line, Column: "DtcsCode", Value: raw, Action: ActionUnsupported, Blocking: true,
					Detail: fmt.Sprintf("DtcsCode %q is not a valid code number", raw),
				})
			case !capsHasDTCSCode(caps, code):
				entries = append(entries, LossEntry{
					Line: line, Column: "DtcsCode", Value: raw, Action: ActionUnsupported, Blocking: true,
					Detail: fmt.Sprintf("DTCS code %q is not in the %s's code table", raw, caps.Model),
				})
			default:
				data.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: code}
			}
		}
		if hasDTCSPol {
			raw := strings.TrimSpace(cell("DtcsPolarity"))
			if capsHasDTCSPolarity(caps, raw) {
				data.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: raw}
			} else {
				entries = append(entries, LossEntry{
					Line: line, Column: "DtcsPolarity", Value: raw, Action: ActionUnsupported, Blocking: true,
					Detail: fmt.Sprintf("DTCS polarity %q is not one the %s expresses", raw, caps.Model),
				})
			}
		}
	case "Cross":
		if !setMode(spec.ToneModeCross, "cross") {
			break
		}
	default:
		entries = append(entries, LossEntry{
			Line: line, Column: "Tone", Value: toneRaw, Action: ActionUnsupported, Blocking: true,
			Detail: fmt.Sprintf("unrecognised Tone value %q", toneRaw),
		})
	}
	return entries
}

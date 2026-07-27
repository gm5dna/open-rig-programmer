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
	// FT-710 has no field to hold it at all.
	ActionDropped = "dropped"
	// ActionApproximated means the CHIRP data was kept but altered to
	// fit the FT-710's constraints (e.g. a tag truncated or
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
// no FT-710 field for. Per row, an empty cell in one of these is
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
// recognises at all, whether or not it maps to an FT-710 field
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
// display-name equivalent. CW and CWR both exist on the FT-710 as
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
// must all be zero, or the value has a sub-Hz remainder the FT-710
// cannot store (errCHIRPFreqFractionalHz).
func parseCHIRPFrequency(s string) (uint32, error) {
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
	// land back inside uint32 range and be accepted as if it were a
	// small, legitimate frequency — exactly the kind of miscomputed value
	// this project must never send to a radio. Rearranging the bound
	// (mhz > (max-hzFrac)/1_000_000) answers the same question without
	// ever performing the multiplication that could overflow.
	max := uint64(math.MaxUint32)
	if hzFrac > max || mhz > (max-hzFrac)/1_000_000 {
		return 0, errCHIRPFreqRange
	}
	total := mhz*1_000_000 + hzFrac
	return uint32(total), nil
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
// encode/decode combination, and true, or ("", false) when this radio
// expresses no such state. As with shiftFor, spec.Validate guarantees at
// most one state per combination.
func toneStateFor(caps spec.Capabilities, encodes, decodes bool) (string, bool) {
	for _, s := range caps.CTCSSStates {
		if s.Encodes == encodes && s.Decodes == decodes {
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

// chirpTagByteOK reports whether b is a legal FT-710 tag byte: printable
// ASCII 0x20-0x7E, excluding ';' (0x3B). This restates
// codeplug's validTagByte (which is unexported) rather than reaching
// into that package for it — core/csvio depends only on core/codeplug's
// exported surface, core/spec, and stdlib.
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

// importCHIRPRow builds the Channel one CHIRP data row describes (nil if
// its Location cannot be mapped to any FT-710 slot at all) and every
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

	// Frequency -> FreqHz.
	freqRaw := cell("Frequency")
	freqHz, err := parseCHIRPFrequency(freqRaw)
	if err != nil {
		detail := "Frequency is not a valid decimal MHz value"
		switch {
		case errors.Is(err, errCHIRPFreqFractionalHz):
			detail = "Frequency has a sub-Hz fractional remainder the FT-710 cannot store"
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

	// Duplex -> Shift. Which wire value means "up", "down" or "none" is
	// this radio's own vocabulary, so it is asked for by DIRECTION rather
	// than named here.
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

	// Offset: FT-710 has no per-channel repeater offset at all.
	if offsetRaw := cell("Offset"); isNonZeroCHIRPOffset(offsetRaw) {
		entries = append(entries, LossEntry{
			Line: line, Column: "Offset", Value: offsetRaw, Action: ActionDropped, Blocking: false,
			Detail: "FT-710 stores no per-channel repeater offset; shift magnitude is a global menu setting",
		})
	}

	// Tone/rToneFreq/cToneFreq -> CTCSS/CTCSSTone. rToneFreq/cToneFreq
	// are read ONLY when the Tone column selects them: CHIRP populates
	// both on essentially every row regardless of which (if either) is
	// active, so treating the inactive one as "lost data" would swamp
	// the report with noise on every ordinary row.
	switch toneRaw := cell("Tone"); toneRaw {
	case "":
		data.CTCSSTone = codeplug.ToneField{State: codeplug.Unknown}
		v, ok := toneStateFor(caps, false, false)
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
		v, ok := toneStateFor(caps, true, false)
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
		v, ok := toneStateFor(caps, true, true)
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

	// Skip -> ScanSkip.
	switch skipRaw := cell("Skip"); skipRaw {
	case "S":
		data.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
	case "":
		data.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: false}
	default:
		data.ScanSkip = codeplug.BoolField{State: codeplug.Unknown}
		entries = append(entries, LossEntry{
			Line: line, Column: "Skip", Value: skipRaw, Action: ActionDropped, Blocking: false,
			Detail: fmt.Sprintf("CHIRP Skip value %q has no FT-710 equivalent; scan-skip left unresolved", skipRaw),
		})
	}

	// Recognised-but-unmapped columns: silent when at their default,
	// non-blocking dropped when carrying real data (see
	// chirpExtraColumns).
	for _, col := range chirpExtraColumns {
		v := cell(col)
		if v == "" || v == chirpExtraColumnDefaults[col] {
			continue
		}
		entries = append(entries, LossEntry{
			Line: line, Column: col, Value: v, Action: ActionDropped, Blocking: false,
			Detail: fmt.Sprintf("FT-710 has no equivalent field for CHIRP %s; value discarded", col),
		})
	}

	return &codeplug.Channel{Slot: slot, Data: data}, entries
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
// Like Import, this is a SYNTACTIC transform: it does not consult
// codeplug.Validate or any Capabilities, and a channel with no Blocking
// entries against it is not thereby guaranteed valid for any particular
// radio (e.g. its frequency band). Validate remains the semantic gate
// callers must run before a send.
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

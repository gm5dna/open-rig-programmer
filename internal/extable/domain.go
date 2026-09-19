// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"regexp"
	"strconv"
	"strings"
)

// Domain is a Table 2 address's value domain, parsed from its P4 legend —
// the manual's parameter-description column (Row.P4, ParseCSV), carried
// today only as audit text (extable.go:45-58,586-588). Codes holds
// enumerated sentinel values (e.g. 0 for "OFF"); Lo/Hi/Step describe a
// stepped numeric range (Step defaults to 1; Hi == 0 means no range);
// Signed marks a range whose wire form carries an explicit sign character —
// -00 and +00 are two distinct legal codes, not one zero (milestone spec
// §2's 26 sign-bearing addresses). Both arms may be populated at once: a
// hybrid legend mixing a code with a range (table2.csv:201, below).
//
// core/cat/domain.go (task b2) declares the same four fields as its own
// type, generated into as literal Go source by RenderWriteGo — this Domain
// is build-time tooling only, the same "never imported by core/cat" seam
// extable.go's own doc comment already keeps (ex_crosscheck_test.go,
// internal/fakeradio/ex.go:161-168).
type Domain struct {
	Codes        []int
	Lo, Hi, Step int
	Signed       bool
}

// Contains reports whether v is a legal value of d: an enumerated code, or
// inside the stepped Lo..Hi range on a Step boundary.
func (d Domain) Contains(v int) bool {
	for _, c := range d.Codes {
		if c == v {
			return true
		}
	}
	if d.Hi == 0 {
		return false
	}
	step := d.Step
	if step == 0 {
		step = 1
	}
	return v >= d.Lo && v <= d.Hi && (v-d.Lo)%step == 0
}

// signedRangeRe matches the sign-bearing range shape (spec §2's 26
// addresses): "-20 - -00 (or +00) - +10 ...". -00/+00 are the same wire
// value under two spellings, so only the outer bounds are captured.
var signedRangeRe = regexp.MustCompile(`^([-+]\d+)\s*-\s*[-+]00\s*\(or\s*[-+]00\)\s*-\s*([-+]\d+)`)

// steppedP4Re matches a P4-parenthetical stepped range: "(P4= 0020 - 4000,
// 20 msec/step)", "(P4 = 0000 - 1000, 10 kHz/step)", "(P4 = 0000 - 3000, 10
// Hz steps)" — three unit spellings (slash or not, singular or plural), so
// the step's unit text is matched loosely up to the literal "step".
var steppedP4Re = regexp.MustCompile(`\(P4\s*=\s*(\d+)\s*-\s*(\d+)\s*,\s*(\d+)[^,)]*?step`)

// plainP4Re matches a P4-parenthetical range with no step: "(P4 = 005 -
// 100)". Tried after steppedP4Re, whose trailing comma this pattern's
// closing ")" excludes.
var plainP4Re = regexp.MustCompile(`\(P4\s*=\s*(\d+)\s*-\s*(\d+)\)`)

// pureRangeRe matches a bare numeric range with no P4 parenthetical at all
// (table2.csv:57 USB MOD GAIN, "000 - 100") — anchored at the start, so it
// never matches an enum's per-code label range (table2.csv:55 TX BPF SEL,
// "0: 50 - 3050 ...", which starts "0:" not a digit-hyphen pair).
var pureRangeRe = regexp.MustCompile(`^(\d+)\s*-\s*(\d+)\b`)

// codeRe matches one enumerated P4 code entry: digits, then a colon
// (optionally preceded by whitespace — table2.csv:201's "00 : OFF", the
// hybrid fixture's own space-before-colon), then trailing whitespace.
var codeRe = regexp.MustCompile(`(\d+)\s*:\s*`)

// ParseP4Domain parses p4 — Table 2's manual parameter-description legend
// (Row.P4) — into a Domain. ok is false for a legend none of the patterns
// below recognise; the milestone spec (§2) holds such an address read-only
// regardless of Session W.
func ParseP4Domain(p4 string) (Domain, bool) {
	if m := signedRangeRe.FindStringSubmatch(p4); m != nil {
		lo, loErr := strconv.Atoi(m[1])
		hi, hiErr := strconv.Atoi(m[2])
		if loErr != nil || hiErr != nil {
			return Domain{}, false
		}
		return Domain{Lo: lo, Hi: hi, Step: 1, Signed: true}, true
	}
	if strings.Contains(p4, ":") {
		return parseEnumOrHybrid(p4)
	}
	if m := steppedP4Re.FindStringSubmatch(p4); m != nil {
		lo, loErr := strconv.Atoi(m[1])
		hi, hiErr := strconv.Atoi(m[2])
		step, stepErr := strconv.Atoi(m[3])
		if loErr != nil || hiErr != nil || stepErr != nil || step == 0 {
			return Domain{}, false
		}
		return Domain{Lo: lo, Hi: hi, Step: step}, true
	}
	if m := plainP4Re.FindStringSubmatch(p4); m != nil {
		lo, loErr := strconv.Atoi(m[1])
		hi, hiErr := strconv.Atoi(m[2])
		if loErr != nil || hiErr != nil {
			return Domain{}, false
		}
		return Domain{Lo: lo, Hi: hi, Step: 1}, true
	}
	if m := pureRangeRe.FindStringSubmatch(p4); m != nil {
		lo, loErr := strconv.Atoi(m[1])
		hi, hiErr := strconv.Atoi(m[2])
		if loErr != nil || hiErr != nil {
			return Domain{}, false
		}
		return Domain{Lo: lo, Hi: hi, Step: 1}, true
	}
	return Domain{}, false
}

// parseEnumOrHybrid handles every colon-bearing legend: a pure enum
// (table2.csv:55 TX BPF SEL — each code's label happens to contain its own
// hyphenated range, which carries no colon of its own and so is never
// mistaken for a code boundary) and a hybrid legend (table2.csv:201
// PRMTRC EQ1 FREQ, "00 : OFF 01: 100 Hz - 07: 700 Hz"): a hyphen directly
// linking one code's label to the next code entry means the range is the
// two CODES flanking that hyphen (01, 07) — not the labels after each colon
// (100 Hz, 700 Hz), the opposite reading from a stepped range's own hyphen
// (table2.csv:46), where the numbers flanking IT are themselves the range.
// Any other codes (00, here) are kept as enumerated sentinels.
//
// ponytail: a code with no colon at all (table2.csv:191's "17 ATT" typo,
// between "16:BAND DOWN" and "18:IPO") is swallowed into the PRECEDING
// code's label text rather than recognised as its own code — the resulting
// Domain silently omits it. Safe by construction (Contains then refuses a
// legal value rather than admitting an illegal one, never the reverse), but
// a real gap; fix by requiring codes strictly increasing by 1 and inserting
// the missing one, if Session R's sweep ever needs that address writable.
func parseEnumOrHybrid(p4 string) (Domain, bool) {
	locs := codeRe.FindAllStringSubmatchIndex(p4, -1)
	if locs == nil {
		return Domain{}, false
	}
	codes := make([]int, len(locs))
	for i, m := range locs {
		n, err := strconv.Atoi(p4[m[2]:m[3]])
		if err != nil {
			return Domain{}, false
		}
		codes[i] = n
	}
	for i := 1; i < len(locs); i++ {
		between := strings.TrimSpace(p4[locs[i-1][1]:locs[i][0]])
		if !strings.HasSuffix(between, "-") {
			continue
		}
		lo, hi := codes[i-1], codes[i]
		rest := make([]int, 0, len(codes)-2)
		rest = append(rest, codes[:i-1]...)
		rest = append(rest, codes[i+1:]...)
		return Domain{Codes: rest, Lo: lo, Hi: hi, Step: 1}, true
	}
	return Domain{Codes: codes}, true
}

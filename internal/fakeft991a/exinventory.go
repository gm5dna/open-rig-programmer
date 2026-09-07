// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// # STANDARD LIBRARY ONLY, and why that is the whole point
//
// The parsing below imports nothing project-internal, and in particular NOT
// internal/extable — the machinery that derives the DIALECT's inventory from
// transcription A. imports_test.go's recursive fence enforces that
// mechanically, and the reason is the design this file exists to serve: the
// dialect's inventory comes from transcription A by one piece of code, this
// fake's from transcription B by the code below, and core/transport's
// cross-check proves the two agree. One parser on both sides of that
// comparison would reproduce a shared parsing bug into both inventories
// invisibly.
//
// It was a generator emitting a checked-in table until 06/09/2026. The
// generated file, and the second copy of the parser it needed, are gone; the
// projection now happens at init from the same committed bytes. The CSV itself
// has NOT moved — core/transport's cross-check reads it by path.

// transcriptionB is this package's OWN COPY of transcription B, embedded and
// projected here at init. PROVENANCE.md records where the copy came from and
// why it is a copy rather than a move.
//
//go:embed transcription-b.csv
var transcriptionB []byte

// exItems is this fake's EX (MENU) inventory: one entry per menu address, in
// the chart's own order, with the raw P4 reply WIDTH that address answers as a
// token '1'..'8' — a numeric field of that many raw ASCII bytes. There is NO
// text token: this chart's transcription carries no column from which a text
// item could be identified, so every row is projected as numeric (widthToken
// says what that does and does not claim).
//
// THE ADDRESS IS A SINGLE COMPONENT: the chart's three-digit MENU Number is the
// whole address, with P2 and P3 zero (core/cat's EXAddressSingle), which is why
// this radio's EX read frame is six bytes — the narrowest in the family. There
// are no groups, so this is a flat list rather than the FT-891's widths
// strings.
//
// The parameterless rows are EXCLUDED but still counted in the run: the chart
// prints them with no parameter at all, so they name no field an EX frame could
// read or write (parameterlessToken, and plan decision P18 on why the two
// transcriptions spell one printed hyphen differently).
var exItems = mustItems(transcriptionB)

// mustItems is the init-time projection. Every malformed input PANICS rather
// than yielding a shorter table: this CSV is a committed, hash-frozen
// evidential artefact, so anything the projection cannot read is a finding, and
// a fake answering from a truncated inventory would be worse than one that
// refuses to start.
func mustItems(data []byte) []exItem {
	rows, err := parseB(data)
	if err != nil {
		panic("fakeft991a: the embedded transcription B: " + err.Error())
	}
	if err := checkRun(rows); err != nil {
		panic("fakeft991a: the embedded transcription B: " + err.Error())
	}
	out := make([]exItem, 0, len(rows))
	for _, r := range rows {
		if r.excluded {
			continue
		}
		out = append(out, exItem{addr: r.addr, width: r.token})
	}
	if len(out) == 0 {
		panic("fakeft991a: the embedded transcription B projected no items")
	}
	return out
}

// bHeader is transcription B's exact header row, as delivered and committed.
// It is pinned so that a schema change fails LOUDLY here rather than being
// silently misparsed into a plausible wrong table.
//
// IT IS THE FT-891'S HEADER TOO, BYTE FOR BYTE, and that is the one place this
// generator's refusals cannot help: the two radios' transcriptions were
// delivered in the same three-column schema by the same brief. What separates
// them is the ADDRESS WIDTH — parseMenuNumber refuses anything that is not
// exactly three digits, so an FT-891 row ("0101,AGC FAST DELAY,4") is rejected
// at its first data line rather than read as a plausible FT-991A address.
var bHeader = []string{"menu_number", "name", "digits"}

// Column indices into a B record.
const (
	colMenuNumber = iota
	colName
	colDigits
	numCols
)

// menuNumberDigits is the width of B's address cell: the chart's three-digit
// MENU Number, "P1 : 001 - 153 (MENU Number)" as the EX block prints it. There
// is no second or third component, which is what makes this radio's EX read
// frame six bytes — the narrowest in the family.
const menuNumberDigits = 3

// maxWidth is the widest raw P4 field this chart declares: 8. It comes from
// exactly ONE row, 151 PRESET FREQUENCY, whose "00030000 ~ 47000000" parameter
// is eight digits wide, and it is pinned independently from both sides —
// exinventory_test.go's TestParseB_TheOnlyEightWideRowIs151 from B, and
// core/cat/ft991a/crosscheck_test.go's widestRowDigits/widestRowAddr from A.
//
// EIGHT, where the FT-891's alphabet stops at five and the FTdx10's at four.
// It is a reading of THIS chart and not a widening of theirs.
const maxWidth = 8

// parameterlessToken is the Digits cell this transcription writes for a row
// that prints NO parameter: a question mark.
//
// # IT IS '?' HERE AND '-' IN internal/extable, AND BOTH ARE RIGHT (plan P18)
//
// One glyph is printed on the page. Row 087 RADIO ID's Digits cell prints a
// SINGLE HYPHEN, and its parameter legend prints ten spaced hyphens; both were
// confirmed at 900 dpi by the quarantined agent that produced this artefact
// (core/cat/ft991a/testdata/transcription-b.md §2(b)).
//
// The two derivations of that page then SPELL it differently, because they
// were written under different transcription conventions:
//
//   - TRANSCRIPTION A keeps the raw '-', and internal/extable's
//     ParameterlessExcluded policy keys on that byte
//     (internal/extable/profile.go's ft991aProfile);
//   - TRANSCRIPTION B — this file's source — was told to write '?' for any
//     cell that is not an integer, so it reads "087,RADIO ID,?" and its own
//     report says in terms that the cell prints a hyphen.
//
// So this generator's token is '?' BECAUSE THAT IS WHAT ITS OWN SOURCE SPELLS.
// It is not extable's bound consulted from here, nor extable's bound restated
// here: each generator reads the spelling of the artefact it is generating
// from, which is the project's standing rule that a bound is consulted from
// the same place as its datum. Teaching this generator extable's '-' would
// break that rule in the one direction that matters — it would make this side
// of the cross-check depend on the other side's reading of the page.
//
// The milestone's plan states this ruling (P18) rather than leaving the two
// generators, written weeks apart, to meet it separately. Task 6's cross-check
// of the two transcriptions normalises '?' to '-' on this ONE address at
// comparison time, and never by editing either artefact.
const parameterlessToken = "?"

// parameterlessAddrs are the addresses this chart prints with no parameter at
// all, and which are therefore EXCLUDED from the inventory: a menu number
// naming no field is not an address an EX frame could read or write.
//
// It is spelt here as this generator's OWN declaration, read from this
// package's own source artefact — not imported from, nor derived from,
// internal/extable's ft991aProfile.ParameterlessAddresses, which says the same
// thing about the same row from transcription A. Two independent readings of
// one page is the whole mechanism of the cross-check; one of them consulting
// the other would dissolve it.
//
// The rule is enforced in BOTH directions (parseB): a '?' cell on an address
// in this set is excluded, and a '?' cell on any other address is REFUSED —
// as is an address in this set whose cell is NOT '?', which would mean the
// chart, or its transcription, had changed under a declaration that no longer
// describes it.
var parameterlessAddrs = map[string]bool{"087": true}

// row is one parsed B data row, reduced to what the projection needs.
type row struct {
	// addr is the whole EX address: the menu number's three ASCII digits, as
	// the wire carries them.
	addr string
	// num is that address as a number, for the consecutive-run check.
	num int
	// token is the width token: '1'..'8' for a numeric field of that many
	// bytes. There is no text token — see widthToken.
	token byte
	// excluded marks a row that is transcribed and counted but names no
	// field: parameterlessAddrs. Its token is meaningless and it is not
	// rendered.
	excluded bool
	// line is the 1-based physical line in the CSV, header included, for
	// error messages and for the emitted provenance comment.
	line int
}

// parseB parses transcription B into rows, in file order. Every malformed
// input is an error rather than a skipped row: this CSV is a committed,
// hash-frozen evidential artefact, so anything the projection cannot read is a
// finding to report, never a row to drop.
//
// The ONE row it does not project is the parameterless one, and that is not a
// skip: it is parsed, counted and marked, so that checkRun still sees the
// chart's whole run of addresses and the rendered file can name what it left
// out.
func parseB(data []byte) ([]row, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = numCols // the header sets it; this states it up front
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("empty CSV: no header row")
	}
	if got := records[0]; !slices.Equal(got, bHeader) {
		return nil, fmt.Errorf("header row is %v, want %v — transcription B's schema changed, or the wrong file was read", got, bHeader)
	}
	if len(records) == 1 {
		return nil, fmt.Errorf("CSV has a header and no data rows")
	}

	out := make([]row, 0, len(records)-1)
	for i, rec := range records[1:] {
		line := i + 2 // 1-based, header included
		addr, num, err := parseMenuNumber(rec[colMenuNumber])
		if err != nil {
			return nil, fmt.Errorf("line %d: menu_number cell: %w", line, err)
		}
		if strings.TrimSpace(rec[colName]) == "" {
			return nil, fmt.Errorf("line %d (%s): empty name cell — a blank name is the signature of a misparsed row, not an item without a name", line, rec[colMenuNumber])
		}

		digits := strings.TrimSpace(rec[colDigits])
		parameterless := parameterlessAddrs[addr]
		switch {
		case digits == parameterlessToken && parameterless:
			// The declared parameterless row, spelt as its own artefact spells
			// it. Counted, not projected — see parameterlessToken.
			out = append(out, row{addr: addr, num: num, excluded: true, line: line})
			continue
		case digits == parameterlessToken:
			return nil, fmt.Errorf("line %d (%s %s): digits cell %q means NO PARAMETER, and %s is not one of this generator's declared parameterless addresses (%v) — either the chart prints a row this generator does not know is parameterless, or the transcription has drifted; arbitrate against the PDF rather than widening the declaration to fit",
				line, rec[colMenuNumber], rec[colName], parameterlessToken, addr, sortedAddrs())
		case parameterless:
			return nil, fmt.Errorf("line %d (%s %s): %s is declared parameterless but its digits cell is %q, not %q — the chart, or its transcription, no longer matches the declaration",
				line, rec[colMenuNumber], rec[colName], addr, digits, parameterlessToken)
		}

		token, err := widthToken(digits)
		if err != nil {
			return nil, fmt.Errorf("line %d (%s %s): %w", line, rec[colMenuNumber], rec[colName], err)
		}
		out = append(out, row{addr: addr, num: num, token: token, line: line})
	}
	return out, nil
}

// sortedAddrs renders parameterlessAddrs deterministically, for an error
// message that must not vary between runs over one map.
func sortedAddrs() []string {
	out := make([]string, 0, len(parameterlessAddrs))
	for a := range parameterlessAddrs {
		out = append(out, a)
	}
	slices.Sort(out)
	return out
}

// widthToken derives one width token from B's digits cell.
//
// A width of 1..maxWidth is a numeric field of that many raw ASCII bytes and
// yields that digit itself. EVERY OTHER VALUE IS REFUSED, including 12 — and
// the refusal of 12 is the one worth stating, because it is where this
// generator, like the FT-891's, deliberately declines to do what the FTdx10's
// does.
//
// That generator reads a 12 as the text item (MY CALL.) when B's P4 cell also
// describes a character count, and emits a 'T' token which the fake expands to
// twelve SPACES rather than twelve zeros. THIS CHART'S B HAS NO SUCH CELL:
// three columns, no parameter legend, no text flag. So there is nothing here
// from which textness could be decided, and deciding it on the width alone
// would invent wire behaviour. The quarantined agent looked for one and
// recorded finding none — "Free-text parameter legends: none"
// (core/cat/ft991a/testdata/transcription-b.md §2(a)) — but that is a report
// about the chart, not a column in the delivered file, and this generator can
// only read the file.
//
// Refusing is therefore the honest answer AND the one that fails loudly: if
// this chart ever turns out to carry a text row, the generator stops rather
// than guessing, and the arbitration is against the PDF. The independent check
// on the whole question is the cross-check, whose dialect side comes from
// transcription A — which does carry a text column.
func widthToken(digits string) (byte, error) {
	s := strings.TrimSpace(digits)
	if len(s) != 1 || !isDigit(s[0]) {
		return 0, fmt.Errorf("digits cell %q is not exactly one ASCII digit", digits)
	}
	n := int(s[0] - '0')
	if n < 1 || n > maxWidth {
		return 0, fmt.Errorf("digits %d is outside 1-%d: the inventory has no token for it, and this three-column schema carries nothing from which a wider or a text field could be described", n, maxWidth)
	}
	return byte('0' + n), nil
}

// parseMenuNumber reads one of B's address cells — "087" — as the whole EX
// address it is, returning both the three ASCII digits the wire carries and
// their numeric value.
//
// It refuses anything that is not exactly three ASCII digits, so a cell of a
// different shape cannot be silently reduced to a plausible address. Two
// mistakes it is aimed at in particular: a FOUR-digit cell (the FT-891's pair
// address, whose transcription B has this file's exact header, and which would
// otherwise parse as a plausible three-digit address with one digit quietly
// discarded), and a two-digit one (a lost leading zero).
func parseMenuNumber(cell string) (addr string, num int, err error) {
	s := strings.TrimSpace(cell)
	if len(s) != menuNumberDigits {
		return "", 0, fmt.Errorf("%q is not exactly three ASCII digits", cell)
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return "", 0, fmt.Errorf("%q is not exactly three ASCII digits", cell)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil { // unreachable: every byte is a digit
		return "", 0, fmt.Errorf("%q is not exactly three ASCII digits", cell)
	}
	return s, n, nil
}

// checkRun enforces the one structural property this flat chart has, in place
// of the FT-891 generator's group rules: the menu numbers are ONE CONSECUTIVE
// RUN, opening at 001 and increasing by exactly one to the last row, with no
// gap and no repeat.
//
// That is what B's own reconciliation records — "both passes read 001 … 153,
// strictly increasing by one, no gaps and no repeats"
// (core/cat/ft991a/testdata/transcription-b.md §3) — and it is worth enforcing
// rather than trusting, because it is what makes the rendered file's
// address-to-CSV-line mapping true: a row's line is its address plus one,
// EXCLUDED ROWS INCLUDED, so the generated file can state the mapping once
// instead of repeating a line number on each of 152 entries.
//
// It is a refusal rather than a repair. A gap in a transcription of a ruled
// chart is either a transcription defect or a chart this projection cannot
// model, and both are findings.
func checkRun(rows []row) error {
	if len(rows) == 0 {
		return fmt.Errorf("no rows")
	}
	if rows[0].num != 1 {
		return fmt.Errorf("line %d: the chart opens at %s, want 001", rows[0].line, rows[0].addr)
	}
	for i := 1; i < len(rows); i++ {
		if want := rows[i-1].num + 1; rows[i].num != want {
			return fmt.Errorf("line %d: menu number %s follows %s — the chart's numbers must run consecutively, and %03d is missing or repeated", rows[i].line, rows[i].addr, rows[i-1].addr, want)
		}
	}
	return nil
}

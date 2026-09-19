// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// Domain is one FT-710 EX address's write value domain — the P4 legend's
// enumerated codes and/or stepped numeric range. It is the SAME FOUR FIELDS
// as internal/extable.Domain (task b1's ParseP4Domain), hand-written here
// rather than imported: core/cat must not import internal/extable, which is
// build-time tooling that RENDERS core/cat source text, not a runtime
// dependency of it (see EXWriteItem's own doc comment, and
// core/transport/ex_crosscheck_test.go / internal/fakeradio/ex.go:161-168,
// the existing seam this milestone keeps both packages clear of).
//
// Codes holds enumerated sentinel values (e.g. 0 for "OFF"); Lo/Hi/Step
// describe a stepped numeric range (Step defaults to 1; Hi == 0 means no
// range); Signed marks a range whose wire form carries an explicit sign
// character — -00 and +00 are two distinct legal codes, not one zero
// (milestone spec §2's 26 sign-bearing addresses). Both arms may be
// populated at once: a hybrid legend mixing a code with a range
// (table2.csv:201, PRMTRC EQ1 FREQ).
type Domain struct {
	Codes        []int
	Lo, Hi, Step int
	Signed       bool
}

// Contains reports whether v is a legal value of d: an enumerated code, or
// inside the stepped Lo..Hi range on a Step boundary. The zero Domain (an
// unparsed P4 legend, or an address Session W has not characterised)
// contains nothing — Contains then refuses every value rather than admitting
// one, the safe direction for a write gate.
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

// EXWriteItem is one FT-710 write descriptor: a Table 2 address's value
// Domain plus the Set width the radio actually answers with. It is
// generated into exwrite_gen.go by internal/extable.RenderWriteGo (task
// b2), which emits a literal Go composite for this type by field name —
// exactly the textual-emission trick RenderGo already uses for EXItem, for
// a type it never imports.
//
// One item is generated per table2.csv row, all 296, admitted or not — the
// milestone's denylist (core/cat/exdenylist.go) plays no part in this
// table's generation; task (c) filters it into the dialect's actual write
// gate.
//
// ObservedSetWidth is the rendered sentinel 0 until Session W's hardware
// Set-width characterisation (table2-write-observed.csv) fills it in, on
// exinventory.go's own ObservedReadWidth precedent: the sentinel is
// RENDERED, not absent, so every consumer checks it explicitly rather than
// trusting map presence. Read width must never size a Set — see
// ObservedReadWidth's own warning — so this field, not Digits or
// ObservedReadWidth, is what a Set frame is built against.
type EXWriteItem struct {
	// Addr is the (P1,P2,P3) EX address.
	Addr EXAddress
	// Domain is this address's write value domain, parsed from the manual's
	// P4 legend. The zero Domain means the legend did not parse — no Codes,
	// no range — and Contains refuses every value for it, which holds the
	// address read-only regardless of ObservedSetWidth.
	Domain Domain
	// ObservedSetWidth is the P4 wire width Session W's hardware Set-write
	// characterisation observed for this address. Zero means "not yet
	// characterised" — the rendered sentinel, never an omission.
	ObservedSetWidth int
	// ObservedSetShape classifies that answer: "numeric", "signed" or
	// "text", on ObservedReadShape's own three-class precedent. Empty means
	// no observation.
	ObservedSetShape string
}

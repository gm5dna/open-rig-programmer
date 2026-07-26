// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// slotSpace describes one radio family's memory slot numbering: which
// 3-byte wire forms exist and what each means. DATA, not code, so a
// second dialect is a different table rather than a different function.
type slotSpace struct {
	memoryLo, memoryHi int    // inclusive decimal range, e.g. 1..99
	sixtyLo, sixtyHi   int    // inclusive decimal range, e.g. 501..599; 0,0 if absent
	pmsPairs           int    // e.g. 9 -> P1L..P9U; valid range 0..9 — the pair number is a single wire digit ('1'-'9'), so this can never validly exceed 9; consumers must go through pmsCap() rather than trust this raw; 0 if absent
	emgWire            string // "" if this family has no emergency channel
	noneWire           string // the "VFO or MT or QMB" form, e.g. "000"
}

// Dialect is one radio family's CAT variation: everything this codec
// needs that differs between models sharing the classic NEWCAT grammar.
//
// It carries DATA, not frame shapes. A deliberate M9b scope decision
// recorded in the design document: the FTdx10/101 manuals document a
// combined ~50-byte MT record frame against the FT-710's short form, but
// that difference is unverified against hardware, and the FT-710's own MT
// is the precedent for a manual being wrong about exactly this.
// Per-command frame-shape variants are M9c's.
//
// THE RECEIVER IS LOAD-BEARING. Every method here, and every helper those
// methods delegate to, must read this struct rather than a package-level
// global. A method that takes a Dialect and consults a global has the
// shape of a seam and none of the substance, and while only one dialect
// exists no ordinary test catches it — see seconddialect_test.go (Task
// 57), which is the test that does.
//
// The ZERO VALUE IS INERT, deliberately. An exported struct always has a
// constructible zero value, so `var d cat.Dialect` compiles and
// d.AllowedCommand (from Task 54) is a non-nil method value that would
// satisfy transport.NewEngine's nil check. A zero Dialect therefore carries no
// slot space, no modes and no inventory, and consequently builds nothing
// and accepts nothing.
type Dialect struct {
	catID     string
	modeNames map[Mode]string
	slots     slotSpace

	exItems    []EXItem
	exMembers  map[EXAddress]bool   // this dialect's OWN membership index
	exByTriple map[[3]int]EXAddress // this dialect's OWN decimal-triple index
}

// FT710 is the Yaesu FT-710 dialect: the only configured one that exists.
var FT710 = Dialect{
	catID:     "0800",
	modeNames: modeNames,
	slots: slotSpace{
		memoryLo: 1, memoryHi: 99,
		sixtyLo: 501, sixtyHi: 599,
		pmsPairs: 9,
		emgWire:  "EMG",
		noneWire: "000",
	},
	exItems:    exItemsGen,
	exMembers:  buildEXMembers(exItemsGen),
	exByTriple: buildEXByTriple(exItemsGen),
}

// buildEXMembers indexes items for membership tests.
func buildEXMembers(items []EXItem) map[EXAddress]bool {
	m := make(map[EXAddress]bool, len(items))
	for _, it := range items {
		m[it.Addr] = true
	}
	return m
}

// buildEXByTriple indexes items by their decimal (P1,P2,P3) triple, so
// Dialect.NewEXAddress can validate a caller-supplied triple by lookup
// alone — no numeric range logic on the components, exactly as the
// package-level NewEXAddress has always done, but against THIS dialect's
// own inventory rather than a package global.
func buildEXByTriple(items []EXItem) map[[3]int]EXAddress {
	m := make(map[[3]int]EXAddress, len(items))
	for _, it := range items {
		m[[3]int{int(it.Addr.P1), int(it.Addr.P2), int(it.Addr.P3)}] = it.Addr
	}
	return m
}

// Configured reports whether this Dialect carries data. False for the
// zero value; see the type's doc comment.
func (d Dialect) Configured() bool { return d.catID != "" && d.modeNames != nil }

// CATID is the four-character identity this radio answers "ID;" with.
func (d Dialect) CATID() string { return d.catID }

// ValidMode reports whether m is a mode nibble this dialect knows.
func (d Dialect) ValidMode(m Mode) bool {
	_, ok := d.modeNames[m]
	return ok
}

// ModeName returns the display name for m under this dialect, or a
// diagnostic placeholder for a Mode it does not know.
func (d Dialect) ModeName(m Mode) string {
	if name, ok := d.modeNames[m]; ok {
		return name
	}
	return unknownModeName(m)
}

// EXItems returns a fresh copy of this dialect's EX inventory: callers
// have always been free to mutate what they get back, and one caller's
// mutation must never become everyone's inventory.
func (d Dialect) EXItems() []EXItem {
	out := make([]EXItem, len(d.exItems))
	copy(out, d.exItems)
	return out
}

// EXAddresses returns this dialect's EX addresses in inventory order.
func (d Dialect) EXAddresses() []EXAddress {
	out := make([]EXAddress, len(d.exItems))
	for i, it := range d.exItems {
		out[i] = it.Addr
	}
	return out
}

// KnownEXAddress reports whether a is in THIS dialect's inventory.
// MEMBERSHIP, never a numeric range: Table 2 is sparse and its P1 groups
// are not contiguous.
func (d Dialect) KnownEXAddress(a EXAddress) bool { return d.exMembers[a] }

// pmsCap returns this dialect's PMS pair count, clamped to 9. The wire
// form's pair digit is a single ASCII byte ('1'-'9'), so pmsPairs can
// never validly exceed 9 no matter what a dialect's data configures —
// codex review Important-2 measured an uncapped pmsPairs building
// multi-byte wire forms ("P12L") that the SAME dialect's own ParseSlot
// then rejected. classifySlot and PMSSlot both consume this rather than
// the raw field, so the cap is expressed exactly once.
func (d Dialect) pmsCap() int {
	if d.slots.pmsPairs > 9 {
		return 9
	}
	return d.slots.pmsPairs
}

// classifySlot reports what kind of slot, if any, wire represents under
// this dialect's slot space. Every slot-taking method routes through it.
func (d Dialect) classifySlot(wire string) slotKind {
	if len(wire) != 3 {
		return slotKindInvalid
	}
	if d.slots.noneWire != "" && wire == d.slots.noneWire {
		return slotKindNone
	}
	if d.slots.emgWire != "" && wire == d.slots.emgWire {
		return slotKindEMG
	}

	allDigits := true
	for i := 0; i < len(wire); i++ {
		if wire[i] < '0' || wire[i] > '9' {
			allDigits = false
			break
		}
	}
	if allDigits {
		n := int(wire[0]-'0')*100 + int(wire[1]-'0')*10 + int(wire[2]-'0')
		switch {
		case d.slots.memoryHi > 0 && n >= d.slots.memoryLo && n <= d.slots.memoryHi:
			return slotKindMemory
		case d.slots.sixtyHi > 0 && n >= d.slots.sixtyLo && n <= d.slots.sixtyHi:
			// ASSUMED: the reference marks 5xx numbering as unverified.
			return slotKind60m
		default:
			return slotKindInvalid
		}
	}

	if pc := d.pmsCap(); pc > 0 &&
		wire[0] == 'P' &&
		wire[1] >= '1' && wire[1] <= byte('0'+pc) &&
		(wire[2] == 'L' || wire[2] == 'U') {
		return slotKindPMS
	}

	return slotKindInvalid
}

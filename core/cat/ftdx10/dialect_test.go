// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10_test

import (
	"fmt"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
)

// This file is M9c-4 task 6's evidence: the FTdx10 dialect held to core/cat's
// conformance suite, then pinned against cat.FT710 in both directions — what
// the two radios SHARE (the mode table and the slot space, which this package
// retypes rather than references) and what they must NOT (the CAT ID, the MT
// frame form, the MW write kind and the EX inventory).
//
// The identity pins exist because this package's mode table and slot space
// are FRESH TRANSCRIPTIONS. core/cat's are unexported, so there is nothing to
// share even if sharing were right; retyping buys independence and costs a
// copy error, and these tests are where a copy error is caught. They are
// deliberately BEHAVIOURAL — everything is asked through the exported API,
// over the whole wire space, rather than by comparing two tables — because a
// table comparison would prove the tables match and say nothing about what
// either dialect DOES with them.
//
// TWO ASSUMED MEMBERS ARE EMBEDDED IN WHAT THE PINS COMPARE, and are noted
// at each site: cat.ModeUnset ('0', "-") is in no FTdx10 mode legend, and the
// none wire "000" is in no FTdx10 slot legend. So "identical to the FT-710"
// here means identical INCLUDING two members inherited from it. doc.go's
// ASSUMED register carries the full statement and the Stage R capture that
// lifts each.

// TestConformance runs core/cat's whole exported-API conformance suite over
// the real FTdx10 dialect: every builder's frames are well-formed and
// admitted by this dialect's own gate, both wrong-form APIs refuse and are
// seen to refuse, the clarifier's endpoints build and one step past is
// refused, and the walk fails if it built nothing.
//
// dialecttest.RunZeroValue is NOT called here. It is universal — a property
// of the zero cat.Dialect, not of any model — and core/cat already runs it.
func TestConformance(t *testing.T) {
	dialecttest.Run(t, ftdx10.Dialect())
}

// TestModeIdentityWithFT710 sweeps ALL 256 wire bytes through both dialects
// and requires the same verdict and the same display name from each.
//
// The whole byte space, not the sixteen this package declares: a table with a
// SPURIOUS member — a lower-case 'a', a stray 'G' — is invisible to a walk
// over its own keys, and that is precisely the copy error a fresh
// transcription risks. The sweep also fixes the count at 16, so a table that
// silently lost a mode fails here rather than passing a comparison of two
// short tables.
func TestModeIdentityWithFT710(t *testing.T) {
	d := ftdx10.Dialect()

	valid := 0
	for c := 0; c < 256; c++ {
		m := cat.Mode(byte(c))
		gotValid, wantValid := d.ValidMode(m), cat.FT710.ValidMode(m)
		if gotValid != wantValid {
			t.Errorf("ValidMode(%#02x): FTdx10 %v, FT-710 %v — the two mode tables are transcribed from the same 1-F legend and must accept exactly the same bytes", c, gotValid, wantValid)
			continue
		}
		if !gotValid {
			continue
		}
		valid++
		if got, want := d.ModeName(m), cat.FT710.ModeName(m); got != want {
			t.Errorf("ModeName(%#02x): FTdx10 %q, FT-710 %q", c, got, want)
		}
	}
	if valid != 16 {
		t.Errorf("the FTdx10 dialect accepts %d mode bytes, want 16 — the manual's legend runs 1-F (fifteen) plus the ASSUMED '0' placeholder", valid)
	}

	// ModeByName is the WRITE direction: it is how a stored channel's mode
	// string becomes a wire byte again. A name that resolves to a different
	// nibble than the one it was rendered from would write the wrong mode
	// into a memory, which no read-side comparison above would notice.
	for c := 0; c < 256; c++ {
		m := cat.Mode(byte(c))
		if !d.ValidMode(m) {
			continue
		}
		name := d.ModeName(m)
		got, ok := d.ModeByName(name)
		if !ok {
			t.Errorf("ModeByName(%q) did not resolve, but ModeName(%#02x) produced it — the two are inverses", name, c)
			continue
		}
		if got != m {
			t.Errorf("ModeByName(%q) = %#02x, want %#02x — a name resolving to another dialect member would write the wrong mode nibble", name, byte(got), c)
		}
	}

	// THE ASSUMED MEMBER, stated rather than left inside the sweep above.
	// cat.ModeUnset appears in no FTdx10 mode legend (MD at layout 1146-1149,
	// MR's P6 at 1192-1194, MT's at 1227-1229, MW's at 1267-1269 all run
	// 1-F). It is here because parsers must accept the placeholder; core/cat
	// refuses to emit it in any Set frame.
	if !d.ValidMode(cat.ModeUnset) {
		t.Error("ValidMode(cat.ModeUnset) is false — the ASSUMED placeholder member is missing, and an FTdx10 answering '0' would be unreadable")
	}
	if got := d.ModeName(cat.ModeUnset); got != "-" {
		t.Errorf("ModeName(cat.ModeUnset) = %q, want %q", got, "-")
	}
}

// threeDigits renders n as the three-digit wire form every slot-bearing
// command carries. Local to this file rather than shared: the conformance
// suite has its own, and a helper reaching across package boundaries to be
// reused is how two walks end up sweeping the same space by accident.
func threeDigits(n int) string {
	return fmt.Sprintf("%03d", n)
}

// TestSlotIdentityWithFT710 compares the two dialects' slot spaces
// BEHAVIOURALLY, over every wire form either can produce: all thousand
// three-digit codes, all eighteen PMS forms, and "EMG".
//
// Behaviour, not tables, and specifically NOT Slot.IsMemory/IsPMS/Is60m/
// IsEMG/IsNone. Those predicates classify through the FT-710's dialect
// whatever Slot they are given — an M9b deferral, ledgered and documented on
// classifySlotWire in core/cat/slot.go — so on an FTdx10 slot they would
// answer for the wrong radio and this test would be pinning that mistake
// rather than the slot space. Everything below goes through a DIALECT
// receiver.
//
// THE NONE WIRE "000" IS ONE OF THE FORMS COMPARED, and it is ASSUMED: no
// FTdx10 slot legend names it (MC's at layout 1131-1133 and MR's at 1184-1185
// give 001-099, P1L-P9U, 5xx and EMG; MW's at 1259 gives only 001-099 and
// P1L-P9U). It is the FT-710's MR-answer fact, and a none wire is
// structurally required by cat.SlotSpace, so one is supplied.
func TestSlotIdentityWithFT710(t *testing.T) {
	d := ftdx10.Dialect()

	wires := make([]string, 0, 1000+18+1)
	for n := 0; n <= 999; n++ {
		wires = append(wires, threeDigits(n))
	}
	for pair := 1; pair <= 9; pair++ {
		wires = append(wires, fmt.Sprintf("P%dL", pair), fmt.Sprintf("P%dU", pair))
	}
	wires = append(wires, "EMG")

	accepted := 0
	for _, w := range wires {
		got, gotErr := d.ParseSlot(w)
		want, wantErr := cat.FT710.ParseSlot(w)
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("ParseSlot(%q): FTdx10 err=%v, FT-710 err=%v — the two slot spaces are declared identically and must accept the same wire forms", w, gotErr, wantErr)
			continue
		}
		if gotErr != nil {
			continue
		}
		accepted++
		if got.Wire() != want.Wire() {
			t.Errorf("ParseSlot(%q): FTdx10 canonicalised to %q, FT-710 to %q", w, got.Wire(), want.Wire())
		}
	}
	// 99 memory + 99 sixty-metre + "000" + 18 PMS + "EMG".
	if want := 99 + 99 + 1 + 18 + 1; accepted != want {
		t.Errorf("the FTdx10 dialect classifies %d of the %d wire forms swept, want %d — a sweep this test agrees on but that accepts nothing would pass every comparison above in silence", accepted, len(wires), want)
	}

	// THE CONSTRUCTORS, over their boundaries in both directions. ParseSlot
	// above proves the two dialects read the same wire space; these prove
	// they WRITE it the same way, which is a different function on a
	// different bound — SixtyMSlot in particular takes an ordinal and derives
	// the wire form from its own sixtyLo, so a dialect with the same range
	// and a different base would agree above and disagree here.
	for n := -1; n <= 101; n++ {
		gotSlot, gotErr := d.MemorySlot(n)
		wantSlot, wantErr := cat.FT710.MemorySlot(n)
		compareConstructor(t, fmt.Sprintf("MemorySlot(%d)", n), gotSlot, gotErr, wantSlot, wantErr)

		gotSlot, gotErr = d.SixtyMSlot(n)
		wantSlot, wantErr = cat.FT710.SixtyMSlot(n)
		compareConstructor(t, fmt.Sprintf("SixtyMSlot(%d)", n), gotSlot, gotErr, wantSlot, wantErr)
	}
	for pair := -1; pair <= 10; pair++ {
		for _, upper := range []bool{false, true} {
			gotSlot, gotErr := d.PMSSlot(pair, upper)
			wantSlot, wantErr := cat.FT710.PMSSlot(pair, upper)
			compareConstructor(t, fmt.Sprintf("PMSSlot(%d, %v)", pair, upper), gotSlot, gotErr, wantSlot, wantErr)
		}
	}
	if got, want := d.EMGSlot().Wire(), cat.FT710.EMGSlot().Wire(); got != want {
		t.Errorf("EMGSlot(): FTdx10 %q, FT-710 %q", got, want)
	}
	if got := d.EMGSlot().Wire(); got != "EMG" {
		t.Errorf("EMGSlot() = %q, want %q — the manual's MC and MR legends both name EMG (EMERGENCY CH)", got, "EMG")
	}
}

// compareConstructor holds one constructor call's outcome on the two
// dialects to each other: the same acceptance verdict, and on acceptance the
// same wire form.
func compareConstructor(t *testing.T, what string, got cat.Slot, gotErr error, want cat.Slot, wantErr error) {
	t.Helper()

	if (gotErr == nil) != (wantErr == nil) {
		t.Errorf("%s: FTdx10 err=%v, FT-710 err=%v", what, gotErr, wantErr)
		return
	}
	if gotErr != nil {
		return
	}
	if got.Wire() != want.Wire() {
		t.Errorf("%s: FTdx10 built %q, FT-710 built %q", what, got.Wire(), want.Wire())
	}
}

// TestDifferencePinCATID is the identity that makes this a different radio at
// all: the FTdx10 answers "ID;" with 0761 (manual rev 2308-F, layout 977),
// the FT-710 with 0800.
func TestDifferencePinCATID(t *testing.T) {
	if got := ftdx10.Dialect().CATID(); got != "0761" {
		t.Errorf("CATID() = %q, want %q", got, "0761")
	}
	if got, other := ftdx10.Dialect().CATID(), cat.FT710.CATID(); got == other {
		t.Errorf("CATID() = %q on BOTH dialects — a shared identity would make radio detection pick whichever driver was registered first", got)
	}
}

// TestDifferencePinMWWriteKind pins the FTdx10's MW P7.
//
// THE CAVEAT IS THE POINT. This manual's MW legend reads "P7 0: (Fixed)"
// (layout 1270), and cat.CombinedMTSetKind is the byte '0' — so the constant
// on the right is the correct SPELLING of what this radio documents. That the
// two coincide is A FACT OF THIS RADIO, not a rule: MW's P7 and the combined
// MT Set's P7 are different fields of different commands that this manual
// happens to fix at the same byte. A dialect whose MW kind differed from the
// combined form's Set constant is entirely expressible, and core/cat keeps
// them apart on purpose — validateCombinedMTFields uses the FORM's constant
// and never this dialect's mwWriteKind. Nothing here may be read as
// permission to derive one from the other.
//
// The FT-710 is the counter-example, and it is asserted rather than described:
// its MW kind is cat.KindMemory ('1'), HW-confirmed.
func TestDifferencePinMWWriteKind(t *testing.T) {
	if got := ftdx10.Dialect().MWWriteKind(); got != cat.CombinedMTSetKind {
		t.Errorf("MWWriteKind() = %q, want %q (cat.CombinedMTSetKind) — this manual's MW legend reads \"P7 0: (Fixed)\"", got, cat.CombinedMTSetKind)
	}
	if got := cat.FT710.MWWriteKind(); got == ftdx10.Dialect().MWWriteKind() {
		t.Errorf("the FT-710's MWWriteKind() is %q too — the two radios document different MW P7 bytes ('1' Memory against '0' Fixed), and a shared value here would mean one of the two transcriptions took the other's", got)
	}
}

// TestDifferencePinMTForm pins the frame shape. The FTdx10's MT carries the
// whole 28-position memory field block plus P11 and a 12-character tag
// (layout 1217-1251); the FT-710's carries a slot, a display flag and a tag.
// They are different commands wearing the same two letters, which is why
// MTForm is data and each form's API refuses a dialect declaring the other.
func TestDifferencePinMTForm(t *testing.T) {
	if got := ftdx10.Dialect().MTForm(); got != cat.MTFormCombined {
		t.Errorf("MTForm() = %v, want %v", got, cat.MTFormCombined)
	}
	if got := cat.FT710.MTForm(); got != cat.MTFormShort {
		t.Errorf("the FT-710's MTForm() = %v, want %v — this pin is a DIFFERENCE, and it proves nothing if both dialects declare the same form", got, cat.MTFormShort)
	}
}

// TestDifferencePinMTAnswerBounds pins the combined answer's EXACT length.
//
// 41 is not written anywhere in core/cat and must not be: the geometry is
// derived, 29 + TagMaxBytes, so a family with a different tag field gets a
// different window from the same code. This test states the arithmetic's
// ANSWER for this radio, which the manual's own Set/Answer chart runs to
// (layout 1230-1251: the 28 shared positions, P11 at 28, a 12-byte P12 tag at
// 29-40, ';' at 41).
func TestDifferencePinMTAnswerBounds(t *testing.T) {
	min, max, err := ftdx10.Dialect().MTAnswerBounds()
	if err != nil {
		t.Fatalf("MTAnswerBounds() = %v, want no error from a configured dialect", err)
	}
	if min != 41 || max != 41 {
		t.Errorf("MTAnswerBounds() = (%d, %d), want (41, 41) — the combined record's length is exact, so its bounds are equal", min, max)
	}

	// The FT-710's window is a range, not a point. Asserting only its
	// inequality keeps this pin about the DIFFERENCE without restating that
	// radio's own numbers, which its own tests own.
	fmin, fmax, ferr := cat.FT710.MTAnswerBounds()
	if ferr != nil {
		t.Fatalf("the FT-710's MTAnswerBounds() = %v", ferr)
	}
	if fmin == fmax {
		t.Errorf("the FT-710's MTAnswerBounds() = (%d, %d) — equal bounds are the COMBINED form's signature, so this difference pin has lost its counter-example", fmin, fmax)
	}
}

// TestDifferencePinEXDisjointness proves the two inventories are DIFFERENT
// TABLES, in both directions, against addresses each chart really carries.
//
// Both directions, because a one-way check passes on a subset: if this
// package had somehow been generated from the FT-710's CSV, "the FTdx10 knows
// something the FT-710 does not" would fail, but "the FT-710 knows something
// the FTdx10 does not" is what catches the reverse mistake — an FTdx10
// inventory that had quietly acquired the FT-710's EXTENSION group.
//
// The addresses are chart-true, and revision 2 of the spec exists because
// revision 1's were not: its "P1=05 exists here" pin laundered this manual's
// anomalous EX grammar header ("P1: 01 - 05") into a fact, when the chart
// populates P1 01-04 only and there is no EXTENSION group anywhere in this
// manual. See doc.go's header-vs-chart anomaly section.
//
//   - (01,06,01) PSK MODE and (01,07,01) RX USOS sit in this chart's ENC/DEC
//     PSK and ENC/DEC RTTY subgroups. The FT-710's chart has no P2 above 05
//     under P1=01, so neither address is a member there.
//   - (06,01,01) PRESET NAME is the FT-710's EXTENSION SETTING / PRESET1
//     group at P1=06. This chart has no P1=06 at all.
func TestDifferencePinEXDisjointness(t *testing.T) {
	d := ftdx10.Dialect()

	for _, a := range []cat.EXAddress{
		{P1: 1, P2: 6, P3: 1},
		{P1: 1, P2: 7, P3: 1},
	} {
		if !d.KnownEXAddress(a) {
			t.Errorf("KnownEXAddress(%v) is false on the FTdx10 — its chart carries this address (ENC/DEC PSK and ENC/DEC RTTY are P2 06 and 07 under P1=01)", a)
		}
		if cat.FT710.KnownEXAddress(a) {
			t.Errorf("KnownEXAddress(%v) is TRUE on the FT-710 — that chart has no P2 above 05 under P1=01, so this pin's disjointness has been lost in that direction", a)
		}
	}

	ext := cat.EXAddress{P1: 6, P2: 1, P3: 1}
	if d.KnownEXAddress(ext) {
		t.Errorf("KnownEXAddress(%v) is TRUE on the FTdx10 — there is no EXTENSION group and no P1=06 anywhere in manual rev 2308-F, so this inventory has taken on the FT-710's", ext)
	}
	if !cat.FT710.KnownEXAddress(ext) {
		t.Errorf("KnownEXAddress(%v) is false on the FT-710 — that is this pin's counter-example, and without it the assertion above proves only that the address is unknown to everybody", ext)
	}
}

// TestEXAnswerBound proves the EX answer's upper length bound is THIS
// DIALECT'S OWN, derived from its own inventory.
//
// The maximum is recomputed here from Dialect().EXItems() rather than taken
// from a constant, from the profile, or from core/cat: the bound and the
// datum it is derived from must not come from the same place twice, which is
// the "bound consulted from one place with its datum taken from another"
// failure M9b hit repeatedly. It reaches 12 through exactly one item, MY CALL.
// at (04,01,01) — the chart's only text row, pinned as the only one by
// crosscheck_test.go.
//
// The behavioural half is what matters: the derived width must be what the
// parser actually enforces, one byte either side.
func TestEXAnswerBound(t *testing.T) {
	d := ftdx10.Dialect()

	items := d.EXItems()
	if len(items) == 0 {
		t.Fatal("EXItems() is empty — every assertion below would be vacuous")
	}
	maxDigits := 0
	for _, it := range items {
		if it.Digits > maxDigits {
			maxDigits = it.Digits
		}
	}
	if maxDigits != 12 {
		t.Fatalf("max(Digits) over the FTdx10's %d inventory items is %d, want 12 (MY CALL. at (04,01,01), the chart's one text row)", len(items), maxDigits)
	}

	// A known address, and the widest legal P4 body for this dialect. The
	// body is returned VERBATIM — no per-item width policy is applied at
	// parse, by core/cat's documented decision — so a 12-byte answer is
	// admissible at any member address, not only at the text row's.
	addr := cat.EXAddress{P1: 4, P2: 1, P3: 1}
	if !d.KnownEXAddress(addr) {
		t.Fatalf("KnownEXAddress(%v) is false — this test needs a member address to answer at", addr)
	}

	body := "GM5DNA......" // 12 bytes
	if len(body) != maxDigits {
		t.Fatalf("the test's P4 body is %d bytes, want %d", len(body), maxDigits)
	}
	frame := []byte("EX" + addr.Wire() + body + ";")
	gotAddr, gotBody, err := d.ParseEXAnswer(frame)
	if err != nil {
		t.Errorf("ParseEXAnswer(%q) = %v — a %d-byte P4 is this dialect's own widest item, so its parser must read one", frame, err, maxDigits)
	} else {
		if gotAddr != addr {
			t.Errorf("ParseEXAnswer(%q) returned address %v, want %v", frame, gotAddr, addr)
		}
		if gotBody != body {
			t.Errorf("ParseEXAnswer(%q) returned P4 %q, want %q verbatim", frame, gotBody, body)
		}
	}

	over := []byte("EX" + addr.Wire() + body + "X" + ";")
	if _, _, err := d.ParseEXAnswer(over); err == nil {
		t.Errorf("ParseEXAnswer(%q) ACCEPTED a %d-byte P4, one past this dialect's widest inventory item — the bound is not deriving from this inventory", over, maxDigits+1)
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts590 "github.com/gm5dna/open-rig-programmer/core/kw/ts590"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// inventoryFor is the row's own generated menu inventory, named here once so
// that every test below asks the SAME question the descriptor builder asks —
// and asks it of the accessor rather than of a literal count.
func inventoryFor(t *testing.T, row Row) []kw.EXItem {
	t.Helper()
	switch row {
	case RowS:
		return kwts590.EXItemsS()
	case RowSG:
		return kwts590.EXItemsSG()
	}
	t.Fatalf("no inventory for %v", row)
	return nil
}

// exAnswer builds the EX answer frame for address id carrying P5 raw:
// "EX" P1P1P1 P2P2 P3 P4 P5 ";" — ten fixed bytes with P5 inserted before
// the terminator, P2 "00" and P3/P4 '0' (590:546-560).
//
// HAND-BUILT AND NOT core/kw's, deliberately: an answer produced by the codec
// under test would pin the parser against a builder that does not exist (this
// codec builds no EX Set and no EX Answer at all, ex.go), and these tests need
// answers the codec would refuse — a wrong address, a P5 wider than the row's
// printed width.
func exAnswer(id, raw string) string { return "EX" + id + "0000" + raw + ";" }

// TestSettingsDescriptor_IsBuiltFromThisRowsOwnInventory is P8's central
// claim and the one a wrong import would break silently: `ts590s-ex@1` is
// built from EXItemsS() and `ts590sg-ex@1` from EXItemsSG(), NEVER from the
// other's.
//
// THE TWO TABLES ARE NOT ONE TABLE WITH TWELVE MORE ROWS. The book prints two
// separate parameter lists — "EX Command Parameter List (for TS-590S)" at
// 590:564 and the SG's at 590:744 — over COLLIDING addresses with different
// meanings: menu 000 is Display brightness on the S (590:569) and read-only
// Firmware Version on the SG (590:749), and every address above it differs
// too. So the count (88 against 100) is the weakest half of this test and the
// LABELS are the strong half: a descriptor built from the wrong inventory
// would publish the wrong setting name at every address, and on the S would
// additionally publish 088-099, twelve addresses outside that row's own
// printed menu domain (590:543, A26).
func TestSettingsDescriptor_IsBuiltFromThisRowsOwnInventory(t *testing.T) {
	for _, row := range bothRows {
		items := inventoryFor(t, row)
		d := SettingsDescriptor(row)

		var got []driver.SettingItem
		for _, m := range d.Menus {
			for _, g := range m.Groups {
				got = append(got, g.Items...)
			}
		}
		if len(got) != len(items) {
			t.Fatalf("%s: descriptor holds %d items, its own inventory %d", modelNameFor(row), len(got), len(items))
		}
		for i, it := range items {
			if got[i].ID != it.Addr.Wire() || got[i].Label != it.Name {
				t.Fatalf("%s: item %d = %q/%q, want %q/%q — in the inventory's own order", modelNameFor(row), i, got[i].ID, got[i].Label, it.Addr.Wire(), it.Name)
			}
		}
	}

	// The colliding address, stated outright: the two rows disagree about
	// what menu 000 IS, which is why there are two inventories at all.
	s, sg := SettingsDescriptor(RowS), SettingsDescriptor(RowSG)
	if a, b := s.Menus[0].Groups[0].Items[0], sg.Menus[0].Groups[0].Items[0]; a.ID != "000" || b.ID != "000" || a.Label == b.Label {
		t.Errorf("menu 000 is %q on the S and %q on the SG; both rows address it and the two must not agree about what it is (590:569 against 590:749)", a.Label, b.Label)
	}
	// And the S publishes nothing above its own printed domain.
	for _, it := range s.Menus[0].Groups[0].Items {
		if it.ID > "087" {
			t.Errorf("the TS-590S descriptor publishes menu %q, outside its printed domain 000 ~ 087 (590:543)", it.ID)
		}
	}
}

// TestSettingsDescriptor_IsOneFlatMenuAndOneGroup is the FLAT shape RULED on
// 05/09/2026 (Stuart decisions row 4; plan P8).
//
// Kenwood's parameter lists print a menu number, a function name and a
// parameter legend, and NO GROUP HIERARCHY AT ALL — which is why all three
// Kenwood profiles register LabelsAbsent and every EXItem's P1Label and
// P2Label is "" (core/kw's EXItem says so in terms). driver.SettingsDescriptor
// is a two-level tree whose Validate requires at least one menu and at least
// one group, so the single menu and the single group here are STRUCTURAL and
// carry no claim about the radio. Inventing a tens-decade tree to look like
// the FT-710's was considered and REFUSED: it would put structure in a book
// that prints none.
func TestSettingsDescriptor_IsOneFlatMenuAndOneGroup(t *testing.T) {
	for _, row := range bothRows {
		d := SettingsDescriptor(row)
		if len(d.Menus) != 1 {
			t.Fatalf("%s: %d menus, want exactly one — this book prints no group structure", modelNameFor(row), len(d.Menus))
		}
		if len(d.Menus[0].Groups) != 1 {
			t.Fatalf("%s: %d groups, want exactly one", modelNameFor(row), len(d.Menus[0].Groups))
		}
		if len(d.Menus[0].Groups[0].Items) != len(inventoryFor(t, row)) {
			t.Errorf("%s: the single group does not hold the whole inventory", modelNameFor(row))
		}
	}
}

// TestSettingsDescriptor_ValidatesAndIDsAreUniqueAtAllThreeLevels is the plan's
// own checklist item, and driver.SettingsDescriptor.Validate is the mechanism:
// it refuses an empty ID or Label at any level, duplicate menu IDs, duplicate
// group IDs, and item IDs that are not unique GLOBALLY across the descriptor.
//
// The item-level half is restated here rather than left to Validate alone,
// because it is the one this radio could plausibly break: the IDs are menu
// addresses out of a GENERATED file, and a transcription that repeated an
// address would produce a descriptor where two settings answered to one ID.
func TestSettingsDescriptor_ValidatesAndIDsAreUniqueAtAllThreeLevels(t *testing.T) {
	for _, row := range bothRows {
		d := SettingsDescriptor(row)
		if err := d.Validate(); err != nil {
			t.Fatalf("%s: SettingsDescriptor.Validate: %v", modelNameFor(row), err)
		}
		seen := map[string]bool{}
		for _, m := range d.Menus {
			for _, g := range m.Groups {
				for _, it := range g.Items {
					if seen[it.ID] {
						t.Errorf("%s: duplicate item ID %q", modelNameFor(row), it.ID)
					}
					seen[it.ID] = true
				}
			}
		}
	}
}

// TestSettingsDescriptor_ItemIDsAreThreeASCIIDigits makes the T1 dependency
// VISIBLE at the driver, which is the whole reason the plan names it here.
//
// A Kenwood menu number is THREE digits (590:543-544, 480:401) where every
// Yaesu row this project ships is four or six, and codeplug's isSettingIDWidth
// — the only place a setting ID's width is judged — had to be widened a third
// time to admit it (core/codeplug/menus.go). Until it was, every one of these
// IDs would have failed clone.ReadSettings' preflight AFTER the descriptor
// itself validated. This test states the shape; TestCloneReadSettings_Walks-
// TheWholeDescriptor exercises the preflight that consumes it.
//
// The DISPLAY form is asserted beside the ID deliberately. They are equal on
// this radio and they are built by two separate expressions, because their
// equality is a fact about this chart — Kenwood prints the address as one
// three-digit Menu No. — and not a rule (the FT-710 prints "01-01-01" against
// a six-digit ID). The FT-891 states the same coincidence for the same reason.
func TestSettingsDescriptor_ItemIDsAreThreeASCIIDigits(t *testing.T) {
	for _, row := range bothRows {
		for _, it := range SettingsDescriptor(row).Menus[0].Groups[0].Items {
			if len(it.ID) != 3 {
				t.Fatalf("%s: item ID %q is %d characters; a Kenwood menu number is three digits", modelNameFor(row), it.ID, len(it.ID))
			}
			for i := 0; i < len(it.ID); i++ {
				if it.ID[i] < '0' || it.ID[i] > '9' {
					t.Fatalf("%s: item ID %q is not three ASCII digits", modelNameFor(row), it.ID)
				}
			}
			if it.Display != it.ID {
				t.Errorf("%s: item %q Display = %q, want the printed Menu No.", modelNameFor(row), it.ID, it.Display)
			}
		}
	}
}

// TestSettingsDescriptor_VersionIsThisRowsOwn: the two rows mint two version
// strings, and a snapshot taken from one must never validate against the
// other's descriptor. They describe different menu tables over colliding
// addresses, so the strings differ by construction and not by convention.
func TestSettingsDescriptor_VersionIsThisRowsOwn(t *testing.T) {
	if got, want := SettingsDescriptor(RowS).Version, "ts590s-ex@1"; got != want {
		t.Errorf("TS-590S descriptor Version = %q, want %q", got, want)
	}
	if got, want := SettingsDescriptor(RowSG).Version, "ts590sg-ex@1"; got != want {
		t.Errorf("TS-590SG descriptor Version = %q, want %q", got, want)
	}
}

// TestSettingsDescriptor_AnUnsetRowPublishesNothing: the zero Row names no
// radio, so it gets the zero descriptor — which driver.SettingsDescriptor.
// Validate refuses. Fail-closed, exactly as Capabilities() and Open do on the
// same value.
func TestSettingsDescriptor_AnUnsetRowPublishesNothing(t *testing.T) {
	d := SettingsDescriptor(RowUnset)
	if len(d.Menus) != 0 || d.Version != "" {
		t.Fatalf("an unset row published %+v", d)
	}
	if err := d.Validate(); err == nil {
		t.Error("the unset row's descriptor validates; it must not")
	}
	if err := New(RowUnset, Simulated).(*ts590Driver).StaticSettingsDescriptor().Validate(); err == nil {
		t.Error("StaticSettingsDescriptor on an unset row validates; it must not")
	}
}

// TestSettingsDescriptor_IsADefensiveCopy: every getter hands out a Clone, so
// a caller that mutated what it was given cannot change what the next caller
// receives. driver.SettingsDescriptor.Clone's own doc comment records why that
// independence is load-bearing.
func TestSettingsDescriptor_IsADefensiveCopy(t *testing.T) {
	d := SettingsDescriptor(RowSG)
	d.Version = "mutated"
	d.Menus[0].Label = "mutated"
	d.Menus[0].Groups[0].Items[0].Label = "mutated"

	fresh := SettingsDescriptor(RowSG)
	if fresh.Version == "mutated" || fresh.Menus[0].Label == "mutated" || fresh.Menus[0].Groups[0].Items[0].Label == "mutated" {
		t.Error("mutating a returned descriptor reached the package-level tree")
	}
}

// TestSettingsDescriptor_ThreeGettersOneTree: the package-level func, the
// driver's StaticSettingsDescriptor (the optional
// driver.StaticSettingsProvider capability) and the session's own
// SettingsDescriptor return equal trees. This driver's settings surface
// depends only on the static inventory for its row — nothing a live session
// discovers, and no Kenwood row discovers anything at all (matrix §3.4) — so
// the three cannot honestly differ.
func TestSettingsDescriptor_ThreeGettersOneTree(t *testing.T) {
	for _, row := range bothRows {
		sess, _ := openTestSession(t, row, radioImage{})
		d := New(row, Simulated).(*ts590Driver).StaticSettingsDescriptor()
		if !equalDescriptors(SettingsDescriptor(row), d) || !equalDescriptors(d, sess.SettingsDescriptor()) {
			t.Errorf("%s: the package func, StaticSettingsDescriptor and Session.SettingsDescriptor disagree", modelNameFor(row))
		}
	}
}

// equalDescriptors compares two trees by value. reflect.DeepEqual would serve;
// this says which level differs when one does.
func equalDescriptors(a, b driver.SettingsDescriptor) bool {
	if a.Version != b.Version || len(a.Menus) != len(b.Menus) {
		return false
	}
	for i := range a.Menus {
		if a.Menus[i].ID != b.Menus[i].ID || a.Menus[i].Label != b.Menus[i].Label || len(a.Menus[i].Groups) != len(b.Menus[i].Groups) {
			return false
		}
		for j := range a.Menus[i].Groups {
			ag, bg := a.Menus[i].Groups[j], b.Menus[i].Groups[j]
			if ag.ID != bg.ID || ag.Label != bg.Label || len(ag.Items) != len(bg.Items) {
				return false
			}
			for k := range ag.Items {
				if ag.Items[k] != bg.Items[k] {
					return false
				}
			}
		}
	}
	return true
}

// TestSession_ImplementsSettingsReader: the capability is an OPTIONAL one a
// caller type-asserts on the CONCRETE session (core/driver/settings.go), so
// nothing but an assertion proves this session offers it.
func TestSession_ImplementsSettingsReader(t *testing.T) {
	sess, _ := openTestSession(t, RowSG, radioImage{})
	if _, ok := driver.Session(sess).(driver.SettingsReader); !ok {
		t.Fatal("*Session does not satisfy driver.SettingsReader")
	}
}

// TestReadSetting_AKnownAnswerIsReturnedVerbatim walks one address on each
// row, at that row's OWN printed width, and pins that the raw P5 comes back
// untouched.
//
// P5 IS RETURNED VERBATIM, INCLUDING ANY TRAILING SPACE. Neither book states
// a padding rule for it — A1's rule is MR/MW's P16 and is scoped to that
// field — so trimming here would apply an assumption the register does not
// make. The free-text row is the case that matters: "up to 8 ASCII
// characters" (590:741 on the S, 590:750 on the SG), and what a radio puts in
// the unused ones is unprinted.
func TestReadSetting_AKnownAnswerIsReturnedVerbatim(t *testing.T) {
	for _, row := range bothRows {
		items := inventoryFor(t, row)
		// The row's widest item, so the answer exercises a P5 of more than
		// one character, and its own trailing space with it.
		widest := items[0]
		for _, it := range items {
			if it.Digits > widest.Digits {
				widest = it
			}
		}
		raw := strings.Repeat("7", widest.Digits-1) + " "
		id := widest.Addr.Wire()

		sess, p := openTestSession(t, row, radioImage{exAnswers: map[string]string{id: exAnswer(id, raw)}})
		val, err := sess.ReadSetting(context.Background(), id)
		if err != nil {
			t.Fatalf("%s: ReadSetting(%q): %v", modelNameFor(row), id, err)
		}
		if val.ID != id || val.State != driver.SettingKnown || val.Raw != raw {
			t.Errorf("%s: ReadSetting(%q) = %+v, want {ID:%q Raw:%q SettingKnown}", modelNameFor(row), id, val, id, raw)
		}
		if got := p.Transcript(); len(got) != len(probeFrames)+1 || got[len(probeFrames)] != "EX"+id+"0000;" {
			t.Errorf("%s: transcript = %v, want the probe and one ten-byte EX read at the full address", modelNameFor(row), got)
		}
	}
}

// TestReadSetting_TheWIDTHBOUNDIsThisRowsOwnInventoryRow is the second half of
// "built from its own inventory, never the other's", and it is the half a
// count cannot see.
//
// core/kw's ParseEXAnswer takes the INVENTORY ROW, not just an address, and
// bounds P5 by that row's printed width (A19: an answer never exceeds the
// width the parameter list prints for that menu number). Menu 000 is where the
// two rows differ most cheaply: "Display brightness", one digit, on the S
// (590:569) and "Version information (4 ASCII characters) read only" on the SG
// (590:749). So a four-character answer to menu 000 is legitimate on the SG
// and REFUSED on the S — and a driver that looked the item up in the wrong
// row's table would get both verdicts backwards.
func TestReadSetting_TheWIDTHBOUNDIsThisRowsOwnInventoryRow(t *testing.T) {
	const id = "000"
	for _, tc := range []struct {
		row     Row
		raw     string
		wantErr bool
	}{
		{RowSG, "1.05", false}, // four characters: the SG's version row
		{RowS, "1.05", true},   // the S prints one digit at this address
		{RowS, "3", false},     // and answers it happily
	} {
		sess, _ := openTestSession(t, tc.row, radioImage{exAnswers: map[string]string{id: exAnswer(id, tc.raw)}})
		val, err := sess.ReadSetting(context.Background(), id)
		switch {
		case tc.wantErr && err == nil:
			t.Errorf("%s: ReadSetting(%q) accepted a %d-character answer and returned %+v; this row prints %d", modelNameFor(tc.row), id, len(tc.raw), val, inventoryFor(t, tc.row)[0].Digits)
		case !tc.wantErr && err != nil:
			t.Errorf("%s: ReadSetting(%q) with a %d-character answer: %v", modelNameFor(tc.row), id, len(tc.raw), err)
		case !tc.wantErr && val.Raw != tc.raw:
			t.Errorf("%s: Raw = %q, want %q", modelNameFor(tc.row), val.Raw, tc.raw)
		}
	}
}

// TestReadSetting_ARejectionIsUnavailableAndNotAnError is the seam's own rule
// (driver.SettingsReader): a known id the radio declines on the wire returns
// SettingValue{State: SettingUnavailable} and NO error.
//
// IT ADDS NO FIFTH READING OF THIS FAMILY'S "?;", which is worth saying on a
// pair of radios whose unattributed NAK already means either "Command syntax
// was incorrect" or "Command was not executed due to the current status of
// the transceiver" and may not appear at all (590:100-108). SettingUnavailable
// INTERPRETS nothing: the address was a member of this row's own printed
// inventory before the frame went out, so a "?;" here records that the radio
// declined to report a setting it declares, and guesses nothing about why.
//
// IT IS THE ONE PLACE THIS DRIVER DOES NOT FAIL A READ WHOLE ON A "?;", and
// the difference is the seam's, not this radio's: ReadChannel's rejection
// fails the session read whole (decision 5) because a channel read has no
// neutral state for "the radio declined", and the settings seam has exactly
// one. core/clone's ReadSettings then continues the walk and marks the
// snapshot incomplete, which is its own rule and pinned in its own package.
func TestReadSetting_ARejectionIsUnavailableAndNotAnError(t *testing.T) {
	for _, row := range bothRows {
		// No exAnswers entry at all: this scripted radio answers "?;".
		sess, p := openTestSession(t, row, radioImage{})
		val, err := sess.ReadSetting(context.Background(), "010")
		if err != nil {
			t.Fatalf("%s: a rejected settings read returned an error: %v", modelNameFor(row), err)
		}
		if val.ID != "010" || val.State != driver.SettingUnavailable || val.Raw != "" {
			t.Errorf("%s: ReadSetting = %+v, want {ID:\"010\" Raw:\"\" SettingUnavailable}", modelNameFor(row), val)
		}
		if got := p.Transcript(); len(got) != len(probeFrames)+1 {
			t.Errorf("%s: transcript = %v, want the probe and exactly one EX read", modelNameFor(row), got)
		}
	}
}

// TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame pins BOTH halves of
// the plan's Task 13 clause "a timeout typed and never retried into a second
// frame".
//
// TYPED: silence is *kw.TimeoutError naming the EX command, which says in as
// many words that it is not an inference of absence — the same wireFailure
// rule the probe's ID; and FV; and the read path's MR already report, applied
// once for the whole family.
//
// AND EXACTLY ONE FRAME. mrSpec carries one retry, because a 120-slot walk
// should not fail on a single swallowed reply. The settings read carries NONE:
// the plan chose it, and pinning the frame COUNT is what makes that zero
// non-vacuous — a spec that silently gained mrSpec's retry would satisfy every
// other assertion here.
func TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame(t *testing.T) {
	sess, p := openTestSession(t, RowSG, radioImage{exSilent: map[string]bool{"010": true}})
	val, err := sess.ReadSetting(context.Background(), "010")
	if err == nil {
		t.Fatalf("a silent settings read succeeded, returning %+v", val)
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("errors.Is(err, transport.ErrTimeout) = false for %v", err)
	}
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("errors.As(err, **kw.TimeoutError) = false for %v", err)
	}
	if to.Command != "EX" {
		t.Errorf("TimeoutError.Command = %q, want EX", to.Command)
	}
	if got := p.Transcript(); len(got) != len(probeFrames)+1 {
		t.Errorf("transcript = %v, want the probe and ONE EX read — this spec carries no retry", got)
	}
}

// TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrame is the settings path's
// own first rung, and it is bank membership's counterpart: an id that is not a
// row of THIS ROW's inventory is refused with nothing sent.
//
// THE ROW-SPECIFIC CASE IS THE ONE THAT MATTERS. "088" is a well-formed
// three-digit address, a real setting on the TS-590SG and outside the
// TS-590S's printed domain of 000 ~ 087 entirely (590:543). A driver that
// consulted one shared table would send it to an S and let the radio answer
// for itself, which is exactly what core/kw's own MaxEXAddress bound exists to
// prevent one layer down.
func TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrame(t *testing.T) {
	for _, tc := range []struct {
		row Row
		id  string
	}{
		{RowS, "088"},   // the SG has it; this row's domain stops at 087
		{RowSG, "100"},  // above the SG's own domain too
		{RowS, "88"},    // two digits: not this family's shape
		{RowSG, "0100"}, // four digits: a sibling family's shape
		{RowSG, "01A"},  // not decimal
		{RowSG, ""},
	} {
		sess, p := openTestSession(t, tc.row, radioImage{})
		val, err := sess.ReadSetting(context.Background(), tc.id)
		if err == nil {
			t.Fatalf("%s: ReadSetting(%q) succeeded, returning %+v", modelNameFor(tc.row), tc.id, val)
		}
		var unknown *UnknownSettingError
		if !errors.As(err, &unknown) {
			t.Errorf("%s: ReadSetting(%q): errors.As(err, **UnknownSettingError) = false for %v", modelNameFor(tc.row), tc.id, err)
		}
		if got := p.Transcript(); len(got) != len(probeFrames) {
			t.Errorf("%s: ReadSetting(%q) put %v on the wire; an unknown id is refused before any frame is built", modelNameFor(tc.row), tc.id, got[len(probeFrames):])
		}
	}
}

// TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth is the negative-space
// proof of kw.PrefixLenMatcher's FULL-ADDRESS OBLIGATION, which that function
// can state but cannot enforce — it has no address-shaped parameter.
//
// Every one of this row's menu addresses answers with a frame starting "EX"
// (590:543-544), so a bare "EX" prefix would let transport.Engine.Do correlate
// a DIFFERENT address's answer — one still in flight, or an unsolicited
// push — as this read's own, and hand back one setting's value labelled as
// another's. The exact length is left variable because there is no single EX
// answer width to pin: P5 is declared "variable length" with no printed
// ceiling on either radio (590:555-556), which is A19, and the per-item bound
// is ParseEXAnswer's afterwards.
func TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth(t *testing.T) {
	sess, _ := openTestSession(t, RowSG, radioImage{})
	match := sess.exSpec("042").Match

	for _, tc := range []struct {
		frame string
		want  bool
	}{
		{exAnswer("042", "1"), true},
		{exAnswer("042", "12345678"), true}, // a variable width is admitted
		{exAnswer("043", "1"), false},       // a neighbour's answer is not ours
		{exAnswer("142", "1"), false},
		{"MR" + strings.Repeat("0", 47) + ";", false},
	} {
		if got := match([]byte(tc.frame)); got != tc.want {
			t.Errorf("exSpec(\"042\").Match(%q) = %v, want %v", tc.frame, got, tc.want)
		}
	}
	if bare := kw.PrefixLenMatcher("EX", 0); !bare([]byte(exAnswer("043", "1"))) {
		t.Error("the bare-prefix control did not accept a foreign address's answer; this test's negative space is empty")
	}
	if n := sess.exSpec("042").RetryReads; n != 0 {
		t.Errorf("exSpec RetryReads = %d, want 0 — a settings timeout is never retried into a second frame", n)
	}
}

// TestReadSetting_IsAtomicUnderOpMu: the whole exchange holds s.opMu, which is
// the Session type's own rule — ONE DRIVER OPERATION at a time, the probe's
// three frames, one ReadChannel, one WriteChannel or one ReadSetting.
//
// It is not protecting the SETTING: one EX read is one Engine.Do and the
// engine already serialises an individual exchange. It is protecting the
// OTHER operations — a settings read landing inside a channel read or a write
// would interleave its own frame with theirs on a radio whose only
// acknowledgement is silence.
//
// THE HOOK IS read.go's readChannelGapHook, reused exactly as the write path's
// own pin reuses it (TestWriteChannel_IsAtomicUnderOpMu): it parks a
// ReadChannel deterministically inside the lock, where scheduling alone would
// almost never reproduce the interleaving.
func TestReadSetting_IsAtomicUnderOpMu(t *testing.T) {
	const id = "001"
	sess, p := openTestSession(t, RowSG, radioImage{
		mrAnswers: map[string]string{mrAddr(id): populatedMR(id)},
		exAnswers: map[string]string{"010": exAnswer("010", "1")},
	})

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	readChannelGapHook = func() {
		entered <- struct{}{}
		<-release
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), id); err != nil {
			t.Errorf("parked ReadChannel: %v", err)
		}
	}()
	<-entered

	settingDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadSetting(context.Background(), "010"); err != nil {
			t.Errorf("ReadSetting: %v", err)
		}
		close(settingDone)
	}()

	select {
	case <-settingDone:
		t.Fatal("ReadSetting returned while a ReadChannel held opMu")
	case <-time.After(250 * time.Millisecond):
	}
	if got := p.Transcript(); len(got) != len(probeFrames) {
		t.Errorf("transcript while the read is parked inside opMu = %v, want the probe's three frames alone", got)
	}

	close(release)
	wg.Wait()

	got := p.Transcript()
	if len(got) != len(probeFrames)+2 {
		t.Fatalf("transcript = %v, want the probe, one MR and one EX", got)
	}
	if !strings.HasPrefix(got[len(probeFrames)], "MR") || !strings.HasPrefix(got[len(probeFrames)+1], "EX") {
		t.Errorf("transcript = %v, want the parked ReadChannel's MR before the released ReadSetting's EX", got)
	}
}

// TestCloneReadSettings_WalksTheWholeDescriptor is the plan's "whole-descriptor
// walk mutation-proved", and it is the only test in this package that runs the
// descriptor through core/clone's own preflight — the T1 dependency's real
// consumer.
//
// clone.ReadSettings validates every descriptor item ID through
// codeplug.MenuSnapshot.Validate BEFORE any wire traffic (core/clone/settings.go),
// and that rule admits exactly 3, 4 or 6 ASCII digits. A three-digit Kenwood ID
// passes only because Stage 0 widened it; without the widening this walk would
// read the whole radio and fail afterwards. Neither the descriptor tests above
// nor core/clone's own fixtures can see that: they validate the tree, and
// core/clone's tests use their own IDs.
//
// MUTATION-PROOF BY CONSTRUCTION: every item is answered with a DIFFERENT
// value, built at that item's own printed width, so a snapshot that mapped one
// answer onto another's ID could not pass unnoticed and a driver that returned
// a value under the wrong ID is caught by core/clone's own ID check. The widths
// come from the inventory BY POSITION rather than by looking each ID up in it,
// so a mis-shaped ID breaks the preflight this test exists to exercise rather
// than the fixture.
func TestCloneReadSettings_WalksTheWholeDescriptor(t *testing.T) {
	const row = RowSG
	items := inventoryFor(t, row)
	d := SettingsDescriptor(row)

	answers := map[string]string{}
	want := map[string]string{}
	var order []string
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			for _, it := range g.Items {
				if len(order) >= len(items) {
					t.Fatalf("the descriptor holds more items than the inventory's %d", len(items))
				}
				raw := fmt.Sprintf("%0*d", items[len(order)].Digits, len(order)%10)
				answers[it.ID] = exAnswer(it.ID, raw)
				want[it.ID] = raw
				order = append(order, it.ID)
			}
		}
	}

	sess, _ := openTestSession(t, row, radioImage{exAnswers: answers})
	snap, err := clone.NewService(sess, clone.SnapshotStore{}).ReadSettings(context.Background())
	if err != nil {
		t.Fatalf("clone.ReadSettings: %v", err)
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("the returned snapshot fails codeplug.MenuSnapshot.Validate: %v", err)
	}
	if snap.Descriptor != d.Version {
		t.Errorf("snapshot Descriptor = %q, want this row's own %q carried through verbatim", snap.Descriptor, d.Version)
	}
	if !snap.Complete {
		t.Error("snapshot Complete = false, want true — every item was answered")
	}
	if len(snap.Entries) != len(order) {
		t.Fatalf("snapshot holds %d entries, the descriptor declares %d items", len(snap.Entries), len(order))
	}
	for i, e := range snap.Entries {
		if e.ID != order[i] {
			t.Fatalf("Entries[%d].ID = %q, want %q — ReadSettings walks the descriptor in its own order", i, e.ID, order[i])
		}
		if e.State != codeplug.MenuKnown || e.Value != want[e.ID] {
			t.Errorf("entry %q = {Value:%q State:%v}, want {Value:%q MenuKnown}", e.ID, e.Value, e.State, want[e.ID])
		}
	}
}

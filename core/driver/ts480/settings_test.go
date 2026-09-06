// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

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
	kwts480 "github.com/gm5dna/open-rig-programmer/core/kw/ts480"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// exAnswer builds the EX answer frame for address id carrying P5 raw:
// "EX" P1P1P1 P2P2 P3 P4 P5 ";" — ten fixed bytes with P5 inserted before the
// terminator, P2 "00" and P3/P4 '0' (480:410).
//
// HAND-BUILT AND NOT core/kw's, deliberately: this codec builds no EX Set and
// no EX Answer at all, so an answer produced by it would pin the parser
// against a builder that does not exist — and these tests need answers the
// codec would refuse, such as a wrong address or a P5 wider than the row's
// printed width.
//
// THE BOOK'S OWN WORKED EXAMPLES ARE THIS SHAPE: "EX00000000; (Display
// illumination OFF)." and "EX00000003; (Display brightness level 3)."
// (480:415-416), and exAnswer("000", "0") reproduces the first byte for byte.
func exAnswer(id, raw string) string { return "EX" + id + "0000" + raw + ";" }

// TestSettingsDescriptor_IsBuiltFromThisRowsOwnInventory: ts480-ex@1 is built
// from kwts480.EXItems() — 61 rows over the printed domain "000 ~ 060: Menu
// No." (480:401) — and from no other table.
//
// THE COUNT IS THE WEAKEST HALF AND THE LABELS ARE THE STRONG ONE. 61 against
// the TS-590S's 88 and the TS-590SG's 100 would catch a wholesale swap, but a
// descriptor built from the wrong inventory would publish the wrong setting
// NAME at every address, which is what a user actually reads.
func TestSettingsDescriptor_IsBuiltFromThisRowsOwnInventory(t *testing.T) {
	items := kwts480.EXItems()
	if len(items) != 61 {
		t.Fatalf("the generated inventory has %d rows, want 61 over the printed domain 000 ~ 060 (480:401, A26)", len(items))
	}
	d := SettingsDescriptor()
	group := d.Menus[0].Groups[0]
	if len(group.Items) != len(items) {
		t.Fatalf("the descriptor holds %d items, the inventory %d", len(group.Items), len(items))
	}
	for i, it := range group.Items {
		want := items[i]
		if it.ID != want.Addr.Wire() {
			t.Errorf("item %d: ID = %q, want %q", i, it.ID, want.Addr.Wire())
		}
		if it.Label != want.Name {
			t.Errorf("item %d (%s): Label = %q, want the chart's own Function column %q", i, it.ID, it.Label, want.Name)
		}
	}
	// The book's own first row, spelled out, so a silent re-ordering of the
	// generated table is visible here as well as in the count.
	if got := group.Items[0]; got.ID != "000" || got.Label != "Display brightness" {
		t.Errorf("the first item is %q/%q, want \"000\"/\"Display brightness\" (480:427)", got.ID, got.Label)
	}
	if got := group.Items[len(group.Items)-1]; got.ID != "060" {
		t.Errorf("the last item is %q, want \"060\" — the printed domain's ceiling (480:401)", got.ID)
	}
}

// TestSettingsDescriptor_IsOneFlatMenuAndOneGroup is Stuart decisions row 4,
// RULED 05/09/2026 (plan P8): FLAT.
//
// Kenwood prints NO GROUP STRUCTURE — the EX parameter list is a menu number,
// a function name and a parameter legend, which is why all three Kenwood
// profiles register LabelsAbsent and every EXItem's P1Label and P2Label is "".
// driver.SettingsDescriptor is a two-level tree whose Validate requires at
// least one menu and at least one group inside it, so the two nodes exist to
// satisfy the neutral type and carry no claim that this radio has a menu
// called "Menu" or a group inside it.
//
// The alternative on the table was a DISPLAY-ONLY decade grouping with the
// item IDs unchanged. It was refused because it would put structure in a book
// that prints none. The cost is named rather than hidden and it is a cost in
// navigation only — 61 items in one list, against the TS-590SG's 100.
func TestSettingsDescriptor_IsOneFlatMenuAndOneGroup(t *testing.T) {
	d := SettingsDescriptor()
	if len(d.Menus) != 1 {
		t.Fatalf("the descriptor has %d menus, want exactly one (P8)", len(d.Menus))
	}
	if len(d.Menus[0].Groups) != 1 {
		t.Fatalf("the menu has %d groups, want exactly one (P8)", len(d.Menus[0].Groups))
	}
	if d.Menus[0].ID != settingsRootID || d.Menus[0].Label != settingsRootLabel {
		t.Errorf("the menu is %q/%q, want %q/%q", d.Menus[0].ID, d.Menus[0].Label, settingsRootID, settingsRootLabel)
	}
	if d.Menus[0].Groups[0].ID != settingsRootID || d.Menus[0].Groups[0].Label != settingsRootLabel {
		t.Errorf("the group is %q/%q, want %q/%q", d.Menus[0].Groups[0].ID, d.Menus[0].Groups[0].Label, settingsRootID, settingsRootLabel)
	}
}

// TestSettingsDescriptor_TheStructuralWordsAreTheTS590sOwn is the T13 review's
// M1 carried into this package: the string "Menu" comes from the books' EX
// PARAMETER-LIST COLUMN HEADING (480:424 and 480:498 for this row's two
// printed lists; 590:566 and 590:746 for the pair's), NOT from 480:401, which
// is the printed menu DOMAIN and is cited for that instead.
//
// The two descriptors must use the SAME structural words, because they are the
// same claim about the same absent structure on three rows of one family. This
// test pins the values rather than the citation, and the citation lives in
// settings.go's own comment where a reader meets it.
func TestSettingsDescriptor_TheStructuralWordsAreTheTS590sOwn(t *testing.T) {
	if settingsRootID != "MENU" {
		t.Errorf("settingsRootID = %q, want %q — the same word core/driver/ts590 uses", settingsRootID, "MENU")
	}
	if settingsRootLabel != "Menu" {
		t.Errorf("settingsRootLabel = %q, want %q — the EX parameter list's own column heading (480:424, 480:498)", settingsRootLabel, "Menu")
	}
}

// TestSettingsDescriptor_ValidatesAndIDsAreUniqueAtAllThreeLevels: the neutral
// type requires a non-empty ID and Label at every level and IDs unique at all
// three, which the flat shape satisfies.
func TestSettingsDescriptor_ValidatesAndIDsAreUniqueAtAllThreeLevels(t *testing.T) {
	d := SettingsDescriptor()
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	seen := map[string]bool{}
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			for _, it := range g.Items {
				if seen[it.ID] {
					t.Errorf("item ID %q appears twice", it.ID)
				}
				seen[it.ID] = true
			}
		}
	}
	if len(seen) != 61 {
		t.Errorf("%d distinct item IDs, want 61", len(seen))
	}
}

// TestSettingsDescriptor_ItemIDsAreThreeASCIIDigits is why Stage 0 had to
// widen codeplug's setting-ID rule a third time: clone.ReadSettings validates
// every descriptor item ID through codeplug.MenuSnapshot.Validate before any
// wire traffic, and until that widening the rule admitted four and six digits
// only.
//
// DISPLAY IS BUILT SEPARATELY FROM ID AND HAPPENS TO EQUAL IT. Kenwood prints
// the address as ONE three-digit Menu No., so the printed form and the wire
// form coincide on this family — a FACT ABOUT THIS CHART, not a rule: the
// FT-710 prints "01-01-01" against a six-digit ID. They are two expressions
// here, and this test states the coincidence so that nobody later
// "de-duplicates" one into the other.
//
// THE MENU NUMBER IS THREE DIGITS THOUGH THIS ROW'S SLOT IDENTIFIERS ARE TWO,
// and the two widths are unrelated: the slot width is MC's printed one
// (480:830) and the menu width is the EX chart's (480:401). A reader who
// "corrected" one to match the other would break the other's wire form.
func TestSettingsDescriptor_ItemIDsAreThreeASCIIDigits(t *testing.T) {
	for _, it := range SettingsDescriptor().Menus[0].Groups[0].Items {
		if len(it.ID) != 3 {
			t.Errorf("item ID %q is not three characters (480:401)", it.ID)
			continue
		}
		for _, b := range []byte(it.ID) {
			if b < '0' || b > '9' {
				t.Errorf("item ID %q is not three ASCII digits", it.ID)
				break
			}
		}
		if it.Display != it.ID {
			t.Errorf("item %q: Display = %q — they are two expressions that coincide on this chart, and both must be built", it.ID, it.Display)
		}
	}
}

// TestSettingsDescriptor_VersionIsThisRowsOwn: the snapshot carries the
// descriptor version through verbatim, so a snapshot taken from one row can
// later be checked against the descriptor version that produced it.
func TestSettingsDescriptor_VersionIsThisRowsOwn(t *testing.T) {
	if got := SettingsDescriptor().Version; got != settingsVersion {
		t.Errorf("Version = %q, want %q", got, settingsVersion)
	}
	if settingsVersion != "ts480-ex@1" {
		t.Errorf("settingsVersion = %q, want %q (P8)", settingsVersion, "ts480-ex@1")
	}
	// AND IT MUST NOT BE EITHER 590 ROW'S. The three menu tables collide on
	// addresses with different meanings, so a snapshot validated against the
	// wrong descriptor would map every value onto the wrong setting.
	for _, foreign := range []string{"ts590s-ex@1", "ts590sg-ex@1"} {
		if settingsVersion == foreign {
			t.Errorf("this row publishes the %s's descriptor version", foreign)
		}
	}
}

// TestSettingsDescriptor_IsADefensiveCopy: nothing outside settings.go may
// hold a reference to the shared tree, because a caller that mutated what it
// was handed would silently change what every later caller received.
func TestSettingsDescriptor_IsADefensiveCopy(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{})
	for _, tc := range []struct {
		name string
		get  func() driver.SettingsDescriptor
	}{
		{"the package func", SettingsDescriptor},
		{"the driver's static provider", New(Simulated).(driver.StaticSettingsProvider).StaticSettingsDescriptor},
		{"the session's", sess.SettingsDescriptor},
	} {
		handed := tc.get()
		handed.Menus[0].Label = "MUTATED"
		handed.Menus[0].Groups[0].Items[0].Label = "MUTATED"
		again := tc.get()
		if again.Menus[0].Label == "MUTATED" || again.Menus[0].Groups[0].Items[0].Label == "MUTATED" {
			t.Errorf("%s: a caller's mutation reached the next caller", tc.name)
		}
	}
}

// TestSession_ImplementsTheSettingsPair keeps the compile-time assertions
// honest at run time too: a driver that lost an optional interface would still
// build, and internal/wiring type-asserts for each.
func TestSession_ImplementsTheSettingsPair(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{})
	if _, ok := any(sess).(driver.SettingsReader); !ok {
		t.Error("*Session does not implement driver.SettingsReader")
	}
	if _, ok := any(New(Simulated)).(driver.StaticSettingsProvider); !ok {
		t.Error("the driver does not implement driver.StaticSettingsProvider")
	}
}

// TestReadSetting_AKnownAnswerIsReturnedVerbatim: P5 comes back as the radio
// spelt it, with no value semantics of any kind applied — the tree carries an
// address, the chart's Function name and a display form, and no legend, no
// units, no enumerated options and no default.
func TestReadSetting_AKnownAnswerIsReturnedVerbatim(t *testing.T) {
	sess, p := openSession(t, Simulated, radioImage{
		exAnswers: map[string]string{"000": exAnswer("000", "3")},
	})
	got, err := sess.ReadSetting(context.Background(), "000")
	if err != nil {
		t.Fatalf("ReadSetting(\"000\"): %v", err)
	}
	if got.ID != "000" || got.State != driver.SettingKnown || got.Raw != "3" {
		t.Errorf("ReadSetting = %+v, want {ID:000 Raw:3 SettingKnown}", got)
	}
	// THE READ IS TEN BYTES AND THE ANSWER IS ELEVEN, which is the whole of
	// the EX pair's shape: "E X P1 P1 P1 P2 P2 P3 P4 ;" is the Read chart's
	// ten fixed bytes (480:410), and the Set and Answer append P5. The book's
	// own worked examples are the ELEVEN-byte form — "EX00000000; (Display
	// illumination OFF)." and "EX00000003; (Display brightness level 3)."
	// (480:415-416) — so a reader who took one of them for a read frame would
	// be one byte out, which is why both widths are named here.
	want := append(append([]string{}, probeFrames...), "EX0000000;")
	if len(p.Transcript()) != len(want) || p.Transcript()[3] != want[3] {
		t.Errorf("transcript = %v, want the probe's three frames and one %q — the ten-byte Read frame of 480:410, not the eleven-byte Set of 480:415", p.Transcript(), want[3])
	}
	if len(want[3]) != kw.EXReadLen {
		t.Errorf("this test's expected read frame is %d bytes, want kw.EXReadLen (%d)", len(want[3]), kw.EXReadLen)
	}
}

// TestReadSetting_TheWidthBoundIsThisRowsOwnInventoryRow is A19 applied where
// it belongs: the ceiling is PER MENU NUMBER, from the transcribed chart, and
// the bound is consulted from the SAME inventory row the address was looked up
// in — which is why settingsSurface holds a map and not merely a descriptor.
//
// Menu 000 prints ONE digit (480:427) and the two-digit menus are named by the
// block's own prose, "Menu No. 32, 35 and 48 ~ 52 use 2-digit parameters"
// (480:411) — a sentence that OMITS menu 034, which the chart itself prints as
// two digits. That contradiction is erratum E22 and is core/kw/ts480's to
// carry; what this test needs from it is only that the widths come from the
// inventory rather than from a literal here.
func TestReadSetting_TheWidthBoundIsThisRowsOwnInventoryRow(t *testing.T) {
	items := kwts480.EXItems()
	byID := map[string]kw.EXItem{}
	for _, it := range items {
		byID[it.Addr.Wire()] = it
	}
	for _, id := range []string{"000", "032", "034", "048"} {
		it, ok := byID[id]
		if !ok {
			t.Fatalf("the inventory has no menu %s", id)
		}
		atWidth := strings.Repeat("7", it.Digits)
		tooWide := atWidth + "7"

		sess, _ := openSession(t, Simulated, radioImage{
			exAnswers: map[string]string{id: exAnswer(id, atWidth)},
		})
		got, err := sess.ReadSetting(context.Background(), id)
		if err != nil {
			t.Errorf("menu %s at its own printed width (%d): %v", id, it.Digits, err)
		} else if got.Raw != atWidth {
			t.Errorf("menu %s: Raw = %q, want %q", id, got.Raw, atWidth)
		}

		sess, _ = openSession(t, Simulated, radioImage{
			exAnswers: map[string]string{id: exAnswer(id, tooWide)},
		})
		if _, err := sess.ReadSetting(context.Background(), id); !errors.Is(err, kw.ErrParse) {
			t.Errorf("menu %s one digit wider than its printed width: err = %v, want a parse refusal (A19)", id, err)
		}
	}
}

// TestReadSetting_ARejectionIsUnavailableAndNotAnError is driver.SettingsReader's
// own stated contract, and it is THE ONE PATH IN THIS DRIVER WHERE A REJECTION
// DOES NOT FAIL THE OPERATION WHOLE.
//
// It adds no new reading of this family's unattributed NAK: the address was a
// member of this row's own printed inventory before the frame went out, so a
// "?;" here records that the radio declined to report a setting it declares,
// and guesses nothing about why. A CHANNEL read has no neutral state meaning
// "the radio declined" — which is why ReadChannel's "?;" fails the read whole
// (decision 5) — and the settings seam has exactly one.
func TestReadSetting_ARejectionIsUnavailableAndNotAnError(t *testing.T) {
	// An address with no entry in the image is answered "?;".
	sess, _ := openSession(t, Simulated, radioImage{})
	got, err := sess.ReadSetting(context.Background(), "005")
	if err != nil {
		t.Fatalf("a \"?;\" answer returned an error: %v", err)
	}
	if got.State != driver.SettingUnavailable {
		t.Errorf("State = %v, want SettingUnavailable", got.State)
	}
	if got.ID != "005" {
		t.Errorf("ID = %q, want the requested address carried through", got.ID)
	}
	if got.Raw != "" {
		t.Errorf("Raw = %q on an unavailable setting, want empty", got.Raw)
	}
}

// TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame: SILENCE stays a
// failure, typed by wireFailure as *kw.TimeoutError, which says in as many
// words that it is not an inference of absence — the same rule, and the same
// function, as the probe's ID; and TY; and the read path's MR.
//
// ZERO RETRIES, where mrSpec carries one, and the two paths differ in what a
// retry would buy: a whole-radio read is a hundred MR exchanges that should
// not fail on one swallowed reply, while a settings read is one item a caller
// asked for and can ask for again. Nothing here claims a TS-480 would answer a
// repeat differently — no Kenwood radio has been on a wire for this project —
// the driver simply does not send a second frame the caller did not ask for.
// PINNED BY FRAME COUNT.
func TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame(t *testing.T) {
	sess, p := openSession(t, Simulated, radioImage{
		exSilent: map[string]bool{"010": true},
	})
	_, err := sess.ReadSetting(context.Background(), "010")
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("err = %v (%T), want *kw.TimeoutError", err, err)
	}
	if to.Book != kw.Book480 || to.Command != "EX" {
		t.Errorf("timeout = %v/%q, want the 480's book and \"EX\"", to.Book, to.Command)
	}
	if got := p.Transcript(); len(got) != len(probeFrames)+1 {
		t.Errorf("transcript = %v, want the probe's three frames and EXACTLY ONE EX read", got)
	}
}

// TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrame: membership is checked
// against THIS ROW's inventory, not against the printed domain, and the
// refusal happens before any wire traffic.
//
// "061" IS THE CASE THAT MATTERS: one past this row's printed ceiling of 060
// (480:401), where the TS-590S's domain runs to 087 and the TS-590SG's to 099
// (590:543-544). A four- or six-digit Yaesu ID lands here too.
func TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrame(t *testing.T) {
	for _, id := range []string{"061", "099", "0000", "000000", "00", "abc", ""} {
		sess, p := openSession(t, Simulated, radioImage{})
		_, err := sess.ReadSetting(context.Background(), id)
		var unknown *UnknownSettingError
		if !errors.As(err, &unknown) {
			t.Errorf("ReadSetting(%q): err = %v (%T), want *UnknownSettingError", id, err, err)
			continue
		}
		if unknown.ID != id || unknown.Model != modelName {
			t.Errorf("ReadSetting(%q): the refusal names %q/%q", id, unknown.ID, unknown.Model)
		}
		if got := len(p.Transcript()); got != len(probeFrames) {
			t.Errorf("ReadSetting(%q) sent %d frames; an unknown id is refused before any frame", id, got)
		}
	}
}

// TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth is the
// negative-space proof of the shared-prefix-family rule, which
// kw.PrefixLenMatcher's own doc comment states and which it cannot enforce,
// having no address-shaped parameter.
//
// Every one of this row's 61 menu addresses answers with a frame starting
// "EX", so a bare prefix would let transport.Engine.Do correlate a DIFFERENT
// address's still-in-flight answer as this read's own and hand back one
// setting's value labelled as another's.
//
// THE EXACT LENGTH IS 0 — VARIABLE, and this is the ONE frame in this family
// that is: P5 is "A string of characters (Variable length). Normally 1-digit
// for the TS-480." with no printed ceiling (480:409-411), which is A19. Every
// other frame this row sends or receives is fixed (ID 6, TY 6, MC 6, MR 50).
func TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{})
	spec := sess.exSpec("034")
	if spec.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 — a settings read is never retried into a second frame", spec.RetryReads)
	}
	if spec.Match == nil {
		t.Fatal("the spec carries no matcher")
	}
	// Our own address's answer, at two different widths: both admitted.
	for _, raw := range []string{"7", "77", "777"} {
		if !spec.Match([]byte(exAnswer("034", raw))) {
			t.Errorf("the matcher refused our own address's answer at width %d; the width is A19's and is applied by the parser, not by correlation", len(raw))
		}
	}
	// A DIFFERENT address's answer: refused, which a bare "EX" prefix would
	// not do.
	if spec.Match([]byte(exAnswer("035", "7"))) {
		t.Error("the matcher accepted menu 035's answer for a read of 034 — the prefix must carry the full three-digit address")
	}
	if kw.PrefixLenMatcher("EX", 0)([]byte(exAnswer("035", "7"))) != true {
		t.Error("the bare-prefix control did not accept a foreign address; this test's negative space is not what it claims")
	}
}

// TestReadSetting_AForeignAddressAnswerIsNeverCorrelated is the same fact
// end to end: an answer naming a different menu number never reaches this
// read at all, so the read times out rather than returning another setting's
// value under this one's ID.
func TestReadSetting_AForeignAddressAnswerIsNeverCorrelated(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{
		exAnswers: map[string]string{"034": exAnswer("035", "7")},
	})
	_, err := sess.ReadSetting(context.Background(), "034")
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("err = %v, want a timeout — a foreign address's answer must not be correlated as this read's", err)
	}
}

// TestReadSetting_IsAtomicUnderOpMu is the Session type's own rule applied to
// the third operation: ONE DRIVER OPERATION at a time (P13/P14). The lock is
// not protecting the setting — one EX read is one Engine.Do, and the engine
// already serialises an individual exchange — it is protecting the OTHERS: a
// settings read landing inside Open's three-frame probe or a ReadChannel would
// interleave a frame of its own with theirs.
func TestReadSetting_IsAtomicUnderOpMu(t *testing.T) {
	sess, p := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("01"): populatedMR("01")},
		exAnswers: map[string]string{"000": exAnswer("000", "3")},
	})

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	var parkOnce sync.Once
	readChannelGapHook = func() {
		select {
		case entered <- struct{}{}:
		default:
		}
		parkOnce.Do(func() { <-release })
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), "01"); err != nil {
			t.Errorf("parked ReadChannel: %v", err)
		}
	}()
	<-entered

	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = sess.ReadSetting(context.Background(), "000")
		close(done)
	}()

	select {
	case <-done:
		t.Error("a settings read completed while a channel read held opMu")
	case <-time.After(150 * time.Millisecond):
	}
	if got := p.Transcript(); len(got) != len(probeFrames) {
		t.Errorf("transcript = %v while one operation is parked inside the lock, want the probe's frames alone", got)
	}
	close(release)
	wg.Wait()
}

// TestCloneReadSettings_WalksTheWholeDescriptor exercises the T1 dependency
// rather than merely stating it: clone.ReadSettings validates every descriptor
// item ID through codeplug.MenuSnapshot.Validate BEFORE any wire traffic, and
// a three-digit ID passes only because Stage 0 widened that rule.
//
// MUTATION-PROOF BY CONSTRUCTION: every item is answered with a DIFFERENT
// value, built at that item's own printed width, so a snapshot that mapped one
// answer onto another's ID could not pass unnoticed. The widths come from the
// inventory BY POSITION rather than by looking each ID up in it, so a
// mis-shaped ID breaks the preflight this test exists to exercise rather than
// the fixture.
func TestCloneReadSettings_WalksTheWholeDescriptor(t *testing.T) {
	items := kwts480.EXItems()
	d := SettingsDescriptor()

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

	sess, _ := openSession(t, Simulated, radioImage{exAnswers: answers})
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

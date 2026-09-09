// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

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
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// inventory is this row's own generated menu inventory, asked of the
// ACCESSOR rather than of a literal count: no count for either chart appears
// anywhere in this package, because each row's count is its own registered
// profile's ExpectedRows and a second copy here would be a figure nobody
// derived.
func inventory() []kw.EXItem { return ma.EXItems990S() }

// exAnswer builds the SHORT EX answer frame for the five-digit address id
// carrying P5 raw: "EX" + P1P2P2P3P3 + P4 + P5 + ";", where P4 is the space
// both books print — "Response is always a space" (990:1737-1741).
//
// HAND-BUILT AND NOT core/kw/ma's, deliberately: an answer produced by the
// codec under test would pin the parser against a builder that does not exist
// (this codec builds no EX Set and no EX Answer at all), and these tests need
// answers the codec would refuse — a foreign address, a P5 wider than the
// row's printed width.
func exAnswer(id, raw string) string { return "EX" + id + " " + raw + ";" }

// exAnswerFixed is the SAME answer in the twenty-four-byte form this book's
// own Set/Answer diagram draws, with P5 filling its fifteen-wide window at
// positions 9-23 and ';' nailed to 24 (990:1738-1747). That the two forms
// both exist is erratum E19, and both are admitted.
func exAnswerFixed(id, raw string) string {
	return "EX" + id + " " + raw + strings.Repeat(" ", 15-len(raw)) + ";"
}

// TestSettingsDescriptor_IsBuiltFromThisRowsOwnInventory is P8's central
// claim and the one a wrong import would break silently: `ts990s-ex@1` is
// built from THIS book's chart and never from the sibling's.
//
// THE COUNT IS THE WEAKEST HALF OF THIS TEST AND THE LABELS ARE THE STRONG
// HALF. The two books' EX charts are different charts in different books, not
// one chart with rows added: a descriptor built from the wrong one would
// publish the wrong setting name at every address it happened to share.
func TestSettingsDescriptor_IsBuiltFromThisRowsOwnInventory(t *testing.T) {
	items := inventory()
	d := SettingsDescriptor()

	var got []driver.SettingItem
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			got = append(got, g.Items...)
		}
	}
	if len(got) != len(items) {
		t.Fatalf("descriptor holds %d items, this row's own inventory %d", len(got), len(items))
	}
	l := layout()
	for i, it := range items {
		if got[i].ID != l.WireEXAddress(it.Addr) || got[i].Label != it.Name {
			t.Fatalf("item %d = %q/%q, want %q/%q — in the inventory's own order",
				i, got[i].ID, got[i].Label, l.WireEXAddress(it.Addr), it.Name)
		}
	}
	if err := d.Validate(); err != nil {
		t.Errorf("the descriptor does not validate: %v", err)
	}
	if d.Version != settingsVersion {
		t.Errorf("Version = %q, want %q", d.Version, settingsVersion)
	}
}

// TestSettingsDescriptor_IsOneFlatMenuAndOneGroup is the FLAT shape RULED on
// 05/09/2026 (Stuart decisions row 4; plan P8).
//
// This chart prints a three-part address, a Function name and a value legend,
// and NO GROUP HIERARCHY AT ALL — its columns are P1, P2, P3, Function and
// P5, which is why this row's extable profile registers LabelsAbsent and
// every EXItem's P1Label and P2Label is "". driver.SettingsDescriptor is a
// two-level tree whose Validate requires at least one menu and one group, so
// the single menu and the single group here are STRUCTURAL and carry no claim
// about the radio. A tens-decade tree, or one grouped on P1's two printed
// values, would put structure in a book that prints none.
func TestSettingsDescriptor_IsOneFlatMenuAndOneGroup(t *testing.T) {
	d := SettingsDescriptor()
	if len(d.Menus) != 1 {
		t.Fatalf("%d menus, want exactly one", len(d.Menus))
	}
	if len(d.Menus[0].Groups) != 1 {
		t.Fatalf("%d groups, want exactly one", len(d.Menus[0].Groups))
	}
	if len(d.Menus[0].Groups[0].Items) != len(inventory()) {
		t.Error("the single group does not hold the whole inventory")
	}
}

// TestSettingsDescriptor_ItemIDsAreFiveASCIIDigitsAndDisplayIsSeparate is
// decision 13's shape in this package: the ID is the radio's own WIRE
// address, five digits — which is the whole reason core/codeplug's
// setting-ID width rule had to admit five — and the Display form is built
// SEPARATELY, in the chart's own three-column spacing.
//
// THE TWO ARE NEVER DE-DUPLICATED INTO ONE. They differ here, unlike pair 1's
// row where the printed form and the wire form coincide, and stating that
// keeps a later reader from making the ID the display form of whatever
// address shape comes next.
func TestSettingsDescriptor_ItemIDsAreFiveASCIIDigitsAndDisplayIsSeparate(t *testing.T) {
	seen := map[string]bool{}
	for _, it := range SettingsDescriptor().Menus[0].Groups[0].Items {
		if len(it.ID) != 5 {
			t.Fatalf("item ID %q is %d characters, want five (990:1720-1735)", it.ID, len(it.ID))
		}
		for i := 0; i < len(it.ID); i++ {
			if it.ID[i] < '0' || it.ID[i] > '9' {
				t.Fatalf("item ID %q is not five ASCII digits", it.ID)
			}
		}
		if seen[it.ID] {
			t.Fatalf("item ID %q appears twice", it.ID)
		}
		seen[it.ID] = true
		if want := it.ID[:1] + " " + it.ID[1:3] + " " + it.ID[3:]; it.Display != want {
			t.Errorf("item %q Display = %q, want the chart's own spacing %q", it.ID, it.Display, want)
		}
	}
	if !seen["00000"] || !seen["10000"] {
		t.Error("the descriptor is missing one of the two printed lists: 0 00 00 (Menu) and 1 00 00 (Advanced Menu)")
	}
}

// TestSettingsDescriptor_CarriesTheFirmwareConditionalRowsLikeAnyOther is the
// forward note T13's review left for this task. Five rows of this chart are
// printed with a firmware condition — 0 00 34, 0 02 12, 0 03 10, 0 07 19 and
// 0 08 33 (990:1814, 1866, 1924, 2054, 2163) — and the settings surface READS
// them like any other row: nothing here branches on FV, which the driver does
// not carry at all, because a menu row's presence is the CHART's statement
// and not a runtime one. The marker travels in the row's own printed Function
// name, so a user sees it.
func TestSettingsDescriptor_CarriesTheFirmwareConditionalRowsLikeAnyOther(t *testing.T) {
	labels := map[string]string{}
	for _, it := range SettingsDescriptor().Menus[0].Groups[0].Items {
		labels[it.ID] = it.Label
	}
	for _, id := range []string{"00034", "00212", "00310", "00719", "00833"} {
		label, ok := labels[id]
		if !ok {
			t.Errorf("menu %q is not in the descriptor", id)
			continue
		}
		if !strings.Contains(strings.ToLower(label), "firmware") {
			t.Errorf("menu %q is labelled %q, and this chart prints a firmware condition on it", id, label)
		}
	}
}

// TestSettingsDescriptor_IsACopyPerCall: nothing outside settings.go may hold
// a reference to the tree built once at package initialisation, because a
// caller that mutated what it was handed would silently change what every
// later caller received.
func TestSettingsDescriptor_IsACopyPerCall(t *testing.T) {
	d := SettingsDescriptor()
	d.Menus[0].Groups[0].Items[0].Label = "mutated"
	if got := SettingsDescriptor().Menus[0].Groups[0].Items[0].Label; got == "mutated" {
		t.Error("SettingsDescriptor hands out the shared tree; a caller's edit reached the next caller")
	}
}

// TestSettingsDescriptor_TheThreeGettersAgree: the package-level func, the
// driver's StaticSettingsDescriptor and the session's SettingsDescriptor are
// one tree, because this driver's settings depend only on a static inventory
// and no Kenwood row discovers anything at all.
func TestSettingsDescriptor_TheThreeGettersAgree(t *testing.T) {
	d := New(Simulated)
	static, ok := d.(driver.StaticSettingsProvider)
	if !ok {
		t.Fatal("the driver does not implement driver.StaticSettingsProvider")
	}
	sess, _ := openTestSession(t, radioImage{})
	reader, ok := any(sess).(driver.SettingsReader)
	if !ok {
		t.Fatal("the session does not implement driver.SettingsReader")
	}
	for name, got := range map[string]driver.SettingsDescriptor{
		"driver":  static.StaticSettingsDescriptor(),
		"session": reader.SettingsDescriptor(),
	} {
		want := SettingsDescriptor()
		if got.Version != want.Version || len(got.Menus[0].Groups[0].Items) != len(want.Menus[0].Groups[0].Items) {
			t.Errorf("%s descriptor differs from the package-level one", name)
		}
	}
}

// TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth is the
// negative-space proof for the ONE property this row's EX matcher must have:
// the prefix carries the whole five-digit address, never the bare "EX"
// command name, because every one of this radio's menu addresses answers with
// a frame starting "EX" and a bare prefix would let a DIFFERENT address's
// still-in-flight answer be correlated as this read's own.
//
// THE EXACT LENGTH IS 0 — variable — because P5's width is per item class and
// this book admits two answer shapes for one address (E19).
func TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth(t *testing.T) {
	sess, _ := openTestSession(t, radioImage{})
	spec := sess.exSpec("00000")
	if spec.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 — a settings read is one item a caller can ask for again", spec.RetryReads)
	}
	for _, tc := range []struct {
		frame string
		admit bool
	}{
		{exAnswer("00000", "010"), true},
		{exAnswerFixed("00000", "010"), true},
		{exAnswer("00001", "010"), false},
		{exAnswer("10000", "010"), false},
	} {
		if got := spec.Match([]byte(tc.frame)); got != tc.admit {
			t.Errorf("Match(%q) = %v, want %v", tc.frame, got, tc.admit)
		}
	}
}

// TestReadSetting_ReadsBothAnswerFormsAndTakesThePrintedWidth is E19
// exercised on the wire, and it also states this surface's OWN posture: RAW
// VALUES ONLY, at the row's own printed width.
//
// The fixed form's P5 is fifteen bytes with the value and its pad
// undifferentiated, and NEITHER BOOK SAYS WHICH BYTE THE PAD IS — which is
// exactly why the codec hands the split to its caller rather than guessing:
// "a caller reading a setting must take the first item.Digits characters as
// the value and treat the remainder as pad" (core/kw/ma/envelope.go:376-381).
// THIS IS THAT CALLER, and until the milestone-close review (S1-MED-1) it
// discharged the obligation nowhere, so 193 of the 194 rows of a shipped
// settings CSV carried pad in the value column.
//
// IT IS A PREFIX TAKE, NOT A TRIM, and that is what answers the objection
// this comment used to carry — that this row's TextWidths of {10, 15} could
// make a text row's trailing space content. A19 is a CEILING: for a row
// printed at fifteen the prefix IS the whole fifteen bytes and nothing is
// removed; only bytes past the row's own printed width are dropped, and no
// printed answer occupies those. Nor did the split need a fleet seam change:
// item.Digits is in hand at ReadSetting's own first line, so
// driver.SettingItem never has to carry a width.
//
// THE SHORT FORM IS UNCHANGED, which is the other half of the pin: a value
// already at its printed width comes back byte for byte, so this is one
// behaviour on two answer shapes rather than a rule that fires on one.
func TestReadSetting_ReadsBothAnswerFormsAndTakesThePrintedWidth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		frame string
		want  string
	}{
		{"the short form the P5 note requires", exAnswer("00000", "010"), "010"},
		// The fixed form's pad is DROPPED, and the row's printed width is
		// what draws the line: menu 00000 prints three digits, so the
		// value is the leading three of the fifteen the radio sent.
		{"the fixed 24-byte form the diagram draws (E19)", exAnswerFixed("00000", "010"), "010"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, p := openTestSession(t, radioImage{exAnswers: map[string]string{"00000": tc.frame}})
			got, err := sess.ReadSetting(context.Background(), "00000")
			if err != nil {
				t.Fatalf("ReadSetting: %v", err)
			}
			if got.ID != "00000" || got.State != driver.SettingKnown || got.Raw != tc.want {
				t.Errorf("ReadSetting = %+v, want ID 00000, SettingKnown and Raw %q", got, tc.want)
			}
			if frames := p.Transcript(); len(frames) != len(probeFrames)+1 || frames[len(probeFrames)] != "EX00000;" {
				t.Errorf("transcript = %v, want the probe and ONE eight-byte EX read carrying no P4 (990:1734-1736)", frames)
			}
		})
	}
}

// TestReadSetting_TheWidestPrintedClassIsReadAtItsOwnWidth exercises the
// eighth-digit class this book prints and the sibling's does not —
// "Frequency settings use 8 digits" (990:1749-1750) — which is where this
// row's extable profile gets its MaxDigits of 8. The width is the INVENTORY
// ROW's, applied by the codec, so a value at the printed width comes back
// verbatim and a wider one is refused rather than truncated (A19).
func TestReadSetting_TheWidestPrintedClassIsReadAtItsOwnWidth(t *testing.T) {
	const id = "00805" // 0 08 05, Fixed Mode LF Band Lower Limit, 8 digits
	sess, _ := openTestSession(t, radioImage{exAnswers: map[string]string{id: exAnswer(id, "00030000")}})
	got, err := sess.ReadSetting(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadSetting: %v", err)
	}
	if got.Raw != "00030000" {
		t.Errorf("Raw = %q, want the eight digits the chart prints for this row", got.Raw)
	}

	wide, _ := openTestSession(t, radioImage{exAnswers: map[string]string{id: exAnswer(id, "000300000")}})
	if _, err := wide.ReadSetting(context.Background(), id); !errors.Is(err, kw.ErrParse) {
		t.Errorf("a nine-character answer for an eight-digit row = %v, want the codec's A19 refusal", err)
	}
}

// TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrameIsBuilt: membership is
// checked HERE and not in core/kw/ma, and it covers two refusals a caller
// cannot tell apart from the id alone and does not need to — a malformed
// shape (a Yaesu sibling's four- or six-digit ID, or pair 1's three-digit
// Kenwood one) and a well-formed five-digit address this chart does not
// print. Both mean the same thing: this radio has no such setting.
//
// THE CHART IS SPARSE, which is why membership rather than a scalar bound is
// the check at all: "09999" is inside every printed component domain and is
// not a row of this chart.
func TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrameIsBuilt(t *testing.T) {
	for _, id := range []string{
		"",
		"000",    // pair 1's three-digit Kenwood menu number
		"010101", // a Yaesu (P1,P2,P3) triple
		"0000",   // a Yaesu pair
		"09999",  // well formed, five digits, and no row of this chart
		"20000",  // P1 outside the two printed values
		"0000A",  // not decimal
	} {
		sess, p := openTestSession(t, radioImage{})
		before := len(p.Transcript())
		_, err := sess.ReadSetting(context.Background(), id)
		var unknown *UnknownSettingError
		if !errors.As(err, &unknown) {
			t.Errorf("id %q: errors.As(*UnknownSettingError) = false for %v", id, err)
		}
		if got := len(p.Transcript()); got != before {
			t.Errorf("id %q: %d frames were sent; the refusal is settled from this row's own inventory alone", id, got-before)
		}
	}
}

// TestReadSetting_ARejectionIsSettingUnavailableAndNotAnError is
// driver.SettingsReader's own stated contract, and it is the ONE path in this
// driver where a "?;" does not fail the operation whole: the address was a
// member of this row's own inventory before the frame went out, so the
// rejection records that the radio declined to report a setting it declares,
// and guesses nothing about why. A channel read has no neutral state meaning
// "the radio declined"; the settings seam has exactly one.
func TestReadSetting_ARejectionIsSettingUnavailableAndNotAnError(t *testing.T) {
	sess, _ := openTestSession(t, radioImage{exReject: map[string]bool{"00000": true}})
	got, err := sess.ReadSetting(context.Background(), "00000")
	if err != nil {
		t.Fatalf("ReadSetting: %v, want no error", err)
	}
	if got.State != driver.SettingUnavailable || got.Raw != "" {
		t.Errorf("ReadSetting = %+v, want SettingUnavailable with no Raw", got)
	}
}

// TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame: silence stays a
// failure, typed as *kw.TimeoutError, which says in as many words that it is
// not an inference of absence — and the driver sends no second frame the
// caller did not ask for.
func TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame(t *testing.T) {
	sess, p := openTestSession(t, radioImage{exSilent: map[string]bool{"00000": true}})
	_, err := sess.ReadSetting(context.Background(), "00000")
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("errors.As(*kw.TimeoutError) = false for %v", err)
	}
	if got := len(p.Transcript()); got != len(probeFrames)+1 {
		t.Errorf("transcript = %v, want the probe and exactly one EX read", p.Transcript())
	}
}

// TestReadSetting_AForeignAddressAnswerIsNeverCorrelated: the whole address
// is the correlation key, so an answer naming another menu is never delivered
// as this read's own — the read times out instead of returning one setting's
// value under another's name.
func TestReadSetting_AForeignAddressAnswerIsNeverCorrelated(t *testing.T) {
	sess, _ := openTestSession(t, radioImage{exAnswers: map[string]string{"00000": exAnswer("00001", "010")}})
	_, err := sess.ReadSetting(context.Background(), "00000")
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("err = %v, want a timeout; a foreign address's answer must not be correlated as this read's own", err)
	}
}

// TestReadSetting_AnAnswerWiderThanItsPrintedWidthIsRefused keeps A19 the
// CODEC's verdict rather than this driver's: the width bound is the inventory
// row's own Digits, applied by ParseEXAnswer, and the driver adds only the
// context the parser cannot know.
func TestReadSetting_AnAnswerWiderThanItsPrintedWidthIsRefused(t *testing.T) {
	sess, _ := openTestSession(t, radioImage{exAnswers: map[string]string{"00000": exAnswer("00000", "0102")}})
	_, err := sess.ReadSetting(context.Background(), "00000")
	if !errors.Is(err, kw.ErrParse) {
		t.Fatalf("err = %v, want the codec's parse refusal (A19)", err)
	}
	if !strings.Contains(err.Error(), "00000") {
		t.Errorf("the message does not name the setting: %v", err)
	}
}

// TestReadSetting_IsAtomicUnderOpMu: the whole exchange holds s.opMu, which
// is the Session type's own rule — ONE DRIVER OPERATION at a time. It is not
// protecting the setting (one EX read is one Engine.Do, which the engine
// already serialises); it is protecting the OTHERS, on a radio whose only
// acknowledgement of a Set is silence.
//
// THE HOOK IS read.go's readChannelGapHook, which parks a ReadChannel
// deterministically inside the lock.
func TestReadSetting_IsAtomicUnderOpMu(t *testing.T) {
	sess, p := openTestSession(t, radioImage{
		ma0Answers: map[string]string{"001": populatedMA0("001")},
		exAnswers:  map[string]string{"00000": exAnswer("00000", "010")},
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
		if _, err := sess.ReadChannel(context.Background(), "001"); err != nil {
			t.Errorf("parked ReadChannel: %v", err)
		}
	}()
	<-entered

	settingDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadSetting(context.Background(), "00000"); err != nil {
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
		t.Fatalf("transcript = %v, want the probe, one MA0 and one EX", got)
	}
	if !strings.HasPrefix(got[len(probeFrames)], "MA0") || !strings.HasPrefix(got[len(probeFrames)+1], "EX") {
		t.Errorf("transcript = %v, want the parked ReadChannel's MA0 before the released ReadSetting's EX", got)
	}
}

// TestCloneReadSettings_WalksTheWholeDescriptor is the only test in this
// package that runs the descriptor through core/clone's own preflight, and it
// is where decision 13's widening is actually exercised.
//
// clone.ReadSettings validates every descriptor item ID through
// codeplug.MenuSnapshot.Validate BEFORE any wire traffic, and that rule
// admits exactly 3, 4, 5 or 6 ASCII digits. THIS ROW'S FIVE-DIGIT IDs PASS
// ONLY BECAUSE STAGE 0 WIDENED IT; without the widening this walk would read
// the whole radio and fail afterwards. Neither the descriptor tests above nor
// core/clone's own fixtures can see that.
//
// MUTATION-PROOF BY CONSTRUCTION: every item is answered with a DIFFERENT
// value built at that item's own printed width, so a snapshot that mapped one
// answer onto another's ID could not pass unnoticed.
func TestCloneReadSettings_WalksTheWholeDescriptor(t *testing.T) {
	items := inventory()
	d := SettingsDescriptor()

	answers := map[string]string{}
	want := map[string]string{}
	var order []string
	for _, it := range d.Menus[0].Groups[0].Items {
		if len(order) >= len(items) {
			t.Fatalf("the descriptor holds more items than the inventory's %d", len(items))
		}
		raw := fmt.Sprintf("%0*d", items[len(order)].Digits, len(order)%10)
		answers[it.ID] = exAnswer(it.ID, raw)
		want[it.ID] = raw
		order = append(order, it.ID)
	}

	sess, _ := openTestSession(t, radioImage{exAnswers: answers})
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

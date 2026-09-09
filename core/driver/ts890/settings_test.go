// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

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
)

// exAnswer renders one EX answer: "EX" + the five-character address + P4,
// which "is always a space" in an answer (890:1912-1915), + P5 + ';'.
func exAnswer(addr, value string) string { return "EX" + addr + " " + value + ";" }

// inventory is this row's own generated menu inventory, reached through the
// PACKAGE-LEVEL accessor rather than the layout's, so a descriptor built from
// one and a test asserting against the other cross-check each other.
func inventory() []kw.EXItem { return ma.EXItems890S() }

// firstItems returns the first n rows of the inventory, for tests that need a
// concrete address without transcribing one.
func firstItems(t *testing.T, n int) []kw.EXItem {
	t.Helper()
	items := inventory()
	if len(items) < n {
		t.Fatalf("this row's inventory holds %d rows, want at least %d", len(items), n)
	}
	return items[:n]
}

// wireOf renders an inventory row's five-character wire address, which is also
// its descriptor item ID.
func wireOf(item kw.EXItem) string { return layout().WireEXAddress(item.Addr) }

// TestSettingsDescriptor_IsBuiltFromThisRowsOwnInventory walks the whole tree
// against the generated inventory, ROW BY ROW AND IN ORDER: every item's ID is
// that row's wire address and every Label its chart Function column verbatim.
//
// NO COUNT IS TRANSCRIBED HERE. The plan states none and the orchestrator
// holds it (P21): the descriptor's population IS the inventory's, so the
// assertion is an identity rather than a number this file could get wrong.
func TestSettingsDescriptor_IsBuiltFromThisRowsOwnInventory(t *testing.T) {
	items := inventory()
	d := SettingsDescriptor()
	if len(d.Menus) != 1 || len(d.Menus[0].Groups) != 1 {
		t.Fatalf("descriptor has %d menus, want one menu holding one group", len(d.Menus))
	}
	got := d.Menus[0].Groups[0].Items
	if len(got) != len(items) {
		t.Fatalf("the descriptor holds %d items, the inventory %d", len(got), len(items))
	}
	for i, it := range items {
		if want := wireOf(it); got[i].ID != want {
			t.Errorf("item %d ID = %q, want the wire address %q", i, got[i].ID, want)
		}
		if got[i].Label != it.Name {
			t.Errorf("item %d Label = %q, want the chart's Function column %q", i, got[i].Label, it.Name)
		}
	}
}

// TestSettingsDescriptor_IsOneFlatMenuAndOneGroup pins P8's shape, which is a
// RULING rather than an oversight (Stuart decisions row 4, applied unchanged
// from pair 1): neither book prints a group structure at all, so every
// P1Label and P2Label in the inventory is empty, and a decade grouping would
// put this project's invention in the tree as though it were the radio's own
// menu hierarchy. The cost is navigation only.
func TestSettingsDescriptor_IsOneFlatMenuAndOneGroup(t *testing.T) {
	d := SettingsDescriptor()
	if len(d.Menus) != 1 {
		t.Fatalf("Menus = %d, want 1", len(d.Menus))
	}
	m := d.Menus[0]
	if m.ID != settingsRootID || m.Label != settingsRootLabel {
		t.Errorf("menu = {%q %q}, want {%q %q}", m.ID, m.Label, settingsRootID, settingsRootLabel)
	}
	if len(m.Groups) != 1 {
		t.Fatalf("Groups = %d, want 1", len(m.Groups))
	}
	for _, it := range inventory() {
		if it.P1Label != "" || it.P2Label != "" {
			t.Fatalf("inventory row %v carries group labels: this chart prints none", it.Addr)
		}
	}
}

// TestSettingsDescriptor_ValidatesAndIDsAreUnique runs the neutral type's own
// rule — at least one menu, at least one group, non-empty IDs and labels at
// every level, IDs unique at all three — which is what core/clone applies
// before any wire traffic.
func TestSettingsDescriptor_ValidatesAndIDsAreUnique(t *testing.T) {
	d := SettingsDescriptor()
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	seen := map[string]bool{}
	for _, it := range d.Menus[0].Groups[0].Items {
		if seen[it.ID] {
			t.Errorf("item ID %q appears twice", it.ID)
		}
		seen[it.ID] = true
	}
}

// TestSettingsDescriptor_ItemIDsAreTheFiveCharacterWireAddress is the reason
// Stage 0 had to widen codeplug's setting-ID rule a fourth time: this family's
// address is a GROUPED triple — one menu-type digit, two category digits and
// two item digits (890:1897-1911) — where every other registered row's is one
// flat field. clone.ReadSettings validates every ID through
// codeplug.MenuSnapshot.Validate before any frame goes out, and until that
// widening the rule admitted three, four and six digits only.
func TestSettingsDescriptor_ItemIDsAreTheFiveCharacterWireAddress(t *testing.T) {
	for _, it := range SettingsDescriptor().Menus[0].Groups[0].Items {
		if len(it.ID) != 5 {
			t.Errorf("item ID %q is %d characters, want five (890:1900)", it.ID, len(it.ID))
			continue
		}
		for i := 0; i < len(it.ID); i++ {
			if it.ID[i] < '0' || it.ID[i] > '9' {
				t.Errorf("item ID %q is not five ASCII digits", it.ID)
				break
			}
		}
	}
}

// TestSettingsDescriptor_DisplayIsThePrintedAddressAndNotTheWireOne is the
// warning pair 1's settings.go and the FT-891's both carry, and on this row it
// is not a coincidence to guard against but a real difference: the chart
// prints the address as THREE CELLS — "0   00   00" under headings P1, P2, P3
// (890:1937-1939) — while the wire runs them together into five characters. So
// the display form and the ID are built separately and are not equal, and
// nobody may later "de-duplicate" one into the other.
func TestSettingsDescriptor_DisplayIsThePrintedAddressAndNotTheWireOne(t *testing.T) {
	items := inventory()
	got := SettingsDescriptor().Menus[0].Groups[0].Items
	for i, it := range items {
		want := fmt.Sprintf("%d %02d %02d", it.Addr.P1, it.Addr.P2, it.Addr.P3)
		if got[i].Display != want {
			t.Errorf("item %d Display = %q, want the chart's own three cells %q", i, got[i].Display, want)
		}
		if got[i].Display == got[i].ID {
			t.Errorf("item %d Display equals its ID: the printed form and the wire form differ on this row", i)
		}
	}
}

// TestSettingsDescriptor_VersionIsThisRowsOwn pins the string
// codeplug.MenuSnapshot.Descriptor carries through verbatim, so a snapshot can
// later be checked against the descriptor version that produced it. ONE string
// for one package, where pair 1 needs two: this package serves one registry
// row.
func TestSettingsDescriptor_VersionIsThisRowsOwn(t *testing.T) {
	if got := SettingsDescriptor().Version; got != "ts890s-ex@1" {
		t.Errorf("Version = %q, want %q", got, "ts890s-ex@1")
	}
}

// TestSettingsDescriptor_ThreeGettersOneTreeAndEachIsACopy pins both halves:
// the package function, the driver's StaticSettingsDescriptor and the
// session's SettingsDescriptor return the same tree, and each returns a
// defensive Clone — a caller that mutated what it was handed must never alter
// what every later caller receives.
func TestSettingsDescriptor_ThreeGettersOneTreeAndEachIsACopy(t *testing.T) {
	d := New(Simulated)
	static, ok := d.(driver.StaticSettingsProvider)
	if !ok {
		t.Fatal("the driver does not implement driver.StaticSettingsProvider")
	}
	sess, _ := openTestSession(t, Simulated, radioImage{})

	pkg := SettingsDescriptor()
	for name, got := range map[string]driver.SettingsDescriptor{
		"StaticSettingsDescriptor":   static.StaticSettingsDescriptor(),
		"Session.SettingsDescriptor": sess.SettingsDescriptor(),
	} {
		if got.Version != pkg.Version || len(got.Menus[0].Groups[0].Items) != len(pkg.Menus[0].Groups[0].Items) {
			t.Errorf("%s returned a different tree", name)
		}
	}

	mutated := SettingsDescriptor()
	mutated.Menus[0].Groups[0].Items[0].Label = "MUTATED"
	if SettingsDescriptor().Menus[0].Groups[0].Items[0].Label == "MUTATED" {
		t.Error("mutating a returned descriptor changed the shared original")
	}
}

// TestSession_ImplementsSettingsReader is the interface half: without it the
// registration task would wire a session whose settings surface is
// unreachable, and nothing else in this package would notice.
func TestSession_ImplementsSettingsReader(t *testing.T) {
	sess, _ := openTestSession(t, Simulated, radioImage{})
	if _, ok := any(sess).(driver.SettingsReader); !ok {
		t.Fatal("*Session does not implement driver.SettingsReader")
	}
}

// TestReadSetting_AKnownAnswerIsReturnedVerbatim is the ordinary path: one EX
// frame, and P5 returned exactly as the radio spelled it.
//
// RAW VALUES ONLY, AND NO VALUE SEMANTICS AT ALL. The tree carries an address,
// the chart's own Function name and a display form; it carries no value
// legend, no units, no enumerated options and no default. Every legend in this
// parameter list is therefore untouched by this surface, which is what keeps a
// menu read a transcription of what the radio said rather than an
// interpretation of it.
func TestReadSetting_AKnownAnswerIsReturnedVerbatim(t *testing.T) {
	item := firstItems(t, 1)[0]
	id := wireOf(item)
	sess, port := openTestSession(t, Simulated, radioImage{
		exAnswers: map[string]string{id: exAnswer(id, "002")},
	})
	got, err := sess.ReadSetting(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadSetting: %v", err)
	}
	if got.ID != id || got.State != driver.SettingKnown || got.Raw != "002" {
		t.Errorf("ReadSetting = %+v, want {ID:%s Raw:002 SettingKnown}", got, id)
	}
	want := append(append([]string{}, probeFrames...), "EX"+id+";")
	if got := port.Transcript(); len(got) != len(want) || got[len(got)-1] != want[len(want)-1] {
		t.Errorf("transcript = %v, want %v — one EX read and nothing else", got, want)
	}
}

// TestReadSetting_TheWidthBoundIsThisRowsOwnInventoryRow pins A19: the ceiling
// on P5 is the width THIS row's chart prints for THAT menu number, taken from
// the inventory row rather than from a family-wide constant. A shorter answer
// is admitted — A19 claims a maximum and nothing else — and a wider one is
// refused as a parse error.
func TestReadSetting_TheWidthBoundIsThisRowsOwnInventoryRow(t *testing.T) {
	var item kw.EXItem
	for _, it := range inventory() {
		if it.Digits == 3 {
			item = it
			break
		}
	}
	if item.Digits != 3 {
		t.Fatal("no three-digit row in this row's inventory")
	}
	id := wireOf(item)

	t.Run("a shorter answer is admitted", func(t *testing.T) {
		sess, _ := openTestSession(t, Simulated, radioImage{
			exAnswers: map[string]string{id: exAnswer(id, "1")},
		})
		got, err := sess.ReadSetting(context.Background(), id)
		if err != nil {
			t.Fatalf("ReadSetting: %v", err)
		}
		if got.Raw != "1" {
			t.Errorf("Raw = %q, want %q", got.Raw, "1")
		}
	})

	t.Run("a wider answer is refused", func(t *testing.T) {
		sess, _ := openTestSession(t, Simulated, radioImage{
			exAnswers: map[string]string{id: exAnswer(id, "0021")},
		})
		_, err := sess.ReadSetting(context.Background(), id)
		if !errors.Is(err, kw.ErrParse) {
			t.Fatalf("err = %v, want a kw parse error naming this row's printed width", err)
		}
	})
}

// TestReadSetting_ARejectionIsUnavailableAndNotAnError is the ONE path in this
// driver where a "?;" does not fail the operation whole — driver.SettingsReader's
// own stated contract, and it adds no new reading of this family's
// unattributed NAK: the address was a member of this row's own printed
// inventory before the frame went out, so the outcome records that the radio
// declined to report a setting it declares, and guesses nothing about why. A
// channel read has no neutral state meaning "the radio declined"; the settings
// seam has exactly one.
func TestReadSetting_ARejectionIsUnavailableAndNotAnError(t *testing.T) {
	id := wireOf(firstItems(t, 1)[0])
	sess, _ := openTestSession(t, Simulated, radioImage{}) // an unscripted address answers "?;"
	got, err := sess.ReadSetting(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadSetting: %v, want no error", err)
	}
	if got.State != driver.SettingUnavailable || got.Raw != "" {
		t.Errorf("ReadSetting = %+v, want {SettingUnavailable, no value}", got)
	}
}

// TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame pins the NO-RETRY
// clause by frame COUNT. A whole-radio channel read is a hundred exchanges
// that should not fail on one swallowed reply; a settings read is one item a
// caller asked for and can ask for again. Nothing here claims a TS-890S would
// answer a repeat differently — no Kenwood radio has been on a wire for this
// project — the driver simply does not send a second frame the caller did not
// ask for.
func TestReadSetting_ATimeoutIsTypedAndSendsExactlyOneFrame(t *testing.T) {
	id := wireOf(firstItems(t, 1)[0])
	sess, port := openTestSession(t, Simulated, radioImage{exSilent: map[string]bool{id: true}})
	_, err := sess.ReadSetting(context.Background(), id)
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("err = %v (%T), want a *kw.TimeoutError", err, err)
	}
	frames := 0
	for _, f := range port.Transcript() {
		if strings.HasPrefix(f, "EX") {
			frames++
		}
	}
	if frames != 1 {
		t.Errorf("a timed-out settings read sent %d EX frames, want exactly one", frames)
	}
}

// TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrame is the MEMBERSHIP check
// decision 13 moved the truncation guard onto: an address that is not a row of
// this row's own inventory is refused with *UnknownSettingError BEFORE any
// frame is built.
//
// BOTH CHARTS ARE SPARSE, with gaps inside every category, so membership is
// the only honest bound — a scalar ceiling would admit an address no chart
// prints, and this book says what the radio does with one: "Entering a
// non-existing number causes an error to occur" (890:1904).
func TestReadSetting_AnUnknownIDIsRefusedBeforeAnyFrame(t *testing.T) {
	for _, tc := range []struct{ name, id string }{
		{"a malformed width", "000"},
		{"a well-formed address this chart does not print", "09999"},
		{"a sibling row's four-digit form", "0000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, port := openTestSession(t, Simulated, radioImage{})
			_, err := sess.ReadSetting(context.Background(), tc.id)
			var unknown *UnknownSettingError
			if !errors.As(err, &unknown) {
				t.Fatalf("err = %v (%T), want an *UnknownSettingError", err, err)
			}
			for _, f := range port.Transcript() {
				if strings.HasPrefix(f, "EX") {
					t.Errorf("an unknown id put %q on the wire", f)
				}
			}
		})
	}
}

// TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth is the
// negative-space proof for the matcher: the prefix is "EX" plus the WHOLE
// five-character address, never the bare command name, because every one of
// this row's menu addresses answers with a frame starting "EX" — so a bare
// prefix would let the engine correlate a DIFFERENT address's still-in-flight
// answer as this read's own and hand back one setting's value under another's
// name.
//
// THE EXACT LENGTH IS LEFT 0 — VARIABLE. This row's EX answer has no fixed
// width at all: its ruler reads "9~" and the terminator's header cell is the
// letter "x" (890:1898-1904), so only the prefix is checked here and
// ParseEXAnswer applies the per-item printed width afterwards.
func TestExSpec_CarriesTheFullAddressAndAdmitsAVariableWidth(t *testing.T) {
	items := firstItems(t, 2)
	mine, other := wireOf(items[0]), wireOf(items[1])
	sess, _ := openTestSession(t, Simulated, radioImage{})
	match := sess.exSpec(mine).Match

	for _, tc := range []struct {
		name  string
		frame string
		want  bool
	}{
		{"this address's answer", exAnswer(mine, "002"), true},
		{"the same answer at another width", exAnswer(mine, "0021234"), true},
		{"another address's answer", exAnswer(other, "002"), false},
		{"a bare EX prefix", "EX;", false},
	} {
		if got := match([]byte(tc.frame)); got != tc.want {
			t.Errorf("%s: match(%q) = %v, want %v", tc.name, tc.frame, got, tc.want)
		}
	}
}

// TestReadSetting_AForeignAddressAnswerIsNeverCorrelated is the same rule
// end to end: a frame naming another menu address is not this read's answer,
// is never delivered to the parser, and the read times out rather than
// returning one setting's value under another's name.
func TestReadSetting_AForeignAddressAnswerIsNeverCorrelated(t *testing.T) {
	items := firstItems(t, 2)
	mine, other := wireOf(items[0]), wireOf(items[1])
	sess, _ := openTestSession(t, Simulated, radioImage{
		exAnswers: map[string]string{mine: exAnswer(other, "002")},
	})
	_, err := sess.ReadSetting(context.Background(), mine)
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("err = %v (%T), want a *kw.TimeoutError", err, err)
	}
}

// TestReadSetting_IsAtomicUnderOpMu pins the Session type's own rule — ONE
// DRIVER OPERATION at a time. The lock is not protecting the setting, since
// one EX read is one exchange and the engine already serialises those; it is
// protecting the OTHERS: a settings read landing inside a channel WRITE's
// read-then-Set pair would interleave a frame of its own between two frames
// that must not be split.
func TestReadSetting_IsAtomicUnderOpMu(t *testing.T) {
	id := wireOf(firstItems(t, 1)[0])
	sess, port := openTestSession(t, Simulated, radioImage{
		ma0Answers: map[string]string{"007": answerFor(t, 7, occupiedSimplex())},
		exAnswers:  map[string]string{id: exAnswer(id, "002")},
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
		if _, err := sess.ReadChannel(context.Background(), "007"); err != nil {
			t.Errorf("parked ReadChannel: %v", err)
		}
	}()
	<-entered

	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadSetting(context.Background(), id); err != nil {
			t.Errorf("ReadSetting: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("ReadSetting returned while a ReadChannel held opMu")
	case <-time.After(250 * time.Millisecond):
	}
	if got := port.Transcript(); len(got) != len(probeFrames) {
		t.Errorf("transcript while the read is parked inside opMu = %v, want the probe's frames alone", got)
	}

	close(release)
	wg.Wait()

	got := port.Transcript()
	if len(got) != len(probeFrames)+2 {
		t.Fatalf("transcript = %v, want the probe, one MA0 and one EX", got)
	}
	if !strings.HasPrefix(got[len(probeFrames)], "MA0") || !strings.HasPrefix(got[len(probeFrames)+1], "EX") {
		t.Errorf("transcript = %v, want the parked read's MA0 before the released settings read's EX", got)
	}
}

// TestCloneReadSettings_WalksTheWholeDescriptor is the only test in this
// package that runs the descriptor through core/clone's own preflight, and it
// is what makes the five-digit ID widening a live dependency rather than a
// stated one: clone.ReadSettings validates every descriptor item ID through
// codeplug.MenuSnapshot.Validate BEFORE any wire traffic, and without the
// widening this walk would read the whole radio and fail afterwards.
//
// MUTATION-PROOF BY CONSTRUCTION: every item is answered with a DIFFERENT
// value, built at that item's own printed width, so a snapshot that mapped one
// answer onto another's ID could not pass unnoticed.
func TestCloneReadSettings_WalksTheWholeDescriptor(t *testing.T) {
	items := inventory()
	d := SettingsDescriptor()

	answers := map[string]string{}
	want := map[string]string{}
	var order []string
	for _, it := range d.Menus[0].Groups[0].Items {
		raw := fmt.Sprintf("%0*d", items[len(order)].Digits, len(order)%10)
		answers[it.ID] = exAnswer(it.ID, raw)
		want[it.ID] = raw
		order = append(order, it.ID)
	}

	sess, _ := openTestSession(t, Simulated, radioImage{exAnswers: answers})
	snap, err := clone.NewService(sess, clone.SnapshotStore{}).ReadSettings(context.Background())
	if err != nil {
		t.Fatalf("clone.ReadSettings: %v", err)
	}
	if err := snap.Validate(); err != nil {
		t.Fatalf("the returned snapshot fails codeplug.MenuSnapshot.Validate: %v", err)
	}
	if snap.Descriptor != d.Version {
		t.Errorf("snapshot Descriptor = %q, want %q carried through verbatim", snap.Descriptor, d.Version)
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

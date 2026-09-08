// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestLayouts_AreConfiguredAndTheZeroLayoutIsNot is the fail-closed pin. A
// zero Layout describes no radio, and every builder and parser in this
// package asks Configured() first.
func TestLayouts_AreConfiguredAndTheZeroLayoutIsNot(t *testing.T) {
	if !Layout890().Configured() {
		t.Error("Layout890 is not Configured")
	}
	if !Layout990().Configured() {
		t.Error("Layout990 is not Configured")
	}
	if (Layout{}).Configured() {
		t.Error("the zero Layout reports Configured — it describes no radio and must fail closed")
	}
}

// TestNewLayout_RefusesTheTwoRECORDBooksByName is the CONVERSE of core/kw's
// own NewLayout refusal, and it is the door this package has to shut.
//
// core/kw's Book.valid() now says "a document this package has READ", which
// is all four books; core/kw's NewLayout tests for Book590 and Book480 BY
// NAME so that no 50-byte MR/MW layout can claim to speak an MA document.
// Nothing in that stops the mistake in this direction — an ma.Layout minted
// on Book590 would be a 40-to-57-byte MA codec claiming to speak a book whose
// memory command is MR/MW — so this constructor tests for its own two books
// by name too, and the message points back at core/kw.
func TestNewLayout_RefusesTheTwoRECORDBooksByName(t *testing.T) {
	for _, book := range []kw.Book{kw.BookUnset, kw.Book590, kw.Book480} {
		cfg := layout890Config()
		cfg.Book = book
		if _, err := newLayout(cfg); err == nil {
			t.Errorf("newLayout accepted %v — this package's record is the MA family's, not the 50-byte MR/MW one", book)
		} else if !errors.Is(err, kw.ErrLayoutInvalid) {
			t.Errorf("newLayout(%v) returned %v, want an error wrapping kw.ErrLayoutInvalid", book, err)
		} else if book != kw.BookUnset && !strings.Contains(err.Error(), "core/kw") {
			t.Errorf("newLayout(%v) said %q — the refusal must point the caller at core/kw, which is where that book's record type lives", book, err)
		}
	}
}

// TestLayout_AxesAreThisBooksOwn pins every axis of both rows against the
// chart it was read off. The citations are in layout.go at each field.
func TestLayout_AxesAreThisBooksOwn(t *testing.T) {
	tests := []struct {
		name       string
		l          Layout
		book       kw.Book
		model      string
		catID      string
		slots      []kw.SlotRange
		modeCount  int
		maxTone    uint8
		maxCTCSS   uint8
		mainSub    bool
		sampleMode byte
		sampleName string
	}{
		{
			name:  "TS-890S",
			l:     Layout890(),
			book:  kw.Book890,
			model: "TS-890S",
			catID: "024",
			slots: []kw.SlotRange{
				{Class: kw.SlotMemory, Lo: 0, Hi: 99},
				{Class: kw.SlotScan, Lo: 100, Hi: 109},
				{Class: kw.SlotExtension, Lo: 110, Hi: 119},
			},
			modeCount:  14,
			maxTone:    50,
			maxCTCSS:   49,
			mainSub:    false,
			sampleMode: 'F',
			sampleName: "AM-D",
		},
		{
			name:  "TS-990S",
			l:     Layout990(),
			book:  kw.Book990,
			model: "TS-990S",
			catID: "022",
			slots: []kw.SlotRange{
				{Class: kw.SlotMemory, Lo: 0, Hi: 99},
				{Class: kw.SlotScan, Lo: 100, Hi: 109},
				{Class: kw.SlotExtension, Lo: 110, Hi: 119},
			},
			modeCount:  22,
			maxTone:    50,
			maxCTCSS:   49,
			mainSub:    true,
			sampleMode: 'N',
			sampleName: "AM-D3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.l.Book(); got != tt.book {
				t.Errorf("Book() = %v, want %v", got, tt.book)
			}
			if got := tt.l.Model(); got != tt.model {
				t.Errorf("Model() = %q, want %q", got, tt.model)
			}
			if got := tt.l.CATID(); got != tt.catID {
				t.Errorf("CATID() = %q, want %q", got, tt.catID)
			}
			gotSlots := tt.l.Slots()
			if len(gotSlots) != len(tt.slots) {
				t.Fatalf("Slots() = %v, want %v", gotSlots, tt.slots)
			}
			for i := range gotSlots {
				if gotSlots[i] != tt.slots[i] {
					t.Errorf("Slots()[%d] = %v, want %v", i, gotSlots[i], tt.slots[i])
				}
			}
			if got := len(tt.l.ModeNames()); got != tt.modeCount {
				t.Errorf("ModeNames() has %d entries, want %d live legend values", got, tt.modeCount)
			}
			if got, ok := tt.l.ModeName(tt.sampleMode); !ok || got != tt.sampleName {
				t.Errorf("ModeName(%q) = %q, %v; want %q, true", tt.sampleMode, got, ok, tt.sampleName)
			}
			if got := tt.l.MaxToneIndex(); got != tt.maxTone {
				t.Errorf("MaxToneIndex() = %d, want %d", got, tt.maxTone)
			}
			if got := tt.l.MaxCTCSSIndex(); got != tt.maxCTCSS {
				t.Errorf("MaxCTCSSIndex() = %d, want %d", got, tt.maxCTCSS)
			}
			if got := tt.l.HasMainSub(); got != tt.mainSub {
				t.Errorf("HasMainSub() = %v, want %v", got, tt.mainSub)
			}
		})
	}
}

// TestModeLegends_OmitTheTwoUnusedValuesOnBOTHROWS is pair 1's A18b shape on
// these two charts: OM P2 prints "0: Unused" and "8: Unused" in both books
// (890:3977, 890:3985; 990:3707, 990:3715), so neither value names a mode a
// channel can be in, and a legend that named either would let this codec
// publish "Unused" as a mode.
func TestModeLegends_OmitTheTwoUnusedValuesOnBOTHROWS(t *testing.T) {
	for _, l := range []Layout{Layout890(), Layout990()} {
		for _, b := range []byte{'0', '8'} {
			if name, ok := l.ModeName(b); ok {
				t.Errorf("%s: ModeName(%q) = %q — both books print that value \"Unused\"", l.Model(), b, name)
			}
		}
	}
	for _, b := range []byte{'0', '8'} {
		cfg := layout890Config()
		cfg.ModeNames[b] = "Unused"
		if _, err := newLayout(cfg); err == nil {
			t.Errorf("newLayout accepted a legend naming %q — the constructor must refuse what the chart prints \"Unused\"", b)
		}
	}
}

// TestLayouts_DifferInBothDirections states each per-radio difference as a
// fact about TWO radios, so a value copied from one row to the other fails
// here rather than shipping.
func TestLayouts_DifferInBothDirections(t *testing.T) {
	a, b := Layout890(), Layout990()

	if a.Book() == b.Book() {
		t.Error("the two rows share a Book — they are two documents, and an \"O;\" quotes the book's own cause sentence")
	}
	if a.Model() == b.Model() {
		t.Error("the two rows share a Model")
	}
	if a.CATID() == b.CATID() {
		t.Error("the two rows share a CAT ID — 024 is printed with a model name and 022 bare, and the probe compares against this row's own")
	}
	if len(a.ModeNames()) == len(b.ModeNames()) {
		t.Error("the two mode legends are the same size — the 890S's OM P2 stops at F and the 990S's runs to N")
	}
	// The 890S's legend ends where the 990S's data-mode families begin.
	if _, ok := a.ModeName('G'); ok {
		t.Error("the TS-890S publishes a name for OM P2 = G — its legend stops at F (890:3992)")
	}
	if _, ok := b.ModeName('G'); !ok {
		t.Error("the TS-990S publishes no name for OM P2 = G — LSB-D2 is printed at 990:3723")
	}
	// The same nibble is spelt differently by the two books, which is the
	// data-mode NUMBERING the 990S has and the 890S has not.
	an, _ := a.ModeName('C')
	bn, _ := b.ModeName('C')
	if an == bn {
		t.Errorf("both rows spell OM P2 = C %q — the 890S prints LSB-D (890:3989) and the 990S LSB-D1 (990:3719)", an)
	}
	if a.HasMainSub() == b.HasMainSub() {
		t.Error("the two rows agree about Main/Sub — the 990S puts a Main/Sub pointer on MN, MV, OM, TN and CN and the 890S has one nowhere")
	}
	// The tone domains AGREE, and that agreement is pinned too: it is a
	// fact about both charts, not a value one row borrowed from the other.
	if a.MaxToneIndex() != b.MaxToneIndex() || a.MaxCTCSSIndex() != b.MaxCTCSSIndex() {
		t.Error("the two tone domains disagree — both books print TN 00-50 and CN 00-49")
	}
	// And neither row's TN ceiling may be pair 1's: those charts stop at 42.
	if a.MaxToneIndex() == 42 || b.MaxToneIndex() == 42 {
		t.Error("a tone ceiling is pair 1's 43-entry domain — this pair's charts are eight entries longer")
	}
}

// TestLayout_AccessorsCopy pins that a caller holding what these functions
// return cannot edit the radio. Every axis that is a container is checked;
// the scalars cannot be aliased.
func TestLayout_AccessorsCopy(t *testing.T) {
	l := Layout890()

	slots := l.Slots()
	if len(slots) == 0 {
		t.Fatal("Slots() is empty — this test would pass vacuously")
	}
	slots[0] = kw.SlotRange{Class: kw.SlotScan, Lo: 900, Hi: 999}
	if got := l.Slots()[0]; got != (kw.SlotRange{Class: kw.SlotMemory, Lo: 0, Hi: 99}) {
		t.Errorf("Slots() handed out its own slice: after mutation the layout reads %v", got)
	}

	modes := l.ModeNames()
	if len(modes) == 0 {
		t.Fatal("ModeNames() is empty — this test would pass vacuously")
	}
	modes['1'] = "TAMPERED"
	if got, _ := l.ModeName('1'); got == "TAMPERED" {
		t.Error("ModeNames() handed out its own map")
	}

	// EXItems is copied by the same rule. The inventory is empty until its
	// transcription lands, so the copy behaviour is pinned by
	// kw.TestCopyEXItems_ReturnsAnIndependentSlice, over real data; what is
	// pinned here is that the accessor goes through kw.CopyEXItems at all,
	// which an aliasing implementation would fail on a non-empty layout.
	fixture := testLayoutWithEXItems(t, []kw.EXItem{{Addr: kw.EXAddress{P1: 0, P2: 3, P3: 1}, Name: "fixture", Digits: 3}})
	items := fixture.EXItems()
	items[0].Name = "TAMPERED"
	if got := fixture.EXItems()[0].Name; got != "fixture" {
		t.Errorf("EXItems() handed out its own slice: after mutation the layout reads %q", got)
	}
}

// TestNewLayout_RefusesEveryMissingAxis is the "every axis is explicit with a
// refusing default" half of the fail-closed rule. A zero value that merely
// disabled a check would be a weaker layout reached by forgetting something.
func TestNewLayout_RefusesEveryMissingAxis(t *testing.T) {
	tests := []struct {
		name  string
		spoil func(*layoutConfig)
	}{
		{"no model", func(c *layoutConfig) { c.Model = "" }},
		{"no CAT ID", func(c *layoutConfig) { c.CATID = "" }},
		{"a CAT ID that is not three digits", func(c *layoutConfig) { c.CATID = "24" }},
		{"a CAT ID that is not digits", func(c *layoutConfig) { c.CATID = "02X" }},
		{"no slot classes", func(c *layoutConfig) { c.Slots = nil }},
		{"a slot class with no class", func(c *layoutConfig) {
			c.Slots = []kw.SlotRange{{Lo: 0, Hi: 99}}
		}},
		{"a slot range that runs backwards", func(c *layoutConfig) {
			c.Slots = []kw.SlotRange{{Class: kw.SlotMemory, Lo: 99, Hi: 0}}
		}},
		{"two slot ranges that overlap", func(c *layoutConfig) {
			c.Slots = []kw.SlotRange{
				{Class: kw.SlotMemory, Lo: 0, Hi: 99},
				{Class: kw.SlotScan, Lo: 99, Hi: 109},
			}
		}},
		{"slot ranges out of order", func(c *layoutConfig) {
			c.Slots = []kw.SlotRange{
				{Class: kw.SlotScan, Lo: 100, Hi: 109},
				{Class: kw.SlotMemory, Lo: 0, Hi: 99},
			}
		}},
		{"a negative slot number", func(c *layoutConfig) {
			c.Slots = []kw.SlotRange{{Class: kw.SlotMemory, Lo: -1, Hi: 99}}
		}},
		{"no mode legend", func(c *layoutConfig) { c.ModeNames = nil }},
		{"an empty mode legend", func(c *layoutConfig) { c.ModeNames = map[byte]string{} }},
		{"a mode legend entry with no name", func(c *layoutConfig) { c.ModeNames['1'] = "" }},
		{"a mode legend value outside the printed alphabet", func(c *layoutConfig) { c.ModeNames['!'] = "Bang" }},
		{"no tone ceiling", func(c *layoutConfig) { c.MaxToneIndex = 0 }},
		{"no CTCSS ceiling", func(c *layoutConfig) { c.MaxCTCSSIndex = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := layout890Config()
			tt.spoil(&cfg)
			l, err := newLayout(cfg)
			if err == nil {
				t.Fatalf("newLayout accepted a config with %s", tt.name)
			}
			if !errors.Is(err, kw.ErrLayoutInvalid) {
				t.Errorf("newLayout returned %v, want an error wrapping kw.ErrLayoutInvalid", err)
			}
			if l.Configured() {
				t.Error("newLayout returned a Configured layout alongside its error")
			}
		})
	}
}

// TestNewLayout_AcceptsAnEmptyInventory is the bootstrap's own pin, and it
// is deliberate rather than a gap: mustNewLayout runs at initialisation, so
// refusing an empty inventory would stop the package compiling until its
// transcription lands. An empty membership set refuses every EX address,
// which is the closed direction; the staleness tests and the cross-check are
// what stop a bootstrap file shipping.
func TestNewLayout_AcceptsAnEmptyInventory(t *testing.T) {
	cfg := layout890Config()
	cfg.EXItems = nil
	l, err := newLayout(cfg)
	if err != nil {
		t.Fatalf("newLayout refused an empty inventory: %v", err)
	}
	if _, ok := l.EXItem(kw.EXAddress{P1: 0, P2: 0, P3: 0}); ok {
		t.Error("an empty inventory reported a member — the closed direction is that it has none")
	}
}

// TestLayout_EXItemIsMembershipOverTheWholeTriple is the check BuildEXRead,
// ParseEXAnswer and the outbound gate all ask. Both charts are sparse, so a
// scalar bound would admit an address no chart prints.
func TestLayout_EXItemIsMembershipOverTheWholeTriple(t *testing.T) {
	want := kw.EXItem{Addr: kw.EXAddress{P1: 0, P2: 3, P3: 1}, Name: "fixture", Digits: 3}
	l := testLayoutWithEXItems(t, []kw.EXItem{want})

	got, ok := l.EXItem(want.Addr)
	if !ok {
		t.Fatalf("EXItem(%v) reported no member, want the fixture row", want.Addr)
	}
	if got != want {
		t.Errorf("EXItem(%v) = %+v, want %+v", want.Addr, got, want)
	}

	// Every component is part of the key: a neighbour on any one of the
	// three is a different menu entry, and on a sparse chart it may not
	// exist at all.
	for _, miss := range []kw.EXAddress{
		{P1: 1, P2: 3, P3: 1},
		{P1: 0, P2: 4, P3: 1},
		{P1: 0, P2: 3, P3: 2},
	} {
		if _, ok := l.EXItem(miss); ok {
			t.Errorf("EXItem(%v) reported a member — the whole (P1,P2,P3) triple is the key", miss)
		}
	}

	// A zero Layout is a member of nothing.
	if _, ok := (Layout{}).EXItem(want.Addr); ok {
		t.Error("the zero Layout reported an EX member")
	}
}

// TestNewFramingFor_RefusesAnUnconfiguredLayout keeps this constructor's own
// refusal here rather than leaning on kw.NewFramingWithGate's: a zero
// Layout's AllowedCommand is a perfectly non-nil method value and the
// constructor below cannot tell it from a real gate.
func TestNewFramingFor_RefusesAnUnconfiguredLayout(t *testing.T) {
	f, err := NewFramingFor(Layout{})
	if err == nil {
		t.Fatal("NewFramingFor accepted an unconfigured layout")
	}
	if !errors.Is(err, kw.ErrLayoutInvalid) {
		t.Errorf("NewFramingFor returned %v, want an error wrapping kw.ErrLayoutInvalid", err)
	}
	if f != nil {
		t.Errorf("NewFramingFor returned a non-nil framing alongside its error: %v", f)
	}
}

// TestNewFramingFor_BuildsForBothRows pins that each row's framing carries
// that row's own book, which is what decides the cause sentence an "E;" or
// an "O;" closes the session with.
func TestNewFramingFor_BuildsForBothRows(t *testing.T) {
	seen := map[string]bool{}
	for _, tt := range []struct {
		l    Layout
		cite string
	}{
		{Layout890(), "890:"},
		{Layout990(), "990:"},
	} {
		l := tt.l
		f, err := NewFramingFor(l)
		if err != nil {
			t.Fatalf("%s: NewFramingFor: %v", l.Model(), err)
		}
		if f == nil {
			t.Fatalf("%s: NewFramingFor returned a nil framing and no error", l.Model())
		}
		ff, ok := f.(transport.FatalFramer)
		if !ok {
			t.Fatalf("%s: the framing does not implement transport.FatalFramer — \"E;\" and \"O;\" would not end the stream", l.Model())
		}
		var se *kw.StreamError
		if err := ff.IsFatal([]byte("O;")); !errors.As(err, &se) {
			t.Fatalf("%s: IsFatal(\"O;\") = %v, want a *kw.StreamError", l.Model(), err)
		}
		if !strings.Contains(se.Error(), tt.cite) {
			t.Errorf("%s: the stream error quotes %q, which cites no %s line — an \"O;\" must carry THIS book's own cause sentence", l.Model(), se.Error(), tt.cite)
		}
		if seen[se.Error()] {
			t.Errorf("%s: both rows' stream errors are the same string — the two books are two documents", l.Model())
		}
		seen[se.Error()] = true
	}
}

// testLayoutWithEXItems mints a layout carrying items, so that the
// membership path has a positive control while both real inventories are
// still bootstrap-empty.
func testLayoutWithEXItems(t *testing.T, items []kw.EXItem) Layout {
	t.Helper()
	cfg := layout890Config()
	cfg.EXItems = items
	l, err := newLayout(cfg)
	if err != nil {
		t.Fatalf("newLayout: %v", err)
	}
	return l
}

// TestLayout890Config_IsAcceptedUnspoiled is the refusal table's positive
// control: without it every row above could pass on a config that was
// invalid to begin with.
func TestLayout890Config_IsAcceptedUnspoiled(t *testing.T) {
	if _, err := newLayout(layout890Config()); err != nil {
		t.Fatalf("the unspoiled TS-890S config was refused: %v", err)
	}
}

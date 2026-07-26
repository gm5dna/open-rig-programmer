// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// TestFT710SettingsDescriptor_StaticEqualsSession pins the FT-710
// descriptor's shape against the M8a generated inventory it is built
// from: 296 items (cat.Dialect.EXItems' documented count), exactly 5 menus (P1 in
// {01,02,03,04,06} — no P1=05, see cat.KnownEXAddress's doc comment) and
// 21 groups (internal/fakeradio's own independently-derived exGroups has
// the identical count, ex.go), every item ID six digits, and a literal
// spot-check on the very first inventory row.
func TestFT710SettingsDescriptor_StaticEqualsSession(t *testing.T) {
	static := SettingsDescriptor()
	_, sess := openSession(t, Simulated)
	fromSession := sess.SettingsDescriptor()

	if static.Version != fromSession.Version {
		t.Errorf("static.Version = %q, session.Version = %q, want equal", static.Version, fromSession.Version)
	}
	if len(static.Menus) != len(fromSession.Menus) {
		t.Fatalf("static has %d menus, session has %d, want equal", len(static.Menus), len(fromSession.Menus))
	}
	for i := range static.Menus {
		if static.Menus[i].ID != fromSession.Menus[i].ID {
			t.Errorf("menu[%d].ID: static=%q session=%q", i, static.Menus[i].ID, fromSession.Menus[i].ID)
		}
	}

	if static.Version != "ft710-ex@1" {
		t.Errorf("Version = %q, want \"ft710-ex@1\" (minted here, task 33's MenuSnapshot.Descriptor carries it verbatim)", static.Version)
	}

	if err := static.Validate(); err != nil {
		t.Errorf("static SettingsDescriptor().Validate() = %v, want nil", err)
	}

	wantItems := len(cat.FT710.EXItems())
	if wantItems != 296 {
		t.Fatalf("cat.FT710.EXItems() = %d items, want 296 (test's own assumption is stale)", wantItems)
	}

	var gotItems, gotGroups int
	itemIDs := make(map[string]bool)
	for _, m := range static.Menus {
		gotGroups += len(m.Groups)
		for _, g := range m.Groups {
			gotItems += len(g.Items)
			for _, it := range g.Items {
				if len(it.ID) != 6 {
					t.Errorf("item ID %q has length %d, want 6", it.ID, len(it.ID))
				}
				itemIDs[it.ID] = true
			}
		}
	}
	if gotItems != wantItems {
		t.Errorf("total items = %d, want %d (== len(cat.FT710.EXItems()))", gotItems, wantItems)
	}
	if len(itemIDs) != wantItems {
		t.Errorf("distinct item IDs = %d, want %d — item IDs must be globally unique", len(itemIDs), wantItems)
	}
	if len(static.Menus) != 5 {
		t.Errorf("menu count = %d, want 5 (P1 in {01,02,03,04,06})", len(static.Menus))
	}
	if gotGroups != 21 {
		t.Errorf("group count = %d, want 21 (matches fakeradio's independently-derived exGroups count)", gotGroups)
	}

	// Spot-check: the very first Table 2 row, "010101" AF TREBLE GAIN,
	// under menu "01" / group "0101", Display "01-01-01".
	if len(static.Menus) == 0 || static.Menus[0].ID != "01" {
		t.Fatalf("Menus[0].ID = %q, want \"01\"", firstMenuID(static))
	}
	menu01 := static.Menus[0]
	if len(menu01.Groups) == 0 || menu01.Groups[0].ID != "0101" {
		t.Fatalf("Menus[0].Groups[0].ID = %q, want \"0101\"", firstGroupID(menu01))
	}
	group0101 := menu01.Groups[0]
	if len(group0101.Items) == 0 {
		t.Fatal("Menus[0].Groups[0].Items is empty")
	}
	first := group0101.Items[0]
	if first.ID != "010101" {
		t.Errorf("first item ID = %q, want \"010101\"", first.ID)
	}
	if first.Display != "01-01-01" {
		t.Errorf("first item Display = %q, want \"01-01-01\"", first.Display)
	}
}

func firstMenuID(d driver.SettingsDescriptor) string {
	if len(d.Menus) == 0 {
		return ""
	}
	return d.Menus[0].ID
}

func firstGroupID(m driver.SettingMenu) string {
	if len(m.Groups) == 0 {
		return ""
	}
	return m.Groups[0].ID
}

// TestSession_ReadSetting_KnownValue_Sim seeds one EX address's raw P4 via
// fakeradio.WithEXSetting and confirms ReadSetting returns it Known,
// verbatim.
func TestSession_ReadSetting_KnownValue_Sim(t *testing.T) {
	const addr = "010101" // AF TREBLE GAIN, a 3-digit numeric item
	_, sess := openSession(t, Simulated, fakeradio.WithEXSetting(addr, "123"))

	got, err := sess.ReadSetting(testCtx(t), addr)
	if err != nil {
		t.Fatalf("ReadSetting(%q): unexpected error: %v", addr, err)
	}
	want := driver.SettingValue{ID: addr, Raw: "123", State: driver.SettingKnown}
	if got != want {
		t.Errorf("ReadSetting(%q) = %+v, want %+v", addr, got, want)
	}
}

// TestSession_ReadSetting_Unavailable_Sim forces a known-inventory address
// to answer "?;" via fakeradio.WithEXUnavailable and confirms ReadSetting
// maps it to SettingUnavailable with an empty Raw and NO error — the "?;"
// -> empty-result rule, mirroring ReadChannel's empty-slot mapping.
func TestSession_ReadSetting_Unavailable_Sim(t *testing.T) {
	const addr = "010101"
	_, sess := openSession(t, Simulated, fakeradio.WithEXUnavailable(addr))

	got, err := sess.ReadSetting(testCtx(t), addr)
	if err != nil {
		t.Fatalf("ReadSetting(%q): unexpected error: %v, want nil (a \"?;\" rejection is a recorded fact, not an error)", addr, err)
	}
	if got.State != driver.SettingUnavailable {
		t.Errorf("ReadSetting(%q).State = %v, want SettingUnavailable", addr, got.State)
	}
	if got.Raw != "" {
		t.Errorf("ReadSetting(%q).Raw = %q, want \"\" when Unavailable", addr, got.Raw)
	}
	if got.ID != addr {
		t.Errorf("ReadSetting(%q).ID = %q, want %q", addr, got.ID, addr)
	}
}

// TestSession_ReadSetting_UnknownID_RefusedBeforeWire proves an id that
// does not parse to a known EX address is refused with
// *UnknownSettingError BEFORE any wire traffic at all, using the same
// countingPort technique TestWriteChannel_RefusedBeforeWire uses
// (ft710_test.go / write_test.go).
func TestSession_ReadSetting_UnknownID_RefusedBeforeWire(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"malformed shape", "not-an-address"},
		{"well-formed but out-of-inventory (no P1=05 group)", "050101"},
		{"well-formed but out-of-inventory (P3 beyond group size)", "019901"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp, sess := openCountingSession(t, Simulated)

			baseline := cp.writes.Load()
			_, err := sess.ReadSetting(testCtx(t), tt.id)

			var unknownErr *UnknownSettingError
			if !errors.As(err, &unknownErr) {
				t.Fatalf("ReadSetting(%q) error = %v (%T), want *UnknownSettingError", tt.id, err, err)
			}
			if unknownErr.ID != tt.id {
				t.Errorf("UnknownSettingError.ID = %q, want %q", unknownErr.ID, tt.id)
			}
			if got := cp.writes.Load(); got != baseline {
				t.Errorf("refused ReadSetting(%q) produced %d wire writes, want 0 (refusal must precede ALL wire traffic)", tt.id, got-baseline)
			}
		})
	}
}

// TestParseEXResponse_Table drives the PURE parseEXResponse helper
// directly with hand-built frames — see its doc comment for why the
// wrong-address branch can ONLY be exercised this way, not through the
// real Session.ReadSetting path.
func TestParseEXResponse_Table(t *testing.T) {
	items := cat.FT710.EXItems()
	requested := items[0].Addr // "010101"
	other := items[1].Addr     // "010102" — a DIFFERENT known address

	tests := []struct {
		name         string
		frame        []byte
		want         driver.SettingValue
		wantErr      bool
		wantMismatch bool
	}{
		{
			name:  "known answer",
			frame: []byte(fmt.Sprintf("EX%s123;", requested.Wire())),
			want:  driver.SettingValue{ID: requested.Wire(), Raw: "123", State: driver.SettingKnown},
		},
		{
			name:  "rejection frame",
			frame: []byte("?;"),
			want:  driver.SettingValue{ID: requested.Wire(), State: driver.SettingUnavailable},
		},
		{
			name:         "wrong-address answer",
			frame:        []byte(fmt.Sprintf("EX%s5;", other.Wire())),
			wantErr:      true,
			wantMismatch: true,
		},
		{
			name:    "malformed frame",
			frame:   []byte("garbage;"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEXResponse(cat.FT710, requested, tt.frame)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseEXResponse: got nil error, want an error")
				}
				if tt.wantMismatch {
					var mismatch *SettingAnswerMismatchError
					if !errors.As(err, &mismatch) {
						t.Fatalf("error = %v (%T), want *SettingAnswerMismatchError", err, err)
					}
					if mismatch.Requested != requested.Wire() || mismatch.Answered != other.Wire() {
						t.Errorf("mismatch = %+v, want Requested=%q Answered=%q", mismatch, requested.Wire(), other.Wire())
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("parseEXResponse: unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("parseEXResponse = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestSession_ReadSetting_ConcurrencyWithReadChannel runs ReadSetting and
// ReadChannel concurrently against the SAME session, mirroring
// TestSession_ReadWriteChannel_ConcurrentHammer_Coherent's shape
// (concurrency_test.go): both methods hold the same s.opMu for their
// whole exchange, so this proves ReadSetting's addition introduces no
// data race (run with -race, per this project's verification step) and
// that neither method's outcome is corrupted by the other's interleaving.
func TestSession_ReadSetting_ConcurrencyWithReadChannel(t *testing.T) {
	const addr = "010101"
	_, sess := openSession(t, Simulated, fakeradio.WithEXSetting(addr, "007"))

	const (
		settingReaders = 2
		channelReaders = 2
		iterations     = 5
	)

	var wg sync.WaitGroup
	errCh := make(chan error, settingReaders+channelReaders)

	for i := 0; i < settingReaders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < iterations; n++ {
				got, err := sess.ReadSetting(testCtx(t), addr)
				if err != nil {
					errCh <- fmt.Errorf("setting reader %d iteration %d: ReadSetting: %w", i, n, err)
					return
				}
				if got.State != driver.SettingKnown || got.Raw != "007" {
					errCh <- fmt.Errorf("setting reader %d iteration %d: ReadSetting = %+v, want Known/\"007\"", i, n, got)
					return
				}
			}
		}(i)
	}

	for i := 0; i < channelReaders; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < iterations; n++ {
				ch, err := sess.ReadChannel(testCtx(t), "001")
				if err != nil {
					errCh <- fmt.Errorf("channel reader %d iteration %d: ReadChannel: %w", i, n, err)
					return
				}
				if ch.Empty() {
					errCh <- fmt.Errorf("channel reader %d iteration %d: ReadChannel returned empty, want the factory's populated M-01", i, n)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

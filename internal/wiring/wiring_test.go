// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

const testCtxTimeout = 30 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// TestNewRealDriver_HWVerifiedWriteSet is the post-M5b-flip rewrite of
// TestNewRealDriver_AllFieldsUnverified (task-11 brief §3, moved from
// cmd/rigprog/wiring_test.go by task-15's extraction). The old test
// pinned "the real wiring path can write NOTHING" — a safety assertion
// the M5b hardware trials deliberately retired (13/07/2026,
// docs/hardware-notes.md "M5b write trials"; writeTrialsComplete
// flipped with evidence). Its honest replacement, NOT a deletion: the
// driver the REAL wiring path constructs must be write-capable for
// EXACTLY the six hardware-verified fields and NOTHING else — an
// over-broad writable set here would arm writes the trials never
// verified. Real-radio writes are gated by the clone service's
// choreography (confirmation digest, firmware gate, per-slot
// write-then-verify) and internal/guards' import-graph pin, not by a
// capability veto. Asserted via realDrivers[DefaultModel] — task 39's
// model-keyed real-driver table's FT-710 entry, which is NewRealDriver
// itself and the exact constructor OpenRealSessionFor looks up — so no
// serial port is ever opened.
func TestNewRealDriver_HWVerifiedWriteSet(t *testing.T) {
	writable := map[spec.Field]bool{
		spec.FieldFrequency:  true,
		spec.FieldMode:       true,
		spec.FieldCTCSSState: true,
		spec.FieldShift:      true,
		spec.FieldTag:        true,
		spec.FieldTagDisplay: true,
	}

	d := realDrivers[DefaultModel]()
	caps := d.Capabilities()
	if len(caps.Banks) == 0 {
		t.Fatal("NewRealDriver().Capabilities() has zero banks — sanity check failed, the guard below would pass vacuously")
	}
	fieldsChecked := 0
	for _, b := range caps.Banks {
		if len(b.Fields) == 0 {
			t.Errorf("bank %s: zero fields — sanity check failed for this bank", b.ID)
		}
		for f, fs := range b.Fields {
			fieldsChecked++
			if got, want := fs.CanWrite(), writable[f]; got != want {
				t.Errorf("bank %s field %s: CanWrite()==%v, want %v — the real wiring path must be write-capable for exactly the M5b-verified field set", b.ID, f, got, want)
			}
		}
	}
	if fieldsChecked == 0 {
		t.Fatal("examined zero fields — sanity check failed, the guard above would pass vacuously")
	}
	// The clarifier must be Inert specifically (transmitted but ignored,
	// HW-CONFIRMED) — not merely "not writable".
	if fs := caps.FieldSupport(spec.BankMemory, spec.FieldClarifier); fs.Write != spec.Inert {
		t.Errorf("MEM FieldClarifier.Write = %s, want Inert", fs.Write)
	}
}

// TestOpenFakeSessionFor_DefaultModel exercises the fake wiring path
// end-to-end at DefaultModel against the default (ImageUK) fakeradio
// image, confirming it yields a working driver.Session and that closeAll
// releases both the session and the fakeradio cleanly.
func TestOpenFakeSessionFor_DefaultModel(t *testing.T) {
	sess, closeAll, err := OpenFakeSessionFor(testCtx(t), DefaultModel)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor: unexpected error: %v", err)
	}
	if sess == nil {
		t.Fatal("OpenFakeSessionFor: nil session with nil error")
	}
	id := sess.Identity()
	if id.CATID != "0800" {
		t.Errorf("Identity().CATID = %q, want %q", id.CATID, "0800")
	}
	region, ok := sess.(driver.RegionReporter)
	if !ok {
		t.Fatal("session does not implement driver.RegionReporter — sanity check failed")
	}
	if got := region.Region(); got != "no-60m" {
		t.Errorf("Region() = %q, want %q (default fakeradio image is ImageUK, HW-CONFIRMED 2026-07-13 to have no 5xx bank)", got, "no-60m")
	}
	if err := closeAll(); err != nil {
		t.Errorf("closeAll: unexpected error: %v", err)
	}
}

// TestOpenRealSessionFor_BadPort confirms the real wiring path surfaces a
// port-open failure as a plain error (not a panic), for a path that
// cannot possibly exist.
func TestOpenRealSessionFor_BadPort(t *testing.T) {
	sess, closeAll, err := OpenRealSessionFor(testCtx(t), DefaultModel, "/dev/nonexistent-rigprog-test-port")
	if err == nil {
		t.Fatal("OpenRealSessionFor: expected an error opening a nonexistent port, got nil")
	}
	if sess != nil || closeAll != nil {
		t.Errorf("OpenRealSessionFor: expected nil session/closeAll on error, got sess=%v closeAllIsNil=%v", sess, closeAll == nil)
	}
}

// TestOpenRealSessionFor_UnknownModel confirms an unrecognised model fails
// with a typed *UnknownModelError BEFORE any port is touched — the error
// must name the supported list, not merely "unknown".
func TestOpenRealSessionFor_UnknownModel(t *testing.T) {
	sess, closeAll, err := OpenRealSessionFor(testCtx(t), "FT-NONEXISTENT", "/dev/nonexistent-rigprog-test-port")
	if sess != nil || closeAll != nil {
		t.Errorf("OpenRealSessionFor(unknown model): expected nil session/closeAll, got sess=%v closeAllIsNil=%v", sess, closeAll == nil)
	}
	var unknownErr *UnknownModelError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("OpenRealSessionFor(unknown model): err = %v, want *UnknownModelError", err)
	}
	if unknownErr.Model != "FT-NONEXISTENT" {
		t.Errorf("UnknownModelError.Model = %q, want %q", unknownErr.Model, "FT-NONEXISTENT")
	}
	if !reflect.DeepEqual(unknownErr.Supported, SupportedModels()) {
		t.Errorf("UnknownModelError.Supported = %v, want %v", unknownErr.Supported, SupportedModels())
	}
}

// TestSupportedModels_SortedNonEmpty pins SupportedModels' two structural
// guarantees: sorted order (so output is deterministic for a CLI listing
// or GUI picker) and non-empty (the FT-710 entry is always present).
func TestSupportedModels_SortedNonEmpty(t *testing.T) {
	got := SupportedModels()
	if len(got) == 0 {
		t.Fatal("SupportedModels() returned an empty slice, want at least DefaultModel")
	}
	if !sort.StringsAreSorted(got) {
		t.Errorf("SupportedModels() = %v, want sorted", got)
	}
	found := false
	for _, m := range got {
		if m == DefaultModel {
			found = true
		}
	}
	if !found {
		t.Errorf("SupportedModels() = %v, want it to contain DefaultModel %q", got, DefaultModel)
	}
}

// TestStaticCapabilities_FT710EqualsDriver pins StaticCapabilities'
// equivalence to the table's own constructor: for DefaultModel, it must
// return exactly what realDrivers[DefaultModel]().Capabilities() (i.e.
// NewRealDriver().Capabilities()) reports.
func TestStaticCapabilities_FT710EqualsDriver(t *testing.T) {
	got, err := StaticCapabilities(DefaultModel)
	if err != nil {
		t.Fatalf("StaticCapabilities(%q): unexpected error: %v", DefaultModel, err)
	}
	want := NewRealDriver().Capabilities()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StaticCapabilities(%q) != NewRealDriver().Capabilities()", DefaultModel)
	}
}

// TestStaticCapabilities_UnknownModel confirms an unrecognised model fails
// with a typed *UnknownModelError rather than a zero-value success.
func TestStaticCapabilities_UnknownModel(t *testing.T) {
	_, err := StaticCapabilities("FT-NONEXISTENT")
	var unknownErr *UnknownModelError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("StaticCapabilities(unknown model): err = %v, want *UnknownModelError", err)
	}
	if unknownErr.Model != "FT-NONEXISTENT" {
		t.Errorf("UnknownModelError.Model = %q, want %q", unknownErr.Model, "FT-NONEXISTENT")
	}
}

// TestStaticSettingsDescriptor_FT710 pins StaticSettingsDescriptor's
// equivalence to the table's own driver: for DefaultModel it must report
// present=true and the exact tree
// NewRealDriver().(driver.StaticSettingsProvider).StaticSettingsDescriptor()
// returns — the FT-710 driver implements the optional capability
// unconditionally of profile (task 37).
func TestStaticSettingsDescriptor_FT710(t *testing.T) {
	got, ok, err := StaticSettingsDescriptor(DefaultModel)
	if err != nil {
		t.Fatalf("StaticSettingsDescriptor(%q): unexpected error: %v", DefaultModel, err)
	}
	if !ok {
		t.Fatalf("StaticSettingsDescriptor(%q): ok = false, want true (the FT-710 driver implements driver.StaticSettingsProvider)", DefaultModel)
	}
	provider, providerOK := NewRealDriver().(driver.StaticSettingsProvider)
	if !providerOK {
		t.Fatal("NewRealDriver() does not implement driver.StaticSettingsProvider — sanity check failed")
	}
	want := provider.StaticSettingsDescriptor()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("StaticSettingsDescriptor(%q) != driver's own StaticSettingsDescriptor()", DefaultModel)
	}
}

// TestStaticSettingsDescriptor_UnknownModel confirms an unrecognised model
// fails with a typed *UnknownModelError, distinct from the ok=false case a
// known-but-non-implementing model would report.
func TestStaticSettingsDescriptor_UnknownModel(t *testing.T) {
	_, ok, err := StaticSettingsDescriptor("FT-NONEXISTENT")
	if ok {
		t.Error("StaticSettingsDescriptor(unknown model): ok = true, want false")
	}
	var unknownErr *UnknownModelError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("StaticSettingsDescriptor(unknown model): err = %v, want *UnknownModelError", err)
	}
}

// TestSynthesiseDiscoveredBanks_UnknownModelFalse: an unrecognised model
// returns (nil, false) — SynthesiseDiscoveredBanks' signature carries no
// error return, so this is how it reports "no classification happened"
// for a model this package does not support at all.
func TestSynthesiseDiscoveredBanks_UnknownModelFalse(t *testing.T) {
	banks, ok := SynthesiseDiscoveredBanks("FT-NONEXISTENT", []string{"501", "502", "EMG"})
	if ok {
		t.Error("SynthesiseDiscoveredBanks(unknown model): ok = true, want false")
	}
	if banks != nil {
		t.Errorf("SynthesiseDiscoveredBanks(unknown model): banks = %#v, want nil", banks)
	}
}

// TestSynthesiseDiscoveredBanks_FT710MatchesDriver pins
// SynthesiseDiscoveredBanks' equivalence to the table's own driver: for
// DefaultModel it must report ok=true and the exact banks
// NewRealDriver().(driver.DiscoveredBankSynthesizer).SynthesiseDiscoveredBanks
// returns for the same slot list (the same fixture
// core/driver/ft710's TestSynthesiseDiscoveredBanks_MatchesLiveDiscovery
// uses: a 60m pair, EMG, and one unclassifiable slot).
func TestSynthesiseDiscoveredBanks_FT710MatchesDriver(t *testing.T) {
	slots := []string{"501", "502", "EMG", "0X1"}

	got, ok := SynthesiseDiscoveredBanks(DefaultModel, slots)
	if !ok {
		t.Fatalf("SynthesiseDiscoveredBanks(%q, ...): ok = false, want true (the FT-710 driver implements driver.DiscoveredBankSynthesizer)", DefaultModel)
	}

	synth, synthOK := NewRealDriver().(driver.DiscoveredBankSynthesizer)
	if !synthOK {
		t.Fatal("NewRealDriver() does not implement driver.DiscoveredBankSynthesizer — sanity check failed")
	}
	want := synth.SynthesiseDiscoveredBanks(slots)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SynthesiseDiscoveredBanks(%q, ...) = %#v,\nwant %#v (must equal the driver's own classification)", DefaultModel, got, want)
	}
}

// TestOpenFakeSessionFor_UnknownModel confirms OpenFakeSessionFor fails
// with a typed *UnknownModelError, naming the supported list, when asked
// for a model this package does not support — BEFORE any fake rig is
// constructed (a leaked fakeradio.Radio here would hang the test process
// on t.Cleanup-less exit).
func TestOpenFakeSessionFor_UnknownModel(t *testing.T) {
	sess, closeAll, err := OpenFakeSessionFor(testCtx(t), "FT-NONEXISTENT")
	if sess != nil || closeAll != nil {
		t.Errorf("OpenFakeSessionFor(unknown model): expected nil session/closeAll, got sess=%v closeAllIsNil=%v", sess, closeAll == nil)
	}
	var unknownErr *UnknownModelError
	if !errors.As(err, &unknownErr) {
		t.Fatalf("OpenFakeSessionFor(unknown model): err = %v, want *UnknownModelError", err)
	}
	if unknownErr.Model != "FT-NONEXISTENT" {
		t.Errorf("UnknownModelError.Model = %q, want %q", unknownErr.Model, "FT-NONEXISTENT")
	}
	if !reflect.DeepEqual(unknownErr.Supported, SupportedModels()) {
		t.Errorf("UnknownModelError.Supported = %v, want %v", unknownErr.Supported, SupportedModels())
	}
}

// TestResolveSnapshotDir_Override pins the same --snapshot-dir override
// rule cmd/rigprog's own resolveSnapshotDir pins (fileio.go): given a
// non-empty override, return it verbatim. internal/wiring's copy exists
// so app/ (which cannot import cmd/rigprog, a cmd-local package) has
// somewhere shared to get this 3-line UserConfigDir rule from, per
// task-15 brief §2's Connect bullet.
func TestResolveSnapshotDir_Override(t *testing.T) {
	got, err := ResolveSnapshotDir("/tmp/some/override")
	if err != nil {
		t.Fatalf("ResolveSnapshotDir(override): unexpected error: %v", err)
	}
	if got != "/tmp/some/override" {
		t.Errorf("ResolveSnapshotDir(override) = %q, want %q", got, "/tmp/some/override")
	}
}

// TestResolveSnapshotDir_Default pins the default:
// <UserConfigDir>/rigprog/snapshots — the same default cmd/rigprog uses,
// so a GUI snapshot/journal and a CLI one land in the same place absent
// an override.
func TestResolveSnapshotDir_Default(t *testing.T) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Skipf("os.UserConfigDir unavailable in this environment: %v", err)
	}
	got, err := ResolveSnapshotDir("")
	if err != nil {
		t.Fatalf("ResolveSnapshotDir(\"\"): unexpected error: %v", err)
	}
	want := filepath.Join(cfgDir, "rigprog", "snapshots")
	if got != want {
		t.Errorf("ResolveSnapshotDir(\"\") = %q, want %q", got, want)
	}
}

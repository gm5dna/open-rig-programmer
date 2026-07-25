// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Compile-time proof that task 37's optional seam interfaces
// (core/driver/optional.go) are satisfied with NO signature change:
// *Session's existing Region/Diagnostics methods (ft710.go, predating
// this task) already have exactly the shapes RegionReporter/
// DiagnosticsReporter name, and *ft710Driver's new
// StaticSettingsDescriptor/SynthesiseDiscoveredBanks methods
// (settings.go/ft710.go) satisfy the two driver-level interfaces. Any
// signature drift here fails the BUILD, not merely a test.
var (
	_ driver.RegionReporter            = (*Session)(nil)
	_ driver.DiagnosticsReporter       = (*Session)(nil)
	_ driver.StaticSettingsProvider    = (*ft710Driver)(nil)
	_ driver.DiscoveredBankSynthesizer = (*ft710Driver)(nil)
)

// TestStaticSettingsDescriptor_MatchesPackageFunc: the driver-level
// optional capability must return exactly what the package-level
// SettingsDescriptor func (and, per its own doc comment, Session's own
// SettingsDescriptor method) already returns — this driver's settings
// tree depends only on the static EX inventory, never on anything a live
// session discovers, so all three call sites agree.
func TestStaticSettingsDescriptor_MatchesPackageFunc(t *testing.T) {
	drv := New(Simulated)
	provider, ok := drv.(driver.StaticSettingsProvider)
	if !ok {
		t.Fatal("New(Simulated) does not implement driver.StaticSettingsProvider")
	}

	got := provider.StaticSettingsDescriptor()
	want := SettingsDescriptor()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("StaticSettingsDescriptor() != package-level SettingsDescriptor()")
	}
	if err := got.Validate(); err != nil {
		t.Errorf("StaticSettingsDescriptor().Validate() = %v, want nil", err)
	}
}

// TestSynthesiseDiscoveredBanks_MatchesLiveDiscovery is the task-37
// brief's named equivalence test: classifying
// ["501","502","EMG","0X1"] must yield banks equal to
// effectiveCapabilities' own discovered banks for the SAME 60m
// slots/EMG flag, across the FULL surface (slot order, Slots, NoBlank,
// labels, every field-support value) — "0X1" is neither a static-bank
// member nor a parseable 60m/EMG wire form, so it is unclassifiable and
// omitted, never guessed into a bank.
func TestSynthesiseDiscoveredBanks_MatchesLiveDiscovery(t *testing.T) {
	drv := New(Simulated)
	synth, ok := drv.(driver.DiscoveredBankSynthesizer)
	if !ok {
		t.Fatal("New(Simulated) does not implement driver.DiscoveredBankSynthesizer")
	}

	got := synth.SynthesiseDiscoveredBanks([]string{"501", "502", "EMG", "0X1"})

	base := drv.Capabilities()
	want := effectiveCapabilities(base, []string{"501", "502"}, true).Banks[len(base.Banks):]

	if !reflect.DeepEqual(got, want) {
		t.Errorf("SynthesiseDiscoveredBanks(...) = %#v,\nwant %#v (must equal effectiveCapabilities' own discovered banks)", got, want)
	}
}

// TestSynthesiseDiscoveredBanks_UnclassifiableOmitted: a slot that is
// neither a static-bank member nor a parseable 60m/EMG wire form is
// dropped silently, never guessed into either bank.
func TestSynthesiseDiscoveredBanks_UnclassifiableOmitted(t *testing.T) {
	drv := New(Simulated)
	synth := drv.(driver.DiscoveredBankSynthesizer)

	got := synth.SynthesiseDiscoveredBanks([]string{"0X1", "garbage", "", "600"})
	if len(got) != 0 {
		t.Errorf("SynthesiseDiscoveredBanks(unclassifiable-only) = %#v, want no banks", got)
	}
}

// TestSynthesiseDiscoveredBanks_ExcludesStaticallyClaimedSlots: an
// ordinary MEM/PMS slot already claimed by this driver's own static
// Capabilities().Banks must never be reclassified into a discovered
// bank — the DiscoveredBankSynthesizer contract (core/driver/optional.go)
// only ever adds banks Open's discovery would ADD, never restates a
// static one.
func TestSynthesiseDiscoveredBanks_ExcludesStaticallyClaimedSlots(t *testing.T) {
	drv := New(Simulated)
	synth := drv.(driver.DiscoveredBankSynthesizer)

	got := synth.SynthesiseDiscoveredBanks([]string{"001", "P1L"})
	if len(got) != 0 {
		t.Errorf("SynthesiseDiscoveredBanks(statically-claimed slots only) = %#v, want no banks", got)
	}
}

// TestSynthesiseDiscoveredBanks_PreservesInputOrder: the 60M bank's
// Slots must preserve the ORDER slots appeared in the input, not a
// numerically sorted order.
func TestSynthesiseDiscoveredBanks_PreservesInputOrder(t *testing.T) {
	drv := New(Simulated)
	synth := drv.(driver.DiscoveredBankSynthesizer)

	got := synth.SynthesiseDiscoveredBanks([]string{"503", "501", "502"})

	var sixty *spec.Bank
	for i := range got {
		if got[i].ID == spec.Bank60m {
			sixty = &got[i]
		}
	}
	if sixty == nil {
		t.Fatal("SynthesiseDiscoveredBanks did not produce a 60M bank")
	}
	want := []string{"503", "501", "502"}
	if !reflect.DeepEqual(sixty.Slots, want) {
		t.Errorf("60M.Slots = %v, want %v (input order preserved, not sorted)", sixty.Slots, want)
	}
}

// TestSynthesiseDiscoveredBanks_PreservesDuplicateEMG: offline
// classification can be handed a semantically INVALID slot list (LoadFile
// validates only AFTER loading), so the same "EMG" wire form may appear
// more than once. The pre-M9a offline app synthesis preserved EVERY input
// occurrence; a live session, probing the single physical EMG slot, can
// never produce a duplicate, so this is the one case offline synthesis
// deliberately diverges from effectiveCapabilities' single-EMG bank —
// every field OTHER than Slots must still match it byte-for-byte.
func TestSynthesiseDiscoveredBanks_PreservesDuplicateEMG(t *testing.T) {
	drv := New(Simulated)
	synth := drv.(driver.DiscoveredBankSynthesizer)

	got := synth.SynthesiseDiscoveredBanks([]string{"EMG", "EMG"})

	var emgBank *spec.Bank
	for i := range got {
		if got[i].ID == spec.BankEMG {
			emgBank = &got[i]
		}
	}
	if emgBank == nil {
		t.Fatalf("SynthesiseDiscoveredBanks([EMG EMG]) produced no EMG bank; got %#v", got)
	}
	if want := []string{"EMG", "EMG"}; !reflect.DeepEqual(emgBank.Slots, want) {
		t.Errorf("EMG.Slots = %v, want %v (every input occurrence preserved, not collapsed to one)", emgBank.Slots, want)
	}

	// Everything but Slots must equal the single-EMG bank
	// effectiveCapabilities mints — only the duplicate slot list diverges.
	base := drv.Capabilities()
	single := effectiveCapabilities(base, nil, true).Banks[len(base.Banks):]
	if len(single) != 1 {
		t.Fatalf("effectiveCapabilities(single EMG) produced %d discovered banks, want 1", len(single))
	}
	if emgBank.ID != single[0].ID || emgBank.Label != single[0].Label || emgBank.NoBlank != single[0].NoBlank {
		t.Errorf("duplicate-EMG bank shape = {ID:%v Label:%q NoBlank:%v}, want {ID:%v Label:%q NoBlank:%v} (only Slots may differ)", emgBank.ID, emgBank.Label, emgBank.NoBlank, single[0].ID, single[0].Label, single[0].NoBlank)
	}
	if !reflect.DeepEqual(emgBank.Fields, single[0].Fields) {
		t.Error("duplicate-EMG bank Fields differ from the single-EMG bank's, want identical (only Slots may differ)")
	}
}

// TestSynthesiseDiscoveredBanks_NoneDiscovered: an input with nothing
// classifiable produces zero banks, never an error and never an empty
// placeholder bank.
func TestSynthesiseDiscoveredBanks_NoneDiscovered(t *testing.T) {
	drv := New(Simulated)
	synth := drv.(driver.DiscoveredBankSynthesizer)

	got := synth.SynthesiseDiscoveredBanks(nil)
	if len(got) != 0 {
		t.Errorf("SynthesiseDiscoveredBanks(nil) = %#v, want no banks", got)
	}
}

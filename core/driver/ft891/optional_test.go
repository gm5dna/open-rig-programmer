// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Compile-time proof of the seams this package satisfies. Any signature
// drift fails the BUILD, not merely a test.
//
// DiscoveredBankSynthesizer is the one that matters most here, because its
// ABSENCE fails silently: internal/wiring's synthesis helper asks for the
// interface and returns false when a driver does not satisfy it, and the
// app then renders no discovered banks for an offline FT-891 codeplug — no
// error anywhere. A compile-time assertion is the only thing that turns a
// method renamed or a receiver changed into a loud failure.
var (
	_ driver.Driver                    = (*ft891Driver)(nil)
	_ driver.Session                   = (*Session)(nil)
	_ driver.DiscoveredBankSynthesizer = (*ft891Driver)(nil)
	_ driver.DiagnosticsReporter       = (*Session)(nil)
)

// TestSession_DoesNotReportRegion pins an ABSENCE, deliberately: this
// driver does not implement driver.RegionReporter and must not acquire one
// by accident.
//
// The FT-710's Region() maps a discovered 5xx/EMG count onto a
// regulatory-region name using fingerprints that are that radio's own —
// partly hardware-confirmed, partly still assumed. No FT-891 inventory has
// ever been observed, and this radio's own legend says only "U.S. and U.K.
// version only" (layout 962), which is a statement about market rather than
// a mapping from a channel count to a region. Borrowing the FT-710's
// fingerprints would mean answering a question about one radio with another
// radio's evidence. Callers already tolerate absence (the capability is
// reached by a two-result type assertion), so codeplug.RadioInfo.Region
// simply stays unset for an FT-891.
//
// A Stage R session that enumerates a real FT-891's 5 MHz bank is what would
// make a region vocabulary possible — at which point this test is deleted
// with the same commit that adds the method, and the evidence goes in the
// commit message. It is NOT a gap to be filled meanwhile, which is exactly
// why the absence is asserted rather than merely mentioned.
func TestSession_DoesNotReportRegion(t *testing.T) {
	var sess driver.Session = &Session{}
	if rr, ok := sess.(driver.RegionReporter); ok {
		t.Errorf("*ft891.Session implements driver.RegionReporter (Region() = %q) — this driver has no honest region vocabulary; see this test's doc comment and doc.go before adding one", rr.Region())
	}
}

// TestSynthesiseDiscoveredBanks_MatchesLiveDiscovery is the live-vs-offline
// equivalence, table-driven over every input class the offline path can
// actually meet: sparse wires, input order that is not sorted order, the
// LAST declared 5xx slot, EMG alone and alongside, malformed wire forms, and
// slots a static bank already claims.
//
// The expectation is not hand-written bank literals but
// effectiveCapabilities' OWN output for the equivalent discovery — the same
// function Open calls — so this asserts the two derivations agree across the
// whole surface (label, ID, Slots, NoBlank and every field-support value)
// rather than across whichever fields a hand-written fixture happened to
// mention. SynthesiseDiscoveredBanks reuses that function internally, which
// is what makes the agreement structural; this test is what keeps it so if
// either side is ever rewritten.
//
// The companion leg, over a REAL session's discovered wires, is in
// ft891_test.go's discovery test — the one place the input is data neither
// test invented.
func TestSynthesiseDiscoveredBanks_MatchesLiveDiscovery(t *testing.T) {
	drv := New(Simulated)
	synth, ok := drv.(driver.DiscoveredBankSynthesizer)
	if !ok {
		t.Fatal("New(Simulated) does not implement driver.DiscoveredBankSynthesizer")
	}
	base := drv.Capabilities()

	for _, tt := range []struct {
		name    string
		input   []string
		want60m []string
		wantEMG bool
	}{
		{
			name:    "sparse wires, as discovery would have found them",
			input:   []string{"503", "510"},
			want60m: []string{"503", "510"},
		},
		{
			name:    "input ORDER is preserved, not sorted",
			input:   []string{"510", "503", "501"},
			want60m: []string{"510", "503", "501"},
		},
		{
			name:    "the last declared 5xx slot alone",
			input:   []string{"510"},
			want60m: []string{"510"},
		},
		{
			name:    "EMG alone",
			input:   []string{"EMG"},
			wantEMG: true,
		},
		{
			name:    "5xx and EMG together",
			input:   []string{"503", "EMG"},
			want60m: []string{"503"},
			wantEMG: true,
		},
		{
			// Unclassifiable wire forms are OMITTED, never guessed into a
			// bank: "511" is one past this radio's PRINTED ceiling (the
			// legend runs 501-510, layout 962, where the FT-710's and
			// FTdx10's say only "5xx" and their 599 is an assumption),
			// "5011" is the right prefix at the wrong width, and the empty
			// string is what a malformed file hands over.
			name:  "malformed and out-of-range wires are omitted",
			input: []string{"0X1", "garbage", "", "511", "600", "5011"},
		},
		{
			// A slot one of this driver's own static banks already claims
			// never appears in a discovered bank.
			//
			// HONEST ABOUT WHAT THIS CANNOT ATTRIBUTE: two independent
			// refusals stand in the way, and for this radio's bank shapes
			// no input can separate them. SynthesiseDiscoveredBanks
			// excludes claimed slots explicitly, AND a MEM ("001"-"099") or
			// PMS ("P1L"-"P9U") wire form classifies as neither 60m nor
			// EMG, so it would be omitted even with the exclusion deleted.
			// The case therefore pins the OUTCOME the contract promises,
			// not the mechanism. The explicit exclusion earns its keep
			// against a future static bank that claimed a 5xx or EMG wire
			// form — a shape this driver's capability data cannot currently
			// produce — where its absence would put one slot in two banks
			// and fail spec.Capabilities.Validate.
			name:  "statically-claimed slots are excluded",
			input: []string{"001", "099", "P1L", "P9U"},
		},
		{
			name:    "a realistic mixture of all five classes",
			input:   []string{"001", "503", "0X1", "EMG", "P1L", "510"},
			want60m: []string{"503", "510"},
			wantEMG: true,
		},
		{
			name:  "nil input produces no banks",
			input: nil,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := synth.SynthesiseDiscoveredBanks(tt.input)
			want := effectiveCapabilities(base, tt.want60m, tt.wantEMG).Banks[len(base.Banks):]
			if !reflect.DeepEqual(got, want) {
				t.Errorf("SynthesiseDiscoveredBanks(%v) =\n %#v\nwant (effectiveCapabilities' own discovered banks)\n %#v", tt.input, got, want)
			}
		})
	}
}

// TestSynthesiseDiscoveredBanks_DuplicateEMGCollapses pins this driver's
// choice for a semantically invalid input: a repeated "EMG" yields ONE EMG
// slot, the single physical channel.
//
// Live discovery probes one EMG slot and can never produce a duplicate, so
// collapsing keeps offline synthesis identical to what a live session
// carries — which is the property the equivalence test above depends on.
// core/driver/ft710 makes the OPPOSITE choice, preserving every occurrence,
// for a compatibility reason of its own; that is its history, not a rule
// this driver inherits. Pinned so it stays a decision rather than an
// accident of whichever loop shape was written first.
func TestSynthesiseDiscoveredBanks_DuplicateEMGCollapses(t *testing.T) {
	synth := New(Simulated).(driver.DiscoveredBankSynthesizer)

	got := synth.SynthesiseDiscoveredBanks([]string{"EMG", "EMG", "EMG"})

	var emg *spec.Bank
	for i := range got {
		if got[i].ID == spec.BankEMG {
			emg = &got[i]
		}
	}
	if emg == nil {
		t.Fatalf("SynthesiseDiscoveredBanks([EMG EMG EMG]) produced no EMG bank; got %#v", got)
	}
	if want := []string{"EMG"}; !reflect.DeepEqual(emg.Slots, want) {
		t.Errorf("EMG.Slots = %v, want %v (one physical channel, however many times the input names it)", emg.Slots, want)
	}
}

// TestSynthesiseDiscoveredBanks_NeverClaimsAWritableDiscoveredBank: the
// offline path must force every Write to Unsupported exactly as live
// discovery does — and must zero the tag pair the same way (P4) — on the
// SIMULATED profile in particular, the one whose static banks are genuinely
// write-Supported and carry a READABLE tag, so an inherited Fields map would
// advertise both a writable 5xx slot and a tag MR cannot deliver.
func TestSynthesiseDiscoveredBanks_NeverClaimsAWritableDiscoveredBank(t *testing.T) {
	synth := New(Simulated).(driver.DiscoveredBankSynthesizer)

	banks := synth.SynthesiseDiscoveredBanks([]string{"501", "EMG"})
	if len(banks) != 2 {
		t.Fatalf("SynthesiseDiscoveredBanks([501 EMG]) produced %d banks, want 2", len(banks))
	}
	for _, b := range banks {
		if !b.NoBlank {
			t.Errorf("bank %s NoBlank = false, want true", b.ID)
		}
		for _, f := range allFields {
			fs := b.Fields[f]
			if fs.Write != spec.Unsupported {
				t.Errorf("bank %s field %s: Write = %s, want Unsupported on a discovered bank", b.ID, f, fs.Write)
			}
			if (f == spec.FieldTag || f == spec.FieldTagDisplay) && fs != (spec.FieldSupport{}) {
				t.Errorf("bank %s field %s = {Read:%s Write:%s}, want the ZERO FieldSupport — MR's answer carries neither (P4, matrix §2.5)", b.ID, f, fs.Read, fs.Write)
			}
		}
	}
}

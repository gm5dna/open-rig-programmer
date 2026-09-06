// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
)

// The two slot-domain refusal sentences, byte for byte, as every TOKEN-PMS
// dialect this repository registers renders them today.
//
// They are CONSTANTS here rather than a per-dialect table because the claim
// is that all five token-PMS dialects agree: S0.2 replaced two hardcoded
// sentences with Dialect.mwSlotDomainRefusal/mtSlotDomainRefusal, composed
// from the receiver's own memory range, PMS domain and declared special
// banks, on the argument that each of them composes back to the frozen text.
// One shared constant is that argument written down; a table of five would
// let a drifting dialect be "fixed" by editing its own row.
//
// THE SIXTH REGISTERED DIALECT IS NOT AMONG THEM AND CANNOT BE: the FT-991A's
// PMS pairs are the decimal channels 100-117 and it declares neither a 5 MHz
// bank nor an emergency channel, so its composed sentences are DIFFERENT
// text by design and sharing these constants would be meaningless. Its shape
// is covered on a synthetic numeric-PMS dialect inside core/cat, by
// TestSlotDomainText_NumericPMSDialect. That is why this test's name says
// token-PMS and not "every registered dialect", which is what it said until
// the FT-991A milestone's closing review (O-M3) — the name is the claim, and
// the claim had quietly become false.
const (
	frozenMWSlotDomainRefusal = `MW: slot must be Writable() (memory 001-099 or PMS P1L-P9U; 5xx/EMG/"000" rejected)`
	frozenMTSlotDomainRefusal = `MT: slot must be memory (001-099) or PMS (P1L-P9U); 5xx/EMG rejected by project policy pending M5a, "000"/invalid rejected per reference`
)

// TestSlotDomainRefusals_EveryTokenPMSDialectIsByteIdentical pins the
// composed MW and MT slot-domain sentences for ALL FIVE token-PMS dialects.
//
// WHY IT LIVES IN core/transport. The FT-710's own sentences are pinned
// inside core/cat (TestSlotDomainText_FT710SentencesAreByteIdentical) and its
// MW render is baked into six lines of core/cat/testdata/frame-corpus.golden,
// one of the twenty paths the milestone golden gate forbids moving. The other
// four dialects live in core/cat's own subpackages, which core/cat cannot
// import, and their testdata holds MR/MT/MW/EX vectors and no refusal text at
// all — so until now the claim in mw.go and doc.go that "all four token
// dialects render byte-for-byte what stood here" was true and unpinned
// (Stage 0 close review, seat 2 LOW-2). This package's tests already import
// all three sibling dialect packages, so the pin costs no new dependency.
//
// It is deliberately a WHOLE-SENTENCE comparison of ParseError.Reason and not
// a Contains: the sentences are the thing being frozen, and a substring test
// would pass on a sentence that had grown a clause.
func TestSlotDomainRefusals_EveryTokenPMSDialectIsByteIdentical(t *testing.T) {
	for _, tc := range []struct {
		name string
		d    cat.Dialect
	}{
		{"FT-710", cat.FT710},
		{"FTdx10", ftdx10.Dialect()},
		{"FTdx101D", ftdx101.DialectD()},
		{"FTdx101MP", ftdx101.DialectMP()},
		{"FT-891", ft891.Dialect()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The none form is the slot every one of these dialects refuses
			// at both builders while still PARSING it, so the refusal under
			// test is the slot-domain one and not a parse failure upstream.
			none, err := tc.d.ParseSlot("000")
			if err != nil {
				t.Fatalf("ParseSlot(%q): unexpected error: %v", "000", err)
			}

			_, err = tc.d.BuildMWSet(cat.MemoryData{Slot: none})
			if got := slotRefusalReason(t, err, "BuildMWSet"); got != frozenMWSlotDomainRefusal {
				t.Errorf("BuildMWSet slot refusal =\n\t%q\nwant\n\t%q", got, frozenMWSlotDomainRefusal)
			}

			// The MT sentence is reached through the builder THIS dialect's
			// form and P11 policy admit; the other two refuse on the form or
			// the policy before the slot is ever examined, which would pin
			// nothing.
			combined := cat.MemoryData{Slot: none, Kind: cat.CombinedMTSetKind}
			switch {
			case tc.d.MTForm() == cat.MTFormShort:
				_, err = tc.d.BuildMTSet(none, false, "AB")
			case tc.d.MTP11() == cat.P11TagDisplay:
				_, err = tc.d.BuildMTSetCombinedDisplay(combined, "AB", false)
			default:
				_, err = tc.d.BuildMTSetCombined(combined, "AB")
			}
			if got := slotRefusalReason(t, err, "the MT Set builder"); got != frozenMTSlotDomainRefusal {
				t.Errorf("MT Set slot refusal =\n\t%q\nwant\n\t%q", got, frozenMTSlotDomainRefusal)
			}
		})
	}
}

// slotRefusalReason unwraps the *cat.ParseError a builder returns and answers
// its Reason — the sentence itself, without the "cat: parse error: … (input=…)"
// wrapper Error() adds, so the pin compares what the renderer composed and
// nothing else.
func slotRefusalReason(t *testing.T, err error, what string) string {
	t.Helper()
	if err == nil {
		t.Fatalf("%s accepted the none slot", what)
	}
	var pe *cat.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("%s refused with %T (%v), want *cat.ParseError", what, err, err)
	}
	return pe.Reason
}

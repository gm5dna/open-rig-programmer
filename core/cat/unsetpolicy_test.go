// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// This file is the S0-close review's HIGH-1 fix: every Stage 0 policy axis
// that reads through a plain classify-then-compare (rather than a switch
// whose default refuses) let a ZERO policy fall to whichever reading the
// classify step reached first, memory/PMS included. mcpolicy_test.go,
// memoryp5_test.go and mtp11_test.go each already carry a defense-in-depth
// test for their own axis's BUILDER; this file is the one place that walks
// EVERY axis — MCSelects, MT.ReadSlots, MT.P11 and, for completeness,
// MemoryP5 — through the BUILDER, the OUTBOUND GATE and, where the axis has
// one, the PARSER, on a dialect built by copying a valid, registered-shape
// fixture and zeroing the unexported field directly. That mirrors
// TestMTP11_ZeroPolicyRefusesRatherThanDefaultingWide's (mtp11_test.go)
// reason for doing the same: a zero policy is impossible past NewDialect's
// V9/V13/V14 clauses (dialectvalidate.go), so this is the only way to reach
// the code those clauses cannot.
//
// A new file rather than an appendix to mcpolicy_test.go/mtp11_test.go/
// memoryp5_test.go, for the reason mcpolicy_test.go's own header gives:
// those files carry mc_test.go/mtcombined_test.go/mw_test.go's neighbouring
// golden-literal pins, and a literal added anywhere in a pinned file
// renumbers every one after it.

// TestUnsetPolicy_MCSelects_RefusesEverySlotAtBuilderAndGate is HIGH-1's
// MC arm: before the fix, mcSendValid returned true for memory and PMS
// slots UNCONDITIONALLY — a zero MCSlotPolicy never reached the switch on
// the policy at all, so "MC001;" both built and passed the gate.
func TestUnsetPolicy_MCSelects_RefusesEverySlotAtBuilderAndGate(t *testing.T) {
	base := combinedDialect // MCSelectsAll: builds every slot class below
	zeroed := base
	zeroed.slots.mcSelects = 0

	mem, err := base.MemorySlot(1)
	if err != nil {
		t.Fatalf("fixture broken: MemorySlot(1): %v", err)
	}
	sixty, err := base.SixtyMSlot(1)
	if err != nil {
		t.Fatalf("fixture broken: SixtyMSlot(1): %v", err)
	}

	for _, s := range []Slot{mem, sixty} {
		// BUILDER: a zero MCSlotPolicy must refuse this slot even though
		// it is memory or PMS — the class the pre-fix code admitted
		// unconditionally, before ever consulting the policy.
		if cmd, err := zeroed.BuildMCSet(s); err == nil {
			t.Errorf("zeroed.BuildMCSet(%q) succeeded, emitting %q — a zero MCSlotPolicy must refuse EVERY slot, memory and PMS included, not fall to the narrow (memory/PMS) reading", s.Wire(), cmd.Bytes())
		} else if !strings.Contains(err.Error(), "MC") {
			t.Errorf("zeroed.BuildMCSet(%q) refused with %q, which does not mention MC", s.Wire(), err)
		} else if !strings.Contains(err.Error(), "policy unset") {
			// THE WORDING IS PART OF THE REFUSAL. The declared-narrow arm
			// says "memory and PMS only", which is a claim about a dialect's
			// printed MC legend; a policy that declares nothing has no
			// legend to make that claim from, so this arm must take the
			// P5/P11 sites' "policy unset — refusing to guess" wording
			// instead. Without this assertion the two arms could quietly
			// collapse back into one message asserting the narrow reading of
			// a dialect that stated none.
			t.Errorf("zeroed.BuildMCSet(%q) refused with %q, which does not say the policy is unset — it must not assert the narrow (memory/PMS) reading of a dialect that declares no MC domain", s.Wire(), err)
		}

		// GATE: the same wire form, built by the WIDE dialect (so the
		// frame itself is unimpeachable), must be refused by the zeroed
		// dialect's gate too — the builder and the gate share
		// mcSendValid, and this proves the sharing still holds after the
		// fix.
		cmd, err := base.BuildMCSet(s)
		if err != nil {
			t.Fatalf("fixture broken: base.BuildMCSet(%q): %v", s.Wire(), err)
		}
		frame := cmd.Bytes()
		if zeroed.AllowedCommand(frame) {
			t.Errorf("zeroed's gate ADMITTED %q — a zero MCSlotPolicy must refuse at the gate too, not just the builder", frame)
		}

		// POSITIVE CONTROL: base itself, unzeroed, still builds and admits
		// this slot — so the refusal above is the zeroed field's, not some
		// other property of the fixture or the slot.
		if !base.AllowedCommand(frame) {
			t.Errorf("base's own gate refused its own builder's %q — the fixture is broken, not the policy under test", frame)
		}
	}
}

// TestUnsetPolicy_MTReadSlots_RefusesAtBuilderAndGate is HIGH-1's MT-read
// arm: before the fix, mtReadSlotValid returned true for memory and PMS
// slots UNCONDITIONALLY, the same shape as mcSendValid's bug.
func TestUnsetPolicy_MTReadSlots_RefusesAtBuilderAndGate(t *testing.T) {
	base := combinedDialect // MTReadsReadable
	zeroed := base
	zeroed.mt.ReadSlots = 0

	mem, err := base.MemorySlot(1)
	if err != nil {
		t.Fatalf("fixture broken: MemorySlot(1): %v", err)
	}

	if cmd, err := zeroed.BuildMTRead(mem); err == nil {
		t.Errorf("zeroed.BuildMTRead(%q) succeeded, emitting %q — a zero MTReadSlotPolicy must refuse EVERY slot, memory and PMS included, not fall to the narrow (memory/PMS) reading", mem.Wire(), cmd.Bytes())
	} else if !strings.Contains(err.Error(), "MT") {
		t.Errorf("zeroed.BuildMTRead(%q) refused with %q, which does not mention MT", mem.Wire(), err)
	} else if !strings.Contains(err.Error(), "policy unset") {
		// See the MC case above: the declared-narrow wording asserts
		// "memory and PMS only" of a printed MT slot legend, and an unset
		// policy has none to assert.
		t.Errorf("zeroed.BuildMTRead(%q) refused with %q, which does not say the policy is unset — it must not assert the narrow (memory/PMS) reading of a dialect that declares no MT read domain", mem.Wire(), err)
	}

	cmd, err := base.BuildMTRead(mem)
	if err != nil {
		t.Fatalf("fixture broken: base.BuildMTRead(%q): %v", mem.Wire(), err)
	}
	frame := cmd.Bytes()
	if zeroed.AllowedCommand(frame) {
		t.Errorf("zeroed's gate ADMITTED %q — a zero MTReadSlotPolicy must refuse at the gate too, not just the builder", frame)
	}
	if !base.AllowedCommand(frame) {
		t.Errorf("base's own gate refused its own builder's %q — the fixture is broken, not the policy under test", frame)
	}
}

// TestUnsetPolicy_MTP11_RefusesAtBuilderParserAndGate is HIGH-1's P11 arm,
// and the one Codex's review named directly: p11Valid's pre-fix if/else
// read "anything that is not P11TagDisplay" as P11Fixed, so a zero
// MTP11Policy took the P11Fixed reading and admitted a combined answer
// whose byte 28 happened to be combinedMTP11 ('0') — exactly the byte
// BuildMTSetCombined writes under P11Fixed, which is why the frame this
// test parses is built from a P11Fixed fixture: it is the frame the
// pre-fix bug actually accepted.
//
// mtp11_test.go's TestMTP11_ZeroPolicyRefusesRatherThanDefaultingWide
// covers the BUILDER half of this same axis already; this is the
// PARSER-and-GATE half Codex's review asked for (also added directly to
// that test, below).
func TestUnsetPolicy_MTP11_RefusesAtBuilderParserAndGate(t *testing.T) {
	base := combinedDialect // P11Fixed
	zeroed := base
	zeroed.mt.P11 = 0

	rec := p11TestRecord(t, base)

	// BUILDER: both combined-form APIs must refuse.
	if cmd, err := zeroed.BuildMTSetCombined(rec, "AB"); err == nil {
		t.Errorf("zeroed.BuildMTSetCombined succeeded, emitting %q — a zero MTP11Policy must refuse, not default to the P11Fixed reading", cmd.Bytes())
	} else if !strings.Contains(err.Error(), "P11") {
		t.Errorf("zeroed.BuildMTSetCombined refused with %q, which does not mention P11", err)
	}
	if cmd, err := zeroed.BuildMTSetCombinedDisplay(rec, "AB", true); err == nil {
		t.Errorf("zeroed.BuildMTSetCombinedDisplay succeeded, emitting %q — a zero MTP11Policy must refuse this API too", cmd.Bytes())
	} else if !strings.Contains(err.Error(), "P11") {
		t.Errorf("zeroed.BuildMTSetCombinedDisplay refused with %q, which does not mention P11", err)
	}

	// A VALID 35-byte combined frame (combinedDialect's own
	// mtCombinedLen: 29 + 6-byte tag), built by the UNZEROED P11Fixed
	// dialect, so byte 28 is combinedMTP11 ('0') — the exact byte the
	// pre-fix p11Valid's else-arm admitted for ANY non-TagDisplay policy,
	// zero included.
	cmd, err := base.BuildMTSetCombined(rec, "AB")
	if err != nil {
		t.Fatalf("fixture broken: base.BuildMTSetCombined: %v", err)
	}
	frame := cmd.Bytes()
	if got := frame[mtCombinedP11Offset]; got != combinedMTP11 {
		t.Fatalf("fixture broken: frame %q carries %q at byte 28, want the printed-fixed %q", frame, got, combinedMTP11)
	}

	// PARSERS: BOTH must refuse — the display-less one (its own form) and
	// the display-bearing one (which a zero policy is not, either).
	if m, tag, err := zeroed.ParseMTAnswerCombined(frame); err == nil {
		t.Errorf("zeroed.ParseMTAnswerCombined(%q) accepted it, returning (%+v, %q) — a zero MTP11Policy must refuse rather than read byte 28 as the printed-fixed schema byte", frame, m, tag)
	} else if !strings.Contains(err.Error(), "P11") {
		t.Errorf("zeroed.ParseMTAnswerCombined(%q) refused with %q, which does not mention P11", frame, err)
	}
	if m, tag, disp, err := zeroed.ParseMTAnswerCombinedDisplay(frame); err == nil {
		t.Errorf("zeroed.ParseMTAnswerCombinedDisplay(%q) accepted it, returning (%+v, %q, %v) — a zero MTP11Policy must refuse rather than read byte 28 as a live TAG flag", frame, m, tag, disp)
	} else if !strings.Contains(err.Error(), "P11") {
		t.Errorf("zeroed.ParseMTAnswerCombinedDisplay(%q) refused with %q, which does not mention P11", frame, err)
	}

	// GATE: the same frame, which p11Valid also gates.
	if zeroed.AllowedCommand(frame) {
		t.Errorf("zeroed's gate ADMITTED %q, whose byte 28 is the printed-fixed %q — a zero MTP11Policy must refuse at the gate too, not just the parsers", frame, combinedMTP11)
	}

	// POSITIVE CONTROL: base itself, unzeroed, still builds, parses and
	// admits this exact frame.
	if !base.AllowedCommand(frame) {
		t.Errorf("base's own gate refused its own builder's %q — the fixture is broken, not the policy under test", frame)
	}
	if _, _, err := base.ParseMTAnswerCombined(frame); err != nil {
		t.Errorf("base.ParseMTAnswerCombined(%q) = %v — it is that policy's own parser", frame, err)
	}
}

// TestUnsetPolicy_MemoryP5_RefusesAtBuilderParserAndGate is the fourth
// Stage 0 axis, MemoryP5Policy. Unlike the three above it is included for
// COMPLETENESS and VERIFICATION, not as a fix: wave 1 of S0-MEM already
// gave parseMemoryFields, encodeMemoryFields and validateMWFields
// (memdata.go, mw.go, mtcombined.go) a switch with a refusing default —
// see mtp11_test.go's TestMemoryP5_ZeroPolicyRefusesRatherThanDefaultingWide
// for the builder/encoder/parser proof. This test adds the GATE leg that
// test does not carry, so all four Stage 0 axes have builder+gate+parser
// evidence in one place.
func TestUnsetPolicy_MemoryP5_RefusesAtBuilderParserAndGate(t *testing.T) {
	base := FT710 // P5TxClar
	zeroed := base
	zeroed.memoryP5 = 0

	rec := p5TestRecord(t, base, false)

	if cmd, err := zeroed.BuildMWSet(rec); err == nil {
		t.Errorf("zeroed.BuildMWSet succeeded, emitting %q — a zero MemoryP5Policy must refuse, not default to the wide (TxClar) reading", cmd.Bytes())
	} else if !strings.Contains(err.Error(), "P5") {
		t.Errorf("zeroed.BuildMWSet refused with %q, which does not mention P5", err)
	}

	cmd, err := base.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("fixture broken: base.BuildMWSet: %v", err)
	}
	frame := cmd.Bytes()
	if zeroed.AllowedCommand(frame) {
		t.Errorf("zeroed's gate ADMITTED %q — a zero MemoryP5Policy must refuse at the gate too", frame)
	}
	if !base.AllowedCommand(frame) {
		t.Errorf("base's own gate refused its own builder's %q — the fixture is broken, not the policy under test", frame)
	}

	mrFrame := append([]byte(nil), frame...)
	mrFrame[0], mrFrame[1] = 'M', 'R'
	if got, err := zeroed.ParseMRAnswer(mrFrame); err == nil {
		t.Errorf("zeroed.ParseMRAnswer(%q) accepted it, returning %+v — a zero MemoryP5Policy must refuse rather than decode byte 21 as the TX clarifier flag", mrFrame, got)
	} else if !strings.Contains(err.Error(), "P5") {
		t.Errorf("zeroed.ParseMRAnswer(%q) refused with %q, which does not mention P5", mrFrame, err)
	}
}

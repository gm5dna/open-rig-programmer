// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// bank60mLabel and bankEMGLabel are the display labels this driver mints
// for the two DISCOVERED banks — both by live discovery
// (effectiveCapabilities) and by offline classification
// (SynthesiseDiscoveredBanks), so the two paths cannot drift apart.
//
// They are THIS package's own consts, not core/driver/ft710's: the strings
// coincide today because both radios' banks mean the same thing to a user
// and the neutral bank IDs (spec.Bank60m, spec.BankEMG) render the same
// way across the app, but a display label is not a protocol fact and
// nothing forces the two to stay equal. The FTdx10's own slot legends call
// these "5xx (5MHz BAND)" and "EMG (EMERGENCY CH)"; "60 m" is the band's
// amateur name.
const (
	bank60mLabel = "60 m channels"
	bankEMGLabel = "Emergency (EMG)"
)

// writeTrialsComplete is THIS driver's hardware write guard, and it is
// FALSE: no FTdx10 has ever been written to by this project. There is no
// docs/hardware-notes.md section for this model, no write-trial protocol
// run, and no captured frame from a real one.
//
// While it is false there is no hardware-verified capability profile for
// this driver to select AT ALL — deliberately not even a placeholder one:
// a RealHardware session gets CapabilitiesUnverified (see
// ftdx10Driver.Capabilities), nothing is writable anywhere, and the
// capability gate refuses every write before a frame is built.
//
// It is consulted by no production code, and that is the point. The
// FT-710's flip is the template for what changing this must look like
// (core/driver/ft710/caps.go's own writeTrialsComplete doc comment): a
// TWO-PART change — this constant AND a CapabilitiesRealHardware profile
// built field class by field class from the trial evidence, AND the
// Capabilities switch rewritten to select it — with the evidence linked
// and the pin test below rewritten so the flip is a visible, reviewable
// test change. Making this constant load-bearing on its own would mean a
// one-character edit could unlock a write, which is exactly what the
// FT-710's pre-flip design refused.
//
// The pin: TestWriteTrialsComplete_PinnedFalse asserts both halves — the
// constant is false, AND a RealHardware driver's baseline is genuinely
// nothing-writable, so a constant-only edit cannot pass while leaving the
// consequence untested.
const writeTrialsComplete = false

// modelName is the FTdx10's display name — the future driver registry key,
// and necessarily equal to Capabilities().Model (core/driver.Driver's
// contract). The exact spelling is not this task's choice to make: it is
// already pinned elsewhere in the project, by internal/wiring's
// TestModelSlug ({"FTdx10", "ftdx10"}) and its
// ResolveSnapshotDir test, both of which predate any FTdx10 driver. The
// registration task's radiotext entry takes this same spelling — and NOT
// "FT-DX10", which internal/radiotext's TestFor_UnknownModel deliberately
// lists among the near-misses that must stay unknown.
const modelName = "FTdx10"

// catID is the identity an FTdx10 answers "ID;" with, sourced from the
// dialect rather than restated here: one place this string exists, and the
// value the ID probe compares against is the same value the capability
// data advertises. TestCATID_ComesFromTheDialect pins both the linkage and
// the documented literal.
var catID = catDialect.CATID()

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE, mirroring the FT-710's
// reasoning: a forgotten or zero-valued Profile must fail towards the
// real-hardware capability set — which for this driver is the
// all-Unverified one, nothing writable — and NEVER towards the
// simulator's, whose Supported writes are a claim about
// internal/fakedx10 and about nothing else. Any OTHER unrecognised
// Profile value fails the same way, through Capabilities' explicit
// default arm.
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio.
	// While writeTrialsComplete is false it selects
	// CapabilitiesUnverified: reads labelled Unverified, every candidate
	// field's Write Unverified, nothing writable.
	RealHardware Profile = iota
	// Simulated is the profile for internal/fakedx10-backed sessions ONLY
	// (the CLI's --fake mode, the GUI's demo mode): Write Supported for
	// the six fields the combined MT form can express, so the write
	// choreography can be exercised end to end with no hardware at risk.
	Simulated
)

// modeNames returns the selectable mode display names this radio's
// capability data advertises, in wire-code order, DERIVED FROM THE DIALECT
// rather than transcribed here.
//
// There is deliberately no local mode table. The FT-710's driver keeps one
// (its modeTable) and pins it against its dialect with a test, because it
// predates the dialect seam; a second driver copying that shape would copy
// its one real hazard too — a transcription that drifts from the radio's
// own legend and offers a user a mode the radio has never heard of, with
// only a test standing between the two spellings. Enumerating the dialect
// instead makes the drift unrepresentable: the names ARE the dialect's,
// which transcribed them once, from the FTdx10 manual's four identical mode
// legends (core/cat/ftdx10's modeNames).
//
// Wire-code order comes free: cat.Mode's underlying value IS the wire byte,
// so ascending byte order is the legend's own order ('1'-'9' then 'A'-'F').
//
// cat.ModeUnset ('0', "-") is excluded explicitly. It is a
// parse-accept-only placeholder that appears in NO FTdx10 mode legend (the
// dialect's own ASSUMED register entry 4) and is present in the dialect's
// table only so that parsers may accept it; offering it as a selectable
// mode would invite a user to write a value core/cat refuses to emit.
//
// Nothing on a wire path consults this: the read path renders through the
// session's own dialect (s.dialect.ModeName, read.go) and the write path
// will resolve through dialect.ModeByName.
func modeNames() []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		m := cat.Mode(byte(b))
		if m == cat.ModeUnset || !catDialect.ValidMode(m) {
			continue
		}
		names = append(names, catDialect.ModeName(m))
	}
	return names
}

// memSlots returns the MEM bank's slot inventory, "001".."099", built
// through the DIALECT's own MemorySlot so the wire forms this capability
// data advertises are the ones that radio's slot space actually accepts —
// never a locally formatted string that ParseSlot might later refuse. The
// range's end is where MemorySlot stops accepting an ordinal, which is
// core/cat/ftdx10's declared memoryLo/memoryHi (1..99) and not a number
// written out here.
func memSlots() []string {
	var slots []string
	for n := 1; ; n++ {
		s, err := catDialect.MemorySlot(n)
		if err != nil {
			return slots
		}
		slots = append(slots, s.Wire())
	}
}

// pmsSlots returns the PMS bank's slot inventory, "P1L".."P9U", built
// through the dialect's PMSSlot for the same reason memSlots uses
// MemorySlot: the pair count is the dialect's declared PMSPairs (9), read
// by walking until it refuses, not restated here.
func pmsSlots() []string {
	var slots []string
	for pair := 1; ; pair++ {
		lower, err := catDialect.PMSSlot(pair, false)
		if err != nil {
			return slots
		}
		upper, err := catDialect.PMSSlot(pair, true)
		if err != nil {
			// Unreachable: PMSSlot's bound is on the pair, not the
			// half. Refuse to emit a half-pair rather than guess.
			return slots
		}
		slots = append(slots, lower.Wire(), upper.Wire())
	}
}

// bankFields builds the per-field support map shared by the MEM and PMS
// banks. EVERY spec.Field this project models is listed explicitly,
// including the four that are the zero FieldSupport: a field left out of
// this map reads identically to a field deliberately zeroed
// (Capabilities.FieldSupport returns the zero value for an absent key),
// and only a written-down zero is legible as a decision.
//
//   - rw covers the five fields the combined MT record expresses and this
//     driver maps in both directions: frequency, mode, CTCSS STATE, shift
//     and tag.
//
//   - clar covers spec.FieldClarifier separately, purely so the two
//     profiles can differ on it without the rest moving. It is
//     deliberately NOT spec.Inert in any profile: Inert is the FT-710's
//     HARDWARE finding (that radio accepts the clarifier on a write and
//     reads back zeros), and this radio has never been asked. See doc.go's
//     non-borrowing note.
//
//   - spec.FieldTagDisplay is the ZERO FieldSupport — Read AND Write
//     Unsupported — on every bank and in every profile, and this is a
//     MANUAL FACT rather than an assumption: the FTdx10's combined MT
//     record has no display flag. Its 41 positions are fully accounted
//     for (slot, frequency, clarifier sign and magnitude, the two
//     clarifier flags, mode, kind, CTCSS state, the fixed "00" P9, shift,
//     the fixed "0" P11, a 12-byte tag, terminator), and
//     cat.Dialect.BuildMTSetCombined's signature takes no display
//     argument because there is nowhere to put one. Reads therefore report
//     codeplug.Unavailable (read.go) — "this radio has no such field" —
//     which is a different statement from Unknown, and the FTdx10 is this
//     project's first radio to make it.
//
//   - spec.FieldCTCSSTone and spec.FieldScanSkip are the zero
//     FieldSupport for a weaker reason, recorded as ASSUMED-register
//     entry 6: the record carries a CTCSS STATE byte but no tone number
//     and no skip flag, and nothing has verified what the state byte does
//     on this radio.
//
//   - spec.FieldErase is the zero FieldSupport in both directions, on
//     every bank and profile: the FTdx10's CAT command set contains no
//     erase command, so this driver has no erase to offer. Whether some
//     Set frame has an erasing side effect on this radio is unknown and
//     deliberately not claimed — Unsupported is the direction that needs
//     no evidence, and it keeps a populated channel going back to empty
//     permanently blocked (codeplug.Diff gates on FieldErase, not on
//     Bank.NoBlank).
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw, clar spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  clar,
		spec.FieldCTCSSState: rw,
		spec.FieldShift:      rw,
		spec.FieldTag:        rw,
		// No display flag exists in the combined MT record — a manual
		// fact, not an assumption. See the doc comment above.
		spec.FieldTagDisplay: {},
		// Tone number and scan skip: ASSUMED-register entry 6.
		spec.FieldCTCSSTone: {},
		spec.FieldScanSkip:  {},
		// No CAT erase command exists in this radio's command set.
		spec.FieldErase: {},
	}
}

// baseCapabilities assembles the static baseline both profiles share,
// with the given per-bank field maps.
//
// ALL FIFTEEN spec.Capabilities fields are populated explicitly — that is
// the D-caps-explicit decision, and TestCapabilities_EveryFieldExplicit
// reflects over the struct to enforce it. A zero left in a capability
// field is not a neutral omission: a zero MaxFreqHz reads as "no ceiling"
// to every validator, a zero TagLen makes core/csvio's CHIRP import
// silently erase every channel name, and an empty Bauds makes
// core/transport substitute a guessed baud. Where the honest value is
// unverified it is populated anyway and the ASSUMED register carries the
// provenance (entries 3, 4 and 5).
//
// Banks: MEM "001"-"099" and PMS "P1L"-"P9U", both with NoBlank stated
// FALSE explicitly (see the per-bank comments). NO 5xx or EMG bank: that
// inventory is DISCOVERED per session by Open, never asserted statically —
// see doc.go, "Discovery walks the WHOLE declared range".
func baseCapabilities(memFields, pmsFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: "Memories",
				Slots: memSlots(),
				// NoBlank FALSE, stated: an empty memory channel is an
				// ordinary state on this radio as on any other, and a
				// NoBlank MEM bank would make codeplug.Validate refuse
				// every candidate with a single blank channel. The one
				// channel this driver claims must stay populated is
				// RequiredSlots' "001" (ASSUMED-register entry 5), which
				// is the per-slot mechanism, not the per-bank one.
				NoBlank: false,
				Fields:  memFields,
			},
			{
				ID:    spec.BankPMS,
				Label: "Scan limits (PMS)",
				Slots: pmsSlots(),
				// NoBlank FALSE, stated: nothing establishes that an
				// FTdx10 ships with its PMS pairs populated, and the
				// FT-710's own NoBlank PMS bank was REMOVED at M5b for
				// exactly the failure a wrong guess here causes — real
				// radios shipped all-PMS-empty, so codeplug.Validate
				// rejected every real-derived candidate before Diff ever
				// ran, including MEM-only edits. A populated PMS slot
				// going back to empty stays blocked regardless, by
				// FieldErase never being writable.
				NoBlank: false,
				Fields:  pmsFields,
			},
		},
		Modes: modeNames(),
		// TagLen: the combined MT record's P12 field is 12 characters —
		// "TAG Characters (up to 12 characters) (ASCII)" — and the
		// dialect's MTPolicy.TagMaxBytes is 12 to match. The byte the
		// radio PADS a short tag with is the dialect's own ASSUMED
		// TagFill (its register entry 1); the WIDTH is manual-evidenced.
		TagLen: 12,
		// Clarifier policy, from the dialect: the 0000-9990 Hz range is
		// manual-evidenced (the MR/MT/MW legends and the RD/RU command
		// pages all agree), the 10 Hz STEP is ASSUMED — in the DIALECT's
		// register (entry 2), cited here and deliberately not
		// re-registered as this driver's own.
		ClarMaxHz:  9990,
		ClarStepHz: 10,
		// The 50-tone chart, manual-evidenced: Table 1, entries 000-049,
		// 67.0-254.1 Hz, cross-checked element for element against
		// spec.standardCTCSSTones at spec review and re-checked by
		// TestCTCSSTones_MatchTheStandardChart.
		CTCSSTones: tones[:],
		// CAT RATE, menu 3-01-08 (manual line 811): FOUR rates and no
		// 115200 — the first real Bauds divergence from the FT-710, and
		// manual-evidenced. DefaultBaud is the ASSUMED one (register
		// entry 3): this manual has no factory-default column at all.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// ASSUMED bounds (register entry 4): this manual states no range,
		// only a 9-digit "Frequency (Hz)" field. MaxFreqHz must not be
		// left zero — a zero ceiling reads as "unbounded".
		MinFreqHz: 30_000,
		MaxFreqHz: 75_000_000,
		// ASSUMED (register entry 5): no FTdx10 statement requires
		// channel 001 to stay populated.
		RequiredSlots: []string{"001"},
		// The shift and CTCSS-state vocabularies the wire protocol
		// expresses. Both are the family-wide standard sets: this radio's
		// P10 shift legend ("0 simplex, 1 plus, 2 minus") and P8 CTCSS
		// legend ("0 off, 1 ENC/DEC, 2 ENC") are the same three-value
		// vocabularies core/cat parses for it, which is what
		// core/cat/ftdx10's reused-command verification established when
		// it accepted the shared codec.
		ShiftOptions: spec.StandardShiftOptions(),
		CTCSSStates:  spec.StandardCTCSSStates(),
	}
}

// CapabilitiesUnverified is the all-Unverified FAIL-SAFE profile, and it
// is what a RealHardware FTdx10 session gets today: every field the
// combined MT record expresses is labelled Read Unverified / Write
// Unverified — documented in the CAT manual and exercised against scripted
// peers, but never proven against a radio — and every field the record does
// not express stays the zero FieldSupport.
//
// Because Unverified makes FieldSupport.CanWrite false, this profile
// blocks every write project-wide: codeplug.Diff refuses the change, the
// clone service refuses to execute a plan containing it, and
// Session.WriteChannel re-checks and refuses before building a frame. It
// is also what any UNRECOGNISED Profile value selects — the failure
// direction is always "nothing writable".
//
// The READ labels are Unverified rather than Supported for the same
// reason: this driver's read path has been exercised against a fake and a
// manual, and no FTdx10 has ever answered a frame.
func CapabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	clar := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw, clar), bankFields(rw, clar))
}

// CapabilitiesSimulated is the internal/fakedx10-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the SIX fields the combined MT form can express — frequency,
// mode, clarifier, CTCSS state, shift and tag — on MEM and PMS alike.
//
// Against the fake, hardware risk is moot and the write choreography
// itself is what is being exercised end to end (the MT-only Set, the
// one-step WriteResult, the journal's projection of it), so claiming
// Supported here is a claim about the fake and is true of it.
//
// The CLARIFIER IS SUPPORTED, NOT Inert — the one place this profile
// deliberately declines to mirror the FT-710's, whose clarifier is Inert
// in every profile because a real FT-710 was observed ignoring it.
// internal/fakedx10 stores the clarifier and the combined Set round-trips
// it byte-faithfully, so Supported describes what happens; see doc.go's
// non-borrowing note for what a future Stage W finding would change, and
// where.
//
// tag_display, ctcss_tone, scan_skip and erase stay the zero
// FieldSupport, simulator or not: the FORM cannot express them (or, for
// tone and skip, is not known to), and no amount of cooperative fake on
// the other end of the wire changes what the frame has room for.
func CapabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	clar := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw, clar), bankFields(rw, clar))
}

// readOnlyFields derives a DISCOVERED bank's field map from base's MEM
// bank: Read supports carried through unchanged (whatever the profile
// says about reading), every Write forced to spec.Unsupported.
//
// No profile — not even Simulated — may claim a discovered 5xx/EMG slot
// writable: cat.Slot.Writable excludes those slots from MW, and
// cat.Dialect's own combined-MT write policy (mtSlotValid) refuses them
// too, so a Supported label would advertise a write the codec will not
// build. Each call returns a fresh map.
func readOnlyFields(base spec.Capabilities) map[spec.Field]spec.FieldSupport {
	mem, _ := base.Bank(spec.BankMemory) // already a defensive copy
	fields := make(map[spec.Field]spec.FieldSupport, len(mem.Fields))
	for f, fs := range mem.Fields {
		fs.Write = spec.Unsupported
		fields[f] = fs
	}
	return fields
}

// cloneCapabilities returns a deep copy of caps: Banks (each with fresh
// Slots and Fields) and every other slice independently allocated, so
// mutating the copy can never reach the original.
//
// Load-bearing for the write gate, exactly as in the FT-710's driver:
// Session.Capabilities hands copies out, and a caller mutating one must
// never alter what WriteChannel enforces.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		// Capabilities.Bank returns a defensive copy (fresh Slots and
		// Fields) — reuse that guarantee rather than restating per-field
		// copying here.
		cp, _ := caps.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
	}
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), caps.CTCSSTones...)
	out.Bauds = append([]int(nil), caps.Bauds...)
	out.RequiredSlots = append([]string(nil), caps.RequiredSlots...)
	out.ShiftOptions = append([]spec.ShiftOption(nil), caps.ShiftOptions...)
	out.CTCSSStates = append([]spec.ToneState(nil), caps.CTCSSStates...)
	return out
}

// effectiveCapabilities builds a Session's capability set: a deep copy of
// the profile baseline plus one READ-ONLY bank per discovered inventory —
// 60M when any 5xx slot answered, EMG when EMG did, in that fixed order.
//
// Both discovered banks are NoBlank TRUE: those channels exist because
// they answered a read, and this radio's CAT surface has no way to blank
// them (no erase command, and its MW/MT write policies exclude 5xx/EMG
// slots outright). That is a statement about the protocol, not about the
// radio's factory contents.
func effectiveCapabilities(base spec.Capabilities, slots60m []string, emg bool) spec.Capabilities {
	caps := cloneCapabilities(base)

	if len(slots60m) > 0 {
		caps.Banks = append(caps.Banks, spec.Bank{
			ID:      spec.Bank60m,
			Label:   bank60mLabel,
			Slots:   append([]string(nil), slots60m...),
			NoBlank: true,
			Fields:  readOnlyFields(base),
		})
	}
	if emg {
		caps.Banks = append(caps.Banks, spec.Bank{
			ID:      spec.BankEMG,
			Label:   bankEMGLabel,
			Slots:   []string{catDialect.EMGSlot().Wire()},
			NoBlank: true,
			Fields:  readOnlyFields(base),
		})
	}
	return caps
}

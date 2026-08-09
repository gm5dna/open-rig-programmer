// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// bank60mLabel and bankEMGLabel are the shared display labels for the
// discovered 60m/EMG banks — task 37 (M9a-1) deduplication: BOTH
// effectiveCapabilities (ft710.go, live discovery via Open) and
// SynthesiseDiscoveredBanks (ft710.go, offline classification of a slot
// list) mint banks with these exact labels, so the two paths can never
// drift apart. These consts are now the SINGLE source of these labels:
// task 41 migrated app/uispec.go's offline synthesis onto
// SynthesiseDiscoveredBanks (through wiring.SynthesiseDiscoveredBanks), so
// the app no longer restates the literals in bank-label consts of its own
// — it renders whatever label this driver mints. TestGetUISpec_ConnectedSimulated
// and TestGetUISpec_OfflineWorkingCopy_Synthesises60mEMGBanks (app/) pin
// the rendered labels against these strings, so a drift here fails a test.
const (
	bank60mLabel = "60 m channels"
	bankEMGLabel = "Emergency (EMG)"
)

// writeTrialsComplete is THE hardware write guard for this driver — and
// it FLIPPED to true in the M5b PR (13/07/2026), exactly as its own
// pre-flip doc comment required: with hardware evidence linked, the
// hardware-verified profile introduced alongside it
// (CapabilitiesRealHardware), and the pin test rewritten
// (TestWriteTrialsComplete_FlippedTrue_M5b) so the flip is a visible,
// reviewable test change, never a silent constant edit.
//
// EVIDENCE (docs/hardware-notes.md, "M5b write trials" and its "Batch 9"
// supplementary-trials subsection — full per-step protocol outcomes and
// findings; raw transcript in the git-ignored docs/fixtures-private/):
// the committed write-trial protocol ran clean against Stuart's physical
// UK FT-710 on 13/07/2026, controller-driven, on sacrificial channel
// M-95 (plus 096, P1L/P1U) — every field class exercised, INCLUDING the
// gap Codex M5b review finding #2 (adjudicated HIGH) caught in the
// FIRST evidence pass: batch 9, later the same evening, closed it —
// all 15 applicable mode nibbles (`1`-`F`) accepted with no rejection
// and round-trip confirmed (`1`-`5` and the sweep's terminal value `F`
// directly, via an explicit MR read-back; the run of `6`-`E` consistent
// with the same accept/reject mechanism already hardware-validated
// elsewhere in this session — an IMMEDIATE ~10 ms `?;` on any rejection,
// never observed for any of the fifteen); all four tag-boundary
// character sets (all-`Z`, all-`0`, mixed alphanumeric-with-punctuation,
// heavy punctuation) byte-exact at the full 12-byte length; the PMS
// pair's own tag-set (display flag + tag), mode, CTCSS state, and shift
// all live-proven on P1L with individual read-back confirmation; P1U
// created (kind `1`, mirroring P1L). Frequency, CTCSS states, shift, and
// tags/display were exercised across both batches, immediate readback
// after every write, TX indicator watched throughout (no TX key-up).
// Restoration (Codex M5b fix wave, Fix 6 — honest wording; this comment
// used to say "everything restored"): the sacrificial channel, M-95, WAS
// restored byte-identical to its baseline except the non-writable P7
// bit. The empty-slot-create (096) and PMS-pair (P1L/P1U) trial
// artefacts were NOT restored to non-existence — no CAT erase command
// exists, so a populated slot cannot be blanked back over CAT at all —
// and remain on the radio pending manual front-panel cleanup (see
// docs/hardware-notes.md's "Restoration" and "Batch 9" sections). This
// does not weaken the flip's evidence: every write these trials made was
// verified, and the unrestorable state is itself a confirmed protocol
// fact (no erase exists), not an anomaly.
//
// While this was false, every real-hardware session got
// CapabilitiesUnverified (nothing writable anywhere); now RealHardware
// sessions get CapabilitiesRealHardware — Write: Supported for exactly
// the six HW-verified fields, everything else honest (see that
// profile's doc comment). What still stands between a user and a wire
// write, post-flip: codeplug.Diff's per-field/Inert/erase gates, the
// clone service's full choreography (fresh baseline, immutable plan,
// dual-digest recheck, session binding, explicit confirmation digest,
// first-write firmware confirmation, per-slot verify-read +
// write-then-verify), Session.WriteChannel's own capability re-check,
// and internal/guards' import-graph pin on the write path itself.
//
// FLIP-PRECONDITION ADJUDICATION (restated here per the M5b brief, for
// review): the ledgered "write-capability split" precondition (Codex M3
// — split the write-capable surface into its own package so the
// compiler enforces the composition) was adjudicated
// RATIFIED-AS-NOT-NEEDED before this flip: internal/guards'
// import-graph test pins the write path repo-wide (transport.Engine.Do
// and the Set-frame builders reachable only via core/driver/**;
// WriteChannel only via core/driver/** and core/clone/**); three
// consecutive Codex milestone reviews (M3, M4, M6) confirmed no
// writable RealHardware composition existed; and the M3 adjudication
// placed external importers outside the threat model (nothing stops a
// third-party module calling the transport directly — the guards pin
// THIS repository's composition, which is what the layered write-safety
// story actually promises). The composition discipline those guards
// enforce is unchanged by this flip.
const writeTrialsComplete = true

// modelName is the FT-710's display name.
const modelName = "FT-710"

// catID is the identity an FT-710 answers "ID;" with ("ID; -> ID0800;",
// golden vector G1), sourced from the codec's dialect rather than restated
// here — one place this string exists, and the value M9c's driver
// registration will read too. TestCATID_ComesFromTheDialect pins both the
// linkage and the documented literal.
var catID = catDialect.CATID()

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE: a forgotten or zero-valued
// Profile always fails towards the real-hardware capability set — whose
// every write is gated by the clone choreography and was hardware-
// verified at M5b — and NEVER towards the simulator's profile, whose
// Supported reads (and pre-flip, writes) were never a claim about real
// hardware. Any OTHER unrecognised Profile value fails harder still, to
// the all-Unverified fail-safe (see ft710Driver.Capabilities).
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio:
	// CapabilitiesRealHardware now writeTrialsComplete is true (the M5b
	// flip — write-Supported for exactly the six hardware-verified
	// fields); it would fall back to the all-Unverified
	// CapabilitiesUnverified were the flip ever reverted.
	RealHardware Profile = iota
	// Simulated is the profile for fakeradio-backed sessions ONLY (the
	// CLI's --fake mode, the GUI's demo mode): Write = Supported for the
	// fields the codec can express, so write paths can be exercised
	// end-to-end with no hardware at risk.
	Simulated
)

// modeTable is the single source of truth pairing each selectable cat
// mode with its display name, in wire-code order, for CAPABILITY
// RENDERING (caps.Modes and the modeByName lookup below) ONLY.
//
// It is NOT consulted by the write path. Before task 67 (M9c-0),
// buildWriteCommands (write.go) resolved a channel's Mode string through
// modeByName, built from this table independently of any session's
// dialect — so a dialect that renamed a mode had no effect on what got
// written, and cat.Dialect's own name-uniqueness rule protected nothing
// (Codex spec review, finding 7, recorded on cat.Dialect.ModeByName's own
// doc comment). buildWriteCommands now resolves through
// dialect.ModeByName instead — the SESSION's own dialect, copied from
// whichever driver Opened it — so this table's names must still agree
// with cat.FT710's: TestModeTable_MatchesFT710Dialect
// (modebyname_test.go) pins that every name here resolves through
// cat.FT710.ModeByName to the same cat.Mode, and TestModes_MatchCatModeNames
// pins this table's round-trip against caps.Modes.
//
// The read path is s.dialect.ModeName (read.go's ReadChannel, rerouted
// at M9b task 56), NOT cat.Mode.String: a rendered mode string is
// user-visible, so it must come from the mode table of the radio that
// answered rather than the FT-710's on some other radio's behalf.
// Mode.String survives only as a dialect-free diagnostic fallback (see
// its own doc comment). For cat.FT710 the two are byte-identical by
// construction — cat/dialect.go wires the dialect's modeNames field to
// the package map Mode.String reads — which is why this table can pin
// both spellings at once, and TestModes_MatchCatModeNames renders
// through the dialect so it guards the real path, not the fallback.
//
// ModeUnset ('0', "-") is deliberately absent: it is a
// parse-accept-only placeholder, never a selectable mode.
var modeTable = []struct {
	name string
	mode cat.Mode
}{
	{"LSB", cat.ModeLSB},
	{"USB", cat.ModeUSB},
	{"CW-U", cat.ModeCWU},
	{"FM", cat.ModeFM},
	{"AM", cat.ModeAM},
	{"RTTY-L", cat.ModeRTTYL},
	{"CW-L", cat.ModeCWL},
	{"DATA-L", cat.ModeDATAL},
	{"RTTY-U", cat.ModeRTTYU},
	{"DATA-FM", cat.ModeDATAFM},
	{"FM-N", cat.ModeFMN},
	{"DATA-U", cat.ModeDATAU},
	{"AM-N", cat.ModeAMN},
	{"PSK", cat.ModePSK},
	{"DATA-FM-N", cat.ModeDATAFMN},
}

// modeByName is a capability-rendering display-name -> cat.Mode lookup,
// built from modeTable (see its doc comment for the round-trip
// invariant). The write path does NOT use it — see modeTable's doc
// comment — it exists for TestModes_MatchCatModeNames' round-trip pin.
var modeByName = buildModeByName()

func buildModeByName() map[string]cat.Mode {
	m := make(map[string]cat.Mode, len(modeTable))
	for _, e := range modeTable {
		m[e.name] = e.mode
	}
	return m
}

// modeNames returns a fresh copy of modeTable's display names, in order.
func modeNames() []string {
	names := make([]string, len(modeTable))
	for i, e := range modeTable {
		names[i] = e.name
	}
	return names
}

// memSlots returns the MEM bank's slot inventory: "001".."099".
func memSlots() []string {
	slots := make([]string, 0, 99)
	for n := 1; n <= 99; n++ {
		slots = append(slots, fmt.Sprintf("%03d", n))
	}
	return slots
}

// pmsSlots returns the PMS bank's slot inventory: "P1L","P1U".."P9U".
func pmsSlots() []string {
	slots := make([]string, 0, 18)
	for pair := 1; pair <= 9; pair++ {
		slots = append(slots, fmt.Sprintf("P%dL", pair), fmt.Sprintf("P%dU", pair))
	}
	return slots
}

// bankFields builds the per-field support map shared by the MEM and PMS
// banks: rw applies to six of the seven fields the MR/MW/MT codec can
// express (frequency, mode, ctcss_state, shift, tag, tag_display); clar
// applies to FieldClarifier separately — HW-CONFIRMED 2026-07-13 (M5b
// write trials, docs/hardware-notes.md): the radio silently IGNORES the
// clarifier value and Rx/Tx flags on an MW write (accepted with no "?;"
// rejection, read back zeros every time), so EVERY profile marks its
// Write spec.Inert — transmitted but ignored; codeplug.Diff blocks a
// CHANGED clarifier with a precise reason and lets an unchanged one flow
// (see spec.Inert's doc comment for the split enforcement). Its Read is
// each profile's ordinary read level: the MR answer's clarifier bytes
// parse correctly (a front-panel-set clarifier — never observed, not
// ruled out — would presumably read back). erase applies to FieldErase;
// and FieldCTCSSTone/FieldScanSkip are always the zero FieldSupport
// (Unsupported/Unsupported) — the CAT protocol has no way to read OR
// write a memory channel's tone or scan skip (they only exist in the
// radio's own memory UI), so no profile may ever claim them. Each call
// returns a fresh map.
//
// FieldCTCSSTone.Read is HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md
// §MR bytes-25/26 refutation), not merely inferred from the manual's
// silence: live probe read M-06 (CTCSS tone 146.2 Hz demonstrably SET
// and ACTIVE, P8=1, on the radio) and its MR answer's bytes 25-26 (P9)
// still read fixed "00" — the Hamlib live-tone-index theory is refuted.
// There is no side channel; per-channel tone is permanently unreadable
// over CAT on this firmware, not just unverified.
//
// FieldScanSkip.Read was NOT probed at M5a (a read-only session — see
// docs/hardware-notes.md's "Explicitly not probed" list, "ScanSkip
// readability" bullet, and its "Consequences applied" table row) and
// stays exactly as it was: Unsupported by design-time reasoning (the
// fully-specified 28-byte MR/MW frame layout has no field for it at
// all — every byte position is already accounted for by
// frequency/clarifier/mode/kind/CTCSS/P9/shift), not by a live hardware
// trial. This is a narrower, purely structural claim than CTCSSTone's,
// and is left honestly unconfirmed by hardware pending M5b/a future
// session that might reveal a side channel this project has not yet
// looked for.
func bankFields(rw, clar, erase spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency: rw,
		spec.FieldMode:      rw,
		// Clarifier: Write Inert in every profile — HW-CONFIRMED ignored
		// on write; see the doc comment above.
		spec.FieldClarifier:  clar,
		spec.FieldCTCSSState: rw,
		spec.FieldShift:      rw,
		spec.FieldTag:        rw,
		spec.FieldTagDisplay: rw,
		// CTCSSTone/ScanSkip: not expressible via CAT in either
		// direction — see the doc comment above.
		spec.FieldCTCSSTone: {},
		spec.FieldScanSkip:  {},
		spec.FieldErase:     erase,
	}
}

// baseCapabilities assembles the static baseline shared by both profiles,
// with the given per-bank field maps. Banks: MEM "001"-"099" (M-01,
// "001", individually required — the radio always keeps it populated)
// and PMS "P1L"-"P9U". PMS is deliberately NOT NoBlank (Codex M5b fix
// wave, Fix 3, adjudicated HIGH — REMOVED, was NoBlank:true): real
// radios ship all-PMS-empty (M5a's characterised radio began with all 18
// PMS slots empty, and M5b's write trials created only P1L, leaving the
// rest untouched — docs/hardware-notes.md), so a NoBlank PMS bank made
// codeplug.Validate reject EVERY real-derived candidate containing an
// empty PMS slot before Diff ever ran — including a plain MEM-only edit
// that never touches PMS at all. A populated PMS slot going back to
// empty stays safely blocked regardless: FieldErase is nowhere
// write-Supported (see bankFields), independent of NoBlank. NO 60m/EMG
// banks here: their inventory is region-dependent and DISCOVERED per
// session by Open, never asserted statically.
func baseCapabilities(memFields, pmsFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		Banks: []spec.Bank{
			{
				ID:     spec.BankMemory,
				Label:  "Memories",
				Slots:  memSlots(),
				Fields: memFields,
			},
			{
				ID:     spec.BankPMS,
				Label:  "Scan limits (PMS)",
				Slots:  pmsSlots(),
				Fields: pmsFields,
			},
		},
		Modes:      modeNames(),
		TagLen:     12,   // reference §MT: "tag, up to 12 ASCII characters"
		ClarMaxHz:  9990, // reference P3: "4-digit offset 0000-9990 Hz"
		ClarStepHz: 10,   // reference P3: 10 Hz steps
		CTCSSTones: tones[:],
		// CAT-1 serial rates per the FT-710 operation manual's CAT RATE
		// menu; 38400 8-N-2 is the reference's stated default
		// (transport.DefaultBaud agrees).
		Bauds:       []int{4800, 9600, 19200, 38400, 115200},
		DefaultBaud: 38400,
		// Storable frequency bounds = the RECEIVE tuning range, 30 kHz -
		// 75 MHz, per the FT-710 operation manual's specifications page
		// (RX covers more than TX; a memory channel may legitimately hold
		// any receivable frequency).
		MinFreqHz:     30_000,
		MaxFreqHz:     75_000_000,
		RequiredSlots: []string{"001"}, // M-01 must never be empty
		// Shift and CTCSS-state vocab: the FT-710's own literals, task 38
		// (M9a-2) moved out of core/codeplug's validation switches into
		// this capability data.
		ShiftOptions: spec.StandardShiftOptions(),
		CTCSSStates:  spec.StandardCTCSSStates(),
	}
}

// CapabilitiesUnverified is the all-Unverified FAIL-SAFE profile: what
// every real-hardware session got while writeTrialsComplete was false,
// and what any UNRECOGNISED Profile value still gets today (see
// ft710Driver.Capabilities — the failure direction stays "nothing
// writable" even post-flip). Every expressible field's Read AND Write
// support is spec.Unverified — documented in the CAT reference manual
// and exercised against fakeradio, but treated as unproven — EXCEPT the
// clarifier, whose Write is spec.Inert (HW-CONFIRMED 2026-07-13 ignored
// on write — see bankFields; a hardware finding does not un-happen just
// because this fail-safe profile is selected).
// Because neither Unverified nor Inert makes CanWrite() true, this
// profile blocks every write project-wide (codeplug.Diff, the clone
// service, and Session.WriteChannel's own re-check — an every-field-
// Unverified entry blocks on the six rw fields regardless of the
// clarifier). FieldErase on MEM stays Unverified here (Codex M5b fix
// wave, Fix 6 — this text used to call it "an open M5b question": M5b
// has since settled it permanently, for every profile — no CAT erase
// command exists at all, HW-CONFIRMED, see CapabilitiesRealHardware's
// FieldErase evidence, which reflects the answer as Unsupported
// outright. This fail-safe profile's label simply has not been
// tightened to match: CanWrite() is already false for Unverified exactly
// as it is for Unsupported, so nothing behaves differently either way).
// PMS erase is Unsupported outright, independent of the (now removed —
// Fix 3) NoBlank flag: the same no-CAT-erase finding applies to PMS too,
// so a populated PMS slot going back to empty stays blocked here as
// well.
func CapabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	clar := spec.FieldSupport{Read: spec.Unverified, Write: spec.Inert}
	memFields := bankFields(rw, clar, spec.FieldSupport{Read: spec.Unsupported, Write: spec.Unverified})
	pmsFields := bankFields(rw, clar, spec.FieldSupport{})
	return baseCapabilities(memFields, pmsFields)
}

// CapabilitiesSimulated is the fakeradio-backed profile (CLI --fake, GUI
// demo) ONLY — never a real radio: Write = Supported for exactly the
// fields the codec can express AND real hardware honours (frequency,
// mode, ctcss_state, shift, tag, tag_display, on MEM and PMS), because
// against the simulator "hardware" risk is moot and the write path
// itself is what is being exercised. The clarifier's Write is spec.Inert
// here too (HW-CONFIRMED 2026-07-13 — see bankFields), so --fake/demo
// behaviour matches real-radio behaviour: a changed clarifier is Blocked
// by codeplug.Diff in the simulator exactly as it would be against the
// real radio, and fakeradio itself mirrors the ignore (MW clarifier
// fields accepted, stored as zeros — internal/fakeradio's handleMW).
// Erase stays Unsupported (the codec has no erase command to express,
// simulator or not — and NO CAT erase exists, HW-CONFIRMED), and
// CTCSSTone/ScanSkip stay Unsupported (the CAT protocol cannot express
// them regardless of what is on the other end of the wire).
func CapabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	clar := spec.FieldSupport{Read: spec.Supported, Write: spec.Inert}
	memFields := bankFields(rw, clar, spec.FieldSupport{})
	pmsFields := bankFields(rw, clar, spec.FieldSupport{})
	return baseCapabilities(memFields, pmsFields)
}

// CapabilitiesRealHardware is the REAL-HARDWARE profile now
// writeTrialsComplete is true: the write support the M5b trials
// actually verified, field class by field class, and nothing more.
// Evidence per field class (docs/hardware-notes.md, "M5b write trials"
// and its "Batch 9" supplementary-trials subsection — Codex M5b review
// finding #2, adjudicated HIGH, caught the first evidence pass falling
// short of this claim for mode/tag/PMS coverage; batch 9, the same
// evening, closed the gap — all 13/07/2026 against a physical UK
// FT-710):
//
//   - frequency/mode/ctcss_state/shift (MW) and tag/tag_display (MT):
//     Write Supported — every class exercised live with immediate
//     readback: frequency changes; ALL 15 applicable mode nibbles
//     (`1`-`F`), each accepted with no rejection (`1`-`5` and the
//     sweep's terminal `F` independently round-trip-confirmed via an
//     explicit MR read-back — see core/cat/hw_derived_m5b_test.go's
//     batch-9 vectors); all three CTCSS states; all three shifts; tag
//     set/clear and both display-flag values, INCLUDING all four
//     tag-boundary character sets (all-`Z`, all-`0`, mixed
//     alphanumeric-with-punctuation, heavy punctuation) byte-exact at
//     the full 12-byte length. Proven on MEM AND on PMS (kind '1' — the
//     kind-pairing bug fix): the PMS pair's own tag-set, mode, CTCSS
//     state, and shift changes were separately live-proven on P1L
//     (individual read-back per change), and P1U was created (kind
//     `1`), including an empty-slot create (096, MEM).
//   - clarifier: Write Inert — transmitted but IGNORED by the radio
//     (accepted without rejection, read back zeros every time); see
//     bankFields and spec.Inert. NOT Supported: a changed value
//     provably never takes effect.
//   - ctcss_tone/scan_skip: Write (and Read) Unsupported — the CAT
//     protocol has no command for either, in either direction; M5b
//     additionally PROVED the stored tone survives an MW rewrite
//     (docs/hardware-notes.md §Tone preservation), so leaving them
//     un-writable is safe as well as honest. Scan-skip preservation
//     was never probed.
//   - erase: Write Unsupported — no longer merely unverified: NO CAT
//     erase exists, HW-CONFIRMED by a properly ISOLATED 13/07/2026
//     re-probe (four range/mode-isolated candidate MW frames, every one
//     rejected with "?;" — the radio range-checks frequency on every MW;
//     see docs/hardware-notes.md's "No CAT erase" section for why the
//     FIRST all-zero-frequency probe alone was confounded — it carried
//     mode nibble '0' (ModeUnset — a documented P6 value, but one no
//     Set-frame builder may emit), so its rejection proved nothing
//     specific to erase). Erased entries stay Blocked permanently,
//     on MEM and PMS alike — PMS is no longer a NoBlank bank (Codex M5b
//     fix wave, Fix 3: real radios ship all-PMS-empty), but a populated
//     PMS slot going back to empty is blocked exactly the same way MEM's
//     is, by FieldErase never being write-Supported.
//
// Read supports deliberately stay spec.Unverified for the six rw fields
// (and the clarifier): M5a exercised the read path extensively against
// this one radio, but the M5a repo-consequences task deliberately did
// not promote Read labels, and this flip changes ONLY what its hardware
// evidence covers — the WRITE direction. (Promoting reads would also
// leak Read: Supported into the discovered 60m/EMG banks via
// readOnlyFields — banks that have never been observed on any real
// radio.) Nothing consults Read support as a gate; this is labelling
// honesty, not a behaviour difference.
//
// The 60m/EMG discovered banks remain fully read-only regardless of
// this profile: effectiveCapabilities forces every Write to Unsupported
// for them (write was never probed — no such bank exists on the
// characterised radio — and cat.Dialect.writableSlot structurally
// excludes their slots from MW anyway).
func CapabilitiesRealHardware() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Supported}
	clar := spec.FieldSupport{Read: spec.Unverified, Write: spec.Inert}
	memFields := bankFields(rw, clar, spec.FieldSupport{Read: spec.Unsupported, Write: spec.Unsupported})
	pmsFields := bankFields(rw, clar, spec.FieldSupport{})
	return baseCapabilities(memFields, pmsFields)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// HW-DERIVED 2026-07-13 evening, UK FT-710, firmware V01-12 — M5b WRITE
// TRIALS, docs/hardware-notes.md.
//
// These vectors were captured live against Stuart's physical FT-710
// (controller-driven, sacrificial channel M-95 "BBC ANT 3" 11.685 MHz AM,
// plus empty-slot 096 and PMS pair P1L/P1U trials) and are ADDITIVE to the
// manual-derived G1-G12 golden vectors and the M5a HW-derived vectors
// (hw_derived_test.go): they prove the codec's WRITE-direction
// accept/reject behaviour against REAL wire exchanges, not just documented
// examples or the simulator's own modelling. Nothing below replaces an
// existing golden vector or weakens an existing assertion. Full transcript
// (PRIVATE, never committed, redaction policy docs/fixtures.md):
// docs/fixtures-private/m5b-trials.private-capture.
//
// "BBC ANT 3", "TRIAL 96" and "TRIAL 1" are NOT personal data: the first
// is a well-known BBC World Service shortwave relay label (already
// ledgered in .superpowers/sdd/progress.md's M5b sacrificial-channel
// nomination), the latter two are trial-artefact tags this session itself
// created — none of the three identify Stuart or reveal his real
// operating data. The session's one genuinely personal exchange (a
// same-data rewrite of M-06, carrying Stuart's real callsign) is
// deliberately NOT reproduced here; see hw_derived_test.go's existing
// MYCALL-redacted M-06 vectors for that channel's already-redacted
// coverage.

// mustMemorySlot/mustPMSSlot are defined in mt_test.go and reused here.

// TestBuildMWSet_HWDerived_M5b_Accepted pins the two live-accepted MW
// frames the M5b kind-pairing bug fix rests on: a memory-channel write
// (M-95, kind '1') and — the HW-CONFIRMED correction — a PMS write ALSO
// carrying kind '1' (KindMemory), not kind '5' (KindPMS) as this
// project's former ASSUMED pairing claimed.
func TestBuildMWSet_HWDerived_M5b_Accepted(t *testing.T) {
	tests := []struct {
		name string
		m    MemoryData
		want string
	}{
		{
			name: "M-95 memory write, AM 11.685 MHz, kind Memory",
			m: MemoryData{
				Slot:   mustMemorySlot(t, 95),
				FreqHz: 11_685_000,
				Mode:   ModeAM,
				Kind:   KindMemory,
				CTCSS:  CTCSSOff,
				Shift:  ShiftSimplex,
			},
			want: "MW095011685000+000000510000;",
		},
		{
			name: "P1L PMS write, LSB 7.100000 MHz, kind Memory (HW-CONFIRMED — NOT KindPMS)",
			m: MemoryData{
				Slot:   mustPMSSlot(t, 1, false),
				FreqHz: 7_100_000,
				Mode:   ModeLSB,
				Kind:   KindMemory,
				CTCSS:  CTCSSOff,
				Shift:  ShiftSimplex,
			},
			want: "MWP1L007100000+000000110000;",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := BuildMWSet(tc.m)
			if err != nil {
				t.Fatalf("BuildMWSet(%+v): unexpected error: %v", tc.m, err)
			}
			if got := string(cmd.Bytes()); got != tc.want {
				t.Errorf("BuildMWSet() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildMWSet_HWDerived_M5b_Rejected pins the three live-rejected MW
// frames: kind '0' (VFO) on a memory slot, kind '5' (KindPMS) on a PMS
// slot — the exact bug this task fixes — and the NO-CAT-ERASE probe
// (all-zero frequency), confirming the Erased->Blocked design.
// AllowedCommand (the transport-layer last-defence gate) must refuse
// every one of these exactly as BuildMWSet does — see
// TestAllowedCommand_HWDerived_M5b_RejectsSameFrames alongside it.
func TestBuildMWSet_HWDerived_M5b_Rejected(t *testing.T) {
	tests := []struct {
		name string
		m    MemoryData
	}{
		{
			name: "kind '0' (VFO) rejected on a memory slot",
			m: MemoryData{
				Slot:   mustMemorySlot(t, 95),
				FreqHz: 11_690_000,
				Mode:   ModeAM,
				Kind:   KindVFO,
				CTCSS:  CTCSSOff,
				Shift:  ShiftSimplex,
			},
		},
		{
			name: "kind '5' (KindPMS) rejected on a PMS slot — the live bug this task fixes",
			m: MemoryData{
				Slot:   mustPMSSlot(t, 1, false),
				FreqHz: 7_100_000,
				Mode:   ModeLSB,
				Kind:   KindPMS,
				CTCSS:  CTCSSOff,
				Shift:  ShiftSimplex,
			},
		},
		{
			// The live erase-probe frame ("MW096000000000+000000010000;")
			// actually carried mode digit '0' (ModeUnset), which
			// BuildMWSet would ALSO reject on its own separate terms. This
			// test deliberately substitutes ModeAM so the ONLY violation
			// under test is the zero frequency — isolating the NO-CAT-ERASE
			// assertion from the unrelated ModeUnset rejection; see
			// TestAllowedCommand_HWDerived_M5b_RejectsSameFrames below for
			// the exact live bytes (mode digit included) run through the
			// wire-level gate instead.
			name: "erase probe (all-zero frequency) rejected — NO CAT ERASE, Erased->Blocked confirmed",
			m: MemoryData{
				Slot:   mustMemorySlot(t, 96),
				FreqHz: 0,
				Mode:   ModeAM,
				Kind:   KindMemory,
				CTCSS:  CTCSSOff,
				Shift:  ShiftSimplex,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BuildMWSet(tc.m); err == nil {
				t.Fatalf("BuildMWSet(%+v): want error (HW-CONFIRMED rejected), got success", tc.m)
			}
		})
	}
}

// TestAllowedCommand_HWDerived_M5b_RejectsSameFrames pins the SAME three
// live-rejected frames directly as raw wire bytes through AllowedCommand
// — the transport-layer last-defence gate — rather than through
// BuildMWSet's MemoryData constructor, so the two independent checking
// paths (builder-side validation, wire-level re-validation) are BOTH
// proven against the exact bytes the radio rejected.
func TestAllowedCommand_HWDerived_M5b_RejectsSameFrames(t *testing.T) {
	tests := []string{
		"MW095011690000+000000500000;", // kind '0' (VFO) on a memory slot
		"MWP1L007100000+000000150000;", // kind '5' (KindPMS) on a PMS slot
		"MW096000000000+000000010000;", // erase probe: all-zero frequency
	}
	for _, frame := range tests {
		t.Run(frame, func(t *testing.T) {
			if AllowedCommand([]byte(frame)) {
				t.Errorf("AllowedCommand(%q) = true, want false (HW-CONFIRMED rejected)", frame)
			}
		})
	}
}

// TestAllowedCommand_HWDerived_M5b_AcceptsSameFrames is
// TestBuildMWSet_HWDerived_M5b_Accepted's AllowedCommand-side mirror.
func TestAllowedCommand_HWDerived_M5b_AcceptsSameFrames(t *testing.T) {
	tests := []string{
		"MW095011685000+000000510000;",
		"MWP1L007100000+000000110000;",
	}
	for _, frame := range tests {
		t.Run(frame, func(t *testing.T) {
			if !AllowedCommand([]byte(frame)) {
				t.Errorf("AllowedCommand(%q) = false, want true (HW-CONFIRMED accepted)", frame)
			}
		})
	}
}

// TestParseMRAnswer_HWDerived_M5b_P1L pins the live failure frame this
// task's read-side bug fix turns from an abort into a passing vector:
// "MRP1L007100000+000000110000;" — the M5b read-back of the CAT-written
// P1L slot, carrying kind '1' (KindMemory), not kind '5' (KindPMS). At
// the codec level (this test) the frame has always parsed cleanly —
// cat.ParseMRAnswer only checks that P7 is a STRUCTURALLY valid kind
// digit, never bank-specific pairing; the abort lived one layer up, in
// core/driver/ft710's wantKind check (see
// core/driver/ft710/read_test.go's
// TestReadChannel_HWDerived_M5b_PMSKindLeniency for the driver-level
// vector this exact frame's decoded fields feed).
func TestParseMRAnswer_HWDerived_M5b_P1L(t *testing.T) {
	frame := "MRP1L007100000+000000110000;"
	got, err := ParseMRAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMRAnswer(%q): unexpected error: %v", frame, err)
	}
	want := MemoryData{
		Slot:   mustPMSSlot(t, 1, false),
		FreqHz: 7_100_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		Mode:   ModeLSB,
		Kind:   KindMemory,
		CTCSS:  CTCSSOff,
		Shift:  ShiftSimplex,
	}
	if got != want {
		t.Errorf("ParseMRAnswer(%q) = %+v, want %+v", frame, got, want)
	}
}

// TestBuildMTSet_HWDerived_M5b_TagSetAndClear pins two live MT-set
// exchanges: a normal tag set (fire-and-forget accept — see
// core/cat/mt.go) and a tag-CLEAR via an all-spaces 12-byte tag, both
// with the display flag off. CONFIRMED (was ASSUMED): MT set is
// fire-and-forget silent accept; tag-clear via all-spaces MT works. Both
// tags are given SPACE-PADDED to 12 bytes, exactly as the M5b trial
// harness sent them on the wire. (Since the tag-normalisation fix wave,
// BuildMTSet ALSO emits the all-spaces form itself for tag == "" — see
// TestBuildMTSet_HWDerived_ZeroByteFormNeverEmitted below — but a
// NON-EMPTY tag remains variable-length and unpadded per golden vectors
// G8/G9, so these caller-padded 12-byte frames pass through byte-exact
// as ever.)
func TestBuildMTSet_HWDerived_M5b_TagSetAndClear(t *testing.T) {
	tests := []struct {
		name string
		slot Slot
		tag  string
		want string
	}{
		{
			name: "tag set, display off (M-96 trial artefact tag, caller-padded to 12 bytes)",
			slot: mustMemorySlot(t, 96),
			tag:  "TRIAL 96    ", // "TRIAL 96" + 4 trailing spaces = 12 bytes
			want: "MT0960TRIAL 96    ;",
		},
		{
			name: "tag-clear via all-spaces tag",
			slot: mustMemorySlot(t, 96),
			tag:  "            ", // 12 spaces
			want: "MT0960            ;",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := BuildMTSet(tc.slot, false, tc.tag)
			if err != nil {
				t.Fatalf("BuildMTSet(%q, false, %q): unexpected error: %v", tc.slot.Wire(), tc.tag, err)
			}
			if got := string(cmd.Bytes()); got != tc.want {
				t.Errorf("BuildMTSet() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBuildMTSet_HWDerived_ZeroByteFormNeverEmitted pins the 13/07/2026
// tag-clear probes (docs/fixtures-private/m5b-trials.private-capture,
// stages tagclear/tagclear2 — the third production bug of the tag fix
// wave): the 0-byte-tag MT Set "MT0960;" was live-REJECTED ("?;", ~4 ms,
// existing tag SURVIVED), while the all-spaces 12-byte Set
// "MT0960            ;" was live-ACCEPTED and cleared the tag (read
// back all-spaces). BuildMTSet therefore encodes an EMPTY tag as the
// accepted clear form and NEVER emits the rejected 0-byte frame — the
// exact frame the write path used to send for Tag == "", which made
// every tag-clear write abort against the real radio.
func TestBuildMTSet_HWDerived_ZeroByteFormNeverEmitted(t *testing.T) {
	cmd, err := BuildMTSet(mustMemorySlot(t, 96), false, "")
	if err != nil {
		t.Fatalf("BuildMTSet(096, false, \"\"): unexpected error: %v", err)
	}
	got := string(cmd.Bytes())
	if rejected := "MT0960;"; got == rejected {
		t.Fatalf("BuildMTSet(empty tag) = %q — the live-REJECTED 0-byte form must never be emitted", rejected)
	}
	if want := "MT0960            ;"; got != want {
		t.Errorf("BuildMTSet(empty tag) = %q, want %q (the live-ACCEPTED all-spaces clear form)", got, want)
	}
}

// TestBuildMWSet_HWDerived_M5b_Batch9_HighNibbleMode pins a live-accepted
// high-nibble mode write from the M5b SUPPLEMENTARY trials (batch 9,
// 13/07/2026, later the same evening — Codex M5b fix wave, Fix 2, the
// adjudicated remedy for review finding #2, adjudicated HIGH: the first
// evidence pass had no accepted `6`-`F` mode write at all). M-95
// rewritten with mode `F` (DATA-FM-N) — the top of the 15-nibble sweep
// this batch ran end to end (every nibble `1`-`F` accepted, no `?;`) —
// and, since this is the sweep's LAST value, independently
// round-trip-confirmed by the read-back that follows the very next (tag)
// write in the same transcript (docs/hardware-notes.md, "Batch 9").
func TestBuildMWSet_HWDerived_M5b_Batch9_HighNibbleMode(t *testing.T) {
	m := MemoryData{
		Slot:   mustMemorySlot(t, 95),
		FreqHz: 11_685_000,
		Mode:   ModeDATAFMN,
		Kind:   KindMemory,
		CTCSS:  CTCSSOff,
		Shift:  ShiftSimplex,
	}
	cmd, err := BuildMWSet(m)
	if err != nil {
		t.Fatalf("BuildMWSet(%+v): unexpected error: %v", m, err)
	}
	want := "MW095011685000+000000F10000;"
	if got := string(cmd.Bytes()); got != want {
		t.Errorf("BuildMWSet() = %q, want %q", got, want)
	}
}

// TestBuildMTSet_HWDerived_M5b_Batch9_TagBoundariesAndPMS pins two more
// live-accepted MT-set exchanges from the M5b SUPPLEMENTARY trials
// (batch 9): a heavy-punctuation 12-byte tag on M-95 (one of four
// boundary-character sets this batch proved byte-exact — all-`Z`,
// all-`0`, mixed alphanumeric-with-punctuation, and this one, the
// upper-ASCII boundary the first batch never sent), and the PMS pair's
// own tag-set with the display flag ON (P1L, "PMS TAG TEST") — the PMS
// MT-set Codex's review finding #2 found entirely missing from the
// first evidence pass. Both were individually confirmed round-trip via
// the read-back immediately following them in the transcript
// (docs/hardware-notes.md, "Batch 9").
func TestBuildMTSet_HWDerived_M5b_Batch9_TagBoundariesAndPMS(t *testing.T) {
	tests := []struct {
		name    string
		slot    Slot
		display bool
		tag     string
		want    string
	}{
		{
			name:    "heavy punctuation tag, M-95, display off",
			slot:    mustMemorySlot(t, 95),
			display: false,
			tag:     "!#$%&'()*+,-",
			want:    "MT0950!#$%&'()*+,-;",
		},
		{
			name:    "PMS tag-set, P1L, display ON",
			slot:    mustPMSSlot(t, 1, false),
			display: true,
			tag:     "PMS TAG TEST",
			want:    "MTP1L1PMS TAG TEST;",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := BuildMTSet(tc.slot, tc.display, tc.tag)
			if err != nil {
				t.Fatalf("BuildMTSet(%q, %v, %q): unexpected error: %v", tc.slot.Wire(), tc.display, tc.tag, err)
			}
			if got := string(cmd.Bytes()); got != tc.want {
				t.Errorf("BuildMTSet() = %q, want %q", got, tc.want)
			}
		})
	}
}

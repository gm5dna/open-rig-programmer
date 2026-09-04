// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// ft710EquivalentConfig returns a cat.DialectConfig whose every field is
// copied from cat.FT710's own construction (dialect.go), so a dialect built
// from it parses slots, validates MT tags and clarifier values, and checks
// the MW write kind exactly as the production dialect does.
//
// A test that needs a dialect differing from the FT-710 in ONE respect
// mutates one field of a fresh copy and builds — the single difference is
// then the only variable under test. (The ModeNames map is freshly
// allocated on every call, so a caller may mutate it in place.)
func ft710EquivalentConfig() cat.DialectConfig {
	return cat.DialectConfig{
		CATID: "0800",
		ModeNames: map[cat.Mode]string{
			cat.ModeLSB:     "LSB",
			cat.ModeUSB:     "USB",
			cat.ModeCWU:     "CW-U",
			cat.ModeFM:      "FM",
			cat.ModeAM:      "AM",
			cat.ModeRTTYL:   "RTTY-L",
			cat.ModeCWL:     "CW-L",
			cat.ModeDATAL:   "DATA-L",
			cat.ModeRTTYU:   "RTTY-U",
			cat.ModeDATAFM:  "DATA-FM",
			cat.ModeFMN:     "FM-N",
			cat.ModeDATAU:   "DATA-U",
			cat.ModeAMN:     "AM-N",
			cat.ModePSK:     "PSK",
			cat.ModeDATAFMN: "DATA-FM-N",
		},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 501, SixtyHi: 599,
			PMSPairs:      9,
			EmergencyWire: "EMG",
			NoneWire:      "000",
			MCSelects:     cat.MCSelectsAll,
		},
		EXAddressForm: cat.EXAddressTriple,
		MT:            cat.MTPolicy{Form: cat.MTFormShort, ReadSlots: cat.MTReadsReadable, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
		Clarifier:     cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:      cat.P5TxClar,
		MWWriteKind:   cat.KindMemory,
	}
}

// renamedUSBDialect builds an otherwise-FT710-equivalent cat.Dialect whose
// ONLY difference is cat.ModeUSB's display name: "USB-ALT" rather than
// "USB".
//
// It exists to prove buildWriteCommands (and, through it,
// Session.WriteChannel) resolves a channel's Mode string through the
// SESSION's own dialect rather than this package's private modeByName
// table (built from modeTable — see caps.go): a dialect that renames a
// mode is exactly the case NewDialect's name-uniqueness rule was meant to
// protect, and until this task nothing downstream of the write path
// noticed a rename at all.
func renamedUSBDialect(t *testing.T) cat.Dialect {
	t.Helper()
	cfg := ft710EquivalentConfig()
	cfg.ModeNames[cat.ModeUSB] = "USB-ALT" // renamed from FT710's "USB"
	d, err := cat.NewDialect(cfg)
	if err != nil {
		t.Fatalf("cat.NewDialect(renamed-USB config): unexpected error: %v", err)
	}
	return d
}

// TestWriteChannel_ResolvesModeThroughSessionDialect is task 67's
// TDD-first test: a session whose dialect knows cat.ModeUSB under
// "USB-ALT" — a name this package's own modeTable/modeByName have never
// heard of — must still write the channel correctly, because the mode
// lookup goes through the SESSION's dialect. Before write.go routes
// buildWriteCommands' lookup through dialect.ModeByName, it consults this
// package's own modeByName (built from modeTable, independent of any
// session's dialect), which has no "USB-ALT" entry — so the write is
// wrongly refused, and this test fails.
func TestWriteChannel_ResolvesModeThroughSessionDialect(t *testing.T) {
	radio, sess := openSession(t, Simulated)
	sess.dialect = renamedUSBDialect(t)

	ch := writableChannel("010")
	ch.Data.Mode = "USB-ALT"

	res, err := sess.WriteChannel(testCtx(t), ch)
	if err != nil {
		t.Fatalf("WriteChannel with a dialect-only mode name %q: unexpected error: %v (the driver's private modeByName table was consulted instead of the session's own dialect)", ch.Data.Mode, err)
	}
	assertSteps(t, res, wantSteps(true, true, true, true))

	state, ok := radio.SlotState("010")
	if !ok {
		t.Fatalf("SlotState(\"010\"): no state stored after WriteChannel")
	}
	if state.Mode != byte(cat.ModeUSB) {
		t.Errorf("stored Mode = %q, want %q (cat.ModeUSB, resolved via the session's own dialect under its renamed display name)", state.Mode, byte(cat.ModeUSB))
	}
}

// TestModeTable_MatchesFT710Dialect pins modeTable's equivalence to
// cat.FT710 now the write path resolves modes through the session's
// dialect rather than modeByName (built from modeTable — see caps.go's
// doc comment). modeTable stays a deliberate SECOND transcription, kept
// for capability rendering (caps.Modes), but every name it carries must
// still resolve through cat.FT710.ModeByName to the very same cat.Mode:
// otherwise a rendered capability name could silently drift from what the
// write path — now the dialect's own authority — will actually accept.
func TestModeTable_MatchesFT710Dialect(t *testing.T) {
	for _, e := range modeTable {
		mode, ok := cat.FT710.ModeByName(e.name)
		if !ok {
			t.Errorf("cat.FT710.ModeByName(%q): no entry, want modeTable's %v", e.name, e.mode)
			continue
		}
		if mode != e.mode {
			t.Errorf("cat.FT710.ModeByName(%q) = %v, want modeTable's %v", e.name, mode, e.mode)
		}
	}
}

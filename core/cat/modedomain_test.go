// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// TestParseMode_RefusalNamesThisDialectsOwnDomain is the FT-891 Stage 0
// prose fix (spec, Stage 0 close): ParseMode's refusal used to read "want
// '0'-'9' or 'A'-'F'" on EVERY dialect — the FT-710's contiguous nibble
// space, stated as though it were the protocol's — which is simply false for
// a radio whose mode legend has a hole.
//
// The FT-710's own text is unchanged BYTE FOR BYTE, which is what keeps
// core/cat/testdata/parser-corpus.golden's three ParseMode rows where they
// are; a dialect with a hole gets its own domain instead of the FT-710's.
func TestParseMode_RefusalNamesThisDialectsOwnDomain(t *testing.T) {
	// The FT-710: the exact string this refusal has always carried.
	_, err := FT710.ParseMode('G')
	if err == nil {
		t.Fatal("FT710.ParseMode('G') succeeded")
	}
	const ft710Text = "invalid mode code: want '0'-'9' or 'A'-'F'"
	if !strings.Contains(err.Error(), ft710Text) {
		t.Errorf("FT710.ParseMode('G') = %q, want it to contain %q — this text is pinned in parser-corpus.golden and must not move", err, ft710Text)
	}

	// A dialect whose mode table has a HOLE, which is the FT-891's shape:
	// 1-9 with nothing at 'A', then B, C, D. The old sentence would have
	// told a user to send 'A', 'E' or 'F' — bytes this radio rejects.
	holed := mustFixtureDialect(DialectConfig{
		CATID: "0891",
		ModeNames: map[Mode]string{
			ModeUnset: "-",
			Mode('1'): "LSB", Mode('2'): "USB", Mode('3'): "CW",
			Mode('B'): "FM-N", Mode('C'): "DATA-USB", Mode('D'): "AM-N",
		},
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 99, PMSPairs: 9,
			NoneWire: "000", MCSelects: MCSelectsMemoryPMS,
		},
		EXAddressForm: EXAddressPair, // the FT-891's shape throughout, as the comment above says
		MT: MTPolicy{
			Form: MTFormShort, ReadSlots: MTReadsMemoryPMS,
			TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' ',
		},
		Clarifier:   ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:    P5Fixed,
		MWWriteKind: KindMemory,
	})

	for _, c := range []byte{'A', 'E', 'F'} {
		_, err := holed.ParseMode(c)
		if err == nil {
			t.Fatalf("the holed dialect parsed %q, which is not in its table", c)
		}
		if strings.Contains(err.Error(), "'A'-'F'") {
			t.Errorf("ParseMode(%q) on a dialect with a hole reads %q — it is naming the FT-710's mode space, and the byte it tells the user to send is one this radio rejects", c, err)
		}
	}
	got := holed.modeDomainText()
	const want = "'0'-'3' or 'B'-'D'"
	if got != want {
		t.Errorf("modeDomainText() = %q, want %q — the '0'-'3' run is ModeUnset plus LSB/USB/CW, and the hole at 'A' must show", got, want)
	}

	// A single-byte table renders as one quoted byte, not a degenerate range.
	single := mustFixtureDialect(DialectConfig{
		CATID:     "0892",
		ModeNames: map[Mode]string{ModeUSB: "USB"},
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 9, PMSPairs: 0,
			NoneWire: "000", MCSelects: MCSelectsAll,
		},
		EXAddressForm: EXAddressTriple, // not this fixture's axis: the one-mode table is
		MT: MTPolicy{
			Form: MTFormShort, ReadSlots: MTReadsReadable,
			TagMaxBytes: 4, ClearTagByte: ' ',
		},
		Clarifier:   ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:    P5TxClar,
		MWWriteKind: KindMemory,
	})
	if got, want := single.modeDomainText(), "'2'"; got != want {
		t.Errorf("modeDomainText() on a one-mode dialect = %q, want %q", got, want)
	}

	// The zero Dialect declares no modes at all, and must say so rather than
	// leaving "want " followed by nothing.
	var zero Dialect
	if _, err := zero.ParseMode('2'); err == nil {
		t.Error("the zero Dialect parsed a mode")
	} else if !strings.Contains(err.Error(), "declares none") {
		t.Errorf("the zero Dialect's refusal reads %q — a dialect with no modes must say so", err)
	}
}

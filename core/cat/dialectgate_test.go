// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// The property in this file is the one that would have caught the defect
// the spec review found: a caller-built dialect emitting a frame that this
// program's own gate then approved, containing a byte no CAT reference
// documents (a NUL mode nibble, an emergency wire of "\x00AB").
//
// Two things about revision 1 of this test were wrong, and both are worth
// stating because either alone made it worthless:
//
//  1. It drove only the EXISTING fixtures, all of which use safe printable
//     bytes. Deleting the wire-domain rules from NewDialect would have left
//     it green. A property about what a HOSTILE config can do must build
//     hostile configs.
//
//  2. It asserted "no frame contains a byte outside the domain", which is
//     LITERALLY FALSE for every frame: the domain excludes ';' precisely
//     because ';' terminates a frame, and every CAT frame ends with one.
//     The interior and the terminator are separate claims.

// adversarialConfigs returns configs that probe the wire-byte domain from
// every field a dialect can put bytes on the wire from.
//
// Each is expected to be REFUSED. The test asserts refusal, and then —
// crucially — separately asserts that the accepted configs really do
// produce clean frames, so this cannot pass by NewDialect refusing
// everything.
func adversarialConfigs() []struct {
	name   string
	mutate func(*DialectConfig)
} {
	return []struct {
		name   string
		mutate func(*DialectConfig)
	}{
		{"NUL mode key", func(c *DialectConfig) { c.ModeNames[Mode(0x00)] = "NUL" }},
		{"DEL mode key", func(c *DialectConfig) { c.ModeNames[Mode(0x7F)] = "DEL" }},
		{"high-byte mode key", func(c *DialectConfig) { c.ModeNames[Mode(0xFF)] = "HIGH" }},
		{"terminator mode key", func(c *DialectConfig) { c.ModeNames[Mode(';')] = "SEMI" }},
		{"NUL in emergency wire", func(c *DialectConfig) { c.Slots.EmergencyWire = "\x00AB" }},
		{"terminator in emergency wire", func(c *DialectConfig) { c.Slots.EmergencyWire = "A;B" }},
		{"terminator in none wire", func(c *DialectConfig) { c.Slots.NoneWire = "8;8" }},
		{"NUL in CATID", func(c *DialectConfig) { c.CATID = "08\x000" }},
		{"terminator in CATID", func(c *DialectConfig) { c.CATID = "08;0" }},
		{"NUL clear-tag byte", func(c *DialectConfig) { c.MT.ClearTagByte = 0x00 }},
		{"terminator clear-tag byte", func(c *DialectConfig) { c.MT.ClearTagByte = ';' }},
		{"high-byte clear-tag byte", func(c *DialectConfig) { c.MT.ClearTagByte = 0xA0 }},
	}
}

// TestNewDialect_RefusesEveryAdversarialWireByteConfig drives the hostile
// half.
func TestNewDialect_RefusesEveryAdversarialWireByteConfig(t *testing.T) {
	cases := adversarialConfigs()
	if len(cases) == 0 {
		t.Fatal("no adversarial configs — the property would run vacuously")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaselineConfig()
			tc.mutate(&cfg)
			d, err := NewDialect(cfg)
			if err == nil {
				t.Fatalf("NewDialect accepted a config with %s — it could emit that byte into a frame its own gate then admits", tc.name)
			}
			if d.Configured() {
				t.Error("the refused config still returned a Configured dialect")
			}
		})
	}
}

// TestEveryDialect_BuiltFramesAreCleanAndGateAdmissible is the positive
// half, and the one that makes the refusals above meaningful.
//
// For every dialect the package can see, every frame its builders produce
// must:
//
//   - contain only permitted bytes in its INTERIOR;
//   - end with exactly one ';' and contain no other;
//   - be admitted by that dialect's OWN gate.
//
// The last is the real safety property. A builder and a gate that disagree
// are worse than either being wrong alone: it means the program cannot send
// a command it believes is valid, or — in the other direction — the gate is
// not actually checking what the builder emits.
func TestEveryDialect_BuiltFramesAreCleanAndGateAdmissible(t *testing.T) {
	totalFrames := 0

	for _, nd := range allTestDialects() {
		perDialect := 0

		build := func(what string, frame []byte, err error) {
			t.Helper()
			if err != nil {
				return // a dialect legitimately refusing to build is not this test's business
			}
			perDialect++
			totalFrames++

			if len(frame) == 0 {
				t.Errorf("%s: %s produced an empty frame", nd.name, what)
				return
			}
			// Terminator: exactly one, at the end.
			if frame[len(frame)-1] != ';' {
				t.Errorf("%s: %s frame %q does not end with ';'", nd.name, what, frame)
			}
			if n := strings.Count(string(frame), ";"); n != 1 {
				t.Errorf("%s: %s frame %q contains %d semicolons, want exactly 1 — a second one splits this into two commands on the wire", nd.name, what, frame, n)
			}
			// Interior: every byte in the permitted domain.
			for i, b := range frame[:len(frame)-1] {
				if !validWireByte(b) {
					t.Errorf("%s: %s frame %q has byte %#02x at offset %d, outside the permitted interior domain", nd.name, what, frame, b, i)
				}
			}
			// The dialect's own gate must admit its own builder's output.
			if !nd.dia.AllowedCommand(frame) {
				t.Errorf("%s: its own gate refused %s frame %q", nd.name, what, frame)
			}
		}

		// Fixed-form builders every dialect has.
		build("ID read", nd.dia.BuildIDRead().Bytes(), nil)
		build("AI set", nd.dia.BuildAISet(false).Bytes(), nil)
		build("MC read", nd.dia.BuildMCRead().Bytes(), nil)

		// Slot-bearing builders, over every slot this dialect classifies.
		for n := 0; n <= 999; n++ {
			slot, err := nd.dia.ParseSlot(threeDigits(n))
			if err != nil {
				continue
			}
			if cmd, err := nd.dia.BuildMRRead(slot); err == nil {
				build("MR read", cmd.Bytes(), nil)
			}
			if cmd, err := nd.dia.BuildMTRead(slot); err == nil {
				build("MT read", cmd.Bytes(), nil)
			}
			if cmd, err := nd.dia.BuildMCSet(slot); err == nil {
				build("MC set", cmd.Bytes(), nil)
			}
			// A tag write, including the CLEAR form, which is where a
			// dialect's own ClearTagByte reaches the wire.
			if cmd, err := nd.dia.BuildMTSet(slot, false, ""); err == nil {
				build("MT set (cleared tag)", cmd.Bytes(), nil)
			}
			if cmd, err := nd.dia.BuildMTSet(slot, false, "AB"); err == nil {
				build("MT set (tagged)", cmd.Bytes(), nil)
			}
		}

		// EX reads, over this dialect's own inventory.
		for _, a := range nd.dia.EXAddresses() {
			if cmd, err := nd.dia.BuildEXRead(a); err == nil {
				build("EX read", cmd.Bytes(), nil)
			}
		}

		// NON-VACUITY, per dialect. A property that ran zero times for a
		// dialect passes in silence, which is exactly how a fixture with a
		// mistyped slot space would go unnoticed.
		if perDialect == 0 {
			t.Errorf("%s: no frames were built — this dialect contributed nothing and the property passed vacuously for it", nd.name)
		}
	}

	if totalFrames < 100 {
		t.Errorf("only %d frames checked across all dialects; expected hundreds — the walk is not reaching the builders", totalFrames)
	}
	t.Logf("checked %d built frames across %d dialects", totalFrames, len(allTestDialects()))
}

func threeDigits(n int) string {
	return string([]byte{byte('0' + n/100), byte('0' + (n/10)%10), byte('0' + n%10)})
}

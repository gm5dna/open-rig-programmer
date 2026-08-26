// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	civic905 "github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// writeTrialsComplete is FALSE for the IC-905, on the IC-905's own
// evidence: no IC-905 has ever been asked anything by this project, no
// 1A 00 set has ever been sent to one, no FB has ever been observed
// from one, and every byte of every golden vector is
// documentation-derived (matrix section 3.14). This model has NO
// registered sibling, so there is no sibling FALSE to share or be
// confused with: one row, one model, one registry.
//
// BOTH HALVES, so flipping the constant alone unlocks nothing.
func TestWriteTrialsComplete_PinnedFalse(t *testing.T) {
	if writeTrialsComplete {
		t.Fatal("writeTrialsComplete is true — no IC-905 has ever been written to")
	}
	caps := New(RealHardware).Capabilities()
	for _, b := range caps.Banks {
		for f := range b.Fields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("bank %s field %s is writable in the real-hardware profile", b.ID, f)
			}
		}
	}
}

// tierFieldsMustBeEmpty names the SIX spec.Capabilities fields this
// matrix grades as a WRITTEN-DOWN ZERO for this radio, each with the row
// that grades it. Empty is the decision; populating one would be the
// mistake, so the rule is inverted for these rather than waived.
//
// REV 1 said "five" over a list of six (Fable F11(a)); there are six.
var tierFieldsMustBeEmpty = map[string]string{
	"ClarMaxHz":     "matrix section 1 row 6 — the 1A 00 record carries no clarifier/RIT field",
	"ClarStepHz":    "matrix section 1 row 7 — the same absence, graded separately because they are separate struct fields",
	"CTCSSTones":    "matrix section 1 row 8 — the tone is a BCD frequency, not an index into a chart; the numeric CTCSSToneRange carries it instead",
	"RequiredSlots": "matrix section 1 row 13 — this radio's non-clearable set is a whole bank (Bank.NoBlank on CALL), not an individual-slot list",
	"ShiftOptions":  "matrix section 1 row 14 — superseded by DuplexOptions; the two vocabularies never coexist on one model (D4)",
	"CTCSSStates":   "matrix section 1 row 15 — superseded by ToneModes, whose vocabulary has eight values, not three",
}

// TestCapabilities_EveryFieldExplicit reflects over spec.Capabilities and
// requires that no field is left at its zero value by accident. A zero in
// a capability field is never neutral: a zero MaxFreqHz reads as "no
// ceiling" to every validator, a zero TagLen makes csvio's CHIRP import
// truncate every name to nothing, and a non-positive baud reaches
// transport.OpenSerial's silent substitution.
//
// The six fields tierFieldsMustBeEmpty names are inverted rather than
// waived: they must be EMPTY, and the test fails if one is ever filled
// in.
func TestCapabilities_EveryFieldExplicit(t *testing.T) {
	// 22 since Wave 2.5's E3 added CTCSSToneRange.
	const wantFieldCount = 22

	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", capabilitiesUnverified()},
		{"Simulated", capabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.caps)
			typ := v.Type()
			if typ.NumField() != wantFieldCount {
				t.Fatalf("spec.Capabilities has %d fields, this test knows %d — a field was added or removed and this driver must decide about it explicitly, not inherit a zero", typ.NumField(), wantFieldCount)
			}
			for i := 0; i < typ.NumField(); i++ {
				name := typ.Field(i).Name
				f := v.Field(i)
				if why, inverted := tierFieldsMustBeEmpty[name]; inverted {
					if !f.IsZero() || (f.Kind() == reflect.Slice && f.Len() != 0) {
						t.Errorf("field %s is populated — empty is the decision here (%s)", name, why)
					}
					continue
				}
				if f.IsZero() {
					t.Errorf("field %s is the zero value — every spec.Capabilities field must be populated explicitly (see this test's doc comment for why zero is never neutral)", name)
					continue
				}
				if f.Kind() == reflect.Slice && f.Len() == 0 {
					t.Errorf("field %s is an empty (but non-nil) slice — populated means populated", name)
				}
			}
		})
	}
}

// TestProfiles_Validate runs spec.Capabilities.Validate over both static
// profiles. It covers E5's canonical rule directly: this model expresses
// FOUR duplex values over three directions (OFF and RPS share
// DuplexOff) and EIGHT tone modes over five semantics (four of them
// Cross), so both vocabularies carry a duplicated semantic and each must
// name exactly one canonical entry.
func TestProfiles_Validate(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", capabilitiesUnverified()},
		{"Simulated", capabilitiesSimulated()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.caps.Validate(); err != nil {
				t.Fatalf("Validate: %v", err)
			}
		})
	}
}

// TestToneDomain_AdmitsEveryToneThisRecordCanCarry pins E3 END TO END —
// through codeplug.ToneField.Valid, not by inspecting the literal.
//
// 885 is the load-bearing case: it is the 88.5 Hz both golden vectors
// carry, and under REV 1's CTCSSTones: nil it failed CLOSED, so every
// channel this driver reads with a tone would have failed validation
// (Codex 5, Fable F1).
//
// ZERO MUST FAIL. T1(2) says zero is not a tone; the declared range
// starts at 1, and spec.Validate refuses MinDeciHz <= 0 outright. The
// READ MAPPING, not the capability, is what handles a zero on the wire
// (Task 13 step 2b).
func TestToneDomain_AdmitsEveryToneThisRecordCanCarry(t *testing.T) {
	caps := capabilitiesUnverified()

	for _, v := range []spec.Tone{1, 885, 2999} {
		f := codeplug.ToneField{State: codeplug.Known, Value: v}
		if err := f.Valid(caps); err != nil {
			t.Errorf("a Known tone of %d deciHz is refused: %v — the printed digit range reaches 0..2999 tenths (PDF p.24, folio 23)", int(v), err)
		}
	}
	for _, v := range []spec.Tone{0, 3000} {
		f := codeplug.ToneField{State: codeplug.Known, Value: v}
		if err := f.Valid(caps); err == nil {
			t.Errorf("a Known tone of %d deciHz is ADMITTED — 0 is not a tone (T1(2)) and 3000 is past the printed 100 Hz digit's ceiling of 2 (R11 forbids widening)", int(v))
		}
	}
}

// TestDTCSCodes_AreThe512PrintedCodesAndNothingWider pins the explicit
// table E3 keeps for DTCS, and with it the primary gate for OQ-6's
// digit-range rule: the codes are three octal digits (PDF p.24, folio
// 23, "DTCS code and polarity setting"), so 512 of them, every decimal
// digit 7 or less.
func TestDTCSCodes_AreThe512PrintedCodesAndNothingWider(t *testing.T) {
	caps := capabilitiesUnverified()
	codes := caps.DTCSCodes

	if len(codes) != 512 {
		t.Fatalf("DTCSCodes has %d entries, want 512 (three octal digits)", len(codes))
	}
	if codes[0] != 0 {
		t.Errorf("DTCSCodes[0] = %d, want 0 (code 000)", codes[0])
	}
	if codes[len(codes)-1] != 777 {
		t.Errorf("DTCSCodes[last] = %d, want 777", codes[len(codes)-1])
	}
	for i, c := range codes {
		if i > 0 && c <= codes[i-1] {
			t.Fatalf("DTCSCodes is not strictly ascending at index %d: %d after %d", i, c, codes[i-1])
		}
		for n := c; n > 0; n /= 10 {
			if n%10 > 7 {
				t.Fatalf("DTCSCodes[%d] = %d has a digit above 7 — the printed ranges are 0 ~ 7 (PDF p.24, folio 23)", i, c)
			}
		}
	}

	// And codeplug.Validate is the primary gate: a Known dtcs_code of
	// 778 is refused BEFORE this driver is reached.
	if err := (codeplug.IntField{State: codeplug.Known, Value: 778}).Valid(codes); err == nil {
		t.Error("a Known dtcs_code of 778 is admitted by the table — the 0-7 digit rule must be enforced by the table itself, not only by the driver's defence-in-depth re-check")
	}
}

// TestProfilesNeverEmitConsented pins that neither STATIC profile carries
// spec.ConsentedUnverified. Consent is a statement about a SESSION;
// internal/wiring reads the static set to decide whether consent is
// needed at all, and the registry refuses a static profile that already
// claims it.
func TestProfilesNeverEmitConsented(t *testing.T) {
	for _, tt := range []struct {
		name string
		caps spec.Capabilities
	}{
		{"Unverified", capabilitiesUnverified()},
		{"Simulated", capabilitiesSimulated()},
		{"RealHardware via New", New(RealHardware).Capabilities()},
		{"Simulated via New", New(Simulated).Capabilities()},
		{"unrecognised Profile via New", New(Profile(99)).Capabilities()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, b := range tt.caps.Banks {
				for f, fs := range b.Fields {
					if fs.Read == spec.ConsentedUnverified || fs.Write == spec.ConsentedUnverified {
						t.Errorf("bank %s field %s carries ConsentedUnverified in a STATIC profile", b.ID, f)
					}
				}
			}
		})
	}
}

// TestBanks_ShapeAndSparseDescriptors pins the two banks and their
// shapes, and the absence of the four this model does not have.
//
// MEM's Slots is EMPTY in the static baseline: it is a sparse bank whose
// materialised set is discovered at Open (Task 13). CALL's twelve are
// static, spelled in their OWN namespace ("C01".."C12", ruling R4).
func TestBanks_ShapeAndSparseDescriptors(t *testing.T) {
	caps := capabilitiesUnverified()

	if len(caps.Banks) != 2 {
		t.Fatalf("this model has %d banks, want 2 (MEM and CALL) — no SCAN (matrix section 1b: a swept, MANUAL-EVIDENCED absence), no PMS, no 60M, no EMG", len(caps.Banks))
	}
	for _, absent := range []spec.BankID{spec.BankScan, spec.BankPMS, spec.Bank60m, spec.BankEMG} {
		if _, ok := caps.Bank(absent); ok {
			t.Errorf("bank %s is present — this model has no such bank (matrix section 1b)", absent)
		}
	}

	mem, ok := caps.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("no MEM bank")
	}
	if !mem.Sparse || mem.Groups != 100 || mem.PerGroup != 100 || mem.Budget != 500 {
		t.Errorf("MEM = {Sparse:%v Groups:%d PerGroup:%d Budget:%d}, want {true 100 100 500} (matrix section 1b; the budget is ASSUMED, ic905.group_budget, lift ic905-R-09)",
			mem.Sparse, mem.Groups, mem.PerGroup, mem.Budget)
	}
	if mem.NoBlank {
		t.Error("MEM NoBlank is true — the clear form explicitly admits memory groups 00 00 ~ 00 99 (PDF p.19, folio 18)")
	}
	if len(mem.Slots) != 0 {
		t.Errorf("MEM has %d static slots — a sparse bank's materialised set is DISCOVERED at Open, never asserted statically", len(mem.Slots))
	}

	call, ok := caps.Bank(spec.BankCall)
	if !ok {
		t.Fatal("no CALL bank")
	}
	if call.Sparse || call.Groups != 0 || call.PerGroup != 0 || call.Budget != 0 {
		t.Errorf("CALL = {Sparse:%v Groups:%d PerGroup:%d Budget:%d}, want a dense bank with all three descriptors zero", call.Sparse, call.Groups, call.PerGroup, call.Budget)
	}
	if !call.NoBlank {
		t.Error(`CALL NoBlank is false — the clear form prints "You cannot specify group \"01 00\" (Call channel group)" (PDF p.19, folio 18)`)
	}
	want := []string{"C01", "C02", "C03", "C04", "C05", "C06", "C07", "C08", "C09", "C10", "C11", "C12"}
	if !slices.Equal(call.Slots, want) {
		t.Errorf("CALL Slots = %v, want %v — twelve channels, two per band over six bands (PDF p.19, folio 18)", call.Slots, want)
	}
}

// fieldGrid is matrix section 2's eighty gradings, transcribed: twenty
// spec.Fields, each with whether MEM and CALL express it. The matrix
// grades MEM and CALL IDENTICALLY on every one of the twenty (the CALL
// column's one documented difference is byte (5)'s value constraint,
// which is not a support difference), so one column of booleans carries
// both banks and the test asserts them separately.
var fieldGrid = []struct {
	field     spec.Field
	expressed bool
	row       string
}{
	{spec.FieldFrequency, true, "row 1 — bytes (6)~(10)"},
	{spec.FieldMode, true, "row 2 — byte (11)"},
	{spec.FieldClarifier, false, "row 3 — MANUAL-EVIDENCED zero: no clarifier/RIT field among the record's eighteen indices"},
	{spec.FieldCTCSSState, false, "row 4 — CHOICE zero, superseded by tone_mode: the record's tone-state vocabulary has eight values, not three"},
	{spec.FieldCTCSSTone, false, "row 5 — CHOICE zero, superseded by tone_tx/tone_rx: the tone is a BCD frequency, not a table index"},
	{spec.FieldShift, false, "row 6 — CHOICE zero, superseded by duplex: the record expresses four duplex values including RPS"},
	{spec.FieldTag, true, "row 7 — bytes 53~68"},
	{spec.FieldTagDisplay, false, "row 8 — CHOICE zero: one writable name field, no separate display-only tag"},
	{spec.FieldScanSkip, false, "row 9 — CHOICE zero: byte (5)'s star nibble is SELECT-group membership and is NEVER mapped as skip on an Icom"},
	{spec.FieldErase, false, "row 10 — CHOICE zero, tier-wide: no clear builder, no clear frame, and ConsentUnverifiedWrites structurally never consents erase"},
	{spec.FieldTxFrequency, false, "row 11 — MANUAL-EVIDENCED zero: no TX frequency field and no duplicated TX block"},
	{spec.FieldDuplex, true, "row 12 — byte (14) high nibble"},
	{spec.FieldOffset, true, "row 13 — bytes (26)~(28)"},
	{spec.FieldToneMode, true, "row 14 — byte (14) low nibble"},
	{spec.FieldToneTx, true, "row 15 — bytes (16)~(18); block assignment ASSUMED, ic905.tone_block_assignment, lift ic905-R-07"},
	{spec.FieldToneRx, true, "row 16 — bytes (19)~(21); same single assumption, same single lift"},
	{spec.FieldDTCSCode, true, "row 17 — within (22)~(24)"},
	{spec.FieldDTCSPolarity, true, "row 18 — byte (22); nibble assignment ASSUMED, ic905.dtcs_polarity_nibbles, lift ic905-R-08"},
	{spec.FieldFilter, true, "row 19 — byte (12)"},
	{spec.FieldDataMode, true, "row 20 — byte (13)"},
}

// TestFieldGrid_MatchesTheMatrix compares matrix section 2's eighty
// gradings against caps.FieldSupport, for both banks and both profiles.
//
// FieldErase is zero in BOTH profiles, and so are scan_skip, shift,
// ctcss_state, ctcss_tone, clarifier, tag_display and tx_frequency —
// eight written-down zeros, each named in fieldGrid with its row.
func TestFieldGrid_MatchesTheMatrix(t *testing.T) {
	if len(fieldGrid) != 20 {
		t.Fatalf("fieldGrid has %d rows, want the twenty spec.Fields this project models", len(fieldGrid))
	}

	for _, prof := range []struct {
		name  string
		caps  spec.Capabilities
		grade spec.Support
	}{
		{"Unverified", capabilitiesUnverified(), spec.Unverified},
		{"Simulated", capabilitiesSimulated(), spec.Supported},
	} {
		for _, bank := range []spec.BankID{spec.BankMemory, spec.BankCall} {
			for _, row := range fieldGrid {
				t.Run(prof.name+"/"+string(bank)+"/"+string(row.field), func(t *testing.T) {
					want := spec.FieldSupport{}
					if row.expressed {
						want = spec.FieldSupport{Read: prof.grade, Write: prof.grade}
					}
					if got := prof.caps.FieldSupport(bank, row.field); got != want {
						t.Errorf("FieldSupport(%s, %s) = %+v, want %+v (matrix section 2 %s)", bank, row.field, got, want, row.row)
					}
				})
			}
		}
	}
}

// TestFrequencyRangeReachesTenPointFiveGigahertz pins the value that
// forced D4's uint64 widening: 10.5 GHz is about 2.4 times uint32's
// ceiling, so the pre-tier struct could not hold this radio's ceiling at
// all (matrix section 1 rows 11-12; PDF p.20, folio 19, band stacking
// register, row 06 | 10G | 10000.000000 ~ 10500.000000).
func TestFrequencyRangeReachesTenPointFiveGigahertz(t *testing.T) {
	caps := capabilitiesUnverified()
	if caps.MinFreqHz != 144_000_000 {
		t.Errorf("MinFreqHz = %d, want 144000000 — ASSUMED as a memory-record limit: ic905.min_storable_hz, lift ic905-R-05", caps.MinFreqHz)
	}
	if caps.MaxFreqHz != 10_500_000_000 {
		t.Errorf("MaxFreqHz = %d, want 10500000000 — ASSUMED likewise: ic905.max_storable_hz, lift ic905-R-06", caps.MaxFreqHz)
	}
	if caps.MaxFreqHz <= 4_294_967_295 {
		t.Error("MaxFreqHz fits a uint32 — this driver's whole reason for D4's widening has gone missing")
	}
}

// TestTagCharset_IsTheProfilesOwn pins that the name charset the
// capability data advertises is the one the CI-V profile's own layout
// accepts: derived from civ.Profile.NameCharset rather than restated, so
// a charset the codec would refuse can never be offered to a user.
func TestTagCharset_IsTheProfilesOwn(t *testing.T) {
	caps := capabilitiesUnverified()
	want := string(civic905.Profile().NameCharset())
	if caps.TagCharset != want {
		t.Fatalf("TagCharset = %q, want the profile's own %q", caps.TagCharset, want)
	}
	if len(caps.TagCharset) != 95 {
		t.Errorf("TagCharset holds %d bytes, want 95 (PDF p.20, folio 19, plus the ASSUMED 0x20 — ic905.name_pad_byte, lift ic905-R-17)", len(caps.TagCharset))
	}
	if !strings.Contains(caps.TagCharset, " ") {
		t.Error("TagCharset has no space — the pad byte must be a charset member (civ V4)")
	}
	if caps.TagLen != 16 {
		t.Errorf(`TagLen = %d, want 16 (PDF p.19: "53~68: Memory name setting (16 characters, fixed)")`, caps.TagLen)
	}
}

// TestModel_IsTheProfilesOwn pins that this driver names the same radio
// its dialect does — one source, so a registry key cannot drift from a
// profile.
func TestModel_IsTheProfilesOwn(t *testing.T) {
	if got := New(RealHardware).Model(); got != civic905.Model {
		t.Errorf("Model() = %q, want %q", got, civic905.Model)
	}
	if got := New(RealHardware).Capabilities().Model; got != civic905.Model {
		t.Errorf("Capabilities().Model = %q, want %q — Driver.Model must equal Capabilities().Model", got, civic905.Model)
	}
}

// TestConsent_TransformsOnlyTheSessionSet.
//
// Consent transforms the SESSION's capabilities, once, at assembly —
// never the static Capabilities(), which is what internal/wiring reads to
// decide whether consent is needed at all. If the option reached the
// static set, the app would stop asking.
func TestConsent_TransformsOnlyTheSessionSet(t *testing.T) {
	t.Parallel()
	p := newRespondingPort(t, occupiedAt(wireAddr{0, 0}))
	drv := New(RealHardware, WithConsentedUnverifiedWrites())

	// The STATIC set is untouched: nothing writable, nothing consented.
	static := drv.Capabilities()
	for _, b := range static.Banks {
		for f, fs := range b.Fields {
			if fs.Write == spec.ConsentedUnverified {
				t.Errorf("static bank %s field %s is ConsentedUnverified — the option must not reach the registry's own view", b.ID, f)
			}
			if fs.CanWrite() {
				t.Errorf("static bank %s field %s is writable", b.ID, f)
			}
		}
	}

	sess, err := drv.Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	// The SESSION's set carries the consent, and only on the write side.
	caps := sess.Capabilities()
	if !caps.FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Error("a consented session cannot write frequency — the transform did not reach the session's set")
	}
	for _, b := range caps.Banks {
		for f, fs := range b.Fields {
			if fs.Read == spec.ConsentedUnverified {
				t.Errorf("bank %s field %s carries ConsentedUnverified on the READ side, which is a write-only state", b.ID, f)
			}
		}
	}

	// And an UNCONSENTED session of the same profile still cannot write.
	p2 := newRespondingPort(t, occupiedAt(wireAddr{0, 0}))
	plain, err := New(RealHardware).Open(context.Background(), p2.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = plain.Close() })
	if plain.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Error("an UNCONSENTED real-hardware session can write — consent must be the only route past the fail-safe")
	}
}

// TestConsent_NeverConsentsErase proves the structural exclusion on THIS
// driver's own profile rather than trusting the transform's own test.
// spec.ConsentUnverifiedWrites never touches spec.FieldErase, so
// populated-to-empty stays blocked whatever a profile's labels say — and
// this tier ships no erase path in any case.
func TestConsent_NeverConsentsErase(t *testing.T) {
	t.Parallel()
	consented := spec.ConsentUnverifiedWrites(capabilitiesUnverified())
	for _, bank := range []spec.BankID{spec.BankMemory, spec.BankCall} {
		fs := consented.FieldSupport(bank, spec.FieldErase)
		if fs.CanWrite() {
			t.Errorf("bank %s FieldErase is writable after consent: %+v", bank, fs)
		}
		if fs != (spec.FieldSupport{}) {
			t.Errorf("bank %s FieldErase = %+v after consent, want the zero FieldSupport it started as", bank, fs)
		}
	}
}

// TestConsent_RefusesAnUnrecognisedProfile: a profile the Capabilities
// switch would fail safe on must not have consent drift open on it.
// profileRecognised restates the declared set for exactly that reason.
func TestConsent_RefusesAnUnrecognisedProfile(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		profile    Profile
		recognised bool
	}{
		{"RealHardware", RealHardware, true},
		{"Simulated", Simulated, true},
		{"an unrecognised value", Profile(99), false},
		{"a negative value", Profile(-1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, ok := New(tt.profile, WithConsentedUnverifiedWrites()).(*ic905Driver)
			if !ok {
				t.Fatal("New did not return an *ic905Driver")
			}
			if got := d.profileRecognised(); got != tt.recognised {
				t.Errorf("profileRecognised() = %v, want %v", got, tt.recognised)
			}
			caps := d.sessionCapabilities(nil, "AC:94")
			writable := caps.FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite()
			if writable != tt.recognised {
				t.Errorf("a consented %s session is writable = %v, want %v — an unrecognised profile stays on the untransformed fail-safe even WITH the option", tt.name, writable, tt.recognised)
			}
		})
	}
}

// TestDriver_ReportsOneStopBit (E2, ruling R2).
//
// Every Icom driver in the tier reports 1, as its own ASSUMED register
// entry with its own named per-model lift (spec D3.1). The consequence of
// NOT implementing the reporter is concrete and testable: internal/wiring
// would fall back to transport.DefaultStopBits, which is 2 — the Yaesu
// value, on a radio this tier assumes speaks 8-N-1.
func TestDriver_ReportsOneStopBit(t *testing.T) {
	t.Parallel()
	for _, profile := range []Profile{RealHardware, Simulated, Profile(99)} {
		r, ok := New(profile).(driver.SerialFramingReporter)
		if !ok {
			t.Fatalf("the driver for profile %d does not implement driver.SerialFramingReporter — internal/wiring would open the port at transport.DefaultStopBits, which is %d", profile, transport.DefaultStopBits)
		}
		if got := r.StopBits(); got != 1 {
			t.Errorf("StopBits() = %d, want 1 — ASSUMED, D5 entry 8, lift ic905-R-10", got)
		}
	}
	if transport.DefaultStopBits == 1 {
		t.Error("transport.DefaultStopBits is 1, so this reporter no longer changes anything — the test's own premise has moved and the register entry needs re-reading")
	}
}

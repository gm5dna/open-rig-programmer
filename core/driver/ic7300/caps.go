// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300

import (
	"fmt"

	ic7300civ "github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The bank labels this driver publishes. Two banks, and only two: MEM and
// SCAN. There is no CALL bank, no PMS pair and no group addressing on this
// model — all MANUAL-EVIDENCED absences (matrix §1b), and all recorded here
// as banks that do not exist rather than as banks left out.
const (
	bankMemoryLabel = "Memories"
	bankScanLabel   = "Scan edges (P1/P2)"
)

// writeTrialsComplete is FALSE, and it is the whole write guard.
//
// NO IC-7300 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Matrix §3.14
// states it for this model alone — "The registered sibling's FALSE is not
// stated here" is the MK2's mirror of the same sentence — so no write trial
// has completed, and the RealHardware profile therefore grades every field
// Unverified, which CanWrite() refuses. Consent
// (spec.ConsentUnverifiedWrites, applied only when the user passed
// WithConsentedUnverifiedWrites) is the ONE key that opens the gate on real
// hardware today, and it opens it for the caller's own explicitly accepted
// risk, never for this table's authority.
//
// Flipping it is a HARDWARE milestone with evidence, per model. The pin in
// caps_test.go names what a flip must be accompanied by.
const writeTrialsComplete = false

// Profile selects which capability description a driver value publishes.
//
// Two profiles, mirroring core/driver/ftdx101's: the fail-safe one a real
// radio gets, and the one the fake gets. The ZERO VALUE IS THE SAFE ONE —
// RealHardware — so a caller that forgets to choose gets the profile that
// writes nothing, not the profile that writes everything.
type Profile int

const (
	// RealHardware is the fail-safe profile: every field this record
	// carries is graded Unverified, which is unwritable, because no
	// IC-7300 has ever been asked anything (writeTrialsComplete).
	RealHardware Profile = iota
	// Simulated is the profile a fake radio gets: the same fields graded
	// Supported, so the write choreography is exercisable without a
	// consent flag and without ever touching hardware.
	Simulated
)

// String renders the profile for diagnostics.
func (p Profile) String() string {
	switch p {
	case RealHardware:
		return "RealHardware"
	case Simulated:
		return "Simulated"
	default:
		return fmt.Sprintf("Profile(%d)", int(p))
	}
}

// memSlots is the MEM bank's canonical slot inventory: "001".."099",
// M-CH01..M-CH99 as the front panel names them (D11).
//
// DENSE, not sparse. This model addresses a memory channel with two packed
// BCD bytes and nothing else, so every slot in the range is addressable and
// the bank lists all of them; spec.Bank.Sparse and its three companions
// stay zero, which spec.Capabilities.Validate enforces as a set.
func memSlots() []string {
	slots := make([]string, 0, 99)
	for n := 1; n <= 99; n++ {
		slots = append(slots, fmt.Sprintf("%03d", n))
	}
	return slots
}

// scanSlots is the SCAN bank's inventory: "P1" and "P2", which is what the
// manual prints and what codeplug.DisplaySlot's identity fallback passes
// through unchanged (D11). On the wire they are the channel addresses
// 01 00 and 01 01 — channels 100 and 101 in this profile's channel space —
// and read.go's parseSlot is the one place that mapping is written down.
func scanSlots() []string {
	return []string{"P1", "P2"}
}

// bankFields is one bank's field-support map, parameterised by the grade
// the record's OWN fields carry — Unverified on RealHardware, Supported on
// Simulated.
//
// EVERY ONE OF THE TWENTY spec.Fields IS NAMED, including the eleven this
// radio does not have. A Field left out of the map answers the zero
// FieldSupport anyway (spec.Capabilities.FieldSupport), so naming it adds
// no behaviour — it adds the RECORD that somebody looked at the field and
// decided, which is what caps_test.go's allFields pin exists to keep true.
//
// The nine graded rw are exactly the fields the 1A 00 record carries:
//
//   - frequency, tx_frequency — ④–⑧ and ❹–⑧, five packed-BCD bytes each.
//     The transmit frequency is a DISTINCT field, so a split channel round
//     trips.
//   - mode, filter, data_mode, tone_mode — ⑨, ⑩ and ⑪'s two nibbles.
//   - tone_tx, tone_rx — ⑫–⑭ and ⑮–⑰, BCD tenths of a hertz.
//   - tag — ⑱–㉗, ten bytes.
//
// The eleven graded ZERO, each for a stated reason:
//
//   - clarifier, ctcss_state, ctcss_tone, shift — the Yaesu vocabulary,
//     which this record does not carry at all (matrix §1 rows 6, 7, 14, 15;
//     spec D4 puts ctcss_state's job on tone_mode for Icom models).
//
//   - duplex, offset — MANUAL-EVIDENCED absences (matrix §1b). Spec D6 puts
//     per-channel duplex and offset OUT OF SCOPE for this pair, and this is
//     the honest empty shape enabler E5b exists to admit: a bank that
//     reaches neither shift nor duplex is not required to name a
//     vocabulary for either.
//
//   - dtcs_code, dtcs_polarity — the record carries no DTCS field
//     (matrix §1b). Vacuous rather than skipped.
//
//   - scan_skip — the tier's hard constraint: ③'s SELECT nibble is group
//     MEMBERSHIP, the inverse of a skip flag, and mapping it as skip is
//     forbidden (plan decision D4). The value is not discarded — it lives
//     inside the civ record on civ.FieldSelect and round-trips there.
//
//   - tag_display — this record carries NO display flag, so a read reports
//     Unavailable and the grading says the same thing rather than
//     declaring a readable field whose every read is Unavailable. The
//     in-tree precedent is core/driver/ftdx10/caps.go:194 and
//     core/driver/ftdx101/caps.go:225 on their own flagless records
//     (plan decision D5, R13).
//
//     IT IS A NARROWING, AND IT IS RECORDED AS ONE. This model's matrix §2
//     row 8 grades the field read-✓ on both banks, as a CHOICE rather than
//     manual evidence; the ZERO FieldSupport here is narrower than that row,
//     so it IS a deviation and is recorded as IC-7300 matrix ERRATUM 13 on
//     the flagless-record precedent above. Plan D5's "no deviation from
//     either matrix remains on this field" was true of the MK2 only, whose
//     §2 row 8 already grades it — — — — and whose rev-1 proposed erratum is
//     recorded WITHDRAWN. The two models agree; only one of them had to move
//     to get there.
//
//   - erase — this tier ships no erase path (spec D4). The document prints
//     TWO clear forms and neither is implemented; doc.go says what a future
//     write-trial milestone would need.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// In the 1A 00 record.
		spec.FieldFrequency:   rw,
		spec.FieldMode:        rw,
		spec.FieldTag:         rw,
		spec.FieldTxFrequency: rw,
		spec.FieldFilter:      rw,
		spec.FieldDataMode:    rw,
		spec.FieldToneMode:    rw,
		spec.FieldToneTx:      rw,
		spec.FieldToneRx:      rw,

		// Not in the 1A 00 record. The zero FieldSupport, named.
		spec.FieldClarifier:    {},
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldShift:        {},
		spec.FieldTagDisplay:   {},
		spec.FieldScanSkip:     {},
		spec.FieldDuplex:       {},
		spec.FieldOffset:       {},
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},
		spec.FieldErase:        {},
	}
}

// baseCapabilities builds the whole capability description, with EVERY ONE
// of spec.Capabilities' twenty-eight fields set explicitly — every zero
// value named by caps_test.go's deliberatelyZero audit, including D8's
// five receiver vocabularies,
// are deliberately zero are set to their zero value in the literal below,
// beside the reading that says why, and caps_test.go's reflection pin
// refuses a field that is neither non-zero nor named in its
// deliberatelyZero map.
//
// Additions design D4.2 moved the pinned count from twenty-seven to
// twenty-eight by requiring this driver's transmitter anatomy explicitly.
func baseCapabilities(memFields, scanFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		// Matrix §1 row 1.
		Model: "IC-7300",
		// The CI-V ADDRESS HEX, not a CAT ID: CI-V has no ID string, and
		// spec D3.2 fixes the address as this field's content. The 19 00
		// token this driver observes at Open is APPENDED to the session's
		// Identity.CATID ("94:<token>") and compared against nothing — the
		// reply value is undocumented on every model in this tier.
		// Matrix §3.4, PDF p.126: CI-V Address (Default: 94h).
		CATID:    "94",
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: bankMemoryLabel,
				Slots: memSlots(),
				// FALSE: this document says nothing about whether a memory
				// channel must stay populated (matrix §1b).
				NoBlank: false,
				Fields:  memFields,
			},
			{
				ID:    spec.BankScan,
				Label: bankScanLabel,
				Slots: scanSlots(),
				// FALSE, and it is a decision rather than an oversight.
				// The MK2's document prints "P1 and P2 cannot be cleared"
				// and its SCAN bank is NoBlank as a result; THIS document
				// says nothing of the kind (matrix §1b, register entry
				// `ic7300-scan-edge-noblank`, lift `ic7300-scan-edge-read`),
				// and borrowing the sibling's sentence is exactly the
				// cross-model contamination both matrices' §4 forbid.
				NoBlank: false,
				Fields:  scanFields,
			},
		},
		// The eight mode names ⑨ / ❾ can hold, in wire-value order
		// (matrix §1 row 2; PDF p.167 "① Operating mode"). 0x06 IS ABSENT
		// FROM THE PRINTED COLUMN and is invented nowhere: a record
		// carrying it fails the read with a *civ.ParseError naming the byte
		// and the offset (plan decision D12).
		Modes: []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R"},
		// ⑱–㉗, ten bytes (matrix §3.9, PDF p.169 "Up to 10 characters.").
		TagLen: 10,
		// DELIBERATELY ZERO: there is no clarifier/RIT field in the 1A 00
		// record at all (matrix §1 rows 6 and 7 grade both a poor fit).
		// Graded, not silently omitted.
		ClarMaxHz:  0,
		ClarStepHz: 0,
		// DELIBERATELY EMPTY, and the deviation is recorded. Matrix §1 row 8
		// lists the fifty printed selectable tone frequencies (PDF p.64) —
		// but those are the PANEL-selectable set, and the record stores a
		// BCD FREQUENCY indexing no table, as that row says in terms. A
		// fifty-entry list here would fail closed on every encodable value
		// outside it. The fifty stay in doc.go. The deviation is RECORDED,
		// and it landed as IC-7300 matrix ERRATUM 12 (REV 3's change log
		// called it "proposed erratum 3", which is the number it was asked
		// for under, not the number it was given).
		CTCSSTones: nil,
		// THE TONE DOMAIN, declared as the numeric range E3 added for
		// exactly this shape (D16, ruling T1). The printed per-digit legend
		// (PDF p.171) gives `100 Hz digit: 0 ~ 2` and `10/1/0.1 Hz digits:
		// 0 ~ 9`, i.e. 0..2999 deciHz on a 0.1 Hz grid; intersected with the
		// capability floor of 1 — because 0 Hz is not a tone — the declared
		// domain is {1, 2999, 1}. The civ layer stays lossless over the
		// whole encodable range, zero included, and read.go maps an
		// out-of-domain tone (zero included) to Unknown rather than handing
		// up a Known value codeplug.ToneField.Valid would refuse.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1},
		// The [USB] rate list (matrix §1 row 9; this tier connects over USB,
		// PDF p.160 "◇ CI-V connection").
		Bauds: []int{4800, 9600, 19200, 38400, 57600, 115200},
		// A CHOICE, and doc.go argues it: there is NO numeric factory
		// default — both baud items ship set to `Auto` (matrix §3.3,
		// MANUAL-EVIDENCED) — so 19200 is chosen as the highest rate present
		// in BOTH the [USB] list and the [REMOTE] list (4800/9600/19200),
		// which is the one rate that still works when the user has
		// `CI-V USB Port` set to `Link to [REMOTE]` (matrix §3.16 A4).
		// Register entry `ic7300-default-open-baud`, lift `ic7300-open-rate`.
		DefaultBaud: 19200,
		// The COVERAGE floor (PDF p.150). The encoding admits 0 Hz, so
		// 30 000 is the tighter documented bound at this end.
		MinFreqHz: 30_000,
		// The STORABLE ceiling (PDF p.167): the record's 10 MHz digit is
		// capped at 6 and the 100 MHz and 1 GHz digits are printed fixed 0.
		// The 74 800 000 figure on PDF p.150 is TUNING COVERAGE, and
		// publishing it would let a codeplug carry a value this encoder must
		// afterwards refuse. In each direction the tighter of the two
		// printed bounds governs (plan decision D6). Matrix Erratum 4(a)
		// marked §1 row 12 PENDING this plan's explicit decision; the
		// decision landed as matrix ERRATUM 10, which discharges that
		// PENDING marker — so 4(a) is closed rather than outstanding.
		MaxFreqHz: 69_999_999,
		// DELIBERATELY EMPTY: nothing in this document is declared
		// never-empty (matrix §1 row 13; plan decision D8).
		RequiredSlots: nil,
		// DELIBERATELY EMPTY: no shift or duplex field exists on this model
		// (matrix §1 row 14). Enabler E5b is what admits the shape: no bank
		// reaches FieldShift or FieldDuplex, so no vocabulary is demanded.
		// Inventing a dummy one to satisfy a validator would be dishonest.
		ShiftOptions: nil,
		// DELIBERATELY EMPTY: displaced by ToneModes on Icom models
		// (spec D4; matrix §1 row 15).
		CTCSSStates: nil,
		// DELIBERATELY EMPTY: MANUAL-EVIDENCED absence (matrix §1b, duplex).
		DuplexOptions: nil,
		// ⑪'s LOW nibble: "0: OFF, 1: TONE, 2: TSQL" (PDF p.169's ⑪ detail
		// box). Three values, three distinct semantics, so no entry needs
		// spec.ToneMode.Canonical.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		},
		// DELIBERATELY EMPTY, both: the record carries no DTCS field at all
		// (matrix §1b). A polarity table for a field this radio does not
		// have would be an invention, so the absence is recorded rather
		// than filled.
		DTCSPolarities: nil,
		DTCSCodes:      nil,
		// ⑩ / ❿, the whole byte (PDF p.167, "② Filter").
		Filters: []string{"FIL1", "FIL2", "FIL3"},
		// TAKEN FROM THE PROFILE, never restated. The 95 bytes are the
		// codec's own charset, so the driver's advertised set and the set
		// civ's validName enforces cannot drift apart — and a name this
		// driver advertises as legal is one BuildMemorySet will accept.
		TagCharset: string(ic7300civ.Profile().NameCharset()),
	}
}

// capabilitiesUnverified is the RealHardware profile: every field the
// record carries graded Unverified, which is documented-but-unproven and
// therefore UNWRITABLE (spec.FieldSupport.CanWrite). It is what
// writeTrialsComplete == false means in capability terms, and it is the
// profile a real IC-7300 gets.
func capabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

// capabilitiesSimulated is the profile a fake radio gets: the same fields
// graded Supported, so the write choreography is exercisable end to end
// without a consent flag and without any claim about hardware.
func capabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

// cloneCapabilities returns a deep copy of caps: Banks (with their Slots
// and Fields), every slice, and the tone RANGE pointer.
//
// THE POINTER IS THE ONE THAT BITES. spec.Capabilities.CTCSSToneRange is a
// *ToneRange, so a shallow copy would hand every caller the same range
// value a session enforces against; a caller widening its Max would widen
// the gate. spec.ConsentUnverifiedWrites copies it for exactly this reason,
// and this function must too.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	if caps.Banks != nil {
		out.Banks = make([]spec.Bank, 0, len(caps.Banks))
		for _, b := range caps.Banks {
			// Capabilities.Bank already returns a defensive copy.
			cp, _ := caps.Bank(b.ID)
			out.Banks = append(out.Banks, cp)
		}
	}
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), caps.CTCSSTones...)
	if caps.CTCSSToneRange != nil {
		r := *caps.CTCSSToneRange
		out.CTCSSToneRange = &r
	}
	out.Bauds = append([]int(nil), caps.Bauds...)
	out.RequiredSlots = append([]string(nil), caps.RequiredSlots...)
	out.ShiftOptions = append([]spec.ShiftOption(nil), caps.ShiftOptions...)
	out.CTCSSStates = append([]spec.ToneState(nil), caps.CTCSSStates...)
	out.DuplexOptions = append([]spec.DuplexOption(nil), caps.DuplexOptions...)
	out.ToneModes = append([]spec.ToneMode(nil), caps.ToneModes...)
	out.DTCSPolarities = append([]string(nil), caps.DTCSPolarities...)
	out.DTCSCodes = append([]int(nil), caps.DTCSCodes...)
	out.Filters = append([]string(nil), caps.Filters...)
	return out
}

// ic7300Driver is this package's driver.Driver implementation.
//
// Task 14 gives it Open, the probe and the session; this file gives it the
// two things a driver must be able to answer before any port exists — which
// model it is, and what that model can do.
type ic7300Driver struct {
	profile Profile

	// transportLogger, when set, is handed to the engine so a session's
	// wire traffic can be traced. Nil by default: a driver that logged
	// unasked would write a user's memory contents somewhere they did not
	// choose.
	transportLogger transport.Logger

	// consentUnverifiedWrites records that the user explicitly accepted
	// writing fields no IC-7300 has ever confirmed. It is applied at
	// SESSION capability assembly, never here: Driver.Capabilities is the
	// static baseline, and consent is a property of a session the user
	// asked for.
	consentUnverifiedWrites bool
}

// New returns a driver value for the IC-7300 under the given profile.
//
// The zero Profile is RealHardware, the fail-safe one, so a caller that
// passes nothing at all gets the description that writes nothing.
//
// IT WILL RETURN THE NEUTRAL driver.Driver once the Session is complete
// (write.go), and everything above this package holds the seam rather than
// this type. The two optional capabilities this driver additionally
// implements — driver.SerialFramingReporter on the DRIVER,
// driver.DiagnosticsReporter on the SESSION — are reached by the house's
// two-result type assertion, never by a concrete type a caller would have to
// import this package to name.
func New(p Profile, opts ...Option) driver.Driver {
	d := &ic7300Driver{profile: p}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Option configures a driver value at construction.
type Option func(*ic7300Driver)

// WithConsentedUnverifiedWrites records that the user has explicitly
// accepted writing fields that no IC-7300 has ever confirmed. It is the
// second key to the hardware-write gate (spec.Support's own words), and it
// is applied only to a profile this driver recognises.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic7300Driver) { d.consentUnverifiedWrites = true }
}

// Model is the display name and registry key. It equals
// Capabilities().Model, which driver.Registry.Register enforces.
func (d *ic7300Driver) Model() string { return "IC-7300" }

// Capabilities returns the STATIC baseline for this driver's profile.
//
// AN UNRECOGNISED PROFILE FALLS BACK TO THE FAIL-SAFE ONE, not to the
// simulated one: a Profile value outside the two declared constants is a
// construction mistake, and the safe answer to a construction mistake is
// the description that writes nothing.
func (d *ic7300Driver) Capabilities() spec.Capabilities {
	switch d.profile {
	case Simulated:
		return capabilitiesSimulated()
	case RealHardware:
		return capabilitiesUnverified()
	default:
		return capabilitiesUnverified()
	}
}

// profileRecognised reports whether this driver's profile is one of the two
// declared constants. Consent is applied only to a recognised profile —
// mirroring core/driver/ftdx101 — so a forged profile value cannot pick up
// a consented capability set on the way past.
func (d *ic7300Driver) profileRecognised() bool {
	switch d.profile {
	case Simulated, RealHardware:
		return true
	}
	return false
}

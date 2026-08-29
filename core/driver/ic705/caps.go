// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	civic705 "github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is this radio's hardware write guard, and it is
// FALSE: no IC-705 has ever been asked anything by this project — not a
// byte sent, not a byte received.
//
// While it is false there is no hardware-verified capability profile at
// all, so a RealHardware session gets capabilitiesUnverified: every field
// the record expresses labelled Read Unverified / Write Unverified, which
// makes spec.FieldSupport.CanWrite false and refuses every write
// project-wide — at codeplug.Diff, at the clone service, and again in this
// driver's own WriteChannel. The ONE route past it is the user's own
// consent (WithConsentedUnverifiedWrites), which relabels write-side
// Unverified as ConsentedUnverified for one session and leaves this static
// profile untouched.
//
// FLIPPING IT IS A FOUR-PART CHANGE, deliberately: this constant, AND a
// capabilitiesRealHardware built field class by field class from trial
// evidence, AND the Capabilities switch that selects it, AND the pin test
// that holds this line down. No production code reads the constant, so a
// one-character edit unlocks nothing on its own.
const writeTrialsComplete = false

// Profile selects which capability description a driver hands out.
//
// The zero value is RealHardware ON PURPOSE, and every unrecognised value
// fails safe to the same set: the failure direction for a forged or
// corrupted Profile is always "nothing writable", never a writable set.
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio.
	// While writeTrialsComplete is false it selects the all-Unverified
	// set: reads labelled Unverified, every mapped field's Write
	// Unverified, nothing writable.
	RealHardware Profile = iota
	// Simulated is the profile for internal/fakeic705-backed sessions ONLY
	// (the CLI's --fake mode, the GUI's demo mode): Write Supported for
	// the thirteen fields this record expresses, so the write choreography
	// can be exercised end to end with no hardware at risk.
	Simulated
)

// The two bank labels, kept beside the namespaces they describe.
const (
	memBankLabel  = "Memories"
	callBankLabel = "Call channels"
)

// callSlots is the CALL bank's complete, fixed inventory — four channels,
// displayed G101-001…G101-004 under the one wire = display − 1 rule (see
// slots.go). The radio prints them as group 0100, channels 0000-0003, and
// labels them 144 C1/C2 and 430 C1/C2; that printed numbering is display
// cosmetics, deferred per spec D4 adjudication 14 and recorded in doc.go.
func callSlots() []string {
	out := make([]string, 0, callChannels)
	for c := 1; c <= callChannels; c++ {
		out = append(out, spec.SparseSlot(callDisplayGroup, c))
	}
	return out
}

// bankFields writes down ALL TWENTY of matrix §2's rows for one bank —
// including the zeros, which is the whole point. A field left OUT of the
// map reads as Unsupported by accident, and an accident is
// indistinguishable from a decision when the consequence is "this radio
// cannot do that".
//
// rw is the support pair every field this record EXPRESSES carries; the
// profiles differ in nothing else.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	none := spec.FieldSupport{}
	return map[spec.Field]spec.FieldSupport{
		// The thirteen the 111-byte record carries, each mapped by a span
		// in core/civ/ic705's layout.
		spec.FieldFrequency:    rw,
		spec.FieldMode:         rw,
		spec.FieldTag:          rw,
		spec.FieldTxFrequency:  rw,
		spec.FieldDuplex:       rw,
		spec.FieldOffset:       rw,
		spec.FieldToneMode:     rw,
		spec.FieldToneTx:       rw,
		spec.FieldToneRx:       rw,
		spec.FieldDTCSCode:     rw,
		spec.FieldDTCSPolarity: rw,
		spec.FieldFilter:       rw,
		spec.FieldDataMode:     rw,

		// The Yaesu-family four: this radio expresses duplex and
		// tone_mode instead, and matrix §2 grades all four Unsupported on
		// both banks. E5b is what lets ShiftOptions and CTCSSStates stay
		// empty in consequence — a bank that reaches neither field has no
		// vocabulary to name.
		spec.FieldClarifier:  none,
		spec.FieldCTCSSState: none,
		spec.FieldCTCSSTone:  none,
		spec.FieldShift:      none,

		// tag_display: this record carries a name and no separate
		// "display the name" flag (matrix §2).
		spec.FieldTagDisplay: none,

		// scan_skip: O-6, and this DIVERGES from matrix §2's Supported
		// grading with the matrix's own blessing (§2's A2 calls the
		// mapping "a live question for the plan"). The ★n nibble at
		// record offset 0 marks a channel INTO one of three select-scan
		// groups; spec.FieldScanSkip is a two-valued "skip this one".
		// Mapping a four-valued group membership onto a boolean skip
		// would be a lie in both directions, and the tier's hard
		// constraint settles it: never map it as skip. The nibble is
		// UNMAPPED in the record, so a ★-marked channel is REFUSED on
		// write rather than silently demoted to OFF.
		spec.FieldScanSkip: none,

		// erase: spec D4 adjudication 19 — no erase path at all this
		// tier. TWO clear wire forms exist on this radio (matrix §3.13)
		// and neither is built, admitted by the gate, or reachable from
		// any capability label.
		spec.FieldErase: none,
	}
}

// baseCapabilities is everything both profiles share: the vocabularies,
// the bounds, the two banks' shapes. Only the field-support pair differs
// between them, and it arrives as an argument for exactly that reason.
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	p := civic705.Profile()
	return spec.Capabilities{
		Model: "IC-705", // matrix §1 row 1
		// CATID is the radio's CI-V ADDRESS on this tier, not a Yaesu
		// four-digit ID answer (matrix §1 row 2). The `19 00` reply value
		// is undocumented on all six models in this tier, so a session
		// records the observed token beside this address and matches it
		// against nothing (spec D3.2, D5 entry 7, lift L-IDTOKEN).
		CATID:    "A4",
		Transmit: spec.HasTransmitter,

		// The ten operating modes in the manual's own printed order (PDF
		// p.18, folio 17, `• Operating mode`). The DV code's reading as
		// hex 0x17 is ASSUMED — ic705-dv-mode-code, lift L-DV-MODE — and
		// the assumption lives in the civ layout's enum, not here.
		Modes: []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "WFM", "CW-R", "RTTY-R", "DV"},

		// TagLen and TagCharset come from THE PROFILE ITSELF, so
		// TagByteOK (the model/CSV side) and civ's name validator (the
		// wire side) cannot drift: 16 bytes, 0x20..0x7E. The charset is
		// the printed enumeration plus the ASSUMED space (L-NAME-SPACE);
		// see core/civ/ic705/profile.go for why those add up to a
		// contiguous run and why that is an observation rather than a
		// licence to widen.
		TagLen:     p.NameLength(),
		TagCharset: string(p.NameCharset()),

		// The written-down zeros. Every one is audited by
		// caps_test.go's deliberatelyZero table, with the register entry
		// that owes the real number:
		//   MinFreqHz/MaxFreqHz — the radio's STORABLE range, which the
		//   matrix leaves ASSUMED and unfilled (§1 rows 11-12).
		//   spec.Capabilities' own doc says these mean the RADIO's
		//   lowest/highest storable frequency, NOT the record field's
		//   encoding range, and filling them with the encoding range
		//   would be exactly the widening R11 forbids
		//   (ic705-min-storable-frequency / L-FREQ-FLOOR,
		//   ic705-max-storable-frequency / L-FREQ-CEIL).
		//   ClarMaxHz/ClarStepHz — no per-channel clarifier at all.
		//   RequiredSlots — nothing per-slot is documented never-empty;
		//   the CALL bank's NoBlank carries what IS claimed.
		MinFreqHz:     0,
		MaxFreqHz:     0,
		ClarMaxHz:     0,
		ClarStepHz:    0,
		RequiredSlots: nil,

		// THE TONE DOMAIN IS A RANGE, NOT A LIST (E3, T1(2)). This
		// radio's tone field is a NUMBER — three packed-BCD bytes of
		// tenths of a hertz — so a fifty-entry chart would describe the
		// wrong shape entirely. The printed digit ranges (PDF p.23, folio
		// 22: "100Hz digit: 0 ~ 2" then "0 ~ 9" three times) make
		// 0…2999 deciHz ENCODABLE; the DECLARED floor is 1 because 0 Hz
		// is not a tone, and spec.Validate refuses MinDeciHz <= 0
		// outright. The gap between encodable and declared is therefore
		// EXACTLY the single value zero (O-12), which the radio uses for
		// "no tone set" and which T1 routes through Unknown-on-read
		// (read.go) and preserve-the-radio's-own-number-on-write
		// (write.go) rather than through the tone vocabulary. The printed
		// step is 0.1 Hz.
		CTCSSTones:     nil,
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1},

		// ASSUMED PLACEHOLDERS, and the Basic Manual's narrow admission
		// (R14) did NOT settle them — it settled a negative instead. The
		// microUSB CI-V port is baud-agnostic: "You can communicate
		// regardless of the PC software's baud rate setting" (Basic
		// Manual rev 9, PDF p.92, printed folio 13-2, §13 CONNECTOR
		// INFORMATION, [microUSB] › USB Serial Port, read off a 300 dpi
		// page render). No default rate is printed anywhere in the three
		// admitted values' pages, so these stay ASSUMED —
		// ic705-default-baud / L-BAUD and ic705-baud-list / L-BAUDLIST.
		// The failure mode of a wrong default is benign here twice over:
		// a CDC port ignores the rate, and a rate the radio did not
		// answer at produces no address-matched reply, so the probe fails
		// honestly rather than misreading.
		Bauds:       []int{4800, 9600, 19200, 38400, 57600, 115200},
		DefaultBaud: 19200,

		// The Yaesu halves of the two vocabulary pairs are EMPTY, and
		// that is a positive statement rather than a gap: this radio
		// expresses duplex and tone_mode. E5b admits the emptiness
		// because no bank of this radio reaches FieldShift or
		// FieldCTCSSState; before E5b, spec.Validate refused it.
		ShiftOptions: nil,
		CTCSSStates:  nil,

		// Three duplex directions, one wire code each, so E5's canonical
		// marking is trivially satisfied — asserted by
		// TestVocabulariesAreCanonicalAndDistinct rather than assumed.
		DuplexOptions: []spec.DuplexOption{
			{Value: "OFF", Direction: spec.DuplexOff},
			{Value: "DUP-", Direction: spec.DuplexDown},
			{Value: "DUP+", Direction: spec.DuplexUp},
		},
		// Four tone modes, four distinct semantics, likewise one each.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
			{Value: "DTCS", Semantics: spec.ToneModeDTCS},
		},
		// The first letter is the TRANSMIT polarity, per the diagram's
		// left-nibble leader. ASSUMED — ic705-dtcs-nibble-roles, lift
		// L-DTCS-POLARITY.
		DTCSPolarities: []string{"NN", "NR", "RN", "RR"},
		DTCSCodes:      dtcsCodes(),
		Filters:        []string{"FIL1", "FIL2", "FIL3"},

		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: memBankLabel,
				// SLOTS EMPTY, AND THAT IS THE CONTRACT. A sparse bank's
				// Slots lists what a READ MATERIALISED (spec.Bank.Sparse),
				// and the static baseline has read nothing — it describes
				// the MODEL, before any radio has been probed.
				// Session.Capabilities is where this radio's actual
				// occupied slots appear, put there by Open's inventory
				// walk (inventory.go).
				Slots:       nil,
				NoBlank:     false,
				Sparse:      true,
				Groups:      memGroups,
				GroupBase:   1,
				PerGroup:    memPerGroup,
				ChannelBase: 1,
				// Budget is the number of POPULATED channels this radio
				// holds across the whole sparse space at once — 500
				// against 10 000 addresses, which is what makes the space
				// sparse by construction. ASSUMED: ic705-group-budget,
				// lift L-BUDGET-CEILING. It is enforced at
				// codeplug.Diff time and NEVER on the wire (what an
				// over-budget IC-705 actually does is undocumented —
				// L-OVERBUDGET).
				Budget: 500,
				Fields: bankFields(rw),
			},
			{
				ID:    spec.BankCall,
				Label: callBankLabel,
				Slots: callSlots(),
				// NoBlank: a CHOICE over the MANUAL-EVIDENCED clear-form
				// restriction (matrix §1b). "A call channel can never be
				// empty" is a separate ASSUMED claim —
				// ic705-call-channel-emptiness, lift L-CALL-EMPTY — and
				// this flag is how the model layer expresses it. Dense,
				// so the three sparse numbers stay zero or spec.Validate
				// refuses the bank.
				NoBlank: true,
				Fields:  bankFields(rw),
			},
		},
	}
}

// capabilitiesUnverified is the all-Unverified FAIL-SAFE profile, and it is
// what a RealHardware IC-705 session gets today.
//
// The READ labels are Unverified rather than Supported for the same reason
// the write ones are: this driver's read path has been exercised against a
// manual, a set of transcription legs and a fake, and NO IC-705 HAS EVER
// ANSWERED A FRAME. The profile describes the EVIDENCE; consent is a
// decision about risk, not evidence, and it is applied per session
// downstream of this value (sessionCapabilities), never here — which is
// what internal/wiring.NeedsUnverifiedConsent reads.
func capabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// capabilitiesSimulated is the internal/fakeic705-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// the thirteen fields the 111-byte record expresses.
//
// Against the fake, hardware risk is moot and the write choreography itself
// is what is being exercised end to end, so claiming Supported here is a
// claim about internal/fakeic705 and about nothing else. The seven fields
// this record cannot express stay the zero FieldSupport, simulator or not:
// no amount of cooperative fake on the other end of the wire changes what
// the frame has room for.
func capabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}

// dtcsCodes generates this field's PRINTED domain: the 512 codes 000..777
// whose every decimal digit is 0-7, strictly ascending.
//
// GOVERNED BY ENABLERS E3, in terms: "DTCS stays an explicit table: the 512
// codes 000..777 (every digit ≤ 7) are NOT a contiguous range; models
// supply the table (generator fine)." The 512 values are exactly what PDF
// p.23 (folio 22), `• DTCS code and polarity setting`, evidences — three
// printed digit leaders, each "0 ~ 7" — so this is the field's printed
// domain, MANUAL-EVIDENCED, not a widening.
//
// THE REVIEW FINDING THAT THIS SHOULD BE EMPTY OR NARROWED WAS DISPUTED,
// AND THE DISPUTE WAS SUSTAINED (plan O-10, 24/08/2026). An empty table
// fails CLOSED on every Known DTCS code (codeplug.IntField.Valid), so this
// radio's DTCS channels would be unreadable rather than merely unverified —
// strictly worse than recording the narrowing as an assumption. The
// narrowing to the radio's ACTUAL selectable subset is not abandoned: it is
// carried as the ASSUMED register entry ic705-dtcs-selectable-set, lift
// L-DTCS-SET (capture: step the radio's DTCS item through its positions and
// record the codes offered).
func dtcsCodes() []int {
	out := make([]int, 0, 512)
	for h := 0; h <= 7; h++ {
		for t := 0; t <= 7; t++ {
			for u := 0; u <= 7; u++ {
				out = append(out, h*100+t*10+u)
			}
		}
	}
	return out
}

// cloneCapabilities returns a deep copy of caps: every bank (with its own
// Slots and Fields) and every top-level slice freshly allocated, sharing no
// storage with the input.
//
// THE COPY IS LOAD-BEARING. A session hands its capabilities out on every
// Capabilities() call, and the Fields maps inside are this project's
// hardware-write gate data (spec.FieldSupport.CanWrite). A caller that
// could reach back through a returned value and flip a label would be
// rewriting the gate every other caller enforces — including this driver's
// own WriteChannel, which re-checks against the session's internal copy.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		// spec.Capabilities.Bank already returns a defensive copy with
		// fresh Slots and Fields; reuse that guarantee rather than
		// restating per-field copying here. The ok result cannot be false
		// — b came out of caps.Banks and Bank scans that same slice — and
		// a duplicate BankID, the only way it could serve the wrong bank,
		// is refused by spec.Capabilities.Validate, which both profiles
		// pass under test.
		cp, _ := caps.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
	}
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), caps.CTCSSTones...)
	if caps.CTCSSToneRange != nil {
		// A POINTER, so `out := caps` aliased it. ToneRange has no
		// reference-typed field, so one struct copy is a complete deep
		// copy.
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

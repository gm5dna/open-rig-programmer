// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import (
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Profile selects which capability set a driver built by New hands its
// sessions.
//
// RealHardware is the ZERO VALUE, on purpose: the failure direction for a
// forged, corrupted or simply uninitialised Profile is always the
// fail-safe set, never a writable one. Any value this package does not
// recognise selects the same fail-safe, and the consent transform is
// applied only for a RECOGNISED value (see profileRecognised), so an
// unrecognised profile goes on writing nothing however the consent option
// is set.
type Profile int

const (
	// RealHardware is a session with a physical IC-9700. While
	// writeTrialsComplete is false it selects CapabilitiesUnverified —
	// every write column Unverified, nothing writable without the user's
	// recorded consent.
	RealHardware Profile = iota
	// Simulated is a session backed by internal/fakeic9700, where
	// hardware safety is moot: writes are Supported so the fake exercises
	// the same choreography a consented real session would.
	Simulated
)

// writeTrialsComplete is FALSE, and every claim this package makes about
// writing rests on that.
//
// NO IC-9700 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Not one frame
// has been sent to one, no answer has been captured from one, and every
// byte in core/civ/ic9700 is transcribed from a PDF. Until the Stage-W
// write trials in doc.go's register have happened, a real-hardware
// session gets the all-Unverified set and the only key that opens the
// write gate is the user's explicitly recorded consent — which is a
// statement about the operator, not evidence about the radio.
//
// Flipping it is a hardware milestone with its own review, not a code
// change: it would have to be accompanied by the capture each Stage-W
// entry names.
const writeTrialsComplete = false

// The band-plan bounds this radio's memories can hold, from the band
// table's own edges (matrix §1 #11/#12) rather than from the record
// field's arithmetic capacity. The five packed-BCD frequency bytes could
// encode ten digits; what the radio will STORE is the narrower claim, and
// it is the one a caller needs.
const (
	minFreqHz = 144_000_000
	maxFreqHz = 1_300_000_000
)

// tagCharset is every printable ASCII byte, 0x20..0x7E — the same 95 the
// dialect's own name charset carries.
//
// IT IS SUPPLIED RATHER THAN DEFAULTED, and the difference from
// core/spec's default tag rule is one character, ';'. The default
// excludes it because a semicolon terminates a NEWCAT frame and a tag
// containing one could smuggle a second command onto the wire; CI-V is
// binary framing with no such terminator, and this radio's own "Codes for
// character entries" tables print ';' among the 32 symbol rows, so the
// Yaesu-shaped exclusion would refuse a byte this radio's own document
// shows.
//
// THE SPACE IS A SEPARATE, WEAKER CLAIM. Those same tables print every
// OTHER character in this set but OMIT a space row; the document instead
// prints, against `1A 00`, "Memory name / All characters are usable."
// core/spec's default already permits 0x20, so this charset does not add
// it — but accepting 0x20 is ASSUMED, not manual-evidenced, exactly as
// the dialect's own nameCharset is graded: spec D5 entry 3's family
// hazard, matrix §3.9, this model's own row.
const tagCharset = " !\"#$%&'()*+,-./0123456789:;<=>?@" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`" +
	"abcdefghijklmnopqrstuvwxyz{|}~"

// modeNames is the ten wire modes in the manual's own printed order (PDF
// p.14's operating mode table), which is what the UI shows.
//
// The BASE of the two two-digit codes is ASSUMED HEXADECIMAL — register
// entry `ic9700-mode-codes-are-hexadecimal`. It matters for DV (17) and
// DD (22) only; the dialect carries the bytes and this list carries the
// names, and the two are held together by the crosscheck in
// core/civ/ic9700.
var modeNames = []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "DV", "DD"}

// baudRates are the six printed CI-V rates (PDF p.13 footnote *4).
var baudRates = []int{4800, 9600, 19200, 38400, 57600, 115200}

// defaultBaud is ASSUMED — register entry `ic9700-factory-default-baud`,
// lift R2. This guide defers the factory rate to the instruction manual
// (PDF p.4 "Preparing" names a speed and points elsewhere), so there is no
// evidence here; 19200 is the middle of the printed six and the rate Icom
// most commonly ships. A multi-rate probe is not available to this driver
// — the open path is internal/wiring's.
const defaultBaud = 19200

// duplexOptions carries THREE entries for a wire field with FOUR values,
// and the missing one is the whole of OQ-6.
//
// core/spec declares exactly three DuplexDirections, and ⑬'s high nibble
// prints four values: 0=Duplex OFF, 1=Duplex−, 2=Duplex+, 3=RPS. RPS is a
// repeater mode with no direction to give, so it cannot be named here at
// all. The dialect still carries it (a channel set to RPS round-trips
// exactly), a READ presents such a channel's duplex as Unknown — honest:
// "this radio has a value here this codeplug's vocabulary cannot name" —
// and a WRITE is REFUSED with RPS named. Flattening it onto OFF would be a
// lie about the radio; Unavailable would be a lie about the field.
//
// No entry carries Canonical: E5 requires it only where a direction is
// expressed more than once, and here three entries give three distinct
// directions.
func duplexOptions() []spec.DuplexOption {
	return []spec.DuplexOption{
		{Value: "OFF", Direction: spec.DuplexOff},
		{Value: "DUP-", Direction: spec.DuplexDown},
		{Value: "DUP+", Direction: spec.DuplexUp},
	}
}

// toneModes is ⑬'s LOW nibble, four values with four distinct semantics.
func toneModes() []spec.ToneMode {
	return []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		{Value: "DTCS", Semantics: spec.ToneModeDTCS},
	}
}

// dtcsCodes generates the 512 admissible DTCS codes: every value from 0 to
// 777 whose every decimal digit is 0..7, ascending.
//
// GENERATED RATHER THAN TRANSCRIBED, and E3 sanctions exactly this: the
// 512 codes are not a contiguous range (777 is followed by nothing, and
// 8 and 9 never appear in any digit place), so the numeric ToneRange
// shape cannot describe them — but they ARE a complete three-digit octal
// space, which one loop states without 512 chances to mistype. The
// evidence is PDF p.21's "DTCS code and polarity setting": First, Second
// and Third digit each printed as `0 ~ 7`.
func dtcsCodes() []int {
	out := make([]int, 0, 8*8*8)
	for a := 0; a < 8; a++ {
		for b := 0; b < 8; b++ {
			for c := 0; c < 8; c++ {
				out = append(out, a*100+b*10+c)
			}
		}
	}
	return out
}

// toneRange is the E3 numeric tone domain — the shape a radio whose tone
// field is a NUMBER needs, and every CI-V model is one.
//
// THE ARITHMETIC, so no later reader has to re-derive it. PDF p.21's
// "Repeater tone/tone squelch frequency settings" prints the digit places
// and nothing else: 100 Hz digit `0 ~ 2`, 10 Hz digit `0 ~ 9`, 1 Hz digit
// `0 ~ 9`, 0.1 Hz digit `0 ~ 9`. That is an ENCODABLE range of
// 0.0 Hz … 299.9 Hz in 0.1 Hz steps. The capability floor is 1 deciHz —
// zero is not a tone, and the landed ToneRange refuses MinDeciHz <= 0 —
// so the declared minimum is max(printed 0, 1) = 1. The one encodable
// value the capability excludes is therefore ZERO, and read.go is where
// that difference is handled (T1(3): a wire zero reads back Unknown,
// never a Known value ToneField.Valid would refuse).
//
// spec.StandardCTCSSTones() is REJECTED and recorded as rejected: that
// 50-tone chart is verified against a YAESU manual, and the matrix review
// commends this matrix for citing no Yaesu value as evidence about an
// Icom.
//
// WHAT IT COSTS: E3's own UI disposition. The tone PICKER stays
// list-driven, so on a range-declaring model the grid shows and
// round-trips tones while the picker cannot offer them. A Wave-4 item,
// and one of this driver's honesty rows.
func toneRange() *spec.ToneRange {
	return &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1}
}

// bankFields is the per-field support grid, identical on all three banks.
//
// THE COMMAND IS MANUAL-EVIDENCED: all three banks are addressed by the
// same `1A 00` form (matrix §2 preamble) — the document draws the record
// once and the channel-number legend covers all three ranges. THAT SCAN
// AND CALL ANSWER WITH A RECORD OF THE SAME SHAPE AS MEM IS A SEPARATE
// CLAIM, and it is ASSUMED, not printed: register entry
// `ic9700-scan-call-addressable`, doc.go, lift R15. This grid is built as
// if a scan-edge memory and a call channel were the same 111 bytes at a
// different address; if that assumption is wrong, there is a per-bank
// difference this grid does not describe.
//
// WHAT IS ABSENT IS THE INTERESTING HALF. A Field this map does not list
// answers the zero FieldSupport — Unsupported both ways — and six Fields
// are deliberately absent:
//
//   - spec.FieldErase. The clear form is PRINTED (matrix §3.13) and is
//     deliberately unshipped: no builder exists, the gate has no branch
//     that could admit one, and the consent transform exempts erase
//     structurally. See doc.go's erase record.
//   - spec.FieldScanSkip. Field ④ is a four-valued SELECT-memory group
//     tag (0=OFF, 1=★1, 2=★2, 3=★3), not a boolean skip flag — the tier
//     rule is that scan_skip on Icom is SELECT group membership and is
//     never mapped as skip (OQ-4). The value is READ by the dialect and
//     visible in diagnostics; it is not offered here as something it is
//     not.
//   - spec.FieldClarifier and spec.FieldTagDisplay. The record carries
//     neither (matrix §1 #6/#7) — a MANUAL-EVIDENCED absence.
//   - spec.FieldShift, spec.FieldCTCSSState and spec.FieldCTCSSTone. The
//     Yaesu vocabularies, replaced on this family by FieldDuplex,
//     FieldToneMode and the FieldToneTx/FieldToneRx pair. E5b is what
//     lets a model declare neither Yaesu list and still pass Validate.
//
// A Known value for any of the six therefore meets the capability gate in
// write.go and is REFUSED BY NAME rather than dropped — which is the
// Wave-1 C2 contract: silently dropping a value the caller explicitly
// marked Known would be a lie.
func bankFields(write spec.Support) map[spec.Field]spec.FieldSupport {
	fs := spec.FieldSupport{Read: spec.Unverified, Write: write}
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:    fs,
		spec.FieldMode:         fs,
		spec.FieldTag:          fs,
		spec.FieldTxFrequency:  fs,
		spec.FieldDuplex:       fs,
		spec.FieldOffset:       fs,
		spec.FieldToneMode:     fs,
		spec.FieldToneTx:       fs,
		spec.FieldToneRx:       fs,
		spec.FieldDTCSCode:     fs,
		spec.FieldDTCSPolarity: fs,
		spec.FieldFilter:       fs,
		spec.FieldDataMode:     fs,
	}
}

// banks builds the three banks with the given write grade.
//
// NoBlank ON SCAN AND CALL, and not on MEM. The printed clear form
// (matrix §3.13) admits only 0001~0099 — MANUAL-EVIDENCED. That the radio
// ACTUALLY REFUSES to clear a scan edge or a call channel is a separate,
// narrower claim, and it is ASSUMED, not printed: register entry
// `ic9700-scan-call-not-clearable`, doc.go, lift W6. Recording that
// assumption as NoBlank is what stops a generic layer planning an erase
// the printed form has no range for — and it is a separate statement from
// FieldErase's absence, which says this driver ships no erase at all.
//
// DENSE, not sparse. Sparse/Groups/PerGroup/Budget describe the Icom
// tier's group-addressed models (the 705, the 905), where a read
// materialises a handful of slots out of a 100x100 space under a
// population budget. This radio's space is small and completely
// enumerable — 321 slots — so all four stay zero, which
// Capabilities.Validate requires whenever Sparse is false.
func banks(write spec.Support) []spec.Bank {
	return []spec.Bank{
		{
			ID:     spec.BankMemory,
			Label:  "Memories",
			Slots:  bankSlots(spec.BankMemory),
			Fields: bankFields(write),
		},
		{
			ID:      spec.BankScan,
			Label:   "Program scan edges",
			Slots:   bankSlots(spec.BankScan),
			NoBlank: true,
			Fields:  bankFields(write),
		},
		{
			ID:      spec.BankCall,
			Label:   "Call channels",
			Slots:   bankSlots(spec.BankCall),
			NoBlank: true,
			Fields:  bankFields(write),
		},
	}
}

// capabilities is the one description of this radio, parameterised by the
// single thing the two profiles differ in.
//
// CATID IS "A2" — THE ADDRESS, AND NOT A TOKEN. Spec D3.2's CI-V identity
// is the radio's CI-V address (matrix §3.4), which is what an
// address-matched reply proves. The `19 00` answer's data byte is
// undocumented on all six models in this tier (D5 entry 7, lift R6), so
// the probe RECORDS it beside the address and matches it against nothing;
// see ic9700.go, where Identity.CATID is assembled.
func capabilities(write spec.Support) spec.Capabilities {
	return spec.Capabilities{
		Model:  "IC-9700",
		CATID:  "A2",
		Banks:  banks(write),
		Modes:  append([]string(nil), modeNames...),
		TagLen: 16,

		// The record carries no clarifier at all (matrix §1 #6/#7), so
		// both numbers are zero and FieldClarifier is unreachable on
		// every bank. A poor fit stated as zero rather than left to a
		// default a later reader would take for a real offset.
		ClarMaxHz:  0,
		ClarStepHz: 0,

		// The tone domain is a RANGE and the DTCS codes are a TABLE, and
		// the two shapes are not interchangeable: see toneRange and
		// dtcsCodes. CTCSSTones stays nil — Validate refuses a profile
		// declaring both shapes.
		CTCSSToneRange: toneRange(),
		DTCSCodes:      dtcsCodes(),

		Bauds:       append([]int(nil), baudRates...),
		DefaultBaud: defaultBaud,

		MinFreqHz: minFreqHz,
		MaxFreqHz: maxFreqHz,

		// RequiredSlots is empty: NoBlank carries this radio's only
		// must-stay-populated constraint, and it carries it for whole
		// banks rather than for individual slots (matrix §1 #13).
		RequiredSlots: nil,

		// The Yaesu vocabularies are EMPTY, positively. This family
		// expresses duplex and tone_mode instead, and E5b is what makes
		// the empty pair admissible: fail-closed is preserved through
		// those fields' Unsupported grades rather than through a
		// non-empty list nobody would use.
		ShiftOptions: nil,
		CTCSSStates:  nil,

		DuplexOptions:  duplexOptions(),
		ToneModes:      toneModes(),
		DTCSPolarities: []string{"NN", "NR", "RN", "RR"},
		Filters:        []string{"FIL1", "FIL2", "FIL3"},
		TagCharset:     tagCharset,
	}
}

// CapabilitiesUnverified is the FAIL-SAFE set: every field's write column
// is spec.Unverified, so CanWrite is false everywhere and no change
// reaches a radio.
//
// It is what a RealHardware session gets while writeTrialsComplete is
// false, which is to say today and until a hardware milestone says
// otherwise. The user's recorded consent transforms it — every write-side
// Unverified becoming ConsentedUnverified — at session-capability
// assembly and nowhere else; see WithConsentedUnverifiedWrites. Consent
// widens what may be ATTEMPTED and is never evidence that it works.
func CapabilitiesUnverified() spec.Capabilities {
	return capabilities(spec.Unverified)
}

// CapabilitiesSimulated is the set a session backed by the fake gets:
// Write Supported for the thirteen fields the record maps, because
// "hardware safety" is not a question one can ask of a simulator.
//
// The READ column stays Unverified, deliberately. The fake answers from
// the same manual-derived geometry the real radio is BELIEVED to use, so
// a read through it is exactly as unproven as a read through a real one;
// marking it Supported would launder a transcription into a hardware
// fact. Erase and the six absent fields are absent here too — a simulator
// may not offer a capability the shipped driver has no frame for.
func CapabilitiesSimulated() spec.Capabilities {
	return capabilities(spec.Supported)
}

// cloneCapabilities returns a deep copy of caps: Banks (with their Slots
// and Fields) and every other slice or pointer freshly allocated.
//
// THE COPY IS LOAD-BEARING, not hygiene. What Session.Capabilities hands
// out is this project's hardware-write gate data
// (spec.FieldSupport.CanWrite), and the session's own WriteChannel
// re-checks against the value it KEEPS. An aliasing copy would let a
// caller who tweaked a returned FieldSupport — to experiment, or by
// accident — silently redefine the gate every other caller enforces.
//
// It mirrors core/driver/ftdx101's function of the same name and extends
// it by the Icom tier's own fields: the two duplex/tone vocabularies, the
// DTCS table and polarity list, the filter list, and the tone RANGE, which
// is a POINTER and would otherwise be shared outright.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		// Capabilities.Bank returns a defensive copy (fresh Slots and
		// Fields); reuse that guarantee rather than restating per-field
		// copying here. The ok result cannot be false: b came out of
		// caps.Banks and Bank scans that same slice for b.ID, and
		// Validate refuses a duplicate BankID, which is the only way the
		// lookup could serve the wrong one.
		cp, _ := caps.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
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

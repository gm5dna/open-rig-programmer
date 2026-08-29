// SPDX-License-Identifier: GPL-3.0-or-later

package spec

// ConsentUnverifiedWrites returns a deep copy of caps in which every
// write-side Unverified label has become ConsentedUnverified — the
// project's ONE consent transform. Drivers call it at session-capability
// assembly, and only when their consent option was passed and the selected
// profile is one they recognise, so a capability set carrying the state is
// always one the user asked for. One implementation exists so drivers
// cannot drift on what consent means.
//
// Two exclusions are structural — properties of this function rather than
// of any profile's labels, so no driver table can weaken them:
//
//   - WRITE SIDE ONLY. Read labels are never transformed. Reads already
//     flow, so an Unverified read label is a true statement that consent
//     does not alter; and ConsentedUnverified is a write-only state that
//     Capabilities.Validate rejects read-side (see validate.go).
//   - FieldErase NEVER. Erase is not consentable at any label, so
//     populated-to-empty stays blocked structurally rather than by a
//     profile remembering to say Unsupported. The FT-710's fail-safe
//     profile, for one, leaves MEM FieldErase Write: Unverified purely
//     because Unverified and Unsupported are equally unwritable today; an
//     unqualified transform would mint a consented erase from that label,
//     unblocking codeplug.Diff's erase gate and making clone/execute's
//     documented-unreachable DiffErased branch reachable.
//
// Every other label passes through untouched: Supported, Unsupported and
// Inert write labels, the zero FieldSupport, and all of caps' non-bank
// data. A capability set with no write-side Unverified therefore comes
// back deep-equal to its input (the FT-710's RealHardware profile is
// exactly that shape), and the transform is idempotent — ConsentedUnverified
// is not Unverified, so a second application converts nothing.
//
// The returned value shares no storage with caps: Banks (with their Slots
// and Fields) and every other slice are freshly allocated, and caps itself
// is never modified. That matters because caps is typically a driver's
// long-lived baseline: an aliasing copy would let a consented session
// silently redefine the write gate every other caller enforces.
func ConsentUnverifiedWrites(caps Capabilities) Capabilities {
	out := caps

	// A nil Banks copies to nil, not an empty non-nil slice, matching
	// copyBank's treatment of a nil Slots/Fields.
	if caps.Banks != nil {
		out.Banks = make([]Bank, 0, len(caps.Banks))
		for _, b := range caps.Banks {
			// copyBank gives fresh Slots and Fields; the loop below then
			// rewrites only the copy's labels.
			cp := copyBank(b)
			for f, fs := range cp.Fields {
				if f != FieldErase && fs.Write == Unverified {
					fs.Write = ConsentedUnverified
					cp.Fields[f] = fs
				}
			}
			out.Banks = append(out.Banks, cp)
		}
	}

	// The remaining top-level slices carry no Support labels, but they are
	// copied so the result shares no storage with caps at all. append to a
	// nil slice preserves a nil input as nil.
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]Tone(nil), caps.CTCSSTones...)
	// The tone RANGE is a pointer, so `out := caps` aliased it. ToneRange
	// has no reference-typed field, so one fresh copy of the struct is a
	// complete deep copy — and it is what keeps the promise this function
	// makes about sharing no storage with caps at all.
	if caps.CTCSSToneRange != nil {
		r := *caps.CTCSSToneRange
		out.CTCSSToneRange = &r
	}
	out.Bauds = append([]int(nil), caps.Bauds...)
	out.RequiredSlots = append([]string(nil), caps.RequiredSlots...)
	out.ShiftOptions = append([]ShiftOption(nil), caps.ShiftOptions...)
	out.CTCSSStates = append([]ToneState(nil), caps.CTCSSStates...)
	// The Icom-tier vocabularies (design D4). They carry no Support
	// labels either, and are copied for the same reason: the result must
	// share no storage with caps AT ALL, so that a consented session can
	// never reach back into a driver's long-lived baseline. The FIELD
	// maps above need no per-field enumeration for the tier's new
	// Fields: the loop walks whatever Fields a bank declares, so a new
	// Field is covered by construction — which TestConsentUnverifiedWrites
	// _CoversTierAddedFields verifies rather than assuming (design D4,
	// round 2 F9).
	out.DuplexOptions = append([]DuplexOption(nil), caps.DuplexOptions...)
	out.ToneModes = append([]ToneMode(nil), caps.ToneModes...)
	out.DTCSPolarities = append([]string(nil), caps.DTCSPolarities...)
	out.DTCSCodes = append([]int(nil), caps.DTCSCodes...)
	out.Filters = append([]string(nil), caps.Filters...)
	// The D8 receiver vocabularies are copied explicitly because the
	// top-level slices and pointer are not covered by the bank-map walk.
	out.TuningSteps = append([]string(nil), caps.TuningSteps...)
	if caps.ProgramTuningStepRange != nil {
		r := *caps.ProgramTuningStepRange
		out.ProgramTuningStepRange = &r
	}
	out.AttenuatorDB = append([]int(nil), caps.AttenuatorDB...)
	out.PreampOptions = append([]string(nil), caps.PreampOptions...)
	out.AntennaOptions = append([]string(nil), caps.AntennaOptions...)
	return out
}

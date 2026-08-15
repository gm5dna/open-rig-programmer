// SPDX-License-Identifier: GPL-3.0-or-later

package spec

// Support describes how confident this project is that a radio/protocol
// can do something, independent of whether that confidence has been
// tested against real hardware.
//
// The states matter because "documented" and "proven" are not the same
// thing, and this project treats them differently: it hard-gates all
// writes to a real radio behind hardware verification sessions. Unverified
// exists precisely to hold that gate open — capability data can describe a
// field as documented or assumed-workable without that description alone
// being enough to authorise a write. ConsentedUnverified is what that
// description becomes once the user has explicitly accepted the risk for
// this radio. See FieldSupport.CanWrite.
type Support int

const (
	// Unsupported means the radio/protocol cannot do this at all.
	Unsupported Support = iota
	// Unverified means this is documented in the manual, or assumed by
	// analogy with a sibling model/field, but has not been proven against
	// real hardware. Read-only tools may still use Unverified data (e.g.
	// to display an assumed value), but it must never authorise a write to
	// a real radio.
	Unverified
	// Supported means this has been proven on hardware, or is beyond
	// doubt from the manual (e.g. structural framing bytes). Only
	// Supported unlocks writes; see FieldSupport.CanWrite.
	Supported
	// Inert means the protocol TRANSMITS this field's value on every
	// write, but the radio silently ignores it — a CHANGED value cannot
	// be honoured. HW-CONFIRMED 2026-07-13 (M5b write trials against a
	// real UK FT-710, docs/hardware-notes.md §Clarifier): the FT-710
	// accepted MW frames carrying non-zero clarifier values and Rx/Tx
	// clarifier flags without any "?;" rejection, then read back zeros
	// every time — the transmitted values are simply discarded.
	//
	// Inert exists because a fixed-layout write frame cannot OMIT such a
	// field (every byte position is always transmitted), so marking it
	// Unsupported would wrongly block EVERY write via the all-or-nothing
	// per-field gate (codeplug.Diff treats every transmitted field as
	// touched), while marking it Supported would wrongly promise that a
	// changed value takes effect. The enforcement is split, and both
	// halves document it: codeplug.Diff blocks an entry whose Inert
	// field's value CHANGES (an intent the radio will not honour) and
	// lets an UNCHANGED Inert value flow — it transmits the baseline
	// value, the radio ignores it, and the read-back verify still
	// matches; the driver's WriteChannel re-check treats Inert as
	// acceptable-to-transmit, since it holds the channel but not the
	// baseline. CanWrite() stays false for Inert — nothing may treat an
	// Inert field as genuinely writable.
	Inert
	// ConsentedUnverified means the field is documented in the CAT
	// reference and exercised against the fake, has NEVER been proven
	// against a real radio — and the user has explicitly accepted writing
	// it anyway. It is the only state whose truth is about the operator
	// rather than the hardware: Supported says "the radio proved this",
	// ConsentedUnverified says "the radio has not, and the user chose to
	// proceed regardless".
	//
	// It is a WRITE-side state only. Reads already flow, and an Unverified
	// read label is a true statement that consent does not alter, so
	// Capabilities.Validate REJECTS Read == ConsentedUnverified (see
	// validate.go).
	//
	// It is also SESSION-ONLY. No profile constructor ever contains it:
	// the baselines describe evidence, and consent is not evidence. The
	// state is minted at session-capability assembly by the shared
	// transform, which each driver invokes only when its consent option
	// was passed — so a capability set carrying it is always one the user
	// asked for, never one a driver's static tables asserted.
	//
	// CanWrite() is true for it: consent is the second key to the
	// hardware-write gate (see FieldSupport.CanWrite). Consent widens WHAT
	// may be attempted, never HOW carefully: every permitted write still
	// runs the full clone-level write+verify pair, and a read-back that
	// does not match still fails the write exactly as it would for a
	// Supported field.
	ConsentedUnverified
)

// String returns the constant's identifier ("Unsupported", "Unverified",
// "Supported", "Inert", "ConsentedUnverified"). Values outside the
// declared constants return a diagnostic placeholder rather than
// panicking.
func (s Support) String() string {
	switch s {
	case Unsupported:
		return "Unsupported"
	case Unverified:
		return "Unverified"
	case Supported:
		return "Supported"
	case Inert:
		return "Inert"
	case ConsentedUnverified:
		return "ConsentedUnverified"
	default:
		return "Support(invalid)"
	}
}

// FieldSupport records, independently, how well a single field's read path
// and write path are supported.
type FieldSupport struct {
	Read  Support
	Write Support
}

// CanWrite reports whether this field may be written to a real radio. It
// is true when Write == Supported or Write == ConsentedUnverified, and
// false for every other state.
//
// This is the hardware-write gate for the whole project: every real-radio
// write path (clone service, UI "apply" actions) must consult CanWrite
// before sending a value to hardware, and must refuse when it is false.
// TWO keys open it, and they are different kinds of claim:
//
//   - Supported — HARDWARE EVIDENCE. The field has been proven on a real
//     radio, or is beyond doubt from the manual. Hardware verification
//     trials (M5b, 13/07/2026 for the FT-710) are what flip a field's
//     Write support from Unverified to Supported.
//   - ConsentedUnverified — RECORDED USER CONSENT. The field is
//     documented and fake-exercised but never hardware-proven, and the
//     user has explicitly accepted writing it anyway for this radio. The
//     state is minted per session by the consent transform, never by a
//     profile constructor; see its doc comment.
//
// Unverified WITHOUT that recorded consent stays deliberately NOT
// writable — a field that is merely documented or assumed, not proven on
// hardware and not consented to, must not be trusted with a write.
//
// Inert is likewise NOT writable, and consent does not change that: an
// Inert field's value is transmitted but ignored by the radio, so no
// caller may claim a write of it takes effect — see Inert's doc comment
// for the Diff-side changed-value gate that (alone) decides when an Inert
// field blocks a send.
func (f FieldSupport) CanWrite() bool {
	return f.Write == Supported || f.Write == ConsentedUnverified
}

// Unreachable reports whether this field is Unsupported in BOTH
// directions — the radio's protocol is not believed to reach it at all,
// as opposed to reaching it unverifiedly (Unverified) or inertly
// (Inert). Callers use it to decide whether an externally supplied
// value for the field can even be a claim about this radio: see
// core/csvio's CHIRP skip/tag-display construction and app's
// tag-display default.
func (f FieldSupport) Unreachable() bool {
	return f.Read == Unsupported && f.Write == Unsupported
}

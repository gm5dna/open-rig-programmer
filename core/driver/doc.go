// SPDX-License-Identifier: GPL-3.0-or-later

// Package driver defines the seam between generic layers and
// radio-specific protocol code: the Driver/Session interfaces, the
// Registry that assembles them, and the typed errors their contracts
// promise.
//
// # The seam philosophy
//
// Everything above this package — the UI, validation, the clone service,
// the CLI — is generic: it works from capability data
// (spec.Capabilities) and the neutral codeplug model
// (codeplug.Channel), and it never knows a wire protocol's field names,
// framing, or quirks. Everything below it — a driver package such as
// core/driver/ft710 — is radio-specific: protocol quirks live in the
// driver, and ONLY in the driver. The FT-710's empty-slot "?;" mapping,
// its Kind-on-write pairing, its regional 60 m channel discovery, its
// mode-name table: none of that ever leaks above this interface. A
// second radio model is added by writing a second driver package and
// registering it — no generic code changes.
//
// The split of knowledge is:
//
//   - Driver.Capabilities: the STATIC baseline — what the model can do,
//     before any radio has been probed. No discovered banks.
//   - Session.Capabilities: the EFFECTIVE capabilities — baseline plus
//     what Open discovered about this specific radio (e.g. its regional
//     60 m/EMG inventory, as read-only banks). Both must pass
//     spec.Capabilities.Validate.
//
// # The write-guard mechanism
//
// This project hard-gates every write to real hardware behind hardware
// verification trials (milestone M5b). The gate is data, not code paths:
// spec.FieldSupport.CanWrite() is true only for Write == spec.Supported
// or Write == spec.ConsentedUnverified (the user's recorded consent),
// and every write path — codeplug.Diff's Blocked marking, the clone
// service's refusal to execute blocked entries, and each driver
// Session's own WriteChannel re-check (defence in depth, below all of
// them) — consults it.
//
// A driver package participates by exposing capability PROFILES. The
// FT-710 driver has three:
//
//   - CapabilitiesRealHardware: what real-hardware sessions get now the
//     ft710 package's writeTrialsComplete constant has flipped to true
//     (the M5b PR, 13/07/2026, hardware evidence linked in its doc
//     comment): Write = Supported for exactly the six fields the live
//     write trials verified, the clarifier Inert (transmitted but
//     ignored by the radio, HW-CONFIRMED), and tone/scan-skip/erase
//     Unsupported.
//   - CapabilitiesUnverified: every per-field Write support is
//     spec.Unverified. Because Unverified.CanWrite() == false, every
//     change is blocked. This was every real-hardware session's profile
//     before the flip; it remains the FAIL-SAFE any unrecognised
//     Profile value selects. The user's consent does NOT reach it ON
//     THIS RADIO: an FT-710 selects it only for an UNRECOGNISED Profile
//     value, and every driver applies the consent transform only for a
//     recognised one — so the FT-710's fail-safe goes on writing nothing
//     however the option is set. (Read that as a fact about the FT-710's
//     profile arrangement, not a universal one: on the FTdx10 and the
//     FTdx101 the all-Unverified set is what RealHardware itself
//     selects, so there consent does reach it, by design.)
//   - CapabilitiesSimulated: Write = Supported for the same six fields
//     (aligned with CapabilitiesRealHardware, clarifier Inert included,
//     so --fake behaviour matches real behaviour), used ONLY for
//     simulator-backed sessions (fakeradio: the CLI's --fake mode, the
//     GUI's demo mode) where "hardware" safety is moot.
//
// Session.WriteChannel's own refusal (ErrWriteRefused, before any wire
// traffic) is the last line: even if a bug above the driver let a
// blocked change through, the driver re-checks field writability against
// its session capabilities and refuses.
package driver

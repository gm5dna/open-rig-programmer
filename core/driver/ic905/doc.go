// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic905 is the driver for the Icom IC-905: the one package in
// this model's tier that opens a session, probes a radio, discovers its
// memory inventory, reads a channel and refuses to write one.
//
// # What this package is
//
// It is the SEAM SIDE of the IC-905. core/civ/ic905 is the DIALECT —
// data only, a civ.ProfileConfig and the frozen evidence that binds it —
// and this package is the only one that may reach for
// transport.NewEngineWith, transport.Engine.Do and civ.BuildMemorySet
// for this model. Generic layers above (the clone service, the CLI, the
// GUI) hold a driver.Driver and a driver.Session and never learn that
// CI-V exists.
//
// It REGISTERS NOTHING. Wiring the IC-905 into internal/wiring is a
// later, separate commit; this package produces a driver that is ready
// to register.
//
// # Hardware status: UNVERIFIED
//
// NO IC-905 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. No frame has
// been sent to one, none has been captured from one, and every byte this
// driver builds or expects is derived from the IC-905 CI-V REFERENCE
// GUIDE and from the four quarantined evidence artefacts frozen under
// core/civ/ic905/testdata. writeTrialsComplete is pinned FALSE (caps.go)
// and TestWriteTrialsComplete_PinnedFalse pins BOTH halves — the constant
// AND that nothing is writable in the real-hardware profile — so flipping
// the constant alone unlocks nothing.
//
// # The two record lengths
//
// This model's memory record has TWO documented lengths, because its
// frequency field has two documented widths. Both numbers are
// RECORD-ONLY — the payload after the four channel-address bytes, which
// is what civ.Profile's layouts denote (spec Erratum 1):
//
//	Condition A   5-byte frequency   record 64   frame 75 (7 + 4 + 64)
//	Condition B   6-byte frequency   record 65   frame 76 (7 + 4 + 65)
//
// Condition A is the shape PDF p.19 (folio 18)'s "• Memory content"
// diagram draws, and it is MANUAL-EVIDENCED addend by addend. Condition B
// is the 10 GHz form and is ASSUMED (register: the D5 model-specific
// entry for the 905; lift: ic905-R-06).
//
// The consequences run right through this package, and they are
// deliberately asymmetric:
//
//   - The PROBE accepts EITHER length as a confirmation (ic905.go). The
//     accepted set IS the fingerprint (spec D3.2), and this model's set
//     has two members.
//   - A READ handles both, because every field but the frequency is fixed
//     width: a 64-byte answer means five frequency bytes and a 65-byte
//     answer means six, with no discriminator needed beyond the length.
//   - A WRITE emits 64 and nothing else, because civ.ProfileConfig's
//     BuildLength is a single static int and the tier writes only the
//     shape its document draws. A 10 GHz write is therefore REFUSED
//     before the wire, naming spec.FieldFrequency and the lift that would
//     settle it (write.go, rung 5) — never truncated, never sent at an
//     assumed width.
//
// # What this driver refuses, and why refusal is the feature
//
// Twenty-seven of the sixty-four record bytes have no home in the neutral
// memory model: byte ⑤ (Split and Select memory setting), ⑮ (digital
// squelch), ㉕ (DV digital code squelch) and the three eight-byte
// D-STAR call-sign blocks ㉙~㊱, ㊲~㊹ and ㊺~52. They sit in the
// profile's Fixed template, and civ's encoder fills unmapped bytes from
// it — so a write to a channel that actually carries a call sign, a
// digital squelch, a DV code squelch OR A SELECT ★ TAG would silently
// replace them.
//
// The tier's E6 ruling forbids that: a driver may write a slot ONLY when
// its unmapped regions equal the profile's Fixed template, and anything
// else is REFUSED with the reason named, never rewritten. WriteChannel
// therefore reads the target channel first and refuses with a
// driver.WriteRefusedError naming the offending byte range. The stated
// cost is accepted for this tier: D-STAR-carrying, digital-squelch-set
// and SELECT-tagged channels are refused, never corrupted, at one extra
// read per write.
//
// # The banks, and their two namespaces
//
// MEM is a SPARSE bank over a 100 × 100 address space (10,000 addresses)
// with an ASSUMED budget of 500 occupied slots. Its slots are spelled in
// spec.SparseSlot's shared namespace, "G01-001" … "G100-100", and its
// materialised set is DISCOVERED at Open — core/clone.Service.ReadAll
// walks Capabilities().Banks[].Slots, so a sparse bank that published
// none would return no memories at all.
//
// CALL is a DENSE bank of twelve named slots in a DISTINCT namespace,
// "C01" … "C12", mapping to wire group 100, channels 0…11. The
// disjointness is structural rather than arithmetic:
// spec.ParseSparseSlot refuses any string without a leading "G", so no
// CALL slot can be read as a MEM address and no MEM address can render as
// a CALL slot.
//
// # This package's own limits, stated as limits
//
// The IC-905 CI-V REFERENCE GUIDE documents a CLEAR form for 1A 00 —
// FE FE AC E0 1A 00 <group> <chan> FF FD, with the CALL group excluded
// ("You cannot specify group \"01 00\"") — and this tier implements NONE
// of it. There is no clear builder, the gate admits no clear frame,
// spec.FieldErase is the zero FieldSupport in both profiles, and
// spec.ConsentUnverifiedWrites structurally never consents erase. An
// empty channel handed to WriteChannel is refused naming
// spec.FieldErase. What a future write-trial milestone would need is
// recorded here and nowhere implemented.
//
// The DTCS code's printed digit range is 0–7, which civ's BCD encoder
// (0–9) does not enforce; the primary gate is the explicit 512-code table
// this package declares (caps.go's dtcsCodes), which codeplug.Validate
// consults, and write.go re-checks it as defence in depth.
//
// The profile carries one global channel range (0–99), so the builder and
// gate admit CALL-group channels 12–99, which this document does not
// define. Nothing reaches them: the CALL bank is twelve named slots, bank
// walks never produce such an address, and a write to one arriving by
// hand is refused as a slot in no effective bank.
package ic905

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
// hand is refused before it can reach a bank at all — slotAddress bounds
// the CALL namespace to twelve.
//
// # The control lines, and a radio that can key itself from one
//
// PDF p.9 (folio 8)'s 1A 05 items 01 36, 01 37 and 01 38 each offer
// "01=USB (A) DTR, 02=USB (A) RTS, 03=USB (B) DTR, 04=USB (B) RTS" for
// USB SEND and for CW/RTTY keying. So A DRIVER THAT RAISED OR LOWERED DTR
// OR RTS AT OPEN ON THIS RADIO COULD KEY ITS TRANSMITTER (matrix §3.2).
// The conservative policy is to assert neither line.
//
// core/transport's OpenSerial drives BOTH LOW before returning the port —
// SetRTS(false) and SetDTR(false), core/transport/port.go — and this
// worktree may not change core/transport. The finding is recorded here
// and handed upward rather than acted on.
//
// Register: ic905.control_lines_at_open. Lift: Stage W capture
// ic905-W-01 — with the transceiver's SEND and keying assignments at
// their factory values, open the CI-V port and record whether the radio
// keys.
//
// # The serial rates: a CHOICE over an ASSUMED default
//
// THIS DOCUMENT PRINTS NO RATE, ANYWHERE. PDF p.3 (folio 2)'s
// "◇ Preparing" defers the address, the data communication speed and the
// transceive function to the IC-905 Basic manual, and a sweep of every
// command-table page finds no rate figure at all (matrix §1 rows 9–10,
// §3.3).
//
// So the two capability fields are DIFFERENT KINDS OF CLAIM and are
// graded separately:
//
//   - Bauds is a CHOICE — which rates this programme OFFERS in its UI.
//     The list is the CI-V family's conventional set, not a statement
//     about what an IC-905 accepts. Register: ic905.bauds. Lift: Stage R
//     capture ic905-R-04 — a rate sweep on one IC-905: open at each
//     offered rate in turn, send FE FE AC E0 19 00 FD at each, and record
//     which rates return an address-matched reply.
//   - DefaultBaud is ASSUMED, and it is the operational one: internal/wiring
//     opens a real radio at exactly this rate. Register:
//     ic905.default_baud — DISTINCT from ic905.bauds, and distinct from
//     any AUTO setting, which this document does not mention either.
//     Lift: Stage R capture ic905-R-03 — on an IC-905 whose CI-V menu is
//     at factory defaults, open at the assumed default rate and record
//     whether FE FE AC E0 19 00 FD is answered.
//
// Both must be non-empty whatever the evidence, because
// spec.Capabilities.Validate requires DefaultBaud greater than zero and
// present in Bauds — and because transport.OpenSerial treats a
// non-positive rate as "unset" and silently substitutes its own default,
// which is exactly the sort of quiet substitution a written-down value
// prevents.
//
// The two entries live in the PROFILE's register (core/civ/ic905/doc.go),
// not this package's five: they describe the radio's serial surface
// rather than anything this driver's code decides.
//
// # The ASSUMED register — FIVE entries, and they are this DRIVER's
//
// core/civ/ic905/doc.go carries NINETEEN, which are the PROFILE's. These
// five are the ones this package's own code acts on, and NO ENTRY IS
// COUNTED TWICE: nineteen there plus five here is TWENTY-FOUR distinct
// entries. Every capture named below is on an IC-905, and no capture from
// any other model lifts any entry here.
//
//  1. ic905.control_lines_at_open — whether asserting or releasing DTR or
//     RTS at open keys this radio's transmitter. The 1A 05 assignments
//     above are printed; what a driver's port-open does to a radio
//     carrying them is not. Lift: ic905-W-01, above.
//
//  2. ic905.write_full_record_required — that a 1A 00 set must carry the
//     WHOLE record. The document draws one complete layout and never
//     authorises a short write except in the clear form (matrix §3.10),
//     and every rung of the write ladder rests on it. Lift: Stage W
//     capture ic905-W-02 — send one complete 1A 00 set to a scratch
//     channel and record whether the answer is FB or FA.
//
//  3. Serial framing 8-N-1 (spec D5 entry 8) — see the Driver's StopBits
//     method for the full argument. It is transcribed HERE and NOT into
//     core/civ/ic905/doc.go, because StopBits() lives on the concrete
//     Driver; recording the move is what keeps the two registers from
//     double-counting it. Lift: Stage R capture ic905-R-10.
//
//  4. ic905.create_default_tone — what tone value the radio itself writes
//     into a channel created with tone mode OFF. MANUAL-EVIDENCED
//     ABSENCE: this document prints the tone field's digit ranges (PDF
//     p.24, folio 23) and NO DEFAULT VALUE ANYWHERE, unlike the models
//     whose manuals print "Default: 88.5 Hz", swept across the tone
//     sections and the command table. The decision to REFUSE an
//     empty-slot create that carries no explicit tone follows from that
//     absence. Lift: Stage R capture ic905-R-18 — create a channel from
//     the front panel with tone mode OFF, read it with 1A 00, and record
//     bytes ⑯~⑱ and ⑲~㉑. Scope: what that one radio stores in those
//     spans for that one channel.
//
//  5. ic905.mode_band_constraint — that the MEMORY RECORD enforces what
//     the command table's footnotes state: DD and ATV only on the
//     1200 MHz band or higher (PDF p.17, folio 16, mode table footnote),
//     RPS only with DD, and DUP± only with a mode other than DD (PDF
//     p.19, folio 18, the ⑭ breakout's note). The footnotes are printed
//     against the standalone commands, not against 1A 00, so applying
//     them to the record is the assumption. This driver REFUSES the
//     forbidden combinations, which is the conservative direction either
//     way. Lift: Stage R capture ic905-R-19 — store a DD channel below
//     1200 MHz from the front panel, read it back, and record whether the
//     radio accepted it.
package ic905

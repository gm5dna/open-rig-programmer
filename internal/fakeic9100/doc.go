// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic9100 is an independent, stdlib-only IC-9100 CI-V simulator.
//
// Its wire register is the manual-derived `1A 00` data area: a 3-byte
// AddressFormBankChannel selector (one band byte, then a two-byte packed-BCD
// channel number) followed by a 57-byte record. The selector's band byte
// takes the record's own printed values, 00 (HF/50 MHz) to 03 (1200 MHz);
// the channel byte pair is packed BCD 0001-0099. SetSlot("HF-001", …) and a
// wire read of band 00 channel 0001 name one record — the same
// band-name-plus-zero-padded-channel form the matrix proposes
// (`docs/superpowers/icom-matrices/ic9100-capability-matrix.md` §1b, "Slot
// string"). The record itself is OPAQUE to this package: no field inside it
// is decoded, so the D-STAR call-sign bytes, the digital-squelch byte and
// every other field are stored and echoed back exactly as received, never
// interpreted — the wire-level counterpart of the driver-side precedent the
// matrix cites (§1b, "no `spec.Field`… so none is mapped", following
// `ic7100-capability-matrix.md`). There is no TX-duplicate block on this
// record (matrix §1b, §7 of the wave spec) and so no block-equality rule to
// enforce, unlike this project's IC-7100 fake.
//
// THE QUARANTINE: this package was authored from the IC-9100 capability
// matrix and the wave spec alone — never from `core/civ/ic9100` or
// `core/driver/ic9100`, which this package is the independent OTHER witness
// to. Where this package and that dialect agree, the agreement is evidence,
// because two readings of one document landed in the same place; if one had
// imported the other, agreement would be a tautology.
//
// THE HARD RULE: stdlib only, plus the one permitted share,
// internal/fakepipe (the net.Pipe pair and its goroutine bookkeeping — sees
// []byte and a duration, carries no framing and no field layout).
// imports_test.go proves it, recursively.
//
// # The headline finding this package follows
//
// The radio's own CI-V address is 7Ch, NOT 88h. The 88h figure traced by
// the matrix (§3.4) to a documented Icom manual erratum — the IC-9100
// manual's own data-format diagram is captioned "Controller to IC-7100" /
// "IC-7100 to controller", boilerplate copy-pasted from Icom's IC-7100
// documentation without updating the model name or the example address.
// `radioAddrDefault` here is 0x7C, matching the wave spec's own
// reconciliation (spec.md §7, "IC-9100: CI-V address is 7Ch, not
// 88h/E0h… §1 heading superseded").
//
// # The band-byte resolution this package follows
//
// The manual draws the band byte and the two channel bytes inside one
// continuous numbered diagram, with no visual break marking an "address"
// separate from the "record" — the same evidentiary gap IC-7100's own bank
// byte faces. Stuart's 12/09/2026 ruling, recorded by citation in the matrix
// (§3.11) and restated in spec.md §7, resolves it as a 3-byte
// AddressFormBankChannel selector: `AddressWidth = 3`, `RecordOnlyLength =
// 60 - 3 = 57`. This package's `RecordLen` and `selector` follow that
// ruling exactly; SCAN and CALL, which the matrix shows share the identical
// band+channel address form (§1b, "Banks"), are out of scope this cycle
// (wave spec §1) and so this fake refuses every channel outside 0001-0099,
// the same treatment IC-7100's own special channels receive.
//
// # What it answers
//
//	19 00                                    -> 19 00 and the model name
//	1A 00, a selector, nothing more, occupied -> 1A 00, the selector, 57 record bytes
//	1A 00, a selector, nothing more, empty    -> FA (or an all-FF record, WithAllFFEmpty)
//	1A 00, a selector, a whole 57-byte record -> FB, and the record stored
//	1A 00, a selector, any other length       -> FA
//	1A 00, the printed one-byte clear form    -> FA
//	1A 00, a selector outside 0001-0099       -> FA (scan edges, call channel: out of scope, matrix §1b)
//	any other command, addressed to this radio -> FA
//	a frame addressed anywhere else            -> nothing at all
//
// # The assumed register
//
// Everything below is the OTHER admissible reading of a page the manual
// leaves open, named against the matrix section that records it. Nothing
// here has been put to a radio (matrix §0, "Hardware status"); every entry
// stands until a real IC-9100 settles it.
//
//  1. EMPTY-CHANNEL READ ANSWERS FA (matrix §3.8a, ic9100-empty-channel-fa).
//     Only the write-side clear form is printed; a read of an unwritten
//     channel is undocumented. WithAllFFEmpty builds the other reading
//     (§3.8b, ic9100-all-ff-record) — a separate entry, since one capture
//     cannot establish both.
//
//  2. NO ECHO BY DEFAULT (matrix §3.6, ic9100-echo-default). A
//     whole-document search for "echo" returns nothing CI-V-shaped, but the
//     [REMOTE] jack is a shared bus on which echo would be a property of
//     the wiring rather than a setting the document states either way.
//     WithUSBEcho builds the radio that echoes.
//
//  3. THE BROADCAST ADDRESS IS to=00 (matrix §3.5, ic9100-broadcast-address-
//     form). Transceive defaults ON (MANUAL-EVIDENCED), but the document
//     never prints 00 as an address value; to=00 is the family convention,
//     asserted by WithTransceiveFlood. WithAddressedFlood emits the same
//     identity content addressed to the controller instead, for the
//     different code path an addressed flood exercises.
//
//  4. THE BARE-SELECTOR READ FORM (matrix §3.7, ic9100-read-request-form).
//     `1A 00` followed by the selector and nothing else is read as a read
//     request; no shortened read form is printed, and the one worked
//     example on the page is the complete record.
//
//  5. A SHORT SET IS REFUSED (matrix §3.10, ic9100-short-set-acceptance).
//     The document says nothing about a partial set; this fake accepts
//     only a complete 57-byte record.
//
//  6. THE IDENTITY REPLY IS THE MODEL NAME (matrix §3.12,
//     ic9100-id-reply-value). `19 00`'s reply value is not printed; the
//     model name string lets a consumer prove its driver records whatever
//     it gets rather than matching a value, following this project's
//     IC-7851/IC-7850 fake.
//
// The OK/NG codes (FB/FA), the FE FE … FD frame shape and the two
// addresses are PRINTED (matrix §3.4, §3.10) even though the one diagram
// showing them is mislabelled for a different model; the frame SHAPE is a
// tier-wide Icom CI-V constant, corroborated across every model in this
// corpus, and is not itself an assumption.
//
// # Hardware status: UNVERIFIED
//
// No IC-9100 has ever been asked anything by this project (matrix §0). No
// hardware claim is made anywhere in this package.
package fakeic9100

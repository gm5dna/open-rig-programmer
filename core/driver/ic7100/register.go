// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

// Driver-register ledger. Each ASSUMED value has one IC-7100 hardware lift;
// the profile-side entries live in core/civ/ic7100/doc.go.
//
//   - ic7100-id-reply-value: read 19 00 at address 88h and record the answer
//     bytes. The driver records them as diagnostics and never matches them.
//   - ic7100-default-baud-auto: with factory Auto, open at the selected
//     19200 baud and confirm the first 19 00; record the radio version because
//     PDF p.317 warns that defaults differ by transceiver version.
//   - ic7100-control-lines: open USB1 with RTS and DTR deasserted and confirm
//     19 00. TestControlLinePolicyMakesNoDriverAssertion pins the current
//     reliance on the existing serial-open defaults (both low).
//   - ic7100-storable-frequency-range: store 30 kHz and 470 MHz from the
//     front panel and read both back.
//   - ic7100-dtcs-code-clamp: write off-list DTCS 000 to a scratch channel
//     and read it back. Until then the matrix's 104-code CHOICE is enforced.
//   - ic7100-tone-range-step: set all 50 chart tones and read ⑮–⑰, then try
//     an off-chart tenth of a hertz. The declared domain is the wire
//     field's own BCD capacity, 000.0–299.9 Hz at 0.1 Hz with the floor
//     raised off zero — pinned by TestCapabilityValuesFromMatrix and
//     TestCTCSSToneDomainAdmitsEveryChartTone, and following IC-7300 matrix
//     erratum 12 — so what the lift settles is what the RADIO accepts off
//     the printed chart, not what this driver declares.
//   - ic7100-out-of-coverage-write: write 300.000000 MHz to a scratch channel
//     and record whether it is preserved, clamped, or refused with FA.
const driverRegister = "ic7100 Stage-2 driver register: seven open hardware lifts"

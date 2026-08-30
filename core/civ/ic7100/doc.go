// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7100 describes the IC-7100 CI-V memory-record dialect.
//
// The profile is manual-derived and has not been exercised against a radio.
// These Stage-1 assumptions remain pinned to one named hardware lift each:
//
//   - ic7100-read-request-form: send FE FE 88 E0 1A 00 01 00 01 FD and
//     record the reply form. TestAddressBoundaryFrames pins the assumed form.
//   - ic7100-name-pad-byte: set a 3-character name and read all 16 name bytes.
//     TestProfilePolicy pins the conservative 0x20 policy.
//   - ic7100-tag-charset-on-wire: write ';', '\\' and '~' in a name and read
//     it back. TestProfilePolicy pins the explicit printable-ASCII policy.
//   - ic7100-record-length: read one occupied channel and count the data
//     bytes. The Stage-1 geometry and golden tests pin 111 record bytes plus
//     the three-byte address.
//   - ic7100-wire-order: store a known 16-character name, read the channel,
//     and record where it begins in the data area. TestAddressWidthAndGeometry
//     and TestRecordRoundTrip pin record offset 95, data-area byte 99.
//   - ic7100-tx-block-mandatory: write split-off with deliberately different
//     TX data and record what is retained. TestGeometryTXDuplicate pins only
//     the documented 47-byte arithmetic; it does not claim bench verification.
//   - ic7100-dv-mode-code: store a DV channel and read byte 10. The record
//     tests pin the assumed hexadecimal 0x17 mapping.
//   - ic7100-tone-range-step: set all 50 chart tones, read ⑮–⑰, then test an
//     off-chart tenth of a hertz. TestRecordToneRangeDecision pins the present
//     endpoint-only, fail-closed arithmetic policy without claiming the
//     irregular printed chart is an exact range.
//   - ic7100-special-bank-byte: select scan edge 0100 and call channel 0106,
//     read each with 1A 00, and record byte 1. TestAddressesOutsideBaseRectangleAreRefused
//     pins the resulting fail-closed scope until that lift exists.
//
// The CI-V echo, broadcast-address and serial-framing assumptions affect the
// transport/driver boundary rather than this data-only profile. Stage 2 owns
// their declarations; this package does not expose a framing reporter or send
// any transceive mutation.
package ic7100

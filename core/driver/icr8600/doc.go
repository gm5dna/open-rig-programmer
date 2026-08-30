// SPDX-License-Identifier: GPL-3.0-or-later

// Package icr8600 implements the Icom IC-R8600 CI-V memory driver.
//
// The radio is receive-only. No initialisation write is sent, no erase or
// transceive-setting command is built, and physical-radio writes remain
// Unverified until the caller supplies explicit per-session consent.
//
// Driver-side assumption register (all lifts are Stage R on an IC-R8600):
//
//   - icr8600-id-token: send 19 00 and record the whole address-matched
//     reply; the observed value is diagnostic and is never matched.
//   - icr8600-serial-framing: try 8-N-1 and 8-N-2 and record which produces
//     a clean 19 00 reply; the guide states no CI-V framing.
//   - icr8600-control-lines: repeat 19 00 with RTS/DTR asserted and low and
//     record whether both configurations answer; the guide names neither.
//   - icr8600-address-move: move the receiver off 96h, prove 96h times out,
//     and record the reply at the new address.
//   - icr8600-transceive-default: factory-reset the receiver and record
//     whether unsolicited frames arrive before any setting write.
//   - icr8600-echo-default: factory-reset, send 19 00 over each USB port and
//     record whether the exact transmitted frame is echoed.
//   - icr8600-baud-set: photograph the CI-V Baud Rate menu and list every
//     choice; the capability list is otherwise inferred from the guide's
//     CI-V preamble table.
//   - icr8600-default-baud: factory-reset, read the baud menu without
//     changing it, and confirm that 19 00 opens at the recorded rate.
//
// Passing tests prove conformance to the frozen manual-derived evidence and
// conservative choices. They do not claim hardware validation.
package icr8600

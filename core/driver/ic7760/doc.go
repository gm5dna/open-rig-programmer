// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7760 implements the IC-7760 CI-V memory driver.
//
// # Provenance and scope
//
// The sole radio authority is the IC-7760 CI-V Reference Guide, Revision 2,
// document A7788-8EX-2, May 2025: 28 PDF pages (cover, folios 1–26 and back
// cover). Byte claims are cross-checked against the frozen L/W/B/G artefacts
// in core/civ/ic7760/testdata and the IC-7760 matrix, rev 1 plus errata 1–10.
// No IC-7760 has been queried or written by this project, so all reachable
// hardware fields remain Unverified and writeTrialsComplete stays false.
//
// The guide prints radio address B2 and controller address E0. This driver
// consequently sends only these documented/admitted grammars:
//
//   - FE FE B2 E0 19 00 FD
//   - FE FE B2 E0 1A 00 <two-byte channel> FD
//   - FE FE B2 E0 1A 00 <two-byte channel> <25-byte record> FD
//
// It never sends erase, command 0B, a transceive setting, or command 1A 05.
// The supported transport is the controller rear-panel [USB B] connection;
// its two virtual COM ports are called USB (A) and USB (B). The RF-deck
// [REMOTE] path, the bridge address and LAN CI-V are outside this wave.
// A radio moved away from B2, an unsuitable USB virtual port, an absent/off
// radio and an unsuitable assumed baud all appear as silence. There is no
// --civ-address option and no automatic baud sweep.
//
// # Control-line safety
//
// PDF p.9 (folio 8) prints 1A 05 01 33, 1A 05 01 34 and 1A 05 01 35.
// Their domain assigns USB SEND, CW keying and RTTY keying to USB (A) DTR,
// USB (A) RTS, USB (B) DTR or USB (B) RTS. Therefore
// transport safety obligation 4 drives both RTS and DTR low while opening;
// the driver does not toggle either line afterwards. No lift below sends a
// 1A 05 frame: menu settings are made at the radio front panel.
//
// # Serial framing: the evidence, and it is an absence
//
// MANDATORY HAZARD SENTENCE (matrix §3.1). Icom manuals print
// "8 bit / 1 stop bit" style lines about the DATA/RTTY application port.
// SUCH A LINE IS NOT EVIDENCE about CI-V serial framing: only a statement
// explicitly about the CI-V, [REMOTE] or USB CI-V link counts.
//
// On this radio the hazard is vacuous:
// there is not even a misleading line to be misread.
// "stop bit", "parity", "data bit",
// "8 bit", "flow control", "bps", "baud", "4800", "9600", "19200",
// "38400" and "115200" appear NOWHERE in the document — about any port —
// across the whole extracted text and the rendered CI-V pages (PDF p.3
// folio 2 "CI-V connection"/"Preparing"/"About the data format" and
// PDF p.10 folio 9's 1A 05 01 50 – 1A 05 01 55 CI-V settings block, which
// carries a transceive switch, a bridge address, an ANT output flag, two
// echo-back flags and a USB (B) function selector, and no rate item at
// all). The one adjacent line, PDF p.3 (folio 2), names a "data
// communication speed" that "needs to be set when the cable is connected
// to the [REMOTE] jack on the RF deck's rear panel"; it states no framing.
//
// MATERIALITY: transport.DefaultStopBits is 2, so a driver that did not
// report its framing would open an IC-7760 at 8-N-2 against the tier's
// assumed 8-N-1. That is why StopBits exists here rather than being left
// to a default, and why the value carries the register entry below.
//
// # Assumption register
//
// Each entry below is the exact matrix register name and has exactly one
// model-specific lift. Every capture is from an IC-7760 at address B2.
//
//   - Register entry `ic7760-serial-framing` (ASSUMED). Lift: Stage R opens
//     the controller [USB B] CI-V port at 8-N-1 and 8-N-2, sends
//     FE FE B2 E0 19 00 FD at each setting and records which framing answers.
//   - Register entry `ic7760-control-lines` (ASSUMED). Lift: Stage R assigns
//     USB (A) RTS through the front-panel setting represented by
//     1A 05 01 33, opens the port with host-default line states and records
//     whether the IC-7760 enters transmit; 1A 05 01 34 and 1A 05 01 35 are
//     the adjacent CW/RTTY keying rows establishing the same hazard.
//   - Register entry `ic7760-default-baud` (ASSUMED). Lift: Stage R records
//     the factory CI-V baud menu value and whether USB answers regardless of
//     the host's nominal rate.
//   - Register entry `ic7760-baud-list` (ASSUMED). Lift: Stage R photographs
//     the controller's CI-V menu and records every selectable rate.
//   - Register entry `ic7760-address-menu` (ASSUMED). Lift: Stage R records
//     the menu's factory B2 value and complete selectable range.
//   - Register entry `ic7760-transceive-default` (ASSUMED). Lift: Stage R
//     observes an unmodified B2 radio for 60 seconds without sending and
//     records whether unsolicited traffic appears.
//   - Register entry `ic7760-broadcast-form` (ASSUMED). Lift: Stage R enables
//     transceive at the front panel, changes the VFO and records the emitted
//     frame's destination byte.
//   - Register entry `ic7760-echo-default` (ASSUMED). Lift: Stage R sends
//     FE FE B2 E0 19 00 FD on USB (A) and USB (B), recording the exact echo
//     and answer ordering on each.
//   - Register entry `ic7760-read-request-form` (ASSUMED). Lift: Stage R
//     sends FE FE B2 E0 1A 00 00 01 FD and captures the complete answer.
//   - Register entry `ic7760-empty-reply-fa` (ASSUMED). Lift: Stage R clears
//     memory 99 at the front panel, reads it at B2 and records whether FA is
//     returned.
//   - Register entry `ic7760-empty-reply-ff` (ASSUMED). Lift: in that same
//     Stage R empty-slot experiment, if a record rather than FA is returned,
//     the one captured record establishes whether all 25 bytes are FF.
//   - Register entry `ic7760-write-full-record` (ASSUMED). Lift: Stage W,
//     with owner consent, sends one full 25-byte B2 set to memory 99, records
//     FB or FA and reads the slot back.
//   - Register entry `ic7760-id-reply` (ASSUMED). Lift: Stage R sends
//     FE FE B2 E0 19 00 FD and records the raw reply token; the driver records
//     that token diagnostically and never matches it.
//   - Register entry `ic7760-clear-scope` (ASSUMED). Lift: a future Stage W
//     milestone, after a separate gate/spec change and explicit erase
//     consent, tests the printed memory-only clear scope. This driver admits
//     no clear grammar.
//   - Register entry `ic7760-freq-range` (ASSUMED). Lift: Stage R stores the
//     lowest and highest front-panel frequencies in memories and reads both
//     back at B2.
//   - Register entry `ic7760-usb-b-function` (ASSUMED). Lift: Stage R sends
//     FE FE B2 E0 19 00 FD to both enumerated virtual COM ports and records
//     which answers on an unmodified radio.
//
// The 99 MEM addresses plus P1/P2 inventory is MANUAL-EVIDENCED at PDF
// p.20 (folio 19) and PDF p.4 (folio 3), per additions-spec Erratum 5. It
// therefore has no assumption-register entry. P1/P2 share the drawn record
// shape only under the separately named profile assumption.
package ic7760

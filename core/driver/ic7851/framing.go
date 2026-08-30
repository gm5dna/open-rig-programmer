// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import "github.com/gm5dna/open-rig-programmer/core/driver"

// StopBits reports one stop bit for the CI-V link (8-N-1), per spec D3.1.
//
// ASSUMED, ON NO EVIDENCE FROM THIS RADIO'S DOCUMENT. The IC-7850/IC-7851
// Instruction Manual says nothing about serial framing anywhere: "stop
// bit", "start bit", "parity", "data bit", "8 bit", "flow control", "Xon"
// and "handshake" have zero hits across all 283 pages, about any port
// (matrix §3.1). doc.go §2 reproduces that sweep in full, names the four
// pages where such a statement would live and does not, and carries the
// mandatory hazard sentence about DATA/RTTY-port "8 bit / 1 stop" lines.
//
// Register entry: ic7851-serial-framing.
//
// THE ONE LIFT, on an IC-7851 itself: open its [USB B] CI-V port at
// 9600 8-N-1, send FE FE 8E E0 19 00 FD, and confirm an address-matched
// answer; then repeat at 8-N-2 and at 8-E-1 and record which framings the
// radio answers under. SCOPE: that capture settles which framing THAT
// radio's [USB B] CI-V endpoint accepts and nothing wider — not the
// [REMOTE] jack, not the [LAN] port, and not the IC-7850, which needs its
// own entry and its own lift.
//
// IT IS ON THE DRIVER, NOT THE SESSION, and that is forced:
// internal/wiring holds the driver value BEFORE the port is opened, and
// the stop bits are chosen at open. A session-side reporter could only be
// consulted after the framing had already been guessed.
//
// PROFILE-INDEPENDENT, and deliberately so: which capability set a caller
// asked for says nothing about how the radio frames a byte on the wire. An
// unrecognised Profile reports 1 like the rest, and TestStopBits exercises
// both rows and the simulated arm.
//
// MATERIALITY: transport.DefaultStopBits is 2, so a driver that did NOT
// implement this interface would have its port opened at 8-N-2 — the
// silent divergence from the tier's assumed 8-N-1 that spec D3.1 exists to
// prevent. internal/wiring consults this and REFUSES any value but 1 or 2
// rather than substituting a default, so a zero could never quietly become
// 8-N-2 either.
func (d *ic7851Driver) StopBits() int { return 1 }

// Compile-time proof that the CONCRETE driver — the value internal/wiring
// holds — carries the optional capability.
var _ driver.SerialFramingReporter = (*ic7851Driver)(nil)

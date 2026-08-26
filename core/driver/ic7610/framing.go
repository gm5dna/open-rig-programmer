// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import "github.com/gm5dna/open-rig-programmer/core/driver"

// StopBits reports one stop bit for the CI-V link (8-N-1), per spec D3.1.
//
// ASSUMED, ON NO EVIDENCE FROM THIS RADIO'S DOCUMENT. The IC-7610 CI-V
// Reference Guide says nothing about serial framing anywhere: the words
// "stop bit", "data bit", "parity" and "8 bit" appear in none of its 17
// pages, about any port (matrix §3.1, reproduced in full in doc.go with
// the four-row table naming which port each rate-bearing line is about,
// and with the mandatory hazard sentence about DATA/RTTY-port
// "8 bit / 1 stop" lines).
//
// Register home: D5 entry 8, "Serial framing 8-N-1 (D3.1) per model".
//
// LIFT R8 — Stage R capture ic7610-framing-8n1: with an IC-7610 at its
// factory CI-V settings, open its USB CI-V endpoint at 8-N-1 and then at
// 8-N-2, send FE FE 98 E0 19 00 FD at each, and record which framing
// returns a well-formed address-matched frame and which returns nothing or
// garbage. SCOPE: that capture settles which framing THAT radio's USB CI-V
// endpoint accepts, and nothing wider — not the [REMOTE] jack, not the
// [LAN] port, and not any other model.
//
// IT IS ON THE DRIVER, NOT THE SESSION, and enabler E2 records why that is
// forced: internal/wiring holds the driver value BEFORE the port is
// opened, and the stop bits are chosen at open. A session-side reporter
// could only be consulted after the framing had already been guessed.
//
// PROFILE-INDEPENDENT, and deliberately so: which capability set a caller
// asked for says nothing about how the radio frames a byte on the wire. An
// unrecognised Profile reports 1 like the rest.
//
// MATERIALITY: transport.DefaultStopBits is 2, so a driver that did NOT
// implement this interface would have its port opened at 8-N-2 — the
// silent divergence from the tier's assumed 8-N-1 that spec D3.1 exists to
// prevent. internal/wiring consults this and REFUSES any value but 1 or 2
// rather than substituting a default, so a zero could never quietly become
// 8-N-2 either.
func (d *ic7610Driver) StopBits() int { return 1 }

// Compile-time proof that the CONCRETE driver — the value internal/wiring
// holds — carries the optional capability.
var _ driver.SerialFramingReporter = (*ic7610Driver)(nil)

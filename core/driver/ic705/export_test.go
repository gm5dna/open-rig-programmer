// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import "github.com/gm5dna/open-rig-programmer/core/transport"

// This file is the standard Go export_test.go bridge: it is compiled ONLY
// into this package's tests, so nothing it names reaches the production
// surface, and its identifiers ARE visible to the external test package
// ic705_test — which is where the fake↔driver end-to-end lives, because
// the plan puts it there and because an external package is what proves
// the driver's EXPORTED surface is sufficient for a real consumer.
//
// It exists for exactly one reason. The end-to-end opens ten-odd sessions,
// each of which walks a thousand memory addresses, and transport.Engine
// paces every completed exchange with a 20 ms sleep — so an honest
// end-to-end would spend six minutes asleep. WithNoPacingForTest injects
// the same clock the in-package tests use (noSettleClock): real time
// everywhere it matters, and no sleep for the pacing delay alone. Every
// timeout, idle gap and drain cap still runs on the real clock, which is
// why the flood, drain and quarantine behaviour the end-to-end asserts is
// the production behaviour rather than a fast-forwarded imitation of it.

// WithNoPacingForTest returns an Option that removes the engine's
// inter-exchange pacing sleep and nothing else. Test-only; see this file's
// header.
func WithNoPacingForTest() Option {
	return withEngineOptions(transport.WithClock(noSettleClock{}))
}

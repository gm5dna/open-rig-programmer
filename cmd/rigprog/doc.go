// SPDX-License-Identifier: GPL-3.0-or-later

// Command rigprog is open-rig-programmer's command-line memory
// programmer for the Yaesu FT-710 — the project's composition root,
// wiring core/driver, core/driver/ft710, core/transport, core/clone,
// core/codeplug, core/csvio, core/spec, and internal/fakeradio into a
// single binary.
//
// Subcommands: ports, probe, read, write, diff, export, import. Task 11
// implemented ports and probe; task 12 added read and diff; task 13 added
// export and import (both OFFLINE — see export.go/import.go — never
// opening a radio session); task 14 added write (write.go) — the only
// subcommand that sends anything to a radio, reaching core/clone's
// Service.PrepareSend/Execute (the sole permitted write path; see
// internal/guards) and never core/driver's Session.WriteChannel
// directly.
//
// Radio-touching subcommands (probe, and later read/write/diff) take
// exactly one of --port PATH (a real serial port) or --fake (the
// in-process internal/fakeradio simulator) — never both, never neither.
// The pairing of the simulator with core/driver/ft710's Simulated
// profile is structurally exclusive from a real port: see this
// package's wiring.go.
//
// Human-readable command results go to stdout; progress, diagnostics,
// and errors go to stderr. Exit codes are a stable, script-facing
// contract — see exitcode.go.
package main

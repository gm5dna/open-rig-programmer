// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec describes WHAT a radio can do, in neutral terms, so that
// generic code (the UI, validation, and the clone service) never
// hardcodes facts about a specific radio. The design rule is: capability
// data drives generic code, and protocol quirks live in drivers.
//
// A radio driver package (e.g. the FT-710 driver, added in a later task)
// constructs one Capabilities value describing that radio: which memory
// banks it has, which fields are supported for each, and shared data like
// the CTCSS tone table. Generic code then asks that Capabilities value
// what it may do — it never asks "is this an FT-710?".
//
// The central piece of that design is the Support five-state
// (Unsupported/Unverified/Supported/Inert/ConsentedUnverified — Inert
// added at M5b for a field whose value a fixed-layout write frame must
// always transmit but the radio silently ignores, and
// ConsentedUnverified for an unproven write the user has explicitly
// accepted; see Support's own doc comment) and FieldSupport.CanWrite:
// this project hard-gates all writes to real hardware behind hardware
// verification sessions, and CanWrite() returning true only for a field
// whose Write support is Supported or ConsentedUnverified is the
// mechanism that enforces that gate — Inert, like plain Unverified and
// Unsupported, never makes CanWrite() true. Unverified capability data
// may still be surfaced read-only (e.g. documented-but-unproven fields
// shown in a UI), but on its own it can never authorise a write.
//
// This package imports nothing project-internal — not even core/cat — and
// performs no I/O, so that any layer can depend on it without pulling in
// a wire protocol or persistence format. It carries no JSON tags either:
// persistence is the codeplug package's concern, not this one's.
package spec

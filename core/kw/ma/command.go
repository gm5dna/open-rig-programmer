// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"bytes"
	"fmt"
)

// Command is an outbound MA-family command frame whose bytes were produced
// and validated by a builder in THIS package. The zero value is invalid; use
// IsZero to check for it (returned by fallible builders alongside their
// error).
//
// ma.Command AND kw.Command ARE DIFFERENT TYPES CARRYING DIFFERENT FAMILIES'
// FRAMES, and a reader who assumes they are one has assumed the whole safety
// argument away. kw.Command's only minter, kw's own newCommand, is
// UNEXPORTED by design — "only builders mint one" is what stops an
// unvalidated frame reaching the wire wearing the type the gate trusts — so a
// sibling codec must either be handed an exported minter, which weakens that
// invariant for the whole family, or declare its own. This is the second,
// which is the core/cat-and-core/civ precedent one family over: two dialects,
// two Command types, one transport.Command interface (spec decision 3).
// TestCommand_IsNotAKWCommand pins the non-identity.
//
// The contract is kw.Command's, restated: a check-then-write TOCTOU closure.
// A raw []byte can never leave this package pretending to be a validated
// command — only a builder can construct one, via the unexported newCommand —
// and Bytes() hands out a fresh, independent copy on every call. Without it a
// caller-held slice could be mutated after the gate approved it and before
// the transport wrote it, so the bytes the gate judged and the bytes the
// radio received would be different bytes.
//
// Command satisfies the neutral transport.Command interface (Bytes and
// String); layout.go asserts that with the COMPILER rather than leaving it
// to shape.
type Command struct {
	frame []byte
}

// newCommand builds a Command from frame. frame must already be a freshly
// allocated, non-aliased buffer this package's own builder just constructed
// — every builder satisfies that by construction (make/append into a local
// slice, never a caller-supplied one) — so newCommand does not copy on
// construction; the isolation guarantee lives in Bytes(), which is what
// callers outside this package actually use.
func newCommand(frame []byte) Command {
	return Command{frame: frame}
}

// Bytes returns a defensive copy of c's wire bytes. Every call allocates and
// returns an independent copy: callers may freely mutate what they get back,
// with no effect on c or on any other copy.
func (c Command) Bytes() []byte {
	return copyBytes(c.frame)
}

// String renders c safely for logs: %q-quoted, so control bytes, embedded
// quotes and any other non-printable or adversarial content cannot corrupt
// or spoof surrounding log output.
func (c Command) String() string {
	return fmt.Sprintf("%q", c.frame)
}

// IsZero reports whether c is the zero Command — never built by a package
// builder. Fallible builders return the zero Command alongside a non-nil
// error.
func (c Command) IsZero() bool {
	return c.frame == nil
}

// copyBytes returns an independent copy of b, and nil for nil.
//
// IT IS THIS PACKAGE'S OWN because kw's is unexported. One line on stdlib's
// bytes.Clone (which already returns nil for nil) — the alternative,
// exporting kw's, would put a copy helper on the family's public surface for
// no caller's benefit.
func copyBytes(b []byte) []byte {
	return bytes.Clone(b)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"bytes"
	"fmt"
)

// Command is an outbound Kenwood command frame whose bytes were produced
// and validated by a builder in this package. The zero value is invalid;
// use IsZero to check for it (returned by fallible builders alongside their
// error).
//
// It is core/cat's and core/civ's Command shape, restated rather than
// shared, for the reason the fence gives (imports_test.go): the two
// protocols have nothing in common below this type's contract.
//
// That contract is a check-then-write TOCTOU closure. A raw []byte can
// never leave this package pretending to be a validated command — only a
// builder can construct one, via the unexported newCommand — and Bytes()
// hands out a fresh, independent copy on every call. Without it a
// caller-held slice could be mutated after the gate approved it and before
// the transport wrote it, so the bytes the gate judged and the bytes the
// radio received would be different bytes.
//
// Command satisfies the neutral transport.Command interface (Bytes and
// String); framing.go asserts that with the COMPILER rather than leaving it
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

// Bytes returns a defensive copy of c's wire bytes. Every call allocates
// and returns an independent copy: callers may freely mutate what they get
// back, with no effect on c or on any other copy.
func (c Command) Bytes() []byte {
	return bytes.Clone(c.frame)
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

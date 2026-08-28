// SPDX-License-Identifier: GPL-3.0-or-later

package civ

// Command is an outbound CI-V command frame whose bytes were produced and
// validated by a builder in this package. The zero value is invalid; use
// IsZero to check for it (returned by fallible builders alongside their
// error).
//
// It is core/cat's Command shape, for core/cat's reason, restated here
// rather than shared: this package must never import core/cat (guarded in
// both directions), and the two protocols have nothing in common below the
// type's contract.
//
// That contract is a check-then-write TOCTOU closure. A raw []byte can
// never leave this package pretending to be a validated command — only a
// builder can construct one, via the unexported newCommand — and Bytes()
// hands out a fresh, independent copy on every call. Without it a
// caller-held slice could be mutated after AllowedCommand approved it and
// before the transport wrote it, so the bytes the gate judged and the
// bytes the radio received would be different bytes. With Command, the
// transport always writes bytes nobody else can reach.
//
// Command satisfies the neutral transport.Command interface (Bytes and
// String), and since framing.go that is asserted by the COMPILER rather
// than left to shape: `var _ transport.Command = Command{}` sits beside
// the adapter's own Framing assertion.
type Command struct {
	frame []byte
}

// newCommand builds a Command from frame. frame must already be a freshly
// allocated, non-aliased buffer this package's own builder just
// constructed — every builder satisfies that by construction (make/append
// into a local slice, never a caller-supplied one) — so newCommand does not
// copy on construction; the isolation guarantee lives in Bytes(), which is
// what callers outside this package actually use.
func newCommand(frame []byte) Command {
	return Command{frame: frame}
}

// Bytes returns a defensive copy of c's wire bytes. Every call allocates
// and returns an independent copy: callers may freely mutate what they get
// back, with no effect on c or on any other copy.
func (c Command) Bytes() []byte {
	return copyBytes(c.frame)
}

// String renders c for diagnostics as space-separated hex pairs — the form
// every Icom document prints its frames in, and the only readable one for
// a binary protocol. See hexFrame.
func (c Command) String() string {
	if len(c.frame) == 0 {
		return "<zero civ.Command>"
	}
	return hexFrame(c.frame)
}

// IsZero reports whether c is the zero Command — never built by a package
// builder. Fallible builders return the zero Command alongside a non-nil
// error.
func (c Command) IsZero() bool {
	return c.frame == nil
}

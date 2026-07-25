// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// Command is an outbound CAT command frame whose bytes were produced and
// validated by a builder in this package. The zero value is invalid; use
// IsZero to check for it (returned by fallible builders on error).
//
// Command exists so that a raw []byte can never leave this package
// pretending to be a validated command: only a builder can construct one
// (via the unexported newCommand), and its Bytes() method hands out a
// fresh, independent copy on every call. That closes a
// check-then-write-time-of-check-to-time-of-use (TOCTOU) window that a
// plain []byte return value left open — a caller-held slice could
// previously be mutated after AllowedCommand validated it but before the
// transport actually wrote it to the radio. With Command, the transport
// always writes bytes nobody else can reach.
type Command struct {
	frame []byte
}

// newCommand builds a Command from frame. frame must already be a freshly
// allocated, non-aliased buffer that this package's own builder just
// constructed — every builder in this package satisfies that by
// construction (make/append into a local slice, never a caller-supplied
// one) — so newCommand does not itself copy on construction; the
// isolation guarantee lives in Bytes(), which is what callers outside this
// package actually use to read the frame back out.
func newCommand(frame []byte) Command {
	return Command{frame: frame}
}

// Bytes returns a defensive copy of c's wire bytes. Every call allocates
// and returns an independent copy: callers may freely mutate the returned
// slice, and mutating one returned copy has no effect on c or on any other
// copy obtained from a previous or later call.
func (c Command) Bytes() []byte {
	return copyBytes(c.frame)
}

// String renders c safely for logs: %q-quoted, so control bytes, embedded
// quotes, and any other non-printable or adversarial content cannot
// corrupt or spoof surrounding log output.
func (c Command) String() string {
	return fmt.Sprintf("%q", c.frame)
}

// IsZero reports whether c is the zero Command — never built by a package
// builder. Fallible builders return the zero Command alongside a non-nil
// error.
func (c Command) IsZero() bool {
	return c.frame == nil
}

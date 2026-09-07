// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"io"
)

// This file holds the fake's runtime plumbing: the port the driver opens onto
// internal/fakepipe's in-memory line. Nothing here knows a CI-V byte from any
// other byte.

// port is the reader/writer the driver opens: it reads what the radio said and
// writes what the controller says.
type port struct{ radio *Radio }

func (p *port) Read(b []byte) (int, error)  { return p.radio.pipe.Host().Read(b) }
func (p *port) Write(b []byte) (int, error) { return p.radio.pipe.Host().Write(b) }

// Close closes the whole fake, not just this end of it: a driver that closes
// its port has finished with the radio, and the radio's emitter goroutines must
// stop with it rather than outlive the test that made them.
func (p *port) Close() error { return p.radio.Close() }

var _ io.ReadWriteCloser = (*port)(nil)

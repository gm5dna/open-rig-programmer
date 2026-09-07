// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import (
	"io"
)

// This file holds the fake's runtime plumbing — the port the driver opens onto
// internal/fakepipe's in-memory line, and the memory image. Nothing here knows
// a CI-V byte from any other byte, and nothing here interprets a record.

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

// slot is one memory channel of the fake's image. A slot that is not occupied
// is one a read cannot satisfy — which is what WithEmptySlot asks for and what
// a channel that was never seeded gets anyway.
type slot struct {
	record   []byte
	occupied bool
}

// image is the fake's memory: channels keyed by their three address bytes.
//
// The key is the RAW address bytes as they arrived on the wire, not a decoded
// (bank, channel) pair. That is deliberate: a request naming a channel this
// package would refuse to seed simply misses the map, rather than being decoded
// into something it is not.
type image struct {
	slots map[string]slot
}

func newImage() *image { return &image{slots: make(map[string]slot)} }

// seed installs one slot before the radio starts answering.
func (i *image) seed(addr []byte, record []byte, occupied bool) {
	i.slots[string(addr)] = slot{record: append([]byte(nil), record...), occupied: occupied}
}

// read returns a copy of the record at addr, and whether that channel is
// occupied at all.
func (i *image) read(addr []byte) ([]byte, bool) {
	s, ok := i.slots[string(addr)]
	if !ok || !s.occupied {
		return nil, false
	}
	return append([]byte(nil), s.record...), true
}

// write stores a record at addr and marks the channel occupied.
func (i *image) write(addr []byte, record []byte) {
	i.slots[string(addr)] = slot{record: append([]byte(nil), record...), occupied: true}
}

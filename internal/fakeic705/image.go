// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

import "fmt"

// Slot names one memory slot by the two numbers the memory-record address
// carries: the memory group and the memory channel. It is the map key of an
// Image and the argument pair of every slot-addressed call in this package.
//
// The numbers are DECIMAL, not the packed BCD they travel as. Group 100 is the
// call-channel group — the printed value "0100" — and it is written here as the
// integer 100, so that a caller never has to think in nibbles to seed a slot.
type Slot struct {
	Group   int
	Channel int
}

// String renders a slot the way the diagram's own address field prints it:
// four digits of group, four of channel.
func (s Slot) String() string { return fmt.Sprintf("%04d/%04d", s.Group, s.Channel) }

// Image is a set of memory slots and the records they hold — the whole of a
// fake radio's memory at construction, seeded in one go by WithFactoryImage.
//
// The records are OPAQUE BYTE STRINGS to this package. It stores them, serves
// them and compares them; it never reads a field out of one. That is the
// deliberate shape of this fake (doc.go, "What this fake knows about a record"),
// and it is what lets an Image hold a record of any length at all — including
// the wrong length, which is exactly what a test proving a driver's length
// fingerprint needs.
//
// A slot ABSENT from the map is an unwritten channel: a read of it draws NG.
// A slot present with a zero-length record is PRESENT — occupied, holding
// nothing — which is a different thing and answers a read with a record of no
// bytes.
type Image map[Slot][]byte

// RecordLen is the number of bytes of memory record a `1A 00` set must carry
// after its four address bytes for this fake to accept it: 111.
//
// Where 111 comes from, and why this package does not re-derive it: the
// memory-content diagram's own byte positions run 1 to 115, of which positions
// 1-4 are the group and channel address (the two transcripts under
// core/civ/ic705/testdata agree on both numbers, independently measured), so a
// record is 115 - 4 = 111 bytes. NOTHING IN THIS PACKAGE KNOWS WHAT ANY OF
// THOSE 111 BYTES MEANS.
const RecordLen = 111

// BlankRecord returns a fresh record of the accepted length, all zero bytes.
//
// It is a FIXTURE, not a factory-default memory channel: no IC-705 has ever
// been read by this project, and the diagram documents each field's vocabulary
// and never a shipped value, so there is nothing to source a real one from
// (doc.go, register entry 8; PROVENANCE.md). Zero is chosen because it is the
// only fill that asserts nothing: it is a legal packed-BCD digit, and it is
// what the diagram's one printed literal — the second nibble of diagram byte
// 15, labelled "Fixed" — already prints.
func BlankRecord() []byte { return make([]byte, RecordLen) }

// DefaultImage returns a fresh Image seeded with a handful of occupied slots,
// each holding a BlankRecord: three memory channels in group 0, one in group 1,
// and the four call channels.
//
// It is NOT what New() starts with. A fake radio with no options is EMPTY —
// see New's doc comment for why — and this fixture is opt-in via
// WithFactoryImage, so that a test's own seeding is never competing with a
// default it did not ask for.
//
// The SHAPE is what it exists for: a sparse, discontinuous inventory with two
// populated groups and a populated call-channel group, so that a walk that
// enumerates slots has something to find, something to skip, and a group
// boundary to cross. The CONTENTS assert nothing (BlankRecord).
func DefaultImage() Image {
	img := Image{}
	for _, s := range []Slot{
		{Group: 0, Channel: 0},
		{Group: 0, Channel: 1},
		{Group: 0, Channel: 7},
		{Group: 1, Channel: 0},
		{Group: callChannelGroup, Channel: 0},
		{Group: callChannelGroup, Channel: 1},
		{Group: callChannelGroup, Channel: 2},
		{Group: callChannelGroup, Channel: 3},
	} {
		img[s] = BlankRecord()
	}
	return img
}

// EmptyImage returns a fresh Image with no slots at all — a radio nobody has
// ever programmed. It is what New() uses when no image is given, and it exists
// as a named constructor so that a test asking for an empty radio says so.
func EmptyImage() Image { return Image{} }

// Clone returns an independent copy of the image, records included. Seeding a
// Radio clones, so a caller may keep and reuse an Image without a radio's
// writes reaching into it.
func (i Image) Clone() Image {
	out := make(Image, len(i))
	for slot, rec := range i {
		out[slot] = append([]byte(nil), rec...)
	}
	return out
}

// With returns the image with one slot set, so that a fixture can be built in
// one expression. It mutates and returns the receiver; use Clone first if that
// matters.
func (i Image) With(group, channel int, record []byte) Image {
	mustBeAddressable(group, channel)
	i[Slot{Group: group, Channel: channel}] = append([]byte(nil), record...)
	return i
}

// mustBeAddressable panics unless group and channel fit the four packed-BCD
// digits the address field gives each of them.
//
// It does NOT check the group and channel RANGES the manual states (0000-0099
// plus 0100, and that group's own channel range). Those are WIRE rules,
// enforced where the wire is read, and an operator seeding state deliberately
// stands outside them: a test may want a slot the radio would refuse to
// address, precisely to prove that the refusal is about the address and not
// about what is behind it. What cannot be tolerated is a number the address
// field could not carry at all, which is a programming error in the test and is
// made to stop the programme rather than encode a silently wrong slot.
func mustBeAddressable(group, channel int) {
	if group < 0 || group > 9999 {
		panic(fmt.Sprintf("fakeic705: memory group %d does not fit the address field's four packed-BCD digits (0-9999)", group))
	}
	if channel < 0 || channel > 9999 {
		panic(fmt.Sprintf("fakeic705: memory channel %d does not fit the address field's four packed-BCD digits (0-9999)", channel))
	}
}

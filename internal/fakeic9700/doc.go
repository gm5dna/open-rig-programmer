// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic9700 is a fake IC-9700 on the far end of a pipe: a CI-V
// transceiver that answers frames, holds a memory image, and can be told to
// misbehave in the specific ways a driver test needs to see.
//
// # THE HARD RULE
//
// This package imports THE STANDARD LIBRARY AND NOTHING ELSE. Not
// core/civ/ic9700, not its profile, not its golden vectors, not its field
// ledger, not core/driver, not core/codeplug, not core/spec, and not another
// fake. imports_test.go proves it, walking this directory and every directory
// beneath it, and it landed before any of the code below.
//
// The rule is not tidiness. A fake exists to be the OTHER witness in a test:
// the driver says what it believes the radio will say, the fake says what it
// believes the radio will say, and the test is worth something only because the
// two beliefs were formed separately. Let this package import the dialect it is
// tested against and the two witnesses become one — a systematic misreading of
// the record would agree with itself end to end and every test would go green
// while proving nothing.
//
// # WHERE THIS FAKE'S KNOWLEDGE CAME FROM
//
// Two kinds of fact, from two separate places, and no third place.
//
// The WIRE facts are quoted in this package's own brief out of the IC-9700 CI-V
// Reference Guide (p.4/folio 3, p.6/folio 5, p.13/folio 12): the FE FE …  FD
// frame, the A2 and E0 addresses, the FB and FA codes, 19 00 and 1A 00, the
// to=00 broadcast, and the up-to-119 preamble bytes. They are in parser.go, and
// none of them is a record secret.
//
// The RECORD facts are two, and only two, and they come from two independent
// transcriptions of the radio's printed memory-record diagrams — one carrying
// each field's meaning and values, one carrying each field's measured byte and
// nibble positions. They are set out in full at the top of image.go, and
// PROVENANCE.md names the artefacts. Nothing else about the record is known
// here, and nothing else about it is needed: WithSlot hands this package a raw
// record, and it stores and serves those bytes back without interpreting one of
// them.
//
// No IC-9700 was ever asked anything.
//
// # THE RECORD LENGTH STOP
//
// The two artefacts do not agree on how long a memory record is, and this
// package does not resolve it.
//
// The semantic leg measures 114 bytes — one per drawn cell, with each elided
// group taken at the length of its printed index range — and says plainly that
// the document prints no total and no byte addresses, so 114 is its measurement
// and not a printed figure. The geometry leg counts what the picture actually
// draws: 38 cells, of which several are dotted continuation boxes standing for
// an unstated number of omitted bytes, and it records seven separate STOPs
// rather than reconcile the two counts. Neither leg claims a length the other
// confirms.
//
// So this package HAS no record length. It serves the length it was handed —
// whatever WithSlot seeded, or whatever WithRecordLength said — and it enforces
// a write's length only once one of those has told it what length it serves.
// Given neither, it enforces nothing. That is the STOP recorded rather than
// worked around: a fake that picked 114, or 38, would be asserting a fact its
// evidence does not carry, and a driver test that passed against it would be
// evidence of agreement with a guess.
//
// The related divergence — the printed field indices running 1 to 67 while the
// drawn cells run 1 to 38, the gap widening at each continuation box — is the
// same STOP seen from the other side. It is why this package refuses to know
// any byte offset past position 4, and why it addresses a channel by the three
// bytes both legs agree on and nothing more.
//
// # WHAT IT ANSWERS
//
// Frames addressed to its own address, and nothing else — a frame naming
// another address gets no reply at all, not even a reject. Its answers name
// itself as from.
//
//   - 19 00 gets an ID reply carrying this radio's own CI-V address.
//   - 1A 00 with a channel address and no more is a read: the seeded record if
//     that channel is occupied, the NG code if it is not.
//   - 1A 00 with a channel address and a record is a write: the OK code and the
//     record stored, or the NG code if its length is not the length being
//     served.
//   - 1A 00 in the printed clearing form — the address, then FF where field ④
//     stands, and nothing after — is refused.
//   - anything else addressed to it gets the NG code.
//
// It tolerates leading noise and any number of repeated preamble bytes, and it
// records every frame it received for Transcript.
//
// # WHAT IT CAN BE TOLD TO DO WRONG
//
// Each option exists for a fault a driver has to survive, and they are not
// interchangeable with one another:
//
//   - WithEmptySlot: a channel that answers the reject code.
//   - WithRecordLength: answers of a length the driver did not expect. It
//     changes the LENGTH of the answers occupied slots give and does not
//     occupy anything by itself, so a wrong-length ANSWER needs a WithSlot too.
//   - WithBroadcasts and WithAddressedFlood: two DIFFERENT species of
//     unsolicited traffic. Broadcasts carry to=00 and a controller's
//     accumulator drops them, so they never reach its engine; the flood is
//     addressed to the controller and does reach it, which is the only reason a
//     drain cap is reachable at all. A test that wants to prove a cap needs the
//     second; one that wants to prove noise is ignored needs the first.
//   - WithAnswerAddress: an answer that is perfectly well formed and names the
//     wrong channel.
//   - WithEchoBack: a line that echoes, so that the driver's own frames come
//     back at it before the answer does.
package fakeic9700

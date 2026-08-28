// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	// ALIASED deliberately, and every test file in this package does the
	// same. The DIALECT package's own name is also "ic9700", and so is
	// this driver package's; an unaliased import would put two meanings
	// on one spelling inside a single test package, so the CI-V side is
	// always civic9700 here and the bare ic9700 always means the driver.
	// core/driver/ftdx101's catftdx101 alias is the house precedent.
	civic9700 "github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// THIS FILE CONSUMES ENABLER E1; IT DOES NOT RE-TEST IT. The CI-V framing
// adapter, the three answer matchers and the two CommandSpec helpers all
// live in core/civ/framing.go and carry their own suite there. What is
// proved here is narrower and is this worktree's own business: that THIS
// PROFILE works through that landed adapter, and that the one property the
// driver's own T2 check exists to compensate for — the memory matcher's
// deliberate envelope-only reach — is true of this radio's frames too.
//
// NO LOCAL ADAPTER, NO LOCAL MATCHER, NO LOCAL DrainPolicy (adjudication
// R1). Five radio worktrees writing one shared file is five conflicting
// versions of it, and a duplicate in a second home is worse than a missing
// one. Nothing under core/civ is created or modified by this file's task.

// memoryAnswerFrame builds a well-formed `1A 00` memory ANSWER for the
// given band and channel, from the golden record, as THIS radio would
// send it.
//
// It is built with the profile's OWN builder and then reversed at the
// envelope, rather than assembled byte by byte: BuildMemorySet emits
// `FE FE A2 E0 1A 00 <addr> <record> FD` — controller to radio — and an
// answer is the same body with the two address bytes the other way round.
// Hand-assembling the 121 bytes here would put a second, unchecked copy of
// the record geometry in a test file, where a transcription slip would
// look like a matcher failure.
func memoryAnswerFrame(t *testing.T, band, channel int) []byte {
	t.Helper()
	return memoryAnswerFromAddress(t, civic9700.Profile().RadioAddress(), band, channel)
}

// memoryAnswerFromAddress is memoryAnswerFrame with the ANSWERING radio's
// address chosen by the caller, so a test can serve a frame from a station
// that is not this radio. addr is the `from` byte of the answer.
func memoryAnswerFromAddress(t *testing.T, addr byte, band, channel int) []byte {
	t.Helper()
	rec := goldenRecordAt(band, channel)
	cmd, err := civic9700.Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet(%v): %v", rec.Address, err)
	}
	frame := append([]byte(nil), cmd.Bytes()...)
	// frame[2] is `to` and frame[3] is `from` on a command; an answer
	// carries the controller in `to` and the radio in `from`.
	frame[2] = civ.ControllerAddressDefault
	frame[3] = addr
	return frame
}

// goldenRecordAt is leg G's transcribed record, re-addressed. Every one of
// the fourteen mapped field ids is present, because civ's own validator
// requires a value for every field the layout maps.
func goldenRecordAt(band, channel int) civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: band, Channel: channel},
		Select:       civ.Available("OFF"),
		RXFreqHz:     civ.Available(uint64(145_500_000)),
		Mode:         civ.Available("FM"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		Duplex:       civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		ToneTXDeciHz: civ.Available(uint64(885)),
		ToneRXDeciHz: civ.Available(uint64(885)),
		DTCSPolarity: civ.Available("NN"),
		DTCSCode:     civ.Available(uint64(23)),
		OffsetHz:     civ.Available(uint64(600_000)),
		TXFreqHz:     civ.Available(uint64(145_500_000)),
		Name:         civ.Available("INVERNESS GB3CFR"),
	}
}

func TestProfileBuildsAFramingAdapter(t *testing.T) {
	f, err := civ.NewFraming(civic9700.Profile())
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	if got := f.InitSequence(); len(got) != 0 {
		t.Fatalf("InitSequence has %d commands, want 0 — CI-V Init writes NOTHING to a radio, ever", len(got))
	}
	if got := f.DrainPolicy().Cap; got <= 0 {
		t.Error("DrainPolicy has no absolute cap; a transceive flood could wedge the open")
	}
}

func TestUnconfiguredProfileBuildsNoFraming(t *testing.T) {
	if _, err := civ.NewFraming(civ.Profile{}); err == nil {
		t.Fatal("a zero profile produced a framing adapter")
	}
}

func TestTheThreeMatchersAcceptThisRadiosAnswersAndRejectAnothers(t *testing.T) {
	// The matchers come FROM THE CODEC (adjudication (a)) and E1 makes
	// the ack SOURCE-ADDRESS-CHECKED. All THREE are exercised.
	p := civic9700.Profile()
	ack := p.AcknowledgementMatcher()
	id := p.TransceiverIDAnswerMatcher()
	mem := p.MemoryAnswerMatcher()

	for _, tc := range []struct {
		name    string
		matcher func([]byte) bool
		frame   []byte
		want    bool
	}{
		{"our ack", ack, []byte{0xFE, 0xFE, 0xE0, 0xA2, 0xFB, 0xFD}, true},
		{"another radio's ack", ack, []byte{0xFE, 0xFE, 0xE0, 0x94, 0xFB, 0xFD}, false},
		{"our nak is not an ack", ack, []byte{0xFE, 0xFE, 0xE0, 0xA2, 0xFA, 0xFD}, false},
		{"our ID answer", id, []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x19, 0x00, 0xA2, 0xFD}, true},
		{"another radio's ID answer", id, []byte{0xFE, 0xFE, 0xE0, 0x94, 0x19, 0x00, 0x94, 0xFD}, false},
		{"a memory answer is not an ID answer", id, memoryAnswerFrame(t, 1, 1), false},
		{"our memory answer", mem, memoryAnswerFrame(t, 1, 1), true},
		{"another radio's memory answer", mem, memoryAnswerFromAddress(t, 0x94, 1, 1), false},
		{"an ID answer is not a memory answer", mem, []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x19, 0x00, 0xA2, 0xFD}, false},
	} {
		if got := tc.matcher(tc.frame); got != tc.want {
			t.Errorf("%s: matcher = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTheMemoryMatcherIsEnvelopeOnlyWhichIsWhyT2Exists(t *testing.T) {
	// LANDED implementation decision 3: MemoryAnswerMatcher checks the
	// envelope and the minimum address width, and matches NO channel
	// address (core/civ/framing.go). This test pins that fact HERE so the
	// driver-side T2 equality check is visibly load-bearing rather than
	// belt-and-braces.
	p := civic9700.Profile()
	mem := p.MemoryAnswerMatcher()
	if !mem(memoryAnswerFrame(t, 1, 1)) || !mem(memoryAnswerFrame(t, 3, 99)) {
		t.Fatal("the matcher should accept ANY well-formed memory answer from this radio")
	}
	// Both match: the matcher cannot tell the driver which slot answered.
	// Only the driver's decoded-address comparison can (T2).
}

func TestTheTwoSpecHelpersStateTheirClassAndTheirRetryPolicy(t *testing.T) {
	// The two helpers are what keeps this package free of a
	// transport.CommandSpec literal (adjudication R1). What they promise
	// is checked here, once, so that every later call site can simply use
	// them: a read is a ClassRead that may be retried, and an acknowledged
	// write is a ClassWriteWithAck that NEVER is.
	p := civic9700.Profile()

	read := civ.CIVReadSpec(p.MemoryAnswerMatcher(), 2)
	if read.Class != transport.ClassRead {
		t.Errorf("CIVReadSpec Class = %v, want ClassRead", read.Class)
	}
	if read.RetryReads != 2 {
		t.Errorf("CIVReadSpec RetryReads = %d, want the 2 it was given", read.RetryReads)
	}
	if read.Match == nil {
		t.Error("CIVReadSpec carries no matcher")
	}

	write := civ.CIVWriteWithAckSpec(p.AcknowledgementMatcher())
	if write.Class != transport.ClassWriteWithAck {
		t.Errorf("CIVWriteWithAckSpec Class = %v, want ClassWriteWithAck", write.Class)
	}
	if write.RetryReads != 0 {
		t.Errorf("CIVWriteWithAckSpec RetryReads = %d, want 0 — a write is never resent", write.RetryReads)
	}
}

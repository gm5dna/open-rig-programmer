// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"testing"
)

// answerFor turns a command frame into the answer the radio would send:
// the same bytes with the two address bytes swapped. Tests use it to reach
// the parsers without a second builder that only tests would need — and,
// deliberately, it produces a frame the gate must REFUSE.
func answerFor(frame []byte) []byte {
	out := copyBytes(frame)
	out[2], out[3] = out[3], out[2]
	return out
}

// TestEveryBuilderOutputIsAdmittedByItsOwnGate is this package's central
// safety property. A builder and a gate that disagree are worse than
// either being wrong alone: the program cannot send a command it believes
// is valid, or the gate is not checking what the builder emits.
func TestEveryBuilderOutputIsAdmittedByItsOwnGate(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p
			built := map[string]int{}

			check := func(what string, frame []byte) {
				t.Helper()
				built[what]++
				if !WellFormed(frame) {
					t.Fatalf("%s produced a malformed frame %s", what, hexFrame(frame))
				}
				if len(frame) > p.MaxFrame() {
					t.Fatalf("%s produced %d bytes, past this profile's own %d-byte bound", what, len(frame), p.MaxFrame())
				}
				if !p.AllowedCommand(frame) {
					t.Fatalf("its own gate REFUSED %s frame %s", what, hexFrame(frame))
				}
			}

			check("transceiver ID read", mustCommand(p.BuildTransceiverIDRead()).Bytes())

			lo, hi := p.ChannelRange()
			groups := 1
			if p.AddressForm() != AddressFormFlat {
				groups = p.Groups()
			}
			for g := 0; g < groups; g++ {
				grp := g
				if p.AddressForm() == AddressFormFlat {
					grp = 0
				}
				for _, ch := range []int{lo, (lo + hi) / 2, hi} {
					addr := ChannelAddress{Group: grp, Channel: ch}
					check("memory read", mustCommand(p.BuildMemoryRead(addr)).Bytes())
				}
			}

			for _, length := range p.RecordLengths() {
				if length != p.BuildRecordLength() {
					continue // the builder emits one length; the gate accepts all
				}
				rec := sampleRecord(t, p, length)
				check("memory set", mustCommand(p.BuildMemorySet(rec)).Bytes())
			}

			for _, what := range []string{"transceiver ID read", "memory read", "memory set"} {
				if built[what] == 0 {
					t.Errorf("builder %q contributed no frames — either this profile refuses it for every input this test can offer, or the builder was dropped from the walk, and both are defects this property must not pass over", what)
				}
			}
			t.Logf("%s: %v", p.Model(), built)
		})
	}
}

// TestGateRefusesEveryOtherProfilesFrames is the peer property: the gate
// judges for the profile it is called on and no other.
//
// It is the check a package-level datum cannot pass. Every fixture
// disagrees about its address, so a gate reading the receiver refuses its
// neighbours' frames and a gate reading a global admits them.
func TestGateRefusesEveryOtherProfilesFrames(t *testing.T) {
	type built struct {
		owner string
		frame []byte
	}
	var all []built
	for _, np := range allTestProfiles() {
		p := np.p
		for _, c := range []Command{
			mustCommand(p.BuildTransceiverIDRead()),
			mustCommand(p.BuildMemoryRead(sampleAddress(p))),
			mustCommand(p.BuildMemorySet(sampleRecord(t, p, p.BuildRecordLength()))),
		} {
			all = append(all, built{np.name, c.Bytes()})
		}
	}
	if len(all) < 6 {
		t.Fatalf("only %d frames were built — this check would be nearly vacuous", len(all))
	}

	refusals := 0
	for _, np := range allTestProfiles() {
		for _, b := range all {
			if b.owner == np.name {
				continue
			}
			if np.p.AllowedCommand(b.frame) {
				t.Errorf("%s's gate ADMITTED %s's frame %s — a gate consulting a package-level datum instead of its receiver would do exactly this", np.name, b.owner, hexFrame(b.frame))
				continue
			}
			refusals++
		}
	}
	if refusals == 0 {
		t.Fatal("no cross-profile refusal was observed — the walk is broken")
	}
	t.Logf("%d cross-profile frames refused", refusals)
}

// TestGateRefusesTheUnacceptable is what stops "every built frame is
// admitted by its own gate" from being satisfied by a gate that admits
// everything.
func TestGateRefusesTheUnacceptable(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p
			radio, ctrl := p.RadioAddress(), p.ControllerAddress()
			set := mustCommand(p.BuildMemorySet(sampleRecord(t, p, p.BuildRecordLength()))).Bytes()
			read := mustCommand(p.BuildMemoryRead(sampleAddress(p))).Bytes()

			cases := []struct {
				what  string
				frame []byte
			}{
				{"nil", nil},
				{"empty", []byte{}},
				{"a lone preamble", []byte{PreambleByte}},
				{"a frame with no body", []byte{PreambleByte, PreambleByte, radio, ctrl, EndByte}},
				{"one preamble byte only", []byte{PreambleByte, radio, ctrl, CmdTransceiverID, SubTransceiverID, EndByte}},
				{"no terminator", []byte{PreambleByte, PreambleByte, radio, ctrl, CmdTransceiverID, SubTransceiverID}},
				{"an ACKNOWLEDGEMENT", []byte{PreambleByte, PreambleByte, radio, ctrl, AckByte, EndByte}},
				{"a REJECTION", []byte{PreambleByte, PreambleByte, radio, ctrl, NakByte, EndByte}},
				{"the transceiver-ID ANSWER shape", []byte{PreambleByte, PreambleByte, radio, ctrl, CmdTransceiverID, SubTransceiverID, radio, EndByte}},
				{"19 01 — a sub-command this tier does not build", []byte{PreambleByte, PreambleByte, radio, ctrl, CmdTransceiverID, 0x01, EndByte}},
				{"1A 01 — the memory-keyer surface", []byte{PreambleByte, PreambleByte, radio, ctrl, CmdMemory, 0x01, 0x00, 0x01, EndByte}},
				{"1A 05 — the menu surface this tier refuses outright", []byte{PreambleByte, PreambleByte, radio, ctrl, CmdMemory, 0x05, 0x00, 0x01, 0x00, EndByte}},
				{"1C 00 — a command number no builder names", []byte{PreambleByte, PreambleByte, radio, ctrl, 0x1C, 0x00, 0x00, EndByte}},
				{"0F 01 — the split/duplex surface", []byte{PreambleByte, PreambleByte, radio, ctrl, 0x0F, 0x01, EndByte}},
				{"addressed to the broadcast address", withAddresses(read, 0x00, ctrl)},
				{"addressed to another station", withAddresses(read, radio^0x01, ctrl)},
				{"addressed to the CONTROLLER — an answer, never a command", withAddresses(read, ctrl, radio)},
				{"claiming to be from the radio", withAddresses(read, radio, radio)},
				{"claiming to be from another controller", withAddresses(read, radio, ctrl^0x01)},
				{"a memory read one address byte short", truncateBody(read, 1)},
				{"a memory set one record byte short", truncateBody(set, 1)},
				{"the ANSWER to a memory set", answerFor(set)},
				{"the ANSWER to a memory read", answerFor(read)},
			}
			// A channel one past this profile's own ceiling, hand-built
			// rather than through the builder (which refuses it).
			_, hi := p.ChannelRange()
			if hi < maxChannelDecimal {
				over := copyBytes(read)
				ch, err := encodeBCDNumber(uint64(hi+1), 2, OrderBigEndian)
				if err == nil {
					copy(over[len(over)-1-2:], ch)
					cases = append(cases, struct {
						what  string
						frame []byte
					}{"a channel past this profile's own ceiling", over})
				}
			}
			refused := 0
			for _, tc := range cases {
				if p.AllowedCommand(tc.frame) {
					t.Errorf("its gate ADMITTED %s: %s", tc.what, hexFrame(tc.frame))
					continue
				}
				refused++
			}
			if refused != len(cases) {
				t.Errorf("%d of %d unacceptable frames were refused", refused, len(cases))
			}
			if refused == 0 {
				t.Error("nothing was offered to the gate — this check would pass on a gate that admits everything")
			}
			t.Logf("%s: %d unacceptable frames refused", p.Model(), refused)
		})
	}
}

// TestGateRefusesARecordItsOwnBuilderWouldNotHaveWritten is the re-encode
// rule's own test: a frame whose record is well-formed field by field but
// carries a byte no builder would emit is refused.
func TestGateRefusesARecordItsOwnBuilderWouldNotHaveWritten(t *testing.T) {
	p := flatProfile
	set := mustCommand(p.BuildMemorySet(sampleRecord(t, p, p.BuildRecordLength()))).Bytes()
	if !p.AllowedCommand(set) {
		t.Fatal("the builder's own frame was refused before the mutation")
	}

	// Byte 11's nibbles are the filter and the data mode, and this
	// profile defines filter values 0..2 only. A 3 in the high nibble
	// decodes to nothing, so the gate refuses on the enum.
	unknownEnum := copyBytes(set)
	unknownEnum[6+2+11] = 0x30 | (unknownEnum[6+2+11] & 0x0F)
	if p.AllowedCommand(unknownEnum) {
		t.Error("the gate admitted a record carrying a filter value this profile does not define")
	}

	// The name field's last byte set to a charset byte that is not the pad
	// still round-trips, so THAT one must be admitted — the check above
	// must not be passing because the gate refuses every mutation.
	nameEnd := copyBytes(set)
	nameEnd[6+2+36] = 'Z'
	if !p.AllowedCommand(nameEnd) {
		t.Error("the gate refused a record its own builder COULD have written — the re-encode rule is too strict, and the check above proves nothing")
	}

	// A name byte outside the charset is refused.
	badName := copyBytes(set)
	badName[6+2+27] = 0x01
	if p.AllowedCommand(badName) {
		t.Error("the gate admitted a name byte outside this profile's charset")
	}
}

// TestParsersRoundTripEveryBuiltCommand walks the answer direction.
func TestParsersRoundTripEveryBuiltCommand(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p

			// The transceiver-ID answer: address bytes swapped, one data
			// byte appended before the terminator.
			idCmd := mustCommand(p.BuildTransceiverIDRead()).Bytes()
			answer := answerFor(idCmd)
			answer = append(answer[:len(answer)-1], p.RadioAddress(), EndByte)
			token, err := p.ParseTransceiverID(answer)
			if err != nil {
				t.Fatalf("ParseTransceiverID(%s): %v", hexFrame(answer), err)
			}
			if token == "" {
				t.Error("ParseTransceiverID returned an empty token")
			}

			// The memory answer: the SET frame with its addresses swapped
			// is exactly the answer's shape.
			rec := sampleRecord(t, p, p.BuildRecordLength())
			setCmd := mustCommand(p.BuildMemorySet(rec)).Bytes()
			back, err := p.ParseMemoryAnswer(answerFor(setCmd))
			if err != nil {
				t.Fatalf("ParseMemoryAnswer: %v", err)
			}
			if back != rec {
				t.Fatalf("memory answer round trip:\n got %+v\nwant %+v", back, rec)
			}
		})
	}
}

func TestParsersRefuseTheWrongDirectionAndTheWrongRadio(t *testing.T) {
	p := flatProfile
	setCmd := mustCommand(p.BuildMemorySet(sampleRecord(t, p, p.BuildRecordLength()))).Bytes()
	answer := answerFor(setCmd)

	if _, err := p.ParseMemoryAnswer(setCmd); err == nil {
		t.Error("ParseMemoryAnswer accepted a COMMAND frame — on a bus that echoes it would read this program's own writes as answers")
	}
	if _, err := p.ParseMemoryAnswer(withAddresses(answer, p.ControllerAddress(), p.RadioAddress()^0x01)); err == nil {
		t.Error("ParseMemoryAnswer accepted a frame from another radio")
	}
	if _, err := p.ParseMemoryAnswer(withAddresses(answer, 0x00, p.RadioAddress())); err == nil {
		t.Error("ParseMemoryAnswer accepted a broadcast frame")
	}
	if _, err := p.ParseTransceiverID(answer); err == nil {
		t.Error("ParseTransceiverID accepted a memory answer")
	}
}

func TestParseMemoryAnswer_WrongRecordLengthIsAnError(t *testing.T) {
	p := flatProfile
	setCmd := mustCommand(p.BuildMemorySet(sampleRecord(t, p, p.BuildRecordLength()))).Bytes()
	answer := answerFor(setCmd)

	// One byte short: a length no layout claims.
	short := append(copyBytes(answer[:len(answer)-2]), EndByte)
	_, err := p.ParseMemoryAnswer(short)
	if err == nil {
		t.Fatal("ParseMemoryAnswer accepted a record of an unaccepted length — spec D4 makes this an ERROR, not a partial parse")
	}
	if !errors.Is(err, ErrRecordLength) {
		t.Fatalf("error %v does not match ErrRecordLength", err)
	}
}

// TestBuildersRefuseAddressesOutsideTheirOwnSpace.
func TestBuildersRefuseAddressesOutsideTheirOwnSpace(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p
			_, hi := p.ChannelRange()
			bad := ChannelAddress{Group: sampleAddress(p).Group, Channel: hi + 1}
			if c, err := p.BuildMemoryRead(bad); err == nil {
				t.Errorf("BuildMemoryRead built %s for a channel past this profile's ceiling", c)
			} else if !c.IsZero() {
				t.Error("BuildMemoryRead returned a non-zero Command alongside its error")
			}
			rec := sampleRecord(t, p, p.BuildRecordLength())
			rec.Address = bad
			if c, err := p.BuildMemorySet(rec); err == nil {
				t.Errorf("BuildMemorySet built %s for a channel past this profile's ceiling", c)
			} else if !c.IsZero() {
				t.Error("BuildMemorySet returned a non-zero Command alongside its error")
			}
		})
	}
}

// mustCommand unwraps a builder result. It PANICS rather than taking a
// *testing.T, because Go cannot spread a two-value call into a function
// that also takes a t — and a builder failing on a fixture this package
// itself constructed is a broken fixture, which is worth a stack trace.
func mustCommand(c Command, err error) Command {
	if err != nil {
		panic("civ test: builder failed: " + err.Error())
	}
	return c
}

// withAddresses returns a copy of frame with its `to` and `from` bytes
// replaced.
func withAddresses(frame []byte, to, from byte) []byte {
	out := copyBytes(frame)
	out[2], out[3] = to, from
	return out
}

// truncateBody removes n bytes from just before the terminator.
func truncateBody(frame []byte, n int) []byte {
	out := copyBytes(frame[:len(frame)-1-n])
	return append(out, EndByte)
}

// TestGateRefusesARecordByteNoBuilderWouldWrite is the re-encode rule's
// own property: a memory set carrying anything in a byte the layout does
// not map — a reserved position, or a Fixed template constant altered — is
// refused, because no builder could have produced it.
//
// A rule that merely validated each mapped field in isolation would admit
// every one of these.
func TestGateRefusesARecordByteNoBuilderWouldWrite(t *testing.T) {
	exercised := 0
	for _, np := range allTestProfiles() {
		p := np.p
		length := p.BuildRecordLength()
		unmapped := unmappedRecordBytes(t, p, length)
		if len(unmapped) == 0 {
			continue
		}
		t.Run(np.name, func(t *testing.T) {
			exercised++
			set := mustCommand(p.BuildMemorySet(sampleRecord(t, p, length))).Bytes()
			if !p.AllowedCommand(set) {
				t.Fatal("the builder's own frame was refused before any mutation")
			}
			recordStart := 6 + p.AddressForm().addressBytes()
			for _, off := range unmapped {
				mutated := copyBytes(set)
				mutated[recordStart+off] ^= 0x01
				if p.AllowedCommand(mutated) {
					t.Errorf("the gate ADMITTED a record whose unmapped byte %d was altered: %s", off, hexFrame(mutated))
				}
			}
		})
	}
	if exercised == 0 {
		t.Fatal("no fixture has an unmapped record byte, so the re-encode rule was never exercised — every profile mapping every byte of its record makes this property untestable, which is why bandProfile has reserved bytes")
	}
}

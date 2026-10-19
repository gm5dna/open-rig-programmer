// SPDX-License-Identifier: GPL-3.0-or-later

// Package icomtest holds the table-driven read and write suites, and the
// scripted radio they drive, shared by the engine siblings' tests. It carries
// no citations: each package supplies its own facts through a Fixture.
package icomtest

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Session is a driver session plus the answer-mismatch counter every sibling
// carries.
type Session interface {
	driver.Session
	AnswerMismatches() uint64
}

// RowOffset names a record byte and the value a row writes into it.
type RowOffset struct {
	Name string
	Off  int
	Val  byte
}

// DifferRow is a prior record whose unmapped region differs, and the
// refusal expected for it.
type DifferRow struct {
	RowOffset
	Offset int
	Nibble string
	Got    byte
}

// Fixture is everything a package supplies to the shared suites.
type Fixture struct {
	Params *icom.Params
	// Load is codeplug.Load, passed in because only test files may name it.
	Load func(string) (*codeplug.Codeplug, error)
	// Addr is the radio's CI-V address.
	Addr byte
	// Golden is the 25-byte record the suites read and write.
	Golden []byte
	// New builds the driver under test.
	New func(realHardware, consented bool) driver.Driver
	// Simulated and Unverified are the two capability profiles.
	Simulated, Unverified func() spec.Capabilities
	// AsDomain and AsUnmapped unwrap the package's own error types, or
	// return nil.
	AsDomain   func(error) *driver.OutOfDomainError
	AsUnmapped func(error) *driver.UnmappedRegionError
	// MaxFreqHz and MaxToneDeciHz are the package's encodable ceilings.
	MaxFreqHz, MaxToneDeciHz uint64
	// DefaultToneEntry is the text a create-without-tone refusal must carry.
	DefaultToneEntry string
	// SmuggleRows are records the read must accept with the unmapped
	// regions Unavailable; DifferRows are those a write must refuse.
	SmuggleRows []RowOffset
	DifferRows  []DifferRow
}

func (fx Fixture) port(t *testing.T, img RadioImage) *ScriptedPort {
	t.Helper()
	return NewPort(t, fx.Addr, img)
}

func (fx Fixture) memReadFrame(ch int) []byte { return MemReadFrame(fx.Addr, ch) }

func (fx Fixture) openWith(t *testing.T, p *ScriptedPort) Session {
	t.Helper()
	sess, err := fx.New(false, false).Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open: %v\ntranscript:\n  %s", err, HexFrames(p.Transcript()))
	}
	s, ok := sess.(Session)
	if !ok {
		t.Fatalf("Open returned %T, want a Session with AnswerMismatches", sess)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func (fx Fixture) occupiedRadio() RadioImage {
	return RadioImage{IDToken: []byte{fx.Addr}, Records: map[int][]byte{1: fx.Golden}}
}

func (fx Fixture) slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	return icom.SlotToAddress(fx.Params.Name, slot)
}

func (fx Fixture) addressToSlot(a civ.ChannelAddress) (string, error) {
	return icom.AddressToSlot(fx.Params.Name, a, "%03d")
}

func (fx Fixture) requestedFields(d codeplug.ChannelData) []spec.Field {
	return icom.RequestedFields(fx.Params, d)
}

func (fx Fixture) outsideVocabulary(d codeplug.ChannelData, caps spec.Capabilities) (spec.Field, string) {
	return icom.OutsideVocabulary(fx.Params, d, caps)
}

func (fx Fixture) unmappedRegionsDiffer(raw []byte) error {
	return icom.UnmappedRegionsDiffer(fx.Params, raw)
}

func (fx Fixture) toneRefusalReason(create bool) string {
	return icom.ToneRefusalReason(fx.Params, create)
}

// RunReadSuite runs every shared read test against fx.
func RunReadSuite(t *testing.T, fx Fixture) {
	for _, c := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"TestReadChannel_RefusesFrequencyAboveCeiling", fx.tReadChannel_RefusesFrequencyAboveCeiling},
		{"TestReadChannel_AcceptsFrequencyAtCeiling", fx.tReadChannel_AcceptsFrequencyAtCeiling},
		{"TestSlotAddressRoundTrip", fx.tSlotAddressRoundTrip},
		{"TestReadChannel_MapsEveryField", fx.tReadChannel_MapsEveryField},
		{"TestReadChannel_EmptySlotIsAnEmptyChannel", fx.tReadChannel_EmptySlotIsAnEmptyChannel},
		{"TestReadChannel_WrongLengthAborts", fx.tReadChannel_WrongLengthAborts},
		{"TestReadChannel_UndecodableEnumIsAnError", fx.tReadChannel_UndecodableEnumIsAnError},
		{"TestReadChannel_OutOfDomainToneReadsAsUnknown", fx.tReadChannel_OutOfDomainToneReadsAsUnknown},
		{"TestReadChannel_AnswerForAnotherChannelIsCaughtBeforeAnyUse", fx.tReadChannel_AnswerForAnotherChannelIsCaughtBeforeAnyUse},
		{"TestReadChannel_UnmappedNibblesAreNotSmuggledThrough", fx.tReadChannel_UnmappedNibblesAreNotSmuggledThrough},
	} {
		t.Run(c.name, c.run)
	}
}

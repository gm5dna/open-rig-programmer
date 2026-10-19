// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom/icomtest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

const radioAddr = 0xB2

type (
	scriptedPort = icomtest.ScriptedPort
	radioImage   = icomtest.RadioImage
)

const (
	memReadFrameLen = icomtest.MemReadFrameLen
	memSetFrameLen  = icomtest.MemSetFrameLen
	idReadFrameLen  = icomtest.IDReadFrameLen
)

var (
	hexFrames     = icomtest.HexFrames
	nakFrame      = icomtest.NakFrame
	ackFrame      = icomtest.AckFrame
	bcdChannel    = icomtest.BCDChannel
	encodeChannel = icomtest.EncodeChannel
	bcd3          = icomtest.BCD3
)

func newScriptedPort(t *testing.T, img radioImage) *scriptedPort {
	t.Helper()
	return icomtest.NewPort(t, radioAddr, img)
}

var fixture = icomtest.Fixture{
	Params: &params,
	Load:   codeplug.Load,
	Addr:   radioAddr,
	Golden: goldenRecord,
	New: func(realHardware, consented bool) driver.Driver {
		var opts []Option
		if consented {
			opts = append(opts, WithConsentedUnverifiedWrites())
		}
		if realHardware {
			return New(RealHardware, opts...)
		}
		return New(Simulated, opts...)
	},
	Simulated:  capabilitiesSimulated,
	Unverified: capabilitiesUnverified,
	AsDomain: func(err error) *driver.OutOfDomainError {
		var e *OutOfDomainError
		if errors.As(err, &e) {
			return &e.OutOfDomainError
		}
		return nil
	},
	AsUnmapped: func(err error) *driver.UnmappedRegionError {
		var e *UnmappedRegionError
		if errors.As(err, &e) {
			return &e.UnmappedRegionError
		}
		return nil
	},
	MaxFreqHz:        MaxEncodableFreqHz,
	MaxToneDeciHz:    MaxToneDeciHz,
	DefaultToneEntry: "ic7760-tone-domain",
	SmuggleRows: []icomtest.RowOffset{
		{Name: "byte 0 = 0x02, SELECT ★2", Off: 0, Val: 0x02},
		{Name: "byte 0 = 0x03, SELECT ★3", Off: 0, Val: 0x03},
		{Name: "byte 8 = 0x21, DATA 2 + TONE", Off: 8, Val: 0x21},
		{Name: "byte 8 = 0x31, DATA 3 + TONE", Off: 8, Val: 0x31},
	},
	DifferRows: []icomtest.DifferRow{
		{RowOffset: icomtest.RowOffset{Name: "byte 0 low = 2, SELECT ★2", Off: 0, Val: 0x02}, Offset: 0, Nibble: "low", Got: 0x02},
		{RowOffset: icomtest.RowOffset{Name: "byte 0 low = 1, SELECT ★1", Off: 0, Val: 0x01}, Offset: 0, Nibble: "low", Got: 0x01},
		{RowOffset: icomtest.RowOffset{Name: "byte 0 high = 1, not the printed Fixed 0", Off: 0, Val: 0x10}, Offset: 0, Nibble: "high", Got: 0x01},
		{RowOffset: icomtest.RowOffset{Name: "byte 8 high = 2, DATA 2", Off: 8, Val: 0x21}, Offset: 8, Nibble: "high", Got: 0x02},
		{RowOffset: icomtest.RowOffset{Name: "byte 8 high = 1, DATA 1", Off: 8, Val: 0x11}, Offset: 8, Nibble: "high", Got: 0x01},
	},
}

func TestSharedReadSuite(t *testing.T) { icomtest.RunReadSuite(t, fixture) }

const probeSlotCount = 10

var containsField = icomtest.ContainsField

func goodChannel(slot string) codeplug.Channel { return fixture.GoodChannel(slot) }

func slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	return icom.SlotToAddress(params.Name, slot)
}

func TestSharedWriteSuite(t *testing.T) { icomtest.RunWriteSuite(t, fixture) }

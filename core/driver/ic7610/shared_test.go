// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	"errors"
	"testing"

	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom/icomtest"
)

const radioAddr = 0x98

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
	DefaultToneEntry: "ic7610-default-tone-undocumented",
	SmuggleRows: []icomtest.RowOffset{
		{Name: "byte 0 = 0x02, SELECT ★2", Off: 0, Val: 0x02},
		{Name: "byte 0 = 0x03, SELECT ★3", Off: 0, Val: 0x03},
		{Name: "byte 8 = 0x21, DATA 2 + TONE", Off: 8, Val: 0x21},
		{Name: "byte 8 = 0x31, DATA 3 + TONE", Off: 8, Val: 0x31},
	},
	DifferRows: []icomtest.DifferRow{
		{RowOffset: icomtest.RowOffset{Name: "byte 0 low = 2, SELECT ★2", Off: 0, Val: 0x02}, Offset: civic7610.SelectNibbleOffset, Nibble: "low", Got: 0x02},
		{RowOffset: icomtest.RowOffset{Name: "byte 0 low = 1, SELECT ★1", Off: 0, Val: 0x01}, Offset: civic7610.SelectNibbleOffset, Nibble: "low", Got: 0x01},
		{RowOffset: icomtest.RowOffset{Name: "byte 0 high = 1, not the printed Fixed 0", Off: 0, Val: 0x10}, Offset: civic7610.SelectNibbleOffset, Nibble: "high", Got: 0x01},
		{RowOffset: icomtest.RowOffset{Name: "byte 8 high = 2, DATA 2", Off: 8, Val: 0x21}, Offset: civic7610.DataModeNibbleOffset, Nibble: "high", Got: 0x02},
		{RowOffset: icomtest.RowOffset{Name: "byte 8 high = 1, DATA 1", Off: 8, Val: 0x11}, Offset: civic7610.DataModeNibbleOffset, Nibble: "high", Got: 0x01},
	},
}

func TestSharedReadSuite(t *testing.T) { icomtest.RunReadSuite(t, fixture) }

func withRecord(offset int, b ...byte) []byte {
	rec := append([]byte(nil), goldenRecord...)
	copy(rec[offset:], b)
	return rec
}

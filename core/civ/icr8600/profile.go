// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"sort"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Stable model data shared with the eventual driver. These constants are
// kept beside the CI-V profile so the wire and neutral capability layers do
// not acquire two independent copies of the receiver's identity and memory
// geometry.
const (
	Model             = "IC-R8600"
	RadioAddress byte = 0x96

	// ControllerAddress is civ's zero/default controller address. The
	// ProfileConfig below deliberately leaves ControllerAddress zero so
	// NewProfile's documented defaulting semantics remain exercised.
	ControllerAddress byte = civ.ControllerAddressDefault

	// MaxFrame admits the 55-byte full G witness (and the frozen set's
	// neighbouring totals) without treating that evidence as a record-shape
	// declaration. TestGoldenVectorsReplay pins the exact accounting.
	MaxFrame = 64

	MemoryGroups           = 100
	MemoryGroupBase        = 0
	MemoryChannelsPerGroup = 100
	MemoryChannelBase      = 0

	NameLength = 16
	// NameCharset is the printed set mapped to printable ASCII 0x20..0x7e.
	// THE CODES ARE ASSUMED: register icr8600-name-charset-codes. Stage R
	// lifts the assumption by writing and reading representatives from every
	// printed character class. In particular, ';' and '|' are both members.
	NameCharset = " !\"#$%&'()*+,-./0123456789:;<=>?@" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`" +
		"abcdefghijklmnopqrstuvwxyz{|}~"
	// NamePad is ASSUMED ASCII space: register icr8600-name-pad. Stage R
	// lifts it by reading a short front-panel name and recording every byte
	// in the field's trailing tail.
	NamePad byte = 0x20

	// DigitalTailRefusalReason is shared with the eventual driver so E6
	// names the state the operator must edit consistently. The assumed bytes
	// belong to icr8600-tail-templates (plan alias
	// icr8600-digital-tail-template), pinned by
	// TestTailTemplatesAndFMToneFieldsEncodeEveryDeclaredClass.
	DigitalTailRefusalReason = "D-STAR/P25/NXDN/DCR/dPMR digital-squelch bytes differ from the assumed template — set them at the radio"
)

// MemoryBank returns the IC-R8600's one currently declared neutral memory
// bank. A fresh value is returned on every call so a later driver may attach
// materialised Slots and field-support labels without mutating package state.
//
// The populated-channel capacity is unresolved (register icr8600-budget;
// Stage R lift: fill ordinary memories until the receiver refuses another),
// so BudgetUnstated is the positive declaration and Budget remains zero.
// Groups 0100 and 0101 are deliberately absent, and 0102 remains absent while
// icr8600-scan-edge-encoding awaits its Stage R scan-edge-read capture.
func MemoryBank() spec.Bank {
	return spec.Bank{
		ID:             spec.BankMemory,
		Sparse:         true,
		Groups:         MemoryGroups,
		GroupBase:      MemoryGroupBase,
		PerGroup:       MemoryChannelsPerGroup,
		ChannelBase:    MemoryChannelBase,
		BudgetUnstated: true,
	}
}

// Profile returns the IC-R8600's CI-V dialect.
func Profile() civ.Profile { return profile }

var profile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        Model,
	RadioAddress: RadioAddress,
	// ControllerAddress is deliberately zero: civ.NewProfile resolves it
	// to civ.ControllerAddressDefault (0xE0).
	MaxFrame: MaxFrame,

	AddressForm: civ.AddressFormWideGroupChannel,
	Groups:      MemoryGroups,
	GroupBase:   MemoryGroupBase,
	ChannelLo:   MemoryChannelBase,
	ChannelHi:   MemoryChannelBase + MemoryChannelsPerGroup - 1,
	// ExtraRanges is deliberately empty. Normal MEM is exactly groups
	// 0000..0099 by channels 00..99; 0100/0101 are other families and 0102
	// remains unresolved under icr8600-scan-edge-encoding.

	NameLength:  NameLength,
	NameCharset: NameCharset,
	NamePad:     NamePad,

	Layouts:       recordLayouts(),
	Discriminator: civ.DiscriminatorModeByte,
	ModeKey: civ.FieldSpan{
		Field: civ.FieldMode, Offset: 6, Length: 1,
		Encoding: civ.EncodingEnum,
	},
})

func recordLayouts() []civ.RecordLayout {
	return []civ.RecordLayout{
		recordLayout("NONE", 37, map[byte]string{
			0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW",
			0x04: "FSK", 0x06: "WFM", 0x07: "CW-R", 0x08: "FSK-R",
			0x11: "S-AM (D)", 0x14: "S-AM (L)", 0x15: "S-AM (U)",
		}),
		recordLayout("D-STAR", 39, map[byte]string{0x17: "D-STAR"}),
		recordLayout("P25", 41, map[byte]string{0x16: "P25"}),
		recordLayout("NXDN", 43, map[byte]string{0x19: "NXDN-VN", 0x20: "NXDN-N"}),
		recordLayout("FM", 44, map[byte]string{0x05: "FM"}),
		recordLayout("DCR", 44, map[byte]string{0x21: "DCR"}),
		recordLayout("dPMR", 45, map[byte]string{0x18: "dPMR"}),
	}
}

func recordLayout(class string, length int, modes map[byte]string) civ.RecordLayout {
	modeValues := make([]byte, 0, len(modes))
	for value := range modes {
		modeValues = append(modeValues, value)
	}
	sort.Slice(modeValues, func(i, j int) bool { return modeValues[i] < modeValues[j] })
	fields := commonFields(modes)
	if class == "FM" {
		fields = append(fields, fmTailFields()...)
	}
	return civ.RecordLayout{
		Length:     length,
		ModeClass:  class,
		ModeValues: modeValues,
		Fields:     fields,
		Fixed:      digitalTailTemplate(class, length),
	}
}

func commonFields(modes map[byte]string) []civ.FieldSpan {
	return []civ.FieldSpan{
		enumSpan(civ.FieldSelect, 0, civ.NibbleLow, map[byte]string{
			0x00: "OFF", 0x01: "SEL1", 0x02: "SEL2", 0x03: "SEL3",
			0x04: "SEL4", 0x05: "SEL5", 0x06: "SEL6", 0x07: "SEL7",
			0x08: "SEL8", 0x09: "SEL9",
		}),
		bcdSpan(civ.FieldRXFrequency, 1, 5, civ.OrderLittleEndian, 1),
		enumSpan(civ.FieldMode, 6, civ.NibbleWhole, modes),
		enumSpan(civ.FieldFilter, 7, civ.NibbleWhole, map[byte]string{
			0x01: "FIL1", 0x02: "FIL2", 0x03: "FIL3",
		}),
		enumSpan(civ.FieldDuplex, 8, civ.NibbleLow, map[byte]string{
			0x00: "OFF", 0x01: "DUP-", 0x02: "DUP+",
		}),
		bcdSpan(civ.FieldOffset, 9, 4, civ.OrderLittleEndian, 100),
		enumSpan(civ.FieldTuningStepEnabled, 13, civ.NibbleWhole, map[byte]string{
			0x00: "OFF", 0x01: "ON",
		}),
		enumSpan(civ.FieldTuningStep, 14, civ.NibbleWhole, map[byte]string{
			0x01: "100 Hz", 0x02: "1 kHz", 0x03: "2.5 kHz",
			0x04: "3.125 kHz", 0x05: "5 kHz", 0x06: "6.25 kHz",
			0x07: "8.33 kHz", 0x08: "9 kHz", 0x09: "10 kHz",
			0x10: "12.5 kHz", 0x11: "20 kHz", 0x12: "25 kHz",
			0x13: "100 kHz", 0x14: "programmable tuning step",
		}),
		// B and PDF p.12 agree that the digit weights are 1 kHz,
		// 100 Hz, 100 kHz, 10 kHz. Little-endian BCD with a 100 Hz
		// scale preserves that deliberately non-monotonic byte order.
		bcdSpan(civ.FieldProgramTuningStep, 15, 2, civ.OrderLittleEndian, 100),
		bcdSpan(civ.FieldAttenuator, 17, 1, civ.OrderBigEndian, 1),
		enumSpan(civ.FieldPreamp, 18, civ.NibbleLow, map[byte]string{
			0x00: "OFF", 0x01: "ON",
		}),
		enumSpan(civ.FieldAntenna, 19, civ.NibbleLow, map[byte]string{
			0x00: "ANT1", 0x01: "ANT2", 0x02: "ANT3",
		}),
		enumSpan(civ.FieldIPPlus, 20, civ.NibbleLow, map[byte]string{
			0x00: "OFF", 0x01: "ON",
		}),
		{Field: civ.FieldName, Offset: 21, Length: NameLength, Encoding: civ.EncodingName},
	}
}

func enumSpan(field civ.FieldID, offset int, nibble civ.NibbleSel, values map[byte]string) civ.FieldSpan {
	return civ.FieldSpan{Field: field, Offset: offset, Length: 1, Nibble: nibble, Encoding: civ.EncodingEnum, Enum: values}
}

func bcdSpan(field civ.FieldID, offset, length int, order civ.ByteOrder, scale uint64) civ.FieldSpan {
	return civ.FieldSpan{Field: field, Offset: offset, Length: length, Encoding: civ.EncodingBCDNumber, Order: order, Scale: scale}
}

func fmTailFields() []civ.FieldSpan {
	return []civ.FieldSpan{
		enumSpan(civ.FieldToneMode, 37, civ.NibbleLow, map[byte]string{
			0x00: "OFF", 0x01: "TSQL", 0x02: "DTCS",
		}),
		bcdSpan(civ.FieldToneRX, 38, 3, civ.OrderBigEndian, 1),
		// The first DTCS byte carries only receive polarity. The code's
		// three printed digits occupy the following two bytes.
		enumSpan(civ.FieldDTCSPolarity, 41, civ.NibbleLow, map[byte]string{
			0x00: "Normal", 0x01: "Reverse",
		}),
		bcdSpan(civ.FieldDTCSCode, 42, 2, civ.OrderBigEndian, 1),
	}
}

func digitalTailTemplate(class string, length int) []byte {
	var tail []byte
	switch class {
	case "D-STAR":
		tail = []byte{0x02, 0x12}
	case "P25":
		tail = []byte{0x01, 0x02, 0x09, 0x03}
	case "NXDN":
		tail = []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00}
	case "DCR":
		tail = []byte{0x01, 0x01, 0x23, 0x00, 0x00, 0x00, 0x00}
	case "dPMR":
		tail = []byte{0x01, 0x01, 0x23, 0x12, 0x00, 0x00, 0x00, 0x00}
	default:
		return nil
	}
	// ASSUMED: matrix register icr8600-tail-templates, called
	// icr8600-digital-tail-template by the plan. G chose these writable
	// states; the guide prints no defaults. Stage R lifts the assumption
	// by reading a factory-fresh channel of every digital class. Leaving
	// them unmapped makes the gate enforce E6 by byte identity.
	fixed := make([]byte, length)
	copy(fixed[37:], tail)
	return fixed
}

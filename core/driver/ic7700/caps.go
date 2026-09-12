// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is this model's hardware write guard, and it is
// FALSE.
//
// FALSE BECAUSE NO IC-7700 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
// (matrix §0, §3.14). Every byte in this package's tables came from the
// IC-7700 Instruction Manual (Revision 7, 232 pages, gitignored) through
// the reviewed capability matrix (docs/superpowers/icom-matrices/
// ic7700-capability-matrix.md) and from nothing else — there is no
// standalone CI-V reference guide for this model; CI-V is Section 14 of
// the full manual.
//
// FLIPPING IT IS A TWO-PART CHANGE, stated here so a one-line edit cannot
// unlock a write:
//
//  1. this constant, AND
//  2. a capabilitiesRealHardware profile built field by field from an
//     IC-7700's OWN trial evidence — never from this model's manual, and
//     never from another model's capture, AND
//  3. the Capabilities switch in ic7700.go rewritten to select it.
const writeTrialsComplete = false

// Profile selects the static capability arm used by the driver.
//
// The zero value is RealHardware ON PURPOSE: a forgotten or zero-valued
// Profile must fail TOWARDS the real-hardware capability set — which,
// while the write-trial guard is false, is the all-Unverified one, nothing
// writable — and NEVER towards the simulator's, whose Supported writes are
// a claim about the in-package scripted port and about nothing else.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// The numeric bounds this driver DECLARES and, at WriteChannel's mandatory-
// field rung, ENFORCES.
//
// THE RADIO'S NUMBERS, NOT THE FIELD'S CAPACITY. The record's frequency
// digits can express up to 69 999 999 Hz (matrix §1 rows 15-16: the 10 MHz
// digit is printed 0-6 over a fixed "0 : 0" fifth cell) and the tone
// digits up to 254.9 Hz (matrix §1 row 12), but this radio's own STORABLE
// bounds are not printed anywhere and stay ASSUMED (register entries
// ic7700-storable-frequency-floor/-ceiling). Declaring the wire's
// ENCODABLE bounds — rather than inventing a narrower figure this
// document does not state — is the CHOICE: it authorises exactly what the
// record can carry and nothing this document claims about the radio's
// actual coverage.
const (
	// MinFreqHz/MaxFreqHz are the RX/TX frequency spans' own encoding
	// bounds. Matrix §1 rows 15-16.
	MinFreqHz = 0
	MaxFreqHz = 69_999_999
	// MinToneDeciHz/MaxToneDeciHz are the tone spans' own BCD capacity, in
	// tenths of a hertz: matrix §1 rows 11-12, PDF p.211 (folio 14-11),
	// whose rotated leaders print "100Hz digit: 0-2" and "10/1/0.1 Hz
	// digit: 0-9" each, giving 000.0-254.9 Hz. The floor is 1, not the
	// printed 0, because 0 Hz is not a tone and spec.ToneRange requires
	// MinDeciHz > 0.
	MinToneDeciHz  = 1
	MaxToneDeciHz  = 2549
	ToneStepDeciHz = 1
)

// deliberatelyZero is this driver's audit table: every spec.Capabilities
// field left at its zero value, with the reason. TestDeliberatelyZeroAudit
// reflects over the struct and requires each field to be either populated
// (cited in baseCapabilities) or listed here — the two sets must partition
// the struct exactly.
var deliberatelyZero = map[string]string{
	"ClarMaxHz":              "the 1A 00 record has no clarifier field: RIT is command 21 00, live-VFO only, outside the record (matrix §1 row 9)",
	"ClarStepHz":             "the same: no clarifier field, no step (matrix §1 row 10)",
	"CTCSSTones":             "this radio's tone spans are BCD FREQUENCIES, not indices into a chart. CTCSSToneRange is the declaration (matrix §1 row 11)",
	"RequiredSlots":          "no channel is documented as one that must stay populated (matrix §1 row 17); the scan edges' non-clearable rule is a WHOLE-BANK fact this driver does not carry as Bank.NoBlank either — see the SCAN bank's own NoBlank comment",
	"ShiftOptions":           "the Yaesu repeater-shift vocabulary. This radio's own vocabulary is the Split ON/OFF boolean plus an independent TX block, not a shift direction (matrix §1 row 18)",
	"CTCSSStates":            "the Yaesu tone-state vocabulary. This radio expresses tone through ToneModes, the Icom one (matrix §1 row 19)",
	"DuplexOptions":          "the record has no fixed-offset repeater-duplex field: what it has instead is Split ON/OFF plus tx_frequency (matrix §1 row 20)",
	"TuningSteps":            "additions design D8 — a transceiver record, no receiver tuning-step field (matrix §1 row 25)",
	"ProgramTuningStepRange": "additions design D8 — no programmable tuning-step field (matrix §1 row 26)",
	"AttenuatorDB":           "additions design D8 — no per-channel attenuator field; command 11 is a live front-panel set item, not a record field (matrix §1 row 27)",
	"PreampOptions":          "additions design D8 — no preamp field (matrix §1 row 28)",
	"AntennaOptions":         "additions design D8 — no per-channel antenna field; command 12 is a live set item, not a record field (matrix §1 row 29)",
	"DTCSPolarities":         "the strings \"DTCS\" and \"DCS\" occur nowhere in the swept command table and data-description sections; the tone-type vocabulary is the 3-valued OFF/TONE/TSQL, not 4-valued (matrix §1 row 22)",
	"DTCSCodes":              "the same sweep: no DTCS code table is printed anywhere (matrix §1 row 23)",
	"NoTag":                  "the IC-7700 supports channel names via the ten-byte name field idx29-38; NoTag is false (matrix §1 rows 7-8)",
	"MinFreqHz":              "the RX/TX frequency spans' own encoding floor is 0 Hz, which is also Go's uint64 zero value — a populated field that happens to equal its zero (matrix §1 row 15)",
	"TagCharset":             "empty is this driver's CHOICE (matrix §1 row 30): the document's own statement is broader than any table this driver can transcribe with confidence, so TagByteOK's default rule (printable ASCII 0x20-0x7E, excluding ';') is used instead of a narrower or wider set this document does not itself enumerate",
}

// memSlots is the MEM bank's inventory: "001".."099".
//
// Matrix §1 row 5 / §1b — the two-byte channel selector runs 1..99 for
// the memories (BCD "00 01".."00 99").
func memSlots() []string { return spec.NumberedSlots(1, 99, "%03d") }

// scanSlots is the SCAN bank's inventory: the two scan edges P1, P2.
//
// Matrix §1 row 5 / §1b — P1 (channel 100, wire "01 00") and P2 (channel
// 101, wire "01 01") are two more values of the SAME two-byte selector,
// not a protocol-level bank; this project models them as a bank only
// because the neutral memory model needs the memory/scan-edge distinction.
func scanSlots() []string { return []string{"P1", "P2"} }

// bankFields returns one bank's field map, with rw applied to every field
// this record maps and the zero FieldSupport everywhere else.
//
// IDENTICAL FOR MEM AND SCAN (matrix §2): both banks read and write the
// SAME 1A 00 record at different selector values. Whether every field is
// HONOURED on a scan edge is a separate, ASSUMED question (matrix §2
// SCAN-bank header, register entry ic7700-scan-edge-record-fields) that
// this grading does not answer either way.
//
// EVERY spec.Field IS LISTED, including the ones this radio does not have:
// an absent key and an explicit zero FieldSupport mean the same thing to
// spec.Capabilities.FieldSupport, but only one is a decision a reader can
// check. Each call returns a fresh map, so no two banks share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The record maps these directly — core/civ/ic7700's layout()
		// carries a FieldSpan for each. idx1-5 (rx freq), idx6 (mode),
		// idx7 (filter), idx8 low nibble (tone_mode), idx9-11 (tone_tx),
		// idx12-14 (tone_rx), idx29-38 (name). Matrix §1b/§3.15.
		spec.FieldFrequency: rw,
		spec.FieldMode:      rw,
		spec.FieldFilter:    rw,
		spec.FieldToneMode:  rw,
		spec.FieldToneTx:    rw,
		spec.FieldToneRx:    rw,
		spec.FieldTag:       rw,

		// tx_frequency — idx15-19, the TX-duplicate block's OWN frequency
		// span, a distinct civ.FieldTXFrequency FieldSpan (matrix §1b, §2
		// row 11, coordinator ruling 12/09/2026). MANUAL-EVIDENCED that
		// this span exists and is always present on the wire; what it
		// CARRIES on a Split-OFF (simplex) channel is not printed (matrix
		// §1 row 4). This driver's own write policy settles that for
		// itself — see write.go's mirror-on-simplex behaviour and
		// Capabilities().SimplexTx below — which is what makes Sup/Sup
		// the honest grade here rather than a manual fact.
		spec.FieldTxFrequency: rw,

		// RULING E6 — THE UNMAPPED SPLIT/SELECT BYTE. idx0 (`e`) carries
		// the split ON/OFF flag in its HIGH nibble and a four-valued
		// select-scan GROUP marker in its LOW nibble (matrix §3.15(1)).
		// Neither has a faithful neutral home: codeplug.ChannelData's
		// ScanSkip is a BoolField, and the split flag itself gates
		// nothing this tier's write contract reads (tx_frequency is
		// written unconditionally regardless of the flag). A 4->2
		// collapse of the select marker would rewrite a user's group on
		// every write-back while readback verification compared equal —
		// the silent corruption this project refuses. So the WHOLE byte
		// is UNMAPPED in the civ layout (matches the IC-7610 precedent
		// for its analogous byte) and carries the zero FieldSupport here.
		//
		// THIS GRADE DIFFERS FROM THE MATRIX, which reads §2 MEM row 9 as
		// Sup/Sup for scan_skip on wire-capability grounds alone; ic7610,
		// ic7300 and ic7851 all record the identical divergence for their
		// own analogous byte and this driver follows the same tier-wide
		// precedent (ruling E6 overrides the matrix's optimistic grade).
		spec.FieldScanSkip: {},
		// idx8 HIGH nibble — the four-valued data mode (matrix §3.15.1),
		// UNMAPPED for the identical BoolField-collapse reason.
		spec.FieldDataMode: {},

		// MANUAL-EVIDENCED ABSENCE: the 1A 00 record has no such span.
		spec.FieldClarifier:  {}, // matrix §1 row 9
		spec.FieldShift:      {}, // matrix §1 row 18
		spec.FieldDuplex:     {}, // matrix §1 row 20
		spec.FieldOffset:     {}, // matrix §1 row 20
		spec.FieldTagDisplay: {}, // matrix §2 MEM row 8 — one name field only,
		// no per-channel display toggle
		spec.FieldCTCSSState:   {}, // superseded by tone_mode (matrix §1 row 19)
		spec.FieldCTCSSTone:    {}, // superseded by tone_tx/tone_rx (matrix §1 row 11)
		spec.FieldDTCSCode:     {}, // matrix §1 row 23
		spec.FieldDTCSPolarity: {}, // matrix §1 row 22

		// ERASE. The wire form exists (1A 00 <ch> FF, PDF p.213; whole
		// command 0B, PDF p.203) but no IC-7700 has ever been asked to
		// use either, and the scan edges are separately documented as
		// non-clearable (matrix §1b, §2 SCAN row 10, §3.13, PDF p.136's
		// CLEAR column reading "No" for P1/P2). The zero FieldSupport
		// keeps core/clone/execute.go's DiffErased branch unreachable for
		// this model on both banks.
		spec.FieldErase: {},
	}
}

// baseCapabilities assembles the static baseline both profiles share, with
// the given per-bank field maps.
func baseCapabilities(memFields, scanFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		// Matrix §1 row 1. PDF metadata title "IC-7700 INSTRUCTION MANUAL".
		Model: "IC-7700",
		// Matrix §1 row 2 / §3.4 — CI-V address 74h, MANUAL-EVIDENCED: PDF
		// p.180 (folio 12-18), "CI-V Address": "The IC-7700's address is
		// 74h." The 19 00 reply token is a per-session observation this
		// driver RECORDS and NEVER MATCHES (register entry
		// ic7700-id-token); this static field carries the address alone
		// (spec D3.2).
		CATID:    "74",
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: "Memories",
				Slots: memSlots(),
				// Matrix §1b Bank descriptors, "Bank.NoBlank — MEM": false,
				// a CHOICE (conservative) — the document does not state
				// what an unwritten channel answers (register entry
				// ic7700-empty-channel-reply).
				NoBlank: false,
				Fields:  memFields,
			},
			{
				ID:    spec.BankScan,
				Label: "Scan edges",
				Slots: scanSlots(),
				// Matrix §1b Bank descriptors, "Bank.NoBlank — SCAN":
				// false, also a CHOICE despite the genuine data point
				// that scan edges cannot be CLEARED (PDF p.136's CLEAR
				// column reads "No" for P1/P2): "cannot be cleared" is
				// not the same claim as "is never blank from the
				// factory", so the matrix does not lift this to
				// MANUAL-EVIDENCED true and neither does this driver.
				NoBlank: false,
				Fields:  scanFields,
			},
		},
		// Matrix §1 row 6 — PDF p.210 (folio 14-10), "Operating mode",
		// Command 01/04/06: 00 LSB, 01 USB, 02 AM, 03 CW, 04 RTTY, 05 FM,
		// 07 CW-R, 08 RTTY-R, 12 PSK, 13 PSK-R. Codes 06 and 09-11 are
		// printed nowhere and are deliberately absent: a record carrying
		// one fails to decode with a parse error naming the offset
		// (register entry ic7700-mode-code-completeness).
		Modes: []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "PSK", "PSK-R"},
		// Matrix §1 row 24 — PDF p.210, filter column: 01 FIL1, 02 FIL2,
		// 03 FIL3. 0x00 is not a member: a record whose filter byte is
		// 0x00 fails to decode rather than reading as "no filter".
		Filters: []string{"FIL1", "FIL2", "FIL3"},
		// Matrix §1 row 21 — PDF p.213 (folio 14-13), the `!1` sub-diagram
		// upper leader: 0 OFF, 1 TONE, 2 TSQL.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		},
		// Matrix §1 rows 11-12. See MinToneDeciHz/MaxToneDeciHz.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: MinToneDeciHz, MaxDeciHz: MaxToneDeciHz, StepDeciHz: ToneStepDeciHz},
		// Matrix §1 row 13 — MANUAL-EVIDENCED, and SHARPLY NARROWER than
		// every other 7610-family model in this tree: PDF p.179 (folio
		// 12-17), "CI-V Baud Rate": "300, 1200, 4800, 9600, 19200 bps and
		// 'Auto' are available." No rate above 19200 exists on this
		// model.
		Bauds: []int{300, 1200, 4800, 9600, 19200},
		// Matrix §1 row 14 / §3.3 / §3.16 ADDED-1 — the factory default is
		// "Auto" (PDF p.179: "(default: Auto)"), an auto-bauding mode
		// with NO representation in `DefaultBaud int`. DefaultBaud=19200
		// is therefore a CHOICE, not a reading: 19200 is this radio's own
		// documented ceiling among the five listed rates (there is no
		// rate above it to prefer, unlike the other family members' 6-way
		// lists topping out at 115200), so it is the fastest rate this
		// model's own manual actually names. What makes an arbitrary
		// choice safe is the failure mode, not the choice: Open's probe
		// requires an address-matched 19 00 reply, and silence is
		// silence, so a wrong guess costs a clean timeout and never a
		// wrong byte (spec D3.3). Register entry ic7700-default-baud.
		DefaultBaud: 19200,
		MinFreqHz:   MinFreqHz,
		MaxFreqHz:   MaxFreqHz,
		// Matrix §1 rows 7-8 — PDF p.213, "Memory name setting": "Up to
		// 10 characters."
		TagLen: 10,
		// Matrix §1 row 30 — CHOICE. The document's own statement ("All
		// characters are available") is read as broader than any table
		// this driver can transcribe with confidence (no digit/space row
		// is printed the way IC-7610's document supplies one), and
		// leaving TagCharset empty defers to spec.Capabilities.TagByteOK's
		// own default rule (printable ASCII 0x20-0x7E, excluding ';')
		// rather than asserting a narrower or wider set this document does
		// not itself enumerate. Register entry ic7700-tagcharset-space.
		TagCharset: "",
		// SimplexTx — matrix §1 row 4 grades this ASSUMED: the manual
		// states the TX-duplicate block is "still necessary" even with
		// Split OFF, but does not say what it HOLDS then. This driver's
		// own write policy (write.go) answers that for itself: a
		// Split-OFF (TxFreqHz not Known) write MIRRORS the receive
		// frequency into the TX-duplicate span, because the manual gives
		// no other value to write and inventing one from nothing would be
		// worse than "transmits where it receives". SimplexTxEqualsRx
		// states that write-time policy, exactly as the read path (which
		// always decodes idx15-19 into TxFreqHz) reports it back.
		SimplexTx: spec.SimplexTxEqualsRx,
	}
}

// capabilitiesUnverified is the REAL-HARDWARE profile while the
// write-trial guard is false: every mapped field Read Unverified and Write
// Unverified, so NOTHING is writable without recorded consent.
func capabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

// capabilitiesSimulated is the in-package scripted-port-backed profile
// (CLI --fake, GUI demo) and NEVER a real radio: Read AND Write Supported
// for every field the 1A 00 record maps, on MEM and SCAN alike.
//
// EVERY WRITTEN-DOWN ZERO STAYS ZERO, simulator or not — including
// scan_skip and data_mode. E6's unmapped regions are unmapped against the
// simulator too: no amount of cooperative scripting on the other end of
// the wire changes what the record has room for.
func capabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

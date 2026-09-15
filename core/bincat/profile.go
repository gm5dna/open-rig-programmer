// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import "github.com/gm5dna/open-rig-programmer/core/civ"

// The opcodes this milestone's write choreography and read model use —
// shared byte-for-byte between FT-890 and FT-900 (spec.md §Frame grammar:
// "opcode-for-opcode" identical) — so they are package constants rather
// than per-Profile data, exactly as CI-V's CmdMemory is a constant and not
// a Profile field.
//
// THE LIST STOPS HERE, DELIBERATELY (plan.md's Phase 1 v2 redesign,
// Codex #3). No Recall (0x02): every driver this milestone builds reads
// via Status Update directly and declines MemorySelector, so Recall is
// unused surface. No SPLIT, LOCK, M>VFO, UP/DOWN, Pacing, PTT, TUNER,
// START, A=B, Memory Scan Skip, Step Op Freq, Read Meter, Display
// Brightness or Read Flags — none of the write choreography or read model
// this milestone implements sends any of them, and a gate is only as safe
// as the set of opcodes it can name.
const (
	// OpStore is "VFO→M" (FT-890/900): commit whatever is currently
	// active on VFO-A into the named channel. Args: [CH, P2, —, —].
	OpStore byte = 0x03
	// OpABSelect selects VFO-A (V=0) or VFO-B (V=1) as the active VFO.
	// Args: [V, —, —, —].
	OpABSelect byte = 0x05
	// OpClarifier sets the RX/TX clarifier. Args: [C1, C2, C3, C4]. The
	// FT-890/900 manuals' own printed byte layout for this opcode is not
	// legibly recoverable from the evidence this package was built
	// against, so AllowedCommand re-validates only the opcode and the
	// frame's grammar for it, not a per-argument domain — see
	// AllowedCommand's own doc comment.
	OpClarifier byte = 0x09
	// OpSetFreq sets VFO-A's operating frequency: four packed-BCD bytes,
	// least significant decimal pair first (bcd.go), then this opcode.
	OpSetFreq byte = 0x0A
	// OpSetMode sets VFO-A's mode. Args: [M, —, —, —].
	OpSetMode byte = 0x0C
	// OpStatusUpdate reads back 1, 18, 19 or a model's own full-dump
	// length of bytes, selected by U (arg 1) and, for U=UMemoryRecord,
	// CH (arg 4). Args: [U, —, —, CH].
	OpStatusUpdate byte = 0x10
	// OpShift sets the repeater shift direction: SIMPLEX (0), Minus (1)
	// or Plus (2), confirmed identical across all four radios this
	// family covers (spec.md §Write model step 5). Args: [R, —, —, —].
	OpShift byte = 0x84
	// OpTone sets the CTCSS tone code, 0x00-0x20. Args: [CC, —, —, —].
	OpTone byte = 0x90
	// OpOffset sets the repeater shift magnitude — meaningful only when
	// OpShift last sent Minus or Plus. Args: [0x00, S2, S3, S4]; the
	// manual states the first byte must be zero. FT-890/900's own layout
	// (Profile.OffsetZeroArg/OffsetBoundArg default to this one); see
	// those fields' doc comments for FT-1000MP's differently-shaped
	// F9H frame under the same opcode.
	OpOffset byte = 0xF9
	// OpIdentity is "Read Internal Status Flags" (FAH) — FT-1000MP's own
	// identity discriminator (spec.md §Identity probe; ft1000mp
	// capability matrix §1.7), unused by FT-890/900 (whose own
	// discriminator is the full-dump length probe, spec.md's Amendment
	// 14/09/2026 — they have no Model-ID reply at all). Args: [—, —, —,
	// F], where F selects the reply shape — see IdentityShort/
	// IdentityLong.
	OpIdentity byte = 0xFA
)

// IdentityShort and IdentityLong are OpIdentity's F argument (arg index
// 3) values: IdentityShort's 5-byte reply is 3 Status Flag bytes
// followed by the 2 Model-ID bytes (`03H`,`93H` for FT-1000MP/Mark-V);
// IdentityLong's 6-byte reply is six Status Flag bytes and carries no
// ID at all (ft1000mpmarkv_manual layout:5210-5220, printed p.96-97,
// "Read Internal Status Flags" row of Opcode Command Chart (4)). Only
// IdentityShort is useful as an identity probe.
const (
	IdentityShort byte = 0x00
	IdentityLong  byte = 0x01
)

// U values for OpStatusUpdate — how much of the radio's RAM table to
// return (the FT-890/900 manuals' own "Status Update Data Selection"
// table).
const (
	UFullDump      byte = 0 // the whole RAM table — Profile.FullDumpLen bytes
	UMemoryNumber  byte = 1 // 1 byte: current memory number
	UOperatingData byte = 2 // 19 bytes: current operating data record
	UBothVFOs      byte = 3 // 18 bytes: VFO-A then VFO-B, 9 bytes each
	UMemoryRecord  byte = 4 // Profile.RecordLen bytes: the named CH's record
)

// vfoBothLen is UBothVFOs' fixed reply length: two 9-byte VFO records.
const vfoBothLen = 18

// This family's 5-value mode legend (FT-890/900's memory-record Mode
// byte, offset 6 of the 9-byte VFO/Memory Data Record). FT-920 and
// FT-1000MP's own 12-value legend belongs to their own Profile, not
// shipped this milestone.
const (
	ModeLSB byte = 0
	ModeUSB byte = 1
	ModeCW  byte = 2
	ModeAM  byte = 3
	ModeFM  byte = 4
)

// Profile is this family's Dialect/Profile analogue: the opcodes above are
// package constants, unchanging across every radio this family registers,
// and Profile carries only what genuinely VARIES per radio — slot space,
// record geometry, full-dump length, mode legend and tone-byte presence.
//
// ONE GO TYPE, MANY VALUES (ponytail — no per-radio type). FT-890 and
// FT-900 need two distinct VALUES because their slot count and full-dump
// length genuinely differ (32/649 vs 100/1941), not because their shape
// differs. Both values are built and owned by core/driver/ft890900
// (a later phase), not by this package — mirroring core/civ's Profile
// type being generic while each model's own value lives in its
// core/civ/<model> package.
type Profile struct {
	// Model names the radio for diagnostics, e.g. "FT-890". Empty means
	// unconfigured — see Configured.
	Model string
	// CATID is a fixed, DISPLAY-ONLY synthetic identifier. This family
	// has no wire CAT-ID byte at all (point-to-point, no device
	// address), so nothing here is wire-derived: it exists only to
	// satisfy spec.Capabilities.Validate and the maker-wide registration
	// tests, and identity is proved on the wire by a driver's own probe,
	// never by this string.
	CATID string

	// SlotBase is the lowest channel number this radio accepts (1 for
	// both FT-890 and FT-900); SlotCount is how many consecutive
	// channels follow it. Valid channels are SlotBase..SlotBase+SlotCount-1.
	SlotBase  int
	SlotCount int

	// RecordLen is the per-channel/VFO Status Update record length (19
	// for FT-890/900). FullDumpLen is U=UFullDump's whole-RAM-table reply
	// length — the one genuine wire difference between FT-890 and
	// FT-900 (649 vs 1941 bytes).
	RecordLen   int
	FullDumpLen int

	// Modes maps this radio's wire mode byte to its canonical name.
	// FT-890/900 share the same 5-value legend (ModeLSB..ModeFM); an
	// empty map means unconfigured.
	Modes map[byte]string

	// HasTone reports whether this radio's record carries a CTCSS tone
	// byte at all (true for FT-890/900; FT-1000MP has none — spec.md
	// §Tone table).
	HasTone bool

	// StoreChanArg is the OpStore (VFO→M/Enter) argument index (0-3)
	// carrying the target channel number. Zero value 0 is FT-890/900's
	// own layout ("Args: [CH, P2, —, —]", OpStore's doc comment) — the
	// package's long-standing default, so neither existing Profile
	// literal needs to change. FT-1000MP's own opcode table places its
	// channel argument (X) in the 4th byte instead (ft1000mp capability
	// matrix §1.8: "VFO/MEM – – – X 03H"), a genuinely different frame
	// layout under the same opcode, not a guess — its driver sets this
	// to 3.
	StoreChanArg int

	// OffsetZeroArg and OffsetBoundArg are OpOffset (F9H) argument
	// indices: OffsetZeroArg's byte must be exactly 0x00, OffsetBoundArg's
	// must be 0-2 (the SIMPLEX/Minus/Plus-shaped byte this opcode
	// reuses). FT-890/900's own manual states the FIRST byte must be
	// zero (index 0 — OffsetZeroArg's zero value, so the existing
	// Profile literal is unaffected) and its own second byte carries the
	// bounded value; OFFSETBOUNDARG'S ZERO VALUE IS TREATED AS INDEX 1
	// (offsetBoundArg()) rather than 0, precisely so it need not be set
	// to preserve that default. FT-1000MP's manual states the OPPOSITE
	// byte must be zero (X4, its 4th byte) with X3 (its 3rd byte)
	// carrying the bounded 00H/01H/02H value
	// (ft1000mpmarkv_manual layout:5195-5210, printed p.95-96,
	// "Repeater Offset" row) — a genuinely different byte layout under
	// the same opcode, not a guess; its driver sets OffsetZeroArg=3,
	// OffsetBoundArg=2.
	OffsetZeroArg  int
	OffsetBoundArg int

	// Record offsets, all relative to the start of a 19-byte VFO/Memory
	// Data Record (FT-890 p.33 / FT-900 pp.44-45, identical shape: 1
	// leading Memory Status Flags byte, then a 9-byte front sub-record).
	// The record's SECOND (rear) 9-byte sub-record is not offered an
	// offset at all — see record.go's doc comment.
	FreqOffset  int // 3 bytes, 24-bit binary, 10s of Hz, byte 0 MSB
	ClarOffset  int // 2 bytes, 2's-complement signed offset
	ModeOffset  int // 1 byte
	ToneOffset  int // 1 byte — meaningless unless HasTone
	FlagsOffset int // 1 byte, VFO/Memory Operating Flags (carries shift)
}

// Configured reports whether p describes a real radio. A zero Profile is
// constructible by anyone (`var p bincat.Profile`), and NewFraming refuses
// one outright — core/civ's NewFraming applies the identical guard to a
// zero civ.Profile, for the identical reason: a Framing built from an
// unconfigured Profile is a non-nil interface value whose Allow method
// admits nothing while claiming to speak for a radio.
func (p Profile) Configured() bool {
	return p.Model != "" && p.SlotCount > 0 && p.RecordLen > 0 &&
		p.FullDumpLen > 0 && len(p.Modes) > 0
}

// offsetBoundArg returns OffsetBoundArg, treating its zero value as index
// 1 (FT-890/900's own layout) rather than 0 — see OffsetBoundArg's doc
// comment for why the zero value cannot mean "index 0" here.
func (p Profile) offsetBoundArg() int {
	if p.OffsetBoundArg != 0 {
		return p.OffsetBoundArg
	}
	return 1
}

// MaxFrame is the largest single reply this Profile's radio can return —
// its own full-dump length — which is what the accumulator must be able
// to buffer whole, since U=UFullDump has no smaller boundary within it.
func (p Profile) MaxFrame() int {
	if p.FullDumpLen > FrameLen {
		return p.FullDumpLen
	}
	return FrameLen
}

// ValidChannel reports whether ch is inside this radio's
// SlotBase..SlotBase+SlotCount-1 range.
func (p Profile) ValidChannel(ch int) bool {
	return p.SlotCount > 0 && ch >= p.SlotBase && ch < p.SlotBase+p.SlotCount
}

// ReplyLength reports how many bytes frame's reply will be, and whether it
// expects one at all.
//
// EVERY OPCODE BUT OpStatusUpdate GETS NO REPLY (false): this family's
// write model is fire-and-forget, no ack and no NAK (spec.md §Context) —
// the same property this package's IsRejection encodes as always false.
// Only a Status Update solicits an answer, and because this protocol has
// no terminator, the accumulator can learn the answer's length from
// nowhere but the request about to be sent — which is exactly what
// framing.go's NoteSent calls this for.
func (p Profile) ReplyLength(frame []byte) (n int, ok bool) {
	opcode, args, err := ParseFrame(frame)
	if err != nil {
		return 0, false
	}
	if opcode == OpIdentity {
		return identityReplyLength(args[3])
	}
	if opcode != OpStatusUpdate {
		return 0, false
	}
	switch args[0] {
	case UFullDump:
		return p.FullDumpLen, true
	case UMemoryNumber:
		return 1, true
	case UOperatingData:
		return p.RecordLen, true
	case UBothVFOs:
		return vfoBothLen, true
	case UMemoryRecord:
		return p.RecordLen, true
	default:
		return 0, false
	}
}

// identityReplyLength reports OpIdentity's (FAH) reply length for the F
// argument in frame, when frame is a well-formed OpIdentity command —
// see IdentityShort/IdentityLong.
func identityReplyLength(f byte) (n int, ok bool) {
	switch f {
	case IdentityShort:
		return 5, true
	case IdentityLong:
		return 6, true
	default:
		return 0, false
	}
}

// AllowedCommand is this family's outbound write gate — the last defence
// before a physical radio sees these bytes (safety obligation 1,
// core/transport's Framing.Allow) — re-validating the COMPLETE five-byte
// grammar rather than trusting the opcode byte alone: an unconfigured
// Profile, the wrong length, an opcode this milestone never sends, or an
// argument outside its own documented domain are all refused.
//
// EVERY BOUND CHECKED BELOW IS ONE THE EVIDENCE ACTUALLY STATES (the
// FT-890/900 CAT Commands table, or the FT-1000MP worked examples for the
// opcodes shared across the family) — this is not a guess at plausible
// ranges. A byte the manual marks "—" (padding, value unimportant) is
// accepted at any value; OpClarifier's own argument layout is one such
// case where even the DOCUMENTED byte is not legibly recoverable from this
// package's evidence, so only its opcode and grammar are re-validated —
// see OpClarifier's own doc comment.
func (p Profile) AllowedCommand(frame []byte) bool {
	if !p.Configured() {
		return false
	}
	opcode, args, err := ParseFrame(frame)
	if err != nil {
		return false
	}
	switch opcode {
	case OpStore:
		return p.ValidChannel(int(args[p.StoreChanArg]))
	case OpABSelect:
		return args[0] == 0 || args[0] == 1
	case OpClarifier:
		return true
	case OpSetFreq:
		for _, b := range args {
			if _, err := civ.DecodeBCD2(b); err != nil {
				return false
			}
		}
		return true
	case OpSetMode:
		_, ok := p.Modes[args[0]]
		return ok
	case OpStatusUpdate:
		switch args[0] {
		case UFullDump, UMemoryNumber, UOperatingData, UBothVFOs:
			return true
		case UMemoryRecord:
			return p.ValidChannel(int(args[3]))
		default:
			return false
		}
	case OpShift:
		return args[0] <= 2
	case OpTone:
		return args[0] <= 0x20
	case OpOffset:
		return args[p.OffsetZeroArg] == 0x00 && args[p.offsetBoundArg()] <= 2
	case OpIdentity:
		return args[3] == IdentityShort || args[3] == IdentityLong
	default:
		return false
	}
}

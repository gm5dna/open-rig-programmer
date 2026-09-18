// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import "fmt"

// This file is the TS-2000/TS-2000X/TS-B2000 Satellite Memory codec: SA
// (set/read the satellite status, ts2000:11304-11333, book p.135) and SI
// (enter the satellite memory name, ts2000:11404-11413). It follows
// core/kw/mc.go's shape — plain build/parse functions over a fixed byte
// layout, no answer-domain narrowing — but lives in THIS package rather
// than core/kw because SA/SI belongs to this one document alone: no other
// Kenwood row registered in this project prints it, and core/kw's own
// kw.Command can only be minted by core/kw's own builders (kw.newCommand
// is unexported by design). Command below is this package's own minter,
// the core/kw/ma precedent (core/kw/ma/command.go) restated for a second
// sibling codec: two dialects, two Command types, one
// transport.Command interface.
//
// NO FREQUENCY FIELD ANYWHERE IN THIS RECORD — the manual says so
// outright: "Use the FA (downlink) or FB (uplink) command to change the
// frequencies." (ts2000:11330-11331). core/driver/ts2000's own bank
// registration marks spec.FieldFrequency Unsupported on this bank for
// exactly that reason.
//
// WHICH OF P1/P3-P7 ARE PER-CHANNEL DATA. The manual's "STORING SATELLITE
// MEMORY CHANNELS" section (ts2000:5364-5379) says a channel stores TRACE
// and TRACE REVERSE state, and the band-direction assignment is discussed
// in the same operating flow ("CHANGING THE FREQUENCY BAND",
// ts2000:5384-5390) — so P3 (uplink/downlink swap), P5 (TRACE) and P6
// (TRACE REV) are per-channel and became core/spec Fields
// (FieldSatBandSwap, FieldSatTrace, FieldSatTraceRev). P1 (whole-radio
// satellite mode on/off), P4 (CTRL main/sub) and P7 (MULTI/CH control
// mode) are NOT: P1 is a single radio-wide toggle (ts2000:11310-11311,
// "0: Satellite mode OFF / 1: Satellite mode ON" — not "channel N's
// satellite mode"), and [CTRL] and the MULTI/CH knob's VFO/memory split
// are both GENERIC, whole-radio front-panel concepts documented
// elsewhere in this book with no satellite tie at all (ts2000:4688-4722,
// "CONTROLLING THE SUB-RECEIVER"/"TX BAND AND CONTROL BAND" — [CTRL]
// moves the control focus between MAIN and SUB during ORDINARY,
// non-satellite operation too). SatelliteRecord still carries all seven,
// because the wire frame is fixed-width and always carries all seven;
// only three of them have a core/spec.Field home.
//
// SA SET'S OWN SHAPE IS UNVERIFIED AND ASSUMED (matrix precedent for this
// document: no TS-2000/2000X/B2000 has ever been asked anything). The
// manual prints ONE Set form carrying P2 (the channel) alongside P1 and
// P3-P7 in the same frame, with no separate "recall channel N" command
// and no CAT-reachable equivalent of the front panel's M.IN commit step
// (ts2000:5364-5379 describes M.IN, not a CAT command). This package
// therefore reads BuildSASet the way this project reads every other
// Kenwood memory Set — MW's own precedent: a Set addressed to channel N
// writes that channel's own data directly, the same "just write it"
// contract core/driver/ts2000/write.go already carries for MW. The
// DRIVER (core/driver/ts2000/satellite.go), not this codec, is what
// keeps a satellite write from being a read-corrupting recall: it always
// pre-reads the bare "SA;" answer and threads the live P1/P4/P7 it
// returns back into the Set unchanged, so a per-channel write never
// invents a value for a field this codec does not model as per-channel.

// Command is an outbound Satellite Memory command frame (SA or SI) whose
// bytes were produced and validated by a builder in THIS package. See
// this file's own doc comment for why it is not a kw.Command.
type Command struct {
	frame []byte
}

func newCommand(frame []byte) Command {
	return Command{frame: frame}
}

// Bytes returns a defensive copy of c's wire bytes, satisfying
// transport.Command.
func (c Command) Bytes() []byte {
	return append([]byte(nil), c.frame...)
}

// String renders c safely for logs, satisfying transport.Command.
func (c Command) String() string {
	return fmt.Sprintf("%q", c.frame)
}

// IsZero reports whether c is the zero Command — never built by a package
// builder.
func (c Command) IsZero() bool {
	return c.frame == nil
}

// SatelliteRecord is one SA exchange's full parameter set: P1 through P7,
// plus P8 (the name), which only the Answer carries — a Set never sends
// it (SI does, separately). See this file's own doc comment for which
// fields core/spec publishes.
type SatelliteRecord struct {
	// SatModeOn is P1: whole-radio satellite mode, "0: OFF / 1: ON"
	// (ts2000:11310-11311). NOT per-channel.
	SatModeOn bool
	// Channel is P2: the satellite memory channel, 0-9
	// (ts2000:11312-11313).
	Channel int
	// MainIsDownlink is P3: false is "Main transceiver (uplink)/
	// Sub-receiver (downlink)" (the printed '0'), true is "Main
	// transceiver (downlink)/Sub-receiver (uplink)" (the printed '1')
	// (ts2000:11314-11317). Per-channel — FieldSatBandSwap.
	MainIsDownlink bool
	// CtrlOnSub is P4: "0: CTRL is on the main transceiver / 1: CTRL is
	// on the sub-receiver" (ts2000:11317-11318). NOT per-channel — the
	// same whole-radio [CTRL] focus documented at ts2000:4688-4722.
	CtrlOnSub bool
	// TraceOn is P5: TRACE on/off (ts2000:11319-11320). Per-channel —
	// FieldSatTrace.
	TraceOn bool
	// TraceRevOn is P6: TRACE REV on/off (ts2000:11320-11321).
	// Per-channel — FieldSatTraceRev.
	TraceRevOn bool
	// MultiCHMemoryMode is P7: "0: MULTI/CH control (VFO mode) / 1:
	// MULTI/CH control (Memory channel)" (ts2000:11322-11324). NOT
	// per-channel — a front-panel knob mode, not a satellite attribute.
	MultiCHMemoryMode bool
	// Name is P8: the eight-character satellite channel name
	// (ts2000:11330), ANSWER-ONLY — a Set carries no P8 (SI sets the name
	// instead). Stored/rendered verbatim as eight bytes, the same
	// convention every sibling Kenwood codec's name field uses.
	Name string
}

// The SA frame widths, counted off the manual's own column ruler
// (ts2000:11292-11333).
const (
	// saReadLen is "S A ;" (ts2000:11299-11300).
	saReadLen = 3
	// saSetLen is "S A P1 P2 P3 P4 P5 P6 P7 ;" — ten bytes
	// (ts2000:11296-11298).
	saSetLen = 10
	// saAnswerLen is "S A P1 P2 P3 P4 P5 P6 P7 P8(x8) ;" — eighteen
	// bytes: the same seven flag positions as the Set, then the
	// eight-byte name, then the terminator (ts2000:11304-11309).
	saAnswerLen = 18
	saNameLen   = 8
)

// The SI frame width (ts2000:11400-11413).
const (
	// siSetLen is "S I P1 P2(x8) ;" — twelve bytes: the channel digit,
	// the eight-byte name, the terminator.
	siSetLen = 12
)

var saReadFrame = []byte("SA;")

// BuildSARead builds the bare satellite-status read, "SA;"
// (ts2000:11299-11300). It carries no channel: this book's SA Read
// answers with whatever channel is CURRENTLY SELECTED, not one this
// programme addresses — see this file's own doc comment.
func BuildSARead() (Command, error) {
	return newCommand(append([]byte(nil), saReadFrame...)), nil
}

func boolDigit(on bool) byte {
	if on {
		return '1'
	}
	return '0'
}

// BuildSASet builds "S A P1 P2 P3 P4 P5 P6 P7 ;" (ts2000:11296-11298) for
// rec. rec.Name and rec.Channel outside 0-9 are refused; every other
// field is a plain boolean, which this fixed-width frame always has
// SOME digit for, so there is nothing else to validate.
func BuildSASet(rec SatelliteRecord) (Command, error) {
	if rec.Channel < 0 || rec.Channel > 9 {
		return Command{}, fmt.Errorf("ts2000: SA set: channel %d is outside the satellite bank's 0-9 space", rec.Channel)
	}
	frame := make([]byte, 0, saSetLen)
	frame = append(frame, 'S', 'A')
	frame = append(frame, boolDigit(rec.SatModeOn))
	frame = append(frame, byte('0'+rec.Channel))
	frame = append(frame, boolDigit(rec.MainIsDownlink))
	frame = append(frame, boolDigit(rec.CtrlOnSub))
	frame = append(frame, boolDigit(rec.TraceOn))
	frame = append(frame, boolDigit(rec.TraceRevOn))
	frame = append(frame, boolDigit(rec.MultiCHMemoryMode))
	frame = append(frame, ';')
	if len(frame) != saSetLen {
		return Command{}, fmt.Errorf("ts2000: SA set: built %d bytes, want exactly %d (ts2000:11296-11298)", len(frame), saSetLen)
	}
	return newCommand(frame), nil
}

// parseBoolDigit decodes one of this record's plain '0'/'1' bytes,
// naming what and where on failure.
func parseBoolDigit(frame []byte, off int, what string) (bool, error) {
	switch frame[off] {
	case '0':
		return false, nil
	case '1':
		return true, nil
	default:
		return false, fmt.Errorf("ts2000: SA answer: position %d (%s) is %q, want '0' or '1'", off+1, what, frame[off])
	}
}

// ParseSAAnswer decodes "S A P1 P2 P3 P4 P5 P6 P7 P8(x8) ;"
// (ts2000:11304-11309).
func ParseSAAnswer(frame []byte) (SatelliteRecord, error) {
	if len(frame) != saAnswerLen {
		return SatelliteRecord{}, fmt.Errorf("ts2000: SA answer: %d bytes, want exactly %d (ts2000:11304-11309)", len(frame), saAnswerLen)
	}
	if frame[0] != 'S' || frame[1] != 'A' {
		return SatelliteRecord{}, fmt.Errorf("ts2000: SA answer: does not start \"SA\"")
	}
	if frame[saAnswerLen-1] != ';' {
		return SatelliteRecord{}, fmt.Errorf("ts2000: SA answer: does not end \";\"")
	}
	var rec SatelliteRecord
	var err error
	if rec.SatModeOn, err = parseBoolDigit(frame, 2, "P1, satellite mode"); err != nil {
		return SatelliteRecord{}, err
	}
	if frame[3] < '0' || frame[3] > '9' {
		return SatelliteRecord{}, fmt.Errorf("ts2000: SA answer: position 4 (P2, channel) is %q, want '0'-'9'", frame[3])
	}
	rec.Channel = int(frame[3] - '0')
	if rec.MainIsDownlink, err = parseBoolDigit(frame, 4, "P3, uplink/downlink swap"); err != nil {
		return SatelliteRecord{}, err
	}
	if rec.CtrlOnSub, err = parseBoolDigit(frame, 5, "P4, CTRL main/sub"); err != nil {
		return SatelliteRecord{}, err
	}
	if rec.TraceOn, err = parseBoolDigit(frame, 6, "P5, TRACE"); err != nil {
		return SatelliteRecord{}, err
	}
	if rec.TraceRevOn, err = parseBoolDigit(frame, 7, "P6, TRACE REV"); err != nil {
		return SatelliteRecord{}, err
	}
	if rec.MultiCHMemoryMode, err = parseBoolDigit(frame, 8, "P7, MULTI/CH mode"); err != nil {
		return SatelliteRecord{}, err
	}
	rec.Name = string(frame[9 : 9+saNameLen])
	return rec, nil
}

// BuildSISet builds "S I P1 P2(x8) ;" (ts2000:11400-11409) for channel,
// naming name verbatim as the eight bytes of P2 — space-padded on the
// right when shorter, refused when longer, the same convention every
// sibling Kenwood codec's name field uses.
func BuildSISet(channel int, name string) (Command, error) {
	if channel < 0 || channel > 9 {
		return Command{}, fmt.Errorf("ts2000: SI set: channel %d is outside the satellite bank's 0-9 space", channel)
	}
	if len(name) > saNameLen {
		return Command{}, fmt.Errorf("ts2000: SI set: name %q is %d bytes, want at most %d (ts2000:11330)", name, len(name), saNameLen)
	}
	frame := make([]byte, 0, siSetLen)
	frame = append(frame, 'S', 'I')
	frame = append(frame, byte('0'+channel))
	frame = append(frame, name...)
	for len(frame) < siSetLen-1 {
		frame = append(frame, ' ')
	}
	frame = append(frame, ';')
	if len(frame) != siSetLen {
		return Command{}, fmt.Errorf("ts2000: SI set: built %d bytes, want exactly %d (ts2000:11400-11409)", len(frame), siSetLen)
	}
	return newCommand(frame), nil
}

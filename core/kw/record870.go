// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"bytes"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// This file is the TS-870S's own second memory grid: 22 bytes, not a
// shorter reading of the family's fifty, because P2 does not exist on this
// document at all — every field from P3 onward sits ONE BYTE EARLIER than
// the family's fixed offsets (matrix-ts870s.md's own citation, quoted
// verbatim by the Phase 2 brief: "P3@4-5 not 5-6, freq@6-16 not 7-17,
// mode@17 not 18, lockout@18 not 19, tone-mode@19 not 20, tone number@20-21
// not 21-22"). layout.go's axis idiom varies a byte's MEANING at a FIXED
// offset, never the offset itself, so this is a SECOND RECORD TYPE —
// core/kw/ma's shape, applied to a second document — rather than a
// RecordLen value on the shared Layout. See NewLayout's own refusal for why
// Book870S is excluded from kw.Layout's family check by name.
//
// IT IS ONE ROW'S TYPE, NOT A FAMILY'S, AND THAT IS WHY IT CARRIES NO
// AXES. core/kw/ma exists because the TS-890S and the TS-990S share ONE
// grid and differ on some of it; the TS-870S has no sibling on this grid
// at all (matrix-ts870s.md: "one row, bare New"), so there is nothing for
// an axis to vary between here. Mode and ToneMode are reused BY VALUE from
// the family package; the lockout byte and the tone-mode domain are this
// row's own constants rather than a Byte19Meaning/ToneModeSet axis with a
// single member each.
//
// Six live parameters only — P1, P3, P4, P5, P6, P7, P8 — matrix-ts870s.md:
// "P2 and P9 have no byte at all (Parameter Table prints '-', not a digit
// count)". P6 here is this document's own channel-lockout byte, one
// position earlier than the family's byte 19; P7 admits only two values,
// OFF and TONE (matrix: "ToneModes is 2 values (OFF/TONE)"), narrower than
// even the TS-570's ToneModesTwo, which is why this type hardcodes the
// domain rather than borrowing that axis value.

const (
	rec870Len         = 22
	rec870PrefixOff   = 0
	rec870P1Off       = 2
	rec870ChanOff     = 3
	rec870ChanDigits  = 2
	rec870FreqOff     = 5
	rec870FreqDigits  = 11
	rec870ModeOff     = 16
	rec870LockoutOff  = 17
	rec870ToneModeOff = 18
	rec870ToneOff     = 19
	rec870ToneDigits  = 2
	rec870TermOff     = 21

	// rec870MinToneIndex/rec870MaxToneIndex are THIS ROW'S OWN tone-chart
	// bound, 01-39 (matrix-ts870s.md §1.9's 39-entry SUBTONE TABLE) — NOT
	// the family's kw.MinToneIndex/kw.MaxToneIndex (00-42, the 590/480
	// TN/CN chart width), which this file used before the Lift K follow-up
	// (13/09/2026) and which wrongly let indices 00 and 40-42 round-trip
	// through a codec this document's own chart does not print. There is
	// no index 00 on this chart at all: the matrix's own table starts at
	// 01 (670 decihertz) and ends at 39 (17500).
	rec870MinToneIndex = 1
	rec870MaxToneIndex = 39
)

// Layout870Config is what a future ts870s model package hands
// NewLayout870. EVERY FIELD IS REQUIRED, on Layout's own rule: a zero
// Layout870 fails closed rather than guessing.
type Layout870Config struct {
	Model        string
	MaxEXAddress uint8
	// ModeNames is this row's MD legend, on Layout.ModeNames's own rules:
	// it may not name ModeNone or ModeTune.
	ModeNames map[Mode]string
	// ChannelLo/ChannelHi is this row's one flat memory bank. The matrix
	// records no scan or extension class on this document (channel 99's
	// P1=1 VFO-scan collision is "resolved the TS-480 way", matrix-
	// ts870s.md), so the family's SlotClass/SlotRange machinery would be
	// unused generality here — P1 is always '0'.
	ChannelLo, ChannelHi int
}

// Layout870 is the TS-870S's reading of its own 22-byte grid. The fields
// are unexported and the accessors copy, on Layout's own rule.
type Layout870 struct {
	model                string
	maxEXAddress         uint8
	modeNames            map[Mode]string
	channelLo, channelHi int
}

// NewLayout870 validates cfg and returns the layout it describes, or the
// zero Layout870 and an error wrapping ErrLayoutInvalid.
func NewLayout870(cfg Layout870Config) (Layout870, error) {
	if cfg.Model == "" {
		return Layout870{}, fmt.Errorf("%w: Model is empty — every refusal this layout produces names the row it speaks for", ErrLayoutInvalid)
	}
	if cfg.MaxEXAddress == 0 {
		return Layout870{}, fmt.Errorf("%w (%s): the highest printed EX menu number is unset", ErrLayoutInvalid, cfg.Model)
	}
	if len(cfg.ModeNames) == 0 {
		return Layout870{}, fmt.Errorf("%w (%s): the mode legend is empty — MR/MW P5 carries no legend of its own, so a layout with no MD legend can name no channel's mode", ErrLayoutInvalid, cfg.Model)
	}
	seen := make(map[string]Mode, len(cfg.ModeNames))
	for m, name := range cfg.ModeNames {
		if !m.documentedNibble() {
			return Layout870{}, fmt.Errorf("%w (%s): the mode legend names nibble %q, which this document does not print", ErrLayoutInvalid, cfg.Model, byte(m))
		}
		if !m.namesAMode() {
			return Layout870{}, fmt.Errorf("%w (%s): the mode legend names nibble %q, which is a setting failure or unused rather than a mode a channel can be in", ErrLayoutInvalid, cfg.Model, byte(m))
		}
		if name == "" {
			return Layout870{}, fmt.Errorf("%w (%s): the mode legend gives nibble %q an empty name", ErrLayoutInvalid, cfg.Model, byte(m))
		}
		if prev, dup := seen[name]; dup {
			return Layout870{}, fmt.Errorf("%w (%s): the mode legend gives the name %q to both nibble %q and nibble %q", ErrLayoutInvalid, cfg.Model, name, byte(prev), byte(m))
		}
		seen[name] = m
	}
	if cfg.ChannelLo < 0 || cfg.ChannelHi < cfg.ChannelLo || cfg.ChannelHi > 99 {
		return Layout870{}, fmt.Errorf("%w (%s): channel range %d-%d is not inside 0-99, the two digits P3 carries on this grid", ErrLayoutInvalid, cfg.Model, cfg.ChannelLo, cfg.ChannelHi)
	}

	names := make(map[Mode]string, len(cfg.ModeNames))
	for m, n := range cfg.ModeNames {
		names[m] = n
	}
	return Layout870{
		model:        cfg.Model,
		maxEXAddress: cfg.MaxEXAddress,
		modeNames:    names,
		channelLo:    cfg.ChannelLo,
		channelHi:    cfg.ChannelHi,
	}, nil
}

// MustNewLayout870 is NewLayout870 for a package-level literal, panicking
// on a config it would refuse — MustNewLayout's own reason.
func MustNewLayout870(cfg Layout870Config) Layout870 {
	l, err := NewLayout870(cfg)
	if err != nil {
		panic(err)
	}
	return l
}

// Configured reports whether l carries data. False for the zero Layout870.
func (l Layout870) Configured() bool { return l.modeNames != nil }

// Model is this row's own name, as its refusals quote it.
func (l Layout870) Model() string { return l.model }

// Book is Book870S, always: unlike kw.Layout, which serves several rows
// sharing one document, Layout870 exists for exactly one row, so there is
// no axis for this to vary and no config field for it — see the type's own
// doc comment for why this second record type carries no axes at all.
func (l Layout870) Book() Book { return Book870S }

// MaxEXAddress is the highest EX menu number this row's book prints.
func (l Layout870) MaxEXAddress() uint8 { return l.maxEXAddress }

// ModeNames returns an independent copy of this row's mode legend.
func (l Layout870) ModeNames() map[Mode]string {
	out := make(map[Mode]string, len(l.modeNames))
	for m, n := range l.modeNames {
		out[m] = n
	}
	return out
}

// parseMode870 resolves a P5 wire byte against this layout's own legend.
func (l Layout870) parseMode870(c byte) (Mode, bool) {
	m := Mode(c)
	_, ok := l.modeNames[m]
	return m, ok
}

// Record870 is the decoded content of one 22-byte MR answer or MW Set
// frame on the TS-870S's own document — Record's shape, for the second
// grid. THE RAW-BYTE FIELDS ARE RAW ON PURPOSE, Record's own reason: the
// layout says what a byte means and a driver maps it to a neutral field.
type Record870 struct {
	Channel int
	FreqHz  uint64
	Mode    Mode
	// Lockout is P6, raw wire byte: '0' OFF / '1' ON — this document's own
	// channel lockout, the family's Byte19Lockout meaning reused by value
	// one byte earlier than the family's own byte 19.
	Lockout byte
	// ToneMode is P7: only ToneModeOff and ToneModeTone are legal on this
	// document (matrix-ts870s.md: "ToneModes is 2 values (OFF/TONE)").
	ToneMode  ToneMode
	ToneIndex int

	// Empty reports that the frame carried P4-P8 all zero: "For a vacant
	// channel, the Answer command sends '0' for all parameters except the
	// memory channel number." (ts870s:9101-9104, matrix §1.15/§2.1) — this
	// row's own vacant-channel sentence, the TS-570's own no-tail note
	// (record.go's emptyWindowHiNoTail) verbatim, read independently
	// against this document. Set by the parser only; a builder never
	// writes an empty channel (BuildMWSet refuses one).
	Empty bool
}

// ParseMRAnswer decodes a 22-byte MR ANSWER under this layout's reading of
// its own grid.
func (l Layout870) ParseMRAnswer(frame []byte) (Record870, error) {
	return l.parseRecordFrame870("MR", "MR answer", frame)
}

// isEmptyWindow870 reports whether every byte of frame from P4 through P8
// (0-indexed rec870FreqOff through rec870ToneOff+rec870ToneDigits-1,
// inclusive) is '0' — record.go's isEmptyWindow, restated for this row's
// own offsets rather than shared with it: the two vacant windows agree on
// WHICH fields (P4-P8, "all parameters except the memory channel number",
// ts870s:9101-9104) but not on WHERE those fields sit in the frame, since
// this grid's fields all sit one byte earlier than the family's own
// (P2Unused's absence, doc.go).
func isEmptyWindow870(frame []byte) bool {
	const hi = rec870ToneOff + rec870ToneDigits - 1
	if len(frame) <= hi {
		return false
	}
	for _, b := range frame[rec870FreqOff : hi+1] {
		if b != '0' {
			return false
		}
	}
	return true
}

func (l Layout870) parseRecordFrame870(command, what string, frame []byte) (Record870, error) {
	if !l.Configured() {
		return Record870{}, newParseError(frame, "%s: this layout is unconfigured and describes no radio, so no byte of this frame has a meaning to read", what)
	}
	if len(frame) != rec870Len {
		n := len(frame)
		if n > maxParseErrorFrameLen {
			n = maxParseErrorFrameLen
		}
		return Record870{}, &RecordLengthError{
			Command: command,
			Got:     len(frame),
			Want:    rec870Len,
			reason:  "the TS-870S's own 22-byte grid is a distinct record from the shared kw.Layout family's",
			Frame:   copyBytes(frame[:n]),
		}
	}
	if frame[rec870PrefixOff] != command[0] || frame[rec870PrefixOff+1] != command[1] {
		return Record870{}, newParseError(frame, "%s: missing %q prefix", what, command)
	}
	if frame[rec870TermOff] != ';' {
		return Record870{}, newParseError(frame, "%s: missing ';' terminator at position %d", what, rec870TermOff+1)
	}

	p1 := frame[rec870P1Off]
	if p1 != '0' {
		return Record870{}, newParseError(frame, "%s: P1 is %q — this grid has no scan/extension class to select (channel 99's VFO-scan half is resolved the TS-480 way, matrix-ts870s.md), so P1 is always '0'", what, p1)
	}

	chanDigits := frame[rec870ChanOff : rec870ChanOff+rec870ChanDigits]
	for i, b := range chanDigits {
		if b < '0' || b > '9' {
			return Record870{}, newParseError(frame, "%s: P3 byte %d is %q, not a digit", what, i+1, b)
		}
	}
	channel := int(chanDigits[0]-'0')*10 + int(chanDigits[1]-'0')
	if channel < l.channelLo || channel > l.channelHi {
		return Record870{}, newParseError(frame, "%s: channel %d is outside the %s's %d-%d bank", what, channel, l.model, l.channelLo, l.channelHi)
	}

	// A18a's TS-870S reading, TESTED BEFORE ANY OTHER FIELD IS INTERPRETED
	// — the same ordering record.go's own vacant-window check uses and for
	// the same reason: P5 is byte 17 here, inside this window, so a vacant
	// channel answers with mode nibble '0', a nibble no legend names. A
	// parser that reached parseMode870 first would refuse at the first
	// vacant channel and fail a whole-radio read of a fresh TS-870S.
	if isEmptyWindow870(frame) {
		return Record870{Channel: channel, Empty: true}, nil
	}

	freq, err := parseDigits(frame[rec870FreqOff:rec870FreqOff+rec870FreqDigits], "P4, the frequency")
	if err != nil {
		return Record870{}, newParseError(frame, "%s: %v", what, err)
	}

	mode, ok := l.parseMode870(frame[rec870ModeOff])
	if !ok {
		return Record870{}, newParseError(frame, "%s: P5 is %q, which the %s's MD legend does not name as a mode", what, frame[rec870ModeOff], l.model)
	}

	lockout := frame[rec870LockoutOff]
	if lockout != '0' && lockout != '1' {
		return Record870{}, newParseError(frame, "%s: P6 is %q, and this document prints only '0' Lockout OFF and '1' Lockout ON there", what, lockout)
	}

	tone := ToneMode(frame[rec870ToneModeOff])
	if tone != ToneModeOff && tone != ToneModeTone {
		return Record870{}, newParseError(frame, "%s: P7 is %q, and this document's P7 prints only '0' OFF and '1' TONE", what, byte(tone))
	}

	toneIdx, err := parseDigits(frame[rec870ToneOff:rec870ToneOff+rec870ToneDigits], "P8, the tone number")
	if err != nil {
		return Record870{}, newParseError(frame, "%s: %v", what, err)
	}
	if toneIdx < rec870MinToneIndex || toneIdx > rec870MaxToneIndex {
		return Record870{}, newParseError(frame, "%s: P8 is %d, outside this row's own %02d-%d chart (matrix §1.9), not the family's 00-%d", what, toneIdx, rec870MinToneIndex, rec870MaxToneIndex, MaxToneIndex)
	}

	return Record870{
		Channel:   channel,
		FreqHz:    freq,
		Mode:      mode,
		Lockout:   lockout,
		ToneMode:  tone,
		ToneIndex: int(toneIdx),
	}, nil
}

// BuildMWSet builds the 22-byte memory-channel write for rec.
func (l Layout870) BuildMWSet(rec Record870) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "MW set: this layout is unconfigured and describes no radio")
	}
	if rec.Channel < l.channelLo || rec.Channel > l.channelHi {
		return Command{}, newParseError(nil, "MW set: channel %d is outside the %s's %d-%d bank", rec.Channel, l.model, l.channelLo, l.channelHi)
	}
	if rec.Empty {
		// record.go's own rule, restated: the only documented clear on
		// this document's own family relative (the 590/480 short-MW erase)
		// has no TS-870S counterpart printed anywhere in this manual, and
		// this codec builds no erase frame of any kind. Writing an EMPTY
		// record would ask this builder to manufacture P4-P8 as zero,
		// which is indistinguishable from a genuine 0 Hz/mode-'0' channel
		// this document does not describe either.
		return Command{}, newParseError(nil, "MW set: rec.Empty is true, and this codec builds no empty/erase MW frame for the TS-870S")
	}
	if rec.FreqHz == 0 {
		return Command{}, newParseError(nil, "MW set: P4 is zero")
	}
	if rec.FreqHz > MaxRecordFreqHz {
		return Command{}, &OutOfDomainError{Field: "P4, the memory frequency", Value: rec.FreqHz, Digits: rec870FreqDigits, Max: MaxRecordFreqHz}
	}
	if _, ok := l.parseMode870(rec.Mode.Wire()); !ok {
		return Command{}, newParseError(nil, "MW set: P5 is %q, which the %s's MD legend does not name as a mode", rec.Mode.Wire(), l.model)
	}
	if rec.Lockout != '0' && rec.Lockout != '1' {
		return Command{}, newParseError(nil, "MW set: P6 is %q, and this document prints only '0'/'1'", rec.Lockout)
	}
	if rec.ToneMode != ToneModeOff && rec.ToneMode != ToneModeTone {
		return Command{}, newParseError(nil, "MW set: P7 is %q, and this document's P7 prints only '0' OFF and '1' TONE", rec.ToneMode.Wire())
	}
	if rec.ToneIndex < rec870MinToneIndex || rec.ToneIndex > rec870MaxToneIndex {
		return Command{}, newParseError(nil, "MW set: P8 is %d, outside this row's own %02d-%d chart (matrix §1.9)", rec.ToneIndex, rec870MinToneIndex, rec870MaxToneIndex)
	}

	frame := make([]byte, rec870Len)
	frame[rec870PrefixOff], frame[rec870PrefixOff+1] = 'M', 'W'
	frame[rec870P1Off] = '0'
	copy(frame[rec870ChanOff:], fmt.Sprintf("%02d", rec.Channel))
	copy(frame[rec870FreqOff:], fmt.Sprintf("%0*d", rec870FreqDigits, rec.FreqHz))
	frame[rec870ModeOff] = rec.Mode.Wire()
	frame[rec870LockoutOff] = rec.Lockout
	frame[rec870ToneModeOff] = rec.ToneMode.Wire()
	copy(frame[rec870ToneOff:], fmt.Sprintf("%0*d", rec870ToneDigits, rec.ToneIndex))
	frame[rec870TermOff] = ';'

	if len(frame) != rec870Len {
		return Command{}, newParseError(frame, "MW set: built %d bytes, want exactly %d", len(frame), rec870Len)
	}
	return newCommand(frame), nil
}

// BuildIDRead builds the transceiver-identity read, "ID;" — the SAME three
// bytes kw.Layout's own BuildIDRead builds (identity.go). ID is common to
// every Kenwood book this codec speaks and this row's own document is no
// exception: Read "ID;", Answer "I D" + P1 + ";" (matrix §1.2, PDF p.97
// printed ~91, ts870s:8969-9002) — the identical six-byte, three-digit
// shape identity.go already describes, which is why this is a literal
// rather than a second grammar description.
func (l Layout870) BuildIDRead() (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "ID read: this layout is unconfigured and describes no radio")
	}
	return newCommand([]byte(idReadFrame)), nil
}

// ParseIDAnswer decodes "I D P1 P1 P1 ;" exactly as kw.Layout's own
// ParseIDAnswer does (identity.go) — this row's own document prints the
// same six-byte, three-digit shape (matrix §1.2), and "015" is the printed
// value (matrix §1.2, ts870s:8300-8302).
func (l Layout870) ParseIDAnswer(frame []byte) (string, error) {
	if !l.Configured() {
		return "", newParseError(frame, "ID answer: this layout is unconfigured and describes no radio")
	}
	if len(frame) != IDAnswerLen {
		return "", newParseError(frame, "ID answer: got %d bytes, want exactly %d", len(frame), IDAnswerLen)
	}
	if frame[0] != 'I' || frame[1] != 'D' {
		return "", newParseError(frame, "ID answer: missing \"ID\" prefix")
	}
	if frame[len(frame)-1] != ';' {
		return "", newParseError(frame, "ID answer: missing ';' terminator")
	}
	field := frame[2 : 2+IDDigits]
	for i, b := range field {
		if b < '0' || b > '9' {
			return "", newParseError(frame, "ID answer: P1 byte %d is %q, not a digit (matrix §1.2 prints three digits)", i+1, b)
		}
	}
	return string(field), nil
}

// BuildMRRead builds the memory-channel read request for channel: "M R P1
// P3 P3 ;", six bytes. P1 is always '0': this grid has no scan/extension
// class to select through a read any more than BuildMWSet's own P1 refusal
// admits one on write — channel 99's P1=1 Start/End half is resolved the
// TS-480 way and stays unreachable through this programme (matrix
// §1.4/§2.2).
func (l Layout870) BuildMRRead(channel int) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "MR read: this layout is unconfigured and describes no radio")
	}
	if channel < l.channelLo || channel > l.channelHi {
		return Command{}, newParseError(nil, "MR read: channel %d is outside the %s's %d-%d bank", channel, l.model, l.channelLo, l.channelHi)
	}
	frame := make([]byte, 0, 6)
	frame = append(frame, 'M', 'R', '0')
	frame = append(frame, fmt.Sprintf("%0*d", rec870ChanDigits, channel)...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// MRAnswerMatcher returns the ANSWER MATCHER for one MR read of channel —
// kw.Layout.MRAnswerMatcher's own pattern (matcher.go), restated for this
// row's shifted offsets: the frame must be this grid's own 22 bytes, start
// "MR", and name channel in its own P3 digits. It reads only the channel
// number, not the whole record, so a corrupt answer TO THIS READ is still
// delivered to ParseMRAnswer and refused there with a message naming what
// was wrong with it, rather than silently missing its match and being
// reported as a timeout.
func (l Layout870) MRAnswerMatcher(channel int) func(frame []byte) bool {
	return func(frame []byte) bool {
		if len(frame) != rec870Len {
			return false
		}
		if frame[rec870PrefixOff] != 'M' || frame[rec870PrefixOff+1] != 'R' {
			return false
		}
		d := frame[rec870ChanOff : rec870ChanOff+rec870ChanDigits]
		if d[0] < '0' || d[0] > '9' || d[1] < '0' || d[1] > '9' {
			return false
		}
		got := int(d[0]-'0')*10 + int(d[1]-'0')
		return got == channel
	}
}

// AllowedCommand admits the four grammars a live TS-870S session actually
// needs: the ONE AI Set (initFrame, "AI0;") transport.Engine.Init writes
// at the start of every Kenwood session — this document prints "AI AUTO
// INFORMATION" in its own command index (ts870s:8612), so admitting it is
// not a borrowed convention — the fixed "ID;" read, an MR READ this
// layout's own BuildMRRead would have produced, and an MW SET this
// layout's own BuildMWSet would have produced — validMWCommand's pattern
// (allowlist.go) applied to each: decode, re-validate every field,
// rebuild, and demand equality with what came in.
//
// WIDENED FROM ONE GRAMMAR TO FOUR in the Lift K follow-up (13/09/2026),
// which is what let a TS-870S driver open a live session at all
// (NewFramingFor870's own doc comment): a session that could not even
// clear transport.Engine.Init (which writes AI0; unconditionally, the
// whole Kenwood family's own preamble) could never reach the ID probe
// that follows it. Layout870 still has no MC or EX support — a future
// addition of either must widen this method in the same commit, on the
// standing rule that this is the last defence before bytes reach a
// physical radio.
func (l Layout870) AllowedCommand(frame []byte) bool {
	if !l.Configured() {
		return false
	}
	if len(frame) < IDReadLen || !exactlyOneTrailingSemicolon(frame) {
		return false
	}
	switch string(frame[:2]) {
	case "AI":
		return string(frame) == initFrame
	case "ID":
		return string(frame) == idReadFrame
	case "MR":
		return l.validMRRead870(frame)
	case "MW":
		rec, err := l.parseRecordFrame870("MW", "MW set", frame)
		if err != nil {
			return false
		}
		cmd, err := l.BuildMWSet(rec)
		if err != nil {
			return false
		}
		return bytes.Equal(cmd.Bytes(), frame)
	default:
		return false
	}
}

// validMRRead870 is AllowedCommand's MR leg: decode the channel BuildMRRead
// would need, rebuild, and demand byte equality — the same discipline
// allowlist.go's validMRCommand applies to the family's own MR read.
func (l Layout870) validMRRead870(frame []byte) bool {
	if len(frame) != 6 {
		return false
	}
	if frame[rec870P1Off] != '0' {
		return false
	}
	d := frame[rec870ChanOff : rec870ChanOff+rec870ChanDigits]
	if d[0] < '0' || d[0] > '9' || d[1] < '0' || d[1] > '9' {
		return false
	}
	channel := int(d[0]-'0')*10 + int(d[1]-'0')
	cmd, err := l.BuildMRRead(channel)
	if err != nil {
		return false
	}
	return bytes.Equal(cmd.Bytes(), frame)
}

// NewFramingFor870 returns the transport.Framing for a CONFIGURED
// Layout870: Book870S's own framing.NewFraming, gated by this layout's own
// (currently one-grammar) AllowedCommand rather than kw.Layout's eight —
// NewFramingFor's shape (allowlist.go), for the second record type. This
// is the constructor a TS-870S driver calls; NewFramingFor itself only
// accepts a kw.Layout and always refused this row (the Lift K follow-up
// gap this function closes).
func NewFramingFor870(l Layout870) (transport.Framing, error) {
	if !l.Configured() {
		return nil, fmt.Errorf("%w: the layout is unconfigured and describes no radio, so its outbound gate would speak for none", ErrLayoutInvalid)
	}
	return NewFramingWithGate(Book870S, l.AllowedCommand)
}

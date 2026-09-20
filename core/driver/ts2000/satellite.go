// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts2000 "github.com/gm5dna/open-rig-programmer/core/kw/ts2000"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// This file is the BankSatellite half of ReadChannel/WriteChannel
// (read.go/write.go dispatch on it by bank ID), over core/kw/ts2000's own
// SA/SI codec (satellite.go there). Status: Unverified read AND write,
// consent-gated exactly like the memory bank (caps.go:334-335) — same
// mechanism, same fail-safe posture, no TS-2000/2000X/B2000 has ever been
// asked anything by this project.
//
// satAnswerLen is core/kw/ts2000's own saAnswerLen restated: an SA answer
// is eighteen bytes (kwts2000.ParseSAAnswer's doc comment). It is
// restated rather than exported from that package because it is a
// transport-matcher width, not a codec fact this package otherwise needs.
const satAnswerLen = 18

// parseSatelliteSlot decodes a canonical BankSatellite slot identifier —
// a single digit "0".."9" (caps.go's satSlots) — into the channel number
// SA's own P2 carries.
func parseSatelliteSlot(id string) (int, error) {
	if len(id) != 1 || id[0] < '0' || id[0] > '9' {
		return 0, fmt.Errorf("a satellite bank slot identifier on this row is a single digit \"0\"-\"9\", got %q", id)
	}
	return int(id[0] - '0'), nil
}

// saAnswerSpec is the transport spec for the bare "SA;" read: any frame
// starting "SA" and exactly eighteen bytes long is this exchange's
// answer — there is no per-channel address to narrow the match on, since
// SA's Read carries none (kwts2000's own doc comment).
func (s *Session) saAnswerSpec() transport.CommandSpec {
	return s.newReadSpec(kw.PrefixLenMatcher("SA", satAnswerLen), 1)
}

// readSA sends the bare "SA;" read and parses its answer: whichever
// channel is CURRENTLY SELECTED on the radio, which core/kw/ts2000's own
// doc comment explains is the only channel this command can observe.
func (s *Session) readSA(ctx context.Context) (kwts2000.SatelliteRecord, error) {
	cmd, err := kwts2000.BuildSARead()
	if err != nil {
		return kwts2000.SatelliteRecord{}, fmt.Errorf("ts2000: satellite read: %w", err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.saAnswerSpec())
	if err != nil {
		return kwts2000.SatelliteRecord{}, fmt.Errorf("ts2000: satellite read: %w", wireFailure(s.layout, "SA", err))
	}
	rec, err := kwts2000.ParseSAAnswer(frame)
	if err != nil {
		return kwts2000.SatelliteRecord{}, fmt.Errorf("ts2000: satellite read: %w", err)
	}
	return rec, nil
}

// readSatelliteChannel implements ReadChannel's BankSatellite half: id
// names one of the ten slots, and this succeeds ONLY when that channel
// happens to be the one "SA;" currently reports — a real protocol
// limitation (core/kw/ts2000/satellite.go's doc comment), surfaced the
// same way an ordinary memory answer naming the wrong channel is
// (AnswerMismatchError), not invented for this bank.
func (s *Session) readSatelliteChannel(ctx context.Context, id string) (codeplug.Channel, error) {
	channel, err := parseSatelliteSlot(id)
	if err != nil {
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{Slot: id, Model: s.p.name, Reason: err.Error()}}
	}
	rec, err := s.readSA(ctx)
	if err != nil {
		return codeplug.Channel{}, err
	}
	if rec.Channel != channel {
		return codeplug.Channel{}, &AnswerMismatchError{
			Model: "ts2000", Requested: id, Answered: fmt.Sprintf("%d", rec.Channel),
		}
	}
	return codeplug.Channel{Slot: id, Data: satelliteChannelData(rec)}, nil
}

// satelliteChannelData maps one parsed SA answer onto the neutral channel
// model. Every field this bank does not reach (satelliteBankFields, all
// Unsupported) is published Unavailable, the same convention the memory
// bank's own channelData gives its own unreached fields — including
// FreqHz/Mode/CTCSS/Shift, which stay at their PLAIN zero value because
// this record has no such position at all (caps.go's own doc comment:
// "Use the FA (downlink) or FB (uplink) command").
func satelliteChannelData(rec kwts2000.SatelliteRecord) *codeplug.ChannelData {
	return &codeplug.ChannelData{
		Tag: rec.Name,
		// No tag-display flag, no scan lockout, no clarifier — this
		// record carries none of them.
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},

		TxFreqHz: codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},

		ToneMode: codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:   codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:   codeplug.ToneField{State: codeplug.Unavailable},

		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		Filter:       codeplug.StringField{State: codeplug.Unavailable},
		DataMode:     codeplug.BoolField{State: codeplug.Unavailable},

		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},

		// The three fields this bank DOES reach (SA's P3/P5/P6).
		SatBandSwap: codeplug.BoolField{State: codeplug.Known, Value: rec.MainIsDownlink},
		SatTrace:    codeplug.BoolField{State: codeplug.Known, Value: rec.TraceOn},
		SatTraceRev: codeplug.BoolField{State: codeplug.Known, Value: rec.TraceRevOn},
	}
}

// sendSatellite writes cmd and types the two wire events a Kenwood
// exchange can meet — write.go's own send, generalised to
// transport.Command (kwts2000.Command is not a kw.Command; see
// core/kw/ts2000/satellite.go's own doc comment for why the two families
// mint separate Command types) and to a caller-supplied label so the
// wireFailure message names "SA" or "SI" rather than always "MW".
func (s *Session) sendSatellite(ctx context.Context, cmd transport.Command, label string) error {
	if _, err := s.eng.Do(ctx, cmd, transport.CommandSpec{Class: transport.ClassWrite}); err != nil {
		return wireFailure(s.layout, label, err)
	}
	return nil
}

// writeSatelliteChannel implements WriteChannel's BankSatellite half:
// SA Set (P3/P5/P6, with the live P1/P4/P7 this record also carries
// preserved from a pre-write "SA;" read — core/kw/ts2000/satellite.go's
// own doc comment explains why SA Set is read as a direct per-channel
// write, MW's own precedent, and why P1/P4/P7 must be threaded through
// unchanged rather than invented), THEN SI Set (the name).
//
// BOTH COMMANDS ARE BUILT BEFORE EITHER IS SENT — the ts2000 memory
// write's own ladder (write.go's WriteChannel doc comment) — so a
// SI-side refusal (an over-length name) is reported before SA ever
// reaches the wire.
//
// CALLED WITH s.opMu ALREADY HELD: WriteChannel (write.go) takes the
// operation mutex once, for the whole call including both frames below,
// before it can tell which bank ch.Slot names — exactly the reason the
// memory write path's own comment gives.
func (s *Session) writeSatelliteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	channel, err := parseSatelliteSlot(ch.Slot)
	if err != nil {
		return res, &UnknownSlotError{driver.UnknownSlotError{Slot: ch.Slot, Model: s.p.name, Reason: err.Error()}}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &UnknownSlotError{driver.UnknownSlotError{
			Slot: ch.Slot, Model: s.p.name,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}}
	}

	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this radio's PC-command reference documents no SA/SI-reachable erase route for the Satellite Memory bank — FieldErase is not write-Supported on this bank",
		}
	}
	data := *ch.Data

	for _, m := range []struct {
		field    spec.Field
		state    codeplug.FieldState
		position string
	}{
		{spec.FieldSatBandSwap, data.SatBandSwap.State, "SA P3 at position 5, the uplink/downlink swap (ts2000:11314-11317)"},
		{spec.FieldSatTrace, data.SatTrace.State, "SA P5 at position 7, TRACE (ts2000:11319-11320)"},
		{spec.FieldSatTraceRev, data.SatTraceRev.State, "SA P6 at position 8, TRACE REV (ts2000:11320-11321)"},
	} {
		if m.state != codeplug.Known {
			return res, mandatoryFieldRefusal(ch.Slot, m.field, m.state, m.position)
		}
	}

	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	var unwritable []spec.Field
	for _, f := range []spec.Field{spec.FieldTag, spec.FieldSatBandSwap, spec.FieldSatTrace, spec.FieldSatTraceRev} {
		fs := s.caps.FieldSupport(bank.ID, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session",
		}
	}

	siCmd, err := kwts2000.BuildSISet(channel, data.Tag)
	if err != nil {
		return res, fmt.Errorf("ts2000: WriteChannel %s: %w: %w", ch.Slot, driver.ErrWriteRefused, err)
	}

	// THE ONE READ, preserving P1 (satellite mode)/P4 (CTRL)/P7 (MULTI/CH
	// mode) — live radio state this record also carries but this bank has
	// no per-channel Field for (satelliteChannelData's own doc comment
	// and core/kw/ts2000/satellite.go's).
	//
	// TOCTOU WINDOW: a front-panel change to P1/P4/P7 between this read
	// and the SA Set below reaching the wire is silently overwritten by
	// the value read here — small (one exchange wide, under s.opMu) but
	// real, and distinct from writing back a STALE cached read (there is
	// none: this read is always fresh, immediately before the Set).
	current, err := s.readSA(ctx)
	if err != nil {
		return res, err
	}

	saCmd, err := kwts2000.BuildSASet(kwts2000.SatelliteRecord{
		SatModeOn:         current.SatModeOn,
		Channel:           channel,
		MainIsDownlink:    data.SatBandSwap.Value,
		CtrlOnSub:         current.CtrlOnSub,
		TraceOn:           data.SatTrace.Value,
		TraceRevOn:        data.SatTraceRev.Value,
		MultiCHMemoryMode: current.MultiCHMemoryMode,
	})
	if err != nil {
		return res, fmt.Errorf("ts2000: WriteChannel %s: %w: %w", ch.Slot, driver.ErrWriteRefused, err)
	}

	res.Steps = []driver.WriteStep{{Command: "SA"}, {Command: "SI"}}
	if derr := s.sendSatellite(ctx, saCmd, "SA"); derr != nil {
		res.Steps[0].Sent = errors.Is(derr, transport.ErrRejected)
		return res, fmt.Errorf("ts2000: WriteChannel %s: %w", ch.Slot, derr)
	}
	res.Steps[0].Sent = true

	if derr := s.sendSatellite(ctx, siCmd, "SI"); derr != nil {
		res.Steps[1].Sent = errors.Is(derr, transport.ErrRejected)
		return res, fmt.Errorf("ts2000: WriteChannel %s: %w", ch.Slot, derr)
	}
	res.Steps[1].Sent = true
	return res, nil
}

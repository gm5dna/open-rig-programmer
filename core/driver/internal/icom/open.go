// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Opened is what a successful Open hands back: the pieces a package builds
// its own Session from.
type Opened struct {
	Eng    *transport.Engine
	Stats  civ.AccumulatorStatsReporter
	Report OpenReport
	// ID is the identity passed in, its CATID extended with what the radio
	// answered to 19 00.
	ID driver.Identity
}

// Open is the shared Open choreography, and nothing else on the wire:
// build the CI-V framing, build the engine, Init (a drain alone on CI-V,
// so no radio mutation ever happens here), probe 19 00 for an
// address-matched reply, then read up to len(p.ProbeSlots) memory channels
// for a record whose length confirms the fingerprint.
//
// Open takes ownership of port on BOTH outcomes: the caller's Session
// releases it on success, and Open itself closes it before returning an
// error.
func Open(ctx context.Context, p *Params, port transport.Port, id driver.Identity) (Opened, error) {
	// ONE ENGINE PER NewFraming VALUE: the adapter refuses a second
	// NewAccumulator call loudly, so the framing is built HERE, per Open,
	// and never cached or shared across sessions.
	framing, err := civ.NewFraming(p.Profile())
	if err != nil {
		_ = port.Close()
		return Opened{}, fmt.Errorf("%s: framing: %w", p.Name, err)
	}
	// THE DIAGNOSTICS CARRIER, and why the assertion is two-result:
	// NewFraming's declared result is the neutral transport.Framing, so
	// the adapter's own counters are reachable only through the house
	// optional-capability pattern. Engine.UnexpectedFrames is no
	// substitute: on a CI-V bus it reports a healthy zero on a line
	// saturated with transceive, because the accumulator swallowed those
	// frames first.
	stats, ok := framing.(civ.AccumulatorStatsReporter)
	if !ok {
		_ = port.Close()
		return Opened{}, fmt.Errorf("%s: the CI-V framing does not report accumulator stats — this driver's diagnostics require civ.AccumulatorStatsReporter", p.Name)
	}
	eng, err := transport.NewEngineWith(port, framing)
	if err != nil {
		// NewEngineWith has not taken the port on this path (it refuses
		// before touching it), so closing it here is Open's own ownership
		// obligation, not a double close.
		_ = port.Close()
		return Opened{}, fmt.Errorf("%s: Open: %w", p.Name, err)
	}
	o, err := open(ctx, p, eng, stats, id)
	if err != nil {
		// eng owns port from here on, so closing eng is what releases it.
		_ = eng.Close()
		return Opened{}, err
	}
	return o, nil
}

// open is Open's body, factored so the error path closes eng in exactly
// one place.
func open(ctx context.Context, p *Params, eng *transport.Engine, stats civ.AccumulatorStatsReporter, id driver.Identity) (Opened, error) {
	prof := p.Profile()
	var report OpenReport

	// THE NONFATAL HALF OF THE DRAIN POLICY. Init is a drain alone on
	// CI-V, bounded by an ABSOLUTE cap precisely so it cannot fail the
	// open: a line saturated with traffic addressed to this controller
	// never yields the idle gap, and refusing to open on that basis would
	// make a busy bus indistinguishable from a broken radio. So the
	// INITIAL failure is recorded and stepped over.
	//
	// EVERY LATER DRAIN FAILURE IS FATAL, and the asymmetry is the point:
	// once the session is exchanging frames, a drain that cannot find
	// quiet means this program can no longer tell its own answers from
	// somebody else's, and continuing would be guessing.
	//
	// A BROADCAST FLOOD NEVER GETS HERE AT ALL. Frames addressed to 0x00
	// are counted and dropped by the accumulator before any engine event,
	// so InitDrainCapExceeded can only ever be true under a
	// CONTROLLER-ADDRESSED flood.
	if err := eng.Init(ctx); err != nil {
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			return Opened{}, fmt.Errorf("%s: Open: %w", p.Name, err)
		}
		report.InitDrainCapExceeded = true
	}

	// THE IDENTITY PROBE. What identifies the radio is that an
	// ADDRESS-MATCHED 19 00 reply arrived at all: the reply VALUE is
	// undocumented, so it is recorded as a diagnostic and compared
	// against nothing. The address check belongs to the CODEC (the matcher
	// checks both the `to` and the `from` byte) and is never a rule
	// written here. One retry: an identity read is idempotent, and an open
	// should survive a single swallowed reply.
	idCmd, err := prof.BuildTransceiverIDRead()
	if err != nil {
		return Opened{}, fmt.Errorf("%s: Open: building the 19 00 read: %w", p.Name, err)
	}
	frame, err := eng.Do(ctx, idCmd, civ.CIVReadSpec(prof.TransceiverIDAnswerMatcher(), 1))
	if err != nil {
		// A RADIO AT ANOTHER CI-V ADDRESS LANDS HERE, as a timeout and not
		// as a wrong-radio refusal — nothing was heard from, so nothing
		// can be attributed. A radio at a CI-V baud other than the assumed
		// one lands here identically, so a wrong default-baud guess costs
		// a clean timeout and never a wrong byte.
		return Opened{}, fmt.Errorf("%s: Open: 19 00 identity probe: %w", p.Name, err)
	}
	token, err := prof.ParseTransceiverID(frame)
	if err != nil {
		return Opened{}, fmt.Errorf("%s: Open: 19 00 identity probe: %w", p.Name, err)
	}
	// hex.DecodeString rather than re-slicing the frame: the token is the
	// codec's own rendering of the answer's data bytes, and taking them
	// back from it keeps every piece of frame geometry inside core/civ.
	// An odd-length token cannot arise (the codec renders whole bytes),
	// and if one ever did the raw form is simply not recorded.
	if raw, derr := hex.DecodeString(token); derr == nil {
		report.IDToken = raw
	}
	// The static CATID is the address alone; the session's is that address
	// followed by what this radio actually answered.
	id.CATID = fmt.Sprintf("%02x%s", prof.RadioAddress(), token)

	// THE OCCUPIED-SLOT SEARCH, and the fingerprint it confirms. A
	// rejection means "empty, keep looking"; a record confirms the length;
	// any other error aborts the open.
	for i, a := range p.ProbeSlots {
		report.SlotsTried = i + 1
		raw, empty, err := probeSlot(ctx, p, eng, prof, a)
		if err != nil {
			return Opened{}, err
		}
		if empty {
			continue
		}
		report.Fingerprinted = true
		report.RecordLength = len(raw)
		break
	}
	// AN EMPTY RADIO OPENS ANYWAY, on address evidence alone. Refusing
	// here would make a radio whose memories are all empty unprogrammable
	// by this programme, which is precisely the radio a user most wants to
	// programme.

	report.WireAtOpen = stats.AccumulatorStats()

	return Opened{Eng: eng, Stats: stats, Report: report, ID: id}, nil
}

// probeSlot performs ONE 1A 00 read for the occupied-slot search and
// reports the raw record, or that the slot is empty.
//
// FA IS AN ERROR, NOT A FRAME. Engine.Do consumes the FA and returns
// transport.ErrRejected with NO frame, so the empty branch keys on
// errors.Is(err, transport.ErrRejected) and never on "an FA arrived".
//
// ANSWER-ADDRESS EQUALITY. The landed MemoryAnswerMatcher is deliberately
// envelope-only, so the driver compares the decoded ChannelAddress against
// the one it asked for. That comparison is NOT the first check:
// MemoryAnswerRecord rejects a record of the wrong length (turned into the
// package's length-mismatch error below) before a ChannelAddress exists to
// compare. A wrong-channel answer that is also the wrong length is
// therefore reported as a length mismatch. Both outcomes fail closed.
func probeSlot(ctx context.Context, p *Params, eng *transport.Engine, prof civ.Profile, a civ.ChannelAddress) ([]byte, bool, error) {
	cmd, err := prof.BuildMemoryRead(a)
	if err != nil {
		return nil, false, fmt.Errorf("%s: Open: building the 1A 00 read for %s: %w", p.Name, a, err)
	}
	frame, err := eng.Do(ctx, cmd, civ.CIVReadSpec(prof.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("%s: Open: probing %s: %w", p.Name, a, err)
	}
	got, raw, err := prof.MemoryAnswerRecord(frame)
	if err != nil {
		var lengthErr *civ.RecordLengthError
		if errors.As(err, &lengthErr) {
			return nil, false, p.NewRecordLen(RecordLengthMismatchError{Err: lengthErr, Got: lengthErr.Got, Want: p.RecordLength, Slot: a})
		}
		return nil, false, fmt.Errorf("%s: Open: probing %s: %w", p.Name, a, err)
	}
	if got != a {
		return nil, false, &driver.AnswerMismatchError[civ.ChannelAddress]{Model: p.Name, Requested: a, Answered: got}
	}
	return raw, false, nil
}

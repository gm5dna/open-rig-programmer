// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Want identifies the radio an Open insists it is talking to.
//
// AN ARGUMENT, NOT A Params FIELD, because one driver package can drive
// more than one radio: core/driver/ftdx101 opens the D and the MP from a
// single package with a single Params, and only the Want differs between
// them.
type Want struct {
	// CATID is the ID; answer this Open accepts, and nothing else.
	CATID string
	// Model is the display name for WrongRadioError.WantModel. EMPTY
	// leaves the refusal in its ID-only form, which is what a driver
	// that cannot name the models behind other IDs must do.
	Model string
	// Sibling names the model a foreign CAT ID belongs to, for a family
	// whose members this driver can tell apart. nil leaves GotModel
	// empty, and the refusal then names IDs where it cannot name models.
	Sibling func(catID string) string
}

// NewEngine builds the transport engine an Open drives, wiring in the
// driver's transport logger when it has one.
//
// IT TAKES OWNERSHIP OF port ON FAILURE — closing it before returning —
// because driver.Driver.Open's contract is that the caller never closes
// the port itself once Open has been called.
func NewEngine(port transport.Port, dialect cat.Dialect, logger transport.Logger, p *Params) (*transport.Engine, error) {
	var opts []transport.Option
	if logger != nil {
		opts = append(opts, transport.WithLogger(logger))
	}
	eng, err := transport.NewEngine(port, dialect, opts...)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("%s: Open: %w", p.Name, err)
	}
	return eng, nil
}

// Handshake initialises eng and confirms the radio on the other end is
// the one want names, returning the CAT ID it answered with.
//
// THE ID PROBE IS THE WHOLE GATE. A radio that answers with another
// model's ID is refused with *driver.WrongRadioError before anything else
// is asked of it, because every codec above this point is derived from a
// dialect chosen for one model, and a wrong model's answers would be
// decoded by the wrong grammar.
func Handshake(ctx context.Context, eng *transport.Engine, dialect cat.Dialect, p *Params, want Want) (string, error) {
	if err := eng.Init(ctx); err != nil {
		return "", fmt.Errorf("%s: Open: %w", p.Name, err)
	}

	frame, err := eng.Do(ctx, dialect.BuildIDRead(), IDSpec())
	if err != nil {
		return "", fmt.Errorf("%s: Open: ID probe: %w", p.Name, err)
	}
	got, err := dialect.ParseIDAnswer(frame)
	if err != nil {
		return "", fmt.Errorf("%s: Open: ID probe: %w", p.Name, err)
	}
	if got != want.CATID {
		wrong := &driver.WrongRadioError{Want: want.CATID, Got: got, WantModel: want.Model}
		if want.Sibling != nil {
			wrong.GotModel = want.Sibling(got)
		}
		return "", wrong
	}
	return got, nil
}

// MTSpec is the transport spec for an MT read, its exact answer length
// DERIVED FROM THE DIALECT rather than written out: the geometry belongs
// to the record layout, and a length restated here could drift from it.
//
// A dialect reporting a length WINDOW is refused rather than pinned to
// the window's top, because transport.CATReadSpec takes one exact length
// and silently accepting the top would mean accepting a short answer as
// complete.
func MTSpec(dialect cat.Dialect, p *Params) (transport.CommandSpec, error) {
	lo, hi, err := dialect.MTAnswerBounds()
	if err != nil {
		return transport.CommandSpec{}, fmt.Errorf("%s: MT answer geometry: %w", p.Name, err)
	}
	if lo != hi {
		return transport.CommandSpec{}, fmt.Errorf("%s: MT answer geometry: this dialect reports a %d..%d length WINDOW, but transport.CATReadSpec takes a single exact length — a windowed answer needs a spec that expresses the window, not its top", p.Name, lo, hi)
	}
	return transport.CATReadSpec("MT", hi, p.MTRetries), nil
}

// DiscoverInventory walks this radio's 5xx (60 m) slots and its EMG slot,
// reporting which are populated — the per-radio, REGIONAL inventory no
// static capability table can know, since one model's firmware carries
// the 60 m channels its region licenses and another's carries none.
//
// The walk's end is the DIALECT's: it asks for slot n until SixtyMSlot
// refuses, so the count is a property of the dialect and not a number
// written out here. A radio whose EMG slot is empty on the wire has none.
//
// A Params with NoProbe has no such inventory at all and returns nothing,
// which is the FT-991A: its manual declares no 5xx or EMG slot, so there
// is nothing to ask about and no probe is sent.
func DiscoverInventory(ctx context.Context, eng *transport.Engine, dialect cat.Dialect, p *Params) (slots60m []string, emg bool, err error) {
	if p.Probe == NoProbe {
		return nil, false, nil
	}

	for n := 1; ; n++ {
		slot, serr := dialect.SixtyMSlot(n)
		if serr != nil {
			break
		}
		populated, perr := ProbeSlot(ctx, eng, dialect, p, slot)
		if perr != nil {
			return nil, false, perr
		}
		if populated {
			slots60m = append(slots60m, slot.Wire())
		}
	}

	emgSlot := dialect.EMGSlot()
	if emgSlot.Wire() == "" {
		return slots60m, false, nil
	}
	emg, err = ProbeSlot(ctx, eng, dialect, p, emgSlot)
	if err != nil {
		return nil, false, err
	}
	return slots60m, emg, nil
}

// ProbeSlot reports whether slot holds a channel.
//
// A "?;" REJECTION IS READ AS ABSENT, not as an error: a radio whose
// firmware does not carry the slot at all refuses the read, and that is
// the answer the sweep is asking for. Anything else that goes wrong is an
// error, including an answer naming a DIFFERENT slot — the corruption
// this project refuses everywhere, since the transport's own matcher
// checks the command name and not the address.
func ProbeSlot(ctx context.Context, eng *transport.Engine, dialect cat.Dialect, p *Params, slot cat.Slot) (bool, error) {
	var (
		cmd      cat.Command
		cmdSpec  transport.CommandSpec
		err      error
		answered string
	)

	switch p.Probe {
	case ProbeMR:
		cmd, err = dialect.BuildMRRead(slot)
		cmdSpec = transport.CATReadSpec("MR", p.MRAnswerLen, 1)
	default:
		cmd, err = dialect.BuildMTRead(slot)
		if err == nil {
			cmdSpec, err = MTSpec(dialect, p)
		}
	}
	if err != nil {
		return false, err
	}

	frame, err := eng.Do(ctx, cmd, cmdSpec)
	if errors.Is(err, cat.ErrRejected) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("probe %s: %w", slot.Wire(), err)
	}

	if p.Probe == ProbeMR {
		m, perr := dialect.ParseMRAnswer(frame)
		if perr != nil {
			return false, fmt.Errorf("probe %s: %w", slot.Wire(), perr)
		}
		answered = m.Slot.Wire()
	} else {
		m, _, perr := dialect.ParseMTAnswerCombined(frame)
		if perr != nil {
			return false, fmt.Errorf("probe %s: %w", slot.Wire(), perr)
		}
		answered = m.Slot.Wire()
	}

	if answered != slot.Wire() {
		return false, &driver.AnswerMismatchError[string]{Model: p.Name, Requested: slot.Wire(), Answered: answered}
	}
	return true, nil
}

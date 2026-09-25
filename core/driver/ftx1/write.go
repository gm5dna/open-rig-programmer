// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// fnfSpec is the transport spec for a fire-and-forget Set (MW, MT set): no
// answer expected, only the bounded listen for a delayed "?;" rejection.
func fnfSpec() transport.CommandSpec { return transport.CATWriteSpec() }

// ctcssByName is ctcssNames' (read.go) write-direction inverse — the six
// wire states this radio's write path accepts.
var ctcssByName = map[string]cat.CTCSSState{
	"OFF":      cat.CTCSSOff,
	"ENC-DEC":  cat.CTCSSEncDec,
	"ENC":      cat.CTCSSEnc,
	"DCS":      cat.CTCSSDCSEncDec,
	"PR-FREQ":  cat.CTCSSState('4'),
	"REV-TONE": cat.CTCSSState('5'),
}

// requestedFields lists every spec.Field a write of data actually
// requests: the six fields the MW+MT frame pair always carries
// (frequency/mode/clarifier/ctcss_state/shift/tag), plus CTCSSTone and
// ScanSkip when — and only when — their FieldState is Known (per
// codeplug's write rule, Unknown/Unavailable mean "preserve whatever the
// radio has", i.e. nothing is requested for that field), plus the
// seventeen Icom-tier fields (SHARED, yaesu.TierRequestedFields), each
// only when Known.
//
// UNLIKE THE FT-710, THERE IS NO FieldTagDisplay HERE, conditional or
// otherwise: MTFormShortNoDisplay has no display byte at all (dialect.go),
// so a channel carrying a Known TagDisplay is refused by the general
// capability gate below exactly like any other field this codec cannot
// express (caps.go's bankFields marks FieldTagDisplay the zero
// FieldSupport on every bank) — there is no per-field frame-mapping
// refusal to write here as well, because buildMWMTCommands never reads
// TagDisplay.Value at all.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
	}
	if data.CTCSSTone.State == codeplug.Known {
		fields = append(fields, spec.FieldCTCSSTone)
	}
	if data.ScanSkip.State == codeplug.Known {
		fields = append(fields, spec.FieldScanSkip)
	}
	for _, t := range yaesu.TierRequestedFields {
		if t.Present(data) {
			fields = append(fields, t.Field)
		}
	}
	return fields
}

// WriteChannel implements driver.Session: MW (channel data) then MT
// (tag), both fire-and-forget with the transport's bounded "?;" listen —
// the FT-710's own two-frame choreography, adapted for a tag frame that
// carries no display byte.
//
// Refusal comes FIRST, before ANY wire traffic — defence in depth below
// the clone service: this method re-derives the channel's requested field
// set (requestedFields) and re-checks each against THIS session's
// capabilities (FieldSupport.CanWrite, with spec.Inert additionally
// acceptable-to-transmit — moot on this radio today: no field is ever
// marked Inert, see caps.go). On the all-Unverified fail-safe profile
// NOTHING is writable, so every channel is refused here unless the
// session was built WithConsentedUnverifiedWrites; an empty channel
// (erase) is refused outright, since no CAT erase command is documented
// anywhere in this manual and FieldErase is nowhere write-Supported.
//
// Kind-on-write: the MW frame's P7 is THIS SESSION'S dialect's declared
// write kind (cat.Dialect.MWWriteKind — dialect.go's own MWWriteKind:
// cat.KindMemory, ASSUMED by analogy with the FT-710's HW-CONFIRMED
// finding, never itself confirmed for the FTX-1 — matrix §2, probe 4).
//
// NO read-back: WriteChannel reports only sent/unrejected (see
// driver.WriteResult). Reading the slot back and comparing is the clone
// service's job.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// Every refusal below returns res unchanged — an EXPLICITLY EMPTY
	// step list, never nil (driver.WriteResult's own doc comment: a nil
	// slice would marshal as JSON null, read by an auditor as "unknown"
	// rather than "nothing was attempted").
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	if _, err := s.dialect.ParseSlot(ch.Slot); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid slot: %v", err)}
	}
	bank, ok := s.caps.BankOf(ch.Slot)
	if !ok {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "erase cannot be expressed by the CAT codec (no erase command is documented for a memory channel), and FieldErase is not write-Supported",
		}
	}

	if field, err := driver.CheckFieldStates(s.caps, *ch.Data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	var unwritable []spec.Field
	for _, f := range requestedFields(*ch.Data) {
		fs := s.caps.FieldSupport(bank, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session (the CAT codec cannot express the field, or this session's capability profile does not support writing it)",
		}
	}

	// Build the MT frame FIRST, pure and with no wire traffic, so a
	// tag-encoding failure refuses before any byte goes out — the shared
	// MW body below both builds AND sends the MW frame in one call, so
	// there is no later point to refuse from before that frame is on the
	// wire.
	sl, err := s.dialect.ParseSlot(ch.Slot)
	if err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	mtCmd, err := s.dialect.BuildMTSetNoDisplay(sl, ch.Data.Tag)
	if err != nil {
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTag},
			Reason: fmt.Sprintf("cannot encode MT frame: %v", err),
		}
	}

	// Pre-declared TWO-entry, unlike the shared body's own one-entry
	// MRWriteChannel: dropping the MT placeholder on an MW-side failure
	// would silently change this radio's WriteResult shape.
	res.Steps = []driver.WriteStep{{Command: "MW"}, {Command: "MT"}}
	const (
		mwStep = 0
		mtStep = 1
	)

	mwRes, err := yaesu.MRWriteChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, ch)
	if len(mwRes.Steps) > 0 {
		res.Steps[mwStep] = mwRes.Steps[0]
	}
	if err != nil {
		return res, err
	}

	if _, err := s.eng.Do(ctx, mtCmd, fnfSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			res.Steps[mtStep].Sent = true
			return res, fmt.Errorf("ftx1: WriteChannel %s: MT rejected by radio: %w", ch.Slot, err)
		}
		return res, fmt.Errorf("ftx1: WriteChannel %s: MT: %w", ch.Slot, err)
	}
	res.Steps[mtStep].Sent, res.Steps[mtStep].Confirmed = true, true

	return res, nil
}

// buildMWMTCommands maps a populated channel onto its MW and MT Set
// frames, refusing (typed, via *driver.WriteRefusedError) any value the
// codec cannot express. Kept as a Session method — not inlined at
// WriteChannel's own call site, which now builds the two frames in the
// opposite order for the refusal-before-wire reason given there — because
// write_test.go exercises it directly. The MW frame goes through the
// shared yaesu.BuildMWCommand body (mrParams, read.go); the MT frame
// stays bespoke, unchanged: MTFormShortNoDisplay is this radio's own
// frame shape, with nothing in core/driver/internal/yaesu that builds it.
func (s *Session) buildMWMTCommands(ch codeplug.Channel) (mwCmd, mtCmd cat.Command, err error) {
	mwCmd, err = yaesu.BuildMWCommand(s.dialect, s.caps, &mrParams, ch)
	if err != nil {
		return cat.Command{}, cat.Command{}, err
	}

	sl, err := s.dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	// No display argument: MTFormShortNoDisplay's own builder takes a
	// slot and a tag only (mtnodisplay.go).
	mtCmd, err = s.dialect.BuildMTSetNoDisplay(sl, ch.Data.Tag)
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTag},
			Reason: fmt.Sprintf("cannot encode MT frame: %v", err),
		}
	}
	return mwCmd, mtCmd, nil
}

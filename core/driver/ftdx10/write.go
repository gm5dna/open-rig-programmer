// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ctcssByName is this radio's CTCSS state vocabulary, as the map the
// tests walk. params.CTCSS is the ordered original: the order is the
// manual's own legend order, which the refusal text names.
var ctcssByName = yaesu.CTCSSMap(params.CTCSS)

// shiftByName is the repeater-shift vocabulary, the same three names on
// every one of these radios.
var shiftByName = yaesu.ShiftByName

// mtSetSpec is the transport spec for the MT Set frame.
func mtSetSpec() transport.CommandSpec { return yaesu.MTSetSpec() }

// bankFor reports which bank of this session's capabilities owns slot.
func (s *Session) bankFor(slot string) (spec.BankID, bool) { return s.caps.BankOf(slot) }

// requestedFields is the set of spec.Fields a write of data asks for —
// see yaesu.RequestedFields.
func requestedFields(data codeplug.ChannelData) []spec.Field { return yaesu.RequestedFields(data) }

// tierRequestedFields is the seventeen tier-added fields and their
// per-channel presence predicates — see yaesu.TierRequestedFields.
var tierRequestedFields = yaesu.TierRequestedFields

// WriteChannel implements driver.Session: writes one memory channel as a
// single combined MT Set frame — the shared Yaesu one-frame write path,
// this radio's refusals and codec coming from params.
//
// It takes the session's operation mutex for the whole operation.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return yaesu.WriteChannel(ctx, s.eng, s.dialect, s.caps, &params, ch)
}

// buildWriteCommand renders ch as this radio's combined MT Set frame, or
// refuses naming the field at fault — see yaesu.BuildWriteCommand for the
// order the refusals fire in.
//
// It takes the session's capability set, which this radio's write path
// genuinely consults: params.CheckFreqRange refuses a frequency outside
// the range those capabilities declare, before the codec's own encoding
// is asked.
func buildWriteCommand(dialect cat.Dialect, caps spec.Capabilities, ch codeplug.Channel) (cat.Command, error) {
	return yaesu.BuildWriteCommand(dialect, caps, &params, ch)
}

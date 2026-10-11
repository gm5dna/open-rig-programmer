// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Params is everything the shared Open and Read engine needs to know
// about one radio. Every value is that radio's own: the engine holds no
// model literal and no default that would act as a claim about a model,
// so an unset field means "not this radio" and never "the usual".
// Profile and NewRecordLen are the exceptions: without them there is no
// radio and no error type to raise, so Open and ReadRaw refuse a Params
// that leaves either unset. Domain, ReadCheck and TXDuplicate may be left
// unset.
//
// Each driver package declares one Params value and passes a pointer to
// it into the engine functions beside its own state, which stays on its
// own Session.
type Params struct {
	// Name is the lower-case package name: the prefix of every error the
	// engine raises and the model in AnswerMismatchError.
	Name string
	// Profile is the radio's CI-V profile, the only source of frames and
	// answer matchers. Required.
	Profile func() civ.Profile
	// RecordLength is the record-only length the profile declares, named
	// in RecordLengthMismatchError.
	RecordLength int
	// ProbeSlots is the occupied-slot search schedule: the memory channels
	// Open reads, in order, before giving up and opening UNFINGERPRINTED.
	//
	// BOUNDED ON PURPOSE. The fingerprint's job is to confirm a record length,
	// which one occupied channel settles; walking all 99 to find a radio's
	// only populated memory would put ninety-nine exchanges into every open
	// for no further evidence. Ten is enough to clear a radio whose first few
	// memories happen to be empty and short enough that an entirely empty
	// radio opens promptly — on address evidence alone, which spec D3.2
	// explicitly allows.
	ProbeSlots []civ.ChannelAddress
	// Domain, if set, refuses a channel carrying a value this radio's
	// record cannot express. A read hands it a channel carrying only
	// FreqHz, plus TxFreqHz (Known) when TXDuplicate is set.
	Domain func(codeplug.ChannelData, spec.Capabilities) error
	// ReadCheck, if set, refuses a record from its raw bytes after the
	// empty-slot test and before the parse. Its error is returned as given.
	ReadCheck func(raw []byte) error
	// TXDuplicate marks a radio whose record always carries its own
	// TX-duplicate frequency: a read reports it Known, and a record
	// without one is an error. Unset, TxFreqHz reads Unavailable.
	TXDuplicate bool
	// NewRecordLen wraps the engine's mismatch in the package's own error
	// type, which owns the wording. Required.
	NewRecordLen func(RecordLengthMismatchError) error
}

// validate refuses a Params missing a field the engine cannot do without,
// so a half-filled value is an error rather than a nil-function panic.
func (p *Params) validate() error {
	if p.Profile == nil || p.NewRecordLen == nil {
		return fmt.Errorf("%s: the engine's Params leaves Profile or NewRecordLen unset", p.Name)
	}
	return nil
}

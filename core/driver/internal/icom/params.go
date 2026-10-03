// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Params is everything the shared Open and Read engine needs to know
// about one radio. Every value is that radio's own: the engine holds no
// model literal and no default that would act as a claim about a model,
// so an unset field means "not this radio" and never "the usual".
//
// Each driver package declares one Params value and passes a pointer to
// it into the engine functions beside its own state, which stays on its
// own Session.
type Params struct {
	// Name is the lower-case package name: the prefix of every error the
	// engine raises and the model in AnswerMismatchError.
	Name string
	// Profile is the radio's CI-V profile, the only source of frames and
	// answer matchers.
	Profile func() civ.Profile
	// RecordLength is the record-only length the profile declares, named
	// in RecordLengthMismatchError.
	RecordLength int
	// ProbeSlots is the occupied-slot search schedule, in order.
	ProbeSlots []civ.ChannelAddress
	// Domain refuses a channel carrying a value this radio's record
	// cannot express. A read hands it a channel carrying only FreqHz.
	Domain func(codeplug.ChannelData, spec.Capabilities) error
	// ReadCheck, if set, refuses a record from its raw bytes after the
	// empty-slot test and before the parse. Its error is returned as given.
	ReadCheck func(raw []byte) error
	// NewRecordLen wraps the engine's mismatch in the package's own error
	// type, which owns the wording.
	NewRecordLen func(RecordLengthMismatchError) error
}

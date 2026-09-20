// SPDX-License-Identifier: GPL-3.0-or-later

// Package icom holds the struct shapes six or eight Icom driver packages
// (ic7200, ic7410, ic7600, ic7610, ic7700, ic7760, ic7800, ic7851) each
// minted their own byte-identical copy of, mirroring
// core/driver/internal/yaesu's own reason for existing. Unlike that
// package this one holds no shared BEHAVIOUR — no Params, no funcs — only
// the two struct shapes whose FIELDS are identical across their owning
// packages while their Error() wording is not: each owning package keeps
// its own Error() by embedding one of these, so a merged field layout
// costs nothing in wire-facing text.
package icom

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// OpenReport is what Open observed while probing, kept on a Session
// rather than on the neutral seam: driver.SessionDiagnostics carries ONE
// aggregate counter (core/driver/optional.go) and cannot carry any of
// this, and widening the neutral seam is a tier-shared change five
// worktrees would want. Six Icom packages (ic7600, ic7610, ic7700,
// ic7760, ic7800, ic7851) minted byte-identical copies of this shape.
type OpenReport struct {
	// IDToken is what the radio answered to 19 00, recorded and never
	// matched.
	IDToken []byte
	// SlotsTried is how many channels the occupied-slot search read
	// before it stopped.
	SlotsTried int
	// Fingerprinted is false when every probed slot was rejected — an
	// empty radio, opened on address evidence alone.
	Fingerprinted bool
	// RecordLength is the record-only length the fingerprint confirmed,
	// or 0 when Fingerprinted is false.
	RecordLength int
	// InitDrainCapExceeded records that Engine.Init hit its absolute
	// drain cap and that Open continued anyway — the NONFATAL half of
	// R9-SPLIT. It can only be true under a CONTROLLER-ADDRESSED flood:
	// a to=00 broadcast flood never reaches the engine at all.
	InitDrainCapExceeded bool
	// WireAtOpen is the adapter's own counter snapshot taken when Open
	// returned, so a broadcast-saturated line is visible even though the
	// engine saw nothing.
	WireAtOpen civ.AccumulatorStats
}

// RecordLengthMismatchError reports that a memory answer carried a record
// at a length its profile does not declare — the probe's CONTINUOUS
// length fingerprint failing.
//
// IT NAMES NO FOUND MODEL, and driver.WrongRadioError is deliberately not
// used for that reason: that type's whole shape is a pair of CAT IDs, and
// filling it with lengths would put a made-up identity in a field callers
// render as one. Cross-model record-length distinctness is a TIER-LEVEL
// check and no owning package holds a table of other radios' lengths, so
// the honest refusal says what was measured, what was expected, and that
// the expectation is itself ASSUMED. Eight Icom packages (ic7200, ic7410,
// ic7600, ic7610, ic7700, ic7760, ic7800, ic7851) minted byte-identical
// copies of this shape; each keeps its own Error() by embedding, since
// the citation clause differs by package.
type RecordLengthMismatchError struct {
	// Err is the codec refusal whose measured and accepted lengths are
	// retained for errors.As callers.
	Err *civ.RecordLengthError
	// Got is the record-only length the radio's answer carried.
	Got int
	// Want is the length this profile declares.
	Want int
	// Slot is the channel the answer spoke for.
	Slot civ.ChannelAddress
}

// Unwrap exposes both classifications supported by the harmonised
// mismatch wrappers. errors.Unwrap returns nil for this multi-error form;
// callers use errors.Is and errors.As. Identical across every owning
// package, so it lives here once and reaches each embedding package's own
// *RecordLengthMismatchError by promotion.
func (e *RecordLengthMismatchError) Unwrap() []error {
	return []error{driver.ErrWrongRadio, e.Err}
}

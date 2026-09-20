// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// settingsDescriptorVersion is minted HERE — the exact string this
// driver's SettingsDescriptor identifies itself with, and the one
// codeplug.MenuSnapshot.Descriptor carries through verbatim so a snapshot
// can later be checked against the descriptor version that produced it.
// The "@1" is this shape's own generation: a later change to how THIS
// driver builds its tree increments it here alone.
const settingsDescriptorVersion = "ft710-ex@1"

// ft710SettingsDescriptor is built ONCE, at package init, from the EX
// inventory this package's dialect carries.
//
// Every getter returns a Clone() of this and never the value itself:
// nothing outside this file may hold a reference to the shared original,
// because a caller that mutated the tree it was handed would silently
// change what every later caller received (driver.SettingsDescriptor.
// Clone's own doc comment).
var ft710SettingsDescriptor = buildSettingsDescriptor(catDialect)

// buildSettingsDescriptor builds this radio's tree from dialect's EX
// inventory — the shared body, this radio's own grouping and version
// coming from params. Nothing about the FT-710's own numbers is written
// into it: they are properties of the inventory it is handed, asserted in
// settings_test.go against the dialect rather than against literals.
func buildSettingsDescriptor(dialect cat.Dialect) driver.SettingsDescriptor {
	return yaesu.BuildDescriptor(dialect, &params)
}

// SettingsDescriptor returns the FT-710's radio-neutral settings
// descriptor: a defensive Clone() of the tree built once at package init.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ft710SettingsDescriptor.Clone()
}

// SettingsDescriptor implements the optional driver.SettingsReader
// capability (see that interface's doc comment) on the concrete *Session.
// Identical to the package-level func and to the driver's
// StaticSettingsDescriptor (ft710.go): this driver's settings tree
// depends only on the static EX inventory, never on anything a live
// session discovers.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError is the refusal ReadSetting returns for an id that
// does not name a known FT-710 EX (MENU) address — refused BEFORE any
// wire traffic, exactly like ReadChannel's malformed-slot refusal.
type UnknownSettingError = driver.UnknownSettingError

// SettingAnswerMismatchError is the refusal for an EX answer that named a
// DIFFERENT wire address than the one just requested.
type SettingAnswerMismatchError = driver.SettingAnswerMismatchError

// exSpec is the transport spec for an EX read of addr.
func exSpec(dialect cat.Dialect, addr cat.EXAddress) transport.CommandSpec {
	return yaesu.EXSpec(dialect, addr)
}

// parseEXResponse interprets the outcome of one EX exchange for
// requested — see yaesu.ParseEXResponse, including why its wrong-address
// branch is reachable only from a test.
func parseEXResponse(dialect cat.Dialect, requested cat.EXAddress, frame []byte) (driver.SettingValue, error) {
	return yaesu.ParseEXResponse(dialect, &params, requested, frame)
}

// ReadSetting implements the optional driver.SettingsReader capability:
// reads one FT-710 EX (MENU) setting by its opaque, radio-neutral id,
// which this driver mints as the setting's EX wire address.
//
// It holds the session's operation mutex for the whole read, like this
// driver's two-exchange memory operations: one rule — an operation on a
// session excludes another — rather than an exception per method.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return yaesu.ReadSetting(ctx, s.eng, s.dialect, &params, id)
}

// WriteSetting implements the optional driver.SettingsWriter capability:
// writes one FT-710 EX (MENU) setting by its opaque, radio-neutral id
// (this driver's EX wire address), then verifies it with a paired read.
//
// id is parsed FIRST, entirely before any wire traffic, exactly as
// ReadSetting does: a malformed shape, or a well-formed address that is
// not a member of this dialect's inventory, refuses with
// *UnknownSettingError and nothing is ever sent.
//
// s.dialect.BuildEXSet is the ONE gate below that: it refuses (before any
// wire traffic) for every other reason the write descriptor can name —
// denied, held, uncharacterised (no Session W row yet, the write-gate's
// bootstrap-closed default), wrong width, or a value outside the
// address's domain — one call, so none of those reasons can drift apart
// (core/cat/ex.go's exSetP4OK doc comment). NO CONSENT PREDICATE (R1):
// there is no settingsCanWrite check and no consented parameter here —
// this call site is the whole of the write gate.
//
// Past that gate the Set is fire-and-forget (waitFireAndForget via
// fnfSpec/engine.go's ClassWrite path): a "?;" within the error window is
// SettingWriteRefused; silence is provisional success, settled only by
// the paired read-back that follows, via yaesu.ReadSetting directly
// (never s.ReadSetting, which takes s.opMu itself and would deadlock
// against the lock this method already holds for the whole pair).
//
// It holds the session's operation mutex for the whole Set+read-back
// pair, like ReadSetting and the memory write path: one rule, an
// operation on a session excludes another, rather than an exception per
// method.
func (s *Session) WriteSetting(ctx context.Context, id, value string) (driver.SettingWriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	addr, err := s.dialect.ParseEXAddress(id)
	if err != nil {
		return driver.SettingWriteResult{}, &driver.UnknownSettingError{Model: params.Model, ID: id}
	}

	cmd, err := s.dialect.BuildEXSet(addr, value)
	if err != nil {
		return driver.SettingWriteResult{}, fmt.Errorf("%s: WriteSetting %s: %w", params.Name, id, err)
	}

	res := driver.SettingWriteResult{ID: id, Wanted: value, Step: driver.WriteStep{Command: "EX"}}

	if _, err := s.eng.Do(ctx, cmd, fnfSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			// The frame WAS transmitted; the radio explicitly refused it
			// within its error window.
			res.Step.Sent = true
			res.Outcome = driver.SettingWriteRefused
			return res, fmt.Errorf("%s: WriteSetting %s: rejected by radio: %w", params.Name, id, err)
		}
		// Transport-level failure: the Set's fate is not attributable.
		res.Outcome = driver.SettingWriteOutcomeUnknown
		return res, fmt.Errorf("%s: WriteSetting %s: %w", params.Name, id, err)
	}
	res.Step.Sent, res.Step.Confirmed = true, true

	got, err := yaesu.ReadSetting(ctx, s.eng, s.dialect, &params, id)
	if err != nil {
		// The paired read-back's own outcome is unattributable (a
		// transport failure, not a "?;" — yaesu.ReadSetting maps "?;" to
		// SettingUnavailable, not an error): the Set may well have
		// landed, but nothing here can say so.
		res.Outcome = driver.SettingWriteOutcomeUnknown
		return res, fmt.Errorf("%s: WriteSetting %s: verify read-back: %w", params.Name, id, err)
	}
	if got.State == driver.SettingUnavailable {
		// The Set drew no rejection, but the immediate read-back was
		// itself refused: the link answered SOMETHING, but not the value
		// just written, so this is not a mismatch against a known value
		// — it is unresolved.
		res.Outcome = driver.SettingWriteOutcomeUnknown
		return res, fmt.Errorf("%s: WriteSetting %s: verify read-back refused (\"?;\")", params.Name, id)
	}

	res.Observed = got.Raw
	if got.Raw != value {
		res.Outcome = driver.SettingWriteVerifyMismatch
		return res, &driver.SettingVerifyMismatchError{ID: id, Wanted: value, Observed: got.Raw}
	}

	res.Outcome = driver.SettingWriteAccepted
	return res, nil
}

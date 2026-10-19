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

// WriteSetting is the shared body of every Yaesu driver's optional
// driver.SettingsWriter capability: writes one EX (MENU) setting by its
// opaque, radio-neutral id (the EX wire address), then verifies it with a
// paired read.
//
// id is parsed FIRST, entirely before any wire traffic, exactly as
// ReadSetting does: a malformed shape, or a well-formed address that is
// not a member of this dialect's inventory, refuses with
// *UnknownSettingError and nothing is ever sent.
//
// dialect.BuildEXSet is the ONE gate below that: it refuses (before any
// wire traffic) for every other reason the write descriptor can name —
// denied, held, uncharacterised (no Session W row yet, the write-gate's
// bootstrap-closed default), wrong width, or a value outside the
// address's domain — one call, so none of those reasons can drift apart
// (core/cat/ex.go's exSetP4OK doc comment). NO CONSENT PREDICATE (R1):
// there is no settingsCanWrite check and no consented parameter here —
// this call site is the whole of the write gate.
//
// Past that gate the Set is fire-and-forget (waitFireAndForget via
// transport.CATWriteSpec, the engine's ClassWrite path): a "?;" within the error window is
// SettingWriteRefused; silence is provisional success, settled only by
// the paired read-back that follows, via yaesu.ReadSetting directly
// (never a session's ReadSetting, which takes the operation mutex itself and
// would deadlock against the lock the caller already holds for the pair).
//
// It runs the whole Set+read-back pair; the caller holds its own operation
// mutex around the call, as for ReadSetting, and nothing here takes a lock.
func WriteSetting(ctx context.Context, eng *transport.Engine, dialect cat.Dialect, p *Params, id, value string) (driver.SettingWriteResult, error) {
	addr, err := dialect.ParseEXAddress(id)
	if err != nil {
		return driver.SettingWriteResult{}, &driver.UnknownSettingError{Model: p.Model, ID: id}
	}

	cmd, err := dialect.BuildEXSet(addr, value)
	if err != nil {
		return driver.SettingWriteResult{}, fmt.Errorf("%s: WriteSetting %s: %w", p.Name, id, err)
	}

	res := driver.SettingWriteResult{ID: id, Wanted: value, Step: driver.WriteStep{Command: "EX"}}

	if _, err := eng.Do(ctx, cmd, transport.CATWriteSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			// The frame WAS transmitted; the radio explicitly refused it
			// within its error window.
			res.Step.Sent = true
			res.Outcome = driver.SettingWriteRefused
			return res, fmt.Errorf("%s: WriteSetting %s: rejected by radio: %w", p.Name, id, err)
		}
		// Transport-level failure: the Set's fate is not attributable.
		res.Outcome = driver.SettingWriteOutcomeUnknown
		return res, fmt.Errorf("%s: WriteSetting %s: %w", p.Name, id, err)
	}
	res.Step.Sent, res.Step.Confirmed = true, true

	got, err := ReadSetting(ctx, eng, dialect, p, id)
	if err != nil {
		// The paired read-back's own outcome is unattributable (a
		// transport failure, not a "?;" — yaesu.ReadSetting maps "?;" to
		// SettingUnavailable, not an error): the Set may well have
		// landed, but nothing here can say so.
		res.Outcome = driver.SettingWriteOutcomeUnknown
		return res, fmt.Errorf("%s: WriteSetting %s: verify read-back: %w", p.Name, id, err)
	}
	if got.State == driver.SettingUnavailable {
		// The Set drew no rejection, but the immediate read-back was
		// itself refused: the link answered SOMETHING, but not the value
		// just written, so this is not a mismatch against a known value
		// — it is unresolved.
		res.Outcome = driver.SettingWriteOutcomeUnknown
		return res, fmt.Errorf("%s: WriteSetting %s: verify read-back refused (\"?;\")", p.Name, id)
	}

	res.Observed = got.Raw
	if got.Raw != value {
		res.Outcome = driver.SettingWriteVerifyMismatch
		return res, &driver.SettingVerifyMismatchError{ID: id, Wanted: value, Observed: got.Raw}
	}

	res.Outcome = driver.SettingWriteAccepted
	return res, nil
}

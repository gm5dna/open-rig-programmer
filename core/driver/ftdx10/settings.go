// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// settingsDescriptorVersion is minted HERE — the exact string this
// driver's SettingsDescriptor identifies itself with, and the one
// codeplug.MenuSnapshot.Descriptor carries through verbatim so a snapshot
// can later be checked against the descriptor version that produced it.
//
// A DIFFERENT string from the FT-710's "ft710-ex@1", necessarily: the two
// descriptors describe different radios' menus (197 items in four menus
// here, 296 in five there), so a snapshot taken from one must never
// validate against the other. The "@1" is this shape's own generation —
// a later change to how THIS driver builds its tree increments it here
// alone.
const settingsDescriptorVersion = "ftdx10-ex@1"

// ftdx10SettingsDescriptor is built ONCE, at package init, from the EX
// inventory this package's dialect carries (catDialect.EXItems, generated
// from core/cat/ftdx10/table2.csv).
//
// Every getter returns a Clone() of this and never the value itself:
// nothing outside this file may hold a reference to the shared original,
// because a caller that mutated the tree it was handed would silently
// change what every later caller received (driver.SettingsDescriptor.
// Clone's own doc comment).
var ftdx10SettingsDescriptor = buildSettingsDescriptor(catDialect)

// buildSettingsDescriptor builds this radio's tree from dialect's EX
// inventory — the shared body, this radio's own grouping and version
// coming from params. Nothing about the FTdx10's own numbers (197 items,
// P1 in {01,02,03,04}, 18 groups) is written into it: they are properties
// of the inventory, asserted in settings_test.go against the dialect.
func buildSettingsDescriptor(dialect cat.Dialect) driver.SettingsDescriptor {
	return yaesu.BuildDescriptor(dialect, &params)
}

// SettingsDescriptor returns the FTdx10's radio-neutral settings
// descriptor: a defensive Clone() of the tree built once at package init.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ftdx10SettingsDescriptor.Clone()
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (core/driver/optional.go): the
// driver-level, no-session-required counterpart to
// Session.SettingsDescriptor. Identical to it and to the package-level
// func: this driver's settings tree depends only on the static EX
// inventory, never on anything a live session discovers.
func (d *ftdx10Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements the optional driver.SettingsReader
// capability on the concrete *Session. It deliberately does NOT consult
// s.dialect, even though the session carries one: the package-level tree
// is built from catDialect, the single dialect every session of this
// driver is opened with, and a per-session rebuild would cost 197 items of
// allocation per call to produce the identical answer.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError is the refusal ReadSetting returns for an id that
// does not name a known FTdx10 EX (MENU) address — refused BEFORE any wire
// traffic, exactly like ReadChannel's malformed-slot refusal.
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
// reads one FTdx10 EX (MENU) setting by its opaque, radio-neutral id,
// which this driver mints as the setting's 6-digit EX wire address.
//
// It takes the operation mutex like every other operation on this
// Session: one EX read is one exchange today, and holding the mutex costs
// nothing while keeping the rule "an operation on this session excludes
// another" true of the whole surface rather than of most of it.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return yaesu.ReadSetting(ctx, s.eng, s.dialect, &params, id)
}

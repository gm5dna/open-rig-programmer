// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

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
// can later be checked against the descriptor version that produced it. A
// different string from every other radio's, so a snapshot taken from one
// can never validate against another's.
const settingsDescriptorVersion = "ft991a-ex@1"

// flatMenuID is the single menu and group this radio's descriptor holds —
// see buildSettingsDescriptor.
const flatMenuID = yaesu.FlatMenuID

// ft991aSettingsDescriptor is built ONCE, at package init, from the EX
// inventory this package's dialect carries.
//
// Every getter returns a Clone() of this and never the value itself:
// nothing outside this file may hold a reference to the shared original,
// because a caller that mutated the tree it was handed would silently
// change what every later caller received.
var ft991aSettingsDescriptor = buildSettingsDescriptor(catDialect)

// buildSettingsDescriptor builds this radio's tree from dialect's EX
// inventory — the shared body, with this radio's own grouping from
// params: the FT-991A's manual prints ONE FLAT MENU LIST with no chart
// hierarchy at all, so yaesu.GroupFlat puts every item in a single menu
// holding a single group rather than inventing a partition the manual
// does not have, and the item Display is the wire address.
func buildSettingsDescriptor(dialect cat.Dialect) driver.SettingsDescriptor {
	return yaesu.BuildDescriptor(dialect, &params)
}

// SettingsDescriptor returns the FT-991A's radio-neutral settings
// descriptor: a defensive Clone() of the tree built once at package init.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ft991aSettingsDescriptor.Clone()
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (core/driver/optional.go): the
// driver-level, no-session-required counterpart to
// Session.SettingsDescriptor. Identical to it and to the package-level
// func: this driver's settings tree depends only on the static EX
// inventory, never on anything a live session discovers.
func (d *ft991aDriver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements the optional driver.SettingsReader
// capability on the concrete *Session, identical to the two above.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError is the refusal ReadSetting returns for an id that
// does not name a known FT-991A EX (MENU) address — refused BEFORE any
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
// reads one FT-991A EX (MENU) setting by its opaque, radio-neutral id,
// which this driver mints as the setting's EX wire address.
//
// It takes the session's operation mutex — pinned, not merely asserted,
// by TestReadSetting_HoldsOpMuAgainstAConcurrentWrite, which parks a read
// mid-operation through params.ReadGap and proves a concurrent
// WriteChannel cannot put a frame on the wire until this returns.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return yaesu.ReadSetting(ctx, s.eng, s.dialect, &params, id)
}

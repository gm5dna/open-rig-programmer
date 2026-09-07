// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

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
// ONE VERSION FOR BOTH SIBLINGS, because there is one menu surface: the D
// and the MP share an EX inventory, and the descriptor is built once from
// modelD's dialect for that reason. A different string from every other
// radio's, so a snapshot taken from one can never validate against
// another's.
const settingsDescriptorVersion = "ftdx101-ex@1"

// ftdx101SettingsDescriptor is built ONCE, at package init, from the EX
// inventory this package's dialect carries.
//
// Every getter returns a Clone() of this and never the value itself:
// nothing outside this file may hold a reference to the shared original,
// because a caller that mutated the tree it was handed would silently
// change what every later caller received.
var ftdx101SettingsDescriptor = buildSettingsDescriptor(modelD.dialect)

// buildSettingsDescriptor builds this radio's tree from dialect's EX
// inventory — the shared body, this radio's own grouping and version
// coming from params. Nothing about the FTdx101's own item count or menu
// partition is written into it: they are properties of the inventory,
// asserted in settings_test.go against the dialect.
func buildSettingsDescriptor(dialect cat.Dialect) driver.SettingsDescriptor {
	return yaesu.BuildDescriptor(dialect, &params)
}

// SettingsDescriptor returns the FTdx101's radio-neutral settings
// descriptor: a defensive Clone() of the tree built once at package init.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ftdx101SettingsDescriptor.Clone()
}

// StaticSettingsDescriptor implements the optional
// driver.StaticSettingsProvider capability (core/driver/optional.go): the
// driver-level, no-session-required counterpart to
// Session.SettingsDescriptor. Identical to it and to the package-level
// func, for both siblings alike: this driver's settings tree depends only
// on the static EX inventory the two share.
func (d *ftdx101Driver) StaticSettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// SettingsDescriptor implements the optional driver.SettingsReader
// capability on the concrete *Session. It deliberately does NOT consult
// s.dialect: both siblings' dialects carry the same EX inventory, so a
// per-session rebuild would allocate the whole tree to produce the
// identical answer.
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError is the refusal ReadSetting returns for an id that
// does not name a known FTdx101 EX (MENU) address — refused BEFORE any
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
// reads one FTdx101 EX (MENU) setting by its opaque, radio-neutral id,
// which this driver mints as the setting's 6-digit EX wire address.
//
// It goes through the session's OWN dialect, not modelD's, so an MP
// session reads the MP's addresses even though the descriptor above was
// built once from the D.
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	return yaesu.ReadSetting(ctx, s.eng, s.dialect, &params, id)
}

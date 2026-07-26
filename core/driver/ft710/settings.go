// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// settingsDescriptorVersion is minted HERE — the exact string this
// driver's SettingsDescriptor identifies itself with. Task 33's
// codeplug.MenuSnapshot.Descriptor carries this string through verbatim,
// so a snapshot can later be checked against the descriptor version that
// produced it.
const settingsDescriptorVersion = "ft710-ex@1"

// ft710SettingsDescriptor is built ONCE, at package init, from the M8a
// generated EX inventory (cat.EXItems) — see buildSettingsDescriptor.
// Every getter (the package-level SettingsDescriptor func and
// Session.SettingsDescriptor) returns a Clone() of this, never the value
// itself: nothing outside this file may ever hold a reference to the
// shared original.
var ft710SettingsDescriptor = buildSettingsDescriptor()

// buildSettingsDescriptor builds the FT-710's driver.SettingsDescriptor
// from cat.EXItems(): one SettingMenu per distinct P1 (ID the 2-digit
// decimal P1, e.g. "01"; Label the manual's P1Label), one SettingGroup per
// distinct (P1,P2) pair nested under its menu (ID the 4-digit P1P2, e.g.
// "0101"; Label the manual's P2Label), and one SettingItem per inventory
// row nested under its group, IN INVENTORY ORDER (ID the item's 6-digit
// wire address via EXAddress.Wire, Label the manual's Function name,
// Display the human "P1-P2-P3" form, e.g. "01-01-01").
//
// cat.EXItems already returns its 296 rows sorted by (P1,P2,P3) (see its
// doc comment), so a single linear pass — opening a new menu or group only
// when the running P1/P2 changes from the PREVIOUS item — reproduces Table
// 2's own grouping without any extra sorting or map bookkeeping. Each
// menu/group pointer used to append is re-derived fresh, by index, on
// every iteration (never carried across iterations): this sidesteps any
// question of whether an earlier append's slice growth could invalidate a
// held pointer, at the cost of one extra slice index per item — a
// non-issue at 296 items, run once at package init.
func buildSettingsDescriptor() driver.SettingsDescriptor {
	items := cat.FT710.EXItems()

	d := driver.SettingsDescriptor{Version: settingsDescriptorVersion}

	for _, it := range items {
		menuID := fmt.Sprintf("%02d", it.Addr.P1)
		groupID := fmt.Sprintf("%02d%02d", it.Addr.P1, it.Addr.P2)

		if len(d.Menus) == 0 || d.Menus[len(d.Menus)-1].ID != menuID {
			d.Menus = append(d.Menus, driver.SettingMenu{ID: menuID, Label: it.P1Label})
		}
		menu := &d.Menus[len(d.Menus)-1]

		if len(menu.Groups) == 0 || menu.Groups[len(menu.Groups)-1].ID != groupID {
			menu.Groups = append(menu.Groups, driver.SettingGroup{ID: groupID, Label: it.P2Label})
		}
		group := &menu.Groups[len(menu.Groups)-1]

		group.Items = append(group.Items, driver.SettingItem{
			ID:      it.Addr.Wire(),
			Label:   it.Name,
			Display: fmt.Sprintf("%02d-%02d-%02d", it.Addr.P1, it.Addr.P2, it.Addr.P3),
		})
	}

	return d
}

// SettingsDescriptor returns the FT-710's radio-neutral settings
// descriptor: a defensive Clone() of the package-level tree
// buildSettingsDescriptor built once at package init. Every call returns
// an independent copy — see driver.SettingsDescriptor.Clone's doc comment
// for why that independence is load-bearing.
func SettingsDescriptor() driver.SettingsDescriptor {
	return ft710SettingsDescriptor.Clone()
}

// SettingsDescriptor implements the optional driver.SettingsReader
// capability (see that interface's doc comment) on the concrete *Session.
// Identical to the package-level SettingsDescriptor func: the descriptor's
// shape depends only on the static EX inventory, never on anything Open
// discovered about a specific radio (unlike Capabilities, which folds in
// per-session 60m/EMG discovery).
func (s *Session) SettingsDescriptor() driver.SettingsDescriptor {
	return SettingsDescriptor()
}

// UnknownSettingError reports that ReadSetting's id argument does not name
// a known FT-710 EX (MENU) address — refused BEFORE any wire traffic,
// exactly like ReadChannel's malformed-slot refusal (read.go,
// cat.ParseSlot's error path).
type UnknownSettingError struct {
	// ID is the caller-supplied setting ID that did not parse.
	ID string
}

// Error implements the error interface.
func (e *UnknownSettingError) Error() string {
	return fmt.Sprintf("ft710: ReadSetting: %q is not a known FT-710 setting ID", e.ID)
}

// SettingAnswerMismatchError reports that an EX answer named a DIFFERENT
// wire address than the one just requested. Deliberately a distinct type
// from the existing slot-worded *AnswerMismatchError (read.go/ft710.go):
// that type's fields and message are worded for memory-channel slots
// ("requested slot ... but the answer names slot ..."), and EX addresses
// are a structurally different namespace (six-digit P1P2P3 triples, never
// a cat.Slot) — reusing it would blur two unrelated wire-address kinds
// under one error shape.
type SettingAnswerMismatchError struct {
	// Requested is the six-digit wire address the read asked for.
	Requested string
	// Answered is the six-digit wire address the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *SettingAnswerMismatchError) Error() string {
	return fmt.Sprintf("ft710: ReadSetting: requested EX address %q but the answer names address %q — refusing to map a reply onto the wrong setting", e.Requested, e.Answered)
}

// exSpec is the transport spec for an EX read of addr. ExpectPrefix MUST
// carry the FULL six-digit wire address, never the bare "EX" command name
// — see transport.CommandSpec.ExpectPrefix's own doc comment: EX shares
// its two-byte command prefix across all 296 Table 2 addresses, so a bare
// "EX" would let Engine.Do correlate a DIFFERENT address's still-in-flight
// answer (or an unsolicited push) as this read's own. ExpectLen is left
// at its zero value (variable length): the P4 body's width varies 1-12
// bytes across items (cat.EXItem.Digits/Text), so only the prefix is
// checked — mirroring mtSpec's own variable-length answer, ft710.go. One
// retry — reads are idempotent, exactly mrSpec/mtSpec's rationale.
func exSpec(addr cat.EXAddress) transport.CommandSpec {
	return transport.CommandSpec{ExpectPrefix: "EX" + addr.Wire(), RetryReads: 1}
}

// parseEXResponse interprets the outcome of one EX exchange for requested:
//   - a rejection frame (cat.IsRejection) maps to
//     SettingValue{ID: requested.Wire(), State: SettingUnavailable}, with
//     NO error — mirroring the project's established "?;" -> empty-result
//     rule (ReadChannel's empty-slot mapping, read.go);
//   - a well-formed EX answer naming requested's own address maps to
//     SettingKnown, with Raw the P4 body verbatim (cat.ParseEXAnswer's own
//     no-trim, no-typed-value policy — see its doc comment);
//   - a well-formed EX answer naming a DIFFERENT address is refused with
//     *SettingAnswerMismatchError;
//   - anything else (a malformed frame cat.ParseEXAnswer rejects) is a
//     plain wrapped error.
//
// This is a PURE function — no ctx, no *Session, no wire I/O — deliberately
// separated from ReadSetting's exchange so it can be (and is)
// unit-tested directly with hand-built frame values, rather than only
// through a session-level fault injection. That separation matters most
// for the wrong-address branch: transport.Engine.Do's own full-address
// correlation (exSpec's ExpectPrefix carries the complete six-digit
// address, not just "EX") means Do can only ever return a frame that
// ALREADY matches that prefix as a successful answer — any genuinely
// differently-addressed reply fails Do's own matching and is counted as
// an unexpected frame instead of being handed back as this read's answer.
// So the wrong-address branch is, BY DESIGN, unreachable through the real
// Session.ReadSetting path; calling this helper directly with a
// hand-built mismatched frame is the only way to exercise it, and to
// prove the defence-in-depth check actually works should that engine
// guarantee ever regress. ReadSetting reaches the OTHER two branches for
// real: a genuine rejection is reconstructed as the literal "?;" bytes
// (see ReadSetting) before being handed to this same function, so all
// three response-interpretation rules live in exactly one place.
func parseEXResponse(requested cat.EXAddress, frame []byte) (driver.SettingValue, error) {
	id := requested.Wire()

	if cat.IsRejection(frame) {
		return driver.SettingValue{ID: id, State: driver.SettingUnavailable}, nil
	}

	addr, raw, err := cat.FT710.ParseEXAnswer(frame)
	if err != nil {
		return driver.SettingValue{}, fmt.Errorf("ft710: ReadSetting %s: %w", id, err)
	}
	if addr.Wire() != id {
		return driver.SettingValue{}, &SettingAnswerMismatchError{Requested: id, Answered: addr.Wire()}
	}

	return driver.SettingValue{ID: id, Raw: raw, State: driver.SettingKnown}, nil
}

// rejectionFrameBytes is the literal "?;" NAK frame, reconstructed by
// ReadSetting from Engine.Do's cat.ErrRejected sentinel — see ReadSetting's
// doc comment for why.
var rejectionFrameBytes = []byte("?;")

// ReadSetting implements the optional driver.SettingsReader capability:
// reads one FT-710 EX (MENU) setting by its opaque, radio-neutral id
// (which the FT-710 mints as the setting's 6-digit EX wire address — see
// buildSettingsDescriptor).
//
// id is parsed via cat.ParseEXAddress FIRST, entirely before any wire
// traffic: a failure (malformed shape, or a syntactically well-formed
// address that is not a Table 2 member) returns *UnknownSettingError and
// nothing is ever sent — exactly ReadChannel's malformed-slot refusal
// shape (read.go).
//
// The whole exchange holds s.opMu for its full duration, mirroring
// ReadChannel's Fix-2 discipline (see Session's doc comment, ft710.go):
// even though a single EX read is only ONE transport.Engine.Do call
// (unlike ReadChannel's MR+MT pair), holding opMu here still serialises it
// against a concurrent ReadChannel/WriteChannel's own multi-command
// sequence, so a settings read can never interleave into the gap between
// another logical operation's Do calls.
//
// Rejection mechanism (the thing this task's brief left to the code, not
// prescribed): Engine.Do surfaces a "?;" reply as the cat.ErrRejected
// ERROR SENTINEL — detected here via errors.Is, exactly as ReadChannel
// detects it (read.go) — never as returned frame bytes; Do's own
// waitForAnswer (core/transport/engine.go) checks cat.IsRejection
// internally and converts a rejection straight into that sentinel before
// ever returning to its caller. ReadSetting reconstructs the canonical
// "?;" bytes from that sentinel and hands them to parseEXResponse exactly
// like a real answer frame, so that function remains the SINGLE place
// which interprets what a response means, for every outcome alike (see
// its own doc comment).
func (s *Session) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	addr, err := cat.FT710.ParseEXAddress(id)
	if err != nil {
		return driver.SettingValue{}, &UnknownSettingError{ID: id}
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	cmd, err := cat.FT710.BuildEXRead(addr)
	if err != nil {
		// Unreachable in practice: cat.ParseEXAddress above already
		// enforces the identical Table 2 membership BuildEXRead itself
		// checks (both via cat.KnownEXAddress) — kept as defence in
		// depth rather than a silent assumption.
		return driver.SettingValue{}, fmt.Errorf("ft710: ReadSetting %s: %w", addr.Wire(), err)
	}

	frame, err := s.eng.Do(ctx, cmd, exSpec(addr))
	switch {
	case errors.Is(err, cat.ErrRejected):
		frame = rejectionFrameBytes
	case err != nil:
		return driver.SettingValue{}, fmt.Errorf("ft710: ReadSetting %s: %w", addr.Wire(), err)
	}

	return parseEXResponse(addr, frame)
}

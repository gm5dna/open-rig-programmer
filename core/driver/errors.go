// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"errors"
	"fmt"
)

// ErrAnswerMismatch is the sentinel for the refusal every driver mints its
// own copy of: a memory answer whose decoded channel address is not the
// one that was asked for.
//
// IT CARRIES NO MODEL NAME, because it is the CLASS of failure, not one
// radio's instance of it. The model is on the error VALUE
// (AnswerMismatchError.Model), which is what a message shows and what
// errors.As recovers; the sentinel is what errors.Is answers "did a radio
// answer about the wrong channel?" with, across every driver at once.
// That question is a caller's, and before this sentinel existed a caller
// had to know all seventeen driver packages to ask it.
var ErrAnswerMismatch = errors.New("driver: a memory answer named a channel other than the one requested")

// AnswerMismatchError reports that a memory answer's decoded channel
// address was not the one asked for, naming both.
//
// THE CHECK IS THE DRIVER'S BECAUSE NOTHING BELOW IT MAKES ONE. The
// transports' answer matchers are envelope-only by design — Icom's
// civ.Profile.MemoryAnswerMatcher checks to/from/cn/sc and not the
// channel, and a NEWCAT prefix matcher checks the command name — so an
// answer for channel 7 satisfies the spec for a read of channel 3
// perfectly well. A record silently mis-attributed to the wrong channel
// is the corruption this whole project refuses, so the comparison is made
// BEFORE ANY USE of the answer: before empty recognition, before caching,
// before record mapping and before a write merge.
//
// T is the address type, so a driver that names slots with strings and one
// that names channels with integers share the type rather than each
// declaring their own; comparable is all the equality test needs.
type AnswerMismatchError[T comparable] struct {
	// Model is the radio the refusal came from, e.g. "ic7300".
	Model string
	// Requested is the address the read asked for.
	Requested T
	// Answered is the address the reply actually named.
	Answered T
}

// Error implements the error interface.
func (e *AnswerMismatchError[T]) Error() string {
	return fmt.Sprintf("%s: requested %v but the answer names %v — refusing to map a reply onto the wrong slot",
		e.Model, e.Requested, e.Answered)
}

// Is lets errors.Is(err, ErrAnswerMismatch) match. It is preferred to
// Unwrap here because the generic type has no single wrapped value to
// return and a caller's question is about the class, not a chain.
func (e *AnswerMismatchError[T]) Is(target error) bool { return target == ErrAnswerMismatch }

// UnknownSettingError reports that ReadSetting's id argument does not name
// a setting this radio has — refused BEFORE any wire traffic, exactly like
// ReadChannel's malformed-slot refusal.
type UnknownSettingError struct {
	// Model is the radio the refusal came from, e.g. "ft710".
	Model string
	// ID is the caller-supplied setting ID that did not parse.
	ID string
}

// Error implements the error interface.
func (e *UnknownSettingError) Error() string {
	return fmt.Sprintf("%s: ReadSetting: %q is not a known %s setting ID", e.Model, e.ID, e.Model)
}

// SettingAnswerMismatchError reports that a settings answer named a
// DIFFERENT wire address than the one just requested.
//
// Deliberately a distinct type from the slot-worded AnswerMismatchError,
// and NOT a T=string instance of it: that type's message is worded for
// memory-channel slots, and a settings address is a structurally different
// namespace (a NEWCAT EX triple, never a slot). Blurring the two under one
// error shape would leave a caller unable to tell which kind of wire
// address the radio got wrong.
//
// It carries NO errors.Is sentinel, where the slot-worded type does: a
// caller asking "did the radio answer about the wrong CHANNEL?" has a real
// errors.Is question put to a branch a live read reaches, whereas this one
// is reached only through a driver's defence in depth — full-address
// correlation in the engine makes it unreachable on the real path — so
// errors.As on the concrete type is the whole interface it needs.
type SettingAnswerMismatchError struct {
	// Model is the radio the refusal came from, e.g. "ftdx10".
	Model string
	// Requested is the wire address the read asked for.
	Requested string
	// Answered is the wire address the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *SettingAnswerMismatchError) Error() string {
	return fmt.Sprintf("%s: ReadSetting: requested EX address %q but the answer names address %q — refusing to map a reply onto the wrong setting",
		e.Model, e.Requested, e.Answered)
}

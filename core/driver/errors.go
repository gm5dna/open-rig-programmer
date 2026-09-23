// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/spec"
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

// ErrRecordDecode is the sentinel a driver's ReadChannel wraps a genuine
// record-decode failure with: a reply that arrived intact at the
// transport level (no timeout, no framing/context error) but could not be
// decoded into a channel record — a parse failure, a length/fingerprint
// mismatch, or the like. It is deliberately narrow: a driver wraps ONLY
// the call that turns received wire bytes into a structured record
// (e.g. a dialect/profile's ParseXXAnswer, or a bincat record decode) —
// never a transport-layer error (core/clone/read.go's readAll classifies
// those as fatal, same as today), and never a semantic/mapping failure
// after a successful decode (an unrecognised mode byte, an out-of-range
// value) — those stay fatal too, since the record itself decoded fine.
//
// core/clone/read.go's readAll checks this alongside ErrAnswerMismatch:
// a slot whose read fails this way is recorded in
// codeplug.RadioInfo.FailedSlots rather than aborting the whole read, so
// one corrupted reply does not discard every other slot.
var ErrRecordDecode = errors.New("driver: a reply could not be decoded into a channel record")

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

// ErrUnknownSlot is the sentinel for the refusal every Kenwood ts* driver
// mints its own copy of: a slot identifier that names no channel the row
// publishes. Model-neutral like ErrAnswerMismatch — the model lives on the
// error VALUE (UnknownSlotError.Model), not the class.
var ErrUnknownSlot = errors.New("driver: slot is not one this row/model publishes")

// UnknownSlotError reports a slot identifier that is not in this row's or
// model's published set — malformed or outside the printed space. No frame
// is sent.
//
// Seven Kenwood packages (ts2000, ts480, ts570, ts590, ts870s, ts890,
// ts990) minted this same three-field shape (two of them, ts890 and
// ts990, leaving Model unset and splicing a package constant into the
// message instead — this shared type gives every package all three
// fields so none has to). Their Error() wording differs by package (a
// literal prefix, and ts870s's shorter template), so each package keeps
// its own Error() by embedding this struct rather than sharing one
// method — merging the message would change byte-identical wire-facing
// text for no reason.
type UnknownSlotError struct {
	// Slot is the identifier that was requested.
	Slot string
	// Model is the row/model it was requested of, e.g. "TS-2000". Empty
	// where the owning package instead splices its own constant into the
	// message (ts890, ts990).
	Model string
	// Reason says how it failed.
	Reason string
}

// ErrOutOfDomain is the sentinel for the refusal every Icom driver mints
// its own copy of: a Known value lies outside what this radio's record
// (or declared capability domain) can encode.
var ErrOutOfDomain = errors.New("driver: a Known value lies outside what this radio's record can encode")

// OutOfDomainError reports a field value outside what a radio's record or
// declared domain can encode. Eight Icom packages (ic7200, ic7410,
// ic7600, ic7610, ic7700, ic7760, ic7800, ic7851) minted this shape in
// two field widths — {Field,Value,Max} and {Field,Value,Min,Max,Where} —
// and two message templates, one plain and one adding a gate-domain
// note; this type unions the fields (Min/Where empty where a package
// never had them) and each package keeps its own Error() by embedding,
// since the template differs by package.
type OutOfDomainError struct {
	// Field is the neutral field whose value was refused.
	Field spec.Field
	// Value is what was asked for on a write, or what was decoded on a
	// read, in the field's own neutral unit.
	Value uint64
	// Max is the largest value this radio's record (or declared domain)
	// can encode for it.
	Max uint64
	// Min is the declared lower bound, where a package has one. Zero
	// (and unused) for packages that only ever had a ceiling.
	Min uint64
	// Where names the capability the bounds came from, for the message.
	// Empty for packages that never had one.
	Where string
}

// ErrUnmappedRegion is the sentinel for the refusal every Icom driver
// mints its own copy of: a slot's unmapped record regions differ from
// this profile's Fixed template (the E6-style refusal).
var ErrUnmappedRegion = errors.New("driver: the slot's unmapped record regions differ from this profile's Fixed template")

// UnmappedRegionError reports that a slot's unmapped record regions do
// not match this profile's Fixed template. Eight Icom packages (ic7200,
// ic7410, ic7600, ic7610, ic7700, ic7760, ic7800, ic7851) minted this
// shape; two (ic7200, ic7410) check whole bytes and have no Nibble field,
// the other six check nibble-granularity and add one. This type unions
// them (Nibble empty for the whole-byte two); each package keeps its own
// Error() by embedding, since wording (and the reasons switch) differs by
// package.
type UnmappedRegionError struct {
	// Offset is the 0-based record byte whose unmapped region differed.
	Offset int
	// Nibble is "low", "high" or "whole" — which part of that byte, for
	// the six packages that check at nibble granularity. Empty for
	// ic7200 and ic7410, which check the whole byte and never set it.
	Nibble string
	// Want and Got are that region's value in the template and in the
	// slot's actual record.
	Want, Got byte
}

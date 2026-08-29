// SPDX-License-Identifier: GPL-3.0-or-later

package civ

// ProfileConfig is the input to NewProfile: everything that varies between
// Icom radios sharing the CI-V grammar, as plain data.
//
// A FLAT STRUCT rather than functional options, deliberately, and for
// core/cat's DialectConfig reason: a Profile carries DATA, not behaviour,
// and a flat config can be validated exhaustively in one place. "Is every
// required field set and mutually consistent?" is a question this shape
// can answer and a half-applied set of options cannot.
//
// Every field a per-model package fills is ASSUMED until a named hardware
// lift says otherwise; the per-model registers carry that, and doc.go
// carries the conventions this package itself assumes.
type ProfileConfig struct {
	// Model is the radio this profile describes, for diagnostics — e.g.
	// "IC-7300". It is also this package's Configured() witness, which is
	// why it may not be empty: an unlabelled profile in a log line is a
	// profile nobody can attribute a wrong byte to.
	Model string

	// RadioAddress is the model's DEFAULT CI-V address, the `to` byte of
	// every frame this program sends it and the `from` byte of every frame
	// it accepts back.
	//
	// A user who has moved their radio off its default address is
	// unreachable this tier — spec D3.3 makes that a documented
	// limitation, not a bug.
	RadioAddress byte

	// ControllerAddress is the address this program answers to. Zero
	// selects ControllerAddressDefault (0xE0), the CI-V convention.
	//
	// Zero is a safe "omitted" marker because 0x00 is the BROADCAST
	// address and can never be a controller's own.
	ControllerAddress byte

	// MaxFrame is the longest frame this radio's accumulator will
	// assemble. Zero selects DefaultMaxFrame. It must be large enough for
	// the frames this very profile builds, which validation checks — a
	// profile whose own memory set exceeds its own frame bound could never
	// read back what it wrote.
	MaxFrame int

	// AddressForm is how this model addresses a memory channel. No
	// default: see AddressForm.
	AddressForm AddressForm

	// Groups is the COUNT of groups (or bands) under a grouped address
	// form; valid indices are GroupBase..GroupBase+Groups-1. Must be 0
	// under AddressFormFlat.
	//
	// A COUNT rather than a ceiling, so that the count and the base are
	// separate statements: how many groups a radio has, and what it calls
	// the first one, are different facts and a model can surprise you on
	// either.
	Groups int

	// GroupBase is the index the radio itself gives its FIRST group or
	// band — the WIRE index, what it prints and what it sends. Zero is
	// the default and is what every profile written before the Icom tier
	// meant; the IC-9700 numbers its three groups 1, 2 and 3 and declares
	// GroupBase 1.
	//
	// IT IS PROFILE DATA, NOT FORM DATA, because one address form serves
	// radios that disagree about it: the 9700's groups start at 1 while
	// the IC-705's and IC-905's start at 0 and run to a CALL group the
	// radio numbers 100. What the FORM declares is the width of its index
	// field (AddressForm.groupCapacity), and validation joins the two —
	// GroupBase + Groups − 1 must be an index that width can carry.
	//
	// Must be 0 under AddressFormFlat, which has no index field at all:
	// an inapplicable field is explicitly zero rather than ignored, on
	// this config's standing rule.
	GroupBase int

	// ChannelLo and ChannelHi are the inclusive channel range, per group
	// where the form is grouped.
	ChannelLo, ChannelHi int

	// ExtraRanges extends the base rectangle as an exact UNION. Keeping
	// each range separate prevents admission of holes in their closure;
	// TestExtraRangesAreAUnionNotARectangularClosure pins all three callers.
	ExtraRanges []AddressRange

	// NameLength is the width of this model's channel-name field in
	// bytes, or 0 for a model with no name field.
	NameLength int

	// NameCharset is every byte a name may carry. Order is irrelevant;
	// duplicates are refused as a transcription error.
	//
	// It is PROFILE data rather than a package constant because the
	// charsets differ per model, and spec D5 entry 3 — the name pad byte
	// and space handling — records that every model's printed charset
	// table OMITS the space while the radios plainly accept one. So this
	// is a transcription each model's evidence leg settles, and a shared
	// table would impose one model's reading on all six.
	NameCharset string

	// NamePad is the byte a short name is padded to NameLength with, and
	// the byte trimmed from the end of a name read back. 0x20 is the
	// ASSUMED value for every model in this tier (spec D5 entry 3, the
	// name pad byte and space handling).
	//
	// It may legitimately be a member of NameCharset — on most models the
	// pad byte IS the space, which is also a legal name character. The
	// consequence is stated rather than hidden: a name ENDING in the pad
	// byte does not round-trip, because padding erases the
	// data-versus-fill distinction on the wire.
	NamePad byte

	// Layouts is one entry per accepted record SHAPE: per length for the
	// two length-keyed kinds, per ModeClass for DiscriminatorModeByte
	// (where two layouts may share a length). See RecordLayout.
	Layouts []RecordLayout

	// Discriminator names the rule picking among the accepted lengths.
	Discriminator RecordDiscriminator

	// ModeKey names the record byte or nibble which selects a layout under
	// DiscriminatorModeByte. It is zero for the two length discriminators.
	ModeKey FieldSpan

	// BuildLength is the accepted length BuildMemorySet EMITS. It must be
	// one of the layouts' lengths.
	//
	// It is declared rather than derived because a multi-length model has
	// no obvious "the" length: the IC-905 accepts two, and which one this
	// program writes is a decision its evidence leg makes, not one this
	// package should take by picking the longest or the first.
	BuildLength int
}

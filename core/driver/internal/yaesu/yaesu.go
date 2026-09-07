// SPDX-License-Identifier: GPL-3.0-or-later

// Package yaesu holds the bodies the Yaesu NEWCAT driver packages used to
// carry a copy of each: the EX (menu) settings surface, the memory write
// path, and the Open handshake. The five packages differed in a handful
// of lines apiece — a name, a legend, one extra refusal — so what varies
// lives in a Params value per radio and the body lives here once.
//
// A STRUCT AND FUNCS, NOT AN INTERFACE. Each driver package keeps its own
// Driver and Session types and calls into these functions; nothing here
// knows what a Session is. That is what lets the optional-capability
// implementations (DiscoveredBankSynthesizer, SettingsReader, …) and each
// package's own error sentinels stay where callers already find them.
//
// A nil hook field means "not this radio", never "use a default that
// happens to be wrong": Params.Probe nil is a radio with no 5xx/EMG
// discovery, Params.RefuseTxClar empty is a radio whose TX clarifier is
// writable. The two nil-means-the-majority-form fields, Group and
// Display, are documented as such on the fields themselves.
package yaesu

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Params is one radio's configuration of the shared bodies. One value per
// radio, package-level in the driver package that owns it — the FTdx101
// declares two, one per sibling, because its two models differ in dialect
// and CAT ID while sharing everything else.
type Params struct {
	// Name is the error prefix every message this package mints carries,
	// which is the driver package's own name, e.g. "ftdx10".
	Name string
	// Model is the human model name, e.g. "FTdx10" — what the settings
	// errors name the radio as. For the FTdx101 it is the PACKAGE's name
	// ("FTdx101"), not a sibling's, matching what those errors said
	// before the fold.
	Model string
	// Dialect is the dialect this radio's static tables are built from
	// (the settings descriptor). Per-session code passes the session's
	// own dialect explicitly instead, so the FTdx101's two siblings can
	// share one Params.
	Dialect cat.Dialect
	// CATID is the ID; answer this radio's Open accepts.
	CATID string

	// WantModel is WrongRadioError.WantModel. EMPTY keeps the ID-only
	// refusal text (the FTdx10's and the FT-710's).
	WantModel string
	// GotModelOf names the model a foreign CAT ID belongs to, for the
	// FTdx101's two siblings. nil means the driver cannot name what it
	// found, and GotModel is left empty.
	GotModelOf func(catID string) string
	// Probe selects the discovery read Open's 5xx/EMG sweep uses. THE
	// ZERO VALUE IS NoProbe — a radio with no such inventory to discover
	// (the FT-991A) needs no field set, and cannot get a sweep by
	// accident.
	Probe ProbeKind
	// MRAnswerLen is the exact MR answer length in bytes, for ProbeMR.
	MRAnswerLen int
	// MTRetries is the retry count on an MT read spec: 1 where the MT
	// read is the driver's own idempotent probe, 0 where the dialect's
	// MT answer is only ever solicited once.
	MTRetries int

	// DescriptorVersion is the settings descriptor's own version string,
	// minted by the driver package, e.g. "ftdx10-ex@1".
	DescriptorVersion string
	// Group partitions the EX inventory into menus and groups. nil is
	// the majority form: one menu per P1 labelled from the manual's P1
	// column, one group per (P1,P2) labelled from its P2 column.
	Group func(cat.EXItem) (menuID, menuLabel, groupID, groupLabel string)
	// Display renders an item's human-facing position. nil is the
	// dialect's own EX wire address.
	Display func(cat.EXAddress) string
	// ReadGap, when set, is called between a settings read's exchange and
	// its interpretation. A test seam for the FT-991A's concurrency
	// proof; nil everywhere in production.
	ReadGap func()

	// CTCSS is the CTCSS state vocabulary this radio's write path
	// accepts, IN LEGEND ORDER — the refusal text names the states in
	// this order, so the slice is both the lookup and the legend.
	CTCSS []CTCSSName
	// EraseReason is the refusal text for a write of an empty channel.
	EraseReason string
	// BuildMT builds the Set frame. display carries the tag-display flag
	// for the radios whose combined record has one; the radios whose
	// record has none ignore it.
	BuildMT func(dialect cat.Dialect, data cat.MemoryData, tag string, display bool) (cat.Command, error)
	// RefuseTagDisplayUnknown, when non-empty, is the refusal text for a
	// tag display that is not codeplug.Known — the FT-891's live-tag
	// flag, which has no "leave it alone" encoding.
	RefuseTagDisplayUnknown string
	// RefuseTxClar, when non-empty, is the refusal text for a set TX
	// clarifier flag — the FT-891, whose record has no such flag.
	RefuseTxClar string
	// RefuseModeUnset, when non-empty, is the refusal text for
	// cat.ModeUnset reaching a Set frame.
	RefuseModeUnset string
	// CheckFreqRange makes the write path refuse a frequency outside the
	// session's own capability range.
	CheckFreqRange bool
}

// CTCSSName is one entry of a radio's CTCSS state vocabulary.
type CTCSSName struct {
	Name  string
	State cat.CTCSSState
}

// ProbeKind names the read Open's 5xx/EMG discovery sweep uses to ask
// whether one slot is populated.
type ProbeKind int

const (
	// NoProbe is the zero value: this radio has no discoverable 5xx/EMG
	// inventory, and Open runs no sweep at all.
	NoProbe ProbeKind = iota
	// ProbeMT discovers with an MT (combined memory) read.
	ProbeMT
	// ProbeMR discovers with an MR (memory record) read.
	ProbeMR
)

// IDSpec is the transport spec for the ID; probe every one of these
// radios opens with: a 7-byte answer, one retry, the probe being
// idempotent.
func IDSpec() transport.CommandSpec {
	return transport.CATReadSpec("ID", 7, 1)
}

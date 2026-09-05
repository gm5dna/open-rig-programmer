// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import "github.com/gm5dna/open-rig-programmer/core/spec"

// This file names the seam's OPTIONAL capabilities: shapes a
// driver.Session or driver.Driver CONCRETE type may implement, never
// added to the Session or Driver interfaces themselves. The precedent is
// already established twice over — driver.SettingsReader (settings.go)
// and core/clone/memory_selector.go's MemorySelector — and every doc
// comment below restates why: a radio (or a future driver package) whose
// protocol has no concept of, say, a settings surface or a discoverable
// regional bank must never be forced to implement one just to satisfy a
// mandatory interface method. A caller that wants one of these
// capabilities performs a plain type assertion — sess.(driver.RegionReporter),
// drv.(driver.StaticSettingsProvider) — against the concrete value a
// driver's Open (or the driver value itself) returned.
//
// core/driver/ft710 implements every capability named here. That was a
// fact about THIS task (37) and never a promise, and M9c-6 proved the
// point: core/driver/ftdx10 — the second driver, registered that milestone
// — implements StaticSettingsProvider and DiscoveredBankSynthesizer but
// deliberately NOT RegionReporter, because no honest FTdx10 region
// vocabulary exists (that driver's doc.go says why at length). "Optional"
// is therefore load-bearing rather than theoretical: every caller
// consulting one of these interfaces must be written to tolerate "not
// implemented" (the two-result type assertion), and at least one of them is
// now genuinely absent at runtime.

// RegionReporter is an OPTIONAL capability a driver.Session's CONCRETE
// type may implement: report the regulatory region session discovery
// implied. Deliberately NOT added to driver.Session itself — the FT-710's
// region derivation from its discovered 60 m/EMG inventory
// (core/driver/ft710.deriveRegion) is a discovery quirk specific to
// drivers whose radio exposes enough on-air structure to derive a region
// at all, not a seam-level contract every future driver must implement.
//
// core/driver/ft710.Session's existing Region() method (ft710.go, from
// before this task) already has exactly this shape — task 37 adds no new
// method there, only this interface naming the shape, plus a
// compile-time assertion proving the two agree.
type RegionReporter interface {
	// Region reports the regulatory region session discovery implied, in
	// whatever vocabulary the concrete driver defines (the FT-710's is
	// "UK", "US", "no-60m", or "unknown-N"/"unknown-16plus" — see
	// core/driver/ft710.Session.Region's own doc comment). Callers record
	// it in codeplug.RadioInfo.Region.
	Region() string
}

// DiagnosticsReporter is an OPTIONAL capability a driver.Session's
// CONCRETE type may implement: report transport-level health counters as
// a point-in-time snapshot. Deliberately NOT added to driver.Session
// itself, for the same reason RegionReporter and SettingsReader are not:
// which diagnostics exist — or whether a driver's transport exposes any
// counters worth surfacing at all — is a per-driver matter.
//
// core/driver/ft710.Session's existing Diagnostics() method (ft710.go,
// from before this task) already has exactly this shape — task 37 adds
// no new method there, only this interface naming the shape (and
// SessionDiagnostics itself, moved here from core/driver/ft710 — see its
// own doc comment below), plus a compile-time assertion proving the two
// agree.
type DiagnosticsReporter interface {
	// Diagnostics reports this session's transport-level health counters
	// as a point-in-time snapshot.
	Diagnostics() SessionDiagnostics
}

// SessionDiagnostics is a snapshot of transport-level health counters a
// session may report via the optional DiagnosticsReporter capability.
// Moved here from core/driver/ft710 (task 37, the M9a radio-neutral core
// refactor): the SHAPE was never FT-710-specific — only the driver
// backing it was — so it belongs on the neutral seam, not inside one
// driver's package. core/driver/ft710 keeps its exported name,
// ft710.SessionDiagnostics, as a type alias to this type (see ft710.go),
// so no existing caller of that name breaks.
//
// A struct — not a bare counter return — so future counters (e.g. retry
// totals, quarantine drains) can be added without another signature
// change.
type SessionDiagnostics struct {
	// UnexpectedFrames counts frames a session's underlying transport
	// received that did not match the expectation in force at the time
	// (core/driver/ft710's Session.Diagnostics wraps its
	// transport.Engine.UnexpectedFrames counter). Nonzero on an
	// otherwise-working session is a wire-health signal worth surfacing
	// to a user: another application may be sharing the port, or replies
	// may be arriving late enough to miss their windows. The field's
	// MEANING here is generic ("frames that did not match"); the
	// counting mechanism behind it is each driver's own.
	UnexpectedFrames uint64
}

// SerialFramingReporter is an OPTIONAL capability a driver.Driver's
// CONCRETE type may implement: state how many STOP BITS this radio's
// control port expects, so the composition root can open the port the way
// the radio's protocol actually frames a byte.
//
// IT IS ON THE DRIVER, NOT THE SESSION, AND THAT IS FORCED. The stop bits
// are a property of the port, chosen when the port is opened; the session
// is what a driver's Open returns once the port already exists. A
// Session-side reporter could only ever be consulted after the framing had
// already been guessed, which is to say never usefully. internal/wiring
// holds the driver value before it opens anything, and that is the one
// moment this question can be asked at.
//
// Deliberately NOT added to driver.Driver itself, and deliberately NOT a
// spec.Capabilities field. The M9c-5 (E2) rule stands, upheld at M9c-6
// and restated by spec D3.1: a Capabilities framing field is added only
// with HARDWARE EVIDENCE, and none of the five Yaesu models has any — the
// FTdx10's CAT manual makes no framing statement anywhere, so 8-N-2 for it
// is an ASSUMED register entry with a named lift rather than a fact. A
// driver that has nothing honest to say implements nothing, and
// internal/wiring opens its port at transport.DefaultStopBits exactly as
// before. All five registered Yaesu drivers are in that position today.
//
// THE ICOM SIDE IS WHY IT EXISTS (spec D3.1): every Icom driver in the
// tier reports 1, as its own ASSUMED register entry with its own named
// per-model lift. The warning spec D3.1 carries goes with it — Icom
// manuals print "8 bit / 1 stop" lines about the DATA/RTTY port, which is
// NOT the CI-V port and is NOT evidence for CI-V framing.
type SerialFramingReporter interface {
	// StopBits is how many stop bits this radio's control port expects:
	// 1 or 2, and nothing else. There is NO "unset" value — a driver
	// that implements this interface is making a statement, and
	// internal/wiring REFUSES any other number rather than substituting
	// a default, because a zero silently becoming 8-N-2 would put a
	// guess on the wire wearing a driver's authority.
	StopBits() int
}

// StaticSettingsProvider is an OPTIONAL capability a driver.Driver's
// CONCRETE type may implement: expose the settings/menu surface's STATIC
// baseline shape — the same two-level SettingsDescriptor tree
// SettingsReader.SettingsDescriptor (settings.go) returns per session,
// but callable BEFORE any session exists, mirroring Driver.Capabilities'
// own static-baseline framing (see driver.go's Driver.Capabilities doc
// comment: "what the radio model can do before any radio has been
// probed"). Deliberately NOT added to driver.Driver itself, for the same
// reason SettingsReader is not added to driver.Session: a radio whose
// protocol has no settings concept at all must never be forced to
// implement this.
type StaticSettingsProvider interface {
	// StaticSettingsDescriptor returns the driver's static settings tree
	// — a defensive copy (SettingsDescriptor.Clone), independent of any
	// session. For a driver whose descriptor shape never varies by
	// session (as core/driver/ft710's does not — its EX inventory is
	// fixed, not discovered), this and a session's own
	// SettingsReader.SettingsDescriptor return equal trees; a future
	// driver whose settings surface DID vary per session would be free
	// to disagree between the two.
	StaticSettingsDescriptor() SettingsDescriptor
}

// DiscoveredBankSynthesizer is an OPTIONAL capability a driver.Driver's
// CONCRETE type may implement: classify an OFFLINE slot list — e.g. a
// working codeplug loaded from an earlier read, with no live session at
// all — into the same read-only banks a live session's Open would have
// discovered for a radio whose inventory looks like that. Deliberately
// NOT added to driver.Driver itself, for the same reason
// StaticSettingsProvider is not: whether a radio even has a
// region-dependent, only-discoverable-per-session bank at all (the
// FT-710's 60 m/EMG) is a per-driver matter, and a driver with no such
// concept has nothing to synthesise.
//
// Introduced (task 37) for the offline UI synthesis app/uispec.go's
// synthesiseDiscoveredBanks once performed by hand for the FT-710,
// duplicating knowledge core/driver/ft710.effectiveCapabilities also
// encodes. Task 41 migrated that call site onto this interface (through
// wiring.SynthesiseDiscoveredBanks), so the hand-rolled classification is
// gone: the app now synthesises offline banks through the very driver code
// live discovery uses, and the two can no longer drift.
type DiscoveredBankSynthesizer interface {
	// SynthesiseDiscoveredBanks classifies slots — claimed by NONE of
	// this driver's own Capabilities().Banks — into the read-only banks
	// a live session would have discovered for a radio whose inventory
	// contains them: the same label, ID, Slots (preserving slots' INPUT
	// order), NoBlank, and field-support values a live
	// Session.Capabilities() would carry for the equivalent discovery.
	// Slots that are neither claimed by a static bank nor classifiable
	// into a discovered one — a malformed or unrecognised wire form —
	// are OMITTED, never guessed into a bank.
	SynthesiseDiscoveredBanks(slots []string) []spec.Bank
}

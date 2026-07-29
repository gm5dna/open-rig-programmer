// SPDX-License-Identifier: GPL-3.0-or-later

// Package wiring holds the session-construction plumbing shared by every
// composition root in this repository (cmd/rigprog, app/) — extracted
// from cmd/rigprog/wiring.go by task-15 so the GUI (app/) has the exact
// same registry/driver/session wiring the CLI already proved, rather
// than a second, independently-drifting copy.
//
// The deliberate structural-exclusivity shape that constrained
// cmd/rigprog/wiring.go (task-11 brief §3) is preserved EXACTLY here:
// two fully self-contained constructors — OpenRealSessionFor (this file)
// and OpenFakeSessionFor (fake.go) — with no shared helper accepting a
// profile alongside a port. That absence is the point: it keeps the
// invalid RealHardware/fakeradio or Simulated/real-port pairings
// structurally unrepresentable in the code shape, not merely unreached.
// ft710.Simulated is referenced in exactly ONE non-test .go file
// repo-wide — fake.go — pinned by internal/guards'
// TestSimulatedProfileTokensConfinement (extended by task-15 to be
// repo-wide rather than cmd/rigprog-local; folded from the single-driver
// guard task-15 extended into this data-driven guard at Task 58).
//
// Task 39 (the M9a radio-neutral core refactor) generalised this package
// to model-keyed dispatch: OpenRealSessionFor (this file) and
// OpenFakeSessionFor (fake.go) are the two fully self-contained,
// model-keyed constructors carrying the structural-exclusivity shape
// above. They were joined, briefly, by two DefaultModel-only compatibility
// wrappers (OpenRealSession/OpenFakeSession) so every caller outside this
// package could compile unchanged; Tasks 40 (cmd/rigprog) and 41 (app/)
// migrated those callers onto the -For functions, and — once neither had
// any reference left, confirmed by grep — task 41 deleted the wrappers and
// their UnexpectedSessionTypeError/UnexpectedFakeSessionTypeError types.
package wiring

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// DefaultModel names the driver.Registry key the model-keyed lookups below
// use when a caller has not (or cannot yet) name a model explicitly —
// every composition root (cmd/rigprog, app/) keys off this constant
// explicitly today, since neither yet has a model-selection surface. It
// stays exactly "FT-710" until a caller actually asks for a second model.
const DefaultModel = "FT-710"

// realDrivers is the model-keyed table of real-hardware driver
// constructors: model name -> a constructor building THAT model's
// real-profile driver.Driver. It is the single source of truth
// SupportedModels, OpenRealSessionFor, StaticCapabilities,
// StaticSettingsDescriptor, and SynthesiseDiscoveredBanks all key off —
// adding a second radio model to this package means adding one entry
// here (plus fake.go's own table for the simulated/demo path), never
// touching the functions themselves.
var realDrivers = map[string]func() driver.Driver{
	DefaultModel: NewRealDriver,
}

// SupportedModels returns every model name this package can open a real
// session against, sorted, so a caller (a CLI listing supported radios, a
// GUI picker) gets deterministic output.
func SupportedModels() []string {
	models := make([]string, 0, len(realDrivers))
	for m := range realDrivers {
		models = append(models, m)
	}
	sort.Strings(models)
	return models
}

// UnknownModelError is returned by every model-keyed lookup in this
// package (OpenRealSessionFor, OpenFakeSessionFor, StaticCapabilities,
// StaticSettingsDescriptor) when model names no registered driver.
// SynthesiseDiscoveredBanks, whose signature carries no error return,
// reports the equivalent condition as its bool false instead.
type UnknownModelError struct {
	// Model is the unrecognised model name the caller asked for.
	Model string
	// Supported is the full list of model names this package DOES
	// support at the time of the failed lookup (SupportedModels()'s own
	// sorted output), so the error message can name what a caller
	// should have asked for instead.
	Supported []string
}

// Error implements the error interface.
func (e *UnknownModelError) Error() string {
	return fmt.Sprintf("wiring: unknown model %q (supported: %s)", e.Model, strings.Join(e.Supported, ", "))
}

// realDriverFor looks model up in realDrivers and constructs its driver,
// or fails with *UnknownModelError. It is the shared entry point every
// model-keyed real-driver lookup in this file (OpenRealSessionFor,
// StaticCapabilities, StaticSettingsDescriptor, SynthesiseDiscoveredBanks)
// goes through, so "which models this package supports" has exactly one
// answer.
func realDriverFor(model string) (driver.Driver, error) {
	ctor, ok := realDrivers[model]
	if !ok {
		return nil, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}
	return ctor(), nil
}

// RegisterDriverError is NewRegistry's typed failure when
// driver.Registry.Register itself refuses d (e.g. a duplicate Model()).
// Error() carries this package's own generic "wiring: ..." wording (this
// package is shared by cmd/rigprog and app/, neither of which this
// package should assume any wording preference for); a caller wanting a
// DIFFERENT wording — e.g. cmd/rigprog's own pre-extraction "cmd/rigprog:
// register driver: ..." text (Fix 7, adjudicated LOW, Codex M6 #7) —
// should errors.As against this and Cause rather than relying on
// Error()'s text (the same pattern internal/csvmerge's
// InventoryMismatchError/UnknownSlotsError use for the identical reason
// — see cmd/rigprog/import.go's mergeCSV/mergeCHIRP aliases).
type RegisterDriverError struct{ Cause error }

func (e *RegisterDriverError) Error() string {
	return fmt.Sprintf("wiring: register driver: %v", e.Cause)
}

func (e *RegisterDriverError) Unwrap() error { return e.Cause }

// NewRegistry builds a fresh driver.Registry containing exactly one
// driver, d, registered under its own Model(). It is shared by both
// wiring constructors (this file's OpenRealSessionFor and fake.go's
// OpenFakeSessionFor); deliberately, it accepts only a driver.Driver —
// never an ft710.Profile alongside a port — so it cannot become the
// "profile + port" seam the structural-exclusivity constraint (task-11
// brief §3) rules out. Whatever pairing a caller wants must already be
// baked into d before this function ever sees it.
func NewRegistry(d driver.Driver) (*driver.Registry, error) {
	reg := driver.NewRegistry()
	if err := reg.Register(d); err != nil {
		return nil, &RegisterDriverError{Cause: err}
	}
	return reg, nil
}

// NewRealDriver builds the ft710 driver for a real-hardware session:
// profile ft710.RealHardware, the zero value. It is split out from
// OpenRealSessionFor so the capability set it implies — post-M5b-flip,
// write-capable for EXACTLY the six hardware-verified fields and
// nothing else (ft710.CapabilitiesRealHardware; before the flip,
// nothing writable at all) — can be pinned by a unit test that never
// opens a serial port (see TestNewRealDriver_HWVerifiedWriteSet).
func NewRealDriver() driver.Driver {
	return ft710.New(ft710.RealHardware)
}

// openSerial is OpenRealSessionFor's test seam: production code always
// leaves this at transport.OpenSerial, and OpenRealSessionFor calls it
// instead of transport.OpenSerial directly. It exists for exactly one
// property — that the baud handed to the serial layer is the DRIVER's
// own Capabilities().DefaultBaud rather than transport's package
// default — which is otherwise inexpressible from this package:
// transport.SerialConfig is carried into an OS-level open, and nothing
// transport exports lets a test read back the config a completed open
// used (transport's own openPort seam is package-private, and a test
// here cannot reach it). A recording seam at THIS call site is therefore
// the only place the disagreement between a driver's DefaultBaud and
// transport.DefaultBaud can be observed at all. Deliberately a
// package-level variable rather than a parameter on OpenRealSessionFor:
// adding one would put a port-construction hook in the public signature
// of a constructor whose whole shape (see this file's package comment)
// exists to keep invalid profile/port pairings unrepresentable.
var openSerial = transport.OpenSerial

// OpenRealSessionFor opens a session against a real radio of model,
// attached at portPath: a real serial port via transport.OpenSerial,
// paired with model's own real-hardware driver constructor from
// realDrivers. This is one of exactly two model-keyed wiring
// constructors (see fake.go's OpenFakeSessionFor); there is deliberately
// no shared helper taking a port alongside an ft710.Profile, so the
// invalid RealHardware/fakeradio (or Simulated/real-port) pairing stays
// unrepresentable in the code shape, not merely unreached.
//
// The port is opened at the DRIVER's own factory-default CAT baud
// (Capabilities().DefaultBaud), not at transport's package default: the
// two agree for the FT-710 (both 38400) and there is no behaviour change
// today, but a second registered model whose radio ships at a different
// rate would otherwise have been opened at the FT-710's — a
// radio-specific fact read from the wrong radio's table.
//
// An unrecognised model fails with *UnknownModelError BEFORE any port is
// touched. transport.OpenSerial (like driver.Driver.Open) owns the port
// it opens on both outcomes: on any error below, the port is already
// closed by whichever call failed, and this function never closes it
// itself.
func OpenRealSessionFor(ctx context.Context, model, portPath string) (driver.Session, func() error, error) {
	d, err := realDriverFor(model)
	if err != nil {
		return nil, nil, err
	}

	reg, err := NewRegistry(d)
	if err != nil {
		return nil, nil, err
	}
	drv, ok := reg.Get(model)
	if !ok {
		// Unreachable while TestDriverTableKeysMatchDriverModel holds: d
		// was just registered under its own Model(), which that test pins
		// equal to this table key. Returned rather than ignored so a
		// future table whose key drifted from its driver's Model() fails
		// with this package's own typed error instead of a nil-pointer
		// panic when drv is used below.
		return nil, nil, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}

	port, err := openSerial(portPath, transport.SerialConfig{
		// The baud is the radio's, read from the driver in hand (d is
		// the very value NewRegistry registered and reg.Get returned
		// above as drv — TestDriverTableKeysMatchDriverModel pins the
		// key they share).
		Baud: d.Capabilities().DefaultBaud,
		// The stop bits are NOT model-derived, BY RECORDED DECISION
		// (M9c-5 E2): spec.Capabilities gains no framing field this
		// milestone, so every model opens at transport's fixed default
		// (8-N-2). The FTdx10 is ASSUMED to share it until its framing
		// is verified against its own manual at M9c-6; if it does not,
		// the field is added then, with hardware evidence, rather than
		// guessed now.
		StopBits: transport.DefaultStopBits,
	})
	if err != nil {
		return nil, nil, &OpenSerialError{Port: portPath, Cause: err}
	}

	sess, err := drv.Open(ctx, port, driver.Identity{Port: portPath})
	if err != nil {
		// Open owns port on both outcomes: it is already closed.
		return nil, nil, &OpenSessionError{Port: portPath, Cause: err}
	}
	return sess, sess.Close, nil
}

// StaticCapabilities returns model's static baseline capability
// description — the same value NewRealDriver().Capabilities() reports for
// DefaultModel — via a registry lookup (mirroring OpenRealSessionFor's own
// construction, so Registry.Register's Capabilities().Validate check runs
// here too) plus Driver.Capabilities(). Fails with *UnknownModelError for
// an unrecognised model.
func StaticCapabilities(model string) (spec.Capabilities, error) {
	d, err := realDriverFor(model)
	if err != nil {
		return spec.Capabilities{}, err
	}
	reg, err := NewRegistry(d)
	if err != nil {
		return spec.Capabilities{}, err
	}
	drv, ok := reg.Get(model)
	if !ok {
		// Unreachable while TestDriverTableKeysMatchDriverModel holds: d
		// was just registered under its own Model(), which that test pins
		// equal to this table key. Returned rather than ignored so a
		// future table whose key drifted from its driver's Model() fails
		// with this package's own typed error instead of a nil-pointer
		// panic inside Capabilities().
		return spec.Capabilities{}, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}
	return drv.Capabilities(), nil
}

// StaticSettingsDescriptor returns model's driver-level settings tree —
// the driver.StaticSettingsProvider capability (core/driver/optional.go)
// — when model's driver implements it. The bool result reports whether
// the capability is present: false (with a zero SettingsDescriptor and a
// nil error) when model is known but its driver has no settings surface
// at all, distinct from a non-nil error, which means model itself is
// unrecognised.
func StaticSettingsDescriptor(model string) (driver.SettingsDescriptor, bool, error) {
	d, err := realDriverFor(model)
	if err != nil {
		return driver.SettingsDescriptor{}, false, err
	}
	prov, ok := d.(driver.StaticSettingsProvider)
	if !ok {
		return driver.SettingsDescriptor{}, false, nil
	}
	return prov.StaticSettingsDescriptor(), true, nil
}

// SynthesiseDiscoveredBanks classifies an offline slot list into the
// read-only banks a live session would have discovered for model, via the
// driver.DiscoveredBankSynthesizer capability (core/driver/optional.go),
// when model's driver implements it. The bool result is false whenever
// the classification did not happen — for BOTH an unrecognised model and
// a known model whose driver lacks the capability — mirroring this
// function's error-free signature (D6/F4): a caller that needs to tell
// the two apart should check model against SupportedModels() itself.
func SynthesiseDiscoveredBanks(model string, slots []string) ([]spec.Bank, bool) {
	d, err := realDriverFor(model)
	if err != nil {
		return nil, false
	}
	synth, ok := d.(driver.DiscoveredBankSynthesizer)
	if !ok {
		return nil, false
	}
	return synth.SynthesiseDiscoveredBanks(slots), true
}

// OpenSerialError is OpenRealSessionFor's typed failure when
// transport.OpenSerial itself cannot open portPath. See
// RegisterDriverError's doc comment for why Error() carries this
// package's own generic wording, and why a caller wanting different
// wording should use Port/Cause via errors.As instead.
type OpenSerialError struct {
	Port  string
	Cause error
}

func (e *OpenSerialError) Error() string {
	return fmt.Sprintf("wiring: open serial port %q: %v", e.Port, e.Cause)
}

func (e *OpenSerialError) Unwrap() error { return e.Cause }

// OpenSessionError is OpenRealSessionFor's typed failure when the driver's
// own Open fails against an already-open serial port. See
// RegisterDriverError's doc comment.
type OpenSessionError struct {
	Port  string
	Cause error
}

func (e *OpenSessionError) Error() string {
	return fmt.Sprintf("wiring: open session on %q: %v", e.Port, e.Cause)
}

func (e *OpenSessionError) Unwrap() error { return e.Cause }

// ModelSlug turns a model name into a filesystem-safe directory
// component: lowercased, with each run of non-alphanumeric characters
// collapsed to a single "-" (e.g. "FTDX101D/MP" -> "ftdx101d-mp"). Used
// to give each model its own snapshot/journal directory.
func ModelSlug(model string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(model) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// ResolveSnapshotDir returns the snapshot/journal directory a
// radio-touching composition root should use: override verbatim if
// non-empty, otherwise "<os.UserConfigDir()>/rigprog/snapshots" — the
// same default cmd/rigprog's own resolveSnapshotDir (cmd/rigprog/
// fileio.go) uses, so a GUI-written snapshot/journal and a CLI-written
// one land in the same place absent an override. It does not touch the
// filesystem — callers create the directory (mode 0700) on demand.
//
// model then decides whether that base directory is used directly or
// namespaced (task-7, D9): DefaultModel stays at the base directory
// unchanged — byte-identical to the pre-task-7 behaviour — so every
// snapshot written before per-model subdirectories existed is still
// found. Any other model gets its own <base>/<model-slug>/
// subdirectory, applied to an explicit override too, since two models
// sharing one explicitly-named directory is exactly the collision this
// rule exists to prevent. A model whose ModelSlug is "" (no
// alphanumeric characters at all) is refused with an error rather than
// silently falling back to the base directory — filepath.Join drops
// empty elements, so an unguarded empty slug would resolve to exactly
// DefaultModel's own path, the very collision this rule exists to
// prevent.
//
// Deliberately duplicated here rather than exported from cmd/rigprog:
// cmd/rigprog is a cmd-local package app/ must not import (task-15
// brief §2's Connect bullet); this 3-line rule is cheap enough to
// restate directly rather than force an import cmd/rigprog was never
// meant to expose.
func ResolveSnapshotDir(override, model string) (string, error) {
	base := override
	if base == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("wiring: determining default snapshot directory: %w", err)
		}
		base = filepath.Join(cfgDir, "rigprog", "snapshots")
	}
	if model == DefaultModel {
		return base, nil
	}
	slug := ModelSlug(model)
	if slug == "" {
		return "", fmt.Errorf("wiring: resolving snapshot directory: model %q has no filesystem-safe characters to slug — refusing to fall back to the base directory and collide with %s's", model, DefaultModel)
	}
	return filepath.Join(base, slug), nil
}

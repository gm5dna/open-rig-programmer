// SPDX-License-Identifier: GPL-3.0-or-later

// Package wiring holds the session-construction plumbing shared by every
// composition root in this repository (cmd/rigprog, app/) — extracted
// from cmd/rigprog/wiring.go by task-15 so the GUI (app/) has the exact
// same registry/driver/session wiring the CLI already proved, rather
// than a second, independently-drifting copy.
//
// The deliberate structural-exclusivity shape that constrained
// cmd/rigprog/wiring.go (task-11 brief §3) is preserved EXACTLY here: two
// fully self-contained session paths — the REAL one (this file:
// OpenRealSessionWith, the single implementation, plus OpenRealSessionFor,
// its zero-option delegate — two exported names over one body) and the
// SIMULATED one (fake.go's OpenFakeSessionFor) — with no shared helper
// accepting a profile alongside a port. That absence is the point: it keeps
// the invalid RealHardware/fake-rig or Simulated/real-port pairings
// structurally unrepresentable in the code shape, not merely unreached. What
// a caller may vary on the real path is bounded by SessionOptions, which
// carries the user's consent and may never carry a profile or a port object
// — the constraint is about what can be PAIRED, and it is untouched by the
// second name.
//
// EACH registered driver's simulated-profile selector — ft710.Simulated,
// ftdx10.Simulated since M9c-6, and ftdx101.Simulated since M9d-2 (ONE
// token for two registered models, since one driver package drives both
// FTDX101 siblings) — is referenced in exactly ONE non-test
// .go file repo-wide, fake.go, pinned per driver by internal/guards'
// TestSimulatedProfileTokensConfinement (extended by task-15 to be
// repo-wide rather than cmd/rigprog-local; folded from the single-driver
// guard task-15 extended into this data-driven guard at Task 58, which is
// why registering a second driver added a table ROW there rather than a
// second test).
//
// Task 39 (the M9a radio-neutral core refactor) generalised this package
// to model-keyed dispatch: the real path (this file) and
// OpenFakeSessionFor (fake.go) are the two fully self-contained,
// model-keyed session paths carrying the structural-exclusivity shape
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
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx10"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx101"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// DefaultModel names the driver.Registry key the model-keyed lookups below
// use when a caller has not (or cannot yet) name a model explicitly: the
// FALLBACK model, not the only registrable one. cmd/rigprog resolves it
// when --model is absent, and app/ when the frontend passes "" (it has no
// model picker yet — M9c-6's ledgered exclusion).
//
// It stays exactly "FT-710" however many other models are registered:
// which radio a caller gets by DEFAULT is a compatibility
// promise about every file, snapshot and journal written before any second
// model existed (see ResolveSnapshotDir's own model rule), not a statement
// about how many models this package supports. Changing it would silently
// re-point every default-model caller at a different radio. That is why
// this comment carries no registration COUNT: the count has changed twice
// already (M9c-6, M9d-2) and the promise has not moved.
const DefaultModel = "FT-710"

// FTdx10Model names the FTdx10's realDrivers/fakeDrivers key, which must
// equal ftdx10.New(...).Model() — that agreement is not assumed here but
// pinned by TestDriverTableKeysMatchDriverModel, which walks both tables.
// A named constant rather than a bare literal at each of its uses (the two
// table keys) because the two MUST be the same string: a typo in one alone
// would build a model openable for real but not simulated, which is the
// very drift TestRealAndFakeDriverTablesAgree exists to catch — and this
// way it cannot happen at all. DefaultModel is exported and this is too,
// so a caller naming the model (a CLI --model default, a GUI picker) has
// the same kind of handle for both.
const FTdx10Model = "FTdx10"

// FTdx101DModel names the FTDX101D's realDrivers/fakeDrivers key, which must
// equal ftdx101.NewD(...).Model() — pinned, like FTdx10Model's, by
// TestDriverTableKeysMatchDriverModel walking both tables. A named constant
// rather than a bare literal at each of its uses for exactly the reason
// FTdx10Model is one: the two table keys MUST be the same string, and a typo
// in one alone would build a model openable for real but not simulated.
//
// THE SPELLING IS THE PROJECT'S, of a manual fact (matrix §1.1). The manual
// prints "FTDX101D" in full capitals throughout; this project writes
// "FTdx101D", matching how "FTdx10" and "FT-710" are already spelt here. The
// two are NOT interchangeable: this constant is the driver-registry key and
// the radiotext key, and internal/radiotext deliberately leaves "FTDX101D"
// unknown so a caller that reached for the manual's spelling fails loudly
// rather than serving blank advisories.
//
// NOT the same string as internal/extable's "FTdx101D/MP", which is the
// JOINT inventory form: one EX profile serves both radios because the manual
// prints Table 2 once for the pair. That is a statement about a shared
// chart; this is a registry key for one radio.
const FTdx101DModel = "FTdx101D"

// FTdx101MPModel names the FTDX101MP's realDrivers/fakeDrivers key, which
// must equal ftdx101.NewMP(...).Model(). See FTdx101DModel for the spelling
// rule and the extable-form distinction, which apply here unchanged.
//
// TWO constants for two radios, and no shared "FTdx101" handle between them,
// deliberately: core/driver/ftdx101 offers NewD and NewMP over one
// implementation and no bare New, and core/cat/ftdx101 offers DialectD and
// DialectMP over one config and no bare Dialect(), for the same reason —
// there are two models, so neither is the other's fallback and neither is
// reachable by a caller that failed to choose.
const FTdx101MPModel = "FTdx101MP"

// IC7610Model names the IC-7610's realDrivers/fakeDrivers key, which must
// equal ic7610.New(...).Model() — pinned, like the three Yaesu constants
// above, by TestDriverTableKeysMatchDriverModel walking both tables. A
// named constant rather than a bare literal at each of its uses for the
// same reason those three are: the two table keys MUST be the same
// string, and a typo in one alone would build a model openable for real
// but not simulated.
//
// THIS IS THE FIRST NON-YAESU REGISTRATION (Wave 4, task R1). The spelling
// is the manufacturer's own, and it carries the hyphen ic7610.go's own
// Model() method and Capabilities().Model both declare ("IC-7610", not
// "IC7610" or "ic7610") — the two agreements TestDriverTableKeysMatchDriverModel
// and internal/guards' simulated-token guard both depend on, exactly as
// they do for every Yaesu row.
const IC7610Model = "IC-7610"

// realDrivers is the model-keyed table of real-hardware driver
// constructors: model name -> a constructor building THAT model's
// real-profile driver.Driver. It is the single source of truth
// SupportedModels, OpenRealSessionWith, StaticCapabilities,
// StaticSettingsDescriptor, and SynthesiseDiscoveredBanks all key off —
// adding a radio model to this package means adding one entry here (plus
// fake.go's own table for the simulated/demo path), never touching the
// functions themselves.
//
// The FTdx10 (M9c-6 task 6) is the first model added that way, and it was
// exactly that: this entry, one in fake.go, one radiotext entry, and not a
// line of the functions below. Every all-registered-models test in this
// package walks it by existing.
//
// The FTdx101D and FTdx101MP (M9d-2 task 7) are the second and third, and
// they added the same three things EACH. They are SIBLINGS — one driver
// package, one dialect config, one simulator, differing in a name and a CAT
// ID — and they still get two rows here rather than one, because this table
// is keyed by MODEL and a user selects a radio, not a family. Sharing a row
// would mean choosing which sibling a "FTdx101" selection meant, which is
// the choice core/driver/ftdx101 refuses to offer (no bare New) and
// core/cat/ftdx101 refuses to offer (no bare Dialect()).
//
// EACH ROW TAKES THE USER'S CONSENT (the unverified-write-consent
// milestone, task 8) and all four spend it identically: consent false calls
// that model's pinned zero-argument constructor unchanged — so the default
// path is not merely "still working" but byte-identical to the one it
// replaced, pinned by TestRealDriverFor_DefaultPathByteIdentical — and
// consent true builds the SAME profile with that driver package's own
// WithConsentedUnverifiedWrites().
//
// The FT-710's row carries the option too, though its real-hardware
// capability set has no Unverified write left for the transform to touch:
// the option is a proven no-op there (core/driver/ft710's own tests own that
// proof, so this table need not restate it). A row that omitted it would be
// a second shape to reason about for no gain — and one that would quietly
// stop being a no-op the day that radio gained an unverified field.
//
// CONSENT REACHES A SESSION, NEVER A STATIC SURFACE. Every driver's
// WithConsentedUnverifiedWrites leaves its static Capabilities untouched and
// shows up only in the set Open assembles, which is why the three static
// callers below pass false and mean it, and why the option's proof is a
// session-level test (TestOpenRealSessionWith_ConsentedSessionCaps) rather
// than a capability comparison here.
var realDrivers = map[string]func(consent bool) driver.Driver{
	DefaultModel: func(consent bool) driver.Driver {
		if consent {
			return ft710.New(ft710.RealHardware, ft710.WithConsentedUnverifiedWrites())
		}
		return NewRealDriver()
	},
	FTdx10Model: func(consent bool) driver.Driver {
		if consent {
			return ftdx10.New(ftdx10.RealHardware, ftdx10.WithConsentedUnverifiedWrites())
		}
		return NewFTdx10RealDriver()
	},
	FTdx101DModel: func(consent bool) driver.Driver {
		if consent {
			return ftdx101.NewD(ftdx101.RealHardware, ftdx101.WithConsentedUnverifiedWrites())
		}
		return NewFTdx101DRealDriver()
	},
	FTdx101MPModel: func(consent bool) driver.Driver {
		if consent {
			return ftdx101.NewMP(ftdx101.RealHardware, ftdx101.WithConsentedUnverifiedWrites())
		}
		return NewFTdx101MPRealDriver()
	},
	IC7610Model: func(consent bool) driver.Driver {
		if consent {
			return ic7610.New(ic7610.RealHardware, ic7610.WithConsentedUnverifiedWrites())
		}
		return NewIC7610RealDriver()
	},
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

// realDriverFor looks model up in realDrivers and constructs its driver at
// the caller's consent, or fails with *UnknownModelError. It is the shared
// entry point every model-keyed real-driver lookup in this file
// (OpenRealSessionWith, StaticCapabilities, StaticSettingsDescriptor,
// SynthesiseDiscoveredBanks) goes through, so "which models this package
// supports" has exactly one answer.
//
// consent is the USER's recorded acceptance of writing this radio's
// unverified fields, threaded through to the driver package's own
// WithConsentedUnverifiedWrites (see realDrivers). It is a plain bool and
// this package reads no store to obtain it: whoever calls decides, and
// nothing here can turn a caller's "no" into a "yes".
func realDriverFor(model string, consent bool) (driver.Driver, error) {
	ctor, ok := realDrivers[model]
	if !ok {
		return nil, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}
	return ctor(consent), nil
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

// NewFTdx10RealDriver builds the ftdx10 driver for a real-hardware
// session: profile ftdx10.RealHardware, the zero value — the FTdx10's
// half of the realDrivers table, split out for the same reason
// NewRealDriver is (a test can pin the capability set the real wiring path
// implies without opening a port).
//
// What that capability set IS differs from the FT-710's in the one way
// that matters, and it is the whole reason this entry is safe to register
// against real hardware at all: ftdx10's writeTrialsComplete is FALSE, so
// a RealHardware FTdx10 driver reports ftdx10.CapabilitiesUnverified —
// every candidate field's Write spec.Unverified, nothing writable
// anywhere. No FTdx10 has been written to by this project, and the
// capability gate refuses before any frame is built. Registering the model
// therefore adds a READ/probe path against real hardware and no write path
// (see core/driver/ftdx10/doc.go's write guard, and its ASSUMED register
// for what a Stage R session would lift).
//
// That is the whole truth for THIS constructor, and this constructor is
// what realDrivers' FTdx10 row returns for every unconsented caller. The
// CONSENTED row is a different construction — ftdx10.New(RealHardware,
// WithConsentedUnverifiedWrites()), built only when the user's recorded
// grant says so — and the session IT assembles re-labels those write-side
// Unverified fields spec.ConsentedUnverified, which FieldSupport.CanWrite
// opens. Even there the driver's STATIC Capabilities is untouched, which
// is exactly what lets NeedsUnverifiedConsent read it to decide the radio
// is consent-eligible at all. So "no write path" remains the answer for
// every caller who has not asked for one, and the write path a consenting
// user gets is one they were warned about and chose.
func NewFTdx10RealDriver() driver.Driver {
	return ftdx10.New(ftdx10.RealHardware)
}

// NewFTdx101DRealDriver builds the ftdx101 driver for a real-hardware
// FTDX101D session: profile ftdx101.RealHardware, the zero value — the
// FTdx101D's half of the realDrivers table, split out for the same reason
// NewRealDriver and NewFTdx10RealDriver are (a test can pin the capability
// set the real wiring path implies without opening a port).
//
// READ/PROBE ONLY, and by the same mechanism the FTdx10's entry is: this
// driver's writeTrialsCompleteD is FALSE, so a RealHardware FTDX101D driver
// reports the all-Unverified capability set — every candidate field's Write
// spec.Unverified, nothing writable on any bank. No FTDX101D has been
// written to by this project, and the capability gate refuses before any
// frame is built. Registering the model therefore adds a READ/probe path
// against real hardware and, for an UNCONSENTED session (the consent
// exception is named at the foot of this comment), NO write path (see
// core/driver/ftdx101/doc.go's write guard, and its ASSUMED register for
// what a Stage W session would lift).
//
// The FAIL-SAFE DIRECTION is worth restating because it is what makes this
// safe to register at all: an unrecognised Profile value selects the
// all-Unverified set too, never the simulator's write-Supported one. There
// is no value a caller can pass to this package that produces a
// write-capable real-hardware FTDX101D driver — with ONE named exception,
// which is not a value at all but a decision: SessionOptions'
// ConsentUnverifiedWrites, spent from the user's own recorded grant, makes
// realDrivers build the consented variant instead of this constructor's
// product, and the SESSION that variant opens carries
// spec.ConsentedUnverified in place of spec.Unverified and can therefore
// write. The exception is deliberately narrow and deliberately loud: it is
// unreachable without a stored grant, it never alters this driver's static
// capability set, it never touches FieldErase, and it is skipped for an
// unrecognised Profile — so the fail-safe direction above survives it
// intact.
func NewFTdx101DRealDriver() driver.Driver {
	return ftdx101.NewD(ftdx101.RealHardware)
}

// NewFTdx101MPRealDriver builds the ftdx101 driver for a real-hardware
// FTDX101MP session: profile ftdx101.RealHardware, the zero value. Same
// reasoning as NewFTdx101DRealDriver in every respect — the MP's own write
// guard is writeTrialsCompleteMP, and it is false for the MP's own reasons
// (no FTDX101MP has ever been written to by this project; the D's trials
// would not lift it, since the two radios share a manual and not a serial
// port).
//
// A SEPARATE CONSTRUCTOR rather than a model parameter, deliberately: the
// driver package fixes its exported surface as two thin constructors so
// that a registration-table closure cannot hold a forged model value, and
// this table's two rows are exactly the callers that shape was chosen for.
func NewFTdx101MPRealDriver() driver.Driver {
	return ftdx101.NewMP(ftdx101.RealHardware)
}

// NewIC7610RealDriver builds the ic7610 driver for a real-hardware
// session: profile ic7610.RealHardware, the zero value — the IC-7610's
// half of the realDrivers table, split out for the same reason the four
// Yaesu constructors above are (a test can pin the capability set the
// real wiring path implies without opening a port).
//
// READ/PROBE ONLY, and by the same mechanism as every Yaesu row: this
// driver's writeTrialsComplete (core/driver/ic7610/caps.go) is FALSE, so a
// RealHardware IC-7610 driver reports the all-Unverified capability set —
// every mapped field's Write spec.Unverified, nothing writable on either
// bank. No IC-7610 has been written to by this project, and the
// capability gate refuses before any frame is built.
//
// THE FAIL-SAFE DIRECTION IS UNCHANGED BY THIS BEING A CI-V DRIVER RATHER
// THAN A CAT ONE: an unrecognised Profile value selects the all-Unverified
// set too (ic7610.go's Capabilities switch), never the simulator's
// write-Supported one, and the one named exception — SessionOptions'
// ConsentUnverifiedWrites, spent from the user's own recorded grant — is
// exactly the mechanism the Yaesu rows use, reaching realDrivers'
// IC7610Model row above and never this constructor.
func NewIC7610RealDriver() driver.Driver {
	return ic7610.New(ic7610.RealHardware)
}

// openSerial is OpenRealSessionWith's test seam (and so OpenRealSessionFor's
// too, that being its zero-option delegate): production code always leaves
// this at transport.OpenSerial, and OpenRealSessionWith calls it
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
// package-level variable rather than a parameter on either function (or a
// SessionOptions field):
// adding one would put a port-construction hook in the public signature
// of a constructor whose whole shape (see this file's package comment)
// exists to keep invalid profile/port pairings unrepresentable.
var openSerial = transport.OpenSerial

// SessionOptions carries what a caller may vary about a real-hardware
// session beyond the model and the port. It is a struct rather than a
// parameter so that a later option is an added field, not a fifth argument
// at every call site — but it is deliberately NOT a general options bag: it
// may never grow a driver profile or a port object, which would recreate
// exactly the "profile + port" seam this file's structural-exclusivity shape
// exists to rule out.
type SessionOptions struct {
	// ConsentUnverifiedWrites is the USER's recorded acceptance of writing
	// this radio's unverified fields, passed to model's driver as that
	// package's WithConsentedUnverifiedWrites (see realDrivers). FALSE is
	// the zero value and the default, so OpenRealSessionFor's zero-option
	// delegation is the pre-consent behaviour exactly.
	//
	// A BOOL, and this package reads no consent store to fill it: whose
	// consent it is, where it was recorded and whether it is still current
	// are questions for the composition root that owns the user (see
	// internal/userconfig). Wiring's job is to spend the answer, not to
	// find it — which is also why no userconfig import appears in this
	// package.
	ConsentUnverifiedWrites bool
}

// OpenRealSessionWith opens a session against a real radio of model,
// attached at portPath, under opts: a real serial port via
// transport.OpenSerial, paired with model's own real-hardware driver
// constructor from realDrivers, built at opts.ConsentUnverifiedWrites.
//
// It is the ONE real implementation of this package's real-session path, and
// OpenRealSessionFor below is its zero-option delegate — two exported names
// over one body, not two constructors. The structural-exclusivity shape both
// carry is unchanged and is what SessionOptions is bounded by: neither
// function, and no helper either reaches, lets a caller supply a driver
// profile or a port object, so the invalid RealHardware/fakeradio (or
// Simulated/real-port) pairing stays unrepresentable in the code shape
// rather than merely unreached. fake.go's OpenFakeSessionFor is the
// simulated half of that shape and is wholly separate from this one.
//
// CONSENT REACHES THE SESSION ALONE. The option transforms the capability
// set the driver's Open assembles (write-side spec.Unverified becomes
// spec.ConsentedUnverified) and leaves that driver's static Capabilities
// untouched — which is why this function is where the option is proved
// (TestOpenRealSessionWith_ConsentedSessionCaps, over every consent-eligible
// model) and why the static lookups below pass false.
//
// The port is opened at the DRIVER's own factory-default CAT baud
// (Capabilities().DefaultBaud), not at transport's package default: all
// FOUR registered values agree with transport's today (the FT-710's 38400,
// the FTdx10's ASSUMED 38400 — core/driver/ftdx10's register entry, whose
// lift is the rate a factory-configured radio's ID exchange answers at —
// and the FTdx101D's and FTdx101MP's, ASSUMED 38400 on the same footing
// and with the same per-model lift), so there is no behaviour change, but a
// registered model whose radio ships at a different rate would otherwise
// have been opened at the FT-710's — a radio-specific fact read from the
// wrong radio's table.
// TestOpenRealSessionFor_BaudFollowsADisagreeingDriver proves the
// derivation with a fixture that actually disagrees, since no registered
// model does.
//
// An unrecognised model fails with *UnknownModelError BEFORE any port is
// touched. transport.OpenSerial (like driver.Driver.Open) owns the port
// it opens on both outcomes: on any error below, the port is already
// closed by whichever call failed, and this function never closes it
// itself.
func OpenRealSessionWith(ctx context.Context, model, portPath string, opts SessionOptions) (driver.Session, func() error, error) {
	d, err := realDriverFor(model, opts.ConsentUnverifiedWrites)
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

	stopBits, err := stopBitsFor(d)
	if err != nil {
		return nil, nil, err
	}

	port, err := openSerial(portPath, transport.SerialConfig{
		// The baud is the radio's, read from the driver in hand (d is
		// the very value NewRegistry registered and reg.Get returned
		// above as drv — TestDriverTableKeysMatchDriverModel pins the
		// key they share).
		Baud: d.Capabilities().DefaultBaud,
		// The stop bits are the DRIVER's where the driver has something
		// honest to say, and transport's fixed default (8-N-2) where it
		// has not — see stopBitsFor.
		StopBits: stopBits,
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

// OpenRealSessionFor opens a session against a real radio of model, attached
// at portPath, with NO options: it is OpenRealSessionWith's zero-option
// delegate and has no body of its own, so a caller that has no consent
// decision to express — and every caller written before consent existed —
// gets exactly the pre-consent behaviour, by construction rather than by
// agreement between two implementations
// (TestOpenRealSessionFor_DelegatesZeroOptions pins the sessions equal).
//
// Its signature is deliberately unchanged. The consent-bearing path is a new
// NAME, not a new argument on this one: threading a bool through every
// existing call site would have made every caller state a consent position,
// including the many that have none.
func OpenRealSessionFor(ctx context.Context, model, portPath string) (driver.Session, func() error, error) {
	return OpenRealSessionWith(ctx, model, portPath, SessionOptions{})
}

// StaticCapabilities returns model's static baseline capability
// description — the same value NewRealDriver().Capabilities() reports for
// DefaultModel — via a registry lookup (mirroring OpenRealSessionFor's own
// construction, so Registry.Register's Capabilities().Validate check runs
// here too) plus Driver.Capabilities(). Fails with *UnknownModelError for
// an unrecognised model.
func StaticCapabilities(model string) (spec.Capabilities, error) {
	// consent false: a static surface describes the radio, never a user's consent.
	d, err := realDriverFor(model, false)
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

// NeedsUnverifiedConsent reports whether model is CONSENT-ELIGIBLE: whether
// its real-hardware baseline still carries a write-side spec.Unverified
// anywhere, and so whether a user's recorded consent could open a write gate
// on it at all. Fails with *UnknownModelError for an unrecognised model, and
// returns false alongside it — a caller that ignored the error must not be
// told an unnameable radio can be consented to.
//
// ONE implementation, exported, because two composition roots ask the same
// question: the CLI's "settings unverified-writes" (which models it lists as
// on/off, and which it refuses a grant for) and the GUI's own consent
// surface. A private copy in either would let the two disagree about which
// radios consent even applies to — an FT-710 owner asked to authorise an
// unverified write that cannot exist, or an FTdx10 owner refused a grant that
// would have unlocked one.
//
// STATIC capabilities, never a session's, and that is what makes the question
// answerable before any port is opened: the consent transform leaves a
// driver's static set untouched (see OpenRealSessionWith), so this predicate
// describes the RADIO — "has this project written to one of these and proved
// it?" — and never a particular user's decision. spec.ConsentedUnverified is
// deliberately not counted: it is what a consented SESSION carries, and it
// cannot appear in a static set at all. Nor is a write-side Unverified on
// spec.FieldErase — see consentCouldUnlockAWrite.
func NeedsUnverifiedConsent(model string) (bool, error) {
	caps, err := StaticCapabilities(model)
	if err != nil {
		return false, err
	}
	return consentCouldUnlockAWrite(caps), nil
}

// consentCouldUnlockAWrite reports whether caps carries a write-side
// spec.Unverified that a grant could actually turn into a permitted write.
//
// spec.FieldErase is SKIPPED, and that is the whole reason this is a named
// predicate rather than an inline loop. spec.ConsentUnverifiedWrites
// structurally exempts FieldErase — it converts every other Unverified
// write label to ConsentedUnverified and leaves erase exactly as it found
// it — so an Unverified erase is not something consent can unlock. Counting
// it would make a radio whose ONLY write-side Unverified sat on erase
// "consent-eligible": its owner would be shown the arming dialogue, asked
// to authorise an unverified write, and (in the GUI) put through a
// disconnect/reconnect to grant something that provably changes nothing.
//
// No registered model has that shape today, which is exactly why the rule
// is written down here and pinned by a fixture
// (TestConsentCouldUnlockAWrite_EraseOnlyIsNotEligible) rather than left to
// be noticed when one arrives.
func consentCouldUnlockAWrite(caps spec.Capabilities) bool {
	for _, b := range caps.Banks {
		for f, fs := range b.Fields {
			if f != spec.FieldErase && fs.Write == spec.Unverified {
				return true
			}
		}
	}
	return false
}

// StaticSettingsDescriptor returns model's driver-level settings tree —
// the driver.StaticSettingsProvider capability (core/driver/optional.go)
// — when model's driver implements it. The bool result reports whether
// the capability is present: false (with a zero SettingsDescriptor and a
// nil error) when model is known but its driver has no settings surface
// at all, distinct from a non-nil error, which means model itself is
// unrecognised.
func StaticSettingsDescriptor(model string) (driver.SettingsDescriptor, bool, error) {
	// consent false: a static surface describes the radio, never a user's consent.
	d, err := realDriverFor(model, false)
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
	// consent false: a static surface describes the radio, never a user's consent.
	d, err := realDriverFor(model, false)
	if err != nil {
		return nil, false
	}
	synth, ok := d.(driver.DiscoveredBankSynthesizer)
	if !ok {
		return nil, false
	}
	return synth.SynthesiseDiscoveredBanks(slots), true
}

// stopBitsFor is spec D3.1's serial-framing rule in one place: consult
// the driver's OPTIONAL driver.SerialFramingReporter, and refuse anything
// it reports other than 1 or 2.
//
// STILL NOT A spec.Capabilities FIELD. The M9c-5 (E2) rule — a framing
// field only with hardware evidence — is untouched, and the four
// registered Yaesu models still reach the serial layer at
// transport.DefaultStopBits, because none of them implements the
// interface. What D3.1 adds is somewhere for a driver that DOES have a
// framing fact to put it, and the Icom tier's six models are that case.
//
// ZERO IS REFUSED WITH THE REST, and that is the rule's whole substance.
// The obvious implementation — "if the report is <= 0, use the default" —
// is exactly what must not happen: a driver whose StopBits() returns the
// zero value has not asked for 8-N-2, it has failed to answer, and
// substituting a guess there would put transport's default on the wire
// under a driver's authority and with no diagnostic. A driver with
// nothing to say implements NOTHING; a driver that implements this
// interface must mean what it returns.
//
// The refusal happens BEFORE the port is opened, so a misconfigured
// driver never gets as far as touching hardware.
func stopBitsFor(d driver.Driver) (int, error) {
	r, ok := d.(driver.SerialFramingReporter)
	if !ok {
		// The E2-owed FTdx10 verification is CLOSED, and closed as
		// SILENCE: that radio's CAT manual makes no framing statement
		// anywhere (M9c-6 spec D-framing), so 8-N-2 for the FTdx10 is an
		// ASSUMED entry in core/driver/ftdx10's own register with a named
		// hardware lift — not a verified fact. Same for the other three.
		return transport.DefaultStopBits, nil
	}
	got := r.StopBits()
	if got != 1 && got != 2 {
		return 0, &UnsupportedStopBitsError{Model: d.Model(), StopBits: got}
	}
	return got, nil
}

// UnsupportedStopBitsError is OpenRealSessionWith's typed refusal when a
// driver's driver.SerialFramingReporter reports a stop-bit count that is
// not 1 or 2. Returned BEFORE any port is opened.
//
// It names the MODEL as well as the number, because the fault is in a
// driver's registration table and the message has to say which driver's.
type UnsupportedStopBitsError struct {
	// Model is the driver that reported StopBits.
	Model string
	// StopBits is what it reported.
	StopBits int
}

func (e *UnsupportedStopBitsError) Error() string {
	return fmt.Sprintf("wiring: driver %s reports %d stop bits; only 1 or 2 are supported (a driver with no framing evidence must not implement driver.SerialFramingReporter at all — zero is not a request for the default)", e.Model, e.StopBits)
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

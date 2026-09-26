// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"maps"
	"slices"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakeyaesuclone"
)

// cloneModels maps every OPERATOR-SELECTABLE clone-mode model name to the
// SINGLE core/clonewire.Profile that name identifies — a SEPARATE
// registry from realDrivers/fakeDrivers (Codex blocker 1, plan.md Phase
// 4): no clone-mode model implements driver.Session, so it cannot be a
// realDrivers/fakeDrivers entry, and SupportedModels()/StaticCapabilities
// must never see it (TestCloneModelsDisjointFromSessionModels proves
// they don't).
//
// Model identity is OPERATOR-ASSERTED, not wire-inferred (spec.md
// §Identity probe): CHIRP itself cannot distinguish FT-857 from FT-857D
// (or FT-897 from FT-897D) — one MODEL string, one image length, for
// each pair (core/clonewire/profile.go). Offering only the ONE Profile
// the caller named as the candidate set means OpenCloneReader never
// returns ErrImageAmbiguous for an ordinary same-length sibling pair; the
// operator's selection is trusted, and the CLI/GUI say so in their own
// output (cmd/rigprog/cloneread.go).
var cloneModels = map[string][]clonewire.Profile{
	clonewire.FT817.Model:     {clonewire.FT817},
	clonewire.FT817ND.Model:   {clonewire.FT817ND},
	clonewire.FT817NDUS.Model: {clonewire.FT817NDUS},
	clonewire.FT818.Model:     {clonewire.FT818},
	clonewire.FT818NDUS.Model: {clonewire.FT818NDUS},
	clonewire.FT857.Model:     {clonewire.FT857},
	clonewire.FT857D.Model:    {clonewire.FT857D},
	clonewire.FT857US.Model:   {clonewire.FT857US},
	clonewire.FT857DUS.Model:  {clonewire.FT857DUS},
	clonewire.FT897.Model:     {clonewire.FT897},
	clonewire.FT897D.Model:    {clonewire.FT897D},
	clonewire.FT897US.Model:   {clonewire.FT897US},
	clonewire.FT897DUS.Model:  {clonewire.FT897DUS},
}

// CloneModels returns every clone-mode model name OpenCloneReader
// accepts, sorted — mirrors SupportedModels()'s shape, but is an entirely
// separate list (see cloneModels' doc comment).
func CloneModels() []string {
	return slices.Sorted(maps.Keys(cloneModels))
}

// OpenCloneReader resolves model against cloneModels and returns the
// single candidate Profile that model names. Unknown model returns
// *UnknownModelError with Supported set to CloneModels() (not
// SupportedModels()), so the message names the right list.
func OpenCloneReader(model string) ([]clonewire.Profile, error) {
	profiles, ok := cloneModels[model]
	if !ok {
		return nil, &UnknownModelError{Model: model, Supported: CloneModels()}
	}
	return profiles, nil
}

// OpenCloneRealPort opens portPath for a clone-mode read at profile's own
// serial parameters (Baud, StopBits) — NEVER transport.DefaultBaud/
// DefaultStopBits, which are an FT-710 CAT estimate with no bearing on
// this family (spec.md §Frame grammar). DataBits/Parity are not
// separately configurable (transport.SerialConfig's own fixed 8-N, which
// happens to match every Profile in this family).
func OpenCloneRealPort(portPath string, profile clonewire.Profile) (transport.Port, error) {
	return openSerial(portPath, transport.SerialConfig{
		Baud:     profile.Baud,
		StopBits: profile.StopBits,
	})
}

// OpenCloneFakePort starts an internal/fakeyaesuclone.Radio playing
// model's Profile and returns its PC-side port plus a closer. Unknown
// model returns *UnknownModelError, same as OpenCloneReader.
func OpenCloneFakePort(model string) (transport.Port, func() error, error) {
	profiles, ok := cloneModels[model]
	if !ok {
		return nil, nil, &UnknownModelError{Model: model, Supported: CloneModels()}
	}
	radio := fakeyaesuclone.New(profiles[0])
	return radio.Port(), radio.Close, nil
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic705 "github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// newEngine is the ONE place this driver builds a CI-V engine.
//
// The framing comes from the CODEC (spec D2; enabler E1), never from a
// local adapter. An adapter here would duplicate four sibling packages',
// would have to re-derive E1's lock over the accumulator — which
// core/civ/accumulator.go ("not safe for concurrent use") and
// core/transport/framing.go ("an implementation that shares state between
// the two needs its own lock") between them make a documented data race —
// and would sit below deviation (a)'s settled "matchers from the codec"
// bar. This driver's own engine_test.go proves the landed lock holds under
// -race, because it is this driver that would corrupt memory if it did not.
//
// It returns the FRAMING CARRIER as well as the engine. transport.Engine
// keeps its framing in an unexported field, so this is the only moment the
// concrete value is in anyone's hand; a Session that never saw it could
// never assert civ.AccumulatorStatsReporter later, and its Diagnostics()
// would report zero for exactly the frames it exists to count — the CI-V
// accumulator drops broadcasts INTERNALLY, before the engine can see one.
func newEngine(port transport.Port, opts ...transport.Option) (*transport.Engine, civ.AccumulatorStatsReporter, error) {
	fr, stats, err := newFramingFor(civic705.Profile())
	if err != nil {
		return nil, nil, err
	}
	eng, err := transport.NewEngineWith(port, fr, opts...)
	if err != nil {
		return nil, nil, err
	}
	return eng, stats, nil
}

// newFramingFor builds the CI-V framing for p and asserts the optional
// stats capability on it, returning both or neither.
//
// IT IS A SEPARATE FUNCTION SO THE FAILURE PATH IS REACHABLE FROM A TEST.
// newEngine names this radio's profile itself — that is the point of a
// per-model driver — so without this seam the "an unconfigured profile is
// refused, never defaulted" property could only be asserted about
// civ.NewFraming rather than about this driver's use of it, which is the
// half that could actually regress.
//
// Both failures are TOTAL: nothing is returned alongside an error, and
// there is no fallback framing anywhere on this path. A driver that
// quietly substituted a default framing would install an outbound gate
// that speaks for no radio, which is precisely the last defence this
// project has before bytes reach hardware.
func newFramingFor(p civ.Profile) (transport.Framing, civ.AccumulatorStatsReporter, error) {
	fr, err := civ.NewFraming(p)
	if err != nil {
		return nil, nil, fmt.Errorf("ic705: framing: %w", err)
	}
	stats, ok := fr.(civ.AccumulatorStatsReporter)
	if !ok {
		// Unreachable against the landed adapter, which asserts the
		// interface at compile time in its own package — and checked
		// anyway, because the alternative to a loud failure here is a
		// session whose broadcast diagnostics silently read zero.
		return nil, nil, fmt.Errorf("ic705: framing %T does not report accumulator stats", fr)
	}
	return fr, stats, nil
}

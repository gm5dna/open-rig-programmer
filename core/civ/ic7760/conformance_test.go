// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
)

type conformanceAudit struct {
	t      *testing.T
	errors []string
}

func (a *conformanceAudit) Helper()                      { a.t.Helper() }
func (a *conformanceAudit) Logf(f string, args ...any)   { a.t.Logf(f, args...) }
func (a *conformanceAudit) Fatal(args ...any)            { a.t.Fatal(args...) }
func (a *conformanceAudit) Fatalf(f string, args ...any) { a.t.Fatalf(f, args...) }
func (a *conformanceAudit) Errorf(f string, args ...any) {
	a.errors = append(a.errors, fmt.Sprintf(f, args...))
}

func TestConformance(t *testing.T) {
	// civtest's deliberately simpler shared address walk treats base-hi+1
	// as invalid. For this profile that value is the first exact ExtraRange
	// endpoint, P1. Keep civtest.Run, capture that one intended disagreement,
	// and fail on every other shared-suite finding.
	audit := &conformanceAudit{t: t}
	civtest.Run(audit, ic7760.Profile())
	if len(audit.errors) != 1 || !strings.Contains(audit.errors[0], "BuildMemoryRead(g0/ch100) built") || !strings.Contains(audit.errors[0], "outside this profile's own space") {
		t.Fatalf("civtest.Run findings = %q, want only the pinned base-hi/P1 disagreement", audit.errors)
	}
}

// This local pin keeps the model-specific receiver in the shared
// conformance run genuinely disagreeing with the suite's fixtures: B2,
// flat P1/P2 extras and a ten-byte name bound must all be consulted from
// this profile rather than replaced by a shared constant.
func TestConformanceUsesTheIC7760ProfileData(t *testing.T) {
	p := ic7760.Profile()
	if p.RadioAddress() != 0xB2 || p.AddressForm() != civ.AddressFormFlat || p.NameLength() != 10 {
		t.Fatalf("IC-7760-specific profile data drifted: address=%02X form=%v name=%d", p.RadioAddress(), p.AddressForm(), p.NameLength())
	}
	if _, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: 101}); err != nil {
		t.Fatalf("IC-7760 P2 extra address refused: %v", err)
	}
	if _, err := p.BuildMemoryRead(civ.ChannelAddress{Channel: 102}); err == nil {
		t.Fatal("shared conformance receiver admitted channel 102 past IC-7760 P2")
	}
}

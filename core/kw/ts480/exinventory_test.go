// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// THIS FILE PINS A SIGNATURE, NOT VALUES — see core/kw/ts590's twin for the
// full reasoning. exinventory_gen.go is a BOOTSTRAP placeholder and lane
// P's file; its contents are pinned by lane P's own staleness test and by
// T10's cross-check, never here.

// The signature, asserted by the COMPILER. Lane P must not touch this name.
var _ func() []kw.EXItem = EXItems

// TestAccessor_IsCallable is the runtime half.
func TestAccessor_IsCallable(t *testing.T) {
	got := EXItems()
	if got == nil {
		t.Error("EXItems() returned nil — an accessor over a copy never returns nil, even for an empty inventory")
	}
	if len(got) != len(EXItems()) {
		t.Error("EXItems() is not stable across calls")
	}
}

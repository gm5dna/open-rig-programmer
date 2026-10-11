// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"io"
	"sync/atomic"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

type nopPort struct{ io.ReadWriter }

func (nopPort) Close() error { return nil }

// TestUnsetRequiredParamsAreErrorsNotPanics pins that a Params missing
// Profile or NewRecordLen is refused at engine entry rather than panicking
// on a nil function.
func TestUnsetRequiredParamsAreErrorsNotPanics(t *testing.T) {
	p := &Params{Name: "unset"}
	if _, err := Open(t.Context(), p, nopPort{}, driver.Identity{}); err == nil {
		t.Error("Open with an unset Params returned no error")
	}
	if _, _, _, err := ReadRaw(t.Context(), p, nil, new(atomic.Uint64), civ.ChannelAddress{Channel: 1}); err == nil {
		t.Error("ReadRaw with an unset Params returned no error")
	}
}

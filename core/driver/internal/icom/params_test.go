// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// failPort fails every transfer, so a regression that reaches the wire
// fails an assertion cleanly instead of nil-panicking.
type failPort struct{}

var errFailPort = errors.New("failPort: the engine must not reach the wire")

func (failPort) Read([]byte) (int, error)  { return 0, errFailPort }
func (failPort) Write([]byte) (int, error) { return 0, errFailPort }
func (failPort) Close() error              { return nil }

// TestUnsetRequiredParamsAreErrorsNotPanics pins that a Params missing
// Profile or NewRecordLen is refused at Open rather than panicking on a nil
// function. ReadRaw is not checked: it trusts a Params Open accepted.
func TestUnsetRequiredParamsAreErrorsNotPanics(t *testing.T) {
	p := &Params{Name: "unset"}
	_, err := Open(t.Context(), p, failPort{}, driver.Identity{})
	if err == nil {
		t.Fatal("Open with an unset Params returned no error")
	}
	if errors.Is(err, errFailPort) {
		t.Errorf("Open reached the wire with an unset Params: %v", err)
	}
}

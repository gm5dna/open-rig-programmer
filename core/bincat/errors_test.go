// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"errors"
	"testing"
)

func TestInvalidProfileWrapsSentinel(t *testing.T) {
	err := invalidProfile("bad field %s", "X")
	if !errors.Is(err, ErrInvalidProfile) {
		t.Fatalf("invalidProfile error = %v, does not wrap ErrInvalidProfile", err)
	}
	if err.Error() == "" {
		t.Fatal("invalidProfile produced an empty message")
	}
}

func TestSentinelsAreDistinct(t *testing.T) {
	sentinels := []error{ErrFrame, ErrBCD, ErrInvalidProfile, ErrRecord}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %d (%v) unexpectedly matches sentinel %d (%v)", i, a, j, b)
			}
		}
	}
}

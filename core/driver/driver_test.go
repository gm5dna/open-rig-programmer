// SPDX-License-Identifier: GPL-3.0-or-later

package driver_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestWrongRadioError(t *testing.T) {
	var err error = &driver.WrongRadioError{Want: "0800", Got: "0761"}

	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Errorf("errors.Is(WrongRadioError, ErrWrongRadio) = false, want true")
	}

	var wre *driver.WrongRadioError
	if !errors.As(err, &wre) {
		t.Fatalf("errors.As(WrongRadioError) = false, want true")
	}
	if wre.Want != "0800" || wre.Got != "0761" {
		t.Errorf("WrongRadioError fields = %q/%q, want 0800/0761", wre.Want, wre.Got)
	}

	msg := err.Error()
	if !strings.Contains(msg, "0800") || !strings.Contains(msg, "0761") {
		t.Errorf("Error() = %q, want both the expected and the found CAT ID present", msg)
	}
}

func TestWriteRefusedError(t *testing.T) {
	tests := []struct {
		name         string
		err          *driver.WriteRefusedError
		wantContains []string
	}{
		{
			name: "with fields",
			err: &driver.WriteRefusedError{
				Slot:   "001",
				Fields: []spec.Field{spec.FieldCTCSSTone, spec.FieldScanSkip},
				Reason: "not writable per this session's capabilities",
			},
			wantContains: []string{"001", "ctcss_tone", "scan_skip", "not writable"},
		},
		{
			name: "reason only",
			err: &driver.WriteRefusedError{
				Slot:   "P1L",
				Reason: "erase cannot be expressed",
			},
			wantContains: []string{"P1L", "erase cannot be expressed"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error = tt.err
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Errorf("errors.Is(WriteRefusedError, ErrWriteRefused) = false, want true")
			}
			msg := err.Error()
			for _, want := range tt.wantContains {
				if !strings.Contains(msg, want) {
					t.Errorf("Error() = %q, want it to contain %q", msg, want)
				}
			}
		})
	}
}

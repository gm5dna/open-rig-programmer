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

// TestWrongRadioError_Text pins BOTH rendered forms of
// WrongRadioError.Error() as exact literals (M9d-2 task 1, spec A5).
//
// The ID-only text is the one this type has always produced, and it is
// pinned here BYTE-FOR-BYTE precisely because the optional WantModel and
// GotModel fields were added underneath it: a driver that cannot name
// the models its CAT IDs belong to (the FT-710's and the FTdx10's never
// can) must keep getting the pre-existing wording, since rendered
// refusals are recorded in baselines. The named text is the additive
// form, which fires ONLY when BOTH names are populated — one name alone
// is not enough to render the "identifies as X; you selected Y"
// sentence, so it falls back to IDs.
func TestWrongRadioError_Text(t *testing.T) {
	tests := []struct {
		name string
		err  *driver.WrongRadioError
		want string
	}{
		{
			name: "ID only",
			err:  &driver.WrongRadioError{Want: "0800", Got: "0761"},
			want: `driver: connected radio identified as CAT ID "0761", want "0800" — wrong radio model on this port`,
		},
		{
			name: "both names",
			err: &driver.WrongRadioError{
				Want: "0681", Got: "0682",
				WantModel: "FTdx101D", GotModel: "FTdx101MP",
			},
			want: `driver: connected radio identifies as FTdx101MP (CAT ID "0682"); you selected FTdx101D (CAT ID "0681") — wrong radio model on this port`,
		},
		{
			name: "want name only falls back to IDs",
			err: &driver.WrongRadioError{
				Want: "0681", Got: "0682", WantModel: "FTdx101D",
			},
			want: `driver: connected radio identified as CAT ID "0682", want "0681" — wrong radio model on this port`,
		},
		{
			name: "got name only falls back to IDs",
			err: &driver.WrongRadioError{
				Want: "0681", Got: "0682", GotModel: "FTdx101MP",
			},
			want: `driver: connected radio identified as CAT ID "0682", want "0681" — wrong radio model on this port`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWrongRadioError_IsAs_BothShapes: the additive names change the
// rendered text and NOTHING else — errors.Is against the sentinel and
// errors.As with Want/Got (and now the names) preserved must behave
// identically whether or not the model names are populated.
func TestWrongRadioError_IsAs_BothShapes(t *testing.T) {
	tests := []struct {
		name string
		err  *driver.WrongRadioError
		// The model names the recovered error must carry, as LITERALS.
		//
		// They are literals rather than a read-back of tt.err's own fields
		// because errors.As recovers THE VERY POINTER the table holds, so
		// `wre.WantModel != tt.err.WantModel` compares a field with itself
		// and can never fail — which is exactly what this pair did until
		// the M9d ledgered-minors wave. The IDs at hand below were always
		// written as literals; these now match that shape.
		wantModel string
		gotModel  string
	}{
		{
			// wantModel/gotModel left at "": the additive names must stay
			// ABSENT on the ID-only shape, and the zero value asserts it.
			name: "ID only",
			err:  &driver.WrongRadioError{Want: "0681", Got: "0682"},
		},
		{
			name: "both names",
			err: &driver.WrongRadioError{
				Want: "0681", Got: "0682",
				WantModel: "FTdx101D", GotModel: "FTdx101MP",
			},
			wantModel: "FTdx101D",
			gotModel:  "FTdx101MP",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error = tt.err

			if !errors.Is(err, driver.ErrWrongRadio) {
				t.Errorf("errors.Is(WrongRadioError, ErrWrongRadio) = false, want true")
			}

			var wre *driver.WrongRadioError
			if !errors.As(err, &wre) {
				t.Fatalf("errors.As(WrongRadioError) = false, want true")
			}
			if wre.Want != "0681" || wre.Got != "0682" {
				t.Errorf("WrongRadioError IDs = %q/%q, want 0681/0682", wre.Want, wre.Got)
			}
			if wre.WantModel != tt.wantModel || wre.GotModel != tt.gotModel {
				t.Errorf("WrongRadioError names = %q/%q, want %q/%q", wre.WantModel, wre.GotModel, tt.wantModel, tt.gotModel)
			}
		})
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

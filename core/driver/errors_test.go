// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorMessages(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{
			"a string-addressed answer mismatch",
			&AnswerMismatchError[string]{Model: "ftdx10", Requested: "003", Answered: "004"},
			"ftdx10: requested 003 but the answer names 004 — refusing to map a reply onto the wrong slot",
		},
		{
			"an integer-addressed answer mismatch",
			&AnswerMismatchError[int]{Model: "ic7610", Requested: 3, Answered: 7},
			"ic7610: requested 3 but the answer names 7 — refusing to map a reply onto the wrong slot",
		},
		{
			"an unknown setting",
			&UnknownSettingError{Model: "ft710", ID: "nope"},
			`ft710: ReadSetting: "nope" is not a known ft710 setting ID`,
		},
		{
			"a settings answer mismatch",
			&SettingAnswerMismatchError{Model: "ftdx10", Requested: "010101", Answered: "010102"},
			`ftdx10: ReadSetting: requested EX address "010101" but the answer names address "010102" — refusing to map a reply onto the wrong setting`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q\n    want %q", got, tc.want)
			}
		})
	}
}

func TestAnswerMismatchErrorIs(t *testing.T) {
	mismatch := &AnswerMismatchError[string]{Model: "ic7300", Requested: "003", Answered: "004"}
	if !errors.Is(mismatch, ErrAnswerMismatch) {
		t.Error("errors.Is(err, ErrAnswerMismatch) = false; want true")
	}
	// Wrapped, which is how a driver's read path returns it.
	if !errors.Is(fmt.Errorf("reading slot 003: %w", mismatch), ErrAnswerMismatch) {
		t.Error("errors.Is on a wrapped mismatch = false; want true")
	}
	if errors.Is(mismatch, ErrWrongRadio) {
		t.Error("errors.Is(err, ErrWrongRadio) = true; want false — Is must not match every sentinel")
	}
	var recovered *AnswerMismatchError[string]
	if !errors.As(fmt.Errorf("wrapped: %w", mismatch), &recovered) || recovered.Answered != "004" {
		t.Error("errors.As did not recover the concrete error with its addresses")
	}
	if errors.Is(&UnknownSettingError{Model: "ft710", ID: "x"}, ErrAnswerMismatch) {
		t.Error("an unknown-setting error must not match the answer-mismatch sentinel")
	}
}

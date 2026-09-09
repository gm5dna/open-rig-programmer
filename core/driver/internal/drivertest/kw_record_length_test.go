// SPDX-License-Identifier: GPL-3.0-or-later

package drivertest

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// TestAssertKenwoodMAFrameLengthMismatch_AcceptsAndRefuses pins the
// range-shaped helper both ways at once, which is the only way an assertion
// helper can be tested: a helper that never fails passes every caller's test
// vacuously, and a helper that always fails is caught by its first call site.
//
// The fixtures are the two MA-family length refusals this helper is written
// for, quoted from core/kw/ma verbatim so a reworded codec message shows up
// here rather than in two driver packages: the TS-890S's RANGE (codec890.go's
// "runs 40 to 50") and the TS-990S's EQUALITY (codec990.go's "is exactly
// 57"). Only their shared opening is asserted — see the helper.
func TestAssertKenwoodMAFrameLengthMismatch_AcceptsAndRefuses(t *testing.T) {
	ma890 := &kw.ParseError{
		Frame:  []byte("MA0000;"),
		Reason: "MA0 answer: the frame is 7 bytes, and the TS-890S grid runs 40 to 50 — thirty-nine printed positions, a name of up to ten characters (890:3208-3209) and a terminator that floats at 40 + len(name) (890:3181-3182); the 40-byte minimum is A17",
	}
	ma990 := &kw.ParseError{
		Frame:  []byte("MA0000;"),
		Reason: "MA0 answer: the frame is 7 bytes, and the TS-990S grid is exactly 57 — eighteen parameters, a ten-byte name window at 47-56 and ';' nailed to 57 (990:2919-2938)",
	}

	for name, err := range map[string]error{
		"890S range": ma890,
		"990S exact": ma990,
	} {
		rec := &errorRecorder{TB: t}
		AssertKenwoodMAFrameLengthMismatch(rec, err, "MA0 answer", 7)
		if len(rec.errs) != 0 {
			t.Errorf("%s: helper reported %v on a conforming refusal", name, rec.errs)
		}
	}

	for name, err := range map[string]error{
		// Not this family's error at all: a bare error carries neither
		// the sentinel a caller classifies on nor a Reason to read.
		"untyped": errors.New("MA0 answer: the frame is 7 bytes, and the TS-890S grid runs 40 to 50"),
		// The right type, the wrong measured length — the mistake the
		// helper exists to catch, since a codec that reported the WANTED
		// width where the MEASURED one belongs would look identical to a
		// caller checking only that some refusal happened.
		"wrong length": &kw.ParseError{Reason: "MA0 answer: the frame is 41 bytes, and the TS-890S grid runs 40 to 50"},
		// The right type and length under another frame's name.
		"wrong command": &kw.ParseError{Reason: "EX answer: the frame is 7 bytes, and the TS-890S grid runs 40 to 50"},
	} {
		rec := &errorRecorder{TB: t}
		AssertKenwoodMAFrameLengthMismatch(rec, err, "MA0 answer", 7)
		if len(rec.errs) == 0 {
			t.Errorf("%s: helper reported nothing on a non-conforming refusal", name)
		}
	}
}

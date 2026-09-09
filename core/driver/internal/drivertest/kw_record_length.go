// SPDX-License-Identifier: GPL-3.0-or-later

package drivertest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// AssertKenwoodRecordLengthMismatch pins the record-length refusal contract
// for the Kenwood family: a caller can CLASSIFY the failure, RECOVER the
// codec's two measured lengths, and the user-facing text is EXACT.
//
// IT IS A SIBLING OF AssertRecordLengthMismatch AND NOT A REPLACEMENT FOR IT,
// and the two cannot be one function. That helper asserts CI-V's contract —
// errors.Is(err, driver.ErrWrongRadio) and a *civ.RecordLengthError — because
// on the Icom side a mis-sized record is a PROBE-TIME RADIO CLASSIFICATION: a
// frame of the wrong width says the thing on the other end of the cable is a
// different radio. A Kenwood short "MR…" is neither of those things. It is a
// malformed memory frame on the READ path, typed *kw.RecordLengthError and
// wrapping kw.ErrParse, and it says nothing about which radio answered.
// Calling the CI-V helper from a Kenwood driver would make that driver import
// core/civ and claim "wrong radio" for a frame width, which is false.
//
// WHY IT LIVES HERE AND NOT IN A DRIVER PACKAGE. The plan's L5 obligation
// asked core/driver/ts590 to consume the fleet helper; T11 established that it
// could not, and the review RULED that the obligation is discharged by a
// Kenwood-shaped sibling landing at T14 — the first point at which TWO drivers
// need it. core/driver/ts590 and core/driver/ts480 both call it, and neither
// keeps a private copy: a contract asserted twice in two packages is one edit
// from being asserted two different ways.
//
// core/kw cannot call it either, and for a different reason again: this
// package is core/driver/internal/…, which Go's internal rule makes reachable
// only from within core/driver. core/kw/record_test.go therefore keeps its own
// in-package assertion, and core/kw/parse.go's RecordLengthError doc comment
// records both facts.
//
// AssertKenwoodMAFrameLengthMismatch is the RANGE-SHAPED SIBLING of
// AssertKenwoodRecordLengthMismatch, for the MA-family memory frame — the
// first frame in this repository whose length is not a constant.
//
// WHY IT CANNOT BE THE FUNCTION ABOVE. That one asserts a *kw.RecordLengthError
// carrying GOT and WANT, and its message quotes both: on the 590 pair every
// memory frame is exactly kw.RecordLen bytes, so a width refusal has one
// number to be wrong against. A TS-890S MA0 answer is 40 to 50 bytes, because
// its terminator floats at 40 + len(name) (890:3181-3182), so there is no
// single WANT to pass; core/kw/ma therefore refuses with a plain *kw.ParseError
// whose Reason spells its own book's bound in prose. Handing a range to the
// function above as want would mean choosing one end of it and asserting a
// number the codec never printed.
//
// WHAT IS ASSERTED IS THEREFORE THE SHARED HALF AND NOT THE PROSE: the failure
// is CLASSIFIABLE (kw.ErrParse), RECOVERABLE as a *kw.ParseError so a caller
// can read the frame back, and its Reason OPENS by naming the frame and the
// MEASURED length. The two codecs' bounds clauses differ in kind — the
// TS-890S's is a range and the TS-990S's an equality — and pinning either
// here would make one row's helper the other's, which is the sibling
// inheritance this milestone's per-row rule exists to prevent. Each driver
// pins its own bound clause in its own package.
//
// CONSUMED BY core/driver/ts890 AND core/driver/ts990 ONLY. core/kw/ma keeps
// its own in-package assertions and does not migrate to this one: this
// package is core/driver/internal/…, which Go's internal rule makes reachable
// only from within core/driver, exactly as the paragraph above records for
// core/kw.
//
// what is the frame name the codec's message opens with ("MA0 answer"); got
// is the measured width.
func AssertKenwoodMAFrameLengthMismatch(t testing.TB, err error, what string, got int) {
	t.Helper()
	if !errors.Is(err, kw.ErrParse) {
		t.Errorf("errors.Is(err, kw.ErrParse) = false for %v", err)
	}
	var parseErr *kw.ParseError
	if !errors.As(err, &parseErr) {
		// Errorf and not Fatalf: this helper is itself under test
		// (kw_record_length_test.go), and a Fatalf would abort the test
		// checking that it refuses rather than record the refusal.
		t.Errorf("errors.As(err, **kw.ParseError) = false for %v", err)
		return
	}
	want0 := fmt.Sprintf("%s: the frame is %d bytes, ", what, got)
	if !strings.HasPrefix(parseErr.Reason, want0) {
		t.Errorf("ParseError.Reason = %q, want it to open with %q", parseErr.Reason, want0)
	}
}

// command is the two-letter frame name ("MR" or "MW"); got and want are the
// measured and the printed widths.
func AssertKenwoodRecordLengthMismatch(t testing.TB, err error, command string, got, want int) {
	t.Helper()
	if !errors.Is(err, kw.ErrParse) {
		t.Errorf("errors.Is(err, kw.ErrParse) = false for %v", err)
	}
	var lengthErr *kw.RecordLengthError
	if !errors.As(err, &lengthErr) {
		t.Fatalf("errors.As(err, **kw.RecordLengthError) = false for %v", err)
	}
	if lengthErr.Command != command || lengthErr.Got != got || lengthErr.Want != want {
		t.Errorf("kw.RecordLengthError = %s %d/%d, want %s %d/%d", lengthErr.Command, lengthErr.Got, lengthErr.Want, command, got, want)
	}
	// THE TEXT IS CHECKED BY PREFIX AND NOT BY EQUALITY, which is the one
	// place this helper's contract is looser than the CI-V one's. kw's
	// message ends with a %q-quoted copy of the offending frame — radio
	// bytes, deliberately truncated and deliberately quoted — so an exact
	// comparison would make every caller restate fifty bytes of fixture. The
	// half that matters is the opening, which is where the two measured
	// lengths are.
	want0 := fmt.Sprintf("kw: %s memory frame is %d bytes, want exactly %d bytes", command, got, want)
	if text := lengthErr.Error(); !strings.HasPrefix(text, want0) {
		t.Errorf("Error() = %q, want it to open with %q", text, want0)
	}
}

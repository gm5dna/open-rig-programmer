// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"fmt"
	"strings"
)

// KindMismatchError reports that an MR answer's P7 kind byte was not one
// of this radio's accepted read-side values.
//
// Five Yaesu packages (ft450d, ft950, ftdx1200, ftdx3000, ft2000) minted
// byte-identical copies of this shape and its Error() wording, differing
// only in the "Model:" prefix each package's own literal supplied —
// folded into a field here rather than left as five copies. ft710's own
// KindMismatchError is richer (a sentinel, Unwrap, and a
// single-vs-multi-Want distinction the live P1L failure needed) and is
// NOT this type; it stays where it is.
type KindMismatchError struct {
	// Model is the radio the refusal came from, e.g. "ft450d".
	Model string
	// Slot is the canonical wire-form slot that was read.
	Slot string
	// Got is the P7 kind byte the answer carried.
	Got byte
	// Want lists every kind byte this radio's read side accepts.
	Want []byte
}

// Error implements the error interface.
func (e *KindMismatchError) Error() string {
	want := make([]string, len(e.Want))
	for i, k := range e.Want {
		want[i] = fmt.Sprintf("%q", rune(k))
	}
	return fmt.Sprintf("%s: MR answer for slot %q carries kind %q, want one of {%s}", e.Model, e.Slot, rune(e.Got), strings.Join(want, ","))
}

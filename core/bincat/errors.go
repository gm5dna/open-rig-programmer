// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"errors"
	"fmt"
)

// ErrFrame is the sentinel every frame build/parse failure wraps: this
// family has no preamble and no terminator, so a wrong byte count is the
// entire structural check a frame gets.
var ErrFrame = errors.New("bincat: frame")

// ErrBCD is the sentinel every packed-BCD failure wraps: a value too wide
// for its field, a field width this package will not build, or a wire
// byte carrying a nibble above 9 (the latter surfaces via civ.DecodeBCD2,
// re-wrapped so a bincat caller need match only this one sentinel).
var ErrBCD = errors.New("bincat: packed BCD")

// ErrInvalidProfile is the sentinel every Profile construction/validation
// failure wraps, so a caller can tell a malformed model table from any
// other error without matching message text — core/civ's ErrInvalidProfile
// precedent, restated rather than imported (siblings, not relatives).
var ErrInvalidProfile = errors.New("bincat: invalid profile")

func invalidProfile(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidProfile, fmt.Sprintf(format, args...))
}

// ErrRecord is the sentinel every record-decode failure wraps.
var ErrRecord = errors.New("bincat: record")

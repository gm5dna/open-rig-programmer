// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import "errors"

// ErrImageIncomplete is the sentinel every failed-receive path wraps: a
// deadline (start, inter-block or total) expired, a block's checksum
// failed, or bytes arrived beyond the matched schedule's last block
// (trailing bytes). Per spec.md §Read model point 5, every one of these is
// a full refusal of the whole image — never a partial Image.
var ErrImageIncomplete = errors.New("clonewire: image incomplete")

// ErrImageIncompatible is returned when a fully-received image's length
// matches none of the candidate Profiles offered to Receive — the
// candidate set does not describe this radio's image at all
// (spec.md §Identity probe).
var ErrImageIncompatible = errors.New("clonewire: image length matches no candidate profile")

// ErrImageAmbiguous is returned when a fully-received image's length
// matches more than one candidate Profile — spec.md §Identity probe
// requires this be reported, not silently resolved by picking one. This is
// NOT driver.ErrWrongRadio, which assumes a live per-session probe reply
// this family has no equivalent of.
var ErrImageAmbiguous = errors.New("clonewire: image length matches more than one candidate profile")

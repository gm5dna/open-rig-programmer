// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic9100 implements safe IC-9100 memory cloning over CI-V.
//
// The driver is manual-derived and unverified on hardware
// (docs/superpowers/icom-matrices/ic9100-capability-matrix.md, §0). Erase
// is never consented, writes are never retransmitted, and Open sends no
// mutation.
//
// # The headline finding
//
// CI-V address 7Ch, not the 88h/E0h pair the wave's spec.md §1 and this
// package's own dispatch title assumed — see core/civ/ic9100/doc.go and
// matrix §3.4 for the full derivation. Capabilities.CATID and the probe's
// static address are both "7C".
//
// # D-STAR bytes carry no neutral field
//
// The record's digital squelch byte, its second "digital code squelch"
// byte and its three 8-byte D-STAR call signs have no spec.Field anywhere
// in this codebase — the identical situation the IC-7100 driver's own
// caps.go/write.go record for its D-STAR family sibling. TestFieldAudit
// CoversEverySpecField pins that every spec.Field is either mapped or
// given a reason; write.go's unmappedRegions is the E6-style
// preserve-or-refuse list for these five regions plus the split flag.
//
// # No TX-duplicate block, so no tx_frequency
//
// Unlike IC-7100/IC-7700 in the same v1.7.0 wave, this record carries no
// TX-side frequency block (matrix §1b); spec.FieldTxFrequency is
// Unsupported and write.go's conditionalWriteFields still checks for it,
// so a caller-supplied Known TxFreqHz is refused by name rather than
// silently dropped.
//
// # The UX-9100 4th band is deferred
//
// Groups is 3, not 4: core/civ/ic9100/doc.go and this package's own
// caps.go (maxFreqHz) explain why the optional UX-9100's 1200 MHz band is
// not modelled this cycle — its own frequency-field encoding is UNRESOLVED
// (matrix §1 row 16 / §3.15(6)).
package ic9100

// SPDX-License-Identifier: GPL-3.0-or-later

// Package csvio reads and writes memory-channel data as CSV, for two
// distinct purposes:
//
//   - Export/Import is this project's OWN CSV schema — a lossless,
//     round-trippable representation of []codeplug.Channel meant for
//     editing a channel list in a spreadsheet. Export always writes one
//     row per slot, including empty slots, so a full radio image
//     round-trips exactly: Import(Export(channels)) reproduces channels
//     field for field, including every FieldState. This is a closed
//     loop between this package's own two functions; it makes no claim
//     about any other CSV dialect.
//
//   - ImportCHIRP is a best-effort MIGRATION path from CHIRP-next's CSV
//     export format, which models a materially different (and in places
//     incompatible) radio: fields the FT-710 has no equivalent for are
//     dropped, values that need reshaping to fit are approximated, and
//     values with no safe interpretation at all are flagged blocking.
//     Every one of these is recorded as a LossEntry in the returned
//     LossReport — nothing is ever silently discarded. See ImportCHIRP's
//     doc comment for the exact contract: it always returns the fullest
//     Channels and LossReport it can build, and it is the CALLER's job
//     to check LossReport.HasBlocking() before treating the result as
//     usable for a send — this project never sends data to a radio that
//     was imported with an unresolved loss.
//
// Both import paths are SYNTACTIC ONLY: they turn well-formed CSV cells
// into codeplug.Channel/ChannelData values without judging whether those
// values make sense for any particular radio. Neither consults
// codeplug.Validate or a spec.Capabilities, and successfully imported
// data is not thereby guaranteed valid — codeplug.Validate remains the
// one semantic gate a caller must run before treating any codeplug
// (regardless of its origin) as ready to send. This package does not
// duplicate any of Validate's rules.
//
// Dependency rule: this package imports core/codeplug, core/spec, and
// the standard library only — never core/cat. It has no knowledge of
// the CAT wire protocol or any live radio I/O.
package csvio

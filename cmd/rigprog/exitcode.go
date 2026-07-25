// SPDX-License-Identifier: GPL-3.0-or-later

package main

// Exit codes rigprog returns. THIS TABLE IS A STABLE, SCRIPT-FACING
// CONTRACT: black-box tests assert these exact numbers, and every
// subcommand this project ever adds must conform to it rather than invent
// its own. Do not renumber or repurpose an existing code — extend an
// existing class instead.
const (
	// exitSuccess means the command completed.
	exitSuccess = 0
	// exitError means I/O, radio comms, wrong radio, or an unexpected
	// failure.
	exitError = 1
	// exitUsage means an unknown subcommand/flag, or missing/
	// contradictory arguments.
	exitUsage = 2
	// exitBlocked means blocking validation issues, blocking CHIRP loss
	// entries, or (task-25 brief, adjudicated remedy) a write plan whose
	// only pending changes are ALL Blocked — most often a channel delete,
	// since the FT-710 has no CAT erase command. That last case is
	// deliberately NOT exitSuccess: "nothing to send" is reserved for a
	// working copy that genuinely matches the radio (see runWrite/
	// writeNothingSendableReport, write.go) — a blocked-only plan means
	// real pending edits could not be honoured, which callers must be able
	// to detect from the exit code alone, not just by parsing stdout.
	exitBlocked = 3
	// exitRefused means a send was refused before any write reached the
	// radio: stale baseline, session changed, confirmation mismatch or
	// declined, or firmware unconfirmed.
	exitRefused = 4
	// exitAborted means a transfer was aborted after at least one write
	// attempt reached the radio — see the run's journal.
	exitAborted = 5
)

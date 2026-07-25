// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
)

// newInterruptContext returns a context cancelled on the process's first
// Ctrl-C (SIGINT), together with the context.CancelFunc a caller MUST
// defer-call to stop listening once the command it guards has finished —
// this is the "shared command scaffolding" task-12 brief §1 asks for so
// every radio-touching subcommand (probe, read, diff, and later write)
// cancels its in-flight ReadAll/PrepareSend/Execute the same way, rather
// than each reinventing signal wiring. export/import (task 13) are
// OFFLINE — they never open a radio session, so they have no in-flight
// operation to cancel and do not use this.
func newInterruptContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}

// isCancelled reports whether err is, or wraps (at any depth), either
// context.Canceled (newInterruptContext firing — Ctrl-C) or
// context.DeadlineExceeded. Both are treated identically by every
// radio-touching subcommand: a plain, quiet "cancelled" message rather
// than the generic error path.
func isCancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

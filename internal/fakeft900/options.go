// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft900

import "time"

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// WithLatency makes every ORDINARY reply (everything except a chunked
// full-dump — see WithChunkedFullDump) wait d before being written to the
// port. The wait is interruptible: a Close mid-wait abandons the reply.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.pipe.Latency = d }
}

// WithChunkedFullDump makes a U=uFullDump Status Update reply arrive as n
// separate writes, each preceded by a sleep of delay, instead of one
// instantaneous write. n*delay is the reply's total elapsed time — set it
// past a caller's read timeout to exercise that timeout against a fake
// that is still genuinely, slowly, sending real bytes, rather than one
// that never answers at all. n<=1 (including the zero value) disables
// chunking: the default, and every reply besides U=0, behaves as one
// ordinary write.
func WithChunkedFullDump(n int, delay time.Duration) Option {
	return func(r *Radio) {
		r.chunkFullDumpN = n
		r.chunkDelay = delay
	}
}

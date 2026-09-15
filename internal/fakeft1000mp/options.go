// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft1000mp

import "time"

// WithLatency makes every reply the fake sends wait d before being
// written to the port. The wait is interruptible — a Close mid-wait
// abandons the reply (Radio.pipe's own promptness guarantee). Does not
// apply to a chunked full dump — see WithFullDumpChunking's own gap.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.pipe.Latency = d }
}

// WithFullDumpChunking makes a U=00H Status Update reply arrive as
// several separate writes of at most chunkBytes each, gap apart, instead
// of one. This package's own test hook (doc.go ASSUMED #4, not a manual
// figure) for exercising a caller's own long-read timeout — e.g.
// chunkBytes=400, gap=300ms delivers the 1,863-byte dump over five writes
// spanning about 1.2s, comfortably past transport.DefaultTimeout's 1s.
func WithFullDumpChunking(chunkBytes int, gap time.Duration) Option {
	return func(r *Radio) {
		r.fullDumpChunk = chunkBytes
		r.fullDumpGap = gap
	}
}

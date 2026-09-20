// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft1000mp

import "time"

// WithLatency makes every reply the fake sends wait d before being
// written to the port. The wait is interruptible — a Close mid-wait
// abandons the reply (Radio.pipe's own promptness guarantee).
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.pipe.Latency = d }
}

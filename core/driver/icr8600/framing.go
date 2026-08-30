// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import "github.com/gm5dna/open-rig-programmer/core/driver"

// StopBits reports the ASSUMED 8-N-1 CI-V framing. Register
// icr8600-serial-framing is lifted at Stage R by trying 19 00 at 8-N-1 and
// 8-N-2 on an IC-R8600 and recording which produces a clean reply. The guide
// contains no CI-V framing statement. core/serial already drives RTS/DTR low;
// icr8600-control-lines remains an assumption and this driver changes no line.
func (d *icr8600Driver) StopBits() int { return 1 }

var _ driver.SerialFramingReporter = (*icr8600Driver)(nil)

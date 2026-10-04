// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import "github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"

func recordIsAbsent(raw []byte) bool { return icom.RecordIsAbsent(raw) }

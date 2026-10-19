// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import "github.com/gm5dna/open-rig-programmer/core/cat"

// held is the FTdx101D/MP's held-read-only set: rows that are not denied but
// must not be written on paper evidence alone (spec v2 §2.2). It is the ONLY
// place holds live. Each value is the reason plus the line in
// docs/fixtures-private/manuals/ftdx101_layout.txt.
var held = map[cat.EXAddress]string{
	{P1: 1, P2: 5, P3: 14}: "SHIFT FREQUENCY: legend prints 1: twice and no 0, the typo hardware corrected on the FT-710 (layout 793)",
	{P1: 1, P2: 5, P3: 13}: "MARK FREQUENCY: legend identical to the FT-710 print where code 2 was refused (layout 792)",
	{P1: 3, P2: 1, P3: 23}: "KEYBOARD LANGUAGE: code 11 LEVEL is a stray from the neighbouring legend (layout 879-883)",
	{P1: 2, P2: 1, P3: 12}: "CW BK-IN DELAY: duplicated 05:250 and an ellipsis hiding codes 08-30, no domain can be built (layout 825-827)",
	{P1: 3, P2: 1, P3: 14}: "QMB CH: slot addressing unproven off the FT-710 (layout 867)",
	{P1: 3, P2: 1, P3: 15}: "MEM GROUP: slot addressing unproven off the FT-710 (layout 869)",
	{P1: 3, P2: 4, P3: 5}:  "VOX SELECT: selects the VOX source, MIC or DATA (layout 933)",
	{P1: 3, P2: 4, P3: 6}:  "DATA VOX GAIN: can turn existing data audio into an unintended transmission (layout 934)",
}

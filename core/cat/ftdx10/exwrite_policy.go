// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import "github.com/gm5dna/open-rig-programmer/core/cat"

// held is the FTdx10's held-read-only set: rows that are not denied but must
// not be written on paper evidence alone (spec v2 §2.2). It is the ONLY place
// holds live. Each value is the reason plus the line in
// docs/fixtures-private/manuals/ftdx10_layout.txt.
var held = map[cat.EXAddress]string{
	{P1: 1, P2: 5, P3: 16}: "SHIFT FREQUENCY: legend prints 1: twice and no 0, the typo hardware corrected on the FT-710 (layout 755)",
	{P1: 1, P2: 5, P3: 15}: "MARK FREQUENCY: legend identical to the FT-710 print where code 2 was refused (layout 754)",
	{P1: 1, P2: 3, P3: 20}: "TONE FREQ: the FT-710 row of this shape printed Digits 2 and accepted 3 (layout 710)",
	{P1: 3, P2: 1, P3: 20}: "KEYBOARD LANGUAGE: code 11 LEVEL is a stray from the neighbouring legend (layout 825-829)",
	{P1: 3, P2: 1, P3: 11}: "QMB CH: slot addressing unproven off the FT-710 (layout 814)",
	{P1: 3, P2: 1, P3: 12}: "MEM GROUP: slot addressing unproven off the FT-710 (layout 815)",
	{P1: 3, P2: 4, P3: 5}:  "VOX SELECT: selects the VOX source, MIC or DATA (layout 875)",
	{P1: 3, P2: 4, P3: 6}:  "DATA VOX GAIN: can turn existing data audio into an unintended transmission (layout 876)",
}

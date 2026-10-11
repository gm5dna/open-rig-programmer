// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
)

// AnswerMismatches reports how many memory answers this session has seen
// whose decoded channel address was not the one requested (tier ruling
// T2). A diagnostic count beside the typed error, so a bus that
// occasionally mis-attributes is visible even when each individual read
// was refused correctly.
func (s *Session) AnswerMismatches() uint64 { return s.answerMismatches.Load() }

// ReadChannel implements driver.Session: ONE 1A 00 read, mapped into one
// codeplug.Channel by the shared engine.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	return icom.ReadChannel(ctx, &params, s.eng, s.caps, &s.answerMismatches, slot)
}

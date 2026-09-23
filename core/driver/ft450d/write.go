// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// WriteChannel implements driver.Session: a single MW Set, fire-and-forget
// with the transport's bounded "?;" listen, via the shared
// core/driver/internal/yaesu body. THIS RADIO HAS NO MT COMMAND (matrix
// §0), so there is no second frame to sequence beside it, and MW's own P7
// is write-fixed (dialect.go's MWWriteKind) rather than a
// session-supplied kind — both already true of the shared body's own
// shape.
//
// A write to a PMS slot (501-504) is refused by the shared body's ordinary
// capability gate, not by a bank-specific branch here: s.caps.FieldSupport
// (spec.BankPMS, f) reports Write: Unsupported for every one of the six
// requested fields (caps.go's pmsFields), so the shared body's "unwritable"
// check is non-empty for any populated PMS write, on both profiles,
// unconditionally (the SAFE SHAPE ruling's caution, not a
// hardware-verification state consent could open).
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	return yaesu.MRWriteChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, ch)
}

// buildMWCommand maps a populated channel onto its MW Set frame via the
// shared body. Kept as a Session method (rather than inlined at its only
// call site) because write_test.go exercises it directly.
func (s *Session) buildMWCommand(ch codeplug.Channel) (cat.Command, error) {
	return yaesu.BuildMWCommand(s.dialect, s.caps, &mrParams, ch)
}

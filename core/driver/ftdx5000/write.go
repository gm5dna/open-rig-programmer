// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// WriteChannel implements driver.Session: a single, fire-and-forget MW
// frame, via the shared core/driver/internal/yaesu body. THIS RADIO HAS
// NO MT COMMAND (matrix §2; doc.go), so — same as ftdx9000 — this write is
// a single plain MW (yaesu.MRWriteChannel/BuildMWCommand, not
// yaesu.WriteChannel/BuildWriteCommand, which build only the combined MT
// form the 4-rig family uses).
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	return yaesu.MRWriteChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, ch)
}

// buildMWCommand maps a populated channel onto its MW Set frame via the
// shared body. Kept as a Session method (rather than inlined at its only
// call site) so write_test.go can exercise it directly — the same shape
// every migrated sibling driver's write.go keeps.
func (s *Session) buildMWCommand(ch codeplug.Channel) (cat.Command, error) {
	return yaesu.BuildMWCommand(s.dialect, s.caps, &mrParams, ch)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx3000

import (
	"context"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
)

// WriteChannel implements driver.Session: a single MW Set, fire-and-forget
// with the transport's bounded "?;" listen, via the shared
// core/driver/internal/yaesu body. A Known CTCSSTone is refused by the
// shared body's own capability gate (mrParams's doc comment, read.go):
// this radio's caps.go marks FieldCTCSSTone write-Unsupported, so no
// wire traffic goes out before that refusal fires.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	return yaesu.MRWriteChannel(ctx, s.eng, s.dialect, s.caps, &mrParams, ch)
}

// buildMWCommand maps a populated channel onto its MW Set frame via the
// shared body. Kept as a Session method (rather than inlined at its only
// call site) because write_test.go exercises it directly. Never reached
// with a Known CTCSSTone: WriteChannel's own capability gate always
// refuses that first, so ToneIndex is always the shared body's zero
// value here — exactly what MW's printed-fixed "00" write requires
// (dialect.go's P9ToneIndexReadOnly).
func (s *Session) buildMWCommand(ch codeplug.Channel) (cat.Command, error) {
	return yaesu.BuildMWCommand(s.dialect, s.caps, &mrParams, ch)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestWriteChannel_TimeoutQuarantinesTheLateAcknowledgement is the half of
// the ack-timeout story the test above cannot tell.
//
// One transmission and no retry is what the timeout test proves. It says
// nothing about the FB that turns up AFTER the caller has been told the
// outcome is unknown, and that frame is the dangerous one: it belongs to a
// transaction that is over, and the very next exchange is waiting for an
// answer of its own. If it were left on the line, a read could return
// somebody else's frame, or an FB could be read as a rejection-free
// success of a command that never got one.
//
// So this issues a SUBSEQUENT operation. The write times out; the post-write
// quarantine drain runs unconditionally and consumes the late FB; the read
// that follows gets its own answer and the right record.
func TestWriteChannel_TimeoutQuarantinesTheLateAcknowledgement(t *testing.T) {
	img := fixture.AckingRadio()
	img.AckSets = false
	img.LateSetAck = transport.DefaultTimeout + 100*time.Millisecond
	img.Records[42] = goldenRecord
	p := newScriptedPort(t, img)
	s := openWith(t, p)
	openFrames := len(p.Transcript())

	unexpectedBefore := s.eng.UnexpectedFrames()
	if _, err := s.WriteChannel(t.Context(), goodChannel("042")); err == nil {
		t.Fatal("WriteChannel succeeded; the acknowledgement arrived after the answer wait had expired")
	}

	// THE SUBSEQUENT OPERATION. It must get its OWN answer.
	ch, err := s.ReadChannel(t.Context(), "042")
	if err != nil {
		t.Fatalf("the read after a quarantined write failed: %v", err)
	}
	if ch.Empty() || ch.Data.FreqHz != 14_250_000 || ch.Data.Tag != "HOME QTH01" {
		t.Errorf("the read after a quarantined write returned %+v, want channel 42's own record", ch.Data)
	}
	if n := s.AnswerMismatches(); n != 0 {
		t.Errorf("AnswerMismatches() = %d, want 0 - no exchange may have been served another transaction's frame", n)
	}

	// THE LATE FB WAS REALLY ON THE LINE, and was accounted for as
	// unexpected rather than consumed as an answer. Without this the test
	// would pass just as happily against a radio that never acked at all,
	// which is the case the timeout test above already covers.
	if got := s.WireStats().Acknowledgements; got != 1 {
		t.Errorf("the adapter saw %d acknowledgement frames, want 1 - the late FB never arrived and this test proves nothing", got)
	}
	if got := s.eng.UnexpectedFrames(); got != unexpectedBefore+1 {
		t.Errorf("Engine.UnexpectedFrames() went %d -> %d, want exactly one more - the late FB must be discarded as belonging to a finished transaction, never read as the next exchange's answer", unexpectedBefore, got)
	}

	// Still exactly one set frame: the quarantine is not a retry.
	var sets int
	for _, f := range fixture.FramesAfterOpen(t, p, openFrames) {
		if len(f) >= memSetFrameLen {
			sets++
		}
	}
	if sets != 1 {
		t.Errorf("%d set frames reached the port, want exactly 1", sets)
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7700 "github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func TestStopBits(t *testing.T) {
	for _, d := range []driver.Driver{New(RealHardware), New(Simulated)} {
		r, ok := d.(driver.SerialFramingReporter)
		if !ok || r.StopBits() != 1 {
			t.Fatalf("%s does not report 8-N-1", d.Model())
		}
	}
}

func TestProfileProbeShape(t *testing.T) {
	p := civic7700.Profile()
	if p.RadioAddress() != 0x74 || p.BuildRecordLength() != civic7700.RecordOnlyLength || len(p.RecordLengths()) != 1 || !p.AcceptsRecordLength(civic7700.RecordOnlyLength) {
		t.Fatalf("probe shape: address=%x length=%d records=%v", p.RadioAddress(), p.BuildRecordLength(), p.RecordLengths())
	}
	if p.ControllerAddress() != civ.ControllerAddressDefault {
		t.Fatalf("controller address = %#02x", p.ControllerAddress())
	}
}

// TestOpen_ControlLinesAreNeverToggled is a HAZARD test: matrix §3.2 finds
// no printed statement that [RS-232C]'s RTS/DTR carry any PTT or keying
// function on this radio (unlike the USB-CDC ports on this tier's other
// members), and this project's own CHOICE is to assert no such expectation
// onto either line regardless. transport.OpenSerial drives both low before
// this driver ever sees the port, and THIS DRIVER NEVER TOUCHES EITHER
// AGAIN.
func TestOpen_ControlLinesAreNeverToggled(t *testing.T) {
	port := newScriptedPort(t, occupiedImage(t))
	sess, err := New(RealHardware).Open(t.Context(), port.Port(), driver.Identity{Port: "/dev/fake"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	if _, err := sess.ReadChannel(t.Context(), "001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	rts, dtr := port.controlLineCalls()
	if rts != 0 || dtr != 0 {
		t.Errorf("the driver reached for a modem control line (rts=%d dtr=%d); it must never assert RTS or DTR", rts, dtr)
	}
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"net"
	"sync/atomic"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7851"
)

func TestStopBits(t *testing.T) {
	for _, d := range []driver.Driver{New7851(), New7850(), New7851(WithSimulatedProfile())} {
		r, ok := d.(driver.SerialFramingReporter)
		if !ok || r.StopBits() != 1 {
			t.Fatalf("%s does not report 8-N-1", d.Model())
		}
	}
}

func TestProfileProbeShape(t *testing.T) {
	p := civic7851.Profile()
	if p.RadioAddress() != 0x8e || p.BuildRecordLength() != 25 || len(p.RecordLengths()) != 1 || !p.AcceptsRecordLength(25) {
		t.Fatalf("probe shape: address=%x length=%d records=%v", p.RadioAddress(), p.BuildRecordLength(), p.RecordLengths())
	}
	if p.ControllerAddress() != civ.ControllerAddressDefault {
		t.Fatalf("controller address = %#02x", p.ControllerAddress())
	}
}

// controlLinePort is a transport.Port that ALSO offers the two modem
// control lines, and records any use of them.
type controlLinePort struct {
	net.Conn
	touched atomic.Int64
}

func (p *controlLinePort) SetRTS(bool) error { p.touched.Add(1); return nil }
func (p *controlLinePort) SetDTR(bool) error { p.touched.Add(1); return nil }

// TestOpen_ControlLinesAreNeverToggled is a HAZARD test, not a protocol
// one.
//
// On this radio either USB endpoint's DTR or RTS can be assigned to PTT
// (USB SEND) or to CW/RTTY keying, and the document never says which
// endpoint carries CI-V — so a driver that asserted either line on opening
// the CI-V port could key the transmitter of a radio whose owner has made
// that assignment. transport.OpenSerial drives both low before this driver
// sees the port, and THIS DRIVER NEVER TOUCHES EITHER.
//
// transport.Port is an io.ReadWriteCloser and carries neither method, so
// the only route to one is a type assertion. This port offers both; the
// test exists to make such an assertion visible the moment anyone writes
// one.
func TestOpen_ControlLinesAreNeverToggled(t *testing.T) {
	r := fakeic7851.New()
	defer func() { _ = r.Close() }()
	r.SetSlot("001", e2eSeed(1).record(t))

	port := &controlLinePort{Conn: r.Port()}
	sess, err := New7851().Open(t.Context(), port, driver.Identity{Port: "/dev/fake"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	if _, err := sess.ReadChannel(t.Context(), "001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if n := port.touched.Load(); n != 0 {
		t.Errorf("the driver reached for a modem control line %d time(s); it must never assert RTS or DTR", n)
	}
}

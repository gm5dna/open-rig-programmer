// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import (
	"net"
	"sync/atomic"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7410 "github.com/gm5dna/open-rig-programmer/core/civ/ic7410"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func TestStopBits(t *testing.T) {
	for _, d := range []driver.Driver{New(), New(WithSimulatedProfile())} {
		r, ok := d.(driver.SerialFramingReporter)
		if !ok || r.StopBits() != 1 {
			t.Fatalf("%s does not report 8-N-1", d.Model())
		}
	}
}

func TestProfileProbeShape(t *testing.T) {
	p := civic7410.Profile()
	if p.RadioAddress() != 0x80 || p.BuildRecordLength() != 40 || len(p.RecordLengths()) != 1 || !p.AcceptsRecordLength(40) {
		t.Fatalf("probe shape: address=%#02x length=%d records=%v", p.RadioAddress(), p.BuildRecordLength(), p.RecordLengths())
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
// one: this driver must never assert RTS or DTR on a CI-V port, on any
// radio in this tier.
func TestOpen_ControlLinesAreNeverToggled(t *testing.T) {
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{1: vector1.record(t)}}
	sp := newScriptedPort(t, img)
	port := &controlLinePort{Conn: sp.Port().(net.Conn)}

	sess, err := New(WithSimulatedProfile()).Open(t.Context(), port, driver.Identity{Port: "/dev/fake"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	if _, err := sess.ReadChannel(t.Context(), "0001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if n := port.touched.Load(); n != 0 {
		t.Errorf("the driver reached for a modem control line %d time(s); it must never assert RTS or DTR", n)
	}
}

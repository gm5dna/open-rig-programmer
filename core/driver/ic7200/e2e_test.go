// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// TestOpen_AddressMatchedIDIsRequired pins spec D3.2's opening move: what
// identifies the radio is that an ADDRESS-MATCHED 19 00 reply arrived at
// all — a reply from another station, or silence, must not open a
// session.
func TestOpen_AddressMatchedIDIsRequired(t *testing.T) {
	t.Run("address-matched answer opens", func(t *testing.T) {
		t.Parallel()
		p := newScriptedPort(t, occupiedRadio())
		s := openWith(t, p)
		if !strings.HasPrefix(s.Identity().CATID, "76") {
			t.Errorf("CATID = %q, want prefix 76", s.Identity().CATID)
		}
	})
	t.Run("silence refuses to open", func(t *testing.T) {
		t.Parallel()
		p := newScriptedPort(t, radioImage{idToken: nil})
		d := New(Simulated)
		_, err := d.Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
		if err == nil {
			t.Fatal("Open succeeded against silence, want an error")
		}
	})
}

// TestOpen_EmptyRadioOpensAnyway pins spec D3.2: a radio whose memories
// are all empty must still open, on address evidence alone.
func TestOpen_EmptyRadioOpensAnyway(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0x76}})
	s := openWith(t, p)
	length, confirmed := s.Fingerprint()
	if confirmed {
		t.Errorf("Fingerprint = %d, confirmed %v; want unconfirmed on an all-empty radio", length, confirmed)
	}
}

// TestOpen_RecordLengthMismatchDuringProbe pins that the length
// fingerprint is checked continuously from the very first probed record.
func TestOpen_RecordLengthMismatchDuringProbe(t *testing.T) {
	p := newScriptedPort(t, radioImage{
		idToken: []byte{0x76},
		records: map[int][]byte{1: goldenRecord[:len(goldenRecord)-1]},
	})
	d := New(Simulated)
	_, err := d.Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	var mismatch *RecordLengthMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("Open = %v, want *RecordLengthMismatchError", err)
	}
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Errorf("Open error does not satisfy errors.Is(err, driver.ErrWrongRadio)")
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

// TestOpen_ControlLinesAreNeverToggled pins that this driver never
// asserts RTS or DTR on the port it is handed.
//
// Matrix §3.2/ADDED-2: unlike the IC-7610 template this driver copies,
// this radio's CI-V path is a dedicated [REMOTE] jack through an
// external CT-17 to an RS-232C port, not a USB CDC endpoint sharing an
// assignable control line with PTT/keying — so the specific hazard the
// IC-7610 driver's own version of this test guards against does not
// transfer. The underlying property is still worth holding: transport.Port
// is an io.ReadWriteCloser and carries neither method, so the only route
// to one is a type assertion, and this driver must never make one.
func TestOpen_ControlLinesAreNeverToggled(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	cp := &controlLinePort{Conn: p.host}
	d := New(Simulated)
	sess, err := d.Open(t.Context(), cp, driver.Identity{Port: "/dev/fake"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	if _, err := sess.ReadChannel(t.Context(), "001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if n := cp.touched.Load(); n != 0 {
		t.Errorf("the driver reached for a modem control line %d time(s); it must never assert RTS or DTR", n)
	}
}

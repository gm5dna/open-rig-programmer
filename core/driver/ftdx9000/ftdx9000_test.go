// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// testCtx returns a context with a generous deadline, cancelled at test
// cleanup — the shared shape every sibling driver's test helper uses.
func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// openSession opens a Session against a respondingPort serving img, via the
// public Driver.Open, and fails the test on any error.
func openSession(t *testing.T, profile Profile, img slotImage) (*respondingPort, *Session) {
	t.Helper()
	p := newRespondingPort(t, img)
	d := New(profile)
	sess, err := d.Open(testCtx(t), p.Port(), driver.Identity{Port: "test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	return p, s
}

// TestNew_ProfileSelection pins Capabilities' three arms.
func TestNew_ProfileSelection(t *testing.T) {
	if got := New(Simulated).Capabilities(); got.Model != modelName {
		t.Errorf("Simulated Capabilities().Model = %q, want %q", got.Model, modelName)
	}
	if got := New(RealHardware).Capabilities(); got.NoTag != true || got.TagLen != 0 {
		t.Errorf("RealHardware Capabilities() = %+v, want NoTag/TagLen unchanged from the unverified baseline", got)
	}
	unrecognised := Profile(99)
	if got, want := New(unrecognised).Capabilities(), CapabilitiesUnverified(); got.Model != want.Model {
		t.Errorf("unrecognised Profile fell through to something other than the fail-safe baseline")
	}
}

// TestOpen_Success exercises the whole handshake against a well-behaved
// peer, on each of the three documented CAT IDs (matrix §1.2).
func TestOpen_Success(t *testing.T) {
	for _, id := range acceptedCATIDs {
		t.Run(id, func(t *testing.T) {
			p := newRespondingPort(t, slotImage{catID: id})
			d := New(Simulated)
			sess, err := d.Open(testCtx(t), p.Port(), driver.Identity{Port: "test"})
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer sess.Close()
			if got := sess.Identity().CATID; got != id {
				t.Errorf("Identity().CATID = %q, want %q", got, id)
			}
		})
	}
}

// TestOpen_WrongRadio pins the refusal when the ID probe answers with none
// of the three documented CAT IDs.
func TestOpen_WrongRadio(t *testing.T) {
	p := newRespondingPort(t, slotImage{catID: "0800"}) // an FT-710's ID
	d := New(Simulated)
	sess, err := d.Open(testCtx(t), p.Port(), driver.Identity{Port: "test"})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open with a foreign CAT ID succeeded")
	}
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want *driver.WrongRadioError", err)
	}
	if wrong.Got != "0800" {
		t.Errorf("WrongRadioError.Got = %q, want %q", wrong.Got, "0800")
	}
	if wrong.WantModel != modelName {
		t.Errorf("WrongRadioError.WantModel = %q, want %q", wrong.WantModel, modelName)
	}
}

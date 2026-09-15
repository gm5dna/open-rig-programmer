// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// statusUpdateFrame builds the wire bytes of a Status-Update memory-record
// probe request, matching bincat.BuildFrame's own argument-then-opcode
// order.
func statusUpdateFrame(ch byte) [5]byte {
	return [5]byte{bincat.UMemoryRecord, 0, 0, ch, bincat.OpStatusUpdate}
}

// blankRecord returns an arbitrary-but-valid 19-byte record: identity
// probes never inspect the content, only that a reply of the right length
// arrived (or did not).
func blankRecord() []byte { return make([]byte, 19) }

func TestOpen_FT890AgainstFT890Image_Succeeds(t *testing.T) {
	p := newScriptedPort(t)
	p.setAnswer(statusUpdateFrame(probeChannel1), blankRecord())
	// probeChannel2 (33) left unanswered: correct FT-890 behaviour.

	sess, err := NewFT890(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()
	if got := sess.Identity().CATID; got != ft890CATID {
		t.Errorf("CATID = %q, want %q", got, ft890CATID)
	}
}

func TestOpen_FT900AgainstFT900Image_Succeeds(t *testing.T) {
	p := newScriptedPort(t)
	p.setAnswer(statusUpdateFrame(probeChannel1), blankRecord())
	p.setAnswer(statusUpdateFrame(probeChannel2), blankRecord())

	sess, err := NewFT900(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()
	if got := sess.Identity().CATID; got != ft900CATID {
		t.Errorf("CATID = %q, want %q", got, ft900CATID)
	}
}

func TestOpen_FT890AgainstFT900Image_WrongRadio(t *testing.T) {
	p := newScriptedPort(t)
	p.setAnswer(statusUpdateFrame(probeChannel1), blankRecord())
	p.setAnswer(statusUpdateFrame(probeChannel2), blankRecord()) // FT-900 behaviour: answers channel 33

	_, err := NewFT890(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open error = %v, want errors.Is(_, driver.ErrWrongRadio)", err)
	}
	assertOnlyProbesSent(t, p)
}

func TestOpen_FT900AgainstFT890Image_WrongRadio(t *testing.T) {
	p := newScriptedPort(t)
	p.setAnswer(statusUpdateFrame(probeChannel1), blankRecord())
	// probeChannel2 left unanswered: FT-890 behaviour.

	_, err := NewFT900(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open error = %v, want errors.Is(_, driver.ErrWrongRadio)", err)
	}
	assertOnlyProbesSent(t, p)
}

func TestOpen_NoRadio_PlainErrorNotWrongRadio(t *testing.T) {
	p := newScriptedPort(t) // no answers configured at all

	_, err := NewFT890(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded against a silent port, want an error")
	}
	if errors.Is(err, driver.ErrWrongRadio) {
		t.Errorf("Open error = %v, want a plain transport-style error, not ErrWrongRadio (nothing has distinguished a model yet)", err)
	}
}

// assertOnlyProbesSent checks that a failed identity probe sent exactly
// the two probe frames and nothing else — no A/B-select/SetFreq/SetMode/
// Clar/Shift/Offset/Tone/Store opcode followed a mismatch (plan.md Phase
// 3 Agent A brief).
func assertOnlyProbesSent(t *testing.T, p *scriptedPort) {
	t.Helper()
	got := p.Transcript()
	want := [][5]byte{statusUpdateFrame(probeChannel1), statusUpdateFrame(probeChannel2)}
	if len(got) != len(want) {
		t.Fatalf("frames sent = %v, want exactly the two identity probes %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d = % x, want % x", i, got[i], want[i])
		}
	}
}

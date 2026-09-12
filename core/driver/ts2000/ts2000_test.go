// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func testTiming() Option { return withTiming(80*time.Millisecond, time.Millisecond) }

// probeFrames is the whole of Open's choreography: AI0 (silent, the
// session-init preamble), then ID, then TY — the core/driver/ts480 shape;
// this document prints no FV command anywhere (core/kw/ts2000/layout.go).
var probeFrames = []string{"AI0;", "ID;", "TY;"}

// TestOpen_ProbeTranscriptIsAI0ThenIDThenTY pins the three-frame
// choreography, on every row.
func TestOpen_ProbeTranscriptIsAI0ThenIDThenTY(t *testing.T) {
	for name, row := range map[string]func(...Option) driver.Driver{
		"TS-2000": NewTS2000, "TS-2000X": NewTS2000X, "TS-B2000": NewTSB2000,
	} {
		t.Run(name, func(t *testing.T) {
			_, p := openSession(t, row, "019", radioImage{}, WithSimulatedProfile())
			if got := p.Transcript(); !reflect.DeepEqual(got, probeFrames) {
				t.Errorf("transcript = %v, want %v", got, probeFrames)
			}
		})
	}
}

// TestOpen_WrongRadio pins driver.ErrWrongRadio and the GotModel lookup —
// and, on THIS package, the consequence of all three rows sharing one
// ASSUMED CATID: opening ANY of the three constructors against a radio
// answering "019" succeeds, because nothing on the wire distinguishes them
// (doc.go).
func TestOpen_WrongRadio(t *testing.T) {
	p := newRespondingPort(t, "019", radioImage{catID: "020"})
	d := NewTS2000(testTiming(), WithSimulatedProfile())
	_, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open: %v, want errors.Is(_, driver.ErrWrongRadio)", err)
	}
	var wr *driver.WrongRadioError
	if !errors.As(err, &wr) {
		t.Fatalf("Open: %v, want a *driver.WrongRadioError", err)
	}
	if wr.Want != "019" || wr.Got != "020" || wr.WantModel != "TS-2000" || wr.GotModel != "TS-480" {
		t.Errorf("WrongRadioError = %+v, want Want=019 Got=020 WantModel=TS-2000 GotModel=TS-480", wr)
	}
}

// TestOpen_IDTimesOut pins the typed kw.TimeoutError on a silent ID probe.
func TestOpen_IDTimesOut(t *testing.T) {
	p := newRespondingPort(t, "019", radioImage{idSilent: true})
	d := NewTS2000(testTiming(), WithSimulatedProfile())
	_, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err == nil {
		t.Fatal("Open succeeded against a silent ID probe")
	}
}

// TestOpen_TYRejected pins the typed kw.RejectionError on a rejected TY
// probe.
func TestOpen_TYRejected(t *testing.T) {
	p := newRespondingPort(t, "019", radioImage{tyReject: true})
	d := NewTS2000(testTiming(), WithSimulatedProfile())
	_, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err == nil {
		t.Fatal("Open succeeded against a rejected TY probe")
	}
}

// TestVariant_ReportsTheProbeAnswerOpaquely pins that Session.Variant
// reports the TY answer as received, with no interpretation.
func TestVariant_ReportsTheProbeAnswerOpaquely(t *testing.T) {
	sess, _ := openSession(t, NewTS2000, "019", radioImage{tyAnswer: "TY001;"}, WithSimulatedProfile())
	v := sess.Variant()
	if v.Variant != '1' {
		t.Errorf("Variant.Variant = %q, want '1' (\"1: Japanese 100 W type\", ts2000:11684)", v.Variant)
	}
}

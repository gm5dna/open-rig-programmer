// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"strings"
	"testing"
)

// TestBuildAIRead_AndTheOneSetState. The read is "A I ;" (590:164, 480:193)
// and the only Set this programme ever builds is "A I 0 ;" (590:160,
// 480:189) — the one value both books print identically.
func TestBuildAIRead_AndTheOneSetState(t *testing.T) {
	for _, l := range []Layout{layout590SG(), layout590S(), layout480()} {
		read, err := l.BuildAIRead()
		if err != nil {
			t.Fatalf("%s: BuildAIRead: %v", l.Model(), err)
		}
		if got := string(read.Bytes()); got != "AI;" {
			t.Errorf("%s: BuildAIRead built %q, want %q (590:164, 480:193)", l.Model(), got, "AI;")
		}
		set, err := l.BuildAISetOff()
		if err != nil {
			t.Fatalf("%s: BuildAISetOff: %v", l.Model(), err)
		}
		if got := string(set.Bytes()); got != "AI0;" {
			t.Errorf("%s: BuildAISetOff built %q, want %q (590:159-160, 480:185-189)", l.Model(), got, "AI0;")
		}
		if len(set.Bytes()) != AISetLen || len(read.Bytes()) != AIReadLen {
			t.Errorf("%s: AI frames are %d and %d bytes, want %d and %d", l.Model(), len(read.Bytes()), len(set.Bytes()), AIReadLen, AISetLen)
		}
	}
}

// TestBuildAISetOff_IsTheSAMEDATUMAsTheSessionInitFrame. framing.InitSequence
// writes AI0; at open and this builder writes AI0; on demand; two literals a
// file apart would be one edit from disagreeing, and the disagreement would
// leave a session that believed it had disabled Auto Information talking to
// a radio that had not.
func TestBuildAISetOff_IsTheSAMEDATUMAsTheSessionInitFrame(t *testing.T) {
	f, err := NewFraming(Book590)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	seq := f.InitSequence()
	if len(seq) != 1 {
		t.Fatalf("InitSequence returned %d commands, want 1", len(seq))
	}
	built, err := layout590SG().BuildAISetOff()
	if err != nil {
		t.Fatalf("BuildAISetOff: %v", err)
	}
	if got, want := string(built.Bytes()), string(seq[0].Bytes()); got != want {
		t.Errorf("BuildAISetOff built %q and InitSequence writes %q — they are one datum", got, want)
	}
}

// TestAI_NoOtherStateIsEverBuilt is the plan's negative pin. The two books'
// value sets DIFFER — 0/2/4 on the 590 pair (590:159-162) against 0/1/2/3 on
// the 480 (480:185-190) — and every non-zero value in either set turns Auto
// Information ON, which pushes unsolicited frames at a session that is
// correlating answers by prefix. There is no builder that can be asked for
// one, which is what this test asserts: the API surface, not a refusal
// inside it.
func TestAI_NoOtherStateIsEverBuilt(t *testing.T) {
	for _, l := range []Layout{layout590SG(), layout590S(), layout480()} {
		cmd, err := l.BuildAISetOff()
		if err != nil {
			t.Fatalf("%s: BuildAISetOff: %v", l.Model(), err)
		}
		body := string(cmd.Bytes())
		if !strings.HasSuffix(body, "0;") {
			t.Errorf("%s: the one AI Set this codec builds is %q, and its state byte is not '0'", l.Model(), body)
		}
	}
}

// TestAI_ZeroLayoutBuildsNothing. Both AI frames consult no radio datum, so
// without the Configured guard a zero Layout would emit a state-changing Set
// on behalf of no radio at all.
func TestAI_ZeroLayoutBuildsNothing(t *testing.T) {
	var l Layout
	if _, err := l.BuildAIRead(); err == nil {
		t.Error("a zero Layout built an AI read")
	}
	if _, err := l.BuildAISetOff(); err == nil {
		t.Error("a zero Layout built an AI Set")
	}
}

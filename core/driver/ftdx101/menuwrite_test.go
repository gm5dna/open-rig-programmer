// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	catftdx101 "github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx101"
)

// --- Menu (EX) write: consent-selected dialect, four-way table, per model ---

const (
	admittedAddr    = "010109" // SSB OUT LEVEL, Digits 3, 000..100 (layout 700s)
	deniedAddr      = "010112" // REAR SELECT, denied (spec v2 §2.1)
	heldVOXSelect   = "030405" // VOX SELECT, held (layout 933)
	heldDataVOXGain = "030406" // DATA VOX GAIN, held (layout 934)
	heldShiftFreq   = "010514" // SHIFT FREQUENCY, held (layout 793)
)

// descWrites returns every item's Write label keyed by ID.
func descWrites(d driver.SettingsDescriptor) map[string]spec.Support {
	out := map[string]spec.Support{}
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			for _, it := range g.Items {
				out[it.ID] = it.Write
			}
		}
	}
	return out
}

func countLabels(m map[string]spec.Support) map[spec.Support]int {
	out := map[spec.Support]int{}
	for _, w := range m {
		out[w]++
	}
	return out
}

// exSetFrames returns the EX Set frames (longer than the 9-byte read) a port saw.
func exSetFrames(p *respondingPort) []string {
	var out []string
	for _, f := range p.Transcript() {
		if strings.HasPrefix(f, "EX") && len(f) > exReadFrameLen {
			out = append(out, f)
		}
	}
	return out
}

// openFake opens a session of a fresh fakedx101 radio of m's model.
func openFake(t *testing.T, m testModel, profile Profile, opts ...Option) (*fakedx101.Radio, *Session) {
	t.Helper()
	newFake := fakedx101.NewD
	if m.catID == "0682" {
		newFake = fakedx101.NewMP
	}
	r := newFake()
	t.Cleanup(func() { _ = r.Close() })
	sess, err := m.newDrv(profile, opts...).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, sess.(*Session)
}

// TestMenuWrite_UnconsentedRefusesBeforeTheWire is rows (1) and (2): with no
// option, and with channel consent only (the two bits are separate), every EX
// Set is refused before a byte is sent and no descriptor item is writable.
func TestMenuWrite_UnconsentedRefusesBeforeTheWire(t *testing.T) {
	for _, m := range testModels {
		for name, opts := range map[string][]Option{
			"no options":           nil,
			"channel consent only": {WithConsentedUnverifiedWrites()},
		} {
			t.Run(m.name+"/"+name, func(t *testing.T) {
				p, sess := openSession(t, m, RealHardware, slotImage{exSetSilent: true}, opts...)
				res, err := sess.WriteSetting(testCtx(t), admittedAddr, "050")
				if err == nil {
					t.Fatalf("WriteSetting = %+v, nil; want a refusal", res)
				}
				if res != (driver.SettingWriteResult{}) {
					t.Errorf("refused result = %+v, want zero", res)
				}
				if got := exSetFrames(p); len(got) != 0 {
					t.Errorf("refusal sent EX Set frames %q, want none", got)
				}
				labels := countLabels(descWrites(sess.SettingsDescriptor()))
				if labels[spec.Unverified] != 193 || len(labels) != 1 {
					t.Errorf("descriptor labels = %v, want 193 Unverified only", labels)
				}
			})
		}
	}
}

// TestMenuWrite_ConsentedRealSession is row (3), per model.
func TestMenuWrite_ConsentedRealSession(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			r, sess := openFake(t, m, RealHardware, WithConsentedMenuWrites())

			res, err := sess.WriteSetting(testCtx(t), admittedAddr, "050")
			if err != nil {
				t.Fatalf("WriteSetting: %v", err)
			}
			if res.Outcome != driver.SettingWriteAccepted || res.Observed != "050" {
				t.Errorf("result = %+v, want Accepted with Observed 050", res)
			}
			if got, _ := r.EXState(admittedAddr); got != "050" {
				t.Errorf("fake state = %q, want 050", got)
			}

			for _, tc := range []struct{ name, id, val string }{
				{"denied", deniedAddr, "0"},
				{"held VOX SELECT", heldVOXSelect, "1"},
				{"held DATA VOX GAIN", heldDataVOXGain, "050"},
				{"held SHIFT FREQUENCY", heldShiftFreq, "1"},
				{"outside the domain", admittedAddr, "101"},
				{"wrong width", admittedAddr, "50"},
			} {
				before, _ := r.EXState(tc.id)
				res, err := sess.WriteSetting(testCtx(t), tc.id, tc.val)
				if err == nil || res != (driver.SettingWriteResult{}) {
					t.Errorf("%s: WriteSetting = %+v, %v; want a zero-result refusal", tc.name, res, err)
				}
				if tc.id != admittedAddr {
					if after, _ := r.EXState(tc.id); after != before {
						t.Errorf("%s: fake state moved %q -> %q", tc.name, before, after)
					}
				}
			}
			if got, _ := r.EXState(admittedAddr); got != "050" {
				t.Errorf("a refused write changed the fake: %q", got)
			}

			w := descWrites(sess.SettingsDescriptor())
			labels := countLabels(w)
			if labels[spec.ConsentedUnverified] != 157 || labels[spec.Unverified] != 36 || labels[spec.Supported] != 0 || len(labels) != 2 {
				t.Errorf("descriptor labels = %v, want 157 ConsentedUnverified and 36 Unverified", labels)
			}
			for _, id := range []string{deniedAddr, heldVOXSelect, heldDataVOXGain, heldShiftFreq} {
				if w[id] != spec.Unverified {
					t.Errorf("%s label = %v, want Unverified", id, w[id])
				}
			}
		})
	}
}

// TestMenuWrite_SimulatedIsSupportedNeverConsented is row (4), per model.
func TestMenuWrite_SimulatedIsSupportedNeverConsented(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			_, sess := openFake(t, m, Simulated)
			if res, err := sess.WriteSetting(testCtx(t), admittedAddr, "050"); err != nil || res.Outcome != driver.SettingWriteAccepted {
				t.Fatalf("WriteSetting = %+v, %v", res, err)
			}
			labels := countLabels(descWrites(sess.SettingsDescriptor()))
			if labels[spec.Supported] != 157 || labels[spec.ConsentedUnverified] != 0 {
				t.Errorf("descriptor labels = %v, want 157 Supported and no ConsentedUnverified", labels)
			}
			paper := catftdx101.PaperDialectD()
			if m.catID == "0682" {
				paper = catftdx101.PaperDialectMP()
			}
			if want := yaesu.BuildDescriptor(paper, &params, spec.Supported); !reflect.DeepEqual(sess.SettingsDescriptor(), want) {
				t.Error("the Simulated session's descriptor differs from the paper-dialect one")
			}
		})
	}
}

// TestMenuWrite_PaperSessionDoesNotLoosenALaterUnconsentedOne is the
// driver-level aliasing check: a paper session's write leaves a later
// unconsented real session of either model refusing every address.
func TestMenuWrite_PaperSessionDoesNotLoosenALaterUnconsentedOne(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			_, paper := openFake(t, m, RealHardware, WithConsentedMenuWrites())
			if _, err := paper.WriteSetting(testCtx(t), admittedAddr, "050"); err != nil {
				t.Fatalf("paper WriteSetting: %v", err)
			}
			p, plain := openSession(t, m, RealHardware, slotImage{exSetSilent: true})
			for _, it := range m.params.dialect.EXItems() {
				wire := m.params.dialect.EXWire(it.Addr)
				if res, err := plain.WriteSetting(testCtx(t), wire, strings.Repeat("0", it.Digits)); err == nil {
					t.Fatalf("unconsented session wrote %s: %+v", wire, res)
				}
			}
			if got := exSetFrames(p); len(got) != 0 {
				t.Errorf("unconsented session sent %q", got)
			}
		})
	}
}

// TestMenuWrite_OutcomesRefusedAndMismatch covers the two non-accepted
// outcomes over the scripted radio, per model.
func TestMenuWrite_OutcomesRefusedAndMismatch(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name+"/radio refuses", func(t *testing.T) {
			_, sess := openSession(t, m, RealHardware, slotImage{}, WithConsentedMenuWrites())
			res, err := sess.WriteSetting(testCtx(t), admittedAddr, "050")
			if err == nil || res.Outcome != driver.SettingWriteRefused || !res.Step.Sent {
				t.Errorf("= %+v, %v; want SettingWriteRefused with the frame sent", res, err)
			}
		})
		t.Run(m.name+"/read-back differs", func(t *testing.T) {
			img := slotImage{exSetSilent: true, exAnswers: map[string]string{admittedAddr: "EX010109040;"}}
			_, sess := openSession(t, m, RealHardware, img, WithConsentedMenuWrites())
			res, err := sess.WriteSetting(testCtx(t), admittedAddr, "050")
			var mm *driver.SettingVerifyMismatchError
			if !errors.As(err, &mm) || res.Outcome != driver.SettingWriteVerifyMismatch || res.Observed != "040" {
				t.Errorf("= %+v, %v; want a verify mismatch observing 040", res, err)
			}
		})
	}
}

// TestNoParseEXAnswerResultReachesEngineDo: an EX Answer and an EX Set share
// one shape (spec v2 §3.5), so nothing in this package may feed a parsed
// answer to Engine.Do. Non-test sources only; none may mention both.
func TestNoParseEXAnswerResultReachesEngineDo(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		if strings.Contains(src, "ParseEXAnswer") && strings.Contains(src, ".Do(") {
			t.Errorf("%s calls both ParseEXAnswer and Engine.Do: audit that no parsed answer is sent", f)
		}
	}
}

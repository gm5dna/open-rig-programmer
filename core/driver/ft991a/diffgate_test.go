// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// This file covers the two things the write path's own unit tests cannot
// reach, because they are decided one layer up: what codeplug.Diff does at
// PLAN time with the two fields matrix erratum M-E3 is about, and what
// core/clone does at EXECUTE time with the write→verify pair the driver
// deliberately does not perform itself (plan P12).
//
// THE FT-891 EXEMPLAR'S FILE OF THIS NAME EXISTS TO PIN TWO REFUSALS. This
// one exists to pin their ABSENCE, which is the harder thing to notice
// going missing:
//
//   - A non-Known TagDisplay is NOT blocked at plan time here. codeplug.
//     Diff's gate fires only when the target bank's FieldTagDisplay.Write
//     is anything other than spec.Unsupported — and on this radio it is
//     exactly that, because MT's P11 legend prints "0: (Fixed)" (layout
//     1015) and caps.go grades the field the zero FieldSupport. So a
//     CHIRP-imported row, which arrives Unknown because CHIRP's schema has
//     no display-flag column, plans and sends cleanly here where it is
//     blocked per channel on the FT-891.
//
//   - A true TxClar is not blocked at plan time either — and, unlike the
//     FT-891, it is not refused at the driver afterwards. Byte 21 is a live
//     TX-clarifier state on this radio (layout 1004 and the four sibling
//     blocks), so it is WRITTEN, and the round trip below carries it in
//     both directions.
//
// The scripted responder serves these tests, not internal/fakeft991a: the
// fake lands on the other Stage 2 lane and does not exist on this branch.
// The end-to-end pass through the REGISTERED fake is task 15a's, after the
// lane merge.

// asReadChannelData is a channel as THIS DRIVER'S READ PATH produces one:
// the writable fields, TagDisplay Unavailable (the register's THE
// PRINTED-FIXED BYTES ARE ANSWERED AS PRINTED entry standing behind the
// legend), CTCSSTone and ScanSkip Unknown (TONE-NUMBER UNREACHABILITY and
// SCAN-SKIP UNREACHABILITY), and all seventeen Icom-tier fields Unavailable
// (plan P12). A baseline that differed from a real read in any of those
// would make every diff entry Modified and prove nothing about the gate
// under test.
func asReadChannelData(mutate func(*codeplug.ChannelData)) *codeplug.ChannelData {
	d := *writableChannel().Data
	if mutate != nil {
		mutate(&d)
	}
	return &d
}

// TestDiff_BlocksNeitherTagDisplayNorTxClarOnThisRadio pins both halves of
// M-E3 at the layer that decides them, with no wire and no session:
// codeplug.Diff against this driver's own Simulated capabilities.
//
// The FieldSupport assertion is not decoration. It is the CONDITION the
// TagDisplay gate keys on, so without it a future capability change could
// start blocking every channel again while this test went on passing for
// the wrong reason.
func TestDiff_BlocksNeitherTagDisplayNorTxClarOnThisRadio(t *testing.T) {
	caps := CapabilitiesSimulated()
	if got := caps.FieldSupport(spec.BankMemory, spec.FieldTagDisplay).Write; got != spec.Unsupported {
		t.Fatalf("FieldTagDisplay.Write = %v, want spec.Unsupported — codeplug.Diff's TagDisplay gate keys on exactly that, and this test asserts nothing unless it holds", got)
	}
	baseline := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: modelName, CATID: catID},
		Channels: []codeplug.Channel{{Slot: "001", Data: asReadChannelData(nil)}},
	}

	for _, tt := range []struct {
		name  string
		after *codeplug.ChannelData
	}{
		{
			// The shape a CHIRP import produces: the file simply does not
			// say what the display flag should be. On this radio nothing
			// ever will, and nothing needs to.
			name:  "a non-Known TagDisplay is NOT blocked — this radio has no display flag",
			after: asReadChannelData(func(d *codeplug.ChannelData) { d.TagDisplay = codeplug.BoolField{State: codeplug.Unknown} }),
		},
		{
			name:  "a changed TxClar is NOT blocked — byte 21 is live here",
			after: asReadChannelData(func(d *codeplug.ChannelData) { d.TxClar = false }),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			file := &codeplug.Codeplug{
				Schema:   codeplug.CurrentSchema,
				Radio:    baseline.Radio,
				Channels: []codeplug.Channel{{Slot: "001", Data: tt.after}},
			}
			res, err := codeplug.Diff(baseline, file, caps)
			if err != nil {
				t.Fatalf("Diff: %v", err)
			}
			if len(res.Entries) != 1 {
				t.Fatalf("Diff produced %d entries, want 1", len(res.Entries))
			}
			e := res.Entries[0]
			if e.Kind != codeplug.DiffModified {
				t.Errorf("entry.Kind = %v, want %v", e.Kind, codeplug.DiffModified)
			}
			if e.Blocked {
				t.Fatalf("entry.Blocked = true (%q), want false", e.BlockReason)
			}
		})
	}
}

// cloneFixture opens a Simulated session over img, builds a clone.Service
// against it, and returns the service plus a codeplug matching what a read
// of img yields — every slot of every bank this session publishes, empty
// except "001", which img populates with populatedMT.
//
// The baseline is BUILT rather than read back a second time, and the tests
// below assert Report.Unchanged to prove it matched: a hand-built baseline
// that disagreed with the radio would show up as extra Modified entries
// (and, before any write, as a verify-read drift abort), so the saving is
// one whole 117-slot read per test rather than a weakened assertion.
func cloneFixture(t *testing.T, img slotImage) (*respondingPort, *clone.Service, *codeplug.Codeplug) {
	t.Helper()
	p, sess := openSession(t, Simulated, img)
	service := clone.NewService(sess, clone.SnapshotStore{Dir: t.TempDir()})

	cp := &codeplug.Codeplug{
		Schema: codeplug.CurrentSchema,
		Radio:  codeplug.RadioInfo{Model: modelName, CATID: catID},
	}
	for _, b := range sess.Capabilities().Banks {
		for _, slot := range b.Slots {
			ch := codeplug.Channel{Slot: slot}
			if slot == "001" {
				ch.Data = asReadChannelData(nil)
			}
			cp.Channels = append(cp.Channels, ch)
		}
	}
	return p, service, cp
}

// withSlot001 returns a deep-enough copy of cp whose "001" carries data.
func withSlot001(cp *codeplug.Codeplug, data *codeplug.ChannelData) *codeplug.Codeplug {
	out := *cp
	out.Channels = append([]codeplug.Channel(nil), cp.Channels...)
	for i := range out.Channels {
		if out.Channels[i].Slot == "001" {
			out.Channels[i].Data = data
		}
	}
	return &out
}

// TestClone_WriteThenVerifyIsClonesOwnPair is the write→read-back round
// trip, driven by core/clone rather than by the driver — which is the whole
// content of plan P12's boundary: WriteChannel sends and reports
// sent/unrejected, and the pair that follows it belongs one layer up.
//
// The scripted radio ECHOES an accepted Set back as that slot's MT answer
// (slotImage.echoSets), which is available here only because this radio's
// Set and Answer share the same 41 positions. IT IS NOT EVIDENCE ABOUT ANY
// REAL FT-991A — none has ever been connected to this project. What it
// demonstrates is that this driver's write and its read agree about every
// position of the frame, in both directions, through clone's own comparison
// rather than a test's.
//
// THE DELTA IS CHOSEN TO RUN THROUGH BYTE 21, the position no registered
// sibling can vary: it turns the TX clarifier OFF and changes the tag. On
// the FT-891 the same delta would abort the send at the driver.
func TestClone_WriteThenVerifyIsClonesOwnPair(t *testing.T) {
	p, service, baseline := cloneFixture(t, slotImage{
		mtAnswers: map[string]string{"001": populatedMT("001")},
		echoSets:  true,
	})

	file := withSlot001(baseline, asReadChannelData(func(d *codeplug.ChannelData) {
		d.Tag = "NET"
		d.TxClar = false
	}))
	plan, err := service.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	before := len(p.Transcript())
	report, err := service.Execute(testCtx(t), plan, plan.ConfirmationDigest(), clone.ExecuteOptions{
		// Obligation 10's first-write gate: a human-supplied string. It is
		// not a claim about any real radio — no FT-991A has ever been
		// connected to this project — only the value the gate requires.
		FirmwareConfirmed: "scripted-peer",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if report.Aborted || report.Written != 1 || report.Verified != 1 {
		t.Fatalf("Report = {Aborted:%v Written:%d Verified:%d}, want one channel written and verified", report.Aborted, report.Written, report.Verified)
	}
	// Unchanged counts every slot the hand-built baseline matched — the
	// proof that this fixture's baseline really is what the radio holds.
	if report.Unchanged != len(baseline.Channels)-1 {
		t.Errorf("Report.Unchanged = %d, want %d — the baseline this test built must match the radio exactly, or the delta under test is not the only one", report.Unchanged, len(baseline.Channels)-1)
	}

	// The wire, in order: clone's per-slot verify-read (obligation 11), the
	// ONE combined Set, then clone's read-back. The Set's bytes are
	// re-derived by hand here for the same reason
	// TestWriteChannel_OneCombinedMTSetFrame derives its own.
	//
	//	MT|001|145500000|-|0150|1|0|4|0|1|00|1|0|NET_________|;
	const wantSet = "MT001145500000-0150104010010NET         ;"
	if len(wantSet) != 41 {
		t.Fatalf("the hand-derived Set is %d bytes, not the chart's 41", len(wantSet))
	}
	got := p.Transcript()[before:]
	want := []string{"MT001;", wantSet, "MT001;"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wire carried %v, want %v", got, want)
	}
}

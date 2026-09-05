// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// This file covers the two things the write path's own unit tests cannot
// reach, because they are decided one layer up: what codeplug.Diff does with
// this radio's two mandatory semantic refusals at PLAN time, and what
// core/clone does with them at EXECUTE time.
//
// The asymmetry between the two is the point, and it is a recorded plan
// decision (P5, P6, and the matrix's "Plan decision recorded (§2.2)"):
//
//   - A non-Known TagDisplay is BLOCKED AT PLAN TIME. codeplug.Diff already
//     blocks it for any target that transmits the flag, which this radio
//     does, so a CHIRP-imported row (which arrives Unknown, because CHIRP's
//     schema has no display-flag column) never reaches the wire at all and
//     the user is told once, per channel, with the instruction that clears
//     it. The driver's own refusal is defence in depth behind that.
//
//   - A true TxClar is NOT blocked at plan time. There is no Diff gate for
//     it this milestone — that would be a shared seam, and opening one is
//     not this driver's to do — so a foreign TxClar-true channel (from a
//     native file written for an FT-710 or FTdx10, a CSV import, or a GUI
//     paste) passes planning and ABORTS THE SEND at the driver, at that
//     slot. That is honest but expensive, and it is why the registration
//     task names it in this radio's user-facing text.
//
// The scripted responder serves these tests, not internal/fakeft891: the
// fake lands on the other Stage 2 lane and does not exist on this branch.

// asReadChannelData is a channel as THIS DRIVER'S READ PATH produces one:
// the writable fields, CTCSSTone and ScanSkip Unknown (the ASSUMED
// register's TONE AND SCAN-SKIP UNREACHABILITY entry), and all seventeen
// Icom-tier fields Unavailable (plan P12). A baseline that differed from a
// real read in any of those would make every diff entry Modified and prove
// nothing about the gate under test.
func asReadChannelData(mutate func(*codeplug.ChannelData)) *codeplug.ChannelData {
	d := tierUnavailable(*writableChannel().Data)
	if mutate != nil {
		mutate(&d)
	}
	return &d
}

// TestDiff_BlocksTagDisplayAtPlanTimeButNotTxClar pins both halves of the
// asymmetry above, at the layer that decides it, with no wire and no
// session: codeplug.Diff against this driver's own Simulated capabilities.
func TestDiff_BlocksTagDisplayAtPlanTimeButNotTxClar(t *testing.T) {
	caps := CapabilitiesSimulated()
	baseline := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: modelName, CATID: catID},
		Channels: []codeplug.Channel{{Slot: "001", Data: asReadChannelData(nil)}},
	}

	for _, tt := range []struct {
		name        string
		after       *codeplug.ChannelData
		wantBlocked bool
		wantReason  string
	}{
		{
			// The shape a CHIRP import produces: the file simply does not
			// say what the display flag should be.
			name:        "a non-Known TagDisplay is blocked",
			after:       asReadChannelData(func(d *codeplug.ChannelData) { d.TagDisplay = codeplug.BoolField{State: codeplug.Unknown} }),
			wantBlocked: true,
			wantReason:  "tag display",
		},
		{
			name:        "a true TxClar is NOT blocked — there is no plan-time gate for it",
			after:       asReadChannelData(func(d *codeplug.ChannelData) { d.TxClar = true }),
			wantBlocked: false,
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
			if e.Blocked != tt.wantBlocked {
				t.Fatalf("entry.Blocked = %v (%q), want %v", e.Blocked, e.BlockReason, tt.wantBlocked)
			}
			if tt.wantReason != "" && !strings.Contains(strings.ToLower(e.BlockReason), tt.wantReason) {
				t.Errorf("entry.BlockReason = %q, want it to mention %q", e.BlockReason, tt.wantReason)
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

// execute runs plan through service with the confirmations Execute demands.
func execute(t *testing.T, service *clone.Service, plan *clone.SendPlan) (*clone.Report, error) {
	t.Helper()
	return service.Execute(testCtx(t), plan, plan.ConfirmationDigest(), clone.ExecuteOptions{
		// Obligation 10's first-write gate: a human-supplied string. It is
		// not a claim about any real radio — no FT-891 has ever been
		// connected to this project — only the value the gate requires.
		FirmwareConfirmed: "scripted-peer",
	})
}

// TestClone_TxClarTrueAbortsTheSendAtTheDriver is plan P5's end of the
// asymmetry, driven through the layer that actually pays for it.
//
// The delta is a single channel whose TX-clarifier flag is true — a value
// no FT-891 read can ever produce (under P5Fixed the parser REQUIRES '0' at
// byte 21 and returns TxClar false), so it can only have come from another
// radio's file. It passes planning unblocked, reaches WriteChannel, and is
// refused there BEFORE any frame is built; core/clone turns that into an
// *AbortedError naming the slot, whose cause is the driver's own
// *driver.WriteRefusedError naming spec.FieldClarifier.
//
// Nothing is written and nothing reaches the wire for that slot: Written 0,
// and the transcript's last frame is the verify-read, never a Set.
func TestClone_TxClarTrueAbortsTheSendAtTheDriver(t *testing.T) {
	p, service, baseline := cloneFixture(t, slotImage{mtAnswers: map[string]string{"001": populatedMT("001")}})

	file := withSlot001(baseline, asReadChannelData(func(d *codeplug.ChannelData) { d.TxClar = true }))
	plan, err := service.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v — a TxClar-true channel must PLAN cleanly; there is no Diff gate for it this milestone", err)
	}

	before := len(p.Transcript())
	report, err := execute(t, service, plan)
	if err == nil {
		t.Fatal("Execute = nil error, want the driver's TxClar refusal to abort the send")
	}
	var aborted *clone.AbortedError
	if !errors.As(err, &aborted) {
		t.Fatalf("error %v (%T) is not a *clone.AbortedError", err, err)
	}
	if aborted.Slot != "001" {
		t.Errorf("AbortedError.Slot = %q, want \"001\"", aborted.Slot)
	}
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("the abort's cause %v is not a *driver.WriteRefusedError — the refusal must survive the abort so a caller can see WHICH field stopped the send", err)
	}
	if want := []spec.Field{spec.FieldClarifier}; len(refused.Fields) != 1 || refused.Fields[0] != want[0] {
		t.Errorf("WriteRefusedError.Fields = %v, want %v", refused.Fields, want)
	}
	if !strings.Contains(refused.Reason, "P5") {
		t.Errorf("WriteRefusedError.Reason = %q, want it to name P5", refused.Reason)
	}

	if report == nil {
		t.Fatal("Execute returned no partial report alongside its abort")
	}
	if !report.Aborted || report.Written != 0 || report.Verified != 0 {
		t.Errorf("Report = {Aborted:%v Written:%d Verified:%d}, want an abort with nothing written", report.Aborted, report.Written, report.Verified)
	}
	// Unchanged counts every slot the hand-built baseline matched — the
	// proof that this fixture's baseline really is what the radio holds.
	if report.Unchanged != len(baseline.Channels)-1 {
		t.Errorf("Report.Unchanged = %d, want %d — the baseline this test built must match the radio exactly, or the delta under test is not the only one", report.Unchanged, len(baseline.Channels)-1)
	}
	for _, frame := range p.Transcript()[before:] {
		if len(frame) > mtReadFrameLen && strings.HasPrefix(frame, "MT") {
			t.Errorf("a combined MT Set reached the wire (%q) — the refusal is pre-wire", frame)
		}
	}
}

// TestClone_WriteThenVerifyIsClonesOwnPair is the write→read-back round
// trip, driven by core/clone rather than by the driver — which is the whole
// content of plan P3's boundary: WriteChannel sends and reports
// sent/unrejected, and the pair that follows it belongs one layer up.
//
// The scripted radio ECHOES an accepted Set back as that slot's MT answer
// (slotImage.echoSets), which is exactly the byte-faithful read-back the
// driver register's A SINGLE COMBINED MT SET SUFFICES entry names as its
// lift — and is available here only because this radio's Set and Answer
// share the same 41 positions. IT IS NOT EVIDENCE ABOUT ANY REAL FT-891:
// what it demonstrates is that this driver's write and its read agree about
// every position of the frame, in both directions, through clone's own
// comparison rather than a test's.
//
// The delta changes the tag AND turns the live TAG display flag OFF, so the
// round trip runs through byte 28 in both directions — the one field of
// this record no registered sibling has.
func TestClone_WriteThenVerifyIsClonesOwnPair(t *testing.T) {
	p, service, baseline := cloneFixture(t, slotImage{
		mtAnswers: map[string]string{"001": populatedMT("001")},
		echoSets:  true,
	})

	file := withSlot001(baseline, asReadChannelData(func(d *codeplug.ChannelData) {
		d.Tag = "NET"
		d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: false}
	}))
	plan, err := service.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	before := len(p.Transcript())
	report, err := execute(t, service, plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if report.Aborted || report.Written != 1 || report.Verified != 1 {
		t.Fatalf("Report = {Aborted:%v Written:%d Verified:%d}, want one channel written and verified", report.Aborted, report.Written, report.Verified)
	}
	if report.Unchanged != len(baseline.Channels)-1 {
		t.Errorf("Report.Unchanged = %d, want %d", report.Unchanged, len(baseline.Channels)-1)
	}

	// The wire, in order: clone's per-slot verify-read (obligation 11),
	// the ONE combined Set, then clone's read-back. The Set's bytes are
	// re-derived by hand here for the same reason
	// TestWriteChannel_OneCombinedMTSetFrame derives its own.
	//
	//	MT|001|014250000|-|0150|1|0|2|0|1|00|1|0|NET_________|;
	const wantSet = "MT001014250000-0150102010010NET         ;"
	got := p.Transcript()[before:]
	want := []string{"MT001;", wantSet, "MT001;"}
	if len(got) != len(want) {
		t.Fatalf("wire carried %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("frame %d = %q, want %q", i, got[i], want[i])
		}
	}
	if len(wantSet) != 41 {
		t.Fatalf("the hand-derived Set is %d bytes, not the chart's 41", len(wantSet))
	}
}

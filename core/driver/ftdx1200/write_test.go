// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func populatedChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 14_250_000,
			Mode:   "USB",
			CTCSS:  "OFF",
			Shift:  "SIMPLEX",
		},
	}
}

func TestWriteChannel_Succeeds(t *testing.T) {
	p, s := openSession(t, Simulated, slotImage{})
	res, err := s.WriteChannel(testCtx(t), populatedChannel("001"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("WriteResult = %+v, want one Sent+Confirmed MW step", res)
	}
	transcript := p.Transcript()
	if len(transcript) != 3 { // AI0, ID, MW
		t.Fatalf("transcript = %v, want [AI0;, ID;, MW...;]", transcript)
	}
	mw := transcript[2]
	if !strings.HasPrefix(mw, "MW001") {
		t.Errorf("MW frame = %q, want it to start \"MW001\"", mw)
	}
	if got := mw[21:23]; got != "00" {
		t.Errorf("MW P9 = %q, want \"00\"", got)
	}
}

func TestWriteChannel_EmptyChannelRefusesErase(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	if _, err := s.WriteChannel(testCtx(t), codeplug.Channel{Slot: "001"}); err == nil {
		t.Fatal("WriteChannel of an empty channel succeeded, want a refusal — no erase command exists")
	}
}

func TestWriteChannel_RejectedByRadio(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{rejectSets: true})
	if _, err := s.WriteChannel(testCtx(t), populatedChannel("001")); err == nil {
		t.Fatal("WriteChannel succeeded against a radio that rejects every MW Set, want an error")
	}
}

func TestBuildMWCommand_UnknownModeRefuses(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	ch := populatedChannel("001")
	ch.Data.Mode = "NOT-A-MODE"
	if _, err := s.buildMWCommand(ch); err == nil {
		t.Fatal("buildMWCommand accepted an unknown mode name")
	}
}

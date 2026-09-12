// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func TestReadChannel_OccupiedSlot(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)

	ch, err := s.ReadChannel(t.Context(), "001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data == nil {
		t.Fatal("ReadChannel returned an empty channel for an occupied slot")
	}
	if ch.Data.FreqHz != 14_250_000 || ch.Data.Mode != "USB" {
		t.Fatalf("ReadChannel = freq %d mode %q, want 14250000 USB", ch.Data.FreqHz, ch.Data.Mode)
	}
	if ch.Data.Filter.State != codeplug.Known || ch.Data.Filter.Value != "Wide" {
		t.Fatalf("Filter = %+v, want Known Wide", ch.Data.Filter)
	}
	if ch.Data.DataMode.State != codeplug.Known || !ch.Data.DataMode.Value {
		t.Fatalf("DataMode = %+v, want Known true", ch.Data.DataMode)
	}
	if ch.Data.TxFreqHz.State != codeplug.Known || ch.Data.TxFreqHz.Value != 14_250_000 {
		t.Fatalf("TxFreqHz = %+v, want Known 14250000", ch.Data.TxFreqHz)
	}
	if ch.Data.Tag != "" {
		t.Errorf("Tag = %q, want empty — this radio is NoTag", ch.Data.Tag)
	}
	if ch.Data.ToneMode.State != codeplug.Unavailable {
		t.Errorf("ToneMode = %+v, want Unavailable — this record has no tone field", ch.Data.ToneMode)
	}
}

func TestReadChannel_EmptySlot(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio()) // channel 1 only
	s := openWith(t, p)

	ch, err := s.ReadChannel(t.Context(), "002")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data != nil {
		t.Fatalf("ReadChannel of an empty slot returned Data %+v, want nil", ch.Data)
	}
}

func TestReadChannel_ScanEdges(t *testing.T) {
	p := newScriptedPort(t, radioImage{
		idToken: []byte{0x76},
		records: map[int][]byte{1: goldenRecord, 200: goldenRecord, 201: goldenRecord},
	})
	s := openWith(t, p)

	for _, slot := range []string{"P1", "P2"} {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel %s: %v", slot, err)
		}
		if ch.Data == nil || ch.Data.FreqHz != 14_250_000 {
			t.Fatalf("ReadChannel %s = %+v, want a populated 14.25 MHz channel", slot, ch.Data)
		}
	}
}

func TestReadChannel_MalformedSlot(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)

	for _, slot := range []string{"", "0000", "P3", "200"} {
		if _, err := s.ReadChannel(t.Context(), slot); err == nil {
			t.Errorf("ReadChannel(%q) succeeded, want an error", slot)
		}
	}
}

// TestReadChannel_AnswerMismatchRefuses pins tier ruling T2: a memory
// answer naming a different channel than the one requested must never be
// mapped onto the requested slot.
func TestReadChannel_AnswerMismatchRefuses(t *testing.T) {
	p := newScriptedPort(t, radioImage{
		idToken: []byte{0x76},
		// Channel 1 is valid and UNMISNAMED, so Open's own probe (which
		// walks channels 1..10) fingerprints normally; only channel 50 —
		// outside the probe's range — is misnamed, so the mismatch this
		// test wants is met exactly once, by the read under test.
		records: map[int][]byte{1: goldenRecord, 50: goldenRecord},
		answerAddress: func(asked int) int {
			if asked == 50 {
				return 51
			}
			return asked
		},
	})
	s := openWith(t, p)

	_, err := s.ReadChannel(t.Context(), "050")
	var mismatch *AnswerMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("ReadChannel = %v, want *AnswerMismatchError", err)
	}
	if s.AnswerMismatches() != 1 {
		t.Errorf("AnswerMismatches() = %d, want 1", s.AnswerMismatches())
	}
}

// TestReadChannel_RecordLengthMismatchIsAnError pins that a record at a
// length this profile does not declare aborts the read rather than
// producing a partial channel.
func TestReadChannel_RecordLengthMismatchIsAnError(t *testing.T) {
	p := newScriptedPort(t, radioImage{
		idToken: []byte{0x76},
		// Channel 1 is a valid record so Open's own probe fingerprints
		// normally; channel 2 is one byte short, so ONLY the read under
		// test meets the mismatch.
		records: map[int][]byte{1: goldenRecord, 2: goldenRecord[:len(goldenRecord)-1]},
	})
	s := openWith(t, p)

	if _, err := s.ReadChannel(t.Context(), "002"); err == nil {
		t.Fatal("ReadChannel of a short record succeeded, want an error")
	}
}

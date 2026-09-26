// SPDX-License-Identifier: GPL-3.0-or-later

package fakeyaesuclone

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
)

func blockCount(p clonewire.Profile) int {
	n := 0
	for _, b := range p.BlockSchedule {
		if p.AckExpected && b.Ack {
			n++
		}
	}
	return n
}

// runReceive arms and receives against radio's port, using the given
// candidate set (defaulting to []clonewire.Profile{profile} when nil).
func runReceive(t *testing.T, ctx context.Context, radio *Radio, candidates []clonewire.Profile) (clonewire.Image, error) {
	t.Helper()
	rec, err := clonewire.Arm(ctx, radio.Port(), candidates)
	if err != nil {
		return clonewire.Image{}, err
	}
	<-rec.Armed()
	return rec.Receive(ctx)
}

// TestEndToEnd_OneProfilePerFamily proves clonewire.Arm -> Receive against
// this fake, for a representative Profile in each of the three families,
// yields the expected Image and an ACK count equal to the block count —
// the Phase 3 brief's required end-to-end proof.
func TestEndToEnd_OneProfilePerFamily(t *testing.T) {
	cases := []struct {
		name    string
		profile clonewire.Profile
	}{
		{"FT-817", clonewire.FT817},
		{"FT-817ND", clonewire.FT817ND},
		{"FT-817ND(US)", clonewire.FT817NDUS},
		{"FT-818", clonewire.FT818},
		{"FT-818ND(US)", clonewire.FT818NDUS},
		{"FT-857", clonewire.FT857},
		{"FT-857D", clonewire.FT857D},
		{"FT-897", clonewire.FT897},
		{"FT-897D", clonewire.FT897D},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			radio := New(tc.profile)
			img, err := runReceive(t, ctx, radio, []clonewire.Profile{tc.profile})
			if err != nil {
				t.Fatalf("Receive: %v", err)
			}
			<-radio.Wait()
			if radio.Err() != nil {
				t.Fatalf("Radio.Err: %v", radio.Err())
			}

			want := BuildImage(tc.profile)
			if len(img.Raw) != len(want) {
				t.Fatalf("Image.Raw is %d bytes, want %d", len(img.Raw), len(want))
			}
			for i := range want {
				if img.Raw[i] != want[i] {
					t.Fatalf("Image.Raw differs from BuildImage at byte %d: got 0x%02X want 0x%02X", i, img.Raw[i], want[i])
				}
			}

			wantAcks := blockCount(tc.profile)
			if got := radio.AckCount(); got != wantAcks {
				t.Errorf("AckCount = %d, want %d (block count)", got, wantAcks)
			}
		})
	}
}

// TestEndToEnd_SingleModelCandidateResolves proves a candidate set naming
// only one member of a byte-identical pair (e.g. just FT857, not FT857D)
// resolves cleanly — model identity for this pair is operator-asserted by
// which candidate set is offered, not decided by the image (phase2.md
// verdict).
func TestEndToEnd_SingleModelCandidateResolves(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	radio := New(clonewire.FT857)
	img, err := runReceive(t, ctx, radio, []clonewire.Profile{clonewire.FT857})
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if img.Profile.ProfileID != clonewire.FT857.ProfileID {
		t.Errorf("resolved profile %q, want %q", img.Profile.ProfileID, clonewire.FT857.ProfileID)
	}
}

// TestEndToEnd_CombinedFT857FamilyAmbiguous proves that offering BOTH
// FT857 and FT857D as candidates against an FT857-shaped image returns
// ErrImageAmbiguous — CHIRP supplies no byte distinguishing them
// (phase2.md verdict), so a combined candidate set cannot resolve one.
func TestEndToEnd_CombinedFT857FamilyAmbiguous(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	radio := New(clonewire.FT857)
	_, err := runReceive(t, ctx, radio, []clonewire.Profile{clonewire.FT857, clonewire.FT857D})
	if !errors.Is(err, clonewire.ErrImageAmbiguous) {
		t.Fatalf("got err = %v, want ErrImageAmbiguous", err)
	}
}

// TestRadio_FailsOnWrongAckByte proves the fake enforces the RADIO role
// (doc.go): any byte other than 0x06 sent to it after a block fails the
// transfer.
func TestRadio_FailsOnWrongAckByte(t *testing.T) {
	radio := New(clonewire.FT817)
	conn := radio.Port()
	firstLen := clonewire.FT817.BlockSchedule[0].Len

	buf := make([]byte, firstLen)
	if _, err := readFull(conn, buf); err != nil {
		t.Fatalf("reading first block: %v", err)
	}
	if _, err := conn.Write([]byte{0x15}); err != nil { // NAK, not ACK
		t.Fatalf("writing wrong ack byte: %v", err)
	}

	select {
	case <-radio.Wait():
	case <-time.After(10 * time.Second):
		t.Fatal("radio did not finish after a wrong ACK byte")
	}
	if radio.Err() == nil {
		t.Fatal("Radio.Err() = nil, want a failure after a wrong ACK byte")
	}
}

// TestRadio_FailsOnMissingAck proves the fake fails a transfer whose peer
// never ACKs the first block at all (ackWaitTimeout is exercised with a
// short-lived context here via radio.Close, not a full 5s wait).
func TestRadio_FailsOnMissingAck(t *testing.T) {
	radio := New(clonewire.FT817)
	conn := radio.Port()
	firstLen := clonewire.FT817.BlockSchedule[0].Len

	buf := make([]byte, firstLen)
	if _, err := readFull(conn, buf); err != nil {
		t.Fatalf("reading first block: %v", err)
	}
	// Never ACK; close the PC side instead, which lets the radio's own
	// wait unblock with an error rather than the test paying the full
	// ackWaitTimeout.
	_ = conn.Close()

	select {
	case <-radio.Wait():
	case <-time.After(10 * time.Second):
		t.Fatal("radio did not finish after the PC side closed without ACKing")
	}
	if radio.Err() == nil {
		t.Fatal("Radio.Err() = nil, want a failure when no ACK arrives")
	}
}

func readFull(r interface {
	Read([]byte) (int, error)
}, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

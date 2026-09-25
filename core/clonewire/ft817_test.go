// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

// shortDeadlines returns a copy of p with much shorter deadlines than the
// real, CHIRP-informed ASSUMED values — the wire schedule/checksums/
// offsets are unchanged, only how long a test waits for the trailing
// drain-window check.
func shortDeadlines(p Profile) Profile {
	p.StartDeadline = 500 * time.Millisecond
	p.InterBlockDeadline = 100 * time.Millisecond
	p.TotalDeadline = 10 * time.Second
	return p
}

// syntheticYaesuImage builds an ImageLen-byte payload for p: every byte is
// 0xFF (CHIRP's own "unused channel" fill), except channel 0's record,
// which is set to a known, decodable USB channel — proving DecodeRecord
// against a real record offset/width, not just a slicing count.
func syntheticYaesuImage(p Profile) []byte {
	img := make([]byte, p.ImageLen)
	for i := range img {
		img[i] = 0xFF
	}
	rec := make([]byte, p.RecordWidth)
	for i := range rec {
		rec[i] = 0xFF
	}
	rec[yaesuModeOffset] = 0x01 // USB (yaesuModes[1])
	rec[yaesuNarrowOffset] = 0x00
	binary.BigEndian.PutUint32(rec[p.FreqOffset:], 1430000) // *10 = 14,300,000 Hz
	copy(rec[p.FreqOffset+8:], []byte("TESTCH  "))
	copy(img[p.RecordOffset:], rec)
	return img
}

// sendYaesuBlocks plays the RADIO role over conn: reframes payload per p's
// BlockSchedule ([blocknum][chunk][checksum]) and requires exactly one ACK
// (0x06) after every block, matching CHIRP's own _clone_in loop, which ACKs
// every block including every repeat.
func sendYaesuBlocks(t *testing.T, conn net.Conn, p Profile, payload []byte) {
	t.Helper()
	pos := 0
	ack := make([]byte, 1)
	for i, b := range p.BlockSchedule {
		n := b.Len - b.HeaderBytes - b.TrailerBytes
		chunk := make([]byte, 0, b.Len)
		chunk = append(chunk, byte(i))
		chunk = append(chunk, payload[pos:pos+n]...)
		var sum byte
		for _, x := range payload[pos : pos+n] {
			sum += x
		}
		chunk = append(chunk, sum)
		pos += n

		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		if _, err := conn.Write(chunk); err != nil {
			t.Fatalf("write block %d: %v", i, err)
		}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if n, err := conn.Read(ack); err != nil || n != 1 || ack[0] != ackByte {
			t.Fatalf("ACK for block %d: n=%d err=%v byte=%v", i, n, err, ack)
		}
	}
	if pos != len(payload) {
		t.Fatalf("blocks consumed %d payload bytes, want %d", pos, len(payload))
	}
}

// roundTrip arms and receives p's own synthetic image against the given
// candidate set, run under -race via net.Pipe, mirroring receive_test.go's
// own pattern.
func roundTrip(t *testing.T, p Profile, candidates []Profile) Image {
	t.Helper()
	pcConn, radioConn := net.Pipe()
	defer pcConn.Close()
	defer radioConn.Close()

	r, err := Arm(context.Background(), pcConn, candidates)
	if err != nil {
		t.Fatalf("Arm: %v", err)
	}

	type result struct {
		img Image
		err error
	}
	resCh := make(chan result, 1)
	go func() {
		img, err := r.Receive(context.Background())
		resCh <- result{img, err}
	}()

	sendYaesuBlocks(t, radioConn, p, syntheticYaesuImage(p))

	select {
	case res := <-resCh:
		if res.err != nil {
			t.Fatalf("Receive: %v", res.err)
		}
		return res.img
	case <-time.After(5 * time.Second):
		t.Fatal("Receive did not complete")
		return Image{}
	}
}

func assertRoundTrip(t *testing.T, p Profile) {
	t.Helper()
	img := roundTrip(t, shortDeadlines(p), []Profile{shortDeadlines(p)})
	if len(img.Raw) != p.ImageLen {
		t.Fatalf("%s: Raw len = %d, want %d", p.Model, len(img.Raw), p.ImageLen)
	}
	if len(img.Records) != p.ChannelCount {
		t.Fatalf("%s: len(Records) = %d, want %d", p.Model, len(img.Records), p.ChannelCount)
	}
	if len(img.Channels) != p.ChannelCount {
		t.Fatalf("%s: len(Channels) = %d, want %d", p.Model, len(img.Channels), p.ChannelCount)
	}
	got := img.Channels[0]
	if got.Empty {
		t.Fatalf("%s: Channels[0] reported Empty", p.Model)
	}
	if got.FreqHz != 14300000 || got.Mode != "USB" || got.Name != "TESTCH" {
		t.Fatalf("%s: Channels[0] = %+v, want {FreqHz:14300000 Mode:USB Name:TESTCH}", p.Model, got)
	}
	if !img.Channels[1].Empty {
		t.Fatalf("%s: Channels[1] should read Empty (0xFF fill)", p.Model)
	}
}

func TestFT817Family_RoundTrip(t *testing.T) {
	for _, p := range FT817Family {
		p := p
		t.Run(p.Model, func(t *testing.T) {
			assertRoundTrip(t, p)
		})
	}
}

func TestFT817Family_Incompatible(t *testing.T) {
	candidates := make([]Profile, len(FT817Family))
	for i, p := range FT817Family {
		candidates[i] = shortDeadlines(p)
	}
	_, err := matchAndParse(make([]byte, 12345), candidates)
	if !errors.Is(err, ErrImageIncompatible) {
		t.Fatalf("err = %v, want ErrImageIncompatible", err)
	}
}

// TestFT817Family_NoAmbiguity documents the plan's own "remaining open
// risk": CHIRP shows five DISTINCT lengths for this family, so no two real
// Profile values here ever collide — the ambiguous-match path is proven
// with real CHIRP-informed data in the FT-857/FT-897 family tests instead
// (ft857_test.go, ft897_test.go), where CHIRP genuinely does not
// distinguish two models by length.
func TestFT817Family_NoAmbiguity(t *testing.T) {
	seen := map[int]string{}
	for _, p := range FT817Family {
		if other, ok := seen[p.ImageLen]; ok {
			t.Fatalf("unexpected length collision: %s and %s both %d bytes", other, p.Model, p.ImageLen)
		}
		seen[p.ImageLen] = p.Model
	}
}

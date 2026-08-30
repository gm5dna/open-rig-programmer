// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// Every expected frame in this file is written out BY HAND from the printed
// diagrams, never assembled by calling this package's own builders — otherwise
// the test would only prove that buildFrame agrees with itself.

const testTimeout = 2 * time.Second

// exchange writes one frame to the radio and returns the frames it said back,
// stopping when want frames have arrived or the timeout expires.
func exchange(t *testing.T, r *Radio, want int, request []byte) [][]byte {
	t.Helper()
	if _, err := r.Port().Write(request); err != nil {
		t.Fatalf("writing %s: %v", hexBytes(request), err)
	}
	return readFrames(t, r, want)
}

// readFrames reads until want whole frames have arrived, or the timeout.
func readFrames(t *testing.T, r *Radio, want int) [][]byte {
	t.Helper()
	if want == 0 {
		// Nothing is expected. Give the radio a moment to be wrong, then look.
		time.Sleep(50 * time.Millisecond)
	}

	type result struct{ frames [][]byte }
	done := make(chan result, 1)
	go func() {
		acc := newAccumulator()
		var got [][]byte
		buf := make([]byte, 512)
		for len(got) < want {
			n, err := r.Port().Read(buf)
			for _, body := range acc.feed(buf[:n]) {
				got = append(got, canonicalFrame(body))
			}
			if err != nil {
				break
			}
		}
		done <- result{got}
	}()

	if want == 0 {
		// Closing the radio wakes the reader with io.EOF, so the goroutine
		// above returns whatever had already arrived — which must be nothing.
		r.Close()
	}
	select {
	case res := <-done:
		return res.frames
	case <-time.After(testTimeout):
		t.Fatalf("timed out waiting for %d frames from the radio", want)
		return nil
	}
}

// assertFrames compares what the radio said with hand-written expectations.
func assertFrames(t *testing.T, got, want [][]byte) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("radio said %d frames %v, want %d %v", len(got), hexFrames(got), len(want), hexFrames(want))
	}
	for i := range got {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("frame %d:\n got %s\nwant %s", i, hexBytes(got[i]), hexBytes(want[i]))
		}
	}
}

// equalBlockRecord builds a 111-byte record whose transmit duplicate carries
// the same bytes as its receive payload — the shape the printed NOTE describes
// — with a distinctive filler so that a misplaced block is visible in a failure
// message. It is built by hand here, from the offsets this test file states
// itself, not from the package's constants.
func equalBlockRecord(name string) []byte {
	const (
		split = 1
		rx    = 47
		tx    = 47
		nm    = 16
	)
	rec := make([]byte, 0, split+rx+tx+nm)
	rec = append(rec, 0x00) // field (4): split OFF, select memory OFF

	payload := make([]byte, rx)
	for i := range payload {
		payload[i] = byte(0x10 + i)
	}
	rec = append(rec, payload...)
	rec = append(rec, payload...)

	tag := []byte("                ")
	copy(tag, name)
	rec = append(rec, tag[:nm]...)
	return rec
}

func TestReadTransceiverID(t *testing.T) {
	// PDF p.364 (folio 20-5): "19 | 00 | (Data column empty) | Read the
	// transceiver ID". The request carries no data area. What comes back is
	// undocumented; see doc.go, entry 4.
	r := New(WithIdentityToken([]byte{0x88}))
	defer r.Close()

	got := exchange(t, r, 1, []byte{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD})
	assertFrames(t, got, [][]byte{
		{0xFE, 0xFE, 0xE0, 0x88, 0x19, 0x00, 0x88, 0xFD},
	})
}

func TestReadTransceiverID_DefaultTokenIsObviouslyInvented(t *testing.T) {
	r := New()
	defer r.Close()

	got := exchange(t, r, 1, []byte{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD})
	assertFrames(t, got, [][]byte{
		{0xFE, 0xFE, 0xE0, 0x88, 0x19, 0x00, 0xDE, 0xAD, 0xFD},
	})
}

func TestReadOfASeededChannel(t *testing.T) {
	// The read request is the G leg's own vector, hand-transcribed:
	// FE FE 88 E0 1A 00 01 00 01 FD — bank A, memory channel 1.
	//
	// The answer is the same command with the same three address bytes and the
	// 111 record bytes behind them, addresses swapped: 121 bytes in all.
	rec := equalBlockRecord("HOME BASE")
	r := New(WithSlot(1, 1, rec))
	defer r.Close()

	got := exchange(t, r, 1, []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0xFD})

	want := []byte{0xFE, 0xFE, 0xE0, 0x88, 0x1A, 0x00, 0x01, 0x00, 0x01}
	want = append(want, rec...)
	want = append(want, 0xFD)
	if len(want) != 121 {
		t.Fatalf("the hand-built expectation is %d bytes, want 121 — 6 framing and command bytes, 114 data bytes, one FD", len(want))
	}
	assertFrames(t, got, [][]byte{want})
}

func TestReadOfAnEmptyChannelIsRefused(t *testing.T) {
	// ASSUMED — doc.go entry 1 (ic7100-empty-channel-fa). The document never
	// says what a read of a cleared channel returns.
	r := New(WithSlot(1, 1, equalBlockRecord("A")), WithEmptySlot(1, 2))
	defer r.Close()

	for _, addr := range [][]byte{{0x01, 0x00, 0x02}, {0x03, 0x00, 0x77}} {
		req := append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00}, addr...)
		req = append(req, 0xFD)
		got := exchange(t, r, 1, req)
		assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}})
	}
}

func TestReadOfAnEmptyChannelCanAnswerAnAllFFRecordInstead(t *testing.T) {
	// The other admissible reading — doc.go entry 2 (ic7100-all-ff-record).
	r := New(WithAllFFEmptyRecord())
	defer r.Close()

	got := exchange(t, r, 1, []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0xFD})

	want := []byte{0xFE, 0xFE, 0xE0, 0x88, 0x1A, 0x00, 0x01, 0x00, 0x01}
	want = append(want, bytes.Repeat([]byte{0xFF}, 111)...)
	want = append(want, 0xFD)
	assertFrames(t, got, [][]byte{want})
}

func TestAddressesOutsideThePrintedRectangleAreRefused(t *testing.T) {
	r := New(WithSlot(1, 1, equalBlockRecord("A")))
	defer r.Close()

	tests := []struct {
		name string
		addr []byte
	}{
		{"bank 00 — not a printed bank code", []byte{0x00, 0x00, 0x01}},
		{"bank 06 — not a printed bank code", []byte{0x06, 0x00, 0x01}},
		{"channel 0000 — outside the field legend's range", []byte{0x01, 0x00, 0x00}},
		{"channel 0100 — programmed scan edge 1A, out of scope", []byte{0x01, 0x01, 0x00}},
		{"channel 0106 — call channel 144-C1, out of scope", []byte{0x01, 0x01, 0x06}},
		{"channel 0109 — call channel 430-C2, out of scope", []byte{0x01, 0x01, 0x09}},
		{"a channel byte that is not packed BCD", []byte{0x01, 0x00, 0xAB}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00}, tt.addr...)
			req = append(req, 0xFD)
			got := exchange(t, r, 1, req)
			assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}})
		})
	}
}

func TestSetStoresTheRecordAndAcknowledges(t *testing.T) {
	r := New()
	defer r.Close()

	rec := equalBlockRecord("SCRATCH")
	req := append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x02, 0x00, 0x42}, rec...)
	req = append(req, 0xFD)

	got := exchange(t, r, 1, req)
	assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFB, 0xFD}})

	held, ok := r.Slot(2, 42)
	if !ok {
		t.Fatal("bank B channel 42 is still empty after an acknowledged set")
	}
	if !bytes.Equal(held, rec) {
		t.Errorf("stored record differs from the one sent:\n got %s\nwant %s", hexBytes(held), hexBytes(rec))
	}

	// And it reads back.
	back := exchange(t, r, 1, []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x02, 0x00, 0x42, 0xFD})
	want := []byte{0xFE, 0xFE, 0xE0, 0x88, 0x1A, 0x00, 0x02, 0x00, 0x42}
	want = append(want, rec...)
	want = append(want, 0xFD)
	assertFrames(t, back, [][]byte{want})
}

func TestSetOfTheManualsOwnWorkedRecord(t *testing.T) {
	// The complete-record write the G leg derived from the printed encodings,
	// hand-transcribed here from its byte-by-byte walk and NOT read from the
	// .golden file: 145.500000 MHz, FM/FIL1, repeater tone and tone squelch
	// 88.5 Hz, DTCS 023, duplex offset 0.600000 MHz, UR "CQCQCQ", R1 and R2
	// blank, the transmit block a verbatim copy of frame bytes 11-57, and the
	// name "HOME BASE".
	//
	// It exists here as an independent check on this package's geometry: if the
	// record were 107 bytes, or the name began at data-area byte 52, this frame
	// would not be accepted.
	rxBlock := []byte{
		0x00, 0x00, 0x50, 0x45, 0x01, 0x05, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08,
		0x85, 0x00, 0x08, 0x85, 0x00, 0x00, 0x23, 0x00, 0x00, 0x60, 0x00,
		0x43, 0x51, 0x43, 0x51, 0x43, 0x51, 0x20, 0x20,
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
		0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20,
	}
	if len(rxBlock) != 47 {
		t.Fatalf("the hand-transcribed receive block is %d bytes, want 47", len(rxBlock))
	}
	name := []byte{0x48, 0x4F, 0x4D, 0x45, 0x20, 0x42, 0x41, 0x53, 0x45, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20, 0x20}

	frame := []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0x00}
	frame = append(frame, rxBlock...)
	frame = append(frame, rxBlock...)
	frame = append(frame, name...)
	frame = append(frame, 0xFD)
	if len(frame) != 121 {
		t.Fatalf("the hand-transcribed set frame is %d bytes, want 121", len(frame))
	}

	r := New()
	defer r.Close()

	got := exchange(t, r, 1, frame)
	assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFB, 0xFD}})

	held, ok := r.Slot(1, 1)
	if !ok {
		t.Fatal("bank A channel 1 is empty after the manual's own worked record was accepted")
	}
	if got, want := len(held), 111; got != want {
		t.Fatalf("stored %d record bytes, want %d", got, want)
	}
	// The name must land on the last sixteen bytes of the record, which is the
	// wire-order claim this package makes and could get wrong.
	if !bytes.Equal(held[len(held)-16:], name) {
		t.Errorf("the name landed at %s, want the last sixteen bytes to be %s", hexBytes(held[len(held)-16:]), hexBytes(name))
	}
}

func TestSetWithAnUnequalTransmitBlockIsRefused(t *testing.T) {
	// ASSUMED — doc.go entry 6 (ic7100-tx-block-mandatory).
	rec := equalBlockRecord("SPLIT")
	rec[1+47] ^= 0x01 // first byte of the transmit duplicate

	r := New()
	defer r.Close()

	req := append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01}, rec...)
	req = append(req, 0xFD)

	got := exchange(t, r, 1, req)
	assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}})

	if _, ok := r.Slot(1, 1); ok {
		t.Error("a refused set still mutated the image")
	}
}

func TestSetWithAnUnequalTransmitBlockCanBeAccepted(t *testing.T) {
	// The other reading of the printed NOTE — "We recommend" is advisory, and
	// the document never says the radio refuses.
	rec := equalBlockRecord("SPLIT")
	rec[1+47] ^= 0x01

	r := New(WithUnequalTransmitBlockAccepted())
	defer r.Close()

	req := append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01}, rec...)
	req = append(req, 0xFD)

	got := exchange(t, r, 1, req)
	assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFB, 0xFD}})

	held, ok := r.Slot(1, 1)
	if !ok || !bytes.Equal(held, rec) {
		t.Error("the record was not stored verbatim once the equality rule was turned off")
	}
}

func TestSetOfTheWrongLengthIsRefused(t *testing.T) {
	r := New()
	defer r.Close()

	full := equalBlockRecord("LEN")
	for _, n := range []int{1, 104, 110, 112, 114} {
		rec := make([]byte, n)
		copy(rec, full)
		req := append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01}, rec...)
		req = append(req, 0xFD)

		got := exchange(t, r, 1, req)
		assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}})
		if _, ok := r.Slot(1, 1); ok {
			t.Fatalf("a %d-byte set still mutated the image", n)
		}
	}
}

func TestWithAcceptedRecordLengthMovesTheLengthRule(t *testing.T) {
	// 104 is the record-only length the diagram bar's own (52)~(60) label
	// implies — the near miss a text-only reading lands on. A driver that
	// fingerprints on 111 has to be able to meet a radio that answers 104.
	r := New(WithAcceptedRecordLength(104))
	defer r.Close()

	short := make([]byte, 104)
	req := append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01}, short...)
	req = append(req, 0xFD)
	assertFrames(t, exchange(t, r, 1, req), [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFB, 0xFD}})

	full := equalBlockRecord("NO")
	req = append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x02}, full...)
	req = append(req, 0xFD)
	assertFrames(t, exchange(t, r, 1, req), [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}})
}

func TestShortSetsCanBeAcceptedButTheClearFormNeverIs(t *testing.T) {
	// ASSUMED — doc.go entry 6 again: the document never says what happens to
	// a set that stops before the transmit block.
	r := New(WithShortSetsAccepted())
	defer r.Close()

	short := equalBlockRecord("SHORT")[:48] // field (4) and the receive payload
	req := append([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01}, short...)
	req = append(req, 0xFD)
	assertFrames(t, exchange(t, r, 1, req), [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFB, 0xFD}})

	// The printed clearing form is a short set too, and it stays refused: this
	// tier sends no clear, and a fake that honoured one would be simulating a
	// radio nobody is driving. See doc.go, entry 12.
	clear := []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFD}
	assertFrames(t, exchange(t, r, 1, clear), [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}})
	if _, ok := r.Slot(1, 1); !ok {
		t.Error("the refused clear form emptied the channel anyway")
	}
}

func TestTheClearFormIsRefusedInBothPrintedReadings(t *testing.T) {
	r := New(WithSlot(1, 1, equalBlockRecord("KEEP")))
	defer r.Close()

	// With the bank byte, and without it — the "About clearing operation" block
	// omits field (1), so the document supports neither reading over the other.
	for _, req := range [][]byte{
		{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFD},
		{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFF, 0xFD},
	} {
		assertFrames(t, exchange(t, r, 1, req), [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}})
	}
	if _, ok := r.Slot(1, 1); !ok {
		t.Error("a refused clear emptied the channel")
	}
}

func TestUnknownCommandsAreRefused(t *testing.T) {
	r := New()
	defer r.Close()

	tests := []struct {
		name string
		req  []byte
	}{
		{"1A 01, the band stacking register", []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x01, 0x07, 0x03, 0xFD}},
		{"1A 05, the set-mode head", []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x05, 0x00, 0x18, 0xFD}},
		{"0B, memory clear", []byte{0xFE, 0xFE, 0x88, 0xE0, 0x0B, 0xFD}},
		{"18 01, power on", []byte{0xFE, 0xFE, 0x88, 0xE0, 0x18, 0x01, 0xFD}},
		{"19 01, not the transceiver-ID sub-command", []byte{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x01, 0xFD}},
		{"1A 00 with no address at all", []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0xFD}},
		{"1A 00 with two address bytes", []byte{0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0xFD}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertFrames(t, exchange(t, r, 1, tt.req), [][]byte{{0xFE, 0xFE, 0xE0, 0x88, 0xFA, 0xFD}})
		})
	}
}

func TestFramesAddressedElsewhereGetNothingAtAll(t *testing.T) {
	// Not even a reject. A radio at a different address never hears the frame,
	// and the controller times out; a fake that answered NG instead would make
	// a driver's timeout branch untestable. ASSUMED — doc.go, entry 11.
	r := New()

	foreign := [][]byte{
		{0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD}, // addressed to another radio
		{0xFE, 0xFE, 0xE0, 0x88, 0x19, 0x00, 0xFD}, // addressed to the controller
		{0xFE, 0xFE, 0x00, 0x88, 0x19, 0x00, 0xFD}, // a broadcast
	}
	for _, f := range foreign {
		if _, err := r.Port().Write(f); err != nil {
			t.Fatalf("writing %s: %v", hexBytes(f), err)
		}
	}

	// readFrames(0) closes the radio, which is what wakes the reader.
	assertFrames(t, readFrames(t, r, 0), nil)

	// But every one of them reached the radio and is on the transcript: what a
	// test wants to know is what arrived, not only what was answered.
	if got, want := len(r.Transcript()), len(foreign); got != want {
		t.Errorf("transcript holds %d frames, want %d", got, want)
	}
}

func TestTheAnswerGoesBackToWhoeverAsked(t *testing.T) {
	// The printed frame's index (3) is "Controller's default address" — a
	// default, on a bus the manual says may carry up to four CI-V devices. So
	// the answer names the requester, and does not assume E0.
	r := New()
	defer r.Close()

	got := exchange(t, r, 1, []byte{0xFE, 0xFE, 0x88, 0xEF, 0x19, 0x00, 0xFD})
	assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xEF, 0x88, 0x19, 0x00, 0xDE, 0xAD, 0xFD}})
}

func TestWithRadioAddress(t *testing.T) {
	// CI-V Address is a set-mode item (PDF p.334, folio 17-25, "Default: 88h"),
	// so a radio at another address is a real radio and not a fault.
	r := New(WithRadioAddress(0x22))
	defer r.Close()

	if _, err := r.Port().Write([]byte{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD}); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := exchange(t, r, 1, []byte{0xFE, 0xFE, 0x22, 0xE0, 0x19, 0x00, 0xFD})
	assertFrames(t, got, [][]byte{{0xFE, 0xFE, 0xE0, 0x22, 0x19, 0x00, 0xDE, 0xAD, 0xFD}})
}

func TestWithEcho(t *testing.T) {
	// ASSUMED — doc.go, entry 3 (ic7100-echo-default). The manual has no
	// echo-back setting and says nothing about the USB path echoing.
	r := New(WithEcho())
	defer r.Close()

	got := exchange(t, r, 2, []byte{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD})
	assertFrames(t, got, [][]byte{
		{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD},
		{0xFE, 0xFE, 0xE0, 0x88, 0x19, 0x00, 0xDE, 0xAD, 0xFD},
	})
}

func TestWithTransceiveBroadcasts(t *testing.T) {
	// ASSUMED — doc.go, entry 5. The document never prints 00 as an address.
	r := New(WithTransceiveBroadcasts(5 * time.Millisecond))
	defer r.Close()

	got := readFrames(t, r, 3)
	for i, f := range got {
		if len(f) < 3 || f[2] != 0x00 {
			t.Errorf("broadcast %d = %s, want a frame addressed to 00", i, hexBytes(f))
		}
	}
}

func TestWithAddressedFlood(t *testing.T) {
	// A separate option because the two species exercise different code: a
	// to=00 broadcast dies at a controller's address filter, while a frame
	// addressed to the controller reaches its engine.
	r := New(WithAddressedFlood(5 * time.Millisecond))
	defer r.Close()

	got := readFrames(t, r, 3)
	for i, f := range got {
		if len(f) < 3 || f[2] != 0xE0 {
			t.Errorf("flood frame %d = %s, want a frame addressed to E0", i, hexBytes(f))
		}
	}
}

func TestTranscriptNormalisesThePreambleRunAndKeepsOrder(t *testing.T) {
	r := New()
	defer r.Close()

	// The manual's own power-ON example, seven extra preamble bytes and all.
	exchange(t, r, 1, []byte{
		0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE,
		0x88, 0xE0, 0x18, 0x01, 0xFD,
	})
	exchange(t, r, 1, []byte{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD})

	assertFrames(t, r.Transcript(), [][]byte{
		{0xFE, 0xFE, 0x88, 0xE0, 0x18, 0x01, 0xFD},
		{0xFE, 0xFE, 0x88, 0xE0, 0x19, 0x00, 0xFD},
	})
}

func TestCloseIsIdempotentAndWakesThePort(t *testing.T) {
	r := New()
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	buf := make([]byte, 8)
	if _, err := r.Port().Read(buf); err != io.EOF {
		t.Errorf("Read after Close = %v, want io.EOF", err)
	}
}

func TestOptionPanicsNameTheirMistake(t *testing.T) {
	tests := []struct {
		name string
		call func()
	}{
		{"a bank the page never prints", func() { WithSlot(6, 1, nil) }},
		{"a channel the page never prints", func() { WithSlot(1, 0, nil) }},
		{"a special channel, deliberately out of scope", func() { WithSlot(1, 106, nil) }},
		{"a record carrying the preamble byte", func() { WithSlot(1, 1, []byte{0x00, 0xFE}) }},
		{"a record carrying the end-of-message byte", func() { WithSlot(1, 1, []byte{0x00, 0xFD}) }},
		{"an empty slot outside the rectangle", func() { WithEmptySlot(0, 1) }},
		{"a record length of zero", func() { WithAcceptedRecordLength(0) }},
		{"the preamble byte as a radio address", func() { WithRadioAddress(0xFE) }},
		{"an identity token carrying the end-of-message byte", func() { WithIdentityToken([]byte{0xFD}) }},
		{"a broadcast interval of zero", func() { WithTransceiveBroadcasts(0) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("no panic")
				}
			}()
			tt.call()
		})
	}
}

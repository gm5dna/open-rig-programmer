// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft890

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// frame builds one 5-byte command: args left to right, opcode last.
func frame(opcode byte, args [4]byte) []byte {
	return []byte{args[0], args[1], args[2], args[3], opcode}
}

// readExactly reads exactly n bytes from r or fails the test. It is used
// throughout because this protocol has no terminator — a caller always
// knows in advance how many bytes a given command's reply is.
func readExactly(t *testing.T, r io.Reader, n int) []byte {
	t.Helper()
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("reading %d bytes: %v", n, err)
	}
	return buf
}

// assertSilence proves NOTHING arrives within a short window — the
// out-of-range/no-ack signal this family uses instead of an error frame.
func assertSilence(t *testing.T, port io.ReadWriteCloser) {
	t.Helper()
	_ = port.(interface{ SetReadDeadline(time.Time) error }).SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	buf := make([]byte, 1)
	n, err := port.Read(buf)
	if n != 0 || err == nil {
		t.Fatalf("expected silence, got %d byte(s), err=%v", n, err)
	}
}

func TestFullDumpLength(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	if _, err := port.Write(frame(opStatusUpdate, [4]byte{uFullDump, 0, 0, 0})); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := readExactly(t, port, fullDumpLen)
	if len(got) != fullDumpLen {
		t.Fatalf("full dump length = %d, want exactly %d (matrix §1.10)", len(got), fullDumpLen)
	}
}

func TestPerChannelStatusUpdate(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	if _, err := port.Write(frame(opStatusUpdate, [4]byte{uMemoryRecord, 0, 0, 5})); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := readExactly(t, port, recordLen)
	if len(got) != recordLen {
		t.Fatalf("channel record length = %d, want %d", len(got), recordLen)
	}
}

func TestOutOfRangeChannelIsSilence(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	// CH=0x21 (33) is one past FT-890's 32-channel range (matrix §1.8).
	if _, err := port.Write(frame(opStatusUpdate, [4]byte{uMemoryRecord, 0, 0, 0x21})); err != nil {
		t.Fatalf("Write: %v", err)
	}
	assertSilence(t, port)
}

func TestInRangeIdentityProbeAnswers(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	// CH=1 must answer (matrix §1.8's first probe).
	if _, err := port.Write(frame(opStatusUpdate, [4]byte{uMemoryRecord, 0, 0, 1})); err != nil {
		t.Fatalf("Write: %v", err)
	}
	readExactly(t, port, recordLen)
}

func TestSecondSubRecordEchoedUnmodifiedAcrossReads(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	readCh := func() []byte {
		if _, err := port.Write(frame(opStatusUpdate, [4]byte{uMemoryRecord, 0, 0, 7})); err != nil {
			t.Fatalf("Write: %v", err)
		}
		return readExactly(t, port, recordLen)
	}

	first := readCh()
	rear1 := append([]byte(nil), first[10:19]...)
	if bytes.Equal(rear1, make([]byte, 9)) {
		t.Fatal("rear sub-record is all zero — the echo test would pass vacuously")
	}

	// Store into the SAME channel (mutates the front sub-record only).
	if _, err := port.Write(frame(opStore, [4]byte{7, 0, 0, 0})); err != nil {
		t.Fatalf("Write (Store): %v", err)
	}

	second := readCh()
	rear2 := second[10:19]
	if !bytes.Equal(rear1, rear2) {
		t.Fatalf("rear sub-record changed across reads/Store: %x -> %x — this fake's rear echo must stay byte-identical (doc.go: not a preservation claim about any real radio, just this fake's own simplification)", rear1, rear2)
	}
}

func TestVFOWritesAndStoreMutateChannel(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	send := func(opcode byte, args [4]byte) {
		t.Helper()
		if _, err := port.Write(frame(opcode, args)); err != nil {
			t.Fatalf("Write opcode %#02x: %v", opcode, err)
		}
	}

	send(opABSelect, [4]byte{0, 0, 0, 0})            // VFO-A active
	send(opSetFreq, [4]byte{0x00, 0x50, 0x42, 0x01}) // 14.25000 MHz (matrix §1.1 worked example)
	send(opSetMode, [4]byte{2, 0, 0, 0})             // CW
	send(opClarifier, [4]byte{1, 0, 0x00, 0x50})     // clarifier on, +50 Hz
	send(opShift, [4]byte{2, 0, 0, 0})               // +shift
	send(opOffset, [4]byte{0x00, 0, 0x00, 0x05})     // 5 Hz-ish magnitude, accepted
	send(opTone, [4]byte{0x03, 0, 0, 0})             // tone code 3
	send(opStore, [4]byte{10, 0, 0, 0})              // VFO->M into channel 10

	if _, err := port.Write(frame(opStatusUpdate, [4]byte{uMemoryRecord, 0, 0, 10})); err != nil {
		t.Fatalf("Write: %v", err)
	}
	rec := readExactly(t, port, recordLen)

	front := rec[1:10]
	freq := uint32(front[1])<<16 | uint32(front[2])<<8 | uint32(front[3])
	if want := uint32(1425000); freq != want {
		t.Errorf("stored freq = %d tens-of-Hz, want %d", freq, want)
	}
	if mode := front[6]; mode != 2 {
		t.Errorf("stored mode = %d, want 2 (CW)", mode)
	}
	if tone := front[7]; tone != 3 {
		t.Errorf("stored tone = %#02x, want 0x03", tone)
	}
	if flags := front[8]; flags&0x10 == 0 {
		t.Errorf("stored operating flags = %#02x, want +shift bit (0x10) set", flags)
	}
	clar := int16(uint16(front[4])<<8 | uint16(front[5]))
	if clar != 50 {
		t.Errorf("stored clarifier = %d Hz, want 50", clar)
	}
}

func TestStoreOutOfRangeChannelDoesNothing(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	if _, err := port.Write(frame(opStore, [4]byte{0x21, 0, 0, 0})); err != nil {
		t.Fatalf("Write: %v", err)
	}
	assertSilence(t, port) // Store never replies anyway; this proves it truly did nothing observable
}

func TestChunkedFullDumpArrivesAsMultiplePiecesAndMatchesUnchunked(t *testing.T) {
	plain := New()
	defer plain.Close()
	if _, err := plain.Port().Write(frame(opStatusUpdate, [4]byte{uFullDump, 0, 0, 0})); err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := readExactly(t, plain.Port(), fullDumpLen)

	const chunks = 4
	const perChunkDelay = 20 * time.Millisecond
	chunked := New(WithChunkedFullDump(chunks, perChunkDelay))
	defer chunked.Close()

	start := time.Now()
	if _, err := chunked.Port().Write(frame(opStatusUpdate, [4]byte{uFullDump, 0, 0, 0})); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got := readExactly(t, chunked.Port(), fullDumpLen)
	elapsed := time.Since(start)

	if !bytes.Equal(got, want) {
		t.Fatal("chunked full dump content differs from an unchunked one")
	}
	if min := time.Duration(chunks) * perChunkDelay; elapsed < min {
		t.Errorf("elapsed %v, want at least %v (%d chunks * %v) — proves the sleeps actually happened, the mechanism a real round-trip test scales up past 1s to exercise a caller's read timeout", elapsed, min, chunks, perChunkDelay)
	}
}

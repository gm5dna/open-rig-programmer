// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft1000mp

import (
	"io"
	"testing"
	"time"
)

// sendFrame writes one 5-byte frame to the radio's port.
func sendFrame(t *testing.T, port io.Writer, args [4]byte, opcode byte) {
	t.Helper()
	frame := [frameLen]byte{args[0], args[1], args[2], args[3], opcode}
	if _, err := port.Write(frame[:]); err != nil {
		t.Fatalf("Write frame: %v", err)
	}
}

func TestFAHIdentity_FiveByteForm(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	sendFrame(t, port, [4]byte{0, 0, 0, 0x00}, opFAH) // F=00H
	got := make([]byte, 5)
	if _, err := io.ReadFull(port, got); err != nil {
		t.Fatalf("read FAH reply: %v", err)
	}
	want := []byte{0x00, 0x00, 0x00, 0x03, 0x93}
	if string(got) != string(want) {
		t.Errorf("FAH 5-byte reply = % X, want % X (matrix §1.7 identity 03H/93H)", got, want)
	}
}

func TestFAHIdentity_SixByteForm(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	sendFrame(t, port, [4]byte{0, 0, 0, 0x01}, opFAH) // F=01H
	got := make([]byte, 6)
	if _, err := io.ReadFull(port, got); err != nil {
		t.Fatalf("read FAH reply: %v", err)
	}
	for i, b := range got {
		if b != 0 {
			t.Errorf("FAH 6-byte reply[%d] = %#02x, want 0x00 (doc.go ASSUMED #1)", i, b)
		}
	}
}

func TestFullDump_Length(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	sendFrame(t, port, [4]byte{ufullDump, 0, 0, 0}, opStatusUpdate)
	got := make([]byte, fullDumpLen)
	if _, err := io.ReadFull(port, got); err != nil {
		t.Fatalf("read full dump: %v", err)
	}
	if fullDumpLen != 1863 {
		t.Fatalf("fullDumpLen = %d, want 1863 (matrix's own documented total)", fullDumpLen)
	}
}

func TestFreqToRecordBytes_WorkedExample(t *testing.T) {
	// matrix §1.3: 14.25000 MHz = 1,425,000 tens-of-Hz, read back as
	// "00 05 24 10" — the manual's own worked example, reproduced here as
	// the digit-reversal of the write side's "00 50 42 01".
	got := freqToRecordBytes(1425000)
	want := [4]byte{0x00, 0x05, 0x24, 0x10}
	if got != want {
		t.Errorf("freqToRecordBytes(1425000) = % X, want % X", got, want)
	}
}

func TestDecodeBCDField_WorkedExample(t *testing.T) {
	v, ok := decodeBCDField([4]byte{0x00, 0x50, 0x42, 0x01})
	if !ok {
		t.Fatal("decodeBCDField rejected the manual's own worked example")
	}
	if v != 1425000 {
		t.Errorf("decodeBCDField(00 50 42 01) = %d, want 1425000", v)
	}
}

// memoryOffset is the full dump's byte offset of channel ch's (1-based)
// 16-byte record — 6 flag bytes + 1 current-channel byte, then the
// current-op/VFO-A/VFO-B triple, then (ch-1) memories before it.
func memoryOffset(ch int) int {
	return 7 + recordLen*(numFixedRecords+(ch-1))
}

func TestStoreEnter_CopiesLiveVFOIntoChannel(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	sendFrame(t, port, [4]byte{0x00, 0x50, 0x42, 0x01}, opSetFreq) // 1,425,000 tens-of-Hz
	sendFrame(t, port, [4]byte{0x02, 0, 0, 0}, opSetMode)          // CW
	sendFrame(t, port, [4]byte{0x01, 0, 0, 0}, opShift)            // Minus
	sendFrame(t, port, [4]byte{storeEnter, 0, 0, 0x05}, opStore)   // Store into channel 5 (1-based)

	sendFrame(t, port, [4]byte{ufullDump, 0, 0, 0}, opStatusUpdate)
	dump := make([]byte, fullDumpLen)
	if _, err := io.ReadFull(port, dump); err != nil {
		t.Fatalf("read full dump: %v", err)
	}

	off := memoryOffset(5)
	rec := dump[off : off+recordLen]
	if got, want := [4]byte(rec[1:5]), freqToRecordBytes(1425000); got != want {
		t.Errorf("channel 5 freq bytes = % X, want % X", got, want)
	}
	if rec[7] != 0x02 {
		t.Errorf("channel 5 mode byte = %#02x, want 0x02 (CW)", rec[7])
	}
	if rec[9] != shiftFlagMinus {
		t.Errorf("channel 5 flags byte = %#02x, want %#02x (Minus)", rec[9], shiftFlagMinus)
	}

	// Channel 1-based numbering: channel 1's record must be untouched.
	off1 := memoryOffset(1)
	for i, b := range dump[off1 : off1+recordLen] {
		if b != 0 {
			t.Errorf("channel 1 byte %d = %#02x, want 0x00 — Store to channel 5 must not touch channel 1 (1-based numbering, matrix §1.4)", i, b)
		}
	}
}

func TestStoreEnter_RejectsMaskAndOutOfRange(t *testing.T) {
	r := New()
	defer r.Close()
	port := r.Port()

	sendFrame(t, port, [4]byte{0x00, 0x50, 0x42, 0x01}, opSetFreq)
	sendFrame(t, port, [4]byte{0x01, 0, 0, 0x01}, opStore)       // K=Mask, not Enter — unwired
	sendFrame(t, port, [4]byte{storeEnter, 0, 0, 0x00}, opStore) // X=0, out of 1..113
	sendFrame(t, port, [4]byte{storeEnter, 0, 0, 0x72}, opStore) // X=114, out of 1..113

	sendFrame(t, port, [4]byte{ufullDump, 0, 0, 0}, opStatusUpdate)
	dump := make([]byte, fullDumpLen)
	if _, err := io.ReadFull(port, dump); err != nil {
		t.Fatalf("read full dump: %v", err)
	}
	for ch := 1; ch <= numMemories; ch++ {
		off := memoryOffset(ch)
		for i, b := range dump[off : off+recordLen] {
			if b != 0 {
				t.Fatalf("channel %d byte %d = %#02x, want 0x00 — Mask and out-of-range Store must both be silently ignored", ch, i, b)
			}
		}
	}
}

func TestFullDumpChunking_ExceedsOneSecond(t *testing.T) {
	r := New(WithFullDumpChunking(400, 300*time.Millisecond))
	defer r.Close()
	port := r.Port()

	start := time.Now()
	sendFrame(t, port, [4]byte{ufullDump, 0, 0, 0}, opStatusUpdate)
	got := make([]byte, fullDumpLen)
	if _, err := io.ReadFull(port, got); err != nil {
		t.Fatalf("read chunked full dump: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed <= time.Second {
		t.Errorf("chunked full dump took %v, want > 1s (this test's whole point: exercising a caller's own long-read timeout)", elapsed)
	}
}

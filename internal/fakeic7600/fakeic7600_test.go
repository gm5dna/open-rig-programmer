// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7600

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// mkFrame, answer and exchange are hand-built, independently of this
// package's own answerFrame/parser (named mkFrame rather than frame to
// avoid colliding with parser.go's own frame type) — the same discipline
// internal/fakeic7610, internal/fakeic7851 and internal/fakeic7800 all
// follow, so a bug shared between the fake's protocol code and its own
// tests cannot hide behind agreement with itself.
func mkFrame(to, from byte, payload ...byte) []byte {
	return append([]byte{0xFE, 0xFE, to, from}, append(payload, 0xFD)...)
}
func answer(payload ...byte) []byte { return mkFrame(AddrController, AddrRadio, payload...) }

func exchange(t *testing.T, c net.Conn, req []byte) []byte {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	if _, err := c.Write(req); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4096)
	n, err := c.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte(nil), b[:n]...)
}

// exchangeSilent writes req and expects the read to time out (or return an
// error): the radio must not answer.
func exchangeSilent(t *testing.T, c net.Conn, req []byte) ([]byte, error) {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(150 * time.Millisecond))
	if _, err := c.Write(req); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4096)
	n, err := c.Read(b)
	return append([]byte(nil), b[:n]...), err
}

func rec(fill byte) []byte { return bytes.Repeat([]byte{fill}, RecordLen) }

// TestRecordLengthIsTheMatrixS311Sum re-does the matrix §3.11 arithmetic in
// the test, so a change to the constant has to argue with the numbers
// rather than merely move them: 1 (select) + 5 (freq) + 2 (mode/filter) +
// 1 (tone-type/data-mode nibble pair) + 3 (tone_tx) + 3 (tone_rx) +
// 10 (name) = 25, excluding the two channel-selector bytes.
func TestRecordLengthIsTheMatrixS311Sum(t *testing.T) {
	sum := 1 + 5 + 2 + 1 + 3 + 3 + 10
	if sum != RecordLen {
		t.Fatalf("matrix §3.11 field widths sum to %d, RecordLen = %d", sum, RecordLen)
	}
}

// TestGoldenVectorsReadAndSet replays core/civ/ic7600/testdata/
// ic7600-vectors.golden's own read-record and set-record-name-with-space
// frames byte for byte, so this independently-authored fake is checked
// against the same frozen evidence the production codec's own tests are.
// Byte 17 (the record's byte 11, data-mode/tone-mode nibble pair) is 0x01
// here — LOW nibble TONE, HIGH nibble data-mode OFF — per the golden
// vector's own worked example (ic7600-golden-assumptions.csv), the
// opposite nibble assignment from internal/fakeic7800's own fixture (see
// doc.go, "Record length"). This package never interprets the byte either
// way; the test only proves the stored bytes round-trip unchanged.
func TestGoldenVectorsReadAndSet(t *testing.T) {
	r := New()
	defer r.Close()

	name := []byte("HOME QTH01")
	if len(name) != NameLen {
		t.Fatalf("golden fixture name is %d bytes, want %d", len(name), NameLen)
	}
	set := append([]byte{
		0x00,                         // select memory setting: OFF
		0x00, 0x00, 0x25, 0x14, 0x00, // frequency, 5 bytes
		0x01, 0x01, // mode USB, filter FIL1
		0x01,             // data mode OFF (hi) / tone mode TONE (lo)
		0x00, 0x08, 0x85, // tone_tx 88.5 Hz
		0x00, 0x10, 0x00, // tone_rx 100.0 Hz
	}, name...)
	if len(set) != RecordLen {
		t.Fatalf("built golden set record is %d bytes, want %d", len(set), RecordLen)
	}

	setFrame := mkFrame(AddrRadio, AddrController, append([]byte{cnMemory, scMemory, 0x00, 0x01}, set...)...)
	wantSetFrame := []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x00,
		0x00, 0x01, 0x00, 0x00, 0x00, 0x25, 0x14, 0x00, 0x01, 0x01, 0x01, 0x00, 0x08, 0x85,
		0x00, 0x10, 0x00, 0x48, 0x4F, 0x4D, 0x45, 0x20, 0x51, 0x54, 0x48, 0x30, 0x31, 0xFD}
	if !bytes.Equal(setFrame, wantSetFrame) {
		t.Fatalf("built set frame = % X, want the golden vector % X (core/civ/ic7600/testdata/ic7600-vectors.golden, set-record-name-with-space)", setFrame, wantSetFrame)
	}

	if got := exchange(t, r.Port(), setFrame); !bytes.Equal(got, answer(CodeOK)) {
		t.Fatalf("set = % X, want OK", got)
	}

	readFrame := mkFrame(AddrRadio, AddrController, cnMemory, scMemory, 0x00, 0x01)
	wantReadFrame := []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFD}
	if !bytes.Equal(readFrame, wantReadFrame) {
		t.Fatalf("built read frame = % X, want the golden vector % X (read-record)", readFrame, wantReadFrame)
	}

	want := answer(append([]byte{cnMemory, scMemory, 0x00, 0x01}, set...)...)
	if got := exchange(t, r.Port(), readFrame); !bytes.Equal(got, want) {
		t.Fatalf("read = % X, want % X", got, want)
	}
}

// TestTransceiverIDCommandIsRecognised replays the golden vector's
// read-transceiver-id request and checks only that this radio answers with
// something addressed back to the controller carrying cn/sc 19 00 — not the
// data bytes, which are this package's own invented token (doc.go).
func TestTransceiverIDCommandIsRecognised(t *testing.T) {
	r := New()
	defer r.Close()
	req := []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x19, 0x00, 0xFD}
	got := exchange(t, r.Port(), req)
	if len(got) < 6 || got[0] != 0xFE || got[1] != 0xFE || got[2] != AddrController || got[3] != AddrRadio || got[4] != cnID || got[5] != scID {
		t.Fatalf("19 00 answer = % X, want a frame headed FE FE E0 7A 19 00", got)
	}
}

// TestPreamblePaddingIsTolerated replays the golden vector's
// manual-example-14 padded frame (nine leading 0xFE bytes before the
// address pair) to prove the reassembler skips the whole run — and, since
// this radio recognises no 18 01, that it is refused with NG rather than
// silently dropped.
func TestPreamblePaddingIsTolerated(t *testing.T) {
	r := New()
	defer r.Close()
	padded := []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0x7A, 0xE0, 0x18, 0x01, 0xFD}
	if got := exchange(t, r.Port(), padded); !bytes.Equal(got, answer(CodeNG)) {
		t.Fatalf("padded 18 01 = % X, want NG", got)
	}
}

// TestScanEdgeSelectorsAddressP1AndP2 pins the complete flat selector space
// matrix §1b names: 0001-0099 memories, 0100 P1, 0101 P2.
func TestScanEdgeSelectorsAddressP1AndP2(t *testing.T) {
	r := New()
	defer r.Close()
	r.SetSlot(ChanP1, MemState{Raw: rec(0x21)})
	r.SetSlot(ChanP2, MemState{Raw: rec(0x22)})
	r.SetSlot(99, MemState{Raw: rec(0x99)})

	for _, tc := range []struct {
		name string
		sel  [2]byte
		want []byte
	}{
		{"099", [2]byte{0x00, 0x99}, rec(0x99)},
		{"P1", [2]byte{0x01, 0x00}, rec(0x21)},
		{"P2", [2]byte{0x01, 0x01}, rec(0x22)},
	} {
		t.Run("read "+tc.name, func(t *testing.T) {
			got := exchange(t, r.Port(), mkFrame(AddrRadio, AddrController, cnMemory, scMemory, tc.sel[0], tc.sel[1]))
			want := answer(append([]byte{cnMemory, scMemory, tc.sel[0], tc.sel[1]}, tc.want...)...)
			if !bytes.Equal(got, want) {
				t.Fatalf("read %s = % X, want % X", tc.name, got, want)
			}
		})
	}

	// A set addressed to P2 over the wire must land where SetSlot/SlotState
	// look, or the fake's two halves disagree about the same channel.
	written := rec(0x33)
	setP2 := mkFrame(AddrRadio, AddrController, append([]byte{cnMemory, scMemory, 0x01, 0x01}, written...)...)
	if got := exchange(t, r.Port(), setP2); !bytes.Equal(got, answer(CodeOK)) {
		t.Fatalf("set P2 = % X, want OK", got)
	}
	st, ok := r.SlotState(ChanP2)
	if !ok || !bytes.Equal(st.Raw, written) {
		t.Fatalf("SlotState(ChanP2) = %v % X, want true % X", ok, st.Raw, written)
	}
}

// TestSelectorsOutsideTheFlatSpaceAreRefused keeps 0102-and-beyond, and
// non-BCD nibbles, outside the 1-101 space the matrix names.
func TestSelectorsOutsideTheFlatSpaceAreRefused(t *testing.T) {
	r := New()
	defer r.Close()
	for _, sel := range [][2]byte{
		{0x00, 0x00}, // 0000: below the space
		{0x01, 0x02}, // 0102: one past P2
		{0x02, 0x00}, // 0200: far past P2
		{0x00, 0x9A}, // non-BCD low nibble
		{0x00, 0xA0}, // non-BCD high nibble
		{0x10, 0x00}, // non-zero thousands digit
	} {
		if got := exchange(t, r.Port(), mkFrame(AddrRadio, AddrController, cnMemory, scMemory, sel[0], sel[1])); !bytes.Equal(got, answer(CodeNG)) {
			t.Fatalf("selector % X = % X, want NG", sel, got)
		}
	}
}

// TestEmptyChannelReplyModes pins matrix §3.8(a)'s default (NG) and the
// §3.8(b) alternative WithAllFFEmpty offers, on an otherwise identical
// radio, for the same unset channel.
func TestEmptyChannelReplyModes(t *testing.T) {
	req := mkFrame(AddrRadio, AddrController, cnMemory, scMemory, 0x00, 0x01)

	t.Run("default NG", func(t *testing.T) {
		r := New()
		defer r.Close()
		if got := exchange(t, r.Port(), req); !bytes.Equal(got, answer(CodeNG)) {
			t.Fatalf("empty read = % X, want NG", got)
		}
	})
	t.Run("WithAllFFEmpty", func(t *testing.T) {
		r := New(WithAllFFEmpty())
		defer r.Close()
		want := answer(append([]byte{cnMemory, scMemory, 0x00, 0x01}, rec(0xFF)...)...)
		if got := exchange(t, r.Port(), req); !bytes.Equal(got, want) {
			t.Fatalf("empty read (all-FF) = % X, want % X", got, want)
		}
	})

	// Both scan edges answer the same as an unset memory channel — WIDER
	// than the single capture matrix §3.8(a) names; see doc.go.
	r := New()
	defer r.Close()
	for _, sel := range [][2]byte{{0x01, 0x00}, {0x01, 0x01}} {
		if got := exchange(t, r.Port(), mkFrame(AddrRadio, AddrController, cnMemory, scMemory, sel[0], sel[1])); !bytes.Equal(got, answer(CodeNG)) {
			t.Fatalf("unset scan edge % X = % X, want NG", sel, got)
		}
	}
}

// TestDeliberateRefusals pins the printed clear form, "Memory clear" (0B),
// and the menu surface (1A 05) as REFUSED, deliberately — matrix §3.13;
// doc.go, "What this radio answers, and what it refuses".
func TestDeliberateRefusals(t *testing.T) {
	r := New()
	defer r.Close()
	r.SetSlot(1, MemState{Raw: rec(0x77)})

	for _, tc := range []struct {
		name string
		req  []byte
	}{
		{"1A 00 clear form (selector + FF)", mkFrame(AddrRadio, AddrController, cnMemory, scMemory, 0x00, 0x01, 0xFF)},
		{"0B memory clear", mkFrame(AddrRadio, AddrController, cnClear)},
		{"1A 05 menu surface", mkFrame(AddrRadio, AddrController, cnMemory, scMenu, 0x00, 0x52)},
		{"no command at all", mkFrame(AddrRadio, AddrController)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := exchange(t, r.Port(), tc.req); !bytes.Equal(got, answer(CodeNG)) {
				t.Fatalf("%s = % X, want NG", tc.name, got)
			}
		})
	}

	// The refused clear must not have touched channel 1.
	st, ok := r.SlotState(1)
	if !ok || !bytes.Equal(st.Raw, rec(0x77)) {
		t.Fatalf("channel 1 after refused clear = %v % X, want untouched", ok, st.Raw)
	}
	// Three of the four refusals above carried a command byte and were
	// logged; "no command at all" has nothing to log (handleFrame returns
	// before logCommand for that case, matching internal/fakeic7610's own
	// precedent) — it is still refused, just not a "seen command".
	log := r.CommandLog()
	if len(log) != 3 {
		t.Fatalf("CommandLog has %d entries, want 3 (the clear form, 0B and 1A 05 — not the commandless frame)", len(log))
	}
}

// TestShortSetHandling pins matrix §3.10's default (a short record is
// refused, same as a long one) and the WithShortSetAccepted alternative
// this package offers (doc.go, register FAKE-3).
func TestShortSetHandling(t *testing.T) {
	short := rec(0x11)[:RecordLen-3]
	long := append(rec(0x11), 0x11, 0x11, 0x11)
	req := func(payload []byte) []byte {
		return mkFrame(AddrRadio, AddrController, append([]byte{cnMemory, scMemory, 0x00, 0x02}, payload...)...)
	}

	t.Run("default refuses short and long", func(t *testing.T) {
		r := New()
		defer r.Close()
		if got := exchange(t, r.Port(), req(short)); !bytes.Equal(got, answer(CodeNG)) {
			t.Fatalf("short set = % X, want NG", got)
		}
		if got := exchange(t, r.Port(), req(long)); !bytes.Equal(got, answer(CodeNG)) {
			t.Fatalf("long set = % X, want NG", got)
		}
		if _, ok := r.SlotState(2); ok {
			t.Fatal("a refused set wrote a record")
		}
	})

	t.Run("WithShortSetAccepted pads short, still refuses long", func(t *testing.T) {
		r := New(WithShortSetAccepted())
		defer r.Close()
		if got := exchange(t, r.Port(), req(short)); !bytes.Equal(got, answer(CodeOK)) {
			t.Fatalf("short set = % X, want OK", got)
		}
		want := append(append([]byte(nil), short...), make([]byte, RecordLen-len(short))...)
		st, ok := r.SlotState(2)
		if !ok || !bytes.Equal(st.Raw, want) {
			t.Fatalf("stored record = %v % X, want % X (zero-padded tail)", ok, st.Raw, want)
		}
		if got := exchange(t, r.Port(), req(long)); !bytes.Equal(got, answer(CodeNG)) {
			t.Fatalf("long set under WithShortSetAccepted = % X, want NG", got)
		}
	})
}

// TestOnlyThisRadioIsAnswered pins matrix §3.4's address filter: a frame
// whose `to` byte is not 0x7A gets no reply, no state change and no
// CommandLog entry, and the radio still serves its own address afterwards.
func TestOnlyThisRadioIsAnswered(t *testing.T) {
	r := New()
	defer r.Close()
	for _, to := range []byte{0x00, 0x98, 0x8E, 0x7B} {
		if got, err := exchangeSilent(t, r.Port(), mkFrame(to, AddrController, cnID, scID)); err == nil {
			t.Fatalf("to=%02X answered with % X", to, got)
		}
	}
	if len(r.CommandLog()) != 0 {
		t.Fatalf("CommandLog has entries after frames addressed elsewhere, want none")
	}
	got := exchange(t, r.Port(), mkFrame(AddrRadio, AddrController, cnID, scID))
	if len(got) < 4 || got[2] != AddrController || got[3] != AddrRadio {
		t.Fatalf("after ignored frames, own address unanswered: % X", got)
	}
}

// TestFloods pins the two flood shapes: broadcast (to=00, ASSUMED matrix
// §3.5(b)) and controller-addressed (to=E0, synthetic) — each the ID
// answer with `to` swapped, and independent of one another.
func TestFloods(t *testing.T) {
	r := New(WithTransceiveFlood(2*time.Millisecond), WithIDToken([]byte{0x11, 0x22}))
	c := r.Port()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	b := make([]byte, 64)
	n, err := c.Read(b)
	if err != nil {
		t.Fatalf("no broadcast flood frame arrived: %v", err)
	}
	want := mkFrame(AddrBroadcast, AddrRadio, cnID, scID, 0x11, 0x22)
	if got := b[:n]; !bytes.Equal(got, want) {
		t.Fatalf("broadcast flood frame = % X, want % X", got, want)
	}
	r.StopFloods()
	if err := r.Close(); err != nil {
		t.Fatalf("Close after stopped flood: %v", err)
	}

	r2 := New(WithAddressedFlood(2*time.Millisecond), WithIDToken([]byte{0x33}))
	defer r2.Close()
	c2 := r2.Port()
	_ = c2.SetDeadline(time.Now().Add(2 * time.Second))
	n2, err2 := c2.Read(b)
	if err2 != nil {
		t.Fatalf("no addressed flood frame arrived: %v", err2)
	}
	want2 := mkFrame(AddrController, AddrRadio, cnID, scID, 0x33)
	if got := b[:n2]; !bytes.Equal(got, want2) {
		t.Fatalf("addressed flood frame = % X, want % X", got, want2)
	}
}

// TestCloseUnderFlood pins clean shutdown while both flood goroutines are
// mid-write against an unread port — the outQueue exists precisely so this
// does not deadlock the writer goroutine.
func TestCloseUnderFlood(t *testing.T) {
	r := New(WithTransceiveFlood(time.Millisecond), WithAddressedFlood(time.Millisecond))
	time.Sleep(20 * time.Millisecond)
	closed := make(chan error, 1)
	go func() { closed <- r.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked under flood")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close = %v", err)
	}
}

// TestSetSlotPanicsOnBadInput pins the two programming-error panics: an
// unaddressable channel, and a record of the wrong length.
func TestSetSlotPanicsOnBadInput(t *testing.T) {
	r := New()
	defer r.Close()

	mustPanic := func(t *testing.T, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatal("expected a panic")
			}
		}()
		fn()
	}
	mustPanic(t, func() { r.SetSlot(0, MemState{Raw: rec(0x00)}) })
	mustPanic(t, func() { r.SetSlot(100, MemState{Raw: rec(0x00)}) })
	mustPanic(t, func() { r.SetSlot(1, MemState{Raw: rec(0x00)[:RecordLen-1]}) })
}

// TestWithRecordLengthOverride proves the length check moves with the
// option, in both directions.
func TestWithRecordLengthOverride(t *testing.T) {
	r := New(WithRecordLength(4))
	defer r.Close()
	rec4 := []byte{0xAA, 0xBB, 0xCC, 0xDD}
	setFrame := mkFrame(AddrRadio, AddrController, append([]byte{cnMemory, scMemory, 0x00, 0x01}, rec4...)...)
	if got := exchange(t, r.Port(), setFrame); !bytes.Equal(got, answer(CodeOK)) {
		t.Fatalf("4-byte set = % X, want OK", got)
	}
	// The default 25-byte length is now refused.
	setFrame25 := mkFrame(AddrRadio, AddrController, append([]byte{cnMemory, scMemory, 0x00, 0x02}, rec(0x01)...)...)
	if got := exchange(t, r.Port(), setFrame25); !bytes.Equal(got, answer(CodeNG)) {
		t.Fatalf("25-byte set under WithRecordLength(4) = % X, want NG", got)
	}
}

func TestWithRecordLengthPanicsOnZero(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	WithRecordLength(0)(&config{})
}

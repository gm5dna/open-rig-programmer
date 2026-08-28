// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300mk2

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"
)

const testTimeout = 3 * time.Second

// request builds a frame FROM the controller TO the given radio address. It is
// written out here by hand rather than by calling the package's own frame
// builder, so that a bug in that builder cannot cancel itself out.
func request(to byte, payload ...byte) []byte {
	f := []byte{0xFE, 0xFE, to, 0xE0}
	f = append(f, payload...)
	return append(f, 0xFD)
}

type readDeadliner interface{ SetReadDeadline(time.Time) error }

// readFrame reads bytes from the port until an FD arrives and returns
// everything read, framing included. It fails the test on timeout.
func readFrame(t *testing.T, port io.ReadWriteCloser) []byte {
	t.Helper()
	if d, ok := port.(readDeadliner); ok {
		_ = d.SetReadDeadline(time.Now().Add(testTimeout))
		defer func() { _ = d.SetReadDeadline(time.Time{}) }()
	}
	var out []byte
	b := make([]byte, 1)
	for {
		n, err := port.Read(b)
		if n > 0 {
			out = append(out, b[0])
			if b[0] == 0xFD {
				return out
			}
		}
		if err != nil {
			t.Fatalf("reading a frame: %v (got %X so far)", err, out)
		}
	}
}

// exchange writes one request and reads one whole frame back.
func exchange(t *testing.T, r *Radio, req []byte) []byte {
	t.Helper()
	if _, err := r.Port().Write(req); err != nil {
		t.Fatalf("writing %X: %v", req, err)
	}
	return readFrame(t, r.Port())
}

// newRadio builds a radio and registers its Close.
func newRadio(t *testing.T, opts ...Option) *Radio {
	t.Helper()
	r := New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func wantFrame(t *testing.T, got, want []byte, what string) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("%s: got %X, want %X", what, got, want)
	}
}

// eventually polls cond until it holds or the timeout expires.
//
// Sent() needs it, and Received() does not. A frame is recorded in Sent() only
// once its write has fully landed — and on a net.Pipe the write lands when the
// READER consumes the last byte, so the test goroutine can be back from
// readFrame a hair before the radio's goroutine has finished recording. That
// is a real ordering, not a flake to paper over: a frame is on the wire before
// the writer knows it got there.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// wantSentCount waits for Sent() to reach n frames and then requires it to
// stay there.
func wantSentCount(t *testing.T, r *Radio, n int, what string) {
	t.Helper()
	eventually(t, what, func() bool { return len(r.Sent()) >= n })
	if got := len(r.Sent()); got != n {
		t.Errorf("%s: Sent() has %d frames, want %d", what, got, n)
	}
}

// memRead builds a 1A 00 read of one channel address.
func memRead(to, b0, b1 byte) []byte { return request(to, 0x1A, 0x00, b0, b1) }

// memSet builds a 1A 00 set of one channel address.
func memSet(to, b0, b1 byte, rec []byte) []byte {
	p := append([]byte{0x1A, 0x00, b0, b1}, rec...)
	return request(to, p...)
}

// ---------------------------------------------------------------------------
// The two acknowledgement frames
// ---------------------------------------------------------------------------

// TestAcknowledgementFramesAreSixBytes pins the two frames byte for byte,
// against literals rather than against the builders that make them.
func TestAcknowledgementFramesAreSixBytes(t *testing.T) {
	r := newRadio(t)
	if got, want := r.pass(), []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFB, 0xFD}; !bytes.Equal(got, want) {
		t.Errorf("PASS frame = %X, want %X", got, want)
	}
	if got, want := r.fail(), []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}; !bytes.Equal(got, want) {
		t.Errorf("FAIL frame = %X, want %X", got, want)
	}
}

// ---------------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------------

func TestIdentityReadAnswersTheConfiguredAddress(t *testing.T) {
	r := newRadio(t)
	got := exchange(t, r, request(0xB6, 0x19, 0x00))
	wantFrame(t, got, []byte{0xFE, 0xFE, 0xE0, 0xB6, 0x19, 0x00, 0xB6, 0xFD}, "identity answer")
}

func TestIdentityReadWithIDToken(t *testing.T) {
	r := newRadio(t, WithIDToken([]byte{0x9C, 0x41}))
	got := exchange(t, r, request(0xB6, 0x19, 0x00))
	wantFrame(t, got, []byte{0xFE, 0xFE, 0xE0, 0xB6, 0x19, 0x00, 0x9C, 0x41, 0xFD}, "identity answer with a fixed token")
}

func TestIdentityReadIsExactlyOneNine00(t *testing.T) {
	r := newRadio(t)
	fail := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}
	for _, req := range [][]byte{
		request(0xB6, 0x19),             // no sub-command
		request(0xB6, 0x19, 0x01),       // a sub-command this radio has not got
		request(0xB6, 0x19, 0x00, 0x00), // "sent with no data area" — this has one
	} {
		wantFrame(t, exchange(t, r, req), fail, "identity variant")
	}
}

func TestWithIDTokenPanicsOnAnEmptyToken(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("WithIDToken(nil) did not panic — the identity answer carries at least one data byte")
		}
	}()
	New(WithIDToken(nil))
}

// ---------------------------------------------------------------------------
// The memory command
// ---------------------------------------------------------------------------

func TestReadOfAChannelNeverWrittenFails(t *testing.T) {
	r := newRadio(t)
	fail := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}
	for _, addr := range [][2]byte{{0x00, 0x01}, {0x00, 0x99}, {0x01, 0x00}, {0x01, 0x01}} {
		wantFrame(t, exchange(t, r, memRead(0xB6, addr[0], addr[1])), fail, "read of an empty channel")
	}
}

func TestSetThenRead(t *testing.T) {
	r := newRadio(t)
	rec := validTestRecord()

	// Channel 042: packed BCD, so 00 42 and not 00 2A.
	got := exchange(t, r, memSet(0xB6, 0x00, 0x42, rec))
	wantFrame(t, got, []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFB, 0xFD}, "set answer")

	stored, ok := r.Channel("042")
	if !ok {
		t.Fatal("Channel(\"042\") reports nothing stored after an accepted set")
	}
	if !bytes.Equal(stored, rec) {
		t.Errorf("stored record = %X, want %X", stored, rec)
	}

	want := append([]byte{0xFE, 0xFE, 0xE0, 0xB6, 0x1A, 0x00, 0x00, 0x42}, rec...)
	want = append(want, 0xFD)
	wantFrame(t, exchange(t, r, memRead(0xB6, 0x00, 0x42)), want, "read answer")
}

func TestScanEdgesAreOrdinaryChannelsForReadAndSet(t *testing.T) {
	r := newRadio(t)
	rec := validTestRecord()
	for _, e := range []struct {
		slot   string
		b0, b1 byte
	}{{"P1", 0x01, 0x00}, {"P2", 0x01, 0x01}} {
		wantFrame(t, exchange(t, r, memSet(0xB6, e.b0, e.b1, rec)),
			[]byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFB, 0xFD}, "set of "+e.slot)
		if _, ok := r.Channel(e.slot); !ok {
			t.Errorf("Channel(%q) reports nothing stored", e.slot)
		}
		want := append([]byte{0xFE, 0xFE, 0xE0, 0xB6, 0x1A, 0x00, e.b0, e.b1}, rec...)
		want = append(want, 0xFD)
		wantFrame(t, exchange(t, r, memRead(0xB6, e.b0, e.b1)), want, "read of "+e.slot)
	}
}

func TestSetOfTheWrongLengthFails(t *testing.T) {
	r := newRadio(t)
	fail := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}
	rec := validTestRecord()

	for _, n := range []int{1, 2, 44, 46, 47, 90} {
		body := make([]byte, n)
		copy(body, rec)
		wantFrame(t, exchange(t, r, memSet(0xB6, 0x00, 0x01, body)), fail, "a set whose record is the wrong length")
	}
	if _, ok := r.Channel("001"); ok {
		t.Error("a rejected set stored something")
	}
}

func TestSetOfAValueTheManualDoesNotPrintFails(t *testing.T) {
	r := newRadio(t)
	fail := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}

	for _, tt := range []struct {
		name  string
		off   int
		value byte
	}{
		{"mode 06", offMode, 0x06},
		{"filter 00", offMode + 1, 0x00},
		{"a frequency nibble above 9", offFrequency, 0xA0},
		{"the 10 MHz digit above 7", offFrequency + 3, 0x80},
		{"a NUL in the name", offName, 0x00},
		{"a tone squelch first byte that is not 00", offToneSquelch, 0x11},
		{"a transmit-side mode the table does not print", offShadowMode, 0x0F},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := validTestRecord()
			rec[tt.off] = tt.value
			wantFrame(t, exchange(t, r, memSet(0xB6, 0x00, 0x07, rec)), fail, "set carrying "+tt.name)
			if _, ok := r.Channel("007"); ok {
				t.Error("a rejected set stored something")
			}
		})
	}
}

func TestSetToAnAddressOutsideTheThreeFormsFails(t *testing.T) {
	r := newRadio(t)
	fail := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}
	rec := validTestRecord()
	for _, addr := range [][2]byte{{0x00, 0x00}, {0x00, 0x9A}, {0x01, 0x02}, {0x02, 0x00}, {0xFF, 0xFF}} {
		wantFrame(t, exchange(t, r, memSet(0xB6, addr[0], addr[1], rec)), fail, "set to an unprinted address")
		wantFrame(t, exchange(t, r, memRead(0xB6, addr[0], addr[1])), fail, "read of an unprinted address")
	}
	if n := len(r.Channels()); n != 0 {
		t.Errorf("%d channels stored, want 0", n)
	}
}

// ---------------------------------------------------------------------------
// Refusals
// ---------------------------------------------------------------------------

// TestTheClearFormsAreRefused is doc.go's "Why the clear forms are refused",
// pinned: the documented clear forms answer FAIL and leave the stored record
// exactly where it was — including at the two scan edges.
func TestTheClearFormsAreRefused(t *testing.T) {
	fail := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}
	rec := validTestRecord()

	for _, e := range []struct {
		slot   string
		b0, b1 byte
	}{{"001", 0x00, 0x01}, {"P1", 0x01, 0x00}, {"P2", 0x01, 0x01}} {
		t.Run(e.slot, func(t *testing.T) {
			r := newRadio(t, WithChannel(e.slot, rec))

			// 1A 00 <address> FF, the documented per-channel clear.
			wantFrame(t, exchange(t, r, memSet(0xB6, e.b0, e.b1, []byte{0xFF})), fail, "1A 00 clear")
			// 0B, the documented clear of the selected channel.
			wantFrame(t, exchange(t, r, request(0xB6, 0x0B)), fail, "0B clear")

			stored, ok := r.Channel(e.slot)
			if !ok {
				t.Fatalf("Channel(%q) is empty — a refused clear destroyed the record", e.slot)
			}
			if !bytes.Equal(stored, rec) {
				t.Errorf("stored record changed to %X", stored)
			}
		})
	}
}

func TestEverythingElseFails(t *testing.T) {
	r := newRadio(t)
	fail := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}
	for _, tt := range []struct {
		name string
		req  []byte
	}{
		{"0B, memory clear", request(0xB6, 0x0B)},
		{"18 00, power off", request(0xB6, 0x18, 0x00)},
		{"18 01, power on", request(0xB6, 0x18, 0x01)},
		{"1A 05, a set-mode item", request(0xB6, 0x1A, 0x05, 0x01, 0x07)},
		{"1A 01, another sub-command", request(0xB6, 0x1A, 0x01)},
		{"1A alone", request(0xB6, 0x1A)},
		{"03, read frequency", request(0xB6, 0x03)},
		{"a frame with no command byte at all", request(0xB6)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			wantFrame(t, exchange(t, r, tt.req), fail, tt.name)
		})
	}
}

// ---------------------------------------------------------------------------
// Addressing
// ---------------------------------------------------------------------------

func TestFramesAddressedElsewhereAreCountedAndIgnored(t *testing.T) {
	r := newRadio(t)

	elsewhere := request(0x94, 0x19, 0x00) // an IC-7300 MK1's default address
	if _, err := r.Port().Write(elsewhere); err != nil {
		t.Fatalf("writing: %v", err)
	}
	// The only way to prove silence is to prove the NEXT frame's answer is the
	// first thing on the wire.
	wantFrame(t, exchange(t, r, request(0xB6, 0x19, 0x00)),
		[]byte{0xFE, 0xFE, 0xE0, 0xB6, 0x19, 0x00, 0xB6, 0xFD}, "identity answer after an ignored frame")

	got := r.Received()
	if len(got) != 2 {
		t.Fatalf("Received() has %d frames, want 2 — an ignored frame is still counted", len(got))
	}
	wantFrame(t, got[0], elsewhere, "the ignored frame is in Received()")
	wantSentCount(t, r, 1, "the ignored frame must not have been answered")
}

// TestWithRadioAddressMovesEveryAnswer is the register's "the radio's address
// is not a literal" claim, pinned on all four answer kinds.
func TestWithRadioAddressMovesEveryAnswer(t *testing.T) {
	const addr = 0x22
	rec := validTestRecord()
	r := newRadio(t, WithRadioAddress(addr), WithChannel("001", rec))

	wantFrame(t, exchange(t, r, request(addr, 0x19, 0x00)),
		[]byte{0xFE, 0xFE, 0xE0, addr, 0x19, 0x00, addr, 0xFD}, "identity answer")

	wantRead := append([]byte{0xFE, 0xFE, 0xE0, addr, 0x1A, 0x00, 0x00, 0x01}, rec...)
	wantRead = append(wantRead, 0xFD)
	wantFrame(t, exchange(t, r, memRead(addr, 0x00, 0x01)), wantRead, "record answer")

	wantFrame(t, exchange(t, r, memSet(addr, 0x00, 0x02, rec)),
		[]byte{0xFE, 0xFE, 0xE0, addr, 0xFB, 0xFD}, "OK frame")

	wantFrame(t, exchange(t, r, request(addr, 0x0B)),
		[]byte{0xFE, 0xFE, 0xE0, addr, 0xFA, 0xFD}, "NG frame")

	// And the default address is now somebody else's.
	if _, err := r.Port().Write(request(0xB6, 0x19, 0x00)); err != nil {
		t.Fatalf("writing: %v", err)
	}
	wantFrame(t, exchange(t, r, request(addr, 0x19, 0x00)),
		[]byte{0xFE, 0xFE, 0xE0, addr, 0x19, 0x00, addr, 0xFD}, "a frame to B6 was ignored")
}

// TestSameAddressCollision is the whole reason WithRadioAddress exists: a
// radio configured with the CONTROLLER's address sends answers whose `to` and
// `from` are both E0.
func TestSameAddressCollision(t *testing.T) {
	r := newRadio(t, WithRadioAddress(0xE0))
	got := exchange(t, r, request(0xE0, 0x19, 0x00))
	wantFrame(t, got, []byte{0xFE, 0xFE, 0xE0, 0xE0, 0x19, 0x00, 0xE0, 0xFD}, "the collision answer")
}

// ---------------------------------------------------------------------------
// Framing
// ---------------------------------------------------------------------------

func TestLeadingNoiseAndExtraPreamblesAreTolerated(t *testing.T) {
	r := newRadio(t)
	noisy := []byte{0x00, 0xFF, 0x55, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xB6, 0xE0, 0x19, 0x00, 0xFD}
	if _, err := r.Port().Write(noisy); err != nil {
		t.Fatalf("writing: %v", err)
	}
	wantFrame(t, readFrame(t, r.Port()),
		[]byte{0xFE, 0xFE, 0xE0, 0xB6, 0x19, 0x00, 0xB6, 0xFD}, "answer to a noisy request")

	got := r.Received()
	if len(got) != 1 {
		t.Fatalf("Received() has %d frames, want 1", len(got))
	}
	// Normalised: the noise is gone and the five preamble bytes are two.
	wantFrame(t, got[0], []byte{0xFE, 0xFE, 0xB6, 0xE0, 0x19, 0x00, 0xFD}, "the normalised frame")
}

func TestReassembler(t *testing.T) {
	frame := []byte{0xFE, 0xFE, 0xB6, 0xE0, 0x19, 0x00, 0xFD}

	t.Run("a whole frame in one push", func(t *testing.T) {
		a := &reassembler{max: maxFrameBytes}
		out := a.push(frame)
		if len(out) != 1 || !bytes.Equal(out[0], frame) {
			t.Fatalf("push = %X, want one %X", out, frame)
		}
	})

	t.Run("a frame arriving one byte at a time", func(t *testing.T) {
		a := &reassembler{max: maxFrameBytes}
		var out [][]byte
		for _, b := range frame {
			out = append(out, a.push([]byte{b})...)
		}
		if len(out) != 1 || !bytes.Equal(out[0], frame) {
			t.Fatalf("got %X, want one %X", out, frame)
		}
	})

	t.Run("a lone preamble byte is noise", func(t *testing.T) {
		a := &reassembler{max: maxFrameBytes}
		out := a.push([]byte{0xFE, 0xB6, 0xE0, 0x19, 0x00, 0xFD})
		if len(out) != 0 {
			t.Fatalf("got %X, want nothing — the preamble is two bytes", out)
		}
		if out := a.push(frame); len(out) != 1 {
			t.Fatalf("the reassembler did not resynchronise: got %X", out)
		}
	})

	t.Run("an FE inside a body restarts the frame", func(t *testing.T) {
		a := &reassembler{max: maxFrameBytes}
		out := a.push([]byte{0xFE, 0xFE, 0xB6, 0xE0, 0x19})
		if len(out) != 0 {
			t.Fatalf("got %X, want nothing yet", out)
		}
		out = a.push(frame) // its leading FE FE aborts the partial body
		if len(out) != 1 || !bytes.Equal(out[0], frame) {
			t.Fatalf("got %X, want one %X", out, frame)
		}
	})

	t.Run("an empty frame is not a frame", func(t *testing.T) {
		a := &reassembler{max: maxFrameBytes}
		if out := a.push([]byte{0xFE, 0xFE, 0xFD}); len(out) != 0 {
			t.Fatalf("got %X, want nothing — there is no body to address", out)
		}
	})

	t.Run("the accumulator is capped", func(t *testing.T) {
		a := &reassembler{max: 64}
		junk := make([]byte, 0, 200)
		junk = append(junk, 0xFE, 0xFE)
		for i := 0; i < 200; i++ {
			junk = append(junk, 0x01)
		}
		if out := a.push(junk); len(out) != 0 {
			t.Fatalf("got %X, want nothing", out)
		}
		if len(a.body) > a.max {
			t.Errorf("accumulator grew to %d bytes, past the cap of %d", len(a.body), a.max)
		}
		if out := a.push(frame); len(out) != 1 {
			t.Fatalf("the reassembler did not resynchronise after the cap: got %X", out)
		}
	})
}

// ---------------------------------------------------------------------------
// Transcripts
// ---------------------------------------------------------------------------

func TestReceivedAndSentAreDefensiveCopies(t *testing.T) {
	r := newRadio(t)
	exchange(t, r, request(0xB6, 0x19, 0x00))

	got := r.Received()
	if len(got) != 1 {
		t.Fatalf("Received() has %d frames, want 1", len(got))
	}
	got[0][2] = 0x00
	if again := r.Received(); again[0][2] != 0xB6 {
		t.Error("mutating the result of Received() reached the radio")
	}

	wantSentCount(t, r, 1, "the answer")
	sent := r.Sent()
	sent[0][3] = 0x00
	if again := r.Sent(); again[0][3] != 0xB6 {
		t.Error("mutating the result of Sent() reached the radio")
	}
}

func TestWithEchoPutsEveryFrameSeenBackOnTheWire(t *testing.T) {
	r := newRadio(t, WithEcho(true))
	req := request(0xB6, 0x19, 0x00)
	if _, err := r.Port().Write(req); err != nil {
		t.Fatalf("writing: %v", err)
	}
	wantFrame(t, readFrame(t, r.Port()), req, "the echo comes first")
	wantFrame(t, readFrame(t, r.Port()),
		[]byte{0xFE, 0xFE, 0xE0, 0xB6, 0x19, 0x00, 0xB6, 0xFD}, "then the answer")

	// A frame addressed elsewhere is echoed too — the bus does not read
	// addresses — and then not answered.
	other := request(0x94, 0x19, 0x00)
	if _, err := r.Port().Write(other); err != nil {
		t.Fatalf("writing: %v", err)
	}
	wantFrame(t, readFrame(t, r.Port()), other, "the echo of a frame addressed elsewhere")

	wantSentCount(t, r, 3, "Sent() is the wire, echoes included")
}

func TestChannelsIsACopy(t *testing.T) {
	rec := validTestRecord()
	r := newRadio(t, WithChannel("001", rec), WithChannel("P2", rec))

	all := r.Channels()
	if len(all) != 2 {
		t.Fatalf("Channels() has %d entries, want 2", len(all))
	}
	all["001"][0] = 0xFF
	delete(all, "P2")

	again := r.Channels()
	if len(again) != 2 {
		t.Errorf("deleting from the result of Channels() reached the radio")
	}
	if again["001"][0] == 0xFF {
		t.Error("mutating a record from Channels() reached the radio")
	}

	one, ok := r.Channel("001")
	if !ok {
		t.Fatal("Channel(\"001\") is empty")
	}
	one[0] = 0xFF
	if again, _ := r.Channel("001"); again[0] == 0xFF {
		t.Error("mutating the result of Channel() reached the radio")
	}
	if _, ok := r.Channel("003"); ok {
		t.Error("Channel(\"003\") reports something stored")
	}
	if _, ok := r.Channel("nonsense"); ok {
		t.Error("Channel(\"nonsense\") reports something stored")
	}
}

// ---------------------------------------------------------------------------
// Seeding options
// ---------------------------------------------------------------------------

func TestWithChannelPanicsOnABadArgument(t *testing.T) {
	for _, tt := range []struct {
		name string
		run  func()
	}{
		{"an address outside the three forms", func() { New(WithChannel("100", validTestRecord())) }},
		{"a scan edge that does not exist", func() { New(WithChannel("P3", validTestRecord())) }},
		{"a record of the wrong length", func() { New(WithChannel("001", make([]byte, 44))) }},
		{"WithRawChannel with a bad address", func() { New(WithRawChannel("000", nil)) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("did not panic")
				}
			}()
			tt.run()
		})
	}
}

// TestWithChannelDoesNotCheckValues is doc.go's ASSUMED register entry 18: a
// seeded record is stored verbatim, so a read answers a value the manual never
// prints — the seam for driving a reader's error paths.
func TestWithChannelDoesNotCheckValues(t *testing.T) {
	rec := validTestRecord()
	rec[offMode] = 0x06 // a mode code page 16 does not print
	r := newRadio(t, WithChannel("001", rec))

	want := append([]byte{0xFE, 0xFE, 0xE0, 0xB6, 0x1A, 0x00, 0x00, 0x01}, rec...)
	want = append(want, 0xFD)
	wantFrame(t, exchange(t, r, memRead(0xB6, 0x00, 0x01)), want, "a seeded record with an unprinted value")

	// The wire is still checked, though.
	wantFrame(t, exchange(t, r, memSet(0xB6, 0x00, 0x02, rec)),
		[]byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFA, 0xFD}, "the same record arriving over the wire")
}

// TestWithRawChannelAnswersAForeignLength is why WithRawChannel exists.
func TestWithRawChannelAnswersAForeignLength(t *testing.T) {
	short := []byte{0x01, 0x02, 0x03}
	long := make([]byte, recordLen+7)
	for i := range long {
		long[i] = byte(i)
	}
	r := newRadio(t, WithRawChannel("001", short), WithRawChannel("P1", long))

	for _, tt := range []struct {
		b0, b1 byte
		rec    []byte
	}{{0x00, 0x01, short}, {0x01, 0x00, long}} {
		want := append([]byte{0xFE, 0xFE, 0xE0, 0xB6, 0x1A, 0x00, tt.b0, tt.b1}, tt.rec...)
		want = append(want, 0xFD)
		wantFrame(t, exchange(t, r, memRead(0xB6, tt.b0, tt.b1)), want, "a foreign-length record answered as stored")
	}
}

// ---------------------------------------------------------------------------
// Latency, floods, shutdown
// ---------------------------------------------------------------------------

func TestWithLatencyDelaysTheAnswer(t *testing.T) {
	r := newRadio(t, WithLatency(120*time.Millisecond))
	start := time.Now()
	exchange(t, r, request(0xB6, 0x19, 0x00))
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("the answer arrived after %v, want at least the scripted 120ms (allowing for timer slack)", elapsed)
	}
}

// TestCloseInterruptsAPendingLatencyWait is why the wait is a select and not a
// time.Sleep: a test may script seconds of latency without paying for them at
// teardown.
func TestCloseInterruptsAPendingLatencyWait(t *testing.T) {
	r := New(WithLatency(30 * time.Second))
	if _, err := r.Port().Write(request(0xB6, 0x19, 0x00)); err != nil {
		t.Fatalf("writing: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		done <- r.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-time.After(testTimeout):
		t.Fatal("Close did not return — it waited out the scripted latency")
	}
}

func TestCloseIsIdempotentAndGivesTheHostEOF(t *testing.T) {
	r := New()
	if err := r.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := r.Port().Read(buf); err != io.EOF {
		t.Errorf("reading the host end after Close = %v, want io.EOF", err)
	}
}

// TestTransceiveBroadcastsDoNotWedgeTheRadio floods the wire with frames
// addressed to 00 and requires a real command to still be answered amongst
// them.
func TestTransceiveBroadcastsDoNotWedgeTheRadio(t *testing.T) {
	r := newRadio(t, WithTransceiveBroadcasts(time.Millisecond))
	assertAnsweredAmongstTheNoise(t, r, 0x00)
}

// TestAddressedFloodDoesNotWedgeTheRadio does the same with frames addressed
// to the CONTROLLER, the only traffic a reading engine cannot dismiss on the
// address byte alone.
func TestAddressedFloodDoesNotWedgeTheRadio(t *testing.T) {
	r := newRadio(t, WithAddressedFlood(time.Millisecond))
	assertAnsweredAmongstTheNoise(t, r, 0xE0)
}

// assertAnsweredAmongstTheNoise reads frames until both the answer to a real
// command and at least one unsolicited frame addressed to wantTo have turned
// up. Their ORDER is not asserted: which lands first is a race between the
// exchange and the flood's period, and pinning it would be pinning the race.
func assertAnsweredAmongstTheNoise(t *testing.T, r *Radio, wantTo byte) {
	t.Helper()

	if _, err := r.Port().Write(request(0xB6, 0x19, 0x00)); err != nil {
		t.Fatalf("writing: %v", err)
	}
	answer := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0x19, 0x00, 0xB6, 0xFD}
	unsolicited := []byte{0xFE, 0xFE, wantTo, 0xB6, 0x00, 0x00, 0x00, 0x25, 0x14, 0x00, 0xFD}

	sawAnswer, sawNoise := false, false
	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		frame := readFrame(t, r.Port())
		switch {
		case bytes.Equal(frame, unsolicited):
			sawNoise = true
		case bytes.Equal(frame, answer):
			sawAnswer = true
		default:
			t.Fatalf("unexpected frame %X", frame)
		}
		if sawAnswer && sawNoise {
			return
		}
	}
	t.Fatalf("timed out: saw the answer = %v, saw unsolicited traffic = %v", sawAnswer, sawNoise)
}

// TestFloodFramesReachTheTranscript checks that Sent() records the unsolicited
// traffic too (doc.go ASSUMED 17).
func TestFloodFramesReachTheTranscript(t *testing.T) {
	r := newRadio(t, WithAddressedFlood(time.Millisecond))
	unsolicited := []byte{0xFE, 0xFE, 0xE0, 0xB6, 0x00, 0x00, 0x00, 0x25, 0x14, 0x00, 0xFD}
	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		if bytes.Equal(readFrame(t, r.Port()), unsolicited) {
			break
		}
	}
	eventually(t, "an unsolicited frame to appear in Sent()", func() bool {
		for _, f := range r.Sent() {
			if bytes.Equal(f, unsolicited) {
				return true
			}
		}
		return false
	})
}

// TestFloodDoesNotBlockACloseThatNobodyIsReading is the reason unsolicited
// writes carry a deadline: a caller that walks away must not leave a goroutine
// parked on the write lock for ever.
func TestFloodDoesNotBlockACloseThatNobodyIsReading(t *testing.T) {
	r := New(WithAddressedFlood(time.Millisecond), WithTransceiveBroadcasts(time.Millisecond))
	time.Sleep(30 * time.Millisecond) // let both goroutines pile up against a reader that never came

	done := make(chan error, 1)
	go func() { done <- r.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Close: %v", err)
		}
	case <-time.After(testTimeout):
		t.Fatal("Close did not return with an unread flood in flight")
	}
}

func TestZeroPeriodDisablesTheUnsolicitedOptions(t *testing.T) {
	r := newRadio(t, WithAddressedFlood(0), WithTransceiveBroadcasts(-time.Second))
	wantFrame(t, exchange(t, r, request(0xB6, 0x19, 0x00)),
		[]byte{0xFE, 0xFE, 0xE0, 0xB6, 0x19, 0x00, 0xB6, 0xFD}, "the only frame on the wire")
	wantSentCount(t, r, 1, "a zero period must emit nothing")
}

// TestConcurrentAccessorsAreSafe is here for -race: the accessors are called
// from other goroutines while the servicing goroutine is writing state.
func TestConcurrentAccessorsAreSafe(t *testing.T) {
	r := newRadio(t)
	rec := validTestRecord()

	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = r.Received()
				_ = r.Sent()
				_ = r.Channels()
				_, _ = r.Channel("001")
			}
		}()
	}

	for n := 1; n <= 20; n++ {
		wantFrame(t, exchange(t, r, memSet(0xB6, 0x00, byte(n/10)<<4|byte(n%10), rec)),
			[]byte{0xFE, 0xFE, 0xE0, 0xB6, 0xFB, 0xFD}, "set under concurrent readers")
	}
	close(stop)
	wg.Wait()

	if n := len(r.Channels()); n != 20 {
		t.Errorf("%d channels stored, want 20", n)
	}
}

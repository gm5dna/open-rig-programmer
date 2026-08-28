// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic705

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"
	"time"
)

// Everything in this file goes over the WIRE — bytes into Port(), bytes back
// out — and every expectation is written by hand from the frame grammar. No
// test asks this package's own builders what a reply should look like.

const testTimeout = 2 * time.Second

// wire is a test's end of the port: it writes frames in and reassembles frames
// out, with a deadline, so that a test that hangs fails instead.
type wire struct {
	t    *testing.T
	port io.ReadWriteCloser
	conn interface {
		SetReadDeadline(time.Time) error
	}
	acc *reassembler
	// pending holds frames already reassembled but not yet returned.
	pending []accEvent
}

func dial(t *testing.T, r *Radio) *wire {
	t.Helper()
	port := r.Port()
	conn, ok := port.(interface{ SetReadDeadline(time.Time) error })
	if !ok {
		t.Fatalf("Port() returned %T, which has no read deadline; these tests would hang on a fault", port)
	}
	return &wire{t: t, port: port, conn: conn, acc: newReassembler(maxAccumulatorBytes)}
}

func (w *wire) send(frame ...byte) {
	w.t.Helper()
	if _, err := w.port.Write(frame); err != nil {
		w.t.Fatalf("writing % X to the port: %v", frame, err)
	}
}

// next returns the next whole frame the radio sent, or fails the test if none
// arrives within testTimeout.
func (w *wire) next() []byte {
	w.t.Helper()
	for {
		if len(w.pending) > 0 {
			ev := w.pending[0]
			w.pending = w.pending[1:]
			if ev.overflow {
				w.t.Fatalf("the radio sent more than %d bytes without a terminator", maxAccumulatorBytes)
			}
			return ev.frame
		}
		if err := w.conn.SetReadDeadline(time.Now().Add(testTimeout)); err != nil {
			w.t.Fatalf("SetReadDeadline: %v", err)
		}
		buf := make([]byte, 4096)
		n, err := w.port.Read(buf)
		if n > 0 {
			w.pending = append(w.pending, w.acc.push(buf[:n])...)
		}
		if err != nil && len(w.pending) == 0 {
			w.t.Fatalf("reading a reply: %v", err)
		}
	}
}

// silent asserts that nothing at all comes back within d.
func (w *wire) silent(d time.Duration) {
	w.t.Helper()
	if err := w.conn.SetReadDeadline(time.Now().Add(d)); err != nil {
		w.t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := make([]byte, 4096)
	n, err := w.port.Read(buf)
	if n > 0 {
		w.t.Fatalf("expected silence, got % X", buf[:n])
	}
	if err == nil {
		w.t.Fatal("expected silence and a read deadline, got a clean zero-byte read")
	}
	var nerr interface{ Timeout() bool }
	if !errors.As(err, &nerr) || !nerr.Timeout() {
		w.t.Fatalf("expected a read timeout, got %v", err)
	}
}

// readFrame is the request a controller sends to read a memory slot, written
// out by hand: preamble, to the radio, from the controller, 1A 00, four address
// bytes, terminator.
func readFrame(g0, g1, c0, c1 byte) []byte {
	return []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x00, g0, g1, c0, c1, 0xFD}
}

// setFrame is the same with a record appended.
func setFrame(g0, g1, c0, c1 byte, record []byte) []byte {
	f := []byte{0xFE, 0xFE, 0xA4, 0xE0, 0x1A, 0x00, g0, g1, c0, c1}
	f = append(f, record...)
	return append(f, 0xFD)
}

func TestReadTransceiverID(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	w.send(0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD)
	got := w.next()

	// Written out: preamble, to the controller, from the radio, 19 00, the
	// payload, terminator. The payload's VALUE is this fake's own invention
	// (register entry 7) and only its SHAPE is asserted here — a non-empty data
	// area that carries no terminator byte, since one would truncate the frame.
	if len(got) < 8 {
		t.Fatalf("ID answer is % X, too short to carry a payload at all", got)
	}
	head := []byte{0xFE, 0xFE, 0xE0, 0xA4, 0x19, 0x00}
	if !bytes.Equal(got[:6], head) {
		t.Errorf("ID answer opens % X, want % X", got[:6], head)
	}
	if got[len(got)-1] != 0xFD {
		t.Errorf("ID answer does not end in FD: % X", got)
	}
	payload := got[6 : len(got)-1]
	if len(payload) == 0 {
		t.Error("ID answer carries no payload")
	}
	if bytes.IndexByte(payload, 0xFD) >= 0 {
		t.Errorf("ID payload % X contains a terminator, which would truncate the frame", payload)
	}
	if bytes.IndexByte(payload, 0xFE) >= 0 {
		t.Errorf("ID payload % X contains FE, which a reassembler could read as a preamble", payload)
	}
}

func TestReadTransceiverID_ToleratesPreamblePadding(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	w.send(0xFE, 0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD)
	got := w.next()
	if !bytes.Equal(got[:6], []byte{0xFE, 0xFE, 0xE0, 0xA4, 0x19, 0x00}) {
		t.Errorf("a padded request drew % X, want an ID answer", got)
	}
}

func TestReadMemory_OccupiedSlotAnswersTheRecord(t *testing.T) {
	record := make([]byte, RecordLen)
	for i := range record {
		record[i] = byte(i % 10) // decimal-safe filler; asserts nothing about fields
	}
	r := New(WithRecord(3, 42, record))
	defer r.Close()
	w := dial(t, r)

	w.send(readFrame(0x00, 0x03, 0x00, 0x42)...)
	got := w.next()

	want := append([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x00, 0x03, 0x00, 0x42}, record...)
	want = append(want, 0xFD)
	if !bytes.Equal(got, want) {
		t.Errorf("read answer = % X\nwant                = % X", got, want)
	}
}

func TestReadMemory_UnwrittenSlotAnswersNG(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	w.send(readFrame(0x00, 0x03, 0x00, 0x42)...)
	if got := w.next(); !bytes.Equal(got, nakBytes) {
		t.Errorf("a read of an unwritten slot drew % X, want % X", got, nakBytes)
	}
}

func TestReadMemory_TheCallChannelGroupIsReadable(t *testing.T) {
	record := BlankRecord()
	r := New(WithRecord(100, 3, record))
	defer r.Close()
	w := dial(t, r)

	w.send(readFrame(0x01, 0x00, 0x00, 0x03)...)
	got := w.next()

	want := append([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x01, 0x00, 0x00, 0x03}, record...)
	want = append(want, 0xFD)
	if !bytes.Equal(got, want) {
		t.Errorf("read of the call channel group = % X\nwant                             = % X", got, want)
	}
}

func TestReadMemory_OutOfRangeAddressesAnswerNG(t *testing.T) {
	// Every slot the ranges allow is occupied, so a NG here is about the
	// ADDRESS and cannot be about the state behind it.
	img := EmptyImage()
	for g := 0; g <= 99; g++ {
		for c := 0; c <= 99; c++ {
			img.With(g, c, BlankRecord())
		}
	}
	for c := 0; c <= 3; c++ {
		img.With(100, c, BlankRecord())
	}
	// And the out-of-range slots the requests below name are occupied too.
	img.With(101, 0, BlankRecord())
	img.With(100, 4, BlankRecord())

	r := New(WithFactoryImage(img))
	defer r.Close()
	w := dial(t, r)

	tests := []struct {
		name string
		addr [4]byte
	}{
		{"group 0101 is past the last memory group", [4]byte{0x01, 0x01, 0x00, 0x00}},
		{"the call channel group has no channel 0004", [4]byte{0x01, 0x00, 0x00, 0x04}},
		{"a memory group has no channel 0100", [4]byte{0x00, 0x00, 0x01, 0x00}},
		{"group 9999", [4]byte{0x99, 0x99, 0x00, 0x00}},
		{"a nibble that is not a decimal digit", [4]byte{0x00, 0x0A, 0x00, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w.send(readFrame(tt.addr[0], tt.addr[1], tt.addr[2], tt.addr[3])...)
			if got := w.next(); !bytes.Equal(got, nakBytes) {
				t.Errorf("drew % X, want % X", got, nakBytes)
			}
		})
	}
}

func TestSetMemory_AcceptsARecordOfTheStatedLength(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	record := make([]byte, RecordLen)
	for i := range record {
		record[i] = byte(i % 10)
	}
	w.send(setFrame(0x00, 0x07, 0x00, 0x11, record)...)

	if got := w.next(); !bytes.Equal(got, ackBytes) {
		t.Fatalf("an accepted set drew % X, want the OK frame % X", got, ackBytes)
	}
	stored, occupied := r.SlotState(7, 11)
	if !occupied {
		t.Fatal("after an accepted set the slot is not occupied")
	}
	if !bytes.Equal(stored, record) {
		t.Errorf("stored record = % X\nwant                = % X", stored, record)
	}
	if r.SetsSeen() != 1 {
		t.Errorf("SetsSeen() = %d, want 1", r.SetsSeen())
	}
}

// TestSetMemory_TheRecordLengthRuleIsAbsolute is the rule the manual states,
// pinned from both sides: nothing but RecordLen is taken, and a refusal changes
// no state at all — not truncated, not padded, not partially written.
func TestSetMemory_TheRecordLengthRuleIsAbsolute(t *testing.T) {
	lengths := []int{1, 38, 39, 40, RecordLen - 1, RecordLen + 1, 115, 200}
	for _, n := range lengths {
		t.Run(itoa(n)+" bytes", func(t *testing.T) {
			r := New()
			defer r.Close()
			w := dial(t, r)

			record := bytes.Repeat([]byte{0x01}, n)
			w.send(setFrame(0x00, 0x07, 0x00, 0x11, record)...)

			if got := w.next(); !bytes.Equal(got, nakBytes) {
				t.Fatalf("a %d-byte record drew % X, want the NG frame % X", n, got, nakBytes)
			}
			if _, occupied := r.SlotState(7, 11); occupied {
				t.Error("a refused set wrote to the slot anyway")
			}
			if r.SetsSeen() != 1 {
				t.Errorf("SetsSeen() = %d, want 1 — a refused attempt is still an attempt", r.SetsSeen())
			}
		})
	}
}

func TestSetMemory_ARefusedSetDoesNotDisturbTheSlotItNamed(t *testing.T) {
	original := bytes.Repeat([]byte{0x09}, RecordLen)
	r := New(WithRecord(7, 11, original))
	defer r.Close()
	w := dial(t, r)

	w.send(setFrame(0x00, 0x07, 0x00, 0x11, bytes.Repeat([]byte{0x01}, 39))...)
	if got := w.next(); !bytes.Equal(got, nakBytes) {
		t.Fatalf("drew % X, want % X", got, nakBytes)
	}
	stored, occupied := r.SlotState(7, 11)
	if !occupied || !bytes.Equal(stored, original) {
		t.Errorf("after a refused set the slot holds (% X, occupied=%v), want the original record untouched", stored, occupied)
	}
}

func TestSetMemory_OutOfRangeAddressIsRefusedAndCounted(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	w.send(setFrame(0x01, 0x00, 0x00, 0x04, BlankRecord())...)
	if got := w.next(); !bytes.Equal(got, nakBytes) {
		t.Fatalf("drew % X, want % X", got, nakBytes)
	}
	if _, occupied := r.SlotState(100, 4); occupied {
		t.Error("a set to an out-of-range slot stored something")
	}
	if r.SetsSeen() != 1 {
		t.Errorf("SetsSeen() = %d, want 1", r.SetsSeen())
	}
}

func TestSetMemory_CreatesAnAbsentSlot(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	if _, occupied := r.SlotState(0, 0); occupied {
		t.Fatal("a fresh radio already holds group 0 channel 0")
	}
	w.send(setFrame(0x00, 0x00, 0x00, 0x00, BlankRecord())...)
	if got := w.next(); !bytes.Equal(got, ackBytes) {
		t.Fatalf("drew % X, want % X", got, ackBytes)
	}
	if _, occupied := r.SlotState(0, 0); !occupied {
		t.Error("a set to an absent slot did not create it")
	}
}

func TestSetThenRead_RoundTripsByteForByte(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	record := make([]byte, RecordLen)
	for i := range record {
		record[i] = byte(i)
		if record[i] == 0xFD || record[i] == 0xFE {
			record[i] = 0x00 // those two would truncate or re-preamble the frame
		}
	}
	w.send(setFrame(0x00, 0x00, 0x00, 0x05, record)...)
	if got := w.next(); !bytes.Equal(got, ackBytes) {
		t.Fatalf("set drew % X, want % X", got, ackBytes)
	}
	w.send(readFrame(0x00, 0x00, 0x00, 0x05)...)
	got := w.next()

	want := append([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x05}, record...)
	want = append(want, 0xFD)
	if !bytes.Equal(got, want) {
		t.Errorf("round trip = % X\nwant            = % X", got, want)
	}
}

// TestInjectedRecordsAreServedVerbatimAtAnyLength is the ability the
// wrong-sibling end-to-end test rests on: the length rule governs the WIRE, and
// an operator seeding state stands outside it.
func TestInjectedRecordsAreServedVerbatimAtAnyLength(t *testing.T) {
	for _, n := range []int{0, 1, 39, 110, 112, 300} {
		t.Run(itoa(n)+" bytes", func(t *testing.T) {
			record := bytes.Repeat([]byte{0x07}, n)
			r := New(WithRecord(0, 1, record))
			defer r.Close()
			w := dial(t, r)

			stored, occupied := r.SlotState(0, 1)
			if !occupied {
				t.Fatal("the seeded slot is not occupied")
			}
			if len(stored) != n {
				t.Errorf("SlotState returned %d bytes, want %d", len(stored), n)
			}

			w.send(readFrame(0x00, 0x00, 0x00, 0x01)...)
			got := w.next()
			want := append([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x01}, record...)
			want = append(want, 0xFD)
			if !bytes.Equal(got, want) {
				t.Errorf("read answer = % X\nwant             = % X", got, want)
			}
		})
	}
}

func TestSlotState_ReturnsACopy(t *testing.T) {
	r := New(WithRecord(0, 1, BlankRecord()))
	defer r.Close()

	first, _ := r.SlotState(0, 1)
	first[0] = 0xFF
	second, _ := r.SlotState(0, 1)
	if second[0] != 0x00 {
		t.Error("mutating a SlotState result reached the radio's own state")
	}
}

func TestSlotState_DistinguishesAbsentFromEmpty(t *testing.T) {
	r := New(WithRecord(0, 1, []byte{}))
	defer r.Close()

	if rec, occupied := r.SlotState(0, 1); !occupied || len(rec) != 0 {
		t.Errorf("a slot holding a zero-length record reports (%v, %v), want (empty, true)", rec, occupied)
	}
	if rec, occupied := r.SlotState(0, 2); occupied || rec != nil {
		t.Errorf("an untouched slot reports (%v, %v), want (nil, false)", rec, occupied)
	}
}

// TestCounters_ARefusalIsVisibleAndSilenceIsNot is the counters' whole purpose:
// telling "nothing was sent" from "something was sent and refused".
func TestCounters_ARefusalIsVisibleAndSilenceIsNot(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	if r.FramesSeen() != 0 || r.SetsSeen() != 0 {
		t.Fatalf("a fresh radio reports FramesSeen=%d SetsSeen=%d, want 0 and 0", r.FramesSeen(), r.SetsSeen())
	}

	// A read: one frame, no set.
	w.send(readFrame(0x00, 0x00, 0x00, 0x00)...)
	w.next()
	if r.FramesSeen() != 1 || r.SetsSeen() != 0 {
		t.Errorf("after one read: FramesSeen=%d SetsSeen=%d, want 1 and 0", r.FramesSeen(), r.SetsSeen())
	}

	// A refused set: still a set attempt.
	w.send(setFrame(0x00, 0x00, 0x00, 0x00, []byte{0x01})...)
	w.next()
	if r.FramesSeen() != 2 || r.SetsSeen() != 1 {
		t.Errorf("after a refused set: FramesSeen=%d SetsSeen=%d, want 2 and 1", r.FramesSeen(), r.SetsSeen())
	}

	// A frame addressed elsewhere is parsed and counted, and answered with
	// nothing.
	w.send(0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD)
	w.silent(150 * time.Millisecond)
	if r.FramesSeen() != 3 || r.SetsSeen() != 1 {
		t.Errorf("after a frame for another station: FramesSeen=%d SetsSeen=%d, want 3 and 1", r.FramesSeen(), r.SetsSeen())
	}
}

// TestAnswerNextReadWithAddress_MisreportsExactlyOneAnswer.
func TestAnswerNextReadWithAddress_MisreportsExactlyOneAnswer(t *testing.T) {
	record := BlankRecord()
	r := New(WithRecord(0, 1, record), WithRecord(0, 2, record))
	defer r.Close()
	w := dial(t, r)

	r.AnswerNextReadWithAddress(0, 99)

	w.send(readFrame(0x00, 0x00, 0x00, 0x01)...)
	got := w.next()
	want := append([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x99}, record...)
	want = append(want, 0xFD)
	if !bytes.Equal(got, want) {
		t.Errorf("the misbehaving answer = % X\nwant                        = % X", got, want)
	}

	// Spent: the next read is honest again.
	w.send(readFrame(0x00, 0x00, 0x00, 0x02)...)
	got = w.next()
	want = append([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x02}, record...)
	want = append(want, 0xFD)
	if !bytes.Equal(got, want) {
		t.Errorf("the following answer = % X\nwant                      = % X", got, want)
	}
}

// TestAnswerNextReadWithAddress_SurvivesARejection: a read that draws NG spends
// nothing, so the hook is still armed for the next read that answers.
func TestAnswerNextReadWithAddress_SurvivesARejection(t *testing.T) {
	record := BlankRecord()
	r := New(WithRecord(0, 1, record))
	defer r.Close()
	w := dial(t, r)

	r.AnswerNextReadWithAddress(100, 3)

	w.send(readFrame(0x00, 0x00, 0x00, 0x55)...) // unwritten: NG
	if got := w.next(); !bytes.Equal(got, nakBytes) {
		t.Fatalf("drew % X, want % X", got, nakBytes)
	}
	w.send(readFrame(0x00, 0x00, 0x00, 0x01)...)
	got := w.next()
	want := append([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00, 0x01, 0x00, 0x00, 0x03}, record...)
	want = append(want, 0xFD)
	if !bytes.Equal(got, want) {
		t.Errorf("the misbehaving answer = % X\nwant                        = % X", got, want)
	}
}

func TestAnswerNextReadWithAddress_CanNameAnAddressTheRadioWouldRefuse(t *testing.T) {
	record := BlankRecord()
	r := New(WithRecord(0, 1, record))
	defer r.Close()
	w := dial(t, r)

	r.AnswerNextReadWithAddress(9999, 9999)
	w.send(readFrame(0x00, 0x00, 0x00, 0x01)...)
	got := w.next()
	if !bytes.Equal(got[6:10], []byte{0x99, 0x99, 0x99, 0x99}) {
		t.Errorf("answer address = % X, want 99 99 99 99", got[6:10])
	}
}

// TestBroadcastsAreAddressedToNobody: the two floods differ in exactly the byte
// a consumer's adapter filters on, which is the whole reason they are two
// options.
func TestBroadcastsAreAddressedToNobody(t *testing.T) {
	r := New(WithBroadcastEvery(time.Millisecond))
	defer r.Close()
	w := dial(t, r)

	for i := 0; i < 3; i++ {
		got := w.next()
		if len(got) < 4 {
			t.Fatalf("broadcast %d is % X, too short to carry addresses", i, got)
		}
		if got[2] != 0x00 {
			t.Errorf("broadcast %d is addressed to %02X, want 00", i, got[2])
		}
		if got[3] != 0xA4 {
			t.Errorf("broadcast %d is from %02X, want A4", i, got[3])
		}
		if got[len(got)-1] != 0xFD {
			t.Errorf("broadcast %d does not end in FD: % X", i, got)
		}
	}
}

func TestNeverQuiet_KeepsTalking(t *testing.T) {
	r := New(WithNeverQuiet())
	defer r.Close()
	w := dial(t, r)

	for i := 0; i < 20; i++ {
		got := w.next()
		if got[2] != 0x00 {
			t.Fatalf("frame %d is addressed to %02X, want the broadcast address 00", i, got[2])
		}
	}
}

func TestAddressedFlood_IsAddressedToTheController(t *testing.T) {
	r := New(WithAddressedFlood())
	defer r.Close()
	w := dial(t, r)

	for i := 0; i < 20; i++ {
		got := w.next()
		if got[2] != 0xE0 {
			t.Fatalf("frame %d is addressed to %02X, want the controller address E0", i, got[2])
		}
		if got[3] != 0xA4 {
			t.Errorf("frame %d is from %02X, want A4", i, got[3])
		}
	}
}

// TestTheTwoFloodsAreDistinct pins the ONE difference that makes them separate
// options: the destination address, which is what a consumer's adapter filters
// on. Everything else about the two frames is the same.
func TestTheTwoFloodsAreDistinct(t *testing.T) {
	rb := New(WithNeverQuiet())
	defer rb.Close()
	wb := dial(t, rb)
	broadcast := wb.next()

	ra := New(WithAddressedFlood())
	defer ra.Close()
	wa := dial(t, ra)
	addressed := wa.next()

	if broadcast[2] == addressed[2] {
		t.Fatalf("both floods address %02X; they must differ, or a test cannot tell an adapter's drop from an engine's drain", broadcast[2])
	}
	if len(broadcast) != len(addressed) {
		t.Errorf("broadcast is %d bytes and the addressed flood %d; only the destination should differ", len(broadcast), len(addressed))
	}
	if !bytes.Equal(broadcast[3:], addressed[3:]) {
		t.Errorf("the two floods differ past the destination byte: % X against % X", broadcast[3:], addressed[3:])
	}
}

// TestAFloodDoesNotStopTheRadioAnswering: a flooded radio still serves reads,
// which is what makes a flood test about the CONSUMER rather than about the
// fake falling over.
func TestAFloodDoesNotStopTheRadioAnswering(t *testing.T) {
	r := New(WithAddressedFlood(), WithRecord(0, 1, BlankRecord()))
	defer r.Close()
	w := dial(t, r)

	w.send(readFrame(0x00, 0x00, 0x00, 0x01)...)

	deadline := time.Now().Add(testTimeout)
	for time.Now().Before(deadline) {
		got := w.next()
		if len(got) > 6 && got[4] == 0x1A && got[5] == 0x00 {
			return // the memory answer got through the flood
		}
	}
	t.Fatal("no memory answer arrived while the radio was flooding")
}

// TestNoUnsolicitedTrafficByDefault: register entry 9. A radio with no flood
// option writes only in reply to a frame that arrived.
func TestNoUnsolicitedTrafficByDefault(t *testing.T) {
	r := New(WithFactoryImage(DefaultImage()))
	defer r.Close()
	w := dial(t, r)

	w.silent(200 * time.Millisecond)
}

func TestWithLatency_DelaysTheReply(t *testing.T) {
	const latency = 120 * time.Millisecond
	r := New(WithLatency(latency))
	defer r.Close()
	w := dial(t, r)

	start := time.Now()
	w.send(0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD)
	w.next()
	if elapsed := time.Since(start); elapsed < latency {
		t.Errorf("the reply took %v, want at least the configured %v", elapsed, latency)
	}
}

// TestCloseIsPromptDuringALatencyWait: Close must not have to wait out a
// scripted delay. This is what Radio.shutdown exists for.
func TestCloseIsPromptDuringALatencyWait(t *testing.T) {
	r := New(WithLatency(30 * time.Second))
	w := dial(t, r)
	w.send(0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD)

	// Let the request reach the servicing goroutine and park in its wait.
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("Close did not return while a latency wait was pending")
	}
}

// TestCloseIsPromptDuringAFlood: the same promptness with an emitter blocked in
// a Write that nobody is reading.
func TestCloseIsPromptDuringAFlood(t *testing.T) {
	r := New(WithNeverQuiet())

	time.Sleep(50 * time.Millisecond) // let the emitter block in its Write

	done := make(chan struct{})
	go func() {
		r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("Close did not return while an emitter was blocked writing")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	r := New()
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestClosedRadioGivesTheHostEOF(t *testing.T) {
	r := New()
	port := r.Port()
	r.Close()

	buf := make([]byte, 8)
	_, err := port.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Errorf("reading a closed radio gave %v, want io.EOF — the signal a host should get from a radio that went away", err)
	}
}

func TestPortIsStable(t *testing.T) {
	r := New()
	defer r.Close()
	if r.Port() != r.Port() {
		t.Error("Port() returned two different connections")
	}
}

// TestAccumulatorOverflowIsAnsweredOverTheWire drives register entry 6 through
// the real port rather than the reassembler alone.
func TestAccumulatorOverflowIsAnsweredOverTheWire(t *testing.T) {
	r := New()
	defer r.Close()
	w := dial(t, r)

	w.send(bytes.Repeat([]byte{0x00}, maxAccumulatorBytes+1)...)
	if got := w.next(); !bytes.Equal(got, nakBytes) {
		t.Fatalf("an overflow drew % X, want the NG frame % X", got, nakBytes)
	}

	// Resync, then a normal exchange.
	w.send(0xFD)
	w.send(0xFE, 0xFE, 0xA4, 0xE0, 0x19, 0x00, 0xFD)
	got := w.next()
	if !bytes.Equal(got[:6], []byte{0xFE, 0xFE, 0xE0, 0xA4, 0x19, 0x00}) {
		t.Errorf("after resync the ID read drew % X", got)
	}
}

// TestConcurrentAccessorsDoNotRace exercises the accessors against the
// servicing goroutine. It is here for -race, which is how the suite is run.
func TestConcurrentAccessorsDoNotRace(t *testing.T) {
	r := New(WithFactoryImage(DefaultImage()))
	defer r.Close()
	w := dial(t, r)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			r.SlotState(0, 0)
			r.FramesSeen()
			r.SetsSeen()
			r.AnswerNextReadWithAddress(0, 5)
		}
	}()

	for i := 0; i < 20; i++ {
		w.send(setFrame(0x00, 0x00, 0x00, 0x00, BlankRecord())...)
		w.next()
		w.send(readFrame(0x00, 0x00, 0x00, 0x00)...)
		w.next()
	}
	close(stop)
	<-done
}

// itoa keeps the subtest names readable without pulling strconv in for one
// call site.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// TestMain keeps a stray goroutine leak from a flood test out of the way of the
// exit code, and gives the suite one place to grow a check if it needs one.
func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

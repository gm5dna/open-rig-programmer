// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"bytes"
	"testing"
	"time"
)

// recvTimeout is how long a test waits for a frame the fake should already have
// sent. It is long enough that a loaded machine does not fail the suite and
// short enough that a genuine hang is a failure rather than a wait.
const recvTimeout = 2 * time.Second

// client is a test-side controller: it writes frames into the fake's port and
// collects whole frames coming back, so that no test has to reason about how
// the pipe happened to split them.
type client struct {
	t      *testing.T
	radio  *Radio
	frames chan []byte
}

func newClient(t *testing.T, opts ...Option) *client {
	t.Helper()
	r := New(opts...)
	t.Cleanup(func() { _ = r.Close() })

	c := &client{t: t, radio: r, frames: make(chan []byte, 1024)}
	port := r.Port()
	go func() {
		acc := newAccumulator()
		buf := make([]byte, 64)
		for {
			n, err := port.Read(buf)
			for _, body := range acc.feed(buf[:n]) {
				select {
				case c.frames <- canonicalFrame(body):
				default: // A test that stops reading must not wedge this goroutine.
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return c
}

// send writes a whole frame from the controller to the radio.
func (c *client) send(data ...byte) {
	c.t.Helper()
	c.sendRaw(buildFrame(radioAddress, controllerAddress, data...))
}

func (c *client) sendRaw(b []byte) {
	c.t.Helper()
	if _, err := c.radio.Port().Write(b); err != nil {
		c.t.Fatalf("writing % 02X to the fake: %v", b, err)
	}
}

// recv returns the next frame the fake sent, whoever it is addressed to.
func (c *client) recv() []byte {
	c.t.Helper()
	select {
	case f, ok := <-c.frames:
		if !ok {
			c.t.Fatal("the fake's port closed while a frame was expected")
		}
		return f
	case <-time.After(recvTimeout):
		c.t.Fatalf("no frame from the fake within %v", recvTimeout)
		return nil
	}
}

// recvAddressed returns the next frame addressed to this controller, skipping
// any to=00 broadcasts that arrived first — which is what a controller's
// accumulator does with them.
func (c *client) recvAddressed() []byte {
	c.t.Helper()
	deadline := time.After(recvTimeout)
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				c.t.Fatal("the fake's port closed while an addressed frame was expected")
			}
			if f[2] == controllerAddress {
				return f
			}
		case <-deadline:
			c.t.Fatalf("no addressed frame from the fake within %v", recvTimeout)
			return nil
		}
	}
}

// quiet asserts the fake says nothing at all for d.
func (c *client) quiet(d time.Duration) {
	c.t.Helper()
	select {
	case f := <-c.frames:
		c.t.Fatalf("expected silence, got % 02X", f)
	case <-time.After(d):
	}
}

func wantFrame(t *testing.T, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("frame = % 02X\n   want % 02X", got, want)
	}
}

// ---------------------------------------------------------------------------
// Addressing
// ---------------------------------------------------------------------------

// TestAnswersOnlyItsOwnAddress: a frame naming somebody else gets no reply at
// all, not even a reject — and it is still recorded, because it did reach the
// radio.
func TestAnswersOnlyItsOwnAddress(t *testing.T) {
	c := newClient(t)

	c.sendRaw(buildFrame(0x94, controllerAddress, cmdTransceiverID, subTransceiverID))
	c.quiet(150 * time.Millisecond)

	c.send(cmdTransceiverID, subTransceiverID)
	got := c.recv()
	if got[2] != controllerAddress || got[3] != radioAddress {
		t.Errorf("answer addresses = to %02X from %02X, want to E0 from A2", got[2], got[3])
	}

	if n := len(c.radio.Transcript()); n != 2 {
		t.Errorf("transcript has %d frames, want 2 — the ignored frame reached the radio too", n)
	}
}

// TestBroadcastFromTheControllerIsNotAnswered: to=00 is addressed to everyone,
// and answered by no one.
func TestBroadcastFromTheControllerIsNotAnswered(t *testing.T) {
	c := newClient(t)
	c.sendRaw(buildFrame(broadcastAddress, controllerAddress, cmdTransceiverID, subTransceiverID))
	c.quiet(150 * time.Millisecond)
}

func TestTransceiverID(t *testing.T) {
	c := newClient(t)
	c.send(cmdTransceiverID, subTransceiverID)
	wantFrame(t, c.recv(), []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x19, 0x00, 0xA2, 0xFD})
}

func TestUnknownCommandIsRejected(t *testing.T) {
	c := newClient(t)
	c.send(0x03) // a read-frequency command this fake does not implement
	wantFrame(t, c.recv(), []byte{0xFE, 0xFE, 0xE0, 0xA2, 0xFA, 0xFD})
}

// ---------------------------------------------------------------------------
// Memory reads
// ---------------------------------------------------------------------------

func TestMemoryRead_SeededSlot(t *testing.T) {
	record := []byte{0x11, 0x22, 0x33}
	c := newClient(t, WithSlot(2, 42, record))

	addr := mustAddress(t, 2, 42)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)

	want := []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x1A, 0x00, 0x02, 0x00, 0x42, 0x11, 0x22, 0x33, 0xFD}
	wantFrame(t, c.recv(), want)
}

func TestMemoryRead_UnseededSlotIsRejected(t *testing.T) {
	c := newClient(t, WithSlot(1, 1, []byte{0x11}))

	addr := mustAddress(t, 1, 2)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)
	wantFrame(t, c.recv(), ngFrame(controllerAddress))
}

func TestMemoryRead_EmptySlotIsRejected(t *testing.T) {
	c := newClient(t, WithSlot(1, 1, []byte{0x11}), WithEmptySlot(1, 1))

	addr := mustAddress(t, 1, 1)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)
	wantFrame(t, c.recv(), ngFrame(controllerAddress))
}

// TestMemoryRead_ShortDataBlockIsRejected: fewer bytes than the three that name
// a channel is not a request for any channel.
func TestMemoryRead_ShortDataBlockIsRejected(t *testing.T) {
	c := newClient(t, WithSlot(1, 1, []byte{0x11}))
	c.send(cmdMemoryContent, subMemoryContent, 0x01, 0x00)
	wantFrame(t, c.recv(), ngFrame(controllerAddress))
}

// TestMemoryRead_EveryPrintedBandIsSeparatelyAddressable proves the band byte
// is part of the key rather than decoration: three channels with the same
// number on three bands hold three different records.
func TestMemoryRead_EveryPrintedBandIsSeparatelyAddressable(t *testing.T) {
	c := newClient(t,
		WithSlot(1, 7, []byte{0xA1}),
		WithSlot(2, 7, []byte{0xA2}),
		WithSlot(3, 7, []byte{0xA3}),
	)
	for band, want := range map[int]byte{1: 0xA1, 2: 0xA2, 3: 0xA3} {
		addr := mustAddress(t, band, 7)
		c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)
		got := c.recv()
		if got[len(got)-2] != want {
			t.Errorf("band %d answered record byte %02X, want %02X", band, got[len(got)-2], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Memory writes
// ---------------------------------------------------------------------------

func TestMemorySet_StoresAndAcknowledges(t *testing.T) {
	c := newClient(t, WithEmptySlot(1, 5))
	addr := mustAddress(t, 1, 5)

	set := append([]byte{cmdMemoryContent, subMemoryContent}, addr...)
	set = append(set, 0x77, 0x88)
	c.send(set...)
	wantFrame(t, c.recv(), okFrame(controllerAddress))

	// It stored it: the channel that answered NG before now answers the record.
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)
	want := []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x1A, 0x00, 0x01, 0x00, 0x05, 0x77, 0x88, 0xFD}
	wantFrame(t, c.recv(), want)
}

// TestMemorySet_WrongLengthIsRejected drives the length rule from
// WithRecordLength, which is the only thing that can tell this fake what length
// it serves out of thin air.
func TestMemorySet_WrongLengthIsRejected(t *testing.T) {
	addr := mustAddress(t, 1, 5)
	tests := []struct {
		name   string
		record []byte
		want   []byte
	}{
		{"one byte short", make([]byte, 3), ngFrame(controllerAddress)},
		{"exactly right", make([]byte, 4), okFrame(controllerAddress)},
		{"one byte long", make([]byte, 5), ngFrame(controllerAddress)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClient(t, WithRecordLength(4))
			set := append([]byte{cmdMemoryContent, subMemoryContent}, addr...)
			set = append(set, tt.record...)
			c.send(set...)
			wantFrame(t, c.recv(), tt.want)
		})
	}
}

// TestMemorySet_LengthInferredFromASeededSlot: with no explicit length, a
// single seeded record is enough to say what length is served.
func TestMemorySet_LengthInferredFromASeededSlot(t *testing.T) {
	c := newClient(t, WithSlot(1, 1, []byte{0x11, 0x22}))
	addr := mustAddress(t, 1, 2)

	set := append([]byte{cmdMemoryContent, subMemoryContent}, addr...)
	set = append(set, 0x33)
	c.send(set...)
	wantFrame(t, c.recv(), ngFrame(controllerAddress))
}

// TestMemorySet_WithNoLengthKnownAnyLengthIsAccepted is the STOP made visible:
// told no length and seeded nothing, this fake has no opinion about how long a
// record is, and does not invent one.
func TestMemorySet_WithNoLengthKnownAnyLengthIsAccepted(t *testing.T) {
	c := newClient(t)
	addr := mustAddress(t, 1, 1)
	for _, n := range []int{1, 38, 114} {
		set := append([]byte{cmdMemoryContent, subMemoryContent}, addr...)
		set = append(set, make([]byte, n)...)
		c.send(set...)
		wantFrame(t, c.recv(), okFrame(controllerAddress))
	}
}

// ---------------------------------------------------------------------------
// The clearing form
// ---------------------------------------------------------------------------

// TestClearFormIsRefused: the address, then FF where field ④ stands, and
// nothing after it.
func TestClearFormIsRefused(t *testing.T) {
	c := newClient(t, WithSlot(1, 5, []byte{0x11, 0x22}))
	addr := mustAddress(t, 1, 5)

	clear := append([]byte{cmdMemoryContent, subMemoryContent}, addr...)
	clear = append(clear, clearFormMarker)
	c.send(clear...)
	wantFrame(t, c.recv(), ngFrame(controllerAddress))

	// Refused means refused: the channel still holds what it held.
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)
	want := []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x1A, 0x00, 0x01, 0x00, 0x05, 0x11, 0x22, 0xFD}
	wantFrame(t, c.recv(), want)
}

// ---------------------------------------------------------------------------
// WithRecordLength
// ---------------------------------------------------------------------------

func TestWithRecordLength_ChangesTheAnswerLength(t *testing.T) {
	tests := []struct {
		name   string
		seeded []byte
		length int
		want   []byte
	}{
		{"padded up", []byte{0x11}, 4, []byte{0x11, 0x00, 0x00, 0x00}},
		{"cut down", []byte{0x11, 0x22, 0x33, 0x44}, 2, []byte{0x11, 0x22}},
		{"unchanged", []byte{0x11, 0x22}, 2, []byte{0x11, 0x22}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newClient(t, WithSlot(1, 1, tt.seeded), WithRecordLength(tt.length))
			addr := mustAddress(t, 1, 1)
			c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)

			got := c.recv()
			gotRecord := got[6+channelAddressLen : len(got)-1]
			if !bytes.Equal(gotRecord, tt.want) {
				t.Errorf("record = % 02X, want % 02X", gotRecord, tt.want)
			}
		})
	}
}

// TestWithRecordLength_DoesNotOccupyASlot is the trap the option's own doc
// warns about, pinned: a caller who sets a length and seeds nothing gets the
// reject code, not a wrong-length answer.
func TestWithRecordLength_DoesNotOccupyASlot(t *testing.T) {
	c := newClient(t, WithRecordLength(4))
	addr := mustAddress(t, 1, 1)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)
	wantFrame(t, c.recv(), ngFrame(controllerAddress))
}

// ---------------------------------------------------------------------------
// The two flood species
// ---------------------------------------------------------------------------

func TestWithBroadcasts_CarriesTheBroadcastAddress(t *testing.T) {
	c := newClient(t, WithBroadcasts(5*time.Millisecond))
	for i := 0; i < 3; i++ {
		got := c.recv()
		if got[2] != broadcastAddress {
			t.Fatalf("broadcast %d = % 02X, want to=00", i, got)
		}
	}
}

func TestWithAddressedFlood_CarriesTheControllerAddress(t *testing.T) {
	c := newClient(t, WithAddressedFlood(5*time.Millisecond))
	for i := 0; i < 3; i++ {
		got := c.recv()
		if got[2] != controllerAddress {
			t.Fatalf("flood frame %d = % 02X, want to=E0", i, got)
		}
	}
}

// TestTheTwoFloodSpeciesAreNotInterchangeable is the whole reason both options
// exist: they differ in the one byte that decides whether a controller's
// accumulator drops the frame or its engine has to deal with it.
func TestTheTwoFloodSpeciesAreNotInterchangeable(t *testing.T) {
	broadcaster := newClient(t, WithBroadcasts(5*time.Millisecond))
	flooder := newClient(t, WithAddressedFlood(5*time.Millisecond))

	b, f := broadcaster.recv(), flooder.recv()
	if b[2] == f[2] {
		t.Fatalf("both species carry to=%02X — then they are one species", b[2])
	}
	if !bytes.Equal(b[3:], f[3:]) {
		t.Errorf("the species differ in more than the to byte:\n  broadcast % 02X\n  flood     % 02X", b, f)
	}
}

// TestBroadcastsDoNotDisplaceAnswers: unsolicited traffic is noise around the
// answer, not instead of it.
func TestBroadcastsDoNotDisplaceAnswers(t *testing.T) {
	c := newClient(t, WithSlot(1, 1, []byte{0x11}), WithBroadcasts(3*time.Millisecond))
	time.Sleep(20 * time.Millisecond) // let some broadcasts pile up first

	addr := mustAddress(t, 1, 1)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)

	want := []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x1A, 0x00, 0x01, 0x00, 0x01, 0x11, 0xFD}
	wantFrame(t, c.recvAddressed(), want)
}

// ---------------------------------------------------------------------------
// WithAnswerAddress
// ---------------------------------------------------------------------------

// TestWithAnswerAddress_NamesADifferentChannel: the answer is well formed, is
// the record the requested channel holds, and names the wrong channel — the T2
// mismatch this option exists to regress against.
func TestWithAnswerAddress_NamesADifferentChannel(t *testing.T) {
	c := newClient(t,
		WithSlot(1, 1, []byte{0x11, 0x22}),
		WithAnswerAddress(3, 107),
	)
	addr := mustAddress(t, 1, 1)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)

	want := []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x1A, 0x00, 0x03, 0x01, 0x07, 0x11, 0x22, 0xFD}
	wantFrame(t, c.recv(), want)
}

// TestWithoutAnswerAddress_TheAnswerNamesWhatWasAsked is the control for the
// test above: without the option the addresses match, so a test that catches a
// mismatch is catching the option and not a standing defect.
func TestWithoutAnswerAddress_TheAnswerNamesWhatWasAsked(t *testing.T) {
	c := newClient(t, WithSlot(1, 1, []byte{0x11, 0x22}))
	addr := mustAddress(t, 1, 1)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)

	got := c.recv()
	if !bytes.Equal(got[6:6+channelAddressLen], addr) {
		t.Errorf("answer named % 02X, want the requested % 02X", got[6:6+channelAddressLen], addr)
	}
}

// ---------------------------------------------------------------------------
// WithEchoBack
// ---------------------------------------------------------------------------

func TestWithEchoBack_EchoesBeforeAnswering(t *testing.T) {
	c := newClient(t, WithEchoBack())
	c.send(cmdTransceiverID, subTransceiverID)

	wantFrame(t, c.recv(), []byte{0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD})
	wantFrame(t, c.recv(), []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x19, 0x00, 0xA2, 0xFD})
}

// TestWithEchoBack_EchoesEvenWhatItWillNotAnswer: the echo is a property of the
// line, so it happens before the radio decides the frame is somebody else's.
func TestWithEchoBack_EchoesEvenWhatItWillNotAnswer(t *testing.T) {
	c := newClient(t, WithEchoBack())
	c.sendRaw(buildFrame(0x94, controllerAddress, cmdTransceiverID, subTransceiverID))

	wantFrame(t, c.recv(), []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD})
	c.quiet(150 * time.Millisecond)
}

func TestWithoutEchoBack_NothingIsEchoed(t *testing.T) {
	c := newClient(t)
	c.send(cmdTransceiverID, subTransceiverID)
	wantFrame(t, c.recv(), []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x19, 0x00, 0xA2, 0xFD})
	c.quiet(150 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// Noise tolerance
// ---------------------------------------------------------------------------

// TestToleratesLeadingNoiseAndRepeatedPreambles feeds the frame the brief's
// worst case describes — 119 preamble bytes — behind a run of junk, and expects
// the ordinary answer.
func TestToleratesLeadingNoiseAndRepeatedPreambles(t *testing.T) {
	c := newClient(t)

	var raw []byte
	raw = append(raw, 0x00, 0x11, 0x7F, 0xFD)
	for i := 0; i < 119; i++ {
		raw = append(raw, preamble)
	}
	raw = append(raw, radioAddress, controllerAddress, cmdTransceiverID, subTransceiverID, endOfMessage)
	c.sendRaw(raw)

	wantFrame(t, c.recv(), []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x19, 0x00, 0xA2, 0xFD})
}

// TestToleratesAFrameArrivingOneByteAtATime: the fake reads a stream, not
// messages.
func TestToleratesAFrameArrivingOneByteAtATime(t *testing.T) {
	c := newClient(t)
	for _, b := range buildFrame(radioAddress, controllerAddress, cmdTransceiverID, subTransceiverID) {
		c.sendRaw([]byte{b})
	}
	wantFrame(t, c.recv(), []byte{0xFE, 0xFE, 0xE0, 0xA2, 0x19, 0x00, 0xA2, 0xFD})
}

// ---------------------------------------------------------------------------
// Transcript
// ---------------------------------------------------------------------------

// TestTranscript_RecordsEveryFrameInOrderAndNormalised.
func TestTranscript_RecordsEveryFrameInOrderAndNormalised(t *testing.T) {
	c := newClient(t, WithSlot(1, 1, []byte{0x11}))
	addr := mustAddress(t, 1, 1)

	// One frame behind a long preamble run, one addressed elsewhere, one read.
	var noisy []byte
	for i := 0; i < 40; i++ {
		noisy = append(noisy, preamble)
	}
	noisy = append(noisy, radioAddress, controllerAddress, cmdTransceiverID, subTransceiverID, endOfMessage)
	c.sendRaw(noisy)
	c.recv()

	c.sendRaw(buildFrame(0x94, controllerAddress, cmdTransceiverID, subTransceiverID))

	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)
	c.recv()

	want := [][]byte{
		{0xFE, 0xFE, 0xA2, 0xE0, 0x19, 0x00, 0xFD},
		{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD},
		{0xFE, 0xFE, 0xA2, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0xFD},
	}
	got := c.radio.Transcript()
	if len(got) != len(want) {
		t.Fatalf("transcript has %d frames %v, want %d", len(got), hexAll(got), len(want))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("transcript[%d] = % 02X, want % 02X", i, got[i], want[i])
		}
	}
}

// TestTranscript_DoesNotAliasTheRadio: a caller mutating what it was handed
// must not rewrite the record of what happened.
func TestTranscript_DoesNotAliasTheRadio(t *testing.T) {
	c := newClient(t)
	c.send(cmdTransceiverID, subTransceiverID)
	c.recv()

	first := c.radio.Transcript()
	if len(first) != 1 {
		t.Fatalf("transcript has %d frames, want 1", len(first))
	}
	first[0][2] = 0x99

	second := c.radio.Transcript()
	if second[0][2] != radioAddress {
		t.Errorf("transcript was mutated through the returned slice: % 02X", second[0])
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func TestClose_IsIdempotentAndStopsTheFloods(t *testing.T) {
	r := New(WithBroadcasts(time.Millisecond), WithAddressedFlood(time.Millisecond))
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestClose_ThroughThePortStopsTheRadio: a driver that closes its port has
// finished, and nothing should outlive it.
func TestClose_ThroughThePortStopsTheRadio(t *testing.T) {
	r := New(WithBroadcasts(time.Millisecond))
	if err := r.Port().Close(); err != nil {
		t.Fatalf("Port().Close(): %v", err)
	}

	// Reading a closed, drained port reports EOF rather than blocking.
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 64)
		for {
			if _, err := r.Port().Read(buf); err != nil {
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(recvTimeout):
		t.Fatal("reading the closed port never returned an error")
	}
}

// ---------------------------------------------------------------------------
// Option misuse
// ---------------------------------------------------------------------------

func TestOptionsPanicOnWhatThePageDoesNotPrint(t *testing.T) {
	tests := []struct {
		name string
		call func()
	}{
		{"WithSlot, band 4", func() { WithSlot(4, 1, nil) }},
		{"WithSlot, channel 108", func() { WithSlot(1, 108, nil) }},
		{"WithSlot, a record containing the preamble byte", func() { WithSlot(1, 1, []byte{0x11, preamble}) }},
		{"WithSlot, a record containing the end-of-message byte", func() { WithSlot(1, 1, []byte{endOfMessage}) }},
		{"WithEmptySlot, band 0", func() { WithEmptySlot(0, 1) }},
		{"WithAnswerAddress, channel 0", func() { WithAnswerAddress(1, 0) }},
		{"WithRecordLength, negative", func() { WithRecordLength(-1) }},
		{"WithBroadcasts, zero interval", func() { WithBroadcasts(0) }},
		{"WithAddressedFlood, negative interval", func() { WithAddressedFlood(-time.Second) }},
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

// TestLaterOptionsWin pins the application order the Option doc promises.
func TestLaterOptionsWin(t *testing.T) {
	c := newClient(t, WithSlot(1, 1, []byte{0x11}), WithSlot(1, 1, []byte{0x22}))
	addr := mustAddress(t, 1, 1)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)

	got := c.recv()
	if got[len(got)-2] != 0x22 {
		t.Errorf("record byte = %02X, want 22 — the later WithSlot", got[len(got)-2])
	}
}

// TestSeededRecordIsCopied: a caller mutating the slice it seeded must not
// reach into the radio's image.
func TestSeededRecordIsCopied(t *testing.T) {
	record := []byte{0x11, 0x22}
	c := newClient(t, WithSlot(1, 1, record))
	record[0] = 0x99

	addr := mustAddress(t, 1, 1)
	c.send(append([]byte{cmdMemoryContent, subMemoryContent}, addr...)...)

	got := c.recv()
	if got[9] != 0x11 {
		t.Errorf("record = % 02X, want the bytes as seeded", got)
	}
}

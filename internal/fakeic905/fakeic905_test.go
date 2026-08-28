// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic905

import (
	"bytes"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

// Every expectation in this file is a hand-written literal, built from the
// frame diagrams quoted in the task brief from the IC-905 CI-V REFERENCE GUIDE,
// PDF p.3 (folio 2), "◇ About the data format":
//
//	Preamble           FE FE
//	End of message     FD
//	Radio address      AC
//	Controller address E0
//	Frame, PC -> radio FE FE AC E0 <cn> [<sc>] [data] FD
//	Frame, radio -> PC FE FE E0 AC <cn> [<sc>] [data] FD
//	OK  (ack)          FE FE E0 AC FB FD
//	NG  (reject)       FE FE E0 AC FA FD
//
// and, for the two-byte group and channel fields, from the value lists in
// core/civ/ic905/testdata/ic905-transcription-b.csv (see parser_test.go's
// TestBCD2_MatchesThePrintedValueLists). Nothing here calls this package's own
// builders to compute what it then asserts.

// ---------------------------------------------------------------------------
// Frame literals, written by hand
// ---------------------------------------------------------------------------

// toRadio wraps payload as a PC -> radio frame: FE FE AC E0 <payload> FD.
func toRadio(payload ...byte) []byte {
	f := []byte{0xFE, 0xFE, 0xAC, 0xE0}
	f = append(f, payload...)
	return append(f, 0xFD)
}

// fromRadio wraps payload as a radio -> PC frame: FE FE E0 AC <payload> FD.
func fromRadio(payload ...byte) []byte {
	f := []byte{0xFE, 0xFE, 0xE0, 0xAC}
	f = append(f, payload...)
	return append(f, 0xFD)
}

// ackFrame and ngFrame are the two fixed codes, spelt out.
var (
	ackFrame = []byte{0xFE, 0xFE, 0xE0, 0xAC, 0xFB, 0xFD}
	ngFrame  = []byte{0xFE, 0xFE, 0xE0, 0xAC, 0xFA, 0xFD}
)

// memRead is a `1A 00` request carrying nothing but the four address bytes.
func memRead(g1, g2, c1, c2 byte) []byte {
	return toRadio(0x1A, 0x00, g1, g2, c1, c2)
}

// memSet is a `1A 00` request carrying the four address bytes and a record.
func memSet(g1, g2, c1, c2 byte, record []byte) []byte {
	p := []byte{0x1A, 0x00, g1, g2, c1, c2}
	p = append(p, record...)
	return toRadio(p...)
}

// pattern builds a deterministic record of n bytes carrying no 0xFE and no
// 0xFD, so a record can never be mistaken for framing.
func pattern(n int, seed byte) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte((int(seed) + i*7) % 0xF0)
	}
	return out
}

// ---------------------------------------------------------------------------
// Port helpers
// ---------------------------------------------------------------------------

func newTestRadio(t *testing.T, opts ...Option) *Radio {
	t.Helper()
	r := New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func writeToPort(t *testing.T, r *Radio, b []byte) {
	t.Helper()
	if _, err := r.Port().Write(b); err != nil {
		t.Fatalf("writing % X to the port: %v", b, err)
	}
}

// tryReadFrame reads bytes from the port until an end-of-message byte arrives
// or d elapses. It is deliberately byte-at-a-time and deliberately naive about
// framing: it must not reuse the fake's own reassembler.
func tryReadFrame(r *Radio, d time.Duration) ([]byte, error) {
	c, ok := r.Port().(net.Conn)
	if !ok {
		return nil, errors.New("Port() is not a net.Conn, so a read deadline cannot be set")
	}
	if err := c.SetReadDeadline(time.Now().Add(d)); err != nil {
		return nil, err
	}
	defer func() { _ = c.SetReadDeadline(time.Time{}) }()

	var out []byte
	one := make([]byte, 1)
	for {
		n, err := c.Read(one)
		if n > 0 {
			out = append(out, one[0])
			if one[0] == 0xFD {
				return out, nil
			}
		}
		if err != nil {
			return out, err
		}
	}
}

// readTimeout is generous on purpose: these tests assert WHAT arrives, not how
// fast, and a loaded machine must not turn a correctness test into a flake.
const readTimeout = 5 * time.Second

func readFrame(t *testing.T, r *Radio) []byte {
	t.Helper()
	f, err := tryReadFrame(r, readTimeout)
	if err != nil {
		t.Fatalf("reading a frame: %v (got % X so far)", err, f)
	}
	return f
}

// silenceWindow is how long "no reply at all" waits before believing itself.
var silenceWindow = 250 * time.Millisecond

func expectSilence(t *testing.T, r *Radio) {
	t.Helper()
	f, err := tryReadFrame(r, silenceWindow)
	if err == nil {
		t.Fatalf("the radio replied % X; it must not reply at all", f)
	}
	if len(f) != 0 {
		t.Fatalf("the radio sent %d bytes (% X) before the window closed; it must send none", len(f), f)
	}
}

func wantFrame(t *testing.T, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Errorf("reply = % X\nwant     % X", got, want)
	}
}

// ---------------------------------------------------------------------------
// Read transceiver ID — cn 19, sc 00
// ---------------------------------------------------------------------------

// TestIdentityRequestIsAnsweredByAnAddressSwappedFrameCarryingTheConfiguredToken
// is the wire fact "Read transceiver ID cn=19 sc=00, request carries NO data
// bytes", plus the two frame diagrams: the request goes out to AC from E0 and
// the answer comes back to E0 from AC.
func TestIdentityRequestIsAnsweredByAnAddressSwappedFrameCarryingTheConfiguredToken(t *testing.T) {
	token := []byte{0x01, 0x23, 0x45}
	r := newTestRadio(t, WithIdentityToken(token))

	writeToPort(t, r, toRadio(0x19, 0x00))
	wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0x01, 0x23, 0x45))
}

// TestADifferentIdentityTokenComesBackUnchanged is the whole point of the
// Option: the fake asserts no fact about what a real IC-905 answers, so a
// consumer may pin any token and prove the driver RECORDS what it got rather
// than matching a value.
func TestADifferentIdentityTokenComesBackUnchanged(t *testing.T) {
	r := newTestRadio(t, WithIdentityToken([]byte{0x77}))
	writeToPort(t, r, toRadio(0x19, 0x00))
	wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0x77))
}

// TestTheDefaultIdentityTokenIsFixedAndArbitrary — a fake with no Option still
// answers, with the constructor's own token, and answers the same thing twice.
func TestTheDefaultIdentityTokenIsFixedAndArbitrary(t *testing.T) {
	r := newTestRadio(t)

	writeToPort(t, r, toRadio(0x19, 0x00))
	first := readFrame(t, r)
	writeToPort(t, r, toRadio(0x19, 0x00))
	second := readFrame(t, r)

	if !bytes.Equal(first, second) {
		t.Fatalf("two identity reads answered differently: % X then % X", first, second)
	}
	if len(first) <= 6 {
		t.Fatalf("the default identity answer % X carries no data bytes; the fake must answer something", first)
	}
	head := []byte{0xFE, 0xFE, 0xE0, 0xAC, 0x19, 0x00}
	if !bytes.HasPrefix(first, head) || first[len(first)-1] != 0xFD {
		t.Errorf("the default identity answer % X is not FE FE E0 AC 19 00 <data> FD", first)
	}
}

// TestAnIdentityRequestCarryingDataIsRefused — the wire facts say the request
// carries NO data bytes, so one that does is malformed.
func TestAnIdentityRequestCarryingDataIsRefused(t *testing.T) {
	r := newTestRadio(t)
	writeToPort(t, r, toRadio(0x19, 0x00, 0x11))
	wantFrame(t, readFrame(t, r), ngFrame)
}

// ---------------------------------------------------------------------------
// Reassembly, at the port
// ---------------------------------------------------------------------------

// TestLeadingNoiseBeforeThePreambleIsTolerated — rubbish on the line before the
// first FE FE must not cost the frame that follows it.
func TestLeadingNoiseBeforeThePreambleIsTolerated(t *testing.T) {
	r := newTestRadio(t, WithIdentityToken([]byte{0x77}))

	noisy := append([]byte{0x00, 0x11, 0x7F, 0x22}, toRadio(0x19, 0x00)...)
	writeToPort(t, r, noisy)
	wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0x77))
}

// TestExtraPreamblePaddingIsTolerated — a controller that pads the preamble is
// still understood.
func TestExtraPreamblePaddingIsTolerated(t *testing.T) {
	r := newTestRadio(t, WithIdentityToken([]byte{0x77}))

	padded := append([]byte{0xFE, 0xFE, 0xFE}, toRadio(0x19, 0x00)...)
	writeToPort(t, r, padded)
	wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0x77))
}

// TestAnOverLengthFrameIsRefusedAsItsOwnEventWithoutWedgingTheReader — the run
// that never ends draws ONE rejection, and the good frame behind it is still
// answered.
func TestAnOverLengthFrameIsRefusedAsItsOwnEventWithoutWedgingTheReader(t *testing.T) {
	r := newTestRadio(t, WithIdentityToken([]byte{0x77}))

	over := []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A}
	for i := 0; i < maxBodyBytes+50; i++ {
		over = append(over, 0x11)
	}
	over = append(over, 0xFD)

	writeToPort(t, r, over)
	wantFrame(t, readFrame(t, r), ngFrame)

	writeToPort(t, r, toRadio(0x19, 0x00))
	wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0x77))
}

// ---------------------------------------------------------------------------
// Memory contents — cn 1A, sc 00
// ---------------------------------------------------------------------------

// TestAMemoryReadReturnsTheSeededRecordAtItsSeededLength is the brief's
// arbitrary-length requirement, stated as a test: the fake holds raw bytes and
// has NO opinion about how many of them there should be. 64 is the length the
// printed diagram measures; 65, 39 and 7 are lengths no IC-905 would send, and
// the fake returns each exactly as seeded.
func TestAMemoryReadReturnsTheSeededRecordAtItsSeededLength(t *testing.T) {
	for _, n := range []int{64, 65, 39, 7} {
		n := n
		t.Run(lengthName(n), func(t *testing.T) {
			rec := pattern(n, 0x30)
			r := newTestRadio(t, WithRecord(0, 7, rec))

			writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x07))

			want := fromRadio(append([]byte{0x1A, 0x00, 0x00, 0x00, 0x00, 0x07}, rec...)...)
			wantFrame(t, readFrame(t, r), want)

			got, ok := r.Record(0, 7)
			if !ok {
				t.Fatal("Record reports the channel unoccupied after WithRecord seeded it")
			}
			if !bytes.Equal(got, rec) {
				t.Errorf("Record = % X, want % X", got, rec)
			}
		})
	}
}

func lengthName(n int) string {
	switch n {
	case 64:
		return "64 bytes, the length the printed diagram measures"
	case 65:
		return "65 bytes, one more than the diagram measures"
	case 39:
		return "39 bytes, a length no IC-905 would send"
	default:
		return "a short record"
	}
}

// TestAMemoryReadOfAnEmptyChannelIsRefused — ASSUMED behaviour, doc.go register
// entry 1, lift ic905-R-14.
func TestAMemoryReadOfAnEmptyChannelIsRefused(t *testing.T) {
	r := newTestRadio(t, WithRecord(0, 7, pattern(64, 0x30)), WithEmpty(0, 8))

	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x08))
	wantFrame(t, readFrame(t, r), ngFrame)

	if rec, ok := r.Record(0, 8); ok {
		t.Errorf("Record reports channel 8 occupied (% X) after WithEmpty", rec)
	}
}

// TestAMemoryReadOfAChannelNoImageEverHeldIsRefused — the same rule, reached
// without WithEmpty.
func TestAMemoryReadOfAChannelNoImageEverHeldIsRefused(t *testing.T) {
	r := newTestRadio(t)
	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x77))
	wantFrame(t, readFrame(t, r), ngFrame)
}

// TestAMemorySetAtTheHeldLengthIsAcknowledgedAndRecordReturnsTheNewBytes — the
// accepted-set path: OK code, and the fake now holds what arrived.
func TestAMemorySetAtTheHeldLengthIsAcknowledgedAndRecordReturnsTheNewBytes(t *testing.T) {
	old := pattern(64, 0x30)
	fresh := pattern(64, 0x81)
	r := newTestRadio(t, WithRecord(0, 7, old))

	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x07, fresh))
	wantFrame(t, readFrame(t, r), ackFrame)

	got, ok := r.Record(0, 7)
	if !ok {
		t.Fatal("Record reports channel 7 unoccupied after an accepted set")
	}
	if !bytes.Equal(got, fresh) {
		t.Errorf("Record = % X, want the bytes just written % X", got, fresh)
	}
}

// TestAMemorySetAtAnyOtherLengthIsRefusedAndRecordIsUnchanged is the
// record-length rejection rule: a set whose record length is not the length the
// fake holds for that channel is answered NG, and nothing is stored.
func TestAMemorySetAtAnyOtherLengthIsRefusedAndRecordIsUnchanged(t *testing.T) {
	held := pattern(64, 0x30)

	for _, n := range []int{63, 65, 39, 1} {
		n := n
		t.Run(lengthName(n), func(t *testing.T) {
			r := newTestRadio(t, WithRecord(0, 7, held))

			writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x07, pattern(n, 0x81)))
			wantFrame(t, readFrame(t, r), ngFrame)

			got, ok := r.Record(0, 7)
			if !ok {
				t.Fatal("Record reports channel 7 unoccupied after a REFUSED set")
			}
			if !bytes.Equal(got, held) {
				t.Errorf("Record = % X after a refused set, want the untouched % X", got, held)
			}
		})
	}
}

// TestAMemorySetToAChannelTheFakeDoesNotHoldSeedsItAtWhateverLengthArrived —
// there is no held length to disagree with, so any length is accepted.
func TestAMemorySetToAChannelTheFakeDoesNotHoldSeedsItAtWhateverLengthArrived(t *testing.T) {
	fresh := pattern(65, 0x81)
	r := newTestRadio(t, WithEmpty(0, 8))

	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x08, fresh))
	wantFrame(t, readFrame(t, r), ackFrame)

	got, ok := r.Record(0, 8)
	if !ok {
		t.Fatal("Record reports channel 8 unoccupied after an accepted set seeded it")
	}
	if !bytes.Equal(got, fresh) {
		t.Errorf("Record = % X, want % X", got, fresh)
	}

	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x08))
	wantFrame(t, readFrame(t, r), fromRadio(append([]byte{0x1A, 0x00, 0x00, 0x00, 0x00, 0x08}, fresh...)...))
}

// TestTheCallChannelGroupIsAddressableToo — group 100 is the printed `01 00`
// (ic905-transcription-b.csv, ①, ②: "01 00: Call channel group"), and the fake
// keys on the two address fields rather than on the memory group alone.
func TestTheCallChannelGroupIsAddressableToo(t *testing.T) {
	rec := pattern(64, 0x55)
	r := newTestRadio(t, WithRecord(100, 0, rec))

	writeToPort(t, r, memRead(0x01, 0x00, 0x00, 0x00))
	wantFrame(t, readFrame(t, r), fromRadio(append([]byte{0x1A, 0x00, 0x01, 0x00, 0x00, 0x00}, rec...)...))

	// The same channel number in the MEMORY group is a different channel.
	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x00))
	if got := readFrame(t, r); bytes.Equal(got, ngFrame) {
		// Either answer is possible depending on the default image; what must
		// NOT happen is the call channel's record coming back.
		return
	} else if bytes.Contains(got, rec) {
		t.Errorf("a read of group 00 00 returned the CALL group's record % X", rec)
	}
}

// TestTheClearFormIsRefused — ic905-transcription-b.csv's D2 block, "To clear
// the memory channel contents on 1A 00:", prints ⑤ as `"FF,"` and ⑥ ~ as
// `None`: the clear form is the four address bytes followed by a single FF.
// This tier does not send it, so this fake refuses it. doc.go register entry 8.
func TestTheClearFormIsRefused(t *testing.T) {
	held := pattern(64, 0x30)
	r := newTestRadio(t, WithRecord(0, 7, held))

	writeToPort(t, r, toRadio(0x1A, 0x00, 0x00, 0x00, 0x00, 0x07, 0xFF))
	wantFrame(t, readFrame(t, r), ngFrame)

	got, ok := r.Record(0, 7)
	if !ok {
		t.Fatal("the refused clear form cleared the channel anyway")
	}
	if !bytes.Equal(got, held) {
		t.Errorf("Record = % X after a refused clear, want the untouched % X", got, held)
	}
}

// TestAMemoryRequestWithNoAddressBytesIsRefused — fewer than the four printed
// address bytes is malformed, not a read of channel zero.
func TestAMemoryRequestWithNoAddressBytesIsRefused(t *testing.T) {
	r := newTestRadio(t)
	for _, short := range [][]byte{
		toRadio(0x1A, 0x00),
		toRadio(0x1A, 0x00, 0x00),
		toRadio(0x1A, 0x00, 0x00, 0x00, 0x00),
	} {
		writeToPort(t, r, short)
		wantFrame(t, readFrame(t, r), ngFrame)
	}
}

// ---------------------------------------------------------------------------
// Everything this tier refuses to send
// ---------------------------------------------------------------------------

// TestTheCommandsThisTierRefusesToSendAreAllRefused — the fake models a radio
// that refuses what this tier refuses to send, so each of these draws NG and
// nothing else.
func TestTheCommandsThisTierRefusesToSendAreAllRefused(t *testing.T) {
	r := newTestRadio(t)

	tests := []struct {
		name    string
		payload []byte
	}{
		{"1A 01 — band stacking register", []byte{0x1A, 0x01, 0x00, 0x01}},
		{"1A 02", []byte{0x1A, 0x02, 0x00}},
		{"1A 05 — set mode", []byte{0x1A, 0x05, 0x01, 0x17}},
		{"09", []byte{0x09}},
		{"0A", []byte{0x0A}},
		{"0B", []byte{0x0B}},
		{"A0", []byte{0xA0}},
		{"1A with an unknown sub-command", []byte{0x1A, 0x7E, 0x00}},
		{"an unknown command", []byte{0x7C, 0x01}},
		{"19 with an unknown sub-command", []byte{0x19, 0x01}},
		{"19 with no sub-command", []byte{0x19}},
		{"1A with no sub-command", []byte{0x1A}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeToPort(t, r, toRadio(tt.payload...))
			wantFrame(t, readFrame(t, r), ngFrame)
		})
	}
}

// ---------------------------------------------------------------------------
// Addressing
// ---------------------------------------------------------------------------

// TestAFrameAddressedElsewhereGetsNoReplyAtAll — NOT an NG. A radio at another
// address simply never sees the frame, and the controller times out; a fake
// that answered would make the driver's timeout branch untestable.
func TestAFrameAddressedElsewhereGetsNoReplyAtAll(t *testing.T) {
	r := newTestRadio(t)

	tests := []struct {
		name  string
		frame []byte
	}{
		{"a well-formed request addressed to some other radio", []byte{0xFE, 0xFE, 0x5C, 0xE0, 0x19, 0x00, 0xFD}},
		{"a broadcast frame, to = 00", []byte{0xFE, 0xFE, 0x00, 0xE0, 0x19, 0x00, 0xFD}},
		{"a frame addressed to the controller itself", []byte{0xFE, 0xFE, 0xE0, 0xAC, 0x19, 0x00, 0xFD}},
		{"a MALFORMED frame addressed elsewhere", []byte{0xFE, 0xFE, 0x5C, 0xFD}},
		{"a frame with no address byte at all", []byte{0xFE, 0xFE, 0xFD}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeToPort(t, r, tt.frame)
			expectSilence(t, r)
		})
	}

	// ... and the radio is still awake afterwards.
	writeToPort(t, r, toRadio(0x19, 0x00))
	if got := readFrame(t, r); len(got) == 0 {
		t.Fatal("the radio stopped answering after ignoring frames addressed elsewhere")
	}
}

// TestAMalformedFrameAddressedToTheRadioIsRefused — the other half of the same
// rule: malformed AND addressed to AC draws NG.
func TestAMalformedFrameAddressedToTheRadioIsRefused(t *testing.T) {
	r := newTestRadio(t)
	writeToPort(t, r, []byte{0xFE, 0xFE, 0xAC, 0xFD})
	wantFrame(t, readFrame(t, r), ngFrame)
}

// ---------------------------------------------------------------------------
// The two floods
// ---------------------------------------------------------------------------

// broadcastFrame and addressedFrame are the two forms, written out. Neither has
// been observed from any IC-905: the fake emits the forms the tier's filter is
// designed for. doc.go register entries 2 and 3.
var (
	broadcastFrame = []byte{0xFE, 0xFE, 0x00, 0xAC, 0x00, 0x11, 0x22, 0x33, 0x44, 0xFD}
	addressedFrame = []byte{0xFE, 0xFE, 0xE0, 0xAC, 0x00, 0x55, 0x66, 0x77, 0x88, 0xFD}
)

// TestTransceiveBroadcastsArriveWithNoRequestOutstanding — a radio that never
// goes quiet. Nothing has been asked of it and frames keep coming.
func TestTransceiveBroadcastsArriveWithNoRequestOutstanding(t *testing.T) {
	r := newTestRadio(t, WithTransceiveBroadcasts(2*time.Millisecond, broadcastFrame))

	for i := 0; i < 3; i++ {
		wantFrame(t, readFrame(t, r), broadcastFrame)
	}
}

// TestTransceiveBroadcastsKeepArrivingWhileARequestIsOutstanding — the flood
// must INTERLEAVE with request handling, not wait for it. The reply is held
// back by WithLatency; broadcasts must arrive during that wait.
func TestTransceiveBroadcastsKeepArrivingWhileARequestIsOutstanding(t *testing.T) {
	r := newTestRadio(t,
		WithTransceiveBroadcasts(2*time.Millisecond, broadcastFrame),
		WithLatency(300*time.Millisecond),
		WithIdentityToken([]byte{0x77}),
	)

	// Drain one broadcast so we know the flood has started.
	wantFrame(t, readFrame(t, r), broadcastFrame)

	writeToPort(t, r, toRadio(0x19, 0x00))

	wantReply := fromRadio(0x19, 0x00, 0x77)
	broadcastsBefore := 0
	for i := 0; i < 500; i++ {
		got := readFrame(t, r)
		if bytes.Equal(got, wantReply) {
			break
		}
		if !bytes.Equal(got, broadcastFrame) {
			t.Fatalf("unexpected frame % X while the request was outstanding", got)
		}
		broadcastsBefore++
	}
	if broadcastsBefore == 0 {
		t.Fatal("no broadcast arrived between the request and its reply — the flood waited for the request instead of interleaving with it")
	}
}

// TestTheAddressedFloodEmitsFramesAddressedToTheController — the other flood,
// the only kind that can reach a controller's engine at all.
func TestTheAddressedFloodEmitsFramesAddressedToTheController(t *testing.T) {
	r := newTestRadio(t, WithAddressedFlood(2*time.Millisecond, addressedFrame))

	for i := 0; i < 3; i++ {
		got := readFrame(t, r)
		wantFrame(t, got, addressedFrame)
		if len(got) > 2 && got[2] != 0xE0 {
			t.Fatalf("flood frame addressed to %#02x, want the controller's E0", got[2])
		}
	}
}

// TestBothFloodsRunAtOnce — they are separate options over separate tickers.
func TestBothFloodsRunAtOnce(t *testing.T) {
	r := newTestRadio(t,
		WithTransceiveBroadcasts(2*time.Millisecond, broadcastFrame),
		WithAddressedFlood(2*time.Millisecond, addressedFrame),
	)

	sawBroadcast, sawAddressed := false, false
	for i := 0; i < 200 && !(sawBroadcast && sawAddressed); i++ {
		switch got := readFrame(t, r); {
		case bytes.Equal(got, broadcastFrame):
			sawBroadcast = true
		case bytes.Equal(got, addressedFrame):
			sawAddressed = true
		default:
			t.Fatalf("unexpected frame % X", got)
		}
	}
	if !sawBroadcast || !sawAddressed {
		t.Errorf("saw broadcast = %v, addressed = %v; want both", sawBroadcast, sawAddressed)
	}
}

// TestAFloodFrameAddressedTheWrongWayPanics — the option's whole promise is
// which address the frames carry, so a frame that contradicts it is a
// programming error in the test that wrote it and must stop loudly.
func TestAFloodFrameAddressedTheWrongWayPanics(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{"a broadcast option given a controller-addressed frame", func() { New(WithTransceiveBroadcasts(time.Millisecond, addressedFrame)) }},
		{"an addressed option given a broadcast frame", func() { New(WithAddressedFlood(time.Millisecond, broadcastFrame)) }},
		{"a frame that is not a frame", func() { New(WithAddressedFlood(time.Millisecond, []byte{0x00, 0x11})) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("returned normally, want a panic")
				}
			}()
			tt.fn()
		})
	}
}

// ---------------------------------------------------------------------------
// Frames(), Record() and Close()
// ---------------------------------------------------------------------------

// TestFramesRecordsEveryFrameItReceivedInOrder — the seam a consumer uses to
// assert that a refused write put nothing on the wire.
func TestFramesRecordsEveryFrameItReceivedInOrder(t *testing.T) {
	held := pattern(64, 0x30)
	r := newTestRadio(t, WithRecord(0, 7, held))

	id := toRadio(0x19, 0x00)
	read := memRead(0x00, 0x00, 0x00, 0x07)
	badSet := memSet(0x00, 0x00, 0x00, 0x07, pattern(39, 0x81))

	writeToPort(t, r, id)
	readFrame(t, r)
	writeToPort(t, r, read)
	readFrame(t, r)
	writeToPort(t, r, badSet)
	wantFrame(t, readFrame(t, r), ngFrame)

	got := r.Frames()
	want := [][]byte{id, read, badSet}
	if len(got) != len(want) {
		t.Fatalf("Frames() returned %d frames, want %d: % X", len(got), len(want), got)
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("Frames()[%d] = % X, want % X", i, got[i], want[i])
		}
	}

	// The refused set is on the wire; its bytes are NOT in the record.
	rec, ok := r.Record(0, 7)
	if !ok || !bytes.Equal(rec, held) {
		t.Errorf("Record = % X (ok=%v) after a refused set, want the untouched % X", rec, ok, held)
	}
}

// TestFramesRecordsAFrameAddressedElsewhere — the fake SAW it, even though it
// said nothing, and a consumer needs to be able to tell those two apart.
func TestFramesRecordsAFrameAddressedElsewhere(t *testing.T) {
	r := newTestRadio(t)
	elsewhere := []byte{0xFE, 0xFE, 0x5C, 0xE0, 0x19, 0x00, 0xFD}

	writeToPort(t, r, elsewhere)
	expectSilence(t, r)

	got := r.Frames()
	if len(got) != 1 || !bytes.Equal(got[0], elsewhere) {
		t.Fatalf("Frames() = % X, want exactly [% X]", got, elsewhere)
	}
}

// TestFramesDoesNotRecordFramesTheFakeSENT — Frames() is what the fake
// RECEIVED. A flood running beside it must not appear there.
func TestFramesDoesNotRecordFramesTheFakeSENT(t *testing.T) {
	r := newTestRadio(t, WithTransceiveBroadcasts(2*time.Millisecond, broadcastFrame))
	for i := 0; i < 5; i++ {
		readFrame(t, r)
	}
	if got := r.Frames(); len(got) != 0 {
		t.Errorf("Frames() = % X after five broadcasts and no request, want none", got)
	}
}

// TestRecordReturnsACopy — a consumer that mutates what it got must not be
// mutating the fake's state.
func TestRecordReturnsACopy(t *testing.T) {
	held := pattern(64, 0x30)
	r := newTestRadio(t, WithRecord(0, 7, held))

	got, ok := r.Record(0, 7)
	if !ok {
		t.Fatal("Record reports the channel unoccupied")
	}
	got[0] ^= 0xFF

	again, _ := r.Record(0, 7)
	if !bytes.Equal(again, held) {
		t.Errorf("Record = % X after the caller mutated an earlier copy, want % X", again, held)
	}
}

// TestWithRecordCopiesItsInput — likewise on the way in.
func TestWithRecordCopiesItsInput(t *testing.T) {
	seed := pattern(64, 0x30)
	original := append([]byte(nil), seed...)
	r := newTestRadio(t, WithRecord(0, 7, seed))

	seed[0] ^= 0xFF

	got, _ := r.Record(0, 7)
	if !bytes.Equal(got, original) {
		t.Errorf("Record = % X after the caller mutated the slice it seeded with, want % X", got, original)
	}
}

// TestCloseIsIdempotentAndPrompt — including with a long latency scripted, so a
// test may script one without paying for it at teardown.
func TestCloseIsIdempotentAndPrompt(t *testing.T) {
	r := New(WithLatency(30 * time.Second))

	writeToPort(t, r, toRadio(0x19, 0x00))

	start := time.Now()
	if err := r.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("Close took %v; it must not wait out a pending latency", d)
	}
}

// TestPortReturnsTheSameConnectionEveryTime.
func TestPortReturnsTheSameConnectionEveryTime(t *testing.T) {
	r := newTestRadio(t)
	if r.Port() != r.Port() {
		t.Error("Port() returned two different connections")
	}
}

// ---------------------------------------------------------------------------
// The record-field naming table
// ---------------------------------------------------------------------------

// TestTheRecordFieldTableIsGaplessAndSumsTo68 checks state.go's transcribed
// field table against the arithmetic BOTH artefacts print for themselves:
//
//   - ic905-transcription-b.md: "2+2+1+5+2+1+1+1+3+3+3+1+3+8+8+8+16 = 68 in
//     total, which equals the highest printed index, 68 ... no gap, overlap or
//     shortfall exists."
//   - ic905-geometry-witness.md: "the index sequence printed in the band runs
//     1…68 with no repeat, no gap, no out-of-order index".
//
// The table is naming only — nothing in this package consults it when handling
// a frame — but a mistyped offset in it would mislead every later reader, so it
// is pinned.
func TestTheRecordFieldTableIsGaplessAndSumsTo68(t *testing.T) {
	if len(d1RecordFields) != 17 {
		t.Fatalf("the table has %d fields, want the 17 rows ic905-transcription-b.csv carries for D1", len(d1RecordFields))
	}

	next := 1
	total := 0
	for _, f := range d1RecordFields {
		if f.First != next {
			t.Errorf("field %q starts at byte %d, want %d — the table has a gap or an overlap", f.Index, f.First, next)
		}
		if f.Last < f.First {
			t.Errorf("field %q runs backwards: %d-%d", f.Index, f.First, f.Last)
		}
		if w := f.Last - f.First + 1; w != f.Width {
			t.Errorf("field %q spans %d bytes (%d-%d) but records a width of %d", f.Index, w, f.First, f.Last, f.Width)
		}
		total += f.Width
		next = f.Last + 1
	}
	if total != 68 {
		t.Errorf("the table sums to %d bytes, want the printed 68", total)
	}
	if next-1 != 68 {
		t.Errorf("the table ends at byte %d, want the highest printed index 68", next-1)
	}
}

// TestTheRecordIsWhatFollowsTheFourAddressBytes — the printed block is 68 bytes
// and its first four are the group and channel fields (indices 1-4), so the
// record this fake stores is bytes 5-68: 64 of them. That is where the DRIVER's
// {64, 65} fingerprint comes from; it is not a rule of this fake, which holds
// any length at all (see TestAMemoryReadReturnsTheSeededRecordAtItsSeededLength).
func TestTheRecordIsWhatFollowsTheFourAddressBytes(t *testing.T) {
	if d1RecordFields[0].Index != "1, 2" || d1RecordFields[1].Index != "3, 4" {
		t.Fatalf("the first two fields are %q and %q, want the printed `1, 2` and `3, 4`", d1RecordFields[0].Index, d1RecordFields[1].Index)
	}
	if addressBytes != 4 {
		t.Errorf("addressBytes = %d, want 4 (indices 1-4: the group and channel fields)", addressBytes)
	}
	if recordFirstByte != 5 || recordLastByte != 68 {
		t.Errorf("the record spans bytes %d-%d, want 5-68", recordFirstByte, recordLastByte)
	}
	if printedRecordLen != 64 {
		t.Errorf("printedRecordLen = %d, want 64", printedRecordLen)
	}
}

// TestTheTwoRepeatedLabelsAreTranscribedAsPrinted — STOP 1 in both artefacts:
// bytes 16-18 and 19-21 carry word-for-word identical printed labels. Neither
// artefact repairs it, and neither does this table.
func TestTheTwoRepeatedLabelsAreTranscribedAsPrinted(t *testing.T) {
	var a, b string
	for _, f := range d1RecordFields {
		switch f.Index {
		case "16~18":
			a = f.Label
		case "19~21":
			b = f.Label
		}
	}
	if a == "" || b == "" {
		t.Fatalf("the table is missing 16~18 (%q) or 19~21 (%q)", a, b)
	}
	if a != b {
		t.Errorf("16~18 is labelled %q and 19~21 %q; both artefacts record them as word-for-word identical (STOP 1), transcribed as printed and NOT repaired", a, b)
	}
	if a != "Repeater tone frequency setting" {
		t.Errorf("the label is %q, want the printed \"Repeater tone frequency setting\"", a)
	}
}

// TestTheClearFormTableIsTheD2Block — the four D2 rows, transcribed.
func TestTheClearFormTableIsTheD2Block(t *testing.T) {
	if len(d2ClearFields) != 4 {
		t.Fatalf("the clear-form table has %d rows, want the 4 D2 rows of ic905-transcription-b.csv", len(d2ClearFields))
	}
	if clearFormLen != 5 {
		t.Errorf("clearFormLen = %d, want 5 — indices 1-4 (group and channel) plus index 5, printed as \"FF,\"", clearFormLen)
	}
	if clearFormByte != 0xFF {
		t.Errorf("clearFormByte = %#02x, want FF", clearFormByte)
	}
}

// ---------------------------------------------------------------------------
// A guard against this file being the only thing that ever ran
// ---------------------------------------------------------------------------

// TestTheProvenanceNoteIsPresent — PROVENANCE.md is part of the deliverable,
// and a package whose provenance note has been deleted is not this package.
func TestTheProvenanceNoteIsPresent(t *testing.T) {
	info, err := os.Stat("PROVENANCE.md")
	if err != nil {
		t.Fatalf("PROVENANCE.md: %v", err)
	}
	if info.Size() == 0 {
		t.Error("PROVENANCE.md is empty")
	}
}

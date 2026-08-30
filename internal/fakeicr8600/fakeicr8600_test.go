// SPDX-License-Identifier: GPL-3.0-or-later

package fakeicr8600

import (
	"bytes"
	"errors"
	"net"
	"testing"
	"time"
)

// Every expectation in this file is a HAND-WRITTEN LITERAL. Nothing here calls
// this package's own builders, tables or constants to compute the bytes it then
// asserts — the whole value of an independently authored fake evaporates the
// moment its tests ask it what it thinks the answer is.
//
// The literals come from two printed sources, by way of the quarantined
// artefacts:
//
//   - the frame diagram on PDF p.3 (folio 2) of the IC-R8600 CI-V REFERENCE
//     GUIDE, rev A7375-2EX-3a, as core/civ/icr8600/testdata/
//     IC-R8600-golden-provenance.md quotes it:
//
//     Preamble            FE FE       ("Preamble code (fixed)")
//     End of message      FD          ("End of message code (fixed)")
//     Receiver address    96          ("Receiver's default address")
//     Controller address  E0          ("Controller's default address")
//     Controller -> radio FE FE 96 E0 <cn> [<sc>] [data] FD
//     Radio -> controller FE FE E0 96 <cn> [<sc>] [data] FD
//     OK  (ack)           FE FE E0 96 FB FD   ("OK code (fixed)")
//     NG  (reject)        FE FE E0 96 FA FD   ("NG code (fixed)")
//
//   - the record geometry measured on PDF pp.12-15 (folios 11-14) and recorded
//     in IC-R8600-geometry-witness.csv/.md and IC-R8600-transcription-b.csv,
//     with the field values from the domains those two print.
//
// The record bodies below were derived here, field by field, from those
// artefacts; they agree byte for byte with the seven record vectors in
// IC-R8600-vectors.golden, which is a cross-check and not the source.

// ---------------------------------------------------------------------------
// Frame literals, written by hand
// ---------------------------------------------------------------------------

// toRadio wraps payload as a controller -> radio frame: FE FE 96 E0 … FD.
func toRadio(payload ...byte) []byte {
	f := []byte{0xFE, 0xFE, 0x96, 0xE0}
	f = append(f, payload...)
	return append(f, 0xFD)
}

// fromRadio wraps payload as a radio -> controller frame: FE FE E0 96 … FD.
func fromRadio(payload ...byte) []byte {
	f := []byte{0xFE, 0xFE, 0xE0, 0x96}
	f = append(f, payload...)
	return append(f, 0xFD)
}

var (
	ackFrame = []byte{0xFE, 0xFE, 0xE0, 0x96, 0xFB, 0xFD}
	ngFrame  = []byte{0xFE, 0xFE, 0xE0, 0x96, 0xFA, 0xFD}
)

// memRead is a `1A 00` request carrying nothing but the four address bytes —
// the read-request form the guide never prints (ASSUMED, doc.go register
// entry 4).
func memRead(g1, g2, c1, c2 byte) []byte {
	return toRadio(0x1A, 0x00, g1, g2, c1, c2)
}

// memSet is a `1A 00` request carrying the four address bytes and a record.
func memSet(g1, g2, c1, c2 byte, record []byte) []byte {
	p := []byte{0x1A, 0x00, g1, g2, c1, c2}
	p = append(p, record...)
	return toRadio(p...)
}

// memAnswer is the answer to a read: `1A 00`, the four address bytes, the
// record.
func memAnswer(g1, g2, c1, c2 byte, record []byte) []byte {
	p := []byte{0x1A, 0x00, g1, g2, c1, c2}
	p = append(p, record...)
	return fromRadio(p...)
}

// ---------------------------------------------------------------------------
// The record head and the six tails, spelt out by hand
// ---------------------------------------------------------------------------

// headFor is the 37-byte record-only head, written out field by field in the
// printed index order of PDF p.12 (folio 11), with mode as printed index 11 and
// filter as printed index 12.
//
// Widths, from IC-R8600-geometry-witness.md's position arithmetic, less the
// four address bytes the record excludes:
//
//	(5)      skip/select                1
//	(6)-(10) receiving frequency        5
//	(11)     receiving mode             1
//	(12)     filter setting             1
//	(13)     duplex                     1
//	(14)-(17) offset frequency          4
//	(18)     tuning step function       1
//	(19)     tuning step setting        1
//	(20),(21) programmable tuning step  2
//	(22)     attenuator                 1
//	(23)     preamplifier               1
//	(24)     antenna                    1
//	(25)     IP+                        1
//	(26)-(41) memory name              16
//	                                   --
//	                                   37
func headFor(mode byte, name string) []byte {
	head := []byte{
		0x00,                         // (5)  skip/select: SKIP OFF, select OFF
		0x00, 0x00, 0x50, 0x45, 0x01, // (6)-(10) 145.500000 MHz
		mode,                   // (11) receiving mode
		0x01,                   // (12) filter setting FIL1
		0x00,                   // (13) duplex: 0 fixed, OFF
		0x00, 0x00, 0x00, 0x00, // (14)-(17) offset frequency, zero
		0x01,       // (18) TS function ON
		0x05,       // (19) tuning step setting
		0x90, 0x00, // (20),(21) programmable tuning step, drawn digit order
		0x00, // (22) attenuator OFF
		0x01, // (23) 0 fixed, preamp ON
		0x00, // (24) 0 fixed, ANT1
		0x00, // (25) 0 fixed, IP+ OFF
	}
	return append(head, padName(name)...)
}

// padName spells a memory name as the 16 bytes of printed indices (26)-(41):
// one ASCII byte per character, padded to 16 with the space character.
//
// BOTH halves are ASSUMED (doc.go register entries 8 and 9): PDF p.11 (folio
// 10) lists the selectable characters and the total character number 16, and
// prints no code for any of them and no pad byte. This helper spells the
// assumption out by hand rather than calling the package's own encoder.
func padName(s string) []byte {
	out := make([]byte, 16)
	for i := range out {
		out[i] = 0x20
	}
	copy(out, s)
	return out
}

// The six drawn tails, each written out from its own diagram. Byte counts:
// FM 7 (D2), P25 4 (D3), D-STAR 2 (D4), dPMR 8 (D5), NXDN 6 (D6), DCR 7 (D7).
var (
	// D2, PDF p.13: (42) tone squelch type = 1 TSQL; (43)-(45) tone squelch
	// frequency 88.5 Hz; (46)-(48) DTCS code 023, polarity Normal.
	tailFM = []byte{0x01, 0x00, 0x08, 0x85, 0x00, 0x00, 0x23}
	// D3, PDF p.13: (42) D.SQL type = 1 NAC; (43)-(45) NAC 293h.
	tailP25 = []byte{0x01, 0x02, 0x09, 0x03}
	// D4, PDF p.13: (42) D.SQL type = 2 CSQL (this block prints no code 1);
	// (43) CSQL code 12.
	tailDSTAR = []byte{0x02, 0x12}
	// D5, PDF p.14: (42) type = 1 COM ID; (43),(44) COM ID 123; (45) CC 12;
	// (46) scrambler OFF; (47)-(49) scrambler key 00000.
	tailDPMR = []byte{0x01, 0x01, 0x23, 0x12, 0x00, 0x00, 0x00, 0x00}
	// D6, PDF p.14: (42) type = 1 RAN; (43) RAN 05; (44) encryption OFF;
	// (45)-(47) encryption key 00000.
	tailNXDN = []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00}
	// D7, PDF p.15: (42) type = 1 UC; (43),(44) UC code 123; (45) encryption
	// OFF; (46)-(48) encryption key 00000.
	tailDCR = []byte{0x01, 0x01, 0x23, 0x00, 0x00, 0x00, 0x00}
)

// recordFor glues a head and a tail into one whole record.
func recordFor(mode byte, name string, tail []byte) []byte {
	return append(headFor(mode, name), tail...)
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
// or d elapses. Deliberately byte-at-a-time and deliberately naive about
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
const silenceWindow = 250 * time.Millisecond

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
// Framing: 96 and E0
// ---------------------------------------------------------------------------

// TestAFrameAddressedToTheReceiverIsAnswered pins the "Controller to IC-R8600"
// direction: to = 96, from = E0, and the answer swaps them.
func TestAFrameAddressedToTheReceiverIsAnswered(t *testing.T) {
	r := newTestRadio(t)
	writeToPort(t, r, toRadio(0x19, 0x00))

	got := readFrame(t, r)
	if len(got) < 4 {
		t.Fatalf("answer % X is too short to carry an address pair", got)
	}
	if got[2] != 0xE0 || got[3] != 0x96 {
		t.Errorf("answer addressed to %#02X from %#02X; the radio -> controller diagram draws to = E0, from = 96", got[2], got[3])
	}
}

// TestAFrameAddressedElsewhereDrawsNoReplyAtAll is the difference that makes a
// driver's timeout branch testable: a radio at another address never hears the
// frame, whereas one that heard a frame it cannot honour says NG.
func TestAFrameAddressedElsewhereDrawsNoReplyAtAll(t *testing.T) {
	r := newTestRadio(t)
	writeToPort(t, r, []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD})
	expectSilence(t, r)
}

// TestTheRadioAddressCanBeMoved is the `icr8600-address-move` entry made
// exercisable: this program ships no --civ-address flag, so a moved receiver is
// unreachable and simply times out. Both halves are asserted here.
func TestTheRadioAddressCanBeMoved(t *testing.T) {
	r := newTestRadio(t, WithRadioAddress(0x7A))

	writeToPort(t, r, toRadio(0x19, 0x00))
	expectSilence(t, r)

	writeToPort(t, r, []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x19, 0x00, 0xFD})
	got := readFrame(t, r)
	if len(got) < 4 || got[2] != 0xE0 || got[3] != 0x7A {
		t.Fatalf("answer % X; a receiver moved to 7A answers from 7A", got)
	}
}

// TestAMalformedFrameAddressedToTheReceiverIsRefused — too short to carry to,
// from and a command byte. It is malformed, not empty, and the receiver heard
// it, so it draws NG rather than silence.
func TestAMalformedFrameAddressedToTheReceiverIsRefused(t *testing.T) {
	r := newTestRadio(t)
	writeToPort(t, r, []byte{0xFE, 0xFE, 0x96, 0xE0, 0xFD})
	wantFrame(t, readFrame(t, r), ngFrame)
}

// TestPreamblePaddingIsTolerated — the one worked example the guide prints
// (PDF p.9, folio 8, "Example: When using 4800 bps") pads the power-on command
// with five extra FE bytes, so a run of more than two opens a frame just the
// same.
func TestPreamblePaddingIsTolerated(t *testing.T) {
	r := newTestRadio(t)
	writeToPort(t, r, []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0x96, 0xE0, 0x19, 0x00, 0xFD})
	wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0xDE, 0xAD))
}

// ---------------------------------------------------------------------------
// Read receiver ID — cn 19, sc 00
// ---------------------------------------------------------------------------

// TestIdentityRequestIsAnsweredWithTheConfiguredToken. The command table on PDF
// p.5 (folio 4) prints "19 / 00 / <blank Data cell> / Read the receiver ID":
// the request carries no data area, and the ANSWER's value is printed nowhere.
func TestIdentityRequestIsAnsweredWithTheConfiguredToken(t *testing.T) {
	r := newTestRadio(t, WithIDToken([]byte{0x01, 0x23, 0x45}))
	writeToPort(t, r, toRadio(0x19, 0x00))
	wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0x01, 0x23, 0x45))
}

// TestTheDefaultIdentityTokenIsArbitraryAndStable — a fake with no Option still
// answers, with the constructor's own conspicuously invented token, twice
// alike.
func TestTheDefaultIdentityTokenIsArbitraryAndStable(t *testing.T) {
	r := newTestRadio(t)
	for i := 0; i < 2; i++ {
		writeToPort(t, r, toRadio(0x19, 0x00))
		wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0xDE, 0xAD))
	}
}

// TestAnIdentityRequestCarryingDataIsRefused — the Data cell is blank, so a
// request with a data area is not the printed request.
func TestAnIdentityRequestCarryingDataIsRefused(t *testing.T) {
	r := newTestRadio(t)
	writeToPort(t, r, toRadio(0x19, 0x00, 0x00))
	wantFrame(t, readFrame(t, r), ngFrame)
}

// ---------------------------------------------------------------------------
// Reading a record — the full response, and every declared tail
// ---------------------------------------------------------------------------

// TestReadingAnOccupiedChannelAnswersCommandAddressAndRecord spells the whole
// 55-byte FM answer out by hand: FE FE E0 96 1A 00, four address bytes, a
// 44-byte record, FD.
func TestReadingAnOccupiedChannelAnswersCommandAddressAndRecord(t *testing.T) {
	rec := recordFor(0x05, "FM", tailFM)
	r := newTestRadio(t, WithRecord(0, 1, rec))

	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x01))
	got := readFrame(t, r)

	wantFrame(t, got, memAnswer(0x00, 0x00, 0x00, 0x01, rec))
	if len(got) != 55 {
		t.Errorf("answer is %d bytes; the FM record is 44 record-only bytes, 48 in the data area, 55 in the frame", len(got))
	}
}

// declaredTails is the seven layouts the document draws, each with the wire
// mode code that selects it and the record-only length it produces. Written out
// by hand from PDF pp.12-15; nothing here reads this package's own mode table.
//
// The two NXDN wire codes are one layout: PDF p.14 heads a single diagram "For
// receiving an NXDN signal" and the mode table prints both 19 NXDN-VN and
// 20 NXDN-N.
var declaredTails = []struct {
	class     string
	mode      byte
	tail      []byte
	recordLen int
}{
	{"NONE (AM)", 0x02, nil, 37},
	{"D-STAR", 0x17, tailDSTAR, 39},
	{"P25", 0x16, tailP25, 41},
	{"NXDN-VN", 0x19, tailNXDN, 43},
	{"NXDN-N", 0x20, tailNXDN, 43},
	{"FM", 0x05, tailFM, 44},
	{"DCR", 0x21, tailDCR, 44},
	{"dPMR", 0x18, tailDPMR, 45},
}

// TestEveryDeclaredTailIsServed reads back one channel per declared mode class
// and fails loudly if any is missing, short, long or altered. This is the test
// the plan asks to fail explicitly when a declared tail is not served.
func TestEveryDeclaredTailIsServed(t *testing.T) {
	for i, tc := range declaredTails {
		t.Run(tc.class, func(t *testing.T) {
			rec := recordFor(tc.mode, tc.class, tc.tail)
			if len(rec) != tc.recordLen {
				t.Fatalf("the hand-written %s record is %d bytes, not the %d the diagram measures — the TEST is wrong", tc.class, len(rec), tc.recordLen)
			}
			ch := byte(i)
			r := newTestRadio(t, WithRecord(0, int(ch), rec))

			writeToPort(t, r, memRead(0x00, 0x00, 0x00, ch))
			wantFrame(t, readFrame(t, r), memAnswer(0x00, 0x00, 0x00, ch, rec))
		})
	}
}

// TestTheDefaultImageServesEveryDeclaredModeClass — a fake built with no
// options at all already holds one channel per layout, so a consumer need seed
// nothing to exercise all seven. The lengths are asserted, not the contents:
// which channels are occupied is this package's invention (doc.go entry 10).
func TestTheDefaultImageServesEveryDeclaredModeClass(t *testing.T) {
	seen := map[int]bool{}
	for ch := 0; ch < 8; ch++ {
		rec, ok := defaultRadioRecord(t, ch)
		if !ok {
			t.Fatalf("the default image holds nothing at group 0 channel %d; it must serve every declared mode class", ch)
		}
		seen[len(rec)] = true
	}
	for _, want := range []int{37, 39, 41, 43, 44, 45} {
		if !seen[want] {
			t.Errorf("no default channel holds a %d-byte record; the accepted record-only set is {37,39,41,43,44,45}", want)
		}
	}
}

// defaultRadioRecord reads one channel of a default fake OFF THE WIRE, so the
// assertion is about what the fake serves rather than what it stores.
func defaultRadioRecord(t *testing.T, channel int) ([]byte, bool) {
	t.Helper()
	r := newTestRadio(t)
	writeToPort(t, r, memRead(0x00, 0x00, 0x00, byte(channel)))
	got := readFrame(t, r)
	if bytes.Equal(got, ngFrame) {
		return nil, false
	}
	if len(got) < 11 {
		t.Fatalf("answer % X is too short to be a record answer", got)
	}
	return got[10 : len(got)-1], true
}

// TestReadingAnEmptyChannelIsAnsweredNG — ASSUMED, doc.go register entry 5.
// PDF p.3 (folio 2) defines FA only as the generic NG code; nothing in the
// guide says what a read of an unwritten channel returns.
func TestReadingAnEmptyChannelIsAnsweredNG(t *testing.T) {
	r := newTestRadio(t, WithEmpty(0, 3))
	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x03))
	wantFrame(t, readFrame(t, r), ngFrame)
}

// TestTheEmptyReplyCanBeAnAllFFRecordInstead — the SECOND, separate assumption
// (doc.go register entry 6). The two are graded apart because one capture
// cannot establish both, and a driver must be exercisable against either.
func TestTheEmptyReplyCanBeAnAllFFRecordInstead(t *testing.T) {
	r := newTestRadio(t, WithEmpty(0, 3), WithEmptyReplyAllFF())
	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x03))

	allFF := bytes.Repeat([]byte{0xFF}, 37)
	wantFrame(t, readFrame(t, r), memAnswer(0x00, 0x00, 0x00, 0x03, allFF))
}

// TestAReadOfAnUnheldGroupIsAnsweredNG — groups 0100 (Auto Write), 0101 (Scan
// Skip) and 0102 (Programmable Scan Edge) are printed on PDF p.12 but this fake
// seeds nothing in them and invents no encoding for the A/B-suffixed scan-edge
// channels. It holds nothing there, and says so.
func TestAReadOfAnUnheldGroupIsAnsweredNG(t *testing.T) {
	r := newTestRadio(t)
	for _, g := range [][2]byte{{0x01, 0x00}, {0x01, 0x01}, {0x01, 0x02}} {
		writeToPort(t, r, memRead(g[0], g[1], 0x00, 0x00))
		wantFrame(t, readFrame(t, r), ngFrame)
	}
}

// ---------------------------------------------------------------------------
// Writing a record
// ---------------------------------------------------------------------------

// TestAnAcceptedSetIsAcknowledgedAndStored — one accepted write, end to end:
// FB comes back, Record returns the bytes that arrived, and a read serves them.
func TestAnAcceptedSetIsAcknowledgedAndStored(t *testing.T) {
	rec := recordFor(0x21, "DCR HERE", tailDCR)
	r := newTestRadio(t, WithEmpty(0, 7))

	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x07, rec))
	wantFrame(t, readFrame(t, r), ackFrame)

	stored, ok := r.Record(0, 7)
	if !ok {
		t.Fatal("Record(0, 7) holds nothing after an acknowledged set")
	}
	if !bytes.Equal(stored, rec) {
		t.Errorf("stored % X\nwant    % X", stored, rec)
	}

	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x07))
	wantFrame(t, readFrame(t, r), memAnswer(0x00, 0x00, 0x00, 0x07, rec))
}

// TestFMAndDCRShareALengthAndAreToldApartByTheModeByte is the whole reason this
// fake reads one byte of a record. Both records are 44 record-only bytes; only
// the mode byte and the tail contents differ, and both must be accepted and
// stored distinctly.
func TestFMAndDCRShareALengthAndAreToldApartByTheModeByte(t *testing.T) {
	fm := recordFor(0x05, "FM", tailFM)
	dcr := recordFor(0x21, "DCR", tailDCR)
	if len(fm) != 44 || len(dcr) != 44 {
		t.Fatalf("the hand-written records are %d and %d bytes; both diagrams measure 44 record-only bytes", len(fm), len(dcr))
	}
	if bytes.Equal(fm, dcr) {
		t.Fatal("the two records are identical; the whole point is that one length carries two different contents")
	}

	r := newTestRadio(t, WithEmpty(0, 20), WithEmpty(0, 21))

	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x20, fm))
	wantFrame(t, readFrame(t, r), ackFrame)
	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x21, dcr))
	wantFrame(t, readFrame(t, r), ackFrame)

	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x20))
	wantFrame(t, readFrame(t, r), memAnswer(0x00, 0x00, 0x00, 0x20, fm))
	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x21))
	wantFrame(t, readFrame(t, r), memAnswer(0x00, 0x00, 0x00, 0x21, dcr))
}

// TestASetWhoseLengthDisagreesWithItsModeIsRefused — the mode byte names a
// layout and the layout names a length. A record in the accepted SET but wrong
// for its own mode is still wrong.
func TestASetWhoseLengthDisagreesWithItsModeIsRefused(t *testing.T) {
	tests := []struct {
		name string
		rec  []byte
	}{
		{"FM mode byte on the dPMR length", recordFor(0x05, "X", tailDPMR)},
		{"DCR mode byte on the NXDN length", recordFor(0x21, "X", tailNXDN)},
		{"AM mode byte carrying an FM tail", recordFor(0x02, "X", tailFM)},
		{"D-STAR mode byte with no tail at all", recordFor(0x17, "X", nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRadio(t, WithEmpty(0, 9))
			writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x09, tt.rec))
			wantFrame(t, readFrame(t, r), ngFrame)
			if _, ok := r.Record(0, 9); ok {
				t.Error("a refused set stored something; it must store nothing at all")
			}
		})
	}
}

// TestASetAtASiblingModelsRecordLengthIsRefused — the wrong-sibling refusal.
// 64 bytes is the IC-905's record, 65 its longer sibling's, and 42 no
// IC-R8600 layout's; none is in {37,39,41,43,44,45}.
func TestASetAtASiblingModelsRecordLengthIsRefused(t *testing.T) {
	for _, n := range []int{7, 36, 38, 40, 42, 46, 64, 65} {
		rec := make([]byte, n)
		// A mode byte that would be perfectly good at the right length, so the
		// refusal is unambiguously about the LENGTH.
		if n > 6 {
			rec[6] = 0x05
		}
		r := newTestRadio(t, WithEmpty(0, 11))
		writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x11, rec))
		wantFrame(t, readFrame(t, r), ngFrame)
		if _, ok := r.Record(0, 11); ok {
			t.Errorf("a %d-byte set was stored; it must be refused and stored nowhere", n)
		}
	}
}

// TestASetWhoseModeByteNamesNoLayoutIsRefused — the mode table prints eighteen
// codes and 09, 10, 12 and 13 are printed nowhere. A fake that guessed a layout
// for them would be inventing one.
func TestASetWhoseModeByteNamesNoLayoutIsRefused(t *testing.T) {
	for _, mode := range []byte{0x09, 0x10, 0x12, 0x13, 0x22, 0x99, 0xFF} {
		rec := recordFor(mode, "X", nil)
		r := newTestRadio(t, WithEmpty(0, 12))
		writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x12, rec))
		wantFrame(t, readFrame(t, r), ngFrame)
		if _, ok := r.Record(0, 12); ok {
			t.Errorf("a set with the undeclared mode byte %#02X was stored", mode)
		}
	}
}

// TestASetOverAnOccupiedChannelMayChangeItsLength — a channel holding an AM
// record accepts an FM record over it, because the mode byte, not the held
// length, decides what is valid. This is the difference from the fixed-geometry
// Icom fakes and it must not regress.
func TestASetOverAnOccupiedChannelMayChangeItsLength(t *testing.T) {
	am := recordFor(0x02, "AM", nil)
	fm := recordFor(0x05, "FM", tailFM)
	r := newTestRadio(t, WithRecord(0, 4, am))

	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x04, fm))
	wantFrame(t, readFrame(t, r), ackFrame)

	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x04))
	wantFrame(t, readFrame(t, r), memAnswer(0x00, 0x00, 0x00, 0x04, fm))
}

// ---------------------------------------------------------------------------
// The clear form, and the short set
// ---------------------------------------------------------------------------

// TestTheClearFormIsRefusedAndChangesNothing. The form IS printed — PDF p.15
// (folio 14): "(1),(2): group; (3),(4): Memory channel number; (5): 'FF';
// (6)~: None" — and this tier ships no erase path at all, so the fake refuses
// it and leaves the channel exactly as it was. doc.go register entry 7.
func TestTheClearFormIsRefusedAndChangesNothing(t *testing.T) {
	rec := recordFor(0x05, "KEEP ME", tailFM)
	r := newTestRadio(t, WithRecord(0, 5, rec))

	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x05, []byte{0xFF}))
	wantFrame(t, readFrame(t, r), ngFrame)

	held, ok := r.Record(0, 5)
	if !ok || !bytes.Equal(held, rec) {
		t.Errorf("after a refused clear the channel holds % X (present=%v); it must be untouched", held, ok)
	}
}

// TestIsClearForm pins the clear form directly, because its refusal is
// invisible on the wire: a clear frame and a one-byte nonsense record both draw
// NG, so only a direct assertion can show that the erase branch is the one that
// bit. The form is the four address bytes, then a single FF, then nothing —
// PDF p.15 (folio 14), "(5): 'FF' / (6)~: None".
func TestIsClearForm(t *testing.T) {
	tests := []struct {
		name string
		rest []byte
		want bool
	}{
		{"the printed clear form", []byte{0xFF}, true},
		{"nothing after the address is a read", nil, false},
		{"one byte that is not FF", []byte{0x00}, false},
		{"FF followed by more is not the clear form", []byte{0xFF, 0xFF}, false},
		{"a whole record beginning FF", append([]byte{0xFF}, make([]byte, 36)...), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClearForm(tt.rest); got != tt.want {
				t.Errorf("isClearForm(% X) = %v, want %v", tt.rest, got, tt.want)
			}
		})
	}
}

// TestAShortSetIsRefusedByDefault. PDF p.12's note says a short set IS accepted
// for FM and Digital modes, with "the default value applied to the omitted
// items" — but the guide prints no default for any omitted byte, so a fake that
// filled them in would be inventing seven values. Refusing is this package's
// choice; WithShortSetsAccepted exists so the open point stays exercisable.
func TestAShortSetIsRefusedByDefault(t *testing.T) {
	head := recordFor(0x05, "FM", nil)
	if len(head) != 37 {
		t.Fatalf("the hand-written head is %d bytes, not 37", len(head))
	}
	r := newTestRadio(t, WithEmpty(0, 6))
	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x06, head))
	wantFrame(t, readFrame(t, r), ngFrame)
	if _, ok := r.Record(0, 6); ok {
		t.Error("a refused short set stored something")
	}
}

// TestAShortSetIsAcceptedWhenTheOptionSaysSo, and the omitted tail bytes are
// the caller's declared fill — never a value this package invented on its own
// authority.
func TestAShortSetIsAcceptedWhenTheOptionSaysSo(t *testing.T) {
	head := recordFor(0x05, "FM", nil)
	r := newTestRadio(t, WithEmpty(0, 6), WithShortSetsAccepted(0x00))

	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x06, head))
	wantFrame(t, readFrame(t, r), ackFrame)

	want := append(append([]byte(nil), head...), bytes.Repeat([]byte{0x00}, 7)...)
	writeToPort(t, r, memRead(0x00, 0x00, 0x00, 0x06))
	wantFrame(t, readFrame(t, r), memAnswer(0x00, 0x00, 0x00, 0x06, want))
}

// TestAShortSetForANoTailModeIsJustAFullSet — an AM record is 37 bytes because
// its layout is 37 bytes, not because anything was omitted, so it is accepted
// with the option off.
func TestAShortSetForANoTailModeIsJustAFullSet(t *testing.T) {
	rec := recordFor(0x02, "AM", nil)
	r := newTestRadio(t, WithEmpty(0, 6))
	writeToPort(t, r, memSet(0x00, 0x00, 0x00, 0x06, rec))
	wantFrame(t, readFrame(t, r), ackFrame)
}

// ---------------------------------------------------------------------------
// Commands this tier never sends
// ---------------------------------------------------------------------------

// TestCommandsOutsideTheTwoAdmittedGrammarsAreRefused. Several of these are
// real IC-R8600 commands — 1A 05 heads the set-mode table on PDF pp.6-7, 1A 0B
// is the programmable scan-start record on PDF p.15, 18 01 is the power-on the
// worked example illustrates — so refusing them is this FAKE's tier policy, not
// a fact about the receiver. doc.go register entry 11.
func TestCommandsOutsideTheTwoAdmittedGrammarsAreRefused(t *testing.T) {
	payloads := [][]byte{
		{0x1A, 0x05, 0x00, 0x92, 0x01}, // CI-V transceive function ON
		{0x1A, 0x0B, 0x00},             // programmable scan start data
		{0x1A, 0x11},                   // read the CI-V connection terminal
		{0x18, 0x01},                   // power on
		{0x1B, 0x01},                   // TSQL frequency
		{0x00},                         // transceive frequency output
		{0x04},                         // read the operating mode
	}
	r := newTestRadio(t)
	for _, p := range payloads {
		writeToPort(t, r, toRadio(p...))
		wantFrame(t, readFrame(t, r), ngFrame)
	}
}

// TestAMemoryFrameTooShortToCarryAnAddressIsRefused.
func TestAMemoryFrameTooShortToCarryAnAddressIsRefused(t *testing.T) {
	r := newTestRadio(t)
	for n := 0; n < 4; n++ {
		writeToPort(t, r, toRadio(append([]byte{0x1A, 0x00}, make([]byte, n)...)...))
		wantFrame(t, readFrame(t, r), ngFrame)
	}
}

// ---------------------------------------------------------------------------
// Echo, broadcasts and the frame log
// ---------------------------------------------------------------------------

// TestTheBusEchoIsOffByDefaultAndEchoesEverythingWhenOn. Neither default is
// printed (PDF p.7 names two per-port echo-back settings and prints no default
// for either), so echo is off unless asked for; when on it is the frame
// VERBATIM, including one addressed to another radio, because a bus echo is a
// property of the wire and not of who was being spoken to.
func TestTheBusEchoIsOffByDefaultAndEchoesEverythingWhenOn(t *testing.T) {
	t.Run("off by default", func(t *testing.T) {
		r := newTestRadio(t)
		req := toRadio(0x19, 0x00)
		writeToPort(t, r, req)
		wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0xDE, 0xAD))
	})

	t.Run("on, and byte-identical", func(t *testing.T) {
		r := newTestRadio(t, WithEcho(true))
		req := toRadio(0x19, 0x00)
		writeToPort(t, r, req)
		wantFrame(t, readFrame(t, r), req)
		wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0xDE, 0xAD))
	})

	t.Run("on, a frame for another radio is echoed and not answered", func(t *testing.T) {
		r := newTestRadio(t, WithEcho(true))
		req := []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}
		writeToPort(t, r, req)
		wantFrame(t, readFrame(t, r), req)
		expectSilence(t, r)
	})
}

// TestTransceiveBroadcastsArriveUnaskedAndAreAddressedToTheBroadcastByte. The
// guide draws two `to` values, 96 and E0, and no broadcast frame anywhere; the
// 00 form is ASSUMED (doc.go register entry 3) and is the form the tier's
// address filter is designed for.
func TestTransceiveBroadcastsArriveUnaskedAndAreAddressedToTheBroadcastByte(t *testing.T) {
	r := newTestRadio(t, WithTransceiveBroadcasts(5*time.Millisecond))
	got := readFrame(t, r)
	if len(got) < 4 || got[2] != 0x00 {
		t.Fatalf("unsolicited frame % X is not addressed to the broadcast byte 00", got)
	}
	if got[3] != 0x96 {
		t.Errorf("unsolicited frame is from %#02X; a broadcast comes from the receiver, 96", got[3])
	}
}

// TestFramesRecordsWhatArrivedIncludingWhatWasRefusedAndIgnored. "The fake
// never saw it" and "the fake saw it and held its tongue" are different facts,
// and a consumer asserting that a refused write put nothing on the wire needs
// to tell them apart.
func TestFramesRecordsWhatArrivedIncludingWhatWasRefusedAndIgnored(t *testing.T) {
	r := newTestRadio(t)

	elsewhere := []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}
	writeToPort(t, r, elsewhere)
	expectSilence(t, r)

	clear := memSet(0x00, 0x00, 0x00, 0x05, []byte{0xFF})
	writeToPort(t, r, clear)
	wantFrame(t, readFrame(t, r), ngFrame)

	frames := r.Frames()
	if len(frames) != 2 {
		t.Fatalf("Frames() returned %d frames, want 2", len(frames))
	}
	if !bytes.Equal(frames[0], elsewhere) {
		t.Errorf("frames[0] = % X, want % X", frames[0], elsewhere)
	}
	if !bytes.Equal(frames[1], clear) {
		t.Errorf("frames[1] = % X, want % X", frames[1], clear)
	}
}

// TestLatencyDelaysTheAnswerWithoutStoppingTheRadio.
func TestLatencyDelaysTheAnswerWithoutStoppingTheRadio(t *testing.T) {
	r := newTestRadio(t, WithLatency(40*time.Millisecond))
	start := time.Now()
	writeToPort(t, r, toRadio(0x19, 0x00))
	wantFrame(t, readFrame(t, r), fromRadio(0x19, 0x00, 0xDE, 0xAD))
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Errorf("the answer arrived after %v; WithLatency(40ms) must hold it back", elapsed)
	}
}

// TestCloseIsIdempotentAndLeavesTheConsumerAnEOF.
func TestCloseIsIdempotentAndLeavesTheConsumerAnEOF(t *testing.T) {
	r := New()
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := r.Port().Read(make([]byte, 1)); err == nil {
		t.Error("reading a closed radio's port succeeded; the consumer's end must see the radio go away")
	}
}

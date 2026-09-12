// SPDX-License-Identifier: GPL-3.0-or-later

package ts570

import (
	"bytes"
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end parses the frames the driver writes and answers each one per a
// configurable image. Deliberately a leaner cut of
// core/driver/ts480/respondingport_test.go's own shape — this row's own
// choreography is ID-only (doc.go), so there is no FV/TY leg and no EX leg
// to script.
type respondingPort struct {
	host, remote net.Conn
	mu           sync.Mutex
	received     []string
}

type radioImage struct {
	catID     string // "" selects the driver's own row.
	idSilent  bool
	idReject  bool
	mrAnswers map[string]string // key: frame[2:6], the four addressing bytes
	mrSilent  map[string]bool
	mwReject  bool
}

func newRespondingPort(t *testing.T, img radioImage) *respondingPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &respondingPort{host: host, remote: remote}
	t.Cleanup(func() {
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve(img)
	return p
}

func (p *respondingPort) serve(img radioImage) {
	buf := make([]byte, 256)
	var acc []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				i := bytes.IndexByte(acc, ';')
				if i < 0 {
					break
				}
				frame := string(acc[:i+1])
				acc = acc[i+1:]
				p.mu.Lock()
				p.received = append(p.received, frame)
				p.mu.Unlock()
				if reply := img.reply(frame); reply != "" {
					if _, werr := p.remote.Write([]byte(reply)); werr != nil {
						return
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *respondingPort) transcript() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.received...)
}

func (img radioImage) reply(frame string) string {
	switch {
	case frame == "ID;":
		switch {
		case img.idSilent:
			return ""
		case img.idReject:
			return "?;"
		}
		return "ID" + img.catID + ";"
	case strings.HasPrefix(frame, "AI"):
		return ""
	case strings.HasPrefix(frame, "MR") && len(frame) == kw.MRReadLen:
		addr := frame[2:6]
		if img.mrSilent[addr] {
			return ""
		}
		if ans, ok := img.mrAnswers[addr]; ok {
			return ans
		}
		return "?;"
	case strings.HasPrefix(frame, "MW"):
		if img.mwReject {
			return "?;"
		}
		return ""
	default:
		return "?;"
	}
}

func testTiming() Option { return withTiming(200*time.Millisecond, 5*time.Millisecond) }

func openSession(t *testing.T, m modelParams, profile Profile, img radioImage, opts ...Option) (*Session, *respondingPort) {
	t.Helper()
	if img.catID == "" {
		img.catID = m.catID
	}
	p := newRespondingPort(t, img)
	d := newDriver(m, profile, append([]Option{testTiming()}, opts...)...)
	sess, err := d.Open(context.Background(), p.host, driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open(%s): %v", m.name, err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), p
}

// mrAddr renders the four addressing bytes of an MR read of channel number
// (P1 '0', P2 unused printed '0', P3 the two-digit channel number).
func mrAddr(number int) string {
	return string([]byte{'0', '0', byte('0' + number/10), byte('0' + number%10)})
}

// TestOpen_IdentityOnly pins the ID-only probe: exactly one frame beyond
// AI0; goes out.
func TestOpen_IdentityOnly(t *testing.T) {
	sess, p := openSession(t, modelD, RealHardware, radioImage{})
	if got := sess.Identity().CATID; got != "017" {
		t.Errorf("Identity().CATID = %q, want %q", got, "017")
	}
	if got := p.transcript(); len(got) != 2 || got[0] != "AI0;" || got[1] != "ID;" {
		t.Errorf("transcript = %v, want exactly [AI0; ID;]", got)
	}
}

// TestOpen_WrongRadio refuses a session against another row's ID.
func TestOpen_WrongRadio(t *testing.T) {
	p := newRespondingPort(t, radioImage{catID: modelS.catID})
	_, err := NewD(RealHardware, testTiming()).Open(context.Background(), p.host, driver.Identity{})
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open err = %v, want *driver.WrongRadioError", err)
	}
	if wrong.Want != "017" || wrong.Got != "018" || wrong.GotModel != "TS-570S" {
		t.Errorf("WrongRadioError = %+v", wrong)
	}
}

// mr28 builds a legal 28-byte MR answer for one channel: 14.25 MHz, USB,
// unlocked, tone ON, P8 index 02 (719 decihertz, 71.9 Hz —
// ts570CTCSSTones[1]).
func mr28(number int) string {
	b := make([]byte, 28)
	copy(b, "MR")
	b[2] = '0'
	copy(b[3:6], mrAddr(number)[1:])
	copy(b[6:17], "00014250000") // P4, 11 digits, 14,250,000 Hz
	b[17] = '2'                  // USB
	b[18] = '0'                  // lockout off
	b[19] = '1'                  // tone ON
	copy(b[20:22], "02")
	b[27] = ';'
	return string(b)
}

// TestReadChannel_PopulatedChannel round-trips an ordinary channel.
func TestReadChannel_PopulatedChannel(t *testing.T) {
	sess, _ := openSession(t, modelD, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr(7): mr28(7)},
	})
	ch, err := sess.ReadChannel(context.Background(), "07")
	if err != nil {
		t.Fatalf("ReadChannel(07): %v", err)
	}
	if ch.Data == nil {
		t.Fatal("ReadChannel(07): Data is nil, want a populated channel")
	}
	if ch.Data.FreqHz != 14_250_000 || ch.Data.Mode != "USB" {
		t.Errorf("channel = %+v", ch.Data)
	}
	if ch.Data.ToneMode.Value != "ON" || ch.Data.ToneTx.Value != 719 || ch.Data.ToneRx.Value != 719 {
		t.Errorf("tone = mode %+v tx %+v rx %+v", ch.Data.ToneMode, ch.Data.ToneTx, ch.Data.ToneRx)
	}
}

// TestReadChannel_TrulyEmptyChannel pins this row's own documented vacant
// shape (read.go's isVacantAnswer): P4 through P8 all zero, channel number
// preserved, is the empty channel, never a failure — routed around
// core/kw's isEmptyWindow, which cannot see this on a 28-byte frame.
func TestReadChannel_TrulyEmptyChannel(t *testing.T) {
	b := make([]byte, 28)
	copy(b, "MR0007")
	for i := 6; i <= 26; i++ {
		b[i] = '0'
	}
	b[27] = ';'
	sess, _ := openSession(t, modelD, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr(7): string(b)},
	})
	ch, err := sess.ReadChannel(context.Background(), "07")
	if err != nil {
		t.Fatalf("ReadChannel(07): %v", err)
	}
	if ch.Data != nil {
		t.Errorf("ReadChannel(07) = %+v, want an empty channel (nil Data)", ch.Data)
	}
}

// TestWriteChannel_CurrentlyRefusedByCoreKWsOwnGate PINS A GENUINE core/kw
// GAP RATHER THAN THIS PACKAGE'S OWN BEHAVIOUR: every field this ladder
// accepts still fails at the wire, on every profile, because
// core/kw.BuildMWSet and core/kw.Layout.AllowedCommand disagree with
// EACH OTHER for a RecordLen=28 (no-tail) layout, and this package cannot
// resolve that disagreement without editing core/kw (out of scope; see
// doc.go).
//
// BuildMWSet leaves positions 23-27 (the P9 "NOT USED" span, matrix §1.2)
// as Go's zero byte, 0x00 — never written on the hasTail-false path
// (core/kw/builders.go: the tail block that would fill them is guarded by
// `if hasTail`). A 0x00 byte fails the outbound envelope's printable-ASCII
// rule. During this package's own development a fix was tried — filling
// that span with the '0' filler byte the codec already uses for the
// P2Unused position — and it does NOT help: AllowedCommand's
// validMWCommand re-parses the frame and re-builds it via BuildMWSet, then
// requires byte-for-byte equality with THAT rebuild, which carries the
// same zero bytes. No 28-byte MW frame can satisfy both checks
// simultaneously as core/kw stands today.
//
// So this test asserts the CURRENT, fully reproducible outcome — a gate
// refusal, not a wire round trip — so a future core/kw fix that fills the
// P9 span on the build side breaks this test LOUDLY rather than leaving a
// silently-still-broken write path uncaught.
func TestWriteChannel_CurrentlyRefusedByCoreKWsOwnGate(t *testing.T) {
	sess, _ := openSession(t, modelS, Simulated, radioImage{})
	ch := codeplug.Channel{Slot: "07", Data: &codeplug.ChannelData{
		FreqHz:   7_100_000,
		Mode:     "LSB",
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 670},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 670},
	}}
	_, err := sess.WriteChannel(context.Background(), ch)
	if err == nil {
		t.Fatal("WriteChannel succeeded — core/kw's BuildMWSet/AllowedCommand gap for this row appears to be fixed; update this test (and doc.go, and reviews/driver-ts570.md) to assert the round trip instead")
	}
	if !strings.Contains(err.Error(), "AllowedCommand") {
		t.Fatalf("WriteChannel err = %v, want a gate refusal naming AllowedCommand (the known core/kw gap)", err)
	}
}

// TestWriteChannel_ToneTxRxMismatchRefused pins the one rung this row adds
// beyond every sibling package's ladder: one byte cannot carry two tones.
func TestWriteChannel_ToneTxRxMismatchRefused(t *testing.T) {
	sess, _ := openSession(t, modelD, Simulated, radioImage{})
	ch := codeplug.Channel{Slot: "07", Data: &codeplug.ChannelData{
		FreqHz:   7_100_000,
		Mode:     "LSB",
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "ON"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 670},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 719},
	}}
	if _, err := sess.WriteChannel(context.Background(), ch); err == nil {
		t.Fatal("WriteChannel accepted disagreeing tone_tx/tone_rx, which this row's one P8 byte cannot carry")
	}
}

// TestWriteChannel_EraseRefused pins that this row's stronger documented
// erase citation still cannot be built (doc.go, caps.go).
func TestWriteChannel_EraseRefused(t *testing.T) {
	sess, _ := openSession(t, modelD, Simulated, radioImage{})
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "07"})
	if err == nil {
		t.Fatal("WriteChannel accepted an empty (erase) channel, which no Kenwood row in this codec can build")
	}
}

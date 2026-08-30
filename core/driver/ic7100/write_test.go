// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func openProfileSession(t *testing.T, profile Profile, p *respondingPort, opts ...Option) *Session {
	t.Helper()
	sess, err := New(profile, opts...).Open(context.Background(), p.Port(), driver.Identity{Port: "test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session)
}

func writableChannel(t *testing.T, s *Session) codeplug.Channel {
	t.Helper()
	ch, err := s.ReadChannel(context.Background(), "A-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data == nil {
		t.Fatal("A-001 is empty")
	}
	ch.Data.Tag = "WRITE TEST"
	return ch
}

func countSets(frames [][]byte) int {
	n := 0
	for _, f := range frames {
		if len(f) == 121 && f[4] == 0x1A && f[5] == 0x00 {
			n++
		}
	}
	return n
}

func TestWriteChannelUnconsentedStopsBeforeWire(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	s := openProfileSession(t, RealHardware, p)
	ch := writableChannel(t, s)
	before := len(p.frames())
	result, err := s.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("error = %v, want ErrWriteRefused", err)
	}
	if len(result.Steps) != 0 || len(p.frames()) != before {
		t.Errorf("unconsented refusal = %+v, sent %d frames", result, len(p.frames())-before)
	}
}

func TestWriteChannelConsentedSendsOneFullAcknowledgedSet(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	s := openProfileSession(t, RealHardware, p, WithConsentedUnverifiedWrites())
	ch := writableChannel(t, s)
	beforeSets := countSets(p.frames())
	result, err := s.WriteChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(result.Steps) != 1 || !result.Steps[0].Sent || !result.Steps[0].Confirmed {
		t.Errorf("result = %+v", result)
	}
	if got := countSets(p.frames()) - beforeSets; got != 1 {
		t.Errorf("sent %d sets, want exactly one", got)
	}
	stored := p.recordAt(1, 1)
	if len(stored) != 111 {
		t.Fatalf("stored record length = %d", len(stored))
	}
	if !bytes.Equal(stored[1:48], stored[48:95]) {
		t.Error("TX data-area bytes 52–98 were not generated from RX bytes 5–51 (47-byte arithmetic)")
	}
}

func TestWriteChannelTimeoutNeverRetransmits(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)), withNoSetAnswer())
	s := openProfileSession(t, Simulated, p)
	ch := writableChannel(t, s)
	beforeSets := countSets(p.frames())
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	result, err := s.WriteChannel(ctx, ch)
	if err == nil {
		t.Fatal("timeout write succeeded")
	}
	if got := countSets(p.frames()) - beforeSets; got != 1 {
		t.Errorf("timeout sent %d sets, want one and never retransmitted", got)
	}
	if len(result.Steps) != 1 || result.Steps[0].Sent || result.Steps[0].Confirmed {
		t.Errorf("timeout result = %+v, want unattributable", result)
	}
}

func TestWriteChannelFAIsExplicitRadioRefusal(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)), withRejectedSets())
	s := openProfileSession(t, Simulated, p)
	result, err := s.WriteChannel(context.Background(), writableChannel(t, s))
	if err == nil || !strings.Contains(err.Error(), "FA") {
		t.Fatalf("error = %v, want explicit FA", err)
	}
	if len(result.Steps) != 1 || !result.Steps[0].Sent || result.Steps[0].Confirmed {
		t.Errorf("FA result = %+v", result)
	}
}

func TestWriteChannelEraseAndOutOfBaseAlwaysRefuseBeforeTraffic(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	s := openProfileSession(t, RealHardware, p, WithConsentedUnverifiedWrites())
	before := len(p.frames())
	for _, ch := range []codeplug.Channel{{Slot: "A-001"}, {Slot: "A-100", Data: &codeplug.ChannelData{FreqHz: 145_500_000, Mode: "FM", Tag: "X"}}} {
		result, err := s.WriteChannel(context.Background(), ch)
		if !errors.Is(err, driver.ErrWriteRefused) || len(result.Steps) != 0 {
			t.Errorf("WriteChannel(%s) = %+v, %v", ch.Slot, result, err)
		}
	}
	if len(p.frames()) != before {
		t.Errorf("local refusals sent %d frames", len(p.frames())-before)
	}
}

func TestWriteChannelOffsetAboveDocumentedMaximumRefusesBeforeTraffic(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	s := openProfileSession(t, Simulated, p)
	ch := writableChannel(t, s)
	ch.Data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 10_000_000}
	before := len(p.frames())

	result, err := s.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) || !strings.Contains(err.Error(), "9,999,900") {
		t.Fatalf("error = %v, want documented-offset ErrWriteRefused", err)
	}
	if len(result.Steps) != 0 || len(p.frames()) != before {
		t.Errorf("offset refusal = %+v, sent %d frames", result, len(p.frames())-before)
	}
}

func TestWriteChannelE6RefusesUnmodelledDStarDSQLCSQLBytes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset int
	}{{"DSQL", 10}, {"CSQL", 20}, {"D-STAR UR", 24}} {
		t.Run(tc.name, func(t *testing.T) {
			record := occupiedRecord(t)
			record[tc.offset] ^= 0x01
			p := newRespondingPort(t, withRecord(1, 1, record))
			s := openProfileSession(t, Simulated, p)
			// Build the requested neutral values from the clean evidence record;
			// the write's own mandatory read is the altered record under test.
			cleanPort := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
			cleanSession := openProfileSession(t, Simulated, cleanPort)
			ch := writableChannel(t, cleanSession)
			beforeSets := countSets(p.frames())
			result, err := s.WriteChannel(context.Background(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("error = %v, want named %s ErrWriteRefused", err, tc.name)
			}
			if len(result.Steps) != 0 || countSets(p.frames()) != beforeSets {
				t.Errorf("E6 result = %+v; a set reached the wire", result)
			}
		})
	}
}

func TestWriteChannelE6RefusesSplitONWithoutClearingIt(t *testing.T) {
	record := occupiedRecord(t)
	record[0] |= 0x10 // record byte ④ high nibble: Split ON
	p := newRespondingPort(t, withRecord(1, 1, record))
	s := openProfileSession(t, Simulated, p)

	cleanPort := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	cleanSession := openProfileSession(t, Simulated, cleanPort)
	ch := writableChannel(t, cleanSession)
	beforeSets := countSets(p.frames())

	result, err := s.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) || !strings.Contains(strings.ToLower(err.Error()), "split") {
		t.Fatalf("error = %v, want named split-flag ErrWriteRefused", err)
	}
	if len(result.Steps) != 0 || countSets(p.frames()) != beforeSets {
		t.Errorf("split E6 result = %+v; a set reached the wire", result)
	}
}

func TestWriteChannelNormalFMRecordCanWrite(t *testing.T) {
	p := newRespondingPort(t, withRecord(1, 1, occupiedRecord(t)))
	s := openProfileSession(t, Simulated, p)
	if _, err := s.WriteChannel(context.Background(), writableChannel(t, s)); err != nil {
		t.Fatalf("normal FM write: %v", err)
	}
}

func TestWriteChannelDriverStillHasNoSerialFramingReporter(t *testing.T) {
	if _, ok := New(RealHardware).(driver.SerialFramingReporter); ok {
		t.Fatal("IC-7100 driver exposes framing from PDF p.174's unrelated DV low-speed data line")
	}
}

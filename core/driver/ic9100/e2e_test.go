// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic9100 "github.com/gm5dna/open-rig-programmer/core/civ/ic9100"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

func testRecord(t *testing.T, band, channel int, freqHz uint64, name string) civ.MemoryRecord {
	t.Helper()
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: band, Channel: channel},
		Select:       civ.Available("OFF"),
		RXFreqHz:     civ.Available(freqHz),
		OffsetHz:     civ.Available(uint64(600_000)),
		ToneTXDeciHz: civ.Available(uint64(885)),
		ToneRXDeciHz: civ.Available(uint64(885)),
		DTCSCode:     civ.Available(uint64(23)),
		DTCSPolarity: civ.Available("NN"),
		Duplex:       civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		Mode:         civ.Available("FM"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		Name:         civ.Available(name),
	}
}

// recordBytes renders rec as the 57 raw record bytes a memory answer
// carries, using this package's own civ encoder.
func recordBytes(t *testing.T, rec civ.MemoryRecord) []byte {
	t.Helper()
	cmd, err := civic9100.Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet(%v): %v", rec.Address, err)
	}
	b := cmd.Bytes()
	return append([]byte(nil), b[9:len(b)-1]...)
}

func openScripted(t *testing.T, img radioImage, profile driver.Profile, opts ...Option) (*Session, *scriptedPort) {
	t.Helper()
	p := newScriptedPort(t, img)
	sess, err := New(profile, opts...).Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), p
}

func TestE2E_ProbeFingerprintsFromAddressMatchedIdentityReply(t *testing.T) {
	occupied := recordBytes(t, testRecord(t, 0, 1, 145_500_000, "HOME BASE"))
	s, _ := openScripted(t, radioImage{
		idToken: []byte{0xDE, 0xAD},
		records: map[bandChannel]([]byte){{0, 1}: occupied},
	}, Simulated)

	d := s.CIVDiagnostics()
	if !d.Fingerprinted || d.Status != "FINGERPRINTED 57 B" {
		t.Errorf("diagnostics = %+v, want a 57-byte fingerprint from the first occupied slot", d)
	}
	if d.ProbeSlotsRead != 1 {
		t.Errorf("ProbeSlotsRead = %d, want 1 — HF-001 is occupied and the search is bounded", d.ProbeSlotsRead)
	}
	if got, want := s.Identity().CATID, "7C:dead"; got != want {
		t.Errorf("CATID = %q, want %q — the headline-finding address 7C, followed by the observed token", got, want)
	}
}

func TestE2E_EmptyRadioOpensExplicitlyUnfingerprinted(t *testing.T) {
	s, _ := openScripted(t, radioImage{idToken: []byte{0x12}}, Simulated)
	d := s.CIVDiagnostics()
	if d.Fingerprinted || d.Status != "UNFINGERPRINTED" {
		t.Errorf("diagnostics = %+v, want an explicitly unfingerprinted open", d)
	}
	if d.ProbeSlotsRead != probeSlots {
		t.Errorf("ProbeSlotsRead = %d, want the bounded search's full %d", d.ProbeSlotsRead, probeSlots)
	}
}

func TestE2E_ReadChannel_OccupiedAndEmptyForms(t *testing.T) {
	occupied := recordBytes(t, testRecord(t, 1, 50, 433_500_000, "144 MID"))
	s, _ := openScripted(t, radioImage{
		idToken: []byte{0x00},
		records: map[bandChannel]([]byte){{1, 50}: occupied},
	}, Simulated)

	ch, err := s.ReadChannel(context.Background(), "144-050")
	if err != nil || ch.Data == nil {
		t.Fatalf("ReadChannel(144-050) = %+v, %v", ch, err)
	}
	if ch.Data.FreqHz != 433_500_000 || ch.Data.Tag != "144 MID" {
		t.Errorf("ReadChannel(144-050) = %+v", ch.Data)
	}
	// TxFreqHz must be Unavailable: this record has no TX-side frequency
	// block (matrix §1b).
	if ch.Data.TxFreqHz.State != codeplug.Unavailable {
		t.Errorf("TxFreqHz = %+v, want Unavailable — no tx_frequency span exists on this record", ch.Data.TxFreqHz)
	}

	// An FA-rejected read is an empty channel, not an error.
	empty, err := s.ReadChannel(context.Background(), "HF-002")
	if err != nil {
		t.Fatalf("ReadChannel(HF-002) unexpected error: %v", err)
	}
	if empty.Data != nil {
		t.Errorf("ReadChannel(HF-002) came back populated, want empty (FA)")
	}
}

func TestE2E_ReadChannel_AllFFRecordIsAlsoEmpty(t *testing.T) {
	allFFRecord := make([]byte, civic9100.RecordLength)
	for i := range allFFRecord {
		allFFRecord[i] = 0xFF
	}
	s, _ := openScripted(t, radioImage{
		idToken: []byte{0x00},
		records: map[bandChannel]([]byte){{0, 3}: allFFRecord},
	}, Simulated)
	ch, err := s.ReadChannel(context.Background(), "HF-003")
	if err != nil {
		t.Fatalf("ReadChannel(HF-003): %v", err)
	}
	if ch.Data != nil {
		t.Errorf("ReadChannel(HF-003) decoded an all-FF record into a channel")
	}
}

func writableChannel(t *testing.T) codeplug.Channel {
	t.Helper()
	return codeplug.Channel{Slot: "HF-001", Data: &codeplug.ChannelData{
		FreqHz: 7_100_000, Mode: "LSB", Tag: "WRITE TST",
		Filter:       codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
		ToneMode:     codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:       codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
		ToneRx:       codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
		DTCSCode:     codeplug.IntField{State: codeplug.Known, Value: 23},
		DTCSPolarity: codeplug.StringField{State: codeplug.Known, Value: "NN"},
		Duplex:       codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		OffsetHz:     codeplug.FreqField{State: codeplug.Known, Value: 600_000},
		DataMode:     codeplug.BoolField{State: codeplug.Known, Value: false},
	}}
}

func TestE2E_ConsentedWriteIsAcknowledgedAndReadsBack(t *testing.T) {
	occupied := recordBytes(t, testRecord(t, 0, 1, 145_500_000, "OLD NAME"))
	s, p := openScripted(t, radioImage{
		idToken: []byte{0x00},
		records: map[bandChannel]([]byte){{0, 1}: occupied},
		ackSets: true,
	}, RealHardware, WithConsentedUnverifiedWrites())

	res, err := s.WriteChannel(context.Background(), writableChannel(t))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Fatalf("steps = %+v, want one sent and FB-confirmed frame", res.Steps)
	}
	if got := countSets(p.Transcript()); got != 1 {
		t.Errorf("the radio received %d sets, want exactly 1", got)
	}
}

func TestE2E_WriteChannel_RefusedWithoutConsent(t *testing.T) {
	occupied := recordBytes(t, testRecord(t, 0, 1, 145_500_000, "OLD NAME"))
	s, p := openScripted(t, radioImage{
		idToken: []byte{0x00},
		records: map[bandChannel]([]byte){{0, 1}: occupied},
		ackSets: true,
	}, RealHardware)

	before := len(p.Transcript())
	_, err := s.WriteChannel(context.Background(), writableChannel(t))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want ErrWriteRefused on an unconsented real-hardware session", err)
	}
	if got := len(p.Transcript()) - before; got != 0 {
		t.Errorf("the refusal put %d frames on the wire, want none", got)
	}
}

func TestE2E_WriteChannel_DStarUnmappedRegionDiffersRefuses(t *testing.T) {
	// A slot whose D-STAR destination call sign differs from the assumed
	// "CQCQCQ  " template: doc.go's E6-style preserve-or-refuse rule must
	// refuse the write rather than silently rewrite the region.
	rec := testRecord(t, 0, 4, 145_500_000, "DSTAR CH")
	raw := recordBytes(t, rec)
	raw[civic9100.DestCallOffset] = 'Z' // differs from the template's 'C'

	s, p := openScripted(t, radioImage{
		idToken: []byte{0x00},
		records: map[bandChannel]([]byte){{0, 4}: raw},
		ackSets: true,
	}, RealHardware, WithConsentedUnverifiedWrites())

	before := countSets(p.Transcript())
	ch := writableChannel(t)
	ch.Slot = "HF-004"
	_, err := s.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want ErrWriteRefused for a D-STAR-region mismatch", err)
	}
	if got := countSets(p.Transcript()) - before; got != 0 {
		t.Errorf("the refusal put %d set frame(s) on the wire, want none", got)
	}
}

func TestE2E_WriteChannel_AckTimeoutSendsExactlyOneSet(t *testing.T) {
	s, p := openScripted(t, radioImage{idToken: []byte{0x00}}, RealHardware, WithConsentedUnverifiedWrites())

	before := countSets(p.Transcript())
	_, err := s.WriteChannel(context.Background(), writableChannel(t))
	if err == nil {
		t.Fatal("the driver reported success for a set the radio never acknowledged")
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("err = %v, want transport.ErrTimeout", err)
	}
	if got := countSets(p.Transcript()) - before; got != 1 {
		t.Errorf("the timed-out write put %d set frames on the wire, want exactly 1 — a write is NEVER resent", got)
	}
}

func TestE2E_Open_WrongRecordLengthIsRefusedAndCanBeAttributed(t *testing.T) {
	const nearMiss = 25
	s, err := New(Simulated).Open(context.Background(), newScriptedPort(t, radioImage{
		idToken: []byte{0x00},
		records: map[bandChannel]([]byte){{0, 1}: make([]byte, nearMiss)},
	}).Port(), driver.Identity{})
	if err == nil {
		_ = s.Close()
		t.Fatal("Open accepted a radio whose records are 25 bytes")
	}
	var wrong *driver.WrongRadioError
	if !errors.Is(err, driver.ErrWrongRadio) || !errors.As(err, &wrong) {
		t.Fatalf("err = %v, want a WrongRadioError", err)
	}
	if wrong.Want != "record 57" || wrong.Got != "record 25" {
		t.Errorf("WrongRadioError = %+v, want the two RECORD-ONLY lengths", wrong)
	}
}

func countSets(frames [][]byte) int {
	n := 0
	for _, f := range frames {
		if len(f) >= memSetFrameLen && f[4] == civ.CmdMemory && f[5] == civ.SubMemoryContents {
			n++
		}
	}
	return n
}

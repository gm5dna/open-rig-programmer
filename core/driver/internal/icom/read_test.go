// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

const testRadioAddr = 0x74

// freqOnlyProfile is a one-channel profile whose record maps a frequency and
// nothing else, so ReadChannel meets "no mode" after the frequency.
var freqOnlyProfile = civ.MustNewProfile(civ.ProfileConfig{
	Model:        "TEST",
	RadioAddress: testRadioAddr,
	AddressForm:  civ.AddressFormFlat,
	ChannelLo:    1,
	ChannelHi:    1,
	Layouts: []civ.RecordLayout{{
		Length: 5,
		Fixed:  make([]byte, 5),
		Fields: []civ.FieldSpan{{Field: civ.FieldRXFrequency, Offset: 0, Length: 5, Encoding: civ.EncodingBCDNumber, Order: civ.OrderLittleEndian, Scale: 1}},
	}},
	Discriminator: civ.DiscriminatorSingleLength,
	BuildLength:   5,
})

// answerPort answers every read with one canned frame, then reports EOF.
type answerPort struct{ answer []byte }

func (p *answerPort) Read(b []byte) (int, error) {
	if len(p.answer) == 0 {
		return 0, io.EOF
	}
	n := copy(b, p.answer)
	p.answer = p.answer[n:]
	return n, nil
}
func (*answerPort) Write(b []byte) (int, error) { return len(b), nil }
func (*answerPort) Close() error                { return nil }

type testDomainError struct{ driver.OutOfDomainError }

func (e *testDomainError) Error() string { return "test: out of domain" }
func (e *testDomainError) Unwrap() error { return driver.ErrOutOfDomain }

// readFreqOnly reads channel 1 of a freqOnlyProfile radio holding hz, under
// a Domain refusing anything above 100 MHz.
func readFreqOnly(t *testing.T, hz uint64) error {
	t.Helper()
	cmd, err := freqOnlyProfile.BuildMemorySet(civ.MemoryRecord{Address: civ.ChannelAddress{Channel: 1}, RXFreqHz: civ.Available(hz)})
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	answer := append([]byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), testRadioAddr}, frame[4:]...)
	framing, err := civ.NewFraming(freqOnlyProfile)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	eng, err := transport.NewEngineWith(&answerPort{answer: answer}, framing)
	if err != nil {
		t.Fatalf("NewEngineWith: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	p := &Params{
		Name:    "test",
		Profile: func() civ.Profile { return freqOnlyProfile },
		Domain: func(d codeplug.ChannelData, _ spec.Capabilities) error {
			if d.FreqHz > 100_000_000 {
				return &testDomainError{driver.OutOfDomainError{Field: spec.FieldFrequency, Value: d.FreqHz, Max: 100_000_000}}
			}
			return nil
		},
	}
	_, err = ReadChannel(t.Context(), p, eng, spec.Capabilities{}, new(atomic.Uint64), "001")
	return err
}

// TestReadChannelDomainRefusalPrecedesNoMode pins the error precedence: an
// out-of-domain RX frequency is reported as the domain refusal even though
// the record also carries no mode, and a record inside the domain still
// reaches the "no mode" refusal.
func TestReadChannelDomainRefusalPrecedesNoMode(t *testing.T) {
	err := readFreqOnly(t, 200_000_000)
	var domain *testDomainError
	if !errors.As(err, &domain) {
		t.Fatalf("ReadChannel = %v, want the domain refusal ahead of \"no mode\"", err)
	}
	if err := readFreqOnly(t, 14_250_000); err == nil || !strings.Contains(err.Error(), "no mode") {
		t.Fatalf("ReadChannel in-domain = %v, want the \"no mode\" refusal", err)
	}
}

// TestReadChannelDomainRefusalIsADecodeFailure pins that a read-side Domain
// refusal is classed per slot (driver.ErrRecordDecode, which core/clone
// records in FailedSlots) and still matches the package's own error, with
// the model named once.
func TestReadChannelDomainRefusalIsADecodeFailure(t *testing.T) {
	err := readFreqOnly(t, 200_000_000)
	if !errors.Is(err, driver.ErrRecordDecode) || !errors.Is(err, driver.ErrOutOfDomain) {
		t.Fatalf("errors.Is(%v, ErrRecordDecode) or ErrOutOfDomain = false", err)
	}
	if want := "ReadChannel 001: test: out of domain"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

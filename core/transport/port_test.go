// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"go.bug.st/serial"
)

// fakeSerialPort is the test seam's fake: a minimal in-memory stand-in for
// go.bug.st/serial.Port that OpenSerial's openPort seam can return instead
// of a real device, so config mapping and error wrapping are testable
// without physical hardware (see openPort's doc comment).
type fakeSerialPort struct {
	rts, dtr      []bool // every SetRTS/SetDTR call, in order
	readTimeout   time.Duration
	readTimeoutN  int
	closeErr      error
	closed        bool
	setRTSErr     error
	setDTRErr     error
	setTimeoutErr error
}

func (f *fakeSerialPort) Read(p []byte) (int, error)  { return 0, io.EOF }
func (f *fakeSerialPort) Write(p []byte) (int, error) { return len(p), nil }
func (f *fakeSerialPort) Close() error                { f.closed = true; return f.closeErr }
func (f *fakeSerialPort) SetRTS(rts bool) error {
	f.rts = append(f.rts, rts)
	return f.setRTSErr
}
func (f *fakeSerialPort) SetDTR(dtr bool) error {
	f.dtr = append(f.dtr, dtr)
	return f.setDTRErr
}
func (f *fakeSerialPort) SetReadTimeout(t time.Duration) error {
	f.readTimeout = t
	f.readTimeoutN++
	return f.setTimeoutErr
}

// withFakeOpenPort swaps the openPort seam for the duration of a test,
// restoring the original on cleanup, and returns the *serial.Mode OpenSerial
// actually requested (captured by the fake's open function) once the test
// body has called OpenSerial.
func withFakeOpenPort(t *testing.T, port *fakeSerialPort, openErr error) *capturedMode {
	t.Helper()
	cm := &capturedMode{}
	orig := openPort
	openPort = func(path string, mode *serial.Mode) (serialPort, error) {
		cm.path = path
		cm.mode = mode
		if openErr != nil {
			return nil, openErr
		}
		return port, nil
	}
	t.Cleanup(func() { openPort = orig })
	return cm
}

type capturedMode struct {
	path string
	mode *serial.Mode
}

// --- SerialConfig mapping ---

func TestOpenSerial_ConfigMapping(t *testing.T) {
	tests := []struct {
		name         string
		cfg          SerialConfig
		wantBaud     int
		wantStopBits serial.StopBits
	}{
		{"zero value selects defaults", SerialConfig{}, DefaultBaud, serial.TwoStopBits},
		{"explicit baud, default stop bits", SerialConfig{Baud: 9600}, 9600, serial.TwoStopBits},
		{"explicit 1 stop bit", SerialConfig{Baud: 4800, StopBits: 1}, 4800, serial.OneStopBit},
		{"explicit 2 stop bits", SerialConfig{Baud: 115200, StopBits: 2}, 115200, serial.TwoStopBits},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := &fakeSerialPort{}
			cm := withFakeOpenPort(t, fp, nil)

			p, err := OpenSerial("/dev/cu.fake", tt.cfg)
			if err != nil {
				t.Fatalf("OpenSerial: unexpected error: %v", err)
			}
			defer p.Close()

			if cm.path != "/dev/cu.fake" {
				t.Errorf("openPort path = %q, want %q", cm.path, "/dev/cu.fake")
			}
			if cm.mode == nil {
				t.Fatal("openPort mode = nil")
			}
			if cm.mode.BaudRate != tt.wantBaud {
				t.Errorf("BaudRate = %d, want %d", cm.mode.BaudRate, tt.wantBaud)
			}
			if cm.mode.DataBits != 8 {
				t.Errorf("DataBits = %d, want 8", cm.mode.DataBits)
			}
			if cm.mode.Parity != serial.NoParity {
				t.Errorf("Parity = %v, want NoParity", cm.mode.Parity)
			}
			if cm.mode.StopBits != tt.wantStopBits {
				t.Errorf("StopBits = %v, want %v", cm.mode.StopBits, tt.wantStopBits)
			}
		})
	}
}

func TestOpenSerial_RejectsUnsupportedStopBits(t *testing.T) {
	fp := &fakeSerialPort{}
	withFakeOpenPort(t, fp, nil)

	_, err := OpenSerial("/dev/cu.fake", SerialConfig{StopBits: 3})
	if err == nil {
		t.Fatal("OpenSerial with StopBits=3: want error, got nil")
	}
	if fp.closed {
		t.Error("port was opened (and closed) despite an invalid config — openPort should never have been called")
	}
}

// --- RTS/DTR/read-timeout sequencing (safety obligation 4) ---

func TestOpenSerial_DrivesRTSAndDTRLowImmediately(t *testing.T) {
	fp := &fakeSerialPort{}
	withFakeOpenPort(t, fp, nil)

	p, err := OpenSerial("/dev/cu.fake", SerialConfig{})
	if err != nil {
		t.Fatalf("OpenSerial: unexpected error: %v", err)
	}
	defer p.Close()

	if len(fp.rts) != 1 || fp.rts[0] != false {
		t.Errorf("SetRTS calls = %v, want exactly one call with false", fp.rts)
	}
	if len(fp.dtr) != 1 || fp.dtr[0] != false {
		t.Errorf("SetDTR calls = %v, want exactly one call with false", fp.dtr)
	}
	if fp.readTimeoutN != 1 {
		t.Errorf("SetReadTimeout call count = %d, want 1", fp.readTimeoutN)
	}
	if fp.readTimeout <= 0 {
		t.Errorf("SetReadTimeout duration = %v, want a short positive poll interval", fp.readTimeout)
	}
}

func TestOpenSerial_SetRTSFailure_ClosesPortAndReturnsError(t *testing.T) {
	fp := &fakeSerialPort{setRTSErr: errors.New("boom")}
	withFakeOpenPort(t, fp, nil)

	_, err := OpenSerial("/dev/cu.fake", SerialConfig{})
	if err == nil {
		t.Fatal("OpenSerial: want error when SetRTS fails, got nil")
	}
	if !fp.closed {
		t.Error("port was not closed after SetRTS failed")
	}
}

func TestOpenSerial_SetDTRFailure_ClosesPortAndReturnsError(t *testing.T) {
	fp := &fakeSerialPort{setDTRErr: errors.New("boom")}
	withFakeOpenPort(t, fp, nil)

	_, err := OpenSerial("/dev/cu.fake", SerialConfig{})
	if err == nil {
		t.Fatal("OpenSerial: want error when SetDTR fails, got nil")
	}
	if !fp.closed {
		t.Error("port was not closed after SetDTR failed")
	}
}

// --- Busy-error wrapping ---

func TestOpenSerial_WrapsPortBusyError(t *testing.T) {
	// serial.PortError's fields are unexported, so a zero-value literal is
	// the only *serial.PortError a test outside go.bug.st/serial can
	// construct — its Code() is the zero PortErrorCode, which IS PortBusy
	// (PortBusy is the first, zero-valued iota constant; pinned by
	// TestPortBusyIsZeroValue below so this construction can't silently
	// stop meaning what this test assumes it means).
	busyErr := &serial.PortError{}
	withFakeOpenPort(t, nil, busyErr)

	_, err := OpenSerial("/dev/cu.fake", SerialConfig{})
	if err == nil {
		t.Fatal("OpenSerial: want error, got nil")
	}
	if !errors.Is(err, busyErr) {
		t.Errorf("wrapped error does not wrap the original busy error: %v", err)
	}
	got := err.Error()
	if !containsAll(got, "busy", "WSJT-X", "flrig") {
		t.Errorf("wrapped error = %q, want it to mention port contention and likely culprits", got)
	}
}

func TestPortBusyIsZeroValue(t *testing.T) {
	if serial.PortBusy != 0 {
		t.Fatalf("serial.PortBusy = %d, want 0 (TestOpenSerial_WrapsPortBusyError depends on this)", serial.PortBusy)
	}
}

func TestOpenSerial_WrapsGenericOpenError_WithoutBusyWording(t *testing.T) {
	plain := errors.New("some other failure")
	withFakeOpenPort(t, nil, plain)

	_, err := OpenSerial("/dev/cu.fake", SerialConfig{})
	if err == nil {
		t.Fatal("OpenSerial: want error, got nil")
	}
	if !errors.Is(err, plain) {
		t.Errorf("wrapped error does not wrap the original error: %v", err)
	}
	if containsAll(err.Error(), "busy") {
		t.Errorf("wrapped error = %q, should not claim the port is busy for an unrelated error", err.Error())
	}
}

func containsAll(s string, subs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if !strings.Contains(lower, strings.ToLower(sub)) {
			return false
		}
	}
	return true
}

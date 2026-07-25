// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"fmt"
	"io"
	"time"

	"go.bug.st/serial"
)

// Port is anything the transaction engine can read from, write to, and
// close: a real serial connection opened by OpenSerial, or — in tests —
// fakeradio's Port().
type Port io.ReadWriteCloser

// DefaultBaud and DefaultStopBits are OpenSerial's defaults when
// SerialConfig's corresponding field is left at its zero value.
//
// PRE-HARDWARE ESTIMATE: 38400 8-N-2 is the FT-710 CAT reference's stated
// default for the CAT-1 (Enhanced/USB) interface; not yet confirmed against
// physical hardware — to be verified at M5a.
const (
	DefaultBaud     = 38400
	DefaultStopBits = 2
)

// pollInterval is the read timeout OpenSerial configures on the underlying
// serial.Port. It is NOT a protocol deadline — the engine layers its own
// answer/error-window deadlines on top of raw reads — it only bounds how
// long a single Read() call can block, so a goroutine reading the port
// notices a Close() (or, on real hardware, simply returns control
// periodically) promptly instead of blocking indefinitely inside the OS
// call. PRE-HARDWARE ESTIMATE — polling granularity, not a protocol timing
// value; to be reviewed at M5a alongside the other defaults.
const pollInterval = 100 * time.Millisecond

// SerialConfig configures OpenSerial. The zero value selects DefaultBaud
// (38400) and DefaultStopBits (2); data bits (8) and parity (none) are
// fixed by the protocol reference and are not configurable.
type SerialConfig struct {
	// Baud is the serial bit rate. <= 0 selects DefaultBaud.
	Baud int
	// StopBits is 1 or 2. <= 0 selects DefaultStopBits (2). Any other
	// value is a configuration error.
	StopBits int
}

// resolveConfig fills zero-valued SerialConfig fields with their documented
// defaults. Pure, so OpenSerial's default-selection logic is testable
// independently of any actual port.
func resolveConfig(cfg SerialConfig) SerialConfig {
	if cfg.Baud <= 0 {
		cfg.Baud = DefaultBaud
	}
	if cfg.StopBits <= 0 {
		cfg.StopBits = DefaultStopBits
	}
	return cfg
}

// stopBitsMode maps SerialConfig's plain integer StopBits (1 or 2) to
// go.bug.st/serial's StopBits enum, rejecting anything else — the wire
// protocol reference only ever documents 1 or 2 stop bits, and
// OnePointFiveStopBits is not a meaningful serial-port setting for this
// radio.
func stopBitsMode(n int) (serial.StopBits, error) {
	switch n {
	case 1:
		return serial.OneStopBit, nil
	case 2:
		return serial.TwoStopBits, nil
	default:
		return 0, fmt.Errorf("transport: unsupported StopBits %d (want 1 or 2)", n)
	}
}

// serialPort is the minimal subset of go.bug.st/serial.Port's method set
// that OpenSerial needs. Defining this narrow interface — rather than using
// serial.Port directly as openPort's return type — keeps the test seam
// small: a test's fake need only implement these six methods, not
// serial.Port's full surface (SetMode, Drain, ResetInputBuffer,
// ResetOutputBuffer, GetModemStatusBits, Break). go.bug.st/serial.Port
// satisfies serialPort structurally, with no adapter needed.
type serialPort interface {
	io.ReadWriteCloser
	SetRTS(rts bool) error
	SetDTR(dtr bool) error
	SetReadTimeout(t time.Duration) error
}

// openPort is OpenSerial's test seam: production code always leaves this at
// its default (a thin call to serial.Open), and OpenSerial calls it instead
// of serial.Open directly. Tests substitute a fake here to exercise config
// mapping and error wrapping without a real serial device — go.bug.st/serial
// cannot be driven against a fake port in CI, so this is the ONLY way to
// unit-test OpenSerial's own logic (see port_test.go). This is deliberately
// a package-level variable, not an injected dependency on Engine or a
// broader abstraction: OpenSerial is the one caller, and no other seam is
// needed anywhere else in this package.
var openPort = func(path string, mode *serial.Mode) (serialPort, error) {
	return serial.Open(path, mode)
}

// OpenSerial opens path as an 8-data-bit, no-parity serial connection at
// cfg.Baud (default DefaultBaud) with cfg.StopBits (default
// DefaultStopBits) stop bits.
//
// Immediately after opening — safety obligation 4 — it drives RTS and DTR
// low (SetRTS(false), SetDTR(false)) before returning the port to the
// caller. go.bug.st/serial does not expose a way to request these lines'
// state be low AT THE INSTANT the OS opens the underlying device node
// (Mode.InitialStatusBits only controls what it sets them to once open
// completes); whatever the hardware UART/USB-serial bridge drives them to
// by default in the brief window between the OS-level open and these two
// calls landing is unknown and unverified — this is an M5a hardware
// observation item, not something this package can control or test.
//
// It then configures a short internal read poll interval (see
// pollInterval) — NOT a protocol deadline; the engine layers its own
// request/answer deadlines on top of raw reads from the returned Port.
//
// If the underlying open fails because the OS reports the port already in
// use or inaccessible, the returned error is wrapped with a hint toward
// likely contention culprits (see wrapOpenError) while still satisfying
// errors.Is/errors.As against the original error.
func OpenSerial(path string, cfg SerialConfig) (Port, error) {
	cfg = resolveConfig(cfg)
	sb, err := stopBitsMode(cfg.StopBits)
	if err != nil {
		return nil, err
	}

	mode := &serial.Mode{
		BaudRate: cfg.Baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: sb,
	}

	p, err := openPort(path, mode)
	if err != nil {
		return nil, wrapOpenError(path, err)
	}

	if err := p.SetRTS(false); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("transport: open %s: SetRTS(false): %w", path, err)
	}
	if err := p.SetDTR(false); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("transport: open %s: SetDTR(false): %w", path, err)
	}
	if err := p.SetReadTimeout(pollInterval); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("transport: open %s: SetReadTimeout: %w", path, err)
	}

	return p, nil
}

// wrapOpenError wraps err (as returned by openPort) with additional context
// for a human reading the error. When err is a *serial.PortError whose Code
// indicates the port is busy or access was denied — the two OS conditions
// that almost always mean another application already has the port open —
// the wrapped message names likely culprits (WSJT-X, flrig: the common
// amateur-radio programs that hold a CAT serial port open). The original
// error is always preserved for errors.Is/errors.As.
func wrapOpenError(path string, err error) error {
	var portErr *serial.PortError
	if errors.As(err, &portErr) {
		switch portErr.Code() {
		case serial.PortBusy, serial.PermissionDenied:
			return fmt.Errorf("transport: open %s: port busy — is WSJT-X/flrig using it? (%w)", path, err)
		}
	}
	return fmt.Errorf("transport: open %s: %w", path, err)
}

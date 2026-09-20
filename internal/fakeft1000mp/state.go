// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft1000mp

import (
	"io"
	"sync"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// recordLen is the 16-byte VFO/Memory Data Record width (matrix §1.2).
const recordLen = 16

// numMemories is the 99 MEM + 9 P + 5 QMB slots the family's own opcode
// chart ranges over (matrix §4 Banks, "113 total").
const numMemories = 99 + 9 + 5

// numFixedRecords is the current-op, VFO-A and VFO-B records that precede
// the 113 memories in a full dump (matrix §1.6).
const numFixedRecords = 3

// fullDumpLen is the documented total: 6 Status Flag bytes, 1 current-
// channel byte, then (3+113) 16-byte records — 1,863 bytes (matrix §1.9).
const fullDumpLen = 6 + 1 + (numFixedRecords+numMemories)*recordLen

// modeNames is the 12-value mode legend the 0CH SetMode opcode argument
// carries (matrix §1.5) — the domain handleSetMode enforces on a write.
// The 16-byte record's own byte 7 does NOT carry this value directly —
// see modeFamilyIndex below.
var modeNames = map[byte]string{
	0x00: "LSB", 0x01: "USB", 0x02: "CW", 0x03: "CW-R",
	0x04: "AM", 0x05: "AM-SYNC", 0x06: "FM", 0x07: "FM-W",
	0x08: "RTTY-L", 0x09: "RTTY-U", 0x0A: "PKT-L", 0x0B: "PKT-F",
}

// modeFamilyIndex compresses a 0CH opcode mode code (0x00-0x0B) into the
// 16-byte record's own 3-bit Operating Mode family code (matrix §1.5,
// ft1000mpmarkv_manual layout:4929-4946, printed p.90-91's "Operating
// Mode Byte (7)": "Bits5-7 Mode Data (3-bit code): LSB=000 USB=001 CW=010
// AM=011 FM=100 RTTY=101 PKT=110"). LSB and USB are unpaired singles;
// every later adjacent pair (CW/CW-R, AM/AM-SYNC, FM/FM-W, RTTY-L/RTTY-U,
// PKT-L/PKT-F) shares one family — reimplemented independently here, per
// THE HARD RULE, from the same manual passage core/driver/ft1000mp's own
// caps.go cites for its (separately derived) recordModeBase table.
func modeFamilyIndex(code byte) byte {
	if code < 2 {
		return code
	}
	return (code-2)/2 + 2
}

// Radio is a simulated FT-1000MP/Mark-V: an in-memory duplex pipe
// presenting the host end via Port(), serviced from the Radio's own
// goroutine using this package's independent parser (parser.go). Safe for
// concurrent use (run tests with -race) other than Port()'s connection
// itself, which only the serving goroutine ever touches.
type Radio struct {
	pipe *fakepipe.Pipe

	mu sync.Mutex

	// The one live VFO register this fake models (doc.go's ASSUMED
	// register #3: no A/B select, so there is only ever one). freq is in
	// tens of Hz — bincat.DecodeBCD's own unit, matching the write side's
	// worked example (14.25000 MHz = 1,425,000).
	vfoFreqTensOfHz uint64
	vfoMode         byte
	vfoFlags        byte // VFO/Memory Operating Flags byte (shift bits only)

	// memories[i] is channel i+1's stored 16-byte record (matrix §1.4's
	// 1-based override): memories[0] is channel 1, memories[112] is QMB5.
	// Zero until a Store/Enter targets it.
	memories [numMemories][recordLen]byte
}

// Option configures a *Radio at construction time. See New.
type Option func(*Radio)

// New constructs a *Radio and starts its servicing goroutine.
func New(opts ...Option) *Radio {
	r := &Radio{pipe: fakepipe.New()}
	for _, opt := range opts {
		opt(r)
	}
	r.serve()
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
func (r *Radio) Port() io.ReadWriteCloser { return r.pipe.Host() }

// Close shuts the fake radio down — see fakeft950.Radio.Close's identical
// contract (safe more than once, leaves the host end open for io.EOF).
func (r *Radio) Close() error { return r.pipe.Close() }

// serve starts the Radio's own goroutine: the only one that ever reads or
// writes the pipe, so no synchronisation is needed around the connection
// itself.
func (r *Radio) serve() {
	acc := &reassembler{}
	r.pipe.Go(func() {
		r.pipe.ReadLoop(func(b []byte) {
			for _, frame := range acc.push(b) {
				r.handleEvent(frame)
			}
		})
	})
}

// handleEvent processes one complete 5-byte frame and sends whatever
// reply handleFrame produces. nil is silence — this family's own
// no-reply convention for every write and every unhandled opcode (matrix
// §Context: an illegal or unrecognised command makes the radio "do
// nothing" — there is no NAK anywhere in this protocol).
func (r *Radio) handleEvent(frame [frameLen]byte) {
	if reply := r.handleFrame(frame); reply != nil {
		r.rawWrite(reply)
	}
}

// rawWrite sends data to the port, honouring the configured per-reply
// latency (WithLatency). Errors are not reported — a write failing
// because the peer has gone away is an expected outcome, not a bug here.
func (r *Radio) rawWrite(data []byte) {
	r.pipe.Write(data)
}

// vfoRecordBytesLocked renders the live VFO register as a 16-byte record.
// Caller must hold r.mu.
func (r *Radio) vfoRecordBytesLocked() [recordLen]byte {
	var rec [recordLen]byte
	fb := freqToRecordBytes(r.vfoFreqTensOfHz)
	copy(rec[1:5], fb[:])
	rec[7] = modeFamilyIndex(r.vfoMode) << 5
	rec[9] = r.vfoFlags
	return rec
}

// buildFullDump renders the whole 1,863-byte Status Update (U=00H) reply:
// 6 Status Flag bytes (doc.go ASSUMED #1, always zero), the current-
// channel byte (ASSUMED #2, always 00H — Recall Memory is unwired), then
// the current-op/VFO-A/VFO-B triple (ASSUMED #3: VFO-B and "current op"
// are unmodelled, VFO-A is the one live register) and the 113 memory
// records in fixed channel order.
func (r *Radio) buildFullDump() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]byte, 0, fullDumpLen)
	out = append(out, make([]byte, 6)...) // Status Flags
	out = append(out, 0x00)               // current-channel byte

	vfo := r.vfoRecordBytesLocked()
	out = append(out, vfo[:]...)                  // current op (mirrors VFO-A)
	out = append(out, vfo[:]...)                  // VFO-A
	out = append(out, make([]byte, recordLen)...) // VFO-B (unmodelled)

	for i := range r.memories {
		out = append(out, r.memories[i][:]...)
	}
	return out
}

// freqToRecordBytes converts a tens-of-Hz value into the 16-byte record's
// own read-side encoding — see doc.go's frequency section. It is the
// digit-reversal of the write-side BCD digit string, packed two digits
// per byte, MSB nibble first; freqToRecordBytes_test.go pins the manual's
// own worked example (1,425,000 -> 00 05 24 10) directly.
func freqToRecordBytes(v uint64) [4]byte {
	var digits [8]byte // digits[0] most significant, digits[7] least
	x := v
	for i := 7; i >= 0; i-- {
		digits[i] = byte(x % 10)
		x /= 10
	}
	var out [4]byte
	for i := 0; i < 4; i++ {
		hi := digits[7-2*i]
		lo := digits[6-2*i]
		out[i] = hi<<4 | lo
	}
	return out
}

package fakeic7760

import (
	"fmt"
	"strconv"
)

const (
	AddrRadio      byte = 0xB2
	AddrController byte = 0xE0
	AddrBroadcast  byte = 0
	CodeOK         byte = 0xFB
	CodeNG         byte = 0xFA
	RecordLen           = 25
	NameLen             = 10
	NamePad        byte = 0x20
	ChanP1              = -1
	ChanP2              = -2
)

// RecordLen is the B/W-derived 27-byte printed span minus its two selector
// bytes; TestRecordGeometryAndSelectors pins that arithmetic for this fake.

type MemState struct{ Raw []byte }

func (m MemState) clone() MemState { return MemState{Raw: append([]byte(nil), m.Raw...)} }
func selectorFor(ch int) (byte, byte, bool) {
	if ch == ChanP1 {
		return 1, 0, true
	}
	if ch == ChanP2 {
		return 1, 1, true
	}
	if ch < 1 || ch > 99 {
		return 0, 0, false
	}
	return 0, byte(ch/10<<4 | ch%10), true
}
func channelFor(hi, lo byte) (int, bool) {
	if hi == 1 && (lo == 0 || lo == 1) {
		if lo == 0 {
			return ChanP1, true
		}
		return ChanP2, true
	}
	if hi != 0 || lo>>4 > 9 || lo&15 > 9 {
		return 0, false
	}
	n := int(lo>>4)*10 + int(lo&15)
	return n, n >= 1 && n <= 99
}
func parseSlot(s string) (int, bool) {
	if s == "P1" {
		return ChanP1, true
	}
	if s == "P2" {
		return ChanP2, true
	}
	if len(s) != 3 {
		return 0, false
	}
	n, e := strconv.Atoi(s)
	return n, e == nil && n >= 1 && n <= 99
}
func (r *Radio) SetSlot(ch int, m MemState) {
	if _, _, ok := selectorFor(ch); !ok {
		panic(fmt.Sprintf("fakeic7760: channel %d is not addressable", ch))
	}
	if len(m.Raw) != r.recordLen {
		panic(fmt.Sprintf("fakeic7760: channel %d record has length %d, want %d", ch, len(m.Raw), r.recordLen))
	}
	r.mu.Lock()
	r.slots[ch] = m.clone()
	r.mu.Unlock()
}
func (r *Radio) SlotState(ch int) (MemState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.slots[ch]
	return m.clone(), ok
}
func (r *Radio) ClearSlot(ch int)             { r.mu.Lock(); delete(r.slots, ch); r.mu.Unlock() }
func (r *Radio) Record(ch int) ([]byte, bool) { m, ok := r.SlotState(ch); return m.Raw, ok }
func (r *Radio) BytesWritten() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.bytesWritten...)
}
func (r *Radio) CommandLog() [][2]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][2]byte(nil), r.commands...)
}

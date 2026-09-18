// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftx1

import (
	"bytes"
	"testing"
)

// wireExchange writes req to r's port and returns whatever comes back.
func wireExchange(t *testing.T, r *Radio, req string) []byte {
	t.Helper()
	port := r.Port()
	if _, err := port.Write([]byte(req)); err != nil {
		t.Fatalf("Write %q: %v", req, err)
	}
	buf := make([]byte, 64)
	n, err := port.Read(buf)
	if err != nil {
		t.Fatalf("Read after %q: %v", req, err)
	}
	return buf[:n]
}

// --- Framing ---

func TestReassembler_SplitAcrossWrites(t *testing.T) {
	a := newReassembler(256)
	if evs := a.push([]byte("I")); len(evs) != 0 {
		t.Fatalf("push(%q) produced %d events, want 0", "I", len(evs))
	}
	evs := a.push([]byte("D;"))
	if len(evs) != 1 || string(evs[0].frame) != "ID;" {
		t.Fatalf("push(%q) = %+v, want one frame \"ID;\"", "D;", evs)
	}
}

func TestReassembler_OverflowThenResync(t *testing.T) {
	a := newReassembler(4)
	evs := a.push([]byte("TOOLONG"))
	if len(evs) != 1 || !evs[0].overflow {
		t.Fatalf("push(overlong) = %+v, want one overflow event", evs)
	}
	evs = a.push([]byte("garbage;ID;"))
	if len(evs) != 1 || string(evs[0].frame) != "ID;" {
		t.Fatalf("push after overflow = %+v, want one frame \"ID;\"", evs)
	}
}

// --- Dispatch: unknown/unmodelled commands ---

func TestHandleFrame_UnknownCommandRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "ZZ;"); !bytes.Equal(got, rejection) {
		t.Errorf("ZZ; = %q, want %q", got, rejection)
	}
}

func TestHandleFrame_CommandNameCaseInsensitive(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "id;"); !bytes.Equal(got, []byte("ID0840;")) {
		t.Errorf("id; = %q, want %q", got, "ID0840;")
	}
}

// TestMCRefuses covers doc.go's "MC IS NOT MODELLED AT ALL" — every MC
// frame shape falls through to the unknown-command default.
func TestMCRefuses(t *testing.T) {
	r := New()
	defer r.Close()
	for _, req := range []string{"MC;", "MC00001;", "MC000010842;"} {
		if got := wireExchange(t, r, req); !bytes.Equal(got, rejection) {
			t.Errorf("%s = %q, want %q (MC is not modelled)", req, got, rejection)
		}
	}
}

// TestVMGTEXRefuse covers the plan's own scoping: VM, GT and EX are all
// out of scope this milestone and fall through the same default.
func TestVMGTEXRefuse(t *testing.T) {
	r := New()
	defer r.Close()
	for _, req := range []string{"VM;", "GT;", "EX0100000;"} {
		if got := wireExchange(t, r, req); !bytes.Equal(got, rejection) {
			t.Errorf("%s = %q, want %q", req, got, rejection)
		}
	}
}

// --- Address-domain grammar ---

func TestClassifySlot(t *testing.T) {
	tests := []struct {
		addr string
		want slotKind
	}{
		{"00001", slotMemory},
		{"00999", slotMemory},
		{"00000", slotInvalid}, // the "VFO or MT or QMB" none-form, unmodelled
		{"P-01L", slotPMS},
		{"P-50U", slotPMS},
		{"P-51L", slotInvalid}, // one past the 50-pair ceiling
		{"P-00L", slotInvalid}, // pair 0 does not exist
		{"50001", slotFiveMHz},
		{"50020", slotFiveMHz},
		{"50021", slotInvalid}, // one past the 5 MHz band's own ceiling
		{"EMGCH", slotEMG},
		{"01000", slotInvalid}, // one past the memory ceiling
		{"0AB01", slotInvalid}, // not digits, not a recognised token form
		{"0001", slotInvalid},  // 4 bytes, not 5
	}
	for _, tt := range tests {
		if got := classifySlot(tt.addr); got != tt.want {
			t.Errorf("classifySlot(%q) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

// TestMR_AddressDomainRejection pins the three address-domain rejections
// the milestone brief names by name: pair 51, 01000, 50021 — each a
// grammatically-close but out-of-domain address, checked over the wire
// via MR (which answers "?;" identically for "invalid" and "unpopulated",
// doc.go's register entry EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS).
func TestMR_AddressDomainRejection(t *testing.T) {
	r := New()
	defer r.Close()
	for _, addr := range []string{"P-51L", "01000", "50021"} {
		if got := wireExchange(t, r, "MR"+addr+";"); !bytes.Equal(got, rejection) {
			t.Errorf("MR%s; = %q, want %q", addr, got, rejection)
		}
	}
}

func TestMW_AddressDomainRejection(t *testing.T) {
	r := New()
	defer r.Close()
	block := memBlockBytes("P-51L", "007000000", '+', "0000", false, false, modeUSB, kindPMS, '0', '0')
	if got := wireExchange(t, r, "MW"+string(block)+";"); !bytes.Equal(got, rejection) {
		t.Errorf("MW P-51L; = %q, want %q", got, rejection)
	}
	if got := wireExchange(t, r, "MRP-51L;"); !bytes.Equal(got, rejection) {
		t.Errorf("a refused MW must not have created the slot: MRP-51L; = %q, want %q", got, rejection)
	}
}

// --- MR/MW round trip, one address per bank ---

func memBlockBytes(addr, freq string, clarSign byte, clarMag string, rx, tx bool, mode, kind, tone, shift byte) []byte {
	var b []byte
	b = append(b, addr...)
	b = append(b, freq...)
	b = append(b, clarSign)
	b = append(b, clarMag...)
	b = append(b, boolFlagByte(rx))
	b = append(b, boolFlagByte(tx))
	b = append(b, mode)
	b = append(b, kind)
	b = append(b, tone)
	b = append(b, p9Fixed...)
	b = append(b, shift)
	return b
}

func TestMW_ThenMR_RoundTripsPerBank(t *testing.T) {
	r := New()
	defer r.Close()

	tests := []struct {
		name string
		addr string
		kind byte
	}{
		{"memory", "00050", kindMemory},
		{"PMS", "P-02L", kindPMS},
		{"5 MHz", "50010", kindMemory},
		{"EMGCH", "EMGCH", kindMemory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := memBlockBytes(tt.addr, "007123456", '+', "0000", false, true, modeUSB, tt.kind, '2', '1')
			frame := append(append([]byte("MW"), body...), ';')
			port := r.Port()
			if _, err := port.Write(frame); err != nil {
				t.Fatalf("Write MW: %v", err)
			}
			got := wireExchange(t, r, "MR"+tt.addr+";")
			want := append(append([]byte("MR"), body...), ';')
			if !bytes.Equal(got, want) {
				t.Errorf("MR%s; after MW = %q, want %q", tt.addr, got, want)
			}
		})
	}
}

func TestMW_RefusesNonFixedP9(t *testing.T) {
	r := New()
	defer r.Close()
	body := memBlockBytes("00051", "007000000", '+', "0000", false, false, modeUSB, kindMemory, '0', '0')
	// Corrupt the P9 field (offsets 24-25) away from the fixed "00".
	body[blkP9Start] = '9'
	frame := append(append([]byte("MW"), body...), ';')
	if got := wireExchange(t, r, string(frame)); !bytes.Equal(got, rejection) {
		t.Errorf("MW with non-fixed P9 = %q, want %q", got, rejection)
	}
	if got := wireExchange(t, r, "MR00051;"); !bytes.Equal(got, rejection) {
		t.Errorf("MR00051; after a refused MW = %q, want %q (slot must stay empty)", got, rejection)
	}
}

func TestMR_EmptySlotRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MR00500;"); !bytes.Equal(got, rejection) {
		t.Errorf("MR00500; (unpopulated) = %q, want %q", got, rejection)
	}
}

// --- MT round trip ---

func TestMT_SetAnswersWithAnEcho(t *testing.T) {
	r := New()
	defer r.Close()
	tag := padTag("TEST TAG")
	frame := "MT00060" + tag + ";"
	got := wireExchange(t, r, frame)
	want := "MT00060" + tag + ";"
	if string(got) != want {
		t.Errorf("MT set = %q, want the echoed answer %q", got, want)
	}
	// And a subsequent Read returns the same tag.
	if got := wireExchange(t, r, "MT00060;"); string(got) != want {
		t.Errorf("MT00060; after the set = %q, want %q", got, want)
	}
}

func TestMT_ReadOfUntaggedAddressRejected(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MT00061;"); !bytes.Equal(got, rejection) {
		t.Errorf("MT00061; (never tagged) = %q, want %q", got, rejection)
	}
}

func TestMT_SetDoesNotTouchMWState_AndViceVersa(t *testing.T) {
	r := New()
	defer r.Close()

	// MT-set an address MW has never touched: MR must still see it as
	// empty (doc.go's register entry MW AND MT MUTATE INDEPENDENT
	// FIELDS).
	tag := padTag("ONLY TAG")
	wireExchange(t, r, "MT00062"+tag+";")
	if got := wireExchange(t, r, "MR00062;"); !bytes.Equal(got, rejection) {
		t.Errorf("MR00062; after an MT-only set = %q, want %q", got, rejection)
	}

	// MW-set an address MT has never touched: an MT read must still see
	// it as untagged. MW is fire-and-forget (no reply, doc.go's register
	// entry MT'S SET ANSWERS WITH AN ECHO — MW's own converse), so this
	// writes directly rather than through wireExchange, which always
	// waits for a reply.
	body := memBlockBytes("00063", "007000000", '+', "0000", false, false, modeUSB, kindMemory, '0', '0')
	port := r.Port()
	if _, err := port.Write(append([]byte("MW"+string(body)), ';')); err != nil {
		t.Fatalf("Write MW: %v", err)
	}
	if got := wireExchange(t, r, "MT00063;"); !bytes.Equal(got, rejection) {
		t.Errorf("MT00063; after an MW-only set = %q, want %q", got, rejection)
	}
}

func TestMT_RejectsWrongLength(t *testing.T) {
	r := New()
	defer r.Close()
	if got := wireExchange(t, r, "MT0006;"); !bytes.Equal(got, rejection) {
		t.Errorf("MT0006; (4-byte addr) = %q, want %q", got, rejection)
	}
}

func TestValidTag_RejectsSemicolonAndControlBytesAndWrongLength(t *testing.T) {
	if validTag([]byte("short")) {
		t.Error("validTag accepted a tag shorter than 12 bytes")
	}
	if validTag([]byte("has;semicolon")) {
		t.Error("validTag accepted a 13-byte tag anyway")
	}
	injected := append([]byte("HOME"), ';', 'I', 'D', ';', ' ', ' ', ' ', ' ')
	if len(injected) != tagWireLen {
		t.Fatalf("test fixture is %d bytes, want %d", len(injected), tagWireLen)
	}
	if validTag(injected) {
		t.Error("validTag accepted a tag containing ';' — command injection must be refused")
	}
	control := []byte("HOME\x00     ")
	if validTag(control) {
		t.Error("validTag accepted a tag containing a control byte")
	}
	if !validTag([]byte("PLAIN TAG   ")) {
		t.Error("validTag refused an ordinary 12-byte printable-ASCII tag")
	}
}

// --- Field validators ---

func TestValidModeByte(t *testing.T) {
	for _, m := range []byte("0123456789ABCDEFHI") {
		if !validModeByte(m) {
			t.Errorf("validModeByte(%q) = false, want true", m)
		}
	}
	for _, m := range []byte("GJ") {
		if validModeByte(m) {
			t.Errorf("validModeByte(%q) = true, want false — G/J are ASSUMED reserved (spec.md §5)", m)
		}
	}
}

func TestValidKindByte_And_ValidToneByte(t *testing.T) {
	for _, b := range []byte("012345") {
		if !validKindByte(b) {
			t.Errorf("validKindByte(%q) = false, want true", b)
		}
		if !validToneByte(b) {
			t.Errorf("validToneByte(%q) = false, want true", b)
		}
	}
	if validKindByte('6') {
		t.Error("validKindByte('6') = true, want false — only 0-5 are printed")
	}
	if validToneByte('6') {
		t.Error("validToneByte('6') = true, want false — only 0-5 are printed")
	}
}

func TestPMSWire(t *testing.T) {
	tests := []struct {
		pair int
		half byte
		want string
	}{
		{1, 'L', "P-01L"},
		{1, 'U', "P-01U"},
		{50, 'L', "P-50L"},
		{50, 'U', "P-50U"},
	}
	for _, tt := range tests {
		if got := pmsWire(tt.pair, tt.half); got != tt.want {
			t.Errorf("pmsWire(%d, %q) = %q, want %q", tt.pair, tt.half, got, tt.want)
		}
	}
}

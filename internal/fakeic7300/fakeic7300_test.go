// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

// --- Helpers ---

// newRadio builds a radio, registers its Close and returns it. Every test uses
// it so that no test can leave a servicing goroutine behind.
func newRadio(t *testing.T, opts ...Option) *Radio {
	t.Helper()
	r := New(opts...)
	t.Cleanup(func() {
		if err := r.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return r
}

// ctrlFrame builds a frame FROM the controller: FE FE <to> E0 <body> FD.
func ctrlFrame(to byte, body ...byte) []byte {
	return buildFrame(to, controllerAddress, body)
}

// radioFrame builds a frame the radio would send: FE FE E0 <from> <body> FD.
func radioFrame(from byte, body ...byte) []byte {
	return buildFrame(controllerAddress, from, body)
}

func okFrame(from byte) []byte { return radioFrame(from, codeOK) }
func ngFrame(from byte) []byte { return radioFrame(from, codeNG) }

func writeBytes(t *testing.T, r *Radio, b []byte) {
	t.Helper()
	if err := r.hostConn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetWriteDeadline: %v", err)
	}
	if _, err := r.hostConn.Write(b); err != nil {
		t.Fatalf("writing %X: %v", b, err)
	}
}

// readFrame reads until one complete frame has arrived from the radio.
func readFrame(t *testing.T, r *Radio) []byte {
	t.Helper()
	if err := r.hostConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	f := newFramer(maxFrameBytes)
	buf := make([]byte, 512)
	for {
		n, err := r.hostConn.Read(buf)
		if n > 0 {
			if frames := f.push(buf[:n]); len(frames) > 0 {
				return frames[0]
			}
		}
		if err != nil {
			t.Fatalf("reading a frame: %v", err)
			return nil
		}
	}
}

// exchange writes one frame and reads the one answer it draws.
func exchange(t *testing.T, r *Radio, req []byte) []byte {
	t.Helper()
	writeBytes(t, r, req)
	return readFrame(t, r)
}

// drain reads frames from the host end in its own goroutine, so that a test
// driving a radio which is also emitting unsolicited traffic cannot deadlock:
// net.Pipe is synchronous, so a host that blocks in Write while the radio is
// blocked in Write would wedge, exactly as a real reader that stops reading
// would. Frames are dropped once the channel is full.
func drain(r *Radio) <-chan []byte {
	out := make(chan []byte, 4096)
	go func() {
		defer close(out)
		f := newFramer(maxFrameBytes)
		buf := make([]byte, 512)
		for {
			n, err := r.hostConn.Read(buf)
			for _, frame := range f.push(buf[:n]) {
				select {
				case out <- frame:
				default:
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return out
}

// validRecord is a record this radio accepts: every nibble-coded field inside
// its printed vocabulary and every name byte inside the character span.
func validRecord() []byte {
	rec := make([]byte, recordLen)
	rec[offSplitSelect] = 0x10 // ③ high 1 = Split ON, low 0 = Select memory OFF
	copy(rec[offFrequency:], []byte{0x00, 0x40, 0x07, 0x14, 0x00})
	copy(rec[offMode:], []byte{0x01, 0x02})
	rec[offDataTone] = 0x01 // ⑪ high 0 = Data mode OFF, low 1 = TONE
	copy(rec[offRepeaterTone:], []byte{0x00, 0x08, 0x85})
	copy(rec[offToneSquelch:], []byte{0x00, 0x08, 0x85})
	// ❹–⓱ mirrors ④–⑰, as the NOTE recommends — nothing in this fake checks
	// that it does.
	copy(rec[offRepeatBlock:], rec[offFrequency:offRepeatBlock])
	copy(rec[offName:], []byte("TEST      "))
	return rec
}

func readReq(hi, lo byte) []byte {
	return ctrlFrame(defaultRadioAddress, cmdMemory, subMemoryContent, hi, lo)
}

func setReq(hi, lo byte, rec []byte) []byte {
	return ctrlFrame(defaultRadioAddress, append([]byte{cmdMemory, subMemoryContent, hi, lo}, rec...)...)
}

// --- The record's derived shape ---

// TestRecordLayoutIsContiguousAndSumsTo39 pins the derivation in record.go
// against itself: the eight fields after the channel address, at the widths
// their printed index ranges give, tile the record with no gap and no overlap.
func TestRecordLayoutIsContiguousAndSumsTo39(t *testing.T) {
	fields := []struct {
		name  string
		off   int
		width int
	}{
		{"③ Split and Select", offSplitSelect, lenSplitSelect},
		{"④–⑧ Operating frequency", offFrequency, lenFrequency},
		{"⑨, ⑩ Operating mode", offMode, lenMode},
		{"⑪ Data mode and tone type", offDataTone, lenDataTone},
		{"⑫–⑭ Repeater tone frequency", offRepeaterTone, lenRepeaterTone},
		{"⑮–⑰ Tone squelch frequency", offToneSquelch, lenToneSquelch},
		{"❹–⓱ (no label printed)", offRepeatBlock, lenRepeatBlock},
		{"⑱–㉗ Memory name", offName, lenName},
	}
	next := 0
	for _, f := range fields {
		if f.off != next {
			t.Errorf("%s starts at %d, want %d — the fields must tile the record", f.name, f.off, next)
		}
		next = f.off + f.width
	}
	if next != recordLen {
		t.Errorf("the fields sum to %d bytes, want %d", next, recordLen)
	}
	if recordLen != 39 {
		t.Errorf("recordLen = %d, want 39 (1+5+2+1+3+3+14+10; 41 with the two channel-address bytes)", recordLen)
	}
	// The NOTE's claim, which fixes the repeat block's width independently:
	// "The same data as ④–⑰ are stored in ❹–⓱".
	mirrored := lenFrequency + lenMode + lenDataTone + lenRepeaterTone + lenToneSquelch
	if mirrored != lenRepeatBlock {
		t.Errorf("④–⑰ is %d bytes but ❹–⓱ is %d — the NOTE says one mirrors the other", mirrored, lenRepeatBlock)
	}
}

// TestValidateRecord covers the vocabulary rules field by field, including both
// sides of every boundary the two legends print.
func TestValidateRecord(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func([]byte)
		wantErr bool
	}{
		{"the valid record", func([]byte) {}, false},
		{"③ high nibble 1 = Split ON", func(r []byte) { r[offSplitSelect] = 0x10 }, false},
		{"③ high nibble 2 is not printed", func(r []byte) { r[offSplitSelect] = 0x20 }, true},
		{"③ low nibble 3 = ★3", func(r []byte) { r[offSplitSelect] = 0x03 }, false},
		{"③ low nibble 4 is not printed", func(r []byte) { r[offSplitSelect] = 0x04 }, true},
		{"③ both nibbles at their maxima", func(r []byte) { r[offSplitSelect] = 0x13 }, false},
		{"⑪ high nibble 1 = Data mode ON", func(r []byte) { r[offDataTone] = 0x10 }, false},
		{"⑪ high nibble 2 is not printed", func(r []byte) { r[offDataTone] = 0x20 }, true},
		{"⑪ low nibble 2 = TSQL", func(r []byte) { r[offDataTone] = 0x02 }, false},
		{"⑪ low nibble 3 is not printed", func(r []byte) { r[offDataTone] = 0x03 }, true},
		{"⑱–㉗ space, the pad", func(r []byte) { copy(r[offName:], "          ") }, false},
		{"⑱–㉗ tilde, the highest printed code", func(r []byte) { r[offName+9] = 0x7E }, false},
		{"⑱–㉗ 1F is below the span", func(r []byte) { r[offName] = 0x1F }, true},
		{"⑱–㉗ 7F is above the span", func(r []byte) { r[offName+4] = 0x7F }, true},
		{"⑱–㉗ FF is above the span", func(r []byte) { r[offName+9] = 0xFF }, true},
		{"④–⑧ is unstated: FF passes", func(r []byte) { r[offFrequency] = 0xFF }, false},
		{"⑨, ⑩ is unstated: FF passes", func(r []byte) { r[offMode+1] = 0xFF }, false},
		{"⑫–⑭ is unstated: FF passes", func(r []byte) { r[offRepeaterTone+2] = 0xFF }, false},
		{"⑮–⑰ is unstated: FF passes", func(r []byte) { r[offToneSquelch] = 0xFF }, false},
		{"❹–⓱ carries no printed vocabulary: FF passes", func(r []byte) { r[offRepeatBlock+7] = 0xFF }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := validRecord()
			tt.mutate(rec)
			err := validateRecord(rec)
			if tt.wantErr != (err != nil) {
				t.Fatalf("validateRecord = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRecordRejectsEveryOtherLength(t *testing.T) {
	for _, n := range []int{0, 1, 2, recordLen - 1, recordLen + 1, 41, 17} {
		rec := make([]byte, n)
		for i := range rec {
			rec[i] = ' ' // in every printed vocabulary this fake enforces
		}
		if err := validateRecord(rec); err == nil {
			t.Errorf("validateRecord accepted a %d-byte record; only %d is a record", n, recordLen)
		}
	}
}

// --- The channel address space ---

func TestChannelAddressSpaceIsTheThreePrintedForms(t *testing.T) {
	tests := []struct {
		hi, lo byte
		want   string
		ok     bool
	}{
		{0x00, 0x01, "001", true},
		{0x00, 0x09, "009", true},
		{0x00, 0x10, "010", true},
		{0x00, 0x99, "099", true},
		{0x01, 0x00, "P1", true},
		{0x01, 0x01, "P2", true},
		{0x00, 0x00, "", false}, // the legend's range opens at 01
		{0x00, 0x9A, "", false}, // not two BCD digits
		{0x00, 0xA0, "", false},
		{0x01, 0x02, "", false}, // there is no P3
		{0x02, 0x00, "", false}, // there is no third page
		{0xFF, 0xFF, "", false},
	}
	for _, tt := range tests {
		got, ok := canonicalSlot(tt.hi, tt.lo)
		if ok != tt.ok || got != tt.want {
			t.Errorf("canonicalSlot(%02X, %02X) = %q, %v; want %q, %v", tt.hi, tt.lo, got, ok, tt.want, tt.ok)
		}
		if !ok {
			continue
		}
		hi, lo, back := slotAddressBytes(got)
		if !back || hi != tt.hi || lo != tt.lo {
			t.Errorf("slotAddressBytes(%q) = %02X %02X, %v; want %02X %02X, true", got, hi, lo, back, tt.hi, tt.lo)
		}
	}
	for _, bad := range []string{"", "0", "00", "0001", "100", "000", "P0", "P3", "p1", "abc"} {
		if _, _, ok := slotAddressBytes(bad); ok {
			t.Errorf("slotAddressBytes(%q) accepted a slot outside the three printed forms", bad)
		}
	}
}

// --- The wire ---

func TestOKAndNGAreExactlySixBytes(t *testing.T) {
	r := newRadio(t)

	// 0B is not a command this fake models, so it draws the NG frame.
	got := exchange(t, r, ctrlFrame(defaultRadioAddress, 0x0B))
	want := []byte{0xFE, 0xFE, 0xE0, 0x94, 0xFA, 0xFD}
	if !bytes.Equal(got, want) {
		t.Fatalf("NG = % X, want % X", got, want)
	}

	got = exchange(t, r, setReq(0x00, 0x01, validRecord()))
	want = []byte{0xFE, 0xFE, 0xE0, 0x94, 0xFB, 0xFD}
	if !bytes.Equal(got, want) {
		t.Fatalf("OK = % X, want % X", got, want)
	}
}

func TestIdentityRead(t *testing.T) {
	t.Run("default token is one byte, the configured address", func(t *testing.T) {
		r := newRadio(t)
		got := exchange(t, r, ctrlFrame(defaultRadioAddress, cmdID, subID))
		want := radioFrame(defaultRadioAddress, cmdID, subID, defaultRadioAddress)
		if !bytes.Equal(got, want) {
			t.Fatalf("identity answer = % X, want % X", got, want)
		}
		if body := frameBody(got); len(body) < 5 {
			t.Fatalf("identity answer carries no data byte: % X", got)
		}
	})

	t.Run("WithIDToken fixes the value", func(t *testing.T) {
		r := newRadio(t, WithIDToken([]byte{0xAB, 0xCD}))
		got := exchange(t, r, ctrlFrame(defaultRadioAddress, cmdID, subID))
		want := radioFrame(defaultRadioAddress, cmdID, subID, 0xAB, 0xCD)
		if !bytes.Equal(got, want) {
			t.Fatalf("identity answer = % X, want % X", got, want)
		}
	})

	t.Run("WithIDToken copies its argument", func(t *testing.T) {
		token := []byte{0x11}
		r := newRadio(t, WithIDToken(token))
		token[0] = 0x22
		got := exchange(t, r, ctrlFrame(defaultRadioAddress, cmdID, subID))
		if want := radioFrame(defaultRadioAddress, cmdID, subID, 0x11); !bytes.Equal(got, want) {
			t.Fatalf("identity answer = % X, want % X — the option must copy", got, want)
		}
	})

	t.Run("19 with any other subcommand is refused", func(t *testing.T) {
		r := newRadio(t)
		for _, sc := range []byte{0x01, 0x02, 0xFF} {
			got := exchange(t, r, ctrlFrame(defaultRadioAddress, cmdID, sc))
			if want := ngFrame(defaultRadioAddress); !bytes.Equal(got, want) {
				t.Errorf("19 %02X answered % X, want NG", sc, got)
			}
		}
	})
}

func TestMemorySetThenReadRoundTrips(t *testing.T) {
	r := newRadio(t)
	rec := validRecord()

	for _, ch := range []struct {
		hi, lo byte
		slot   string
	}{
		{0x00, 0x01, "001"},
		{0x00, 0x99, "099"},
		{0x01, 0x00, "P1"},
		{0x01, 0x01, "P2"},
	} {
		if got := exchange(t, r, setReq(ch.hi, ch.lo, rec)); !bytes.Equal(got, okFrame(defaultRadioAddress)) {
			t.Fatalf("set %s answered % X, want OK", ch.slot, got)
		}
		stored, ok := r.Channel(ch.slot)
		if !ok || !bytes.Equal(stored, rec) {
			t.Fatalf("Channel(%q) = % X, %v; want the record just set", ch.slot, stored, ok)
		}

		got := exchange(t, r, readReq(ch.hi, ch.lo))
		want := radioFrame(defaultRadioAddress, append([]byte{cmdMemory, subMemoryContent, ch.hi, ch.lo}, rec...)...)
		if !bytes.Equal(got, want) {
			t.Fatalf("read %s answered % X, want % X", ch.slot, got, want)
		}
		// The record sits at a fixed offset and is exactly recordLen bytes.
		if body := frameBody(got); len(body) != 6+recordLen {
			t.Fatalf("read %s answered a %d-byte body, want %d (to, from, 1A, 00, two channel bytes, record)", ch.slot, len(body), 6+recordLen)
		}
	}
}

func TestReadOfAnUnwrittenChannelIsNG(t *testing.T) {
	r := newRadio(t, WithChannel("001", validRecord()))
	if got := exchange(t, r, readReq(0x00, 0x02)); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
		t.Fatalf("read of an unwritten channel answered % X, want NG", got)
	}
	if got := exchange(t, r, readReq(0x01, 0x00)); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
		t.Fatalf("read of an unwritten P1 answered % X, want NG", got)
	}
	if _, ok := r.Channel("002"); ok {
		t.Fatal("a read must not create a channel")
	}
}

func TestSetOfAWrongLengthRecordIsNGAndStoresNothing(t *testing.T) {
	r := newRadio(t)
	rec := validRecord()
	for _, n := range []int{0, 1, recordLen - 1, recordLen + 1} {
		wrong := make([]byte, n)
		for i := range wrong {
			if i < len(rec) {
				wrong[i] = rec[i]
			} else {
				wrong[i] = ' '
			}
		}
		got := exchange(t, r, setReq(0x00, 0x05, wrong))
		if !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
			t.Errorf("a %d-byte record answered % X, want NG", n, got)
		}
		if _, ok := r.Channel("005"); ok {
			t.Fatalf("a %d-byte record was stored", n)
		}
	}
	if got := exchange(t, r, setReq(0x00, 0x05, rec)); !bytes.Equal(got, okFrame(defaultRadioAddress)) {
		t.Fatalf("the %d-byte record answered % X, want OK", recordLen, got)
	}
}

// TestTheDocumentedClearFormsAreRefused is the fence this fake exists to hold:
// the page prints a clear procedure and this fake refuses it, because the
// software under test ships no erase path (doc.go, "Why the clear forms are
// refused").
func TestTheDocumentedClearFormsAreRefused(t *testing.T) {
	r := newRadio(t, WithChannel("001", validRecord()), WithChannel("P1", validRecord()))

	clears := [][]byte{
		// ①,②: memory channel; ③: "FF"; ④: None — the list as printed.
		setReq(0x00, 0x01, []byte{0xFF}),
		setReq(0x01, 0x00, []byte{0xFF}),
		// The same shape with the record's whole width filled with FF, which
		// is the other thing somebody reaching for an erase would try.
		setReq(0x00, 0x01, bytes.Repeat([]byte{0xFF}, recordLen)),
	}
	for _, req := range clears {
		if got := exchange(t, r, req); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
			t.Errorf("a clear form answered % X, want NG", got)
		}
	}

	// Nothing was cleared, and nothing acknowledged.
	for _, slot := range []string{"001", "P1"} {
		rec, ok := r.Channel(slot)
		if !ok || !bytes.Equal(rec, validRecord()) {
			t.Errorf("Channel(%q) = % X, %v after the clear attempts; want the seeded record intact", slot, rec, ok)
		}
	}
	for _, f := range r.Sent() {
		if bytes.Equal(f, okFrame(defaultRadioAddress)) {
			t.Fatal("a clear attempt was acknowledged with OK")
		}
	}
}

func TestSetOfARecordOutsideThePrintedVocabulariesIsNG(t *testing.T) {
	r := newRadio(t)
	for _, tt := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"③ high nibble", func(rec []byte) { rec[offSplitSelect] = 0x20 }},
		{"③ low nibble", func(rec []byte) { rec[offSplitSelect] = 0x04 }},
		{"⑪ high nibble", func(rec []byte) { rec[offDataTone] = 0x30 }},
		{"⑪ low nibble", func(rec []byte) { rec[offDataTone] = 0x0F }},
		{"⑱–㉗ name byte", func(rec []byte) { rec[offName+3] = 0x00 }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := validRecord()
			tt.mutate(rec)
			if got := exchange(t, r, setReq(0x00, 0x07, rec)); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
				t.Fatalf("answered % X, want NG", got)
			}
			if _, ok := r.Channel("007"); ok {
				t.Fatal("the record was stored")
			}
		})
	}
}

func TestSetOrReadOfAnAddressOutsideTheThreeFormsIsNG(t *testing.T) {
	r := newRadio(t)
	rec := validRecord()
	for _, ch := range [][2]byte{{0x00, 0x00}, {0x00, 0x9A}, {0x01, 0x02}, {0x02, 0x00}, {0xFF, 0xFF}} {
		if got := exchange(t, r, readReq(ch[0], ch[1])); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
			t.Errorf("read %02X %02X answered % X, want NG", ch[0], ch[1], got)
		}
		if got := exchange(t, r, setReq(ch[0], ch[1], rec)); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
			t.Errorf("set %02X %02X answered % X, want NG", ch[0], ch[1], got)
		}
	}
	if n := len(r.Channels()); n != 0 {
		t.Fatalf("%d channels stored, want 0", n)
	}
	// A 1A 00 frame with fewer than two channel-address bytes is refused too.
	for _, short := range [][]byte{{}, {0x00}} {
		req := ctrlFrame(defaultRadioAddress, append([]byte{cmdMemory, subMemoryContent}, short...)...)
		if got := exchange(t, r, req); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
			t.Errorf("a %d-byte data area answered % X, want NG", len(short), got)
		}
	}
}

func TestEverythingElseIsNG(t *testing.T) {
	r := newRadio(t)
	reqs := []struct {
		name string
		body []byte
	}{
		{"0B", []byte{0x0B}},
		{"18 00", []byte{0x18, 0x00}},
		{"18 01", []byte{0x18, 0x01}},
		{"1A 05", []byte{0x1A, 0x05}},
		{"1A 05 00 91", []byte{0x1A, 0x05, 0x00, 0x91}},
		{"1A 01", []byte{0x1A, 0x01}},
		{"1A 02", []byte{0x1A, 0x02}},
		{"03", []byte{0x03}},
		{"1C 00", []byte{0x1C, 0x00}},
	}
	for _, tt := range reqs {
		t.Run(tt.name, func(t *testing.T) {
			got := exchange(t, r, ctrlFrame(defaultRadioAddress, tt.body...))
			if !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
				t.Fatalf("answered % X, want NG", got)
			}
		})
	}
}

// --- Framing ---

func TestFramingToleratesNoiseAndExtraPreambles(t *testing.T) {
	r := newRadio(t)

	noisy := []byte{0x00, 0x11, 0x22, 0xFD, 0x7E}
	noisy = append(noisy, preamble, preamble, preamble, preamble)
	noisy = append(noisy, defaultRadioAddress, controllerAddress, cmdID, subID, endOfMessage)

	writeBytes(t, r, noisy)
	got := readFrame(t, r)
	if want := radioFrame(defaultRadioAddress, cmdID, subID, defaultRadioAddress); !bytes.Equal(got, want) {
		t.Fatalf("answer = % X, want % X", got, want)
	}

	// The transcript normalises the four preamble bytes to two, and the noise
	// before them is gone.
	rec := r.Received()
	if len(rec) != 1 {
		t.Fatalf("Received() has %d frames, want 1", len(rec))
	}
	want := ctrlFrame(defaultRadioAddress, cmdID, subID)
	if !bytes.Equal(rec[0], want) {
		t.Fatalf("Received()[0] = % X, want % X", rec[0], want)
	}
}

func TestFramerUnitBehaviour(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want [][]byte
	}{
		{
			"a plain frame",
			[]byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}},
		},
		{
			"leading noise is dropped",
			[]byte{0x01, 0x02, 0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}},
		},
		{
			"a single FE is not a preamble",
			[]byte{0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD},
			nil,
		},
		{
			"five preamble bytes normalise to two",
			[]byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}},
		},
		{
			"two frames back to back",
			[]byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD, 0xFE, 0xFE, 0x94, 0xE0, 0x0B, 0xFD},
			[][]byte{
				{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD},
				{0xFE, 0xFE, 0x94, 0xE0, 0x0B, 0xFD},
			},
		},
		{
			"an FE inside a body abandons the partial and starts a new run",
			[]byte{0xFE, 0xFE, 0x94, 0xE0, 0xFE, 0xFE, 0x94, 0xE0, 0x0B, 0xFD},
			[][]byte{{0xFE, 0xFE, 0x94, 0xE0, 0x0B, 0xFD}},
		},
		{
			"FE FE FD is a complete, bodyless frame",
			[]byte{0xFE, 0xFE, 0xFD},
			[][]byte{{0xFE, 0xFE, 0xFD}},
		},
		{
			"a terminator with no preamble is noise",
			[]byte{0xFD, 0xFD, 0xFD},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFramer(maxFrameBytes)
			got := f.push(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d frames (% X), want %d", len(got), got, len(tt.want))
			}
			for i := range got {
				if !bytes.Equal(got[i], tt.want[i]) {
					t.Errorf("frame %d = % X, want % X", i, got[i], tt.want[i])
				}
			}
		})
	}

	t.Run("a frame split across chunks is reassembled", func(t *testing.T) {
		f := newFramer(maxFrameBytes)
		whole := []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}
		var got [][]byte
		for _, b := range whole {
			got = append(got, f.push([]byte{b})...)
		}
		if len(got) != 1 || !bytes.Equal(got[0], whole) {
			t.Fatalf("byte-at-a-time framing gave % X, want one frame % X", got, whole)
		}
	})

	t.Run("an over-long body is dropped and framing resumes", func(t *testing.T) {
		f := newFramer(8)
		if frames := f.push(append([]byte{0xFE, 0xFE}, bytes.Repeat([]byte{0x42}, 40)...)); len(frames) != 0 {
			t.Fatalf("an over-long body produced %d frames, want 0", len(frames))
		}
		if frames := f.push([]byte{0xFD}); len(frames) != 0 {
			t.Fatalf("the dropped partial's terminator produced %d frames, want 0", len(frames))
		}
		whole := []byte{0xFE, 0xFE, 0x94, 0xE0, 0x0B, 0xFD}
		frames := f.push(whole)
		if len(frames) != 1 || !bytes.Equal(frames[0], whole) {
			t.Fatalf("framing did not resume: got % X", frames)
		}
	})

	t.Run("the returned frames do not alias the accumulator", func(t *testing.T) {
		f := newFramer(maxFrameBytes)
		frames := f.push([]byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD, 0xFE, 0xFE, 0x94, 0xE0, 0x0B, 0xFD})
		if len(frames) != 2 {
			t.Fatalf("got %d frames, want 2", len(frames))
		}
		if want := []byte{0xFE, 0xFE, 0x94, 0xE0, 0x19, 0x00, 0xFD}; !bytes.Equal(frames[0], want) {
			t.Fatalf("the first frame was overwritten by the second: % X", frames[0])
		}
	})
}

// --- Addressing ---

func TestFramesForAnotherAddressAreCountedAndIgnored(t *testing.T) {
	r := newRadio(t)

	for _, to := range []byte{0x88, 0x00, controllerAddress} {
		writeBytes(t, r, ctrlFrame(to, cmdID, subID))
	}
	// The radio must not have answered any of them, which is proved by the
	// next frame — one addressed to it — drawing the FIRST answer on the wire.
	got := exchange(t, r, ctrlFrame(defaultRadioAddress, cmdID, subID))
	if want := radioFrame(defaultRadioAddress, cmdID, subID, defaultRadioAddress); !bytes.Equal(got, want) {
		t.Fatalf("answer = % X, want % X", got, want)
	}
	if sent := r.Sent(); len(sent) != 1 {
		t.Fatalf("Sent() has %d frames, want 1 — only the addressed frame draws an answer", len(sent))
	}
	if rec := r.Received(); len(rec) != 4 {
		t.Fatalf("Received() has %d frames, want 4 — every complete frame is recorded, addressed or not", len(rec))
	}
	r.mu.Lock()
	ignored := r.ignored
	r.mu.Unlock()
	if ignored != 3 {
		t.Fatalf("ignored = %d, want 3", ignored)
	}
}

// TestWithRadioAddressMovesEveryAnswerByte is the property doc.go states in
// words: the radio's address is a configuration, and every byte of every answer
// that shows it follows.
func TestWithRadioAddressMovesEveryAnswerByte(t *testing.T) {
	const moved byte = 0x88
	r := newRadio(t, WithRadioAddress(moved), WithChannel("001", validRecord()))

	// The identity answer.
	got := exchange(t, r, ctrlFrame(moved, cmdID, subID))
	if want := radioFrame(moved, cmdID, subID, moved); !bytes.Equal(got, want) {
		t.Errorf("identity answer = % X, want % X", got, want)
	}
	// The OK frame.
	got = exchange(t, r, setReq2(moved, 0x00, 0x02, validRecord()))
	if want := okFrame(moved); !bytes.Equal(got, want) {
		t.Errorf("OK = % X, want % X", got, want)
	}
	// The NG frame.
	got = exchange(t, r, ctrlFrame(moved, 0x0B))
	if want := ngFrame(moved); !bytes.Equal(got, want) {
		t.Errorf("NG = % X, want % X", got, want)
	}
	// The record answer.
	got = exchange(t, r, ctrlFrame(moved, cmdMemory, subMemoryContent, 0x00, 0x01))
	want := radioFrame(moved, append([]byte{cmdMemory, subMemoryContent, 0x00, 0x01}, validRecord()...)...)
	if !bytes.Equal(got, want) {
		t.Errorf("record answer = % X, want % X", got, want)
	}

	// And the default address is now somebody else's: a frame to 94 is ignored.
	writeBytes(t, r, ctrlFrame(defaultRadioAddress, cmdID, subID))
	got = exchange(t, r, ctrlFrame(moved, cmdID, subID))
	if want := radioFrame(moved, cmdID, subID, moved); !bytes.Equal(got, want) {
		t.Fatalf("after a frame to 94, the next answer was % X, want % X", got, want)
	}
}

// setReq2 is setReq with an explicit destination address.
func setReq2(to, hi, lo byte, rec []byte) []byte {
	return ctrlFrame(to, append([]byte{cmdMemory, subMemoryContent, hi, lo}, rec...)...)
}

// TestSameAddressCollision is the case WithRadioAddress exists for: a radio
// configured with the CONTROLLER's own address, so that a frame addressed to
// the controller is one the radio answers, and its answer is addressed to
// itself.
func TestSameAddressCollision(t *testing.T) {
	r := newRadio(t, WithRadioAddress(controllerAddress))
	got := exchange(t, r, ctrlFrame(controllerAddress, cmdID, subID))
	want := radioFrame(controllerAddress, cmdID, subID, controllerAddress)
	if !bytes.Equal(got, want) {
		t.Fatalf("answer = % X, want % X (to and from both E0)", got, want)
	}
	if got[2] != controllerAddress || got[3] != controllerAddress {
		t.Fatalf("answer addresses = %02X %02X, want E0 E0", got[2], got[3])
	}
}

// --- Options ---

func TestWithEcho(t *testing.T) {
	t.Run("off by default", func(t *testing.T) {
		r := newRadio(t)
		got := exchange(t, r, ctrlFrame(defaultRadioAddress, 0x0B))
		if !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
			t.Fatalf("first frame back = % X, want the NG answer and no echo", got)
		}
	})

	t.Run("on, the request comes back first", func(t *testing.T) {
		r := newRadio(t, WithEcho(true))
		req := ctrlFrame(defaultRadioAddress, 0x0B)
		writeBytes(t, r, req)
		if got := readFrame(t, r); !bytes.Equal(got, req) {
			t.Fatalf("echo = % X, want the request % X", got, req)
		}
		if got := readFrame(t, r); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
			t.Fatalf("answer = % X, want NG", got)
		}
		if n := len(r.Sent()); n != 2 {
			t.Fatalf("Sent() has %d frames, want 2 (the echo and the answer)", n)
		}
	})

	t.Run("on, a frame for another radio is echoed and not answered", func(t *testing.T) {
		r := newRadio(t, WithEcho(true))
		req := ctrlFrame(0x88, cmdID, subID)
		writeBytes(t, r, req)
		if got := readFrame(t, r); !bytes.Equal(got, req) {
			t.Fatalf("echo = % X, want the request % X", got, req)
		}
		// Nothing further: the next frame on the wire answers the NEXT request.
		got := exchange(t, r, ctrlFrame(defaultRadioAddress, 0x0B))
		if !bytes.Equal(got, ctrlFrame(defaultRadioAddress, 0x0B)) {
			t.Fatalf("second echo = % X, want the second request", got)
		}
		if got := readFrame(t, r); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
			t.Fatalf("answer = % X, want NG", got)
		}
	})
}

func TestWithLatencyDelaysTheReply(t *testing.T) {
	const latency = 120 * time.Millisecond
	r := newRadio(t, WithLatency(latency))
	start := time.Now()
	got := exchange(t, r, ctrlFrame(defaultRadioAddress, 0x0B))
	elapsed := time.Since(start)
	if !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
		t.Fatalf("answer = % X, want NG", got)
	}
	if elapsed < latency {
		t.Fatalf("the reply arrived after %v, want at least %v", elapsed, latency)
	}
}

func TestCloseIsPromptDuringAPendingLatency(t *testing.T) {
	r := New(WithLatency(30 * time.Second))
	writeBytes(t, r, ctrlFrame(defaultRadioAddress, 0x0B))

	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- r.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("Close took %v, want prompt — the latency wait must be interruptible", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked on a pending latency")
	}
}

func TestCloseGivesTheHostEOFAndIsIdempotent(t *testing.T) {
	r := New()
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	buf := make([]byte, 8)
	_, err := r.Port().Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("host Read after Close = %v, want io.EOF (Close must shut only the radio's end)", err)
	}
}

func TestPortIsTheHostEndAndIsStable(t *testing.T) {
	r := newRadio(t)
	var p io.ReadWriteCloser = r.Port()
	if p == nil {
		t.Fatal("Port() returned nil")
	}
	if r.Port() != p {
		t.Fatal("Port() returned a different connection on the second call")
	}
	// It really is the wire: a frame written through Port() is answered.
	if _, err := p.Write(ctrlFrame(defaultRadioAddress, 0x0B)); err != nil {
		t.Fatalf("Write through Port(): %v", err)
	}
	if got := readFrame(t, r); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
		t.Fatalf("answer = % X, want NG", got)
	}
}

func TestWithTransceiveBroadcastsDoesNotWedgeTheRadio(t *testing.T) {
	r := newRadio(t, WithTransceiveBroadcasts(time.Millisecond))
	frames := drain(r)

	// Broadcasts arrive unasked, addressed to 00.
	deadline := time.After(5 * time.Second)
	seen := 0
	for seen < 3 {
		select {
		case f := <-frames:
			if f == nil {
				t.Fatal("the port closed while waiting for broadcasts")
			}
			if body := frameBody(f); len(body) >= 2 && body[0] == broadcastAddress && body[1] == defaultRadioAddress {
				seen++
			}
		case <-deadline:
			t.Fatalf("saw %d broadcasts in 5s, want 3", seen)
		}
	}

	// And through the flood, the radio still answers.
	writeBytes(t, r, ctrlFrame(defaultRadioAddress, 0x0B))
	want := ngFrame(defaultRadioAddress)
	for {
		select {
		case f := <-frames:
			if f == nil {
				t.Fatal("the port closed before the answer arrived")
			}
			if bytes.Equal(f, want) {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatal("the broadcast flood wedged the radio: no answer in 5s")
		}
	}
}

func TestWithAddressedFloodTargetsTheController(t *testing.T) {
	r := newRadio(t, WithAddressedFlood(time.Millisecond))
	frames := drain(r)

	seen := 0
	deadline := time.After(5 * time.Second)
	for seen < 5 {
		select {
		case f := <-frames:
			if f == nil {
				t.Fatal("the port closed while waiting for flood frames")
			}
			body := frameBody(f)
			if len(body) < 2 {
				t.Fatalf("flood frame % X carries no address pair", f)
			}
			if body[0] != controllerAddress {
				t.Fatalf("flood frame addressed to %02X, want E0 — that is the whole point of this option", body[0])
			}
			if body[1] != defaultRadioAddress {
				t.Fatalf("flood frame from %02X, want 94", body[1])
			}
			seen++
		case <-deadline:
			t.Fatalf("saw %d flood frames in 5s, want 5 — the flood must never go quiet", seen)
		}
	}
}

func TestUnsolicitedTrafficIsOffUnlessAskedFor(t *testing.T) {
	r := newRadio(t)
	if err := r.hostConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	buf := make([]byte, 64)
	n, err := r.hostConn.Read(buf)
	if err == nil {
		t.Fatalf("read %d unsolicited bytes (% X) from a radio with no broadcast or flood option", n, buf[:n])
	}
	if n != 0 {
		t.Fatalf("read %d unsolicited bytes: % X", n, buf[:n])
	}
}

func TestWithChannelSeedsOneSlot(t *testing.T) {
	rec := validRecord()
	other := validRecord()
	other[offName] = 'X'

	r := newRadio(t, WithChannel("001", rec), WithChannel("P2", other))
	got, ok := r.Channel("001")
	if !ok || !bytes.Equal(got, rec) {
		t.Fatalf("Channel(\"001\") = % X, %v", got, ok)
	}
	got, ok = r.Channel("P2")
	if !ok || !bytes.Equal(got, other) {
		t.Fatalf("Channel(\"P2\") = % X, %v", got, ok)
	}
	if n := len(r.Channels()); n != 2 {
		t.Fatalf("%d channels seeded, want 2 — WithChannel seeds ONE slot", n)
	}

	// The option copied its argument: mutating the caller's slice afterwards
	// cannot reach the radio.
	rec[offName] = 'Z'
	if got, _ := r.Channel("001"); got[offName] == 'Z' {
		t.Fatal("WithChannel stored the caller's slice rather than a copy")
	}
}

// TestWithRawChannelBypassesTheLengthCheck is the seam a caller needs to make
// the radio answer a read with a record no set could ever have stored.
func TestWithRawChannelBypassesTheLengthCheck(t *testing.T) {
	foreign := bytes.Repeat([]byte{0xEE}, recordLen+7)
	r := newRadio(t, WithRawChannel("001", foreign))

	got, ok := r.Channel("001")
	if !ok || !bytes.Equal(got, foreign) {
		t.Fatalf("Channel(\"001\") = % X, %v; want the foreign record", got, ok)
	}
	answer := exchange(t, r, readReq(0x00, 0x01))
	want := radioFrame(defaultRadioAddress, append([]byte{cmdMemory, subMemoryContent, 0x00, 0x01}, foreign...)...)
	if !bytes.Equal(answer, want) {
		t.Fatalf("read answered % X, want the foreign-length record % X", answer, want)
	}

	// A SET of the same foreign record is still refused: the bypass is the
	// option's, not the wire's.
	if got := exchange(t, r, setReq(0x00, 0x02, foreign)); !bytes.Equal(got, ngFrame(defaultRadioAddress)) {
		t.Fatalf("a foreign-length set answered % X, want NG", got)
	}
}

func TestOptionsPanicOnValuesThatCannotBeHonoured(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
	}{
		{"WithIDToken(nil)", WithIDToken(nil)},
		{"WithIDToken(empty)", WithIDToken([]byte{})},
		{"WithRadioAddress(FE)", WithRadioAddress(preamble)},
		{"WithRadioAddress(FD)", WithRadioAddress(endOfMessage)},
		{"WithChannel(bad address)", WithChannel("100", validRecord())},
		{"WithChannel(P3)", WithChannel("P3", validRecord())},
		{"WithChannel(short record)", WithChannel("001", []byte{0x00})},
		{"WithChannel(bad nibble)", WithChannel("001", badNibbleRecord())},
		{"WithRawChannel(bad address)", WithRawChannel("000", []byte{0x00})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("did not panic")
				}
			}()
			r := New(tt.opt)
			r.Close()
		})
	}
}

func badNibbleRecord() []byte {
	rec := validRecord()
	rec[offSplitSelect] = 0x99
	return rec
}

// --- Transcripts and stored state ---

func TestTranscriptsAreDefensiveCopies(t *testing.T) {
	r := newRadio(t)
	exchange(t, r, ctrlFrame(defaultRadioAddress, 0x0B))

	rec := r.Received()
	if len(rec) != 1 {
		t.Fatalf("Received() has %d frames, want 1", len(rec))
	}
	rec[0][0] = 0x00
	if again := r.Received(); again[0][0] != preamble {
		t.Fatal("mutating Received()'s result reached the radio's own transcript")
	}

	sent := r.Sent()
	if len(sent) != 1 {
		t.Fatalf("Sent() has %d frames, want 1", len(sent))
	}
	sent[0][0] = 0x00
	if again := r.Sent(); again[0][0] != preamble {
		t.Fatal("mutating Sent()'s result reached the radio's own transcript")
	}
}

func TestChannelAndChannelsReturnCopies(t *testing.T) {
	r := newRadio(t, WithChannel("001", validRecord()))

	got, ok := r.Channel("001")
	if !ok {
		t.Fatal("Channel(\"001\") reported nothing stored")
	}
	got[offName] = 'Z'
	if again, _ := r.Channel("001"); again[offName] == 'Z' {
		t.Fatal("Channel returned the radio's own slice")
	}

	all := r.Channels()
	all["999"] = validRecord()
	all["001"][offName] = 'Z'
	if len(r.Channels()) != 1 {
		t.Fatal("mutating Channels()'s map reached the radio's own map")
	}
	if again, _ := r.Channel("001"); again[offName] == 'Z' {
		t.Fatal("Channels returned the radio's own record slices")
	}

	if _, ok := r.Channel("002"); ok {
		t.Fatal("Channel reported a slot that was never written")
	}
}

func TestANewRadioIsEmpty(t *testing.T) {
	r := newRadio(t)
	if n := len(r.Channels()); n != 0 {
		t.Fatalf("a new radio holds %d channels, want 0", n)
	}
	if n := len(r.Received()); n != 0 {
		t.Fatalf("a new radio has received %d frames, want 0", n)
	}
	if n := len(r.Sent()); n != 0 {
		t.Fatalf("a new radio has sent %d frames, want 0", n)
	}
}

func TestSetOverwritesAndTranscriptsAccumulateInOrder(t *testing.T) {
	r := newRadio(t)
	first := validRecord()
	second := validRecord()
	copy(second[offName:], []byte("SECOND    "))

	exchange(t, r, setReq(0x00, 0x03, first))
	exchange(t, r, setReq(0x00, 0x03, second))

	got, ok := r.Channel("003")
	if !ok || !bytes.Equal(got, second) {
		t.Fatalf("Channel(\"003\") = % X, %v; want the second record", got, ok)
	}

	rec, sent := r.Received(), r.Sent()
	if len(rec) != 2 || len(sent) != 2 {
		t.Fatalf("Received() has %d and Sent() has %d frames, want 2 and 2", len(rec), len(sent))
	}
	if !bytes.Equal(rec[0], setReq(0x00, 0x03, first)) || !bytes.Equal(rec[1], setReq(0x00, 0x03, second)) {
		t.Fatal("Received() is not in arrival order")
	}
	for i, f := range sent {
		if !bytes.Equal(f, okFrame(defaultRadioAddress)) {
			t.Errorf("Sent()[%d] = % X, want OK", i, f)
		}
	}
}

func TestConcurrentInspectionIsSafe(t *testing.T) {
	r := newRadio(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			r.Channels()
			r.Channel("001")
			r.Received()
			r.Sent()
		}
	}()
	for i := 0; i < 20; i++ {
		exchange(t, r, setReq(0x00, 0x01, validRecord()))
	}
	<-done
}

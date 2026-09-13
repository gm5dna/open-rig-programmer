// SPDX-License-Identifier: GPL-3.0-or-later

package fakets570

import (
	"bytes"
	"net"
	"testing"
	"time"
)

// exchange writes req and returns whatever reply arrives within a short
// deadline. Use expectSilence, not exchange, for a request this fake accepts
// with a fire-and-forget success — Read would otherwise block forever
// waiting for a reply that is never sent.
func exchange(t *testing.T, c net.Conn, req string) []byte {
	t.Helper()
	if _, err := c.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	if err := c.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4096)
	n, err := c.Read(b)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return append([]byte(nil), b[:n]...)
}

// expectSilence writes req and asserts nothing arrives within a short
// deadline — the shape of every accepted Set this fake takes.
func expectSilence(t *testing.T, c net.Conn, req string) {
	t.Helper()
	if _, err := c.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	if err := c.SetReadDeadline(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 4096)
	n, err := c.Read(b)
	if err == nil {
		t.Fatalf("expected silence, got % X", b[:n])
	}
	if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
		t.Fatalf("expected a read-deadline timeout, got: %v", err)
	}
}

func TestNew_DefaultRowIsTS570D(t *testing.T) {
	r := New()
	defer r.Close()
	if got, want := exchange(t, r.Port(), "ID;"), []byte("ID017;"); !bytes.Equal(got, want) {
		t.Fatalf("ID = %q, want %q", got, want)
	}
}

func TestWithModelName_SelectsTheRowsCATID(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"TS-570D", "ID017;"},
		{"TS-570S", "ID018;"},
		{"TS-570DG", "ID000;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New(WithModelName(tc.name))
			defer r.Close()
			if got := exchange(t, r.Port(), "ID;"); !bytes.Equal(got, []byte(tc.want)) {
				t.Fatalf("ID = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWithModelName_PanicsOnAnUnknownRow(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an unknown row")
		}
	}()
	WithModelName("TS-570X")
}

func TestWithCATID_OverridesTheDGPlaceholder(t *testing.T) {
	r := New(WithModelName("TS-570DG"), WithCATID("099"))
	defer r.Close()
	if got, want := exchange(t, r.Port(), "ID;"), []byte("ID099;"); !bytes.Equal(got, want) {
		t.Fatalf("ID = %q, want %q", got, want)
	}
}

func TestWithCATID_PanicsOnTheWrongShape(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for a non-three-digit CATID")
		}
	}()
	WithCATID("1")
}

func TestID_HasNoSetDirection(t *testing.T) {
	r := New()
	defer r.Close()
	if got := exchange(t, r.Port(), "ID017;"); !bytes.Equal(got, rejection) {
		t.Fatalf("ID with a body = %q, want %q", got, rejection)
	}
}

func TestMR_UnwrittenChannelAnswersTheZeroRecord(t *testing.T) {
	r := New()
	defer r.Close()
	want := buildMRAnswer(42, HalfRXOrStart, zeroRecord())
	// Request body is P1+P2+P3(2 digits), four bytes: "0"+"0"+"42".
	if got := exchange(t, r.Port(), "MR0042;"); !bytes.Equal(got, want) {
		t.Fatalf("MR = % X, want % X", got, want)
	}
}

func TestMW_StoresAndMR_ReadsItBack(t *testing.T) {
	r := New()
	defer r.Close()
	// MW P1=0 P2=0 P3=05 freq=00014250000 mode=2(USB) lockout=0 tonemode=1
	// toneno=17 P9=00000
	req := "MW0" + "0" + "05" + "00014250000" + "2" + "0" + "1" + "17" + "00000;"
	expectSilence(t, r.Port(), req)
	want := buildMRAnswer(5, HalfRXOrStart, MemState{
		Freq: "00014250000", P2: '0', Mode: '2', Lockout: '0', ToneMode: '1', ToneNo: "17", P9: "00000",
	})
	if got := exchange(t, r.Port(), "MR0005;"); !bytes.Equal(got, want) {
		t.Fatalf("MR after MW = % X, want % X", got, want)
	}
}

func TestMW_AllZeroFrequencyVacatesTheChannel(t *testing.T) {
	r := New()
	defer r.Close()
	r.SetChannel(7, HalfRXOrStart, MemState{Freq: "00007000000", P2: '0', Mode: '3', Lockout: '0', ToneMode: '0', ToneNo: "00", P9: "00000"})
	if _, ok := r.ChannelState(7, HalfRXOrStart); !ok {
		t.Fatal("setup: channel 7 was not staged")
	}
	req := "MW0" + "0" + "07" + allZeroFreq + "0" + "0" + "0" + "00" + "00000;"
	expectSilence(t, r.Port(), req)
	if _, ok := r.ChannelState(7, HalfRXOrStart); ok {
		t.Fatal("channel 7 still has a stored record after an all-zero-frequency MW")
	}
	want := buildMRAnswer(7, HalfRXOrStart, zeroRecord())
	if got := exchange(t, r.Port(), "MR0007;"); !bytes.Equal(got, want) {
		t.Fatalf("MR after erase = % X, want % X (the zero record)", got, want)
	}
}

func TestMR_TheTwoHalvesAreSeparateRecords(t *testing.T) {
	r := New(WithChannel(3, HalfRXOrStart, MemState{Freq: "00007000000", P2: '0', Mode: '0', Lockout: '0', ToneMode: '0', ToneNo: "00", P9: "00000"}))
	defer r.Close()
	// The TX/End half of channel 3 was never written.
	want := buildMRAnswer(3, HalfTXOrEnd, zeroRecord())
	if got := exchange(t, r.Port(), "MR1003;"); !bytes.Equal(got, want) {
		t.Fatalf("MR TX half = % X, want % X", got, want)
	}
}

func TestMW_RefusesMalformedFields(t *testing.T) {
	base := func(p1, p2, ch, freq, mode, lockout, tonemode, toneno, p9 string) string {
		return "MW" + p1 + p2 + ch + freq + mode + lockout + tonemode + toneno + p9 + ";"
	}
	ok := func() string { return base("0", "0", "05", "00014250000", "2", "0", "1", "17", "00000") }
	for _, tc := range []struct {
		name string
		req  string
	}{
		{"bad half", base("2", "0", "05", "00014250000", "2", "0", "1", "17", "00000")},
		{"control code in P2", base("0", "\x01", "05", "00014250000", "2", "0", "1", "17", "00000")},
		{"terminator in P2", base("0", ";", "05", "00014250000", "2", "0", "1", "17", "00000")},
		{"non-digit channel", base("0", "0", "5X", "00014250000", "2", "0", "1", "17", "00000")},
		{"non-digit frequency", base("0", "0", "05", "0001425000X", "2", "0", "1", "17", "00000")},
		{"non-digit mode", base("0", "0", "05", "00014250000", "X", "0", "1", "17", "00000")},
		{"bad lockout", base("0", "0", "05", "00014250000", "2", "2", "1", "17", "00000")},
		{"bad tone mode", base("0", "0", "05", "00014250000", "2", "0", "2", "17", "00000")},
		{"non-digit tone number", base("0", "0", "05", "00014250000", "2", "0", "1", "1X", "00000")},
		{"control code in P9", base("0", "0", "05", "00014250000", "2", "0", "1", "17", "0000\x1f")},
		{"short frame", "MW00005;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New()
			defer r.Close()
			if got := exchange(t, r.Port(), tc.req); !bytes.Equal(got, rejection) {
				t.Fatalf("%s = %q, want %q", tc.name, got, rejection)
			}
		})
	}
	// Sanity: the "ok" shape used to build every malformed variant above is
	// itself accepted, so each failure above is attributable to the one
	// field it mutates.
	r := New()
	defer r.Close()
	if got := exchange(t, r.Port(), ok()+"MR0005;"); bytes.Equal(got, rejection) {
		t.Fatalf("the baseline well-formed MW was itself refused: %q", got)
	}
}

func TestMR_RefusesMalformedRequests(t *testing.T) {
	for _, tc := range []struct{ name, req string }{
		{"bad half", "MR2005;"},          // P1='2': not '0' or '1'
		{"non-digit channel", "MR000X;"}, // P3 = "0X"
		{"too short", "MR005;"},          // body 3 bytes, not 4
		{"too long", "MR00005;"},         // body 5 bytes, not 4
		{"control code filler in P2", "MR0\x0105;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := New()
			defer r.Close()
			if got := exchange(t, r.Port(), tc.req); !bytes.Equal(got, rejection) {
				t.Fatalf("%s = %q, want %q", tc.name, got, rejection)
			}
		})
	}
}

func TestUnknownCommandIsRefused(t *testing.T) {
	r := New()
	defer r.Close()
	if got := exchange(t, r.Port(), "FA;"); !bytes.Equal(got, rejection) {
		t.Fatalf("FA (a command this row's CAT-record shape does not carry) = %q, want %q", got, rejection)
	}
}

func TestWithEmptyChannel_ForcesAWrittenChannelBackToTheZeroRecord(t *testing.T) {
	r := New(
		WithChannel(9, HalfRXOrStart, MemState{Freq: "00009000000", P2: '0', Mode: '0', Lockout: '0', ToneMode: '0', ToneNo: "00", P9: "00000"}),
		WithEmptyChannel(9),
	)
	defer r.Close()
	want := buildMRAnswer(9, HalfRXOrStart, zeroRecord())
	if got := exchange(t, r.Port(), "MR0009;"); !bytes.Equal(got, want) {
		t.Fatalf("MR after WithEmptyChannel = % X, want % X", got, want)
	}
}

func TestAccumulatorOverflowRejectsOnceAndResyncs(t *testing.T) {
	r := New()
	defer r.Close()
	c := r.Port()
	// One Write: the overflow-triggering run PLUS the ';' that ends the
	// resync, so the reply this produces is read before any further Write —
	// net.Pipe is synchronous, and a Write-Write-Read ordering here would
	// deadlock the radio's own goroutine against the host's.
	overflow := append(bytes.Repeat([]byte{'X'}, maxAccumulatorBytes+1), ';')
	got := exchange(t, c, string(overflow))
	if !bytes.Equal(got, rejection) {
		t.Fatalf("overflow reply = %q, want %q", got, rejection)
	}
	// The link is not wedged: an ordinary request afterwards still answers.
	if got := exchange(t, c, "ID;"); !bytes.Equal(got, []byte("ID017;")) {
		t.Fatalf("after overflow: %q", got)
	}
}

// TestWithStreamError_ReplacesTheScriptedExchangeAndNothingElse pins the two
// serial-line error tokens the manual prints beside "?;" (printed folio 70,
// layout lines 5158-5175) — doc.go register entry 7, lift-K follow-up
// e7515d0. Exchange 2 (the second frame handled) is scripted to answer "E;"
// in place of its ordinary ID reply; exchange 1 and exchange 3 are
// untouched, proving the script is per-exchange, not sticky.
func TestWithStreamError_ReplacesTheScriptedExchangeAndNothingElse(t *testing.T) {
	r := New(WithStreamError(StreamErrorE, 2))
	defer r.Close()
	c := r.Port()

	if got := exchange(t, c, "ID;"); !bytes.Equal(got, []byte("ID017;")) {
		t.Fatalf("exchange 1 = %q, want the ordinary ID reply", got)
	}
	if got := exchange(t, c, "ID;"); !bytes.Equal(got, []byte("E;")) {
		t.Fatalf("exchange 2 = %q, want the scripted %q", got, "E;")
	}
	if got := exchange(t, c, "ID;"); !bytes.Equal(got, []byte("ID017;")) {
		t.Fatalf("exchange 3 = %q, want the ordinary ID reply again", got)
	}
}

// TestWithStreamError_ReplacesAFireAndForgetSuccessToo pins that a scripted
// stream error pre-empts even an accepted Set's silence — the manual's two
// tokens are not command outcomes, so they can arrive in place of any reply
// at all, including no reply.
func TestWithStreamError_ReplacesAFireAndForgetSuccessToo(t *testing.T) {
	r := New(WithStreamError(StreamErrorO, 1))
	defer r.Close()
	req := "MW0" + "0" + "05" + "00014250000" + "2" + "0" + "1" + "17" + "00000;"
	if got := exchange(t, r.Port(), req); !bytes.Equal(got, []byte("O;")) {
		t.Fatalf("scripted exchange over an accepted MW = %q, want %q", got, "O;")
	}
}

func TestWithStreamError_PanicsOnAnUnsetKindOrAnExchangeBelowOne(t *testing.T) {
	for _, tc := range []struct {
		name string
		fn   func()
	}{
		{"unset kind", func() { WithStreamError(StreamErrorUnset, 1) }},
		{"exchange zero", func() { WithStreamError(StreamErrorE, 0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic")
				}
			}()
			tc.fn()
		})
	}
}

func TestClose_IsPromptWithNoTraffic(t *testing.T) {
	r := New()
	done := make(chan error, 1)
	go func() { done <- r.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close blocked")
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close = %v", err)
	}
}

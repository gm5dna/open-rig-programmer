// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// This file's golden-exchange tests recompute every expected reply byte
// string as a literal, directly from the protocol reference's tables and
// golden vectors (G1-G12) — never by calling fakeradio's own builders
// (buildMRAnswer, buildMTReply, ...). That independence is the whole
// point of a golden test: it must be possible for buildMRAnswer to have a
// bug and still be CAUGHT by the test, which cannot happen if the test's
// "expected" value comes from the same buggy function.

const testTimeout = 2 * time.Second

// newTestRadio constructs a *Radio for a test, registers its Close() as
// cleanup, and returns both the Radio (for SlotState/CurrentChannel
// assertions) and its Port().
func newTestRadio(t *testing.T, opts ...Option) (*Radio, io.ReadWriteCloser) {
	t.Helper()
	r := New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	return r, r.Port()
}

func writeFrame(t *testing.T, w io.Writer, s string) {
	t.Helper()
	if _, err := w.Write([]byte(s)); err != nil {
		t.Fatalf("Write(%q): unexpected error: %v", s, err)
	}
}

// deadliner is satisfied by net.Pipe's Conn (and, via pass-through, by
// countingReader in faults_test.go): a real, cancelling read deadline,
// rather than a goroutine-based "abandon and hope" timeout. That
// distinction matters here: net.Pipe's Write is a rendezvous with
// WHICHEVER Read is CURRENTLY blocked (verified empirically — the oldest
// blocked reader wins, and a second, later reader is left blocked
// indefinitely). A helper built on "spawn a goroutine, give up on it at a
// deadline" would leave that abandoned goroutine's Read blocked
// indefinitely, and a reply arriving after the fact could rendezvous
// with THAT abandoned goroutine instead of a later readOneFrame call's
// own — silently swallowing it. Using SetReadDeadline makes the timeout
// a genuine cancellation of the SAME Read call, with no goroutine left
// behind, so this hazard cannot occur.
type deadliner interface {
	SetReadDeadline(time.Time) error
}

// readOneFrame reads until it has accumulated one complete ';'-terminated
// frame (looping over possibly many small Read calls, so it works
// identically whether or not FaultChunkedReplies is active), or until
// timeout elapses with nothing arriving (timedOut == true), or the
// connection reports a non-timeout error (e.g. io.EOF after a
// disconnect). r must implement deadliner (both net.Pipe's Conn and
// countingReader do).
func readOneFrame(t *testing.T, r io.Reader, timeout time.Duration) (frame []byte, err error, timedOut bool) {
	t.Helper()
	d, ok := r.(deadliner)
	if !ok {
		t.Fatalf("readOneFrame: %T does not implement deadliner (SetReadDeadline)", r)
	}
	if err := d.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		t.Fatalf("SetReadDeadline: unexpected error: %v", err)
	}
	defer func() { _ = d.SetReadDeadline(time.Time{}) }()

	buf := make([]byte, 256)
	var acc []byte
	for {
		n, rerr := r.Read(buf)
		acc = append(acc, buf[:n]...)
		if len(acc) > 0 && acc[len(acc)-1] == ';' {
			return acc, nil, false
		}
		if rerr != nil {
			if ne, ok := rerr.(net.Error); ok && ne.Timeout() {
				return acc, nil, true
			}
			return acc, rerr, false
		}
	}
}

func mustReadFrame(t *testing.T, r io.Reader) string {
	t.Helper()
	frame, err, timedOut := readOneFrame(t, r, testTimeout)
	if timedOut {
		t.Fatalf("readOneFrame: timed out after %v waiting for a reply", testTimeout)
	}
	if err != nil && len(frame) == 0 {
		t.Fatalf("readOneFrame: unexpected error: %v", err)
	}
	return string(frame)
}

// assertNoReply confirms nothing arrives within a short window — used to
// verify fire-and-forget success (no acknowledgement).
func assertNoReply(t *testing.T, r io.Reader) {
	t.Helper()
	frame, _, timedOut := readOneFrame(t, r, 150*time.Millisecond)
	if !timedOut {
		t.Fatalf("expected no reply, got %q", frame)
	}
}

// --- Golden exchanges (reference doc "Golden vectors" table, G1-G12) ---

func TestGolden_ID(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "ID;")
	// G1: ID; -> ID0800;
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Errorf("ID; -> %q, want %q", got, want)
	}
}

func TestGolden_AI(t *testing.T) {
	_, conn := newTestRadio(t)

	// G2: AI0; is fire-and-forget (reference: "Set-only commands ...
	// produce NO acknowledgement on success").
	writeFrame(t, conn, "AI0;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "AI;")
	if got, want := mustReadFrame(t, conn), "AI0;"; got != want {
		t.Errorf("AI; after AI0; -> %q, want %q", got, want)
	}

	writeFrame(t, conn, "AI1;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "AI;")
	if got, want := mustReadFrame(t, conn), "AI1;"; got != want {
		t.Errorf("AI; after AI1; -> %q, want %q", got, want)
	}
}

func TestGolden_MR_EmptySlot(t *testing.T) {
	// G3: "MR007;" (read M-07) — M-07 is not in the factory image, so
	// this exercises the empty-slot ASSUMED rule, not a populated
	// answer.
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MR007;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("MR007; (empty slot) -> %q, want %q", got, want)
	}
}

func TestGolden_MR_M01(t *testing.T) {
	// G4: MR001007000000+000000110000; — the answer to reading M-01,
	// which the factory image populates at 7.000000 MHz LSB.
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MR001;")
	want := "MR001007000000+000000110000;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MR001; -> %q, want %q", got, want)
	}
}

func TestGolden_MR_PMS_P1L(t *testing.T) {
	// G6: MRP1L001810000+000000150000; — PMS P1L, populated by the
	// factory image at 1.810000 MHz LSB, kind PMS(5).
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MRP1L;")
	want := "MRP1L001810000+000000150000;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MRP1L; -> %q, want %q", got, want)
	}
}

func TestGolden_MW_MR_RoundTrip_G5(t *testing.T) {
	// G5: MW005014250000+000000210000; — write M-05, 14.250000 MHz, USB,
	// P7=1 (ASSUMED), CTCSS off, simplex. Fire-and-forget on success;
	// MR005; must then echo the identical field values back (with "MR"
	// in place of "MW").
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MW005014250000+000000210000;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "MR005;")
	want := "MR005014250000+000000210000;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MR005; after MW005...; -> %q, want %q", got, want)
	}
}

func TestGolden_MW_MR_RoundTrip_G7_AllFieldsNonDefault(t *testing.T) {
	// G7: MW099052354000-012010411002; — write M-99 exercising every
	// field != default: clar -0120 rx-on/tx-off, FM, P7=1, CTCSS
	// ENC/DEC, minus shift.
	//
	// HW-CORRECTED 2026-07-13 (M5b write trials, doc.go register item
	// 20): the manual-derived expectation used to be a FULL echo,
	// clarifier included — hardware refuted that: the radio accepts the
	// clarifier value and Rx/Tx flags but silently IGNORES them (reads
	// back zeros every time), while honouring every other field. The
	// readback below therefore zeroes exactly the clarifier fields
	// ("+00000" for "-01201") and echoes the rest verbatim. See
	// TestHWDerived_MW_ClarifierIgnored_M5b for the live trial vectors
	// this correction derives from.
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MW099052354000-012010411002;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "MR099;")
	want := "MR099052354000+000000411002;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MR099; after MW099...; -> %q, want %q (clarifier ignored, everything else honoured)", got, want)
	}
}

// TestHWDerived_MW_KindMatrix_M5b pins the M5b write-trial kind matrix
// against Stuart's real UK FT-710 (13/07/2026, docs/hardware-notes.md):
// MW is accepted with kind '1' (KindMemory) for BOTH memory and PMS
// slots, and REJECTED with an immediate "?;" for kind '0' (KindVFO) and
// kind '5' (KindPMS) — even when writing to a PMS slot, which the
// manual's own worked example implied should pair with kind '5' (WRONG,
// hardware-refuted). This reproduces the live bug this task fixes:
// before the fix, fakeradio permissively accepted any kind byte '0'-'5'
// on MW, which let a PMS write with kind '5' succeed here even though
// the real radio rejects it — masking the write.go bug rather than
// catching it.
func TestHWDerived_MW_KindMatrix_M5b(t *testing.T) {
	tests := []struct {
		name   string
		frame  string
		accept bool
	}{
		{"MEM kind 1 accepted", "MW095011685000+000000510000;", true},
		{"PMS kind 1 accepted (hardware-refuted former assumption was kind 5)", "MWP1L007100000+000000110000;", true},
		{"MEM kind 0 (VFO) rejected", "MW095011690000+000000500000;", false},
		{"PMS kind 5 (KindPMS) rejected", "MWP1L007100000+000000150000;", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, tt.frame)
			if tt.accept {
				assertNoReply(t, conn)
			} else if got, want := mustReadFrame(t, conn), "?;"; got != want {
				t.Errorf("%s -> %q, want %q", tt.frame, got, want)
			}
		})
	}
}

// TestHWDerived_MW_ClarifierIgnored_M5b pins the M5b clarifier finding
// (HW-CONFIRMED 2026-07-13, docs/hardware-notes.md): MW frames carrying
// a non-zero clarifier value and/or Rx/Tx clarifier flags are ACCEPTED
// (no "?;" rejection) but the values are silently IGNORED — the channel
// reads back zeros every time. The frames below are live trial vectors
// verbatim: writes to M-95 carrying clar +0100 with rx=1, then clar
// -0250 with rx=1 AND tx=1, each read back as "+000000". Every OTHER
// field in the same frame IS honoured (frequency/mode/kind/CTCSS/shift
// echo normally).
func TestHWDerived_MW_ClarifierIgnored_M5b(t *testing.T) {
	tests := []struct {
		name  string
		write string
		want  string // MR readback: clarifier zeroed, everything else as written
	}{
		{
			name:  "clar +0100 rx=1 ignored (live vector)",
			write: "MW095011685000+010010510000;",
			want:  "MR095011685000+000000510000;",
		},
		{
			name:  "clar -0250 rx=1 tx=1 ignored (live vector, sign + value + both flags at once)",
			write: "MW095011685000-025011510000;",
			want:  "MR095011685000+000000510000;",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, tt.write)
			assertNoReply(t, conn) // accepted — fire-and-forget, no rejection

			writeFrame(t, conn, "MR095;")
			if got := mustReadFrame(t, conn); got != tt.want {
				t.Errorf("MR095; after %s -> %q, want %q (clarifier fields stored as zeros)", tt.write, got, tt.want)
			}
		})
	}
}

// TestHWDerived_MW_PMS_ReadsBackKindMemory pins the M5b finding that a
// CAT-written PMS slot reads back kind '1' (KindMemory), not '5'
// (KindPMS) — HW-CONFIRMED 2026-07-13, docs/hardware-notes.md. This is
// the live failure frame `MRP1L007100000+000000110000;` reproduced
// end-to-end through fakeradio's own MW/MR handling (see
// core/driver/ft710/read_test.go for the driver-level HW-derived vector
// built from this exact exchange).
func TestHWDerived_MW_PMS_ReadsBackKindMemory(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MWP1L007100000+000000110000;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "MRP1L;")
	want := "MRP1L007100000+000000110000;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MRP1L; after CAT-written MW -> %q, want %q (kind '1', not '5')", got, want)
	}
}

func TestGolden_MT_SetRead_G8(t *testing.T) {
	// G8/G10: MT0011CALLING FREQ; sets M-01's tag (12-char tag, display
	// on); MT001; reads it back unchanged.
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MT0011CALLING FREQ;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "MT001;")
	want := "MT0011CALLING FREQ;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MT001; after MT0011CALLING FREQ; -> %q, want %q", got, want)
	}
}

func TestGolden_MT_SetRead_G9(t *testing.T) {
	// G9: MT005040M; sets M-05's tag, display OFF, tag "40M" (variable
	// length, 3 chars).
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MT005040M;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "MT005;")
	want := "MT005040M;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MT005; after MT005040M; -> %q, want %q", got, want)
	}
}

func TestGolden_MT_On60mSlot(t *testing.T) {
	// Brief: "MT<slot><display><tag>; set -> accepted for ALL slot kinds
	// incl. 5xx/EMG" — the fake models the radio's grammar, not a future
	// host policy. 501 is a grammatically valid 60m slot FORM regardless
	// of the factory image (HW-CONFIRMED 2026-07-13: ImageUK, the default
	// here, no longer populates any 5xx slot at all — see image.go — so
	// this exercises MT-set's slot-form check, not a populated channel).
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MT5011CHAN A;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "MT501;")
	want := "MT5011CHAN A;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MT501; after MT5011CHAN A; -> %q, want %q", got, want)
	}
}

func TestGolden_MT_ReadOnNeverTouchedSlot(t *testing.T) {
	// HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md §Empty/out-of-range
	// slots; live evidence "MT010; -> ?;" with AND without a preceding
	// MR010;, M-10 never MW'd or MT-set): a slot with NO recorded state at
	// all — never MW'd, never MT-set, only the factory image's absence —
	// answers "?;" to MT, exactly like MR/MC. This OVERTURNS the former
	// ASSUMED design (MT read of an empty slot succeeding with display=0,
	// empty tag): the real radio does not. M-50 is not in the factory
	// image, so this exercises that never-touched state.
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MR050;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Fatalf("test precondition failed: MR050; = %q, want %q (M-50 must be unpopulated)", got, want)
	}

	writeFrame(t, conn, "MT050;")
	want := "?;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MT050; (never-touched slot) -> %q, want %q", got, want)
	}
}

func TestGolden_MT_ReadOnNeverTouchedSlot_WithoutPrecedingMR(t *testing.T) {
	// HW-CONFIRMED 2026-07-13: the live probe hit "MT010; -> ?;" as a
	// standalone exchange too (no MR immediately before it on that exact
	// slot) — pinned here as ITS OWN case (M-51, never MR'd in this test)
	// so the never-touched rule cannot be mistaken for something MR010's
	// prior read caused.
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MT051;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("MT051; (never-touched slot, no preceding MR) -> %q, want %q", got, want)
	}
}

func TestGolden_MC_RecallAndRead_G11(t *testing.T) {
	// G11: MC099; recalls M-99. M-99 must be populated first (MC-set on
	// an empty slot is "?;" — see TestMC_EmptySlot). MC; then reads back
	// the current channel.
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MW099052354000-012010411002;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "MC099;")
	assertNoReply(t, conn)

	writeFrame(t, conn, "MC;")
	if got, want := mustReadFrame(t, conn), "MC099;"; got != want {
		t.Errorf("MC; after MC099; -> %q, want %q", got, want)
	}
}

func TestGolden_MC_DefaultIsVFO000(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MC;")
	if got, want := mustReadFrame(t, conn), "MC000;"; got != want {
		t.Errorf("MC; with no prior recall -> %q, want %q", got, want)
	}
}

func TestGolden_Rejection_UnknownCommand(t *testing.T) {
	// Reference: "Unknown-but-well-formed commands (e.g. FA...;) -> ?;"
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "FA01234567;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("FA01234567; (unknown command) -> %q, want %q", got, want)
	}
}

func TestGolden_Rejection_Garbage(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "!@#$%^;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("garbage -> %q, want %q", got, want)
	}
}

// --- Chunked delivery (sending, not FaultChunkedReplies, which is
// about replies) ---

func TestChunkedDelivery_ByteAtATime(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, b := range []byte("MR001;") {
		writeFrame(t, conn, string(b))
	}
	want := "MR001007000000+000000110000;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MR001; sent byte-at-a-time -> %q, want %q", got, want)
	}
}

func TestChunkedDelivery_TwoCommandsInOneWrite(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "ID;MR001;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Errorf("first reply = %q, want %q", got, want)
	}
	if got, want := mustReadFrame(t, conn), "MR001007000000+000000110000;"; got != want {
		t.Errorf("second reply = %q, want %q", got, want)
	}
}

func TestChunkedDelivery_CommandSplitAcrossThreeWrites(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MR")
	writeFrame(t, conn, "00")
	writeFrame(t, conn, "1;")
	want := "MR001007000000+000000110000;"
	if got := mustReadFrame(t, conn); got != want {
		t.Errorf("MR001; split across 3 writes -> %q, want %q", got, want)
	}
}

// --- Empty/malformed/oversized rejections, and accumulator overflow ---

func TestRejection_MalformedSlot(t *testing.T) {
	tests := []string{"MR100;", "MRP0L;", "MRP9X;", "MREM;", "MR000;"}
	for _, frame := range tests {
		t.Run(frame, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, frame)
			if got, want := mustReadFrame(t, conn), "?;"; got != want {
				t.Errorf("%s -> %q, want %q", frame, got, want)
			}
		})
	}
}

func TestRejection_MW_DisallowedSlots(t *testing.T) {
	// Rejection cases: MW to 501/EMG/000.
	base := "007000000+000000110000;" // valid field content, only the slot varies
	for _, slot := range []string{"501", "EMG", "000"} {
		t.Run(slot, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, "MW"+slot+base)
			if got, want := mustReadFrame(t, conn), "?;"; got != want {
				t.Errorf("MW to slot %s -> %q, want %q", slot, got, want)
			}
		})
	}
}

func TestRejection_MT_OversizedTag(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MT0011ABCDEFGHIJKLM;") // 13-char tag
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("MT with 13-char tag -> %q, want %q", got, want)
	}
}

func TestRejection_MT_TagWithControlByte(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MT0011AB\x01CD;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("MT with a control byte in the tag -> %q, want %q", got, want)
	}
}

// TestHWDerived_MT_ZeroByteTagSetRejected_SpacesFormClears pins the
// 13/07/2026 tag-clear probes (docs/fixtures-private/
// m5b-trials.private-capture, stages tagclear/tagclear2;
// docs/hardware-notes.md §Empty-slot create, tag-clear), which caught
// this fake's third proven divergence from hardware:
//
//  1. A ZERO-byte-tag MT Set ("MT0011;" — exactly the frame the write
//     path used to emit for Tag == "") is REJECTED ("?;", ~4 ms live)
//     and the existing tag SURVIVES — the fake formerly ACCEPTED it and
//     cleared the tag, which the real radio provably does not.
//  2. The all-spaces 12-byte tag Set IS the radio's (one proven)
//     tag-CLEAR mechanism: accepted, and the tag reads back all-spaces
//     — which cat.ParseMTAnswer's trim then models as "" (the earlier
//     half of this fix wave).
func TestHWDerived_MT_ZeroByteTagSetRejected_SpacesFormClears(t *testing.T) {
	_, conn := newTestRadio(t)

	// Give M-01 a tag to survive/clear.
	writeFrame(t, conn, "MT0011CALLING FREQ;")
	assertNoReply(t, conn)

	// 1. The 0-byte-tag Set draws "?;" and the tag survives.
	writeFrame(t, conn, "MT0011;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Fatalf("MT Set with 0-byte tag -> %q, want %q (HW-CONFIRMED rejection)", got, want)
	}
	writeFrame(t, conn, "MT001;")
	if got, want := mustReadFrame(t, conn), "MT0011CALLING FREQ;"; got != want {
		t.Errorf("MT001; after rejected 0-byte Set -> %q, want %q (tag must survive)", got, want)
	}

	// 2. The all-spaces 12-byte Set clears: accepted, reads back as the
	// spaces it stored (wire-level; the model-level "" comes from
	// cat.ParseMTAnswer's trim, not from this fake).
	writeFrame(t, conn, "MT0010            ;")
	assertNoReply(t, conn)
	writeFrame(t, conn, "MT001;")
	if got, want := mustReadFrame(t, conn), "MT0010            ;"; got != want {
		t.Errorf("MT001; after all-spaces clear -> %q, want %q", got, want)
	}
}

func TestRejection_000IsNeverAValidRequestSlot(t *testing.T) {
	// doc.go ASSUMED register #15: "000" only ever appears inside
	// answers; a request naming it is a malformed slot, for every
	// command that takes a slot parameter.
	tests := []string{
		"MR000;",
		"MW000007000000+000000110000;",
		"MT000;",
		"MT0001HELLO;",
		"MC000;",
	}
	for _, frame := range tests {
		t.Run(frame, func(t *testing.T) {
			_, conn := newTestRadio(t)
			writeFrame(t, conn, frame)
			if got, want := mustReadFrame(t, conn), "?;"; got != want {
				t.Errorf("%s -> %q, want %q", frame, got, want)
			}
		})
	}
}

func TestMC_EmptySlot(t *testing.T) {
	_, conn := newTestRadio(t)
	writeFrame(t, conn, "MC050;") // M-50 not populated
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("MC050; (empty slot) -> %q, want %q", got, want)
	}
}

func TestAccumulatorOverflow_ThenResync(t *testing.T) {
	_, conn := newTestRadio(t)

	garbage := make([]byte, 300)
	for i := range garbage {
		garbage[i] = 'X'
	}
	// 300 'X's (exceeding the ~256 cap), then a terminator (completing
	// the resync), then a normal command, all in ONE Write.
	frame := string(garbage) + ";ID;"
	writeFrame(t, conn, frame)

	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("overflow reply = %q, want %q", got, want)
	}
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Errorf("post-resync reply = %q, want %q (resync must have recovered)", got, want)
	}
}

func TestAccumulatorOverflow_ResyncAcrossWrites(t *testing.T) {
	_, conn := newTestRadio(t)

	garbage := make([]byte, 300)
	for i := range garbage {
		garbage[i] = 'Y'
	}
	writeFrame(t, conn, string(garbage)) // no terminator at all yet
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Errorf("overflow reply = %q, want %q", got, want)
	}

	// The resync-terminating ';' and the next command arrive in a
	// SEPARATE, later Write.
	writeFrame(t, conn, ";ID;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Errorf("post-resync reply (resync spanned writes) = %q, want %q", got, want)
	}
}

// --- Fire-and-forget confirmation ---

func TestFireAndForgetCommandsProduceNoReply(t *testing.T) {
	tests := []struct {
		name  string
		frame string
	}{
		{"MW", "MW001007000000+000000110000;"},
		{"MT-set", "MT0011TEST;"},
		{"MC-set", "MC099;"}, // preceded by MW below so it's populated
		{"AI-set", "AI0;"},
	}
	_, conn := newTestRadio(t)
	// Populate M-99 first so the MC-set case isn't rejected as empty.
	writeFrame(t, conn, "MW099007000000+000000110000;")
	assertNoReply(t, conn)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeFrame(t, conn, tt.frame)
			assertNoReply(t, conn)
		})
	}
}

// --- Close / disconnect basics (fault-specific disconnect tests live in
// faults_test.go) ---

func TestClose_HostReadGetsEOF(t *testing.T) {
	r, conn := newTestRadio(t)
	if err := r.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}
	buf := make([]byte, 8)
	_, err := conn.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Errorf("Read after Close: err = %v, want io.EOF", err)
	}
}

func TestClose_IsIdempotent(t *testing.T) {
	r, _ := newTestRadio(t)
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: unexpected error: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: unexpected error: %v", err)
	}
}

// --- SlotState / CurrentChannel inspection API ---

func TestSlotState_ReflectsMWWrite(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "MW005014250000+000000210000;")
	assertNoReply(t, conn)

	s, ok := r.SlotState("005")
	if !ok {
		t.Fatal("SlotState(\"005\") ok = false, want true after MW")
	}
	if !s.Populated {
		t.Error("SlotState(\"005\").Populated = false, want true after MW")
	}
	if s.Freq != "014250000" || s.Mode != '2' || s.Kind != '1' {
		t.Errorf("SlotState(\"005\") = %+v, want Freq=014250000 Mode=2 Kind=1", s)
	}
}

func TestSlotState_MTDoesNotPopulate(t *testing.T) {
	// ASSUMED (doc.go register #5): MT-set does not mark Populated.
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "MT0501HELLO;") // M-50 unpopulated
	assertNoReply(t, conn)

	s, ok := r.SlotState("050")
	if !ok {
		t.Fatal("SlotState(\"050\") ok = false, want true after MT (tag state was recorded)")
	}
	if s.Populated {
		t.Error("SlotState(\"050\").Populated = true, want false — MT alone must not populate a channel")
	}
	if s.Tag != "HELLO" || !s.TagDisplay {
		t.Errorf("SlotState(\"050\") tag = %q display = %v, want \"HELLO\" true", s.Tag, s.TagDisplay)
	}
}

func TestCurrentChannel_TracksMCSet(t *testing.T) {
	r, conn := newTestRadio(t)
	if got := r.CurrentChannel(); got != "000" {
		t.Errorf("CurrentChannel() before any MC-set = %q, want \"000\"", got)
	}
	writeFrame(t, conn, "MW099052354000-012010411002;")
	assertNoReply(t, conn)
	writeFrame(t, conn, "MC099;")
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != "099" {
		t.Errorf("CurrentChannel() after MC099; = %q, want \"099\"", got)
	}
}

// TestCurrentChannel_TracksMWWrite: HW-CONFIRMED 2026-07-13 (M5b write
// trials, docs/hardware-notes.md) — an MW write moves the radio's
// selection to the written slot, hands-off, with no MC-set involved at
// all (bulk sends drag the selection through every written channel).
// This is the new obligation clone's Execute snapshots/restores around
// (core/clone's MemorySelector).
func TestCurrentChannel_TracksMWWrite(t *testing.T) {
	r, conn := newTestRadio(t)
	if got := r.CurrentChannel(); got != "000" {
		t.Errorf("CurrentChannel() before any write = %q, want \"000\"", got)
	}
	writeFrame(t, conn, "MW005014250000+000000210000;")
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != "005" {
		t.Errorf("CurrentChannel() after MW005...; = %q, want \"005\" (MW moves selection)", got)
	}

	writeFrame(t, conn, "MW099052354000-012010411002;")
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != "099" {
		t.Errorf("CurrentChannel() after a SECOND MW = %q, want \"099\" (bulk sends drag selection through every written channel)", got)
	}
}

// TestCurrentChannel_RejectedMWDoesNotMoveSelection: a REJECTED MW (bad
// kind, disallowed slot, etc.) never touches the radio's stored state —
// it must not move the current-channel selection either.
func TestCurrentChannel_RejectedMWDoesNotMoveSelection(t *testing.T) {
	r, conn := newTestRadio(t)
	writeFrame(t, conn, "MW005014250000+000000210000;")
	assertNoReply(t, conn)
	if got := r.CurrentChannel(); got != "005" {
		t.Fatalf("CurrentChannel() after MW005...; = %q, want \"005\"", got)
	}

	// Rejected: kind '5' (KindPMS) on a PMS write — HW-CONFIRMED rejected.
	writeFrame(t, conn, "MWP1L007100000+000000150000;")
	if got, want := mustReadFrame(t, conn), "?;"; got != want {
		t.Fatalf("MW with rejected kind -> %q, want %q", got, want)
	}
	if got := r.CurrentChannel(); got != "005" {
		t.Errorf("CurrentChannel() after a REJECTED MW = %q, want \"005\" unchanged", got)
	}
}

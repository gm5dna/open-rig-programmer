// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"errors"
	"testing"
)

// frameFor builds an arbitrary well-formed frame for accumulator tests.
// It bypasses the builders deliberately: the accumulator's job is to
// reassemble whatever arrives, including frames no builder would emit.
func frameFor(to, from byte, body ...byte) []byte {
	out := []byte{PreambleByte, PreambleByte, to, from}
	out = append(out, body...)
	return append(out, EndByte)
}

// pushAll feeds every chunk and returns the concatenated frames, failing
// on any error.
func pushAll(t *testing.T, a *FrameAccumulator, chunks ...[]byte) [][]byte {
	t.Helper()
	var out [][]byte
	for i, c := range chunks {
		got, err := a.Push(c)
		if err != nil {
			t.Fatalf("Push(chunk %d, % 02x): %v", i, c, err)
		}
		out = append(out, got...)
	}
	return out
}

func TestAccumulator_SplitsAndCoalesces(t *testing.T) {
	const ctrl = 0xE0
	a := NewFrameAccumulator(0, ctrl)

	f1 := frameFor(ctrl, 0x94, 0x19, 0x00, 0x94)
	f2 := frameFor(ctrl, 0x94, AckByte)

	// One frame split across three reads, then two frames in one read.
	got := pushAll(t, a,
		f1[:1], f1[1:4], f1[4:],
		append(append([]byte{}, f2...), f2...),
	)
	if len(got) != 3 {
		t.Fatalf("got %d frames, want 3: % 02x", len(got), got)
	}
	if string(got[0]) != string(f1) {
		t.Fatalf("frame 0 = % 02x, want % 02x", got[0], f1)
	}
	for i := 1; i < 3; i++ {
		if string(got[i]) != string(f2) {
			t.Fatalf("frame %d = % 02x, want % 02x", i, got[i], f2)
		}
	}
	if s := a.Stats(); s.Frames != 3 {
		t.Fatalf("Stats().Frames = %d, want 3", s.Frames)
	}
}

func TestAccumulator_ToleratesLeadingNoise(t *testing.T) {
	const ctrl = 0xE0
	a := NewFrameAccumulator(0, ctrl)
	f := frameFor(ctrl, 0x94, AckByte)

	noisy := append([]byte{0x00, 0xFF, 0x7B, 0x01}, f...)
	got := pushAll(t, a, noisy)
	if len(got) != 1 || string(got[0]) != string(f) {
		t.Fatalf("got % 02x, want one frame % 02x", got, f)
	}
	if s := a.Stats(); s.NoiseBytes != 4 {
		t.Fatalf("Stats().NoiseBytes = %d, want 4", s.NoiseBytes)
	}
}

func TestAccumulator_ToleratesExtraPreamblePadding(t *testing.T) {
	const ctrl = 0xE0
	a := NewFrameAccumulator(0, ctrl)
	canonical := frameFor(ctrl, 0x94, 0x19, 0x00, 0x94)

	// Six preamble bytes rather than two. The returned frame must be
	// NORMALISED to the canonical two: everything downstream — the gate,
	// the answer matcher, and echo comparison against a frame this
	// package built — works on canonical frames, and a padded copy that
	// is byte-different from an identical canonical one would defeat all
	// three.
	padded := append([]byte{PreambleByte, PreambleByte, PreambleByte, PreambleByte}, canonical...)
	got := pushAll(t, a, padded)
	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1: % 02x", len(got), got)
	}
	if string(got[0]) != string(canonical) {
		t.Fatalf("padded frame came back as % 02x, want the canonical % 02x", got[0], canonical)
	}
}

func TestAccumulator_ResynchronisesOnATruncatedFrame(t *testing.T) {
	const ctrl = 0xE0
	a := NewFrameAccumulator(0, ctrl)
	good := frameFor(ctrl, 0x94, 0x19, 0x00, 0x94)

	// A frame that lost its terminator, immediately followed by a whole
	// one. The partial must be abandoned, not spliced into the good frame.
	truncated := []byte{PreambleByte, PreambleByte, ctrl, 0x94, 0x1A, 0x00}
	got := pushAll(t, a, append(append([]byte{}, truncated...), good...))
	if len(got) != 1 || string(got[0]) != string(good) {
		t.Fatalf("got % 02x, want one frame % 02x", got, good)
	}
	if s := a.Stats(); s.Truncated != 1 {
		t.Fatalf("Stats().Truncated = %d, want 1", s.Truncated)
	}
}

func TestAccumulator_EnforcesTheProfileSuppliedMaximum(t *testing.T) {
	const ctrl = 0xE0
	const max = 16
	a := NewFrameAccumulator(max, ctrl)

	good := frameFor(ctrl, 0x94, AckByte)
	oversize := frameFor(ctrl, 0x94, make([]byte, max)...)

	frames, err := a.Push(append(append([]byte{}, good...), oversize...))
	// As with core/cat's accumulator: frames found BEFORE the violation
	// are still returned alongside the error.
	if len(frames) != 1 || string(frames[0]) != string(good) {
		t.Fatalf("frames before the violation = % 02x, want % 02x", frames, good)
	}
	if err == nil {
		t.Fatal("an oversize frame was accepted")
	}
	if !errors.Is(err, ErrFrameTooLong) {
		t.Fatalf("error %v does not match ErrFrameTooLong", err)
	}
	var tooLong *FrameTooLongError
	if !errors.As(err, &tooLong) {
		t.Fatalf("error %v is not a *FrameTooLongError", err)
	}
	if tooLong.DiscardedLen <= 0 {
		t.Fatalf("DiscardedLen = %d, want the discarded byte count", tooLong.DiscardedLen)
	}

	// And a run of bytes that NEVER terminates must not grow the buffer
	// without bound either.
	a2 := NewFrameAccumulator(max, ctrl)
	if _, err := a2.Push(append([]byte{PreambleByte, PreambleByte}, make([]byte, max*4)...)); err == nil {
		t.Fatal("an unterminated run past the maximum was accepted")
	}
	// A fresh Push after a violation starts clean, exactly as core/cat's
	// accumulator does — the caller need not rebuild it.
	if got := pushAll(t, a2, good); len(got) != 1 {
		t.Fatalf("after a violation the accumulator returned %d frames for a good one", len(got))
	}
}

func TestAccumulator_DropsTheFirstEchoOfANotedSentFrame(t *testing.T) {
	const ctrl = 0xE0
	const radio = 0x94
	a := NewFrameAccumulator(0, ctrl)

	sent := frameFor(radio, ctrl, 0x19, 0x00)
	answer := frameFor(ctrl, radio, 0x19, 0x00, 0x94)

	a.NoteSent(sent)

	// The bus (or the USB adapter) hands our own frame back before the
	// answer. It must be dropped, and it must NOT be counted as
	// unexpected traffic even though its `to` byte is the radio's.
	got := pushAll(t, a, append(append([]byte{}, sent...), answer...))
	if len(got) != 1 || string(got[0]) != string(answer) {
		t.Fatalf("got % 02x, want only the answer % 02x", got, answer)
	}
	s := a.Stats()
	if s.Echoes != 1 {
		t.Fatalf("Stats().Echoes = %d, want 1", s.Echoes)
	}
	if s.Unexpected != 0 {
		t.Fatalf("Stats().Unexpected = %d, want 0 — an echo is not unexpected traffic", s.Unexpected)
	}
}

func TestAccumulator_DropsOnlyTheFIRSTEcho(t *testing.T) {
	const ctrl = 0xE0
	const radio = 0x94
	a := NewFrameAccumulator(0, ctrl)

	sent := frameFor(radio, ctrl, 0x19, 0x00)
	a.NoteSent(sent)

	// A SECOND byte-identical frame is not our echo: one write produced
	// one echo, and a repeat is real bus traffic. It falls through to the
	// address filter, which counts it as unexpected (its `to` is the
	// radio, not us) and does not return it.
	got := pushAll(t, a, append(append([]byte{}, sent...), sent...))
	if len(got) != 0 {
		t.Fatalf("got % 02x, want no frames", got)
	}
	s := a.Stats()
	if s.Echoes != 1 {
		t.Fatalf("Stats().Echoes = %d, want exactly 1", s.Echoes)
	}
	if s.Unexpected != 1 {
		t.Fatalf("Stats().Unexpected = %d, want 1 — the repeat is bus traffic, not an echo", s.Unexpected)
	}
}

func TestAccumulator_NoteSentCopiesAndDoesNotRetain(t *testing.T) {
	const ctrl = 0xE0
	const radio = 0x94
	a := NewFrameAccumulator(0, ctrl)

	sent := frameFor(radio, ctrl, 0x19, 0x00)
	caller := append([]byte(nil), sent...)
	a.NoteSent(caller)

	// The transport's contract (spec D2): NoteSent copies what it needs
	// and must not retain the passed slice. Mutating the caller's buffer
	// afterwards must not change which frame is recognised as the echo.
	for i := range caller {
		caller[i] = 0x00
	}

	got := pushAll(t, a, sent)
	if len(got) != 0 {
		t.Fatalf("got % 02x, want the echo dropped", got)
	}
	if s := a.Stats(); s.Echoes != 1 {
		t.Fatalf("Stats().Echoes = %d, want 1 — NoteSent retained the caller's slice", s.Echoes)
	}
}

func TestAccumulator_NotedSentFramesAreBounded(t *testing.T) {
	const ctrl = 0xE0
	a := NewFrameAccumulator(0, ctrl)

	// A radio that never echoes leaves every noted frame outstanding.
	// The list must not grow without bound: the oldest are forgotten.
	first := frameFor(0x94, ctrl, 0x19, byte(0))
	for i := 0; i < maxNotedSent*3; i++ {
		a.NoteSent(frameFor(0x94, ctrl, 0x19, byte(i)))
	}
	if n := a.notedLen(); n > maxNotedSent {
		t.Fatalf("noted %d sent frames, want at most %d", n, maxNotedSent)
	}
	// The FIRST one has been forgotten, so its late echo is ordinary
	// traffic rather than a silent drop.
	pushAll(t, a, first)
	if s := a.Stats(); s.Echoes != 0 {
		t.Fatalf("Stats().Echoes = %d, want 0 — a forgotten note must not still match", s.Echoes)
	}
}

func TestAccumulator_CountsButNeverReturnsTrafficAddressedElsewhere(t *testing.T) {
	const ctrl = 0xE0
	const radio = 0x94
	a := NewFrameAccumulator(0, ctrl)

	ours := frameFor(ctrl, radio, 0x19, 0x00, 0x94)
	broadcast := frameFor(0x00, radio, 0x00, 0x00, 0x50, 0x45, 0x14, 0x00)
	otherController := frameFor(0xE1, radio, 0x19, 0x00, 0x94)

	got := pushAll(t, a, broadcast, otherController, ours, broadcast)
	if len(got) != 1 || string(got[0]) != string(ours) {
		t.Fatalf("got % 02x, want only the frame addressed to us % 02x", got, ours)
	}
	s := a.Stats()
	if s.Unexpected != 3 {
		t.Fatalf("Stats().Unexpected = %d, want 3", s.Unexpected)
	}
	if s.Frames != 1 {
		t.Fatalf("Stats().Frames = %d, want 1", s.Frames)
	}
}

// TestAccumulator_SurvivesAContinuousFlood is the spec's Testing line: a
// transceive flood that NEVER goes quiet must not wedge or grow the
// accumulator, and must not hide the one frame addressed to us.
func TestAccumulator_SurvivesAContinuousFlood(t *testing.T) {
	const ctrl = 0xE0
	const radio = 0x94
	a := NewFrameAccumulator(0, ctrl)

	broadcast := frameFor(0x00, radio, 0x00, 0x00, 0x50, 0x45, 0x14, 0x00)
	ours := frameFor(ctrl, radio, AckByte)

	const rounds = 2000
	returned := 0
	for i := 0; i < rounds; i++ {
		chunk := append([]byte{}, broadcast...)
		if i == rounds/2 {
			chunk = append(chunk, ours...)
		}
		chunk = append(chunk, broadcast...)
		frames, err := a.Push(chunk)
		if err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		returned += len(frames)
	}
	if returned != 1 {
		t.Fatalf("a %d-round flood returned %d frames, want exactly the 1 addressed to us", rounds, returned)
	}
	if n := a.bufLen(); n != 0 {
		t.Fatalf("the accumulator retained %d bytes after a flood that always ended on a frame boundary", n)
	}
	if s := a.Stats(); s.Unexpected != rounds*2 {
		t.Fatalf("Stats().Unexpected = %d, want %d", s.Unexpected, rounds*2)
	}
}

func TestAccumulator_CountsRejectionsAndAcknowledgements(t *testing.T) {
	const ctrl = 0xE0
	const radio = 0x94
	a := NewFrameAccumulator(0, ctrl)

	nak := frameFor(ctrl, radio, NakByte)
	ack := frameFor(ctrl, radio, AckByte)

	got := pushAll(t, a, nak, ack, ack)
	if len(got) != 3 {
		t.Fatalf("got %d frames, want 3", len(got))
	}
	s := a.Stats()
	if s.Rejections != 1 || s.Acknowledgements != 2 {
		t.Fatalf("Stats() rejections=%d acks=%d, want 1 and 2", s.Rejections, s.Acknowledgements)
	}
}

func TestAccumulator_ReturnedFramesNeverAliasTheChunk(t *testing.T) {
	const ctrl = 0xE0
	a := NewFrameAccumulator(0, ctrl)

	f := frameFor(ctrl, 0x94, 0x19, 0x00, 0x94)
	chunk := append([]byte(nil), f...)
	got := pushAll(t, a, chunk)
	if len(got) != 1 {
		t.Fatalf("got %d frames, want 1", len(got))
	}
	for i := range chunk {
		chunk[i] = 0x00
	}
	if string(got[0]) != string(f) {
		t.Fatalf("the returned frame aliased the caller's chunk: % 02x", got[0])
	}
}

func TestAccumulator_HoldsASplitPreamblePairAcrossReads(t *testing.T) {
	const ctrl = 0xE0
	a := NewFrameAccumulator(0, ctrl)
	f := frameFor(ctrl, 0x94, AckByte)

	// The two preamble bytes arrive in different reads, with noise before
	// them. A scanner that discarded the lone trailing FE as noise would
	// lose the frame.
	got := pushAll(t, a, []byte{0x00, PreambleByte}, f[1:])
	if len(got) != 1 || string(got[0]) != string(f) {
		t.Fatalf("got % 02x, want % 02x", got, f)
	}
}

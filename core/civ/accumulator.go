// SPDX-License-Identifier: GPL-3.0-or-later

package civ

// DefaultMaxFrame is the maximum frame length, in bytes, a FrameAccumulator
// enforces when NewFrameAccumulator is given a non-positive maxFrame.
//
// A RESOURCE bound, not a protocol fact. The widest frame this tier
// expects is a memory set for the model with the widest record — the
// IC-705's 115 assumed bytes plus a three-byte address and seven bytes of
// framing, 125 — so 256 leaves twice that while still refusing to buffer a
// wedged line without limit. A profile declares its OWN maximum
// (Profile.MaxFrame), and that is what the transport passes here; this
// default exists for the callers that have no profile to hand.
const DefaultMaxFrame = 256

// maxNotedSent bounds how many un-echoed sent frames an accumulator
// remembers.
//
// The engine holds ONE outstanding request at a time (spec D2), so in
// normal operation at most one note is ever live. The bound is larger than
// that because Init and a drain may note a frame each in quick succession,
// and smaller than any figure at which a radio that never echoes could
// grow this list into a leak: a REMOTE-bus radio echoes and a
// direct-USB one may not, and the accumulator cannot tell which it is
// talking to. When the list is full the OLDEST note is forgotten, which is
// the safe direction to lose one in — a forgotten note means a late echo
// is reported as ordinary traffic (counted, never matched), where the
// alternative, an unbounded list, means an old note silently swallowing a
// genuine answer that happens to be byte-identical.
const maxNotedSent = 8

// AccumulatorStats is one accumulator's running tally, for diagnostics.
//
// Every field is a COUNT, never a reason to change behaviour: the counts
// are what a session's diagnostics report, and what the tests here assert
// on so that "tolerated" and "silently dropped" are distinguishable.
type AccumulatorStats struct {
	// Frames is the number of complete frames RETURNED to the caller —
	// after echo removal and after the address filter.
	Frames int
	// Echoes is the number of frames dropped because they byte-equalled a
	// frame NoteSent had recorded.
	Echoes int
	// Unexpected is the number of complete frames whose `to` byte was not
	// the controller's address: transceive broadcasts (to = 0x00) and
	// traffic between other stations on the bus. They are counted here and
	// never returned — see Push.
	Unexpected int
	// Rejections and Acknowledgements count the FA and FB frames among
	// those RETURNED.
	Rejections       int
	Acknowledgements int
	// NoiseBytes is the number of bytes discarded before a preamble pair —
	// line noise, or the tail of a frame whose start was missed.
	NoiseBytes int
	// Truncated is the number of partial frames abandoned because a new
	// preamble arrived before their terminator.
	Truncated int
}

// FrameAccumulator reassembles CI-V frames — PreambleByte PreambleByte …
// EndByte — from arbitrary stream chunks, dropping this program's own
// echoed frames and filtering out traffic addressed to anyone else.
//
// WHAT IT TOLERATES, and why each one is a real wire condition:
//
//   - LEADING NOISE. A session may open mid-frame, or the line may carry
//     rubbish before the radio speaks. Everything before the first
//     preamble pair is discarded and counted.
//   - EXTRA PREAMBLE PADDING. Icom radios may send more than two leading
//     PreambleByte. The frame is NORMALISED to exactly two, because
//     everything downstream — the gate, the codec's answer matcher, and
//     echo comparison against a frame this package BUILT — works on
//     canonical frames, and a padded copy that is byte-different from an
//     identical canonical one would defeat all three.
//   - TRUNCATION. A frame whose terminator never arrives, followed by a
//     new preamble, is abandoned at the resynchronisation point rather
//     than spliced into its successor.
//   - ECHO. On a REMOTE-bus radio, and through some USB adapters, a frame
//     this program writes comes straight back. The engine reports each
//     write through NoteSent before the port write, and the FIRST
//     subsequent byte-equal frame is dropped.
//   - BROADCASTS. Transceive is factory-ON on at least four models in
//     this tier and this tier ships no off-switch, so unsolicited frames
//     addressed to 0x00 (or to another controller) arrive constantly.
//     They are counted and NEVER RETURNED, which is how transceive is
//     tolerated without touching the radio's settings.
//
// The zero value is not usable; construct one with NewFrameAccumulator. A
// FrameAccumulator is not safe for concurrent use.
type FrameAccumulator struct {
	buf        []byte
	max        int
	controller byte
	noted      [][]byte
	stats      AccumulatorStats
}

// NewFrameAccumulator returns a FrameAccumulator that treats more than
// maxFrame accumulated bytes without completing an in-bound frame as
// stream contamination (see Push), and that returns only frames addressed
// to controller.
//
// maxFrame <= 0 selects DefaultMaxFrame. controller is the address this
// program answers to — Profile.ControllerAddress(), which is
// ControllerAddressDefault unless a profile says otherwise.
func NewFrameAccumulator(maxFrame int, controller byte) *FrameAccumulator {
	if maxFrame <= 0 {
		maxFrame = DefaultMaxFrame
	}
	return &FrameAccumulator{max: maxFrame, controller: controller}
}

// NoteSent records that frame has just been written to the port, so the
// first byte-equal frame read back can be recognised as this program's own
// echo rather than as an answer.
//
// It COPIES what it needs and does not retain the caller's slice — the
// transport seam's contract, and a real requirement rather than a nicety:
// the engine's write buffer is reused, so a retained slice would be
// compared against whatever the NEXT write put there.
//
// Call it BEFORE the port write. A radio on a REMOTE bus can echo faster
// than the writing goroutine returns, and a note that arrives after its
// own echo is a note that never matches.
func (a *FrameAccumulator) NoteSent(frame []byte) {
	if len(frame) == 0 {
		return
	}
	if len(a.noted) >= maxNotedSent {
		// Forget the oldest. See maxNotedSent for why that is the safe
		// direction.
		a.noted = append(a.noted[:0], a.noted[1:]...)
	}
	a.noted = append(a.noted, copyBytes(frame))
}

// Stats returns a snapshot of this accumulator's counters.
func (a *FrameAccumulator) Stats() AccumulatorStats { return a.stats }

// notedLen and bufLen exist for this package's own tests, which assert
// that neither list grows without bound under a flood or a radio that
// never echoes.
func (a *FrameAccumulator) notedLen() int { return len(a.noted) }
func (a *FrameAccumulator) bufLen() int   { return len(a.buf) }

// Push appends chunk to the accumulator's buffer and extracts every
// complete frame now available that is addressed to this controller and is
// not an echo of a noted sent frame.
//
// Each returned frame is an independent, CANONICAL copy: exactly two
// preamble bytes, whatever the radio padded with, and never aliasing
// chunk, an earlier chunk, or the accumulator's own buffer.
//
// BOUNDED ACCUMULATION, on core/cat's terms and for the same reason. Once
// more than maxFrame bytes have accumulated without forming an in-bound
// frame — whether because no terminator arrived, or because one arrived
// only after the frame had grown past the bound — Push:
//
//   - returns any complete frames found BEFORE the violation (as with
//     io.Reader, process a non-empty frames slice even when err != nil);
//   - discards everything from the violation to the end of the buffer,
//     including bytes that might look like valid frames: once a stream is
//     known contaminated, resynchronising on what merely resembles a frame
//     boundary is not safe;
//   - returns a *FrameTooLongError recording how many bytes were
//     discarded (errors.Is-compatible with ErrFrameTooLong);
//   - resets its buffer, so the next Push starts clean without the caller
//     rebuilding it.
func (a *FrameAccumulator) Push(chunk []byte) (frames [][]byte, err error) {
	buf := append(a.buf, chunk...)
	i := 0

	for {
		p := indexPreamblePair(buf, i)
		if p < 0 {
			// No preamble pair yet. A LONE trailing preamble byte may be
			// the first half of a pair split across reads, so it is kept;
			// everything else from i is noise.
			keep := len(buf)
			if keep > 0 && buf[keep-1] == PreambleByte {
				keep--
			}
			if keep < i {
				keep = i
			}
			a.stats.NoiseBytes += keep - i
			if len(buf)-keep > a.max {
				// Cannot happen with the single retained byte above, but
				// the bound is asserted rather than assumed.
				a.buf = nil
				return frames, &FrameTooLongError{DiscardedLen: len(buf) - keep}
			}
			a.buf = copyBytes(buf[keep:])
			return frames, nil
		}
		a.stats.NoiseBytes += p - i

		// Skip the whole preamble run: the body starts after the LAST
		// PreambleByte, and everything before it is padding.
		last := p
		for last+1 < len(buf) && buf[last+1] == PreambleByte {
			last++
		}
		if last+1 >= len(buf) {
			// The run reaches the end of the buffer; wait for the body.
			// Retain a canonical two-byte preamble plus nothing else, so a
			// pathological run of preamble bytes cannot itself grow the
			// buffer.
			a.buf = []byte{PreambleByte, PreambleByte}
			return frames, nil
		}

		end := -1
		resync := -1
		for j := last + 1; j < len(buf); j++ {
			// Candidate canonical length: two preamble bytes plus the
			// bytes from last+1 through j INCLUSIVE — that is, the frame
			// this would be if buf[j] turned out to be the terminator,
			// which is exactly the frame the branch below emits.
			//
			// THE `+1` THAT IS NOT HERE IS THE POINT. Counting the
			// terminator a SECOND time would refuse a frame of length
			// exactly max, while builders.go permits len(frame) ==
			// p.maxFrame and V9 permits MaxFrame == need. A profile whose
			// MaxFrame was computed exactly would then build a set its own
			// gate admits and its own accumulator discards as
			// contamination — the precise failure V9's message says it
			// exists to prevent — and could never read back the answer it
			// wrote. If buf[j] is NOT the terminator the frame is longer,
			// and the next iteration bounds it.
			if 2+(j-last) > a.max {
				discarded := len(buf) - p
				a.buf = nil
				return frames, &FrameTooLongError{DiscardedLen: discarded}
			}
			if buf[j] == EndByte {
				end = j
				break
			}
			if buf[j] == PreambleByte {
				resync = j
				break
			}
		}

		if resync >= 0 {
			a.stats.Truncated++
			i = resync
			continue
		}
		if end < 0 {
			// Incomplete. Retain from the canonical preamble onwards so
			// the next Push resumes exactly here.
			rest := make([]byte, 0, 2+len(buf)-(last+1))
			rest = append(rest, PreambleByte, PreambleByte)
			rest = append(rest, buf[last+1:]...)
			if len(rest) > a.max {
				a.buf = nil
				return frames, &FrameTooLongError{DiscardedLen: len(rest)}
			}
			a.buf = rest
			return frames, nil
		}

		frame := make([]byte, 0, 2+(end-last))
		frame = append(frame, PreambleByte, PreambleByte)
		frame = append(frame, buf[last+1:end+1]...)
		i = end + 1

		if len(frame) < minFrameLen {
			// "FE FE FD" and friends: a terminator with no body. Not a
			// frame, not noise worth resynchronising on — count the bytes
			// and move past it.
			a.stats.NoiseBytes += len(frame)
			continue
		}
		if a.takeEcho(frame) {
			a.stats.Echoes++
			continue
		}
		if frame[2] != a.controller {
			a.stats.Unexpected++
			continue
		}

		a.stats.Frames++
		switch {
		case IsRejection(frame):
			a.stats.Rejections++
		case IsAcknowledgement(frame):
			a.stats.Acknowledgements++
		}
		frames = append(frames, frame)
	}
}

// takeEcho reports whether frame byte-equals a noted sent frame, removing
// that note if so. Only the FIRST match is consumed: one write produces at
// most one echo, and a second identical frame is real bus traffic.
func (a *FrameAccumulator) takeEcho(frame []byte) bool {
	for i, n := range a.noted {
		if string(n) == string(frame) {
			a.noted = append(a.noted[:i], a.noted[i+1:]...)
			return true
		}
	}
	return false
}

// indexPreamblePair returns the index of the first PreambleByte of the
// first preamble PAIR at or after from, or -1.
func indexPreamblePair(buf []byte, from int) int {
	for j := from; j+1 < len(buf); j++ {
		if buf[j] == PreambleByte && buf[j+1] == PreambleByte {
			return j
		}
	}
	return -1
}

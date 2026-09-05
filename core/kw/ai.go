// SPDX-License-Identifier: GPL-3.0-or-later

package kw

// AI IS THE ONE COMMAND IN THIS CODEC WITH A STATE BYTE AND NO WAY TO ASK
// FOR ANY STATE BUT ONE.
//
// The books' value sets DIFFER. The 590 pair print "0: AI OFF / 2: AI ON
// (without backup) / 4: AI ON (with backup)" and no 1 or 3 at all
// (590:159-162); the 480 prints "0: AI OFF / 1: Only old AI format is ON /
// 2: Only extended AI format is ON / 3: Both formats are ON" (480:185-190).
// So there is no single non-zero value that means the same thing on both
// radios, and a builder taking a state byte would have to carry a per-book
// legend to keep a caller from asking one radio for the other's setting.
//
// It would also have nothing to build for. Every non-zero value in either
// set turns Auto Information ON, and an AI-ON radio pushes frames nobody
// asked for — a response per changed parameter on the 590 pair
// (590:165-167), an IF frame every 1.5 s on the 480 while the IF parameters
// change (480:194-195) — into a session that correlates answers by prefix
// and length. This programme therefore builds exactly one Set, "AI0;", and
// the API is what enforces that: there is no parameter to pass a 2 or a 4
// to. TestAI_NoOtherStateIsEverBuilt asserts the surface rather than a
// refusal inside it.

// The AI frame lengths, counted off each book's printed position ruler.
const (
	// AIReadLen is "A I ;" (590:164, 480:193).
	AIReadLen = 3
	// AISetLen is "A I P1 ;" (590:160, 480:189) — and the same four bytes
	// are its Answer (590:168, 480:197), which is why the outbound gate
	// admits exactly the one Set frame this codec builds and no other
	// four-byte AI frame.
	AISetLen = 4
)

// aiReadFrame is the whole of the AI Read chart on both radios.
const aiReadFrame = "AI;"

// BuildAIRead builds the Auto Information read, "AI;" (590:164, 480:193).
//
// NOTHING IN THIS MILESTONE SENDS IT: the session writes AI0; at open and
// never asks what the state was. It exists for the reason BuildMCRead does —
// the frame is printed on both radios and the gate admits it, and a gate
// admitting a frame no builder can produce is a gate nothing pins.
func (l Layout) BuildAIRead() (Command, error) {
	return l.buildFixedFrame("AI read", aiReadFrame, AIReadLen)
}

// BuildAISetOff builds the ONE Auto Information Set this codec has: "AI0;",
// "0: AI OFF" (590:159-160, 480:185-189).
//
// IT RETURNS THE SAME DATUM framing.InitSequence WRITES, not a second copy
// of it: initFrame is the constant both read. A session that had disabled
// Auto Information with one spelling and re-disabled it with another would
// still work, but the day one of the two was edited the other would be a
// silently different frame, and the failure — a radio pushing unsolicited
// frames into a prefix-matched session — is exactly the one this frame
// exists to prevent.
func (l Layout) BuildAISetOff() (Command, error) {
	return l.buildFixedFrame("AI set", initFrame, AISetLen)
}

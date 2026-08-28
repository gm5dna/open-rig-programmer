// SPDX-License-Identifier: GPL-3.0-or-later

// Package civ_test's tier file is THE ONE PLACE the six Icom models'
// record shapes are compared with each other, and it is deliberately not
// in any model's own package: no per-model worktree may claim a
// cross-model distinctness property, and every one of the six drivers'
// doc comments says so in its own words (core/driver/ic9700/ic9700.go's
// probeFingerprint: "Cross-model record-length distinctness is a
// TIER-level Wave-4 check needing a registry-wide table of accepted
// lengths"; core/driver/ic7300mk2/doc.go's "The wrong-sibling
// fingerprint" section: "Cross-model record-length distinctness is a
// TIER-level check belonging to registration"). This file is that check.
//
// It lives in core/civ as an EXTERNAL test package. core/civ can never
// import core/civ/ic7610 and its five siblings — they import core/civ,
// and the import would be a cycle — but a `package civ_test` file
// compiles separately and may, which is the same escape hatch
// core/civ/civtest's own tests use.
//
// # What the probe actually uses the length for
//
// EVERY ONE OF THE SIX DRIVERS PROBES THE ADDRESS FIRST. Open sends
// `19 00` and requires an ADDRESS-MATCHED reply (the value is
// undocumented on all six documents, spec D5 entry 7, so it is recorded
// and never compared), and only then walks a bounded run of memory
// channels for a record whose length confirms the profile. The six CI-V
// addresses are already distinct — 98h, 94h, B6h, A4h, A2h, ACh — so IN
// THE FIELD a wrong radio on the port does not answer at all and the open
// times out. THE LENGTH FINGERPRINT IS THEREFORE DEFENCE IN DEPTH, NOT
// THE PRIMARY DISCRIMINATOR: it protects against SAME-ADDRESS confusion
// only — a radio moved onto this address, or a bus mis-set — which is
// exactly what core/driver/ic7300mk2/doc.go and core/driver/ic905's
// Open comment both already say.
//
// # The limitation this file records honestly
//
// TWO OF THE SIX SHARE A RECORD-ONLY LENGTH. The IC-705 and the IC-9700
// both accept exactly {111}, so a fingerprint that compared record-only
// lengths ALONE could not tell those two apart. What separates them is
// the ADDRESS WIDTH — four bytes against three — and it does so in TWO
// independent places, of which only the first ever fires.
//
// THE FIRST REFUSAL IS THE ADDRESS, NOT THE LENGTH, and the ORDER inside
// civ.Profile.MemoryAnswerRecord (core/civ/parse.go:96) is what decides
// that: it takes the profile's OWN address width off the front of the
// data area, runs decodeAddress — and so validAddress
// (core/civ/profile.go:316) — at parse.go:105, and only reaches
// AcceptsRecordLength at parse.go:110 if that succeeded. THE TWO ADDRESS
// GEOMETRIES ARE MUTUALLY UNDECODABLE: read four bytes off a
// (band, channel) address and the leading `01`/`02`/`03` band byte lands
// in the wide form's two-byte group field, giving a group of 100, 200 or
// 300 and a channel of 100 or more; read three bytes off a
// (group, channel) address and the wide form's leading `00` becomes a
// band of 0, which the IC-9700 numbers from 1. Neither is in range, in
// either direction, for ANY address either radio has —
// TestTierRecordShapes_705And9700 sweeps all 321 of the IC-9700's and
// all 10,100 of the IC-705's and gets a *civ.ParseError every time, and
// a *civ.RecordLengthError never.
//
// THE LENGTH ARITHMETIC IS THE SECOND REFUSAL, and it is genuinely
// independent — a 114-byte data area stripped by four is a 110-byte
// record and a 115-byte one stripped by three is 112, neither of which
// the other profile accepts — but the address check pre-empts it in
// practice, so nothing observable ever reaches it. The test pins that
// arithmetic separately, as arithmetic, rather than claiming it is what
// fires.
//
// THE CONSEQUENCE IS IN ONE DRIVER, and only one. Of this pair, only
// core/driver/ic705/ic705.go:360-367 branches on errors.As(&lenErr) to
// mint a driver.WrongRadioError, and FOR THIS PAIR THAT BRANCH CAN NEVER
// FIRE: an IC-9700 moved onto A4h fails an IC-705 open with an
// unattributed address parse error, not with "wrong radio". The open
// still fails, which is the safety property, but the diagnosis a user
// sees names no model — a reporting limitation, recorded rather than
// papered over, and NOT a hole in the refusal.
//
// THE IC-9700 MINTS NO driver.WrongRadioError AT ALL, for any pair, BY
// DESIGN — not merely because this pair pre-empts it.
// core/driver/ic9700/ic9700.go's probeFingerprint wraps every error from
// MemoryAnswerRecord generically, its doc.go says so in as many words
// ("No driver.WrongRadioError is ever minted here"), and that package's
// TestUnexpectedLengthIsRefusedWithoutAttribution pins it: naming a model
// from {111} would be the cross-model claim no per-model worktree may
// make, which is the reason this file exists. Nothing here changes that,
// and this file mints nothing of its own.
//
// The three drivers that DO mint one on a length mismatch are the IC-7300
// and IC-7300MK2 (each with a one-entry sibling hint for the other,
// rendered *provisional*) and the IC-905 (whose attribution comes from a
// caller-supplied WithSiblingRecordLengths table — which no registered
// composition passes, so its refusal today populates neither model field
// and renders as the ID-only form).
//
// AND EVERY NUMBER HERE IS AN ASSUMED DERIVATION. No document in this
// tier prints a record total; each per-model package derives its length
// from printed field widths, and no radio has ever answered a frame to
// confirm one (each model's own register entry carries the lift). This
// file compares what those derivations say about one another; it does not
// turn six assumptions into a measurement.
package civ_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
)

// tierShape is one model's measured record geometry: the two properties
// a CI-V memory answer can be told apart by, and the wire figure they
// combine into.
type tierShape struct {
	model string
	// addressBytes is the wire width of the address field ahead of the
	// record. MEASURED, not declared: civ.AddressForm.addressBytes is
	// unexported and unreachable from here, so measureShape reads it off
	// two frames this profile builds.
	addressBytes int
	// recordLengths is the profile's accepted RECORD-ONLY length set,
	// ascending — spec D1's set, and spec D3.2's length fingerprint in
	// one value (civ.Profile.RecordLengths).
	recordLengths []int
	// frameLengths is recordLengths with addressBytes added to each: the
	// `1A 00` DATA AREA a radio's answer carries, which is the figure a
	// human reading a wire capture sees and the one the two 111-byte
	// models differ in.
	frameLengths []int
}

// tierProfiles is every Icom profile this project registers, in
// registration order (internal/wiring's realDrivers table).
//
// SIX ROWS, and TestTierRecordShapes_PairwiseDistinguishable insists on
// six: a seventh model registered without a row here would be compared
// against nothing, which is the one way this check could pass while
// meaning less than it says.
func tierProfiles() []civ.Profile {
	return []civ.Profile{
		ic7610.Profile(),
		ic7300.Profile(),
		ic7300mk2.Profile(),
		ic705.Profile(),
		ic9700.Profile(),
		ic905.Profile(),
	}
}

// measureShape reads one profile's shape through the EXPORTED API alone,
// hard-coding no number this file has not derived from the profile in
// hand.
//
// THE ADDRESS WIDTH IS THE DIFFERENCE BETWEEN TWO FRAMES. `19 00` and
// `1A 00 <address>` share their entire framing overhead — FE FE, the two
// address bytes, two command bytes and FD — so the whole of the length
// difference between them is the address field. That is a measurement of
// what this profile PUTS ON THE WIRE, which is the thing that matters
// here; a switch over civ.AddressForm's exported constants would only be
// this file restating record.go's own table back at it.
func measureShape(t *testing.T, p civ.Profile) tierShape {
	t.Helper()
	idRead, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("%s: BuildTransceiverIDRead: %v", p.Model(), err)
	}
	// The first addressable channel of the first group this radio has:
	// GroupBase is the WIRE index of that group (0 for the five
	// zero-based models, 1 for the IC-9700's A/B/C bands) and a flat
	// profile refuses any group but 0, which GroupBase already is.
	lo, _ := p.ChannelRange()
	memRead, err := p.BuildMemoryRead(civ.ChannelAddress{Group: p.GroupBase(), Channel: lo})
	if err != nil {
		t.Fatalf("%s: BuildMemoryRead(first channel): %v", p.Model(), err)
	}
	addressBytes := len(memRead.Bytes()) - len(idRead.Bytes())
	if addressBytes <= 0 {
		t.Fatalf("%s: measured address width %d — a `1A 00` read must be longer than a `19 00` read by exactly its address field", p.Model(), addressBytes)
	}
	lengths := p.RecordLengths()
	if len(lengths) == 0 {
		t.Fatalf("%s: RecordLengths() is empty — a profile accepting no record length fingerprints nothing", p.Model())
	}
	frames := make([]int, len(lengths))
	for i, n := range lengths {
		frames[i] = n + addressBytes
	}
	return tierShape{
		model:         p.Model(),
		addressBytes:  addressBytes,
		recordLengths: lengths,
		frameLengths:  frames,
	}
}

// distinguishable reports whether two shapes can be told apart by the two
// properties spec D3.2's probe actually has in hand, and says which one
// did it.
//
// THE ORDER IS THE PROBE'S ORDER. A disjoint accepted-length set is the
// strong form: the fingerprint alone separates the pair, with no appeal
// to the address. Only when the sets overlap does the address width carry
// the separation — which is a weaker claim, since it holds on the WIRE
// (through MemoryAnswerRecord's strip) rather than in the length
// comparison itself, and the reason string says so.
func distinguishable(a, b tierShape) (bool, string) {
	if disjointLengths(a.recordLengths, b.recordLengths) {
		return true, fmt.Sprintf("accepted record-only lengths are disjoint (%v / %v)", a.recordLengths, b.recordLengths)
	}
	if a.addressBytes != b.addressBytes {
		return true, fmt.Sprintf("record-only lengths overlap (%v / %v), separated by ADDRESS WIDTH (%d / %d) and so by wire data area (%v / %v)", a.recordLengths, b.recordLengths, a.addressBytes, b.addressBytes, a.frameLengths, b.frameLengths)
	}
	return false, fmt.Sprintf("record-only lengths overlap (%v / %v) at address width %d — neither property separates this pair", a.recordLengths, b.recordLengths, a.addressBytes)
}

// disjointLengths reports whether two ascending length sets share no
// member.
func disjointLengths(x, y []int) bool {
	for _, n := range x {
		for _, m := range y {
			if n == m {
				return false
			}
		}
	}
	return true
}

// recordedLimitations names pairs this project has ADJUDICATED as
// indistinguishable and accepted, keyed "MODEL-A|MODEL-B" in
// tierProfiles order.
//
// IT IS EMPTY, and that is a measurement rather than an aspiration: as of
// the tier close no registered pair collides. It exists so that a
// pre-existing design fact discovered later can be RECORDED HONESTLY
// instead of failing a suite that would then be quietly disabled —
// putting a pair here is an admission written down, not a licence, and
// the pair is still logged on every run. A NEW collision, from a model
// registered after this file was written, fails: that is a regression in
// the tier's shape, not a fact about a radio.
var recordedLimitations = map[string]string{}

// TestTierRecordShapes_PairwiseDistinguishable is the tier-close check:
// the six registered Icom profiles' (address width, accepted record-only
// length set) tuples are pairwise distinguishable.
//
// It prints the measured table on every run, pass or fail. The table is
// the deliverable as much as the verdict is — six models' record
// geometry in one place is exactly what no per-model worktree could
// write down.
func TestTierRecordShapes_PairwiseDistinguishable(t *testing.T) {
	profiles := tierProfiles()
	if len(profiles) != 6 {
		t.Fatalf("tierProfiles() has %d rows, want the 6 registered Icom models — a model missing here is compared against nothing", len(profiles))
	}
	shapes := make([]tierShape, len(profiles))
	for i, p := range profiles {
		shapes[i] = measureShape(t, p)
	}

	t.Log("measured record shapes (every figure read off the profile, none hard-coded here):")
	t.Logf("  %-12s %-14s %-18s %s", "MODEL", "ADDRESS BYTES", "RECORD-ONLY", "WIRE DATA AREA")
	for _, s := range shapes {
		// The length sets are rendered to a string FIRST: a width verb
		// applied to a slice pads every ELEMENT of it, which turns a
		// column meant to line up into "[25                ]".
		t.Logf("  %-12s %-14d %-18s %s", s.model, s.addressBytes, fmt.Sprint(s.recordLengths), fmt.Sprint(s.frameLengths))
	}
	t.Log("HONESTLY RECORDED: every length above is an ASSUMED derivation from printed field widths — no document in this tier prints a record total and no radio has confirmed one — and the length fingerprint is DEFENCE IN DEPTH behind the `19 00` address probe every one of the six drivers runs first, since the six CI-V addresses are already distinct and a wrong radio simply does not answer.")

	seen := 0
	for i := 0; i < len(shapes); i++ {
		for j := i + 1; j < len(shapes); j++ {
			seen++
			a, b := shapes[i], shapes[j]
			ok, why := distinguishable(a, b)
			if ok {
				t.Logf("%s vs %s: %s", a.model, b.model, why)
				continue
			}
			key := a.model + "|" + b.model
			if note, recorded := recordedLimitations[key]; recorded {
				t.Logf("LIMITATION (recorded, not a regression) %s vs %s: %s — %s", a.model, b.model, why, note)
				continue
			}
			t.Errorf("%s vs %s: %s — a pair the probe cannot separate is a radio this program could open as the wrong model; if this is a fact about the radios rather than a mistake in a profile, record it in recordedLimitations with the adjudication that accepted it", a.model, b.model, why)
		}
	}
	if want := 15; seen != want {
		t.Errorf("compared %d pairs, want %d — the pairwise walk is broken and this test passed vacuously", seen, want)
	}
}

// TestTierRecordShapes_705And9700 pins the ONE pair that shares a
// record-only length, and pins WHERE its separation comes from.
//
// A test that only asserted "all pairs distinguishable" would go on
// passing if the IC-705's address form were widened to the IC-9700's:
// the sets would still overlap, the widths would now agree, and the
// generic loop would report the collision — but nothing would say that
// this pair's separation was ever supposed to be the address width in
// the first place. This does.
func TestTierRecordShapes_705And9700(t *testing.T) {
	p705 := measureShape(t, ic705.Profile())
	p9700 := measureShape(t, ic9700.Profile())

	if !equalLengths(p705.recordLengths, p9700.recordLengths) {
		t.Fatalf("IC-705 record-only lengths %v, IC-9700 %v — this test exists BECAUSE the two share a length; if they no longer do, the plan's premise has changed and this test should be rewritten rather than relaxed", p705.recordLengths, p9700.recordLengths)
	}
	if disjointLengths(p705.recordLengths, p9700.recordLengths) {
		t.Fatalf("IC-705 %v and IC-9700 %v are disjoint after comparing equal — disjointLengths is broken", p705.recordLengths, p9700.recordLengths)
	}
	if p705.addressBytes == p9700.addressBytes {
		t.Fatalf("IC-705 and IC-9700 both address a channel in %d bytes AND accept the same record-only lengths %v — the ONLY thing separating these two radios has gone", p705.addressBytes, p705.recordLengths)
	}
	t.Logf("IC-705 and IC-9700 share record-only %v; address widths %d and %d separate them", p705.recordLengths, p705.addressBytes, p9700.addressBytes)

	// And the separation reaches the wire: the figure that actually
	// differs between these two radios' answers is the `1A 00` data area.
	if !disjointLengths(p705.frameLengths, p9700.frameLengths) {
		t.Errorf("IC-705 wire data areas %v and IC-9700's %v are not disjoint — with the record-only lengths equal, an answer from one radio would fingerprint as the other's", p705.frameLengths, p9700.frameLengths)
	}
	t.Logf("wire data areas: IC-705 %v, IC-9700 %v — disjoint, so an answer from one is refused by the other's profile", p705.frameLengths, p9700.frameLengths)

	// THE SECOND REFUSAL, PINNED AS ARITHMETIC. Strip each profile's own
	// address width off the OTHER's data area and the remainder is not a
	// record either profile accepts. This is a real and independent
	// property — but it is not the one that fires, and the sweep below is
	// what establishes which does.
	for _, tc := range []struct {
		reader   civ.Profile
		dataArea int
		width    int
	}{
		{ic705.Profile(), p9700.frameLengths[0], p705.addressBytes},
		{ic9700.Profile(), p705.frameLengths[0], p9700.addressBytes},
	} {
		remainder := tc.dataArea - tc.width
		if tc.reader.AcceptsRecordLength(remainder) {
			t.Errorf("%s accepts a record of %d bytes (%d-byte data area less its own %d-byte address) — the length arithmetic no longer separates this pair either", tc.reader.Model(), remainder, tc.dataArea, tc.width)
		}
		// AND THE GATE IS REACHABLE, which is what stops the sweep below
		// passing for the wrong reason. Give this profile its OWN address
		// geometry — so answerBody and decodeAddress both succeed — with
		// the foreign remainder as the record, and the length check is
		// what refuses it. Without this leg a crossAnswer frame that had
		// been rejected by answerBody's `from` check would also count as
		// a *civ.ParseError, and the sweep would report the wrong gate.
		selfAddr := civ.ChannelAddress{Group: tc.reader.GroupBase(), Channel: firstChannel(tc.reader)}
		_, _, err := tc.reader.MemoryAnswerRecord(answerFrame(t, tc.reader, tc.reader, selfAddr, remainder))
		var lenErr *civ.RecordLengthError
		if !errors.As(err, &lenErr) {
			t.Errorf("%s reading a well-addressed answer carrying a %d-byte record: err = %v (%T), want a *civ.RecordLengthError — the length gate is unreachable, so the sweep below cannot say which gate refused", tc.reader.Model(), remainder, err, err)
			continue
		}
		if lenErr.Got != remainder {
			t.Errorf("%s: RecordLengthError.Got = %d, want %d", tc.reader.Model(), lenErr.Got, remainder)
		}
		t.Logf("%s refuses a well-addressed %d-byte record with %v — the second gate is real and reachable", tc.reader.Model(), remainder, lenErr)
	}

	// THE FIRST REFUSAL, MEASURED RATHER THAN DESCRIBED. Every address
	// either radio has, in both directions, exhaustively: 321 for the
	// IC-9700 (3 bands x channels 1..107) and 10,100 for the IC-705 (101
	// groups x channels 0..99). Each one becomes the `1A 00` answer that
	// radio would put on the wire IF IT HAD BEEN MOVED ONTO THE OTHER'S
	// CI-V ADDRESS — the same-address confusion spec D3.2's fingerprint
	// exists for, and the only circumstance in which these two can meet
	// at all, since each answers at its own distinct address and a wrong
	// one does not answer.
	//
	// The verdict is uniform and it is the ADDRESS: *civ.ParseError from
	// decodeAddress every time, *civ.RecordLengthError not once.
	sweep := func(t *testing.T, src, dst civ.Profile, addrs []civ.ChannelAddress) {
		t.Helper()
		if len(addrs) == 0 {
			t.Fatalf("%s -> %s: no addresses swept — this leg would pass vacuously", src.Model(), dst.Model())
		}
		parseErrs, lenErrs, accepted := 0, 0, 0
		for _, a := range addrs {
			_, _, err := dst.MemoryAnswerRecord(crossAnswer(t, src, dst, a))
			var lenErr *civ.RecordLengthError
			var parseErr *civ.ParseError
			switch {
			case err == nil:
				accepted++
				if accepted == 1 {
					t.Errorf("%s's answer for %+v is ACCEPTED by the %s profile — one radio's memory answer read as the other's, which is the whole failure this pair's separation exists to prevent", src.Model(), a, dst.Model())
				}
			case errors.As(err, &lenErr):
				lenErrs++
			case errors.As(err, &parseErr):
				parseErrs++
			default:
				t.Errorf("%s's answer for %+v refused by the %s profile with an unexpected error type %T: %v", src.Model(), a, dst.Model(), err, err)
			}
		}
		if accepted != 0 {
			t.Errorf("%s -> %s: %d of %d answers accepted, want 0", src.Model(), dst.Model(), accepted, len(addrs))
		}
		if parseErrs != len(addrs) {
			t.Errorf("%s -> %s: %d of %d refusals were address parse errors, want all of them", src.Model(), dst.Model(), parseErrs, len(addrs))
		}
		if lenErrs != 0 {
			t.Errorf("%s -> %s: %d refusals were *civ.RecordLengthError — the doc block and the drivers' WrongRadioError branches both say this pair never reaches the length check, and that is no longer true", src.Model(), dst.Model(), lenErrs)
		}
		t.Logf("%s answers read by the %s profile: %d addresses, %d address parse errors, %d length errors, %d accepted", src.Model(), dst.Model(), len(addrs), parseErrs, lenErrs, accepted)
	}

	var ic9700Addrs []civ.ChannelAddress
	lo9700, hi9700 := ic9700.Profile().ChannelRange()
	for band := ic9700.Profile().GroupBase(); band < ic9700.Profile().GroupBase()+ic9700.Profile().Groups(); band++ {
		for ch := lo9700; ch <= hi9700; ch++ {
			ic9700Addrs = append(ic9700Addrs, civ.ChannelAddress{Group: band, Channel: ch})
		}
	}
	var ic705Addrs []civ.ChannelAddress
	lo705, hi705 := ic705.Profile().ChannelRange()
	for g := ic705.Profile().GroupBase(); g < ic705.Profile().GroupBase()+ic705.Profile().Groups(); g++ {
		for ch := lo705; ch <= hi705; ch++ {
			ic705Addrs = append(ic705Addrs, civ.ChannelAddress{Group: g, Channel: ch})
		}
	}
	if len(ic9700Addrs) != 321 {
		t.Fatalf("swept %d IC-9700 addresses, want 321 (3 bands x 107 channels)", len(ic9700Addrs))
	}
	if len(ic705Addrs) != 10100 {
		t.Fatalf("swept %d IC-705 addresses, want 10,100 (101 groups x 100 channels)", len(ic705Addrs))
	}
	sweep(t, ic9700.Profile(), ic705.Profile(), ic9700Addrs)
	sweep(t, ic705.Profile(), ic9700.Profile(), ic705Addrs)
}

// crossAnswer builds the `1A 00` memory answer a radio with src's address
// geometry and record length would put on the wire IF IT HAD BEEN MOVED
// ONTO dst's CI-V address.
//
// THE MOVED ADDRESS IS THE POINT, and it is why this cannot just hand
// dst's parser a frame src built. The six CI-V addresses are distinct, so
// a frame carrying src's own `from` byte is refused by answerBody
// (core/civ/parse.go:31) before any geometry is looked at — a THIRD and
// even earlier refusal, and the one that operates in the field. What spec
// D3.2's length fingerprint is FOR is the case that gets past it: a radio
// moved onto this address, or a bus mis-set. So the address bytes and the
// record width are src's, and only the two frame address bytes are dst's.
//
// It delegates the framing to answerFrame, which every leg of this file
// shares.
func crossAnswer(t *testing.T, src, dst civ.Profile, addr civ.ChannelAddress) []byte {
	t.Helper()
	return answerFrame(t, src, dst, addr, src.BuildRecordLength())
}

// answerFrame builds a `1A 00` answer carrying SRC's address bytes for
// addr and a zeroed record of recordLen bytes, framed as arriving from
// DST's radio to DST's controller.
//
// A zeroed record because every test here asks WHICH GATE REFUSES, and
// under both refusals the record's contents reach no field decoder at
// all. The address field is taken from src's OWN read frame rather than
// re-encoded here: `1A 00 <address>` has a fixed seven-byte overhead, so
// the bytes between the sub-command and the terminator are exactly what
// src's encoder produced, and this helper invents no BCD of its own.
func answerFrame(t *testing.T, src, dst civ.Profile, addr civ.ChannelAddress, recordLen int) []byte {
	t.Helper()
	read, err := src.BuildMemoryRead(addr)
	if err != nil {
		t.Fatalf("%s: BuildMemoryRead(%+v): %v", src.Model(), addr, err)
	}
	body := read.Bytes()
	addrBytes := body[6 : len(body)-1]

	frame := make([]byte, 0, 7+len(addrBytes)+recordLen)
	frame = append(frame, civ.PreambleByte, civ.PreambleByte, dst.ControllerAddress(), dst.RadioAddress(), civ.CmdMemory, civ.SubMemoryContents)
	frame = append(frame, addrBytes...)
	frame = append(frame, make([]byte, recordLen)...)
	return append(frame, civ.EndByte)
}

// firstChannel is the lowest channel number a profile addresses.
func firstChannel(p civ.Profile) int {
	lo, _ := p.ChannelRange()
	return lo
}

// TestTierRecordShapes_CheckIsNotVacuous is the permanent red proof:
// distinguishable must REFUSE a pair the six real profiles never
// produce.
//
// Two shapes that agree on both properties are what the check is for, and
// a check that answered "distinguishable" to everything would have passed
// the loop above in silence. The pair here is a LOCAL FIXTURE — the
// frozen per-model profiles are not touched, and could not be: a
// civ.Profile is immutable and built from a compile-time literal.
func TestTierRecordShapes_CheckIsNotVacuous(t *testing.T) {
	twin := tierShape{model: "TWIN-A", addressBytes: 3, recordLengths: []int{111}, frameLengths: []int{114}}
	other := twin
	other.model = "TWIN-B"
	if ok, why := distinguishable(twin, other); ok {
		t.Errorf("distinguishable(identical shapes) = true (%s), want false — the pairwise check would pass vacuously", why)
	}

	// One byte of address width is enough to separate them, and nothing
	// else about the pair changed.
	wider := other
	wider.addressBytes = 4
	wider.frameLengths = []int{115}
	if ok, _ := distinguishable(twin, wider); !ok {
		t.Error("distinguishable(same lengths, different address widths) = false, want true — this is exactly the IC-705/IC-9700 case")
	}

	// So is a disjoint length set at the same address width.
	longer := other
	longer.recordLengths = []int{112}
	longer.frameLengths = []int{115}
	if ok, _ := distinguishable(twin, longer); !ok {
		t.Error("distinguishable(disjoint lengths, same address width) = false, want true")
	}
}

// equalLengths reports whether two ascending length sets are identical.
func equalLengths(x, y []int) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

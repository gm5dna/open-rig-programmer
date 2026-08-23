// SPDX-License-Identifier: GPL-3.0-or-later

// Package civtest is core/civ's conformance suite for a civ.Profile,
// expressed entirely through the EXPORTED API.
//
// WHY IT EXISTS. core/civ holds in-package walks that are the real safety
// property of this program — every frame a profile's builders emit is
// well-formed, is admitted by that profile's OWN gate, and is refused by
// every other profile's — and they drive allTestProfiles(), an in-package
// fixture. A per-model package such as core/civ/ic7300 imports core/civ,
// so it can never reach that fixture: the import would be a cycle. Without
// this package a new Icom model would arrive with no way to run the
// properties that matter, and "it compiles and its own unit tests pass"
// would be the whole of its evidence.
//
// So the subset of those properties that can be stated through exported
// identifiers lives here, as a function any package can call:
//
//	func TestConformance(t *testing.T) { civtest.Run(t, ic7300.Profile()) }
//
// It is a SUBSET, deliberately: several in-package checks read unexported
// helpers and cannot be restated. What this package can state, it states
// in full, and it COUNTS what it does so a profile that quietly
// contributes nothing fails loudly rather than passing in silence.
//
// WHY IT IS A NON-TEST FILE, AND WHY IT DOES NOT IMPORT "testing". The
// suite must be importable by ANOTHER package's tests, and a _test.go file
// is importable by nobody — so this is the deliberate net/http/httptest
// shape. Unlike httptest it imports only "sort" and core/civ: the T
// interface below is what keeps "testing" out, and it earns its keep twice
// over, because it is also the only way Run's refusal of an unconfigured
// profile can itself be tested (see T's own doc comment). Nothing in the
// production tree imports this package, and nothing should: its only
// callers are _test.go files, which is a guard rule (internal/guards)
// rather than a convention.
//
// WHY Run REFUSES THE ZERO PROFILE INSTEAD OF TESTING IT. "A zero Profile
// refuses everything" is a property this suite owns, but it is NOT what
// Run does when handed one. Run t.Fatal's, and RunZeroValue is a separate
// exported entry point. The reason is the vacuity trap: a model package
// whose exported profile was never initialised — a failed init, a typo
// selecting the wrong var — would call Run and, if Run silently switched
// to the refusal suite, receive a green conformance report for a radio it
// cannot describe. That is precisely the class of silent pass every walk
// in core/civ is written to prevent, so Run says what happened and names
// the other entry point instead.
package civtest

import (
	"sort"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

// T is the subset of *testing.T this suite uses. A *testing.T satisfies it,
// so callers write civtest.Run(t, p) exactly as they would against a
// *testing.T parameter.
//
// IT IS AN INTERFACE FOR ONE REASON, and it is a deliberate difference
// from core/cat/dialecttest, which takes a *testing.T. The vacuity trap
// this suite exists to close — Run REFUSING an unconfigured profile rather
// than quietly running the refusal suite over it — is the single most
// important thing about Run's contract, and with a concrete *testing.T
// there is no way to test it: testing.TB cannot be implemented outside the
// testing package (it has an unexported method), and a real t.Fatal fails
// the very test asserting the refusal. A four-method interface makes the
// property provable, and costs callers nothing.
type T interface {
	Helper()
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	Logf(format string, args ...any)
}

// minConformanceFrames is the floor on the total frame count for one
// profile. A radio this program can describe has a channel space and at
// least one record layout, so it produces dozens; the floor is set well
// below that because its job is to catch a walk that has stopped reaching
// the builders at all, not to police how many channels a model has.
const minConformanceFrames = 8

// maxAddressSamples bounds how many channels Run reads per group. A
// grouped model has 100 groups of 100 channels, and a suite that walked
// every one of them would spend its time proving the same property ten
// thousand times.
const maxAddressSamples = 5

// Run holds p to every conformance property this package can state through
// core/civ's exported API, and fails t with a self-describing message for
// each violation.
//
// Every helper here calls t.Helper(), so a failure is reported at the
// caller's Run(t, p) line rather than inside this package: the message
// carries the detail, and the line number that matters to a model
// package's author is their own.
//
// It is an ERROR to call Run with an unconfigured profile — see the
// package doc comment for why that is a fatal misuse rather than a silent
// switch to RunZeroValue.
func Run(t T, p civ.Profile) {
	t.Helper()

	if !p.Configured() {
		t.Fatal("civtest.Run was given an UNCONFIGURED profile (Configured() == false). " +
			"This is a misuse, not a conformance failure: an uninitialised package-level var reaches here " +
			"looking exactly like a radio, and reporting PASS for it is the silent green this suite exists " +
			"to prevent. If you meant to check the zero value's refusals, call civtest.RunZeroValue(t).")
	}

	r := &run{t: t, p: p, frames: map[string]int{}, refusals: map[string]int{}, lengthsSeen: map[int]int{}}

	r.checkProfileSelfConsistency()
	r.checkTransceiverIDRead()
	r.checkMemoryReads()
	r.checkMemorySets()
	r.checkEveryAcceptedLength()
	r.checkNameLengthIsTheProfilesOwn()
	r.checkGateRefusesTheUnacceptable()
	r.checkGateRefusesAMutatedRecordByte()
	r.checkNonVacuity()
}

// run is one Run's state: the profile under test, plus the counters that
// make every property non-vacuous.
type run struct {
	t T
	p civ.Profile

	// frames counts built-and-checked frames per builder; refusals counts
	// refusals a check needed to SEE, per kind. A silent skip and an
	// enforced rule are indistinguishable without the second map.
	frames   map[string]int
	refusals map[string]int
	total    int

	// lengthsSeen counts records packed and gated per accepted record
	// length, so a profile whose second layout was never reached fails
	// rather than passing on the strength of its first.
	lengthsSeen map[int]int

	// roundTrips counts records that survived build -> parse intact.
	roundTrips int
}

func (r *run) name() string { return r.p.Model() }

// checkFrame is the property the whole suite is built around: this frame
// is well-formed on the wire, fits this profile's own bound, and this
// profile's own gate admits it.
//
// A builder and a gate that disagree are worse than either being wrong
// alone: it means the program cannot send a command it believes is valid,
// or the gate is not checking what the builder emits.
func (r *run) checkFrame(what string, frame []byte) {
	r.t.Helper()

	r.frames[what]++
	r.total++

	if !civ.WellFormed(frame) {
		r.t.Errorf("%s: %s produced a frame that is not well-formed: %v", r.name(), what, frame)
		return
	}
	if to, _ := civ.FrameTo(frame); to != r.p.RadioAddress() {
		r.t.Errorf("%s: %s addressed its frame to %#02x, not to this profile's radio (%#02x)", r.name(), what, to, r.p.RadioAddress())
	}
	if from, _ := civ.FrameFrom(frame); from != r.p.ControllerAddress() {
		r.t.Errorf("%s: %s sent its frame from %#02x, not from this profile's controller (%#02x)", r.name(), what, from, r.p.ControllerAddress())
	}
	if len(frame) > r.p.MaxFrame() {
		r.t.Errorf("%s: %s produced %d bytes, past this profile's own %d-byte frame bound — its own accumulator would discard the exchange as contamination", r.name(), what, len(frame), r.p.MaxFrame())
	}
	if !r.p.AllowedCommand(frame) {
		r.t.Errorf("%s: its own gate REFUSED %s frame %v — a builder and a gate that disagree mean this profile cannot send a command it believes is valid", r.name(), what, frame)
	}
}

// checkProfileSelfConsistency holds the profile's own accessors to the
// invariants NewProfile promises, so a hand-built Profile (or a future
// constructor bug) cannot reach the rest of this suite.
func (r *run) checkProfileSelfConsistency() {
	r.t.Helper()
	p := r.p

	if p.RadioAddress() == p.ControllerAddress() {
		r.t.Errorf("%s: the radio and controller share address %#02x — echo removal and answer matching would both be undecidable", r.name(), p.RadioAddress())
	}
	lengths := p.RecordLengths()
	if len(lengths) == 0 {
		r.t.Fatalf("%s: RecordLengths() is empty — no record could be read or written", r.name())
	}
	found := false
	for _, n := range lengths {
		if n == p.BuildRecordLength() {
			found = true
		}
		if !p.AcceptsRecordLength(n) {
			r.t.Errorf("%s: AcceptsRecordLength(%d) is false for a length RecordLengths() reports", r.name(), n)
		}
	}
	if !found {
		r.t.Errorf("%s: BuildRecordLength() = %d is not among the accepted lengths %v — the builder would emit a record this profile's own parser refuses", r.name(), p.BuildRecordLength(), lengths)
	}
	// A length no layout claims must be refused, or the fingerprint the
	// probe reads (spec D3.2) would accept anything.
	unclaimed := lengths[len(lengths)-1] + 1
	if p.AcceptsRecordLength(unclaimed) {
		r.t.Errorf("%s: AcceptsRecordLength(%d) is true for a length no layout declares — the probe's length fingerprint would accept any record at all", r.name(), unclaimed)
	} else {
		r.refusals["unaccepted record length"]++
	}

	switch p.Discriminator() {
	case civ.DiscriminatorSingleLength:
		if len(lengths) != 1 {
			r.t.Errorf("%s: Discriminator is %v with %d accepted lengths", r.name(), p.Discriminator(), len(lengths))
		}
	case civ.DiscriminatorRecordLength:
		if len(lengths) < 2 {
			r.t.Errorf("%s: Discriminator is %v with %d accepted length", r.name(), p.Discriminator(), len(lengths))
		}
	default:
		r.t.Errorf("%s: Discriminator is %v — the zero value is not a rule", r.name(), p.Discriminator())
	}

	if p.AddressForm() == civ.AddressFormFlat && p.Groups() != 0 {
		r.t.Errorf("%s: a flat address form reports %d groups", r.name(), p.Groups())
	}
	if p.AddressForm() != civ.AddressFormFlat && p.Groups() < 1 {
		r.t.Errorf("%s: address form %v reports %d groups", r.name(), p.AddressForm(), p.Groups())
	}
	if p.NameLength() > 0 && len(p.NameCharset()) == 0 {
		r.t.Errorf("%s: NameLength() is %d with an empty charset — no name would be expressible", r.name(), p.NameLength())
	}
}

func (r *run) checkTransceiverIDRead() {
	r.t.Helper()

	cmd, err := r.p.BuildTransceiverIDRead()
	if err != nil {
		r.t.Errorf("%s: BuildTransceiverIDRead: %v", r.name(), err)
		return
	}
	frame := cmd.Bytes()
	r.checkFrame("transceiver ID read", frame)
	if cmd.String() == "" {
		r.t.Errorf("%s: the built Command's String() is empty — it is what a diagnostic line prints", r.name())
	}

	// Bytes() must hand out a fresh copy every call: the TOCTOU closure
	// the type exists for.
	a, b := cmd.Bytes(), cmd.Bytes()
	if len(a) > 0 && &a[0] == &b[0] {
		r.t.Errorf("%s: two Command.Bytes() calls returned the same backing array — a caller could mutate a frame after the gate approved it and before the port write", r.name())
	}
	a[len(a)-1] = 0x00
	if c := cmd.Bytes(); c[len(c)-1] != 0xFD {
		r.t.Errorf("%s: mutating a returned Command.Bytes() changed the Command", r.name())
	}

	// The ANSWER: the same frame with its addresses swapped and a data
	// byte appended. Built here rather than by a builder because this
	// package must not add a production builder no radio path needs.
	answer := swapAddresses(frame)
	answer = append(answer[:len(answer)-1], r.p.RadioAddress(), civ.EndByte)
	token, err := r.p.ParseTransceiverID(answer)
	if err != nil {
		r.t.Errorf("%s: ParseTransceiverID rejected the answer to its own read (%v): %v", r.name(), answer, err)
	} else if token == "" {
		r.t.Errorf("%s: ParseTransceiverID returned an empty token — spec D3.2 records it as a diagnostic, so it must carry something to record", r.name())
	}

	// And the answer must NOT be admitted by the gate: an answer is never
	// a legal outbound command.
	if r.p.AllowedCommand(answer) {
		r.t.Errorf("%s: its gate ADMITTED a transceiver-ID ANSWER %v — an answer is never a legal outbound command, and admitting one lets a captured reply be written back", r.name(), answer)
	} else {
		r.refusals["answer frame at the gate"]++
	}
}

func (r *run) checkMemoryReads() {
	r.t.Helper()

	for _, addr := range r.addresses() {
		cmd, err := r.p.BuildMemoryRead(addr)
		if err != nil {
			r.t.Errorf("%s: BuildMemoryRead(%v): %v", r.name(), addr, err)
			continue
		}
		r.checkFrame("memory read", cmd.Bytes())
	}

	// Out-of-space addresses must be refused, with the zero Command.
	_, hi := r.p.ChannelRange()
	bad := []civ.ChannelAddress{{Group: r.firstGroup(), Channel: hi + 1}}
	if lo, _ := r.p.ChannelRange(); lo > 0 {
		bad = append(bad, civ.ChannelAddress{Group: r.firstGroup(), Channel: lo - 1})
	}
	if r.p.AddressForm() != civ.AddressFormFlat {
		bad = append(bad, civ.ChannelAddress{Group: r.p.Groups(), Channel: hi})
	} else {
		bad = append(bad, civ.ChannelAddress{Group: 1, Channel: hi})
	}
	for _, addr := range bad {
		cmd, err := r.p.BuildMemoryRead(addr)
		if err == nil {
			r.t.Errorf("%s: BuildMemoryRead(%v) built %v for an address outside this profile's own space", r.name(), addr, cmd.Bytes())
			continue
		}
		if !cmd.IsZero() {
			r.t.Errorf("%s: BuildMemoryRead(%v) returned a non-zero Command alongside its error", r.name(), addr)
		}
		r.refusals["address outside the profile's space"]++
	}
}

// checkEveryAcceptedLength holds EVERY layout the profile declares to the
// round-trip and the gate, not just the one the builder emits.
//
// WHY IT IS SEPARATE FROM checkMemorySets. BuildMemorySet emits
// BuildRecordLength and no other length (core/civ's AllowedCommand doc
// comment argues that width at length), so a suite driven by the builder
// alone can never reach a second layout. For the one model in this tier
// with two record lengths — the IC-905, spec D6 — that meant civtest would
// certify a profile whose second layout had never been decoded by anything.
//
// The record bytes are packed HERE, from the profile's own exported layout
// data, rather than by core/civ. That is not a workaround for a missing
// builder: it makes the round trip a genuine cross-check rather than the
// codec agreeing with itself, because the gate's re-encode step compares
// these bytes against core/civ's own encoding of the record they decode
// to. A disagreement between the two encoders fails here loudly.
//
// The frame is assembled from the profile's OWN BuildMemoryRead output —
// that frame is `FE FE <radio> <ctrl> 1A 00 <address> FD`, so dropping its
// terminator and appending the record gives the set frame without this
// package having to know how an address is encoded.
func (r *run) checkEveryAcceptedLength() {
	r.t.Helper()
	p := r.p
	addr := r.addresses()[0]

	read, err := p.BuildMemoryRead(addr)
	if err != nil {
		r.t.Errorf("%s: BuildMemoryRead(%v): %v", r.name(), addr, err)
		return
	}
	prefix := read.Bytes()
	prefix = prefix[:len(prefix)-1]

	for _, length := range p.RecordLengths() {
		rec, ok := r.sampleRecord(addr, length)
		if !ok {
			continue
		}
		record, ok := r.packRecord(rec, length)
		if !ok {
			continue
		}
		frame := make([]byte, 0, len(prefix)+length+1)
		frame = append(frame, prefix...)
		frame = append(frame, record...)
		frame = append(frame, civ.EndByte)

		r.lengthsSeen[length]++

		if !civ.WellFormed(frame) {
			r.t.Errorf("%s: a %d-byte record packed from this profile's own layout is not a well-formed frame: %v", r.name(), length, frame)
			continue
		}
		if !p.AllowedCommand(frame) {
			r.t.Errorf("%s: its own gate REFUSED a set at accepted length %d, built from this profile's OWN layout data: %v — either the layout describes a record the profile cannot write, or its encoder and this suite's disagree about the bytes", r.name(), length, frame)
			continue
		}

		back, err := p.ParseMemoryAnswer(swapAddresses(frame))
		if err != nil {
			r.t.Errorf("%s: ParseMemoryAnswer refused a %d-byte record its own layout describes: %v", r.name(), length, err)
			continue
		}
		if back != rec {
			r.t.Errorf("%s: a %d-byte record did not survive pack -> parse:\n got %+v\nwant %+v", r.name(), length, back, rec)
			continue
		}
		r.roundTrips++
	}
}

// checkGateRefusesAMutatedRecordByte exercises the RE-ENCODE EQUALITY
// rule, the mechanism that makes "the gate admits only builder-producible
// frames" literal rather than approximate.
//
// Without it this suite could not tell a gate that re-encodes from one
// that checks each field in isolation: every other property here offers
// the gate frames a builder DID produce. So this one takes a frame the
// builder produced and alters a NIBBLE no span maps — a reserved byte, a
// documented constant from the layout's Fixed template — leaving every
// field it decodes to untouched. Field-by-field validation admits that
// frame; a re-encode refuses it, because the encoder would have written
// the template's value there.
//
// PER NIBBLE, NOT PER BYTE, because core/civ's V8 lets a layout map one
// nibble of a byte while its Fixed template speaks for the other. A
// whole-byte view counts such a byte as mapped and never mutates the
// constant beside the enum — exactly the freedom a model package is most
// likely to be the first to use.
//
// A profile whose layout maps every nibble has nothing to mutate and is
// SKIPPED, loudly: the log line says so, and checkNonVacuity does not
// require the counter, because "this model's record has no reserved
// bytes" is a fact about the radio rather than a gap in the suite.
func (r *run) checkGateRefusesAMutatedRecordByte() {
	r.t.Helper()
	p := r.p
	length := p.BuildRecordLength()

	layout, ok := p.LayoutFor(length)
	if !ok {
		r.t.Errorf("%s: LayoutFor(%d) is missing for its own BuildRecordLength", r.name(), length)
		return
	}
	unmapped := unmappedNibbles(layout)
	if len(unmapped) == 0 {
		r.t.Logf("%s: every nibble of its %d-byte record is mapped, so the gate's re-encode rule has nothing to mutate here — skipped", r.name(), length)
		return
	}

	addr := r.addresses()[0]
	rec, ok := r.sampleRecord(addr, length)
	if !ok {
		return
	}
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		r.t.Errorf("%s: BuildMemorySet(%v): %v", r.name(), addr, err)
		return
	}
	frame := cmd.Bytes()
	if !p.AllowedCommand(frame) {
		r.t.Errorf("%s: its own gate refused its own set frame before any mutation: %v", r.name(), frame)
		return
	}
	// The record occupies the bytes before the terminator.
	start := len(frame) - 1 - length

	for _, tgt := range unmapped {
		at := start + tgt.off
		// THREE candidate flips, all INSIDE the unmapped nibble so no
		// mapped field changes value. Only two bytes are framing bytes, so
		// three distinct candidates cannot all be one: the mutation always
		// happens, and a silently skipped offset — which would contribute
		// nothing while looking like a pass — cannot arise.
		alt, ok := mutateNibble(frame[at], tgt.high)
		if !ok {
			r.t.Errorf("%s: no in-nibble alteration of record byte %d (%#02x) avoided a framing byte — three candidates cannot all be 0xFE or 0xFD, so this suite's own mutation is broken", r.name(), tgt.off, frame[at])
			continue
		}
		mutated := make([]byte, len(frame))
		copy(mutated, frame)
		mutated[at] = alt
		if !civ.WellFormed(mutated) {
			r.t.Errorf("%s: altering UNMAPPED record byte %d to %#02x made the frame malformed (%v) — one interior byte that is not a framing byte cannot do that, so this suite's own mutation is broken", r.name(), tgt.off, alt, mutated)
			continue
		}
		if p.AllowedCommand(mutated) {
			r.t.Errorf("%s: its gate ADMITTED a set whose UNMAPPED %s of record byte %d was altered to %#02x (%v) — no builder would write that byte, so the gate's re-encode equality step is not doing its work and \"admits only builder-producible frames\" is false", r.name(), tgt.half(), tgt.off, alt, mutated)
			continue
		}
		r.refusals["a record byte no builder would write"]++
	}
}

// mutateNibble alters ONE nibble of b and reports the result, or false if
// every candidate landed on a framing byte — which cannot happen, there
// being three candidates and two framing bytes, and is reported rather
// than skipped so that "cannot happen" is checked rather than assumed.
func mutateNibble(b byte, high bool) (byte, bool) {
	for _, bit := range []byte{0x01, 0x02, 0x04} {
		if high {
			bit <<= 4
		}
		alt := b ^ bit
		if alt != civ.PreambleByte && alt != civ.EndByte {
			return alt, true
		}
	}
	return 0, false
}

// unmappedNibble names one HALF of one record byte that no field span
// claims — the granularity core/civ's V8 works at, and so the granularity
// the mutation check has to work at too.
type unmappedNibble struct {
	off  int
	high bool
}

func (u unmappedNibble) half() string {
	if u.high {
		return "HIGH nibble"
	}
	return "LOW nibble"
}

// unmappedNibbles returns the nibbles of layout's record that no field
// span claims — the halves a Fixed template speaks for, or that are zero.
//
// The claim split mirrors core/civ's own: only an EncodingEnum span
// declared on NibbleHigh or NibbleLow claims half a byte; every other
// encoding claims both halves. It is written out here rather than
// imported because civtest reads the profile through the EXPORTED API
// alone, as a model package does.
func unmappedNibbles(layout civ.RecordLayout) []unmappedNibble {
	mappedHigh := make([]bool, layout.Length)
	mappedLow := make([]bool, layout.Length)
	for _, sp := range layout.Fields {
		for off := sp.Offset; off < sp.Offset+sp.Length && off < layout.Length; off++ {
			high, low := true, true
			if sp.Encoding == civ.EncodingEnum {
				switch sp.Nibble {
				case civ.NibbleHigh:
					low = false
				case civ.NibbleLow:
					high = false
				}
			}
			if high {
				mappedHigh[off] = true
			}
			if low {
				mappedLow[off] = true
			}
		}
	}
	var out []unmappedNibble
	for i := 0; i < layout.Length; i++ {
		if !mappedHigh[i] {
			out = append(out, unmappedNibble{off: i, high: true})
		}
		if !mappedLow[i] {
			out = append(out, unmappedNibble{off: i})
		}
	}
	return out
}

// packRecord renders rec as a length-byte record using ONLY p's exported
// layout data — the second encoder checkEveryAcceptedLength's cross-check
// rests on. It mirrors core/civ's encodeRecord deliberately and states no
// policy of its own: a value that will not fit is this suite's own bug in
// sampleRecord, and says so.
func (r *run) packRecord(rec civ.MemoryRecord, length int) ([]byte, bool) {
	r.t.Helper()

	layout, ok := r.p.LayoutFor(length)
	if !ok {
		r.t.Errorf("%s: LayoutFor(%d) is missing for a length RecordLengths() reports", r.name(), length)
		return nil, false
	}

	out := make([]byte, length)
	if len(layout.Fixed) == length {
		copy(out, layout.Fixed)
	}
	for _, sp := range layout.Fields {
		switch sp.Encoding {
		case civ.EncodingBCDNumber:
			v, ok := getNumeric(rec, sp.Field)
			if !ok {
				r.t.Errorf("%s: this suite built a record with no %s for a layout that maps it", r.name(), sp.Field)
				return nil, false
			}
			b, ok := packBCD(v/sp.Scale, sp.Length, sp.Order)
			if !ok {
				r.t.Errorf("%s: %s = %d does not fit its %d-byte field at length %d — this suite's own sample is out of range", r.name(), sp.Field, v, sp.Length, length)
				return nil, false
			}
			copy(out[sp.Offset:], b)
		case civ.EncodingEnum:
			name, ok := getText(rec, sp.Field)
			if !ok {
				r.t.Errorf("%s: this suite built a record with no %s for a layout that maps it", r.name(), sp.Field)
				return nil, false
			}
			var wire byte
			found := false
			for v, n := range sp.Enum {
				if n == name {
					wire, found = v, true
					break
				}
			}
			if !found {
				r.t.Errorf("%s: %s = %q is not a value this profile's own enum defines", r.name(), sp.Field, name)
				return nil, false
			}
			switch sp.Nibble {
			case civ.NibbleHigh:
				out[sp.Offset] = out[sp.Offset]&0x0F | wire<<4
			case civ.NibbleLow:
				out[sp.Offset] = out[sp.Offset]&0xF0 | wire&0x0F
			default:
				out[sp.Offset] = wire
			}
		case civ.EncodingName:
			name, _ := getText(rec, sp.Field)
			for i := 0; i < sp.Length; i++ {
				if i < len(name) {
					out[sp.Offset+i] = name[i]
				} else {
					out[sp.Offset+i] = r.p.NamePad()
				}
			}
		default:
			r.t.Errorf("%s: field %s has encoding %v", r.name(), sp.Field, sp.Encoding)
			return nil, false
		}
	}
	return out, true
}

// packBCD is this package's own packed-BCD encoder, written from the wire
// convention rather than borrowed from core/civ — which is the whole point
// of it existing.
func packBCD(v uint64, n int, order civ.ByteOrder) ([]byte, bool) {
	if n < 1 {
		return nil, false
	}
	out := make([]byte, n)
	rest := v
	for i := 0; i < n; i++ {
		pair := rest % 100
		rest /= 100
		b := byte(pair/10)<<4 | byte(pair%10)
		switch order {
		case civ.OrderLittleEndian:
			out[i] = b
		case civ.OrderBigEndian:
			out[n-1-i] = b
		default:
			return nil, false
		}
	}
	if rest != 0 {
		return nil, false
	}
	return out, true
}

func (r *run) checkMemorySets() {
	r.t.Helper()

	length := r.p.BuildRecordLength()
	for _, addr := range r.addresses() {
		rec, ok := r.sampleRecord(addr, length)
		if !ok {
			return
		}
		cmd, err := r.p.BuildMemorySet(rec)
		if err != nil {
			r.t.Errorf("%s: BuildMemorySet(%v): %v", r.name(), addr, err)
			continue
		}
		frame := cmd.Bytes()
		r.checkFrame("memory set", frame)

		back, err := r.p.ParseMemoryAnswer(swapAddresses(frame))
		if err != nil {
			r.t.Errorf("%s: ParseMemoryAnswer rejected the answer form of its own set frame: %v", r.name(), err)
			continue
		}
		if back != rec {
			r.t.Errorf("%s: a record did not survive build -> parse:\n got %+v\nwant %+v", r.name(), back, rec)
			continue
		}
		r.roundTrips++

		if r.p.AllowedCommand(swapAddresses(frame)) {
			r.t.Errorf("%s: its gate ADMITTED a memory ANSWER — an answer is never a legal outbound command", r.name())
		} else {
			r.refusals["answer frame at the gate"]++
		}
	}

	// A NAME CONTAINING THE PAD BYTE, where the charset allows one: spec
	// D5 entry 3's awkward case — the name pad byte and space handling —
	// and the vector the evidence legs are asked to capture.
	if r.p.NameLength() >= 3 {
		charset := r.p.NameCharset()
		if contains(charset, r.p.NamePad()) {
			other := firstOtherThan(charset, r.p.NamePad())
			if other != 0 {
				rec, ok := r.sampleRecord(r.addresses()[0], length)
				if ok {
					rec.Name = civ.Available(string([]byte{other, r.p.NamePad(), other}))
					cmd, err := r.p.BuildMemorySet(rec)
					if err != nil {
						r.t.Errorf("%s: BuildMemorySet refused a name containing its own pad byte: %v", r.name(), err)
					} else {
						r.checkFrame("memory set (name with the pad byte)", cmd.Bytes())
						back, err := r.p.ParseMemoryAnswer(swapAddresses(cmd.Bytes()))
						if err != nil {
							r.t.Errorf("%s: ParseMemoryAnswer: %v", r.name(), err)
						} else if back != rec {
							r.t.Errorf("%s: a name with an INTERIOR pad byte was lost in the round trip: got %v, want %v", r.name(), back.Name, rec.Name)
						} else {
							r.roundTrips++
						}
					}
				}
			}
		}
	}
}

// checkNameLengthIsTheProfilesOwn requires the builder to refuse a name
// ONE BYTE past this profile's own field.
//
// It is here because of what the Wave-1b review demonstrated. Both halves
// of the internal/guards rule-5 fence are shape-only tripwires keyed on
// fixed name lists, and the review evaded both: it kept validName a
// Profile method and replaced p.nameLength with a package constant. Every
// guard passed, this whole suite passed, and the result was a
// gate-approved set carrying a 14-character name that the encoder
// silently truncated into a 10-byte field — a write the caller did not
// ask for. Only core/civ's own disagreeing fixtures caught it, and a
// per-model profile is not in reach of those.
//
// So the property is stated HERE too, where a Wave 3 model package runs
// it: the name bound is the RECEIVER's, and a name past it is refused
// rather than truncated. A profile with no name field has nothing to
// check and is skipped, loudly.
func (r *run) checkNameLengthIsTheProfilesOwn() {
	r.t.Helper()
	p := r.p

	n := p.NameLength()
	if n == 0 {
		r.t.Logf("%s: this profile has no name field, so the name bound has nothing to check here — skipped", r.name())
		return
	}
	charset := p.NameCharset()
	fill := firstOtherThan(charset, p.NamePad())
	if fill == 0 {
		r.t.Errorf("%s: every byte of its name charset is the pad byte", r.name())
		return
	}

	long := make([]byte, n+1)
	for i := range long {
		long[i] = fill
	}

	rec, ok := r.sampleRecord(r.addresses()[0], p.BuildRecordLength())
	if !ok {
		return
	}
	rec.Name = civ.Available(string(long))
	cmd, err := p.BuildMemorySet(rec)
	if err == nil {
		r.t.Errorf("%s: BuildMemorySet ACCEPTED a %d-byte name for its own %d-byte field, emitting %v — the encoder truncates, so this is a write the caller did not ask for, and it is what a name bound taken from anywhere but this profile looks like", r.name(), len(long), n, cmd.Bytes())
		return
	}
	if !cmd.IsZero() {
		r.t.Errorf("%s: BuildMemorySet returned a non-zero Command alongside its refusal of an over-long name", r.name())
	}
	r.refusals["a name past the profile's own field"]++
}

// checkGateRefusesTheUnacceptable is what stops "its own gate admits every
// frame it builds" from being satisfied by a gate that admits everything.
func (r *run) checkGateRefusesTheUnacceptable() {
	r.t.Helper()
	p := r.p
	radio, ctrl := p.RadioAddress(), p.ControllerAddress()

	frames := [][]byte{
		nil,
		{},
		{civ.PreambleByte},
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, civ.EndByte},
		{civ.PreambleByte, radio, ctrl, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, civ.CmdTransceiverID, civ.SubTransceiverID},
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, civ.AckByte, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, civ.NakByte, civ.EndByte},
		// The surfaces this tier refuses outright (spec D1, non-goals):
		// the menu, the memory keyer, transceive, and a clear-shaped form.
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, 0x1A, 0x05, 0x00, 0x01, 0x00, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, 0x1A, 0x01, 0x00, 0x01, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, 0x1C, 0x00, 0x01, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, 0x0F, 0x01, civ.EndByte},
		// A frame carrying a second one after an interior terminator.
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte, civ.PreambleByte, civ.PreambleByte, radio, ctrl, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte},
		// Addressed elsewhere.
		{civ.PreambleByte, civ.PreambleByte, 0x00, ctrl, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, radio ^ 0x01, ctrl, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, ctrl, radio, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, radio, ctrl ^ 0x01, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte},
	}
	for _, frame := range frames {
		if p.AllowedCommand(frame) {
			r.t.Errorf("%s: its gate ADMITTED %v — no profile may admit a malformed frame, an answer, a command it cannot build, or one addressed to another station", r.name(), frame)
			continue
		}
		r.refusals["malformed or forbidden frame at the gate"]++
	}
}

// checkNonVacuity is the half of this suite that fails when nothing
// happened. Every property above is a loop over data the profile supplies,
// and a profile supplying none would satisfy all of them in silence —
// which is exactly how a builder dropped from the walk goes unnoticed.
func (r *run) checkNonVacuity() {
	r.t.Helper()

	for _, what := range []string{"transceiver ID read", "memory read", "memory set"} {
		if r.frames[what] == 0 {
			r.t.Errorf("%s: builder %q contributed no frames — either this profile refuses it for every input this suite can offer, or the builder was dropped from the walk, and both are defects this property must not pass over", r.name(), what)
		}
	}
	for _, what := range []string{
		"malformed or forbidden frame at the gate",
		"answer frame at the gate",
		"address outside the profile's space",
		"unaccepted record length",
	} {
		if r.refusals[what] == 0 {
			r.t.Errorf("%s: no %q refusal was observed — the check either never ran or never had anything to refuse, and a rule that was never exercised is not evidence that it is enforced", r.name(), what)
		}
	}
	if r.roundTrips == 0 {
		r.t.Errorf("%s: not one record survived build -> parse — this profile's codec does not round-trip", r.name())
	}
	// EVERY declared layout must have been reached. A profile whose second
	// record length was never packed, gated or parsed is a profile whose
	// second layout has no evidence at all behind it, and the builder alone
	// can never reach one.
	for _, length := range r.p.RecordLengths() {
		if r.lengthsSeen[length] == 0 {
			r.t.Errorf("%s: accepted record length %d was never packed and gated — the builder emits only %d, so a layout this walk does not reach is a layout nothing has ever decoded", r.name(), length, r.p.BuildRecordLength())
		}
	}
	if r.total < minConformanceFrames {
		r.t.Errorf("%s: only %d frames were built and checked in total — this walk is not reaching the builders", r.name(), r.total)
	}

	r.t.Logf("%s: %d frames checked; per builder: %v; refusals seen: %v; record round trips: %d; records per accepted length: %v",
		r.name(), r.total, r.frames, r.refusals, r.roundTrips, r.lengthsSeen)
}

// addresses returns a sample of addresses valid under p's own address
// form: the ends of the channel range and a few in between, over the first
// and last group where the form is grouped.
func (r *run) addresses() []civ.ChannelAddress {
	p := r.p
	lo, hi := p.ChannelRange()

	channels := []int{lo, hi}
	if hi > lo {
		channels = append(channels, (lo+hi)/2)
	}
	for i := 1; len(channels) < maxAddressSamples && lo+i < hi; i++ {
		channels = append(channels, lo+i)
	}

	groups := []int{0}
	if p.AddressForm() != civ.AddressFormFlat {
		groups = []int{0}
		if p.Groups() > 1 {
			groups = append(groups, p.Groups()-1)
		}
	}

	var out []civ.ChannelAddress
	for _, g := range groups {
		for _, c := range channels {
			out = append(out, civ.ChannelAddress{Group: g, Channel: c})
		}
	}
	return out
}

// firstGroup is the lowest valid group index under p's form. Both forms
// number from 0 (core/civ's doc.go, GROUP AND BAND INDICES ARE NUMBERED
// FROM 0), and a flat form has no group at all, so it is 0 either way.
func (r *run) firstGroup() int { return 0 }

// sampleRecord builds a record every field p's layout maps is present in
// and no field it does not map is — the shape BuildMemorySet requires —
// from p's OWN exported layout data.
//
// It is the whole reason Profile.Layouts returns deep copies of its enum
// maps: this suite must be able to name a value each enum actually
// defines, for a model it has never seen.
func (r *run) sampleRecord(addr civ.ChannelAddress, length int) (civ.MemoryRecord, bool) {
	r.t.Helper()

	layout, ok := r.p.LayoutFor(length)
	if !ok {
		r.t.Errorf("%s: LayoutFor(%d) is missing for its own BuildRecordLength", r.name(), length)
		return civ.MemoryRecord{}, false
	}

	rec := civ.MemoryRecord{Address: addr}
	for _, sp := range layout.Fields {
		switch sp.Encoding {
		case civ.EncodingBCDNumber:
			// A wire value using most of the field's own digit positions,
			// scaled to the neutral unit — a multiple of the scale by
			// construction, and always inside the field.
			capacity := uint64(1)
			for i := 0; i < 2*sp.Length; i++ {
				capacity *= 10
			}
			setNumeric(&rec, sp.Field, ((capacity-1)/3)*sp.Scale)
		case civ.EncodingEnum:
			names := make([]string, 0, len(sp.Enum))
			for _, n := range sp.Enum {
				names = append(names, n)
			}
			if len(names) == 0 {
				r.t.Errorf("%s: field %s has an empty enum", r.name(), sp.Field)
				return civ.MemoryRecord{}, false
			}
			sort.Strings(names)
			setText(&rec, sp.Field, names[0])
		case civ.EncodingName:
			setText(&rec, sp.Field, r.sampleName())
		default:
			r.t.Errorf("%s: field %s has encoding %v", r.name(), sp.Field, sp.Encoding)
			return civ.MemoryRecord{}, false
		}
	}
	return rec, true
}

// sampleName returns a name valid under p and NEVER ending in the pad
// byte, which would come back trimmed and not round-trip exactly.
func (r *run) sampleName() string {
	n := r.p.NameLength()
	if n == 0 {
		return ""
	}
	var out []byte
	for _, b := range r.p.NameCharset() {
		if b == r.p.NamePad() {
			continue
		}
		out = append(out, b)
		if len(out) == n {
			break
		}
	}
	if len(out) == 0 {
		r.t.Errorf("%s: every byte of its name charset is the pad byte — no name could round-trip", r.name())
	}
	return string(out)
}

// setNumeric and setText reach a MemoryRecord's fields by id from OUTSIDE
// core/civ, where the package's own accessors are unexported.
//
// They are exhaustive switches, and a field id they do not handle is a
// LOUD failure rather than a silent no-op: a new field the suite silently
// skipped would make every record it builds incomplete, and
// BuildMemorySet would then refuse them all with a message about the
// profile rather than about this suite.
// getNumeric and getText are setNumeric and setText's readers: the same
// exhaustive switch, and the same loud panic on a field id core/civ has
// grown and this suite has not learned.
func getNumeric(rec civ.MemoryRecord, id civ.FieldID) (uint64, bool) {
	switch id {
	case civ.FieldRXFrequency:
		return rec.RXFreqHz.Get()
	case civ.FieldTXFrequency:
		return rec.TXFreqHz.Get()
	case civ.FieldOffset:
		return rec.OffsetHz.Get()
	case civ.FieldToneTX:
		return rec.ToneTXDeciHz.Get()
	case civ.FieldToneRX:
		return rec.ToneRXDeciHz.Get()
	case civ.FieldDTCSCode:
		return rec.DTCSCode.Get()
	default:
		panic("civtest: no numeric field for id " + string(id) + " — core/civ's vocabulary grew and this suite was not updated")
	}
}

func getText(rec civ.MemoryRecord, id civ.FieldID) (string, bool) {
	switch id {
	case civ.FieldDuplex:
		return rec.Duplex.Get()
	case civ.FieldMode:
		return rec.Mode.Get()
	case civ.FieldFilter:
		return rec.Filter.Get()
	case civ.FieldDataMode:
		return rec.DataMode.Get()
	case civ.FieldToneMode:
		return rec.ToneMode.Get()
	case civ.FieldDTCSPolarity:
		return rec.DTCSPolarity.Get()
	case civ.FieldName:
		return rec.Name.Get()
	case civ.FieldSelect:
		return rec.Select.Get()
	default:
		panic("civtest: no text field for id " + string(id) + " — core/civ's vocabulary grew and this suite was not updated")
	}
}

func setNumeric(rec *civ.MemoryRecord, id civ.FieldID, v uint64) {
	switch id {
	case civ.FieldRXFrequency:
		rec.RXFreqHz = civ.Available(v)
	case civ.FieldTXFrequency:
		rec.TXFreqHz = civ.Available(v)
	case civ.FieldOffset:
		rec.OffsetHz = civ.Available(v)
	case civ.FieldToneTX:
		rec.ToneTXDeciHz = civ.Available(v)
	case civ.FieldToneRX:
		rec.ToneRXDeciHz = civ.Available(v)
	case civ.FieldDTCSCode:
		rec.DTCSCode = civ.Available(v)
	default:
		panic("civtest: no numeric field for id " + string(id) + " — core/civ's vocabulary grew and this suite was not updated")
	}
}

func setText(rec *civ.MemoryRecord, id civ.FieldID, v string) {
	switch id {
	case civ.FieldDuplex:
		rec.Duplex = civ.Available(v)
	case civ.FieldMode:
		rec.Mode = civ.Available(v)
	case civ.FieldFilter:
		rec.Filter = civ.Available(v)
	case civ.FieldDataMode:
		rec.DataMode = civ.Available(v)
	case civ.FieldToneMode:
		rec.ToneMode = civ.Available(v)
	case civ.FieldDTCSPolarity:
		rec.DTCSPolarity = civ.Available(v)
	case civ.FieldName:
		rec.Name = civ.Available(v)
	case civ.FieldSelect:
		rec.Select = civ.Available(v)
	default:
		panic("civtest: no text field for id " + string(id) + " — core/civ's vocabulary grew and this suite was not updated")
	}
}

// swapAddresses returns a copy of frame with its `to` and `from` bytes
// exchanged: a command frame becomes the answer's shape, and vice versa.
//
// It is here rather than in core/civ deliberately. Turning a command into
// an answer is something only a TEST wants, and a production helper that
// did it would be a builder for frames no radio path ever sends.
func swapAddresses(frame []byte) []byte {
	out := make([]byte, len(frame))
	copy(out, frame)
	if len(out) >= 4 {
		out[2], out[3] = out[3], out[2]
	}
	return out
}

func contains(bs []byte, b byte) bool {
	for _, x := range bs {
		if x == b {
			return true
		}
	}
	return false
}

func firstOtherThan(bs []byte, b byte) byte {
	for _, x := range bs {
		if x != b {
			return x
		}
	}
	return 0
}

// RunZeroValue holds the ZERO civ.Profile to the one property it has: it
// refuses everything it is offered.
//
// A separate exported entry point rather than something Run detects — the
// package doc comment gives the reason at length. The short version is
// that an uninitialised profile reaching a conformance suite must be a
// loud failure, not a different suite quietly passing.
//
// UNLIKE core/cat's equivalent, NOT ONE BUILDER EMITS. core/cat has three
// builders whose frames are fixed literals and which therefore produce
// bytes on any receiver at all; CI-V has none, because every frame's `to`
// and `from` bytes are profile data. So the containment here is total
// rather than resting on the gate alone — and the gate is still asserted,
// because that is the property that matters.
func RunZeroValue(t T) {
	t.Helper()

	var zero civ.Profile

	if zero.Configured() {
		t.Fatal("the zero civ.Profile reports Configured() == true — every refusal below rests on it not being configured")
	}
	if zero.Model() != "" {
		t.Errorf("zero profile: Model() = %q, want empty", zero.Model())
	}
	if n := len(zero.RecordLengths()); n != 0 {
		t.Errorf("zero profile: RecordLengths() has %d entries, want none", n)
	}
	if n := len(zero.Layouts()); n != 0 {
		t.Errorf("zero profile: Layouts() has %d entries, want none", n)
	}
	if zero.AcceptsRecordLength(1) {
		t.Errorf("zero profile: AcceptsRecordLength(1) is true — it declares no record geometry at all")
	}

	builders := []struct {
		what string
		cmd  civ.Command
		err  error
	}{}
	add := func(what string, c civ.Command, err error) {
		builders = append(builders, struct {
			what string
			cmd  civ.Command
			err  error
		}{what, c, err})
	}
	idCmd, idErr := zero.BuildTransceiverIDRead()
	add("BuildTransceiverIDRead", idCmd, idErr)
	rdCmd, rdErr := zero.BuildMemoryRead(civ.ChannelAddress{Channel: 1})
	add("BuildMemoryRead", rdCmd, rdErr)
	stCmd, stErr := zero.BuildMemorySet(civ.MemoryRecord{})
	add("BuildMemorySet", stCmd, stErr)

	for _, b := range builders {
		if b.err == nil {
			t.Errorf("zero profile: %s SUCCEEDED, emitting %s — an unconfigured profile must build nothing", b.what, b.cmd)
			continue
		}
		if !b.cmd.IsZero() {
			t.Errorf("zero profile: %s returned a non-zero Command alongside its error", b.what)
		}
	}

	if _, err := zero.ParseTransceiverID([]byte{civ.PreambleByte, civ.PreambleByte, 0xE0, 0x94, civ.CmdTransceiverID, civ.SubTransceiverID, 0x94, civ.EndByte}); err == nil {
		t.Errorf("zero profile: ParseTransceiverID accepted an answer — it has no address to attribute one to")
	}
	memAnswer := []byte{civ.PreambleByte, civ.PreambleByte, 0xE0, 0x94, civ.CmdMemory, civ.SubMemoryContents, 0x00, 0x01, 0x00, civ.EndByte}
	if _, err := zero.ParseMemoryAnswer(memAnswer); err == nil {
		t.Errorf("zero profile: ParseMemoryAnswer accepted a memory answer — it has no layout to decode one with")
	}
	if _, record, err := zero.MemoryAnswerRecord(memAnswer); err == nil {
		t.Errorf("zero profile: MemoryAnswerRecord split an answer into % 02x — it has no address geometry to split one with, and the raw-bytes hook must not be the way past an unconfigured profile", record)
	}

	offered := [][]byte{
		{civ.PreambleByte, civ.PreambleByte, 0x94, 0xE0, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, 0x94, 0xE0, civ.CmdMemory, civ.SubMemoryContents, 0x00, 0x01, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, 0x94, 0xE0, civ.CmdMemory, civ.SubMemoryContents, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, 0xE0, 0x94, civ.AckByte, civ.EndByte},
		{civ.PreambleByte, civ.PreambleByte, 0x00, 0x94, civ.CmdTransceiverID, civ.SubTransceiverID, civ.EndByte},
	}
	refused := 0
	for _, frame := range offered {
		if zero.AllowedCommand(frame) {
			t.Errorf("zero profile: its gate ADMITTED %v — an unconfigured profile must authorise nothing, or a program holding one could put bytes on a wire to a radio it cannot describe", frame)
			continue
		}
		refused++
	}
	if refused != len(offered) {
		t.Errorf("zero profile: %d of %d offered frames were refused", refused, len(offered))
	}
	if refused == 0 {
		t.Errorf("zero profile: nothing was offered to the gate at all — this check would pass on a gate that admits everything")
	}

	t.Logf("zero profile: %d builders refused, %d frames refused at the gate", len(builders), refused)
}

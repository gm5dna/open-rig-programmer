// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic9700 "github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic9700"
)

// THE TWO WITNESSES MEET, HAVING NEVER READ EACH OTHER.
//
// Everything above this line in this package was written against a
// scripted responder that answers from a table this package wrote. That
// proves the driver consistent with itself and nothing more. The fake is
// the OTHER witness: an independently authored, stdlib-only radio whose
// implementer received a brief and the printed diagrams and never this
// plan, this driver, `core/civ/ic9700`, or its golden vectors. Where the
// two agree, two separate readings of one document agree; where they
// disagree, one of them is wrong and the disagreement is a finding.
//
// WHY THE TEST LIVES HERE. The Yaesu equivalent sits in
// `internal/wiring/wiring_test.go`, and `internal/wiring` is forbidden in
// Wave 3. `internal/guards`' TestSimulatedProfileTokensConfinement walks a
// hardcoded table of REGISTERED drivers — the IC-9700 is not in it until
// Wave 4 — and its confinement clause is about NON-TEST files, so a
// `_test.go` naming `ic9700.Simulated` and `fakeic9700.New` violates
// nothing. The Wave-4 registration commit adds the wiring row and the
// guard row together.
//
// THE FAKE HAS NO RECORD LENGTH, and that is its own recorded STOP, not a
// gap. Its two transcription legs disagree — one measures 114 bytes, the
// other counts 38 drawn cells of which several are dotted continuation
// boxes — so it refuses to pick one. It serves the length it was handed
// and enforces a write's length only once something has told it one
// (`WithSlot`'s seed, or `WithRecordLength`). Every test below therefore
// seeds 111-byte records built by THIS side's builder, which is exactly
// the shape the agreement is supposed to be about: the fake asserts
// nothing about the length, the driver asserts 111, and the fake serves
// back what it was given. A wrong-length answer needs BOTH
// `WithRecordLength` and a seeded slot the probe will reach.

// mustOpen opens a Simulated session against the fake and fails the test
// if it will not open.
//
// Simulated rather than RealHardware because the far end IS a simulator:
// spec.CapabilitiesSimulated is the profile whose writes are Supported,
// and "hardware safety" is not a question one can ask of a pipe. A
// RealHardware session over the same fake would refuse every write, which
// TestEndToEnd_WriteOne asserts alongside the write it does perform.
func mustOpen(t *testing.T, radio *fakeic9700.Radio) driver.Session {
	t.Helper()
	sess, err := ic9700.New(ic9700.Simulated).Open(context.Background(), radio.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

// recordBytes renders a neutral record as the 111 raw bytes a memory
// answer carries, using THIS side's builder.
//
// The fake is handed bytes and hands them back; it interprets none of
// them. So the driver's own encoder is what puts meaning into the seed,
// and the driver's own decoder is what has to find that meaning again
// after a round trip through a package that never looked at it.
func recordBytes(t *testing.T, rec civ.MemoryRecord) []byte {
	t.Helper()
	frame, err := civic9700.Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet(%v): %v", rec.Address, err)
	}
	b := frame.Bytes()
	// FE FE <to> <from> 1A 00 <3 address bytes> = 9, then the record,
	// then FD.
	return append([]byte(nil), b[9:len(b)-1]...)
}

// wrongLengthRecord is n bytes of filler: a record at a length this
// profile does not declare.
//
// The bytes are 0x00 because a seeded record may contain neither 0xFE nor
// 0xFD — the fake refuses one that does, on the wire's own grounds — and
// because their VALUES are irrelevant here: the probe refuses on the
// LENGTH, before any byte of the record is interpreted.
func wrongLengthRecord(t *testing.T, n int) []byte {
	t.Helper()
	return make([]byte, n)
}

func TestEndToEnd_ProbeFingerprint(t *testing.T) {
	// The probe asks two questions and changes nothing, against a radio
	// that never heard of this driver. The identity half is the CI-V
	// ADDRESS — an address-matched reply is what D3.2 requires — and the
	// fingerprint half is the RECORD-ONLY length of 111, which is what
	// `Profile.MemoryAnswerRecord` measures after stripping the three
	// address bytes the wire also carries.
	radio := fakeic9700.New(fakeic9700.WithSlot(1, 1, occupiedRecord(t)))
	defer radio.Close()

	sess := mustOpen(t, radio)
	if got := sess.Identity().CATID; !bytes.Contains([]byte(got), []byte("A2")) {
		t.Errorf("CATID = %q, want the address half present", got)
	}
	d := sess.(*ic9700.Session).CIVDiagnostics()
	if !d.Fingerprinted {
		t.Error("the probe did not fingerprint this profile against a 111-byte record")
	}
	if d.AnswerMismatches != 0 {
		t.Errorf("AnswerMismatches = %d on a well-behaved radio, want 0", d.AnswerMismatches)
	}
	if len(d.IDToken) == 0 {
		t.Error("no identity token was recorded")
	}

	// OPEN PUT NOTHING BUT THE TWO READ GRAMMARS ON THE WIRE, checked on
	// the fake's own transcript rather than on this package's recording
	// port — a second witness, and an independently authored one.
	//
	// THIS AUDIT IS ABOUT Open AND SAYS SO. The wider claim — that the
	// tier writes nothing to a radio outside the consented memory-set
	// path — covers a whole session including its writes, and is proved
	// by TestEndToEnd_TheWholeSessionSendsOnlyTheGrammarsItCanBuild
	// below. Stating it here would have been prose wider than its scope.
	assertOnlyTheBuildableGrammars(t, radio, 0)
}

// buildableReadFrames precomputes every frame this profile's OWN builders
// can produce for a plain read, so assertOnlyTheBuildableGrammars can
// require EXACT byte equality rather than a cn/sc/length match.
//
// The identity read takes no parameter, so there is exactly one legal
// frame. A memory read is parameterised by the channel address, so every
// address this profile's own ChannelRange/Groups declare is built once and
// kept as a set — 3 bands × 107 channels here — and a wire frame is
// accepted only if it is byte-identical to ONE of them.
func buildableReadFrames(t *testing.T) (identityRead []byte, memoryReads map[string]bool) {
	t.Helper()
	p := civic9700.Profile()

	idCmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}

	memoryReads = map[string]bool{}
	groups, base := p.Groups(), p.GroupBase()
	lo, hi := p.ChannelRange()
	for g := base; g < base+groups; g++ {
		for c := lo; c <= hi; c++ {
			cmd, err := p.BuildMemoryRead(civ.ChannelAddress{Group: g, Channel: c})
			if err != nil {
				continue // an address this profile itself refuses builds nothing to compare against
			}
			memoryReads[string(cmd.Bytes())] = true
		}
	}
	return idCmd.Bytes(), memoryReads
}

// assertOnlyTheBuildableGrammars requires that every frame the fake
// received is one of the THREE this tier can build, and that exactly
// wantSets of them are memory sets.
//
// EVERY LENGTH IS EXACT, and that is what makes the audit an audit rather
// than a command-number check:
//
//	 7 = FE FE A2 E0 19 00 FD                            the identity read
//	10 = FE FE A2 E0 1A 00 <3 address bytes> FD          a memory read
//	121 = the same, plus the 111-byte record             a memory set
//
// A `19 00` carrying data, a `1A 00` read with a record stapled to it, and
// a set at any other width are all refused here. The clear form is
// `1A 00 <3 address bytes> FF`, eleven bytes, and matches none of the
// three — which is the structural half of "erase is unshipped".
//
// LENGTH AND COMMAND NUMBER ARE NOT THE WHOLE AUDIT, and a same-width frame
// can satisfy both while still being no frame this tier's own machinery
// would produce — a corrupted address, a byte a builder never writes at an
// unmapped offset, an addressed-but-otherwise-malformed record. So each
// grammar is checked against the ACTUAL MACHINERY: the two reads against
// buildableReadFrames' exact byte sets (there is no "close enough" for a
// frame carrying no data to validate), and every SET against
// civ.Profile.AllowedCommand — the same gate core/civ's Framing.Allow calls
// before a real write reaches the wire, which decodes, re-validates and
// re-encodes the record and only admits an exact match.
func assertOnlyTheBuildableGrammars(t *testing.T, radio *fakeic9700.Radio, wantSets int) {
	t.Helper()
	const (
		identityRead = 7
		memoryRead   = 10
		memorySet    = 4 + 2 + civic9700.AddressBytes + civic9700.RecordLength + 1
	)
	p := civic9700.Profile()
	idFrame, memReadFrames := buildableReadFrames(t)

	sets := 0
	for _, f := range radio.Transcript() {
		cn, sc, ok := civ.FrameCommand(f)
		switch {
		case ok && cn == 0x19 && sc == 0x00 && len(f) == identityRead:
			if !bytes.Equal(f, idFrame) {
				t.Errorf("identity read % X is not byte-identical to BuildTransceiverIDRead's own % X", f, idFrame)
			}
		case ok && cn == 0x1A && sc == 0x00 && len(f) == memoryRead:
			if !memReadFrames[string(f)] {
				t.Errorf("memory read % X matches no address BuildMemoryRead builds for this profile", f)
			}
		case ok && cn == 0x1A && sc == 0x00 && len(f) == memorySet:
			sets++
			if !p.AllowedCommand(f) {
				t.Errorf("memory set % X is refused by this profile's own AllowedCommand gate — it is not a frame the builder could have produced", f)
			}
		default:
			t.Errorf("the session put % X on the wire — no builder in this tier names that frame", f)
		}
	}
	if sets != wantSets {
		t.Errorf("the radio received %d memory sets, want %d", sets, wantSets)
	}
}

func TestEndToEnd_ReadAll(t *testing.T) {
	// Every seeded slot reads back with the values the fake holds —
	// across TWO BANDS, because the band half of the address is the whole
	// reason this model's slot spelling carries one, and a driver that
	// dropped it would read three different memories as one.
	sameBand := goldenRecordAt(1, 4)
	sameBand.RXFreqHz = civ.Available(uint64(145_600_000))
	sameBand.TXFreqHz = civ.Available(uint64(145_600_000))
	sameBand.Mode = civ.Available("USB")
	sameBand.Name = civ.Available("SAME BAND")

	otherBand := goldenRecordAt(2, 1)
	otherBand.RXFreqHz = civ.Available(uint64(433_500_000))
	otherBand.TXFreqHz = civ.Available(uint64(433_500_000))
	otherBand.Mode = civ.Available("FM")
	otherBand.Name = civ.Available("OTHER BAND")

	radio := fakeic9700.New(
		fakeic9700.WithSlot(1, 1, occupiedRecord(t)),
		fakeic9700.WithSlot(1, 4, recordBytes(t, sameBand)),
		fakeic9700.WithSlot(2, 1, recordBytes(t, otherBand)),
	)
	defer radio.Close()
	sess := mustOpen(t, radio)

	for _, tc := range []struct {
		slot     string
		wantFreq uint64
		wantMode string
		wantTag  string
	}{
		{"144-001", 145_500_000, "FM", "INVERNESS GB3CFR"},
		{"144-004", 145_600_000, "USB", "SAME BAND"},
		{"430-001", 433_500_000, "FM", "OTHER BAND"},
	} {
		ch, err := sess.ReadChannel(context.Background(), tc.slot)
		if err != nil {
			t.Fatalf("ReadChannel(%s): %v", tc.slot, err)
		}
		if ch.Data == nil {
			t.Fatalf("ReadChannel(%s): empty, want the seeded record", tc.slot)
		}
		if ch.Data.FreqHz != tc.wantFreq {
			t.Errorf("%s: FreqHz = %d, want %d", tc.slot, ch.Data.FreqHz, tc.wantFreq)
		}
		if ch.Data.Mode != tc.wantMode {
			t.Errorf("%s: Mode = %q, want %q", tc.slot, ch.Data.Mode, tc.wantMode)
		}
		if ch.Data.Tag != tc.wantTag {
			t.Errorf("%s: Tag = %q, want %q", tc.slot, ch.Data.Tag, tc.wantTag)
		}
		// The tier fields survive the round trip too, which is what makes
		// this a record test rather than a frequency test.
		if ch.Data.ToneTx.State != codeplug.Known || ch.Data.ToneTx.Value != spec.Tone(885) {
			t.Errorf("%s: ToneTx = %+v, want Known 885 deciHz", tc.slot, ch.Data.ToneTx)
		}
		if ch.Data.OffsetHz.State != codeplug.Known || ch.Data.OffsetHz.Value != 600_000 {
			t.Errorf("%s: OffsetHz = %+v, want Known 600000", tc.slot, ch.Data.OffsetHz)
		}
	}

	// 144-001 and 430-001 are DIFFERENT MEMORIES with the same channel
	// number, and the fake agrees with the driver about that only because
	// both encode the band into the address.
	first, err := sess.ReadChannel(context.Background(), "144-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	other, err := sess.ReadChannel(context.Background(), "430-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if first.Data.Tag == other.Data.Tag {
		t.Errorf("channel 1 of two bands read back the same memory (%q) — the band byte was lost", first.Data.Tag)
	}
}

func TestEndToEnd_WriteOne(t *testing.T) {
	// A Simulated session writes one channel, the fake acknowledges with
	// FB, and a re-read returns it. The fake enforces the write's length
	// because the seeded slot told it one — 111 bytes, inferred from what
	// it was handed, never asserted by it.
	radio := fakeic9700.New(fakeic9700.WithSlot(1, 1, occupiedRecord(t)))
	defer radio.Close()
	sess := mustOpen(t, radio)

	res, err := sess.WriteChannel(context.Background(), fullyKnownChannelAt("144-002"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Fatalf("steps = %+v, want one sent and confirmed frame", res.Steps)
	}

	back, err := sess.ReadChannel(context.Background(), "144-002")
	if err != nil {
		t.Fatalf("ReadChannel after write: %v", err)
	}
	if back.Data == nil {
		t.Fatal("the slot read back empty after an acknowledged write")
	}
	want := writableChannelData()
	if back.Data.FreqHz != want.FreqHz || back.Data.Mode != want.Mode || back.Data.Tag != want.Tag {
		t.Errorf("read back %d/%q/%q, want %d/%q/%q",
			back.Data.FreqHz, back.Data.Mode, back.Data.Tag, want.FreqHz, want.Mode, want.Tag)
	}
	if back.Data.ToneTx.Value != want.ToneTx.Value || back.Data.DTCSCode.Value != want.DTCSCode.Value {
		t.Errorf("read back tone %v / DTCS %v, want %v / %v",
			back.Data.ToneTx.Value, back.Data.DTCSCode.Value, want.ToneTx.Value, want.DTCSCode.Value)
	}

	// THE OTHER PROFILE OVER THE SAME RADIO REFUSES THE SAME WRITE.
	// Simulated is not a licence the driver grants itself: a RealHardware
	// session gets the all-Unverified set while writeTrialsComplete is
	// false, and nothing about the far end being a simulator changes it.
	real9700 := fakeic9700.New(fakeic9700.WithSlot(1, 1, occupiedRecord(t)))
	defer real9700.Close()
	hw, err := ic9700.New(ic9700.RealHardware).Open(context.Background(), real9700.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer hw.Close()
	if _, err := hw.WriteChannel(context.Background(), fullyKnownChannelAt("144-003")); !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want ErrWriteRefused on an unconsented real-hardware session", err)
	}
}

func TestEndToEnd_WrongSiblingRefusedByRecordLength(t *testing.T) {
	// The fingerprint only fires on an OCCUPIED slot, so the fake is
	// given one INSIDE the probe's bounded window (band 1, channels 1..8)
	// as well as the wrong length: WithRecordLength alone would leave
	// every probe read answering NG and the session would open
	// UNFINGERPRINTED instead of refusing.
	radio := fakeic9700.New(
		fakeic9700.WithSlot(1, 1, wrongLengthRecord(t, 64)),
		fakeic9700.WithRecordLength(64),
	)
	defer radio.Close()
	_, err := ic9700.New(ic9700.Simulated).Open(context.Background(), radio.Port(), driver.Identity{})
	var rl *civ.RecordLengthError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want *civ.RecordLengthError", err)
	}
	if rl.Got != 64 {
		t.Errorf("RecordLengthError.Got = %d, want 64 (record-only)", rl.Got)
	}
	// Cross-model record-length distinctness is a TIER-level Wave-4
	// check: this driver refuses the length and names no model.
	var wrong *driver.WrongRadioError
	if errors.As(err, &wrong) {
		t.Fatalf("the driver attributed the radio to %q", wrong.GotModel)
	}
}

func TestEndToEnd_AnswerNamingAnotherSlotIsRefused(t *testing.T) {
	// T2's MANDATORY per-driver mismatch regression test, through the
	// fake. The landed memory matcher is envelope-only, so this reply
	// reaches the driver and only the decoded-address comparison stops it.
	radio := fakeic9700.New(
		fakeic9700.WithSlot(1, 1, occupiedRecord(t)),
		fakeic9700.WithAnswerAddress(1, 7), // answers name channel 7 whatever was asked
	)
	defer radio.Close()
	sess := mustOpen(t, radio)

	_, err := sess.ReadChannel(context.Background(), "144-001")
	if !errors.Is(err, ic9700.ErrAnswerMismatch) {
		t.Fatalf("err = %v, want ErrAnswerMismatch", err)
	}
	var mm *ic9700.AnswerMismatchError
	if !errors.As(err, &mm) {
		t.Fatalf("err = %v, want *ic9700.AnswerMismatchError", err)
	}
	if mm.Requested.Channel != 1 || mm.Answered.Channel != 7 {
		t.Errorf("mismatch = requested %v answered %v", mm.Requested, mm.Answered)
	}
	if sess.(*ic9700.Session).CIVDiagnostics().AnswerMismatches == 0 {
		t.Error("the mismatch was not counted")
	}

	// And the PROBE refused to fingerprint from that same answer, so the
	// session opened on address evidence alone rather than measuring a
	// record belonging to another channel.
	if sess.(*ic9700.Session).CIVDiagnostics().Fingerprinted {
		t.Error("the probe fingerprinted this profile from a misdirected answer")
	}
}

func TestEndToEnd_EmptySlotsDoNotAbortTheWalk(t *testing.T) {
	// R10 end to end: the fake answers NG for unseeded channels, and the
	// walk returns EMPTY channels for them rather than stopping. This is
	// the regression REV 1 would have shipped — a rejected read reported
	// as an error aborts core/clone's whole ReadAll on the first unwritten
	// slot, which on any real radio is most of them.
	radio := fakeic9700.New(
		fakeic9700.WithSlot(1, 1, occupiedRecord(t)),
		fakeic9700.WithEmptySlot(1, 2),
		fakeic9700.WithSlot(1, 4, occupiedRecord(t)),
	)
	defer radio.Close()
	sess := mustOpen(t, radio)

	populated := 0
	for _, slot := range []string{"144-001", "144-002", "144-003", "144-004"} {
		ch, err := sess.ReadChannel(context.Background(), slot)
		if err != nil {
			t.Fatalf("%s: %v", slot, err)
		}
		if ch.Slot != slot {
			t.Errorf("Slot = %q, want %q — an empty channel still names its slot", ch.Slot, slot)
		}
		if ch.Data != nil {
			populated++
		}
	}
	if populated != 2 {
		t.Errorf("read %d populated channels, want 2", populated)
	}
}

func TestEndToEnd_BroadcastFloodOpensCleanlyAndIsCounted(t *testing.T) {
	// R9-SPLIT (a) mirrored at the e2e. WithBroadcasts emits to=00 only:
	// the accumulator drops them before the engine sees one, so the idle
	// timer is never re-armed, Init returns nil, NO drain-cap diagnostic
	// exists, and the frames appear only in AccumulatorStats().Unexpected.
	radio := fakeic9700.New(
		fakeic9700.WithSlot(1, 1, occupiedRecord(t)),
		fakeic9700.WithBroadcasts(2*time.Millisecond),
	)
	defer radio.Close()
	sess := mustOpen(t, radio)

	d := sess.(*ic9700.Session).CIVDiagnostics()
	if d.InitDrainCapExceeded {
		t.Error("a broadcast flood produced a drain-cap diagnostic")
	}
	if d.Accumulator.Unexpected == 0 {
		t.Error("the broadcasts were not counted")
	}
	// The engine's own counter is systematically zero here, which is why
	// the driver holds the framing value and asks IT.
	if got := sess.(driver.DiagnosticsReporter).Diagnostics().UnexpectedFrames; got != 0 {
		t.Logf("engine UnexpectedFrames = %d (informational)", got)
	}
}

func TestEndToEnd_ControllerAddressedFloodOpensNonfatally(t *testing.T) {
	// R9-SPLIT (b) mirrored: only this species can reach the drain cap,
	// because only these frames pass the accumulator's address filter and
	// re-arm the idle timer. The INITIAL failure is recorded and forgiven.
	radio := fakeic9700.New(
		fakeic9700.WithSlot(1, 1, occupiedRecord(t)),
		fakeic9700.WithAddressedFlood(2*time.Millisecond),
	)
	defer radio.Close()
	sess := mustOpen(t, radio)

	if !sess.(*ic9700.Session).CIVDiagnostics().InitDrainCapExceeded {
		t.Error("the drain-cap event was swallowed; nonfatal does not mean unrecorded")
	}
}

func TestEndToEnd_EraseIsRefused(t *testing.T) {
	// Erase is UNSHIPPED, and three independent things say so. The wire
	// form EXISTS on this radio — matrix §3.13 prints `1A 00 <addr> FF`,
	// and the fake knows it well enough to refuse it — which is exactly
	// why the absence has to be structural rather than incidental.
	radio := fakeic9700.New(fakeic9700.WithSlot(1, 1, occupiedRecord(t)))
	defer radio.Close()
	sess := mustOpen(t, radio)

	// One: no consent opens the gate, on either profile.
	for name, caps := range map[string]spec.Capabilities{
		"unverified": ic9700.CapabilitiesUnverified(),
		"simulated":  ic9700.CapabilitiesSimulated(),
	} {
		consented := spec.ConsentUnverifiedWrites(caps)
		for _, bank := range consented.Banks {
			if consented.FieldSupport(bank.ID, spec.FieldErase).CanWrite() {
				t.Errorf("%s/%s: consent opened the erase gate", name, bank.ID)
			}
		}
	}

	// Two: an empty channel is refused, naming the reason, and the gate
	// admits no clear frame even if one were somehow built.
	if _, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "144-001"}); !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want ErrWriteRefused for an empty channel", err)
	}
	clear := []byte{0xFE, 0xFE, 0xA2, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFD}
	if civic9700.Profile().AllowedCommand(clear) {
		t.Fatal("the outbound gate admitted the clear frame")
	}

	// Three: the fake never saw one. Its transcript is the second
	// witness — the clear form is `1A 00 <3 address bytes> FF`, four data
	// bytes ending in FF.
	for _, f := range radio.Transcript() {
		cn, sc, ok := civ.FrameCommand(f)
		if !ok || cn != 0x1A || sc != 0x00 {
			continue
		}
		payload := f[6 : len(f)-1]
		if len(payload) == 4 && payload[3] == 0xFF {
			t.Errorf("a clear frame reached the radio: % X", f)
		}
	}
}

func TestEndToEnd_TheWholeSessionSendsOnlyTheGrammarsItCanBuild(t *testing.T) {
	// THE TIER-WIDE CLAIM, audited over a WHOLE SESSION rather than over
	// its opening: probe, write, re-read. TestEndToEnd_ProbeFingerprint
	// checks the same property over Open alone, which is a narrower thing
	// than "the tier writes nothing to a radio outside the consented
	// memory-set path" — and the erase test cannot close the gap either,
	// because both of its write attempts are refused BEFORE the wire, so
	// its transcript is an Open transcript too.
	//
	// This session actually writes. Exactly one memory set is expected,
	// at exactly 121 bytes, and every other frame must be one of the two
	// read grammars at their exact widths.
	radio := fakeic9700.New(fakeic9700.WithSlot(1, 1, occupiedRecord(t)))
	defer radio.Close()
	sess := mustOpen(t, radio)

	if _, err := sess.WriteChannel(context.Background(), fullyKnownChannelAt("144-002")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	back, err := sess.ReadChannel(context.Background(), "144-002")
	if err != nil {
		t.Fatalf("ReadChannel after write: %v", err)
	}
	if back.Data == nil {
		t.Fatal("the slot read back empty after an acknowledged write")
	}

	// One set — the write's own — and nothing else that is not a read.
	// The write path also performs its mandatory preservation read of the
	// target slot before building anything (T5's single read), so the
	// read count is not asserted: what matters is that no frame outside
	// the three buildable grammars appeared, and that the session wrote
	// ONCE.
	assertOnlyTheBuildableGrammars(t, radio, 1)
}

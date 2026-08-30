// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7851"
)

// THIS FILE IS WHERE TWO INDEPENDENT READINGS OF PDF P.263 MEET.
//
// internal/fakeic7851 was written by an implementer who read the committed
// evidence — the geometry witness and the B transcription — and never
// opened core/civ/ic7851 or this package. Its RecordLen was derived from
// the transcription's own width_bytes column; this driver's came through
// the plan's one table by another route. TestE2E_ProbeFingerprints is
// where the two are OBSERVED to agree rather than assumed to.
//
// A DISAGREEMENT HERE IS A STOP FOR ARBITRATION AGAINST THE PDF and is
// NEVER fixed by editing the fake to match the codec.
//
// Every record below is assembled BY OFFSET from neutral test literals.
// Nothing in this file calls the profile's builders to make a fixture: a
// fixture built by the encoder under test would agree with it by
// construction.

// ------------------------------------------------------ record fixtures

// The wire codes, written out here from the printed tables rather than
// taken from the codec, whose enums are what these records exercise.
// RULING OQ1: the codes are hexadecimal, so PSK is 0x12.
var (
	e2eModes = []struct {
		code byte
		name string
	}{
		{0x00, "LSB"}, {0x01, "USB"}, {0x02, "AM"}, {0x03, "CW"}, {0x04, "RTTY"},
		{0x05, "FM"}, {0x07, "CW-R"}, {0x08, "RTTY-R"}, {0x12, "PSK"}, {0x13, "PSK-R"},
	}
	e2eFilters = []struct {
		code byte
		name string
	}{{0x01, "FIL1"}, {0x02, "FIL2"}, {0x03, "FIL3"}}
	e2eToneModes = []struct {
		code byte
		name string
	}{{0x00, "OFF"}, {0x01, "TONE"}, {0x02, "TSQL"}}
)

// e2eFields is one slot's content in NEUTRAL terms, so each test states
// what it put in the radio rather than what a builder produced.
type e2eFields struct {
	freqHz   uint64
	mode     string
	filter   string
	toneMode string
	toneTx   uint64 // deci-Hz
	toneRx   uint64 // deci-Hz
	name     string
}

// bcdLE renders n as width bytes of little-endian packed BCD: the
// frequency strip's convention, least significant digit pair first.
func bcdLE(n uint64, width int) []byte {
	out := make([]byte, width)
	for i := 0; i < width; i++ {
		lo := byte(n % 10)
		n /= 10
		hi := byte(n % 10)
		n /= 10
		out[i] = hi<<4 | lo
	}
	return out
}

// bcdBE renders n as width bytes of big-endian packed BCD: the tone
// strip's convention, which is the OPPOSITE of the frequency strip's on
// the same radio.
func bcdBE(n uint64, width int) []byte {
	out := bcdLE(n, width)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// record assembles the 25-byte record BY OFFSET, from the plan's one
// table. Printed index N sits at offset N-3; ⑧, ⑫ and ⑮ are the printed
// fixed-zero pairs and are left at zero, as is ③ and ⑪'s high nibble.
func (f e2eFields) record(t *testing.T) []byte {
	t.Helper()
	rec := make([]byte, civic7851.RecordOnlyLength)
	copy(rec[1:5], bcdLE(f.freqHz, 4))
	rec[6] = codeFor(t, "mode", f.mode)
	rec[7] = codeFor(t, "filter", f.filter)
	rec[8] = codeFor(t, "tone_mode", f.toneMode) & 0x0F
	copy(rec[10:12], bcdBE(f.toneTx, 2))
	copy(rec[13:15], bcdBE(f.toneRx, 2))
	for i := 15; i < 25; i++ {
		rec[i] = 0x20
	}
	copy(rec[15:25], f.name)
	return rec
}

func codeFor(t *testing.T, kind, name string) byte {
	t.Helper()
	var table []struct {
		code byte
		name string
	}
	switch kind {
	case "mode":
		table = e2eModes
	case "filter":
		table = e2eFilters
	case "tone_mode":
		table = e2eToneModes
	}
	for _, e := range table {
		if e.name == name {
			return e.code
		}
	}
	t.Fatalf("no %s code for %q", kind, name)
	return 0
}

// e2eSeed varies every mapped field across the slot inventory so no two
// channels share a value by accident. Both tones stay inside the declared
// 670..2541 chart, so both come back Known.
func e2eSeed(i int) e2eFields {
	m := e2eModes[i%len(e2eModes)]
	fl := e2eFilters[i%len(e2eFilters)]
	tm := e2eToneModes[i%len(e2eToneModes)]
	return e2eFields{
		freqHz:   1_800_000 + uint64(i)*470_000,
		mode:     m.name,
		filter:   fl.name,
		toneMode: tm.name,
		toneTx:   uint64(670 + (i%50)*37),
		toneRx:   uint64(700 + (i%40)*41),
		name:     fmt.Sprintf("CH%03d TEST", i),
	}
}

// e2eSlots is every slot this radio declares, in capability order: 99
// memories and the two scan edges.
func e2eSlots() []string {
	var out []string
	for _, b := range capabilitiesUnverified().Banks {
		out = append(out, b.Slots...)
	}
	return out
}

// ---------------------------------------------------------- the wire tap

// tap wraps the fake's port and keeps the exact bytes this driver wrote.
//
// IT IS THE ONLY THING THAT CAN PROVE THE NEGATIVES. "Init sends nothing",
// "a refusal costs no wire traffic" and "a lost write is never resent" are
// all claims about bytes that did NOT appear, and only a byte log can
// settle one.
type tap struct {
	conn     net.Conn
	mu       sync.Mutex
	sent     []byte
	dropSets atomic.Bool
}

func (t *tap) Read(p []byte) (int, error) { return t.conn.Read(p) }

func (t *tap) Write(p []byte) (int, error) {
	t.mu.Lock()
	t.sent = append(t.sent, p...)
	t.mu.Unlock()
	if t.dropSets.Load() && isMemorySet(p) {
		// SWALLOWED, NOT REJECTED. The radio never sees the frame and
		// never answers, which is exactly the ambiguity an acknowledgement
		// timeout describes.
		return len(p), nil
	}
	return t.conn.Write(p)
}

func (t *tap) Close() error { return t.conn.Close() }

// frames splits the byte log into complete CI-V frames.
func (t *tap) frames() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out [][]byte
	b := t.sent
	for {
		i := bytes.Index(b, []byte{0xfe, 0xfe})
		if i < 0 {
			return out
		}
		b = b[i:]
		j := bytes.IndexByte(b, 0xfd)
		if j < 0 {
			return out
		}
		out = append(out, append([]byte(nil), b[:j+1]...))
		b = b[j+1:]
	}
}

// isMemorySet reports whether b is one whole 1A 00 frame carrying a
// record — as against the read request, which carries only the address.
func isMemorySet(b []byte) bool {
	return len(b) > 6+2+1 && b[0] == 0xfe && b[1] == 0xfe &&
		b[4] == 0x1a && b[5] == 0x00 && b[len(b)-1] == 0xfd
}

func countSets(frames [][]byte) int {
	n := 0
	for _, f := range frames {
		if isMemorySet(f) {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------- the fixtures

func newFake(t *testing.T, opts ...fakeic7851.Option) *fakeic7851.Radio {
	t.Helper()
	r := fakeic7851.New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	return r
}

// openFake opens a session against r through a tap, and returns both.
func openFake(t *testing.T, r *fakeic7851.Radio, newDriver func(...Option) driver.Driver, opts ...Option) (*Session, *tap) {
	t.Helper()
	s, tp, err := tryOpenFake(t, r, newDriver, opts...)
	if err != nil {
		t.Fatalf("Open against the fake: %v", err)
	}
	return s, tp
}

func tryOpenFake(t *testing.T, r *fakeic7851.Radio, newDriver func(...Option) driver.Driver, opts ...Option) (*Session, *tap, error) {
	t.Helper()
	tp := &tap{conn: r.Port()}
	sess, err := newDriver(opts...).Open(t.Context(), tp, driver.Identity{Port: "/dev/fake"})
	if err != nil {
		return nil, tp, err
	}
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, tp, nil
}

// constructors is the pair every E2E case is run for. THE TWO ROWS SHARE
// ONE IMPLEMENTATION, ONE PROFILE AND ONE ADDRESS, and the probe cannot
// tell them apart (matrix §4): the user picks the row. Running both is how
// that claim is exercised rather than merely written down.
var constructors = []struct {
	model string
	make  func(...Option) driver.Driver
}{
	{"IC-7851", New7851},
	{"IC-7850", New7850},
}

// -------------------------------------------------------------- the cases

// TestE2E_ProbeFingerprints is the independence check landing, and the
// ID-token rule with it: what identifies the radio is that an
// ADDRESS-MATCHED 19 00 reply arrived at all. The reply VALUE is
// undocumented (register entry ic7851-id-reply-value), so it is RECORDED
// and never compared — which three different tokens opening a session is
// what demonstrates.
func TestE2E_ProbeFingerprints(t *testing.T) {
	if fakeic7851.RecordLen != civic7851.RecordOnlyLength {
		t.Fatalf("STOP — the fake derives a %d-byte record from the frozen transcription and this driver's profile declares %d. "+
			"The two were read off PDF p.263 independently; a disagreement is orchestrator arbitration AGAINST THE PAGE, never an edit to either side",
			fakeic7851.RecordLen, civic7851.RecordOnlyLength)
	}
	for _, c := range constructors {
		for _, token := range []string{"IC-7851", "IC-7850", "\x01\x02"} {
			t.Run(c.model+"/"+fmt.Sprintf("%q", token), func(t *testing.T) {
				r := newFake(t, fakeic7851.WithModelName(token))
				r.SetSlot("001", e2eSeed(1).record(t))

				s, _ := openFake(t, r, c.make)
				length, confirmed := s.Fingerprint()
				if !confirmed || length != civic7851.RecordOnlyLength {
					t.Errorf("Fingerprint() = (%d, %v), want (%d, true)", length, confirmed, civic7851.RecordOnlyLength)
				}
				if got := string(s.OpenDiagnostics().IDToken); got != token {
					t.Errorf("IDToken = %q, want the radio's answer %q recorded verbatim", got, token)
				}
				wantCATID := fmt.Sprintf("8e%x", token)
				if got := s.Identity().CATID; got != wantCATID {
					t.Errorf("Identity().CATID = %q, want %q — the static address followed by the observed token", got, wantCATID)
				}
				if got := s.Capabilities().Model; got != c.model {
					t.Errorf("the session's capabilities name %q; the ROW is the user's choice and the probe cannot narrow it", got)
				}
			})
		}
	}
}

// TestE2E_InitAndProbeSendNothingElse pins Open's ENTIRE wire traffic.
//
// CI-V Init is a bounded DRAIN and nothing else — no transceive-off write,
// no 1A 05, no clear — so opening a session mutates nothing. The proof is
// the byte log, not the source.
func TestE2E_InitAndProbeSendNothingElse(t *testing.T) {
	r := newFake(t)
	r.SetSlot("001", e2eSeed(1).record(t))
	_, tp := openFake(t, r, New7851)

	frames := tp.frames()
	if len(frames) != 2 {
		t.Fatalf("Open put %d frames on the wire: % X", len(frames), frames)
	}
	if got, want := frames[0], []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x19, 0x00, 0xfd}; !bytes.Equal(got, want) {
		t.Errorf("the first frame is % X, want the 19 00 identity read % X", got, want)
	}
	if got, want := frames[1], []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01, 0xfd}; !bytes.Equal(got, want) {
		t.Errorf("the second frame is % X, want the 1A 00 read of channel 1 % X", got, want)
	}
}

// TestE2E_EmptyRadioOpensUnfingerprinted covers BOTH unverified empty
// readings, which are two separate register entries and which one capture
// could not establish together: a rejected read (ic7851-empty-reply-fa)
// and a full record of all 0xFF (ic7851-all-ff-record).
//
// An empty slot comes back as an EMPTY CHANNEL, never an error that would
// abort a caller's read of the whole radio.
func TestE2E_EmptyRadioOpensUnfingerprinted(t *testing.T) {
	for _, tc := range []struct {
		name         string
		opt          fakeic7851.Option
		fingerprints bool
	}{
		{"FA rejection", fakeic7851.WithEmptyReplyFA(), false},
		{"an all-FF record", fakeic7851.WithAllFFEmpty(), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newFake(t, tc.opt)
			s, _ := openFake(t, r, New7851)

			// THE TWO EMPTY READINGS REACH THE PROBE DIFFERENTLY, and
			// the difference is a fact about the wire rather than a
			// choice. An FA is consumed by the engine and returns NO
			// FRAME, so the probe learns nothing and the bounded search
			// runs to its end; an all-FF answer IS a record of this
			// profile's declared length, so the LENGTH fingerprint is
			// genuinely confirmed by it even though the slot is empty.
			// "This radio's records are 25 bytes" and "this slot holds
			// something" are different claims and only the first is what
			// the fingerprint makes.
			rep := s.OpenDiagnostics()
			if tc.fingerprints {
				if length, confirmed := s.Fingerprint(); !confirmed || length != civic7851.RecordOnlyLength {
					t.Errorf("Fingerprint() = (%d, %v), want (%d, true): an all-FF answer still settles the record LENGTH", length, confirmed, civic7851.RecordOnlyLength)
				}
			} else {
				if length, confirmed := s.Fingerprint(); confirmed || length != 0 {
					t.Errorf("Fingerprint() = (%d, %v), want (0, false): an FA teaches the probe nothing and the radio opens on ADDRESS evidence alone", length, confirmed)
				}
				if rep.Fingerprinted || rep.SlotsTried != probeSlotCount {
					t.Errorf("OpenDiagnostics() = %+v, want the whole bounded search run and no fingerprint", rep)
				}
			}
			for _, slot := range e2eSlots() {
				ch, err := s.ReadChannel(t.Context(), slot)
				if err != nil {
					t.Fatalf("ReadChannel %s on an empty radio: %v — an unset slot must come back empty, not as an error", slot, err)
				}
				if !ch.Empty() {
					t.Errorf("ReadChannel %s returned %+v, want an EMPTY channel", slot, ch.Data)
				}
				if ch.Slot != slot {
					t.Errorf("ReadChannel %s carried slot %q", slot, ch.Slot)
				}
			}
		})
	}
}

// checkReadAll seeds every one of the 101 slots, reads each back, and
// compares against what the test put in. Shared by the plain run, the
// USB-echo run and the flood run.
func checkReadAll(t *testing.T, newDriver func(...Option) driver.Driver, opts ...fakeic7851.Option) *Session {
	t.Helper()
	r := newFake(t, opts...)
	slots := e2eSlots()
	if len(slots) != 101 {
		t.Fatalf("this radio declares %d slots, want 101 (99 memories and 2 scan edges)", len(slots))
	}
	want := make(map[string]e2eFields, len(slots))
	for i, slot := range slots {
		f := e2eSeed(i)
		want[slot] = f
		r.SetSlot(slot, f.record(t))
	}

	s, _ := openFake(t, r, newDriver)
	for _, slot := range slots {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel %s: %v", slot, err)
		}
		if ch.Empty() {
			t.Fatalf("ReadChannel %s came back empty; the slot was seeded", slot)
		}
		f, d := want[slot], ch.Data
		if d.FreqHz != f.freqHz {
			t.Errorf("%s FreqHz = %d, want %d", slot, d.FreqHz, f.freqHz)
		}
		if d.Mode != f.mode {
			t.Errorf("%s Mode = %q, want %q", slot, d.Mode, f.mode)
		}
		if d.Filter != (codeplug.StringField{State: codeplug.Known, Value: f.filter}) {
			t.Errorf("%s Filter = %+v, want Known %q", slot, d.Filter, f.filter)
		}
		if d.ToneMode != (codeplug.StringField{State: codeplug.Known, Value: f.toneMode}) {
			t.Errorf("%s ToneMode = %+v, want Known %q", slot, d.ToneMode, f.toneMode)
		}
		if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(f.toneTx)}) {
			t.Errorf("%s ToneTx = %+v, want Known %d", slot, d.ToneTx, f.toneTx)
		}
		if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(f.toneRx)}) {
			t.Errorf("%s ToneRx = %+v, want Known %d", slot, d.ToneRx, f.toneRx)
		}
		if d.Tag != f.name {
			t.Errorf("%s Tag = %q, want %q", slot, d.Tag, f.name)
		}
		// E6: the two unmapped nibbles are Unavailable throughout, and so
		// is every field the record does not carry. Never Unknown, which
		// would claim the radio has one and this read did not learn it.
		for name, state := range map[string]codeplug.FieldState{
			"ScanSkip": d.ScanSkip.State, "DataMode": d.DataMode.State,
			"TagDisplay": d.TagDisplay.State, "CTCSSTone": d.CTCSSTone.State,
			"TxFreqHz": d.TxFreqHz.State, "Duplex": d.Duplex.State,
			"OffsetHz": d.OffsetHz.State, "DTCSCode": d.DTCSCode.State,
			"DTCSPolarity": d.DTCSPolarity.State,
		} {
			if state != codeplug.Unavailable {
				t.Errorf("%s %s = %q, want Unavailable", slot, name, state)
			}
		}
	}
	return s
}

// TestE2E_ReadsEveryOneOfThe101Slots covers the whole declared inventory
// on both rows: 001–099 and the two scan edges, which are two more values
// of the same two-byte selector (0100 and 0101) rather than a separate
// bank in the wire protocol.
func TestE2E_ReadsEveryOneOfThe101Slots(t *testing.T) {
	for _, c := range constructors {
		t.Run(c.model, func(t *testing.T) { checkReadAll(t, c.make) })
	}
}

// TestE2E_ExactEchoIsSuppressed drives a fake that echoes every frame it
// receives, which is what a linked USB/[REMOTE] pair does
// (register entry ic7851-echo-link-to-remote).
//
// SUPPRESSION IS BYTE IDENTITY against frames this program recorded
// sending, never a position or a count: a count-based rule would drop a
// real answer that happened to arrive in an echo's place.
func TestE2E_ExactEchoIsSuppressed(t *testing.T) {
	s := checkReadAll(t, New7851, fakeic7851.WithUSBEcho())
	stats := s.WireStats()
	if stats.Echoes == 0 {
		t.Error("no echoed frame was suppressed, although the radio echoed every frame it received")
	}
	if stats.Frames == 0 {
		t.Error("no frame reached the engine")
	}
}

// TestE2E_FloodsDoNotStarveTheSession runs the whole read-all under a
// continuous flood of frames the radio was not asked for.
//
// THE TWO FLOODS TAKE DIFFERENT PATHS ON PURPOSE. A BROADCAST flood
// (to = 00, the transceive form this driver assumes under register entry
// ic7851-broadcast-address-form) never reaches the engine at all: the
// accumulator counts and drops it, which is why the driver's diagnostics
// SUM the adapter's counter with the engine's rather than trusting either.
// A CONTROLLER-ADDRESSED flood does reach the engine and is what the
// bounded Init drain exists to survive.
func TestE2E_FloodsDoNotStarveTheSession(t *testing.T) {
	t.Run("broadcast", func(t *testing.T) {
		s := checkReadAll(t, New7851, fakeic7851.WithTransceiveFlood(500*time.Microsecond))
		if s.WireStats().Unexpected == 0 {
			t.Error("the adapter counted no unexpected frame under a broadcast flood")
		}
		if s.Diagnostics().UnexpectedFrames == 0 {
			t.Error("Diagnostics() reported a healthy zero on a saturated line — the adapter's count must be summed in")
		}
	})
	t.Run("controller-addressed", func(t *testing.T) {
		checkReadAll(t, New7851, fakeic7851.WithAddressedFlood(500*time.Microsecond))
	})
}

// TestE2E_ConsentedWriteAndReadback is the one acknowledged write in this
// suite, and it is only reachable through recorded consent: without
// WithConsentedUnverifiedWrites every mapped field is Unverified and the
// capability gate refuses before any wire traffic.
func TestE2E_ConsentedWriteAndReadback(t *testing.T) {
	for _, c := range constructors {
		for _, slot := range []string{"007", "P2"} {
			t.Run(c.model+"/"+slot, func(t *testing.T) {
				before := e2eSeed(7)

				// ONE FAKE PER SESSION. Closing a session closes the port
				// it was handed, and the fake has exactly one host end,
				// so three sessions against one radio would be three
				// sessions against a closed pipe.
				rPlain, rSim, r := newFake(t), newFake(t), newFake(t)
				for _, f := range []*fakeic7851.Radio{rPlain, rSim, r} {
					f.SetSlot(slot, before.record(t))
				}

				// WITHOUT CONSENT: refused, and no set on the wire.
				plain, plainTap := openFake(t, rPlain, c.make)
				after := e2eFields{
					freqHz: 21_205_000, mode: "CW", filter: "FIL3", toneMode: "TSQL",
					toneTx: 1000, toneRx: 1738, name: "AFTER 2",
				}
				ch := codeplug.Channel{Slot: slot, Data: &codeplug.ChannelData{
					FreqHz: after.freqHz, Mode: after.mode,
					Filter:   codeplug.StringField{State: codeplug.Known, Value: after.filter},
					ToneMode: codeplug.StringField{State: codeplug.Known, Value: after.toneMode},
					ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(after.toneTx)},
					ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(after.toneRx)},
					Tag:      after.name,
				}}
				sets := countSets(plainTap.frames())
				if _, err := plain.WriteChannel(t.Context(), ch); !errors.Is(err, driver.ErrWriteRefused) {
					t.Fatalf("an unconsented write returned %v, want driver.ErrWriteRefused", err)
				}
				if got := countSets(plainTap.frames()); got != sets {
					t.Errorf("an unconsented write put %d set frame(s) on the wire; a capability refusal precedes ALL wire traffic", got-sets)
				}

				// AND THE SIMULATED ARM IS NOT A BACK DOOR TO A RADIO.
				// It claims Supported writes about internal/fakeic7851
				// and about nothing else; the REAL-HARDWARE arm above is
				// what a physical radio gets, and it needs consent.
				sim, _ := openFake(t, rSim, c.make, WithSimulatedProfile())
				if !sim.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
					t.Error("the simulated arm cannot write the frequency it declares Supported")
				}
				if plain.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
					t.Error("the REAL-HARDWARE arm declares a writable field with both write-trial guards false")
				}

				// WITH CONSENT: one set, acknowledged, and the radio's own
				// record changes to match.
				s, tp := openFake(t, r, c.make, WithConsentedUnverifiedWrites())
				sets = countSets(tp.frames())
				res, err := s.WriteChannel(t.Context(), ch)
				if err != nil {
					t.Fatalf("WriteChannel: %v", err)
				}
				if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed || res.Steps[0].Command != memorySetStep {
					t.Fatalf("WriteResult = %+v, want one sent and confirmed %s step", res.Steps, memorySetStep)
				}
				if got := countSets(tp.frames()) - sets; got != 1 {
					t.Errorf("the write put %d set frames on the wire, want exactly 1", got)
				}

				// THE RADIO'S OWN BYTES, not just the driver's readback.
				state, ok := r.SlotState(slot)
				if !ok {
					t.Fatalf("the fake holds no record for %s after the write", slot)
				}
				if want := after.record(t); !bytes.Equal(state.Raw, want) {
					t.Errorf("the radio now holds % X, want % X", state.Raw, want)
				}

				// AND THE READBACK AGREES. WriteChannel performs none of
				// this itself — verification is the clone service's job —
				// so the read is a separate exchange.
				got, err := s.ReadChannel(t.Context(), slot)
				if err != nil {
					t.Fatalf("ReadChannel after the write: %v", err)
				}
				if got.Data == nil || got.Data.FreqHz != after.freqHz || got.Data.Mode != after.mode || got.Data.Tag != after.name {
					t.Errorf("readback = %+v, want the record just written", got.Data)
				}
			})
		}
	}
}

// TestE2E_WrongRecordLengthRefusesTheRadio drives a fake configured to
// answer with a record of a length this profile does not declare.
//
// The refusal NAMES NO FOUND MODEL: cross-model record-length
// distinctness is a tier-level Wave-4 check and this package holds no
// table of other radios' lengths. It still satisfies
// errors.Is(err, driver.ErrWrongRadio), because whatever is on this port,
// its memory records are not the shape this driver writes.
func TestE2E_WrongRecordLengthRefusesTheRadio(t *testing.T) {
	for _, length := range []int{24, 26} {
		t.Run(fmt.Sprintf("%d-byte records", length), func(t *testing.T) {
			r := newFake(t, fakeic7851.WithRecordLength(length))
			r.SetSlot("001", make([]byte, length))

			_, _, err := tryOpenFake(t, r, New7851)
			if !errors.Is(err, driver.ErrWrongRadio) {
				t.Fatalf("Open against a %d-byte radio returned %v, want driver.ErrWrongRadio", length, err)
			}
			var e *RecordLengthMismatchError
			if !errors.As(err, &e) {
				t.Fatalf("Open returned %v, want *RecordLengthMismatchError", err)
			}
			if e.Got != length || e.Want != civic7851.RecordOnlyLength {
				t.Errorf("RecordLengthMismatchError = %+v, want got %d want %d", e, length, civic7851.RecordOnlyLength)
			}
		})
	}
}

// TestE2E_BothEraseFormsAreRefused covers the two clear shapes this
// document PRINTS and this tier does not ship — the 1A 00 + FF form and
// top-level command 0B — from both ends.
//
// From the DRIVER's end: erase is Channel.Data == nil, FieldErase carries
// the zero FieldSupport in both capability arms, and
// spec.ConsentUnverifiedWrites structurally never reaches it, so the
// refusal stands under consent too and costs no wire traffic. From the
// WIRE's end: neither form is ever emitted, which the byte log settles.
func TestE2E_BothEraseFormsAreRefused(t *testing.T) {
	r := newFake(t)
	r.SetSlot("001", e2eSeed(1).record(t))
	s, tp := openFake(t, r, New7851, WithConsentedUnverifiedWrites())

	// Exercise the whole read/write choreography first, so the byte log
	// below is a log of this driver working rather than of it idle.
	if _, err := s.ReadChannel(t.Context(), "001"); err != nil {
		t.Fatal(err)
	}
	res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "001"})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("an erase returned %v, want driver.ErrWriteRefused", err)
	}
	if len(res.Steps) != 0 {
		t.Errorf("an erase produced steps %+v; the refusal precedes all wire traffic", res.Steps)
	}
	var refusal *driver.WriteRefusedError
	if errors.As(err, &refusal) {
		if len(refusal.Fields) != 1 || refusal.Fields[0] != spec.FieldErase {
			t.Errorf("the erase refusal named %v, want FieldErase alone", refusal.Fields)
		}
	}

	for _, f := range tp.frames() {
		if f[4] == 0x09 || f[4] == 0x0a || f[4] == 0x0b {
			t.Errorf("the driver emitted the top-level clear form % X", f)
		}
		if f[4] == 0x1a && f[5] == 0x00 && len(f) == 10 && f[8] == 0xff {
			t.Errorf("the driver emitted the 1A 00 + FF clear form % X", f)
		}
		if f[4] == 0x1a && f[5] != 0x00 {
			t.Errorf("the driver emitted a 1A sub-command other than 00: % X", f)
		}
	}

	// AND THE RADIO REFUSES BOTH FORMS ANYWAY, which is what makes this a
	// property of the exchange and not only of this driver. Written
	// straight to a second fake's port, since nothing in this programme
	// can build either frame.
	r2 := fakeic7851.New()
	defer func() { _ = r2.Close() }()
	port := r2.Port()
	for _, tc := range []struct {
		name  string
		frame []byte
	}{
		{"1A 00 + FF", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01, 0xff, 0xfd}},
		{"top-level 0B", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x0b, 0xfd}},
		{"top-level 09", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x09, 0xfd}},
		{"top-level 0A", []byte{0xfe, 0xfe, 0x8e, 0xe0, 0x0a, 0xfd}},
	} {
		if err := port.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := port.Write(tc.frame); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		buf := make([]byte, 64)
		n, err := port.Read(buf)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if want := []byte{0xfe, 0xfe, 0xe0, 0x8e, 0xfa, 0xfd}; !bytes.Equal(buf[:n], want) {
			t.Errorf("%s was answered % X, want the rejection % X", tc.name, buf[:n], want)
		}
	}
}

// TestE2E_AcknowledgementTimeoutSendsExactlyOneSet is the never-retransmit
// rule, end to end.
//
// A lost write's outcome is genuinely ambiguous, and resending one is how
// a radio ends up written twice — so the write spec fixes its retry count
// at zero and the engine refuses a non-zero one before writing anything.
// The tap swallows the set frame so the radio never answers, and what
// proves the rule is the BYTE LOG showing exactly one set, not the result
// flags.
//
// BOTH RESULT FLAGS ARE FALSE on this path, and deliberately: a false Sent
// means "the outcome is NOT known-clean", which is exactly a write whose
// acknowledgement never came.
func TestE2E_AcknowledgementTimeoutSendsExactlyOneSet(t *testing.T) {
	r := newFake(t)
	r.SetSlot("003", e2eSeed(3).record(t))
	s, tp := openFake(t, r, New7851, WithConsentedUnverifiedWrites())

	before := countSets(tp.frames())
	tp.dropSets.Store(true)
	res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "003", Data: &codeplug.ChannelData{
		FreqHz: 7_100_000, Mode: "LSB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(1000)},
		Tag:      "LOST",
	}})
	if err == nil {
		t.Fatal("a write whose acknowledgement never arrived reported success")
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("WriteChannel returned %v, want a transport timeout", err)
	}
	if got := countSets(tp.frames()) - before; got != 1 {
		t.Fatalf("the lost write put %d set frames on the wire, want exactly 1 — a write is NEVER resent", got)
	}
	if len(res.Steps) != 1 || res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("WriteResult = %+v, want one step with BOTH flags false: the outcome is unknown, not attributable", res.Steps)
	}

	// AND THE RADIO'S RECORD IS UNCHANGED, because the frame never
	// reached it. The point of the case is that the driver cannot know
	// that, and does not guess.
	state, _ := r.SlotState("003")
	if !bytes.Equal(state.Raw, e2eSeed(3).record(t)) {
		t.Error("the radio's record changed although the set frame was swallowed")
	}
}

// TestE2E_MovedAddressTimesOutCleanly is spec D3.3's limitation, observed.
//
// This driver builds every frame for 8E and the codec refuses any answer
// not from 8E; there is no --civ-address option and no address sweep. A
// radio whose owner has moved it is therefore UNREACHABLE, and the failure
// is a clean timeout rather than a wrong-radio refusal: nothing was heard
// from, so nothing can be attributed. A wrong default-baud guess fails
// identically, which is what makes that guess safe.
func TestE2E_MovedAddressTimesOutCleanly(t *testing.T) {
	r := newFake(t, fakeic7851.WithRadioAddress(0x94))
	r.SetSlot("001", e2eSeed(1).record(t))

	_, tp, err := tryOpenFake(t, r, New7851)
	if err == nil {
		t.Fatal("Open succeeded against a radio at another CI-V address")
	}
	if errors.Is(err, driver.ErrWrongRadio) {
		t.Errorf("Open returned a wrong-radio refusal (%v); nothing was heard from, so nothing can be attributed", err)
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("Open returned %v, want a clean timeout", err)
	}
	for _, f := range tp.frames() {
		if f[2] != 0x8e {
			t.Errorf("the driver addressed a frame to %#02x: % X", f[2], f)
		}
	}
}

// TestE2E_AnswerForAnotherChannelIsRefused is tier ruling T2 through the
// whole stack: the landed memory-answer matcher is ENVELOPE-ONLY, so an
// answer about channel 7 satisfies the spec for a read of channel 3
// perfectly well and the DRIVER is what must compare the addresses.
//
// The mis-addressed answer is injected on the wire alongside a working
// radio, so the check is exercised where it actually runs — before empty
// recognition, before mapping, and before any write merge.
func TestE2E_AnswerForAnotherChannelIsRefused(t *testing.T) {
	seed := e2eSeed(5)
	rec := seed.record(t)
	// The answer envelope: from the radio, to the controller, naming
	// channel 7 while the driver asked about channel 3.
	answer := append([]byte{0xfe, 0xfe, 0xe0, 0x8e, 0x1a, 0x00, 0x00, 0x07}, rec...)
	answer = append(answer, 0xfd)

	host, radio := net.Pipe()
	defer func() { _ = host.Close() }()
	go func() {
		defer func() { _ = radio.Close() }()
		buf := make([]byte, 256)
		for {
			n, err := radio.Read(buf)
			if err != nil {
				return
			}
			in := append([]byte(nil), buf[:n]...)
			switch {
			case len(in) >= 6 && in[4] == 0x19:
				_, _ = radio.Write([]byte{0xfe, 0xfe, 0xe0, 0x8e, 0x19, 0x00, 0x8e, 0xfd})
			case len(in) == 9 && in[4] == 0x1a && in[7] == 0x03:
				// THE ONE MIS-ATTRIBUTION. Every other read is answered
				// correctly, so the session opens normally and the T2
				// check is exercised on a working bus rather than on a
				// radio that is plainly broken.
				_, _ = radio.Write(answer)
			case len(in) == 9 && in[4] == 0x1a:
				honest := append([]byte{0xfe, 0xfe, 0xe0, 0x8e, 0x1a, 0x00, in[6], in[7]}, rec...)
				_, _ = radio.Write(append(honest, 0xfd))
			default:
				_, _ = radio.Write([]byte{0xfe, 0xfe, 0xe0, 0x8e, 0xfa, 0xfd})
			}
		}
	}()

	sess, err := New7851(WithSimulatedProfile()).Open(t.Context(), host, driver.Identity{Port: "/dev/pipe"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := sess.(*Session)
	defer func() { _ = s.Close() }()

	_, err = s.ReadChannel(t.Context(), "003")
	var e *AnswerMismatchError
	if !errors.As(err, &e) {
		t.Fatalf("ReadChannel got %v, want *AnswerMismatchError", err)
	}
	if e.Want.Channel != 3 || e.Got.Channel != 7 {
		t.Errorf("AnswerMismatchError = %+v, want want-3 got-7", e)
	}
	if !errors.Is(err, ErrAnswerMismatch) {
		t.Error("the refusal does not satisfy errors.Is(err, ErrAnswerMismatch)")
	}
	if s.AnswerMismatches() == 0 {
		t.Error("the mis-addressed answer was not counted")
	}
}

// TestE2E_FixedDigitRecordIsRefusedOnTheWire is F1's read-side refusal,
// end to end: a radio answering with a digit in printed ⑧, ⑫ or ⑮ has
// answered with a record this document does not draw, and reading it would
// hand the caller a frequency 100 MHz out or a tone the write path would
// then quietly zero.
func TestE2E_FixedDigitRecordIsRefusedOnTheWire(t *testing.T) {
	for _, off := range []int{civic7851.FreqFixedOffset, civic7851.ToneTXFixedOffset, civic7851.ToneRXFixedOffset} {
		t.Run(fmt.Sprintf("record byte %d", off), func(t *testing.T) {
			r := newFake(t)
			r.SetSlot("001", e2eSeed(1).record(t))
			bad := e2eSeed(2).record(t)
			bad[off] = 0x01
			r.SetSlot("002", bad)

			s, _ := openFake(t, r, New7851)
			_, err := s.ReadChannel(t.Context(), "002")
			var e *FixedDigitError
			if !errors.As(err, &e) {
				t.Fatalf("ReadChannel got %v, want *FixedDigitError", err)
			}
			if e.Offset != off || e.Got != 0x01 {
				t.Errorf("FixedDigitError = %+v, want offset %d got 0x01", e, off)
			}
			// AND THE GOOD SLOT STILL READS. The refusal is per record,
			// not a session this driver has given up on.
			if _, err := s.ReadChannel(t.Context(), "001"); err != nil {
				t.Errorf("the untouched slot failed too: %v", err)
			}
		})
	}
}

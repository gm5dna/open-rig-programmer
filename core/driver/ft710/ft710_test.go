// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// These tests drive the ft710 driver against fakeradio — an independent,
// byte-level CAT peer — through the driver's own real transport.Engine
// (constructed inside Open). fakeradio is imported in _test files only.

const testCtxTimeout = 30 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// testIdentity is the caller-side Identity every test session opens with.
var testIdentity = driver.Identity{Port: "fake-pipe", USBSerial: "SIM0001"}

// openSession constructs a fakeradio.Radio with opts, opens an ft710
// session over its port with the given profile, and registers cleanup for
// both. It returns the Radio (for SlotState/fault setup) and the concrete
// *Session (for Region()).
func openSession(t *testing.T, profile Profile, opts ...fakeradio.Option) (*fakeradio.Radio, *Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })

	sess, err := New(profile).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	fs, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned a %T, want *ft710.Session", sess)
	}
	return r, fs
}

// minimalImage is a factory image with M-01 only: no 60m channels, no
// EMG. Discovery against it finds nothing (region "no-60m" —
// HW-CONFIRMED 2026-07-13, docs/hardware-notes.md §60m regional finding
// — a zero-60m/zero-EMG inventory is a known real UK FT-710 variant, not
// an anomaly), which also pins the wire-exchange numbering
// write_test.go's fault tests rely on (AI0=1, ID=2, MR501=3 rejected,
// MREMG=4 rejected).
func minimalImage() map[string]fakeradio.MemState {
	return map[string]fakeradio.MemState{
		"001": {
			Freq: "007000000", ClarSign: '+', ClarMag: "0000",
			Mode: '1', Kind: '1', CTCSS: '0', Shift: '0',
			Populated: true,
		},
	}
}

// TestOpen_DefaultImage_NoSixtyMetreBank exercises the DEFAULT fakeradio
// image (ImageUK) via the production wiring path (no WithFactoryImage
// override), which is what --fake/demo-mode sessions actually get.
// HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md §60m regional finding):
// Stuart's UK FT-710 has no 5xx bank and no EMG channel at all, so
// ImageUK no longer synthesises one (see image.go) — this REPLACES the
// former TestOpen_UKImage, which asserted the opposite (a 7-channel 60m
// bank) against the old, now-corrected image. Tests that still want 60m/
// EMG bank DISCOVERY coverage use ImageUS explicitly (see
// TestOpen_USImage below).
func TestOpen_DefaultImage_NoSixtyMetreBank(t *testing.T) {
	_, sess := openSession(t, Simulated) // fakeradio.New defaults to ImageUK

	if got := sess.Region(); got != "no-60m" {
		t.Errorf("Region() = %q, want \"no-60m\"", got)
	}

	id := sess.Identity()
	if id.CATID != "0800" {
		t.Errorf("Identity().CATID = %q, want \"0800\" (from the ID; probe)", id.CATID)
	}
	if id.Port != testIdentity.Port || id.USBSerial != testIdentity.USBSerial {
		t.Errorf("Identity() = %+v, want caller-supplied Port/USBSerial preserved (%+v)", id, testIdentity)
	}

	caps := sess.Capabilities()
	if err := caps.Validate(); err != nil {
		t.Errorf("effective Capabilities().Validate() = %v, want nil", err)
	}

	if _, ok := caps.Bank(spec.Bank60m); ok {
		t.Error("effective caps contain a discovered 60M bank on the default (UK) image — HW-CONFIRMED: UK radios have no 5xx bank")
	}
	if _, ok := caps.Bank(spec.BankEMG); ok {
		t.Error("effective caps contain an EMG bank on the default (UK) image — UK radios have no EMG channel")
	}
}

func TestOpen_USImage(t *testing.T) {
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(fakeradio.ImageUS))

	if got := sess.Region(); got != "US" {
		t.Errorf("Region() = %q, want \"US\"", got)
	}

	caps := sess.Capabilities()
	if err := caps.Validate(); err != nil {
		t.Errorf("effective Capabilities().Validate() = %v, want nil", err)
	}

	bank, ok := caps.Bank(spec.Bank60m)
	if !ok {
		t.Fatal("effective caps missing the discovered 60M bank")
	}
	if len(bank.Slots) != 15 || bank.Slots[0] != "501" || bank.Slots[14] != "515" {
		t.Errorf("60M bank slots = %v, want [\"501\"..\"515\"] (15 slots)", bank.Slots)
	}

	emg, ok := caps.Bank(spec.BankEMG)
	if !ok {
		t.Fatal("effective caps missing the discovered EMG bank")
	}
	if len(emg.Slots) != 1 || emg.Slots[0] != "EMG" {
		t.Errorf("EMG bank slots = %v, want [\"EMG\"]", emg.Slots)
	}
}

// TestOpen_60mOverflowSentinel (M3 Codex-review fix wave, Fix 6): 60m
// discovery probes at most max60mProbe (15) slots — the largest KNOWN
// regional set — but nothing guarantees a real radio (or a future
// firmware/region) stops there, and silently reporting "US" for a radio
// with MORE than 15 populated 60m channels would misdescribe its
// inventory as complete. When all 15 answer, discovery probes ONE
// sentinel slot ("516"); if that ALSO answers, the bank is capped at the
// 15 verified slots, the region reports "unknown-16plus" (never "US"),
// and the anomaly is logged via the driver's transport logger. Doctored
// fake: ImageUS (501-515 + EMG) with "516" overlaid construction-time via
// WithSlot (which must come AFTER WithFactoryImage — see its doc
// comment).
func TestOpen_60mOverflowSentinel(t *testing.T) {
	logger := &recordingLogger{}
	r := fakeradio.New(
		fakeradio.WithFactoryImage(fakeradio.ImageUS),
		fakeradio.WithSlot("516", fakeradio.MemState{
			Freq: "005560000", ClarSign: '+', ClarMag: "0000",
			Mode: '2', Kind: '1', CTCSS: '0', Shift: '0',
			Populated: true,
		}),
	)
	t.Cleanup(func() { _ = r.Close() })

	sess, err := New(Simulated, WithTransportLogger(logger)).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	fs := sess.(*Session)

	if got := fs.Region(); got != "unknown-16plus" {
		t.Errorf("Region() = %q, want \"unknown-16plus\" (sentinel slot 516 answered — this is NOT a verified US inventory)", got)
	}

	caps := sess.Capabilities()
	if err := caps.Validate(); err != nil {
		t.Errorf("effective Capabilities().Validate() = %v, want nil", err)
	}
	bank, ok := caps.Bank(spec.Bank60m)
	if !ok {
		t.Fatal("effective caps missing the discovered 60M bank")
	}
	if len(bank.Slots) != 15 || bank.Slots[0] != "501" || bank.Slots[14] != "515" {
		t.Errorf("60M bank slots = %v, want capped at the 15 VERIFIED slots [\"501\"..\"515\"] — the sentinel proves more exist, but only verified slots may be claimed", bank.Slots)
	}

	found := false
	for _, rec := range logger.snapshot() {
		if strings.Contains(rec, "516") {
			found = true
		}
	}
	if !found {
		t.Errorf("logger records = %v, want one mentioning the sentinel slot 516 overflow", logger.snapshot())
	}
}

func TestOpen_MinimalImage_UnknownRegion(t *testing.T) {
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

	if got := sess.Region(); got != "no-60m" {
		t.Errorf("Region() = %q, want \"no-60m\" (HW-CONFIRMED 2026-07-13: a zero-60m/zero-EMG inventory is a known real variant)", got)
	}
	caps := sess.Capabilities()
	if err := caps.Validate(); err != nil {
		t.Errorf("effective Capabilities().Validate() = %v, want nil", err)
	}
	if _, ok := caps.Bank(spec.Bank60m); ok {
		t.Error("effective caps contain a 60M bank with nothing discovered")
	}
	if _, ok := caps.Bank(spec.BankEMG); ok {
		t.Error("effective caps contain an EMG bank with nothing discovered")
	}
}

// TestDeriveRegion (Codex M5a fix wave, Fix 5): a table-driven unit test
// against deriveRegion directly — the retained seven-channel "UK" branch
// lost direct coverage when TestOpen_UKImage was replaced by
// TestOpen_DefaultImage_NoSixtyMetreBank (M5a's HW-CONFIRMED "no-60m"
// finding), and nothing else in this file calls deriveRegion(7, false,
// false) at all. This does not exercise Open/discovery — see
// TestOpen_USImage, TestOpen_DefaultImage_NoSixtyMetreBank,
// TestOpen_MinimalImage_UnknownRegion and TestOpen_60mOverflowSentinel
// above for that — it pins deriveRegion's own switch, case by case.
func TestDeriveRegion(t *testing.T) {
	tests := []struct {
		name             string
		count60m         int
		emg, overflow60m bool
		want             string
	}{
		{
			// ASSUMED (never HW-confirmed — M5a characterised only the
			// zero-60m/zero-EMG UK variant): mirrors fakeradio's ImageUK
			// factory shape, 7 60m channels, no EMG.
			name:     "UK fingerprint (7, no EMG) - ASSUMED",
			count60m: 7, emg: false, overflow60m: false,
			want: "UK",
		},
		{
			// ASSUMED: mirrors fakeradio's ImageUS factory shape, 15 60m
			// channels plus EMG.
			name:     "US fingerprint (15, with EMG) - ASSUMED",
			count60m: 15, emg: true, overflow60m: false,
			want: "US",
		},
		{
			// HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md §60m
			// regional finding): Stuart's real UK FT-710 has neither a
			// 5xx bank nor an EMG channel.
			name:     "no-60m (0, no EMG) - HW-CONFIRMED",
			count60m: 0, emg: false, overflow60m: false,
			want: "no-60m",
		},
		{
			name:     "generic unrecognised shape (3, no EMG) falls to unknown-N",
			count60m: 3, emg: false, overflow60m: false,
			want: "unknown-3",
		},
		{
			name:     "15 without EMG does not match the US fingerprint",
			count60m: 15, emg: false, overflow60m: false,
			want: "unknown-15",
		},
		{
			name:     "7 with EMG does not match the UK fingerprint",
			count60m: 7, emg: true, overflow60m: false,
			want: "unknown-7",
		},
		{
			// Fix 6 (M3 review): the overflow sentinel overrides
			// everything else, however US-like the first 15 slots look.
			name:     "overflow sentinel overrides a US-shaped count",
			count60m: 15, emg: true, overflow60m: true,
			want: "unknown-16plus",
		},
		{
			name:     "overflow sentinel overrides even a zero count",
			count60m: 0, emg: false, overflow60m: true,
			want: "unknown-16plus",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := deriveRegion(tt.count60m, tt.emg, tt.overflow60m); got != tt.want {
				t.Errorf("deriveRegion(%d, %v, %v) = %q, want %q", tt.count60m, tt.emg, tt.overflow60m, got, tt.want)
			}
		})
	}
}

// TestOpen_DiscoveredBanksReadOnly: whatever the profile — including
// Simulated, whose MEM/PMS fields ARE writable — every field of a
// discovered 60M/EMG bank must be read-only: MW cannot target 5xx/EMG
// slots at all (cat.Slot.Writable), so no capability data may claim
// otherwise.
func TestOpen_DiscoveredBanksReadOnly(t *testing.T) {
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(fakeradio.ImageUS))
	caps := sess.Capabilities()

	for _, bankID := range []spec.BankID{spec.Bank60m, spec.BankEMG} {
		bank, ok := caps.Bank(bankID)
		if !ok {
			t.Fatalf("missing bank %s", bankID)
		}
		for f := range bank.Fields {
			if bank.Fields[f].CanWrite() {
				t.Errorf("discovered bank %s field %s: CanWrite() = true, want false (read-only bank)", bankID, f)
			}
		}
	}
}

// TestSession_CapabilitiesDefensiveCopy: mutating the Capabilities a
// session hands out must never alter what the session itself enforces —
// the returned value is the caller's own copy, not a window onto the
// write gate's data. Post-M5b-flip the tamper targets FieldCTCSSTone
// (Unsupported in every profile — the CAT protocol cannot write it)
// rather than the old FieldFrequency, which is now LEGITIMATELY
// writable on RealHardware and so could no longer witness a leak.
func TestSession_CapabilitiesDefensiveCopy(t *testing.T) {
	_, sess := openSession(t, RealHardware)

	tampered := sess.Capabilities()
	for i := range tampered.Banks {
		tampered.Banks[i].Fields[spec.FieldCTCSSTone] = spec.FieldSupport{
			Read: spec.Supported, Write: spec.Supported,
		}
	}

	fresh := sess.Capabilities()
	if fresh.FieldSupport(spec.BankMemory, spec.FieldCTCSSTone).CanWrite() {
		t.Fatal("mutating a returned Capabilities leaked into the session's own copy — the write gate can be corrupted from outside")
	}
}

// TestCloneCapabilities_VocabIndependence checks that cloneCapabilities
// deep-copies ShiftOptions and CTCSSStates (task 38/M9a-2): mutating
// either slice on the clone must never be observable through the
// original Capabilities value or through a second, separate clone —
// exactly the load-bearing guarantee cloneCapabilities' doc comment
// already claims for Modes/CTCSSTones/Bauds/RequiredSlots, now extended
// to the two vocab slices.
func TestCloneCapabilities_VocabIndependence(t *testing.T) {
	orig := spec.Capabilities{
		ShiftOptions: []string{"SIMPLEX", "PLUS", "MINUS"},
		CTCSSStates: []spec.ToneState{
			{Value: "OFF", RequiresTone: false},
			{Value: "ENC-DEC", RequiresTone: true},
			{Value: "ENC", RequiresTone: true},
		},
	}

	clone := cloneCapabilities(orig)
	clone.ShiftOptions[0] = "TAMPERED"
	clone.CTCSSStates[0] = spec.ToneState{Value: "TAMPERED", RequiresTone: true}
	clone.ShiftOptions = append(clone.ShiftOptions, "EXTRA")
	clone.CTCSSStates = append(clone.CTCSSStates, spec.ToneState{Value: "EXTRA"})

	if orig.ShiftOptions[0] != "SIMPLEX" {
		t.Errorf("orig.ShiftOptions[0] = %q after mutating a clone, want unaffected %q", orig.ShiftOptions[0], "SIMPLEX")
	}
	if len(orig.ShiftOptions) != 3 {
		t.Errorf("len(orig.ShiftOptions) = %d after appending to a clone, want unaffected 3", len(orig.ShiftOptions))
	}
	if orig.CTCSSStates[0] != (spec.ToneState{Value: "OFF", RequiresTone: false}) {
		t.Errorf("orig.CTCSSStates[0] = %+v after mutating a clone, want unaffected {OFF false}", orig.CTCSSStates[0])
	}
	if len(orig.CTCSSStates) != 3 {
		t.Errorf("len(orig.CTCSSStates) = %d after appending to a clone, want unaffected 3", len(orig.CTCSSStates))
	}

	// A second, independent clone of the (still-unaffected) original must
	// also be unaffected — confirming the tampering never reached the
	// shared source either.
	again := cloneCapabilities(orig)
	if again.ShiftOptions[0] != "SIMPLEX" || len(again.ShiftOptions) != 3 {
		t.Errorf("cloneCapabilities(orig).ShiftOptions = %v after a prior clone was mutated, want unaffected [SIMPLEX PLUS MINUS]", again.ShiftOptions)
	}
	if again.CTCSSStates[0] != (spec.ToneState{Value: "OFF", RequiresTone: false}) || len(again.CTCSSStates) != 3 {
		t.Errorf("cloneCapabilities(orig).CTCSSStates = %+v after a prior clone was mutated, want unaffected", again.CTCSSStates)
	}
}

func TestSession_CloseIdempotent(t *testing.T) {
	r := fakeradio.New()
	t.Cleanup(func() { _ = r.Close() })

	sess, err := New(Simulated).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("first Close() = %v, want nil", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil (idempotent)", err)
	}
}

// TestOpen_WrongRadio: a raw net.Pipe stub that answers every ID; probe
// with ID0761; (an FT-DX10's CAT ID) and stays silent otherwise. Open
// must fail with ErrWrongRadio carrying what was found, and must close
// the port it took ownership of.
func TestOpen_WrongRadio(t *testing.T) {
	host, remote := net.Pipe()
	t.Cleanup(func() { _ = host.Close(); _ = remote.Close() })

	go func() {
		buf := make([]byte, 256)
		var acc []byte
		for {
			n, err := remote.Read(buf)
			if n > 0 {
				acc = append(acc, buf[:n]...)
				for {
					i := bytes.IndexByte(acc, ';')
					if i < 0 {
						break
					}
					frame := string(acc[:i+1])
					acc = acc[i+1:]
					if frame == "ID;" {
						_, _ = remote.Write([]byte("ID0761;"))
					}
					// Anything else (AI0;) gets silence — which IS the
					// fire-and-forget success signal.
				}
			}
			if err != nil {
				return
			}
		}
	}()

	_, err := New(RealHardware).Open(testCtx(t), host, driver.Identity{Port: "pipe"})
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open = %v, want errors.Is match against driver.ErrWrongRadio", err)
	}
	var wre *driver.WrongRadioError
	if !errors.As(err, &wre) {
		t.Fatalf("Open error %v is not a *driver.WrongRadioError", err)
	}
	if wre.Got != "0761" || wre.Want != "0800" {
		t.Errorf("WrongRadioError = got %q want %q; expected got \"0761\" want \"0800\"", wre.Got, wre.Want)
	}

	// Open took ownership of the port and failed: it must have closed it.
	if _, werr := host.Write([]byte("x")); werr == nil {
		t.Error("host port still writable after a failed Open — Open must close the port on error")
	}
}

// TestDriver_ModelAndBaseline: the driver's static baseline is
// profile-selected and contains no discovered banks. Post-M5b-flip, the
// RealHardware case's expectation changed from CanWrite false (the old
// write-guard pin) to true (the trials verified the write path — see
// TestWriteTrialsComplete_FlippedTrue_M5b); the SAFETY property this
// table still pins is the last row: an unrecognised Profile value must
// NEVER select a writable capability set, flip or no flip.
func TestDriver_ModelAndBaseline(t *testing.T) {
	tests := []struct {
		name         string
		profile      Profile
		wantWritable bool // FieldFrequency on MEM
	}{
		{"RealHardware -> hardware-verified (M5b flip)", RealHardware, true},
		{"Simulated -> Simulated", Simulated, true},
		{"unrecognised Profile fails safe (all-Unverified, nothing writable)", Profile(99), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := New(tt.profile)
			if d.Model() != "FT-710" {
				t.Errorf("Model() = %q, want \"FT-710\"", d.Model())
			}
			caps := d.Capabilities()
			if err := caps.Validate(); err != nil {
				t.Errorf("baseline Capabilities().Validate() = %v, want nil", err)
			}
			if _, ok := caps.Bank(spec.Bank60m); ok {
				t.Error("baseline contains a 60M bank — discovery is per-session")
			}
			got := caps.FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite()
			if got != tt.wantWritable {
				t.Errorf("MEM frequency CanWrite() = %v, want %v", got, tt.wantWritable)
			}
		})
	}
}

// countingPort wraps a Port and counts Write calls, so a test can assert
// that a refused operation produced ZERO wire traffic.
type countingPort struct {
	inner  io.ReadWriteCloser
	writes atomic.Int64
}

func (p *countingPort) Read(b []byte) (int, error) { return p.inner.Read(b) }
func (p *countingPort) Write(b []byte) (int, error) {
	p.writes.Add(1)
	return p.inner.Write(b)
}
func (p *countingPort) Close() error { return p.inner.Close() }

// openCountingSession is openSession over a countingPort, for the
// zero-wire-traffic refusal tests.
func openCountingSession(t *testing.T, profile Profile, opts ...fakeradio.Option) (*countingPort, *Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	cp := &countingPort{inner: r.Port()}

	sess, err := New(profile).Open(testCtx(t), cp, testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return cp, sess.(*Session)
}

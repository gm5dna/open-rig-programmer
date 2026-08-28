// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// testCtxTimeout bounds every test's context — generous, since a full
// MEM+PMS ReadAll over fakeradio (117+ slots, each paced by the
// transport's DefaultSettle) genuinely takes a few real seconds.
const testCtxTimeout = 60 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// testIdentity is the caller-side Identity every test session opens with.
var testIdentity = driver.Identity{Port: "fake-pipe", USBSerial: "SIM0001"}

// minimalFactoryImage is a factory image with ONLY M-01 populated — no
// other MEM slot, no 60m channels, no EMG, and (Codex M5b fix wave, Fix 3,
// adjudicated HIGH) EVERY PMS slot left EMPTY: real radios ship
// all-PMS-empty (M5a's characterised radio began with all 18 PMS slots
// empty, and M5b created only P1L, leaving the rest untouched — see
// docs/hardware-notes.md), so an all-populated PMS bank was never a
// realistic factory shape — it existed only because the PMS bank used to
// carry NoBlank:true, which codeplug.Validate would otherwise have
// rejected an empty-PMS baseline against. NoBlank has been removed from
// every profile (core/driver/ft710/caps.go) precisely because it made
// the newly-armed real-radio write path nonfunctional against the very
// radio that supplied its evidence — this factory image now matches
// that reality instead of papering over it. Tests that specifically need
// a populated PMS slot (e.g. the RealHardware real-shape test) overlay
// one via fakeradio.WithSlot, exactly as the observed radio's own shape
// does (P1L alone). Discovery against this image finds nothing
// (Region() "no-60m" — HW-CONFIRMED 2026-07-13 label for a
// zero-60m/zero-EMG inventory, see docs/hardware-notes.md). This keeps
// tests that do not specifically care about a realistic factory image
// (most of them: PrepareSend/Execute exercise the choreography, not the
// read mapping) close to as fast as a full 117-slot ReadAll can be.
func minimalFactoryImage() map[string]fakeradio.MemState {
	return map[string]fakeradio.MemState{
		"001": {
			Freq: "007000000", ClarSign: '+', ClarMag: "0000",
			Mode: '1', Kind: '1', CTCSS: '0', Shift: '0',
			Populated: true,
		},
	}
}

// happyPathImage is minimalFactoryImage plus a SECOND populated MEM slot,
// "005" — used by tests that need two pre-existing, independently
// modifiable MEM channels (e.g. "mutate 2 channels + add 1").
func happyPathImage() map[string]fakeradio.MemState {
	slots := minimalFactoryImage()
	slots["005"] = fakeradio.MemState{
		Freq: "014000000", ClarSign: '+', ClarMag: "0000",
		Mode: '2', Kind: '1', CTCSS: '0', Shift: '0',
		Populated: true,
	}
	return slots
}

// minimalFactoryPopulated mirrors, as codeplug.ChannelData, exactly what
// minimalFactoryImage's MemState values decode to via a real ReadChannel:
// "001" at 7.000000 MHz LSB, every PMS slot empty (real shape — see
// minimalFactoryImage's doc comment). Kept as its OWN function (not
// derived from the image) so matchingCandidateFile can build a candidate
// matching a given factory image WITHOUT ever reading the radio — see
// matchingCandidateFile's doc comment.
func minimalFactoryPopulated() map[string]*codeplug.ChannelData {
	m := map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 7_000_000, "").Data,
	}
	m["001"].Mode = "LSB"
	return m
}

// happyPathPopulated is minimalFactoryPopulated plus "005" at 14.000000
// MHz USB, matching happyPathImage.
func happyPathPopulated() map[string]*codeplug.ChannelData {
	m := minimalFactoryPopulated()
	m["005"] = writableChannel("005", 14_000_000, "").Data
	return m
}

// matchingCandidateFile builds a *codeplug.Codeplug describing EXACTLY
// populated's state for every slot caps lists (so codeplug.Diff reports
// every one of those slots Unchanged), except each slot named in edits,
// which carries that entry's Data instead (nil erases/leaves it empty). A
// slot in neither map is left empty. Building the candidate this way
// needs no preliminary ReadAll of its own: PrepareSend's own fresh read
// is the only read the whole test performs, keeping these tests to one
// ~140-exchange ReadAll instead of two. populated MUST match whichever
// factory image the session under test was opened with (see
// minimalFactoryPopulated/happyPathPopulated) — a mismatch manufactures
// spurious Added/Modified entries the test did not intend.
func matchingCandidateFile(caps spec.Capabilities, populated map[string]*codeplug.ChannelData, edits map[string]*codeplug.ChannelData) *codeplug.Codeplug {
	var channels []codeplug.Channel
	for _, bank := range caps.Banks {
		for _, slot := range bank.Slots {
			if data, ok := edits[slot]; ok {
				channels = append(channels, codeplug.Channel{Slot: slot, Data: copyChannelData(data)})
				continue
			}
			if data, ok := populated[slot]; ok {
				channels = append(channels, codeplug.Channel{Slot: slot, Data: copyChannelData(data)})
				continue
			}
			channels = append(channels, codeplug.Channel{Slot: slot})
		}
	}
	return &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: generatorID,
		Radio:     codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
		Channels:  channels,
	}
}

// countingPort wraps a Port and counts Write calls, so a test can assert
// that a phase of execution produced ZERO (or an exact number of) wire
// writes. Mirrors core/driver/ft710's test helper of the same name/shape.
//
// It also KEEPS the bytes (M9c-5 task 2): a count alone cannot answer
// "did anything at all reach the wire for THIS slot" when some other slot
// is legitimately being written in the same run, which is exactly what
// the blocked-send end-to-end test has to prove. See frames.
type countingPort struct {
	inner  io.ReadWriteCloser
	writes atomic.Int64
	mu     sync.Mutex
	sent   []byte
}

func (p *countingPort) Read(b []byte) (int, error) { return p.inner.Read(b) }
func (p *countingPort) Write(b []byte) (int, error) {
	p.writes.Add(1)
	p.mu.Lock()
	p.sent = append(p.sent, b...)
	p.mu.Unlock()
	return p.inner.Write(b)
}
func (p *countingPort) Close() error { return p.inner.Close() }

// mark returns the number of bytes written to the port so far, for
// framesSince below: a test takes a mark after PrepareSend, so what it
// then inspects is EXACTLY Execute's own wire traffic and not the
// preceding full-image read's.
func (p *countingPort) mark() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.sent)
}

// framesSince returns every complete ';'-terminated frame written after
// mark, in order. The transport writes whole commands, so splitting on
// ';' recovers exactly the frames this project put on the wire.
func (p *countingPort) framesSince(mark int) []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for _, f := range strings.Split(string(p.sent[mark:]), ";") {
		if f != "" {
			out = append(out, f+";")
		}
	}
	return out
}

// frameTargetsSlot reports whether frame is one of this project's
// slot-addressed commands (MW/MT/MR/MC — every frame either the write
// path or the read path emits for a specific channel) aimed at slot. All
// four carry the canonical 3-character slot immediately after the
// two-character mnemonic, so one test can ask the question for any of
// them without knowing which phase produced it.
func frameTargetsSlot(frame, slot string) bool {
	if len(frame) < 5 {
		return false
	}
	switch frame[:2] {
	case "MW", "MT", "MR", "MC":
		return frame[2:5] == slot
	}
	return false
}

// openCountingSimSession is openSimSession, but with the session's port
// wrapped in a countingPort so a test can inspect wire-write counts at any
// point (e.g. immediately after PrepareSend, then again after Execute).
func openCountingSimSession(t *testing.T, opts ...fakeradio.Option) (*fakeradio.Radio, *countingPort, driver.Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	cp := &countingPort{inner: r.Port()}

	sess, err := ft710.New(ft710.Simulated).Open(testCtx(t), cp, testIdentity)
	if err != nil {
		t.Fatalf("Open (Simulated, counting): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, cp, sess
}

// openCountingRealHardwareSession is openRealHardwareSession with the
// same countingPort wrapping as openCountingSimSession.
func openCountingRealHardwareSession(t *testing.T, opts ...fakeradio.Option) (*fakeradio.Radio, *countingPort, driver.Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	cp := &countingPort{inner: r.Port()}

	sess, err := ft710.New(ft710.RealHardware).Open(testCtx(t), cp, testIdentity)
	if err != nil {
		t.Fatalf("Open (RealHardware, counting): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, cp, sess
}

// openSimSession opens an ft710.Simulated session against a fresh
// fakeradio.Radio built with opts, registering cleanup for both. Simulated
// capabilities make MEM/PMS's codec-expressible fields write-Supported, so
// this is what every test exercising an actual delta write uses.
func openSimSession(t *testing.T, opts ...fakeradio.Option) (*fakeradio.Radio, driver.Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })

	sess, err := ft710.New(ft710.Simulated).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open (Simulated): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, sess
}

// consentedCapsSession wraps a real fake-backed session, overriding ONLY
// Capabilities() to relabel MEM's hardware-verified write fields as
// ConsentedUnverified. The underlying driver still enforces its own
// (Supported) labels, so writes flow — which is exactly the shape of a
// consented sibling session (Diff consults the session caps; the driver
// enforces the same set in production, and its acceptance here stands in
// for that). A deliberate, documented exception to this package's
// stubs-for-misbehaviour-only house rule (settings_test.go:90-95): no
// registered driver can yet produce ConsentedUnverified against a fake
// without the sibling drivers' full discovery walk, and this wrapper
// keeps ReadChannel/WriteChannel REAL.
//
// The relabelling is ROUTED THROUGH THE PRODUCTION TRANSFORM rather than
// writing ConsentedUnverified itself: this wrapper demotes MEM's
// write-Supported fields to Unverified and hands the result to
// spec.ConsentUnverifiedWrites, the one function a real consented
// session's labels also come from. So the fixture cannot mint a
// capability shape production can never produce — most pointedly a
// consented ERASE, which that transform exempts structurally
// (spec/consent.go) and which this fixture would otherwise have minted
// the day some profile made MEM FieldErase write-Supported, silently
// turning the pair test into a proof of something false.
//
// One consequence of overriding ONLY Capabilities(), noted so no test
// reads more into a journal than is there: the optional CONCRETE-type
// interfaces this package reaches for by assertion — MemorySelector
// (memory_selector.go), regioner (read.go), driver.SettingsReader
// (settings.go) — are not promoted through an embedded interface, so a
// Service over this wrapper skips the courtesy memory-selection
// snapshot/restore (no "mc_snapshot"/"mc_restore" lines) and reads a
// blank Region. Neither touches the write+verify pair itself, which is
// what the consented-labels test exists to prove.
type consentedCapsSession struct{ driver.Session }

// Capabilities implements driver.Session in two steps, neither of which
// names ConsentedUnverified itself.
//
// Step 1 demotes the underlying session's MEM fields whose Write is
// spec.Supported to spec.Unverified — the ONLY thing this fixture
// invents, and it invents an honest label rather than a consented one:
// "documented, unproven on hardware", which is precisely the state a
// sibling radio's real profile is in before its user consents. Read
// labels, the clarifier's Inert Write, and the PMS bank are passed
// through exactly as the driver minted them.
//
// Step 2 hands that to spec.ConsentUnverifiedWrites — the project's ONE
// consent transform, the same function a real consented session's labels
// come from — which turns every write-side Unverified into
// ConsentedUnverified, exempts FieldErase, and deep-copies. Routing
// through it means this fixture's capability shape cannot drift from
// what production can actually produce: it gets the erase exemption (and
// any future structural exclusion) for free, by construction, instead of
// by this wrapper remembering to reimplement it.
//
// The Banks slice and the MEM Fields map are freshly allocated in step 1
// too, so even the pre-transform value can never reach back into
// anything the driver holds (Capabilities() already hands out a
// defensive copy — this is belt and braces, exactly as spec.copyBank is).
func (s consentedCapsSession) Capabilities() spec.Capabilities {
	caps := s.Session.Capabilities()
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i, b := range banks {
		if b.ID != spec.BankMemory {
			continue
		}
		fields := make(map[spec.Field]spec.FieldSupport, len(b.Fields))
		for f, fs := range b.Fields {
			if fs.Write == spec.Supported {
				fs.Write = spec.Unverified
			}
			fields[f] = fs
		}
		b.Fields = fields
		banks[i] = b
	}
	caps.Banks = banks
	return spec.ConsentUnverifiedWrites(caps)
}

// openConsentedSimSession opens a Simulated fake-backed session exactly as
// openSimSession does, then wraps it in consentedCapsSession — the
// consented-labels session every test in this package uses. Both the
// wrapper AND the underlying real session are returned: a test asserting
// on the driver's own (untransformed) labels needs the latter.
func openConsentedSimSession(t *testing.T, opts ...fakeradio.Option) (*fakeradio.Radio, driver.Session, driver.Session) {
	t.Helper()
	r, sess := openSimSession(t, opts...)
	return r, consentedCapsSession{Session: sess}, sess
}

// tagPadWireInterceptor wraps a fakeradio port, rewriting exactly one
// wire reply (wantFrame, matched byte-for-byte) to padFrame before the
// bytes ever reach cat.Dialect.ParseMTAnswer. It exists SOLELY so
// TestExecute_LiveBugRepro_UnpaddedTagWriteReadBackPadded can reproduce,
// at the wire level, the HW-CONFIRMED live-radio behaviour this task
// fixes (docs/fixtures-private, 13/07/2026 production write: an MT-set
// tag written UNPADDED to the wire comes back on a later MT read
// space-padded to the full 12-byte field) — WITHOUT changing
// internal/fakeradio's own default wire simulation, which deliberately
// does not model this quirk (see that package's doc.go register item 8)
// and must keep behaving identically for every other test in the suite.
// Every byte that is not an exact match for wantFrame passes through
// completely unmodified, reassembled one ';'-terminated unit at a time
// (mirroring how fakeradio's own accumulator, and the transport engine
// reading it, already treat the wire — see FaultChunkedReplies for proof
// the engine tolerates arbitrary chunking of these same units).
type tagPadWireInterceptor struct {
	io.Writer
	io.Closer
	r          *bufio.Reader
	wantFrame  []byte
	padFrame   []byte
	pending    []byte
	pendingErr error
}

func newTagPadWireInterceptor(rw io.ReadWriteCloser, wantFrame, padFrame []byte) *tagPadWireInterceptor {
	return &tagPadWireInterceptor{
		Writer:    rw,
		Closer:    rw,
		r:         bufio.NewReader(rw),
		wantFrame: wantFrame,
		padFrame:  padFrame,
	}
}

// Read implements io.Reader, serving one (possibly rewritten) complete
// wire unit per underlying ReadBytes call, split across as many Read
// calls as the caller's buffer requires.
func (t *tagPadWireInterceptor) Read(p []byte) (int, error) {
	if len(t.pending) == 0 {
		frame, err := t.r.ReadBytes(';')
		if len(frame) == 0 {
			return 0, err
		}
		if bytes.Equal(frame, t.wantFrame) {
			frame = t.padFrame
		}
		t.pending = frame
		if err != nil {
			// A read error alongside a non-empty frame is surfaced on the
			// FOLLOWING call, once pending is drained — io.Reader callers
			// must still see the frame's bytes first.
			t.pendingErr = err
		}
	}
	n := copy(p, t.pending)
	t.pending = t.pending[n:]
	if len(t.pending) == 0 && t.pendingErr != nil {
		err := t.pendingErr
		t.pendingErr = nil
		return n, err
	}
	return n, nil
}

// openSimSessionWithWirePad is openSimSession, but every reply exactly
// matching wantFrame on the wire is rewritten to padFrame first — see
// tagPadWireInterceptor.
func openSimSessionWithWirePad(t *testing.T, wantFrame, padFrame []byte, opts ...fakeradio.Option) (*fakeradio.Radio, driver.Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })

	port := newTagPadWireInterceptor(r.Port(), wantFrame, padFrame)
	sess, err := ft710.New(ft710.Simulated).Open(testCtx(t), port, testIdentity)
	if err != nil {
		t.Fatalf("Open (Simulated, wire-intercepted): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, sess
}

// openRealHardwareSession opens an ft710.RealHardware session — the
// post-M5b-flip hardware-verified profile (writeTrialsComplete true,
// see core/driver/ft710/caps.go): the six verified fields are
// write-Supported, the clarifier Inert, tone/skip/erase Unsupported.
// (Before the flip this helper — then openUnverifiedSession — always
// yielded the all-Unverified profile where every diff entry Blocked.)
// Used to prove Execute's behaviour on the REAL profile specifically:
// blocked entries are still never written (see
// TestExecute_RealHardwareProfile_BlockedEntriesNeverWritten).
func openRealHardwareSession(t *testing.T, opts ...fakeradio.Option) (*fakeradio.Radio, driver.Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })

	sess, err := ft710.New(ft710.RealHardware).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open (RealHardware): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, sess
}

// failingJournal wraps a real journalAppender (normally a *Journal
// obtained from the same SnapshotStore/path Execute or PrepareSend would
// use) but fails Append for exactly one chosen event name — the seam
// Fix 2's journal-durability tests use to inject an append failure at a
// precise point in Execute's/PrepareSend's journal timeline, without
// needing a real unwritable filesystem path. Every OTHER event still
// appends through to inner untouched, so a test can assert the rest of
// the journal (e.g. "prepare", an "abort" line) still exists.
type failingJournal struct {
	inner  journalAppender
	failOn string
}

// Append implements journalAppender: fails with a plain (non-nil, never
// wrapping anything from the production Journal type) error for failOn,
// delegates to inner otherwise.
func (f *failingJournal) Append(now time.Time, event string, fields map[string]any) error {
	if event == f.failOn {
		return fmt.Errorf("injected failure for journal event %q", event)
	}
	return f.inner.Append(now, event, fields)
}

// Path implements journalAppender by delegating to inner.
func (f *failingJournal) Path() string { return f.inner.Path() }

// readJournalRecords reads path (a journal *.jsonl file) and returns
// every line's full decoded JSON record, in file order — for tests
// asserting on a field other than "event"/"slot" (e.g.
// firmware_confirmed's "version"), where readJournalEvents/
// readJournalEventDetails (execute_test.go) are not enough.
func readJournalRecords(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		rec := map[string]any{}
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("journal line does not parse as JSON: %v (%q)", err, line)
		}
		out = append(out, rec)
	}
	return out
}

// newStore returns a SnapshotStore rooted at a fresh t.TempDir().
func newStore(t *testing.T) SnapshotStore {
	t.Helper()
	return SnapshotStore{Dir: t.TempDir()}
}

// readDir returns the names of every entry directly inside dir.
func readDir(t *testing.T, dir string) ([]string, error) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

// hasSuffixOrp reports whether name is a snapshot file (".orp.json").
func hasSuffixOrp(name string) bool {
	return strings.HasSuffix(name, snapshotSuffix)
}

// stepClock returns an injectable Now that starts at start and advances by
// 1 second on every call — deterministic (no reliance on the wall clock)
// while still giving every timestamp-derived artefact (snapshot filenames,
// journal lines) a distinct value, exactly the shape of clock a real
// Service needs and a test can still assert exact values against.
func stepClock(start time.Time) func() time.Time {
	n := 0
	return func() time.Time {
		t := start.Add(time.Duration(n) * time.Second)
		n++
		return t
	}
}

// writableChannel returns a fully-writable populated channel for slot:
// every FieldState-carrying field Unknown (nothing inexpressible
// requested), everything else set to plain, codec-expressible values.
// Mirrors core/driver/ft710's write_test.go helper of the same name/shape.
// yaesuTierUnavailable sets every one of the ten fields the Icom tier
// added to Unavailable and returns d.
//
// It is what a READ of any radio registered today reports for all ten
// (core/driver/*/read.go), what a load of a pre-tier codeplug migrates
// to, and what a version-1 CSV import produces. A test fixture that left
// them at the zero value would differ from a real read in ten fields,
// and codeplug.Diff compares ChannelData with ==, so an otherwise
// identical channel would plan as MODIFIED.
func yaesuTierUnavailable(d *codeplug.ChannelData) *codeplug.ChannelData {
	d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.Duplex = codeplug.StringField{State: codeplug.Unavailable}
	d.OffsetHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.ToneMode = codeplug.StringField{State: codeplug.Unavailable}
	d.ToneTx = codeplug.ToneField{State: codeplug.Unavailable}
	d.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
	d.DTCSCode = codeplug.IntField{State: codeplug.Unavailable}
	d.DTCSPolarity = codeplug.StringField{State: codeplug.Unavailable}
	d.Filter = codeplug.StringField{State: codeplug.Unavailable}
	d.DataMode = codeplug.BoolField{State: codeplug.Unavailable}
	return d
}

func writableChannel(slot string, freqHz uint64, tag string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: yaesuTierUnavailable(&codeplug.ChannelData{
			FreqHz:     freqHz,
			Mode:       "USB",
			CTCSS:      "OFF",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      "SIMPLEX",
			Tag:        tag,
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: tag != ""},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		}),
	}
}

// withChannel returns a deep copy of cp with slot's Channel replaced by
// {Slot: slot, Data: data} (data may be nil, for an erase/empty edit) — a
// test-only Codeplug editor, used to build a candidate file from a
// baseline read without ever mutating the baseline itself.
func withChannel(cp *codeplug.Codeplug, slot string, data *codeplug.ChannelData) *codeplug.Codeplug {
	out := &codeplug.Codeplug{
		Schema:    cp.Schema,
		Generator: cp.Generator,
		Radio:     cp.Radio,
		Channels:  copyChannels(cp.Channels),
	}
	found := false
	for i, ch := range out.Channels {
		if ch.Slot == slot {
			out.Channels[i] = codeplug.Channel{Slot: slot, Data: copyChannelData(data)}
			found = true
			break
		}
	}
	if !found {
		out.Channels = append(out.Channels, codeplug.Channel{Slot: slot, Data: copyChannelData(data)})
	}
	return out
}

// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// settingsProgressCall records one ReadSettings Progress invocation, for
// tests that assert on the exact sequence.
type settingsProgressCall struct {
	phase string
	done  int
	total int
	id    string
}

// settingsDescriptorItemIDs returns every item ID in d, in the exact
// menus->groups->items order ReadSettings walks it — for tests asserting
// on Entries/progress order without duplicating the walk themselves.
func settingsDescriptorItemIDs(d driver.SettingsDescriptor) []string {
	var ids []string
	for _, m := range d.Menus {
		for _, g := range m.Groups {
			for _, it := range g.Items {
				ids = append(ids, it.ID)
			}
		}
	}
	return ids
}

// exCountingPort wraps a Port and counts outbound writes whose command
// frame begins with "EX" — the settings-read wire prefix (exSpec,
// core/driver/ft710/settings.go) — so a test can assert a phase of
// execution sent ZERO EX exchanges (the channel path's decoupling from
// the settings-read path). transport.Engine.Do sends one full command
// frame per Write call (never a partial frame split across two Writes),
// so inspecting each Write's own payload is sufficient — no reassembly
// needed. Mirrors countingPort's shape (helpers_test.go).
type exCountingPort struct {
	inner    io.ReadWriteCloser
	exWrites atomic.Int64
}

func (p *exCountingPort) Read(b []byte) (int, error) { return p.inner.Read(b) }
func (p *exCountingPort) Write(b []byte) (int, error) {
	if bytes.HasPrefix(b, []byte("EX")) {
		p.exWrites.Add(1)
	}
	return p.inner.Write(b)
}
func (p *exCountingPort) Close() error { return p.inner.Close() }

// openExCountingSimSession is openSimSession, but with the session's port
// wrapped in an exCountingPort so a test can inspect EX-specific wire
// traffic at any point.
func openExCountingSimSession(t *testing.T, opts ...fakeradio.Option) (*fakeradio.Radio, *exCountingPort, driver.Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })
	ep := &exCountingPort{inner: r.Port()}

	sess, err := ft710.New(ft710.Simulated).Open(testCtx(t), ep, testIdentity)
	if err != nil {
		t.Fatalf("Open (Simulated, EX-counting): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, ep, sess
}

// stubSession is a minimal driver.Session double. Per the task brief's
// carve-out, a stub is used ONLY for the two properties the REAL ft710
// driver cannot be made to exhibit: a session with no settings surface at
// all (TestReadSettings_SessionWithoutReader), and driver-misbehaviour
// injections a real driver's own type/wire discipline can never produce —
// a malformed descriptor, or a ReadSetting answering with the wrong ID
// (TestReadSettings_MalformedDescriptorRefusedBeforeWire,
// TestReadSettings_WrongIDFromDriverAborts). Every OTHER ReadSettings
// test in this file exercises the real *ft710.Session against fakeradio,
// per this package's house rule (core/clone/memory_selector.go's
// MemorySelector doc comment).
type stubSession struct {
	caps spec.Capabilities
}

func (s *stubSession) Identity() driver.Identity       { return driver.Identity{} }
func (s *stubSession) Capabilities() spec.Capabilities { return s.caps }
func (s *stubSession) ReadChannel(context.Context, string) (codeplug.Channel, error) {
	return codeplug.Channel{}, errors.New("stubSession: ReadChannel not implemented")
}
func (s *stubSession) WriteChannel(context.Context, codeplug.Channel) (driver.WriteResult, error) {
	return driver.WriteResult{}, errors.New("stubSession: WriteChannel not implemented")
}
func (s *stubSession) Close() error { return nil }

// stubSettingsSession adds driver.SettingsReader to stubSession, with a
// caller-supplied descriptor and ReadSetting behaviour — see stubSession's
// doc comment for why this exists only for the two driver-misbehaviour
// properties a real driver cannot produce.
type stubSettingsSession struct {
	stubSession
	descriptor  driver.SettingsDescriptor
	readSetting func(ctx context.Context, id string) (driver.SettingValue, error)
}

func (s *stubSettingsSession) SettingsDescriptor() driver.SettingsDescriptor { return s.descriptor }
func (s *stubSettingsSession) ReadSetting(ctx context.Context, id string) (driver.SettingValue, error) {
	return s.readSetting(ctx, id)
}

// TestReadSettings_FullCycle_Sim: a full ReadSettings against fakeradio's
// default EX state (every address answers Known — EXDefaults, seeded in
// New) assembles a MenuSnapshot with every descriptor item present, in
// descriptor order, all Known, Complete true, Descriptor ==
// "ft710-ex@1", a WithEXSetting-seeded value spot-checked verbatim, and
// the whole snapshot passing its own Validate.
func TestReadSettings_FullCycle_Sim(t *testing.T) {
	const spotAddr = "010101" // AF TREBLE GAIN
	_, sess := openSimSession(t, fakeradio.WithEXSetting(spotAddr, "042"))

	svc := NewService(sess, newStore(t))

	snap, err := svc.ReadSettings(testCtx(t))
	if err != nil {
		t.Fatalf("ReadSettings: unexpected error: %v", err)
	}
	if snap == nil {
		t.Fatal("snapshot is nil, want a populated *MenuSnapshot")
	}

	if snap.Descriptor != "ft710-ex@1" {
		t.Errorf("Descriptor = %q, want \"ft710-ex@1\"", snap.Descriptor)
	}
	if !snap.Complete {
		t.Error("Complete = false, want true (every descriptor item answered Known)")
	}

	reader := sess.(driver.SettingsReader)
	wantIDs := settingsDescriptorItemIDs(reader.SettingsDescriptor())
	if wantIDs[0] != "010101" || len(wantIDs) != 296 {
		t.Fatalf("test's own assumption about the FT-710 descriptor is stale: first=%q count=%d, want first=\"010101\" count=296", wantIDs[0], len(wantIDs))
	}

	if len(snap.Entries) != len(wantIDs) {
		t.Fatalf("len(Entries) = %d, want %d (one per descriptor item)", len(snap.Entries), len(wantIDs))
	}
	var spot *codeplug.MenuEntry
	for i, id := range wantIDs {
		e := snap.Entries[i]
		if e.ID != id {
			t.Fatalf("Entries[%d].ID = %q, want %q (descriptor order)", i, e.ID, id)
		}
		if e.State != codeplug.MenuKnown {
			t.Errorf("Entries[%d] (%q).State = %q, want %q", i, e.ID, e.State, codeplug.MenuKnown)
		}
		if e.Value == "" {
			t.Errorf("Entries[%d] (%q).Value is empty, want a non-empty Known value", i, e.ID)
		}
		if id == spotAddr {
			spot = &snap.Entries[i]
		}
	}
	if spot == nil {
		t.Fatalf("no entry found for spot-checked id %q", spotAddr)
	}
	if spot.Value != "042" {
		t.Errorf("Entries[%q].Value = %q, want %q (the WithEXSetting-seeded value)", spotAddr, spot.Value, "042")
	}

	if err := snap.Validate(); err != nil {
		t.Errorf("snapshot.Validate() = %v, want nil", err)
	}
}

// TestReadSettings_RejectedItem_PartialSnapshot: one address forced
// unavailable (fakeradio.WithEXUnavailable, Task 32's fakeradio seam)
// comes back MenuUnavailable with an empty Value; every other item is
// still Known; Complete is false; ReadSettings itself returns no error —
// a partial snapshot is data, not a failure.
func TestReadSettings_RejectedItem_PartialSnapshot(t *testing.T) {
	const rejectedAddr = "010101"
	_, sess := openSimSession(t, fakeradio.WithEXUnavailable(rejectedAddr))

	svc := NewService(sess, newStore(t))
	snap, err := svc.ReadSettings(testCtx(t))
	if err != nil {
		t.Fatalf("ReadSettings: unexpected error: %v, want nil (a rejected item is a partial snapshot, not a failure)", err)
	}
	if snap == nil {
		t.Fatal("snapshot is nil, want a populated (partial) *MenuSnapshot")
	}
	if snap.Complete {
		t.Error("Complete = true, want false (one item is unavailable)")
	}

	var got *codeplug.MenuEntry
	unavailableCount := 0
	for i := range snap.Entries {
		e := &snap.Entries[i]
		switch e.State {
		case codeplug.MenuUnavailable:
			unavailableCount++
			got = e
		case codeplug.MenuKnown:
			// expected for every other item
		default:
			t.Errorf("Entries[%d] (%q).State = %q, want %q or %q", i, e.ID, e.State, codeplug.MenuKnown, codeplug.MenuUnavailable)
		}
	}
	if unavailableCount != 1 {
		t.Fatalf("unavailable entries = %d, want exactly 1", unavailableCount)
	}
	if got.ID != rejectedAddr {
		t.Errorf("unavailable entry ID = %q, want %q", got.ID, rejectedAddr)
	}
	if got.Value != "" {
		t.Errorf("unavailable entry Value = %q, want empty", got.Value)
	}

	if err := snap.Validate(); err != nil {
		t.Errorf("snapshot.Validate() = %v, want nil", err)
	}
}

// TestReadSettings_HardErrorAborts: a fault-injected transport failure
// (FaultDropReplies) mid-run makes one ReadSetting exchange fail outright
// (as distinct from a "?;" rejection, which is not an error at all — see
// TestReadSettings_RejectedItem_PartialSnapshot). ReadSettings must abort
// with an error and return a nil snapshot — failures are failures, never
// papered over the way a rejection is.
//
// Exchange arithmetic (mirrors preparedThreeDeltaPlan's documented
// pattern, execute_test.go): Open against fakeradio's default image (no
// 60m/no EMG, exactly minimalFactoryImage/happyPathImage's own region) =
// AI0;(1) + ID;(2) + MR501;(3, rejected: no 60m) + MREMG;(4, rejected: no
// EMG) = 4 exchanges. ReadSettings' own first EX read is therefore
// exchange 5; each subsequent item is exactly one further exchange (a
// single transport.Engine.Do call per item — see exSpec's doc comment,
// core/driver/ft710/settings.go). FaultDropReplies(7) fails the THIRD
// item's read (items 1-2, exchanges 5-6, succeed first) — proving the
// abort happens mid-run, not merely "on the very first item".
func TestReadSettings_HardErrorAborts(t *testing.T) {
	const failExchange = 7
	_, sess := openSimSession(t, fakeradio.WithFault(fakeradio.FaultDropReplies(failExchange)))

	svc := NewService(sess, newStore(t))
	snap, err := svc.ReadSettings(testCtx(t))
	if err == nil {
		t.Fatal("ReadSettings = nil error, want an error (transport failure mid-run)")
	}
	if snap != nil {
		t.Errorf("snapshot = %+v, want nil on error", snap)
	}
}

// TestReadSettings_CancelledBetweenItems: cancelling the caller's ctx
// from within the Progress callback, immediately after the FIRST item's
// read completes, must be observed BEFORE the second item's ReadSetting
// call — the wrapped context error is returned, and no snapshot at all
// (partial or otherwise) comes back.
func TestReadSettings_CancelledBetweenItems(t *testing.T) {
	_, sess := openSimSession(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var progressCalls int
	svc := NewService(sess, newStore(t), WithProgress(func(phase string, done, total int, id string) {
		progressCalls++
		if phase == "read-settings" && done == 1 {
			cancel()
		}
	}))

	snap, err := svc.ReadSettings(ctx)
	if err == nil {
		t.Fatal("ReadSettings = nil error, want a wrapped context error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
	}
	if snap != nil {
		t.Errorf("snapshot = %+v, want nil (no snapshot at all, partial or otherwise, on cancellation)", snap)
	}
	if progressCalls != 1 {
		t.Errorf("progress called %d times, want exactly 1 (only the first item completed before cancellation was observed, between items)", progressCalls)
	}
}

// TestReadSettings_MalformedDescriptorRefusedBeforeWire: a stub
// SettingsReader whose descriptor fails Validate() (a duplicate item ID)
// is refused with an error and ZERO ReadSetting calls — descriptor
// validation happens entirely before any wire traffic, so a driver bug in
// the descriptor itself can never even start a read.
func TestReadSettings_MalformedDescriptorRefusedBeforeWire(t *testing.T) {
	var calls int
	sess := &stubSettingsSession{
		stubSession: stubSession{caps: spec.Capabilities{Model: "STUB-1"}},
		descriptor: driver.SettingsDescriptor{
			Version: "stub@1",
			Menus: []driver.SettingMenu{{
				ID: "01", Label: "M1",
				Groups: []driver.SettingGroup{{
					ID: "0101", Label: "G1",
					Items: []driver.SettingItem{
						{ID: "010101", Label: "A"},
						{ID: "010101", Label: "B (duplicate ID)"},
					},
				}},
			}},
		},
		readSetting: func(ctx context.Context, id string) (driver.SettingValue, error) {
			calls++
			return driver.SettingValue{ID: id, State: driver.SettingKnown, Raw: "x"}, nil
		},
	}
	svc := NewService(sess, newStore(t))

	snap, err := svc.ReadSettings(testCtx(t))
	if err == nil {
		t.Fatal("ReadSettings = nil error, want an error (malformed descriptor)")
	}
	if snap != nil {
		t.Errorf("snapshot = %+v, want nil", snap)
	}
	if calls != 0 {
		t.Errorf("ReadSetting called %d times, want 0 (descriptor must be validated before any wire traffic)", calls)
	}
}

// TestReadSettings_BadItemIDShapeRefusedBeforeWire (Codex M8b #5): a stub
// SettingsReader whose descriptor is structurally valid (non-empty, unique
// IDs, so SettingsDescriptor.Validate passes) but whose item ID is NOT the
// 6-ASCII-digit shape a MenuSnapshot requires is refused BEFORE any read.
// Without the item-ID preflight this failed only AFTER every read, when the
// built snapshot was validated.
func TestReadSettings_BadItemIDShapeRefusedBeforeWire(t *testing.T) {
	var calls int
	sess := &stubSettingsSession{
		stubSession: stubSession{caps: spec.Capabilities{Model: "STUB-1"}},
		descriptor: driver.SettingsDescriptor{
			Version: "stub@1",
			Menus: []driver.SettingMenu{{
				ID: "01", Label: "M1",
				Groups: []driver.SettingGroup{{
					ID: "0101", Label: "G1",
					Items: []driver.SettingItem{
						{ID: "01011", Label: "A (5 chars, not a valid snapshot ID)"},
					},
				}},
			}},
		},
		readSetting: func(ctx context.Context, id string) (driver.SettingValue, error) {
			calls++
			return driver.SettingValue{ID: id, State: driver.SettingKnown, Raw: "x"}, nil
		},
	}
	svc := NewService(sess, newStore(t))

	snap, err := svc.ReadSettings(testCtx(t))
	if err == nil {
		t.Fatal("ReadSettings = nil error, want an error (item ID not a valid snapshot ID)")
	}
	if snap != nil {
		t.Errorf("snapshot = %+v, want nil", snap)
	}
	if calls != 0 {
		t.Errorf("ReadSetting called %d times, want 0 (item-ID shape must be validated before any wire traffic)", calls)
	}
	var me *codeplug.MenuEntryError
	if !errors.As(err, &me) {
		t.Errorf("errors.As(err, *codeplug.MenuEntryError) = false (err = %v)", err)
	}
}

// TestReadSettings_WrongIDFromDriverAborts: a stub SettingsReader whose
// ReadSetting answers with a SettingValue.ID that does NOT match the id
// requested is a driver bug ReadSettings refuses to paper over — it
// aborts with an error naming BOTH the requested and the wrongly-returned
// ID, rather than recording the (wrong) answer against the item that was
// actually asked for.
func TestReadSettings_WrongIDFromDriverAborts(t *testing.T) {
	const requested = "010101"
	const wrongAnswer = "999999"
	sess := &stubSettingsSession{
		stubSession: stubSession{caps: spec.Capabilities{Model: "STUB-1"}},
		descriptor: driver.SettingsDescriptor{
			Version: "stub@1",
			Menus: []driver.SettingMenu{{
				ID: "01", Label: "M1",
				Groups: []driver.SettingGroup{{
					ID: "0101", Label: "G1",
					Items: []driver.SettingItem{{ID: requested, Label: "A"}},
				}},
			}},
		},
		readSetting: func(ctx context.Context, id string) (driver.SettingValue, error) {
			return driver.SettingValue{ID: wrongAnswer, State: driver.SettingKnown, Raw: "x"}, nil
		},
	}
	svc := NewService(sess, newStore(t))

	snap, err := svc.ReadSettings(testCtx(t))
	if err == nil {
		t.Fatal("ReadSettings = nil error, want an error (driver returned a mismatched ID)")
	}
	if snap != nil {
		t.Errorf("snapshot = %+v, want nil", snap)
	}
	if !strings.Contains(err.Error(), requested) || !strings.Contains(err.Error(), wrongAnswer) {
		t.Errorf("error %q does not name both the requested (%q) and returned (%q) IDs", err.Error(), requested, wrongAnswer)
	}
}

// TestReadSettings_BusyExclusion: ReadSettings and ReadAll, run
// concurrently against the SAME Service, must never interleave their wire
// traffic — whichever call arrives while the other is already running is
// refused outright with a typed *BusyError naming the holder, in BOTH
// directions. Mirrors TestExecute_ConcurrentCalls_OneBusy's shape
// (execute_test.go).
func TestReadSettings_BusyExclusion(t *testing.T) {
	t.Run("ReadSettings holds the lock, ReadAll is refused", func(t *testing.T) {
		_, sess := openSimSession(t)
		svc := NewService(sess, newStore(t))

		started := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			close(started)
			_, err := svc.ReadSettings(testCtx(t))
			done <- err
		}()

		<-started
		time.Sleep(5 * time.Millisecond)

		_, err2 := svc.ReadAll(testCtx(t))
		err1 := <-done

		if err1 != nil {
			t.Fatalf("ReadSettings (the lock holder): unexpected error: %v", err1)
		}
		var be *BusyError
		if !errors.As(err2, &be) {
			t.Fatalf("ReadAll error = %v, want a *BusyError", err2)
		}
		if be.InProgress != "ReadSettings" {
			t.Errorf("BusyError.InProgress = %q, want \"ReadSettings\"", be.InProgress)
		}
	})

	t.Run("ReadAll holds the lock, ReadSettings is refused", func(t *testing.T) {
		_, sess := openSimSession(t)
		svc := NewService(sess, newStore(t))

		started := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			close(started)
			_, err := svc.ReadAll(testCtx(t))
			done <- err
		}()

		<-started
		time.Sleep(5 * time.Millisecond)

		_, err2 := svc.ReadSettings(testCtx(t))
		err1 := <-done

		if err1 != nil {
			t.Fatalf("ReadAll (the lock holder): unexpected error: %v", err1)
		}
		var be *BusyError
		if !errors.As(err2, &be) {
			t.Fatalf("ReadSettings error = %v, want a *BusyError", err2)
		}
		if be.InProgress != "ReadAll" {
			t.Errorf("BusyError.InProgress = %q, want \"ReadAll\"", be.InProgress)
		}
	})
}

// TestReadSettings_SessionWithoutReader: a session whose concrete type
// does not implement driver.SettingsReader at all is refused with
// errors.Is(err, ErrSettingsUnsupported), naming the model, and returns a
// nil snapshot.
func TestReadSettings_SessionWithoutReader(t *testing.T) {
	sess := &stubSession{caps: spec.Capabilities{Model: "STUB-1"}}
	svc := NewService(sess, newStore(t))

	snap, err := svc.ReadSettings(testCtx(t))
	if !errors.Is(err, ErrSettingsUnsupported) {
		t.Fatalf("errors.Is(err, ErrSettingsUnsupported) = false; err = %v", err)
	}
	var unsupported *SettingsUnsupportedError
	if !errors.As(err, &unsupported) {
		t.Fatalf("error = %v (%T), want *SettingsUnsupportedError", err, err)
	}
	if unsupported.Model != "STUB-1" {
		t.Errorf("SettingsUnsupportedError.Model = %q, want %q", unsupported.Model, "STUB-1")
	}
	if snap != nil {
		t.Errorf("snapshot = %+v, want nil", snap)
	}
}

// TestReadSettings_ProgressContract: the recorded progress sequence pins
// phase "read-settings", 1-based/monotonic done against a fixed total
// equal to the descriptor's own item count, and the setting ID (never
// empty) as the name parameter — for every one of the 296 FT-710 EX
// items, in descriptor order.
func TestReadSettings_ProgressContract(t *testing.T) {
	_, sess := openSimSession(t)

	var calls []settingsProgressCall
	svc := NewService(sess, newStore(t), WithProgress(func(phase string, done, total int, id string) {
		calls = append(calls, settingsProgressCall{phase, done, total, id})
	}))

	snap, err := svc.ReadSettings(testCtx(t))
	if err != nil {
		t.Fatalf("ReadSettings: unexpected error: %v", err)
	}

	reader := sess.(driver.SettingsReader)
	wantIDs := settingsDescriptorItemIDs(reader.SettingsDescriptor())

	if len(calls) != len(wantIDs) {
		t.Fatalf("progress called %d times, want %d (once per descriptor item)", len(calls), len(wantIDs))
	}
	for i, c := range calls {
		if c.phase != "read-settings" {
			t.Errorf("progress[%d].phase = %q, want \"read-settings\"", i, c.phase)
		}
		if c.done != i+1 {
			t.Errorf("progress[%d].done = %d, want %d (1-based, monotonic)", i, c.done, i+1)
		}
		if c.total != len(wantIDs) {
			t.Errorf("progress[%d].total = %d, want %d", i, c.total, len(wantIDs))
		}
		if c.id != wantIDs[i] {
			t.Errorf("progress[%d].id = %q, want %q (descriptor order)", i, c.id, wantIDs[i])
		}
	}
	if len(snap.Entries) != len(wantIDs) {
		t.Errorf("len(snap.Entries) = %d, want %d", len(snap.Entries), len(wantIDs))
	}
}

// TestReadAll_NoSettingsTrafficAndNilMenus: the decoupling pin for the
// READ side. ReadAll must never touch the settings surface at all — zero
// EX exchanges on the wire — and the Codeplug it returns carries a nil
// Menus (ReadAll has never set that field; this pins the fact down so a
// future change cannot silently start doing so without a test noticing).
func TestReadAll_NoSettingsTrafficAndNilMenus(t *testing.T) {
	_, ep, sess := openExCountingSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t))

	cp, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("ReadAll: unexpected error: %v", err)
	}
	if cp.Menus != nil {
		t.Errorf("Menus = %+v, want nil — ReadAll must never touch the settings path", cp.Menus)
	}
	if got := ep.exWrites.Load(); got != 0 {
		t.Errorf("EX writes during ReadAll = %d, want 0", got)
	}
}

// TestPrepareSend_PerformsNoSettingsTraffic: the decoupling pin for the
// SEND side. A candidate carrying a NON-NIL MenuSnapshot must still make
// PrepareSend perform ZERO settings-read progress and ZERO EX wire
// exchanges, and the resulting plan (BaselineDigest, ConfirmationDigest,
// and Diff summary) must be IDENTICAL to the very same candidate with a
// nil Menus — proving file.Menus is completely inert to PrepareSend.
// codeplug.Digest/Diff both operate on Channels alone (never Menus), so
// this identity is expected; the point of the test is pinning down that
// PrepareSend's CODE never branches on file.Menus being present to
// trigger a settings read, which no digest equality alone could prove.
//
// Both PrepareSend calls run against the SAME Service/session (so
// identity and generation — two of ConfirmationDigest's own inputs,
// plan.go — are identical by construction) and against an UNCHANGED
// radio (PrepareSend never writes), so both calls' fresh baseline reads,
// and therefore BaselineDigest, are expected to agree regardless of this
// test's own point.
func TestPrepareSend_PerformsNoSettingsTraffic(t *testing.T) {
	_, ep, sess := openExCountingSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))

	var settingsProgressCalls int
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)), WithProgress(func(phase string, done, total int, id string) {
		if phase == "read-settings" {
			settingsProgressCalls++
		}
	}))

	caps := sess.Capabilities()
	fileNilMenus := matchingCandidateFile(caps, minimalFactoryPopulated(), nil)
	fileWithMenus := matchingCandidateFile(caps, minimalFactoryPopulated(), nil)
	fileWithMenus.Menus = &codeplug.MenuSnapshot{
		Descriptor: "ft710-ex@1",
		Complete:   true,
		Entries:    []codeplug.MenuEntry{{ID: "010101", Value: "042", State: codeplug.MenuKnown}},
	}

	planA, err := svc.PrepareSend(testCtx(t), fileNilMenus)
	if err != nil {
		t.Fatalf("PrepareSend (nil Menus): unexpected error: %v", err)
	}
	exAfterA := ep.exWrites.Load()

	planB, err := svc.PrepareSend(testCtx(t), fileWithMenus)
	if err != nil {
		t.Fatalf("PrepareSend (non-nil Menus): unexpected error: %v", err)
	}
	exAfterB := ep.exWrites.Load()

	if settingsProgressCalls != 0 {
		t.Errorf("read-settings progress calls = %d, want 0 (PrepareSend must never touch the settings path)", settingsProgressCalls)
	}
	if exAfterA != 0 {
		t.Errorf("EX writes after PrepareSend(nil Menus) = %d, want 0", exAfterA)
	}
	if exAfterB != exAfterA {
		t.Errorf("EX writes after PrepareSend(non-nil Menus) = %d, want unchanged at %d", exAfterB, exAfterA)
	}

	if planA.ConfirmationDigest() != planB.ConfirmationDigest() {
		t.Errorf("ConfirmationDigest differs: nil-Menus=%q non-nil-Menus=%q, want identical", planA.ConfirmationDigest(), planB.ConfirmationDigest())
	}
	if planA.BaselineDigest() != planB.BaselineDigest() {
		t.Errorf("BaselineDigest differs: nil-Menus=%q non-nil-Menus=%q, want identical", planA.BaselineDigest(), planB.BaselineDigest())
	}
	diffA, diffB := planA.Diff(), planB.Diff()
	if diffA.Added != diffB.Added || diffA.Modified != diffB.Modified || diffA.Erased != diffB.Erased ||
		diffA.Unchanged != diffB.Unchanged || diffA.Blocked != diffB.Blocked || len(diffA.Entries) != len(diffB.Entries) {
		t.Errorf("Diff summary differs between nil-Menus and non-nil-Menus candidates:\n  nil-Menus:     %+v\n  non-nil-Menus: %+v", diffA, diffB)
	}
}

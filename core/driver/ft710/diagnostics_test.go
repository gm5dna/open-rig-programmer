// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// recordingLogger implements transport.Logger by appending every formatted
// message, mutex-guarded (the engine's reader goroutine may log
// concurrently with test assertions). Mirrors core/transport's own test
// helper of the same name/shape.
type recordingLogger struct {
	mu      sync.Mutex
	records []string
}

func (l *recordingLogger) Printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, fmt.Sprintf(format, args...))
}

func (l *recordingLogger) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.records...)
}

// TestWithTransportLogger_ReceivesEngineLines (M3 Codex-review fix wave,
// Fix 5, adjudicated MEDIUM): before this fix the driver built its
// transport.Engine with no logger at all, so the engine's diagnostics —
// unexpected frames, quarantine drains, contamination — vanished into the
// engine's nopLogger default with no way for ANY production caller to see
// them (transport safety obligation 3 says "surfaced, never silently
// discarded", but the driver's composition silently discarded them one
// layer up). WithTransportLogger threads a transport.Logger through
// Open's NewEngine call; a spurious frame injected mid-exchange must
// reach it.
func TestWithTransportLogger_ReceivesEngineLines(t *testing.T) {
	spurious := []byte("FA00007000000;")
	logger := &recordingLogger{}

	r := fakeradio.New(
		fakeradio.WithFactoryImage(minimalImage),
		// Exchange 1 is Open's own AI0; — the spurious frame arrives
		// before it, during a live exchange, exactly the "unexpected
		// frame" case obligation 3 is about.
		fakeradio.WithFault(fakeradio.FaultSpuriousFrame(spurious, 1)),
	)
	t.Cleanup(func() { _ = r.Close() })

	sess, err := New(Simulated, WithTransportLogger(logger)).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	found := false
	for _, rec := range logger.snapshot() {
		if strings.Contains(rec, "FA00007000000") {
			found = true
		}
	}
	if !found {
		t.Errorf("transport logger records = %v, want one mentioning the spurious frame %q — WithTransportLogger must thread through to the engine", logger.snapshot(), spurious)
	}
}

// TestSession_Diagnostics_CountsSpuriousFrame (M3 Codex-review fix wave,
// Fix 5): Session.Diagnostics() surfaces the engine's unexpected-frames
// counter (transport safety obligation 3's counter) to driver callers,
// who otherwise have no path to the *transport.Engine buried inside the
// session. A spurious frame injected during Open's ID probe must show up
// in UnexpectedFrames.
func TestSession_Diagnostics_CountsSpuriousFrame(t *testing.T) {
	spurious := []byte("FA00007000000;")

	r := fakeradio.New(
		fakeradio.WithFactoryImage(minimalImage),
		// Exchange 2 is Open's ID; probe — a read exchange, so the
		// spurious frame is received (and counted) while Do is actively
		// waiting for the ID answer.
		fakeradio.WithFault(fakeradio.FaultSpuriousFrame(spurious, 2)),
	)
	t.Cleanup(func() { _ = r.Close() })

	sess, err := New(Simulated).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	fs := sess.(*Session)
	diag := fs.Diagnostics()
	if diag.UnexpectedFrames != 1 {
		t.Errorf("Diagnostics().UnexpectedFrames = %d, want 1 (the injected spurious frame)", diag.UnexpectedFrames)
	}
}

// TestSession_Diagnostics_ZeroOnCleanSession: a fault-free session
// reports zero unexpected frames — the counter must not pick up noise
// from a normal Open+read sequence.
func TestSession_Diagnostics_ZeroOnCleanSession(t *testing.T) {
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

	if _, err := sess.ReadChannel(testCtx(t), "001"); err != nil {
		t.Fatalf("ReadChannel: unexpected error: %v", err)
	}
	if diag := sess.Diagnostics(); diag.UnexpectedFrames != 0 {
		t.Errorf("Diagnostics().UnexpectedFrames = %d, want 0 for a clean session", diag.UnexpectedFrames)
	}
}

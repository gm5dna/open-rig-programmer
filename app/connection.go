// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// ListPorts lists candidate serial ports (transport.Discover), mapped to
// the display shape a port picker needs.
func (a *App) ListPorts() ([]PortEntry, error) {
	infos, err := transport.Discover()
	if err != nil {
		return nil, fmt.Errorf("app: listing ports: %w", err)
	}
	out := make([]PortEntry, len(infos))
	for i, in := range infos {
		out[i] = PortEntry{Path: in.Path, Description: in.Description, Score: in.Score, Hints: in.Hints}
	}
	return out, nil
}

// GetSupportedModels returns every radio model name this build can open a
// real session against (internal/wiring.SupportedModels' own sorted
// output) — registry-driven, so a future second driver appears here with
// no change to this method. app/ has no model-selection surface yet
// (task 41, M9a-5: every Connect/ConnectDemo call still keys off
// wiring.DefaultModel) — this method exists so the frontend can start
// rendering the list before that surface lands.
func (a *App) GetSupportedModels() []string {
	return wiring.SupportedModels()
}

// Connect opens a session against a real FT-710 on portPath.
func (a *App) Connect(portPath string) (ConnectionInfo, error) {
	return a.connect(false, portPath)
}

// ConnectDemo opens a session against the in-process simulated radio
// (internal/fakeradio, via internal/wiring.OpenFakeSessionFor) — never
// real hardware.
func (a *App) ConnectDemo() (ConnectionInfo, error) {
	return a.connect(true, "")
}

// connect is Connect/ConnectDemo's shared body. It deliberately calls
// wiring.OpenRealSessionFor/wiring.OpenFakeSessionFor — internal/wiring's
// own two self-contained, model-keyed constructors — rather than
// re-deriving the profile/session pairing here, preserving the
// structural-exclusivity shape those constructors exist to enforce.
// Both are called at wiring.DefaultModel: app/ has no model-selection
// surface yet (task 41, M9a-5).
func (a *App) connect(demo bool, portPath string) (ConnectionInfo, error) {
	a.mu.Lock()
	if a.conn != nil {
		a.mu.Unlock()
		return ConnectionInfo{}, ErrAlreadyConnected
	}
	if a.transfer.running {
		a.mu.Unlock()
		return ConnectionInfo{}, ErrTransferRunning
	}
	a.mu.Unlock()

	snapshotDir, err := wiring.ResolveSnapshotDir("", wiring.DefaultModel)
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("app: resolving snapshot directory: %w", err)
	}
	if err := os.MkdirAll(snapshotDir, 0o700); err != nil {
		return ConnectionInfo{}, fmt.Errorf("app: creating snapshot directory %s: %w", snapshotDir, err)
	}

	var (
		sess   driver.Session
		closer func() error
	)
	if demo {
		sess, closer, err = wiring.OpenFakeSessionFor(a.ctx, wiring.DefaultModel)
	} else {
		sess, closer, err = wiring.OpenRealSessionFor(a.ctx, wiring.DefaultModel, portPath)
	}
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("app: connecting: %w", err)
	}

	svc := clone.NewService(sess, clone.SnapshotStore{Dir: snapshotDir}, clone.WithProgress(a.progressCallback()))

	a.mu.Lock()
	if a.conn != nil {
		// Lost a race against a concurrent Connect/ConnectDemo call.
		a.mu.Unlock()
		_ = closer()
		return ConnectionInfo{}, ErrAlreadyConnected
	}
	a.conn = &connectionState{session: sess, closer: closer, svc: svc, demo: demo}
	a.mu.Unlock()

	id := sess.Identity()
	// Region is an OPTIONAL capability (core/driver/optional.go's
	// RegionReporter) a session's concrete type may implement — never part
	// of driver.Session itself, since region derivation from a discovered
	// 60 m/EMG inventory is an FT-710 discovery quirk, not a seam-level
	// contract. Absence (a future driver with no such concept) reports "".
	region := ""
	if rr, ok := sess.(driver.RegionReporter); ok {
		region = rr.Region()
	}
	return ConnectionInfo{
		Model:     sess.Capabilities().Model,
		CATID:     id.CATID,
		Port:      id.Port,
		USBSerial: id.USBSerial,
		Region:    region,
		Demo:      demo,
	}, nil
}

// Disconnect closes the current session. Refuses while a transfer is
// running (task-15 brief §2) — cancel or wait for it first — or while
// Fix 2's App-level exclusive-operation reservation is held by a
// concurrently-running ReadRadio/DiffAgainstRadio/PrepareSend/
// ReadSettingsRadio (task 35) (adjudicated HIGH, Codex M6 #2: without
// this, clearConnection() could render Idle on the frontend while a read
// was still using the session underneath it).
func (a *App) Disconnect() error {
	a.mu.Lock()
	if err := a.checkNotBusyLocked(); err != nil {
		a.mu.Unlock()
		return err
	}
	conn := a.conn
	a.conn = nil
	// A plan bound to the now-closed session's Service can never
	// succeed (Execute would refuse it with SessionChangedError even if
	// somehow retried against a new connection) — clear it for hygiene.
	a.currentPlan = nil
	a.mu.Unlock()

	if conn == nil {
		return ErrNotConnected
	}
	return conn.closer()
}

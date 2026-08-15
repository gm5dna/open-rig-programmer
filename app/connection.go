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
// output) — registry-driven, which M9c-6 confirmed rather than merely
// promised (registering the FTdx10 made this method return two models with
// no change to a line of it) and M9d-2 confirmed again (the FTdx101D and
// FTdx101MP took it to four, still with no change to a line of it).
// It is the list Connect/ConnectDemo's own
// model parameter is validated against (see connectModel), so a frontend
// model picker built on this method can never offer a model the connect
// path would then refuse.
func (a *App) GetSupportedModels() []string {
	return wiring.SupportedModels()
}

// Connect opens a session against a real radio of model, attached at
// portPath. An empty model means wiring.DefaultModel (the FT-710); any
// other value must name a model GetSupportedModels lists, or the call is
// refused before anything is opened or created — see connectModel.
func (a *App) Connect(portPath, model string) (ConnectionInfo, error) {
	return a.connect(false, portPath, model)
}

// ConnectDemo opens a session against the in-process simulated radio for
// model — never real hardware. Which simulator that is is the WIRING's
// business, not this method's: internal/wiring.OpenFakeSessionFor looks the
// model up in its own table and builds that model's own fake rig
// (internal/fakeradio for the FT-710, internal/fakedx10 for the FTdx10,
// internal/fakedx101 for both FTDX101 siblings). model follows Connect's
// rule exactly.
func (a *App) ConnectDemo(model string) (ConnectionInfo, error) {
	return a.connect(true, "", model)
}

// connectModel resolves Connect/ConnectDemo's model parameter to the model
// name the connect path's three model-keyed wiring calls all use (M9c-5
// E4).
//
// An empty request resolves through currentModel's OWN no-state tail
// rather than naming wiring.DefaultModel a second time here: this package
// has one model resolver, and routing the default through it keeps the
// connect default and the offline default structurally the same value,
// unable to drift apart.
//
// nil/nil is passed deliberately, and is not merely "there is no
// connection yet". The connect path must NEVER infer which radio to open
// from a loaded working copy: a user who loads an FTdx10 file and then
// clicks Connect with a real FT-710 on the cable would otherwise have the
// FTdx10's driver opened against FT-710 hardware. An empty model means
// "the default", never "whatever the file says" — naming a model is the
// caller's explicit act.
//
// That scenario stopped being hypothetical at M9c-6: a second model was
// registered, so an FTdx10 working copy and an FT-710 on the cable became a
// combination a user can really produce. M9d-2 made it four models and
// added a sharper case — an FTDX101D working copy with an FTDX101MP on the
// cable, two radios that differ in one CAT ID and nothing else. The wrong
// pairing would still be
// caught one layer down — each driver's Open probes the CAT ID and refuses
// a radio that is not its own (*driver.WrongRadioError, which since M9d-1
// carries WantModel and GotModel so the refusal can NAME both siblings) —
// but this rule is
// what stops the mismatch being ATTEMPTED, and it is the reason the
// protection does not depend on every future radio answering a
// distinguishable ID.
//
// A non-empty model is validated against supportedModels() — the same
// membership check, over the same list, that cmd/rigprog's validateModel
// (cmd/rigprog/wiring.go) applies to --model, reported with the same
// *wiring.UnknownModelError shape, whose Error() names every supported
// model. Validation runs BEFORE any side-effecting step (a directory
// created, a session opened), matching the CLI, rather than relying on
// the eventual OpenRealSessionFor/OpenFakeSessionFor lookup's own
// identical error after the snapshot directory has already been made.
func connectModel(requested string) (string, error) {
	if requested == "" {
		return currentModel(nil, nil), nil
	}
	for _, m := range supportedModels() {
		if m == requested {
			return requested, nil
		}
	}
	return "", &wiring.UnknownModelError{Model: requested, Supported: supportedModels()}
}

// connect is Connect/ConnectDemo's shared body. It deliberately calls
// wiring.OpenRealSessionFor/wiring.OpenFakeSessionFor — internal/wiring's
// own self-contained, model-keyed session paths (the real one is
// wiring.OpenRealSessionWith, of which OpenRealSessionFor is the
// zero-option delegate this function wants: it expresses no consent
// position) — rather than re-deriving the profile/session pairing here,
// preserving the structural-exclusivity shape those paths exist to
// enforce: no caller of either can supply a driver profile or a port
// object.
// Both, and the snapshot directory alongside them, are called at the ONE
// model connectModel resolved from requestedModel (M9c-5 E4) — never at
// wiring.DefaultModel independently, so a session, its snapshot directory
// and its journal can never belong to different radios.
//
// The model is validated first, before the already-connected/transfer
// guards: it is a pure argument check, and refusing a bad argument before
// reading App state keeps the refusal independent of when the call
// happens to arrive.
func (a *App) connect(demo bool, portPath, requestedModel string) (ConnectionInfo, error) {
	model, err := connectModel(requestedModel)
	if err != nil {
		return ConnectionInfo{}, fmt.Errorf("app: connecting: %w", err)
	}

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

	snapshotDir, err := wiring.ResolveSnapshotDir("", model)
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
		sess, closer, err = wiring.OpenFakeSessionFor(a.ctx, model)
	} else {
		sess, closer, err = wiring.OpenRealSessionFor(a.ctx, model, portPath)
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

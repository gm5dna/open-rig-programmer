// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// cloneGeneratorID identifies this GUI's clone-mode reads in a saved
// codeplug's Generator field, mirroring cmd/rigprog/read.go's
// cliGeneratorID and core/clone/service.go's generatorID — one constant
// per entry point, never shared, so a file's own Generator says which of
// the three actually produced it.
const cloneGeneratorID = "open-rig-programmer/app-clone"

// ErrCloneNotArmed is returned by ReceiveCloneImage (or CancelCloneRead)
// when no ArmCloneRead call is currently armed.
var ErrCloneNotArmed = errors.New("app: no clone-mode read is armed; call ArmCloneRead first")

// cloneReadState is one armed clone-mode read, held on the App between
// ArmCloneRead and the ReceiveCloneImage/CancelCloneRead call that
// follows it — see App.cloneState's doc comment (app.go) for the
// reservation this pairs with.
type cloneReadState struct {
	reception *clonewire.Reception
	port      transport.Port
	model     string

	// cancelled is set by CancelCloneRead when it runs while ArmCloneRead's
	// slow work (opening the reader/port, arming the wire) is still in
	// flight, i.e. before a.cloneState even held this cs — see
	// ArmCloneRead's publishClonePort/publishCloneReception calls. Guarded
	// by a.mu, like every other field here.
	cancelled bool
}

// openCloneRealPort is the real-port constructor ArmCloneRead calls, held
// in a variable for the same reason connection.go's openRealSessionWith
// is: a test cannot otherwise substitute internal/wiring.
// OpenCloneFakePort (there is no real hardware for any clone-mode radio —
// see plan.md's Verdict) without opening an actual serial port.
// PRODUCTION ALWAYS LEAVES IT AT wiring.OpenCloneRealPort.
var openCloneRealPort = wiring.OpenCloneRealPort

// GetCloneModels returns every clone-mode model name ArmCloneRead
// accepts (internal/wiring.CloneModels' own sorted output) — a SEPARATE
// list from GetSupportedModels, never merged with it: a clone-mode model
// implements no driver.Session (plan.md Phase 4's disjoint-registry
// proof), so it can never appear in the ordinary connect/read/send flow.
func (a *App) GetCloneModels() []string {
	return wiring.CloneModels()
}

// ArmCloneRead opens portPath for a clone-mode read of model, arms
// core/clonewire's receive primitive on it, and emits "clone:armed" once
// the arm/deadline clocks are live — spec.md §Read model point 1,
// §UI/CLI entry: arm-before-prompt is a definite ordering, so the
// frontend must not show its "put the radio into clone-send mode" prompt
// until this event has fired.
//
// Reserves a.opBusy as "ArmCloneRead" for the whole window between this
// call and the ReceiveCloneImage/CancelCloneRead call that follows it —
// the same reservation ReadRadio/PrepareSend/ReadSettingsRadio use, so a
// concurrent ordinary read/send and a clone read refuse each other
// exactly as those already refuse each other (reservation.go). Released
// again by ReceiveCloneImage/CancelCloneRead, or here on any failure to
// arm.
//
// a.cloneState is published the moment the reservation is taken, before
// any of the slow reader/port/wire-arm work below runs, so a
// CancelCloneRead racing in from a closed dialog always finds something
// to cancel. Without this, a cancel arriving while this call was still
// opening the port would see a.cloneState still nil, return
// ErrCloneNotArmed having released nothing, while this call went on to
// finish, store its own state, and keep the reservation -- leaking the
// port and a.opBusy for the rest of the session. publishClonePort/
// publishCloneReception below check cs.cancelled at each later step so
// whichever call "wins" releases the reservation exactly once.
func (a *App) ArmCloneRead(portPath, model string) error {
	a.mu.Lock()
	if err := a.reserveOpLocked("ArmCloneRead"); err != nil {
		a.mu.Unlock()
		return err
	}
	cs := &cloneReadState{model: model}
	a.cloneState = cs
	a.mu.Unlock()

	reader, profiles, err := wiring.OpenCloneReader(model)
	if err != nil {
		a.abandonCloneArm(cs)
		return fmt.Errorf("app: arming clone read: %w", err)
	}
	port, err := openCloneRealPort(portPath, profiles[0])
	if err != nil {
		a.abandonCloneArm(cs)
		return fmt.Errorf("app: arming clone read: opening port: %w", err)
	}
	if !a.publishClonePort(cs, port) {
		// CancelCloneRead ran while we were still opening the port: it has
		// already cleared a.cloneState and released a.opBusy, so we only
		// need to close what it could not see yet.
		_ = port.Close()
		return ErrCloneNotArmed
	}
	reception, err := reader.Arm(a.ctx, port, profiles)
	if err != nil {
		_ = port.Close()
		a.abandonCloneArm(cs)
		return fmt.Errorf("app: arming clone read: %w", err)
	}
	<-reception.Armed()

	if !a.publishCloneReception(cs, reception) {
		_ = port.Close()
		return ErrCloneNotArmed
	}

	a.emit("clone:armed", CloneArmedEvent{Model: model})
	return nil
}

// abandonCloneArm cleans up after an ArmCloneRead failure that happened
// before cs.port was published, so a concurrent CancelCloneRead (if any)
// saw no port to close. If cs.cancelled is already true, CancelCloneRead
// got there first and has already cleared a.cloneState and released
// a.opBusy -- do nothing further, so the reservation is released exactly
// once either way.
func (a *App) abandonCloneArm(cs *cloneReadState) {
	a.mu.Lock()
	cancelled := cs.cancelled
	if !cancelled {
		a.cloneState = nil
	}
	a.mu.Unlock()
	if !cancelled {
		a.releaseOp()
	}
}

// publishClonePort records the opened port on cs, unless CancelCloneRead
// has already cancelled it -- in which case Cancel has released a.opBusy
// and the caller must close the port itself. Returns false in that case.
func (a *App) publishClonePort(cs *cloneReadState, port transport.Port) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if cs.cancelled {
		return false
	}
	cs.port = port
	return true
}

// publishCloneReception is publishClonePort's counterpart for the
// reception handle reader.Arm returns.
func (a *App) publishCloneReception(cs *cloneReadState, reception *clonewire.Reception) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if cs.cancelled {
		return false
	}
	cs.reception = reception
	return true
}

// CancelCloneRead releases an ArmCloneRead call that the operator never
// followed through to ReceiveCloneImage (the dialogue was closed at the
// pick/arming/prompt stage) — without this, a.opBusy's reservation would
// stay held for the rest of the session, refusing every later ReadRadio/
// PrepareSend/ReadSettingsRadio. ErrCloneNotArmed if nothing is armed.
func (a *App) CancelCloneRead() error {
	a.mu.Lock()
	cs := a.cloneState
	if cs == nil {
		a.mu.Unlock()
		return ErrCloneNotArmed
	}
	cs.cancelled = true
	a.cloneState = nil
	port := cs.port // may still be nil if ArmCloneRead hasn't opened it yet
	a.mu.Unlock()
	if port != nil {
		_ = port.Close()
	}
	a.releaseOp()
	return nil
}

// ReceiveCloneImage completes an ArmCloneRead call: it blocks until the
// whole image has arrived (or a deadline/checksum refusal), then loads
// the result as the working copy exactly as ReadRadio does (baseline,
// working, workingPath/dirty/baselineStale all reset) — the operator's
// existing SaveFile/SaveFileAs then carries the codeplug's RawImage
// field to disk through the same core/codeplug.Save call an ordinary
// read/edit session already uses (app/fileio.go), with no
// clone-mode-specific save path needed.
//
// Model identity is OPERATOR-ASSERTED (spec.md §Identity probe): CHIRP's
// own source cannot distinguish FT-857 from FT-857D, or FT-897 from
// FT-897D. RadioInfo.Model records exactly the string the operator
// selected in ArmCloneRead — the image itself cannot confirm it against
// a same-length sibling.
//
// Emits transfer:progress/transfer:done with Kind "clone", reusing the
// existing payload shapes (types.go) so StatusBar/AlertStrip render this
// transfer with no new code. Any of ErrImageIncomplete/
// ErrImageIncompatible/ErrImageAmbiguous (spec.md §Read model point 5:
// every one is a full refusal, never a partial image) is reported as
// Outcome "refused"; the frontend's CloneReadDialog renders Message
// directly, per errorText.js's existing convention.
func (a *App) ReceiveCloneImage() (CodeplugView, error) {
	a.mu.Lock()
	cs := a.cloneState
	a.mu.Unlock()
	if cs == nil {
		return CodeplugView{}, ErrCloneNotArmed
	}
	defer func() {
		_ = cs.port.Close()
		a.mu.Lock()
		a.cloneState = nil
		a.mu.Unlock()
		a.releaseOp()
	}()

	a.emit("transfer:progress", ProgressEvent{Phase: "clone-receive", Done: 0, Total: 1})

	img, err := cs.reception.Receive(a.ctx)
	if err != nil {
		outcome, message := classifyCloneReceiveOutcome(err)
		a.emitDone("clone", outcome, nil, message)
		return CodeplugView{}, fmt.Errorf("app: receiving clone image: %w", err)
	}

	cp := &codeplug.Codeplug{
		Generator: cloneGeneratorID,
		Radio: codeplug.RadioInfo{
			Model:  cs.model,
			ReadAt: time.Now(),
		},
		Channels: clonewire.MapChannels(img),
		RawImage: &codeplug.RawImageBlob{
			Model:     cs.model,
			ProfileID: img.Profile.ProfileID,
			Bytes:     img.Raw,
		},
	}

	a.mu.Lock()
	a.baseline = cp
	a.working = deepCopyCodeplug(cp)
	a.bumpWorkingRevLocked()
	a.workingPath = ""
	a.dirty = false
	a.baselineStale = false
	view := a.codeplugViewLocked()
	a.mu.Unlock()

	a.emitDone("clone", "ok", nil, "")
	return view, nil
}

// classifyCloneReceiveOutcome maps Receive's typed refusal to
// transfer:done's Outcome vocabulary plus a Message the frontend shows
// verbatim (errorText.js) — mirrors cmd/rigprog/cloneread.go's
// classifyCloneReceiveErr, restated here since that one is unexported in
// a package this one must not import. ErrImageIncompatible's message
// hints at the other regional/model variant, per the brief: this
// family's siblings (FT-817ND vs FT-817NDUS, FT-857 vs FT-857D, ...) are
// registered as SEPARATE clone models (internal/wiring/clonefake.go), so
// "wrong model selected" often means "pick the sibling instead".
func classifyCloneReceiveOutcome(err error) (outcome, message string) {
	if isCancelled(err) {
		return "cancelled", "cancelled"
	}
	switch {
	case errors.Is(err, clonewire.ErrImageIncomplete):
		return "refused", fmt.Sprintf("Incomplete clone-mode image: %v", err)
	case errors.Is(err, clonewire.ErrImageIncompatible):
		return "refused", fmt.Sprintf("Incompatible clone-mode image: %v — try the other regional or model variant (e.g. the ND/US suffix, or the plain vs D suffix), since this project registers each as a separate model.", err)
	case errors.Is(err, clonewire.ErrImageAmbiguous):
		return "refused", fmt.Sprintf("Ambiguous clone-mode image: %v", err)
	default:
		return "error", err.Error()
	}
}

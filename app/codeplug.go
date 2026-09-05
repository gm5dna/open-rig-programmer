// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// codeplugViewLocked builds a CodeplugView from the App's current
// working copy. Callers must hold a.mu and have already checked
// a.working != nil.
func (a *App) codeplugViewLocked() CodeplugView {
	return CodeplugView{
		Schema:        a.working.Schema,
		Generator:     a.working.Generator,
		Radio:         a.working.Radio,
		Channels:      copyChannels(a.working.Channels),
		WorkingPath:   a.workingPath,
		Dirty:         a.dirty,
		BaselineStale: a.baselineStale,
	}
}

// ReadRadio reads every slot from the connected radio (svc.ReadAll),
// setting BOTH baseline (the fresh read itself) and working (an
// independent deep copy of it) — clearing dirty, baselineStale, and
// workingPath (a fresh read is not the same artefact as whatever was
// last loaded/saved). Emits transfer:progress during the read and
// transfer:done (Kind "read") exactly once on completion.
//
// Fix 2 (adjudicated HIGH, Codex M6 #2): reserves the App-level
// exclusive-operation slot (a.opBusy) for its whole duration, so
// UpdateChannel(s)/SaveFile*/LoadFile*/Import*/Export*/Disconnect/
// another radio op are refused with a typed busy error rather than
// racing this call's eventual commit — see reservation.go. Without this,
// a slow ReadRadio could silently overwrite edits made while it was
// still in flight (the reservation prevents any edit from happening at
// all during that window, so there is nothing to overwrite).
func (a *App) ReadRadio() (CodeplugView, error) {
	a.mu.Lock()
	conn := a.conn
	if conn == nil {
		a.mu.Unlock()
		return CodeplugView{}, ErrNotConnected
	}
	if err := a.reserveOpLocked("ReadRadio"); err != nil {
		a.mu.Unlock()
		return CodeplugView{}, err
	}
	a.mu.Unlock()
	defer a.releaseOp()

	cp, err := conn.svc.ReadAll(a.ctx)
	if err != nil {
		outcome, message := classifyReadDiffOutcome(err)
		a.emitDone("read", outcome, nil, message)
		return CodeplugView{}, fmt.Errorf("app: reading radio: %w", friendlyErr(err))
	}

	a.mu.Lock()
	a.baseline = cp
	a.working = deepCopyCodeplug(cp)
	a.bumpWorkingRevLocked() // Fix 4: working-copy content replaced
	a.workingPath = ""
	a.dirty = false
	a.baselineStale = false
	view := a.codeplugViewLocked()
	a.mu.Unlock()

	a.emitDone("read", "ok", nil, "")
	return view, nil
}

// GetCodeplug returns the current working copy, or ErrNothingLoaded if
// ReadRadio/LoadFile has never populated one.
func (a *App) GetCodeplug() (CodeplugView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.working == nil {
		return CodeplugView{}, ErrNothingLoaded
	}
	return a.codeplugViewLocked(), nil
}

// applyEditsLocked is UpdateChannel/UpdateChannels' shared body. Every
// ch.Slot must already exist in working's inventory — task-15 brief §2:
// "slot must exist in working's inventory" — checked for the WHOLE
// batch before applying ANY of it (refuse wholesale, matching this
// project's established convention: see internal/csvmerge's
// MergeCSV/MergeCHIRP), so a bulk paste with one bad slot never
// partially applies. Runs exactly one codeplug.Validate pass afterwards
// (task-15 brief §2: "one validation pass"), against the same
// connected-authoritative/disconnected-advisory capabilities Validate()
// uses. Callers must hold a.mu and have already checked a.working != nil.
func (a *App) applyEditsLocked(chs []codeplug.Channel) (EditResult, error) {
	index := make(map[string]int, len(a.working.Channels))
	for i, c := range a.working.Channels {
		index[c.Slot] = i
	}
	for _, ch := range chs {
		if _, ok := index[ch.Slot]; !ok {
			return EditResult{}, &UnknownSlotError{Slot: ch.Slot}
		}
	}
	// The CONNECTED session's capabilities when connected, which is the
	// right question for the Validate below (task-15 brief §2's
	// connected-authoritative/disconnected-advisory rule) and for nothing
	// else in this function.
	caps, _ := currentCaps(a.conn, a.working)
	for _, ch := range chs {
		a.working.Channels[index[ch.Slot]] = ch
	}
	// An edit may carry bare Absent from a v2 import or frontend state; key
	// unreachable fields before Diff can mistake them for a radio change (see
	// TestUpdateChannel_NormalisesUnreachableAbsentTierField) — against THE
	// WORKING COPY'S OWN model, so a field the working copy's radio really
	// has stays Absent for Validate to refuse even when a different radio is
	// connected (see
	// TestUpdateChannel_MismatchedConnectedModelKeepsReachableAbsent, and
	// normaliseTierFieldsForOwnModel for why).
	normaliseTierFieldsForOwnModel(a.working)
	a.bumpWorkingRevLocked() // Fix 4: working-copy channels mutated
	a.dirty = true

	issues := codeplug.Validate(a.working, caps)
	return EditResult{Issues: issuesToView(issues), Dirty: a.dirty}, nil
}

// UpdateChannel applies one channel edit to the working copy. Refused
// with a typed busy error while Fix 2's reservation is held by a
// concurrently-running ReadRadio/DiffAgainstRadio/PrepareSend/
// ReadSettingsRadio (task 35), or while a send transfer is running — see
// checkNotBusyLocked.
func (a *App) UpdateChannel(ch codeplug.Channel) (EditResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.checkNotBusyLocked(); err != nil {
		return EditResult{}, err
	}
	if a.working == nil {
		return EditResult{}, ErrNothingLoaded
	}
	return a.applyEditsLocked([]codeplug.Channel{ch})
}

// UpdateChannels applies a bulk edit (e.g. a paste) to the working
// copy — same semantics as UpdateChannel, one validation pass for the
// whole batch, and the same busy refusal.
func (a *App) UpdateChannels(chs []codeplug.Channel) (EditResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.checkNotBusyLocked(); err != nil {
		return EditResult{}, err
	}
	if a.working == nil {
		return EditResult{}, ErrNothingLoaded
	}
	return a.applyEditsLocked(chs)
}

// Validate validates the working copy: against the connected session's
// effective capabilities (authoritative) when connected, or the static
// offline baseline (advisory: true) when not — see currentCaps' doc
// comment.
func (a *App) Validate() (ValidationView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.working == nil {
		return ValidationView{}, ErrNothingLoaded
	}
	caps, advisory := currentCaps(a.conn, a.working)
	issues := codeplug.Validate(a.working, caps)
	return ValidationView{Issues: issuesToView(issues), Advisory: advisory}, nil
}

// DiffAgainstRadio computes a fresh, read-only comparison between the
// radio's CURRENT contents and the working copy: a fresh svc.ReadAll
// (never the App's cached baseline) diffed via codeplug.Diff against a
// DEEP COPY of working (Fix 2, adjudicated HIGH, Codex M6 #2: the old
// shape captured a.working's own pointer under a quick lock and then
// dereferenced it OUTSIDE mu for the whole ReadAll/Diff duration —
// concurrent with UpdateChannel(s), which mutates that same struct under
// mu, that was a genuine unsynchronized read/write), using the connected
// session's capabilities. Unlike PrepareSend, this never snapshots,
// journals, or mutates the App's baseline/working state — it is purely
// informational. Requires a connection and a loaded working copy. Emits
// transfer:progress during the read and transfer:done (Kind "diff")
// exactly once on completion.
//
// Reserves the App-level exclusive-operation slot (a.opBusy) for its
// whole duration — see ReadRadio's doc comment for why, and
// reserveOpLocked's doc comment for why this checks ONLY a.opBusy (not
// a.transfer.running): a DiffAgainstRadio call made during a running
// send transfer continues to collide with clone.Service's OWN op lock
// instead, exactly as before this fix.
func (a *App) DiffAgainstRadio() (DiffView, error) {
	a.mu.Lock()
	conn := a.conn
	if conn == nil {
		a.mu.Unlock()
		return DiffView{}, ErrNotConnected
	}
	if a.working == nil {
		a.mu.Unlock()
		return DiffView{}, ErrNothingLoaded
	}
	if err := a.reserveOpLocked("DiffAgainstRadio"); err != nil {
		a.mu.Unlock()
		return DiffView{}, err
	}
	workingCopy := deepCopyCodeplug(a.working)
	a.mu.Unlock()
	defer a.releaseOp()

	freshBaseline, err := conn.svc.ReadAll(a.ctx)
	if err != nil {
		outcome, message := classifyReadDiffOutcome(err)
		a.emitDone("diff", outcome, nil, message)
		return DiffView{}, fmt.Errorf("app: diffing against radio: %w", friendlyErr(err))
	}

	result, err := codeplug.Diff(freshBaseline, workingCopy, conn.session.Capabilities())
	if err != nil {
		a.emitDone("diff", "error", nil, err.Error())
		return DiffView{}, fmt.Errorf("app: diffing against radio: %w", err)
	}

	a.emitDone("diff", "ok", nil, "")
	return DiffView{Diff: buildDiffSummary(result)}, nil
}

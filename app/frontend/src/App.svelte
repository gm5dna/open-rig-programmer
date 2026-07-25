<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Application shell (task-16 brief §2): wholesale replacement of the M0
	// Wails template. Layout per the plan: connection bar, action bar,
	// (alert strip when there's something to say), view switch (task 36),
	// content (ChannelGrid or SettingsViewer), status bar. The grid
	// placeholder became the real ChannelGrid in task 17.
	import ConnectionBar from './lib/ConnectionBar.svelte'
	import ActionBar from './lib/ActionBar.svelte'
	import AlertStrip from './lib/AlertStrip.svelte'
	import ChannelGrid from './lib/ChannelGrid.svelte'
	import SettingsViewer from './lib/SettingsViewer.svelte'
	import StatusBar from './lib/StatusBar.svelte'
	import { appState } from './lib/state/app.svelte.js'
	import { initTransferEvents, refreshUISpec, refreshSettingsSpec, refreshAppVersion, setWindowTitle } from './lib/bridge/bindings.js'

	$effect(() => {
		initTransferEvents()
		// Initial (offline-baseline) grid/settings layout; refreshed again
		// by the bridge after connect/disconnect (settings) or
		// connect/disconnect/read/load/import (channels).
		void refreshUISpec()
		void refreshSettingsSpec()
		// The build version, fetched once — it cannot change while the app
		// is running, so nothing refreshes it.
		void refreshAppVersion()
	})

	// Channels|Settings view switch (task 36, M8b-6): pure frontend state
	// (appState.activeView) — Go is never consulted, and switching loses
	// no data in either view (both read straight from appState).
	// ConnectionBar/ActionBar/AlertStrip/StatusBar stay global and
	// unconditional; only the content area below switches. A small,
	// complete tablist — same pattern as ChannelGrid's bank tabs and
	// SettingsViewer's own menu tabs (roving tabindex, ArrowLeft/Right
	// wrap, Home/End).
	/** @type {{id: 'channels' | 'settings', label: string}[]} */
	const VIEWS = [
		{ id: 'channels', label: 'Channels' },
		{ id: 'settings', label: 'Settings' },
	]

	/** @param {'channels' | 'settings'} id */
	function selectView(id) {
		if (appState.activeView === id) return
		appState.setActiveView(id)
	}

	/** @param {KeyboardEvent} e @param {number} index */
	function onViewTabKeydown(e, index) {
		let target = null
		if (e.key === 'ArrowRight') target = (index + 1) % VIEWS.length
		else if (e.key === 'ArrowLeft') target = (index - 1 + VIEWS.length) % VIEWS.length
		else if (e.key === 'Home') target = 0
		else if (e.key === 'End') target = VIEWS.length - 1
		if (target === null) return
		e.preventDefault()
		selectView(VIEWS[target].id)
		document.getElementById(`view-tab-${VIEWS[target].id}`)?.focus()
	}

	// Title bar (task-18: "Open Rig Programmer — <filename>[*]", dirty
	// asterisk). Wails' own runtime already exposes WindowSetTitle — no Go
	// binding gap — so this effect just derives the display string and
	// hands it to bindings.js (the only wailsjs importer).
	$effect(() => {
		const cp = appState.codeplug
		if (!cp) {
			setWindowTitle('Open Rig Programmer')
			return
		}
		const name = cp.WorkingPath ? cp.WorkingPath.split(/[\\/]/).pop() : 'Untitled codeplug'
		setWindowTitle(`Open Rig Programmer — ${name}${appState.dirty ? '*' : ''}`)
	})
</script>

<ConnectionBar />
<ActionBar />
<AlertStrip />

<div class="view-tabs" role="tablist" aria-label="Views">
	{#each VIEWS as view, i (view.id)}
		<button
			type="button"
			role="tab"
			id={`view-tab-${view.id}`}
			aria-selected={appState.activeView === view.id}
			aria-controls="view-panel"
			tabindex={appState.activeView === view.id ? 0 : -1}
			class="view-tab"
			class:active={appState.activeView === view.id}
			onclick={() => selectView(view.id)}
			onkeydown={(e) => onViewTabKeydown(e, i)}
		>
			{view.label}
		</button>
	{/each}
</div>

<div class="view-panel" id="view-panel" role="tabpanel" aria-labelledby={`view-tab-${appState.activeView}`}>
	{#if appState.activeView === 'settings'}
		<SettingsViewer />
	{:else}
		<ChannelGrid />
	{/if}
</div>

<StatusBar />

<style>
	.view-tabs {
		display: flex;
		gap: var(--space-1);
		padding: var(--space-2) var(--space-4) 0;
		background: var(--colour-bg);
		border-top: 1px solid var(--colour-hairline);
	}

	.view-tab {
		display: inline-flex;
		align-items: center;
		padding: var(--space-2) var(--space-4);
		border: 1px solid var(--colour-hairline);
		border-bottom: none;
		border-radius: var(--radius-sm) var(--radius-sm) 0 0;
		background: var(--colour-panel-sunken);
		color: var(--colour-text-dim);
		font-size: 13px;
		font-weight: 600;
	}

	.view-tab.active {
		color: var(--colour-text);
		box-shadow: inset 0 2px 0 var(--colour-accent);
	}

	.view-panel {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}
</style>

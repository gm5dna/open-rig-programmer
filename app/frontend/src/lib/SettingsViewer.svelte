<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Read-only settings browser (task 36, M8b-6). Structure/labels come
	// wholesale from appState.settingsSpec (GetSettingsSpec — works
	// offline, never a value); content comes from appState.settings
	// (GetSettings/ReadSettingsRadio). ZERO protocol facts hardcoded here
	// — every menu/group/item label and ID is spec-supplied; the synthetic
	// vitest fixture (SettingsViewer.test.js) proves it.
	//
	// Own menu-level tablist (the spec's Menus): fresh markup, NOT copied
	// from ChannelGrid.svelte, but BEHAVIOURALLY the same complete tab
	// pattern (roving tabindex, ArrowLeft/Right wrap, Home/End,
	// aria-selected, aria-controls -> a single shared tabpanel).
	//
	// Empty states (brief §"Decided design"): no codeplug -> guidance
	// text; no settings snapshot yet -> the SAME "Read settings from
	// radio" button doubles as the CTA (kept as ONE persistent control,
	// not a duplicate, so the read-button-disabled-reason tests below
	// have exactly one target regardless of which empty state is
	// showing); HasLegacy -> a notice that older preserved menu data
	// exists but cannot be rendered; unrecognised file IDs (not in the
	// spec) -> their own section.
	import { appState } from './state/app.svelte.js'
	import { readSettingsRadio } from './bridge/bindings.js'
	import ToolButton from './ToolButton.svelte'

	/** @typedef {import('../../wailsjs/go/models').main.SettingMenuView} SettingMenuView */
	/** @typedef {import('../../wailsjs/go/models').main.SettingItemView} SettingItemView */
	/** @typedef {import('../../wailsjs/go/models').main.SettingEntryView} SettingEntryView */

	const spec = $derived(appState.settingsSpec)
	const menus = $derived(spec?.Menus ?? [])

	/** @type {string | null} */
	let activeMenuId = $state(null)
	const activeMenu = $derived(menus.find((m) => m.ID === activeMenuId) ?? menus[0] ?? null)

	/** @param {string} id */
	function selectMenu(id) {
		// Guards on the DERIVED active menu, not the raw activeMenuId field
		// — mirrors ChannelGrid's selectBank (before any explicit
		// selection, activeMenuId is still null while activeMenu already
		// falls back to menus[0], so the first tab renders pre-highlighted;
		// guarding on activeMenuId alone would let a click on that
		// already-active first tab slip through as a "real" selection).
		if (activeMenu?.ID === id) return
		activeMenuId = id
	}

	/** @param {KeyboardEvent} e @param {number} index */
	function onMenuTabKeydown(e, index) {
		let target = null
		if (e.key === 'ArrowRight') target = (index + 1) % menus.length
		else if (e.key === 'ArrowLeft') target = (index - 1 + menus.length) % menus.length
		else if (e.key === 'Home') target = 0
		else if (e.key === 'End') target = menus.length - 1
		if (target === null || menus.length === 0) return
		e.preventDefault()
		selectMenu(menus[target].ID)
		document.getElementById(`settings-menu-tab-${menus[target].ID}`)?.focus()
	}

	// --- entries: joined to spec items by ID -------------------------------

	/** @type {Map<string, SettingEntryView>} */
	const entriesById = $derived.by(() => {
		/** @type {Map<string, SettingEntryView>} */
		const m = new Map()
		for (const entry of appState.settings?.Entries ?? []) m.set(entry.ID, entry)
		return m
	})

	/** Every item ID the spec (any menu, any group) knows about — used to
	 * find file entries the spec does NOT recognise.
	 * @type {Set<string>} */
	const specItemIds = $derived.by(() => {
		/** @type {Set<string>} */
		const ids = new Set()
		for (const menu of menus) {
			for (const group of menu.Groups ?? []) {
				for (const item of group.Items ?? []) ids.add(item.ID)
			}
		}
		return ids
	})

	const unrecognisedEntries = $derived((appState.settings?.Entries ?? []).filter((e) => !specItemIds.has(e.ID)))

	/** The badge text for one spec item's row, or null for a plain "known"
	 * value (brief: "no badge for known"). A spec item with no matching
	 * entry at all (the working copy's snapshot is incomplete — see
	 * SettingsView.Complete) is treated the same as "not yet read": there
	 * is no FT-710 State value for "absent", so this is a generic UI word,
	 * not a protocol fact.
	 * @param {SettingEntryView | undefined} entry @returns {string | null} */
	function badgeText(entry) {
		if (!entry) return 'not read'
		if (entry.State === 'known') return null
		return entry.State
	}

	// --- read settings -------------------------------------------------------

	async function doReadSettings() {
		try {
			await readSettingsRadio()
		} catch {
			// Alert strip already carries the message.
		}
	}
</script>

<div class="settings-region">
	{#if spec === null}
		<div class="settings-empty">
			<p class="settings-empty-title">Settings layout unavailable</p>
			<p class="settings-empty-hint">The radio's settings menu structure could not be loaded — see the alert above, then reconnect or reload.</p>
		</div>
	{:else}
		<div class="settings-toolbar">
			<ToolButton
				label="Read settings from radio"
				onclick={doReadSettings}
				disabled={!appState.canReadSettings}
				tooltip={appState.canReadSettings ? '' : appState.readSettingsBlockedReason}
			/>
		</div>

		{#if appState.settings?.HasLegacy}
			<p class="legacy-notice">
				This file carries preserved legacy menu data from an older format — it is kept safe but cannot be shown here.
			</p>
		{/if}

		{#if !appState.settings?.HasSnapshot}
			<div class="settings-empty">
				{#if appState.codeplug === null}
					<p class="settings-empty-title">No codeplug loaded</p>
					<p class="settings-empty-hint">Read Radio or open a saved codeplug to see settings here.</p>
				{:else}
					<p class="settings-empty-title">No settings read yet</p>
					<p class="settings-empty-hint">Read settings from the radio to see them here.</p>
				{/if}
			</div>
		{:else}
			<div class="menu-tabs" role="tablist" aria-label="Settings menus">
				{#each menus as menu, i (menu.ID)}
					<button
						type="button"
						role="tab"
						id={`settings-menu-tab-${menu.ID}`}
						aria-selected={activeMenu?.ID === menu.ID}
						aria-controls="settings-menu-panel"
						tabindex={activeMenu?.ID === menu.ID ? 0 : -1}
						class="menu-tab"
						class:active={activeMenu?.ID === menu.ID}
						onclick={() => selectMenu(menu.ID)}
						onkeydown={(e) => onMenuTabKeydown(e, i)}
					>
						{menu.Label}
					</button>
				{/each}
			</div>

			<div
				class="settings-panel"
				id="settings-menu-panel"
				role="tabpanel"
				aria-labelledby={activeMenu ? `settings-menu-tab-${activeMenu.ID}` : undefined}
			>
				{#if activeMenu}
					{#each activeMenu.Groups ?? [] as group (group.ID)}
						<section class="settings-group">
							<h3 class="settings-group-heading">{group.Label}</h3>
							<table class="report-table settings-table">
								<thead>
									<tr>
										<th scope="col" class="col-display">Display</th>
										<th scope="col">Label</th>
										<th scope="col">Value</th>
										<th scope="col">State</th>
									</tr>
								</thead>
								<tbody>
									{#each group.Items ?? [] as item (item.ID)}
										{@const entry = entriesById.get(item.ID)}
										{@const badge = badgeText(entry)}
										<tr>
											<td class="col-display">{item.Display}</td>
											<td>{item.Label}</td>
											<td>{entry ? entry.Value : '—'}</td>
											<td>
												{#if badge}
													<span class="settings-badge">{badge}</span>
												{/if}
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</section>
					{/each}
				{/if}
			</div>

			{#if unrecognisedEntries.length > 0}
				<section class="settings-group settings-unrecognised">
					<h3 class="settings-group-heading">Unrecognised settings</h3>
					<table class="report-table settings-table">
						<thead>
							<tr>
								<th scope="col">ID</th>
								<th scope="col">Value</th>
								<th scope="col">State</th>
							</tr>
						</thead>
						<tbody>
							{#each unrecognisedEntries as entry (entry.ID)}
								{@const badge = badgeText(entry)}
								<tr>
									<td>{entry.ID}</td>
									<td>{entry.Value}</td>
									<td>
										{#if badge}
											<span class="settings-badge">{badge}</span>
										{/if}
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</section>
			{/if}
		{/if}
	{/if}
</div>

<style>
	.settings-region {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow-y: auto;
	}

	/* --- empty states — mirrors ChannelGrid's .grid-empty look --- */

	.settings-empty {
		flex: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--space-1);
		margin: var(--space-4);
		background: var(--colour-panel-sunken);
		border: 1px dashed var(--colour-hairline);
		border-radius: var(--radius-md);
		text-align: center;
		padding: var(--space-5);
	}

	.settings-empty-title {
		margin: 0;
		font-size: 14px;
		font-family: var(--font-mono);
		color: var(--colour-text);
	}

	.settings-empty-hint {
		margin: 0;
		font-size: 12.5px;
		color: var(--colour-text-dim);
	}

	/* --- toolbar / legacy notice --- */

	.settings-toolbar {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-3) var(--space-4) 0;
	}

	.legacy-notice {
		margin: var(--space-2) var(--space-4) 0;
		padding: var(--space-2) var(--space-3);
		background: var(--colour-warn-bg);
		border-left: 3px solid var(--colour-warn);
		border-radius: var(--radius-sm);
		font-size: 12px;
		color: var(--colour-text-dim);
	}

	/* --- menu tabs --- */

	.menu-tabs {
		display: flex;
		gap: var(--space-1);
		padding: var(--space-2) var(--space-4) 0;
		background: var(--colour-bg);
	}

	.menu-tab {
		display: inline-flex;
		align-items: center;
		padding: var(--space-2) var(--space-4);
		border: 1px solid var(--colour-hairline);
		border-bottom: none;
		border-radius: var(--radius-sm) var(--radius-sm) 0 0;
		background: var(--colour-panel-sunken);
		color: var(--colour-text-dim);
		font-size: 12.5px;
	}

	.menu-tab.active {
		color: var(--colour-text);
		box-shadow: inset 0 2px 0 var(--colour-accent);
	}

	/* --- panel / tables --- */

	.settings-panel {
		margin: 0 var(--space-4);
		border: 1px solid var(--colour-hairline);
		border-radius: 0 var(--radius-sm) var(--radius-sm) var(--radius-sm);
		background: var(--colour-panel-sunken);
		padding: var(--space-3);
	}

	.settings-group {
		margin: var(--space-4) var(--space-4) 0;
	}

	.settings-group:first-child {
		margin-top: 0;
	}

	.settings-unrecognised {
		margin-bottom: var(--space-4);
	}

	.settings-group-heading {
		margin: 0 0 var(--space-2);
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--colour-text-faint);
	}

	.settings-table {
		width: 100%;
	}

	.col-display {
		font-family: var(--font-mono);
		color: var(--colour-text-dim);
		white-space: nowrap;
		width: 1%;
	}

	.settings-badge {
		display: inline-block;
		font-size: 10.5px;
		font-family: var(--font-mono);
		padding: 1px 6px;
		border-radius: 8px;
		background: var(--colour-warn-bg);
		color: var(--colour-warn);
		white-space: nowrap;
	}
</style>

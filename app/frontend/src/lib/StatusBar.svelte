<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Status bar (task-16 brief §3): dirty dot + validation-error count +
	// transfer progress (phase + counter + slot). Read-only — every field
	// here comes straight from appState, updated by transfer:progress/
	// transfer:done via bindings.js.
	import { appState } from './state/app.svelte.js'
	import { phaseLabel, kindLabel } from './transferLabels.js'

	const blockingCount = $derived(appState.blockingIssues.length)
	const progress = $derived(appState.transfer.progress)
	const progressPct = $derived(
		progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0
	)
	const lastOutcome = $derived(appState.transfer.lastOutcome)
</script>

<div class="status-bar">
	<div class="status-item">
		<span class="dot" class:dot-warn={appState.dirty}></span>
		<span>{appState.dirty ? 'Unsaved changes' : 'Saved'}</span>
	</div>

	<div class="status-item">
		<span class="dot" class:dot-danger={blockingCount > 0}></span>
		<span>{blockingCount} {blockingCount === 1 ? 'validation issue' : 'validation issues'}</span>
	</div>

	<div class="status-item status-transfer">
		{#if appState.transfer.active}
			<span class="transfer-label">
				{kindLabel(appState.transfer.kind)}: {phaseLabel(progress.phase)}
				{#if progress.total > 0}
					{progress.done}/{progress.total}
				{/if}
				{#if progress.slot}
					<span class="transfer-slot">{progress.slot}</span>
				{/if}
			</span>
			<span class="progress-track" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={progressPct}>
				<span class="progress-fill" style:width={`${progressPct}%`}></span>
			</span>
		{:else if lastOutcome && lastOutcome.Outcome !== 'ok'}
			<span class="transfer-label transfer-label-muted">
				Last {kindLabel(lastOutcome.Kind)}: {lastOutcome.Outcome}
			</span>
		{:else}
			<span class="transfer-label transfer-label-muted">Idle</span>
		{/if}
	</div>

	<!-- Build version. Rendered from GetAppVersion's Display, composed in
	     Go: the "unreleased build" wording is a statement the program
	     makes, not one assembled here. Absent until the startup fetch
	     resolves, and absent entirely if it fails — better no chip than a
	     wrong or placeholder version. -->
	{#if appState.appVersion}
		<div class="status-item status-version">
			<span
				class="version-label"
				class:version-label-unreleased={!appState.appVersion.IsRelease}
				data-testid="app-version"
			>{appState.appVersion.Display}</span>
		</div>
	{/if}
</div>

<style>
	.status-bar {
		display: flex;
		align-items: center;
		gap: var(--space-5);
		padding: var(--space-2) var(--space-4);
		background: var(--colour-panel-raised);
		border-top: 1px solid var(--colour-hairline);
		font-size: 12px;
		color: var(--colour-text-dim);
		flex-shrink: 0;
	}

	.status-item {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	.dot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background: var(--colour-text-faint);
		flex-shrink: 0;
	}

	.dot-warn {
		background: var(--colour-accent);
		box-shadow: 0 0 5px var(--colour-accent);
	}

	.dot-danger {
		background: var(--colour-danger);
		box-shadow: 0 0 5px var(--colour-danger);
	}

	.status-transfer {
		flex: 1;
		justify-content: flex-end;
		min-width: 0;
	}

	.transfer-label {
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		color: var(--colour-text);
		font-family: var(--font-mono);
		font-size: 11.5px;
	}

	.transfer-label-muted {
		color: var(--colour-text-faint);
	}

	.transfer-slot {
		color: var(--colour-accent-strong);
	}

	.status-version {
		flex-shrink: 0;
	}

	.version-label {
		font-family: var(--font-mono);
		font-size: 11.5px;
		color: var(--colour-text-faint);
		white-space: nowrap;
	}

	.version-label-unreleased {
		font-style: italic;
	}

	.progress-track {
		width: 8rem;
		height: 6px;
		border-radius: 3px;
		background: var(--colour-panel-sunken);
		overflow: hidden;
		flex-shrink: 0;
	}

	.progress-fill {
		display: block;
		height: 100%;
		background: var(--colour-accent);
		transition: width var(--transition-fast);
	}

	@media (prefers-reduced-motion: reduce) {
		.progress-fill {
			transition: none;
		}
	}
</style>

<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Dismissible inline alert strip for binding rejections (task-16 brief
	// §3: "not window.alert — never trigger native modal JS dialogs").
	// Reads appState.alerts directly and dismisses through appState — kept
	// deliberately dumb (no local state) so any caller of
	// bindings.js's reportError ends up here automatically.
	//
	// task-18: alert.kind distinguishes a rejection (danger red, the
	// original styling) from an informational toast (teal-ish neutral —
	// "friendly toast" for NothingToSend, "success toast with path" for
	// Export CSV) — both are the same dismissible strip, never a native
	// dialog; only the colouring differs.
	import { appState } from './state/app.svelte.js'
</script>

{#if appState.alerts.length > 0}
	<div class="alert-strip" role="alert" aria-live="assertive">
		{#each appState.alerts as alert (alert.id)}
			<div class="alert" class:alert-info={alert.kind === 'info'}>
				<span class="alert-message">{alert.message}</span>
				<button
					type="button"
					class="alert-dismiss"
					aria-label="Dismiss this message"
					onclick={() => appState.dismissAlert(alert.id)}
				>
					&times;
				</button>
			</div>
		{/each}
	</div>
{/if}

<style>
	.alert-strip {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		padding: var(--space-2) var(--space-4);
		background: var(--colour-bg);
		border-bottom: 1px solid var(--colour-hairline);
	}

	.alert {
		display: flex;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
		background: var(--colour-danger-bg);
		border-left: 3px solid var(--colour-danger);
		border-radius: var(--radius-sm);
	}

	.alert-info {
		background: rgba(67, 209, 176, 0.12);
		border-left-color: var(--colour-good);
	}

	.alert-message {
		flex: 1;
		font-size: 12.5px;
		color: var(--colour-text);
		word-break: break-word;
	}

	.alert-dismiss {
		flex-shrink: 0;
		width: 22px;
		height: 22px;
		display: grid;
		place-items: center;
		font-size: 16px;
		line-height: 1;
		color: var(--colour-text-dim);
		border-radius: var(--radius-sm);
		transition: color var(--transition-fast), background-color var(--transition-fast);
	}

	.alert-dismiss:hover {
		color: var(--colour-text);
		background: rgba(255, 255, 255, 0.08);
	}
</style>

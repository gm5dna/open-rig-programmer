<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Small presentational helper for ActionBar's toolbar buttons: a
	// disabled button still needs to explain itself, but WebKit (the
	// Wails webview engine) does not reliably show a `title` tooltip on a
	// disabled <button> itself — wrapping it in a span with the title
	// works around that, so this is the one place that trick lives.

	/** @type {{label: string, onclick?: () => void, disabled?: boolean, tooltip?: string}} */
	let { label, onclick = () => {}, disabled = false, tooltip = '' } = $props()
</script>

{#if disabled && tooltip}
	<span class="tool-btn-wrap" title={tooltip}>
		<button type="button" class="tool-btn" disabled aria-disabled="true">{label}</button>
	</span>
{:else}
	<button type="button" class="tool-btn" {disabled} {onclick} title={tooltip || undefined}>
		{label}
	</button>
{/if}

<style>
	.tool-btn {
		padding: var(--space-2) var(--space-3);
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-sm);
		background: var(--colour-panel-raised);
		color: var(--colour-text);
		font-size: 12.5px;
		transition: border-color var(--transition-fast), background-color var(--transition-fast), opacity var(--transition-fast);
	}

	.tool-btn:hover:not(:disabled) {
		border-color: var(--colour-accent);
	}

	.tool-btn:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.tool-btn-wrap {
		display: inline-flex;
	}
</style>

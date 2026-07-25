<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Connection flow (task-16 brief §3): port dropdown populated on mount
	// and on refresh; Connect/Disconnect with busy state; a Demo entry kept
	// as a wholly separate control (not a dropdown option) so it cannot be
	// mistaken for a real port even under a quick glance; a radio badge
	// styled like the rig's own VFO readout.
	import { appState } from './state/app.svelte.js'
	import { listPorts, connect, connectDemo, disconnect } from './bridge/bindings.js'

	let selectedPort = $state('')
	/** Which button triggered the current connect attempt — appState.connecting
	 * is one shared flag (bindings.js can't tell connect() from
	 * connectDemo() apart), so the label/spinner choice lives here.
	 * @type {'connect' | 'demo' | null} */
	let pendingAction = $state(null)

	$effect(() => {
		listPorts()
	})

	// Keep the selection valid as the port list changes (a refresh could
	// drop the previously-selected path).
	$effect(() => {
		if (selectedPort && !appState.ports.some((p) => p.Path === selectedPort)) {
			selectedPort = ''
		}
	})

	const busy = $derived(appState.connecting)
	const canPickPort = $derived(!appState.connected && !busy)

	/** @param {import('../../wailsjs/go/models').main.PortEntry} port */
	function portLabel(port) {
		const hints = port.Hints?.length ? ` (${port.Hints.join(', ')})` : ''
		return port.Description ? `${port.Path} — ${port.Description}${hints}` : `${port.Path}${hints}`
	}

	/** @param {unknown} iso */
	function formatReadTime(iso) {
		if (!iso || typeof iso !== 'string') return null
		const d = new Date(iso)
		if (Number.isNaN(d.getTime())) return null
		return d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', hour12: false })
	}

	const readTime = $derived(formatReadTime(appState.codeplug?.Radio?.read_at))
	const connection = $derived(appState.connection)

	async function handleConnect() {
		if (!selectedPort) return
		pendingAction = 'connect'
		try {
			await connect(selectedPort)
		} catch {
			// Alert strip already carries the message (bindings.js).
		} finally {
			pendingAction = null
		}
	}

	async function handleDemo() {
		pendingAction = 'demo'
		try {
			await connectDemo()
		} catch {
			// Alert strip already carries the message.
		} finally {
			pendingAction = null
		}
	}

	async function handleDisconnect() {
		try {
			await disconnect()
		} catch {
			// Alert strip already carries the message.
		}
	}
</script>

<div class="connection-bar">
	<div class="port-controls">
		<label class="field-label" for="port-select">Port</label>
		<select
			id="port-select"
			bind:value={selectedPort}
			disabled={!canPickPort || appState.portsLoading}
		>
			<option value="" disabled>
				{appState.portsLoading ? 'Scanning…' : appState.ports.length ? 'Select a port…' : 'No ports found'}
			</option>
			{#each appState.ports as port (port.Path)}
				<option value={port.Path}>{portLabel(port)}</option>
			{/each}
		</select>
		<button
			type="button"
			class="icon-button"
			aria-label="Refresh port list"
			title="Refresh port list"
			disabled={appState.portsLoading || appState.connected}
			onclick={() => listPorts()}
		>
			&#8635;
		</button>

		{#if appState.connected}
			<button
				type="button"
				class="btn btn-secondary"
				disabled={appState.transfer.active}
				title={appState.transfer.active ? 'A transfer is already running' : ''}
				onclick={handleDisconnect}
			>
				Disconnect
			</button>
		{:else}
			<button
				type="button"
				class="btn btn-primary"
				disabled={!selectedPort || busy}
				onclick={handleConnect}
			>
				{pendingAction === 'connect' ? 'Connecting…' : 'Connect'}
			</button>
		{/if}

		<button
			type="button"
			class="btn btn-demo"
			disabled={appState.connected || busy}
			onclick={handleDemo}
		>
			{pendingAction === 'demo' ? 'Starting demo…' : 'Demo (simulated radio)'}
		</button>
	</div>

	<div class="badge-slot">
		{#if connection}
			<div class="radio-badge" class:demo={connection.Demo}>
				<span class="status-dot" aria-hidden="true"></span>
				{#if connection.Demo}
					<span class="demo-tag">DEMO</span>
				{/if}
				<span class="badge-text">
					{connection.Model} · ID {connection.CATID}{connection.Demo
						? ''
						: ` · ${connection.Port}`}
					{#if readTime}
						<span class="badge-read"> · read {readTime}</span>
					{/if}
				</span>
			</div>
		{:else}
			<div class="radio-badge radio-badge-empty">
				<span class="status-dot status-dot-off" aria-hidden="true"></span>
				<span class="badge-text">Not connected</span>
			</div>
		{/if}
	</div>
</div>

<style>
	.connection-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-4);
		padding: var(--space-3) var(--space-4);
		background: var(--colour-panel-raised);
		border-bottom: 1px solid var(--colour-hairline);
		flex-wrap: wrap;
	}

	.port-controls {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		flex-wrap: wrap;
	}

	.field-label {
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--colour-text-faint);
	}

	select {
		background: var(--colour-panel-sunken);
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-sm);
		padding: var(--space-2) var(--space-3);
		min-width: 15rem;
		max-width: 22rem;
	}

	select:disabled {
		opacity: 0.55;
		cursor: not-allowed;
	}

	.icon-button {
		width: 30px;
		height: 30px;
		display: grid;
		place-items: center;
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-sm);
		color: var(--colour-text-dim);
		transition: border-color var(--transition-fast), color var(--transition-fast);
	}

	.icon-button:hover:not(:disabled) {
		color: var(--colour-text);
		border-color: var(--colour-text-dim);
	}

	.icon-button:disabled {
		opacity: 0.4;
		cursor: not-allowed;
	}

	.btn {
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius-sm);
		border: 1px solid transparent;
		font-weight: 600;
		font-size: 12.5px;
		white-space: nowrap;
		transition: background-color var(--transition-fast), border-color var(--transition-fast), opacity var(--transition-fast);
	}

	.btn:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.btn-primary {
		background: var(--colour-accent);
		color: #241605;
	}

	.btn-primary:hover:not(:disabled) {
		background: var(--colour-accent-strong);
	}

	.btn-secondary {
		background: transparent;
		border-color: var(--colour-hairline);
		color: var(--colour-text);
	}

	.btn-secondary:hover:not(:disabled) {
		border-color: var(--colour-text-dim);
	}

	.btn-demo {
		background: transparent;
		border: 1px dashed var(--colour-demo);
		color: var(--colour-demo);
	}

	.btn-demo:hover:not(:disabled) {
		background: rgba(180, 140, 242, 0.12);
	}

	.badge-slot {
		flex-shrink: 0;
	}

	/* The signature element: a small VFO/LCD-style inset readout, the one
	 * place this design spends its boldness — everything else stays
	 * quiet. Monospace, an inset "bezel" shadow, a status dot. */
	.radio-badge {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--colour-panel-sunken);
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-md);
		box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.45);
		font-family: var(--font-mono);
		font-size: 12.5px;
	}

	.radio-badge-empty {
		color: var(--colour-text-faint);
	}

	.status-dot {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--colour-good);
		box-shadow: 0 0 6px var(--colour-good);
		flex-shrink: 0;
	}

	.status-dot-off {
		background: var(--colour-text-faint);
		box-shadow: none;
	}

	.radio-badge.demo .status-dot {
		background: var(--colour-demo);
		box-shadow: 0 0 6px var(--colour-demo);
	}

	.demo-tag {
		font-family: var(--font-ui);
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.08em;
		color: var(--colour-bg);
		background: var(--colour-demo);
		border-radius: 3px;
		padding: 1px 5px;
	}

	.badge-text {
		color: var(--colour-text);
		white-space: nowrap;
	}

	.badge-read {
		color: var(--colour-text-dim);
	}

	.radio-badge.demo {
		border: 1px dashed var(--colour-demo);
		background-image: repeating-linear-gradient(
			135deg,
			rgba(180, 140, 242, 0.08) 0,
			rgba(180, 140, 242, 0.08) 6px,
			transparent 6px,
			transparent 12px
		);
	}
</style>

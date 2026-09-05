<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Connection flow (task-16 brief §3): port dropdown populated on mount
	// and on refresh; Connect/Disconnect with busy state; a Demo entry kept
	// as a wholly separate control (not a dropdown option) so it cannot be
	// mistaken for a real port even under a quick glance; a radio badge
	// styled like the rig's own VFO readout.
	//
	// Task 13 (M9d) adds the model picker beside the port picker: same
	// label/select idiom, fed entirely by Go's GetSupportedModels, and
	// disabled on the same gate the port picker uses (a session's model is
	// fixed once it is open).
	//
	// Task 14 (M9d) adds the unverified-write consent affordances, and this
	// bar is where they live because this is where a session is opened and
	// described: a PERSISTENT "Unverified writes…" control (the grants panel
	// must be reachable whether or not anything is connected — a user has to
	// be able to revoke a grant for a radio that is not on the cable), and
	// the standing amber indicator, which is a SHORTCUT to that same panel
	// rather than the only way in. Both dialogue modes are mounted here.
	import { appState } from './state/app.svelte.js'
	import { listPorts, refreshSupportedModels, connect, connectDemo, disconnect } from './bridge/bindings.js'
	import UnverifiedWritesDialog from './UnverifiedWritesDialog.svelte'

	let selectedPort = $state('')
	/** Which button triggered the current connect attempt — appState.connecting
	 * is one shared flag (bindings.js can't tell connect() from
	 * connectDemo() apart), so the label/spinner choice lives here.
	 * @type {'connect' | 'demo' | null} */
	let pendingAction = $state(null)

	$effect(() => {
		listPorts()
		// Fetched once, like the app version: which models a build supports
		// cannot change while it runs. Never throws (bindings.js) — a failed
		// fetch leaves the picker with its default entry alone, which still
		// connects.
		void refreshSupportedModels()
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
	/** The model picker's gate — deliberately the same one the port picker
	 * uses: a session's model is fixed the moment it opens, so the choice
	 * can only be changed while disconnected and idle. */
	const canPickModel = $derived(!appState.connected && !busy)

	/** @param {Event & {currentTarget: HTMLSelectElement}} e */
	function onModelChange(e) {
		// Every mutation goes through appState's own setters (see
		// app.svelte.js's module comment) — bindings.js reads
		// appState.selectedModel back when Connect/Demo is pressed.
		appState.setSelectedModel(e.currentTarget.value)
	}

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

	/** Whether the connected radio has no transmitter — UISpecView.Transmit,
	 * which is core/spec.Capabilities.Transmit stringified
	 * ("has_transmitter" / "receive_only" / "" for a capability set that
	 * never stated its anatomy). The badge is where this app already says
	 * WHICH radio is on the cable, so it is where the one fact about that
	 * radio's anatomy belongs.
	 *
	 * ONLY 'receive_only' says anything: a transceiver needs no label, and
	 * the unspecified zero value is not evidence of either answer.
	 *
	 * It is NOT consulted anywhere else, and specifically not to decide
	 * which columns the channel grid renders — that is BankView.Fields'
	 * contract alone (grid/columns.js's columnsFor), derived per bank from
	 * the radio's own FieldSupport, and it must not gain a second source
	 * that could disagree with it. */
	const receiveOnly = $derived(appState.uiSpec?.Transmit === 'receive_only')

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
		<label class="field-label" for="model-select">Radio</label>
		<select
			id="model-select"
			class="model-select"
			value={appState.selectedModel}
			disabled={!canPickModel}
			onchange={onModelChange}
			title="Which radio to open a session as"
		>
			<!-- The empty value is not a placeholder: it is a real choice
			     meaning "this build's own default radio", which is what the
			     app has always connected as. It is deliberately not named
			     here — nothing bound exposes wiring.DefaultModel, and a
			     hard-coded name would drift the day the default changes. -->
			<option value="">Default</option>
			{#each appState.supportedModels as model (model)}
				<option value={model}>{model}</option>
			{/each}
		</select>

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

		<!-- Never disabled: the grants panel is a management surface, not an
		     action on the session. Whether a change it offers can be MADE
		     right now is the panel's own guard (appState.
		     canChangeUnverifiedConsent), which explains itself in place. -->
		<button
			type="button"
			class="btn btn-secondary"
			title="Which radios may be written to with unverified commands"
			onclick={() => appState.openUnverifiedGrants()}
		>
			Unverified writes…
		</button>
	</div>

	<div class="badge-slot">
		{#if appState.unverifiedWritesArmed}
			<!-- Amber, and derived from the live session's capability label
			     alone (appState.unverifiedWritesArmed — see its doc comment):
			     never from the settings file, which would light for a grant
			     the running session has not actually spent. -->
			<button
				type="button"
				class="armed-badge"
				title="This session may write commands that have never been proven on real hardware"
				onclick={() => appState.openUnverifiedGrants()}
			>
				Unverified writes enabled
			</button>
		{/if}

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
					{#if receiveOnly}
						<span class="badge-receive-only"> · Receive only</span>
					{/if}
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

<!-- The arming question takes precedence over the panel: it is asked once,
     about the radio just connected, and answering it is what makes the
     panel's row for that radio meaningful. -->
{#if appState.unverifiedConsentPrompt}
	<UnverifiedWritesDialog mode="arm" />
{:else if appState.unverifiedGrantsOpen}
	<UnverifiedWritesDialog mode="manage" />
{/if}

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

	/* A model name is a short string — the port picker's width would leave
	 * this one mostly empty. */
	.model-select {
		min-width: 8rem;
		max-width: 12rem;
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
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	/* The standing amber state (spec: "a standing amber 'unverified writes
	 * enabled' state"), in the same idiom every other caution in this app
	 * uses — AlertStrip's tinted ground and left rule, sized to sit beside
	 * the VFO readout rather than compete with it. */
	.armed-badge {
		padding: var(--space-2) var(--space-3);
		background: var(--colour-warn-bg);
		border: 1px solid var(--colour-warn);
		border-left-width: 3px;
		border-radius: var(--radius-sm);
		color: var(--colour-warn);
		font-size: 11.5px;
		font-weight: 600;
		letter-spacing: 0.02em;
		white-space: nowrap;
	}

	.armed-badge:hover {
		background: var(--colour-warn);
		color: var(--colour-bg);
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

	/* Dimmed like the read time beside it: a standing fact about the
	   radio, not a warning about anything the user has done. */
	.badge-receive-only {
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

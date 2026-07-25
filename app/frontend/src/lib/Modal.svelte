<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Generic in-app modal shell (task-18 brief: "modal dialogues are
	// in-app components with focus management (trap focus in the modal,
	// Escape closes where safe — never mid-transfer)"; NEVER a native
	// alert/confirm/prompt). Every task-18 dialogue (send review/transfer,
	// dirty-guard, import result) wraps its content in this — it owns
	// nothing about WHAT it shows, only the modal mechanics.
	//
	// Focus trap: Tab/Shift-Tab cycle within the dialog's own focusable
	// elements only (never escaping to the page behind); the element that
	// had focus before the modal opened is restored when it closes.
	// `closable` gates BOTH Escape and a backdrop click — the send-flow
	// dialogue passes `closable={false}` while its transfer is actually
	// running, so a transfer can never be dismissed out from under the
	// user by an accidental Escape or click (the brief's "never
	// mid-transfer").

	/** @type {{
	 *   labelledBy: string,
	 *   closable?: boolean,
	 *   onclose?: () => void,
	 *   children?: import('svelte').Snippet,
	 * }} */
	let { labelledBy, closable = true, onclose = () => {}, children } = $props()

	/** @type {HTMLElement | undefined} */
	let dialogEl = $state(undefined)
	/** @type {Element | null} */
	let previouslyFocused = null

	const FOCUSABLE_SELECTOR =
		'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

	/** @returns {HTMLElement[]} */
	function focusableElements() {
		if (!dialogEl) return []
		return Array.from(dialogEl.querySelectorAll(FOCUSABLE_SELECTOR)).filter(
			/** @returns {el is HTMLElement} */
			(el) => el instanceof HTMLElement && el.offsetParent !== null
		)
	}

	// Focused synchronously, NOT via tick().then(...): $effect already runs
	// after this component (and its snippet children) have committed to
	// the DOM, so dialogEl and its descendants are present by the time
	// this body runs. An earlier version deferred through tick(), which
	// left a real (if narrow) window where a Tab pressed immediately after
	// mount could race the deferred initial-focus microtask and have its
	// effect undone — caught by Modal.test.js's Tab-wrap tests failing
	// intermittently depending on await timing.
	$effect(() => {
		previouslyFocused = document.activeElement
		const first = focusableElements()[0]
		;(first ?? dialogEl)?.focus()
		return () => {
			if (previouslyFocused instanceof HTMLElement) previouslyFocused.focus()
		}
	})

	/** @param {KeyboardEvent} e */
	function onKeydown(e) {
		if (e.key === 'Escape') {
			if (!closable) return
			e.preventDefault()
			onclose()
			return
		}
		if (e.key !== 'Tab') return
		const items = focusableElements()
		if (items.length === 0) {
			e.preventDefault()
			return
		}
		const first = items[0]
		const last = items[items.length - 1]
		if (e.shiftKey && document.activeElement === first) {
			e.preventDefault()
			last.focus()
		} else if (!e.shiftKey && document.activeElement === last) {
			e.preventDefault()
			first.focus()
		}
	}

	function onBackdropClick() {
		if (closable) onclose()
	}
</script>

<div class="modal-backdrop" onclick={onBackdropClick} role="presentation">
	<div
		class="modal-panel"
		role="dialog"
		aria-modal="true"
		aria-labelledby={labelledBy}
		tabindex="-1"
		bind:this={dialogEl}
		onkeydown={onKeydown}
		onclick={(e) => e.stopPropagation()}
	>
		{@render children?.()}
	</div>
</div>

<style>
	.modal-backdrop {
		position: fixed;
		inset: 0;
		z-index: 100;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: var(--space-5);
		background: rgba(8, 11, 16, 0.72);
	}

	/* A lifted panel — outset shadow, the inverse of the radio badge's
	 * inset "readout" bezel — so a modal reads as something ABOVE the
	 * instrument panel, not another sunken readout. */
	.modal-panel {
		width: min(640px, 100%);
		max-height: min(720px, 100%);
		display: flex;
		flex-direction: column;
		background: var(--colour-panel-raised);
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-md);
		box-shadow: 0 16px 48px rgba(0, 0, 0, 0.55);
		overflow: hidden;
	}

	@media (prefers-reduced-motion: no-preference) {
		.modal-panel {
			animation: modal-in var(--transition-fast);
		}
	}

	@keyframes modal-in {
		from {
			opacity: 0;
			transform: translateY(4px) scale(0.99);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
</style>

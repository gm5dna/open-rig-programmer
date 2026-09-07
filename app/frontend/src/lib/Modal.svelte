<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<script>
	// Generic in-app modal shell (task-18 brief: "modal dialogues are
	// in-app components with focus management (trap focus in the modal,
	// Escape closes where safe — never mid-transfer)"; NEVER a native
	// alert/confirm/prompt). Every task-18 dialogue (send review/transfer,
	// dirty-guard, import result, confirm) wraps its content in this — it
	// owns nothing about WHAT it shows, only the modal mechanics.
	//
	// A native <dialog> shown via showModal() gives Tab/Shift-Tab focus
	// containment and the ::backdrop overlay for free (ponytail audit
	// 2026-09-06, finding 44) — no hand-rolled focus trap needed. It does
	// NOT give initial focus on the first focusable descendant (the HTML
	// spec's own focusing steps only honour an explicit `autofocus`
	// attribute, which none of our dynamic children declare) or restore
	// focus to the trigger on close, so both stay explicit below, same as
	// before. `closable` gates BOTH Escape (via the 'cancel' event) and a
	// backdrop click — the send-flow dialogue passes `closable={false}`
	// while its transfer is actually running, so a transfer can never be
	// dismissed out from under the user by an accidental Escape or click
	// (the brief's "never mid-transfer").

	/** @type {{
	 *   labelledBy: string,
	 *   closable?: boolean,
	 *   onclose?: () => void,
	 *   children?: import('svelte').Snippet,
	 * }} */
	let { labelledBy, closable = true, onclose = () => {}, children } = $props()

	/** @type {HTMLDialogElement | undefined} */
	let dialogEl = $state(undefined)
	/** @type {Element | null} */
	let previouslyFocused = null

	const FOCUSABLE_SELECTOR =
		'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

	// Focused synchronously, NOT via tick().then(...): $effect already runs
	// after this component (and its snippet children) have committed to
	// the DOM, so dialogEl and its descendants are present by the time
	// this body runs. showModal() itself must run here too (not in
	// markup) — the dialog must already be in the DOM to open it.
	$effect(() => {
		previouslyFocused = document.activeElement
		dialogEl?.showModal()
		const first = dialogEl?.querySelector(FOCUSABLE_SELECTOR)
		;(/** @type {HTMLElement | null} */ (first) ?? dialogEl)?.focus()
		return () => {
			if (previouslyFocused instanceof HTMLElement) previouslyFocused.focus()
		}
	})

	/** The 'cancel' event fires on Escape, before the dialog closes —
	 * cancelable, so preventDefault() here is what keeps it open while
	 * !closable (mirrors the old keydown handler's early return).
	 * @param {Event} e */
	function onCancel(e) {
		if (!closable) e.preventDefault()
	}

	/** A click that lands on the dialog element ITSELF (never a
	 * descendant — the ::backdrop pseudo-element is not a real hit-testable
	 * child, so a genuine backdrop click always targets dialogEl directly)
	 * closes it, gated by `closable` exactly like Escape.
	 * @param {MouseEvent} e */
	function onBackdropClick(e) {
		if (e.target === dialogEl && closable) onclose()
	}
</script>

<!-- svelte-ignore a11y_no_redundant_roles -- explicit role="dialog" kept
     (finding 44: "keep aria-labelledby/role semantics") for the tests
     and any tooling that queries it, even though <dialog> implies it. -->
<dialog
	bind:this={dialogEl}
	class="modal-panel"
	role="dialog"
	aria-modal="true"
	aria-labelledby={labelledBy}
	tabindex="-1"
	oncancel={onCancel}
	onclose={() => onclose()}
	onclick={onBackdropClick}
>
	{@render children?.()}
</dialog>

<style>
	/* A lifted panel — outset shadow, the inverse of the radio badge's
	 * inset "readout" bezel — so a modal reads as something ABOVE the
	 * instrument panel, not another sunken readout. */
	.modal-panel {
		width: min(640px, 100%);
		max-height: min(720px, calc(100% - 2 * var(--space-5)));
		margin: auto;
		padding: 0;
		display: flex;
		flex-direction: column;
		background: var(--colour-panel-raised);
		border: 1px solid var(--colour-hairline);
		border-radius: var(--radius-md);
		box-shadow: 0 16px 48px rgba(0, 0, 0, 0.55);
		overflow: hidden;
	}

	.modal-panel::backdrop {
		background: rgba(8, 11, 16, 0.72);
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

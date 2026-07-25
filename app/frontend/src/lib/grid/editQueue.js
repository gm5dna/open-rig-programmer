// SPDX-License-Identifier: GPL-3.0-or-later

// A FIFO edit queue (Fix 3, adjudicated MED, Codex M6 #3): ChannelGrid's
// cell commits used to be fire-and-forget, each sending a WHOLE
// ChannelData built EAGERLY from whatever the grid displayed at commit
// time. Two rapid edits to different fields of the SAME slot could both
// be built from the same pre-edit base (neither has seen the other's
// result yet), so whichever Go call's response happened to settle LAST
// silently discarded the other field — a last-write-win clobber
// determined by promise-settlement order, not commit order.
//
// This queue stores each edit as (slot, transform) — a pure function
// from the slot's CURRENT ChannelData to its next ChannelData — rather
// than a pre-built channel, and materialises + sends the outgoing
// channel from the FRESHEST data only once it is actually dequeued,
// awaiting each `commit` call before starting the next. Two edits to the
// same slot therefore always compose correctly: the second always sees
// the first's already-applied result, regardless of how quickly they
// were made.
//
// Global-ish by construction (one queue instance per ChannelGrid mount —
// "global is fine — edits are small", per the adjudicated remedy): edits
// to DIFFERENT slots share the one FIFO too, simply serialising Go calls
// one at a time rather than tracking per-slot queues, which the small
// data volumes here do not need.

/**
 * @typedef {Object} EditQueueDeps
 * @property {(slot: string) => any} getData - returns the freshest data
 *   for slot (or null/undefined if the slot has none yet), read at
 *   DEQUEUE time — never at enqueue time.
 * @property {(channel: {slot: string, data: any}) => Promise<any>} commit
 *   - sends one materialised channel and resolves/rejects with the
 *   result.
 * @property {(err: unknown) => void} [onError] - called if commit
 *   rejects; the queue keeps draining the rest regardless — one failed
 *   edit must never silently swallow the edits queued after it.
 */

/**
 * @param {EditQueueDeps} deps
 */
export function createEditQueue({ getData, commit, onError }) {
	/** @type {{slot: string, apply: (data: any) => any}[]} */
	const queue = []
	let draining = false
	/** @type {(() => void)[]} */
	let idleWaiters = []

	function settleIdleWaiters() {
		const waiters = idleWaiters
		idleWaiters = []
		for (const resolve of waiters) resolve()
	}

	async function drain() {
		if (draining) return
		draining = true
		while (queue.length > 0) {
			const next = queue.shift()
			if (!next) break
			const { slot, apply } = next
			const data = apply(getData(slot))
			try {
				await commit({ slot, data })
			} catch (err) {
				onError?.(err)
			}
		}
		draining = false
		settleIdleWaiters()
	}

	/**
	 * Enqueues one edit: apply(freshData) must return the FULL next
	 * ChannelData for slot (typically `{ ...cloneData(freshData), field:
	 * newValue }`) — computed lazily, only once this edit is actually
	 * dequeued.
	 * @param {string} slot
	 * @param {(data: any) => any} apply
	 */
	function enqueue(slot, apply) {
		queue.push({ slot, apply })
		void drain()
	}

	/** Resolves once every currently-queued (and any since-enqueued
	 * before it settles) edit has been committed — test-only convenience,
	 * not needed by ChannelGrid itself (which is fire-and-forget from the
	 * caller's perspective, same as before this fix — only the ORDERING
	 * and freshness of what gets sent changed).
	 * @returns {Promise<void>} */
	function idle() {
		if (!draining && queue.length === 0) return Promise.resolve()
		return new Promise((resolve) => idleWaiters.push(resolve))
	}

	return { enqueue, idle }
}

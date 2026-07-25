// SPDX-License-Identifier: GPL-3.0-or-later

// createEditQueue tests (Fix 3, adjudicated MED, Codex M6 #3): pure
// logic, no DOM/appState — a stub getData/commit pair stands in for
// ChannelGrid's real channelBySlot/updateChannel.

import { describe, it, expect, vi } from 'vitest'
import { createEditQueue } from '../editQueue.js'

describe('createEditQueue', () => {
	it('materialises and commits a single queued edit from getData at dequeue time', async () => {
		const getData = vi.fn().mockReturnValue({ freq_hz: 7000000, tag: 'MYCALL' })
		const commit = vi.fn().mockResolvedValue(undefined)
		const queue = createEditQueue({ getData, commit })

		queue.enqueue('001', (data) => ({ ...data, freq_hz: 7100000 }))
		await queue.idle()

		expect(commit).toHaveBeenCalledTimes(1)
		expect(commit).toHaveBeenCalledWith({ slot: '001', data: { freq_hz: 7100000, tag: 'MYCALL' } })
	})

	it('awaits each commit before dispatching the next — never two in flight at once', async () => {
		/** @type {(() => void)[]} */
		const releases = []
		const commit = vi.fn(
			() =>
				new Promise((resolve) => {
					releases.push(() => resolve(undefined))
				})
		)
		const getData = vi.fn().mockReturnValue({ freq_hz: 7000000 })
		const queue = createEditQueue({ getData, commit })

		queue.enqueue('001', (data) => ({ ...data, freq_hz: 7100000 }))
		queue.enqueue('001', (data) => ({ ...data, freq_hz: 7200000 }))

		await Promise.resolve()
		await Promise.resolve()
		expect(commit).toHaveBeenCalledTimes(1) // the second must NOT have started yet

		releases[0]()
		await Promise.resolve()
		await Promise.resolve()
		expect(commit).toHaveBeenCalledTimes(2) // now dispatched

		releases[1]()
		await queue.idle()
	})

	it('out-of-order regression: two rapid edits to different fields of the same slot both survive, materialised from the FRESHEST data at each dequeue', async () => {
		/** @type {Record<string, any>} */
		let store = { '001': { freq_hz: 7000000, tag: 'MYCALL' } }
		const getData = vi.fn((slot) => store[slot])
		const commit = vi.fn(async (channel) => {
			// Mirrors the real bridge: a successful commit updates the
			// backing store the NEXT dequeue's getData will read.
			store[channel.slot] = channel.data
		})
		const queue = createEditQueue({ getData, commit })

		// Edit 1: frequency. Edit 2: tag — fired before edit 1's commit
		// resolves (the fire-and-forget bug this fix closes would build
		// edit 2's payload from the STALE pre-edit-1 base).
		queue.enqueue('001', (data) => ({ ...data, freq_hz: 7100000 }))
		queue.enqueue('001', (data) => ({ ...data, tag: 'NEWCALL' }))

		await queue.idle()

		expect(commit).toHaveBeenCalledTimes(2)
		// The SECOND commit's payload (materialised at ITS OWN dequeue time,
		// after the first had already applied) must carry BOTH fields.
		const secondPayload = commit.mock.calls[1][0]
		expect(secondPayload.data.freq_hz).toBe(7100000)
		expect(secondPayload.data.tag).toBe('NEWCALL')
		expect(store['001']).toEqual({ freq_hz: 7100000, tag: 'NEWCALL' })
	})

	it('a rejected commit does not stop the queue draining the rest', async () => {
		const getData = vi.fn().mockReturnValue({ freq_hz: 7000000 })
		const commit = vi.fn().mockRejectedValueOnce(new Error('boom')).mockResolvedValue(undefined)
		const onError = vi.fn()
		const queue = createEditQueue({ getData, commit, onError })

		queue.enqueue('001', (data) => ({ ...data, freq_hz: 7100000 }))
		queue.enqueue('001', (data) => ({ ...data, freq_hz: 7200000 }))
		await queue.idle()

		expect(commit).toHaveBeenCalledTimes(2)
		expect(onError).toHaveBeenCalledTimes(1)
	})

	it('queues edits to DIFFERENT slots independently — both still commit in order', async () => {
		const getData = vi.fn((slot) => ({ slot, freq_hz: 7000000 }))
		const order = []
		const commit = vi.fn(async (channel) => {
			order.push(channel.slot)
		})
		const queue = createEditQueue({ getData, commit })

		queue.enqueue('001', (data) => data)
		queue.enqueue('002', (data) => data)
		await queue.idle()

		expect(order).toEqual(['001', '002'])
	})
})

// SPDX-License-Identifier: GPL-3.0-or-later

// The channel grid's column model: definitions (id, header label, the
// IssueView field name each column decorates from), per-cell
// editability, display formatting, Added-channel defaults and
// per-column paste-cell parsing. Pure data in, pure data out — no DOM,
// no state module.
//
// NO protocol vocabulary lives here (task-17 brief, controller
// amendment): modes, shift/CTCSS-state options, the tone table, the
// tag/clarifier limits and — since M9c-5's review (W1) — the Added-row
// tag-display default all arrive via the UISpec (GetUISpec — see
// bridge/bindings.js). The only fixed knowledge is STRUCTURAL: which
// columns exist, which ChannelData key each edits, and the
// codeplug.FieldState write rule (only 'known' is ever sent; a
// non-known ToneField/BoolField must carry a zero Value —
// core/codeplug/fieldstate.go's Valid).

import { hzToMHz, mhzToHz } from './freq.js'

/** @typedef {import('../../../wailsjs/go/models').codeplug.ChannelData} ChannelData */
/** @typedef {import('../../../wailsjs/go/models').main.BankView} BankView */
/** @typedef {import('../../../wailsjs/go/models').main.UISpecView} UISpecView */

/**
 * @typedef {Object} Column
 * @property {string} id
 * @property {string} label   column header (British English)
 * @property {string | null} field   IssueView.Field this column decorates from (null: never decorated)
 */

/** The ten brief-mandated columns, in visible order. @type {Column[]} */
export const COLUMNS = [
	{ id: 'slot', label: 'Slot', field: null },
	{ id: 'freq', label: 'Frequency (MHz)', field: 'frequency' },
	{ id: 'mode', label: 'Mode', field: 'mode' },
	{ id: 'clar', label: 'Clarifier (Hz)', field: 'clarifier' },
	{ id: 'shift', label: 'Shift', field: 'shift' },
	{ id: 'ctcss', label: 'CTCSS', field: 'ctcss_state' },
	{ id: 'tone', label: 'Tone', field: 'ctcss_tone' },
	{ id: 'skip', label: 'Scan skip', field: 'scan_skip' },
	{ id: 'tag', label: 'Tag', field: 'tag' },
	{ id: 'tagDisplay', label: 'Tag display', field: 'tag_display' },
]

/**
 * Whether this cell can open an editor. Bank-level read-only locking is
 * the component's job (BankView.ReadOnly gates the whole row before
 * this is consulted); this answers the per-cell question only:
 *   - Slot: never.
 *   - An empty slot (data null): only Frequency — editing it populates
 *     the slot (Added). Every other column needs a populated channel.
 *   - Tone/Scan skip: only when their FieldState is 'known' — the CAT
 *     protocol cannot read them, so a radio read leaves them 'unknown'
 *     (not sent on a write; whether the radio's own setting survives an
 *     unrelated write is unverified pending M5b hardware trials — see
 *     docs/hardware-notes.md's "M5b write-trial protocol"); a loaded
 *     file may carry them 'known'.
 *   - Tag display: when its FieldState is 'known' OR 'unknown', and never
 *     when it is 'unavailable' (M9c-5 E1 — it is a BoolField now, not a
 *     bare bool; M9c-6 D5b added the 'unknown' half).
 *
 *     Its provenance is what makes it differ from tone/skip. The CAT
 *     protocol DOES read this field where it exists, so a radio read and a
 *     migrated legacy file both leave it 'known', while a CHIRP import
 *     leaves it honestly 'unknown' — a QUESTION PUT TO THE USER, since the
 *     send plan blocks that channel until it is answered ("tag display
 *     unknown — set On or Off before sending"). Admitting 'unknown' here
 *     is what lets the user answer it in the cell (the toggle's first
 *     press means Display OFF — see ChannelGrid.svelte's toggleCell for
 *     the invention rule); before M9c-6 the only route was PASTING on/off
 *     into the column, which is still there and still the bulk route.
 *
 *     Tone and scan skip stay 'known'-only for the opposite reason: their
 *     'unknown' is not a question the user may answer, because on every
 *     radio that can produce it the field is UNREACHABLE — Unsupported in
 *     both directions, spec.FieldSupport.Unreachable — so answering it
 *     would only manufacture a value that is never sent. The rule is
 *     capability-derived, not a fact about all radios: a future radio
 *     whose scan-skip is genuinely writable imports {known, …} directly
 *     (core/csvio's chirpScanSkip) and is editable here by that route, so
 *     nothing needs relaxing when one arrives.
 *
 *     This paragraph got MORE load-bearing at M9d-2 task 8, which is why
 *     it now says which radios: a CHIRP import used to hand every channel
 *     a {known, false} scan skip regardless of the radio, and that Known
 *     was the state blocking the whole channel at plan time (the M9c-6
 *     manifest's A7 finding). Such imports now arrive 'unknown', so this
 *     column is non-editable for them — unchanged behaviour reached by a
 *     different route, since a Known-but-unwritable cell was equally
 *     pointless to edit.
 *
 *     UNAVAILABLE is refused for both, and by paste too (M9c-5 review W2,
 *     see paste.js): there is no flag in that radio's frame for any value
 *     to go into, so there is no question outstanding either.
 * @param {Column} column
 * @param {ChannelData | null | undefined} data
 * @returns {boolean}
 */
export function isCellEditable(column, data) {
	switch (column.id) {
		case 'slot':
			return false
		case 'freq':
			return true
		case 'tone':
			return data?.ctcss_tone?.state === 'known'
		case 'skip':
			return data?.scan_skip?.state === 'known'
		case 'tagDisplay': {
			const state = data?.tag_display?.state
			return state === 'known' || state === 'unknown'
		}
		default:
			return data != null
	}
}

/**
 * A tone in decihertz as display text, preferring the UISpec's own
 * table (Go-formatted), falling back to the same arithmetic for a
 * value the table does not list.
 * @param {number} decihertz
 * @param {UISpecView} uiSpec
 * @returns {string}
 */
function toneDisplay(decihertz, uiSpec) {
	const entry = (uiSpec.Tones ?? []).find((t) => t.Decihertz === decihertz)
	if (entry) return entry.Display
	return `${(decihertz / 10).toFixed(1)} Hz`
}

/**
 * The cell's display text. Empty slots render '' for every non-slot
 * column (the component adds the "empty" affordance itself); unreadable
 * tone/skip render an em dash (greyed by the component, with the
 * unverified-preservation tooltip), and so does a tag display that is
 * not 'known' — On/Off is a claim about the radio, and this grid does
 * not make claims it cannot support.
 * @param {Column} column
 * @param {ChannelData | null | undefined} data
 * @param {UISpecView} uiSpec
 * @returns {string}
 */
export function displayValue(column, data, uiSpec) {
	if (data == null) return ''
	switch (column.id) {
		case 'freq':
			return hzToMHz(data.freq_hz)
		case 'mode':
			return data.mode ?? ''
		case 'clar': {
			const hz = data.clar_hz ?? 0
			return hz > 0 ? `+${hz}` : String(hz)
		}
		case 'shift':
			return data.shift ?? ''
		case 'ctcss':
			return data.ctcss ?? ''
		case 'tone': {
			const tone = data.ctcss_tone
			if (tone?.state !== 'known' || typeof tone.value !== 'number') return '—'
			return toneDisplay(tone.value, uiSpec)
		}
		case 'skip': {
			const skip = data.scan_skip
			if (skip?.state !== 'known') return '—'
			return skip.value ? 'On' : 'Off'
		}
		case 'tag':
			return data.tag ?? ''
		case 'tagDisplay': {
			const display = data.tag_display
			if (display?.state !== 'known') return '—'
			return display.value ? 'On' : 'Off'
		}
		default:
			return ''
	}
}

/**
 * Defaults for a channel being ADDED by populating an empty slot's
 * frequency: neutral values drawn from the UISpec's own vocabularies
 * (Go lists the neutral option first in each — pinned by its tests),
 * with tone/scan skip 'unknown' (never sent on a write — see
 * core/codeplug/fieldstate.go's write rule; whether the radio's own
 * setting is actually preserved by an unrelated write is a separate,
 * still-open M5b question), exactly as a radio read leaves them.
 *
 * Tag display comes from the BANK, and that closes the last hardcoded
 * protocol fact in this module (M9c-5 review W1). It used to be the JS
 * literal `{state:'known', value:false}` — the FT-710's answer, asserted
 * unconditionally, with a revisit point recorded because the UISpec
 * carried no model. The UISpec now carries the answer itself:
 * BankView.TagDisplayDefault is derived per bank from that radio's own
 * spec.FieldTagDisplay support (app/uispec.go's bankTagDisplayDefault),
 * so a radio whose memory frame has no display flag serves
 * {state:'unavailable'} and this factory carries it straight through
 * instead of manufacturing a Known value the radio has no room for. For
 * the FT-710 the served value is the same Known-off it always was, for
 * the recorded reason: tag display is a MANDATORY wire field where it
 * exists (core/cat's MT set frame takes the display flag as a required
 * argument), and an 'unknown' would create a channel the send plan
 * immediately blocks.
 *
 * The value is COPIED (cloneFieldState), never aliased: the UISpec is
 * shared, long-lived state, and a row handed a reference into it would
 * let one channel's later edit reach every other Added row.
 * @param {UISpecView} uiSpec
 * @param {BankView} bank   the bank the new row belongs to
 * @param {number} freqHz
 * @returns {ChannelData}
 */
export function newChannelData(uiSpec, bank, freqHz) {
	return /** @type {ChannelData} */ ({
		freq_hz: freqHz,
		mode: uiSpec.Modes[0],
		clar_hz: 0,
		rx_clar: false,
		tx_clar: false,
		ctcss: uiSpec.CTCSSStateOptions[0],
		ctcss_tone: { state: 'unknown' },
		shift: uiSpec.ShiftOptions[0],
		tag: '',
		// Go always emits TagDisplayDefault (no omitempty), so the
		// 'unknown' fallback only bites on a hand-built BankView — where it
		// refuses to invent a value, exactly as cloneData does.
		tag_display: cloneFieldState(bank?.TagDisplayDefault),
		scan_skip: { state: 'unknown' },
	})
}

/**
 * A ToneField/BoolField copy honouring the codeplug write rule: the
 * `value` key exists only when state is 'known' (a non-known state must
 * carry a ZERO value — core/codeplug/fieldstate.go's Valid — and an
 * absent JSON key unmarshals to exactly that).
 * @param {{state?: string, value?: unknown} | undefined} f
 * @param {string} fallbackState
 */
function cloneFieldState(f, fallbackState = 'unknown') {
	const state = f?.state ?? fallbackState
	if (state === 'known' && f?.value !== undefined) return { state, value: f.value }
	return { state }
}

/**
 * A deep copy of one channel's data, normalising absent optional keys
 * to explicit zero values so an edited copy always round-trips the full
 * shape to UpdateChannel.
 * @param {ChannelData} data
 * @returns {ChannelData}
 */
export function cloneData(data) {
	return /** @type {ChannelData} */ ({
		freq_hz: data.freq_hz,
		mode: data.mode,
		clar_hz: data.clar_hz ?? 0,
		rx_clar: data.rx_clar ?? false,
		tx_clar: data.tx_clar ?? false,
		ctcss: data.ctcss,
		ctcss_tone: cloneFieldState(data.ctcss_tone),
		shift: data.shift,
		tag: data.tag ?? '',
		// Go always emits tag_display now (no omitempty), so the fallback
		// only bites on a hand-built shape: 'unknown' there refuses to
		// invent a value, exactly as tone/scan skip do.
		tag_display: cloneFieldState(data.tag_display),
		scan_skip: cloneFieldState(data.scan_skip),
	})
}

/** Accepted boolean spellings for pasted Scan skip / Tag display cells. */
const BOOL_WORDS = new Map([
	['on', true], ['true', true], ['yes', true], ['1', true],
	['off', false], ['false', false], ['no', false], ['0', false],
])

/**
 * Parses one pasted cell for a column into a partial-ChannelData patch.
 * PARSING only, not validation: numeric text must become numbers and
 * booleans booleans, but vocabulary/range checking stays with Go
 * (mode/shift/CTCSS pass through as trimmed text).
 * @param {Column} column
 * @param {string} text   non-empty (paste.js skips empty cells)
 * @param {UISpecView} uiSpec
 * @returns {{ok: true, patch: Partial<ChannelData>} | {ok: false, reason: string}}
 */
export function parsePasteCell(column, text, uiSpec) {
	switch (column.id) {
		case 'freq': {
			const hz = mhzToHz(text)
			if (hz === null) return { ok: false, reason: `"${text.trim()}" is not a frequency in MHz` }
			return { ok: true, patch: { freq_hz: hz } }
		}
		case 'mode':
			return { ok: true, patch: { mode: text.trim() } }
		case 'shift':
			return { ok: true, patch: { shift: text.trim() } }
		case 'ctcss':
			return { ok: true, patch: { ctcss: text.trim() } }
		case 'clar': {
			const trimmed = text.trim()
			if (!/^[+-]?\d+$/.test(trimmed)) {
				return { ok: false, reason: `"${trimmed}" is not a whole number of hertz for the clarifier` }
			}
			return { ok: true, patch: { clar_hz: Number(trimmed) } }
		}
		case 'tone': {
			const trimmed = text.trim().replace(/\s*Hz$/i, '')
			if (!/^\d+(\.\d+)?$/.test(trimmed)) {
				return { ok: false, reason: `"${text.trim()}" is not a tone in Hz` }
			}
			const value = Math.round(Number(trimmed) * 10)
			return { ok: true, patch: { ctcss_tone: /** @type {ChannelData['ctcss_tone']} */ ({ state: 'known', value }) } }
		}
		case 'skip': {
			const value = BOOL_WORDS.get(text.trim().toLowerCase())
			if (value === undefined) return { ok: false, reason: `"${text.trim()}" is not on/off for Scan skip` }
			return { ok: true, patch: { scan_skip: /** @type {ChannelData['scan_skip']} */ ({ state: 'known', value }) } }
		}
		case 'tag':
			return { ok: true, patch: { tag: text } }
		case 'tagDisplay': {
			const value = BOOL_WORDS.get(text.trim().toLowerCase())
			if (value === undefined) return { ok: false, reason: `"${text.trim()}" is not on/off for Tag display` }
			return { ok: true, patch: { tag_display: /** @type {ChannelData['tag_display']} */ ({ state: 'known', value }) } }
		}
		default:
			return { ok: false, reason: `the ${column.label} column cannot be pasted into` }
	}
}

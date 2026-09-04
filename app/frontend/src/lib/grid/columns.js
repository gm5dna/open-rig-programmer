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
 * The columns the Icom tier added, in ChannelData's own declaration
 * order. Unlike COLUMNS these are CONDITIONAL: one renders only where
 * the bank's own capabilities say the radio has that field
 * (BankView.Fields, from app/uispec.go's bankTierFields). `key` is the
 * ChannelData key the column edits; `field` is the spec.Field it
 * decorates IssueViews from, and the two agree by construction here.
 *
 * On the four Yaesu models every bank's Fields list is empty, so none of
 * these renders and the grid is exactly the ten-column grid it has
 * always been. The registered IC-7610 is the first radio for which any
 * of them renders: its memory record maps tone_mode, tone_tx, tone_rx
 * and filter, and nothing else here.

 * @typedef {Object} TierColumn
 * @property {string} id
 * @property {string} label   column header (British English)
 * @property {string} field   the spec.Field this column is, and decorates from
 * @property {string} key     the ChannelData key it edits
 * @property {'freq'|'tone'|'int'|'bool'|'text'} kind   how its value is rendered and parsed
 */

/** @type {TierColumn[]} */
export const TIER_COLUMNS = [
	{ id: 'txFreq', label: 'TX frequency (MHz)', field: 'tx_frequency', key: 'tx_frequency', kind: 'freq' },
	{ id: 'duplex', label: 'Duplex', field: 'duplex', key: 'duplex', kind: 'text' },
	{ id: 'offset', label: 'Offset (MHz)', field: 'offset', key: 'offset', kind: 'freq' },
	{ id: 'toneMode', label: 'Tone mode', field: 'tone_mode', key: 'tone_mode', kind: 'text' },
	{ id: 'toneTx', label: 'TX tone', field: 'tone_tx', key: 'tone_tx', kind: 'tone' },
	{ id: 'toneRx', label: 'RX tone', field: 'tone_rx', key: 'tone_rx', kind: 'tone' },
	{ id: 'dtcsCode', label: 'DTCS code', field: 'dtcs_code', key: 'dtcs_code', kind: 'int' },
	{ id: 'dtcsPolarity', label: 'DTCS polarity', field: 'dtcs_polarity', key: 'dtcs_polarity', kind: 'text' },
	{ id: 'filter', label: 'Filter', field: 'filter', key: 'filter', kind: 'text' },
	{ id: 'dataMode', label: 'Data mode', field: 'data_mode', key: 'data_mode', kind: 'bool' },
	{ id: 'tuningStepEnabled', label: 'Tuning step enabled', field: 'tuning_step_enabled', key: 'tuning_step_enabled', kind: 'bool' },
	{ id: 'tuningStep', label: 'Tuning step', field: 'tuning_step', key: 'tuning_step', kind: 'text' },
	{ id: 'programTuningStep', label: 'Program tuning step (Hz)', field: 'program_tuning_step', key: 'program_tuning_step', kind: 'int' },
	{ id: 'attenuator', label: 'Attenuator (dB)', field: 'attenuator', key: 'attenuator', kind: 'int' },
	{ id: 'preamp', label: 'Preamp', field: 'preamp', key: 'preamp', kind: 'text' },
	{ id: 'antenna', label: 'Antenna', field: 'antenna', key: 'antenna', kind: 'text' },
	{ id: 'ipPlus', label: 'IP+', field: 'ip_plus', key: 'ip_plus', kind: 'bool' },
]

/** TIER_COLUMNS keyed by column id, for the per-column helpers below. */
const TIER_BY_ID = new Map(TIER_COLUMNS.map((c) => [c.id, c]))

/**
 * The TierColumn a rendered column IS, or null for one of the ten that
 * predate the tier.
 *
 * Exported because ChannelGrid.svelte chooses a cell's editor by KIND
 * (TierColumn.kind — freq/int/text take the free-text editor and the
 * parser below, tone the UISpec's own tone list, bool the one-keystroke
 * toggle), and the lookup rule — BY ID, never by position — belongs
 * here beside the table it reads rather than restated in the component.
 * @param {Column} column
 * @returns {TierColumn | null}
 */
export function tierColumnFor(column) {
	return TIER_BY_ID.get(column.id) ?? null
}

/**
 * The columns to render for one bank: always the ten in COLUMNS, then
 * every TIER_COLUMNS entry that bank's capabilities say the radio
 * actually has (BankView.Fields).
 *
 * The asymmetry is deliberate and is the tier's whole frontend
 * contract. The ten pre-tier columns stay UNCONDITIONAL — their per-CELL
 * rules are state-based (isCellEditable), and re-deriving their
 * VISIBILITY from capabilities is a separate decision nobody has taken —
 * while a tier column has no such history and would be meaningless on a
 * radio with no such field. On the four Yaesu models Fields is empty on
 * every bank, so this returns exactly COLUMNS and their grid does not
 * change by so much as a column; on the IC-7610 it appends that radio's
 * own four.
 *
 * A missing or hand-built bank (no Fields) answers the same way an empty
 * one does: no tier columns. Refusing to invent a column for a radio
 * that has not said it has the field is the same posture as the rest of
 * this module.
 * @param {BankView | null | undefined} bank
 * @returns {Column[]}
 */
export function columnsFor(bank) {
	const fields = bank?.Fields
	if (!fields || fields.length === 0) return COLUMNS
	const present = new Set(fields)
	return [...COLUMNS, ...TIER_COLUMNS.filter((c) => present.has(c.field))]
}

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
	const tier = TIER_BY_ID.get(column.id)
	if (tier) {
		// A tier column's cell follows tag_display's rule, not
		// tone/scan-skip's: 'known', 'unknown' AND 'absent' are all
		// editable, and only 'unavailable' never is. The column only
		// renders where the radio HAS the field (columnsFor), so an
		// unresolved value there is a question the user may legitimately
		// answer — and 'absent' (the zero FieldState) is exactly that
		// question unanswered, same as 'unknown'. It reaches here two
		// ways: a missing key on a frontend-built row (columnsFor and
		// newChannelData both omit a key the radio does not reach), or an
		// empty string on a row the Go backend marshalled, which sends an
		// Absent FieldState as `{"state": ""}` — every tier field in
		// core/codeplug/channel.go and FieldState itself
		// (core/codeplug/fieldstate.go) has no `omitempty` on `state`, so
		// app/codeplug.go hands the frontend that empty string verbatim
		// rather than dropping the key. Only 'unavailable' stays refused:
		// it says the frame has no room for a value at all, so there is no
		// question outstanding to answer.
		if (data == null) return false // no populated channel: same empty-slot rule as every other column
		const state = /** @type {Record<string, any>} */ (data)[tier.key]?.state
		return state === 'known' || state === 'unknown' || state === undefined || state === ''
	}
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
	const tier = TIER_BY_ID.get(column.id)
	if (tier) {
		const f = /** @type {Record<string, any>} */ (data)[tier.key]
		// Anything but a Known value renders the em dash the grid already
		// uses for "this cell makes no claim" — unavailable, unknown and
		// the absent zero state alike. On/Off, a tone in Hz or a
		// frequency in MHz are claims about the radio, and this grid does
		// not make claims it cannot support.
		if (f?.state !== 'known') return '—'
		// A KNOWN field with no `value` key is Known ZERO, not a missing
		// answer: core/codeplug/fieldstate.go marks FieldState.Value
		// `json:"value,omitempty"`, so Go drops the key for every zero it
		// sends. Zero is a real answer on both numeric kinds — "zero, when
		// present, means off" for the attenuator (core/spec/capabilities.go's
		// AttenuatorDB), and a simplex channel's offset is a Known 0 Hz — so
		// reading the missing key as the em dash above would hide a
		// radio-supplied value inside the mark reserved for "no claim made".
		// Pinned by "renders a Known ZERO as zero" (tierColumns.test.js),
		// which asserts the two spellings agree.
		switch (tier.kind) {
			case 'freq':
				return hzToMHz(typeof f.value === 'number' ? f.value : 0)
			case 'tone':
				// The tone kind is NOT normalised the same way: zero decihertz
				// is not a tone any radio's table lists, so a Known tone with
				// no value is malformed rather than an answer, and the em dash
				// stays the honest rendering of it.
				return typeof f.value === 'number' ? toneDisplay(f.value, uiSpec) : '—'
			case 'int':
				return String(typeof f.value === 'number' ? f.value : 0)
			case 'bool':
				return f.value ? 'On' : 'Off'
			default:
				return f.value ?? ''
		}
	}
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
		// Every tier field this bank REACHES starts 'unknown' — the state
		// that says the answer is not yet in hand, which is the true
		// thing about a row that has just been created. No value is
		// invented, and a field the bank does not reach gets no key at
		// all, leaving it absent. On the four Yaesu models Fields is
		// empty on every bank, so this adds nothing; on the IC-7610 it
		// adds that radio's own four.
		...tierDefaults(bank),
	})
}

/**
 * The {state:'unknown'} starting value for every tier-added field the
 * bank reaches, keyed by ChannelData key.
 * @param {BankView | null | undefined} bank
 * @returns {Record<string, {state: string}>}
 */
function tierDefaults(bank) {
	const fields = bank?.Fields
	if (!fields || fields.length === 0) return {}
	const present = new Set(fields)
	/** @type {Record<string, {state: string}>} */
	const out = {}
	for (const column of TIER_COLUMNS) {
		if (present.has(column.field)) out[column.key] = { state: 'unknown' }
	}
	return out
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
		// The tier's ten are carried through VERBATIM, with no fallback
		// state: whatever Go sent is what goes back. That matters even on
		// a radio with none of them, where every one arrives
		// 'unavailable' — inventing an 'unknown' fallback here would turn
		// an untouched channel into a modified one the moment any cell
		// was edited, because codeplug.Diff compares ChannelData whole.
		// cloneTierFields spreads them in so an absent key stays absent.
		...cloneTierFields(data),
	})
}

/**
 * A copy of the tier-added fields present on data, key by key. A field
 * the incoming data does not carry is OMITTED rather than defaulted:
 * absent is a real state (core/codeplug's codeplug.Absent — "this
 * codeplug never spoke about the field"), and an absent JSON key
 * unmarshals to exactly it.
 * @param {ChannelData} data
 * @returns {Record<string, {state?: string, value?: unknown}>}
 */
function cloneTierFields(data) {
	/** @type {Record<string, {state?: string, value?: unknown}>} */
	const out = {}
	for (const column of TIER_COLUMNS) {
		const f = /** @type {Record<string, any>} */ (data)[column.key]
		if (f === undefined) continue
		out[column.key] = cloneFieldState(f)
	}
	return out
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
	const tier = TIER_BY_ID.get(column.id)
	if (tier) {
		const parsed = parseTierCell(tier, text)
		if (!parsed.ok) return parsed
		return { ok: true, patch: /** @type {Partial<ChannelData>} */ ({ [tier.key]: parsed.value }) }
	}
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

/**
 * Parses one pasted cell for a tier column into the {state, value}
 * shape its ChannelData field takes. PARSING only, exactly as
 * parsePasteCell is: whether the value is in this radio's vocabulary is
 * Go's question (codeplug.Validate), so a text cell passes through
 * trimmed and unjudged.
 *
 * Every result is state 'known'. A paste is somebody typing a value; the
 * non-Known states are not things a cell can be pasted INTO, and the
 * grid refuses those cells before reaching here (isCellEditable).
 * @param {TierColumn} column
 * @param {string} text   non-empty (paste.js skips empty cells)
 * @returns {{ok: true, value: {state: string, value: unknown}} | {ok: false, reason: string}}
 */
function parseTierCell(column, text) {
	const trimmed = text.trim()
	switch (column.kind) {
		case 'freq': {
			const hz = mhzToHz(trimmed)
			if (hz === null) return { ok: false, reason: `"${trimmed}" is not a frequency in MHz` }
			return { ok: true, value: { state: 'known', value: hz } }
		}
		case 'tone': {
			const hz = trimmed.replace(/\s*Hz$/i, '')
			if (!/^\d+(\.\d+)?$/.test(hz)) {
				return { ok: false, reason: `"${trimmed}" is not a tone in Hz` }
			}
			return { ok: true, value: { state: 'known', value: Math.round(Number(hz) * 10) } }
		}
		case 'int': {
			if (!/^\d+$/.test(trimmed)) {
				return { ok: false, reason: `"${trimmed}" is not a whole number for ${column.label}` }
			}
			return { ok: true, value: { state: 'known', value: Number(trimmed) } }
		}
		case 'bool': {
			const value = BOOL_WORDS.get(trimmed.toLowerCase())
			if (value === undefined) return { ok: false, reason: `"${trimmed}" is not on/off for ${column.label}` }
			return { ok: true, value: { state: 'known', value } }
		}
		default:
			return { ok: true, value: { state: 'known', value: trimmed } }
	}
}

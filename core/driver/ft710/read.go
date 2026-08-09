// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// readChannelGapHook, when non-nil, is called by ReadChannel between its
// MR and MT wire exchanges — a test-only seam, mirroring
// core/transport's openPort precedent (port.go: "OpenSerial's test seam
// ... production code always leaves this at its default"). Always nil in
// production, so it costs one nil check per read and nothing else.
//
// This exists for TestSession_ReadChannel_NotTornByConcurrentWriteChannel
// (M3 Codex-review fix wave, Fix 2, adjudicated MEDIUM) to
// deterministically force a concurrent WriteChannel's entire MW+MT
// sequence into the gap between ReadChannel's own two exchanges, rather
// than relying on random goroutine scheduling to occasionally hit it: Go's
// sync.Mutex "barging" behaviour (an immediately-re-locking goroutine
// almost always wins the fast-path CAS over a goroutine merely woken from
// waiting) makes the unsynchronised version of this race exceedingly hard
// to reproduce via black-box concurrent hammering alone — confirmed
// empirically (thousands of read/write iterations, zero reproductions)
// before this seam was added. See also the companion, randomly-scheduled
// stress test alongside it for a secondary, best-effort check.
var readChannelGapHook func()

// ctcssNames maps the wire CTCSS state to codeplug's display spelling
// ("OFF", "ENC-DEC", "ENC" — the strings codeplug.Validate checks for).
// Deliberately NOT cat.CTCSSState.String(), whose spellings ("off",
// "ENC/DEC") are log labels, not model values.
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:    "OFF",
	cat.CTCSSEncDec: "ENC-DEC",
	cat.CTCSSEnc:    "ENC",
}

// ctcssByName is ctcssNames' write-direction inverse.
var ctcssByName = map[string]cat.CTCSSState{
	"OFF":     cat.CTCSSOff,
	"ENC-DEC": cat.CTCSSEncDec,
	"ENC":     cat.CTCSSEnc,
}

// shiftNames maps the wire shift state to codeplug's display spelling
// ("SIMPLEX", "PLUS", "MINUS").
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// shiftByName is shiftNames' write-direction inverse.
var shiftByName = map[string]cat.Shift{
	"SIMPLEX": cat.ShiftSimplex,
	"PLUS":    cat.ShiftPlus,
	"MINUS":   cat.ShiftMinus,
}

// acceptedKinds is the LENIENT, documented set of P7 kind bytes an MR
// answer for slot may legitimately carry, per its bank.
//
// HW-CONFIRMED 2026-07-13 (M5b write trials against Stuart's real UK
// FT-710, docs/hardware-notes.md): a populated PMS slot's MR answer
// carried kind '1' (KindMemory), exactly as CAT-written — this project's
// former STRICT read-side check (kind must equal exactly KindPMS for
// EVERY PMS slot) is HARDWARE-REFUTED, and aborted whole reads
// (`rigprog read`, GUI ReadRadio, clone's PrepareSend) on any radio with
// a populated PMS pair. Reproduced live: an MR answer for slot "P1L"
// carrying kind '1' — want '5' — raised *KindMismatchError. The
// front-panel-created-PMS kind-byte variant remains UNKNOWN (never
// observed at M5b), so PMS read acceptance stays LENIENT — {KindMemory,
// KindPMS} — rather than narrowing to just '1'. Memory-bank slots
// (memory channels ONLY) accept {KindVFO, KindMemory, KindUnset}: '0'
// (VFO), '1' (Memory, the overwhelmingly common case), and '4' (the
// documented "-" placeholder cat.Dialect.ParseMRAnswer already accepts
// structurally) — all three genuinely observed on MEM (M5a/M5b).
//
// Discovered 60m/EMG banks (Codex M5b fix wave, Fix 5, adjudicated
// MEDIUM) do NOT share MEM's lenient set: they were never probed at all
// (doc.go register item 4; fakeradio's own kind60mEMG constant is
// ASSUMED, not HW-derived), so leniency borrowed from MEM's hardware
// findings would smuggle an unearned assumption onto a completely
// different, unverified bank — and, before this fix, could accept a
// contradictory answer (e.g. kind '0'/VFO on a 60m channel) into a fresh
// baseline with no HW evidence either way. These banks stay STRICT —
// {KindMemory} only — until a live session actually characterises one
// (there is no radio with a populated 5xx/EMG bank in this project's
// evidence to date). This is a READ-direction leniency ONLY, for every
// bank alike: write NEVER re-emits a read kind — write.go's
// buildWriteCommands always builds a fresh MW frame carrying the session
// dialect's own declared write kind, KindMemory for the FT-710 (and
// 60m/EMG slots can never reach a write at all: cat.Dialect.writableSlot,
// reached from cat's validateMWFields, structurally excludes them — and
// it decides on the dialect being written THROUGH, which is what makes it
// a gate rather than a hint) — so accepting a wider read-side set on
// MEM/PMS cannot smuggle a stale or wrong kind back onto the wire.
//
// That citation named cat.Slot.Writable until M9d removed it. The
// replacement is deliberately named without a line number: this comment
// has now been invalidated once by a change in another package, and a bare
// symbol name is the form that survives the next one.
func acceptedKinds(slot cat.Slot) []byte {
	switch {
	case slot.IsPMS():
		return []byte{cat.KindMemory, cat.KindPMS}
	case slot.Is60m() || slot.IsEMG():
		return []byte{cat.KindMemory}
	default:
		return []byte{cat.KindVFO, cat.KindMemory, cat.KindUnset}
	}
}

// kindAccepted reports whether got is one of slot's acceptedKinds.
func kindAccepted(slot cat.Slot, got byte) bool {
	for _, k := range acceptedKinds(slot) {
		if k == got {
			return true
		}
	}
	return false
}

// ReadChannel implements driver.Session: MR (channel data) + MT (tag)
// reads, mapped into one codeplug.Channel.
//
// The empty-slot rule: a "?;" rejection of the MR read is mapped to an
// EMPTY channel, not an error. ASSUMED (cross-ref fakeradio's register,
// item 2, and doc.go): the manual does not document what an MR read of
// an unpopulated slot answers, and "?;" is the protocol's single
// unattributed NAK — it could in principle mean something else went
// wrong, but "empty slot" is the only cause a well-formed MR read of a
// grammatical slot is known to have. Verify at M5a.
//
// CTCSSTone and ScanSkip come back FieldState Unknown, ALWAYS: the CAT
// protocol has no command that reads a memory channel's tone or scan
// skip, so the driver must never guess a value ("Unknown" means
// "preserve whatever the radio has" to every write path downstream).
// M5a is scoped to investigate a tone-index side channel (e.g. whether
// the MR answer's fixed "00" P9 field is really a tone index on some
// firmware); nothing is stored for P9 today — cat.Dialect.ParseMRAnswer
// already validates it is the documented constant "00", so there is
// nothing slot-specific to preserve.
//
// Kind sanity: the answer's P7 kind byte must be one of the slot's
// bank's acceptedKinds (LENIENT, HW-CONFIRMED 2026-07-13 — see
// acceptedKinds' doc comment) — a value outside that set means the radio
// and this driver disagree about what the slot IS, and mapping it anyway
// could mislabel data. The typed *KindMismatchError carries the full
// detail for the caller to log.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	// Fix 2: hold opMu for the WHOLE MR+MT sequence, not just around each
	// individual Do call — see Session's doc comment.
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel: %w", err)
	}

	mrCmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		// e.g. the answer-only "000" placeholder: grammatical per
		// ParseSlot, but never a legal read target.
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel: %w", err)
	}

	frame, err := s.eng.Do(ctx, mrCmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		// ASSUMED empty-slot answer — see the doc comment above.
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}
	if !kindAccepted(sl, m.Kind) {
		return codeplug.Channel{}, &KindMismatchError{Slot: sl.Wire(), Got: m.Kind, Want: acceptedKinds(sl)}
	}

	if readChannelGapHook != nil {
		readChannelGapHook()
	}

	// Tag + display via MT. fakeradio (register item 4) answers MT for
	// ANY grammatical slot, populated or not, so after a successful MR a
	// rejection here is a genuine error, not an empty-slot signal.
	mtCmd, err := s.dialect.BuildMTRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel %s: %w", sl.Wire(), err)
	}
	tframe, err := s.eng.Do(ctx, mtCmd, mtSpec())
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel %s: MT: %w", sl.Wire(), err)
	}
	tslot, display, tag, err := s.dialect.ParseMTAnswer(tframe)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel %s: %w", sl.Wire(), err)
	}
	if tslot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Requested: sl.Wire(), Answered: tslot.Wire()}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		// Unreachable after ParseMRAnswer's own validation; refuse
		// rather than silently mislabel if it ever isn't.
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ft710: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz: m.FreqHz,
			// Rendered through THIS session's dialect, not cat.Mode.String:
			// this string is user-visible (it lands in the codeplug, the
			// CLI's channel listing and the GUI's grid), so it must be the
			// mode table of the radio that answered, never the FT-710's
			// table on some other radio's behalf. ModeName gives the
			// display name (e.g. "USB"); for the odd-state ModeUnset it is
			// "-", mapped through faithfully — codeplug.Validate flags it
			// as not a selectable mode. Mode.String survives as a
			// dialect-free diagnostic fallback only (see its doc comment).
			Mode:      s.dialect.ModeName(m.Mode),
			ClarHz:    int(m.ClarHz),
			RxClar:    m.RxClar,
			TxClar:    m.TxClar,
			CTCSS:     ctcss,
			CTCSSTone: codeplug.ToneField{State: codeplug.Unknown}, // unreadable via CAT — see doc comment
			Shift:     shift,
			Tag:       tag,
			// Known, unlike the two Unknown fields below: the MT answer this
			// channel was built from CARRIES the display flag (P1), so it was
			// genuinely read from the radio rather than assumed.
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: display},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown}, // unreadable via CAT — see doc comment
		},
	}, nil
}

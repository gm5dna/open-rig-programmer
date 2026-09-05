// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// readChannelGapHook, when non-nil, is called by ReadChannel between the MT
// read's rejection and the cross-check's MR read — a test-only seam,
// mirroring core/driver/ft710's namesake and core/transport's openPort
// precedent. Always nil in production, so it costs one nil check per
// rejected MT read and nothing else.
//
// It exists for TestReadChannel_CrossCheckIsAtomicUnderOpMu, to force a
// concurrent operation deterministically into the gap the operation mutex
// exists to close, rather than relying on goroutine scheduling to hit it:
// Go's sync.Mutex favours an immediately-re-locking goroutine so heavily
// that this interleaving is near-impossible to reproduce by hammering
// alone, which is the finding the FT-710's own gap hook records.
var readChannelGapHook func()

// ctcssNames maps the wire CTCSS state to codeplug's display spelling
// ("OFF", "ENC-DEC", "ENC" — the strings codeplug.Validate checks for, and
// the ones this driver's own Capabilities.CTCSSStates advertises).
// Deliberately NOT cat.CTCSSState.String(), whose spellings ("off",
// "ENC/DEC") are log labels rather than model values.
//
// Three states and no more: this radio's P8 legend prints exactly `0: CTCSS
// "OFF" 1: CTCSS ENC/DEC 2: CTCSS ENC` on all five blocks that carry it (MR
// 977, MT 1012, MW 1048, IF 790, OI 1136), and CT's fourth value — `3: DCS
// "ON"` (414) — is LIVE STATE on a different command, not a memory field
// (matrix §1.17).
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:    "OFF",
	cat.CTCSSEncDec: "ENC-DEC",
	cat.CTCSSEnc:    "ENC",
}

// shiftNames maps the wire shift state to codeplug's display spelling
// ("SIMPLEX", "PLUS", "MINUS"), from this radio's own P10 legend "0: Simplex
// 1: Plus Shift 2: Minus Shift", printed identically on MR (979), MT (1015),
// MW (1050), IF (792) and OI (1138) — matrix §1.16.
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// mtSpec builds the transport spec for a combined MT read against d: the
// prefix, the EXACT answer length, and NO RETRY.
//
// THE LENGTH IS DERIVED FROM THE DIALECT, never written here. d's own
// MTAnswerBounds reports the geometry its declared MT form and tag width
// imply — for this radio's combined form that is an exact 41, being
// 29 + TagMaxBytes — and there is deliberately no 41 in this package's
// production code at all. Two reasons, both load-bearing: a literal would
// silently keep answering 41 for a dialect whose tag field was a different
// width, and the combined answer's exactness is itself an ASSUMPTION the
// dialect carries (its register entry THE COMBINED MT ANSWER'S EXACT
// LENGTH, 41), whose recorded Stage R contingency is a 30..41 WINDOW. If
// that contingency is ever taken, the bounds move in core/cat and this spec
// moves with them.
//
// The equal-bounds check is what makes that safe rather than lucky:
// transport.CATReadSpec takes a single exact length, so a dialect reporting
// a genuine window has no honest exact length and gets an error instead of
// its window's top silently becoming a hard requirement (which would reject
// every shorter answer as unmatched, i.e. as a timeout).
//
// RETRYREADS IS 0, AND THAT IS A DECISION (plan P11). Every sibling's read
// spec carries one retry on the reasoning that a read is idempotent and a
// single swallowed reply should not fail an operation — which is true here
// too. What is different is the premise: this radio's Control Command List
// says MT has NO READ AT ALL (layout 166) while its detail block prints one
// (1016), and the driver register's MT READ IS SUPPORTED FOR MEMORY AND PMS
// entry is exactly that unresolved question. Retrying would double the MT
// frames sent to a radio that may be answering nothing on purpose, and it
// would make a timeout's transcript two frames where the design says one. A
// timeout is therefore ONE MT frame and then MTReadTimeoutError.
func mtSpec(d cat.Dialect) (transport.CommandSpec, error) {
	lo, hi, err := d.MTAnswerBounds()
	if err != nil {
		return transport.CommandSpec{}, fmt.Errorf("ft891: MT answer geometry: %w", err)
	}
	if lo != hi {
		return transport.CommandSpec{}, fmt.Errorf("ft891: MT answer geometry: this dialect reports a %d..%d length WINDOW, but transport.CATReadSpec takes a single exact length — a windowed answer needs a spec that expresses the window, not its top", lo, hi)
	}
	return transport.CATReadSpec("MT", hi, 0), nil
}

// mrAnswerLen is the length of this radio's MR answer: 28 bytes, "MR" + the
// 28-position field block's own 25 bytes at offsets 2-26 + ';'.
//
// WRITTEN DOWN HERE RATHER THAN DERIVED, because core/cat exposes no
// accessor for the shared block's width — core/driver/ft710 carries the same
// literal for the same reason, and the combined MT read's length above is
// derived precisely because the dialect DOES expose that one. The authority
// is this radio's own MR Answer chart, which runs to 28 (layout 968-975)
// and matches core/cat/memdata.go's field block position for position — the
// reused-command verification verdict in core/cat/ft891/doc.go — and the
// frame-geometry witness, which counted "2 MR Answer frames (28 bytes)"
// independently (core/cat/ft891/testdata/provenance.md §MR).
const mrAnswerLen = 28

// mrSpec builds the transport spec for an MR read: the prefix, the fixed
// 28-byte answer, and ONE retry.
//
// One retry, where the MT read's spec takes none: nothing about MR is in
// doubt on this radio. Its availability row is `MR | MEMORY CHANNEL READ |
// X O O X` (layout 164), no other record contradicts it, and a read is
// idempotent — so the ordinary reasoning (a single swallowed reply must not
// fail an operation) applies unchanged. It is the MT READ whose very
// existence the manual disagrees with itself about, and only that spec
// declines the retry (see mtSpec).
func mrSpec() transport.CommandSpec {
	return transport.CATReadSpec("MR", mrAnswerLen, 1)
}

// ErrMTReadRejectedForOccupiedSlot is the sentinel a caller compares
// against (via errors.Is) when this radio answered "?;" to an MT read of a
// slot whose own MR read returns a record. The error actually returned is
// an *MTReadRejectedForOccupiedSlotError naming the slot.
var ErrMTReadRejectedForOccupiedSlot = errors.New("ft891: MT read rejected for a slot that MR reports as occupied")

// MTReadRejectedForOccupiedSlotError reports the third row of the read
// truth table (matrix §3.5): MT answered "?;" and the cross-check's MR read
// of the same slot returned a well-formed record, so the slot is OCCUPIED
// and MT refused it.
//
// IT NAMES A CONTRADICTION; IT DOES NOT DIAGNOSE ONE. "?;" is the
// protocol's single unattributed NAK and carries no reason code, so three
// readings stay consistent with the manual and this project cannot tell
// them apart: (a) the Control Command List is right and MT genuinely has no
// Read on this radio, (b) MT has a Read but refused this particular slot,
// or (c) something transient. The driver refuses loudly rather than picking
// one.
//
// THE SESSION READ FAILS WHOLE, not per-slot, and that is the design's
// point: a partial read that silently dropped occupied channels would be a
// codeplug the user could not tell from a complete one. core/clone's
// ReadAll propagates the first ReadChannel error, so returning it here is
// what makes the whole read fail.
//
// It unwraps to BOTH its own sentinel and cat.ErrRejected: the radio's
// answer really was a rejection, so a caller handling rejections generically
// must still see one, while a caller that knows about this radio's
// contradiction can match the specific case.
//
// A degraded MR-only read path for memory and PMS is an explicit NON-GOAL
// of this milestone: this refusal is the honest placeholder, and the driver
// register's MT READ IS SUPPORTED FOR MEMORY AND PMS entry names the one
// capture that would turn it into a finding.
type MTReadRejectedForOccupiedSlotError struct {
	// Slot is the canonical wire-form slot whose read was refused.
	Slot string
}

// Error implements the error interface. The text names the slot, both of
// the manual's disagreeing records, and the capture that settles them,
// because a user whose read fails this way is entitled to know that the
// manual — not this software — is where the ambiguity started.
func (e *MTReadRejectedForOccupiedSlotError) Error() string {
	return fmt.Sprintf("ft891: MT read of slot %q was rejected (\"?;\") but an MR read of the same slot returned a record, so the slot is occupied and MT refused it — this radio's manual contradicts itself about whether MT can be read at all (its Control Command List gives MT Set only, layout 166, while MT's own detail block prints a Read chart at layout 1016 and a full 41-position Answer chart) and \"?;\" carries no reason code, so this driver names the contradiction rather than diagnosing it; the whole session read fails, because a partial read that silently dropped occupied channels would be a codeplug you could not tell from a complete one. ONE MT read of a known-populated memory channel on a real FT-891 settles it — the driver register's \"MT READ IS SUPPORTED FOR MEMORY AND PMS\" entry", e.Slot)
}

// Unwrap exposes both meanings: the specific case (this package's own
// sentinel) and the general one (the radio rejected a frame).
func (e *MTReadRejectedForOccupiedSlotError) Unwrap() []error {
	return []error{ErrMTReadRejectedForOccupiedSlot, cat.ErrRejected}
}

// ErrMTReadTimeout is the sentinel a caller compares against (via
// errors.Is) when an MT read drew no answer at all. The error actually
// returned is an *MTReadTimeoutError naming the slot and carrying the
// transport's own timeout error.
var ErrMTReadTimeout = errors.New("ft891: MT read timed out")

// MTReadTimeoutError reports the fourth row of the read truth table (matrix
// §3.5): the combined MT read of a memory or PMS slot drew NO ANSWER, so
// the session read fails whole.
//
// IT IS THE SAME TYPE FAMILY as MTReadRejectedForOccupiedSlotError, and the
// family is a deliberate shape rather than a coincidence: both are typed,
// both name the slot, both fail the whole session read, and both exist
// because this radio's MT read is the one frame whose very availability is
// in question. A caller can therefore tell "the radio refused" from "the
// radio said nothing" while treating both as the same class of failure.
//
// NO RETRY PRECEDES IT (plan P11, mtSpec's RetryReads 0) and NO MR FOLLOWS
// IT: the cross-check is the answer to a REJECTION, not to silence. A
// timeout says nothing about whether the slot is occupied, so asking MR
// would be interpreting a transport failure as a protocol answer.
type MTReadTimeoutError struct {
	// Slot is the canonical wire-form slot whose read timed out.
	Slot string
	// Err is the transport's own error, kept so errors.Is reaches
	// transport.ErrTimeout and so a log can carry the transport's wording.
	Err error
}

// Error implements the error interface.
func (e *MTReadTimeoutError) Error() string {
	return fmt.Sprintf("ft891: MT read of slot %q drew no answer: %v — the whole session read fails, with no retry and no MR cross-check (a cross-check answers a rejection, not silence); whether this radio answers MT reads at all is the driver register's \"MT READ IS SUPPORTED FOR MEMORY AND PMS\" entry", e.Slot, e.Err)
}

// Unwrap exposes both meanings: this package's own sentinel and the
// transport's error (so errors.Is(err, transport.ErrTimeout) matches).
func (e *MTReadTimeoutError) Unwrap() []error {
	return []error{ErrMTReadTimeout, e.Err}
}

// ErrMRReadRejectedForDiscoveredSlot is the sentinel a caller compares
// against (via errors.Is) when a 5xx or EMG slot that answered a
// well-formed MR read during this session's Open rejects the identical MR
// read later, at ReadChannel time. The error actually returned is an
// *MRReadRejectedForDiscoveredSlotError naming the slot.
var ErrMRReadRejectedForDiscoveredSlot = errors.New("ft891: MR read rejected for a discovered slot that answered at Open")

// MRReadRejectedForDiscoveredSlotError reports a discovered-slot "?;" at
// ReadChannel time (matrix erratum M-E6, §3.8.4; the driver register's A
// DISCOVERED SLOT KEEPS ANSWERING MR WITHIN A SESSION entry).
//
// THE SLOT ANSWERED THIS EXACT FRAME DURING OPEN — that is why it is in
// this bank at all — and now refuses it. That is a DIFFERENT proposition
// from the "?;" ON A 5xx/EMG DISCOVERY PROBE MEANS ABSENT FROM THIS RADIO
// entry, which governs the FIRST read of a slot, at the moment discovery
// decides bank MEMBERSHIP: by ReadChannel time membership is already
// settled, and reading the same rejection as CHANNEL STATE would be a
// fourth, unregistered interpretation of "?;" where matrix §3.8 draws only
// three.
//
// IT NAMES THE CONTRADICTION; IT DOES NOT DIAGNOSE IT — same posture as
// MTReadRejectedForOccupiedSlotError, and for the same reason: "?;" carries
// no reason code, so nothing here is assumed about WHY the radio stopped
// answering.
//
// THE SESSION READ FAILS WHOLE, not per-slot: this bank's own capabilities
// publish NoBlank true (effectiveCapabilities' "these channels exist
// because they answered a read"), so reporting an empty channel here would
// silently contradict that claim and surface later as a codeplug.Validate
// error naming the codeplug rather than the radio — the same failure mode
// MTReadRejectedForOccupiedSlotError's doc comment states in terms.
//
// It unwraps to both its own sentinel and cat.ErrRejected: the radio's
// answer really was a rejection, so a caller handling rejections
// generically must still see one, while a caller that knows about this
// specific contradiction can match it.
type MRReadRejectedForDiscoveredSlotError struct {
	// Slot is the canonical wire-form slot whose read was refused.
	Slot string
}

// Error implements the error interface. The text names the slot, the
// contradiction (this slot answered the identical MR read at Open and
// refuses it now), and the ONE capture that would settle it.
func (e *MRReadRejectedForDiscoveredSlotError) Error() string {
	return fmt.Sprintf("ft891: MR read of discovered slot %q was rejected (\"?;\") but this radio answered the identical MR read during this session's Open — the bank exists only because that earlier read succeeded, and a rejection now means the radio has stopped reporting a channel it reported minutes earlier; the whole session read fails, because this bank's own capabilities declare it NoBlank and an empty channel here would silently contradict that and surface later as a codeplug validation error naming the codeplug rather than the radio. A second MR read of the same slot within one session on a real FT-891 is the one capture that would settle it — the driver register's \"A DISCOVERED SLOT KEEPS ANSWERING MR WITHIN A SESSION\" entry", e.Slot)
}

// Unwrap exposes both meanings: this package's own sentinel and the general
// one (the radio rejected a frame).
func (e *MRReadRejectedForDiscoveredSlotError) Unwrap() []error {
	return []error{ErrMRReadRejectedForDiscoveredSlot, cat.ErrRejected}
}

// ErrSlotNotInSessionBanks is the sentinel a caller compares against (via
// errors.Is) when ReadChannel is asked to read a syntactically valid 5xx or
// EMG slot that did not answer this SESSION's own discovery walk at Open —
// it is in neither the 60M nor the EMG bank effectiveCapabilities (caps.go)
// published for this session. The error actually returned is a
// *SlotNotInSessionBanksError naming the slot.
var ErrSlotNotInSessionBanks = errors.New("ft891: slot did not answer this session's discovery walk")

// SlotNotInSessionBanksError reports that ReadChannel was asked for a 5xx or
// EMG slot outside this session's own discovered banks (C-H1, closing review
// wave 2).
//
// NO FRAME IS SENT: this check runs BEFORE readDiscovered builds an MR read
// at all, on membership Open already settled by its own discovery walk
// (ft891.go's discoverInventory/probeSlot, matrix §3.4) — the radio is never
// asked. That is what makes this a DIFFERENT error from
// *MRReadRejectedForDiscoveredSlotError, not a narrower spelling of it:
// readDiscovered's whole premise, the driver register's A DISCOVERED SLOT
// KEEPS ANSWERING MR WITHIN A SESSION entry, is that the slot ANSWERED THE
// IDENTICAL MR READ AT OPEN — which a slot outside these banks never did.
// Dispatching such a slot to readDiscovered anyway would send a fresh
// "MR501;" and, on the radio's ordinary "?;" for an absent slot, report
// *MRReadRejectedForDiscoveredSlotError — falsely claiming the slot answered
// at Open, in flat contradiction of the very capabilities
// (effectiveCapabilities' NoBlank banks) this session published. Before this
// fix ReadChannel dispatched on sl.Is60m()/sl.IsEMG() alone, with no
// membership check at all — the RED-PROOF is recorded on
// TestReadChannel_DiscoveredSlotOutsideSessionBanksIsRefusedLocally.
//
// NOT cat.ErrRejected: no "?;" was received, because nothing was sent.
type SlotNotInSessionBanksError struct {
	// Slot is the canonical wire-form slot that was requested.
	Slot string
}

// Error implements the error interface.
func (e *SlotNotInSessionBanksError) Error() string {
	return fmt.Sprintf("ft891: ReadChannel: slot %q did not answer during this session's discovery, so no read is sent", e.Slot)
}

// Unwrap exposes this package's own sentinel.
func (e *SlotNotInSessionBanksError) Unwrap() error {
	return ErrSlotNotInSessionBanks
}

// ReadChannel implements driver.Session: matrix §3.5's truth table, exactly.
//
//   - MEM and PMS are read by ONE combined 41-byte MT read. A well-formed
//     answer carries the field block, the LIVE display flag and the tag
//     together, so it is an ATOMIC snapshot: the two-frame stitch the
//     FT-710's MR+MT read has to guard against (field block from one radio
//     state, tag from a later one) is structurally impossible.
//   - MT "?;" costs ONE MR read of the same slot — the CROSS-CHECK. MR "?;"
//     too means the slot is EMPTY (Data nil, the slot carried through, no
//     error); an MR RECORD means the slot is occupied and MT refused it,
//     which is *MTReadRejectedForOccupiedSlotError and fails the whole
//     session read.
//   - An MT TIMEOUT is *MTReadTimeoutError: one frame, no retry, no MR.
//   - 60M and EMG are read by ONE MR read and NEVER an MT read, with Tag
//     and TagDisplay Unavailable, because MR's 28-position answer carries
//     neither (matrix §2.5).
//
// THE WHOLE OPERATION IS HELD UNDER opMu (plan P3, spec S-E4, matrix M-E2).
// transport.Engine serialises each individual exchange, not a pair, so
// without the lock a concurrent operation could land between the MT "?;"
// and the MR that interprets it and the cross-check would be reasoning
// about a different radio state.
//
// TagDisplay comes back KNOWN on MEM and PMS, with the flag byte 28
// carried, and this radio is the first of the family for which that is
// true: MT's P11 legend reads `0: TAG "OFF" 1: TAG "ON"` (layout 1016)
// where its combined-form siblings print "0: (Fixed)" and their drivers
// report Unavailable. That the radio really REPORTS the byte rather than
// always answering '0' is the register's READ-BACK OF THE TAG DISPLAY FLAG
// entry.
//
// TxClar can never come back true: under the dialect's MemoryP5 =
// cat.P5Fixed, core/cat's parser REQUIRES '0' at byte 21 and returns
// TxClar false (matrix §2.2). The strictness has its own register entry,
// P5 IS ANSWERED '0' — if this radio answers something else, every read
// refuses, and that is a finding rather than a tweak.
//
// CTCSSTone and ScanSkip come back codeplug.Unknown, ALWAYS: the register's
// TONE AND SCAN-SKIP UNREACHABILITY entry. "Unknown" means "preserve
// whatever the radio has" to every write path downstream, which is the only
// honest instruction for a field this driver cannot see.
//
// KIND CHECKING IS THE PARSER'S, not this driver's. core/cat narrows P7 —
// the combined answer's to the tolerated read pair {'0' VFO, '1' Memory}
// (the DIALECT register's THE COMBINED ANSWER'S P7 READ DOMAIN entry), the
// 28-byte block's to the documented '0'-'5' — so an out-of-vocabulary byte
// comes back as a *cat.ParseError, wrapped here with the slot. No
// per-class narrowing is added on top, and this manual's SIX-value IF P7
// (layout 789) and SEVEN-value OI P7 (1134-1135) are deliberately not read
// across into the memory record's: see doc.go.
//
// Error typing, three classes: PARSE failures stay *cat.ParseError under a
// wrap (errors.As finds them, and the wrap adds the slot the bare parser
// could not know); the SLOT-ECHO check raises *AnswerMismatchError; and the
// two MT-read refusals above are this driver's own family. None is a bare
// fmt.Errorf. A FOURTH, pre-wire refusal sits ahead of all of them for a 5xx
// or EMG slot: *SlotNotInSessionBanksError, when the slot never answered
// this session's own discovery walk (C-H1) — no frame is sent for it at all,
// so it is not one of the three "?;" interpretations above and carries none
// of their premises.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	// Held for the WHOLE operation, cross-check included — see the doc
	// comment and the Session type's.
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel: %w", err)
	}

	if sl.Is60m() || sl.IsEMG() {
		// SESSION MEMBERSHIP, before any frame is built (C-H1): a slot is
		// dispatched to readDiscovered only if it is one of THIS session's
		// own discovered banks (s.bankFor — write.go's, walked over the
		// same s.caps.Banks readDiscovered's own doc comment cites). A 5xx
		// or EMG slot outside them never answered at Open, so readDiscovered
		// would be sending a read whose premise — "this exact frame
		// answered during Open" — cannot hold; see
		// *SlotNotInSessionBanksError's doc comment for the mistake this
		// closes and why it is not the same mistake
		// *MRReadRejectedForDiscoveredSlotError exists to name.
		if _, ok := s.bankFor(sl.Wire()); !ok {
			return codeplug.Channel{}, &SlotNotInSessionBanksError{Slot: sl.Wire()}
		}
		return s.readDiscovered(ctx, sl)
	}
	return s.readMemoryOrPMS(ctx, sl)
}

// readMemoryOrPMS is the combined-MT read with its per-slot MR cross-check
// — the first four rows of the truth table. Called with opMu held.
func (s *Session) readMemoryOrPMS(ctx context.Context, sl cat.Slot) (codeplug.Channel, error) {
	cmd, err := s.dialect.BuildMTRead(sl)
	if err != nil {
		// e.g. the answer-only none form — the DIALECT register's entry
		// SlotSpace.NoneWire = "000", ASSUMED because that form appears in
		// no FT-891 slot legend: grammatical per ParseSlot, never a legal
		// read target. (A 5xx or EMG slot never reaches here: it is
		// dispatched to the MR-only path above, and this dialect would
		// refuse the frame anyway.)
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel: %w", err)
	}
	cmdSpec, err := mtSpec(s.dialect)
	if err != nil {
		return codeplug.Channel{}, err
	}

	frame, err := s.eng.Do(ctx, cmd, cmdSpec)
	switch {
	case errors.Is(err, cat.ErrRejected):
		return s.crossCheck(ctx, sl)
	case errors.Is(err, transport.ErrTimeout):
		// One MT frame has gone out and nothing came back. No retry
		// (mtSpec), and no MR: see MTReadTimeoutError.
		return codeplug.Channel{}, &MTReadTimeoutError{Slot: sl.Wire(), Err: err}
	case err != nil:
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel %s: MT: %w", sl.Wire(), err)
	}

	m, tag, display, err := s.dialect.ParseMTAnswerCombinedDisplay(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}

	data, err := s.channelData(m, sl)
	if err != nil {
		return codeplug.Channel{}, err
	}
	data.Tag = tag
	// KNOWN, not Unavailable: byte 28 is a live TAG flag on this radio and
	// this answer carried it. See the method doc comment.
	data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: display}
	return codeplug.Channel{Slot: sl.Wire(), Data: data}, nil
}

// crossCheck answers the question an MT "?;" leaves open: is the slot empty,
// or did MT refuse a slot that is actually occupied? ONE MR read decides
// (matrix §3.5, §3.8.2). Called with opMu held, from readMemoryOrPMS only.
func (s *Session) crossCheck(ctx context.Context, sl cat.Slot) (codeplug.Channel, error) {
	if readChannelGapHook != nil {
		readChannelGapHook()
	}

	cmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel %s: cross-check: %w", sl.Wire(), err)
	}
	frame, err := s.eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		// ASSUMED EMPTY — the register's "?;" ON AN MR READ OF A MEMORY OR
		// PMS SLOT MEANS THE SLOT IS EMPTY entry. This is the only site in
		// this driver that reads an MR rejection of a memory or PMS slot,
		// because it is the only place such a frame is ever sent.
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel %s: cross-check MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel %s: cross-check: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}
	// A record came back: the slot is occupied and MT refused it. Refuse
	// the whole session read rather than reporting either a blank channel
	// or a diagnosis this manual does not support.
	return codeplug.Channel{}, &MTReadRejectedForOccupiedSlotError{Slot: sl.Wire()}
}

// readDiscovered is the truth table's fifth row: a 5xx or EMG slot, read by
// ONE MR frame and never an MT one. Called with opMu held.
//
// The MT read is not merely avoided here, it is UNBUILDABLE: MT's own slot
// legend prints memory and PMS only (layout 998-999) where MR's prints all
// four classes (960-964), so under MTPolicy.ReadSlots = cat.MTReadsMemoryPMS
// the codec and the outbound gate both refuse an "MT501;". This function is
// what makes that refusal never fire in ordinary use, and
// TestOpen_NeverBuildsAnMTReadOfADiscoveredSlot is the negative pin.
//
// A "?;" HERE IS A TYPED REFUSAL, not an empty channel (matrix erratum
// M-E6, §3.8.4, from the task-1 review). The register's "?;" ON A 5xx/EMG
// DISCOVERY PROBE MEANS ABSENT FROM THIS RADIO entry governs the FIRST MR
// read of a slot, at Open, where the question is bank MEMBERSHIP; by
// ReadChannel time membership is already settled by that earlier answer,
// and reading the SAME rejection as CHANNEL STATE would be a fourth,
// unregistered interpretation of "?;" where matrix §3.8 draws only three.
// It also contradicts this bank's own capabilities: effectiveCapabilities
// (caps.go) publishes the discovered bank NoBlank TRUE — "these channels
// exist because they answered a read" — so a slot that answered at Open
// and now refuses is exactly a slot for which that premise has failed, and
// reporting the one channel shape NoBlank declares impossible would not
// suppress the anomaly, it would defer and misattribute it to a LATER
// codeplug.Validate error naming the codeplug rather than the radio. See
// *MRReadRejectedForDiscoveredSlotError and the driver register's A
// DISCOVERED SLOT KEEPS ANSWERING MR WITHIN A SESSION entry.
func (s *Session) readDiscovered(ctx context.Context, sl cat.Slot) (codeplug.Channel, error) {
	cmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		// The none form again — see readMemoryOrPMS. BuildMRRead refuses
		// it for the same reason (the DIALECT register's
		// SlotSpace.NoneWire = "000" entry).
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel: %w", err)
	}
	frame, err := s.eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		return codeplug.Channel{}, &MRReadRejectedForDiscoveredSlotError{Slot: sl.Wire()}
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft891: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}

	data, err := s.channelData(m, sl)
	if err != nil {
		return codeplug.Channel{}, err
	}
	// UNAVAILABLE, and the Tag string stays empty: MR's 28-position answer
	// carries neither a tag field nor a display flag (layout 968-975), so
	// there is no value to report and Unknown would be the wrong word —
	// Unknown means "the radio has one and this read did not learn it".
	// The capability half agrees: readOnlyFields (caps.go) gives both
	// fields the ZERO FieldSupport on these banks, so Diff excludes them,
	// the grid will not offer them and csvio spells them Unavailable.
	//
	// THE HONEST READING IS "this driver's read of this bank cannot reach
	// the field", NOT "this radio has no such field" (matrix §2.5).
	data.Tag = ""
	data.TagDisplay = codeplug.BoolField{State: codeplug.Unavailable}
	return codeplug.Channel{Slot: sl.Wire(), Data: data}, nil
}

// channelData maps the shared 28-position field block — the part MR's
// answer and the combined MT answer carry identically — into a
// codeplug.ChannelData, leaving Tag and TagDisplay to the caller, which is
// the ONLY part of the mapping the two frames differ about.
//
// BOTH READ PATHS COME THROUGH HERE, which is what makes plan P12's
// requirement structural rather than duplicated: all seventeen tier fields
// are set Unavailable in one place, so the fleet's no-Absent pin holds and a
// fresh FT-891 read saves as schema 3 whichever bank it came from.
func (s *Session) channelData(m cat.MemoryData, sl cat.Slot) (*codeplug.ChannelData, error) {
	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		// Unreachable after core/cat's own CTCSS validation; refuse rather
		// than silently mislabel if it ever is not.
		return nil, fmt.Errorf("ft891: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return nil, fmt.Errorf("ft891: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	return &codeplug.ChannelData{
		// uint64 since the Icom tier widened the neutral model (design D4):
		// a widening conversion from this protocol's uint32, which can
		// never lose anything. The narrowing direction — the write path —
		// is the checked one (cat.MemoryFreqHz).
		FreqHz: uint64(m.FreqHz),
		// Rendered through THIS SESSION'S dialect, not cat.Mode.String: the
		// string is user-visible (it lands in the codeplug, the CLI listing
		// and the GUI grid), so it must come from the mode table of the
		// radio that answered — and on this radio that matters as it does
		// on no sibling, because SIX of its twelve names disagree with the
		// FTdx10's at the same nibble. ModeName gives the display name; the
		// odd-state cat.ModeUnset renders "-" and is mapped through
		// faithfully (codeplug.Validate flags it as not a selectable mode,
		// which is the right outcome for a placeholder this radio's legends
		// do not list — the DIALECT register's THE cat.ModeUnset MEMBER OF
		// THE MODE TABLE entry). That the parser refuses any P6 nibble
		// outside the transcribed 1-9/B-D table is the DRIVER register's
		// THE MODE NIBBLE'S TOP END entry: this manual's three mode legends
		// print no member above 'D', but a legend is a statement about what
		// the chart draws, not a guarantee about what the radio will ever
		// send.
		Mode: s.dialect.ModeName(m.Mode),
		// The magnitude is manual-evidenced (P3's four digits at positions
		// 16-19); the byte carrying a NEGATIVE sign is not — the DIALECT
		// register's THE CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII
		// HYPHEN-MINUS 0x2D entry, which this manual prints as a single
		// hyphen on all five blocks but which pdftotext would have
		// flattened an en dash into without trace. core/cat accepts only
		// '+' or '-' when reading the sign, so a radio using another byte
		// fails the parse loudly here rather than silently reading a
		// negative offset as positive.
		ClarHz: int(m.ClarHz),
		RxClar: m.RxClar,
		// Always false, and structurally so: under this dialect's P5Fixed
		// the parser REQUIRES '0' at byte 21 (matrix §2.2). Carried through
		// from the parser rather than hard-coded, so that a change to that
		// policy shows up here rather than being masked.
		TxClar: m.TxClar,
		CTCSS:  ctcss,
		// The register's TONE AND SCAN-SKIP UNREACHABILITY entry: no tone
		// number is readable.
		CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
		Shift:     shift,
		// The register's TONE AND SCAN-SKIP UNREACHABILITY entry: no
		// scan-skip flag is readable.
		ScanSkip: codeplug.BoolField{State: codeplug.Unknown},
		// The seventeen fields the Icom model extensions added to the
		// neutral memory model (design D4/D8). UNAVAILABLE on this radio:
		// this family's memory frame carries none of them, so there is no
		// value to read and no question for the user. The matching half is
		// in caps.go, where this radio's banks grade every one of them the
		// zero FieldSupport.
		//
		// Not Absent (the zero FieldState), deliberately. Absent means
		// "this codeplug never spoke about the field", which is true of a
		// schema-3 FILE and false of a RADIO READ: a read of this radio is
		// a positive statement that the frame has no such field, and
		// Unavailable is the state that says so.
		TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:              codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
		ToneMode:            codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}, nil
}

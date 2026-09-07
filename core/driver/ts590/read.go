// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// readChannelGapHook, when non-nil, is called by ReadChannel with opMu
// already held and before any frame is built — a test-only seam, mirroring
// core/driver/ft891's and core/driver/ft710's namesakes. Always nil in
// production, so it costs one nil check per read.
//
// It exists for TestReadChannel_IsAtomicUnderOpMu, to park one operation
// INSIDE the lock deterministically rather than relying on goroutine
// scheduling: Go's sync.Mutex favours an immediately-re-locking goroutine so
// heavily that the interleaving opMu exists to forbid is near-impossible to
// reproduce by hammering alone.
var readChannelGapHook func()

// toneModeNames maps the record's P7 wire byte to the neutral tone_mode
// vocabulary this row publishes (matrix §1.19).
//
// FOUR VALUES, and the fourth is what makes the two tone indices'
// independence expressible: "0: TONE/CTCSS OFF, 1: TONE ON, 2: CTCSS ON,
// 3: Cross Tone ON" (590:1549-1553). The strings are Capabilities.ToneModes'
// own, so a Known value this map produces is one codeplug.StringField.Valid
// admits.
//
// core/kw's Layout.ValidToneMode has already refused any byte outside THIS
// ROW's printed legend before a Record exists, so the map's miss branch is
// unreachable in practice; it refuses rather than mislabels if it ever is not.
var toneModeNames = map[kw.ToneMode]string{
	kw.ToneModeOff:   "OFF",
	kw.ToneModeTone:  "TONE",
	kw.ToneModeCTCSS: "CTCSS",
	kw.ToneModeCross: "CROSS",
}

// filterLabels maps byte 28 to the two printed FILTER labels
// (590:1560-1563). Consulted only on the SG row — see channelData.
var filterLabels = map[byte]string{
	'0': filterALabel,
	'1': filterBLabel,
}

// ErrUnknownSlot is the sentinel a caller compares against (via errors.Is)
// when a slot identifier is not one this ROW publishes. The error actually
// returned is an *UnknownSlotError.
var ErrUnknownSlot = errors.New("ts590: slot is not one this row publishes")

// UnknownSlotError reports a slot identifier that is not in any bank of this
// row — whether because it is malformed, because it names a channel outside
// the row's printed space, or because it names one of the ten the driver
// deliberately does not publish.
//
// NO FRAME IS SENT. The check is bank MEMBERSHIP, settled entirely from this
// session's own published capabilities, so the radio is never asked.
//
// THE TS-590SG'S EXTENSION CHANNELS 110-119 ARE REFUSED HERE, AND BY EXACTLY
// THIS MECHANISM — Stuart decisions row 6, RULED 05/09/2026. The distinction
// that makes that honest is between the CODEC'S SLOT DOMAIN and the DRIVER'S
// PUBLISHED BANKS, which are two different questions. The book prints
// 110-119 for the SG (590:1346-1347), so core/kw/ts590's SG layout DECLARES
// them and an "MC115;" answer or an "MR0115;" from a front-panel recall
// parses rather than being refused as malformed (A12's counterpart). What an
// extension channel IS is never explained anywhere in the book (A11), so the
// ten slots are not published as memories until A11 lifts — and a read or a
// write naming one is therefore refused THE SAME SHAPE as any other slot this
// row does not publish, with no invented radio behaviour and no special case
// in the code. TestReadChannel_TheSGExtensionChannelsAreNotSlotIDs pins that
// "110", "115" and "119" take this branch on the SG exactly as "999" does on
// both rows.
type UnknownSlotError struct {
	// Slot is the identifier that was requested.
	Slot string
	// Model is the row it was requested of.
	Model string
	// Reason says which of the three ways it failed.
	Reason string
}

// Error implements the error interface.
func (e *UnknownSlotError) Error() string {
	return fmt.Sprintf("ts590: slot %q is not published by the %s: %s", e.Slot, e.Model, e.Reason)
}

// Unwrap lets errors.Is(err, ErrUnknownSlot) match.
func (e *UnknownSlotError) Unwrap() error { return ErrUnknownSlot }

// ErrAnswerMismatch is the sentinel a caller compares against (via errors.Is)
// when a slot-addressed answer names a DIFFERENT channel than the one just
// requested. The error actually returned is an *AnswerMismatchError.
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports the requested and the answered slot; the
// shared form (driver.AnswerMismatchError) carries the model name so this
// package needs no typed error of its own.
//
// The transport's quarantine discipline makes this unlikely — a stale
// same-shape reply should have been drained — but the driver still refuses to
// map an answer onto the wrong slot rather than storing one channel's content
// under another's identifier.
type AnswerMismatchError = driver.AnswerMismatchError[string]

// AnswerP1MismatchError reports an MR answer whose P1 names a different half
// of the addressing than the read asked for, on a slot whose IDENTIFIER
// cannot carry that difference.
//
// IT IS THE MEM SLOT'S CASE, and it exists because Slot.String() is not a
// complete rendering of what was addressed. On a section channel P1 IS in the
// identifier ("100L" against "100U"), so AnswerMismatchError catches a wrong
// half there; on a MEM slot the string is three digits and P1 is discarded.
// An answer carrying P1='1' is the TRANSMIT side of a split channel
// (590:1519-1520), so accepting one for the P1='0' read this driver sends
// would store a transmit frequency as the channel's receive frequency and
// leave TxFreqHz Unavailable — silent loss that codeplug.Validate cannot see,
// which is what decision 11 and M-E2 exist to prevent.
//
// The comparison is the read-side twin of one core/kw's builder already makes
// (kw.BuildMWSet refuses a record whose AnswerP1 disagrees with its slot's
// class, 590:1529-1531). It is NOT reachable from this driver's own traffic
// today — plan P13 sends P1=1 only on a SCAN 'U' slot — so what it refuses is
// a radio, a cable or a stale frame contradicting the request; it becomes
// live in the driver's own choreography the day A9 lifts and P1=1 reads of
// ordinary memories begin. TestReadChannel_AnAnswerWhoseP1DisagreesIsRefusedOnAMEMSlot
// pins it on both rows.
//
// It shares AnswerMismatchError's sentinel because a caller asking "did this
// answer name what I asked for?" is asking one question, and the two types
// are what distinguish the two ways the answer can differ.
type AnswerP1MismatchError struct {
	// Slot is the identifier both the request and the answer named.
	Slot string
	// Requested is the P1 byte the read sent, derived from the slot's class.
	Requested byte
	// Answered is P1 exactly as the answer carried it.
	Answered byte
}

// Error implements the error interface.
func (e *AnswerP1MismatchError) Error() string {
	return fmt.Sprintf("ts590: slot %q was read with P1=%q but the answer carries P1=%q — on this radio P1=1 is the transmit side of a split channel (590:1519-1520), and storing it as the channel's frequency would lose the difference silently", e.Slot, e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerP1MismatchError) Unwrap() error { return ErrAnswerMismatch }

// parseSlotID splits a canonical slot identifier into the channel number and,
// for a section-defined channel, which of its two frequencies is meant.
//
// IT IS SYNTAX ONLY, and that is deliberate: it is the first rung of the
// plan's P7 ladder (ParseSlot, then bank membership, then the rest), and it
// must not consult a row. If it refused an out-of-space number itself, then
// "115" would be refused by two different mechanisms on the two rows — the
// codec's slot space on the S and the driver's published banks on the SG —
// and the refusal a user saw would depend on which sibling they own. Bank
// membership answers for both rows in one shape; see UnknownSlotError.
//
// THE WIRE FORM IS THE PROJECT'S CHOICE OVER THE RADIO'S PRINTED WIDTHS
// (matrix §1.4.1): three digits on these rows, because MC prints a hundreds
// digit and a two-digit remainder (590:1333), with an "L" or "U" suffix on a
// section-defined channel because ONE wire channel there holds TWO
// frequencies and a neutral channel holds one (§1.4.3). It is the exact form
// kw.Slot.String renders, which is what the bank inventories are built from,
// so the two cannot drift.
func parseSlotID(id string) (number int, half kw.ScanHalf, err error) {
	digits := id
	half = kw.ScanHalfNone
	if n := len(id); n == 4 {
		switch id[3] {
		case 'L':
			half = kw.ScanLower
		case 'U':
			half = kw.ScanUpper
		default:
			return 0, 0, fmt.Errorf("slot %q: a four-character identifier must end in \"L\" or \"U\", naming the START or the END frequency of a section-defined channel (590:1449-1451)", id)
		}
		digits = id[:3]
	}
	if len(digits) != 3 {
		return 0, 0, fmt.Errorf("slot %q: a slot identifier on this family is three digits, optionally followed by \"L\" or \"U\"", id)
	}
	for _, b := range []byte(digits) {
		if b < '0' || b > '9' {
			return 0, 0, fmt.Errorf("slot %q: the channel number is three decimal digits", id)
		}
	}
	return int(digits[0]-'0')*100 + int(digits[1]-'0')*10 + int(digits[2]-'0'), half, nil
}

// bankFor returns the bank of THIS SESSION's published capabilities that
// holds id, and whether any does.
//
// It walks the session's own effective set rather than a package-level table,
// so a slot is admitted only if the very capabilities this session handed its
// caller say it exists.
func (s *Session) bankFor(id string) (spec.Bank, bool) {
	for _, b := range s.caps.Banks {
		for _, slot := range b.Slots {
			if slot == id {
				return b, true
			}
		}
	}
	return spec.Bank{}, false
}

// bankNames renders this session's bank inventories for a refusal, so the
// message says what this radio publishes rather than what the family does.
func (s *Session) bankNames() string {
	parts := make([]string, 0, len(s.caps.Banks))
	for _, b := range s.caps.Banks {
		if len(b.Slots) == 0 {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s %s-%s", b.ID, b.Slots[0], b.Slots[len(b.Slots)-1]))
	}
	return strings.Join(parts, ", ")
}

// mrSpec is the transport spec for one MR read of slot: the codec's own MR
// answer matcher, and one retry.
//
// IT IS kw.Layout.MRAnswerMatcher AND NOT kw.PrefixLenMatcher, and the
// difference is the ANSWERED CHANNEL. Every memory answer this family sends
// is fifty bytes and starts "MR", with the channel number at P2/P3 rather
// than immediately after the command name, so no prefix a caller can spell
// separates one channel's answer from another's: under a prefix-and-length
// matcher a very late answer to a PREVIOUS read is delivered as this read's
// own. The driver would then refuse it — the *AnswerMismatchError below is
// exactly that defence — but refusing is the wrong verdict for a frame that
// was never this read's answer, and it costs the retry that would have
// succeeded. The codec's matcher skips it instead, and the read times out or
// its retry answers. Pinned by
// TestReadChannel_ALateAnswerIsNeverTheNextReadsAnswer.
//
// IT COMPARES THE SLOT AND NOT P1, which is the matcher's own division
// (core/kw/matcher.go): a section channel's wrong HALF is still correlated
// and still refused precisely, by *AnswerP1MismatchError or by the slot-string
// comparison, rather than disappearing into a timeout.
//
// THE LENGTH IS STILL core/kw's, inside that matcher rather than passed to
// it: a frame of any width but kw.RecordLen is not this read's answer at all,
// so a 49-byte "MR…" never reaches Layout.ParseMRAnswer and the read times
// out instead. That is the right order — the frame the radio sent is not the
// frame this command asked for — and the codec's own width predicate stays
// the authority on what a memory frame is (see
// TestReadChannel_AShortMRAnswerNeverReachesTheParser, which pins both
// halves).
//
// ONE RETRY, and it is the ordinary reasoning: a read is idempotent and a
// single swallowed reply should not fail a whole-radio read of 120 slots. The
// retry is preceded by the engine's own drain-to-quiet quarantine, so it
// cannot correlate a stale answer with the second attempt. What a retry does
// NOT do is turn silence into information: a read that still draws nothing
// fails the session read WHOLE, with the typed error kw.NewTimeoutError
// builds, because both books say the NAK is unreliable (590:106-108,
// 480:136-138) and a timeout is therefore neither "absent" nor "rejected".
func (s *Session) mrSpec(slot kw.Slot) transport.CommandSpec {
	return s.newReadSpec(s.layout.MRAnswerMatcher(slot), 1)
}

// ReadChannel implements driver.Session: ONE MR frame per slot, and nothing
// else (plan P13, matrix §3.8).
//
//   - MR P1=0 on every MEM slot, and on a SCAN slot's LOWER half — the
//     printed START frequency (590:1449-1451).
//   - MR P1=1 ONLY for a SCAN slot's UPPER half, the printed END frequency
//     (590:1449-1451, the MW half at 590:1529-1531). It is a DOCUMENTED read,
//     which is why A9 does not gate it: A9 is about P1=1 on an ORDINARY
//     memory channel, where what a simplex channel answers is unprinted.
//   - MC IS NEVER USED TO READ, and no discovery frame of any kind is ever
//     built (decision 5's two independent grounds; doc.go).
//   - P1 IS DERIVED FROM THE SLOT'S CLASS AND NEVER FROM A SPLIT STATE, which
//     is kw.Slot.P1's rule (M9) and this driver's only source for the byte.
//
// THE WHOLE OPERATION IS HELD UNDER opMu (P13/P14). transport.Engine
// serialises each individual exchange, not a whole driver operation, and this
// session's operations are not all single exchanges; holding the lock for
// every one of them, rather than only for the several-frame ones, is what
// makes the rule "one driver operation at a time" rather than "one driver
// operation at a time, except the short ones".
//
// FAILURE MODES, and the design's reading of each:
//
//   - An UNKNOWN SLOT is refused before any frame is built —
//     *UnknownSlotError, which is also how the TS-590SG's unpublished
//     extension channels are refused.
//   - A "?;" IS A DEFINITIVE REJECTION, NEVER "absent" (decision 5,
//     P13). The books print it as EITHER "Command syntax was incorrect" OR
//     "Command was not executed due to the current status of the transceiver"
//     (590:100-105) — indistinguishable — so it is reported as the typed
//     kw.RejectionError, which names both causes and the transient sentence,
//     and it fails the session read WHOLE.
//   - A TIMEOUT is the typed kw.TimeoutError, which says in as many words
//     that it is not an inference of absence. It fails the session read
//     whole too.
//   - AN EMPTY CHANNEL IS NOT A FAILURE AND IS NOT A REJECTION: an MR answer
//     whose P4-P15 are all zero is the documented empty channel of
//     590:1492-1493 — A18a, documentary FACT rather than assumption — and is
//     reported as codeplug.Channel{Slot: id} with a nil Data, which is the
//     driver seam's own spelling of "this slot is empty". The whole-range
//     test is core/kw's, applied before any field is interpreted, so a fresh
//     radio's zero mode nibble never reaches the mode legend.
//
// core/clone's ReadAll propagates the first ReadChannel error, so returning
// one here is what makes a whole-radio read fail whole rather than silently
// dropping a channel — a partial read the user could not tell from a complete
// one is the failure this ordering exists to prevent.
func (s *Session) ReadChannel(ctx context.Context, id string) (codeplug.Channel, error) {
	// Held for the WHOLE operation — see the doc comment and the Session
	// type's.
	s.opMu.Lock()
	defer s.opMu.Unlock()
	if readChannelGapHook != nil {
		readChannelGapHook()
	}

	number, half, err := parseSlotID(id)
	if err != nil {
		return codeplug.Channel{}, &UnknownSlotError{Slot: id, Model: modelNameFor(s.row), Reason: err.Error()}
	}
	bank, ok := s.bankFor(id)
	if !ok {
		return codeplug.Channel{}, &UnknownSlotError{
			Slot:   id,
			Model:  modelNameFor(s.row),
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}
	slot, err := s.layout.NewSlot(number, half)
	if err != nil {
		// Unreachable for a slot the banks published, since every published
		// identifier was rendered by this same layout's NewSlot (caps.go).
		// Refuse rather than build a frame from a slot the codec disowns.
		return codeplug.Channel{}, &UnknownSlotError{Slot: id, Model: modelNameFor(s.row), Reason: err.Error()}
	}

	cmd, err := s.layout.BuildMRRead(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts590: ReadChannel %s: %w", id, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.mrSpec(slot))
	if err != nil {
		// wireFailure is what types a "?;" and a timeout — see there, and
		// see this function's own doc comment for what each means on this
		// family. It is shared with the probe so one rule is applied once.
		return codeplug.Channel{}, fmt.Errorf("ts590: ReadChannel %s: %w", id, wireFailure(s.layout, "MR", err))
	}

	rec, err := s.layout.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts590: ReadChannel %s: %w", id, err)
	}
	if got := rec.Slot.String(); got != id {
		return codeplug.Channel{}, &AnswerMismatchError{Model: "ts590", Requested: id, Answered: got}
	}
	if want := slot.P1(); rec.AnswerP1 != want {
		// The half of the addressing the identifier does not carry — see
		// AnswerP1MismatchError. On a SCAN slot the check above has already
		// caught a wrong half, so this one is the MEM slot's.
		return codeplug.Channel{}, &AnswerP1MismatchError{Slot: id, Requested: want, Answered: rec.AnswerP1}
	}
	if rec.Empty {
		// A18a, and it is DOCUMENTARY FACT on these rows: "If the selected
		// channel is empty, P4 ~ P15 will be 0 and P16 will be blank"
		// (590:1492-1493). Data nil is the seam's "this slot is empty".
		return codeplug.Channel{Slot: id}, nil
	}

	data, err := s.channelData(rec, bank)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts590: ReadChannel %s: %w", id, err)
	}
	return codeplug.Channel{Slot: id, Data: data}, nil
}

// channelData maps one parsed 50-byte record onto the neutral channel model.
//
// ONE MAPPER FOR BOTH BANKS, which is what makes the seventeen Unavailable
// tier fields structural rather than duplicated: a fresh read of either bank
// leaves no tier field Absent, so it saves as the lowest schema and the
// fleet's no-Absent rule holds wherever the channel came from.
//
// The two per-context cells are the same two the capability table varies on,
// and they are read from the SAME PLACE as their capability grade — bank for
// one, row for the other — so a mapping and its grade cannot disagree:
//
//   - TxFreqHz is UNAVAILABLE, on both banks and always (matrix §2.4). In
//     MEM the field is GRADED and the value is genuinely not read: what an MR
//     with P1=1 answers on a SIMPLEX channel is unprinted on both radios,
//     which is A9, so the read choreography is P1=0 always and a fresh read
//     learns nothing about a channel's TX side. Unavailable means "not read",
//     not "absent"; publishing every channel as simplex would assert a fact
//     on no evidence. In SCAN the field is UNSUPPORTED, because P1=1 there is
//     the section channel's END frequency and not a transmit frequency
//     (M-E2) — the two halves are two ordinary slots carrying two ordinary
//     FieldFrequency values. The neutral model has one state for both cases
//     because codeplug.FieldState has three members and none of them means
//     "the capability table says this field does not exist"; the capability
//     table says that, and it is where a caller must look.
//
//     UNAVAILABLE ALONE PROTECTS NOTHING, and the refusal therefore lives in
//     the write path: codeplug.Diff counts tx_frequency only when its state
//     is Known, so an Unavailable one never reaches the per-field write gate
//     at all. Decision 11's requirement — a non-empty channel write needs a
//     KNOWN TX disposition where the bank publishes the field — is the write
//     ladder's (Stage 2 task 12), and it is what closes the silent-flattening
//     loop that an MW with P1=0 opens by explicitly converting a split
//     channel to simplex (590:1521-1523).
//
//   - Filter is KNOWN from byte 28 on the SG row and UNAVAILABLE on the S
//     row, matching FieldFilter's grade exactly. Byte 28 is ACCEPTED AS '0'
//     OR '1' ON PARSE ON BOTH ROWS — that is core/kw/ts590's
//     Byte28FilterEither on the S — because requiring '0' would make a
//     TS-590S at firmware >= 2.00 using FILTER B fail the WHOLE channel
//     read. What the S row cannot do is publish the value: a static
//     per-model capability table cannot say "Supported iff FV >= 2.00"
//     (Q12, §2.7), and reporting a Known filter under an Unsupported grade
//     would offer the user a column the write path must then refuse.
//
// THE MODE NAME IS THE LAYOUT'S, not a local table, and on these rows it is a
// function of TWO bytes: P5 = '4' is FM (590:1358) and P14 is "00: FM Normal"
// or "01: FM Narrow" (590:1569-1571), so kw.RecordModeName synthesises FM and
// FM-N from the pair. The names it returns are the same values
// Capabilities.Modes advertises, because caps.go asks the layout for them by
// the same route.
func (s *Session) channelData(rec kw.Record, bank spec.Bank) (*codeplug.ChannelData, error) {
	mode, ok := s.layout.RecordModeName(rec)
	if !ok {
		// Unreachable after core/kw's own parse: ParseMode refuses a nibble
		// outside this row's legend, and checkByte3940 refuses a P14 that is
		// neither printed value. Refuse rather than publish a channel whose
		// mode this row does not name.
		return nil, fmt.Errorf("unmapped mode nibble %v with P14 %q", rec.Mode, rec.Byte3940Wire())
	}
	toneMode, ok := toneModeNames[rec.ToneMode]
	if !ok {
		// Unreachable after Layout.ValidToneMode; see toneModeNames.
		return nil, fmt.Errorf("unmapped tone mode %v", rec.ToneMode)
	}
	toneTx, err := s.tone(rec.ToneIndex, "P8, the TN index")
	if err != nil {
		return nil, err
	}
	toneRx, err := s.tone(rec.CTCSSIndex, "P9, the CN index")
	if err != nil {
		return nil, err
	}

	filter := codeplug.StringField{State: codeplug.Unavailable}
	if s.row == RowSG {
		label, ok := filterLabels[rec.Byte28]
		if !ok {
			// Unreachable after Layout.checkByte28, which admits '0' and
			// '1' and nothing else on a row whose byte 28 is a filter.
			return nil, fmt.Errorf("unmapped byte 28 %q", rec.Byte28)
		}
		filter = codeplug.StringField{State: codeplug.Known, Value: label}
	}

	return &codeplug.ChannelData{
		// P4, eleven digits at bytes 7-17 (590:1541-1543).
		FreqHz: rec.FreqHz,
		// P5 x P14; see the doc comment.
		Mode: mode,

		// The 50-byte record has NO CLARIFIER POSITION over a complete
		// 47-byte account (590:1539-1577) — matrix M-E5 — and these three
		// members are plain scalars with no state to say so, so the honest
		// reading is their zero value, which is also what a write path reads
		// as "the caller set nothing here". It does NOT mean these radios
		// have no clarifier: they have RIT and XIT as radio-level settings
		// no memory channel stores.
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		// The Yaesu half of the vocabulary pair (decision 6): ctcss_state is
		// displaced by tone_mode below, and this record has no shift
		// selector at all — split is expressed only as two frames.
		CTCSS: "",
		Shift: "",
		// The Yaesu tone INDEX field, which is ONE field where this record
		// carries TWO independent indices; the values live on ToneTx and
		// ToneRx.
		CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},

		// P16 at bytes 42-49, right-trimmed of the spaces a write pads it
		// with — A1, assumed rather than printed.
		Tag: rec.Name,
		// No tag-display flag exists anywhere in this record; the capability
		// table grades the field the ZERO FieldSupport to say the same
		// thing.
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// P15 at byte 41: "0: Channel Lockout OFF / 1: Channel Lockout ON"
		// (590:1572-1574). Graded in SCAN as well as MEM: whether locking
		// out a scan edge is MEANINGFUL is not printed anywhere, and that
		// the byte is present and readable is (§2.2).
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: rec.Byte41 == '1'},

		// See the doc comment: not read in MEM (A9), and not a transmit
		// frequency at all in SCAN (M-E2).
		TxFreqHz: codeplug.FreqField{State: codeplug.Unavailable},
		// No duplex selector and no offset magnitude anywhere in the record.
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},
		// P7 at byte 20, four values (590:1549-1553).
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneMode},
		// P8 at 21-22 and P9 at 23-24, INDEPENDENT transmit and receive
		// indices into the printed charts — which is what makes "3: Cross
		// Tone ON" expressible at all (590:1167-1171).
		ToneTx: toneTx,
		ToneRx: toneRx,
		// DCS appears nowhere in either book.
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		// P11 at byte 28, on the SG row only; see the doc comment.
		Filter: filter,
		// P6 at byte 19, "refer to the DA command" (590:1546-1548). The
		// TS-480 spends this byte on its channel lockout instead, which is
		// the divergence with no roadmap line and that row's business.
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: rec.Byte19 == '1'},
		// Bytes 39-40 are the FM Normal/Narrow flag on these rows, folded
		// into the mode NAME above rather than published as a step; no step
		// magnitude, no on/off flag and no attenuator, preamp, antenna or
		// IP+ position exists in this record.
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}, nil
}

// tone maps one printed chart index onto the neutral tone field.
//
// THE DOMAIN IS A CAPABILITY, AND IT IS ASKED OF THE CAPABILITY SET rather
// than of the local table: s.caps.AdmitsTone is the ONE predicate every
// validator above this driver applies, so asking it here is what keeps a read
// from constructing a Known value codeplug.ToneField.Valid would then refuse.
//
// core/kw has already bounded both indices to their own printed charts —
// P8 to TN's 00-42 and P9 to CN's 00-41 (A21) — so a record that reaches here
// carries an index this row's 43-entry table has an entry for, and the
// out-of-range branch is a refusal rather than a clamp if it ever does not.
// It is also why a 1750 Hz tone_rx cannot come off a radio: CN's chart stops
// at 41, so the index that would produce it is refused at the parser.
func (s *Session) tone(index int, field string) (codeplug.ToneField, error) {
	if index < 0 || index >= len(kenwoodCTCSSTones) {
		return codeplug.ToneField{}, fmt.Errorf("%s is %d, outside the 43-entry chart this row publishes (590:2296-2306)", field, index)
	}
	value := kenwoodCTCSSTones[index]
	if !s.caps.AdmitsTone(value) {
		return codeplug.ToneField{}, fmt.Errorf("%s is %d, which is %v — a tone this row's capability table does not admit", field, index, value)
	}
	return codeplug.ToneField{State: codeplug.Known, Value: value}, nil
}

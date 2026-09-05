// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// readChannelGapHook, when non-nil, is called by ReadChannel with opMu already
// held and before any frame is built — a test-only seam, mirroring
// core/driver/ts590's and core/driver/ft891's namesakes. Always nil in
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
// THREE VALUES, WHERE THE 590 PAIR HAVE FOUR: "0: OFF, 1: TONE, 2: CTCSS"
// (480:964, and identically on MR at 480:922). The fourth, "3: Cross Tone ON"
// (590:1549-1553), has no counterpart in this legend, and core/kw's
// ToneModesThree axis is what refuses it before a Record exists — so this
// map's miss branch is unreachable in practice and refuses rather than
// mislabels if it ever is not.
//
// THE STRINGS ARE Capabilities.ToneModes' OWN, so a Known value this map
// produces is one codeplug.StringField.Valid admits. THE DIRECTIONS THOSE
// STRINGS IMPLY ARE K-D1 AND ARE ASSUMED — this book prints the three as bare
// labels and never says which direction each acts in (doc.go).
var toneModeNames = map[kw.ToneMode]string{
	kw.ToneModeOff:   "OFF",
	kw.ToneModeTone:  "TONE",
	kw.ToneModeCTCSS: "CTCSS",
}

// ErrUnknownSlot is the sentinel a caller compares against (via errors.Is)
// when a slot identifier is not one this row publishes. The error actually
// returned is an *UnknownSlotError.
var ErrUnknownSlot = errors.New("ts480: slot is not one this row publishes")

// UnknownSlotError reports a slot identifier that is not in this row's bank —
// whether because it is malformed or because it names a channel outside the
// printed space.
//
// NO FRAME IS SENT. The check is bank MEMBERSHIP, settled entirely from this
// session's own published capabilities, so the radio is never asked.
//
// THE 590 PAIR'S OWN SPELLINGS LAND HERE, and that is worth stating because it
// is the near miss a reader moving between the two packages will make: "042"
// and "100L" are perfectly good slot identifiers on those rows and are not
// identifiers at all on this one, whose MC prints neither a hundreds digit nor
// a section-channel half (480:827, 480:830).
type UnknownSlotError struct {
	// Slot is the identifier that was requested.
	Slot string
	// Model is the row it was requested of.
	Model string
	// Reason says how it failed.
	Reason string
}

// Error implements the error interface.
func (e *UnknownSlotError) Error() string {
	return fmt.Sprintf("ts480: slot %q is not published by the %s: %s", e.Slot, e.Model, e.Reason)
}

// Unwrap lets errors.Is(err, ErrUnknownSlot) match.
func (e *UnknownSlotError) Unwrap() error { return ErrUnknownSlot }

// ErrAnswerMismatch is the sentinel a caller compares against (via errors.Is)
// when a slot-addressed answer does not name what the read asked for. The
// errors actually returned are *AnswerMismatchError and
// *AnswerP1MismatchError.
var ErrAnswerMismatch = errors.New("ts480: answer does not name what was requested")

// AnswerMismatchError reports the requested and the answered channel.
//
// It is THIS package's OWN typed error, in this package's own namespace: the
// sibling drivers have same-shaped ones and none imports another, because a
// caller distinguishing which radio's read went wrong needs distinct types and
// a shared one would put a radio-specific failure on a seam meant to be
// neutral.
//
// IT IS NOT REACHABLE THROUGH THE CORRELATION PATH, and that is by design
// rather than by luck: kw.Layout.MRAnswerMatcher already compares the answered
// slot NUMBER, so another channel's answer is never delivered as this read's
// and the read times out instead. This guard is what refuses a frame that
// somehow arrived correlated and then decoded to a different channel — a
// contradiction rather than a stale frame — and refusing beats storing one
// channel's content under another's identifier.
type AnswerMismatchError struct {
	// Requested is the slot identifier the read asked for.
	Requested string
	// Answered is the slot identifier the reply actually named.
	Answered string
}

// Error implements the error interface.
func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ts480: requested slot %q but the answer names slot %q — refusing to map a reply onto the wrong slot", e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }

// AnswerP1MismatchError reports an MR answer whose P1 names a different half
// of the addressing than the read asked for.
//
// ON THIS ROW IT IS THE ONLY FORM THE CHECK CAN TAKE, because this row's slot
// identifier carries no half at all: the 590 pair spell a section channel's
// two frequencies "100L" and "100U", so a wrong half there is caught by the
// slot-string comparison, while every identifier here is two digits and P1 is
// discarded from it.
//
// WHAT IT REFUSES IS A REAL LOSS. P1='1' is "1: TX frequency" (480:951) and,
// on channels 90-99, the section END frequency (480:986-987). This driver
// sends P1='0' on every read (kw.Slot.P1 derives it from the slot's class and
// this row declares only SlotMemory), so accepting a P1='1' answer would store
// the wrong frequency as the channel's own with tx_frequency left Unavailable
// — silent loss codeplug.Validate cannot see, and exactly what M-E2 and
// decision 11 exist to prevent.
//
// IT IS NOT REACHABLE FROM THIS DRIVER'S OWN TRAFFIC TODAY — nothing here ever
// asks for P1='1' — so what it refuses is a radio, a cable or a stale frame
// contradicting the request. The comparison is the read-side twin of one
// core/kw's builder already makes (kw.BuildMWSet refuses a record whose
// AnswerP1 disagrees with its slot's class).
//
// It shares AnswerMismatchError's sentinel because a caller asking "did this
// answer name what I asked for?" is asking one question, and the two types are
// what distinguish the two ways the answer can differ.
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
	return fmt.Sprintf("ts480: slot %q was read with P1=%q but the answer carries P1=%q — on this radio P1=1 is the transmit side of a split channel (480:951), and on channels 90-99 the section end frequency (480:986-987); storing either as the channel's frequency would lose the difference silently", e.Slot, e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerP1MismatchError) Unwrap() error { return ErrAnswerMismatch }

// parseSlotID reads a canonical slot identifier as a channel number.
//
// TWO DIGITS, AND NO HALF SUFFIX, which is the whole of this function's
// difference from core/driver/ts590's namesake — see slotID (caps.go) for why
// the wire form is two digits here and three there. There is no "L"/"U" to
// parse because this row has no section-channel CLASS for a half to belong to:
// channels 90-99 are ordinary memories that also answer a second frame
// (480:943-944, 480:986-987, §1.4.3), and this programme never sends that
// second frame.
//
// IT IS SYNTAX ONLY, and that is deliberate: it is the first rung of the
// plan's P7 ladder (parse, then bank membership, then the rest). Bank
// membership is what refuses an in-syntax number outside the published space,
// in one shape, so a user never meets two different refusals for one mistake.
func parseSlotID(id string) (int, error) {
	if len(id) != 2 {
		return 0, fmt.Errorf("slot %q: a slot identifier on this row is exactly two digits, the width its own book prints for a memory channel (480:830, 480:955) — the TS-590 pair's three-digit form and their \"L\"/\"U\" section-channel halves are not identifiers here", id)
	}
	for _, b := range []byte(id) {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("slot %q: the channel number is two decimal digits", id)
		}
	}
	return int(id[0]-'0')*10 + int(id[1]-'0'), nil
}

// bankFor returns the bank of THIS SESSION's published capabilities that holds
// id, and whether any does.
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

// bankNames renders this session's bank inventory for a refusal, so the
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
// IT IS kw.Layout.MRAnswerMatcher AND NOT kw.PrefixLenMatcher, FROM BIRTH ON
// THIS ROW (the T13 review's ruling). Every memory answer this family sends is
// fifty bytes and starts "MR", with the channel number at P2/P3 rather than
// immediately after the command name, so no prefix a caller can spell
// separates one channel's answer from another's: under a prefix-and-length
// matcher a very late answer to a PREVIOUS read is delivered as this read's
// own. The driver would then refuse it — *AnswerMismatchError is exactly that
// defence — but refusing is the wrong verdict for a frame that was never this
// read's answer, and it costs the retry that would have succeeded.
//
// IT COMPARES THE SLOT AND NOT P1, which is the matcher's own division: a
// wrong P1 is still correlated and still refused precisely, by
// *AnswerP1MismatchError, rather than disappearing into a timeout.
//
// AND ON THIS ROW IT ADMITS ONE SPELLING THE PARSER THEN REFUSES, which is a
// consequence worth stating where a reader of the read path will meet it.
// core/kw's parseSlot takes byte 4 as a hundreds digit on the 590 pair and, on
// the P2FixedZero arm this row uses, does not look at it at all. So an answer
// spelling P2 as the 590 pair's SPACE (590:1334-1337) IS correlated as this
// read's own — and kw.Layout.ParseMRAnswer then refuses it, because
// checkPrintedFixed runs first and requires the '0' this book prints
// ("Always 0 for the TS-480.", 480:953). THE OUTCOME A USER SEES IS A NAMED
// PARSE REFUSAL CITING 480:953, NEVER A TIMEOUT, and that is the right side to
// err on: the frame really was the answer to this read, so reporting it as
// "no answer" would hide a radio disagreeing with its own book behind a
// symptom that looks like a dead cable.
// TestReadChannel_ASpaceInByteFourIsCORRELATEDANDTHENREFUSED pins both halves.
//
// THE LENGTH IS STILL core/kw's, inside that matcher rather than passed to it:
// a frame of any width but kw.RecordLen is not this read's answer at all, so a
// 49-byte "MR…" never reaches ParseMRAnswer and the read times out instead.
//
// ONE RETRY, and it is the ordinary reasoning: a read is idempotent and a
// single swallowed reply should not fail a whole-radio read of a hundred
// slots. The retry is preceded by the engine's own drain-to-quiet quarantine,
// so it cannot correlate a stale answer with the second attempt. What a retry
// does NOT do is turn silence into information: a read that still draws
// nothing fails the session read WHOLE, with the typed error
// kw.NewTimeoutError builds, because this book says the NAK is unreliable
// (480:136-138) and a timeout is therefore neither "absent" nor "rejected".
func (s *Session) mrSpec(slot kw.Slot) transport.CommandSpec {
	return s.newReadSpec(s.layout.MRAnswerMatcher(slot), 1)
}

// ReadChannel implements driver.Session: ONE MR frame per slot, and nothing
// else (plan P13, matrix §3.8).
//
//   - MR P1=0 ON EVERY SLOT, AND NEVER P1=1. kw.Slot.P1 derives the byte from
//     the slot's CLASS and this row declares one flat SlotMemory range
//     (480:955), so there is no slot here whose P1 is '1'. THE P1=1 HALF OF
//     CHANNELS 90-99 IS THEREFORE UNREACHABLE THROUGH THIS PROGRAMME — the
//     book really does print it, "Memory channel 90 ~ 99: P1=0 (start
//     frequency), P1=1 (end frequency)" (480:943-944, 480:986-987), and
//     decision 15 rules those ten ordinary memories rather than a scan class
//     because a slot string is unique across a codeplug and a Bank.Fields map
//     is per bank. That is a real loss and it is published rather than papered
//     over; the ten CHANNELS are read like any other.
//   - MC IS NEVER USED TO READ, and no discovery frame of any kind is ever
//     built (decision 5's two independent grounds; doc.go). Recalling a channel
//     changes the radio's operating state.
//
// THE WHOLE OPERATION IS HELD UNDER opMu (P13/P14). transport.Engine
// serialises each individual exchange, not a whole driver operation; holding
// the lock for every one of them, rather than only for the several-frame ones,
// is what makes the rule "one driver operation at a time" rather than "one
// driver operation at a time, except the short ones".
//
// FAILURE MODES, and the design's reading of each:
//
//   - An UNKNOWN SLOT is refused before any frame is built —
//     *UnknownSlotError.
//   - A "?;" IS A DEFINITIVE REJECTION, NEVER "absent" (decision 5, P13). This
//     book prints it as EITHER "Command syntax was incorrect." OR "Command was
//     not executed due to the current status of the transceiver (even though
//     the command syntax was correct)." (480:130-135) — indistinguishable — so
//     it is reported as the typed kw.RejectionError, which names both causes
//     and the transient sentence, and it fails the session read WHOLE.
//   - A TIMEOUT is the typed kw.TimeoutError, which says in as many words that
//     it is not an inference of absence. It fails the session read whole too.
//   - AN EMPTY CHANNEL IS NOT A FAILURE AND IS NOT A REJECTION — and on THIS
//     row that sentence rests on A4, which is UNLIFTED and is this row's
//     registration gate (L-HW-3, §3.13). The structural rule is core/kw's and
//     is applied on both rows: an MR answer whose P4-P15 are all zero, with a
//     blank P16, is reported as codeplug.Channel{Slot: id} with a nil Data.
//     THAT IS A PROPERTY OF THE FRAME AND NOT A CLAIM ABOUT THE RADIO. The 590
//     pair's book documents the behaviour (590:1492-1493); this one prints
//     nothing about an empty channel anywhere, so if A4 is false — if an MR of
//     an unwritten channel answers "?;" instead — decision 5 makes that a
//     definitive rejection and a fresh TS-480 out of the box cannot be read at
//     all. There is no fallback and no honest way to invent one, which is why
//     this row is built and NOT registered (doc.go, P3).
//
// core/clone's ReadAll propagates the first ReadChannel error, so returning one
// here is what makes a whole-radio read fail whole rather than silently
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

	number, err := parseSlotID(id)
	if err != nil {
		return codeplug.Channel{}, &UnknownSlotError{Slot: id, Model: modelName, Reason: err.Error()}
	}
	bank, ok := s.bankFor(id)
	if !ok {
		return codeplug.Channel{}, &UnknownSlotError{
			Slot:   id,
			Model:  modelName,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}
	slot, err := s.layout.NewSlot(number, kw.ScanHalfNone)
	if err != nil {
		// Unreachable for a slot the bank published, since every published
		// identifier was rendered from this same layout's NewSlot
		// (caps.go). Refuse rather than build a frame from a slot the codec
		// disowns.
		return codeplug.Channel{}, &UnknownSlotError{Slot: id, Model: modelName, Reason: err.Error()}
	}

	cmd, err := s.layout.BuildMRRead(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts480: ReadChannel %s: %w", id, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.mrSpec(slot))
	if err != nil {
		// wireFailure is what types a "?;" and a timeout — see there, and
		// see this function's own doc comment for what each means on this
		// radio. It is shared with the probe so one rule is applied once.
		return codeplug.Channel{}, fmt.Errorf("ts480: ReadChannel %s: %w", id, wireFailure(s.layout, "MR", err))
	}

	// THE SIXTEEN PRINTED-FIXED BYTES ARE REQUIRED HERE, INSIDE THIS CALL,
	// AND THE CHOICE IS A24's. kw.Layout.ParseMRAnswer runs checkPrintedFixed
	// before it reads any field, and on a Book480 layout its refusal names
	// A24 by name. THE STRICTNESS IS A CHOICE RATHER THAN A DEDUCTION on this
	// row: the book's own general note permits a SET to fill a parameter "not
	// applicable to this transceiver" with "any character except the ASCII
	// control codes (00 to 1Fh) and the terminator (;)" (480:108-111), so a
	// radio ANSWERING a hard-wired byte with something else would not
	// necessarily be faulty. A24's lift is a dozen real reads of a real
	// TS-480 (L-HW-18), and a single counter-example turns the rule from
	// "required" into "accepted and normalised" — which would be a change in
	// core/kw, not here. Sixteen bytes in six runs, against the 590 pair's
	// thirteen in three (§5).
	rec, err := s.layout.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts480: ReadChannel %s: %w", id, err)
	}
	// THE COMPARISON IS OF THE NUMBER AND NOT OF kw.Slot.String(), which is
	// the one place this driver may not reuse the codec's own rendering:
	// Slot.String formats "%03d" for the 590 pair's printed width and this
	// row's identifiers are two digits (slotID, caps.go). Comparing strings
	// here would refuse every genuine answer.
	if got := rec.Slot.Number(); got != number {
		return codeplug.Channel{}, &AnswerMismatchError{Requested: id, Answered: slotID(got)}
	}
	if want := slot.P1(); rec.AnswerP1 != want {
		return codeplug.Channel{}, &AnswerP1MismatchError{Slot: id, Requested: want, Answered: rec.AnswerP1}
	}
	if rec.Empty {
		// See ReadChannel's doc comment: the STRUCTURE is core/kw's and the
		// claim that this radio ever sends such a frame is A4, unlifted.
		return codeplug.Channel{Slot: id}, nil
	}

	data, err := s.channelData(rec, bank)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts480: ReadChannel %s: %w", id, err)
	}
	return codeplug.Channel{Slot: id, Data: data}, nil
}

// channelData maps one parsed 50-byte record onto the neutral channel model.
//
// ONE MAPPER, ONE BANK, which is what makes the twenty-two Unavailable tier
// fields structural rather than duplicated: a fresh read leaves no tier field
// Absent, so it saves as the lowest schema and the fleet's no-Absent rule
// holds.
//
// FOUR CELLS ARE READ DIFFERENTLY FROM THE TS-590 PAIR'S, and every one of
// them is a byte the two books spend differently (§5). They are listed here
// because a reader who knows the 590 mapper will otherwise look for them in
// the wrong place:
//
//   - SCAN SKIP COMES FROM BYTE 19. "Lockout status. 0: Lockout OFF, 1:
//     Lockout ON" (480:962). The 590 pair carry their lockout at byte 41
//     (590:1572-1574) and this radio prints a constant there ("Always 0 for
//     the TS-480.", 480:982), so the two radios swap the bytes and a mapper
//     that read byte 41 here would report every channel unlocked.
//   - DATA MODE IS UNAVAILABLE, because byte 19 is spent on the lockout and
//     there is no data-mode position anywhere in this record. §5 calls this
//     the divergence no roadmap line records, and it is the sharpest single
//     argument for two capability tables rather than one.
//   - THE MODE NAME IS THE LAYOUT'S AND IS ONE BYTE, not two. On the 590 pair
//     kw.RecordModeName synthesises FM and FM-N from P5=4 × P14; here P14 is
//     the tuning step (480:979) and the layout's Byte3940 axis says so, so the
//     same method returns the plain legend name. Asking the layout rather than
//     a local table is what keeps this mapper and Capabilities.Modes the same
//     vocabulary.
//   - THE TWO TONE INDICES ARE PARSED AND NOT PUBLISHED. core/kw bounds P8
//     against TN's printed 00 ~ 42 (480:1557) and P9 against CN's 00 ~ 41
//     (480:337), so the record carries two valid indices — and this book
//     prints NEITHER CHART ("Refer to page 32 of the TS-480 instruction
//     manual", 480:1559-1560; page 33 for CN, 480:339-340), so this programme
//     cannot say what either index means in hertz. The neutral model's tone
//     fields are HERTZ-VALUED and there is no opaque-index field, so both are
//     reported Unavailable — matching FieldToneTx/FieldToneRx's zero grade
//     exactly, and read from the same place as that grade. Publishing the
//     index as though it were a frequency, or borrowing the TS-590's chart
//     across the model boundary, are the two things §1.9 and Q2 refuse.
//
// TONE MODE IS PUBLISHED THOUGH ITS VALUES ARE NOT, and the asymmetry is
// deliberate and documented (§1.19): a TS-480 channel's tone MODE round-trips
// and its tone VALUE does not — a documented lossy round trip. Publishing OFF
// alone instead would refuse to READ the channels that carry a tone at all.
// What the loss costs on the WRITE side is Q2's subsidiary cause inside A22's
// refusal (write.go).
func (s *Session) channelData(rec kw.Record, bank spec.Bank) (*codeplug.ChannelData, error) {
	mode, ok := s.layout.RecordModeName(rec)
	if !ok {
		// Unreachable after core/kw's own parse: ParseMode refuses a nibble
		// outside this row's legend. Refuse rather than publish a channel
		// whose mode this row does not name.
		return nil, fmt.Errorf("unmapped mode nibble %v", rec.Mode)
	}
	toneMode, ok := toneModeNames[rec.ToneMode]
	if !ok {
		// Unreachable after Layout.ValidToneMode, whose ToneModesThree axis
		// refuses '3' on this row; see toneModeNames.
		return nil, fmt.Errorf("unmapped tone mode %v", rec.ToneMode)
	}
	// The tone-mode string must be one this session's own capability set
	// admits, asked of the SAME place a validator above this driver will ask.
	if !bank.Fields[spec.FieldToneMode].Unreachable() && !toneModeAdmitted(s.caps, toneMode) {
		return nil, fmt.Errorf("tone mode %q is not one this row's capability table publishes", toneMode)
	}

	return &codeplug.ChannelData{
		// P4, eleven digits at bytes 7-17 (480:957).
		FreqHz: rec.FreqHz,
		// P5 alone; see the doc comment.
		Mode: mode,

		// The 50-byte record has NO CLARIFIER POSITION over this book's own
		// complete 47-byte account (480:951-984) — matrix M-E5 — and these
		// three members are plain scalars with no state to say so, so the
		// honest reading is their zero value. It does NOT mean this radio
		// has no clarifier: it has RIT and XIT as radio-level settings no
		// memory channel stores, and RC "Clears the RIT offset frequency"
		// (480:1205) is this book's own proof of it.
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		// The Yaesu half of the vocabulary pair (decision 6): ctcss_state is
		// displaced by tone_mode below, and this record has no shift
		// selector at all.
		CTCSS: "",
		Shift: "",
		// The Yaesu tone INDEX field, which is ONE field where this record
		// carries TWO indices — and neither of those has a chart in this
		// book in any case.
		CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},

		// P16 at bytes 42-49, right-trimmed of the spaces a write pads it
		// with — A1, assumed rather than printed, with this book's own KY
		// precedent for a different command.
		Tag: rec.Name,
		// No tag-display flag exists anywhere in this record; the capability
		// table grades the field the ZERO FieldSupport to say the same
		// thing.
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// P6 at BYTE 19 — the channel lockout on this radio (480:962). See
		// the doc comment: the 590 pair read the same neutral field from
		// byte 41.
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: rec.Byte19 == '1'},

		// Unsupported on the WHOLE row (M-E2): channels 90-99 are ordinary
		// memories that also answer a second frame, and no bank split can
		// separate them.
		TxFreqHz: codeplug.FreqField{State: codeplug.Unavailable},
		// No duplex selector and no offset magnitude anywhere in the record.
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},
		// P7 at byte 20, THREE values (480:964). The directions the strings
		// imply are K-D1 and are assumed.
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneMode},
		// P8 at 21-22 and P9 at 23-24: parsed by the codec, bounded by the
		// printed ranges, and NOT published — see the doc comment.
		ToneTx: codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx: codeplug.ToneField{State: codeplug.Unavailable},
		// DCS appears nowhere in either book.
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		// Byte 28 is "Always 0 for the TS-480." (480:973); there is no
		// per-channel filter selection to report.
		Filter: codeplug.StringField{State: codeplug.Unavailable},
		// Byte 19 is the LOCKOUT here, published above as scan_skip. There
		// is no data-mode position in this record at all.
		DataMode: codeplug.BoolField{State: codeplug.Unavailable},
		// Bytes 39-40 ARE a step on this row (480:979) and the field is
		// REFUSED rather than absent: ST's legend is mode-conditional over
		// two ranges (480:1494-1500) and no flat vocabulary is honest, so
		// the value is not published and the write path refuses every
		// channel write in consequence (A22, write.go). No step magnitude in
		// hertz and no on/off flag exists at all.
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		// Radio-level functions with no record position (480:1185,
		// 480:1055-1058), and an Icom concept this book never mentions.
		AttenuatorDB: codeplug.IntField{State: codeplug.Unavailable},
		Preamp:       codeplug.StringField{State: codeplug.Unavailable},
		Antenna:      codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:       codeplug.BoolField{State: codeplug.Unavailable},
	}, nil
}

// toneModeAdmitted reports whether value is one of the tone-mode strings caps
// publishes. It is asked of the CAPABILITY SET rather than of the local map so
// that a read cannot construct a Known value codeplug.StringField.Valid would
// then refuse.
func toneModeAdmitted(caps spec.Capabilities, value string) bool {
	for _, tm := range caps.ToneModes {
		if tm.Value == value {
			return true
		}
	}
	return false
}

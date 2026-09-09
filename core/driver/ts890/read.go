// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// readChannelGapHook, when non-nil, is called by ReadChannel with opMu
// already held and before any frame is built — a test-only seam, mirroring
// core/driver/ts590's and core/driver/ft710's namesakes. Always nil in
// production, so it costs one nil check per read.
//
// It exists for TestReadChannel_IsAtomicUnderOpMu, to park one operation
// INSIDE the lock deterministically rather than relying on goroutine
// scheduling: Go's sync.Mutex favours an immediately-re-locking goroutine so
// heavily that the interleaving opMu exists to forbid is near-impossible to
// reproduce by hammering alone.
var readChannelGapHook func()

// toneModeNames maps the record's P5 wire byte to the neutral tone_mode
// vocabulary this row publishes.
//
// FOUR VALUES: "0: OFF / 1: Tone / 2: CTCSS / 3: Cross Tone" (890:3180-3185),
// corroborated by the standalone TO command's identical legend
// (890:5170-5178). The strings are Capabilities.ToneModes' own, so a Known
// value this map produces is one codeplug.StringField.Valid admits.
//
// THE VALUES ARE THE BOOK'S AND THE MEANINGS ARE NOT — register K-D1
// (doc.go, matrix M-E2). Nothing in this map depends on the meanings; what
// does is the capability table's Semantics and the assignment of P6 to
// tone_tx and P7 to tone_rx below.
//
// core/kw/ma has already refused any byte outside the printed legend before a
// Record exists, so the miss branch is unreachable in practice; it refuses
// rather than mislabels if it ever is not.
var toneModeNames = map[byte]string{
	'0': "OFF",
	'1': "TONE",
	'2': "CTCSS",
	'3': "CROSS",
}

// ErrUnknownSlot is the sentinel a caller compares against (via errors.Is)
// when a slot identifier is not one this row publishes. The error actually
// returned is an *UnknownSlotError.
var ErrUnknownSlot = errors.New("ts890: slot is not one this row publishes")

// UnknownSlotError reports a slot identifier that is not in this row's bank —
// whether because it is malformed, because it names a channel outside the
// printed space, or because it names one of the twenty this driver
// deliberately does not publish.
//
// NO FRAME IS SENT. The check is bank MEMBERSHIP, settled entirely from this
// session's own published capabilities, so the radio is never asked.
//
// SLOTS 100-119 ARE REFUSED HERE, AND BY EXACTLY THIS MECHANISM. The
// distinction that makes that honest is between the CODEC'S SLOT DOMAIN and
// the DRIVER'S PUBLISHED BANKS, which are two different questions. The book
// prints the whole space, "000 ~ 119" with the class map beneath it
// (890:3167-3169), so core/kw/ma's layout DECLARES all three classes and an
// answer naming one parses rather than being refused as malformed. What an
// MA0 read of a Programmable VFO slot ANSWERS is nowhere printed (A9), what
// its second frequency would MEAN is nowhere printed (A7, MA6 having an end
// frequency of its own at 890:3315-3323), and the E channels are explained
// nowhere in this book at all (A10). So the twenty are not published, and a
// read naming one is refused THE SAME SHAPE as any other slot this row does
// not publish — no invented radio behaviour and no special case in the code.
type UnknownSlotError struct {
	// Slot is the identifier that was requested.
	Slot string
	// Reason says which of the ways it failed.
	Reason string
}

// Error implements the error interface.
func (e *UnknownSlotError) Error() string {
	return fmt.Sprintf("ts890: slot %q is not published by the %s: %s", e.Slot, modelName, e.Reason)
}

// Unwrap lets errors.Is(err, ErrUnknownSlot) match.
func (e *UnknownSlotError) Unwrap() error { return ErrUnknownSlot }

// parseSlotID reads a canonical slot identifier as a channel number.
//
// IT IS SYNTAX ONLY, and that is deliberate: it is the first rung of the
// ladder, and bank membership is the next. If it refused an out-of-space
// number itself, "115" would be refused by two different mechanisms — the
// codec's slot space and the driver's published banks — and which one a user
// met would depend on the number rather than on the question.
//
// THE WIRE FORM IS THE RADIO'S OWN PRINTED WIDTH (matrix §1.4.1): three
// digits, because MA0's P1 is three cells over a printed domain of "000 ~
// 119" (890:3166-3168). There is no "L"/"U" suffix on this row, where pair 1
// has one, because one channel number reaches one record here: the second
// frequency is a FIELD of that record, not a second addressable half. It is
// the exact form ma.Slot.String renders, which is what the bank inventory is
// built from, so the two cannot drift.
func parseSlotID(id string) (int, error) {
	if len(id) != 3 {
		return 0, fmt.Errorf("slot %q: a slot identifier on this row is three digits (890:3166-3168)", id)
	}
	for _, b := range []byte(id) {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("slot %q: the channel number is three decimal digits", id)
		}
	}
	return int(id[0]-'0')*100 + int(id[1]-'0')*10 + int(id[2]-'0'), nil
}

// bankNames renders this session's bank inventory for a refusal, so the
// message says what THIS radio publishes rather than what the family does.
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

// ma0Spec is the transport spec for one MA0 read of slot: the codec's own
// answer matcher, and one retry.
//
// IT IS ma.Layout.MA0AnswerMatcher AND NOT kw.PrefixLenMatcher, and the
// reason is the FLOATING TERMINATOR rather than pair 1's (whose channel
// number sits too deep in the frame for a prefix). Here the prefix IS the
// whole correlation key — "MA0" plus the three digits, positions 1-6
// (890:3184-3186) — but the width is a RANGE of 40 to 50, so a
// prefix-and-exact-length matcher cannot express it and
// PrefixLenMatcher's unbounded branch would correlate a two-hundred-byte run
// of noise that happened to open with the right six bytes. The layout's
// matcher takes the range: it correlates what the book can produce, refuses
// what it cannot, and still delivers a corrupt-but-plausible frame to the
// parser so it is refused WITH A MESSAGE rather than reported as a timeout.
//
// BOTH HALVES REST ON A5 — that the radio spells the channel back as three
// zero-padded digits and never space-pads it the way the 590SG's MC does.
// The failure direction is safe: a space-padded answer MISSES the matcher and
// times out; it cannot be mis-attributed to another channel.
//
// ONE RETRY, and it is the ordinary reasoning: a read is idempotent and a
// single swallowed reply should not fail a whole-radio read of a hundred
// slots. The retry is preceded by the engine's own drain-to-quiet quarantine,
// so it cannot correlate a stale answer with the second attempt. What a retry
// does NOT do is turn silence into information — see ReadChannel.
func (s *Session) ma0Spec(slot ma.Slot) transport.CommandSpec {
	return s.newReadSpec(s.layout.MA0AnswerMatcher(slot), 1)
}

// ReadChannel implements driver.Session: ONE MA0 Read frame per slot, and
// nothing else (plan P12, matrix §3.8).
//
// A18 IS WHAT MAKES THIS WORK AT ALL, and it is the load-bearing read
// assumption of the whole milestone, cited here because this is where it
// bites. NO MN PRECEDES THE READ: the frame carries its own channel number
// (890:3184-3186), so the command is stateless on its face — but MA1, MA7 and
// MI all act on "the channel appointed when using this command"
// (890:3237-3238), so the family HAS state and MA0 Read's independence from it
// is not printed. Spec decision 5 removed MN from the roster in both
// directions, so this driver could not select a channel even if it had to;
// the assumption is therefore promoted rather than withdrawn, and A18's own
// note says it plainly: IF A18 IS FALSE, NO READ ON THIS ROW WORKS AT ALL.
// Its lift is L-HW-12, hardware item 10 — from VFO mode, with no MN sent,
// read channel 005 — and the HANDOFF names it beside item 1 as the gate whose
// failure is TOTAL rather than partial. It is cheap; run it early.
//
// THE WHOLE OPERATION IS HELD UNDER opMu (plan P13). transport.Engine
// serialises each individual exchange, not a whole driver operation, and this
// session's operations are not all single exchanges.
//
// FAILURE MODES, and the design's reading of each:
//
//   - An UNKNOWN SLOT is refused before any frame is built —
//     *UnknownSlotError, which is also how the unpublished classes 100-119
//     are refused.
//   - A "?;" IS A DEFINITIVE REJECTION, NEVER "absent". The book prints it as
//     EITHER "Command syntax was incorrect" OR "Command was not executed due
//     to the current status of the transceiver" (890:106-112) —
//     indistinguishable — so it is reported as the typed kw.RejectionError,
//     which names both causes and the transient sentence, and it fails the
//     session read WHOLE.
//   - A TIMEOUT is the typed kw.TimeoutError, which says in as many words
//     that it is not an inference of absence. It fails the session read whole
//     too.
//   - AN EMPTY CHANNEL IS NOT A FAILURE AND IS NOT A REJECTION (plan P14).
//     See below.
//
// core/clone's ReadAll returns on the FIRST ReadChannel error and abandons
// the whole radio's read (core/clone/read.go:65-67), so returning one here is
// what makes a whole-radio read fail whole rather than silently dropping a
// channel — and it is also why every refusal this driver can foresee at read
// time is a Record STATE and not an error.
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
		return codeplug.Channel{}, &UnknownSlotError{Slot: id, Reason: err.Error()}
	}
	if _, ok := s.caps.BankOf(id); !ok {
		return codeplug.Channel{}, &UnknownSlotError{
			Slot:   id,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}
	slot, err := s.layout.NewSlot(number)
	if err != nil {
		// Unreachable for a slot the bank published, since every published
		// identifier was rendered by this same layout's NewSlot (caps.go).
		// Refuse rather than build a frame from a slot the codec disowns.
		return codeplug.Channel{}, &UnknownSlotError{Slot: id, Reason: err.Error()}
	}

	cmd, err := s.layout.BuildMA0Read(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts890: ReadChannel %s: %w", id, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.ma0Spec(slot))
	if err != nil {
		// wireFailure is what types a "?;" and a timeout — see there, and
		// see this function's own doc comment for what each means. It is
		// shared with the probe so one rule is applied once.
		return codeplug.Channel{}, fmt.Errorf("ts890: ReadChannel %s: %w", id, wireFailure(s.layout.Book(), "MA0", err))
	}

	rec, err := s.layout.ParseMA0Answer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts890: ReadChannel %s: %w", id, err)
	}
	if rec.Empty {
		// THE EMPTY PREDICATE IS A PREDICATE AND NOT A VALIDITY RULE (plan
		// P14). The book documents the blank channel as a normal state with
		// a defined answer — "When reading a blank channel, parameters P2 to
		// P12 becomes blank" (890:3215-3216) — and core/kw/ma tests that
		// window BEFORE any per-field domain parse, so a blank mode byte is
		// the empty marker rather than a parse failure. Data nil is the
		// driver seam's own spelling of "this slot is unassigned"; it is
		// NEVER an error, because clone.ReadAll abandons a whole radio's
		// read on the first channel error and one unused slot would
		// therefore make a fresh radio unreadable end to end.
		//
		// P13 IS IGNORED BY THE PREDICATE AND ITS RESIDUE IS CARRIED HERE.
		// This book's blank note stops at P12 and says nothing about the
		// name window (erratum E4), so a fresh radio may answer a blank
		// channel with bytes still in it. That such a residue is not
		// channel content is A21, and the residue is reported rather than
		// discarded silently or raised as an error — the third option being
		// the one that would cost the whole read.
		s.noteNameResidue(id, rec)
		return codeplug.Channel{Slot: id}, nil
	}

	data, err := s.channelData(rec)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts890: ReadChannel %s: %w", id, err)
	}
	return codeplug.Channel{Slot: id, Data: data}, nil
}

// noteNameResidue reports A21's residue on an unassigned channel, and is a
// no-op when there is none or when no logger was given.
//
// THE LOGGER IS THE ONLY SINK A DRIVER HAS for a per-channel note:
// codeplug.Channel carries no detail field, driver.SessionDiagnostics is a
// counter, and an error would cost the whole radio's read. Reporting it is
// what P14 means by carrying the residue rather than discarding it.
//
// The note is reachable only when a caller supplies a logger; nothing
// outside core/driver does so today, so T17's registration must wire one
// for this row or the residue is unobservable in the shipping programme.
func (s *Session) noteNameResidue(id string, rec ma.Record) {
	if s.logger == nil || rec.NameResidue == "" {
		return
	}
	s.logger.Printf("ts890: slot %s is unassigned by the blank-window predicate (P2-P12 blank, 890:3215-3216) but its name window carries %q; this book's blank note stops at P12 (erratum E4) and A21 is the assumption that such a residue is not channel content", id, rec.NameResidue)
}

// channelData maps one parsed MA0 record onto the neutral channel model.
//
// THE MODE NAME IS caps.go's modeName AND NOT A SECOND RULE HERE, which is
// what makes the name this read publishes byte-for-byte the one
// Capabilities.Modes advertises. It is a function of TWO bytes on this row —
// P3, the mode, and P4, the FM Normal/Narrow flag (890:3174-3178) — and the
// synthesis lives in one place for both callers.
//
// EVERY ONE OF THE TWENTY-SEVEN NEUTRAL FIELDS IS ANSWERED, Known or
// Unavailable and never Absent: Absent is the one state meaning "nobody has
// said anything", which a read has no business producing, and
// codeplug.FieldState.RepresentableByOmission would promote the saved schema
// besides. drivertest.AssertFreshReadSaveLoad is the fleet's pin on that.
//
// THE SECONDARY SIDE IS READ AND HAS NOWHERE TO GO, and that is spec
// decision 9 rather than an omission here. codeplug.ChannelData has ONE mode,
// ONE FM width and ONE tone tuple; this record gives the transmit side its own
// mode P9 and its own width P10 (890:3193-3200). Those values are parsed,
// checked against their printed domains and then dropped from the neutral
// channel — a read does not fail on that, because refusing to read a channel
// the user can see on the front panel would be worse than reporting the part
// the model holds. What protects them is the WRITE path, which compares the
// radio's current secondary fields against what its Set would emit and
// refuses the write naming the P-numbers (T12's rung 11).
func (s *Session) channelData(rec ma.Record) (*codeplug.ChannelData, error) {
	mode, ok := modeName(s.layout, rec.Mode, rec.FMNarrow)
	if !ok {
		// Unreachable after core/kw/ma's own parse, which refuses a byte
		// outside this row's legend — including '0' and '8', both printed
		// "Unused" (890:3977, 890:3985). Refuse rather than publish a
		// channel whose mode this row does not name.
		return nil, fmt.Errorf("unmapped mode byte %q with the FM width flag %v", rec.Mode, rec.FMNarrow)
	}
	toneMode, ok := toneModeNames[rec.ToneType]
	if !ok {
		// Unreachable after the codec's own tone-type check; see
		// toneModeNames.
		return nil, fmt.Errorf("unmapped tone type %q", rec.ToneType)
	}
	toneTx, err := s.tone(rec.ToneIndex, "P6, the TN index")
	if err != nil {
		return nil, err
	}
	toneRx, err := s.tone(rec.CTCSSIndex, "P7, the CN index")
	if err != nil {
		return nil, err
	}

	return &codeplug.ChannelData{
		// P2, eleven digits at bytes 7-17 (890:3171-3172).
		FreqHz: rec.FreqHz,
		// P3 x P4; see the doc comment.
		Mode: mode,

		// THE GRID HAS NO CLARIFIER POSITION over the complete
		// thirteen-parameter account (890:3164-3209) — matrix M-E4/M-E5 —
		// and these three members are plain scalars with no state to say so,
		// so the honest reading is their zero value, which is also what a
		// write path reads as "the caller set nothing here". It does NOT
		// mean this radio has no clarifier: it has RIT and XIT (890:4558,
		// 890:5411) as radio-level settings no memory channel stores.
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		// The Yaesu half of the vocabulary pair: ctcss_state is displaced by
		// tone_mode below, and this record has no shift selector at all —
		// split is an absolute second frequency plus a flag.
		CTCSS: "",
		Shift: "",
		// The Yaesu tone INDEX field, which is ONE field where this record
		// carries TWO independent indices; the values live on ToneTx and
		// ToneRx.
		CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},

		// P13 from byte 40, up to ten characters, CARRIED VERBATIM
		// (890:3208-3209). The terminator floats at 40 + len(name)
		// (890:3181-3182), so nothing is padded and nothing is trimmed:
		// "AB ;" and "AB;" are distinct frames to the radio's own parser, so
		// a trailing space is real content. This row's half of A1 is
		// therefore NOT an assumption (core/kw/ma/doc.go, the C-MED-1
		// reversal); the TS-990S's fixed window is the row that assumes a
		// pad.
		Tag: rec.Name,
		// No tag-display flag exists anywhere in this grid; the capability
		// table grades the field the ZERO FieldSupport to say the same
		// thing.
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// P12 at byte 39: "0: Lockout OFF / 1: Lockout ON" (890:3205-3207).
		// THE TWO ROWS OF THIS PAIR ENCODE IT DIFFERENTLY — the TS-990S
		// prints 1/2 (erratum E8) — so the neutral field is the boolean and
		// each codec spells it its own radio's way.
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: rec.Lockout},

		// P8 at bytes 25-35, with the split flag P11 at byte 38
		// (890:3191-3192, 890:3201-3203). KNOWN ON EVERY CHANNEL AND NEVER
		// Unavailable (matrix §2.4, M-E7), and that is this pair's largest
		// gain over pair 1: ONE answer carries both frequencies and the
		// flag, where the 590 rows had to publish every channel's TX side
		// Unavailable because the only route to it was a second frame nobody
		// could send blind.
		//
		// THE VALUE IS THE RECORD'S OWN AND NOTHING IS INVENTED FOR A
		// SIMPLEX CHANNEL. The book prints the unsplit case: "When reading a
		// single memory channel, all parameters for Split Transmission
		// become 0" (890:3217-3218) — A16, DOCUMENTED rather than assumed —
		// so a Known zero here IS the record's own statement that this
		// channel has no separate transmit frequency. Publishing the receive
		// frequency instead would assert a byte the radio did not send, and
		// would make a genuinely split channel whose two sides happen to be
		// equal indistinguishable from a simplex one.
		TxFreqHz: codeplug.FreqField{State: codeplug.Known, Value: rec.TXFreqHz},
		// No duplex selector and no offset magnitude anywhere in the grid;
		// a case-insensitive search of this book for "duplex" returns zero
		// hits (§1.18).
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},
		// P5 at byte 20, four values (890:3180-3185). VALUES printed,
		// SEMANTICS assumed — K-D1.
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneMode},
		// P6 at 21-22 and P7 at 23-24, INDEPENDENT transmit and receive
		// indices into the two printed charts (890:3186-3190), which is what
		// makes "3: Cross Tone" expressible at all. THAT P6 IS THE TRANSMIT
		// ONE AND P7 THE RECEIVE ONE IS K-D1 and not this book's.
		ToneTx: toneTx,
		ToneRx: toneRx,
		// DCS appears NOWHERE in this book — zero hits for DCS or DTCS in
		// any command, legend or menu row.
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		// No filter byte in the grid, unlike the TS-590SG's byte 28. This
		// radio HAS per-mode filter selection as FL0-FL3 at radio level
		// (890:2410, 890:2431, 890:2458, 890:2480); a round trip through
		// this programme does not preserve it, and cannot.
		Filter: codeplug.StringField{State: codeplug.Unavailable},
		// M-E3: no DA command and no data byte on this radio. The data-ness
		// is in the mode NAMES — LSB-D, USB-D, FM-D, AM-D at legend values
		// C-F (890:3989-3992) — so a data channel arrives here as a Mode
		// string, not as a flag, and this field has nothing to carry. It is
		// the sharpest single divergence from pair 1, whose 590 rows read
		// this from byte 19.
		DataMode: codeplug.BoolField{State: codeplug.Unavailable},
		// No step field of any kind in the grid, and no ST command anywhere
		// in this book; and no attenuator, preamp, antenna or IP+ position
		// either (RA, PA and AN are radio-level).
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
// core/kw/ma has already bounded both indices to this row's own printed
// charts — P6 to TN's 00-50 and P7 to CN's 00-49 — so a record that reaches
// here carries an index the 51-entry table has an entry for, and the
// out-of-range branch is a refusal rather than a clamp if it ever does not.
// It is also why a 1750 Hz tone_rx cannot come OFF a radio: the CN chart stops
// at 49 (890:1354-1369), so the index that would produce it is refused at the
// parser. The 1750 Hz refusal in the write path is about a value arriving
// from a FILE.
func (s *Session) tone(index int, field string) (codeplug.ToneField, error) {
	if index < 0 || index >= len(ctcssTones890) {
		return codeplug.ToneField{}, fmt.Errorf("%s is %d, outside the 51-entry chart this row publishes (890:5149-5163)", field, index)
	}
	value := ctcssTones890[index]
	if !s.caps.AdmitsTone(value) {
		return codeplug.ToneField{}, fmt.Errorf("%s is %d, which is %v — a tone this row's capability table does not admit", field, index, value)
	}
	return codeplug.ToneField{State: codeplug.Known, Value: value}, nil
}

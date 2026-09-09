// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// readChannelGapHook, when non-nil, is called by ReadChannel with opMu already
// held and before any frame is built — a test-only seam, mirroring
// core/driver/ts590's namesake. Always nil in production, so it costs one nil
// check per read.
//
// It exists for TestReadChannel_IsAtomicUnderOpMu, to park one operation
// INSIDE the lock deterministically rather than relying on goroutine
// scheduling: Go's sync.Mutex favours an immediately-re-locking goroutine so
// heavily that the interleaving opMu exists to forbid is near-impossible to
// reproduce by hammering alone.
var readChannelGapHook func()

// toneModeNames maps MA0 P6's wire byte to the neutral tone_mode vocabulary
// this row publishes (matrix §1.19): "0: FM Tone function OFF for frequency 1
// / 1: Tone / 2: CTCSS / 3: Cross Tone" (990:2915-2919). The strings are
// Capabilities.ToneModes' own, so a Known value this map produces is one
// codeplug.StringField.Valid admits.
//
// WHAT THE FOUR VALUES MEAN IS K-D1, not this map: the values are the chart's
// and only their SEMANTICS are borrowed from a different radio's book (M-E2,
// doc.go). core/kw/ma's checkToneType has already refused any byte outside the
// printed legend before a Record exists, so the miss branch is unreachable in
// practice; it refuses rather than mislabels if it ever is not.
var toneModeNames = map[byte]string{
	'0': "OFF",
	'1': "TONE",
	'2': "CTCSS",
	'3': "CROSS",
}

// ErrUnknownSlot is the sentinel a caller compares against (via errors.Is)
// when a slot identifier is not one this row publishes. The error actually
// returned is an *UnknownSlotError.
var ErrUnknownSlot = errors.New("ts990: slot is not one this row publishes")

// UnknownSlotError reports a slot identifier that is not in this row's one
// bank — whether because it is malformed, because it names a channel outside
// the printed space, or because it names one of the twenty the driver
// deliberately does not publish.
//
// NO FRAME IS SENT. The check is bank MEMBERSHIP, settled entirely from this
// session's own published capabilities, so the radio is never asked.
//
// SLOTS 100-119 ARE REFUSED HERE, AND BY EXACTLY THIS MECHANISM (decisions.md
// row 14, matrix §1.4.2 and §1.4.3). The distinction that makes that honest is
// between the CODEC'S SLOT DOMAIN and the DRIVER'S PUBLISHED BANKS, which are
// two different questions. The book prints all three classes (990:2894-2896),
// so core/kw/ma's layout DECLARES 100-119 and a frame naming one parses rather
// than being refused as malformed. What such a channel HOLDS is another
// matter: what an MA0 read of a section-defined channel answers is A9, what
// its frequency 2 means is A8, and the only thing this book says about an E
// channel anywhere is the number-mapping sentence itself. So the twenty slots
// are not published until those lift — and a read naming one is refused THE
// SAME SHAPE as any other slot this row does not publish, with no invented
// radio behaviour and no special case in the code.
type UnknownSlotError struct {
	// Slot is the identifier that was requested.
	Slot string
	// Reason says which of the ways it failed.
	Reason string
}

// Error implements the error interface.
func (e *UnknownSlotError) Error() string {
	return fmt.Sprintf("ts990: slot %q is not published by the %s: %s", e.Slot, modelName, e.Reason)
}

// Unwrap lets errors.Is(err, ErrUnknownSlot) match.
func (e *UnknownSlotError) Unwrap() error { return ErrUnknownSlot }

// parseSlotID reads a canonical slot identifier as this row's channel number.
//
// IT IS SYNTAX ONLY, and that is deliberate: it is the first rung of the
// ladder, and it must not consult the layout. If it refused an out-of-space
// number itself, "115" would be refused by two different mechanisms — the
// codec's slot space and the driver's published banks — and which one a user
// met would depend on nothing they could see. Bank membership answers for the
// whole space in one shape; see UnknownSlotError.
//
// THE WIRE FORM IS THREE DIGITS AND NO SUFFIX (matrix §1.4.1). The book prints
// P1 as three cells over a three-digit domain (990:2893-2895), and — unlike
// pair 1's rows — there is no section-channel half to spell, because this
// family has no P1 selector for one. It is the exact form ma.Slot.String
// renders, which is what the bank inventory is built from, so the two cannot
// drift.
func parseSlotID(id string) (int, error) {
	if len(id) != 3 {
		return 0, fmt.Errorf("slot %q: a slot identifier on this row is three digits (990:2893-2895)", id)
	}
	n := 0
	for _, b := range []byte(id) {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("slot %q: the channel number is three decimal digits", id)
		}
		n = n*10 + int(b-'0')
	}
	return n, nil
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

// ma0Spec is the transport spec for one MA0 read of slot: an EXACT
// prefix-and-length matcher, and one retry.
//
// THE WHOLE CORRELATION KEY SPELLS AS A PREFIX, which is why this row needs no
// matcher code at all (matrix §3.8). core/kw's MRAnswerMatcher decodes the slot
// because the 590's channel number sits after P1, so no prefix a caller can
// spell separates one channel's answer from another's; here the number is
// bytes 4-6, immediately after the opcode (990:2916-2918), and the answer's
// width is nailed at 57 (990:2938) — so kw.PrefixLenMatcher is exact and is
// used as it stands. PrefixLenMatcher's exactLen <= 0 branch is deliberately
// NOT used, and core/kw/doc.go says why; the sibling row is the one that needs
// a length RANGE, because its terminator floats.
//
// IT RESTS ON A5: that the radio spells the channel back as three zero-padded
// digits and never space-pads it the way the 590SG's MC does. THE FAILURE
// DIRECTION IS SAFE — a space-padded answer MISSES the matcher and the read
// times out; it cannot be mis-attributed to another channel.
//
// A frame of any width but 57 is not this read's answer at all, so it never
// reaches the parser and the read times out instead. That is the right order —
// the frame the radio sent is not the frame this command asked for — and the
// codec stays the authority on what a memory frame IS
// (TestReadChannel_AMisSizedAnswerNeverReachesTheParser pins both halves).
//
// PAIR 1'S ANSWER-MISMATCH REFUSAL HAS NOTHING TO CATCH HERE, and that is
// worth stating because its absence otherwise reads as a gap.
// core/driver/ts590 compares the answered slot against the requested one — and
// a second time on the P1 byte its identifier cannot carry — because the 590's
// matcher decodes a channel number that sits AFTER P1, so a frame can be
// correlated and still name the wrong half. On this row the correlation key IS
// the whole identifier: a delivered frame's bytes 4-6 are the three digits the
// prefix compared, so a comparison against them is a tautology, and a rung
// whose red proof cannot be written honestly is not a rung. What refuses an
// answer naming another channel is this matcher, by never delivering it —
// TestReadChannel_AnAnswerNamingAnotherSlotIsNeverDelivered.
//
// ONE RETRY, and it is the ordinary reasoning: a read is idempotent and a
// single swallowed reply should not fail a whole-radio read of a hundred
// slots. The retry is preceded by the engine's own drain-to-quiet quarantine,
// so it cannot correlate a stale answer with the second attempt. What a retry
// does NOT do is turn silence into information: a read that still draws
// nothing fails the session read WHOLE, with the typed error
// kw.NewTimeoutError builds, because the book says the NAK is unreliable and a
// timeout is therefore neither "absent" nor "rejected".
func (s *Session) ma0Spec(slot ma.Slot) transport.CommandSpec {
	return s.newReadSpec(s.layout.MA0AnswerMatcher(slot), 1)
}

// ReadChannel implements driver.Session: ONE MA0 Read frame per slot, and
// nothing else (plan P12, matrix §3.8).
//
//   - THE FRAME CARRIES ITS OWN CHANNEL NUMBER AND NO MN PRECEDES IT, which is
//     A18 — the load-bearing read assumption of the whole milestone. Both Read
//     forms in this family carry their own number, so the command is stateless
//     on its face; but MA1, MA7 and MI all act on "the channel appointed when
//     using this command", so the family HAS state and MA0 Read's independence
//     from it is nowhere printed. Decision 5's removal of MN from the outbound
//     roster PROMOTES the assumption rather than withdrawing it: with no MN
//     builder this driver could not select a channel even if it had to. IF A18
//     IS FALSE, NO READ ON THIS ROW WORKS AT ALL, and its lift is hardware item
//     10 (L-HW-12) — from VFO mode, with no MN sent, read channel 005.
//   - NO DISCOVERY FRAME OF ANY KIND IS EVER BUILT (matrix §3.4), on two
//     independent grounds: the book says the NAK is unreliable, so a probe's
//     silence carries no information; and the slot space is fully printed, so a
//     probe would ask a question the book answers.
//
// THE WHOLE OPERATION IS HELD UNDER opMu (P12/P13). transport.Engine serialises
// each individual exchange, not a whole driver operation, and this session's
// operations are not all single exchanges; holding the lock for every one of
// them, rather than only for the several-frame ones, is what makes the rule
// "one driver operation at a time" rather than "one driver operation at a
// time, except the short ones".
//
// FAILURE MODES, and the design's reading of each:
//
//   - An UNKNOWN SLOT is refused before any frame is built — *UnknownSlotError,
//     which is also how the twenty unpublished upper-class slots are refused.
//   - A "?;" IS A DEFINITIVE REJECTION, NEVER "absent". The book prints it as
//     EITHER a syntax error OR a command not executed in the transceiver's
//     current status — indistinguishable — so it is reported as the typed
//     kw.RejectionError and it fails the session read WHOLE.
//   - A TIMEOUT is the typed kw.TimeoutError, which says in as many words that
//     it is not an inference of absence. It fails the session read whole too.
//   - AN UNASSIGNED CHANNEL IS NOT A FAILURE AND IS NOT A REJECTION: an answer
//     whose BYTES 7-56 are blank — P2 to P18, which on this row is the WHOLE
//     record and includes the name window — is the documented blank channel of
//     990:2962-2963, and is reported as codeplug.Channel{Slot: id} with a nil
//     Data, which is the driver seam's own spelling of "this slot is empty".
//     The sibling row's note stops at P12 and leaves its name window
//     unspecified, which is why it has a residue arm and this one has none.
//     The predicate is
//     the codec's and runs before any per-field domain parse, which is what
//     stops a blank P17 — a byte MA0 prints only as '1' or '2' — raising on
//     every unused slot of a fresh radio.
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
		return codeplug.Channel{}, fmt.Errorf("ts990: ReadChannel %s: %w", id, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.ma0Spec(slot))
	if err != nil {
		// wireFailure is what types a "?;" and a timeout — see there, and
		// see this function's own doc comment for what each means. It is
		// shared with the probe so one rule is applied once.
		return codeplug.Channel{}, fmt.Errorf("ts990: ReadChannel %s: %w", id, wireFailure("MA0", err))
	}

	rec, err := s.layout.ParseMA0Answer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts990: ReadChannel %s: %w", id, err)
	}
	if rec.Empty {
		// The blank-channel note covers P2 TO P18 on this row — the name
		// window included (990:2962-2963) — so unlike the sibling row there
		// is no residue to carry in the read's detail. Data nil is the
		// seam's "this slot is empty".
		return codeplug.Channel{Slot: id}, nil
	}

	data, err := s.channelData(rec)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts990: ReadChannel %s: %w", id, err)
	}
	return codeplug.Channel{Slot: id, Data: data}, nil
}

// channelData maps one parsed 57-byte record onto the neutral channel model.
//
// THE SECONDARY SIDE IS READ, DOMAIN-CHECKED AND PUBLISHED NOWHERE, and that
// is matrix §2.7 — the pair's largest loss and the mirror of §2.4's gain.
// codeplug.ChannelData has ONE mode, ONE tone tuple and, for a second
// frequency, only TxFreqHz. This record has more: frequency 2 carries a
// complete tuple of its own — mode P10, FM width P11, tone function P12, tone
// P13, CTCSS P14 (990:2929-2945) — plus the dual-reception flag P16
// (990:2949-2951), which names no field in the neutral model at all. The codec
// parses and domain-checks every one of them; this function then has nowhere
// to put five of them, AND A READ DOES NOT FAIL ON THAT, because refusing to
// read a channel the user can see on the front panel would be worse than
// reporting the part the model holds. The consequence is confined to the write
// path: such channels are READABLE but NOT REWRITABLE, and T14's rung 11 is
// where that refusal quotes the P-numbers and both values.
//
// TX_FREQUENCY IS KNOWN ON EVERY CHANNEL, and this is the pair's largest gain
// over pair 1 (§2.4). The 590 rows had to publish it Unavailable because a
// second frame nobody could send blind was the only route to it; here ONE
// answer returns both sides and the flag, so the split disposition is never
// guessed. WHAT DECIDES IT IS P15 AND NOT THE MERE PRESENCE OF A FREQUENCY 2:
// "0: Simplex / 1: Split" (990:2946-2948), and a DUAL-RECEPTION channel
// carries a live frequency 2 with P15 = 0 whose second side is a sub-band
// RECEIVE frequency. Publishing that as tx_frequency would put a receive
// frequency in a transmit field, which is the same class of mistake pair 1's
// M-E2 caught one book over. Simplex therefore reports Known 0 — "this channel
// stores no independent transmit frequency" — which is a fact the answer
// carries, not an absence.
//
// THE MODE NAME IS modeDisplayName's, the same function the capability list
// asks, so the name a read produces is byte-for-byte one Capabilities().Modes
// advertises. On this row it is a function of TWO bytes: P4 is the OM P2
// legend value and P5 is "0: FM Wide / 1: FM Narrow for frequency 1"
// (990:2912-2914), orthogonal to it.
func (s *Session) channelData(rec ma.Record) (*codeplug.ChannelData, error) {
	mode, ok := modeDisplayName(s.layout, rec.Mode, rec.FMNarrow)
	if !ok {
		// Unreachable after core/kw/ma's own parse: checkModeByte refuses a
		// byte outside this row's legend, which is where '0' and '8' —
		// both printed "Unused" (990:3707, 990:3715) — are already gone.
		// Refuse rather than publish a channel whose mode this row does not
		// name.
		return nil, fmt.Errorf("unmapped mode byte %q with the FM width flag %v", rec.Mode, rec.FMNarrow)
	}
	toneMode, ok := toneModeNames[rec.ToneType]
	if !ok {
		// Unreachable after core/kw/ma's checkToneType; see toneModeNames.
		return nil, fmt.Errorf("unmapped tone function %q", rec.ToneType)
	}
	toneTx, err := s.tone(rec.ToneIndex, "P7, the TN index")
	if err != nil {
		return nil, err
	}
	toneRx, err := s.tone(rec.CTCSSIndex, "P8, the CN index")
	if err != nil {
		return nil, err
	}

	// P15 decides; see the doc comment.
	txFreq := uint64(0)
	if rec.Split {
		txFreq = rec.TXFreqHz
	}

	return &codeplug.ChannelData{
		// P3, eleven digits at bytes 8-18 (990:2905-2906).
		FreqHz: rec.FreqHz,
		// P4 x P5; see the doc comment.
		Mode: mode,

		// The eighteen parameters account for every byte of the grid
		// (990:2891-2956) and NONE of them is an RIT/XIT offset — M-E4.
		// These three members are plain scalars with no state to say so, so
		// the honest reading is their zero value, which is also what a write
		// path reads as "the caller set nothing here". It does NOT mean this
		// radio has no clarifier: it has RIT and XIT as radio-level settings
		// no memory channel stores (990:4252, 990:5210).
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		// The Yaesu half of the vocabulary pair (decision 6): ctcss_state is
		// displaced by tone_mode below, and this record has no shift
		// selector at all — split is an absolute second frequency.
		CTCSS: "",
		Shift: "",
		// The Yaesu tone INDEX field, which is ONE field where this record
		// carries TWO independent indices on the primary side alone; the
		// values live on ToneTx and ToneRx.
		CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},

		// P18, the fixed ten-byte window at 47-56, right-trimmed of the
		// spaces the codec pads it with — A1, assumed rather than printed.
		Tag: rec.Name,
		// No tag-display flag exists anywhere in this record; the capability
		// table grades the field the ZERO FieldSupport to say the same
		// thing.
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// P17 at byte 46, and the codec has already refused any byte but
		// '1' and '2' — this radio's own encoding, where its MA3 uses 0/1
		// (erratum E8, matrix §2.9).
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: rec.Lockout},

		// P9 with P15; see the doc comment. NEVER Unavailable on this row
		// (M-E7), and never produced at all for a slot in 100-119, because
		// no channel is ever produced for one.
		TxFreqHz: codeplug.FreqField{State: codeplug.Known, Value: txFreq},
		// A case-insensitive search of this book for "duplex" returns zero
		// hits, and no per-channel offset magnitude exists in the grid.
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},
		// P6 at byte 21, four values (990:2915-2919). Their SEMANTICS are
		// K-D1 (doc.go), not this line.
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneMode},
		// P7 at 22-23 and P8 at 24-25, INDEPENDENT transmit and receive
		// indices into the printed charts — which is what makes "3: Cross
		// Tone" expressible at all.
		ToneTx: toneTx,
		ToneRx: toneRx,
		// DCS appears nowhere in this book.
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		// No filter byte in this grid, unlike the 590SG's byte 28; FL0-FL3
		// are radio-level (§1.22, M-E5).
		Filter: codeplug.StringField{State: codeplug.Unavailable},
		// M-E3: this radio has no DA command and this grid has no data byte
		// — the data-ness is INSIDE the mode legend, which is why a
		// data-mode channel comes back as "LSB-D2" and not as LSB with a
		// flag. Reporting a Known false here would say "this channel is not
		// a data channel", which the record does not say.
		DataMode: codeplug.BoolField{State: codeplug.Unavailable},
		// P5 is the FM width flag, folded into the mode NAME above rather
		// than published as a step; no step magnitude, no on/off flag, and
		// no attenuator, preamp, antenna or IP+ position exists in this
		// record — and this radio has no ST command at all.
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
// core/kw/ma has already bounded both indices to their own printed charts — P7
// to TN's 00-50 and P8 to CN's 00-49 — so a record that reaches here carries an
// index this row's 51-entry table has an entry for, and the out-of-range branch
// is a refusal rather than a clamp if it ever does not. It is also why a
// 1750 Hz tone_rx cannot come off a radio: CN's chart stops at 49
// (990:1251-1264), so the index that would produce it is refused at the parser.
func (s *Session) tone(index int, field string) (codeplug.ToneField, error) {
	if index < 0 || index >= len(ctcssTones) {
		return codeplug.ToneField{}, fmt.Errorf("%s is %d, outside the 51-entry chart this row publishes (990:4960-4972)", field, index)
	}
	value := ctcssTones[index]
	if !s.caps.AdmitsTone(value) {
		return codeplug.ToneField{}, fmt.Errorf("%s is %d, which is %v — a tone this row's capability table does not admit", field, index, value)
	}
	return codeplug.ToneField{State: codeplug.Known, Value: value}, nil
}

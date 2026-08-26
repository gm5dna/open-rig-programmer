// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7300 is the Icom IC-7300 driver: ONE driver package for ONE
// registered radio model, over the CI-V codec in core/civ and this model's
// own profile in core/civ/ic7300.
//
// ONE MODEL PER PACKAGE, and the IC-7300MK2 has its own
// (core/driver/ic7300mk2). That is the OPPOSITE of core/driver/ftdx101,
// which drives two radios from one package — and the difference is
// evidential, not stylistic. Yaesu prints ONE CAT manual for the FTDX101D
// and the FTDX101MP and distinguishes them in three places. Icom prints two
// entirely separate documents here, and each is SILENT ABOUT THE OTHER
// RADIO: the IC-7300's 180-page full manual contains no occurrence of MK2,
// MKII, MK-2 or Mark II anywhere (matrix §4), and the MK2's 27-page CI-V
// Reference Guide never mentions the IC-7300 (its own §4). Both matrices'
// §4 close with the same rule in terms: no assumption in one may be read as
// covering the other model, and no lift in one lifts anything for the
// sibling. A parameterised driver would carry that separation as a table
// and invite exactly the borrowing the two documents forbid.
//
// It therefore does NOT import core/driver/ic7300mk2, nor any other driver
// package, and neither imports this one. Where a decision here looks like
// the MK2's, the comment at the decision says whether the agreement is a
// manual fact of THIS radio, a structural requirement, or an assumption —
// in which case it is in the register below, scoped to this model, naming a
// capture ON AN IC-7300.
//
// # Provenance
//
// Everything protocol-shaped here comes from the IC-7300 FULL MANUAL,
// through core/civ/ic7300's profile. That package's doc.go carries the
// wire-level citations, the record geometry, the name policy and the
// PROFILE half of the ASSUMED register; this package cites it and
// duplicates none of it. The per-value evidence status of everything this
// driver claims is
// docs/superpowers/icom-matrices/ic7300-capability-matrix.md, field by
// field.
//
// NO IC-7300 HAS EVER BEEN ASKED ANYTHING by this project, and nothing in
// this package has been sent to a radio. There is no docs/hardware-notes.md
// section for this model, no golden vector captured from one, and no
// Stage R or Stage W session. Green tests here mean two independently
// written readings of one manual agree with each other and with this code.
// The register below is where that is written down rather than glossed.
//
// # The write guard
//
// writeTrialsComplete is FALSE (caps.go) and pinned false by its own test,
// ROW BY ROW: the pin walks every bank and every one of the twenty
// spec.Fields and requires CanWrite() false on the RealHardware profile. A
// RealHardware session therefore gets the all-Unverified capability set —
// every field the 1A 00 record carries graded spec.Unverified, which is
// documented-but-unproven and unwritable — so, UNLESS THE USER HAS
// CONSENTED, codeplug.Diff blocks every change, the clone service refuses
// to execute one, and WriteChannel's own capability re-check refuses before
// any frame is built. An unrecognised Profile value fails the same way (see
// ic7300Driver.Capabilities): the failure direction for a forged or
// corrupted Profile is always "nothing writable".
//
// THE ONE ROUTE PAST THE GUARD IS THE USER'S RECORDED CONSENT. A
// RealHardware session opened with WithConsentedUnverifiedWrites carries
// spec.ConsentedUnverified where the profile said spec.Unverified, and
// CanWrite() opens. Consent widens WHAT may be attempted and never HOW
// carefully: every refusal in the ladder below still applies, and erase is
// structurally exempt from the transform (spec.ConsentUnverifiedWrites
// never consents FieldErase).
//
// Flipping writeTrialsComplete is a HARDWARE milestone with evidence, ON AN
// IC-7300. Matrix §3.14 states the FALSE for this model alone.
//
// # The refusal ladder, and its ONE recorded exception (ruling T5)
//
// driver.Session's contract says a write is refused BEFORE ANY WIRE
// TRAFFIC. This driver honours that for every refusal that is LOCALLY
// DECIDABLE, and it names the one that cannot be. WriteChannel's rungs, in
// order:
//
//  1. the slot parses;
//  2. the slot is in a bank this radio has;
//  3. the channel is EMPTY — an erase request, refused naming
//     spec.FieldErase. Structurally third: an empty channel has no Data at
//     all, so every FieldState check below it would dereference nil;
//  4. field validity and vocabularies;
//  5. the CanWrite() gate over the requested-field set below;
//  6. the mandatory-Known rules (a non-Known value for a field the record
//     cannot omit is REFUSED, never synthesised);
//
// — every rung above is locally decidable and precedes ALL wire traffic,
// and each of their tests asserts a TRANSCRIPT DELTA OF ZERO —
//
//  7. ONE read: the preservation read of the slot;
//  8. the answer's channel-address check, then the THREE READ-DEPENDENT
//     refusals — the CREATE (an empty slot has no SELECT value to preserve),
//     E6's unmapped-region (split-flag) mismatch, and the SCAN bank's
//     printed ③-must-be-zero constraint;
//  9. the set.
//
// RUNG 8 IS THE SINGLE RECORDED EXCEPTION to "before any wire traffic", and
// core/driver/driver.go's Session contract names it: a refusal that depends
// on the SLOT'S CURRENT STATE cannot precede the read that obtains that
// state. Nothing else in this driver refuses after a byte has moved.
//
// THE SCAN CONSTRAINT IS READ-DEPENDENT, and that is why it sits at rung 8
// rather than among the locally decidable ones. It judges record byte ③,
// whose SELECT nibble NO spec.Field carries at all (plan decision D4) — so
// the only place its value exists is the record the radio just handed back,
// exactly as for E6's own check. Checking it earlier would mean judging a
// datum before the read that produces it.
//
// # The requested-field set: this driver's write-gate contract
//
// WriteChannel re-derives, from the channel itself, which spec.Fields a
// write actually REQUESTS, and gates each against this session's
// capabilities. NINETEEN of the twenty spec.Fields can appear; the
// twentieth, spec.FieldErase, deliberately cannot, and its absence is the
// design rather than an oversight.
//
// UNCONDITIONAL (7) — the fields the 1A 00 record ALWAYS carries, and this
// driver therefore always encodes, changed or not:
//
//	frequency, mode, tag, tx_frequency, filter, data_mode, tone_mode
//
// The Yaesu trio (clarifier, ctcss_state, shift) is NOT among them: this
// model's §2 grid grades all three Unsupported, and a driver that requested
// them would refuse every write it could ever make.
//
// CONDITIONAL (12), appended in codeplug.ChannelData's own declaration
// order, each with the predicate its REAL representation permits — because
// several of these members are PLAIN SCALARS with no FieldState, and a
// ".State == Known" predicate could not have been written for them:
//
//	clarifier      ClarHz int / RxClar / TxClar  ClarHz != 0 || RxClar || TxClar
//	ctcss_state    CTCSS string                  CTCSS != ""
//	ctcss_tone     CTCSSTone ToneField           .State == Known
//	shift          Shift string                  Shift != ""
//	tag_display    TagDisplay BoolField          .State == Known
//	scan_skip      ScanSkip BoolField            .State == Known
//	duplex         Duplex StringField            .State == Known
//	offset         OffsetHz FreqField            .State == Known
//	tone_tx        ToneTx ToneField              .State == Known
//	tone_rx        ToneRx ToneField              .State == Known
//	dtcs_code      DTCSCode IntField             .State == Known
//	dtcs_polarity  DTCSPolarity StringField      .State == Known
//
// For the three scalar-backed rows the predicate answers "did the caller
// actually set this?", which is the only question a scalar can answer. A
// channel this driver produced answers false to all three, so an ordinary
// write is unaffected; a channel loaded from a file written for a DIFFERENT
// radio is caught and refused BY NAME rather than silently dropped.
//
// tone_tx and tone_rx are CONDITIONAL rather than mandatory-Known, and that
// is ruling T1(4): a non-Known tone is PRESERVED from the just-read record's
// own bytes, so requesting the field would ask the gate about a value the
// caller never set.
//
// spec.FieldErase has NO codeplug.ChannelData member at all — erasure IS
// Channel.Data == nil — so it cannot be derived from a channel's content
// and belongs to rung 3, not to the field gate.
//
// # Split is REFUSED, not cleared, and that cost is accepted (E6, D14)
//
// Record byte ③ carries the SELECT group in its low nibble and the SPLIT
// flag in its high one. The profile maps the low nibble and leaves the high
// one UNMAPPED, under a full-length all-zero Fixed template. That is
// deliberate: civ's decodeRecord ignores every nibble no span maps, and
// encodeRecord writes the template there — so a driver that neither carried
// ③'s high half through nor refused would SILENTLY CLEAR a user's split
// flag on every write, and no layer above could see it happen.
//
// Enabler ruling E6 binds every write: a slot may be written ONLY when the
// record's unmapped regions equal the profile's Fixed template. On this
// model the unmapped region is exactly ③'s high nibble, so:
//
//   - a Split-OFF channel writes normally, its SELECT nibble carried
//     through unchanged from the record the radio holds;
//   - a Split-ON channel READS normally — frequency, mode, name and both
//     tones all map — but CANNOT BE WRITTEN BACK. The write is refused,
//     loudly, naming the split flag and the slot.
//
// That is the cost, stated and accepted for this tier: refused, not
// corrupted. It belongs in the user-facing README honesty rows as well as
// here.
//
// # An empty slot cannot be CREATED, and that too is a refusal
//
// A write to a slot the radio reports empty has nothing to preserve. Two
// rungs then have no honest source: ③'s SELECT nibble, which NO spec.Field
// carries (plan decision D4 forbids mapping it as scan_skip), and, behind
// it, the two tone spans, for which this document prints no default. Only a
// Known value is ever encoded and nothing is synthesised, so a create
// REFUSES rather than inventing either. Register entries
// `ic7300-select-nibble-on-create` and
// `ic7300-documented-default-tone-absent` below.
//
// # The duplicated TX block, and what it costs the user
//
// The record carries ④–⑰ twice: once as the receive block and once, as
// ❹–⓱, as the transmit block used when Split is ON. The transmit FREQUENCY
// is a distinct field (❹–⑧ ↔ spec.FieldTxFrequency), so a split channel
// round trips. The other NINE of those fourteen bytes — TX mode 1, TX
// filter 1, TX data/tone 1, TX repeater tone 3, TX tone squelch 3; 5 + 9 =
// 14 — have no neutral codeplug field of their own and reuse their RX
// copies' field ids. Two consequences travel to the user, not only to this
// comment:
//
//   - A CODEPLUG CANNOT REPRESENT a channel whose TX mode, filter, data
//     mode or tones differ from its RX ones. This driver always mirrors the
//     RX side into the TX block, which is what both manuals recommend
//     ("We recommend that you set the same data as ④–⑰").
//   - A RADIO WHOSE TX BLOCK DISAGREES WITH ITS RX BLOCK FAILS THE WHOLE
//     READ. civ's decodeRecord checks duplicated spans for agreement rather
//     than letting the last copy win, so such a record returns a
//     *civ.ParseError and ReadAll aborts. Refusing beats silently keeping
//     one copy, but a user whose radio holds such a channel sees a read
//     that stops.
//
// # An unprinted enum value fails the read (plan decision D12)
//
// Mode code 06 is printed NOWHERE in this document (matrix §3.16 A7) and no
// value is invented for it. civ's decodeRecord has no "unknown enum"
// representation, so a record carrying 06 in the mode byte fails with a
// *civ.ParseError naming the byte and its offset, ReadChannel returns that
// error and ReadAll aborts. That is spec D4's own rule for malformed
// records applied one level down. The matrices call it "an unknown mode";
// in this code it is a parse error, and the two are the same event.
//
// # The wrong-sibling fingerprint, and what it does NOT protect against
//
// The IC-7300 answers at 94h and the IC-7300MK2 at B6h. IN THE FIELD THE
// TWO CANNOT CONFUSE EACH OTHER: a wrong sibling on this port simply does
// not answer, and the open times out. The record-length fingerprint spec
// D3.2 requires protects against SAME-ADDRESS confusion only — a radio
// moved onto this address, or a bus mis-set — and that is the case the
// probe's length check exists for.
//
// This driver therefore carries a ONE-ENTRY foreign-length table: 45 bytes
// is attributed to "IC-7300MK2 (provisional)". It is a HINT, not a
// distinctness claim. BOTH lengths — this model's 39 and the sibling's 45 —
// are ASSUMED derivations from printed field widths (neither document
// prints a total), so the attribution is rendered with the word
// *provisional* and with both numbers named. Cross-model record-length
// distinctness is a TIER-level check belonging to registration, and it is
// what may add or correct entries here; nothing in this package claims it.
//
// # Control lines at open
//
// core/transport.OpenSerial drives RTS and DTR LOW immediately after
// opening the port (safety obligation 4, core/transport/port.go:148-155,
// core/transport/doc.go:261-262), and this driver asserts neither. On this
// radio `USB SEND`, `USB Keying (CW)` and `USB Keying (RTTY)` can each bind
// DTR or RTS on the CI-V port to the TRANSMITTER, and all three ship OFF —
// MANUAL-EVIDENCED (matrix §3.16 A5, PDF p.127). That the factory defaults
// make a deasserted open harmless is the EVIDENCE; that a driver may open
// at all without keying a radio the user has configured for `USB SEND` is
// ASSUMED. Every capture brief for this model must record those three
// items' values plus `CI-V USB Port` BEFORE the port is opened.
//
// # Serial framing
//
// StopBits() returns 1 (ic7300.go), on the CONCRETE DRIVER rather than the
// session, because internal/wiring holds the driver before any port exists.
// It is ASSUMED at spec D5 entry 8. THE HAZARD, repeated wherever the claim
// appears: this document prints no character format anywhere (matrix §3.1),
// and an Icom manual's "8 bit / 1 stop" line about the DATA/RTTY port is
// NOT evidence about CI-V. The two are different ports.
//
// # Erase: two printed forms, neither shipped
//
// spec.FieldErase carries the ZERO FieldSupport on both banks even though
// this document prints TWO clear forms — a 1A 00 set whose ③ is "FF" with
// no further data (PDF p.169's right-hand column), and command 0B. Neither
// is implemented, and spec.ConsentUnverifiedWrites structurally refuses to
// consent erase whatever a profile says. What a future write-trial
// milestone would need, as matrix §3.13 lists it:
//
//  1. which of the two forms the radio actually accepts, and whether the
//     other is refused or silently ignored;
//  2. what a clear of an ALREADY-EMPTY channel answers (FB, FA, or
//     silence);
//  3. whether a cleared channel then reads back as FA or as an all-FF
//     record — the two halves of D5 entry 2 are still unseparated;
//  4. whether P1/P2 accept a clear at all;
//  5. whether a clear disturbs any neighbouring channel.
//
// # THE ASSUMED REGISTER — this driver's half
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION (core/civ/doc.go's
// convention). The PROFILE half — the read-request form, the name space and
// pad, the mandatory full record, the wire order, the record length, the
// open rate and the symbol codes — is in core/civ/ic7300/doc.go and is not
// repeated here.
//
// Every entry below is scoped to the IC-7300 ALONE and names ONE capture ON
// AN IC-7300. Nothing here covers the IC-7300MK2. The rows whose home is
// "D5 entry N" are THIS MODEL'S INSTANCE of the tier-wide recurring entry.
//
//   - empty-channel read answers FA — D5 entry 2(a). That a read of an
//     unwritten channel draws FE FE E0 94 FA FD rather than a record is
//     ASSUMED: this document prints no answer for the empty case. Every
//     read path in this driver treats transport.ErrRejected — which is what
//     Engine.Do returns when it CONSUMES an FA — as an empty channel, never
//     as an error.
//     LIFT: Stage R capture `ic7300-empty-read` — read a channel known to
//     be unwritten and record exactly what comes back.
//
//   - an all-FF record means empty — D5 entry 2(b), graded separately
//     because neither capture can lift the other. ReadChannel recognises an
//     all-0xFF record as an EMPTY channel before the record parser sees it.
//     The recognition is EXACT: a record that is all-FF except one byte is
//     NOT empty and is parsed like any other, which is what stops the check
//     drifting into a heuristic.
//     LIFT: Stage R capture `ic7300-ff-record` — read a channel known to be
//     unwritten and record whether the answer is an FA or a record of FFs.
//
//   - `19 00` reply value — D5 entry 7. The reply's data byte is
//     undocumented. This driver RECORDS it (Identity.CATID becomes
//     "94:<token>") and COMPARES IT AGAINST NOTHING: what identifies the
//     radio at this step is that an ADDRESS-MATCHED reply arrived at all.
//     LIFT: Stage R capture `ic7300-id-token` — send 19 00 and record the
//     answer's data bytes verbatim.
//
//   - serial framing 8-N-1 — D5 entry 8. See "Serial framing" above for the
//     DATA/RTTY hazard.
//     LIFT: Stage R capture `ic7300-framing` — open at 8-N-1 and at 8-N-2
//     and record which answers 19 00.
//
//   - transceive broadcast address form `to = 00` — D5 entry 9. That a
//     transceive broadcast carries 00 in its `to` byte, and is therefore
//     excluded by civ.FrameAccumulator's address filter rather than by
//     writing a setting to the radio, is ASSUMED. NOTHING IN THIS TIER
//     SENDS A TRANSCEIVE-OFF: Framing.InitSequence() is empty and this
//     driver's Open writes nothing before its identity read.
//     LIFT: Stage R capture `ic7300-broadcast-to-byte` — leave transceive
//     at its factory setting, turn the VFO knob, and record the address
//     bytes of the frames that appear.
//
//   - `ic7300-remote-bus-echo` — whether a frame this program writes comes
//     straight back on the CI-V line. The adapter suppresses an echo by
//     BYTE IDENTITY against what NoteSent recorded, never by position or by
//     count, so the driver is correct either way; the entry records that
//     which behaviour occurs is unestablished.
//     LIFT: Stage R capture `ic7300-remote-echo` — send 19 00 over the USB
//     CI-V port and over the [REMOTE] jack and record, for each, whether
//     the sent frame is read back before the answer.
//
//   - `ic7300-write-ack-form` — that an accepted 1A 00 SET draws the exact
//     six-byte FB and a refused one draws FA. This driver waits for the FB
//     with transport.ClassWriteWithAck, and the matcher is
//     SOURCE-ADDRESS-CHECKED so another station's FB on a shared bus is
//     never read as ours. A set is NEVER retransmitted on timeout.
//     LIFT: Stage W capture `ic7300-set-ack` — send a valid set and an
//     invalid one and record what each draws.
//
//   - `ic7300-tx-block-independence` — whether the radio HONOURS the
//     transmit block's own mode/filter/tone bytes when Split is ON, or
//     ignores them in favour of the receive block's. This driver always
//     mirrors, so nothing it writes can distinguish the two; the entry
//     records that the question is open and that the mirroring is a policy
//     rather than a finding.
//     LIFT: Stage W capture `ic7300-split-tx-write` — write a channel whose
//     TX block deliberately differs, read it back, and record whether the
//     difference survived.
//
//   - `ic7300-scan-edge-record-shape` — that a P1/P2 record has the SAME 39
//     bytes as a memory channel's. This document does not say so. It is why
//     the open probe is confined to MEM channels 1..8: a short or FA answer
//     from 1A 00 01 00 would be a fact about the scan-edge bank rather than
//     a fault, and the probe must not learn the length fingerprint from a
//     record whose layout is not established.
//     LIFT: Stage R capture `ic7300-scan-edge-record` — read 1A 00 01 00
//     and count the record bytes.
//
//   - `ic7300-scan-edge-noblank` — whether P1 and P2 may be blank at all.
//     This document says nothing about it, so spec.Bank.NoBlank is FALSE on
//     both banks here. The MK2's document DOES say its P1/P2 cannot be
//     cleared, and that sentence is the MK2's alone.
//     LIFT: Stage R capture `ic7300-scan-edge-read` — read P1 and P2 on a
//     factory-reset radio and record whether either answers empty.
//
//   - `ic7300-select-memory-as-scan-skip` — RECORDED AS A DEVIATION AND
//     REFUSED. Matrix §2 row 9 chooses to map ③'s SELECT nibble onto
//     spec.FieldScanSkip. The tier's hard constraint overrides it: SELECT
//     on Icom is group MEMBERSHIP, the inverse of a skip flag, and the
//     mapping is forbidden (plan decision D4). The MK2 matrix's §3.16 A10
//     reaches the same place on its own reading. The value is not
//     discarded — it round-trips inside the civ record on civ.FieldSelect.
//     Proposed IC-7300 matrix erratum.
//     LIFT: Stage W capture `ic7300-scan-edge-select-write` — set a channel
//     into SEL1 from the panel, read the record, and record ③.
//
//   - `ic7300-control-lines-at-open` — that CI-V works with RTS and DTR
//     both deasserted, on a radio whose three USB keying items are at their
//     printed factory OFF. See "Control lines at open" above.
//     LIFT: Stage R capture `ic7300-open-control-lines` — record the three
//     items' values and `CI-V USB Port`, then open with both lines low and
//     record whether 19 00 is answered and whether the radio keys.
//
//   - `ic7300-frequency-ceiling` — that a MEMORY CHANNEL cannot store more
//     than 69 999 999 Hz, which is what the record's printed digit ranges
//     imply and what MaxFreqHz publishes. The 70 MHz transmitter band is
//     version-dependent (PDF p.150 footnote *2), and the coverage ceiling
//     of 74 800 000 Hz is a different claim about a different thing.
//     LIFT: Stage R capture `ic7300-70mhz-read` — on a version with the
//     70 MHz band, store a 70 MHz channel from the panel, read it back and
//     record the five frequency bytes.
//
//   - `ic7300-select-nibble-on-create` — that a channel this program
//     CREATES has no honest SELECT value to write. No spec.Field carries
//     one (D4), so the create is refused rather than defaulted to OFF.
//     Defaulting would write a group membership the user never chose.
//     LIFT: Stage W capture `ic7300-select-nibble-create` — write a 1A 00
//     set to an empty channel with ③ = 00, read it back, and record whether
//     the radio accepted it and what ③ then holds.
//
//   - `ic7300-zero-tone-means-unset` — that a record's 00 00 00 in a tone
//     span means "no tone was ever set" rather than a 0.0 Hz tone. The
//     declared tone DOMAIN starts at 1 deciHz because 0 Hz is not a tone,
//     so a read maps an out-of-domain tone number — zero included — to
//     Unknown rather than handing up a Known value codeplug.ToneField.Valid
//     would then refuse (ruling T1(3)).
//     LIFT: Stage R capture `ic7300-zero-tone-read` — read a channel whose
//     tone has never been set and record the three tone bytes.
//
//   - `ic7300-documented-default-tone-absent` — that this document prints
//     NO default tone for the 1A 00 record. The only 88.5 Hz in this
//     model's evidence is the golden leg's own chosen sample value, which
//     is a vector's content and not a radio's default. So a CREATE has no
//     tone to write and refuses, naming the field. NOTE THE ORDERING: a
//     create already refuses one rung earlier, on the SELECT nibble, so the
//     tone rung is currently unreachable for creates — recorded so that a
//     later resolution of the SELECT question cannot silently enable an
//     unsourced tone.
//     LIFT: a matrix erratum recording a printed default would flip this
//     entry and make the rung live.
//
// # Non-goals
//
// No registration. Neither internal/wiring's driver tables nor
// SupportedModels is touched by this package; registration is a separate
// commit on the integration branch. No re-export of civ frame machinery
// outside this package. No ReadAll — that is clone.Service's, walking
// Session.Capabilities().Banks and calling ReadChannel per slot. No
// read-back verification after a write — that is the clone service's too.
package ic7300

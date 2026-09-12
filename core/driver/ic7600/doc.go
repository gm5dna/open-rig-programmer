// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7600 is the Icom IC-7600's driver: the capability profiles,
// the session probe, the acknowledged write, and the serial-framing
// report. It sits above the neutral driver.Driver/Session seam, on a
// BINARY codec, exactly as core/driver/ic7610 does.
//
// The wire codec lives next door in core/civ/ic7600 and is imported here
// as civic7600. NOTHING in this package builds a frame, a matcher, a
// command-spec helper or a drain policy of its own: the CI-V framing
// adapter is core/civ's (enabler E1), the answer matchers are the
// profile's, and the two command specs come from civ.CIVReadSpec and
// civ.CIVWriteWithAckSpec.
//
// # Provenance
//
// Everything protocol-shaped here comes from the IC-7600's own
// Instruction Manual (no separate CI-V Reference Guide is published for
// this model), through core/civ/ic7600's profile and through the IC-7600
// capability matrix (docs/superpowers/icom-matrices/
// ic7600-capability-matrix.md rev 1). NO IC-7600 HAS EVER BEEN CONNECTED
// TO THIS PROJECT. Every value below is a reading of a document or a
// stated choice; none is a hardware finding, and writeTrialsComplete
// (caps.go) is FALSE in consequence.
//
// This package does not import core/driver/ftdx101, core/driver/ftdx10,
// core/driver/ft710 or core/driver/ic7610, and must not: a sibling
// package's shape is a template, but its VALUES are its own radio's
// manual reading, and importing one is how another radio's evidence
// silently becomes this one's claim. Where this file cites the IC-7610's
// own findings (prior art for ruling E6, RULING OQ1), the citation is to
// core/civ/ic7610/doc.go's text, never to an import.
//
// # 1. SERIAL FRAMING - ASSUMED 8-N-1, ON NO EVIDENCE FROM THIS DOCUMENT
//
// framing.go implements driver.SerialFramingReporter, returning
// StopBits() == 1. ASSUMED tier convention (spec D3.1), not a reading of
// this radio's document.
//
// WHAT THIS DOCUMENT SAYS ABOUT SERIAL FRAMING: the words "stop bit", "data bit", "parity" and "8 bit" appear NOWHERE in the document about the CI-V/USB/REMOTE link (matrix S3.1) - every hit found in the command-table and connector sections is Baudot/RTTY-mode content unrelated to serial framing terminology. No line anywhere states data bits, parity or stop bits for the CI-V USB port or the [REMOTE] jack.
//
// THE MANDATORY HAZARD SENTENCE, unchanged from the IC-7610 matrix's own: a DATA/RTTY application-port "8 bit / 1 stop bit" line is NOT EVIDENCE about CI-V serial framing - SUCH A LINE IS NOT EVIDENCE about the link this driver opens. Only a statement explicitly about the CI-V/USB/REMOTE link counts. (This document, swept in full, prints no such line at all - unlike the IC-7610's, which prints one, and is why the sentence stands as a hazard for every model in this wave, not only the ones whose page carries it.)
//
// MATERIALITY, verified: transport.DefaultStopBits is 2, so without the
// reporter an IC-7600 would open at 8-N-2 against the tier's assumed
// 8-N-1.
//
// Register home: D5 entry 8. Lift R8 - see framing.go.
//
// # 2. CONTROL-LINE POLICY - THIS DRIVER NEVER TOGGLES RTS OR DTR, AND ON
// # THIS MODEL THERE IS NO HAZARD TO GUARD AGAINST
//
// WHAT THIS DOCUMENT SAYS ABOUT RTS/DTR ON THE CI-V PORT, OR FLOW CONTROL
// ON THE CI-V LINK: NOTHING (matrix S3.2). Swept the command table and the
// two CI-V connector sections (PDF p.20 folio 11; PDF p.168 folio 159).
//
// A GENUINE DIVERGENCE FROM THE IC-7610, RECORDED SO A FUTURE READER DOES
// NOT ASSUME THE SAME HAZARD CARRIED ACROSS: unlike the IC-7610, this
// radio's command table carries NO "USB SEND"/"USB Keying" item assigning
// DTR or RTS of the CI-V USB port to PTT or CW/RTTY keying (matrix S3.2,
// S3.16 ADDED-2 "none") - that Icom feature post-dates this model. The
// only RTS/TXD wiring this document shows (PDF p.32, folio 23, "FSK and
// AFSK connections") is a third-party interface circuit from the
// [ACC1]/[MIC] accessory connector into a PC's RS-232 COM port for
// external FSK keying - a different, non-CI-V signal path, and this
// package does not read it as CI-V-port control-line evidence.
//
// THE POLICY IS UNCHANGED regardless: transport.OpenSerial already drives
// both lines low at open (core/transport/port.go, safety obligation 4),
// before this driver ever sees the port, and THIS DRIVER NEVER TOGGLES
// EITHER. transport.Port is an io.ReadWriteCloser and carries neither
// method, so the only way to reach one is a type assertion;
// TestOpen_ControlLinesAreNeverToggled exists to catch one.
//
// NO REGISTER ENTRY IS CREATED HERE: unlike the IC-7610's own
// ic7610-control-lines-inert (its R16), this model's document gives that
// entry nothing to guard against - there is no printed hazard whose
// absence needs confirming from real hardware.
//
// # 3. THE PROBE
//
// Open's whole wire traffic is: NOTHING for Init, one 19 00 read, and up
// to probeSlotCount 1A 00 reads.
//
//   - NO RADIO MUTATION AT INIT, EVER. E1's InitSequence() is EMPTY.
//
//   - THE IDENTITY STEP. A 19 00 read to 7Ah, with an ADDRESS-MATCHED
//     reply REQUIRED. The reply VALUE is undocumented on every model in
//     this tier (D5 entry 7, matrix lift R7), so it is RECORDED and NEVER
//     MATCHED: Session.Identity().CATID is the static address followed by
//     the observed token.
//
//   - THE BOUNDED OCCUPIED-SLOT SEARCH. Channels 1..probeSlotCount are
//     read until one answers with a record. A rejection means "empty,
//     keep looking" (tier ruling T4, errors.Is(err, transport.ErrRejected)).
//
//   - THE 25-BYTE FINGERPRINT, CONTINUOUS. Twenty-five is the RECORD-ONLY
//     length (spec Erratum 1; the data area including the address is 27,
//     the whole set frame 34). Confirmed at the probe and RE-VALIDATED ON
//     EVERY RECORD READ. A record at any other length fails with
//     *RecordLengthMismatchError, satisfying errors.Is(err,
//     driver.ErrWrongRadio), NAMING NO FOUND MODEL: this model has no
//     registered sibling (matrix S4).
//
//   - AN EMPTY RADIO OPENS UNFINGERPRINTED, on address evidence alone
//     (spec D3.2, D5 entry 2(a), matrix lift R2a).
//
//   - THE R9-SPLIT INIT-UNDER-FLOOD RULE, BOTH HALVES - identical
//     mechanism to the IC-7610's own (a) broadcast frames never reach the
//     engine; (b) a controller-addressed flood drives Init's drain to its
//     cap, NONFATAL-WITH-DIAGNOSTIC, with every LATER drain failure
//     FAIL-CLOSED.
//
//   - THE TWO LIMITATIONS, STATED PLAINLY (spec D3.3). A radio at a CI-V
//     address other than 7Ah times out - this driver builds every frame
//     for 7Ah and the codec refuses any answer not from 7Ah, and there is
//     no --civ-address option (matrix S3.4 records that this radio's own
//     document, unlike the IC-7610's, states the change mechanism - main
//     dial, range 01h-DFh - directly; this project still ships no flag to
//     reach it).
//
//   - FINGERPRINT AND OPENDIAGNOSTICS ARE PACKAGE ACCESSORS, NOT
//     NEUTRAL-SEAM ADDITIONS.
//
// # 4. THE DEFAULT BAUD
//
// DefaultBaud is 19200, graded ASSUMED. Register home:
// `ic7600-auto-baud-mechanics` (civ PROFILE register,
// core/civ/ic7600/doc.go); matrix lift, S3.3.
//
// BETTER EVIDENCED THAN THE IC-7610 IN ONE RESPECT, WORSE IN ANOTHER. PDF
// p.151 (folio 142)'s CI-V Baud Rate item states the factory default
// DIRECTLY, as the literal value "Auto" - genuinely better evidence than
// the IC-7610's own document, which marks no default at all. What "Auto"
// resolves to on the wire for a given controller rate is NOT stated,
// and THAT is what stays ASSUMED here: this driver still needs ONE
// concrete rate to open the port at (internal/wiring cannot sweep), so
// 19200 is picked as an ARBITRARY CHOICE within the five rates the
// front panel names (300, 1200, 4800, 9600, 19200 - matrix S1 row 13),
// no reading of this document favouring one of them over another as the
// rate an auto-bauding radio would lock onto.
//
// WHAT MAKES IT SAFE IS NOT THE CHOICE BUT THE GRADING AND THE FAILURE
// MODE: the probe requires an address-matched 19 00 reply, and silence is
// silence, so a wrong guess costs A CLEAN TIMEOUT AT Open AND NEVER A
// WRONG BYTE. THE DRIVER CANNOT SWEEP: internal/wiring opens the port
// from Capabilities().DefaultBaud.
//
// # 5. THE E6 RULING AND ITS COST, AND THE ONE MECHANICAL DIVERGENCE FROM
// # THE IC-7610
//
// Ruling E6 (core/civ/ic7610/doc.go), applied directly: A SLOT MAY BE
// WRITTEN ONLY WHEN ITS UNMAPPED REGIONS EQUAL THE PROFILE'S Fixed
// TEMPLATE; ANYTHING ELSE IS REFUSED WITH THE REASON NAMED, NEVER
// REWRITTEN.
//
// On this model the unmapped regions are byte ③ WHOLE (a four-valued
// SELECT-group marker, 0=OFF / 1=★1 / 2=★2 / 3=★3; matrix S3.16 ADDED-1)
// and byte ⑪'s HIGH nibble (a four-valued data mode, 0=OFF / 1=DATA 1 /
// 2=DATA 2 / 3=DATA 3; matrix S2 row 20, no divergence from the
// IC-7610 there). Their neutral homes, codeplug.ChannelData.ScanSkip and
// .DataMode, are BOTH BoolField.
//
// THE DIVERGENCE, IN FULL (matrix S3.15(a); core/civ/ic7600's
// SelectByteOffset comment): the IC-7610's own page draws byte ③ as a
// two-nibble split with a stated "Fixed" high nibble, which is what lets
// that driver's UnmappedRegionError name a "low" or "high" nibble
// specifically. THIS RADIO'S OWN PAGE ASSERTS NO SUCH SPLIT - byte ③ is
// one undivided enum cell - so write.go's unmappedRegionsDiffer compares
// it WHOLE, and *UnmappedRegionError's Nibble field carries "whole" for
// this offset rather than "low" or "high". The MECHANISM is identical
// either way (compare against the Fixed template's zero, refuse on any
// difference); only the GRANULARITY of what is named in the refusal
// differs, because that is all this radio's own document supports.
//
// THE COSTS, stated as E6 requires: a channel in a SELECT group, or whose
// data mode is DATA 1/2/3, CANNOT BE WRITTEN BY THIS PROGRAMME AT ALL;
// neither field can be read back; every eligible write costs one extra
// read exchange.
//
// ON ICOM, scan_skip IS SELECT-GROUP MEMBERSHIP, NEVER A SKIP.
//
// # 6. THE DEFERRED GATE-DOMAIN GAP, RECORDED AND NOT PAPERED OVER
//
// Identical shape to the IC-7610's own S6 (core/driver/ic7610/doc.go):
// civ.FieldSpan HAS NO NUMERIC DOMAIN, so civ.Profile.AllowedCommand would
// ADMIT a 1A 00 set carrying 70 MHz or a tone above 299.9 Hz, even though
// matrix S1 rows 15/16 fix the encodable ceiling at 69 999 999 Hz and row
// 12 fixes the tone digits at 0..2999 deci-Hz - IDENTICAL bounds to the
// IC-7610's own. codeplug.Validate and the tone range already bound every
// path through the model layer; WriteChannel and ReadChannel carry the
// same driver-level *OutOfDomainError defence in depth
// (MaxEncodableFreqHz, MaxToneDeciHz, caps.go) as the IC-7610's own
// package. The gap is the same orchestrator-deferred one, not a new
// decision made here.
//
// # 7. ERASE
//
// FieldErase carries the zero FieldSupport in BOTH profiles;
// spec.ConsentUnverifiedWrites structurally never consents it; and
// core/clone/execute.go's DiffErased branch stays UNREACHABLE.
//
// THE WIRE FORM EXISTS HERE, IN TWO SHAPES (matrix S3.13, both recorded
// as evidence only):
//
//   - (a) The 1A 00 clear form. PDF p.178 (folio 169), under ③: "To
//     program the blank channel, enter 'FF' to ③ after the memory channel
//     number (① and ②). This completes the memory channel programming." -
//     the frame FE FE 7A E0 1A 00 <ch-hi> <ch-lo> FF FD. UNLIKE THE
//     IC-7610, this radio's own page prints NO Fixed-0-vs-FF contradiction
//     to reconcile (matrix S3.8(b), S3.13, S3.15(c)): its ③ sub-diagram
//     states no "Fixed" constraint at all. The clear list's channel range
//     is printed as ①,② Memory channel (0001~0099) only - it does not
//     name the scan edges.
//   - (b) Command 0B, "Memory clear" (PDF p.169, folio 160) - a whole
//     command, no sub-command, no data, clearing the currently selected
//     channel.
//
// NO CLEAR COMMAND EXISTS IN THIS TIER. There is no builder for either
// form, and core/civ's AllowedCommand admits only 19 00, a valid 1A 00
// read and a re-validated 1A 00 set.
//
// WHAT A FUTURE WRITE-TRIAL MILESTONE ON THE IC-7600 WOULD NEED (matrix
// S3.13): a hardware-verified read path; a captured answer to each clear
// form; a FieldErase FieldSupport value earned by that capture; a builder
// and gate admission for whichever form the capture validates; and
// writeTrialsComplete moved off FALSE (S3.14). Until every one of those
// lands, DiffErased stays unreachable.
//
// # 8. RULING OQ1 - THE RADIX OF THE PRINTED MODE CODES, APPLIED BY
// # DIRECT PRECEDENT
//
// RULING OQ1 (24/08/2026, orchestrator, core/civ/ic7610/doc.go): THE
// PRINTED MODE CODES ARE HEXADECIMAL. PSK is the wire byte 0x12 and PSK-R
// is 0x13. This radio's own document prints the identical two codes in
// the identical style (matrix S1 row 6) and offers no reading that would
// unsettle the ruling, so it is applied here by direct precedent rather
// than re-derived: register entry ic7600-mode-code-radix cites OQ1 rather
// than repeating its reasoning.
//
// # The ic7600 DRIVER register
//
// Three entries, homed HERE (fewer than the IC-7610's six: no control-
// lines entry applies - S2 above - and the mode-code-radix and
// filter-value-set entries are carried by direct citation to the
// IC-7610's own rather than re-derived, per S8 above and profile.go's
// enum comments).
//
//   - ic7600-storable-frequency-ceiling / -floor (mirrors the IC-7610's
//     R17a/R17b) - what the radio will actually STORE and return, as
//     against what the record can ENCODE. GRADE: the ENCODING bounds
//     (69 999 999 Hz ceiling, 0 Hz floor) are MANUAL-EVIDENCED (matrix S1
//     rows 15/16, PDF p.175 folio 166's five-cell strip); the radio's
//     TUNING/STORAGE bounds are ASSUMED.
//     STAGE W LIFTS IT WITH: capture ic7600-storable-ceiling-ch03 and
//     ic7600-storable-floor-ch04, identical procedure to the IC-7610's own
//     (write successively higher/lower frequencies with 1A 00, read each
//     back, record the highest/lowest value acknowledged and returned
//     unchanged) but taken on an IC-7600.
//
//   - ic7600-scan-edge-record-fields (R18) - that a scan edge's record
//     honours the record's NON-FREQUENCY fields the way a memory
//     channel's does. GRADE: ASSUMED (matrix S2, "Whether the radio
//     HONOURS each field on a scan edge is ASSUMED").
//     STAGE W LIFTS IT WITH: capture ic7600-scan-edge-p1, identical
//     procedure to the IC-7610's own R18.
//
//   - ic7600-select-marker-semantics (mirrors the IC-7610's R20) - what
//     byte ③'s values 1, 2 and 3 actually DO to a scan on real hardware,
//     and (new to this radio, because there is no nibble split to ask the
//     same question of a "Fixed" high nibble) what a NON-ZERO,
//     UNDOCUMENTED byte ③ value (e.g. the range above 3) does, since this
//     page prints no Fixed-template refusal mechanism of its own.
//     GRADE: the VOCABULARY is MANUAL-EVIDENCED (PDF p.178, corroborated
//     by command 0E's B0-B2 sub-commands, PDF p.169 folio 160); what each
//     value DOES on a real radio is ASSUMED. This is the region ruling E6
//     leaves UNMAPPED.
//     STAGE W LIFTS IT WITH: capture ic7600-select-marker-ch05, identical
//     procedure to the IC-7610's own R20 (write byte ③ = 02, start a
//     Select memory scan pointed at group 2 with command 0E, and record
//     whether the channel is scanned and how the front panel labels it).
//
// # The civ PROFILE register - LISTED HERE, REPRODUCED IN FULL THERE
//
// These entries live in core/civ/ic7600/doc.go: ic7600-auto-baud-mechanics
// (R11), ic7600-civ-rate-list (R12), ic7600-1a00-set-ack (R14),
// ic7600-full-record-mandatory (R15), plus the shared D5 entries 1, 2(a),
// 2(b), 3, 4, 5, 6, 7, 8, 9. There is no ic7600-usb-echo-default entry:
// unlike the IC-7610, this document carries no CI-V USB Echo Back item at
// all (matrix S3.6) - this radio predates the feature, and echo is
// handled structurally regardless (NoteSent + drop-first-match).
package ic7600

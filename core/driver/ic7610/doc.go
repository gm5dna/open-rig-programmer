// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7610 is the Icom IC-7610's driver: the capability profiles,
// the session probe, the acknowledged write, and the serial-framing
// report. It is core/driver/ftdx101's sibling above the neutral
// driver.Driver/Session seam, and the first driver in this tree to sit on
// a BINARY codec rather than a ';'-terminated one.
//
// The wire codec lives next door in core/civ/ic7610 and is imported here
// as civic7610, exactly as core/driver/ftdx101 imports core/cat/ftdx101 as
// catftdx101. NOTHING in this package builds a frame, a matcher, a
// command-spec helper or a drain policy of its own: the CI-V framing
// adapter is core/civ's (enabler E1), the answer matchers are the
// profile's, and the two command specs come from civ.CIVReadSpec and
// civ.CIVWriteWithAckSpec. A local re-implementation of any of them would
// be a second reading of the same protocol, free to drift from the one the
// gate enforces.
//
// # Provenance
//
// Everything protocol-shaped here comes from the IC-7610 CI-V Reference
// Guide rev 4, through core/civ/ic7610's profile and through the
// IC-7610 capability matrix (docs/superpowers/icom-matrices/,
// ic7610-capability-matrix.md rev 1 plus its Errata). NO IC-7610 HAS EVER
// BEEN CONNECTED TO THIS PROJECT. Every value below is a reading of a
// document or a stated choice; none is a hardware finding, and
// writeTrialsComplete (caps.go) is FALSE in consequence.
//
// This package does not import core/driver/ftdx101, core/driver/ftdx10 or
// core/driver/ft710, and must not: those drivers' shapes are templates,
// but their VALUES are their own radios' manual readings, and importing
// one is how a Yaesu radio's evidence silently becomes an Icom's claim.
//
// # 1. SERIAL FRAMING - ASSUMED 8-N-1, ON NO EVIDENCE FROM THIS DOCUMENT
//
// Task 10's framing.go implements driver.SerialFramingReporter on the
// CONCRETE DRIVER, returning StopBits() == 1. The number is an ASSUMED
// tier convention (spec D3.1: "Every Icom driver returns 1, as an ASSUMED
// register entry with a named lift per model"), not a reading of this
// radio's document.
//
// WHAT THIS DOCUMENT SAYS ABOUT SERIAL FRAMING: NOTHING. All 17 pages
// were rendered at 300 dpi and read. The words "stop bit", "data bit",
// "parity" and "8 bit" appear NOWHERE in the document, about any port
// (matrix §3.1).
//
// THE MANDATORY HAZARD SENTENCE, written down even though this document
// prints no such line: Icom manuals print "8 bit / 1 stop bit" style lines
// about the DATA/RTTY application port, and SUCH A LINE IS NOT EVIDENCE
// ABOUT CI-V SERIAL FRAMING. Only a statement explicitly about the CI-V /
// REMOTE / USB CI-V link counts. It is recorded here so that a later
// reader does not go looking for one and mistake a neighbouring line for
// it.
//
// The four rate-bearing or framing-adjacent lines this document does
// print, and which port each is about (matrix §3.1's table):
//
//   - PDF p.3 (folio 2), "◇ Preparing": "To control the transceiver, first
//     set its address, data communication speed, and transceive function.
//     These settings are set in Set mode (Refer to the IC-7610 instruction
//     manual)." — CI-V, explicitly. NOT framing evidence: it names "data
//     communication speed" as a settable item and defers its value, and
//     everything about the link's framing, to a document this project does
//     not hold.
//   - PDF p.10 (folio 9), "◇ Command table" footnote *4: "• 115200 bps:
//     150 FEs" … "• 4800 bps: 7 FEs" — CI-V, explicitly (the preamble a
//     power-ON frame needs at each rate). NOT framing evidence: line rates
//     only, no word about data bits, parity or stop bits.
//   - PDF p.10 (folio 9), footnote *7: "need to select 115200 in the CI-V
//     Baud Rate item", "MENU » SET > Connectors > CI-V" — CI-V,
//     explicitly, but scoped to using [USB 1] for scope waveform output.
//     NOT framing evidence.
//   - PDF p.7 (folio 6), row "1A 05 01 21": "Connectors > Decode Baud Rate
//     (00=4800 bps, 01=9600 bps, 02=19200 bps, 03=38400 bps)" — NOT CI-V.
//     This is the internal RTTY/PSK DECODER's baud rate, and IT MUST NEVER
//     BE READ AS CI-V EVIDENCE. It is the nearest thing in this document
//     to the classic DATA/RTTY-port trap: a "baud rate" row sitting three
//     rows below the CI-V rows in the same column.
//
// MATERIALITY, verified: transport.DefaultStopBits is 2, so without the
// reporter an IC-7610 would open at 8-N-2 against the tier's assumed
// 8-N-1 — the silent divergence spec D3.1 exists to prevent.
//
// Register home: D5 entry 8, "Serial framing 8-N-1 (D3.1) per model".
// Lift R8 — see framing.go, which carries the capture verbatim.
//
// # 2. CONTROL-LINE POLICY - THIS DRIVER NEVER TOGGLES RTS OR DTR
//
// WHAT THE DOCUMENT SAYS ABOUT RTS/DTR AT OPEN, OR ABOUT FLOW CONTROL ON
// THE CI-V LINK: NOTHING. All 17 pages swept at 300 dpi. There is no
// handshake statement, no flow-control setting and no statement about the
// state of any modem control line (matrix §3.2).
//
// WHAT IT DOES SAY, AND WHY IT MATTERS - A HAZARD, NOT A PERMISSION. PDF
// p.7 (folio 6), "◇ Command table", left column, three consecutive rows:
//
//	1A 05  00 95  00 ~ 04  Connectors > USB SEND/Keying > USB SEND
//	                       (00=OFF, 01=USB1(A) DTR, 02=USB1(A) RTS,
//	                        03=USB1(B) DTR, 04=USB1(B) RTS)
//	1A 05  00 96  00 ~ 04  … USB Keying (CW)    — same five values
//	1A 05  00 97  00 ~ 04  … USB Keying (RTTY)  — same five values
//
// MANUAL-EVIDENCED: on this radio, DTR and RTS of either USB serial
// endpoint can be assigned to PTT (USB SEND) or to CW/RTTY keying. The
// CI-V endpoint is one of those same USB serial ports — PDF p.3 (folio 2),
// "◇ CI-V connection": "Use a USB cable (user supplied) to connect the
// IC-7610 and the PC (controller)", with the callout "To the [USB 1] port"
// on a Type-B socket.
//
// ERRATUM 8's ADDITION: THE DOCUMENT NEVER SAYS WHICH OF THE TWO ENDPOINTS
// CARRIES CI-V. All 17 pages were swept; no line identifies the CI-V
// endpoint among the [USB 1] sub-ports. That silence is part of why the
// conservative choice is the only defensible one — a driver cannot know
// whether the endpoint it has opened is the one whose DTR or RTS has been
// assigned to PTT.
//
// CONSEQUENCE, RECORDED: a driver that asserts RTS or DTR when it opens
// the CI-V port CAN KEY THE TRANSMITTER of a radio whose owner has set
// USB SEND to that line.
//
// THE POLICY: transport.OpenSerial already drives both lines low at open
// (core/transport/port.go, safety obligation 4), before this driver ever
// sees the port, and THIS DRIVER NEVER TOGGLES EITHER. transport.Port is
// an io.ReadWriteCloser and carries neither method, so the only way to
// reach one is a type assertion; TestOpen_ControlLinesAreNeverToggled
// exists to make such an assertion visible if anyone ever writes one.
//
// # 3. THE PROBE
//
// Open's whole wire traffic is: NOTHING for Init, one 19 00 read, and up
// to probeSlotCount 1A 00 reads. TestOpen_InitWritesNothing compares the
// exact byte sequence.
//
//   - NO RADIO MUTATION AT INIT, EVER. E1's InitSequence() is EMPTY
//     (core/civ/framing.go), which is a safety property rather than an
//     omission: transceive broadcasts are excluded STRUCTURALLY, by
//     address filtering, instead of by writing a transceive-off setting,
//     so opening a session touches nothing outside the consent regime. No
//     1A 05, no transceive set, no clear.
//
//   - THE IDENTITY STEP. A 19 00 read to 98h, with an ADDRESS-MATCHED
//     reply REQUIRED. The reply VALUE is undocumented on every model in
//     this tier (D5 entry 7, matrix lift R7), so it is RECORDED and NEVER
//     MATCHED: Session.Identity().CATID is the static address followed by
//     the observed token, and OpenReport.IDToken keeps the raw bytes for a
//     future hardware lift to compare against. Three different tokens all
//     open a session, and TestOpen_IDTokenIsRecordedNeverMatched pins that.
//
//   - THE BOUNDED OCCUPIED-SLOT SEARCH. Channels 1..probeSlotCount are
//     read until one answers with a record. A rejection means "empty, keep
//     looking" — and under tier ruling T4 that branch keys on
//     errors.Is(err, transport.ErrRejected), because Engine.Do consumes
//     the FA and returns NO frame. Nothing here calls civ.IsRejection.
//
//   - THE 25-BYTE FINGERPRINT, AND IT IS CONTINUOUS. Twenty-five is the
//     RECORD-ONLY length, excluding the two channel-selector bytes (spec
//     Erratum 1; the data area including them is 27 and the whole set
//     frame is 34). It is confirmed at the probe and then RE-VALIDATED ON
//     EVERY RECORD READ, because civ.Profile.MemoryAnswerRecord checks the
//     length on every call. A record at any other length fails with an
//     error satisfying errors.Is(err, driver.ErrWrongRadio) — see
//     RecordLengthMismatchError, which NAMES NO FOUND MODEL:
//     cross-model record-length distinctness is a TIER-LEVEL WAVE-4 CHECK
//     and this model has no registered sibling.
//
//   - AN EMPTY RADIO OPENS UNFINGERPRINTED, on address evidence alone
//     (spec D3.2, D5 entry 2(a), matrix lift R2a). Refusing there would
//     make a radio whose memories are all empty unprogrammable by this
//     programme, which is precisely the radio a user most wants to
//     programme.
//
//   - THE R9-SPLIT INIT-UNDER-FLOOD RULE, BOTH HALVES. (a) A BROADCAST
//     flood (to = 00) never reaches the engine at all: civ's accumulator
//     counts and NEVER RETURNS those frames, so the drain's idle timer is
//     never re-armed, Engine.Init SUCCEEDS, and what rises is the
//     ADAPTER's Unexpected counter. (b) A CONTROLLER-ADDRESSED flood
//     (to = E0) does reach the engine and drives Init's drain to its
//     absolute cap; that INITIAL failure is NONFATAL-WITH-DIAGNOSTIC
//     (OpenReport.InitDrainCapExceeded), because E1's drain is bounded
//     precisely so it cannot fail the open. EVERY LATER QUARANTINE DRAIN
//     FAILURE REMAINS FAIL-CLOSED: once the session is exchanging frames,
//     a drain that cannot find quiet means this program can no longer tell
//     its own answers from somebody else's. Two tests hold the two halves
//     apart so neither can be relaxed into the other.
//
//   - THE TWO LIMITATIONS, STATED PLAINLY (spec D3.3). A radio at a CI-V
//     address other than 98h times out — this driver builds every frame
//     for 98h and the codec refuses any answer not from 98h, and there is
//     no --civ-address option. A DIFFERENT ICOM MODEL at ITS factory
//     address times out identically: nothing was heard from, so nothing
//     can be attributed, and Open reports a timeout rather than a
//     wrong-radio refusal.
//
//   - FINGERPRINT AND OPENDIAGNOSTICS ARE IC7610-PACKAGE ACCESSORS, NOT
//     NEUTRAL-SEAM ADDITIONS. driver.SessionDiagnostics carries ONE
//     aggregate counter, and widening the neutral seam to carry a per-tier
//     fingerprint is a tier-shared change five worktrees would want a say
//     in. A deliberate scope choice.
//
// # 4. THE DEFAULT BAUD (OQ2)
//
// DefaultBaud is 19200, graded ASSUMED. Register home:
// `ic7610-default-baud` (civ PROFILE register, core/civ/ic7610/doc.go);
// matrix lift R11.
//
// THE ARBITRARINESS IS RECORDED. The document names six rates (PDF p.10's
// footnote *4) and MARKS NO DEFAULT: PDF p.3 defers the value to the
// IC-7610 instruction manual, which this project does not hold. The choice
// of 19200 within that printed set is the plan's, and no reading of this
// document favours one of the six.
//
// WHAT MAKES IT SAFE IS NOT THE CHOICE BUT THE GRADING AND THE FAILURE
// MODE: the probe requires an address-matched 19 00 reply, and silence is
// silence, so a wrong guess costs A CLEAN TIMEOUT AT Open AND NEVER A
// WRONG BYTE. THE DRIVER CANNOT SWEEP: internal/wiring opens the port from
// Capabilities().DefaultBaud, and Wave 3 may never touch internal/wiring.
// This belongs in the README's honesty rows at Wave 4, alongside the
// no---civ-address one.
//
// # 5. THE E6 RULING AND ITS COST
//
// Ruling E6: A SLOT MAY BE WRITTEN ONLY WHEN ITS UNMAPPED REGIONS EQUAL
// THE PROFILE'S Fixed TEMPLATE; ANYTHING ELSE IS REFUSED WITH THE REASON
// NAMED, NEVER REWRITTEN.
//
// On this model the unmapped regions are byte ③'s LOW nibble (a
// four-valued SELECT-group marker, 0=OFF / 1=★1 / 2=★2 / 3=★3; matrix
// §3.16 ADDED-1) and byte ⑪'s HIGH nibble (a four-valued data mode,
// 0=OFF / 1=DATA 1 / 2=DATA 2 / 3=DATA 3; matrix §1b as corrected by
// erratum 5, scope widened by erratum 12). Their neutral homes,
// codeplug.ChannelData.ScanSkip and .DataMode, are BOTH BoolField.
//
// THE COSTS, stated as E6 requires:
//
//   - A CHANNEL THAT IS IN A SELECT GROUP (★1/★2/★3), OR WHOSE DATA MODE
//     IS DATA 1/2/3, CANNOT BE WRITTEN BY THIS PROGRAMME AT ALL. The write
//     is REFUSED with the reason named. It is NEVER silently downgraded to
//     ★1/DATA 1 and NEVER silently cleared to OFF.
//   - Those two fields cannot be read back, exposed or edited: they are
//     Unavailable on every read, because an unmapped region is not decoded.
//   - EVERY ELIGIBLE WRITE COSTS ONE EXTRA READ EXCHANGE — the E6
//     comparison read, which is also tier ruling T5's one recorded
//     exception to "refusal before any wire traffic".
//   - These grades DIFFER from matrix §2 as it stood at rev 1, which had
//     scan_skip Sup/Sup on MEM and data_mode Sup/Sup on both banks. The
//     matrix's own errata 9 and 10 now record the zero FieldSupport on
//     both banks; erratum 12 widens ADDED-1 to cover byte ⑪.
//
// ON ICOM, scan_skip IS SELECT-GROUP MEMBERSHIP, NEVER A SKIP.
//
// # 5b. THE ONE APPROXIMATION IN THE WRITE GATE'S FIELD SET
//
// write.go's conditionalRequestedFields appends every state-bearing field
// OUTSIDE the base set when it is Known, so the capability gate can REFUSE
// a request rather than drop it. Ten of the twelve predicates are exact
// tri-state tests. Two of the remaining three plain fields — CTCSS and
// Shift — are exact too, because an empty string is a vocabulary member of
// no radio.
//
// THE CLARIFIER IS THE APPROXIMATION. codeplug.ChannelData carries it as
// ClarHz plus two bools, with no state, so a channel asking for an
// explicitly-ZERO clarifier is indistinguishable from one that never
// carried a clarifier at all: FieldClarifier is not appended, and the gate
// never sees the request.
//
// ON THIS MODEL IT COSTS NOTHING, and the reason is worth writing down
// rather than assuming: the 1A 00 record has no clarifier span, so there
// is nothing to write even if the gate saw it; and core/codeplug's own
// touchedFields treats clarifier as one of the UNCONDITIONAL six and
// FILTERS IT OUT on a bank that cannot reach the field, so the clone
// service never requests it either. The gap is therefore unreachable
// through the model layer, exactly like §6's. It is recorded here as an
// honesty row, and it would need re-examining for any Icom model whose
// record DOES carry a clarifier.
//
// # 6. THE DEFERRED GATE-DOMAIN GAP, RECORDED AND NOT PAPERED OVER
//
// civ.FieldSpan HAS NO NUMERIC DOMAIN. civ's validateSpanValue
// (core/civ/recordcodec.go) checks only BCD width and scale, so
// civ.Profile.AllowedCommand — the last defence before a radio sees bytes
// — WOULD ADMIT a 1A 00 set carrying 70 MHz, or a tone above 299.9 Hz,
// even though matrix §1 row 12 fixes the encodable ceiling at 69 999 999
// Hz and §1 row 8 fixes the tone digits at 0..2999 deci-Hz.
//
// WHAT IS ALREADY CLOSED, verified in code: codeplug.Validate bounds the
// primary frequency against caps.MinFreqHz/MaxFreqHz
// (core/codeplug/validate.go), and since enabler E3 the tone range bounds
// tones. So EVERY PATH THAT REACHES THIS DRIVER THROUGH THE MODEL LAYER IS
// ALREADY COVERED; the residual exposure is the gate's own blindness to a
// frame arriving by any other route.
//
// WHAT THIS DRIVER DOES ABOUT IT: WriteChannel's ladder carries
// driver-level PRE-BUILD TYPED REFUSALS for a Known frequency above
// MaxEncodableFreqHz and a Known tone above MaxToneDeciHz
// (*OutOfDomainError). THESE ARE DEFENCE IN DEPTH AND THEY ARE NOT THE
// GATE.
//
// THE ORCHESTRATOR DEFERRED gate-level enforcement on 24/08/2026 to a
// post-Wave-3 enabler follow-up, on three grounds:
//
//  1. The width is bounded by the upper layers named above, so no path
//     through the model layer is exposed while the deferral stands.
//  2. The gap FLIPS VISIBLY when it closes.
//     TestNumericRefusalIsDefenceInDepthNotTheGate asserts that the gate
//     currently ADMITS an out-of-domain frame; a later enabler that closes
//     the gap turns that test red, which is a reviewable change rather
//     than a silent one. DO NOT DELETE THAT TEST TO "FIX" IT.
//  3. No mid-flight enabler scope grows: Wave 2.5 is already specified and
//     sequenced, and widening it would re-open a dual-reviewed document.
//
// The follow-up's likely shape, for whoever picks it up: FieldSpan gains
// optional Min/Max in the field's neutral unit, enforced in
// validateSpanValue on both the encode and the gate paths, with per-model
// negative gate tests and the four Yaesu profiles pinned unchanged.
//
// # 7. ERASE
//
// FieldErase carries the zero FieldSupport in BOTH profiles;
// spec.ConsentUnverifiedWrites structurally never consents it (its
// `f != FieldErase` guard); and core/clone/execute.go's DiffErased branch
// stays UNREACHABLE for this model.
//
// UNLIKE YAESU, THE WIRE FORM EXISTS HERE, IN TWO SHAPES (matrix §3.13,
// both MANUAL-EVIDENCED as printed forms, both recorded as evidence only):
//
//   - (a) The 1A 00 clear form. PDF p.12 (folio 11), right column, under
//     the heading "To clear the memory channel contents on 1A 00:":
//     "①, ②: Memory channel (00 01~00 99)", "③: FF", "④: None" — a
//     three-byte data area, i.e. the frame
//     FE FE 98 E0 1A 00 <ch-hi> <ch-lo> FF FD. Note the internal conflict
//     with the ③ sub-diagram's Fixed 0 high nibble, recorded at matrix
//     §3.8(b) and UNRESOLVED. Note also that the clear list's channel
//     range is printed as 00 01~00 99 only — it does not name the scan
//     edges.
//   - (b) Command 0B, "Memory clear". PDF p.4 (folio 3), "◇ Command
//     table", left column, row "0B | (blank) | Memory clear" — a whole
//     command, no sub-command, no data, which clears the currently
//     selected memory channel.
//
// NO CLEAR COMMAND EXISTS IN THIS TIER — a CHOICE fixed by the tier spec
// (D1 and D4 "Erase"). There is no builder for either form, and core/civ's
// AllowedCommand admits only 19 00, a valid 1A 00 read and a re-validated
// 1A 00 set, so neither clear frame can reach the wire.
// TestWriteChannel_NoClearFrameIsReachable asserts the driver never builds
// one either.
//
// WHAT A FUTURE WRITE-TRIAL MILESTONE ON THE IC-7610 WOULD NEED (matrix
// §3.13, reproduced verbatim): all of the following, on an IC-7610 Stuart
// or a collaborator actually owns: a hardware-verified read path (so a
// cleared channel can be confirmed cleared); a resolution of the ③
// Fixed-0 versus "FF" conflict above, from the radio rather than from the
// page; a captured answer to each of the two clear forms, showing which is
// acknowledged and what the radio's own front panel then displays; a
// FieldErase FieldSupport value earned by that capture rather than
// assumed; a builder and a gate admission for whichever form the capture
// validates; and writeTrialsComplete moved off FALSE for this model
// (§3.14). Until every one of those lands, DiffErased stays unreachable.
//
// # 8. THE STALE "4-CHARACTER CAT ID" COMMENTS
//
// core/spec/capabilities.go's CATID field, core/driver/driver.go's
// Identity.CATID and Driver.Open, and core/codeplug/radioinfo.go all
// describe CATID as a "4-character CAT ID answer". THAT DOES NOT DESCRIBE
// A CI-V RADIO: this driver's CATID is the CI-V address followed by an
// observed 19 00 token of undocumented width, which spec D3.2 generalises
// those comments to cover.
//
// ALL THREE FILES ARE OUTSIDE THIS WORKTREE'S OWNERSHIP. They are recorded
// here, by name, as contradicting spec D3.2, and left for the Wave-4 doc
// pass.
//
// # 9. RULING OQ1 - THE RADIX OF THE PRINTED MODE CODES
//
// RULING OQ1 (24/08/2026, orchestrator): THE PRINTED MODE CODES ARE
// HEXADECIMAL. PSK is the wire byte 0x12 and PSK-R is 0x13.
//
// REASON: the document prints every command, sub-command and address as
// hexadecimal byte pairs, and the "①Receiving mode" column is a column of
// the same kind. The one place it prints packed BCD of a decimal number
// naming a hex value (PDF p.7's "1A 05 01 13 … (00 00=00h ~ 02 23=DFh)
// (in Hexadecimal)") announces itself as such; this column does not.
//
// The ruling touches PSK and PSK-R alone — the other eight codes are
// identical under either reading. Its capture is register entry
// `ic7610-mode-code-radix` in core/civ/ic7610/doc.go. Reversing the ruling
// is the orchestrator's to do, not an editor's, and
// core/civ/ic7610/crosscheck_test.go says so at the failure site.
//
// # The ic7610 DRIVER register
//
// Six entries, homed HERE, each with its capture block reproduced from the
// matrix. The civ-homed entries live in core/civ/ic7610/doc.go and are
// LISTED at the foot of this register so a reader knows the set is not
// complete without them.
//
//   - ic7610-control-lines-inert (R16) - that RTS and DTR left DEASSERTED
//     neither key the radio nor prevent CI-V traffic.
//     GRADE: ASSUMED. The document says nothing about any modem control
//     line (§3.2); what it does say is that either endpoint's DTR or RTS
//     can be ASSIGNED to PTT or keying, which is a hazard rather than a
//     permission. The project CHOICE is to open with both deasserted and
//     never toggle them.
//     STAGE R LIFTS IT WITH: capture ic7610-control-lines-inert - with
//     USB SEND, USB Keying (CW) and USB Keying (RTTY) all set to 00 (OFF)
//     from the front panel, open the IC-7610's USB CI-V endpoint with RTS
//     and DTR deasserted, send FE FE 98 E0 19 00 FD, and record whether an
//     answer arrives and whether the radio's TX indicator lights.
//     SCOPE: that one radio, that one port, those three settings.
//
//   - ic7610-storable-frequency-ceiling (R17a) - what the radio will
//     actually STORE and return, as against what the record can ENCODE.
//     GRADE: the ENCODING bound (69 999 999 Hz) is MANUAL-EVIDENCED —
//     PDF p.11 (folio 10)'s five-cell frequency strip prints cell 5 as
//     "0 : 0" with the rotated labels "1 GHz digit: 0 (Fixed)" and
//     "100 MHz digit: 0 (Fixed)", and cell 4's high nibble as "10 MHz
//     digit: 0-6". The radio's TUNING ceiling is ASSUMED.
//     spec.Capabilities.MaxFreqHz carries the ENCODABLE figure, because it
//     is the only number the document yields (matrix erratum 11). The
//     distinction is recorded in the comment beside MaxEncodableFreqHz in
//     caps.go — NOT in deliberatelyZero, which is the R11 audit's arm for
//     a field left ZERO, and this one is populated; it discharges R11
//     through the audit's populated arm instead.
//     STAGE W LIFTS IT WITH: capture ic7610-storable-ceiling-ch03 - on one
//     IC-7610, write memory channel 03 with 1A 00 at successively higher
//     frequencies - 60.000000, 69.999999 MHz - reading each back with
//     1A 00 after the acknowledgement, and record the highest value the
//     radio both acknowledges with FB and returns unchanged.
//     SCOPE: the highest frequency THAT radio stores and returns for THAT
//     one channel. It says nothing about what the radio will TUNE, nothing
//     about the Sub band, and nothing about any other model.
//
//   - ic7610-storable-frequency-floor (R17b) - the same question at the
//     other end.
//     GRADE: the ENCODING bound (0 Hz) is MANUAL-EVIDENCED — the same
//     strip's eight variable nibbles are labelled 0-9, so 0 is encodable
//     in every one. The radio's TUNING floor is ASSUMED.
//     spec.Capabilities.MinFreqHz carries 0, and THAT one IS in caps.go's
//     deliberatelyZero table, because zero is this radio's declared floor
//     rather than an omission.
//     STAGE W LIFTS IT WITH: capture ic7610-storable-floor-ch04 - on the
//     same radio, write memory channel 04 with 1A 00 at successively lower
//     frequencies - 0.030000, 0.010000, 0.000000 MHz - reading each back,
//     and record the lowest value the radio both acknowledges with FB and
//     returns unchanged.
//     SCOPE: the lowest frequency THAT radio stores and returns for THAT
//     one channel. Same exclusions as R17a.
//
//   - ic7610-scan-edge-record-fields (R18) - that a scan edge's record
//     honours the record's NON-FREQUENCY fields the way a memory
//     channel's does.
//     GRADE: ASSUMED. Matrix §3.15(d): P1 and P2 are not a separate bank
//     in the wire protocol at all - they are two more values of the same
//     two-byte selector, so the record is the SAME record. Whether the
//     radio HONOURS every field on an edge is a different question, and
//     this document does not answer it. Erratum 2 folds the SCAN bank's
//     NoBlank claim under this same lift; P2 is uncovered even by that.
//     STAGE W LIFTS IT WITH: capture ic7610-scan-edge-p1 - on one IC-7610
//     whose P1 has never been set, first read 01 00 with 1A 00 and record
//     the answer frame verbatim; then write 01 00 with a full record
//     carrying a distinct value in every non-frequency field (USB, FIL2,
//     DATA 1, TONE, repeater tone 88.5 Hz, tone squelch 100.0 Hz, name
//     EDGEP1TST), read 01 00 back, and record which of those fields return
//     the value written.
//     SCOPE: what THAT radio answers for THAT one unwritten scan edge, and
//     which record fields THAT radio honours on THAT one edge. It says
//     nothing about P2, nothing about an ordinary memory channel, and
//     nothing about any other model.
//
//   - ic7610-mode-code-completeness (R19) - that the ten printed mode
//     codes are the COMPLETE set, i.e. that 06 and 09-11 name nothing.
//     GRADE: ASSUMED. The ten pairs themselves are MANUAL-EVIDENCED
//     (PDF p.11's "①Receiving mode" column, corroborated at PDF p.14's
//     "Command: 26"); that no eleventh exists is not stated anywhere. A
//     record carrying an unlisted code therefore FAILS TO DECODE with a
//     parse error naming the offset, which is the honest outcome.
//     STAGE R LIFTS IT WITH: capture ic7610-mode-code-sweep - on one
//     IC-7610, step the Main band through every mode position the front
//     panel offers, reading 04 after each and recording the mode code
//     returned; then send each code printed nowhere in the table - 06, 09,
//     10, 11 - with 06 and record whether the radio answers FB or FA.
//     SCOPE: which mode codes THAT radio reports and accepts on THAT band.
//     It does not establish what an unlisted code MEANS, and it says
//     nothing about any other model.
//
//   - ic7610-select-marker-semantics (R20) - what byte ③'s low-nibble
//     values 1, 2 and 3 actually DO to a scan.
//     GRADE: the VOCABULARY is MANUAL-EVIDENCED - PDF p.12 (folio 11)'s ③
//     sub-diagram prints "0=OFF / 1= ★1 / 2= ★2 / 3= ★3", and PDF p.4
//     (folio 3)'s command 0E rows (B1 01~03 "Set the channel as a Select
//     channel (01=SEL1, 02=SEL2, 03=SEL3)", B0 "Clear the Select channel
//     setting", B2 00~03, 23 "Start a Select memory scan") show what the
//     values name. What each value DOES to a scan on a real radio is
//     ASSUMED. This is the nibble ruling E6 leaves UNMAPPED.
//     STAGE W LIFTS IT WITH: capture ic7610-select-marker-ch05 - write
//     memory channel 05 with byte ③ = 02, then start a Select memory scan
//     pointed at SEL2 (0E B2 02) and record whether channel 05 is scanned,
//     and whether the front panel shows it as ★2.
//     SCOPE: what the value 2 means on THAT one channel on THAT radio.
//
// # The civ PROFILE register - LISTED HERE, REPRODUCED IN FULL THERE
//
// These entries live in core/civ/ic7610/doc.go with their capture blocks;
// they are named here so a reader of this register knows the set is not
// complete without them: ic7610-read-request-form (R1),
// ic7610-empty-channel-fa (R2a), ic7610-empty-channel-ff (R2b),
// ic7610-name-space-character (R3), ic7610-name-pad-byte (R4),
// ic7610-wire-order (R5), ic7610-record-length (R6),
// ic7610-id-token (R7), ic7610-transceive-broadcast-address (R9),
// ic7610-transceive-factory-default (R10), ic7610-default-baud (R11),
// ic7610-civ-rate-list (R12), ic7610-usb-echo-default (R13),
// ic7610-1a00-set-ack (R14), ic7610-full-record-mandatory (R15),
// ic7610-mode-code-radix (RULING OQ1), ic7610-filter-value-set,
// ic7610-default-tone-undocumented and ic7610-e6-unmapped-regions.
//
// # A procedural fact every capture above inherits (matrix ADDED-3)
//
// PDF p.4 (folio 3)'s grey NOTE panel: "Operating some control dials
// overrides CI-V commands. If a control dial (such as the AF Volume dial
// that has a mark on it) is rotated after sending a CI-V command, the
// command will be overwritten by the operation." MANUAL-EVIDENCED. On this
// radio a readback that disagrees with what was written is not necessarily
// a protocol fault — it may be a hand on the front panel. EVERY Stage R
// and Stage W capture named above must record that no control was touched
// during the capture.
package ic7610

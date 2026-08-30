// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7851 is the Icom IC-7851/IC-7850 driver: the capability
// profiles, the session probe, the acknowledged write, and the
// serial-framing report. It sits above the neutral driver.Driver/Session
// seam on a BINARY codec rather than a ';'-terminated one.
//
// The wire codec lives next door in core/civ/ic7851 and is imported here
// as civic7851. NOTHING in this package builds a frame, a matcher, a
// command-spec helper or a drain policy of its own: the CI-V framing
// adapter is core/civ's, the answer matchers are the profile's, and the
// two command specs come from civ.CIVReadSpec and civ.CIVWriteWithAckSpec.
// A local re-implementation of any of them would be a second reading of
// the same protocol, free to drift from the one the gate enforces.
//
// This package does not import another model's driver and must not: those
// drivers' SHAPES are precedents, but their VALUES are their own radios'
// manual readings, and importing one is how another radio's evidence
// silently becomes this one's claim.
//
// # 0. The document
//
// There is NO standalone CI-V reference guide for this model. The whole
// authority is
//
//	IC-7850/IC-7851 Instruction Manual, Revision 3,
//	document code A7205H-1EX-3, 283 PDF pages, (c) 2015-2018 Icom Inc.
//
// Its CI-V material is Section 18, "CONTROL COMMAND", which is PDF pages
// 250-265 (printed folios 18-1 to 18-16). The memory record this driver
// reads and writes is PDF p.263 (folio 18-14), "• Memory content setting /
// Command: 1A 00". Citations below give the PDF page first and the printed
// folio in brackets, because Icom's byte diagrams are vector artwork and a
// layout-text line number is neither checkable nor evidence.
//
// The reading is the IC-7851 capability matrix trio under
// docs/superpowers/icom-matrices/ (ic7851-capability-matrix.md rev 1 plus
// Errata 1-15, its report, and its Approved-with-fixes review), and
// core/civ/ic7851's frozen evidence legs under that package's testdata.
//
// NO IC-7851 AND NO IC-7850 HAS EVER BEEN CONNECTED TO THIS PROJECT.
// Every value below is a reading of that document or a stated choice; none
// is a hardware finding, and writeTrialsComplete7851 and
// writeTrialsComplete7850 (caps.go) are FALSE in consequence.
//
// # 1. THE TWO MODELS ARE INDISTINGUISHABLE BY THIS PROGRAMME'S EVIDENCE
//
// New7851 and New7850 are two rows over ONE implementation, ONE
// civ.Profile and ONE address, differing in Model(), Identity and the
// registry row Wave 4 will add. There is no bare New.
//
// THE USER PICKS THE ROW, AND THE PROBE CANNOT NARROW IT (matrix §4):
//
//   - The two share one manual: PDF p.1 (front cover), "THE TRANSCEIVERS /
//     IC-7850 / IC-7851 / Instruction Manual"; PDF metadata Title
//     "IC-7850/IC-7851 Instruction Manual".
//   - They share one CI-V address: PDF p.229 (folio 15-18), item "CI-V
//     Address (Default: 8Eh)", with the note '"8Eh" is the default address
//     of IC-7850/IC-7851.'
//   - They share one frame shape: PDF p.251 (folio 18-2), "◇ Data format",
//     whose diagram legends read "Controller to IC-7850/IC-7851" and
//     "IC-7850/IC-7851 to controller" — the document addresses the pair
//     jointly.
//   - The 19 00 reply value is undocumented for both (§3.12a), so the
//     probe has nothing to separate them with either.
//
// Matrix §4.1 lists the eight places in 283 pages where the document names
// the two models apart, of which exactly ONE is inside Section 18: PDF
// p.255 (folio 18-6), the row `1A 05 0078`, "Send/read screen image type
// (00=A, 01=B, 02=50th Anniversary for only IC-7850)". That is a 1A 05
// menu value, and this tier's gate refuses ALL 1A 05, so it is unreachable
// by this programme on either model. Nothing in the CI-V section
// distinguishes their memory behaviour.
//
// EVIDENCE FOR ONE MODEL IS NEVER EVIDENCE FOR THE OTHER. The two
// write-trial constants are deliberately separate for that reason, and no
// assumption in the register below covers both with one entry: where the
// IC-7850 needs the same assumption it gets its own entry with its own
// lift on an IC-7850 (matrix §4.2).
//
// TestE2E_ProbeFingerprints and TestE2E_ConsentedWriteAndReadback run
// every case for both constructors, which is how the shared-implementation
// claim is exercised rather than merely written down.
//
// # 2. SERIAL FRAMING — ASSUMED 8-N-1, ON NO EVIDENCE FROM THIS DOCUMENT
//
// framing.go implements driver.SerialFramingReporter on the CONCRETE
// driver, returning StopBits() == 1. That is the tier-wide Icom assumption
// (spec D3.1), not a reading of this radio's document.
//
// WHAT THIS DOCUMENT SAYS ABOUT SERIAL FRAMING: NOTHING, ABOUT ANY PORT.
// All 283 pages of extracted text were searched for "stop bit", "start
// bit", "parity", "data bit", "8 bit", "8-bit", "bit/1", "flow control",
// "Xon" and "handshake" — zero hits for every one (matrix §3.1). The only
// occurrence of the word "bit" in the whole document is a sampling-rate
// line in the USB audio specifications.
//
// THE MANDATORY HAZARD SENTENCE, written down even though this document
// prints no such line: Icom manuals print "8 bit / 1 stop bit"-style lines
// about the DATA/RTTY application port, and SUCH A LINE IS NOT EVIDENCE
// ABOUT CI-V SERIAL FRAMING. Only a statement explicitly about the CI-V /
// [REMOTE] / USB CI-V link would count. It is recorded here so a later
// reader does not go looking for one and mistake a neighbour for it.
//
// The pages where a framing statement would live and does not: PDF p.251
// (folio 18-2), the CI-V setup page, which names the three things to set —
// "its address, data communication speed, and transceive function" — and
// framing is not among them; PDF pp.228-229 (folios 15-17, 15-18), where
// every CI-V set-mode item is enumerated and there is no framing item; PDF
// p.274 (folio 20-5), "■ [REMOTE] jack"; and PDF p.267 (folio 19-2), "•
// CI-V connector: 2-conductor 3.5 (d) mm", which is a connector and not a
// framing statement.
//
// MATERIALITY, verified: transport.DefaultStopBits is 2, so a driver that
// did NOT implement this interface would have its port opened at 8-N-2 —
// the silent divergence spec D3.1 exists to prevent. internal/wiring
// consults this and refuses any value but 1 or 2 rather than substituting
// a default. TestStopBits pins the value on both rows and on both
// capability arms.
//
// # 3. CONTROL-LINE POLICY — THIS DRIVER NEVER TOGGLES RTS OR DTR
//
// No page states what the radio expects on RTS or DTR, and there is no
// flow-control setting anywhere (matrix §3.2). The [REMOTE] jack is a
// 2-conductor 3.5 mm mini plug — a single-wire bidirectional bus with no
// modem control lines at all — so on that path the question does not
// arise.
//
// ON THE [USB B] PATH IT DOES, AND THE DOCUMENT SUPPLIES THE HAZARD RATHER
// THAN THE ANSWER. PDF p.229 (folio 15-18) and PDF p.230 (folio 15-19),
// three consecutive items:
//
//	USB SEND           (Default: OFF)  OFF | USB1 DTR | USB1 RTS | USB2 DTR | USB2 RTS
//	USB Keying (CW)    (Default: OFF)  the same five options
//	USB Keying (RTTY)  (Default: OFF)  the same five options
//
// with "USB1 DTR: Use the DTR terminal on the CI-V (PC) side" and "USB1
// RTS: Use the RTS terminal on the CI-V (PC) side".
//
// MANUAL-EVIDENCED: at the factory all three are OFF, so DTR and RTS on
// the CI-V virtual port carry no function. THE HAZARD, RECORDED AND NOT
// RESOLVED: they can be moved to USB1 DTR or USB1 RTS, which binds the
// very lines a host CDC driver typically raises when it opens a port to
// the transmitter's SEND or to CW/RTTY keying. On a radio whose owner has
// done so, a session open that asserted RTS or DTR could key a 200 W
// transmitter.
//
// THE POLICY: transport.OpenSerial already drives both lines low at open,
// before this driver ever sees the port, and THIS DRIVER NEVER TOGGLES
// EITHER. transport.Port is an io.ReadWriteCloser and carries neither
// method, so the only route to one is a type assertion;
// TestOpen_ControlLinesAreNeverToggled hands the driver a port that DOES
// offer both and counts any use, so such an assertion becomes visible the
// moment anyone writes one.
//
// # 4. THE PROBE
//
// Open's whole wire traffic is: NOTHING for Init, one 19 00 read, and up
// to probeSlotCount 1A 00 reads. TestE2E_InitAndProbeSendNothingElse
// compares the exact byte sequence against
//
//		FE FE 8E E0 19 00 FD
//		FE FE 8E E0 1A 00 00 01 FD
//
//	  - NO RADIO MUTATION AT INIT, EVER. The CI-V InitSequence is EMPTY, a
//	    safety property rather than an omission: transceive broadcasts are
//	    excluded STRUCTURALLY, by address filtering, instead of by writing a
//	    transceive-off setting, so opening a session touches nothing outside
//	    the consent regime. No 1A 05, no transceive set, no clear. There is
//	    no admitted frame that could turn transceive off even if this
//	    project wanted one — CI-V Transceive is a set-mode item and 1A 05 is
//	    a tier non-goal.
//
//	  - THE IDENTITY STEP. A 19 00 read to 8E, with an ADDRESS-MATCHED reply
//	    REQUIRED. PDF p.253 (folio 18-4)'s command table prints the row
//	    "19 | 00 | Read the transceiver ID" with NO reply value and no "see
//	    p." cross-reference: no page in this document says what an IC-7851
//	    answers. So the value is RECORDED and NEVER MATCHED —
//	    Session.Identity().CATID is the static address followed by the
//	    observed token, and OpenReport.IDToken keeps the raw bytes for a
//	    future hardware lift to compare against. TestE2E_ProbeFingerprints
//	    opens a session against three different tokens, which is what makes
//	    "never matched" a demonstrated claim.
//
//	  - THE BOUNDED OCCUPIED-SLOT SEARCH. Channels 1..probeSlotCount are
//	    read until one answers with a record. A rejection means "empty, keep
//	    looking", and that branch keys on errors.Is(err,
//	    transport.ErrRejected) because Engine.Do consumes the FA and returns
//	    NO frame. Nothing here calls civ.IsRejection.
//
//	  - THE 25-BYTE FINGERPRINT, AND IT IS CONTINUOUS. Twenty-five is the
//	    RECORD-ONLY length, excluding the two channel-selector bytes; the
//	    data area including them is 27 and the whole set frame is 34. It is
//	    confirmed at the probe and RE-VALIDATED ON EVERY RECORD READ,
//	    because civ.Profile.MemoryAnswerRecord checks the length on every
//	    call. A record at any other length fails with an error satisfying
//	    errors.Is(err, driver.ErrWrongRadio) — see RecordLengthMismatchError,
//	    which NAMES NO FOUND MODEL, because cross-model record-length
//	    distinctness is a TIER-LEVEL WAVE-4 check and this package holds no
//	    table of other radios' lengths.
//	    TestE2E_WrongRecordLengthRefusesTheRadio drives a 24- and a 26-byte
//	    radio.
//
//	  - AN EMPTY RADIO OPENS UNFINGERPRINTED, on address evidence alone
//	    (spec D3.2). Refusing there would make a radio whose memories are
//	    all empty unprogrammable by this programme, which is precisely the
//	    radio a user most wants to programme. The two unverified empty
//	    readings reach the probe differently and
//	    TestE2E_EmptyRadioOpensUnfingerprinted keeps them apart: an FA
//	    returns no frame and teaches the probe nothing, while an all-FF
//	    answer IS a record of the declared length and settles the LENGTH
//	    even though the slot is empty.
//
//	  - THE INIT-UNDER-FLOOD RULE, BOTH HALVES. (a) A BROADCAST flood
//	    (to = 00) never reaches the engine at all: civ's accumulator counts
//	    and NEVER RETURNS those frames, so the drain's idle timer is never
//	    re-armed, Engine.Init SUCCEEDS, and what rises is the ADAPTER's
//	    Unexpected counter — which is why Session.Diagnostics SUMS the
//	    adapter's count with the engine's rather than trusting either.
//	    (b) A CONTROLLER-ADDRESSED flood does reach the engine and can drive
//	    Init's drain to its absolute cap; that INITIAL failure is
//	    NONFATAL-WITH-DIAGNOSTIC (OpenReport.InitDrainCapExceeded), because
//	    the drain is bounded precisely so it cannot fail the open. EVERY
//	    LATER DRAIN FAILURE REMAINS FATAL: once the session is exchanging
//	    frames, a drain that cannot find quiet means this programme can no
//	    longer tell its own answers from somebody else's.
//	    TestE2E_FloodsDoNotStarveTheSession runs a whole 101-slot read under
//	    each flood.
//
//	  - THE TWO LIMITATIONS, STATED PLAINLY (spec D3.3). A radio at a CI-V
//	    address other than 8E TIMES OUT: this driver builds every frame for
//	    8E, the codec refuses any answer not from 8E, and there is no
//	    --civ-address option and no address sweep.
//	    TestE2E_MovedAddressTimesOutCleanly observes it, and observes that
//	    the failure is a timeout rather than a wrong-radio refusal —
//	    nothing was heard from, so nothing can be attributed. A DIFFERENT
//	    ICOM MODEL at ITS factory address fails identically.
//
//	  - Fingerprint, OpenDiagnostics and WireStats are THIS PACKAGE'S
//	    accessors, not neutral-seam additions. driver.SessionDiagnostics
//	    carries ONE aggregate counter, and widening the neutral seam to
//	    carry a per-tier fingerprint is a tier-shared change several
//	    worktrees would want a say in. A deliberate scope choice.
//
// # 5. THE DEFAULT BAUD
//
// DefaultBaud is 19200, graded ASSUMED under register entry
// ic7851-auto-baud-open.
//
// THE ARBITRARINESS IS RECORDED. THIS RADIO HAS NO NUMERIC FACTORY
// DEFAULT: PDF p.228 (folio 15-17), "CI-V Baud Rate (Default: Auto)" and
// PDF p.229 (folio 15-18), "CI-V USB Baud Rate (Default: Auto)", both with
// "When 'Auto' is selected, the baud rate is automatically set according
// to the data rate of the connected controller." The tier probe opens "at
// the model's default baud"; on this model there is no such number, only
// an auto-detector, and the choice of 19200 within the printed set is the
// plan's.
//
// WHAT MAKES IT SAFE IS NOT THE CHOICE BUT THE GRADING AND THE FAILURE
// MODE: the probe requires an address-matched 19 00 reply, and silence is
// silence, so a wrong guess costs A CLEAN TIMEOUT AT Open AND NEVER A
// WRONG BYTE. THE DRIVER CANNOT SWEEP: internal/wiring opens the port from
// Capabilities().DefaultBaud, and this worktree may never touch
// internal/wiring.
//
// THE RATE LIST IS PER-PORT AND spec.Capabilities HAS ONE (matrix
// §3.16.1). [USB B] prints 4800, 9600, 19200, 38400, 57600, 115200 and
// Auto; [REMOTE] (the CT-17 path) prints only 4800, 9600, 19200 and Auto.
// Declaring the USB superset is the CHOICE, and its cost is that the port
// picker will offer 57600 and 115200 to a user who has wired a CT-17 to
// [REMOTE], where the radio cannot go above 19200. A README honesty row at
// Wave 4; register entry ic7851-baud-list-per-port.
//
// # 6. THE E6 RULING AND ITS COST
//
// Ruling E6: A SLOT MAY BE WRITTEN ONLY WHEN ITS UNMAPPED REGIONS EQUAL
// THE PROFILE'S Fixed TEMPLATE; ANYTHING ELSE IS REFUSED WITH THE REASON
// NAMED, NEVER REWRITTEN.
//
// On this model the unmapped regions are byte ③'s LOW nibble — a
// four-valued SELECT-group marker printed "00 : OFF / 01 : ★1 / 02 : ★2 /
// 03 : ★3" (PDF p.263, folio 18-14; matrix §3.16.2) — and byte ⑪'s HIGH
// nibble, a four-valued data mode printed "0: OFF, 1: DATA 1, 2: DATA 2,
// 3: DATA 3". Their neutral homes, codeplug.ChannelData.ScanSkip and
// .DataMode, are BOTH BoolField.
//
// ON ICOM, scan_skip IS SELECT-GROUP MEMBERSHIP, NEVER A SKIP. A 4-to-2
// collapse would rewrite a user's select group or data mode on every
// write-back while readback verification compared equal.
//
// THE COSTS, stated as E6 requires:
//
//   - A CHANNEL THAT IS IN A SELECT GROUP (★1/★2/★3), OR WHOSE DATA MODE
//     IS DATA 1/2/3, CANNOT BE WRITTEN BY THIS PROGRAMME AT ALL. The write
//     is REFUSED with the reason named. It is NEVER silently downgraded to
//     ★1/DATA 1 and NEVER silently cleared to OFF.
//   - Those two fields cannot be read back, exposed or edited: they are
//     Unavailable on every read, because an unmapped region is not
//     decoded.
//   - EVERY ELIGIBLE WRITE COSTS ONE EXTRA READ EXCHANGE — the E6
//     comparison read, which is also tier ruling T5's one recorded
//     exception to "refusal before any wire traffic".
//
// TestE6RefusesNonTemplateUnmappedBytes and TestE6AcceptsTemplate pin the
// three nibbles and the one that is mapped.
//
// # 6b. THE THREE PRINTED-FIXED BYTES, AND WHY THEY ARE NOT SPANNED
//
// Three record bytes are drawn with a literal 0 in BOTH nibbles: ⑧, whose
// rotated leaders read "1000 MHz digit: 0 (Fixed)" and "100 MHz digit:
// 0 (Fixed)" (PDF p.260, folio 18-11; matrix §3.16.3), and ⑫ and ⑮, the
// leading cell of the repeater-tone diagram both tone triples point at,
// whose leaders read "Fixed digit: 0*" (PDF p.262, folio 18-13; matrix
// §3.16.4).
//
// core/civ/ic7851's layout therefore stops each numeric span SHORT of its
// fixed byte and lets the Fixed template supply it. civ.FieldSpan carries
// no numeric domain, so a span covering one of those bytes would encode a
// digit into it for any large enough value AND re-encode it identically at
// the gate; the template only supplies bytes no span maps.
//
// THE COST OF THAT EXCLUSION IS ON THE READ PATH, and this driver pays it
// rather than hiding it: the record parser no longer looks at those bytes,
// so a radio answering with a digit in one would be read as a value 100
// times smaller and written back with the byte quietly zeroed. readRaw
// refuses such a record instead (*FixedDigitError), after the all-FF empty
// branch and before the parse. TestFixedDigitBytesAreRefusedOnRead and
// TestE2E_FixedDigitRecordIsRefusedOnTheWire cover both halves.
//
// # 7. THE ONE APPROXIMATION IN THE WRITE GATE'S FIELD SET
//
// write.go's conditionalRequestedFields appends every state-bearing field
// OUTSIDE the base set when it is Known, so the capability gate can REFUSE
// a request rather than drop it. Every tri-state predicate is exact, and
// so are the two plain-string ones — CTCSS and Shift — because an empty
// string is a vocabulary member of no radio.
//
// THE CLARIFIER IS THE APPROXIMATION. codeplug.ChannelData carries it as
// ClarHz plus two bools, with no state, so a channel asking for an
// explicitly-ZERO clarifier is indistinguishable from one that never
// carried a clarifier at all: FieldClarifier is not appended and the gate
// never sees the request.
//
// ON THIS MODEL IT COSTS NOTHING, and the reason is worth writing down
// rather than assuming: the 1A 00 record has no clarifier span, so there
// is nothing to write even if the gate saw it; and core/codeplug's own
// touchedFields treats clarifier as one of the unconditional six and
// FILTERS IT OUT on a bank that cannot reach the field, so the clone
// service never requests it either. An honesty row, and one that would
// need re-examining for any Icom model whose record DOES carry a
// clarifier.
//
// # 8. THE DEFERRED GATE-DOMAIN GAP, RECORDED AND NOT PAPERED OVER
//
// civ.FieldSpan HAS NO NUMERIC DOMAIN. civ's validateSpanValue checks only
// BCD width and scale, so civ.Profile.AllowedCommand — the last defence
// before a radio sees bytes — WOULD ADMIT a 1A 00 set carrying 65 MHz,
// which is above this radio's printed 60 MHz receiver ceiling and still
// inside what the four variable frequency bytes can encode.
//
// WHAT IS ALREADY CLOSED, verified in code: codeplug.Validate bounds the
// primary frequency against caps.MinFreqHz/MaxFreqHz, and the tone range
// bounds tones. So EVERY PATH THAT REACHES THIS DRIVER THROUGH THE MODEL
// LAYER IS ALREADY COVERED; the residual exposure is the gate's own
// blindness to a frame arriving by any other route.
//
// WHAT THIS DRIVER DOES ABOUT IT: WriteChannel's rung 4 carries a
// driver-level PRE-BUILD TYPED REFUSAL (*OutOfDomainError) for a Known
// frequency or tone outside THIS SESSION'S declared capability bounds —
// both ends of both domains. THIS IS DEFENCE IN DEPTH AND IT IS NOT THE
// GATE.
//
// TestNumericRefusalIsDefenceInDepthNotTheGate asserts that the gate
// currently ADMITS such a frame while the driver refuses it. An enabler
// that closes the gap turns that test red, which is a reviewable change
// rather than a silent one. DO NOT DELETE THAT TEST TO "FIX" IT. The
// follow-up's likely shape: FieldSpan gains optional Min/Max in the
// field's neutral unit, enforced in validateSpanValue on both the encode
// and the gate paths.
//
// # 9. ERASE
//
// FieldErase carries the zero FieldSupport in BOTH capability arms;
// spec.ConsentUnverifiedWrites structurally never consents it (its
// `f != FieldErase` guard); and core/clone/execute.go's DiffErased branch
// stays UNREACHABLE for this model.
//
// UNLIKE YAESU, THE WIRE FORM EXISTS HERE, IN TWO SHAPES (matrix §3.13,
// both MANUAL-EVIDENCED as printed forms, both recorded as evidence only):
//
//   - (a) The 1A 00 clear form. PDF p.263 (folio 18-14), the hatched note
//     immediately under the ①,② legend: "To clear the memory channel
//     contents, add the code "FF" after the memory channel number.
//     (instead of the data ③ to ㉗) This completes the memory clearing." —
//     i.e. FE FE 8E E0 1A 00 <ch-hi> <ch-lo> FF FD, a 3-byte data area
//     rather than 27. The gate refuses it because a 1-byte record-only
//     payload is not in the declared singleton {25}. Note also that the
//     clear note's channel range is printed 00 01~00 99 only: it does not
//     name the scan edges, and PDF p.181 (folio 11-2) gives the Scan Edge
//     row CLEAR = "No" independently.
//   - (b) Top-level command 0B. PDF p.252 (folio 18-3), "◇ Command table",
//     three consecutive rows with no sub-command and no data: 09 "Memory
//     write", 0A "Memory to VFO", 0B "Memory clear".
//
// NO CLEAR COMMAND EXISTS IN THIS TIER — a CHOICE fixed by the tier spec.
// There is no builder for any of those forms, and core/civ's
// AllowedCommand admits only 19 00, a valid 1A 00 read and a re-validated
// 1A 00 set. TestWriteChannel_NoClearFrameIsReachable asserts the gate
// refuses all four frames and that no capability arm — consented or not —
// makes FieldErase writable; TestE2E_BothEraseFormsAreRefused asserts the
// driver emits none of them across a whole read/write cycle and that the
// radio itself answers FA to each.
//
// WHAT A FUTURE WRITE-TRIAL MILESTONE WOULD NEED (matrix §3.13): a
// hardware-verified read path, so a cleared channel can be confirmed
// cleared; a captured answer to each of the two clear forms, showing which
// is acknowledged and what the front panel then displays; a FieldErase
// FieldSupport value earned by that capture rather than assumed; a builder
// and a gate admission for whichever form the capture validates; and
// neither per-model write-trial guard may move off FALSE.
//
// # 10. RULING OQ1 — THE RADIX OF THE PRINTED MODE CODES
//
// RULING OQ1 (orchestrator): THE PRINTED MODE CODES ARE HEXADECIMAL, so
// PSK is the wire byte 0x12 and PSK-R is 0x13.
//
// REASON: the document prints every command, sub-command and address as
// hexadecimal byte pairs and PDF p.260 (folio 18-11)'s "① Operating mode"
// column is a column of the same kind. The ruling touches PSK and PSK-R
// alone — the other eight codes are identical under either reading.
// Reversing it is the ORCHESTRATOR'S to do and not an editor's;
// core/civ/ic7851/crosscheck_test.go says so at the comparison that would
// first show the change.
//
// # THE NINETEEN REGISTER ENTRIES
//
// The matrix implies exactly nineteen, every one scoped to the IC-7851 and
// every one carrying exactly ONE lift on an IC-7851 itself. NONE OF THEM
// COVERS THE IC-7850: where that model needs the same assumption it gets
// its own entry on its own row with its own lift.
//
// Six are homed in the civ PROFILE register (core/civ/ic7851/doc.go) and
// thirteen here. All nineteen are listed together, because a reader of
// either register must be able to see that the set is not complete without
// the other.
//
//	ENTRY                              HOME    LIFT (one, on an IC-7851)
//	ic7851-read-request-form           driver  Send FE FE 8E E0 1A 00 00 01 FD; record the answer frame in full.
//	ic7851-empty-reply-fa              driver  Clear M-CH50 at the front panel; read it; record the answer verbatim.
//	ic7851-all-ff-record               driver  If that read returns a full-length record, record its 27 bytes and whether every data byte is FF.
//	ic7851-name-pad-byte               driver  Name M-CH01 "AB"; read back; record bytes ⑲-㉗.
//	ic7851-name-digit-space-codes      driver  Name M-CH02 "A 1"; read back; record bytes ⑱⑲⑳.
//	ic7851-record-length               civ     Read M-CH01; count the bytes between the sub-command and FD.
//	ic7851-id-reply-value              driver  Send 19 00 at 8Eh; record the answer frame verbatim.
//	ic7851-serial-framing              driver  Open [USB B] at 9600 8-N-1, send 19 00, confirm an address-matched answer; repeat at 8-N-2 and 8-E-1.
//	ic7851-broadcast-address-form      driver  With transceive ON, turn the main dial while capturing; record the destination byte of the command-00 frames.
//	ic7851-auto-baud-open              driver  Power on at factory defaults, open at 19200, send one 19 00; record whether it is answered first time.
//	ic7851-rts-dtr-at-open             driver  With USB SEND = USB1 RTS, open the port with this programme's settings; record whether the radio transmits.
//	ic7851-echo-link-to-remote         driver  At factory defaults (echo-back OFF, port Linked), send 19 00 over [USB B]; record whether the sent frame is read back.
//	ic7851-write-ack-fb                driver  Send a 1A 00 set for M-CH99 with a space-containing name; record the answer — FB, FA or silence.
//	ic7851-tone-step-domain            driver  Send 1B 00 with an off-chart tone (e.g. 0700); read back; record accept, clamp or FA.
//	ic7851-baud-list-per-port          driver  Confirm 19 00 at 115200 over [USB B]; then at 19200 and 38400 over [REMOTE] with a CT-17.
//	ic7851-select-memory-vocabulary    driver  Set M-CH03 to ★2 at the front panel; read back; record byte ③.
//	ic7851-fixed-nibble-reencode       civ     Read any programmed channel; record byte ⑧ to confirm it is 00.
//	ic7851-tone-fixed-byte             civ     Set M-CH04's repeater tone to 88.5 Hz; read back; record bytes ⑫⑬⑭ (expected 00 08 85).
//	ic7851-memory-freq-bounds          driver  Store 0.030000 MHz in M-CH01 at the front panel; read the channel back.
//
// THREE OF THOSE LIFTS ARE THIS DRIVER'S OWN GRADING, restated where the
// value lives: ic7851-id-reply-value beside the probe (§4), ic7851-serial-
// framing beside StopBits (framing.go), and ic7851-memory-freq-bounds
// beside MinRadioFreqHz/MaxRadioFreqHz (caps.go). The rest are cited at
// the code that depends on them.
//
// TWO PRINTED FACTS AND TWO ASSUMPTIONS SIT BESIDE EACH OTHER AND MUST NOT
// BE CONFLATED. Transceive ON (PDF p.229, "CI-V Transceive (Default: ON)")
// and USB echo-back OFF (same page, "CI-V USB Echo Back (Default: OFF)")
// are PRINTED DEFAULTS. That a transceive broadcast reaching this
// controller carries the destination byte 00, and that a linked
// USB/[REMOTE] echo is absent or structurally removable, are ASSUMPTIONS —
// entries ic7851-broadcast-address-form and ic7851-echo-link-to-remote.
// The document prints 00h in a CI-V address context exactly once and it is
// a DIFFERENT setting ("CI-V USB/LAN➡REMOTE Transceive Address (Default:
// 00h)", which governs the address the radio uses when it BRIDGES a
// control signal out of [REMOTE]). Matrix §3.5 records both and resolves
// neither.
//
// # A procedural fact every capture above inherits
//
// PDF p.252 (folio 18-3)'s grey NOTE panel: "Operating some control dials
// overrides CI-V commands. If a control dial (such as the AF Volume dial
// that has a mark on it) is rotated after sending a CI-V command, the
// command will be overwritten by the operation." MANUAL-EVIDENCED. On this
// radio a readback that disagrees with what was written is not necessarily
// a protocol fault — it may be a hand on the front panel. EVERY lift above
// must record that no control was touched during the capture.
//
// # Wave-4 hand-off
//
// NEITHER ROW IS REGISTERED, deliberately: registration, the README and
// radiotext rows, and the tier's record-shape table are Wave 4's, not this
// worktree's. This package joins the DECLARED INDISTINGUISHABLE SET
// {IC-7610, IC-7851/IC-7850, IC-7760} at record-only 25 bytes and flat
// two-byte geometry, and TestTierRecordShapes_DistinctOrDeclared remains
// an icom-core responsibility.
package ic7851

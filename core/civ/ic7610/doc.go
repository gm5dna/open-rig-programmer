// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7610 holds the Icom IC-7610's CI-V dialect: the memory record's
// geometry, its three value enums, its name charset and the civ.Profile that
// binds them. It is DATA ONLY - no driver, no fake, no registration, no
// session, no wire. The package cannot register itself with the application:
// SupportedModels derives solely from internal/wiring's driver table.
//
// # Provenance
//
// Everything here comes from the IC-7610 CI-V Reference Guide rev 4
// (docs/fixtures-private/manuals/, gitignored, so the page references below
// are citations rather than links), read four times over by four QUARANTINED
// agents that never opened this repository and never saw one another's work:
//
//   - L, the field ledger - labels and printed indices
//     (testdata/ic7610-field-ledger.{md,csv});
//   - W, the geometry witness - byte and nibble positions measured off
//     400-500 dpi raster renders (testdata/ic7610-geometry-witness.{md,csv});
//   - B, the semantic transcription - widths, encodings and value lists,
//     derived without sight of L (testdata/ic7610-transcription-b.{md,csv});
//   - G, the golden vectors and their assumption register
//     (testdata/ic7610-vectors.golden, ic7610-golden-assumptions.csv,
//     ic7610-golden-provenance.md).
//
// crosscheck_test.go, geometry_test.go and golden_test.go bind those four
// readings to this package's profile and to one another. Agreement between
// independent blind derivations is the evidence; those tests are where it is
// made mechanical rather than asserted in prose. freeze_test.go hashes all
// nine artefacts, and no test in this package may modify one: a disagreement
// between an artefact and the codec is a STOP for orchestrator arbitration
// AGAINST THE PDF, never a fixed artefact.
//
// The capability matrix (docs/superpowers/icom-matrices/
// ic7610-capability-matrix.md rev 1 plus its Errata) is the graded evidence
// authority every section number cited below points into.
//
// NO IC-7610 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project. Every
// statement in this package is a reading of a manual, and the register at the
// foot of this file is the list of the places where it is not even that.
//
// # The three lengths, and the offset rule worked through
//
// Spec Erratum 1 requires a per-radio package to pin BOTH length conventions
// with the address width named, because carrying one number without the other
// is the single failure the matrix review called downstream-fatal:
//
//	RecordOnlyLength  25   the 1A 00 data block EXCLUDING the two channel
//	                       selector bytes (1),(2) - what civ.Profile carries,
//	                       and what BuildMemorySet's <record> denotes
//	DataAreaLength    27   the 1A 00 data block INCLUDING them - the matrix's
//	                       own eight-term addition (S3.11), whose sum equals
//	                       the last printed index, (27)
//	AddressBytes       2   <ch-hi> <ch-lo>, civ.AddressFormFlat
//
// A 1A 00 set frame is therefore 34 bytes (6 + 2 + 25 + 1), a 1A 00 read
// frame 9 (6 + 2 + 1) and a 19 00 read frame 7 (6 + 1).
//
// civ.FieldSpan.Offset is 0-BASED FROM THE START OF THE RECORD, and the
// record begins at printed index (3) because (1),(2) are the address and lie
// outside it. So:
//
//	record byte      = printed index - 2     (1-based within the record)
//	FieldSpan.Offset = printed index - 3     (0-based)
//
//	printed    record bytes  Offset  Length  mapped to
//	(1),(2)    -             -       2       the ADDRESS, outside the record
//	(3)        1             0       1       UNMAPPED (E6)
//	(4)~(8)    2-6           1       5       rx_frequency
//	(9)        7             6       1       mode
//	(10)       8             7       1       filter
//	(11) hi    9             8       -       UNMAPPED (E6)
//	(11) lo    9             8       1       tone_mode, NibbleLow
//	(12)~(14)  10-12         9       3       tone_tx
//	(15)~(17)  13-15         12      3       tone_rx
//	(18)~(27)  16-25         15      10      name
//
// The widths sum to 1+5+1+1+1+3+3+10 = 25. The accepted record-length set is
// {25}, a single-length profile under civ.DiscriminatorSingleLength, and that
// set is the probe's fingerprint (spec D3.2). THIS PACKAGE MAKES NO
// CROSS-MODEL DISTINCTNESS CLAIM: whether 25 tells this radio apart from its
// siblings is a tier-level Wave-4 check, not one made here.
//
// # The two channel-selector bytes are the ADDRESS, by tier convention
//
// PDF p.12 draws (1),(2) INSIDE the 1A 00 data block, as the strip's own
// first brace, and the matrix's S3.11 scope note reads the page that way
// correctly: on the page they are record content, not a separate address
// field. The TIER's convention (spec Erratum 1) nevertheless puts them
// outside the record and inside civ.ChannelAddress, so that one accounting
// serves six models. Both statements are true of different things, and the
// pair of length constants above is how this package keeps them straight.
//
// # Ruling E6: the two nibbles this record deliberately does not map
//
// Two of this record's regions carry FOUR-VALUED radio fields whose neutral
// homes are BOOLEAN:
//
//   - byte (3)'s LOW nibble is a select-scan GROUP marker, printed
//     0=OFF / 1=(star)1 / 2=(star)2 / 3=(star)3 (matrix S3.16 ADDED-1). Its
//     neutral home, codeplug.ChannelData.ScanSkip, is a BoolField. It is not
//     a skip flag at all: a non-zero (3) puts the channel into one of three
//     named SELECT groups a select-memory scan can be pointed at. On Icom,
//     scan_skip is SELECT-group membership, never a skip.
//   - byte (11)'s HIGH nibble is a four-valued data mode, printed
//     0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3 (matrix S1b as corrected by
//     erratum 5). Its neutral home, codeplug.ChannelData.DataMode, is also a
//     BoolField.
//
// A 4->2 collapse would rewrite a user's SELECT group or data mode on every
// write-back while readback verification compared EQUAL - the clone service's
// own check would not catch it, because the value would match what the driver
// believes it sent. TIER RULING E6 settles this without a local choice:
//
//	a driver may write a slot ONLY when its unmapped regions equal the
//	profile's Fixed template; anything else is REFUSED with the reason
//	named, never rewritten.
//
// So both nibbles are UNMAPPED in the layout, the profile carries an explicit
// 25-byte all-zero Fixed template, and SelectNibbleOffset and
// DataModeNibbleOffset are exported so the driver's refusal check names the
// two regions rather than re-deriving them. Byte (3)'s HIGH nibble is the
// page's own printed Fixed 0 and is unmapped for that separate reason.
//
// THE COSTS, STATED AS E6 REQUIRES:
//
//   - A channel that is in a SELECT group ((star)1/(star)2/(star)3), or whose
//     data mode is DATA 1/2/3, CANNOT BE WRITTEN BY THIS PROGRAMME AT ALL -
//     the write is REFUSED with the reason named. It is never silently
//     downgraded to (star)1/DATA 1 and never silently cleared to OFF.
//   - Those two fields cannot be read back, exposed or edited: they are
//     Unavailable on every read, because an unmapped region is not decoded.
//   - Every eligible write costs one extra read exchange, the E6 comparison.
//   - These grades DIFFER from matrix S2, which has scan_skip Sup/Sup on MEM
//     and data_mode Sup/Sup on both banks. Two matrix errata were proposed
//     under adjudication R16 and adjudicated 24/08/2026.
//
// The gate enforces the same thing independently: Profile.AllowedCommand
// decodes, re-validates and re-encodes byte-identically, so a set carrying a
// non-zero unmapped nibble is refused at the last defence too.
//
// # No clear builder, though this radio documents TWO clear forms
//
// Unlike the Yaesu models, where the wire form does not exist, THE WIRE FORM
// EXISTS HERE, IN TWO SHAPES:
//
//   - PDF p.12 (folio 11), right column, "To clear the memory channel
//     contents on 1A 00:", printing three lines - (1),(2): Memory channel
//     (00 01~00 99) / (3): "FF" / (4): None - i.e. the frame
//     FE FE 98 E0 1A 00 <ch-hi> <ch-lo> FF FD;
//   - PDF p.4 (folio 3), the command table's row 0B | (blank) | Memory clear,
//     which is a whole command, no sub-command, no data, clearing the
//     currently selected memory channel.
//
// core/civ ships NO clear builder, and Profile.AllowedCommand has no branch
// that could admit either frame: it admits only 19 00, a valid 1A 00 read and
// a re-validated 1A 00 set. crosscheck_test.go's D4 leg and golden_test.go's
// gate table both prove the refusal rather than leaving it to be believed.
// Every Icom driver gives spec.FieldErase the zero FieldSupport,
// spec.ConsentUnverifiedWrites structurally never consents erase, and
// core/clone/execute.go's DiffErased branch stays unreachable.
//
// WHAT A FUTURE WRITE-TRIAL MILESTONE WOULD NEED, all of it, on an IC-7610
// somebody actually owns (matrix S3.13's six items):
//
//  1. a hardware-verified read path, so a cleared channel can be confirmed
//     cleared;
//  2. a resolution of the (3) Fixed-0 versus "FF" conflict below, FROM THE
//     RADIO rather than from the page;
//  3. a captured answer to each of the two clear forms, showing which is
//     acknowledged and what the front panel then displays;
//  4. a spec.FieldErase FieldSupport value earned by that capture rather than
//     assumed;
//  5. a builder and a gate admission for whichever form the capture
//     validates;
//  6. writeTrialsComplete moved off FALSE for this model.
//
// Until every one of those lands, DiffErased stays unreachable.
//
// # The (3) Fixed-0 versus "FF" contradiction - RECORDED, RECONCILED BY
// # NEITHER
//
// The (3) sub-diagram on PDF p.12 prints the byte as 0 | X with its left
// nibble's leader labelled Fixed - which admits no value but 0 - while the
// clear list four inches to the right on the SAME PAGE prints (3): "FF",
// whose high nibble is F. The landed G leg recorded the identical conflict as
// its STOP 2 and built no clear vector; the matrix (S3.8(b)) records both
// statements and reconciles neither; and neither does this package. The
// Fixed template's byte 0 is 0x00 because that is what the RECORD diagram
// prints for a record; the clear list describes a different frame this tier
// does not build.
//
// # The PDF p.14 asterisk, and why it is not a second record length
//
// PDF p.14 (folio 13)'s tone strip captions its first cell (1)* and prints
// *Not necessary when setting a frequency. beneath it. That is the one
// conditional width printed anywhere near these fields, and it is NOT read
// across into a second 1A 00 record length, for three reasons the matrix
// (S3.11) states rather than adjudicates:
//
//  1. the asterisk is attached to command 1B 00 / 1B 01's own data area, on
//     the page that defines those commands, not to 1A 00;
//  2. the 1A 00 strip on PDF p.12 allots (12) and (15) their own printed
//     indices inside an index run complete from (1) to (27); dropping either
//     would leave a hole in that run;
//  3. the landed G leg recorded the identical conflict as its STOP 1 and
//     built its vector from the memory-content strip, deriving no total other
//     than 34 frame bytes (27 record + 7 overhead).
//
// # ADDED-3: a front-panel control can overwrite a command already sent
//
// PDF p.4 (folio 3), the grey NOTE: panel at the head of the page:
// "Operating some control dials overrides CI-V commands. If a control dial
// (such as the AF Volume dial that has a mark on it) is rotated after sending
// a CI-V command, the command will be overwritten by the operation."
// MANUAL-EVIDENCED. It changes what silence and what a mismatched readback
// MEAN during a capture: on this radio a readback that disagrees with what
// was written is not necessarily a protocol fault, it may be a hand on the
// front panel. EVERY Stage R and Stage W capture procedure named below must
// therefore record that no control was touched during the capture. No
// register entry: this is a procedural fact about capture, not a claim
// awaiting a lift.
//
// # The CI-V time-out timer
//
// PDF p.6 (folio 5) prints 1A 05  00 31  00 ~ 05  Function > Time-Out Timer
// (CI-V) (00=OFF, 01=3 min. ... 05=30 min.). It is not a framing line, not a
// rate and not a memory-record field, so it constrains nothing this package
// declares - but it is a CI-V-SCOPED TRANSMIT TIME-OUT that could cut a long
// Stage W capture short, and it is recorded here so the person taking one
// knows to look at it.
//
// # THE REGISTER
//
// Every entry names the assumption, what depends on it, and the ONE capture
// that lifts it. The captures are individual on purpose: a single IC-7610
// session does not retire this register wholesale, it retires the assumptions
// its own frames actually speak to, and an entry whose capture was not taken
// stays here afterwards. No entry covers two models, and no entry is lifted
// by a capture on another radio.
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION. A positional citation
// ("entry 6") is correct only until somebody adds or reorders an entry, and
// it then silently points at the wrong assumption rather than failing.
//
// THERE ARE THREE HOMES AND THIS FILE OWNS TWO. The matrix's S5 table assigns
// each entry a home; the DRIVER-homed entries are listed at the foot of this
// file by name and R-number and reproduced in full in
// core/driver/ic7610/doc.go, never duplicated in both places.
//
// # D5 register (spec-level, shared across the tier)
//
//   - D5 entry 1 - the 1A 00 READ-REQUEST FORM (R1). ASSUMED: that a read is
//     1A 00 <ch-hi> <ch-lo> with no data bytes, nine frame bytes in all. The
//     document prints no 1A 00 read request anywhere; the selector VALUES are
//     manual-evidenced (PDF p.12's (1),(2) value list), what is assumed is
//     that a read carries them at all. WHAT DEPENDS ON IT: every read this
//     driver makes, and therefore the probe, the empty-slot search and the
//     E6 preservation read. It is the G leg's single inherited_assumed run,
//     bytes 7-8 of the read-record vector.
//     STAGE R LIFTS IT WITH: capture ic7610-read-request-ch01 - send exactly
//     FE FE 98 E0 1A 00 00 01 FD to an IC-7610 at its default address and
//     record every byte it returns. Scope: whether THAT nine-byte frame is
//     accepted as a read of memory channel 01 on that radio. It settles
//     nothing about a 1A 00 read carrying no data bytes, nothing about any
//     other channel number, and nothing about any other model.
//
//   - D5 entry 2(a) - AN UNWRITTEN CHANNEL ANSWERS FA (R2a). ASSUMED. PDF
//     p.3's NG strip prints FA under a rotated NG code (Fixed) leader; it
//     does NOT say that an unwritten memory channel is what provokes it, and
//     nothing in the 17 pages describes a read of an empty channel. WHAT
//     DEPENDS ON IT: the driver's empty-slot recognition, which keys on
//     errors.Is(err, transport.ErrRejected) - the framing consumes an FA and
//     returns no frame (tier ruling T4).
//     STAGE R LIFTS IT WITH: capture ic7610-empty-read-ch99 - on an IC-7610
//     whose memory channel 99 has been confirmed blank from the front panel,
//     send FE FE 98 E0 1A 00 00 99 FD and record every byte returned. Scope:
//     what THAT radio returns for THAT one unwritten channel. It says nothing
//     about an all-FF record, nothing about the scan edges, and nothing about
//     any other model.
//
//   - D5 entry 2(b) - AN ALL-FF RECORD READ BACK MEANS EMPTY (R2b). ASSUMED,
//     AND UNRESOLVED EITHER WAY. The only FF this document attaches to the
//     memory record is on the WRITE side, in the clear list above; that is a
//     clear COMMAND whose byte (3) is FF, not a statement about what a read
//     returns. THIS PACKAGE MAKES NO SUCH CLAIM: golden_test.go's
//     TestGolden_AllFFRecordFailsToParse pins that an all-FF record fails to
//     decode with a parse error naming an offset, because 0xF is in none of
//     the three mapped enums, and records the question rather than answering
//     it.
//     STAGE W LIFTS IT WITH: capture ic7610-ff-record-ch50 - write a 27-byte
//     record of FF bytes to memory channel 50 with 1A 00, then read channel
//     50 back and photograph the front-panel channel display. Scope: what
//     THAT radio does with THAT one all-FF record on THAT one channel.
//
//   - D5 entry 3 - THE MEMORY-NAME SPACE CHARACTER IS 0x20 (R3). ASSUMED, and
//     it is NameCharset's fourth source. Neither 1A 00 character table prints
//     a space row, while the same block's footnote lists "(space)" among the
//     usable memory-name characters; the document prints a space's ASCII code
//     twice, both times under OTHER commands (PDF p.11's CW message contents,
//     "Space | 20"; PDF p.14's memory keyer entries, "space | 20 | Word
//     space"). The G leg derived its set-record vector's byte 28 from those
//     rows. WHAT DEPENDS ON IT: whether a name containing a space is
//     writable at all.
//     STAGE W LIFTS IT WITH: capture ic7610-name-space-ch01 - write memory
//     channel 01 with the ten name bytes 48 4F 4D 45 20 51 54 48 30 31
//     ("HOME QTH01", a space at position 5) and photograph the front-panel
//     memory-name display. Scope: whether 20 at THAT one position renders as
//     a space on THAT radio.
//
//   - D5 entry 3 - THE MEMORY-NAME PAD BYTE IS 0x20 (R4), graded separately
//     from the space code and from the length. ASSUMED. The field is a fixed
//     ten bytes but the legend says "Up to 10 characters."; the document
//     nowhere states what a controller sends, or what the radio returns, for
//     a name shorter than ten. The G leg avoided the gap by writing a full
//     ten-character name. WHAT DEPENDS ON IT: every short name, in both
//     directions - civ.ProfileConfig.NamePad pads outbound and trims inbound,
//     so a name ENDING in a space does not round-trip.
//     STAGE R LIFTS IT WITH: capture ic7610-name-pad-ch02 - set memory
//     channel 02's name to the three characters ABC from the front panel,
//     then read channel 02 with 1A 00 and record record bytes 18-27 verbatim.
//     Scope: what THAT radio puts in the seven unused name bytes of THAT one
//     channel.
//
//   - D5 entry 5 - WIRE ORDER EQUALS PRINTED INDEX ORDER (R5). ASSUMED. This
//     model makes the assumption easy - there is no duplicated TX block, so
//     the printed index order and the wire order SHOULD coincide - and
//     "should" is not "does". The printed indices are LOGICAL; W records wire
//     positions, B records printed indices; crosscheck_test.go pins the
//     mapping. WHAT DEPENDS ON IT: every offset in THE ONE TABLE above.
//     STAGE R LIFTS IT WITH: capture ic7610-wire-order-ch01 - set memory
//     channel 01 from the front panel so that every field takes a distinct,
//     unambiguous value (14.250000 MHz, USB, FIL2, DATA 1, TONE, repeater
//     tone 88.5 Hz, tone squelch 100.0 Hz, name HOME QTH01, select (star)2),
//     read it with 1A 00, and record all 27 data bytes in order. Scope: the
//     wire position of each field in THAT one record on THAT radio.
//
//   - D5 entry 6 - THE RECORD IS 27 DATA-AREA BYTES (R6). ASSUMED, because
//     the 27 is a DERIVATION and not a printed value: every addend of the
//     matrix's eight-term addition (S3.11) is manual-evidenced, the total is
//     not. The document prints no record byte-count of any kind for 1A 00,
//     swept across all 17 pages. The internal check the strip offers is
//     nevertheless exact - the sum equals the last printed index, (27), and
//     the eight groups tile 1-27 with no gap and no overlap - and the landed
//     W leg's own position arithmetic reaches the same eight widths and the
//     same total independently. WHAT DEPENDS ON IT: RecordOnlyLength,
//     DataAreaLength, the single-length accepted set {25}, and therefore the
//     probe's fingerprint.
//     STAGE R LIFTS IT WITH: capture ic7610-record-length-ch01 - read memory
//     channel 01 with 1A 00 and count the data bytes lying between the
//     answer's 1A 00 <ch-hi> <ch-lo> and its terminating FD. Scope: the
//     length of THAT one answer from THAT radio.
//
//   - D5 entry 7 - THE 19 00 REPLY VALUE (R7), undocumented on ALL SIX
//     models. ASSUMED only in the sense that nothing is claimed: the reply
//     value is printed nowhere in this document (17 pages swept at 300 dpi),
//     so the probe RECORDS it in diagnostics and NEVER MATCHES it - an
//     address-matched reply is required, its content is not.
//     golden_test.go's TestGolden_TransceiverID asserts the token comes back
//     and deliberately asserts nothing about WHICH token. WHAT DEPENDS ON IT:
//     nothing in this package's behaviour, and that is the point.
//     STAGE R LIFTS IT WITH: capture ic7610-id-1900 - send
//     FE FE 98 E0 19 00 FD to an IC-7610 at its default address and record
//     the answer frame verbatim. Scope: what THAT radio answers to THAT
//     frame.
//
//   - D5 entry 8 - SERIAL FRAMING IS 8-N-1 (R8), i.e. the driver's
//     StopBits() == 1. ASSUMED, on NO evidence from this document: the
//     nearest rows are a CI-V baud-rate menu item, a scope-waveform footnote
//     naming 115200, and - the trap - a Decode Baud Rate row three rows below
//     the CI-V rows in the same column, which is the internal RTTY/PSK
//     DECODER's rate and must never be read as a framing statement. WHAT
//     DEPENDS ON IT: core/driver/ic7610's SerialFramingReporter, and
//     therefore what internal/wiring opens the port at.
//     STAGE R LIFTS IT WITH: capture ic7610-framing-8n1 - with an IC-7610 at
//     its factory CI-V settings, open its USB CI-V endpoint at 8-N-1 and then
//     at 8-N-2, send FE FE 98 E0 19 00 FD at each, and record which framing
//     returns a well-formed address-matched frame and which returns nothing
//     or garbage. Scope: which framing THAT radio's USB CI-V endpoint
//     accepts, and nothing wider - not the [REMOTE] jack, not the [LAN] port,
//     and not any other model.
//
//   - D5 entry 9 - TRANSCEIVE BROADCASTS CARRY to = 00 (R9). ASSUMED. This
//     document prints no transceive broadcast frame; the only
//     answer-direction skeleton it prints (PDF p.3's IC-7610 to controller
//     strip) shows to = E0, the SOLICITED case. WHAT DEPENDS ON IT: the whole
//     no-mutation-at-Init design - broadcasts are excluded by ADDRESS
//     FILTERING rather than by writing a transceive-off setting to the radio,
//     and civ's accumulator counts and drops every frame whose to byte is not
//     the controller's.
//     STAGE R LIFTS IT WITH: capture ic7610-transceive-broadcast - with CI-V
//     Transceive ON, connect an IC-7610, turn the main dial, and record the
//     to byte of every unsolicited frame the radio emits. Scope: the to byte
//     THAT radio uses for its own broadcasts, and nothing about any other
//     model.
//
// # civ PROFILE register (this package's own)
//
//   - ic7610-transceive-factory-default (R10) - that CI-V Transceive is ON at
//     the factory. ASSUMED: the document prints the transceive commands (PDF
//     p.4's rows 00 and 01, "Send frequency data (transceive)" and "Send mode
//     data (transceive)") but marks no default. WHAT DEPENDS ON IT: how much
//     unsolicited traffic a session should expect, and therefore whether the
//     broadcast-flood path is the common case or the rare one. NOTHING in
//     this package changes if it is wrong: no transceive-off command is
//     written, core/civ ships no transceive-set builder, the gate admits
//     none, and the framing's InitSequence() is EMPTY.
//     STAGE R LIFTS IT WITH: capture ic7610-transceive-factory-default - on
//     an IC-7610 taken to MENU > SET > Others > Reset > All Reset, send
//     FE FE 98 E0 1A 05 01 12 FD and record the value in the answer. Scope:
//     that one radio's post-reset value of that one set item.
//
//   - ic7610-default-baud (R11) - that the factory-default CI-V baud rate is
//     19200. ASSUMED, and the CHOICE OF 19200 WITHIN THE PRINTED SET IS
//     ARBITRARY AND IS RECORDED AS SUCH: the document defers the value to the
//     instruction manual, so no reading of it favours one of the six rates.
//     Spec D3.2's "A4-evidenced" cannot be met on this model, which the
//     matrix flagged to the orchestrator and the orchestrator ruled stays
//     ASSUMED. WHAT DEPENDS ON IT: what internal/wiring opens the port at,
//     since the driver cannot sweep - wiring opens from
//     Capabilities().DefaultBaud and Wave 3 may never touch internal/wiring.
//     WHY THAT IS SAFE: a wrong guess produces a CLEAN TIMEOUT AT Open, NEVER
//     A WRONG BYTE, because the probe requires an address-matched 19 00 reply
//     and silence is silence.
//     STAGE R LIFTS IT WITH: capture ic7610-default-baud - on an IC-7610
//     taken to MENU > SET > Others > Reset > All Reset, read the
//     Connectors > CI-V > CI-V Baud Rate item from the front panel and
//     photograph it. Scope: that one radio's post-reset value of that one
//     item.
//
//   - ic7610-civ-rate-list (R12) - that the six named rates
//     {4800, 9600, 19200, 38400, 57600, 115200} are the COMPLETE CI-V rate
//     list. The six are MANUAL-EVIDENCED as named for the CI-V link; their
//     COMPLETENESS is ASSUMED. WHAT DEPENDS ON IT: the Bauds list a user may
//     choose from, and therefore whether a radio moved off its default is
//     reachable at all.
//     STAGE R LIFTS IT WITH: capture ic7610-civ-rate-list - step the
//     front-panel CI-V Baud Rate item through every position in turn and
//     record the complete printed list. Scope: the rates that one radio
//     offers.
//
//   - ic7610-usb-echo-default (R13) - the factory-default CI-V USB Echo Back
//     value. That the SETTING EXISTS is manual-evidenced (PDF p.7's row
//     1A 05 01 16 00 or 01, Connectors > CI-V > CI-V USB Echo Back
//     (00=OFF, 01=ON)); NO DEFAULT IS MARKED, so the default is ASSUMED. WHAT
//     DEPENDS ON IT: nothing in this package, and by design little in the
//     driver - echo suppression MUST match the recorded bytes (byte identity)
//     and MUST NOT suppress by position or by count, so a session with echo
//     on and a session with echo off both work without knowing which they
//     are.
//     STAGE R LIFTS IT WITH: capture ic7610-usb-echo-default - on an IC-7610
//     taken to MENU > SET > Others > Reset > All Reset, send
//     FE FE 98 E0 1A 05 01 16 FD and record the value in the answer. Scope:
//     that one radio's post-reset value of that one item.
//
//   - ic7610-1a00-set-ack (R14) - that a 1A 00 set is acknowledged with FB
//     and rejected with FA. ASSUMED for 1A 00 specifically: the OK/NG codes
//     themselves are manual-evidenced (PDF p.3's two strips), and the one
//     place the document ties an acknowledgement to a command is command 29's
//     own note, which this package does not read across. WHAT DEPENDS ON IT:
//     the driver's acknowledged write - civ.CIVWriteWithAckSpec matches the
//     acknowledgement and Engine.Do turns an FA into transport.ErrRejected
//     with no frame (T4).
//     STAGE W LIFTS IT WITH: capture ic7610-set-ack-ch01 - send the 34-byte
//     set-record-name-with-space golden frame to an IC-7610 and record the
//     answer frame verbatim. Scope: what THAT radio answers to THAT one set
//     frame.
//
//   - ic7610-full-record-mandatory (R15) - that the full record is mandatory
//     on write. ASSUMED: the document prints one full-record form and one
//     three-index clear form, and never says whether a short record is
//     accepted, padded or rejected. WHAT DEPENDS ON IT: that this driver must
//     read a slot before writing it (every field it does not have must come
//     from the radio), which is also what makes E6's comparison read free.
//     STAGE W LIFTS IT WITH: capture ic7610-short-record-ch03 - send a 1A 00
//     set to memory channel 03 carrying only the first 17 record bytes, and
//     record the answer frame. Scope: whether THAT one truncated record is
//     accepted or rejected by THAT radio.
//
//   - ic7610-mode-code-radix - whether the printed 12 and 13 in the Receiving
//     mode column are the WIRE BYTES 0x12 and 0x13 or the DECIMAL values 12
//     and 13 (0x0C and 0x0D). The other eight printed codes are identical
//     under either reading; only PSK and PSK-R differ. The document uses both
//     conventions in different places - it prints every command, sub-command
//     and address as hexadecimal byte pairs, and it also prints one row where
//     a numeral pair is packed BCD of a DECIMAL number naming a hex value
//     (PDF p.7, 1A 05 01 13 ... (00 00=00h ~ 02 23=DFh) (in Hexadecimal),
//     where 02 23 is BCD 223 = 0xDF) - and neither settles this column. WHAT
//     DEPENDS ON IT: whether a PSK or PSK-R channel decodes at all, and what
//     byte a PSK write puts on the wire.
//     RULED 24/08/2026 by the orchestrator: HEXADECIMAL. PSK is 0x12 and
//     PSK-R is 0x13, as modeEnum carries them. REASON: everywhere else in
//     this document family a printed digit-pair lands on the wire as its
//     literal nibbles (packed BCD - frequencies, tones, channel numbers), and
//     the tier's adjudicated sibling register entries read the mode byte the
//     same way (ic9700-mode-codes-are-hexadecimal, ic705-dv-mode-code); a
//     decimal reading would make this radio's mode byte the one field in the
//     family whose printed digits are NOT its nibbles, with no printed
//     warrant for the exception. A wrong ruling mis-decodes two mode values
//     on read and refuses or mis-encodes on write - both liftable, neither
//     corrupting. THE GRADE STAYS ASSUMED.
//     STAGE R LIFTS IT WITH: capture ic7610-mode-code-radix - put the Main
//     band in PSK from the front panel, send FE FE 98 E0 04 FD, and record
//     the mode byte returned. Scope: that one radio, that one mode. It is the
//     same radio session R19's sweep needs and may be taken alongside it.
//
//   - ic7610-filter-value-set - that byte (10) carries only 01, 02 or 03 in a
//     1A 00 record. ASSUMED: PDF p.11's Filter setting column prints three
//     values and no default, and its fourth and fifth rows are printed "-"
//     (matrix Errata (rev 1) erratum 6). 0x00 IS DELIBERATELY NOT A MEMBER of
//     filterEnum - inventing a fourth value would be a radio claim. WHAT
//     DEPENDS ON IT: whether a real record decodes; a (10) of 00 fails with a
//     parse error naming the offset.
//     STAGE R LIFTS IT WITH ITS OWN CAPTURE, NOT WITH R5 - R5 reads one
//     channel whose filter is FIL2 and cannot establish that no other value
//     exists: capture ic7610-filter-value-sweep - set memory channel 06 to
//     each filter position the front panel offers in turn, reading 1A 00
//     after each, and record every distinct value byte (10) takes; then send
//     a 1A 00 set with (10) = 00 and record whether the radio answers FB or
//     FA. Scope: which filter values THAT radio reports and accepts on THAT
//     channel.
//
//   - ic7610-default-tone-undocumented - that this document prints NO DEFAULT
//     TONE VALUE anywhere. All 17 pages were swept for the matrix and no
//     "Default:" line appears against the tone frequency; the only tone
//     material is the 1B 00 / 1B 01 digit strip on PDF p.14. WHAT DEPENDS ON
//     IT: whether an empty-slot CREATE can supply a tone at all. Tier ruling
//     T1(5) says a create whose tone fields are not Known writes the manual's
//     DOCUMENTED default if there is one and otherwise REFUSES NAMING THE
//     FIELD, so THIS MODEL TAKES THE REFUSE ARM. The usability cost is real
//     and is accepted: no channel can be CREATED on this radio without
//     explicit tone values.
//     STAGE R LIFTS IT WITH: capture ic7610-default-tone - on an IC-7610
//     taken to MENU > SET > Others > Reset > All Reset, send
//     FE FE 98 E0 1B 00 FD and FE FE 98 E0 1B 01 FD and record the values in
//     the answers. Scope: that one radio's post-reset value of those two
//     items.
//
//   - ic7610-e6-unmapped-regions - that byte (3)'s low nibble and byte (11)'s
//     high nibble are the ONLY regions of this record with no faithful
//     neutral home, and that a radio's own value for either is preserved by
//     REFUSING the write rather than by rewriting it. WHAT DEPENDS ON IT:
//     which channels this programme can write at all - see the E6 costs
//     above.
//     LIFTED BY TWO CAPTURES, NEITHER OF WHICH SPEAKS FOR THE OTHER: for the
//     SELECT half, R20's capture ic7610-select-marker-ch05 (reproduced in
//     core/driver/ic7610/doc.go under ic7610-select-marker-semantics); for
//     the data-mode half, capture ic7610-data-mode-nibble - set memory
//     channel 07's data mode to DATA 2 from the front panel, read 1A 00, and
//     record byte (11). Scope: what the value 2 looks like in THAT one
//     record on THAT radio.
//
// # ic7610 DRIVER register - LISTED HERE, REPRODUCED IN FULL THERE
//
// These six entries live in core/driver/ic7610/doc.go. They are named here so
// that a reader of this register knows the set is not complete without them,
// and their capture blocks are deliberately NOT duplicated:
//
//   - ic7610-control-lines-inert (R16) - that RTS and DTR deasserted neither
//     key the radio nor block CI-V;
//   - ic7610-storable-frequency-ceiling (R17a);
//   - ic7610-storable-frequency-floor (R17b);
//   - ic7610-scan-edge-record-fields (R18) - that scan-edge records honour
//     the record's non-frequency fields;
//   - ic7610-mode-code-completeness (R19) - that the ten printed mode codes
//     are the complete set, i.e. that 06 and 09-11 name nothing;
//   - ic7610-select-marker-semantics (R20) - byte (3)'s select-marker
//     semantics.
package ic7610

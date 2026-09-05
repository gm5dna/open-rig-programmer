// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft891 is the Yaesu FT-891 driver: the project's FOURTH
// implementation of core/driver's Driver/Session seam over the Yaesu
// NEWCAT grammar, and the second written for a radio this project has
// never connected to anything.
//
// It is structured on core/driver/ftdx10 — the M9c-6 shape — and it
// deliberately does NOT import it, nor core/driver/ft710 or
// core/driver/ftdx101. Nothing in this package may: a sibling driver's
// shape is a template, but its VALUES are that radio's readings and
// findings, and importing them is how one radio's evidence silently
// becomes another's claim. Where a decision here looks like a sibling's,
// the comment at the decision says whether the agreement is a manual fact
// of THIS radio, a structural requirement, or an assumption (in which case
// it is in the register below).
//
// # Provenance
//
// Everything protocol-shaped here comes from the Yaesu FT-891 CAT
// Operation Reference Book, revision 1909-C, through core/cat/ft891's
// dialect — the ONE core/cat instance this package names (catDialect,
// caps.go). Citations of the form "layout N" are line numbers in that
// manual's layout-preserved extraction (both PDF and extraction are
// gitignored, so these are citations, not links); the dialect's own doc.go
// carries the digests, the chart-defect record, the MT contradiction
// record, its EIGHT-entry ASSUMED register and its reused-command
// verification verdict. This package CITES the dialect's entries BY NAME
// where it depends on them and duplicates none of them.
//
// Every capability VALUE in caps.go comes from
// docs/superpowers/ft891-capability-matrix.md (revision 1 + errata
// M-E1..M-E6), with the matrix section cited at the field it sets. The
// matrix is a gitignored record, so those citations are citations too; the
// values they justify are pinned by caps_test.go.
//
// NO FT-891 HAS EVER BEEN ASKED ANYTHING by this project, and none is
// available to it. There is no docs/hardware-notes.md section for this
// model, no observation CSV, no corrections file, no captured frame, and
// no Stage R or Stage W session. Every statement here is a reading of a
// manual or a written-down assumption, and the register below is where the
// difference is recorded rather than glossed.
//
// # The write guard
//
// writeTrialsComplete is FALSE (caps.go) and pinned false in both halves
// by its own test. A RealHardware session therefore gets
// CapabilitiesUnverified — every candidate field's Write spec.Unverified,
// nothing writable anywhere — so, UNLESS THE USER HAS CONSENTED (next
// section), codeplug.Diff blocks every change, the clone service refuses
// to execute one, and Session.WriteChannel's own capability re-check
// refuses before any frame is built. An unrecognised Profile value fails
// the same way (see ft891Driver.Capabilities): the failure direction for a
// forged or corrupted Profile is always "nothing writable".
//
// # The one route past the guard: the user's recorded consent
//
// A RealHardware session opened with WithConsentedUnverifiedWrites — the
// option internal/wiring spends a user's stored grant through — carries
// spec.ConsentedUnverified where the profile said spec.Unverified, and
// FieldSupport.CanWrite is true for that state. Such a session CAN write
// this radio, and the three gates above let it, by design: the user has
// been shown the warning and accepted the risk for this model.
//
// What that does NOT change: the EVIDENCE. writeTrialsComplete stays
// false, this package's static Capabilities() stays all-Unverified (which
// is what internal/wiring.NeedsUnverifiedConsent reads to decide the radio
// is consent-eligible at all), no hardware note is written, and every
// unconsented session behaves exactly as the paragraph above describes.
//
// The transform is applied ONCE, at session-capability assembly
// (ft891Driver.sessionCapabilities), and only for a RECOGNISED Profile, so
// the "forged or corrupted Profile" direction survives consent untouched.
//
// # The MT Read contradiction, and why this driver does not settle it
//
// THIS RADIO'S MANUAL CONTRADICTS ITSELF ABOUT WHETHER MT CAN BE READ.
// The Control Command List gives MT "MEMORY WRITE & TAG" the columns Set
// O, Read X, Answer X (layout 166) — Set only — while MT's own detail
// block prints a Read chart ("M T P0 P0 P0 ;", layout 1016) and a full
// 41-position Answer chart (1018-1027), in the same block on the same
// printed page. Both cannot be true, and every registered sibling's list
// says O O O, so this radio is the only one of the four whose two records
// disagree (matrix §3.12; core/cat/ft891/doc.go, "The MT contradiction").
//
// The geometry witness found direct evidence that these charts DO contain
// printing errors — the VX, VM and ZI Set charts on folio 18 each print a
// terminator in two positions, verified at 600 dpi
// (core/cat/ft891/testdata/provenance.md §Disagreements, item 8) — which
// does not settle which record is wrong, but does settle that "it is
// printed, therefore it is true" is unavailable here for either of them.
//
// THE DIALECT DECLINED TO SETTLE IT and said the question was the
// driver's. THIS DRIVER DECLINES TOO, and read.go is built around not
// settling it: memory and PMS are read by MT with a per-slot MR
// CROSS-CHECK, and an MT "?;" on a slot MR shows to be OCCUPIED becomes
// the typed, whole-session ErrMTReadRejectedForOccupiedSlot, which NAMES
// the contradiction and does not diagnose it. "?;" carries no reason code,
// so three readings stay consistent with the manual and this project
// cannot tell them apart: the command list is right and MT has no Read
// here; MT has a Read but refused this particular slot; or something
// transient. The refusal fails the read WHOLE rather than per-slot,
// because a partial read that silently dropped occupied channels would be
// a codeplug the user could not tell from a complete one.
//
// The premise is registered as an assumption — the entry MT READ IS
// SUPPORTED FOR MEMORY AND PMS — and its refutation is exactly the first
// of those three readings. A degraded MR-only mode for memory and PMS is
// an explicit NON-GOAL of this milestone: the refusal is the honest
// placeholder.
//
// # Discovery walks the WHOLE declared range, by MR
//
// Open probes EVERY slot core/cat/ft891's SlotSpace declares — 501 through
// 510, ascending — and then EMG: at most ELEVEN frames per session
// (matrix §3.4). There is no contiguity assumption, no sentinel, no early
// termination and no cap. The FT-710's discovery rules (stop at the first
// rejection, cap at 15, probe one sentinel past the cap) are FT-710
// HARDWARE facts about a radio whose factory 5xx channels are believed
// contiguous and non-erasable, and none of that is known here; a sparse
// bank — a populated 503 after an empty 502 — must not be silently
// truncated.
//
// THE PROBES ARE MR READS, not MT reads, which is where this driver
// departs from both combined-form siblings. MT's own slot legend on this
// radio prints memory and PMS ONLY (layout 998-999) where MR's prints all
// four classes (960-964), so an "MT501;" is a frame this manual does not
// describe — and under the dialect's MTPolicy.ReadSlots = MTReadsMemoryPMS
// the codec and the outbound gate both refuse to build one. A negative
// pin on the transcript holds it (TestOpen_NeverBuildsAnMTReadOfADiscoveredSlot).
//
// The cost is ELEVEN exchanges per Open, a fraction of the FTdx10's ~100,
// because this radio's manual prints the bank's actual bounds where the
// FTdx10's prints only "5xx". NOBODY narrows the walk — settle override,
// early termination, range shrink — without an orchestrator-adjudicated
// change; TestOpen_DiscoveryProbesEveryDeclaredSlot pins the full ordered
// transcript so that a regression is a test failure rather than a silently
// shorter walk.
//
// # The read choreography, and why this Session HAS an operation mutex
//
// The truth table read.go implements (matrix §3.5) is:
//
//   - MEM/PMS, one combined 41-byte MT read: a well-formed answer is the
//     channel, field block and tag and display flag together, ATOMICALLY.
//   - MEM/PMS, MT answers "?;": ONE MR read of the same slot. MR also
//     "?;" — the slot is EMPTY. MR returns a record — the slot is
//     OCCUPIED and MT refused it, so ErrMTReadRejectedForOccupiedSlot and
//     the session read fails WHOLE.
//   - MEM/PMS, MT TIMES OUT: MTReadTimeoutError, the session read fails
//     whole, and NO RETRY (the MT read's spec carries RetryReads 0) and no
//     MR — a timeout is one MT frame and nothing else.
//   - 60M/EMG: ONE MR read per slot, and never an MT read. Tag and
//     TagDisplay come back Unavailable, because MR's 28-position answer
//     carries neither (matrix §2.5).
//
// A memory or PMS read is therefore potentially TWO exchanges, so the
// one-exchange-per-operation property the FTdx10's Session relies on DOES
// NOT HOLD HERE. transport.Engine serialises each individual exchange, not
// a pair, so this Session carries an operation mutex (opMu) held across
// the whole cross-check: otherwise a concurrent operation could land
// between the MT "?;" and the MR that interprets it, and the cross-check
// would be reasoning about a different radio state. The FTdx10's "no
// opMu" comment is that driver's consequence of its own choreography and
// is not the shape to copy.
//
// The mutex guards a SINGLE DRIVER OPERATION (spec erratum S-E4, matrix
// M-E2): the MT+MR cross-check inside ReadChannel, and WriteChannel's one
// exchange (write.go). The write-then-verify PAIR is
// core/clone's, as the driver interface assigns it, and is deliberately
// not held under one driver lock.
//
// # No per-class kind narrowing
//
// The FT-710's driver keeps its own P7 kind vocabulary per bank
// (acceptedKinds) because its leniencies are ITS live observations. This
// driver has none, so it adds no check of its own: core/cat's parsers
// already narrow P7 — the combined answer's to the documented read pair
// {'0' VFO, '1' Memory}, the 28-byte block's to the documented '0'-'5' —
// and an out-of-vocabulary byte surfaces as a *cat.ParseError, wrapped
// here with the slot being read.
//
// THIS MANUAL INVITES A WIDENING AND IT IS REFUSED. IF's P7 prints SIX
// values (layout 789) and OI's prints SEVEN (1134-1135), but those are
// IF's and OI's own parameters on their own frames, not the memory
// record's: widening MT's accepted P7 on the strength of a different
// command's legend would invent a distinction this manual does not draw
// for MT. The tolerance the combined parser does apply is the DIALECT
// register's entry "THE COMBINED ANSWER'S P7 READ DOMAIN", cited here and
// deliberately NOT re-registered as a driver entry: it is already a
// dialect assumption by that name, and neither register may absorb the
// other.
//
// # RegionReporter is NOT implemented
//
// core/driver.RegionReporter is optional (core/driver/optional.go) and
// this driver does not satisfy it. The FT-710's Region() derives a
// regulatory-region string from its discovered 5xx/EMG inventory using
// fingerprints that are that radio's own, partly hardware-confirmed and
// partly still assumed. There is no honest FT-891 vocabulary to report: no
// FT-891 inventory has ever been observed, and this radio's own legend
// says only "U.S. and U.K. version only" (layout 962) — which is a
// statement about market, not a mapping from a channel count to a region.
// Callers already tolerate absence (the capability is reached by a
// two-result type assertion), so codeplug.RadioInfo.Region simply stays
// unset for an FT-891.
//
// # CAT reaches this radio over a bridge with TWO UART endpoints
//
// The FT-891's CAT path is a "built-in USB to Dual UART Bridge" (layout
// 24-27) and the word "Dual" is the ONLY mention of the second endpoint in
// the whole document: unlike the FTdx101's manual, this one never says
// which of the two enumerated COM ports carries CAT (matrix §3.13). There
// is NO ACTION for this package — this project opens whatever port it is
// given, and this manual gives no way to detect which one — but it changes
// what "the radio did not answer" means for every capture below, so ANY
// FT-891 capture must record which enumerated serial device it was taken
// on, and a capture that cannot say is not evidence about framing or
// control lines. The registration task carries the same caveat into
// internal/radiotext's per-model prose.
//
// # The ASSUMED register
//
// SIXTEEN ENTRIES, covering the behaviours this driver encodes that are
// NOT FT-891-manual facts. Each is listed here once, marked ASSUMED at the
// point of use, and paired with the ONE Stage R or Stage W capture that
// lifts it. The captures are individual on purpose: one FT-891 session
// does not retire this register wholesale, it retires the assumptions its
// own frames actually speak to, and an entry whose capture was not taken
// stays here afterwards.
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION. The numbering is for
// readability; every citation of this register — here, in this package's
// other files, and in its tests — names the entry's subject. A bare "entry
// 6" is correct only until an entry is inserted, and it then silently
// points at the wrong assumption rather than failing. The FT-891 DIALECT
// register carries the identical rule in terms.
//
// This is the DRIVER's register. The DIALECT's EIGHT entries
// (core/cat/ft891/doc.go) are separate, and SEVEN of them are CITED in
// this package at the sites that depend on them:
//
//   - MTPolicy.TagFill = ' ' — caps.go's TagLen (a width is evidenced, a
//     fill is not) and read.go, where the answer's tag field is trimmed.
//   - THE COMBINED MT ANSWER'S EXACT LENGTH, 41 — read.go's mtSpec, which
//     derives the length from the dialect precisely because the
//     exactness is that entry's assumption and its recorded contingency
//     is a 30..41 WINDOW.
//   - SlotSpace.NoneWire = "000" — read.go's BuildMTRead/BuildMRRead
//     refusal sites, where the answer-only none form is grammatical per
//     ParseSlot and never a legal read target.
//   - THE cat.ModeUnset MEMBER OF THE MODE TABLE — caps.go's modeNames,
//     which excludes it.
//   - ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990 —
//     caps.go's ClarStepHz/ClarMaxHz.
//   - THE CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D —
//     read.go, where a negative offset's sign is decoded.
//   - THE COMBINED ANSWER'S P7 READ DOMAIN — read.go, the tolerant parse
//     the read path relies on. It is NOT a driver entry, and the "No
//     per-class kind narrowing" section above says why.
//
// The eighth, THE MC ANSWER DOMAIN BEYOND MEMORY AND PMS, is deliberately
// NOT reached: this driver sends no MC frame at all. It is named here so
// the count of eight is visibly complete, and so that a later milestone
// adding an MC path knows the assumption is already registered and must be
// cited, not re-invented.
//
// Correcting a dialect entry is a change in core/cat/ft891; correcting one
// of these is a change here. NEITHER REGISTER MAY ABSORB THE OTHER.
//
//  1. FRAMING: 8 DATA BITS, NO PARITY, TWO STOP BITS (core/transport's
//     DefaultStopBits, which every session this driver opens inherits —
//     there is no framing field in spec.Capabilities and, per the
//     E2-recorded decision, none is added without hardware evidence).
//     THIS MANUAL IS SILENT ON FRAMING: it states no stop-bit count, no
//     parity and no data-bit width anywhere, its connection section
//     (layout 20-57) describes a USB cable and a built-in bridge and no
//     line discipline, and its serial menu rows are 0506 CAT RATE, 0507
//     CAT TOT and 0508 CAT RTS only (553-555). 8-N-2 is the FT-710's own
//     documented framing, inherited here as a family working value.
//     STAGE R LIFTS IT WITH: an ID exchange at the answering baud with the
//     port opened 8-N-2 — a clean "ID0650;" confirms the framing is at
//     least compatible. THE LIFT MUST DISTINGUISH FRAMING FROM THE CONTROL
//     LINES (the CONTROL-LINE POLICY entry): silence at a known-correct
//     baud is NOT evidence about stop bits until 0508 CAT RTS has been
//     toggled at the radio and the exchange retried, because a handshake
//     refusal and a framing mismatch present identically as nothing coming
//     back. Try 8-N-1 only after that, and record which of the two changed
//     the outcome — and record the port (see the dual-UART section).
//
//  2. CONTROL-LINE POLICY: that driving RTS and DTR low unconditionally is
//     safe on THIS radio. core/transport.OpenSerial does exactly that
//     immediately after opening any port (safety obligation 4), for every
//     model, with no per-radio policy — and this radio has an opinion
//     about the line that the transport does not consult: menu 0508 CAT
//     RTS, "0: DISABLE 1: ENABLE" (layout 555). What its factory setting
//     is, and whether a low RTS with it enabled stalls the CAT link, is
//     unknown. The FT-891 additionally routes PTT and keying through
//     RTS/DTR in six menus — 0607 AM PTT SELECT (579), 0712 PC KEYING
//     (591), 0810 DATA PTT SELECT (602), 0903 PKT PTT SELECT (607), 1005
//     RTTY SHIFT PORT (615), 1108 SSB PTT SELECT (629) — so on a radio
//     configured that way a driven control line is not merely a handshake
//     question. DRIVING BOTH LOW IS THE DIRECTION THAT CANNOT KEY A
//     TRANSMITTER, which is why the inherited policy is the conservative
//     one; that reasoning is a CHOICE and the safety claim behind it is
//     the assumption.
//     STAGE R LIFTS IT WITH: one ID exchange in each 0508 CAT RTS state,
//     everything else held constant. If it answers in both, the policy is
//     safe here and this entry closes; if it answers in only one, the
//     transport needs a per-radio control-line policy and this becomes a
//     spec'd capability rather than an assumption. TAKE THIS CAPTURE
//     BEFORE CONCLUDING ANYTHING ABOUT THE FRAMING ENTRY.
//
//  3. DefaultBaud 38400 (caps.go). The RATE LIST is manual-evidenced —
//     menu 0506 CAT RATE, "0: 4800 bps 1: 9600 bps 2: 19200 bps 3: 38400
//     bps" (layout 553), four rates and no 115200 — but THE FACTORY
//     DEFAULT IS NOT IN THIS MANUAL AT ALL. Its menu chart has no
//     factory-default column: the headers are "P1 | Function | P2 |
//     Digits" (524) and the trailing 1 on line 553 is the DIGITS field,
//     exactly as this project's generated inventory reads it. The FTdx10
//     milestone once misread that digit as a default index and concluded
//     9600; the misreading is recorded here so it cannot recur silently on
//     this radio. The legend's first option being 4800 is not evidence
//     either — that is the option list's ordering.
//     WHY 38400 (matrix erratum M-E4): it is the value the three
//     registered Yaesu siblings carry, two of them ASSUMED on identical
//     grounds, and NOTHING IN THIS MANUAL BEARS ON IT. The FT-710's
//     "same-generation default" justification is expressly withdrawn — the
//     FT-891 is a 2016 radio. It matters because internal/wiring's
//     OpenRealSessionFor opens a real radio at exactly this driver's
//     DefaultBaud and NO BAUD OVERRIDE EXISTS IN THE CLI OR THE GUI, so a
//     wrong value here leaves a real FT-891 reachable only by changing
//     menu 0506 on the radio's own front panel.
//     STAGE R LIFTS IT WITH: the baud a FACTORY-CONFIGURED FT-891's ID
//     exchange actually answers at — try 38400 first, then the other three
//     in turn. The answering rate is the fact; a radio whose CAT RATE has
//     been changed by its owner cannot settle it, so the capture must
//     record whether menu 0506 was known-untouched.
//
//  4. MinFreqHz 30_000 / MaxFreqHz 56_000_000 — THE FA/FB RANGE READ AS
//     THE MEMORY-STORABLE RANGE (caps.go). The NUMBERS are
//     manual-evidenced, and this radio is better evidenced here than the
//     FTdx10, whose manual carries no range statement at all: FA's P1
//     legend "000030000 - 056000000 (Hz)" (layout 702) and FB's identical
//     one (718) are the only frequency range this manual prints anywhere.
//     WHAT IS ASSUMED is the step from that to the memory-storable domain:
//     FA and FB are the VFOs, and the memory blocks' own legends say only
//     "Frequency (Hz)" (MT 1001, MW 1038) or "VFO-A Frequency (Hz)" (MR
//     965, IF 779) over a nine-digit field, which bounds the ENCODING and
//     says nothing about what a memory channel will store. That the two
//     domains coincide is the assumption. MaxFreqHz is additionally the
//     ledgered dangerous-zero field — a zero reads as "no ceiling" to
//     every validator — so it MUST be populated; this entry is the honesty
//     about where the number came from, not a licence to leave it empty.
//     STAGE R LIFTS IT WITH: MT Sets at the claimed floor and ceiling and
//     just outside them, to a sacrificial channel, recording which are
//     accepted. The radio range-checks frequency on write (a real FT-710
//     demonstrably does), so acceptance and rejection are both
//     informative. The FT-891 operating manual's specifications page would
//     settle the RADIO's range but not the MEMORY's.
//
//  5. RequiredSlots {"001"} (caps.go). That memory channel 001 must never
//     be empty. THIS MANUAL STATES NO SUCH RULE ANYWHERE. The FT-710's
//     M-01 is individually required because that radio keeps it populated
//     — an FT-710 hardware fact, not borrowed. Claiming it makes codeplug
//     validation refuse a candidate whose 001 is blank, which is the
//     conservative direction (refuse rather than write a state the radio
//     may not tolerate), but it IS a claim.
//     STAGE R LIFTS IT WITH: observation of channel 001 on a real FT-891 —
//     whether the radio ships with it populated, and whether the front
//     panel will erase it at all. A radio that erases 001 happily drops
//     this from RequiredSlots.
//
//  6. TONE AND SCAN-SKIP UNREACHABILITY (caps.go's zero FieldSupport for
//     spec.FieldCTCSSTone and spec.FieldScanSkip; read.go's Unknown
//     states). Neither field is claimed readable or writable in either
//     direction, on any bank or profile. WHAT IS STRUCTURAL AND
//     MANUAL-EVIDENCED: the combined MT record accounts for every one of
//     its 41 positions (slot, frequency, clarifier sign and magnitude, the
//     two clarifier flags, mode, kind, CTCSS state, the fixed "00" P9,
//     shift, the P11 TAG flag, the 12-byte tag, terminator — legends at
//     layout 998-1017, counted twice off 300 dpi renders), P9 is
//     documented "00: (Fixed)" (1013), and no command this driver sends
//     carries a tone NUMBER or a scan-skip flag for a memory channel at
//     all. WHAT IS ASSUMED is the step from that to "these fields are
//     unreachable on this radio": nothing verifies that the CTCSS-state
//     byte means anything live here, and whether some OTHER command could
//     reach a channel's stored tone number is not established either way.
//     CN reaches the radio's CURRENT tone number (365-375), which is live
//     state and not a channel's stored value. The FT-710's answer — that
//     nothing can reach a channel's tone, and that its P9 reads fixed "00"
//     with a tone demonstrably set and active — is that radio's hardware
//     finding and is not borrowed.
//     STAGE R LIFTS IT WITH: one channel set to a known CTCSS tone from
//     the front panel, then read over CAT. If any byte of the answer
//     tracks the tone number, this entry is refuted and the capability
//     opens; if P9 reads "00" as documented, the entry closes as a
//     confirmed protocol limit rather than an assumption. The scan-skip
//     half needs the same experiment with the front-panel skip flag.
//
//  7. MT READ IS SUPPORTED FOR MEMORY AND PMS (read.go's whole
//     MEM/PMS path). The premise the read choreography rests on, and the
//     one the manual contradicts itself about — see the MT Read
//     contradiction section above. What is registered is the PREMISE; the
//     CAUSE of a refusal is deliberately not registered, because nothing
//     is assumed about it.
//     STAGE R LIFTS IT WITH: ONE MT read of a KNOWN-POPULATED memory
//     channel on a real FT-891. A well-formed 41-byte answer lifts the
//     entry and settles the contradiction in the detail block's favour; a
//     "?;" — with the same slot's MR read returning a record in the same
//     session — confirms the command list and turns
//     ErrMTReadRejectedForOccupiedSlot from a placeholder into a finding.
//     THIS IS THE SINGLE MOST VALUABLE CAPTURE ON THE WHOLE FT-891 LIST,
//     and the capture protocol should take it FIRST.
//
//  8. "?;" ON AN MR READ OF A MEMORY OR PMS SLOT MEANS THE SLOT IS EMPTY
//     (read.go's cross-check). In the cross-check, an MR "?;" following an
//     MT "?;" is read as "this slot is empty" and mapped to an EMPTY
//     codeplug.Channel — Data nil, the slot carried through — rather than
//     an error. "?;" is the protocol's SINGLE unattributed NAK
//     (cat.ErrRejected's own doc comment: unknown commands, bad
//     parameters, wrong radio state, an empty memory slot and anything
//     else that goes wrong all produce it), so reading "empty" out of it
//     is an interpretation. The FT-710's MR-on-empty behaviour is that
//     radio's hardware finding; this radio's is not. Note that this driver
//     never sends an MR read of a memory or PMS slot EXCEPT as the
//     cross-check's second frame, so the interpretation has exactly one
//     site.
//     STAGE R LIFTS IT WITH: one MR read of a memory channel known-empty
//     from the front panel, and one of a known-populated channel, in the
//     same session.
//
//  9. THE ACKNOWLEDGEMENT CONVENTIONS: that an accepted Set draws no reply
//     at all and that a rejected one draws exactly one "?;". AN INHERITED
//     FRAMING CONVENTION, NOT A READING OF THIS MANUAL — which states
//     neither half, and whose layout extraction contains no '?' character
//     anywhere. MT's availability row grounds none of it: that row marks
//     which FORMS the command has, and its Ans. column marks the existence
//     of the ANSWER FORM (which is why read-only MR carries Ans. O too,
//     layout 164), so reading a reply convention off it is a non sequitur
//     — the misreading that was M9d-2's erratum 1 and is not repeated
//     here. This entry is carried as the SECOND HALF of the entry A SINGLE
//     COMBINED MT SET SUFFICES..., because ONE capture lifts both and one
//     line of code (the write's transport.CommandSpec) asserts both; it is
//     stated separately because the matrix does, and because the
//     convention also governs the AI0 init frame Open sends, which that
//     entry does not cover.
//     STAGE W LIFTS IT WITH: the first write trial, WITH THE PORT WATCHED
//     BETWEEN THE SET AND THE READ-BACK. Whatever the port carries in the
//     gap — nothing, a "?;", or an acknowledgement this project has no
//     handling for — is the lift, and it must be recorded even when it is
//     nothing, because "no bytes observed" is the whole content of the
//     claim.
//
//  10. "?;" ON A 5xx/EMG DISCOVERY PROBE MEANS ABSENT FROM THIS RADIO
//     (ft891.go's discoverInventory/probeSlot). Discovery treats a
//     rejection as "this radio does not have that channel" and a
//     well-formed MR answer as "it does". The FT-710's equivalent
//     interpretation is hardware-confirmed for THAT radio (live probes of
//     a never-populated and an out-of-inventory slot both answered "?;"),
//     and this radio's probe is an MR read where the FT-710's and
//     FTdx10's are MT reads — so even the sibling's shape of the
//     assumption is not the same frame.
//     STAGE R LIFTS IT WITH: an MR enumeration of 501..510 and EMG on a
//     U.K.- or U.S.-market FT-891 with a POPULATED 5 MHz bank,
//     cross-checked against the channels the front panel shows. Which
//     wire numbers answer and which reject is then a fact. THIS CAPTURE
//     ALSO SPEAKS TO the entry THE 5 MHz BANK'S PRESENCE ON A
//     U.K.-MARKET UNIT — two entries, one possible capture; record both
//     explicitly, and record the unit's market.
//
//  11. THE MODE NIBBLE'S TOP END: that 'D' (AM-N) is the highest nibble
//     this radio ever puts in P6, that 'A' is genuinely a hole, and that
//     'E' and 'F' never appear. All three mode legends print "... 9:
//     RTTY-USB A: - B: FM-N ..." and stop at D (layout 972-974,
//     1007-1010, 1043-1046), and the dash is the chart's way of printing
//     "nothing here" — but a legend is a statement about what the chart
//     draws, and the parse refuses any byte outside the transcribed
//     table.
//     STAGE R LIFTS IT WITH: one read of every occupied channel on a
//     radio whose owner has used its full mode range, plus an MD read in
//     each selectable mode. Any nibble outside 1-9/B-D refutes the entry
//     and the dialect's table must widen.
//
//  12. THE 5 MHz BANK'S PRESENCE ON A U.K.-MARKET UNIT: that a U.K.
//     FT-891 has 501..510 at all, and that all ten answer. The NUMBERING
//     is transcribed rather than assumed on this radio — MR's legend
//     prints "501 - 510 (5 MHz, U.S. and U.K. version only)" (layout 962)
//     where the FT-710's and FTdx10's print only "5xx", which is why the
//     dialect carries the bounds with a citation and its register
//     deliberately has no entry for them. What is assumed is the REGION
//     CONDITION's consequence: "U.S. and U.K. version only" is a fact
//     about which unit is in front of you, which is why the banks are
//     discovered rather than declared, and nothing here says all ten
//     channels of a U.K. unit will answer.
//     STAGE R LIFTS IT WITH: the SAME capture as the "?;" ON A 5xx/EMG
//     DISCOVERY PROBE entry, on a unit whose market is recorded. Two
//     entries, one possible capture — record both.
//
//  13. READ-BACK OF THE TAG DISPLAY FLAG: that the radio REPORTS byte 28
//     in its MT answer rather than, say, always answering '0'. That the
//     byte is a LIVE FLAG is MANUAL-EVIDENCED and is this radio's own
//     inversion of both combined-form siblings (MT's P11 legend, `0: TAG
//     "OFF" 1: TAG "ON"`, layout 1016); what is not established is the
//     READ direction. core/clone's writableFieldsMismatch compares
//     TagDisplay only when BOTH sides are Known, so a radio that answered
//     a constant would surface as a verify mismatch rather than silent
//     corruption; that is the failure mode this entry protects.
//     STAGE R LIFTS IT WITH: one MT read of a channel whose TAG display
//     has been turned ON from the front panel, and one of a channel whose
//     display is OFF. Byte 28 differing between the two IS the read-back.
//
//  14. P5 IS ANSWERED '0': that this radio really does put '0' in byte 21
//     of every MR and MT answer, as its five legends print (MR 971, MT
//     1006, MW 1042, IF 783, OI 1129), so the STRICT parse never refuses
//     a real answer. Under the dialect's MemoryP5 = cat.P5Fixed,
//     core/cat's parseMemoryFields REQUIRES '0' there and returns TxClar
//     false, which is what makes it impossible for a channel read from an
//     FT-891 to carry a true TX-clarifier flag. The strictness was
//     adjudicated (P5 is printed on five blocks where P7's read direction
//     is printed on none), and the residual risk is recorded here rather
//     than pretended away: IF THE RADIO ANSWERS SOMETHING ELSE, EVERY
//     READ REFUSES.
//     STAGE R LIFTS IT WITH: any MR or MT answer captured from any
//     channel, byte 21 dumped as a RAW value. A single non-'0' byte
//     converts the parse posture from strict to tolerant and is a
//     finding, not a tweak.
//
//  15. A SINGLE COMBINED MT SET SUFFICES TO CREATE OR OVERWRITE A CHANNEL,
//     INCLUDING AN EMPTY ONE — AND AN ACCEPTED SET DRAWS NO REPLY WHILE A
//     REJECTED ONE DRAWS "?;". TWO ASSUMED HALVES IN ONE ENTRY, because
//     ONE capture lifts both and one line of code (the write's
//     transport.CommandSpec) asserts both. IT LANDS WITH THE DRIVER
//     SKELETON, ONE TASK AHEAD OF THE WRITE PATH, because it is the
//     assumption the whole MT-only choreography rests on and it must not
//     arrive later than the design it justifies.
//     WHAT IS MANUAL-EVIDENCED is that ONE frame carries everything: the
//     41-byte Set carries the full field block, the display flag and the
//     tag (layout 996-1027), so MW would write the same fields
//     redundantly in a strictly smaller frame (28 bytes, 1034-1042) and
//     could not carry the tag or the flag at all — and MW's own P1 legend
//     is restricted to memory and PMS (1035-1036), a second reason not to
//     reach for it. This driver sends no MW frame.
//     WHAT IS ASSUMED, FIRST HALF: whether this radio accepts the
//     combined Set as a complete channel definition, and whether it does
//     so for a slot that does not yet exist. The FT-710's own empty-slot
//     create is HW-CONFIRMED for ITS two-frame MW+MT choreography, which
//     is not this one.
//     WHAT IS ASSUMED, SECOND HALF: the acknowledgement convention — see
//     THE ACKNOWLEDGEMENT CONVENTIONS above, which is this half stated in
//     its own right.
//     STAGE W LIFTS BOTH HALVES AT ONCE WITH: the FIRST write trial — one
//     combined MT Set to a sacrificial EMPTY channel, then an MT read
//     back, then the same against an already-populated channel, WITH THE
//     PORT WATCHED between the Set and the read-back. Byte-faithful
//     read-back on both is the first half's lift; anything else
//     (rejection, partial field application, tag written without the
//     field block) converts the write path to a two-frame choreography
//     and this half to a finding.
//
//  16. A DISCOVERED SLOT KEEPS ANSWERING MR WITHIN A SESSION (read.go's
//     readDiscovered). That a 5xx or EMG slot which answered a well-formed
//     MR read during Open answers the SAME MR read again, later in the
//     same session. THIS MANUAL STATES NO SUCH RULE: MR's slot legend says
//     a well-formed answer means present and a "?;" means absent (the "?;"
//     ON A 5xx/EMG DISCOVERY PROBE MEANS ABSENT FROM THIS RADIO entry), but
//     that entry is about the FIRST read of a slot in a session, at the
//     moment discovery is deciding bank MEMBERSHIP — it says nothing about
//     a SECOND read of the identical frame, later, once membership is
//     already settled. Reading that second "?;" as "empty" would be a
//     FOURTH interpretation of this protocol's single unattributed NAK,
//     where matrix §3.8 draws only three, and it is the one interpretation
//     with no register entry and no matrix section (matrix erratum M-E6,
//     §3.8.4, from the task-1 review).
//     WHAT MAKES "EMPTY" DISHONEST HERE, SPECIFICALLY: the bank this slot
//     lives in is published NoBlank TRUE by this very session
//     (caps.go's effectiveCapabilities, "these channels exist because they
//     answered a read"). A slot that answered at Open and rejects at
//     ReadChannel time is exactly a slot for which that premise has
//     failed, so emitting the one channel shape the bank's own NoBlank
//     flag declares impossible would not suppress the anomaly, it would
//     DEFER AND MISATTRIBUTE it: the read would look complete and a LATER
//     codeplug.Validate would blame the codeplug for something the radio
//     did (verified empirically in the task-1 review: such a read, then
//     Validate, produces `slot "503" is part of NoBlank bank "60M" and
//     must stay populated, but is empty`).
//     THE FIX IS A TYPED REFUSAL, in the same shape as
//     MTReadRejectedForOccupiedSlotError: *MRReadRejectedForDiscoveredSlotError
//     names the slot and the contradiction — this slot answered an MR read
//     during this session's Open and refuses the identical frame now — and
//     does not diagnose it, because "?;" carries no reason code here
//     either. The session read fails WHOLE, for the same reason the
//     memory/PMS refusal does: a partial read that silently dropped a
//     slot the driver itself just discovered would be a codeplug the user
//     could not tell from a complete one.
//     STAGE R LIFTS IT WITH: a second MR read of the same slot within one
//     session on a real FT-891 — the same frame the discovery probe
//     already sent, repeated once more (e.g. at the point ReadChannel
//     would otherwise send it). Both answers agreeing closes the entry;
//     either one rejecting having accepted the other is the finding this
//     refusal exists to report honestly rather than mask.
package ft891

// SPDX-License-Identifier: GPL-3.0-or-later

// Package ftdx101 is the Yaesu FTdx101 driver: ONE driver package for TWO
// registered radio models, the FTDX101D and the FTDX101MP, exposed as the
// thin constructors NewD and NewMP over one parameterised implementation.
//
// It is the project's THIRD implementation of core/driver's Driver/Session
// seam and the first to serve more than one model. Yaesu prints ONE CAT
// manual for both radios and distinguishes them in exactly three places —
// the ID answer's P1 legend, the P4 VALUE ranges of three MAX POWER rows in
// Table 2, and the PC command's P1 range — of which only the first touches
// anything this package models. The sole model-conditional capability
// values are therefore Model and CATID, and modelParams (ftdx101.go) is
// where they live.
//
// It is structured on core/driver/ftdx10 and it deliberately does NOT
// import it, nor core/driver/ft710. Nothing here may: those drivers' shapes
// are templates, but their VALUES are their own radios' manual readings and
// hardware findings, and importing them is how one radio's evidence
// silently becomes another's claim. Where a decision here looks like the
// FTdx10's, the comment at the decision says whether the agreement is a
// manual fact of THIS radio, a structural requirement, or an assumption (in
// which case it is in the register below).
//
// # Provenance
//
// Everything protocol-shaped here comes from the Yaesu
// FTDX101MP/FTDX101D CAT Operation Reference Manual, edition 2308-L,
// through core/cat/ftdx101's dialects — the ONE core/cat package this
// package names, at exactly two call sites, one per model (modelD and
// modelMP, ftdx101.go). "Layout N" citations in this package are line
// numbers in the layout-preserved extraction of that manual recorded at
// docs/fixtures-private/manuals/m9d-manual-provenance.md; the manual and
// its extraction are gitignored (Yaesu copyright), so these are citations,
// not links. The per-value evidence status of everything this driver claims
// is docs/superpowers/m9d2-capability-matrix.md, field by field and
// separately for the two models; the dialect package's own doc.go carries
// the wire-level citations, its SEVEN-entry ASSUMED register, its
// reused-command verification verdict and the chart defects. This package
// CITES both and duplicates neither.
//
// NO FTdx101 OF EITHER MODEL HAS EVER BEEN ASKED ANYTHING by this project,
// and nothing in this package has been written to a radio. There is no
// docs/hardware-notes.md section for either model, no golden vector
// captured from either, and no Stage R or Stage W session for either. That
// is the difference between this driver and the FT-710's, and the register
// below is where it is written down rather than glossed.
//
// # The write guard, per model
//
// writeTrialsCompleteD and writeTrialsCompleteMP are BOTH FALSE (caps.go)
// and pinned false by their own test, ROW BY ROW. A RealHardware session of
// either model therefore gets the all-Unverified capability set — every
// candidate field's Write spec.Unverified, nothing writable anywhere — so,
// UNLESS THE USER HAS CONSENTED (below),
// codeplug.Diff blocks every change, the clone service refuses to execute
// one, and the write path's own capability re-check refuses before any
// frame is built. An unrecognised Profile value fails the same way (see
// ftdx101Driver.Capabilities): the failure direction for a forged or
// corrupted Profile is always "nothing writable".
//
// TWO constants and not one, because the evidence is per model: a write
// trial on a D lifts the D's guard and nothing else, and a single shared
// constant could not express a one-model flip. Flipping either is an
// explicit M9d non-goal.
//
// THE ONE ROUTE PAST THE GUARD IS THE USER'S RECORDED CONSENT. A
// RealHardware session opened with WithConsentedUnverifiedWrites — the
// option internal/wiring spends a user's stored grant through — carries
// spec.ConsentedUnverified where the profile said spec.Unverified, and
// FieldSupport.CanWrite is true for that state, so such a session CAN
// write. That is the design, not a leak: the user has been shown the
// warning and accepted the risk for this model. What it does not change is
// the EVIDENCE — both constants stay false, this package's static
// Capabilities() stays all-Unverified (which is what
// internal/wiring.NeedsUnverifiedConsent reads to call either model
// consent-eligible in the first place), and every unconsented session
// behaves exactly as above. Consent is PER MODEL for the same reason the
// constants are: a grant recorded against the D is stored under the D's
// own slug and unlocks nothing on an MP. The transform is applied once, at
// sessionCapabilities, and only for a RECOGNISED Profile, so the forged-
// or-corrupted-Profile direction survives consent untouched.
//
// The Simulated profile is write-Supported, against internal/fakedx101
// only — see the non-borrowing note below.
//
// # The Simulated profile's clarifier is Supported, not Inert
//
// The FT-710's Simulated profile marks FieldClarifier's Write spec.Inert in
// EVERY profile, and that is a HARDWARE FINDING about the FT-710: on
// 13/07/2026 a real one accepted MW frames carrying non-zero clarifier
// values without rejection and read back zeros every time. It is not
// borrowed here. Neither FTdx101 has been asked what it does with a
// clarifier value, so this driver has no finding to record — and the
// Simulated profile describes what happens against internal/fakedx101,
// which stores the clarifier and returns it byte-faithfully through the
// combined MT form. Simulated therefore carries clarifier Write
// spec.Supported, and the profile's honesty is intact: it is a claim about
// the fake, which is where that profile is the only legal one.
//
// If a Stage W session ever shows an FTdx101 ignoring the clarifier the way
// the FT-710 does, the change is a per-profile Inert here PLUS the same
// change in the fake, FOR THAT MODEL — never one of the two, and never both
// models on one radio's evidence.
//
// # MR is deliberately unused
//
// The read path is MT-ONLY: one combined 41-byte MT read carries the whole
// field block AND the tag (read.go). MR is never sent by this driver — not
// by ReadChannel, and not by discovery, which probes with MT reads for the
// same reason.
//
// This is a DESIGN DECISION, not an omission, and it is written here so
// that nobody later "completes" the read path by adding the MR exchange the
// FT-710 needs. The combined answer is an ATOMIC snapshot: a two-frame
// stitch (field block from one radio state, tag from a later one) is
// structurally impossible, whereas the FT-710's MR+MT pair has to hold a
// session-wide lock across the gap to avoid tearing. That the combined form
// carries everything is MANUAL-EVIDENCED — the MT Set/Answer position chart
// runs to 41, the 28 shared positions plus P11 fixed "0" at 28, a
// 12-character P12 tag at 29-40 and ';' at 41 (legends at layout 1311-1330,
// independently counted off 300 dpi raster renders in
// core/cat/ftdx101/testdata/geometry-witness.csv) — and MT's availability
// row is O O O X (layout 334) while MR's is X O O X (layout 331), so both
// directions exist and this driver simply does not use one of them. MR
// stays fully covered at the DIALECT level — golden vectors,
// core/cat/dialecttest's conformance suite, the fake's answers — because a
// dialect describes a radio's protocol, not one driver's use of it.
//
// The same one-exchange property is why this Session carries no operation
// mutex: transport.Engine already serialises each individual exchange, and
// every logical operation here is exactly one. A future FTdx101 operation
// needing two frames needs an opMu with it, and must not assume this one is
// safe without it.
//
// # No per-class kind narrowing, and a legend that invites one
//
// The FT-710's driver keeps its own P7 kind vocabulary per bank because its
// leniencies are ITS live observations. This driver has none, so it adds no
// check of its own: cat.Dialect.ParseMTAnswerCombined already narrows P7 to
// the combined record's OWN documented read pair {'0' VFO, '1' Memory} —
// MT's P7 legend reads "Set: 0: (Fixed) / Read: 0: VFO 1: Memory" (layout
// 1324), and MR's answer P7 gives the same two values (layout 1290) — and
// an out-of-vocabulary byte surfaces from there as a *cat.ParseError,
// wrapped by this driver with the slot it was reading.
//
// THIS MANUAL PRINTS A SIX-VALUE P7 ELSEWHERE, and it must not be read
// across: IF's and OI's P7 legends give "0: VFO 1: Memory 2: Memory Tune 3:
// Quick Memory Bank (QMB) 4: - 5: PMS" (layout 1092-1093 and 1447-1448).
// Those are IF's and OI's own parameters on their own frames, not the
// memory record's. Widening MT's accepted P7 to six values on the strength
// of a different command's legend would be inventing a distinction this
// manual does not draw for MT.
//
// # RegionReporter is NOT implemented
//
// core/driver.RegionReporter is optional (core/driver/optional.go), and
// this driver does not satisfy it, for EITHER model. The FT-710's Region()
// derives a regulatory-region string from its discovered 5xx/EMG inventory
// using fingerprints — 7 channels and no EMG is "UK", 15 plus EMG is "US" —
// that are that radio's own, partly hardware-confirmed and partly still
// assumed. There is no honest FTdx101 vocabulary to report: NO FTdx101
// INVENTORY OF EITHER MODEL HAS EVER BEEN OBSERVED, so a discovered count
// maps to no region name this project could defend, and borrowing the
// FT-710's fingerprints would mean answering a question about one radio
// with another radio's evidence. Callers already tolerate absence (the
// capability is reached by a two-result type assertion), so nothing breaks:
// codeplug.RadioInfo.Region simply stays unset for an FTdx101.
//
// A Stage R session that enumerates a real FTdx101's 5xx/EMG bank is what
// would make a region vocabulary possible — and, per the per-model rule
// below, one taken from a D would say nothing about an MP. It is not a code
// gap to be filled meanwhile.
//
// # Discovery walks the WHOLE declared range
//
// Open probes EVERY slot core/cat/ftdx101's SlotSpace declares — 501
// through 599, ascending — and then EMG. There is no contiguity assumption,
// no sentinel, no early termination and no bound: the FT-710's discovery
// rules (stop at the first rejection, cap at 15, probe one sentinel past
// the cap) are FT-710 HARDWARE facts about a radio whose factory 5xx
// channels are believed contiguous and non-erasable, and none of that is
// known for either radio here. A sparse bank — a populated 503 after an
// empty 502 — must not be silently truncated, and the only way to be sure
// of that with no evidence is to ask every declared slot. The 5xx family's
// and EMG's EXISTENCE is manual-evidenced (five of the six slot legends
// carry "5xx (5MHz BAND), EMG (EMERGENCY CH)"); their NUMBERING is not, and
// is the DIALECT register's entry "SlotSpace.SixtyLo/SixtyHi = 501/599",
// cited here and not restated.
//
// Discovery REFUSES RATHER THAN GUESSES. A malformed answer or an answer
// naming a different slot aborts the walk, fails Open with the typed error
// (a wrapped *cat.ParseError or this package's *AnswerMismatchError), and
// closes the port; no bank is synthesised from the partial result. Half a
// walk is not a smaller inventory, it is an unknown one.
//
// The cost is ~100 exchanges per Open, about 2-2.5 s at the engine's
// default per-exchange settle, paid by every FTdx101 session this project
// opens including in tests — and TWICE OVER across the package's gates,
// because two registered models mean two models' worth of sessions. That is
// ACCEPTED and budgeted (M9d-2 plan, "Wall-clock is budgeted, not
// optimised"). NOBODY narrows it — settle override, early termination,
// range shrink — without an orchestrator-adjudicated change: an ad-hoc
// optimisation here is exactly how the termination assumptions this design
// refused would come back. TestOpen_DiscoveryProbesEveryDeclaredSlot pins
// the full ordered transcript, for each model, so that a regression is a
// test failure rather than a silently shorter walk.
//
// # The ASSUMED register
//
// NINE ENTRIES, covering TEN behaviours this driver encodes that are NOT
// FTdx101-manual facts. Each is listed here once, marked ASSUMED at the
// point of use, and paired with the ONE Stage R or Stage W capture that
// lifts it. Entry 9 carries two behaviours rather than one — the combined
// Set's sufficiency and the acknowledgement convention that governs what
// comes back from it — because ONE capture, the first write trial, lifts
// both and because one line of code (write.go's zero CommandSpec) asserts
// both. The entry says so in its own words; the numbering is left alone,
// since a renumbering would silently invalidate every citation into this
// register from the fake, the dialect and the matrix.
//
// EVERY ENTRY IS TRACKED PER MODEL, and this is the register's one
// structural difference from the FTdx10's. There are two radios here, and
// A CAPTURE FROM ONE IS NEVER EVIDENCE ABOUT THE OTHER: the D and the MP
// share a manual, not a serial port, so "the D answered 0005 to a
// non-multiple-of-10 clarifier Set" is evidence about the D and about
// nothing of the MP's. Every lift below is therefore worded per model, and
// an entry stays open for whichever model has not been asked — a
// half-retired entry is the normal state of this register, not an error in
// it. The captures are individual on purpose in the other direction too:
// one session does not retire the register wholesale, it retires the
// assumptions its own frames actually speak to.
//
// EVERY CAPTURE MUST RECORD ITS PORT. This radio offers two CAT paths and
// they are NOT equivalent (matrix §3.12, manual-evidenced): it "contains
// two virtual COM ports, an Enhanced COM Port and a Standard COM Port",
// only the Enhanced one being for CAT (layout 75-79); AI "is available only
// when PC is connected with USB cable" (layout 381), which is exactly the
// command Open's init sends; the rear RS-232C jack cannot be used while the
// external antenna tuner is selected (layout 111-112, restated at 123); and
// PS is unavailable over it (layout 105-106, 1542-1544). A capture taken
// over RS-232C cannot lift the AI-related half of the open choreography at
// all, and silence on the wrong virtual COM port looks exactly like a wrong
// baud or a framing mismatch — which entries 1 and 2 below both depend on
// being able to tell apart.
//
// This is the DRIVER's register. The DIALECT's SEVEN entries
// (core/cat/ftdx101/doc.go) are separate and are CITED BY NAME at this
// driver's own points of dependence — never by position, because a register
// renumbered at a review leaves positional citations pointing at the wrong
// entry. All seven are depended on here:
//
//   - "MTPolicy.TagFill = ' '" — the byte a short P12 tag is padded with
//     and trimmed of, on every combined MT answer ReadChannel maps
//     (read.go) and every Set write.go's buildWriteCommand builds. TagLen 12
//     itself does not depend on it (that width is manual-evidenced, layout
//     1330).
//   - "ClarifierPolicy.StepHz = 10" — the capability value ClarStepHz
//     (caps.go) and the multiple-of-step rule core/cat enforces on every
//     combined-MT Set write.go's buildWriteCommand builds.
//   - "SlotSpace.NoneWire = \"000\"" — the read path's refusal of the
//     answer-only none form: grammatical per ParseSlot, never a legal read
//     target, refused by BuildMTRead and documented at that call site
//     (read.go).
//   - "the cat.ModeUnset member of the mode table" — the ONE member
//     caps.go's modeNames excludes from the advertised mode list, so that
//     no user is offered a mode core/cat refuses to emit.
//   - "SlotSpace.SixtyLo/SixtyHi = 501/599" — the extent of the discovery
//     walk (ftdx101.go's discoverInventory), which asks the dialect for
//     successive 5xx ordinals until it refuses rather than writing a bound
//     down.
//   - "The combined MT answer's EXACT length (consumed here as
//     MTAnswerBounds() = (41, 41))" — the ExpectLen of every MT read this
//     driver sends (read.go's mtSpec), derived from the dialect precisely
//     so that the recorded 30..41 window contingency would move it.
//   - "The CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D
//     ('-')" — the byte a negative clarifier offset is read at (read.go's
//     ClarHz mapping) and written with in the frames write.go's
//     buildWriteCommand builds — the two call sites a Stage W lift of this
//     entry must look at. This manual prints that direction as a two-hyphen
//     glyph and the quarantined golden deriver recorded it UNREADABLE rather
//     than resolving it.
//
// Correcting one of those is a dialect change; correcting one of these is a
// change here. Neither list may absorb the other.
//
//  1. FRAMING: 8 data bits, no parity, TWO stop bits (core/transport's
//     DefaultStopBits, core/transport/port.go:27, which every session this
//     driver opens inherits — there is no framing field in
//     spec.Capabilities and, per the E2-recorded decision, none is added
//     without hardware evidence). THIS MANUAL IS SILENT ON FRAMING: it
//     states no stop-bit count, no parity and no data-bit width anywhere.
//     Its connection sections (layout 16-126) describe cables, virtual COM
//     ports and a level converter but no line discipline, and its serial
//     menu block carries only 232C RATE, 232C TIME OUT TIMER, CAT RATE, CAT
//     TIME OUT TIMER and CAT RTS (layout 861-865). 8-N-2 is the FT-710's
//     own documented framing, inherited here as a same-generation working
//     value by way of a transport default.
//     STAGE R LIFTS IT, PER MODEL, WITH: an ID exchange at the answering
//     baud with that radio's port opened 8-N-2 — a clean "ID0681;" (D) or
//     "ID0682;" (MP) confirms the framing is at least compatible for that
//     model. THE LIFT MUST DISTINGUISH FRAMING FROM THE CONTROL LINES
//     (entry 2): silence at a known-correct baud is NOT evidence about stop
//     bits until CAT RTS has been toggled at the radio and the exchange
//     retried, because a handshake refusal and a framing mismatch present
//     identically as nothing coming back. Try 8-N-1 only after that has
//     been done, and record which of the two changed the outcome — and on
//     which port (see the register preamble).
//
//  2. CONTROL-LINE POLICY: that driving RTS and DTR low unconditionally is
//     safe on these radios. core/transport.OpenSerial does exactly that
//     immediately after opening any port (SetRTS(false), SetDTR(false) —
//     safety obligation 4, core/transport/port.go:107-119), for every
//     model, with no per-radio policy, and this driver adds none. THIS
//     RADIO HAS A CAT RTS MENU ITEM OF ITS OWN: (03,01,13) CAT RTS, P4
//     legend "0: DISABLE 1: ENABLE" at layout 865 (printed page 11),
//     committed at core/cat/ftdx101/table2.csv and generated into that
//     package's exinventory_gen.go. So the radio evidently has an opinion
//     about the line that this project's transport does not consult. What
//     that menu's factory setting is, and whether a low RTS with it ENABLEd
//     stalls the CAT link, is unknown for both models. NOTE THE ADDRESS:
//     the FTdx10's CAT RTS sits at (03,01,10) and this radio's at
//     (03,01,13); the addresses are not interchangeable, and this one is
//     established from the FTdx101's own chart.
//     STAGE R LIFTS IT, PER MODEL, WITH: one ID exchange in each CAT RTS
//     menu state on that radio, everything else held constant. If the
//     exchange answers in both, the policy is safe on that model and the
//     entry closes for it; if it answers in only one, the transport needs a
//     per-radio control-line policy and this becomes a spec'd capability
//     rather than an assumption. Take this capture BEFORE concluding
//     anything about entry 1.
//
//  3. DefaultBaud 38400 (caps.go). THE RATE MENU IS MANUAL-EVIDENCED —
//     (03,01,11) CAT RATE, P4 legend "0: 4800 bps 1: 9600 bps 2: 19200 bps
//     3: 38400 bps" at layout 863 (printed page 11), four rates and no
//     115200 — but THE FACTORY DEFAULT IS NOT IN THIS CAT MANUAL AT ALL.
//     Table 2's column headers are "P1 P2 P3 Function P4 Digits" (layout
//     716, printed page 10): there is no factory-default column, and the
//     trailing 1 on layout 863 is the DIGITS field, exactly how the
//     committed inventory reads it (core/cat/ftdx101/table2.csv, digits=1).
//     That is the misreading the FTdx10 spec made once and recorded so it
//     could not recur; it is recorded again here for the same reason, and
//     because §1.9's citation sits one line from this value in caps.go, a
//     reader who read it as covering both would promote an assumption by
//     proximity. The FT-710's 38400 is an OPERATING-manual fact for that
//     radio; this project holds no FTdx101 operating manual, so 38400 here
//     is the same-generation family default and nothing more. It matters
//     operationally: internal/wiring's OpenRealSessionFor opens a real
//     radio at exactly this driver's DefaultBaud.
//     STAGE R LIFTS IT, PER MODEL, WITH: the baud a FACTORY-CONFIGURED
//     radio of that model answers an ID exchange at — try 38400 first, then
//     the other three in turn. The answering rate is the fact; a radio
//     whose CAT RATE has been changed by its owner cannot settle it, so the
//     capture must record whether the menu was known-untouched.
//
//  4. MinFreqHz 30_000 / MaxFreqHz 75_000_000 (caps.go). This manual
//     carries NO storable-frequency range statement. Every frequency legend
//     says only "VFO-A Frequency (Hz)" or "Frequency (Hz)" over a 9-digit
//     field (layout 1084 for IF, 1280 MR, 1314 MT, 1354 MW, 1438 OI), which
//     bounds the ENCODING at 999999999 and says nothing about what either
//     radio will store. The nearest thing to a range is BS (BAND SELECT)'s
//     P1 legend — "00: 1.8 MHz … 10: 50 MHz, 11: GEN, 17: 70 MHz" (layout
//     506-514) — and IT IS NOT EVIDENCE FOR THESE FIELDS: it names transmit
//     bands and a general-coverage position, not a receive floor or a
//     storable ceiling, and reading a 70 MHz entry as a MaxFreqHz would be
//     inventing a number. It is named here so a later reader does not
//     "find" it and quietly promote this entry. MaxFreqHz is additionally
//     the ledgered dangerous-zero field — a zero there reads as "no
//     ceiling" to every validator — so it MUST be populated; this entry is
//     the honesty about where the number came from, not a licence to leave
//     it empty.
//     STAGE R LIFTS IT, PER MODEL, WITH: the specifications page of THAT
//     MODEL'S operating manual (a document, not a session — the cheapest
//     lift in this register), or, failing that, edge probes: MT Sets at the
//     claimed floor and ceiling and just outside them, to a sacrificial
//     channel, recording which are accepted. The D and the MP are different
//     radios in this respect in principle, so the D's specifications page
//     is not evidence about the MP.
//
//  5. RequiredSlots {"001"} (caps.go). That memory channel 001 must never
//     be empty. This manual states no such rule for either model anywhere.
//     Claiming it makes codeplug validation refuse a candidate whose 001 is
//     blank, which is the conservative direction — refuse rather than write
//     a state the radio may not tolerate — but it IS a claim. It is the
//     per-SLOT mechanism; Bank.NoBlank is the per-BANK one and is false on
//     both static banks, deliberately (asserting NoBlank on MEM to protect
//     001 would be the M5b PMS failure repeated).
//     STAGE R LIFTS IT, PER MODEL, WITH: observation of channel 001 on a
//     real radio of that model — whether it ships with 001 populated, and
//     whether the front panel will erase it at all. A radio that erases 001
//     happily drops this from its RequiredSlots.
//
//  6. TONE AND SCAN-SKIP UNREACHABILITY (caps.go's zero FieldSupport for
//     spec.FieldCTCSSTone and spec.FieldScanSkip; read.go's Unknown
//     states). Neither field is claimed readable or writable in either
//     direction on any bank or profile. WHAT IS TRUE HERE IS STRUCTURAL:
//     the combined MT record accounts for every one of its 41 positions —
//     slot, frequency, clarifier sign and magnitude, the two clarifier
//     flags, mode, kind, CTCSS state, the fixed "00" P9, shift, the fixed
//     "0" P11, the 12-byte tag, terminator (geometry-witness.csv's MT rows;
//     legends at layout 1312-1330) — P9 is documented fixed "00" (layout
//     1326), and no command this driver sends carries a tone NUMBER or a
//     scan-skip flag for a memory channel at all. WHAT IS ASSUMED is the
//     step from that to "these fields are unreachable on this radio":
//     nothing verifies that the CTCSS-state byte means anything live here,
//     and whether some OTHER command in this manual could reach a memory
//     channel's tone number is not established either way. The FT-710's
//     answer — that none can, its P9 reading fixed "00" with a tone
//     demonstrably set and active — is that radio's hardware finding and is
//     not borrowed.
//     STAGE R LIFTS IT, PER MODEL, WITH: one channel set to a known CTCSS
//     tone from the front panel of that radio, then read over CAT — if any
//     byte of the answer tracks the tone number, the entry is refuted for
//     that model and the capability opens; if P9 reads "00" as documented,
//     it closes as a confirmed protocol limit rather than an assumption.
//     The scan-skip half needs the same experiment with the front-panel
//     skip flag.
//
//  7. "?;" ON A 5xx/EMG DISCOVERY PROBE MEANS ABSENT FROM THIS RADIO
//     (ftdx101.go's discoverInventory/probeSlot). Discovery treats a
//     rejection as "this radio does not have that channel" and a
//     well-formed answer as "it does". "?;" is the protocol's SINGLE
//     unattributed NAK (cat.ErrRejected's own doc comment): it is also what
//     an unknown command, a bad parameter and a wrong radio state get, so
//     reading "absent" out of it is an interpretation. The FT-710's
//     equivalent interpretation is hardware-confirmed for that radio (live
//     probes of a never-populated and an out-of-inventory slot both
//     answered "?;"); neither FTdx101's is.
//     STAGE R LIFTS IT, PER MODEL, WITH: an MT enumeration of the whole 5xx
//     range on a radio of that model with a POPULATED 5xx bank,
//     cross-checked against the channels the front panel shows. Which wire
//     numbers answer and which reject is then a fact for that model.
//     TWO REGISTER ENTRIES, ONE POSSIBLE CAPTURE — RECORD BOTH EXPLICITLY.
//     An enumeration extended to include 500 speaks to THIS entry (what a
//     rejection MEANS) and to the DIALECT register's "SlotSpace.SixtyLo/
//     SixtyHi = 501/599" (where the range STARTS AND STOPS). They are
//     separate assumptions; one session can retire both for one model, but
//     a session that answers only one leaves the other open, and an entry
//     retired by implication is an entry nobody can audit. The dialect's
//     entry words its lift as an MR enumeration because a dialect describes
//     the protocol; this one words it as MT because this driver's discovery
//     sends MT and never MR. Either command's enumeration serves.
//
//  8. "?;" ON A COMBINED-MT READ OF AN EMPTY SLOT (read.go's ReadChannel).
//     A rejection is mapped to an EMPTY codeplug.Channel — Data nil, the
//     slot carried through — rather than an error, so a read of a blank
//     channel is an ordinary outcome. The FT-710's equivalent was verified
//     for ITS MR read, NOT for this frame, and neither FTdx101's combined
//     MT read of an empty channel has ever been seen. The failure mode if
//     this is wrong is quiet and bad: a transport-level problem manifesting
//     as "?;" would be recorded as a genuinely empty channel and could
//     later be written back as one.
//     STAGE R LIFTS IT, PER MODEL, WITH: one MT read of a channel known
//     empty from that radio's front panel, and one of a channel known
//     populated, in the same session. Two different answers confirm the
//     mapping for that model; two rejections mean the driver has been
//     reading blank channels out of a broken link.
//     This entry and entry 7 SHARE ONE WIRE SIGNAL AND ONE MECHANISM and
//     are deliberately kept apart: a capture that settles one does not
//     settle the other.
//
//  9. A SINGLE COMBINED MT SET SUFFICES TO CREATE OR OVERWRITE A CHANNEL,
//     INCLUDING AN EMPTY ONE — AND AN ACCEPTED SET DRAWS NO REPLY WHILE A
//     REJECTED ONE DRAWS "?;". TWO ASSUMPTIONS, ONE ENTRY, because one
//     design decision rests on both: the write path sends the combined Set
//     with the ZERO transport.CommandSpec, which is a claim about what the
//     frame does AND a claim about what comes back. THE ENTRY LANDS HERE
//     WITH THE DRIVER SKELETON, ONE TASK AHEAD OF THE WRITE PATH IT
//     GOVERNS, because it is the assumption the whole MT-only choreography
//     — including the read side's atomicity argument above — rests on, and
//     it must not arrive later than the design it justifies.
//
//     WHAT IS MANUAL-EVIDENCED is that ONE frame carries everything: the
//     41-byte Set carries the full field block and the tag (layout
//     1311-1330; geometry-witness.csv's MT set rows), so MW would write the
//     same fields redundantly with a strictly smaller frame (28 bytes,
//     layout 1352-1367), and MW's own restriction to "001-099 (Memory
//     Channel), P1L -P9U (PMS)" (layout 1353) is a second reason not to
//     reach for it.
//
//     WHAT IS ASSUMED, FIRST HALF: that either radio accepts the combined
//     Set as a COMPLETE CHANNEL DEFINITION, and that it does so for a slot
//     that does not yet exist. The FT-710's own empty-slot create is
//     HW-CONFIRMED for ITS two-frame MW+MT choreography, which is not this
//     one.
//
//     WHAT IS ASSUMED, SECOND HALF — THE ACKNOWLEDGEMENT CONVENTION, added
//     at the M9d-2 milestone review, which found it travelling here
//     unregistered: that an accepted combined Set produces NO REPLY AT ALL
//     and that a rejected one produces exactly one "?;". THAT IS AN
//     INHERITED FRAMING CONVENTION, NOT A READING OF THIS MANUAL. The
//     FT-710's manual states silence-on-success as a general framing rule
//     and this project adopted it there; THIS manual states neither half.
//     It describes Set, Read and Answer commands and the terminator (layout
//     190-193, 227-229) and never says what a radio returns to a Set it
//     honours, or to one it cannot; its layout-preserved extraction of rev
//     2308-L contains no '?' character anywhere.
//     AND MT'S AVAILABILITY ROW DOES NOT SUPPLY IT. That row (layout 334)
//     gives Set O, Read O, Answer O, AI X, and this entry read the Answer O
//     as "so a Set produces no answer" until the milestone review. It is a
//     non sequitur, and it is corrected rather than deleted because the
//     wrong version was the stated reason for a live design decision. The
//     table's own header reads "Set Read Ans. AI" over each of its two
//     command columns (layout 236, spacing normalised here): the Ans.
//     column marks the EXISTENCE OF THE COMMAND'S ANSWER FORM — the frame
//     that comes back to a READ — which is why MR, a
//     read-only command, carries an Answer O too (its row is X O O X,
//     layout 331). The column grounds nothing whatever about Sets.
//     THE PAIRED ANALYSES ARE THE FAKE'S, cited and NOT absorbed:
//     internal/fakedx101/doc.go's register entry 11 (an accepted Set
//     produces no reply, a rejected one exactly one "?;" — with the one
//     piece of partial evidence there is, MW's O X X X row at layout 336)
//     and its entry 16 (the "?;" convention ITSELF is inherited and
//     unattested on these radios). Those are claims about the FAKE; this
//     one is a claim about what the DRIVER expects of a real radio, and a
//     capture retires them together or not at all. Neither register may
//     absorb the other.
//     THE SITES THAT DEPEND ON IT, all three: write.go's mtSetSpec (the
//     zero CommandSpec — no ExpectPrefix, so transport treats silence as
//     acceptance and a bounded "?;" listen as the only failure signal),
//     write.go's WriteChannel through it, and ftdx101.go's Open, whose AI0
//     init frame is sent the same way.
//
//     STAGE W LIFTS BOTH HALVES AT ONCE, PER MODEL, WITH: the FIRST write
//     trial on that radio — one combined MT Set to a sacrificial EMPTY
//     channel, then an MT read back, then the same against an
//     already-populated channel, WITH THE PORT WATCHED FOR ANY REPLY AT ALL
//     between the Set and the read. Byte-faithful read-back on both is the
//     first half's lift; anything else (rejection, partial field
//     application, tag written without the field block) converts the write
//     path to a two-frame choreography and that half to a finding. Whatever
//     the port carries in the gap — nothing, a "?;", or an acknowledgement
//     this project has no handling for — is the second half's lift, and it
//     must be recorded even when it is nothing, because "no bytes observed"
//     is the whole content of the claim. A radio that acknowledged a Set
//     would break this driver rather than merely surprise it: the spec
//     waits for no prefix, so the acknowledgement would be left in the
//     buffer for the next exchange to trip over.
//     RECORD THE PORT WITH BOTH (see the register preamble): silence on the
//     Standard COM Port looks exactly like silence-on-success.
//     NOTHING MAY BE WRITTEN AT M9d-2 REGARDLESS: both write guards are
//     false and both stay false.
package ftdx101

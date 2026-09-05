// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft991a is the Yaesu FT-991A driver: the project's FIFTH
// implementation of core/driver's Driver/Session seam over the Yaesu
// NEWCAT grammar, written — like the FTdx10, FTdx101 and FT-891 drivers —
// for a radio this project has never connected to anything.
//
// It is structured on core/driver/ft891, the newest of those, and it
// deliberately does NOT import it, nor core/driver/ftdx10,
// core/driver/ftdx101 or core/driver/ft710. Nothing in this package may: a
// sibling driver's shape is a template, but its VALUES are that radio's
// readings and findings, and importing them is how one radio's evidence
// silently becomes another's claim. Where a decision here looks like a
// sibling's, the comment at the decision says whether the agreement is a
// manual fact of THIS radio, a structural requirement, or an assumption (in
// which case it is in the register below).
//
// THE EXEMPLAR IS A SHAPE AND NOT AN ANSWER, and on two rungs it is exactly
// inverted — see "The two rungs the FT-891 has that this radio must not
// copy". A faithful copy of that driver would refuse a live field and demand
// an absent one.
//
// # Provenance
//
// Everything protocol-shaped here comes from the Yaesu FT-991A CAT
// Operation Reference Book, revision 1711-D, through core/cat/ft991a's
// dialect — the ONE core/cat instance this package names (catDialect,
// caps.go). Citations of the form "layout N" are line numbers in that
// manual's layout-preserved extraction (both PDF and extraction are
// gitignored, so these are citations, not links); the dialect's own doc.go
// carries the digests, the chart-defect record, its ELEVEN-entry ASSUMED
// register and its reused-command verification verdict. This package CITES
// the dialect's entries BY NAME where it depends on them and restates none
// of them.
//
// Every capability VALUE in caps.go comes from
// docs/superpowers/ft991a-capability-matrix.md, with the matrix section
// cited at the field it sets. The matrix is a gitignored record, so those
// citations are citations too; the values they justify are pinned by
// caps_test.go.
//
// NO FT-991A HAS EVER BEEN ASKED ANYTHING by this project, and none is
// available to it. There is no docs/hardware-notes.md section for this
// model, no observation CSV, no corrections file, no captured frame, and no
// Stage R or Stage W session. Every statement here is a reading of a manual
// or a written-down assumption, and the register below is where the
// difference is recorded rather than glossed.
//
// # The write guard
//
// writeTrialsComplete is FALSE (caps.go) and pinned false in both halves by
// its own test. A RealHardware session therefore gets
// CapabilitiesUnverified — every candidate field's Write spec.Unverified,
// nothing writable anywhere — so, UNLESS THE USER HAS CONSENTED (next
// section), codeplug.Diff blocks every change, the clone service refuses to
// execute one, and Session.WriteChannel's own capability re-check refuses
// before any frame is built. An unrecognised Profile value fails the same
// way (see ft991aDriver.Capabilities): the failure direction for a forged or
// corrupted Profile is always "nothing writable".
//
// # The one route past the guard: the user's recorded consent
//
// A RealHardware session opened with WithConsentedUnverifiedWrites — the
// option internal/wiring spends a user's stored grant through — carries
// spec.ConsentedUnverified where the profile said spec.Unverified, and
// spec.FieldSupport.CanWrite is true for that state. The STATIC capability
// set this driver publishes is untouched by the option, and every
// unconsented session still refuses: consent is a decision about risk, not
// evidence, so the profile keeps describing the evidence (none) either way.
//
// Two guards keep the route narrow. spec.ConsentUnverifiedWrites exempts
// spec.FieldErase, so no consent can mint an erase — which on this radio is
// doubly idle, since its Control Command List (layout 123-196) contains no
// erase command at all. And an unrecognised Profile stays on the
// untransformed fail-safe even WITH the option (profileRecognised), so no
// value a caller can pass produces a writable session.
//
// # There is NO discovery, and the pin is a NEGATIVE one
//
// Open sends the family's AI0; preamble and then ID;, and NOTHING ELSE. The
// transcript is two frames on a match and two on a mismatch, port closed
// (matrix §3.4, plan P10 and task 10). There is no slot walk and no probe of
// any bank, because there is nothing to discover: "5xx", "5 MHz", "5MHz" and
// "EMG" appear in NO slot legend of this manual — checked mechanically over
// the whole extraction and recorded at core/cat/ft991a/dialect.go's SixtyLo
// /SixtyHi and EmergencyWire — and both banks this radio does have are dense
// and complete at construction.
//
// THIS IS WHERE THE FT-991A COSTS LEAST AND IS MOST EASILY GOT WRONG. The
// FT-891 spends up to eleven MR exchanges per Open and needs two register
// entries and two "?;" interpretations for the banks it finds. Here all of
// that collapses to an absence — and an absence cannot be asserted by
// watching the right thing happen. A regression that added a walk would
// simply work, slowly, against any peer that answered. So the pin is
// negative and it is the whole transcript: TestOpen_SendsExactlyTwoFrames
// and TestOpen_NeverBuildsAnMROfANonExistentBank.
//
// # The read choreography, and why this Session still has an operation mutex
//
// ReadChannel is ONE combined 41-byte MT read for memory and PMS alike, and
// MR is never sent. The combined answer carries the field block and the tag
// together, so it is an ATOMIC snapshot of the channel: the two-frame stitch
// the FT-710's MR+MT read has to guard against (field block from one radio
// state, tag from a later one) is structurally impossible rather than merely
// locked against.
//
// THERE IS NO MR CROSS-CHECK, AND STAGE 2 MUST NOT INVENT ONE. The FT-891's
// whole read design is built around a contradiction in ITS manual — a
// Control Command List giving MT Set only (ft891_layout.txt:166) against a
// detail block printing a Read chart (:1016) — and this manual has no
// analogue: its availability row is "MT | MEMORY CHANNEL TAG | O O O X"
// (layout 181) and its MT block prints all three charts filled (998-1033),
// with a Read chart at 1018 and a full 41-position Answer chart at 1019-1033.
// Evidence leg G checked the two records against each other for six commands
// and found no disagreement at all. So there is no
// ErrMTReadRejectedForOccupiedSlot analogue here, and no timeout branch of
// that family either (spec §Error handling; plan §Plan-vs-spec ruling 5): an
// MT timeout is a TRANSPORT timeout, surfaced as the transport's own error
// under this path's ordinary slot-naming wrap, and NOT re-typed — errors.Is
// finds transport.ErrTimeout and errors.As finds no driver type standing
// between them. THE LOAD-BEARING HALF OF THAT RULING IS "NOT RE-TYPED"
// (matrix §3.5's clarification of 05/09): the wrap adds the slot the
// transport cannot know, which every other error on this path also carries,
// and it is the fleet convention — core/driver/ft891/read.go wraps the same
// class with the byte-identical format string. If a future capture ever puts
// this radio's MT availability in question, the FT-891's MTReadTimeoutError
// is the shape to add.
//
// THE MUTEX IS STILL EARNED, and by a different property. transport.Engine
// serialises each individual exchange, so one MT read needs no lock of its
// own — but opMu guards a whole DRIVER OPERATION (spec erratum S-E4, matrix
// M-E2), and this session has more than one kind: a read, a write (task 11)
// and a settings read (task 12) must not interleave their frames. The
// concurrency pin is that two racing ReadChannels cannot interleave two MT
// frames. IT IS NOT HELD ACROSS WRITE-THEN-VERIFY: that pair belongs to
// core/clone, as the driver interface assigns it, and holding a driver lock
// across it would serialise two operations the seam deliberately keeps
// separate.
//
// # The two rungs the FT-891 has that this radio must NOT copy
//
// Matrix erratum M-E3, recorded so the exclusion is named rather than left
// for a reviewer to notice. The FT-891 is the closer exemplar on four axes
// this radio genuinely shares — a paper radio, the consent route, an
// MT-centred read with RetryReads 0, and a write gate carrying the 05/09
// sweep's driver.CheckFieldStates walk. It does not share the fifth and
// sixth, and each is exactly inverted:
//
//   - P5 IS LIVE HERE. `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"` is printed on
//     every FT-991A block carrying the 28-position grid — MR 971, MT 1004,
//     MW 1042, IF 787, OI 1122 — where the FT-891 prints `P5 0: (Fixed)`.
//     The dialect carries cat.P5TxClar, so core/cat's builders ACCEPT a
//     TxClar-true record here and refuse one there. The FT-891's
//     `data.TxClar == true` pre-wire refusal must therefore NOT be carried
//     across: it would refuse, on every write, a field this radio's five
//     legends print as live. The refusal that does belong is already in
//     core/cat, in the opposite direction.
//
//   - P11 IS FIXED HERE. MT's P11 legend reads `P11 0: (Fixed)` (layout
//     1015) where the FT-891 prints `0: TAG "OFF" 1: TAG "ON"`. The dialect
//     carries cat.MTPolicy.P11 = cat.P11Fixed, under which the
//     display-BEARING codec pair (BuildMTSetCombinedDisplay,
//     ParseMTAnswerCombinedDisplay) REFUSES and BuildMTSetCombined /
//     ParseMTAnswerCombined are the only route. So the FT-891's TagDisplay
//     rung — which demands a Known flag — would demand a field this frame
//     does not have. read.go answers spec.FieldTagDisplay
//     codeplug.Unavailable, and caps.go grades it the zero FieldSupport.
//
// # No per-class kind narrowing
//
// KIND CHECKING IS THE PARSER'S, not this driver's, and on this radio the
// combined answer's read domain is TRANSCRIBED rather than assumed: MT's own
// P7 legend prints both directions in full, `P7 Set: 0: (Fixed) / Read: 0:
// VFO 1: Memory` (layout 1009), so the '0'/'1' core/cat accepts is this
// manual's own statement — where the FT-891 needed a dialect register entry
// for the same domain.
//
// IF's P7 is a SEVEN-value list (`0: VFO 1: Memory 2: Memory Tune 3: Quick
// Memory Bank (QMB) 4: QMB-MT 5: PMS 6: HOME`, layout 792-793) and OI's is
// the narrow two (1127). Both are those commands' own parameters on their
// own frames, and neither is read across into the memory record's P7 in
// either direction. An out-of-vocabulary byte surfaces from core/cat as a
// *cat.ParseError, which this driver wraps with the slot it was reading, and
// no per-class narrowing is added on top.
//
// # RegionReporter is NOT implemented
//
// driver.RegionReporter exists for a radio whose session can say which
// regional variant it is talking to. This driver's Open asks nothing beyond
// ID;, and this manual prints no region-conditional memory bank at all —
// where the FT-891's 5 MHz legend carries "U.S. and U.K. version only"
// (ft891_layout.txt:962), this one has no such bank to qualify. There is
// therefore no region for a session to report, and implementing the
// interface would mean answering a question this radio has not been asked.
//
// # CAT reaches this radio over TWO enumerated paths, and the menu that
// # gates one of them prints a defect
//
// This project opens one serial port and speaks CAT over it. The FT-991A
// offers two physical paths and the manual describes both (matrix §3.12):
// the rear-panel CAT jack, an RS-232C port with a built-in level converter
// (layout 21-25, its own DE-9 pin table at 17-32), gated by menu
// "028 GPS/232C SELECT" (558) and rated by menu "029 232C RATE" (559); and
// the rear-panel USB jack, a built-in USB-to-DUAL-UART bridge (46-47), rated
// by menu "031 CAT RATE" (561). DefaultBaud's register entry names 031 and
// excludes 029 for that reason (plan P11).
//
// THE WORD "DUAL" IS THE ONLY MENTION OF THE SECOND USB ENDPOINT in the
// whole document. The FTdx101's manual NAMES its two virtual ports (Enhanced
// for CAT, Standard for TX control); this one does not, so an operator has
// no documentary way to know which of the two enumerated COM ports is the
// CAT one and must determine it empirically.
//
// AND THE GATING MENU'S OWN ROW IS ONE OF THE CHART'S PRINTED DEFECTS:
// "028 GPS/232C SELECT" prints `0: GPS1 1: GPS2 3: RS232C` (layout 558) —
// key `2:` is SKIPPED. Whether the radio's P2 domain really has a hole at 2
// or the chart lost an entry is not knowable from this manual, and nothing
// here fills the gap in (core/cat/ft991a/doc.go, "Chart printing defects").
//
// No action for this driver's code — it opens whatever port it is given —
// but ANY FT-991A CAPTURE MUST RECORD WHICH ENUMERATED SERIAL DEVICE IT WAS
// TAKEN ON, and whether menu 028 was set to RS232C. A capture that cannot
// say is not evidence about framing or control lines, which is what makes
// this section a precondition of the first two register entries rather than
// background.
//
// # Two captures the Stage 1 evidence does not cover
//
// Distinct from the register below, and recorded here because they are gaps
// in the GOLDEN VECTORS rather than assumptions in the code: evidence leg G
// built this radio's vector files from the manual's own charts, and two
// shapes this driver can produce or accept have no vector behind them.
//
//   - ONE MR READ OF A CHANNEL WITH THE TX CLARIFIER ON — byte 21, position
//     21, answered '1'. No G vector carries it, so the live P5 this radio
//     prints on all five blocks (971, 1004, 1042, 787, 1122) is exercised in
//     this package only against hand-built fixtures. It is the byte that
//     most distinguishes this radio from the FT-891 exemplar (erratum
//     M-E3), and the one a copied driver would refuse.
//   - ONE MW OR MT OF A PMS SLOT, 100-117. No G MW vector names one, so the
//     numeric PMS form — this fleet's first — reaches the wire in this
//     package's tests and nowhere in the committed evidence.
//
// Neither is an assumption this driver makes: both are captures that would
// turn a manual reading into an observation. They are listed as a section
// rather than as register entries because the register is for values and
// behaviours this code encodes without manual evidence, and these two are
// manual-evidenced and merely unwitnessed.
//
// # The ASSUMED register
//
// TEN ENTRIES, covering the behaviours this DRIVER encodes that are NOT
// FT-991A-manual facts. Each is listed here once, marked ASSUMED at the
// point of use, and paired with the ONE Stage R or Stage W capture that
// lifts it. The captures are individual on purpose: one FT-991A session does
// not retire this register wholesale, it retires the assumptions its own
// frames actually speak to, and an entry whose capture was not taken stays
// here afterwards.
//
// THAT RULE IS WHY THERE ARE TEN AND NOT EIGHT. Entries 4, 5 and 6 were one
// bundled "TONE-NUMBER, DCS-CODE AND SCAN-SKIP UNREACHABILITY" until matrix
// erratum M-E10, whose whole argument is this paragraph applied to itself:
// the bundle named three lifting experiments, so under one entry to ONE
// capture it was three entries, and an operator who took the CTCSS-tone
// capture could retire no part of it. Matrix §4b labels the three 4a, 4b and
// 4c; they are 4, 5 and 6 here, which is one more reason the rule below is
// what it is.
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION. The numbering is for
// readability; every citation of this register — here, in this package's
// other files, and in its tests — names the entry's subject. A bare "entry
// 6" is correct only until an entry is inserted, and it then silently points
// at the wrong assumption rather than failing. The FT-991A DIALECT register
// carries the identical rule in terms.
//
// THE DIALECT'S ELEVEN ARE THE SAME ELEVEN THIS FILE CARRIES, and that is
// the spec's own arrangement (§The ASSUMED register): ONE register of record
// for the radio, written in core/cat/ft991a/doc.go and mirrored here, cited
// BY NAME at the sites in this package that depend on it and restated
// nowhere. register_test.go holds the two copies together mechanically,
// reading the dialect's own doc.go rather than trusting a transcription.
// ALL ELEVEN are reached by this package, which is a consequence of the
// register being shared rather than of this radio being better understood.
// One of the eleven is reached FORWARD rather than by code standing today —
// ROW 087 RADIO ID'S EXCLUSION, whose site is the settings descriptor task
// 12 has yet to write — and it is named here so the count is not read as
// eleven live dependence sites:
//
//   - MTPolicy.TagFill = ' ' — caps.go's TagLen (a width is evidenced, a
//     fill is not) and read.go, where the answer's tag field is trimmed
//     back off.
//   - THE COMBINED MT ANSWER'S EXACT LENGTH, 41 — read.go's mtSpec, which
//     derives the length from the dialect precisely because the exactness
//     is that entry's assumption and its recorded contingency is a 30..41
//     WINDOW.
//   - SlotSpace.NoneWire = "000" — read.go's BuildMTRead refusal site,
//     where the answer-only none form is grammatical per ParseSlot and
//     never a legal read target.
//   - THE cat.ModeUnset MEMBER OF THE MODE TABLE — caps.go's modeNames,
//     which excludes it, and read.go, which maps it through faithfully when
//     a radio sends it.
//   - ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz = 9990 —
//     caps.go's ClarMaxHz/ClarStepHz, which CONSULT the dialect rather than
//     re-transcribing the pair.
//   - THE CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII HYPHEN-MINUS 0x2D
//     ('-') — read.go, where a negative offset's sign is decoded. This
//     manual prints the token as TWO hyphens on all five blocks carrying
//     the legend (IF 784, MR 967, MT 1000, MW 1038, OI 1118) and evidence
//     leg G confirmed at 1800 dpi that the doubling is REAL rather than a
//     flattened dash — while P3 has five positions and a four-digit offset,
//     leaving exactly ONE for the direction. So the page prints two glyphs
//     into a one-byte field and which byte the wire wants is undecided
//     (matrix erratum M-E4).
//   - THE DCS STATES' SET ACCEPTANCE — caps.go's ctcssStates, which
//     publishes all five P8 values as writable, and (task 11) the write
//     path that may put '3' or '4' on the wire.
//   - ROW 087 RADIO ID'S EXCLUSION — the settings descriptor (task 12),
//     whose item count is the dialect's inventory: 152 items for a chart
//     printing 153 rows.
//   - FRAMING: 8 DATA BITS, NO PARITY, TWO STOP BITS — reached BY ABSENCE:
//     this package declares no driver.SerialFramingReporter, so every
//     session opens at core/transport's DefaultStopBits.
//     TestDriverDeclaresNoOptionalCapabilitiesItMustNot is the negative pin.
//   - DefaultBaud 38400 — caps.go's DefaultBaud, whose comment additionally
//     names menu 031 CAT RATE and EXCLUDES menu 029 232C RATE as the
//     RS-232C jack's rate (plan P11): both print the same four rates, so
//     nothing changes numerically, but a user sent to 029 sets the wrong
//     port's rate. internal/wiring's OpenRealSessionFor opens a real radio
//     at exactly this value and NO baud override exists in the CLI or the
//     GUI, so a wrong value leaves a real FT-991A reachable only through
//     its own menu.
//   - THE ACKNOWLEDGEMENT CONVENTIONS — read.go's reading of "?;" as a
//     rejection at all, and (task 11) the write path's reading of silence
//     as acceptance. This manual describes no ACK/NAK vocabulary beyond the
//     "?;" every Yaesu CAT manual in this repository shows for a rejected
//     command, and it never says whether an accepted Set answers at all.
//
// NO EQUIVALENT "ALL TEN ARE REACHED" CLAIM IS MADE FOR THE TEN BELOW, and
// CONTROL-LINE POLICY is why: it has no dependence site in this package at
// all. core/transport.OpenSerial drives RTS and DTR for every model, this
// driver has no code for either, and the entry is here because the
// assumption is made ON THIS RADIO'S BEHALF rather than because this package
// encodes it. Disclosed rather than quietly excused, so that a reader
// counting citations against entries finds the shortfall accounted for.
//
// Correcting a dialect entry is a change in core/cat/ft991a; correcting one
// of the ten below is a change here.
// NEITHER REGISTER MAY ABSORB THE OTHER, even though this milestone shares
// their content: the eleven are facts about the dialect and the codec, and
// these ten are facts about this driver's choreography and its capability
// values.
//
//  1. CONTROL-LINE POLICY: that driving RTS and DTR low unconditionally is
//     safe on THIS radio. core/transport.OpenSerial does exactly that
//     immediately after opening any port (safety obligation 4), for every
//     model, with no per-radio policy — and this radio has an opinion about
//     the line that the transport does not consult: menu "033 CAT RTS",
//     legend "0: DISABLE 1: ENABLE" (layout 563). What its factory setting
//     is, and whether a low RTS with it enabled stalls the CAT link, is
//     unknown. ("032 CAT TOT", 562, is the CAT time-out timer and is named
//     only so it is not mistaken for a handshake setting.) SIX menu rows
//     additionally route PTT or keying through RTS/DTR — 047 AM PTT SELECT
//     (583), 060 PC KEYING (596), 071 DATA PTT SELECT (607), 076 FM PKT PTT
//     SELECT (612), 096 RTTY SHIFT PORT (632) and 108 SSB PTT SELECT (644)
//     — so on a radio configured that way a driven control line is not
//     merely a handshake question. DRIVING BOTH LOW IS THE DIRECTION THAT
//     CANNOT KEY A TRANSMITTER, which is why the inherited policy is the
//     conservative one; that reasoning is a CHOICE and the safety claim
//     behind it is the assumption. This manual's DE-9 pin table gives the
//     CAT (RS-232C) jack's RTS and CTS an I/O column of "---" and a
//     Function column of "---" (rows at layout 30 and 31), which is the
//     closest it comes to a control-line statement and says NOTHING about
//     the USB path this driver's sessions actually open.
//     STAGE R LIFTS IT WITH: one ID exchange in each 033 CAT RTS state, ON
//     THE USB DEVICE, everything else held constant. If it answers in both,
//     the policy is safe here and this entry closes; if it answers in only
//     one, the transport needs a per-radio control-line policy and this
//     becomes a spec'd capability rather than an assumption. TAKE THIS
//     CAPTURE BEFORE CONCLUDING ANYTHING ABOUT THE DIALECT REGISTER'S
//     FRAMING ENTRY: silence at a known-correct baud is not evidence about
//     stop bits until this menu has been toggled, because a handshake
//     refusal and a framing mismatch present identically as nothing coming
//     back.
//
//  2. MinFreqHz 30 000 / MaxFreqHz 470 000 000 — THE FA/FB RANGE READ AS
//     THE MEMORY-STORABLE RANGE (caps.go). The NUMBERS are
//     manual-evidenced, and this radio is better evidenced here than the
//     FTdx10, whose manual carries no range statement at all: FA's P1
//     legend "000030000 - 470000000 (Hz)" (layout 699) and FB's identical
//     one (715) are the only frequency range this manual prints in a frame
//     legend, and a grep for either endpoint returns exactly those two
//     lines. WHAT IS ASSUMED IS THE STEP from "the VFO tuning domain" to
//     "the memory-storable domain": FA and FB are the VFOs, and the memory
//     blocks' own P2 legends say only "VFO-A Frequency (Hz)" (MR 966, MT
//     999, IF 783), "Frequency (Hz)" (MW 1037) or "VFO-B Frequency (Hz)"
//     (OI 1117) over a nine-digit field, which bounds the ENCODING and says
//     nothing about what a memory channel will store. A SECOND PRINTING OF
//     THE SAME ENDPOINTS IS DELIBERATELY NOT USED as evidence for these
//     fields: menu row "151 PRESET FREQUENCY" prints "00030000 ~ 47000000"
//     in units of 10 Hz (692), and it is the WIRES-X preset — what the
//     RADIO tunes, not what a memory channel stores. THIS IS THE FIRST
//     REGISTERED YAESU WITH VHF/UHF, more than eight times the FT-891's
//     56 MHz, so a fixture reused across the two models is in range here
//     and out of range there — and only one of those directions fails
//     loudly.
//     STAGE R LIFTS IT WITH: MT Sets at the claimed floor and ceiling and
//     just outside them, to a sacrificial channel, recording which are
//     accepted. The radio range-checks frequency on write (a real FT-710
//     demonstrably does), so acceptance and rejection are both informative.
//     The FT-991A operating manual's specifications page would settle the
//     RADIO's range but not the MEMORY's, and this project does not hold
//     that manual.
//
//  3. RequiredSlots {"001"} (caps.go): that memory channel 001 must never
//     be empty. THIS MANUAL STATES NO SUCH RULE ANYWHERE. The FT-710's M-01
//     is individually required because that radio keeps it populated — an
//     FT-710 hardware fact, not borrowed. Claiming it makes codeplug
//     validation refuse a candidate whose 001 is blank, which is the
//     conservative direction (refuse rather than write a state the radio
//     may not tolerate), but it IS a claim, kept as the fleet's convention
//     and labelled as convention rather than as this manual's statement.
//     RequiredSlots is the PER-SLOT mechanism and Bank.NoBlank the
//     PER-BANK one; this driver uses the per-slot one for its single
//     claimed channel and reaches for the per-bank one on neither bank.
//     STAGE R LIFTS IT WITH: observation of channel 001 on a real FT-991A —
//     whether the radio ships with it populated, and whether the front
//     panel will erase it at all. A radio that erases 001 happily drops
//     this from RequiredSlots.
//
//  4. TONE-NUMBER UNREACHABILITY: that no CAT command exposes a memory
//     channel's CTCSS/DCS tone number (caps.go's zero FieldSupport for
//     spec.FieldCTCSSTone, FieldToneTx and FieldToneRx; read.go's
//     codeplug.Unknown for the tone). WHAT IS STRUCTURAL AND
//     MANUAL-EVIDENCED: the combined MT record accounts for every one of
//     its 41 positions and none of them is a tone number (evidence leg G's
//     position-by-position field map, testdata/mt-vectors.golden), and P9
//     is documented "00: (Fixed)" on four blocks (MT 1012, MR 979, MW
//     1050, IF 797) and "0: (Fixed)" on the fifth (OI 1130, the mirror
//     printing defect the dialect's doc.go records); the number itself is
//     CN's, "P2 0: CTCSS 1: DCS" with "000 - 049: Tone Frequency Number"
//     (364-374) — LIVE STATE on a different command, which no frame this
//     codec builds writes together with a memory record. WHAT IS ASSUMED
//     is the step from that to "unreachable on this radio": nothing
//     verifies that some OTHER command in this manual could reach a memory
//     channel's stored tone number, and the FT-710's answer that none can
//     is that radio's hardware finding.
//     STAGE R LIFTS IT WITH: one channel set to a known CTCSS tone from the
//     front panel, then read over CAT — if any byte of the answer tracks
//     the tone number, the entry is refuted and the capability opens; if P9
//     reads "00" as documented, the entry closes as a confirmed protocol
//     limit.
//
//  5. DCS-CODE UNREACHABILITY: that no CAT command exposes a memory
//     channel's DCS code (caps.go's zero FieldSupport for
//     spec.FieldDTCSCode). IT IS A SEPARATE ENTRY FROM TONE-NUMBER
//     UNREACHABILITY BECAUSE ITS CAPTURE IS SEPARATE (matrix erratum
//     M-E10): a tone-number byte turning up says nothing about the DCS
//     code, so one capture cannot retire both, and this register's own rule
//     is one entry to ONE capture. The 41-position count is the structural
//     half here too, and the code is CN's, "000 - 103: DCS Code Number"
//     (364-374). THE DCS GAP IS THIS RADIO'S OWN AND IT IS USER-VISIBLE:
//     its P8 can SAY DCS (the five-state vocabulary, caps.go) and its
//     record cannot carry the code, so this programme can read and write
//     "this channel uses DCS encode+decode" while being unable to read,
//     write or even display WHICH code.
//     STAGE R LIFTS IT WITH: the TONE-NUMBER UNREACHABILITY experiment
//     repeated with a DCS CODE set from the front panel.
//
//  6. SCAN-SKIP UNREACHABILITY: that no CAT command exposes a memory
//     channel's front-panel skip flag (caps.go's zero FieldSupport for
//     spec.FieldScanSkip; read.go's codeplug.Unknown for the skip flag).
//     SEPARATE FROM THE OTHER TWO UNREACHABILITY ENTRIES ON THE SAME
//     REASONING (matrix erratum M-E10): neither a tone number nor a DCS
//     code turning up says anything about the skip flag. No skip position
//     exists anywhere in the 41-byte MT record or the 28-byte MR/MW block,
//     which is the structural half; WHAT IS ASSUMED is that no other
//     command reaches one either.
//     STAGE R LIFTS IT WITH: the same experiment again with the
//     front-panel skip flag set.
//
//  7. MT "?;" ON A MEMORY OR PMS SLOT MEANS THE SLOT IS EMPTY (read.go).
//     "?;" is the protocol's SINGLE unattributed NAK (core/cat/errors.go:
//     "returned for unknown commands, bad parameters, wrong radio state, an
//     empty memory slot, and anything else that goes wrong — the wire
//     protocol gives no way to tell these apart"), so reading "empty" out
//     of it is an interpretation. The FT-710's MR-on-empty behaviour is
//     that radio's hardware finding on a different command; the FT-991A's
//     MT-on-empty is not established at all. THIS DRIVER MAKES EXACTLY ONE
//     "?;" INTERPRETATION AND HAS EXACTLY ONE SITE FOR IT, which is the
//     whole benefit of having no discovery walk and no cross-check: it
//     never sends any other frame that could draw a "?;" from a slot, where
//     the FT-891 has four such readings. A TIMEOUT IS DELIBERATELY NOT
//     INTERPRETED — read.go surfaces the transport's own error under this
//     path's ordinary slot-naming wrap and NOT re-typed (errors.Is finds
//     transport.ErrTimeout, errors.As finds no driver type between them),
//     with no retry, so silence is never read as emptiness.
//     STAGE R LIFTS IT WITH: one MT read of a memory channel known-empty
//     from the front panel, and one of a known-populated channel, in the
//     same session.
//
//  8. THE MODE NIBBLE'S DOMAIN: that '1'-'9' and 'A'-'E' is the whole set
//     this radio ever puts in P6, with NO HOLE and NO 'F'. All five mode
//     legends print exactly those fourteen (MR 973-975, MT 1006-1008, MW
//     1044-1046, IF 789-791, OI 1124-1126), and the dialect transcribes
//     them — but a legend is a statement about what the chart draws, not a
//     guarantee about what the radio will ever send, and core/cat's parser
//     refuses any P6 byte outside the transcribed table. So a radio that
//     answered 'F' would fail every read of that channel rather than
//     mislabel it, which is the right direction and still an assumption.
//     THE FTdx10 FILLS 'F' WITH "DATA-FM-N" AND THE FT-891 PRINTS A HOLE AT
//     'A': neither shape may be read across, and 'E' is C4FM here where the
//     FTdx10's is PSK — one nibble, two REAL and different modes, which is
//     why core/cat's package-level Mode.String() fallback is ACTIVELY WRONG
//     for this radio and read.go renders through the session's dialect
//     instead.
//     STAGE R LIFTS IT WITH: one read of every occupied channel on a radio
//     whose owner has used its full mode range, plus an MD read in each
//     selectable mode. Any nibble outside the fourteen refutes the entry
//     and the parse must widen.
//
//  9. THE PRINTED-FIXED BYTES ARE ANSWERED AS PRINTED: that P9 really is
//     "00" and P11 really is '0' in every MR and MT answer, so core/cat's
//     strict parse never refuses a real answer. Both are printed constants
//     in this manual (P9 at MT 1012 and its siblings; P11 at MT 1015), and
//     the dialect encodes P11 as cat.P11Fixed, under which the parser
//     REQUIRES the byte — so if a real FT-991A ever answers something else,
//     EVERY READ OF THAT CHANNEL REFUSES rather than silently mis-reading a
//     flag. That is the safe direction and it is still an assumption. THE
//     FT-891's OWN REGISTER HAS THE SAME CLASS OF ENTRY ABOUT A DIFFERENT
//     BYTE — P5 there, because P5 is fixed there and LIVE here (erratum
//     M-E3) — and the two must not be conflated.
//     STAGE R LIFTS IT WITH: any MR or MT answer captured from any channel,
//     bytes 25-26 and 28 dumped as RAW VALUES rather than rendered. A
//     single non-conforming byte converts the parse posture from strict to
//     tolerant and is a finding, not a tweak.
//
//  10. A DCS-STATE CHANNEL'S CODE SURVIVES A REWRITE: that writing a memory
//     whose P8 is '3' or '4', with no CN code sent, leaves the radio's
//     stored code for that channel unchanged. This is the hazard of
//     shipping a state whose code is unreachable (DCS-CODE
//     UNREACHABILITY, cited by name because the numbering moved when
//     M-E10 split that entry out of a bundle): the frame
//     carries no code, so this driver cannot write one, and what the radio
//     does with the code it already holds is observable only on hardware.
//     It is a DIFFERENT question from the dialect register's THE DCS
//     STATES' SET ACCEPTANCE, which asks whether such a Set is accepted at
//     all; this asks what it costs when it is.
//     STAGE R LIFTS IT WITH: set a channel's DCS code from the front panel,
//     read it back on the panel, write that channel through this programme
//     with the same P8 state, then read the code on the panel again.
package ft991a

// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts480 implements the Kenwood TS-480 memory driver over core/kw's
// PC-command codec.
//
// # THE ROW IS BUILT AND IT IS NOT REGISTERED
//
// This package is complete, tested and faked to the same bar as the two
// TS-590 rows, and at this milestone's close it is ABSENT from
// internal/wiring's realDrivers and fakeDrivers tables and from
// SupportedModels(). That is decision 10 and plan P3, and it is a MECHANISM
// rather than a promise: there is no release-time override.
//
// The reason is A4, and it is a shippability question rather than a detail.
// The read-side rule that a zero record is an EMPTY CHANNEL rests on "If the
// selected channel is empty, P4 ~ P15 will be 0 and P16 will be blank"
// (590:1492-1493) — a TS-590SG sentence. THIS BOOK PRINTS NOTHING ABOUT AN
// EMPTY CHANNEL ANYWHERE. If A4 is false — if an MR of an unwritten channel
// answers "?;" rather than a zero record — then decision 5 makes that "?;" a
// definitive rejection, the session read fails whole, and A FRESH TS-480 OUT
// OF THE BOX CANNOT BE READ AT ALL. There is no fallback and no honest way to
// invent one: reading "?;" as "absent" is exactly what this milestone refuses
// to do.
//
// The gate, transcribed because every value in this package is conditional on
// it: the evidence artefact is internal/wiring/testdata/ts480-a4-observation.json,
// TRACKED IN GIT — deliberately not under docs/superpowers/ or any
// fixtures-private path, both of which are gitignored, so an artefact there
// would be absent in a fresh clone and the guard would read "no evidence" on a
// machine that had simply not been given the file. The guard is
// internal/wiring's, in three legs: absent → ABSENT (a branch, not a skip),
// present → PRESENT only after parsing it and asserting the bar, and a
// non-vacuity leg so that a file recording zero trials fails loudly rather
// than counting as absence. A4's lift is L-HW-3, hardware confirmation item 3.
//
// A future registration is TEN edits and not one, and the plan's P3 carries
// the list; nine of the ten fail loudly in the suite the moment the row
// registers without them, and the tenth at the byte-identity gate.
//
// # Provenance
//
// Every capability value comes from the A4-format capability matrix
// (docs/superpowers/kenwood-590sg-480-capability-matrix.md), which derives
// each from the TS-480 PC control command reference of 27/11/2003 — cited
// throughout as 480:LINE against that document's own layout extraction — and
// records its evidence status. Where a value is ASSUMED rather than printed,
// the assumption's register entry is cited BY NAME (A1…A27) at the code that
// depends on it; the authoritative register itself lives in core/kw/doc.go,
// and no entry is re-registered here.
//
// The codec, the frame grammars and the outbound gate are core/kw's; this
// row's layout value and its 61-row menu inventory are core/kw/ts480's. This
// package holds the neutral capability table, the identity probe, the read
// choreography, the write path that refuses everything, and the one settings
// descriptor with the menu read behind it.
//
// # The Kenwood DRIVER register
//
// Two claims a Kenwood driver makes have no home in the design's ASSUMED
// register, because they arise from the capability model and from session
// setup rather than from a sentence in a book (matrix §6). They carry K-
// numbers, and the prefix records where a correction lands: CORRECTING AN
// A-NUMBER IS A DESIGN CHANGE, CORRECTING A K-NUMBER IS A DRIVER-PACKAGE
// CHANGE, and no task may quietly move one for the other. NEITHER REGISTER MAY
// ABSORB THE OTHER.
//
//	K-D1  THE TS-480's P7 SEMANTICS ARE THE TS-590's, AND THIS ROW IS THE ONLY
//	      ONE THAT CLAIMS IT. This book prints P7's legend as bare labels —
//	      "0: OFF, 1: TONE, 2: CTCSS" (480:964, and identically on MR at
//	      480:922, and again on IF at 480:723) — and never says which
//	      DIRECTION each value acts in. This package nonetheless publishes
//	      "1: TONE" as spec.ToneModeCTCSS (transmits a tone, requires none on
//	      receive) and "2: CTCSS" as spec.ToneModeCTCSSRxSquelch (requires a
//	      received tone, transmits none), which is the TS-590's reading.
//
//	      THE ONLY SENTENCE THAT SETTLES IT IS IN THE OTHER BOOK: IF P14's
//	      note, "When Tone is ON, this number is the Tone frequency. When
//	      CTCSS is ON, this number is the CTCSS frequency. When Cross Tone is
//	      ON, the transceiver transmits on the Tone frequency and receives on
//	      the CTCSS frequency" (590:1167-1171). By decision 13 that sentence
//	      does not reach this row, so the semantics here are a CLAIM and not a
//	      reading. core/driver/ts590's own doc.go names K-D1 and points here
//	      precisely so that a reader who meets K-D2 first does not conclude
//	      the driver register has one row.
//
//	      SCOPE: the TS-480 row ONLY.
//	      LIFT: L-DOC-5 or L-HW-20. L-DOC-5 is the TS-480 INSTRUCTION MANUAL's
//	      tone chapter — the same fetch Q1 and Q2 need, so it costs nothing
//	      extra if that fetch happens. L-HW-20 is a wire trial: write a
//	      channel with P7='1', recall it, key the transmitter into a dummy
//	      load and observe whether a tone is transmitted; then P7='2' and
//	      observe whether receive is muted until a tone is present. THE WIRE
//	      LIFT IS NOT AVAILABLE UNTIL A4 HAS REGISTERED THIS ROW, and it needs
//	      a write, which A22 also refuses — so L-DOC-5 is the reachable one.
//
//	      WHAT IT COSTS TODAY: nothing a user can see. tone_tx and tone_rx are
//	      Unsupported on this row for a different reason (Q2: neither printed
//	      chart is in this book), so no tone VALUE is published in either
//	      direction and the mode's direction reaches no frame this milestone
//	      builds. It is registered because the neutral vocabulary carries the
//	      direction whether or not anything reads it, and an unregistered
//	      claim is one nobody would think to lift.
//
//	K-D2  RTS AND DTR AT OPEN ARE WHATEVER THE TRANSPORT'S DEFAULT LEAVES
//	      THEM, AND NEITHER RADIO REQUIRES OTHERWISE. This driver sets no
//	      control line: it hands transport.NewEngineWith the port it was given
//	      and changes nothing about it. NEITHER BOOK STATES A REST STATE for
//	      either line. This radio's own RTS/CTS sentence — "The required
//	      control is achieved by using the RTS and CTS lines" (480:36-40) — is
//	      about the flow-control PROTOCOL, not about a line's level at open,
//	      and the 590 pair's "Flow Control — Hardware flow control is
//	      possible" (590:60) is weaker still.
//
//	      IT MUST NOT BE INHERITED FROM THE YAESU SIDE: the registered Yaesu
//	      drivers drive RTS and DTR low at open, and doing the same here
//	      because it is what a neighbouring package does would be a claim
//	      about a radio nobody has connected.
//
//	      SCOPE: all three registry rows, PER (row, path) — the same entry
//	      core/driver/ts590/doc.go carries, stated in both packages because
//	      each driver is where its own half is corrected.
//	      LIFT: L-HW-21, one session opened against each of the design's five
//	      (row, path) legs with the control lines logged — the TS-590S's USB-B
//	      port and its RS-232C connector, the TS-590SG's USB-B port and its
//	      RS-232C connector, and THE TS-480's 9-pin D-sub, which is the only
//	      path this radio has (480:19-25). EACH LEG LIFTS ITS OWN (row, path);
//	      a row is lifted when all of its paths are. The legs are A25's, and
//	      for A25's reason: a USB-B leg's behaviour is partly the
//	      Kenwood-supplied virtual COM driver's, which the RS-232C leg does
//	      not share — and this row, having no USB path at all, is lifted by a
//	      single leg.
//
// # What makes this row different from the TS-590 pair
//
// The two radios share a 50-byte GRID and a name at bytes 42-49, and diverge
// on what several of the fields mean. Each divergence below reaches a
// capability value or a runtime branch in this package, and each is named here
// so that a reader can check this package against the list rather than against
// their memory (matrix §5, and core/kw/ts480/doc.go for the codec half):
//
//  1. SIXTEEN PRINTED-FIXED BYTES IN SIX RUNS, against the 590 pair's thirteen
//     in three — this row's own P2 (480:953), P11 (480:973) and P15 (480:982)
//     on top of the P10/P12/P13 both books print. Every one is REQUIRED ON
//     PARSE (decision 7), and on this row that strictness is a CHOICE rather
//     than a deduction: the book's own general note permits a SET to fill an
//     inapplicable parameter with "any character except the ASCII control
//     codes (00 to 1Fh) and the terminator (;)" (480:108-111), so a radio
//     answering a hard-wired byte with something else would not necessarily be
//     faulty. That choice is A24, its lift is a dozen real reads (L-HW-18), and
//     a single counter-example turns the rule from "required" into "accepted
//     and normalised".
//
//  2. NO FV, AND TY IN ITS PLACE. This document prints no FV command anywhere,
//     no revision number, no part code and no firmware statement — erratum
//     E15 — so kw.Layout.BuildFVRead refuses a Book480 layout outright and the
//     probe's third frame is "TY;", a HARDWARE VARIANT read (480:1621-1634).
//     The two are read with OPPOSITE failure policies and the asymmetry is the
//     design's own rule: an unexpected TY P2 REFUSES the session, because P2 is
//     a printed four-value legend and a fifth value means an unread variant
//     whose capability table this programme would be inventing; an unparseable
//     FV on the 590 pair degrades instead, because its grammar is A13, assumed
//     from a single worked example. REFUSE WHERE THE DOCUMENT IS COMPLETE AND
//     WE ARE OUTSIDE IT; DEGRADE WHERE THE DOCUMENT IS THIN.
//
//  3. ONE FLAT MEM BANK OF TWO-DIGIT SLOTS, AND NO SCAN BANK. MC's P1 is
//     "Always 0 for the TS-480 (Memory bank number)." (480:827) and its P2
//     "00 ~ 99: Channel number" (480:830), so the canonical slot string here is
//     two digits where the 590 pair's is three — the one place this driver may
//     not reuse kw.Slot.String's own "%03d" rendering (caps.go's slotID).
//     Channels 90-99 do answer a second frame through the P1 overload
//     (480:943-944, 480:986-987), but decision 15 rules them ORDINARY MEMORIES
//     rather than a scan class: a slot string is unique across a codeplug and a
//     Bank.Fields map is per bank, so a scan bank would either duplicate ten
//     slot identities or take ten ordinary memories from the owner. The
//     consequence is published rather than papered over — the P1=1 half of
//     90-99 is UNREACHABLE through this programme, and FieldTxFrequency is
//     Unsupported on the whole row (M-E2).
//
//  4. FOUR PRINTED VARIANTS, ONE REGISTRY ROW. TY's P2 discriminates the
//     TS-480HX (200 W), the TS-480SAT (100 W + AT) and two Japanese types
//     (480:1626-1629), and the neutral memory model expresses none of the
//     difference — same ID, same 50-byte record, same 00-99 space. The variant
//     is reported through Session.Variant for the probe note and never reaches
//     the registry key (decision 4, §1.1).
//
//  5. E13 AND E15, THE TWO ERRATA THAT REACH THIS PACKAGE'S BEHAVIOUR. E13:
//     the two books give DIFFERENT causes for the same "O;" token — "Receive
//     data was sent but processing was not completed." here (480:143-144)
//     against a receive-buffer overrun on the 590 pair — which is why a Book is
//     a semantic in this codec and why wireFailure builds its typed errors from
//     the LAYOUT's book rather than from a package default. E15 is the missing
//     firmware version, above.
//
//  6. A22 AND Q2 — THE WRITE PATH THAT REFUSES EVERYTHING. Bytes 39-40 are the
//     tuning step here (480:979) where the 590 pair carry an FM Normal/Narrow
//     flag, ST's legend is mode-conditional over two ranges (480:1494-1500),
//     and no flat capability vocabulary is truthful over two legends — so
//     FieldTuningStep is published Unsupported, the source channel never
//     retains the raw index, and EVERY TS-480 CHANNEL WRITE IS REFUSED (A22,
//     decision 12). Q2 — that neither tone chart is in this book — rides inside
//     that refusal as a SUBSIDIARY cause rather than as a rung of its own,
//     because a rung below A22 could never execute. write.go carries the whole
//     relationship at the site.
//
// # What this package deliberately does NOT do
//
//   - NO DISCOVERY, on two independent grounds either of which would suffice
//     (decision 5, matrix §3.4): the book says the NAK is unreliable
//     ("Occasionally this message may not appear due to microprocessor
//     transients in the transceiver", 480:136-138), so a probe's silence
//     carries no information at all; and the slot space is fully printed
//     (480:955), so a probe would ask a question the book answers. The bank is
//     static and no bank is appended at Open.
//   - NO MC READ, and no MC frame of any kind on the read path (P13).
//     Recalling a channel changes the radio's operating state.
//   - NO MW OF ANY WIDTH, on any profile, under any consent (A22).
//   - NO ERASE, no transceive-set and no auto-baud, which are this
//     repository's standing rules and are additionally what the codec's own
//     outbound gate enforces. On this row the no-erase rule needs no exception
//     at all: the radio has no erase route to refuse (480:1205).
//   - NO CHIRP-SPECIFIC WORK (P16). chirpModeMap's CW → "CW-U", CWR → "CW-L"
//     and RTTY → "RTTY-U" name no mode this row publishes, so those rows block
//     with ActionUnsupported exactly as they do on the eleven Icom models and
//     the FT-891. Kenwood spells RTTY "FSK", which adds a third spelling family
//     to a deferred fleet-wide question; it is recorded and acted on nowhere.
package ts480

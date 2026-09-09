// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts990 implements the Kenwood TS-990S memory driver over
// core/kw/ma's MA0 PC-command codec.
//
// ONE REGISTRY ROW, ONE PACKAGE, AND NO Row ARGUMENT. core/driver/ts590
// serves two rows from one package because the TS-590S and the TS-590SG share
// a book, a 50-byte grid, a mode legend and both tone charts, and differ on
// four things. This radio and the TS-890S share a FAMILY and an ENVELOPE and
// almost nothing below it: seventeen divergences, of which the frame length
// (57 fixed against 40-50 floating), the parameter count (eighteen against
// thirteen), the mode legend (twenty-four values against sixteen), the number
// of tone tuples in one record (two against one), the Main/Sub band pointer
// and the channel-type byte are each on their own a sufficient reason for two
// packages (matrix §4, §5; spec decision 2). NOT ONE VALUE IN THIS PACKAGE IS
// COPIED FROM core/driver/ts890: where the two rows agree the agreement is a
// coincidence of two readings of two books.
//
// READ-ONLY IN PRACTICE AT THIS MILESTONE. writeTrialsComplete is false: no
// TS-990S has ever been written to — or asked anything at all — by this
// project, so every write is behind the unverified-write consent route and
// every pre-wire refusal still fires ahead of it. Nothing in this package
// claims hardware verification.
//
// # Provenance
//
// Every capability value comes from the A4-format capability matrix
// (docs/superpowers/kenwood-890s-990s-capability-matrix.md), which derives
// each from the "TS-990S PC Control Command Reference Guide" — cited
// throughout as 990:LINE against that document's own layout extraction — and
// records its evidence status. Where a value is ASSUMED rather than printed,
// the assumption's register entry is cited BY NAME (A1…A22) at the code that
// depends on it; the authoritative register itself lives in core/kw/ma/doc.go
// alongside the errata schedule E1–E19, and no entry is re-registered here.
//
// The codec, the frame grammars and the outbound gate are core/kw/ma's; this
// row's layout values are ma.Layout990()'s. This package holds the neutral
// capability table, the identity probe, the read choreography, the write
// refusal ladder and the settings descriptor with the menu read behind it.
//
// # The Kenwood MA DRIVER register — K-D1 and K-D2, for THIS ROW
//
// Two claims this driver makes have no home in the design's ASSUMED register,
// because they arise from the capability model and from session setup rather
// than from a sentence in the book (matrix §6). They carry K- numbers, and
// the prefix records where a correction lands: CORRECTING AN A-NUMBER IS A
// DESIGN CHANGE, CORRECTING A K-NUMBER IS A DRIVER-PACKAGE CHANGE, and no
// task may quietly move one for the other. NEITHER REGISTER MAY ABSORB THE
// OTHER.
//
// BOTH ENTRIES ARE WRITTEN TWICE IN THIS MILESTONE, ONCE PER DRIVER PACKAGE,
// EACH WITH ITS OWN ROW'S LIFT (plan P17, matrix §6). A single shared entry
// would be exactly the sibling inheritance the per-row rule exists to
// prevent: an assumption is lifted by an observation, and an observation
// comes from ONE radio. The entries below claim the TS-990S and nothing else,
// and the sibling package's identically-numbered entries claim the TS-890S
// and nothing else.
//
//	K-D1  THE TONE-MODE SEMANTICS ARE THE TS-590'S, AND THIS BOOK DOES NOT
//	      SAY SO. MA0 P6 prints four values for frequency 1 — "0: FM Tone
//	      function OFF / 1: Tone / 2: CTCSS / 3: Cross Tone" (990:2915-2919)
//	      — corroborated by the standalone TO command's P2 (990:4978-4989),
//	      and P12 prints the same four for frequency 2 (990:2935-2939). THE
//	      VALUES ARE PRINTED; WHAT THEY MEAN IS NOT. This package maps 1 onto
//	      spec.ToneModeCTCSS (transmits a tone, requires none on receive), 2
//	      onto spec.ToneModeCTCSSRxSquelch (requires a received tone,
//	      transmits none) and 3 onto spec.ToneModeCross, and — with them —
//	      the TN index onto FieldToneTx and the CN index onto FieldToneRx.
//
//	      THE WHOLE EVIDENCE FOR THAT IS ONE SENTENCE IN A DIFFERENT RADIO'S
//	      BOOK: the TS-590's IF P14 note, "When Tone is ON, this number is
//	      the Tone frequency. When CTCSS is ON, this number is the CTCSS
//	      frequency. When Cross Tone is ON, the transceiver transmits on the
//	      Tone frequency and receives on the CTCSS frequency"
//	      (590:1167-1171). THIS BOOK CONTAINS NEITHER THAT SENTENCE NOR AN IF
//	      COMMAND BLOCK AT ALL — searched for "transmits on", "receives on",
//	      "is the Tone frequency" and "is the CTCSS frequency", zero hits
//	      each, and for a command block whose mnemonic is IF, zero hits. This
//	      is matrix erratum M-E2, which the design did not notice.
//
//	      WHY THE ASSUMPTION IS TAKEN RATHER THAN REFUSED: publishing OFF
//	      alone until this lifts would refuse to READ every channel carrying
//	      a tone, and would make a CTCSS repeater memory unreadable on a
//	      radio this programme otherwise reads whole. Publishing four with
//	      the semantics recorded ASSUMED is the honest position — the VALUES
//	      round-trip on the evidence of the charts, and only their meaning is
//	      borrowed. spec.ToneModeCross additionally requires a model that
//	      expresses the fields the combination needs, and this row does: an
//	      independent tone index and an independent CTCSS index side by side
//	      in one record.
//
//	      SCOPE: the TS-990S row. It says nothing about the TS-890S, whose
//	      own package carries its own K-D1.
//	      LIFT: the TS-990S INSTRUCTION MANUAL's tone chapter, or a wire
//	      trial on a TS-990S that sets P6 to 1 and observes whether the radio
//	      transmits a tone and whether its squelch opens. IT LIFTS THIS ROW
//	      ONLY: two radios' firmware are two radios' firmware.
//
//	K-D2  RTS AND DTR AT OPEN ARE WHATEVER THE TRANSPORT'S DEFAULT LEAVES
//	      THEM, AND THIS RADIO REQUIRES NOTHING OTHERWISE. This driver sets
//	      no control line: it hands transport.NewEngineWith the port it was
//	      given and changes nothing about it. THIS BOOK STATES NO REST STATE
//	      for either line. What it does print is "Flow Control — Hardware
//	      flow control is possible" (990:21) — possible, not required — and
//	      that row is about the flow-control PROTOCOL, not about a line's
//	      level at open.
//
//	      IT MUST NOT BE INHERITED FROM THE YAESU SIDE: the registered Yaesu
//	      drivers drive RTS and DTR low at open, and doing the same here
//	      because it is what a neighbouring package does would be a claim
//	      about a radio nobody has connected.
//
//	      SCOPE: the TS-990S row, PER (row, path). This radio has three
//	      connectors and this programme drives one of the two serial paths at
//	      a time — a COM/RS-232C connector (990:27-31) and a USB-B virtual
//	      COM port needing a Kenwood-supplied pre-installed driver
//	      (990:33-40); the LAN connector is out of scope.
//	      LIFT: one session opened against EACH of this row's two serial
//	      paths with the control lines logged. A USB-B leg's behaviour is
//	      partly the Kenwood-supplied virtual COM driver's, which the RS-232C
//	      leg does not share, so one leg does not lift the other. THE ROW IS
//	      LIFTED WHEN BOTH OF ITS PATHS ARE.
//
// # The write ladder (task 14)
//
// Recorded here because this file is where A14 and A1 are cited for this row,
// and because both facts are settled by the CODEC rather than by the driver —
// so a reader of write.go alone would have to re-derive them.
//
// THE NAME WINDOW PADS ON BUILD AND TRIMS ON PARSE (A1), so a trailing space
// is silently absorbed. This row's P18 is a FIXED ten-byte window at bytes
// 47-56 with the terminator nailed to 57 (990:2955-2956, 990:2938), and
// core/kw/ma pads a short name with ASCII space on build and right-trims on
// parse. A CHIRP-imported name ending in a space — core/csvio/chirp.go:301
// leaves one untrimmed — therefore round-trips through this row's codec with
// the space absorbed by the pad/trim pair, and NO SPECIAL CASE IS NEEDED IN
// THE WRITE LADDER. The sibling row reaches the same place by the opposite
// mechanism: its terminator floats, so "AB ;" and "AB;" are distinct frames
// and its codec carries P13 verbatim in both directions. The write-back audit
// this would need is ALREADY HERE — core/clone/execute.go compares
// want.Tag != got.Tag — and it needs no code of its own for this case: a
// trailing-space tag reads back plain, so the comparison reports a legible
// FieldTag mismatch rather than staying silent. driver.WriteResult carries no
// per-channel detail to say WHY the tags differ, but the audit does not go
// quiet — it names the field.
//
// THE CODEC NORMALISES THE CLASS BYTE AND THE DUAL/SECTION-DEFINED REFUSAL IS
// THEREFORE THE DRIVER'S (A14). MA0 P2 is the channel type — "0: Single / 1:
// Dual / 2: Section defined" — printed with "this parameter is ignored. Enter
// a dummy value" (990:2897-2903), which names no value. core/kw/ma chooses
// '0' and emits it on EVERY build; that is A14, the milestone's ONLY defaulted
// byte, lift L-HW-11 (Set with P2 = '0' and again with P2 = '9', reading back
// each time), and it is why the defaulted-byte list is one item long where
// pair 1's was thirteen and sixteen. The codec does not consult the Class it
// parsed and does not refuse a channel read back as Dual or Section defined.
// SO THE OBLIGATION SPEC DECISION 9 PLACES ON THIS PAIR — quote what you
// refuse, by P-number and both values — BELONGS TO TASK 14'S WRITE PATH for a
// Dual or Section-defined write target, and this task records only the
// read-side state: Class is parsed, kept on the Record, and published nowhere.
//
// # What this package deliberately does NOT do
//
//   - NO DISCOVERY, on two independent grounds either of which would suffice
//     (matrix §3.4): this book says the NAK is unreliable ("Occasionally,
//     this message may not appear due to microprocessor transients in the
//     transceiver", 990:114-116), so a probe's silence carries no information
//     at all; and the slot space is fully printed (990:2893-2896), so a probe
//     would ask a question the book answers. Every bank is static and no bank
//     is appended at Open.
//   - NO MN, IN EITHER DIRECTION, and no discovery frame of any kind on the
//     read path (plan P12). An MN Set changes the radio's selected memory
//     channel, which is an operating-state change this programme does not
//     make — and the consequence is that A18 carries every read in the
//     milestone rather than merely the first: with no MN builder this driver
//     could not select a channel even if it had to, so IF A18 IS FALSE NO
//     READ ON THIS ROW WORKS AT ALL.
//   - NO SUB BAND. This book puts a Main/Sub pointer on five commands — MN,
//     MV, OM P1, TN P1 and CN P1 — and this driver reads and writes the MAIN
//     band only. IT IS NOT A BANK and that is the structural point: no byte
//     of MA0 names a band (990:2891-2903 is the whole parameter head), so the
//     record is not multiplied by two and there is nothing for a second bank
//     to hold. The dual side enters the record only as frequency 2 plus the
//     P16 dual-reception flag, which matrix §2.7 refuses to rewrite.
//   - NO ERASE, no transceive-set and no auto-baud, which are this
//     repository's standing rules and are additionally what the codec's own
//     outbound gate enforces. This radio prints a dedicated deletion command,
//     MA5 (990:3042-3047) — stronger evidence than pair 1's ambiguous MW side
//     effect, and the standing rule declines it anyway.
package ts990

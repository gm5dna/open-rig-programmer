// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts890 implements the Kenwood TS-890S memory driver over
// core/kw/ma's MA0 codec.
//
// ONE REGISTRY ROW, ONE PACKAGE, AND NO Row ARGUMENT. The TS-890S and the
// TS-990S have separate books, separate MA0 grids — thirteen parameters
// against eighteen — and separate mode legends, so unlike the 590 pair there
// is nothing for one package to share between them. core/driver/ts990 is the
// sibling, and NOT ONE VALUE IN THIS PACKAGE MAY BE COPIED FROM IT: every
// capability here is a transcription from this radio's own book, cited at the
// site that carries it.
//
// READ-ONLY IN PRACTICE AT THIS MILESTONE. writeTrialsComplete is false: no
// TS-890S has ever been written to — or asked anything at all — by this
// project, so every write is behind the unverified-write consent route and
// every pre-wire refusal still fires ahead of it. Nothing in this package
// claims hardware verification.
//
// # Provenance
//
// Every capability value comes from the A4-format capability matrix
// (docs/superpowers/kenwood-890s-990s-capability-matrix.md), which derives
// each from the "TS-890S PC Control Command Reference Guide" — cited
// throughout as 890:LINE against that document's own layout extraction — and
// records its evidence status. Where a value is ASSUMED rather than printed,
// the assumption's register entry is cited BY NAME (A1…A22) at the code that
// depends on it; the authoritative register itself lives in core/kw/ma/doc.go
// and no entry is re-registered here.
//
// The codec, the frame grammars and the outbound gate are core/kw/ma's; the
// envelope beneath them is core/kw's. This package holds the neutral
// capability table, the identity probe, the read choreography, the write
// refusal ladder (Stage 2 task 12) and the settings descriptor.
//
// # The Kenwood MA DRIVER register — THIS ROW ONLY
//
// Two claims this driver makes have no home in the design's ASSUMED register,
// because they arise from the capability model and from session setup rather
// than from a sentence in the book (matrix §6). They carry K- numbers, and
// the prefix records where a correction lands: CORRECTING AN A-NUMBER IS A
// DESIGN CHANGE, CORRECTING A K-NUMBER IS A DRIVER-PACKAGE CHANGE, and no
// task may quietly move one for the other. NEITHER REGISTER MAY ABSORB THE
// OTHER.
//
// BOTH ENTRIES ARE WRITTEN TWICE, ONCE PER PACKAGE, EACH WITH ITS OWN ROW'S
// LIFT (matrix §6, plan P17). A single shared entry would be exactly the
// sibling inheritance the per-row rule exists to prevent: two radios' firmware
// are two radios' firmware, and an observation made on one lifts one row.
//
//	K-D1  THE TS-890S'S TONE-MODE SEMANTICS ARE THE TS-590'S, AND THAT IS AN
//	      ASSUMPTION THIS DRIVER MAKES FROM ANOTHER RADIO'S BOOK (matrix
//	      §1.19, M-E2). The four VALUES are printed here — MA0 P5 "0: OFF /
//	      1: Tone / 2: CTCSS / 3: Cross Tone" (890:3180-3185), corroborated by
//	      the standalone TO command's identical legend (890:5170-5178). What
//	      is NOT printed is what they MEAN. Pair 1's mapping rested on one
//	      sentence in the 590 book — IF P14's "When Tone is ON, this number is
//	      the Tone frequency. When CTCSS is ON, this number is the CTCSS
//	      frequency. When Cross Tone is ON, the transceiver transmits on the
//	      Tone frequency and receives on the CTCSS frequency" — AND THIS BOOK
//	      CONTAINS NEITHER THAT SENTENCE NOR AN IF COMMAND BLOCK AT ALL. So
//	      three things are borrowed: that spec.ToneModeCTCSS (transmits a
//	      tone, requires none on receive) is value 1, that
//	      spec.ToneModeCTCSSRxSquelch (requires a received tone, transmits
//	      none) is value 2, and — with them — that P6's TN index is
//	      spec.FieldToneTx and P7's CN index is spec.FieldToneRx
//	      (890:3186-3190).
//
//	      WHY IT IS PUBLISHED ANYWAY. The alternative is to publish OFF alone,
//	      which would refuse to READ every channel carrying a tone and make a
//	      CTCSS repeater memory unreadable on a radio this programme otherwise
//	      reads whole. The values round-trip on the charts' own evidence; only
//	      their meaning is borrowed, and this entry is where that is recorded.
//
//	      SCOPE: the TS-890S row alone.
//	      LIFT: this radio's INSTRUCTION MANUAL's tone chapter, or a wire
//	      trial that sets the tone type to 1 and observes whether the radio
//	      transmits a tone and whether its squelch opens. IT LIFTS THIS ROW
//	      ONLY — a TS-990S trial says nothing about this one.
//
//	K-D2  RTS AND DTR AT OPEN ARE WHATEVER THE TRANSPORT'S DEFAULT LEAVES
//	      THEM, AND THIS RADIO DOES NOT REQUIRE OTHERWISE (matrix §3.2). This
//	      driver sets no control line: it hands transport.NewEngineWith the
//	      port it was given and changes nothing about it. THE BOOK STATES NO
//	      REST STATE for either line. Its transport table says only "Flow
//	      Control — Hardware flow control is possible" (890:19) — possible,
//	      not required — which is a statement about the PROTOCOL and not about
//	      a line's level at open.
//
//	      IT MUST NOT BE INHERITED FROM THE YAESU SIDE: the registered Yaesu
//	      drivers drive RTS and DTR low at open, and doing the same here
//	      because it is what a neighbouring package does would be a claim
//	      about a radio nobody has connected.
//
//	      SCOPE: the TS-890S row, PER PATH.
//	      LIFT: one session opened against each of this radio's two serial
//	      paths — the RS-232C COM connector (890:24-29) and the USB-B port
//	      (890:31-42), which needs a Kenwood-supplied virtual COM driver —
//	      with the control lines logged. TWO LEGS, and the row is lifted when
//	      both are: a USB-B leg's behaviour is partly that supplied driver's,
//	      which the RS-232C leg does not share.
//
// # What this package deliberately does NOT do
//
//   - NO DISCOVERY, on two independent grounds either of which would suffice
//     (matrix §3.4): the book says the NAK is unreliable, so a probe's silence
//     carries no information at all; and the slot space is fully printed —
//     MA0 P1 is "000 ~ 119" (890:3167) — so a probe would ask a question the
//     book answers. Every bank is static and no bank is appended at Open.
//   - NO MN FRAME OF ANY KIND, in either direction (spec decisions 5 and 15).
//     An MN Set changes which memory channel the radio has selected, which is
//     an operating-state change this programme does not make; and MA0's own
//     Read carries its channel number (890:3184-3186), so nothing has to
//     select one. THAT AN MA0 READ NEEDS NO PRECEDING MN IS A18, the
//     load-bearing read assumption of this milestone, cited in read.go where
//     it bites.
//   - NO ERASE, over a PRINTED erase command (M-E4). This book carries a
//     dedicated MA5 "Memory Channel (Channel Deletion)" (890:3305, 890:3311)
//     — stronger evidence than pair 1's ambiguous MW side effect — and the
//     standing no-erase rule declines to build it anyway. spec.FieldErase is
//     the zero FieldSupport in consequence.
//   - NO TRANSCEIVE-SET AND NO AUTO-BAUD, which are this repository's
//     standing rules and are additionally what core/kw/ma's outbound gate
//     enforces.
//   - NO SLOT OUTSIDE 000-099 IS PUBLISHED. The Programmable VFO channels
//     100-109 and the E channels 110-119 are printed (890:3168-3169) and
//     published in no bank of this row: what an MA0 read of one ANSWERS is
//     nowhere printed (A9, A10). See caps.go.
package ts890

// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts590 implements the Kenwood TS-590S and TS-590SG memory driver
// over core/kw's PC-command codec.
//
// TWO REGISTRY ROWS, ONE PACKAGE, AND A REQUIRED Row ARGUMENT. Kenwood prints
// both radios in one PC Control Command Reference Guide, and that is a
// property of the BOOK rather than of the radios: they have different
// firmware, different menu domains and a byte whose liveness differs between
// them. New therefore takes a Row with no usable zero, so no caller can drive
// an SG with an S's row by omission.
//
// READ-ONLY IN PRACTICE AT THIS MILESTONE. writeTrialsComplete is false on
// both rows: no TS-590 has ever been written to — or asked anything at all —
// by this project, so every write is behind the unverified-write consent
// route and every pre-wire refusal still fires ahead of it. Nothing in this
// package claims hardware verification.
//
// # Provenance
//
// Every capability value comes from the A4-format capability matrix
// (docs/superpowers/kenwood-590sg-480-capability-matrix.md), which derives
// each from the "TS-590S/TS-590SG PC Control Command Reference Guide" — cited
// throughout as 590:LINE against that document's own layout extraction — and
// records its evidence status. Where a value is ASSUMED rather than printed,
// the assumption's register entry is cited BY NAME (A1…A27) at the code that
// depends on it; the authoritative register itself lives in core/kw/doc.go,
// and no entry is re-registered here.
//
// The codec, the frame grammars and the outbound gate are core/kw's; the two
// rows' layout values are core/kw/ts590's. This package holds the neutral
// capability table, the identity probe, the read choreography and — from
// Stage 2 task 12 — the write refusal ladder.
//
// # The Kenwood DRIVER register
//
// Two claims this driver makes have no home in the design's ASSUMED register,
// because they arise from the capability model and from session setup rather
// than from a sentence in the book (matrix §6). They carry K- numbers, and
// the prefix records where a correction lands: CORRECTING AN A-NUMBER IS A
// DESIGN CHANGE, CORRECTING A K-NUMBER IS A DRIVER-PACKAGE CHANGE, and no
// task may quietly move one for the other. NEITHER REGISTER MAY ABSORB THE
// OTHER.
//
//	K-D1  THE TS-480 ROW'S ENTRY, AND IT IS NOT HERE. K-D1 records that the
//	      TS-480's P7 semantics are the TS-590's, and it lands in
//	      core/driver/ts480/doc.go, on that row's own terms. It is named here
//	      only so that a reader who meets K-D2 first does not conclude the
//	      driver register has one row. This package's rows need no such
//	      entry: their P7 semantics are printed, in IF P14's note at
//	      590:1167-1171.
//
//	K-D2  RTS AND DTR AT OPEN ARE WHATEVER THE TRANSPORT'S DEFAULT LEAVES
//	      THEM, AND NEITHER RADIO REQUIRES OTHERWISE. This driver sets no
//	      control line: it hands transport.NewEngineWith the port it was
//	      given and changes nothing about it. NEITHER BOOK STATES A REST
//	      STATE for either line. The 590 book says only "Flow Control —
//	      Hardware flow control is possible" (590:60) — possible, not
//	      required — over two paths, a COM/RS-232C connector and a USB-B
//	      virtual COM port needing a Kenwood-supplied driver (590:35-44). The
//	      TS-480's RTS/CTS sentence (480:37-40) is about the flow-control
//	      protocol, not about a line's level at open, and this entry covers
//	      that row too.
//
//	      IT MUST NOT BE INHERITED FROM THE YAESU SIDE: the registered Yaesu
//	      drivers drive RTS and DTR low at open, and doing the same here
//	      because it is what the neighbouring package does would be a claim
//	      about a radio nobody has connected.
//
//	      SCOPE: all three registry rows, PER (row, path).
//	      LIFT: one session opened against each of the design's five (row,
//	      path) legs with the control lines logged — the TS-590S's USB-B port
//	      and its RS-232C connector, the TS-590SG's USB-B port and its
//	      RS-232C connector, and the TS-480's 9-pin D-sub, which is the only
//	      path that radio has. EACH LEG LIFTS ITS OWN (row, path); a row is
//	      lifted when all of its paths are. The legs are A25's, and for A25's
//	      reason: a USB-B leg's behaviour is partly the Kenwood-supplied
//	      virtual COM driver's, which the RS-232C leg does not share.
//
// # The four divergences that make these two rows and not one
//
// One book covers both radios and marks several blocks "[TS-590S / TS-590SG
// common]" precisely because the rest are not common. FOUR divergences reach
// a capability value or a runtime branch (matrix §4), and each is named here
// so that a reader can check this package against the list rather than
// against their memory:
//
//  1. BYTE 28 (P11) — a live FILTER A/B selector on the SG; "always 0" on an
//     S at firmware 1.xx and UNSTATED at >= 2.00. The legend is common
//     (590:1560-1563); the firmware note is scoped to one row (590:1478, and
//     MW's differing wording at 590:1564, which is erratum E7). It reaches
//     Capabilities.Filters and FieldFilter: the SG publishes two labels and
//     grades the field, the S publishes an empty list and zeroes it. A14.
//
//  2. FV AND WHAT BRANCHES ON IT — the S row's WRITE path alone. An S at
//     >= 2.00, or one whose FV answer this programme cannot parse as A13's
//     M.NN form, has channel writes refused; NOTHING ON THE SG ROW BRANCHES
//     ON FV at all, because the SG's byte 28 is live by construction. The
//     four-character width is pinned by the answer chart (590:1037) and, on
//     the SG only, by a second corroborator — menu 000 is "Version
//     information (4 ASCII characters) read only" (590:749) — which is
//     SG-ONLY because the S's menu 000 is Display brightness (590:569). So
//     on the S row the width rests on the answer chart alone. A13.
//
//  3. MENU DOMAIN — 000 ~ 087 on the S and 000 ~ 099 on the SG
//     (590:543-544), which is why core/kw/ts590 holds TWO generated menu
//     inventories and why the settings descriptors are two. It reaches this
//     package at the settings path (Stage 2 task 13). A26.
//
//  4. SLOT CEILING — the extension channels 110-119 are the SG's
//     (590:1346-1347, the printed "E00 ~ P09" being erratum E2), and the S's
//     own ceiling is never stated, which is A12. It reaches Capabilities'
//     banks: BOTH rows publish a 100-slot MEM bank, because Stuart RULED on
//     05/09/2026 that the SG's ten extension channels are OMITTED until A11
//     lifts. A11 and A12.
//
// EVERYTHING ELSE IN THE BOOK IS COMMON TO BOTH ROWS — the whole 50-byte
// grid, the mode legend, both tone charts, the slot-space notes for 000-109,
// the error table and the transport page. But THE EVIDENCE STATUS OF EVERY
// COMMON VALUE IS STILL TRACKED PER ROW, because an assumption is lifted by
// an observation and an observation comes from one radio. A trial run on an
// SG that ticked the S's box would let this programme enable channel writes
// on a radio no one had ever connected (A9, A23).
//
// # What this package deliberately does NOT do
//
//   - NO DISCOVERY, on two independent grounds either of which would suffice
//     (decision 5, matrix §3.4): the books say the NAK is unreliable
//     ("Occasionally, this message may not appear due to microprocessor
//     transients in the transceiver", 590:106-108), so a probe's silence
//     carries no information at all; and the slot space is fully printed
//     (590:1345-1347), so a probe would ask a question the book answers.
//     Every bank is static and no bank is appended at Open.
//   - NO MC READ, and no MC frame of any kind on the read path (P13).
//     Recalling a channel changes the radio's operating state.
//   - NO ERASE, no transceive-set and no auto-baud, which are this
//     repository's standing rules and are additionally what the codec's own
//     outbound gate enforces.
package ts590

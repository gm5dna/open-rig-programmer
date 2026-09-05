// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts480 holds the TS-480 half of the Kenwood codec: its layout
// value and its menu inventory.
//
// THE ROW IS BUILT AND NOT REGISTERED at this milestone's close. A4 — that
// an MR of an empty channel ANSWERS rather than rejecting on this radio —
// is documented for the 590 pair (590:1492-1493) and entirely unprinted for
// the 480, and if it is false a fresh TS-480 cannot be read at all. Its
// lift, L-HW-3, is hardware confirmation item 3, and it is a GATE rather
// than a priority: everything else on the list buys capability, item 3 buys
// shippability. Until it lifts, this package exists, is tested, and is
// absent from internal/wiring's tables and from SupportedModels().
//
// The codec itself — framing, the accumulator, the matcher, the EX types
// and the typed error family — is core/kw's. This package carries only what
// is this radio's. It never imports core/cat or core/civ
// (core/kw/imports_test.go's fence covers this directory too).
//
// # The layout axes, and the line of this book each rests on
//
// This document prints the SAME 50-byte grid the 590 book prints — same
// order, same widths (480:923-943 for MR, 480:955-976 for MW) — and then
// gives several of those bytes different jobs. layout.go carries each
// citation at its own field and layout_test.go pins every value, so the
// table below is a reader's index rather than a second copy:
//
//	byte 4     P2   A printed constant, "Always 0 for the TS-480."
//	                (480:953), where the 590 pair carry the channel's
//	                hundreds digit. There is no bank field in this record
//	                at all: MC's own P1 reads "0: Always 0 for the TS-480
//	                (Memory bank number)." (480:827).
//	byte 19    P6   The CHANNEL LOCKOUT, "Lockout status. 0: Lockout OFF,
//	                1: Lockout ON." (480:962), where the 590 pair carry the
//	                data mode. The two radios then swap: they carry their
//	                lockout at byte 41 and this one prints a constant
//	                there.
//	byte 20    P7   THREE tone modes, "0: OFF, 1: TONE, 2: CTCSS"
//	                (480:964). No cross tone: the 590 pair's fourth value
//	                has no counterpart in this legend.
//	byte 28    P11  "Always 0 for the TS-480." (480:973).
//	bytes 39-40 P14 The tuning step, "Step size. Refer to the ST command."
//	                (480:979). ST's own legend is mode-conditional over two
//	                different ranges — 00 ~ 04 for SSB/CW/FSK and 00 ~ 09
//	                for AM/FM, with index 00 meaning 0.5 kHz in the first
//	                and 5 kHz in the second (480:1494-1500) — which is why
//	                A22 refuses every TS-480 channel write. That refusal is
//	                the driver's; this axis is what tells it which byte it
//	                is looking at.
//	byte 41    P15  "Always 0 for the TS-480." (480:982).
//	P5's legend     MR/MW P5 carries none of its own — "Mode. Refer to the
//	                MD command." (480:917, 480:959) — so the memory mode
//	                vocabulary IS MD's (480:843-854). Nibbles 0 and 8 read
//	                "No mode (Not used for the TS-480)" and "Tune (Not used
//	                for the TS-480)" (480:843, 480:853) and name no mode a
//	                channel can be in, so neither is in the legend. The 590
//	                pair's empty-channel reading of nibble 0 (A18a) is NOT
//	                available here: this book prints no empty-channel note
//	                anywhere, which is A4.
//	slot space      One flat bank, "00 ~ 99: Memory channel number"
//	                (480:955). Channels 90-99 also answer a second frame
//	                through the P1 overload (480:943-944, 480:986-987), but
//	                decision 15 rules them ordinary memories rather than a
//	                scan class, so this row gets no scan bank and the P1=1
//	                half of 90-99 is unreachable through this programme.
//	printed-fixed   SIXTEEN of the 47 parameter bytes, in six runs: the
//	                three both books print — P10 (480:971), P12 (480:975)
//	                and P13 (480:977) — plus this radio's own P2 (480:953),
//	                P11 (480:973) and P15 (480:982). Every one is REQUIRED
//	                ON PARSE and not merely emitted on build (decision 7),
//	                and on this row that strictness is a CHOICE rather than
//	                a deduction: A24, whose ground is the book's own
//	                general note that a Set may fill an inapplicable
//	                parameter with "any character except the ASCII control
//	                codes (00 to 1Fh) and the terminator (;)"
//	                (480:108-111). A24's lift is a dozen real reads of a
//	                real TS-480; a single counter-example turns the rule
//	                from "required" into "accepted and normalised".
//
// # This package's share of the errata schedule
//
// The schedule itself lives once, in core/kw/doc.go, with the ASSUMED
// register. These are the rows this book raises; each is named there in
// full and is listed here so that a reader of this package finds them:
//
//	E8   MR P8 cross-refers to "page 35" in a 24-page document (480:924)
//	     and the parallel MW P8 refers to TN instead and spells it "Refero"
//	     (480:966).
//	E9   The block headed XI prints its charts as "X T" (480:1745,
//	     480:1753), colliding with the real XT two pages later. Neither
//	     command is built or parsed here.
//	E10  TY is headed "Sets or reads the microprocessor fimware type"
//	     (480:1621) and has an EMPTY Set chart (480:1625), so it is
//	     read-only despite the heading.
//	E11  XO P1 reads "0: Plus diretion" (480:1771). Not built here.
//	E12  MD names nibble 7 "CWR (CW Reverse)" and nibble 9 "FSR (FSK
//	     Reverse)" (480:852, 480:854) where the 590 book names the same
//	     nibbles CW-R and FSK-R (590:1361, 590:1363). This package
//	     publishes the programme's own consistent spellings; the printed
//	     forms are recorded so nobody re-derives them as a correction.
//	E14  Nineteen rows read exactly "Always 0 for the TS-480"; one more
//	     drops the article (480:292). With the "Always 00", "Always 000"
//	     and "Always 000000000" variants the hard-wired family is 26
//	     printed rows — of which six rows fall inside the memory grid,
//	     sixteen bytes in all, and those bytes are this layout's
//	     printed-fixed set.
//	E15  No revision number, no part code, no firmware statement anywhere,
//	     an explicit no-support disclaimer, and TY P1 "Reserved" (480:8-9,
//	     480:12-15, 480:1623): this radio has no CAT-readable firmware
//	     version, which is why the layout refuses to build an FV read.
//	E17  The template prints Set / Read / Answer labels whether or not the
//	     direction exists, so an ABSENT direction is an EMPTY CHART UNDER A
//	     PRINTED LABEL — MR Set (480:911), MW Read and Answer (480:980,
//	     480:985), ID Set (480:679) and TY Set (480:1625). A transcriber
//	     who reads the label as evidence invents three commands per block.
//	E21  IF's P14 prints "Tone number (00 ~ 42). Refer to the TN and CN
//	     command." (480:725) against this book's own CN domain of 00 ~ 41
//	     (480:337). IF is not built here; it is recorded because a later
//	     milestone that builds IF must not read P14 against CN.
//	E22  The EX block's own prose names the two-digit menus — "Menu No.
//	     32, 35 and 48 ~ 52 use 2-digit parameters" (480:411) — and OMITS
//	     menu 034, "CW RX pitch/ TX sidetone frequency", whose printed grid
//	     runs 400/450/…/850 under codes 0 ~ 9 and continues into the "Over"
//	     column (480:483-485). THE CHART AND THE BLOCK DISAGREE, and it is
//	     the one printed defect this package's three-legged menu
//	     cross-check cannot find: both transcription legs read 034 as two
//	     digits, and so does the chart, and three faithful readings of one
//	     incomplete sentence agree perfectly.
//	     crosscheck_test.go's TestCrossCheck_TheMenu034Erratum pins BOTH
//	     sets — the chart's and the sentence's — so that neither
//	     "correcting" 034 to one digit nor quietly adding it to the
//	     sentence's list passes unremarked. This repository has no TS-480
//	     to ask which of the two a radio answers.
//
// AND ONE THAT IS NOT A DEFECT AT ALL: E18, the ANTI-defect. "Se t" for
// "Set" throughout (480:155, 480:338, 480:679, 480:829, 480:1143, 480:1515)
// is PageMaker letter-spacing, not a document error, and it is recorded so
// that no transcriber "corrects" it into evidence.
//
// The ASSUMED register lives once, in core/kw/doc.go, and is not restated
// here.
package ts480

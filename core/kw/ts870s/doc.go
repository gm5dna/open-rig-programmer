// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts870s holds the TS-870S row: a Layout870 value over core/kw's
// own second record type (record870.go), which this package constructs
// but does not modify.
//
// ONE ROW, BARE CONSTRUCTOR (matrix "ONE ROW, ONE COLUMN"). There is no
// sibling on this document, so there is nothing for a New<Variant>/
// New<Variant> pair to distinguish; Layout is the package-level value, the
// same shape core/kw/ts480's bare Layout480 var takes.
//
// THE CODEC IS core/kw/record870.go'S, NOT THIS PACKAGE'S. Layout870,
// Record870, NewLayout870, ParseMRAnswer and BuildMWSet are all landed
// there (Lift K, commit 748ef05) as purely additive machinery for a row
// whose grid is NOT a prefix of the shared 50-byte family (its offsets
// shift rather than merely stop early — matrix-ts870s.md's own finding).
// This package supplies only the one Layout870Config value TS-870S's own
// document settles.
//
// # Citations
//
// docs/superpowers/ts870s-capability-matrix.md ("the matrix") is the
// authority for every value below; ts870s:NNNN cites
// docs/fixtures-private/manuals/ts870s_manual_mirror_layout.txt directly,
// for the one value (MaxEXAddress) the matrix deliberately left ungraded.
//
// # ASSUMED register
//
//	A1  Build-side FieldMode refusal at P5 nibble 0 ("No mode") or 8 ("No
//	    Mode"): NEITHER IS DOCUMENTED AS A WRITE BEHAVIOUR (matrix §2.1),
//	    only as a read legend value. This package's ModeNames legend
//	    below names neither nibble — kw.Mode.namesAMode() is false for
//	    both, and NewLayout870 REFUSES any ModeNames entry that is not a
//	    real mode — so the refusal is the landed codec's own construction
//	    check, not code this package adds. Lift: one wire trial per
//	    value, no sibling row to multiply it across (matrix §6).
//	A2  DefaultBaud = 9600 is ASSUMED BY THE WAVE'S BLANKET RULE (every
//	    paper registration is), despite being the one DefaultBaud this
//	    wave's own document states outright: "The defaults in the
//	    TS-870S are 9600 bps and 1 stop bit." (ts870s:5869-5870, matrix
//	    §1.12). Register home: this driver register (core/driver/ts870s);
//	    lift: a wire read of the port at 9600 on first contact.
//
// # The lift-K gap: no Book870S stream-error citation
//
// core/kw/errors.go's newStreamError panics on Book870S (and Book570) —
// S2/S3's evidence transcription for this document stops at the
// command-table pages, so neither "E;" nor "O;" has a cited cause
// sentence here, and inventing one would be fabrication (framing.go's own
// doc comment, and errors.go's newStreamError doc comment, both say so in
// as many words: "a live NewFraming session for either book must not be
// wired up ... until a citation lands here").
//
// THIS PACKAGE ROUTES AROUND THE GAP RATHER THAN RESOLVING IT (brief
// option 2, not option 1): core/kw.NewFramingFor takes a kw.Layout, not a
// Layout870, so there is today no live-framing constructor Layout870
// could even be handed to — Book870S's stream-health machinery is
// structurally unreachable from this package, not merely undialled. See
// core/driver/ts870s's own doc comment for the driver-side consequence
// (no live Open this phase).
//
// # MaxEXAddress — the one value the matrix left for this package to cite
//
// EX/menu inventory is OUT of this wave's scope for every one of the
// seven packages (matrix §3, brief "Settled answers" §4): no EXItems
// table is built here, and the matrix itself does not grade a menu
// domain. Layout870Config.MaxEXAddress is nonetheless a REQUIRED
// structural field on the landed type (NewLayout870 refuses zero) — it
// bounds BuildEXRead/the outbound gate, not a claim that this package
// reads or writes any menu. 68 is the highest Menu No. this document's
// own EX parameter table prints ("MENU SELECTION TABLE FOR 'EX' COMMAND,
// PARAMETER 36", PDF pp.86-87 printed 80-81, ts870s:8468-8508: Menu Nos.
// 00-68; two rows also print function numbers 69-73 as a wider VALUE
// domain for that same menu column, not a wider ADDRESS). This is this
// package's own citation, not the matrix's: the matrix scoped EX out and
// so never settled this number.
//
// # A landed-codec imprecision, noted rather than fixed
//
// record870.go's BuildMWSet/ParseMRAnswer bound P8 against the shared
// kw.MinToneIndex/kw.MaxToneIndex (00-42, the family's TN/CN chart width)
// rather than against this row's own 39-entry SUBTONE TABLE (01-39,
// matrix §1.9). A tone index of 40-42 would therefore build and parse
// through this codec despite this document's own chart not printing it.
// record870.go is landed and this package does not modify it (brief); it
// is recorded here so a later reader does not mistake the wider bound for
// a fact about this radio.
package ts870s

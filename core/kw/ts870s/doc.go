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
// # The lift-K gap: CLOSED (13/09/2026 follow-up, commit e7515d0)
//
// core/kw/errors.go's newStreamError used to panic on Book870S (and
// Book570): S2/S3's evidence transcription for this document stopped at
// the command-table pages, so neither "E;" nor "O;" had a cited cause
// sentence. The Lift K follow-up read each document's own full manual
// text instead and found the table for both — this document's own "E;"
// sentence carries one fewer comma than the original four books'
// (commErrorCauseNoComma, ts870s:8445-8447), and its "O;" sentence agrees
// with the TS-480's (ts870s:8449-8450) — so newStreamError no longer
// panics on Book870S at all. The same follow-up added Layout870.Book(),
// a one-grammar AllowedCommand and NewFramingFor870, so a live session is
// now possible; core/driver/ts870s's own doc comment carries what it
// actually builds and what it deliberately still does not.
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
// # Two record870.go items, both FIXED in this follow-up
//
// record870.go's BuildMWSet/ParseMRAnswer used to bound P8 against the
// shared kw.MinToneIndex/kw.MaxToneIndex (00-42, the family's TN/CN chart
// width) rather than this row's own 39-entry SUBTONE TABLE (01-39,
// matrix §1.9) — a tone index of 40-42 wrongly round-tripped. It now
// bounds P8 against its own rec870MinToneIndex/rec870MaxToneIndex (1-39).
//
// record870.go also carried no "P4-P8 all zero" empty-channel pre-check
// (unlike the family's kw.Record), so a genuinely vacant channel's mode
// byte ('0') was refused by the mode-legend check before ever reaching
// an empty-window test. Record870 now carries an Empty field and
// isEmptyWindow870 tests P4-P8 before any of them is interpreted — the
// manual's own sentence, "the Answer command sends '0' for all
// parameters except the memory channel number" (ts870s:9101-9104),
// finally round-trips.
//
// Both were small, cited fixes made directly in core/kw/record870.go
// once this package's own driver follow-up needed a working live session
// (this package does not otherwise touch that file).
package ts870s

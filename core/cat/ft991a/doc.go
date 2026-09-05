// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft991a holds the Yaesu FT-991A's CAT dialect data. So far that is
// its EX menu inventory alone, transcribed from the manual's menu chart.
//
// # Provenance
//
// Everything here comes from the Yaesu FT-991A CAT Operation Reference
// Manual, revision 1711-D (docs/fixtures-private/manuals/ft991a_cat_1711-D.pdf
// and its layout extraction ft991a_layout.txt — both gitignored, so the line
// references throughout this package are citations, not links). The untitled
// menu chart that follows the EX MENU command block spans layout lines
// 530-694; the EX inventory transcribed from it lives in table2.csv, whose
// header carries the transcription conventions and the chart's verbatim
// defects and is the provenance record of first resort for this package.
//
// NO FT-991A HARDWARE HAS EVER BEEN ASKED ANYTHING by this project, and none
// is available to it. Every statement in this package is a reading of a
// manual, and nothing here may be quoted as verification.
//
// THIS DOC COMMENT IS A STUB. The dialect, its ASSUMED register and its
// reused-command verification are not written yet; when they are, this file
// carries them, as core/cat/ft891/doc.go does for that model.
package ft991a

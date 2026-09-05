// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"bufio"
	"io"
)

// utf8BOM is the UTF-8 encoding of U+FEFF ZERO WIDTH NO-BREAK SPACE, used
// as a byte-order mark. In UTF-8 a byte order needs no marking, so the
// sequence carries no information here — it is purely an encoding hint
// some Windows software writes and some Windows software expects.
const utf8BOM = "\xEF\xBB\xBF"

// skipUTF8BOM returns a reader over r with at most ONE leading UTF-8 BOM
// removed. Any other content, including a second BOM, is passed through
// untouched.
//
// Why this exists: Excel's "CSV UTF-8 (Comma delimited)" — the only Excel
// save format that keeps non-ASCII channel names intact, so the one a
// Windows user with an accented callsign in a tag is steered towards —
// writes those three bytes in front of the header. encoding/csv does not
// treat them specially, so without this they become part of the first
// header cell: "slot" arrives as a four-character name no user can see is
// different, and Import rejects the file for an unknown column and a
// missing required one that are the same column. ImportCHIRP fares worse,
// refusing the file outright for a missing core column (Location).
//
// Scope, deliberately narrow. This strips a BOM; it does not decode
// UTF-16, transcode anything, or touch line endings — encoding/csv already
// drops the CR of a CRLF pair itself, so a Windows-saved CSV needs nothing
// further. Only the two READERS use it: Export never writes a BOM, and
// must not start (see TestExport_NoBOM_AndLFLineEndings — a BOM there
// would change every exported byte).
//
// A bufio.Reader is the mechanism because a caller's io.Reader may be
// unseekable (an os.File, a network body, a strings.Reader alike) and may
// deliver the first three bytes across several Read calls; Peek handles
// both, and a short file — empty, or shorter than the BOM — simply peeks
// what there is and matches nothing. Pinned by TestImport_BOMStrippedOnlyOnce
// and TestImport_EmptyAndBOMOnlyInput (and their CHIRP twins).
func skipUTF8BOM(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	head, err := br.Peek(len(utf8BOM))
	if err == nil && string(head) == utf8BOM {
		_, _ = br.Discard(len(utf8BOM))
	}
	return br
}

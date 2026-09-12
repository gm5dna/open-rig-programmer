// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

const (
	// RecordOnlyLength is what civ.Profile carries and what BuildMemorySet's
	// <record> argument denotes.
	//
	// spec.md's ruling 1's "25 B, unchanged by coincidence of the repack" is
	// SUPERSEDED — matrix §1b/§3.11 found a fifteen-byte TX-duplicate block
	// (printed FILLED reference circles ❹–⑱, distinct from the OUTLINE
	// circles used everywhere else on PDF p.115) that the tier spec's own
	// arithmetic and the S1 evidence leg both missed. The corrected sum is
	// 1+5+2+1+1+3+3+15+9 = 40 bytes (matrix §3.11; ruled by the orchestrator
	// at spec.md's ruling 7).
	RecordOnlyLength = 40
	// DataAreaLength includes the two-byte flat channel selector.
	DataAreaLength = 42
	AddressBytes   = 2

	// SelectSplitOffset is record byte 0 (printed ③): a whole byte, BOTH
	// nibbles UNMAPPED. High nibble is Select-memory OFF/ON, low nibble is
	// Split OFF/ON (matrix §1b, offset 0 row). Neither has a neutral home —
	// codeplug carries no per-channel "in a select group" concept distinct
	// from ScanSkip's own meaning on THIS radio (matrix §2 MEM row 9: this
	// byte is NOT a scan-skip vocabulary at all, unlike the IC-7610
	// family's superficially similar byte in the same position) — so the
	// whole byte is UNMAPPED rather than split field-by-field.
	SelectSplitOffset = 0

	// TXDupUnmappedOffset and TXDupUnmappedLength name the ten bytes of the
	// fifteen-byte TX-duplicate block that carry NO neutral field: TX mode,
	// TX filter, TX data mode, TX tone-mode nibble, TX repeater tone
	// frequency (3B) and TX tone-squelch frequency (3B) — matrix §1b's
	// decomposition table, rel. offsets 5-14 of the duplicate block, i.e.
	// absolute record offsets 21-30. Only the block's first five bytes
	// (rel. 0-4, absolute 16-20) carry a neutral field: FieldTXFrequency,
	// per spec.md's ruling 7 ("map the TX frequency span to
	// spec.FieldTxFrequency ... the remaining TX-side mode/filter/data/tone
	// bytes are deliberately zero").
	TXDupUnmappedOffset = 21
	TXDupUnmappedLength = 10
)

// NameCharset is every byte a memory name may carry: A-Z, a-z, 0-9, space
// and the symbol table, transcribed table by table from PDF p.113 (printed
// 106), "Character code setting", Command 1A 00 — the SAME page as every
// other symbol, which is why space is a DIRECT reading here rather than
// the cross-command inference the IC-7610 family's matrix needed (matrix
// §1 row 25, §3.9(ii)).
const NameCharset = "" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"0123456789" +
	" " + // Space 20 — printed directly, matrix §3.9(ii)
	"!#$%&\\?\"'`^+-*/.,:;=<>()[]{}|_~@"

// FixedTemplate is a fresh 40-byte all-zero template: the value every
// unmapped record region (SelectSplitOffset and the ten TX-duplicate-block
// bytes at TXDupUnmappedOffset) is judged against on write. A fresh make()
// per call, so no caller can move the thing every write is judged against.
func FixedTemplate() []byte { return make([]byte, RecordOnlyLength) }

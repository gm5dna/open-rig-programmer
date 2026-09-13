// SPDX-License-Identifier: GPL-3.0-or-later

package ts570

import "github.com/gm5dna/open-rig-programmer/core/kw"

// THE THREE ROW VALUES, AND WHY THEY DIFFER ONLY IN NAME.
//
// TS-570D and TS-570S share one document and, per the Parameter Table's own
// MODEL NUMBER format (matrix §2, "TS-570S: 018 TS-570D: 017"), the same
// 28-byte record in every other respect: same P2Unused byte 4, same
// Byte19Lockout byte 19, same ToneModesTwo byte 20, same eight-mode legend,
// same 000-099 slot space. TS-570DG has no independent reading at all — see
// doc.go — so LayoutDG is LayoutD's config with only Model changed and its
// CATID left empty (ASSUMED, ts570-dg-catid-assumed).
//
// THE VALUES ARE MINTED ONCE, AT INITIALISATION, AND HANDED OUT BY VALUE,
// exactly as core/kw/ts590's are: kw.Layout's fields are unexported and its
// accessors copy, and MustNewLayout turns a config kw.NewLayout would
// refuse into a start-up panic rather than a zero Layout failing closed
// silently everywhere later.

// maxEXAddress is this document's own printed MENU NUMBER domain: "Represented
// using 000~051." (matrix §2, Parameter Table Format 35, PDF p.79 printed
// 73). It is REQUIRED by kw.NewLayout regardless of scope — see doc.go's
// "EX/menu inventory is out of scope" section for why setting it here is
// not building the deferred inventory.
const maxEXAddress = 51

var (
	layoutD  = kw.MustNewLayout(rowConfig("TS-570D"))
	layoutS  = kw.MustNewLayout(rowConfig("TS-570S"))
	layoutDG = kw.MustNewLayout(rowConfig("TS-570DG"))
)

// LayoutD returns the TS-570D row's reading of the shared memory grid.
func LayoutD() kw.Layout { return layoutD }

// LayoutS returns the TS-570S row's reading of the shared memory grid. It
// differs from LayoutD only in Model; every axis, the slot space and the
// EX domain are the document's own, shared by both named rows.
func LayoutS() kw.Layout { return layoutS }

// LayoutDG returns the TS-570DG row's reading of the shared memory grid.
//
// UNVERIFIED-BY-INHERITANCE (matrix §5): no document names this row
// independently. Every value is LayoutD's own, carried across under the
// third-party catalogue and product-page grouping that is the whole of
// this row's evidence.
func LayoutDG() kw.Layout { return layoutDG }

// rowConfig is the one config all three rows share, differing only in the
// Model string the row's own refusals and Model() will quote.
//
// IT IS A FUNCTION RATHER THAN A SHARED VARIABLE, on core/kw/ts590's own
// precedent: a package-level kw.LayoutConfig copied and amended would share
// ModeNames and Slots between rows, and a mutation to one would edit all
// three. Every call builds fresh containers.
func rowConfig(model string) kw.LayoutConfig {
	return kw.LayoutConfig{
		// The one document all three rows speak (matrix §0), which is
		// what an "O;" would quote its cause sentence from — except that
		// Book570 has none transcribed yet; see doc.go's "two named gaps".
		Book:  kw.Book570,
		Model: model,

		// The true 28-byte PREFIX of the family's 50-byte grid (matrix
		// §1.4): positions 1-22 line up field for field, and the record
		// simply stops after position 27 with the terminator at 28.
		RecordLen: 28,

		// Byte 4 (P2): the Parameter Table prints a dash, no digit count
		// at all — neither a hundreds digit nor a printed "0" (matrix
		// §1.2, doc.go).
		P2: kw.P2Unused,
		// Byte 19 (P6): the channel lockout, "0: Not locked out / 1:
		// Locked out" (matrix §1.2, Format 10) — the TS-480's reading,
		// not the 590 pair's data mode.
		Byte19: kw.Byte19Lockout,
		// Byte 20 (P7): "0: OFF / 1: ON" only (matrix §1.2, Format 1) —
		// narrower than every other registered row.
		ToneModes: kw.ToneModesTwo,

		// RecordLen 28 has no tail past P8 (byte 22): NewLayout refuses
		// Byte28/Byte3940/Byte41/P10/P12/P13 set on a row this narrow, so
		// none is supplied here.

		MaxEXAddress: maxEXAddress,

		ModeNames: modeNames(),

		// 000-099, one flat MEM bank (matrix §2 Banks): no hundreds
		// digit exists (P2Unused), so the channel number is P3's two
		// digits alone, the same ceiling a P2FixedZero row carries.
		// Channels 90-99 answer the documented Start/End scan-edge
		// overload (matrix §2, decision 15) as ORDINARY memories, not a
		// second SlotScan bank.
		Slots: []kw.SlotRange{
			{Class: kw.SlotMemory, Lo: 0, Hi: 99},
		},

		// No printed-fixed run at all: this row's 28-byte record has no
		// byte past position 27, so none of P10/P12/P13/byte 28/byte 41
		// exists to hard-wire.
		PrintedFixed: nil,
	}
}

// modeNames is the MD legend this row's P5 reads against (matrix §1.3):
// byte-for-byte the family's own eight named modes. MR/MW P5 carries no
// legend of its own here either — the Parameter Table's Format 2 entry is
// "0: No selection, 1: LSB, 2: USB, 3: CW, 4: FM, 5: AM, 6: FSK, 7: CW-R, 8:
// No selection, 9: FSK-R" (matrix §1.3), the identical ten nibbles
// core/kw.Mode already declares, so this row needs zero new vocabulary.
func modeNames() map[kw.Mode]string {
	return map[kw.Mode]string{
		kw.ModeLSB:  "LSB",
		kw.ModeUSB:  "USB",
		kw.ModeCW:   "CW",
		kw.ModeFM:   "FM",
		kw.ModeAM:   "AM",
		kw.ModeFSK:  "FSK",
		kw.ModeCWR:  "CW-R",
		kw.ModeFSKR: "FSK-R",
	}
}

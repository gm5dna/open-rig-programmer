// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "fmt"

// EXAddress is one Kenwood menu/EX address.
//
// THE THREE FIELD NAMES ARE NOT NEGOTIABLE. internal/extable's RenderGo
// emits "{Addr: kw.EXAddress{P1: n, P2: n, P3: n}, ...}" literally — the
// renderer hard-codes the type name and all three field names, and
// parameterising them was considered and rejected as more code for no gain
// (spec §"What internal/extable needs", item 3). So this struct carries P2
// and P3 even though a Kenwood address uses only P1.
//
// P2 AND P3 ARE ZERO ON EVERY KENWOOD ROW, and that is the AddressSingle
// rule Stage 0's task 2 landed in internal/extable: its ParseCSV arm
// refuses a row whose p2 or p3 column is anything else rather than dropping
// it. The wire field is a THREE-DIGIT menu number and nothing more —
// "E X P1 P1 P1 P2 P2 P3 P4 ;" is the whole 10-byte read frame, with P2
// always "00" and P3 and P4 always '0' (590:546-553, 480:402-407).
//
// The zero value is address 000, which is a real address on all three
// radios — Display brightness on the TS-590S (590:569) and the TS-480
// (480:427), Firmware Version (read only) on the TS-590SG (590:749). So an
// unset EXAddress is NOT distinguishable from a valid one by inspection,
// and membership is the INVENTORY's business, never this type's.
type EXAddress struct {
	P1, P2, P3 uint8
}

// Wire renders a as the EX address field: three zero-padded decimal digits,
// P1 alone.
//
// IT IS A METHOD ON THE ADDRESS, WHICH core/cat DELETED FOR GOOD REASON AND
// WHICH IS SAFE HERE. There, an address could not know how many digits its
// family's field carried — six under EXAddressTriple, four under
// EXAddressPair — so any wire-shaped method on the address gave every radio
// the FT-710's answer. This family has ONE width on the three MR/MW rows:
// the printed frame is three digits on the TS-590S, the TS-590SG and the
// TS-480 alike (590:552, 480:410), and internal/extable registers those
// three profiles as AddressSingle. There is no second form for a method to
// get wrong.
//
// THE SCOPE IS THOSE THREE ROWS, NOT THE WHOLE FAMILY. The TS-890S and the
// TS-990S rows have their own EX rendering, core/kw/ma's WireEXAddress, and
// it is a different width — which is core/cat's lesson applied rather than
// repeated: this method is safe because the rows that can reach it all share
// one width, not because the family has only one.
//
// IT FAILS CLOSED on a non-zero P2 or P3. Such a value is not a Kenwood
// address at all, and rendering P1 alone from one would silently discard
// information the caller believed it had supplied; "" then reaches the
// outbound gate as a frame that cannot pass it. core/cat's wireEXAddress
// returns "" for an unspecified form for the same reason.
func (a EXAddress) Wire() string {
	if a.P2 != 0 || a.P3 != 0 {
		return ""
	}
	return fmt.Sprintf("%03d", a.P1)
}

// String returns a NON-WIRE debug rendering, "P1=000 P2=00 P3=00".
//
// core/cat's precedent, and both of its properties: it carries bytes no
// address field may hold, so a debug print can never be read back as one,
// and it names ALL THREE components, which the wire form deliberately does
// not.
func (a EXAddress) String() string {
	return fmt.Sprintf("P1=%03d P2=%02d P3=%02d", a.P1, a.P2, a.P3)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"fmt"
	"strconv"
	"strings"
)

// This radio's three banks (matrix §4 "Banks"): MEM 1-99, P1-9, QMB1-5 —
// 113 slots total, the same range the Opcode Command Chart's own
// "X = 01H ~ 71H, corresponding to memories 1 ~ 99, P1 ~ P9, and QMB 1 ~
// 5" names (matrix §1.4/§1.8).
const (
	memCount = 99
	pCount   = 9
	qmbCount = 5
)

// dumpHeaderLen is the full dump's own fixed prefix before its first
// 16-byte record: 6 Status Flag bytes plus 1 current-memory-channel byte
// (spec.md §Read model; matrix §1.6 "1,863 bytes: 6 flag bytes + 1
// current-channel byte + 116x16-byte records").
const dumpHeaderLen = 7

// dumpRecordCount is every 16-byte record the full dump carries: current
// Operating Data, VFO-A, VFO-B, then the 113 memories, in that fixed
// order (matrix §1.6, ft1000mpmarkv_manual layout:5058-5061).
const dumpRecordCount = 3 + memCount + pCount + qmbCount

// memSlots, pSlots and qmbSlots return this radio's three bank
// inventories in wire-form, e.g. "1".."99", "P1".."P9", "QMB1".."QMB5".
func memSlots() []string { return numberedSlots("", 1, memCount) }
func pSlots() []string   { return numberedSlots("P", 1, pCount) }
func qmbSlots() []string { return numberedSlots("QMB", 1, qmbCount) }

func numberedSlots(prefix string, from, count int) []string {
	slots := make([]string, count)
	for i := range slots {
		slots[i] = fmt.Sprintf("%s%d", prefix, from+i)
	}
	return slots
}

// parseSlot splits a wire-form slot into its bank prefix and 1-based
// number, refusing anything outside the three declared banks/ranges.
func parseSlot(slot string) (prefix string, n int, err error) {
	switch {
	case strings.HasPrefix(slot, "QMB"):
		prefix = "QMB"
	case strings.HasPrefix(slot, "P"):
		prefix = "P"
	default:
		prefix = ""
	}
	numStr := strings.TrimPrefix(slot, prefix)
	n, convErr := strconv.Atoi(numStr)
	if convErr != nil || n < 1 {
		return "", 0, fmt.Errorf("%w: %q is not a recognised ft1000mp slot", errBadSlot, slot)
	}
	switch prefix {
	case "":
		if n > memCount {
			return "", 0, fmt.Errorf("%w: %q is outside MEM's 1..%d range", errBadSlot, slot, memCount)
		}
	case "P":
		if n > pCount {
			return "", 0, fmt.Errorf("%w: %q is outside P's 1..%d range", errBadSlot, slot, pCount)
		}
	case "QMB":
		if n > qmbCount {
			return "", 0, fmt.Errorf("%w: %q is outside QMB's 1..%d range", errBadSlot, slot, qmbCount)
		}
	}
	return prefix, n, nil
}

// recordIndex returns slot's 0-based position among the dump's 116
// records — index 0 is current Operating Data, 1 is VFO-A, 2 is VFO-B,
// 3.. are the 113 memories in fixed MEM-then-P-then-QMB order (matrix
// §1.6). This does NOT depend on the channel-numbering-base ambiguity
// (§1.4/doc.go): it is the dump's own physical layout, never an opcode
// argument.
func recordIndex(slot string) (int, error) {
	prefix, n, err := parseSlot(slot)
	if err != nil {
		return 0, err
	}
	const firstMemory = 3
	switch prefix {
	case "":
		return firstMemory + (n - 1), nil
	case "P":
		return firstMemory + memCount + (n - 1), nil
	case "QMB":
		return firstMemory + memCount + pCount + (n - 1), nil
	default:
		return 0, fmt.Errorf("%w: %q", errBadSlot, slot)
	}
}

// storeArg returns slot's Store/Enter (and Recall) opcode argument X,
// per the 15/09/2026 override's ASSUMED 1-based channel base (matrix
// §1.4): channel N's argument is X=N directly, MEM 1-99 -> 01H-63H, P1-9
// -> 64H-6CH, QMB1-5 -> 6DH-71H — the SAME 01H~71H range the Opcode
// Command Chart states.
func storeArg(slot string) (byte, error) {
	prefix, n, err := parseSlot(slot)
	if err != nil {
		return 0, err
	}
	switch prefix {
	case "":
		return byte(n), nil
	case "P":
		return byte(memCount + n), nil
	case "QMB":
		return byte(memCount + pCount + n), nil
	default:
		return 0, fmt.Errorf("%w: %q", errBadSlot, slot)
	}
}

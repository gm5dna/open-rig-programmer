// Package fakeic7760 is an independently authored IC-7760 CI-V simulator.
// Its register is the frozen B/W evidence: selectors 00 01–00 99, 01 00 and
// 01 01; a derived 25-byte record; command 1A 00; and ACK/NG FB/FA.
// Assumed empty-channel FA, identity token, echo placement, transceive address,
// and P1/P2 record shape are isolated below and configurable where useful.
package fakeic7760

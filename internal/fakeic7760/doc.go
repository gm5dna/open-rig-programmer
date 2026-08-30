// Package fakeic7760 is an independently authored IC-7760 CI-V simulator.
// Its register is the frozen B/W evidence: selectors 00 01–00 99, 01 00 and
// 01 01; a derived 25-byte record; command 1A 00; ACK/NG FB/FA; and the
// printed B2/E0 address pair, both halves of which it requires.
// Every assumed behaviour — empty-channel FA, the inbound all-FF reading,
// the identity token, echo placement, the broadcast form, full-record
// enforcement and the P1/P2 record shape — is reached through an option named
// for the capability-matrix register entry that owns it; options.go and
// PROVENANCE.md list the mapping.
package fakeic7760

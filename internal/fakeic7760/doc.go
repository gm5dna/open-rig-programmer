// Package fakeic7760 is an independently authored IC-7760 CI-V simulator.
// Its register is the frozen B/W evidence: selectors 00 01–00 99, 01 00 and
// 01 01; a derived 25-byte record; command 1A 00; ACK/NG FB/FA; and the
// printed B2/E0 address pair, both halves of which it requires.
// Every assumed behaviour — empty-channel FA, the inbound all-FF reading,
// the identity token, echo placement, the broadcast form, full-record
// enforcement and the P1/P2 record shape — is reached through an option named
// for the capability-matrix register entry that owns it; options.go and
// PROVENANCE.md list the mapping.
//
// THE ONE EXCEPTION IS internal/fakepipe (added 06/09/2026): the net.Pipe pair,
// the goroutine bookkeeping, the interruptible latency wait and the raw write.
// It is permitted because it is PROTOCOL-FREE — it sees []byte and a duration
// and nothing else, so it carries no framing, no field layout and no reply
// building. A bug in it therefore cannot make a wrong codec look right; it can
// only stop bytes moving, which this package's own tests notice at once.
// Everything above the wire — the reassembler, the parser, the image, the
// replies — stays here, written independently.
package fakeic7760

// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft710 is the Yaesu FT-710 driver: the first (and reference)
// implementation of core/driver's Driver/Session seam. Every FT-710
// protocol quirk lives HERE — the layers above see only capability data
// and codeplug values, and the layers below (core/cat, core/transport)
// stay radio-behaviour-neutral.
//
// # The write guard
//
// This package owns the project's hardware write gate: three capability
// profiles (CapabilitiesRealHardware, CapabilitiesUnverified,
// CapabilitiesSimulated) and the package-level constant
// writeTrialsComplete — which FLIPPED to true in the M5b PR
// (13/07/2026), with the hardware evidence linked in its doc comment
// (caps.go) and in docs/hardware-notes.md's "M5b write trials" section.
// RealHardware sessions now get CapabilitiesRealHardware: Write
// Supported for exactly the six HW-verified fields
// (frequency/mode/ctcss_state/shift/tag/tag_display, MEM and PMS),
// clarifier Inert (transmitted but ignored — HW-CONFIRMED),
// tone/skip/erase Unsupported (protocol-impossible / NO CAT erase,
// HW-CONFIRMED). Unrecognised Profile values still fail safe to the
// all-Unverified profile (nothing writable).
//
// Post-flip, what stands between a user and a wire write is no longer a
// capability veto but the layered choreography the veto used to sit in
// front of: codeplug.Diff's per-field/Inert/erase gates, the clone
// service's full send choreography (fresh baseline, immutable plan,
// digest + session + confirmation binding, firmware gate, per-slot
// verify-read and write-then-verify), Session.WriteChannel's own
// capability re-check before any wire traffic, and internal/guards'
// repo-wide import-graph pin on the write path. The ledgered
// write-capability-split flip precondition was adjudicated
// RATIFIED-AS-NOT-NEEDED — see writeTrialsComplete's doc comment for
// the restated reasoning.
//
// # Quirks register
//
// Like fakeradio's ASSUMED register (internal/fakeradio/doc.go), every
// place this driver encodes a protocol behaviour the manual does not
// nail down is listed here in one place, for M5a/M5b reconciliation
// against real hardware. Where a quirk mirrors a fakeradio register
// entry, the entry number is cross-referenced — the two lists must be
// reconciled TOGETHER: correcting the fake without the driver (or vice
// versa) would let tests keep passing against a behaviour real hardware
// does not have.
//
//  1. Empty-slot "?;" mapping (read.go, ReadChannel; fakeradio register
//     item 2). An MR read answered "?;" is mapped to an EMPTY channel,
//     not an error. HW-CONFIRMED 2026-07-13 (docs/hardware-notes.md
//     §Empty/out-of-range slots): live probe "MR010;" (never-populated
//     slot) -> "?;", and "MR100;" (grammatically valid, out-of-inventory
//     slot) -> the identical "?;" — the manual did not document either
//     case; hardware confirms both mean "empty"/"nothing there", not
//     some other unattributed failure.
//
//  2. Kind-on-write and kind-on-read leniency (write.go,
//     buildWriteCommands; read.go, acceptedKinds; fakeradio
//     register item 9; core/cat/mw.go). HW-CONFIRMED 2026-07-13 (M5b
//     write trials, docs/hardware-notes.md): MW's P7 is ALWAYS sent as
//     '1' (KindMemory), for BOTH memory AND PMS slots — the former
//     ASSUMED pairing (KindPMS '5' for a PMS slot) is REFUTED: a live
//     PMS write carrying KindPMS was REJECTED (immediate "?;"), the
//     identical write carrying KindMemory was accepted. On READ, a
//     populated PMS slot answered MR with kind '1' (as CAT-written) —
//     contradicting this driver's former STRICT read-side check, which
//     demanded '5' for every PMS slot and aborted the whole read
//     (`rigprog read`/GUI ReadRadio/PrepareSend) on any radio with a
//     populated PMS pair (reproduced live: `ft710: MR answer for slot
//     "P1L" carries kind '1', want '5' for this slot's bank`). The FIX:
//     read now accepts a LENIENT, documented set per bank — {'0','1','4'}
//     for memory-bank slots, {'0','1','5'} for PMS slots; write NEVER
//     re-emits a read kind, it always builds fresh with KindMemory, so
//     this leniency cannot smuggle a stale/wrong kind back onto the
//     wire. The PMS set gained '0' on 30/08/2026 (HW-OBSERVED, field
//     report under v1.2.0): the CAT-written P1L of 13/07/2026 read back
//     kind '0' with no intervening write or front-panel edit, aborting
//     every read of the radio — P7 is radio-internal state the radio
//     itself changes, not a bank label (read.go, acceptedKinds).
//
//  3. 60 m numbering and discovery (ft710.go, discoverInventory;
//     fakeradio register items 10, 12, 13). 60 m channels are ASSUMED
//     numbered "501" upward; discovery reads sequentially and stops at
//     the FIRST "?;" rejection, capped at 15 (the US set — the largest
//     known). This assumes factory 60 m channels are contiguous,
//     populated, and non-erasable; an empty-but-existing 60 m slot
//     would truncate discovery. The region fingerprints (7 = "UK",
//     15+EMG = "US", zero+no-EMG = "no-60m", else "unknown-N") mirror
//     fakeradio's factory images. When all 15 known slots answer, ONE
//     sentinel slot ("516") is probed too (M3 Codex-review fix wave, Fix
//     6): if it ALSO answers, the bank is capped at the 15 VERIFIED
//     slots and the region reports "unknown-16plus" — never "US" — with
//     the anomaly logged via the driver's transport logger; M5a must
//     extend max60mProbe (and the sentinel's bound with it) if a
//     legitimate >15-channel set exists. HW-CONFIRMED 2026-07-13 for the
//     zero-inventory case ONLY (docs/hardware-notes.md §60m regional
//     finding): Stuart's UK FT-710 has no 5xx bank at all, so discovery
//     correctly finds nothing and deriveRegion reports "no-60m" — a
//     known real variant, not an anomaly. The "501" numbering
//     assumption, the 7-channel "UK" fingerprint, and the 15+EMG "US"
//     fingerprint remain wholly ASSUMED: no radio with an actual 5xx
//     bank has been characterised yet. Verify at a future M5a-class
//     session against a 60m-bearing radio.
//
//  4. 60 m/EMG kind byte (read.go, acceptedKinds; fakeradio register item
//     11). MR answers for 5xx/EMG slots are expected to carry P7 = '1'
//     (memory-like): the manual's kind table has no distinct code for
//     them. A different value fails ReadChannel's kind check. Codex M5b
//     fix wave, Fix 5 (adjudicated MEDIUM): acceptedKinds used to apply
//     MEM's LENIENT {'0','1','4'} set to 60m/EMG too (item 2's leniency
//     was HW-earned for MEM specifically, never for these banks) — it now
//     branches explicitly per bank: PMS {'0','1','5'}; MEM {'0','1','4'};
//     discovered 60m/EMG STRICT {'1'} only, until an actual 5xx/EMG-bearing
//     radio is characterised. Verify at a future M5a-class session.
//
//  5. MT short-form answer (read.go, ReadChannel; fakeradio register
//     items 4 and 8, and fakeradio's buildMTReply). The driver parses
//     the MT answer as the Set-shaped short form ("MT" + slot + display
//     + 0-12 byte tag + ";") via cat.Dialect.ParseMTAnswer. HW-CONFIRMED
//     2026-07-13 (docs/hardware-notes.md §MT short-form answer): live
//     probe "MT006;" -> "MT0061" + a 4-char tag + 8 trailing spaces
//     (12 bytes total) — the short form, not Hamlib's longer combined
//     form, and the radio DOES pad a short tag with trailing spaces to
//     the full 12-byte field, both for a front-panel-origin tag and
//     (HW-CONFIRMED the hard way, 13/07/2026, the first real-radio
//     production write — see fakeradio register item 8) for a tag
//     written via CAT MT-set. cat.Dialect.ParseMTAnswer now TRIMS those
//     trailing spaces before returning (Fix: tag normalisation) rather
//     than preserving them verbatim: padding is purely a wire-encoding
//     detail, and the driver's ReadChannel result — like every other
//     model-level tag in this project — is always the trimmed form.
//
//  6. MT read of a populated slot never legitimately answers "?;"
//     (read.go; fakeradio register item 4): tag state is modelled as
//     independent of channel-data state, so after a successful MR, an
//     MT rejection is treated as a real error, not an empty-tag signal.
//     Consistent with M5a's live evidence (every MT006 following a
//     successful MR006 answered cleanly, never "?;"), though the session
//     did not specifically try to provoke a rejection here — still
//     largely ASSUMED as a universal claim.
//
//  7. P9 carries nothing worth preserving (read.go; core/cat/mr.go).
//     The MR answer's P9 field is documented as fixed "00" and
//     cat.Dialect.ParseMRAnswer rejects anything else, so the driver
//     stores nothing for it. HW-CONFIRMED 2026-07-13
//     (docs/hardware-notes.md §MR bytes-25/26 refutation): M5a's
//     scoped tone-index question is SETTLED — bytes 25-26 read "00" on
//     M-06 even with a CTCSS tone (146.2 Hz) SET and ACTIVE (P8=1) on
//     the radio at capture time. P9 is NOT a live tone index on this
//     firmware; there is nothing slot-specific to preserve here, and
//     there will not be. See caps.go's FieldCTCSSTone doc comment for
//     the capability-level consequence.
//
// # Out of scope (deliberately)
//
// Tone capture via MC/CN (the M5a tone-index question) and write
// verification by read-back (the clone service owns verify policy) are
// NOT this package's job. EX menu settings gained READ-ONLY support at
// M8b-2 (task 32, settings.go): SettingsDescriptor/ReadSetting implement
// the optional driver.SettingsReader capability over the M8a EX
// inventory. EX menu SET remains out of scope — fakeradio itself does
// not model an EX-set exchange yet (internal/fakeradio/doc.go register
// item 24), and no wire evidence exists for it; a future milestone would
// add both the fake and this driver's write side together.
package ft710

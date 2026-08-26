// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic9700 is the Icom IC-9700's driver: the probe that mutates
// nothing, the read that knows two ways of saying "empty", and the write
// that refuses every byte it cannot name.
//
// It speaks CI-V through core/civ and this radio's dialect in
// core/civ/ic9700, and it holds no frame machinery of its own: the framing
// adapter, the three answer matchers and the two CommandSpec helpers are
// the shared enablers' (core/civ/framing.go), and nothing here re-exports
// them.
//
// THIS PACKAGE IMPORTS NO SIBLING DRIVER PACKAGE, and the rule is not
// hygiene. Importing core/driver/ftdx101 or core/driver/ft710 to reuse a
// helper is how one radio's EVIDENCE silently becomes another's CLAIM: a
// name map, a refusal message or a default lifted across carries the
// manual it was read from, and nothing downstream would say so. Where a
// shape is genuinely shared this package restates it and says which
// package it mirrors — requestedFields is the example, and it is a mirror
// precisely because the Yaesu version's unconditional set would refuse
// every write on this radio.
//
// # NO IC-9700 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
//
// Not one frame has been sent to one, not one answer has been captured
// from one, and every byte in core/civ/ic9700 is transcribed from a PDF.
// writeTrialsComplete is FALSE, every write column is spec.Unverified, and
// the only key that opens the write gate is the user's explicitly recorded
// consent — which is a statement about the operator, not evidence about
// the radio. The register below is what a hardware session would settle,
// entry by entry.
//
// # The register
//
// Thirty-four entries: ELEVEN inherited FAMILY assumptions (spec D5), each
// as THIS MODEL'S OWN ROW; EIGHTEEN `ic9700` DRIVER entries from the
// capability matrix's §5; and FIVE this driver ADDS, proposed back to the
// matrix as errata.
//
// CITE ENTRIES BY NAME, NEVER BY POSITION. A positional citation — "entry
// 6" — is correct only until somebody adds or reorders an entry, and it
// then points silently at the wrong assumption instead of failing. The
// spec's D5 register is a numbered list, so its number is its address, but
// every citation of it here names the subject beside the number so that a
// renumbered spec produces a visible contradiction rather than a silent
// misattribution.
//
// Each entry states the assumption, what depends on it, and the ONE
// capture that would lift it — core/cat/ftdx10/doc.go's three-part form.
//
// # FAMILY entries (spec D5), this model's own rows
//
// THE `1A 00` READ-REQUEST FORM IS `1A 00 <addr>` WITH NO DATA — spec D5
// entry 1, matrix §3.11 (a); civ.Profile.BuildMemoryRead. No document in
// this tier prints the read request, only the answer and the set. WHAT
// DEPENDS ON IT: every memory read this driver makes, and therefore every
// clone. LIFTED BY: R9 — one `1A 00 01 00 0C` sent to a real radio, with
// the raw request and the raw answer captured.
//
// AN EMPTY CHANNEL ANSWERS `FA` — spec D5 entry 2(a), matrix §3.8(a);
// read.go's transport.ErrRejected branch. WHAT DEPENDS ON IT: whether an
// unwritten channel reads as an empty codeplug.Channel or as a failure
// that aborts the whole ReadAll. LIFTED BY: R11 — read a channel known to
// be unwritten and capture the answer verbatim.
//
// AN OCCUPIED-LOOKING RECORD OF `FF`s MEANS EMPTY — spec D5 entry 2(b),
// matrix §3.8(b); read.go's allFF branch, which runs BEFORE the record
// parser because civ.decodeRecord would reject 0xFF against this profile's
// enums and report a parse error instead. WHAT DEPENDS ON IT: whether a
// radio that answers this way reads as empty or as corrupt. LIFTED BY:
// R12 — the same capture as R11, read on a radio that answers a record
// rather than FA.
//
// THE MEMORY-NAME PAD BYTE IS `0x20` — spec D5 entry 3 (pad half), matrix
// §3.9; core/civ/ic9700's NamePad. WHAT DEPENDS ON IT: every name shorter
// than sixteen characters this driver writes. LIFTED BY: R13 — write a
// short name from the front panel and read the record back, recording the
// trailing bytes.
//
// `0x20` IS ACCEPTED IN A WRITTEN MEMORY NAME — spec D5 entry 3 (space
// half), matrix §3.9; the profile's charset, which admits the space the
// call-sign table prints and most Icom charset tables omit. WHAT DEPENDS
// ON IT: every multi-word channel name. LIFTED BY: W4 — write a name
// containing a space and read it back.
//
// THE DUPLICATED TX BLOCK IS MANDATORY ON WRITE — spec D5 entry 4, matrix
// §3.10; the layout maps ❺~❺❶ as repeats, so civ.encodeRecord writes both
// copies and "the driver always sends the full record" is true by
// construction. WHAT DEPENDS ON IT: whether a memory set is accepted at
// all. LIFTED BY: W2 — a set whose duplicated block deliberately disagrees
// with its primary, read back.
//
// WIRE ORDER IS DIAGRAM ORDER PAST THE DUPLICATED BLOCK — spec D5 entry 5,
// matrix §3.10; the layout's offsets. WHAT DEPENDS ON IT: every field
// after record offset 48. LIFTED BY: R20 — one occupied-channel read whose
// bytes are walked against the diagram.
//
// THE RECORD TOTAL LENGTH IS 111 BYTES — spec D5 entry 6, matrix §3.11;
// DERIVED, term by term, from the printed field widths. It is what the
// profile is BUILT from. WHAT DEPENDS ON IT: the probe's length
// fingerprint, and every set frame's width. LIFTED BY: R10 — one
// occupied-channel read, byte-counted.
//
// Kept EXPLICITLY DISTINCT from the driver entry
// `ic9700-record-length-observed` below (matrix erratum 10): the
// derivation is what the profile is built from, the observation is what
// the radio returns, and a disagreement between them is a finding that
// cannot exist if the two collapse into one entry.
//
// THE `19 00` REPLY VALUE — spec D5 entry 7, matrix §3.12; ic9700.go's
// probeIdentity, which RECORDS the token and matches it against nothing.
// The value is undocumented on all six models in this tier. WHAT DEPENDS
// ON IT: nothing, deliberately — and that is the entry's content. What
// identifies the radio is that an ADDRESS-MATCHED reply arrived. LIFTED
// BY: R6 — one `19 00` transaction, the answer's data bytes recorded.
//
// SERIAL FRAMING IS 8-N-1 — spec D5 entry 8, matrix §3.1; StopBits(). A
// full 28-page sweep of this guide found NO bit count, parity or stop-bit
// count for any port, and the "8 bit / 1 stop" lines Icom prints elsewhere
// are about the DATA/RTTY application port, which is not CI-V. WHAT
// DEPENDS ON IT: whether a session opens on a framing the radio can read
// at all. LIFTED BY: R1 — one `19 00` transaction at 8-N-1, then at
// 8-N-2; which framing draws an address-matched reply.
//
// THE TRANSCEIVE BROADCAST FORM IS `to=00` — spec D5 entry 9, matrix
// §3.5(b); the accumulator's address filter, which is what makes a
// broadcast flood invisible to the engine and visible only in
// AccumulatorStats.Unexpected. WHAT DEPENDS ON IT: the R9-SPLIT rule this
// driver implements at Init, and the diagnostics that report it. LIFTED
// BY: R5 — capture a transceive broadcast from a radio with transceive on
// and record its `to` byte.
//
// # `ic9700` DRIVER entries (matrix §5 rows 12–29)
//
// ic9700-transceive-factory-default — whether transceive is on out of the
// box, which decides how much traffic a session meets before it has asked
// anything. WHAT DEPENDS ON IT: how often the R9-SPLIT nonfatal path is
// taken in practice. LIFTED BY: R4 — power on a factory radio and record
// whether unsolicited frames appear.
//
// ic9700-factory-default-baud — caps.go's DefaultBaud, 19200. This guide
// DEFERS the factory rate to the instruction manual (PDF p.4 "Preparing"
// names a speed and points elsewhere), so 19200 is a choice among the six
// printed rates and not a reading. WHAT DEPENDS ON IT: whether a session
// opens at all on an untouched radio. LIFTED BY: R2 — `19 00` attempted at
// each printed rate on a factory radio; which rate answers.
//
// ic9700-baud-set-exhaustive — that the six printed rates are ALL of them.
// WHAT DEPENDS ON IT: caps.Bauds, and therefore what the UI offers.
// LIFTED BY: R18 — set each rate from the front panel and confirm the menu
// offers no seventh.
//
// ic9700-default-address-reachable — that a factory radio answers at A2.
// WHAT DEPENDS ON IT: the probe's whole identity step, which requires an
// address-matched reply. LIFTED BY: R3 — `19 00` to A2 on a factory radio.
//
// ic9700-echo-default-and-removal — whether this radio's CI-V jack echoes,
// and whether the accumulator's byte-identity suppression removes it. WHAT
// DEPENDS ON IT: whether a driver's own frame can be mistaken for an
// answer. LIFTED BY: R7 — send one frame and capture everything that comes
// back.
//
// ic9700-control-lines-inert-at-open — that asserting DTR or RTS at open
// does not key the transmitter. Matrix §3.2: `1A 05` items 0120–0122
// assign DTR/RTS to SEND or CW/RTTY keying, so asserting either COULD key
// it. core/transport.OpenSerial already drives both lines low at open
// (port.go), which matches the conservative reading; the residual gap is
// the window between the OS-level open and those two calls, which port.go
// records as an unverified hardware item. Nothing in this package can
// close it — the open path is internal/wiring's. WHAT DEPENDS ON IT:
// whether plugging in can transmit. LIFTED BY: R8 — open the port with a
// dummy load and a power meter attached.
//
// ic9700-accepted-ctcss-set — WHICH of the printed 0.1–299.9 Hz domain a
// real radio accepts. caps.go declares the whole printed range as a
// spec.ToneRange; the NARROWING is unknown. WHAT DEPENDS ON IT: whether a
// tone this driver calls valid is one the radio will store. LIFTED BY:
// R17 — set tones across the range from the front panel and read them
// back.
//
// ic9700-storable-frequency-bounds — caps.go's MinFreqHz and MaxFreqHz,
// taken from the BAND TABLE's edges rather than from the frequency field's
// arithmetic capacity. WHAT DEPENDS ON IT: which frequencies the write
// path refuses locally. LIFTED BY: R19 — store frequencies at and beyond
// each edge and read them back.
//
// ic9700-band-byte-selects-space — that ① selects a genuinely separate
// memory space per band, so 144-001 and 430-001 are different memories.
// WHAT DEPENDS ON IT: the whole slot spelling in slots.go, and therefore
// every slot name a codeplug file holds. LIFTED BY: R14 — store different
// contents at channel 1 of two bands and read both back.
//
// ic9700-scan-call-addressable — that 0100~0107 are readable and writable
// by the same `1A 00` form as 0001~0099. WHAT DEPENDS ON IT: the SCAN and
// CALL banks existing at all. LIFTED BY: R15 — read each of the eight
// addresses, recording each answering address's full data block and byte
// count.
//
// ic9700-1a07-set-only — that the memory-related `1A 07` form is set-only
// and has no read. WHAT DEPENDS ON IT: nothing this driver ships; it is
// recorded so a later milestone does not assume a read exists. LIFTED BY:
// R16 — attempt the read form and record the answer.
//
// ic9700-scan-call-not-clearable — that the printed clear form's
// 0001~0099 range means scan edges and call channels cannot be emptied.
// caps.go states it as NoBlank on those two banks. WHAT DEPENDS ON IT:
// whether a generic layer may plan an erase there. LIFTED BY: W6 — attempt
// a clear at 0100 and record the answer.
//
// ic9700-clear-form-accepted — that the printed clear form works. This
// tier SHIPS NO CLEAR: see "Erase is unshipped" below. WHAT DEPENDS ON IT:
// nothing today. LIFTED BY: W5 — send the clear form to a scratch channel.
//
// ic9700-write-ack — that a memory set is acknowledged with the six-byte
// `FB` and rejected with `FA`. write.go's ClassWriteWithAck exchange rests
// on it entirely. WHAT DEPENDS ON IT: whether a write reports success
// honestly, or reports a timeout for a write the radio accepted. LIFTED
// BY: W1 — one memory set to a scratch channel, everything that comes back
// captured.
//
// ic9700-short-record-rejected — that a set carrying fewer than 111 record
// bytes is refused rather than half-applied. WHAT DEPENDS ON IT: the
// safety of the whole write path if a frame is ever truncated in flight.
// LIFTED BY: W3 — send a deliberately short set to a scratch channel.
//
// ic9700-record-length-observed — that a real radio ANSWERS with 111
// record bytes. Distinct from FAMILY D5 entry 6 above, which is the
// DERIVATION: this is the observation, and a disagreement between them is
// a finding. WHAT DEPENDS ON IT: the probe's fingerprint being a real
// check rather than a tautology. LIFTED BY: R10 — one occupied-channel
// read, byte-counted.
//
// ic9700-band-mode-duplex-constraints — that the radio REACTS to an
// invalid mode/duplex/band combination as the manual's notes imply.
// write.go enforces all three constraints as pre-build refusals, so this
// entry lifts the REACTION and not the constraints, which are printed.
// WHAT DEPENDS ON IT: nothing this driver does; it would tell us whether
// the refusals are protecting the user from the radio or merely from
// themselves. LIFTED BY: R21 — attempt each excluded combination and
// record the answer.
//
// writeTrialsComplete IS FALSE FOR THE IC-9700 — caps.go. WHAT DEPENDS ON
// IT: every write column in both capability profiles. LIFTED BY: W7 — the
// whole Stage-W programme above, reviewed.
//
// # The FIVE entries this driver ADDS
//
// ic9700-mode-codes-are-hexadecimal — core/civ/ic9700's modeNames and
// caps.go's Modes. Matrix §1 #4 gives the mode set as wire 00, 01, 02, 03,
// 04, 05, 07, 08, 17, 22. For eight of the ten the base does not matter;
// for DV (17) and DD (22) it does — read as decimal they would be bytes
// 0x11 and 0x16. The document prints the glyphs, not the base. WHAT
// DEPENDS ON IT: every DV or DD channel this driver reads or writes, and
// the DD-only-in-band-3 refusal. LIFTED BY: R22 — one `1A 00` read of a
// channel set to DV from the front panel, recording byte ⑩.
//
// ic9700-unmapped-regions-refused — write.go's templateGuard. Fifty-two of
// the 111 record bytes have no civ.FieldID home (⑭ digital squelch, ㉔ DV
// code squelch, the three eight-byte call signs, and each of their copies
// inside the duplicated block), and civ.encodeRecord writes them from the
// layout's Fixed template. Under E6 a slot whose unmapped regions differ
// from the template is REFUSED, never rewritten. WHAT DEPENDS ON IT:
// whether this tier can write an IC-9700 channel without destroying data
// it cannot represent. LIFTED BY: W8 — one write-trial that sets a scratch
// channel's UR call sign from the front panel, writes the channel through
// this driver, and re-reads the unmapped regions.
//
// ic9700-unmapped-template-is-the-golden-state — core/civ/ic9700's
// fixedTemplateBytes. The template's values are taken from the frozen
// golden's record, so the ONE channel state this tier can write is the one
// leg G transcribed: UR `CQCQCQ` plus two pad spaces, blank R1 and R2, ⑭
// and ㉔ zero. `CQCQCQ` is the D-STAR broadcast destination and is the
// plausible factory state of an untouched memory, but THAT IS AN
// INFERENCE: no capture shows what a factory IC-9700 memory carries in
// those 52 bytes.
//
// THE ASYMMETRY IS WHAT MAKES THE INFERENCE SHIPPABLE. The guard compares
// and REFUSES on mismatch in every case, so if the golden state IS the
// factory state the guard admits the common channel and refuses the
// unusual one, and if it is NOT the guard simply refuses MORE channels.
// The failure mode of a wrong template is more refusals, never corruption
// — and the refusal happens before any frame is built, so no
// wrong-template outcome reaches a radio at all. WHAT DEPENDS ON IT: which
// real channels the E6 guard admits. LIFTED BY: R23 — one `1A 00` read of
// an untouched factory-default memory channel, recording the unmapped
// regions verbatim.
//
// ic9700-duplicate-block-agrees-on-read — core/civ/ic9700's layout, which
// maps the duplicated TX block's non-frequency fields as REPEATS of their
// primary FieldIDs, so civ.decodeRecord requires the two copies to AGREE
// and a disagreeing record fails to parse rather than letting one copy
// win. The manual's grey NOTE asserts the identity ("The same data as ⑤ ~
// 51 are stored in ❺ ~ 51"); no capture confirms it. WHAT DEPENDS ON IT:
// whether ReadAll succeeds on a radio whose Split settings differ between
// the blocks. LIFTED BY: W2 — a set whose duplicated block deliberately
// disagrees, read back.
//
// ic9700-no-documented-default-tone — write.go's CREATE refusal. Tier
// ruling T1(5) lets an empty-slot CREATE write a manual-DOCUMENTED default
// tone when ToneMode is Known OFF. THIS MANUAL DOCUMENTS NONE: PDF p.21's
// tone diagram prints digit ranges only, and leg G's 88.5 Hz is recorded
// in its own provenance as a CHOICE, not a printed default. A
// MANUAL-EVIDENCED ABSENCE. WHAT DEPENDS ON IT: whether a CREATE with
// unspecified tones can proceed at all — under T1(5) it is REFUSED naming
// the field. LIFTED BY: R24 — read a factory-default channel whose tone
// has never been set and record its tone spans verbatim.
//
// # The write guard
//
// writeTrialsComplete is FALSE. A RealHardware session therefore gets
// CapabilitiesUnverified, whose every write column is spec.Unverified and
// whose FieldSupport.CanWrite is false everywhere. The user's recorded
// consent (WithConsentedUnverifiedWrites) transforms that set at
// session-capability assembly and nowhere else, through
// spec.ConsentUnverifiedWrites — the project's one definition of what
// consent means. Consent widens what may be ATTEMPTED and is never
// evidence that it works: every permitted write still runs the clone
// layer's full write-then-verify pair.
//
// # Erase is unshipped, and the wire form EXISTS
//
// Unlike the Yaesu case, this is not an absence in the document. Matrix
// §3.13 records a printed clear form — `1A 00 <addr> FF` — and this tier
// deliberately does not ship it. No builder in core/civ names it,
// civ.Profile.AllowedCommand has no branch that could admit it,
// spec.FieldErase gets a zero spec.FieldSupport on every bank here,
// spec.ConsentUnverifiedWrites exempts erase structurally so no consent
// can mint one, and core/clone/execute.go's DiffErased branch therefore
// stays unreachable. WriteChannel refuses an empty codeplug.Channel
// outright, naming the reason.
//
// What a future write-trial milestone would need is four deliberate
// additions and not one: a builder, a gate branch, a golden vector, and a
// consent path — plus the W5 and W6 captures above.
//
// # Serial framing
//
// StopBits() returns 1, as spec D3.1 requires of every Icom driver, and
// the claim is ASSUMED (FAMILY D5 entry 8 above, lift R1). The reporter is
// on the DRIVER rather than the Session because internal/wiring holds the
// driver value before the port is opened.
//
// RECORDED AS HISTORY, NOT AS A DESIGN: before enabler E2 landed the
// optional interface, a session opened at transport.DefaultStopBits, which
// is 2. That was never a claim about this radio — it was the pre-Icom
// family's default reaching a family it was not written for.
//
// # Known costs, stated because they are the design
//
// THE TONE PICKER CANNOT OFFER THIS RADIO'S TONES. E3's own UI disposition:
// the picker is list-driven, so on a range-declaring model the grid shows
// and round-trips tones while the picker has no list to populate from. A
// Wave-4 item.
//
// FOUR KINDS OF CHANNEL ARE REFUSED ON WRITE, never corrupted: one whose
// DV call signs differ from the template's, one with digital squelch set,
// one in a SELECT-memory star group, and one set to RPS. E6 accepts
// exactly these costs across the tier. Each refusal names its reason and
// happens before any frame is built.
//
// A CREATE WITH UNSPECIFIED TONES IS REFUSED, because this manual
// documents no default tone to write
// (`ic9700-no-documented-default-tone`).
//
// SCAN SKIP IS NOT OFFERED AT ALL. Field ④ is a four-valued SELECT-memory
// group tag and the neutral field is a boolean; the dialect READS the
// value and this driver declines to present it as something it is not
// (OQ-4). The matrix's §2 `scan_skip` row reads Unver./Unver. on MEM,
// which this decision contradicts; an orchestrator erratum is owed and is
// flagged rather than performed here.
//
// # What this package does NOT claim
//
// CROSS-MODEL RECORD-LENGTH DISTINCTNESS. The probe refuses an unexpected
// record length WITHOUT naming a model: deciding that {111} identifies
// this radio rather than another would need a registry-wide table of every
// model's accepted lengths, which is a Wave-4 tier-level check. No
// driver.WrongRadioError is ever minted here.
//
// REGISTRATION. Nothing here adds a row to internal/wiring's driver table
// or to any guard's table; registration is a Wave-4 commit.
package ic9700

// SPDX-License-Identifier: GPL-3.0-or-later

// Package civ is the CI-V codec: the frame grammar, the packed-BCD
// helpers, the per-model Profile, the three builders this tier ships, the
// parsers, and the outbound gate that admits nothing else.
//
// It is core/cat's SIBLING, not its relative. CI-V is binary framing —
// FE FE <to> <from> <cn> [<sc>] <data> FD — with packed BCD, nibble-packed
// enums, unsolicited transceive broadcasts and bus echo; CAT is printable
// ASCII terminated by ';'. Nothing in core/cat is reused, in either
// direction, and internal/guards enforces that both ways
// (TestCATandCIVDoNotImportEachOther). Where the two packages share a
// SHAPE — Command's TOCTOU closure, the Dialect/Profile seam, the
// exhaustive flat config, the exported conformance suite — this package
// restates it rather than importing it, because the first shared helper
// is how a sibling becomes a dependency.
//
// # What this package does NOT do yet
//
// IT DOES NOT IMPORT core/transport, and that is deliberate. Spec D2
// generalises transport.Engine over a Framing seam, and the CI-V adapter
// for that seam belongs in this package — NewAccumulator, IsRejection,
// Allow, InitSequence, DrainPolicy, NoteSent. It is a SEPARATE FOLLOW-UP
// TASK, landing after the transport seam itself merges; every piece it
// needs is already here and exported:
//
//	Framing method     this package's piece
//	-----------------  -------------------------------------------
//	NewAccumulator     Profile.NewAccumulator / NewFrameAccumulator
//	IsRejection        IsRejection
//	Allow              Profile.AllowedCommand
//	InitSequence       EMPTY — see below
//	NoteSent           FrameAccumulator.NoteSent
//
// InitSequence is empty for CI-V, and that is a safety property rather
// than an omission (spec D2, adjudication 3): the CAT framing sends AI0;
// at Init, and the CI-V framing sends NOTHING. Transceive broadcasts are
// excluded structurally, by address matching in the accumulator, so this
// tier never writes a setting to anyone's radio to quieten the bus.
//
// # The error values
//
// This package mints its own ErrFrameTooLong, ErrRecordLength, ErrParse
// and ErrInvalidProfile, and deliberately mints NO ErrRejected. See
// errors.go: a rejection is a wire condition this package RECOGNISES, and
// which error value a rejected command surfaces as belongs to the engine
// that issued it — spec D2 puts that in core/transport. The framing
// adapter task reconciles the rest onto the transport's re-exported names.
//
// # Non-goals, and why they are absent rather than refused
//
// There is NO clear/erase builder, NO transceive-set builder and NO menu
// surface (1A 05) in this tier — spec D1, adjudications 3 and 19. Icom
// documents a clear form; this package does not build one, AllowedCommand
// has no branch that could admit one, and every Icom driver reports
// FieldErase unsupported. The guarantee is structural: a frame no builder
// can name is refused by construction rather than by a rule someone could
// later relax. What a future write-trial milestone would need is a
// builder, a gate branch, a golden vector and a consent path — four
// deliberate additions, not an oversight.
//
// # ASSUMED — the package-level register
//
// Every Icom byte this tier ships is manual-derived and labelled
// Unverified. This register carries the assumptions THIS PACKAGE makes as
// PACKAGE-LEVEL CONVENTIONS — facts it applies to every model, which no
// per-model table can override. The PER-MODEL registers (record geometry,
// name charset and pad byte, record lengths, default address, channel
// space) are the Wave 3 model packages' own, and each carries its own
// named lift on that model, in core/cat/ftdx10/doc.go's form.
//
// EIGHT members of this package are NOT manual facts. They are
// conventions this package applies to every Icom model in the tier, they
// are marked ASSUMED at the point of use, and every model package's own
// register sits beneath them rather than restating them. Each is listed
// here so the set has one statement of record, and each is lifted
// individually by a named capture, never wholesale.
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION. Every citation of this
// register elsewhere in the repository names the entry's title. The
// convention was adopted at M9d-2 (core/cat/ftdx10/doc.go carries the
// reasoning): a positional citation — "entry 6" — is correct only until
// somebody adds or reorders an entry, and it then silently points at the
// wrong assumption rather than failing. The same rule governs how this
// package cites the SPEC's register: D5 is a numbered list in the spec,
// so the number is its address, but every citation here names the
// subject beside it ("spec D5 entry 3, the name pad byte") so that a
// renumbered spec produces a visible contradiction instead of a silent
// misattribution.
//
// Each entry names the assumption, what depends on it, and the ONE
// capture that would lift it. "LIFTED BY" describes a capture on ANY
// model in the tier unless it says otherwise — these are conventions, so
// one radio's bytes settle them for the package, which is exactly why
// they are here rather than repeated six times. AN ENTRY THAT NAMES A
// MODEL MEANS IT: its claim is no wider than the capture named, which is
// the lift-mismatch rule applied to this register itself.
//
//   - THE CHANNEL NUMBER IS TWO PACKED-BCD BYTES, MOST SIGNIFICANT PAIR
//     FIRST (profile.go's encodeAddress). This entry claims the CHANNEL
//     FIELD and nothing else: the width of a grouped model's whole
//     address field belongs to A GROUPED MODEL'S ADDRESS FIELD IS THREE
//     BYTES below, because it is the group byte that makes the difference
//     and only a grouped model's capture can speak to it. Spec D5 entry
//     1, the `1A 00` read-request form, records the request as
//     undocumented, and the documents that draw it do not print the
//     address field's width or its digit order at all:
//     the two-byte big-endian reading is the one consistent with a
//     three-digit channel space (1..99 on the 7610, 0..99 per group on
//     the 705) and with the frequency fields' opposite convention being
//     remarked on where it appears. It is a deduction from the geometry,
//     not a byte any manual states.
//     WHAT DEPENDS ON IT: every memory read and every memory set this
//     tier sends, and the gate that admits them.
//     LIFTED BY: one `1A 00` read of a KNOWN channel — channel 12 rather
//     than channel 1, so a digit-order error is visible — with the raw
//     request and the raw answer captured as bytes. Any model in the tier
//     will do, the channel field being common to all of them. If the
//     answer echoes an address this package would not have built, the
//     encoding is wrong rather than merely unattested.
//
//   - A GROUPED MODEL'S ADDRESS FIELD IS THREE BYTES: ONE PACKED-BCD
//     GROUP BYTE BEFORE THE CHANNEL PAIR, AND GROUP AND BAND INDICES ARE
//     NUMBERED FROM 0 (profile.go's encodeAddress and
//     AddressForm.addressBytes; record.go's ChannelAddress;
//     profilevalidate.go's maxGroupCount). Profile.Groups is a COUNT.
//     The WIDTH is a deduction: a grouped space needs an index, one
//     packed-BCD byte holds it, and no document in this tier prints the
//     field. Zero-based is forced by arithmetic — the index is one
//     packed-BCD byte and the 705 and 905 have 100 groups each (spec D6),
//     which leaves no hundredth value if counting starts at 1 — but
//     "forced by arithmetic" is not the same as documented, and the ORDER
//     of the two components is a free choice this package has made.
//     WHAT DEPENDS ON IT: which channel a grouped or band-addressed model
//     reads and writes, and the frame length its own bound is checked
//     against.
//     LIFTED BY: ON THE IC-705 OR IC-905 — a FLAT model's capture cannot
//     speak to any of this — one `1A 00` read of a channel in a group
//     that is NOT the first, group 2 channel 3, with the raw bytes
//     captured. The field's length fixes the width; the group byte's
//     value fixes the numbering base; its position fixes the order. The
//     IC-9700's band form needs its own capture: nothing here has been
//     shown to carry from a group-addressed model to a band-addressed
//     one beyond the shared arithmetic.
//
//   - NO FRAMING BYTE APPEARS INSIDE A FRAME'S BODY (frame.go's
//     WellFormed; profilevalidate.go's refusal of 0xFE/0xFD in enum
//     values, name charsets and Fixed templates). CI-V reserves 0xFE and
//     0xFD for the preamble and terminator, and no data field carries
//     either. Packed BCD cannot produce them, and no documented enum
//     value in this tier reaches 0xFA and above — but that is an
//     observation about the values printed, not a rule any manual states.
//     WHAT DEPENDS ON IT: the injection defence. An interior 0xFD splits
//     one gate-approved buffer into two commands on the wire; an interior
//     0xFE lets a receiver resynchronise on the tail. Both are refused,
//     which means a model whose record genuinely contained one would be
//     unrepresentable here — a loud failure at profile construction, not
//     a silent misread.
//     LIFTED BY: any captured record whose bytes include 0xFE or 0xFD in
//     a data position. That would be a STOP-and-respec, not a parameter
//     change: the whole frame grammar would need an escape convention.
//
//   - THE PREAMBLE'S LENGTH CARRIES NO MEANING (accumulator.go). A radio
//     may send more than two 0xFE bytes as padding, and the accumulator
//     NORMALISES every received frame to exactly two before returning it.
//     Padding tolerance is spec D1's; the normalisation is this package's
//     decision, and it assumes a three-preamble frame and a
//     two-preamble frame with identical bodies are the same frame.
//     WHAT DEPENDS ON IT: echo comparison (a padded echo must still match
//     the canonical frame this package sent), and any answer matcher the
//     framing adapter builds on frame offsets.
//     LIFTED BY: a capture in which a radio's padding VARIES between two
//     frames carrying the same body. That would confirm the padding is
//     noise, as assumed. A radio whose padding was load-bearing would
//     show it by answering differently to the two forms.
//
//   - THE FA AND FB FRAMES ARE EXACTLY SIX BYTES (frame.go's IsRejection
//     and IsAcknowledgement). A rejection or an acknowledgement is
//     FE FE <to> <from> FA|FB FD and carries no data.
//     WHAT DEPENDS ON IT: whether a write's outcome is reported as
//     accepted, refused, or unexpected traffic. An FA-shaped frame with
//     data is deliberately NONE of the three: calling it a rejection
//     would attribute a refusal to the radio on evidence this package
//     does not have.
//     LIFTED BY: one memory set the radio ACCEPTS and one it REFUSES —
//     an out-of-range channel will do — with both answers captured whole.
//
//   - THE `19 00` ANSWER CARRIES AT LEAST ONE DATA BYTE (parse.go's
//     ParseTransceiverID). Spec D5 entry 7 records the reply VALUE as
//     undocumented on all six models, and this package accordingly never
//     matches it — but it does assume there IS one, and refuses an
//     answer with an empty body.
//     WHAT DEPENDS ON IT: the probe's identity step (spec D3.2), which
//     requires an address-matched reply and records the token as a
//     diagnostic. A radio answering `19 00` with an empty body would be
//     refused here rather than opening an unfingerprinted session.
//     LIFTED BY: one `19 00` exchange on any model, raw bytes captured.
//     The token is what the register wants recorded, whatever it is.
//
//   - THE MEMORY ANSWER REPEATS THE ADDRESS BEFORE THE RECORD (parse.go's
//     ParseMemoryAnswer). The answer to `1A 00 <addr>` is
//     `1A 00 <addr> <record>`, the same address field the request
//     carried, in the same encoding.
//     WHAT DEPENDS ON IT: which channel a decoded record is attributed
//     to, and — through the record's LENGTH, which this package derives
//     by subtracting the address field — the probe's length fingerprint
//     (spec D3.2). An answer omitting the address would make every record
//     read as the wrong length and every channel as the wrong channel.
//     LIFTED BY: the same single `1A 00` read the address-encoding entry
//     asks for. One capture settles both.
//
//   - A TRAILING PAD BYTE IN A NAME IS PADDING (recordcodec.go's
//     EncodingName decode). A name is trimmed of trailing pad bytes on
//     the way in and padded to the field width on the way out, so a name
//     that genuinely ENDS in the pad byte does not round-trip. On most
//     models the pad byte is the space, which is also a legal name
//     character (spec D5 entry 3, the name pad byte), so this is a real
//     loss rather than a theoretical one — and it is stated here rather
//     than hidden, because padding erases the data-versus-fill
//     distinction on the wire and no decoder can recover it.
//     WHAT DEPENDS ON IT: what a channel called "CALL " comes back as.
//     LIFTED BY: one capture of a channel whose name is set, through the
//     radio's own front panel, to end in a space. If the field comes back
//     distinguishable from the same name without it — a length byte, a
//     terminator, a different fill — then the pad convention is richer
//     than this package models and EncodingName gains a variant.
//
// # Where the rest of the assumptions live
//
// Everything else is per-model and belongs to a per-model register:
// default CI-V address, channel space, record length(s) and the
// discriminator between them, every field's offset, width, nibble and
// encoding, every enum's wire values, the name length, charset and pad
// byte, and the serial framing (spec D3.1). A Profile is refused at
// construction if any of it is internally inconsistent — profilevalidate.go
// has nine rules and names the offending field and value in every one —
// but validation cannot tell a self-consistent transcription from a
// correct one. That is what the per-model evidence chain and the consent
// gate are for.
package civ

// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic705 is the Icom IC-705's driver: the probe, the read path, the
// acknowledged write, and the capability data that says what this project
// believes the radio can do. It builds on core/civ/ic705's dialect (the
// `1A 00` memory record) and on core/civ's own CI-V framing adapter; it
// contains no frame machinery of its own.
//
// # Provenance
//
// Everything here comes from the Icom IC-705 CI-V REFERENCE GUIDE,
// revision A7560-8EX-6 (© 2020–2023 Icom Inc., Jan. 2023), 31 PDF pages,
// SHA-256 36876db53a4dec7a9d74133ac4546bd161bcb6d56ee7c79668ff00cf1f92ea9c,
// at docs/fixtures-private/manuals/ic705_civ_rev6.pdf — gitignored, so
// every citation below is a citation and not a link. The folio offset is
// printed page = PDF page − 1 (corrected 23/08/2026,
// docs/fixtures-private/manuals/ic705-manual-provenance.md §Erratum), and
// every page citation gives the PDF page first with the folio in brackets.
// The capability grid's evidence of record is
// docs/superpowers/icom-matrices/ic705-capability-matrix.md (rev 1 +
// errata 1–20 — errata 16–20 were adjudicated by the orchestrator on
// 24/08/2026, and they are the five this plan itself proposed); where its
// body and an erratum disagree, THE ERRATUM STANDS.
//
// NO IC-705 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT — not a byte
// sent, not a byte received. Every statement here is a reading of a
// document, which is why the register below exists and why each entry
// names the ONE hardware observation that would lift it.
//
// # The Basic Manual's narrow admission
//
// docs/fixtures-private/manuals/ic705_basic_rev9.pdf (IC-705 BASIC MANUAL,
// revision 9, Dec. 2025, 100 pages, SHA-256
// 96c8e0c790f244c9de80ed81032ec4af69d1498f77c6185ca351f268f0941497) is
// admitted for THREE VALUES ONLY (tier ruling R14) — the transceive
// default, the USB echo-back default and the CI-V baud default — and for
// nothing else. THE CI-V REFERENCE GUIDE REMAINS THE SOLE LAYOUT
// AUTHORITY: not one byte position, width, enum or record length may come
// from the Basic Manual. What the page images settled, read off 300 dpi
// renders rather than off extracted text:
//
//   - CI-V Transceive — Default: ON. PDF p.69, printed folio 8-16,
//     §8 SET MODE, "MENU » SET > Connectors > CI-V", heading
//     "CI-V Transceive (Default: ON)". THE BREADCRUMB IS THE CI-V ONE, not
//     the USB SEND/Keying one at the head of the same page: that box
//     governs the three USB SEND/Keying items in the left column, while the
//     CI-V box at the foot of the left column opens the group that runs
//     CI-V Address → CI-V Transceive → CI-V USB Echo Back, continuing at
//     the head of the right column. It is the component a capture would
//     navigate by, so it is stated exactly. This RE-GRADES
//     ic705-transceive-factory-default from ASSUMED to MANUAL-EVIDENCED,
//     and it is the reason this driver treats a line that never goes quiet
//     at Init as a normal operating state.
//   - CI-V USB Echo Back — Default: OFF. Same page, folio and CI-V
//     breadcrumb, heading "CI-V USB Echo Back (Default: OFF)". This
//     re-grades
//     ic705-usb-echo-back-default. The adapter's echo suppression is
//     unaffected either way: it matches recorded bytes, so an echo that
//     never arrives costs nothing.
//   - THE BAUD DEFAULT IS NOT SETTLED, and what the images settle instead
//     is a NEGATIVE: this radio's CI-V port is a microUSB CDC interface,
//     and the manual states "You can communicate regardless of the PC
//     software's baud rate setting" (PDF p.92, printed folio 13-2,
//     §13 CONNECTOR INFORMATION, [microUSB] › USB Serial Port). No default
//     rate is printed anywhere, so ic705-default-baud and ic705-baud-list
//     stay ASSUMED with lifts L-BAUD and L-BAUDLIST. Admission is
//     permission to look, not permission to assume.
//
// The same CI-V group settles, in passing, what the CI-V guide already
// says: "CI-V Address (Default: A4)", with the note "'A4' is the default
// address of the IC-705".
//
// # The write guard
//
// writeTrialsComplete is FALSE, and while it is false there is no
// hardware-verified capability profile at all: a RealHardware session gets
// capabilitiesUnverified, every field Read Unverified / Write Unverified,
// spec.FieldSupport.CanWrite false, and every write refused — at
// codeplug.Diff, at the clone service, and again in WriteChannel. The ONE
// route past it is the user's own consent
// (WithConsentedUnverifiedWrites), applied per session and never to the
// static baseline.
//
// FLIPPING IT IS A FOUR-PART CHANGE, deliberately: the constant, AND a
// capabilitiesRealHardware built field class by field class from trial
// evidence, AND the Capabilities switch that selects it, AND the pin test
// that holds the line down. NO PRODUCTION CODE READS THE CONSTANT, so a
// one-character edit unlocks nothing on its own.
//
// # The two slot namespaces
//
//	| Bank | Display strings     | Wire                      |
//	| MEM  | G01-001 … G100-100  | group 0-99,  channel 0-99 |
//	| CALL | G101-001 … G101-004 | group 100,   channel 0-3  |
//
// ONE RULE, BOTH BANKS: wire = display − 1, for group and channel alike.
// G101 lies outside MEM's addressable space (spec.Bank.WithinSpace
// requires group ≤ Groups, and MEM declares 100), so NO STRING CAN NAME
// TWO BANKS — which matters because codeplug.Channel carries no bank
// identifier and codeplug resolves a slot to a bank by linear scan, so a
// colliding string would resolve silently to whichever bank came first.
// TestSlotNamespacesCannotCollide walks all 10 004 display slots rather
// than sampling them, because injectivity is not a property a sample can
// establish.
//
// The radio's OWN printed numbering — group 0100, channels 0000-0003, and
// the front-panel labels 144 C1/C2 and 430 C1/C2 (matrix §1b) — is DISPLAY
// COSMETICS, deferred per spec D4 adjudication 14. A per-model display
// mapping is a later milestone's work; nothing here invents one.
//
// # The seven costs a user meets as refusals
//
// A user meets each of these as a refusal, and deserves to find it written
// down:
//
//  1. A channel carrying D-STAR ROUTING, a DIGITAL-SQUELCH setting, SPLIT
//     ON, or a ★1/★2/★3 SELECT MARKING is REFUSED, never rewritten
//     (enabler E6; O-2; O-6). The whole 115-byte data area goes out on
//     every memory set, and no spec.Field claims those 53 bytes, so a
//     write would overwrite them with this profile's Fixed template.
//     Refusing is the only honest option: this driver refuses, it never
//     corrupts.
//  2. A CREATE requires EXPLICIT VALUES for every mapped field (R6). A
//     create carrying only a frequency, a mode and a name is refused with
//     the missing fields named — not quietly completed with zeros, which
//     would write a tone, a filter and a duplex the caller never chose.
//  3. A channel whose DUPLICATED TX BLOCK DISAGREES with its RX fields
//     fails to parse (spec D5 entry 4), so ReadChannel errors and
//     core/clone's ReadAll ABORTS. One exotic split channel — the case the
//     TX block exists for — makes the radio UNCLONEABLE until it is
//     changed, not merely unwritable. Softening this to an empty channel
//     with a diagnostic is a defensible future decision; it is a
//     DELIBERATE one, and TestASplitChannelWhoseTXBlockDisagreesFailsHonestly
//     is the failing test that would have to be updated to make it.
//  4. THE TONE PICKER IS LIST-DRIVEN, so on this model the grid shows and
//     round-trips tones but the picker cannot offer them: this radio
//     declares a numeric RANGE rather than a chart (enabler E3's recorded
//     cost, a Wave-4 honesty row).
//  5. A CREATE whose tone fields are not Known is REFUSED (O-11). There is
//     no prior record to preserve a tone from, and this radio's CI-V
//     Reference Guide prints NO DEFAULT TONE anywhere — not in the `1A 00`
//     legend (PDF p.19, folio 18), not in `• Repeater tone/tone squelch
//     frequency settings` (PDF p.23, folio 22), and matrix erratum 4
//     established that this document marks no factory defaults at all. NO
//     ic705-default-tone REGISTER ENTRY EXISTS, and that is deliberate:
//     refusing is the ABSENCE of an assumption, and filing one would make
//     a claim this driver does not make.
//  6. An ADD to a slot the bounded inventory walk NEVER VISITED is REFUSED
//     if the pre-write read finds a record there (ruling T3), naming
//     WithFullInventoryWalk() as the remedy. This is the price of a
//     bounded default walk, and it is paid in refusals rather than in
//     overwritten channels.
//  7. A memory answer whose channel address is not the one requested fails
//     that read with ErrAnswerMismatch (ruling T2) rather than being
//     silently relabelled. The landed memory-answer matcher is
//     ENVELOPE-ONLY by decision, so this check is the driver's or nobody's.
//
// # The residual gate width (O-9, RULED 24/08/2026)
//
// core/civ carries ONE channel range per profile, and this profile's is
// 0..99 — so civ.Profile.AllowedCommand ADMITS a CALL-group address with a
// channel of 4..99, although the manual documents only 0000-0003. That is
// a RECORDED property of this driver, not an undocumented hole, and three
// things carry it:
//
//   - slotToAddress REFUSES those addresses FIRST, before any builder is
//     reached, and TestCallChannelsAboveFourAreRefusedBeforeAnyBuilder
//     SWEEPS the whole range rather than sampling it;
//   - WriteChannel's rung 2 bank check protects every write, pinned by
//     TestEveryLocalRefusalPrecedesTheRead's "rung 2 bank check" case,
//     which sends zero frames. That case has to take the CALL bank away
//     from a session to reach the rung at all — in the shipped
//     configuration rungs 1 and 2 agree by construction, because
//     slotToAddress's space IS the two banks — and it is worth having
//     because the CAPABILITY SET, not slots.go, is what every other layer
//     of this project enforces against;
//   - this paragraph records the width.
//
// Narrowing the profile's ChannelHi to 3 instead would make 96 of every
// MEMORY group's channels unaddressable, and enabler E4 deliberately did
// not grow a per-group cap mid-implementation. The width is bounded (96
// addresses inside one group, on a command this driver cannot be made to
// emit), argued, and covered twice over — which is spec Erratum 2's own
// standard for a deliberate gate width. A per-group cap becomes a
// post-Wave-3 enabler follow-up IF HARDWARE OR USAGE SHOWS THE NEED.
//
// # The control-line policy — RECORDED, NOT CHANGED
//
// core/transport.OpenSerial drives RTS and DTR LOW at open (safety
// obligation 4, core/transport/port.go). On this radio that is not a
// neutral default: matrix §3.2 evidences that `1A 05 0125/0126/0127` let a
// user bind SEND (PTT) or CW/RTTY keying to DTR or RTS ON THE VERY CDC
// PORT THIS PROJECT OPENS (PDF p.9). Asserting either line at open could
// key the transmitter of a radio so configured. The existing LOW-at-open
// behaviour is therefore correct for this radio and is RECORDED here
// rather than changed. The factory value of those three settings is
// ASSUMED — ic705-usb-send-keying-default, lift L-CTRLLINES.
//
// # Erase — the wire forms exist here, and are shipped nowhere
//
// Unlike some models in this tier, this radio documents TWO ways to clear
// a memory (matrix §3.13): a `1A 00` set carrying FF at the fifth data
// position, and command `0B`. NEITHER is built by core/civ, admitted by
// its gate, or reachable from any capability label — spec.FieldErase is
// Unsupported on both banks, spec.ConsentUnverifiedWrites structurally
// exempts it, and WriteChannel refuses an empty channel at rung 3. That is
// spec D4 adjudication 19: no erase path at all this tier. A future
// write-trial milestone would need matrix §3.13's five-step programme
// before any of it changed.
//
// # The tone domain (O-12)
//
// ENCODABLE 0–2999 deciHz — the printed digit ranges (PDF p.23, folio 22:
// "100Hz digit: 0 ~ 2", then "0 ~ 9" three times). DECLARED
// {MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1}. The gap is EXACTLY THE
// SINGLE VALUE ZERO, which the radio uses for "no tone set" and which
// spec.Validate refuses to admit as a declared minimum. Ruling T1 routes
// that one value around the vocabulary rather than through it: a read maps
// it to Unknown (read.go), and a write copies the JUST-READ record's own
// number back out verbatim (write.go rung 10), which is preservation of
// the radio's value rather than synthesis of a new one. A create has no
// prior record and is refused — see cost 5.
//
// # The two printed ceilings, and why they are the driver's business
//
// Two of this record's fields are bounded more tightly by the manual's own
// printed DIGIT LEADERS than by the byte width core/civ can check, so the
// refusals live in this driver's write ladder (rung 7) and nowhere else:
//
//   - FREQUENCY: "1 GHz digit: (fixed)" and "100 MHz digit: 0 ~ 4" (PDF
//     p.18, folio 17; matrix ERRATUM 7). A frequency at or above 500 MHz is
//     refused, on FreqHz and TxFreqHz alike. core/civ cannot catch it —
//     500,000,000 fits five packed-BCD bytes perfectly well — so without
//     this rung a consented write would put a 5 in a digit the manual
//     bounds at 4.
//   - OFFSET: the three-byte field has a FIXED 10 MHz digit (same page;
//     matrix ERRATUM 8 — the ceiling is 9.9999 MHz, not 9.99). An offset
//     above 9,999,900 Hz is refused.
//
// Both are locally decidable, so both precede all wire traffic, and both
// are tested AT THE BOUNDARY (499,999,999 and 9,999,900 accepted;
// 500,000,000 and 10,000,000 refused) rather than at a comfortable
// distance from it.
//
// # The ELEVEN D5 family register entries
//
// These are the tier-wide assumptions this model inherits, each with the
// one IC-705 observation that would lift it:
//
//   - D5 1 — the `1A 00` READ-REQUEST FORM (address, no record). No
//     document in this tier prints it. Lift L-READFORM: send it to a real
//     IC-705 and record the answer.
//   - D5 2(a) — an EMPTY CHANNEL ANSWERS FA. Lift L-EMPTY-FA: read a
//     channel known to be unwritten and record the reply.
//   - D5 2(b) — an ALL-0xFF RECORD MEANS EMPTY. Lift L-EMPTY-FF: as
//     above, on a radio that answers with a record rather than FA.
//   - D5 3 — NAME-SPACE HANDLING (0x20 is a legal name byte). Lift
//     L-NAME-SPACE: store a name containing a space and read it back.
//   - D5 3, second limb — THE NAME PAD BYTE is 0x20. Lift L-NAME-PAD:
//     store a short name and record the trailing bytes.
//   - D5 4 — THE DUPLICATED TX BLOCK IS MANDATORY ON WRITE. Lift
//     L-TXBLOCK-SHORT: send a set whose block disagrees and record whether
//     the radio accepts it.
//   - D5 5 — WIRE ORDER IS DIAGRAM ORDER past the duplicated block; the
//     printed indices are LOGICAL (matrix erratum 10). Lift L-RECLEN.
//   - D5 6 — THE RECORD'S TOTAL LENGTH is 111 bytes, derived and never
//     printed. Lift L-RECLEN: read one channel and count.
//   - D5 7 — THE `19 00` REPLY VALUE is undocumented, so it is RECORDED
//     AND NEVER MATCHED. Lift L-IDTOKEN: read it once and record it.
//   - D5 8 — SERIAL FRAMING (one stop bit). Lift L-FRAMING; see StopBits.
//   - D5 9 — THE TRANSCEIVE BROADCAST FORM is addressed to 0x00. Lift
//     L-BROADCAST-FORM: capture one with transceive on.
//
// And the model-specific one the tier's D5 table leaves per model:
// OVER-BUDGET BEHAVIOUR — what an IC-705 does when asked to store more
// than its budget. Lift L-OVERBUDGET. This driver never finds out: the
// budget is enforced at codeplug.Diff time and NEVER on the wire.
//
// # The driver's own ASSUMED register
//
//   - ic705-group-budget — 500 populated channels across the sparse memory
//     space. Lift L-BUDGET-CEILING: fill the radio and record where it
//     stops.
//   - ic705-call-channel-emptiness — that a call channel can never be
//     empty (the CALL bank's NoBlank). Lift L-CALL-EMPTY: try to clear one
//     from the front panel.
//   - ic705-min-storable-frequency and ic705-max-storable-frequency — the
//     radio's own tuning floor and ceiling, which the matrix leaves
//     unfilled (§1 rows 11-12). BOTH BACK ZEROED CAPABILITY FIELDS in the
//     deliberatelyZero audit (caps_test.go): the record field's ENCODING
//     range is not the radio's storable range, and filling one with the
//     other would be a widening. Lifts L-FREQ-FLOOR and L-FREQ-CEIL: store
//     the lowest and highest frequencies the radio accepts.
//   - ic705-ctcss-selectable-set — the tones the radio actually offers,
//     inside the declared 0.1–299.9 Hz domain. Lift L-CTCSS-SET: step the
//     tone item through its positions and record what appears.
//   - ic705-dtcs-selectable-set — likewise for DTCS, inside the 512-code
//     printed domain. Lift L-DTCS-SET. The TABLE ITSELF stays explicit and
//     complete (enabler E3; plan O-10, dispute SUSTAINED 24/08/2026): an
//     empty table fails closed on every Known code, which would make this
//     radio's DTCS channels unreadable rather than merely unverified.
//   - ic705-usb-send-keying-default — the factory value of `1A 05
//     0125/0126/0127`. Lift L-CTRLLINES; see the control-line policy above.
//   - ic705-select-memory-mapping — that the ★n nibble at record offset 0
//     is SELECT-SCAN GROUP MEMBERSHIP rather than a skip flag. Lift
//     L-SELECT-SCAN: mark a channel ★1 from the front panel and record the
//     byte. THIS DRIVER AGREES WITH THE MATRIX, and it took an erratum to
//     get there: §2 originally graded scan_skip Supported, its own A2
//     called the mapping "a live question for the plan", the plan answered
//     it (O-6), and MATRIX ERRATUM 17 (adjudicated 24/08/2026) re-graded
//     the row Unsupported on both banks with the ★n nibble recorded as an
//     unmapped record area. So spec.FieldScanSkip is Unsupported on both
//     banks here, ReadChannel reports ScanSkip Unavailable, and a
//     ★-marked channel is REFUSED on write, never demoted — which is now
//     the matrix's own position rather than a divergence from it.
//   - ic705-transceive-factory-default (L-TRANSCEIVE-DEFAULT) and
//     ic705-usb-echo-back-default (L-ECHO) — RE-GRADED to MANUAL-EVIDENCED
//     by the Basic Manual's admission above; the lifts remain named so a
//     capture can still confirm them on the wire.
//   - ic705-default-baud (L-BAUD) and ic705-baud-list (L-BAUDLIST) — still
//     ASSUMED; see the negative finding above.
//   - ic705-dv-mode-code (L-DV-MODE), ic705-unmapped-record-areas
//     (L-UNMAPPED) and ic705-blank-callsign-byte (L-CALLSIGN-BLANK) — the
//     model package's own entries, restated here because this driver's
//     refusals depend on them: the unmapped inventory is what rung 11
//     compares, and the blank call-sign byte is what makes an ordinary
//     channel writable at all.
//
// # What is NOT here
//
//   - NO REGISTRATION. internal/wiring, internal/guards and
//     internal/radiotext are untouched this wave; registration is a
//     Wave-4 commit.
//   - NO ReadAll. Reading a whole radio is core/clone's loop over the
//     slots Open's inventory walk materialised (inventory.go).
//   - NO CROSS-MODEL LENGTH TABLE. A foreign record length is refused with
//     GotModel EMPTY, because naming the model it belongs to would need
//     the other five models' length sets — a TIER-level Wave-4 check this
//     driver must not claim.
//   - NO --civ-address FLAG, ever, this tier (spec D3.3). An IC-705 moved
//     off address A4h is unreachable by this program, and the README says
//     so.
package ic705

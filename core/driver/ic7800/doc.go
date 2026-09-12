// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7800 implements driver.Driver for the Icom IC-7800, over the
// dialect core/civ/ic7800 declares. It owns probing, capabilities and
// write policy; the wire geometry lives next door.
//
// # Shape
//
// This driver is a literal copy of core/driver/ic7610's mechanics — Open's
// choreography, the occupied-slot probe, the T2/T4 refusal rules, the E6
// unmapped-nibble refusal — with only the model name, the CI-V address
// (6Ah, not 98h) and the evidence citations changed. spec.md §1 states the
// IC-7800 needs "no offset changes" relative to the IC-7610, and nothing
// in this package's own read/write mechanics is model-specific either: the
// tier rulings (E6, T2, T4, D3.2, D3.3) apply identically.
//
// # StopBits — ASSUMED, on no evidence from this document
//
// The IC-7800 manual's 230 pages say nothing about serial framing for the
// CI-V link: the words "stop bit", "parity" and "8 bit" do not appear
// against the [REMOTE] jack or the USB CI-V endpoint (the manual instead
// prints a "Decode Baud Rate" row for the internal RTTY/PSK DECODER, three
// rows below the CI-V rate item, which must never be read as a framing
// statement — the same trap the IC-7610's own D5 register names). Register
// home: ic7800-framing-8n1, ASSUMED 8-N-1 by tier convention (spec D3.1).
// LIFT: with an IC-7800 at factory CI-V settings, open the [REMOTE] jack at
// 8-N-1 and then at 8-N-2, send FE FE 6A E0 19 00 FD at each, and record
// which framing returns a well-formed address-matched reply.
//
// # Default CI-V baud — ASSUMED numeric resolution of a printed "Auto"
//
// PDF p.177 (folio 12-21) prints: "Sets the CI-V data transfer rate. 300,
// 1200, 4800, 9600, 19200 bps and 'Auto' are available. (default: Auto)."
// "Auto" is a negotiation mode, not a number internal/wiring can open a
// port at, so this driver's capabilitiesUnverified/Simulated resolve it to
// 19200 bps — the same arbitrary choice the IC-7610's own register records
// for its own six-rate list, chosen for the same reason: a wrong guess
// costs a clean timeout at Open (an address-matched 19 00 reply is
// required and silence is silence), never a wrong byte. Register home:
// ic7800-default-baud-resolution. LIFT: on an IC-7800 taken to a factory
// reset, read the CI-V Baud Rate item from the front panel and photograph
// it.
//
// # Erase — the wire form exists; this tier never sends it
//
// PDF p.211 documents a clear as the channel number followed by FF in
// place of the data bytes, and PDF p.201's command table separately lists
// command 0B, "Memory clear". Neither is ever built or admitted: FieldErase
// carries the zero FieldSupport in every capability profile this driver
// returns, and spec.ConsentUnverifiedWrites structurally never consents it
// (its `f != FieldErase` guard). core/clone/execute.go's DiffErased branch
// is unreachable for this model.
//
// # Register entries homed here
//
// The tier-wide rulings (E6, T2, T4, D3.2, D3.3) and the assumptions
// core/civ/ic7800/doc.go's register carries are not repeated here. This
// package additionally owns:
//
//   - ic7800-framing-8n1 (above);
//   - ic7800-default-baud-resolution (above);
//   - ic7800-control-lines-inert — that RTS and DTR deasserted neither key
//     the radio nor block CI-V. ASSUMED, on the same reasoning as every
//     other CI-V driver in this tier: the [REMOTE] jack carries no RTS/DTR
//     pins on this model at all (a 2-conductor 3.5 mm mini-jack, matrix
//     "Notable structural findings"), so the question is closer to moot
//     here than on the IC-7610's DATA port. LIFT: with an IC-7800
//     connected, deassert RTS and DTR, send FE FE 6A E0 19 00 FD, and
//     record whether an address-matched reply still arrives.
//   - ic7800-storable-frequency-ceiling / -floor — the ENCODABLE bound
//     (MaxEncodableFreqHz, from the record's five-byte BCD span) is what
//     this package declares; whether a real IC-7800 STORES every value up
//     to 69 999 999 Hz, or refuses above its printed 60 MHz receiver
//     ceiling (PDF p.214), is unestablished. LIFT: attempt to write a
//     frequency between 60 000 000 and 69 999 999 Hz and record whether
//     the radio accepts or refuses it.
package ic7800

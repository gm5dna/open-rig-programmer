// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7760 describes the IC-7760 CI-V memory dialect.
//
// The profile uses a two-byte flat packed-BCD address. Its record-only length
// is 25 bytes; the 27-byte data-area convention includes those two address
// bytes. The printed “Memory group number” is ruled to be the channel number
// by E1, and the printed MEM 01–99 plus SCAN P1/P2 inventory is accepted by E2.
// The select byte is SELECT membership (OFF, ★1, ★2, ★3), not a binary scan
// skip field; P1/P2 writes require its fixed zero value.
//
// Values absent from the manual remain assumptions and name their future
// lifts: ic7760-name-space-code, ic7760-name-pad-byte, ic7760-tone-domain,
// ic7760-record-length, and the Stage R framing/control/baud/address,
// transceive, broadcast, echo, read-form, empty-reply and USB-B registers.
// No registration, transport, erase, or unsupported command is provided here.
package ic7760

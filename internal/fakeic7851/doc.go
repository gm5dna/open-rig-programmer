// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeic7851 is an independent, stdlib-only IC-7851/IC-7850 CI-V
// simulator. Its wire register is the manual-derived B/W record: 2 selector
// bytes followed by 25 bytes (③, ④–⑧, ⑨–⑩, ⑪, ⑫–⑭, ⑮–⑰, ⑱–㉗).
// The two selector bytes are packed BCD over the complete flat space B row D1
// prints: 0001–0099 are memory channels 1 to 99, 0100 is programmed scan edge
// P1 and 0101 is P2, so SetSlot("P1") and a read of 0100 name one record
// (TestScanEdgeSelectorsAddressP1AndP2). The radio answers only frames
// addressed to it from the controller at 0xE0 and stays silent otherwise
// (TestOnlyTheControllerIsAnswered); every reply is framed from the address
// WithRadioAddress configured, never a literal 8E
// (TestMovedRadioAddressFramesEveryReply).
// Empty reads use FA by default (ASSUMED: ic7851-empty-reply-fa); optionally
// all-FF records model the alternative empty convention (ic7851-all-ff-record).
// WithUSBEcho enables exact line echo (ic7851-echo-link-to-remote), while the
// two flood options expose the assumed broadcast destination and synthetic
// controller-addressed traffic (ic7851-broadcast-address-form). Short sets
// are refused by default; WithShortSetAcknowledgement exposes the open edge
// under ic7851-write-ack-fb. TestNoCoreImports pins the stdlib-only fence,
// and the package tests pin the wire grammar independently of these builders.
package fakeic7851

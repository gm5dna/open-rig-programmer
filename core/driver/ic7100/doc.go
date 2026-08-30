// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7100 implements safe IC-7100 memory cloning over CI-V.
//
// The driver is manual-derived and unverified on hardware. RealHardware is
// therefore fail-closed unless the user explicitly consents to unverified
// writes. Erase is never consented, writes are never retransmitted, and Open
// sends no mutation. The package is deliberately not registered here.
//
// CI-V serial framing is not reported. The sole 8-N-1 sentence in the manual
// is PDF p.174's DV low-speed data application, not the CI-V/REMOTE link;
// TestNewProfilesFailSafeAndDoNotExposeSerialFraming pins that distinction.
package ic7100

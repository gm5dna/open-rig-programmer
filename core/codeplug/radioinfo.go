// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import "time"

// RadioInfo describes the radio a Codeplug's Channels were read from (or
// are destined for), and when.
type RadioInfo struct {
	// Model is the radio's display name, e.g. "FT-710".
	Model string `json:"model"`
	// CATID is the radio's 4-character CAT ID answer, e.g. "0800".
	CATID string `json:"cat_id"`
	// ReadAt is when this data was read from the radio.
	ReadAt time.Time `json:"read_at"`
	// Port is the serial port the radio was connected on, e.g.
	// "/dev/cu.usbserial-XXX".
	Port string `json:"port,omitempty"`
	// USBSerial is the USB device serial number, when known.
	USBSerial string `json:"usb_serial,omitempty"`
	// FirmwareConfirmed is the firmware version the user has manually
	// entered after reading it off the radio's front panel — CAT has no
	// command to read this, so it can only ever come from the user.
	FirmwareConfirmed string `json:"firmware_confirmed,omitempty"`
	// Region is the regulatory region this read assumed, e.g. "UK" — it
	// is the basis for band-plan-dependent decisions such as the 60 m
	// channel inventory.
	Region string `json:"region,omitempty"`
	// BaselineDigest is the hex-encoded SHA-256 digest (see Digest) of
	// the Channels this RadioInfo accompanies, computed at read time. A
	// send confirmation is bound to this value: any later reconnect,
	// re-read, or edit produces a different digest.
	//
	// In a SAVED file this is a DURABLE content digest, and it is
	// evidence rather than a checksum to re-verify. A file written under
	// an older schema keeps the digest it was written with, and after
	// migration (schema 2 to 3, where ChannelData.TagDisplay became a
	// BoolField) that value no longer equals Digest over the migrated
	// channels. Such a digest is non-recomputable legacy evidence and is
	// deliberately left alone — never rewritten on load, and never
	// treated as a mismatch to report. See Digest's doc comment for the
	// full reasoning, including why digest versioning was rejected.
	BaselineDigest string `json:"baseline_digest,omitempty"`
}

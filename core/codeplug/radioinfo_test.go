// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"encoding/json"
	"testing"
	"time"
)

// TestRadioInfoJSON_OptionalFieldsOmitted checks that a RadioInfo with
// only its non-omitempty fields set marshals without the optional keys
// present at all.
func TestRadioInfoJSON_OptionalFieldsOmitted(t *testing.T) {
	readAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	ri := RadioInfo{
		Model:  "FT-710",
		CATID:  "0800",
		ReadAt: readAt,
	}

	b, err := json.Marshal(ri)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	for _, key := range []string{"model", "cat_id", "read_at"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("marshalled JSON missing required key %q: %s", key, b)
		}
	}
	for _, key := range []string{"port", "usb_serial", "firmware_confirmed", "region", "baseline_digest"} {
		if _, ok := raw[key]; ok {
			t.Errorf("marshalled JSON has optional key %q present with zero value, want omitted: %s", key, b)
		}
	}
}

// TestRadioInfoJSON_RoundTrip populates every field and checks a lossless
// marshal/unmarshal round trip.
func TestRadioInfoJSON_RoundTrip(t *testing.T) {
	readAt := time.Date(2026, 7, 10, 12, 34, 56, 0, time.UTC)
	want := RadioInfo{
		Model:             "FT-710",
		CATID:             "0800",
		ReadAt:            readAt,
		Port:              "/dev/cu.usbserial-1234",
		USBSerial:         "AB12CD34",
		FirmwareConfirmed: "1.06",
		Region:            "UK",
		BaselineDigest:    "deadbeef",
	}

	b, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got RadioInfo
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	got.ReadAt = got.ReadAt.UTC()
	if !got.ReadAt.Equal(want.ReadAt) {
		t.Errorf("ReadAt = %v, want %v", got.ReadAt, want.ReadAt)
	}
	got.ReadAt = want.ReadAt // neutralise for the rest of the comparison
	if got != want {
		t.Errorf("RadioInfo round trip = %+v, want %+v", got, want)
	}
}

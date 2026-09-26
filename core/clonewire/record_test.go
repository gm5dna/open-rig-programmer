// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"errors"
	"testing"
)

func TestParseImageRoundTrip(t *testing.T) {
	p := Profile{
		Model:        "TEST",
		ImageLen:     10,
		RecordWidth:  2,
		ChannelCount: 5,
	}
	raw := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	img, err := ParseImage(raw, p)
	if err != nil {
		t.Fatalf("ParseImage: %v", err)
	}
	if len(img.Raw) != len(raw) {
		t.Fatalf("Raw len = %d, want %d", len(img.Raw), len(raw))
	}
	if len(img.Records) != p.ChannelCount {
		t.Fatalf("len(Records) = %d, want %d", len(img.Records), p.ChannelCount)
	}
	for i, rec := range img.Records {
		want := []byte{byte(i * 2), byte(i*2 + 1)}
		if string(rec) != string(want) {
			t.Errorf("Records[%d] = %v, want %v", i, rec, want)
		}
	}

	// The returned Image must not alias raw.
	raw[0] = 0xFF
	if img.Raw[0] == 0xFF {
		t.Error("ParseImage's Raw aliases the input slice")
	}
}

func TestParseImageNoChannels(t *testing.T) {
	p := Profile{Model: "TEST", ImageLen: 4}
	img, err := ParseImage([]byte{1, 2, 3, 4}, p)
	if err != nil {
		t.Fatalf("ParseImage: %v", err)
	}
	if img.Records != nil {
		t.Errorf("Records = %v, want nil for ChannelCount 0", img.Records)
	}
}

func TestParseImageWrongLength(t *testing.T) {
	p := Profile{Model: "TEST", ImageLen: 10}
	if _, err := ParseImage([]byte{1, 2, 3}, p); !errors.Is(err, ErrRecord) {
		t.Errorf("err = %v, want ErrRecord", err)
	}
}

func TestParseImageRecordsDontFit(t *testing.T) {
	p := Profile{Model: "TEST", ImageLen: 4, RecordWidth: 3, ChannelCount: 2}
	if _, err := ParseImage([]byte{1, 2, 3, 4}, p); !errors.Is(err, ErrRecord) {
		t.Errorf("err = %v, want ErrRecord", err)
	}
}

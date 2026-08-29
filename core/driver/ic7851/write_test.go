// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"errors"
	"testing"

)

func TestE6RefusesNonTemplateUnmappedBytes(t *testing.T) {
	raw := make([]byte, civicRecordLength())
	raw[0] = 0x01
	var e *UnmappedRegionError
	if !errors.As(unmappedRegionsDiffer(raw), &e) || e.Offset != 0 || e.Nibble != "low" { t.Fatalf("E6 result = %v", unmappedRegionsDiffer(raw)) }
}

func TestE6AcceptsTemplate(t *testing.T) {
	if err := unmappedRegionsDiffer(make([]byte, civicRecordLength())); err != nil { t.Fatal(err) }
}

func civicRecordLength() int { return 25 }

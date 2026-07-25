// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"testing"
)

// TestFormatProgressLine pins task-12 brief §1's example rendering,
// "read 42/117 M-42", including the codeplug.DisplaySlot mapping from
// canonical wire-form slot to display form.
func TestFormatProgressLine(t *testing.T) {
	got := formatProgressLine("read", 42, 117, "042")
	want := "read 42/117 M-42\n"
	if got != want {
		t.Errorf("formatProgressLine(read, 42, 117, 042) = %q, want %q", got, want)
	}
}

// TestFormatProgressLine_NonMappedSlot confirms a slot DisplaySlot leaves
// unchanged (e.g. a PMS pair) still renders sensibly.
func TestFormatProgressLine_NonMappedSlot(t *testing.T) {
	got := formatProgressLine("write", 3, 9, "P2U")
	want := "write 3/9 P2U\n"
	if got != want {
		t.Errorf("formatProgressLine(write, 3, 9, P2U) = %q, want %q", got, want)
	}
}

// TestProgressPrinter confirms progressPrinter's returned clone.Progress
// writes formatProgressLine's output to the given writer, one call per
// invocation.
func TestProgressPrinter(t *testing.T) {
	var buf bytes.Buffer
	p := progressPrinter(&buf)
	p("read", 1, 2, "001")
	p("read", 2, 2, "002")

	want := "read 1/2 M-01\nread 2/2 M-02\n"
	if buf.String() != want {
		t.Errorf("progressPrinter output = %q, want %q", buf.String(), want)
	}
}

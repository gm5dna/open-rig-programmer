// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// ft710MemChannel is a plain, fully-expressible MEM channel for this
// radio's REAL profile: every field the MW/MT frame carries has a value,
// and the three fields the frame has NO room for (ctcss_tone, scan_skip)
// or that this radio DOES carry (tag_display) are set the way a read of a
// real FT-710 leaves them — tag_display Known (the flag is mandatory and
// always read back), ctcss_tone and scan_skip Unavailable.
//
// Callers mutate the returned value; each call allocates its own.
func ft710MemChannel(freqHz uint64, tag string) *codeplug.ChannelData {
	return &codeplug.ChannelData{
		FreqHz:     freqHz,
		Mode:       "FM",
		CTCSS:      "OFF",
		Shift:      "SIMPLEX",
		Tag:        tag,
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
	}
}

// TestDiff_KnownUnreachableFieldRefusedOnRealProfile is the profile-keyed
// pin for the refusal an adversarial review (Wave-1c review 1, finding 1)
// caught the Icom tier silently deleting: on THIS radio — the one this
// project has hardware for, through its real profile, not a fixture —
// spec.FieldCTCSSTone and spec.FieldScanSkip are the zero FieldSupport
// (caps.go: no CAT frame carries either), so a candidate channel that
// carries a KNOWN value for one is a request the radio cannot honour, and
// this project's posture on such a request is to REFUSE it at plan time,
// never to drop the value and write the rest.
//
// The route is ordinary, not exotic: core/csvio maps a `ctcss_tone` cell
// of "88.5" and a `scan_skip` cell of "yes" straight to Known, and the
// project's own CSV golden carries both — `rigprog import` then `rigprog
// write` is all it takes.
func TestDiff_KnownUnreachableFieldRefusedOnRealProfile(t *testing.T) {
	caps := CapabilitiesRealHardware()
	for _, f := range []spec.Field{spec.FieldCTCSSTone, spec.FieldScanSkip} {
		if fs := caps.FieldSupport(spec.BankMemory, f); !fs.Unreachable() {
			t.Fatalf("%s support = %+v, want the zero FieldSupport: this test's premise is that no MW/MT frame carries it", f, fs)
		}
	}

	for _, tt := range []struct {
		name   string
		field  spec.Field
		mutate func(*codeplug.ChannelData)
	}{
		{
			name:   "a Known ctcss_tone",
			field:  spec.FieldCTCSSTone,
			mutate: func(d *codeplug.ChannelData) { d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 885} },
		},
		{
			name:   "a Known scan_skip",
			field:  spec.FieldScanSkip,
			mutate: func(d *codeplug.ChannelData) { d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true} },
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			baseline := &codeplug.Codeplug{
				Schema:   codeplug.CurrentSchema,
				Radio:    codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
				Channels: []codeplug.Channel{{Slot: "001", Data: ft710MemChannel(145_500_000, "GB3TEST")}},
			}
			after := ft710MemChannel(145_500_000, "GB3TEST")
			tt.mutate(after)
			file := &codeplug.Codeplug{
				Schema:   codeplug.CurrentSchema,
				Radio:    codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
				Channels: []codeplug.Channel{{Slot: "001", Data: after}},
			}

			res, err := codeplug.Diff(baseline, file, caps)
			if err != nil {
				t.Fatalf("Diff() error = %v", err)
			}
			if len(res.Entries) != 1 {
				t.Fatalf("Entries = %d, want 1", len(res.Entries))
			}
			e := res.Entries[0]
			if e.Kind != codeplug.DiffModified {
				t.Fatalf("Kind = %q, want %q", e.Kind, codeplug.DiffModified)
			}
			if !e.Blocked {
				t.Fatalf("Blocked = false, want true: %s is unreachable on this radio, so the request cannot be honoured and must be refused, not silently dropped", tt.field)
			}
			want := string(tt.field) + " not writable on this radio"
			if !strings.Contains(e.BlockReason, want) {
				t.Errorf("BlockReason = %q, want it to contain %q", e.BlockReason, want)
			}
		})
	}
}

// TestDiff_UnreachableFieldNobodyAskedForDoesNotBlock is the converse
// pin, and the reason the refusal above is keyed on KNOWN rather than on
// the capability alone: an Unavailable ctcss_tone/scan_skip — what every
// read of a real FT-710 produces, on every channel — is not a request at
// all, and must flow.
func TestDiff_UnreachableFieldNobodyAskedForDoesNotBlock(t *testing.T) {
	caps := CapabilitiesRealHardware()
	baseline := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Channels: []codeplug.Channel{{Slot: "001", Data: ft710MemChannel(145_500_000, "GB3TEST")}},
	}
	file := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Channels: []codeplug.Channel{{Slot: "001", Data: ft710MemChannel(145_600_000, "GB3TEST")}},
	}

	res, err := codeplug.Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	e := res.Entries[0]
	if e.Kind != codeplug.DiffModified {
		t.Fatalf("Kind = %q, want %q", e.Kind, codeplug.DiffModified)
	}
	if e.Blocked {
		t.Errorf("Blocked = true (%q), want a plain frequency edit to flow", e.BlockReason)
	}
}

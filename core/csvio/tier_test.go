// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// yaesuLikeChannels is the channel set testdata/canonical-v1-export.csv
// was produced from, by the EXPORTER AS IT STOOD AT 074ed97 — the commit
// this tier branched from. Every tier field is Unavailable, which is
// what a read of any registered radio produces and what a load of any
// schema-1/2/3 file migrates to.
//
// It covers what the byte-identity pin needs: all three FieldStates on
// both tri-state columns, an empty slot, a negative clarifier, the
// formula-injection escape and the leading-apostrophe double-escape, and
// a uint32-maximum frequency.
func yaesuLikeChannels() []codeplug.Channel {
	tier := func(d *codeplug.ChannelData) *codeplug.ChannelData {
		d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable}
		d.Duplex = codeplug.StringField{State: codeplug.Unavailable}
		d.OffsetHz = codeplug.FreqField{State: codeplug.Unavailable}
		d.ToneMode = codeplug.StringField{State: codeplug.Unavailable}
		d.ToneTx = codeplug.ToneField{State: codeplug.Unavailable}
		d.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
		d.DTCSCode = codeplug.IntField{State: codeplug.Unavailable}
		d.DTCSPolarity = codeplug.StringField{State: codeplug.Unavailable}
		d.Filter = codeplug.StringField{State: codeplug.Unavailable}
		d.DataMode = codeplug.BoolField{State: codeplug.Unavailable}
		d.TuningStepEnabled = codeplug.BoolField{State: codeplug.Unavailable}
		d.TuningStep = codeplug.StringField{State: codeplug.Unavailable}
		d.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Unavailable}
		d.AttenuatorDB = codeplug.IntField{State: codeplug.Unavailable}
		d.Preamp = codeplug.StringField{State: codeplug.Unavailable}
		d.Antenna = codeplug.StringField{State: codeplug.Unavailable}
		d.IPPlus = codeplug.BoolField{State: codeplug.Unavailable}
		return d
	}
	return []codeplug.Channel{
		{Slot: "001", Data: tier(&codeplug.ChannelData{
			FreqHz: 14250000, Mode: "USB",
			CTCSS: "OFF", CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
			Shift: "SIMPLEX", Tag: "CALLING",
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		})},
		{Slot: "002", Data: tier(&codeplug.ChannelData{
			FreqHz: 145500000, Mode: "FM", ClarHz: -120, RxClar: true, TxClar: true,
			CTCSS: "ENC-DEC", CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: 885},
			Shift: "MINUS", Tag: "=SUM(A1)",
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
			ScanSkip:   codeplug.BoolField{State: codeplug.Known, Value: true},
		})},
		{Slot: "003"},
		{Slot: "P1L", Data: tier(&codeplug.ChannelData{
			FreqHz: 4294967295, Mode: "CW-U",
			CTCSS: "OFF", CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},
			Shift: "PLUS", Tag: "'quoted",
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
		})},
	}
}

// TestExport_YaesuOutputByteIdentical is the native-CSV half of the
// tier's byte-identity requirement: an export of content no tier field
// records is byte-for-byte what this program produced before the tier —
// version-1 header, version-1 rows, same escaping.
func TestExport_YaesuOutputByteIdentical(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "canonical-v1-export.csv"))
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	var got bytes.Buffer
	if err := Export(&got, yaesuLikeChannels()); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if got.String() != string(want) {
		t.Errorf("Export() is not byte-identical to the pre-tier exporter.\n--- want ---\n%s\n--- got ---\n%s", want, got.String())
	}
}

// TestExport_VersionChosenByContent pins the versioning rule: the
// version-1 header while nothing is recorded, the version-2 header as
// soon as one tier field is — the exporter's analogue of
// core/codeplug's lowest-schema writer.
func TestExport_VersionChosenByContent(t *testing.T) {
	headerOf := func(channels []codeplug.Channel) string {
		var buf bytes.Buffer
		if err := Export(&buf, channels); err != nil {
			t.Fatalf("Export() error = %v", err)
		}
		return strings.SplitN(buf.String(), "\n", 2)[0]
	}

	v1 := strings.Join(header, ",")
	v2 := strings.Join(headerV2, ",")

	if got := headerOf(yaesuLikeChannels()); got != v1 {
		t.Errorf("header for all-Unavailable content = %q, want the version-1 header", got)
	}
	if got := headerOf(nil); got != v1 {
		t.Errorf("header for no channels = %q, want the version-1 header", got)
	}
	if got := headerOf([]codeplug.Channel{{Slot: "003"}}); got != v1 {
		t.Errorf("header for an empty slot = %q, want the version-1 header", got)
	}

	for name, mutate := range map[string]func(*codeplug.ChannelData){
		"a Known tier field":    func(d *codeplug.ChannelData) { d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"} },
		"an Unknown tier field": func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{State: codeplug.Unknown} },
	} {
		t.Run(name, func(t *testing.T) {
			channels := yaesuLikeChannels()
			mutate(channels[0].Data)
			if got := headerOf(channels); got != v2 {
				t.Errorf("header = %q, want the version-2 header", got)
			}
		})
	}
}

// TestExportImport_TierRoundTrip: a version-2 export re-imports to
// exactly the channels it came from, every state included — the same
// losslessness the version-1 schema has always promised.
func TestExportImport_TierRoundTrip(t *testing.T) {
	want := []codeplug.Channel{
		{Slot: "G01-001", Data: &codeplug.ChannelData{
			FreqHz: 10_000_000_000, Mode: "USB", Tag: "10G BEACON",
			CTCSS: "", CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},
			Shift:        "",
			TagDisplay:   codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:     codeplug.BoolField{State: codeplug.Known, Value: true},
			TxFreqHz:     codeplug.FreqField{State: codeplug.Known, Value: 10_000_600_000},
			Duplex:       codeplug.StringField{State: codeplug.Known, Value: "DUP+"},
			OffsetHz:     codeplug.FreqField{State: codeplug.Known, Value: 600_000},
			ToneMode:     codeplug.StringField{State: codeplug.Known, Value: "TSQL"},
			ToneTx:       codeplug.ToneField{State: codeplug.Known, Value: 885},
			ToneRx:       codeplug.ToneField{State: codeplug.Unknown},
			DTCSCode:     codeplug.IntField{State: codeplug.Known, Value: 754},
			DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
			Filter:       codeplug.StringField{State: codeplug.Known, Value: "FIL2"},
			DataMode:     codeplug.BoolField{State: codeplug.Known, Value: false},
		}},
		// A channel whose tier fields are all Absent, alongside one whose
		// are not: the file has to carry the distinction, since it is
		// version 2 either way.
		{Slot: "G01-002", Data: &codeplug.ChannelData{
			FreqHz: 145_500_000, Mode: "FM",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			TagDisplay: codeplug.BoolField{State: codeplug.Unknown},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		}},
		{Slot: "G01-003"},
	}
	for i := range want {
		if want[i].Data != nil {
			markReceiverFieldsUnavailable(want[i].Data)
		}
	}

	var buf bytes.Buffer
	if err := Export(&buf, want); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	got, err := Import(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Import() error = %v\n%s", err, buf.String())
	}
	if len(got) != len(want) {
		t.Fatalf("Import() returned %d channels, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Slot != want[i].Slot {
			t.Errorf("channel %d slot = %q, want %q", i, got[i].Slot, want[i].Slot)
		}
		if (got[i].Data == nil) != (want[i].Data == nil) {
			t.Errorf("channel %d emptiness differs: got %v, want %v", i, got[i].Data, want[i].Data)
			continue
		}
		if got[i].Data != nil && *got[i].Data != *want[i].Data {
			t.Errorf("channel %d round-tripped differently:\n got %+v\nwant %+v", i, *got[i].Data, *want[i].Data)
		}
	}
}

// TestImport_AcceptsBothHeaderVersions: a version-1 file imports with
// every tier field left Absent — the file has no column for them and
// therefore says nothing — and a version-2 file imports with the states
// its cells spell.
func TestImport_AcceptsBothHeaderVersions(t *testing.T) {
	t.Run("version 1 says the radio has no such field", func(t *testing.T) {
		body := strings.Join(header, ",") + "\n" +
			"001,M-01,14250000,USB,,,,OFF,,SIMPLEX,CALLING,yes,no\n"
		got, err := Import(strings.NewReader(body))
		if err != nil {
			t.Fatalf("Import() error = %v", err)
		}
		if len(got) != 1 || got[0].Data == nil {
			t.Fatalf("Import() = %+v, want one populated channel", got)
		}
		d := got[0].Data
		for name, state := range map[string]codeplug.FieldState{
			"tx_frequency": d.TxFreqHz.State, "duplex": d.Duplex.State,
			"offset": d.OffsetHz.State, "tone_mode": d.ToneMode.State,
			"tone_tx": d.ToneTx.State, "tone_rx": d.ToneRx.State,
			"dtcs_code": d.DTCSCode.State, "dtcs_polarity": d.DTCSPolarity.State,
			"filter": d.Filter.State, "data_mode": d.DataMode.State,
			"tuning_step_enabled": d.TuningStepEnabled.State,
			"tuning_step":         d.TuningStep.State,
			"program_tuning_step": d.ProgramTuningStepHz.State,
			"attenuator":          d.AttenuatorDB.State,
			"preamp":              d.Preamp.State, "antenna": d.Antenna.State,
			"ip_plus": d.IPPlus.State,
		} {
			// Unavailable, not the zero value: a version-1 file was
			// written by a build that modelled none of these fields,
			// for a radio that has none — which is what a READ of that
			// radio reports too, so an imported channel still compares
			// equal to the baseline it will be diffed against.
			if state != codeplug.Unavailable {
				t.Errorf("%s = %q, want Unavailable: a version-1 file says the radio has no such field", name, state)
			}
		}
	})

	t.Run("version 2 reads every reserved spelling", func(t *testing.T) {
		body := strings.Join(headerV2, ",") + "\n" +
			"001,M-01,14250000,USB,,,,OFF,,SIMPLEX,CALLING,yes,no," +
			"145000000,DUP-,600000,TSQL,88.5,n/a,23,absent,FIL1,yes\n"
		got, err := Import(strings.NewReader(body))
		if err != nil {
			t.Fatalf("Import() error = %v", err)
		}
		d := got[0].Data
		want := codeplug.ChannelData{
			FreqHz: 14250000, Mode: "USB", CTCSS: "OFF",
			CTCSSTone:    codeplug.ToneField{State: codeplug.Unknown},
			Shift:        "SIMPLEX",
			Tag:          "CALLING",
			TagDisplay:   codeplug.BoolField{State: codeplug.Known, Value: true},
			ScanSkip:     codeplug.BoolField{State: codeplug.Known, Value: false},
			TxFreqHz:     codeplug.FreqField{State: codeplug.Known, Value: 145000000},
			Duplex:       codeplug.StringField{State: codeplug.Known, Value: "DUP-"},
			OffsetHz:     codeplug.FreqField{State: codeplug.Known, Value: 600000},
			ToneMode:     codeplug.StringField{State: codeplug.Known, Value: "TSQL"},
			ToneTx:       codeplug.ToneField{State: codeplug.Known, Value: 885},
			ToneRx:       codeplug.ToneField{State: codeplug.Unavailable},
			DTCSCode:     codeplug.IntField{State: codeplug.Known, Value: 23},
			DTCSPolarity: codeplug.StringField{State: codeplug.Absent},
			Filter:       codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
			DataMode:     codeplug.BoolField{State: codeplug.Known, Value: true},
		}
		markReceiverFieldsUnavailable(&want)
		if *d != want {
			t.Errorf("Import() =\n %+v\nwant\n %+v", *d, want)
		}
	})

	t.Run("an empty slot row in a version-2 file", func(t *testing.T) {
		body := strings.Join(headerV2, ",") + "\n" +
			"003,M-03" + strings.Repeat(",", len(headerV2)-2) + "\n"
		got, err := Import(strings.NewReader(body))
		if err != nil {
			t.Fatalf("Import() error = %v", err)
		}
		if len(got) != 1 || got[0].Data != nil {
			t.Fatalf("Import() = %+v, want one EMPTY channel", got)
		}
	})
}

// TestImport_FreqHzReachesPastUint32 pins the widened parse (design D4,
// round 2 C8/F11): import.go's freq_hz parse was a hard 32-bit one, so a
// 10 GHz channel was unreadable no matter what radio it was for.
func TestImport_FreqHzReachesPastUint32(t *testing.T) {
	body := strings.Join(header, ",") + "\n" +
		"001,M-01,10000000000,USB,,,,OFF,,SIMPLEX,BEACON,yes,no\n"
	got, err := Import(strings.NewReader(body))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if got[0].Data.FreqHz != 10_000_000_000 {
		t.Errorf("FreqHz = %d, want 10000000000", got[0].Data.FreqHz)
	}
}

// icomCHIRPCapabilities is a Capabilities in the shape the Icom tier
// will register: the duplex/tone-mode vocabularies, a memory bank
// declaring the tier's fields, and no Yaesu shift/CTCSS-state
// vocabulary. A TEST fixture — no Icom driver exists yet and none of
// these values is evidence.
func icomCHIRPCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: "TEST-ICOM", CATID: "A4",
		Modes: []string{"USB", "FM"}, TagLen: 10,
		CTCSSTones:  tones[:],
		Bauds:       []int{19200},
		DefaultBaud: 19200,
		DuplexOptions: []spec.DuplexOption{
			{Value: "OFF", Direction: spec.DuplexOff},
			{Value: "DUP+", Direction: spec.DuplexUp},
			{Value: "DUP-", Direction: spec.DuplexDown},
		},
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
			{Value: "DTCS", Semantics: spec.ToneModeDTCS},
		},
		DTCSPolarities: []string{"NN", "NR", "RN", "RR"},
		DTCSCodes:      []int{23, 25, 754},
		Filters:        []string{"FIL1", "FIL2"},
		Banks: []spec.Bank{{
			ID: spec.BankMemory, Label: "Memories",
			Slots: []string{"G01-001", "G01-002", "G01-003"},
			Fields: map[spec.Field]spec.FieldSupport{
				spec.FieldFrequency:    {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldMode:         {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldTag:          {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldTxFrequency:  {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldDuplex:       {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldOffset:       {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldToneMode:     {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldToneTx:       {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldToneRx:       {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldDTCSCode:     {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldDTCSPolarity: {Read: spec.Supported, Write: spec.Unverified},
				spec.FieldFilter:       {Read: spec.Supported, Write: spec.Unverified},
			},
		}},
	}
}

// TestImportCHIRP_CapabilityAwareDuplexAndTone walks the mappings the
// tier made capability-aware (design D4): duplex with a real offset,
// CHIRP's split as a stored transmit frequency, and DTCS as a code plus
// polarity — every one of which the pre-tier importer refused or dropped
// because no radio it knew had anywhere to put it.
func TestImportCHIRP_CapabilityAwareDuplexAndTone(t *testing.T) {
	caps := icomCHIRPCapabilities()
	head := "Location,Name,Frequency,Duplex,Offset,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity,Mode,Skip\n"

	t.Run("a minus shift keeps its offset", func(t *testing.T) {
		body := head + "1,GB3TEST,145.700000,-,0.600000,Tone,88.5,88.5,023,NN,FM,\n"
		channels, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		if report.HasBlocking() {
			t.Fatalf("report has blocking entries: %+v", report.Entries)
		}
		d := channels[0].Data
		if d.Duplex != (codeplug.StringField{State: codeplug.Known, Value: "DUP-"}) {
			t.Errorf("Duplex = %+v, want Known DUP-", d.Duplex)
		}
		if d.OffsetHz != (codeplug.FreqField{State: codeplug.Known, Value: 600_000}) {
			t.Errorf("OffsetHz = %+v, want Known 600000", d.OffsetHz)
		}
		if d.ToneMode.Value != "TONE" || d.ToneTx.Value != 885 {
			t.Errorf("ToneMode = %+v, ToneTx = %+v, want TONE / 88.5", d.ToneMode, d.ToneTx)
		}
		if d.Shift != "" {
			t.Errorf("Shift = %q, want empty: this radio has no shift vocabulary", d.Shift)
		}
	})

	t.Run("split becomes a stored transmit frequency", func(t *testing.T) {
		body := head + "1,SPLIT,145.700000,split,433.500000,,88.5,88.5,023,NN,FM,\n"
		channels, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		if report.HasBlocking() {
			t.Fatalf("report has blocking entries: %+v", report.Entries)
		}
		d := channels[0].Data
		if d.TxFreqHz != (codeplug.FreqField{State: codeplug.Known, Value: 433_500_000}) {
			t.Errorf("TxFreqHz = %+v, want Known 433500000", d.TxFreqHz)
		}
		if d.Duplex.Value != "OFF" {
			t.Errorf("Duplex = %+v, want the radio's OFF value: a split channel has two frequencies, not a shift", d.Duplex)
		}
	})

	t.Run("DTCS becomes a code and a polarity", func(t *testing.T) {
		body := head + "1,DCSCH,145.700000,,0.000000,DTCS,88.5,88.5,754,RN,FM,\n"
		channels, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		if report.HasBlocking() {
			t.Fatalf("report has blocking entries: %+v", report.Entries)
		}
		d := channels[0].Data
		if d.ToneMode.Value != "DTCS" {
			t.Errorf("ToneMode = %+v, want DTCS", d.ToneMode)
		}
		if d.DTCSCode != (codeplug.IntField{State: codeplug.Known, Value: 754}) {
			t.Errorf("DTCSCode = %+v, want Known 754", d.DTCSCode)
		}
		if d.DTCSPolarity != (codeplug.StringField{State: codeplug.Known, Value: "RN"}) {
			t.Errorf("DTCSPolarity = %+v, want Known RN", d.DTCSPolarity)
		}
		// And the DTCS columns must NOT also be reported as dropped.
		for _, e := range report.Entries {
			if e.Column == "DtcsCode" || e.Column == "DtcsPolarity" {
				t.Errorf("a consumed column was also reported dropped: %+v", e)
			}
		}
	})

	t.Run("a code outside the radio's table blocks", func(t *testing.T) {
		body := head + "1,DCSCH,145.700000,,0.000000,DTCS,88.5,88.5,999,NN,FM,\n"
		_, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		if !report.HasBlocking() {
			t.Fatalf("report = %+v, want a blocking entry for a code outside the table", report.Entries)
		}
	})
}

// TestImportCHIRP_YaesuBranchUnchanged is the converse pin, and the one
// that matters most: a radio with no Icom vocabulary takes the original
// branch, and its LossReport and channel are identical to what the
// pre-tier importer produced — split still refused, Offset still
// dropped, DTCS still refused.
func TestImportCHIRP_YaesuBranchUnchanged(t *testing.T) {
	caps := ft710LikeCapabilities()
	head := "Location,Name,Frequency,Duplex,Offset,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity,Mode,Skip\n"

	t.Run("split is refused", func(t *testing.T) {
		body := head + "1,SPLIT,14.250000,split,0.600000,,88.5,88.5,023,NN,USB,\n"
		_, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		found := false
		for _, e := range report.Entries {
			if e.Column == "Duplex" && e.Blocking &&
				strings.Contains(e.Detail, "split-frequency duplex (independent TX/RX frequencies) has no") {
				found = true
			}
		}
		if !found {
			t.Errorf("entries = %+v, want the unchanged blocking split entry", report.Entries)
		}
	})

	t.Run("a non-zero offset is dropped with its original wording", func(t *testing.T) {
		body := head + "1,REP,14.250000,-,0.600000,,88.5,88.5,023,NN,USB,\n"
		channels, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		found := false
		for _, e := range report.Entries {
			if e.Column == "Offset" && !e.Blocking &&
				e.Detail == "FT-710 stores no per-channel repeater offset; shift magnitude is a global menu setting" {
				found = true
			}
		}
		if !found {
			t.Errorf("entries = %+v, want the unchanged dropped-Offset entry", report.Entries)
		}
		// And nothing tier-shaped was invented on the channel: every
		// field this radio cannot reach comes back Unavailable — the
		// same answer chirpTagDisplay has always given for the display
		// flag, and the same one a read of this radio gives.
		d := channels[0].Data
		if d.Duplex.State != codeplug.Unavailable || d.OffsetHz.State != codeplug.Unavailable || d.TxFreqHz.State != codeplug.Unavailable {
			t.Errorf("tier fields = %+v/%+v/%+v, want all Unavailable on a radio that has none of them", d.Duplex, d.OffsetHz, d.TxFreqHz)
		}
		if d.Shift != "MINUS" {
			t.Errorf("Shift = %q, want MINUS", d.Shift)
		}
	})

	t.Run("DTCS is refused", func(t *testing.T) {
		body := head + "1,DCSCH,14.250000,,0.000000,DTCS,88.5,88.5,023,NN,USB,\n"
		_, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		found := false
		for _, e := range report.Entries {
			if e.Column == "Tone" && e.Blocking && strings.Contains(e.Detail, "CAT has no DCS memory write") {
				found = true
			}
		}
		if !found {
			t.Errorf("entries = %+v, want the unchanged blocking DTCS entry", report.Entries)
		}
	})
}

// TestNeedsTierColumns_MatchesSchemaFor states the invariant the two
// writers share: the CSV exporter and the codeplug file writer must
// agree, channel for channel, about whether a tier field records
// anything — otherwise a codeplug could round-trip through schema 3
// while its CSV export claimed version 2, or the reverse.
func TestNeedsTierColumns_MatchesSchemaFor(t *testing.T) {
	for name, channels := range map[string][]codeplug.Channel{
		"all Unavailable": yaesuLikeChannels(),
		"one Known":       withTierField(func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{State: codeplug.Known, Value: "FIL1"} }),
		"one Unknown":     withTierField(func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{State: codeplug.Unknown} }),
		"one Absent":      withTierField(func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{} }),
	} {
		t.Run(name, func(t *testing.T) {
			wantV2 := needsTierColumns(channels)
			// core/codeplug decides the same question with the same
			// predicate; reproduce it here rather than importing an
			// unexported function.
			gotV2 := false
			for _, ch := range channels {
				if ch.Data == nil {
					continue
				}
				d := *ch.Data
				if d.TxFreqHz.State.Recorded() || d.Duplex.State.Recorded() ||
					d.OffsetHz.State.Recorded() || d.ToneMode.State.Recorded() ||
					d.ToneTx.State.Recorded() || d.ToneRx.State.Recorded() ||
					d.DTCSCode.State.Recorded() || d.DTCSPolarity.State.Recorded() ||
					d.Filter.State.Recorded() || d.DataMode.State.Recorded() {
					gotV2 = true
				}
			}
			if wantV2 != gotV2 {
				t.Errorf("needsTierColumns() = %v, but the Recorded rule says %v", wantV2, gotV2)
			}
		})
	}
}

// withTierField returns yaesuLikeChannels with one channel's tier field
// mutated, for the table above.
func withTierField(mutate func(*codeplug.ChannelData)) []codeplug.Channel {
	channels := yaesuLikeChannels()
	mutate(channels[0].Data)
	return channels
}

// TestHeaderVersions_V1IsAPrefixOfV2 pins the structural property the
// whole two-version scheme rests on: version 2 ADDS columns, in order,
// and never reorders or renames one — which is why a version-1 file is
// still valid and why column lookup by name serves both.
func TestHeaderVersions_V1IsAPrefixOfV2(t *testing.T) {
	if len(headerV2) != len(header)+len(tierColumns) {
		t.Fatalf("headerV2 has %d columns, want %d + %d", len(headerV2), len(header), len(tierColumns))
	}
	if !reflect.DeepEqual(headerV2[:len(header)], header) {
		t.Errorf("headerV2 does not start with the version-1 header:\n got %v\nwant %v", headerV2[:len(header)], header)
	}
	if !reflect.DeepEqual(headerV2[len(header):], tierColumns) {
		t.Errorf("headerV2's tail = %v, want %v", headerV2[len(header):], tierColumns)
	}
}

// TestImportCHIRP_FilterAndDataModeDefaultToUnknown pins the two tier
// fields NO CHIRP column speaks to (Wave-1c review 1, finding 4). Every
// other tier field is filled in by the duplex or tone branch where the
// radio reaches it; Filter and DataMode have no branch, so the defaulting
// step has to give them their answer or they stay Absent — the one state
// that means "this channel says nothing at all", which codeplug.Validate
// reports as an error on EVERY imported channel and codeplug.Diff counts
// as a modification in a field the file never mentioned.
//
// Unknown is the honest answer where the radio has the field: "this radio
// has it and this file did not say". It is what importCHIRPDuplexIcom's
// own doc comment promises for any field a row does not speak to.
func TestImportCHIRP_FilterAndDataModeDefaultToUnknown(t *testing.T) {
	head := "Location,Name,Frequency,Duplex,Offset,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity,Mode,Skip\n"
	body := head + "1,GB3TEST,145.700000,-,0.600000,Tone,88.5,88.5,023,NN,FM,\n"

	t.Run("a bank that reaches both", func(t *testing.T) {
		caps := icomCHIRPCapabilities()
		fields := caps.Banks[0].Fields
		fields[spec.FieldDataMode] = spec.FieldSupport{Read: spec.Supported, Write: spec.Unverified}
		for _, f := range []spec.Field{spec.FieldFilter, spec.FieldDataMode} {
			if caps.FieldSupport(spec.BankMemory, f).Unreachable() {
				t.Fatalf("%s is Unreachable: this subtest's premise is a bank that HAS it", f)
			}
		}

		channels, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		if report.HasBlocking() {
			t.Fatalf("report has blocking entries: %+v", report.Entries)
		}
		d := channels[0].Data
		if d.Filter != (codeplug.StringField{State: codeplug.Unknown}) {
			t.Errorf("Filter = %+v, want Unknown: this radio has a filter and no CHIRP column names one", d.Filter)
		}
		if d.DataMode != (codeplug.BoolField{State: codeplug.Unknown}) {
			t.Errorf("DataMode = %+v, want Unknown: this radio has a data-mode flag and no CHIRP column names one", d.DataMode)
		}
	})

	t.Run("a bank that reaches neither", func(t *testing.T) {
		caps := ft710LikeCapabilities()
		channels, _, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		d := channels[0].Data
		if d.Filter != (codeplug.StringField{State: codeplug.Unavailable}) {
			t.Errorf("Filter = %+v, want Unavailable on a radio with no such field", d.Filter)
		}
		if d.DataMode != (codeplug.BoolField{State: codeplug.Unavailable}) {
			t.Errorf("DataMode = %+v, want Unavailable on a radio with no such field", d.DataMode)
		}
	})
}

func TestImportCHIRP_ReceiverFieldsFollowReachability(t *testing.T) {
	const body = "Location,Name,Frequency,Duplex,Offset,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity,Mode,Skip\n" +
		"1,GB3TEST,145.700000,-,0.600000,Tone,88.5,88.5,023,NN,FM,\n"
	receiverFields := []spec.Field{
		spec.FieldTuningStepEnabled, spec.FieldTuningStep, spec.FieldProgramTuningStep,
		spec.FieldAttenuator, spec.FieldPreamp, spec.FieldAntenna, spec.FieldIPPlus,
	}
	states := func(d *codeplug.ChannelData) []codeplug.FieldState {
		return []codeplug.FieldState{
			d.TuningStepEnabled.State, d.TuningStep.State, d.ProgramTuningStepHz.State,
			d.AttenuatorDB.State, d.Preamp.State, d.Antenna.State, d.IPPlus.State,
		}
	}

	for _, tt := range []struct {
		name string
		caps spec.Capabilities
		want codeplug.FieldState
	}{
		{"reachable", func() spec.Capabilities {
			caps := icomCHIRPCapabilities()
			for _, f := range receiverFields {
				caps.Banks[0].Fields[f] = spec.FieldSupport{Read: spec.Supported, Write: spec.Unverified}
			}
			return caps
		}(), codeplug.Unknown},
		{"unreachable", ft710LikeCapabilities(), codeplug.Unavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			channels, _, err := ImportCHIRP(strings.NewReader(body), tt.caps)
			if err != nil {
				t.Fatal(err)
			}
			for i, got := range states(channels[0].Data) {
				if got != tt.want {
					t.Errorf("receiver field %s state = %q, want %q", receiverFields[i], got, tt.want)
				}
			}
		})
	}
}

// TestImportCHIRP_IcomToneBranchDefaultsCTCSSTone pins the PRE-tier
// CTCSSTone on the Icom tone branch (Wave-1c review 2, finding N1).
//
// markUnreachableTierFields answers for the tier's ten fields only, and
// the only other place a CHIRP import writes CTCSSTone is
// importCHIRPToneCTCSS — the branch a bank reaching spec.FieldToneMode
// does NOT take. So before the fix every channel imported for such a bank
// carried the zero ToneField, Absent, and codeplug.Validate reported
// `codeplug: ToneField: invalid State ""` as an error on every one of
// them.
//
// The answer is chirpTagDisplay's, asked of spec.FieldCTCSSTone:
// Unavailable where the bank cannot reach it — the normal Icom shape,
// since this tier expresses tones through tone_mode/tone_tx/tone_rx —
// and Unknown where it can. The Yaesu branch is untouched and its
// per-row answers are re-pinned here so the fix cannot have moved them.
func TestImportCHIRP_IcomToneBranchDefaultsCTCSSTone(t *testing.T) {
	head := "Location,Name,Frequency,Duplex,Offset,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity,Mode,Skip\n"
	rows := []string{
		"1,GB3TEST,145.700000,-,0.600000,Tone,88.5,88.5,023,NN,FM,\n",
		"2,GB3TSQL,145.725000,-,0.600000,TSQL,88.5,88.5,023,NN,FM,\n",
		"3,GB3DTCS,145.750000,,0.000000,DTCS,88.5,88.5,023,NN,FM,\n",
	}
	body := head + strings.Join(rows, "")

	t.Run("an Icom bank that cannot reach ctcss_tone", func(t *testing.T) {
		caps := icomCHIRPCapabilities()
		if caps.FieldSupport(spec.BankMemory, spec.FieldToneMode).Unreachable() {
			t.Fatalf("tone_mode is Unreachable: this subtest's premise is a bank that takes the Icom branch")
		}
		if !caps.FieldSupport(spec.BankMemory, spec.FieldCTCSSTone).Unreachable() {
			t.Fatalf("ctcss_tone is reachable: this subtest's premise is the normal Icom shape, a bank that lists no ctcss_tone at all")
		}

		channels, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		if report.HasBlocking() {
			t.Fatalf("report has blocking entries: %+v", report.Entries)
		}
		for _, ch := range channels {
			if ch.Data.CTCSSTone != (codeplug.ToneField{State: codeplug.Unavailable}) {
				t.Errorf("slot %s: CTCSSTone = %+v, want Unavailable on a bank with no such field", ch.Slot, ch.Data.CTCSSTone)
			}
		}

		// The consequence the state exists to prevent: Validate must
		// report NOTHING about ctcss_tone on any imported channel.
		for _, issue := range validateImported(t, channels, caps) {
			if issue.Field == spec.FieldCTCSSTone {
				t.Errorf("Validate reported a ctcss_tone issue on an imported channel: %+v", issue)
			}
		}
	})

	t.Run("an Icom bank that does reach ctcss_tone", func(t *testing.T) {
		caps := icomCHIRPCapabilities()
		caps.Banks[0].Fields[spec.FieldCTCSSTone] = spec.FieldSupport{Read: spec.Supported, Write: spec.Unverified}

		channels, report, err := ImportCHIRP(strings.NewReader(body), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		if report.HasBlocking() {
			t.Fatalf("report has blocking entries: %+v", report.Entries)
		}
		for _, ch := range channels {
			if ch.Data.CTCSSTone != (codeplug.ToneField{State: codeplug.Unknown}) {
				t.Errorf("slot %s: CTCSSTone = %+v, want Unknown: this radio HAS the field and no CHIRP column speaks to it", ch.Slot, ch.Data.CTCSSTone)
			}
		}
		for _, issue := range validateImported(t, channels, caps) {
			if issue.Field == spec.FieldCTCSSTone {
				t.Errorf("Validate reported a ctcss_tone issue on an imported channel: %+v", issue)
			}
		}
	})

	t.Run("a Yaesu bank is unchanged", func(t *testing.T) {
		caps := ft710LikeCapabilities()
		if !caps.FieldSupport(spec.BankMemory, spec.FieldToneMode).Unreachable() {
			t.Fatalf("premise: this bank must NOT reach tone_mode, so the import takes the CTCSS branch")
		}

		channels, report, err := ImportCHIRP(strings.NewReader(head+rows[0]+rows[1]), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		if report.HasBlocking() {
			t.Fatalf("report has blocking entries: %+v", report.Entries)
		}
		want := codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)}
		for _, ch := range channels {
			if ch.Data.CTCSSTone != want {
				t.Errorf("slot %s: CTCSSTone = %+v, want %+v from the Yaesu branch's own rToneFreq/cToneFreq mapping", ch.Slot, ch.Data.CTCSSTone, want)
			}
		}
	})
}

// validateImported runs codeplug.Validate over the imported channels,
// padding the slots the file did not name with empty ones so the only
// issues that can come back are about the channels themselves.
func validateImported(t *testing.T, channels []codeplug.Channel, caps spec.Capabilities) []codeplug.Issue {
	t.Helper()
	have := make(map[string]bool, len(channels))
	for _, ch := range channels {
		have[ch.Slot] = true
	}
	all := append([]codeplug.Channel(nil), channels...)
	for _, b := range caps.Banks {
		for _, slot := range b.Slots {
			if !have[slot] {
				all = append(all, codeplug.Channel{Slot: slot})
			}
		}
	}
	cp := &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: "csvio test",
		Radio:     codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
		Channels:  all,
	}
	return codeplug.Validate(cp, caps)
}

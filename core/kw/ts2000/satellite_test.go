// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import "testing"

func TestBuildSARead(t *testing.T) {
	cmd, err := BuildSARead()
	if err != nil {
		t.Fatalf("BuildSARead: %v", err)
	}
	if got, want := string(cmd.Bytes()), "SA;"; got != want {
		t.Errorf("BuildSARead = %q, want %q", got, want)
	}
}

func TestBuildSASet(t *testing.T) {
	for _, tt := range []struct {
		name string
		rec  SatelliteRecord
		want string
	}{
		{
			name: "every flag off, channel 0",
			rec:  SatelliteRecord{Channel: 0},
			want: "SA0000000;",
		},
		{
			name: "every flag on, channel 9",
			rec: SatelliteRecord{
				SatModeOn: true, Channel: 9, MainIsDownlink: true,
				CtrlOnSub: true, TraceOn: true, TraceRevOn: true, MultiCHMemoryMode: true,
			},
			want: "SA1911111;",
		},
		{
			name: "channel 5, only the per-channel flags set",
			rec:  SatelliteRecord{Channel: 5, MainIsDownlink: true, TraceOn: true},
			want: "SA0510100;",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := BuildSASet(tt.rec)
			if err != nil {
				t.Fatalf("BuildSASet(%+v): %v", tt.rec, err)
			}
			if got := string(cmd.Bytes()); got != tt.want {
				t.Errorf("BuildSASet(%+v) = %q, want %q", tt.rec, got, tt.want)
			}
		})
	}
}

func TestBuildSASet_RefusesAnOutOfRangeChannel(t *testing.T) {
	for _, channel := range []int{-1, 10, 99} {
		if _, err := BuildSASet(SatelliteRecord{Channel: channel}); err == nil {
			t.Errorf("BuildSASet(channel %d) = nil error, want a refusal", channel)
		}
	}
}

func TestParseSAAnswer(t *testing.T) {
	frame := []byte("SA1911111SAT-9   ;")
	rec, err := ParseSAAnswer(frame)
	if err != nil {
		t.Fatalf("ParseSAAnswer(%q): %v", frame, err)
	}
	want := SatelliteRecord{
		SatModeOn: true, Channel: 9, MainIsDownlink: true,
		CtrlOnSub: true, TraceOn: true, TraceRevOn: true, MultiCHMemoryMode: true,
		Name: "SAT-9   ",
	}
	if rec != want {
		t.Errorf("ParseSAAnswer(%q) = %+v, want %+v", frame, rec, want)
	}
}

func TestParseSAAnswer_Refusals(t *testing.T) {
	for _, tt := range []struct {
		name  string
		frame string
	}{
		{"too short", "SA;"},
		{"too long", "SA0000000012345678;"},
		{"wrong prefix", "MA1911111SAT-9   ;"},
		{"no terminator", "SA1911111SAT-9   x"},
		{"P1 not 0/1", "SA2911111SAT-9   ;"},
		{"P2 not a digit", "SA1A11111SAT-9   ;"},
		{"P3 not 0/1", "SA1921111SAT-9   ;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseSAAnswer([]byte(tt.frame)); err == nil {
				t.Errorf("ParseSAAnswer(%q) = nil error, want a refusal", tt.frame)
			}
		})
	}
}

func TestBuildSASet_ParseSAAnswer_RoundTrip(t *testing.T) {
	// Set carries no P8 (SI carries the name instead), so the round trip
	// here is only over P1-P7: build a Set, then check ParseSAAnswer
	// recovers the same seven flags from an ANSWER shaped frame with the
	// Set's own P1-P7 bytes and an arbitrary name appended.
	rec := SatelliteRecord{
		SatModeOn: true, Channel: 3, MainIsDownlink: false,
		CtrlOnSub: true, TraceOn: false, TraceRevOn: true, MultiCHMemoryMode: false,
	}
	setCmd, err := BuildSASet(rec)
	if err != nil {
		t.Fatalf("BuildSASet: %v", err)
	}
	setFrame := setCmd.Bytes()
	// "SA" + P1..P7 (9 bytes) + ';' == 10 bytes; splice in an 8-byte name
	// before the terminator to make an 18-byte answer frame.
	answerFrame := append(append([]byte(nil), setFrame[:len(setFrame)-1]...), []byte("NAME    ;")...)
	got, err := ParseSAAnswer(answerFrame)
	if err != nil {
		t.Fatalf("ParseSAAnswer(%q): %v", answerFrame, err)
	}
	want := rec
	want.Name = "NAME    "
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestBuildSISet(t *testing.T) {
	for _, tt := range []struct {
		name    string
		channel int
		nm      string
		want    string
	}{
		{"exact eight characters", 0, "SATCHAN0", "SI0SATCHAN0;"},
		{"shorter, space-padded", 5, "SO-50", "SI5SO-50   ;"},
		{"empty name", 9, "", "SI9        ;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := BuildSISet(tt.channel, tt.nm)
			if err != nil {
				t.Fatalf("BuildSISet(%d, %q): %v", tt.channel, tt.nm, err)
			}
			if got := string(cmd.Bytes()); got != tt.want {
				t.Errorf("BuildSISet(%d, %q) = %q, want %q", tt.channel, tt.nm, got, tt.want)
			}
		})
	}
}

func TestBuildSISet_Refusals(t *testing.T) {
	if _, err := BuildSISet(-1, "X"); err == nil {
		t.Error("BuildSISet(channel -1) = nil error, want a refusal")
	}
	if _, err := BuildSISet(10, "X"); err == nil {
		t.Error("BuildSISet(channel 10) = nil error, want a refusal")
	}
	if _, err := BuildSISet(0, "NINECHRS!"); err == nil {
		t.Error("BuildSISet(9-byte name) = nil error, want a refusal")
	}
}

func TestCommand_BytesReturnsAnIndependentCopy(t *testing.T) {
	cmd, err := BuildSARead()
	if err != nil {
		t.Fatalf("BuildSARead: %v", err)
	}
	b1 := cmd.Bytes()
	b1[0] = 'X'
	b2 := cmd.Bytes()
	if b2[0] != 'S' {
		t.Errorf("mutating one Bytes() call's result changed the next: %q", b2)
	}
}

func TestCommand_IsZero(t *testing.T) {
	var zero Command
	if !zero.IsZero() {
		t.Error("the zero Command is not IsZero()")
	}
	cmd, err := BuildSARead()
	if err != nil {
		t.Fatalf("BuildSARead: %v", err)
	}
	if cmd.IsZero() {
		t.Error("a built Command reports IsZero()")
	}
}

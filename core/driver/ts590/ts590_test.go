// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// testTiming keeps a scripted exchange's deadlines short. transport's own
// DefaultTimeout is one second and its DefaultSettle 20 ms, which are radio
// paces; a net.Pipe answers instantly, and a whole-bank read at the fleet
// defaults would spend most of a minute settling.
func testTiming() Option { return withTiming(80*time.Millisecond, time.Millisecond) }

// openTestSession opens row against a scripted radio serving img, failing the
// test if Open does. The returned port is the transcript's source.
func openTestSession(t *testing.T, row Row, img radioImage, opts ...Option) (*Session, *respondingPort) {
	t.Helper()
	p := newRespondingPort(t, row, img)
	d := New(row, Simulated, append([]Option{testTiming()}, opts...)...)
	sess, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open(%s): %v", modelNameFor(row), err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*Session), p
}

// TestNew_ModelAndCATIDPerRow pins the two registry keys and their three-digit
// identities (matrix §1.1, §1.2), and that Capabilities().Model equals the
// registry key, which core/driver.Driver's contract requires.
func TestNew_ModelAndCATIDPerRow(t *testing.T) {
	for _, tc := range []struct {
		row          Row
		model, catID string
	}{
		{RowS, "TS-590S", "021"},
		{RowSG, "TS-590SG", "023"},
	} {
		d := New(tc.row, RealHardware)
		if got := d.Model(); got != tc.model {
			t.Errorf("Model() = %q, want %q", got, tc.model)
		}
		if got := d.Capabilities().Model; got != tc.model {
			t.Errorf("Capabilities().Model = %q, want %q (core/driver.Driver's contract)", got, tc.model)
		}
		if got := d.Capabilities().CATID; got != tc.catID {
			t.Errorf("Capabilities().CATID = %q, want %q", got, tc.catID)
		}
	}
}

// TestNew_AnUnsetRowFailsClosed is P1's "no zero value" applied to every
// surface a caller can reach: the zero Row names no radio, so the driver it
// builds names none either and refuses to open at all.
func TestNew_AnUnsetRowFailsClosed(t *testing.T) {
	d := New(RowUnset, Simulated)
	if got := d.Model(); got != "" {
		t.Errorf("Model() = %q on an unset row, want the empty string — an unset row must match no registry key", got)
	}
	if caps := d.Capabilities(); !reflect.DeepEqual(caps, spec.Capabilities{}) {
		t.Errorf("Capabilities() = %+v on an unset row, want the zero value, which spec.Validate refuses", caps)
	}
	p := newRespondingPort(t, RowS, radioImage{})
	_, err := d.Open(context.Background(), p.Port(), driver.Identity{})
	if !errors.Is(err, ErrRowUnset) {
		t.Errorf("Open on an unset row: err = %v, want ErrRowUnset", err)
	}
	if got := p.Transcript(); len(got) != 0 {
		t.Errorf("Open on an unset row sent %v, want no frame at all", got)
	}
}

// TestDriver_ProfileSelection pins the fail-safe direction: RealHardware —
// the ZERO Profile — and ANY unrecognised value select the all-Unverified
// set, never the simulator's (matrix §2.1).
func TestDriver_ProfileSelection(t *testing.T) {
	for _, row := range bothRows {
		var zero Profile
		if zero != RealHardware {
			t.Fatalf("the zero Profile is %v, want RealHardware (matrix §2.1)", zero)
		}
		for _, profile := range []Profile{RealHardware, Profile(7), Profile(-1)} {
			got := New(row, profile).Capabilities()
			if !reflect.DeepEqual(got, CapabilitiesUnverified(row)) {
				t.Errorf("%s: profile %v does not select CapabilitiesUnverified", modelNameFor(row), profile)
			}
		}
		if got := New(row, Simulated).Capabilities(); !reflect.DeepEqual(got, CapabilitiesSimulated(row)) {
			t.Errorf("%s: Simulated does not select CapabilitiesSimulated", modelNameFor(row))
		}
	}
}

// TestStopBits_IsOneOnEachConcreteDriverValue is P10 leg 1: a POSITIVE
// assertion on each concrete driver value, never a type switch that would
// pass when the interface is absent.
//
// It is a SESSION PRECONDITION, not a nicety: transport.DefaultStopBits is 2
// (the Yaesu family's framing), and both Kenwood books specify one —
// "Stop Bit 1" (590:54-59). A driver that omitted the interface would open
// every session at the wrong framing and fail like a dead port.
func TestStopBits_IsOneOnEachConcreteDriverValue(t *testing.T) {
	for _, row := range bothRows {
		d := New(row, RealHardware)
		r, ok := d.(driver.SerialFramingReporter)
		if !ok {
			t.Fatalf("%s: the driver does not implement driver.SerialFramingReporter", modelNameFor(row))
		}
		if got := r.StopBits(); got != 1 {
			t.Errorf("%s: StopBits() = %d, want 1 (§3.1, 590:54-59)", modelNameFor(row), got)
		}
	}
}

// TestOpen_ProbeTranscriptIsAI0ThenIDThenFV pins P9 and matrix §3.5/§3.6:
// session init is AI0; and the identity probe is TWO frames, ID; then FV;.
// Three frames go out and the probe is two of them.
func TestOpen_ProbeTranscriptIsAI0ThenIDThenFV(t *testing.T) {
	for _, row := range bothRows {
		_, p := openTestSession(t, row, radioImage{})
		want := []string{"AI0;", "ID;", "FV;"}
		if got := p.Transcript(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: transcript = %v, want %v", modelNameFor(row), got, want)
		}
	}
}

// TestOpen_IdentityReachesTheSession pins that the probe's answer is
// authoritative: it overwrites whatever the caller supplied.
func TestOpen_IdentityReachesTheSession(t *testing.T) {
	for _, tc := range []struct {
		row   Row
		catID string
	}{{RowS, "021"}, {RowSG, "023"}} {
		sess, _ := openTestSession(t, tc.row, radioImage{})
		id := sess.Identity()
		if id.CATID != tc.catID {
			t.Errorf("%s: Identity().CATID = %q, want %q", modelNameFor(tc.row), id.CATID, tc.catID)
		}
		if id.Port != "/dev/test" {
			t.Errorf("%s: Identity().Port = %q, want the caller's", modelNameFor(tc.row), id.Port)
		}
	}
}

// TestOpen_WrongRadio pins the refusal both ways round, its typed identity,
// its transcript and the closed port.
//
// THE TRANSCRIPT IS THE HALF THAT MATTERS: a wrong radio receives AI0; and
// ID; and NOTHING MORE — no FV;, no MR read, no discovery frame of any kind.
func TestOpen_WrongRadio(t *testing.T) {
	for _, tc := range []struct {
		name     string
		row      Row
		answered string
		wantText string
	}{
		{
			// The sibling row of this very package: an SG answering an S's
			// probe. Both names are printed in this book (590:1114-1116).
			name: "the other 590 row", row: RowS, answered: "023",
			wantText: `driver: connected radio identifies as TS-590SG (CAT ID "023"); you selected TS-590S (CAT ID "021") — wrong radio model on this port`,
		},
		{
			// 020 is the TS-480, built by this milestone and not registered.
			name: "the TS-480", row: RowSG, answered: "020",
			wantText: `driver: connected radio identifies as TS-480 (CAT ID "020"); you selected TS-590SG (CAT ID "023") — wrong radio model on this port`,
		},
		{
			// 024 is the TS-890S, NAMED in its own book — the tier's second
			// pair, which this milestone does not build.
			name: "the TS-890S", row: RowS, answered: "024",
			wantText: `driver: connected radio identifies as TS-890S (CAT ID "024"); you selected TS-590S (CAT ID "021") — wrong radio model on this port`,
		},
		{
			// 022 is the tier's other second-pair sibling, and its book
			// prints the ID BARE with no model name beside it (matrix §1.2),
			// so this refusal names the ID and invents no model for it.
			name: "the bare 022", row: RowS, answered: "022",
			wantText: `driver: connected radio identified as CAT ID "022", want "021" — wrong radio model on this port`,
		},
		{
			name: "an ID in no Kenwood legend", row: RowSG, answered: "999",
			wantText: `driver: connected radio identified as CAT ID "999", want "023" — wrong radio model on this port`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newRespondingPort(t, tc.row, radioImage{catID: tc.answered})
			d := New(tc.row, Simulated, testTiming())
			sess, err := d.Open(context.Background(), p.Port(), driver.Identity{})
			if err == nil {
				_ = sess.Close()
				t.Fatal("Open succeeded against the wrong radio")
			}
			if !errors.Is(err, driver.ErrWrongRadio) {
				t.Errorf("errors.Is(err, driver.ErrWrongRadio) = false for %v", err)
			}
			var wrong *driver.WrongRadioError
			if !errors.As(err, &wrong) {
				t.Fatalf("errors.As(err, **driver.WrongRadioError) = false for %v", err)
			}
			if wrong.Got != tc.answered || wrong.Want != catIDFor(tc.row) {
				t.Errorf("WrongRadioError = Got %q/Want %q, want %q/%q", wrong.Got, wrong.Want, tc.answered, catIDFor(tc.row))
			}
			if got := wrong.Error(); got != tc.wantText {
				t.Errorf("Error() =\n%q\nwant\n%q", got, tc.wantText)
			}
			wantTranscript := []string{"AI0;", "ID;"}
			if got := p.Transcript(); !reflect.DeepEqual(got, wantTranscript) {
				t.Errorf("transcript = %v, want %v — a wrong radio is asked nothing more", got, wantTranscript)
			}
			// The port is closed: Open takes ownership on both outcomes.
			if _, werr := p.Port().Write([]byte("X;")); werr == nil {
				t.Error("the port is still writable after a refused Open")
			}
		})
	}
}

// TestOpen_FVIsParsedUnderA13sGrammar pins what the probe LEARNS from FV, on
// both rows and in both directions.
//
// A13 IS THE GRAMMAR AND IT IS ASSUMED, not printed: the answer chart pins
// the four-character WIDTH (590:1037) and the only format statement anywhere
// is one worked example, "for firmware version 1.00, it reads FV1.00;"
// (590:1035). So the parse may fail without the session failing — see
// TestOpen_AnUnparseableFVOpensASession.
//
// THE SG LEG EXERCISES THE SAME PARSER, NOT A13'S CLAIM. A13 is explicitly
// unclaimed on that row (core/kw/doc.go: no session has read an SG's FV
// answer, and nothing downstream reads FV to decide anything there), and
// parseFirmwareVersion is row-independent; running both rows pins that it
// stays row-independent, and attributes no SG parse to a claim A13 does not
// make.
func TestOpen_FVIsParsedUnderA13sGrammar(t *testing.T) {
	for _, tc := range []struct {
		answer       string
		wantRaw      string
		wantOK       bool
		major, minor int
	}{
		{"FV1.00;", "1.00", true, 1, 0},
		{"FV1.07;", "1.07", true, 1, 7},
		{"FV2.00;", "2.00", true, 2, 0},
		{"FV2.12;", "2.12", true, 2, 12},
		// Four characters that are not M.NN: the width holds, the grammar
		// does not, and A13 is exactly the claim that fails here.
		{"FV1.0 ;", "1.0 ", false, 0, 0},
		{"FVABCD;", "ABCD", false, 0, 0},
		{"FV12.0;", "12.0", false, 0, 0},
	} {
		for _, row := range bothRows {
			sess, _ := openTestSession(t, row, radioImage{fvAnswer: tc.answer})
			if sess.fvAnswer != tc.wantRaw {
				t.Errorf("%s %q: fvAnswer = %q, want the four characters verbatim %q", modelNameFor(row), tc.answer, sess.fvAnswer, tc.wantRaw)
			}
			if sess.fvGrammarOK != tc.wantOK {
				t.Errorf("%s %q: fvGrammarOK = %v, want %v (A13's M.NN form)", modelNameFor(row), tc.answer, sess.fvGrammarOK, tc.wantOK)
			}
			if tc.wantOK && (sess.fvMajor != tc.major || sess.fvMinor != tc.minor) {
				t.Errorf("%s %q: version = %d.%02d, want %d.%02d", modelNameFor(row), tc.answer, sess.fvMajor, sess.fvMinor, tc.major, tc.minor)
			}
		}
	}
}

// TestOpen_AnUnparseableFVOpensASession is the design's asymmetry: degrade
// where the document is thin and we may simply have guessed its grammar
// wrong. Session refusal on FV would make a legitimate TS-590 completely
// unreadable on the strength of A13.
func TestOpen_AnUnparseableFVOpensASession(t *testing.T) {
	for _, row := range bothRows {
		sess, p := openTestSession(t, row, radioImage{fvAnswer: "FVWXYZ;"})
		if sess.fvGrammarOK {
			t.Errorf("%s: an FV of \"WXYZ\" parsed under A13's grammar", modelNameFor(row))
		}
		if sess.fvAnswer != "WXYZ" {
			t.Errorf("%s: fvAnswer = %q; the raw bytes go into the probe note verbatim", modelNameFor(row), sess.fvAnswer)
		}
		want := []string{"AI0;", "ID;", "FV;"}
		if got := p.Transcript(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: transcript = %v, want %v", modelNameFor(row), got, want)
		}
	}
}

// TestOpen_AProbeFrameThatDoesNotANSWERRefusesTheSession is the other side of
// that asymmetry, and it is not the same situation. FV's EXISTENCE is printed
// for both rows (590:1034-1037), as ID's is; a radio that says nothing, or
// rejects, is outside a document that is complete here, and the standing rule
// is to refuse there and degrade only where the document is thin.
//
// IT ALSO PINS THE TYPE, WHICH IS THE POINT OF THE SECOND ASSERTION. The two
// wire events are reported with core/kw's own typed errors on the PROBE path
// exactly as on the read path — a "?;" naming the two indistinguishable
// causes and the transient-suppression sentence, silence saying in as many
// words that it is not an inference of absence — and the command each names
// is the frame that actually failed. errors.Is alone cannot see that: it
// passes on the bare transport error too.
//
// RED PROOF, observed before wireFailure was applied to the probe: both ID
// rows and both FV rows failed at "errors.As(err, **kw.TimeoutError) = false"
// / "**kw.RejectionError = false", the message being the transport's own.
func TestOpen_AProbeFrameThatDoesNotANSWERRefusesTheSession(t *testing.T) {
	for _, tc := range []struct {
		name    string
		img     radioImage
		is      error
		command string
	}{
		{"ID silence", radioImage{idSilent: true}, transport.ErrTimeout, "ID"},
		{"ID rejected", radioImage{idReject: true}, transport.ErrRejected, "ID"},
		{"FV silence", radioImage{fvSilent: true}, transport.ErrTimeout, "FV"},
		{"FV rejected", radioImage{fvReject: true}, transport.ErrRejected, "FV"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newRespondingPort(t, RowS, tc.img)
			d := New(RowS, Simulated, testTiming())
			sess, err := d.Open(context.Background(), p.Port(), driver.Identity{})
			if err == nil {
				_ = sess.Close()
				t.Fatalf("Open succeeded against a radio that never answered %s;", tc.command)
			}
			if !errors.Is(err, tc.is) {
				t.Errorf("errors.Is(err, %v) = false for %v", tc.is, err)
			}
			var (
				rejection *kw.RejectionError
				timeout   *kw.TimeoutError
				gotCmd    string
			)
			switch {
			case errors.As(err, &rejection):
				gotCmd = rejection.Command
			case errors.As(err, &timeout):
				gotCmd = timeout.Command
			default:
				t.Fatalf("neither *kw.RejectionError nor *kw.TimeoutError for %v — the probe path must type these two the way the read path does", err)
			}
			if gotCmd != tc.command {
				t.Errorf("the typed error names command %q, want %q", gotCmd, tc.command)
			}
		})
	}
}

// TestOpen_ConsentTransformsTheSessionSetOnly pins the one route past the
// Unverified labels, and that it is a statement about a SESSION: the static
// Capabilities internal/wiring publishes is untouched.
func TestOpen_ConsentTransformsTheSessionSetOnly(t *testing.T) {
	for _, row := range bothRows {
		sess, _ := openTestSession(t, row, radioImage{}, func(d *ts590Driver) { d.profile = RealHardware })
		mem, _ := sess.Capabilities().Bank(spec.BankMemory)
		if mem.Fields[spec.FieldFrequency].CanWrite() {
			t.Errorf("%s: an UNCONSENTED RealHardware session can write", modelNameFor(row))
		}

		p := newRespondingPort(t, row, radioImage{})
		d := New(row, RealHardware, testTiming(), WithConsentedUnverifiedWrites())
		opened, err := d.Open(context.Background(), p.Port(), driver.Identity{})
		if err != nil {
			t.Fatalf("%s: Open: %v", modelNameFor(row), err)
		}
		t.Cleanup(func() { _ = opened.Close() })
		cmem, _ := opened.Capabilities().Bank(spec.BankMemory)
		if !cmem.Fields[spec.FieldFrequency].CanWrite() {
			t.Errorf("%s: a CONSENTED RealHardware session still cannot write frequency", modelNameFor(row))
		}
		if cmem.Fields[spec.FieldErase].CanWrite() {
			t.Errorf("%s: consent minted an erase", modelNameFor(row))
		}
		if smem, _ := d.Capabilities().Bank(spec.BankMemory); smem.Fields[spec.FieldFrequency].CanWrite() {
			t.Errorf("%s: consent reached the STATIC capability set", modelNameFor(row))
		}
	}
}

// TestOpen_ConsentIsSkippedForAnUnrecognisedProfile: the fail-safe direction
// survives consent.
func TestOpen_ConsentIsSkippedForAnUnrecognisedProfile(t *testing.T) {
	p := newRespondingPort(t, RowSG, radioImage{})
	d := New(RowSG, Profile(9), testTiming(), WithConsentedUnverifiedWrites())
	sess, err := d.Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	mem, _ := sess.Capabilities().Bank(spec.BankMemory)
	if mem.Fields[spec.FieldFrequency].CanWrite() {
		t.Error("an unrecognised Profile with consent produced a writable session")
	}
}

// TestSession_DiagnosticsAndClose pins the two small session surfaces: the
// optional diagnostics interface, and that Close is idempotent.
func TestSession_DiagnosticsAndClose(t *testing.T) {
	sess, _ := openTestSession(t, RowS, radioImage{})
	var _ driver.DiagnosticsReporter = sess
	if got := sess.Diagnostics().UnexpectedFrames; got != 0 {
		t.Errorf("UnexpectedFrames = %d on a clean session, want 0", got)
	}
	first := sess.Close()
	if second := sess.Close(); !errors.Is(second, first) && second != first {
		t.Errorf("Close is not idempotent: %v then %v", first, second)
	}
}

var (
	_ driver.Driver                = (*ts590Driver)(nil)
	_ driver.Session               = (*Session)(nil)
	_ driver.SerialFramingReporter = (*ts590Driver)(nil)
	_ driver.DiagnosticsReporter   = (*Session)(nil)
)

// TestOpen_TheFVAnswerIsReadableForTheProbeNote pins the design's stated
// mitigation for the S row's firmware-blind filter grade: the raw FV bytes
// reach a caller REGARDLESS OF PARSE, so an owner of a 2.xx TS-590S can see
// that their radio has a capability this row does not publish.
//
// The unparseable row is the one that matters — a session whose grammar
// assumption (A13) failed must still be able to report what it was told.
func TestOpen_TheFVAnswerIsReadableForTheProbeNote(t *testing.T) {
	for _, tc := range []struct{ answer, want string }{
		{"FV1.00;", "1.00"},
		{"FV2.12;", "2.12"},
		{"FVWXYZ;", "WXYZ"}, // A13's grammar fails; the bytes still surface
	} {
		for _, row := range bothRows {
			sess, _ := openTestSession(t, row, radioImage{fvAnswer: tc.answer})
			if got := sess.FirmwareAnswer(); got != tc.want {
				t.Errorf("%s %q: FirmwareAnswer() = %q, want the four characters verbatim %q", modelNameFor(row), tc.answer, got, tc.want)
			}
		}
	}
}

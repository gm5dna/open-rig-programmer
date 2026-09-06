// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// probeFrames is Open's whole choreography: THREE frames go out and the probe
// is two of them (plan P9). AI0; is transport.Engine.Init's own frame, filed
// by matrix §3.6 as session init rather than as part of the probe.
var probeFrames = []string{"AI0;", "ID;", "TY;"}

// testTiming keeps a scripted exchange's deadlines short. transport's own
// DefaultTimeout is one second and its DefaultSettle 20 ms, which are radio
// paces; a net.Pipe answers instantly, and a whole-bank read at the fleet
// defaults would spend most of a minute settling.
func testTiming() Option { return withTiming(80*time.Millisecond, time.Millisecond) }

// TestOpen_ProbeTranscriptIsAI0ThenIDThenTY is P9's transcript, and the ONE
// frame that differs from the 590 pair's is the third: this book prints no FV
// command anywhere and no firmware statement anywhere either (erratum E15), so
// TY — a HARDWARE VARIANT read (480:1621-1634) — is what stands in its place.
func TestOpen_ProbeTranscriptIsAI0ThenIDThenTY(t *testing.T) {
	_, p := openSession(t, Simulated, radioImage{})
	if got := p.Transcript(); !reflect.DeepEqual(got, probeFrames) {
		t.Errorf("transcript = %v, want %v", got, probeFrames)
	}
}

// TestOpen_NoFVFrameIsEverBuilt is E15's driver-level consequence, asserted
// from the codec's own refusal rather than from the transcript alone: a driver
// that reached for FV; on this row would be emitting three bytes the 2003
// document never prints.
func TestOpen_NoFVFrameIsEverBuilt(t *testing.T) {
	_, err := layout().BuildFVRead()
	if err == nil {
		t.Fatal("layout().BuildFVRead() succeeded; the TS-480 has no FV command at all (E15)")
	}
	_, p := openSession(t, Simulated, radioImage{})
	for _, frame := range p.Transcript() {
		if strings.HasPrefix(frame, "FV") {
			t.Errorf("the probe sent %q; this book prints no FV command (E15)", frame)
		}
	}
}

// TestOpen_IdentityReachesTheSession pins that the probed CATID and the
// caller-supplied port path both land on driver.Identity.
func TestOpen_IdentityReachesTheSession(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{})
	id := sess.Identity()
	if id.CATID != catID {
		t.Errorf("Identity().CATID = %q, want %q", id.CATID, catID)
	}
	if id.Port != "/dev/test" {
		t.Errorf("Identity().Port = %q, want the caller's own", id.Port)
	}
}

// TestOpen_WrongRadio: an ID answer that is not 020 refuses before any other
// frame, names the sibling where a document prints a name for it, and closes
// the port.
//
// 022 IS DELIBERATELY UNNAMED. The design requires 022 and 024 to be named
// explicitly when either is what answered, and both ARE — by their ID, which
// is the refusal's own Got field. What differs is that 024 additionally has a
// printed model name in the TS-890S book, while 022 is printed bare in the
// TS-990S book with no model name beside it (matrix §1.2). Supplying a name
// here would put one in a manufacturer's mouth on the strength of which book
// the token was found in.
func TestOpen_WrongRadio(t *testing.T) {
	for _, tc := range []struct {
		got      string
		gotModel string
	}{
		{"021", "TS-590S"},
		{"023", "TS-590SG"},
		{"022", ""},
		{"024", "TS-890S"},
		{"099", ""},
	} {
		p := newRespondingPort(t, radioImage{catID: tc.got})
		d := New(Simulated, testTiming())
		_, err := d.Open(context.Background(), p.Port(), driver.Identity{})
		if !errors.Is(err, driver.ErrWrongRadio) {
			t.Fatalf("ID %s: err = %v, want driver.ErrWrongRadio", tc.got, err)
		}
		var wrong *driver.WrongRadioError
		if !errors.As(err, &wrong) {
			t.Fatalf("ID %s: err is not a *driver.WrongRadioError", tc.got)
		}
		if wrong.Got != tc.got || wrong.Want != catID {
			t.Errorf("ID %s: Got/Want = %q/%q, want %q/%q", tc.got, wrong.Got, wrong.Want, tc.got, catID)
		}
		if wrong.GotModel != tc.gotModel {
			t.Errorf("ID %s: GotModel = %q, want %q", tc.got, wrong.GotModel, tc.gotModel)
		}
		if wrong.WantModel != modelName {
			t.Errorf("ID %s: WantModel = %q, want %q", tc.got, wrong.WantModel, modelName)
		}
		// NOTHING MORE IS SENT: a wrong radio receives the preamble and
		// "ID;" and no third frame.
		if got := p.Transcript(); !reflect.DeepEqual(got, []string{"AI0;", "ID;"}) {
			t.Errorf("ID %s: transcript = %v, want [AI0; ID;] — no TY may follow a wrong identity", tc.got, got)
		}
	}
}

// TestOpen_TheFourPrintedVariantsAreAccepted walks P2's whole printed legend,
// "0: TS-480HX (200 W)" … "3: Japanese 20 W type" (480:1626-1629).
//
// ONE ROW FOR FOUR VARIANTS (decision 4, §1.1): the neutral memory model
// expresses none of the difference — same ID, same 50-byte record, same 00-99
// space — so the variant goes in the probe note and never in the registry key.
func TestOpen_TheFourPrintedVariantsAreAccepted(t *testing.T) {
	for _, tc := range []struct {
		variant byte
		name    string
	}{
		{'0', "TS-480HX (200 W)"},
		{'1', "TS-480SAT (100 W + AT)"},
		{'2', "Japanese 50 W type"},
		{'3', "Japanese 20 W type"},
	} {
		sess, _ := openSession(t, Simulated, radioImage{tyAnswer: "TY00" + string(tc.variant) + ";"})
		got := sess.Variant()
		if got.Variant != tc.variant {
			t.Errorf("Variant().Variant = %q, want %q", got.Variant, tc.variant)
		}
		if got.VariantName() != tc.name {
			t.Errorf("Variant().VariantName() = %q, want %q (480:1626-1629)", got.VariantName(), tc.name)
		}
		if got.Reserved != "00" {
			t.Errorf("Variant().Reserved = %q, want the two bytes the image served", got.Reserved)
		}
		// The model NEVER varies with the variant.
		if sess.Capabilities().Model != modelName {
			t.Errorf("variant %q changed the registry key to %q", tc.variant, sess.Capabilities().Model)
		}
	}
}

// TestOpen_AnUnexpectedTYVariantREFUSESTheSession is the asymmetry the design
// states in one place (spec §Error handling): a malformed TY and a malformed
// FV are NOT treated alike.
//
// P2 IS A PRINTED FOUR-VALUE LEGEND (480:1626-1629), not an assumed grammar,
// so an unexpected value means an UNREAD VARIANT whose capability table this
// programme would be inventing — and the rule is to REFUSE where the document
// is complete and we are outside it. The 590 pair's FV takes the other branch
// for the other reason: its grammar is A13, assumed from a single worked
// example, so an unparseable answer degrades rather than refusing a whole
// legitimate radio.
//
// THE PORT IS CLOSED AND NO FOURTH FRAME GOES OUT.
func TestOpen_AnUnexpectedTYVariantREFUSESTheSession(t *testing.T) {
	for _, answer := range []string{"TY004;", "TY009;", "TY00X;", "TY00 ;"} {
		p := newRespondingPort(t, radioImage{tyAnswer: answer})
		d := New(Simulated, testTiming())
		sess, err := d.Open(context.Background(), p.Port(), driver.Identity{})
		if err == nil {
			_ = sess.Close()
			t.Fatalf("TY answer %q opened a session; decision 4 refuses a fifth variant rather than reporting it opaquely or defaulting it to one of the four", answer)
		}
		if !errors.Is(err, kw.ErrParse) {
			t.Errorf("TY answer %q: err = %v, want the codec's typed parse refusal", answer, err)
		}
		if !strings.Contains(err.Error(), "480:1626-1629") {
			t.Errorf("TY answer %q: err = %v, want the refusal to cite the printed legend", answer, err)
		}
		if got := p.Transcript(); !reflect.DeepEqual(got, probeFrames) {
			t.Errorf("TY answer %q: transcript = %v, want exactly the probe's three frames", answer, got)
		}
	}
}

// TestOpen_AFiveOrSevenByteTYRefusesTheSession is the width half of the same
// grammar, and it is TWO facts rather than one — the shape
// core/driver/ts590's short-MR pin has, one command over.
//
// FIRST, THE CORRELATION FACT. The probe's spec is
// kw.PrefixLenMatcher("TY", kw.TYAnswerLen), an EXACT width, so a five- or
// seven-byte "TY…" is not this read's answer at all: it is never delivered,
// the probe times out after its one retry, and the parser is never given the
// chance to interpret a frame the command did not ask for. THE SESSION IS
// REFUSED EITHER WAY, which is what the plan's bullet asks for; what this half
// pins is that the refusal arrives as a TIMEOUT rather than as a variant
// verdict, because a frame of the wrong width is not a TY answer this
// programme can read a variant out of.
//
// SECOND, THE WIDTH PREDICATE ITSELF, on the same bytes: the answer chart is
// "T Y P1 P1 P2 ;", SIX bytes (480:1634), and kw.ParseTYAnswer refuses
// anything else before it looks at P2 at all. That ordering is why a five-byte
// frame's message names the width and not a variant.
func TestOpen_AFiveOrSevenByteTYRefusesTheSession(t *testing.T) {
	for _, answer := range []string{"TY01;", "TY00011;"} {
		p := newRespondingPort(t, radioImage{tyAnswer: answer})
		d := New(Simulated, testTiming())
		sess, err := d.Open(context.Background(), p.Port(), driver.Identity{})
		if err == nil {
			_ = sess.Close()
			t.Fatalf("TY answer %q (%d bytes) opened a session; the chart prints six (480:1634)", answer, len(answer))
		}
		var to *kw.TimeoutError
		if !errors.As(err, &to) {
			t.Errorf("TY answer %q: err = %v (%T), want a typed timeout — an exact-width matcher must not correlate it", answer, err, err)
		}
		if got := p.Transcript(); len(got) != 4 {
			t.Errorf("TY answer %q: transcript = %v, want AI0;/ID; and TWO TY attempts (the probe's one retry)", answer, got)
		}

		// The codec's own width predicate, on the same bytes.
		if _, perr := layout().ParseTYAnswer([]byte(answer)); !errors.Is(perr, kw.ErrParse) {
			t.Errorf("ParseTYAnswer(%q) = %v, want a parse refusal naming the width", answer, perr)
		}
	}
}

// TestOpen_AProbeFrameThatDoesNotANSWERRefusesTheSession pins the TWO
// transport-level events on BOTH probe frames, and that each is TYPED in this
// family's own vocabulary rather than left as the transport's bare error.
//
// A "?;" is *kw.RejectionError, which names the two indistinguishable causes
// the book prints (480:130-135) and cites the transient-suppression sentence
// (480:136-138); silence is *kw.TimeoutError, which says in as many words
// that it is not an inference of absence.
func TestOpen_AProbeFrameThatDoesNotANSWERRefusesTheSession(t *testing.T) {
	for _, tc := range []struct {
		name    string
		img     radioImage
		want    []string
		command string
		reject  bool
	}{
		// A REJECTION IS DEFINITIVE AND IS NEVER RETRIED; SILENCE DRAWS THE
		// ONE RETRY the identity specs carry, which is why the silent rows
		// expect the probe frame twice. A read is idempotent and Open should
		// survive a single swallowed reply — what the retry does NOT do is
		// turn silence into information.
		{"ID rejected", radioImage{idReject: true}, []string{"AI0;", "ID;"}, "ID", true},
		{"ID silent", radioImage{idSilent: true}, []string{"AI0;", "ID;", "ID;"}, "ID", false},
		{"TY rejected", radioImage{tyReject: true}, probeFrames, "TY", true},
		{"TY silent", radioImage{tySilent: true}, append(append([]string{}, probeFrames...), "TY;"), "TY", false},
	} {
		p := newRespondingPort(t, tc.img)
		d := New(Simulated, testTiming())
		sess, err := d.Open(context.Background(), p.Port(), driver.Identity{})
		if err == nil {
			_ = sess.Close()
			t.Fatalf("%s: Open succeeded", tc.name)
		}
		if tc.reject {
			var rej *kw.RejectionError
			if !errors.As(err, &rej) {
				t.Errorf("%s: err = %v (%T), want *kw.RejectionError", tc.name, err, err)
			} else if rej.Command != tc.command {
				t.Errorf("%s: rejection names command %q, want %q", tc.name, rej.Command, tc.command)
			}
		} else {
			var to *kw.TimeoutError
			if !errors.As(err, &to) {
				t.Errorf("%s: err = %v (%T), want *kw.TimeoutError", tc.name, err, err)
			} else if to.Command != tc.command {
				t.Errorf("%s: timeout names command %q, want %q", tc.name, to.Command, tc.command)
			}
		}
		if got := p.Transcript(); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: transcript = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestOpen_TheTYAnswerIsCarriedOpaquely is decision 4's P1 half: the two
// bytes are printed "Reserved" (480:1623) and this programme reports them
// without interpreting them — including bytes above 0x7E, which no other
// field in the Kenwood codec admits.
//
// THE ACCESSOR EXISTS BECAUSE THE ANSWER IS A REPORTED FACT (§1.1: "The
// variant goes in the probe note, never in the key"), and it is the half a
// driver package can honestly own; RENDERING it is the registration task's
// (T18), exactly as Session.FirmwareAnswer is on the 590 pair. A caller that
// renders Reserved for a human MUST %q-quote it — kw.TYAnswer's own doc
// comment carries that obligation, and this driver's error paths obey it.
func TestOpen_TheTYAnswerIsCarriedOpaquely(t *testing.T) {
	for _, reserved := range []string{"00", "AB", "\x7f\xff", "  "} {
		sess, _ := openSession(t, Simulated, radioImage{tyAnswer: "TY" + reserved + "1;"})
		if got := sess.Variant().Reserved; got != reserved {
			t.Errorf("Variant().Reserved = %q, want %q verbatim", got, reserved)
		}
	}
}

// TestSession_DiagnosticsAndClose: the engine's counters reach a caller
// through the optional reporter, and Close is idempotent.
func TestSession_DiagnosticsAndClose(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{})
	if got := sess.Diagnostics().UnexpectedFrames; got != 0 {
		t.Errorf("UnexpectedFrames = %d on a fresh session, want 0", got)
	}
	first := sess.Close()
	if second := sess.Close(); !errors.Is(second, first) && second != first {
		t.Errorf("Close twice = %v then %v, want the same result", first, second)
	}
}

// TestOpen_ARowNeedsNoRowArgument records the ONE structural difference from
// core/driver/ts590's constructor, so that a reader moving between the two
// packages does not look for a Row here.
//
// core/driver/ts590.New takes a REQUIRED Row because one package serves two
// registry rows that differ on byte 28's write policy and on their slot
// ceilings (P1). This package serves ONE row: TY's four printed variants
// (480:1626-1629) are not registry rows — the neutral memory model expresses
// none of the difference between them — so there is nothing for a caller to
// choose and nothing to fail closed on.
func TestOpen_ARowNeedsNoRowArgument(t *testing.T) {
	var d driver.Driver = New(Simulated)
	if got := d.Model(); got != modelName {
		t.Errorf("Model() = %q, want %q on a driver built with no row argument", got, modelName)
	}
	if _, ok := any(d).(interface{ StopBits() int }); !ok {
		t.Error("the driver lost driver.SerialFramingReporter")
	}
	// The framing this driver builds is the LAYOUT's, never the
	// envelope-only kw.NewFraming(book): internal/guards' own
	// TestKenwoodDriversUseNewFramingFor holds the production half down,
	// and this is the behavioural half — the gate refuses a frame the
	// envelope would admit.
	framing, err := kw.NewFramingFor(layout())
	if err != nil {
		t.Fatalf("kw.NewFramingFor: %v", err)
	}
	if framing.Allow([]byte("MW" + strings.Repeat("0", 39) + ";")) {
		t.Error("this row's framing admits a 42-byte MW; NewFramingFor puts the eight per-row grammars in front of the envelope")
	}
}

// TestOpen_TakesOwnershipOfThePortOnBothOutcomes: Open closes the port itself
// before returning an error, and the Session's Close releases it on success.
func TestOpen_TakesOwnershipOfThePortOnBothOutcomes(t *testing.T) {
	p := newRespondingPort(t, radioImage{catID: "021"})
	d := New(Simulated, testTiming())
	if _, err := d.Open(context.Background(), p.Port(), driver.Identity{}); err == nil {
		t.Fatal("Open succeeded against a TS-590S")
	}
	if _, err := p.Port().Write([]byte("ID;")); err == nil {
		t.Error("the port is still writable after a failed Open; Open owns it on both outcomes")
	}
}

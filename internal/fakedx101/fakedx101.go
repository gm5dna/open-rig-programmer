// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import (
	"io"
	"net"
	"sync"
	"time"
)

// catIDLen is the width of the ID answer's P1 field: four bytes, from that
// command's own chart (layout 1069-1078, the Answer row "I D P1 P1 P1 P1 ;").
const catIDLen = 4

// The two CAT IDs, from ID's P1 legend: "0681: FTDX101D" (layout 1070) and
// "0682: FTDX101MP" (layout 1072). This is the FIRST of the three places the
// manual distinguishes the models and the only one this package can express —
// docs/superpowers/m9d2-capability-matrix.md §1.2 and §4.
//
// They are written out here rather than fetched from core/cat/ftdx101's
// dialect, which also carries them, because THE HARD RULE forbids importing it
// (doc.go). The cross-check that the two spellings agree is
// core/driver/ftdx101's ID probe run against this fake: a mistyped digit here
// makes an FTdx101 driver refuse its own simulator with a
// *driver.WrongRadioError naming both models, which is a loud failure by
// construction.
const (
	catIDD  = "0681"
	catIDMP = "0682"
)

// Radio is a simulated FTdx101 — a D or an MP, per the constructor used: an
// in-memory duplex pipe presenting the host end via Port(), serviced from the
// Radio's own goroutine using this package's independent parser (parser.go). A
// Radio is safe for concurrent use: SlotState, CurrentChannel and Close may all
// be called from goroutines other than whatever is reading or writing Port()
// (run tests with -race).
type Radio struct {
	hostConn net.Conn // returned by Port(); the caller's end
	fakeConn net.Conn // serviced by serve(); the radio's own end

	// catID is WHICH RADIO THIS IS — "0681" for a D, "0682" for an MP — and
	// it is the whole of the difference between the two models on this wire
	// (parser.go's ID section). Like latency it is populated only while
	// newRadio runs, before serve() starts, and never mutated afterwards, so
	// buildIDAnswer may read it without r.mu.
	//
	// It is deliberately NOT an Option: a fake radio's identity is fixed by
	// which constructor built it, exactly as the driver's is (plan D1, and
	// core/driver/ftdx101/caps.go's Profile comment), so no caller can hand a
	// D radio an MP's identity or leave it blank.
	catID string

	// latency is populated only while newRadio's options run and never
	// mutated afterwards, so serve() may read it without r.mu.
	latency time.Duration

	mu             sync.Mutex
	slots          map[string]MemState
	currentChannel string
	ai             byte              // '0' or '1'; OFF at construction — a manual fact, layout 384
	exSettings     map[string]string // EX (MENU) six-digit address -> raw P4; see ex.go

	// shutdown is closed (exactly once, by closePipes) when the radio goes
	// away. WithLatency's wait selects against it (sleepInterruptible) instead
	// of calling bare time.Sleep, so Close never has to wait out a pending
	// scripted delay before its wg.Wait on serve() can return — the promptness
	// internal/wiring's OpenFakeSessionFor relies on for the FT-710's fake and
	// the FTdx10's, and will rely on for this one.
	shutdown chan struct{}

	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup
}

// NewD constructs a simulated FTDX101D (CAT ID 0681) and starts its servicing
// goroutine. Without a WithFactoryImage option the slot map defaults to
// DefaultImage().
//
// TWO THIN CONSTRUCTORS OVER ONE IMPLEMENTATION, and no exported model enum or
// model Option: which radio a fake is must be fixed by the call that builds it,
// not by a value a caller could leave zero or get wrong. This is plan decision
// D1, and it is the shape core/cat/ftdx101 uses for DialectD/DialectMP
// (dialect.go:67-76) and core/driver/ftdx101 for its own NewD/NewMP.
func NewD(opts ...Option) *Radio { return newRadio(catIDD, opts...) }

// NewMP constructs a simulated FTDX101MP (CAT ID 0682). Same shape and same
// reasoning as NewD; the two radios differ on this wire in the ID answer and
// nowhere else (parser.go's ID section, and
// TestTheTwoModelsDifferOnlyInTheIDAnswer).
func NewMP(opts ...Option) *Radio { return newRadio(catIDMP, opts...) }

// newRadio builds a *Radio answering catID and starts its servicing goroutine.
// Private: the exported constructors above are the only two identities this
// package admits.
//
// It PANICS on a catID that is not four digits, deliberately, in the same
// spirit as image.go's fixture constructors: both call sites are compile-time
// constants of this file, so a bad one is a programming error that must stop
// the programme loudly rather than ship a fake answering a subtly wrong — or a
// wrongly sized — ID frame, which every consumer would then read as "some other
// radio" several layers from the typo.
func newRadio(catID string, opts ...Option) *Radio {
	if len(catID) != catIDLen {
		panic("fakedx101: CAT ID must be four bytes wide — ID's P1 field is four positions (layout 1069-1078)")
	}
	for i := 0; i < len(catID); i++ {
		if !isDigit(catID[i]) {
			panic("fakedx101: CAT ID must be four digits — the legend's own forms are 0681 (FTDX101D) and 0682 (FTDX101MP)")
		}
	}

	hostConn, fakeConn := net.Pipe()
	r := &Radio{
		hostConn: hostConn,
		fakeConn: fakeConn,
		catID:    catID,
		slots:    DefaultImage(),
		// The answer-only none form: what "MC;" reports before any MC-set has
		// happened. The wire spelling is the DIALECT's ASSUMED NoneWire
		// (core/cat/ftdx101/doc.go's "SlotSpace.NoneWire = \"000\"" entry),
		// cited not re-derived — it appears in no FTdx101 slot legend.
		currentChannel: slotNoneWire,
		// OFF at construction, modelling a freshly-powered radio. NOT an
		// assumption on these radios: their own AI page states "This parameter
		// is set to '0' (OFF) automatically when the transceiver is turned
		// 'OFF'" (layout 384), which is why this behaviour is absent from the
		// ASSUMED register — see doc.go's "What is NOT in this register".
		ai: '0',
		// A FRESH COPY per radio, seeded from the generated projection of
		// transcription B (ex.go). EXDefaults() returns an independent map by
		// contract, which is what lets WithEXSetting and WithEXUnavailable
		// overlay one radio's menu without reaching any other radio's — and it
		// is the same map for a D and for an MP, because the chart is printed
		// once for both models.
		exSettings: EXDefaults(),
		shutdown:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(r)
	}

	r.wg.Add(1)
	go r.serve()
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
// Repeated calls return the same connection.
func (r *Radio) Port() io.ReadWriteCloser { return r.hostConn }

// Close shuts the fake radio down: it closes the RADIO's own end of the pipe
// and waits for the servicing goroutine to exit. Safe to call more than once.
//
// Deliberately closes only fakeConn, not hostConn — fakeradio's reasoning,
// verbatim, because it is a property of net.Pipe rather than of any of the
// three radios: Close() only reports io.ErrClosedPipe to a Read or Write made
// against the END YOU YOURSELF closed, while a pending or subsequent Read on
// the other (still-open) end sees io.EOF, which is exactly the signal a host
// should get from "the radio went away". Closing hostConn here too would flip
// that to io.ErrClosedPipe for the caller, a worse and less consistent signal.
// A caller that wants to release its own Port() handle explicitly may still
// call r.Port().Close() itself.
func (r *Radio) Close() error {
	err := r.closePipes()
	r.wg.Wait()
	return err
}

// closePipes is the idempotent, race-safe close of the radio's own pipe end.
// Factored out of the public Close() so that anything running INSIDE serve()
// can shut the pipe without deadlocking: Close() waits on r.wg, which only
// reaches zero once serve() has returned.
func (r *Radio) closePipes() error {
	r.closeOnce.Do(func() {
		// Close shutdown FIRST: a serve goroutine parked in a latency wait
		// wakes immediately, before (or regardless of) noticing the pipe
		// itself closing.
		close(r.shutdown)
		r.closeErr = r.fakeConn.Close()
	})
	return r.closeErr
}

// sleepInterruptible waits d, returning early (false) if the radio's shutdown
// channel closes first. Returns true when the full d genuinely elapsed; d <= 0
// returns true at once.
func (r *Radio) sleepInterruptible(d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-r.shutdown:
		return false
	}
}

// serve is the Radio's own goroutine: it reads from fakeConn, reassembles
// frames, and drives command handling and replies. It is the ONLY goroutine
// that ever reads or writes fakeConn (see rawWrite), so no synchronisation is
// needed around the connection itself — only around the shared state behind
// Radio.mu, which SlotState and CurrentChannel also touch from test goroutines.
func (r *Radio) serve() {
	defer r.wg.Done()

	acc := newReassembler(maxAccumulatorBytes)
	buf := make([]byte, 4096)
	for {
		n, err := r.fakeConn.Read(buf)
		if n > 0 {
			for _, ev := range acc.push(buf[:n]) {
				r.handleEvent(ev)
			}
		}
		if err != nil {
			return
		}
	}
}

// handleEvent processes one reassembler event — a complete frame, or an
// accumulator overflow — and sends whatever reply it produces.
//
// A nil reply is silence, and silence is SUCCESS for every Set this radio takes
// (doc.go register entry 11): the accepted-Set path and the nothing-to-say path
// are the same path, deliberately, so a handler cannot acknowledge a Set by
// accident.
func (r *Radio) handleEvent(ev accEvent) {
	var reply []byte
	if ev.overflow {
		reply = rejection
	} else {
		reply = r.handleFrame(ev.frame)
	}
	if reply == nil {
		return
	}
	r.rawWrite(reply)
}

// rawWrite sends data to the port, honouring the configured per-reply latency.
// Errors are not reported: a write failing because the peer has gone away
// (closed the port, or stopped reading) is an expected outcome, not a bug in
// the fake. The latency wait is interruptible — a Close mid-wait abandons the
// write, since the pipe is gone and the bytes could never arrive.
func (r *Radio) rawWrite(data []byte) {
	if r.latency > 0 && !r.sleepInterruptible(r.latency) {
		return
	}
	_, _ = r.fakeConn.Write(data)
}

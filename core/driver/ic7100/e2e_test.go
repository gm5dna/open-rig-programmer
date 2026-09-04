// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7100 "github.com/gm5dna/open-rig-programmer/core/civ/ic7100"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7100"
)

// THE TWO WITNESSES MEET, HAVING NEVER READ EACH OTHER.
//
// Every other test in this package answers from respondingport_test.go, a
// table this package wrote, which proves the driver consistent with itself
// and nothing more. internal/fakeic7100 is the OTHER witness: a stdlib-only
// radio derived from the same printed pages by agents who never opened
// core/civ/ic7100, this driver, or its golden vectors. Where the two agree
// here, two independent readings of the same printed pages — PDF p.361
// (folio 20-2), the CI-V connection and data-format page, and PDF p.375
// (folio 20-16), the one-page "Memory content setting" diagram that
// actually carries the record layout — landed in the same place, which
// is evidence; where they disagreed, the disagreement is recorded rather
// than assumed away.
//
// WHERE THIS TEST LIVES AND WHAT IT MAY TOUCH. Wave 3 registers nothing:
// internal/wiring, internal/guards, internal/radiotext, cmd/rigprog and
// app/ are untouched, so the Yaesu tier's wiring-level end-to-end has no
// counterpart available here and the meeting happens in the driver's own
// package. The coupling is the fake's SERIAL PORT and nothing else: the
// driver is handed radio.Port() and never a scripted responder, and
// Transcript/Slot are read only afterwards, to ask the radio what it heard
// and what it kept.
//
// THE FAKE'S ASSUMPTIONS ARE ITS OWN. internal/fakeic7100/doc.go carries a
// register of fourteen ASSUMED behaviours, ten of them sharing a name with
// this model's capability-matrix entries so that ONE capture retires both
// readings at once. The tests below exercise BOTH readings wherever the
// fake offers an option for the other one — FA and all-FF empties, echo and
// no echo, both flood species — because a driver that survives only the
// guess this project happened to make has been proved nothing about.

// e2eOpen starts a fake radio and opens one session onto its serial port.
//
// The session's Close closes the engine, which closes the port, which
// closes the radio — so the radio's own Close is registered too, for the
// paths that never reach a session at all.
func e2eOpen(t *testing.T, profile Profile, driverOpts []Option, fakeOpts ...fakeic7100.Option) (*fakeic7100.Radio, *Session) {
	t.Helper()
	radio := fakeic7100.New(fakeOpts...)
	t.Cleanup(func() { _ = radio.Close() })
	sess, err := New(profile, driverOpts...).Open(context.Background(), radio.Port(), driver.Identity{Port: "fake"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return radio, sess.(*Session)
}

// e2eRecord is one neutral memory record, in THIS side's own vocabulary.
//
// RX and TX carry the same frequency deliberately. The fake refuses a set
// whose transmit duplicate differs from its receive payload — its register
// entry 6, ic7100-tx-block-mandatory, the printed NOTE read as a rule — and
// this profile emits the duplicate from the same fields shifted by 47, so
// the two agree only while the two frequencies agree. A split record is not
// what this tier writes.
func e2eRecord(bank, channel int, freqHz uint64, name string) civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: bank, Channel: channel},
		Select:       civ.Available("OFF"),
		RXFreqHz:     civ.Available(freqHz),
		TXFreqHz:     civ.Available(freqHz),
		OffsetHz:     civ.Available(uint64(600_000)),
		ToneTXDeciHz: civ.Available(uint64(885)),
		ToneRXDeciHz: civ.Available(uint64(885)),
		DTCSCode:     civ.Available(uint64(23)),
		DTCSPolarity: civ.Available("NN"),
		Duplex:       civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		Mode:         civ.Available("FM"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		Name:         civ.Available(name),
	}
}

// e2eRecordBytes renders a record as the 111 raw bytes a memory answer
// carries, using THIS side's builder.
//
// The fake is handed bytes and hands them back; it decodes none of them. So
// this side's encoder is what puts meaning into a seed, and this side's
// decoder is what has to find that meaning again after a round trip through
// a package that never looked at it.
func e2eRecordBytes(t *testing.T, rec civ.MemoryRecord) []byte {
	t.Helper()
	cmd, err := civic7100.Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet(%v): %v", rec.Address, err)
	}
	b := cmd.Bytes()
	// FE FE 88 E0 1A 00 <3 address bytes> = 9 bytes, then the record, then FD.
	return append([]byte(nil), b[9:len(b)-1]...)
}

// e2eBuildableReads precomputes every frame this profile's OWN builders can
// produce for a plain read, so the grammar audit can require EXACT byte
// equality rather than a command-number and length match.
func e2eBuildableReads(t *testing.T) (identity []byte, memory map[string]bool) {
	t.Helper()
	p := civic7100.Profile()
	idCmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	memory = map[string]bool{}
	for bank := 1; bank <= 5; bank++ {
		for channel := 1; channel <= 99; channel++ {
			cmd, err := p.BuildMemoryRead(civ.ChannelAddress{Group: bank, Channel: channel})
			if err != nil {
				continue // an address this profile itself refuses builds nothing to compare against
			}
			memory[string(cmd.Bytes())] = true
		}
	}
	return idCmd.Bytes(), memory
}

// e2eAuditGrammars requires that every frame the fake RECEIVED is one of the
// three this tier can build, and that exactly wantSets of them were sets.
//
// The widths are exact, and that is what makes it an audit rather than a
// command-number check:
//
//	  7 = FE FE 88 E0 19 00 FD                          the identity read
//	 10 = FE FE 88 E0 1A 00 <3 address bytes> FD        a memory read
//	121 = the same, plus the 111-byte record            a memory set
//
// Both printed readings of the clearing form — 1A 00 with a bank byte and
// without — are eleven and ten bytes ending in FF, and the ten-byte one has
// a read's exact width, so neither could pass on length alone. The reads are
// therefore checked byte-for-byte against what this profile's own builders
// emit, and every set against civ.Profile.AllowedCommand, the same gate
// Framing.Allow calls before a write reaches the wire.
func e2eAuditGrammars(t *testing.T, radio *fakeic7100.Radio, wantSets int) {
	t.Helper()
	const (
		identityRead = 7
		memoryRead   = 10
		memorySet    = 4 + 2 + civic7100.AddressBytes + civic7100.RecordLength + 1
	)
	p := civic7100.Profile()
	idFrame, memReads := e2eBuildableReads(t)

	sets := 0
	for _, f := range radio.Transcript() {
		cn, sc, ok := civ.FrameCommand(f)
		switch {
		case ok && cn == 0x19 && sc == 0x00 && len(f) == identityRead:
			if !bytes.Equal(f, idFrame) {
				t.Errorf("identity read % X is not byte-identical to BuildTransceiverIDRead's own % X", f, idFrame)
			}
		case ok && cn == 0x1A && sc == 0x00 && len(f) == memoryRead:
			if !memReads[string(f)] {
				t.Errorf("memory read % X matches no address BuildMemoryRead builds for this profile", f)
			}
		case ok && cn == 0x1A && sc == 0x00 && len(f) == memorySet:
			sets++
			if !p.AllowedCommand(f) {
				t.Errorf("memory set % X is refused by this profile's own AllowedCommand gate — it is not a frame the builder could have produced", f)
			}
		default:
			t.Errorf("the session put % X on the wire — no builder in this tier names that frame", f)
		}
	}
	if sets != wantSets {
		t.Errorf("the radio received %d memory sets, want %d", sets, wantSets)
	}
}

// e2eReads counts the memory reads in a transcript.
func e2eReads(frames [][]byte) int {
	n := 0
	for _, f := range frames {
		if len(f) == 10 && f[4] == 0x1A && f[5] == 0x00 {
			n++
		}
	}
	return n
}

func TestE2E_ProbeFingerprintsFromTheFakesAddressMatchedIdentityReply(t *testing.T) {
	// The probe asks two questions and changes nothing. The identity half
	// is the CI-V ADDRESS — an address-matched 19 00 answer is what the
	// design requires, and the undocumented token is RECORDED, never
	// matched (register entry ic7100-id-reply-value). The fingerprint half
	// is the record-only length of 111, measured after the three address
	// bytes the wire also carries, on an OCCUPIED and DECODABLE record.
	radio, s := e2eOpen(t, Simulated, nil, fakeic7100.WithSlot(1, 1, occupiedRecord(t)))

	d := s.CIVDiagnostics()
	if !d.Fingerprinted || d.Status != "FINGERPRINTED 111 B" {
		t.Errorf("diagnostics = %+v, want a 111-byte fingerprint from the first occupied slot", d)
	}
	if d.ProbeSlotsRead != 1 {
		t.Errorf("ProbeSlotsRead = %d, want 1 — A-001 is occupied and the search is bounded", d.ProbeSlotsRead)
	}
	if d.AnswerMismatches != 0 {
		t.Errorf("AnswerMismatches = %d against a well-behaved radio, want 0", d.AnswerMismatches)
	}
	if d.IDToken == "" {
		t.Error("no identity token was recorded from the fake's 19 00 answer")
	}
	if got, want := s.Identity().CATID, "88:"+d.IDToken; got != want {
		t.Errorf("CATID = %q, want %q — the address is the identity and the token is a diagnostic joined to it", got, want)
	}

	// OPEN PUT NOTHING BUT THE TWO READ GRAMMARS ON THE WIRE, checked on
	// the fake's own transcript rather than on this package's recording
	// port: a second witness, independently authored. This claim is about
	// Open alone; the whole-session claim is the last test in this file.
	e2eAuditGrammars(t, radio, 0)
}

func TestE2E_AnEmptyRadioOpensExplicitlyUnfingerprinted(t *testing.T) {
	// A radio with nothing stored answers FA to every probe read — the
	// fake's register entry 1, ic7100-empty-channel-fa — and the session
	// opens on ADDRESS evidence alone rather than inventing a fingerprint
	// or refusing to open a radio that is merely empty.
	radio, s := e2eOpen(t, Simulated, nil)

	d := s.CIVDiagnostics()
	if d.Fingerprinted || d.Status != "UNFINGERPRINTED" {
		t.Errorf("diagnostics = %+v, want an explicitly unfingerprinted open", d)
	}
	if d.ProbeSlotsRead != probeSlots {
		t.Errorf("ProbeSlotsRead = %d, want the bounded search's full %d", d.ProbeSlotsRead, probeSlots)
	}
	e2eAuditGrammars(t, radio, 0)
}

func TestE2E_TheIdentityTokenIsRecordedNotMatched(t *testing.T) {
	// The 19 00 answer's data bytes are UNDOCUMENTED on both sides: the
	// command table's Data column is blank, so the fake's DE AD default is
	// an invention it says is one, and this driver must record whatever it
	// is handed. A driver that matched a value would fail here and would
	// have been wrong on hardware for a reason no capture could have found.
	_, s := e2eOpen(t, Simulated, nil,
		fakeic7100.WithIdentityToken([]byte{0x12, 0x34}),
		fakeic7100.WithSlot(1, 1, occupiedRecord(t)),
	)
	d := s.CIVDiagnostics()
	if !strings.Contains(strings.ToUpper(d.IDToken), "1234") {
		t.Errorf("IDToken = %q, want the pinned 12 34 the radio actually answered", d.IDToken)
	}
	if !strings.Contains(s.Identity().CATID, "1234") {
		t.Errorf("CATID = %q, want the answered token joined to the address", s.Identity().CATID)
	}
}

func TestE2E_ReadAllWalksEveryDeclaredSlot(t *testing.T) {
	// EVERY SLOT THE CAPABILITIES DECLARE — all 495 of the dense A-001 to
	// E-099 space — against a radio that holds three of them, on three
	// different banks. The bank half of the address is the whole reason
	// this model's slot spelling carries one: a driver that dropped it
	// would read five different memories as one.
	first := e2eRecordBytes(t, e2eRecord(1, 1, 145_500_000, "BANK A FIRST"))
	middle := e2eRecordBytes(t, e2eRecord(3, 50, 433_500_000, "BANK C MIDDLE"))
	last := e2eRecordBytes(t, e2eRecord(5, 99, 50_150_000, "BANK E LAST"))

	radio, s := e2eOpen(t, Simulated, nil,
		fakeic7100.WithSlot(1, 1, first),
		fakeic7100.WithSlot(3, 50, middle),
		fakeic7100.WithSlot(5, 99, last),
	)
	probeReads := e2eReads(radio.Transcript())

	cp, err := clone.NewService(s, clone.SnapshotStore{}).ReadAll(context.Background())
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(cp.Channels) != 495 {
		t.Fatalf("channels = %d, want the 495 dense slots the capabilities declare", len(cp.Channels))
	}
	if cp.Channels[0].Slot != "A-001" || cp.Channels[494].Slot != "E-099" {
		t.Fatalf("walk ran %q..%q, want A-001..E-099", cp.Channels[0].Slot, cp.Channels[494].Slot)
	}

	occupied := map[string]struct {
		freq uint64
		tag  string
	}{
		"A-001": {145_500_000, "BANK A FIRST"},
		"C-050": {433_500_000, "BANK C MIDDLE"},
		"E-099": {50_150_000, "BANK E LAST"},
	}
	found := 0
	for _, ch := range cp.Channels {
		want, seeded := occupied[ch.Slot]
		if !seeded {
			// R10 end to end: an unwritten channel is an EMPTY channel, not
			// an error that aborts the walk on the first one — which, on any
			// real radio, is most of them.
			if ch.Data != nil {
				t.Errorf("%s came back populated; only the three seeded channels hold anything", ch.Slot)
			}
			continue
		}
		found++
		if ch.Data == nil {
			t.Errorf("%s came back empty, want the seeded record", ch.Slot)
			continue
		}
		if ch.Data.FreqHz != want.freq || ch.Data.Tag != want.tag {
			t.Errorf("%s = %d Hz %q, want %d Hz %q", ch.Slot, ch.Data.FreqHz, ch.Data.Tag, want.freq, want.tag)
		}
		if ch.Data.Mode != "FM" || ch.Data.Filter.Value != "FIL1" {
			t.Errorf("%s = mode %q filter %+v, want FM/FIL1 — the tier fields survive the round trip too", ch.Slot, ch.Data.Mode, ch.Data.Filter)
		}
		// EVERY TIER FIELD THE SEED CARRIES, not a sample of them. The
		// seeded record holds 88.5 Hz on both tones, DTCS 023 NN, duplex
		// OFF, tone mode OFF and data mode OFF; a read that quietly turned
		// any of them Unknown — which is exactly what a tone domain too
		// narrow for the radio's own chart does, since Session.toneField
		// downgrades a tone the capabilities will not admit — would have
		// passed a mode-and-filter check unnoticed.
		if ch.Data.ToneTx.State != codeplug.Known || ch.Data.ToneTx.Value != 885 {
			t.Errorf("%s: ToneTx = %+v, want Known 885 — the seed's 88.5 Hz", ch.Slot, ch.Data.ToneTx)
		}
		if ch.Data.ToneRx.State != codeplug.Known || ch.Data.ToneRx.Value != 885 {
			t.Errorf("%s: ToneRx = %+v, want Known 885 — the seed's 88.5 Hz", ch.Slot, ch.Data.ToneRx)
		}
		if ch.Data.DTCSCode.State != codeplug.Known || ch.Data.DTCSCode.Value != 23 {
			t.Errorf("%s: DTCSCode = %+v, want Known 23 — the seed's 023", ch.Slot, ch.Data.DTCSCode)
		}
		if ch.Data.DTCSPolarity.State != codeplug.Known || ch.Data.DTCSPolarity.Value != "NN" {
			t.Errorf("%s: DTCSPolarity = %+v, want Known NN", ch.Slot, ch.Data.DTCSPolarity)
		}
		if ch.Data.Duplex.State != codeplug.Known || ch.Data.Duplex.Value != "OFF" {
			t.Errorf("%s: Duplex = %+v, want Known OFF", ch.Slot, ch.Data.Duplex)
		}
		if ch.Data.ToneMode.State != codeplug.Known || ch.Data.ToneMode.Value != "OFF" {
			t.Errorf("%s: ToneMode = %+v, want Known OFF", ch.Slot, ch.Data.ToneMode)
		}
		if ch.Data.DataMode.State != codeplug.Known || ch.Data.DataMode.Value {
			t.Errorf("%s: DataMode = %+v, want Known false", ch.Slot, ch.Data.DataMode)
		}
		if ch.Data.OffsetHz.State != codeplug.Known || ch.Data.OffsetHz.Value != 600_000 {
			t.Errorf("%s: OffsetHz = %+v, want Known 600000", ch.Slot, ch.Data.OffsetHz)
		}
		if ch.Data.TxFreqHz.State != codeplug.Known || ch.Data.TxFreqHz.Value != want.freq {
			t.Errorf("%s: TxFreqHz = %+v, want the duplicate block's own %d", ch.Slot, ch.Data.TxFreqHz, want.freq)
		}
	}
	if found != len(occupied) {
		t.Errorf("the walk visited %d of the %d seeded slots", found, len(occupied))
	}

	// The radio was asked for each declared slot exactly once, and for
	// nothing else. The fake refuses an address outside banks 01–05 and
	// channels 0001–0099 outright (its register entry 10), so an
	// out-of-scope read would also have come back FA and been silently
	// reported as an empty channel — the count is what makes it visible.
	if got := e2eReads(radio.Transcript()) - probeReads; got != 495 {
		t.Errorf("the walk sent %d memory reads, want exactly 495", got)
	}
	e2eAuditGrammars(t, radio, 0)
}

func TestE2E_OneConsentedWriteIsAcknowledgedAndReadsBack(t *testing.T) {
	// ONE consented write, on a REAL-HARDWARE profile: consent is what
	// opens the gate while writeTrialsComplete is false, and the far end
	// being a simulator is not itself a licence.
	radio, s := e2eOpen(t, RealHardware, []Option{WithConsentedUnverifiedWrites()},
		fakeic7100.WithSlot(1, 1, occupiedRecord(t)))

	ch := writableChannel(t, s)
	result, err := s.WriteChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(result.Steps) != 1 || !result.Steps[0].Sent || !result.Steps[0].Confirmed {
		t.Fatalf("steps = %+v, want one sent and FB-confirmed frame", result.Steps)
	}

	back, err := s.ReadChannel(context.Background(), "A-001")
	if err != nil {
		t.Fatalf("ReadChannel after write: %v", err)
	}
	if back.Data == nil {
		t.Fatal("the slot read back empty after an acknowledged write")
	}
	if back.Data.Tag != "WRITE TEST" || back.Data.FreqHz != ch.Data.FreqHz || back.Data.Mode != ch.Data.Mode {
		t.Errorf("read back %q/%d/%q, want %q/%d/%q", back.Data.Tag, back.Data.FreqHz, back.Data.Mode,
			"WRITE TEST", ch.Data.FreqHz, ch.Data.Mode)
	}

	// What the RADIO kept, asked of the radio rather than of the answer it
	// gave back. The transmit duplicate is the fake's own acceptance rule
	// (its register entry 6) and this profile's +47 arithmetic; the two
	// were derived separately and this is where they have to agree.
	stored, occupied := radio.Slot(1, 1)
	if !occupied {
		t.Fatal("the radio holds nothing at A-001 after an acknowledged set")
	}
	if len(stored) != civic7100.RecordLength {
		t.Fatalf("stored record is %d bytes, want %d", len(stored), civic7100.RecordLength)
	}
	if !bytes.Equal(stored[1:48], stored[48:95]) {
		t.Error("the stored record's transmit duplicate does not equal its receive payload")
	}

	// Exactly one set, never resent: RetryReads is zero on this class, so a
	// retransmission is not representable in the spec — but the property
	// worth pinning is the one on the WIRE, because resending an accepted
	// set writes the channel twice, and the radio's transcript is the only
	// vantage point from which the wire can be counted.
	if got := countSets(radio.Transcript()); got != 1 {
		t.Errorf("the radio received %d sets, want exactly 1", got)
	}
}

func TestE2E_TheSameWriteIsRefusedWithoutConsent(t *testing.T) {
	// The other profile over the same radio refuses the same write, before
	// any wire traffic. Nothing about the far end being a simulator changes
	// what an unconsented real-hardware session may do.
	radio, s := e2eOpen(t, RealHardware, nil, fakeic7100.WithSlot(1, 1, occupiedRecord(t)))
	ch := writableChannel(t, s)
	before := len(radio.Transcript())

	result, err := s.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want ErrWriteRefused on an unconsented real-hardware session", err)
	}
	if len(result.Steps) != 0 {
		t.Errorf("result = %+v, want no steps at all", result)
	}
	if got := len(radio.Transcript()) - before; got != 0 {
		t.Errorf("the refusal put %d frames on the wire, want none", got)
	}
}

func TestE2E_TheRadiosFARefusalOfASetIsReportedAndNeverRetransmitted(t *testing.T) {
	// A radio that will not store what it was sent. The fake accepts a set
	// only at the record length it was told to accept — its register entry
	// 8, ic7100-record-length, whose other reading is the 104-byte text-only
	// one — so a radio built to accept 110 refuses this profile's 111-byte
	// set with FA while still ANSWERING the 111 bytes it was seeded with,
	// which is what lets the session open at all.
	radio, s := e2eOpen(t, Simulated, nil,
		fakeic7100.WithSlot(1, 1, occupiedRecord(t)),
		fakeic7100.WithAcceptedRecordLength(110),
	)
	ch := writableChannel(t, s)

	result, err := s.WriteChannel(context.Background(), ch)
	if err == nil {
		t.Fatal("the driver reported success for a set the radio refused")
	}
	if !errors.Is(err, transport.ErrRejected) || !strings.Contains(err.Error(), "FA") {
		t.Fatalf("err = %v, want an explicit FA refusal", err)
	}
	if len(result.Steps) != 1 || !result.Steps[0].Sent || result.Steps[0].Confirmed {
		t.Errorf("result = %+v, want one sent but unconfirmed step", result)
	}
	if got := countSets(radio.Transcript()); got != 1 {
		t.Errorf("the radio received %d sets, want exactly 1 — a refused set is never resent", got)
	}
	stored, _ := radio.Slot(1, 1)
	if !bytes.Equal(stored, occupiedRecord(t)) {
		t.Error("the refused set changed what the radio holds")
	}
}

func TestE2E_AWriteTimeoutIsQuarantinedAndTheSetIsNeverRetransmitted(t *testing.T) {
	// A LOST ACKNOWLEDGEMENT, PUT THROUGH THE FAKE'S OWN SERIAL PORT. The
	// radio HEARS one set frame, STORES it exactly as it always does, and
	// then says nothing at all — no FB, and no FA either, because nothing
	// was refused (fakeic7100.WithNoSetAnswer, and internal/fakeic7100/
	// doc.go is explicit that this is the package's one TEST LEVER rather
	// than a reading of an open page: no capture from a radio could settle
	// a lost acknowledgement, so it carries no register entry).
	//
	// That is the only far-end condition that reaches the tier's write
	// rule (core/transport, "Command classes are stated, not inferred"):
	// a memory set is an ACKNOWLEDGED write, transmitted EXACTLY ONCE,
	// never retransmitted when the acknowledgement fails to arrive, with
	// the outcome reported as UNATTRIBUTABLE and a quarantine afterwards
	// whatever happened. A radio that always answers cannot take a driver
	// down that branch at all.
	//
	// Nothing here is timed against a deadline that might expire while the
	// fake is mid-answer: the context is Background, the wait that ends is
	// the ENGINE's own answer timeout, and the fake sends not one byte in
	// reply to the set, so there is no race between the two goroutines to
	// lose. TestWriteChannelTimeoutNeverRetransmits (write_test.go:88) pins
	// the same shape through the scripted port; this is the second witness
	// reaching it from the far end.
	radio, s := e2eOpen(t, Simulated, nil,
		fakeic7100.WithSlot(1, 1, occupiedRecord(t)),
		fakeic7100.WithNoSetAnswer(),
	)
	ch := writableChannel(t, s)
	beforeSets := countSets(radio.Transcript())

	result, err := s.WriteChannel(context.Background(), ch)
	if err == nil {
		t.Fatal("the driver reported success for a set the radio never acknowledged")
	}
	// A TIMEOUT, NOT A REFUSAL, and the distinction is the whole point:
	// FA is the radio saying no, and silence is the radio saying nothing.
	// The driver may not collapse the second onto the first.
	if !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("err = %v, want transport.ErrTimeout — the acknowledgement never came", err)
	}
	if errors.Is(err, transport.ErrRejected) {
		t.Fatalf("err = %v reads as an FA refusal; the radio refused nothing, it answered nothing", err)
	}
	if !strings.Contains(err.Error(), "unattributable") || !strings.Contains(err.Error(), "not retransmitted") {
		t.Errorf("err = %v, want it to say the fate is unattributable and the set was not retransmitted", err)
	}
	// BOTH FLAGS FALSE. One step, because one set frame was built and put
	// on the wire; Sent false because "sent" here means an outcome the
	// radio attributed, and Confirmed false because no FB arrived. This is
	// the shape write_test.go:88 pins through the scripted port.
	if len(result.Steps) != 1 || result.Steps[0].Sent || result.Steps[0].Confirmed {
		t.Errorf("result = %+v, want exactly one step with Sent and Confirmed both false — the write's fate is unattributable", result)
	}
	if got := countSets(radio.Transcript()) - beforeSets; got != 1 {
		t.Errorf("the radio received %d sets, want exactly 1 — an unacknowledged set is NEVER resent, because resending an accepted one writes the channel twice", got)
	}

	// THE RADIO KEPT IT. Asked of the radio rather than of the answer it
	// never gave, which is how a test tells a lost ACKNOWLEDGEMENT from a
	// write that never landed: the driver could not know this, and rightly
	// reported the fate as unattributable, but the record is there.
	stored, occupied := radio.Slot(1, 1)
	if !occupied {
		t.Fatal("the radio holds nothing at A-001; the set was lost, not merely unacknowledged")
	}
	if len(stored) != civic7100.RecordLength {
		t.Fatalf("stored record is %d bytes, want %d", len(stored), civic7100.RecordLength)
	}

	// THE QUARANTINE DRAINS AND THE SESSION SURVIVES. A write whose
	// outcome is unknown leaves the stream suspect, so the next exchange
	// drains it to quiet before trusting anything as its own answer. The
	// property that matters to a caller is that this ENDS: the drain
	// reaches quiet against a radio that has gone silent, and the very
	// next read on the SAME session is answered normally rather than
	// coming back ErrQuarantineFailed with the engine wedged shut.
	//
	// WHAT THIS DOES AND DOES NOT PIN, stated rather than implied. From
	// the far end it pins that the drain SUCCEEDS: a quarantine that
	// cannot reach quiet fails this read outright. It cannot pin that the
	// drain was ATTEMPTED, because a radio that has genuinely gone silent
	// leaves nothing for a drain to find, so skipping one would look
	// identical here. That half is transport's own to pin, against a
	// stream that keeps talking — see TestFlood_NextDoRefusesToTransmit
	// AfterAFailedQuarantine in core/transport/flood_test.go.
	back, err := s.ReadChannel(context.Background(), "A-001")
	if err != nil {
		t.Fatalf("ReadChannel after the timed-out write: %v — the post-write quarantine did not drain and the session is wedged", err)
	}
	if back.Data == nil {
		t.Fatal("A-001 read back empty after a stored set")
	}
	if back.Data.Tag != "WRITE TEST" || back.Data.FreqHz != ch.Data.FreqHz {
		t.Errorf("read back %q/%d, want %q/%d — the stored record is what the radio should serve", back.Data.Tag, back.Data.FreqHz, "WRITE TEST", ch.Data.FreqHz)
	}
	// And the quarantine put nothing of its own on the wire: still one set
	// across the whole session, and every frame the radio heard is one
	// this profile's own builders could have produced.
	if got := countSets(radio.Transcript()) - beforeSets; got != 1 {
		t.Errorf("after the quarantine and the following read the radio had received %d sets, want still exactly 1", got)
	}
	e2eAuditGrammars(t, radio, 1)
}

func TestE2E_AWrongRecordLengthIsRefusedAndCanBeAttributed(t *testing.T) {
	// 104 bytes is THE near miss, not an arbitrary wrong number: taking the
	// diagram bar's own (52)~(60) label at face value gives a 107-byte data
	// area and a 104-byte record, which is where a text-only reading of
	// PDF p.375 lands. Both packages record that reading and neither took
	// it, so a radio answering 104 is the sibling-shaped confusion the
	// fingerprint exists to catch.
	const nearMiss = 104

	radio := fakeic7100.New(
		fakeic7100.WithSlot(1, 1, make([]byte, nearMiss)),
		fakeic7100.WithAcceptedRecordLength(nearMiss),
	)
	defer radio.Close()
	sess, err := New(Simulated).Open(context.Background(), radio.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open accepted a radio whose records are 104 bytes")
	}
	// THE PROBE'S LENGTH REFUSAL IS AN IDENTITY VERDICT, NOT A PARSE ERROR,
	// and that is why civ.RecordLengthError is NOT in this chain:
	// Session.wrongRecordLength translates the length it measured into a
	// driver.WrongRadioError carrying both record-only lengths as its
	// Want and Got, which is what a caller deciding "this is not the
	// radio you asked for" can act on. TestProbeRejectsWrongRecordLength
	// Continuously (probe_test.go:35) pins the same shape through the
	// scripted port; asserting a wrapped *civ.RecordLengthError here would
	// have been this test inventing a contract the driver never had.
	var lengthErr *civ.RecordLengthError
	if errors.As(err, &lengthErr) {
		t.Errorf("err = %v carries a *civ.RecordLengthError; the probe's verdict is an identity refusal", err)
	}
	var wrong *driver.WrongRadioError
	if !errors.Is(err, driver.ErrWrongRadio) || !errors.As(err, &wrong) {
		t.Fatalf("err = %v, want a WrongRadioError", err)
	}
	if wrong.Want != "record 111" || wrong.Got != "record 104" {
		t.Errorf("WrongRadioError = %+v, want the two RECORD-ONLY lengths — the wire also carried three address bytes, and a data-area count would read 107 here", wrong)
	}
	// UNATTRIBUTED by default. Cross-model record-length distinctness is a
	// TIER-level Wave-4 check with its own declared table; this driver
	// refuses the length and names no model until one is injected.
	if wrong.WantModel != "" || wrong.GotModel != "" {
		t.Errorf("the driver attributed the radio to %q against %q with no sibling table", wrong.GotModel, wrong.WantModel)
	}
}

func TestE2E_AnInjectedSiblingLengthIsAProvisionalModelNameDiagnostic(t *testing.T) {
	// The same refusal, with a sibling table supplied by tier integration.
	// The attribution is a DIAGNOSTIC and says so: both compared lengths
	// are ASSUMED derivations from printed field widths, so a name here is
	// provisional until one of them is measured against a radio.
	const nearMiss = 104
	radio := fakeic7100.New(
		fakeic7100.WithSlot(1, 1, make([]byte, nearMiss)),
		fakeic7100.WithAcceptedRecordLength(nearMiss),
	)
	defer radio.Close()

	d := New(Simulated, WithSiblingRecordLengths(SiblingLengths{nearMiss: "Synthetic 104-byte sibling"}))
	if d.Model() != "IC-7100" || d.Capabilities().Model != "IC-7100" {
		t.Errorf("Model = %q / %q, want IC-7100 on both", d.Model(), d.Capabilities().Model)
	}
	sess, err := d.Open(context.Background(), radio.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open accepted a radio whose records are 104 bytes")
	}
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("err = %v, want *driver.WrongRadioError", err)
	}
	if wrong.WantModel != "IC-7100" || wrong.GotModel != "Synthetic 104-byte sibling" {
		t.Errorf("WrongRadioError = %+v, want both model names", wrong)
	}
	if !strings.Contains(strings.ToUpper(err.Error()), "PROVISIONAL") {
		t.Errorf("err = %v, want the attribution marked provisional", err)
	}
}

func TestE2E_BothEmptyChannelFormsReadAsEmpty(t *testing.T) {
	// TWO SEPARATE ASSUMPTIONS, two register entries on each side, and one
	// capture cannot retire both: the fake's entry 1
	// (ic7100-empty-channel-fa) answers a read of an unoccupied channel
	// with the NG code, and its entry 2 (ic7100-all-ff-record) answers with
	// a full-length record of FF bytes instead. The driver must read both
	// as EMPTY, and must fingerprint on neither.
	t.Run("FA", func(t *testing.T) {
		_, s := e2eOpen(t, Simulated, nil, fakeic7100.WithSlot(1, 1, occupiedRecord(t)))
		for _, slot := range []string{"A-002", "B-001", "E-099"} {
			ch, err := s.ReadChannel(context.Background(), slot)
			if err != nil {
				t.Errorf("ReadChannel(%s) = %v, want an empty channel and no error", slot, err)
			}
			if ch.Data != nil {
				t.Errorf("ReadChannel(%s) came back populated", slot)
			}
			if ch.Slot != slot {
				t.Errorf("Slot = %q, want %q — an empty channel still names its slot", ch.Slot, slot)
			}
		}
	})

	t.Run("all-FF", func(t *testing.T) {
		// Every unoccupied channel now answers 111 FF bytes, so the probe
		// must walk past A-001 and A-002 and fingerprint on the real record
		// at A-003 rather than measuring an emptiness marker into evidence.
		_, s := e2eOpen(t, Simulated, nil,
			fakeic7100.WithAllFFEmptyRecord(),
			fakeic7100.WithSlot(1, 3, occupiedRecord(t)),
		)
		d := s.CIVDiagnostics()
		if !d.Fingerprinted || d.ProbeSlotsRead != 3 {
			t.Errorf("diagnostics = %+v; the all-FF answers at A-001 and A-002 are empties, and A-003 supplies the fingerprint", d)
		}
		for _, slot := range []string{"A-001", "A-002", "E-099"} {
			ch, err := s.ReadChannel(context.Background(), slot)
			if err != nil {
				t.Errorf("ReadChannel(%s) = %v, want an empty channel and no error", slot, err)
			}
			if ch.Data != nil {
				t.Errorf("ReadChannel(%s) decoded an all-FF record into a channel", slot)
			}
		}
	})

	t.Run("an all-FF radio opens unfingerprinted", func(t *testing.T) {
		_, s := e2eOpen(t, Simulated, nil, fakeic7100.WithAllFFEmptyRecord())
		if d := s.CIVDiagnostics(); d.Fingerprinted || d.Status != "UNFINGERPRINTED" {
			t.Errorf("diagnostics = %+v, want an explicitly unfingerprinted open", d)
		}
	})
}

func TestE2E_EchoChangesNothingAndIsSuppressedExactly(t *testing.T) {
	// WHETHER A REAL IC-7100 ECHOES IS UNKNOWN — the manual has no
	// echo-back setting anywhere, but [REMOTE] is a shared bus on which
	// echo would be a property of the wiring rather than a setting (the
	// fake's register entry 3, ic7100-echo-default). So the driver has to
	// survive both readings, and this is the one it was not built against.
	//
	// The echo is EXACT: the fake echoes the frame as it normalised it on
	// receipt, exactly two preamble bytes, and civ's accumulator drops a
	// frame that byte-equals one it noted sending. So every echo must land
	// in Echoes and none in Unexpected — an echo counted as unexpected
	// traffic would mean the two sides disagree about the canonical form of
	// a frame, which is a finding rather than a nuisance.
	radio, s := e2eOpen(t, Simulated, nil,
		fakeic7100.WithEcho(),
		fakeic7100.WithSlot(1, 1, occupiedRecord(t)),
	)

	if d := s.CIVDiagnostics(); !d.Fingerprinted {
		t.Errorf("the probe did not fingerprint through an echoing radio: %+v", d)
	}
	ch, err := s.ReadChannel(context.Background(), "A-001")
	if err != nil || ch.Data == nil {
		t.Fatalf("ReadChannel through an echoing radio = %+v, %v", ch, err)
	}
	if ch.Data.Tag != "HOME BASE" {
		t.Errorf("Tag = %q, want the seeded HOME BASE", ch.Data.Tag)
	}
	ch.Data.Tag = "ECHO TEST"
	if _, err := s.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel through an echoing radio: %v", err)
	}

	d := s.CIVDiagnostics()
	if d.Accumulator.Echoes == 0 {
		t.Error("Echoes is zero against a radio echoing every frame — byte-identity suppression is what makes echo a non-event, and it must be visible in the counters")
	}
	if d.Accumulator.Unexpected != 0 {
		t.Errorf("Unexpected = %d with echo on; an echo counted as foreign traffic means the two sides disagree about a frame's canonical form", d.Accumulator.Unexpected)
	}
	if got := countSets(radio.Transcript()); got != 1 {
		t.Errorf("the radio received %d sets through an echoing line, want exactly 1", got)
	}
	stored, occupied := radio.Slot(1, 1)
	if !occupied || !bytes.Contains(stored, []byte("ECHO TEST")) {
		t.Error("the write did not land through an echoing line")
	}
	e2eAuditGrammars(t, radio, 1)
}

func TestE2E_ATransceiveBroadcastFloodNeverReachesTheEngine(t *testing.T) {
	// The to=00 species (the fake's register entry 5,
	// ic7100-broadcast-address-form). civ's accumulator counts such a frame
	// and NEVER returns it, so it never becomes an engine event, the
	// drain's idle timer is never re-armed, Init succeeds, and NO drain-cap
	// diagnostic exists. The frames are visible only as Unexpected — which
	// is exactly why the driver reports the framing's counter rather than
	// the engine's.
	radio, s := e2eOpen(t, Simulated, nil,
		fakeic7100.WithSlot(1, 1, occupiedRecord(t)),
		fakeic7100.WithTransceiveBroadcasts(2*time.Millisecond),
	)

	d := s.CIVDiagnostics()
	if d.InitDrainCapExceeded {
		t.Error("a to=00 flood produced a drain-cap diagnostic; those frames never reach the engine")
	}
	if !d.Fingerprinted {
		t.Error("the probe did not fingerprint through a broadcast flood")
	}
	if d.Accumulator.Unexpected == 0 {
		t.Error("the broadcasts were not counted; the framing's counter is the only place they are visible")
	}

	// And the session still works while the line jabbers.
	ch, err := s.ReadChannel(context.Background(), "A-001")
	if err != nil || ch.Data == nil {
		t.Fatalf("ReadChannel under a broadcast flood = %+v, %v", ch, err)
	}
	if after := s.CIVDiagnostics().Accumulator.Unexpected; after <= d.Accumulator.Unexpected {
		t.Errorf("Unexpected went %d -> %d, want it rising while the flood runs", d.Accumulator.Unexpected, after)
	}
	e2eAuditGrammars(t, radio, 0)
}

func TestE2E_AControllerAddressedFloodIsNonfatalAndDiagnosed(t *testing.T) {
	// The to=E0 species, a SEPARATE assumption with the same standing, and
	// the only one that can reach the drain's cap: these frames pass the
	// address filter, become engine events, and re-arm the idle timer. That
	// INITIAL failure is nonfatal-with-diagnostic — Init's drain is bounded
	// precisely so that a jabbering line cannot fail the open — and
	// "nonfatal" does not mean unrecorded.
	_, s := e2eOpen(t, Simulated, nil,
		fakeic7100.WithSlot(1, 1, occupiedRecord(t)),
		fakeic7100.WithAddressedFlood(2*time.Millisecond),
	)
	if !s.CIVDiagnostics().InitDrainCapExceeded {
		t.Error("the drain-cap event was swallowed; nonfatal does not mean unrecorded")
	}
}

func TestE2E_EveryEraseFormIsRefused(t *testing.T) {
	// ERASE IS UNSHIPPED, and four independent things say so. The wire form
	// EXISTS on this radio — PDF p.375's "About clearing operation" block
	// prints it, in two readings the fake knows well enough to refuse both
	// of (its register entry 12) — which is exactly why the absence has to
	// be structural rather than incidental.
	radio, s := e2eOpen(t, Simulated, []Option{WithConsentedUnverifiedWrites()},
		fakeic7100.WithSlot(1, 1, occupiedRecord(t)))

	// One: no consent opens the gate, on either profile.
	for name, caps := range map[string]spec.Capabilities{
		"unverified": CapabilitiesUnverified(),
		"simulated":  CapabilitiesSimulated(),
	} {
		consented := spec.ConsentUnverifiedWrites(caps)
		for _, bank := range consented.Banks {
			if consented.FieldSupport(bank.ID, spec.FieldErase).CanWrite() {
				t.Errorf("%s/%s: consent opened the erase gate", name, bank.ID)
			}
		}
	}

	// Two: an empty channel is an erase, is named as one, and is refused
	// before any wire traffic.
	before := len(radio.Transcript())
	result, err := s.WriteChannel(context.Background(), codeplug.Channel{Slot: "A-001"})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want ErrWriteRefused for an empty channel", err)
	}
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) || len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldErase {
		t.Errorf("refusal = %+v, want it to name FieldErase", refused)
	}
	if len(result.Steps) != 0 || len(radio.Transcript()) != before {
		t.Errorf("the erase refusal produced %+v and %d frames", result, len(radio.Transcript())-before)
	}

	// Three: the outbound gate admits neither printed reading of the clear
	// form, so nothing this tier could build would carry one.
	for name, frame := range map[string][]byte{
		"with the bank byte":    {0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x01, 0xFF, 0xFD},
		"without the bank byte": {0xFE, 0xFE, 0x88, 0xE0, 0x1A, 0x00, 0x00, 0x01, 0xFF, 0xFD},
	} {
		if civic7100.Profile().AllowedCommand(frame) {
			t.Errorf("the outbound gate admitted the clear form %s: % X", name, frame)
		}
	}

	// Four: the radio never heard one. Its transcript is the second
	// witness, and it records the frames it REFUSES too — so a clear this
	// driver built and the fake rejected would still show up here.
	for _, f := range radio.Transcript() {
		cn, sc, ok := civ.FrameCommand(f)
		if !ok || cn != 0x1A || sc != 0x00 {
			continue
		}
		payload := f[6 : len(f)-1]
		if len(payload) <= civic7100.AddressBytes+1 && payload[len(payload)-1] == 0xFF {
			t.Errorf("a clear frame reached the radio: % X", f)
		}
	}
}

func TestE2E_TheWholeSessionSendsOnlyTheGrammarsItCanBuild(t *testing.T) {
	// THE TIER-WIDE CLAIM, audited over a WHOLE SESSION rather than over
	// its opening: probe, walk, write, re-read. The narrower Open-only
	// version is in the fingerprint test, and the erase test cannot close
	// the gap either, because both of its refusals happen before the wire,
	// so its transcript is an Open transcript too.
	//
	// This session actually writes. Exactly one memory set is expected, at
	// exactly 121 bytes, and every other frame must be one of the two read
	// grammars at their exact widths and byte-identical to what this
	// profile's own builders emit.
	radio, s := e2eOpen(t, Simulated, nil, fakeic7100.WithSlot(1, 1, occupiedRecord(t)))

	for _, slot := range []string{"A-001", "A-002", "C-050", "E-099"} {
		if _, err := s.ReadChannel(context.Background(), slot); err != nil {
			t.Fatalf("ReadChannel(%s): %v", slot, err)
		}
	}
	if _, err := s.WriteChannel(context.Background(), writableChannel(t, s)); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	back, err := s.ReadChannel(context.Background(), "A-001")
	if err != nil || back.Data == nil {
		t.Fatalf("ReadChannel after write = %+v, %v", back, err)
	}
	if back.Data.Tag != "WRITE TEST" {
		t.Errorf("Tag = %q after the write, want WRITE TEST", back.Data.Tag)
	}
	e2eAuditGrammars(t, radio, 1)
}

// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// Identity names one physical radio attachment: which port it is on, and
// what the radio said it is. A Driver.Open caller fills in Port (and
// USBSerial, when discovery knew it); the driver itself fills in CATID
// from its ID; probe, overwriting whatever the caller supplied — the
// probe's answer is authoritative.
type Identity struct {
	// CATID is a CAT ID (four hex digits on Yaesu; the CI-V address,
	// optionally with a recorded token, on Icom), e.g. "0800" for an
	// FT-710 from its ID; probe, or "98:<token>" for an IC-7610 from its
	// 19 00 probe.
	//
	// The tier's CATID casing is NOT uniform, and each model's own
	// convention is fixed rather than chosen per session: the IC-7610,
	// IC-7300 and IC-7300MK2 render their CI-V address in lower case
	// ("98", "94:<token>", "b6:<token>"), while the IC-705, IC-9700 and
	// IC-905 render theirs in upper case ("A4:<token>", "A2" or
	// "A2/<TOKEN>", "AC:<TOKEN>"). Comparisons against CATID are exact
	// case per model — internal/wiring's per-model identity test
	// (TestOpenFakeSessionFor_EveryRegisteredModel) does a plain string
	// equality against each model's own declared Capabilities().CATID,
	// so a driver that rendered its own address in the wrong case would
	// fail that check against itself, never against another model.
	CATID string
	// USBSerial is the USB device serial number from port discovery
	// (transport.PortInfo.USBSerial); "" when the port is not USB or the
	// OS did not report one.
	USBSerial string
	// Port is the OS device path the session is (or will be) open on,
	// e.g. "/dev/cu.SLAB_USBtoUART".
	Port string
}

// WriteStep reports how far ONE frame of a Session.WriteChannel call got.
//
// Command is that frame's mnemonic as the radio's own protocol names it
// (e.g. "MW", "MT") — a LABEL for the audit trail and for a human reading
// a journal, never a token a generic layer is expected to branch on. No
// code above the driver seam may switch on its value: two radios may
// choose different mnemonics, different numbers of steps, or one combined
// frame where another needs two, and every such shape is equally valid
// here.
//
// The CAT protocol's write commands are fire-and-forget — a successful Set
// produces NO acknowledgement, only a bounded listen for a possible "?;"
// rejection (transport.CommandSpec.ErrorWindow) — so Confirmed means
// exactly "no rejection arrived in the window", never "the radio
// positively acknowledged it". Positive verification (reading the slot
// back and comparing) is deliberately NOT the driver's job: it belongs to
// the clone service, which owns the whole write-then-verify workflow.
//
// A false Sent flag means the frame's outcome is NOT known-clean: it may
// never have been transmitted at all (an earlier step failed first, so
// this one was never reached), or a transport-level failure left its
// outcome unknowable. The accompanying error distinguishes the cases;
// either way the caller must treat the slot's on-radio state as
// unverified.
type WriteStep struct {
	// Command is the frame's mnemonic, e.g. "MW", "MT".
	Command string
	// Sent reports that the frame was transmitted with an attributable
	// outcome — success or an explicit rejection.
	Sent bool
	// Confirmed reports that the frame's error window elapsed with no
	// "?;" rejection (fire-and-forget accepted).
	Confirmed bool
}

// WriteResult reports, step by step, how far a Session.WriteChannel call
// got: one WriteStep per frame the driver's write choreography intends,
// in the order the driver intends to send them.
//
// The steps are the WHOLE intended sequence, declared once the frames have
// been built and before the first goes out — not a log of what happened to
// be attempted. A step that is present with Sent false therefore reads as
// "this frame was part of the plan and never attributably went out", which
// is precisely what an aborted sequence should report. A refusal that
// happens BEFORE the frames are built has no sequence to describe and
// reports an empty (non-nil) Steps slice.
//
// Why steps rather than named flags: this type sits on the NEUTRAL driver
// seam, above which nothing may know a wire protocol. Its predecessor
// (removed by M9c-5's E6) carried four booleans named for the FT-710's
// own two frames — the sent/confirmed pair of each, spelt out — which was
// that one radio's choreography written into the seam every future radio
// plugs into. A radio that writes a channel and its label in ONE combined
// frame has no honest answer to such flags: reported as "the tag frame
// went, the data frame did not", its perfectly ordinary write would read,
// in FT-710 terms, as a tag-only write that never touched the channel
// data. Steps let each driver report what it actually did, and let the
// neutral layers record it without pretending to understand it.
// internal/guards' TestRetiredWriteResultNamesAreGone holds the retired
// names down.
type WriteResult struct {
	// Steps are the write's frames in send order. Empty (never nil) when
	// the write was refused before any frame was built.
	Steps []WriteStep
}

// Driver is one radio model's protocol implementation: the seam every
// future radio plugs into. Generic layers (UI, clone service, CLI) hold
// Drivers and Sessions only — they never import a driver's protocol
// package, and never learn a wire protocol's field names or quirks.
type Driver interface {
	// Model is the radio's display name, e.g. "FT-710" — the Registry
	// key. It must equal Capabilities().Model.
	Model() string

	// Capabilities returns the STATIC baseline capability description for
	// this driver's profile: what the radio model can do before any
	// radio has been probed. It contains no discovered banks (e.g. no
	// 60 m/EMG slot inventory — those are region-dependent and only
	// knowable per-session; see Session.Capabilities). The returned value
	// must pass spec.Capabilities.Validate; Registry.Register enforces
	// this.
	Capabilities() spec.Capabilities

	// Open establishes a session with the radio on port. It probes the
	// radio's identity (the ID; command) and fails with an error
	// satisfying errors.Is(err, ErrWrongRadio) — carrying what was
	// actually found, see WrongRadioError — if the answer is not this
	// driver's radio. On success the returned Session's Identity()
	// carries the probed CATID alongside the caller-supplied Port and
	// USBSerial from id.
	//
	// Open takes ownership of port on BOTH outcomes: the Session's Close
	// releases it on success, and Open itself closes it before returning
	// an error — the caller never closes port directly once Open has
	// been called.
	Open(ctx context.Context, port transport.Port, id Identity) (Session, error)
}

// Session is one open, probed connection to a radio. Implementations must
// be safe for concurrent use (the underlying transport engine serialises
// exchanges); Close must be idempotent.
type Session interface {
	// Identity returns who this session is talking to: the probed CATID
	// plus the caller-supplied port path and USB serial from Open.
	Identity() Identity

	// Capabilities returns this session's EFFECTIVE capabilities: the
	// driver's static baseline plus whatever Open discovered about this
	// specific radio (e.g. its regional 60 m/EMG channel inventory, as
	// read-only banks). The returned value passes
	// spec.Capabilities.Validate, and is a defensive copy: mutating it
	// can never alter what the session itself enforces (its write gate
	// re-checks against internal state, not against copies it handed
	// out).
	Capabilities() spec.Capabilities

	// ReadChannel reads one memory slot (canonical wire form, e.g.
	// "001", "P1L", "501", "EMG") and returns it as a codeplug.Channel.
	// A "?;" rejection of the read is mapped to an EMPTY channel
	// (Data == nil) — the protocol's assumed empty-slot answer — not an
	// error. Every other failure (malformed slot, transport failure,
	// malformed or inconsistent answer) is a typed error. Fields the
	// protocol cannot read (e.g. the FT-710's CTCSS tone and scan skip)
	// come back with FieldState Unknown, never a guessed value.
	ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error)

	// WriteChannel writes one populated channel to its slot. It is only
	// legal for a channel whose EVERY requested field is writable per
	// this session's Capabilities (FieldSupport.CanWrite); the driver
	// re-checks this itself — defence in depth below the clone service —
	// and refuses, with an error satisfying errors.Is(err,
	// ErrWriteRefused), BEFORE any wire traffic. A field carrying
	// FieldState Known that the protocol cannot express (e.g. CTCSS
	// tone, scan skip) is likewise refused, never silently dropped. The
	// returned WriteResult reports which of the write's steps were sent
	// and unrejected; WriteChannel performs NO read-back verification —
	// that is the clone service's job.
	//
	// THE ONE RECORDED EXCEPTION TO "BEFORE ANY WIRE TRAFFIC" (Icom
	// tier, ruling T5). Refusals derived from CAPABILITIES and from the
	// CHANNEL'S OWN CONTENT — field validity, vocabularies, cross-field
	// constraints, mandatory-Known rules — are locally decidable, and
	// every one of them precedes ALL wire traffic, as above. A refusal
	// that requires the SLOT'S CURRENT STATE cannot be: the Icom tier's
	// E6 rule refuses a write whose unmapped record regions differ from
	// the profile's Fixed template, and a driver cannot know those
	// regions without reading the slot. Such a refusal therefore
	// NECESSARILY FOLLOWS the single read that obtains that state, and
	// is the one recorded exception. A driver orders its ladder
	// accordingly: every locally decidable refusal first, then ONE read,
	// then the read-dependent refusals, then the write.
	WriteChannel(ctx context.Context, ch codeplug.Channel) (WriteResult, error)

	// Close releases the session and its underlying port. Idempotent.
	Close() error
}

// ErrWrongRadio is the sentinel a caller should compare against (via
// errors.Is) when Driver.Open's ID; probe answered with a different
// radio's CAT ID. The error actually returned is a *WrongRadioError
// carrying both the expected and the found ID.
var ErrWrongRadio = errors.New("driver: connected radio did not identify as the expected model")

// WrongRadioError reports that an ID; probe answered with a CAT ID other
// than the driver's own — the port is attached to some other radio (or
// something else entirely that happens to speak CAT).
type WrongRadioError struct {
	// Want is the CAT ID the driver expected, e.g. "0800".
	Want string
	// Got is the CAT ID the probe actually returned, e.g. "0761" (an
	// FT-DX10).
	Got string

	// WantModel and GotModel are OPTIONAL display names, additive as of
	// M9d-2 (spec A5): a driver that can NAME the models its IDs belong
	// to may populate both, and Error() then spells the refusal out in
	// model names as well as IDs. Drivers that cannot (the FT-710's and
	// FTdx10's never do) leave them empty and Error()'s ID-only text is
	// byte-identical to what it was before these fields existed — that
	// text is pinned by test, because rendered refusals are recorded in
	// baselines. Is/As semantics are unchanged either way.
	WantModel string
	GotModel  string
}

// Error implements the error interface.
func (e *WrongRadioError) Error() string {
	if e.WantModel != "" && e.GotModel != "" {
		return fmt.Sprintf("driver: connected radio identifies as %s (CAT ID %q); you selected %s (CAT ID %q) — wrong radio model on this port", e.GotModel, e.Got, e.WantModel, e.Want)
	}
	return fmt.Sprintf("driver: connected radio identified as CAT ID %q, want %q — wrong radio model on this port", e.Got, e.Want)
}

// Unwrap lets errors.Is(err, ErrWrongRadio) match.
func (e *WrongRadioError) Unwrap() error { return ErrWrongRadio }

// ErrWriteRefused is the sentinel a caller should compare against (via
// errors.Is) when Session.WriteChannel refused a channel BEFORE any wire
// traffic — the channel requests something this session's capabilities do
// not permit writing. The error actually returned is a
// *WriteRefusedError naming the slot, the offending fields (when the
// refusal is per-field), and the reason.
var ErrWriteRefused = errors.New("driver: write refused: channel requests something this session cannot write")

// WriteRefusedError reports why Session.WriteChannel refused a channel
// without sending anything.
type WriteRefusedError struct {
	// Slot is the canonical wire-form slot the refused channel targeted.
	Slot string
	// Fields lists the spec.Fields that caused the refusal, when the
	// refusal is per-field (e.g. unwritable fields, or a Known value the
	// protocol cannot express). Empty when Reason alone describes it.
	Fields []spec.Field
	// Reason is a human-readable explanation.
	Reason string
}

// Error implements the error interface.
func (e *WriteRefusedError) Error() string {
	if len(e.Fields) == 0 {
		return fmt.Sprintf("driver: write to slot %q refused: %s", e.Slot, e.Reason)
	}
	names := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		names[i] = string(f)
	}
	return fmt.Sprintf("driver: write to slot %q refused (%s): %s", e.Slot, strings.Join(names, ", "), e.Reason)
}

// Unwrap lets errors.Is(err, ErrWriteRefused) match.
func (e *WriteRefusedError) Unwrap() error { return ErrWriteRefused }

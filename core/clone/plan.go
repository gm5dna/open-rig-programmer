// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// SendPlan is the immutable result of PrepareSend: a fresh baseline read,
// diffed against a candidate file, bound to the exact digests, session
// identity, and Service generation Execute re-checks before writing
// anything (obligations 2, 3, 4).
//
// SendPlan is deliberately opaque outside this package: every field is
// unexported, and the accessors below hand out defensive copies. Nothing
// external can mutate what Execute will actually send — editing the file a
// caller passed to PrepareSend, after the fact (even editing the very
// *codeplug.Codeplug value passed in), can never change what a plan built
// from it writes. See PrepareSend's doc comment.
type SendPlan struct {
	preparedAt time.Time
	identity   driver.Identity
	generation int64

	// baseline and candidate are PRIVATE deep copies (obligation 2): the
	// exact channel values PrepareSend read/received, independent of
	// anything the caller still holds a reference to.
	baseline  []codeplug.Channel
	candidate []codeplug.Channel
	// candidateRadio is a copy of file.Radio at PrepareSend time, kept
	// only so Execute's re-Validate (obligation 3) has a RadioInfo to
	// check the candidate's claimed radio identity against — RadioInfo
	// holds no pointers, so this is already a full copy.
	candidateRadio codeplug.RadioInfo

	baselineDigest  string
	candidateDigest string

	diff   codeplug.DiffResult
	issues []codeplug.Issue

	snapshotPath string
}

// Diff returns a defensive copy of this plan's DiffResult, computed at
// PrepareSend time against the fresh baseline — safe for a caller to
// display and mutate freely without affecting the plan or any other call's
// result.
func (p *SendPlan) Diff() codeplug.DiffResult {
	return copyDiffResult(p.diff)
}

// Issues returns a defensive copy of the Issues codeplug.Validate found for
// the candidate at PrepareSend time.
func (p *SendPlan) Issues() []codeplug.Issue {
	return append([]codeplug.Issue(nil), p.issues...)
}

// BaselineDigest returns the digest of the fresh baseline PrepareSend read
// — the value to DISPLAY alongside Diff() for a user's review. It is
// deliberately NOT the value Execute's confirmedDigest parameter checks
// (see ConfirmationDigest): two plans PrepareSend built from the very same
// baseline read (e.g. two candidate files reviewed back to back against
// one radio state) share BaselineDigest, so it cannot by itself distinguish
// which PLAN a confirmation was actually given for.
func (p *SendPlan) BaselineDigest() string { return p.baselineDigest }

// confirmationDigestDomain is hashed as the first field of every
// ConfirmationDigest, so this SHA-256 use can never collide with a digest
// computed for an unrelated purpose elsewhere in the project (e.g.
// codeplug.Digest's own channel-content hash) even were its inputs to
// coincide by chance.
const confirmationDigestDomain = "open-rig-programmer/core/clone/confirmation-digest-v1"

// writeLengthPrefixed writes a length-prefixed s to h: len(s) as decimal
// digits, ':', then s itself. Framing every field this way (rather than
// concatenating raw strings) means ConfirmationDigest's inputs can never be
// reinterpreted across a field boundary — e.g. baselineDigest="ab" +
// candidateDigest="cd" hashing identically to baselineDigest="a" +
// candidateDigest="bcd" — the way naive concatenation of variable-length
// fields would allow.
func writeLengthPrefixed(h interface{ Write([]byte) (int, error) }, s string) {
	fmt.Fprintf(h, "%d:", len(s))
	h.Write([]byte(s))
}

// ConfirmationDigest returns the exact value Execute's confirmedDigest
// parameter must equal for THIS plan (obligation 5 — see doc.go and
// ErrConfirmationMismatch): a hex-encoded SHA-256 over
// confirmationDigestDomain, this plan's baseline digest, its candidate
// digest, its bound session identity (CATID, USBSerial, Port), and its
// Service generation — every field length-prefixed (writeLengthPrefixed)
// so none can bleed into its neighbour.
//
// This exists, distinct from BaselineDigest (Fix 1, adjudicated HIGH), because
// two plans PrepareSend built from the SAME baseline read but different
// candidate files previously shared BaselineDigest — the value Execute's
// confirmedDigest was checked against — so a caller who confirmed plan A's
// diff could hand that identical digest to Execute(planB, ...) and have it
// silently accepted, executing a diff the user never actually reviewed.
// Folding in the candidate digest makes the confirmation value
// plan-specific rather than merely baseline-specific; folding in identity
// and generation too (duplicating obligation 4's own inputs) means a
// confirmation captured for one session can never be replayed, even
// coincidentally, against a plan bound to a different one.
func (p *SendPlan) ConfirmationDigest() string {
	h := sha256.New()
	writeLengthPrefixed(h, confirmationDigestDomain)
	writeLengthPrefixed(h, p.baselineDigest)
	writeLengthPrefixed(h, p.candidateDigest)
	writeLengthPrefixed(h, p.identity.CATID)
	writeLengthPrefixed(h, p.identity.USBSerial)
	writeLengthPrefixed(h, p.identity.Port)
	writeLengthPrefixed(h, fmt.Sprintf("%d", p.generation))
	return hex.EncodeToString(h.Sum(nil))
}

// SnapshotPath returns the path PrepareSend saved the fresh baseline
// snapshot to (obligation 9).
func (p *SendPlan) SnapshotPath() string { return p.snapshotPath }

// copyChannelData returns a defensive copy of d, or nil if d is nil.
// ChannelData holds only value types (no slices/maps/pointers), so one
// dereference-and-copy is already a full deep copy. Duplicated from
// codeplug's own unexported helper of the same shape: that one is not
// exported, and this package's opacity guarantee (obligation 2) is cheap
// enough to restate directly rather than pull in reflection or an export
// just for this.
func copyChannelData(d *codeplug.ChannelData) *codeplug.ChannelData {
	if d == nil {
		return nil
	}
	cp := *d
	return &cp
}

// copyChannels returns an independently-allocated deep copy of channels:
// a fresh slice, and a fresh *ChannelData (via copyChannelData) per
// element.
func copyChannels(channels []codeplug.Channel) []codeplug.Channel {
	out := make([]codeplug.Channel, len(channels))
	for i, ch := range channels {
		out[i] = codeplug.Channel{Slot: ch.Slot, Data: copyChannelData(ch.Data)}
	}
	return out
}

// copyDiffResult returns a defensive deep copy of d: a fresh Entries slice
// with independently-allocated Before/After copies, so mutating the
// result can never reach back into a SendPlan's own private diff.
func copyDiffResult(d codeplug.DiffResult) codeplug.DiffResult {
	out := d
	out.Entries = make([]codeplug.DiffEntry, len(d.Entries))
	for i, e := range d.Entries {
		e.Before = copyChannelData(e.Before)
		e.After = copyChannelData(e.After)
		out.Entries[i] = e
	}
	return out
}

// capsConsented reports whether any bank field in caps carries the
// spec.ConsentedUnverified WRITE label — that is, whether this session's
// write gate was opened anywhere by a user's recorded consent to an
// unverified write, rather than by hardware evidence alone. (Read labels
// cannot be ConsentedUnverified at all: spec.Capabilities.Validate
// rejects one, and spec.ConsentUnverifiedWrites never mints one, so the
// write-side check below is the whole question.)
//
// It exists for one caller: the "consented_unverified" field of
// PrepareSend's prepare journal line. That field is this plan's DISCLOSED
// DEVIATION from the consent design spec (docs/superpowers/plans/
// 2026-08-14-unverified-write-consent.md, "Spec deviations", item 1): the
// spec assumed the journal already held a capability snapshot a reader
// could work the answer out of afterwards, and it does not — the prepare
// event carries paths, digests and diff counts only. Rather than start
// journaling a whole capability set, the one fact the audit trail needs
// about consent is computed here and recorded as a single boolean.
func capsConsented(caps spec.Capabilities) bool {
	for _, b := range caps.Banks {
		for _, fs := range b.Fields {
			if fs.Write == spec.ConsentedUnverified {
				return true
			}
		}
	}
	return false
}

// PrepareSend builds an immutable SendPlan for sending file to the radio.
//
// Order (fixed, and load-bearing — see doc.go's obligations 1 and 9): a
// fresh ReadAll (obligation 1 — never a cached/prior read); saving that
// read as a snapshot via the Service's SnapshotStore (obligation 9 —
// before Validate/Diff run at all, so even a validation failure still
// leaves a durable record of what the radio held at this moment);
// codeplug.Validate(file, caps) against the session's CURRENT effective
// capabilities (a SeverityError issue is a typed refusal,
// *ValidationFailedError, carrying every Issue found); codeplug.Diff
// against the fresh baseline (a slot-inventory mismatch is likewise
// reported as a *ValidationFailedError, since it means file does not
// descend from a read of this radio's current layout).
//
// The returned plan's baseline and candidate channels are PRIVATE deep
// copies, made before this method returns (obligation 2): mutating file
// afterwards — even the very *codeplug.Codeplug value passed in — can
// never change what Execute later sends.
func (s *Service) PrepareSend(ctx context.Context, file *codeplug.Codeplug) (*SendPlan, error) {
	// Fix 2 (adjudicated MEDIUM): acquire this Service's operation lock for
	// PrepareSend's WHOLE body — including its own nested fresh read below,
	// which therefore calls the unlocked s.readAll, not s.ReadAll (which
	// would otherwise self-refuse with ErrBusy against this very call). See
	// Service.acquireOp's doc comment.
	if err := s.acquireOp("PrepareSend"); err != nil {
		return nil, err
	}
	defer s.releaseOp()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("clone: PrepareSend: %w", err)
	}

	baseline, err := s.readAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("clone: PrepareSend: %w", err)
	}

	snapshotPath, err := s.store.SaveSnapshot(baseline, s.now())
	if err != nil {
		return nil, fmt.Errorf("clone: PrepareSend: %w", err)
	}

	caps := s.sess.Capabilities()

	issues := codeplug.Validate(file, caps)
	if codeplug.HasErrors(issues) {
		return nil, &ValidationFailedError{Issues: append([]codeplug.Issue(nil), issues...)}
	}

	diff, err := codeplug.Diff(baseline, file, caps)
	if err != nil {
		return nil, &ValidationFailedError{Issues: []codeplug.Issue{{
			Severity: codeplug.SeverityError,
			Msg:      fmt.Sprintf("clone: PrepareSend: %v", err),
		}}}
	}

	plan := &SendPlan{
		preparedAt:      s.now(),
		identity:        s.sess.Identity(),
		generation:      s.generation,
		baseline:        copyChannels(baseline.Channels),
		candidate:       copyChannels(file.Channels),
		candidateRadio:  file.Radio,
		baselineDigest:  diff.BaselineDigest,
		candidateDigest: diff.CandidateDigest,
		diff:            diff,
		issues:          append([]codeplug.Issue(nil), issues...),
		snapshotPath:    snapshotPath,
	}

	// PrepareSend's journal line is fail-safe too (the ratified policy in
	// doc.go, "Journal durability policy"): a caller must never be handed
	// a plan whose "prepare" line is not durably recorded, since Execute's
	// later refusal checks and the whole obligation-8 audit trail assume
	// it exists.
	journal := s.openJournal(snapshotPath)
	if err := journal.Append(s.now(), "prepare", map[string]any{
		"snapshot_path": snapshotPath,
		// Recorded on EVERY prepare line, true or false (see capsConsented):
		// an absent field and a false one would otherwise be
		// indistinguishable to a reader of an append-only journal.
		"consented_unverified": capsConsented(caps),
		"baseline_digest":      plan.baselineDigest,
		"candidate_digest":     plan.candidateDigest,
		"added":                diff.Added,
		"modified":             diff.Modified,
		"erased":               diff.Erased,
		"unchanged":            diff.Unchanged,
		"blocked":              diff.Blocked,
	}); err != nil {
		s.logger.Printf("clone: journal %s: failed to append %q event: %v", journal.Path(), "prepare", err)
		return nil, fmt.Errorf("clone: PrepareSend: %w", &JournalFailedError{Event: "prepare", Cause: err})
	}

	return plan, nil
}

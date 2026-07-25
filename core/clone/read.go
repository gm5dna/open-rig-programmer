// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// regioner is the optional accessor a driver.Session's concrete type may
// implement to report its discovered regulatory region (e.g. "UK", "US") —
// see core/driver/ft710.Session.Region, whose doc comment explains why
// this is deliberately NOT part of the driver.Session interface: region
// derivation is a per-driver discovery quirk, not a seam-level contract.
// ReadAll type-asserts for it rather than importing any driver package, so
// this generic layer never learns a specific radio's quirks.
type regioner interface {
	Region() string
}

// ReadAll reads every memory slot in the session's current effective
// capabilities, bank by bank, slot by slot, in Capabilities.Banks/Bank.Slots
// order, and assembles the result into a fresh *codeplug.Codeplug —
// obligation 1's "fresh baseline": this is a full, uncached re-read of the
// radio, every time it is called.
//
// The returned Codeplug's Radio carries: Model and CATID from the
// session's capabilities, ReadAt from the injected clock, Port/USBSerial
// from the session's Identity, Region from the session's optional Region()
// accessor (see regioner; empty when the concrete session does not expose
// one), and BaselineDigest — codeplug.Digest of the channels just read, so
// a later PrepareSend/Execute pair can detect any change to this exact
// read.
//
// Progress is reported once per slot, phase "read", 1-based done against
// the total slot count across every bank. ctx is checked between slots
// (not just once at the start): a caller can cancel a long read (a
// multi-hundred-slot MEM+PMS+60M/EMG image) between any two ReadChannel
// calls.
//
// Fix 2 (adjudicated MEDIUM): ReadAll acquires this Service's operation
// lock (see Service.acquireOp/ErrBusy) for its whole duration, refusing a
// concurrent ReadAll/PrepareSend/Execute call with a typed *BusyError
// rather than interleaving. The actual read logic lives in the unexported
// readAll below, which does NOT itself take the lock — PrepareSend calls
// readAll directly (holding its OWN, single acquireOp for its whole body)
// so it never self-refuses against its own nested fresh read.
func (s *Service) ReadAll(ctx context.Context) (*codeplug.Codeplug, error) {
	if err := s.acquireOp("ReadAll"); err != nil {
		return nil, err
	}
	defer s.releaseOp()
	return s.readAll(ctx)
}

// readAll is ReadAll's body, without taking the operation lock — see
// ReadAll's doc comment for why.
func (s *Service) readAll(ctx context.Context) (*codeplug.Codeplug, error) {
	caps := s.sess.Capabilities()

	total := 0
	for _, bank := range caps.Banks {
		total += len(bank.Slots)
	}

	channels := make([]codeplug.Channel, 0, total)
	done := 0
	for _, bank := range caps.Banks {
		for _, slot := range bank.Slots {
			if err := ctx.Err(); err != nil {
				return nil, fmt.Errorf("clone: ReadAll: %w", err)
			}
			ch, err := s.sess.ReadChannel(ctx, slot)
			if err != nil {
				return nil, fmt.Errorf("clone: ReadAll: slot %q: %w", slot, err)
			}
			channels = append(channels, ch)
			done++
			s.progress("read", done, total, slot)
		}
	}

	id := s.sess.Identity()
	region := ""
	if r, ok := s.sess.(regioner); ok {
		region = r.Region()
	}

	info := codeplug.RadioInfo{
		Model:          caps.Model,
		CATID:          caps.CATID,
		ReadAt:         s.now(),
		Port:           id.Port,
		USBSerial:      id.USBSerial,
		Region:         region,
		BaselineDigest: codeplug.Digest(channels),
	}

	return &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: generatorID,
		Radio:     info,
		Channels:  channels,
	}, nil
}

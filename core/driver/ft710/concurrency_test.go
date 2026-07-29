// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// TestSession_ReadChannel_NotTornByConcurrentWriteChannel (M3 Codex-review
// fix wave, Fix 2, adjudicated MEDIUM): ReadChannel is two separate wire
// exchanges, MR then MT. This test deterministically forces a concurrent
// WriteChannel's ENTIRE MW+MT sequence into the gap between them, using
// readChannelGapHook (read.go) — see that var's doc comment for why a
// test-only seam is used here rather than random goroutine scheduling
// (empirically near-impossible to reproduce this specific race by
// hammering alone, because of how Go's sync.Mutex favours an
// immediately-re-locking goroutine).
//
// Without a session-level lock spanning ReadChannel's whole MR+MT
// sequence, the read's MR reply (captured BEFORE the concurrent write)
// and its MT reply (captured AFTER) combine into a channel that never
// legitimately existed: the OLD frequency paired with the NEW tag. With
// the fix, WriteChannel cannot send even its first frame until
// ReadChannel's lock is released, so by the time the read's hook releases
// and its MT finally goes out, the write still has not landed and the
// read observes the OLD, coherent state throughout.
//
// See TestSession_ReadWriteChannel_ConcurrentHammer_Coherent below for a
// secondary, randomly-scheduled stress version of the same property.
func TestSession_ReadChannel_NotTornByConcurrentWriteChannel(t *testing.T) {
	_, sess := openSession(t, Simulated)

	reached := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	readChannelGapHook = func() {
		once.Do(func() { close(reached) })
		<-release
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	type readResult struct {
		ch  codeplug.Channel
		err error
	}
	readDone := make(chan readResult, 1)
	go func() {
		ch, err := sess.ReadChannel(testCtx(t), "001")
		readDone <- readResult{ch, err}
	}()

	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatal("ReadChannel never reached the gap between its MR and MT exchanges within 5s")
	}

	writeDone := make(chan error, 1)
	go func() {
		_, werr := sess.WriteChannel(testCtx(t), codeplug.Channel{
			Slot: "001",
			Data: &codeplug.ChannelData{
				FreqHz:     21_000_000,
				Mode:       "USB",
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				Tag:        "NEW",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		})
		writeDone <- werr
	}()

	// Give the concurrent WriteChannel a generous, deterministic window to
	// run to completion in the gap — its own MW+MT are each a 150ms
	// fire-and-forget ErrorWindow (>=300ms total), so this sleeps well
	// past that.
	time.Sleep(600 * time.Millisecond)
	close(release)

	var got readResult
	select {
	case got = <-readDone:
	case <-time.After(5 * time.Second):
		t.Fatal("ReadChannel never completed after the gap hook was released")
	}
	if got.err != nil {
		t.Fatalf("ReadChannel: unexpected error: %v", got.err)
	}

	select {
	case werr := <-writeDone:
		if werr != nil {
			t.Fatalf("WriteChannel: unexpected error: %v", werr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WriteChannel never completed")
	}

	if got.ch.Empty() {
		t.Fatal("ReadChannel returned an empty channel, want the factory's populated M-01")
	}
	freqIsOld := got.ch.Data.FreqHz == 7_000_000
	freqIsNew := got.ch.Data.FreqHz == 21_000_000
	tagIsOld := got.ch.Data.Tag == ""
	tagIsNew := got.ch.Data.Tag == "NEW"
	coherent := (freqIsOld && tagIsOld) || (freqIsNew && tagIsNew)
	if !coherent {
		t.Errorf("ReadChannel returned FreqHz=%d Tag=%q — a TORN channel (frequency data from one moment, tag data from another): ReadChannel's MR and MT must be serialised against a concurrent WriteChannel as one atomic operation", got.ch.Data.FreqHz, got.ch.Data.Tag)
	}
}

// TestSession_ReadWriteChannel_ConcurrentHammer_Coherent (M3 Codex-review
// fix wave, Fix 2, adjudicated MEDIUM — the "-race hammer test" named in
// the fix): a secondary, randomly-scheduled stress check alongside
// TestSession_ReadChannel_NotTornByConcurrentWriteChannel's deterministic
// construction above. Several goroutines concurrently read and write the
// same slot; every writer encodes its own FreqHz directly into its Tag
// (e.g. FreqHz=14001007 -> Tag="F14001007"), so a reader can detect
// tearing directly: whatever it reads back must have a self-consistent
// Freq/Tag pairing. Staleness (reading an OLDER writer's value) is fine —
// there is no ordering requirement across writers — but a Freq from one
// write paired with a Tag from another is exactly the bug this hunts for.
// Kept small (each WriteChannel already costs >=300ms — two 150ms
// fire-and-forget error windows — serialised through one transport.Engine)
// so the suite stays fast; the deterministic test above is the primary
// proof (see its doc comment for why this style alone is not reliable
// enough to serve as that primary proof). Run with -race (per this
// project's verification step): the session-level lock this fix adds
// must not introduce a genuine Go data race either.
func TestSession_ReadWriteChannel_ConcurrentHammer_Coherent(t *testing.T) {
	_, sess := openSession(t, Simulated)

	const (
		writers    = 2
		readers    = 2
		iterations = 3
	)

	var wg sync.WaitGroup
	errCh := make(chan error, writers+readers)

	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				freq := uint32(14_000_000 + w*1_000 + i)
				tag := fmt.Sprintf("F%d", freq)
				ch := codeplug.Channel{
					Slot: "001",
					Data: &codeplug.ChannelData{
						FreqHz:     freq,
						Mode:       "USB",
						CTCSS:      "OFF",
						CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
						Shift:      "SIMPLEX",
						Tag:        tag,
						TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
						ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
					},
				}
				if _, err := sess.WriteChannel(testCtx(t), ch); err != nil {
					errCh <- fmt.Errorf("writer %d iteration %d: WriteChannel: %w", w, i, err)
					return
				}
			}
		}(w)
	}

	for rd := 0; rd < readers; rd++ {
		wg.Add(1)
		go func(rd int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				ch, err := sess.ReadChannel(testCtx(t), "001")
				if err != nil {
					errCh <- fmt.Errorf("reader %d iteration %d: ReadChannel: %w", rd, i, err)
					return
				}
				if ch.Empty() {
					// No writer has landed yet — not a tearing signal.
					continue
				}
				wantTag := fmt.Sprintf("F%d", ch.Data.FreqHz)
				if ch.Data.Tag != "" && ch.Data.Tag != wantTag {
					errCh <- fmt.Errorf("reader %d iteration %d: TORN CHANNEL: FreqHz=%d paired with Tag=%q, want %q — a write's MW/MT landed either side of a concurrent read's MR/MT", rd, i, ch.Data.FreqHz, ch.Data.Tag, wantTag)
					return
				}
			}
		}(rd)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

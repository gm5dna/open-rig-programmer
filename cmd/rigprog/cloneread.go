// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// cloneArmedPrompt is the operator prompt printed only once the reception
// is confirmed armed (spec.md §Read model point 1, §UI/CLI entry:
// arm-before-prompt is a definite ordering, not a detail).
const cloneArmedPrompt = "Put the radio into clone-send mode now."

// cmdCloneRead implements "rigprog read --clone": a whole-image
// clone-mode transfer (core/clonewire), entirely separate from the
// ordinary per-channel CAT read cmdRead otherwise performs. It shares
// read's own --port/--fake/--out/--force/--model flags (read.go), but
// validates --model against wiring.CloneModels(), never
// wiring.SupportedModels() — a clone-mode model implements no
// driver.Session, so it can never appear in the ordinary registry
// (internal/wiring's TestCloneModelsDisjointFromSessionModels).
//
// Model identity is OPERATOR-ASSERTED (spec.md §Identity probe): CHIRP's
// own source cannot distinguish FT-857 from FT-857D, or FT-897 from
// FT-897D — wiring.OpenCloneReader(model) offers ONLY the one Profile the
// caller named as the candidate set, so a same-length sibling never
// surfaces as ErrImageAmbiguous here. This command says so explicitly in
// its own stdout, since nothing about the transfer itself can.
func cmdCloneRead(ctx context.Context, model, portPath string, useFake bool, outPath string, force bool, stdout, stderr io.Writer) int {
	havePort := portPath != ""
	if havePort == useFake {
		fmt.Fprintln(stderr, "rigprog read --clone: exactly one of --port or --fake is required")
		printReadUsage(stderr)
		return exitUsage
	}
	if outPath == "" {
		fmt.Fprintln(stderr, "rigprog read --clone: --out is required")
		printReadUsage(stderr)
		return exitUsage
	}

	profiles, err := wiring.OpenCloneReader(model)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog read --clone: %v\n", err)
		printReadUsage(stderr)
		return exitUsage
	}

	// Refuse-overwrite BEFORE anything radio-touching — same rule and
	// same helper as the ordinary read path (cmdRead); saveCodeplugNoClobber
	// below enforces it again AT THE COMMIT, since a clone-mode transfer
	// can run for well over a minute (see profile.go's deadlines).
	refused, err := checkOverwrite(outPath, force)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog read --clone: checking %s: %v\n", outPath, err)
		return exitError
	}
	if refused {
		fmt.Fprintf(stderr, "rigprog read --clone: %s already exists; use --force to overwrite\n", outPath)
		return exitError
	}

	var port transport.Port
	var closePort func() error
	if useFake {
		port, closePort, err = wiring.OpenCloneFakePort(model)
	} else {
		port, err = wiring.OpenCloneRealPort(portPath, profiles[0])
		if err == nil {
			closePort = port.Close
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "rigprog read --clone: opening port: %v\n", err)
		return exitError
	}
	defer func() { _ = closePort() }()

	// Arm FIRST; only once armed does the operator prompt print (spec.md
	// §Read model point 1) — the deadlines named in profiles[0] are
	// already live the instant Arm returns (core/clonewire's own Arm doc
	// comment), so printing any earlier risks losing a fast-starting
	// radio's opening bytes.
	reception, err := clonewire.Arm(ctx, port, profiles)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog read --clone: arming: %v\n", err)
		return exitError
	}
	<-reception.Armed()

	fmt.Fprintln(stderr, cloneArmedPrompt)
	fmt.Fprintf(stderr, "Model as selected: %s. The image cannot confirm this against a same-length sibling this project's own CHIRP source does not distinguish (e.g. FT-857 vs FT-857D) — this is what you selected, not what the image proves.\n", model)

	img, err := reception.Receive(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog read --clone: %s\n", classifyCloneReceiveErr(err))
		return exitError
	}

	cp := &codeplug.Codeplug{
		Generator: cliGeneratorID,
		Radio: codeplug.RadioInfo{
			Model:  model,
			Port:   portPath,
			ReadAt: time.Now(),
		},
		Channels: clonewire.MapChannels(img),
		RawImage: &codeplug.RawImageBlob{
			Model:     model,
			ProfileID: img.Profile.ProfileID,
			Bytes:     img.Raw,
		},
	}

	if err := saveCodeplugNoClobber(outPath, cp, force); err != nil {
		if errors.Is(err, errDestExists) {
			fmt.Fprintf(stderr, "rigprog read --clone: %s already exists; use --force to overwrite\n", outPath)
			return exitError
		}
		fmt.Fprintf(stderr, "rigprog read --clone: saving %s: %v\n", outPath, err)
		return exitError
	}

	writeCloneReadSummary(stdout, model, cp, outPath)
	return exitSuccess
}

// classifyCloneReceiveErr labels Receive's typed refusal so an operator
// sees which of the three failure classes happened (spec.md §Read model
// point 5: every one is a full refusal, never a partial image), without
// this command needing to invent its own wording for core/clonewire's
// own error text.
func classifyCloneReceiveErr(err error) string {
	switch {
	case errors.Is(err, clonewire.ErrImageIncomplete):
		return fmt.Sprintf("incomplete clone-mode image: %v", err)
	case errors.Is(err, clonewire.ErrImageIncompatible):
		return fmt.Sprintf("incompatible clone-mode image: %v", err)
	case errors.Is(err, clonewire.ErrImageAmbiguous):
		return fmt.Sprintf("ambiguous clone-mode image: %v", err)
	default:
		return err.Error()
	}
}

// writeCloneReadSummary writes cmdCloneRead's stdout success summary:
// model (as operator-selected), raw image size, populated channel count,
// output path.
func writeCloneReadSummary(w io.Writer, model string, cp *codeplug.Codeplug, outPath string) {
	fmt.Fprintf(w, "Model:           %s (operator-selected, not wire-confirmed)\n", model)
	fmt.Fprintf(w, "Image bytes:     %d\n", len(cp.RawImage.Bytes))
	fmt.Fprintf(w, "Channels:        %d\n", len(cp.Channels))
	fmt.Fprintf(w, "Populated:       %d\n", countPopulated(cp.Channels))
	fmt.Fprintf(w, "Output:          %s\n", outPath)
}

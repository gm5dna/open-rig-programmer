// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/csvio"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// settingsCSVHeader is the exact, ordered column set "rigprog settings
// --csv" writes (task-34 brief).
var settingsCSVHeader = []string{"id", "menu", "group", "label", "state", "value"}

// cmdSettings implements "rigprog settings" (task-34 brief): render (and
// optionally export to CSV) a codeplug file's menu/settings snapshot,
// grouped by --model's static settings descriptor (wiring.
// StaticSettingsDescriptor, task 40 — default FT-710). OFFLINE — no
// --port/--fake flags exist here at all, exactly like export/import: this
// function never opens a radio session. Flags-first grammar (stdlib flag
// parsing stops at the first positional): every flag must precede FILE,
// exactly like export's own precedent — see TestBlackbox_SettingsUsage.
func cmdSettings(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("settings", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // this function owns all usage/error output.
	csvOut := fs.String("csv", "", "also write the snapshot to this CSV file path (optional)")
	model := fs.String("model", wiring.DefaultModel, "radio model whose settings descriptor to group by")
	force := fs.Bool("force", false, "overwrite --csv if it already exists")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printSettingsUsage(stdout)
			return exitSuccess
		}
		fmt.Fprintf(stderr, "rigprog settings: %v\n", err)
		printSettingsUsage(stderr)
		return exitUsage
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "rigprog settings: exactly one FILE argument is required")
		printSettingsUsage(stderr)
		return exitUsage
	}
	file := fs.Arg(0)

	if !validateModel(stderr, "settings", *model, printSettingsUsage) {
		return exitUsage
	}

	// Refuse-overwrite is checked BEFORE FILE is even loaded — same shared
	// rule, same shared helper, as export --csv (fileio.go).
	if *csvOut != "" {
		refused, err := checkOverwrite(*csvOut, *force)
		if err != nil {
			fmt.Fprintf(stderr, "rigprog settings: checking %s: %v\n", *csvOut, err)
			return exitError
		}
		if refused {
			fmt.Fprintf(stderr, "rigprog settings: %s already exists; use --force to overwrite\n", *csvOut)
			return exitError
		}
	}

	cp, code := loadCodeplugStrict(stderr, "settings", "", file)
	if cp == nil {
		return code
	}

	if err := checkSettingsSnapshot(cp.Menus); err != nil {
		fmt.Fprintf(stderr, "rigprog settings: %s: %v\n", file, err)
		return exitError
	}

	// wiring.StaticSettingsDescriptor(*model) (task 40: no core/driver/
	// ft710 import needed here any more) already validated model is
	// supported above — this call cannot fail on that account, but errors
	// are still handled rather than assumed. !ok means model is known but
	// its driver has no settings surface at all (never true for FT-710
	// today, but a future model's driver could legitimately lack one —
	// core/driver/optional.go's StaticSettingsProvider doc comment).
	descriptor, ok, err := wiring.StaticSettingsDescriptor(*model)
	if err != nil {
		fmt.Fprintf(stderr, "rigprog settings: %v\n", err)
		return exitError
	}
	if !ok {
		fmt.Fprintf(stderr, "rigprog settings: %s has no settings surface to group by\n", *model)
		return exitError
	}

	recognised, unrecognised := settingsRows(cp.Menus.Entries, descriptor)
	writeSettingsRender(stdout, recognised, unrecognised)
	if cp.Menus.Legacy != nil {
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Note: this file also carries preserved legacy menu data, which is not renderable.")
	}

	if *csvOut != "" {
		f, err := openCSVCommit(*csvOut, *force)
		if err != nil {
			if errors.Is(err, errDestExists) {
				fmt.Fprintf(stderr, "rigprog settings: %s already exists; use --force to overwrite\n", *csvOut)
				return exitError
			}
			fmt.Fprintf(stderr, "rigprog settings: creating %s: %v\n", *csvOut, err)
			return exitError
		}
		// Formula-injection escaping (csvio.EscapeCell — the same rule
		// "rigprog export" uses) is applied to every free-text column
		// (menu/group/label/value) by writeSettingsCSV itself.
		if err := writeSettingsCSV(f, recognised, unrecognised); err != nil {
			f.Close()
			fmt.Fprintf(stderr, "rigprog settings: writing %s: %v\n", *csvOut, err)
			return exitError
		}
		if err := f.Close(); err != nil {
			fmt.Fprintf(stderr, "rigprog settings: closing %s: %v\n", *csvOut, err)
			return exitError
		}
		fmt.Fprintf(stdout, "Rows written: %d\n", len(recognised)+len(unrecognised))
		fmt.Fprintf(stdout, "Output:       %s\n", *csvOut)
	}

	return exitSuccess
}

// checkSettingsSnapshot reports why menus carries nothing renderable, or
// nil if it does (task-34 brief §"Exit codes"). Two distinct error
// shapes: a plain absent-or-empty snapshot ("carries no settings
// snapshot") tells the caller to re-read with --settings; a migrated v1
// file (Menus non-nil, zero Entries, Legacy present — see
// core/codeplug/file.go's loadV1) gets its OWN distinguishing message,
// since that file DOES carry menu data, just none of it renderable by
// this build.
func checkSettingsSnapshot(menus *codeplug.MenuSnapshot) error {
	if menus != nil && len(menus.Entries) > 0 {
		return nil
	}
	if menus != nil && menus.Legacy != nil {
		return errors.New(`preserved legacy menu data is present but not renderable; re-read with "rigprog read --settings" to capture a renderable snapshot`)
	}
	return errors.New(`carries no settings snapshot; re-read with "rigprog read --settings" to capture one`)
}

// settingsRow is one rendered/exported settings entry — the shared shape
// writeSettingsRender and writeSettingsCSV both consume, so the render
// and the CSV export are built from exactly the same data, once.
type settingsRow struct {
	// id is the setting's opaque, radio-neutral ID (driver.SettingItem.ID
	// / codeplug.MenuEntry.ID) — e.g. "010101".
	id string
	// display is the human-facing position (driver.SettingItem.Display,
	// e.g. "01-01-01") for a recognised entry; for an unrecognised entry
	// (no descriptor item to derive one from) it falls back to id.
	display string
	// menu/group/label are the descriptor's own labels; empty for an
	// unrecognised entry (there is no descriptor item to derive them
	// from).
	menu, group, label string
	// state is the MenuEntry's own state (codeplug.MenuEntryState),
	// stringified: "known", "unavailable", or "unsupported".
	state string
	// value is the MenuEntry's raw value, verbatim.
	value string
}

// settingsRows walks descriptor (--model's static settings descriptor —
// wiring.StaticSettingsDescriptor, task 40; was ft710.SettingsDescriptor()
// directly, unconditionally, before this task) in its own display order
// (menu -> group -> item), pairing each item with its matching entries[i]
// by ID. An item with no matching entry is silently skipped (not an error
// condition this task's brief defines a behaviour for; it does not arise
// from a file "rigprog read --settings" itself produced, since
// ReadSettings writes exactly one entry per descriptor item — see
// clone.ReadSettings' doc comment). Every entry whose ID is NOT a
// descriptor item at all — carried forward by MergeMenuSnapshots as
// MenuUnsupported when an older descriptor knew an ID the current one no
// longer does — is returned separately as unrecognised, in entries' own
// order, for the "Unrecognised settings" section/rows.
func settingsRows(entries []codeplug.MenuEntry, descriptor driver.SettingsDescriptor) (recognised, unrecognised []settingsRow) {
	byID := make(map[string]codeplug.MenuEntry, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}
	descriptorIDs := make(map[string]bool, len(byID))

	for _, menu := range descriptor.Menus {
		for _, group := range menu.Groups {
			for _, item := range group.Items {
				e, ok := byID[item.ID]
				if !ok {
					continue
				}
				descriptorIDs[item.ID] = true
				recognised = append(recognised, settingsRow{
					id:      item.ID,
					display: item.Display,
					menu:    menu.Label,
					group:   group.Label,
					label:   item.Label,
					state:   string(e.State),
					value:   e.Value,
				})
			}
		}
	}

	for _, e := range entries {
		if descriptorIDs[e.ID] {
			continue
		}
		unrecognised = append(unrecognised, settingsRow{
			id:      e.ID,
			display: e.ID,
			state:   string(e.State),
			value:   e.Value,
		})
	}
	return recognised, unrecognised
}

// writeSettingsRender renders recognised/unrecognised to w: menu heading
// -> group heading -> items as "Display  Label  Value  [state]" (task-34
// brief's exact column set) — state is shown only when it is NOT the
// plain "known" case, mirroring this package's other optional-annotation
// conventions (e.g. diff.go's Blocked marking). recognised is assumed
// already in descriptor (menu -> group -> item) order — settingsRows'
// own contract — so headings are emitted purely on menu/group CHANGE, a
// single forward pass. Unrecognised entries (if any) follow under a
// distinct "Unrecognised settings" heading.
func writeSettingsRender(w io.Writer, recognised, unrecognised []settingsRow) {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	curMenu, curGroup := "", ""
	first := true
	for _, r := range recognised {
		if r.menu != curMenu {
			curMenu, curGroup = r.menu, ""
			if !first {
				fmt.Fprintln(tw)
			}
			fmt.Fprintf(tw, "%s\n", curMenu)
		}
		if r.group != curGroup {
			curGroup = r.group
			fmt.Fprintf(tw, "  %s\n", curGroup)
		}
		state := ""
		if r.state != string(codeplug.MenuKnown) {
			state = r.state
		}
		fmt.Fprintf(tw, "    %s\t%s\t%s\t%s\n", r.display, r.label, r.value, state)
		first = false
	}

	if len(unrecognised) > 0 {
		if !first {
			fmt.Fprintln(tw)
		}
		fmt.Fprintln(tw, "Unrecognised settings")
		for _, r := range unrecognised {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.id, r.value, r.state)
		}
	}

	_ = tw.Flush()
}

// writeSettingsCSV writes recognised then unrecognised to w as CSV,
// header exactly settingsCSVHeader ("id,menu,group,label,state,value").
// Every free-text column (menu/group/label/value) is escaped against
// CSV/formula injection via csvio.EscapeCell — the SAME rule "rigprog
// export" applies (task-34 brief, Codex plan-review F10); id and state
// are not free text (id is always 6 ASCII digits, state is one of three
// fixed enum words), so neither is escaped.
func writeSettingsCSV(w io.Writer, recognised, unrecognised []settingsRow) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(settingsCSVHeader); err != nil {
		return fmt.Errorf("rigprog: settings: csv: writing header: %w", err)
	}
	writeRow := func(r settingsRow) error {
		row := []string{
			r.id,
			csvio.EscapeCell(r.menu),
			csvio.EscapeCell(r.group),
			csvio.EscapeCell(r.label),
			r.state,
			csvio.EscapeCell(r.value),
		}
		return cw.Write(row)
	}
	for _, r := range recognised {
		if err := writeRow(r); err != nil {
			return fmt.Errorf("rigprog: settings: csv: writing row %q: %w", r.id, err)
		}
	}
	for _, r := range unrecognised {
		if err := writeRow(r); err != nil {
			return fmt.Errorf("rigprog: settings: csv: writing row %q: %w", r.id, err)
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("rigprog: settings: csv: %w", err)
	}
	return nil
}

// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// topUsageTextTemplate is rigprog's top-level usage text: printed for a
// bare "rigprog" invocation, "rigprog help", and an unknown subcommand.
// Its %s placeholder is filled at print time (printUsage) with
// wiring.SupportedModels()'s own comma-joined, sorted list — WHATEVER that
// list currently holds — so this line stays accurate without a hand edit
// whenever a driver is registered or removed (task 40, M9a-4: the CLI
// neutralisation). No model name and no model COUNT is written here on
// purpose: an earlier version of this comment named the list's contents
// ("today just \"FT-710\"") and was falsified twice over, at M9c-6 and
// again at M9d-2, while the CODE it describes never needed touching.
// Every OTHER command in the list below is already model-neutral prose
// (no subcommand name here ever mentioned FT-710 directly).
const topUsageTextTemplate = `rigprog is a command-line memory programmer for Yaesu radios (currently: %s).

Usage:
  rigprog <command> [flags]

Commands:
  ports      list candidate serial ports, ranked by likelihood of being the target radio's CAT interface
  probe      open a session and report the connected radio's identity and inventory
  read       read every memory slot from a radio and save it as a codeplug file
  write      send a codeplug file's changes to a radio
  diff       compare a codeplug file against a fresh read of a radio
  export     export a codeplug file to CSV (offline; no radio)
  import     import a CSV (native or CHIRP) into a codeplug file (offline; no radio)
  settings   show (or export to CSV) a codeplug file's menu/settings snapshot (offline; no radio)
             "settings unverified-writes" grants, revokes or lists per-model consent to writing
             fields never proved on that radio (a hardware-verified model has none, and is refused)
  version    report this build's version (also "rigprog -v")
  help       show this message

Run "rigprog <command> -h" for a command's own flags.
`

// printUsage writes rigprog's top-level usage text to w.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, topUsageTextTemplate, strings.Join(wiring.SupportedModels(), ", "))
}

// portsUsageText is "rigprog ports"'s own usage text.
const portsUsageText = `rigprog ports — list candidate serial ports.

Usage:
  rigprog ports

Discovers every serial port the OS currently exposes and prints a table
ranked by how likely each is to be the target radio's CAT interface: path,
score, description, VID:PID, and the signals behind the score. Ranking is
a heuristic; "rigprog probe --port <path>" performs the definitive active
ID check. --fake is not accepted — ports enumerates real devices only.
`

// printPortsUsage writes "rigprog ports"'s usage text to w.
func printPortsUsage(w io.Writer) {
	fmt.Fprint(w, portsUsageText)
}

// probeUsageText is "rigprog probe"'s own usage text.
const probeUsageText = `rigprog probe — identify the radio attached to a port.

Usage:
  rigprog probe --port <path> [--model NAME]
  rigprog probe --fake [--model NAME]

Flags:
  --port PATH   real serial port device path (e.g. /dev/cu.usbserial-XXXX)
  --fake        use the in-process simulated radio instead of a real port
  --model NAME  radio model to target (default: FT-710)

Exactly one of --port or --fake is required. Opens a session — which
probes the radio's identity — and prints model, CAT ID, port, USB
serial, region, and 60 m/EMG inventory. An unrecognised --model refuses
before any port is touched, naming every model this build supports.
`

// printProbeUsage writes "rigprog probe"'s usage text to w.
func printProbeUsage(w io.Writer) {
	fmt.Fprint(w, probeUsageText)
}

// readUsageText is "rigprog read"'s own usage text.
const readUsageText = `rigprog read — read every memory slot from a radio and save it as a codeplug file.

Usage:
  rigprog read --port <path> --out <file> [--settings] [--model NAME] [--force] [--snapshot-dir <dir>]
  rigprog read --fake --out <file> [--settings] [--model NAME] [--force] [--snapshot-dir <dir>]

Flags:
  --port PATH          real serial port device path (e.g. /dev/cu.usbserial-XXXX)
  --fake                use the in-process simulated radio instead of a real port
  --out FILE            output codeplug file path (required)
  --settings            also read the radio's menu/EX settings surface (opt-in; adds significant wire time)
  --model NAME          radio model to target (default: FT-710)
  --force               overwrite --out if it already exists
  --snapshot-dir DIR    snapshot/journal directory (default: <UserConfigDir>/rigprog/snapshots)

Exactly one of --port or --fake is required. Reads every memory slot,
prints progress to stderr, and saves the result to --out. Refuses to
overwrite an existing --out file unless --force is given; never
overwrites as a side effect of a failed read.

Without --settings (the default), the channel read is entirely
unchanged: zero settings/EX wire traffic, and the saved file's "menus"
field is absent. With --settings, after the channel read completes, every
item in the radio's menu/settings surface is read too; the result is
merged into the saved file's menu snapshot, and two extra summary lines
are printed ("Settings read: N", "Settings unavailable: M"). A settings
read failure AFTER a successful channel read aborts the whole command —
--out is never written — rather than saving an artefact that silently
lacks the settings the caller asked for. Use "rigprog settings" (offline)
to inspect or export a file's settings snapshot afterwards.
`

// printReadUsage writes "rigprog read"'s usage text to w.
func printReadUsage(w io.Writer) {
	fmt.Fprint(w, readUsageText)
}

// diffUsageText is "rigprog diff"'s own usage text.
const diffUsageText = `rigprog diff — compare a codeplug file against a fresh read of a radio.

Usage:
  rigprog diff --port <path> [--model NAME] <file>
  rigprog diff --fake [--model NAME] <file>

Flags:
  --port PATH   real serial port device path (e.g. /dev/cu.usbserial-XXXX)
  --fake        use the in-process simulated radio instead of a real port
  --model NAME  radio model to target (default: FT-710)

Exactly one of --port or --fake is required, together with exactly one
FILE argument. Reads a fresh baseline from the radio (progress to
stderr), then reports how FILE differs from it: Added / Modified /
Erased sections, followed by a count line (including Unchanged). This
command is read-only: it never snapshots, journals, or writes to the
radio. Exit 0 whenever the diff was computed and rendered, even when
there are differences or blocked entries.
`

// printDiffUsage writes "rigprog diff"'s usage text to w.
func printDiffUsage(w io.Writer) {
	fmt.Fprint(w, diffUsageText)
}

// writeUsageText is "rigprog write"'s own usage text.
const writeUsageText = `rigprog write — send a codeplug file's changes to a radio.

Usage:
  rigprog write --port <path> [--model NAME] [--yes] [--firmware VER] [--snapshot-dir DIR] FILE
  rigprog write --fake [--model NAME] [--yes] [--firmware VER] [--snapshot-dir DIR] FILE

All flags must precede the FILE argument: stdlib flag parsing stops
reading flags at the first non-flag argument, so a flag placed after
FILE is rejected as an unexpected extra argument, not accepted.

Flags:
  --port PATH           real serial port device path (e.g. /dev/cu.usbserial-XXXX)
  --fake                 use the in-process simulated radio instead of a real port
  --model NAME           radio model to target (default: FT-710)
  --yes                  skip the interactive confirmation prompt (required for non-interactive runs)
  --firmware VER         confirmed radio firmware version (required on this session's first write, non-interactively)
  --snapshot-dir DIR     snapshot/journal directory (default: <UserConfigDir>/rigprog/snapshots)

Exactly one of --port or --fake is required, together with exactly one FILE
argument. Reads a fresh baseline from the radio, prepares a send plan — a
snapshot of that baseline is saved before anything else, even if what
follows refuses — renders the diff (Added/Modified/Erased-unsupported/
Blocked, with reasons), and, once confirmed, writes every unblocked
Added/Modified slot one at a time, verifying each write's read-back before
moving to the next. Blocked entries (including every erase — the radio
has no CAT erase command; delete a channel from the radio's own front
panel: hold [V/M], select the channel, touch [ERASE]) are always shown
before confirmation and are never sent, without prompting either way:
a plan with NOTHING PENDING AT ALL exits 0 ("Nothing to send."); a plan
whose pending changes are ALL blocked exits 3 instead — never the same
"nothing to send" message, since the working copy does NOT match the
radio in that case, only nothing could be honoured.

The FIRST write on a session requires a firmware version confirmed by a
human reading it off the radio's front panel (or SD-card version screen)
— there is no CAT query for it. Once confirmed, later writes on the same
session do not need to repeat it.

Ctrl-C is honoured only BETWEEN slots: an in-flight write+verify pair
always completes before a cancellation is acted on.

Exit codes: 0 success, including "nothing to send" when the working copy
genuinely matches the radio; 2 usage (also: a non-interactive run without
--yes); 3 the candidate file failed validation, OR every pending change
is blocked (nothing was sendable, named with reasons — see above); 4
refused before any write reached the radio (stale baseline, session
changed, confirmation declined or mismatched, or firmware unconfirmed); 5
aborted after at least one write reached the radio — the printed journal
records exactly what happened, and the printed snapshot can be re-sent
with a later "rigprog write" run.
`

// printWriteUsage writes "rigprog write"'s usage text to w.
func printWriteUsage(w io.Writer) {
	fmt.Fprint(w, writeUsageText)
}

// exportUsageText is "rigprog export"'s own usage text.
const exportUsageText = `rigprog export — export a codeplug file to CSV.

Usage:
  rigprog export --csv OUT [--force] FILE

Flags:
  --csv OUT   output CSV file path (required)
  --force     overwrite --csv if it already exists

OFFLINE: does not open a radio session — --port/--fake are not accepted.
Loads FILE strictly, then writes one CSV row per memory slot (including
empty slots) to OUT. This is rigprog's own CSV schema: a lossless round trip
through "rigprog import --csv", including every field's known/unknown/
unavailable state. CHIRP-format CSV is import-only — there is no "--chirp"
export. Refuses to overwrite an existing --csv file unless --force is given.
`

// printExportUsage writes "rigprog export"'s usage text to w.
func printExportUsage(w io.Writer) {
	fmt.Fprint(w, exportUsageText)
}

// importUsageText is "rigprog import"'s own usage text.
const importUsageText = `rigprog import — import a CSV into an existing codeplug file.

Usage:
  rigprog import --csv IN --into BASE --out FILE [--model NAME] [--force]
  rigprog import --chirp IN --into BASE --out FILE [--model NAME] [--force]

Flags:
  --csv IN     rigprog's own CSV format to import (mutually exclusive with --chirp)
  --chirp IN   CHIRP-next CSV format to import (mutually exclusive with --csv)
  --into BASE  existing codeplug JSON to merge onto (required; typically from "rigprog read")
  --out FILE   output codeplug file path (required)
  --model NAME radio model to validate against (default: FT-710)
  --force      overwrite --out if it already exists (required if --out equals --into)

OFFLINE: does not open a radio session — --port/--fake are not accepted.
Exactly one of --csv/--chirp is required, together with --into and --out.

An imported CSV carries no radio identity, and (for CHIRP) no slot
inventory of its own — import is a MERGE onto BASE, not a standalone
codeplug builder. BASE supplies the Radio identity, Schema, and slot
inventory the import merges onto:
  --csv   a full channel set, one row per slot. Its slot inventory must
          match BASE's exactly (rigprog's CSV is a lossless round trip;
          a mismatch means it came from a different radio/region) — on a
          match, BASE's channels are replaced wholesale.
  --chirp a sparse channel set (only the rows CHIRP could map at all).
          Every place CHIRP data could not be carried across losslessly
          is reported to stdout; a Blocking entry refuses the import
          outright (nothing written, exit 3) until resolved. Otherwise,
          matched slots are merged onto BASE by slot — every other BASE
          slot keeps its current contents untouched. Every imported slot
          must already exist in BASE's inventory, or the import is
          refused, naming the offending slots.

After merging, the result is validated against --model's static offline
capabilities, and every issue (error or
warning) is printed under an advisory label — the static baseline lacks
this radio's discovered regional inventory (60m/EMG), so it can report
spurious issues a live session would not; authoritative validation
happens at write time against the connected radio. This never gates the
exit code: a successful merge always writes --out and exits 0. Refuses
to overwrite an existing --out file unless --force is given.
`

// printImportUsage writes "rigprog import"'s usage text to w.
func printImportUsage(w io.Writer) {
	fmt.Fprint(w, importUsageText)
}

// settingsUsageText is "rigprog settings"'s own usage text. Flags-first
// (task-34 brief): every flag must precede FILE, exactly like export's
// own synopsis — see TestBlackbox_SettingsUsage's trailing-flag-form
// pin.
const settingsUsageText = `rigprog settings — show (or export to CSV) a codeplug file's menu/settings snapshot.

Usage:
  rigprog settings [--csv OUT] [--model NAME] [--force] FILE
  rigprog settings unverified-writes
  rigprog settings unverified-writes <model> on|off

Flags:
  --csv OUT     also write the snapshot to this CSV file path (optional)
  --model NAME  radio model whose settings descriptor to group by (default: FT-710)
  --force       overwrite --csv if it already exists

"unverified-writes" is a RESERVED first argument — it selects the consent
sub-mode below, so it can never be read as a FILE to render. A codeplug
file of that exact name must be given another (any extension will do).

CONSENT SUB-MODE (no flags; nothing is read from or written to a radio):
with no further arguments it lists every model this build supports, with
its slug and its current state; with <model> on|off it records a decision
and reports it. Consent applies ONLY to models with unverified write
support — radios this project has never written to, whose write gate is
closed until the owner opens it. A model whose writes are hardware-
verified has nothing to consent to: it is listed as
"n/a (hardware-verified)", and a grant or revoke for it is refused as a
usage error rather than recorded as a decision about nothing. An "off" is
STORED, not forgotten: withholding consent is a decision, and it is kept
so nothing asks again. The decisions live in a settings file shared with
the GUI, whose path the listing prints; a file this build cannot read is
reported, naming it, and is never overwritten.

OFFLINE: does not open a radio session — --port/--fake are not accepted.
Loads FILE strictly, then renders its menu/settings snapshot (captured by
an earlier "rigprog read --settings") grouped by --model's static
menu/group structure: each item shows its display position, label, and
value, with its state noted when it is not a plain known value. Entries
the current build's descriptor does not recognise (e.g. preserved from an
older/newer descriptor version) are listed separately, under
"Unrecognised settings". A file carrying only preserved legacy (pre-schema-2)
menu data, with nothing renderable, is reported as an error distinct from
a file with no settings snapshot at all.

--csv OUT additionally writes the same data as CSV, columns exactly
"id,menu,group,label,state,value" — free-text columns are escaped against
CSV/formula injection (core/csvio.EscapeCell), same rule as
"rigprog export". Refuses to overwrite an existing --csv file unless
--force is given.

Exit codes: 0 the file was loaded and its snapshot rendered, or the
consent decision was listed/recorded (an entry recorded as unavailable in
a partial snapshot is a recorded fact, not an error — still 0); 1 FILE
could not be loaded, or carries no renderable settings snapshot (re-read
it with "rigprog read --settings"), or the settings file could not be
read or replaced (its own message names it); 2 usage (missing FILE, an
unrecognised flag, an unrecognised --model, a flag placed after FILE —
flags must precede FILE — or, in the consent sub-mode, a flag, a wrong
number of arguments, an unrecognised model, a state word that is neither
"on" nor "off", or a model whose writes are hardware-verified). Exit
codes 3/4/5 are not
used by this command: there is no radio session to block, refuse, or
abort against.
`

// printSettingsUsage writes "rigprog settings"'s usage text to w.
func printSettingsUsage(w io.Writer) {
	fmt.Fprint(w, settingsUsageText)
}

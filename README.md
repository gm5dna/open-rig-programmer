# open-rig-programmer

A free, open-source memory-channel programmer for Yaesu HF
transceivers, for macOS and Linux. Read your radio's memory channels
into a file, edit them in a spreadsheet-style grid or your favourite
CSV editor, and send the changes back — with a safety net at every
step.

It ships as two faces of one core: **`rigprog`**, a command-line tool,
and a desktop **GUI**. Both talk to the radio over the ordinary USB
CAT connection; no programming cable or card juggling needed.

## Supported radios

| Radio | Read | Write | Notes |
| --- | --- | --- | --- |
| **FT-710** | ✅ | ✅ | Fully supported. Read and write paths verified against a real UK FT-710 (see `docs/hardware-notes.md`). |
| **FTdx10** | ✅ | ⚠️ opt-in | Read, probe and settings snapshot. Writing is refused until you enable unverified writes for this radio — see below. |
| **FTdx101D** | ✅ | ⚠️ opt-in | As FTdx10. |
| **FTdx101MP** | ✅ | ⚠️ opt-in | As FTdx10. |

The honest small print: the FT-710 is the only radio this project has
ever had on the other end of a cable. The three FTdx models were built
from Yaesu's published CAT manuals and tested against protocol
simulators — no physical FTdx10 or FTdx101 has been connected to this
project. Reading is safe by design (the tool only ever sends documented
read commands to them), and the write path stays shut unless you
deliberately open it, one radio at a time. If you own one and want to
help run the same characterisation trials the FT-710 had, please open
an issue.

### Unverified writes

Every memory-write command this tool would send to an FTdx10 or an
FTdx101 is documented in the manufacturer's CAT reference and exercised
against a simulator here — and none of it has been proven on a real
radio. That is all *unverified* means: not a guess, and not proof
either. So the tool will not send any of it until you say so, per
radio:

```sh
rigprog settings unverified-writes                # list every model and its current state
rigprog settings unverified-writes FTdx10 on      # allow unverified writes to the FTdx10
rigprog settings unverified-writes FTdx10 off     # withhold consent again
```

The GUI asks the same question once, just after you first connect to
one of these radios, and its *Unverified writes…* button opens the same
list at any time — connected or not — to grant or revoke. Both faces
read and write one file (`rigprog/settings.json` under your user
configuration directory; the CLI listing prints the exact path), so a
decision made in either holds for both. An "off" is stored rather than
forgotten: withholding consent is a decision too, and keeping it is
what stops anything asking you twice.

The FT-710 has nothing to consent to — its writes are hardware-verified
— so it is listed as `n/a (hardware-verified)`, and a grant for it is
refused rather than recorded as a decision about nothing.

Consent changes what the tool is allowed to send, not how it sends it.
Everything under *Safety design* below still runs unchanged, and the
per-channel write-then-verify is what limits how far a wrong command
could get: every channel is read straight back after it is written and
compared against what was sent, and the run stops at the first mismatch
rather than carrying on down the list. Deleting a channel stays refused
whatever you consent to, and menu settings stay read-only. Each send's
journal records whether consent is what opened the write gate.

Without a grant nothing is left half-done: `rigprog write` blocks every
change and sends no write command.

## What it does

- Reads the whole memory surface — the 99 regular memories, the 9
  PMS (programmable memory scan) pairs, and on radios that have them
  the fixed 60 m and emergency channels — into a **codeplug file**
  you can keep, diff, and edit.
- A channel grid in the GUI with keyboard navigation, paste, and
  per-column editors — or export/import **CSV** and edit anywhere.
- **CHIRP CSV import**, so an existing channel list can come across
  in one step.
- A **menu settings snapshot**: `rigprog read --settings` captures
  the radio's menu configuration alongside the channels, and
  `rigprog settings` (or the GUI's settings view) displays it.
  Settings are read-only by design — see below.
- A **built-in simulated radio**: every command accepts `--fake` (the
  GUI has a *Demo* button), so you can try the whole workflow with no
  radio attached.

## Safety design

Writing to a radio's memory deserves paranoia, so every send walks
the same choreography, in the CLI and the GUI alike:

1. a **fresh read** of the radio first — never a stale picture;
2. a **snapshot** of the radio's current contents saved to disk
   before anything changes;
3. a **reviewed diff** — you see exactly what would change, including
   anything blocked, and confirm it explicitly;
4. a one-time **firmware version confirmation** on first use;
5. **write-then-verify per channel**, with an append-only journal of
   exactly what happened.

On a radio whose write commands have never been proven on hardware, the
first two steps still run — the radio is read, and a snapshot is kept —
but a further gate stands in front of the send itself: without a grant,
every change is reported as blocked and no write command leaves the
tool. See *Unverified writes* above.

Some things the FT-710 simply cannot do over CAT, and the tool blocks
them honestly rather than pretending: there is no erase command (so
deletions are refused with a reason), per-channel CTCSS tone and
scan-skip have no CAT write, and clarifier changes are refused
because the radio silently ignores them.

Menu settings **writing** is deliberately not implemented — not
unfinished, declined. No code path in this repository can send a
menu-change frame, and the outbound command allowlist refuses the
entire menu-write wire shape. The reasoning, and what would need to
change to revisit it, is recorded in `docs/menu-write-decision.md`.

## Install

Download from the [Releases page](../../releases):

| Platform | Asset |
| --- | --- |
| macOS GUI | `open-rig-programmer-<version>-darwin-universal.app.zip` |
| macOS CLI | `rigprog-<version>-darwin-universal.tar.gz` |
| Linux CLI (x86-64) | `rigprog-<version>-linux-amd64.tar.gz` |
| Linux CLI (ARM64) | `rigprog-<version>-linux-arm64.tar.gz` |
| Linux GUI+CLI .deb (x86-64) | `open-rig-programmer_<version-without-v>_amd64.deb` |
| Linux GUI+CLI .deb (ARM64) | `open-rig-programmer_<version-without-v>_arm64.deb` |

`<version>` is the release tag exactly as it appears (`v1.1.0`);
`<version-without-v>` is that same string with the leading `v` dropped
(`1.1.0`), because Debian package filenames carry no `v`.

macOS GUI first run: the app is ad-hoc signed, so Gatekeeper refuses
a plain double-click. **Right-click the app and choose Open** the
first time; after that it opens normally.

The Linux CLI binaries are static — download, untar, run. On Debian,
Ubuntu and Mint the `.deb` is the whole install in one step: the GUI,
the `rigprog` CLI, a desktop entry and the ModemManager udev rule.
Install it with `sudo apt install ./<the downloaded .deb>`, which
resolves the WebKitGTK and GTK libraries it needs. That means Ubuntu
22.04 or later, Debian 12 or later, and the Mint releases built from
those — the WebKitGTK 4.1 runtime the GUI links against first appeared
in them. On other distributions, take the static CLI tarball or build
the GUI from source (see below).

`SHA256SUMS` on the release page lets you verify any download.

## Getting started

**Firmware**: memory CAT on the FT-710 needs firmware **V01-10 or
later**. There is no CAT query for the firmware version, so check the
radio's front panel or SD-card version screen first.

**Serial, macOS**: nothing to install. The radio's CP2105 USB adapter
shows up with two serial ports; the *Enhanced* one is the CAT port,
and conveniently it is the only one that opens on macOS, so you
cannot land on the wrong one.

**Serial, Linux**: joining the `dialout` group
(`sudo usermod -aG dialout "$USER"`, then log out and in) is the one
manual step. Keeping ModemManager away from the radio needs a udev
rule, and the `.deb` installs that rule for you; tarball and
from-source users still create it by hand. That rule, and the rest of
the port setup, is in
[docs/linux-setup.md](docs/linux-setup.md). (Fair warning: the Linux
instructions have not yet been verified against real hardware — see
that document's Status section.)

**A CLI session** looks like this:

```sh
rigprog ports                                  # list candidate serial ports, ranked
rigprog probe --port /dev/cu.SLAB_USBtoUART    # confirm which port answers, and as what
rigprog read  --port ... --out radio.json      # read all memory slots into a codeplug file
rigprog diff  --port ... edited.json           # preview changes against a fresh read
rigprog write --port ... edited.json           # send the changes, with the full safety flow
```

`ports` is a heuristic shortlist; `probe` is the definitive check —
it opens a session and asks the radio to identify itself. `export`,
`import` and `settings` work offline on codeplug files, no radio
needed. All radio-facing commands take `--model` to pick the driver
(`FT-710` is the default; an unrecognised name refuses immediately
and lists every model the build supports) and `--fake` to use the
simulator instead of a port.

`rigprog settings unverified-writes` is the one sub-mode with no
codeplug file and no radio in it: it lists, grants and revokes per-radio
consent to sending write commands that have never been proven on that
model (see *Unverified writes* above).

`rigprog version` (or `-v`) reports which build you are running —
quote it in bug reports. The GUI shows the same string in its status
bar. A build that says `dev (unreleased build)` did not come from the
release pipeline.

**The GUI** follows the same shape: pick the radio and the port,
connect (or *Demo*), read, edit in the grid, then *Send* — which walks
the identical safety flow, showing the reviewed diff and any blocked
entries with reasons before anything is transmitted. The radio picker
beside the port list offers every model this build supports, so the GUI
reaches the same radios the CLI's `--model` does; it is fixed for as
long as a session is open. Connect to a radio whose writes are
unverified and the consent question appears once, stating plainly what
has and has not been proven before either answer is recorded; the
simulator is not a radio, so *Demo* never asks. While a session is
running with unverified writes enabled, a standing amber marker sits in
the connection bar and leads back to the grants list.

## Building from source

Prerequisites: Go 1.25+, Node.js 22.12+, and the Wails v2 CLI for the
GUI:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
```

(Ensure `$(go env GOPATH)/bin` is on your `PATH`.) On Linux you also
need GTK3 and WebKit2GTK headers:

```sh
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

The frontend must be built first — `app/main.go` embeds its output:

```sh
# 1. Frontend (once, from the repository root)
cd app/frontend && npm install && npm run build

# 2. Core library and CLI (from the repository root)
go test ./...
go build ./cmd/rigprog

# 3. GUI (from app/)
cd app
wails dev                      # live-reloading development build
wails build                    # production build → app/build/bin/
wails build -tags webkit2_41   # Linux needs this build tag
```

Recommended once per clone — a versioned pre-push hook that refuses
to push anything matching a private-fixture pattern (the same guard
CI runs):

```sh
git config core.hooksPath scripts/git-hooks
```

## Repository layout

| Path | Contents |
| --- | --- |
| `core/` | The library: CAT codec (`cat`, plus `cat/ftdx10`, `cat/ftdx101`), capability model (`spec`), codeplug model and diff (`codeplug`), CSV I/O (`csvio`), serial transport (`transport`), radio drivers (`driver/ft710`, `driver/ftdx10`, `driver/ftdx101`), and the safe send choreography (`clone`). |
| `cmd/rigprog/` | The CLI. |
| `app/` | Wails v2 + Svelte desktop GUI. |
| `internal/` | The radio simulators (`fakeradio`, `fakedx10`, `fakedx101`), composition-root wiring, the shared settings store the CLI and GUI both use for unverified-write consent (`userconfig`), menu-table generator (`extable`), and the import-graph guard tests (`guards`). |
| `docs/` | Hardware findings, Linux setup, the menu-write decision, and the fixture redaction policy. |
| `docs/fixtures-private/` | Git-ignored. Raw radio backups and serial captures — never committed. |

## Licence

GPL-3.0-or-later. See [LICENSE](LICENSE) for the full text.

This program is distributed in the hope that it will be useful, but
**WITHOUT ANY WARRANTY**, without even the implied warranty of
merchantability or fitness for a particular purpose — see the GPL for
details. **If you connect this software to your transceiver, you do
so entirely at your own risk**; the authors accept no liability for
any damage to your radio, its firmware, or its memory contents.

## Protocol references

CAT protocol facts are derived from Yaesu's published CAT Operation
Reference Manuals: **FT-710 (2306-C)**, **FTdx10 (2308-F)**, and
**FTdx101D/MP (2308-L)**.

# open-rig-programmer

A free, open-source memory-channel programmer for Yaesu and Icom
amateur radio transceivers, for macOS and Linux. Read your radio's memory channels
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
| **IC-7610** | ✅ | ⚠️ opt-in | Read and probe over CI-V. Writing is refused until you enable unverified writes for this radio — see below. |
| **IC-7300** | ✅ | ⚠️ opt-in | As IC-7610. |
| **IC-7300MK2** | ✅ | ⚠️ opt-in | As IC-7610. |
| **IC-705** | ✅ | ⚠️ opt-in | As IC-7610. |
| **IC-9700** | ✅ | ⚠️ opt-in | As IC-7610. |
| **IC-905** | ✅ | ⚠️ opt-in | As IC-7610. |

The honest small print: the FT-710 is the only radio this project has
ever had on the other end of a cable. The three FTdx models were built
from Yaesu's published CAT manuals and tested against protocol
simulators — no physical FTdx10 or FTdx101 has been connected to this
project. The same is true of every Icom model: IC-7610, IC-7300,
IC-7300MK2, IC-705, IC-9700 and IC-905 were all built from Icom's
published documentation and tested against simulators built
independently from it — a standalone CI-V Reference Guide for five of
the six, and the IC-7300's own CI-V chapter (§19) inside its Full
Manual, since Icom prints no standalone CI-V guide for that model (see
*Protocol references* below for each model's exact citation) — no
physical example of any of the six has been connected to this project
either (see *Icom models* below for what that costs each of them
specifically). Reading is safe
by design (the tool only ever sends documented read commands to them),
and the write path stays shut unless you deliberately open it, one
radio at a time. If you own one and want to help run the same
characterisation trials the FT-710 had, please open an issue.

### Unverified writes

Every memory-write command this tool would send to any of these nine
manual-derived models — FTdx10, FTdx101D, FTdx101MP, IC-7610, IC-7300,
IC-7300MK2, IC-705, IC-9700 and IC-905 — is documented in the
manufacturer's own reference and exercised against a simulator here —
and none of it has been proven on a real radio. That is all
*unverified* means: not a guess, and not proof either. So the tool will
not send any of it until you say so, per radio:

```sh
rigprog settings unverified-writes                # list every model and its current state
rigprog settings unverified-writes FTdx10 on      # allow unverified writes to the FTdx10
rigprog settings unverified-writes FTdx10 off     # withhold consent again
rigprog settings unverified-writes IC-7610 on     # the same grant, for an Icom model
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

### Icom models

IC-7610, IC-7300, IC-7300MK2, IC-705, IC-9700 and IC-905 talk a
different wire protocol from the Yaesu models above (Icom's CI-V,
rather than Yaesu's CAT), and each carries its own honesty rows beyond
the shared "no radio has ever been connected" small print above.

Three costs are shared by all six:

- **No `--civ-address` option.** Each driver talks only to its one
  factory CI-V address (98h IC-7610, 94h IC-7300, B6h IC-7300MK2, A4h
  IC-705, A2h IC-9700, ACh IC-905) and there is no setting to change
  it. Two different things can happen when this driver meets a radio
  it did not expect. A different Icom model at ITS OWN factory address
  simply does not answer — nothing was heard from, so nothing can be
  attributed, and Open reports a plain timeout
  (`core/driver/ic7610/doc.go:188-192`; `core/driver/ic7300/doc.go:229-234`;
  `core/civ/tier_test.go:27-33`). A radio actually sitting ON this
  driver's one address is instead caught by the probe's
  address-geometry and record-length fingerprint — but whether that
  refusal NAMES a wrong radio depends on the model, and attribution is
  not the default: IC-7300, IC-7300MK2, IC-705 and IC-905 each mint a
  `driver.WrongRadioError` naming what they found
  (`core/driver/ic7300/ic7300.go:270`; `core/driver/ic7300mk2/ic7300mk2.go:272`;
  `core/driver/ic705/ic705.go:364`; `core/driver/ic905/ic905.go:495`),
  while IC-7610 and IC-9700 NEVER mint one for any same-address
  collision, by design — they hold no cross-model table and refuse to
  guess an identity they cannot support
  (`core/driver/ic7610/ic7610.go:142-145`;
  `core/driver/ic9700/ic9700.go:325-329`; `core/driver/ic9700/doc.go:391`).
  Even where a driver CAN attribute, one pair defeats it in practice:
  an IC-9700 moved onto the IC-705's A4h address fails the IC-705's
  open as an unattributed address parse error, not as a named
  wrong-radio refusal, because the address-geometry check pre-empts
  the length check that would otherwise have named it
  (`core/civ/tier_test.go:68-80`).
- **The tone picker stays list-driven while every model's tone range
  is numeric (enabler E3).** All six declare a numeric
  `CTCSSToneRange` rather than a fixed tone chart, because their tone
  spans are BCD frequencies, not indices — but the picker widget
  itself was built for a list. The channel grid still shows and
  round-trips tones on every model; only the picker cannot offer them
  (`core/driver/ic7610/caps.go:342-345`; `core/driver/ic9700/caps.go:177-179`;
  same declaration in each of the other four models' own `caps.go`).
- **A channel outside the write gate's template is refused, never
  silently changed (ruling E6).** Every one of the six records carries
  at least one region no `codeplug` field maps, and a slot may be
  written only when those unmapped regions already match the
  profile's template. What that costs differs by model:
  - **IC-7610, IC-705, IC-9700, IC-905**: a channel already in a
    Select scan group (★1/★2/★3) cannot be written by this program at
    all — the SELECT nibble is unmapped and there is no honest value
    to preserve, so the write is refused, naming the reason
    (`core/driver/ic7610/doc.go:220-249`; `core/driver/ic705/write.go:265-280`;
    `core/driver/ic9700/write.go:529-531`; `core/driver/ic905/write.go:578-590`).
    The IC-7610 additionally refuses a channel whose data mode is
    DATA 1/2/3, for the same unmapped-nibble reason
    (`core/driver/ic7610/doc.go:235-238`).
  - **IC-7300 and IC-7300MK2**: a Select-group channel writes
    normally — the SELECT nibble round-trips, carried through
    unchanged from the record the radio holds
    (`core/driver/ic7300/write.go:455-477`;
    `core/driver/ic7300mk2/write.go:463-485`). What is refused instead
    is a Split-ON channel: it reads normally but cannot be written
    back, because the split flag shares record byte ③ with the SELECT
    nibble and the profile leaves the whole byte's high nibble
    unmapped (`core/driver/ic7300/doc.go:174-178`;
    `core/driver/ic7300mk2/doc.go:187-195`). Both also refuse a CREATE
    into an empty slot, since the SELECT nibble has no honest default
    to write (`core/driver/ic7300/write.go:433`;
    `core/driver/ic7300mk2/write.go:441`).
  In every case the write is refused, naming the reason, never
  downgraded or cleared.

All six open at 19200 baud by default, but what grades that default
differs, and none of it is a reading of a printed factory value:

- **IC-7610** — ASSUMED, an arbitrary pick among the six rates the
  guide names; it marks no default at all
  (`core/driver/ic7610/doc.go:200-219`; `core/driver/ic7610/caps.go:349,359`).
- **IC-7300** — a CHOICE: the highest rate common to this radio's
  printed `[USB]` and `[REMOTE]` rate lists, since both its baud items
  ship set to `Auto` (`core/driver/ic7300/caps.go:261-268`).
- **IC-7300MK2** — a conservative derivation from a wake-up-command
  table this guide prints for an unrelated purpose; it names no baud
  list and no factory default at all
  (`core/driver/ic7300mk2/doc.go:301-317`).
- **IC-705** — ASSUMED, and so is the whole baud list: this radio's
  CI-V Reference Guide prints no baud information for the CI-V port at
  all (`core/driver/ic705/caps.go:197-205`; matrix §1 #9).
- **IC-9700** — ASSUMED, the middle of the six rates this guide
  prints; the guide itself defers the factory setting to the
  instruction manual, which this project does not hold
  (`core/driver/ic9700/doc.go:141-144`).
- **IC-905** — ASSUMED (the default) over a CHOICE (the five-rate
  list it is chosen from is the wider CI-V family's conventional set,
  not a claim about what this radio itself accepts): this radio's
  guide prints no rate figure anywhere
  (`core/driver/ic905/doc.go:141-163`).

What is specific to one or two models:

- **IC-705 default discovery finds CALL only.** This radio's MEM bank
  is sparse (a 100-group × 100-channel space), and its slots are only
  known once a walk has visited them. `Connect Demo`, and any Open
  against a radio with nothing seeded, therefore surfaces the four
  fixed CALL channels and zero memories on a whole-radio read
  (`core/driver/ic705/inventory.go:99-103`;
  `internal/fakeic705/fakeic705.go:53-61`; `internal/wiring/fake.go:463-465`).
  The default walk covers only display groups G01-G10 (1,000 CI-V
  exchanges); `WithFullInventoryWalk()` widens it to the whole space
  (10,000 exchanges), but that option has no `rigprog` flag and no GUI
  control — it can only be reached by code that imports the driver
  package directly (`core/driver/ic705/inventory.go:12-16`;
  `core/driver/ic705/ic705.go:52-65`).
- **IC-705's CALL-group channel cap is narrower in practice than in
  the protocol layer.** This radio only documents CALL-group channels
  0-3, but the underlying CI-V layer would technically admit 4-99 too
  (ruling O-9, deliberately deferred). Nothing reaches those extra
  channels in this build: this driver's own slot parser refuses every
  one of them before any wire traffic is sent
  (`core/driver/ic705/doc.go:158-184`).
- **IC-9700's duplex offset scale is an open question (Erratum 14),
  and this project's own write-then-read-back check cannot catch a
  wrong answer.** Two independent readings of the offset field
  disagree by a factor of ten, and the disagreement is unresolved —
  get it wrong and every offset this driver reads or writes is out by
  ×10. Because the encoder and decoder consult the same scale
  constant, a write followed by a read-back is internally consistent
  either way; only a hardware capture against a known physical offset
  can settle which reading is right
  (`core/driver/ic9700/doc.go:308-317`; `core/civ/ic9700/profile.go:293,312`).
- **IC-905's demo radio starts empty by design.** The underlying fake
  rig's own factory-shaped default is ten occupied channels holding an
  all-zero record this driver's own filter refuses to decode, so
  `Connect Demo` deliberately empties all ten before opening rather
  than ship a demo whose first read fails
  (`internal/wiring/fake.go:471-500`).
- **IC-905's default Open discovers its MEM bank by a bounded walk**
  — group 0 in full, then channel 00 of every other group, descending
  into the rest of a group only where its channel 00 answered — not
  the whole 100×100 space, and there is no setting that widens it. A
  channel stored outside that walk is simply not listed; its absence
  from the grid is not evidence the channel is empty
  (`internal/radiotext/radiotext.go:1111`).
- **A non-octal DTCS code (IC-705 and IC-905) reads back as Unknown,
  not as a wrong number, and the same check blocks writing one until
  it is corrected.** Both radios' printed DTCS range is octal (digits
  0-7 only); a decoded value with an 8 or a 9 in it is a real nibble
  pattern but not a DTCS code this project's vocabulary recognises
  (`core/driver/ic705/read.go:160-164`; `core/driver/ic705/write.go:115`;
  `core/driver/ic905/read.go:189-210`; `core/driver/ic905/write.go:458-470`).

## What it does

- Reads the whole memory surface into a **codeplug file** you can
  keep, diff, and edit. On the Yaesu models this is the 99 regular
  memories, the 9 PMS (programmable memory scan) pairs, and on radios
  that have them the fixed 60 m and emergency channels, all over CAT.
  Each Icom model has its own CI-V memory-slot shape instead (see the
  *Icom models* section above for what "the whole memory surface"
  means, and what it costs to discover, on each one).
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
resolves the WebKitGTK and GTK libraries it needs if they are missing
— on a stock Ubuntu 24.04 desktop both were already there, so the
install pulled nothing extra; a minimal or server image, where apt
would have to fetch them, has not been tried. Those libraries exist
from Ubuntu 22.04 and Debian 12 onwards (and the Mint releases built
from those); the package itself is built and checked on Ubuntu 24.04,
and installs on other releases have not yet been tried. On other
distributions, take the static CLI tarball or build the GUI from
source (see below).

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
instructions have not yet been verified against real hardware, though
the packaged install itself has been exercised on Ubuntu 24.04 virtual
machines — see that document's Status section.)

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
| `core/` | The library: CAT codec (`cat`, plus `cat/ftdx10`, `cat/ftdx101`) for the Yaesu models, CI-V codec (`civ`, plus `civ/ic7610`, `civ/ic7300`, `civ/ic7300mk2`, `civ/ic705`, `civ/ic9700`, `civ/ic905`) for the Icom models, capability model (`spec`), codeplug model and diff (`codeplug`), CSV I/O (`csvio`), serial transport (`transport`), radio drivers (`driver/ft710`, `driver/ftdx10`, `driver/ftdx101` for Yaesu; `driver/ic7610`, `driver/ic7300`, `driver/ic7300mk2`, `driver/ic705`, `driver/ic9700`, `driver/ic905` for Icom), and the safe send choreography (`clone`). |
| `cmd/rigprog/` | The CLI. |
| `app/` | Wails v2 + Svelte desktop GUI. |
| `internal/` | The radio simulators — `fakeradio`, `fakedx10`, `fakedx101` for Yaesu; `fakeic7610`, `fakeic7300`, `fakeic7300mk2`, `fakeic705`, `fakeic9700`, `fakeic905` for Icom — composition-root wiring, the shared settings store the CLI and GUI both use for unverified-write consent (`userconfig`), menu-table generator (`extable`), and the import-graph guard tests (`guards`). |
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

CAT protocol facts for the Yaesu models are derived from Yaesu's
published CAT Operation Reference Manuals: **FT-710 (2306-C)**,
**FTdx10 (2308-F)**, and **FTdx101D/MP (2308-L)**.

CI-V protocol facts for the Icom models are derived from Icom's
published CI-V Reference Guides: **IC-7610 (rev 4, Sep 2025)**,
**IC-7300 (Full Manual §19, no standalone CI-V guide, rev 12b, Aug
2024)**, **IC-7300MK2 (rev 0, Oct 2025)**, **IC-705 (rev 6, Jan
2023)**, **IC-9700 (rev 4, Mar 2023)**, and **IC-905 (rev 2, May
2024)**.

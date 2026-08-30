<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<!--
  Template for .github/workflows/release.yml's "Write release notes"
  step. The version placeholder below is replaced with the pushed tag
  by a sed pass before this becomes the release body — so do not write
  that placeholder token in this comment, or it gets substituted here
  too and the sentence stops making sense. Keep this file in sync with
  README.md's "Install" and "First use" sections and
  docs/hardware-notes.md / docs/linux-setup.md /
  docs/menu-write-decision.md, which it cites rather than restates.

  SYNCED WITH THE PUBLISHED v1.0.0 RELEASE (09/08/2026): the v1.0.0
  Release body was authored from this template and this file was then
  updated to match it — four registered models with the per-model
  support table; the old "Status of this draft" publish-gate section
  replaced by the honest evidence-status section (the Linux real-radio
  session has still not run and the notes say so plainly).

  SYNC 22/08/2026: the Linux GUI now ships as a Debian package, so the
  Downloads table carries an amd64 and an arm64 deb row, built by
  release.yml's gui-linux job. The consent prose was corrected — writes
  to the three FTdx models are opt-in, not disarmed, and have been
  since the unverified-writes consent work landed; the wording now
  follows README.md's "Switching on writes for an unverified radio" section. Linux hardware
  evidence is still pending: the evidence-status section says so, and
  must keep saying so until a real-radio Linux session has run.

  SYNC 26/08/2026: the packaged binaries HAVE now been launch-tested.
  Rehearsal debs built by release.yml were installed on clean Ubuntu
  24.04.4 desktop VMs on both architectures (arm64 23/08/2026, amd64
  the same day) — desktop entry, Demo connection and version stamp on
  each; the full Demo edit/Send workflow on arm64 — so the
  evidence-status section no longer says they are untested. No radio
  was attached: real-radio Linux evidence is still pending, and that
  half of the bullet must keep saying so.
  The install-time dependency sentence in the Downloads section was
  also softened, because both GUI libraries were already present on
  the stock desktop image and the VMs never made apt fetch them.

  UPDATED AT THE ICOM TIER'S CLOSE (28/08/2026): the per-model support
  table now carries ten registered models (the four from v1.0.0 plus
  IC-7610/IC-7300/IC-7300MK2/IC-705/IC-9700/IC-905), and the
  Yaesu-specific "99 regular memories plus the 9 PMS pairs" description
  and per-model menu-address counts in "What it does" are scoped to the
  Yaesu models only — the Icom tier has no menu-settings feature and a
  different, per-model memory-slot shape. The consent prose the
  22/08/2026 sync corrected now covers all nine manual-derived models
  rather than the three FTdx ones alone, and the two Icom models that
  discover their memories by a bounded walk carry a footnote under the
  table. The AppImage notes the v1.0.0 sync above used to carry are
  gone with that sync's rewrite: the Debian packages took the Linux
  GUI row, so there is no AppImage decision left to restore.

  UPDATED AT THE FIRST ADDITIONS-TIER REGISTRATION (30/08/2026): the
  per-model support table now carries twelve registered models — the
  ten above plus the IC-7851 and the IC-7850, which are TWO entries
  over ONE radio manual, one CI-V address and one memory format,
  because this project cannot tell the two apart. Every "nine
  manual-derived models" count became ELEVEN with them. The Evidence
  section's source sentence also stopped saying "CI-V Reference
  Guides" alone: this pair has no separate CI-V guide published, and
  its protocol facts come from section 18 of the radio's own
  instruction manual.

  UPDATED AGAIN AT THE SECOND ADDITIONS-TIER REGISTRATION
  (30/08/2026): the table carries THIRTEEN registered models — the
  twelve above plus the IC-7760, a single entry with its own CI-V
  Reference Guide (revision 2, A7788-8EX-2). Every "eleven
  manual-derived models" count became TWELVE with it.

  UPDATED AGAIN AT THE THIRD ADDITIONS-TIER REGISTRATION
  (30/08/2026): FOURTEEN registered models — the thirteen above plus
  the IC-7100, whose protocol facts come from section 20 of the
  radio's own full manual (A7085-2EX-5), there being no separate CI-V
  guide published for it. Every "twelve manual-derived models" count
  became THIRTEEN with it. Its row carries a limitation none of the
  others do: ten of the radio's channels are deliberately NOT read.

  UPDATED AGAIN AT THE FOURTH AND LAST ADDITIONS-TIER REGISTRATION
  (30/08/2026): FIFTEEN registered models — the fourteen above plus
  the IC-R8600, whose protocol facts come from its own CI-V Reference
  Guide (revision 3a, A7375-2EX-3a). Every "thirteen manual-derived
  models" count became FOURTEEN with it. It is the first RECEIVER in
  the table, so the "Yaesu and Icom models" phrasing in the opening
  paragraph now reads "models" rather than "transceivers", and the
  bounded-walk footnote covers three models rather than two.
-->

Open Rig Programmer __VERSION__ — an open-source, cross-platform
memory-channel programmer for the Yaesu FT-710, built as a free
alternative to RT Systems' YPS-FT710, with fourteen further Yaesu and
Icom models — thirteen transceivers and one receiver — registered for
reading and for opt-in writes (see below).

## What it does

- **Read every memory channel** into a codeplug file you can keep,
  diff and re-send. On the Yaesu models this is the 99 regular
  memories plus the 9 PMS (Programmable Memory Scan) pairs, over CAT;
  each Icom model has its own CI-V memory-slot shape instead (a
  group-addressed space, or CALL channels alongside it, per model —
  see the per-model honesty rows in docs/icom-models.md).
- **Edit channels** in a spreadsheet-style grid (GUI) with keyboard
  navigation, paste, per-column editors and drag copy/swap/move.
- **Send changes back safely** (FT-710): read-before-write, a snapshot
  of the radio's existing contents taken before anything changes, a
  reviewed diff you confirm against a digest, and per-channel verify
  after each write. Anything that cannot be written is shown with the
  reason rather than attempted.
- **CSV and CHIRP import/export**, with a report of anything a CHIRP
  file cannot express.
- **Read the radio's menu (EX) settings**, on the Yaesu models, into
  the same file and view or export them — every documented menu
  address for the connected model (FT-710: 296; FTdx10: 197;
  FTdx101D/MP: 193). None of the six Icom drivers expose a settings
  surface.
- Both a **desktop GUI** and a **`rigprog` CLI**, sharing one core.

## Supported radios, and what "supported" means for each

| Model | Read channels | Write channels | Read menu settings | Evidence |
| --- | --- | --- | --- | --- |
| FT-710 | Yes | Yes | Yes | Proven against real hardware (`docs/hardware-notes.md`) |
| FTdx10 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | Yes | CAT manual + simulator only; no real radio has been connected |
| FTdx101D | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | Yes | CAT manual + simulator only; no real radio has been connected |
| FTdx101MP | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | Yes | CAT manual + simulator only; no real radio has been connected |
| IC-7610 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | No | CI-V guide + simulator only; no real radio has been connected |
| IC-7300 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | No | CI-V guide + simulator only; no real radio has been connected |
| IC-7300MK2 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | No | CI-V guide + simulator only; no real radio has been connected |
| IC-705 | Yes\* | ⚠️ **Opt-in** (unverified writes, off by default) | No | CI-V guide + simulator only; no real radio has been connected |
| IC-9700 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | No | CI-V guide + simulator only; no real radio has been connected |
| IC-905 | Yes\* | ⚠️ **Opt-in** (unverified writes, off by default) | No | CI-V guide + simulator only; no real radio has been connected |
| IC-7851 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | No | Instruction manual §18 + simulator only; no real radio has been connected |
| IC-7850 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | No | Instruction manual §18 + simulator only; no real radio has been connected. Shares the IC-7851's manual, address and memory format; this program cannot tell the two apart, so the model shown is the one you picked |
| IC-7760 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | No | CI-V guide + simulator only; no real radio has been connected. Control head's USB socket only — it presents two serial ports and the guide does not say which one answers |
| IC-7100 | Yes | ⚠️ **Opt-in** (unverified writes, off by default) | No | Full manual §20 + simulator only; no real radio has been connected. The 495 ordinary memories (banks A–E) only — the six programmed scan edges and four call channels are not read, the manual never printing the bank number that addresses them |
| IC-R8600 | Yes\* | ⚠️ **Opt-in** (unverified writes, off by default) | No | CI-V guide + simulator only; no real receiver has been connected. A RECEIVER: the grid has no transmit columns at all. Its capacity is not documented, so there is no total to show and no warning before it is full; its speed is assumed on both halves, the guide printing neither a default nor the list of speeds the menu offers (as with the IC-7760); and it cannot create a channel in an empty slot, the write being refused rather than a Select-group setting invented |

\* IC-705, IC-905 and IC-R8600 each discover their memory channels by a
BOUNDED default walk rather than a read of the whole address space (a
100x100-slot, group-addressed space on all three). A channel stored
outside that walk's range is simply not read; its absence is not evidence
the channel is empty. See docs/icom-models.md for each
model's exact bound.

Writes to the fourteen manual-derived models are refused until you enable
unverified writes for that radio, one radio at a time — `rigprog
settings unverified-writes <model> on` (say `rigprog settings
unverified-writes FTdx10 on`, or `rigprog settings unverified-writes
IC-7610 on`), or in the GUI the question it asks once just after you
first connect to such a radio, or its *Unverified writes…* button at
any time. Both faces read and write one settings file
(`rigprog/settings.json` under your user configuration directory), so
a decision made in either holds for both, and an "off" is stored
rather than forgotten. "Unverified" means documented in the
manufacturer's own CAT or CI-V reference and exercised against a
simulator here, not proven on a real radio. The FT-710 is unaffected:
its writes are hardware-verified, so it has nothing to consent to.
Consent changes what the tool is allowed to send, not how it sends it
— the read-before-write, the snapshot, the reviewed diff and the
per-channel verify all still run; README.md's *Switching on writes for an unverified radio*
section has the full mechanism. Each of those fourteen models has a
driver carrying a register of every assumption it makes and the
specific capture from a real radio that would verify it — if you own one of these radios and
want to help, open an issue.

## What it deliberately does not do

- **It does not write menu settings, and will not.** This is a settled
  design decision taken on evidence, not an unfinished feature — the
  FT-710 CAT manual's menu chart proved wrong in both of the two
  respects a read could check, and nothing in the read direction
  establishes what the radio accepts in the write direction. The
  reasoning, and what would have to change to revisit it, is in
  `docs/menu-write-decision.md`.
- **It cannot erase a channel over CAT.** The four Yaesu models have
  no CAT erase command at all; the app says so, and tells you the
  front-panel procedure, rather than silently doing nothing. The six
  Icom models are different: their CI-V references print a clear form
  for a memory channel, but this project deliberately ships no erase
  builder for any of them — spec D1 admits exactly three builders per
  driver (ID read, memory read, memory set), and a clear/erase frame
  is not one of them (`core/civ/doc.go:64`).
- **It does not read per-channel CTCSS tone frequencies.** The FT-710
  does not report them over CAT (established against real hardware —
  `docs/hardware-notes.md`), so the app preserves whatever is on the
  radio instead of guessing.

## Downloads

| Platform | What it is | File |
| --- | --- | --- |
| macOS (Intel + Apple Silicon, universal) | GUI (.app, zipped) | `open-rig-programmer-__VERSION__-darwin-universal.app.zip` |
| macOS (Intel + Apple Silicon, universal) | CLI | `rigprog-__VERSION__-darwin-universal.tar.gz` |
| Linux amd64 | CLI | `rigprog-__VERSION__-linux-amd64.tar.gz` |
| Linux arm64 | CLI | `rigprog-__VERSION__-linux-arm64.tar.gz` |
| Linux amd64 (Debian/Ubuntu/Mint) | GUI + CLI (.deb) | `open-rig-programmer___VERSION_NO_V___amd64.deb` |
| Linux arm64 (Debian/Ubuntu/Mint) | GUI + CLI (.deb) | `open-rig-programmer___VERSION_NO_V___arm64.deb` |

Either Debian package installs the GUI, the `rigprog` CLI, a desktop
entry and the ModemManager udev rule; `sudo apt install ./<file>`
resolves `libwebkit2gtk-4.1-0` and GTK 3 if they are missing. On a
stock Ubuntu 24.04 desktop both were already installed, so the install
fetched nothing extra; a minimal or server image, which would have to
pull the WebKit runtime in, has not been tried. That WebKit runtime
package exists on Ubuntu 22.04 and later, on Debian 12 and later, and
on the Mint releases built from those — not on anything older. On
other distributions, take the CLI tarball or build the GUI from source
(`wails build -tags webkit2_41` in `app/`); either way,
`docs/linux-setup.md` covers the serial-port setup.

`SHA256SUMS` (attached below) covers every file above. Verify with:

```sh
sha256sum -c SHA256SUMS --ignore-missing
```

## Checking which version you have

`rigprog version` prints it; the GUI shows it at the right-hand end of
the status bar. Quote that string in any bug report. A build that
reports `dev (unreleased build)` did not come from this release page —
if you downloaded it here, please say so in the report, because that
would be a packaging fault.

## Firmware requirement

Memory CAT (read and write) on the FT-710 requires firmware **V01-10
or later**. There is no CAT query for the firmware version — check the
radio's front panel or SD-card version screen before connecting.

## macOS: first launch

The .app is only ad-hoc signed (no Apple Developer ID), so Gatekeeper
will refuse to open it the ordinary way the first time. In Finder,
right-click (Control-click) the app and choose **Open**, then confirm
in the dialogue that appears. This is only needed once; after that it
opens normally.

## Linux: serial port access

- Add yourself to the `dialout` group and log out/in (or `newgrp
  dialout`) before either the GUI or the CLI can open the radio's
  serial port: `sudo usermod -aG dialout "$USER"`.
- ModemManager can probe a newly-plugged serial adapter and interfere
  with it; excluding the radio's USB-serial bridge from ModemManager
  via udev is recommended. The Debian packages install that rule for
  you; for the CLI tarball or a build from source, create it by hand —
  `docs/linux-setup.md` has the full instructions and a ready-to-use
  rule.

## What the hardware evidence covers — read this

Every "works on the radio" claim in this project comes from recorded
sessions against **one physical UK FT-710, on macOS**, and
`docs/hardware-notes.md` is the record. Channel reads and writes were
proven on that radio; the menu-settings read was proven separately.
One radio, one region, one firmware version.

What this release has **not** been exercised against:

- **Linux with a real radio.** The Linux CLI binaries are
  cross-compiled (the version stamp is asserted on the amd64 binary,
  which the build runner can execute; arm64 takes the same ldflags),
  and the serial stack is the same code on every platform. The Linux
  GUI is built from the same source on every push to main and every
  pull request, and launched under Xvfb on an amd64 CI runner to
  prove it starts; the release build packages it into the
  Debian packages above, whose contents the same release build then
  checks. The packaged GUI and CLI have since been installed from the
  Debian packages this project's release workflow built at the
  v1.1.0-rc.1 rehearsal tag, onto clean Ubuntu 24.04 desktop virtual
  machines (23/08/2026) on **both** architectures, and
  launched there — GUI started from its installed desktop entry,
  connected to the built-in Demo radio, and the version stamp read
  back from the running binaries on each. What none of that
  involved is a radio: **no real-radio session has been run on Linux**,
  so which serial node is the CAT port, and whether the packaged udev
  rule keeps ModemManager off it, are both unconfirmed.
  `docs/linux-setup.md` carries the port-setup instructions and the
  full status; treat the first Linux session as exploratory and
  read-only first.
- **Any FTdx10, FTdx101D or FTdx101MP.** Everything about those three
  models is derived from the manufacturer's CAT reference manuals
  through a documented transcription-and-cross-check process, and
  exercised against simulators built independently from the same
  manuals. No real radio of any of the three has ever been connected.
  That is why their writes are opt-in rather than on by default,
  needing your explicit consent.
- **Any IC-7610, IC-7300, IC-7300MK2, IC-705, IC-9700, IC-905,
  IC-7851, IC-7850, IC-7760, IC-7100 or IC-R8600.** Everything about
  these eleven models is derived from Icom's own published documentation
  through the same kind of documented process, and exercised against
  simulators built independently from it — the CI-V Reference Guide for
  each of the first six and for the IC-7760 and the IC-R8600, and, where
  no separate CI-V guide is published, a chapter of the radio's own
  manual (section 18 for the IC-7851 and IC-7850, section 20 for the
  IC-7100). No real radio of any of the eleven has ever been connected.
  That is why their writes need your explicit opt-in consent too.

  The IC-7851 and the IC-7850 are listed separately but cannot be told
  apart by this program: they share one manual, one CI-V address and
  one memory format, and the identity command's reply value is printed
  nowhere for either. The model reported is the one you selected.

  The IC-7760 is a two-box radio and only its control head's USB
  connection is supported. That socket presents two serial ports, and
  which of them carries CI-V is a setting on the radio for which the
  guide prints no default — if the first is silent, try the other. The
  guide prints no CI-V speed anywhere either, so 19200 is a guess; a
  wrong one costs a timeout, never a wrong byte.

  The IC-7100 lists its 495 ordinary memories — banks A to E — and
  nothing else. The radio's six programmed scan edges and four call
  channels are real channels, but the manual never says what bank
  number addresses them, so this build refuses them rather than
  guessing an address and reading the wrong thing; their absence from
  the list is not evidence the radio has none. It is also the only Icom
  model here opened with two stop bits rather than one, its manual
  stating no serial format for the CI-V link at all.

  The IC-R8600 is a **receiver**, the first in this table, and the grid
  reflects that rather than merely disabling columns: it has no
  transmit-frequency and no transmit-tone columns, because the radio has
  no transmitter and its memory record carries no such bytes. Three of
  its facts are absences in the guide rather than readings of it. Its
  capacity is never stated, so there is no total to show and this build
  cannot warn you before the receiver is full — what it does when full is
  unknown. Its speed is assumed on both halves, which it ties with the
  IC-7760 for: the guide prints no factory default, mentions no automatic
  setting, and never lists the speeds the menu offers, so both the 19200
  opening rate and the list it came from are assumed. And neither the transceive
  setting nor the echo-back setting of either USB port has a printed
  default, so this build cannot say whether unsolicited frames should be
  expected; any that arrive are counted and ignored. Tone squelch is read
  and written on FM channels only; a D-STAR, P25, NXDN, DCR or dPMR
  channel whose digital-squelch bytes differ from what this build assumes
  cannot be written back, and switching a channel into one of those modes
  is refused rather than invented. It also cannot CREATE a channel in an
  empty slot at all: the record's Select-group setting has no honest
  default to write, so the write path refuses rather than inventing one —
  the same refusal the IC-7300s make, for the same reason.

Anything the project has not observed is labelled as such in the code
and documentation rather than assumed. Reports from real hardware —
especially the fourteen manual-derived, opt-in-write models, and the
FT-710 on Linux — are the most valuable contribution this project can
receive right now.

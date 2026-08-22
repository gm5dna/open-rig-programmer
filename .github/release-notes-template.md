<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<!--
  Template for .github/workflows/release.yml's "Write release notes"
  step. The version placeholder below is replaced with the pushed tag
  by a sed pass before this becomes the release body — so do not write
  that placeholder token in this comment, or it gets substituted here
  too and the sentence stops making sense. Keep this file in sync with
  README.md's quick-start section and docs/hardware-notes.md /
  docs/linux-setup.md / docs/menu-write-decision.md, which it cites
  rather than restates.

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
  follows README.md's "Unverified writes" section. Linux hardware
  evidence is still pending: the evidence-status section says so, and
  must keep saying so until a real-radio Linux session has run.
-->

Open Rig Programmer __VERSION__ — an open-source, cross-platform
memory-channel programmer for the Yaesu FT-710, built as a free
alternative to RT Systems' YPS-FT710, with three further Yaesu models
registered for reading and for opt-in writes.

## What it does

- **Read every memory channel** from the radio over CAT — the 99
  regular memories plus the 9 PMS (Programmable Memory Scan) pairs —
  into a codeplug file you can keep, diff and re-send.
- **Edit channels** in a spreadsheet-style grid (GUI) with keyboard
  navigation, paste, per-column editors and drag copy/swap/move.
- **Send changes back safely** (FT-710): read-before-write, a snapshot
  of the radio's existing contents taken before anything changes, a
  reviewed diff you confirm against a digest, and per-channel verify
  after each write. Anything that cannot be written is shown with the
  reason rather than attempted.
- **CSV and CHIRP import/export**, with a report of anything a CHIRP
  file cannot express.
- **Read the radio's menu (EX) settings** into the same file and view
  or export them — every documented menu address for the connected
  model (FT-710: 296; FTdx10: 197; FTdx101D/MP: 193).
- Both a **desktop GUI** and a **`rigprog` CLI**, sharing one core.

## Supported radios, and what "supported" means for each

| Model | Read channels | Write channels | Read menu settings | Evidence |
| --- | --- | --- | --- | --- |
| FT-710 | Yes | Yes | Yes | Proven against real hardware (`docs/hardware-notes.md`) |
| FTdx10 | Yes | **Opt-in** (unverified writes, off by default) | Yes | CAT manual + simulator only; no real radio has been connected |
| FTdx101D | Yes | **Opt-in** (unverified writes, off by default) | Yes | CAT manual + simulator only; no real radio has been connected |
| FTdx101MP | Yes | **Opt-in** (unverified writes, off by default) | Yes | CAT manual + simulator only; no real radio has been connected |

Writes to the three manual-derived models are refused until you enable
unverified writes for that radio, one radio at a time — `rigprog
settings unverified-writes FTdx10 on`, or in the GUI the question it
asks once just after you first connect to such a radio, or its
*Unverified writes…* button at any time. Both faces read and write one
settings file (`rigprog/settings.json` under your user configuration
directory), so a decision made in either holds for both, and an "off"
is stored rather than forgotten. "Unverified" means documented in the
manufacturer's CAT reference and exercised against a simulator here,
not proven on a real radio. The FT-710 is unaffected: its writes are
hardware-verified, so it has nothing to consent to. Consent changes
what the tool is allowed to send, not how it sends it — the
read-before-write, the snapshot, the reviewed diff and the per-channel
verify all still run. Each of the three drivers carries a register of
every assumption it makes and the specific capture from a real radio
that would verify it — if you own one of these radios and want to
help, open an issue.

## What it deliberately does not do

- **It does not write menu settings, and will not.** This is a settled
  design decision taken on evidence, not an unfinished feature — the
  FT-710 CAT manual's menu chart proved wrong in both of the two
  respects a read could check, and nothing in the read direction
  establishes what the radio accepts in the write direction. The
  reasoning, and what would have to change to revisit it, is in
  `docs/menu-write-decision.md`.
- **It cannot erase a channel over CAT.** These radios have no CAT
  erase command; the app says so, and tells you the front-panel
  procedure, rather than silently doing nothing.
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
pulls in `libwebkit2gtk-4.1-0` and GTK 3 for you. That WebKit runtime
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
  Debian packages above, whose contents CI then checks. The packaged
  binaries themselves have not yet been launch-tested, on either
  architecture, and no real-radio session has been run on Linux at
  all. `docs/linux-setup.md` carries the port-setup instructions;
  treat the first Linux session as exploratory and read-only first.
- **Any FTdx10, FTdx101D or FTdx101MP.** Everything about those three
  models is derived from the manufacturer's CAT reference manuals
  through a documented transcription-and-cross-check process, and
  exercised against simulators built independently from the same
  manuals. No real radio of any of the three has ever been connected.
  That is why their writes are opt-in rather than on by default.

Anything the project has not observed is labelled as such in the code
and documentation rather than assumed. Reports from real hardware —
especially the three manual-derived models, and the FT-710 on Linux —
are the most valuable contribution this project can receive right now.

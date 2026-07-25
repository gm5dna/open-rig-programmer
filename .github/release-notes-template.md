<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<!--
  Template for .github/workflows/release.yml's "Write release notes"
  step. The version placeholder below is replaced with the pushed tag
  (e.g. v1.0.0) by a sed pass before this becomes the draft release's
  body — so do not write that placeholder token in this comment, or it
  gets substituted here too and the sentence stops making sense. Keep
  this file in sync with README.md's quick-start section and
  docs/hardware-notes.md / docs/linux-setup.md /
  docs/menu-write-decision.md, which it cites rather than restates.
-->

Open Rig Programmer __VERSION__ — an open-source, cross-platform
memory-channel programmer for the Yaesu FT-710, built as a free
alternative to RT Systems' YPS-FT710.

**This is a DRAFT release.** See "Status of this draft" at the bottom
before publishing it.

## What it does

- **Read every memory channel** from the radio over CAT — the 99
  regular memories plus the 9 PMS (Programmable Memory Scan) pairs —
  into a codeplug file you can keep, diff and re-send.
- **Edit channels** in a spreadsheet-style grid (GUI) with keyboard
  navigation, paste, per-column editors and drag copy/swap/move.
- **Send changes back safely**: read-before-write, a snapshot of the
  radio's existing contents taken before anything changes, a reviewed
  diff you confirm against a digest, and per-channel verify after each
  write. Anything that cannot be written is shown with the reason
  rather than attempted.
- **CSV and CHIRP import/export**, with a report of anything a CHIRP
  file cannot express.
- **Read the radio's menu (EX) settings** into the same file and view
  or export them — all 296 documented menu addresses.
- Both a **desktop GUI** and a **`rigprog` CLI**, sharing one core.

## What it deliberately does not do

- **It does not write menu settings, and will not.** This is a settled
  design decision taken on evidence, not an unfinished feature — the
  FT-710 CAT manual's menu chart proved wrong in both of the two
  respects a read could check, and nothing in the read direction
  establishes what the radio accepts in the write direction. The
  reasoning, and what would have to change to revisit it, is in
  `docs/menu-write-decision.md`.
- **It cannot erase a channel over CAT.** The FT-710 has no CAT erase
  command; the app says so, and tells you the front-panel procedure,
  rather than silently doing nothing.
- **It does not read per-channel CTCSS tone frequencies.** The radio
  does not report them over CAT (established against real hardware —
  `docs/hardware-notes.md`), so the app preserves whatever is on the
  radio instead of guessing.

## Downloads

| Platform | What it is | File |
| --- | --- | --- |
| macOS (Intel + Apple Silicon, universal) | GUI (.app, zipped) | `open-rig-programmer-__VERSION__-darwin-universal.app.zip` |
| macOS (Intel + Apple Silicon, universal) | CLI | `rigprog-__VERSION__-darwin-universal.tar.gz` |
| Linux amd64 | GUI (AppImage) | `open-rig-programmer-__VERSION__-linux-amd64.AppImage` |
| Linux amd64 | CLI | `rigprog-__VERSION__-linux-amd64.tar.gz` |
| Linux arm64 | CLI | `rigprog-__VERSION__-linux-arm64.tar.gz` |

`SHA256SUMS` (attached below) covers every file above. Verify with:

```sh
sha256sum -c SHA256SUMS --ignore-missing
```

## Checking which version you have

`rigprog version` prints it; the GUI shows it at the right-hand end of
the status bar. Quote that string in any bug report. A build that
reports `dev (unreleased build)` did not come from this release
pipeline — if you downloaded it from this page, please say so in the
report, because that would be a packaging fault.

## Firmware requirement

Memory CAT (read and write) requires FT-710 firmware **V01-10 or
later**. There is no CAT query for the firmware version — check the
radio's front panel or SD-card version screen before connecting.

## macOS: first launch

The .app is only ad-hoc signed (no Apple Developer ID), so Gatekeeper
will refuse to open it the ordinary way the first time. In Finder,
right-click (Control-click) the app and choose **Open**, then confirm
in the dialogue that appears. This is only needed once; after that it
opens normally.

## Linux: serial port access and AppImage runtime

- Add yourself to the `dialout` group and log out/in (or `newgrp
  dialout`) before the CLI or GUI can open the radio's serial port:
  `sudo usermod -aG dialout "$USER"`.
- ModemManager can probe a newly-plugged serial adapter and interfere
  with it; excluding the FT-710's USB-serial bridge from ModemManager
  via udev is recommended. See `docs/linux-setup.md` for the full
  instructions and a ready-to-use udev rule.
- The AppImage needs the **webkit2gtk-4.1** runtime library installed
  on the host (the build-time dependency is `libwebkit2gtk-4.1-dev`;
  most current distributions ship the runtime package, commonly named
  `libwebkit2gtk-4.1-0`, separately from the `-dev` headers).

## What the hardware evidence covers

Every "works on the radio" claim in this project comes from a recorded
session against one physical UK FT-710 on macOS, and
`docs/hardware-notes.md` is the record. Channel reads and writes were
proven on real hardware; the menu-settings read was proven separately.
One radio, one region, one firmware version. Behaviour on another
radio, region or firmware is expected but not evidenced, and anything
the project has not observed is labelled as such rather than assumed.

## Status of this draft

This release is built from a tag, but Linux artefacts have **not yet
been exercised against real FT-710 hardware on Linux** — the project's
plan requires a Linux real-radio session (confirming port mapping,
`dialout` access, and an actual read/write) before Linux artefacts
ship publicly. See `docs/hardware-notes.md` (macOS-only hardware
findings so far) and `docs/linux-setup.md` ("pending" note) for the
current state. Do not publish this draft until that session has run
and this note has been updated to reflect it.

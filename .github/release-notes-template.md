<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
<!--
  Body of every GitHub release, written by release.yml's "Write release
  notes" step: a sed pass replaces the two version placeholders (the
  tag, and the tag without its leading "v" for the .deb file names)
  before this becomes the release text, so never write either token in
  this comment. Before tagging: paste the new version's CHANGELOG.md
  entry into "What changed in this version", and keep the Downloads
  table and the first-launch sections in step with README.md's
  "Install" and "First use". Links into the repository are pinned to
  the tag, so the release page keeps reading correctly after the docs
  move. The history of this file's edits is a private record
  (.superpowers/sdd/release-notes-sync-log.md, not in the repository).
  Keep every paragraph and list item on ONE line: GitHub renders a newline in a release body as a line break, so hard-wrapping shows as ragged lines on the release page.
-->

Open Rig Programmer __VERSION__: a free memory-channel programmer for Yaesu, Icom and Kenwood radios. Read the memories into a file, edit them in a grid or a spreadsheet, send them back over the radio's ordinary USB cable. Desktop app and `rigprog` command line for macOS, Windows and Linux.

Which radios are supported, and how far each has been tested, is in [docs/radio-notes.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/radio-notes.md). Only the FT-710 has ever been connected to this program; every other radio's writes stay switched off until you switch them on for that radio, as the [README](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/README.md) describes under *Switching on writes for an unverified radio*.

## What changed in this version

A minor release: a new radio, a new bank on an existing one, and a vocabulary clean-up across the whole Yaesu fleet.

- **FTX-1** joins the supported Yaesu models, paper-only and opt-in for writes via the unverified-write consent gate — no FTX-1 has ever answered a frame from this project. Reads and writes memory and PMS channels on a widened version of the FT-710's own MR/MW record: 5-digit addresses (`00001`-`00999`), 50 PMS pairs as a hyphenated two-digit token (`P-01L`-`P-50U`), a 5 MHz band (`50001`-`50020`) and a named `EMGCH` channel, both banks read-only even under consent. Tags carry no display byte, so this build shows no Tag Display column for them. Tone widens to six states, adding `PR FREQ` and `REV TONE` to the fleet's vocabulary; no tone-frequency chart exists in the manual, so only the six-state selector is read and written. `EX`, `GT` and `VM` are not implemented; `MC` goes unused — reads go via `MR`/`MT` only, since `MC` carries a per-port byte this project's CAT codec cannot build or parse. This build cannot tell an FTX-1 "Field" body from an "Optima" body: both share CAT identity `0840` and nothing in the manual gives CAT a way to ask, so the body is never inferred. The manual requires main firmware V1.08 or later for CAT to work at all; this build cannot check it — the wire carries no version byte — so a pre-V1.08 radio simply never answers. Further ASSUMED values (channel range past 99, which of three printed 5 MHz ranges is real, MW's P7 enum, TagFill, the clarifier's step size, and more) are listed with their own probe list in [docs/radio-notes.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/radio-notes.md).
- **TS-2000, TS-2000X and B2000** gain a Satellite Memory bank (`SA`/`SI`, 10 channels): a name plus three per-channel flags — band swap, trace and trace-reverse. The record carries no frequency; satellite operation reads that live off `FA`/`FB` instead, and a read returns only the currently selected channel, a protocol limit rather than a bug. Unverified, consent-gated, on the same terms as the ordinary memory bank.
- Internal: Yaesu's separate `ToneSemantics`/`CTCSSStates` vocabulary is gone. Every Yaesu driver now publishes the same `spec.ToneMode` list Icom and Kenwood already used, plus two new DCS members; byte-identical for every existing dialect. The unification caught two latent bugs: a Yaesu-detection check (in the codeplug validator and the app's UI spec) was keying off `CTCSSStates`, and the shared write-gate was feeding the now-shared `ToneModes` list into a tier check meant for Icom only. Both fixed.
- Internal: `core/cat` gained four axes for FTX-1's own record shapes — 5-digit addresses, the dash-token PMS form, a tag form with no display byte, and an `MC`-unsupported gate. That gate closed a pre-existing hole: a fixed `MC;` read frame was previously admitted regardless of policy. `dialecttest` was taught all four axes, so every existing dialect stays exercised (byte-identical for every registered three-digit dialect). Three new `spec.Field` constants for the satellite flags ship fleet-wide — every radio other than TS-2000/2000X/B2000 answers Unavailable for them.
- **FTdx3000 AM-N storability** closed on paper: the CAT manual's own MW/MR memory-record mode legend prints only A/B/C, never D, so AM-N has no byte position in a stored channel at all and no real radio could store it regardless of what this driver writes.
- Settings write stays parked for this milestone, still on the roadmap. Supported models: 45 → 46.

## Downloads

| Platform | What it is | File |
| --- | --- | --- |
| macOS (Intel + Apple Silicon, universal) | App (.app, zipped) | `open-rig-programmer-__VERSION__-darwin-universal.app.zip` |
| macOS (Intel + Apple Silicon, universal) | Command line | `rigprog-__VERSION__-darwin-universal.tar.gz` |
| Windows amd64 | App + command line (installer) | `open-rig-programmer-__VERSION__-windows-amd64-installer.exe` |
| Windows arm64 | App + command line (installer) | `open-rig-programmer-__VERSION__-windows-arm64-installer.exe` |
| Windows amd64 | Command line only (zip) | `rigprog-__VERSION__-windows-amd64.zip` |
| Windows arm64 | Command line only (zip) | `rigprog-__VERSION__-windows-arm64.zip` |
| Linux amd64 (Debian, Ubuntu, Mint) | App + command line (.deb) | `open-rig-programmer___VERSION_NO_V___amd64.deb` |
| Linux arm64 (Debian, Ubuntu, Mint) | App + command line (.deb) | `open-rig-programmer___VERSION_NO_V___arm64.deb` |
| Linux x86_64 (Fedora, openSUSE) | App + command line (.rpm) | `open-rig-programmer-__VERSION_NO_V__.x86_64.rpm` |
| Linux aarch64 (Fedora, openSUSE) | App + command line (.rpm) | `open-rig-programmer-__VERSION_NO_V__.aarch64.rpm` |
| Linux x86_64 (any distribution with Flatpak) | App (Flatpak bundle) | `open-rig-programmer-__VERSION__-x86_64.flatpak` |
| Linux aarch64 (any distribution with Flatpak) | App (Flatpak bundle) | `open-rig-programmer-__VERSION__-aarch64.flatpak` |
| Linux amd64 | Command line | `rigprog-__VERSION__-linux-amd64.tar.gz` |
| Linux arm64 | Command line | `rigprog-__VERSION__-linux-arm64.tar.gz` |

The Windows installers are native-only: the amd64 installer refuses to run on an ARM64 PC even though ARM64 Windows can emulate x64 programs. The Debian package installs the app, the command line, a desktop entry and the ModemManager udev rule; `sudo apt install ./<file>` resolves its GTK and WebKit dependencies (built and tested on Ubuntu 24.04; Ubuntu 22.04, Debian 12 and the Mint releases built from them carry the same packages but have not been tried). The RPM installs the same set (`sudo dnf install ./<file>.rpm`); it is built and its metadata checked in CI but has not been installed on a real Fedora or openSUSE machine. On other distributions install the Flatpak bundle (`flatpak install --user ./<file>.flatpak`, x86_64 or aarch64; needs the GNOME 47 runtime from Flathub, and the app is granted `--device=all` so it can reach the radio's serial port), or take the command-line tarball, a single static binary.

`SHA256SUMS` covers every file above. Verify with:

```sh
sha256sum -c SHA256SUMS --ignore-missing     # Linux
shasum -a 256 -c SHA256SUMS --ignore-missing # macOS
certutil -hashfile <file> SHA256             # Windows, one file at a time
```

## First launch

- **macOS**: the app is signed with a Developer ID and notarised by Apple, so it opens like any other download.
- **Windows**: the installer is unsigned. Edge may flag the download, and SmartScreen shows *Windows protected your PC* the first time: click *More info*, then *Run anyway*. The app needs Microsoft's WebView2 runtime, which Windows 11 already ships; if it is missing the installer is set to download it, a path this project has not tried. On ARM64 install Silicon Labs' CP210x driver by hand before connecting; the radio shows as two COM ports and only one answers. See [docs/windows-setup.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/windows-setup.md).
- **Linux**: join the `dialout` group and log out and in; keep ModemManager off the radio (the .deb installs the rule; the tarball needs it by hand). See [docs/linux-setup.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/linux-setup.md).

**FT-710 firmware**: memory programming needs V01-10 or later. There is no CAT query for the version; check the radio's front panel.

**Which version you have**: `rigprog version` prints it (on Windows, `.\rigprog.exe version` from the folder that holds it; the installer adds nothing to PATH). The app shows it at the right-hand end of the status bar. Quote it in any bug report; a build reporting `dev (unreleased build)` did not come from this page.

## What the hardware evidence covers

Every "works on the radio" claim comes from recorded sessions against **one UK FT-710**, on macOS and, since 05/09/2026, on a Windows 11 ARM64 virtual machine; [docs/hardware-notes.md](https://github.com/gm5dna/open-rig-programmer/blob/__VERSION__/docs/hardware-notes.md) is the record. Not yet exercised against a real radio: **Linux** (the packages have been installed and launched on Ubuntu 24.04 virtual machines, never with a radio attached, so which serial node is the CAT port and whether the udev rule keeps ModemManager off it are unconfirmed); **Windows on amd64**, or any physical Windows PC; and **every radio other than the FT-710**, all of which come from the makers' manuals and simulators built from them, which is why their writes are opt-in. Each of those drivers carries a register of every assumption it makes and the capture from a real radio that would settle it. If you own one of these radios, a read of a single channel is the most valuable thing you could send: [open an issue](https://github.com/gm5dna/open-rig-programmer/issues/new/choose).

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

A patch release: no functional change to the app. Flathub-ready AppStream metainfo (with screenshots and release history) and a source-build Flathub manifest.

- **Flathub-ready metainfo**: `app/build/flatpak/io.github.gm5dna.open-rig-programmer.metainfo.xml` now carries screenshots, a release history and full AppStream metadata for a Flathub submission, with three screenshots of the real app committed alongside it.
- **Source-build Flathub manifest**: `flathub/io.github.gm5dna.open-rig-programmer.yml` builds the app from source (Go 1.25.0 and Node 22 SDK extensions, no vendored binaries) with generated `go-sources.json`/`node-sources.json`, plus a `.github/workflows/flathub.yml` CI job that builds and lints the manifest, appstream and repo on every push touching `flathub/`.
- No functional change to the app itself.

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

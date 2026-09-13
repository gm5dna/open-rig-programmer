<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# Flatpak bundle

Single-file `.flatpak` for Linux, built by `release.yml`'s
`gui-linux-flatpak` job, attached to the GitHub release alongside the
`.deb`. A **bundle, not a Flathub submission** — no remote, no
Flathub updates; submission is deferred (roadmap).

Install:
```
flatpak install --user ./open-rig-programmer-<version>-x86_64.flatpak
flatpak run io.github.gm5dna.open-rig-programmer
```

Serial ports: sandboxed with `--device=all`, every `/dev/tty*` node
visible, same as outside Flatpak; per-device udev narrowing deferred.
The ModemManager-ignore udev rule (`docs/linux-setup.md`) is a
`.deb`/tarball concern, not installed by this bundle.

Desktop file and icon are reused from `app/build/linux/`, not duplicated.

<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Linux packaging assets

These files are packaged verbatim into the Linux `.deb` by
`release.yml`'s `gui-linux` job, via `nfpm.yaml`. `check-deb.sh`
asserts the resulting package is what this directory promises —
identity, dependencies, contents, and maintainer scripts. The udev
rule's manual twin, for users installing without the package, lives in
`docs/linux-setup.md`.

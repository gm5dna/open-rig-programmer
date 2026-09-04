# Build Directory

The build directory is used to house all the build files and assets for your application. 

The structure is:

* bin - Output directory
* darwin - macOS specific files
* linux - Linux packaging assets (see linux/README.md)
* windows - Windows specific files

## Icon

`appicon.svg` is the one source of truth for the application icon. Everything
raster is derived from it by `scripts/render-icons.sh` (run from the repo root;
`--check` verifies the tree matches a fresh render) and committed alongside:

- `appicon.png` (1024×1024) — `wails build` derives the macOS `iconfile.icns` from it
- `windows/icon.ico` (256 → 16) — the Windows executable and installer icon
- `linux/open-rig-programmer-512.png` and `linux/open-rig-programmer.svg` — the
  hicolor entries the `.deb` installs (see `linux/nfpm.yaml`)

Do not hand-edit the rasters: change the SVG and re-run the script.

The mark (chosen 04/09/2026, roadmap Tier 0 P3): three memory rows with one
selected, in the GUI's own navy and amber. It names what the program does — a
list of channels you edit — and stays legible at 16 px, where the radio-shaped
alternatives (a VFO dial, an S-meter, a front panel) blurred. Original work
under the repository licence; no manufacturer marks.

## Mac

The `darwin` directory holds files specific to Mac builds.
These may be customised and used as part of the build. To return these files to the default state, simply delete them
and
build with `wails build`.

The directory contains the following files:

- `Info.plist` - the main plist file used for Mac builds. It is used when building using `wails build`.
- `Info.dev.plist` - same as the main plist file but used when building using `wails dev`.

## Windows

The `windows` directory contains the manifest and rc files used when building with `wails build`.
These may be customised for your application. To return these files to the default state, simply delete them and
build with `wails build`.

- `icon.ico` - The icon used for the application. This is used when building using `wails build`. If you wish to
  use a different icon, simply replace this file with your own. If it is missing, a new `icon.ico` file
  will be created using the `appicon.png` file in the build directory.
- `installer/*` - The files used to create the Windows installer. These are used when building using `wails build`.
- `info.json` - Application details used for Windows builds. The data here will be used by the Windows installer,
  as well as the application itself (right click the exe -> properties -> details)
- `wails.exe.manifest` - The main application manifest file.
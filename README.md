# open-rig-programmer

An open-source, cross-platform (macOS/Linux) memory-channel programmer
for Yaesu HF transceivers, supporting the Yaesu FT-710 today — a free
alternative to RT Systems' YPS-FT710. The core is now built on a
radio-neutral driver architecture, ready for further Yaesu models —
the FTdx10 next — as future work; no second radio driver exists yet.

## Status: pre-release, hardware write trials complete

> **Not yet a release.** Use entirely at your own risk (see Licence
> below), and expect rough edges until v1.
>
> The core is built and tested: the CAT protocol codec, serial
> transport, an independent radio simulator, a `rigprog` CLI, and a
> desktop GUI, all exercised end-to-end against the simulator and
> against a real FT-710.
>
> The real-hardware write path followed the safety design this project
> committed to from the start: it stayed **hard-disabled** until live
> hardware characterisation completed. That has now happened — M5a
> (read-only characterisation) and M5b (write trials against a
> sacrificial memory channel and a nominated empty/PMS pair, with
> per-write readback) both ran against a physical UK FT-710 on
> 13/07/2026 (see `docs/hardware-notes.md`) — and real-radio writes are
> enabled for exactly the fields those trials verified. The sacrificial
> channel itself was restored byte-identical to its baseline; the
> empty-slot-create and PMS trial artefacts were NOT — no CAT erase
> command exists (a confirmed finding, not a gap), so those channels
> remain populated pending manual front-panel cleanup, honestly recorded
> in `docs/hardware-notes.md` rather than glossed over. Every send still
> goes through the full safety choreography: a fresh baseline read, a
> snapshot saved first, a reviewed diff bound to an explicit
> confirmation, a first-use firmware-version confirmation, and
> per-channel write-then-verify with an append-only journal.
>
> Fields the radio cannot honour over CAT remain blocked honestly: no
> erase (the radio has no CAT erase command), no per-channel CTCSS tone
> or scan-skip writes (no CAT command exists), and clarifier *changes*
> are refused because the radio silently ignores them.

## Quick start

### Firmware requirement

Memory CAT (read and write) requires FT-710 firmware **V01-10 or
later**. There is no CAT query for the firmware version — check the
radio's front panel or SD-card version screen first.

### Install

Download from the [Releases page](../../releases):

- **macOS (GUI)**: `open-rig-programmer-<version>-darwin-universal.app.zip`
  — unzip, then **right-click the app and choose Open** the first time
  (the app is only ad-hoc signed, so Gatekeeper refuses a plain
  double-click until you do this once).
- **macOS (CLI)**: `rigprog-<version>-darwin-universal.tar.gz`.
- **Linux (CLI)**: `rigprog-<version>-linux-amd64.tar.gz` or
  `-linux-arm64.tar.gz` — a single static binary.
- **Linux (GUI)**: `open-rig-programmer-<version>-linux-amd64.AppImage`
  — needs the webkit2gtk-4.1 runtime library installed on the host
  (commonly packaged as `libwebkit2gtk-4.1-0`).

Or build the CLI from source (Go 1.25+): `go build ./cmd/rigprog`.
(Note the frontend-first build order under "Building and testing"
below if you want `go test ./...` or the GUI as well — `app/main.go`
embeds the frontend's build output.)

### Serial setup

- **macOS**: no driver installation needed. Both Apple's built-in
  driver and the SiLabs VCP driver work; the radio's CP2105 adapter
  exposes two UART device nodes per driver, of which the **Enhanced
  UART is the CAT port** — and it is the only one of the two that
  opens at all on macOS, so a CAT session cannot land on the wrong
  one (hardware-confirmed; see `docs/hardware-notes.md`, "Port
  mapping").
- **Linux**: add yourself to the `dialout` group
  (`sudo usermod -aG dialout "$USER"`, then log out/in) and exclude
  the radio's USB-serial adapter from ModemManager with a udev rule —
  full instructions, including a ready-to-use rule for the CP2105
  (`10C4:EA70`), in [docs/linux-setup.md](docs/linux-setup.md). Note
  the Linux instructions are not yet verified against real hardware
  (see that document's "Status" section).

### CLI tour

```sh
rigprog ports                                  # list candidate serial ports, ranked
rigprog probe --port /dev/cu.SLAB_USBtoUART    # confirm which port answers as an FT-710
rigprog read  --port ... --out radio.json      # read all memory slots into a codeplug file
rigprog diff  --port ... edited.json           # preview changes against a fresh read
rigprog write --port ... edited.json           # send the changes
```

- `ports` ranks every serial port the OS exposes by how likely it is
  to be the FT-710's CAT interface — a heuristic shortlist, not a
  verdict.
- `probe` performs the definitive check: it opens a session and asks
  the radio to identify itself.
- `read` reads every memory slot and saves a codeplug JSON file you
  can edit, export to CSV, or import into.
- `diff` is read-only: it takes a fresh baseline read and shows what
  your file would change, including anything blocked (erases, and
  fields the radio cannot accept over CAT).
- `write` runs the full safety choreography: a fresh baseline read, a
  snapshot of the radio's current contents saved before anything
  else, a reviewed diff bound to an explicit digest-matched
  confirmation, a first-write firmware confirmation, then
  per-channel write-and-verify with an append-only journal of
  exactly what happened.

Every radio-touching subcommand also accepts `--fake` instead of
`--port`, running against the in-process simulated radio — useful for
trying the whole flow without hardware.

`rigprog version` (or `rigprog -v`) reports which build you are running
— quote it in any bug report. The GUI shows the same string at the
right-hand end of its status bar. A build that says `dev (unreleased
build)` was not produced by the release pipeline.

`probe`, `read`, `diff`, `write`, `import`, and `settings` also accept
`--model NAME` to select which radio driver to target — currently just
`FT-710` (the default), the only driver built so far. The flag exists so
further Yaesu models can be added without changing the CLI surface; an
unrecognised `--model` refuses immediately, naming every model this
build supports.

### GUI

Connect (pick a port, or press **Demo (simulated radio)** to try
everything without hardware — the GUI equivalent of the CLI's
`--fake`), read the radio, edit channels in the grid (keyboard
navigation, paste, per-column editors, CSV/CHIRP import), then Send —
which walks the same safety flow as `rigprog write`: a reviewed diff
in a confirmation dialogue, blocked entries shown with reasons, and
per-channel write-and-verify with progress as it transfers.

## Planned features

- Channel grid covering all 99 regular memories plus the 9 PMS
  (Programmable Memory Scan) pairs.
- CSV import/export of the channel grid.
- Safe transfer to the radio: read-before-write, snapshot of the radio's
  existing memory contents before any change, diff-then-confirm before
  transmitting, and per-channel verify after write.
- Menu/settings **reading** already works: `rigprog read --settings`
  captures a snapshot of the radio's menu (EX) surface into the codeplug
  file, and `rigprog settings FILE` (or the GUI's settings view) renders
  it. That read path has been exercised against a real radio: on 24 July
  2026 all 296 documented menu addresses answered on a UK FT-710 running
  firmware V01-12, over two successive sweeps that matched each other
  exactly (`docs/hardware-notes.md`). One radio, one firmware version,
  one configuration, and the read direction only — that is the whole of
  the evidence.
- Menu/settings **writing** does not exist, is not implied by the read
  evidence above, and is not planned: no code path in this repository
  can send a menu-change frame, and the outbound allowlist rejects the
  entire EX Set/Answer wire shape. That is a deliberate decision taken
  on 25 July 2026, not an unfinished feature — the reasoning, and the
  conditions under which it would be worth revisiting, are recorded in
  `docs/menu-write-decision.md`.
- Further Yaesu radio models beyond the FT-710 — the core's driver
  architecture is already radio-neutral, with the FTdx10 planned next,
  but no second driver has been built yet.

## Repository layout

| Path | Contents |
| --- | --- |
| `core/` | The library: CAT protocol codec (`cat`), capability model (`spec`), codeplug model and diff (`codeplug`), CSV I/O (`csvio`), serial transport (`transport`), radio driver (`driver`, `driver/ft710`), and the safe send choreography (`clone`). |
| `cmd/rigprog/` | The `rigprog` CLI (ports, probe, read, write, diff, export, import). |
| `app/` | Wails v2 + Svelte desktop GUI. |
| `internal/` | The radio simulator (`fakeradio`), composition-root wiring, shared CSV-merge helpers, and the import-graph guard tests (`guards`). |
| `docs/` | Project documentation: hardware findings, Linux setup, and the fixture redaction policy. |
| `docs/fixtures-private/` | Git-ignored. Raw, unredacted radio backups/serial captures — never committed. |
| `.github/workflows/` | CI and release packaging. |
| `go.mod` | Root Go module (`github.com/gm5dna/open-rig-programmer`), shared by the core library, CLI, and the `app/` GUI. |

## Building and testing

### Prerequisites

- Go 1.25+
- Node.js 22.12+ (required by the Vite/`@sveltejs/vite-plugin-svelte`
  peer dependencies)
- Wails CLI v2.13.0:

  ```sh
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0
  ```

  Make sure `$(go env GOPATH)/bin` is on your `PATH` afterwards, or the
  `wails` binary won't be found.

- Linux only: GTK3 and WebKit2GTK development headers, and the
  `webkit2_41` build tag:

  ```sh
  sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
  ```

### Git hooks (recommended)

A versioned pre-push hook refuses to push while any tracked file matches
a private-fixture pattern — the same guard CI runs. Enable it once per
clone:

```sh
git config core.hooksPath scripts/git-hooks
```

### Build

The frontend must be built before `go build`/`go test`, since
`app/main.go` embeds `app/frontend/dist`. Run the blocks below in order,
starting from the repository root; each block picks up in the directory
the previous one left off in.

```sh
# GUI frontend (required once, since app/main.go embeds frontend/dist)
cd app/frontend
npm install
npm run build
```

```sh
# Core library and CLI, from the repository root
cd ../..
go test ./...
```

```sh
# GUI (Wails v2 + Svelte), from the app/ directory
cd app
wails dev     # live-reloading development build
wails build   # production build; output under app/build/bin/

# Linux needs the webkit2_41 build tag:
wails build -tags webkit2_41
```

## Licence

GPL-3.0-or-later. See [LICENSE](LICENSE) for the full text.

This program is distributed in the hope that it will be useful, but
**WITHOUT ANY WARRANTY**, without even the implied warranty of
merchantability or fitness for a particular purpose — see the GPL for
details. **If you connect this software to your transceiver, you do so
entirely at your own risk**; the authors accept no liability for any
damage to your radio, its firmware, or its memory contents.

## Protocol reference

CAT protocol facts used by this project (command set, frame format,
memory channel structure) are derived from Yaesu's published
**FT-710 CAT Operation Reference Manual (2306-C)**.

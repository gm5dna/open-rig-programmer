# Building from source, and the repository layout

Moved verbatim from the README on 28/08/2026.

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

## Building for Windows

The GUI installer and the CLI zip that `release.yml` ships are built
on a `windows-2025` GitHub Actions runner (the "Windows-hosted build"
— never call a cross-compiled build "native"). To build the same
things yourself, either on a Windows machine or by cross-compiling
from macOS/Linux, the CLI has to exist BEFORE the installer step:
`project.nsi` reaches for it by a fixed relative path per architecture,
`..\..\..\dist\<arch>\rigprog.exe`.

```sh
# 1. CLI, one build per architecture, into the path project.nsi expects
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o app/dist/amd64/rigprog.exe ./cmd/rigprog
GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build -o app/dist/arm64/rigprog.exe ./cmd/rigprog

# 2. GUI + NSIS installer, both architectures in one call (from app/)
cd app
wails build -platform windows/amd64,windows/arm64 -nsis
```

`-nsis` needs `makensis` on `PATH` — this project pins NSIS 3.12
(Homebrew's `makensis` on macOS is the same version the release
pipeline installs on the runner). Without `makensis`, `wails build
-nsis` still exits 0 but silently produces no installer — check for
`app/build/bin/Open Rig Programmer-<arch>-installer.exe` rather than
trusting the exit code.

A Wails build (with or without `-nsis`) regenerates the tracked
frontend bindings under `app/frontend/wailsjs/` and, for `-nsis`
specifically, the tracked template file
`app/build/windows/installer/wails_tools.nsh` (its placeholders become
literal values). It does NOT rewrite `app/wails.json`: that file's
`info.productVersion` is stamped only by the release job and by the
local dry-run script, and each of those restores the file itself
afterwards — the build never touches it. Restore the two files a build
DOES change before committing anything:

```sh
git checkout -- app/frontend/wailsjs/ app/build/windows/installer/wails_tools.nsh
```

`app/build/windows/README.md` covers which files under that directory
are template-generated versus hand-edited, and the same churn rule in
full; `docs/windows-setup.md` covers what the installer does once it
runs on a real machine, which nobody has yet done on Windows for this
project (see its "Status" section).

## Repository layout

| Path | Contents |
| --- | --- |
| `core/` | The library: CAT codec (`cat`, plus `cat/ftdx10`, `cat/ftdx101`) for the Yaesu models, CI-V codec (`civ`, plus `civ/ic7610`, `civ/ic7300`, `civ/ic7300mk2`, `civ/ic705`, `civ/ic9700`, `civ/ic905`, `civ/ic7851`, `civ/ic7760`, `civ/ic7100`, `civ/icr8600`) for the Icom models, capability model (`spec`), codeplug model and diff (`codeplug`), CSV I/O (`csvio`), serial transport (`transport`), radio drivers (`driver/ft710`, `driver/ftdx10`, `driver/ftdx101` for Yaesu; `driver/ic7610`, `driver/ic7300`, `driver/ic7300mk2`, `driver/ic705`, `driver/ic9700`, `driver/ic905`, `driver/ic7851`, `driver/ic7760`, `driver/ic7100`, `driver/icr8600` for Icom), and the safe send choreography (`clone`). |
| `cmd/rigprog/` | The CLI. |
| `app/` | Wails v2 + Svelte desktop GUI. |
| `internal/` | The radio simulators — `fakeradio`, `fakedx10`, `fakedx101` for Yaesu; `fakeic7610`, `fakeic7300`, `fakeic7300mk2`, `fakeic705`, `fakeic9700`, `fakeic905`, `fakeic7851`, `fakeic7760`, `fakeic7100`, `fakeicr8600` for Icom — composition-root wiring, the shared settings store the CLI and GUI both use for unverified-write consent (`userconfig`), menu-table generator (`extable`), and the import-graph guard tests (`guards`). |
| `docs/` | Hardware findings, Linux and Windows setup, the menu-write decision, and the fixture redaction policy. |
| `docs/fixtures-private/` | Git-ignored. Raw radio backups and serial captures — never committed. |

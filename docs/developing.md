# Developing

Building from source, the layout of the repository, the documentation map,
the private evidence records, and how a release is made.

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
DOES change before committing anything — but only if you had no
uncommitted edits of your own in either one; check `git diff` first,
and if you did, restore your own saved copies instead of discarding
them (the macOS section below gives the same advice for the frontend's
build collateral):

```sh
git checkout -- app/frontend/wailsjs/ app/build/windows/installer/wails_tools.nsh
```

The release job and the local dry-run script never take this
shortcut: both save the exact pre-build bytes first and restore those
exact bytes afterwards, never a blind `git checkout --` (see
`app/build/windows/README.md`'s "Template-generated files" section).

`app/build/windows/README.md` covers which files under that directory
are template-generated versus hand-edited, and the same churn rule in
full; `docs/windows-setup.md` covers what the installer does once it
runs on a real machine — confirmed on a Windows 11 ARM64 VM,
05/09/2026; amd64 and a physical machine of either architecture
remain untried (see its "Status" section).

## Repository layout

| Path | Contents |
| --- | --- |
| `core/` | The library: CAT codec (`cat`, plus `cat/ftdx10`, `cat/ftdx101`, `cat/ft891`) for the Yaesu models, CI-V codec (`civ`, plus `civ/ic7610`, `civ/ic7300`, `civ/ic7300mk2`, `civ/ic705`, `civ/ic9700`, `civ/ic905`, `civ/ic7851`, `civ/ic7760`, `civ/ic7100`, `civ/icr8600`) for the Icom models, capability model (`spec`), codeplug model and diff (`codeplug`), CSV I/O (`csvio`), serial transport (`transport`), radio drivers (`driver/ft710`, `driver/ftdx10`, `driver/ftdx101`, `driver/ft891` for Yaesu; `driver/ic7610`, `driver/ic7300`, `driver/ic7300mk2`, `driver/ic705`, `driver/ic9700`, `driver/ic905`, `driver/ic7851`, `driver/ic7760`, `driver/ic7100`, `driver/icr8600` for Icom), and the safe send choreography (`clone`). |
| `cmd/rigprog/` | The CLI. |
| `app/` | Wails v2 + Svelte desktop GUI. |
| `internal/` | The radio simulators — `fakeradio`, `fakedx10`, `fakedx101`, `fakeft891` for Yaesu; `fakeic7610`, `fakeic7300`, `fakeic7300mk2`, `fakeic705`, `fakeic9700`, `fakeic905`, `fakeic7851`, `fakeic7760`, `fakeic7100`, `fakeicr8600` for Icom — composition-root wiring, the shared settings store the CLI and GUI both use for unverified-write consent (`userconfig`), menu-table generator (`extable`), and the import-graph guard tests (`guards`). |
| `docs/` | See the docs map below. |
| `docs/fixtures-private/` | Git-ignored. Raw radio backups and serial captures — never committed. |

## macOS: a local `wails build` may fail its signing step

In an iCloud-synced checkout, `wails build`'s final ad-hoc codesign
step can fail with `resource fork, Finder information, or similar
detritus not allowed`: freshly written files carry
`com.apple.provenance` extended attributes. The build itself has
completed by then; only the self-sign step failed. CI is unaffected.

```sh
xattr -cr "app/build/bin/Open Rig Programmer.app"
codesign --force --deep -s - "app/build/bin/Open Rig Programmer.app"
codesign -dv "app/build/bin/Open Rig Programmer.app"   # confirm: Signature=adhoc
```

`wails build` also runs `npm install`, which can leave
`app/frontend/package-lock.json` or `package.json.md5` showing as
modified with no real change; `git checkout --` those unless you meant
a dependency change, and never commit build collateral.

## Docs map

| File | For whom | What |
| --- | --- | --- |
| `README.md` | Radio owners | What it is, install, first use, safety, limits, help wanted |
| `docs/radio-notes.md` | Radio owners | Per-radio capabilities, refusals and guesses, with pointers to the evidence |
| `docs/linux-setup.md`, `docs/windows-setup.md` | Radio owners | Serial-port setup per platform, with each page's own evidence status |
| `docs/menu-write-decision.md` | Both | Why menu settings are read but never written (a decision record) |
| `docs/icom-models.md` | Reviewers | Every Icom limitation with the code and manual citation behind it |
| `docs/hardware-notes.md` | Reviewers | Evidence record: every session against real hardware, by milestone code; section titles are cited from code and must not change |
| `docs/fixtures.md`, `docs/fixtures-history.md` | Contributors | The fixture redaction policy, and the one audit run under it |
| `docs/developing.md` | Contributors | This page |
| `.github/release-notes-template.md` | Contributors | The body of every GitHub release |
| `CHANGELOG.md` | Everyone | What changed in each release |

## Private records

Three directories are git-ignored and exist only on the maintainer's
disk: `docs/superpowers/` (specifications, plans, capability matrices
and baseline manifests), `.superpowers/` (session ledgers, reviews and
handoffs) and `docs/fixtures-private/` (raw radio captures and the
makers' manuals). Code comments, provenance files and this repository's
evidence records cite them by path so that the evidence chain is
traceable by the maintainer, but nothing in the build or the tests
reads them: `internal/guards/freshclone_test.go` proves a fresh clone
builds and tests without any of them. A citation into one of those
paths is a pointer to a private record, not a missing file.

## Releasing

1. Write the version's entry in `CHANGELOG.md` (move the *Unreleased*
   items under the new heading) and paste it into the release-notes
   template's "What changed in this version".
2. Check the template's Downloads table and first-launch sections still
   match the README's "Install" and "First use".
3. Rehearse with an `rc` tag if the shipped bytes changed shape
   (`git tag v1.4.0-rc.1 && git push origin v1.4.0-rc.1`), check the
   draft's assets, then delete the tag and the draft.
4. Tag on `main` with an annotated tag whose message is the changelog
   entry, push it, and let `release.yml` build every artefact into a
   **draft** release. The workflow never publishes; publishing is a
   separate, human decision.
5. Publish the draft. `CHANGELOG.md`'s compare links and the README's
   Releases link need no change.

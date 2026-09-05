<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# The Windows VM verification leg — human checklist

This is the checklist for the ONE Windows hardware session this
milestone runs: a clean Windows 11 ARM64 VM (UTM, `Architecture
aarch64`, `UsbSharing true`), the rehearsal-tag Windows assets
downloaded from the draft GitHub release, and a real FT-710 passed
through. `scripts/windows-smoke.ps1` prints a pointer to this file when
it finishes; read this BEFORE running it, not after — several of the
steps below have to happen first, and the two writes below it are
never made by that script.

Full detail lives in this project's internal design record's
Verification bar and Register sections (register entries cited by name
below — W1, W2, W3, W4, W6, W9, W13); this file is the step-by-step
version to follow on the day, in the same order. Evidence (script
output, screenshots) goes to
`.superpowers/sdd/2026-09-04-windows/evidence/`.

## Safety preamble (read this every time, not just the first)

Before any CAT traffic is sent — the same preamble this project's
earlier real-radio sessions used (`docs/hardware-notes.md`'s "Safety
preamble"):

- A full SD-card backup of the radio's configuration, taken and
  verified present, so a card restore is available if anything goes
  wrong.
- RF output at minimum, on a dummy or low-risk antenna path.
- Stuart present, watching the radio's TX indicator, for the entire
  session — at every connect, not only the first.
- No other application holding the radio's serial ports open.

## Before you start

1. **Record the radio's PTT SELECT / USB keying menu settings from the
   front panel, before anything else.** The every-COM-port probe below
   (step 4) is the first time this project has ever opened the
   CP2105's **Standard** UART anywhere — it does not open at all on
   macOS (`docs/hardware-notes.md:89-99`) — and that UART's RTS/DTR
   lines are the radio's USB keying lines when PTT SELECT points at
   them. Register entry **W6** ("no TX key-up at open with
   `InitialStatusBits` low") is only interpretable against this
   recording: if TX ever keys on the Standard port and PTT SELECT was
   pointed elsewhere, W6 is unaffected; if it was pointed at USB, W6
   has failed. Write the menu setting down before step 4, not after.
2. Confirm host-Mac step 8 of the verification bar has already run:
   one `rigprog probe` on the host Mac with the `InitialStatusBits`
   build, Stuart watching TX (W6's macOS half) — this happens BEFORE
   passthrough, not on the VM, and is the orchestrator's job.
3. Download the rehearsal tag's Windows assets (the exact tag varies
   run to run — `v1.3.0-rc.2` was current on 05/09/2026) from the draft
   GitHub release and verify them against its `SHA256SUMS` BEFORE
   installing anything:
   ```powershell
   certutil -hashfile open-rig-programmer-<rehearsal-tag>-windows-arm64-installer.exe SHA256
   ```
   Compare the printed hash against the matching line in `SHA256SUMS`
   by eye. Note in the evidence HOW the files reached the VM (browser
   download vs. a shared folder) — this is register entry **W9**'s
   context (a browser download carries the mark-of-the-web that drives
   SmartScreen; a shared-folder copy may not).

## The checklist (Verification bar steps 1-7, in order)

1. **Run the installer. Record SmartScreen's behaviour** (screenshot)
   — expected: "Windows protected your PC" → *More info* → *Run
   anyway* (register **W9**).
2. **Confirm the install.** `Open Rig Programmer.exe`, `rigprog.exe`
   and `LICENSE` are in the install directory; a Start-menu entry
   exists; Programs-and-Features shows the numeric version (e.g.
   `1.3.0`, no leading `v`, no `-rc.N` suffix).
   - Right-click `Open Rig Programmer.exe` → *Properties* → *Details*:
     record the Product version and File version shown (expected: the
     numeric tag). If either is blank, that is a finding for the
     write-up, not a failure of this leg — `.NET`'s own
     `FileVersionInfo` reader returned empty for this build's
     language-neutral version resource on the CI runner (release run
     33950484060, 05/09/2026); `check-windows.sh` is the byte-level
     gate that actually enforces this value.
3. **Launch the GUI from the Start menu.** Confirm the status bar
   shows the rehearsal tag. Run the Demo workflow end to end (connect
   to the built-in Demo radio, edit, Send) — this exercises nothing on
   the real radio and needs no passthrough yet.
4. **Pass the CP2105 through** (UTM's USB menu). Confirm in Device
   Manager that a driver is bound (screenshot the name, version and
   provider — registers **W1**, **W2** as applicable). Then run the
   smoke script:
   ```powershell
   .\windows-smoke.ps1 -OutDir <path under .superpowers\sdd\2026-09-04-windows\evidence>
   ```
   It captures OS build/architecture, the `Get-PnpDevice`/driver
   properties for `VID_10C4`, the COM port list, `rigprog version`,
   `rigprog ports`, a `probe` of EVERY COM port found (expected:
   exactly one succeeds — registers **W3**, **W4**, **W13**), and, on
   the port that answered, a `read`/`export`/`import` round trip
   compared byte-for-byte after normalising `read_at`. **It makes no
   write.** Watch the TX indicator at every connect it makes (RF at
   minimum, SD backup already taken — the safety preamble above,
   applied here exactly as it was for M5a).

   **Trial-slot gate — do this before step 5, using the `baseline.json`
   the script just wrote.** The plan for step 5/6 below is to edit and
   then restore memory slot **M-96**. `docs/hardware-notes.md:815-818`
   records M-96 as a CAT-created artefact from an earlier session's
   empty-slot-create trial, left pending front-panel cleanup — it may
   therefore be EMPTY by the time this session runs. Open
   `baseline.json` and confirm M-96's `data` field is **non-null**
   (populated). This matters because the restoration write in step 6
   sends M-96's ENTIRE baseline back, and the FT-710 has no CAT erase:
   restoring an EMPTY baseline slot is a `DiffErased`, which
   `codeplug.Diff`'s own erase gate (`core/codeplug/diff.go:777`, the
   `kind == DiffErased` branch) always marks Blocked rather than sends
   — the restoration would fail by design, not by accident. **If M-96
   is empty**, do not use it: pick another POPULATED slot Stuart names
   instead, and CHANGE one of its existing fields (e.g. its tag) for
   the trial edit — never create a channel in an empty one.
5. **GUI: the trial edit.**
   - The port picker should list both COM ports with their
     descriptions (register **W4**'s friendly-name form, if that shape
     appears). Connect to the port step 4's probe named as the
     answering one.
   - Read the radio.
   - Edit the gated slot (M-96, or the Stuart-named alternative) — a
     small, reversible change (e.g. its tag).
   - **Send — this is write 1 of 2.** Screenshot the confirmation and
     the result; screenshot the status bar showing the version.
6. **CLI: the restoration — write 2 of 2.**
   ```powershell
   rigprog.exe diff --port COMn --model FT-710 baseline.json
   ```
   Confirm the diff shows exactly the M-96 (or named alternative)
   change from step 5, nothing else. Then:
   ```powershell
   rigprog.exe write --port COMn --model FT-710 --firmware <version read off the front panel> baseline.json
   ```
   Answer the interactive confirmation prompt yourself — this is the
   point of running it by hand rather than from a script. Use `--yes`
   only if the session genuinely cannot present the prompt (e.g. a
   remote/non-interactive shell), and say so in the evidence if you
   do. `--firmware` is required on this session's first write and
   takes the version string read directly off the radio's front panel
   or SD-card backup, never guessed.

   Then:
   ```powershell
   rigprog.exe read --port COMn --model FT-710 --out after.json
   ```
   and confirm `after.json` is byte-identical to `baseline.json` after
   normalising `read_at` the same way the smoke script does (a
   regex replace of the `"read_at":"..."` value in both files before
   comparing).
7. **Uninstall.** Confirm the install directory and the Start-menu/
   desktop shortcuts are gone. Confirm `%AppData%\rigprog` is **STILL
   PRESENT** — a package never deletes user data, and the uninstall
   Section is written to prove it (`app/build/windows/README.md`).

## Contingency

If Windows-on-ARM binds no driver to the CP2105 and Silicon Labs offers
no ARM64 package, stop at step 3 — the packaging still merges, and the
docs are written to say "installed and launch-tested on an ARM64 VM;
no radio has been connected on Windows", with W1/W3-W7 staying ASSUMED
and the lifts they still need named. The same applies if UTM cannot
pass the composite CP2105 through at all; in that case the roadmap
records that this needs a physical Windows machine instead of a VM.

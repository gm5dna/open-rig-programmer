# Open Rig Programmer

A free memory-channel programmer for Yaesu, Icom and Kenwood radios.
Read the radio's memories into a file, edit them in a grid or a
spreadsheet, and send them back over the radio's ordinary USB cable.
A desktop app and a command-line tool, `rigprog`, for macOS, Windows
and Linux. It imports and exports CSV, including CHIRP's, reads before
it writes, shows every change for approval, and reads each channel back
after writing it. Nothing is deleted and no menu setting is changed.

![The channel grid, connected to the built-in demo radio](docs/images/app-demo.png)

## A personal project, tested on one radio

This started as a programmer for my own FT-710, and the FT-710 is still
the only radio it has ever been connected to. Reading and writing there
are verified on the real thing.

Many other radios have been added since, built from the makers' own
published protocol manuals and tested against simulators. Reading them
is safe. Writing stays switched off until you switch it on for that
radio, and every write is previewed, snapshotted and read back.

| Radio | Read | Write |
| --- | --- | --- |
| **FT-710** | ✅ | ✅ verified on a real radio |
| **FTdx10**, **FTdx101D**, **FTdx101MP**, **FT-891**, **FT-991A**, **FTdx5000**, **FT-2000**, **FT-2000D**, **FTdx9000** | ✅ | ⚠️ opt-in |
| **IC-7610**, **IC-7300**, **IC-7300MK2**, **IC-705**, **IC-9700**, **IC-905**, **IC-7851**, **IC-7850**, **IC-7760**, **IC-7100**, **IC-7800**, **IC-7600**, **IC-7410**, **IC-7700**, **IC-9100**, **IC-7200** | ✅ | ⚠️ opt-in |
| **IC-R8600** (a receiver) | ✅ | ⚠️ opt-in |
| **TS-590S**, **TS-590SG** | ✅ | ⚠️ opt-in |
| **TS-890S**, **TS-990S** | ✅ | ⚠️ opt-in, existing channels only |
| **TS-2000**, **TS-2000X**, **TS-B2000** | ✅ | ⚠️ opt-in |
| **TS-570D**, **TS-570S** | ✅ | ⚠️ opt-in |
| **TS-870S** | ✅ | ⚠️ opt-in |

Per-radio detail, including where the program is guessing, is in
[docs/radio-notes.md](docs/radio-notes.md).

**If you own one of the opt-in radios, please try it and say how it
went.** A report from a real radio is the most useful thing anyone can
send. [Open an issue](../../issues/new/choose) with the output of
`rigprog version` and `rigprog probe`; one channel's line from a read
is plenty, since a whole memory file carries your callsign.

## Install

Everything is on the [Releases page](../../releases), with a
`SHA256SUMS` file to check any download.

- **macOS**: unzip the app. It is not notarised, so the first time,
  right-click it and choose *Open*.
- **Windows**: run the installer for your machine (amd64 or ARM64).
  SmartScreen will say *Windows protected your PC*: click *More info*,
  then *Run anyway*. [docs/windows-setup.md](docs/windows-setup.md)
  covers the serial driver.
- **Linux**: on Debian, Ubuntu or Mint, `sudo apt install ./<file>.deb`.
  Elsewhere, the tarball is a single static binary.
  [docs/linux-setup.md](docs/linux-setup.md) covers the `dialout` group
  and ModemManager.

## First use

Choose the radio and the port, connect (or pick *Demo* to try it with a
simulated radio), read, edit in the grid, then *Send*. The radio's USB
adapter appears as two serial ports and only one answers. The FT-710
needs firmware V01-10 or later.

The command line does the same job for scripts and backups:

```sh
rigprog read  --port /dev/cu.SLAB_USBtoUART --out radio.json   # save the memories
rigprog write --port /dev/cu.SLAB_USBtoUART edited.json        # preview, then send
```

`rigprog help` lists the rest. Add `--model IC-7300` for another radio,
or `--fake` for the simulator.

### Switching on writes for an unverified radio

The app asks the first time you connect to one of these radios, and its
*Unverified writes…* button opens the list at any time. On the command
line:

```sh
rigprog settings unverified-writes IC-7610 on     # allow writes to the IC-7610
```

Permission changes what the program may send, not how carefully it
sends it.

## For developers

[docs/developing.md](docs/developing.md) covers building from source,
the repository layout, the evidence records and releasing;
[CHANGELOG.md](CHANGELOG.md) lists what changed in each release.

## Licence

GPL-3.0-or-later; see [LICENSE](LICENSE). This program comes
**without any warranty**. If you connect it to your transceiver, you
do so entirely at your own risk; the authors accept no liability for
damage to your radio, its firmware or its memory contents.

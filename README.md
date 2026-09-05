# Open Rig Programmer

Read your Yaesu or Icom radio's memory channels into a file, edit them
in a grid or a spreadsheet, and send them back. It uses the radio's
ordinary USB cable: no programming cable, no SD-card shuffling.

It comes as a desktop app and a command-line tool, `rigprog`, for
macOS, Windows and Linux. Free and open source, under the GPL.

## Which radios

| Radio | Read | Write |
| --- | --- | --- |
| **FT-710** | ✅ | ✅ verified on a real radio |
| **FTdx10**, **FTdx101D**, **FTdx101MP** | ✅ | ⚠️ opt-in |
| **FT-891** | ✅ | ⚠️ opt-in |
| **IC-7610**, **IC-7300**, **IC-7300MK2**, **IC-705**, **IC-9700**, **IC-905**, **IC-7851**, **IC-7850**, **IC-7760**, **IC-7100** | ✅ | ⚠️ opt-in |
| **IC-R8600** (a receiver) | ✅ | ⚠️ opt-in |

Only the FT-710 has ever been connected to this program. Every other
radio was built from the maker's published protocol manual and tested
against a simulator. Reading them is safe: the program sends only
documented read commands. Writing to them stays switched off until you
switch it on, one radio at a time (see below).

What each radio can and cannot do, and where the program is guessing,
is in [docs/radio-notes.md](docs/radio-notes.md). Kenwood radios are
next on the list.

## Install

Download from the [Releases page](../../releases). `SHA256SUMS` on that
page lets you check any download.

| Platform | File |
| --- | --- |
| macOS app | `open-rig-programmer-<version>-darwin-universal.app.zip` |
| macOS command line | `rigprog-<version>-darwin-universal.tar.gz` |
| Windows app + command line (installer) | `open-rig-programmer-<version>-windows-amd64-installer.exe` or `-arm64-installer.exe` |
| Windows command line only | `rigprog-<version>-windows-amd64.zip` or `-arm64.zip` |
| Debian, Ubuntu, Mint (app + command line) | `open-rig-programmer_<version>_amd64.deb` or `_arm64.deb` (version without the leading `v`) |
| Other Linux (command line only) | `rigprog-<version>-linux-amd64.tar.gz` or `-arm64.tar.gz` |

**macOS**: the app is not notarised, so the first time, right-click it
and choose *Open*. After that it opens normally.

**Windows**: the installer is not code-signed, so the first time
SmartScreen shows *Windows protected your PC*: click *More info*, then
*Run anyway*. Pick the installer that matches your machine; the amd64
one refuses to run on an ARM64 PC. The zip is a single `rigprog.exe`
that needs no install. Details, including the serial driver, are in
[docs/windows-setup.md](docs/windows-setup.md).

**Debian, Ubuntu, Mint**: `sudo apt install ./<downloaded file>.deb`
installs the app, the command line, a menu entry and the serial-port
rule. **Other Linux**: the tarball is a single static binary; unpack
and run. Either way, read [docs/linux-setup.md](docs/linux-setup.md)
before connecting a radio.

## First use

**FT-710 firmware**: memory programming needs firmware V01-10 or
later. Check the version on the radio's front panel; the program
cannot ask the radio.

**Serial port**: the radio's USB adapter appears as two serial ports,
and only one of them answers.

- *macOS*: nothing to install, and only the right port will open.
- *Windows*: on ARM64 you must install Silicon Labs' CP210x driver by
  hand; both COM ports look identical, so probe each in turn
  ([docs/windows-setup.md](docs/windows-setup.md)).
- *Linux*: join the `dialout` group and keep ModemManager off the
  radio ([docs/linux-setup.md](docs/linux-setup.md)).

**In the app**: choose the radio and the port, connect (or press
*Demo* to try it with a simulated radio), read, edit in the grid, then
*Send*. Before anything is transmitted, the app shows every change it
is about to make, and anything it refuses to make, with the reason.

**On the command line**:

```sh
rigprog ports                                  # list likely serial ports
rigprog probe --port /dev/cu.SLAB_USBtoUART    # confirm which radio answers
.\rigprog.exe probe --port COM3                # Windows: the port name from Device Manager
rigprog read  --port ... --out radio.json      # save the radio's memories to a file
rigprog diff  --port ... edited.json           # preview what would change
rigprog write --port ... edited.json           # send the changes
```

Add `--model IC-7300` (or any supported name) for a radio other than
the FT-710, and `--fake` to use the simulator instead of a port.
`export` and `import` convert to and from CSV, including CHIRP's CSV,
without a radio. `rigprog version` reports the build; quote it in bug
reports. `rigprog help` lists everything else.

## What keeps your radio safe

Every send, on every radio, follows the same steps: the radio is read
fresh first; a snapshot of what it holds is saved to disk; every change
the program is about to make, and everything it refuses to make, is
shown for your approval with the reason; then each channel is written
and read back, and the send stops at the first mismatch. Deleting a
channel and changing menu settings are never sent, whatever you permit.

### Switching on writes for an unverified radio

Every write command for the opt-in radios comes from the maker's own
manual, but none has been proven on a real radio, so the program
refuses to send any of them until you say so:

```sh
rigprog settings unverified-writes                # list every radio and its setting
rigprog settings unverified-writes IC-7610 on     # allow writes to the IC-7610
rigprog settings unverified-writes IC-7610 off    # withdraw that permission
```

The app asks the same question the first time you connect to one of
these radios, and its *Unverified writes…* button opens the list at
any time. The app and the command line share one settings file, so a
decision made in either holds for both. Permission changes what the
program may send, not how carefully it sends it.

## Limits

- **No deleting a channel.** The Yaesu radios have no delete command.
  The Icom radios do, but this program deliberately does not use it.
- **No writing menu settings.** The program reads and displays a Yaesu
  radio's menu settings, never writes them; the reasoning is in
  [docs/menu-write-decision.md](docs/menu-write-decision.md). Icom
  menus are not read at all.
- **FT-710**: tone, scan-skip and clarifier cannot be set over CAT, so
  edits to them are refused rather than silently dropped.
- Speed guesses, which of two ports answers, channels that are not
  read, and every other per-radio limit are listed in
  [docs/radio-notes.md](docs/radio-notes.md).

## Help wanted

The most useful thing anyone can send is a report from a real radio:

- any radio in the opt-in rows above, even a read of one channel;
- an FT-710 on Linux, which has only been tried in virtual machines
  without a radio;
- an FT-710 on an amd64 Windows PC, or any physical Windows machine.

[Open an issue](../../issues/new/choose) with the output of
`rigprog version` and `rigprog probe`, and what the radio did. One
channel's line from a read is plenty; a whole memory file carries your
callsign.

## For developers

[docs/developing.md](docs/developing.md) covers building from source,
the layout of the repository, the evidence records and releasing.
[CHANGELOG.md](CHANGELOG.md) lists what changed in each release.

## Licence

GPL-3.0-or-later; see [LICENSE](LICENSE). This program comes
**without any warranty**. If you connect it to your transceiver, you
do so entirely at your own risk; the authors accept no liability for
damage to your radio, its firmware or its memory contents.

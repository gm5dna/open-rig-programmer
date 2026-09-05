<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# Radio notes: what each radio can do, and where the program is guessing

This page is for owners. It says, radio by radio, what the program
reads and writes, what it refuses, and which of its settings are
guesses taken from a manual rather than facts observed on a radio. The
evidence behind each statement is in the files named at the end of
each section; those are written for reviewers and contributors.

Two words are used throughout:

- **Verified** means the behaviour has been observed on a real radio in
  a recorded session (`docs/hardware-notes.md`).
- **Opt-in** means every command comes from the maker's published
  protocol manual and has been exercised against a simulator built
  from the same manual, but no real radio of that model has ever been
  connected. The program refuses to write to such a radio until you
  switch writes on for it (README, *Switching on writes for an
  unverified radio*). Reading is always allowed: only documented read
  commands are sent.

Shared by every radio: a channel cannot be deleted from the program
(the Yaesu radios have no such command; the Icom radios do, and the
program deliberately does not use it), and menu settings are never
written (`docs/menu-write-decision.md`).

## Yaesu

### FT-710 (verified)

Reads and writes the 99 memories and the 9 PMS pairs, and reads every
menu setting. Writes were proven on a real radio, including creating a
channel in an empty slot and clearing a tag. Needs firmware V01-10 or
later; the program cannot ask the radio its version.

Refused: tone, scan-skip and clarifier cannot be set over CAT, so an
edit to any of them is refused rather than silently dropped. The
radio does not report per-channel CTCSS tone frequencies, so the
program preserves whatever tone the radio already holds.

Evidence: `docs/hardware-notes.md` (the M5a, M5b and M8c sessions, and
the Windows session of 05/09/2026).

### FTdx10, FTdx101D, FTdx101MP (opt-in)

Read the 99 memories and the 9 PMS pairs and every menu setting, on
the same terms as the FT-710. Writes follow the FT-710's ladder but
have never been sent to a real radio.

Evidence: `core/driver/ftdx10/doc.go` and `core/driver/ftdx101/doc.go`
(each carries a register of every assumption and the capture that
would settle it).

### FT-891 (opt-in)

Reads the 99 memories, the 9 PMS pairs and the 159 menu settings; its
menu addresses are four digits where the other Yaesu radios use six,
and files accept either.

Refused: tone and scan-skip cannot be set over CAT (the memory record
has no tone-number byte and no scan-skip flag), and a transmit-clarifier
flag arriving in a file written for another radio is refused rather
than sent. A CHIRP file's `CW`, `CWR` and `RTTY` rows are not imported:
they resolve to names this radio's own mode list does not print (it
prints `CW`, `CW-R`, `RTTY-LSB` and `RTTY-USB`), so the row is blocked
rather than guessed at.

Guesses: its **speed**. The manual lists four rates and marks none as
the factory setting, so the program opens at 38400; if your radio is
set differently, change menu 0506 on the radio, because the program
has no speed setting. Its **socket**: the USB connection is a
dual-UART bridge, so the radio appears as two serial ports, and the
manual never says which carries CAT; if the first is silent, try the
other. And the manual contradicts itself about whether a memory
channel may be read at all, so a read refused for a channel that is
plainly in use is the manual's ambiguity showing, not a fault in the
program. A single read of an occupied channel on a real FT-891 would
settle it.

Evidence: `core/driver/ft891/doc.go`; the manual is Yaesu's CAT
Operation Reference Manual 1909-C.

## Icom

### Shared by every Icom model

- Each radio is addressed only at its factory CI-V address; there is
  no option to change it.
- Menu settings are not read.
- A few channel states cannot be written back and are refused with the
  reason: a channel in a Select scan group (most models), a split
  channel (IC-7300, IC-7300MK2, IC-7100), DATA modes (IC-7610,
  IC-7851/IC-7850, IC-7760), D-STAR call signs (IC-7100), and the
  digital-squelch bytes of a D-STAR, P25, NXDN, DCR or dPMR channel
  (IC-R8600).
- A channel outside the shape the write gate expects is refused, never
  rewritten to fit.
- The IC-705, IC-905 and IC-R8600 list only the memories their start-up
  scan finds (a bounded walk of the memory space). A channel stored
  outside that walk is simply not listed; its absence is not evidence
  the channel is empty.

Evidence for everything in this section, per model:
`docs/icom-models.md`.

### IC-7610, IC-7300, IC-7300MK2 (opt-in)

Read and write the memory channels. The IC-7300 and IC-7300MK2 cannot
create a channel in an empty slot: the record's Select-group setting
has no honest default, so the write is refused rather than invented.
The IC-7610 refuses, at read time, a frequency its record cannot hold.
Speeds: the IC-7610's is an arbitrary pick among its six rates; the
IC-7300's is the highest rate both radio and program support; the
IC-7300MK2's is derived conservatively from its wake-up command.

Evidence: `docs/icom-models.md` ("Baud rates", and the per-model
bullets).

### IC-705, IC-9700, IC-905 (opt-in)

Read and write the memory channels found by the start-up walk (see
above). The IC-705's walk covers only the first ten of its hundred
memory groups, and nothing in the program widens it; a radio with
nothing in those groups shows its four CALL channels and no memories.
The IC-905's demo radio starts empty by design.
The IC-9700's repeater-offset scale is an unresolved question in the
manual: a wrong reading would put every offset out by ten times, and
the program cannot detect it. A non-octal DTCS code reads back as
unknown on the IC-705 and IC-905. Speeds are assumed for all three.

Evidence: `docs/icom-models.md` (the IC-705, IC-9700 and IC-905
bullets, and Erratum 14 for the offset scale).

### IC-7851 and IC-7850 (opt-in)

These two share one manual, one CI-V address and one memory format, and
the program cannot tell them apart: the model it reports is the one you
picked from the list. The program offers all six of the radio's USB
speeds even if you have wired the older remote-jack path, which stops
at 19200. A record that disagrees with the manual's layout is refused
rather than reinterpreted, and a frequency outside the declared receive
range is refused at read time.

Evidence: `docs/icom-models.md` (the IC-7851/IC-7850 bullets); the
manual is the IC-7851 instruction manual, section 18.

### IC-7760 (opt-in)

This radio is two boxes, and only one connection is supported: the USB
socket on the back of the control head, which appears on your computer
as two serial ports. Which of the two answers is a setting on the
radio, and the manual prints no default, so if the first port is
silent, try the other. The remote socket on the RF deck is not
supported. Its speed is a guess: the manual gives no CI-V speed
anywhere, so the program opens at 19200, and a wrong guess simply times
out. Whether its two scan edges can be cleared at all is not settled.
A frequency its record cannot hold is refused when it is read.

Evidence: `docs/icom-models.md` (the IC-7760 bullets).

### IC-7100 (opt-in)

The program lists this radio's 495 ordinary memories (banks A to E, 99
channels each) and nothing else. The six programmed scan edges and four
call channels are real channels on the radio, but the manual never says
what bank number addresses them, so the program does not read them
rather than guess an address and read the wrong thing. Their absence
from the list is not evidence the radio has none. A record with no
transmit frequency reports that field as unavailable rather than as
0 Hz. This is the only Icom model the program opens with two stop bits,
because its manual states no serial format for the CI-V link at all.

Evidence: `docs/icom-models.md` (the IC-7100 bullets); the manual is
the IC-7100 full manual, section 20.

### IC-R8600 (opt-in; a receiver)

This is a receiver, and the grid says so: there are no
transmit-frequency or transmit-tone columns, because the radio has no
transmitter and its memory record has no such bytes. Four things about
it are guesses the program is honest about:

- **Speed**: the CI-V guide prints no factory default, mentions no
  automatic setting and never lists the speeds the menu offers, so the
  program opens at 19200, and a wrong guess simply times out.
- **Capacity**: the guide never states how many memories the receiver
  holds, so the program has no total to show and cannot warn you before
  the receiver is full; what it does when full is unknown.
- **Connection**: the receiver has four possible control terminals (a
  remote jack, a front and a rear USB port, and a network connection),
  and the program talks over USB, so if one port is silent, check which
  terminal the receiver has been told to use.
- **Digital modes**: tone squelch is read and written on FM channels
  only. A D-STAR, P25, NXDN, DCR or dPMR channel cannot be written back
  unless its digital-squelch bytes match what the program assumes, and
  switching a channel into one of those modes is refused outright; set
  the digital squelch at the receiver.

It cannot create a channel in an empty slot (the record's Select-group
setting has no honest default, the same refusal the IC-7300s make), a
channel marked as skipped is refused rather than rewritten as
unskipped, and a write to a slot the start-up walk never listed is
refused rather than allowed to overwrite a channel nothing read.

Evidence: `docs/icom-models.md` (the IC-R8600 bullets).

## Sources

Protocol facts come from the makers' published documents, each pinned
by revision in the code that transcribes it.

| Radio | Document |
| --- | --- |
| FT-710 | Yaesu CAT Operation Reference Manual 2306-C |
| FTdx10 | Yaesu CAT Operation Reference Manual 2308-F |
| FTdx101D, FTdx101MP | Yaesu CAT Operation Reference Manual 2308-L |
| FT-891 | Yaesu CAT Operation Reference Manual 1909-C |
| IC-7610 | Icom CI-V Reference Guide rev 4 |
| IC-7300 | Icom Full Manual §19, rev 12b |
| IC-7300MK2 | Icom CI-V Reference Guide rev 0 |
| IC-705 | Icom CI-V Reference Guide rev 6 |
| IC-9700 | Icom CI-V Reference Guide rev 4 |
| IC-905 | Icom CI-V Reference Guide rev 2 |
| IC-7851, IC-7850 | Icom Instruction Manual rev 3, section 18 |
| IC-7760 | Icom CI-V Reference Guide rev 2 |
| IC-7100 | Icom Full Manual A7085-2EX-5, section 20 |
| IC-R8600 | Icom CI-V Reference Guide rev 3a |

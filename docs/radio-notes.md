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
(the Yaesu radios have no such command; the Icom radios and the
TS-890S and TS-990S do, and the program deliberately does not use
it), and menu settings are never written
(`docs/menu-write-decision.md`).

## Yaesu

### FT-710 (verified)

Reads and writes the 99 memories and the 9 PMS pairs, and reads every
menu setting. Writes were proven on a real radio, including creating a
channel in an empty slot and clearing a tag. Needs firmware V01-10 or
later; the program cannot ask the radio its version, but memory CAT
arrived with that firmware, so a radio that answers the read has
proved it. Nothing is typed in.

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
menu addresses are four digits, other Yaesu radios use three or six,
and files accept any of the three widths.

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

### FT-991A (opt-in)

Reads the 99 memories, the 9 PMS pairs and 152 menu settings. Two
things this radio shows differently from the others are worth knowing
before they surprise you.

The **PMS pairs are listed as the channel numbers 100 to 117**, which
is what the radio's CAT record uses, while the radio's own front panel
and manual print the same eighteen slots as `P-1L` to `P-9U`. The
numbers are the wire's and the letters are the panel's; they name the
same slots in the same order.

The **settings list shows 152 items where the radio's menu chart prints
153 rows**. Row 087, RADIO ID, is left out because the chart gives it
neither a width nor a parameter — ten printed hyphens and nothing else
— so the program cannot size an answer to it and will not send a
question it cannot read. A single `EX087;` read on a real FT-991A would
settle it.

Refused: the tone frequency number, the DCS code and scan skip are not
part of what this program writes to a channel. The memory record does
carry a five-state tone byte — CTCSS off, CTCSS encode and decode,
CTCSS encode, DCS encode and decode, DCS encode — which the program
reads and writes; what it has no field for is *which* tone or *which*
DCS code. The radio does have a CAT command for those two, `CN`, and it
can set as well as report them — but what it sets is the tone and code
the radio is using now, not what a channel holds, and this program does
not send it. Scan skip has no position anywhere in the memory record.
Set the number, the code and the skip marking at the radio. A CHIRP file's `DTCS` and `Cross` rows are therefore still
refused, and the reason given says so: this radio writes the DCS state
but not the DCS code. A CHIRP file's `CW`, `CWR` and `RTTY` rows are
not imported either, for the FT-891's reason — they resolve to names
this radio's mode list does not print (it prints `CW`, `CW-R`,
`RTTY-LSB` and `RTTY-USB`). And C4FM is one of this radio's fourteen
modes that CHIRP has no name for at all, so no CHIRP file can describe a
C4FM channel.

Guesses: its **speed**. The menu row that sets the rate for the socket
this program uses is 031 CAT RATE, which lists 4800, 9600, 19200 and
38400 and marks none as the factory setting, so the program opens at
38400; if your radio is set differently, change menu 031, because the
program has no speed setting. Menu 029 is not that row — 029 sets the
rate of the rear-panel RS-232C jack, a different port. Its **socket**:
the USB connection is a built-in dual-UART bridge, so the radio appears
as two serial ports and the manual never says which carries CAT; if the
first is silent, try the other.

Evidence: `core/driver/ft991a/doc.go`; the manual is Yaesu's CAT
Operation Reference Book 1711-D.

### FTdx5000 (opt-in)

Read and write the memory and PMS (programmable memory scan) channels.
This radio has no tag/name command anywhere in its manual, so no Tag
column is shown for it. The CTCSS tone is a live tone-table index and is
read and written like the clarifier, shift and CTCSS state; there is no
scan-skip position and no data-mode byte in this 27-byte record. No
FTdx5000 has ever answered a frame from this project, so every write
stays behind the opt-in consent route.

Evidence: `core/driver/ftdx5000/doc.go`; the manual is the Yaesu CAT
Operation Reference Manual, revision 1907-D.

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
An imported CHIRP channel is refused at the write on the IC-7300 and
IC-7300MK2, on the filter and the data mode — CHIRP has a column for
neither, so a CHIRP file alone can never complete a write there — and on
the empty slot itself, since neither radio can create a channel it does
not already hold. The transmit frequency is not among the refusals: a
blank `Duplex` column is the file's own simplex statement, and these
records carry no split flag, so an import states that the channel
transmits where it receives.
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

### IC-7800 (opt-in)

Read and write the memory and scan-edge channels. A HIGH-proximity
clone of the IC-7610's record shape (25 B record-only over a 2-byte
flat address) at its own address, 6Ah: same fields mapped, same fields
left unmapped, and the tone/data-mode nibble pair in byte 8 SWAPPED
relative to the IC-7610's own — caught and fixed during this radio's
own build (`core/driver/ic7800`'s `FieldSpan`).

Evidence: `docs/superpowers/icom-matrices/ic7800-capability-matrix.md`.

### IC-7600 (opt-in)

Read and write the memory and scan-edge channels. Another
HIGH-proximity clone of the IC-7610's record shape at its own address,
7Ah, with one wire deviation of its own: the SELECT byte is a WHOLE
unmapped E6 region on this radio, not the IC-7610's nibble split.

Evidence: `docs/superpowers/icom-matrices/ic7600-capability-matrix.md`.

### IC-7410 (opt-in)

Read and write the memory and scan-edge channels. NOT a clone of the
IC-7610's shape: its own record is 40 bytes, with a genuine
TX-duplicate block. A write that leaves the transmit frequency unset
mirrors the receive frequency into it, per this radio's own document,
rather than refusing.

Evidence: `docs/superpowers/icom-matrices/ic7410-capability-matrix.md`.

### IC-7700 (opt-in)

Read and write the memory and scan-edge channels, and the transmit
(split) frequency. Its RX fields match the IC-7610 family exactly;
its own TX-duplicate block reuses existing field types rather than
introducing new ones. Its 39-byte record over a 2-byte flat address is
the same shape as the already-registered IC-7300's, so a radio moved
onto the wrong factory address answers a length this program cannot
tell apart from that sibling's.

Evidence: `docs/superpowers/icom-matrices/ic7700-capability-matrix.md`.

### IC-9100 (opt-in)

Reads and writes the memory channels of ONE band (this build defers
the optional 4th, 1200 MHz, band; its frequency-field encoding is
unresolved from this document). Its own record additionally carries
duplex, offset, DTCS code and DTCS polarity — richer than every other
radio in this wave — but a CHIRP `DTCS`/`Cross` cell is still refused:
the record names no DCS tone STATE, only the code and polarity bytes,
which this program does not yet import from a CHIRP file. This
radio's own document states no CI-V serial-framing fact, so unlike
its siblings this row opens at the port's own default framing rather
than a driver-asserted stop-bit count.

Evidence: `docs/superpowers/icom-matrices/ic9100-capability-matrix.md`.

### IC-7200 (opt-in)

Read and write the memory and scan-edge channels. This radio has no
channel-name field over CI-V at all — NoTag, the wave's only one — so
no Tag column is shown for it. Its 17-byte record maps no tone field of
any kind and no scan-skip bit either, a stricter absence than every
other radio in this wave: a `Tone` or `Scan Skip` value cannot travel
over this frame at all, set both at the radio. A write that leaves the
transmit frequency unset mirrors the receive frequency into it, on the
same SimplexTxEqualsRx footing as the IC-7410. This driver implements
CI-V serial framing (8-N-1), unlike the IC-9100's own row.

Evidence: `docs/superpowers/icom-matrices/ic7200-capability-matrix.md`.

## Kenwood

### TS-590S and TS-590SG (opt-in)

Read the 100 memory channels, the 10 programmable scan ranges (each a
start and an end frequency, listed as `100L`/`100U` and so on), and the
menu settings: 88 of them on the TS-590S, 100 on the TS-590SG. Tone and
scan skip ARE read and written on these radios, unlike every Yaesu model
above: the memory record carries a tone mode, separate transmit and
receive tone numbers and a channel-lockout flag.

Refused: **a memory channel whose transmit frequency you have not
supplied yourself**, which is every channel as it comes off the radio.
These radios express a split as two frames over one channel number, the
program sends one, and what the second frame answers on a simplex
channel is printed nowhere in the manual — so a read leaves the
transmit frequency unavailable rather than guessing at it, and the
write is refused, naming register entry A9. In practice that means
reading the memories, editing a name and sending them straight back is
refused on every memory channel until you fill the transmit frequency
in; typing the receive frequency there is the simplex channel the one
frame the program sends can express. A **genuine split** is refused
even then, rather than silently written as simplex. A **scan range** is
not affected: in that bank the second frame carries a range's end
frequency rather than a transmit frequency, so there is no transmit
disposition to require. The refusal lifts on a row only when somebody
reads a simplex channel's transmit side off a real radio of that model
and reports what came back.

Also refused: **any channel that is not FM**. It is the broadest refusal
on these radios, so it is worth knowing first: the two bytes the memory
record carries beside the mode have only two printed meanings, "FM
Normal" and "FM Narrow", and the manual never says what they mean in
SSB, CW, AM or FSK — so an SSB or CW channel is refused rather than
written with a byte whose meaning nobody here holds, naming register
entry A23. Every other refusal below applies to the FM channels that
remain. Reading is unaffected: a channel of any mode reads normally.

Also refused: a **1750 Hz receive tone**. It is the last entry of the
tone-number chart these radios print and has no entry at all in the
tone-squelch chart, so the program writes it as a transmit tone and
refuses it as a receive one.

On the **TS-590S only**, the **filter** column cannot be set at all,
and channel writes are refused outright on a radio reporting
**firmware 2.00 or later — or a firmware version the program cannot
read**:
the manual guarantees the filter position in a memory record is unused
only on the 1.xx firmware and says nothing about later versions, and one
entry in the model list cannot say "settable above 2.00". A radio whose
firmware answer does not come back in the one form the manual's worked
example prints takes the same conservative branch, because a version
that cannot be compared cannot be shown to be a 1.xx one; the session
still reads normally. The TS-590SG has no such condition and sets the
filter normally.

**A CHIRP file's ordinary rows import on both radios.** A CHIRP file's
blank `Duplex` column means simplex, and these radios declare no shift
vocabulary at all — their memory record carries no duplex selector — but
a blank column asks for nothing they cannot do, so such a row imports as
simplex and nothing is reported. Two kinds of row are still refused: a
`Duplex` column reading `off`, which asserts "no duplex configured" as
distinct from simplex and has nowhere in the record to go, and `CW`,
`CWR` and `RTTY` rows, which resolve to names these radios' own mode
list does not print — it prints `CW`, `CW-R`, `FSK` and `FSK-R`. The
program's own CSV import and export are unaffected. In v1.4.1 and earlier the
blank `Duplex` column was refused too and no row imported at all.

**An imported CHIRP channel is still refused at the write, on values
CHIRP has no column for.** The transmit frequency is no longer one of
them: a blank `Duplex` column is the file's own simplex statement, and
this record expresses simplex by transmitting where it receives, so an
import now carries that. What stays unsaid is the data mode and both
tone numbers on the TS-590S, those plus the filter on the TS-590SG — a
blank `Tone` column says only that tone is switched off. **CHIRP has no
data-mode column and no filter column, so a CHIRP file alone can never
complete a write on either row.** Fill them in before writing, or write
from the program's own CSV, which carries them.

Channels cannot be deleted.

Guesses: its **speed**. Neither Kenwood manual prints a factory rate, so
the program opens at 9600; if your radio is set differently, change it
at the radio's own menu, because the program has no speed setting and
never probes for one. A wrong speed looks exactly like a dead port. The
program also does not offer **4800**, which the radios do: each manual
attaches a condition to that rate that a flat list of speeds cannot
express.

Not shown: the **band edges**. Neither manual prints a frequency range
for these radios, so the program declares none rather than inventing
one, and a frequency out of range is refused by the radio rather than by
the program.

### TS-890S and TS-990S (opt-in)

Read the 100 memory channels and the menu settings: 158 of them on the
TS-890S, 194 on the TS-990S. Tone and scan skip ARE read and written on
these radios, as on the TS-590 pair above: one memory record carries a
tone mode, separate transmit and receive tone numbers and a
channel-lockout flag, and **one frame carries the whole channel**. That
one difference from the TS-590 pair is what removes their largest
refusal — a memory channel read off one of these radios comes back with
its transmit frequency already known, so reading the memories, editing a
name and sending them straight back works. Every mode these radios
publish can be written too: 16 on the TS-890S and 26 on the TS-990S.

**Registering a radio does not mean the program can fill a blank one.**
Refused: **a write to a channel the radio does not already hold**.
Because one frame carries a whole channel, the program reads the channel
it is about to write and refuses when that read comes back blank:
whether the memory-set command can CREATE an unassigned channel is
printed nowhere for that command, while five sibling commands in the
TS-890S manual and four in the TS-990S manual each print an
unassigned-channel prohibition of their own. So the program refuses,
naming register entry A3, rather than creating a channel on an
assumption. Channels the radio already holds are written normally. In
practice: these radios can be re-programmed, not programmed from empty.
The refusal lifts only when somebody sets a channel confirmed blank at
the radio's own front panel and reports what came back.

Also refused: a channel whose **secondary side** on the radio is not
what the program's own frame would write. The record is read and the
secondary side comes back readable — but the program has a source for
the primary side and none for the secondary, and one frame rewrites the
whole record, so before the write it compares the radio's own secondary
parameters against what it would emit and refuses, naming the parameter
and both values, rather than overwriting a side your file never
described. This is a **refusal about a side the program can read**,
which is a different thing from the fields listed as absent below: those
have no position in the record at all.

Also refused: a **1750 Hz receive tone**, on both rows and for the same
reason as on the TS-590 pair — it is the last entry of the tone-number
chart these radios print and has no entry at all in the tone-squelch
chart, so the program writes it as a transmit tone and refuses it as a
receive one.

On the **TS-990S only**, two further refusals, neither of which the
TS-890S's record can even express. A channel with **dual reception**
switched on is refused outright: that flag describes a second RECEIVER
over the channel's second frequency side, nothing in this program's
channel model names such a thing, and one frame rewrites the whole
record — so the write would silently switch the second receiver off. And
a channel whose own answer says it is **section defined** is refused at
the write, naming register entry A8 — reading it is unaffected.

**Slots 100–119 are not offered on either radio**, and both radios have
them. Neither manual prints what selects a section channel's start
frequency and which its end, so a bank of those slots would be a reading
rather than a transcription; the program publishes the 100 ordinary
memory channels and stops there. The codec parses an answer for those
channels if one arrives — it is the *bank* that is not published, not
the frame that is refused.

Not shown: the **band edges**. Neither manual prints a frequency range
for these radios, so the program declares none rather than inventing
one, and a frequency out of range is refused by the radio rather than by
the program.

**Data modes travel in the mode column.** Both radios spell their data
modes into the mode names themselves — `LSB-D`, `USB-D`, `FM-D`, `AM-D`
on the TS-890S, and three numbered sets (`LSB-D1`, `LSB-D2`, `LSB-D3`
and so on) on the TS-990S — so there is no separate data-mode column for
a CSV or a CHIRP file to carry, and a channel's data disposition
survives a round trip inside its mode name. A file that carried a
data-mode column for one of these radios would be describing a field the
record does not have.

**A CHIRP file's ordinary rows import on both radios**, on exactly the
terms the TS-590 pair's do: a blank `Duplex` column means simplex and
these radios declare no shift vocabulary at all, so such a row imports
and nothing is reported. A `Duplex` column reading `off` is refused, and
`CW`, `CWR` and `RTTY` rows are refused on the mode — they resolve to
names these radios' own mode lists do not print, which are `CW`, `CW-R`,
`FSK` and `FSK-R`. **Two values must be filled in before an imported
CHIRP channel can be written, and they are the two tone numbers**: one
frame carries the tone mode, the transmit tone and the receive tone
together, and these radios' write path requires all three to be known,
of which a CHIRP row gives only the mode. Its ordinary blank `Tone`
column says only that tone is switched off, leaving both numbers unsaid,
and nothing here supplies a value the file did not carry. **The transmit
frequency is no longer among them**: a blank `Duplex` column is the
file's own simplex statement, and each record prints that a simplex
channel's split parameters all read zero, so an import now carries that
value. A row reading `Tone` gives the transmit tone and still leaves the
receive tone to you; a row reading `TSQL` is refused on the tone column
outright, because CHIRP's tone squelch asks for a transmit-and-receive
tone mode neither radio's own memory chart prints. The program's own CSV
import and export are unaffected — a CSV read off the radio carries both
already.

**A write may read back as the old value if the radio is displaying
that channel.** Both manuals print it in the same words: "When setting
the channel currently being accessed, the new settings are reflected the
next time that channel is accessed." A verification read of the channel
on the radio's own front panel can therefore show the previous contents;
move off it and read again.

Channels cannot be deleted. Both radios print a dedicated
channel-deletion command and the program does not build it: this program
does not delete a user's channels, and the outbound gate refuses any
frame of that shape besides, so it cannot be sent by accident.

Guesses: its **speed**. No Kenwood manual held here prints a factory
rate, so the program opens at 9600; if your radio is set differently,
change it at the radio's own menu, because the program has no speed
setting and never probes for one. A wrong speed looks exactly like a
dead port.

**Auto Information is switched off on one connector only.** These radios
set it separately per connector, so a session switches it off on the one
it is using and leaves the others as they were — a logger on another
port will not see its own stream stop.

**One recorded observation is not visible in the program at all.** The
TS-890S's simulated radio models a blank channel that still carries a
leftover name, because the manual's blank-channel note stops short of
the name window and says nothing about it; the driver ignores such a
residue rather than treating it as channel content, and notes it on the
driver's own transport log. Nothing in the app or the command line
prints that note today, and nothing will until somebody reads a channel
confirmed blank off a real TS-890S and reports whether the name window
came back blank (the lift recorded as L-HW-4). It is written down here
so the absence is a decision rather than an oversight.

Evidence: `docs/kenwood-models.md`.

### TS-2000, TS-2000X and TS-B2000 (opt-in)

Read and write the memory and scan-edge channels. Tone and scan skip ARE
read and written, unlike the Yaesu radios this programme also supports:
its 50-byte memory record carries a channel-lockout flag and a tone mode
with separate transmit and receive tone numbers. This radio has an
8-character channel name, so a Tag column is shown for it — the only row
in this wave that has one. A channel is written back once its transmit
frequency, DCS code, REVERSE state and memory group are read from the
radio first. The TS-2000X and TS-B2000 answer identically to the TS-2000
over the wire; this program tells them apart only by which one you chose
when you connected. No radio of this family has ever answered a frame
from this project, so every write stays behind the opt-in consent route.

Evidence: `docs/kenwood-models.md`.

### TS-570D and TS-570S (opt-in)

Read and write the memory channels. Tone mode, both transmit and receive
tone numbers, and scan skip are all read and written, unlike the Yaesu
radios this programme also supports. This radio has no channel-name field
over its interface at all, so no Tag column is shown for it. No TS-570 of
either row has ever answered a frame from this project, so every write
stays behind the opt-in consent route.

Evidence: `docs/kenwood-models.md`.

### TS-870S (opt-in)

Read and write the memory channels. This radio has no channel-name field
over its interface at all — NoTag — so no Tag column is shown for it.
Tone mode and transmit tone ARE read and written (one shared index): its
22-byte record has no receive-tone byte at all, so tone_rx cannot travel
over this frame regardless of what you set at the radio. Scan skip is
also read and written. No TS-870S has ever answered a frame from this
project, so every write stays behind the opt-in consent route.

Evidence: `docs/kenwood-models.md`.

### TS-480 (built, not selectable)

The TS-480's driver exists in this program and the radio is **not in the
model list**: you cannot select it. The reason is a question about an
EMPTY channel. The TS-590SG's manual says that reading a memory channel
that has never been written answers with an all-zero record; the 2003
TS-480 manual says nothing about an empty channel anywhere. If a TS-480
rejects that read instead of answering it, a brand-new TS-480 cannot be
read by this program at all, and registering the radio would mean
offering its owner a read that fails on the first channel.

**What would lift it, and it is not one read.** A valid zero record on at
least **three separate channels**, each confirmed unwritten at the
radio's own front panel, across at least **two sessions**, with no
silence and no `?;` among them, every request and answer kept as the
exact bytes that went over the wire, and an **observer named**.
`internal/wiring/testdata/README.md` is written for the person who would
take that observation and says where it goes; a `go test` run is what
checks it, not a judgement on release day.

Evidence: `docs/kenwood-models.md`.

## Sources

Protocol facts come from the makers' published documents, each pinned
by revision in the code that transcribes it.

| Radio | Document |
| --- | --- |
| FT-710 | Yaesu CAT Operation Reference Manual 2306-C |
| FTdx10 | Yaesu CAT Operation Reference Manual 2308-F |
| FTdx101D, FTdx101MP | Yaesu CAT Operation Reference Manual 2308-L |
| FT-891 | Yaesu CAT Operation Reference Manual 1909-C |
| FT-991A | Yaesu CAT Operation Reference Book 1711-D |
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
| IC-7800 | Icom Instruction Manual, section 14 (no separate CI-V Reference Guide) |
| IC-7600 | Icom CI-V Reference Guide |
| IC-7410 | Icom CI-V Reference Guide |
| IC-7700 | Icom CI-V Reference Guide |
| IC-9100 | Icom CI-V Reference Guide |
| IC-7200 | Icom Advanced Instructions manual (no separate CI-V Reference Guide) |
| TS-590S, TS-590SG | Kenwood PC Control Command reference, revision 3 |
| TS-2000, TS-2000X, TS-B2000 | Kenwood PC Control Command reference (TS-2000 series) |
| TS-570D, TS-570S | Kenwood PC Control Command reference B62-1542-00 |
| TS-870S | Kenwood PC Control Command reference B62-1536-00, via the rigpix.com mirror (12/09/2026 provenance widening) |
| TS-480 (built, not selectable) | Kenwood PC Control Command reference, 2003 |
| FTdx5000 | Yaesu CAT Operation Reference Manual, revision 1907-D |

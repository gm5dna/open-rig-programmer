# Hardware notes: live sessions against a real FT-710

This document records the findings of open-rig-programmer's live
sessions against physical FT-710 hardware: M5a (read-only
characterisation, 13/07/2026), M5b (write trials, the same day's
evening — see "M5b write trials" below) and M8c (menu/EX read
characterisation, 24/07/2026 — see "M8c settings read-characterisation"
near the end). It is the authoritative source
the rest of the repository cites (via "HW-CONFIRMED 2026-07-13"
comments) whenever a design decision changed, or was strengthened, as a
direct result of these sessions. Every raw transcript this document
draws on lives, unredacted, in `docs/fixtures-private/` (git-ignored,
never committed); everything below has been through the redaction
process `docs/fixtures.md` mandates — no real callsign, no personal
frequency list, no other operator-identifying data appears here.

Sections up to and including "Consequences applied" record M5a; the
"M5b write-trial protocol" section and everything after it record M5b.

**Code symbols named in this log are historical, not a current API
reference.** Function and type names from `core/` are recorded as they
stood at the session date of each entry, because this document's value
is being an accurate record of what was observed and decided at the
time; entries are not retro-edited when the code later moves. A later
refactor may therefore have renamed or relocated a symbol named here —
M9b, for instance, moved `core/cat`'s builders and parsers off
package-level functions and onto `cat.Dialect` methods, so `cat.BuildMWSet`
and `cat.ParseMTAnswer` below are now `cat.Dialect.BuildMWSet` and
`cat.Dialect.ParseMTAnswer`. Read the names as they were; check the
source for what they are.

## Session metadata

- **Date**: 13 July 2026, at Stuart's home.
- **Radio**: Yaesu FT-710, UK market variant, CAT ID `0800`.
- **Firmware**: V01-12, read from the radio's own SD-card backup files
  (front-panel confirmed as V01-10 or later during the session; the SD
  card files pin the exact version).
- **Host**: macOS, both CP2105 USB-serial stacks available (Apple's
  built-in driver and the SiLabs VCP driver).
- **Controller**: the session was controller-driven throughout — every
  CAT exchange in this document was issued by the automated test
  harness, not typed manually.

### Safety preamble

Before any CAT traffic was sent:

- A full SD-card backup of the radio's configuration was taken and
  verified present, so a card restore was available as a fallback if
  anything went wrong.
- RF output was set to minimum and the radio was on a dummy/low-risk
  antenna path.
- Stuart was present, watching the radio's TX indicator throughout, for
  the entire session.
- No other application had the radio's serial ports open (no port
  contention).

**No TX key-up occurred at any point in this session.** The entire
session was read-only: `MR`, `MT`, `MC`, `ID`, `AI` reads and queries
only. No `MW` (write), no `MT`-set, no `MC`-set was ever issued. This
matters for how the M5a findings are scoped: anything the M5a
sections call "HW-CONFIRMED" is confirmed for the **read** direction
only; at the time of this session the write direction remained exactly
as ASSUMED. (M5b has since run the write trials — see "M5b write
trials" below for the write-direction record.)

## Port mapping

Four CP2105 USB-serial nodes enumerate on macOS for this radio: two
"Enhanced UART" (ECI) nodes and two "Standard UART" (SCI) nodes, one of
each per driver stack.

| Node | Driver stack | Result |
| --- | --- | --- |
| `/dev/cu.SLAB_USBtoUART` | SiLabs VCP | Enhanced UART — CAT works, `ID;` answers `ID0800;` |
| `/dev/cu.usbserial-<device-id>0` | Apple built-in | Enhanced UART — CAT works, `ID;` answers `ID0800;` |
| `/dev/cu.SLAB_USBtoUART5` | SiLabs VCP | Standard UART — fails to configure at 8-N-2 ("invalid argument") |
| `/dev/cu.usbserial-<device-id>1` | Apple built-in | Standard UART — fails to configure at 8-N-1 (termios error) |

Apple's driver names both nodes from the same hex `<device-id>` base
(redacted here — it is derived from the adapter's own USB serial number
and, unlike the empty value the CAT identity probe returns, persists
across reboots and reconnects); the final digit distinguishes the two
CP2105 interfaces on that base, `0` for ECI (Enhanced/CAT) and `1` for
SCI (Standard).

The Enhanced UART (CAT) port answers correctly on **both** driver
stacks, at 8-N-1 and at 8-N-2 (see "Stop bits" below). The Standard UART
(SCI) port is **unopenable on macOS under both stacks, at every tested
configuration** — it never got far enough to exchange a single CAT
frame. This is a genuinely useful safety property: on macOS, opening the
*wrong* port and accidentally keying the radio via a CAT session pointed
at the Standard UART is structurally impossible, because that port
cannot be opened at all. This has not been re-checked on Linux; that
recheck was deferred to M7 (the pre-release Linux hardware session)
and is **still outstanding as of 26/08/2026** — see "Linux hardware:
still pending" under "Explicitly not probed" below.

USB serial number was **empty** in the CAT identity probe on macOS,
under both driver stacks — session identity binding currently falls back
to the port path rather than a USB serial number. Deriving it from the
IOKit node name instead is a parked improvement item for M5b/M7, not
addressed by this task.

**Node names rotate (observed at M8c, 24/07/2026).** The SiLabs stack's
Enhanced-UART node was `/dev/cu.SLAB_USBtoUART` at M5a and
`/dev/cu.SLAB_USBtoUART7` at M8c, for the same physical adapter and the
same radio; its Standard-UART sibling moved from `…5` to plain
`…SLAB_USBtoUART`, so the M5a table's node-to-role mapping held for the
Apple-stack nodes but NOT for the SiLabs ones. The Apple-driver names
(derived from the adapter's own serial) were stable across both
sessions. Practical consequence: a port path is not durable radio
identity, and "the port that worked last time" is not a safe assumption
on the SiLabs stack — `rigprog probe` remains the definitive check.

## Open-time line behaviour

With RTS/DTR left de-asserted at Enhanced-UART open time, **no TX
key-up occurred** — verified live, with RF at minimum and Stuart
watching the TX indicator throughout the open and the full 117-slot
read that followed. The radio's operating state was undisturbed by the
probe and the full read (front-panel spot-checks after the session
matched the pre-session state).

## Protocol findings

### ID probe

```
TX  ID;
RX  ID0800;                                     (4 ms)
```

Fixed 4-character CAT ID `0800`, exactly as the manual documents and as
`core/cat`/`internal/fakeradio` already modelled. No surprises here —
included for completeness, since it was the very first exchange of the
session.

### MR answer: full byte table (M-06, populated, CTCSS on)

M-06 on Stuart's radio is a populated memory channel, FM, minus shift,
with CTCSS ENC/DEC on and a tone of 146.2 Hz **set and active on the
radio** at capture time (front-panel confirmed before the probe).

```
TX  MR006;
RX  MR006029620000+000000411002;                (11 ms)
```

| 1-idx pos | 0-idx bytes | Field | Value | Meaning |
| --- | --- | --- | --- | --- |
| 1-2 | `[0:2]` | Command | `MR` | — |
| 3-5 | `[2:5]` | P1 slot | `006` | M-06 |
| 6-14 | `[5:14]` | P2 freq (9 digits) | `029620000` | 29.620000 MHz |
| 15 | `14` | P3 clarifier sign | `+` | — |
| 16-19 | `[15:19]` | P3 clarifier magnitude | `0000` | 0 Hz |
| 20 | `19` | P4 RX clarifier | `0` | off |
| 21 | `20` | P5 TX clarifier | `0` | off |
| 22 | `21` | P6 mode | `4` | FM |
| 23 | `22` | P7 kind | `1` | Memory |
| 24 | `23` | P8 CTCSS | `1` | ENC/DEC on |
| **25-26** | **`[24:26]`** | **P9 (documented fixed "00")** | **`00`** | **fixed, DESPITE the live 146.2 Hz tone** |
| 27 | `26` | P10 shift | `2` | minus |
| 28 | `27` | Terminator | `;` | — |

**The refutation**: bytes 25-26 (P9) are the documented fixed `"00"`,
even though this exact channel had a CTCSS tone actively set on the
radio at the moment of the read. This directly refutes the
"Hamlib live-tone-index theory" — the idea that P9 might secretly carry
a live CTCSS tone index on some firmware, distinct from the manual's
"always 00" claim. It does not. There is no side channel: **per-channel
CTCSS tone is not readable over CAT on this firmware, full stop.** The
project's existing "Unknown-field-state" design for `CTCSSTone` (never
guessing a value, always reporting it as unreadable) was exactly right
and needed no correction — this finding *vindicates* a design decision
rather than overturning one.

### MT answer: short-form confirmation (M-06, tag redacted)

```
TX  MT006;
RX  MT0061MYCALL      ;                          (real tag redacted — see below)
```

The real answer frame carried the display flag `1` followed by a
genuine 4-character amateur-radio callsign, space-padded by the radio
itself to the full 12-byte tag field (4 tag characters + 8 trailing
spaces = 12 bytes). Per `docs/fixtures.md`'s redaction policy, the
callsign itself must never appear in a committed file; the frame above
substitutes the placeholder tag `MYCALL` (6 characters), padded the same
way (6 characters + 6 trailing spaces = 12 bytes), so that the frame's
byte **shape** — which is the entire point of this finding — is
preserved exactly while the personal content is removed.

| Bytes | Field | Redacted value | Real value (never committed) |
| --- | --- | --- | --- |
| `MT` (2) | Command | `MT` | `MT` |
| `006` (3) | Slot | `006` | `006` |
| `1` (1) | Display flag | `1` (on) | `1` (on) |
| 12 bytes | Tag, space-padded | `MYCALL      ` | 4-char callsign + 8 spaces |
| `;` (1) | Terminator | `;` | `;` |

Two things are confirmed here:

1. **Short-form answer**: the radio's MT answer is the Set-shaped short
   form ("MT" + slot + display + 0-12 byte tag + ";"), exactly what
   `internal/fakeradio` and `core/cat.ParseMTAnswer` already implemented
   — not a Hamlib-style longer combined form.
2. **Trailing-space padding on read**: the radio pads a short tag with
   trailing spaces to the full 12-byte field on read. This was
   previously unknown/ASSUMED. It is now confirmed **for a
   front-panel-origin tag**; whether a tag written via CAT `MT`-set
   (rather than the front panel) comes back similarly padded remains
   open, since this session issued no writes at all — that is an M5b
   question.

### MC query (current-memory)

```
TX  MC;
RX  MC006;                                       (radio sitting on M-06)
```

Confirms the 3-digit-slot-when-on-a-memory case of MC's answer shape.
The separate, more interesting question — whether `MC;` can ever
legitimately answer `MC000;` when the radio is in VFO mode, not on any
stored memory — was **not** directly tested (this session never put the
radio into VFO mode and queried MC while it was there). The AI1-flood
capture's `IF` frames (a different command, not modelled by this
project's codec) showed a `000` channel field while the VFO dial was
being spun, which is suggestive corroboration for "000 means no
stored-memory state" as a general CAT concept, but it is not direct
evidence for MC's own answer shape. This remains open; see
`core/cat/mc.go`'s doc comment.

### AI query

```
TX  AI;
RX  AI0;
```

Matches the manual-derived golden vector exactly — no surprises, useful
as a hardware-derived confirmation nonetheless.

### Empty and out-of-range slots

```
TX  MR010;      RX  ?;      (M-10, never populated)
TX  MT010;      RX  ?;      (immediately after the MR010; above)
TX  MR100;      RX  ?;      (grammatically valid, but out-of-inventory)
```

The `MT010;` -> `?;` exchange above was reproduced twice: once
immediately after `MR010;` (as shown), and again as a completely
standalone exchange, on a different never-touched slot, with no
preceding `MR` at all. Three rejection cases in total, all producing the
byte-identical `?;` frame — the protocol's single, unattributed NAK,
exactly as documented. The important result here is the **MT** case: a
genuinely never-touched slot (no `MW`, no factory data, no prior
`MT`-set) answers `?;` to an MT read, whether or not an MR read has just
been tried on the same slot. This **overturns** the project's former
design assumption, which had MT read of an empty slot *succeed* (display
off, empty tag). `internal/fakeradio`'s `handleMT` now gates its read
reply on whether the slot has ever been touched at all (present in its
internal state map), matching this finding exactly.

### AI1 flood (dial-spinning chatter)

```
TX  AI1;
... [8 seconds of continuous VFO dial movement, no CAT command sent] ...
TX  AI0;
RX  (restored quiet operation)
TX  AI;
RX  AI0;
```

With Auto Information mode on and the VFO dial spun continuously for
8 seconds, **879 unsolicited frames (13,994 bytes, ≈110 frames/second)**
arrived with no command sent at all. Prefix mix: `FA` ×330, `RM` ×272,
`IF` ×243, `FD` ×34. Sample frames (numeric only, no personal content):

```
FA029676600;
IF000029676700+000000421002;
```

`AI0;` cleanly restored quiet operation afterwards (`AI;` -> `AI0;`
confirmed). This is real, sustained, high-rate unsolicited chatter of
exactly the kind `core/transport.Engine`'s drain-to-quiet design
(`DrainToQuiet`, `quarantineAfterWrite`) exists to absorb before trusting
a `Do` call's own answer — the design was validated against real radio
noise, not just synthetic test cases. No transport-level code changed as
a result; this finding is recorded as supporting evidence for an
existing design, in `internal/fakeradio/doc.go`'s new "Real-hardware
behaviour observed at M5a" section.

## Timing

- `MR` answers landed in **10-11 ms flat** over 20 back-to-back
  zero-settle reads (no inter-command delay, no choke observed).
- A coalesced 3-command write — `MR001;MR002;MR003;` sent as a single
  burst — produced three clean, correctly-ordered answers in **26 ms
  total**. The radio pipelines back-to-back requests rather than
  requiring a full round trip before accepting the next command.
- A full 117-slot `ReadAll` (`MR` + `MT` per slot) completed in
  **4.7 seconds** — real-radio pacing closely matches
  `fakeradio`'s own simulated pacing; the timing-budget concern this
  project had going into M5a (that a real radio might be dramatically
  slower than the simulator) is unfounded.

These numbers are recorded as fact (`internal/fakeradio/doc.go`); the
simulator's own timing *constants* are deliberately untouched by this
task (a parked "pacing-knob" item, out of scope here).

## Stop bits

Both 8-N-1 and 8-N-2 work correctly on the Enhanced UART (ECI) port —
two consistent `ID;` -> `ID0800;` exchanges at each configuration. No
code depends on this beyond the existing default (38400 8-N-2, the
manual's stated default, already what `core/driver/ft710/caps.go`
documents); this finding simply confirms that default remains safe and
that 8-N-1 is available as a working fallback if ever needed.

## 60m regional finding

**Stuart's UK FT-710 has no factory 5xx (60m) bank at all**, and no EMG
channel — front-panel confirmed: there are no `5-xx` channels anywhere
in the radio's 117-slot inventory. UK 5 MHz operation on this radio
lives in ordinary memory channels (in Stuart's own programming, the
020-035 range), not a dedicated CAT-visible bank.

This directly refuted `internal/fakeradio`'s `ImageUK` factory image,
which had invented 7 placeholder 60m channels (501-507) at round 20 kHz
steps from 5.260 MHz — a plausible guess that turned out to be wrong for
this variant. `ImageUK` has been regenerated to contain no 60m/EMG
channels at all (see "Consequences applied" below); `ImageUS` is
unchanged and explicitly relabelled STILL-ASSUMED, since M5a
characterised only a UK-variant radio.

`core/driver/ft710.deriveRegion`'s "unknown-0" result — what a
zero-60m/zero-EMG discovery used to be labelled — is renamed to
`"no-60m"`: this is now a **known real variant**, not an anomaly to be
lumped in with genuinely unrecognised inventory shapes.

## `?;` semantics table

| Command | Slot state | Real-radio answer | fakeradio (before this task) | fakeradio (after this task) |
| --- | --- | --- | --- | --- |
| `MR<slot>` | never populated | `?;` | `?;` (ASSUMED) | `?;` (HW-CONFIRMED) |
| `MR<slot>` | grammatically valid, out-of-inventory (e.g. `100`) | `?;` | `?;` (structural, unlabelled) | `?;` (HW-CONFIRMED) |
| `MT<slot>` | never touched at all (no MW, no factory data, no prior MT-set) | `?;` | **succeeded** (display off, empty tag) — WRONG | `?;` (fixed to match) |
| `MT<slot>` | never touched, with a preceding `MR<slot>` that already answered `?;` | `?;` | (as above) | `?;` (fixed to match) |
| `MR`/PMS pair | all PMS slots empty on this radio | `?;` | `?;` | `?;` (unchanged — see "not probed") |
| `MC;` (query) | radio sitting on a memory | `MC<3-digit slot>;` | matches | matches (now HW-derived-vector-pinned) |
| `MC;` (query) | radio in VFO, no stored memory | not directly observed | n/a | n/a — open question |
| `EX<addr>` | syntactically well-formed six-digit read, out-of-inventory (M8c sampled `050101`, `050505`, `010199`, `019901`, `079901`, `999999` — four of which also sit outside the printed P1/P2/P3 ranges) | `?;` | `?;` (ASSUMED by analogy with `MR`) | `?;` (observed for those six; the wider space stays assumed) |
| `EX<addr>` | in inventory | answers `EX<addr><P4>;` | matches | matches (all 296 answered; none was rejected, so `MenuUnavailable` for a KNOWN address stays an unobserved state) |

## Explicitly not probed

Recorded here, honestly, rather than silently assumed:

- **`?;` semantics with the radio in unusual front-panel states**
  (menu screens, etc.) — this session never put the radio into a menu
  screen and issued CAT commands against it. Whether behaviour differs
  from the normal operating-screen state tested here is unknown.
- **ScanSkip readability**: this read-only session never probed whether
  a memory channel's scan-skip flag is readable over CAT by any means.
  `FieldScanSkip.Read` stays `Unsupported` in the capability model
  (`core/driver/ft710/caps.go`'s `bankFields` comment) purely on
  design-time reasoning — the fully-specified 28-byte `MR`/`MW` frame
  has no byte position left for it, every position already accounted
  for by frequency/clarifier/mode/kind/CTCSS/P9/shift — not because a
  live probe confirmed no side channel exposes it. Unlike
  `FieldCTCSSTone.Read` (HW-CONFIRMED unreadable this session, see "MR
  bytes 25-26" above), ScanSkip's `Unsupported` status remains
  structural only. Deferred to M5b/a future session that might reveal a
  side channel this project has not yet looked for.
- **PMS wire-form validity vs emptiness, conflated in `?;`**: every PMS
  slot (`P1L`-`P9U`) on Stuart's radio is empty (`MRP1L;` -> `?;`, etc.),
  so this session could not distinguish "the PMS wire form itself is
  invalid" from "this specific PMS pair merely has no data stored" — the
  radio's single unattributed NAK does not let you tell those apart from
  the outside. This cross-check is deferred to M5b's write trials (write
  a PMS pair, then confirm reads behave as expected).
- **MC-set (recall) of an empty slot**: this session only exercised the
  MC *read* query (`MC;` -> `MC006;`); MC-set was never issued (no
  writes at all this session). `internal/fakeradio`'s existing "MC-set
  of an empty slot answers `?;`" rule remains ASSUMED, by analogy with
  the now-confirmed MR rule, pending an actual write session.
- **Whether a CAT-`MT`-set tag comes back padded the same way** a
  front-panel-set tag does (see "MT answer" above) — no writes were
  issued this session.
- **Linux port-mapping recheck** — the "wrong-port TX-key structurally
  impossible" finding is macOS-only; deferred to M7's pre-release Linux
  hardware session, which as of 26/08/2026 has still not run (see the
  next subsection).

### Linux hardware: still pending (annotation added 26/08/2026)

Every session recorded in this document was run on macOS. **No FT-710
has ever been connected to a Linux machine by this project**, so the
M7 deferrals above remain open: the port-mapping recheck, which
`/dev/ttyUSB*` node is the CAT-capable Enhanced UART, whether the
`dialout` group step suffices in practice, and whether the packaged
udev rule keeps ModemManager away from the radio.

What *has* been checked on Linux is the packaging, not the radio: the
release build's Debian packages were installed and launched on clean
Ubuntu 24.04.4 desktop virtual machines on both architectures on
23/08/2026, with no radio attached. That evidence — what it covers and
what it does not — is written up in `docs/linux-setup.md`'s Status
section, deliberately kept out of this file, which records real-radio
sessions only.

## Consequences applied

| Finding | Code/doc change |
| --- | --- |
| Empty-slot `MR` -> `?;` | `internal/fakeradio/doc.go` register item 2, `core/driver/ft710/doc.go` quirk 1: ASSUMED -> HW-CONFIRMED |
| Out-of-inventory `MR100;` -> `?;` | `internal/fakeradio/parser.go` (`rejection`, `handleMR` comments): HW-CONFIRMED; `core/cat` HW-derived `IsRejection` vector |
| `MT` on a never-touched slot -> `?;` (overturns prior design) | `internal/fakeradio/parser.go` `handleMT` behavioural fix (TDD, RED->GREEN); `doc.go` items 4/5 rewritten |
| MR006 bytes 25-26 fixed "00" despite a live CTCSS tone | `core/cat` HW-derived `TestParseMRAnswer_HWDerived_M06`; `core/driver/ft710/caps.go` `FieldCTCSSTone.Read` pinned Unsupported with an HW-cited test; `core/driver/ft710/doc.go` quirk 7 settled |
| MT short-form answer + trailing-space padding on read | `internal/fakeradio/parser.go` `buildMTReply` comment (already-correct behaviour, now HW-cited); `core/cat/mt.go` `ParseMTAnswer` comment; `core/cat` HW-derived `TestParseMTAnswer_HWDerived_M06_ShortForm`; `core/driver/ft710/doc.go` quirk 5 |
| MC006 (3-digit slot on a memory) | `core/cat/mc.go` doc note; `core/cat` HW-derived `TestParseMCAnswer_HWDerived_M06` |
| AI1 flood characterisation | `internal/fakeradio/doc.go` new "Real-hardware behaviour observed at M5a" section (documentary; `core/transport`'s drain design untouched) |
| Timing facts (10-11 ms, 26 ms coalesced burst, 4.7 s ReadAll) | Same `internal/fakeradio/doc.go` section (documentary; timing constants untouched) |
| Stop bits (8-N-1 and 8-N-2 both work) | This document only — confirms the existing 38400 8-N-2 default remains safe |
| macOS port mapping (Enhanced works both stacks; Standard unopenable) | This document only — no code change; a structural safety property, not a design decision |
| Open-time RTS/DTR (no TX key-up) | This document only — confirms an existing design property holds on real hardware |
| 60m regional finding (no 5xx bank, no EMG on the UK variant) | `internal/fakeradio/image.go` `ImageUK` regenerated (no 60m/EMG); `ImageUS` relabelled STILL-ASSUMED; `core/driver/ft710.deriveRegion` gains the `"no-60m"` label; repo-wide test sweep (fakeradio, ft710, cmd/rigprog, core/clone, internal/wiring) |
| ScanSkip readability | Not probed this session; `core/driver/ft710/caps.go` `FieldScanSkip.Read` stays `Unsupported`, structurally (frame-layout) reasoned only, not hardware-confirmed. Recorded as explicitly-not-probed above |
| PMS wire-form/emptiness conflation, MC-set-of-empty, menu-state `?;`, CAT-MT-set padding | Recorded as explicitly-not-probed above; nothing changed at M5a. CAT-MT-set padding has SINCE been settled — first at M5b (§Empty-slot create, tag-clear: CAT-written tags read back space-padded) and then the hard way by the 13/07/2026 production write (tag-normalisation fix wave: `cat.ParseMTAnswer` now trims padding at parse; the 0-byte MT-set was additionally proven REJECTED, so `cat.BuildMTSet` emits the all-spaces clear form for an empty tag) |

## M5b write-trial protocol

> **STATUS: EXECUTED, 13/07/2026 evening.** Every step below ran, in
> order, against the nominated sacrificial channel; per-step outcomes
> are recorded under "M5b write trials" ("Protocol execution
> outcomes"), and the full
> findings follow it. The protocol text itself is preserved as written
> beforehand — it is the record of what the session committed to BEFORE
> touching the radio.

M5a was deliberately read-only; nothing in the M5a record licenses a
single `MW`, `MT`-set, or `MC`-set. This section is the durable,
committed record of HOW M5b's write trials must run, so the safety
protocol survives between milestones rather than living only in a
session's working memory. It does not itself authorise anything —
`writeTrialsComplete` (`core/driver/ft710/caps.go`) stays `false` until
the matrix below has actually run clean and the M5b PR says so with
evidence.

- **Sacrificial channel only.** Every trial in this protocol targets a
  single memory channel Stuart nominates as sacrificial before the
  session starts — never M-01 (required, never blanked), never a
  channel holding real operating data. No write of any kind touches any
  other slot.
- **Baseline before any write.** Before the FIRST write of the session,
  the sacrificial channel is inspected and recorded three ways: read on
  the front panel, read fresh over CAT (`MR`/`MT`), and confirmed
  present in a freshly-taken SD-card backup. This baseline is what a
  restore falls back to.
- **One field-class at a time.** Trials proceed field-class by
  field-class, never combined, so a failure implicates exactly one
  thing:
  - all mode nibbles, across bands (LSB/USB/CW/CW-R/AM/FM/etc., per the
    manual's kind table, on frequencies where each is legal);
  - clarifier values and step (including the sign byte and the 10 Hz
    step boundary);
  - shift (simplex/plus/minus);
  - every CTCSS state value (`OFF`/`ENC`/`ENC-DEC`/etc.);
  - tags, including edge characters (space, the ASCII boundary the
    manual documents) and the full 12-byte length limit;
  - the tag-display flag;
  - a PMS slot (`P1L`-`P9U`) — separately from the MEM-bank trials,
    since PMS is a distinct bank with its own NoBlank rule;
  - an empty-slot write (populating a currently-blank slot from
    nothing).
- **TX indicator observed on every single write** — not sampled, not
  assumed quiet from a prior write in the same session. Stuart watches
  the radio's TX indicator for every `MW`/`MT`-set/`MC`-set this
  protocol issues, exactly as the M5a safety preamble required for the
  read-only session.
- **Hidden-field preservation checks (the core open question).** CAT
  cannot read CTCSS tone or ScanSkip (see "Explicitly not probed" and
  `FieldCTCSSTone`/`FieldScanSkip` above), so preservation cannot be
  confirmed by reading back over CAT — front-panel/SD verification is
  the only channel available:
  1. Set a CTCSS tone and a ScanSkip flag on the sacrificial channel
     from the front panel (not via CAT — CAT cannot write them either).
  2. Issue an `MW` that rewrites an UNRELATED field on that same
     channel (e.g. the tag) — mirroring the real risk this protocol
     exists to test: a normal, unrelated write via this project's own
     write path.
  3. Verify on the front panel (never via CAT) whether the tone and
     ScanSkip flag set in step 1 survived that write.
  4. Record the result plainly, whichever way it comes out — this is
     the finding M5a could not obtain and the GUI's tooltip wording
     depends on (see `app/frontend/src/lib/ChannelGrid.svelte`).
- **Erase probing last.** Any erase-shaped trial (all-zero `MW`, or
  whatever the manual/observed behaviour suggests blanks a slot) runs
  only after every other field-class above has completed clean on the
  sacrificial channel, since erase is the one trial class that could
  destroy the very channel every other check depends on.
- **MC operating-state snapshot/restore around any recall.** Before
  issuing any `MC`-set (recall), record the radio's current
  operating-frequency/mode/memory-vs-VFO state, and restore it
  afterwards — a recall trial must not leave the radio parked somewhere
  unexpected for the next step in this protocol.
- **Immediate `MR`+`MT` readback after every write** — every single
  `MW`/`MT`-set is followed immediately by an `MR` and `MT` of the same
  slot, before moving to the next trial, so a divergence is caught at
  the write that caused it, not discovered later.
- **Abort on the first anomaly.** Any result that does not match
  prediction — an unexpected answer, an `?;` where data was expected, a
  front-panel state that does not match what CAT reports, a TX
  indication that should not have happened — stops the session
  immediately. No further trials are attempted "to see if it happens
  again."
- **Restore from SD if anything is doubtful.** If the session aborts,
  or if there is any doubt at all about the sacrificial channel's (or
  the radio's) state afterwards, the session ends with a full SD-card
  restore from the baseline backup, not a manual "put it back" attempt.

**Findings from this matrix update `spec.FieldSupport` flags** (the
per-field Read/Write support the capability model exposes) — a trial
result is the ONLY thing permitted to move a field from `Unverified`
toward `Supported`/`Unsupported` in the RealHardware profile. The write
guard, `writeTrialsComplete` (`core/driver/ft710/caps.go`), flips from
`false` to `true` ONLY once this entire matrix has completed clean, in
the M5b PR itself, with the hardware evidence linked (per
`writeTrialsComplete`'s own doc comment).

This protocol is an M5b-flip precondition IN ADDITION TO, not instead
of, the write-capability-split decision already ledgered in
`.superpowers/sdd/progress.md` (the M5b-FLIP PRECONDITION review item
from the Codex M3 adjudication — "full write-capability split decision
before flipping writeTrialsComplete"): both must be satisfied before
`writeTrialsComplete` may change.

## M5b write trials (13/07/2026 evening)

M5b ran the write-trial protocol above, the same day as M5a, against the
same radio and host, controller-driven throughout (every exchange issued
by the automated harness with a hard slot allowlist of {095, 096, P1L,
P1U} plus the consented one-off on M-06 noted below; journal-before-send
to the private transcript; readback after every write; stop-on-anomaly).
Raw transcript (PRIVATE, never committed):
`docs/fixtures-private/m5b-trials.private-capture`.

### Session metadata

- **Date**: 13 July 2026, evening — continuing M5a's session day.
- **Radio/host/port**: as M5a (UK FT-710, CAT ID `0800`, firmware
  V01-12, macOS, Enhanced UART).
- **Sacrificial channel**: **M-95**, nominated by Stuart before the
  session — a broadcast-listening memory (a BBC World Service relay,
  11.685 MHz AM, label "BBC ANT 3"), easily re-created by hand if
  anything went wrong. Not a personal frequency: a public broadcast
  outlet, reproduced here under the same reasoning as M5a's M-06
  radio parameters (the label names the BBC, not the operator).
  Secondary trial slots: 096 (empty-slot create), P1L/P1U (PMS pair).
- **Safety**: SD-card backup current (baseline confirmed); full CAT
  read digest recorded before the first write; RF minimum; TX indicator
  watched on every write; **no TX key-up occurred at any point**.

### Protocol execution outcomes

| Protocol step | Outcome |
| --- | --- |
| Sacrificial channel only | HELD — writes confined to M-95/096/P1L/P1U, plus ONE consented, pre-agreed deviation: a same-data rewrite of M-06 for the tone-preservation check (see below). No other slot was written. |
| Baseline before any write | DONE — front-panel read, fresh CAT `MR`/`MT`, and SD backup all recorded before the first write. |
| One field-class at a time | DONE — frequency, mode nibbles, CTCSS states, shift, clarifier class, tags/display, PMS, empty-slot create, each isolated (transcript ordering). |
| TX indicator observed on every write | DONE — no TX key-up, all session. |
| Hidden-field preservation checks | TONE: PROVEN preserved (see below). SCAN-SKIP: NOT PROBED — the front-panel menu flow for setting/verifying it defeated the session. |
| Erase probing last | DONE — the all-zero-frequency `MW` probe ran after every other class, and was REJECTED (no CAT erase; see below). |
| MC snapshot/restore around recalls | DONE — `MC` query snapshot, `MC001;` recall, restore; all round-tripped fire-and-forget. |
| Immediate `MR`+`MT` readback after every write | DONE — every write in the transcript is followed by its readback. |
| Abort on first anomaly | NOT TRIGGERED — no unexplained anomaly occurred (the two production-bug discoveries below were resolved empirically in-protocol, kind-first probing having been pre-planned). |
| Restore from SD if doubtful | NOT NEEDED — everything restored over CAT and verified byte-identical (one non-writable exception, P7, below). |

### MW/MT set semantics (timing and acknowledgement)

- `MT` set: fire-and-forget **silent accept** — CONFIRMED (was
  ASSUMED). Tag and display flag round-trip exactly; tag-clear via an
  all-spaces 12-byte tag works; the radio's 12-byte padding behaviour
  matches M5a's read-side finding.
- `MW` set: **silent accept** on success. Rejection is an **IMMEDIATE
  `?;` — ~10 ms**, not delayed: the transport's bounded error-window
  design is validated, and the window can be short.

### The P7 (kind) matrix — and the two production bugs it caught

What was sent × what the radio did (live frames, redaction-clean):

| Frame | Slot bank | P7 sent | Radio's answer |
| --- | --- | --- | --- |
| `MW095011685000+000000510000;` | MEM | `1` (Memory) | accepted (silent) |
| `MW095011690000+000000500000;` | MEM | `0` (VFO) | **rejected** (`?;`, immediate) |
| `MWP1L007100000+000000110000;` | PMS | `1` (Memory) | accepted (silent) |
| `MWP1L007100000+000000150000;` | PMS | `5` (PMS) | **rejected** (`?;`, immediate) |

**P7 on a write MUST be `1` (Memory), for MEM and PMS slots alike.**
The manual's own worked example frame (P7=0) is wrong on hardware, and
its implied PMS pairing (P7=5 for a PMS slot) is wrong too — the
project's founding golden-vector distrust is hardware-vindicated.

**Production bug 1 (write)**: `core/driver/ft710/write.go` sent
KindPMS (`5`) for PMS-slot writes (the ASSUMED pairing) — every
real-radio PMS write would have failed. Fixed: write always sends
KindMemory; `cat.BuildMWSet` and fakeradio enforce/mirror the same.

**Production bug 2 (read)**: a populated PMS channel **reads back kind
`1`** (as CAT-written), and the driver's MR validation demanded `5` for
every PMS slot — aborting whole reads (`rigprog read`, GUI ReadRadio,
clone PrepareSend) on any radio with a populated PMS pair. Reproduced
live: `ft710: MR answer for slot "P1L" carries kind '1', want '5' for
this slot's bank`. Fixed (adjudicated): the write side always sends
`1`; the read side accepts a lenient documented set — {`0`,`1`,`4`} for
memory-bank slots, {`1`,`5`} for PMS (a front-panel-created PMS
channel's kind byte was never observed — see "not probed") — and a
read kind is never re-emitted on write.

P7's full semantics remain partially murky: MEM channels have been
observed reading BOTH `0` and `1` (front-panel-created), only `1` is
writable, and the `0` state is **not recreatable via CAT** — M-95's
original P7=0 could not be restored (see "Restoration" below).

### Clarifier: silently ignored on write

MW frames carrying non-zero clarifier values and Rx/Tx clarifier flags
were **accepted** (no rejection) and **ignored**: every readback showed
zeros (live pairs: `MW095011685000+010010510000;` and
`MW095011685000-025011510000;`, each reading back `+000000`). The
project's write-then-verify design would have caught this in production
as a verify mismatch — the design is vindicated — but honest
capabilities prevent the abort entirely: `FieldClarifier`'s Write is now
`spec.Inert` ("transmitted but ignored") in every profile, and
`codeplug.Diff` blocks a CHANGED clarifier with a precise reason while
letting unchanged values flow. Unobserved within this finding: every
probed channel's STORED clarifier was already zero, so
"ignores-and-preserves" vs "zeroes-on-write" is indistinguishable in the
data; and out-of-range clarifier values were never sent.

### Tone preservation: PROVEN (consented M-06 deviation)

The one open question M5a could not answer: does an MW rewrite destroy
the CAT-invisible per-channel CTCSS tone? Answered with a consented,
pre-agreed one-off deviation from the sacrificial-channel rule: M-06
(M5a's byte-table channel, with CTCSS 146.2 Hz stored and active)
was rewritten with its OWN just-read data (same-data `MW`+`MT`), and the
stored tone was verified INTACT on the front panel afterwards.
**Per-channel tone storage is decoupled from the MW payload**;
tone-bearing channels are safe to rewrite, and the project plan's
contingency for the opposite outcome (blocking rewrites of tone-bearing
channels, had preservation failed) is confirmed unnecessary — it was
never built, and now never needs to be. (Changed-field preservation is
inferred from same-data preservation; scan-skip preservation remains
NOT probed.)

A related interaction finding from the same check: **front-panel edits
on a recalled memory do NOT auto-commit** to the stored channel (the
radio's own M►W flow is required), and a CAT write to a channel
clobbers any uncommitted front-panel operating state on it.

### Selection side-effect: MW moves the radio's selection

An accepted `MW` **moves the radio's selection to the written slot**,
hands-off (confirmed watching the radio) — a bulk send drags the
selection through every written channel. `MC`-set recall and restore
round-trip cleanly (`MC001;` set, `MC;` query, restore — all
fire-and-forget). Consequence applied: the clone service's Execute now
snapshots the current selection before its first write and best-effort
restores it afterwards (`core/clone`, obligation 12; journal events
`mc_snapshot`/`mc_restore`).

### Empty-slot create, tag-clear

`MW` to an empty slot (096) **creates** the channel; the fresh channel
reads back `MT` display flag `0` with an all-spaces 12-byte tag, and
clarifier zeros. Tag-clear via an all-spaces `MT` set works — and the
CAT-`MT`-set-then-read padding question M5a left open is settled along
the way: tags written via CAT read back space-padded to the full
12-byte field, exactly like front-panel-origin tags.

**Follow-up probe (13/07/2026, stages `tagclear`/`tagclear2`, same
private capture): the ZERO-byte-tag `MT` set (`MT0960;`) is REJECTED
(`?;`, ~4 ms) and the existing tag survives — the all-spaces 12-byte
tag is THE (one proven) tag-clear mechanism, not merely *a* working
form.** Consequence applied: `cat.BuildMTSet` encodes an empty tag as
the all-spaces clear form and never emits the rejected 0-byte frame;
`internal/fakeradio` mirrors the rejection (its former acceptance of
the 0-byte set was a proven divergence). See
`core/cat/hw_derived_m5b_test.go`
(`TestBuildMTSet_HWDerived_ZeroByteFormNeverEmitted`) and `core/clone`'s
`TestExecute_TagClear_EndToEnd`.

### No CAT erase (confirmed by a non-confounded re-probe)

The original finding here rested on a SINGLE probe frame,
`MW096000000000+000000010000;`, which was **rejected** (`?;`) and read,
at the time, as proof that no CAT erase mechanism exists. That frame was
**confounded**: it carried a zero frequency (P2, positions 6-14) AND
mode nibble `0` (P6, position 22). Mode `0` IS a documented P6 value —
the CAT manual's own table row reads "0:-", i.e. *unset*, and this codec
models it as `cat.ModeUnset` — but it is a value the builders reject when
constructing a Set frame (a channel cannot be written with no mode), and
Batch 9's mode-nibble sweep (recorded below) exercised only the fifteen
*writable* nibbles `1`-`F`, never `0`. So the radio's rejection of that
frame proved only that AT LEAST ONE of "zero frequency" or "mode = unset"
is refused on a write — not, specifically, that an erase-shaped write is
refused. (An earlier revision of this section asserted that mode `0` was
"not a valid Mode at all". That was itself an overclaim, caught in review
on 13/07/2026, and is corrected here: the conclusion below does not rest
on it.)

**Re-probe (13/07/2026, later the same day, after Batch 9): four
range/mode-isolated candidate MW frames**, same radio/host/day, same
sacrificial-channel protocol (M-95), each frame valid in every OTHER
respect (a real, in-table mode nibble sent wherever a mode was sent at
all, correct slot/kind bytes):

| Candidate | What it isolates | Result |
| --- | --- | --- |
| All-zero frame (freq 0, mode `0`) — the ORIGINAL probe, repeated for continuity | Neither confound removed | REJECTED (`?;`) |
| Zero frequency, valid mode (`1`) | Removes the invalid-mode confound | REJECTED (`?;`) |
| Sub-minimum frequency (below the radio's lowest receivable frequency), valid mode | Tests whether "exactly zero" is special, or any too-low value is | REJECTED (`?;`) |
| Out-of-band-high frequency (above the radio's highest receivable frequency), valid mode | Tests the upper bound symmetrically | REJECTED (`?;`) |

All four were rejected. The consistent picture across all four, plus
Batch 9's mode sweep: **the radio range-checks frequency on every MW,
rejecting anything outside its normal receive range regardless of
intent** — there is no secret sentinel frequency/mode combination that
erases a channel instead of writing (or refusing) one. This properly
ISOLATES the confound the first probe left open and confirms, rather
than merely suggests: **no CAT erase mechanism exists.** The project's
Erased→Blocked design is CORRECT, permanently — not merely unverified,
and no longer resting on one ambiguous trial.

A front-panel erase flow was not sought during M5a/M5b's read-only/write
sessions; it has SINCE been confirmed from the FT-710 operation manual —
see "Front-panel procedures: erase and scan-skip" immediately below.

### Front-panel procedures: erase and scan-skip (FT-710 operation manual)

Two front-panel flows this project cannot reach over CAT at all, both
confirmed from the FT-710 operation manual rather than any live probe —
there is no wire traffic to isolate for either, since CAT has no command
for them:

- **Erase a channel.** (Operation manual, section "Erasing Memory
  Channel Data" — manual editions paginate differently, so the section
  title is cited rather than a page number.) The FT-710 has no CAT
  erase command (see above). To delete a channel on the radio itself:
  press and hold **[V/M]** to open the memory channel list, select the
  channel, then touch **[ERASE]**. This is the procedure the GUI (the
  send-review dialogue's Blocked section, and the delete-confirmation
  dialogue) and the CLI (`rigprog write`'s blocked-only report) both
  surface whenever an erase is blocked — the working file can go ahead
  and mark a channel empty, but making the RADIO agree needs this
  front-panel step; nothing sent over CAT can do it.
- **Scan-skip a channel.** (Operation manual, section "Scan Skip
  Setting".) `FieldScanSkip`
  stays `Read`/`Write` Unsupported project-wide (see "Explicitly not
  probed at M5b" below) — this closes the EXPLANATION gap, not the probe
  gap: the flag is set via press-and-hold **[V/M]** to open the memory
  list, select the channel, touch **[SCAN MEMORY]**, rotate to **SKIP**,
  then press the tuning knob to confirm; a skipped channel is marked
  with an **X** in the memory list. Nothing here is readable or writable
  over CAT — recorded so the front-panel flow is at least documented,
  even though this project still cannot drive or verify it by any other
  means.

### Restoration

- M-95 restored byte-identical to its baseline — with ONE exception the
  protocol could not avoid: its original P7 byte read `0`
  (front-panel-created state) and only `1` is writable via CAT, so the
  restored channel reads P7=`1`. Every other byte identical; the
  channel is functionally indistinguishable. Batch 9 (below) later
  exercised M-95 far more heavily (a full mode-nibble sweep, four tag
  boundary sets) and independently re-confirmed this SAME restoration
  at the end of its own run.
- A full scan of every other channel: byte-identical to the morning
  baseline.
- **Trial artefacts left on the radio pending manual cleanup**: M-96
  (7.030 MHz, blank tag — the empty-slot-create trial) and P1U
  (7.200 MHz, blank tag — the batch-9 PMS-pair-creation trial). No CAT
  erase exists (above), so they must be removed from the front panel.
  P1L (7.100 MHz — the original PMS trial) carries no ADDITIONAL
  artefact beyond what was already true here: batch 9 exercised further
  tag/mode/CTCSS/shift changes on it and then explicitly reset every one
  of them back (blank tag, mode LSB, CTCSS off, shift simplex) — see
  "Batch 9" below.
- No TX key-up at any point, all day.

### Batch 9: supplementary trials closing the mode/tag/PMS gap (13/07/2026, later the same evening)

Codex's M5b milestone review (finding #2, adjudicated HIGH) audited the
raw transcript against the committed protocol above and found the FIRST
evidence pass fell short of "every applicable mode nibble", full-length
tag boundaries, and PMS coverage: accepted `MW` writes existed only for
modes `1`, `3`, `4`, `5`; mode `2` appeared only in a REJECTED P7=`0`
frame (the kind-pairing sweep, not a mode trial); no `6`-`F` write had
been sent at all; the sole PMS success was LSB/OFF/simplex with no PMS
MT-set; and the upper-ASCII tag boundary was never sent. The controller
resolved this with a SUPPLEMENTARY session, same radio/host/day, same
allowlist and safety protocol (sacrificial M-95/096/P1L/P1U,
journal-before-send, readback-after-every-write, stop-on-anomaly) —
closing the evidence gap rather than downgrading the "Full matrix
clean" claim.

- **Mode nibble sweep (M-95).** All 15 applicable mode nibbles (`1`-`F`
  — CAT's mode table, `core/cat/mode.go`) written in sequence, each
  drawing a silent accept: no `?;`, and this session had already
  hardware-validated that a rejection is IMMEDIATE (~10 ms — see "MW/MT
  set semantics" above), so silence at that latency is a reliable
  accept signal, not merely an absence of evidence. `1`, `2`, `3`, `4`,
  and `5` are additionally round-trip-CONFIRMED via an explicit MR
  read-back (`1`/`4`/`5` on MEM across both batches; `2` and a further
  `4` change on PMS, below); the sweep's terminal value, `F`, is
  likewise round-trip-confirmed via the MR read-back that follows the
  very next (tag) write. `6`-`E` were accepted with no rejection but not
  individually read back before the next nibble overwrote them —
  consistent with every OTHER confirmed nibble and the session's own
  validated accept/reject mechanism, but not independently proven
  nibble-by-nibble.
- **Tag boundary sets (M-95).** Four 12-byte tags, each set via `MT`
  and immediately read back byte-exact: all-`Z` (`ZZZZZZZZZZZZ`),
  all-`0` (`000000000000`), mixed alphanumeric-with-punctuation
  (`A-1/B+2 C&3?`), and heavy punctuation (`!#$%&'()*+,-`) — the last
  of these is the upper-ASCII-boundary set the first batch never sent.
- **PMS pair (P1L/P1U).** P1L's tag-set (display flag ON, tag "PMS TAG
  TEST") accepted and read back byte-exact — the PMS MT-set the first
  batch never exercised. P1L's mode (`1`→`2`), CTCSS state (`0`→`1`),
  and shift (`0`→`1`, PLUS) each changed and each independently read
  back correct — the PMS mode/CTCSS/shift coverage the first batch
  never exercised either. P1U created (kind `1`, mirroring P1L's own
  creation) and read back confirming an empty tag/display, exactly like
  P1L's own creation in the first batch.
- **Restoration (batch 9).** M-95 explicitly rewritten back to its
  baseline (mode AM, tag "BBC ANT 3") and read back matching; P1L's
  mode/CTCSS/shift explicitly reset to `1`/OFF/simplex and its tag
  cleared (blank, display off) — see "Restoration" above for what this
  batch does and does not leave behind.

Raw transcript (PRIVATE, same file, later timestamps):
`docs/fixtures-private/m5b-trials.private-capture`. HW-derived vectors:
`core/cat/hw_derived_m5b_test.go`
(`TestBuildMWSet_HWDerived_M5b_Batch9_HighNibbleMode`,
`TestBuildMTSet_HWDerived_M5b_Batch9_TagBoundariesAndPMS`).

### Explicitly not probed at M5b

Recorded honestly, superseding M5a's "Explicitly not probed" list
where they overlap:

- **Scan-skip preservation across an MW rewrite** — the front-panel
  menu flow for setting/verifying scan-skip defeated the session.
  `FieldScanSkip` stays Unsupported/unprobed. The flow itself is NO
  LONGER undocumented, though: "Front-panel procedures: erase and
  scan-skip" above records it from the operation manual (section "Scan
  Skip Setting"), closing
  the long-standing explanation gap for why this project could not find
  it live — a future session can now deliberately reproduce the
  documented sequence rather than search for it from scratch. That is a
  documentation fix only; CAT readability/writability is unchanged and
  still unprobed.
- **Front-panel-created PMS channels' kind byte** — every PMS channel
  observed was CAT-created (reads back `1`); whether the front panel
  creates PMS entries with kind `5` is unknown, which is why read
  acceptance stays lenient ({`1`,`5`}).
- **MC-set (recall) of an EMPTY slot** — still never issued; the
  reject-with-`?;` model remains ASSUMED (fakeradio register item 3).
- **P7's full semantics** — `0` vs `1` on front-panel-created MEM
  channels, and what (if anything) distinguishes them; `2`/`3`/`4`
  were never sent on a write.
- **MEM read kinds `2`/`3`** — never observed on a real read; MEM's
  `acceptedKinds` stays the documented lenient set ({`0`,`1`,`4`} — see
  `core/driver/ft710/read.go`'s Fix 5, Codex M5b fix wave) rather than
  widening it speculatively.
- **Clarifier edge cases** — whether an MW preserves or zeroes a
  NON-zero stored clarifier (none existed to probe), and whether
  out-of-range clarifier values are rejected on the wire.
- **`MC;` in VFO state** ("MC000;" hypothesis) — still not directly
  observed; the clone service's selection snapshot deliberately SKIPS
  its restore rather than guess when the answer does not parse.

### Consequences applied (M5b)

| Finding | Code/doc change |
| --- | --- |
| No CAT erase, RE-CONFIRMED non-confounded (13/07/2026 re-probe, later than Batch 9) | Superseded the single confounded all-zero-frequency probe with four range/mode-isolated candidates, all rejected — see "No CAT erase (confirmed by a non-confounded re-probe)" above. Consequence: GUI (`app/frontend/src/lib/SendFlowDialog.svelte`, `DeleteConfirmDialog.svelte`) and CLI (`cmd/rigprog/write.go`) both stopped reporting a blocked-only send plan (e.g. a pending delete) as the false-parity "Nothing to send." — a blocked-only plan now opens an honest informational state (GUI) / exits `exitBlocked` naming the slot, reason, and this section's front-panel erase procedure (CLI) |
| P7 must be `1` on every MW (bug 1) | `core/driver/ft710/write.go` always sends KindMemory; `core/cat/mw.go` validateMWFields accepts only KindMemory; fakeradio rejects MW kind `0`/`5` (`mwKindAccepted`); HW-derived vectors in `core/cat/hw_derived_m5b_test.go` |
| Populated PMS reads back kind `1` (bug 2) | `core/driver/ft710/read.go` lenient `acceptedKinds` ({`0`,`1`,`4`} MEM-like, {`1`,`5`} PMS); `KindMismatchError.Want` now carries the accepted set; the live P1L frame is a passing vector |
| MW rejection is immediate (~10 ms) | Documentary — validates the transport's bounded error-window design; no constant changed |
| Clarifier ignored on write | New `spec.Inert` support state; `FieldClarifier` Write→Inert in all profiles (`core/driver/ft710/caps.go`); `codeplug.Diff` Inert changed-value gate; fakeradio stores clarifier zeros (register item 20) |
| Tone preservation proven | GUI tone tooltip now states preservation is hardware-verified; scan-skip tooltip stays unverified (`app/frontend/src/lib/ChannelGrid.svelte`) |
| MW moves selection | `core/clone` obligation 12 (MC snapshot/restore around Execute's delta loop); `*ft710.Session.CurrentMemory`/`RecallMemory`; fakeradio tracks selection on MW |
| Empty-slot create works | Documentary — matches existing modelling |
| No CAT erase | Erased→Blocked confirmed; block-reason/comment wording updated from "until hardware verification" to permanent (`core/codeplug/diff.go`, `core/driver/ft710/write.go`) |
| Full matrix clean | `writeTrialsComplete` flips to `true` in this PR (`core/driver/ft710/caps.go`), with the RealHardware profile's verified write set — see that file for the field-by-field evidence citations. Codex M5b review (finding #2, adjudicated HIGH) caught this claim outrunning the first evidence pass for mode nibbles, tag boundaries, and PMS coverage; the batch-9 supplementary trials above closed the gap, so the claim is now true, not merely restated |

## M8c settings read-characterisation (24/07/2026)

The menu/EX read path's first contact with real hardware. Everything in
this section is **read-direction evidence from one radio, one firmware
version and one configuration, in two sweeps minutes apart** — it is not a
verified property of the FT-710 as a model, and it says nothing whatever
about EX **Set** behaviour, which this session did not probe.

### Session metadata

- **Date**: 24 July 2026, at Stuart's home.
- **Radio**: Yaesu FT-710, UK market variant, CAT ID `0800`, firmware
  V01-12 (the same radio as M5a/M5b).
- **Host/port**: macOS, Apple CP2105 stack, Enhanced-UART node
  `/dev/cu.usbserial-<redacted>` (the node name embeds the adapter's
  device serial, so it is redacted here exactly as M5a's was).
- **Controller**: controller-driven throughout — every CAT exchange below
  was issued by tooling, not typed by hand.

### Safety preamble

This session was read-only **by construction as well as by conduct**:

- The outbound allowlist accepts `EX` frames in the 9-byte READ shape
  only (`core/cat/allowlist.go`, `validEXRead`), so the shipped build
  physically cannot emit an EX Set frame.
- No `MW`, no `MT`-set and no `MC`-set was issued at any point.
- One tool outside that allowlist was used: a scratch probe **outside the
  repository** (the M5b precedent for sanctioned raw probing), which
  sends read frames only. Its purpose was to reach addresses the
  allowlist deliberately refuses — see "Out-of-inventory rejections".
- Stuart was present throughout, watching the radio's TX indicator. **No
  TX key-up occurred at any point in this session.**
- A full SD-card configuration backup of this radio already existed from
  the M5a/M5b session day.

### Method

1. `rigprog read --settings` against the real port, twice in succession,
   each to a private capture file; the two results were then compared.
2. Eight scratch read probes: one known-good control, six
   out-of-inventory addresses, then the control repeated.
3. A 30-read back-to-back latency loop on one address.

### Findings

**1. The whole documented inventory is readable.** Both sweeps returned
296 of 296 entries `known`, zero `unavailable`, `complete: true`,
descriptor `ft710-ex@1`. Nothing in Table 2 was rejected by this radio in
this configuration.

**2. The two sweeps were byte-identical to each other** — menu entries
and channel data alike. Reading the menu surface did not perturb it, and
did not perturb the channel surface, over the minutes separating them.
That is what was observed; it is not a claim of determinism in general.

**3. Out-of-inventory rejections answer `?;`.** Six syntactically
well-formed six-digit read addresses, none of them in Table 2 — and four
of them outside the manual's printed P1/P2/P3 ranges as well — were
probed, each answering the same generic,
unattributed NAK as `MR`/`MT` do for out-of-inventory slots:
`EX050101;`, `EX050505;`, `EX010199;` (P3 beyond the group's item
count), `EX019901;` (undocumented P2 subgroup), `EX079901;` (P1 beyond
the documented group set) and `EX999999;`. The control frame answered
identically before and after, so the radio was undisturbed by them. This
is a sample of the out-of-inventory space, not a survey.

**4. The P1=05 grammar anomaly now has evidence behind it.** The CAT
manual's EX grammar note reads "P1 : 01 - 04, 05" while Table 2 shows
four groups at P1 01-04 plus EXTENSION SETTING at P1=06, and no P1=05
group at all. Both probed P1=05 addresses were rejected — consistent
with Table 2 being right and the grammar note's "05" being a typo. Two
samples do not survey the P1=05 space, so this is strong support for the
reading the project already followed rather than proof no P1=05 address
exists anywhere. It is, though, consistent with the membership-not-ranges
validity rule this project already had (`core/cat`'s `KnownEXAddress`):
an address is valid because it is in the transcribed chart, never because
its digits look plausible. A range-based validator would have accepted
both rejected addresses.

**5. One manual width error.** `01 03 21` (RADIO SETTING → MODE FM →
TONE FREQ) answered a **three-byte** P4 where Table 2's Digits column
prints 2. The raw frame was `EX010321012;`, captured by the scratch probe
rather than through this project's own codec, so it is not a parsing
artefact. Every one of the other 295 addresses answered at exactly the
width both independent transcriptions (`core/cat/table2.csv` and
`internal/fakeradio`'s `exGroups`) predicted — including the six
merged-cell MIC items whose width was an interpretive judgement call.
Recorded in `core/cat/table2-corrections.csv` and applied to the
simulator through `fakeradio`'s `exHardwareOverrides`.

**6. One manual chart typo caught, partly settled.** `01 05 16` SHIFT
FREQUENCY's P4 column reads `1: 170 Hz 1: 200 Hz 2: 425 Hz 3: 850 Hz` —
two `1:` codes and no `0:`. The radio answered `0`, which proves a `0`
code exists and therefore that the printed chart is wrong. **Which label
`0` maps to is not established**: the obvious reading is that the first
entry should be `0: 170 Hz`, but no front-panel check of this item was
made, so that remains inference rather than evidence. Recorded that way
in `table2-corrections.csv`; the transcription itself keeps the typo, as
its provenance requires.

**7. Front-panel ground truth matched on all three items checked.**
Stuart read three menu items off the radio's own screens — a 3-digit
numeric (`03 01 01` BEEP LEVEL), a two-digit numeric (`04 01 05` DIMMER
LED) and an enumerated item (`04 01 03` POP-UP TIME) — while the capture
was compared against them. All three matched, including the enumerated
item's chart label and the zero-padding of the 3-digit field: the values
are being decoded correctly, not merely received. **The values
themselves are deliberately not recorded here** — they are this radio's
current configuration, and what the evidence needs to state is that the
comparison was made and matched.

**8. Answer shapes.** Across the 296 addresses: 264 answered plain
digits, 26 answered with an explicit leading sign counted inside the
manual's own width (all 26 of them 3 bytes wide), and the six free-text
items answered exactly 12 bytes, right-space-padded — the same convention
`MT` tags already use, now seen for EX too. The full per-address
width/shape table is `core/cat/table2-observed.csv`; **no value is
committed anywhere**, here or in that artefact, beyond the two the
operator consented to publish (finding 5's TONE FREQ frame and finding
6's SHIFT FREQUENCY code).

**9. Timing.** EX reads answered in 6–7 ms flat over 30 back-to-back
requests with zero settle delay — faster than M5a measured for `MR`
(10–11 ms). The full 296-address sweep took roughly 8 s of wall-clock,
so present pacing has considerable headroom. Recorded as a pacing knob;
nothing was changed on the strength of it.

**10. Port node names are not durable identity.** The SiLabs
Enhanced-UART node was `SLAB_USBtoUART` at M5a and `SLAB_USBtoUART7`
here, for the same physical adapter; the Apple-stack node was
Enhanced-UART in both sessions. Session identity must not be bound to a
node path.

### Explicitly not characterised

Recorded honestly, as M5a's own "Explicitly not probed" section is. A
passive read of whatever the radio happens to be set to cannot establish:

- **Alternate signed-zero.** The `-00` form was never seen in these
  sweeps; whether the radio ever emits it, and whether it would accept
  one, is unknown. (The manual documents both `-00` and `+00` for these
  items, so the question is real, not theoretical.)
- **Ranges, boundaries and sparse enum code sets.** Only the single
  current value of each item was seen, so the manual's ranges remain
  transcription, not observation — apart from the two corrections above.
- **Text charset limits.** The six text items' current contents were
  seen; what characters the radio accepts or rejects was not.
- **Every aspect of EX Set** — which addresses are writable, whether
  out-of-range values are clamped or refused, what a write does to
  related items. Nothing here may be used to size or shape a Set frame.
- **`MenuUnavailable` for an in-inventory address.** Nothing in the
  inventory was rejected, so whether some radio state, region or option
  fit makes a documented address answer `?;` remains unobserved.

All of these need deliberate front-panel mutation and a write path.
**That work is not happening:** the write go/no-go decision was taken on
25/07/2026 and the answer was no — the menu surface is read-only for
v1.x, partly *because* of the gaps listed here. See
`docs/menu-write-decision.md`. The list stands as the record of what a
passive read leaves unknown, not as a to-do list.

### Consequences applied

| Finding | Consequence |
| --- | --- |
| All 296 readable, two identical sweeps | `core/cat/table2-observed.csv` (per-address observed read width and shape, values-free), derived by `internal/extable/observe` and pinned by tests |
| Observed widths/shapes | `cat.EXItem.ObservedReadWidth`/`ObservedReadShape` carried through the generated inventory; `core/transport/ex_crosscheck_test.go` gains a third cross-check comparing the fake's runtime answers against them |
| TONE FREQ width (manual 2, observed 3) | `core/cat/table2-corrections.csv`; `internal/fakeradio`'s `exHardwareOverrides` (the manual transcriptions in `table2.csv` and `exGroups` are deliberately left alone) |
| SHIFT FREQUENCY duplicated enum code | `core/cat/table2-corrections.csv` — the only machine-readable home for a chart-text correction |
| 26 signed items | `exHardwareOverrides` gives the fake an explicit synthetic sign at the observed width, so simulator answers carry the shape the radio uses |
| Out-of-inventory `?;`, P1=05 rejections | `internal/fakeradio/doc.go` register items 23 and 25 record the six and two probes respectively — item 23's whole-space rule stays ASSUMED, now with supporting samples; `core/cat/table2.csv`'s anomaly header and the P1=05 comments across `core/cat` reworded to match |
| Front-panel ground truth | Recorded only — it is the evidence that decoding is correct, and changes no code |
| 6–7 ms latency | Recorded only; pacing unchanged (ledgered as a knob) |
| Node-name rotation | Recorded only — see "Port mapping" |

## Cross-references

- Findings ledger: `.superpowers/sdd/progress.md`, "M5a LIVE SESSION
  (13/07/2026)" and "M5b LIVE TRIALS COMPLETE" sections, and the
  M5b-FLIP PRECONDITION review item (write-capability-split decision —
  adjudicated RATIFIED-AS-NOT-NEEDED; restated in
  `core/driver/ft710/caps.go` and the flip commit message).
- Raw transcripts (PRIVATE, never committed):
  `docs/fixtures-private/m5a-transcript.private-capture`,
  `docs/fixtures-private/m5b-trials.private-capture`,
  `docs/fixtures-private/read-2026-07-13-baseline.json`,
  and for M8c: `m8c-settings-2026-07-24-run1.json`,
  `m8c-settings-2026-07-24-run2.json`, `m8c-run1.private-capture`,
  `m8c-run2.private-capture`, `m8c-exprobe.private-capture`.
- M8c committed evidence: `core/cat/table2-observed.csv` (values-free:
  widths and shape classes only) and `core/cat/table2-corrections.csv`
  (which quotes the two consented values), each carrying its own
  provenance header.
- Redaction policy: `docs/fixtures.md`.

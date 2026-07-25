# Decision: the menu (EX) settings surface is read-only

**Decided 25/07/2026 by Stuart Henderson (project owner). Status:
settled for v1.x.**

open-rig-programmer reads the FT-710's menu settings and will not write
them. This document records that decision, the evidence behind it, and
the conditions under which it would be worth reopening — so the question
does not get quietly re-answered by whoever next looks at the gap.

## The decision

No menu-write path is built. Concretely:

- No code in this repository can send a menu-change frame, and none is
  planned for v1.x.
- The outbound allowlist's rejection of every EX Set/Answer-shaped frame
  (`core/cat/allowlist.go`, `validEXRead`) is the **shipped policy**, not
  a temporary phase restriction. It was written as phase-scoped; this
  decision makes it permanent for v1.x.
- Menu writability stays `Unverified` in the capability model. There is
  no `SettingsWriter` interface and no per-address write gate.
- Radio models added later inherit this: their settings support is a
  read-only snapshot, full stop.

The read side is unaffected and ships as-is: `rigprog read --settings`,
`rigprog settings FILE`, and the GUI's settings view.

## Why

**1. The only source for a write path is a chart that was wrong in both
places we could check it.**

Writing settings safely means a typed value descriptor per item —
allowed wire codes, units, scaling, canonical form, width policy — for
all 296 addresses. Nearly all of that would come from Table 2 of the
FT-710 CAT manual. The hardware session of 24/07/2026
(`docs/hardware-notes.md`, "M8c settings read-characterisation")
compared that chart against a real radio in the only two respects a
passive read can check, and the chart failed both:

- TONE FREQ is documented as 2 digits; the radio answers 3.
- SHIFT FREQUENCY's chart prints the code `1:` twice and offers no `0`;
  the radio answered `0`. The printed codes are wrong, and which label
  `0` actually selects is still unestablished.

Two checks, two errors. That is a small sample, but it is the whole
sample, and it points one way. Building 296 write descriptors on that
source, with no independent way to verify them, is how you end up
writing a plausible-looking wrong value to somebody's radio.

**2. Read evidence does not transfer to the write direction.**

M8c established observed *read* widths and shapes per address, and the
codebase is deliberate about labelling them read-direction-only
(`cat.EXItem.ObservedReadWidth`). Nothing observed in a read tells you
what the radio accepts in a Set frame — not the width, not the
tolerated range, not whether an out-of-range value is clamped or
refused. Establishing that needs deliberate front-panel mutation on real
hardware, per address, which is exactly the work the decision declines.

**3. The value is thin for the users this project has.**

Settings writing earns its keep when you are cloning configuration
between radios, restoring a rig after a reset, or configuring several
identically. The project owner has one radio, and the read-only snapshot
already provides what was actually missing: a durable, diffable record
of how the rig is configured.

**4. The cost is the largest remaining piece of work, plus a hardware
session that mutates a working radio.**

The build (typed descriptors, `BuildEXSet`, allowlist loosening,
`SettingsWriter`, a separate settings send plan, a versioned image
digest, an editable GUI view, guard changes) is a milestone larger than
anything else outstanding. Proving any single address then requires
deliberately changing menu values over CAT and reading them back, on the
only radio the project has.

**5. It removes a defence rather than adding one.**

Today the allowlist rejects an entire wire shape outright — EX Set and
Answer are byte-shape-identical to each other, so refusing the shape
refuses both. A write path necessarily replaces that blanket refusal
with per-address judgement. That is a real reduction in the last line of
defence before bytes reach a physical radio, and it should be bought
with a benefit worth having.

## What this decision is not

It is **not** a claim that the FT-710 refuses menu writes, that CAT
menu-writing is impossible, or that the manual is unusable. It is a
judgement that the evidence available does not support building the
feature safely, and that the feature is not worth the evidence-gathering
it would take.

It is also not a rejection of the safety constraints agreed earlier.
Regardless of any future reversal, these menu classes stay
**permanently non-writable** absent a dedicated per-class safety review:
the CAT link settings (baud rate and interface selection), PTT and
keying via RTS/DTR, TUN/LIN and CAT-3, TX power, emergency TX, and TX
inhibit. Changing the CAT link settings over CAT can sever the link
being used to change them; the rest touch transmit behaviour.

## What would make this worth reopening

All of the following, not any one of them:

1. **A user need that one radio cannot generate** — settings cloning
   across radios, restoring after a reset, or configuring several
   consistently.
2. **Evidence that the chart can be trusted**, or a way to work without
   it: several testers' radios characterised, or a second independent
   transcription that agrees.
3. **The prerequisites in place** — v1.0.0 shipped, and the codec
   dialect seam (M9b) landed first, so that loosening the allowlist
   happens once, in the dialect-parameterised gate, rather than twice.

If it is reopened, the version to build is a **deliberately small
subset** — cosmetic, reversible, front-panel-checkable items such as
beep level, LED dimmer and display timeouts — with every other address
staying read-only. That exercises the whole write path end to end at
close to zero risk, and it is a far cheaper thing to verify on real
hardware than 296 typed descriptors.

## References

- `docs/hardware-notes.md` — the M8c session record, including the two
  chart corrections and an explicit list of what a passive read cannot
  establish.
- `core/cat/table2-corrections.csv` — the machine-readable record of the
  two manual corrections.
- `core/cat/allowlist.go` (`validEXRead`) — where the refusal lives.
- `README.md`, "Planned features" — the user-facing statement.

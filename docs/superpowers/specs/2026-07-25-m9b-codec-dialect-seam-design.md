# M9b — codec dialect seam: design

**Status: approved 25/07/2026. Supersedes the M9 roadmap's A1/A2/D7 where
they differ; every difference is called out below.**

## Purpose

Turn `core/cat` from a single FT-710-flavoured codec into a
dialect-parameterised one, and stop `core/transport` owning the outbound
allowlist. The point is that adding the FTdx10 in M9c becomes *write a
table and register it*, not *fork the codec* — and that the security
gate becomes per-driver and fail-closed rather than a package-level
function anyone can reach.

FT-710 behaviour does not change. That claim is scoped precisely in
"Verification" below; it is not a claim that nothing in the repository
moved.

## Scope decisions

Two decisions taken at design time depart from the approved roadmap.
Both are deliberate.

**1. Dialect carries data, not frame-shape variants.** The roadmap's A1
has the dialect select per-command frame variants, because Codex's
M9-plan review established that the FTdx10/101 manuals document a
~50-byte combined MT record frame against the FT-710's short form.

That difference is real in the manuals and entirely unverified against
hardware — and the FT-710's own MT is the precedent for a manual being
wrong about exactly this. Building a variant-selection mechanism now
means shaping it around an unchecked claim, with no second radio to
test it and no way to find out we guessed wrong until M9c.

So M9b does the structural refactor in isolation, where it can be
verified byte-for-byte against the one radio the project has, and
defers variant selection to M9c, where the FTdx10 gives it a second
implementation to answer to. **Ledgered as deferred, not dropped.**

**2. `core/cat`'s package-level API is not preserved.** Nothing is
published yet — v1.0.0 is not tagged, and is blocked on GitHub Actions
billing and the Linux hardware session — so the churn is free now and
would not be later. Builders and parsers move onto the dialect and every
call site is updated.

The cost is that the easiest proof of "FT-710 unchanged" — an untouched
test suite — is gone. "Verification" below replaces it deliberately.

## Architecture

`cat.Dialect` is an exported struct with unexported fields, carrying
only what genuinely varies across the classic NEWCAT family:

| Carried | Today |
| --- | --- |
| Mode-nibble set | `modeNames`, behind `ParseMode` / `Mode.Wire` |
| EX inventory | generated `exItemsGen`, behind `EXItems` / `KnownEXAddress` |
| Slot-space rules | `classifySlotWire`, and the `Writable` / `readableSlot` policy |
| CAT ID | `catID = "0800"` — a const in `core/driver/ft710/caps.go` |

One package-level instance, `cat.FT710`. Grammar entry points become
methods, so a call reads `cat.FT710.BuildMWSet(…)`.

**Fields stay unexported and there is no exported constructor.** This is
a known, deliberate gap against roadmap A1, which has new-model packages
`core/cat/ftdx10` supplying `func Dialect() cat.Dialect` — impossible
from outside the package while the fields are unexported. M9b has
exactly one dialect, defined in-package, so a constructor now would be
an API shaped by guesswork with no caller to check it against. M9c adds
one when it has a real second caller to shape it. Recorded here so the
gap is a decision rather than an oversight discovered mid-M9c.

The CAT ID moves into the dialect and `caps.go` reads it from there, so
it has one source rather than two. It is the value M9c's registration
needs in any case.

`core/driver/ft710` gains a `cat.Dialect` field and routes every codec
call through it. It is the only production consumer.

### Why a struct with methods

Rejected alternatives:

- **Dialect passed only to the functions that consult it**
  (`cat.AllowedCommand(d, frame)`, but `cat.BuildIDRead()` free-standing).
  More honest — the signature tells you whether something is
  radio-specific — but every axis of variation M9c discovers changes
  another signature, so the churn is paid repeatedly instead of once,
  and call syntax ends up inconsistent.
- **Dialect as an interface.** One implementation, and the roadmap
  already commits the FTX-1 to its own grammar package (A1), so the
  interface would never be satisfied by the radio whose grammar
  actually diverges. Abstraction with no second implementer, now or
  planned.

Methods win because M9c's job should be writing a table, not re-plumbing
signatures. The cost is a receiver on a few methods that do not vary
today — accepted.

## The gate

`core/transport` gains:

```go
type AllowFunc func(frame []byte) bool

func NewEngine(p Port, allow AllowFunc, opts ...Option) (*Engine, error)
```

`Dialect.AllowedCommand` has exactly `AllowFunc`'s signature, so the
driver passes the method value directly — no adapter, no wrapper type.

**Fail-closed, twice.** The roadmap says a nil `allow` means every `Do`
refuses. This design also makes the bad state unconstructable:
`NewEngine` returns a typed error for a nil `allow` before starting the
read goroutine, *and* `Do` still checks defensively before writing. That
is the same reasoning `ErrDisallowedCommand` already documents — the
check "should be unreachable for any Command actually produced by a
core/cat builder", and the Engine "still checks defensively, because it
is the last defence before a physical radio ever sees these bytes" —
applied one layer up. It costs one error check at the single production
call site (`core/driver/ft710/ft710.go:155`) and six in tests.

A distinct `ErrNoAllowlist` sentinel, not a reuse of
`ErrDisallowedCommand`: "this frame is not permitted" and "this engine
was misassembled" are different faults, and conflating them would have a
diagnostic blame the frame for a composition bug. Both refuse.

**What is NOT decoupled.** `core/transport` still imports `core/cat` for
the frame accumulator, `cat.IsRejection` / `ErrRejected`, and
`cat.BuildAISet` for the AI init frame. Only the gate is injected.
Making the init frame injectable is a separate seam the roadmap
deliberately leaves until a rig actually differs, and M9b does not
pretend otherwise.

## Guards, and the pin amendment

The write-path guard matches builders structurally: `sel.X` must be an
`*ast.Ident` naming the `core/cat` import, with `sel.Sel.Name` equal to
`BuildMWSet` / `BuildMTSet` (`internal/guards/importgraph_test.go:251-255`).

Under this design `cat.FT710.BuildMWSet(…)` parses as a *nested*
selector — `sel.X` is itself a `SelectorExpr`, not an `Ident` — so that
check stops matching. A driver calling `s.dialect.BuildMWSet(…)` is the
same shape.

This fails loudly, not silently: the guard's `sawDriverBuildMW`
non-vacuity counter reports "the walker or its filters are broken, and
every check above passed vacuously". The M9a review's F10 requirement
doing its job.

**Remedy:** match on the method name alone — any `SelectorExpr` whose
`Sel.Name` is `BuildMWSet` / `BuildMTSet`, whatever the receiver. Looser
than the package-qualified match, but strictly *more* inclusive, so it
cannot admit a call the current one catches. It is also exactly the
approximation the guard already accepts for `WriteChannel`, which its
own doc comment describes as matching "ANY selector named WriteChannel
outside the allowed trees, whatever the receiver's type" — house
precedent, not a new compromise. Non-vacuity counters stay.

**New guard:** `transport.NewEngine` may be referenced only from
`core/driver/**`, excluding `core/transport` itself and test files, so
nothing outside the driver tree can mint an Engine and choose its gate.

**The byte-identical pin on `importgraph_test.go` is formally amended
here** — which the roadmap always intended M9b or M8e to do, and M8e is
cancelled. The ceremony: ledger what changed and why, state explicitly
that guard semantics are not weakened, and fold the now-redundant
`TestSimulatedTokenSingleNonTestFileRepoWide` into the data-driven
`TestSimulatedProfileTokensConfinement`, retiring the duplicate.

The pin's baseline commit was `38b3087`, which no longer exists after
the history reset of 25/07/2026. The new baseline is the initial commit,
`ed728b9`.

## Verification

**Mint the frame-corpus pin first, as its own commit, before any
signature moves.** Commit a fixed input corpus, drive every builder over
it, concatenate the frames, pin the digest. The refactor must then
reproduce that digest exactly. Done in the other order the pin merely
records whatever the refactor produced, and proves nothing.

**Evidence that must not move.** The manual-derived G1–G12 golden
vectors, the M5a/M5b hardware-derived vectors, the EX observed-CSV pins
and `TestAllowedCommand_RejectsGoldenAnswerFrames`' answer-frame corpus
keep their expected byte literals character-for-character; only
invocations change. A diff to an expected-value literal in any of those
files is a review stop, not a mechanical edit.

**The cross-check must stay a cross-check.**
`core/transport/ex_crosscheck_test.go` has meaning only because
`internal/fakeradio` transcribes Table 2 independently of `core/cat`. If
the dialect ever became fakeradio's source, that test would compare a
value with itself. `TestNoCoreImports` forbids it structurally and stays
untouched.

**Property tests, honestly scoped.** Every builder's output passes its
own gate; answer-only shapes are refused; the Set/Answer-indistinguishable
frames — MT today, and MC, per Codex's F2 finding that "every answer
frame fails" is impossible — live in an explicit audited exception
table. **Cross-dialect negatives are deferred to M9c and ledgered**: with
one dialect such a test cannot fail, and a test that cannot fail is
worse than no test, because it reads as coverage.

**Fail-closed tests.** `NewEngine` with a nil `AllowFunc` returns the
typed error and starts no goroutine; an Engine whose gate is nil refuses
at `Do` without writing; the guard confirms nothing outside
`core/driver/**` references `NewEngine`.

**Scope of the byte-identical claim** — the roadmap's own F12 narrowing,
not something broader: CAT frames, golden and hardware-derived vectors,
codeplug JSON, `Digest`, schema 2, and CLI output are byte-identical
pre/post, plus a byte-compare of `probe --fake` output as M9a did. It is
*not* a claim that nothing else in the repository changed — the API
churn means a great deal did.

**Full local gate**, CI being billing-dead: `gofmt -l .`, `go vet ./...`,
`go build ./...`, `go test ./...`, guards verbose, `-race` on core,
`wails generate module` idempotence, frontend check/test/build, e2e
`probe --fake` and `read --fake --settings`, plus the frame-corpus
digest and the pin-amendment ledger entry.

## Out of scope

- Per-command frame-shape variants (deferred to M9c — see Scope 1).
- Cross-dialect negative property tests (deferred to M9c — no second
  dialect to be negative about).
- Making transport's AI init frame injectable (roadmap risk 10 — a seam
  noted, not built, until a rig differs).
- Any second radio driver, table or manual transcription. M9b ends with
  exactly one dialect.
- Anything touching the menu-write path: it does not exist and is not
  coming (`docs/menu-write-decision.md`).

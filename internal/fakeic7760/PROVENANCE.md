# IC-7760 fake provenance

The independent fake derives its record from the frozen B/W artefacts:
`IC-7760-transcription-b.csv` and `IC-7760-geometry-witness.csv`. Their 27
printed bytes less the two selector bytes gives 25 record bytes; the name is
ten bytes. Selectors and `1A 00` are copied from those artefacts. `FB`/`FA`
are the documented ACK/NG codes. The `B2`/`E0` address pair is copied from the
data-format diagram, and **both halves are required**: a frame addressed to
`B2` from any source other than `E0` is echoed if the echo is on, and then
dropped in silence.

## Every modelled assumption, and the option that reaches it

Each row is a place the document does not speak. Every one is exposed under
the name of the IC-7760 capability matrix register entry that owns it, so the
knob and the entry can be found from each other.

| register entry | option | what this fake does by default |
| --- | --- | --- |
| `ic7760-id-reply` | `WithIDReply` | answers `19 00` with the invented one-byte token `A5`, chosen to be obviously synthetic |
| `ic7760-echo-default` | `WithEchoDefault` | echo off; when on, it reflects **before** the address filter, because an echo is a property of the line and answering is a property of the radio |
| `ic7760-broadcast-form` | `WithBroadcastForm` | unsolicited frames carry `to=00`, `from=B2` |
| `ic7760-empty-reply-fa` | `WithEmptyReplyFA` | an unwritten slot answers `FA`; no other code is modelled |
| `ic7760-empty-reply-ff` | `WithEmptyReplyFF` | a stored record of all `FF` read back is treated as empty |
| `ic7760-scan-edge-record-shape` | `WithScanEdgeRecordShape` | P1 and P2 take the same 25-byte shape as a memory channel |
| `ic7760-write-full-record` | `WithWriteFullRecord` | a `1A 00` set must carry the whole 25-byte layout |
| `ic7760-record-length` | `WithRecordLength` | 25, a derivation and not a printed total |
| `ic7760-address-menu` | `WithRadioAddress` | `B2`, which is printed; that a menu can change it is what is assumed |

## The two `FF` questions are separate

They are asked in opposite directions and are answered independently.

**Inbound — a record of `FF`s the radio hands back.** Register entry
`ic7760-empty-reply-ff`. The guide's only `FF` in the memory context is a
value the CONTROLLER sends to erase; nothing licenses reading it backwards.
This fake reads such a record as empty **by default and by assumption**, and
`WithEmptyReplyFF(false)` turns that reading off, after which the record comes
back as stored.

**Outbound — the printed one-byte clear form.** `1A 00 <hi> <lo> FF`, the
address then a single `FF` in the ③ select-memory byte and nothing after it.
This fake **always refuses it with `FA`**, and that refusal is not a knob: no
clear builder exists in this tier and erase is not admitted, so a code path
that ever emits one should fail in a test rather than quietly empty a
simulator's channel. The refusal is matched explicitly in `write` rather than
left to fall out of the length arithmetic, so it survives every length option
— including `WithWriteFullRecord(false)`, which otherwise admits short sets.
`TestThePrintedClearFormIsRefusedIndependentlyOfShortSets` pins the
independence in both directions.

Short sets and wrong sibling/length forms otherwise return NG. Commands
outside the memory surface and `19 00` return NG; frames that are not this
radio's business return nothing at all.

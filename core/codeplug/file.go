// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// CurrentSchema is the HIGHEST schema version this package writes, and
// the highest it will read. Load rejects a file whose Schema is greater
// than CurrentSchema (see ErrSchemaTooNew) rather than guessing at
// forward compatibility with a format it does not know, and every file
// Load accepts comes back with Schema == CurrentSchema (migrate-on-load).
//
// It is no longer the version Save always emits, and that is the whole
// point of the Icom tier's file work (design D4, adjudication 4; round 2
// F6+C7). Save emits the LOWEST schema that can REPRESENT the content —
// see schemaFor — so a codeplug holding nothing the tier added, with
// every value inside schema 3's ranges, is written as schema 3 exactly
// as it was before the tier existed. That is what makes every existing
// Yaesu codeplug and manifest artefact byte-identical BY CONSTRUCTION
// rather than by a promise nobody can check.
const CurrentSchema = 4

// lowestSchema is the oldest version Save will emit — the floor of
// schemaFor's search. Older schemas are readable (loadV2, loadV1) but
// never written: they are strictly less expressive than 3 in ways that
// LOSE information (v1's opaque menus blob, v1/v2's untyped
// tag_display), so emitting one would be a downgrade, not a
// representation choice.
const lowestSchema = 3

// maxCodeplugFileSize is the largest file Load will read, in bytes. A
// full multi-bank FT-710 image in this package's indented JSON encoding
// is a few hundred KiB at most; 8 MiB is generous headroom while still
// refusing to load an absurdly oversized or corrupted file (e.g. a
// misdirected binary, or a file that grew unboundedly) into memory
// whole.
const maxCodeplugFileSize = 8 << 20 // 8 MiB

// Codeplug is the on-disk file format: a complete radio memory image plus
// enough metadata to know what wrote it, what radio it came from, and
// what schema version it is.
type Codeplug struct {
	// Schema is the file format version. See CurrentSchema.
	Schema int `json:"schema"`
	// Generator identifies the software that wrote this file, e.g.
	// "open-rig-programmer v0.1.0".
	Generator string `json:"generator"`
	// Radio describes the radio these Channels were read from (or are
	// destined for).
	Radio RadioInfo `json:"radio"`
	// Channels holds every memory channel slot, empty or populated.
	Channels []Channel `json:"channels"`
	// Menus holds the optional menu/EX settings snapshot (schema 2). It is
	// nil when a file carries no menu data. See MenuSnapshot.
	Menus *MenuSnapshot `json:"menus,omitempty"`
}

// ErrSchemaTooNew is the sentinel a caller should compare against (via
// errors.Is) when Load rejects a file because its Schema is newer than
// this program's CurrentSchema — the file was written by a newer version
// of this software than can read it. Every error Load returns for that
// condition is a *SchemaError, whose Unwrap returns this sentinel.
var ErrSchemaTooNew = errors.New("codeplug: file schema is newer than this program supports")

// SchemaError reports that a codeplug file's Schema field could not be
// used to load it: either Schema is not a positive integer (0 or
// negative — never a valid version, e.g. an empty/zero-valued file), or
// Schema is greater than CurrentSchema, in which case errors.Is(err,
// ErrSchemaTooNew) reports true.
type SchemaError struct {
	// Schema is the offending value read from the file.
	Schema int
}

// Error implements the error interface. For a too-new schema, the
// message explicitly tells the user to upgrade the app.
func (e *SchemaError) Error() string {
	if e.Schema > CurrentSchema {
		return fmt.Sprintf("codeplug: file schema %d is newer than this program supports (max %d) — upgrade the app to open this file", e.Schema, CurrentSchema)
	}
	return fmt.Sprintf("codeplug: file schema %d is invalid: schema must be a positive integer", e.Schema)
}

// Unwrap lets errors.Is(err, ErrSchemaTooNew) match when Schema is too
// new. For an invalid (0/negative) schema it returns nil: that condition
// is not "too new", it is simply not a valid schema at all.
func (e *SchemaError) Unwrap() error {
	if e.Schema > CurrentSchema {
		return ErrSchemaTooNew
	}
	return nil
}

// UnknownFieldError reports that Load rejected a codeplug file because a
// JSON object somewhere in it contains a key this package's schema does
// not recognise (e.g. a hand-edit that misspells "rx_clar" as
// "rx_clarr"). Without this check, encoding/json would silently ignore
// the unrecognised key and leave the corresponding Go field at its zero
// value — precisely the kind of silent default this package cannot
// tolerate, since Load gates what gets written to a physical radio.
type UnknownFieldError struct {
	// Field is the offending key, e.g. "rx_clarr".
	Field string
}

// Error implements the error interface.
func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("codeplug: unknown field %q", e.Field)
}

// DuplicateKeyError reports that Load rejected a codeplug file because one
// schema-controlled JSON object — i.e. any object outside the single
// opaque subtree the file's schema exempts (the whole "menus" subtree in
// schema 1; only "menus.legacy" in schema 2 and later — see
// MenuSnapshot.Legacy) — repeats the same key twice. encoding/json's normal behaviour for a
// duplicate key is silent "last value wins"; that is unsafe here because a
// hand-edited file could carry two "freq_hz" values and this package would
// accept whichever happened to come last with no indication anything was
// wrong.
type DuplicateKeyError struct {
	// Key is the duplicated key, e.g. "freq_hz".
	Key string
	// Path locates the containing object, e.g. "channels[0].data" — empty
	// for a duplicate at the top level.
	Path string
}

// Error implements the error interface.
func (e *DuplicateKeyError) Error() string {
	if e.Path == "" {
		return fmt.Sprintf("codeplug: duplicate key %q", e.Key)
	}
	return fmt.Sprintf("codeplug: duplicate key %q in %s", e.Key, e.Path)
}

// unknownFieldPrefix is the exact, stable prefix encoding/json uses for
// the error (*json.Decoder).Decode returns when DisallowUnknownFields is
// set and it meets a field absent from the target struct: the message is
// `json: unknown field "<name>"` with nothing after the closing quote, so
// trimming this prefix and the final quote recovers the field name
// exactly.
const unknownFieldPrefix = `json: unknown field "`

// wrapDecodeError turns encoding/json's untyped "unknown field" error
// (see unknownFieldPrefix) into a *UnknownFieldError so a caller can
// errors.As it and learn which key was unrecognised. Any other decode
// error (malformed JSON, a type mismatch, etc.) is returned unchanged.
func wrapDecodeError(err error) error {
	msg := err.Error()
	if strings.HasPrefix(msg, unknownFieldPrefix) && strings.HasSuffix(msg, `"`) {
		field := msg[len(unknownFieldPrefix) : len(msg)-1]
		return &UnknownFieldError{Field: field}
	}
	return err
}

// exemptFunc reports whether the child value reached by descending into
// key from an object at path is exempt from the duplicate-key check — i.e.
// an arbitrary, caller-defined bag whose internal key uniqueness this
// package does not police. Which subtree is exempt depends on the file's
// schema, so Load selects the predicate (see menusWholeExempt /
// menusLegacyExempt) from the version it probed.
type exemptFunc func(path, key string) bool

// menusWholeExempt exempts the ENTIRE top-level "menus" subtree — the
// schema-1 rule, where "menus" was a wholly opaque v1.1 reservation
// (see Codeplug.Menus history).
func menusWholeExempt(path, key string) bool {
	return path == "" && key == "menus"
}

// menusLegacyExempt exempts ONLY "menus.legacy" — the schema-2-and-later
// rule. From schema 2 on, the "menus" object is itself schema-controlled
// (its keys are policed like any other), and the sole opaque bag is
// MenuSnapshot.Legacy, which preserves a migrated v1 payload verbatim
// (duplicate keys included).
func menusLegacyExempt(path, key string) bool {
	return path == "menus" && key == "legacy"
}

// checkDuplicateKeys walks data's JSON token stream and reports the first
// duplicate object key found at any level EXCEPT within the subtree
// isExempt selects (see exemptFunc). data must already be known-valid
// JSON — Load calls this only after a successful strict decode of the same
// bytes — so a syntax error surfacing here would indicate a bug in this
// walk, not bad caller data.
func checkDuplicateKeys(data []byte, isExempt exemptFunc) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	return walkJSONValue(dec, "", false, isExempt)
}

// walkJSONValue consumes exactly one JSON value (object, array, or
// scalar) from dec, applying walkJSONObject's duplicate-key check to
// every object it contains, except within an exempt subtree (see
// checkDuplicateKeys). path is this value's location, used only to build
// a DuplicateKeyError.Path if a duplicate turns up inside it.
func walkJSONValue(dec *json.Decoder, path string, exempt bool, isExempt exemptFunc) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil // string, float64, bool, or nil: no children to walk.
	}
	switch delim {
	case '{':
		return walkJSONObject(dec, path, exempt, isExempt)
	case '[':
		return walkJSONArray(dec, path, exempt, isExempt)
	default:
		// '}'/']' cannot appear here: this call site has not yet consumed
		// the opening delimiter that would make a matching close valid.
		return fmt.Errorf("codeplug: unexpected JSON delimiter %v", delim)
	}
}

// walkJSONObject consumes an object's members up to and including its
// closing '}' (the caller has already consumed the opening '{'). It
// reports a *DuplicateKeyError for the first repeated key, unless exempt.
func walkJSONObject(dec *json.Decoder, path string, exempt bool, isExempt exemptFunc) error {
	seen := make(map[string]bool)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			// Cannot happen for well-formed JSON (Load only calls this
			// after data has already decoded successfully): an object
			// member's name token is always a string.
			return fmt.Errorf("codeplug: malformed object: key token %v is not a string", tok)
		}
		if !exempt {
			if seen[key] {
				return &DuplicateKeyError{Key: key, Path: path}
			}
			seen[key] = true
		}
		childExempt := exempt || isExempt(path, key)
		if err := walkJSONValue(dec, joinJSONPath(path, key), childExempt, isExempt); err != nil {
			return err
		}
	}
	_, err := dec.Token() // the closing '}'
	return err
}

// walkJSONArray consumes an array's elements up to and including its
// closing ']', walking every element for nested duplicate keys (an array
// element carries no key of its own, so there is nothing to check at
// this level beyond recursing into each element).
func walkJSONArray(dec *json.Decoder, path string, exempt bool, isExempt exemptFunc) error {
	i := 0
	for dec.More() {
		if err := walkJSONValue(dec, fmt.Sprintf("%s[%d]", path, i), exempt, isExempt); err != nil {
			return err
		}
		i++
	}
	_, err := dec.Token() // the closing ']'
	return err
}

// joinJSONPath appends key to a dotted path, e.g.
// joinJSONPath("channels[0]", "data") == "channels[0].data";
// joinJSONPath("", "schema") == "schema".
func joinJSONPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// OversizeFileError reports that Load rejected a file because it exceeds
// maxCodeplugFileSize.
type OversizeFileError struct {
	// Path is the file that was too large.
	Path string
	// Size is the file's size in bytes — or, in the rare case this was
	// detected only by the read itself outrunning the limit (the file
	// grew after Load's initial size check; see Load), a LOWER BOUND on
	// its true size rather than an exact value.
	Size int64
}

// Error implements the error interface.
func (e *OversizeFileError) Error() string {
	return fmt.Sprintf("codeplug: %s: file is at least %d bytes, exceeds the %d byte limit", e.Path, e.Size, maxCodeplugFileSize)
}

// OversizeSaveError reports that Save refused to write its output because
// the encoded codeplug exceeds maxCodeplugFileSize — the SAME bound Load
// enforces (see OversizeFileError). Without this refusal Save could produce
// a file this very build's Load would reject as oversized (e.g. a compact
// v1 file just under the limit that gains the schema-2 wrapper and
// indentation on Save, or a large native-v2 Legacy blob), leaving the user
// with output they can no longer open. The refusal happens BEFORE any temp
// file is created, so an existing destination is left untouched.
type OversizeSaveError struct {
	// Path is the destination Save was asked to write.
	Path string
	// Size is the encoded output's size in bytes (newline included).
	Size int
}

// Error implements the error interface.
func (e *OversizeSaveError) Error() string {
	return fmt.Sprintf("codeplug: %s: encoded codeplug is %d bytes, exceeds the %d byte limit — Load would refuse to read it", e.Path, e.Size, maxCodeplugFileSize)
}

// Load reads and parses the codeplug file at path.
//
// It is STRICT about the file's shape, because this package's job is to
// gate what gets written to a physical radio, and a silently-misread
// file is exactly the failure mode that matters here:
//
//   - a file larger than maxCodeplugFileSize is rejected
//     (*OversizeFileError) before its content is ever decoded;
//   - any JSON object key this schema does not recognise is rejected
//     (*UnknownFieldError) rather than silently ignored — a hand-edit
//     that misspells "rx_clar" as "rx_clarr" must fail loudly, not leave
//     RxClar at its zero value;
//   - a duplicate key within any schema-controlled object is rejected
//     (*DuplicateKeyError) rather than resolved "last wins" — except
//     inside the one opaque subtree this package does not police, which
//     the schema selects: the whole "menus" subtree in schema 1, but only
//     "menus.legacy" in schema 2 and later (see MenuSnapshot.Legacy);
//   - trailing data after the top-level JSON value is rejected;
//   - a Schema of 0 or negative, and a Schema greater than CurrentSchema
//     (see ErrSchemaTooNew), are both rejected via a wrapped *SchemaError;
//     on success, Schema == CurrentSchema.
//
// Load decodes VERSION-FIRST, in two passes. Pass 1 leniently probes only
// the "schema" field and rejects an out-of-range version straight away, so
// a file from a newer program reports "upgrade the app" (ErrSchemaTooNew)
// rather than a misleading unknown-field error from a premature strict
// decode. Pass 2 then does the strict, version-appropriate decode, through
// that version's own FROZEN decode structs (see legacyChannel) rather than
// through the live ones.
//
// Every older schema is migrated on load, and every one has its in-memory
// Schema set to CurrentSchema. That is a statement about the IN-MEMORY
// value only: what Save then writes is the lowest schema that can
// represent the content (see schemaFor), which for a migrated v1/v2/v3
// file with nothing the Icom tier added is schema 3 — so a round trip
// does not silently upgrade anybody's file.
//
//   - schema 1: its opaque "menus" payload is preserved verbatim as
//     MenuSnapshot.Legacy (a literal null or absent key becomes a nil
//     snapshot; anything else, including {}, is preserved);
//   - schema 1 and schema 2 alike: every populated channel's legacy
//     "tag_display" bool becomes a BoolField (see
//     migrateLegacyTagDisplay for the rule and the reasoning);
//   - schema 1, 2 and 3 alike: every populated channel's ten
//     tier-added fields become Absent — the state that says the file
//     never spoke about them (see migrateV3ChannelData).
//
// Load never panics: a missing file, malformed JSON, or truncated/corrupted
// JSON are all reported as an error.
func Load(path string) (*Codeplug, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}
	if fi.Size() > maxCodeplugFileSize {
		return nil, fmt.Errorf("codeplug: load: %w", &OversizeFileError{Path: path, Size: fi.Size()})
	}

	// Belt-and-braces against a TOCTOU race (the file growing between the
	// Stat above and this read): io.LimitReader caps what is actually
	// read at one byte past the limit, regardless of what Stat reported,
	// so this function never holds more than maxCodeplugFileSize+1 bytes
	// in memory no matter what.
	b, err := io.ReadAll(io.LimitReader(f, maxCodeplugFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}
	if len(b) > maxCodeplugFileSize {
		return nil, fmt.Errorf("codeplug: load: %w", &OversizeFileError{Path: path, Size: int64(len(b))})
	}

	// Pass 1: version probe (lenient). Reject an out-of-range schema before
	// any strict decode, so the version check always wins over a
	// field-level rejection.
	var probe struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}
	if probe.Schema <= 0 || probe.Schema > CurrentSchema {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, &SchemaError{Schema: probe.Schema})
	}

	// Pass 2: version-appropriate strict decode.
	var cp *Codeplug
	switch probe.Schema {
	case 4:
		cp, err = loadV4(b, path)
	case 3:
		cp, err = loadV3(b, path)
	case 2:
		cp, err = loadV2(b, path)
	case 1:
		cp, err = loadV1(b, path)
	default:
		// Unreachable: probe.Schema is within [1, CurrentSchema] here, and
		// every value in that range has a case above. A gap would mean a
		// CurrentSchema bump landed without a matching decode path.
		return nil, fmt.Errorf("codeplug: load %s: %w", path, &SchemaError{Schema: probe.Schema})
	}
	if err != nil {
		return nil, err
	}

	normaliseTags(cp)
	return cp, nil
}

// loadV4 strictly decodes a schema-4 (current) file into the live
// Codeplug shape, checks for trailing data and duplicate keys (exempting
// only menus.legacy), and validates the menu snapshot. No migration: this
// IS the current schema, so the live structs are the right ones — the
// frozen shapes below exist precisely so that stays true as the live
// structs move on.
func loadV4(b []byte, path string) (*Codeplug, error) {
	var cp Codeplug
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cp); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, wrapDecodeError(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("codeplug: load %s: trailing data after the top-level JSON value", path)
	}
	if err := checkDuplicateKeys(b, menusLegacyExempt); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}
	if err := cp.Menus.Validate(); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}
	return &cp, nil
}

// channelDataV3 is the FROZEN schema-3 on-disk shape of a populated
// channel's data: the ten fields that schema carried, with the types it
// carried them in — a uint32 freq_hz, and no trace of the ten the Icom
// tier added.
//
// It is frozen in the same sense, and for the same reason, as
// legacyChannelData below: schema 3 is no longer the live shape, and a
// decoder that followed the live struct would silently stop decoding the
// format it is named for. Until this tier, loadV3 DID decode straight
// into the live Codeplug — correct exactly as long as the live struct
// was schema 3's, and this is the change that ended that.
//
// It serves BOTH directions: loadV3 decodes through it and saveValue
// encodes through it. One declaration rather than a decode/encode pair
// is deliberate — the two must agree key for key and type for type, and
// two copies would only invite drift. The JSON tags and FIELD ORDER are
// therefore load-bearing on the encode side: encoding/json writes struct
// fields in declaration order, so this ordering is what makes a
// re-saved schema-3 file byte-identical to the one that was loaded.
//
// The leaf types (ToneField, BoolField, RadioInfo, MenuSnapshot) are the
// LIVE ones, reused deliberately and on the record: this tier changes
// none of them, so reusing them states the truth about what v3 held. A
// future schema that changes one of them must freeze it here in the same
// change.
//
// FreqHz is the ONE deliberate departure from "the types schema 3 used":
// v3's freq_hz was a uint32, and this is a uint64. The reason is worth
// stating so it is never "corrected" back. This package's rule is that a
// value which does not fit a schema's ranges FORCES the next schema
// (schemaFor), and a uint64 field with that rule enforced above it
// decodes every value schema 3 could hold while encoding only values
// schemaFor has already proved fit. A uint32 here would instead
// TRUNCATE silently on encode if that rule were ever bypassed — exactly
// the class of failure this package exists to refuse. The decode side is
// protected by encoding/json itself: a freq_hz beyond the field's range
// is a decode error, not a wrap.
type channelDataV3 struct {
	FreqHz     uint64    `json:"freq_hz"`
	Mode       string    `json:"mode"`
	ClarHz     int       `json:"clar_hz,omitempty"`
	RxClar     bool      `json:"rx_clar,omitempty"`
	TxClar     bool      `json:"tx_clar,omitempty"`
	CTCSS      string    `json:"ctcss"`
	CTCSSTone  ToneField `json:"ctcss_tone"`
	Shift      string    `json:"shift"`
	Tag        string    `json:"tag,omitempty"`
	TagDisplay BoolField `json:"tag_display"`
	ScanSkip   BoolField `json:"scan_skip"`
}

// channelV3 is the frozen schema-3 shape of one memory-channel slot.
type channelV3 struct {
	Slot string         `json:"slot"`
	Data *channelDataV3 `json:"data,omitempty"`
}

// codeplugV3 is the frozen schema-3 top-level shape.
type codeplugV3 struct {
	Schema    int           `json:"schema"`
	Generator string        `json:"generator"`
	Radio     RadioInfo     `json:"radio"`
	Channels  []channelV3   `json:"channels"`
	Menus     *MenuSnapshot `json:"menus,omitempty"`
}

// loadV3 strictly decodes a schema-3 file through the frozen v3 shape,
// checks for trailing data and duplicate keys (exempting only
// menus.legacy), validates the menu snapshot, and migrates the result to
// the current schema.
func loadV3(b []byte, path string) (*Codeplug, error) {
	var v3 codeplugV3
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v3); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, wrapDecodeError(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("codeplug: load %s: trailing data after the top-level JSON value", path)
	}
	if err := checkDuplicateKeys(b, menusLegacyExempt); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}
	if err := v3.Menus.Validate(); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}

	return &Codeplug{
		Schema:    CurrentSchema, // migrate-on-load, as for every older schema.
		Generator: v3.Generator,
		Radio:     v3.Radio,
		Channels:  migrateV3Channels(v3.Channels),
		Menus:     v3.Menus,
	}, nil
}

// migrateV3Channels converts a decoded schema-3 channel list into the
// current shape. A nil list migrates to nil (a v3 file with no
// "channels" key still loads with nil Channels); an empty list stays an
// empty, non-nil list.
func migrateV3Channels(v3 []channelV3) []Channel {
	if v3 == nil {
		return nil
	}
	out := make([]Channel, len(v3))
	for i, c := range v3 {
		out[i] = Channel{Slot: c.Slot, Data: migrateV3ChannelData(c.Data)}
	}
	return out
}

// migrateV3ChannelData converts one decoded schema-3 channel's data into
// the current shape, or returns nil for an empty slot.
//
// The migration is EXPLICIT about the ten fields the Icom tier added,
// even though every one of them is left at its zero value: that zero is
// codeplug.Absent, and Absent is a DECISION here, not an oversight. A
// schema-3 file predates the fields entirely, so it says nothing about
// them — which is precisely what Absent means. Unavailable would have
// been a claim about the radio the file came from, Unknown a claim that
// the answer is merely not yet in hand; neither is a statement a v3 file
// ever made. See codeplug.Absent for what follows from that choice.
func migrateV3ChannelData(d *channelDataV3) *ChannelData {
	if d == nil {
		return nil
	}
	return &ChannelData{
		FreqHz:     d.FreqHz,
		Mode:       d.Mode,
		ClarHz:     d.ClarHz,
		RxClar:     d.RxClar,
		TxClar:     d.TxClar,
		CTCSS:      d.CTCSS,
		CTCSSTone:  d.CTCSSTone,
		Shift:      d.Shift,
		Tag:        d.Tag,
		TagDisplay: d.TagDisplay,
		ScanSkip:   d.ScanSkip,
		// The tier-added ten: Absent, deliberately — see above.
	}
}

// legacyChannel is the FROZEN schema-1/schema-2 on-disk shape of one
// memory-channel slot.
//
// ONE shape serves BOTH old versions on purpose, and the reason is
// recorded rather than assumed: schemas 1 and 2 differ only at the TOP
// level (v1's "menus" is an opaque blob, v2's is typed) — their channel
// objects are provably identical, key for key and type for type, as of
// schema 3's introduction. Declaring the same thing twice would only
// invite the two copies to drift.
//
// "Frozen" is the whole point of these types. They must NEVER be replaced
// by, nor made to embed, the live Channel/ChannelData: a legacy decoder
// that follows the live struct silently stops decoding the format it is
// named for the moment the live struct changes — which is exactly what
// schema 3 has just done to tag_display. Editing them is only ever
// correct in order to fix a MIS-STATEMENT of what v1/v2 actually held.
type legacyChannel struct {
	Slot string             `json:"slot"`
	Data *legacyChannelData `json:"data,omitempty"`
}

// legacyChannelData is the frozen schema-1/schema-2 shape of a populated
// channel's data: every key those schemas could carry, with the types they
// carried. It is deliberately exhaustive — a key omitted here would be
// REJECTED by DisallowUnknownFields when a perfectly valid legacy file
// carried it.
//
// tag_display is a *bool, not a bool: v1 and v2 both wrote it with
// omitempty, so a false value and an absent key were indistinguishable in
// the file. The pointer is what lets migrateLegacyTagDisplay tell the two
// apart — and the migration rule is stated for each case separately even
// though both reach the same result today, so that a later revisit changes
// a decision rather than discovering one.
//
// The leaf types (ToneField, BoolField, RadioInfo, MenuSnapshot) are the
// LIVE ones, reused deliberately and on the record: schema 3 does not
// change any of them, so reusing them states the truth about what v1/v2
// held rather than cloning declarations that would immediately drift. That
// reuse is conditional, not permanent — a future schema that changes one
// of those types must freeze it here in the same change, exactly as
// tag_display is being frozen now.
type legacyChannelData struct {
	FreqHz     uint32    `json:"freq_hz"`
	Mode       string    `json:"mode"`
	ClarHz     int       `json:"clar_hz,omitempty"`
	RxClar     bool      `json:"rx_clar,omitempty"`
	TxClar     bool      `json:"tx_clar,omitempty"`
	CTCSS      string    `json:"ctcss"`
	CTCSSTone  ToneField `json:"ctcss_tone"`
	Shift      string    `json:"shift"`
	Tag        string    `json:"tag,omitempty"`
	TagDisplay *bool     `json:"tag_display,omitempty"`
	ScanSkip   BoolField `json:"scan_skip"`
}

// codeplugV2 is the frozen schema-2 top-level shape: the schema-2 channel
// list, with the TYPED menu snapshot schema 2 introduced.
type codeplugV2 struct {
	Schema    int             `json:"schema"`
	Generator string          `json:"generator"`
	Radio     RadioInfo       `json:"radio"`
	Channels  []legacyChannel `json:"channels"`
	Menus     *MenuSnapshot   `json:"menus,omitempty"`
}

// codeplugV1 is the frozen schema-1 top-level shape: the same channel list
// as v2, but with Menus the opaque json.RawMessage the v1.1 reservation
// carried.
type codeplugV1 struct {
	Schema    int             `json:"schema"`
	Generator string          `json:"generator"`
	Radio     RadioInfo       `json:"radio"`
	Channels  []legacyChannel `json:"channels"`
	Menus     json.RawMessage `json:"menus,omitempty"`
}

// loadV2 strictly decodes a schema-2 file through the frozen v2 shape
// (typed *MenuSnapshot), checks for trailing data and duplicate keys
// (exempting only menus.legacy), validates the menu snapshot, and migrates
// the result to the current schema.
func loadV2(b []byte, path string) (*Codeplug, error) {
	var v2 codeplugV2
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v2); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, wrapDecodeError(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("codeplug: load %s: trailing data after the top-level JSON value", path)
	}
	if err := checkDuplicateKeys(b, menusLegacyExempt); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}
	if err := v2.Menus.Validate(); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}

	return &Codeplug{
		Schema:    CurrentSchema, // migrate-on-load: Save always emits the current schema.
		Generator: v2.Generator,
		Radio:     v2.Radio,
		Channels:  migrateLegacyChannels(v2.Channels),
		Menus:     v2.Menus,
	}, nil
}

// loadV1 strictly decodes a schema-1 file through the frozen v1 shape
// (whole-"menus" duplicate-key exemption, matching how v1 was written),
// then migrates it to the current schema: the opaque menus payload is
// preserved verbatim as MenuSnapshot.Legacy, the channels are migrated
// like v2's, and Schema is set to CurrentSchema so Save re-emits the
// current version.
func loadV1(b []byte, path string) (*Codeplug, error) {
	var v1 codeplugV1
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v1); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, wrapDecodeError(err))
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("codeplug: load %s: trailing data after the top-level JSON value", path)
	}
	if err := checkDuplicateKeys(b, menusWholeExempt); err != nil {
		return nil, fmt.Errorf("codeplug: load %s: %w", path, err)
	}

	cp := &Codeplug{
		Schema:    CurrentSchema, // migrate-on-load: Save always emits the current schema.
		Generator: v1.Generator,
		Radio:     v1.Radio,
		Channels:  migrateLegacyChannels(v1.Channels),
	}
	if migratedMenusPresent(v1.Menus) {
		cp.Menus = &MenuSnapshot{Legacy: append(json.RawMessage(nil), v1.Menus...)}
	}
	return cp, nil
}

// migrateLegacyChannels converts a decoded schema-1/schema-2 channel list
// into the current shape. A nil list migrates to nil, so a legacy file
// with no "channels" key still loads with nil Channels exactly as it did
// before schema 3; an empty list stays an empty (non-nil) list.
func migrateLegacyChannels(legacy []legacyChannel) []Channel {
	if legacy == nil {
		return nil
	}
	out := make([]Channel, len(legacy))
	for i, lc := range legacy {
		out[i] = Channel{Slot: lc.Slot, Data: migrateLegacyChannelData(lc.Data)}
	}
	return out
}

// migrateLegacyChannelData converts one decoded legacy channel's data into
// the current shape, or returns nil for an empty slot (Data == nil is the
// sole empty/populated discriminator — see Channel). Every field but
// TagDisplay carries across unchanged, because schema 3 changed only that
// one.
func migrateLegacyChannelData(d *legacyChannelData) *ChannelData {
	if d == nil {
		return nil
	}
	return &ChannelData{
		// uint32 -> uint64 widening: legacyChannelData stays frozen at
		// the type v1/v2 actually used, and the conversion cannot lose
		// anything.
		FreqHz:     uint64(d.FreqHz),
		Mode:       d.Mode,
		ClarHz:     d.ClarHz,
		RxClar:     d.RxClar,
		TxClar:     d.TxClar,
		CTCSS:      d.CTCSS,
		CTCSSTone:  d.CTCSSTone,
		Shift:      d.Shift,
		Tag:        d.Tag,
		TagDisplay: migrateLegacyTagDisplay(d.TagDisplay),
		ScanSkip:   d.ScanSkip,
		// The tier-added ten stay Absent, exactly as for a v3 file — see
		// migrateV3ChannelData.
	}
}

// migrateLegacyTagDisplay maps a schema-1/schema-2 "tag_display" onto the
// current BoolField. Both cases produce Known; only the value differs:
//
//   - PRESENT (true OR false) → {Known, that value}. The file states the
//     flag, so it is known — there is nothing to infer.
//   - ABSENT (nil) → {Known, false}. This is justified as strict
//     BEHAVIOUR PRESERVATION, not as a claim about provenance: v1/v2 wrote
//     the field with omitempty, so an absent key decoded to the zero bool
//     false and WAS ALREADY BEING SENT to the radio as false. Migrating it
//     to Known-false therefore sends exactly the byte it sent before, and
//     nothing about a legacy file's behaviour worsens.
//
// The rejected alternative is recorded: migrating absent → Unknown would
// have been the honest-provenance answer, but it would mass-BLOCK every
// channel of every legacy FT-710 file at plan time (see Diff's TagDisplay
// gate) for a value the program was already, silently, sending. One
// caveat is accepted openly: a CHIRP-sourced schema-2 channel carries a
// MANUFACTURED false, which this rule baptises as Known-false. It too was
// already being sent as false, so no behaviour worsens — and CHIRP imports
// made from schema 3 onwards produce an honest Unknown instead.
func migrateLegacyTagDisplay(v *bool) BoolField {
	if v == nil {
		return BoolField{State: Known, Value: false}
	}
	return BoolField{State: Known, Value: *v}
}

// migratedMenusPresent reports whether a v1 "menus" payload carries data
// worth preserving. An absent key (nil) or a literal JSON null migrates to
// a nil snapshot; anything else — INCLUDING an empty object {} — is
// preserved (predictability over cleverness: {} is kept, not collapsed).
func migratedMenusPresent(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return !bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// normaliseTags trims trailing ASCII spaces from every populated
// channel's Tag, in place. This package's canonical tag form is always
// trimmed (padding is a wire-encoding concern only — see
// cat.Dialect.ParseMTAnswer's doc comment for the HW-CONFIRMED radio
// behaviour this mirrors); a file written before this normalisation
// existed (or hand-edited from a padded value a pre-fix Read left in
// place) still loads correctly, rather than re-triggering the false
// verify-mismatch this normalisation exists to prevent. An all-spaces
// legacy tag (the
// radio's own tag-CLEAR form) trims to "", matching "no tag".
func normaliseTags(cp *Codeplug) {
	for i := range cp.Channels {
		if cp.Channels[i].Data == nil {
			continue
		}
		cp.Channels[i].Data.Tag = strings.TrimRight(cp.Channels[i].Data.Tag, " ")
	}
}

// channelDataV4 is the schema-4 on-disk shape of a populated channel's
// data: schema 3's eleven keys in their original order, then the ten the
// Icom tier added, in the order ChannelData declares them.
//
// It is a SEPARATE declaration from the live ChannelData even though the
// two agree today, and that separation is the point (design D4): Save
// must go through a versioned marshal type, never the live struct, so
// that the next change to the live struct is forced to decide what it
// means for the file format instead of silently altering every file this
// program writes.
type channelDataV4 struct {
	FreqHz       uint64      `json:"freq_hz"`
	Mode         string      `json:"mode"`
	ClarHz       int         `json:"clar_hz,omitempty"`
	RxClar       bool        `json:"rx_clar,omitempty"`
	TxClar       bool        `json:"tx_clar,omitempty"`
	CTCSS        string      `json:"ctcss"`
	CTCSSTone    ToneField   `json:"ctcss_tone"`
	Shift        string      `json:"shift"`
	Tag          string      `json:"tag,omitempty"`
	TagDisplay   BoolField   `json:"tag_display"`
	ScanSkip     BoolField   `json:"scan_skip"`
	TxFreqHz     FreqField   `json:"tx_frequency"`
	Duplex       StringField `json:"duplex"`
	OffsetHz     FreqField   `json:"offset"`
	ToneMode     StringField `json:"tone_mode"`
	ToneTx       ToneField   `json:"tone_tx"`
	ToneRx       ToneField   `json:"tone_rx"`
	DTCSCode     IntField    `json:"dtcs_code"`
	DTCSPolarity StringField `json:"dtcs_polarity"`
	Filter       StringField `json:"filter"`
	DataMode     BoolField   `json:"data_mode"`
}

// channelV4 is the schema-4 shape of one memory-channel slot.
type channelV4 struct {
	Slot string         `json:"slot"`
	Data *channelDataV4 `json:"data,omitempty"`
}

// codeplugV4 is the schema-4 top-level shape.
type codeplugV4 struct {
	Schema    int           `json:"schema"`
	Generator string        `json:"generator"`
	Radio     RadioInfo     `json:"radio"`
	Channels  []channelV4   `json:"channels"`
	Menus     *MenuSnapshot `json:"menus,omitempty"`
}

// schemaFor returns the LOWEST schema version that can represent cp —
// the rule design D4 (adjudication 4; round 2 F6+C7) settled on, and the
// reason every pre-tier file stays byte-identical.
//
// Schema 3 unless one of two things is true of some populated channel:
//
//   - a tier-added field is PRESENT (its state differs from the absent
//     default — see ChannelData.tierFieldsAbsent and codeplug.Absent),
//     because schema 3 has no key to put it in; or
//   - a value does not fit schema 3's ranges — today that means a
//     freq_hz above MaxUint32, which the IC-905's 10 GHz reach makes
//     real.
//
// The second clause is what keeps the frozen schema-3 loader honest: a
// >4.29 GHz frequency forces schema 4 even on a codeplug with no tier
// field at all, so loadV3 can never meet a value its own frozen shape
// would have to distort.
//
// Nothing else participates. In particular the in-memory cp.Schema is
// NOT consulted: it is always CurrentSchema after a Load (migrate-on-
// load), so honouring it would make every re-save a schema-4 file and
// destroy the byte identity this function exists for.
func schemaFor(cp *Codeplug) int {
	for _, ch := range cp.Channels {
		if ch.Data == nil {
			continue
		}
		if !ch.Data.tierFieldsAbsent() {
			return CurrentSchema
		}
		if ch.Data.FreqHz > math.MaxUint32 {
			return CurrentSchema
		}
	}
	return lowestSchema
}

// saveValue returns the versioned marshal value Save should encode for
// cp: a codeplugV3 or a codeplugV4, per schemaFor. cp is never modified.
//
// A nil Channels marshals to a nil slice (JSON null), not an empty
// array, in both versions — preserving exactly what the live struct
// produced before this tier.
func saveValue(cp *Codeplug) any {
	schema := schemaFor(cp)
	if schema == lowestSchema {
		return codeplugV3{
			Schema:    schema,
			Generator: cp.Generator,
			Radio:     cp.Radio,
			Channels:  saveChannelsV3(cp.Channels),
			Menus:     cp.Menus,
		}
	}
	return codeplugV4{
		Schema:    schema,
		Generator: cp.Generator,
		Radio:     cp.Radio,
		Channels:  saveChannelsV4(cp.Channels),
		Menus:     cp.Menus,
	}
}

// saveChannelsV3 projects the live channel list onto the frozen
// schema-3 shape. It is only ever reached when schemaFor has already
// proved every channel representable there, so nothing is lost.
func saveChannelsV3(channels []Channel) []channelV3 {
	if channels == nil {
		return nil
	}
	out := make([]channelV3, len(channels))
	for i, ch := range channels {
		out[i] = channelV3{Slot: ch.Slot}
		if ch.Data == nil {
			continue
		}
		d := ch.Data
		out[i].Data = &channelDataV3{
			FreqHz:     d.FreqHz,
			Mode:       d.Mode,
			ClarHz:     d.ClarHz,
			RxClar:     d.RxClar,
			TxClar:     d.TxClar,
			CTCSS:      d.CTCSS,
			CTCSSTone:  d.CTCSSTone,
			Shift:      d.Shift,
			Tag:        d.Tag,
			TagDisplay: d.TagDisplay,
			ScanSkip:   d.ScanSkip,
		}
	}
	return out
}

// saveChannelsV4 projects the live channel list onto the schema-4 shape.
func saveChannelsV4(channels []Channel) []channelV4 {
	if channels == nil {
		return nil
	}
	out := make([]channelV4, len(channels))
	for i, ch := range channels {
		out[i] = channelV4{Slot: ch.Slot}
		if ch.Data == nil {
			continue
		}
		d := ch.Data
		out[i].Data = &channelDataV4{
			FreqHz:       d.FreqHz,
			Mode:         d.Mode,
			ClarHz:       d.ClarHz,
			RxClar:       d.RxClar,
			TxClar:       d.TxClar,
			CTCSS:        d.CTCSS,
			CTCSSTone:    d.CTCSSTone,
			Shift:        d.Shift,
			Tag:          d.Tag,
			TagDisplay:   d.TagDisplay,
			ScanSkip:     d.ScanSkip,
			TxFreqHz:     d.TxFreqHz,
			Duplex:       d.Duplex,
			OffsetHz:     d.OffsetHz,
			ToneMode:     d.ToneMode,
			ToneTx:       d.ToneTx,
			ToneRx:       d.ToneRx,
			DTCSCode:     d.DTCSCode,
			DTCSPolarity: d.DTCSPolarity,
			Filter:       d.Filter,
			DataMode:     d.DataMode,
		}
	}
	return out
}

// Save writes cp to path atomically and durably.
//
// It marshals cp as indented JSON with a trailing newline, writes that to
// a new temporary file in the SAME directory as path (so the final rename
// is same-filesystem and therefore atomic), fsyncs the temp file's
// contents, closes it, renames it onto path, and then makes a best-effort
// attempt to fsync the containing directory so the rename itself survives
// a crash. A caller therefore never observes a partially written file at
// path: it is always either the previous complete file or the new
// complete file, never a half-written one.
//
// The temp file — and hence the file that ends up at path — keeps
// os.CreateTemp's default mode, 0600, rather than the more usual 0644.
// This is deliberate, not an oversight: a codeplug can carry a callsign
// (in RadioInfo or a channel Tag), so this package defaults to the more
// private, owner-only mode.
//
// Directory fsync can fail, or be unsupported outright, on some
// platforms/filesystems (e.g. it is unsupported on Windows, and can fail
// on some older filesystems). Save treats that failure as non-fatal and
// does not report an error for it alone: by that point os.Rename has
// already completed, so what is lost on a directory-fsync failure is only
// the (much rarer) guarantee that the rename survives a concurrent power
// loss — not the atomicity a concurrent reader observes, which the rename
// itself already provides.
//
// Save refuses to write an inconsistent menu snapshot: cp.Menus.Validate()
// is enforced up front (the SAME rule set Load applies), and a failure
// returns before any temp file is created, so nothing is written at path.
//
// Save encodes with HTML escaping DISABLED (json.Encoder.SetEscapeHTML
// (false)) rather than json.MarshalIndent, whose default HTML escaping
// would rewrite literal '<', '>', '&' (to </>/&) and
// U+2028/U+2029 inside a json.RawMessage. That would silently mutate the
// verbatim-preserved MenuSnapshot.Legacy payload (a migrated v1 opaque
// "menus" blob, whose bytes this package promises to carry through
// untouched — see MenuSnapshot.Legacy), so escaping must stay off. The
// Encoder writes exactly the same 2-space indentation the old
// MarshalIndent+append('\n') produced, and appends its own single trailing
// newline, so non-legacy output is byte-for-byte unchanged.
//
// Save then refuses to write output larger than maxCodeplugFileSize — the
// SAME bound Load enforces (see OversizeSaveError) — BEFORE any temp file
// is created, so Save never produces a file this build's own Load could not
// read back, and an existing destination is preserved on refusal.
//
// SCHEMA CHOICE (design D4): Save writes the LOWEST schema that can
// REPRESENT cp — schemaFor's answer — through that version's own marshal
// type, never through the live struct and never through cp.Schema. A
// codeplug holding nothing the Icom tier added, with every value inside
// schema 3's ranges, is therefore written exactly as this program wrote
// it before that tier existed, byte for byte.
func Save(path string, cp *Codeplug) error {
	if err := cp.Menus.Validate(); err != nil {
		return fmt.Errorf("codeplug: save %s: %w", path, err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	// Through a VERSIONED marshal type, never the live struct (design
	// D4) — and at the LOWEST schema that can represent this content
	// (schemaFor), not whatever cp.Schema happens to say.
	if err := enc.Encode(saveValue(cp)); err != nil {
		return fmt.Errorf("codeplug: save %s: %w", path, err)
	}
	// Encode already appends a single trailing newline, matching the old
	// MarshalIndent+append('\n') output exactly — do NOT append another.
	b := buf.Bytes()

	if len(b) > maxCodeplugFileSize {
		return fmt.Errorf("codeplug: save %s: %w", path, &OversizeSaveError{Path: path, Size: len(b)})
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".codeplug-*.tmp")
	if err != nil {
		return fmt.Errorf("codeplug: save %s: %w", path, err)
	}
	tmpName := tmp.Name()
	// Removed on any early return below; once Rename succeeds this is a
	// no-op because tmpName no longer exists under that name.
	removeTmp := true
	defer func() {
		if removeTmp {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("codeplug: save %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("codeplug: save %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("codeplug: save %s: %w", path, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("codeplug: save %s: %w", path, err)
	}
	removeTmp = false

	// Best-effort directory fsync; see the doc comment above for why a
	// failure here is not itself an error.
	if d, dirErr := os.Open(dir); dirErr == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	return nil
}

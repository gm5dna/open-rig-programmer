// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CurrentSchema is the schema version this package writes, and the
// highest version it will read. Load rejects a file whose Schema is
// greater than CurrentSchema (see ErrSchemaTooNew) rather than guessing
// at forward compatibility with a format it does not know.
const CurrentSchema = 2

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
// schema 1; only "menus.legacy" in schema 2 — see MenuSnapshot.Legacy) —
// repeats the same key twice. encoding/json's normal behaviour for a
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

// menusLegacyExempt exempts ONLY "menus.legacy" — the schema-2 rule. In
// schema 2 the "menus" object is itself schema-controlled (its keys are
// policed like any other), and the sole opaque bag is MenuSnapshot.Legacy,
// which preserves a migrated v1 payload verbatim (duplicate keys included).
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
//     "menus.legacy" in schema 2 (see MenuSnapshot.Legacy);
//   - trailing data after the top-level JSON value is rejected;
//   - a Schema of 0 or negative, and a Schema greater than CurrentSchema
//     (see ErrSchemaTooNew), are both rejected via a wrapped *SchemaError;
//     on success, Schema == CurrentSchema.
//
// Load decodes VERSION-FIRST, in two passes. Pass 1 leniently probes only
// the "schema" field and rejects an out-of-range version straight away, so
// a file from a newer program reports "upgrade the app" (ErrSchemaTooNew)
// rather than a misleading unknown-field error from a premature strict
// decode. Pass 2 then does the strict, version-appropriate decode. A
// schema-1 file is migrated on load — its opaque "menus" payload preserved
// verbatim as MenuSnapshot.Legacy (a literal null or absent key becomes a
// nil snapshot; anything else, including {}, is preserved) — and its
// in-memory Schema set to CurrentSchema so Save always re-emits schema 2.
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

// loadV2 strictly decodes a schema-2 file into the current Codeplug shape
// (typed *MenuSnapshot), checks for trailing data and duplicate keys
// (exempting only menus.legacy), and validates the menu snapshot.
func loadV2(b []byte, path string) (*Codeplug, error) {
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

// codeplugV1 is the schema-1 on-disk shape: identical to Codeplug except
// that Menus is the opaque json.RawMessage the v1.1 reservation carried.
// loadV1 decodes into this, then migrates it into a current Codeplug.
type codeplugV1 struct {
	Schema    int             `json:"schema"`
	Generator string          `json:"generator"`
	Radio     RadioInfo       `json:"radio"`
	Channels  []Channel       `json:"channels"`
	Menus     json.RawMessage `json:"menus,omitempty"`
}

// loadV1 strictly decodes a schema-1 file (whole-"menus" duplicate-key
// exemption, matching how v1 was written), then migrates it to the current
// schema: the opaque menus payload is preserved verbatim as
// MenuSnapshot.Legacy, and Schema is set to CurrentSchema so Save re-emits
// schema 2.
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
		Schema:    CurrentSchema, // migrate-on-load: Save always emits schema 2.
		Generator: v1.Generator,
		Radio:     v1.Radio,
		Channels:  v1.Channels,
	}
	if migratedMenusPresent(v1.Menus) {
		cp.Menus = &MenuSnapshot{Legacy: append(json.RawMessage(nil), v1.Menus...)}
	}
	return cp, nil
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
func Save(path string, cp *Codeplug) error {
	if err := cp.Menus.Validate(); err != nil {
		return fmt.Errorf("codeplug: save %s: %w", path, err)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cp); err != nil {
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

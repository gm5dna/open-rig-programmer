// SPDX-License-Identifier: GPL-3.0-or-later

// Package userconfig persists the small set of decisions a user makes
// once and expects the programme to remember — at present exactly one:
// whether they have consented to unverified writes for a given radio
// model.
//
// The store is a single JSON object at DefaultPath(), shared by the CLI
// and the GUI, and it is designed around three rules that the rest of
// this package exists to keep:
//
//   - A DECLINE IS A DECISION. "Never asked" and "asked and said no" are
//     different states and are stored differently: an explicit false is
//     written to the file, not a deleted key. Callers distinguish them
//     through Settings.UnverifiedWritesFor's second return value, so a
//     user who has refused is not asked again on every session.
//   - A FILE THIS BUILD CANNOT UNDERSTAND IS NEVER OVERWRITTEN. Both
//     Load and SetUnverifiedWrites refuse, with an error naming the path
//     and telling the user to repair it by hand. Silently resetting a
//     corrupt settings file would discard consent decisions and any other
//     preferences without a word.
//   - A NEWER BUILD'S SETTINGS SURVIVE AN OLDER BUILD'S WRITE. Writes
//     merge into the raw JSON object rather than re-serialising a Go
//     struct, so keys this build has never heard of are carried through
//     untouched.
//
// Model slugs are OPAQUE here: this package neither validates them nor
// knows what a model is, and deliberately does not import
// internal/wiring. The caller decides what a slug means; the store only
// remembers what it was told.
package userconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// appDirName is the per-application directory beneath
	// os.UserConfigDir(), matching the one the snapshot/journal paths
	// already use.
	appDirName = "rigprog"

	// settingsFileName is the single settings file the CLI and the GUI
	// share.
	settingsFileName = "settings.json"

	// unverifiedWritesKey is the top-level JSON key holding the
	// per-model consent map. It is named here as well as in Settings'
	// struct tag because the merge path works on the raw object and must
	// address the key without going through the struct.
	unverifiedWritesKey = "unverifiedWrites"

	// configDirPerm is the mode new parent directories are created with.
	// An existing directory's mode is left alone — a user who has
	// tightened it to 0700 meant it.
	configDirPerm = 0o755

	// settingsFilePerm is the settings file's mode: owner-only, because
	// the file records which radios a particular operator owns.
	settingsFilePerm = 0o600
)

// Settings is the decoded settings file.
//
// The zero value is valid and records nothing: it is what Load returns
// for a file that does not exist yet, and it answers every query with
// "never asked".
type Settings struct {
	// UnverifiedWrites maps a model slug to the user's decision about
	// unverified writes for that model. A key's PRESENCE is the record
	// that a decision was taken; its value is the decision. false is a
	// meaningful, stored value — see UnverifiedWritesFor.
	UnverifiedWrites map[string]bool `json:"unverifiedWrites"`
}

// UnverifiedWritesFor reports the user's recorded decision about
// unverified writes for slug.
//
// recorded is map-key presence: true if the user has been asked and
// answered, false if this model has never come up. granted is their
// answer. The three reachable combinations are:
//
//	(false, false) — never asked. The caller should prompt.
//	(true,  true)  — consent granted. Pass the driver's
//	                 WithConsentedUnverifiedWrites option.
//	(false, true)  — declined, or granted and later revoked. The caller
//	                 must NOT prompt again and must NOT pass the option.
//
// (true, false) is unreachable by construction.
//
// It is safe to call on the zero Settings.
func (s Settings) UnverifiedWritesFor(slug string) (granted, recorded bool) {
	granted, recorded = s.UnverifiedWrites[slug]
	return granted, recorded
}

// DefaultPath returns the shared settings file's location,
// "<os.UserConfigDir()>/rigprog/settings.json".
//
// It does not touch the filesystem: neither the directory nor the file
// need exist. SetUnverifiedWrites creates both on demand, and Load treats
// a missing file as an empty one.
func DefaultPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("userconfig: determining the settings directory: %w", err)
	}
	return filepath.Join(cfgDir, appDirName, settingsFileName), nil
}

// Load reads the settings file at path.
//
// A file that does not exist is not an error: Load returns the zero
// Settings, which records nothing. Any other failure — unreadable file,
// malformed JSON, a value of the wrong type — IS an error, and Load
// returns it rather than quietly substituting empty settings. That
// distinction matters: silently treating a corrupt file as empty would
// mean a user's recorded decisions vanish without anyone being told, and
// the next write would replace the file they might still have repaired.
//
// Load never writes anything, including on the corrupt path.
func Load(path string) (Settings, error) {
	raw, err := loadRawObject(path)
	if err != nil {
		return Settings{}, err
	}
	consent, err := decodeUnverifiedWrites(raw, path)
	if err != nil {
		return Settings{}, err
	}
	return Settings{UnverifiedWrites: consent}, nil
}

// SetUnverifiedWrites records the user's decision about unverified writes
// for slug and persists it to the settings file at path, creating the
// file and its parent directory if they do not exist.
//
// on is stored as given. A false is WRITTEN, not deleted: a decline (or a
// revocation of an earlier grant) is a decision the user made, and it has
// to survive so they are not asked the same question at every session.
// Deleting the key would demote it back to "never asked".
//
// Writes merge into the file's raw JSON object, so top-level keys this
// build does not recognise — a newer build's settings, a key added by a
// future feature — are preserved exactly. Only the unverifiedWrites entry
// for slug changes; other slugs' entries are untouched.
//
// If the existing file cannot be parsed, SetUnverifiedWrites refuses and
// returns an error naming the path: it cannot merge into bytes it does
// not understand, and overwriting them would destroy whatever the user
// still had. The original file is left byte-for-byte as it was.
//
// The replacement is atomic. The new content is written to a uniquely
// named temporary file in the SAME directory (so the rename is
// same-filesystem), checked to be non-empty and to parse back into the
// value just set, and only then renamed onto path. A reader therefore
// sees either the complete old file or the complete new one, never a
// half-written one, and a failure at any step leaves the old file intact.
// The parent directory is created 0755 if absent (an existing one's mode
// is left alone) and the file is 0600.
//
// CONCURRENCY: last writer wins, by design. Two writers — typically the
// CLI and the GUI running at once — can each load, modify and replace the
// file, and the later rename silently discards the earlier one's change.
// The unique temporary names mean a torn or interleaved file is
// impossible; nothing here prevents a lost update. This was adjudicated
// at plan review and accepted, because the failure direction is safe:
// consent is read only when a session is CONSTRUCTED, so a lost update
// can at worst mean one session runs with the previous decision and the
// user re-answers; and the GUI's amber unverified-write indicator renders
// from the session's actual capabilities, never from this file, so the
// interface cannot show consent that the running session does not have.
// No lock is taken and no test pins the interleaving — no test could
// honestly do so — which is why the limitation is recorded here.
func SetUnverifiedWrites(path, slug string, on bool) error {
	raw, err := loadRawObject(path)
	if err != nil {
		return err
	}
	consent, err := decodeUnverifiedWrites(raw, path)
	if err != nil {
		return err
	}

	if consent == nil {
		consent = make(map[string]bool, 1)
	}
	consent[slug] = on

	encodedConsent, err := encodeJSON(consent, "")
	if err != nil {
		return fmt.Errorf("userconfig: encoding settings for %s: %w", path, err)
	}
	if raw == nil {
		raw = make(map[string]json.RawMessage, 1)
	}
	raw[unverifiedWritesKey] = json.RawMessage(bytes.TrimRight(encodedConsent, "\n"))

	// Indented with a trailing newline: the corrupt-file error tells the
	// user to repair this by hand, so it has to be legible by hand.
	out, err := encodeJSON(raw, "  ")
	if err != nil {
		return fmt.Errorf("userconfig: encoding settings for %s: %w", path, err)
	}
	if err := verifyEncoded(out, slug, on); err != nil {
		return fmt.Errorf("userconfig: refusing to replace %s: %w", path, err)
	}

	return replaceAtomically(path, out)
}

// loadRawObject reads path and decodes it as a generic JSON object,
// preserving each top-level value's bytes verbatim so unknown keys can be
// written back unchanged.
//
// A missing file yields (nil, nil) — the caller treats a nil map as an
// empty object. Anything else that goes wrong is an error.
func loadRawObject(path string) (map[string]json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("userconfig: reading %s: %w", path, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, corruptFileError(path, err)
	}
	// A file containing the literal "null" decodes into a nil map without
	// error; treat it as empty rather than corrupt, since it round-trips
	// to the same meaning.
	return raw, nil
}

// decodeUnverifiedWrites extracts the consent map from an already-decoded
// raw object. An absent key yields (nil, nil); a value of the wrong shape
// is a corrupt file, not an empty one.
func decodeUnverifiedWrites(raw map[string]json.RawMessage, path string) (map[string]bool, error) {
	v, ok := raw[unverifiedWritesKey]
	if !ok {
		return nil, nil
	}
	var consent map[string]bool
	if err := json.Unmarshal(v, &consent); err != nil {
		return nil, corruptFileError(path, fmt.Errorf("%q: %w", unverifiedWritesKey, err))
	}
	return consent, nil
}

// corruptFileError builds the one error text the corrupt path uses. It
// names the file and says plainly that this programme will not touch it,
// because nothing in this package can repair it: SetUnverifiedWrites
// refuses for the same reason Load does.
func corruptFileError(path string, cause error) error {
	return fmt.Errorf("userconfig: %s is not a settings file this build can read (%w); it has been left exactly as it is — delete or repair the file by hand", path, cause)
}

// encodeJSON marshals v with HTML escaping DISABLED, so that '<', '>',
// '&' and U+2028/U+2029 inside a preserved unknown value survive
// byte-for-byte rather than being rewritten as escapes. indent is passed
// to json.Encoder.SetIndent; "" gives compact output. The result carries
// the Encoder's single trailing newline.
func encodeJSON(v any, indent string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// verifyEncoded is the pre-rename check: the bytes about to replace a
// user's settings must be non-empty and must parse back into exactly the
// decision just taken. It guards against an empty or truncated buffer
// reaching os.Rename, where it would become the file.
func verifyEncoded(b []byte, slug string, on bool) error {
	if len(b) == 0 {
		return errors.New("the encoded settings were empty")
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("the encoded settings did not parse back: %w", err)
	}
	consent, err := decodeUnverifiedWrites(raw, "the encoded settings")
	if err != nil {
		return err
	}
	got, recorded := Settings{UnverifiedWrites: consent}.UnverifiedWritesFor(slug)
	if !recorded || got != on {
		return fmt.Errorf("the encoded settings do not record %q as %v (got granted=%v, recorded=%v)", slug, on, got, recorded)
	}
	return nil
}

// replaceAtomically writes b to path via a uniquely named temporary file
// in the same directory, then renames it into place. Any failure before
// the rename removes the temporary file and leaves path as it was.
func replaceAtomically(path string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, configDirPerm); err != nil {
		return fmt.Errorf("userconfig: creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "settings-*.json")
	if err != nil {
		return fmt.Errorf("userconfig: creating a temporary file beside %s: %w", path, err)
	}
	tmpName := tmp.Name()
	// Cleared once the rename succeeds, after which tmpName no longer
	// exists under that name and the removal would be a no-op anyway.
	removeTmp := true
	defer func() {
		if removeTmp {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("userconfig: writing %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("userconfig: writing %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("userconfig: writing %s: %w", path, err)
	}
	// os.CreateTemp already asks for 0600, but the process umask can
	// clear bits from it; set the mode explicitly so the file that lands
	// at path is owner-only whatever the umask was.
	if err := os.Chmod(tmpName, settingsFilePerm); err != nil {
		return fmt.Errorf("userconfig: setting the mode of %s: %w", path, err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("userconfig: replacing %s: %w", path, err)
	}
	removeTmp = false

	// Best-effort directory fsync so the rename itself survives a power
	// loss. It is unsupported on some platforms (Windows) and filesystems,
	// and by this point the rename has already completed, so a failure
	// here is not reported: what is lost is crash durability, not the
	// atomicity a concurrent reader observes.
	if d, dirErr := os.Open(dir); dirErr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

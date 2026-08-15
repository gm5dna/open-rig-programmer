// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/radiotext"
	"github.com/gm5dna/open-rig-programmer/internal/userconfig"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// This file is the GUI's consent surface: the bound methods a grants
// panel and an arming dialogue are built on, and the helper the connect
// path (connection.go) reads a recorded decision through.
//
// Two rules govern everything below, and they are the CLI's rules too
// (cmd/rigprog/settings.go, cmd/rigprog/wiring.go), deliberately, because
// a user may drive this project from either surface and must not find them
// disagreeing:
//
//   - ELIGIBILITY IS ASKED, NEVER RESTATED. Which radios consent applies
//     to comes from wiring.NeedsUnverifiedConsent and nowhere else. A
//     private copy here would let the GUI offer a grant the CLI refuses,
//     or ask an FT-710 owner to authorise an unverified write that cannot
//     exist.
//   - A STORE THIS BUILD CANNOT READ STOPS THE CALL, in userconfig's own
//     words. Both other readings are dishonest — treating it as
//     unconsented would refuse writes the user had authorised, treating it
//     as consented would authorise writes they had not — and the message
//     userconfig produces names the file and says how to repair it, which
//     a generic "could not read settings" would throw away. Its text
//     therefore reaches the frontend VERBATIM, unwrapped.

// userConfigPath resolves this machine's shared settings file — the store
// holding the user's recorded consent decisions (internal/userconfig),
// shared with the CLI, which reads and writes the same file through its
// own copy of this seam.
//
// A package-level variable rather than a direct call so that a test can
// point the whole app at a file under t.TempDir(): the production location
// is the REAL user's settings file, and no test may read, still less
// write, the settings of whoever runs "go test". Reassigned ONLY by tests
// (see consent_test.go's tempUserConfig), restored via t.Cleanup;
// production code must never reassign it, and no flag or preference can.
var userConfigPath = userconfig.DefaultPath

// consentDecision is what the store and the shared eligibility predicate
// together say about ONE radio: everything the GUI needs to decide whether
// to ask, what to show, and what to open a session with.
//
// Eligible is a fact about the RADIO; Granted and Recorded are facts about
// the USER. Keeping all three in one value is what stops the connect path
// and the consent panel deriving them separately and disagreeing.
type consentDecision struct {
	// Model is the canonical model name the decision belongs to.
	Model string
	// Eligible is wiring.NeedsUnverifiedConsent's answer: whether an
	// unverified write exists on this radio for a consent to unlock.
	Eligible bool
	// Granted and Recorded are the store's two answers (see
	// userconfig.Settings.UnverifiedWritesFor). Both are forced FALSE when
	// Eligible is false — see consentFrom for why.
	Granted  bool
	Recorded bool
}

// sessionOptions turns a decision into the options a real session for this
// radio should be opened with. It is the GUI's equivalent of
// cmd/rigprog's sessionOptionsFor, and spends exactly the same fact.
//
// A non-eligible radio contributes no consent at all (Granted is already
// false by then), which makes a stale key in the settings file
// unspendable. The session it produces is identical either way —
// spec.ConsentUnverifiedWrites changes nothing in a capability set with no
// write-side unverified field — so this costs nothing and keeps the
// no-op explicit rather than incidental.
func (d consentDecision) sessionOptions() wiring.SessionOptions {
	return wiring.SessionOptions{ConsentUnverifiedWrites: d.Granted}
}

// view renders a decision as the struct the frontend receives.
func (d consentDecision) view() UnverifiedWriteConsentView {
	warning := ""
	if d.Eligible {
		warning = fmt.Sprintf(radiotext.UnverifiedWriteWarningTemplate, d.Model)
	}
	return UnverifiedWriteConsentView{
		Model:        d.Model,
		NeedsConsent: d.Eligible,
		Granted:      d.Granted,
		Recorded:     d.Recorded,
		Warning:      warning,
	}
}

// consentFor resolves one model's consent state: the shared eligibility
// predicate, plus the user's recorded decision from the settings store.
//
// An unrecognised model fails with internal/wiring's own
// *UnknownModelError, from the predicate itself, BEFORE the store is
// opened — so an unnameable radio can neither create the settings file nor
// read it. An ABSENT store is not a failure: it is a first-run user, and
// userconfig.Load reports it as the zero Settings, which answers "never
// asked" for every model.
func consentFor(model string) (consentDecision, error) {
	eligible, err := wiring.NeedsUnverifiedConsent(model)
	if err != nil {
		return consentDecision{}, err
	}
	settings, err := loadConsentSettings()
	if err != nil {
		return consentDecision{}, err
	}
	return consentFrom(settings, model, eligible), nil
}

// loadConsentSettings reads the whole settings store once. Both readers
// below go through it — ListUnverifiedWriteConsents in particular loads
// ONCE for the whole listing rather than per model, so every row it
// returns describes the same file at the same moment.
func loadConsentSettings() (userconfig.Settings, error) {
	path, err := userConfigPath()
	if err != nil {
		return userconfig.Settings{}, err
	}
	return userconfig.Load(path)
}

// consentFrom builds one model's decision from already-loaded settings.
//
// The slug is wiring.ModelSlug(model) — the same filesystem-safe form the
// per-model snapshot directories use, so one radio has one name across
// everything this programme stores about it.
//
// A NON-ELIGIBLE radio reports Granted and Recorded false whatever the
// file says. A key under its slug can only have come from a hand-edited
// file, or from a build in which that radio's writes were still unverified
// — and in neither case is it a live grant: there is no unverified write
// left for it to unlock. Reporting it would put a "consented" row in front
// of a user for a radio the same panel says needs no consent, and
// (through sessionOptions) would spend a decision on a session that has
// nothing to spend it on. The key itself is left untouched: this package
// reads the store, it does not tidy it.
func consentFrom(settings userconfig.Settings, model string, eligible bool) consentDecision {
	d := consentDecision{Model: model, Eligible: eligible}
	if !eligible {
		return d
	}
	d.Granted, d.Recorded = settings.UnverifiedWritesFor(wiring.ModelSlug(model))
	return d
}

// GetUnverifiedWriteConsent reports one radio's consent state: whether it
// is consent-eligible at all, whether the user has answered, what they
// answered, and the warning text they would be shown if asked. See
// UnverifiedWriteConsentView for what each field means and why all four
// are needed.
//
// Reading is side-effect free: an absent settings file stays absent, and
// nothing is created by asking.
func (a *App) GetUnverifiedWriteConsent(model string) (UnverifiedWriteConsentView, error) {
	d, err := consentFor(model)
	if err != nil {
		return UnverifiedWriteConsentView{}, err
	}
	return d.view(), nil
}

// SetUnverifiedWriteConsent records the user's decision about unverified
// writes for model — true to consent, false to withhold or revoke — and
// persists it to the shared settings store.
//
// A FALSE is stored, not deleted (userconfig.SetUnverifiedWrites): a
// decline is a decision, and it has to survive so the user is not asked
// the same question at every connection.
//
// Refusals, in order, and both BEFORE the store is touched:
//
//  1. an unrecognised model, with internal/wiring's own typed error;
//  2. a model whose writes are hardware-verified — there is no unverified
//     write here for a consent to unlock, and recording a decision about
//     nothing would be a lie the user could later act on. The wording
//     matches cmd/rigprog's refusal for the same case.
//
// A decision taken while connected does NOT change the running session:
// consent is spent when a session is constructed, so it applies from the
// next connection onwards. Nothing here pretends otherwise, and the amber
// indicator (UISpecView.UnverifiedWritesConsented) keeps reporting the
// live session's own capabilities meanwhile, so the interface cannot show
// an arming that has not happened.
func (a *App) SetUnverifiedWriteConsent(model string, on bool) error {
	eligible, err := wiring.NeedsUnverifiedConsent(model)
	if err != nil {
		return err
	}
	if !eligible {
		return fmt.Errorf("app: unverified-write consent applies only to models with unverified write support; the %s's writes are hardware-verified, so there is no unverified write here for a consent to unlock", model)
	}
	path, err := userConfigPath()
	if err != nil {
		return err
	}
	// userconfig's own wording, verbatim: on the corrupt-file path it names
	// the file and tells the user to repair it by hand, which a generic
	// "could not save settings" would throw away.
	return userconfig.SetUnverifiedWrites(path, wiring.ModelSlug(model), on)
}

// ListUnverifiedWriteConsents returns one row per model this build
// supports, in wiring.SupportedModels' own order — the grants panel's data
// source, so a management surface built on it can never offer, or omit, a
// model the connect path knows about.
//
// Hardware-verified models are LISTED, with NeedsConsent false, rather
// than filtered out: a user looking for a radio should find it and be told
// there is nothing to decide, not be left wondering whether the panel has
// forgotten it.
func (a *App) ListUnverifiedWriteConsents() ([]UnverifiedWriteConsentView, error) {
	settings, err := loadConsentSettings()
	if err != nil {
		return nil, err
	}
	models := wiring.SupportedModels()
	out := make([]UnverifiedWriteConsentView, 0, len(models))
	for _, model := range models {
		eligible, err := wiring.NeedsUnverifiedConsent(model)
		if err != nil {
			// Unreachable: model came from SupportedModels() itself.
			// Returned rather than ignored so a future divergence between
			// the two fails loudly instead of listing a wrong state.
			return nil, err
		}
		out = append(out, consentFrom(settings, model, eligible).view())
	}
	return out, nil
}

// consentedUnverifiedWrites reports whether caps carry a write-side
// spec.ConsentedUnverified anywhere: whether THIS capability set is one a
// user's consent has already opened.
//
// It is the derivation behind UISpecView.UnverifiedWritesConsented, and it
// asks the capability set in hand rather than the settings file on
// purpose (see that field's doc comment). A static baseline can never
// answer true — the consent transform is applied at session assembly and
// leaves a driver's static set untouched (wiring.OpenRealSessionWith) — so
// offline and demo callers get false from the same three lines, with no
// special case to keep in step.
func consentedUnverifiedWrites(caps spec.Capabilities) bool {
	for _, b := range caps.Banks {
		for _, fs := range b.Fields {
			if fs.Write == spec.ConsentedUnverified {
				return true
			}
		}
	}
	return false
}

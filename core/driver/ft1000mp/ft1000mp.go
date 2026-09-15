// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

var _ driver.Driver = (*ft1000mpDriver)(nil)
var _ driver.Session = (*Session)(nil)

// identityTimeout bounds the FAH identity probe: a 5-byte reply at 4800
// bit/s is near-instant, so this need not be generous — a small multiple
// of transport.DefaultTimeout is enough headroom over a real link.
const identityTimeout = 2 * time.Second

// dumpTimeout bounds the U=00H full-dump read: 1,863 bytes at 4800 bit/s
// is ~3.9 s (matrix §1.9) — transport.DefaultTimeout (1 s) is far too
// short, so this driver states its own explicit timeout rather than
// inheriting a default sized for short NEWCAT answers.
const dumpTimeout = 6 * time.Second

// New builds the FT-1000MP driver for profile. RealHardware — the zero
// value — selects the all-Unverified capability set (doc.go's "Override
// in force" note): Store/Enter and every other write-side field is
// Unverified until WithConsentedUnverifiedWrites is also supplied.
func New(profile Profile, opts ...Option) driver.Driver {
	d := &ft1000mpDriver{Base: driver.Base{Profile: profile}}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Option configures the driver New builds.
type Option func(*ft1000mpDriver)

// WithConsentedUnverifiedWrites records that the user has consented to
// writing this radio's Unverified fields — see driver.Base.SessionCaps
// and core/driver/ftdx10's identically-named option, whose reasoning
// applies unchanged here.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ft1000mpDriver) { d.Consented = true }
}

// ft1000mpDriver implements driver.Driver.
type ft1000mpDriver struct {
	driver.Base
}

// Model implements driver.Driver.
func (d *ft1000mpDriver) Model() string { return modelName }

// Capabilities implements driver.Driver: the static baseline for this
// driver's profile.
func (d *ft1000mpDriver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return CapabilitiesSimulated()
	case RealHardware:
		return CapabilitiesUnverified()
	default:
		// An unrecognised Profile fails the same way RealHardware does,
		// through its own explicit arm (core/driver/ftdx10's identical
		// reasoning: a reader must see the fail-safe is a decision, not
		// a coincidence of the switch's shape).
		return CapabilitiesUnverified()
	}
}

// sessionCapabilities is the one place a session's effective capability
// set is assembled: the static baseline, then — only when this driver was
// built with WithConsentedUnverifiedWrites AND its profile is one of the
// declared constants — the consent transform (driver.Base.SessionCaps).
func (d *ft1000mpDriver) sessionCapabilities() spec.Capabilities {
	return d.SessionCaps(d.Capabilities())
}

// Open implements driver.Driver: it builds the transport.Engine over
// port, probes identity (FAH), and returns a Session. Open takes
// ownership of port on both outcomes.
func (d *ft1000mpDriver) Open(ctx context.Context, port transport.Port, id driver.Identity) (driver.Session, error) {
	fr, err := bincat.NewFraming(bincatProfile())
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ft1000mp: Open: %w", err)
	}
	eng, err := transport.NewEngineWith(port, fr)
	if err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("ft1000mp: Open: %w", err)
	}
	sess, err := d.open(ctx, eng, id)
	if err != nil {
		_ = eng.Close()
		return nil, err
	}
	return sess, nil
}

// open is Open's body, factored so the error path can close eng in
// exactly one place.
func (d *ft1000mpDriver) open(ctx context.Context, eng *transport.Engine, id driver.Identity) (*Session, error) {
	// InitSequence is empty for this family (bincat.framing.InitSequence),
	// so Init only drains any stale bytes before the identity probe.
	if err := eng.Init(ctx); err != nil {
		return nil, fmt.Errorf("ft1000mp: Open: %w", err)
	}

	if err := probeIdentity(ctx, eng); err != nil {
		return nil, err
	}
	id.CATID = catID

	return &Session{
		eng:  eng,
		id:   id,
		caps: d.sessionCapabilities(),
	}, nil
}

// probeIdentity sends FAH (F=IdentityShort) and confirms the reply's
// trailing two bytes are the Model ID 03H,93H (matrix §1.7). A wrong pair
// or silence both mean "not this radio" — errors.Is(err,
// driver.ErrWrongRadio).
func probeIdentity(ctx context.Context, eng *transport.Engine) error {
	cmd := bincat.NewCommand(bincat.OpIdentity, [4]byte{0, 0, 0, bincat.IdentityShort})
	frame, err := eng.Do(ctx, cmd, bincat.ReadSpec(identityTimeout, 0))
	if err != nil {
		return &driver.WrongRadioError{Want: modelIDHex, Got: "(no answer)", WantModel: modelName}
	}
	if len(frame) != 5 || frame[3] != modelIDByte1 || frame[4] != modelIDByte2 {
		return &driver.WrongRadioError{Want: modelIDHex, Got: fmt.Sprintf("% x", frame), WantModel: modelName}
	}
	return nil
}

const (
	modelIDByte1 = 0x03
	modelIDByte2 = 0x93
	modelIDHex   = "03 93"
)

// Session is the FT-1000MP's driver.Session: one open, identity-verified
// connection. Safe for concurrent use — transport.Engine serialises
// individual exchanges, and opMu serialises whole driver operations on
// top of that (the same convention every driver in this project's Yaesu
// tier uses, e.g. core/driver/ftdx10.Session).
type Session struct {
	eng  *transport.Engine
	id   driver.Identity
	caps spec.Capabilities // effective; never mutated after Open

	opMu sync.Mutex

	// dump is the cached U=00H full dump (doc.go's read design note):
	// fetched once, on the first ReadChannel call, and reused for every
	// later read this session makes. Guarded by opMu — every ReadChannel
	// and WriteChannel call already holds it for the whole operation.
	dump []byte
}

// Identity implements driver.Session.
func (s *Session) Identity() driver.Identity { return s.id }

// Capabilities implements driver.Session: a defensive copy of this
// session's effective capabilities.
func (s *Session) Capabilities() spec.Capabilities { return s.caps.Clone() }

// Close implements driver.Session. Idempotent.
func (s *Session) Close() error { return s.eng.Close() }

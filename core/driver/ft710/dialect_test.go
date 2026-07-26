// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// TestCATID_ComesFromTheDialect pins the linkage Task 56 introduced: the
// driver's identity value is DERIVED from the codec's dialect, not
// restated alongside it, so the two cannot drift. The second assertion
// keeps the documented literal in the test rather than only in the code —
// a dialect edit that silently changed the FT-710's CAT ID would fail
// here rather than quietly redefining what "an FT-710" means.
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != cat.FT710.CATID() {
		t.Errorf("catID = %q, want cat.FT710.CATID() = %q", catID, cat.FT710.CATID())
	}
	if catID != "0800" {
		t.Errorf("catID = %q, want the documented %q — golden vector G1", catID, "0800")
	}
}

// TestDriver_CarriesADialect pins that New actually populates the field
// Open reads to gate the engine. A zero cat.Dialect would still compile
// and would still produce a non-nil AllowedCommand method value that
// transport.NewEngine accepts — it just would not accept a single frame
// (see cat.Dialect's doc comment), so every session would fail on its own
// AI0 init. Checking Configured() here catches that at the composition,
// where the cause is legible, rather than as a mystifying refusal on the
// wire.
func TestDriver_CarriesADialect(t *testing.T) {
	d, ok := New(Simulated).(*ft710Driver)
	if !ok {
		t.Fatal("New did not return a *ft710Driver")
	}
	if !d.dialect.Configured() {
		t.Fatal("ft710Driver.dialect is a zero cat.Dialect — New must initialise it (an unconfigured gate accepts nothing, so every frame would be refused)")
	}
	if d.dialect.CATID() != cat.FT710.CATID() {
		t.Errorf("driver dialect CATID = %q, want the FT-710's %q", d.dialect.CATID(), cat.FT710.CATID())
	}
}

// TestSession_CarriesTheDriversDialect pins the other half of the plumb:
// the Session a real Open returns codes and decodes through the SAME
// dialect the transport engine was gated with. A session that encoded with
// one radio's dialect while its engine gated with another's would be the
// exact failure this milestone exists to prevent, and it would be
// invisible whilst only one dialect exists.
func TestSession_CarriesTheDriversDialect(t *testing.T) {
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

	if !sess.dialect.Configured() {
		t.Fatal("Session.dialect is a zero cat.Dialect — Open must copy the driver's")
	}
	if sess.dialect.CATID() != cat.FT710.CATID() {
		t.Errorf("Session dialect CATID = %q, want the FT-710's %q — a session must code through the same dialect its engine gates with", sess.dialect.CATID(), cat.FT710.CATID())
	}
}

// TestOpen_UnconfiguredDialectRefusesEveryFrame is the driver-side half of
// the fail-closed story. An ft710Driver whose dialect field is zero-valued
// still hands transport.NewEngine a NON-nil AllowedCommand method value,
// so the constructor's nil check does not (and should not) fire — what
// stops it instead is that an unconfigured dialect's gate accepts nothing,
// so the very first frame the session sends is refused before it reaches
// the wire. Pinned here so the two halves are known to compose: a MISSING
// gate is refused at construction (core/transport's allowfunc_test.go), an
// EMPTY one at the write.
func TestOpen_UnconfiguredDialectRefusesEveryFrame(t *testing.T) {
	r := fakeradio.New(fakeradio.WithFactoryImage(minimalImage))
	t.Cleanup(func() { _ = r.Close() })

	// Deliberately NOT via New: a hand-built driver with the zero dialect.
	d := &ft710Driver{profile: Simulated}

	sess, err := d.Open(testCtx(t), r.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open with a zero dialect succeeded — an unconfigured gate must refuse every frame, so nothing can get out")
	}
	if !errors.Is(err, transport.ErrDisallowedCommand) {
		t.Errorf("Open with a zero dialect = %v, want errors.Is match against transport.ErrDisallowedCommand", err)
	}
}

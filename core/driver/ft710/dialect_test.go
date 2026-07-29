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
// Open hands the engine. A zero cat.Dialect would still compile, and
// (before M9c-5) would still have produced a non-nil AllowedCommand
// method value that transport.NewEngine's nil check could not see, so
// every session would have failed on its own AI0 init instead. Since
// M9c-5 the constructor takes the dialect whole and refuses an
// unconfigured one outright (see TestOpen_UnconfiguredDialectRefusesToOpen
// below), but this check stays: it catches the omission at the
// composition, where the cause is legible, rather than at the first Open.
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

// TestOpen_UnconfiguredDialectRefusesToOpen is the driver-side half of the
// fail-closed story, and M9c-5 (E3) moved WHERE it fires. Until then an
// ft710Driver whose dialect field was zero-valued still handed
// transport.NewEngine a NON-nil AllowedCommand method value, so the
// constructor's nil check could not fire; what stopped the session was
// that an unconfigured dialect's gate accepts nothing, so its very first
// frame was refused at the write (transport.ErrDisallowedCommand). Now
// NewEngine takes the cat.Dialect whole and checks Configured(), so the
// refusal happens EARLIER and says something truer — the driver was never
// given a radio to speak for — and no engine, reader goroutine or wire
// exchange is created on the way to finding out.
//
// The wire-level backstop has not gone anywhere: core/cat still refuses
// every frame for an unconfigured dialect (its own dialect_test.go pins
// it), and Do still consults the gate before every write. This test now
// pins the outermost of the three refusals, which is the one a
// misassembled driver actually meets.
func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	r := fakeradio.New(fakeradio.WithFactoryImage(minimalImage))
	t.Cleanup(func() { _ = r.Close() })

	// Deliberately NOT via New: a hand-built driver with the zero dialect.
	d := &ft710Driver{profile: Simulated}

	sess, err := d.Open(testCtx(t), r.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open with a zero dialect succeeded — a driver with no dialect must not reach the wire at all")
	}
	if !errors.Is(err, transport.ErrUnconfiguredDialect) {
		t.Errorf("Open with a zero dialect = %v, want errors.Is match against transport.ErrUnconfiguredDialect", err)
	}
}

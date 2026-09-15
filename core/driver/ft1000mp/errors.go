// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import "errors"

// errRecord is this package's own record-decode sentinel — record.go's
// parseRecord wraps it, mirroring core/bincat.ErrRecord's role for the
// FT-890/900 shape this driver deliberately does not reuse (record.go's
// doc comment).
var errRecord = errors.New("ft1000mp: record")

// ErrStoreUnresolved is the sentinel WriteChannel's returned error wraps
// when the Store/Enter step (§Write model step 8) itself failed: per the
// 15/09/2026 override and spec.md's Codex #8 correction, a Store/Enter
// failure leaves the memory's TRUE state UNKNOWN until a later,
// successful read establishes it — it is never "resolved" by this
// write's own diagnostic read-back, which is explicitly non-verifying
// (driver.Session.WriteChannel's own contract: WriteChannel performs NO
// read-back verification at all — that is the clone service's job, and
// this sentinel exists only to let the clone service's own diagnostic
// path recognise this specific residual-state case rather than treat
// every write error identically).
var ErrStoreUnresolved = errors.New("ft1000mp: Store/Enter failed: memory outcome UNRESOLVED, not verified")

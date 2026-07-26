// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"testing"
)

// TestNewEngineReachableOnlyFromDriver pins the other half of M9b's
// fail-closed story. NewEngine takes the outbound allowlist as a
// parameter, so WHOEVER CALLS IT CHOOSES THE GATE. That choice belongs to
// the driver layer: a call site in app/ or cmd/ could pass a permissive
// func and bypass every policy layer above it.
//
// Matches both qualified calls (transport.NewEngine) and bare identifiers
// (a dot-import, which the plan review flagged as an evasion the first
// draft missed). core/transport is SCANNED, not skipped — only the
// declaration itself is exempt, so a wrapper constructor there cannot
// hide.
func TestNewEngineReachableOnlyFromDriver(t *testing.T) {
	files := parseRepo(t)

	sawDriverConstruction := false
	scanned := 0

	for _, pf := range files {
		scanned++
		ast.Inspect(pf.file, func(n ast.Node) bool {
			// Exempt the declaration itself.
			if fd, ok := n.(*ast.FuncDecl); ok && fd.Name.Name == "NewEngine" && inTree(pf.relDir, "core/transport") {
				return false
			}

			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			var name string
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			case *ast.Ident:
				name = fn.Name
			default:
				return true
			}
			if name != "NewEngine" {
				return true
			}

			if inTree(pf.relDir, "core/driver") {
				sawDriverConstruction = true
				return true
			}
			t.Errorf("%s: calls NewEngine — an Engine's allowlist is chosen at construction, so only core/driver/** may construct one", pf.relPath)
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("scanned no files — the walker or its filters are broken, and this check passed vacuously")
	}
	if !sawDriverConstruction {
		t.Error("never saw core/driver/** call NewEngine — the walker or its filters are broken, and this check passed vacuously")
	}
}

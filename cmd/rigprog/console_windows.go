// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package main

import "syscall"

// The CLI's prose uses em dashes; a legacy console code page renders them
// as mojibake. Switch output to UTF-8 (65001) at start-up. Failure is
// ignored: the text is still correct, only its rendering suffers.
func init() {
	_, _, _ = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleOutputCP").Call(65001)
}

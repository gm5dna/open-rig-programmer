// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

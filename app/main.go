// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "Open Rig Programmer",
		Width:     1024,
		Height:    768,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		// Non-nil so Wails v2 computes the macOS zoomable flag at all: with
		// Mac nil the flag stays zero and the green title-bar button is
		// disabled (wails/v2 darwin/window.go, `zoomable` only set in the
		// Mac != nil branch).
		Mac:           &mac.Options{},
		OnStartup:     app.startup,
		OnBeforeClose: app.OnBeforeClose,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal("Error:", err.Error())
	}
}

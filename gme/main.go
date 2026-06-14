package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// GME's window is the visual half of the memory editor: it reads the live graph
// over the daemon's socket through the same client the console uses, so shaping
// memory and watching it are one tool.
func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "GME",
		Width:            1100,
		Height:           760,
		MinWidth:         720,
		MinHeight:        480,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 18, G: 20, B: 24, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

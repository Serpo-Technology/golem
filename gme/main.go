package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// The Golem Toolkit's window is the primary way to work a GOLEM brain: it talks
// to the running brain over the signed network transport, so conversing with it,
// feeding its senses, watching its inner life, and shaping its memory are one
// tool over the same connection the console uses.
func main() {
	app := NewApp()
	err := wails.Run(&options.App{
		Title:            "Golem Toolkit",
		Width:            1180,
		Height:           820,
		MinWidth:         820,
		MinHeight:        540,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 18, G: 20, B: 24, A: 1},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

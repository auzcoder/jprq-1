package main

import (
	"context"
	"embed"
	"os"
	"runtime"

	"github.com/azimjohn/jprq/cli"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Switch to CLI if arguments are supplied
	if len(os.Args) > 1 {
		cli.RunCLI()
		return
	}

	// Create an instance of the app structure
	app := NewApp()

	// Define application menu
	appMenu := menu.NewMenu()
	if runtime.GOOS == "darwin" {
		appMenu.Append(menu.AppMenu())
		appMenu.Append(menu.EditMenu())
	}

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Close Window", keys.CmdOrCtrl("w"), func(_ *menu.CallbackData) {
		wailsRuntime.WindowHide(app.ctx)
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit SpeedTunnel", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		app.Quit()
	})

	// Create application with options
	err := wails.Run(&options.App{
		Title:             "SpeedTunnel",
		Width:             800,
		Height:            600,
		MinWidth:          800,
		MinHeight:         600,
		MaxWidth:          1000,
		MaxHeight:         800,
		DisableResize:     false,
		Fullscreen:        false,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Menu:             appMenu,
		BackgroundColour: &options.RGBA{R: 15, G: 20, B: 30, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose: func(ctx context.Context) bool {
			app.clientMu.Lock()
			isExiting := app.exiting
			app.clientMu.Unlock()
			if isExiting {
				return false // Allow exit
			}
			wailsRuntime.WindowHide(ctx)
			return true // Intercept close, hide instead
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "SpeedTunnel",
				Message: "v1.0.0\nDavlatov Abdulhafiz",
			},
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

//go:build wails

package main

import (
	"context"
	"embed"
	"log/slog"

	"gopad/internal/app"
	"gopad/internal/desktop"
	"gopad/internal/runner"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if runner.RunWorkerModeIfEnabled() {
		return
	}

	application := app.New()
	bridge := desktop.NewWailsBridge(application)
	desktop.NativeToolbarUpdater = updateToolbarRunState

	if err := wails.Run(&options.App{
		Title:      "GoPad",
		Width:      1280,
		Height:     800,
		MinWidth:   1024,
		MinHeight:  640,
		OnStartup:  bridge.Startup,
		OnShutdown: bridge.Shutdown,
		OnDomReady: func(_ context.Context) {
			setupNativeToolbar()
		},
		Bind: []interface{}{
			bridge,
		},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				FullSizeContent:            true,
				UseToolbar:                 true,
				HideToolbarSeparator:       true,
			},
			WebviewIsTransparent: false,
			WindowIsTranslucent:  true,
		},
	}); err != nil {
		slog.Error("wails runtime failed", "error", err)
	}
}

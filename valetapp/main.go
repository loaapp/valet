package main

import (
	"context"
	"embed"

	"github.com/loaapp/valet/pkg/client"
	"github.com/loaapp/valet/valetapp/internal/api"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	c := client.New()
	statusSvc := api.NewStatusService(c)
	routeSvc := api.NewRouteService(c)
	tldSvc := api.NewTLDService(c)
	dnsSvc := api.NewDNSService(c)
	settingsSvc := api.NewSettingsService(c)
	agentSvc := api.NewAgentService()
	metricsSvc := api.NewMetricsService(c)
	logsSvc := api.NewLogsService(c)

	err := wails.Run(&options.App{
		Title:  "Valet",
		Width:  900,
		Height: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 27, A: 1},
		OnStartup: func(ctx context.Context) {
			statusSvc.SetContext(ctx)
			routeSvc.SetContext(ctx)
			tldSvc.SetContext(ctx)
			dnsSvc.SetContext(ctx)
			settingsSvc.SetContext(ctx)
			agentSvc.SetContext(ctx)
			metricsSvc.SetContext(ctx)
			logsSvc.SetContext(ctx)
		},
		Bind: []interface{}{
			statusSvc,
			routeSvc,
			tldSvc,
			dnsSvc,
			settingsSvc,
			agentSvc,
			metricsSvc,
			logsSvc,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

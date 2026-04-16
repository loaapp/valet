package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/loaapp/valet/pkg/client"
	"github.com/loaapp/valet/valetapp/internal/api"
	"github.com/loaapp/valet/valetapp/internal/conversations"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// ensureDaemon checks if valetd is running, and if not, launches it from
// the same directory as the current executable (i.e. inside the app bundle).
func ensureDaemon() {
	client := &http.Client{Timeout: 1 * time.Second}
	if resp, err := client.Get("http://localhost:7800/api/v1/status"); err == nil {
		resp.Body.Close()
		return
	}

	// Find valetd next to our binary
	exe, err := os.Executable()
	if err != nil {
		log.Printf("ensureDaemon: could not find executable path: %v", err)
		return
	}
	valetdPath := filepath.Join(filepath.Dir(exe), "valetd")
	if _, err := os.Stat(valetdPath); err != nil {
		log.Printf("ensureDaemon: valetd not found at %s", valetdPath)
		return
	}

	// Launch detached
	cmd := exec.Command(valetdPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		log.Printf("ensureDaemon: failed to start valetd: %v", err)
		return
	}

	// Wait up to 3 seconds for it to become available
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if resp, err := client.Get("http://localhost:7800/api/v1/status"); err == nil {
			resp.Body.Close()
			log.Printf("ensureDaemon: valetd started successfully (pid %d)", cmd.Process.Pid)
			return
		}
	}
	log.Printf("ensureDaemon: valetd started but not responding after 3s")
}

func main() {
	c := client.New()

	convoStore, err := conversations.New()
	if err != nil {
		log.Fatalf("Failed to open conversation store: %v", err)
	}

	statusSvc := api.NewStatusService(c)
	routeSvc := api.NewRouteService(c)
	tldSvc := api.NewTLDService(c)
	dnsSvc := api.NewDNSService(c)
	settingsSvc := api.NewSettingsService(c)
	agentSvc := api.NewAgentService(convoStore)
	metricsSvc := api.NewMetricsService(c)
	logsSvc := api.NewLogsService(c)

	err = wails.Run(&options.App{
		Title:  "Valet",
		Width:  900,
		Height: 620,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 27, A: 1},
		OnStartup: func(ctx context.Context) {
			ensureDaemon()
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

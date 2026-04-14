package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/loaapp/valet/valetd/internal/daemon"
	"github.com/loaapp/valet/valetd/internal/db"
)

func main() {
	apiAddr := flag.String("api", ":7800", "API listen address")
	dnsAddr := flag.String("dns", ":53", "DNS listen address (empty to disable)")
	flag.Parse()

	// Setup logging to file
	setupLogging()

	d := daemon.New(daemon.Config{
		APIAddr: *apiAddr,
		DNSAddr: *dnsAddr,
	})

	if err := d.Start(); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	// Wait for shutdown signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	d.Stop()
}

func setupLogging() {
	dir, err := db.DataDir()
	if err != nil {
		return
	}
	os.MkdirAll(dir, 0o755)

	logPath := filepath.Join(dir, "valetd.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

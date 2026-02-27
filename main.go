package main

import (
	"embed"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ohcnetwork/care_scanner_bridge/internal/config"
	"github.com/ohcnetwork/care_scanner_bridge/internal/scanner"
	"github.com/ohcnetwork/care_scanner_bridge/internal/server"
	"github.com/ohcnetwork/care_scanner_bridge/internal/tray"
)

//go:embed assets/*
var assets embed.FS

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize logger
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Starting Care Scanner Bridge...")

	// Create scanner manager
	scannerManager := scanner.NewManager()

	// Create WebSocket server
	wsServer := server.NewWebSocketServer(cfg.Port, scannerManager)

	// Start WebSocket server in background
	go func() {
		if err := wsServer.Start(); err != nil {
			log.Fatalf("Failed to start WebSocket server: %v", err)
		}
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start system tray (blocks on macOS/Windows)
	go tray.Run(assets, wsServer, scannerManager, cfg)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down...")
	wsServer.Stop()
}

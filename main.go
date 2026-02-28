package main

import (
	"embed"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

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

	// Check if port is already in use and try to kill the process
	if isPortInUse(cfg.Port) {
		log.Printf("Port %d is already in use, attempting to free it...", cfg.Port)
		if err := killProcessOnPort(cfg.Port); err != nil {
			log.Printf("Warning: Could not kill existing process: %v", err)
			showNotification("Care Scanner Bridge", fmt.Sprintf("Port %d is already in use. Please close the other application.", cfg.Port))
			os.Exit(1)
		}
		// Wait a moment for the port to be released
		time.Sleep(500 * time.Millisecond)
	}

	// Create scanner manager
	scannerManager := scanner.NewManager()

	// Create WebSocket server
	wsServer := server.NewWebSocketServer(cfg.Port, scannerManager)

	// Start WebSocket server in background
	serverStarted := make(chan bool, 1)
	go func() {
		// Small delay to let tray initialize first
		time.Sleep(100 * time.Millisecond)
		if err := wsServer.Start(); err != nil {
			log.Printf("Failed to start WebSocket server: %v", err)
			showNotification("Care Scanner Bridge", fmt.Sprintf("Failed to start server: %v", err))
			serverStarted <- false
			return
		}
		serverStarted <- true
	}()

	// Show startup notification
	go func() {
		time.Sleep(500 * time.Millisecond)
		showNotification("Care Scanner Bridge", fmt.Sprintf("Running on port %d. Look for the icon in your menu bar.", cfg.Port))
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

// isPortInUse checks if a port is already in use
func isPortInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// killProcessOnPort attempts to kill the process using the specified port
func killProcessOnPort(port int) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin", "linux":
		// Use lsof to find and kill the process
		cmd = exec.Command("sh", "-c", fmt.Sprintf("lsof -ti:%d | xargs kill -9 2>/dev/null", port))
	case "windows":
		// Use netstat and taskkill on Windows
		cmd = exec.Command("cmd", "/C", fmt.Sprintf("for /f \"tokens=5\" %%a in ('netstat -aon ^| find \":%d\"') do taskkill /F /PID %%a", port))
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	return cmd.Run()
}

// showNotification displays a system notification
func showNotification(title, message string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
		cmd = exec.Command("osascript", "-e", script)
	case "linux":
		cmd = exec.Command("notify-send", title, message)
	case "windows":
		// PowerShell notification
		script := fmt.Sprintf(`
			[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
			$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
			$textNodes = $template.GetElementsByTagName("text")
			$textNodes.Item(0).AppendChild($template.CreateTextNode("%s")) | Out-Null
			$textNodes.Item(1).AppendChild($template.CreateTextNode("%s")) | Out-Null
			$toast = [Windows.UI.Notifications.ToastNotification]::new($template)
			[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("Care Scanner Bridge").Show($toast)
		`, title, message)
		cmd = exec.Command("powershell", "-Command", script)
	default:
		log.Printf("Notification: %s - %s", title, message)
		return
	}

	if err := cmd.Run(); err != nil {
		log.Printf("Failed to show notification: %v", err)
	}
}

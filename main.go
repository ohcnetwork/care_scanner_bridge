package main

import (
	"embed"
	"flag"
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

//go:embed test.html
var testHTML []byte

// Version info (set during build)
var version = "dev"

func main() {
	// Parse command line flags
	headless := flag.Bool("headless", false, "Run without system tray (server only)")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("Care Scanner Bridge version:", version)
		os.Exit(0)
	}

	// On macOS, systray requires the main goroutine to be locked to the main thread
	runtime.LockOSThread()

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
	wsServer := server.NewWebSocketServer(cfg.Port, scannerManager, testHTML)

	// Start WebSocket server in background
	go func() {
		// Small delay to let tray initialize first
		time.Sleep(200 * time.Millisecond)
		log.Printf("Starting WebSocket server on port %d...", cfg.Port)
		if err := wsServer.Start(); err != nil {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	// Show startup notification after a short delay
	go func() {
		time.Sleep(1 * time.Second)
		showNotification("Care Scanner Bridge", fmt.Sprintf("Running on port %d. Look for the icon in your menu bar.", cfg.Port))
	}()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Check if we should disable tray (for debugging or headless operation)
	// Can be set via command line flag (-headless) or environment variable (DISABLE_TRAY=1)
	disableTray := *headless || os.Getenv("DISABLE_TRAY") == "1"

	if disableTray {
		log.Println("System tray disabled, running in headless mode")
		showNotification("Care Scanner Bridge", fmt.Sprintf("Running in headless mode on port %d", cfg.Port))

		// Block on signal in headless mode
		<-sigChan
		log.Println("Shutting down...")
		wsServer.Stop()
		os.Exit(0)
	} else {
		// Handle quit from tray
		go func() {
			<-sigChan
			log.Println("Shutting down...")
			wsServer.Stop()
			os.Exit(0)
		}()

		// Start system tray (this blocks on macOS - MUST be called from main thread)
		log.Println("Starting system tray...")
		tray.Run(assets, wsServer, scannerManager, cfg)
	}
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

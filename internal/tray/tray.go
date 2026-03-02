package tray

import (
	"embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/ohcnetwork/care_scanner_bridge/internal/config"
	"github.com/ohcnetwork/care_scanner_bridge/internal/scanner"
	"github.com/ohcnetwork/care_scanner_bridge/internal/server"
)

var (
	assets         embed.FS
	wsServer       *server.WebSocketServer
	scannerManager *scanner.Manager
	cfg            *config.Config

	mStatus       *systray.MenuItem
	mPairedDevice *systray.MenuItem
	mConnect      *systray.MenuItem
	mDisconnect   *systray.MenuItem
	mForget       *systray.MenuItem
	mPortsMenu    *systray.MenuItem
	mStartAtLogin *systray.MenuItem
	portItems     []*systray.MenuItem

	stopReconnect chan struct{}
)

// Run starts the system tray application
func Run(a embed.FS, ws *server.WebSocketServer, sm *scanner.Manager, c *config.Config) {
	assets = a
	wsServer = ws
	scannerManager = sm
	cfg = c

	systray.Run(onReady, onExit)
}

func onReady() {
	// Set icon
	iconData, err := assets.ReadFile("assets/icon.png")
	if err != nil {
		log.Printf("Failed to load icon: %v", err)
	} else {
		systray.SetIcon(iconData)
	}

	// On macOS, don't show title in menu bar (icon only)
	// On other platforms, show the title
	if runtime.GOOS != "darwin" {
		systray.SetTitle("Care Scanner")
	}
	systray.SetTooltip("Care Scanner Bridge - Serial Barcode Scanner")

	// Status (non-clickable)
	mStatus = systray.AddMenuItem("Status: Disconnected", "Current connection status")
	mStatus.Disable()

	// Paired device info
	mPairedDevice = systray.AddMenuItem("No paired device", "Currently paired scanner")
	mPairedDevice.Disable()

	systray.AddSeparator()

	// Ports submenu
	mPortsMenu = systray.AddMenuItem("Select Port", "Available serial ports")

	systray.AddSeparator()

	// Connect/Disconnect/Forget
	mConnect = systray.AddMenuItem("Connect", "Connect to paired scanner")
	mDisconnect = systray.AddMenuItem("Disconnect", "Disconnect from scanner")
	mDisconnect.Disable()
	mForget = systray.AddMenuItem("Forget Device", "Remove paired device")
	if cfg.LastDevice == "" {
		mForget.Disable()
	}

	systray.AddSeparator()

	// Server info
	mServerInfo := systray.AddMenuItem(fmt.Sprintf("Server: localhost:%d", wsServer.GetPort()), "WebSocket server address")
	mServerInfo.Disable()

	// Open browser
	mOpenBrowser := systray.AddMenuItem("Test Connection", "Open test page in browser")

	systray.AddSeparator()

	// Refresh ports
	mRefresh := systray.AddMenuItem("Refresh Ports", "Scan for available ports")

	systray.AddSeparator()

	// Start at Login (macOS only)
	if runtime.GOOS == "darwin" {
		mStartAtLogin = systray.AddMenuItem("Start at Login", "Launch automatically when you log in")
		if isStartAtLoginEnabled() {
			mStartAtLogin.Check()
		}
		systray.AddSeparator()
	}

	// Quit
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	// Initialize display after all menu items are created
	updatePairedDeviceDisplay()
	refreshPorts()

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mConnect.ClickedCh:
				handleConnect()
			case <-mDisconnect.ClickedCh:
				handleDisconnect()
			case <-mForget.ClickedCh:
				handleForgetDevice()
			case <-mRefresh.ClickedCh:
				refreshPorts()
			case <-mOpenBrowser.ClickedCh:
				openBrowser(fmt.Sprintf("http://localhost:%d", wsServer.GetPort()))
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	// Handle Start at Login clicks (separate goroutine for macOS)
	if runtime.GOOS == "darwin" && mStartAtLogin != nil {
		go func() {
			for range mStartAtLogin.ClickedCh {
				toggleStartAtLogin()
			}
		}()
	}

	// Auto-connect to paired device if configured
	if cfg.AutoConnect && cfg.LastDevice != "" {
		go autoConnectWithRetry()
	}
}

func onExit() {
	// Stop reconnection attempts
	if stopReconnect != nil {
		close(stopReconnect)
	}
	scannerManager.Disconnect()
	log.Println("System tray exiting")
}

// autoConnectWithRetry attempts to connect to the paired device with retries
func autoConnectWithRetry() {
	stopReconnect = make(chan struct{})
	maxRetries := 5
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		select {
		case <-stopReconnect:
			return
		default:
		}

		if cfg.LastDevice == "" {
			return
		}

		log.Printf("Auto-connect attempt %d/%d to %s", i+1, maxRetries, cfg.LastDevice)

		if err := scannerManager.Connect(cfg.LastDevice, cfg.BaudRate); err != nil {
			log.Printf("Auto-connect failed: %v", err)

			if i < maxRetries-1 {
				select {
				case <-stopReconnect:
					return
				case <-time.After(retryDelay):
					// Refresh ports in case device was just plugged in
					refreshPorts()
				}
			}
		} else {
			log.Printf("Auto-connected to paired device: %s", cfg.LastDevice)
			updateStatus()
			return
		}
	}

	log.Printf("Failed to auto-connect after %d attempts", maxRetries)
}

// formatDeviceName extracts a friendly device name from a full path
// e.g., "/dev/cu.usbmodem00000000050C1" -> "usbmodem00000000050C1"
func formatDeviceName(path string) string {
	name := path
	// Remove common prefixes to show just the device name
	prefixes := []string{"/dev/cu.", "/dev/tty.", "/dev/", "COM"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
			break
		}
	}
	return name
}

// updatePairedDeviceDisplay updates the paired device menu item
func updatePairedDeviceDisplay() {
	if mPairedDevice == nil {
		return // Menu not initialized yet
	}

	if cfg.LastDevice != "" {
		// Show friendly device name
		deviceName := formatDeviceName(cfg.LastDevice)
		mPairedDevice.SetTitle(fmt.Sprintf("Paired: %s", deviceName))
		if mForget != nil {
			mForget.Enable()
		}
	} else {
		mPairedDevice.SetTitle("No paired device")
		if mForget != nil {
			mForget.Disable()
		}
	}
}

// handleForgetDevice removes the paired device
func handleForgetDevice() {
	cfg.LastDevice = ""
	cfg.Save()

	// Disconnect if currently connected
	if scannerManager.IsConnected() {
		scannerManager.Disconnect()
	}

	updatePairedDeviceDisplay()
	updateStatus()
	refreshPorts()
	log.Println("Paired device forgotten")
}

func refreshPorts() {
	ports, err := scannerManager.ListPorts()
	if err != nil {
		log.Printf("Failed to list ports: %v", err)
		return
	}

	// Clear existing port items - hide them
	for _, item := range portItems {
		item.Hide()
	}
	portItems = portItems[:0] // Reset slice but keep capacity

	if len(ports) == 0 {
		mPortsMenu.SetTitle("No ports found")
		return
	}

	mPortsMenu.SetTitle(fmt.Sprintf("Select Port (%d found)", len(ports)))

	// Add port items
	for _, port := range ports {
		displayName := formatDeviceName(port.Path)
		if port.Description != "" {
			displayName = fmt.Sprintf("%s - %s", displayName, port.Description)
		}

		// Show indicators for paired and connected status
		isPaired := port.Path == cfg.LastDevice
		if port.IsConnected {
			displayName = "✓ " + displayName + " (connected)"
		} else if isPaired {
			displayName = "🔗 " + displayName + " (paired)"
		}

		item := mPortsMenu.AddSubMenuItem(displayName, fmt.Sprintf("Connect to %s", formatDeviceName(port.Path)))
		portItems = append(portItems, item)

		// Handle port selection in a separate goroutine
		portPath := port.Path // Capture the value
		go func(menuItem *systray.MenuItem, path string) {
			for range menuItem.ClickedCh {
				log.Printf("Port selected and paired: %s", path)

				// Save as paired device
				cfg.LastDevice = path
				cfg.Save()
				updatePairedDeviceDisplay()

				// Connect to the device
				if err := scannerManager.Connect(path, cfg.BaudRate); err != nil {
					log.Printf("Failed to connect to %s: %v", path, err)
				} else {
					log.Printf("Successfully connected and paired to %s", path)
					updateStatus()
				}
				refreshPorts() // Refresh to show updated status
			}
		}(item, portPath)
	}
}

func handleConnect() {
	if cfg.LastDevice == "" {
		log.Println("No port selected")
		return
	}

	if err := scannerManager.Connect(cfg.LastDevice, cfg.BaudRate); err != nil {
		log.Printf("Connect failed: %v", err)
		return
	}

	updateStatus()
}

func handleDisconnect() {
	if err := scannerManager.Disconnect(); err != nil {
		log.Printf("Disconnect failed: %v", err)
		return
	}

	updateStatus()
}

func updateStatus() {
	if scannerManager.IsConnected() {
		mStatus.SetTitle(fmt.Sprintf("Status: Connected (%s)", scannerManager.GetCurrentPort()))
		mConnect.Disable()
		mDisconnect.Enable()
	} else {
		mStatus.SetTitle("Status: Disconnected")
		mConnect.Enable()
		mDisconnect.Disable()
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}

	if cmd != nil {
		cmd.Start()
	}
}

// Start at Login functions for macOS
func getLoginItemPlistPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Library", "LaunchAgents", "com.ohcnetwork.care-scanner-bridge.plist")
}

func isStartAtLoginEnabled() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	plistPath := getLoginItemPlistPath()
	_, err := os.Stat(plistPath)
	return err == nil
}

func toggleStartAtLogin() {
	if runtime.GOOS != "darwin" {
		return
	}

	if isStartAtLoginEnabled() {
		// Disable - remove the plist
		plistPath := getLoginItemPlistPath()
		if err := os.Remove(plistPath); err != nil {
			log.Printf("Failed to remove login item: %v", err)
		} else {
			log.Println("Disabled start at login")
			mStartAtLogin.Uncheck()
		}
	} else {
		// Enable - create the plist
		if err := createLoginItemPlist(); err != nil {
			log.Printf("Failed to create login item: %v", err)
		} else {
			log.Println("Enabled start at login")
			mStartAtLogin.Check()
		}
	}
}

func createLoginItemPlist() error {
	// Get the path to the running executable or app bundle
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	// If running from an app bundle, use the bundle path
	// The executable is at AppName.app/Contents/MacOS/executable
	appPath := execPath
	if idx := strings.Index(execPath, ".app/Contents/MacOS"); idx != -1 {
		appPath = execPath[:idx+4] // Include ".app"
	}

	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.ohcnetwork.care-scanner-bridge</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
    <key>StandardErrorPath</key>
    <string>/tmp/care-scanner-bridge.err</string>
    <key>StandardOutPath</key>
    <string>/tmp/care-scanner-bridge.out</string>
</dict>
</plist>
`, appPath)

	plistPath := getLoginItemPlistPath()

	// Ensure LaunchAgents directory exists
	launchAgentsDir := filepath.Dir(plistPath)
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(plistPath, []byte(plistContent), 0644)
}

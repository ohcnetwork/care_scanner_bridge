package tray

import (
	"embed"
	"fmt"
	"log"
	"os/exec"
	"runtime"

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

	mStatus      *systray.MenuItem
	mConnect     *systray.MenuItem
	mDisconnect  *systray.MenuItem
	mPortsMenu   *systray.MenuItem
	portItems    []*systray.MenuItem
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

	systray.SetTitle("Care Scanner")
	systray.SetTooltip("Care Scanner Bridge - Serial Barcode Scanner")

	// Status (non-clickable)
	mStatus = systray.AddMenuItem("Status: Disconnected", "Current connection status")
	mStatus.Disable()

	systray.AddSeparator()

	// Ports submenu
	mPortsMenu = systray.AddMenuItem("Select Port", "Available serial ports")
	refreshPorts()

	systray.AddSeparator()

	// Connect/Disconnect
	mConnect = systray.AddMenuItem("Connect", "Connect to scanner")
	mDisconnect = systray.AddMenuItem("Disconnect", "Disconnect from scanner")
	mDisconnect.Disable()

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

	// Quit
	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mConnect.ClickedCh:
				handleConnect()
			case <-mDisconnect.ClickedCh:
				handleDisconnect()
			case <-mRefresh.ClickedCh:
				refreshPorts()
			case <-mOpenBrowser.ClickedCh:
				openBrowser(fmt.Sprintf("http://localhost:%d/health", wsServer.GetPort()))
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	// Auto-connect if configured
	if cfg.AutoConnect && cfg.LastDevice != "" {
		go func() {
			if err := scannerManager.Connect(cfg.LastDevice, cfg.BaudRate); err != nil {
				log.Printf("Auto-connect failed: %v", err)
			} else {
				updateStatus()
			}
		}()
	}
}

func onExit() {
	scannerManager.Disconnect()
	log.Println("System tray exiting")
}

func refreshPorts() {
	ports, err := scannerManager.ListPorts()
	if err != nil {
		log.Printf("Failed to list ports: %v", err)
		return
	}

	// Clear existing port items
	for _, item := range portItems {
		item.Hide()
	}
	portItems = nil

	if len(ports) == 0 {
		mPortsMenu.SetTitle("No ports found")
		return
	}

	mPortsMenu.SetTitle(fmt.Sprintf("Select Port (%d found)", len(ports)))

	// Add port items
	for _, port := range ports {
		item := mPortsMenu.AddSubMenuItem(port.Path, port.Description)
		portItems = append(portItems, item)

		// Handle port selection
		go func(p scanner.PortInfo, menuItem *systray.MenuItem) {
			for range menuItem.ClickedCh {
				cfg.LastDevice = p.Path
				cfg.Save()
				if err := scannerManager.Connect(p.Path, cfg.BaudRate); err != nil {
					log.Printf("Failed to connect to %s: %v", p.Path, err)
				} else {
					updateStatus()
				}
			}
		}(port, item)
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

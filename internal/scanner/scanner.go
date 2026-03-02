package scanner

import (
	"log"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// ScanEvent represents a barcode scan event
type ScanEvent struct {
	Barcode   string    `json:"barcode"`
	Port      string    `json:"port"`
	Timestamp time.Time `json:"timestamp"`
}

// PortInfo represents information about a serial port
type PortInfo struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	IsConnected bool   `json:"isConnected"`
}

// Manager manages serial port connections and barcode scanning
type Manager struct {
	mu            sync.RWMutex
	port          serial.Port
	currentPort   string
	baudRate      int
	isConnected   bool
	scanListeners []chan ScanEvent
	stopChan      chan struct{}
}

// NewManager creates a new scanner manager
func NewManager() *Manager {
	return &Manager{
		baudRate:      9600,
		scanListeners: make([]chan ScanEvent, 0),
	}
}

// ListPorts returns available serial ports
func (m *Manager) ListPorts() ([]PortInfo, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}

	result := make([]PortInfo, 0, len(ports))
	for _, p := range ports {
		// Filter to show likely scanner ports
		if isLikelyScannerPort(p) {
			result = append(result, PortInfo{
				Path:        p,
				Description: getPortDescription(p),
				IsConnected: m.currentPort == p && m.isConnected,
			})
		}
	}

	return result, nil
}

// Connect connects to a serial port
func (m *Manager) Connect(portPath string, baudRate int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Disconnect existing connection
	if m.port != nil {
		m.port.Close()
		if m.stopChan != nil {
			close(m.stopChan)
		}
	}

	mode := &serial.Mode{
		BaudRate: baudRate,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portPath, mode)
	if err != nil {
		return err
	}

	m.port = port
	m.currentPort = portPath
	m.baudRate = baudRate
	m.isConnected = true
	m.stopChan = make(chan struct{})

	// Start reading in background
	go m.readLoop()

	log.Printf("Connected to scanner on %s at %d baud", portPath, baudRate)
	return nil
}

// Disconnect disconnects from the current port
func (m *Manager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.port != nil {
		// Mark as disconnected first
		m.isConnected = false
		
		// Close the port (this will cause Read to fail)
		err := m.port.Close()
		m.port = nil
		m.currentPort = ""
		
		// Then stop the read loop
		if m.stopChan != nil {
			close(m.stopChan)
			m.stopChan = nil
		}
		
		log.Println("Disconnected from scanner")
		return err
	}
	return nil
}

// IsConnected returns the connection status
func (m *Manager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isConnected
}

// GetCurrentPort returns the currently connected port
func (m *Manager) GetCurrentPort() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentPort
}

// SimulateScan simulates a barcode scan event (for testing)
func (m *Manager) SimulateScan(barcode string) {
	event := ScanEvent{
		Barcode:   barcode,
		Port:      "test",
		Timestamp: time.Now(),
	}
	log.Printf("Simulating scan: %s", barcode)
	m.notifyListeners(event)
}

// Subscribe adds a listener for scan events
func (m *Manager) Subscribe() chan ScanEvent {
	m.mu.Lock()
	defer m.mu.Unlock()

	ch := make(chan ScanEvent, 10)
	m.scanListeners = append(m.scanListeners, ch)
	return ch
}

// Unsubscribe removes a listener
func (m *Manager) Unsubscribe(ch chan ScanEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, listener := range m.scanListeners {
		if listener == ch {
			m.scanListeners = append(m.scanListeners[:i], m.scanListeners[i+1:]...)
			close(ch)
			break
		}
	}
}

func (m *Manager) readLoop() {
	log.Printf("Starting read loop for port %s", m.currentPort)

	buf := make([]byte, 1024)
	var dataBuffer strings.Builder

	for {
		select {
		case <-m.stopChan:
			log.Printf("Read loop stopped")
			return
		default:
			// Check if port is still valid
			m.mu.RLock()
			port := m.port
			m.mu.RUnlock()

			if port == nil {
				log.Printf("Port closed, stopping read loop")
				return
			}

			// Set read timeout
			port.SetReadTimeout(200 * time.Millisecond)

			n, err := port.Read(buf)
			if err != nil {
				// Check if we're disconnected
				m.mu.RLock()
				connected := m.isConnected
				m.mu.RUnlock()
				if !connected {
					log.Printf("Disconnected, stopping read loop")
					return
				}
				// Timeout or temporary error, continue
				continue
			}

			if n > 0 {
				chunk := string(buf[:n])
				log.Printf("Raw data received (%d bytes): %q", n, chunk)
				dataBuffer.WriteString(chunk)

				// Check if we have a complete scan (ends with \r, \n, or \r\n)
				data := dataBuffer.String()
				if strings.ContainsAny(data, "\r\n") {
					// Split by common line endings
					lines := strings.FieldsFunc(data, func(r rune) bool {
						return r == '\r' || r == '\n'
					})

					for _, line := range lines {
						barcode := strings.TrimSpace(line)
						if barcode == "" {
							continue
						}

						event := ScanEvent{
							Barcode:   barcode,
							Port:      m.currentPort,
							Timestamp: time.Now(),
						}

						log.Printf("Scanned: %s", barcode)
						m.notifyListeners(event)
					}

					// Clear buffer after processing
					dataBuffer.Reset()
				}
			}
		}
	}
}

func (m *Manager) notifyListeners(event ScanEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ch := range m.scanListeners {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

func isLikelyScannerPort(port string) bool {
	// On macOS, USB serial devices typically appear as /dev/cu.usbserial-* or /dev/cu.usbmodem-*
	// Also /dev/tty.* but we prefer /dev/cu.* (cu = calling unit, tty = terminal)
	// Skip /dev/tty.* to avoid duplicates on macOS
	if strings.HasPrefix(port, "/dev/tty.") {
		return false
	}

	// On Linux, they appear as /dev/ttyUSB* or /dev/ttyACM*
	// On Windows, they appear as COM*
	lowerPort := strings.ToLower(port)
	return strings.Contains(lowerPort, "usb") ||
		strings.Contains(lowerPort, "acm") ||
		strings.Contains(lowerPort, "com") ||
		strings.Contains(lowerPort, "serial") ||
		strings.HasPrefix(lowerPort, "/dev/cu.")
}

func getPortDescription(port string) string {
	if strings.Contains(port, "usbserial") {
		return "USB Serial Device"
	}
	if strings.Contains(port, "usbmodem") {
		return "USB Modem Device"
	}
	return "Serial Port"
}

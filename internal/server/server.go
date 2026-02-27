package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/ohcnetwork/care_scanner_bridge/internal/scanner"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow connections from any origin (local app)
		return true
	},
}

// Message types
const (
	MsgTypeConnect    = "connect"
	MsgTypeDisconnect = "disconnect"
	MsgTypeListPorts  = "list_ports"
	MsgTypeScan       = "scan"
	MsgTypeStatus     = "status"
	MsgTypeError      = "error"
	MsgTypePorts      = "ports"
	MsgTypePing       = "ping"
	MsgTypePong       = "pong"
)

// Message represents a WebSocket message
type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
}

// ConnectPayload represents the payload for connect messages
type ConnectPayload struct {
	Port     string `json:"port"`
	BaudRate int    `json:"baudRate,omitempty"`
}

// StatusPayload represents the status response
type StatusPayload struct {
	Connected   bool   `json:"connected"`
	CurrentPort string `json:"currentPort,omitempty"`
}

// WebSocketServer handles WebSocket connections
type WebSocketServer struct {
	port           int
	scanner        *scanner.Manager
	clients        map[*websocket.Conn]bool
	mu             sync.RWMutex
	server         *http.Server
	clientCount    int
}

// NewWebSocketServer creates a new WebSocket server
func NewWebSocketServer(port int, scannerMgr *scanner.Manager) *WebSocketServer {
	return &WebSocketServer{
		port:    port,
		scanner: scannerMgr,
		clients: make(map[*websocket.Conn]bool),
	}
}

// Start starts the WebSocket server
func (s *WebSocketServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	log.Printf("WebSocket server starting on port %d", s.port)
	return s.server.ListenAndServe()
}

// Stop stops the WebSocket server
func (s *WebSocketServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// GetPort returns the server port
func (s *WebSocketServer) GetPort() int {
	return s.port
}

// GetClientCount returns the number of connected clients
func (s *WebSocketServer) GetClientCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.clients)
}

func (s *WebSocketServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"connected": s.scanner.IsConnected(),
		"port":      s.scanner.GetCurrentPort(),
	})
}

func (s *WebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Register client
	s.mu.Lock()
	s.clients[conn] = true
	s.clientCount++
	s.mu.Unlock()

	log.Printf("Client connected (total: %d)", s.GetClientCount())

	// Subscribe to scan events
	scanChan := s.scanner.Subscribe()
	defer s.scanner.Unsubscribe(scanChan)

	// Send scan events to this client
	go func() {
		for event := range scanChan {
			s.sendMessage(conn, Message{
				Type: MsgTypeScan,
				Payload: map[string]interface{}{
					"barcode":   event.Barcode,
					"port":      event.Port,
					"timestamp": event.Timestamp,
				},
			})
		}
	}()

	// Send initial status
	s.sendStatus(conn)

	// Handle incoming messages
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Read error: %v", err)
			break
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("JSON unmarshal error: %v", err)
			continue
		}

		s.handleMessage(conn, msg)
	}

	// Unregister client
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()

	log.Printf("Client disconnected (total: %d)", s.GetClientCount())
}

func (s *WebSocketServer) handleMessage(conn *websocket.Conn, msg Message) {
	switch msg.Type {
	case MsgTypeConnect:
		s.handleConnect(conn, msg)
	case MsgTypeDisconnect:
		s.handleDisconnect(conn)
	case MsgTypeListPorts:
		s.handleListPorts(conn)
	case MsgTypeStatus:
		s.sendStatus(conn)
	case MsgTypePing:
		s.sendMessage(conn, Message{Type: MsgTypePong})
	}
}

func (s *WebSocketServer) handleConnect(conn *websocket.Conn, msg Message) {
	payloadBytes, _ := json.Marshal(msg.Payload)
	var payload ConnectPayload
	json.Unmarshal(payloadBytes, &payload)

	baudRate := payload.BaudRate
	if baudRate == 0 {
		baudRate = 9600
	}

	if err := s.scanner.Connect(payload.Port, baudRate); err != nil {
		s.sendMessage(conn, Message{
			Type:    MsgTypeError,
			Payload: map[string]string{"message": err.Error()},
		})
		return
	}

	// Broadcast status to all clients
	s.broadcastStatus()
}

func (s *WebSocketServer) handleDisconnect(conn *websocket.Conn) {
	if err := s.scanner.Disconnect(); err != nil {
		s.sendMessage(conn, Message{
			Type:    MsgTypeError,
			Payload: map[string]string{"message": err.Error()},
		})
		return
	}

	s.broadcastStatus()
}

func (s *WebSocketServer) handleListPorts(conn *websocket.Conn) {
	ports, err := s.scanner.ListPorts()
	if err != nil {
		s.sendMessage(conn, Message{
			Type:    MsgTypeError,
			Payload: map[string]string{"message": err.Error()},
		})
		return
	}

	s.sendMessage(conn, Message{
		Type:    MsgTypePorts,
		Payload: ports,
	})
}

func (s *WebSocketServer) sendStatus(conn *websocket.Conn) {
	s.sendMessage(conn, Message{
		Type: MsgTypeStatus,
		Payload: StatusPayload{
			Connected:   s.scanner.IsConnected(),
			CurrentPort: s.scanner.GetCurrentPort(),
		},
	})
}

func (s *WebSocketServer) broadcastStatus() {
	msg := Message{
		Type: MsgTypeStatus,
		Payload: StatusPayload{
			Connected:   s.scanner.IsConnected(),
			CurrentPort: s.scanner.GetCurrentPort(),
		},
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for client := range s.clients {
		s.sendMessage(client, msg)
	}
}

func (s *WebSocketServer) sendMessage(conn *websocket.Conn, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("JSON marshal error: %v", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Write error: %v", err)
	}
}

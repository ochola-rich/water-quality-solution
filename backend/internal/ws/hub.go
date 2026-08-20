package ws

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
)

// EventMessage represents a WebSocket notification event
type EventMessage struct {
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Hub manages active WebSocket client connections and message broadcasting
type Hub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

// NewHub creates and returns an initialized WebSocket Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Run starts the Hub event loop in a background goroutine
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()
			log.Printf("[WS Hub] Client connected: %s (Total clients: %d)", conn.RemoteAddr(), len(h.clients))

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
				log.Printf("[WS Hub] Client disconnected: %s (Total clients: %d)", conn.RemoteAddr(), len(h.clients))
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.clients {
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					log.Printf("[WS Hub] Write error for client %s: %v", conn.RemoteAddr(), err)
					conn.Close()
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, conn)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast serializes an event and payload and sends it to all connected clients
func (h *Hub) Broadcast(event string, data interface{}) {
	msg := EventMessage{
		Event:     event,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[WS Hub] JSON marshal error in broadcast: %v", err)
		return
	}

	select {
	case h.broadcast <- payload:
	default:
		log.Printf("[WS Hub] Broadcast channel full, dropping event: %s", event)
	}
}

// Register registers a new client connection
func (h *Hub) Register(conn *websocket.Conn) {
	h.register <- conn
}

// Unregister unregisters an existing client connection
func (h *Hub) Unregister(conn *websocket.Conn) {
	h.unregister <- conn
}

// ClientCount returns the current number of active connections
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

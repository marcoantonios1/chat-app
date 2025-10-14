package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		// for local/dev only — tighten in production
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

func HandleMessage(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "failed to upgrade websocket", http.StatusBadRequest)
		return
	}
	// register
	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		conn.Close()
	}()

	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			// client closed or read error
			return
		}

		// broadcast to all connected clients
		clientsMu.Lock()
		for c := range clients {
			// ignore write errors for simplicity
			_ = c.WriteMessage(mt, msg)
		}
		clientsMu.Unlock()

		log.Printf("📩 Received message: %s", string(msg))
	}
}

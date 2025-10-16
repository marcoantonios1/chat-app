package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		// for local/dev only — tighten in production
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func HandleMessage(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}

	id := r.URL.Query().Get("id") // optional client identifier
	client := &Client{
		ID:   id,
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	// register client with hub
	hub.register <- client

	// writer goroutine: sends messages from client.Send to websocket
	go func(c *Client) {
		defer c.Conn.Close()
		for msg := range c.Send {
			_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}(client)

	// reader: receive messages from this socket and route to hub
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// try to inspect JSON recipient field to route targeted messages
		var payload struct {
			Recipient string `json:"recipient"`
		}
		if err := json.Unmarshal(msg, &payload); err == nil && payload.Recipient != "" {
			hub.targeted <- targetedMessage{to: payload.Recipient, msg: msg}
		} else {
			hub.broadcast <- msg
		}
	}

	// cleanup on disconnect
	hub.unregister <- client
}

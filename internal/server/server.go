package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// time allowed to write a message to the peer.
	writeWait = 10 * time.Second
	// time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// maximum message size allowed from peer.
	maxMessageSize = 64 * 1024
)

type messagePayload struct {
	Type         string `json:"type,omitempty"`
	ID           string `json:"id"`
	Recipient    string `json:"recipient"`
	Body         string `json:"body,omitempty"`
	MsgID        string `json:"msg_id,omitempty"`
	PublicKey    string `json:"public_key,omitempty"`
	EncryptedKey string `json:"encrypted_key,omitempty"`
}

var (
	upgrader = websocket.Upgrader{
		// for local/dev only — tighten in production
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func HandleMessage(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id") // optional client identifier
	if id == "" {
		http.Error(w, "missing id query parameter", http.StatusBadRequest)
		return
	}
	if !IsRegistered(id) {
		http.Error(w, "id not registered", http.StatusForbidden)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusBadRequest)
		return
	}
	client := &Client{
		ID:   id,
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	// register client with hub
	hub.register <- client
	log.Printf("ws: client connected id=%q remote=%s", id, conn.RemoteAddr())

	// writer goroutine: sends messages from client.Send to websocket
	go func(c *Client) {
		defer c.Conn.Close()
		for msg := range c.Send {
			_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("ws: write error for id=%q: %v", c.ID, err)
				return
			}
		}
	}(client)

	// Configure read limits, initial deadline and pong handler
	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Pinger goroutine: sends periodic ping frames
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()
	go func(c *Client) {
		for range pingTicker.C {
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("ws: ping error for id=%q: %v", c.ID, err)
				// close connection to trigger cleanup
				_ = c.Conn.Close()
				return
			}
		}
	}(client)

	// Simple token-bucket rate limiter (5 messages burst, refill ~5/sec)
	rateTokens := make(chan struct{}, 5)
	for i := 0; i < 5; i++ {
		rateTokens <- struct{}{}
	}
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			select {
			case rateTokens <- struct{}{}:
			default:
			}
		}
	}()

	// reader: receive messages from this socket and route to hub
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("ws: read error/closed for id=%q: %v", id, err)
			break
		}

		if len(msg) > maxMessageSize {
			log.Printf("ws: dropping oversized message from id=%q len=%d", id, len(msg))
			continue
		}

		select {
		case <-rateTokens:
			// allowed
		default:
			// notify sender about rate limit (send via client.Send, non-blocking)
			er := messagePayload{Type: "error", Body: "rate limit exceeded"}
			if b, _ := json.Marshal(er); b != nil {
				select {
				case client.Send <- b:
				default:
				}
			}
			log.Printf("ws: rate limit hit for id=%q", id)
			continue
		}

		var payload messagePayload
		if err := json.Unmarshal(msg, &payload); err == nil && payload.Recipient != "" {
			if !IsRegistered(payload.Recipient) {
				er := messagePayload{Type: "error", Body: "recipient not found"}
				if b, _ := json.Marshal(er); b != nil {
					select {
					case client.Send <- b:
					default:
					}
				}
				log.Printf("ws: target not found id=%s from=%s", payload.Recipient, id)
				continue
			}
			hub.targeted <- targetedMessage{to: payload.Recipient, msg: msg, from: id}
			log.Printf("ws: got msg len=%d targeted to=%q from id=%q", len(msg), payload.Recipient, id)
		} else {
			hub.broadcast <- msg
			log.Printf("ws: got msg broadcast len=%d from id=%q", len(msg), id)
		}
	}

	// cleanup on disconnect
	hub.unregister <- client
	_ = conn.Close()
	log.Printf("ws: disconnected id=%q", id)
}

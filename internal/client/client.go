package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type messagePayload struct {
	Type      string `json:"type,omitempty"`
	ID        string `json:"id"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
	MsgID     string `json:"msg_id,omitempty"`
}

var printMu sync.Mutex

func printPrompt() {
	printMu.Lock()
	fmt.Print("> ") // prompt
	printMu.Unlock()
}

func printIncoming(msg string) {
	printMu.Lock()
	fmt.Print("\r")
	fmt.Println("📨", msg)
	fmt.Print("> ")
	printMu.Unlock()
}

func SendAndReceive(rawURL string, initialMsg string, id string, recipient string) error {
	var statusMu sync.Mutex
	statuses := make(map[string]string)
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	q := u.Query()
	q.Set("id", id)
	u.RawQuery = q.Encode()

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer conn.Close()

	// helper to send a body with embedded build id (hidden from user's view)
	sendPayload := func(body, typ, msgID, to string) error {
		payload := messagePayload{Type: typ, ID: id, Body: body, Recipient: to, MsgID: msgID}
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal error: %w", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
		return nil
	}

	sendBody := func(body, typ, msgID string) error {
		return sendPayload(body, typ, msgID, recipient)
	}

	// Send initial message if provided
	if initialMsg != "" {
		msgID := fmt.Sprintf("%d", time.Now().UnixNano())
		if err := sendBody(initialMsg, "msg", msgID); err != nil {
			return fmt.Errorf("initial write error: %w", err)
		}
		statusMu.Lock()
		statuses[msgID] = "sent"
		statusMu.Unlock()
		fmt.Println("📤 Sent:", initialMsg)
	}

	// 1️⃣ Read loop
	go func() {
		for {
			_, m, err := conn.ReadMessage()
			if err != nil {
				fmt.Println("❌ read error:", err)
				return
			}
			// try to decode JSON payload; if it fails, print raw
			var payload messagePayload
			if err := json.Unmarshal(m, &payload); err != nil {
				// not JSON — print raw message
				printIncoming(string(m))
				continue
			}

			// skip messages that originated from this client
			if payload.ID == id && payload.Type != "ack" {
				continue
			}

			// if message is targeted to someone and it's not for this client, skip
			if payload.Recipient != "" && payload.Recipient != id {
				continue
			}

			switch payload.Type {
			case "ack":
				// update status of sent message
				if payload.MsgID != "" {
					statusMu.Lock()
					statuses[payload.MsgID] = payload.Body // e.g. "delivered" or "queued"
					statusMu.Unlock()
					fmt.Println("📫", payload.Body)
				} else {
					fmt.Println("📫", payload.Body)
				}
			default:
				sender := payload.ID
				printIncoming(sender + ": " + payload.Body)

				if payload.MsgID != "" {
					_ = sendPayload("read", "ack", payload.MsgID, payload.ID)
				} else {
					_ = sendPayload("read", "ack", "", payload.ID)
				}
			}
		}
	}()

	// 2️⃣ Write loop — read user input from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for {
		printPrompt()
		if !scanner.Scan() {
			break
		}
		text := scanner.Text()
		if text == "" {
			continue
		}

		msgID := fmt.Sprintf("%d", time.Now().UnixNano())
		if err := sendBody(text, "msg", msgID); err != nil {
			fmt.Println("❌ write error:", err)
			break
		}
		statusMu.Lock()
		statuses[msgID] = "sent"
		statusMu.Unlock()
		fmt.Println("✓ Sent ")
	}

	return scanner.Err()
}

func Listen(url string) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer conn.Close()

	fmt.Println("📡 Connected. Waiting for messages...")

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read error: %w", err)
		}
		fmt.Println("📨 New message:", string(msg))
	}
}

var ErrIDTaken = fmt.Errorf("id already taken")

func Register(registerURL, id string) error {
	body := map[string]string{"id": id}
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}
	resp, err := http.Post(registerURL, "application/json", bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("post error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusConflict {
		return ErrIDTaken
	}
	return fmt.Errorf("register failed: %s", resp.Status)
}

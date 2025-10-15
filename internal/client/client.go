package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
)

type messagePayload struct {
	ID        string `json:"id"`
	Recipient string `json:"recipient"`
	Body      string `json:"body"`
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

func SendAndReceive(url string, initialMsg string, id string, recipient string) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer conn.Close()

	// helper to send a body with embedded build id (hidden from user's view)
	sendBody := func(body string) error {
		payload := messagePayload{ID: id, Body: body, Recipient: recipient}
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal error: %w", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, b); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
		return nil
	}

	// Send initial message if provided
	if initialMsg != "" {
		if err := sendBody(initialMsg); err != nil {
			return fmt.Errorf("initial write error: %w", err)
		}
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
				fmt.Println("📨", string(m))
				printIncoming(string(m))
				continue
			}

			// skip messages that originated from this client
			if payload.ID == id {
				continue
			}

			// if message is targeted to someone and it's not for this client, skip
			if payload.Recipient != "" && payload.Recipient != id {
				continue
			}
			printIncoming(payload.ID + ": " + payload.Body)
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

		if err := sendBody(text); err != nil {
			fmt.Println("❌ write error:", err)
			break
		}
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

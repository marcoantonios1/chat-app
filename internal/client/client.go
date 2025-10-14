package client

import (
	"bufio"
	"fmt"
	"os"

	"github.com/gorilla/websocket"
)

func SendAndReceive(url string, initialMsg string) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer conn.Close()

	// Send initial message if provided
	if initialMsg != "" {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(initialMsg)); err != nil {
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
			fmt.Println("📨", string(m))
		}
	}()

	// 2️⃣ Write loop — read user input from stdin
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ") // prompt
		if !scanner.Scan() {
			break
		}
		text := scanner.Text()
		if text == "" {
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, []byte(text)); err != nil {
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

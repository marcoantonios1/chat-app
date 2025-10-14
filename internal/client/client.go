package client

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

func SendOnce(url, msg string) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}
	defer conn.Close()

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		return fmt.Errorf("write error: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, reply, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read error: %w", err)
	}

	fmt.Println(string(reply))
	return nil
}

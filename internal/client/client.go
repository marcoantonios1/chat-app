package client

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // ⚠️ Allow all for now — restrict later
	},
}

func HandleChat(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	fmt.Println("New client connected!")

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Client disconnected:", err)
			return
		}

		fmt.Println("Message received:", string(msg))
		conn.WriteMessage(websocket.TextMessage, msg) // echo for now
	}
}
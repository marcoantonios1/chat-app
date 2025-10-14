package server

import (
	"fmt"
	"net/http"
)

func HandleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read message from request body
	buf := make([]byte, r.ContentLength)
	_, err := r.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	msg := string(buf)
	fmt.Println("📩 Received message:", msg)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Message received"))
}

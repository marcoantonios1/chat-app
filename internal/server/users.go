package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type registerRequest struct {
	ID string `json:"id"`
}

var (
	users   = make(map[string]bool)
	usersMu sync.Mutex
)

// HandleRegister accepts POST {"id":"..."} and registers the id if available.
// Returns 201 on success, 409 if id already taken.
func HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}

	usersMu.Lock()
	defer usersMu.Unlock()
	if users[req.ID] {
		http.Error(w, "id already taken", http.StatusConflict)
		return
	}
	users[req.ID] = true

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("ok"))
	fmt.Println("🆕 Registered user:", req.ID)
}

// IsRegistered returns whether an id is present (helpful for server logic).
func IsRegistered(id string) bool {
	usersMu.Lock()
	defer usersMu.Unlock()
	return users[id]
}

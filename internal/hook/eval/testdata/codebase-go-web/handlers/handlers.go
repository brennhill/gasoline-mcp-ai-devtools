package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/example/web/db"
)

// HandleUser serves user data.
func HandleUser(w http.ResponseWriter, r *http.Request) {
	users := db.GetUsers()
	_ = json.NewEncoder(w).Encode(users)
}

// HandleHealth serves health check.
func HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

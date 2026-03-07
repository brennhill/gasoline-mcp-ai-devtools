package routes

import (
	"net/http"

	"github.com/example/web/handlers"
)

// Setup creates the HTTP router with all routes.
func Setup() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", handlers.HandleUser)
	mux.HandleFunc("/health", handlers.HandleHealth)
	return mux
}

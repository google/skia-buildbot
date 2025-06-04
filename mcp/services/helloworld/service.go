package helloworld

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.skia.org/infra/go/sklog"
)

type HelloWorldService struct {
}

// Initialize the service with the provided arguments.
func (s HelloWorldService) Init(serviceArgs string) error {
	return nil
}

// Register all the handlers for this service.
func (s HelloWorldService) RegisterHandlers(router *chi.Mux) {
	router.Get("/hello", s.hello)
}

// hello is the handler for the /hello api call.
func (s HelloWorldService) hello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode("Hello World"); err != nil {
		sklog.Errorf("Failed to write JSON response: %s", err)
	}
}

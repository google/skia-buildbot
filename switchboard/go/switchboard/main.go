package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
)

type server struct{}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	return &server{}, nil
}

func (s *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<h1>switchboard</h1>")
}

type Lease struct {
	Pod  string
	Port string
}

func (s *server) lease(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(Lease{
		Pod:  "switch-pod-0",
		Port: "9000",
	}); err != nil {
		httputils.ReportError(w, err, "Failed encoding lease", http.StatusInternalServerError)
	}
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", s.mainHandler).Methods("GET")
	r.HandleFunc("/lease", s.lease).Methods("GET")
}

// See baseapp.App.
func (s *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(new, []string{"switchboard.skia.org"})
}

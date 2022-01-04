package main

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"text/template"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/sklog"
)

type server struct {
	templates *template.Template
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	s := &server{}
	s.loadTemplates()
	return s, nil
}

func (s *server) loadTemplates() {
	s.templates = template.Must(template.New("").Delims("{%", "%}").ParseGlob(
		filepath.Join(*baseapp.ResourcesDir, "*.html"),
	))
}

// sendJSONResponse sends a JSON representation of any data structure as an HTTP response. If the
// conversion to JSON has an error, the error is logged.
func sendJSONResponse(data interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// sendHTMLResponse renders the given template, passing it the current context's CSP nonce. If
// template rendering fails, it logs an error.
func (s *server) sendHTMLResponse(templateName string, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := s.templates.ExecuteTemplate(w, templateName, map[string]string{
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (s *server) machinesPageHandler(w http.ResponseWriter, r *http.Request) {
	s.sendHTMLResponse("index.html", w, r)
}

func (s *server) bloatyHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(lovisolo): Implement.
	res := map[string]string{"msg": "Hello, world!"}
	sendJSONResponse(res, w)
}

// See baseapp.App.
func (s *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", s.machinesPageHandler).Methods("GET")
	r.HandleFunc("/rpc/bloaty/v1", s.bloatyHandler).Methods("GET")
}

// See baseapp.App.
func (s *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(new, []string{"codesize.skia.org"})
}

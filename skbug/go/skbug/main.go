package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/baseapp"
)

// server implements base.App.
type server struct {
}

func newServer() (baseapp.App, error) {
	return &server{}, nil
}

func (srv *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	bug := mux.Vars(r)["bug"]
	if bug == "" {
		http.Redirect(w, r, "https://bugs.chromium.org/p/skia/issues/list", http.StatusTemporaryRedirect)
	} else {
		http.Redirect(w, r, fmt.Sprintf("https://bugs.chromium.org/p/skia/issues/detail?id=%s", bug), http.StatusTemporaryRedirect)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/{bug:[a-z]*}", srv.indexHandler)
}

// See baseapp.App.
func (srv *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(newServer, []string{"skbug.com"})
}

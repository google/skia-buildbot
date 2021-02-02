package main

import (
	"flag"
	"html/template"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/scrap/go/client"
	"go.skia.org/infra/scrap/go/scrap"
)

// flags
var (
	scrapExchange = flag.String("scrapexchange", "http://scrapexchange:9000", "Scrap exchange service HTTP address.")
)

// server is the state of the server.
type server struct {
	scrapClient scrap.ScrapExchange
	templates   *template.Template
}

// See baseapp.Constructor.
func new() (baseapp.App, error) {
	// Need to set the mime-type for wasm files so streaming compile works.
	if err := mime.AddExtensionType(".wasm", "application/wasm"); err != nil {
		return nil, err
	}
	scrapClient, err := client.New(*scrapExchange)
	if err != nil {
		sklog.Fatalf("Failed to create scrap exchange client: %s", err)
	}

	srv := &server{
		scrapClient: scrapClient,
	}
	srv.loadTemplates()
	return srv, nil
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "main.html"),
	))
}

func (srv *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	if err := srv.templates.ExecuteTemplate(w, "main.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.mainHandler)
}

// See baseapp.App.
func (srv *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(new, []string{"shaders.skia.org"})
}

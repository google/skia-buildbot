package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"

	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/scrap/go/client"
	"go.skia.org/infra/scrap/go/fakeclient"
	"go.skia.org/infra/scrap/go/scrap"
	"go.skia.org/infra/shaders/go/config"
)

// flags
var (
	scrapExchange     = flag.String("scrapexchange", "http://scrapexchange:9000", "Scrap exchange service HTTP address.")
	fakeScrapExchange = flag.Bool("fake_scrapexchange", false, "If set to true, --scrapexchange will be ignored and a fake, in-memory implementation will be used instead.")
	fiddleOrigin      = flag.String("fiddle_origin", "https://fiddle.skia.org", `The fiddle origin (e.g. "https://fiddle.skia.org").`)
	jsFiddleOrigin    = flag.String("jsfiddle_origin", "https://jsfiddle.skia.org", `The jsfiddle origin (e.g. "https://jsfiddle.skia.org").`)
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
		return nil, skerr.Wrap(err)
	}
	var scrapClient scrap.ScrapExchange
	if *fakeScrapExchange {
		sklog.Infof("Using fake (in-memory) scrapexchange client")
		scrapClient = fakeclient.New(map[string]scrap.ScrapBody{
			"@default": {
				Type: "sksl",
				Body: blueNeuronShaderBody,
				SKSLMetaData: &scrap.SKSLMetaData{
					ImageURL: "/img/mandrill.png",
				},
			},
		})
	} else {
		var err error
		scrapClient, err = client.New(*scrapExchange)
		if err != nil {
			sklog.Fatalf("Failed to create scrap exchange client: %s", err)
		}
	}

	srv := &server{
		scrapClient: scrapClient,
	}
	srv.loadTemplates()
	return srv, nil
}

// isResourcePathCorsSafe determines if an image is OK to serve to another
// origin. |p| is the resource path relative to the /img directory.
func isResourcePathCorsSafe(p string) bool {
	return strings.HasSuffix(p, ".png")
}

// makeCorsResourceHandler is an HTTP handler function designed for serving files from the
// /img directory allowing cross-origin requests. It will only serve images deemed to be
// OK for other sites to access.
func makeCorsResourceHandler(resourcesDir string) func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		if !isResourcePathCorsSafe(r.URL.Path) {
			err := skerr.Fmt("%q is not an image", r.URL.Path)
			httputils.ReportError(w, err, "Resource not an image.", http.StatusUnauthorized)
			return
		}
		w.Header().Add("Cache-Control", "max-age=300")
		w.Header().Add("Access-Control-Allow-Origin", "*")
		fileServer.ServeHTTP(w, r)
	}
}

func (srv *server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "main.html"),
		filepath.Join(*baseapp.ResourcesDir, "debugger.html"),
	))
}

func (srv *server) pageHandler(w http.ResponseWriter, r *http.Request, p string) {
	w.Header().Set("Content-Type", "text/html")
	if *baseapp.Local {
		srv.loadTemplates()
	}
	context := config.SkShadersConfig{
		FiddleOrigin:   *fiddleOrigin,
		JsFiddleOrigin: *jsFiddleOrigin,
	}
	b, err := json.MarshalIndent(context, "", "  ")
	if err != nil {
		sklog.Errorf("Failed to JSON encode window.shaders context: %s", err)
		return
	}
	if err := srv.templates.ExecuteTemplate(w, p, map[string]interface{}{
		"context": template.JS(string(b)),
		// Look in //shaders/pages/BUILD.bazel for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func (srv *server) mainHandler(w http.ResponseWriter, r *http.Request) {
	srv.pageHandler(w, r, "main.html")
}

func (srv *server) debugHandler(w http.ResponseWriter, r *http.Request) {
	srv.pageHandler(w, r, "debugger.html")
}

func (srv *server) loadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	hashOrName := mux.Vars(r)["hashOrName"]

	body, err := srv.scrapClient.LoadScrap(r.Context(), scrap.SKSL, hashOrName)
	if err != nil {
		httputils.ReportError(w, err, "Failed to read JSON file.", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

func (srv *server) saveHandler(w http.ResponseWriter, r *http.Request) {
	// Decode Request.
	var req scrap.ScrapBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.ReportError(w, err, "Error decoding JSON.", http.StatusBadRequest)
		return
	}
	if req.Type != scrap.SKSL {
		httputils.ReportError(w, fmt.Errorf("Received invalid scrap type: %q", req.Type), "Invalid Type.", http.StatusBadRequest)
		return
	}

	scrapID, err := srv.scrapClient.CreateScrap(r.Context(), req)
	if err != nil {
		httputils.ReportError(w, err, "Error creating scrap.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(scrapID); err != nil {
		sklog.Errorf("Failed to write response: %s", err)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.mainHandler)
	r.HandleFunc("/debug", srv.debugHandler)
	r.HandleFunc("/_/load/{hashOrName:[@0-9a-zA-Z-_]+}", srv.loadHandler).Methods("GET")
	r.HandleFunc("/_/save/", srv.saveHandler).Methods("POST")

	// /img/ is an alias for /dist/ and serves(almost) the same files.
	// It differs from the /dist/ resource handler (defined in baseapp) in two ways:
	//
	// 1. The resource handler allows cross-origin resource fetches.
	// 2. Only shader images are allowed - all other requests will fail.
	r.PathPrefix("/img/").Handler(http.StripPrefix("/img/", http.HandlerFunc(makeCorsResourceHandler(*baseapp.ResourcesDir))))
}

// See baseapp.App.
func (srv *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(new, []string{"shaders.skia.org"}, baseapp.AllowWASM{}, baseapp.AllowAnyImage{})
}

// This is the same shader that is the current default on shaders.skia.org (the
// blue neuron-looking one).
const blueNeuronShaderBody = `
// Source: @notargs https://twitter.com/notargs/status/1250468645030858753
float f(vec3 p) {
    p.z -= iTime * 10.;
    float a = p.z * .1;
    p.xy *= mat2(cos(a), sin(a), -sin(a), cos(a));
    return .1 - length(cos(p.xy) + sin(p.yz));
}

half4 main(vec2 fragcoord) {
    vec3 d = .5 - fragcoord.xy1 / iResolution.y;
    vec3 p=vec3(0);
    for (int i = 0; i < 32; i++) {
      p += f(p) * d;
    }
    return ((sin(p) + vec3(2, 5, 9)) / length(p)).xyz1;
}`

package baseapp

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	Local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	Port         = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	PromPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	ResourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

const (
	SERVER_READ_TIMEOUT  = 5 * time.Minute
	SERVER_WRITE_TIMEOUT = 5 * time.Minute
)

// App is the interface that Constructor returns.
type App interface {
	// AddHandlers is called by Serve and the receiver must add all handlers
	// to the passed in mux.Router.
	AddHandlers(*mux.Router)

	// AddMiddleware returns a list of mux.Middleware's to add to the router.
	// This is a good place to add auth middleware.
	AddMiddleware() []mux.MiddlewareFunc
}

// Constructor is a function that builds an App instance.
//
// Used as a parameter to Serve.
type Constructor func() (App, error)

// cspReporter takes csp failure reports and turns them into structured log
// entries.
func cspReporter(w http.ResponseWriter, r *http.Request) {
	var body interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		sklog.Errorf("Failed to decode csp report: %s", err)
		return
	}
	c := struct {
		Type string      `json:"type"`
		Body interface{} `json:"body"`
	}{
		Type: "csp",
		Body: body,
	}
	b, err := json.Marshal(c)
	if err != nil {
		sklog.Errorf("Failed to marshal csp log entry: %s", err)
		return
	}
	fmt.Println(string(b))
	return
}

// cspString returns a properly formatted content security policy string.
func cspString(allowedHosts []string, local bool, options []Option) string {
	addScriptSrc := ""
	// Currently, when executing WebAssembly, if there is a non-empty CSP policy for a page (such as
	// when we are running with --local), the unsafe-eval policy must be enabled. See
	// https://chromestatus.com/feature/5499765773041664.
	if local || hasWASMOption(options) {
		addScriptSrc = "'unsafe-eval'"
	}

	imgSrc := "'self'"
	if hasAllowAnyImageOption(options) {
		// unsafe-eval allows us to get to the underlying bits of the image.
		imgSrc = "* 'unsafe-eval' blob: data:"
	}

	// This non-local, CSP string without any options passes the tests at https://csp-evaluator.withgoogle.com/.
	//
	// See also: https://csp.withgoogle.com/docs/strict-csp.html
	//
	return fmt.Sprintf("base-uri 'none';  img-src %s ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE %s 'unsafe-inline' https: http: ; report-uri /cspreport ;", imgSrc, addScriptSrc)
}

func securityMiddleware(allowedHosts []string, local bool, options []Option) mux.MiddlewareFunc {

	// Apply CSP and other security minded headers.
	secureMiddleware := secure.New(secure.Options{
		AllowedHosts:          allowedHosts,
		HostsProxyHeaders:     []string{"X-Forwarded-Host"},
		SSLRedirect:           true,
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
		STSSeconds:            60 * 60 * 24 * 365,
		STSIncludeSubdomains:  true,
		ContentSecurityPolicy: cspString(allowedHosts, local, options),
		IsDevelopment:         local,
	})

	return secureMiddleware.Handler
}

// Option is the base type for options passed to Serve().
type Option interface{}

// AllowWASM allows 'unsafe-eval' for scripts, which is needed for WASM.
type AllowWASM struct{}

func hasWASMOption(options []Option) bool {
	for _, opt := range options {
		if _, ok := opt.(AllowWASM); ok {
			return true
		}
	}
	return false
}

// AllowAnyImage allows images to be loaded from all sources, not just self.
type AllowAnyImage struct{}

func hasAllowAnyImageOption(options []Option) bool {
	for _, opt := range options {
		if _, ok := opt.(AllowAnyImage); ok {
			return true
		}
	}
	return false
}

// Serve builds and runs the App in a secure manner in our kubernetes cluster.
//
// The constructor builds an App instance. Note that we don't pass in an App
// instance directly, because we want the constructor called after the
// common.Init*() functions are called, i.e. after flags are parsed.
//
// The allowedHosts are the list of domains that are allowed to make requests
// to this app. Make sure to include the domain name of the app itself. For
// example; []string{"am.skia.org"}.
//
// See https://csp.withgoogle.com/docs/strict-csp.html for more information on
// Strict CSP in general.
//
// For this to work every script and style tag must have a nonce attribute
// whose value matches the one sent in the Content-Security-Policy: header. You
// can have Bazel inject an attribute with a template for the nonce to all
// <script> and <link> tags via the sk_page rule's nonce attribute, e.g.
//
//    load("//infra-sk:index.bzl", "sk_page")
//
//    sk_page(
//        name = "index",
//        html_file = "index.html",
//        nonce = "{% .Nonce %}",
//        ...
//    )
//
// And then include that nonce when expanding any pages:
//
//     if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
//       "Nonce": secure.CSPNonce(r.Context()),
//     }); err != nil {
//      sklog.Errorf("Failed to expand template: %s", err)
//     }
//
// Since our audience is small and only uses modern browsers we shouldn't need
// any further XSS protection. For example, even if a user is logged into
// another Google site that is compromised, while they can request the main
// index page and get both the csrf token and value, they couldn't POST it back
// to the site we are serving since that site wouldn't be listed in
// allowedHosts.
//
// CSP failures will be logged as structured log events.
//
// Static resources, e.g. Bazel-built HTML, CSS and JS files, will be served at
// '/dist/' and will serve the contents of the '/dist' directory.
func Serve(constructor Constructor, allowedHosts []string, options ...Option) {
	// Do common init.
	common.InitWithMust(
		"generic-k8s-app",
		common.PrometheusOpt(PromPort),
		common.MetricsLoggingOpt(),
	)

	// Fix up flag values.
	if *ResourcesDir == "" {
		_, filename, _, _ := runtime.Caller(1)
		*ResourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	// Build App instance.
	app, err := constructor()
	if err != nil {
		sklog.Fatal(err)
	}

	// Add all routing.
	r := mux.NewRouter()
	r.HandleFunc("/cspreport", cspReporter).Methods("POST")
	// The /static/ path is kept for legacy apps, but all apps should migrate to /dist/
	// to work with puppeteer.
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.MakeResourceHandler(*ResourcesDir))))
	r.PathPrefix("/dist/").Handler(http.StripPrefix("/dist/", http.HandlerFunc(httputils.MakeResourceHandler(*ResourcesDir))))
	app.AddHandlers(r)

	// We must specify that we handle /healthz or it will never flow through to our middleware.
	// Even though this handler is never actually called (due to the early termination in
	// httputils.HealthzAndHTTPS), we need to have it added to the routes we handle.
	r.HandleFunc("/healthz", httputils.ReadyHandleFunc)

	// Layer on all the middleware.
	middleware := []mux.MiddlewareFunc{}
	if !*Local {
		middleware = append(middleware, httputils.HealthzAndHTTPS)
	}
	middleware = append(middleware, app.AddMiddleware()...)
	middleware = append(middleware,
		httputils.LoggingGzipRequestResponse,
		securityMiddleware(allowedHosts, *Local, options),
	)
	r.Use(middleware...)

	// Start serving.
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Ready to serve at http://%s%s", hostname, *Port) // The port string includes a colon, e.g. ":8000".
	server := &http.Server{
		Addr:           *Port,
		Handler:        r,
		ReadTimeout:    SERVER_READ_TIMEOUT,
		WriteTimeout:   SERVER_WRITE_TIMEOUT,
		MaxHeaderBytes: 1 << 20,
	}
	sklog.Fatal(server.ListenAndServe())
}

package baseapp

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
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

// Applications that want to use Serve() must conform the App interface.
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

func securityMiddleware(allowedHosts []string) mux.MiddlewareFunc {
	// Configure Content Security Policy (CSP).
	addScriptSrc := ""
	if *Local {
		// webpack uses eval() in development mode, so allow unsafe-eval when local.
		addScriptSrc = "'unsafe-eval'"
	}
	// This non-local CSP string passes the tests at https://csp-evaluator.withgoogle.com/.
	//
	// See also: https://csp.withgoogle.com/docs/strict-csp.html
	cspString := fmt.Sprintf("base-uri 'none';  img-src 'self' ; object-src 'none' ; style-src 'self'  https://fonts.googleapis.com/ https://www.gstatic.com/ 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE 'unsafe-inline' %s https: http: ; report-uri /cspreport ;", addScriptSrc)

	// Apply CSP and other security minded headers.
	secureMiddleware := secure.New(secure.Options{
		AllowedHosts:          allowedHosts,
		HostsProxyHeaders:     []string{"X-Forwarded-Host"},
		SSLRedirect:           true,
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
		STSSeconds:            60 * 60 * 24 * 365,
		STSIncludeSubdomains:  true,
		ContentSecurityPolicy: cspString,
		IsDevelopment:         *Local,
	})

	return secureMiddleware.Handler
}

// Serve builds and runs the App in a secure manner in out kubernetes cluster.
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
// can have webpack inject an attribute with a template for the nonce by adding
// the HtmlWebPackInjectAttributesPlugin to your plugins, i.e.
//
//    config.plugins.push(
//      new HtmlWebpackInjectAttributesPlugin({
//        nonce: "{%.nonce%}",
//      }),
//    );
//
// And then include that nonce when expanding any pages:
//
//    if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
//      "nonce": secure.CSPNonce(r.Context()),
//    }); err != nil {
//     sklog.Errorf("Failed to expand template: %s", err)
//   }
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
// Static resources, e.g. webpack output, will be served at '/static/' and will
// serve the contents of the '/dist' directory.
func Serve(constructor Constructor, allowedHosts []string) {
	// Do common init.
	common.InitWithMust(
		"generic-k8s-app", // The app name is only used by ../go/sklog/cloud_logging, and we don't use that on k8s.
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
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.HandlerFunc(httputils.MakeResourceHandler(*ResourcesDir))))
	app.AddHandlers(r)

	// Layer on all the middleware.
	middleware := []mux.MiddlewareFunc{}
	if !*Local {
		middleware = append(middleware, httputils.HealthzAndHTTPS)
	}
	middleware = append(middleware, app.AddMiddleware()...)
	middleware = append(middleware,
		httputils.LoggingGzipRequestResponse,
		securityMiddleware(allowedHosts),
	)
	r.Use(middleware...)

	// Start serving.
	sklog.Info("Ready to serve.")
	server := &http.Server{
		Addr:           *Port,
		Handler:        r,
		ReadTimeout:    SERVER_READ_TIMEOUT,
		WriteTimeout:   SERVER_WRITE_TIMEOUT,
		MaxHeaderBytes: 1 << 20,
	}
	sklog.Fatal(server.ListenAndServe())
}

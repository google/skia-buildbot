package baseapp

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/skia-dev/secure"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
)

var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

type App interface {
	AddHandlers(*mux.Router)
}

type Constructor func() (App, error)

// cspReportWrapper wraps a handler and intercepts csp failure reports and
// turns them into structured log entries.
//
// Note this should be outside the csrf wrapper since execution may fail
// before the csrf is in place.
func cspReportWrapper(h http.Handler) http.Handler {
	s := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cspreport" && r.Method == "POST" {
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
		} else {
			h.ServeHTTP(w, r)
		}
	}
	return http.HandlerFunc(s)
}

func applySecurityWrappers(h http.Handler, salt []byte, domain string) http.Handler {
	// Configure Content Security Policy (CSP).
	addScriptSrc := ""
	if *local {
		// webpack uses eval() in development mode, so allow unsafe-eval when local.
		addScriptSrc = "'unsafe-eval'"
	}
	// This non-local CSP string passes the tests at https://csp-evaluator.withgoogle.com/.
	//
	// See also: https://csp.withgoogle.com/docs/strict-csp.html
	cspString := fmt.Sprintf("base-uri 'none';  img-src 'self' ; object-src 'none' ; style-src 'self' 'unsafe-inline' ; script-src 'strict-dynamic' $NONCE 'unsafe-inline' %s https: http: ; report-uri /cspreport ;", addScriptSrc)

	// Apply CSP and other security minded headers.
	secureMiddleware := secure.New(secure.Options{
		AllowedHosts:          []string{domain},
		HostsProxyHeaders:     []string{"X-Forwarded-Host"},
		SSLRedirect:           true,
		SSLProxyHeaders:       map[string]string{"X-Forwarded-Proto": "https"},
		STSSeconds:            60 * 60 * 24 * 365,
		STSIncludeSubdomains:  true,
		ContentSecurityPolicy: cspString,
		IsDevelopment:         *local,
	})

	h = secureMiddleware.Handler(h)
	h = csrf.Protect(salt, csrf.Secure(!*local), csrf.Path("/"))(h)
	return h
}

func Serve(constructor Constructor, name string, domain string) {
	common.InitWithMust(
		name,
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	app, err := constructor()
	if err != nil {
		sklog.Fatal(err)
	}
	r := mux.NewRouter()
	app.AddHandlers(r)

	// Setup the salt.
	salt := []byte("32-byte-long-auth-key")
	if !*local {
		var err error
		salt, err = ioutil.ReadFile("/var/skia/salt.txt")
		if err != nil {
			sklog.Fatal(err)
		}
	}

	h := applySecurityWrappers(r, salt, domain)
	if !*local {
		h = httputils.LoggingGzipRequestResponse(h)

		// This needs to be done via opts.
		h = login.RestrictViewer(h)
		h = login.ForceAuth(h, login.DEFAULT_REDIRECT_URL)

		h = httputils.HealthzAndHTTPS(h)
	}
	h = cspReportWrapper(h)
	http.Handle("/", h)
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

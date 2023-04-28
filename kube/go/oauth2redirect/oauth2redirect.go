// Package oauth2redirect is a reverse proxy that runs in front of applications
// and takes care of handling the oauth2 redirect leg of the OAuth 3-legged
// flow. It passes all other traffic to the application it is running in front
// of.
//
// This is useful so that we don't need to redeploy docsyserver everytime a
// change is made to //go/login, instead just this smaller proxy can be deployed
// at the same time as go/auth-proxy is deployed.
package oauth2redirect

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	appName            = "oauth2redirect"
	serverReadTimeout  = time.Hour
	serverWriteTimeout = time.Hour
	drainTime          = time.Minute
)

type proxy struct {
	reverseProxy http.Handler
}

func newProxy(target *url.URL) *proxy {
	reverseProxy := httputil.NewSingleHostReverseProxy(target)

	return &proxy{
		reverseProxy: reverseProxy,
	}
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case login.DefaultOAuth2Callback:
		login.OAuth2CallbackHandler(w, r)
	case login.LoginPath:
		login.LoginHandler(w, r)
	case login.LogoutPath:
		login.LogoutHandler(w, r)
	default:
		p.reverseProxy.ServeHTTP(w, r)
	}
}

// App is the oauth2redirect application.
type App struct {
	port       string
	promPort   string
	local      bool
	targetPort string
	domain     string

	target *url.URL
	server *http.Server
	proxy  *proxy
}

// Flagset constructs a flag.FlagSet for the App.
func (a *App) Flagset() *flag.FlagSet {
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	fs.StringVar(&a.port, "port", ":8000", "HTTP service address (e.g., ':8000')")
	fs.StringVar(&a.promPort, "prom-port", ":20000", "Metrics service address (e.g., ':10110')")
	fs.BoolVar(&a.local, "local", false, "Running locally if true. As opposed to in production.")
	fs.StringVar(&a.targetPort, "target_port", ":9000", "The port we are proxying to, or a full URL.")
	fs.StringVar(&a.domain, "domain", string(login.SkiaOrg), fmt.Sprintf("The domain to handle oauth2 callbacks for, choose from: %q", login.AllDomainNames))

	return fs
}

func newEmptyApp() *App {
	return &App{
		proxy: nil,
	}
}

// New returns a new *App.
func New(ctx context.Context, opts ...login.InitOption) (*App, error) {
	ret := newEmptyApp()

	err := common.InitWith(
		appName,
		common.PrometheusOpt(&ret.promPort),
		common.FlagSetOpt(ret.Flagset()),
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	opts = append(opts, login.DomainName(ret.domain))
	err = login.Init(ctx, login.DefaultOAuth2Callback, "", "", opts...)
	if err != nil {
		sklog.Fatal(err)
	}

	target, err := parseTargetPort(ret.targetPort)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ret.target = target
	ret.registerCleanup()

	return ret, nil
}

// Parses either a port, e.g. ":8000", or a full URL into a *url.URL.
func parseTargetPort(u string) (*url.URL, error) {
	if strings.HasPrefix(u, ":") {
		return url.Parse(fmt.Sprintf("http://localhost%s", u))
	}
	return url.Parse(u)
}

func (a *App) registerCleanup() {
	cleanup.AtExit(func() {
		if a.server != nil {
			sklog.Info("Shutdown server gracefully.")
			ctx, cancel := context.WithTimeout(context.Background(), drainTime)
			err := a.server.Shutdown(ctx)
			if err != nil {
				sklog.Error(err)
			}
			cancel()
		}
	})

}

// Run starts the application serving, it does not return unless there is an
// error or the passed in context is cancelled.
func (a *App) Run(ctx context.Context) error {
	a.proxy = newProxy(a.target)

	var h http.Handler = a.proxy
	if !a.local {
		h = httputils.HealthzAndHTTPS(h)
	}
	server := &http.Server{
		Addr:           a.port,
		Handler:        h,
		ReadTimeout:    serverReadTimeout,
		WriteTimeout:   serverWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	a.server = server

	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		// This is an orderly shutdown.
		return nil
	}
	return skerr.Wrap(err)
}

// Main constructs and runs the application. This function will only return on failure.
func Main() error {
	ctx := context.Background()
	app, err := New(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	return app.Run(ctx)
}

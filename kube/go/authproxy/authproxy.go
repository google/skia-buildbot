// auth-proxy is a reverse proxy that runs in front of applications and takes
// care of authentication.
//
// This is useful for applications like Promentheus that doesn't handle
// authentication itself, so we can run it behind auth-proxy to restrict access.
//
// The auth-proxy application also adds the X-WEBAUTH-USER header to each
// authenticated request and gives it the value of the logged in users email
// address, which can be used for audit logging. The application running behind
// auth-proxy should then use:
//
//     https://pkg.go.dev/go.skia.org/infra/go/alogin/proxylogin
//
// When using --cria_group this application should be run using work-load
// identity with a service account that as read access to CRIA, such as:
//
//     skia-auth-proxy-cria-reader@skia-public.iam.gserviceaccount.com
//
// See also:
//
//     https://chrome-infra-auth.appspot.com/auth/groups/project-skia-auth-service-access
//
//     https://grafana.com/blog/2015/12/07/grafana-authproxy-have-it-your-way/
package authproxy

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy/auth"
	"golang.org/x/oauth2/google"
)

const (
	appName            = "auth-proxy"
	serverReadTimeout  = time.Hour
	serverWriteTimeout = time.Hour
	drainTime          = time.Minute
)

const (
	// Send the logged in user email in the following header. This allows decoupling
	// of authentication from the core of the app. See
	// https://grafana.com/blog/2015/12/07/grafana-authproxy-have-it-your-way/ for
	// how Grafana uses this to support almost any authentication handler.
	WebAuthHeaderName = "X-WEBAUTH-USER"

	WebAuthRoleHeaderName = "X-ROLES"
)

type proxy struct {
	allowPost    bool
	passive      bool
	reverseProxy http.Handler
	authProvider auth.Auth
	allowedRoles map[roles.Role]allowed.Allow
}

func newProxy(target *url.URL, authProvider auth.Auth, allowedRules map[roles.Role]allowed.Allow, allowPost bool, passive bool) *proxy {
	return &proxy{
		reverseProxy: httputil.NewSingleHostReverseProxy(target),
		authProvider: authProvider,
		allowedRoles: allowedRules,
		allowPost:    allowPost,
		passive:      passive,
	}
}

func (p proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	email := p.authProvider.LoggedInAs(r)
	r.Header.Del(WebAuthHeaderName)
	r.Header.Add(WebAuthHeaderName, email)

	authorizedRoles := roles.Roles{}
	for role, allowed := range p.allowedRoles {
		if allowed.Member(email) {
			authorizedRoles = append(authorizedRoles, role)
		}
	}

	r.Header.Del(WebAuthRoleHeaderName)
	r.Header.Add(WebAuthRoleHeaderName, authorizedRoles.ToHeader())

	if r.Method == "POST" && p.allowPost {
		p.reverseProxy.ServeHTTP(w, r)
		return
	}
	if !p.passive {
		if email == "" {
			http.Redirect(w, r, p.authProvider.LoginURL(w, r), http.StatusSeeOther)
			return
		}
		if len(authorizedRoles) == 0 {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
	}
	p.reverseProxy.ServeHTTP(w, r)
}

// App is the auth-proxy application.
type App struct {
	port        string
	promPort    string
	criaGroup   string
	local       bool
	targetPort  string
	allowPost   bool
	allowedFrom string
	passive     bool
	roleFlags   []string

	target       *url.URL
	authProvider auth.Auth
	server       *http.Server
	allowedRoles map[roles.Role]allowed.Allow
}

// Flagset constructs a flag.FlagSet for the App.
func (a *App) Flagset() *flag.FlagSet {
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	fs.StringVar(&a.port, "port", ":8000", "HTTP service address (e.g., ':8000')")
	fs.StringVar(&a.promPort, "prom-port", ":20000", "Metrics service address (e.g., ':10110')")
	fs.StringVar(&a.criaGroup, "cria_group", "", "The chrome infra auth group to use for restricting access. Example: 'google/skia-staff@google.com'")
	fs.BoolVar(&a.local, "local", false, "Running locally if true. As opposed to in production.")
	fs.StringVar(&a.targetPort, "target_port", ":9000", "The port we are proxying to.")
	fs.BoolVar(&a.allowPost, "allow_post", false, "Allow POST requests to bypass auth.")
	fs.StringVar(&a.allowedFrom, "allowed_from", "", "A comma separated list of of domains and email addresses that are allowed to access the site. Example: 'google.com'")
	fs.BoolVar(&a.passive, "passive", false, "If true then allow unauthenticated requests to go through, while still adding logged in users emails in via the webAuthHeaderName.")
	common.MultiStringFlagVar(&a.roleFlags, "role", []string{}, "Define a role and the group (CRIA, domain, email list) that defines who gets that role via flags. For example: --role=viewer=@google.com OR --role=triager=cria_group:project-angle-committers")

	return fs
}

func newEmptyApp() *App {
	return &App{
		allowedRoles: map[roles.Role]allowed.Allow{},
	}
}

// New returns a new *App.
func New(ctx context.Context) (*App, error) {
	ret := newEmptyApp()

	err := common.InitWith(
		appName,
		common.PrometheusOpt(&ret.promPort),
		common.MetricsLoggingOpt(),
		common.FlagSetOpt(ret.Flagset()),
	)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	err = ret.validateFlags()
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	ts, err := google.DefaultTokenSource(ctx, "email")
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	criaClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	err = ret.populateLegacyAllowedRoles(criaClient)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	err = ret.populateAllowedRoles(criaClient)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	authInstance := auth.New()
	err = authInstance.Init(ret.port, ret.local)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	targetURL := fmt.Sprintf("http://localhost%s", ret.targetPort)
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ret.authProvider = authInstance
	ret.target = target
	ret.registerCleanup()

	return ret, nil
}

func (a *App) populateLegacyAllowedRoles(criaClient *http.Client) error {
	var allow allowed.Allow
	if a.criaGroup != "" {
		var err error
		allow, err = allowed.NewAllowedFromChromeInfraAuth(criaClient, a.criaGroup)
		if err != nil {
			return skerr.Wrap(err)
		}
	} else { // --allowed_from
		allow = allowed.NewAllowedFromList(strings.Split(a.allowedFrom, ","))
	}
	a.allowedRoles[roles.Viewer] = allow
	return nil
}

func (a *App) populateAllowedRoles(criaClient *http.Client) error {
	for _, roleFlag := range a.roleFlags {
		parts := strings.Split(roleFlag, "=")
		if len(parts) != 2 {
			return skerr.Fmt("Invalid format for --role flag: %q", roleFlag)
		}
		rolename := roles.RoleFromString(parts[0])
		if rolename == roles.InvalidRole {
			return skerr.Fmt("Invalid Role: %q", roleFlag)
		}
		allowedRuleAsString := parts[1]

		var allow allowed.Allow
		if strings.HasPrefix(allowedRuleAsString, "cria_group:") {
			var err error
			allow, err = allowed.NewAllowedFromChromeInfraAuth(criaClient, allowedRuleAsString[len("cria_group:"):])
			if err != nil {
				return skerr.Fmt("Failed parsing --role flag: %q : %s", roleFlag, err)
			}
		} else {
			allow = allowed.NewAllowedFromList(strings.Split(allowedRuleAsString, ","))
		}
		a.allowedRoles[rolename] = allow
	}
	return nil
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
	var h http.Handler = newProxy(a.target, a.authProvider, a.allowedRoles, a.allowPost, a.passive)
	h = httputils.HealthzAndHTTPS(h)
	server := &http.Server{
		Addr:           a.port,
		Handler:        h,
		ReadTimeout:    serverReadTimeout,
		WriteTimeout:   serverWriteTimeout,
		MaxHeaderBytes: 1 << 20,
	}
	a.server = server

	sklog.Infof("Ready to serve on port %s", a.port)
	err := server.ListenAndServe()
	if err == http.ErrServerClosed {
		// This is an orderly shutdown.
		return nil
	}
	return skerr.Wrap(err)
}

func (a *App) validateFlags() error {
	if len(a.roleFlags) > 0 && (a.criaGroup != "" || a.allowedFrom != "") {
		return fmt.Errorf("Can not mix --role and [--auth_group, --allowed_from] flags.")
	}
	if a.criaGroup != "" && a.allowedFrom != "" {
		return fmt.Errorf("Only one of the flags in [--auth_group, --allowed_from] can be specified.")
	}
	if a.criaGroup == "" && a.allowedFrom == "" {
		return fmt.Errorf("At least one of the flags in [--auth_group, --allowed_from] must be specified.")
	}

	return nil
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

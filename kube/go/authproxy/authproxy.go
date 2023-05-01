// Package authproxy is a reverse proxy that runs in front of applications and
// takes care of authentication.
//
// This is useful for applications like Promentheus that doesn't handle
// authentication itself, so we can run it behind auth-proxy to restrict access.
//
// The auth-proxy application also adds the X-WEBAUTH-USER header to each
// authenticated request and gives it the value of the logged in users email
// address, which can be used for audit logging. The application running behind
// auth-proxy should then use:
//
//	https://pkg.go.dev/go.skia.org/infra/go/alogin/proxylogin
//
// When using --cria_group this application should be run using work-load
// identity with a service account that as read access to CRIA, such as:
//
//	skia-auth-proxy-cria-reader@skia-public.iam.gserviceaccount.com
//
// See also:
//
//	https://chrome-infra-auth.appspot.com/auth/groups/project-skia-auth-service-access
//
//	https://grafana.com/blog/2015/12/07/grafana-authproxy-have-it-your-way/
package authproxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/authproxy/auth"
	"go.skia.org/infra/kube/go/authproxy/mockedauth"
	"go.skia.org/infra/kube/go/authproxy/protoheader"
	"golang.org/x/net/http2"
	"golang.org/x/oauth2/google"
)

const (
	appName             = "auth-proxy"
	serverReadTimeout   = time.Hour
	serverWriteTimeout  = time.Hour
	drainTime           = time.Minute
	criaRefreshDuration = time.Hour
)

const (
	// Send the logged in user email in the following header. This allows decoupling
	// of authentication from the core of the app. See
	// https://grafana.com/blog/2015/12/07/grafana-authproxy-have-it-your-way/ for
	// how Grafana uses this to support almost any authentication handler.

	// WebAuthHeaderName is the name of the header sent to the application that
	// contains the users email address.
	WebAuthHeaderName = "X-WEBAUTH-USER"

	// WebAuthRoleHeaderName is the name of the header sent to the application
	// that contains the users Roles.
	WebAuthRoleHeaderName = "X-WEBAUTH-ROLES"
)

type proxy struct {
	allowPost    bool
	passive      bool
	reverseProxy http.Handler
	authProvider auth.Auth

	// mutex protects allowedRoles
	mutex        sync.RWMutex
	allowedRoles map[roles.Role]allowed.Allow
}

func newProxy(target *url.URL, authProvider auth.Auth, allowPost bool, passive bool, local bool, useHTTP2 bool) *proxy {
	reverseProxy := httputil.NewSingleHostReverseProxy(target)
	if useHTTP2 {
		// [httputil.ReverseProxy] doesn't appear work out of the box for local gRPC requests. Either the
		// proxy or the grpc server will prematurely close the upstream connection before processing the
		// round trip between proxy to grpc upstream, causing an unexpected EOF at the proxy. The proxy
		// then returns Bad Gateway to the client.
		// https://github.com/golang/go/issues/29928 described similar symptoms to what
		// I was seeing. The github issue comments included the fix below, which overrides the default
		// DialTLS function in [http2.Transport] ([tls.Dial]) to use [net.DialTCP] instead.
		// I had also tried [http2.ConfigureTransport] prior to this workaround, but it did not fix the
		// problem.
		reverseProxy.Transport =
			&http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
					ta, err := net.ResolveTCPAddr(network, addr)
					if err != nil {
						return nil, err
					}
					return net.DialTCP(network, nil, ta)
				},
			}
	}

	return &proxy{
		reverseProxy: reverseProxy,
		authProvider: authProvider,
		allowPost:    allowPost,
		passive:      passive,
	}
}

func (p *proxy) setAllowedRoles(allowedRoles map[roles.Role]allowed.Allow) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.allowedRoles = allowedRoles
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	email := p.authProvider.LoggedInAs(r)
	r.Header.Del(WebAuthHeaderName)
	r.Header.Add(WebAuthHeaderName, email)

	p.mutex.RLock()
	authorizedRoles := roles.Roles{}
	for role, allowed := range p.allowedRoles {
		if allowed.Member(email) {
			authorizedRoles = append(authorizedRoles, role)
		}
	}
	p.mutex.RUnlock()

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

// AuthType represents the types of authentication auth-proxy can handle.
type AuthType string

const (
	// OAuth2 uses the legacy OAuth 2.0 flow.
	OAuth2 AuthType = "oauth2"

	// ProtoHeader uses an incoming HTTP header with a serialized proto.
	ProtoHeader AuthType = "protoheader"

	// Mocked uses a string provided on the command line for the user identity
	Mocked AuthType = "mocked"

	// Invalid represents an invalid authentication scheme.
	Invalid AuthType = ""
)

// AllValidAuthTypes is a list of all valid AuthTypes.
var AllValidAuthTypes = []AuthType{OAuth2, ProtoHeader, Mocked}

// ToAuthType converts a string to AuthType, returning Invalid if it is not a
// valid type.
func ToAuthType(s string) AuthType {
	for _, t := range AllValidAuthTypes {
		if s == string(t) {
			return t
		}
	}
	return Invalid
}

// App is the auth-proxy application.
type App struct {
	port                 string
	promPort             string
	local                bool
	targetPort           string
	allowPost            bool
	passive              bool
	roleFlags            []string
	authType             string
	mockLoggedInAs       string
	selfSignLocalhostTLS bool

	target       *url.URL
	authProvider auth.Auth
	server       *http.Server
	criaClient   *http.Client
	proxy        *proxy
}

// Flagset constructs a flag.FlagSet for the App.
func (a *App) Flagset() *flag.FlagSet {
	fs := flag.NewFlagSet(appName, flag.ExitOnError)
	fs.StringVar(&a.port, "port", ":8000", "HTTP service address (e.g., ':8000')")
	fs.StringVar(&a.promPort, "prom-port", ":20000", "Metrics service address (e.g., ':10110')")
	fs.BoolVar(&a.local, "local", false, "Running locally if true. As opposed to in production.")
	fs.StringVar(&a.targetPort, "target_port", ":9000", "The port we are proxying to, or a full URL.")
	fs.BoolVar(&a.allowPost, "allow_post", false, "Allow POST requests to bypass auth.")
	fs.BoolVar(&a.passive, "passive", false, "If true then allow unauthenticated requests to go through, while still adding logged in users emails in via the webAuthHeaderName.")
	common.FSMultiStringFlagVar(fs, &a.roleFlags, "role", []string{}, "Define a role and the group (CRIA, domain, email list) that defines who gets that role via flags. For example: --role=viewer=@google.com OR --role=triager=cria_group:project-angle-committers")
	fs.StringVar(&a.authType, "authtype", string(OAuth2), fmt.Sprintf("The type of authentication to do. Choose from: %q", AllValidAuthTypes))
	fs.StringVar(&a.mockLoggedInAs, "mock_user", "", "If authtype is set to 'mocked', then always return this value for the logged in user identity")
	fs.BoolVar(&a.selfSignLocalhostTLS, "self_sign_localhost_tls", false, "if true, serve TLS using a self-signed certificate for localhost")

	return fs
}

func newEmptyApp() *App {
	return &App{
		proxy: &proxy{},
	}
}

// New returns a new *App.
func New(ctx context.Context) (*App, error) {
	ret := newEmptyApp()

	err := common.InitWith(
		appName,
		common.PrometheusOpt(&ret.promPort),
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
	ret.criaClient = httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	var authInstance auth.Auth
	switch ToAuthType(ret.authType) {
	case ProtoHeader:
		secretClient, err := secret.NewClient(ctx)
		if err != nil {
			return ret, skerr.Wrap(err)
		}
		authInstance, err = protoheader.New(ctx, secretClient)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
	case OAuth2:
		authInstance = auth.New()
	case Mocked:
		authInstance = mockedauth.New(ret.mockLoggedInAs)
	case Invalid:
		return nil, skerr.Fmt("Invalid value for --authtype flag: %q", ret.authType)
	}

	err = authInstance.Init(ctx, ret.port, ret.local)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	target, err := parseTargetPort(ret.targetPort)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	ret.authProvider = authInstance
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

func (a *App) populateAllowedRoles() error {
	allowedRoles := map[roles.Role]allowed.Allow{}
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
			allow, err = allowed.NewAllowedFromChromeInfraAuth(a.criaClient, allowedRuleAsString[len("cria_group:"):])
			if err != nil {
				return skerr.Fmt("Failed parsing --role flag: %q : %s", roleFlag, err)
			}
		} else {
			allow = allowed.NewAllowedFromList(strings.Split(allowedRuleAsString, " "))
		}
		if existing, ok := allowedRoles[rolename]; ok {
			allowedRoles[rolename] = allowed.UnionOf(existing, allow)
		} else {
			allowedRoles[rolename] = allow
		}
	}
	a.proxy.setAllowedRoles(allowedRoles)
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

func genLocalhostCert() (tls.Certificate, error) {
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.Unix()),
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(0, 0, 1),
		SubjectKeyId:          []byte("/CN=localhost"),
		BasicConstraintsValid: true,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{{127, 0, 0, 1}},
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}

	cert, err := x509.CreateCertificate(rand.Reader, template, template,
		priv.Public(), priv)
	if err != nil {
		return tls.Certificate{}, err
	}

	var outCert tls.Certificate
	outCert.Certificate = append(outCert.Certificate, cert)
	outCert.PrivateKey = priv

	return outCert, nil
}

// startAllowedRefresh periodically refreshes the definitions of CRIA groups.
//
// If the passed in context is cancelled then the Go routine will exit.
func (a *App) startAllowedRefresh(ctx context.Context, criaRefreshDuration time.Duration) {
	// Start refreshing the allowed roles from CRIA.
	go func() {
		failedMetric := metrics2.GetCounter("auth_proxy_cria_refresh_failed")
		ticker := time.NewTicker(criaRefreshDuration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := a.populateAllowedRoles()
				if err != nil {
					sklog.Errorf("Refreshing allowed roles: %s", err)
					failedMetric.Inc(1)
				} else {
					failedMetric.Reset()
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Run starts the application serving, it does not return unless there is an
// error or the passed in context is cancelled.
func (a *App) Run(ctx context.Context) error {
	a.proxy = newProxy(a.target, a.authProvider, a.allowPost, a.passive, a.local, a.selfSignLocalhostTLS)
	err := a.populateAllowedRoles()
	if err != nil {
		return skerr.Wrap(err)
	}

	a.startAllowedRefresh(ctx, criaRefreshDuration)

	var h http.Handler = a.proxy
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
	if a.selfSignLocalhostTLS {
		cert, err := genLocalhostCert()
		if err != nil {
			return err
		}
		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		err = server.ListenAndServeTLS("", "")
	} else {
		err = server.ListenAndServe()
	}
	if err == http.ErrServerClosed {
		// This is an orderly shutdown.
		return nil
	}
	return skerr.Wrap(err)
}

func (a *App) validateFlags() error {
	if len(a.roleFlags) == 0 {
		return fmt.Errorf("At least one --role flag must be supplied.")
	}
	if a.authType == string(Mocked) && a.mockLoggedInAs == "" {
		return fmt.Errorf("--mock_user is required when --authtype is %q", Mocked)
	}
	if a.authType != string(Mocked) && a.mockLoggedInAs != "" {
		return fmt.Errorf("--mock_user is not allowed if --authtype is not %q", Mocked)
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

// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"context"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/insights"
	"go.skia.org/infra/perf/go/insights/mcptools"
	"go.skia.org/infra/perf/go/insights/rest"
	"go.skia.org/infra/perf/go/insights/site"
	"golang.org/x/time/rate"
)

const (
	// Headers set by Skia's auth-proxy (//kube/go/authproxy).
	webAuthUserHeader  = "X-WEBAUTH-USER"
	webAuthRolesHeader = "X-WEBAUTH-ROLES"

	// maxLimiters bounds the number of per-caller rate limiters kept in memory.
	maxLimiters = 10000

	// globalRateFactor sets the server-wide rate limit as a multiple of the per-caller limit.
	globalRateFactor = 10
)

// serverOptions configure the HTTP handler.
type serverOptions struct {
	// AllowedHosts are the accepted values of the Host header, e.g. "127.0.0.1:8090". Requests for
	// other hosts are rejected, which defeats DNS rebinding attacks from web pages.
	AllowedHosts []string

	// AllowedHostSuffixes accept whole domains in the Host header, e.g. ".proxy.googlers.com" for
	// the UberProxy PEN hostnames that serve a Cloudtop port. UberProxy only routes a PEN hostname
	// to the address encoded in it, i.e. to this server.
	AllowedHostSuffixes []string

	// AllowedOrigins are the accepted values of the Origin header, e.g. "http://127.0.0.1:8090",
	// besides the origin of the request's own (allowed) host. Browser requests from other origins
	// are rejected.
	AllowedOrigins []string

	// RequireAuth requires the auth-proxy headers and the viewer role.
	RequireAuth bool

	// RateLimitQPS and RateLimitBurst limit the requests per caller.
	RateLimitQPS   float64
	RateLimitBurst int

	// RequestTimeout bounds the time spent on each request, including database queries.
	RequestTimeout time.Duration
}

// newHandler returns the handler that serves the REST API, the MCP endpoint, the landing page and
// the health check.
func newHandler(svc *insights.Service, s *site.Site, opts serverOptions) http.Handler {
	limiter := newRateLimiter(opts.RateLimitQPS, opts.RateLimitBurst, opts.RequireAuth)
	mcpHandler := mcptools.NewHTTPHandler(mcptools.NewServer(svc))

	r := chi.NewRouter()
	r.Use(recoverPanics, accessLog, checkHost(opts.AllowedHosts, opts.AllowedHostSuffixes), checkOrigin(opts.AllowedOrigins), securityHeaders)
	r.NotFound(notFound)
	r.MethodNotAllowed(methodNotAllowed)
	r.Group(func(r chi.Router) {
		r.Use(limiter.middleware, withTimeout(opts.RequestTimeout))
		// The health check needs no credentials, so probes can call it.
		r.Get("/healthz", healthz(svc))
		r.Group(func(r chi.Router) {
			if opts.RequireAuth {
				r.Use(requireViewer)
			}
			rest.New(svc).RegisterHandlers(r)
			r.Handle("/mcp", mcpHandler)
			s.RegisterHandlers(r)
		})
	})
	return r
}

func writeError(w http.ResponseWriter, status int, e *insights.Error) {
	rest.WriteJSON(w, status, insights.ErrorEnvelope{Error: e})
}

func notFound(w http.ResponseWriter, r *http.Request) {
	rest.WriteError(w, insights.NotFound("see /llms.txt or /openapi.json for the available endpoints", "no such endpoint"))
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, &insights.Error{
		Code:    insights.CodeInvalidArgument,
		Message: "method not allowed",
		Hint:    "see /openapi.json for the methods of each endpoint",
	})
}

// recoverPanics turns panics into generic 500 errors. The panic is only logged.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				sklog.Errorf("Panic serving %s %q: %v\n%s", r.Method, r.URL.Path, rec, debug.Stack())
				rest.WriteError(w, &insights.Error{Code: insights.CodeInternal, Message: "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// accessLog logs each request. Only the method and path are logged: query strings and bodies carry
// user input, and headers may carry credentials.
func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		sklog.Infof("%s %q %d %dB %s", r.Method, r.URL.Path, ww.Status(), ww.BytesWritten(), time.Since(start).Round(time.Millisecond))
	})
}

func toSet(values []string) map[string]bool {
	ret := make(map[string]bool, len(values))
	for _, v := range values {
		ret[strings.ToLower(v)] = true
	}
	return ret
}

// hostnameRE matches a DNS hostname, lower-cased.
var hostnameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

// hostAllowed returns true if the Host header value is in set, or if its hostname ends with one of
// the suffixes (which include their leading dot).
func hostAllowed(host string, set map[string]bool, suffixes []string) bool {
	host = strings.ToLower(host)
	if set[host] {
		return true
	}
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	if !hostnameRE.MatchString(name) {
		return false
	}
	for _, s := range suffixes {
		s = strings.ToLower(s)
		if strings.HasPrefix(s, ".") && len(name) > len(s) && strings.HasSuffix(name, s) {
			return true
		}
	}
	return false
}

// checkHost rejects requests whose Host header is not allowed. A web page that rebinds its own
// domain name to 127.0.0.1 sends its own domain as Host, so this stops it from reading the API.
func checkHost(allowed, suffixes []string) func(http.Handler) http.Handler {
	set := toSet(allowed)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !hostAllowed(r.Host, set, suffixes) {
				sklog.Warningf("Rejected a request for host %q.", clipForLog(r.Host))
				writeError(w, http.StatusForbidden, &insights.Error{
					Code:    insights.CodePermissionDenied,
					Message: "host not allowed",
					Hint:    "use the URL shown in the server logs, or start the server with --public_url",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clipForLog shortens s and removes control characters, so request values can't forge log lines.
func clipForLog(s string) string {
	if len(s) > 100 {
		s = s[:100]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// checkOrigin rejects browser requests from other origins. Browsers send Origin on cross-origin
// requests and on POSTs; command-line clients and MCP clients don't send it. The origin of the
// request's own host (already checked by checkHost) is always accepted. No CORS headers are ever
// set, so cross-origin pages also can't read responses.
func checkOrigin(allowed []string) func(http.Handler) http.Handler {
	set := toSet(allowed)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := strings.ToLower(r.Header.Get("Origin")); origin != "" && !set[origin] {
				host := strings.ToLower(r.Host)
				if origin != "https://"+host && origin != "http://"+host {
					writeError(w, http.StatusForbidden, &insights.Error{Code: insights.CodePermissionDenied, Message: "cross-origin requests are not allowed"})
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// securityHeaders sets the headers every response carries. The landing page adds its CSP.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// requireViewer requires an authenticated user with at least the viewer role.
//
// TODO(security): this trusts the X-WEBAUTH-USER and X-WEBAUTH-ROLES headers set by Skia's
// auth-proxy (which must run with --allow_post, since /mcp uses POST). The server must only be
// reachable through the proxy, e.g. enforced by a Kubernetes NetworkPolicy. The perf.v1 API plan
// replaces this with the perf-fe auth middleware.
func requireViewer(next http.Handler) http.Handler {
	allowed := roles.Roles{roles.Viewer, roles.Editor, roles.Admin}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get(webAuthUserHeader)) == "" {
			writeError(w, http.StatusUnauthorized, &insights.Error{
				Code:    insights.CodeUnauthenticated,
				Message: "authentication required",
				Hint:    "call this server through the auth proxy",
			})
			return
		}
		if !roles.FromHeader(r.Header.Get(webAuthRolesHeader)).IsAuthorized(allowed) {
			writeError(w, http.StatusForbidden, &insights.Error{Code: insights.CodePermissionDenied, Message: "the viewer role is required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// withTimeout bounds the time spent on each request. The Perf database pool requires every
// context to have a deadline.
func withTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func healthz(svc *insights.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Ping(r.Context()); err != nil {
			sklog.Errorf("Health check failed: %s", err)
			rest.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		rest.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// rateLimiter limits requests per caller, plus a server-wide limit that protects the database.
type rateLimiter struct {
	qps    rate.Limit
	burst  int
	byUser bool
	global *rate.Limiter

	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

func newRateLimiter(qps float64, burst int, byUser bool) *rateLimiter {
	return &rateLimiter{
		qps:      rate.Limit(qps),
		burst:    burst,
		byUser:   byUser,
		global:   rate.NewLimiter(rate.Limit(qps*globalRateFactor), burst*globalRateFactor),
		limiters: map[string]*rate.Limiter{},
	}
}

// callerKey identifies the caller: the authenticated user when auth is required, else the client
// IP address. X-Forwarded-For is not trusted.
func (l *rateLimiter) callerKey(r *http.Request) string {
	if l.byUser {
		if user := strings.TrimSpace(r.Header.Get(webAuthUserHeader)); user != "" {
			return "user:" + strings.ToLower(user)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}

func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	lim, ok := l.limiters[key]
	if !ok {
		if len(l.limiters) >= maxLimiters {
			// Dropping all limiters bounds memory; it briefly resets the per-caller limits, and the
			// global limit still applies.
			l.limiters = map[string]*rate.Limiter{}
		}
		lim = rate.NewLimiter(l.qps, l.burst)
		l.limiters[key] = lim
	}
	l.mu.Unlock()
	return lim.Allow() && l.global.Allow()
}

func (l *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(l.callerKey(r)) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, &insights.Error{
				Code:    insights.CodeResourceExhausted,
				Message: "too many requests",
				Hint:    "slow down and retry in a second",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
